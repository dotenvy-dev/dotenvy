package convex

import (
	"testing"
)

func TestNew_ValidConfig(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "convex_key_123",
		"deployment":      "my-app-dev",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if prov.Name() != "convex" {
		t.Errorf("Name() = %q, want 'convex'", prov.Name())
	}

	if prov.DisplayName() != "Convex" {
		t.Errorf("DisplayName() = %q, want 'Convex'", prov.DisplayName())
	}
}

func TestNew_MissingDeployKey(t *testing.T) {
	config := map[string]any{
		"deployment": "my-app-dev",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing deploy_key")
	}
}

func TestNew_MissingDeployment(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "convex_key_123",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing deployment")
	}
}

func TestNew_DeployKeyFromConfig(t *testing.T) {
	// deploy_key can come from config directly (not just _resolved_token)
	config := map[string]any{
		"deploy_key": "convex_key_456",
		"deployment": "my-app-dev",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.deployment != "my-app-dev" {
		t.Errorf("deployment = %q, want 'my-app-dev'", p.deployment)
	}
}

func TestProvider_Environments(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "convex_key_123",
		"deployment":      "my-app-dev",
	}

	prov, _ := New(config)
	envs := prov.Environments()

	// Convex only has a single "default" environment per deployment
	if len(envs) != 1 || envs[0] != "default" {
		t.Errorf("Environments() = %v, want ['default']", envs)
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "convex_key_123",
		"deployment":      "my-app-dev",
	}

	prov, _ := New(config)
	mapping := prov.DefaultMapping()

	if mapping["default"] != "test" {
		t.Errorf("DefaultMapping()[default] = %q, want 'test'", mapping["default"])
	}
}
