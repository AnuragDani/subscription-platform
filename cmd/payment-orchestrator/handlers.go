package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ChargeRequest struct {
	SubscriptionID  string  `json:"subscription_id"`
	PaymentMethodID string  `json:"payment_method_id"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	IdempotencyKey  string  `json:"idempotency_key,omitempty"`
}

type ChargeResponse struct {
	Success       bool    `json:"success"`
	TransactionID string  `json:"transaction_id"`
	ProcessorUsed string  `json:"processor_used"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	UserMessage   string  `json:"user_message,omitempty"`
	ErrorCode     string  `json:"error_code,omitempty"`
}

func (o *PaymentOrchestrator) processCharge(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	var req ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Generate idempotency key if not provided
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = uuid.New().String()
	}

	// Generate transaction ID early for event tracking
	transactionID := uuid.New().String()

	// Emit charge initiated event
	if o.events != nil {
		o.events.EmitChargeInitiated(transactionID, req.SubscriptionID, req.Amount, req.Currency)
	}

	// Check idempotency
	ctx := r.Context()
	if cached, err := o.checkIdempotency(ctx, req.IdempotencyKey); err == nil && cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Idempotent-Replay", "true")
		json.NewEncoder(w).Encode(cached)
		return
	}

	// Get routing decision from BPAS
	routingDecision, err := o.bpasClient.GetRoutingDecision(ctx, req.Amount, req.Currency, "")
	if err != nil || routingDecision == nil || routingDecision.PrimaryProcessor == "" {
		log.Printf("BPAS routing failed or returned empty, using defaults: %v", err)
		routingDecision = &RoutingDecision{
			PrimaryProcessor:   "processor_a",
			SecondaryProcessor: "processor_b",
			Confidence:         0.5,
		}
	}

	// Ensure we have valid processor names
	if routingDecision.PrimaryProcessor == "" {
		routingDecision.PrimaryProcessor = "processor_a"
	}
	if routingDecision.SecondaryProcessor == "" {
		routingDecision.SecondaryProcessor = "processor_b"
	}

	log.Printf("Using routing: Primary=%s, Secondary=%s",
		routingDecision.PrimaryProcessor,
		routingDecision.SecondaryProcessor)

	// Make sure it's not nil or empty
	if routingDecision == nil || routingDecision.PrimaryProcessor == "" {
		routingDecision = &RoutingDecision{
			PrimaryProcessor:   "processor_a",
			SecondaryProcessor: "processor_b",
		}
	}

	// Get payment method tokens
	paymentMethod, err := o.db.GetPaymentMethod(ctx, req.PaymentMethodID)
	if err != nil {
		http.Error(w, "Payment method not found", http.StatusNotFound)
		return
	}

	// Try primary processor
	failedOver := false
	result, err := o.chargeWithProcessor(ctx, routingDecision.PrimaryProcessor, req, paymentMethod)
	if err != nil {
		log.Printf("Primary processor %s failed: %v", routingDecision.PrimaryProcessor, err)

		// Emit failover event
		if o.events != nil {
			o.events.EmitFailoverTriggered(transactionID, req.Amount, req.Currency,
				routingDecision.PrimaryProcessor, routingDecision.SecondaryProcessor)
		}
		failedOver = true

		// Try secondary processor
		result, err = o.chargeWithProcessor(ctx, routingDecision.SecondaryProcessor, req, paymentMethod)
		if err != nil {
			log.Printf("Secondary processor %s failed: %v", routingDecision.SecondaryProcessor, err)

			// Both processors failed
			result = &ChargeResponse{
				Success:       false,
				TransactionID: transactionID,
				ProcessorUsed: "none",
				Amount:        req.Amount,
				Currency:      req.Currency,
				ErrorCode:     "PROCESSORS_UNAVAILABLE",
				UserMessage:   "Payment processing temporarily unavailable. Please try again in a few minutes.",
			}
		}
	}

	processorTransactionID := result.TransactionID

	// Use our transaction ID
	result.TransactionID = transactionID

	// Store transaction
	transaction := &Transaction{
		ID:                     transactionID,
		SubscriptionID:         req.SubscriptionID,
		PaymentMethodID:        req.PaymentMethodID,
		ProcessorUsed:          result.ProcessorUsed,
		Amount:                 req.Amount,
		Currency:               req.Currency,
		Status:                 getStatus(result.Success),
		IdempotencyKey:         req.IdempotencyKey,
		ProcessorTransactionID: processorTransactionID,
		ErrorCode:              result.ErrorCode,
		UserErrorMessage:       result.UserMessage,
	}

	if err := o.db.CreateTransaction(ctx, transaction); err != nil {
		log.Printf("Failed to store transaction: %v", err)
	}

	// Cache result for idempotency
	o.cacheIdempotencyResult(ctx, req.IdempotencyKey, result)

	// Emit success or failure event
	duration := time.Since(startTime)
	if o.events != nil {
		if result.Success {
			o.events.EmitChargeSucceeded(transactionID, req.SubscriptionID, req.Amount, req.Currency,
				result.ProcessorUsed, duration)
		} else {
			o.events.EmitChargeFailed(transactionID, req.SubscriptionID, req.Amount, req.Currency,
				result.ProcessorUsed, result.ErrorCode, result.UserMessage)
		}
	}

	// Log failover if it happened
	if failedOver && result.Success {
		log.Printf("Charge succeeded after failover to %s", result.ProcessorUsed)
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if result.Success {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusPaymentRequired)
	}
	json.NewEncoder(w).Encode(result)
}

