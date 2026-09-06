#!/usr/bin/env bash
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
# ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1C:
# the *_script_integrity family had the SAME class of defect the guard config reads (above) had
# before ML-1A/ML-1B, and hades-tf's ML-1B barrier found it was WORSE here, not milder: Go
# project-scope ABORTED the entire `trackfw validate` (fmt.Errorf propagated raw, non-JSON
# stdout) on the first unreadable script — the exact contract break this whole REQ exists to
# close, since a CI consumer of `--json` loses visibility of every other rule, not just this one.
# Go/Node global-scope and Python (both scopes) silenced instead (fail-open). Only Node
# project-scope was already correct. A directory in place of the script (not chmod 000) is this
# block's fixture — deterministic on every platform and uid, same choice as cg-claude-unreadable
# above.
#
# Isolated project + $HOME for this block (never $TMP_DIR/project's shared fixture, which
# run_validator's exit-code==1 assertion elsewhere already owns) — same "own dedicated $HOME"
# precedent as GVP_HOME above.
SIU_TMP_CLEAN=$(printf '%s' "$TMP_DIR" | sed 's#//*#/#g')
SIU_PROJECT="$SIU_TMP_CLEAN/script-integrity-unreadable-project"
mkdir -p \
  "$SIU_PROJECT/docs/adr" \
  "$SIU_PROJECT/docs/req" \
  "$SIU_PROJECT/docs/roadmaps"/{backlog,wip,blocked,done,abandoned} \
  "$SIU_PROJECT/scripts/trackfw-credential-guard.sh" \
  "$SIU_PROJECT/scripts/trackfw-git-branch-guard.sh"
cat >"$SIU_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

run_siu() {
  local output=$1
  shift
  set +e
  (cd "$SIU_PROJECT" && "$@") >"$output" 2>"$output.stderr"
  local status=$?
  set -e
  # Prova que `--json` sobrevive: mesmo com exit != 0 (não esperamos nenhum aqui, severidade
  # default é "warning"), stdout tem que ser JSON válido — o abort antigo do Go produzia stdout
  # VAZIO, o que falha este parse imediatamente e reprova o gate com uma mensagem clara.
  python3 -c "import json,sys; json.load(open(sys.argv[1], encoding='utf-8'))" "$output" || {
    echo "validate parity (script_integrity unreadable, project): $output is not valid JSON (status=$status) — Go-style abort regressed?" >&2
    return 1
  }
}

run_siu "$TMP_DIR/siu-go.json" "$GO_BIN" validate --json
run_siu "$TMP_DIR/siu-node.json" node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_siu "$TMP_DIR/siu-python.json" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

python3 - "$TMP_DIR/siu-go.json" "$TMP_DIR/siu-node.json" "$TMP_DIR/siu-python.json" <<'PY'
import json
import sys

def warning_messages(path, rule):
    with open(path, encoding="utf-8") as stream:
        payload = json.load(stream)
    return sorted(
        item["message"] for item in payload["warnings"] if item.get("rule") == rule
    )

for rule in ("credential_guard_script_integrity", "git_branch_guard_script_integrity"):
    msgs = [warning_messages(path, rule) for path in sys.argv[1:]]
    for path, value in zip(sys.argv[1:], msgs):
        if not value:
            raise SystemExit(
                f"validate parity (script_integrity unreadable, project): {path} produced ZERO "
                f"{rule} warnings — fixture is vacuous, or this CLI regressed to silencing/"
                "aborting on an unreadable script (ROADMAP-2026-09-06 ML-1C)"
            )
        for m in value:
            if "could not be read" not in m:
                raise SystemExit(
                    f"validate parity (script_integrity unreadable, project): {path} {rule} "
                    f"message does not say 'could not be read': {m!r}"
                )
    if msgs[1:] != msgs[:-1]:
        for path, value in zip(sys.argv[1:], msgs):
            print(path, rule, json.dumps(value, indent=2), file=sys.stderr)
        raise SystemExit(
            f"validate parity: {rule} unreadable-script warning message text differs between "
            "runtimes (byte-for-byte comparison, not just rule+file)"
        )
PY

echo "Validate JSON parity checks passed (*_script_integrity unreadable script, project scope, byte-identical across 3 CLIs, --json stays valid — Go no longer aborts)"

# Contraparte de escopo GLOBAL — mesmo achado da barreira: Go silenciava (return nil, nil
# incondicional em qualquer erro de leitura), Node tinha `catch (_) { return [] }` idêntico,
# Python tinha `except OSError: return []` idêntico. $HOME isolado, dedicado a este bloco — e um
# PROJETO próprio, limpo (scripts/*.sh ausentes, não "diretório no lugar do arquivo" como
# SIU_PROJECT acima) para que a violação de escopo de PROJETO não contamine esta contagem: sem
# isso, os dois escopos disparariam juntos e "global scope" deixaria de estar presente em toda
# mensagem coletada.
SIU_HOME="$SIU_TMP_CLEAN/script-integrity-unreadable-home"
mkdir -p \
  "$SIU_HOME/.trackfw/scripts/trackfw-credential-guard.sh" \
  "$SIU_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"

SIU_GLOBAL_PROJECT="$SIU_TMP_CLEAN/script-integrity-unreadable-global-project"
mkdir -p \
  "$SIU_GLOBAL_PROJECT/docs/adr" \
  "$SIU_GLOBAL_PROJECT/docs/req" \
  "$SIU_GLOBAL_PROJECT/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}
cat >"$SIU_GLOBAL_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

run_siu_global() {
  local output=$1
  shift
  (cd "$SIU_GLOBAL_PROJECT" && HOME="$SIU_HOME" "$@") >"$output" 2>"$output.stderr"
}

run_siu_global "$TMP_DIR/siu-global-go.json" "$GO_BIN" validate --json
run_siu_global "$TMP_DIR/siu-global-node.json" node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_siu_global "$TMP_DIR/siu-global-python.json" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

python3 - "$TMP_DIR/siu-global-go.json" "$TMP_DIR/siu-global-node.json" "$TMP_DIR/siu-global-python.json" <<'PY'
import json
import sys

def warning_messages(path, rule):
    with open(path, encoding="utf-8") as stream:
        payload = json.load(stream)
    return sorted(
        item["message"] for item in payload["warnings"] if item.get("rule") == rule
    )

for rule in ("credential_guard_script_integrity", "git_branch_guard_script_integrity"):
    msgs = [warning_messages(path, rule) for path in sys.argv[1:]]
    for path, value in zip(sys.argv[1:], msgs):
        if not value:
            raise SystemExit(
                f"validate parity (script_integrity unreadable, GLOBAL): {path} produced ZERO "
                f"{rule} warnings — fixture is vacuous, or this CLI regressed to silencing on an "
                "unreadable GLOBAL script (ROADMAP-2026-09-06 ML-1C)"
            )
        for m in value:
            if "could not be read" not in m or "global scope" not in m:
                raise SystemExit(
                    f"validate parity (script_integrity unreadable, GLOBAL): {path} {rule} "
                    f"message missing 'could not be read'/'global scope': {m!r}"
                )
    if msgs[1:] != msgs[:-1]:
        for path, value in zip(sys.argv[1:], msgs):
            print(path, rule, json.dumps(value, indent=2), file=sys.stderr)
        raise SystemExit(
            f"validate parity: {rule} GLOBAL-scope unreadable-script warning message text "
            "differs between runtimes"
        )
