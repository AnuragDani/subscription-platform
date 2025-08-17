// cmd/mock-processor-b/main.go
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

type ProcessorB struct {
	mu           sync.RWMutex
	isHealthy    bool
	failureRate  float64
	responseTime time.Duration
	stats        ProcessorStats
}

type ProcessorStats struct {
	TotalRequests       int      `json:"total_requests"`
	SuccessfulCharges   int      `json:"successful_charges"`
	FailedCharges       int      `json:"failed_charges"`
	Refunds             int      `json:"refunds"`
	SuccessRate         float64  `json:"success_rate"`
	AvgResponseTime     int      `json:"avg_response_time_ms"`
	CurrenciesSupported []string `json:"currencies_supported"`
	MultiCurrencyTxns   int      `json:"multi_currency_transactions"`
}

type ChargeRequest struct {
	Amount         int    `json:"amount"`
	Currency       string `json:"currency"`
	Token          string `json:"token"`
	IdempotencyKey string `json:"idempotency_key"`
	NetworkToken   string `json:"network_token,omitempty"`
	ProcessorToken string `json:"processor_token,omitempty"`
	Marketplace    string `json:"marketplace,omitempty"`
}

type ChargeResponse struct {
	Success         bool   `json:"success"`
	TransactionID   string `json:"transaction_id,omitempty"`
	AuthCode        string `json:"auth_code,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
	ProcessorUsed   string `json:"processor_used"`
	TokenType       string `json:"token_type,omitempty"`
	ExchangeRate    string `json:"exchange_rate,omitempty"`
	ProcessedAmount int    `json:"processed_amount,omitempty"`
}

type RefundRequest struct {
	OriginalTransactionID string `json:"original_transaction_id"`
	Amount                int    `json:"amount"`
	Currency              string `json:"currency"`
	Reason                string `json:"reason"`
	IdempotencyKey        string `json:"idempotency_key"`
}

type RefundResponse struct {
	Success           bool   `json:"success"`
	RefundID          string `json:"refund_id,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
	ErrorMessage      string `json:"error_message,omitempty"`
	ProcessedCurrency string `json:"processed_currency,omitempty"`
}

type TokenizeRequest struct {
	CardNumber string `json:"card_number"`
	ExpMonth   int    `json:"exp_month"`
	ExpYear    int    `json:"exp_year"`
	CVV        string `json:"cvv"`
	Currency   string `json:"currency,omitempty"`
}

type TokenizeResponse struct {
	Success               bool   `json:"success"`
	ProcessorToken        string `json:"processor_token,omitempty"`
	LastFour              string `json:"last_four,omitempty"`
	TokenType             string `json:"token_type"`
	ErrorCode             string `json:"error_code,omitempty"`
	ErrorMessage          string `json:"error_message,omitempty"`
	SupportsMultiCurrency bool   `json:"supports_multi_currency"`
}

func NewProcessorB() *ProcessorB {
	return &ProcessorB{
		isHealthy:    true,
		failureRate:  0.10,                   // 10% failure rate (90% success - better than A)
		responseTime: 300 * time.Millisecond, // Slightly slower but more reliable
		stats: ProcessorStats{
			AvgResponseTime:     300,
			CurrenciesSupported: []string{"USD", "EUR", "GBP", "JPY", "AUD", "CAD", "CHF", "SEK", "NOK", "DKK"},
		},
	}
}

