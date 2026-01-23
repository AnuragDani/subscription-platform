package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Publisher sends events to the Payment Orchestrator's WebSocket hub
type Publisher struct {
	orchestratorURL string
	httpClient      *http.Client
}

// NewPublisher creates a new event publisher
func NewPublisher(orchestratorURL string) *Publisher {
	return &Publisher{
		orchestratorURL: orchestratorURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Event represents an event to publish
type Event struct {
	Type  string      `json:"type"`
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// Publish sends an event to the orchestrator
func (p *Publisher) Publish(ctx context.Context, eventType, eventName string, data interface{}) error {
	event := Event{
		Type:  eventType,
		Event: eventName,
		Data:  data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.orchestratorURL+"/internal/events", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event rejected with status: %d", resp.StatusCode)
	}

	return nil
}

// PublishAsync sends an event asynchronously (fire and forget)
func (p *Publisher) PublishAsync(eventType, eventName string, data interface{}) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Ignore errors for async publishing
		p.Publish(ctx, eventType, eventName, data)
	}()
}

// Event type constants
const (
	TypeSubscription = "subscription"
	TypeScheduler    = "scheduler"
	TypeHealth       = "health"
)

// Subscription event constants
const (
	SubscriptionCreated    = "created"
	SubscriptionUpgraded   = "upgraded"
	SubscriptionDowngraded = "downgraded"
	SubscriptionCanceled   = "canceled"
	SubscriptionPastDue    = "past_due"
	SubscriptionCharged    = "charged"
)

// Scheduler event constants
const (
	SchedulerJobStarted     = "job_started"
	SchedulerJobCompleted   = "job_completed"
	SchedulerRetryScheduled = "retry_scheduled"
	SchedulerRetryFailed    = "retry_failed"
	SchedulerRetrySucceeded = "retry_succeeded"
)

// SubscriptionEventData represents subscription event payload
type SubscriptionEventData struct {
	SubscriptionID string  `json:"subscription_id"`
	UserID         string  `json:"user_id"`
	PlanID         string  `json:"plan_id"`
	PlanName       string  `json:"plan_name,omitempty"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	PreviousPlanID string  `json:"previous_plan_id,omitempty"`
}

// SchedulerEventData represents scheduler event payload
type SchedulerEventData struct {
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

// Helper methods for subscription events

// PublishSubscriptionCreated publishes a subscription created event
func (p *Publisher) PublishSubscriptionCreated(data SubscriptionEventData) {
	p.PublishAsync(TypeSubscription, SubscriptionCreated, data)
}

// PublishSubscriptionUpgraded publishes a subscription upgraded event
func (p *Publisher) PublishSubscriptionUpgraded(data SubscriptionEventData) {
	p.PublishAsync(TypeSubscription, SubscriptionUpgraded, data)
}

// PublishSubscriptionDowngraded publishes a subscription downgraded event
func (p *Publisher) PublishSubscriptionDowngraded(data SubscriptionEventData) {
	p.PublishAsync(TypeSubscription, SubscriptionDowngraded, data)
}

// PublishSubscriptionCanceled publishes a subscription canceled event
func (p *Publisher) PublishSubscriptionCanceled(data SubscriptionEventData) {
	p.PublishAsync(TypeSubscription, SubscriptionCanceled, data)
}

// PublishSubscriptionPastDue publishes a subscription past_due event
func (p *Publisher) PublishSubscriptionPastDue(data SubscriptionEventData) {
	p.PublishAsync(TypeSubscription, SubscriptionPastDue, data)
}

// Helper methods for scheduler events

// PublishJobStarted publishes a job started event
func (p *Publisher) PublishJobStarted(data SchedulerEventData) {
	p.PublishAsync(TypeScheduler, SchedulerJobStarted, data)
}

// PublishJobCompleted publishes a job completed event
func (p *Publisher) PublishJobCompleted(data SchedulerEventData) {
	p.PublishAsync(TypeScheduler, SchedulerJobCompleted, data)
}

// PublishRetryScheduled publishes a retry scheduled event
func (p *Publisher) PublishRetryScheduled(data SchedulerEventData) {
	p.PublishAsync(TypeScheduler, SchedulerRetryScheduled, data)
}

// PublishRetryFailed publishes a retry failed event
func (p *Publisher) PublishRetryFailed(data SchedulerEventData) {
	p.PublishAsync(TypeScheduler, SchedulerRetryFailed, data)
}

// PublishRetrySucceeded publishes a retry succeeded event
func (p *Publisher) PublishRetrySucceeded(data SchedulerEventData) {
	p.PublishAsync(TypeScheduler, SchedulerRetrySucceeded, data)
}
