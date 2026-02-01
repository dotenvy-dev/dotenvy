package dotenv

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

func init() {
	provider.Register(provider.ProviderInfo{
		Name:        "dotenv",
		DisplayName: "Local .env",
		Factory:     New,
		EnvVar:      "", // No auth needed
	})
}

// Provider implements a local .env file provider
type Provider struct {
	path string
}

// New creates a new dotenv provider
func New(config map[string]any) (provider.SyncTarget, error) {
	path, _ := config["path"].(string)
	if path == "" {
		path = ".env"
	}
	return &Provider{path: path}, nil
}

func (p *Provider) Name() string        { return "dotenv" }
func (p *Provider) DisplayName() string { return "Local .env" }

func (p *Provider) Environments() []string {
	return []string{"local"}
}

func (p *Provider) DefaultMapping() map[string]string {
	return map[string]string{
		"local": "test",
	}
}

func (p *Provider) Validate(ctx context.Context) error {
	// Check if file is readable (or doesn't exist, which is fine)
	if _, err := os.Stat(p.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot access %s: %w", p.path, err)
	}
	return nil
}

func (p *Provider) List(ctx context.Context, environment string) ([]model.SecretValue, error) {
	file, err := os.Open(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Empty list if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open %s: %w", p.path, err)
	}
	defer file.Close()

	var secrets []model.SecretValue
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

		// Remove surrounding quotes if present
		value = unquote(value)

		secrets = append(secrets, model.SecretValue{
			Name:        key,
			Value:       value,
			Environment: environment,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", p.path, err)
	}

	return secrets, nil
}

func (p *Provider) Set(ctx context.Context, name, value, environment string) error {
	// Read existing content
	existing := make(map[string]string)
	var lines []string

	file, err := os.Open(p.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to open %s: %w", p.path, err)
	}
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)

			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if idx := strings.Index(trimmed, "="); idx != -1 {
				key := strings.TrimSpace(trimmed[:idx])
				existing[key] = line
			}
		}
		file.Close()
	}

	// Update or add the value
	newLine := fmt.Sprintf("%s=%s", name, quoteIfNeeded(value))

	if _, exists := existing[name]; exists {
		// Replace existing line
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if idx := strings.Index(trimmed, "="); idx != -1 {
				key := strings.TrimSpace(trimmed[:idx])
				if key == name {
					lines[i] = newLine
					break
				}
			}
		}
	} else {
		// Append new line
		lines = append(lines, newLine)
	}

	// Write back
	return os.WriteFile(p.path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func (p *Provider) Delete(ctx context.Context, name, environment string) error {
	file, err := os.Open(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open %s: %w", p.path, err)
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Keep line unless it's the one to delete
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if idx := strings.Index(trimmed, "="); idx != -1 {
				key := strings.TrimSpace(trimmed[:idx])
				if key == name {
					continue // Skip this line
				}
			}
		}
		lines = append(lines, line)
	}
	file.Close()

	return os.WriteFile(p.path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
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

func quoteIfNeeded(s string) string {
	// Quote if contains spaces, special chars, or is empty
	if s == "" || strings.ContainsAny(s, " \t\n\"'\\$") {
		// Escape existing quotes and backslashes
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		return "\"" + s + "\""
	}
	return s
}
