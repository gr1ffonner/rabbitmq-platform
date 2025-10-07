package producer

import (
	"context"
	"errors"
	"log/slog"
	"rabbitmq-platform/pkg/broker"
	"time"
)

type Producer struct {
	publisher broker.Publisher
}

func NewProducer(publisher broker.Publisher) *Producer {
	return &Producer{
		publisher: publisher,
	}
}

// publishMessage publishes synchronously
func (c *Producer) PublishMessage(exchange, routingKey string, message any) error {
	slog.Info("publishing message", "exchange", exchange, "routingKey", routingKey)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if c.publisher == nil {
		slog.Error("publisher is nil, cannot publish metric")
		return errors.New("publisher is nil")
	}
	if err := c.publisher.Publish(ctx, exchange, routingKey, message); err != nil {
		slog.Error("failed to publish message", "error", err)
		return err
	}
	slog.Info("message published", "message", message)
	return nil
}
