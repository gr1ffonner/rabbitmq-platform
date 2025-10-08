# Архитектура RabbitMQ Platform

## Разделение ответственности

### pkg/broker - Низкоуровневый слой (инфраструктура)
Только работа с соединением и базовые операции RabbitMQ. Никакой бизнес-логики.

#### broker.go - Управление соединением
```go
type RabbitMQClient struct {
    conn *AMQPConn
    dsn  string
    mu   sync.RWMutex
}

// Методы:
func NewRabbitMQClient(dsn string) (*RabbitMQClient, error)
func (c *RabbitMQClient) Close() error
func (c *RabbitMQClient) GetChannel() (*amqp.Channel, error)
func (c *RabbitMQClient) IsConnected() bool
```

#### consumer.go - Низкоуровневый консьюмер
```go
type ConsumerClient struct {
    client *RabbitMQClient
}

// Методы:
func NewConsumerClient(client *RabbitMQClient) *ConsumerClient
func (c *ConsumerClient) DeclareQueue(name string, durable, autoDelete bool, args amqp.Table) (amqp.Queue, error)
func (c *ConsumerClient) DeclareExchange(name, kind string, durable, autoDelete bool) error
func (c *ConsumerClient) BindQueue(queue, routingKey, exchange string) error
func (c *ConsumerClient) Consume(queue, consumerTag string, autoAck bool) (<-chan amqp.Delivery, error)
func (c *ConsumerClient) Ack(tag uint64, multiple bool) error
func (c *ConsumerClient) Nack(tag uint64, multiple, requeue bool) error
func (c *ConsumerClient) Cancel(consumerTag string) error
```

#### producer.go - Низкоуровневый продюсер
```go
type ProducerClient struct {
    client *RabbitMQClient
}

// Методы:
func NewProducerClient(client *RabbitMQClient) *ProducerClient
func (c *ProducerClient) DeclareExchange(name, kind string, durable, autoDelete bool) error
func (c *ProducerClient) Publish(ctx context.Context, exchange, routingKey string, msg amqp.Publishing) error
```

---

### pkg/broker/consumer.go - Консьюмер с инфраструктурой
Управление процессом консьюминга с DLQ и retry логикой на уровне инфраструктуры.

```go
type Consumer struct {
    client      *RabbitMQClient
    ch          *amqp.Channel
    config      ConsumerConfig
    processFunc ProcessFunc
    stopCh      chan struct{}
    wg          sync.WaitGroup  // инициализируется zero value
}

type ConsumerConfig struct {
    QueueName   string
    Exchange    string
    RoutingKey  string
    IsDLQNeeded bool
}

type ProcessFunc func(msg amqp.Delivery) error

// Методы:
func NewConsumer(client *RabbitMQClient, config ConsumerConfig, processFunc ProcessFunc) (*Consumer, error)
func (c *Consumer) Start(ctx context.Context) error
func (c *Consumer) Stop() error
func (c *Consumer) setupQueue() error          // настройка queue, exchange, binding
func (c *Consumer) setupDLQ() error           // настройка DLQ с TTL для auto-retry
func (c *Consumer) startConsuming(ctx context.Context) error
func (c *Consumer) handleMessage(msg amqp.Delivery)  // обработка с retry/dlq логикой
```

### internal/consumer - Бизнес-специфичные консьюмеры
Содержит только конфиги и ProcessFunc для разных типов консьюмеров.

#### consumer.go
```go
type Consumer struct {
    Config broker.ConsumerConfig
}

func NewConsumer(exchange, queueName, routingKey string, isDLQNeeded bool) Consumer
func (c *Consumer) ProcessMessage(msg amqp.Delivery) error  // бизнес-логика обработки
```

#### dlq-consumer.go
```go
type DLQConsumer struct {
    Config broker.ConsumerConfig
}

func NewDLQConsumer(exchange, queueName, routingKey string, isDLQNeeded bool) DLQConsumer
func (c *DLQConsumer) ProcessMessage(msg amqp.Delivery) error  // логика обработки DLQ
```

---

### internal/producer - Бизнес-логика продюсера
Управление публикацией, форматирование сообщений.

