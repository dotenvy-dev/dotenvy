package provider

import (
	"context"
	"testing"

	"github.com/dotenvy-dev/dotenvy/internal/model"
)

// Mock provider for testing
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string                              { return m.name }
func (m *mockProvider) DisplayName() string                       { return "Mock " + m.name }
func (m *mockProvider) Environments() []string                    { return []string{"test"} }
func (m *mockProvider) DefaultMapping() map[string]string         { return map[string]string{"test": "test"} }
func (m *mockProvider) Validate(ctx context.Context) error        { return nil }
func (m *mockProvider) List(ctx context.Context, env string) ([]model.SecretValue, error) {
	return nil, nil
}
func (m *mockProvider) Set(ctx context.Context, name, value, env string) error { return nil }
func (m *mockProvider) Delete(ctx context.Context, name, env string) error     { return nil }

func mockFactory(config map[string]any) (SyncTarget, error) {
	name, _ := config["name"].(string)
	return &mockProvider{name: name}, nil
}

func TestRegisterAndGet(t *testing.T) {
	// Register a test provider
	Register(ProviderInfo{
		Name:        "test-provider",
		DisplayName: "Test Provider",
		Factory:     mockFactory,
		EnvVar:      "TEST_TOKEN",
	})

	// Get it back
	info, ok := Get("test-provider")
	if !ok {
		t.Fatal("provider not found after registration")
	}

	if info.Name != "test-provider" {
		t.Errorf("Name = %q, want 'test-provider'", info.Name)
	}
	if info.DisplayName != "Test Provider" {
		t.Errorf("DisplayName = %q, want 'Test Provider'", info.DisplayName)
	}
	if info.EnvVar != "TEST_TOKEN" {
		t.Errorf("EnvVar = %q, want 'TEST_TOKEN'", info.EnvVar)
	}
}

func TestGet_NotFound(t *testing.T) {
	_, ok := Get("nonexistent-provider")
	if ok {
		t.Error("Get should return false for nonexistent provider")
	}
}

func TestList(t *testing.T) {
	// Register some providers
	Register(ProviderInfo{Name: "list-test-1", Factory: mockFactory})
	Register(ProviderInfo{Name: "list-test-2", Factory: mockFactory})

	names := List()

	// Should include our test providers
	found1, found2 := false, false
	for _, name := range names {
		if name == "list-test-1" {
			found1 = true
		}
		if name == "list-test-2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Errorf("List() missing registered providers, got %v", names)
	}
}

func TestCreate(t *testing.T) {
	Register(ProviderInfo{
		Name:    "create-test",
		Factory: mockFactory,
	})

	config := map[string]any{"name": "created"}
	prov, err := Create("create-test", config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if prov.Name() != "created" {
		t.Errorf("created provider Name() = %q, want 'created'", prov.Name())
	}
}

func TestCreate_NotFound(t *testing.T) {
	_, err := Create("nonexistent", nil)
	if err == nil {
		t.Error("Create should fail for nonexistent provider")
	}
}

func TestEnvVarFor(t *testing.T) {
	Register(ProviderInfo{
		Name:   "envvar-test",
		EnvVar: "ENVVAR_TEST_TOKEN",
	})

	envVar := EnvVarFor("envvar-test")
	if envVar != "ENVVAR_TEST_TOKEN" {
		t.Errorf("EnvVarFor() = %q, want 'ENVVAR_TEST_TOKEN'", envVar)
	}

	// Unknown provider
	envVar = EnvVarFor("unknown")
	if envVar != "" {
		t.Errorf("EnvVarFor(unknown) = %q, want empty string", envVar)
	}
}