PY

echo "Validate JSON parity checks passed (*_script_integrity unreadable script, GLOBAL scope, byte-identical across 3 CLIs — no longer silenced)"

# ---------------------------------------------------------------------------
# FIFO in place of the guard config/script: hades-tf's ML-1B barrier found this made
# os.ReadFile/fs.readFileSync/open().read() block INDEFINITELY in all 3 runtimes (mkfifo
# .claude/settings.json — trackfw validate --json never returns). Wrapped in a hard timeout via
# background process + sleep + kill (this repo has no portable `timeout` binary — absent by
# default on macOS) so a REGRESSED fix hangs this ONE check for a bounded few seconds and fails
# loudly, instead of hanging `make quality` (and CI) forever.
# ---------------------------------------------------------------------------
SIU_FIFO_PROJECT="$SIU_TMP_CLEAN/script-integrity-fifo-project"
mkdir -p \
  "$SIU_FIFO_PROJECT/docs/adr" \
  "$SIU_FIFO_PROJECT/docs/req" \
  "$SIU_FIFO_PROJECT/docs/roadmaps"/{backlog,wip,blocked,done,abandoned} \
  "$SIU_FIFO_PROJECT/.claude" \
  "$SIU_FIFO_PROJECT/scripts"
cat >"$SIU_FIFO_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF
mkfifo "$SIU_FIFO_PROJECT/scripts/trackfw-credential-guard.sh"
mkfifo "$SIU_FIFO_PROJECT/.claude/settings.json"

run_with_hard_timeout() {
  # $1=seconds, $2=output file, rest=command. Returns 124 on timeout (mirrors coreutils `timeout`'s
  # exit code convention), the command's own exit code otherwise.
  local secs=$1 output=$2
  shift 2
  ( "$@" >"$output" 2>"$output.stderr" ) &
  local pid=$!
  ( sleep "$secs" && kill -9 "$pid" 2>/dev/null ) &
  local watchdog=$!
  local status=0
  wait "$pid" 2>/dev/null || status=$?
  kill "$watchdog" 2>/dev/null || true
  wait "$watchdog" 2>/dev/null || true
  return "$status"
}

for label_bin in "go:$GO_BIN" "node:node $ROOT_DIR/npm/bin/trackfw" "python:env PYTHONPATH=$ROOT_DIR/pypi python3 -m trackfw"; do
  label="${label_bin%%:*}"
  cmd="${label_bin#*:}"
  out="$TMP_DIR/siu-fifo-$label.json"
  set +e
  ( cd "$SIU_FIFO_PROJECT" && run_with_hard_timeout 8 "$out" $cmd validate --json )
  status=$?
  set -e
  if [[ $status -eq 124 || $status -eq 137 ]]; then
    echo "validate parity (script_integrity FIFO): $label runtime HUNG on a FIFO in place of the guard config/script — the exact hang hades-tf's ML-1B barrier reported; the fix regressed" >&2
    exit 1
  fi
  python3 -c "import json,sys; json.load(open(sys.argv[1], encoding='utf-8'))" "$out" || {
    echo "validate parity (script_integrity FIFO): $label output is not valid JSON (status=$status)" >&2
    exit 1
  }
done

python3 - "$TMP_DIR/siu-fifo-go.json" "$TMP_DIR/siu-fifo-node.json" "$TMP_DIR/siu-fifo-python.json" <<'PY'
import json
import sys

def messages(path):
    with open(path, encoding="utf-8") as stream:
        payload = json.load(stream)
    out = []
    for item in payload["violations"] + payload["warnings"]:
        if item.get("rule") in ("credential_guard_hook_resolvable", "credential_guard_script_integrity"):
            out.append(item["message"])
    return sorted(out)

msgs = [messages(path) for path in sys.argv[1:]]
for path, value in zip(sys.argv[1:], msgs):
    if not any("could not be read" in m for m in value):
        raise SystemExit(
            f"validate parity (script_integrity FIFO): {path} did not accuse 'could not be "
            f"read' for the FIFO fixture — obtained: {value}"
        )
PY

echo "Validate JSON parity checks passed (FIFO in place of guard config AND script, all 3 runtimes return within the hard timeout, accuse 'could not be read', --json stays valid)"

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

# ---------------------------------------------------------------------------
# ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco, ML-3A: a regra
# `credential_guard_hook_resolvable` exercitada cross-CLI (os 3 CLIs concordam,
# não só Go do Cenário 47) — 4 casos orientados por fixture de projeto:
#
#   1. claude-absent:  .claude/settings.json com $CLAUDE_PROJECT_DIR/…, script
#      AUSENTE → violação nos 3 CLIs (braço de detecção)
#   2. claude-present: mesmo hook, script PRESENTE e executável → silêncio nos 3
#      (não-regressão/baseline)
#   3. cursor-absent:  .cursor/hooks.json com caminho relativo puro, script
#      AUSENTE → violação nos 3 (prova que o braço relativo é alcançável —
#      discriminante: sem este caso, uma implementação que retornasse ok=false para
#      todo caminho relativo passaria os casos 1/2/4 por omissão)
#   4. cursor-present: mesmo hook Cursor, script PRESENTE → silêncio nos 3
#      (guarda de falso-positivo: caminho relativo legítimo não acusado)
#
# Cada braço filtra as violações/warnings pelo campo "rule" (que os 3 CLIs emitem
# corretamente para esta regra — Python usa _enrich_items(msgs, rule_name) e msgs
# é lista de dicts, não strings, então o tag não se perde como em
# validate_branch_has_wip_roadmap). Nenhum TRACKFW_BRANCH necessário: a regra só
# lê arquivos do projeto, sem chamada git.
#
# $TMP_DIR pode embutir "//" (macOS $TMPDIR com barra final) — normalizado em
# CG_TMP_CLEAN antes de criar os fixtures, pelo mesmo motivo documentado no
# bloco GVP acima (Go colapsa via filepath.Join; Python não — preservaria "//"
# embutido na mensagem quebrando a comparação byte-a-byte por artefato de fixture).
# ---------------------------------------------------------------------------
CG_TMP_CLEAN=$(printf '%s' "$TMP_DIR" | sed 's#//*#/#g')

