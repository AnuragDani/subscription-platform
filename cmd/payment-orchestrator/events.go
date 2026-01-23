package main

import (
	"time"

	ws "github.com/AnuragDani/subscription-platform/internal/websocket"
)

// EventEmitter handles emitting events to WebSocket clients
type EventEmitter struct {
	hub *ws.Hub
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter(hub *ws.Hub) *EventEmitter {
	return &EventEmitter{hub: hub}
}

// EmitChargeInitiated emits a charge initiated event
func (e *EventEmitter) EmitChargeInitiated(transactionID, subscriptionID string, amount float64, currency string) {
	if e.hub == nil {
		return
	}

	e.hub.BroadcastEvent(ws.TypeTransaction, ws.EventChargeInitiated, ws.TransactionData{
		TransactionID:  transactionID,
		SubscriptionID: subscriptionID,
		Amount:         amount,
		Currency:       currency,
		Status:         "initiated",
	})
}

// EmitChargeSucceeded emits a charge succeeded event
func (e *EventEmitter) EmitChargeSucceeded(transactionID, subscriptionID string, amount float64, currency, processor string, duration time.Duration) {
	if e.hub == nil {
		return
	}

	e.hub.BroadcastEvent(ws.TypeTransaction, ws.EventChargeSucceeded, ws.TransactionData{
		TransactionID:  transactionID,
		SubscriptionID: subscriptionID,
		Amount:         amount,
		Currency:       currency,
		ProcessorUsed:  processor,
		Status:         "succeeded",
		Duration:       duration.String(),
	})
}

// EmitChargeFailed emits a charge failed event
func (e *EventEmitter) EmitChargeFailed(transactionID, subscriptionID string, amount float64, currency, processor, errorCode, errorMessage string) {
	if e.hub == nil {
		return
	}

	e.hub.BroadcastEvent(ws.TypeTransaction, ws.EventChargeFailed, ws.TransactionData{
		TransactionID:  transactionID,
		SubscriptionID: subscriptionID,
		Amount:         amount,
		Currency:       currency,
		ProcessorUsed:  processor,
		Status:         "failed",
		ErrorCode:      errorCode,
		ErrorMessage:   errorMessage,
	})
}

// EmitFailoverTriggered emits a failover event
func (e *EventEmitter) EmitFailoverTriggered(transactionID string, amount float64, currency, fromProcessor, toProcessor string) {
	if e.hub == nil {
		return
	}

	e.hub.BroadcastEvent(ws.TypeTransaction, ws.EventFailoverTriggered, ws.TransactionData{
		TransactionID:     transactionID,
		Amount:            amount,
		Currency:          currency,
		ProcessorUsed:     toProcessor,
		PreviousProcessor: fromProcessor,
		Status:            "failover",
	})
}

// EmitRefundProcessed emits a refund processed event
func (e *EventEmitter) EmitRefundProcessed(transactionID string, amount float64, currency, processor string, success bool) {
	if e.hub == nil {
		return
	}

	status := "refunded"
	if !success {
		status = "refund_failed"
	}

	e.hub.BroadcastEvent(ws.TypeTransaction, ws.EventRefundProcessed, ws.TransactionData{
		TransactionID: transactionID,
		Amount:        amount,
		Currency:      currency,
		ProcessorUsed: processor,
		Status:        status,
	})
}

// EmitProcessorHealth emits a processor health event
func (e *EventEmitter) EmitProcessorHealth(processor string, healthy bool, successRate float64) {
	if e.hub == nil {
		return
	}

	event := ws.EventProcessorHealthy
	status := "healthy"
	if !healthy {
		event = ws.EventProcessorUnhealthy
		status = "unhealthy"
	}

	e.hub.BroadcastEvent(ws.TypeHealth, event, ws.HealthData{
		Processor:   processor,
		Status:      status,
		SuccessRate: successRate,
	})
}

// EmitSubscriptionEvent emits a subscription event (for cross-service events)
func (e *EventEmitter) EmitSubscriptionEvent(event string, data ws.SubscriptionData) {
	if e.hub == nil {
		return
	}
	e.hub.BroadcastEvent(ws.TypeSubscription, event, data)
}

// EmitSchedulerEvent emits a scheduler event (for cross-service events)
func (e *EventEmitter) EmitSchedulerEvent(event string, data ws.SchedulerData) {
	if e.hub == nil {
		return
	}
	e.hub.BroadcastEvent(ws.TypeScheduler, event, data)
}
