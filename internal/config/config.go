package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile = "dotenvy.yaml"
	CurrentVersion    = 2
)

// Config represents the dotenvy.yaml configuration (schema only, no values)
type Config struct {
	Version int                   `yaml:"version"`
	APIKey  string                `yaml:"api_key,omitempty"`
	APIURL  string                `yaml:"api_url,omitempty"`
	Secrets []string              `yaml:"secrets"`           // Just names, no values
	Targets map[string]*TargetDef `yaml:"targets,omitempty"`
}

// TargetDef represents a target definition in the config file
type TargetDef struct {
	Type       string            `yaml:"type"`
	Project    string            `yaml:"project,omitempty"`
	Deployment string            `yaml:"deployment,omitempty"`
	ProjectID  string            `yaml:"project_id,omitempty"`
	ServiceID  string            `yaml:"service_id,omitempty"`
	ProjectRef string            `yaml:"project_ref,omitempty"`
	AccountID  string            `yaml:"account_id,omitempty"`
	AppName    string            `yaml:"app_name,omitempty"`
	SiteID     string            `yaml:"site_id,omitempty"`
	Path       string            `yaml:"path,omitempty"`        // For dotenv targets
	Region     string            `yaml:"region,omitempty"`      // AWS region
	Prefix     string            `yaml:"prefix,omitempty"`      // Key prefix (SSM path or GCP prefix)
	Profile    string            `yaml:"profile,omitempty"`     // AWS profile name
	SecretName string            `yaml:"secret_name,omitempty"` // AWS Secrets Manager secret name
	Mapping    map[string]string `yaml:"mapping"`
	Include    []string          `yaml:"include,omitempty,flow"` // Glob patterns
	Exclude    []string          `yaml:"exclude,omitempty,flow"` // Glob patterns
	Token      string            `yaml:"token,omitempty"`
	DeployKey  string            `yaml:"deploy_key,omitempty"`
}

// Load reads and parses the config file
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigFile
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Version == 0 {
		cfg.Version = CurrentVersion
	}

	return &cfg, nil
}

// Save writes the config to a file
func Save(cfg *Config, path string) error {
	if path == "" {
		path = DefaultConfigFile
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Exists checks if a config file exists
func Exists(path string) bool {
	if path == "" {
		path = DefaultConfigFile
	}
	_, err := os.Stat(path)
	return err == nil
}

// GetSecretNames returns the list of secret names
func (c *Config) GetSecretNames() []string {
	return c.Secrets
}

// GetTargets converts config targets to model targets
func (c *Config) GetTargets() []model.Target {
	targets := make([]model.Target, 0, len(c.Targets))
	for name, def := range c.Targets {
		t := model.Target{
			Name:    name,
			Type:    def.Type,
			Mapping: def.Mapping,
			Config:  make(map[string]any),
			Secrets: model.SecretsFilter{
				Include: def.Include,
				Exclude: def.Exclude,
			},
		}

		// Copy provider-specific config
		if def.Project != "" {
			t.Config["project"] = def.Project
		}
		if def.Deployment != "" {
			t.Config["deployment"] = def.Deployment
		}
		if def.ProjectID != "" {
			t.Config["project_id"] = def.ProjectID
		}
		if def.ServiceID != "" {
			t.Config["service_id"] = def.ServiceID
		}
		if def.ProjectRef != "" {
			t.Config["project_ref"] = def.ProjectRef
		}
		if def.AccountID != "" {
			t.Config["account_id"] = def.AccountID
		}
		if def.AppName != "" {
			t.Config["app_name"] = def.AppName
		}
		if def.SiteID != "" {
			t.Config["site_id"] = def.SiteID
		}
		if def.Path != "" {
			t.Config["path"] = def.Path
		}
		if def.Region != "" {
			t.Config["region"] = def.Region
		}
		if def.Prefix != "" {
			t.Config["prefix"] = def.Prefix
		}
		if def.Profile != "" {
			t.Config["profile"] = def.Profile
		}
		if def.SecretName != "" {
			t.Config["secret_name"] = def.SecretName
		}
		if def.Token != "" {
			t.Config["token"] = def.Token
		}
		if def.DeployKey != "" {
			t.Config["deploy_key"] = def.DeployKey
		}

		targets = append(targets, t)
	}
	return targets
}

// GetTarget returns a specific target by name
func (c *Config) GetTarget(name string) (*model.Target, bool) {
	def, ok := c.Targets[name]
	if !ok {
		return nil, false
	}

	t := &model.Target{
		Name:    name,
		Type:    def.Type,
		Mapping: def.Mapping,
		Config:  make(map[string]any),
		Secrets: model.SecretsFilter{
			Include: def.Include,
			Exclude: def.Exclude,
		},
	}

	if def.Project != "" {
		t.Config["project"] = def.Project
	}
	if def.Deployment != "" {
		t.Config["deployment"] = def.Deployment
	}
	if def.ProjectID != "" {
		t.Config["project_id"] = def.ProjectID
	}
	if def.ServiceID != "" {
		t.Config["service_id"] = def.ServiceID
	}
	if def.ProjectRef != "" {
		t.Config["project_ref"] = def.ProjectRef
	}
	if def.AccountID != "" {
		t.Config["account_id"] = def.AccountID
	}
	if def.AppName != "" {
		t.Config["app_name"] = def.AppName
	}
	if def.SiteID != "" {
		t.Config["site_id"] = def.SiteID
	}
	if def.Path != "" {
		t.Config["path"] = def.Path
	}
	if def.Region != "" {
		t.Config["region"] = def.Region
	}
	if def.Prefix != "" {
		t.Config["prefix"] = def.Prefix
	}
	if def.Profile != "" {
		t.Config["profile"] = def.Profile
	}
	if def.SecretName != "" {
		t.Config["secret_name"] = def.SecretName
	}
	if def.Token != "" {
		t.Config["token"] = def.Token
	}
	if def.DeployKey != "" {
		t.Config["deploy_key"] = def.DeployKey
	}

	return t, true
}

// HasSecret checks if a secret name is in the schema
func (c *Config) HasSecret(name string) bool {
	for _, s := range c.Secrets {
		if s == name {
			return true
		}
	}
	return false
}

// AddSecret adds a secret name to the schema
func (c *Config) AddSecret(name string) {
	if !c.HasSecret(name) {
		c.Secrets = append(c.Secrets, name)
	}
}

// AddTarget adds a target to the config
func (c *Config) AddTarget(name string, def *TargetDef) {
	if c.Targets == nil {
		c.Targets = make(map[string]*TargetDef)
	}
	c.Targets[name] = def
}

// NewConfig creates a new empty config
func NewConfig() *Config {
	return &Config{
		Version: CurrentVersion,
		Secrets: []string{},
		Targets: make(map[string]*TargetDef),
	}
}
