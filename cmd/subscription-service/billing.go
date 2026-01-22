package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Invoice represents a billing invoice
type Invoice struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	Amount         int64     `json:"amount"` // in cents
	Currency       string    `json:"currency"`
	Status         string    `json:"status"` // paid, failed, pending
	TransactionID  string    `json:"transaction_id,omitempty"`
	ProcessorUsed  string    `json:"processor_used,omitempty"`
	ErrorCode      string    `json:"error_code,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	CreatedAt      time.Time `json:"created_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
}

// ChargeSubscriptionResponse represents the response from charging a subscription
type ChargeSubscriptionResponse struct {
	Success       bool     `json:"success"`
	Invoice       *Invoice `json:"invoice,omitempty"`
	TransactionID string   `json:"transaction_id,omitempty"`
	ProcessorUsed string   `json:"processor_used,omitempty"`
	ErrorCode     string   `json:"error_code,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

// BillingHandler handles billing-related operations
type BillingHandler struct {
	db                 *DB
	orchestratorClient *PaymentOrchestratorClient
	logger             *log.Logger
}

// NewBillingHandler creates a new billing handler
func NewBillingHandler(db *DB, orchestratorURL string, logger *log.Logger) *BillingHandler {
	return &BillingHandler{
		db:                 db,
		orchestratorClient: NewPaymentOrchestratorClient(orchestratorURL),
		logger:             logger,
	}
}

// ChargeSubscription handles POST /subscriptions/{id}/charge
func (bh *BillingHandler) ChargeSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	subscriptionID := vars["id"]

	// Get subscription
	sub, err := bh.db.GetSubscription(ctx, subscriptionID)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			respondError(w, http.StatusNotFound, "Subscription not found", "SUBSCRIPTION_NOT_FOUND")
			return
		}
		bh.logger.Printf("Error getting subscription: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get subscription", "INTERNAL_ERROR")
		return
	}

	// Check subscription status
	if sub.Status == SubscriptionStatusCanceled {
		respondError(w, http.StatusBadRequest, "Cannot charge canceled subscription", "SUBSCRIPTION_CANCELED")
		return
	}

	// Get plan for amount calculation
	plan, err := bh.db.GetPlan(ctx, sub.PlanID)
	if err != nil {
		bh.logger.Printf("Error getting plan: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get plan", "INTERNAL_ERROR")
		return
	}

	// Calculate charge amount (could include proration in real implementation)
	chargeAmount := float64(plan.Amount) / 100 // Convert from cents to dollars for orchestrator

	// Generate idempotency key
	idempotencyKey := fmt.Sprintf("sub_%s_%s", subscriptionID, time.Now().Format("2006-01-02"))

	// Prepare charge request
	chargeReq := &OrchestratorChargeRequest{
		SubscriptionID:  subscriptionID,
		PaymentMethodID: sub.PaymentMethodID,
		Amount:          chargeAmount,
		Currency:        sub.Currency,
		IdempotencyKey:  idempotencyKey,
	}

	// If no payment method, we need to handle it gracefully
	if sub.PaymentMethodID == "" {
		// For demo purposes, create a mock payment method ID
		chargeReq.PaymentMethodID = "pm_demo_" + uuid.New().String()[:8]
	}

	bh.logger.Printf("Charging subscription %s: amount=%.2f, currency=%s, payment_method=%s",
		subscriptionID, chargeAmount, sub.Currency, chargeReq.PaymentMethodID)

	// Call Payment Orchestrator
	chargeResp, err := bh.orchestratorClient.Charge(ctx, chargeReq)
	bh.logger.Printf("Orchestrator response: resp=%+v, err=%v", chargeResp, err)
	if err != nil {
		bh.logger.Printf("Error calling orchestrator: %v", err)

		// Mark subscription as past_due on payment failure
		bh.db.UpdateSubscriptionStatus(ctx, subscriptionID, SubscriptionStatusPastDue)

		respondJSON(w, http.StatusPaymentRequired, ChargeSubscriptionResponse{
			Success:      false,
			ErrorCode:    "ORCHESTRATOR_ERROR",
			ErrorMessage: "Failed to process payment. Please try again later.",
		})
		return
	}

	// Create invoice record
	now := time.Now()
	invoice := &Invoice{
		ID:             uuid.New().String(),
		SubscriptionID: subscriptionID,
		Amount:         plan.Amount,
		Currency:       sub.Currency,
		TransactionID:  chargeResp.TransactionID,
		ProcessorUsed:  chargeResp.ProcessorUsed,
		PeriodStart:    now,
		PeriodEnd:      calculatePeriodEnd(now, plan.Interval),
		CreatedAt:      now,
	}

	if chargeResp.Success {
		invoice.Status = "paid"
		invoice.PaidAt = &now

		// Advance subscription to next billing period
		_, err = bh.db.AdvanceSubscriptionPeriod(ctx, subscriptionID)
		if err != nil {
			bh.logger.Printf("Error advancing subscription period: %v", err)
		}

		bh.logger.Printf("Subscription %s charged successfully: transaction=%s, processor=%s",
			subscriptionID, chargeResp.TransactionID, chargeResp.ProcessorUsed)

		respondJSON(w, http.StatusOK, ChargeSubscriptionResponse{
			Success:       true,
			Invoice:       invoice,
			TransactionID: chargeResp.TransactionID,
			ProcessorUsed: chargeResp.ProcessorUsed,
		})
	} else {
		invoice.Status = "failed"
		invoice.ErrorCode = chargeResp.ErrorCode
		invoice.ErrorMessage = chargeResp.UserMessage

		// Mark subscription as past_due
		bh.db.UpdateSubscriptionStatus(ctx, subscriptionID, SubscriptionStatusPastDue)

		bh.logger.Printf("Subscription %s charge failed: error=%s",
			subscriptionID, chargeResp.ErrorCode)

		respondJSON(w, http.StatusPaymentRequired, ChargeSubscriptionResponse{
			Success:      false,
			Invoice:      invoice,
			ErrorCode:    chargeResp.ErrorCode,
			ErrorMessage: chargeResp.UserMessage,
		})
	}
}

// ListInvoices handles GET /subscriptions/{id}/invoices
// Note: In a real implementation, invoices would be stored in a database table
// For the demo, we return mock data based on transaction history
func (bh *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	subscriptionID := vars["id"]

	// Get subscription to verify it exists
	sub, err := bh.db.GetSubscription(ctx, subscriptionID)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			respondError(w, http.StatusNotFound, "Subscription not found", "SUBSCRIPTION_NOT_FOUND")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get subscription", "INTERNAL_ERROR")
		return
	}

	// Get plan for amount
	plan, _ := bh.db.GetPlan(ctx, sub.PlanID)
	amount := int64(0)
	if plan != nil {
		amount = plan.Amount
	}

	// For demo purposes, generate mock invoice history
	// In production, query from invoices table
	invoices := generateMockInvoices(subscriptionID, amount, sub.Currency, sub.CreatedAt)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"invoices": invoices,
		"total":    len(invoices),
	})
}

// calculatePeriodEnd calculates the end of the billing period
func calculatePeriodEnd(start time.Time, interval string) time.Time {
	if interval == IntervalYearly {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}

// generateMockInvoices generates mock invoice history for demo purposes
func generateMockInvoices(subscriptionID string, amount int64, currency string, createdAt time.Time) []Invoice {
	var invoices []Invoice

	// Generate invoices for each month since subscription creation
	now := time.Now()
	current := createdAt

	for current.Before(now) {
		periodEnd := current.AddDate(0, 1, 0)
		paidAt := current.Add(time.Hour) // Assume paid 1 hour after period start

		invoice := Invoice{
			ID:             uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s-%s", subscriptionID, current.Format("2006-01")))).String(),
			SubscriptionID: subscriptionID,
			Amount:         amount,
			Currency:       currency,
			Status:         "paid",
			TransactionID:  uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("tx-%s-%s", subscriptionID, current.Format("2006-01")))).String(),
			ProcessorUsed:  "processor_a",
			PeriodStart:    current,
			PeriodEnd:      periodEnd,
			CreatedAt:      current,
			PaidAt:         &paidAt,
		}

		invoices = append(invoices, invoice)
		current = periodEnd
	}

	return invoices
}
