package broker

import (
	"log/slog"
	"rabbitmq-platform/pkg/dlq"
	"sync"
	"time"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

type AMQPConn struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewAMQPConn(dsn string) (*AMQPConn, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to rabbitmq")
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create channel")
	}
	return &AMQPConn{Conn: conn, Channel: ch}, nil
}

func (r *AMQPConn) Close() error {
	_ = r.Channel.Close()
	return r.Conn.Close()
}

type Consumer struct {
	RoutingKey  string
	QueueName   string
	Exchange    string
	IsDLQNeeded bool
	ProcessFunc func(msg amqp.Delivery) error // should return err for dlq handling
}

type RabbitMQClient struct {
	mu            sync.Mutex
	dsn           string
	conn          *AMQPConn
	consumers     []Consumer
	done          chan struct{}
	reconnectTime time.Duration
	pub           Publisher
}

func NewRabbitMQClient(dsn string) (*RabbitMQClient, error) {
	client := &RabbitMQClient{
		dsn:           dsn,
		done:          make(chan struct{}),
		reconnectTime: 5 * time.Second,
	}
	if err := client.connectAndDeclare(); err != nil {
		return nil, err
	}
	go client.monitorReconnect()
	return client, nil
}

func (c *RabbitMQClient) connectAndDeclare() error {
	conn, err := NewAMQPConn(c.dsn)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	c.pub = &publisher{
		ch: conn.Channel,
	}
	return nil
}

func (c *RabbitMQClient) RunConsumers(consumers ...Consumer) {
	c.consumers = consumers
	c.launchConsumers()
}

func (c *RabbitMQClient) launchConsumers() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cons := range c.consumers {
		ch := c.conn.Channel

		err := ch.ExchangeDeclare(cons.Exchange, "topic", true, false, false, false, nil)
		if err != nil {
			slog.Error("cannot declare main exchange", "error", err, "exchange", cons.Exchange)
			continue
		}

		mainQueueArgs := amqp.Table{}
		if cons.IsDLQNeeded {
			// DLQ with TTL (auto-republishes back to main exchange)
			dlqQueueName := cons.QueueName + dlq.DLQSuffix
			dlqRoutingKey := cons.RoutingKey + dlq.DLQSuffix

			slog.Info("declaring dead letter queue", "queue", dlqQueueName, "rk", dlqRoutingKey, "exchange", cons.Exchange)

			dlqQueueArgs := amqp.Table{
				"x-message-ttl":             int32(dlq.RetryTTLSeconds * 1000), // 30 seconds TTL
				"x-dead-letter-exchange":    cons.Exchange,                     // Auto-republish to main exchange
				"x-dead-letter-routing-key": cons.RoutingKey,                   // Back to original routing key
			}
			if _, err := ch.QueueDeclare(dlqQueueName, true, false, false, false, dlqQueueArgs); err != nil {
				slog.Error("failed to declare DLQ", "error", err, "queue", dlqQueueName)
				continue
			}

			if err := ch.QueueBind(dlqQueueName, dlqRoutingKey, cons.Exchange, false, nil); err != nil {
				slog.Error("failed to bind DLQ", "error", err, "queue", dlqQueueName)
				continue
			}

			mainQueueArgs = amqp.Table{
				"x-dead-letter-exchange":    cons.Exchange,
				"x-dead-letter-routing-key": dlqRoutingKey, // Send failed messages to DLQ routing key
			}
		}

		slog.Info("declaring main queue", "queue", cons.QueueName, "rk", cons.RoutingKey, "exchange", cons.Exchange)
		if _, err := ch.QueueDeclare(cons.QueueName, true, false, false, false, mainQueueArgs); err != nil {
			slog.Error("failed to declare main queue", "error", err, "queue", cons.QueueName)
			continue
		}

		err = ch.QueueBind(cons.QueueName, cons.RoutingKey, cons.Exchange, false, nil)
		if err != nil {
			slog.Error("cannot bind queue", "error", err)
			continue
		}

		msgs, err := ch.Consume(cons.QueueName, "", false, false, false, false, nil)
		if err != nil {
			slog.Error("cannot consume queue", "error", err)
			continue
		}

		go func(cn Consumer, deliveries <-chan amqp.Delivery) {
			for msg := range deliveries {
				func() {
					defer func() {
						if r := recover(); r != nil {
							_ = msg.Nack(false, false)
						}
					}()

					retryCount := dlq.GetRetryCountFromHeaders(msg.Headers)
					slog.Info("start message processing",
						"retry_count", retryCount,
						"max_retries", dlq.MaxRetryAttempts,
						"queue", cn.QueueName,
						"rk", cn.RoutingKey,
					)
					err := cn.ProcessFunc(msg)
					if err != nil && retryCount < dlq.MaxRetryAttempts {
						slog.Warn("received error, nacking message", "error", err, "retry_count", retryCount, "max_retries", dlq.MaxRetryAttempts)
						// some logic here
						// retry/non retry err check logic
						// for now nack the message and return, let the broker handle it
						err = msg.Nack(false, false)
						if err != nil {
							slog.Error("failed to nack message", "error", err)
						}
						return
					} else if err != nil && retryCount >= dlq.MaxRetryAttempts {
						// max retries exceeded - ack the message and return
						slog.Error("max retries exceeded - ack the message", "error", err, "retry_count", retryCount)
						err = msg.Ack(false)
						if err != nil {
							slog.Error("failed to ack message", "error", err)
						}
					} else {
						// non error case ack the message and return
						slog.Info("message processed successfully", "retry_count", retryCount, "max_retries", dlq.MaxRetryAttempts)
						err = msg.Ack(false)
						if err != nil {
							slog.Error("failed to ack message", "error", err)
						}
						return
					}
				}()
			}
		}(cons, msgs)
	}
}

func (c *RabbitMQClient) monitorReconnect() {
	for {
		notify := c.conn.Conn.NotifyClose(make(chan *amqp.Error))
		err := <-notify
		if err != nil {
			slog.Error("rabbitmq connection closed", "error", err)
		}

		select {
		case <-c.done:
			return
		default:
			slog.Info("reconnecting to RabbitMQ...")
			for {
				time.Sleep(c.reconnectTime)
				if err := c.connectAndDeclare(); err != nil {
					slog.Error("reconnect failed", "error", err)
					continue
				}
				slog.Info("reconnected to RabbitMQ")
				c.launchConsumers()
				break
			}
		}
	}
}

func (c *RabbitMQClient) Close() error {
	close(c.done)
	return c.conn.Close()
}

func (c *RabbitMQClient) GetPublisher() Publisher {
	return c.pub
}