for cg_fixture in cg-claude-absent cg-claude-present cg-cursor-absent cg-cursor-present cg-claude-noexec cg-claude-notype cg-claude-relativo cg-copilot-relativo-present cg-claude-pwd cg-claude-pwd-quoted cg-claude-absoluto cg-claude-outra-var cg-claude-git-toplevel cg-cursor-pwd cg-claude-invalid-json cg-claude-unreadable cg-claude-utf16; do
  mkdir -p "$CG_TMP_CLEAN/$cg_fixture/docs/roadmaps"/{wip,done}
  cat >"$CG_TMP_CLEAN/$cg_fixture/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
done

# Claude settings.json — formato com "type":"command" (requiresCommandType=true).
# Mesma estrutura que s47_write_claude_guard_hook no Cenário 47.
# cg-claude-noexec compartilha este formato (script presente mas não-executável).
for cg_fixture in cg-claude-absent cg-claude-present cg-claude-noexec; do
  mkdir -p "$CG_TMP_CLEAN/$cg_fixture/.claude"
  cat >"$CG_TMP_CLEAN/$cg_fixture/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"}]}]}}
EOF
done

# cg-claude-notype: sem "type":"command" — o hook é sintaticamente válido mas o
# Claude Code nunca o executa (requiresCommandType=true para Claude). O validador
# deve emitir violation "missing type:command" ANTES de verificar existência do
# script (ML-4B, ROADMAP-2026-08-17 ML-4B). Script ausente — não relevante.
mkdir -p "$CG_TMP_CLEAN/cg-claude-notype/.claude"
cat >"$CG_TMP_CLEAN/cg-claude-notype/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"}]}]}}
EOF

# Cursor hooks.json — formato sem "type" (requiresCommandType=false), caminho relativo puro.
for cg_fixture in cg-cursor-absent cg-cursor-present; do
  mkdir -p "$CG_TMP_CLEAN/$cg_fixture/.cursor"
  cat >"$CG_TMP_CLEAN/$cg_fixture/.cursor/hooks.json" <<'EOF'
{"version":1,"hooks":{"beforeShellExecution":[{"command":"scripts/trackfw-credential-guard.sh"}]}}
EOF
done

# Script presente e executável apenas nos fixtures "present".
for cg_fixture in cg-claude-present cg-cursor-present; do
  mkdir -p "$CG_TMP_CLEAN/$cg_fixture/scripts"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$CG_TMP_CLEAN/$cg_fixture/scripts/trackfw-credential-guard.sh"
  chmod +x "$CG_TMP_CLEAN/$cg_fixture/scripts/trackfw-credential-guard.sh"
done

# cg-claude-noexec: script presente mas não-executável (chmod 644).
# O chmod 644 é explícito e não depende de umask — a intenção deve ser legível no diff.
mkdir -p "$CG_TMP_CLEAN/cg-claude-noexec/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$CG_TMP_CLEAN/cg-claude-noexec/scripts/trackfw-credential-guard.sh"
chmod 644 "$CG_TMP_CLEAN/cg-claude-noexec/scripts/trackfw-credential-guard.sh"
# cg-claude-notype: sem script — o validador deve disparar "missing type:command"
# antes de checar existência/executabilidade do script.

# cg-claude-relativo: .claude/settings.json com caminho relativo puro (forma
# antiga/errada para Claude). "type":"command" PRESENTE — a acusação é pela
# FORMA, não pela ausência de tipo ou do script. Script PRESENTE e executável.
# Isso prova que a regra detecta a forma pelo CLI (requiresVarOrShellPrefix=true
# para Claude), não pela ausência física do script (ROADMAP-2026-08-21 ML-2A).
mkdir -p "$CG_TMP_CLEAN/cg-claude-relativo/.claude"
cat >"$CG_TMP_CLEAN/cg-claude-relativo/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"scripts/trackfw-credential-guard.sh"}]}]}}
EOF
mkdir -p "$CG_TMP_CLEAN/cg-claude-relativo/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$CG_TMP_CLEAN/cg-claude-relativo/scripts/trackfw-credential-guard.sh"
chmod +x "$CG_TMP_CLEAN/cg-claude-relativo/scripts/trackfw-credential-guard.sh"

# cg-copilot-relativo-present: .github/hooks/trackfw-attention.json com caminho
# relativo puro. Copilot tem requiresVarOrShellPrefix=false — relativo é a forma
# CORRETA para ele (campo "bash", "type":"command" como irmão obrigatório).
# Script PRESENTE. Espera-se SILÊNCIO nos 3 CLIs — é o discriminante de
# falso-positivo e o alvo do Cenário P4 direção-B (Cenário 160): flipping
# requiresVarOrShellPrefix para Copilot deve fazer o gate reprovar aqui.
mkdir -p "$CG_TMP_CLEAN/cg-copilot-relativo-present/.github/hooks"
cat >"$CG_TMP_CLEAN/cg-copilot-relativo-present/.github/hooks/trackfw-attention.json" <<'EOF'
{"type":"command","bash":"scripts/trackfw-credential-guard.sh"}
EOF
mkdir -p "$CG_TMP_CLEAN/cg-copilot-relativo-present/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$CG_TMP_CLEAN/cg-copilot-relativo-present/scripts/trackfw-credential-guard.sh"
chmod +x "$CG_TMP_CLEAN/cg-copilot-relativo-present/scripts/trackfw-credential-guard.sh"

# cg-claude-invalid-json: .claude/settings.json com JSON sintaticamente inválido (vírgula sobrando
# fecha o objeto antes da hora) — ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-
# deixa-de-ser-silencio, ML-1A. Nenhum script criado: a regra deve acusar "is not valid JSON" ANTES
# de tentar extrair/resolver qualquer comando — antes desta ML, as 3 implementações silenciavam
# (`continue` mudo) e este arquivo corrompido nunca aparecia em lugar nenhum da saída de
# `validate`. Este MESMO arquivo corrompido é lido por AMBAS as regras (credential_guard_hook_
# resolvable e git_branch_guard_hook_resolvable — ver o bloco gbg_cases mais abaixo, que reusa a
# saída desta mesma fixture em vez de rodar `validate` de novo), provando que a corrupção cega os
# dois controles ao mesmo tempo, não só um.
mkdir -p "$CG_TMP_CLEAN/cg-claude-invalid-json/.claude"
printf '{"hooks": {,}}' > "$CG_TMP_CLEAN/cg-claude-invalid-json/.claude/settings.json"

# cg-claude-unreadable: .claude/settings.json é um DIRETÓRIO, não um arquivo — hades-tf's ML-1A
# barrier reproduced this live (chmod 000, and a directory in place of the file) and found the
# READ failure (distinct from the JSON-parse failure above) taking a different path in each
# runtime: Go aborted the ENTIRE `trackfw validate` (exit 1, empty stdout, no other rule
# reported), while Node/Python silently reported success — ROADMAP-2026-09-06-...-config-
# ilegivel-deixa-de-ser-silencio, ML-1B. A directory (not chmod 000) is the gate's fixture:
# deterministic on every platform and uid — chmod 000 is a no-op when the gate runs as root.
mkdir -p "$CG_TMP_CLEAN/cg-claude-unreadable/.claude/settings.json"

