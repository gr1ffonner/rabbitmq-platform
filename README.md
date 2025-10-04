
# RabbitMQ Platform

A Go-based messaging platform using RabbitMQ for reliable message queuing and processing.

## Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Make
- Environment configuration (.env file)

## Quick Start

### 1. Environment Setup
Create a `.env` file in the project root with:
```bash
RABBITMQ_DEFAULT_USER=admin
RABBITMQ_DEFAULT_PASSWORD=password
```

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

## Services

### RabbitMQ
- **Management UI**: http://localhost:15672
- **AMQP Port**: 5672
- **Management Port**: 15672
- **Default Credentials**: admin/password (configurable via .env)

## Project Structure
```
├── cmd/
│   ├── consumer/     # Consumer application
│   └── producer/     # Producer application
├── internal/
│   ├── consumer/     # Consumer implementation
│   └── producer/     # Producer implementation
├── pkg/
│   ├── broker/       # RabbitMQ client and publisher
│   ├── config/       # Configuration management
│   └── logger/       # Logging utilities
└── docs/             # Documentation
```

## Documentation
- [RabbitMQ Core Concepts](docs/rabbitmq-core.md)
- [Data Flow](docs/data-flow.md)
- [Load Balancing](docs/load-balancing.md)
- [Platform Comparison](docs/comparison.md)