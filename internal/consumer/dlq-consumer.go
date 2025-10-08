package consumer

import (
	"encoding/json"
	"errors"
	"log/slog"
	"rabbitmq-platform/pkg/broker"

	amqp "github.com/rabbitmq/amqp091-go"
)

var errFlag = true

type DLQConsumer struct {
	Config broker.ConsumerConfig
}

// NewTestDlqConsumerConfig creates a config for DLQ testing consumer
func NewDLQConsumer(exchange, queueName, routingKey string, isDLQNeeded bool) DLQConsumer {
	return DLQConsumer{
		Config: broker.ConsumerConfig{
			QueueName:   queueName,
			Exchange:    exchange,
			RoutingKey:  routingKey,
			IsDLQNeeded: isDLQNeeded,
		},
	}
}

// ProcessDlqMessage is a test process function for DLQ consumer
// It simulates failing once and then succeeding
func (c *DLQConsumer) ProcessMessage(msg amqp.Delivery) error {
	// For testing purposes - fail on first attempt
	if errFlag {
		errFlag = false // Reset flag after first failure
		return errors.New("random error for testing DLQ")
	}

	var jsonMsg any
	if err := json.Unmarshal(msg.Body, &jsonMsg); err == nil {
		slog.Info("dlq message received", "message", jsonMsg)
	} else {
		slog.Warn("dlq message received is not valid JSON")
		slog.Info("dlq message received", "message", string(msg.Body))
	}

	return nil
}
