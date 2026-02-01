package convex

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "convex",
		DisplayName: "Convex",
		Factory:     New,
		EnvVar:      "CONVEX_DEPLOY_KEY",
	})
}

// Provider implements the Convex secrets provider
type Provider struct {
	client     *Client
	deployment string
	envVars    map[string]EnvVar // Cache
}

// New creates a new Convex provider
func New(config map[string]any) (provider.SyncTarget, error) {
	token, _ := config["_resolved_token"].(string)
	if token == "" {
		token, _ = config["deploy_key"].(string)
	}
	if token == "" {
		return nil, fmt.Errorf("convex: deploy_key is required")
	}

	deployment, _ := config["deployment"].(string)
	if deployment == "" {
		return nil, fmt.Errorf("convex: deployment is required")
	}

	return &Provider{
		client:     NewClient(token, deployment),
		deployment: deployment,
		envVars:    make(map[string]EnvVar),
	}, nil
}

func (p *Provider) Name() string        { return "convex" }
func (p *Provider) DisplayName() string { return "Convex" }

func (p *Provider) Environments() []string {
	// Convex uses a single "default" environment per deployment
	// Different deployments (dev/prod) are configured as separate targets
	return []string{"default"}
}

func (p *Provider) DefaultMapping() map[string]string {
	return map[string]string{
		"default": "test", // Override based on deployment (dev vs prod)
	}
}

func (p *Provider) Validate(ctx context.Context) error {
	return p.client.ValidateToken(ctx)
}

func (p *Provider) List(ctx context.Context, environment string) ([]model.SecretValue, error) {
	// Convex doesn't have environments per deployment, so we ignore the environment param
	// The environment is handled by having separate deployments (targets)

	envs, err := p.client.ListEnvVars(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	p.envVars = make(map[string]EnvVar)
	for _, e := range envs {
		p.envVars[e.Name] = e
	}

	var secrets []model.SecretValue
	for _, env := range envs {
		secrets = append(secrets, model.SecretValue{
			Name:        env.Name,
			Value:       env.Value,
			Environment: environment,
		})
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	// Environment is ignored for Convex - each deployment is its own target
	return p.client.SetEnvVar(ctx, name, value)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	return p.client.DeleteEnvVar(ctx, name)
}
