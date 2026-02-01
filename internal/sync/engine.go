package sync

import (
	"context"
	"fmt"

	"github.com/dotenvy-dev/dotenvy/internal/auth"
	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

// Engine orchestrates the sync process
type Engine struct{}

// NewEngine creates a new sync engine
func NewEngine() *Engine {
	return &Engine{}
}

// SyncOptions configures sync behavior
type SyncOptions struct {
	DryRun      bool
	Environment string          // "test" or "live"
	Progress    ProgressCallback
}

// ProgressCallback is called during sync operations
type ProgressCallback func(event ProgressEvent)

// ProgressEvent represents a sync progress update
type ProgressEvent struct {
	Phase       string // "diff", "sync"
	TargetName  string
	SecretName  string
	Environment string
	Action      string // "add", "change", "unchanged"
	Success     bool
	Error       error
	Message     string
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	TargetName  string
	Environment string
	Added       int
	Changed     int
	Unchanged   int
	Unknown     int
	Failed      int
	Errors      []error
}

// Preview calculates what would change without applying
func (e *Engine) Preview(ctx context.Context, secretNames []string, src source.Source, target model.Target, remoteEnv string) (*model.TargetDiff, error) {
	// Create provider for the target
	prov, err := createProvider(target)
	if err != nil {
		return nil, err
	}

	// Filter secrets for this target
	filteredNames := FilterSecretNames(secretNames, target)

	// Get values from source
	sourceValues := src.GetAll(filteredNames)

	// Get current values from remote
	remoteSecrets, err := prov.List(ctx, remoteEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets from %s: %w", target.Name, err)
	}

	// Check if provider is write-only
	provInfo, _ := provider.Get(target.Type)
	writeOnly := provInfo.WriteOnly

	// Build remote lookup
	remoteMap := make(map[string]string)
	for _, s := range remoteSecrets {
		remoteMap[s.Name] = s.Value
	}

	// Calculate diff
	diff := &model.TargetDiff{
		TargetName: target.Name,
		TargetType: target.Type,
		Project:    target.GetProject(),
	}

	for _, name := range filteredNames {
		localValue, hasLocal := sourceValues[name]
		remoteValue, hasRemote := remoteMap[name]

		var diffType model.DiffType
		if !hasLocal || localValue == "" {
			// No local value - skip (don't delete remote)
			continue
		} else if writeOnly && hasRemote {
			diffType = model.DiffUnknown
		} else if !hasRemote {
			diffType = model.DiffAdd
		} else if localValue != remoteValue {
			diffType = model.DiffChange
		} else {
			diffType = model.DiffUnchanged
		}

		diff.Diffs = append(diff.Diffs, model.SecretDiff{
			Name:        name,
			Type:        diffType,
			OldValue:    remoteValue,
			NewValue:    localValue,
			Environment: remoteEnv,
		})
	}

	return diff, nil
}

// Sync applies changes to a target
func (e *Engine) Sync(ctx context.Context, secretNames []string, src source.Source, target model.Target, remoteEnv string, opts SyncOptions) (*SyncResult, error) {
	result := &SyncResult{
		TargetName:  target.Name,
		Environment: remoteEnv,
	}

	// Calculate diff first
	diff, err := e.Preview(ctx, secretNames, src, target, remoteEnv)
	if err != nil {
		return nil, err
	}

	if opts.DryRun {
		// Just count what would happen
		for _, d := range diff.Diffs {
			switch d.Type {
			case model.DiffAdd:
				result.Added++
			case model.DiffChange:
				result.Changed++
			case model.DiffUnchanged:
				result.Unchanged++
			case model.DiffUnknown:
				result.Unknown++
			}
		}
		return result, nil
	}

	// Create provider
	prov, err := createProvider(target)
	if err != nil {
		return nil, err
	}

	writer, ok := prov.(provider.Writer)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support writing", target.Type)
	}

	// Apply changes
	for _, d := range diff.Diffs {
		if d.Type == model.DiffUnchanged {
			result.Unchanged++
			continue
		}

		if opts.Progress != nil {
			opts.Progress(ProgressEvent{
				Phase:       "sync",
				TargetName:  target.Name,
				SecretName:  d.Name,
				Environment: remoteEnv,
				Action:      string(d.Type),
				Message:     fmt.Sprintf("Syncing %s", d.Name),
			})
		}

		var syncErr error
		switch d.Type {
		case model.DiffAdd, model.DiffChange, model.DiffUnknown:
			syncErr = writer.Set(ctx, d.Name, d.NewValue, remoteEnv)
		}

		if syncErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", d.Name, syncErr))
			if opts.Progress != nil {
				opts.Progress(ProgressEvent{
					Phase:       "sync",
					TargetName:  target.Name,
					SecretName:  d.Name,
					Environment: remoteEnv,
					Success:     false,
					Error:       syncErr,
				})
			}
		} else {
			switch d.Type {
			case model.DiffAdd:
				result.Added++
			case model.DiffUnknown:
				result.Unknown++
			default:
				result.Changed++
			}
			if opts.Progress != nil {
				opts.Progress(ProgressEvent{
					Phase:       "sync",
					TargetName:  target.Name,
					SecretName:  d.Name,
					Environment: remoteEnv,
					Success:     true,
				})
			}
		}
	}

	return result, nil
}

// CheckAuth validates authentication for a target
func (e *Engine) CheckAuth(target model.Target) auth.AuthStatus {
	return auth.CheckAuth(target.Name, target.Type, target.Config)
}

// Pull fetches secrets from a target
func (e *Engine) Pull(ctx context.Context, target model.Target, remoteEnv string) (map[string]string, error) {
	prov, err := createProvider(target)
	if err != nil {
		return nil, err
	}

	secrets, err := prov.List(ctx, remoteEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	result := make(map[string]string)
	for _, s := range secrets {
		result[s.Name] = s.Value
	}

	return result, nil
}

// createProvider creates a provider instance for a target
func createProvider(target model.Target) (provider.SyncTarget, error) {
	// Check if provider uses SDK-based auth (no token needed)
	provInfo, _ := provider.Get(target.Type)
	if provInfo.SdkAuth || provInfo.EnvVar == "" {
		return provider.Create(target.Type, target.Config)
	}

	// Resolve credentials
	creds, err := auth.ResolveCredentials(target.Type, target.Config)
	if err != nil {
		return nil, fmt.Errorf("authentication failed for %s: %w", target.Name, err)
	}

	// Merge credentials into config
	config := make(map[string]any)
	for k, v := range target.Config {
		config[k] = v
	}
	config["_resolved_token"] = creds.Token

	return provider.Create(target.Type, config)
}
