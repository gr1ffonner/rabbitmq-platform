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

const (
	QueueName  = "test"
	RoutingKey = "test"
	Exchange   = "test"

	DLQQueueName  = "dlq-test"
	DLQRoutingKey = "dlq-test"
	DLQExchange   = "dlq-test"
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

	// Create RabbitMQ client
	client, err := broker.NewRabbitMQClient(cfg.RabbitMQDSN)
	if err != nil {
		logger.Error("Failed to create RabbitMQ client", "error", err)
		panic(err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cnsmr := consumer.NewConsumer(Exchange, QueueName, RoutingKey, false)
	pkgConsumer, err := broker.NewConsumer(client, cnsmr.Config, cnsmr.ProcessMessage)
	if err != nil {
		logger.Error("Failed to create consumer", "error", err)
		panic(err)
	}
	defer pkgConsumer.Stop()

	dlqCnsmr := consumer.NewDLQConsumer(DLQExchange, DLQQueueName, DLQRoutingKey, true)
	pkgDlqConsumer, err := broker.NewConsumer(client, dlqCnsmr.Config, dlqCnsmr.ProcessMessage)
	if err != nil {
		logger.Error("Failed to create DLQ consumer", "error", err)
		panic(err)
	}
	defer pkgDlqConsumer.Stop()

	pkgConsumer.Start(ctx)
	pkgDlqConsumer.Start(ctx)

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("Consumers started, waiting for messages...")

	<-signalCtx.Done()
	logger.Info("Shutting down consumers...")

	// Stop consumers gracefully
	cancel()
	// wg.Wait()

	logger.Info("Consumers stopped successfully")
}
