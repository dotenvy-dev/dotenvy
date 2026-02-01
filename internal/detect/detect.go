package detect

import (
	"os"
	"sort"
	"strings"

	"github.com/dotenvy-dev/dotenvy/internal/source"
)

// Match represents a single key-to-provider match.
type Match struct {
	Key          string // e.g. "VERCEL_TOKEN"
	ProviderName string // e.g. "vercel"
	Confidence   string // "strong" or "weak"
	Reason       string // e.g. "Vercel platform variable"
}

// Result holds all detection output from scanning keys.
type Result struct {
	SourceFile string   // Path to the scanned file
	AllKeys    []string // Every key found in the file
	Matches    []Match  // Keys that matched a provider pattern
	Providers  []string // Deduplicated detected provider names
}

// pattern defines a single matching rule.
type pattern struct {
	prefix     string // Exact prefix to match (or full key for exact matches)
	exact      bool   // If true, match the full key exactly
	provider   string
	confidence string
	reason     string
}

// Patterns are ordered longest-prefix-first so specific matches win.
var patterns []pattern

func init() {
	patterns = []pattern{
		// Exact matches (checked before prefix)
		{prefix: "NEXT_PUBLIC_CONVEX_URL", exact: true, provider: "convex", confidence: "strong", reason: "Convex client URL"},

		// Specific long prefixes first
		{prefix: "NEXT_PUBLIC_SUPABASE_", exact: false, provider: "supabase", confidence: "strong", reason: "Supabase SDK convention"},

		// Standard provider prefixes
		{prefix: "VERCEL_", exact: false, provider: "vercel", confidence: "strong", reason: "Vercel platform variable"},
		{prefix: "CONVEX_", exact: false, provider: "convex", confidence: "strong", reason: "Convex platform variable"},
		{prefix: "RAILWAY_", exact: false, provider: "railway", confidence: "strong", reason: "Railway platform variable"},
		{prefix: "RENDER_", exact: false, provider: "render", confidence: "strong", reason: "Render platform variable"},
		{prefix: "SUPABASE_", exact: false, provider: "supabase", confidence: "strong", reason: "Supabase platform variable"},
		{prefix: "NETLIFY_", exact: false, provider: "netlify", confidence: "strong", reason: "Netlify platform variable"},
		{prefix: "FLY_", exact: false, provider: "flyio", confidence: "strong", reason: "Fly.io platform variable"},

		// Weak/heuristic matches (generic prefix, less confident)
		{prefix: "NEXT_PUBLIC_", exact: false, provider: "vercel", confidence: "weak", reason: "Next.js commonly deployed on Vercel"},
	}

	// Sort by prefix length descending so longer/more-specific patterns match first.
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i].prefix) > len(patterns[j].prefix)
	})
}

// FromKeys detects providers from a list of key names. Pure function, no I/O.
func FromKeys(keys []string) *Result {
	r := &Result{
		AllKeys: keys,
	}

	seen := make(map[string]bool)

	for _, key := range keys {
		for _, p := range patterns {
			var matched bool
			if p.exact {
				matched = key == p.prefix
			} else {
				matched = strings.HasPrefix(key, p.prefix)
			}
			if matched {
				r.Matches = append(r.Matches, Match{
					Key:          key,
					ProviderName: p.provider,
					Confidence:   p.confidence,
					Reason:       p.reason,
				})
				if !seen[p.provider] {
					seen[p.provider] = true
					r.Providers = append(r.Providers, p.provider)
				}
				// A key can match multiple providers, so don't break.
				// But skip patterns for the same provider we already matched
				// at a higher specificity.
				break
			}
		}
	}

	return r
}

// FromFile parses a .env file and detects providers from the keys found.
func FromFile(path string) (*Result, error) {
	fs := source.NewFileSource(path)
	all, err := fs.ListAll()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := FromKeys(keys)
	r.SourceFile = path
	return r, nil
}

// envFilePriority is the search order for auto-discovering .env files.
var envFilePriority = []string{
	".env",
	".env.test",
	".env.local",
	".env.development",
	".env.live",
	".env.production",
}

// FindEnvFile looks for .env files in the current directory, returning the
// first one found according to the priority list. Returns empty string if none found.
func FindEnvFile() string {
	for _, name := range envFilePriority {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}
