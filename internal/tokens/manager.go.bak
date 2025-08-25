package tokens

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TokenManager handles token creation and management
type TokenManager struct {
	networkTokenURL string
	httpClient      *http.Client
}

// TokenRequest represents a request to create a token
type TokenRequest struct {
	CardNumber  string `json:"card_number"`
	ExpMonth    int    `json:"exp_month"`
	ExpYear     int    `json:"exp_year"`
	CVV         string `json:"cvv"`
	Marketplace string `json:"marketplace,omitempty"`
}

// TokenResponse represents the response from token creation
type TokenResponse struct {
	Success       bool          `json:"success"`
	NetworkToken  *NetworkToken `json:"network_token,omitempty"`
	TokenType     string        `json:"token_type"`
	IsPortable    bool          `json:"is_portable"`
	ErrorCode     string        `json:"error_code,omitempty"`
	ErrorMessage  string        `json:"error_message,omitempty"`
	FallbackInfo  *FallbackInfo `json:"fallback_info,omitempty"`
}

// NetworkToken represents a payment token
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

// FallbackInfo contains information about dual vault fallback
type FallbackInfo struct {
	Reason          string `json:"reason"`
	ProcessorAToken string `json:"processor_a_token"`
	ProcessorBToken string `json:"processor_b_token"`
	RequiredAction  string `json:"required_action"`
}

// ValidateTokenRequest represents a token validation request
type ValidateTokenRequest struct {
	NetworkToken string `json:"network_token"`
	Processor    string `json:"processor"`
}

// ValidateTokenResponse represents token validation response
type ValidateTokenResponse struct {
	Valid          bool   `json:"valid"`
	TokenType      string `json:"token_type"`
	IsPortable     bool   `json:"is_portable"`
	CompatibleWith string `json:"compatible_with"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	NetworkToken string `json:"network_token"`
	ExpMonth     int    `json:"exp_month"`
	ExpYear      int    `json:"exp_year"`
}

// RefreshTokenResponse represents token refresh response
type RefreshTokenResponse struct {
	Success         bool   `json:"success"`
	NewNetworkToken string `json:"new_network_token,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
}

