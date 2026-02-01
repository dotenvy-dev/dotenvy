package model

// DiffType represents the type of change
type DiffType string

const (
	DiffAdd       DiffType = "add"
	DiffRemove    DiffType = "remove"
	DiffChange    DiffType = "change"
	DiffUnchanged DiffType = "unchanged"
	DiffUnknown   DiffType = "unknown"
)

// SecretDiff represents a difference for a single secret
type SecretDiff struct {
	Name        string
	Type        DiffType
	OldValue    string
	NewValue    string
	Environment string // Remote environment name
	Sensitive   bool
}

// TargetDiff represents all differences for a target
type TargetDiff struct {
	TargetName string
	TargetType string
	Project    string
	Diffs      []SecretDiff
}

// HasChanges returns true if there are any non-unchanged diffs
func (td TargetDiff) HasChanges() bool {
	for _, d := range td.Diffs {
		if d.Type != DiffUnchanged {
			return true
		}
	}
	return false
}

// CountByType returns the count of diffs by type
func (td TargetDiff) CountByType() map[DiffType]int {
	counts := make(map[DiffType]int)
	for _, d := range td.Diffs {
		counts[d.Type]++
	}
	return counts
}

// GroupByEnvironment groups diffs by their environment
func (td TargetDiff) GroupByEnvironment() map[string][]SecretDiff {
	grouped := make(map[string][]SecretDiff)
	for _, d := range td.Diffs {
		grouped[d.Environment] = append(grouped[d.Environment], d)
	}
	return grouped
}

// SyncResult represents the result of syncing a single secret
type SyncResult struct {
	SecretName  string
	Environment string
	Success     bool
	Error       error
}

// TargetSyncResult represents the result of syncing to a target
type TargetSyncResult struct {
	TargetName string
	Results    []SyncResult
}

// SuccessCount returns the number of successful syncs
func (tsr TargetSyncResult) SuccessCount() int {
	count := 0
	for _, r := range tsr.Results {
		if r.Success {
			count++
		}
	}
	return count
}

// FailureCount returns the number of failed syncs
func (tsr TargetSyncResult) FailureCount() int {
	count := 0
	for _, r := range tsr.Results {
		if !r.Success {
			count++
		}
	}
	return count
}
