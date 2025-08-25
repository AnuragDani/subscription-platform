package models

import (
	"time"

	"github.com/google/uuid"
)

// Subscription represents a user's subscription
type Subscription struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	UserID          uuid.UUID  `json:"user_id" db:"user_id"`
	Status          string     `json:"status" db:"status"`
	PlanID          string     `json:"plan_id" db:"plan_id"`
	Amount          float64    `json:"amount" db:"amount"`
	Currency        string     `json:"currency" db:"currency"`
	BillingCycle    string     `json:"billing_cycle" db:"billing_cycle"`
	NextBillingDate *time.Time `json:"next_billing_date" db:"next_billing_date"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// PaymentMethod represents tokenized payment information
type PaymentMethod struct {
	ID              uuid.UUID `json:"id" db:"id"`
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	NetworkToken    *string   `json:"network_token,omitempty" db:"network_token"`
	ProcessorAToken *string   `json:"processor_a_token,omitempty" db:"processor_a_token"`
	ProcessorBToken *string   `json:"processor_b_token,omitempty" db:"processor_b_token"`
	TokenType       string    `json:"token_type" db:"token_type"` // "network" or "dual_vault"
	LastFour        string    `json:"last_four" db:"last_four"`
	ExpMonth        int       `json:"exp_month" db:"exp_month"`
	ExpYear         int       `json:"exp_year" db:"exp_year"`
	IsDefault       bool      `json:"is_default" db:"is_default"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// Transaction represents a payment attempt or refund
type Transaction struct {
	ID                     uuid.UUID  `json:"id" db:"id"`
	SubscriptionID         *uuid.UUID `json:"subscription_id" db:"subscription_id"`
	PaymentMethodID        *uuid.UUID `json:"payment_method_id" db:"payment_method_id"`
	ProcessorUsed          string     `json:"processor_used" db:"processor_used"`
	Amount                 float64    `json:"amount" db:"amount"`
	Currency               string     `json:"currency" db:"currency"`
	Status                 string     `json:"status" db:"status"`                     // pending, success, failed
	TransactionType        string     `json:"transaction_type" db:"transaction_type"` // charge, refund
	IdempotencyKey         string     `json:"idempotency_key" db:"idempotency_key"`
	ProcessorTransactionID *string    `json:"processor_transaction_id" db:"processor_transaction_id"`
	ErrorCode              *string    `json:"error_code" db:"error_code"`
	ErrorMessage           *string    `json:"error_message" db:"error_message"`
	OriginalTransactionID  *uuid.UUID `json:"original_transaction_id" db:"original_transaction_id"`
	CreatedAt              time.Time  `json:"created_at" db:"created_at"`
}

// RoutingRule represents BPAS routing configuration
type RoutingRule struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	RuleName        string                 `json:"rule_name" db:"rule_name"`
	Priority        int                    `json:"priority" db:"priority"`
	ConditionType   string                 `json:"condition_type" db:"condition_type"`
	ConditionValue  map[string]interface{} `json:"condition_value" db:"condition_value"`
	TargetProcessor string                 `json:"target_processor" db:"target_processor"`
	Percentage      int                    `json:"percentage" db:"percentage"`
	IsActive        bool                   `json:"is_active" db:"is_active"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}

// ProcessorHealth tracks processor status
type ProcessorHealth struct {
	ID                uuid.UUID `json:"id" db:"id"`
	ProcessorName     string    `json:"processor_name" db:"processor_name"`
	IsHealthy         bool      `json:"is_healthy" db:"is_healthy"`
	LastCheck         time.Time `json:"last_check" db:"last_check"`
	FailureCount      int       `json:"failure_count" db:"failure_count"`
	SuccessRate       float64   `json:"success_rate" db:"success_rate"`
	AvgResponseTimeMs int       `json:"avg_response_time_ms" db:"avg_response_time_ms"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// API Request/Response models

// CreateSubscriptionRequest for new subscription creation
type CreateSubscriptionRequest struct {
	UserID        uuid.UUID          `json:"user_id"`
	PlanID        string             `json:"plan_id"`
	PaymentMethod PaymentMethodInput `json:"payment_method"`
}

// PaymentMethodInput for creating payment methods
type PaymentMethodInput struct {
	CardNumber string `json:"card_number"`
	ExpMonth   int    `json:"exp_month"`
	ExpYear    int    `json:"exp_year"`
	CVV        string `json:"cvv,omitempty"`
}

// ChargeRequest for payment processing
type ChargeRequest struct {
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Token          string  `json:"token"`
	IdempotencyKey string  `json:"idempotency_key"`
}

// ChargeResponse from payment processors
type ChargeResponse struct {
	Success       bool    `json:"success"`
	TransactionID string  `json:"transaction_id"`
	AuthCode      *string `json:"auth_code,omitempty"`
	ErrorCode     *string `json:"error_code,omitempty"`
	ErrorMessage  *string `json:"error_message,omitempty"`
}

// RefundRequest for refund processing
type RefundRequest struct {
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Reason        string  `json:"reason"`
}

// NetworkTokenRequest for token creation
type NetworkTokenRequest struct {
	CardNumber string `json:"card_number"`
	ExpMonth   string `json:"exp_month"`
	ExpYear    string `json:"exp_year"`
}

// NetworkTokenResponse from network token service
type NetworkTokenResponse struct {
	Success          bool    `json:"success"`
	NetworkToken     *string `json:"network_token,omitempty"`
	TokenType        string  `json:"token_type"`
	Portable         bool    `json:"portable"`
	Error            *string `json:"error,omitempty"`
	FallbackRequired bool    `json:"fallback_required,omitempty"`
}

// PaymentResult from orchestrator
type PaymentResult struct {
	Success       bool    `json:"success"`
	TransactionID string  `json:"transaction_id"`
	ProcessorUsed string  `json:"processor_used"`
	ErrorMessage  *string `json:"error_message,omitempty"`
}

// HealthResponse for health checks
type HealthResponse struct {
	Service   string                 `json:"service"`
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}
