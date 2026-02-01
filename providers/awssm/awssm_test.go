package awssm

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// mockSMAPI implements smAPI for testing
type mockSMAPI struct {
	secretString string
	exists       bool
	getErr       error
	putErr       error
	createErr    error
	created      bool
	lastPut      string
}

func newMockSMAPI(data map[string]string) *mockSMAPI {
	m := &mockSMAPI{exists: true}
	if data != nil {
		b, _ := json.Marshal(data)
		m.secretString = string(b)
	} else {
		m.exists = false
	}
	return m
}

func (m *mockSMAPI) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if !m.exists {
		return nil, &smtypes.ResourceNotFoundException{Message: aws.String("not found")}
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(m.secretString),
	}, nil
}

func (m *mockSMAPI) PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	m.secretString = aws.ToString(params.SecretString)
	m.lastPut = m.secretString
	return &secretsmanager.PutSecretValueOutput{}, nil
}

func (m *mockSMAPI) CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.exists = true
	m.secretString = aws.ToString(params.SecretString)
	m.created = true
	return &secretsmanager.CreateSecretOutput{}, nil
}

func TestNew_ValidConfig(t *testing.T) {
	mock := newMockSMAPI(map[string]string{})
	p := &Provider{
		client:     newClient(mock, "myapp-dev"),
		region:     "us-east-1",
		secretName: "myapp-dev",
	}

	if p.Name() != "aws-secretsmanager" {
		t.Errorf("Name() = %q, want 'aws-secretsmanager'", p.Name())
	}
	if p.DisplayName() != "AWS Secrets Manager" {
		t.Errorf("DisplayName() = %q, want 'AWS Secrets Manager'", p.DisplayName())
	}
}

func TestNew_MissingRegion(t *testing.T) {
	_, err := New(map[string]any{
		"secret_name": "test",
	})
	if err == nil {
		t.Error("expected error for missing region")
	}
}

func TestNew_MissingSecretName(t *testing.T) {
	_, err := New(map[string]any{
		"region": "us-east-1",
	})
	if err == nil {
		t.Error("expected error for missing secret_name")
	}
}

func TestProvider_Environments(t *testing.T) {
	mock := newMockSMAPI(map[string]string{})
	p := &Provider{client: newClient(mock, "test")}

	envs := p.Environments()
	if len(envs) != 1 || envs[0] != "default" {
		t.Errorf("Environments() = %v, want ['default']", envs)
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	mock := newMockSMAPI(map[string]string{})
	p := &Provider{client: newClient(mock, "test")}

	mapping := p.DefaultMapping()
	if mapping["default"] != "test" {
		t.Errorf("DefaultMapping()[default] = %q, want 'test'", mapping["default"])
	}
}

func TestProvider_Validate_ExistingSecret(t *testing.T) {
	mock := newMockSMAPI(map[string]string{"KEY": "val"})
	p := &Provider{client: newClient(mock, "test")}

	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestProvider_Validate_CreatesSecret(t *testing.T) {
	mock := newMockSMAPI(nil) // secret doesn't exist
	p := &Provider{client: newClient(mock, "test")}

	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	if !mock.created {
		t.Error("expected secret to be created")
	}
}

func TestProvider_List(t *testing.T) {
	mock := newMockSMAPI(map[string]string{
		"DB_URL":  "postgres://localhost",
		"API_KEY": "sk_test_123",
	})
	p := &Provider{client: newClient(mock, "test")}

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

func TestProvider_List_EmptySecret(t *testing.T) {
	mock := newMockSMAPI(nil) // secret doesn't exist
	p := &Provider{client: newClient(mock, "test")}

	secrets, err := p.List(context.Background(), "default")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(secrets) != 0 {
		t.Errorf("List() returned %d secrets, want 0", len(secrets))
	}
}

func TestProvider_Set_NewKey(t *testing.T) {
	mock := newMockSMAPI(map[string]string{
		"EXISTING": "val",
	})
	p := &Provider{client: newClient(mock, "test")}

	err := p.Set(context.Background(), "NEW_KEY", "new_value", "default")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Verify the JSON was updated
	var data map[string]string
	json.Unmarshal([]byte(mock.lastPut), &data)

	if data["EXISTING"] != "val" {
		t.Errorf("EXISTING = %q, want 'val'", data["EXISTING"])
	}
	if data["NEW_KEY"] != "new_value" {
		t.Errorf("NEW_KEY = %q, want 'new_value'", data["NEW_KEY"])
	}
}

func TestProvider_Set_UpdateKey(t *testing.T) {
	mock := newMockSMAPI(map[string]string{
		"KEY": "old_value",
	})
	p := &Provider{client: newClient(mock, "test")}

	err := p.Set(context.Background(), "KEY", "new_value", "default")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	var data map[string]string
	json.Unmarshal([]byte(mock.lastPut), &data)

	if data["KEY"] != "new_value" {
		t.Errorf("KEY = %q, want 'new_value'", data["KEY"])
	}
}

func TestProvider_Delete(t *testing.T) {
	mock := newMockSMAPI(map[string]string{
		"KEY1": "val1",
		"KEY2": "val2",
	})
	p := &Provider{client: newClient(mock, "test")}

	err := p.Delete(context.Background(), "KEY1", "default")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	var data map[string]string
	json.Unmarshal([]byte(mock.lastPut), &data)

	if _, exists := data["KEY1"]; exists {
		t.Error("KEY1 should have been deleted")
	}
	if data["KEY2"] != "val2" {
		t.Errorf("KEY2 = %q, want 'val2'", data["KEY2"])
	}
}

func TestProvider_List_Error(t *testing.T) {
	mock := newMockSMAPI(map[string]string{})
	mock.getErr = fmt.Errorf("access denied")
	p := &Provider{client: newClient(mock, "test")}

	_, err := p.List(context.Background(), "default")
	if err == nil {
		t.Error("expected error from List()")
	}
}

func TestProvider_Set_GetError(t *testing.T) {
	mock := newMockSMAPI(map[string]string{})
	mock.getErr = fmt.Errorf("access denied")
	p := &Provider{client: newClient(mock, "test")}

	err := p.Set(context.Background(), "KEY", "val", "default")
	if err == nil {
		t.Error("expected error from Set()")
	}
}

func TestProvider_Set_PutError(t *testing.T) {
	mock := newMockSMAPI(map[string]string{})
	mock.putErr = fmt.Errorf("access denied")
	p := &Provider{client: newClient(mock, "test")}

	err := p.Set(context.Background(), "KEY", "val", "default")
	if err == nil {
		t.Error("expected error from Set()")
	}
}
