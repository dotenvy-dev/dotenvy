package auth

import (
	"errors"
	"os"

	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

// ErrCredentialsNotFound is returned when no credentials can be resolved
var ErrCredentialsNotFound = errors.New("credentials not found")

// Credentials holds resolved authentication credentials
type Credentials struct {
	Token  string
	Source string // "env", "config", or "prompt"
}

// ResolveCredentials attempts to resolve credentials for a provider
// Resolution order: environment variable > config file
func ResolveCredentials(providerType string, config map[string]any) (*Credentials, error) {
	// 1. Check environment variable (highest priority)
	envVar := provider.EnvVarFor(providerType)
	if envVar != "" {
		if token := os.Getenv(envVar); token != "" {
			return &Credentials{Token: token, Source: "env"}, nil
		}
	}

	// 2. Check config file for token/deploy_key
	if token := getTokenFromConfig(config); token != "" {
		// Expand ${VAR} references in the token
		expanded := os.ExpandEnv(token)
		return &Credentials{Token: expanded, Source: "config"}, nil
	}

	// 3. No credentials found
	return nil, ErrCredentialsNotFound
}

// getTokenFromConfig extracts token from config map
func getTokenFromConfig(config map[string]any) string {
	// Try common token field names
	for _, key := range []string{"token", "deploy_key", "api_key"} {
		if val, ok := config[key].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

// AuthStatus represents the authentication status for a target
type AuthStatus struct {
	TargetName   string
	ProviderType string
	Authenticated bool
	Source        string // How auth was resolved (env, config)
	EnvVar        string // The env var name if applicable
	Error         error
}

// CheckAuth checks authentication status for a target
func CheckAuth(targetName, providerType string, config map[string]any) AuthStatus {
	status := AuthStatus{
		TargetName:   targetName,
		ProviderType: providerType,
		EnvVar:       provider.EnvVarFor(providerType),
	}

	// SDK-auth providers use their own credential chain
	provInfo, _ := provider.Get(providerType)
	if provInfo.SdkAuth {
		status.Authenticated = true
		status.Source = "sdk"
		return status
	}

	creds, err := ResolveCredentials(providerType, config)
	if err != nil {
		status.Error = err
		return status
	}

	status.Authenticated = true
	status.Source = creds.Source
	return status
}
