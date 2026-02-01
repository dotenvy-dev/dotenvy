#!/usr/bin/env bash
set -euo pipefail

# ── E2E Test Script for dotenvy ──────────────────────────────────────────────
# Builds dotenvy, runs the full CLI flow (status, set, sync, pull) against
# real Vercel, Convex, Railway, Fly.io, and Netlify targets, and cleans up.
#
# Required env vars:
#   VERCEL_TOKEN           - Personal Vercel API token
#   CONVEX_DEPLOY_KEY      - Deploy key for a persistent test deployment
#   CONVEX_E2E_DEPLOYMENT  - Deployment name (e.g. focused-sheep-371)
#   RAILWAY_TOKEN          - Railway API token
#   FLY_API_TOKEN          - Fly.io API token (org or deploy token)
#   NETLIFY_TOKEN          - Netlify personal access token
#   NETLIFY_ACCOUNT_ID     - Netlify account/team ID
#   SUPABASE_ACCESS_TOKEN  - Supabase org/personal access token
#   SUPABASE_ORG_ID        - Supabase organization ID
#   RENDER_API_KEY         - Render API key
#
# Optional:
#   VERCEL_TEAM_ID         - Defaults to team_mCCpaS633QcV8w0tKP6OfJeY
#
# All vars can also be set with DOTENVY_ prefix (e.g. DOTENVY_RAILWAY_TOKEN)

# ── Helpers ──────────────────────────────────────────────────────────────────

PASSED=0
FAILED=0
VERCEL_PROJECT_NAME=""
RAILWAY_PROJECT_ID=""
FLY_APP_NAME=""
NETLIFY_SITE_ID=""
SUPABASE_PROJECT_REF=""
RENDER_SERVICE_ID=""
WORK_DIR=""

pass() {
  PASSED=$((PASSED + 1))
  printf "  \033[32m✓ PASS\033[0m %s\n" "$1"
}

fail() {
  FAILED=$((FAILED + 1))
  printf "  \033[31m✗ FAIL\033[0m %s\n" "$1"
}

assert_contains() {
  local output="$1"
  local substring="$2"
  local desc="$3"
  if echo "$output" | grep -qF "$substring"; then
    pass "$desc"
  else
    fail "$desc (expected to contain: '$substring')"
    printf "    actual output:\n%s\n" "$output" | head -20
  fi
}

assert_not_contains() {
  local output="$1"
  local substring="$2"
  local desc="$3"
  if echo "$output" | grep -qF "$substring"; then
    fail "$desc (should NOT contain: '$substring')"
    printf "    actual output:\n%s\n" "$output" | head -20
  else
    pass "$desc"
  fi
}

assert_exit_zero() {
  local code="$1"
  local desc="$2"
  if [ "$code" -eq 0 ]; then
    pass "$desc"
  else
    fail "$desc (exit code: $code)"
  fi
}

# ── Cleanup ──────────────────────────────────────────────────────────────────

