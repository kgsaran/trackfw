#!/usr/bin/env bash
# check-ci-workflow-pin-parity.sh — falsifica a paridade byte a byte dos 3 templates de CI
# gerados pelos 3 CLIs (REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-
# nao-ha-como-pinar.md, AC8; roadmap ML-3A). Modelo de ameaça e alvos de falsificação em
# docs/roadmaps/wip/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-
# trackfw-version.md, seção "Resultado do ML-0A", seção 3.
#
# Três templates, não um — cada um com um builder por runtime:
#   trackfw-gate.yml         buildGitHubActionsWorkflowContent          (Go/Node/Python)
#   .gitlab-ci-trackfw.yml   buildGitLabCIWorkflowContent                (Go/Node/Python)
#   trackfw-validate.yml     BuildDiscoverGitHubActionsWorkflowContent   (Go/Node/Python)
#
# Os builders Go do primeiro par não são exportados — dump via um _test.go temporário
# (zz_dump_ci_workflow_pin_parity_test.go) escrito, executado com `go test -run`, e apagado
# via trap ANTES de qualquer saída do script (inclusive em falha), para nunca deixar um
# arquivo de teste esquecido quebrando o build de todo mundo.
#
# O discriminante de cada cenário de falsificação é a MENSAGEM QUE O PRÓPRIO GATE EMITE
# (check_version_pin/check_timeout_minutes/check_gitlab_timeout/check_discover_pin/
# compare_three_way abaixo) — nunca uma mensagem de CLI. Isso é possível porque essas mesmas
# funções são as que o gate usa para validar o conteúdo REAL: a falsificação injeta a
# regressão diretamente numa CÓPIA do dump real e chama a MESMA função, não um novo caminho.
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

# run_check LABEL FUNC ARGS... — chama FUNC, que imprime a razão em stdout SÓ quando falha
# (retorno != 0) e nada quando passa (retorno 0). Usado tanto para validação real quanto para
# falsificação (via assert_check_fails abaixo), garantindo que os dois caminhos exercitem a
# MESMA função.
run_check() {
  local label=$1; shift
  local out
  if out=$("$@" 2>&1); then
    ok "$label"
  else
    fail "$label" "$out"
  fi
}