# cg-claude-utf16: .claude/settings.json salvo inteiro como UTF-16 (o erro clássico de "Salvar
# como Unicode" do Notepad no Windows) — hades-tf's ML-1A barrier reproduced this live and found
# Python's strict `encoding="utf-8"` read crashing with an unstructured traceback (exit 1, no
# JSON output), while Go/Node stayed silent-safe (json.Unmarshal on raw bytes / lossy decode).
# Distinct message from invalid-json: the real cause is the file's ENCODING, not JSON syntax
# (D4 of ADR-2026-09-04 — classify right, explain right, or the reader investigates the wrong
# thing) — ROADMAP-2026-09-06-...-config-ilegivel-deixa-de-ser-silencio, ML-1B.
mkdir -p "$CG_TMP_CLEAN/cg-claude-utf16/.claude"
python3 -c '
import sys
with open(sys.argv[1], "wb") as f:
    f.write("{\"hooks\":{}}".encode("utf-16"))
' "$CG_TMP_CLEAN/cg-claude-utf16/.claude/settings.json"

# ---------------------------------------------------------------------------
# ROADMAP-2026-08-22 ML-2A: fixtures para os novos casos de classe de ancoragem.
# Referência ao ADR-2026-08-22-postura-do-validate-diante-de-formas-de-hook-
# nao-reconhecidas-classificar-por-ancoragem-nao-por-casamento-com-o-gerado.md
#
# Classe 2 (dependente do cwd) → acusar (requiresVarOrShellPrefix=true para Claude):
#   cg-claude-pwd        — $PWD/scripts/… (cláusula de prefixo literal)
#   cg-claude-pwd-quoted — "$PWD/scripts/…" entre aspas (achado D.3: aspas
#                          removidas por stripOuterQuotesForClassify antes de classificar)
#
# Classe 1 (ancorado) → silencioso:
#   cg-claude-absoluto      — /opt/trackfw/scripts/… (filepath.IsAbs=true → classe 1)
#   cg-claude-git-toplevel  — "$(git rev-parse --show-toplevel)/…" (forma Codex/ML-1A)
#
# Classe 3 (indecidível) → silencioso, residual declarado:
#   cg-claude-outra-var — $OUTRA_VAR/scripts/… ($ mas não $PWD/ nem reconhecido)
#
# Não-regressão (Cursor com $PWD → silêncio via requiresVarOrShellPrefix=false):
#   cg-cursor-pwd — .cursor/hooks.json com $PWD/scripts/… → silêncio
#
# Nota sobre vacuidade em casos silenciosos:
#   cg-claude-absoluto e cg-claude-git-toplevel: resolveCredentialGuardHookPath retorna
#   ok=false para estas formas (default branch), portanto o validador não verifica
#   existência do script → nenhum "but the script does not exist" é emitido.
#   A ausência de violation é real, não por script presente. Residual declarado:
#   "a ausência de erro em classe 1 e 3 é verificável apenas pela sabotagem de sentido
#   oposto (Cenário 165), não por um irmão -absent, pois resolve retorna ok=false antes
#   de inspecionar o filesystem" (ADR-2026-08-22 §3).
#
# cg-claude-outra-var e cg-cursor-pwd: mesma nota — ok=false antes do filesystem.
# ---------------------------------------------------------------------------

# Pre-flight: validar JSON dos novos fixtures com python3 antes de confiar neles.
# O handoff avisou que JSON com aspas embutidas mal escapadas dá falso negativo
# indistinguível de regra que não detecta (barreira de 2026-08-21, ML-1B).
_validate_fixture_json() {
  local path=$1 expected_command=$2
  local got
  got=$(python3 -c "
import json, sys
with open('$path') as f:
  d = json.load(f)
hooks = d['hooks']['PreToolUse'][0]['hooks'][0]
sys.stdout.write(hooks['command'])
" 2>&1) || {
    echo "FAIL [cg-fixture/json-invalid]: $path nao parseia como JSON valido: $got" >&2
    exit 1
  }
  if [ "$got" != "$expected_command" ]; then
    echo "FAIL [cg-fixture/json-command-mismatch]: $path — esperava command=$expected_command, obteve=$got" >&2
    exit 1
  fi
}

# cg-claude-pwd: $PWD/scripts/… → classe 2, acusar (requiresVarOrShellPrefix=true)
mkdir -p "$CG_TMP_CLEAN/cg-claude-pwd/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '\$PWD/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-pwd/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-pwd/.claude/settings.json" \
  '$PWD/scripts/trackfw-credential-guard.sh'

# cg-claude-pwd-quoted: "$PWD/scripts/…" com aspas externas → achado D.3:
# stripOuterQuotesForClassify remove as aspas e classifica como $PWD/ → classe 2 → acusar.
mkdir -p "$CG_TMP_CLEAN/cg-claude-pwd-quoted/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '\"' + r'\$PWD/scripts/trackfw-credential-guard.sh' + '\"'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-pwd-quoted/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-pwd-quoted/.claude/settings.json" \
  '"$PWD/scripts/trackfw-credential-guard.sh"'

# cg-claude-absoluto: /opt/trackfw/scripts/… → classe 1 (filepath.IsAbs=true) → silencioso.
# Script não criado: resolveCredentialGuardHookPath retorna ok=false para caminhos
# absolutos (default branch), não inspeciona o filesystem.
mkdir -p "$CG_TMP_CLEAN/cg-claude-absoluto/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '/opt/trackfw/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-absoluto/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
# Validar cg-claude-absoluto inline (estrutura idêntica à função de pré-voo)
python3 -c "
import json, sys
with open('$CG_TMP_CLEAN/cg-claude-absoluto/.claude/settings.json') as f:
  d = json.load(f)
cmd = d['hooks']['PreToolUse'][0]['hooks'][0]['command']
assert cmd == '/opt/trackfw/scripts/trackfw-credential-guard.sh', 'expected absolute path, got ' + repr(cmd)
sys.stdout.write('OK cg-claude-absoluto JSON\n')
"

# cg-claude-outra-var: \$OUTRA_VAR/scripts/… → classe 3 (indecidível) → silencioso.
mkdir -p "$CG_TMP_CLEAN/cg-claude-outra-var/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '\$OUTRA_VAR/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-outra-var/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"

