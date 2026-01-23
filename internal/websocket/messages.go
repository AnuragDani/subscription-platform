package websocket

import (
	"encoding/json"
	"time"
)

// Message types for WebSocket events
const (
	TypeTransaction  = "transaction"
	TypeSubscription = "subscription"
	TypeScheduler    = "scheduler"
	TypeHealth       = "health"
	TypeRouting      = "routing"
	TypeHeartbeat    = "heartbeat"
)

// Transaction events
const (
	EventChargeInitiated = "charge_initiated"
	EventChargeSucceeded = "charge_succeeded"
	EventChargeFailed    = "charge_failed"
	EventFailoverTriggered = "failover_triggered"
	EventRefundProcessed = "refund_processed"
)

// Subscription events
const (
	EventSubscriptionCreated    = "created"
	EventSubscriptionUpgraded   = "upgraded"
	EventSubscriptionDowngraded = "downgraded"
	EventSubscriptionCanceled   = "canceled"
	EventSubscriptionPastDue    = "past_due"
)

// Scheduler events
const (
	EventJobStarted      = "job_started"
	EventJobCompleted    = "job_completed"
	EventRetryScheduled  = "retry_scheduled"
	EventRetryFailed     = "retry_failed"
	EventRetrySucceeded  = "retry_succeeded"
)

// Health events
const (
	EventProcessorHealthy   = "processor_healthy"
	EventProcessorUnhealthy = "processor_unhealthy"
)

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Event     string      `json:"event"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewMessage creates a new message with the current timestamp
func NewMessage(msgType, event string, data interface{}) *Message {
	return &Message{
		Type:      msgType,
		Event:     event,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}

// ToJSON serializes the message to JSON bytes
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// TransactionData represents transaction event data
type TransactionData struct {
	TransactionID   string  `json:"transaction_id"`
	SubscriptionID  string  `json:"subscription_id,omitempty"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	ProcessorUsed   string  `json:"processor_used,omitempty"`
	PreviousProcessor string `json:"previous_processor,omitempty"`
	Status          string  `json:"status"`
	ErrorCode       string  `json:"error_code,omitempty"`
	ErrorMessage    string  `json:"error_message,omitempty"`
	Duration        string  `json:"duration,omitempty"`
}

// SubscriptionData represents subscription event data
type SubscriptionData struct {
	SubscriptionID string  `json:"subscription_id"`
	UserID         string  `json:"user_id"`
	PlanID         string  `json:"plan_id"`
	PlanName       string  `json:"plan_name,omitempty"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	PreviousPlanID string  `json:"previous_plan_id,omitempty"`
}

// SchedulerData represents scheduler event data
type SchedulerData struct {
	JobID          string `json:"job_id"`
	SubscriptionID string `json:"subscription_id"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Attempt        int    `json:"attempt,omitempty"`
	MaxAttempts    int    `json:"max_attempts,omitempty"`
	NextRetryAt    string `json:"next_retry_at,omitempty"`
	TransactionID  string `json:"transaction_id,omitempty"`
	ProcessorUsed  string `json:"processor_used,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// HealthData represents health event data
type HealthData struct {
	Processor   string `json:"processor"`
	Status      string `json:"status"`
	SuccessRate float64 `json:"success_rate,omitempty"`
	Latency     string `json:"latency,omitempty"`
}

// HeartbeatData represents heartbeat data
type HeartbeatData struct {
	ServerTime    time.Time `json:"server_time"`
	ConnectedAt   time.Time `json:"connected_at"`
	ClientCount   int       `json:"client_count"`
}
