package auth

import (
	"os"
	"testing"

	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	// Register providers for testing
	provider.Register(provider.ProviderInfo{
		Name:        "testprovider",
		DisplayName: "Test Provider",
		EnvVar:      "TEST_PROVIDER_TOKEN",
	})
	provider.Register(provider.ProviderInfo{
		Name:        "vercel",
		DisplayName: "Vercel",
		EnvVar:      "VERCEL_TOKEN",
	})
	provider.Register(provider.ProviderInfo{
		Name:        "convex",
		DisplayName: "Convex",
		EnvVar:      "CONVEX_DEPLOY_KEY",
	})
	provider.Register(provider.ProviderInfo{
		Name:        "railway",
		DisplayName: "Railway",
		EnvVar:      "RAILWAY_TOKEN",
	})
	provider.Register(provider.ProviderInfo{
		Name:        "render",
		DisplayName: "Render",
		EnvVar:      "RENDER_API_KEY",
	})
	provider.Register(provider.ProviderInfo{
		Name:        "supabase",
		DisplayName: "Supabase",
		EnvVar:      "SUPABASE_ACCESS_TOKEN",
		WriteOnly:   true,
	})
	provider.Register(provider.ProviderInfo{
		Name:        "netlify",
		DisplayName: "Netlify",
		EnvVar:      "NETLIFY_TOKEN",
	})
	provider.Register(provider.ProviderInfo{
		Name:        "flyio",
		DisplayName: "Fly.io",
		EnvVar:      "FLY_API_TOKEN",
		WriteOnly:   true,
	})
}

func TestResolveCredentials_EnvVar(t *testing.T) {
	os.Setenv("VERCEL_TOKEN", "env_token_123")
	defer os.Unsetenv("VERCEL_TOKEN")

	creds, err := ResolveCredentials("vercel", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.Token != "env_token_123" {
		t.Errorf("expected env_token_123, got %q", creds.Token)
	}
	if creds.Source != "env" {
		t.Errorf("expected source 'env', got %q", creds.Source)
	}
}

func TestResolveCredentials_Config(t *testing.T) {
	// Make sure env var is not set
	os.Unsetenv("VERCEL_TOKEN")

	config := map[string]any{
		"token": "config_token_456",
	}

	creds, err := ResolveCredentials("vercel", config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.Token != "config_token_456" {
		t.Errorf("expected config_token_456, got %q", creds.Token)
	}
	if creds.Source != "config" {
		t.Errorf("expected source 'config', got %q", creds.Source)
	}
}

func TestResolveCredentials_ConfigWithEnvRef(t *testing.T) {
	os.Unsetenv("VERCEL_TOKEN")
	os.Setenv("MY_VERCEL_TOKEN", "referenced_token")
	defer os.Unsetenv("MY_VERCEL_TOKEN")

	config := map[string]any{
		"token": "${MY_VERCEL_TOKEN}",
	}

	creds, err := ResolveCredentials("vercel", config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.Token != "referenced_token" {
		t.Errorf("expected referenced_token, got %q", creds.Token)
	}
}

func TestResolveCredentials_EnvTakesPrecedence(t *testing.T) {
	os.Setenv("VERCEL_TOKEN", "env_wins")
	defer os.Unsetenv("VERCEL_TOKEN")

	config := map[string]any{
		"token": "config_loses",
	}

	creds, err := ResolveCredentials("vercel", config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.Token != "env_wins" {
		t.Errorf("env var should take precedence, got %q", creds.Token)
	}
	if creds.Source != "env" {
		t.Errorf("expected source 'env', got %q", creds.Source)
	}
}

func TestResolveCredentials_NotFound(t *testing.T) {
	os.Unsetenv("VERCEL_TOKEN")

	_, err := ResolveCredentials("vercel", nil)
	if err != ErrCredentialsNotFound {
		t.Errorf("expected ErrCredentialsNotFound, got %v", err)
	}
}

func TestResolveCredentials_DeployKey(t *testing.T) {
	os.Unsetenv("CONVEX_DEPLOY_KEY")

	config := map[string]any{
		"deploy_key": "convex_key_789",
	}

	creds, err := ResolveCredentials("convex", config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.Token != "convex_key_789" {
		t.Errorf("expected convex_key_789, got %q", creds.Token)
	}
}

func TestCheckAuth(t *testing.T) {
	tests := []struct {
		name          string
		targetName    string
		providerType  string
		config        map[string]any
		envVar        string
		envValue      string
		wantAuth      bool
		wantSource    string
	}{
		{
			name:         "authenticated via env",
			targetName:   "vercel",
			providerType: "vercel",
			envVar:       "VERCEL_TOKEN",
			envValue:     "tok123",
			wantAuth:     true,
			wantSource:   "env",
		},
		{
			name:         "authenticated via config",
			targetName:   "vercel",
			providerType: "vercel",
			config:       map[string]any{"token": "tok456"},
			wantAuth:     true,
			wantSource:   "config",
		},
		{
			name:         "not authenticated",
			targetName:   "vercel",
			providerType: "vercel",
			wantAuth:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean env
			os.Unsetenv("VERCEL_TOKEN")
			os.Unsetenv("CONVEX_DEPLOY_KEY")

			if tt.envVar != "" && tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			status := CheckAuth(tt.targetName, tt.providerType, tt.config)

			if status.Authenticated != tt.wantAuth {
				t.Errorf("Authenticated = %v, want %v", status.Authenticated, tt.wantAuth)
			}
			if tt.wantAuth && status.Source != tt.wantSource {
				t.Errorf("Source = %q, want %q", status.Source, tt.wantSource)
			}
			if status.TargetName != tt.targetName {
				t.Errorf("TargetName = %q, want %q", status.TargetName, tt.targetName)
			}
		})
	}
}
