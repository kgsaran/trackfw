#!/usr/bin/env bash
# check-ci-workflow-job-id-collision.sh — reprova a reintroducao da colisao de job id
# entre os dois workflows de CI que o produto gera (ML-1A,
# ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md).
#
# ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
# removed. npm/src/ and pypi/trackfw/ deleted. This gate now checks Go sources only.
#
# O defeito original: buildGitHubActionsWorkflowContent (trackfw-gate.yml) e
# BuildDiscoverGitHubActionsWorkflowContent (trackfw-validate.yml) declaravam o
# MESMO job id `governance`. Confirmado ao vivo no PR #241.
#
# assert_count, nao assert_has, porque a assinatura de um workflow pode aparecer
# mais de uma vez por engano.
set -euo pipefail

ROOT="${1:-.}"

fail=0
checked=0

# assert_count <label> <file> <exact-string> <expected-occurrences>
assert_count() {
  local label=$1 file=$2 needle=$3 expected_n=$4 got
  checked=$((checked + 1))
  if [[ ! -f "$ROOT/$file" ]]; then
    echo "check-ci-workflow-job-id-collision: $label — arquivo ausente: $file" >&2
    fail=1
    return
  fi
  got=$(grep -cF -- "$needle" "$ROOT/$file" || true)
  if [[ "$got" -ne "$expected_n" ]]; then
    echo "check-ci-workflow-job-id-collision: $label — esperava $expected_n ocorrencia(s), achou $got em $file" >&2
    echo "  esperado: $needle" >&2
    fail=1
  fi
}

GATE_ID="governance-install-script"
VALIDATE_ID="governance-go-install"
OLD_ID_LINE="  governance:"

# --- trackfw-gate.yml (buildGitHubActionsWorkflowContent) — Go only ---------------
# 2 occurrences expected: buildGitHubActionsWorkflowContent(isProducer bool) has two
# template branches (producer and consumer), each containing the same job id.
# The id must be identical in both branches — it is a required_status_checks contract.
assert_count "Go: trackfw-gate.yml usa $GATE_ID" \
  "internal/generators/scaffold.go" "$GATE_ID:" 2

# --- trackfw-validate.yml (BuildDiscoverGitHubActionsWorkflowContent) — Go only ----
# 2 occurrences expected: BuildDiscoverGitHubActionsWorkflowContent(isProducer bool) has
# two template branches (producer and consumer), each containing the same job id.
# The id must be identical in both branches — it is a required_status_checks contract.
assert_count "Go: trackfw-validate.yml usa $VALIDATE_ID" \
  "internal/generators/scaffold_doctor.go" "$VALIDATE_ID:" 2

# --- Anti-regressao: o id antigo colidente nao pode reaparecer nos geradores Go.
# npm/src/ e pypi/trackfw/ removidos em ML-3A (v8). ---
for f in \
  "internal/generators/scaffold.go" \
  "internal/generators/scaffold_doctor.go"; do
  assert_count "nao reintroduz o job id colidente '$OLD_ID_LINE' em $f" \
    "$f" "$OLD_ID_LINE" 0
done

# --- Os dois ids nunca podem ser o mesmo string. ---
checked=$((checked + 1))
if [[ "$GATE_ID" == "$VALIDATE_ID" ]]; then
  echo "check-ci-workflow-job-id-collision: GATE_ID e VALIDATE_ID sao identicos ($GATE_ID) — a colisao seria reintroduzida sob nome novo" >&2
  fail=1
fi

# --- Falsificacao -------------------------------------------------------------------
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-job-id-collision.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# (a) Controle: fixture com o id colidente antigo deve ser detectada.
cat > "$WORK/regressed_scaffold.go" <<'EOF'
jobs:
  governance:
    runs-on: ubuntu-latest
EOF
checked=$((checked + 1))
got=$(grep -cF -- "$OLD_ID_LINE" "$WORK/regressed_scaffold.go" || true)
if [[ "$got" -ne 1 ]]; then
  echo "check-ci-workflow-job-id-collision: falsify/controle — fixture com a colisao reintroduzida deveria contar 1 ocorrencia de '$OLD_ID_LINE', contou $got" >&2
  fail=1
else
  echo "OK falsify/controle: fixture com job id colidente antigo e detectada (1 ocorrencia)"
fi

# (b) O arquivo real tem que ter ZERO ocorrencias do id antigo.
checked=$((checked + 1))
got=$(grep -cF -- "$OLD_ID_LINE" "$ROOT/internal/generators/scaffold.go" || true)
if [[ "$got" -ne 0 ]]; then
  echo "check-ci-workflow-job-id-collision: falsify/real — internal/generators/scaffold.go ainda contem o job id colidente antigo" >&2
  fail=1
else
  echo "OK falsify/real: internal/generators/scaffold.go nao contem mais '$OLD_ID_LINE'"
fi

# --- Guarda de vacuidade ---
# 2 assert_count (job IDs Go — GATE_ID now expects 2 occurrences, VALIDATE_ID expects 2)
# + 2 assert_count (anti-regression Go) + 1 (ID equality) + 2 (falsify) = 7
expected=7
if [[ "$checked" -ne "$expected" ]]; then
  echo "check-ci-workflow-job-id-collision: vacuidade — esperava checar $expected assinaturas, checou $checked" >&2
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "check-ci-workflow-job-id-collision: FALHOU — job id colidente entre trackfw-gate.yml e trackfw-validate.yml" >&2
  exit 1
fi

echo "check-ci-workflow-job-id-collision: OK — $checked assinaturas confirmadas, sem colisao de job id (Go only — v8 single-runtime)"
