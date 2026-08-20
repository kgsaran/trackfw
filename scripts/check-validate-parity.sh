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

# GO_BIN override — segue a convenção de check-branch-new-parity.sh/check-artifact-parity.sh:
# não fixado → compila um binário descartável (comportamento original, preservado);
# relativo → prefixa com ROOT_DIR; absoluto → usado como está. Sem isto o Cenário P4
# (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco, ML-2A) não teria como
# apontar este script para um binário Go sabotado em cópia isolada.
if [[ -z "${GO_BIN:-}" ]]; then
  GOCACHE=${GOCACHE:-/tmp/trackfw-go-cache} go build -o "$TMP_DIR/trackfw-go" ./cmd/trackfw
  GO_BIN="$TMP_DIR/trackfw-go"
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi

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

run_validator "$TMP_DIR/go.json" "$GO_BIN" validate --json
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

run_global_integrity "$TMP_DIR/gvp-go.json" "$GO_BIN" validate --json
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

# ---------------------------------------------------------------------------
# ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-
# fiacao, ML-4B: same gap as the block above, this time for the NEW "hook entry is missing
# \"type\":\"command\"" message "git_branch_guard_hook_resolvable" (GLOBAL scope) emits since
# ML-4B — hades-tf's ML-4A barrier finding reproduced: a global config entry with the CORRECT
# command but MISSING "type":"command" (script present and íntegro) is silently never executed by
# Claude Code, and this rule now reports it. Written by hand in 3 places (Go's
# validateGuardGlobalHookResolvable, Node's validateGuardGlobalHookResolvable, Python's
# validate_guard_global_hook_resolvable) — this block proves the wording is byte-identical, not
# just the (rule, file) key.
#
# Unlike git_branch_guard_script_integrity (default severity "warning"), this rule's default
# severity is "error" (falls through in ruleSeverity — see internal/validator/validator.go's
# comment on git_branch_guard_hook_resolvable), so `validate --json` exits non-zero here; each
# invocation is wrapped in set +e/-e to survive that under this script's `set -euo pipefail`.
#
# $HOME isolado por fixture, nunca o real (mesmo precedente do Cenário 46).
# ---------------------------------------------------------------------------
GVMT_TMP_CLEAN=$(printf '%s' "$TMP_DIR" | sed 's#//*#/#g')
GVMT_HOME="$GVMT_TMP_CLEAN/global-missing-type-home"
mkdir -p "$GVMT_HOME/.claude" "$GVMT_HOME/.trackfw/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$GVMT_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
chmod +x "$GVMT_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
# Entrada CORRETA no comando, mas SEM "type":"command" -- o achado central do ML-4B.
cat >"$GVMT_HOME/.claude/settings.json" <<EOF
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"$GVMT_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"}]}]}}
EOF

GVMT_PROJECT="$TMP_DIR/global-missing-type-project"
mkdir -p \
  "$GVMT_PROJECT/docs/adr" \
  "$GVMT_PROJECT/docs/req" \
  "$GVMT_PROJECT/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}
cat >"$GVMT_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

run_global_missing_type() {
  local output=$1
  shift
  set +e
  (cd "$GVMT_PROJECT" && HOME="$GVMT_HOME" "$@") >"$output" 2>"$output.stderr"
  set -e
}

run_global_missing_type "$TMP_DIR/gvmt-go.json" "$GO_BIN" validate --json
run_global_missing_type "$TMP_DIR/gvmt-node.json" node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_global_missing_type "$TMP_DIR/gvmt-python.json" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

python3 - "$TMP_DIR/gvmt-go.json" "$TMP_DIR/gvmt-node.json" "$TMP_DIR/gvmt-python.json" <<'PY'
import json
import sys

def violation_messages(path):
    with open(path, encoding="utf-8") as stream:
        payload = json.load(stream)
    return sorted(
        item["message"] for item in payload["violations"]
        if item.get("rule") == "git_branch_guard_hook_resolvable"
    )

msgs = [violation_messages(path) for path in sys.argv[1:]]

# P2 vacuity guard: prove the rule actually fired in all 3.
for path, value in zip(sys.argv[1:], msgs):
    if not value:
        raise SystemExit(
            f"validate parity (global missing-type hook_resolvable): {path} produced ZERO "
            "git_branch_guard_hook_resolvable violations — fixture is vacuous, or this CLI "
            "regressed the structural-validity check (ML-4B)"
        )
    for text in value:
        if "does not exist" in text or "not executable" in text:
            raise SystemExit(
                f"validate parity (global missing-type hook_resolvable): {path} reported the "
                f"wrong diagnostic ({text!r}) -- the script exists and is executable, the "
                "fixture's defect is the missing \"type\":\"command\" field, not existence"
            )

