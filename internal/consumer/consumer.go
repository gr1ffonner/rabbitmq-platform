package consumer

import (
	"encoding/json"
	"log/slog"
	"rabbitmq-platform/pkg/broker"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	QueueName  = "test"
	RoutingKey = "test"
	Exchange   = "test"
)

func NewConsumer() broker.Consumer {
	return broker.Consumer{
		RoutingKey: RoutingKey,
		QueueName:  QueueName,
		Exchange:   Exchange,
		ProcessFunc: func(msg amqp.Delivery) error {
			logMessage(msg)
			return nil
		},
	}
}

func logMessage(msg amqp.Delivery) {
	var jsonMsg any
	if err := json.Unmarshal(msg.Body, &jsonMsg); err == nil {
		slog.Info("message received", "queue", QueueName, "routing_key", RoutingKey, "message", jsonMsg)
	} else {
		slog.Warn("message received is not valid JSON")
		slog.Info("message received", "queue", QueueName, "routing_key", RoutingKey, "message", string(msg.Body))
	}
}
