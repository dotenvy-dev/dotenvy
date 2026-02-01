package flyio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.machines.dev/v1"

// Client handles Fly.io Machines API requests
type Client struct {
	token   string
	appName string
	http    *http.Client
}

// NewClient creates a new Fly.io API client
func NewClient(token, appName string) *Client {
	return &Client{
		token:   token,
		appName: appName,
		http:    &http.Client{},
	}
}

// Secret represents a Fly.io secret (list returns name only, no value)
type Secret struct {
	Name      string `json:"name"`
	Digest    string `json:"digest,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ListSecrets retrieves all secret names for an app (values are not returned)
func (c *Client) ListSecrets(ctx context.Context) ([]Secret, error) {
	endpoint := fmt.Sprintf("/apps/%s/secrets", c.appName)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Secrets []Secret `json:"secrets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Secrets, nil
}

// SetSecrets creates or updates secrets via the Machines API.
// The API expects {"values": {"KEY": "VALUE", ...}} with plaintext values.
func (c *Client) SetSecrets(ctx context.Context, values map[string]string) error {
	endpoint := fmt.Sprintf("/apps/%s/secrets", c.appName)

	body, err := json.Marshal(map[string]any{"values": values})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}

	return nil
}

// DeleteSecret deletes a secret by name
func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	endpoint := fmt.Sprintf("/apps/%s/secrets/%s", c.appName, name)

	resp, err := c.doRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
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

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	return c.http.Do(req)
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Message != "" {
			return fmt.Errorf("fly.io API error: %s", errResp.Message)
		}
		if errResp.Error != "" {
			return fmt.Errorf("fly.io API error: %s", errResp.Error)
		}
	}

	return fmt.Errorf("fly.io API error: status %d, body: %s", resp.StatusCode, string(body))
}

// ValidateToken checks if the API token is valid
func (c *Client) ValidateToken(ctx context.Context) error {
	_, err := c.ListSecrets(ctx)
	return err
}
