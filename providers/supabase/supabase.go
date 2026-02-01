package supabase

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "supabase",
		DisplayName: "Supabase",
		Factory:     New,
		EnvVar:      "SUPABASE_ACCESS_TOKEN",
		WriteOnly:   true,
	})
}

// Provider implements the Supabase secrets provider
type Provider struct {
	client     *Client
	projectRef string
}

// New creates a new Supabase provider
func New(config map[string]any) (provider.SyncTarget, error) {
	token, _ := config["_resolved_token"].(string)
	if token == "" {
		token, _ = config["token"].(string)
	}
	if token == "" {
		return nil, fmt.Errorf("supabase: access token is required")
	}

	projectRef, _ := config["project_ref"].(string)
	if projectRef == "" {
		return nil, fmt.Errorf("supabase: project_ref is required")
	}

	return &Provider{
		client:     NewClient(token, projectRef),
		projectRef: projectRef,
	}, nil
}

func (p *Provider) Name() string        { return "supabase" }
func (p *Provider) DisplayName() string { return "Supabase" }

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
			Value:       "", // Supabase API returns names only, not values
			Environment: environment,
		})
	}

	return result, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	return p.client.UpsertSecrets(ctx, []Secret{
		{Name: name, Value: value},
	})
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	return p.client.DeleteSecrets(ctx, []string{name})
}
