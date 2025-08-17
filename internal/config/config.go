package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL             string
	RedisURL                string
	Port                    string
	PaymentOrchestratorURL  string
	SubscriptionServiceURL  string
	BPASServiceURL          string
	ProcessorAURL           string
	ProcessorBURL           string
	NetworkTokenURL         string
	ConfigPath              string
	ProcessorName           string
	SuccessRate             int
	NetworkTokenSuccessRate int
}

func Load() *Config {
	return &Config{
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://payment_user:payment_pass@localhost:5432/payment_db?sslmode=disable"),
		RedisURL:                getEnv("REDIS_URL", "redis://localhost:6379"),
		Port:                    getEnv("PORT", "8080"),
		PaymentOrchestratorURL:  getEnv("PAYMENT_ORCHESTRATOR_URL", "http://localhost:8001"),
		SubscriptionServiceURL:  getEnv("SUBSCRIPTION_SERVICE_URL", "http://localhost:8002"),
		BPASServiceURL:          getEnv("BPAS_SERVICE_URL", "http://localhost:8003"),
		ProcessorAURL:           getEnv("PROCESSOR_A_URL", "http://localhost:8101"),
		ProcessorBURL:           getEnv("PROCESSOR_B_URL", "http://localhost:8102"),
		NetworkTokenURL:         getEnv("NETWORK_TOKEN_URL", "http://localhost:8103"),
		ConfigPath:              getEnv("CONFIG_PATH", "./configs/routing-rules.yaml"),
		ProcessorName:           getEnv("PROCESSOR_NAME", "processor_a"),
		SuccessRate:             getEnvInt("SUCCESS_RATE", 80),
		NetworkTokenSuccessRate: getEnvInt("NETWORK_TOKEN_SUCCESS_RATE", 95),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
