package awsssm

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// mockSSMAPI implements ssmAPI for testing
type mockSSMAPI struct {
	params         map[string]string
	describeErr    error
	getByPathErr   error
	putErr         error
	deleteErr      error
	deletedParams  []string
}

func newMockSSMAPI() *mockSSMAPI {
	return &mockSSMAPI{
		params: make(map[string]string),
	}
}

func (m *mockSSMAPI) DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	if m.describeErr != nil {
		return nil, m.describeErr
	}
	return &ssm.DescribeParametersOutput{}, nil
}

func (m *mockSSMAPI) GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	if m.getByPathErr != nil {
		return nil, m.getByPathErr
	}

	prefix := aws.ToString(params.Path)
	var parameters []types.Parameter
	for name, value := range m.params {
		parameters = append(parameters, types.Parameter{
			Name:  aws.String(prefix + name),
			Value: aws.String(value),
		})
	}

	return &ssm.GetParametersByPathOutput{
		Parameters: parameters,
	}, nil
}

func (m *mockSSMAPI) PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	// Store without prefix (the caller adds prefix)
	m.params[aws.ToString(params.Name)] = aws.ToString(params.Value)
	return &ssm.PutParameterOutput{}, nil
}

func (m *mockSSMAPI) DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	name := aws.ToString(params.Name)
	m.deletedParams = append(m.deletedParams, name)
	delete(m.params, name)
	return &ssm.DeleteParameterOutput{}, nil
}

func TestNew_ValidConfig(t *testing.T) {
	// We can't call New() directly because it creates a real AWS client,
	// but we can test the provider interface with a mock.
	mock := newMockSSMAPI()
	p := &Provider{
		client: newClient(mock, "/myapp/dev/"),
		region: "us-east-1",
		prefix: "/myapp/dev/",
	}

	if p.Name() != "aws-ssm" {
		t.Errorf("Name() = %q, want 'aws-ssm'", p.Name())
	}
	if p.DisplayName() != "AWS SSM Parameter Store" {
		t.Errorf("DisplayName() = %q, want 'AWS SSM Parameter Store'", p.DisplayName())
	}
}

func TestNew_MissingRegion(t *testing.T) {
	_, err := New(map[string]any{})
	if err == nil {
		t.Error("expected error for missing region")
	}
}

func TestProvider_Environments(t *testing.T) {
	mock := newMockSSMAPI()
	p := &Provider{client: newClient(mock, "/")}

	envs := p.Environments()
	if len(envs) != 1 || envs[0] != "default" {
		t.Errorf("Environments() = %v, want ['default']", envs)
	}
}

func TestProvider_DefaultMapping(t *testing.T) {
	mock := newMockSSMAPI()
	p := &Provider{client: newClient(mock, "/")}

	mapping := p.DefaultMapping()
	if mapping["default"] != "test" {
		t.Errorf("DefaultMapping()[default] = %q, want 'test'", mapping["default"])
	}
}

func TestProvider_Validate(t *testing.T) {
	mock := newMockSSMAPI()
	p := &Provider{client: newClient(mock, "/")}

	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestProvider_Validate_Error(t *testing.T) {
	mock := newMockSSMAPI()
	mock.describeErr = fmt.Errorf("access denied")
	p := &Provider{client: newClient(mock, "/")}

	if err := p.Validate(context.Background()); err == nil {
		t.Error("expected error from Validate()")
	}
}

func TestProvider_List(t *testing.T) {
	mock := newMockSSMAPI()
	mock.params["DB_URL"] = "postgres://localhost"
	mock.params["API_KEY"] = "sk_test_123"

	p := &Provider{
		client: newClient(mock, "/myapp/"),
		prefix: "/myapp/",
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

func TestProvider_Set(t *testing.T) {
	mock := newMockSSMAPI()
	p := &Provider{
		client: newClient(mock, "/myapp/"),
		prefix: "/myapp/",
	}

	err := p.Set(context.Background(), "NEW_KEY", "new_value", "default")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// The mock stores with the full prefixed name
	if mock.params["/myapp/NEW_KEY"] != "new_value" {
		t.Errorf("parameter not stored correctly, got params = %v", mock.params)
	}
}

func TestProvider_Delete(t *testing.T) {
	mock := newMockSSMAPI()
	p := &Provider{
		client: newClient(mock, "/myapp/"),
		prefix: "/myapp/",
	}

	err := p.Delete(context.Background(), "OLD_KEY", "default")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if len(mock.deletedParams) != 1 || mock.deletedParams[0] != "/myapp/OLD_KEY" {
		t.Errorf("deleted params = %v, want ['/myapp/OLD_KEY']", mock.deletedParams)
	}
}

func TestProvider_List_Error(t *testing.T) {
	mock := newMockSSMAPI()
	mock.getByPathErr = fmt.Errorf("access denied")
	p := &Provider{
		client: newClient(mock, "/"),
		prefix: "/",
	}

	_, err := p.List(context.Background(), "default")
	if err == nil {
		t.Error("expected error from List()")
	}
}

func TestProvider_Set_Error(t *testing.T) {
	mock := newMockSSMAPI()
	mock.putErr = fmt.Errorf("access denied")
	p := &Provider{
		client: newClient(mock, "/"),
		prefix: "/",
	}

	err := p.Set(context.Background(), "KEY", "val", "default")
	if err == nil {
		t.Error("expected error from Set()")
	}
}

func TestProvider_Delete_Error(t *testing.T) {
	mock := newMockSSMAPI()
	mock.deleteErr = fmt.Errorf("access denied")
	p := &Provider{
		client: newClient(mock, "/"),
		prefix: "/",
	}

	err := p.Delete(context.Background(), "KEY", "default")
	if err == nil {
		t.Error("expected error from Delete()")
	}
}