cleanup() {
  echo ""
  echo "── Cleanup ──"

  # Delete the ephemeral Vercel project
  if [ -n "$VERCEL_PROJECT_NAME" ]; then
    echo "Deleting Vercel project: $VERCEL_PROJECT_NAME"
    curl -sf -X DELETE \
      "https://api.vercel.com/v9/projects/$VERCEL_PROJECT_NAME?teamId=$VERCEL_TEAM_ID" \
      -H "Authorization: Bearer $VERCEL_TOKEN" \
      > /dev/null 2>&1 || echo "  (warning: failed to delete Vercel project)"
  fi

  # Delete the ephemeral Railway project
  if [ -n "$RAILWAY_PROJECT_ID" ]; then
    echo "Deleting Railway project: $RAILWAY_PROJECT_ID"
    curl -sf -X POST https://backboard.railway.com/graphql/v2 \
      -H "Authorization: Bearer $RAILWAY_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"query\":\"mutation { projectDelete(id: \\\"$RAILWAY_PROJECT_ID\\\") }\"}" \
      > /dev/null 2>&1 || echo "  (warning: failed to delete Railway project)"
  fi

  # Delete the ephemeral Fly.io app
  if [ -n "$FLY_APP_NAME" ]; then
    echo "Deleting Fly.io app: $FLY_APP_NAME"
    curl -sf -X DELETE "https://api.machines.dev/v1/apps/$FLY_APP_NAME" \
      -H "Authorization: Bearer $FLY_API_TOKEN" \
      > /dev/null 2>&1 || echo "  (warning: failed to delete Fly.io app)"
  fi

  # Delete the ephemeral Render service
  if [ -n "$RENDER_SERVICE_ID" ]; then
    echo "Deleting Render service: $RENDER_SERVICE_ID"
    curl -sf -X DELETE "https://api.render.com/v1/services/$RENDER_SERVICE_ID" \
      -H "Authorization: Bearer $RENDER_API_KEY" \
      > /dev/null 2>&1 || echo "  (warning: failed to delete Render service)"
  fi

  # Delete the ephemeral Supabase project
  if [ -n "$SUPABASE_PROJECT_REF" ]; then
    echo "Deleting Supabase project: $SUPABASE_PROJECT_REF"
    curl -sf -X DELETE "https://api.supabase.com/v1/projects/$SUPABASE_PROJECT_REF" \
      -H "Authorization: Bearer $SUPABASE_ACCESS_TOKEN" \
      > /dev/null 2>&1 || echo "  (warning: failed to delete Supabase project)"
  fi

  # Delete the ephemeral Netlify site
  if [ -n "$NETLIFY_SITE_ID" ]; then
    echo "Deleting Netlify site: $NETLIFY_SITE_ID"
    curl -sf -X DELETE "https://api.netlify.com/api/v1/sites/$NETLIFY_SITE_ID" \
      -H "Authorization: Bearer $NETLIFY_TOKEN" \
      > /dev/null 2>&1 || echo "  (warning: failed to delete Netlify site)"
  fi

  # Clean up Convex env vars left behind
  if [ -n "${CONVEX_DEPLOY_KEY:-}" ] && [ -n "${CONVEX_E2E_DEPLOYMENT:-}" ]; then
    echo "Cleaning Convex env vars..."
    for key in E2E_KEY_ONE E2E_KEY_TWO E2E_KEY_THREE; do
      curl -sf -X POST \
        "https://$CONVEX_E2E_DEPLOYMENT.convex.cloud/api/update_environment_variables" \
        -H "Authorization: Convex $CONVEX_DEPLOY_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"changes\":[{\"name\":\"$key\"}]}" \
        > /dev/null 2>&1 || true
    done
  fi

  # Remove temp work dir
  if [ -n "$WORK_DIR" ] && [ -d "$WORK_DIR" ]; then
    echo "Removing temp dir: $WORK_DIR"
    rm -rf "$WORK_DIR"
  fi

  echo "Done."
}

trap cleanup EXIT

# ── Phase 1: Validate Prerequisites ─────────────────────────────────────────

echo "── Phase 1: Validate Prerequisites ──"

missing=0
for cmd in go curl jq; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "ERROR: '$cmd' is required but not found in PATH"
    missing=1
  fi
done

# Resolve from DOTENVY_* prefixed vars, falling back to standard names
export VERCEL_TOKEN="${VERCEL_TOKEN:-${DOTENVY_VERCEL_TOKEN:-}}"
export CONVEX_DEPLOY_KEY="${CONVEX_DEPLOY_KEY:-${DOTENVY_CONVEX_DEPLOY_KEY:-}}"
CONVEX_E2E_DEPLOYMENT="${CONVEX_E2E_DEPLOYMENT:-${DOTENVY_CONVEX_DEPLOYMENT:-}}"
export RAILWAY_TOKEN="${RAILWAY_TOKEN:-${DOTENVY_RAILWAY_TOKEN:-}}"
export FLY_API_TOKEN="${FLY_API_TOKEN:-${DOTENVY_FLY_API_TOKEN:-}}"
export NETLIFY_TOKEN="${NETLIFY_TOKEN:-${DOTENVY_NETLIFY_TOKEN:-}}"
NETLIFY_ACCOUNT_ID="${NETLIFY_ACCOUNT_ID:-${DOTENVY_NETLIFY_ACCOUNT_ID:-}}"
export SUPABASE_ACCESS_TOKEN="${SUPABASE_ACCESS_TOKEN:-${DOTENVY_SUPABASE_ACCESS_TOKEN:-}}"
SUPABASE_ORG_ID="${SUPABASE_ORG_ID:-${DOTENVY_SUPABASE_ORG_ID:-}}"
export RENDER_API_KEY="${RENDER_API_KEY:-${DOTENVY_RENDER_API_KEY:-}}"

