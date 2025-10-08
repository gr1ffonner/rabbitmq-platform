package producer

import (
	"context"
	"encoding/json"
	"log/slog"
	"rabbitmq-platform/pkg/broker"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Producer manages message publishing to RabbitMQ
type Producer struct {
	client          *broker.RabbitMQClient
	ch              *amqp.Channel
	defaultExchange string
}

// NewProducer creates a new producer instance
func NewProducer(client *broker.RabbitMQClient, defaultExchange string) (*Producer, error) {
	ch, err := client.NewChannel()
	if err != nil {
		return nil, err
	}

	producer := &Producer{
		client:          client,
		ch:              ch,
		defaultExchange: defaultExchange,
	}

	// Declare default exchange if provided
	if defaultExchange != "" {
		if err := client.DeclareExchange(ch, defaultExchange); err != nil {
			return nil, errors.Wrap(err, "failed to declare default exchange")
		}
	}

	return producer, nil
}

// Close closes the producer's channel
func (p *Producer) Close() error {
	if p.ch != nil {
		return p.ch.Close()
	}
	return nil
}

// PublishMessage publishes a message to the specified exchange with routing key
// Uses default exchange if exchange parameter is empty
func (p *Producer) PublishMessage(ctx context.Context, exchange, routingKey string, message any) error {
	targetExchange := exchange
	if targetExchange == "" {
		targetExchange = p.defaultExchange
	}

	slog.Info("publishing message", "exchange", targetExchange, "routingKey", routingKey)

	// Ensure exchange exists
	if targetExchange != "" {
		if err := p.client.DeclareExchange(p.ch, targetExchange); err != nil {
			slog.Error("failed to declare exchange", "exchange", targetExchange, "error", err)
			return err
		}
	}

	// Marshal message to JSON
	bodyJSON, err := json.Marshal(message)
	if err != nil {
		slog.Error("failed to marshal message body", "error", err)
		return errors.Wrap(err, "failed to marshal message")
	}

	// Publish message
	err = p.ch.PublishWithContext(ctx, targetExchange, routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        bodyJSON,
	})

	if err != nil {
		slog.Error("failed to publish message", "error", err)
		return errors.Wrap(err, "failed to publish message")
	}

	slog.Info("message published", "exchange", targetExchange, "routingKey", routingKey, "message", message)
	return nil
}
