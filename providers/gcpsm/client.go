package gcpsm

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// smClient defines the GCP Secret Manager operations used by the client
type smClient interface {
	ListSecrets(ctx context.Context, req *secretmanagerpb.ListSecretsRequest) secretIterator
	CreateSecret(ctx context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error)
	DeleteSecret(ctx context.Context, req *secretmanagerpb.DeleteSecretRequest) error
	AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error)
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

// secretIterator abstracts the GCP secret iterator for testing
type secretIterator interface {
	Next() (*secretmanagerpb.Secret, error)
}

// gcpClientWrapper wraps the real GCP Secret Manager client
type gcpClientWrapper struct {
	inner *secretmanager.Client
}

func (w *gcpClientWrapper) ListSecrets(ctx context.Context, req *secretmanagerpb.ListSecretsRequest) secretIterator {
	return w.inner.ListSecrets(ctx, req)
}

func (w *gcpClientWrapper) CreateSecret(ctx context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
	return w.inner.CreateSecret(ctx, req)
}

func (w *gcpClientWrapper) DeleteSecret(ctx context.Context, req *secretmanagerpb.DeleteSecretRequest) error {
	return w.inner.DeleteSecret(ctx, req)
}

func (w *gcpClientWrapper) AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	return w.inner.AddSecretVersion(ctx, req)
}

func (w *gcpClientWrapper) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	return w.inner.AccessSecretVersion(ctx, req)
}

func (w *gcpClientWrapper) Close() error {
	return w.inner.Close()
}

// client wraps the GCP Secret Manager API
type client struct {
	api     smClient
	project string
	prefix  string
}

// newClient creates a new GCP Secret Manager client wrapper
func newClient(api smClient, project, prefix string) *client {
	return &client{api: api, project: project, prefix: prefix}
}

// parent returns the GCP resource parent path
func (c *client) parent() string {
	return fmt.Sprintf("projects/%s", c.project)
}

// secretPath returns the full resource path for a secret
func (c *client) secretPath(name string) string {
	return fmt.Sprintf("projects/%s/secrets/%s%s", c.project, c.prefix, name)
}

// Validate checks that credentials and project access work
func (c *client) Validate(ctx context.Context) error {
	it := c.api.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent:   c.parent(),
		PageSize: 1,
	})
	// Just try to iterate — if credentials/project are bad, this will fail
	_, err := it.Next()
	if err != nil && err != iterator.Done {
		return fmt.Errorf("gcp-secret-manager: failed to validate: %w", err)
	}
	return nil
}

// ListSecrets returns all secrets matching the prefix
func (c *client) ListSecrets(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)

	it := c.api.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent: c.parent(),
	})

	for {
		secret, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcp-secret-manager: failed to list secrets: %w", err)
		}

		// Extract secret name from resource path (projects/*/secrets/NAME)
		parts := strings.Split(secret.Name, "/")
		secretID := parts[len(parts)-1]

		// Filter by prefix
		if c.prefix != "" && !strings.HasPrefix(secretID, c.prefix) {
			continue
		}

		// Strip prefix to get the bare name
		bareName := secretID
		if c.prefix != "" {
			bareName = strings.TrimPrefix(secretID, c.prefix)
		}

		// Access latest version
		resp, err := c.api.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: secret.Name + "/versions/latest",
		})
		if err != nil {
			// Skip secrets with no versions
			if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
				continue
			}
			return nil, fmt.Errorf("gcp-secret-manager: failed to access secret %s: %w", bareName, err)
		}

		result[bareName] = string(resp.Payload.Data)
	}

	return result, nil
}

// SetSecret creates or updates a secret
func (c *client) SetSecret(ctx context.Context, name, value string) error {
	fullName := c.secretPath(name)

	// Try to add a new version (assumes secret exists)
	_, err := c.api.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: fullName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(value),
		},
	})
	if err != nil {
		// If secret doesn't exist, create it first
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			_, createErr := c.api.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
				Parent:   c.parent(),
				SecretId: c.prefix + name,
				Secret: &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_Automatic_{
							Automatic: &secretmanagerpb.Replication_Automatic{},
						},
					},
				},
			})
			if createErr != nil {
				return fmt.Errorf("gcp-secret-manager: failed to create secret %s: %w", name, createErr)
			}

			// Now add the version
			_, err = c.api.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
				Parent: fullName,
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte(value),
				},
			})
			if err != nil {
				return fmt.Errorf("gcp-secret-manager: failed to add version for %s: %w", name, err)
			}
			return nil
		}
		return fmt.Errorf("gcp-secret-manager: failed to set secret %s: %w", name, err)
	}

	return nil
}

// DeleteSecret removes a secret
func (c *client) DeleteSecret(ctx context.Context, name string) error {
	err := c.api.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: c.secretPath(name),
	})
	if err != nil {
		// Ignore not-found errors on delete
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil
		}
		return fmt.Errorf("gcp-secret-manager: failed to delete secret %s: %w", name, err)
	}
	return nil
}
