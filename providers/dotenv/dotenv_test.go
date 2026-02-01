package dotenv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{
			name:   "default path",
			config: map[string]any{},
			want:   ".env",
		},
		{
			name:   "custom path",
			config: map[string]any{"path": ".env.local"},
			want:   ".env.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := New(tt.config)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			p := prov.(*Provider)
			if p.path != tt.want {
				t.Errorf("path = %q, want %q", p.path, tt.want)
			}
		})
	}
}

func TestProvider_Metadata(t *testing.T) {
	prov, _ := New(map[string]any{})

	if prov.Name() != "dotenv" {
		t.Errorf("Name() = %q, want 'dotenv'", prov.Name())
	}

	if prov.DisplayName() != "Local .env" {
		t.Errorf("DisplayName() = %q, want 'Local .env'", prov.DisplayName())
	}

	envs := prov.Environments()
	if len(envs) != 1 || envs[0] != "local" {
		t.Errorf("Environments() = %v, want ['local']", envs)
	}

	mapping := prov.DefaultMapping()
	if mapping["local"] != "test" {
		t.Errorf("DefaultMapping() = %v, want local->test", mapping)
	}
}

func TestProvider_List(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	content := `# Comment line
API_KEY=my_api_key
DATABASE_URL=postgres://localhost/db
EMPTY_VALUE=
QUOTED_VALUE="hello world"
SINGLE_QUOTED='single quotes'
SPACED_KEY = value_with_spaces
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	prov, _ := New(map[string]any{"path": envFile})
	ctx := context.Background()

	secrets, err := prov.List(ctx, "local")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Build map for easier testing
	secretMap := make(map[string]string)
	for _, s := range secrets {
		secretMap[s.Name] = s.Value
	}

	tests := []struct {
		name  string
		value string
	}{
		{"API_KEY", "my_api_key"},
		{"DATABASE_URL", "postgres://localhost/db"},
		{"EMPTY_VALUE", ""},
		{"QUOTED_VALUE", "hello world"},       // Quotes stripped
		{"SINGLE_QUOTED", "single quotes"},    // Quotes stripped
		{"SPACED_KEY", "value_with_spaces"},   // Spaces trimmed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := secretMap[tt.name]
			if !ok {
				t.Errorf("secret %s not found", tt.name)
				return
			}
			if got != tt.value {
				t.Errorf("%s = %q, want %q", tt.name, got, tt.value)
			}
		})
	}
}

func TestProvider_List_FileNotFound(t *testing.T) {
	prov, _ := New(map[string]any{"path": "/nonexistent/.env"})
	ctx := context.Background()

	secrets, err := prov.List(ctx, "local")
	if err != nil {
		t.Fatalf("List should not error for nonexistent file: %v", err)
	}

	if len(secrets) != 0 {
		t.Errorf("expected empty list, got %d secrets", len(secrets))
	}
}

func TestProvider_Set(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Start with existing content
	initial := "EXISTING_KEY=existing_value\n"
	if err := os.WriteFile(envFile, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	prov, _ := New(map[string]any{"path": envFile})
	ctx := context.Background()

	// Add new key
	if err := prov.Set(ctx, "NEW_KEY", "new_value", "local"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Update existing key
	if err := prov.Set(ctx, "EXISTING_KEY", "updated_value", "local"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read file and verify
	content, _ := os.ReadFile(envFile)
	lines := strings.Split(string(content), "\n")

	foundExisting, foundNew := false, false
	for _, line := range lines {
		if strings.HasPrefix(line, "EXISTING_KEY=") {
			foundExisting = true
			if line != "EXISTING_KEY=updated_value" {
				t.Errorf("EXISTING_KEY not updated: %s", line)
			}
		}
		if strings.HasPrefix(line, "NEW_KEY=") {
			foundNew = true
			if line != "NEW_KEY=new_value" {
				t.Errorf("NEW_KEY wrong value: %s", line)
			}
		}
	}

	if !foundExisting {
		t.Error("EXISTING_KEY not found in file")
	}
	if !foundNew {
		t.Error("NEW_KEY not found in file")
	}
}

func TestProvider_Set_QuotesSpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	prov, _ := New(map[string]any{"path": envFile})
	ctx := context.Background()

	// Value with spaces should be quoted
	if err := prov.Set(ctx, "SPACED", "hello world", "local"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	content, _ := os.ReadFile(envFile)
	if !strings.Contains(string(content), `SPACED="hello world"`) {
		t.Errorf("value with spaces should be quoted: %s", content)
	}
}

func TestProvider_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	initial := "KEEP_ME=keep\nDELETE_ME=delete\nALSO_KEEP=also\n"
	if err := os.WriteFile(envFile, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	prov, _ := New(map[string]any{"path": envFile})
	ctx := context.Background()

	if err := prov.Delete(ctx, "DELETE_ME", "local"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	content, _ := os.ReadFile(envFile)
	contentStr := string(content)

	if strings.Contains(contentStr, "DELETE_ME") {
		t.Error("DELETE_ME should be removed")
	}
	if !strings.Contains(contentStr, "KEEP_ME") {
		t.Error("KEEP_ME should still exist")
	}
	if !strings.Contains(contentStr, "ALSO_KEEP") {
		t.Error("ALSO_KEEP should still exist")
	}
}

func TestProvider_Delete_FileNotFound(t *testing.T) {
	prov, _ := New(map[string]any{"path": "/nonexistent/.env"})
	ctx := context.Background()

	// Should not error for nonexistent file
	err := prov.Delete(ctx, "KEY", "local")
	if err != nil {
		t.Errorf("Delete should not error for nonexistent file: %v", err)
	}
}

func TestProvider_Validate(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Create file
	os.WriteFile(envFile, []byte("KEY=val"), 0644)

	prov, _ := New(map[string]any{"path": envFile})
	ctx := context.Background()

	if err := prov.Validate(ctx); err != nil {
		t.Errorf("Validate should pass for existing file: %v", err)
	}

	// Nonexistent file is also valid (will be created on write)
	prov2, _ := New(map[string]any{"path": filepath.Join(tmpDir, "new.env")})
	if err := prov2.Validate(ctx); err != nil {
		t.Errorf("Validate should pass for nonexistent file: %v", err)
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`"`, `"`},
		{`""`, ""},
		{`''`, ""},
		{`"mixed'`, `"mixed'`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := unquote(tt.input)
			if got != tt.want {
				t.Errorf("unquote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuoteIfNeeded(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "simple", "simple"},
		{"with space", "hello world", `"hello world"`},
		{"empty", "", `""`},
		{"with quote", `has"quote`, `"has\"quote"`},
		{"with backslash", `has\backslash`, `"has\\backslash"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteIfNeeded(tt.input)
			if got != tt.want {
				t.Errorf("quoteIfNeeded(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuoteIfNeeded_SpecialChars(t *testing.T) {
	// Values with tabs, newlines, etc. should be quoted (but not necessarily escaped)
	special := "has\ttab"
	got := quoteIfNeeded(special)
	if got[0] != '"' || got[len(got)-1] != '"' {
		t.Errorf("value with tab should be quoted, got %q", got)
	}

	special = "has\nnewline"
	got = quoteIfNeeded(special)
	if got[0] != '"' || got[len(got)-1] != '"' {
		t.Errorf("value with newline should be quoted, got %q", got)
	}
}
