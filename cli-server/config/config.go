package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        string
	AccessKey   string
	MaxMessages int
	MessageTTL  time.Duration
}

func LoadFromEnv() *Config {
	return &Config{
		Port:        getEnv("PORT", "8034"),
		AccessKey:   getEnv("ACCESS_KEY", "secure_chat_key_2024"),
		MaxMessages: getEnvAsInt("MAX_MESSAGES", 1000),
		MessageTTL:  getEnvAsDuration("MESSAGE_TTL", 1*time.Minute),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
