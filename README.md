# dotenvy

Sync environment variables across deployment platforms from your terminal.

[dotenvy.dev](https://dotenvy.dev) | [Install](#installation) | MIT License

## The Problem

You have secrets scattered across Vercel, Convex, Railway, Supabase, AWS, and local `.env` files. Keeping them in sync is tedious and error-prone.

## The Solution

Define which secrets you track in `dotenvy.yaml`, store values in `.env` files, and sync to all your platforms with one command.

```bash
dotenvy sync test
```

Your secrets never leave your machine. No middleman, no hosted vault.

```
    Local                          Remote
┌───────────┐    dotenvy sync   ┌───────────┐
│ .env.test  │ ───────────────► │ Vercel     │
│ .env.live  │                  │ Convex     │
│            │ ◄─────────────── │ Railway    │
└───────────┘    dotenvy pull   │ Supabase   │
                                │ AWS / GCP  │
                                │ ...        │
                                └───────────┘
```

## Installation

```bash
go install github.com/dotenvy-dev/dotenvy@latest
```

Or build from source:

```bash
git clone https://github.com/dotenvy-dev/dotenvy.git
cd dotenvy
go build -o dotenvy .
```

## Quick Start

If you already have a `.env` file, point `init` at it — it detects your providers, imports your keys, and bootstraps `.env.test`:

```bash
dotenvy init --from .env
```

Starting fresh? Just run `dotenvy init` and follow the prompts.

Then set up auth for your platforms:

```bash
export VERCEL_TOKEN="your-token"
export CONVEX_DEPLOY_KEY="your-key"
```

Preview and sync:

```bash
dotenvy sync test --dry-run   # preview changes
dotenvy sync test              # apply
```

## Commands

| Command | Description |
|---------|-------------|
| `dotenvy` | Launch interactive TUI |
| `dotenvy init` | Create config, detect platforms |
| `dotenvy add NAME...` | Add secret names to track |
| `dotenvy set KEY=VALUE` | Set a value, add to config, and sync everywhere |
| `dotenvy sync <env>` | Sync local env file to all targets |
| `dotenvy pull <target>` | Pull secrets from a target |
| `dotenvy status` | Show config and auth status |

Key flags:

- `dotenvy sync live --to vercel` — sync to a specific target
- `dotenvy sync test --no-file` — sync from environment variables instead of file
- `dotenvy set KEY=val --env live` — set a production secret
- `dotenvy pull vercel --env production -o .env.live` — pull to a file

## Supported Platforms

| Platform | Config type | Auth | Pull |
|----------|-------------|------|------|
| Vercel | `vercel` | `VERCEL_TOKEN` | yes |
| Convex | `convex` | `CONVEX_DEPLOY_KEY` | yes |
| Railway | `railway` | `RAILWAY_TOKEN` | yes |
| Render | `render` | `RENDER_API_KEY` | yes |
| Supabase | `supabase` | `SUPABASE_ACCESS_TOKEN` | no |
| Netlify | `netlify` | `NETLIFY_TOKEN` | yes |
| Fly.io | `flyio` | `FLY_API_TOKEN` | no |
| AWS Secrets Manager | `aws-secretsmanager` | AWS SDK credentials | yes |
| AWS Parameter Store | `aws-ssm` | AWS SDK credentials | yes |
| GCP Secret Manager | `gcp-secret-manager` | GCP SDK credentials | yes |
| Local .env files | `dotenv` | None | yes |

## Configuration

### Environment Mapping

dotenvy uses two local environments (`test` and `live`) that map to platform-specific environments:

```yaml
targets:
  vercel:
    type: vercel
    project: my-app
    mapping:
      development: test
      preview: test
      production: live
```

### Filtering

Sync only specific secrets to a target:

```yaml
targets:
  vercel:
    type: vercel
    project: my-app
    mapping:
      production: live
    include:
      - "STRIPE_*"
      - "DATABASE_*"
    exclude:
      - "*_DEV"
```

## Conflict Resolution

**`sync` — local wins.** Local values overwrite remote. Empty/missing local values are skipped (remote preserved). No automatic deletes.

| Local | Remote | Result |
|-------|--------|--------|
| `sk_test_new` | `sk_test_old` | Remote updated |
| `sk_test_xxx` | (not set) | Added to remote |
| (empty/missing) | `sk_test_old` | No change |

**`pull` — remote wins.** Remote values overwrite the local file entirely. Secrets not in your `dotenvy.yaml` schema are ignored.

Always use `--dry-run` before syncing to production.

## License

MIT
