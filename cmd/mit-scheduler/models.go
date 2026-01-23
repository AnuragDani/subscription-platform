package main

import (
	"time"
)

// Subscription represents a subscription from the subscription service
type Subscription struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	PlanID            string     `json:"plan_id"`
	PaymentMethodID   string     `json:"payment_method_id"`
	Status            string     `json:"status"`
	Amount            int64      `json:"amount"` // in cents
	Currency          string     `json:"currency"`
	BillingCycle      string     `json:"billing_cycle"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	NextBillingDate    *time.Time `json:"next_billing_date,omitempty"`
	CancelAtPeriodEnd  bool       `json:"cancel_at_period_end"`
	CanceledAt         *time.Time `json:"canceled_at,omitempty"`
	TrialStart         *time.Time `json:"trial_start,omitempty"`
	TrialEnd           *time.Time `json:"trial_end,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Subscription status constants
const (
	SubscriptionStatusActive   = "active"
	SubscriptionStatusPastDue  = "past_due"
	SubscriptionStatusCanceled = "canceled"
	SubscriptionStatusTrialing = "trialing"
)

// Billing interval constants
const (
	IntervalMonthly = "monthly"
	IntervalYearly  = "yearly"
)

// Job represents a scheduler job
type Job struct {
	ID              string     `json:"id"`
	SubscriptionID  string     `json:"subscription_id"`
	Type            string     `json:"type"` // billing, retry
	Status          string     `json:"status"`
	Attempt         int        `json:"attempt"`
	TransactionID   string     `json:"transaction_id,omitempty"`
	ProcessorUsed   string     `json:"processor_used,omitempty"`
	ErrorCode       string     `json:"error_code,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	ScheduledAt     time.Time  `json:"scheduled_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Job status constants
const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
)

// Job type constants
const (
	JobTypeBilling = "billing"
	JobTypeRetry   = "retry"
)

// SchedulerStatus represents the current state of the scheduler
type SchedulerStatus struct {
	Running       bool       `json:"running"`
	LastRun       *time.Time `json:"last_run,omitempty"`
	NextRun       *time.Time `json:"next_run,omitempty"`
	ProcessedLast int        `json:"processed_last"`
	TotalJobs     int        `json:"total_jobs"`
	TickInterval  string     `json:"tick_interval"`
}

// SchedulerConfig holds scheduler configuration
type SchedulerConfig struct {
	TickInterval  time.Duration
	BatchSize     int
	Enabled       bool
}

// DefaultSchedulerConfig returns the default scheduler configuration
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		TickInterval: 60 * time.Second,
		BatchSize:    100,
		Enabled:      true,
	}
}

// ChargeResult represents the result of a charge attempt
type ChargeResult struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transaction_id,omitempty"`
	ProcessorUsed string `json:"processor_used,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}
