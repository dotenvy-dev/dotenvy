package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromKeys_StrongPrefixes(t *testing.T) {
	tests := []struct {
		key      string
		provider string
	}{
		{"VERCEL_TOKEN", "vercel"},
		{"VERCEL_PROJECT_ID", "vercel"},
		{"CONVEX_DEPLOY_KEY", "convex"},
		{"RAILWAY_TOKEN", "railway"},
		{"RENDER_API_KEY", "render"},
		{"SUPABASE_URL", "supabase"},
		{"SUPABASE_ANON_KEY", "supabase"},
		{"NETLIFY_TOKEN", "netlify"},
		{"FLY_API_TOKEN", "flyio"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			r := FromKeys([]string{tt.key})
			if len(r.Matches) != 1 {
				t.Fatalf("expected 1 match, got %d", len(r.Matches))
			}
			if r.Matches[0].ProviderName != tt.provider {
				t.Errorf("expected provider %q, got %q", tt.provider, r.Matches[0].ProviderName)
			}
			if r.Matches[0].Confidence != "strong" {
				t.Errorf("expected strong confidence, got %q", r.Matches[0].Confidence)
			}
		})
	}
}

func TestFromKeys_SpecificBeatsGeneric(t *testing.T) {
	// NEXT_PUBLIC_CONVEX_URL should match convex (exact), not vercel (weak NEXT_PUBLIC_)
	r := FromKeys([]string{"NEXT_PUBLIC_CONVEX_URL"})
	if len(r.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(r.Matches))
	}
	if r.Matches[0].ProviderName != "convex" {
		t.Errorf("expected convex, got %q", r.Matches[0].ProviderName)
	}
	if r.Matches[0].Confidence != "strong" {
		t.Errorf("expected strong, got %q", r.Matches[0].Confidence)
	}
}

func TestFromKeys_SupabaseSDKConvention(t *testing.T) {
	// NEXT_PUBLIC_SUPABASE_URL should match supabase (strong), not vercel (weak)
	r := FromKeys([]string{"NEXT_PUBLIC_SUPABASE_URL"})
	if len(r.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(r.Matches))
	}
	if r.Matches[0].ProviderName != "supabase" {
		t.Errorf("expected supabase, got %q", r.Matches[0].ProviderName)
	}
	if r.Matches[0].Confidence != "strong" {
		t.Errorf("expected strong, got %q", r.Matches[0].Confidence)
	}
}

func TestFromKeys_WeakMatch(t *testing.T) {
	r := FromKeys([]string{"NEXT_PUBLIC_API_URL"})
	if len(r.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(r.Matches))
	}
	if r.Matches[0].ProviderName != "vercel" {
		t.Errorf("expected vercel, got %q", r.Matches[0].ProviderName)
	}
	if r.Matches[0].Confidence != "weak" {
		t.Errorf("expected weak, got %q", r.Matches[0].Confidence)
	}
}

func TestFromKeys_NoMatch(t *testing.T) {
	keys := []string{"DATABASE_URL", "API_KEY", "SECRET_TOKEN", "MY_VAR"}
	r := FromKeys(keys)
	if len(r.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(r.Matches))
	}
	if len(r.AllKeys) != 4 {
		t.Errorf("expected 4 AllKeys, got %d", len(r.AllKeys))
	}
	if len(r.Providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(r.Providers))
	}
}

func TestFromKeys_Empty(t *testing.T) {
	r := FromKeys(nil)
	if len(r.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(r.Matches))
	}
	if len(r.Providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(r.Providers))
	}
}

func TestFromKeys_ProviderDeduplication(t *testing.T) {
	keys := []string{"VERCEL_TOKEN", "VERCEL_PROJECT_ID", "VERCEL_ORG_ID"}
	r := FromKeys(keys)
	if len(r.Matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(r.Matches))
	}
	if len(r.Providers) != 1 {
		t.Errorf("expected 1 deduplicated provider, got %d: %v", len(r.Providers), r.Providers)
	}
	if r.Providers[0] != "vercel" {
		t.Errorf("expected vercel, got %q", r.Providers[0])
	}
}

func TestFromKeys_MultipleProviders(t *testing.T) {
	keys := []string{"VERCEL_TOKEN", "SUPABASE_URL", "DATABASE_URL", "CONVEX_DEPLOY_KEY"}
	r := FromKeys(keys)
	if len(r.Matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(r.Matches))
	}
	if len(r.Providers) != 3 {
		t.Errorf("expected 3 providers, got %d: %v", len(r.Providers), r.Providers)
	}
	if len(r.AllKeys) != 4 {
		t.Errorf("expected 4 AllKeys, got %d", len(r.AllKeys))
	}
}

func TestFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.test")
	content := "VERCEL_TOKEN=abc\nSUPABASE_URL=https://foo.supabase.co\nDATABASE_URL=postgres://localhost/db\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.SourceFile != path {
		t.Errorf("expected SourceFile %q, got %q", path, r.SourceFile)
	}
	if len(r.AllKeys) != 3 {
		t.Errorf("expected 3 AllKeys, got %d: %v", len(r.AllKeys), r.AllKeys)
	}
	if len(r.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(r.Matches))
	}
	if len(r.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d: %v", len(r.Providers), r.Providers)
	}
}

func TestFindEnvFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// No files → empty string
	if got := FindEnvFile(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// Create .env.test — should be found (second priority)
	os.WriteFile(filepath.Join(dir, ".env.test"), []byte("X=1"), 0644)
	if got := FindEnvFile(); got != ".env.test" {
		t.Errorf("expected .env.test, got %q", got)
	}

	// Create .env — should win (first priority)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("Y=2"), 0644)
	if got := FindEnvFile(); got != ".env" {
		t.Errorf("expected .env, got %q", got)
	}
}
