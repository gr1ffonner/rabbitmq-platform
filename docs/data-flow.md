# RabbitMQ Data Flow Documentation

## Overview

This document explains how data flows through the RabbitMQ platform implementation, covering producer-to-consumer message flow, exchange routing strategies, and queue management patterns.

## Architecture Overview

```mermaid
graph TB
    A[Producer Service] -->|Publish Messages| B[RabbitMQ Broker]
    B --> C[Exchange: orders.exchange]
    C --> D[Queue: order.processing]
    C --> E[Queue: order.notifications]
    C --> F[Queue: order.analytics]
    
    D --> G[Consumer 1<br/>Order Processor]
    E --> H[Consumer 2<br/>Notification Service]
    F --> I[Consumer 3<br/>Analytics Service]
```

## RabbitMQ Core Entities

### 1. **Exchanges** (Message Routing)
- **Purpose**: Route messages to queues based on routing rules
- **Types**: Direct, Topic, Fanout, Headers
- **Routing**: Uses routing keys and binding patterns

### 2. **Queues** (Message Storage)
- **Purpose**: Store messages until consumed
- **Features**: Durability, TTL, dead letter handling
- **FIFO**: First-in-first-out message delivery

### 3. **Bindings** (Routing Rules)
- **Purpose**: Link exchanges to queues with routing patterns
- **Patterns**: Exact match, wildcard patterns (`*`, `#`)
- **Multiple**: One queue can have multiple bindings

### 4. **Messages** (Data Carriers)
- **Purpose**: Carry application data with metadata
- **Features**: Headers, routing key, body, properties
- **Format**: JSON serialized message bodies

## Message Flow Patterns

### 1. **Direct Exchange Routing**

```mermaid
sequenceDiagram
    participant P as Producer
    participant E as Direct Exchange
    participant Q as Queue
    participant C as Consumer
    
    P->>E: Publish (routing.key="order.new")
    E->>Q: Route to Queue (binding="order.new")
    Q->>C: Deliver Message
    C->>Q: Ack Message
```

**Characteristics:**
- Exact routing key match
- One-to-one message delivery
- Simple and predictable routing

### 2. **Topic Exchange Routing**

```mermaid
sequenceDiagram
    participant P as Producer
    participant E as Topic Exchange
    participant Q1 as Queue 1 (orders.*)
    participant Q2 as Queue 2 (orders.new)
    participant C1 as Consumer 1
    participant C2 as Consumer 2
    
    P->>E: Publish (routing.key="orders.new")
    E->>Q1: Route (pattern match)
    E->>Q2: Route (exact match)
    Q1->>C1: Deliver Message
    Q2->>C2: Deliver Message
    C1->>Q1: Ack Message
    C2->>Q2: Ack Message
```

**Characteristics:**
- Pattern-based routing with wildcards
- One-to-many message delivery
- Flexible routing logic

### 3. **Fanout Exchange Broadcasting**

```mermaid
sequenceDiagram
    participant P as Producer
    participant E as Fanout Exchange
    participant Q1 as Queue 1
    participant Q2 as Queue 2
    participant Q3 as Queue 3
    participant C1 as Consumer 1
    participant C2 as Consumer 2
    participant C3 as Consumer 3
    
    P->>E: Publish Message
    E->>Q1: Broadcast
    E->>Q2: Broadcast
    E->>Q3: Broadcast
    Q1->>C1: Deliver
    Q2->>C2: Deliver
    Q3->>C3: Deliver
```

**Characteristics:**
- Broadcast to all bound queues
- Ignores routing keys
- Useful for notifications and logging

## Platform Implementation Flow

### Producer Message Flow

```mermaid
graph TD
    A[Producer Application] --> B[Create Message]
    B --> C[JSON Marshal Body]
    C --> D[Declare Exchange]
    D --> E[Publish to Exchange]
    E --> F{Success?}
    F -->|Yes| G[Continue]
    F -->|No| H[Log Error & Retry]
    H --> E
```

**Producer Flow Details:**
1. **Message Creation**: Application creates message with routing key and body
2. **JSON Serialization**: Message body automatically marshaled to JSON
3. **Exchange Declaration**: Topic exchange auto-declared if not exists
4. **Publishing**: Message published with context support
5. **Error Handling**: Automatic retry on transient failures

### Consumer Message Flow

```mermaid
graph TD
    A[Consumer Startup] --> B[Declare Exchange]
    B --> C[Declare Queue]
    C --> D[Bind Queue to Exchange]
    D --> E[Start Consuming]
    E --> F[Receive Message]
    F --> G[Process Message]
    G --> H{Success?}
    H -->|Yes| I[Ack Message]
    H -->|No| J[Nack Message]
    I --> F
    J --> K[Requeue or DLQ]
    K --> F
```

**Consumer Flow Details:**
1. **Setup Phase**: Declare exchanges, queues, and bindings
2. **Message Consumption**: Start consuming messages from queue
3. **Message Processing**: Execute custom processing function
4. **Acknowledgment**: Manual ack/nack based on processing result
5. **Error Recovery**: Panic recovery with automatic nack

