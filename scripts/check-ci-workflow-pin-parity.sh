#!/usr/bin/env bash
# check-ci-workflow-pin-parity.sh — pins the CI workflow template content generated
# by the Go CLI (REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-
# nao-ha-como-pinar.md, AC8; roadmap ML-3A).
#
# ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
# removed. npm/src/ and pypi/trackfw/ deleted. This gate now asserts the Go
# binary's CI template output only — cross-runtime comparison removed.
#
# Três templates, via builder Go:
#   trackfw-gate.yml         buildGitHubActionsWorkflowContent        (Go)
#   .gitlab-ci-trackfw.yml   buildGitLabCIWorkflowContent              (Go)
#   trackfw-validate.yml     BuildDiscoverGitHubActionsWorkflowContent (Go)
#
# Os builders Go não são exportados — dump via um _test.go temporário
# (zz_dump_ci_workflow_pin_parity_test.go) escrito, executado com `go test -run`, e apagado
# via trap ANTES de qualquer saída do script (inclusive em falha), para nunca deixar um
# arquivo de teste esquecido quebrando o build de todo mundo.
#
# O discriminante de cada cenário de falsificação é a MENSAGEM QUE O PRÓPRIO GATE EMITE
# (check_version_pin/check_timeout_minutes/check_gitlab_timeout/check_discover_pin
# abaixo) — nunca uma mensagem de CLI.
set -euo pipefail

export PYTHONIOENCODING=utf-8

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-ci-pin-parity.XXXXXX")
DUMP_TEST="$ROOT_DIR/internal/generators/zz_dump_ci_workflow_pin_parity_test.go"

cleanup() {
  rm -f "$DUMP_TEST"
  rm -rf "$WORK"
}
trap cleanup EXIT

FAIL=0
SCENARIOS_RUN=0
ok()   { echo "OK   [$1]"; SCENARIOS_RUN=$((SCENARIOS_RUN + 1)); }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; SCENARIOS_RUN=$((SCENARIOS_RUN + 1)); }

# run_check LABEL FUNC ARGS...
run_check() {
  local label=$1; shift
  local out
  if out=$("$@" 2>&1); then
    ok "$label"
  else
    fail "$label" "$out"
  fi
}

# assert_check_fails LABEL PATTERN FUNC ARGS...
assert_check_fails() {
  local label=$1 pattern=$2; shift 2
  local out
  if out=$("$@" 2>&1); then
    fail "$label" "esperava falha (motivo contendo '$pattern'), mas o check passou silenciosamente"
    return
  fi
  if ! grep -qF "$pattern" <<<"$out"; then
    fail "$label" "falhou mas sem o motivo esperado ('$pattern'); saída: $out"
    return
  fi
  ok "$label"
}

# ---------------------------------------------------------------------------
# Extrator de conteúdo — dump Go via _test.go temporário.
# ---------------------------------------------------------------------------

