package sync

import (
	"path/filepath"

	"github.com/dotenvy-dev/dotenvy/internal/model"
)

// ShouldSyncSecret determines if a secret should sync to a target based on filters
func ShouldSyncSecret(secretName string, target model.Target) bool {
	// Check target's include patterns (if specified)
	if len(target.Secrets.Include) > 0 && !matchesAny(secretName, target.Secrets.Include) {
		return false
	}

	// Check target's exclude patterns
	if matchesAny(secretName, target.Secrets.Exclude) {
		return false
	}

	return true
}

// FilterSecretNames returns secret names that should sync to a target
func FilterSecretNames(names []string, target model.Target) []string {
	var filtered []string
	for _, name := range names {
		if ShouldSyncSecret(name, target) {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

// matchesAny checks if name matches any of the glob patterns
func matchesAny(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
