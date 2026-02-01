package model

// Secret represents a secret in the schema (name only, no value)
type Secret struct {
	Name string
}

// SecretValue represents a secret with its value (used during sync)
type SecretValue struct {
	Name        string
	Value       string
	Environment string
}

// SecretSet is a collection of secret values
type SecretSet struct {
	Secrets map[string]string // name -> value
}

// NewSecretSet creates a new secret set
func NewSecretSet() *SecretSet {
	return &SecretSet{
		Secrets: make(map[string]string),
	}
}

// Set adds or updates a secret value
func (s *SecretSet) Set(name, value string) {
	s.Secrets[name] = value
}

// Get returns a secret value
func (s *SecretSet) Get(name string) (string, bool) {
	v, ok := s.Secrets[name]
	return v, ok
}

// Names returns all secret names
func (s *SecretSet) Names() []string {
	names := make([]string, 0, len(s.Secrets))
	for name := range s.Secrets {
		names = append(names, name)
	}
	return names
}

// Len returns the number of secrets
func (s *SecretSet) Len() int {
	return len(s.Secrets)
}

// ToSecretValues converts to a slice of SecretValue
func (s *SecretSet) ToSecretValues(env string) []SecretValue {
	values := make([]SecretValue, 0, len(s.Secrets))
	for name, value := range s.Secrets {
		values = append(values, SecretValue{
			Name:        name,
			Value:       value,
			Environment: env,
		})
	}
	return values
}
