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

	cnsm := consumer.NewConsumer(ctx)
	mq.RunConsumers(cnsm)

	<-signalCtx.Done()
	logger.Info("Shutting down consumer")
}

// const (
// 	RabbitDSN          = "amqp://guest:guest@localhost:5672/"
// 	MetricsExchange    = "metrics.ums.sso"
// 	MetricsExchangeTyp = "topic"
// 	MetricsQueue       = "ums_sso_metrics_queue"
// 	MetricsRoutingKey  = "metrics.ums.sso.*"
// )

// func main() {
// 	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
// 	defer stop()

// 	conn, err := amqp.Dial(RabbitDSN)
// 	if err != nil {
// 		log.Fatalf("failed to connect to rabbit: %v", err)
// 	}
// 	defer conn.Close()

// 	ch, err := conn.Channel()
// 	if err != nil {
// 		log.Fatalf("failed to open channel: %v", err)
// 	}
// 	defer ch.Close()

// 	if err := ch.ExchangeDeclare(
// 		MetricsExchange,
// 		MetricsExchangeTyp,
// 		true,
// 		false,
// 		false,
// 		false,
// 		nil,
// 	); err != nil {
// 		log.Fatalf("failed to declare exchange: %v", err)
// 	}

// 	q, err := ch.QueueDeclare(
// 		MetricsQueue,
// 		true,
// 		false,
// 		false,
// 		false,
// 		nil,
// 	)
// 	if err != nil {
// 		log.Fatalf("failed to declare queue: %v", err)
// 	}

// 	if err := ch.QueueBind(
// 		q.Name,
// 		MetricsRoutingKey,
// 		MetricsExchange,
// 		false,
// 		nil,
// 	); err != nil {
// 		log.Fatalf("failed to bind queue: %v", err)
// 	}

// 	msgs, err := ch.Consume(
// 		q.Name,
// 		"",
// 		true,
// 		false,
// 		false,
// 		false,
// 		nil,
// 	)
// 	if err != nil {
// 		log.Fatalf("failed to start consumer: %v", err)
// 	}

// 	slog.Info("metrics consumer started",
// 		"exchange", MetricsExchange,
// 		"queue", q.Name,
// 		"routing_key", MetricsRoutingKey,
// 	)

// 	done := make(chan struct{})
// 	go func() {
// 		for m := range msgs {
// 			slog.Info("metric received",
// 				"rk", m.RoutingKey,
// 				"content_type", m.ContentType,
// 				"body", string(m.Body),
// 			)
// 		}
// 		close(done)
// 	}()

// 	<-ctx.Done()
// 	// allow some time to drain logs
// 	time.Sleep(300 * time.Millisecond)
// }
