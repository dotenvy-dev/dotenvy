package railway

import (
	"testing"
)

func TestNew_ValidConfig(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "test-token",
		"project_id":      "8df3b1d6-2317-4400-b267-56c4a42eed06",
	}

	p, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_MissingToken(t *testing.T) {
	config := map[string]any{
		"project_id": "8df3b1d6-2317-4400-b267-56c4a42eed06",
	}

	_, err := New(config)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestNew_MissingProjectID(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "test-token",
	}

	_, err := New(config)
	if err == nil {
		t.Fatal("expected error for missing project_id")
	}
}

func TestNew_TokenFromConfig(t *testing.T) {
	config := map[string]any{
		"token":      "fallback-token",
		"project_id": "8df3b1d6-2317-4400-b267-56c4a42eed06",
	}

	p, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_WithServiceID(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "test-token",
		"project_id":      "8df3b1d6-2317-4400-b267-56c4a42eed06",
		"service_id":      "4bd252dc-c4ac-4c2e-a52f-051804292035",
	}

	p, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prov := p.(*Provider)
	if prov.client.serviceID != "4bd252dc-c4ac-4c2e-a52f-051804292035" {
		t.Errorf("expected service_id to be set, got %q", prov.client.serviceID)
	}
}

func TestNew_WithoutServiceID(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "test-token",
		"project_id":      "8df3b1d6-2317-4400-b267-56c4a42eed06",
	}

	p, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prov := p.(*Provider)
	if prov.client.serviceID != "" {
		t.Errorf("expected empty service_id, got %q", prov.client.serviceID)
	}
}

func TestProvider_Environments(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "test-token",
		"project_id":      "test-project",
	}

	p, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prov := p.(*Provider)
	envs := prov.Environments()
	if len(envs) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(envs))
	}

	expected := map[string]bool{"production": true, "staging": true}
	for _, env := range envs {
		if !expected[env] {
			t.Errorf("unexpected environment: %s", env)
		}
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	config := map[string]any{
		"_resolved_token": "test-token",
		"project_id":      "test-project",
	}

	p, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prov := p.(*Provider)
	mapping := prov.DefaultMapping()

	if mapping["production"] != "live" {
		t.Errorf("expected production->live, got production->%s", mapping["production"])
	}
	if mapping["staging"] != "test" {
		t.Errorf("expected staging->test, got staging->%s", mapping["staging"])
	}
}
