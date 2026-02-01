package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

// Shared mock provider state for testing
var sharedMockSecrets = make(map[string]map[string]string)

// clearMockSecrets resets the shared mock state between tests
func clearMockSecrets() {
	sharedMockSecrets = make(map[string]map[string]string)
}

// addMockSecret adds a secret to the shared mock state
func addMockSecret(env, name, value string) {
	if sharedMockSecrets[env] == nil {
		sharedMockSecrets[env] = make(map[string]string)
	}
	sharedMockSecrets[env][name] = value
}

// mockProvider for testing sync engine
type mockProvider struct {
	name     string
	setErr   error
	delErr   error
	setCalls []setCall
	delCalls []delCall
}

type setCall struct {
	name, value, env string
}

type delCall struct {
	name, env string
}

func newMockProvider(name string) *mockProvider {
	return &mockProvider{name: name}
}

func (m *mockProvider) Name() string        { return m.name }
func (m *mockProvider) DisplayName() string { return "Mock " + m.name }
func (m *mockProvider) Environments() []string {
	return []string{"development", "production"}
}
func (m *mockProvider) DefaultMapping() map[string]string {
	return map[string]string{"development": "test", "production": "live"}
}
func (m *mockProvider) Validate(ctx context.Context) error { return nil }

func (m *mockProvider) List(ctx context.Context, env string) ([]model.SecretValue, error) {
	var result []model.SecretValue
	if envSecrets, ok := sharedMockSecrets[env]; ok {
		for name, value := range envSecrets {
			result = append(result, model.SecretValue{
				Name:        name,
				Value:       value,
				Environment: env,
			})
		}
	}
	return result, nil
}

func (m *mockProvider) Set(ctx context.Context, name, value, env string) error {
	m.setCalls = append(m.setCalls, setCall{name, value, env})
	if m.setErr != nil {
		return m.setErr
	}
	if sharedMockSecrets[env] == nil {
		sharedMockSecrets[env] = make(map[string]string)
	}
	sharedMockSecrets[env][name] = value
	return nil
}

func (m *mockProvider) Delete(ctx context.Context, name, env string) error {
	m.delCalls = append(m.delCalls, delCall{name, env})
	if m.delErr != nil {
		return m.delErr
	}
	if sharedMockSecrets[env] != nil {
		delete(sharedMockSecrets[env], name)
	}
	return nil
}

func init() {
	// Register mock provider for tests
	provider.Register(provider.ProviderInfo{
		Name:        "mock",
		DisplayName: "Mock Provider",
		Factory: func(config map[string]any) (provider.SyncTarget, error) {
			return newMockProvider("mock"), nil
		},
		EnvVar: "", // No env var requirement - allows config-based auth
	})
}

// mockSource provides secret values for testing
type mockSource struct {
	values map[string]string
}

func newMockSource(values map[string]string) *mockSource {
	return &mockSource{values: values}
}

func (m *mockSource) Get(name string) string {
	return m.values[name]
}

func (m *mockSource) GetAll(names []string) map[string]string {
	result := make(map[string]string)
	for _, name := range names {
		if val, ok := m.values[name]; ok {
			result[name] = val
		}
	}
	return result
}

func (m *mockSource) Name() string {
	return "mock source"
}

// Ensure mockSource implements source.Source
var _ source.Source = (*mockSource)(nil)

func TestEngine_Preview_NewSecrets(t *testing.T) {
	clearMockSecrets() // Reset state
	engine := NewEngine()
	ctx := context.Background()

	secretNames := []string{"API_KEY"}
	src := newMockSource(map[string]string{
		"API_KEY": "test_value",
	})

	target := model.Target{
		Name: "test-target",
		Type: "mock",
		Mapping: map[string]string{
			"development": "test",
		},
		Config: map[string]any{"token": "test"},
	}

	diff, err := engine.Preview(ctx, secretNames, src, target, "development")
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if diff.TargetName != "test-target" {
		t.Errorf("TargetName = %q, want 'test-target'", diff.TargetName)
	}

	// Should have 1 diff (new secret)
	if len(diff.Diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff.Diffs))
	}

	if diff.Diffs[0].Type != model.DiffAdd {
		t.Errorf("expected DiffAdd, got %s", diff.Diffs[0].Type)
	}
}

