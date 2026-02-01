package awsssm

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "aws-ssm",
		DisplayName: "AWS SSM Parameter Store",
		Factory:     New,
		SdkAuth:     true,
		Beta:        true,
	})
}

// Provider implements the AWS SSM Parameter Store provider
type Provider struct {
	client *client
	region string
	prefix string
}

// New creates a new AWS SSM provider
func New(config map[string]any) (provider.SyncTarget, error) {
	region, _ := config["region"].(string)
	if region == "" {
		return nil, fmt.Errorf("aws-ssm: region is required")
	}

	prefix, _ := config["prefix"].(string)
	if prefix == "" {
		prefix = "/"
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
		return nil, fmt.Errorf("aws-ssm: failed to load AWS config: %w", err)
	}

	ssmClient := ssm.NewFromConfig(cfg)

	return &Provider{
		client: newClient(ssmClient, prefix),
		region: region,
		prefix: prefix,
	}, nil
}

func (p *Provider) Name() string        { return "aws-ssm" }
func (p *Provider) DisplayName() string { return "AWS SSM Parameter Store" }

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
	params, err := p.client.ListParameters(ctx)
	if err != nil {
		return nil, err
	}

	var secrets []model.SecretValue
	for name, value := range params {
		secrets = append(secrets, model.SecretValue{
			Name:        name,
			Value:       value,
			Environment: environment,
		})
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	return p.client.PutParameter(ctx, name, value)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	return p.client.DeleteParameter(ctx, name)
}
