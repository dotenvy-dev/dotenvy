package convex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client handles Convex API requests
type Client struct {
	deployKey  string
	deployment string
	http       *http.Client
}

// NewClient creates a new Convex API client
func NewClient(deployKey, deployment string) *Client {
	return &Client{
		deployKey:  deployKey,
		deployment: deployment,
		http:       &http.Client{},
	}
}

// deploymentURL returns the base URL for a deployment
func (c *Client) deploymentURL() string {
	return fmt.Sprintf("https://%s.convex.cloud", c.deployment)
}

// EnvVar represents a Convex environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ListEnvVars retrieves all environment variables for a deployment
func (c *Client) ListEnvVars(ctx context.Context) ([]EnvVar, error) {
	body, err := json.Marshal(map[string]any{
		"path":   "_system/cli/queryEnvironmentVariables",
		"args":   map[string]any{},
		"format": "json",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/query", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Status string   `json:"status"`
		Value  []EnvVar `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("convex query failed: status %s", result.Status)
	}

	return result.Value, nil
}

// SetEnvVar creates or updates an environment variable
func (c *Client) SetEnvVar(ctx context.Context, name, value string) error {
	body, err := json.Marshal(map[string]any{
		"changes": []map[string]string{
			{"name": name, "value": value},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/update_environment_variables", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// DeleteEnvVar deletes an environment variable
func (c *Client) DeleteEnvVar(ctx context.Context, name string) error {
	body, err := json.Marshal(map[string]any{
		"changes": []map[string]string{
			{"name": name},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/update_environment_variables", body)
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

	req, err := http.NewRequestWithContext(ctx, method, c.deploymentURL()+endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Convex "+c.deployKey)
	req.Header.Set("Content-Type", "application/json")

	return c.http.Do(req)
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Message != "" {
			return fmt.Errorf("convex API error: %s", errResp.Message)
		}
		if errResp.Error != "" {
			return fmt.Errorf("convex API error: %s", errResp.Error)
		}
	}

	return fmt.Errorf("convex API error: status %d, body: %s", resp.StatusCode, string(body))
}

// ValidateToken checks if the deploy key is valid
func (c *Client) ValidateToken(ctx context.Context) error {
	// Try to list env vars as a validation check
	_, err := c.ListEnvVars(ctx)
	return err
}
