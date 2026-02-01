package model

// Target represents a sync target (e.g., Vercel project, Convex deployment)
type Target struct {
	Name    string            `yaml:"-"`
	Type    string            `yaml:"type"`
	Mapping map[string]string `yaml:"mapping"` // e.g., development: test, production: live
	Secrets SecretsFilter     `yaml:"secrets,omitempty"`

	// Provider-specific configuration (embedded as raw map)
	Config map[string]any `yaml:",inline"`
}

// SecretsFilter controls which secrets sync to this target
type SecretsFilter struct {
	Include []string `yaml:"include,omitempty,flow"` // Glob patterns
	Exclude []string `yaml:"exclude,omitempty,flow"` // Glob patterns
}

// GetProject returns the project identifier for this target
func (t Target) GetProject() string {
	if p, ok := t.Config["project"].(string); ok {
		return p
	}
	if d, ok := t.Config["deployment"].(string); ok {
		return d
	}
	if s, ok := t.Config["service_id"].(string); ok {
		return s
	}
	if r, ok := t.Config["project_ref"].(string); ok {
		return r
	}
	if a, ok := t.Config["app_name"].(string); ok {
		return a
	}
	if a, ok := t.Config["account_id"].(string); ok {
		return a
	}
	return ""
}

// GetToken returns the token from config if present
func (t Target) GetToken() string {
	if token, ok := t.Config["token"].(string); ok {
		return token
	}
	if key, ok := t.Config["deploy_key"].(string); ok {
		return key
	}
	return ""
}

// LocalEnvironments returns the list of local environment names (test, live)
func (t Target) LocalEnvironments() []string {
	seen := make(map[string]bool)
	var envs []string
	for _, localEnv := range t.Mapping {
		if !seen[localEnv] {
			seen[localEnv] = true
			envs = append(envs, localEnv)
		}
	}
	return envs
}

// RemoteEnvironments returns the list of remote environment names
func (t Target) RemoteEnvironments() []string {
	var envs []string
	for remoteEnv := range t.Mapping {
		envs = append(envs, remoteEnv)
	}
	return envs
}

// MapToRemote converts a local environment (test/live) to remote environment(s)
func (t Target) MapToRemote(localEnv string) []string {
	var remotes []string
	for remote, local := range t.Mapping {
		if local == localEnv {
			remotes = append(remotes, remote)
		}
	}
	return remotes
}

// MapToLocal converts a remote environment to local environment
func (t Target) MapToLocal(remoteEnv string) string {
	if local, ok := t.Mapping[remoteEnv]; ok {
		return local
	}
	// Fallback to "default" mapping if exists
	if local, ok := t.Mapping["default"]; ok {
		return local
	}
	return ""
}
