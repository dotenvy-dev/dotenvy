package supabase

import (
	"testing"
)

func TestNew_ValidConfig(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "sbp_token_123",
		"project_ref":     "abcdefghijklmnop",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if prov.Name() != "supabase" {
		t.Errorf("Name() = %q, want 'supabase'", prov.Name())
	}

	if prov.DisplayName() != "Supabase" {
		t.Errorf("DisplayName() = %q, want 'Supabase'", prov.DisplayName())
	}
}

func TestNew_MissingToken(t *testing.T) {
	config := map[string]any{
		"project_ref": "abcdefghijklmnop",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing access token")
	}
}

func TestNew_MissingProjectRef(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "sbp_token_123",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing project_ref")
	}
}

func TestNew_TokenFromConfig(t *testing.T) {
	config := map[string]any{
		"token":       "sbp_token_456",
		"project_ref": "abcdefghijklmnop",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.projectRef != "abcdefghijklmnop" {
		t.Errorf("projectRef = %q, want 'abcdefghijklmnop'", p.projectRef)
	}
}

func TestProvider_Environments(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "sbp_token_123",
		"project_ref":     "abcdefghijklmnop",
	}

	prov, _ := New(config)
	envs := prov.Environments()

	if len(envs) != 1 || envs[0] != "default" {
		t.Errorf("Environments() = %v, want ['default']", envs)
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "sbp_token_123",
		"project_ref":     "abcdefghijklmnop",
	}

	prov, _ := New(config)
	mapping := prov.DefaultMapping()

	if mapping["default"] != "test" {
		t.Errorf("DefaultMapping()[default] = %q, want 'test'", mapping["default"])
	}
}
