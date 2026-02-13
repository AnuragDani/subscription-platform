package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
)

type RefundRequest struct {
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Reason        string  `json:"reason"`
}

type RefundResponse struct {
	Success       bool    `json:"success"`
	RefundID      string  `json:"refund_id"`
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	ProcessorUsed string  `json:"processor_used"`
	Message       string  `json:"message"`
}

func (o *PaymentOrchestrator) processRefund(w http.ResponseWriter, r *http.Request) {
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get original transaction
	transaction, err := o.db.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Validate refund amount
	if req.Amount > transaction.Amount {
		http.Error(w, "Refund amount exceeds original transaction", http.StatusBadRequest)
		return
	}

	// Route to original processor
	var processor *ProcessorClient
	switch transaction.ProcessorUsed {
	case "processor_a":
		processor = o.processorA
	case "processor_b":
		processor = o.processorB
	default:
		http.Error(w, "Unknown processor", http.StatusInternalServerError)
		return
	}

	// Process refund
	originalProcessorTxID := transaction.ProcessorTransactionID
	if originalProcessorTxID == "" {
		http.Error(w, "Original processor transaction ID missing", http.StatusUnprocessableEntity)
		return
	}

	refundReq := ProcessorRefundRequest{
		OriginalTransactionID: originalProcessorTxID,
		Amount:                req.Amount,
		Reason:                req.Reason,
	}

	refundResp, err := processor.Refund(ctx, refundReq)
	if err != nil {
		log.Printf("Refund failed: %v", err)
		http.Error(w, "Refund processing failed", http.StatusInternalServerError)
		return
	}

	// Store refund transaction
	refundTransaction := &Transaction{
		ID:                    uuid.New().String(),
		SubscriptionID:        transaction.SubscriptionID,
		PaymentMethodID:       transaction.PaymentMethodID,
		ProcessorUsed:         transaction.ProcessorUsed,
		Amount:                -req.Amount, // Negative for refund
		Currency:              transaction.Currency,
		Status:                "refunded",
		TransactionType:       "refund",
		IdempotencyKey:        "refund_" + uuid.New().String(),
		OriginalTransactionID: &req.TransactionID,
	}

	if err := o.db.CreateTransaction(ctx, refundTransaction); err != nil {
		log.Printf("Failed to store refund transaction: %v", err)
	}

	// Return response
	response := RefundResponse{
		Success:       refundResp.Success,
		RefundID:      refundTransaction.ID,
		TransactionID: req.TransactionID,
		Amount:        req.Amount,
		ProcessorUsed: transaction.ProcessorUsed,
		Message:       "Refund processed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (o *PaymentOrchestrator) getStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := o.db.GetTransactionStats(ctx)
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