func (p *ProcessorB) charge(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	p.mu.Lock()
	p.stats.TotalRequests++
	p.mu.Unlock()

	// Simulate processing time (slightly higher than A)
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

	// Check currency support - Processor B supports more currencies
	supportedCurrencies := map[string]bool{
		"USD": true, "EUR": true, "GBP": true, "JPY": true,
		"AUD": true, "CAD": true, "CHF": true, "SEK": true,
		"NOK": true, "DKK": true,
	}

	if !supportedCurrencies[req.Currency] {
		response := ChargeResponse{
			Success:       false,
			ErrorCode:     "CURRENCY_NOT_SUPPORTED",
			ErrorMessage:  fmt.Sprintf("Currency %s not supported", req.Currency),
			ProcessorUsed: "processor_b",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

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
			ProcessorUsed: "processor_b",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Simulate random failures based on failure rate (lower than A)
	if rand.Float64() < failRate {
		p.mu.Lock()
		p.stats.FailedCharges++
		p.mu.Unlock()

		// Processor B has different error patterns (more international)
		errors := []struct {
			code    string
			message string
			status  int
		}{
			{"CARD_DECLINED", "Payment declined by issuing bank", http.StatusPaymentRequired},
			{"FOREIGN_CARD_DECLINED", "Foreign card declined", http.StatusPaymentRequired},
			{"CURRENCY_CONVERSION_FAILED", "Currency conversion failed", http.StatusUnprocessableEntity},
			{"NETWORK_ERROR", "Network connectivity issue", http.StatusServiceUnavailable},
			{"RATE_LIMITED", "Too many requests", http.StatusTooManyRequests},
		}

		errorType := errors[rand.Intn(len(errors))]
		response := ChargeResponse{
			Success:       false,
			ErrorCode:     errorType.code,
			ErrorMessage:  errorType.message,
			ProcessorUsed: "processor_b",
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

	// Track multi-currency transactions
	if req.Currency != "USD" {
		p.stats.MultiCurrencyTxns++
	}
	p.mu.Unlock()

	// Determine token type used
	tokenType := "processor_specific"
	if req.NetworkToken != "" {
		tokenType = "network"
	}

	// Simulate exchange rate for non-USD
	exchangeRate := ""
	processedAmount := req.Amount
	if req.Currency != "USD" {
		// Mock exchange rates
		rates := map[string]float64{
			"EUR": 0.85, "GBP": 0.73, "JPY": 110.0, "AUD": 1.35,
			"CAD": 1.25, "CHF": 0.92, "SEK": 8.5, "NOK": 8.8, "DKK": 6.2,
		}
		if rate, exists := rates[req.Currency]; exists {
			exchangeRate = fmt.Sprintf("1 USD = %.4f %s", rate, req.Currency)
			processedAmount = int(float64(req.Amount) * rate)
		}
	}

	response := ChargeResponse{
		Success:         true,
		TransactionID:   fmt.Sprintf("txn_b_%s", uuid.New().String()[:8]),
		AuthCode:        fmt.Sprintf("auth_%d", rand.Intn(999999)),
		ProcessorUsed:   "processor_b",
		TokenType:       tokenType,
		ExchangeRate:    exchangeRate,
		ProcessedAmount: processedAmount,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorB) refund(w http.ResponseWriter, r *http.Request) {
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

	// Check if transaction ID belongs to processor B
	if !strings.HasPrefix(req.OriginalTransactionID, "txn_b_") {
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
	time.Sleep(150 * time.Millisecond)

	p.mu.Lock()
	p.stats.Refunds++
	p.mu.Unlock()

	// Simulate 98% refund success rate (better than A)
	if rand.Float64() < 0.02 {
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
		Success:           true,
		RefundID:          fmt.Sprintf("ref_b_%s", uuid.New().String()[:8]),
		ProcessedCurrency: req.Currency,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorB) tokenize(w http.ResponseWriter, r *http.Request) {
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
	time.Sleep(200 * time.Millisecond)

	// Extract last 4 digits
	lastFour := req.CardNumber[len(req.CardNumber)-4:]

	response := TokenizeResponse{
		Success:               true,
		ProcessorToken:        fmt.Sprintf("tok_b_%s_%s", lastFour, uuid.New().String()[:8]),
		LastFour:              lastFour,
		TokenType:             "processor_specific",
		SupportsMultiCurrency: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Admin endpoints for testing
func (p *ProcessorB) setFailureRate(w http.ResponseWriter, r *http.Request) {
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
	p.failureRate = rate / 100
	p.mu.Unlock()

	response := map[string]interface{}{
		"message":      "Failure rate updated",
		"failure_rate": rate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorB) setLatency(w http.ResponseWriter, r *http.Request) {
	latencyStr := r.URL.Query().Get("ms")
	if latencyStr == "" {
		http.Error(w, "Missing ms parameter", http.StatusBadRequest)
		return
	}

	latency, err := strconv.Atoi(latencyStr)
	if err != nil || latency < 0 || latency > 10000 {
		http.Error(w, "Invalid latency (0-10000ms)", http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	p.responseTime = time.Duration(latency) * time.Millisecond
	p.mu.Unlock()

	response := map[string]interface{}{
		"message":    "Latency updated",
		"latency_ms": latency,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorB) toggleStatus(w http.ResponseWriter, r *http.Request) {
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

func (p *ProcessorB) getStats(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	stats := p.stats
	healthy := p.isHealthy
	failRate := p.failureRate
	responseTime := p.responseTime
	p.mu.RUnlock()

	response := map[string]interface{}{
		"processor_name":   "processor_b",
		"is_healthy":       healthy,
		"failure_rate":     failRate * 100,
		"response_time_ms": int(responseTime.Milliseconds()),
		"stats":            stats,
		"specialties":      []string{"multi_currency", "international", "higher_fees", "better_reliability"},
		"timestamp":        time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *ProcessorB) health(w http.ResponseWriter, r *http.Request) {
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
		"service":     "mock-processor-b",
		"status":      status,
		"timestamp":   time.Now(),
		"version":     "1.0.0",
		"specialties": []string{"multi_currency", "international_cards"},
		"currencies":  []string{"USD", "EUR", "GBP", "JPY", "AUD", "CAD", "CHF", "SEK", "NOK", "DKK"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func main() {
	processor := NewProcessorB()
	r := mux.NewRouter()

	// Core payment endpoints
	r.HandleFunc("/charge", processor.charge).Methods("POST")
	r.HandleFunc("/refund", processor.refund).Methods("POST")
	r.HandleFunc("/tokenize", processor.tokenize).Methods("POST")

	// Admin endpoints for testing
	r.HandleFunc("/admin/set-failure-rate", processor.setFailureRate).Methods("POST")
	r.HandleFunc("/admin/set-latency", processor.setLatency).Methods("POST")
	r.HandleFunc("/admin/toggle-status", processor.toggleStatus).Methods("POST")
	r.HandleFunc("/admin/stats", processor.getStats).Methods("GET")

	// Health check
	r.HandleFunc("/health", processor.health).Methods("GET")

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	log.Println("Mock Processor B starting on port 8102")
	log.Println("Default settings: 90% success rate, 300ms response time")
	log.Println("Multi-currency support: USD, EUR, GBP, JPY, AUD, CAD, CHF, SEK, NOK, DKK")
	log.Println("Admin endpoints:")
	log.Println("   POST /admin/set-failure-rate?rate=20")
	log.Println("   POST /admin/set-latency?ms=1000")
	log.Println("   POST /admin/toggle-status")
	log.Println("   GET /admin/stats")

	log.Fatal(http.ListenAndServe(":8102", r))
}