## Routing Strategies

### Key-Based Routing (Direct)

```go
// Producer publishes with specific routing key
publisher.Publish(ctx, "orders.exchange", "order.new", orderData)

// Consumer binds to exact routing key
consumer := Consumer{
    Exchange:   "orders.exchange",
    QueueName:  "order.processing",
    RoutingKey: "order.new",
}
```

**Benefits:**
- Predictable message routing
- Simple one-to-one delivery
- Easy to understand and debug

### Pattern-Based Routing (Topic)

```go
// Producer publishes with hierarchical routing key
publisher.Publish(ctx, "events.exchange", "user.profile.updated", userData)

// Consumer binds with wildcard pattern
consumer := Consumer{
    Exchange:   "events.exchange", 
    QueueName:  "user.events",
    RoutingKey: "user.*",  // Matches user.profile.updated, user.login, etc.
}
```

**Wildcard Patterns:**
- `*` - matches exactly one word
- `#` - matches zero or more words
- Example: `orders.#` matches `orders.new`, `orders.new.priority`, etc.

### Broadcast Routing (Fanout)

```go
// All bound queues receive the message regardless of routing key
publisher.Publish(ctx, "broadcast.exchange", "", broadcastData)
```

## Message Transformation and Processing

### Message Structure

```go
type Message struct {
    Exchange    string
    RoutingKey  string
    Headers     map[string]interface{}
    Body        []byte
    Timestamp   time.Time
    MessageID   string
}
```

### JSON Message Processing

```go
func handleOrderMessage(msg amqp.Delivery) {
    var order Order
    if err := json.Unmarshal(msg.Body, &order); err != nil {
        slog.Error("failed to unmarshal order", "error", err)
        return
    }
    
    // Process order
    if err := processOrder(order); err != nil {
        slog.Error("failed to process order", "error", err)
        return
    }
    
    slog.Info("order processed successfully", "order_id", order.ID)
}
```

## Error Handling Flow

### Producer Error Handling

```mermaid
graph TD
    A[Publish Message] --> B{Exchange Exists?}
    B -->|No| C[Auto-Declare Exchange]
    C --> D[Retry Publish]
    B -->|Yes| E[Publish to Exchange]
    E --> F{Success?}
    F -->|Yes| G[Continue]
    F -->|No| H[Log Error]
    H --> I{Retryable?}
    I -->|Yes| J[Exponential Backoff]
    J --> E
    I -->|No| K[Fail Permanently]
```

### Consumer Error Handling

```mermaid
graph TD
    A[Receive Message] --> B[Process Message]
    B --> C{Success?}
    C -->|Yes| D[Ack Message]
    C -->|No| E{Panic?}
    E -->|Yes| F[Recover & Nack]
    E -->|No| G[Nack Message]
    F --> H[Log Error]
    G --> H
    H --> I{Requeue?}
    I -->|Yes| J[Requeue Message]
    I -->|No| K[Send to DLQ]
```

## Connection Management Flow

### Connection Lifecycle

```mermaid
graph TD
    A[Application Start] --> B[Connect to RabbitMQ]
    B --> C{Connection Success?}
    C -->|Yes| D[Create Channel]
    C -->|No| E[Retry Connection]
    E --> B
    D --> F[Setup Consumers]
    F --> G[Monitor Connection]
    G --> H{Connection Lost?}
    H -->|No| G
    H -->|Yes| I[Trigger Reconnection]
    I --> J[Wait Backoff Period]
    J --> B
```

### Reconnection Strategy

```go
func (c *RabbitMQClient) monitorReconnect() {
    for {
        notify := c.conn.Conn.NotifyClose(make(chan *amqp.Error))
        err := <-notify
        
        if err != nil {
            slog.Error("rabbitmq connection closed", "error", err)
            
            // Exponential backoff reconnection
            for {
                time.Sleep(c.reconnectTime)
                if err := c.connectAndDeclare(); err != nil {
                    slog.Error("reconnect failed", "error", err)
                    c.reconnectTime *= 2 // Exponential backoff
                    continue
                }
                
                slog.Info("reconnected to RabbitMQ")
                c.launchConsumers() // Restart all consumers
                c.reconnectTime = 5 * time.Second // Reset backoff
                break
            }
        }
    }
}
```

## Monitoring and Observability

### Key Metrics Tracked

1. **Producer Metrics:**
   - Messages published per second
   - Publishing success/failure rates
   - Exchange declaration events

2. **Consumer Metrics:**
   - Messages consumed per second
   - Processing success/failure rates
   - Queue depth and consumer lag

3. **Connection Metrics:**
   - Connection status and uptime
   - Reconnection frequency
   - Channel utilization

### RabbitMQ Management UI

Access RabbitMQ Management at:
- **URL**: http://localhost:15672
- **Credentials**: guest/guest (default)

**Features:**
- Queue and exchange monitoring
- Message rates and statistics
- Consumer connection status
- Message browsing and management