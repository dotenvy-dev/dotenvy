package vercel

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "vercel",
		DisplayName: "Vercel",
		Factory:     New,
		EnvVar:      "VERCEL_TOKEN",
	})
}

// Provider implements the Vercel secrets provider
type Provider struct {
	client  *Client
	project string
	envVars map[string]EnvVar // Cache of env vars by key
}

// New creates a new Vercel provider
func New(config map[string]any) (provider.SyncTarget, error) {
	token, _ := config["_resolved_token"].(string)
	if token == "" {
		token, _ = config["token"].(string)
	}
	if token == "" {
		return nil, fmt.Errorf("vercel: token is required")
	}

	project, _ := config["project"].(string)
	if project == "" {
		return nil, fmt.Errorf("vercel: project is required")
	}

	teamID, _ := config["team_id"].(string)

	return &Provider{
		client:  NewClient(token, project, teamID),
		project: project,
		envVars: make(map[string]EnvVar),
	}, nil
}

func (p *Provider) Name() string        { return "vercel" }
func (p *Provider) DisplayName() string { return "Vercel" }

func (p *Provider) Environments() []string {
	return []string{"development", "preview", "production"}
}

func (p *Provider) DefaultMapping() map[string]string {
	return map[string]string{
		"development": "test",
		"preview":     "test",
		"production":  "live",
	}
}

func (p *Provider) Validate(ctx context.Context) error {
	return p.client.ValidateToken(ctx)
}

func (p *Provider) List(ctx context.Context, environment string) ([]model.SecretValue, error) {
	// Fetch all env vars if cache is empty
	if len(p.envVars) == 0 {
		envs, err := p.client.ListEnvVars(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range envs {
			p.envVars[e.Key] = e
		}
	}

	var secrets []model.SecretValue
	for _, env := range p.envVars {
		// Check if this env var targets the requested environment
		if containsTarget(env.Target, environment) {
			secrets = append(secrets, model.SecretValue{
				Name:        env.Key,
				Value:       env.Value,
				Environment: environment,
			})
		}
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	// Refresh cache
	envs, err := p.client.ListEnvVars(ctx)
	if err != nil {
		return err
	}
	p.envVars = make(map[string]EnvVar)
	for _, e := range envs {
		p.envVars[e.Key] = e
	}

	existing, exists := p.envVars[name]

	if exists {
		// Check if this existing var targets our environment
		if containsTarget(existing.Target, environment) {
			// Update the existing var
			updated := EnvVar{
				Key:    name,
				Value:  value,
				Target: existing.Target,
				Type:   "encrypted",
			}
			return p.client.UpdateEnvVar(ctx, existing.ID, updated)
		}
		// Var exists but for different environment - add our environment
		updated := EnvVar{
			Key:    name,
			Value:  value,
			Target: append(existing.Target, environment),
			Type:   "encrypted",
		}
		return p.client.UpdateEnvVar(ctx, existing.ID, updated)
	}

	// Create new env var
	newVar := EnvVar{
		Key:    name,
		Value:  value,
		Target: []string{environment},
		Type:   "encrypted",
	}
	return p.client.CreateEnvVar(ctx, newVar)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	// Refresh cache
	envs, err := p.client.ListEnvVars(ctx)
	if err != nil {
		return err
	}
	p.envVars = make(map[string]EnvVar)
	for _, e := range envs {
		p.envVars[e.Key] = e
	}

	existing, exists := p.envVars[name]
	if !exists {
		return nil // Already doesn't exist
	}

	// If var only targets this environment, delete it entirely
	if len(existing.Target) == 1 && existing.Target[0] == environment {
		return p.client.DeleteEnvVar(ctx, existing.ID)
	}

	// Remove environment from targets
	var newTargets []string
	for _, t := range existing.Target {
		if t != environment {
			newTargets = append(newTargets, t)
		}
	}
	updated := EnvVar{
		Key:    existing.Key,
		Value:  existing.Value,
		Target: newTargets,
		Type:   existing.Type,
	}
	return p.client.UpdateEnvVar(ctx, existing.ID, updated)
}

func containsTarget(targets []string, target string) bool {
	for _, t := range targets {
		if t == target {
			return true
		}
	}
	return false
}
