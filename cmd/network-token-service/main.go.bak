// cmd/network-token-service/main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type NetworkTokenService struct {
	mu            sync.RWMutex
	successRate   float64
	networkTokens map[string]*NetworkToken
	stats         NetworkTokenStats
	processorAURL string
	processorBURL string
	isHealthy     bool
}

type NetworkTokenStats struct {
	TotalRequests        int     `json:"total_requests"`
	NetworkTokensCreated int     `json:"network_tokens_created"`
	DualVaultFallbacks   int     `json:"dual_vault_fallbacks"`
	NetworkTokenRate     float64 `json:"network_token_rate"`
	RefreshRequests      int     `json:"refresh_requests"`
	ValidationRequests   int     `json:"validation_requests"`
}

type NetworkToken struct {
	ID               string    `json:"id"`
	NetworkToken     string    `json:"network_token"`
	TokenType        string    `json:"token_type"` // "network" or "dual_vault"
	LastFour         string    `json:"last_four"`
	Brand            string    `json:"brand"`
	ExpiryMonth      int       `json:"expiry_month"`
	ExpiryYear       int       `json:"expiry_year"`
	IsPortable       bool      `json:"is_portable"`
	ProcessorAToken  string    `json:"processor_a_token,omitempty"`
	ProcessorBToken  string    `json:"processor_b_token,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	SupportedMarkets []string  `json:"supported_markets"`
}

type CreateTokenRequest struct {
	CardNumber  string `json:"card_number"`
	ExpMonth    int    `json:"exp_month"`
	ExpYear     int    `json:"exp_year"`
	CVV         string `json:"cvv"`
	Marketplace string `json:"marketplace,omitempty"`
}

type CreateTokenResponse struct {
	Success      bool          `json:"success"`
	NetworkToken *NetworkToken `json:"network_token,omitempty"`
	TokenType    string        `json:"token_type"`
	IsPortable   bool          `json:"is_portable"`
	ErrorCode    string        `json:"error_code,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	FallbackInfo *FallbackInfo `json:"fallback_info,omitempty"`
}

type FallbackInfo struct {
	Reason          string `json:"reason"`
	ProcessorAToken string `json:"processor_a_token"`
	ProcessorBToken string `json:"processor_b_token"`
	RequiredAction  string `json:"required_action"`
}

type ValidateTokenRequest struct {
	NetworkToken string `json:"network_token"`
	Processor    string `json:"processor"` // "processor_a" or "processor_b"
}

type ValidateTokenResponse struct {
	Valid          bool   `json:"valid"`
	TokenType      string `json:"token_type"`
	IsPortable     bool   `json:"is_portable"`
	CompatibleWith string `json:"compatible_with"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

type RefreshTokenRequest struct {
	NetworkToken string `json:"network_token"`
	ExpMonth     int    `json:"exp_month"`
	ExpYear      int    `json:"exp_year"`
}

type RefreshTokenResponse struct {
	Success         bool   `json:"success"`
	NewNetworkToken string `json:"new_network_token,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
}

