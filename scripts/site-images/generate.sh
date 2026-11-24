#!/usr/bin/env bash
# generate.sh — generate all site/public/img/ assets for the trackfw site.
#
# Every command of bin/trackfw runs inside a fully isolated environment (HOME,
# USERPROFILE, CODEX_HOME, XDG_CONFIG_HOME, GIT identity, TZ, PS1) so no
# personal data can appear in generated images.  Follows threat model C1 from
# docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md.
#
# After generating, calls scripts/check-site-images-leak.sh to scan every
# output for leaks.
#
# USAGE
#   bash scripts/site-images/generate.sh
#   # Requires: make build ran first (bin/trackfw must exist)
#   # Requires: vhs on PATH
#   # Requires: chrome-headless-shell at the well-known Playwright cache path
#
# OUTPUT
#   site/public/img/quickstart.gif
#   site/public/img/guard.gif
#   site/public/img/demo.webm
#   site/public/img/board.png

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="$REPO_ROOT/bin/trackfw"
OUTPUT_DIR="$REPO_ROOT/site/public/img"
TAPES_DIR="$SCRIPT_DIR"
DEMO_TAPE="$REPO_ROOT/demo.tape"
CHROME="/Users/kgsaran/Library/Caches/ms-playwright/chromium_headless_shell-1208/chrome-headless-shell-mac-arm64/chrome-headless-shell"

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
WORKDIR=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-site-gen.XXXXXX")
SERVE_PID=""
cleanup() {
  if [ -n "$SERVE_PID" ] && kill -0 "$SERVE_PID" 2>/dev/null; then
    kill "$SERVE_PID" 2>/dev/null || true
    sleep 0.5
    kill -9 "$SERVE_PID" 2>/dev/null || true
  fi
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Pre-flight checks
# ---------------------------------------------------------------------------
if [ ! -x "$BINARY" ]; then
  echo "ERROR: $BINARY not found — run 'make build' first" >&2
  exit 1
fi

if ! command -v vhs >/dev/null 2>&1; then
  echo "ERROR: vhs not found on PATH — install with: brew install vhs" >&2
  exit 1
fi

if [ ! -x "$CHROME" ]; then
  echo "WARNING: chrome-headless-shell not found at $CHROME — board screenshot will be skipped" >&2
  CHROME=""
fi

echo "=== generate.sh: bin/trackfw version ==="
"$BINARY" version

mkdir -p "$OUTPUT_DIR"

# ---------------------------------------------------------------------------
# Isolated environment for all direct binary calls
# ---------------------------------------------------------------------------
export HOME="$WORKDIR/home"
mkdir -p "$HOME"
export USERPROFILE="$HOME"
export CODEX_HOME="$HOME/.codex"
export XDG_CONFIG_HOME="$HOME/.config"
export GIT_AUTHOR_NAME="Demo User"
export GIT_AUTHOR_EMAIL="demo@example.invalid"
export GIT_COMMITTER_NAME="Demo User"
export GIT_COMMITTER_EMAIL="demo@example.invalid"
export TZ=UTC
export PS1='$ '
unset GH_TOKEN   || true
unset GITHUB_TOKEN || true

# Add bin/ to PATH so VHS tapes find the right binary
export PATH="$REPO_ROOT/bin:$PATH"

# Verify isolation: bin/trackfw must run with this isolated HOME
"$BINARY" version >/dev/null

# ---------------------------------------------------------------------------
# Step 1: Run quickstart.tape → site/public/img/quickstart.gif
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 1: quickstart.gif ==="
QUICKSTART_TAPE="$WORKDIR/quickstart.tape"
# VHS Output paths are relative to CWD where vhs is invoked.
# Run vhs from REPO_ROOT with a relative Output path.
sed "s|^Output .*|Output site/public/img/quickstart.gif|" \
  "$TAPES_DIR/quickstart.tape" > "$QUICKSTART_TAPE"

# Verify bin/trackfw version inside the tape env matches what generate.sh uses
# (a system trackfw on PATH would have a different version)
BIN_VERSION=$("$BINARY" version 2>/dev/null | head -1)
echo "  Using bin/trackfw: $BIN_VERSION"

cd "$REPO_ROOT"
vhs "$QUICKSTART_TAPE"

if [ -s "$OUTPUT_DIR/quickstart.gif" ]; then
  QSSIZE=$(wc -c < "$OUTPUT_DIR/quickstart.gif")
  echo "  OK: quickstart.gif ($QSSIZE bytes)"
else
  echo "ERROR: quickstart.gif not generated" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Step 2: Run guard.tape → site/public/img/guard.gif
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 2: guard.gif ==="
GUARD_TAPE="$WORKDIR/guard.tape"
sed "s|^Output .*|Output site/public/img/guard.gif|" \
  "$TAPES_DIR/guard.tape" > "$GUARD_TAPE"

cd "$REPO_ROOT"
vhs "$GUARD_TAPE"

if [ -s "$OUTPUT_DIR/guard.gif" ]; then
  GSIZE=$(wc -c < "$OUTPUT_DIR/guard.gif")
  echo "  OK: guard.gif ($GSIZE bytes)"
else
  echo "ERROR: guard.gif not generated" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Step 3: Derive demo.webm from demo.tape → site/public/img/demo.webm
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 3: demo.webm ==="
DEMO_TAPE_TMP="$WORKDIR/demo.tape"
# VHS Output paths are relative to CWD. Run from REPO_ROOT with relative path.
# Replace "Output docs/demo.gif" with the webm path relative to REPO_ROOT.
sed "s|^Output docs/demo.gif|Output site/public/img/demo.webm|" \
  "$DEMO_TAPE" > "$DEMO_TAPE_TMP"

# Verify the modification happened
if ! grep -E "^Output site/public/img/demo.webm" "$DEMO_TAPE_TMP" >/dev/null 2>&1; then
  echo "ERROR: demo.tape Output replacement did not work" >&2
  exit 1
fi

cd "$REPO_ROOT"
vhs "$DEMO_TAPE_TMP"

if [ -s "$OUTPUT_DIR/demo.webm" ]; then
  DSIZE=$(wc -c < "$OUTPUT_DIR/demo.webm")
  echo "  OK: demo.webm ($DSIZE bytes)"
else
  echo "ERROR: demo.webm not generated" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Step 4: board.png — start trackfw serve on a demo project, screenshot, kill
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 4: board.png ==="

if [ -z "$CHROME" ]; then
  echo "  SKIP: chrome-headless-shell not found — board.png skipped"
else
  # Baseline: check port 4081 is free before we start
  if lsof -nP -iTCP:4081 -sTCP:LISTEN 2>/dev/null | grep -E '4081' >/dev/null 2>&1; then
    echo "ERROR: port 4081 is already in use — cannot start trackfw serve" >&2
    exit 1
  fi

  # Create the demo project with fixture roadmaps in different states
  DEMO_DIR="$WORKDIR/acme-app"
  mkdir -p "$DEMO_DIR"
  (
    cd "$DEMO_DIR"
    git init -q -b main
    git config user.name "Demo User"
    git config user.email "demo@example.invalid"

    cat > trackfw.yaml <<'YAML'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dirs:
  - docs/adr
roadmap_namespacing: flat
YAML

    mkdir -p docs/adr docs/req docs/roadmaps/backlog docs/roadmaps/wip docs/roadmaps/done

    # ADR
    cat > docs/adr/ADR-2026-01-15-authentication-strategy.md <<'MD'
---
status: accepted
date: 2026-01-15
---
# ADR: Authentication Strategy

## Context
The application needs a secure authentication mechanism.

## Decision
Use JWT tokens with bcrypt password hashing.

## Consequences
Stateless auth; horizontal scaling without session storage.

## Alternatives Considered
Session cookies — rejected for scalability.
MD

    # REQ
    cat > docs/req/REQ-2026-01-16-user-authentication.md <<'MD'
---
status: wip
date: 2026-01-16
adr: docs/adr/ADR-2026-01-15-authentication-strategy.md
---
# REQ: User Authentication

## Motivation
Users need to sign in to access their personalized dashboard.

## Acceptance Criteria
- [ ] Login with email and password
- [ ] JWT token returned on success
- [ ] Incorrect credentials return 401
MD

    # Roadmap: done
    cat > docs/roadmaps/done/ROADMAP-2026-01-10-project-setup.md <<'MD'
---
status: done
date: 2026-01-10
req: ""
---
# Roadmap: Project Setup

## Wave 1

### ML-1A — Initialize repository
**Status:** ✅ Done

### ML-1B — Configure CI
**Status:** ✅ Done
MD

    # Roadmap: wip (linked to REQ)
    cat > docs/roadmaps/wip/ROADMAP-2026-01-16-user-authentication.md <<'MD'
---
status: wip
date: 2026-01-16
req: docs/req/REQ-2026-01-16-user-authentication.md
---
# Roadmap: User Authentication

## Wave 1

### ML-1A — Implement login endpoint
**Status:** 🔄 In progress

### ML-1B — Write integration tests
**Status:** ⬜ Pending
MD

    # Roadmap: backlog
    cat > docs/roadmaps/backlog/ROADMAP-2026-01-20-add-oauth-provider.md <<'MD'
---
status: backlog
date: 2026-01-20
req: ""
---
# Roadmap: Add OAuth Provider

## Wave 1

### ML-1A — Implement Google OAuth
**Status:** ⬜ Pending
MD
  )

  # Start trackfw serve in background from demo dir
  (
    cd "$DEMO_DIR"
    "$BINARY" serve --host 127.0.0.1 --port 4081
  ) &
  SERVE_PID=$!

  # Wait for serve to be ready (poll up to 15s)
  READY=0
  for i in $(seq 1 30); do
    if curl -s --max-time 1 http://127.0.0.1:4081/ >/dev/null 2>&1; then
      READY=1
      break
    fi
    sleep 0.5
  done

  if [ "$READY" -eq 0 ]; then
    echo "ERROR: trackfw serve did not start on port 4081 within 15s" >&2
    exit 1
  fi

  echo "  serve ready on port 4081 (PID=$SERVE_PID)"

  # Verify board API is serving safe data (no absolute paths)
  BOARD_JSON=$( { curl -s --max-time 5 http://127.0.0.1:4081/api/board || true; } )
  if printf '%s\n' "$BOARD_JSON" | grep -E '/Users/[a-zA-Z]|/home/[a-zA-Z]' >/dev/null 2>&1; then
    echo "ERROR: /api/board returned an absolute path — board screenshot would leak data" >&2
    printf '%s\n' "$BOARD_JSON" | grep -E '/Users/|/home/' >/dev/null 2>&1 && true
    exit 1
  fi
  echo "  /api/board: no path leaks detected"

  # Screenshot: board only (default view, no chain tab)
  CHROME_DATA="$WORKDIR/chrome-data"
  mkdir -p "$CHROME_DATA"
  "$CHROME" \
    --disable-gpu \
    --hide-scrollbars \
    --virtual-time-budget=5000 \
    --user-data-dir="$CHROME_DATA" \
    --window-size=1280,900 \
    --screenshot="$OUTPUT_DIR/board.png" \
    "http://127.0.0.1:4081/" 2>&1 | head -5

  # Kill serve
  kill "$SERVE_PID" 2>/dev/null || true
  SERVE_PID=""
  sleep 0.3

  if [ -s "$OUTPUT_DIR/board.png" ]; then
    BSIZE=$(wc -c < "$OUTPUT_DIR/board.png")
    echo "  OK: board.png ($BSIZE bytes)"
  else
    echo "ERROR: board.png not generated" >&2
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Step 5: Run the leak scanner on all generated files
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 5: leak scan ==="
bash "$REPO_ROOT/scripts/check-site-images-leak.sh"

echo ""
echo "=== generate.sh: all done ==="
echo "Generated files in $OUTPUT_DIR:"
ls -lh "$OUTPUT_DIR/" 2>/dev/null || echo "  (none)"