if msgs[1:] != msgs[:-1]:
    for path, value in zip(sys.argv[1:], msgs):
        print(path, json.dumps(value, indent=2), file=sys.stderr)
    raise SystemExit(
        "validate parity: git_branch_guard_hook_resolvable GLOBAL-scope missing-\"type\" warning "
        "message text differs between runtimes (byte-for-byte comparison, not just rule+file)"
    )
PY

echo "Validate JSON parity checks passed (global-scope guard missing-type hook_resolvable message, byte-identical across 3 CLIs)"

# ---------------------------------------------------------------------------
# ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco, ML-2A: a regra
# `branch_has_wip_roadmap` aceita roadmap correspondente em `done/`, não só em
# `wip/`, desde a REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade — mas
# esse comportamento nunca tinha sido exercitado cross-CLI (nem aqui, nem em
# check-branch-new-parity.sh, cujos fixtures diziam literalmente "wip/ and done/
# deliberately left empty"). Três casos, orientados por `TRACKFW_BRANCH` — suportado
# de forma idêntica pelos 3 CLIs (internal/validator/validator.go:
# validateBranchHasWIPRoadmap, npm/src/validator/index.js, pypi/trackfw/validator.py:
# validate_branch_has_wip_roadmap), o que dispensa `git checkout` real:
#   1. roadmap em done/ com slug IGUAL     → SEM violação (aceito — o caso central
#      que esta seção nunca provou)
#   2. nenhum roadmap em wip/ nem done/    → violação "no roadmap is in wip/ nor
#      done/" (não-regressão do caso já conhecido)
#   3. roadmap em done/ com slug DIFERENTE → violação "no matching roadmap in wip/
#      nor done/" (discriminante: sem este caso, um gate que aceitasse QUALQUER
#      roadmap em done/ — não só o de slug correspondente — passaria por acidente)
# Cada braço só compara as mensagens da regra `branch_has_wip_roadmap` filtradas do
# JSON (não o payload inteiro) — outras regras podem disparar de forma diferente
# nesta fixture mínima sem invalidar a prova, que é especificamente sobre esta regra.
# ---------------------------------------------------------------------------
mkdir -p \
  "$TMP_DIR/bhr-match/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-nomatch/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-diff/docs/roadmaps"/{wip,done}

for d in bhr-match bhr-nomatch bhr-diff; do
  cat >"$TMP_DIR/$d/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
done

cat >"$TMP_DIR/bhr-match/docs/roadmaps/done/ROADMAP-2026-08-20-minha-feature.md" <<'EOF'
---
status: done
---
# Roadmap: minha feature
EOF

cat >"$TMP_DIR/bhr-diff/docs/roadmaps/done/ROADMAP-2026-08-20-outra-coisa.md" <<'EOF'
---
status: done
---
# Roadmap: outra coisa
EOF
# bhr-nomatch: wip/ e done/ deliberadamente vazios — nenhum roadmap em lugar nenhum.

run_bhr() {
  local output=$1 dir=$2 branch=$3
  shift 3
  set +e
  ( cd "$dir" && TRACKFW_BRANCH="$branch" "$@" ) >"$output" 2>"$output.stderr"
  echo "$?" >"$output.exit"
  set -e
}

run_bhr "$TMP_DIR/bhr-match-go.json"     "$TMP_DIR/bhr-match"   feat/minha-feature       "$GO_BIN" validate --json
run_bhr "$TMP_DIR/bhr-match-node.json"   "$TMP_DIR/bhr-match"   feat/minha-feature       node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_bhr "$TMP_DIR/bhr-match-py.json"     "$TMP_DIR/bhr-match"   feat/minha-feature       env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

run_bhr "$TMP_DIR/bhr-nomatch-go.json"   "$TMP_DIR/bhr-nomatch" feat/sem-roadmap-nenhum  "$GO_BIN" validate --json
run_bhr "$TMP_DIR/bhr-nomatch-node.json" "$TMP_DIR/bhr-nomatch" feat/sem-roadmap-nenhum  node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_bhr "$TMP_DIR/bhr-nomatch-py.json"   "$TMP_DIR/bhr-nomatch" feat/sem-roadmap-nenhum  env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

run_bhr "$TMP_DIR/bhr-diff-go.json"      "$TMP_DIR/bhr-diff"    feat/minha-feature       "$GO_BIN" validate --json
run_bhr "$TMP_DIR/bhr-diff-node.json"    "$TMP_DIR/bhr-diff"    feat/minha-feature       node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_bhr "$TMP_DIR/bhr-diff-py.json"      "$TMP_DIR/bhr-diff"    feat/minha-feature       env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

python3 - "$TMP_DIR" <<'PY'
import json
import os
import sys

tmp = sys.argv[1]