// TokenInfo contains detailed token information
type TokenInfo struct {
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

// ProcessorTokens contains processor-specific tokens for a payment method
type ProcessorTokens struct {
	NetworkToken     string `json:"network_token,omitempty"`
	ProcessorAToken  string `json:"processor_a_token,omitempty"`
	ProcessorBToken  string `json:"processor_b_token,omitempty"`
	TokenType        string `json:"token_type"`
	IsPortable       bool   `json:"is_portable"`
}

// TokenError represents token-related errors
type TokenError struct {
	Code    string
	Message string
	Type    string // "network", "dual_vault", "validation", "refresh"
}

func (e *TokenError) Error() string {
	return fmt.Sprintf("Token %s error (%s): %s", e.Type, e.Code, e.Message)
}

// NewTokenManager creates a new token manager
func NewTokenManager(networkTokenURL string) *TokenManager {
	return &TokenManager{
		networkTokenURL: networkTokenURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateToken attempts to create a network token, falling back to dual vault if needed
func (tm *TokenManager) CreateToken(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", tm.networkTokenURL+"/network-tokens/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := tm.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TokenError{
			Code:    "NETWORK_ERROR",
			Message: fmt.Sprintf("Network error: %v", err),
			Type:    "refresh",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response RefreshTokenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &response, &TokenError{
			Code:    response.ErrorCode,
			Message: response.ErrorMessage,
			Type:    "refresh",
		}
	}

	return &response, nil
}

// GetTokenInfo retrieves detailed information about a token
func (tm *TokenManager) GetTokenInfo(ctx context.Context, networkToken string) (*TokenInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", tm.networkTokenURL+"/network-tokens/"+networkToken, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := tm.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TokenError{
			Code:    "NETWORK_ERROR",
			Message: fmt.Sprintf("Network error: %v", err),
			Type:    "validation",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &TokenError{
			Code:    "TOKEN_NOT_FOUND",
			Message: "Network token not found",
			Type:    "validation",
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tokenInfo TokenInfo
	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &tokenInfo, nil
}

// GetProcessorTokens extracts the appropriate tokens for a specific processor
func (tm *TokenManager) GetProcessorTokens(tokenResponse *TokenResponse, targetProcessor string) *ProcessorTokens {
	if tokenResponse == nil || tokenResponse.NetworkToken == nil {
		return nil
	}

	token := tokenResponse.NetworkToken

	result := &ProcessorTokens{
		TokenType:  token.TokenType,
		IsPortable: token.IsPortable,
	}

	if token.TokenType == "network" {
		// Network tokens work with all processors
		result.NetworkToken = token.NetworkToken
	} else if token.TokenType == "dual_vault" {
		// Use processor-specific tokens
		result.ProcessorAToken = token.ProcessorAToken
		result.ProcessorBToken = token.ProcessorBToken
		
		// Set the network token field to the appropriate processor token for convenience
		switch targetProcessor {
		case "processor_a":
			result.NetworkToken = token.ProcessorAToken
		case "processor_b":
			result.NetworkToken = token.ProcessorBToken
		}
	}

	return result
}

// GetTokenForProcessor returns the appropriate token string for a specific processor
func (tm *TokenManager) GetTokenForProcessor(tokenResponse *TokenResponse, targetProcessor string) (string, error) {
	if tokenResponse == nil || tokenResponse.NetworkToken == nil {
		return "", &TokenError{
			Code:    "NO_TOKEN",
			Message: "No token available",
			Type:    "validation",
		}
	}

	token := tokenResponse.NetworkToken

	if token.TokenType == "network" {
		// Network tokens are portable across all processors
		return token.NetworkToken, nil
	}

	// Dual vault - use processor-specific token
	switch targetProcessor {
	case "processor_a":
		if token.ProcessorAToken == "" {
			return "", &TokenError{
				Code:    "NO_PROCESSOR_TOKEN",
				Message: "No token available for processor A",
				Type:    "dual_vault",
			}
		}
		return token.ProcessorAToken, nil
	case "processor_b":
		if token.ProcessorBToken == "" {
			return "", &TokenError{
				Code:    "NO_PROCESSOR_TOKEN",
				Message: "No token available for processor B",
				Type:    "dual_vault",
			}
		}
		return token.ProcessorBToken, nil
	default:
		return "", &TokenError{
			Code:    "UNKNOWN_PROCESSOR",
			Message: fmt.Sprintf("Unknown processor: %s", targetProcessor),
			Type:    "validation",
		}
	}
}

// IsTokenExpired checks if a token is expired
func (tm *TokenManager) IsTokenExpired(token *NetworkToken) bool {
	if token == nil {
		return true
	}
	return time.Now().After(token.ExpiresAt)
}

// IsTokenExpiringSoon checks if a token expires within the specified duration
func (tm *TokenManager) IsTokenExpiringSoon(token *NetworkToken, within time.Duration) bool {
	if token == nil {
		return true
	}
	return time.Now().Add(within).After(token.ExpiresAt)
}

// GetTokenMetadata extracts useful metadata from a token response
func (tm *TokenManager) GetTokenMetadata(tokenResponse *TokenResponse) map[string]interface{} {
	if tokenResponse == nil || tokenResponse.NetworkToken == nil {
		return nil
	}

	token := tokenResponse.NetworkToken
	metadata := map[string]interface{}{
		"token_type":        token.TokenType,
		"is_portable":       token.IsPortable,
		"last_four":         token.LastFour,
		"brand":            token.Brand,
		"expiry_month":     token.ExpiryMonth,
		"expiry_year":      token.ExpiryYear,
		"supported_markets": token.SupportedMarkets,
		"created_at":       token.CreatedAt,
		"expires_at":       token.ExpiresAt,
	}

	if tokenResponse.FallbackInfo != nil {
		metadata["fallback_reason"] = tokenResponse.FallbackInfo.Reason
		metadata["required_action"] = tokenResponse.FallbackInfo.RequiredAction
	}

	return metadata
}

// CreateTokenWithRetry attempts to create a token with retry logic
func (tm *TokenManager) CreateTokenWithRetry(ctx context.Context, req *TokenRequest, maxRetries int) (*TokenResponse, error) {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Add exponential backoff delay
			delay := time.Duration(attempt) * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		response, err := tm.CreateToken(ctx, req)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if error is retryable
		if tokenErr, ok := err.(*TokenError); ok {
			switch tokenErr.Code {
			case "NETWORK_ERROR", "TIMEOUT", "SERVICE_UNAVAILABLE":
				// Retryable errors
				continue
			default:
				// Non-retryable errors
				return nil, err
			}
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// ValidateAndGetToken validates a token and returns its information
func (tm *TokenManager) ValidateAndGetToken(ctx context.Context, networkToken, processor string) (*TokenInfo, error) {
	// First validate the token
	validation, err := tm.ValidateToken(ctx, networkToken, processor)
	if err != nil {
		return nil, err
	}

	if !validation.Valid {
		return nil, &TokenError{
			Code:    validation.ErrorCode,
			Message: validation.ErrorMessage,
			Type:    "validation",
		}
	}

	// Get detailed token information
	tokenInfo, err := tm.GetTokenInfo(ctx, networkToken)
	if err != nil {
		return nil, err
	}

	return tokenInfo, nil
}network",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response TokenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &response, &TokenError{
			Code:    response.ErrorCode,
			Message: response.ErrorMessage,
			Type:    "network",
		}
	}

	return &response, nil
}

// ValidateToken validates a network token for a specific processor
func (tm *TokenManager) ValidateToken(ctx context.Context, networkToken, processor string) (*ValidateTokenResponse, error) {
	req := ValidateTokenRequest{
		NetworkToken: networkToken,
		Processor:    processor,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", tm.networkTokenURL+"/network-tokens/validate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := tm.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TokenError{
			Code:    "NETWORK_ERROR",
			Message: fmt.Sprintf("Network error: %v", err),
			Type:    "validation",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response ValidateTokenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &response, &TokenError{
			Code:    response.ErrorCode,
			Message: response.ErrorMessage,
			Type:    "validation",
		}
	}

	return &response, nil
}

// RefreshToken refreshes an existing network token
func (tm *TokenManager) RefreshToken(ctx context.Context, networkToken string, expMonth, expYear int) (*RefreshTokenResponse, error) {
	req := RefreshTokenRequest{
		NetworkToken: networkToken,
		ExpMonth:     expMonth,
		ExpYear:      expYear,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", tm.networkTokenURL+"/network-tokens/refresh", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := tm.httpClient.Do(httpReq)
	if err != nil {
		return nil, &TokenError{
			Code:    "NETWORK_ERROR",
			Message: fmt.Sprintf("Network error: %v", err),
			Type:    "