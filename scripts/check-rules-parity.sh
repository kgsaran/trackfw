#!/usr/bin/env bash
# check-rules-parity.sh — proves the three CLI runtimes (Go, Node.js, Python)
# inject the exact same governance rules block, byte-for-byte identical, into
# the four auxiliary rules files created by `trackfw init --ai-tools`.
#
# ML-5G (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador).
#
# Diagnosis this gate closes: the rules block text injected into GEMINI.md,
# .github/copilot-instructions.md, .windsurfrules and
# .amazonq/developer/guidelines.md is hand-maintained in three separate
# generators (internal/generators/agentfiles.go, npm/src/generators/init.js,
# pypi/trackfw/generators/init_gen.py). Content drifted between them (missing
# `analyzing` state, missing ML-lifecycle item, missing Key Commands section,
# and a literal duplicated Architecture Directives block in Go) without any
# gate catching it, because scripts/check-identity-parity.sh only started
# exercising this path once ML-5E wired the injection into the catalog
# install flow — and that gate compares whole-project sha256 snapshots, so
# its failure output ("hash mismatch") does not point at the rules text
# specifically. This gate isolates the rules block on its own.
#
# Strategy: run `<cli> init --ai-tools <tools>` in a throwaway directory for
# each runtime, then diff the four resulting rules files byte-for-byte.
# `init` has no --scope flag (ADR D4/D1): non-interactively it always
# resolves to scope "global", writing the rules files under $HOME rather
# than the project directory — so each runtime gets an isolated $HOME here,
# mirroring check-identity-parity.sh. Follows the conventions of
# check-slash-parity.sh:
# set -euo pipefail, mktemp -d fixtures with a cleanup trap, a vacuity guard
# before comparing, "OK [scenario/name]" on success, and accumulating all
# drift before exiting instead of failing on the first mismatch.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-rules-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-rules-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/go" "$WORK/node" "$WORK/python"

TOOLS="gemini,copilot,windsurf,amazonq"

mkdir -p "$WORK/home-go" "$WORK/home-node" "$WORK/home-python"

# Auxiliary rules files are always written relative to the project cwd
# (generators.InjectRulesForTool(tool, cwd) — see internal/commands/init.go:
# installAITools), independent of the agents/skills catalog scope, which is
# what resolves to "global"/$HOME non-interactively. Isolated $HOME per
# runtime avoids polluting (or reading stale identity from) the real user
# home while still exercising the real non-TTY code path.
(cd "$WORK/go" && HOME="$WORK/home-go" "$GO_BIN" init --ai-tools "$TOOLS" >/dev/null)
(cd "$WORK/node" && HOME="$WORK/home-node" node "$ROOT_DIR/npm/bin/trackfw" init --ai-tools "$TOOLS" >/dev/null)
(cd "$WORK/python" && HOME="$WORK/home-python" PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw init --ai-tools "$TOOLS" >/dev/null)

# Files relative to the project root, one per --ai-tools entry above.
RULES_FILES=(
  "GEMINI.md"
  ".github/copilot-instructions.md"
  ".windsurfrules"
  ".amazonq/developer/guidelines.md"
)

FAIL=0

# ---------------------------------------------------------------------------
# Vacuity guard — the four rules files must exist and be non-empty in all
# three runtimes before any comparison. Without this, three missing files
# would "diff" as identical (nothing to compare) and the gate would pass
# vacuously.
# ---------------------------------------------------------------------------
for RUNTIME in go node python; do
  for FILE in "${RULES_FILES[@]}"; do
    PATH_CANDIDATE="$WORK/$RUNTIME/$FILE"
    if [[ ! -s "$PATH_CANDIDATE" ]]; then
      echo "rules parity drift: $FILE missing or empty ($RUNTIME) — vacuity guard failed" >&2
      FAIL=1
    fi
  done
done

if [[ "$FAIL" -ne 0 ]]; then
  echo "check-rules-parity: vacuity guard failed — generation incomplete, comparison aborted" >&2
  exit 1
fi

echo "OK   [rules-parity/vacuity-guard]"

# ---------------------------------------------------------------------------
# Byte-for-byte comparison of each rules file across the three runtimes.
# ---------------------------------------------------------------------------
for FILE in "${RULES_FILES[@]}"; do
  GO_FILE="$WORK/go/$FILE"
  NODE_FILE="$WORK/node/$FILE"
  PY_FILE="$WORK/python/$FILE"

  if ! cmp -s "$GO_FILE" "$NODE_FILE"; then
    echo "rules parity drift: $FILE differs between go and node" >&2
    diff -u "$GO_FILE" "$NODE_FILE" >&2 || true
    FAIL=1
  fi
  if ! cmp -s "$GO_FILE" "$PY_FILE"; then
    echo "rules parity drift: $FILE differs between go and python" >&2
    diff -u "$GO_FILE" "$PY_FILE" >&2 || true
    FAIL=1
  fi
done

if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi

echo "OK   [rules-parity/three-runtimes-identical]"
echo "Rules block parity checks passed (${#RULES_FILES[@]} files x 3 runtimes)."
