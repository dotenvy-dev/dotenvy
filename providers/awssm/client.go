package awssm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// smAPI defines the Secrets Manager operations used by the client
type smAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
}

// client wraps the Secrets Manager API
type client struct {
	api        smAPI
	secretName string
}

// newClient creates a new Secrets Manager client wrapper
func newClient(api smAPI, secretName string) *client {
	return &client{api: api, secretName: secretName}
}

// Validate checks that credentials work and the secret exists (or can be created)
func (c *client) Validate(ctx context.Context) error {
	_, err := c.api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(c.secretName),
	})
	if err != nil {
		// If secret doesn't exist, try to create it with empty JSON
		var notFound *smtypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return c.createSecret(ctx, "{}")
		}
		return fmt.Errorf("aws-secretsmanager: failed to validate: %w", err)
	}
	return nil
}

// GetJSON retrieves and parses the secret's JSON value
func (c *client) GetJSON(ctx context.Context) (map[string]string, error) {
	output, err := c.api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(c.secretName),
	})
	if err != nil {
		var notFound *smtypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("aws-secretsmanager: failed to get secret: %w", err)
	}

	secretString := aws.ToString(output.SecretString)
	if secretString == "" {
		return make(map[string]string), nil
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(secretString), &data); err != nil {
		return nil, fmt.Errorf("aws-secretsmanager: failed to parse secret JSON: %w", err)
	}

	return data, nil
}

// PutJSON writes the map as a JSON string to the secret
func (c *client) PutJSON(ctx context.Context, data map[string]string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("aws-secretsmanager: failed to marshal JSON: %w", err)
	}

	_, err = c.api.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(c.secretName),
		SecretString: aws.String(string(jsonBytes)),
	})
	if err != nil {
		// If secret doesn't exist, create it
		var notFound *smtypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return c.createSecret(ctx, string(jsonBytes))
		}
		return fmt.Errorf("aws-secretsmanager: failed to put secret value: %w", err)
	}

	return nil
}

// createSecret creates a new secret with the given string value
func (c *client) createSecret(ctx context.Context, value string) error {
	_, err := c.api.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(c.secretName),
		SecretString: aws.String(value),
	})
	if err != nil {
		return fmt.Errorf("aws-secretsmanager: failed to create secret: %w", err)
	}
	return nil
}