# cg-claude-git-toplevel: "$(git rev-parse --show-toplevel)/…" → classe 1 → silencioso.
# Esta é a forma Codex/ML-1A com aspas externas — stripOuterQuotesForClassify remove as
# aspas, e a forma resultante começa com $( → reconhecida como git rev-parse → classe 1.
# NOTA: resolveCredentialGuardHookPath RECONHECE esta forma (codexPrefix case) e resolve
# scripts/trackfw-credential-guard.sh contra a raiz do fixture — o script PRECISA existir
# para não disparar "script does not exist" que mascararia o silêncio de classificação.
mkdir -p "$CG_TMP_CLEAN/cg-claude-git-toplevel/.claude"
python3 -c "
import json
cmd = '\"' + r'\$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh' + '\"'
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': cmd}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-git-toplevel/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
mkdir -p "$CG_TMP_CLEAN/cg-claude-git-toplevel/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$CG_TMP_CLEAN/cg-claude-git-toplevel/scripts/trackfw-credential-guard.sh"
chmod +x "$CG_TMP_CLEAN/cg-claude-git-toplevel/scripts/trackfw-credential-guard.sh"

# cg-cursor-pwd: .cursor/hooks.json com $PWD/… → silencioso porque Cursor tem
# requiresVarOrShellPrefix=false → a guarda de CLI curto-circuita antes de
# consultar o classificador. Prova não-regressão do AC3 da REQ: a regra não
# acusa Cursor mesmo quando a forma seria classe 2 se fosse Claude.
# Residual declarado: ok=false antes do filesystem; ausência de violation é real.
mkdir -p "$CG_TMP_CLEAN/cg-cursor-pwd/.cursor"
python3 -c "
import json
d = {'version': 1, 'hooks': {'beforeShellExecution': [{'command': '\$PWD/scripts/trackfw-credential-guard.sh'}]}}
with open('$CG_TMP_CLEAN/cg-cursor-pwd/.cursor/hooks.json', 'w') as f:
    json.dump(d, f)
"

# ---------------------------------------------------------------------------
# ROADMAP-2026-08-22 ML-4A: fixtures para os achados da barreira ML-3A (Hades).
#
# cg-claude-tilde       — ~/scripts/… sem aspas → classe 1 (tilde expande para
#                         $HOME em shell POSIX — ancorado) → silencioso.
#                         Residual declarado: resolveCredentialGuardHookPath retorna
#                         ok=false para ~/… (excluído do case de relativo puro),
#                         portanto não inspeciona o filesystem; silêncio é real.
#
# cg-claude-tilde-quoted — "~/scripts/…" com aspas → classe 2 (tilde NÃO expande
#                           dentro de aspas duplas) → acusar com "quoted tilde path"
#                           (D4 da ADR-2026-09-04, achado do ML-2B).
#
# cg-claude-tilde-user   — ~alice/scripts/… (nomeado) → classe 2 (expande em POSIX
#                           mas para outro usuário, não o $HOME do agente — indecidível
#                           sem executar shell) → acusar com "named-user tilde path"
#                           (D4 da ADR-2026-09-04, achado do ML-2B).
#
# cg-claude-pwd-braced   — ${PWD}/scripts/… → classe 2 (mesma semântica de $PWD/…;
#                           PWD é mandado pelo POSIX, sempre o cwd) → acusar com
#                           mensagem do $PWD.
#
# cg-claude-sh-c-pwd     — sh -c "$PWD/scripts/…" → classe 2 (cwd-dependent) e
#                           mensagem do $PWD (Contains, não HasPrefix) → acusar.
#
# cg-claude-windows-drive — C:\Users\kg\scripts\… → classe 1 (letra de unidade é
#                           ANCORADA por união, independente do GOOS de quem roda
#                           o validator — ADR-2026-09-04, D1) → silencioso.
# ---------------------------------------------------------------------------

# cg-claude-tilde: ~/scripts/… (sem aspas) → classe 1 → silencioso.
# Residual: resolveCredentialGuardHookPath exclui ~/… do case de relativo puro;
# ok=false silencia sem inspecionar o filesystem.
mkdir -p "$CG_TMP_CLEAN/cg-claude-tilde/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '~/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-tilde/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-tilde/.claude/settings.json" \
  '~/scripts/trackfw-credential-guard.sh'

# cg-claude-tilde-quoted: "~/scripts/…" (com aspas externas no valor JSON) → classe 2
# (tilde não expande dentro de aspas duplas) → acusar com "quoted tilde path"
# (D4 da ADR-2026-09-04, achado do ML-2B).
mkdir -p "$CG_TMP_CLEAN/cg-claude-tilde-quoted/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '\"' + r'~/scripts/trackfw-credential-guard.sh' + '\"'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-tilde-quoted/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-tilde-quoted/.claude/settings.json" \
  '"~/scripts/trackfw-credential-guard.sh"'

# cg-claude-tilde-user: ~alice/scripts/… (nomeado, sem aspas) → classe 2 (~user/ expande em POSIX
# mas para OUTRO usuário, não $HOME do agente — indecidível sem executar shell, não relativo) →
# acusar com "named-user tilde path" (D4 da ADR-2026-09-04, achado do ML-2B).
mkdir -p "$CG_TMP_CLEAN/cg-claude-tilde-user/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '~alice/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-tilde-user/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-tilde-user/.claude/settings.json" \
  '~alice/scripts/trackfw-credential-guard.sh'

# cg-claude-pwd-braced: \${PWD}/scripts/… → classe 2 → acusar com mensagem do $PWD.
mkdir -p "$CG_TMP_CLEAN/cg-claude-pwd-braced/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '\${PWD}/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-pwd-braced/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-pwd-braced/.claude/settings.json" \
  '${PWD}/scripts/trackfw-credential-guard.sh'

# cg-claude-sh-c-pwd: sh -c "\$PWD/scripts/…" → classe 2 (cwd-dependent), mensagem do $PWD.
# Contains("\$PWD") em qualquer posição → "with a \$PWD path" (não "bare relative path").
mkdir -p "$CG_TMP_CLEAN/cg-claude-sh-c-pwd/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': 'sh -c \"\$PWD/scripts/trackfw-credential-guard.sh\"'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-sh-c-pwd/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-sh-c-pwd/.claude/settings.json" \
  'sh -c "$PWD/scripts/trackfw-credential-guard.sh"'

# cg-claude-windows-drive: C:\Users\kg\scripts\… → classe 1 (letra de unidade é ANCORADA por
# união, independente do GOOS de quem roda o validator — ADR-2026-09-04-caminho-posix-ancorado-
# num-config-lido-por-cli-de-agente-e-absoluto-independente-do-so-host, D1) → silencioso.
# É o teste de integração que falsifica o defeito do ML-3A: antes desta ADR, filepath.IsAbs /
# path.isAbsolute / os.path.isabs classificavam esta forma como relativa NO WINDOWS, e o
# validator emitia violation de guard onde não deveria — o controle POSIX exige que aqui,
# em macOS/Linux, a forma continue silenciosa (mesmo veredito que teria em POSIX de qualquer jeito).
mkdir -p "$CG_TMP_CLEAN/cg-claude-windows-drive/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': r'C:\Users\kg\scripts\trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP_CLEAN/cg-claude-windows-drive/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"
_validate_fixture_json "$CG_TMP_CLEAN/cg-claude-windows-drive/.claude/settings.json" \
  'C:\Users\kg\scripts\trackfw-credential-guard.sh'