for var in VERCEL_TOKEN CONVEX_DEPLOY_KEY CONVEX_E2E_DEPLOYMENT RAILWAY_TOKEN FLY_API_TOKEN NETLIFY_TOKEN NETLIFY_ACCOUNT_ID SUPABASE_ACCESS_TOKEN SUPABASE_ORG_ID RENDER_API_KEY; do
  if [ -z "${!var:-}" ]; then
    echo "ERROR: $var (or DOTENVY_ prefixed version) is not set"
    missing=1
  fi
done

if [ "$missing" -ne 0 ]; then
  echo "Aborting: missing prerequisites."
  exit 1
fi

VERCEL_TEAM_ID="${VERCEL_TEAM_ID:-team_mCCpaS633QcV8w0tKP6OfJeY}"

echo "Prerequisites OK"
echo "  VERCEL_TEAM_ID=$VERCEL_TEAM_ID"
echo "  CONVEX_E2E_DEPLOYMENT=$CONVEX_E2E_DEPLOYMENT"

# ── Phase 2: Build Binary ───────────────────────────────────────────────────

echo ""
echo "── Phase 2: Build Binary ──"

WORK_DIR=$(mktemp -d)
echo "Work dir: $WORK_DIR"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
go build -o "$WORK_DIR/dotenvy" "$REPO_ROOT"
echo "Built: $WORK_DIR/dotenvy"

DOTENVY="$WORK_DIR/dotenvy"

# ── Phase 3: Create Vercel Test Project ──────────────────────────────────────

echo ""
echo "── Phase 3: Create Vercel Test Project ──"

VERCEL_PROJECT_NAME="dotenvy-e2e-$(date +%s)"

create_resp=$(curl -sf -X POST \
  "https://api.vercel.com/v9/projects?teamId=$VERCEL_TEAM_ID" \
  -H "Authorization: Bearer $VERCEL_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$VERCEL_PROJECT_NAME\"}")

vercel_project_id=$(echo "$create_resp" | jq -r '.id // empty')
if [ -z "$vercel_project_id" ]; then
  echo "ERROR: Failed to create Vercel project"
  echo "$create_resp" | jq . 2>/dev/null || echo "$create_resp"
  exit 1
fi

echo "Created Vercel project: $VERCEL_PROJECT_NAME (id: $vercel_project_id)"

# ── Phase 3b: Create Railway Test Project ────────────────────────────────────

echo ""
echo "── Phase 3b: Create Railway Test Project ──"

RAILWAY_PROJECT_NAME="dotenvy-e2e-$(date +%s)"

