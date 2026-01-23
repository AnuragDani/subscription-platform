package main

import (
	"time"
)

// DeclineType represents the type of payment decline
type DeclineType string

const (
	DeclineTypeSoft DeclineType = "soft" // Temporary issue, retry
	DeclineTypeHard DeclineType = "hard" // Permanent issue, don't retry
)

// RetryPolicy defines the retry behavior for failed payments
type RetryPolicy struct {
	MaxAttempts    int             `json:"max_attempts"`
	RetryIntervals []time.Duration `json:"retry_intervals"`
}

// DefaultRetryPolicy returns the default retry policy
// 3 attempts at: 1 hour, 24 hours, 72 hours
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: 3,
		RetryIntervals: []time.Duration{
			1 * time.Hour,   // First retry: 1 hour
			24 * time.Hour,  // Second retry: 24 hours
			72 * time.Hour,  // Third retry: 72 hours
		},
	}
}

// GetRetryInterval returns the interval for a given attempt number
func (p *RetryPolicy) GetRetryInterval(attempt int) time.Duration {
	if attempt <= 0 {
		return p.RetryIntervals[0]
	}
	if attempt > len(p.RetryIntervals) {
		return p.RetryIntervals[len(p.RetryIntervals)-1]
	}
	return p.RetryIntervals[attempt-1]
}

// ShouldRetry determines if a payment should be retried based on error code
func (p *RetryPolicy) ShouldRetry(errorCode string, attempt int) (bool, DeclineType) {
	// Check if we've exceeded max attempts
	if attempt >= p.MaxAttempts {
		return false, DeclineTypeHard
	}

	// Classify the error
	declineType := ClassifyError(errorCode)

	// Only retry soft declines
	return declineType == DeclineTypeSoft, declineType
}

// CalculateNextRetry calculates the next retry time
func (p *RetryPolicy) CalculateNextRetry(attempt int) time.Time {
	interval := p.GetRetryInterval(attempt)
	return time.Now().Add(interval)
}

// ClassifyError determines if an error is a soft or hard decline
func ClassifyError(errorCode string) DeclineType {
	// Hard declines - don't retry
	hardDeclines := map[string]bool{
		"card_declined":           true, // Generic decline
		"invalid_card":            true,
		"expired_card":            true,
		"card_not_supported":      true,
		"invalid_account":         true,
		"currency_not_supported":  true,
		"fraud_detected":          true,
		"stolen_card":             true,
		"lost_card":               true,
		"pickup_card":             true,
		"invalid_amount":          true,
		"do_not_honor":            true,
		"account_closed":          true,
		"insufficient_permission": true,
	}

	if hardDeclines[errorCode] {
		return DeclineTypeHard
	}

	// Soft declines - retry
	softDeclines := map[string]bool{
		"insufficient_funds":      true,
		"processing_error":        true,
		"try_again_later":         true,
		"temporary_failure":       true,
		"network_error":           true,
		"timeout":                 true,
		"rate_limit_exceeded":     true,
		"service_unavailable":     true,
		"processor_unavailable":   true,
		"CHARGE_ERROR":            true, // Our internal error
		"ORCHESTRATOR_ERROR":      true, // Our internal error
	}

	if softDeclines[errorCode] {
		return DeclineTypeSoft
	}

	// Unknown errors default to soft decline (will retry)
	return DeclineTypeSoft
}

// RetryStats holds retry queue statistics
type RetryStats struct {
	TotalPending    int     `json:"total_pending"`
	TotalProcessing int     `json:"total_processing"`
	TotalSucceeded  int     `json:"total_succeeded"`
	TotalFailed     int     `json:"total_failed"`
	TotalCanceled   int     `json:"total_canceled"`
	TotalExhausted  int     `json:"total_exhausted"`
	SuccessRate     float64 `json:"success_rate"`
	AvgAttempts     float64 `json:"avg_attempts"`
}
