package provider

import (
	"fmt"
	"sync"
)

var (
	registry = make(map[string]ProviderInfo)
	mu       sync.RWMutex
)

// Register adds a provider to the registry
// This is typically called from provider init() functions
func Register(info ProviderInfo) {
	mu.Lock()
	defer mu.Unlock()
	registry[info.Name] = info
}

// Get retrieves a provider info by name
func Get(name string) (ProviderInfo, bool) {
	mu.RLock()
	defer mu.RUnlock()
	info, ok := registry[name]
	return info, ok
}

// List returns all registered provider names
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Create instantiates a provider by name with the given config
func Create(name string, config map[string]any) (SyncTarget, error) {
	info, ok := Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return info.Factory(config)
}

// IsWriteOnly returns true if the provider cannot read back secret values
func IsWriteOnly(providerType string) bool {
	info, ok := Get(providerType)
	if !ok {
		return false
	}
	return info.WriteOnly
}

// EnvVarFor returns the environment variable name for a provider type
func EnvVarFor(providerType string) string {
	info, ok := Get(providerType)
	if !ok {
		return ""
	}
	return info.EnvVar
}
