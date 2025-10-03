# RabbitMQ Core Documentation

## Overview

RabbitMQ is a robust message broker that implements the Advanced Message Queuing Protocol (AMQP). This document covers core RabbitMQ functionality, architecture, and implementation patterns used in the platform.

## RabbitMQ Architecture

### Core Components

1. **Broker**: RabbitMQ server that routes and stores messages
2. **Exchanges**: Message routing components that receive messages from producers
3. **Queues**: Message storage that holds messages for consumers
4. **Bindings**: Rules that link exchanges to queues via routing keys
5. **Producers**: Applications that publish messages to exchanges
6. **Consumers**: Applications that receive messages from queues
7. **Routing Keys**: Message addressing used for routing decisions

### Exchange Types

```mermaid
graph TB
    A[Producer] -->|Publish with Routing Key| B[Exchange]
    
    B --> C[Direct Exchange<br/>Exact Match]
    B --> D[Topic Exchange<br/>Pattern Match]
    B --> E[Fanout Exchange<br/>Broadcast]
    B --> F[Headers Exchange<br/>Header Match]
    
    C --> G[Queue 1]
    D --> H[Queue 2]
    E --> I[Queue 3]
    F --> J[Queue 4]
```

**Exchange Types:**
- **Direct**: Routes messages with exact routing key match
- **Topic**: Routes messages using wildcard patterns (`*` and `#`)
- **Fanout**: Broadcasts messages to all bound queues
- **Headers**: Routes based on message headers (not routing key)

## Platform Implementation

### RabbitMQ Client

The platform provides a robust RabbitMQ client with automatic reconnection:

```go
type RabbitMQClient struct {
    mu            sync.Mutex
    dsn           string
    conn          *AMQPConn
    consumers     []Consumer
    done          chan struct{}
    reconnectTime time.Duration
    pub           Publisher
}
```

**Client Features:**
- **Auto-Reconnection**: Automatic reconnection on connection failures
- **Consumer Management**: Handles multiple consumers with different routing patterns
- **Publisher Interface**: Unified publishing interface with JSON marshaling
- **Error Recovery**: Graceful handling of network partitions and broker restarts

### Connection Management

```go
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
```

**Connection Features:**
- **Single Channel**: One channel per connection for simplicity
- **Error Wrapping**: Detailed error context for debugging
- **Resource Management**: Proper cleanup of connections and channels

### Producer Implementation

```go
type Publisher interface {
    Publish(ctx context.Context, exchange, routingKey string, body any) error
}

func (p *publisher) Publish(ctx context.Context, exchange, routingKey string, body any) error {
    // Auto-declare exchange if needed
    if exchange != "" {
        if err := p.ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
            return err
        }
    }
    
    // JSON marshal message body
    bodyJSON, err := json.Marshal(body)
    if err != nil {
        return err
    }
    
    // Publish with context
    return p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
        ContentType: "text/plain",
        Body:        bodyJSON,
    })
}
```

**Producer Features:**
- **Auto-Declaration**: Exchanges created automatically if they don't exist
- **JSON Serialization**: Automatic JSON marshaling of message bodies
- **Context Support**: Cancellation and timeout support
- **Topic Exchange**: Uses topic exchanges for flexible routing

### Consumer Implementation

```go
type Consumer struct {
    RoutingKey  string
    QueueName   string
    Exchange    string
    ProcessFunc func(msg amqp.Delivery)
}

func (c *RabbitMQClient) RunConsumers(consumers ...Consumer) {
    c.consumers = consumers
    c.launchConsumers()
}
```

**Consumer Features:**
- **Multiple Consumers**: Support for multiple consumers with different routing patterns
- **Auto-Declaration**: Queues, exchanges, and bindings created automatically
- **Manual Acknowledgment**: Manual message acknowledgment for reliability
- **Panic Recovery**: Graceful handling of consumer panics with NACK

## Message Flow Patterns

### 1. **Direct Messaging**

```mermaid
graph LR
    A[Producer] -->|routing.key| B[Direct Exchange]
    B -->|Exact Match| C[Queue: routing.key]
    C --> D[Consumer]
```

**Characteristics:**
- One-to-one messaging
- Exact routing key match
- Simple and predictable routing

### 2. **Topic-Based Routing**

```mermaid
graph LR
    A[Producer] -->|orders.new| B[Topic Exchange]
    B -->|Pattern: orders.*| C[Queue: order-processing]
    B -->|Pattern: orders.new| D[Queue: notifications]
    C --> E[Order Processor]
    D --> F[Notification Service]
```

**Characteristics:**
- Flexible pattern matching
- Wildcards: `*` (single word), `#` (multiple words)
- One-to-many routing based on patterns

### 3. **Fanout Broadcasting**

