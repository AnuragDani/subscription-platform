package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RetryEntry represents an entry in the retry queue
type RetryEntry struct {
	ID               string     `json:"id"`
	SubscriptionID   string     `json:"subscription_id"`
	Attempt          int        `json:"attempt"`
	MaxAttempts      int        `json:"max_attempts"`
	Status           string     `json:"status"`
	LastErrorCode    string     `json:"last_error_code,omitempty"`
	LastErrorMessage string     `json:"last_error_message,omitempty"`
	DeclineType      string     `json:"decline_type,omitempty"`
	NextRetryAt      time.Time  `json:"next_retry_at"`
	LastAttemptAt    *time.Time `json:"last_attempt_at,omitempty"`
	TransactionID    string     `json:"transaction_id,omitempty"`
	ProcessorUsed    string     `json:"processor_used,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
}

// Retry entry status constants
const (
	RetryStatusPending    = "pending"
	RetryStatusProcessing = "processing"
	RetryStatusSucceeded  = "succeeded"
	RetryStatusFailed     = "failed"
	RetryStatusCanceled   = "canceled"
	RetryStatusExhausted  = "exhausted"
)

// EnsureRetryTableExists creates the retry_queue table if it doesn't exist
func (db *DB) EnsureRetryTableExists(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS retry_queue (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			subscription_id UUID NOT NULL REFERENCES subscriptions(id),
			attempt INT NOT NULL DEFAULT 1,
			max_attempts INT NOT NULL DEFAULT 3,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			last_error_code VARCHAR(100),
			last_error_message TEXT,
			decline_type VARCHAR(50),
			next_retry_at TIMESTAMP NOT NULL,
			last_attempt_at TIMESTAMP,
			transaction_id VARCHAR(255),
			processor_used VARCHAR(100),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			resolved_at TIMESTAMP,
			CONSTRAINT valid_retry_status CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'canceled', 'exhausted'))
		);

		CREATE INDEX IF NOT EXISTS idx_retry_queue_status ON retry_queue(status);
		CREATE INDEX IF NOT EXISTS idx_retry_queue_next_retry ON retry_queue(next_retry_at) WHERE status = 'pending';
		CREATE INDEX IF NOT EXISTS idx_retry_queue_subscription ON retry_queue(subscription_id);
	`

	_, err := db.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create retry_queue table: %w", err)
	}

	return nil
}

// CreateRetryEntry creates a new retry queue entry
func (db *DB) CreateRetryEntry(ctx context.Context, subscriptionID, errorCode, errorMessage string, declineType DeclineType, policy *RetryPolicy) (*RetryEntry, error) {
	entry := &RetryEntry{
		ID:               uuid.New().String(),
		SubscriptionID:   subscriptionID,
		Attempt:          1,
		MaxAttempts:      policy.MaxAttempts,
		Status:           RetryStatusPending,
		LastErrorCode:    errorCode,
		LastErrorMessage: errorMessage,
		DeclineType:      string(declineType),
		NextRetryAt:      policy.CalculateNextRetry(1),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	query := `
		INSERT INTO retry_queue (
			id, subscription_id, attempt, max_attempts, status,
			last_error_code, last_error_message, decline_type,
			next_retry_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (subscription_id) WHERE status = 'pending'
		DO UPDATE SET
			attempt = retry_queue.attempt,
			updated_at = NOW()
		RETURNING id`

	err := db.conn.QueryRowContext(ctx, query,
		entry.ID, entry.SubscriptionID, entry.Attempt, entry.MaxAttempts, entry.Status,
		entry.LastErrorCode, entry.LastErrorMessage, entry.DeclineType,
		entry.NextRetryAt, entry.CreatedAt, entry.UpdatedAt,
	).Scan(&entry.ID)

	if err != nil {
		// If conflict, get existing entry
		existing, getErr := db.GetActiveRetryForSubscription(ctx, subscriptionID)
		if getErr == nil && existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("failed to create retry entry: %w", err)
	}

	return entry, nil
}

// GetRetryEntry retrieves a retry entry by ID
func (db *DB) GetRetryEntry(ctx context.Context, id string) (*RetryEntry, error) {
	query := `
		SELECT id, subscription_id, attempt, max_attempts, status,
			   COALESCE(last_error_code, ''), COALESCE(last_error_message, ''),
			   COALESCE(decline_type, ''), next_retry_at, last_attempt_at,
			   COALESCE(transaction_id, ''), COALESCE(processor_used, ''),
			   created_at, updated_at, resolved_at
		FROM retry_queue WHERE id = $1`

	var entry RetryEntry
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&entry.ID, &entry.SubscriptionID, &entry.Attempt, &entry.MaxAttempts, &entry.Status,
		&entry.LastErrorCode, &entry.LastErrorMessage, &entry.DeclineType,
		&entry.NextRetryAt, &entry.LastAttemptAt,
		&entry.TransactionID, &entry.ProcessorUsed,
		&entry.CreatedAt, &entry.UpdatedAt, &entry.ResolvedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("retry entry not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get retry entry: %w", err)
	}

	return &entry, nil
}

