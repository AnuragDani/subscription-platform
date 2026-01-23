package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection
func NewDB(connectionString string) (*DB, error) {
	conn, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Ping checks if the database is accessible
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// GetSubscriptionsDue retrieves subscriptions due for billing
func (db *DB) GetSubscriptionsDue(ctx context.Context, limit int) ([]Subscription, error) {
	query := `
		SELECT id, user_id, plan_id, COALESCE(payment_method_id::text, ''), status, amount, currency,
			   billing_cycle, current_period_start, current_period_end,
			   next_billing_date, cancel_at_period_end, canceled_at,
			   trial_start, trial_end, created_at, updated_at
		FROM subscriptions
		WHERE status = 'active'
		  AND next_billing_date <= NOW()
		  AND cancel_at_period_end = false
		ORDER BY next_billing_date ASC
		LIMIT $1`

	rows, err := db.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get due subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []Subscription
	for rows.Next() {
		var s Subscription
		var paymentMethodID string
		var amount float64

		err := rows.Scan(
			&s.ID, &s.UserID, &s.PlanID, &paymentMethodID, &s.Status, &amount, &s.Currency,
			&s.BillingCycle, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
			&s.NextBillingDate, &s.CancelAtPeriodEnd, &s.CanceledAt,
			&s.TrialStart, &s.TrialEnd, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}

		if paymentMethodID != "" {
			s.PaymentMethodID = paymentMethodID
		}

		s.Amount = int64(amount * 100)
		subscriptions = append(subscriptions, s)
	}

	return subscriptions, rows.Err()
}

// CreateJob creates a new scheduler job
func (db *DB) CreateJob(ctx context.Context, subscriptionID, jobType string, scheduledAt time.Time) (*Job, error) {
	job := &Job{
		ID:             uuid.New().String(),
		SubscriptionID: subscriptionID,
		Type:           jobType,
		Status:         JobStatusPending,
		Attempt:        1,
		ScheduledAt:    scheduledAt,
		CreatedAt:      time.Now(),
	}

	query := `
		INSERT INTO scheduler_jobs (id, subscription_id, type, status, attempt, scheduled_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := db.conn.ExecContext(ctx, query,
		job.ID, job.SubscriptionID, job.Type, job.Status, job.Attempt, job.ScheduledAt, job.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// UpdateJobStatus updates the status of a job
func (db *DB) UpdateJobStatus(ctx context.Context, jobID string, status string, result *ChargeResult) error {
	now := time.Now()

	var query string
	var args []interface{}

	if result != nil {
		query = `
			UPDATE scheduler_jobs
			SET status = $1, completed_at = $2, transaction_id = $3, processor_used = $4, error_code = $5, error_message = $6
			WHERE id = $7`
		args = []interface{}{status, now, result.TransactionID, result.ProcessorUsed, result.ErrorCode, result.ErrorMessage, jobID}
	} else {
		query = `
			UPDATE scheduler_jobs
			SET status = $1, started_at = $2
			WHERE id = $3`
		args = []interface{}{status, now, jobID}
	}

	_, err := db.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

// GetJob retrieves a job by ID
func (db *DB) GetJob(ctx context.Context, id string) (*Job, error) {
	query := `
		SELECT id, subscription_id, type, status, attempt,
			   COALESCE(transaction_id, ''), COALESCE(processor_used, ''),
			   COALESCE(error_code, ''), COALESCE(error_message, ''),
			   scheduled_at, started_at, completed_at, created_at
		FROM scheduler_jobs WHERE id = $1`

	var job Job
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.SubscriptionID, &job.Type, &job.Status, &job.Attempt,
		&job.TransactionID, &job.ProcessorUsed, &job.ErrorCode, &job.ErrorMessage,
		&job.ScheduledAt, &job.StartedAt, &job.CompletedAt, &job.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return &job, nil
}

// ListJobs retrieves recent jobs
func (db *DB) ListJobs(ctx context.Context, limit int) ([]Job, error) {
	query := `
		SELECT id, subscription_id, type, status, attempt,
			   COALESCE(transaction_id, ''), COALESCE(processor_used, ''),
			   COALESCE(error_code, ''), COALESCE(error_message, ''),
			   scheduled_at, started_at, completed_at, created_at
		FROM scheduler_jobs
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := db.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		err := rows.Scan(
			&job.ID, &job.SubscriptionID, &job.Type, &job.Status, &job.Attempt,
			&job.TransactionID, &job.ProcessorUsed, &job.ErrorCode, &job.ErrorMessage,
			&job.ScheduledAt, &job.StartedAt, &job.CompletedAt, &job.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// GetJobCount returns the total number of jobs
func (db *DB) GetJobCount(ctx context.Context) (int, error) {
	var count int
	err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_jobs").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}
	return count, nil
}

// EnsureJobsTableExists creates the scheduler_jobs table if it doesn't exist
func (db *DB) EnsureJobsTableExists(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS scheduler_jobs (
			id UUID PRIMARY KEY,
			subscription_id UUID NOT NULL REFERENCES subscriptions(id),
			type VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			attempt INT NOT NULL DEFAULT 1,
			transaction_id VARCHAR(255),
			processor_used VARCHAR(100),
			error_code VARCHAR(100),
			error_message TEXT,
			scheduled_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			CONSTRAINT valid_job_status CHECK (status IN ('pending', 'running', 'completed', 'failed')),
			CONSTRAINT valid_job_type CHECK (type IN ('billing', 'retry'))
		);

		CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_status ON scheduler_jobs(status);
		CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_subscription ON scheduler_jobs(subscription_id);
		CREATE INDEX IF NOT EXISTS idx_scheduler_jobs_scheduled_at ON scheduler_jobs(scheduled_at);
	`

	_, err := db.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create scheduler_jobs table: %w", err)
	}

	return nil
}
