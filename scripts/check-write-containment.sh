#!/usr/bin/env bash
# check-write-containment.sh — anti-reintroduction gate for
# ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever, ML-2A.
#
# Why this exists: Wave 1 swept internal/**/*.go so that every write site that
# derives a destination path from a user-supplied root goes through
# pathguard.RejectSymlinks or pathguard.GuardedWrite before writing. This gate
# stops the next ml from reintroducing a raw write site one site at a time,
# unnoticed — which is exactly how the 155 unguarded sites accumulated before
# Wave 1.
#
# Design: scan all production Go files in internal/ for write primitives
# (os.WriteFile, os.Create, os.CreateTemp, os.OpenFile, os.Rename, os.MkdirAll).
# A site is accepted ONLY if its own line OR the line immediately above it
# carries the literal marker "write-containment-allowed:" followed by a reason.
# No proximity window — the marker is a declaration by the author, not an
# inference from nearby code.
#
# This is intentionally different from check-symlink-privilege-guard.sh (which
# uses a ±5 line window). The window design reproved correct code: ML-1D-bis
# existed solely because symlinkOrSkipMetrics was 13 lines above its site, outside
# the window. An entire ML was paid to move a comment. This gate never repeats that.
#
# internal/pathguard/pathguard.go implements GuardedWrite itself (using
# MkdirAll / CreateTemp / Rename) and carries inline markers on those three sites —
# the same self-exemption pattern used by check-raw-read-ban.sh for its
# fail-safe helper.
#
# Non-negotiable — vacuity guard: if the gate examines fewer sites than SITE_FLOOR
# and WRITE_CONTAINMENT_SCAN_DIR is not set (production run), it FAILS with
# "recusando reportar aprovação silenciosa". The floor was determined on 2026-09-21
# by running this gate itself (which skips comment lines) against the production tree:
#   bash scripts/check-write-containment.sh 2>&1 | grep "sítio(s) examinado(s)"
# → 157
# Note: a raw grep count of 158 was off by 1 because roadmap.go:727 has
# os.WriteFile inside a // comment — correctly skipped by this gate.
#
# If new files or old files are legitimately removed, update SITE_FLOOR in the
# same commit — do not leave it stale.
#
# Portability: bash + grep/sed only. No python3, no mapfile with interpolated
# paths (lessons from issues #307, #353, #363). Filesystem enumeration via find
# (not git ls-files) so that untracked new files are caught — git ls-files missed
# an untracked package in ML-1B-bis and the defect was invisible until commit.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

SITE_FLOOR=157

# ──────────────────────────────────────────────────────────────────────────────
# Self-test: 3 arms — unguarded-write REPROVA, marker-accepted PASSA,
# vacuous-scan (empty corpus) REPROVA
# ──────────────────────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--self-test" ]]; then
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT
  SELFTEST_OK=0

  echo "=== [self-test arm 1] write-containment/unguarded-write: os.WriteFile cru → gate REPROVA ==="
  ARM1="$TMP/arm1"
  mkdir -p "$ARM1/internal/pkg"
  cat > "$ARM1/internal/pkg/example.go" <<'GOEOF'
package pkg

import "os"

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
GOEOF
  if WRITE_CONTAINMENT_SCAN_DIR="$ARM1" bash "$REPO_ROOT/scripts/check-write-containment.sh" >/dev/null 2>&1; then
    echo "ERRO: gate deveria ter reprovado com os.WriteFile cru, mas saiu 0" >&2
    exit 1
  fi
  echo "OK: gate reprovou corretamente"
  SELFTEST_OK=$((SELFTEST_OK + 1))

  echo "=== [self-test arm 2] write-containment/marker-accepted: os.WriteFile com marcador → gate PASSA ==="
  ARM2="$TMP/arm2"
  mkdir -p "$ARM2/internal/pkg"
  cat > "$ARM2/internal/pkg/example.go" <<'GOEOF'
package pkg

import "os"

func writeFile(path string, data []byte) error {
	// write-containment-allowed: test fixture, fixed path not derived from user root
	return os.WriteFile(path, data, 0644)
}
GOEOF
  if ! WRITE_CONTAINMENT_SCAN_DIR="$ARM2" bash "$REPO_ROOT/scripts/check-write-containment.sh" >/dev/null 2>&1; then
    echo "ERRO: gate deveria ter passado com marcador presente, mas saiu != 0" >&2
    exit 1
  fi
  echo "OK: gate passou corretamente"
  SELFTEST_OK=$((SELFTEST_OK + 1))

  echo "=== [self-test arm 3] write-containment/vacuous-scan: corpus vazio → gate REPROVA ==="
  ARM3="$TMP/arm3"
  mkdir -p "$ARM3/internal"
  # No Go files — gate must fail with vacuity message
  if WRITE_CONTAINMENT_SCAN_DIR="$ARM3" bash "$REPO_ROOT/scripts/check-write-containment.sh" >/dev/null 2>&1; then
    echo "ERRO: gate deveria ter reprovado com corpus vazio, mas saiu 0" >&2
    exit 1
  fi
  echo "OK: gate reprovou com corpus vazio"
  SELFTEST_OK=$((SELFTEST_OK + 1))

  echo ""
  echo "self-test: 3/3 braços OK"
  exit 0
