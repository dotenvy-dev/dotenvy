package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.supabase.com"

// Client handles Supabase API requests
type Client struct {
	accessToken string
	projectRef  string
	http        *http.Client
}

// NewClient creates a new Supabase API client
func NewClient(accessToken, projectRef string) *Client {
	return &Client{
		accessToken: accessToken,
		projectRef:  projectRef,
		http:        &http.Client{},
	}
}

// Secret represents a Supabase secret (list returns name only, no value)
type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// ListSecrets retrieves all secret names for a project (values are not returned)
func (c *Client) ListSecrets(ctx context.Context) ([]Secret, error) {
	endpoint := fmt.Sprintf("/v1/projects/%s/secrets", c.projectRef)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result []Secret
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// UpsertSecrets creates or updates one or more secrets
func (c *Client) UpsertSecrets(ctx context.Context, secrets []Secret) error {
	endpoint := fmt.Sprintf("/v1/projects/%s/secrets", c.projectRef)

	body, err := json.Marshal(secrets)
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

// DeleteSecrets deletes one or more secrets by name
func (c *Client) DeleteSecrets(ctx context.Context, names []string) error {
	endpoint := fmt.Sprintf("/v1/projects/%s/secrets", c.projectRef)

	body, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "DELETE", endpoint, body)
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

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
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
			return fmt.Errorf("supabase API error: %s", errResp.Message)
		}
		if errResp.Error != "" {
			return fmt.Errorf("supabase API error: %s", errResp.Error)
		}
	}

	return fmt.Errorf("supabase API error: status %d, body: %s", resp.StatusCode, string(body))
}

// ValidateToken checks if the access token is valid
func (c *Client) ValidateToken(ctx context.Context) error {
	_, err := c.ListSecrets(ctx)
	return err
}
