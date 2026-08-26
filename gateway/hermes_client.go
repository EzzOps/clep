package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HermesAdapter handles communication with Hermes Agent
type HermesAdapter struct {
	baseURL string
	client  *http.Client
}

// HermesRequest represents a request to Hermes
type HermesRequest struct {
	Query     string                 `json:"query"`
	Context   map[string]interface{} `json:"context"`
	SessionID string                 `json:"session_id"`
	Stream    bool                   `json:"stream"`
}

// HermesResponse represents a response from Hermes
type HermesResponse struct {
	ID         string `json:"id"`
	Reply      string `json:"reply"`
	TokenCount int    `json:"token_count"`
}

// NewHermesAdapter creates a new HermesAdapter
func NewHermesAdapter(baseURL string) *HermesAdapter {
	return &HermesAdapter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Query sends a query to Hermes and returns the response
func (h *HermesAdapter) Query(query string, context map[string]interface{}, sessionID string) (*HermesResponse, error) {
	url := fmt.Sprintf("%s/api/v1/chat", h.baseURL)

	payload := HermesRequest{
		Query:     query,
		Context:   context,
		SessionID: sessionID,
		Stream:    false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Hermes API error: %d - %s", resp.StatusCode, string(body))
	}

	var hermesResp HermesResponse
	if err := json.NewDecoder(resp.Body).Decode(&hermesResp); err != nil {
		return nil, err
	}

	return &hermesResp, nil
}

// QueryStream sends a streaming query to Hermes
func (h *HermesAdapter) QueryStream(query string, context map[string]interface{}, sessionID string, callback func(string)) error {
	url := fmt.Sprintf("%s/api/v1/chat/stream", h.baseURL)

	payload := HermesRequest{
		Query:     query,
		Context:   context,
		SessionID: sessionID,
		Stream:    true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Hermes API error: %d - %s", resp.StatusCode, string(body))
	}

	// Read SSE stream
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			return err
		}

		if delta, ok := event["delta"].(string); ok {
			callback(delta)
		}

		if done, ok := event["done"].(bool); ok && done {
			break
		}
	}

	return nil
}