```mermaid
graph LR
    A[Producer] --> B[Fanout Exchange]
    B --> C[Queue 1]
    B --> D[Queue 2]
    B --> E[Queue N]
    C --> F[Consumer 1]
    D --> G[Consumer 2]
    E --> H[Consumer N]
```

**Characteristics:**
- Broadcast to all bound queues
- Ignores routing keys
- Useful for notifications and logging

## Queue and Exchange Declaration

### Exchange Declaration

```go
err := channel.ExchangeDeclare(
    "orders.exchange", // name
    "topic",          // type
    true,             // durable
    false,            // auto-deleted
    false,            // internal
    false,            // no-wait
    nil,              // arguments
)
```

### Queue Declaration

```go
queue, err := channel.QueueDeclare(
    "order.processing", // name
    true,              // durable
    false,             // delete when unused
    false,             // exclusive
    false,             // no-wait
    nil,               // arguments
)
```

### Queue Binding

```go
err := channel.QueueBind(
    "order.processing",  // queue name
    "orders.*",         // routing key
    "orders.exchange",  // exchange
    false,              // no-wait
    nil,                // arguments
)
```

## Error Handling & Reliability

### Message Acknowledgment

```go
// Manual acknowledgment for reliability
for msg := range deliveries {
    func() {
        defer func() {
            if r := recover(); r != nil {
                _ = msg.Nack(false, false) // Negative acknowledgment
            }
        }()
        
        consumer.ProcessFunc(msg)
        
        err = msg.Ack(false) // Positive acknowledgment
        if err != nil {
            slog.Error("failed to ack message", "error", err)
        }
    }()
}
```

**Acknowledgment Types:**
- **Ack**: Message processed successfully
- **Nack**: Message processing failed, can be requeued
- **Reject**: Similar to Nack but for single messages

### Connection Recovery

```go
func (c *RabbitMQClient) monitorReconnect() {
    for {
        notify := c.conn.Conn.NotifyClose(make(chan *amqp.Error))
        err := <-notify
        if err != nil {
            slog.Error("rabbitmq connection closed", "error", err)
        }
        
        // Reconnection logic with exponential backoff
        for {
            time.Sleep(c.reconnectTime)
            if err := c.connectAndDeclare(); err != nil {
                slog.Error("reconnect failed", "error", err)
                continue
            }
            slog.Info("reconnected to RabbitMQ")
            c.launchConsumers() // Restart consumers
            break
        }
    }
}
```

**Recovery Features:**
- **Automatic Reconnection**: Detects connection failures and reconnects
- **Consumer Restart**: Automatically restarts consumers after reconnection
- **Exponential Backoff**: Configurable reconnection intervals
- **Graceful Degradation**: Continues operation during temporary failures

## Configuration

### Connection Configuration

```go
// DSN format: amqp://username:password@host:port/vhost
dsn := "amqp://guest:guest@localhost:5672/"

client, err := NewRabbitMQClient(dsn)
if err != nil {
    return err
}
```

### Consumer Configuration

```go
consumers := []Consumer{
    {
        Exchange:    "orders.exchange",
        QueueName:   "order.processing",
        RoutingKey:  "orders.new",
        ProcessFunc: handleOrderMessage,
    },
    {
        Exchange:    "notifications.exchange", 
        QueueName:   "email.queue",
        RoutingKey:  "notifications.email.*",
        ProcessFunc: handleEmailNotification,
    },
}

client.RunConsumers(consumers...)
```

## Monitoring & Operations

### RabbitMQ Management UI

- **URL**: http://localhost:15672 (default)
- **Credentials**: guest/guest (default)
- **Features**: Queue monitoring, exchange management, message browsing

### Health Checks

```go
func (c *RabbitMQClient) HealthCheck() error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.conn == nil || c.conn.Conn.IsClosed() {
        return errors.New("rabbitmq connection is closed")
    }
    
    return nil
}
```

### Logging

Comprehensive structured logging with:
- Connection status and reconnection events
- Message processing status and errors
- Queue and exchange declaration results
- Consumer lifecycle events

## Best Practices

### Message Design

1. **Use JSON**: Consistent message format across services
2. **Include Headers**: Add metadata for routing and processing
3. **Idempotent Processing**: Design consumers to handle duplicate messages
4. **Error Handling**: Implement proper error handling and dead letter queues

### Queue Design

1. **Durable Queues**: Use durable queues for important messages
2. **Naming Convention**: Use consistent naming patterns
3. **TTL Settings**: Set appropriate message time-to-live
4. **Dead Letter Exchanges**: Configure for failed message handling

### Performance

1. **Connection Pooling**: Reuse connections across consumers
2. **Batch Processing**: Process messages in batches when possible
3. **Prefetch Count**: Configure appropriate prefetch limits
4. **Resource Cleanup**: Properly close connections and channels
