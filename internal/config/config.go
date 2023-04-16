package config

import (
	"os"
	"time"
)

type Config struct {
	Redis RedisConfig
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
  CounterTTL time.Duration
}

func LoadConfig() (*Config, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0

  counterTTL := 10*time.Second

	return &Config{
		Redis: RedisConfig{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       redisDB,
      CounterTTL: counterTTL,
		},
	}, nil
}