run_cg() {
  local output=$1 dir=$2
  shift 2
  set +e
  ( cd "$dir" && "$@" ) >"$output" 2>"$output.stderr"
  echo "$?" >"$output.exit"
  set -e
}

for cg_fixture in cg-claude-absent cg-claude-present cg-cursor-absent cg-cursor-present cg-claude-noexec cg-claude-notype cg-claude-relativo cg-copilot-relativo-present cg-claude-pwd cg-claude-pwd-quoted cg-claude-absoluto cg-claude-outra-var cg-claude-git-toplevel cg-cursor-pwd cg-claude-tilde cg-claude-tilde-quoted cg-claude-tilde-user cg-claude-pwd-braced cg-claude-sh-c-pwd cg-claude-windows-drive cg-claude-invalid-json cg-claude-unreadable cg-claude-utf16; do
  run_cg "$CG_TMP_CLEAN/$cg_fixture-go.json"   "$CG_TMP_CLEAN/$cg_fixture" "$GO_BIN" validate --json
  run_cg "$CG_TMP_CLEAN/$cg_fixture-node.json" "$CG_TMP_CLEAN/$cg_fixture" node "$ROOT_DIR/npm/bin/trackfw" validate --json
  run_cg "$CG_TMP_CLEAN/$cg_fixture-py.json"   "$CG_TMP_CLEAN/$cg_fixture" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json
done

python3 - "$CG_TMP_CLEAN" <<'PY'
import json
import os
import sys

tmp = sys.argv[1]

# Todas as 3 implementações tagueiam "rule": "credential_guard_hook_resolvable" corretamente:
# Go     → applyRuleTagged / strings emitidas pelo validateGuardHookResolvable
# Node   → enriquecido em validateResult (index.js)
# Python → _enrich_items(msgs, "credential_guard_hook_resolvable"); msgs é lista de dicts
#          (não strings como validate_branch_has_wip_roadmap), portanto "rule" não se perde.
CG_RULE = "credential_guard_hook_resolvable"

# Marcadores por caso — substring esperada em TODAS as mensagens de violação daquele caso.
# None para casos sem violação.
CG_MARKER_ABSENT = "but the script does not exist"
CG_MARKER_NOEXEC = "not executable"
CG_MARKER_NOTYPE = 'missing "type":"command"'
CG_MARKER_BARE   = "with a bare relative path"
CG_MARKER_PWD    = "with a $PWD path"
# D4 da ADR-2026-09-04-caminho-posix-ancorado-...: "~/…" aspeado deixou de dizer
# "bare relative path" (achado do ML-2B: a frase não nomeava a causa real —
# aspas impedindo a expansão do til, não relatividade) e passou a dizer
# "quoted tilde path".
CG_MARKER_TILDE_QUOTED = "with a quoted tilde path"
CG_MARKER_TILDE_USER   = "with a named-user tilde path"
# ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1A: o
# arquivo é sintaticamente inválido, então nenhum comando é extraível — a regra acusa a
# ILEGIBILIDADE do arquivo, não uma forma de comando específica.
CG_MARKER_INVALID_JSON = "is not valid JSON"
# ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1B: uma
# falha de LEITURA (não de parse) é uma causa diferente — a mensagem distingue as duas para que o
# usuário não investigue sintaxe JSON quando o problema é um bit de permissão ou um diretório no
# lugar do arquivo.
CG_MARKER_COULD_NOT_BE_READ = "could not be read"
# Idem para uma falha de DECODIFICAÇÃO (arquivo não é UTF-8 — ex.: salvo como UTF-16) — terceira
# causa distinta de "invalid JSON" e "could not be read" (D4 da ADR-2026-09-04).
CG_MARKER_NOT_UTF8 = "is not valid UTF-8"


def load(name):
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as fh:
        payload = json.load(fh)
    with open(path + ".exit", encoding="utf-8") as fh:
        exit_code = int(fh.read().strip())
    all_items = payload.get("violations", []) + payload.get("warnings", [])
    matching = [item for item in all_items if item.get("rule") == CG_RULE]
    msgs = sorted(item["message"] for item in matching)
    return exit_code, msgs


# label → (filename-pattern, expect_violation, msg_marker_or_None)
# ML-4B (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco, A-1):
# adicionados cg-claude-noexec (script presente mas não-executável) e
# cg-claude-notype (hook sem "type":"command"), estendendo um padrão já coberto
# no escopo de harness pelo bloco gvmt acima — aqui coberto no escopo de projeto.
cases = {
    "claude-absent":          ("cg-claude-absent-{}.json",          True,  CG_MARKER_ABSENT),
    "claude-present":         ("cg-claude-present-{}.json",         False, None),
    "cursor-absent":          ("cg-cursor-absent-{}.json",          True,  CG_MARKER_ABSENT),
    "cursor-present":         ("cg-cursor-present-{}.json",         False, None),
    "claude-noexec":          ("cg-claude-noexec-{}.json",          True,  CG_MARKER_NOEXEC),
    "claude-notype":          ("cg-claude-notype-{}.json",          True,  CG_MARKER_NOTYPE),
    # ROADMAP-2026-08-21 ML-2A: forma relativa antiga acusada; falso-positivo
    # de Copilot silenciado (requiresVarOrShellPrefix=false para Copilot).
    "claude-relativo":          ("cg-claude-relativo-{}.json",          True,  CG_MARKER_BARE),
    "copilot-relativo-present": ("cg-copilot-relativo-present-{}.json", False, None),
    # ROADMAP-2026-08-22 ML-2A: classificação por ancoragem (ADR-2026-08-22).
    # Classe 2 (dependente do cwd) → acusar:
    "claude-pwd":          ("cg-claude-pwd-{}.json",          True,  CG_MARKER_PWD),
    "claude-pwd-quoted":   ("cg-claude-pwd-quoted-{}.json",   True,  CG_MARKER_PWD),
    # Classe 1 (ancorado) → silencioso:
    "claude-absoluto":     ("cg-claude-absoluto-{}.json",     False, None),
    "claude-git-toplevel": ("cg-claude-git-toplevel-{}.json", False, None),
    # Classe 3 (indecidível) → silencioso, residual declarado:
    "claude-outra-var":    ("cg-claude-outra-var-{}.json",    False, None),
    # Não-regressão AC3: Cursor com $PWD → silêncio (requiresVarOrShellPrefix=false):
    "cursor-pwd":          ("cg-cursor-pwd-{}.json",          False, None),
    # ROADMAP-2026-08-22 ML-4A: achados da barreira ML-3A (Hades).
    # ~/… sem aspas → classe 1 (tilde expande para $HOME) → silencioso:
    "claude-tilde":        ("cg-claude-tilde-{}.json",        False, None),
    # "~/…" com aspas → classe 2 (tilde não expande em aspas duplas) → D4 da
    # ADR-2026-09-04: "quoted tilde path" (não mais "bare relative path"):
    "claude-tilde-quoted": ("cg-claude-tilde-quoted-{}.json", True,  CG_MARKER_TILDE_QUOTED),
    "claude-tilde-user":   ("cg-claude-tilde-user-{}.json",   True,  CG_MARKER_TILDE_USER),
    # ${PWD}/… → classe 2 → mensagem do $PWD:
    "claude-pwd-braced":   ("cg-claude-pwd-braced-{}.json",   True,  CG_MARKER_PWD),
    # sh -c "$PWD/…" → classe 2 → mensagem do $PWD (contains, não startsWith):
    "claude-sh-c-pwd":     ("cg-claude-sh-c-pwd-{}.json",     True,  CG_MARKER_PWD),
    # ADR-2026-09-04 (ML-3A): letra de unidade do Windows → classe 1 (ancorado por união,
    # independente do GOOS) → silencioso. Falsifica o defeito do ML-3A nos 3 CLIs.
    "claude-windows-drive": ("cg-claude-windows-drive-{}.json", False, None),
    # ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1A:
    # JSON sintaticamente inválido — antes desta ML, as 3 implementações silenciavam (`continue`
    # mudo compartilhado por credential_guard_hook_resolvable e git_branch_guard_hook_resolvable).
    "claude-invalid-json": ("cg-claude-invalid-json-{}.json", True, CG_MARKER_INVALID_JSON),
    # ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1B:
    # falha de LEITURA (diretório no lugar do arquivo) — antes desta ML, Go abortava `trackfw
    # validate` inteiro (exit 1, stdout vazio) e Node/Python silenciavam (exit 0, sem violation).
    "claude-unreadable": ("cg-claude-unreadable-{}.json", True, CG_MARKER_COULD_NOT_BE_READ),
    # ML-1B: falha de DECODIFICAÇÃO (arquivo inteiro salvo como UTF-16) — antes desta ML, Python
    # crashava com UnicodeDecodeError não capturada (traceback cru, exit 1, sem JSON estruturado).
    "claude-utf16": ("cg-claude-utf16-{}.json", True, CG_MARKER_NOT_UTF8),
}

