package broker

import (
	"context"
	"encoding/json"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(ctx context.Context, exchange, routingKey string, body any) error
}

type publisher struct {
	ch *amqp.Channel
}

func (p *publisher) Publish(ctx context.Context, exchange, routingKey string, body any) error {
	if exchange != "" {
		if err := p.ch.ExchangeDeclare(
			exchange,
			"topic",
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			slog.Error("failed to declare exchange", "exchange", exchange, "error", err)
			return err
		}
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		slog.Error("failed to marshal msg body", "error", err)
		return err
	}

	return p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        bodyJSON,
	})

}
