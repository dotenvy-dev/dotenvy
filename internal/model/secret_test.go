package model

import "testing"

func TestSecret_Name(t *testing.T) {
	s := Secret{Name: "API_KEY"}
	if s.Name != "API_KEY" {
		t.Errorf("Name = %q, want 'API_KEY'", s.Name)
	}
}

func TestSecretSet_Get(t *testing.T) {
	set := &SecretSet{
		Secrets: map[string]string{
			"API_KEY":      "test_value",
			"DATABASE_URL": "postgres://...",
		},
	}

	tests := []struct {
		name   string
		want   string
		wantOk bool
	}{
		{"API_KEY", "test_value", true},
		{"DATABASE_URL", "postgres://...", true},
		{"NONEXISTENT", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := set.Secrets[tt.name]
			if ok != tt.wantOk {
				t.Errorf("Get(%q) ok = %v, want %v", tt.name, ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("Get(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
