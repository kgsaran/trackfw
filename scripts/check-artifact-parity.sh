#!/usr/bin/env bash
# check-artifact-parity.sh — Verifica que os 3 CLIs (Go, Node.js, Python)
# geram artefatos byte-a-byte idênticos (conteúdo e nome de arquivo).
#
# Artefatos verificados: req, adr, roadmap, slash-command roadmap,
# note + vault/notes/index.md. O gate também executa o ciclo E2E
# backlog → analyzing em layout flat e by_agent nos três runtimes.
#
# Título utilizado: contém acento e cedilha para validar a normalização NFKD
# de slug portável nos 3 runtimes (REQ-2026-07-27-convergencia-templates-python).
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
# Garantir caminho absoluto — Makefile pode passar um path relativo (ex: bin/trackfw)
# que ficaria inválido ao fazer `cd "$WORK/<runtime>"` dentro das subshells.
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-artifact-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/go" "$WORK/node" "$WORK/python"

TITLE="Autenticação e Sessão"
FLAG_TITLE="Integração de Pagamentos"
REQ_FLAG_REL="docs/req/REQ-flag-source.md"
FROM_REQ_TITLE="Fluxo de Pagamentos"

# ── Midnight rollover guard ──────────────────────────────────────────────────
# Captura a data ANTES da geração. Se a data mudar durante o processo os nomes
# de arquivo serão inconsistentes entre runtimes — falha explícita em vez de
# diagnóstico misterioso de diff.
DATE_BEFORE=$(date +%F)

# ── Geração dos artefatos ────────────────────────────────────────────────────
(cd "$WORK/go" && "$GO_BIN"                                       init)
(cd "$WORK/go" && "$GO_BIN"                                       req     new "$TITLE")
(cd "$WORK/go" && "$GO_BIN"                                       adr     new "$TITLE")
(cd "$WORK/go" && "$GO_BIN"                                       roadmap new "$TITLE")
(cd "$WORK/go" && "$GO_BIN"                                       note    new "$TITLE")
cat >"$WORK/go/$REQ_FLAG_REL" <<'EOF'
---
status: Open
date: 2026-07-27
author: ""
adr: ""
roadmap: ""
---

# REQ: Fluxo de Pagamentos

> Date: 2026-07-27 | Status: Open

## Acceptance Criteria
- [ ] Create payment intent
- [ ] Confirm payment status
EOF
(cd "$WORK/go" && "$GO_BIN"                                       roadmap new --title "$FLAG_TITLE" --req "$REQ_FLAG_REL")
(cd "$WORK/go" && "$GO_BIN"                                       roadmap new --from-req "$REQ_FLAG_REL")

(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              init)
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              req     new "$TITLE")
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              adr     new "$TITLE")
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              roadmap new "$TITLE")
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              note    new "$TITLE")
cat >"$WORK/node/$REQ_FLAG_REL" <<'EOF'
---
status: Open
date: 2026-07-27
author: ""
adr: ""
roadmap: ""
---

# REQ: Fluxo de Pagamentos

> Date: 2026-07-27 | Status: Open

## Acceptance Criteria
- [ ] Create payment intent
- [ ] Confirm payment status
EOF
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              roadmap new --title "$FLAG_TITLE" --req "$REQ_FLAG_REL")
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              roadmap new --from-req "$REQ_FLAG_REL")

(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw init)
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw req     new "$TITLE")
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw adr     new "$TITLE")
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw roadmap new "$TITLE")
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw note    new "$TITLE")
cat >"$WORK/python/$REQ_FLAG_REL" <<'EOF'
---
status: Open
date: 2026-07-27
author: ""
adr: ""
roadmap: ""
---

# REQ: Fluxo de Pagamentos

> Date: 2026-07-27 | Status: Open

## Acceptance Criteria
- [ ] Create payment intent
- [ ] Confirm payment status
EOF
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw roadmap new --title "$FLAG_TITLE" --req "$REQ_FLAG_REL")
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw roadmap new --from-req "$REQ_FLAG_REL")

DATE_AFTER=$(date +%F)
if [[ "$DATE_BEFORE" != "$DATE_AFTER" ]]; then
  echo "check-artifact-parity: data rolou durante a geração ($DATE_BEFORE → $DATE_AFTER)" >&2
  echo "  Reexecute o gate." >&2
  exit 1
