package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PaymentOrchestratorClient handles communication with the Payment Orchestrator service
type PaymentOrchestratorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPaymentOrchestratorClient creates a new Payment Orchestrator client
func NewPaymentOrchestratorClient(baseURL string) *PaymentOrchestratorClient {
	return &PaymentOrchestratorClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ChargeRequest represents a request to charge a subscription
type OrchestratorChargeRequest struct {
	SubscriptionID  string  `json:"subscription_id"`
	PaymentMethodID string  `json:"payment_method_id"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	IdempotencyKey  string  `json:"idempotency_key,omitempty"`
}

// ChargeResponse represents the response from a charge request
type OrchestratorChargeResponse struct {
	Success       bool    `json:"success"`
	TransactionID string  `json:"transaction_id"`
	ProcessorUsed string  `json:"processor_used"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	UserMessage   string  `json:"user_message,omitempty"`
	ErrorCode     string  `json:"error_code,omitempty"`
}

// Charge processes a payment through the Payment Orchestrator
func (c *PaymentOrchestratorClient) Charge(ctx context.Context, req *OrchestratorChargeRequest) (*OrchestratorChargeResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/orchestrator/charge", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call orchestrator: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var chargeResp OrchestratorChargeResponse
	if err := json.Unmarshal(body, &chargeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chargeResp, nil
}

// Health checks the health of the Payment Orchestrator
func (c *PaymentOrchestratorClient) Health(ctx context.Context) (bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetStats retrieves statistics from the Payment Orchestrator
func (c *PaymentOrchestratorClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/admin/stats", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}
