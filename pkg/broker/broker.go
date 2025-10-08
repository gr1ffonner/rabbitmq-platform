package broker

import (
	"sync"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQClientInterface defines the interface for RabbitMQ client operations
// This interface is used for mocking in tests
type RabbitMQClientInterface interface {
	Close() error
	NewChannel() (*amqp.Channel, error)
	IsConnected() bool
	DeclareExchange(ch *amqp.Channel, name string) error
	DeclareQueue(ch *amqp.Channel, name string, args amqp.Table) (amqp.Queue, error)
	BindQueue(ch *amqp.Channel, queueName, routingKey, exchange string) error
}

// RabbitMQClient manages RabbitMQ connection and provides infrastructure methods
type RabbitMQClient struct {
	conn *amqp.Connection
	dsn  string
	mu   sync.RWMutex
}

// NewRabbitMQClient creates a new RabbitMQ client with established connection
func NewRabbitMQClient(dsn string) (*RabbitMQClient, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to rabbitmq")
	}

	return &RabbitMQClient{
		conn: conn,
		dsn:  dsn,
	}, nil
}

// Close closes the RabbitMQ connection
func (c *RabbitMQClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn.Close()
	}
	return nil
}

// NewChannel creates a new AMQP channel
// Each consumer/producer should have its own channel for thread-safety
func (c *RabbitMQClient) NewChannel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return nil, errors.New("connection is nil")
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create channel")
	}

	return ch, nil
}

// IsConnected checks if connection is alive
func (c *RabbitMQClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.conn != nil && !c.conn.IsClosed()
}

// DeclareExchange declares an exchange
func (c *RabbitMQClient) DeclareExchange(ch *amqp.Channel, name string) error {
	if ch == nil {
		return errors.New("channel is nil")
	}

	err := ch.ExchangeDeclare(
		name,
		"topic",
		true,
		false, // autoDelete
		false, // internal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return errors.Wrap(err, "failed to declare exchange")
	}

	return nil
}

// DeclareQueue declares a queue
func (c *RabbitMQClient) DeclareQueue(ch *amqp.Channel, name string, args amqp.Table) (amqp.Queue, error) {
	if ch == nil {
		return amqp.Queue{}, errors.New("channel is nil")
	}

	queue, err := ch.QueueDeclare(
		name,
		true,
		false, // autoDelete
		false, // exclusive
		false, // noWait
		args,
	)
	if err != nil {
		return amqp.Queue{}, errors.Wrap(err, "failed to declare queue")
	}

	return queue, nil
}

// BindQueue binds a queue to an exchange with a routing key
func (c *RabbitMQClient) BindQueue(ch *amqp.Channel, queueName, routingKey, exchange string) error {
	if ch == nil {
		return errors.New("channel is nil")
	}

	err := ch.QueueBind(
		queueName,
		routingKey,
		exchange,
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return errors.Wrap(err, "failed to bind queue")
	}

	return nil
}