type TokenInfoResponse struct {
	NetworkToken     string    `json:"network_token"`
	TokenType        string    `json:"token_type"`
	IsPortable       bool      `json:"is_portable"`
	LastFour         string    `json:"last_four"`
	Brand            string    `json:"brand"`
	ExpiryMonth      int       `json:"expiry_month"`
	ExpiryYear       int       `json:"expiry_year"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	SupportedMarkets []string  `json:"supported_markets"`
}

func NewNetworkTokenService() *NetworkTokenService {
	return &NetworkTokenService{
		successRate:   0.95, // 95% success rate for network tokens
		networkTokens: make(map[string]*NetworkToken),
		stats: NetworkTokenStats{
			NetworkTokenRate: 95.0,
		},
		processorAURL: "http://mock-processor-a:8101",
		processorBURL: "http://mock-processor-b:8102",
		isHealthy:     true,
	}
}

func (nts *NetworkTokenService) createToken(w http.ResponseWriter, r *http.Request) {
	nts.mu.Lock()
	nts.stats.TotalRequests++
	nts.mu.Unlock()

	var req CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate card details
	if len(req.CardNumber) < 13 || req.ExpMonth < 1 || req.ExpMonth > 12 || req.ExpYear < 2025 {
		response := CreateTokenResponse{
			Success:      false,
			ErrorCode:    "INVALID_CARD",
			ErrorMessage: "Invalid card details provided",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Simulate processing time
	time.Sleep(200 * time.Millisecond)

	// Extract card information
	lastFour := req.CardNumber[len(req.CardNumber)-4:]
	brand := nts.getCardBrand(req.CardNumber)

	// Determine if network token creation succeeds (95% rate)
	nts.mu.RLock()
	successRate := nts.successRate
	nts.mu.RUnlock()

	if rand.Float64() < successRate {
		// Network token success path (95%)
		networkToken := &NetworkToken{
			ID:               uuid.New().String(),
			NetworkToken:     fmt.Sprintf("ntk_%s_%s_%s", brand, lastFour, uuid.New().String()[:8]),
			TokenType:        "network",
			LastFour:         lastFour,
			Brand:            brand,
			ExpiryMonth:      req.ExpMonth,
			ExpiryYear:       req.ExpYear,
			IsPortable:       true,
			CreatedAt:        time.Now(),
			ExpiresAt:        time.Date(req.ExpYear, time.Month(req.ExpMonth), 1, 0, 0, 0, 0, time.UTC),
			SupportedMarkets: []string{"US", "EU", "UK", "JP", "AU", "CA"},
		}

		nts.mu.Lock()
		nts.networkTokens[networkToken.NetworkToken] = networkToken
		nts.stats.NetworkTokensCreated++
		nts.stats.NetworkTokenRate = float64(nts.stats.NetworkTokensCreated) / float64(nts.stats.TotalRequests) * 100
		nts.mu.Unlock()

		response := CreateTokenResponse{
			Success:      true,
			NetworkToken: networkToken,
			TokenType:    "network",
			IsPortable:   true,
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// Dual vault fallback path (5%)
	nts.mu.Lock()
	nts.stats.DualVaultFallbacks++
	nts.stats.NetworkTokenRate = float64(nts.stats.NetworkTokensCreated) / float64(nts.stats.TotalRequests) * 100
	nts.mu.Unlock()

	// Create processor-specific tokens for dual vault
	processorAToken := fmt.Sprintf("tok_a_%s_%s", lastFour, uuid.New().String()[:8])
	processorBToken := fmt.Sprintf("tok_b_%s_%s", lastFour, uuid.New().String()[:8])

	// Create dual vault token record
	dualVaultToken := &NetworkToken{
		ID:               uuid.New().String(),
		NetworkToken:     fmt.Sprintf("dvt_%s_%s_%s", brand, lastFour, uuid.New().String()[:8]),
		TokenType:        "dual_vault",
		LastFour:         lastFour,
		Brand:            brand,
		ExpiryMonth:      req.ExpMonth,
		ExpiryYear:       req.ExpYear,
		IsPortable:       false,
		ProcessorAToken:  processorAToken,
		ProcessorBToken:  processorBToken,
		CreatedAt:        time.Now(),
		ExpiresAt:        time.Date(req.ExpYear, time.Month(req.ExpMonth), 1, 0, 0, 0, 0, time.UTC),
		SupportedMarkets: []string{"US", "EU", "UK", "JP", "AU", "CA"},
	}

	nts.networkTokens[dualVaultToken.NetworkToken] = dualVaultToken

	// Determine reason for network token failure
	reasons := []string{
		"card_not_supported",
		"issuer_not_participating",
		"geographic_restriction",
		"card_type_unsupported",
	}
	reason := reasons[rand.Intn(len(reasons))]

	response := CreateTokenResponse{
		Success:    true, // Still successful, just using dual vault
		TokenType:  "dual_vault",
		IsPortable: false,
		FallbackInfo: &FallbackInfo{
			Reason:          reason,
			ProcessorAToken: processorAToken,
			ProcessorBToken: processorBToken,
			RequiredAction:  "use_processor_specific_tokens",
		},
		NetworkToken: dualVaultToken,
	}

	json.NewEncoder(w).Encode(response)
}

func (nts *NetworkTokenService) validateToken(w http.ResponseWriter, r *http.Request) {
	nts.mu.Lock()
	nts.stats.ValidationRequests++
	nts.mu.Unlock()

	var req ValidateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	nts.mu.RLock()
	token, exists := nts.networkTokens[req.NetworkToken]
	nts.mu.RUnlock()

	if !exists {
		response := ValidateTokenResponse{
			Valid:        false,
			ErrorCode:    "TOKEN_NOT_FOUND",
			ErrorMessage: "Network token not found",
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check if token is expired
	if time.Now().After(token.ExpiresAt) {
		response := ValidateTokenResponse{
			Valid:        false,
			ErrorCode:    "TOKEN_EXPIRED",
			ErrorMessage: "Network token has expired",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Determine compatibility
	compatible := "both"
	if token.TokenType == "dual_vault" {
		compatible = "processor_specific"
	}

	response := ValidateTokenResponse{
		Valid:          true,
		TokenType:      token.TokenType,
		IsPortable:     token.IsPortable,
		CompatibleWith: compatible,
	}

	json.NewEncoder(w).Encode(response)
}

func (nts *NetworkTokenService) refreshToken(w http.ResponseWriter, r *http.Request) {
	nts.mu.Lock()
	nts.stats.RefreshRequests++
	nts.mu.Unlock()

	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	nts.mu.RLock()
	token, exists := nts.networkTokens[req.NetworkToken]
	nts.mu.RUnlock()

	if !exists {
		response := RefreshTokenResponse{
			Success:      false,
			ErrorCode:    "TOKEN_NOT_FOUND",
			ErrorMessage: "Network token not found",
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Update token expiry
	newExpiresAt := time.Date(req.ExpYear, time.Month(req.ExpMonth), 1, 0, 0, 0, 0, time.UTC)

	nts.mu.Lock()
	token.ExpiryMonth = req.ExpMonth
	token.ExpiryYear = req.ExpYear
	token.ExpiresAt = newExpiresAt
	nts.mu.Unlock()

	response := RefreshTokenResponse{
		Success:         true,
		NewNetworkToken: token.NetworkToken, // In practice, this might be a new token
		ExpiresAt:       newExpiresAt.Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (nts *NetworkTokenService) getTokenInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	networkToken := vars["token"]

	w.Header().Set("Content-Type", "application/json")

	nts.mu.RLock()
	token, exists := nts.networkTokens[networkToken]
	nts.mu.RUnlock()

	if !exists {
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	response := TokenInfoResponse{
		NetworkToken:     token.NetworkToken,
		TokenType:        token.TokenType,
		IsPortable:       token.IsPortable,
		LastFour:         token.LastFour,
		Brand:            token.Brand,
		ExpiryMonth:      token.ExpiryMonth,
		ExpiryYear:       token.ExpiryYear,
		CreatedAt:        token.CreatedAt,
		ExpiresAt:        token.ExpiresAt,
		SupportedMarkets: token.SupportedMarkets,
	}

	json.NewEncoder(w).Encode(response)
}

func (nts *NetworkTokenService) getStats(w http.ResponseWriter, r *http.Request) {
	nts.mu.RLock()
	stats := nts.stats
	nts.mu.RUnlock()

	response := map[string]interface{}{
		"service_name": "network_token_service",
		"stats":        stats,
		"timestamp":    time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (nts *NetworkTokenService) health(w http.ResponseWriter, r *http.Request) {
	nts.mu.RLock()
	healthy := nts.isHealthy
	nts.mu.RUnlock()

	status := "healthy"
	statusCode := http.StatusOK

	if !healthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"service":            "network-token-service",
		"status":             status,
		"timestamp":          time.Now(),
		"version":            "1.0.0",
		"network_token_rate": 95.0,
		"capabilities":       []string{"network_tokens", "dual_vault_fallback", "token_refresh", "cross_processor_portability"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// Admin endpoints
func (nts *NetworkTokenService) setSuccessRate(w http.ResponseWriter, r *http.Request) {
	rateStr := r.URL.Query().Get("rate")
	if rateStr == "" {
		http.Error(w, "Missing rate parameter (0-100)", http.StatusBadRequest)
		return
	}

	var rate float64
	if _, err := fmt.Sscanf(rateStr, "%f", &rate); err != nil || rate < 0 || rate > 100 {
		http.Error(w, "Invalid rate (0-100)", http.StatusBadRequest)
		return
	}

	nts.mu.Lock()
	nts.successRate = rate / 100
	nts.mu.Unlock()

	response := map[string]interface{}{
		"message":            "Network token success rate updated",
		"network_token_rate": rate,
		"dual_vault_rate":    100 - rate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (nts *NetworkTokenService) resetStats(w http.ResponseWriter, r *http.Request) {
	nts.mu.Lock()
	nts.stats = NetworkTokenStats{
		NetworkTokenRate: nts.successRate * 100,
	}
	nts.mu.Unlock()

	response := map[string]interface{}{
		"message": "Statistics reset successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions
func (nts *NetworkTokenService) getCardBrand(cardNumber string) string {
	// Simple card brand detection
	if strings.HasPrefix(cardNumber, "4") {
		return "visa"
	} else if strings.HasPrefix(cardNumber, "5") {
		return "mastercard"
	} else if strings.HasPrefix(cardNumber, "3") {
		return "amex"
	} else if strings.HasPrefix(cardNumber, "6") {
		return "discover"
	}
	return "unknown"
}

func main() {
	service := NewNetworkTokenService()
	r := mux.NewRouter()

	// Core network token endpoints
	r.HandleFunc("/network-tokens/create", service.createToken).Methods("POST")
	r.HandleFunc("/network-tokens/validate", service.validateToken).Methods("POST")
	r.HandleFunc("/network-tokens/refresh", service.refreshToken).Methods("POST")
	r.HandleFunc("/network-tokens/{token}", service.getTokenInfo).Methods("GET")

	// Debug endpoint to list all routes
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Available endpoints:\n"))
		w.Write([]byte("POST /network-tokens/create\n"))
		w.Write([]byte("POST /network-tokens/validate\n"))
		w.Write([]byte("POST /network-tokens/refresh\n"))
		w.Write([]byte("GET /network-tokens/{token}\n"))
		w.Write([]byte("GET /health\n"))
		w.Write([]byte("GET /admin/stats\n"))
	})

	// Admin endpoints
	r.HandleFunc("/admin/set-success-rate", service.setSuccessRate).Methods("POST")
	r.HandleFunc("/admin/reset-stats", service.resetStats).Methods("POST")
	r.HandleFunc("/admin/stats", service.getStats).Methods("GET")

	// Health check
	r.HandleFunc("/health", service.health).Methods("GET")

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	log.Println("🚀 Network Token Service starting on port 8103")
	log.Println("🎯 Network token success rate: 95% (5% dual vault fallback)")
	log.Println("🔧 Admin endpoints:")
	log.Println("   POST /admin/set-success-rate?rate=90")
	log.Println("   POST /admin/reset-stats")
	log.Println("   GET /admin/stats")
	log.Println("💳 Core endpoints:")
	log.Println("   POST /network-tokens/create")
	log.Println("   POST /network-tokens/validate")
	log.Println("   POST /network-tokens/refresh")
	log.Println("   GET /network-tokens/{token}")

	log.Fatal(http.ListenAndServe(":8103", r))
}
