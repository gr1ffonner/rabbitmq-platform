package broker

import (
	"log/slog"
	"sync"
	"time"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

type AMQPConn struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewAMQPConn(dsn string) (*AMQPConn, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to rabbitmq")
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create channel")
	}
	return &AMQPConn{Conn: conn, Channel: ch}, nil
}

func (r *AMQPConn) Close() error {
	_ = r.Channel.Close()
	return r.Conn.Close()
}

type Consumer struct {
	RoutingKey  string
	QueueName   string
	Exchange    string
	ProcessFunc func(msg amqp.Delivery)
}

type RabbitMQClient struct {
	mu            sync.Mutex
	dsn           string
	conn          *AMQPConn
	consumers     []Consumer
	done          chan struct{}
	reconnectTime time.Duration
	pub           Publisher
}

func NewRabbitMQClient(dsn string) (*RabbitMQClient, error) {
	client := &RabbitMQClient{
		dsn:           dsn,
		done:          make(chan struct{}),
		reconnectTime: 5 * time.Second,
	}
	if err := client.connectAndDeclare(); err != nil {
		return nil, err
	}
	go client.monitorReconnect()
	return client, nil
}

func (c *RabbitMQClient) connectAndDeclare() error {
	conn, err := NewAMQPConn(c.dsn)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	c.pub = &publisher{
		ch: conn.Channel,
	}
	return nil
}

func (c *RabbitMQClient) RunConsumers(consumers ...Consumer) {
	c.consumers = consumers
	c.launchConsumers()
}

func (c *RabbitMQClient) launchConsumers() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cons := range c.consumers {
		// Declare exchange for each consumer
		err := c.conn.Channel.ExchangeDeclare(
			cons.Exchange,
			"topic",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			slog.Error("cannot declare exchange", "error", err)
			continue
		}

		_, err = c.conn.Channel.QueueDeclare(
			cons.QueueName,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			slog.Error("cannot declare queue", "error", err)
			continue
		}

		err = c.conn.Channel.QueueBind(
			cons.QueueName,
			cons.RoutingKey,
			cons.Exchange,
			false,
			nil,
		)
		if err != nil {
			slog.Error("cannot bind queue", "error", err)
			continue
		}

		msgs, err := c.conn.Channel.Consume(
			cons.QueueName,
			"",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			slog.Error("cannot consume queue", "error", err)
			continue
		}

		go func(cn Consumer, deliveries <-chan amqp.Delivery) {
			slog.Info("start consuming messages", "queue", cons.QueueName, "rk", cons.RoutingKey, "exchange", cons.Exchange)

			for msg := range deliveries {
				func() {
					defer func() {
						if r := recover(); r != nil {
							_ = msg.Nack(false, false)
						}
					}()

					cn.ProcessFunc(msg)

					err = msg.Ack(false)
					if err != nil {
						slog.Error("failed to ack message", "error", err)
					}
				}()
			}
		}(cons, msgs)
	}
}

func (c *RabbitMQClient) monitorReconnect() {
	for {
		notify := c.conn.Conn.NotifyClose(make(chan *amqp.Error))
		err := <-notify
		if err != nil {
			slog.Error("rabbitmq connection closed", "error", err)
		}

		select {
		case <-c.done:
			return
		default:
			slog.Info("reconnecting to RabbitMQ...")
			for {
				time.Sleep(c.reconnectTime)
				if err := c.connectAndDeclare(); err != nil {
					slog.Error("reconnect failed", "error", err)
					continue
				}
				slog.Info("reconnected to RabbitMQ")
				c.launchConsumers()
				break
			}
		}
	}
}

func (c *RabbitMQClient) Close() error {
	close(c.done)
	return c.conn.Close()
}

func (c *RabbitMQClient) GetPublisher() Publisher {
	return c.pub
}