dump_go() {
  local dest=$1
  cat > "$DUMP_TEST" <<GOEOF
package generators

import (
	"os"
	"testing"
)

func TestZZDumpCIWorkflowPinParity(t *testing.T) {
	if err := os.WriteFile("$dest/gh_go.yml", []byte(buildGitHubActionsWorkflowContent(Config{})), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("$dest/gl_go.yml", []byte(buildGitLabCIWorkflowContent(Config{})), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("$dest/dv_go_consumer.yml", []byte(BuildDiscoverGitHubActionsWorkflowContent(false)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("$dest/dv_go_producer.yml", []byte(BuildDiscoverGitHubActionsWorkflowContent(true)), 0644); err != nil {
		t.Fatal(err)
	}
}
GOEOF
  if ! (cd "$ROOT_DIR" && go test ./internal/generators/ -run TestZZDumpCIWorkflowPinParity -count=1) >"$dest/go-test.log" 2>&1; then
    rm -f "$DUMP_TEST"
    echo "check-ci-workflow-pin-parity: dump Go (go test) falhou:" >&2
    cat "$dest/go-test.log" >&2
    exit 1
  fi
  rm -f "$DUMP_TEST"
}

# ---------------------------------------------------------------------------
# Funções de checagem.
# ---------------------------------------------------------------------------

# check_version_pin FILE EXPECTED
check_version_pin() {
  local file=$1 expected=$2
  local line found
  line=$(grep -m1 'TRACKFW_VERSION: "' "$file" || true)
  if [ -z "$line" ]; then
    echo "TRACKFW_VERSION ausente do bloco env/variables"
    return 1
  fi
  found=$(printf '%s' "$line" | sed -n 's/.*TRACKFW_VERSION: "\([^"]*\)".*/\1/p')
  if [ "$found" != "$expected" ]; then
    echo "TRACKFW_VERSION diverge da versão do binário (esperado '$expected', encontrado '$found')"
    return 1
  fi
  return 0
}

# check_timeout_minutes FILE
check_timeout_minutes() {
  local file=$1
  if ! grep -qF 'timeout-minutes: 10' "$file"; then
    echo "timeout-minutes: 10 ausente no job do GitHub Actions"
    return 1
  fi
  return 0
}

# check_gitlab_timeout FILE
check_gitlab_timeout() {
  local file=$1
  if ! grep -qF 'timeout: 10 minutes' "$file"; then
    echo "timeout: 10 minutes ausente no job do GitLab CI"
    return 1
  fi
  return 0
}

# check_discover_pin FILE EXPECTED
# Verifies the consumer discover template pins go install to @v<expected>.
check_discover_pin() {
  local file=$1 expected=$2
  if grep -qF '@latest' "$file"; then
    echo "go install usa @latest em vez de @v${expected} pinada"
    return 1
  fi
  if ! grep -qF "@v${expected}" "$file"; then
    echo "go install não contém @v${expected} pinada"
    return 1
  fi
  return 0
}

# check_producer_no_go_install FILE
# Verifies the producer discover template compiles from source (no 'run: go install' step).
# Uses "run: go install" (without leading dash) so YAML comments that mention go install
# in the warning text are not mistaken for an executable step.
check_producer_no_go_install() {
  local file=$1
  if grep -qFe 'run: go install' "$file"; then
    echo "template de produtor contém 'run: go install' — deve compilar do fonte"
    return 1
  fi
  if ! grep -qFe 'run: go build' "$file"; then
    echo "template de produtor não contém 'run: go build'"
    return 1
  fi
  return 0
}

# check_action_pins FILE MIN_VERSION
# Verifies that actions/checkout and actions/setup-go in FILE use @vN where N >= MIN_VERSION.
# Detects action version regression (AC4).
check_action_pins() {
  local file=$1 min=$2
  for action in "actions/checkout" "actions/setup-go"; do
    local rv
    # `|| true` prevents set -euo pipefail from aborting when grep finds no match —
    # the caller gets rv="" and the -z check below emits the "pin não encontrado" message
    # instead of an opaque pipeline failure (distinguishes "not found" from "grep failed").
    rv=$(grep -m1 "uses: ${action}@v" "$file" | sed -n 's/.*@v\([0-9]*\).*/\1/p') || true
    if [ -z "$rv" ]; then
      echo "${action} pin não encontrado em $file"
      return 1
    fi
    if [ "$rv" -lt "$min" ]; then
      echo "${action}@v${rv} é inferior ao mínimo esperado @v${min}"
      return 1
    fi
  done
  return 0
}

# ---------------------------------------------------------------------------
# Versão canônica.
# ---------------------------------------------------------------------------
GO_VERSION=$(sed -n 's/.*Version = "\(.*\)"/\1/p' "$ROOT_DIR/internal/version/version.go")
if [ -z "$GO_VERSION" ]; then
  echo "check-ci-workflow-pin-parity: não consegui extrair a versão de internal/version/version.go" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Execução 1 — dump Go, validação dos invariantes de conteúdo.
# ---------------------------------------------------------------------------
RUN1="$WORK/run1"
mkdir -p "$RUN1"
dump_go "$RUN1"

run_check "pin-parity/github-actions/version-pin"              check_version_pin "$RUN1/gh_go.yml" "$GO_VERSION"
run_check "pin-parity/gitlab-ci/version-pin"                   check_version_pin "$RUN1/gl_go.yml" "$GO_VERSION"
run_check "pin-parity/github-actions/timeout-minutes"          check_timeout_minutes "$RUN1/gh_go.yml"
run_check "pin-parity/gitlab-ci/timeout"                       check_gitlab_timeout "$RUN1/gl_go.yml"
run_check "pin-parity/discover-workflow/consumer-version-pin"  check_discover_pin "$RUN1/dv_go_consumer.yml" "$GO_VERSION"
run_check "pin-parity/discover-workflow/producer-no-go-install" check_producer_no_go_install "$RUN1/dv_go_producer.yml"
run_check "pin-parity/discover-workflow/consumer-action-pins"  check_action_pins "$RUN1/dv_go_consumer.yml" 7
run_check "pin-parity/discover-workflow/producer-action-pins"  check_action_pins "$RUN1/dv_go_producer.yml" 7

# ---------------------------------------------------------------------------
# Idempotência — dump uma segunda vez; os 3 arquivos têm que sair byte-idênticos.
# ---------------------------------------------------------------------------
RUN2="$WORK/run2"
mkdir -p "$RUN2"
dump_go "$RUN2"

idempotent_check() {
  local f
  for f in gh_go.yml gl_go.yml dv_go_consumer.yml dv_go_producer.yml; do
    if ! diff -q "$RUN1/$f" "$RUN2/$f" >/dev/null 2>&1; then
      echo "arquivo $f diverge entre a 1a e a 2a execução do gate sobre o mesmo commit"
      return 1
    fi
  done
  return 0
}
run_check "pin-parity/idempotency" idempotent_check

# ---------------------------------------------------------------------------
# Falsificação — injeta cada regressão numa CÓPIA do dump real e chama a MESMA função de
# checagem usada acima.
# ---------------------------------------------------------------------------
FALS="$WORK/falsify"
mkdir -p "$FALS"

# (1) workflow sem TRACKFW_VERSION → reprova.
cp "$RUN1/gh_go.yml" "$FALS/gh-no-version.yml"
sed -i.bak '/TRACKFW_VERSION:/d' "$FALS/gh-no-version.yml"
rm -f "$FALS/gh-no-version.yml.bak"
assert_check_fails "falsify/github-actions/missing-version" "TRACKFW_VERSION ausente" \
  check_version_pin "$FALS/gh-no-version.yml" "$GO_VERSION"

# (2) workflow com versão diferente → reprova.
cp "$RUN1/gh_go.yml" "$FALS/gh-wrong-version.yml"
sed -i.bak "s/TRACKFW_VERSION: \"$GO_VERSION\"/TRACKFW_VERSION: \"0.0.0\"/" "$FALS/gh-wrong-version.yml"
rm -f "$FALS/gh-wrong-version.yml.bak"
assert_check_fails "falsify/github-actions/wrong-version" "TRACKFW_VERSION diverge" \
  check_version_pin "$FALS/gh-wrong-version.yml" "$GO_VERSION"

# (3) timeout-minutes ausente → reprova.
cp "$RUN1/gh_go.yml" "$FALS/gh-no-timeout.yml"
sed -i.bak '/timeout-minutes: 10/d' "$FALS/gh-no-timeout.yml"
rm -f "$FALS/gh-no-timeout.yml.bak"
assert_check_fails "falsify/github-actions/missing-timeout-minutes" "timeout-minutes: 10 ausente" \
  check_timeout_minutes "$FALS/gh-no-timeout.yml"

# (4) timeout: 10 minutes ausente no GitLab CI → reprova.
cp "$RUN1/gl_go.yml" "$FALS/gl-no-timeout.yml"
sed -i.bak '/timeout: 10 minutes/d' "$FALS/gl-no-timeout.yml"
rm -f "$FALS/gl-no-timeout.yml.bak"
assert_check_fails "falsify/gitlab-ci/missing-timeout" "timeout: 10 minutes ausente" \
  check_gitlab_timeout "$FALS/gl-no-timeout.yml"

# (5) @latest no lugar de @v<versão> no template de consumidor → reprova.
cp "$RUN1/dv_go_consumer.yml" "$FALS/dv-consumer-latest.yml"
sed -i.bak "s/@v${GO_VERSION}/@latest/" "$FALS/dv-consumer-latest.yml"
rm -f "$FALS/dv-consumer-latest.yml.bak"
assert_check_fails "falsify/discover-workflow/consumer-latest-not-pinned" "go install usa @latest" \
  check_discover_pin "$FALS/dv-consumer-latest.yml" "$GO_VERSION"

# (6) 'go install' injetado no template de produtor → reprova (AC5/AC8).
cp "$RUN1/dv_go_producer.yml" "$FALS/dv-producer-go-install.yml"
sed -i.bak "s|go build -o /usr/local/bin/trackfw ./cmd/trackfw|go install github.com/kgsaran/trackfw/cmd/trackfw@v${GO_VERSION}|" "$FALS/dv-producer-go-install.yml"
rm -f "$FALS/dv-producer-go-install.yml.bak"
assert_check_fails "falsify/discover-workflow/producer-has-go-install" "template de produtor contém 'run: go install'" \
  check_producer_no_go_install "$FALS/dv-producer-go-install.yml"

# (7) regressão de versão de action no template de consumidor → reprova (AC4).
cp "$RUN1/dv_go_consumer.yml" "$FALS/dv-consumer-action-regression.yml"
sed -i.bak "s|actions/checkout@v7|actions/checkout@v4|" "$FALS/dv-consumer-action-regression.yml"
rm -f "$FALS/dv-consumer-action-regression.yml.bak"
assert_check_fails "falsify/discover-workflow/consumer-action-regression" "actions/checkout@v4 é inferior" \
  check_action_pins "$FALS/dv-consumer-action-regression.yml" 7

# ---------------------------------------------------------------------------
# Guarda de vacuidade.
# ---------------------------------------------------------------------------
if [ "$SCENARIOS_RUN" -eq 0 ]; then
  echo "FAIL [check-ci-workflow-pin-parity]: guarda de vacuidade — nenhum cenário rodou" >&2
  exit 1
fi

echo
if [ "$FAIL" -eq 0 ]; then
  echo "check-ci-workflow-pin-parity: $SCENARIOS_RUN cenários OK (Go only — v8 single-runtime)"
else
  echo "check-ci-workflow-pin-parity: um ou mais cenários FALHARAM ($SCENARIOS_RUN executados)" >&2
fi
exit "$FAIL"
