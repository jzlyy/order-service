package config

import (
	"io/ioutil"
	"os"
	"strings"
)

type Config struct {
	DBUser          string
	DBPassword      string
	DBHost          string
	DBPort          string
	DBName          string
	JWTSecret       string
	RabbitMQURL     string
	OrderExchange   string
	OrderQueue      string
	DeadLetterQueue string
	DelayExchange   string
	MaxPriority     int
}

func LoadConfig() *Config {
	return &Config{
		DBUser:          getEnv("DB_USER", "root"),
		DBPassword:      getEnvFromFile("DB_PASSWORD_FILE", "DB_PASSWORD", "xxxxx"),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "3306"),
		DBName:          getEnv("DB_NAME", "ecommerce"),
		JWTSecret:       getEnvFromFile("JWT_SECRET_FILE", "JWT_SECRET", "G9mCQ19ogTkuWQY9jH2wGZASuGi/JrhstQaZy4k/01o="),
		RabbitMQURL:     getEnv("RABBITMQ_URL", "amqp://admin:rabbitmq@IP:5672/"),
		OrderExchange:   getEnv("ORDER_EXCHANGE", "orders_exchange"),
		OrderQueue:      getEnv("ORDER_QUEUE", "orders_queue"),
		DeadLetterQueue: getEnv("DEAD_LETTER_QUEUE", "dead_letter_queue"),
		DelayExchange:   getEnv("DELAY_EXCHANGE", "delay_exchange"),
		MaxPriority:     10, // 优先级队列最大优先级
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvFromFile(fileKey, envKey, defaultValue string) string {
	if filePath := os.Getenv(fileKey); filePath != "" {
		if content, err := ioutil.ReadFile(filePath); err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return getEnv(envKey, defaultValue)
}