for label, (pattern, expect_violation, msg_marker) in cases.items():
    results = {rt: load(pattern.format(rt)) for rt in ("go", "node", "py")}

    for rt, (exit_code, msgs) in results.items():
        if expect_violation:
            # P2 vacuity guard: a regra REALMENTE disparou neste runtime —
            # um exit != 0 por outra regra qualquer não prova nada aqui.
            if not msgs:
                raise SystemExit(
                    f"credential_guard_hook_resolvable parity ({label}/{rt}): expected "
                    f"violation from rule {CG_RULE!r}, none reported "
                    f"(exit={exit_code}) — fixture vacua ou regra regrediu"
                )
            if msg_marker and not all(msg_marker in m for m in msgs):
                raise SystemExit(
                    f"credential_guard_hook_resolvable parity ({label}/{rt}): "
                    f"mensagem inesperada — esperava {msg_marker!r} em todas: {msgs!r}"
                )
        else:
            # Casos "present": nenhuma violação da regra, qualquer que seja o
            # exit code geral (outras regras podem disparar nesta fixture mínima).
            if msgs:
                raise SystemExit(
                    f"credential_guard_hook_resolvable parity ({label}/{rt}): "
                    f"nenhuma violação da regra esperada (script presente), mas "
                    f"{rt} reportou: {msgs!r}"
                )

    # Comparação cross-CLI (apenas nos casos de detecção, onde msgs != []).
    if expect_violation:
        go_msgs, node_msgs, py_msgs = (results[rt][1] for rt in ("go", "node", "py"))
        if not (go_msgs == node_msgs == py_msgs):
            raise SystemExit(
                f"credential_guard_hook_resolvable parity ({label}): mensagens "
                f"divergem entre runtimes — go={go_msgs!r} node={node_msgs!r} "
                f"py={py_msgs!r}"
            )

print(
    "credential_guard_hook_resolvable parity checks passed "
    "(claude-absent/claude-present/cursor-absent/cursor-present/"
    "claude-noexec/claude-notype/claude-relativo/copilot-relativo-present/"
    "claude-pwd/claude-pwd-quoted/claude-absoluto/claude-git-toplevel/"
    "claude-outra-var/cursor-pwd/"
    "claude-tilde/claude-tilde-quoted/claude-tilde-user/claude-pwd-braced/claude-sh-c-pwd/claude-windows-drive/"
    "claude-invalid-json/claude-unreadable/claude-utf16, "
    "byte-identical across 3 CLIs)"
)
PY

echo "Validate JSON parity checks passed (credential_guard_hook_resolvable cross-CLI: claude-absent detection / claude-present baseline / cursor-absent relative-branch live / cursor-present false-positive guard / claude-noexec not-executable detection / claude-notype missing-type-command detection / claude-relativo bare-relative-path detection / copilot-relativo-present false-positive guard / claude-pwd \$PWD-class2-detected / claude-pwd-quoted quoted-\$PWD-class2-detected / claude-absoluto absolute-class1-silent / claude-git-toplevel git-toplevel-class1-silent / claude-outra-var other-var-class3-silent / cursor-pwd cursor-\$PWD-silent / claude-tilde tilde-class1-silent / claude-tilde-quoted quoted-tilde-class2-quoted-tilde-msg / claude-tilde-user named-user-tilde-class2-named-user-msg / claude-pwd-braced \${PWD}-class2-pwd-msg / claude-sh-c-pwd sh-c-\$PWD-class2-pwd-msg / claude-windows-drive windows-drive-class1-silent / claude-invalid-json malformed-json-detection / claude-unreadable read-failure-detection-no-crash / claude-utf16 not-valid-utf8-detection-distinct-from-invalid-json)"

# ---------------------------------------------------------------------------
# ROADMAP-2026-08-21-validate-detecta-hook-de-guard-na-forma-relativa-antiga,
# ML-2A: a regra `git_branch_guard_hook_resolvable` exercitada cross-CLI para
# a forma relativa antiga e o discriminante de falso-positivo (Cursor):
#
#   1. gbg-claude-relativo: .claude/settings.json com caminho relativo puro
#      para git-branch-guard, script PRESENTE → acusa "bare relative path"
#      nos 3 CLIs (validateGuardHookResolvable compartilhada, mesma lógica).
#   2. gbg-cursor-relativo-present: .cursor/hooks.json com relativo puro,
#      script PRESENTE → SILÊNCIO nos 3 CLIs (Cursor tem
#      requiresVarOrShellPrefix=false — relativo é a forma correta).
#
# Fixtures separados de credential-guard (marcador diferente:
# "trackfw-git-branch-guard.sh") para manter discriminação de regra limpa.
# ---------------------------------------------------------------------------
for gbg_fixture in gbg-claude-relativo gbg-cursor-relativo-present; do
  mkdir -p "$CG_TMP_CLEAN/$gbg_fixture/docs/roadmaps"/{wip,done}
  cat >"$CG_TMP_CLEAN/$gbg_fixture/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