func TestEngine_Preview_UnchangedSecrets(t *testing.T) {
	clearMockSecrets() // Reset state
	engine := NewEngine()
	ctx := context.Background()

	secretNames := []string{"API_KEY"}
	src := newMockSource(map[string]string{
		"API_KEY": "same_value",
	})

	target := model.Target{
		Name:    "test-target",
		Type:    "mock",
		Mapping: map[string]string{"development": "test"},
		Config:  map[string]any{"token": "test"},
	}

	// Pre-populate the shared mock state
	addMockSecret("development", "API_KEY", "same_value")

	diff, err := engine.Preview(ctx, secretNames, src, target, "development")
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if len(diff.Diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff.Diffs))
	}

	if diff.Diffs[0].Type != model.DiffUnchanged {
		t.Errorf("expected DiffUnchanged, got %s", diff.Diffs[0].Type)
	}

	if diff.HasChanges() {
		t.Error("HasChanges() should be false for unchanged secrets")
	}
}

func TestEngine_Preview_ChangedSecrets(t *testing.T) {
	clearMockSecrets() // Reset state
	engine := NewEngine()
	ctx := context.Background()

	secretNames := []string{"API_KEY"}
	src := newMockSource(map[string]string{
		"API_KEY": "new_value",
	})

	target := model.Target{
		Name:    "test-target",
		Type:    "mock",
		Mapping: map[string]string{"development": "test"},
		Config:  map[string]any{"token": "test"},
	}

	// Pre-populate with different value
	addMockSecret("development", "API_KEY", "old_value")

	diff, err := engine.Preview(ctx, secretNames, src, target, "development")
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if len(diff.Diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff.Diffs))
	}

	d := diff.Diffs[0]
	if d.Type != model.DiffChange {
		t.Errorf("expected DiffChange, got %s", d.Type)
	}
	if d.OldValue != "old_value" {
		t.Errorf("OldValue = %q, want 'old_value'", d.OldValue)
	}
	if d.NewValue != "new_value" {
		t.Errorf("NewValue = %q, want 'new_value'", d.NewValue)
	}
}

func TestEngine_Sync_DryRun(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	secretNames := []string{"KEY1"}
	src := newMockSource(map[string]string{
		"KEY1": "val1",
	})

	target := model.Target{
		Name:    "dryrun-target",
		Type:    "mock",
		Mapping: map[string]string{"test": "test"},
		Config:  map[string]any{"token": "test_token"},
	}

	result, err := engine.Sync(ctx, secretNames, src, target, "test", SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Dry run should return results
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestEngine_CheckAuth(t *testing.T) {
	engine := NewEngine()

	target := model.Target{
		Name:   "t1",
		Type:   "vercel",
		Config: map[string]any{"token": "tok1"},
	}

	status := engine.CheckAuth(target)
	// With token in config, should be authenticated
	if !status.Authenticated {
		t.Error("should be authenticated with token in config")
	}
}

func TestSyncTarget_SetError(t *testing.T) {
	// Test that Set errors are properly reported
	errorProv := newMockProvider("error-test")
	errorProv.setErr = errors.New("set failed")

	ctx := context.Background()
	err := errorProv.Set(ctx, "KEY", "val", "test")
	if err == nil {
		t.Error("expected error from Set")
	}

	if err.Error() != "set failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFilterSecretNames_IncludeExclude(t *testing.T) {
	target := model.Target{
		Name: "test",
		Type: "vercel",
		Secrets: model.SecretsFilter{
			Include: []string{"API_*"},
			Exclude: []string{"*_SECRET"},
		},
	}

	names := []string{"API_KEY", "API_SECRET", "DATABASE_URL", "OTHER_SECRET"}
	filtered := FilterSecretNames(names, target)

	// API_KEY matches include, doesn't match exclude
	// API_SECRET matches include but also matches exclude
	// DATABASE_URL doesn't match include
	// OTHER_SECRET doesn't match include

	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered name, got %d: %v", len(filtered), filtered)
	}

	if filtered[0] != "API_KEY" {
		t.Errorf("expected API_KEY, got %s", filtered[0])
	}
}
