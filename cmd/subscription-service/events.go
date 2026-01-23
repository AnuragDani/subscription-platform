package main

import (
	"github.com/AnuragDani/subscription-platform/internal/events"
)

// EventPublisher wraps the events publisher for subscription-specific events
type EventPublisher struct {
	publisher *events.Publisher
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(orchestratorURL string) *EventPublisher {
	return &EventPublisher{
		publisher: events.NewPublisher(orchestratorURL),
	}
}

// EmitSubscriptionCreated emits a subscription created event
func (e *EventPublisher) EmitSubscriptionCreated(sub *Subscription, plan *Plan) {
	if e == nil || e.publisher == nil {
		return
	}

	data := events.SubscriptionEventData{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
		Amount:         float64(sub.Amount) / 100,
		Currency:       sub.Currency,
		Status:         string(sub.Status),
	}

	if plan != nil {
		data.PlanName = plan.DisplayName
	}

	e.publisher.PublishSubscriptionCreated(data)
}

// EmitSubscriptionUpgraded emits a subscription upgraded event
func (e *EventPublisher) EmitSubscriptionUpgraded(sub *Subscription, plan *Plan, previousPlanID string) {
	if e == nil || e.publisher == nil {
		return
	}

	data := events.SubscriptionEventData{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
		PreviousPlanID: previousPlanID,
		Amount:         float64(sub.Amount) / 100,
		Currency:       sub.Currency,
		Status:         string(sub.Status),
	}

	if plan != nil {
		data.PlanName = plan.DisplayName
	}

	e.publisher.PublishSubscriptionUpgraded(data)
}

// EmitSubscriptionDowngraded emits a subscription downgraded event
func (e *EventPublisher) EmitSubscriptionDowngraded(sub *Subscription, plan *Plan, previousPlanID string) {
	if e == nil || e.publisher == nil {
		return
	}

	data := events.SubscriptionEventData{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
		PreviousPlanID: previousPlanID,
		Amount:         float64(sub.Amount) / 100,
		Currency:       sub.Currency,
		Status:         string(sub.Status),
	}

	if plan != nil {
		data.PlanName = plan.DisplayName
	}

	e.publisher.PublishSubscriptionDowngraded(data)
}

// EmitSubscriptionCanceled emits a subscription canceled event
func (e *EventPublisher) EmitSubscriptionCanceled(sub *Subscription) {
	if e == nil || e.publisher == nil {
		return
	}

	e.publisher.PublishSubscriptionCanceled(events.SubscriptionEventData{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
		Amount:         float64(sub.Amount) / 100,
		Currency:       sub.Currency,
		Status:         string(sub.Status),
	})
}

// EmitSubscriptionPastDue emits a subscription past_due event
func (e *EventPublisher) EmitSubscriptionPastDue(sub *Subscription) {
	if e == nil || e.publisher == nil {
		return
	}

	e.publisher.PublishSubscriptionPastDue(events.SubscriptionEventData{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
		Amount:         float64(sub.Amount) / 100,
		Currency:       sub.Currency,
		Status:         "past_due",
	})
}
