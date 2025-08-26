package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ProcessorClient handles communication with payment processors
type ProcessorClient struct {
	name       string
	baseURL    string
	httpClient *http.Client
	healthy    bool
	mu         sync.RWMutex
}

type ProcessorChargeRequest struct {
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Token          string  `json:"network_token,omitempty"`
	IdempotencyKey string  `json:"idempotency_key"`
}

type ProcessorChargeResponse struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transaction_id"`
	AuthCode      string `json:"auth_code,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

type ProcessorRefundRequest struct {
	OriginalTransactionID string  `json:"original_transaction_id"`
	Amount                float64 `json:"amount"`
	Reason                string  `json:"reason"`
}

type ProcessorRefundResponse struct {
	Success  bool   `json:"success"`
	RefundID string `json:"refund_id"`
	Message  string `json:"message"`
}

func NewProcessorClient(name, baseURL string) *ProcessorClient {
	client := &ProcessorClient{
		name:    name,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		healthy: true,
	}

	// Start health check goroutine
	go client.healthCheckLoop()

	return client
}

func (c *ProcessorClient) Charge(ctx context.Context, req ProcessorChargeRequest) (*ProcessorChargeResponse, error) {
	url := fmt.Sprintf("%s/charge", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.markUnhealthy()
		return nil, fmt.Errorf("processor %s request failed: %w", c.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPaymentRequired {
		c.markUnhealthy()
		return nil, fmt.Errorf("processor %s returned status %d", c.name, resp.StatusCode)
	}

	var result ProcessorChargeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *ProcessorClient) Refund(ctx context.Context, req ProcessorRefundRequest) (*ProcessorRefundResponse, error) {
	url := fmt.Sprintf("%s/refund", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("refund request failed: %w", err)
	}
	defer resp.Body.Close()

	var result ProcessorRefundResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *ProcessorClient) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthy
}

func (c *ProcessorClient) markUnhealthy() {
	c.mu.Lock()
	c.healthy = false
	c.mu.Unlock()
}

func (c *ProcessorClient) markHealthy() {
	c.mu.Lock()
	c.healthy = true
	c.mu.Unlock()
}

func (c *ProcessorClient) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		url := fmt.Sprintf("%s/health", c.baseURL)
		resp, err := c.httpClient.Get(url)
		if err != nil || resp.StatusCode != http.StatusOK {
			c.markUnhealthy()
		} else {
			c.markHealthy()
			resp.Body.Close()
		}
	}
}

// BPASClient handles communication with BPAS service
type BPASClient struct {
	baseURL    string
	httpClient *http.Client
}

type RoutingDecision struct {
	PrimaryProcessor   string  `json:"primary_processor"`
	SecondaryProcessor string  `json:"secondary_processor"`
	Confidence         float64 `json:"confidence"`
}

func NewBPASClient(baseURL string) *BPASClient {
	return &BPASClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (c *BPASClient) GetRoutingDecision(ctx context.Context, amount float64, currency, marketplace string) (*RoutingDecision, error) {
	url := fmt.Sprintf("%s/bpas/evaluate", c.baseURL)

	req := map[string]interface{}{
		"amount":      amount,
		"currency":    currency,
		"marketplace": marketplace,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Default routing on BPAS failure
		return &RoutingDecision{
			PrimaryProcessor:   "processor_a",
			SecondaryProcessor: "processor_b",
			Confidence:         0.5,
		}, nil
	}
	defer resp.Body.Close()

	var result RoutingDecision
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// TokenManager handles token selection and management
type TokenManager struct {
	networkTokenURL string
	processorA      *ProcessorClient
	processorB      *ProcessorClient
}

func NewTokenManager(networkTokenURL string, processorA, processorB *ProcessorClient) *TokenManager {
	return &TokenManager{
		networkTokenURL: networkTokenURL,
		processorA:      processorA,
		processorB:      processorB,
	}
}
