// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	// Database
	DatabaseURL string

	// Cache
	RedisURL string

	// Services
	PaymentOrchestratorURL string
	ProcessorAURL          string
	ProcessorBURL          string
	NetworkTokenURL        string
	BPASServiceURL         string
	SubscriptionServiceURL string

	// Server
	Port     string
	LogLevel string

	// External integrations
	ZuoraAPIURL   string
	ZuoraAPIKey   string
	ZuoraTenantID string

	// Timeouts
	HTTPTimeout      time.Duration
	DatabaseTimeout  time.Duration
	ProcessorTimeout time.Duration

	// Feature flags
	EnableNetworkTokens bool
	EnableDualVault     bool
	EnableIdempotency   bool

	// Operational
	MaxRetries      int
	RetryBackoffMs  int
	HealthCheckPath string
}

// Load creates a new config from environment variables
func Load() *Config {
	return &Config{
		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://payment_user:payment_pass@localhost:5432/payment_db"),

		// Cache
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379"),

		// Services
		PaymentOrchestratorURL: getEnv("PAYMENT_ORCHESTRATOR_URL", "http://payment-orchestrator:8001"),
		ProcessorAURL:          getEnv("PROCESSOR_A_URL", "http://mock-processor-a:8101"),
		ProcessorBURL:          getEnv("PROCESSOR_B_URL", "http://mock-processor-b:8102"),
		NetworkTokenURL:        getEnv("NETWORK_TOKEN_URL", "http://network-token-service:8103"),
		BPASServiceURL:         getEnv("BPAS_SERVICE_URL", "http://bpas-service:8003"),
		SubscriptionServiceURL: getEnv("SUBSCRIPTION_SERVICE_URL", "http://subscription-service:8002"),

		// Server
		Port:     getEnv("PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// External integrations (Zuora not used in prototype)
		ZuoraAPIURL:   getEnv("ZUORA_API_URL", ""),
		ZuoraAPIKey:   getEnv("ZUORA_API_KEY", ""),
		ZuoraTenantID: getEnv("ZUORA_TENANT_ID", ""),

		// Timeouts
		HTTPTimeout:      getDurationEnv("HTTP_TIMEOUT", 30*time.Second),
		DatabaseTimeout:  getDurationEnv("DATABASE_TIMEOUT", 10*time.Second),
		ProcessorTimeout: getDurationEnv("PROCESSOR_TIMEOUT", 5*time.Second),

		// Feature flags
		EnableNetworkTokens: getBoolEnv("ENABLE_NETWORK_TOKENS", true),
		EnableDualVault:     getBoolEnv("ENABLE_DUAL_VAULT", true),
		EnableIdempotency:   getBoolEnv("ENABLE_IDEMPOTENCY", true),

		// Operational
		MaxRetries:      getIntEnv("MAX_RETRIES", 3),
		RetryBackoffMs:  getIntEnv("RETRY_BACKOFF_MS", 1000),
		HealthCheckPath: getEnv("HEALTH_CHECK_PATH", "/health"),
	}
}

// LoadForService loads configuration specific to a service
func LoadForService(serviceName string) *Config {
	cfg := Load()

	// Service-specific port overrides
	switch serviceName {
	case "api-gateway":
		cfg.Port = getEnv("PORT", "8080")
	case "payment-orchestrator":
		cfg.Port = getEnv("PORT", "8001")
	case "subscription-service":
		cfg.Port = getEnv("PORT", "8002")
	case "bpas-service":
		cfg.Port = getEnv("PORT", "8003")
	case "network-token-service":
		cfg.Port = getEnv("PORT", "8103")
	case "mit-scheduler":
		cfg.Port = getEnv("PORT", "8004")
	case "mock-processor-a":
		cfg.Port = getEnv("PORT", "8101")
	case "mock-processor-b":
		cfg.Port = getEnv("PORT", "8102")
	}

	return cfg
}

// GetProcessorConfig returns processor-specific configuration
func (c *Config) GetProcessorConfig() map[string]string {
	return map[string]string{
		"processor_a": c.ProcessorAURL,
		"processor_b": c.ProcessorBURL,
	}
}

// GetServiceURLs returns a map of all service URLs
func (c *Config) GetServiceURLs() map[string]string {
	return map[string]string{
		"payment_orchestrator": c.PaymentOrchestratorURL,
		"processor_a":          c.ProcessorAURL,
		"processor_b":          c.ProcessorBURL,
		"network_token":        c.NetworkTokenURL,
		"bpas":                 c.BPASServiceURL,
		"subscription":         c.SubscriptionServiceURL,
	}
}

// IsProductionMode checks if running in production environment
func (c *Config) IsProductionMode() bool {
	return getEnv("ENV", "development") == "production"
}

// IsDevelopmentMode checks if running in development environment
func (c *Config) IsDevelopmentMode() bool {
	return getEnv("ENV", "development") == "development"
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if c.Port == "" {
		return fmt.Errorf("PORT is required")
	}
	return nil
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
