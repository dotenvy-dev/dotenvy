package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		wantSecrets int
		wantTargets int
	}{
		{
			name: "valid config with secrets list",
			content: `
version: 2
secrets:
  - API_KEY
  - DATABASE_URL
targets:
  vercel:
    type: vercel
    project: my-app
    mapping:
      production: live
`,
			wantSecrets: 2,
			wantTargets: 1,
		},
		{
			name: "empty config",
			content: `
version: 2
`,
			wantSecrets: 0,
			wantTargets: 0,
		},
		{
			name:    "invalid yaml",
			content: `not: valid: yaml: [`,
			wantErr: true,
		},
		{
			name: "multiple secrets and targets",
			content: `
version: 2
secrets:
  - KEY1
  - KEY2
  - KEY3
targets:
  t1:
    type: vercel
    project: p1
    mapping:
      dev: test
  t2:
    type: convex
    deployment: d1
    mapping:
      default: test
`,
			wantSecrets: 3,
			wantTargets: 2,
		},
		{
			name: "railway target with project_id and service_id",
			content: `
version: 2
secrets:
  - SECRET_KEY
targets:
  railway:
    type: railway
    project_id: "8df3b1d6-2317-4400-b267-56c4a42eed06"
    service_id: "4bd252dc-c4ac-4c2e-a52f-051804292035"
    mapping:
      production: live
      staging: test
`,
			wantSecrets: 1,
			wantTargets: 1,
		},
		{
			name: "render target with service_id",
			content: `
version: 2
secrets:
  - API_KEY
targets:
  render:
    type: render
    service_id: "srv-abc123def456"
    mapping:
      default: test
`,
			wantSecrets: 1,
			wantTargets: 1,
		},
		{
			name: "supabase target with project_ref",
			content: `
version: 2
secrets:
  - DB_PASSWORD
targets:
  supabase:
    type: supabase
    project_ref: "abcdefghijklmnop"
    mapping:
      default: test
`,
			wantSecrets: 1,
			wantTargets: 1,
		},
		{
			name: "netlify target with account_id and site_id",
			content: `
version: 2
secrets:
  - API_KEY
targets:
  netlify:
    type: netlify
    account_id: "my-team-slug"
    site_id: "abc123-def456"
    mapping:
      production: live
      deploy-preview: test
      branch-deploy: test
      dev: test
`,
			wantSecrets: 1,
			wantTargets: 1,
		},
		{
			name: "flyio target with app_name",
			content: `
version: 2
secrets:
  - SECRET_KEY
targets:
  flyio:
    type: flyio
    app_name: "my-app-staging"
    mapping:
      default: test
`,
			wantSecrets: 1,
			wantTargets: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "dotenvy.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(cfgPath)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(cfg.Secrets) != tt.wantSecrets {
				t.Errorf("got %d secrets, want %d", len(cfg.Secrets), tt.wantSecrets)
			}
			if len(cfg.Targets) != tt.wantTargets {
				t.Errorf("got %d targets, want %d", len(cfg.Targets), tt.wantTargets)
			}
		})
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/dotenvy.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSave(t *testing.T) {
	cfg := NewConfig()
	cfg.AddSecret("API_KEY")
	cfg.AddSecret("DATABASE_URL")
	cfg.AddTarget("vercel", &TargetDef{
		Type:    "vercel",
		Project: "my-app",
		Mapping: map[string]string{"production": "live"},
	})

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "dotenvy.yaml")

	if err := Save(cfg, cfgPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(loaded.Secrets))
	}
	if !loaded.HasSecret("API_KEY") {
		t.Error("missing API_KEY secret")
	}
	if !loaded.HasSecret("DATABASE_URL") {
		t.Error("missing DATABASE_URL secret")
	}
	if loaded.Targets["vercel"].Project != "my-app" {
		t.Errorf("target project mismatch")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	// File doesn't exist
	if Exists(filepath.Join(tmpDir, "dotenvy.yaml")) {
		t.Error("Exists should return false for nonexistent file")
	}

	// Create file
	cfgPath := filepath.Join(tmpDir, "dotenvy.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 2"), 0644); err != nil {
		t.Fatal(err)
	}

	if !Exists(cfgPath) {
		t.Error("Exists should return true for existing file")
	}
}

func TestGetSecretNames(t *testing.T) {
	cfg := &Config{
		Secrets: []string{"KEY1", "KEY2", "KEY3"},
	}

	names := cfg.GetSecretNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(names))
	}

	expected := map[string]bool{"KEY1": true, "KEY2": true, "KEY3": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected secret name: %s", name)
		}
	}
}

