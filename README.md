
# RabbitMQ Platform

## Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Make

## Quick Start

### 2. Infrastructure Management
```bash
# Start RabbitMQ server
make up

# Stop RabbitMQ server
make down
```

### 3. Run Applications
```bash
# Run consumer
make run-consumer

# Run producer
make run-producer
```

## Monitoring
- **Management UI**: http://localhost:15672