fi

DATE="$DATE_AFTER"
SLUG="autenticacao-e-sessao"

# ── Caminhos esperados por tipo ──────────────────────────────────────────────
# EXPECTED_<KIND> é o caminho relativo dentro de cada WORK/<runtime>/.
# O vacuity guard usa esses paths para garantir que cada runtime gerou exatamente
# o arquivo esperado — zero arquivos → falha explícita, nunca passe trivial.
EXPECTED_REQ="docs/req/REQ-${DATE}-${SLUG}.md"
EXPECTED_ADR="docs/adr/ADR-${DATE}-${SLUG}.md"
EXPECTED_ROADMAP="docs/roadmaps/backlog/ROADMAP-${DATE}-${SLUG}.md"
EXPECTED_ROADMAP_FLAGS="docs/roadmaps/backlog/ROADMAP-${DATE}-integracao-de-pagamentos.md"
EXPECTED_ROADMAP_FROM_REQ="docs/roadmaps/backlog/ROADMAP-${DATE}-fluxo-de-pagamentos.md"
EXPECTED_SLASH_ROADMAP=".claude/commands/trackfw/roadmap.md"
EXPECTED_NOTE="vault/notes/${SLUG}-${DATE}.md"
EXPECTED_INDEX="vault/notes/index.md"

KINDS=("req" "adr" "roadmap" "roadmap_flags" "roadmap_from_req" "slash_roadmap" "note" "note_index")

expected_path() {
  case "$1" in
    req)        echo "$EXPECTED_REQ"     ;;
    adr)        echo "$EXPECTED_ADR"     ;;
    roadmap)    echo "$EXPECTED_ROADMAP" ;;
    roadmap_flags) echo "$EXPECTED_ROADMAP_FLAGS" ;;
    roadmap_from_req) echo "$EXPECTED_ROADMAP_FROM_REQ" ;;
    slash_roadmap) echo "$EXPECTED_SLASH_ROADMAP" ;;
    note)       echo "$EXPECTED_NOTE"    ;;
    note_index) echo "$EXPECTED_INDEX"   ;;
  esac
}

# ── Vacuity guard ────────────────────────────────────────────────────────────
FAIL=0
for KIND in "${KINDS[@]}"; do
  REL=$(expected_path "$KIND")
  for RUNTIME in go node python; do
    TARGET="$WORK/$RUNTIME/$REL"
    if [[ ! -f "$TARGET" ]]; then
      echo "artifact parity drift: $KIND ($RUNTIME) — arquivo ausente: $REL" >&2
      FAIL=1
    fi
  done
done

if [[ $FAIL -ne 0 ]]; then
  echo "check-artifact-parity: vacuity guard falhou — geração incompleta, comparação abortada" >&2
  exit 1
fi

# ── Comparação conteúdo e nome de arquivo ────────────────────────────────────
# Nome: os paths relativos dentro de WORK/<runtime> são idênticos por construção
# (todos usam a mesma EXPECTED_<KIND>). O vacuity guard acima já confirmou que
# cada arquivo existe no path exato esperado — a divergência de nome de arquivo
# é detectada quando o runtime gera um path diferente do esperado (vacuity falha).
#
# Conteúdo: diff byte-a-byte acumulando todos os erros antes de sair,
# para que o diagnóstico cubra todos os artefatos divergentes de uma vez.
for KIND in "${KINDS[@]}"; do
  REL=$(expected_path "$KIND")
  GO_FILE="$WORK/go/$REL"
  NODE_FILE="$WORK/node/$REL"
  PY_FILE="$WORK/python/$REL"

  if ! diff -q "$GO_FILE" "$NODE_FILE" >/dev/null 2>&1; then
    echo "artifact parity drift: $KIND (go vs node)" >&2
    diff "$GO_FILE" "$NODE_FILE" >&2 || true
    FAIL=1
  fi

  if ! diff -q "$GO_FILE" "$PY_FILE" >/dev/null 2>&1; then
    echo "artifact parity drift: $KIND (go vs python)" >&2
    diff "$GO_FILE" "$PY_FILE" >&2 || true
    FAIL=1
  fi
done

