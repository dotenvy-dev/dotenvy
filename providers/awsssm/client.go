package awsssm

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ssmAPI defines the SSM operations used by the client
type ssmAPI interface {
	GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
	DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
}

// client wraps the SSM API for parameter operations
type client struct {
	api    ssmAPI
	prefix string
}

// newClient creates a new SSM client wrapper
func newClient(api ssmAPI, prefix string) *client {
	return &client{api: api, prefix: prefix}
}

// Validate checks that credentials and access are working
func (c *client) Validate(ctx context.Context) error {
	_, err := c.api.DescribeParameters(ctx, &ssm.DescribeParametersInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("aws-ssm: failed to validate credentials: %w", err)
	}
	return nil
}

// ListParameters returns all parameters under the configured prefix
func (c *client) ListParameters(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)
	var nextToken *string

	for {
		output, err := c.api.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
			Path:           aws.String(c.prefix),
			WithDecryption: aws.Bool(true),
			Recursive:      aws.Bool(false),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("aws-ssm: failed to list parameters: %w", err)
		}

		for _, param := range output.Parameters {
			name := aws.ToString(param.Name)
			// Strip prefix to get the bare secret name
			if len(name) > len(c.prefix) {
				name = name[len(c.prefix):]
			}
			result[name] = aws.ToString(param.Value)
		}

		nextToken = output.NextToken
		if nextToken == nil {
			break
		}
	}

	return result, nil
}

// PutParameter creates or updates a parameter
func (c *client) PutParameter(ctx context.Context, name, value string) error {
	_, err := c.api.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(c.prefix + name),
		Value:     aws.String(value),
		Type:      types.ParameterTypeSecureString,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("aws-ssm: failed to put parameter %s: %w", name, err)
	}
	return nil
}

// DeleteParameter removes a parameter
func (c *client) DeleteParameter(ctx context.Context, name string) error {
	_, err := c.api.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: aws.String(c.prefix + name),
	})
	if err != nil {
		return fmt.Errorf("aws-ssm: failed to delete parameter %s: %w", name, err)
	}
	return nil
}
