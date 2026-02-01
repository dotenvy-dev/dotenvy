package gcpsm

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockIterator implements secretIterator for testing
type mockIterator struct {
	secrets []*secretmanagerpb.Secret
	idx     int
	err     error
}

func (m *mockIterator) Next() (*secretmanagerpb.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.idx >= len(m.secrets) {
		return nil, iterator.Done
	}
	s := m.secrets[m.idx]
	m.idx++
	return s, nil
}

// mockSMClient implements smClient for testing
type mockSMClient struct {
	secrets         map[string]string // secretID -> value
	project         string
	listErr         error
	accessErr       error
	createErr       error
	addVersionErr   error
	deleteErr       error
	deletedSecrets  []string
	createdSecrets  []string
}

func newMockSMClient(project string, secrets map[string]string) *mockSMClient {
	if secrets == nil {
		secrets = make(map[string]string)
	}
	return &mockSMClient{
		secrets: secrets,
		project: project,
	}
}

func (m *mockSMClient) ListSecrets(ctx context.Context, req *secretmanagerpb.ListSecretsRequest) secretIterator {
	if m.listErr != nil {
		return &mockIterator{err: m.listErr}
	}

	var secrets []*secretmanagerpb.Secret
	for name := range m.secrets {
		secrets = append(secrets, &secretmanagerpb.Secret{
			Name: fmt.Sprintf("projects/%s/secrets/%s", m.project, name),
		})
	}
	return &mockIterator{secrets: secrets}
}

func (m *mockSMClient) CreateSecret(ctx context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	secretID := req.SecretId
	m.createdSecrets = append(m.createdSecrets, secretID)
	// Don't set a value yet — that happens in AddSecretVersion
	return &secretmanagerpb.Secret{
		Name: fmt.Sprintf("%s/secrets/%s", req.Parent, secretID),
	}, nil
}

func (m *mockSMClient) DeleteSecret(ctx context.Context, req *secretmanagerpb.DeleteSecretRequest) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	parts := strings.Split(req.Name, "/")
	secretID := parts[len(parts)-1]
	m.deletedSecrets = append(m.deletedSecrets, secretID)
	delete(m.secrets, secretID)
	return nil
}

func (m *mockSMClient) AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if m.addVersionErr != nil {
		return nil, m.addVersionErr
	}
	// Extract secret ID from parent (projects/*/secrets/NAME)
	parts := strings.Split(req.Parent, "/")
	secretID := parts[len(parts)-1]

	// Check if secret exists
	if _, exists := m.secrets[secretID]; !exists {
		// Check if it was just created
		justCreated := false
		for _, c := range m.createdSecrets {
			if c == secretID {
				justCreated = true
				break
			}
		}
		if !justCreated {
			return nil, status.Error(codes.NotFound, "secret not found")
		}
	}

	m.secrets[secretID] = string(req.Payload.Data)
	return &secretmanagerpb.SecretVersion{
		Name: req.Parent + "/versions/1",
	}, nil
}

func (m *mockSMClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if m.accessErr != nil {
		return nil, m.accessErr
	}
	// Extract secret name from versions path (projects/*/secrets/NAME/versions/latest)
	parts := strings.Split(req.Name, "/")
	// Find "secrets" index and get the next part
	for i, p := range parts {
		if p == "secrets" && i+1 < len(parts) {
			secretID := parts[i+1]
			if val, ok := m.secrets[secretID]; ok {
				return &secretmanagerpb.AccessSecretVersionResponse{
					Payload: &secretmanagerpb.SecretPayload{
						Data: []byte(val),
					},
				}, nil
			}
			return nil, status.Error(codes.NotFound, "version not found")
		}
	}
	return nil, status.Error(codes.NotFound, "secret not found")
}

func (m *mockSMClient) Close() error {
	return nil
}