if [[ $FAIL -ne 0 ]]; then
  exit 1
fi

run_trackfw() {
  local runtime=$1
  local dir=$2
  shift 2
  case "$runtime" in
    go)     (cd "$dir" && "$GO_BIN" "$@") ;;
    node)   (cd "$dir" && node "$ROOT_DIR/npm/bin/trackfw" "$@") ;;
    python) (cd "$dir" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw "$@") ;;
    *)      echo "check-artifact-parity: runtime desconhecido: $runtime" >&2; return 1 ;;
  esac
}

assert_quoted_status_validate() {
  local runtime=$1
  local dir="$WORK/quoted-status-$runtime"

  rm -rf "$dir"
  mkdir -p "$dir/docs/adr" "$dir/docs/req" "$dir/docs/roadmaps/wip"
  cat >"$dir/trackfw.yaml" <<'YAML'
project_name: quoted-status
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
governance_mode: strict
rules:
  folder_status: warning
YAML
  cat >"$dir/docs/adr/ADR-quoted.md" <<'EOF'
---
status: Accepted
date: 2026-07-27
author: ""
---

# ADR: Quoted
EOF
  cat >"$dir/docs/req/REQ-quoted.md" <<'EOF'
---
status: Open
date: 2026-07-27
adr: "docs/adr/ADR-quoted.md"
roadmap: "docs/roadmaps/wip/ROADMAP-quoted.md"
---

# REQ: Quoted

ADR: docs/adr/ADR-quoted.md
Roadmap: docs/roadmaps/wip/ROADMAP-quoted.md
EOF
  cat >"$dir/docs/roadmaps/wip/ROADMAP-quoted.md" <<'EOF'
---
status: "wip"
date: 2026-07-27
req: "docs/req/REQ-quoted.md"
squad: ""
---

# Roadmap: Quoted

> Created: 2026-07-27 | Status: "wip"

## Context
REQ: docs/req/REQ-quoted.md

## Wave 1 — Test
> Dependencies: none

### ML-1A — Test
**Status:** pending
**Files affected:**
**Actions:**

## Acceptance Criteria
- [ ] validate passes
EOF

  validate_out=$(run_trackfw "$runtime" "$dir" validate --json)
  if grep -q "folder_status" <<<"$validate_out"; then
    echo "artifact parity quoted-status failed: $runtime — validate reportou folder_status para status aspeado" >&2
    echo "$validate_out" >&2
    exit 1
  fi
  if ! python3 -c 'import json,sys; p=json.load(sys.stdin); s=p["summary"]; sys.exit(0 if s["violations"] == 0 and s["warnings"] == 0 else 1)' <<<"$validate_out"; then
    echo "artifact parity quoted-status failed: $runtime — validate não retornou 0/0" >&2
    echo "$validate_out" >&2
    exit 1
  fi
}

