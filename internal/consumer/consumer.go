package consumer

import (
	"encoding/json"
	"log/slog"
	"rabbitmq-platform/pkg/broker"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	Config broker.ConsumerConfig
}

// Consumer for logging messages
func NewConsumer(exchange, queueName, routingKey string, isDLQNeeded bool) Consumer {
	return Consumer{
		Config: broker.ConsumerConfig{
			QueueName:   queueName,
			Exchange:    exchange,
			RoutingKey:  routingKey,
			IsDLQNeeded: isDLQNeeded,
		},
	}
}

// ProcessMessage is a process function for consumer
func (c *Consumer) ProcessMessage(msg amqp.Delivery) error {

	var jsonMsg any
	if err := json.Unmarshal(msg.Body, &jsonMsg); err == nil {
		slog.Info("message received", "message", jsonMsg)
	} else {
		slog.Warn("message received is not valid JSON")
		slog.Info("message received", "message", string(msg.Body))
	}

	return nil
}
