package main

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL     string
	RedisURL        string
	ProcessorAURL   string
	ProcessorBURL   string
	NetworkTokenURL string
	BPASServiceURL  string
	Port            string
	LogLevel        string
}

func LoadConfig() *Config {
	cfg := &Config{
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://payment_user:payment_pass@localhost:5432/payment_db?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		ProcessorAURL:   getEnv("PROCESSOR_A_URL", "http://localhost:8101"),
		ProcessorBURL:   getEnv("PROCESSOR_B_URL", "http://localhost:8102"),
		NetworkTokenURL: getEnv("NETWORK_TOKEN_URL", "http://localhost:8103"),
		BPASServiceURL:  getEnv("BPAS_SERVICE_URL", "http://localhost:8003"),
		Port:            getEnv("PORT", "8001"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
	}

	log.Printf("Configuration loaded: Database=%s, Redis=%s",
		maskConnectionString(cfg.DatabaseURL),
		maskConnectionString(cfg.RedisURL))

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func maskConnectionString(conn string) string {
	if len(conn) > 20 {
		return conn[:20] + "..."
	}
	return conn
}