// GetActiveRetryForSubscription gets the active retry entry for a subscription
func (db *DB) GetActiveRetryForSubscription(ctx context.Context, subscriptionID string) (*RetryEntry, error) {
	query := `
		SELECT id, subscription_id, attempt, max_attempts, status,
			   COALESCE(last_error_code, ''), COALESCE(last_error_message, ''),
			   COALESCE(decline_type, ''), next_retry_at, last_attempt_at,
			   COALESCE(transaction_id, ''), COALESCE(processor_used, ''),
			   created_at, updated_at, resolved_at
		FROM retry_queue
		WHERE subscription_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
		LIMIT 1`

	var entry RetryEntry
	err := db.conn.QueryRowContext(ctx, query, subscriptionID).Scan(
		&entry.ID, &entry.SubscriptionID, &entry.Attempt, &entry.MaxAttempts, &entry.Status,
		&entry.LastErrorCode, &entry.LastErrorMessage, &entry.DeclineType,
		&entry.NextRetryAt, &entry.LastAttemptAt,
		&entry.TransactionID, &entry.ProcessorUsed,
		&entry.CreatedAt, &entry.UpdatedAt, &entry.ResolvedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active retry: %w", err)
	}

	return &entry, nil
}

// GetDueRetries retrieves retry entries that are due for processing
func (db *DB) GetDueRetries(ctx context.Context, limit int) ([]RetryEntry, error) {
	query := `
		SELECT id, subscription_id, attempt, max_attempts, status,
			   COALESCE(last_error_code, ''), COALESCE(last_error_message, ''),
			   COALESCE(decline_type, ''), next_retry_at, last_attempt_at,
			   COALESCE(transaction_id, ''), COALESCE(processor_used, ''),
			   created_at, updated_at, resolved_at
		FROM retry_queue
		WHERE status = 'pending' AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1`

	rows, err := db.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get due retries: %w", err)
	}
	defer rows.Close()

	var entries []RetryEntry
	for rows.Next() {
		var entry RetryEntry
		err := rows.Scan(
			&entry.ID, &entry.SubscriptionID, &entry.Attempt, &entry.MaxAttempts, &entry.Status,
			&entry.LastErrorCode, &entry.LastErrorMessage, &entry.DeclineType,
			&entry.NextRetryAt, &entry.LastAttemptAt,
			&entry.TransactionID, &entry.ProcessorUsed,
			&entry.CreatedAt, &entry.UpdatedAt, &entry.ResolvedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan retry entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// ListRetries retrieves all pending retry entries
func (db *DB) ListRetries(ctx context.Context, status string, limit int) ([]RetryEntry, error) {
	query := `
		SELECT id, subscription_id, attempt, max_attempts, status,
			   COALESCE(last_error_code, ''), COALESCE(last_error_message, ''),
			   COALESCE(decline_type, ''), next_retry_at, last_attempt_at,
			   COALESCE(transaction_id, ''), COALESCE(processor_used, ''),
			   created_at, updated_at, resolved_at
		FROM retry_queue`

	args := []any{}
	argNum := 1

	if status != "" {
		query += fmt.Sprintf(" WHERE status = $%d", argNum)
		args = append(args, status)
		argNum++
	}

	query += " ORDER BY next_retry_at ASC"
	query += fmt.Sprintf(" LIMIT $%d", argNum)
	args = append(args, limit)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list retries: %w", err)
	}
	defer rows.Close()

	var entries []RetryEntry
	for rows.Next() {
		var entry RetryEntry
		err := rows.Scan(
			&entry.ID, &entry.SubscriptionID, &entry.Attempt, &entry.MaxAttempts, &entry.Status,
			&entry.LastErrorCode, &entry.LastErrorMessage, &entry.DeclineType,
			&entry.NextRetryAt, &entry.LastAttemptAt,
			&entry.TransactionID, &entry.ProcessorUsed,
			&entry.CreatedAt, &entry.UpdatedAt, &entry.ResolvedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan retry entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// UpdateRetryStatus updates a retry entry status
func (db *DB) UpdateRetryStatus(ctx context.Context, id string, status string) error {
	now := time.Now()

	var query string
	var args []any

	if status == RetryStatusSucceeded || status == RetryStatusFailed ||
		status == RetryStatusCanceled || status == RetryStatusExhausted {
		query = `UPDATE retry_queue SET status = $1, updated_at = $2, resolved_at = $3 WHERE id = $4`
		args = []any{status, now, now, id}
	} else {
		query = `UPDATE retry_queue SET status = $1, updated_at = $2 WHERE id = $3`
		args = []any{status, now, id}
	}

	_, err := db.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update retry status: %w", err)
	}

	return nil
}

// UpdateRetryAfterAttempt updates a retry entry after an attempt
func (db *DB) UpdateRetryAfterAttempt(ctx context.Context, id string, success bool, result *ChargeResult, policy *RetryPolicy) error {
	now := time.Now()

	entry, err := db.GetRetryEntry(ctx, id)
	if err != nil {
		return err
	}

	if success {
		query := `
			UPDATE retry_queue SET
				status = $1, last_attempt_at = $2, transaction_id = $3,
				processor_used = $4, updated_at = $5, resolved_at = $6
			WHERE id = $7`
		_, err = db.conn.ExecContext(ctx, query,
			RetryStatusSucceeded, now, result.TransactionID,
			result.ProcessorUsed, now, now, id,
		)
	} else {
		newAttempt := entry.Attempt + 1

		if newAttempt > entry.MaxAttempts {
			// Exhausted all retries
			query := `
				UPDATE retry_queue SET
					status = $1, attempt = $2, last_attempt_at = $3,
					last_error_code = $4, last_error_message = $5,
					updated_at = $6, resolved_at = $7
				WHERE id = $8`
			_, err = db.conn.ExecContext(ctx, query,
				RetryStatusExhausted, newAttempt, now,
				result.ErrorCode, result.ErrorMessage,
				now, now, id,
			)
		} else {
			// Schedule next retry
			nextRetry := policy.CalculateNextRetry(newAttempt)
			declineType := ClassifyError(result.ErrorCode)

			if declineType == DeclineTypeHard {
				// Hard decline - don't retry
				query := `
					UPDATE retry_queue SET
						status = $1, attempt = $2, last_attempt_at = $3,
						last_error_code = $4, last_error_message = $5,
						decline_type = $6, updated_at = $7, resolved_at = $8
					WHERE id = $9`
				_, err = db.conn.ExecContext(ctx, query,
					RetryStatusFailed, newAttempt, now,
					result.ErrorCode, result.ErrorMessage,
					string(declineType), now, now, id,
				)
			} else {
				// Soft decline - schedule retry
				query := `
					UPDATE retry_queue SET
						status = $1, attempt = $2, last_attempt_at = $3,
						last_error_code = $4, last_error_message = $5,
						decline_type = $6, next_retry_at = $7, updated_at = $8
					WHERE id = $9`
				_, err = db.conn.ExecContext(ctx, query,
					RetryStatusPending, newAttempt, now,
					result.ErrorCode, result.ErrorMessage,
					string(declineType), nextRetry, now, id,
				)
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to update retry after attempt: %w", err)
	}

	return nil
}

// CancelRetry cancels a pending retry
func (db *DB) CancelRetry(ctx context.Context, id string) error {
	return db.UpdateRetryStatus(ctx, id, RetryStatusCanceled)
}

// GetRetryStats returns retry queue statistics
func (db *DB) GetRetryStats(ctx context.Context) (*RetryStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'processing') as processing,
			COUNT(*) FILTER (WHERE status = 'succeeded') as succeeded,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			COUNT(*) FILTER (WHERE status = 'canceled') as canceled,
			COUNT(*) FILTER (WHERE status = 'exhausted') as exhausted,
			COALESCE(AVG(attempt) FILTER (WHERE status IN ('succeeded', 'failed', 'exhausted')), 0) as avg_attempts
		FROM retry_queue`

	var stats RetryStats
	err := db.conn.QueryRowContext(ctx, query).Scan(
		&stats.TotalPending, &stats.TotalProcessing, &stats.TotalSucceeded,
		&stats.TotalFailed, &stats.TotalCanceled, &stats.TotalExhausted,
		&stats.AvgAttempts,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get retry stats: %w", err)
	}

	// Calculate success rate
	total := stats.TotalSucceeded + stats.TotalFailed + stats.TotalExhausted
	if total > 0 {
		stats.SuccessRate = float64(stats.TotalSucceeded) / float64(total) * 100
	}

	return &stats, nil
}
