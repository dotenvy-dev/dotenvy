package railway

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "railway",
		DisplayName: "Railway",
		Factory:     New,
		EnvVar:      "RAILWAY_TOKEN",
	})
}

// Provider implements the Railway secrets provider
type Provider struct {
	client *Client
}

// New creates a new Railway provider
func New(config map[string]any) (provider.SyncTarget, error) {
	token, _ := config["_resolved_token"].(string)
	if token == "" {
		token, _ = config["token"].(string)
	}
	if token == "" {
		return nil, fmt.Errorf("railway: token is required")
	}

	projectID, _ := config["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("railway: project_id is required")
	}

	serviceID, _ := config["service_id"].(string)

	return &Provider{
		client: NewClient(token, projectID, serviceID),
	}, nil
}

func (p *Provider) Name() string        { return "railway" }
func (p *Provider) DisplayName() string { return "Railway" }

func (p *Provider) Environments() []string {
	return []string{"production", "staging"}
}

func (p *Provider) DefaultMapping() map[string]string {
	return map[string]string{
		"production": "live",
		"staging":    "test",
	}
}

func (p *Provider) Validate(ctx context.Context) error {
	return p.client.ValidateToken(ctx)
}

func (p *Provider) List(ctx context.Context, environment string) ([]model.SecretValue, error) {
	envID, err := p.client.ResolveEnvironmentID(ctx, environment)
	if err != nil {
		return nil, err
	}

	vars, err := p.client.ListVariables(ctx, envID)
	if err != nil {
		return nil, err
	}

	secrets := make([]model.SecretValue, 0, len(vars))
	for name, value := range vars {
		secrets = append(secrets, model.SecretValue{
			Name:        name,
			Value:       value,
			Environment: environment,
		})
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	envID, err := p.client.ResolveEnvironmentID(ctx, environment)
	if err != nil {
		return err
	}

	return p.client.UpsertVariable(ctx, envID, name, value)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	envID, err := p.client.ResolveEnvironmentID(ctx, environment)
	if err != nil {
		return err
	}

	return p.client.DeleteVariable(ctx, envID, name)
}