railway_resp=$(curl -sf -X POST https://backboard.railway.com/graphql/v2 \
  -H "Authorization: Bearer $RAILWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"mutation { projectCreate(input: { name: \\\"$RAILWAY_PROJECT_NAME\\\" }) { id } }\"}")

RAILWAY_PROJECT_ID=$(echo "$railway_resp" | jq -r '.data.projectCreate.id // empty')
if [ -z "$RAILWAY_PROJECT_ID" ]; then
  echo "ERROR: Failed to create Railway project"
  echo "$railway_resp" | jq . 2>/dev/null || echo "$railway_resp"
  exit 1
fi

echo "Created Railway project: $RAILWAY_PROJECT_NAME (id: $RAILWAY_PROJECT_ID)"

# ── Phase 3c: Create Fly.io Test App ─────────────────────────────────────────

echo ""
echo "── Phase 3c: Create Fly.io Test App ──"

FLY_APP_NAME="dotenvy-e2e-$(date +%s)"

fly_resp=$(curl -sf -X POST "https://api.machines.dev/v1/apps" \
  -H "Authorization: Bearer $FLY_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"app_name\":\"$FLY_APP_NAME\",\"org_slug\":\"personal\"}")

fly_app_id=$(echo "$fly_resp" | jq -r '.id // empty')
if [ -z "$fly_app_id" ]; then
  echo "ERROR: Failed to create Fly.io app"
  echo "$fly_resp" | jq . 2>/dev/null || echo "$fly_resp"
  exit 1
fi

echo "Created Fly.io app: $FLY_APP_NAME (id: $fly_app_id)"

# ── Phase 3d: Create Netlify Test Site ────────────────────────────────────────

echo ""
echo "── Phase 3d: Create Netlify Test Site ──"

NETLIFY_SITE_NAME="dotenvy-e2e-$(date +%s)"

netlify_resp=$(curl -sf -X POST "https://api.netlify.com/api/v1/sites" \
  -H "Authorization: Bearer $NETLIFY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$NETLIFY_SITE_NAME\",\"account_slug\":\"sahilprasad\"}")

NETLIFY_SITE_ID=$(echo "$netlify_resp" | jq -r '.id // empty')
if [ -z "$NETLIFY_SITE_ID" ]; then
  echo "ERROR: Failed to create Netlify site"
  echo "$netlify_resp" | jq . 2>/dev/null || echo "$netlify_resp"
  exit 1
fi

echo "Created Netlify site: $NETLIFY_SITE_NAME (id: $NETLIFY_SITE_ID)"

# ── Phase 3e: Create Supabase Test Project ────────────────────────────────────

echo ""
echo "── Phase 3e: Create Supabase Test Project ──"

SUPABASE_PROJECT_NAME="dotenvy-e2e-$(date +%s)"

supabase_resp=$(curl -sf -X POST "https://api.supabase.com/v1/projects" \
  -H "Authorization: Bearer $SUPABASE_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$SUPABASE_PROJECT_NAME\",\"organization_id\":\"$SUPABASE_ORG_ID\",\"region\":\"us-east-1\",\"db_pass\":\"e2eTestPw$(date +%s)X\"}")

SUPABASE_PROJECT_REF=$(echo "$supabase_resp" | jq -r '.ref // empty')
if [ -z "$SUPABASE_PROJECT_REF" ]; then
  echo "ERROR: Failed to create Supabase project"
  echo "$supabase_resp" | jq . 2>/dev/null || echo "$supabase_resp"
  exit 1
fi

echo "Created Supabase project: $SUPABASE_PROJECT_NAME (ref: $SUPABASE_PROJECT_REF)"

# ── Phase 3f: Create Render Test Service ──────────────────────────────────────

echo ""
echo "── Phase 3f: Create Render Test Service ──"

RENDER_SERVICE_NAME="dotenvy-e2e-$(date +%s)"

render_resp=$(curl -sf -X POST "https://api.render.com/v1/services" \
  -H "Authorization: Bearer $RENDER_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d "{\"name\":\"$RENDER_SERVICE_NAME\",\"ownerId\":\"tea-cbmbp3vho1kta1m6jne0\",\"type\":\"web_service\",\"autoDeploy\":\"no\",\"serviceDetails\":{\"runtime\":\"image\",\"plan\":\"free\"},\"image\":{\"ownerId\":\"tea-cbmbp3vho1kta1m6jne0\",\"imagePath\":\"busybox:latest\"}}")

RENDER_SERVICE_ID=$(echo "$render_resp" | jq -r '.service.id // empty')
if [ -z "$RENDER_SERVICE_ID" ]; then
  echo "ERROR: Failed to create Render service"
  echo "$render_resp" | jq . 2>/dev/null || echo "$render_resp"
  exit 1
fi

echo "Created Render service: $RENDER_SERVICE_NAME (id: $RENDER_SERVICE_ID)"

# ── Phase 4: Clean Convex State & Write Config ──────────────────────────────

echo ""
echo "── Phase 4: Clean Convex State & Write Config ──"

# Clear any leftover env vars from previous runs
for key in E2E_KEY_ONE E2E_KEY_TWO E2E_KEY_THREE; do
  curl -sf -X POST \
    "https://$CONVEX_E2E_DEPLOYMENT.convex.cloud/api/update_environment_variables" \
    -H "Authorization: Convex $CONVEX_DEPLOY_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"changes\":[{\"name\":\"$key\"}]}" \
    > /dev/null 2>&1 || true
done
echo "Cleared Convex env vars"

cat > "$WORK_DIR/dotenvy-e2e.yaml" <<EOF
version: 2
secrets: []
targets:
  vercel:
    type: vercel
    project: $VERCEL_PROJECT_NAME
    team_id: $VERCEL_TEAM_ID
    mapping:
      development: test
      preview: test
      production: live
  convex:
    type: convex
    deployment: $CONVEX_E2E_DEPLOYMENT
    mapping:
      default: test
  railway:
    type: railway
    project_id: $RAILWAY_PROJECT_ID
    mapping:
      production: test
  flyio:
    type: flyio
    app_name: $FLY_APP_NAME
    mapping:
      default: test
  netlify:
    type: netlify
    account_id: $NETLIFY_ACCOUNT_ID
    site_id: $NETLIFY_SITE_ID
    mapping:
      dev: test
  supabase:
    type: supabase
    project_ref: $SUPABASE_PROJECT_REF
    mapping:
      default: test
  render:
    type: render
    service_id: $RENDER_SERVICE_ID
    mapping:
      default: test
  local_dotenv:
    type: dotenv
    path: .env.e2e-dotenv-target
    mapping:
      local: test
EOF

echo "Config written to $WORK_DIR/dotenvy-e2e.yaml"

# ── Phase 5: Run Tests ──────────────────────────────────────────────────────

echo ""
echo "── Phase 5: Run Tests ──"

# All commands run from WORK_DIR with the e2e config
run() {
  # Run dotenvy from WORK_DIR so .env.test etc. are created there
  (cd "$WORK_DIR" && "$DOTENVY" --config dotenvy-e2e.yaml "$@" --plain 2>&1) || true
}

# For commands that don't accept --plain (status, pull)
run_no_plain() {
  (cd "$WORK_DIR" && "$DOTENVY" --config dotenvy-e2e.yaml "$@" 2>&1) || true
}

# Capture exit code only (discard output)
run_with_code() {
  local code=0
  (cd "$WORK_DIR" && "$DOTENVY" --config dotenvy-e2e.yaml "$@" --plain > /dev/null 2>&1) || code=$?
  echo "$code"
}

# ── Test 1: Status shows all targets as authenticated ──
echo ""
echo "Test 1: status shows all targets authenticated"
output=$(run_no_plain status)
assert_contains "$output" "authenticated" "status output contains 'authenticated'"
assert_contains "$output" "vercel" "status output contains 'vercel'"
assert_contains "$output" "convex" "status output contains 'convex'"
assert_contains "$output" "railway" "status output contains 'railway'"
assert_contains "$output" "flyio" "status output contains 'flyio'"
assert_contains "$output" "netlify" "status output contains 'netlify'"
assert_contains "$output" "supabase" "status output contains 'supabase'"
assert_contains "$output" "render" "status output contains 'render'"
assert_contains "$output" "local_dotenv" "status output contains 'local_dotenv'"
assert_not_contains "$output" "not authenticated" "status has no auth failures"

# ── Test 2: set E2E_KEY_ONE=hello ──
echo ""
echo "Test 2: set E2E_KEY_ONE=hello"
output=$(run set E2E_KEY_ONE=hello)
code=$(run_with_code set E2E_KEY_ONE=hello 2>/dev/null)
# Check exit code (the second run is idempotent, that's fine)
assert_exit_zero "$code" "set E2E_KEY_ONE=hello exits 0"
# Check the .env.test file was created
if [ -f "$WORK_DIR/.env.test" ] && grep -qF "E2E_KEY_ONE=hello" "$WORK_DIR/.env.test"; then
  pass ".env.test contains E2E_KEY_ONE=hello"
else
  fail ".env.test should contain E2E_KEY_ONE=hello"
fi

# ── Test 3: set multiple secrets at once ──
echo ""
echo "Test 3: set E2E_KEY_TWO=second E2E_KEY_THREE=third"
output=$(run set E2E_KEY_TWO=second E2E_KEY_THREE=third)
code=$(run_with_code set E2E_KEY_TWO=second E2E_KEY_THREE=third 2>/dev/null)
assert_exit_zero "$code" "set multiple secrets exits 0"
if grep -qF "E2E_KEY_TWO=second" "$WORK_DIR/.env.test" && grep -qF "E2E_KEY_THREE=third" "$WORK_DIR/.env.test"; then
  pass ".env.test contains both new secrets"
else
  fail ".env.test should contain E2E_KEY_TWO and E2E_KEY_THREE"
fi

# ── Test 4: sync test is idempotent (no changes for Convex) ──
echo ""
echo "Test 4: sync test (idempotent - Convex shows no changes)"
output=$(run sync test)
assert_contains "$output" "No changes" "sync test shows 'No changes' for convex (idempotent)"

# ── Test 5: pull convex --env default shows all 3 secrets with values ──
echo ""
echo "Test 5: pull convex --env default (plaintext values)"
output=$(run_no_plain pull convex --env default)
assert_contains "$output" "E2E_KEY_ONE=hello" "pull convex has E2E_KEY_ONE=hello"
assert_contains "$output" "E2E_KEY_TWO=second" "pull convex has E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull convex has E2E_KEY_THREE=third"

# ── Test 6: pull vercel --env development shows all 3 key names ──
echo ""
echo "Test 6: pull vercel --env development (key names present)"
output=$(run_no_plain pull vercel --env development)
assert_contains "$output" "E2E_KEY_ONE" "pull vercel has E2E_KEY_ONE"
assert_contains "$output" "E2E_KEY_TWO" "pull vercel has E2E_KEY_TWO"
assert_contains "$output" "E2E_KEY_THREE" "pull vercel has E2E_KEY_THREE"

# ── Test 7: pull railway --env production shows all 3 secrets with values ──
echo ""
echo "Test 7: pull railway --env production (plaintext round-trip)"
output=$(run_no_plain pull railway --env production)
assert_contains "$output" "E2E_KEY_ONE=hello" "pull railway has E2E_KEY_ONE=hello"
assert_contains "$output" "E2E_KEY_TWO=second" "pull railway has E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull railway has E2E_KEY_THREE=third"

# ── Test 8: pull flyio --env default blocked (write-only) ──
echo ""
echo "Test 8: pull flyio --env default (blocked, write-only provider)"
if output=$(cd "$WORK_DIR" && "$DOTENVY" --config dotenvy-e2e.yaml pull flyio --env default 2>&1); then
  fail "pull flyio should have failed (write-only)"
else
  assert_contains "$output" "write-only" "pull flyio error mentions write-only"
  pass "pull flyio correctly blocked"
fi

# ── Test 9: pull netlify --env dev shows all 3 secrets with values ──
echo ""
echo "Test 9: pull netlify --env dev (plaintext round-trip)"
output=$(run_no_plain pull netlify --env dev)
assert_contains "$output" "E2E_KEY_ONE=hello" "pull netlify has E2E_KEY_ONE=hello"
assert_contains "$output" "E2E_KEY_TWO=second" "pull netlify has E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull netlify has E2E_KEY_THREE=third"

# ── Test 10: pull supabase --env default blocked (write-only) ──
echo ""
echo "Test 10: pull supabase --env default (blocked, write-only provider)"
if output=$(cd "$WORK_DIR" && "$DOTENVY" --config dotenvy-e2e.yaml pull supabase --env default 2>&1); then
  fail "pull supabase should have failed (write-only)"
else
  assert_contains "$output" "write-only" "pull supabase error mentions write-only"
  pass "pull supabase correctly blocked"
fi

# ── Test 11: pull render --env default shows all 3 secrets with values ──
echo ""
echo "Test 11: pull render --env default (plaintext round-trip)"
output=$(run_no_plain pull render --env default)
assert_contains "$output" "E2E_KEY_ONE=hello" "pull render has E2E_KEY_ONE=hello"
assert_contains "$output" "E2E_KEY_TWO=second" "pull render has E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull render has E2E_KEY_THREE=third"

# ── Test 12: pull local_dotenv --env local shows all 3 secrets with values ──
echo ""
echo "Test 12: pull local_dotenv --env local (plaintext round-trip)"
output=$(run_no_plain pull local_dotenv --env local)
assert_contains "$output" "E2E_KEY_ONE=hello" "pull dotenv has E2E_KEY_ONE=hello"
assert_contains "$output" "E2E_KEY_TWO=second" "pull dotenv has E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull dotenv has E2E_KEY_THREE=third"

# ── Test 13: set E2E_KEY_ONE=updated (only KEY_ONE changes) ──
echo ""
echo "Test 13: set E2E_KEY_ONE=updated (only KEY_ONE marked as changed)"
output=$(run set E2E_KEY_ONE=updated)
assert_contains "$output" "E2E_KEY_ONE" "output mentions E2E_KEY_ONE"
assert_contains "$output" "changed" "output shows 'changed' for updated key"

# ── Test 14: pull convex confirms round-trip ──
echo ""
echo "Test 14: pull convex --env default (round-trip: E2E_KEY_ONE=updated)"
output=$(run_no_plain pull convex --env default)
assert_contains "$output" "E2E_KEY_ONE=updated" "pull convex round-trip: E2E_KEY_ONE=updated"
assert_contains "$output" "E2E_KEY_TWO=second" "pull convex round-trip: E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull convex round-trip: E2E_KEY_THREE=third"

# ── Test 15: pull railway confirms round-trip after update ──
echo ""
echo "Test 15: pull railway --env production (round-trip: E2E_KEY_ONE=updated)"
output=$(run_no_plain pull railway --env production)
assert_contains "$output" "E2E_KEY_ONE=updated" "pull railway round-trip: E2E_KEY_ONE=updated"
assert_contains "$output" "E2E_KEY_TWO=second" "pull railway round-trip: E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull railway round-trip: E2E_KEY_THREE=third"

# ── Test 16: pull flyio still blocked after update (write-only) ──
echo ""
echo "Test 16: pull flyio --env default (still blocked after update, write-only)"
if output=$(cd "$WORK_DIR" && "$DOTENVY" --config dotenvy-e2e.yaml pull flyio --env default 2>&1); then
  fail "pull flyio should have failed (write-only)"
else
  assert_contains "$output" "write-only" "pull flyio post-update error mentions write-only"
  pass "pull flyio correctly blocked post-update"
fi

# ── Test 17: pull netlify confirms round-trip after update ──
echo ""
echo "Test 17: pull netlify --env dev (round-trip: E2E_KEY_ONE=updated)"
output=$(run_no_plain pull netlify --env dev)
assert_contains "$output" "E2E_KEY_ONE=updated" "pull netlify round-trip: E2E_KEY_ONE=updated"
assert_contains "$output" "E2E_KEY_TWO=second" "pull netlify round-trip: E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull netlify round-trip: E2E_KEY_THREE=third"

# ── Test 18: pull supabase still blocked after update (write-only) ──
echo ""
echo "Test 18: pull supabase --env default (still blocked after update, write-only)"
if output=$(cd "$WORK_DIR" && "$DOTENVY" --config dotenvy-e2e.yaml pull supabase --env default 2>&1); then
  fail "pull supabase should have failed (write-only)"
else
  assert_contains "$output" "write-only" "pull supabase post-update error mentions write-only"
  pass "pull supabase correctly blocked post-update"
fi

# ── Test 19: pull render confirms round-trip after update ──
echo ""
echo "Test 19: pull render --env default (round-trip: E2E_KEY_ONE=updated)"
output=$(run_no_plain pull render --env default)
assert_contains "$output" "E2E_KEY_ONE=updated" "pull render round-trip: E2E_KEY_ONE=updated"
assert_contains "$output" "E2E_KEY_TWO=second" "pull render round-trip: E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull render round-trip: E2E_KEY_THREE=third"

# ── Test 20: pull local_dotenv confirms round-trip after update ──
echo ""
echo "Test 20: pull local_dotenv --env local (round-trip: E2E_KEY_ONE=updated)"
output=$(run_no_plain pull local_dotenv --env local)
assert_contains "$output" "E2E_KEY_ONE=updated" "pull dotenv round-trip: E2E_KEY_ONE=updated"
assert_contains "$output" "E2E_KEY_TWO=second" "pull dotenv round-trip: E2E_KEY_TWO=second"
assert_contains "$output" "E2E_KEY_THREE=third" "pull dotenv round-trip: E2E_KEY_THREE=third"

# ── Phase 6: Summary ────────────────────────────────────────────────────────

echo ""
echo "── Summary ──"
TOTAL=$((PASSED + FAILED))
echo "  $PASSED/$TOTAL passed"

if [ "$FAILED" -gt 0 ]; then
  printf "  \033[31m%d FAILED\033[0m\n" "$FAILED"
  exit 1
else
  printf "  \033[32mAll tests passed!\033[0m\n"
  exit 0
fi