fi

# ──────────────────────────────────────────────────────────────────────────────
# Production scan
# ──────────────────────────────────────────────────────────────────────────────
SCAN_DIR="${WRITE_CONTAINMENT_SCAN_DIR:-$REPO_ROOT}"
OVERRIDE_MODE=0
if [[ -n "${WRITE_CONTAINMENT_SCAN_DIR:-}" ]]; then
  OVERRIDE_MODE=1
fi

FAIL=0
SITE_COUNT=0

# Enumerate production Go files via find (not git ls-files) so that untracked
# new files are caught. git ls-files missed untracked packages before (ML-1B-bis).
# Note: if running in override mode (self-test fixture), git add -N is NOT required.
mapfile -t GO_PROD_FILES < <(
  find "$SCAN_DIR/internal" -name '*.go' ! -name '*_test.go' 2>/dev/null | sort
)

FILE_COUNT=${#GO_PROD_FILES[@]}
if [[ "$FILE_COUNT" -eq 0 ]]; then
  echo "FAIL check-write-containment: nenhum arquivo Go de produção encontrado em $SCAN_DIR/internal" >&2
  echo "Corpus vazio — recusando reportar aprovação silenciosa." >&2
  exit 1
fi

echo "check-write-containment: varrendo $FILE_COUNT arquivo(s) de produção Go..."

# Pattern covers all write primitives. Note: os\.Create\( does NOT match
# os.CreateTemp( because the pattern requires ( immediately after Create —
# CreateTemp has 'Temp' between Create and (. Both are listed explicitly.
WRITE_PATTERN='os\.(WriteFile|Create|CreateTemp|OpenFile|Rename|MkdirAll)\('

for FILE in "${GO_PROD_FILES[@]}"; do
  MATCHES=$(grep -n -E "$WRITE_PATTERN" "$FILE" 2>/dev/null || true)
  [[ -z "$MATCHES" ]] && continue

  while IFS= read -r HIT; do
    [[ -z "$HIT" ]] && continue
    LINENUM="${HIT%%:*}"
    CONTENT="${HIT#*:}"

    # Skip comment lines (not a real write site)
    STRIPPED="${CONTENT#"${CONTENT%%[![:space:]]*}"}"
    case "$STRIPPED" in
      '//'*) continue ;;
    esac

    SITE_COUNT=$((SITE_COUNT + 1))

    # Check the site's own line for the marker
    JUSTIFIED=0
    if printf '%s' "$CONTENT" | grep -q "write-containment-allowed:"; then
      JUSTIFIED=1
    fi

    # Check the line immediately above (no window — exactly one line above)
    if [[ "$JUSTIFIED" -eq 0 ]] && [[ "$LINENUM" -gt 1 ]]; then
      PREVNO=$((LINENUM - 1))
      PREVLINE=$(sed -n "${PREVNO}p" "$FILE" 2>/dev/null || true)
      if printf '%s' "$PREVLINE" | grep -q "write-containment-allowed:"; then
        JUSTIFIED=1
      fi
    fi

    REL="${FILE#$SCAN_DIR/}"
    if [[ "$JUSTIFIED" -eq 1 ]]; then
      echo "OK   [write-containment] $REL:$LINENUM — justified inline"
    else
      echo "FAIL [write-containment] unjustified write at $REL:$LINENUM: $(printf '%s' "$CONTENT" | sed 's/^[[:space:]]*//' | cut -c1-120) — adicione '// write-containment-allowed: <razão>' na linha acima, ou roteie a escrita por pathguard"
      FAIL=1
    fi
  done <<< "$MATCHES"
done

echo ""
echo "check-write-containment: $SITE_COUNT sítio(s) examinado(s) em $FILE_COUNT arquivo(s)"

# Vacuity floor: only enforced in production mode (no override). Self-test
# fixtures legitimately scan fewer sites than the production codebase.
if [[ "$OVERRIDE_MODE" -eq 0 ]] && [[ "$SITE_COUNT" -lt "$SITE_FLOOR" ]]; then
  echo "FAIL check-write-containment: apenas $SITE_COUNT sítio(s) examinados, piso é $SITE_FLOOR — recusando reportar aprovação silenciosa" >&2
  echo "Se sítios foram removidos legitimamente, atualize SITE_FLOOR no mesmo commit." >&2
  exit 1
fi

echo ""
if [[ "$FAIL" -ne 0 ]]; then
  echo "check-write-containment: FAIL"
  exit 1
fi
echo "check-write-containment: OK — todos os sítios justificados ($SITE_COUNT examinados)"
exit 0
