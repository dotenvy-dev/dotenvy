package render

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "render",
		DisplayName: "Render",
		Factory:     New,
		EnvVar:      "RENDER_API_KEY",
	})
}

// Provider implements the Render secrets provider
type Provider struct {
	client    *Client
	serviceID string
}

// New creates a new Render provider
func New(config map[string]any) (provider.SyncTarget, error) {
	token, _ := config["_resolved_token"].(string)
	if token == "" {
		token, _ = config["token"].(string)
	}
	if token == "" {
		return nil, fmt.Errorf("render: API key is required")
	}

	serviceID, _ := config["service_id"].(string)
	if serviceID == "" {
		return nil, fmt.Errorf("render: service_id is required")
	}

	return &Provider{
		client:    NewClient(token, serviceID),
		serviceID: serviceID,
	}, nil
}

func (p *Provider) Name() string        { return "render" }
func (p *Provider) DisplayName() string { return "Render" }

func (p *Provider) Environments() []string {
	return []string{"default"}
}

func (p *Provider) DefaultMapping() map[string]string {
	return map[string]string{
		"default": "test",
	}
}

func (p *Provider) Validate(ctx context.Context) error {
	return p.client.ValidateToken(ctx)
}

func (p *Provider) List(ctx context.Context, environment string) ([]model.SecretValue, error) {
	envs, err := p.client.ListEnvVars(ctx)
	if err != nil {
		return nil, err
	}

	var secrets []model.SecretValue
	for _, env := range envs {
		secrets = append(secrets, model.SecretValue{
			Name:        env.Key,
			Value:       env.Value,
			Environment: environment,
		})
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	return p.client.SetEnvVar(ctx, name, value)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	return p.client.DeleteEnvVar(ctx, name)
}
