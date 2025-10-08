CMD_DIR= ./cmd/


# Run consumer
run-consumer:
	@export $$(grep -v '^#' .env | xargs) >/dev/null 2>&1; \
	go run $(CMD_DIR)/consumer/main.go

# Run producer
run-producer:
	@export $$(grep -v '^#' .env | xargs) >/dev/null 2>&1; \
	go run $(CMD_DIR)/producer/main.go


# Start all services
up-dev:
	COMPOSE_PROJECT_NAME=rabbitmq-platform docker compose -f docker-compose.yml  --env-file=.env up -d --build

# Stop all services
down:
	COMPOSE_PROJECT_NAME=rabbitmq-platform docker compose -f docker-compose.yml --env-file=.env down --remove-orphans

# Run tests
test:
	go test ./tests/... -v

# Run tests with race detection
test-race:
	go test -race ./tests/... -v

# Generate mocks using mockery
gen-mocks:
	mockery --name RabbitMQClientInterface --output ./mocks --dir ./pkg/broker

# Install mockery tool
install-mockery:
	go install github.com/vektra/mockery/v2@latest