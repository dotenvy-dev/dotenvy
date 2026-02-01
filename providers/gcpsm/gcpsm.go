package gcpsm

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "gcp-secret-manager",
		DisplayName: "GCP Secret Manager",
		Factory:     New,
		SdkAuth:     true,
		Beta:        true,
	})
}

// Provider implements the GCP Secret Manager provider
type Provider struct {
	client  *client
	project string
	prefix  string
}

// New creates a new GCP Secret Manager provider
func New(config map[string]any) (provider.SyncTarget, error) {
	project, _ := config["project"].(string)
	if project == "" {
		return nil, fmt.Errorf("gcp-secret-manager: project is required")
	}

	prefix, _ := config["prefix"].(string)

	ctx := context.Background()
	smClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcp-secret-manager: failed to create client: %w", err)
	}

	return &Provider{
		client:  newClient(&gcpClientWrapper{inner: smClient}, project, prefix),
		project: project,
		prefix:  prefix,
	}, nil
}

func (p *Provider) Name() string        { return "gcp-secret-manager" }
func (p *Provider) DisplayName() string { return "GCP Secret Manager" }

func (p *Provider) Environments() []string {
	return []string{"default"}
}

func (p *Provider) DefaultMapping() map[string]string {
	return map[string]string{
		"default": "test",
	}
}

func (p *Provider) Validate(ctx context.Context) error {
	return p.client.Validate(ctx)
}

func (p *Provider) List(ctx context.Context, environment string) ([]model.SecretValue, error) {
	data, err := p.client.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}

	var secrets []model.SecretValue
	for name, value := range data {
		secrets = append(secrets, model.SecretValue{
			Name:        name,
			Value:       value,
			Environment: environment,
		})
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	return p.client.SetSecret(ctx, name, value)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	return p.client.DeleteSecret(ctx, name)
}
