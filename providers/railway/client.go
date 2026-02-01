package railway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const graphqlURL = "https://backboard.railway.com/graphql/v2"

// Client handles Railway GraphQL API requests
type Client struct {
	token     string
	projectID string
	serviceID string
	http      *http.Client
	envIDs    map[string]string // environment name -> ID cache
}

// NewClient creates a new Railway API client
func NewClient(token, projectID, serviceID string) *Client {
	return &Client{
		token:     token,
		projectID: projectID,
		serviceID: serviceID,
		http:      &http.Client{},
	}
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ValidateToken checks if the token is valid and fetches environment name->ID mapping
func (c *Client) ValidateToken(ctx context.Context) error {
	_, err := c.fetchEnvironments(ctx)
	return err
}

// fetchEnvironments queries the project and caches environment name->ID mapping
func (c *Client) fetchEnvironments(ctx context.Context) (map[string]string, error) {
	if c.envIDs != nil {
		return c.envIDs, nil
	}

	query := `query project($id: String!) {
		project(id: $id) {
			environments {
				edges {
					node {
						id
						name
					}
				}
			}
		}
	}`

	resp, err := c.doGraphQL(ctx, query, map[string]any{"id": c.projectID})
	if err != nil {
		return nil, err
	}

	var result struct {
		Project struct {
			Environments struct {
				Edges []struct {
					Node struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"environments"`
		} `json:"project"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("railway: failed to decode project response: %w", err)
	}

	c.envIDs = make(map[string]string)
	for _, edge := range result.Project.Environments.Edges {
		c.envIDs[edge.Node.Name] = edge.Node.ID
	}

	return c.envIDs, nil
}

// ResolveEnvironmentID resolves an environment name to its UUID
func (c *Client) ResolveEnvironmentID(ctx context.Context, name string) (string, error) {
	envs, err := c.fetchEnvironments(ctx)
	if err != nil {
		return "", err
	}

	id, ok := envs[name]
	if !ok {
		return "", fmt.Errorf("railway: environment %q not found in project", name)
	}

	return id, nil
}

// ListVariables retrieves all variables for an environment
func (c *Client) ListVariables(ctx context.Context, environmentID string) (map[string]string, error) {
	query := `query variables($projectId: String!, $environmentId: String!, $serviceId: String) {
		variables(projectId: $projectId, environmentId: $environmentId, serviceId: $serviceId)
	}`

	vars := map[string]any{
		"projectId":     c.projectID,
		"environmentId": environmentID,
	}
	if c.serviceID != "" {
		vars["serviceId"] = c.serviceID
	}

	resp, err := c.doGraphQL(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Variables map[string]string `json:"variables"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("railway: failed to decode variables response: %w", err)
	}

	return result.Variables, nil
}

// UpsertVariable creates or updates a variable
func (c *Client) UpsertVariable(ctx context.Context, environmentID, name, value string) error {
	query := `mutation variableUpsert($input: VariableUpsertInput!) {
		variableUpsert(input: $input)
	}`

	input := map[string]any{
		"projectId":     c.projectID,
		"environmentId": environmentID,
		"name":          name,
		"value":         value,
	}
	if c.serviceID != "" {
		input["serviceId"] = c.serviceID
	}

	_, err := c.doGraphQL(ctx, query, map[string]any{"input": input})
	return err
}

// DeleteVariable deletes a variable
func (c *Client) DeleteVariable(ctx context.Context, environmentID, name string) error {
	query := `mutation variableDelete($input: VariableDeleteInput!) {
		variableDelete(input: $input)
	}`

	input := map[string]any{
		"projectId":     c.projectID,
		"environmentId": environmentID,
		"name":          name,
	}
	if c.serviceID != "" {
		input["serviceId"] = c.serviceID
	}

	_, err := c.doGraphQL(ctx, query, map[string]any{"input": input})
	return err
}

func (c *Client) doGraphQL(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	reqBody, err := json.Marshal(graphqlRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return nil, fmt.Errorf("railway: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", graphqlURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("railway: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("railway: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("railway: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("railway API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("railway: failed to decode response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("railway API error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data, nil
}
