package main

import (
	"context"
	"log"
	"time"
)

// Executor handles the execution of billing jobs
type Executor struct {
	db                 *DB
	subscriptionClient *SubscriptionServiceClient
	retryPolicy        *RetryPolicy
	logger             *log.Logger
}

// NewExecutor creates a new executor instance
func NewExecutor(db *DB, subscriptionServiceURL string, logger *log.Logger) *Executor {
	return &Executor{
		db:                 db,
		subscriptionClient: NewSubscriptionServiceClient(subscriptionServiceURL),
		retryPolicy:        DefaultRetryPolicy(),
		logger:             logger,
	}
}

// ExecuteCharge processes a billing charge for a subscription
func (e *Executor) ExecuteCharge(ctx context.Context, job *Job, sub *Subscription) *ChargeResult {
	e.logger.Printf("Executing charge for subscription %s (job=%s, attempt=%d)",
		sub.ID, job.ID, job.Attempt)

	// Call the subscription service to charge
	chargeResp, err := e.subscriptionClient.ChargeSubscription(ctx, sub.ID)
	if err != nil {
		e.logger.Printf("Error charging subscription %s: %v", sub.ID, err)
		return &ChargeResult{
			Success:      false,
			ErrorCode:    "CHARGE_ERROR",
			ErrorMessage: err.Error(),
		}
	}

	result := &ChargeResult{
		Success:       chargeResp.Success,
		TransactionID: chargeResp.TransactionID,
		ProcessorUsed: chargeResp.ProcessorUsed,
		ErrorCode:     chargeResp.ErrorCode,
		ErrorMessage:  chargeResp.ErrorMessage,
	}

	if chargeResp.Success {
		e.logger.Printf("Charge successful for subscription %s: transaction=%s, processor=%s",
			sub.ID, chargeResp.TransactionID, chargeResp.ProcessorUsed)
	} else {
		e.logger.Printf("Charge failed for subscription %s: error=%s, message=%s",
			sub.ID, chargeResp.ErrorCode, chargeResp.ErrorMessage)
	}

	return result
}

// ProcessJob processes a single job
func (e *Executor) ProcessJob(ctx context.Context, job *Job, sub *Subscription) error {
	// Mark job as running
	if err := e.db.UpdateJobStatus(ctx, job.ID, JobStatusRunning, nil); err != nil {
		e.logger.Printf("Error marking job as running: %v", err)
	}

	// Execute the charge
	result := e.ExecuteCharge(ctx, job, sub)

	// Update job status based on result
	var status string
	if result.Success {
		status = JobStatusCompleted
	} else {
		status = JobStatusFailed

		// On failure, create a retry entry
		declineType := ClassifyError(result.ErrorCode)
		if declineType == DeclineTypeSoft {
			_, err := e.db.CreateRetryEntry(ctx, sub.ID, result.ErrorCode, result.ErrorMessage, declineType, e.retryPolicy)
			if err != nil {
				e.logger.Printf("Error creating retry entry for subscription %s: %v", sub.ID, err)
			} else {
				e.logger.Printf("Created retry entry for subscription %s (next retry in %v)",
					sub.ID, e.retryPolicy.GetRetryInterval(1))
			}
		} else {
			e.logger.Printf("Hard decline for subscription %s, not scheduling retry: %s",
				sub.ID, result.ErrorCode)
		}
	}

	if err := e.db.UpdateJobStatus(ctx, job.ID, status, result); err != nil {
		e.logger.Printf("Error updating job status: %v", err)
		return err
	}

	return nil
}

// ProcessRetry processes a single retry entry
func (e *Executor) ProcessRetry(ctx context.Context, entry *RetryEntry) (*ChargeResult, error) {
	e.logger.Printf("Processing retry %s for subscription %s (attempt %d/%d)",
		entry.ID, entry.SubscriptionID, entry.Attempt, entry.MaxAttempts)

	// Mark as processing
	if err := e.db.UpdateRetryStatus(ctx, entry.ID, RetryStatusProcessing); err != nil {
		e.logger.Printf("Error marking retry as processing: %v", err)
	}

	// Get subscription details (we need this for the charge)
	sub, err := e.subscriptionClient.GetSubscription(ctx, entry.SubscriptionID)
	if err != nil {
		e.logger.Printf("Error getting subscription %s: %v", entry.SubscriptionID, err)
		result := &ChargeResult{
			Success:      false,
			ErrorCode:    "SUBSCRIPTION_ERROR",
			ErrorMessage: err.Error(),
		}
		e.db.UpdateRetryAfterAttempt(ctx, entry.ID, false, result, e.retryPolicy)
		return result, err
	}

	// Execute the charge
	chargeResp, err := e.subscriptionClient.ChargeSubscription(ctx, entry.SubscriptionID)
	if err != nil {
		e.logger.Printf("Error charging subscription %s: %v", entry.SubscriptionID, err)
		result := &ChargeResult{
			Success:      false,
			ErrorCode:    "CHARGE_ERROR",
			ErrorMessage: err.Error(),
		}
		e.db.UpdateRetryAfterAttempt(ctx, entry.ID, false, result, e.retryPolicy)
		return result, nil
	}

	result := &ChargeResult{
		Success:       chargeResp.Success,
		TransactionID: chargeResp.TransactionID,
		ProcessorUsed: chargeResp.ProcessorUsed,
		ErrorCode:     chargeResp.ErrorCode,
		ErrorMessage:  chargeResp.ErrorMessage,
	}

	// Update retry entry
	if err := e.db.UpdateRetryAfterAttempt(ctx, entry.ID, result.Success, result, e.retryPolicy); err != nil {
		e.logger.Printf("Error updating retry after attempt: %v", err)
	}

	if result.Success {
		e.logger.Printf("Retry successful for subscription %s: transaction=%s",
			entry.SubscriptionID, result.TransactionID)
	} else {
		e.logger.Printf("Retry failed for subscription %s: error=%s",
			entry.SubscriptionID, result.ErrorCode)
	}

	// Suppress unused variable warning
	_ = sub

	return result, nil
}