# assert_check_fails LABEL PATTERN FUNC ARGS... — como run_check, mas inverte a expectativa:
# espera que FUNC falhe E que a razão impressa contenha PATTERN (o texto que o PRÓPRIO gate
# emite, nunca mensagem de CLI).
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
# Extratores de conteúdo real — um dump por runtime, escrevendo os 3 templates em DEST.
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
	if err := os.WriteFile("$dest/dv_go.yml", []byte(BuildDiscoverGitHubActionsWorkflowContent()), 0644); err != nil {
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

dump_node() {
  local dest=$1
  node -e "
const g = require('$ROOT_DIR/npm/src/generators/init.js');
const d = require('$ROOT_DIR/npm/src/commands/discover.js');
const fs = require('fs');
fs.writeFileSync('$dest/gh_node.yml', g.buildGitHubActionsWorkflowContent({}));
fs.writeFileSync('$dest/gl_node.yml', g.buildGitLabCIWorkflowContent({}));
fs.writeFileSync('$dest/dv_node.yml', d.buildDiscoverGitHubActionsWorkflowContent());
"
}

dump_py() {
  local dest=$1
  PYTHONPATH="$ROOT_DIR/pypi" python3 -c "
from trackfw.generators import init_gen as g
from trackfw.commands import discover as d
open('$dest/gh_py.yml', 'w').write(g.build_github_actions_workflow_content())
open('$dest/gl_py.yml', 'w').write(g.build_gitlab_ci_workflow_content())
open('$dest/dv_py.yml', 'w').write(d.build_discover_github_actions_workflow_content())
"
}

# ---------------------------------------------------------------------------
# Funções de checagem — cada uma É o gate para o invariante que nomeia. Usadas tanto na
# validação real (contra o dump real) quanto nos cenários de falsificação (contra uma cópia
# mutada do dump real), então a razão emitida é sempre a mesma nos dois caminhos.
# ---------------------------------------------------------------------------

# check_version_pin FILE EXPECTED — FILE precisa conter `TRACKFW_VERSION: "EXPECTED"` em
# alguma linha (env: do GitHub Actions ou variables: do GitLab CI, mesma sintaxe nos dois).
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

# check_timeout_minutes FILE — o job do GitHub Actions precisa ter `timeout-minutes: 10`.
check_timeout_minutes() {
  local file=$1
  if ! grep -qF 'timeout-minutes: 10' "$file"; then
    echo "timeout-minutes: 10 ausente no job do GitHub Actions"
    return 1
  fi
  return 0
}

# check_gitlab_timeout FILE — o job do GitLab CI precisa ter `timeout: 10 minutes`.
check_gitlab_timeout() {
  local file=$1
  if ! grep -qF 'timeout: 10 minutes' "$file"; then
    echo "timeout: 10 minutes ausente no job do GitLab CI"
    return 1
  fi
  return 0
}

# check_discover_pin FILE EXPECTED — o passo `go install .../trackfw@vEXPECTED` do
# trackfw-validate.yml precisa estar pinado (nunca `@latest`).
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

# compare_three_way FILE_GO FILE_NODE FILE_PY — diff byte a byte nos 3 pares possíveis,
# nomeando exatamente qual par diverge (AC8: "falha se um dos três divergir, nomeando qual").
compare_three_way() {
  local go_file=$1 node_file=$2 py_file=$3
  local reasons="" bad=0
  if ! diff -q "$go_file" "$node_file" >/dev/null 2>&1; then
    reasons="${reasons}go vs node divergem; "
    bad=1
  fi
  if ! diff -q "$go_file" "$py_file" >/dev/null 2>&1; then
    reasons="${reasons}go vs py divergem; "
    bad=1
  fi
  if ! diff -q "$node_file" "$py_file" >/dev/null 2>&1; then
    reasons="${reasons}node vs py divergem; "
    bad=1
  fi
  if [ "$bad" -eq 1 ]; then
    echo "$reasons"
    return 1
  fi
  return 0
}

# ---------------------------------------------------------------------------
# Versão canônica — internal/version/version.go é a fonte que os 3 runtimes pinam
# (npm/package.json "version" e pypi/trackfw/__init__.py's __version__ fallback são mantidos
# em lockstep manualmente; execução real de `trackfw --version` nos 3 CLIs já confirmou que
# reportam o mesmo valor no HEAD deste commit).
# ---------------------------------------------------------------------------
GO_VERSION=$(sed -n 's/.*Version = "\(.*\)"/\1/p' "$ROOT_DIR/internal/version/version.go")
if [ -z "$GO_VERSION" ]; then
  echo "check-ci-workflow-pin-parity: não consegui extrair a versão de internal/version/version.go" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Execução 1 — dump real dos 3 runtimes, comparação byte a byte, e validação dos invariantes
# de conteúdo (pin de versão, timeout, @v<versão> não-@latest).
# ---------------------------------------------------------------------------
RUN1="$WORK/run1"
mkdir -p "$RUN1"
dump_go "$RUN1"
dump_node "$RUN1"
dump_py "$RUN1"

run_check "pin-parity/github-actions/byte-identical" compare_three_way "$RUN1/gh_go.yml" "$RUN1/gh_node.yml" "$RUN1/gh_py.yml"
run_check "pin-parity/gitlab-ci/byte-identical"      compare_three_way "$RUN1/gl_go.yml" "$RUN1/gl_node.yml" "$RUN1/gl_py.yml"
run_check "pin-parity/discover-workflow/byte-identical" compare_three_way "$RUN1/dv_go.yml" "$RUN1/dv_node.yml" "$RUN1/dv_py.yml"

run_check "pin-parity/github-actions/version-pin"    check_version_pin "$RUN1/gh_go.yml" "$GO_VERSION"
run_check "pin-parity/gitlab-ci/version-pin"         check_version_pin "$RUN1/gl_go.yml" "$GO_VERSION"
run_check "pin-parity/github-actions/timeout-minutes" check_timeout_minutes "$RUN1/gh_go.yml"
run_check "pin-parity/gitlab-ci/timeout"             check_gitlab_timeout "$RUN1/gl_go.yml"
run_check "pin-parity/discover-workflow/version-pin" check_discover_pin "$RUN1/dv_go.yml" "$GO_VERSION"

# ---------------------------------------------------------------------------
# Idempotência — dump uma segunda vez sobre o mesmo commit; os 9 arquivos têm que sair
# byte-idênticos aos da primeira execução.
# ---------------------------------------------------------------------------
RUN2="$WORK/run2"
mkdir -p "$RUN2"
dump_go "$RUN2"
dump_node "$RUN2"
dump_py "$RUN2"

idempotent_check() {
  local f
  for f in gh_go.yml gl_go.yml dv_go.yml gh_node.yml gl_node.yml dv_node.yml gh_py.yml gl_py.yml dv_py.yml; do
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
# checagem usada acima, provando que a razão emitida é a do próprio gate.
# ---------------------------------------------------------------------------
FALS="$WORK/falsify"
mkdir -p "$FALS"

# (1) workflow sem TRACKFW_VERSION → reprova.
cp "$RUN1/gh_go.yml" "$FALS/gh-no-version.yml"
sed -i.bak '/TRACKFW_VERSION:/d' "$FALS/gh-no-version.yml"
rm -f "$FALS/gh-no-version.yml.bak"
assert_check_fails "falsify/github-actions/missing-version" "TRACKFW_VERSION ausente" \
  check_version_pin "$FALS/gh-no-version.yml" "$GO_VERSION"

# (2) workflow com versão diferente da que o binário reporta → reprova.
cp "$RUN1/gh_go.yml" "$FALS/gh-wrong-version.yml"
sed -i.bak "s/TRACKFW_VERSION: \"$GO_VERSION\"/TRACKFW_VERSION: \"0.0.0\"/" "$FALS/gh-wrong-version.yml"
rm -f "$FALS/gh-wrong-version.yml.bak"
assert_check_fails "falsify/github-actions/wrong-version" "TRACKFW_VERSION diverge" \
  check_version_pin "$FALS/gh-wrong-version.yml" "$GO_VERSION"

# (3) timeout-minutes ausente no GitHub Actions → reprova.
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

# (5) @latest no lugar de @v<versão> no trackfw-validate.yml → reprova.
cp "$RUN1/dv_go.yml" "$FALS/dv-latest.yml"
sed -i.bak "s/@v${GO_VERSION}/@latest/" "$FALS/dv-latest.yml"
rm -f "$FALS/dv-latest.yml.bak"
assert_check_fails "falsify/discover-workflow/latest-not-pinned" "go install usa @latest" \
  check_discover_pin "$FALS/dv-latest.yml" "$GO_VERSION"

# (6) par divergente é nomeado — corrompe só a cópia "node" de um trio real e prova que o
# comparador nomeia exatamente "go vs node", não um genérico "diverge".
mkdir -p "$FALS/pair"
cp "$RUN1/gh_go.yml"   "$FALS/pair/gh_go.yml"
cp "$RUN1/gh_node.yml" "$FALS/pair/gh_node.yml"
cp "$RUN1/gh_py.yml"   "$FALS/pair/gh_py.yml"
printf 'x' >> "$FALS/pair/gh_node.yml"
assert_check_fails "falsify/pair-divergence-named" "go vs node divergem" \
  compare_three_way "$FALS/pair/gh_go.yml" "$FALS/pair/gh_node.yml" "$FALS/pair/gh_py.yml"

# ---------------------------------------------------------------------------
# Guarda de vacuidade.
# ---------------------------------------------------------------------------
if [ "$SCENARIOS_RUN" -eq 0 ]; then
  echo "FAIL [check-ci-workflow-pin-parity]: guarda de vacuidade — nenhum cenário rodou" >&2
  exit 1
fi

echo
if [ "$FAIL" -eq 0 ]; then
  echo "check-ci-workflow-pin-parity: $SCENARIOS_RUN cenários OK"
else
  echo "check-ci-workflow-pin-parity: um ou mais cenários FALHARAM ($SCENARIOS_RUN executados)" >&2
fi
exit "$FAIL"
