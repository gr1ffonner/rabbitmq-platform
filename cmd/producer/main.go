package main

import (
	"context"
	"log/slog"
	"time"

	"rabbitmq-platform/internal/producer"
	"rabbitmq-platform/pkg/broker"
	"rabbitmq-platform/pkg/config"
	"rabbitmq-platform/pkg/logger"
)

const (
	exchange      = "test"
	routingKey    = "test"
	dlqExchange   = "dlq-test"
	dlqRoutingKey = "dlq-test"
)

type Message struct {
	Message string `json:"message"`
}

func main() {
	msg := Message{
		Message: "json test field",
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		panic(err)
	}
	slog.Info("Config loaded", "config", cfg)

	logger.InitLogger(cfg.Logger)
	logger := slog.Default()

	logger.Info("Config and logger initialized")

	// Create RabbitMQ client
	client, err := broker.NewRabbitMQClient(cfg.RabbitMQDSN)
	if err != nil {
		logger.Error("Failed to create RabbitMQ client", "error", err)
		panic(err)
	}
	defer client.Close()

	// Create producer with default exchange
	prdcr, err := producer.NewProducer(client, exchange)
	if err != nil {
		logger.Error("Failed to create producer", "error", err)
		panic(err)
	}
	defer prdcr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Publish to main exchange
	if err := prdcr.PublishMessage(ctx, exchange, routingKey, msg); err != nil {
		logger.Error("Failed to publish to main exchange", "error", err)
	}

	// Publish to DLQ exchange
	if err := prdcr.PublishMessage(ctx, dlqExchange, dlqRoutingKey, "dlq test message"); err != nil {
		logger.Error("Failed to publish to dlq exchange", "error", err)
	}

	logger.Info("Messages published successfully")
}
