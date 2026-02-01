package source

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Source provides secret values for syncing
type Source interface {
	// Get returns the value for a secret name, or empty string if not found
	Get(name string) string
	// GetAll returns all secrets that match the given names
	GetAll(names []string) map[string]string
	// Name returns a description of the source
	Name() string
}

// EnvSource reads secrets from environment variables
type EnvSource struct{}

func NewEnvSource() *EnvSource {
	return &EnvSource{}
}

func (e *EnvSource) Get(name string) string {
	return os.Getenv(name)
}

func (e *EnvSource) GetAll(names []string) map[string]string {
	result := make(map[string]string)
	for _, name := range names {
		if val := os.Getenv(name); val != "" {
			result[name] = val
		}
	}
	return result
}

func (e *EnvSource) Name() string {
	return "environment"
}

// FileSource reads secrets from a dotenv file
type FileSource struct {
	path    string
	secrets map[string]string
	loaded  bool
}

func NewFileSource(path string) *FileSource {
	return &FileSource{
		path:    path,
		secrets: make(map[string]string),
	}
}

func (f *FileSource) load() error {
	if f.loaded {
		return nil
	}

	file, err := os.Open(f.path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", f.path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove surrounding quotes
		value = unquote(value)

		f.secrets[key] = value
	}

	f.loaded = true
	return scanner.Err()
}

func (f *FileSource) Get(name string) string {
	if err := f.load(); err != nil {
		return ""
	}
	return f.secrets[name]
}

func (f *FileSource) GetAll(names []string) map[string]string {
	if err := f.load(); err != nil {
		return nil
	}

	result := make(map[string]string)
	for _, name := range names {
		if val, ok := f.secrets[name]; ok {
			result[name] = val
		}
	}
	return result
}

func (f *FileSource) Name() string {
	return f.path
}

// ListAll returns all secrets in the file (not filtered by names)
func (f *FileSource) ListAll() (map[string]string, error) {
	if err := f.load(); err != nil {
		return nil, err
	}
	// Return a copy
	result := make(map[string]string, len(f.secrets))
	for k, v := range f.secrets {
		result[k] = v
	}
	return result, nil
}

// CombinedSource tries multiple sources in order
type CombinedSource struct {
	sources []Source
}

func NewCombinedSource(sources ...Source) *CombinedSource {
	return &CombinedSource{sources: sources}
}

func (c *CombinedSource) Get(name string) string {
	for _, src := range c.sources {
		if val := src.Get(name); val != "" {
			return val
		}
	}
	return ""
}

func (c *CombinedSource) GetAll(names []string) map[string]string {
	result := make(map[string]string)
	// Go in reverse order so earlier sources override later ones
	for i := len(c.sources) - 1; i >= 0; i-- {
		for name, val := range c.sources[i].GetAll(names) {
			result[name] = val
		}
	}
	return result
}

func (c *CombinedSource) Name() string {
	names := make([]string, len(c.sources))
	for i, src := range c.sources {
		names[i] = src.Name()
	}
	return strings.Join(names, " + ")
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// WriteEnvFile writes secrets to a dotenv file
func WriteEnvFile(path string, secrets map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	defer file.Close()

	for name, value := range secrets {
		line := fmt.Sprintf("%s=%s\n", name, quoteIfNeeded(value))
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write to %s: %w", path, err)
		}
	}

	return nil
}

func quoteIfNeeded(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\n\"'\\$") {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		return "\"" + s + "\""
	}
	return s
}
