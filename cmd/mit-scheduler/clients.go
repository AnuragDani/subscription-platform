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

// SubscriptionServiceClient handles communication with the Subscription Service
type SubscriptionServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSubscriptionServiceClient creates a new Subscription Service client
func NewSubscriptionServiceClient(baseURL string) *SubscriptionServiceClient {
	return &SubscriptionServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ChargeRequest represents a request to charge a subscription
type ChargeRequest struct {
	SubscriptionID string `json:"subscription_id"`
}

// ChargeResponse represents the response from a charge request
type ChargeResponse struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transaction_id,omitempty"`
	ProcessorUsed string `json:"processor_used,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	Invoice       *struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Amount int64  `json:"amount"`
	} `json:"invoice,omitempty"`
}

// ChargeSubscription triggers a charge for a subscription
func (c *SubscriptionServiceClient) ChargeSubscription(ctx context.Context, subscriptionID string) (*ChargeResponse, error) {
	url := fmt.Sprintf("%s/subscriptions/%s/charge", c.baseURL, subscriptionID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call subscription service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var chargeResp ChargeResponse
	if err := json.Unmarshal(body, &chargeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chargeResp, nil
}

// GetSubscription retrieves a subscription by ID
func (c *SubscriptionServiceClient) GetSubscription(ctx context.Context, subscriptionID string) (*Subscription, error) {
	url := fmt.Sprintf("%s/subscriptions/%s", c.baseURL, subscriptionID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call subscription service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription not found: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var sub Subscription
	if err := json.Unmarshal(body, &sub); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &sub, nil
}

// Health checks the health of the Subscription Service
func (c *SubscriptionServiceClient) Health(ctx context.Context) (bool, error) {
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
