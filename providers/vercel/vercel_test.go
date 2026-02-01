package vercel

import (
	"testing"
)

func TestNew_ValidConfig(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "tok123",
		"project":         "my-app",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if prov.Name() != "vercel" {
		t.Errorf("Name() = %q, want 'vercel'", prov.Name())
	}

	if prov.DisplayName() != "Vercel" {
		t.Errorf("DisplayName() = %q, want 'Vercel'", prov.DisplayName())
	}
}

func TestNew_MissingToken(t *testing.T) {
	config := map[string]any{
		"project": "my-app",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing token")
	}
}

func TestNew_MissingProject(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "tok123",
	}

	_, err := New(config)
	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestNew_TokenFromConfig(t *testing.T) {
	// Token can also come from config directly (not just _resolved_token)
	config := map[string]any{
		"token":   "tok456",
		"project": "my-app",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.project != "my-app" {
		t.Errorf("project = %q, want 'my-app'", p.project)
	}
}

func TestNew_WithTeamID(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "tok123",
		"project":         "my-app",
		"team_id":         "team_abc",
	}

	prov, err := New(config)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	p := prov.(*Provider)
	if p.client.teamID != "team_abc" {
		t.Errorf("teamID = %q, want 'team_abc'", p.client.teamID)
	}
}

func TestProvider_Environments(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "tok123",
		"project":         "my-app",
	}

	prov, _ := New(config)
	envs := prov.Environments()

	expected := []string{"development", "preview", "production"}
	if len(envs) != len(expected) {
		t.Fatalf("Environments() = %v, want %v", envs, expected)
	}

	for i, env := range expected {
		if envs[i] != env {
			t.Errorf("Environments()[%d] = %q, want %q", i, envs[i], env)
		}
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "tok123",
		"project":         "my-app",
	}

	prov, _ := New(config)
	mapping := prov.DefaultMapping()

	expected := map[string]string{
		"development": "test",
		"preview":     "test",
		"production":  "live",
	}

	for k, v := range expected {
		if mapping[k] != v {
			t.Errorf("DefaultMapping()[%q] = %q, want %q", k, mapping[k], v)
		}
	}
}

func TestContainsTarget(t *testing.T) {
	tests := []struct {
		targets []string
		target  string
		want    bool
	}{
		{[]string{"development", "preview"}, "development", true},
		{[]string{"development", "preview"}, "production", false},
		{[]string{}, "development", false},
		{nil, "development", false},
	}

	for _, tt := range tests {
		got := containsTarget(tt.targets, tt.target)
		if got != tt.want {
			t.Errorf("containsTarget(%v, %q) = %v, want %v", tt.targets, tt.target, got, tt.want)
		}
	}
}
