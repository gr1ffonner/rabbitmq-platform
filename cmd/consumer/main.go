package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"rabbitmq-platform/internal/consumer"
	"rabbitmq-platform/pkg/broker"
	"rabbitmq-platform/pkg/config"
	"rabbitmq-platform/pkg/logger"
	"syscall"
)

func main() {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cnsm := consumer.NewConsumer()
	dlqCnsm := consumer.NewDlqConsumer()

	mq.RunConsumers(cnsm, dlqCnsm)

	<-signalCtx.Done()
	logger.Info("Shutting down consumer")
}
