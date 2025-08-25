// internal/models/models.go
package models

import (
	"time"
)

// Subscription represents a user's subscription
type Subscription struct {
	ID                  string     `json:"id" db:"id"`
	UserID              string     `json:"user_id" db:"user_id"`
	Status              string     `json:"status" db:"status"`
	PlanID              string     `json:"plan_id" db:"plan_id"`
	Amount              float64    `json:"amount" db:"amount"`
	Currency            string     `json:"currency" db:"currency"`
	BillingCycle        string     `json:"billing_cycle" db:"billing_cycle"`
	NextBillingDate     *time.Time `json:"next_billing_date,omitempty" db:"next_billing_date"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
	ZuoraSubscriptionID string     `json:"zuora_subscription_id,omitempty" db:"zuora_subscription_id"`
	BusinessProfileID   string     `json:"business_profile_id,omitempty" db:"business_profile_id"`
}

// PaymentMethod represents a tokenized payment method
type PaymentMethod struct {
	ID                     string    `json:"id" db:"id"`
	UserID                 string    `json:"user_id" db:"user_id"`
	NetworkToken           string    `json:"network_token,omitempty" db:"network_token"`
	ProcessorAToken        string    `json:"processor_a_token,omitempty" db:"processor_a_token"`
	ProcessorBToken        string    `json:"processor_b_token,omitempty" db:"processor_b_token"`
	TokenType              string    `json:"token_type" db:"token_type"`
	LastFour               string    `json:"last_four" db:"last_four"`
	ExpMonth               int       `json:"exp_month,omitempty" db:"exp_month"`
	ExpYear                int       `json:"exp_year,omitempty" db:"exp_year"`
	IsDefault              bool      `json:"is_default" db:"is_default"`
	CreatedAt              time.Time `json:"created_at" db:"created_at"`
	AuthenticationSourceID string    `json:"authentication_source_id,omitempty" db:"authentication_source_id"`
}

// Transaction represents a payment transaction
type Transaction struct {
	ID                     string    `json:"id" db:"id"`
	SubscriptionID         string    `json:"subscription_id" db:"subscription_id"`
	PaymentMethodID        string    `json:"payment_method_id" db:"payment_method_id"`
	ProcessorUsed          string    `json:"processor_used" db:"processor_used"`
	Amount                 float64   `json:"amount" db:"amount"`
	Currency               string    `json:"currency" db:"currency"`
	Status                 string    `json:"status" db:"status"`
	TransactionType        string    `json:"transaction_type" db:"transaction_type"`
	IdempotencyKey         string    `json:"idempotency_key" db:"idempotency_key"`
	ProcessorTransactionID string    `json:"processor_transaction_id,omitempty" db:"processor_transaction_id"`
	ErrorCode              string    `json:"error_code,omitempty" db:"error_code"`
	ErrorMessage           string    `json:"error_message,omitempty" db:"error_message"`
	UserErrorMessage       string    `json:"user_error_message,omitempty" db:"user_error_message"`
	OriginalTransactionID  string    `json:"original_transaction_id,omitempty" db:"original_transaction_id"`
	NetworkTokenUsed       bool      `json:"network_token_used" db:"network_token_used"`
	RPSId                  string    `json:"rps_id,omitempty" db:"rps_id"`
	CreatedAt              time.Time `json:"created_at" db:"created_at"`
}

// RoutingRule represents business routing rules
type RoutingRule struct {
	ID                    string                 `json:"id" db:"id"`
	RuleName              string                 `json:"rule_name" db:"rule_name"`
	Priority              int                    `json:"priority" db:"priority"`
	ConditionType         string                 `json:"condition_type" db:"condition_type"`
	ConditionValue        map[string]interface{} `json:"condition_value" db:"condition_value"`
	TargetProcessor       string                 `json:"target_processor" db:"target_processor"`
	Percentage            int                    `json:"percentage" db:"percentage"`
	IsActive              bool                   `json:"is_active" db:"is_active"`
	SuccessRateThreshold  float64                `json:"success_rate_threshold" db:"success_rate_threshold"`
	FeeBasisPoints        int                    `json:"fee_basis_points" db:"fee_basis_points"`
	SupportedCurrencies   []string               `json:"supported_currencies" db:"supported_currencies"`
	SupportedMarketplaces []string               `json:"supported_marketplaces" db:"supported_marketplaces"`
	TrafficPercentage     int                    `json:"traffic_percentage" db:"traffic_percentage"`
	CreatedAt             time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at" db:"updated_at"`
}

// BusinessProfile represents client-specific routing configuration
type BusinessProfile struct {
	ID                    string                 `json:"id" db:"id"`
	ClientName            string                 `json:"client_name" db:"client_name"`
	WhitelistedProcessors []string               `json:"whitelisted_processors" db:"whitelisted_processors"`
	ProcessorDistribution map[string]interface{} `json:"processor_distribution" db:"processor_distribution"`
	CreatedAt             time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at" db:"updated_at"`
}

// RetryPolicy represents platform-specific retry configuration
type RetryPolicy struct {
	ID               string                 `json:"id" db:"id"`
	PolicyID         string                 `json:"policy_id" db:"policy_id"`
	Platform         string                 `json:"platform" db:"platform"`
	MaxAttempts      int                    `json:"max_attempts" db:"max_attempts"`
	GracePeriodHours int                    `json:"grace_period_hours" db:"grace_period_hours"`
	BackoffStrategy  map[string]interface{} `json:"backoff_strategy" db:"backoff_strategy"`
	Segment          string                 `json:"segment" db:"segment"`
}

// AuditLog represents compliance and debugging logs
type AuditLog struct {
	ID              string                 `json:"id" db:"id"`
	LogID           string                 `json:"log_id" db:"log_id"`
	TransactionID   string                 `json:"transaction_id" db:"transaction_id"`
	EventType       string                 `json:"event_type" db:"event_type"`
	Processor       string                 `json:"processor" db:"processor"`
	RequestPayload  map[string]interface{} `json:"request_payload" db:"request_payload"`
	ResponsePayload map[string]interface{} `json:"response_payload" db:"response_payload"`
	Timestamp       time.Time              `json:"timestamp" db:"timestamp"`
}

// ProcessorHealth represents processor health tracking
type ProcessorHealth struct {
	ID                string    `json:"id" db:"id"`
	ProcessorName     string    `json:"processor_name" db:"processor_name"`
	IsHealthy         bool      `json:"is_healthy" db:"is_healthy"`
	LastCheck         time.Time `json:"last_check" db:"last_check"`
	FailureCount      int       `json:"failure_count" db:"failure_count"`
	SuccessRate       float64   `json:"success_rate" db:"success_rate"`
	AvgResponseTimeMs int       `json:"avg_response_time_ms" db:"avg_response_time_ms"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// Constants for model values
const (
	// Subscription statuses
	SubscriptionStatusActive    = "active"
	SubscriptionStatusCancelled = "cancelled"
	SubscriptionStatusExpired   = "expired"

	// Transaction statuses
	TransactionStatusPending = "pending"
	TransactionStatusSuccess = "success"
	TransactionStatusFailed  = "failed"

	// Transaction types
	TransactionTypeCharge = "charge"
	TransactionTypeRefund = "refund"

	// Token types
	TokenTypeNetwork = "network"
	TokenTypeDual    = "dual"
	TokenTypeSingle  = "single"

	// Processors
	ProcessorA = "processor_a"
	ProcessorB = "processor_b"

	// Platforms
	PlatformWeb     = "web"
	PlatformIOS     = "ios"
	PlatformAndroid = "android"
)