# ACHADO (não corrigido neste ML — registrado no roadmap): pypi/trackfw/validator.py's
# validate_branch_has_wip_roadmap() returns plain strings, not the {"type": "violation",
# "message": ...} dict shape every other Python rule uses. _enrich_items() (validator.py)
# only tags "rule"/"file" onto dict items — a bare string passes through untouched — so
# Python's `validate --json` reports this ONE rule with "rule": null, "file": null while
# Go and Node.js both tag "rule": "branch_has_wip_roadmap" correctly. Confirmed by direct
# invocation of the 3 real binaries against an identical fixture before writing this gate.
# Pre-existing, unrelated to done/ acceptance (reproduces on the plain no-roadmap case
# too) — filtering by "rule" here would make every non-match Python case in this block
# vacuously fail, so filtering below uses the message text (byte-identical across the 3
# CLIs) instead, and the rule/file divergence is pinned explicitly so it cannot silently
# regress further (e.g. Go/Node losing the tag too) without this gate noticing.
BHR_MESSAGE_MARKER = "wip/ nor done/"


def load(name):
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as fh:
        payload = json.load(fh)
    with open(path + ".exit", encoding="utf-8") as fh:
        exit_code = int(fh.read().strip())
    matching = [
        item for item in payload["violations"]
        if BHR_MESSAGE_MARKER in item.get("message", "")
    ]
    msgs = sorted(item["message"] for item in matching)
    rules = sorted({item.get("rule") for item in matching})
    return exit_code, msgs, rules


# label -> (filename pattern, expect violation?, substring expected in every message)
cases = {
    "match": ("bhr-match-{}.json", False, None),
    "nomatch": ("bhr-nomatch-{}.json", True, "no roadmap is in wip/ nor done/"),
    "diff": ("bhr-diff-{}.json", True, "no matching roadmap in wip/ nor done/"),
}

for label, (pattern, expect_violation, expectation) in cases.items():
    results = {rt: load(pattern.format(rt)) for rt in ("go", "node", "py")}

    for rt, (exit_code, msgs, rules) in results.items():
        if expect_violation:
            # P2 vacuity guard: a regra REALMENTE precisa disparar — um exit
            # não-zero por conta de outra regra qualquer nesta fixture mínima
            # não prova nada sobre branch_has_wip_roadmap.
            if not msgs:
                raise SystemExit(
                    f"branch_has_wip_roadmap done/ parity ({label}/{rt}): esperava "
                    "violação da regra branch_has_wip_roadmap, nenhuma foi "
                    f"reportada (violations completo: exit={exit_code}) — fixture "
                    "vacua ou regressão"
                )
            if exit_code == 0:
                raise SystemExit(
                    f"branch_has_wip_roadmap done/ parity ({label}/{rt}): violação "
                    "reportada mas exit code é 0 — severidade default da regra é "
                    "'error', deveria reprovar"
                )
            if not all(expectation in m for m in msgs):
                raise SystemExit(
                    f"branch_has_wip_roadmap done/ parity ({label}/{rt}): mensagem "
                    f"não contém {expectation!r}: {msgs!r}"
                )
            # Achado pinado (ver comentário BHR_MESSAGE_MARKER acima): Go/Node
            # tagueiam "rule": "branch_has_wip_roadmap"; Python tagueia "rule": null
            # para esta regra especificamente. Qualquer mudança nesse conjunto
            # (nos dois sentidos) é uma regressão real e deve reprovar aqui.
            expected_rules = (
                [None] if rt == "py" else ["branch_has_wip_roadmap"]
            )
            if rules != expected_rules:
                raise SystemExit(
                    f"branch_has_wip_roadmap done/ parity ({label}/{rt}): tag "
                    f"'rule' mudou de {expected_rules!r} para {rules!r} — se for "
                    "Python passando a taguear corretamente, atualizar este "
                    "pin e o achado documentado (validate_branch_has_wip_roadmap "
                    "retorna strings, não dicts); se for Go/Node perdendo a tag, "
                    "é regressão real"
                )
        else:
            # Caso central do ML-2A: roadmap correspondente em done/ deve ser
            # ACEITO — nenhuma violação da regra, seja qual for o exit code
            # geral (outra regra pode disparar nesta fixture mínima sem
            # relação com o que está sendo provado aqui).
            if msgs:
                raise SystemExit(
                    f"branch_has_wip_roadmap done/ parity ({label}/{rt}): roadmap "
                    "correspondente em done/ deveria ser aceito (zero violações "
                    f"da regra), mas {rt} reportou {msgs!r}"
                )

    go_msgs, node_msgs, py_msgs = (results[rt][1] for rt in ("go", "node", "py"))
    if not (go_msgs == node_msgs == py_msgs):
        raise SystemExit(
            f"branch_has_wip_roadmap done/ parity ({label}): mensagens divergem "
            f"entre runtimes -- go={go_msgs!r} node={node_msgs!r} py={py_msgs!r}"
        )

print(
    "branch_has_wip_roadmap done/ acceptance parity checks passed "
    "(match/no-roadmap/diff-slug discriminant, byte-identical across 3 CLIs)"
)
PY

echo "Validate JSON parity checks passed (branch_has_wip_roadmap accepting done/, exercised cross-CLI: match / no-roadmap / diff-slug discriminant)"
