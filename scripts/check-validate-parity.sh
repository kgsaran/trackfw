#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-validate-parity.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

# $HOME isolado por padrão para o script inteiro — nunca o real. Sem isto, o bloco de parity
# original (ADR/REQ fixture) também enxergaria o escopo GLOBAL de guards de quem roda o gate desde
# ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-
# fiacao (ML-3A) — mesmo precedente do Cenário 46/check-artifact-parity.sh/check-barrier.sh. O
# bloco de global-scope guard integrity mais abaixo usa seu PRÓPRIO $HOME dedicado
# ($GVP_HOME) para controlar precisamente o que está instalado ali.
#
# GOPATH/GOMODCACHE fixados nos valores REAIS antes de isolar $HOME: `go build` abaixo resolve os
# dois a partir de $HOME por padrão (GOCACHE já era fixo, via /tmp/trackfw-go-cache) — sem isso,
# um $HOME sintético novo a cada run forçaria rebaixar o módulo inteiro.
export GOPATH="${GOPATH:-$(go env GOPATH)}"
export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"
export HOME="$TMP_DIR/home"
mkdir -p "$HOME"

mkdir -p \
  "$TMP_DIR/project/docs/adr" \
  "$TMP_DIR/project/docs/req" \
  "$TMP_DIR/project/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}

cat >"$TMP_DIR/project/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

cat >"$TMP_DIR/project/docs/roadmaps/wip/RM.md" <<'EOF'
---
status: WIP
---
# Roadmap without required governance links
EOF

# ADR não aceito (Status: Proposed) referenciado por REQ Done + REQ Open bloqueada
# pelo mesmo ADR — sem esta fixture, `adr_accepted_when_req_done` e
# `blocked_by_draft_adr` nunca apareciam no corpus comparado abaixo: o guard de
# vacuidade só olha o TOTAL de violações (não por regra), então um CLI poderia
# perder qualquer uma das duas regras e este gate continuaria verde
# (vault/notes/deteccao-de-status-de-adr-divergencias-entre-clis-2026-08-01.md).
cat >"$TMP_DIR/project/docs/adr/ADR-proposed-fixture.md" <<'EOF'
---
status: Proposed
date: 2026-08-01
author: ""
---

# ADR: fixture

> Date: 2026-08-01 | Status: Proposed

## Context
ctx

## Decision
decision
EOF

cat >"$TMP_DIR/project/docs/req/REQ-done-fixture.md" <<'EOF'
---
status: Done
date: 2026-08-01
author: ""
adr: "docs/adr/ADR-proposed-fixture.md"
roadmap: ""
---

# REQ: fixture

> Date: 2026-08-01 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: docs/adr/ADR-proposed-fixture.md

## Linked Roadmap
Roadmap:
EOF

cat >"$TMP_DIR/project/docs/req/REQ-blocked-fixture.md" <<'EOF'
---
status: Open
date: 2026-08-01
author: ""
adr: ""
roadmap: ""
---

# REQ: bloqueada

> Date: 2026-08-01 | Status: Open

## Motivation
motivo

## Acceptance Criteria
- [ ] pendente

## Linked ADR
ADR:

## Blocked by ADRs
- ADR-proposed-fixture.md (Proposed)

## Linked Roadmap
Roadmap:
EOF

GOCACHE=${GOCACHE:-/tmp/trackfw-go-cache} go build -o "$TMP_DIR/trackfw-go" ./cmd/trackfw

run_validator() {
  local output=$1
  shift
  set +e
  (
    cd "$TMP_DIR/project"
    "$@"
  ) >"$output" 2>"$output.stderr"
  local status=$?
  set -e
  if [[ $status -ne 1 ]]; then
    echo "Expected validation exit code 1, got $status for $*" >&2
    return 1
  fi
}

run_validator "$TMP_DIR/go.json" "$TMP_DIR/trackfw-go" validate --json
run_validator "$TMP_DIR/node.json" node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_validator "$TMP_DIR/python.json" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

python3 - "$TMP_DIR/go.json" "$TMP_DIR/node.json" "$TMP_DIR/python.json" <<'PY'
import json
import sys

def contract(path):
    with open(path, encoding="utf-8") as stream:
        payload = json.load(stream)
    return {
        "summary": payload["summary"],
        "violations": sorted(
            (item.get("rule"), item.get("file")) for item in payload["violations"]
        ),
        "warnings": sorted(
            (item.get("rule"), item.get("file")) for item in payload["warnings"]
        ),
    }

contracts = [contract(path) for path in sys.argv[1:]]

# P2 vacuity guard: if all three validators produce zero violations their
# outputs trivially match — a silent pass when the test fixture is broken.
# The run_validator() guard (exit code must be 1) catches the case where the
# validator exits 0, but it does not catch a validator that exits 1 with an
# empty violations list, so we check here explicitly.
if not contracts[0]["violations"]:
    raise SystemExit(
        "validate parity: go output has no violations — "
        "the test fixture may be wrong (vacuous check)"
    )

