package main

import (
	"log/slog"

	"rabbitmq-platform/internal/producer"
	"rabbitmq-platform/pkg/broker"
	"rabbitmq-platform/pkg/config"
	"rabbitmq-platform/pkg/logger"
)

const (
	message = "Hello, world"
)

type Message struct {
	Message string `json:"message"`
}

func main() {

	msg := Message{
		Message: "Hello, world",
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

	mq, err := broker.NewRabbitMQClient(cfg.RabbitMQDSN)
	if err != nil {
		logger.Error("Failed to create RabbitMQ client", "error", err)
		panic(err)
	}
	defer mq.Close()

	prdcr := producer.NewProducer(mq.GetPublisher())

	prdcr.PublishMessage(message)
	prdcr.PublishMessage(msg)
}
