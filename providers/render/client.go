package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.render.com/v1"

// Client handles Render API requests
type Client struct {
	apiKey    string
	serviceID string
	http      *http.Client
}

// NewClient creates a new Render API client
func NewClient(apiKey, serviceID string) *Client {
	return &Client{
		apiKey:    apiKey,
		serviceID: serviceID,
		http:      &http.Client{},
	}
}

// EnvVar represents a Render environment variable
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ListEnvVars retrieves all environment variables for a service
func (c *Client) ListEnvVars(ctx context.Context) ([]EnvVar, error) {
	endpoint := fmt.Sprintf("/services/%s/env-vars", c.serviceID)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result []struct {
		EnvVar EnvVar `json:"envVar"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	envVars := make([]EnvVar, len(result))
	for i, r := range result {
		envVars[i] = r.EnvVar
	}

	return envVars, nil
}

// SetEnvVar creates or updates an environment variable
func (c *Client) SetEnvVar(ctx context.Context, key, value string) error {
	endpoint := fmt.Sprintf("/services/%s/env-vars/%s", c.serviceID, key)

	body, err := json.Marshal(map[string]string{
		"value": value,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "PUT", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}

	return nil
}

// DeleteEnvVar deletes an environment variable by fetching all, filtering, and bulk-putting
func (c *Client) DeleteEnvVar(ctx context.Context, key string) error {
	// Get all current env vars
	envVars, err := c.ListEnvVars(ctx)
	if err != nil {
		return fmt.Errorf("failed to list env vars for delete: %w", err)
	}

	// Filter out the target key
	var remaining []EnvVar
	for _, ev := range envVars {
		if ev.Key != key {
			remaining = append(remaining, ev)
		}
	}

	// Bulk PUT the remaining env vars
	endpoint := fmt.Sprintf("/services/%s/env-vars", c.serviceID)

	body, err := json.Marshal(remaining)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "PUT", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.http.Do(req)
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Message != "" {
			return fmt.Errorf("render API error: %s", errResp.Message)
		}
	}

	return fmt.Errorf("render API error: status %d, body: %s", resp.StatusCode, string(body))
}

// ValidateToken checks if the API key is valid
func (c *Client) ValidateToken(ctx context.Context) error {
	_, err := c.ListEnvVars(ctx)
	return err
}
