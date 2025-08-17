// cmd/mock-processor-a/main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ProcessorA struct {
	mu           sync.RWMutex
	isHealthy    bool
	failureRate  float64
	responseTime time.Duration
	stats        ProcessorStats
}

type ProcessorStats struct {
	TotalRequests     int     `json:"total_requests"`
	SuccessfulCharges int     `json:"successful_charges"`
	FailedCharges     int     `json:"failed_charges"`
	Refunds           int     `json:"refunds"`
	SuccessRate       float64 `json:"success_rate"`
	AvgResponseTime   int     `json:"avg_response_time_ms"`
}

type ChargeRequest struct {
	Amount         int    `json:"amount"`
	Currency       string `json:"currency"`
	Token          string `json:"token"`
	IdempotencyKey string `json:"idempotency_key"`
	NetworkToken   string `json:"network_token,omitempty"`
	ProcessorToken string `json:"processor_token,omitempty"`
}

type ChargeResponse struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transaction_id,omitempty"`
	AuthCode      string `json:"auth_code,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	ProcessorUsed string `json:"processor_used"`
	TokenType     string `json:"token_type,omitempty"`
}

type RefundRequest struct {
	OriginalTransactionID string `json:"original_transaction_id"`
	Amount                int    `json:"amount"`
	Reason                string `json:"reason"`
	IdempotencyKey        string `json:"idempotency_key"`
}

type RefundResponse struct {
	Success      bool   `json:"success"`
	RefundID     string `json:"refund_id,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type TokenizeRequest struct {
	CardNumber string `json:"card_number"`
	ExpMonth   int    `json:"exp_month"`
	ExpYear    int    `json:"exp_year"`
	CVV        string `json:"cvv"`
}

