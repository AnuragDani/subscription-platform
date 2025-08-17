package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	name       string
}

type ChargeRequest struct {
	Amount         int    `json:"amount"`
	Currency       string `json:"currency"`
	Token          string `json:"token,omitempty"`
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

type HealthResponse struct {
	Service     string                 `json:"service"`
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     string                 `json:"version"`
	Specialties []string               `json:"specialties,omitempty"`
	Currencies  []string               `json:"currencies,omitempty"`
	Extra       map[string]interface{} `json:",inline"`
}

type StatsResponse struct {
	ProcessorName  string                 `json:"processor_name"`
	IsHealthy      bool                   `json:"is_healthy"`
	FailureRate    float64                `json:"failure_rate"`
	ResponseTimeMs int                    `json:"response_time_ms,omitempty"`
	Stats          map[string]interface{} `json:"stats"`
	Specialties    []string               `json:"specialties,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

// ProcessorError represents a processor-specific error
type ProcessorError struct {
	Code        string
	Message     string
	StatusCode  int
	Processor   string
	IsRetryable bool
}

func (e *ProcessorError) Error() string {
	return fmt.Sprintf("%s (%s): %s", e.Processor, e.Code, e.Message)
}

// NewClient creates a new processor client
func NewClient(name, baseURL string, timeout time.Duration) *Client {
	return &Client{
		name:    name,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Charge processes a payment charge
func (c *Client) Charge(ctx context.Context, req *ChargeRequest) (*ChargeResponse, error) {
	var response ChargeResponse
	err := c.makeRequest(ctx, "POST", "/charge", req, &response)
	if err != nil {
		return nil, err
	}

	// Convert error response to ProcessorError for better handling
	if !response.Success {
		return &response, &ProcessorError{
			Code:        response.ErrorCode,
			Message:     response.ErrorMessage,
			StatusCode:  0, // Will be set by makeRequest
			Processor:   c.name,
			IsRetryable: c.isRetryableError(response.ErrorCode),
		}
	}

	return &response, nil
}

// Refund processes a refund
func (c *Client) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	var response RefundResponse
	err := c.makeRequest(ctx, "POST", "/refund", req, &response)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return &response, &ProcessorError{
			Code:        response.ErrorCode,
			Message:     response.ErrorMessage,
			Processor:   c.name,
			IsRetryable: false, // Refunds typically shouldn't be retried
		}
	}

	return &response, nil
}

// Tokenize creates a payment token
func (c *Client) Tokenize(ctx context.Context, req *TokenizeRequest) (*TokenizeResponse, error) {
	var response TokenizeResponse
	err := c.makeRequest(ctx, "POST", "/tokenize", req, &response)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return &response, &ProcessorError{
			Code:        response.ErrorCode,
			Message:     response.ErrorMessage,
			Processor:   c.name,
			IsRetryable: false,
		}
	}

	return &response, nil
}

// Health checks processor health
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var response HealthResponse
	err := c.makeRequest(ctx, "GET", "/health", nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetStats retrieves processor statistics
func (c *Client) GetStats(ctx context.Context) (*StatsResponse, error) {
	var response StatsResponse
	err := c.makeRequest(ctx, "GET", "/admin/stats", nil, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// SetFailureRate sets the processor failure rate (for testing)
func (c *Client) SetFailureRate(ctx context.Context, rate float64) error {
	url := fmt.Sprintf("/admin/set-failure-rate?rate=%.1f", rate)
	var response map[string]interface{}
	return c.makeRequest(ctx, "POST", url, nil, &response)
}

// ToggleStatus toggles processor health status (for testing)
func (c *Client) ToggleStatus(ctx context.Context) error {
	var response map[string]interface{}
	return c.makeRequest(ctx, "POST", "/admin/toggle-status", nil, &response)
}

// SetLatency sets processor artificial latency (for testing, Processor B only)
func (c *Client) SetLatency(ctx context.Context, latencyMs int) error {
	url := fmt.Sprintf("/admin/set-latency?ms=%d", latencyMs)
	var response map[string]interface{}
	return c.makeRequest(ctx, "POST", url, nil, &response)
}

// IsHealthy checks if the processor is currently healthy
func (c *Client) IsHealthy(ctx context.Context) bool {
	health, err := c.Health(ctx)
	if err != nil {
		return false
	}
	return health.Status == "healthy"
}

// makeRequest is a helper method for making HTTP requests
func (c *Client) makeRequest(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ProcessorError{
			Code:        "NETWORK_ERROR",
			Message:     fmt.Sprintf("Network error: %v", err),
			Processor:   c.name,
			IsRetryable: true,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle HTTP error status codes
	if resp.StatusCode >= 400 {
		// Try to parse error response
		if response != nil {
			json.Unmarshal(respBody, response)
		}

		return &ProcessorError{
			Code:        c.getErrorCodeFromStatus(resp.StatusCode),
			Message:     string(respBody),
			StatusCode:  resp.StatusCode,
			Processor:   c.name,
			IsRetryable: c.isRetryableStatusCode(resp.StatusCode),
		}
	}

	if response != nil {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// isRetryableError determines if an error code indicates a retryable condition
func (c *Client) isRetryableError(errorCode string) bool {
	retryableErrors := map[string]bool{
		"NETWORK_ERROR":         true,
		"TIMEOUT":               true,
		"PROCESSOR_UNAVAILABLE": true,
		"RATE_LIMITED":          true,
		"INTERNAL_SERVER_ERROR": true,
	}

	return retryableErrors[errorCode]
}

// isRetryableStatusCode determines if an HTTP status code indicates a retryable condition
func (c *Client) isRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case 408, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

// getErrorCodeFromStatus maps HTTP status codes to error codes
func (c *Client) getErrorCodeFromStatus(statusCode int) string {
	switch statusCode {
	case 400:
		return "BAD_REQUEST"
	case 401:
		return "UNAUTHORIZED"
	case 402:
		return "PAYMENT_REQUIRED"
	case 404:
		return "NOT_FOUND"
	case 408:
		return "TIMEOUT"
	case 422:
		return "UNPROCESSABLE_ENTITY"
	case 429:
		return "RATE_LIMITED"
	case 500:
		return "INTERNAL_SERVER_ERROR"
	case 502:
		return "BAD_GATEWAY"
	case 503:
		return "SERVICE_UNAVAILABLE"
	case 504:
		return "GATEWAY_TIMEOUT"
	default:
		return "UNKNOWN_ERROR"
	}
}

// GetName returns the processor name
func (c *Client) GetName() string {
	return c.name
}

// GetBaseURL returns the processor base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}