#### producer.go
```go
type Producer struct {
    producerClient *broker.ProducerClient
    defaultExchange string
}

// Методы:
func NewProducer(client *broker.ProducerClient, defaultExchange string) *Producer
func (p *Producer) PublishMessage(ctx context.Context, exchange, routingKey string, body interface{}) error
func (p *Producer) PublishJSON(ctx context.Context, exchange, routingKey string, body interface{}) error
func (p *Producer) setupExchange(exchange string) error
```

---

## Поток данных

### Consumer Flow
```
cmd/consumer/main.go
    ↓
    создает RabbitMQClient (pkg/broker)
    ↓
    создает internal.Consumer (с Config и ProcessFunc)
    ↓
    создает broker.Consumer (pkg/broker/consumer.go)
    ↓
    broker.Consumer.Start() → setupQueue() → setupDLQ() → startConsuming()
    ↓
    получает сообщения из канала (goroutine с wg)
    ↓
    handleMessage() → internal.Consumer.ProcessFunc() → ack/nack
```

### Producer Flow
```
cmd/producer/main.go
    ↓
    создает RabbitMQClient (pkg/broker)
    ↓
    создает ProducerClient (pkg/broker)
    ↓
    создает Producer (internal/producer)
    ↓
    Producer.PublishMessage() → setupExchange() → ProducerClient.Publish()
```

---

## Преимущества нового подхода

1. **Разделение ответственности**: pkg только инфраструктура, internal только бизнес-логика
2. **Тестируемость**: можно мокать ConsumerClient/ProducerClient
3. **Переиспользование**: pkg может использоваться в других проектах
4. **Гибкость**: легко добавить новую логику в internal без изменения pkg
5. **Управляемость**: можно запускать/останавливать отдельные консьюмеры
6. **Масштабируемость**: просто добавить новые типы консьюмеров (DLQ, retry и т.д.)

---

## Текущая реализация

### pkg/broker содержит:

✅ `RabbitMQClient` - управление соединением  
✅ `Consumer` struct с `ProcessFunc` - инфраструктурный консьюмер  
✅ DLQ логика - настройка очередей с TTL и retry  
✅ Retry логика - подсчет попыток через headers  
✅ `sync.WaitGroup` - для graceful shutdown  
✅ Методы для работы с channel, exchange, queue  
✅ Базовые Publish/Consume операции  
✅ Ack/Nack методы  

### internal/consumer содержит:

✅ Бизнес-специфичные `Consumer` и `DLQConsumer`  
✅ `ProcessFunc` реализации для разных типов обработки  
✅ Конфигурации для каждого типа консьюмера  
✅ JSON unmarshalling и логирование сообщений  

---

## Миграция существующего кода

### До:
```go
// pkg/broker содержит все
consumer := broker.Consumer{
    ProcessFunc: func(msg amqp.Delivery) error { ... },
}
client.RunConsumers(consumer)
```

### После:
```go
// pkg/broker - соединение
client := broker.NewRabbitMQClient(dsn)

// internal/consumer - бизнес-специфичный консьюмер с ProcessFunc
cnsmr := consumer.NewConsumer(exchange, queueName, routingKey, isDLQNeeded)

// pkg/broker - инфраструктурный консьюмер
pkgConsumer, err := broker.NewConsumer(client, cnsmr.Config, cnsmr.ProcessMessage)
pkgConsumer.Start(ctx)
```

---

## Важные детали реализации

### WaitGroup инициализация
`sync.WaitGroup` в `Consumer` struct инициализируется zero value - это безопасно и является стандартной практикой Go:
```go
type Consumer struct {
    wg sync.WaitGroup  // zero value готов к использованию
}
```

### Graceful Shutdown
1. Context cancellation через `ctx.Done()`
2. Или manual stop через `stopCh`
3. `wg.Wait()` ждет завершения всех goroutines
4. Channel close в конце

### DLQ Exchange Configuration
**Важно**: Producer и Consumer должны использовать одинаковый exchange для DLQ сообщений.

**Текущая проблема в cmd/consumer/main.go:**
```go
DLQExchange = "dlq-test"  // ❌ отдельный exchange
```

**Правильно:**
```go
DLQExchange = "test"  // ✅ тот же exchange, что и для основных сообщений
```

DLQ сообщения должны публиковаться в тот же exchange с суффиксом routing key (`-dlq`), а не в отдельный exchange.

