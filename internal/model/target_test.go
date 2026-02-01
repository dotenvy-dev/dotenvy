package model

import (
	"reflect"
	"sort"
	"testing"
)

func TestTarget_GetProject(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{
			name:   "project field",
			config: map[string]any{"project": "my-app"},
			want:   "my-app",
		},
		{
			name:   "deployment field",
			config: map[string]any{"deployment": "my-deploy"},
			want:   "my-deploy",
		},
		{
			name:   "project takes precedence",
			config: map[string]any{"project": "proj", "deployment": "deploy"},
			want:   "proj",
		},
		{
			name:   "empty config",
			config: map[string]any{},
			want:   "",
		},
		{
			name:   "nil config",
			config: nil,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgt := Target{Config: tt.config}
			got := tgt.GetProject()
			if got != tt.want {
				t.Errorf("GetProject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTarget_GetToken(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{
			name:   "token field",
			config: map[string]any{"token": "tok123"},
			want:   "tok123",
		},
		{
			name:   "deploy_key field",
			config: map[string]any{"deploy_key": "key456"},
			want:   "key456",
		},
		{
			name:   "token takes precedence",
			config: map[string]any{"token": "tok", "deploy_key": "key"},
			want:   "tok",
		},
		{
			name:   "empty",
			config: map[string]any{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgt := Target{Config: tt.config}
			got := tgt.GetToken()
			if got != tt.want {
				t.Errorf("GetToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTarget_LocalEnvironments(t *testing.T) {
	tgt := Target{
		Mapping: map[string]string{
			"development": "test",
			"preview":     "test",
			"production":  "live",
		},
	}

	envs := tgt.LocalEnvironments()
	sort.Strings(envs)

	// Should have unique values: test, live
	if len(envs) != 2 {
		t.Errorf("expected 2 unique envs, got %d: %v", len(envs), envs)
	}

	expected := []string{"live", "test"}
	if !reflect.DeepEqual(envs, expected) {
		t.Errorf("LocalEnvironments() = %v, want %v", envs, expected)
	}
}

func TestTarget_RemoteEnvironments(t *testing.T) {
	tgt := Target{
		Mapping: map[string]string{
			"development": "test",
			"preview":     "test",
			"production":  "live",
		},
	}

	envs := tgt.RemoteEnvironments()
	sort.Strings(envs)

	if len(envs) != 3 {
		t.Errorf("expected 3 remote envs, got %d", len(envs))
	}

	expected := []string{"development", "preview", "production"}
	if !reflect.DeepEqual(envs, expected) {
		t.Errorf("RemoteEnvironments() = %v, want %v", envs, expected)
	}
}

func TestTarget_MapToRemote(t *testing.T) {
	tgt := Target{
		Mapping: map[string]string{
			"development": "test",
			"preview":     "test",
			"production":  "live",
		},
	}

	tests := []struct {
		local string
		want  []string
	}{
		{"test", []string{"development", "preview"}},
		{"live", []string{"production"}},
		{"staging", nil}, // no mapping
	}

	for _, tt := range tests {
		t.Run(tt.local, func(t *testing.T) {
			got := tgt.MapToRemote(tt.local)
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MapToRemote(%q) = %v, want %v", tt.local, got, tt.want)
			}
		})
	}
}

func TestTarget_MapToLocal(t *testing.T) {
	tgt := Target{
		Mapping: map[string]string{
			"development": "test",
			"preview":     "test",
			"production":  "live",
			"default":     "test",
		},
	}

	tests := []struct {
		remote string
		want   string
	}{
		{"development", "test"},
		{"preview", "test"},
		{"production", "live"},
		{"unknown", "test"}, // falls back to default
	}

	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			got := tgt.MapToLocal(tt.remote)
			if got != tt.want {
				t.Errorf("MapToLocal(%q) = %q, want %q", tt.remote, got, tt.want)
			}
		})
	}
}

func TestTarget_MapToLocal_NoDefault(t *testing.T) {
	tgt := Target{
		Mapping: map[string]string{
			"production": "live",
		},
	}

	got := tgt.MapToLocal("unknown")
	if got != "" {
		t.Errorf("MapToLocal(unknown) should return empty string without default, got %q", got)
	}
}
