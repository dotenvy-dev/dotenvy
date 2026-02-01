package flyio

import (
	"testing"
)

func TestNew_ValidConfig(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "fly_token_123",
		"app_name":        "my-app-staging",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if prov.Name() != "flyio" {
		t.Errorf("Name() = %q, want 'flyio'", prov.Name())
	}

	if prov.DisplayName() != "Fly.io" {
		t.Errorf("DisplayName() = %q, want 'Fly.io'", prov.DisplayName())
	}
}

func TestNew_MissingToken(t *testing.T) {
	config := map[string]any{
		"app_name": "my-app-staging",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing API token")
	}
}

func TestNew_MissingAppName(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "fly_token_123",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing app_name")
	}
}

func TestNew_TokenFromConfig(t *testing.T) {
	config := map[string]any{
		"token":    "fly_token_456",
		"app_name": "my-app-staging",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.appName != "my-app-staging" {
		t.Errorf("appName = %q, want 'my-app-staging'", p.appName)
	}
}

func TestProvider_Environments(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "fly_token_123",
		"app_name":        "my-app-staging",
	}

	prov, _ := New(config)
	envs := prov.Environments()

	if len(envs) != 1 || envs[0] != "default" {
		t.Errorf("Environments() = %v, want ['default']", envs)
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "fly_token_123",
		"app_name":        "my-app-staging",
	}

	prov, _ := New(config)
	mapping := prov.DefaultMapping()

	if mapping["default"] != "test" {
		t.Errorf("DefaultMapping()[default] = %q, want 'test'", mapping["default"])
	}
}
