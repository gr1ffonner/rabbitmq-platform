package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	RabbitMQDSN string `env:"RABBITMQ_DSN" env-default:"amqp://guest:guest@localhost:5672/"`
	Logger      Logger
}

type Logger struct {
	Level string `env:"LOG_LEVEL" env-default:"info"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	err := cleanenv.ReadEnv(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
