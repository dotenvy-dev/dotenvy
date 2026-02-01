package awssm

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "aws-secretsmanager",
		DisplayName: "AWS Secrets Manager",
		Factory:     New,
		SdkAuth:     true,
		Beta:        true,
	})
}

// Provider implements the AWS Secrets Manager provider (JSON blob mode)
type Provider struct {
	client     *client
	region     string
	secretName string
}

// New creates a new AWS Secrets Manager provider
func New(config map[string]any) (provider.SyncTarget, error) {
	region, _ := config["region"].(string)
	if region == "" {
		return nil, fmt.Errorf("aws-secretsmanager: region is required")
	}

	secretName, _ := config["secret_name"].(string)
	if secretName == "" {
		return nil, fmt.Errorf("aws-secretsmanager: secret_name is required")
	}

	profile, _ := config["profile"].(string)

	// Build AWS config options
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("aws-secretsmanager: failed to load AWS config: %w", err)
	}

	smClient := secretsmanager.NewFromConfig(cfg)

	return &Provider{
		client:     newClient(smClient, secretName),
		region:     region,
		secretName: secretName,
	}, nil
}

func (p *Provider) Name() string        { return "aws-secretsmanager" }
func (p *Provider) DisplayName() string { return "AWS Secrets Manager" }

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
	data, err := p.client.GetJSON(ctx)
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
	data, err := p.client.GetJSON(ctx)
	if err != nil {
		return err
	}

	data[name] = value
	return p.client.PutJSON(ctx, data)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	data, err := p.client.GetJSON(ctx)
	if err != nil {
		return err
	}

	delete(data, name)
	return p.client.PutJSON(ctx, data)
}
