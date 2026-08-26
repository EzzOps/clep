package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PlaneAPI handles communication with Plane API
type PlaneAPI struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewPlaneAPI creates a new PlaneAPI client
func NewPlaneAPI(baseURL, apiKey string) *PlaneAPI {
	return &PlaneAPI{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GetWorkItem retrieves a work item from Plane
func (p *PlaneAPI) GetWorkItem(workItemID string) (PlaneWorkItem, error) {
	url := fmt.Sprintf("%s/api/v1/workitems/%s", p.baseURL, workItemID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return PlaneWorkItem{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", p.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return PlaneWorkItem{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PlaneWorkItem{}, fmt.Errorf("Plane API error: %d - %s", resp.StatusCode, string(body))
	}

	var workItem PlaneWorkItem
	if err := json.NewDecoder(resp.Body).Decode(&workItem); err != nil {
		return PlaneWorkItem{}, err
	}

	return workItem, nil
}

// CreateComment creates a new comment on a work item
func (p *PlaneAPI) CreateComment(workItemID, commentText string) (PlaneComment, error) {
	url := fmt.Sprintf("%s/api/v1/comments", p.baseURL)

	payload := map[string]string{
		"comment":  commentText,
		"workitem": workItemID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return PlaneComment{}, err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return PlaneComment{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", p.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return PlaneComment{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PlaneComment{}, fmt.Errorf("Plane API error: %d - %s", resp.StatusCode, string(body))
	}

	var comment PlaneComment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return PlaneComment{}, err
	}

	return comment, nil
}

// UpdateComment updates an existing comment on a work item
func (p *PlaneAPI) UpdateComment(commentID, commentText string) (PlaneComment, error) {
	url := fmt.Sprintf("%s/api/v1/comments/%s", p.baseURL, commentID)

	payload := map[string]string{
		"comment": commentText,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return PlaneComment{}, err
	}

	req, err := http.NewRequest("PATCH", url, body)
	if err != nil {
		return PlaneComment{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", p.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return PlaneComment{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PlaneComment{}, fmt.Errorf("Plane API error: %d - %s", resp.StatusCode, string(body))
	}

	var comment PlaneComment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return PlaneComment{}, err
	}

	return comment, nil
}
