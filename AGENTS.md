# Agents

Project context for AI coding assistants.

## Architecture

```
cmd/                    # Cobra CLI commands (init, add, set, sync, pull, status)
internal/
  config/               # dotenvy.yaml loading/saving
  auth/                 # Credential resolution (env vars + config)
  model/                # Core types: Secret, Target, Diff, SecretValue
  sync/                 # Sync engine with glob filtering
  source/               # Secret sources (.env files, env vars)
  detect/               # Auto-detection of deployment platforms
  api/                  # Fire-and-forget event posting
  tui/                  # Bubble Tea interactive UI
pkg/provider/           # Provider interface + registry
providers/              # One directory per platform (vercel/, convex/, railway/, etc.)
```

## Provider pattern

All providers implement the `SyncTarget` interface (Reader + Writer) in `pkg/provider/`. Each provider lives in `providers/<name>/` with three files:

- `<name>.go` — implements the provider, registers via `init()`
- `client.go` — HTTP/SDK client
- `<name>_test.go` — tests using a mock client

When adding a provider, follow the existing pattern exactly. Register it in `init()` so it's available via the provider registry.

## Key conventions

- **Two environments**: `test` and `live`. These map to platform-specific names (e.g., Vercel's `development`/`preview`/`production`).
- **Config file**: `dotenvy.yaml` (version 2 schema).
- **Plans and design docs** go in `.plans/`.

## Build and test

```bash
go build -o dotenvy .
go test ./...
```
