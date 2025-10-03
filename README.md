

### Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Make

### Commands

#### Infrastructure Management
```bash
# Start NATS server
make up

# Stop NATS server
make down
```

#### Application
```bash
# Run consumer
make run-consumer

# Run producer
make run-producer
```

## Monitoring
- **NATS Monitor**: http://localhost:8222