package sync

import (
	"testing"

	"github.com/dotenvy-dev/dotenvy/internal/model"
)

func TestShouldSyncSecret(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		target     model.Target
		want       bool
	}{
		{
			name:       "no filters - syncs all",
			secretName: "KEY1",
			target:     model.Target{Name: "vercel"},
			want:       true,
		},
		{
			name:       "target with include pattern - match",
			secretName: "STRIPE_KEY",
			target: model.Target{
				Name:    "vercel",
				Secrets: model.SecretsFilter{Include: []string{"STRIPE_*"}},
			},
			want: true,
		},
		{
			name:       "target with include pattern - no match",
			secretName: "DATABASE_URL",
			target: model.Target{
				Name:    "vercel",
				Secrets: model.SecretsFilter{Include: []string{"STRIPE_*"}},
			},
			want: false,
		},
		{
			name:       "target with exclude pattern - excluded",
			secretName: "CONVEX_URL",
			target: model.Target{
				Name:    "vercel",
				Secrets: model.SecretsFilter{Exclude: []string{"CONVEX_*"}},
			},
			want: false,
		},
		{
			name:       "target with exclude pattern - not excluded",
			secretName: "API_KEY",
			target: model.Target{
				Name:    "vercel",
				Secrets: model.SecretsFilter{Exclude: []string{"CONVEX_*"}},
			},
			want: true,
		},
		{
			name:       "include and exclude - exclude takes precedence",
			secretName: "STRIPE_SECRET",
			target: model.Target{
				Name: "vercel",
				Secrets: model.SecretsFilter{
					Include: []string{"STRIPE_*"},
					Exclude: []string{"*_SECRET"},
				},
			},
			want: false,
		},
		{
			name:       "multiple include patterns - one matches",
			secretName: "API_KEY",
			target: model.Target{
				Name:    "vercel",
				Secrets: model.SecretsFilter{Include: []string{"STRIPE_*", "API_*"}},
			},
			want: true,
		},
		{
			name:       "multiple exclude patterns - one matches",
			secretName: "TEST_SECRET",
			target: model.Target{
				Name:    "vercel",
				Secrets: model.SecretsFilter{Exclude: []string{"DEBUG_*", "*_SECRET"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldSyncSecret(tt.secretName, tt.target)
			if got != tt.want {
				t.Errorf("ShouldSyncSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSecretNames(t *testing.T) {
	names := []string{
		"STRIPE_KEY",
		"STRIPE_SECRET",
		"CONVEX_URL",
		"DATABASE_URL",
	}

	target := model.Target{
		Name:    "vercel",
		Secrets: model.SecretsFilter{Exclude: []string{"CONVEX_*"}},
	}

	filtered := FilterSecretNames(names, target)

	// Should include: STRIPE_KEY, STRIPE_SECRET, DATABASE_URL
	// Should exclude: CONVEX_URL (excluded by pattern)
	if len(filtered) != 3 {
		t.Errorf("expected 3 filtered secrets, got %d: %v", len(filtered), filtered)
	}

	nameSet := make(map[string]bool)
	for _, s := range filtered {
		nameSet[s] = true
	}

	if !nameSet["STRIPE_KEY"] || !nameSet["STRIPE_SECRET"] || !nameSet["DATABASE_URL"] {
		t.Error("missing expected secrets")
	}
	if nameSet["CONVEX_URL"] {
		t.Error("CONVEX_URL should be filtered out")
	}
}

func TestFilterSecretNames_WithInclude(t *testing.T) {
	names := []string{
		"STRIPE_KEY",
		"API_KEY",
		"DATABASE_URL",
	}

	target := model.Target{
		Name:    "vercel",
		Secrets: model.SecretsFilter{Include: []string{"*_KEY"}},
	}

	filtered := FilterSecretNames(names, target)

	// Should include: STRIPE_KEY, API_KEY
	// Should exclude: DATABASE_URL (doesn't match include)
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered secrets, got %d: %v", len(filtered), filtered)
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		patterns []string
		want     bool
	}{
		{"exact match", "API_KEY", []string{"API_KEY"}, true},
		{"wildcard suffix", "STRIPE_KEY", []string{"STRIPE_*"}, true},
		{"wildcard prefix", "MY_SECRET", []string{"*_SECRET"}, true},
		{"no match", "DATABASE_URL", []string{"STRIPE_*", "API_*"}, false},
		{"multiple patterns - one matches", "API_KEY", []string{"STRIPE_*", "API_*"}, true},
		{"empty patterns", "API_KEY", []string{}, false},
		{"question mark wildcard", "KEY1", []string{"KEY?"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAny(tt.input, tt.patterns)
			if got != tt.want {
				t.Errorf("matchesAny(%q, %v) = %v, want %v", tt.input, tt.patterns, got, tt.want)
			}
		})
	}
}
