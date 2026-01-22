// cmd/subscription-service/models.go
package main

import (
	"encoding/json"
	"time"
)

// Plan represents a subscription plan
type Plan struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	DisplayName string          `json:"display_name" db:"display_name"`
	Amount      int64           `json:"amount" db:"amount"` // Amount in cents
	Currency    string          `json:"currency" db:"currency"`
	Interval    string          `json:"interval" db:"interval"` // monthly, yearly
	TrialDays   int             `json:"trial_days" db:"trial_days"`
	Features    json.RawMessage `json:"features" db:"features"`
	IsActive    bool            `json:"is_active" db:"is_active"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// Subscription represents a user's subscription (extended from base model)
type Subscription struct {
	ID                  string     `json:"id" db:"id"`
	UserID              string     `json:"user_id" db:"user_id"`
	PlanID              string     `json:"plan_id" db:"plan_id"`
	PaymentMethodID     string     `json:"payment_method_id" db:"payment_method_id"`
	Status              string     `json:"status" db:"status"`
	Amount              int64      `json:"amount" db:"amount"` // Amount in cents
	Currency            string     `json:"currency" db:"currency"`
	BillingCycle        string     `json:"billing_cycle" db:"billing_cycle"`
	CurrentPeriodStart  *time.Time `json:"current_period_start,omitempty" db:"current_period_start"`
	CurrentPeriodEnd    *time.Time `json:"current_period_end,omitempty" db:"current_period_end"`
	NextBillingDate     *time.Time `json:"next_billing_date,omitempty" db:"next_billing_date"`
	CancelAtPeriodEnd   bool       `json:"cancel_at_period_end" db:"cancel_at_period_end"`
	CanceledAt          *time.Time `json:"canceled_at,omitempty" db:"canceled_at"`
	TrialStart          *time.Time `json:"trial_start,omitempty" db:"trial_start"`
	TrialEnd            *time.Time `json:"trial_end,omitempty" db:"trial_end"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
}

// SubscriptionWithPlan includes plan details
type SubscriptionWithPlan struct {
	Subscription
	Plan *Plan `json:"plan,omitempty"`
}

// Subscription status constants
const (
	SubscriptionStatusActive   = "active"
	SubscriptionStatusPastDue  = "past_due"
	SubscriptionStatusCanceled = "canceled"
	SubscriptionStatusTrialing = "trialing"
	SubscriptionStatusPaused   = "paused"
)

// Billing interval constants
const (
	IntervalMonthly = "monthly"
	IntervalYearly  = "yearly"
)

// Plan IDs for default plans
const (
	PlanBasicMonthly      = "basic_monthly"
	PlanBasicYearly       = "basic_yearly"
	PlanProMonthly        = "pro_monthly"
	PlanProYearly         = "pro_yearly"
	PlanEnterpriseMonthly = "enterprise_monthly"
	PlanEnterpriseYearly  = "enterprise_yearly"
)

// Request/Response structs

// CreateSubscriptionRequest represents a request to create a subscription
type CreateSubscriptionRequest struct {
	UserID          string `json:"user_id"`
	PlanID          string `json:"plan_id"`
	PaymentMethodID string `json:"payment_method_id"`
}

// UpdateSubscriptionRequest represents a request to update a subscription
type UpdateSubscriptionRequest struct {
	PlanID string `json:"plan_id,omitempty"`
	Status string `json:"status,omitempty"`
}

// SubscriptionResponse wraps subscription with additional info
type SubscriptionResponse struct {
	*SubscriptionWithPlan
	ProrationAmount int64  `json:"proration_amount,omitempty"`
	Message         string `json:"message,omitempty"`
}

// PlanListResponse wraps plan list
type PlanListResponse struct {
	Plans []Plan `json:"plans"`
	Total int    `json:"total"`
}

// SubscriptionListResponse wraps subscription list
type SubscriptionListResponse struct {
	Subscriptions []SubscriptionWithPlan `json:"subscriptions"`
	Total         int                    `json:"total"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// Validate checks if CreateSubscriptionRequest is valid
func (r *CreateSubscriptionRequest) Validate() error {
	if r.UserID == "" {
		return ErrMissingUserID
	}
	if r.PlanID == "" {
		return ErrMissingPlanID
	}
	if r.PaymentMethodID == "" {
		return ErrMissingPaymentMethodID
	}
	return nil
}

// Custom errors
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

var (
	ErrMissingUserID          = ValidationError{Field: "user_id", Message: "user_id is required"}
	ErrMissingPlanID          = ValidationError{Field: "plan_id", Message: "plan_id is required"}
	ErrMissingPaymentMethodID = ValidationError{Field: "payment_method_id", Message: "payment_method_id is required"}
	ErrPlanNotFound           = ValidationError{Field: "plan_id", Message: "plan not found"}
	ErrSubscriptionNotFound   = ValidationError{Field: "id", Message: "subscription not found"}
	ErrInvalidStatus          = ValidationError{Field: "status", Message: "invalid subscription status"}
)
