package consumer

import (
	"encoding/json"
	"errors"
	"log/slog"
	"rabbitmq-platform/pkg/broker"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	testDlqQueueName  = "dlq-test"
	testDlqRoutingKey = "dlq-test"
	testDlqExchange   = "dlq-test"
)

var errFlag = true

func NewDlqConsumer() broker.Consumer {
	return broker.Consumer{
		RoutingKey:  testDlqRoutingKey,
		QueueName:   testDlqQueueName,
		Exchange:    testDlqExchange,
		IsDLQNeeded: true,
		ProcessFunc: func(msg amqp.Delivery) error {
			err := processDlq(msg)
			if err != nil {
				slog.Error(
					"failed to process dlq message",
					"err", err,
					"rk", testDlqRoutingKey,
					"queue", testDlqQueueName,
					"body", string(msg.Body),
				)
				return err
			}
			return nil
		},
	}
}

func processDlq(msg amqp.Delivery) error {

	// for testing purposes
	if errFlag {
		errFlag = false // Reset flag after first failure
		return errors.New("random error")
	}

	var jsonMsg any
	if err := json.Unmarshal(msg.Body, &jsonMsg); err == nil {
		slog.Info("message received", "message", jsonMsg)
	} else {
		slog.Warn("message received is not valid JSON")
		slog.Info("message received", "message", string(msg.Body))
	}

	return nil
}
