package netlify

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "netlify",
		DisplayName: "Netlify",
		Factory:     New,
		EnvVar:      "NETLIFY_TOKEN",
	})
}

// Provider implements the Netlify secrets provider
type Provider struct {
	client    *Client
	accountID string
	siteID    string
}

// New creates a new Netlify provider
func New(config map[string]any) (provider.SyncTarget, error) {
	token, _ := config["_resolved_token"].(string)
	if token == "" {
		token, _ = config["token"].(string)
	}
	if token == "" {
		return nil, fmt.Errorf("netlify: API token is required")
	}

	accountID, _ := config["account_id"].(string)
	if accountID == "" {
		return nil, fmt.Errorf("netlify: account_id is required")
	}

	siteID, _ := config["site_id"].(string)

	return &Provider{
		client:    NewClient(token, accountID, siteID),
		accountID: accountID,
		siteID:    siteID,
	}, nil
}

func (p *Provider) Name() string        { return "netlify" }
func (p *Provider) DisplayName() string { return "Netlify" }

func (p *Provider) Environments() []string {
	return []string{"production", "deploy-preview", "branch-deploy", "dev"}
}

func (p *Provider) DefaultMapping() map[string]string {
	return map[string]string{
		"production":     "live",
		"deploy-preview": "test",
		"branch-deploy":  "test",
		"dev":            "test",
	}
}

func (p *Provider) Validate(ctx context.Context) error {
	return p.client.ValidateToken(ctx)
}

func (p *Provider) List(ctx context.Context, environment string) ([]model.SecretValue, error) {
	envVars, err := p.client.ListEnvVars(ctx, environment)
	if err != nil {
		return nil, err
	}

	var secrets []model.SecretValue
	for _, ev := range envVars {
		// Find the value for the requested context
		value := ""
		for _, v := range ev.Values {
			if v.Context == environment {
				value = v.Value
				break
			}
		}

		secrets = append(secrets, model.SecretValue{
			Name:        ev.Key,
			Value:       value,
			Environment: environment,
		})
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	return p.client.SetEnvVar(ctx, name, value, environment)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	return p.client.DeleteEnvVar(ctx, name)
}