func (o *PaymentOrchestrator) chargeWithProcessor(ctx context.Context, processorName string, req ChargeRequest, pm *PaymentMethod) (*ChargeResponse, error) {
	var processor *ProcessorClient
	var token string

	// Select processor and token
	switch processorName {
	case "processor_a":
		processor = o.processorA
		token = o.selectToken(pm, "processor_a")
	case "processor_b":
		processor = o.processorB
		token = o.selectToken(pm, "processor_b")
	default:
		return nil, fmt.Errorf("unknown processor: %s", processorName)
	}

	// Check processor health
	if !processor.IsHealthy() {
		return nil, fmt.Errorf("processor %s is unhealthy", processorName)
	}

	// Process charge
	processorReq := ProcessorChargeRequest{
		Amount:         req.Amount,
		Currency:       req.Currency,
		Token:          token,
		IdempotencyKey: req.IdempotencyKey,
	}

	processorResp, err := processor.Charge(ctx, processorReq)
	if err != nil {
		return nil, err
	}

	return &ChargeResponse{
		Success:       processorResp.Success,
		TransactionID: processorResp.TransactionID,
		ProcessorUsed: processorName,
		Amount:        req.Amount,
		Currency:      req.Currency,
		UserMessage:   mapErrorToUserMessage(processorResp.ErrorCode),
		ErrorCode:     processorResp.ErrorCode,
	}, nil
}

func (o *PaymentOrchestrator) selectToken(pm *PaymentMethod, processor string) string {
	// Prefer network token
	if pm.NetworkToken != "" {
		return pm.NetworkToken
	}

	// Fall back to processor-specific token
	if processor == "processor_a" && pm.ProcessorAToken != "" {
		return pm.ProcessorAToken
	}
	if processor == "processor_b" && pm.ProcessorBToken != "" {
		return pm.ProcessorBToken
	}

	// Default to any available token
	if pm.ProcessorAToken != "" {
		return pm.ProcessorAToken
	}
	if pm.ProcessorBToken != "" {
		return pm.ProcessorBToken
	}

	return ""
}

func (o *PaymentOrchestrator) checkIdempotency(ctx context.Context, key string) (*ChargeResponse, error) {
	// Check Redis cache
	cacheKey := fmt.Sprintf("idempotency:%s", key)
	val, err := o.cache.Get(ctx, cacheKey)
	if err == nil && val != "" {
		var response ChargeResponse
		if err := json.Unmarshal([]byte(val), &response); err == nil {
			return &response, nil
		}
	}

	// Check database
	transaction, err := o.db.GetTransactionByIdempotencyKey(ctx, key)
	if err == nil && transaction != nil {
		return &ChargeResponse{
			Success:       transaction.Status == "success",
			TransactionID: transaction.ID,
			ProcessorUsed: transaction.ProcessorUsed,
			Amount:        transaction.Amount,
			Currency:      transaction.Currency,
			UserMessage:   transaction.UserErrorMessage,
			ErrorCode:     transaction.ErrorCode,
		}, nil
	}

	return nil, fmt.Errorf("not found")
}

func (o *PaymentOrchestrator) cacheIdempotencyResult(ctx context.Context, key string, result *ChargeResponse) {
	cacheKey := fmt.Sprintf("idempotency:%s", key)
	data, _ := json.Marshal(result)
	o.cache.Set(ctx, cacheKey, string(data), 24*time.Hour)
}

func getStatus(success bool) string {
	if success {
		return "success"
	}
	return "failed"
}

func mapErrorToUserMessage(errorCode string) string {
	messages := map[string]string{
		"CARD_DECLINED":         "Your card was declined. Please try a different payment method.",
		"INSUFFICIENT_FUNDS":    "Insufficient funds. Please try again later or use a different card.",
		"CARD_EXPIRED":          "Your card has expired. Please update your payment method.",
		"NETWORK_ERROR":         "Network error. Please try again in a few moments.",
		"PROCESSOR_UNAVAILABLE": "Payment system temporarily unavailable. Please try again later.",
		"FRAUD_SUSPECTED":       "Payment declined for security reasons. Please contact your bank.",
	}

	if msg, ok := messages[errorCode]; ok {
		return msg
	}
	return "Payment could not be processed. Please try again."
}
