package render

import (
	"testing"
)

func TestNew_ValidConfig(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "rnd_key_123",
		"service_id":      "srv-abc123def456",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if prov.Name() != "render" {
		t.Errorf("Name() = %q, want 'render'", prov.Name())
	}

	if prov.DisplayName() != "Render" {
		t.Errorf("DisplayName() = %q, want 'Render'", prov.DisplayName())
	}
}

func TestNew_MissingToken(t *testing.T) {
	config := map[string]any{
		"service_id": "srv-abc123def456",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestNew_MissingServiceID(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "rnd_key_123",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing service_id")
	}
}

func TestNew_TokenFromConfig(t *testing.T) {
	config := map[string]any{
		"token":      "rnd_key_456",
		"service_id": "srv-abc123def456",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.serviceID != "srv-abc123def456" {
		t.Errorf("serviceID = %q, want 'srv-abc123def456'", p.serviceID)
	}
}

func TestProvider_Environments(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "rnd_key_123",
		"service_id":      "srv-abc123def456",
	}

	prov, _ := New(config)
	envs := prov.Environments()

	if len(envs) != 1 || envs[0] != "default" {
		t.Errorf("Environments() = %v, want ['default']", envs)
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "rnd_key_123",
		"service_id":      "srv-abc123def456",
	}

	prov, _ := New(config)
	mapping := prov.DefaultMapping()

	if mapping["default"] != "test" {
		t.Errorf("DefaultMapping()[default] = %q, want 'test'", mapping["default"])
	}
}
