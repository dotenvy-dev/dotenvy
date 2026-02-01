package model

import "testing"

func TestTargetDiff_HasChanges(t *testing.T) {
	tests := []struct {
		name  string
		diffs []SecretDiff
		want  bool
	}{
		{
			name:  "empty diffs",
			diffs: nil,
			want:  false,
		},
		{
			name: "only unchanged",
			diffs: []SecretDiff{
				{Name: "KEY1", Type: DiffUnchanged},
				{Name: "KEY2", Type: DiffUnchanged},
			},
			want: false,
		},
		{
			name: "has add",
			diffs: []SecretDiff{
				{Name: "KEY1", Type: DiffUnchanged},
				{Name: "KEY2", Type: DiffAdd},
			},
			want: true,
		},
		{
			name: "has change",
			diffs: []SecretDiff{
				{Name: "KEY1", Type: DiffChange},
			},
			want: true,
		},
		{
			name: "has remove",
			diffs: []SecretDiff{
				{Name: "KEY1", Type: DiffRemove},
			},
			want: true,
		},
		{
			name: "has unknown",
			diffs: []SecretDiff{
				{Name: "KEY1", Type: DiffUnchanged},
				{Name: "KEY2", Type: DiffUnknown},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := TargetDiff{Diffs: tt.diffs}
			got := td.HasChanges()
			if got != tt.want {
				t.Errorf("HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTargetDiff_CountByType(t *testing.T) {
	td := TargetDiff{
		Diffs: []SecretDiff{
			{Name: "KEY1", Type: DiffAdd},
			{Name: "KEY2", Type: DiffAdd},
			{Name: "KEY3", Type: DiffChange},
			{Name: "KEY4", Type: DiffUnchanged},
			{Name: "KEY5", Type: DiffUnchanged},
			{Name: "KEY6", Type: DiffUnchanged},
			{Name: "KEY7", Type: DiffUnknown},
			{Name: "KEY8", Type: DiffUnknown},
		},
	}

	counts := td.CountByType()

	if counts[DiffAdd] != 2 {
		t.Errorf("DiffAdd count = %d, want 2", counts[DiffAdd])
	}
	if counts[DiffChange] != 1 {
		t.Errorf("DiffChange count = %d, want 1", counts[DiffChange])
	}
	if counts[DiffUnchanged] != 3 {
		t.Errorf("DiffUnchanged count = %d, want 3", counts[DiffUnchanged])
	}
	if counts[DiffRemove] != 0 {
		t.Errorf("DiffRemove count = %d, want 0", counts[DiffRemove])
	}
	if counts[DiffUnknown] != 2 {
		t.Errorf("DiffUnknown count = %d, want 2", counts[DiffUnknown])
	}
}

func TestTargetDiff_GroupByEnvironment(t *testing.T) {
	td := TargetDiff{
		Diffs: []SecretDiff{
			{Name: "KEY1", Environment: "development", Type: DiffAdd},
			{Name: "KEY2", Environment: "development", Type: DiffChange},
			{Name: "KEY3", Environment: "production", Type: DiffAdd},
			{Name: "KEY4", Environment: "production", Type: DiffUnchanged},
		},
	}

	grouped := td.GroupByEnvironment()

	if len(grouped) != 2 {
		t.Errorf("expected 2 groups, got %d", len(grouped))
	}

	if len(grouped["development"]) != 2 {
		t.Errorf("development group should have 2 items, got %d", len(grouped["development"]))
	}

	if len(grouped["production"]) != 2 {
		t.Errorf("production group should have 2 items, got %d", len(grouped["production"]))
	}
}

func TestTargetSyncResult_Counts(t *testing.T) {
	tsr := TargetSyncResult{
		TargetName: "vercel",
		Results: []SyncResult{
			{SecretName: "KEY1", Success: true},
			{SecretName: "KEY2", Success: true},
			{SecretName: "KEY3", Success: false, Error: nil},
			{SecretName: "KEY4", Success: true},
			{SecretName: "KEY5", Success: false, Error: nil},
		},
	}

	if tsr.SuccessCount() != 3 {
		t.Errorf("SuccessCount() = %d, want 3", tsr.SuccessCount())
	}

	if tsr.FailureCount() != 2 {
		t.Errorf("FailureCount() = %d, want 2", tsr.FailureCount())
	}
}

func TestDiffType_Values(t *testing.T) {
	// Ensure diff type constants have expected values
	if DiffAdd != "add" {
		t.Errorf("DiffAdd = %q, want 'add'", DiffAdd)
	}
	if DiffRemove != "remove" {
		t.Errorf("DiffRemove = %q, want 'remove'", DiffRemove)
	}
	if DiffChange != "change" {
		t.Errorf("DiffChange = %q, want 'change'", DiffChange)
	}
	if DiffUnchanged != "unchanged" {
		t.Errorf("DiffUnchanged = %q, want 'unchanged'", DiffUnchanged)
	}
	if DiffUnknown != "unknown" {
		t.Errorf("DiffUnknown = %q, want 'unknown'", DiffUnknown)
	}
}
