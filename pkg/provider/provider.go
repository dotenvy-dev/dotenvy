package provider

import (
	"context"

	"github.com/dotenvy-dev/dotenvy/internal/model"
)

// Provider is the base interface for all secret providers
type Provider interface {
	// Name returns the provider type identifier (e.g., "vercel")
	Name() string
	// DisplayName returns a human-readable name (e.g., "Vercel")
	DisplayName() string
	// Environments returns the list of environments this provider supports
	Environments() []string
	// DefaultMapping returns the default local->remote environment mapping
	DefaultMapping() map[string]string
	// Validate checks if the provider is properly configured and authenticated
	Validate(ctx context.Context) error
}

// Reader can read secrets from a provider
type Reader interface {
	Provider
	// List returns all secrets for a given environment
	List(ctx context.Context, environment string) ([]model.SecretValue, error)
}

// Writer can write secrets to a provider
type Writer interface {
	Provider
	// Set creates or updates a secret
	Set(ctx context.Context, name, value, environment string) error
	// Delete removes a secret
	Delete(ctx context.Context, name, environment string) error
}

// SyncTarget is a provider that supports both reading and writing
type SyncTarget interface {
	Reader
	Writer
}

// Factory creates a new provider instance with the given configuration
type Factory func(config map[string]any) (SyncTarget, error)

// ProviderInfo contains metadata about a registered provider
type ProviderInfo struct {
	Name        string
	DisplayName string
	Factory     Factory
	EnvVar      string // Environment variable for authentication
	WriteOnly   bool   // Provider can't read back secret values
	SdkAuth     bool   // Provider uses SDK credential chain (no token needed)
	Beta        bool   // Provider is in beta
}
