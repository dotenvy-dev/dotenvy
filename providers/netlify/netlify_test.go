package netlify

import (
	"testing"
)

func TestNew_ValidConfig(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "nf_token_123",
		"account_id":      "my-team-slug",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if prov.Name() != "netlify" {
		t.Errorf("Name() = %q, want 'netlify'", prov.Name())
	}

	if prov.DisplayName() != "Netlify" {
		t.Errorf("DisplayName() = %q, want 'Netlify'", prov.DisplayName())
	}
}

func TestNew_MissingToken(t *testing.T) {
	config := map[string]any{
		"account_id": "my-team-slug",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing API token")
	}
}

func TestNew_MissingAccountID(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "nf_token_123",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing account_id")
	}
}

func TestNew_TokenFromConfig(t *testing.T) {
	config := map[string]any{
		"token":      "nf_token_456",
		"account_id": "my-team-slug",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.accountID != "my-team-slug" {
		t.Errorf("accountID = %q, want 'my-team-slug'", p.accountID)
	}
}

func TestNew_WithSiteID(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "nf_token_123",
		"account_id":      "my-team-slug",
		"site_id":         "abc123-def456",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.siteID != "abc123-def456" {
		t.Errorf("siteID = %q, want 'abc123-def456'", p.siteID)
	}
}

func TestProvider_Environments(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "nf_token_123",
		"account_id":      "my-team-slug",
	}

	prov, _ := New(config)
	envs := prov.Environments()

	expected := map[string]bool{
		"production":     true,
		"deploy-preview": true,
		"branch-deploy":  true,
		"dev":            true,
	}

	if len(envs) != 4 {
		t.Fatalf("Environments() returned %d envs, want 4", len(envs))
	}

	for _, env := range envs {
		if !expected[env] {
			t.Errorf("unexpected environment: %q", env)
		}
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "nf_token_123",
		"account_id":      "my-team-slug",
	}

	prov, _ := New(config)
	mapping := prov.DefaultMapping()

	if mapping["production"] != "live" {
		t.Errorf("DefaultMapping()[production] = %q, want 'live'", mapping["production"])
	}
	if mapping["deploy-preview"] != "test" {
		t.Errorf("DefaultMapping()[deploy-preview] = %q, want 'test'", mapping["deploy-preview"])
	}
	if mapping["branch-deploy"] != "test" {
		t.Errorf("DefaultMapping()[branch-deploy] = %q, want 'test'", mapping["branch-deploy"])
	}
	if mapping["dev"] != "test" {
		t.Errorf("DefaultMapping()[dev] = %q, want 'test'", mapping["dev"])
	}
}
