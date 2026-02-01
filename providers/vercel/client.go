package vercel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const baseURL = "https://api.vercel.com"

// Client handles Vercel API requests
type Client struct {
	token     string
	projectID string
	teamID    string
	http      *http.Client
}

// NewClient creates a new Vercel API client
func NewClient(token, projectID, teamID string) *Client {
	return &Client{
		token:     token,
		projectID: projectID,
		teamID:    teamID,
		http:      &http.Client{},
	}
}

// EnvVar represents a Vercel environment variable
type EnvVar struct {
	ID        string   `json:"id,omitempty"`
	Key       string   `json:"key"`
	Value     string   `json:"value"`
	Target    []string `json:"target"`
	Type      string   `json:"type"`
	GitBranch string   `json:"gitBranch,omitempty"`
}

// ListResponse represents the response from listing env vars
type ListResponse struct {
	Envs []EnvVar `json:"envs"`
}

// ListEnvVars retrieves all environment variables for a project
func (c *Client) ListEnvVars(ctx context.Context) ([]EnvVar, error) {
	endpoint := fmt.Sprintf("/v9/projects/%s/env", url.PathEscape(c.projectID))

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Envs, nil
}

// CreateEnvVar creates a new environment variable
func (c *Client) CreateEnvVar(ctx context.Context, env EnvVar) error {
	endpoint := fmt.Sprintf("/v10/projects/%s/env", url.PathEscape(c.projectID))

	body, err := json.Marshal(env)
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

// UpdateEnvVar updates an existing environment variable
func (c *Client) UpdateEnvVar(ctx context.Context, envID string, env EnvVar) error {
	endpoint := fmt.Sprintf("/v9/projects/%s/env/%s", url.PathEscape(c.projectID), url.PathEscape(envID))

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "PATCH", endpoint, body)
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
func (c *Client) DeleteEnvVar(ctx context.Context, envID string) error {
	endpoint := fmt.Sprintf("/v9/projects/%s/env/%s", url.PathEscape(c.projectID), url.PathEscape(envID))

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
	u := baseURL + endpoint
	if c.teamID != "" {
		if bytes.Contains([]byte(u), []byte("?")) {
			u += "&teamId=" + url.QueryEscape(c.teamID)
		} else {
			u += "?teamId=" + url.QueryEscape(c.teamID)
		}
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
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
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("vercel API error: %s (%s)", errResp.Error.Message, errResp.Error.Code)
	}

	return fmt.Errorf("vercel API error: status %d, body: %s", resp.StatusCode, string(body))
}

// ValidateToken checks if the token is valid
func (c *Client) ValidateToken(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "GET", "/v2/user", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}
