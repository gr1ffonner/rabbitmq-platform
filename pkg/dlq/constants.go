package dlq

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	MaxRetryAttempts = 3      // Максимум 3 попытки
	RetryTTLSeconds  = 30     // 30 секунд TTL для retry
	DLQSuffix        = "-dlq" // Суффикс для DLQ

)

// getRetryCountFromHeaders extracts retry count from x-death headers
func GetRetryCountFromHeaders(headers amqp.Table) int {
	if headers == nil {
		return 0
	}

	// Use x-death header which is automatically updated by RabbitMQ
	if deathValue, exists := headers["x-death"]; exists {
		if deathArray, ok := deathValue.([]interface{}); ok && len(deathArray) > 0 {
			if deathInfo, ok := deathArray[0].(amqp.Table); ok {
				if countValue, exists := deathInfo["count"]; exists {
					switch v := countValue.(type) {
					case int:
						return v
					case int32:
						return int(v)
					case int64:
						return int(v)
					}
				}
			}
		}
	}

	return 0
}