// ExecuteRetryBatch processes a batch of due retries
func (e *Executor) ExecuteRetryBatch(ctx context.Context, entries []RetryEntry) *BatchResult {
	start := time.Now()
	result := &BatchResult{
		Jobs: make([]*JobResult, 0, len(entries)),
	}

	for _, entry := range entries {
		chargeResult, err := e.ProcessRetry(ctx, &entry)
		if err != nil {
			result.Failed++
			result.Jobs = append(result.Jobs, &JobResult{
				SubscriptionID: entry.SubscriptionID,
				Success:        false,
				ErrorMessage:   err.Error(),
			})
			continue
		}

		jobResult := &JobResult{
			JobID:          entry.ID,
			SubscriptionID: entry.SubscriptionID,
			Success:        chargeResult.Success,
			TransactionID:  chargeResult.TransactionID,
			ProcessorUsed:  chargeResult.ProcessorUsed,
			ErrorCode:      chargeResult.ErrorCode,
			ErrorMessage:   chargeResult.ErrorMessage,
		}
		result.Jobs = append(result.Jobs, jobResult)

		if chargeResult.Success {
			result.Successful++
		} else {
			result.Failed++
		}

		result.Processed++
	}

	result.Duration = time.Since(start)
	return result
}

// BatchResult holds the results of a batch execution
type BatchResult struct {
	Processed  int            `json:"processed"`
	Successful int            `json:"successful"`
	Failed     int            `json:"failed"`
	Jobs       []*JobResult   `json:"jobs,omitempty"`
	Duration   time.Duration  `json:"duration"`
}

// JobResult holds the result of a single job
type JobResult struct {
	JobID          string `json:"job_id"`
	SubscriptionID string `json:"subscription_id"`
	Success        bool   `json:"success"`
	TransactionID  string `json:"transaction_id,omitempty"`
	ProcessorUsed  string `json:"processor_used,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// ExecuteBatch processes a batch of subscriptions
func (e *Executor) ExecuteBatch(ctx context.Context, subscriptions []Subscription) *BatchResult {
	start := time.Now()
	result := &BatchResult{
		Jobs: make([]*JobResult, 0, len(subscriptions)),
	}

	for _, sub := range subscriptions {
		// Create job
		job, err := e.db.CreateJob(ctx, sub.ID, JobTypeBilling, time.Now())
		if err != nil {
			e.logger.Printf("Error creating job for subscription %s: %v", sub.ID, err)
			result.Failed++
			continue
		}

		// Process job
		if err := e.ProcessJob(ctx, job, &sub); err != nil {
			result.Failed++
			result.Jobs = append(result.Jobs, &JobResult{
				JobID:          job.ID,
				SubscriptionID: sub.ID,
				Success:        false,
				ErrorMessage:   err.Error(),
			})
			continue
		}

		// Get updated job to capture result
		updatedJob, _ := e.db.GetJob(ctx, job.ID)
		if updatedJob != nil {
			jobResult := &JobResult{
				JobID:          updatedJob.ID,
				SubscriptionID: updatedJob.SubscriptionID,
				Success:        updatedJob.Status == JobStatusCompleted,
				TransactionID:  updatedJob.TransactionID,
				ProcessorUsed:  updatedJob.ProcessorUsed,
				ErrorCode:      updatedJob.ErrorCode,
				ErrorMessage:   updatedJob.ErrorMessage,
			}
			result.Jobs = append(result.Jobs, jobResult)

			if jobResult.Success {
				result.Successful++
			} else {
				result.Failed++
			}
		}

		result.Processed++
	}

	result.Duration = time.Since(start)
	return result
}