type TokenizeResponse struct {
	Success        bool   `json:"success"`
	ProcessorToken string `json:"processor_token,omitempty"`
	LastFour       string `json:"last_four,omitempty"`
	TokenType      string `json:"token_type"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

func NewProcessorA() *ProcessorA {
	return &ProcessorA{
		isHealthy:    true,
		failureRate:  0.20, // 20% failure rate (80% success)
		responseTime: 250 * time.Millisecond,
		stats: ProcessorStats{
			AvgResponseTime: 250,
		},
	}
}

func (p *ProcessorA) charge(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	p.mu.Lock()
	p.stats.TotalRequests++
	p.mu.Unlock()

	// Simulate processing time
	time.Sleep(p.responseTime)

	var req ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Amount <= 0 || req.Currency == "" || (req.Token == "" && req.NetworkToken == "" && req.ProcessorToken == "") {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check if processor is healthy
	p.mu.RLock()
	healthy := p.isHealthy
	failRate := p.failureRate
	p.mu.RUnlock()

	if !healthy {
		p.mu.Lock()
		p.stats.FailedCharges++
		p.mu.Unlock()

		response := ChargeResponse{
			Success:       false,
			ErrorCode:     "PROCESSOR_UNAVAILABLE",
			ErrorMessage:  "Payment processor temporarily unavailable",
			ProcessorUsed: "processor_a",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Simulate random failures based on failure rate
	if rand.Float64() < failRate {
		p.mu.Lock()
		p.stats.FailedCharges++
		p.mu.Unlock()

		// Return different error types randomly
		errors := []struct {
			code    string
			message string
			status  int
		}{
			{"CARD_DECLINED", "Payment declined by issuing bank", http.StatusPaymentRequired},
			{"INSUFFICIENT_FUNDS", "Insufficient funds on card", http.StatusPaymentRequired},
			{"CARD_EXPIRED", "Card has expired", http.StatusPaymentRequired},
			{"NETWORK_ERROR", "Network connectivity issue", http.StatusServiceUnavailable},
			{"TIMEOUT", "Request timeout", http.StatusRequestTimeout},
		}

		errorType := errors[rand.Intn(len(errors))]
		response := ChargeResponse{
			Success:       false,
			ErrorCode:     errorType.code,
			ErrorMessage:  errorType.message,
			ProcessorUsed: "processor_a",
		}

		w.WriteHeader(errorType.status)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Success case
	p.mu.Lock()
	p.stats.SuccessfulCharges++
	p.stats.SuccessRate = float64(p.stats.SuccessfulCharges) / float64(p.stats.TotalRequests) * 100
	p.stats.AvgResponseTime = int(time.Since(start).Milliseconds())
	p.mu.Unlock()

	// Determine token type used
	tokenType := "processor_specific"
	if req.NetworkToken != "" {
		tokenType = "network"
	}

	response := ChargeResponse{
		Success:       true,
		TransactionID: fmt.Sprintf("txn_a_%s", uuid.New().String()[:8]),
		AuthCode:      fmt.Sprintf("auth_%d", rand.Intn(999999)),
		ProcessorUsed: "processor_a",
		TokenType:     tokenType,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorA) refund(w http.ResponseWriter, r *http.Request) {
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.OriginalTransactionID == "" || req.Amount <= 0 {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check if transaction ID belongs to processor A
	if !strings.HasPrefix(req.OriginalTransactionID, "txn_a_") {
		response := RefundResponse{
			Success:      false,
			ErrorCode:    "TRANSACTION_NOT_FOUND",
			ErrorMessage: "Transaction not found on this processor",
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	p.mu.Lock()
	p.stats.Refunds++
	p.mu.Unlock()

	// Simulate 95% refund success rate
	if rand.Float64() < 0.05 {
		response := RefundResponse{
			Success:      false,
			ErrorCode:    "REFUND_FAILED",
			ErrorMessage: "Refund could not be processed",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := RefundResponse{
		Success:  true,
		RefundID: fmt.Sprintf("ref_a_%s", uuid.New().String()[:8]),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorA) tokenize(w http.ResponseWriter, r *http.Request) {
	var req TokenizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate card number (basic validation)
	if len(req.CardNumber) < 13 || req.ExpMonth < 1 || req.ExpMonth > 12 || req.ExpYear < 2025 {
		response := TokenizeResponse{
			Success:      false,
			ErrorCode:    "INVALID_CARD",
			ErrorMessage: "Invalid card details provided",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Simulate processing time
	time.Sleep(150 * time.Millisecond)

	// Extract last 4 digits
	lastFour := req.CardNumber[len(req.CardNumber)-4:]

	response := TokenizeResponse{
		Success:        true,
		ProcessorToken: fmt.Sprintf("tok_a_%s_%s", lastFour, uuid.New().String()[:8]),
		LastFour:       lastFour,
		TokenType:      "processor_specific",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Admin endpoints for testing
func (p *ProcessorA) setFailureRate(w http.ResponseWriter, r *http.Request) {
	rateStr := r.URL.Query().Get("rate")
	if rateStr == "" {
		http.Error(w, "Missing rate parameter", http.StatusBadRequest)
		return
	}

	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil || rate < 0 || rate > 100 {
		http.Error(w, "Invalid rate (0-100)", http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	p.failureRate = rate / 100 // Convert percentage to decimal
	p.mu.Unlock()

	response := map[string]interface{}{
		"message":      "Failure rate updated",
		"failure_rate": rate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorA) toggleStatus(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.isHealthy = !p.isHealthy
	status := p.isHealthy
	p.mu.Unlock()

	statusStr := "unhealthy"
	if status {
		statusStr = "healthy"
	}

	response := map[string]interface{}{
		"message": "Processor status toggled",
		"status":  statusStr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorA) getStats(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	stats := p.stats
	healthy := p.isHealthy
	failRate := p.failureRate
	p.mu.RUnlock()

	response := map[string]interface{}{
		"processor_name": "processor_a",
		"is_healthy":     healthy,
		"failure_rate":   failRate * 100,
		"stats":          stats,
		"timestamp":      time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorA) health(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	healthy := p.isHealthy
	p.mu.RUnlock()

	status := "healthy"
	statusCode := http.StatusOK

	if !healthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"service":   "mock-processor-a",
		"status":    status,
		"timestamp": time.Now(),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func main() {
	processor := NewProcessorA()
	r := mux.NewRouter()

	// Core payment endpoints
	r.HandleFunc("/charge", processor.charge).Methods("POST")
	r.HandleFunc("/refund", processor.refund).Methods("POST")
	r.HandleFunc("/tokenize", processor.tokenize).Methods("POST")

	// Admin endpoints for testing
	r.HandleFunc("/admin/set-failure-rate", processor.setFailureRate).Methods("POST")
	r.HandleFunc("/admin/toggle-status", processor.toggleStatus).Methods("POST")
	r.HandleFunc("/admin/stats", processor.getStats).Methods("GET")

	// Health check
	r.HandleFunc("/health", processor.health).Methods("GET")

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	log.Println("Mock Processor A starting on port 8101")
	log.Println("Default settings: 80% success rate, 250ms response time")
	log.Println("Admin endpoints:")
	log.Println("   POST /admin/set-failure-rate?rate=50")
	log.Println("   POST /admin/toggle-status")
	log.Println("   GET /admin/stats")

	log.Fatal(http.ListenAndServe(":8101", r))
}
