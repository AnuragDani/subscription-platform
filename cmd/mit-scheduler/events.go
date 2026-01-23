package main

import (
	"github.com/AnuragDani/subscription-platform/internal/events"
)

// EventPublisher wraps the events publisher for scheduler-specific events
type EventPublisher struct {
	publisher *events.Publisher
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(orchestratorURL string) *EventPublisher {
	return &EventPublisher{
		publisher: events.NewPublisher(orchestratorURL),
	}
}

// EmitJobStarted emits a job started event
func (e *EventPublisher) EmitJobStarted(job *Job) {
	if e == nil || e.publisher == nil {
		return
	}

	e.publisher.PublishJobStarted(events.SchedulerEventData{
		JobID:          job.ID,
		SubscriptionID: job.SubscriptionID,
		Type:           job.Type,
		Status:         "started",
		Attempt:        job.Attempt,
	})
}

// EmitJobCompleted emits a job completed event
func (e *EventPublisher) EmitJobCompleted(job *Job, success bool) {
	if e == nil || e.publisher == nil {
		return
	}

	status := "completed"
	if !success {
		status = "failed"
	}

	e.publisher.PublishJobCompleted(events.SchedulerEventData{
		JobID:          job.ID,
		SubscriptionID: job.SubscriptionID,
		Type:           job.Type,
		Status:         status,
		Attempt:        job.Attempt,
		TransactionID:  job.TransactionID,
		ProcessorUsed:  job.ProcessorUsed,
		ErrorCode:      job.ErrorCode,
		ErrorMessage:   job.ErrorMessage,
	})
}

// EmitRetryScheduled emits a retry scheduled event
func (e *EventPublisher) EmitRetryScheduled(entry *RetryEntry) {
	if e == nil || e.publisher == nil {
		return
	}

	nextRetry := ""
	if !entry.NextRetryAt.IsZero() {
		nextRetry = entry.NextRetryAt.Format("2006-01-02T15:04:05Z")
	}

	e.publisher.PublishRetryScheduled(events.SchedulerEventData{
		JobID:          entry.ID,
		SubscriptionID: entry.SubscriptionID,
		Type:           "retry",
		Status:         "scheduled",
		Attempt:        entry.Attempt,
		MaxAttempts:    entry.MaxAttempts,
		NextRetryAt:    nextRetry,
		ErrorCode:      entry.LastErrorCode,
		ErrorMessage:   entry.LastErrorMessage,
	})
}

// EmitRetryFailed emits a retry failed event
func (e *EventPublisher) EmitRetryFailed(entry *RetryEntry) {
	if e == nil || e.publisher == nil {
		return
	}

	e.publisher.PublishRetryFailed(events.SchedulerEventData{
		JobID:          entry.ID,
		SubscriptionID: entry.SubscriptionID,
		Type:           "retry",
		Status:         "failed",
		Attempt:        entry.Attempt,
		MaxAttempts:    entry.MaxAttempts,
		ErrorCode:      entry.LastErrorCode,
		ErrorMessage:   entry.LastErrorMessage,
	})
}

// EmitRetrySucceeded emits a retry succeeded event
func (e *EventPublisher) EmitRetrySucceeded(entry *RetryEntry) {
	if e == nil || e.publisher == nil {
		return
	}

	e.publisher.PublishRetrySucceeded(events.SchedulerEventData{
		JobID:          entry.ID,
		SubscriptionID: entry.SubscriptionID,
		Type:           "retry",
		Status:         "succeeded",
		Attempt:        entry.Attempt,
		MaxAttempts:    entry.MaxAttempts,
		TransactionID:  entry.TransactionID,
		ProcessorUsed:  entry.ProcessorUsed,
	})
}