# P2 vacuity guard, per-rule: a total-count check alone would still pass if a
# CLI silently dropped one specific rule while others kept firing (masking a
# real regression instead of catching it). Require both rules exercised by
# the ADR/REQ fixture above to be present in every runtime's output.
expected_rules = {"adr_accepted_when_req_done", "blocked_by_draft_adr"}
for path, value in zip(sys.argv[1:], contracts):
    got_rules = {rule for rule, _file in value["violations"]}
    missing = expected_rules - got_rules
    if missing:
        raise SystemExit(
            f"validate parity: {path} is missing expected violation rule(s) "
            f"{sorted(missing)} — the fixture no longer exercises them or a "
            "CLI regressed (vacuous check)"
        )

if contracts[1:] != contracts[:-1]:
    for path, value in zip(sys.argv[1:], contracts):
        print(path, json.dumps(value, indent=2), file=sys.stderr)
    raise SystemExit("validate JSON contract differs between runtimes")
PY

echo "Validate JSON parity checks passed"

# ---------------------------------------------------------------------------
# ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-
# fiacao, ML-3A: the "git_branch_guard_script_integrity"/"credential_guard_script_integrity"
# GLOBAL-scope message is written by hand in 3 places (Go, Node, Python — Python's version is
# assembled from f-string fragments joined mid-sentence). The check above only compares (rule,
# file) tuples — NOT message text — so it would stay green even if the 3 messages diverged in
# wording, as long as the (rule, file) pair matched. This block closes that specific gap: it
# forces the rule to fire in all 3 real CLIs (via a corrupted, unwired script under an isolated
# $HOME — same fixture shape as the Cenário 68 discriminant in check-gates-falsify.sh) and asserts
# the exact warning MESSAGE is byte-identical across Go/Node/Python, not just its (rule, file) key.
#
# $HOME isolado por fixture, nunca o real (mesmo precedente do Cenário 46).
# ---------------------------------------------------------------------------
# $TMP_DIR pode conter "//" embutido quando $TMPDIR (macOS) termina em "/" — mktemp então gera
# ".../T//trackfw-validate-parity.XXXXXX". filepath.Join (Go) e path.join (Node) colapsam essa
# barra dupla ao montar o caminho absoluto do script; os.path.join (Python) NÃO colapsa barras já
# embutidas dentro de um dos argumentos — só entre eles — então a mensagem Python preservaria o
# "//" cru enquanto Go/Node o normalizariam, quebrando a comparação byte-a-byte por um artefato do
# fixture, não do produto (mesmo achado do Cenário 67/ML-2C:
# `internal/generators/guard_path_normalize_test.go`). Normalizado aqui para não confundir esse
# ruído de fixture com uma regressão real de paridade de mensagem.
GVP_TMP_CLEAN=$(printf '%s' "$TMP_DIR" | sed 's#//*#/#g')
GVP_HOME="$GVP_TMP_CLEAN/global-integrity-home"
mkdir -p "$GVP_HOME/.trackfw/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$GVP_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
chmod +x "$GVP_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
# Nenhum config global referencia o script — a fiação é irrelevante para esta regra desde o ML-3A.

GVP_PROJECT="$TMP_DIR/global-integrity-project"
mkdir -p \
  "$GVP_PROJECT/docs/adr" \
  "$GVP_PROJECT/docs/req" \
  "$GVP_PROJECT/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}
cat >"$GVP_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

run_global_integrity() {
  local output=$1
  shift
  (cd "$GVP_PROJECT" && HOME="$GVP_HOME" "$@") >"$output" 2>"$output.stderr"
}

run_global_integrity "$TMP_DIR/gvp-go.json" "$TMP_DIR/trackfw-go" validate --json
run_global_integrity "$TMP_DIR/gvp-node.json" node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_global_integrity "$TMP_DIR/gvp-python.json" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

python3 - "$TMP_DIR/gvp-go.json" "$TMP_DIR/gvp-node.json" "$TMP_DIR/gvp-python.json" <<'PY'
import json
import sys

def warning_messages(path):
    with open(path, encoding="utf-8") as stream:
        payload = json.load(stream)
    return sorted(
        item["message"] for item in payload["warnings"]
        if item.get("rule") == "git_branch_guard_script_integrity"
    )

msgs = [warning_messages(path) for path in sys.argv[1:]]

# P2 vacuity guard: prove the rule actually fired in all 3 — an empty list in any runtime means
# this block is comparing nothing, not proving parity.
for path, value in zip(sys.argv[1:], msgs):
    if not value:
        raise SystemExit(
            f"validate parity (global script integrity): {path} produced ZERO "
            "git_branch_guard_script_integrity warnings — fixture is vacuous, or this CLI "
            "regressed the existence-based trigger (ML-3A)"
        )

if msgs[1:] != msgs[:-1]:
    for path, value in zip(sys.argv[1:], msgs):
        print(path, json.dumps(value, indent=2), file=sys.stderr)
    raise SystemExit(
        "validate parity: git_branch_guard_script_integrity GLOBAL-scope warning message text "
        "differs between runtimes (byte-for-byte comparison, not just rule+file)"
    )
PY

echo "Validate JSON parity checks passed (global-scope guard integrity message, byte-identical across 3 CLIs)"
