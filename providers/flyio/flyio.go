package flyio

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "flyio",
		DisplayName: "Fly.io",
		Factory:     New,
		EnvVar:      "FLY_API_TOKEN",
		WriteOnly:   true,
	})
}

// Provider implements the Fly.io secrets provider
type Provider struct {
	client  *Client
	appName string
}

// New creates a new Fly.io provider
func New(config map[string]any) (provider.SyncTarget, error) {
	token, _ := config["_resolved_token"].(string)
	if token == "" {
		token, _ = config["token"].(string)
	}
	if token == "" {
		return nil, fmt.Errorf("flyio: API token is required")
	}

	appName, _ := config["app_name"].(string)
	if appName == "" {
		return nil, fmt.Errorf("flyio: app_name is required")
	}

	return &Provider{
		client:  NewClient(token, appName),
		appName: appName,
	}, nil
}

func (p *Provider) Name() string        { return "flyio" }
func (p *Provider) DisplayName() string { return "Fly.io" }

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
	secrets, err := p.client.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}

	var result []model.SecretValue
	for _, s := range secrets {
		result = append(result, model.SecretValue{
			Name:        s.Name,
			Value:       "", // Fly.io API returns names only, not values
			Environment: environment,
		})
	}

	return result, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	return p.client.SetSecrets(ctx, map[string]string{name: value})
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	return p.client.DeleteSecret(ctx, name)
}