func TestGetTargets(t *testing.T) {
	cfg := &Config{
		Targets: map[string]*TargetDef{
			"vercel": {
				Type:    "vercel",
				Project: "my-app",
				Mapping: map[string]string{"production": "live"},
				Token:   "tok123",
			},
			"convex": {
				Type:       "convex",
				Deployment: "my-deploy",
				Mapping:    map[string]string{"default": "test"},
				DeployKey:  "key456",
			},
			"railway": {
				Type:      "railway",
				ProjectID: "rail-proj-id",
				ServiceID: "rail-svc-id",
				Mapping:   map[string]string{"production": "live", "staging": "test"},
			},
			"render": {
				Type:      "render",
				ServiceID: "srv-abc123",
				Mapping:   map[string]string{"default": "test"},
			},
			"supabase": {
				Type:       "supabase",
				ProjectRef: "abcdefghijklmnop",
				Mapping:    map[string]string{"default": "test"},
			},
			"netlify": {
				Type:      "netlify",
				AccountID: "my-team-slug",
				SiteID:    "abc123-def456",
				Mapping:   map[string]string{"production": "live", "deploy-preview": "test"},
			},
			"flyio": {
				Type:    "flyio",
				AppName: "my-app-staging",
				Mapping: map[string]string{"default": "test"},
			},
		},
	}

	targets := cfg.GetTargets()
	if len(targets) != 7 {
		t.Fatalf("expected 7 targets, got %d", len(targets))
	}

	for _, tgt := range targets {
		if tgt.Name == "vercel" {
			if tgt.Config["project"] != "my-app" {
				t.Error("vercel project mismatch")
			}
			if tgt.Config["token"] != "tok123" {
				t.Error("vercel token mismatch")
			}
		}
		if tgt.Name == "convex" {
			if tgt.Config["deployment"] != "my-deploy" {
				t.Error("convex deployment mismatch")
			}
			if tgt.Config["deploy_key"] != "key456" {
				t.Error("convex deploy_key mismatch")
			}
		}
		if tgt.Name == "railway" {
			if tgt.Config["project_id"] != "rail-proj-id" {
				t.Error("railway project_id mismatch")
			}
			if tgt.Config["service_id"] != "rail-svc-id" {
				t.Error("railway service_id mismatch")
			}
		}
		if tgt.Name == "render" {
			if tgt.Config["service_id"] != "srv-abc123" {
				t.Error("render service_id mismatch")
			}
		}
		if tgt.Name == "supabase" {
			if tgt.Config["project_ref"] != "abcdefghijklmnop" {
				t.Error("supabase project_ref mismatch")
			}
		}
		if tgt.Name == "netlify" {
			if tgt.Config["account_id"] != "my-team-slug" {
				t.Error("netlify account_id mismatch")
			}
			if tgt.Config["site_id"] != "abc123-def456" {
				t.Error("netlify site_id mismatch")
			}
		}
		if tgt.Name == "flyio" {
			if tgt.Config["app_name"] != "my-app-staging" {
				t.Error("flyio app_name mismatch")
			}
		}
	}
}

func TestAddSecret(t *testing.T) {
	cfg := NewConfig()

	cfg.AddSecret("API_KEY")
	if len(cfg.Secrets) != 1 {
		t.Errorf("expected 1 secret, got %d", len(cfg.Secrets))
	}
	if !cfg.HasSecret("API_KEY") {
		t.Error("API_KEY should be in secrets")
	}

	// Adding duplicate should not add again
	cfg.AddSecret("API_KEY")
	if len(cfg.Secrets) != 1 {
		t.Errorf("expected 1 secret after duplicate add, got %d", len(cfg.Secrets))
	}

	cfg.AddSecret("DATABASE_URL")
	if len(cfg.Secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(cfg.Secrets))
	}
}

func TestHasSecret(t *testing.T) {
	cfg := &Config{
		Secrets: []string{"KEY1", "KEY2"},
	}

	if !cfg.HasSecret("KEY1") {
		t.Error("HasSecret should return true for KEY1")
	}
	if !cfg.HasSecret("KEY2") {
		t.Error("HasSecret should return true for KEY2")
	}
	if cfg.HasSecret("KEY3") {
		t.Error("HasSecret should return false for KEY3")
	}
}

func TestGetTarget(t *testing.T) {
	cfg := &Config{
		Targets: map[string]*TargetDef{
			"vercel": {
				Type:    "vercel",
				Project: "my-app",
				Mapping: map[string]string{"production": "live"},
			},
		},
	}

	// Found
	target, ok := cfg.GetTarget("vercel")
	if !ok {
		t.Error("GetTarget should return true for vercel")
	}
	if target.Name != "vercel" {
		t.Error("target name mismatch")
	}
	if target.Type != "vercel" {
		t.Error("target type mismatch")
	}

	// Not found
	_, ok = cfg.GetTarget("nonexistent")
	if ok {
		t.Error("GetTarget should return false for nonexistent")
	}
}
