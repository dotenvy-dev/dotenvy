package netlify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const baseURL = "https://api.netlify.com/api/v1"

// Client handles Netlify API requests
type Client struct {
	token     string
	accountID string
	siteID    string
	http      *http.Client
}

// NewClient creates a new Netlify API client
func NewClient(token, accountID, siteID string) *Client {
	return &Client{
		token:     token,
		accountID: accountID,
		siteID:    siteID,
		http:      &http.Client{},
	}
}

// EnvVar represents a Netlify environment variable
type EnvVar struct {
	Key    string         `json:"key"`
	Scopes []string       `json:"scopes"`
	Values []EnvVarValue  `json:"values"`
}

// EnvVarValue represents a context-specific value for a Netlify env var
type EnvVarValue struct {
	ID      string `json:"id,omitempty"`
	Value   string `json:"value"`
	Context string `json:"context"`
}

// ListEnvVars retrieves all environment variables, filtered by deploy context
func (c *Client) ListEnvVars(ctx context.Context, deployContext string) ([]EnvVar, error) {
	endpoint := fmt.Sprintf("/accounts/%s/env", c.accountID)

	params := url.Values{}
	if c.siteID != "" {
		params.Set("site_id", c.siteID)
	}
	if deployContext != "" {
		params.Set("context_name", deployContext)
	}

	fullURL := baseURL + endpoint
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result []EnvVar
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// SetEnvVar creates or updates an environment variable for a specific context
func (c *Client) SetEnvVar(ctx context.Context, key, value, deployContext string) error {
	// Check if the env var already exists
	existing, err := c.getEnvVar(ctx, key)
	if err != nil {
		// If not found, create new
		return c.createEnvVar(ctx, key, value, deployContext)
	}

	// Update existing: replace the value for this context
	return c.updateEnvVar(ctx, key, value, deployContext, existing)
}

func (c *Client) getEnvVar(ctx context.Context, key string) (*EnvVar, error) {
	endpoint := fmt.Sprintf("/accounts/%s/env/%s", c.accountID, url.PathEscape(key))

	params := url.Values{}
	if c.siteID != "" {
		params.Set("site_id", c.siteID)
	}

	fullURL := baseURL + endpoint
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("env var not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result EnvVar
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) createEnvVar(ctx context.Context, key, value, deployContext string) error {
	endpoint := fmt.Sprintf("/accounts/%s/env", c.accountID)

	params := url.Values{}
	if c.siteID != "" {
		params.Set("site_id", c.siteID)
	}

	envVars := []EnvVar{
		{
			Key:    key,
			Scopes: []string{"builds", "functions", "runtime", "post_processing"},
			Values: []EnvVarValue{
				{Value: value, Context: deployContext},
			},
		},
	}

	body, err := json.Marshal(envVars)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	fullURL := baseURL + endpoint
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}

	return nil
}

func (c *Client) updateEnvVar(ctx context.Context, key, value, deployContext string, existing *EnvVar) error {
	endpoint := fmt.Sprintf("/accounts/%s/env/%s", c.accountID, url.PathEscape(key))

	params := url.Values{}
	if c.siteID != "" {
		params.Set("site_id", c.siteID)
	}

	// Build updated values: replace or add the context value
	var values []EnvVarValue
	found := false
	for _, v := range existing.Values {
		if v.Context == deployContext {
			values = append(values, EnvVarValue{Value: value, Context: deployContext})
			found = true
		} else {
			values = append(values, v)
		}
	}
	if !found {
		values = append(values, EnvVarValue{Value: value, Context: deployContext})
	}

	update := EnvVar{
		Key:    key,
		Scopes: existing.Scopes,
		Values: values,
	}

	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	fullURL := baseURL + endpoint
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// DeleteEnvVar deletes an environment variable
func (c *Client) DeleteEnvVar(ctx context.Context, key string) error {
	endpoint := fmt.Sprintf("/accounts/%s/env/%s", c.accountID, url.PathEscape(key))

	params := url.Values{}
	if c.siteID != "" {
		params.Set("site_id", c.siteID)
	}

	fullURL := baseURL + endpoint
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.parseError(resp)
	}

	return nil
}

// ValidateToken checks if the API token is valid
func (c *Client) ValidateToken(ctx context.Context) error {
	_, err := c.ListEnvVars(ctx, "")
	return err
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Message != "" {
			return fmt.Errorf("netlify API error: %s", errResp.Message)
		}
		if errResp.Error != "" {
			return fmt.Errorf("netlify API error: %s", errResp.Error)
		}
	}

	return fmt.Errorf("netlify API error: status %d, body: %s", resp.StatusCode, string(body))
}