write_cycle_project() {
  local dir=$1
  local layout=$2
  local roadmap_rel

  rm -rf "$dir"
  mkdir -p "$dir/docs/adr" "$dir/docs/req"

  if [[ "$layout" == "by_agent" ]]; then
    roadmap_rel="docs/roadmaps/zeus/analyzing/ROADMAP-cycle-analyzing.md"
    mkdir -p "$dir/docs/roadmaps/zeus/backlog" "$dir/docs/roadmaps/zeus/analyzing" \
             "$dir/docs/roadmaps/zeus/wip" "$dir/docs/roadmaps/zeus/blocked" \
             "$dir/docs/roadmaps/zeus/done" "$dir/docs/roadmaps/zeus/abandoned"
    cat >"$dir/trackfw.yaml" <<'YAML'
project_name: parity-cycle
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
  - zeus
wip_limit: 1
governance_mode: strict
rules:
  folder_status: warning
YAML
    roadmap_start="$dir/docs/roadmaps/zeus/backlog/ROADMAP-cycle-analyzing.md"
  else
    roadmap_rel="docs/roadmaps/analyzing/ROADMAP-cycle-analyzing.md"
    mkdir -p "$dir/docs/roadmaps/backlog" "$dir/docs/roadmaps/analyzing" \
             "$dir/docs/roadmaps/wip" "$dir/docs/roadmaps/blocked" \
             "$dir/docs/roadmaps/done" "$dir/docs/roadmaps/abandoned"
    cat >"$dir/trackfw.yaml" <<'YAML'
project_name: parity-cycle
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
wip_limit: 1
governance_mode: strict
rules:
  folder_status: warning
YAML
    roadmap_start="$dir/docs/roadmaps/backlog/ROADMAP-cycle-analyzing.md"
  fi

  cat >"$dir/docs/adr/ADR-cycle.md" <<'EOF'
---
status: Accepted
date: 2026-07-27
author: ""
---

# ADR: Cycle

> Date: 2026-07-27 | Status: Accepted
EOF

  cat >"$dir/docs/req/REQ-cycle.md" <<EOF
---
status: Open
date: 2026-07-27
adr: "docs/adr/ADR-cycle.md"
roadmap: "$roadmap_rel"
---

# REQ: Cycle

> Date: 2026-07-27 | Status: Open

ADR: docs/adr/ADR-cycle.md
Roadmap: ${roadmap_rel}
EOF

  cat >"$roadmap_start" <<'EOF'
---
status: backlog
date: 2026-07-27
req: "docs/req/REQ-cycle.md"
squad: ""
---

# Roadmap: Cycle

> Created: 2026-07-27 | Status: backlog

## Context
REQ: docs/req/REQ-cycle.md
ADR: docs/adr/ADR-cycle.md

Roadmap: docs/roadmaps/backlog/ROADMAP-cycle-analyzing.md

## Wave 1 — Test
> Dependencies: none

### ML-1A — Test
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] validate passes
EOF
}

assert_cycle() {
  local runtime=$1
  local layout=$2
  local dir="$WORK/cycle-$layout-$runtime"
  local moved_rel

  write_cycle_project "$dir" "$layout"
  run_trackfw "$runtime" "$dir" roadmap move cycle-analyzing analyzing >/dev/null

  if [[ "$layout" == "by_agent" ]]; then
    moved_rel="docs/roadmaps/zeus/analyzing/ROADMAP-cycle-analyzing.md"
    log_path="$dir/docs/roadmaps/.trackfw-log"
    log_name="zeus/ROADMAP-cycle-analyzing.md"
  else
    moved_rel="docs/roadmaps/analyzing/ROADMAP-cycle-analyzing.md"
    log_path="$dir/docs/roadmaps/.trackfw-log"
    log_name="ROADMAP-cycle-analyzing.md"
  fi

  moved="$dir/$moved_rel"
  if [[ ! -f "$moved" ]]; then
    echo "artifact parity cycle failed: $runtime/$layout — roadmap ausente em analyzing: $moved_rel" >&2
    exit 1
  fi
  if [[ -e "$dir/${moved_rel/analyzing/backlog}" ]]; then
    echo "artifact parity cycle failed: $runtime/$layout — roadmap ainda existe em backlog" >&2
    exit 1
  fi
  if ! grep -q '^status: analyzing$' "$moved"; then
    echo "artifact parity cycle failed: $runtime/$layout — frontmatter não ficou em analyzing" >&2
    exit 1
  fi
  if ! grep -q '^> Created: 2026-07-27 | Status: analyzing$' "$moved"; then
    echo "artifact parity cycle failed: $runtime/$layout — header não ficou em analyzing" >&2
    exit 1
  fi
  if [[ ! -f "$log_path" ]] || ! grep -qF "$log_name" "$log_path" || ! grep -qF "backlog → analyzing" "$log_path"; then
    echo "artifact parity cycle failed: $runtime/$layout — .trackfw-log não registrou backlog → analyzing" >&2
    exit 1
  fi

  validate_out=$(run_trackfw "$runtime" "$dir" validate --json)
  if grep -q "folder_status" <<<"$validate_out"; then
    echo "artifact parity cycle failed: $runtime/$layout — validate reportou folder_status" >&2
    echo "$validate_out" >&2
    exit 1
  fi
}

for RUNTIME in go node python; do
  assert_cycle "$RUNTIME" flat
  assert_cycle "$RUNTIME" by_agent
  assert_quoted_status_validate "$RUNTIME"
done

echo "Artifact parity checks passed (8 artifact types × 3 runtimes; roadmap flags, quoted status, analyzing cycle flat/by_agent)"