done

# gbg-claude-relativo: .claude/settings.json com relativo puro para git-branch-guard.
mkdir -p "$CG_TMP_CLEAN/gbg-claude-relativo/.claude"
cat >"$CG_TMP_CLEAN/gbg-claude-relativo/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"scripts/trackfw-git-branch-guard.sh"}]}]}}
EOF
mkdir -p "$CG_TMP_CLEAN/gbg-claude-relativo/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$CG_TMP_CLEAN/gbg-claude-relativo/scripts/trackfw-git-branch-guard.sh"
chmod +x "$CG_TMP_CLEAN/gbg-claude-relativo/scripts/trackfw-git-branch-guard.sh"

# gbg-cursor-relativo-present: .cursor/hooks.json com relativo puro → silêncio.
mkdir -p "$CG_TMP_CLEAN/gbg-cursor-relativo-present/.cursor"
cat >"$CG_TMP_CLEAN/gbg-cursor-relativo-present/.cursor/hooks.json" <<'EOF'
{"version":1,"hooks":{"beforeShellExecution":[{"command":"scripts/trackfw-git-branch-guard.sh"}]}}
EOF
mkdir -p "$CG_TMP_CLEAN/gbg-cursor-relativo-present/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$CG_TMP_CLEAN/gbg-cursor-relativo-present/scripts/trackfw-git-branch-guard.sh"
chmod +x "$CG_TMP_CLEAN/gbg-cursor-relativo-present/scripts/trackfw-git-branch-guard.sh"

for gbg_fixture in gbg-claude-relativo gbg-cursor-relativo-present; do
  run_cg "$CG_TMP_CLEAN/$gbg_fixture-go.json"   "$CG_TMP_CLEAN/$gbg_fixture" "$GO_BIN" validate --json
  run_cg "$CG_TMP_CLEAN/$gbg_fixture-node.json" "$CG_TMP_CLEAN/$gbg_fixture" node "$ROOT_DIR/npm/bin/trackfw" validate --json
  run_cg "$CG_TMP_CLEAN/$gbg_fixture-py.json"   "$CG_TMP_CLEAN/$gbg_fixture" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json
done

python3 - "$CG_TMP_CLEAN" <<'PY'
import json
import os
import sys

tmp = sys.argv[1]

GBG_RULE = "git_branch_guard_hook_resolvable"
GBG_MARKER_BARE = "with a bare relative path"
# ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1A.
GBG_MARKER_INVALID_JSON = "is not valid JSON"
# ML-1B: mesmas 2 classes novas do bloco credential_guard_hook_resolvable acima.
GBG_MARKER_COULD_NOT_BE_READ = "could not be read"
GBG_MARKER_NOT_UTF8 = "is not valid UTF-8"


def load_gbg(name):
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as fh:
        payload = json.load(fh)
    with open(path + ".exit", encoding="utf-8") as fh:
        exit_code = int(fh.read().strip())
    all_items = payload.get("violations", []) + payload.get("warnings", [])
    matching = [item for item in all_items if item.get("rule") == GBG_RULE]
    msgs = sorted(item["message"] for item in matching)
    return exit_code, msgs


# label → (filename-pattern, expect_violation, msg_marker_or_None)
gbg_cases = {
    "claude-relativo":          ("gbg-claude-relativo-{}.json",          True,  GBG_MARKER_BARE),
    "cursor-relativo-present":  ("gbg-cursor-relativo-present-{}.json",  False, None),
    # ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1A:
    # reusa a MESMA saída da fixture cg-claude-invalid-json (bloco credential_guard_hook_resolvable
    # acima, mesmo arquivo .claude/settings.json corrompido) em vez de rodar `validate` de novo —
    # prova que o mesmo arquivo corrompido cega as DUAS regras simultaneamente, não só uma.
    "claude-invalid-json":      ("cg-claude-invalid-json-{}.json",       True,  GBG_MARKER_INVALID_JSON),
    # ML-1B: mesmo reuso de saída — o arquivo corrompido/ilegível/mal-codificado cega as DUAS
    # regras ao mesmo tempo, não só credential_guard_hook_resolvable.
    "claude-unreadable":        ("cg-claude-unreadable-{}.json",         True,  GBG_MARKER_COULD_NOT_BE_READ),
    "claude-utf16":             ("cg-claude-utf16-{}.json",              True,  GBG_MARKER_NOT_UTF8),
}

for label, (pattern, expect_violation, msg_marker) in gbg_cases.items():
    results = {rt: load_gbg(pattern.format(rt)) for rt in ("go", "node", "py")}

    for rt, (exit_code, msgs) in results.items():
        if expect_violation:
            if not msgs:
                raise SystemExit(
                    f"git_branch_guard_hook_resolvable parity ({label}/{rt}): expected "
                    f"violation from rule {GBG_RULE!r}, none reported "
                    f"(exit={exit_code}) — fixture vacua ou regra regrediu"
                )
            if msg_marker and not all(msg_marker in m for m in msgs):
                raise SystemExit(
                    f"git_branch_guard_hook_resolvable parity ({label}/{rt}): "
                    f"mensagem inesperada — esperava {msg_marker!r} em todas: {msgs!r}"
                )
        else:
            if msgs:
                raise SystemExit(
                    f"git_branch_guard_hook_resolvable parity ({label}/{rt}): "
                    f"nenhuma violacao da regra esperada (Cursor relativo e correto), mas "
                    f"{rt} reportou: {msgs!r}"
                )

    if expect_violation:
        go_msgs, node_msgs, py_msgs = (results[rt][1] for rt in ("go", "node", "py"))
        if not (go_msgs == node_msgs == py_msgs):
            raise SystemExit(
                f"git_branch_guard_hook_resolvable parity ({label}): mensagens "
                f"divergem entre runtimes — go={go_msgs!r} node={node_msgs!r} "
                f"py={py_msgs!r}"
            )

print(
    "git_branch_guard_hook_resolvable parity checks passed "
    "(gbg-claude-relativo bare-relative-path / gbg-cursor-relativo-present false-positive guard / "
    "claude-invalid-json malformed-json-detection / claude-unreadable read-failure-detection / "
    "claude-utf16 not-valid-utf8-detection, "
    "byte-identical across 3 CLIs)"
)
PY

echo "Validate JSON parity checks passed (git_branch_guard_hook_resolvable cross-CLI: gbg-claude-relativo bare-relative-path detection / gbg-cursor-relativo-present false-positive guard / claude-invalid-json malformed-json-detection / claude-unreadable read-failure-detection-no-crash / claude-utf16 not-valid-utf8-detection, same fixture files as credential_guard_hook_resolvable's cg-claude-invalid-json/cg-claude-unreadable/cg-claude-utf16)"
