package broker

import (
	"context"
	"log/slog"
	"rabbitmq-platform/pkg/dlq"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ProcessFunc defines the function signature for message processing
// Should return error if message processing fails (for DLQ handling)
type ProcessFunc func(msg amqp.Delivery) error

// ConsumerConfig holds configuration for a consumer
type ConsumerConfig struct {
	QueueName   string
	Exchange    string
	RoutingKey  string
	IsDLQNeeded bool
}

// Consumer manages message consumption from RabbitMQ
type Consumer struct {
	client      RabbitMQClientInterface
	ch          *amqp.Channel
	config      ConsumerConfig
	processFunc ProcessFunc
	stopCh      chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	stopped     bool
}

// NewConsumer creates a new consumer instance
func NewConsumer(client RabbitMQClientInterface, config ConsumerConfig, processFunc ProcessFunc) (*Consumer, error) {
	ch, err := client.NewChannel()
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client:      client,
		ch:          ch,
		config:      config,
		processFunc: processFunc,
		stopCh:      make(chan struct{}),
	}, nil
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	// Setup infrastructure
	if err := c.setupQueue(); err != nil {
		return err
	}
	// Start consuming
	return c.startConsuming(ctx)
}

func (c *Consumer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return nil // Already stopped
	}
	c.stopped = true

	close(c.stopCh)
	c.wg.Wait()

	if c.ch != nil {
		return c.ch.Close()
	}
	return nil
}

// setupQueue declares exchange, queue and binds them
func (c *Consumer) setupQueue() error {
	// Declare exchange
	if err := c.client.DeclareExchange(c.ch, c.config.Exchange); err != nil {
		slog.Error("cannot declare exchange", "error", err, "exchange", c.config.Exchange)
		return err
	}

	// Prepare queue arguments
	queueArgs := amqp.Table{}
	if c.config.IsDLQNeeded {
		if err := c.setupDLQ(); err != nil {
			slog.Error("cannot setup DLQ", "error", err)
			return err
		}
		dlqRoutingKey := c.config.RoutingKey + dlq.DLQSuffix
		queueArgs = amqp.Table{
			"x-dead-letter-exchange":    c.config.Exchange,
			"x-dead-letter-routing-key": dlqRoutingKey,
		}
	}

	// Declare queue
	slog.Info("declaring main queue", "queue", c.config.QueueName, "rk", c.config.RoutingKey, "exchange", c.config.Exchange)
	if _, err := c.client.DeclareQueue(c.ch, c.config.QueueName, queueArgs); err != nil {
		slog.Error("failed to declare main queue", "error", err, "queue", c.config.QueueName)
		return err
	}

	// Bind queue
	if err := c.client.BindQueue(c.ch, c.config.QueueName, c.config.RoutingKey, c.config.Exchange); err != nil {
		slog.Error("cannot bind queue", "error", err)
		return err
	}

	return nil
}

// setupDLQ sets up Dead Letter Queue with TTL
func (c *Consumer) setupDLQ() error {
	dlqQueueName := c.config.QueueName + dlq.DLQSuffix
	dlqRoutingKey := c.config.RoutingKey + dlq.DLQSuffix

	slog.Info("declaring dead letter queue", "queue", dlqQueueName, "rk", dlqRoutingKey, "exchange", c.config.Exchange)

	// DLQ arguments with TTL for auto-retry
	dlqArgs := amqp.Table{
		"x-message-ttl":             int32(dlq.RetryTTLSeconds * 1000), // TTL in milliseconds
		"x-dead-letter-exchange":    c.config.Exchange,                 // Republish to main exchange
		"x-dead-letter-routing-key": c.config.RoutingKey,               // Back to original routing key
	}

	// Declare DLQ
	if _, err := c.client.DeclareQueue(c.ch, dlqQueueName, dlqArgs); err != nil {
		slog.Error("failed to declare DLQ", "error", err, "queue", dlqQueueName)
		return err
	}

	// Bind DLQ
	if err := c.client.BindQueue(c.ch, dlqQueueName, dlqRoutingKey, c.config.Exchange); err != nil {
		slog.Error("failed to bind DLQ", "error", err, "queue", dlqQueueName)
		return err
	}

	return nil
}

// startConsuming starts the message consumption loop
func (c *Consumer) startConsuming(ctx context.Context) error {
	msgs, err := c.ch.Consume(
		c.config.QueueName,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		slog.Error("cannot start consuming", "error", err)
		return err
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		slog.Info("start consuming messages", "queue", c.config.QueueName, "rk", c.config.RoutingKey, "exchange", c.config.Exchange)

		for {
			select {
			case <-ctx.Done():
				slog.Info("context cancelled, stopping consumer")
				return
			case <-c.stopCh:
				slog.Info("stop signal received, stopping consumer")
				return
			case msg, ok := <-msgs:
				if !ok {
					slog.Warn("delivery channel closed")
					return
				}
				c.handleMessage(msg)
			}
		}
	}()

	return nil
}

// handleMessage processes a single message with retry/DLQ logic
func (c *Consumer) handleMessage(msg amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic during message processing", "panic", r)
			_ = msg.Nack(false, false)
		}
	}()

	retryCount := dlq.GetRetryCountFromHeaders(msg.Headers)
	slog.Info("start message processing",
		"retry_count", retryCount,
		"max_retries", dlq.MaxRetryAttempts,
		"queue", c.config.QueueName,
		"rk", c.config.RoutingKey,
	)

	// Log message content
	err := c.processFunc(msg)

	// Handle result based on error and retry count
	if err != nil && retryCount < dlq.MaxRetryAttempts {
		// Failed but can retry - send to DLQ
		slog.Warn("received error, nacking message", "error", err, "retry_count", retryCount, "max_retries", dlq.MaxRetryAttempts)
		if nackErr := msg.Nack(false, false); nackErr != nil {
			slog.Error("failed to nack message", "error", nackErr)
		}
		return
	} else if err != nil && retryCount >= dlq.MaxRetryAttempts {
		// Max retries exceeded - ack to remove from queue
		slog.Error("max retries exceeded - ack the message", "error", err, "retry_count", retryCount)
		if ackErr := msg.Ack(false); ackErr != nil {
			slog.Error("failed to ack message", "error", ackErr)
		}
		return
	}

	// Success - ack the message
	slog.Info("message processed successfully", "retry_count", retryCount, "max_retries", dlq.MaxRetryAttempts)
	if ackErr := msg.Ack(false); ackErr != nil {
		slog.Error("failed to ack message", "error", ackErr)
	}
}