func TestNew_MissingProject(t *testing.T) {
	_, err := New(map[string]any{})
	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestProvider_Name(t *testing.T) {
	mock := newMockSMClient("my-project", nil)
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	if p.Name() != "gcp-secret-manager" {
		t.Errorf("Name() = %q, want 'gcp-secret-manager'", p.Name())
	}
	if p.DisplayName() != "GCP Secret Manager" {
		t.Errorf("DisplayName() = %q, want 'GCP Secret Manager'", p.DisplayName())
	}
}

func TestProvider_Environments(t *testing.T) {
	mock := newMockSMClient("my-project", nil)
	p := &Provider{client: newClient(mock, "my-project", "")}

	envs := p.Environments()
	if len(envs) != 1 || envs[0] != "default" {
		t.Errorf("Environments() = %v, want ['default']", envs)
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	mock := newMockSMClient("my-project", nil)
	p := &Provider{client: newClient(mock, "my-project", "")}

	mapping := p.DefaultMapping()
	if mapping["default"] != "test" {
		t.Errorf("DefaultMapping()[default] = %q, want 'test'", mapping["default"])
	}
}

func TestProvider_Validate(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{"SECRET": "val"})
	p := &Provider{client: newClient(mock, "my-project", "")}

	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestProvider_Validate_Empty(t *testing.T) {
	mock := newMockSMClient("my-project", nil)
	p := &Provider{client: newClient(mock, "my-project", "")}

	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestProvider_Validate_Error(t *testing.T) {
	mock := newMockSMClient("my-project", nil)
	mock.listErr = fmt.Errorf("permission denied")
	p := &Provider{client: newClient(mock, "my-project", "")}

	if err := p.Validate(context.Background()); err == nil {
		t.Error("expected error from Validate()")
	}
}

func TestProvider_List(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{
		"DB_URL":  "postgres://localhost",
		"API_KEY": "sk_test_123",
	})
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	secrets, err := p.List(context.Background(), "default")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(secrets) != 2 {
		t.Fatalf("List() returned %d secrets, want 2", len(secrets))
	}

	found := make(map[string]string)
	for _, s := range secrets {
		found[s.Name] = s.Value
	}

	if found["DB_URL"] != "postgres://localhost" {
		t.Errorf("DB_URL = %q, want 'postgres://localhost'", found["DB_URL"])
	}
	if found["API_KEY"] != "sk_test_123" {
		t.Errorf("API_KEY = %q, want 'sk_test_123'", found["API_KEY"])
	}
}

func TestProvider_List_WithPrefix(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{
		"myapp_DB_URL": "postgres://localhost",
		"myapp_API":    "key123",
		"other_SECRET": "should_be_filtered",
	})
	p := &Provider{
		client:  newClient(mock, "my-project", "myapp_"),
		project: "my-project",
		prefix:  "myapp_",
	}

	secrets, err := p.List(context.Background(), "default")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(secrets) != 2 {
		t.Fatalf("List() returned %d secrets, want 2", len(secrets))
	}

	found := make(map[string]string)
	for _, s := range secrets {
		found[s.Name] = s.Value
	}

	if found["DB_URL"] != "postgres://localhost" {
		t.Errorf("DB_URL = %q, want 'postgres://localhost'", found["DB_URL"])
	}
	if found["API"] != "key123" {
		t.Errorf("API = %q, want 'key123'", found["API"])
	}
}

func TestProvider_Set_NewSecret(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{})
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	err := p.Set(context.Background(), "NEW_KEY", "new_value", "default")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if mock.secrets["NEW_KEY"] != "new_value" {
		t.Errorf("secret not stored, got %v", mock.secrets)
	}
}

func TestProvider_Set_ExistingSecret(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{
		"KEY": "old_value",
	})
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	err := p.Set(context.Background(), "KEY", "new_value", "default")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if mock.secrets["KEY"] != "new_value" {
		t.Errorf("KEY = %q, want 'new_value'", mock.secrets["KEY"])
	}
}

func TestProvider_Set_WithPrefix(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{})
	p := &Provider{
		client:  newClient(mock, "my-project", "myapp_"),
		project: "my-project",
		prefix:  "myapp_",
	}

	err := p.Set(context.Background(), "KEY", "value", "default")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if mock.secrets["myapp_KEY"] != "value" {
		t.Errorf("secret not stored with prefix, got %v", mock.secrets)
	}
}

func TestProvider_Delete(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{
		"KEY": "value",
	})
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	err := p.Delete(context.Background(), "KEY", "default")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if len(mock.deletedSecrets) != 1 || mock.deletedSecrets[0] != "KEY" {
		t.Errorf("deleted secrets = %v, want ['KEY']", mock.deletedSecrets)
	}
}

func TestProvider_Delete_NotFound(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{})
	mock.deleteErr = status.Error(codes.NotFound, "not found")
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	// Delete of non-existent secret should not error
	err := p.Delete(context.Background(), "MISSING", "default")
	if err != nil {
		t.Errorf("Delete() error = %v, want nil for not-found", err)
	}
}

func TestProvider_List_Error(t *testing.T) {
	mock := newMockSMClient("my-project", nil)
	mock.listErr = fmt.Errorf("permission denied")
	p := &Provider{client: newClient(mock, "my-project", "")}

	_, err := p.List(context.Background(), "default")
	if err == nil {
		t.Error("expected error from List()")
	}
}

func TestProvider_Set_CreateError(t *testing.T) {
	mock := newMockSMClient("my-project", map[string]string{})
	mock.createErr = fmt.Errorf("quota exceeded")
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	err := p.Set(context.Background(), "NEW_KEY", "value", "default")
	if err == nil {
		t.Error("expected error from Set()")
	}
}

func TestProvider_Delete_Error(t *testing.T) {
	mock := newMockSMClient("my-project", nil)
	mock.deleteErr = fmt.Errorf("permission denied")
	p := &Provider{
		client:  newClient(mock, "my-project", ""),
		project: "my-project",
	}

	err := p.Delete(context.Background(), "KEY", "default")
	if err == nil {
		t.Error("expected error from Delete()")
	}
}
