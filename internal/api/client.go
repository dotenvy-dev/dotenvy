package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/user"
	"time"
)

const (
	DefaultAPIURL = "https://dotenvy.dev"
	timeout       = 5 * time.Second
)

// Event represents a sync event to report to the API.
type Event struct {
	Action      string   `json:"action"`
	Environment string   `json:"environment"`
	Target      string   `json:"target,omitempty"`
	Secrets     []string `json:"secrets"`
	User        string   `json:"user,omitempty"`
	Detail      string   `json:"detail,omitempty"`
}

// Client sends events to the dotenvy API.
type Client struct {
	apiURL string
	apiKey string
	http   *http.Client
}

// NewClient creates a new API client. Returns nil if apiKey is empty.
func NewClient(apiKey, apiURL string) *Client {
	if apiKey == "" {
		return nil
	}
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	return &Client{
		apiURL: apiURL,
		apiKey: apiKey,
		http:   &http.Client{Timeout: timeout},
	}
}

// ReportEvent sends a sync event to the API. Fire-and-forget: errors are
// returned but callers should treat them as non-fatal.
func (c *Client) ReportEvent(event Event) error {
	if event.User == "" {
		if u, err := user.Current(); err == nil {
			event.User = u.Username
		}
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	return nil
}
