#!/usr/bin/env bash
# P4 — Falsificação dos gates de paridade (REQ-2026-07-26-robustez-gates)
#
# Cada gate deve reprovar um cenário negativo concreto — "CI verde" sem essa
# prova é um gate não-verificado. Este script monta o cenário quebrado, afirma
# que o gate retorna exit != 0 E que a saída contém o diagnóstico esperado.
# Roda dentro de `make quality`, após os gates positivos.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-falsify.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# $HOME sintético e isolado por padrão para o script INTEIRO — nunca o real. Sem isto, qualquer
# cenário que rode `trackfw validate` (ou qualquer comando que passe por Validate()/
# ValidateTagged()) sem controlar $HOME explicitamente enxerga o escopo GLOBAL de guards de quem
# roda o gate: desde ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
# integridade-independente-de-fiacao (ML-3A),
# git_branch_guard_script_integrity/credential_guard_script_integrity disparam pela EXISTÊNCIA do
# script em ~/.trackfw/scripts/, não mais só quando há fiação — um $HOME real com o harness
# instalado e o script desatualizado (o próprio caso que motivou aquela REQ) faria dezenas de
# cenários pré-existentes, que nunca tiveram nada a ver com guards, reportar um warning
# inesperado. Mesmo precedente do Cenário 46.
#
# GOPATH/GOCACHE/GOMODCACHE são fixados nos valores REAIS antes de isolar $HOME: os binários
# isolados dos Cenários 25+ chamam `go build`, que resolve esses três a partir de $HOME por
# padrão — sem fixá-los explicitamente, um $HOME sintético novo a cada run forçaria `go build` a
# rebaixar o módulo inteiro (lento) e o cache do Go grava arquivos read-only que
# `trap rm -rf "$WORK"` não consegue apagar (permission denied). Isolar só $HOME, com
# GOPATH/GOCACHE/GOMODCACHE reais, dá o melhor dos dois mundos: nenhum `trackfw validate` deste
# script enxerga guards globais reais, e `go build` continua rápido e usa o cache real.
export GOPATH="${GOPATH:-$(go env GOPATH)}"
export GOCACHE="${GOCACHE:-$(go env GOCACHE)}"
export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"
export HOME="$WORK/home"
mkdir -p "$HOME"

# ---------------------------------------------------------------------------
# Helper: assert que o comando retorna exit != 0 E a saída contém o diagnóstico.
# Uso: assert_fails_with LABEL DIAGNOSTIC_PATTERN CMD [ARGS...]
# ---------------------------------------------------------------------------
assert_fails_with() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "FAIL [falsify/$label]: saiu com 0, esperava != 0" >&2
    echo "  output: $out" >&2
    exit 1
  fi
  if ! grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label]: saiu com $status mas falta diagnóstico '$pattern'" >&2
    echo "  output: $out" >&2
    exit 1
  fi
  echo "OK   [falsify/$label]"
}

# ---------------------------------------------------------------------------
# Helper: prova de não-vacuidade quando a regra sob teste é desligável via
# `rules: <nome>: off` (não por edição de código-fonte + rebuild — usado
# pelos Cenários 49/50, que não têm permissão de tocar internal/validator).
# Roda o MESMO comando/critério de assert_fails_with (exit != 0 E mensagem
# presente) contra um fixture com a regra desligada, mas aqui o resultado
# ESPERADO é que esse critério NÃO seja atendido — ou seja, que o braço de
# detecção, se rodasse contra este fixture, ecoaria a mesma linha de FAIL
# que assert_fails_with produziria ("saiu com 0, esperava != 0"). Se o
# critério FOR atendido mesmo com a regra desligada, isso prova que o braço
# de detecção real não depende desta regra — e o cenário reprova aqui.
# ---------------------------------------------------------------------------
assert_would_now_fail() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -ne 0 ]] && grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label/non-vacuity]: com a regra desligada (rules: ...: off), o braço de detecção AINDA passaria (saiu $status e contém '$pattern') — a asserção de detecção não depende desta regra estar ativa" >&2
    echo "  output: $out" >&2
    exit 1
  fi
  echo "PROOF [falsify/$label/non-vacuity]: com a regra desligada, o braço de detecção FALHARIA — assert_fails_with ecoaria \"FAIL [falsify/$label/detected]: saiu com $status, esperava != 0\" (mensagem '$pattern' ausente, exit=$status). Saída real da árvore desligada:"
  echo "$out"
}

# ---------------------------------------------------------------------------
# Helper: cria a estrutura mínima do npm em $1 para os gates que o usam.
# Copia bin/ e src/ do ROOT_DIR; node_modules é symlink (apenas leitura).
# Passa $2 como lista de arquivos extras de src/ a copiar (opcional).
# ---------------------------------------------------------------------------
setup_npm_tree() {
  local dest=$1
  mkdir -p "$dest/npm/bin" "$dest/npm/src"
  cp "$ROOT_DIR/npm/bin/trackfw" "$dest/npm/bin/trackfw"
  # node_modules: symlink para evitar cópia cara
  ln -s "$ROOT_DIR/npm/node_modules" "$dest/npm/node_modules"
  cp "$ROOT_DIR/npm/package.json" "$dest/npm/package.json"
  # Copiar src/ inteiro para que require('./X') funcione
  cp -r "$ROOT_DIR/npm/src/." "$dest/npm/src/"
}

# ---------------------------------------------------------------------------
# Helper: compila um binário Go isolado e falha com diagnóstico explícito.
# Sem isso, `set -e` aborta o harness antes dos cenários seguintes e esconde
# stderr do `go build`, tornando a prova P4 opaca.
# ---------------------------------------------------------------------------
build_go_or_fail() {
  local label=$1
  local module_dir=$2
  local output_bin=$3
  local log_file="$WORK/${label}.log"

  set +e
  (
    cd "$module_dir" &&
      env GOCACHE="$WORK/go-build-cache" go build -o "$output_bin" ./cmd/trackfw
  ) >"$log_file" 2>&1
  local status=$?
  set -e

  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: go build saiu com $status" >&2
    echo "  command: (cd \"$module_dir\" && GOCACHE=\"$WORK/go-build-cache\" go build -o \"$output_bin\" ./cmd/trackfw)" >&2
    echo "  output:" >&2
    sed 's/^/    /' "$log_file" >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Helper: emite scripts/trackfw-git-branch-guard.sh a partir de uma cópia de
# módulo Go isolada (mesmo padrão de build_go_or_fail: copia cmd/+internal/+
# go.mod/go.sum, corrompe UM arquivo, reconstrói) — usado pelos Cenários 58/59
# (ML-1A, ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-
# plugins-e-da-release-7-0-0.md). Em vez de reconstruir o binário `trackfw`
# inteiro (que exigiria simular um wizard interativo de `init` só para chegar
# ao script), adiciona um `cmd/` efêmero PRÓPRIO DA CÓPIA ISOLADA (nunca no
# ROOT_DIR real) que chama generators.GenerateGitBranchGuardScript
# diretamente — mesmo princípio de isolamento de build_go_or_fail, sem
# depender de nenhum subcomando CLI existir.
run_go_guard_dump() {
  local label=$1
  local module_dir=$2
  local out_dir=$3
  local log_file="$WORK/${label}.log"

  mkdir -p "$module_dir/zz_dumpguard" "$out_dir"
  cat > "$module_dir/zz_dumpguard/main.go" <<'GOEOF'
package main

import (
	"os"

	"github.com/kgsaran/trackfw/internal/generators"
)

func main() {
	if err := generators.GenerateGitBranchGuardScript(os.Args[1]); err != nil {
		panic(err)
	}
}
GOEOF

  set +e
  (
    cd "$module_dir" &&
      env GOCACHE="$WORK/go-build-cache" go run ./zz_dumpguard "$out_dir"
  ) >"$log_file" 2>&1
  local status=$?
  set -e

  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: go run ./zz_dumpguard saiu com $status" >&2
    echo "  output:" >&2
    sed 's/^/    /' "$log_file" >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Helper: invoca scripts/trackfw-git-branch-guard.sh com um payload JSON via
# stdin e afirma o exit code esperado (0 = allow silencioso, 2 = block). Os
# helpers assert_fails_with/assert_succeeds não servem aqui porque não
# oferecem stdin — o guard só lê o comando via stdin (formato de hook real).
assert_guard_exit() {
  local label=$1
  local script=$2
  local payload=$3
  local want=$4
  local out status
  set +e
  out=$(bash "$script" <<<"$payload" 2>&1)
  status=$?
  set -e
  if [[ "$status" -ne "$want" ]]; then
    echo "FAIL [falsify/$label]: exit $status, esperava $want" >&2
    echo "  payload: $payload" >&2
    echo "  output: $out" >&2
    exit 1
  fi
  echo "OK   [falsify/$label]: exit $status"
}

# assert_writer_no_epipe: reproduz o repro exato da auditoria do arquiteto
# (ML-1B, ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
# integridade-independente-de-fiacao.md) -- um ESCRITOR EXTERNO real
# (subprocesso python3, não here-string do bash, que via <<< pode mascarar o
# EPIPE por já escrever num arquivo temporário) grava o payload JSON no pipe
# de stdin do guard e captura o stderr do PRÓPRIO ESCRITOR (não o do guard).
# want_writer_ok=1 -> escritor deve terminar sem erro (stderr vazio) e o
# guard deve sair com want_guard_exit; want_writer_ok=0 -> escritor DEVE
# receber EPIPE (stderr não-vazio), provando que o cenário de detecção é
# genuíno (não vácuo) antes de testar o build corrompido.
assert_writer_no_epipe() {
  local label=$1
  local script=$2
  local payload=$3
  local want_guard_exit=$4
  local want_writer_ok=$5
  local werr guard_status writer_status
  werr="$WORK/${label//\//_}.werr"
  set +e
  # O payload NUNCA vai por argv (ARG_MAX/MAX_ARG_STRLEN do Linux: 128 KB por
  # argumento -- um payload de 200 KB embutido na fonte do -c estourava e o
  # escritor nem chegava a nascer, tornando o braço vácuo). Ele entra via a
  # here-string do bash (<<<), que é implementada com um arquivo temporário e
  # redirecionamento de fd -- não conta para o limite de argv/envp. O python3
  # lê do PRÓPRIO stdin (o arquivo temporário do bash) e escreve no seu
  # stdout, que É o pipe real para o guard -- preserva o mecanismo que este
  # helper existe para provar (escritor externo, pipe de verdade, EPIPE
  # observável via BrokenPipeError no stderr do escritor).
  python3 -c "
import sys
data = sys.stdin.read()
sys.stdout.write(data)
sys.stdout.flush()
" <<<"$payload" 2>"$werr" | bash "$script" >/dev/null 2>&1
  guard_status=${PIPESTATUS[1]}
  writer_status=${PIPESTATUS[0]}
  set -e
  if [[ "$guard_status" -ne "$want_guard_exit" ]]; then
    echo "FAIL [falsify/$label]: guard exit $guard_status, esperava $want_guard_exit" >&2
    exit 1
  fi
  local writer_had_error=0
  [[ -s "$werr" ]] && writer_had_error=1
  if [[ "$want_writer_ok" -eq 1 && "$writer_had_error" -eq 1 ]]; then
    echo "FAIL [falsify/$label]: escritor recebeu erro (EPIPE esperado ausente)" >&2
    echo "  writer stderr:" >&2
    sed 's/^/    /' "$werr" >&2
    exit 1
  fi
  if [[ "$want_writer_ok" -eq 0 && "$writer_had_error" -eq 0 ]]; then
    echo "FAIL [falsify/$label]: escritor terminou limpo, EPIPE esperado não ocorreu (cenário vácuo)" >&2
    exit 1
  fi
  echo "OK   [falsify/$label]: guard exit $guard_status, writer_status=$writer_status, escritor_erro=$writer_had_error"
}

# ---------------------------------------------------------------------------
# Cenário 1 — check-static-assets.sh: byte drift em npm/src/serve/static/app.js
# ---------------------------------------------------------------------------
T1="$WORK/s1"
mkdir -p "$T1/scripts" "$T1/internal/serve/static" \
         "$T1/npm/src/serve/static" "$T1/pypi/trackfw/serve/static"
cp -r "$ROOT_DIR/internal/serve/static/." "$T1/internal/serve/static/"
cp -r "$ROOT_DIR/npm/src/serve/static/." "$T1/npm/src/serve/static/"
cp -r "$ROOT_DIR/pypi/trackfw/serve/static/." "$T1/pypi/trackfw/serve/static/"
cp "$ROOT_DIR/scripts/check-static-assets.sh" "$T1/scripts/"
# Corromper: adicionar byte extra em app.js do npm
printf 'X' >> "$T1/npm/src/serve/static/app.js"

assert_fails_with "static-assets/byte-drift" \
  "Static asset byte drift" \
  bash "$T1/scripts/check-static-assets.sh"

# ---------------------------------------------------------------------------
# Cenário 2 — check-integration-assets.sh: byte drift em pypi/catalog.json
# ---------------------------------------------------------------------------
T2="$WORK/s2"
mkdir -p "$T2/scripts" \
         "$T2/internal/integrations" \
         "$T2/npm/src/integrations" \
         "$T2/pypi/trackfw/integrations"
# Copiar apenas as árvores que o gate compara (sem ligar ao Go)
cp -r "$ROOT_DIR/internal/integrations/assets/." "$T2/internal/integrations/assets"
cp -r "$ROOT_DIR/npm/src/integrations/assets/." "$T2/npm/src/integrations/assets"
cp -r "$ROOT_DIR/pypi/trackfw/integrations/assets/." "$T2/pypi/trackfw/integrations/assets"
cp "$ROOT_DIR/npm/package.json" "$T2/npm/package.json"
cp "$ROOT_DIR/pypi/pyproject.toml" "$T2/pypi/pyproject.toml"
cp "$ROOT_DIR/scripts/check-integration-assets.sh" "$T2/scripts/"
# Corromper: adicionar byte extra em catalog.json do pypi
printf 'X' >> "$T2/pypi/trackfw/integrations/assets/catalog.json"

assert_fails_with "integration-assets/byte-drift" \
  "Integration asset byte drift" \
  bash "$T2/scripts/check-integration-assets.sh"

# ---------------------------------------------------------------------------
# Cenário 3 — check-identity-parity.sh: slug vectors drift em npm fixture
#
# O gate checa os fixtures ANTES de iniciar o ciclo de install (linha 51-62),
# então a prova é rápida e não requer execução dos CLIs.
# GO_BIN aponta para o binário já compilado — o if do script detecta e pula.
# ---------------------------------------------------------------------------
T3="$WORK/s3"
mkdir -p "$T3/scripts" \
         "$T3/internal/identity/testdata" \
         "$T3/npm/tests/fixtures"
cp "$ROOT_DIR/internal/identity/testdata/slug_vectors.json" \
   "$T3/internal/identity/testdata/slug_vectors.json"
cp "$ROOT_DIR/npm/tests/fixtures/slug_vectors.json" \
   "$T3/npm/tests/fixtures/slug_vectors.json"
cp "$ROOT_DIR/scripts/check-identity-parity.sh" "$T3/scripts/"
# Corromper: adicionar byte extra no fixture do npm
printf 'X' >> "$T3/npm/tests/fixtures/slug_vectors.json"

assert_fails_with "identity-parity/slug-drift" \
  "slug vectors drift" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T3/scripts/check-identity-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 3b — check-identity-parity.sh: catálogo ganha superfície nova de
#               agente → gate derivado do catálogo deve tentar exercitá-la e
#               reprovar enquanto os CLIs/binários não a reconhecerem.
#
# Objetivo (ML-1B): provar que o gate não depende de edição manual de uma lista
# TARGETS. O catálogo temporário adiciona `codex=experimental`; nenhum arquivo
# real do workspace é alterado.
# ---------------------------------------------------------------------------
T3B="$WORK/s3b"
mkdir -p "$T3B/scripts" "$T3B/internal/integrations/assets" \
         "$T3B/internal/identity/testdata" "$T3B/npm/tests/fixtures" \
         "$T3B/pypi/tests/fixtures"
cp "$ROOT_DIR/scripts/check-identity-parity.sh" "$T3B/scripts/"
cp "$ROOT_DIR/internal/integrations/assets/catalog.json" "$T3B/internal/integrations/assets/catalog.json"
cp "$ROOT_DIR/internal/identity/testdata/slug_vectors.json" \
   "$T3B/internal/identity/testdata/slug_vectors.json"
cp "$ROOT_DIR/npm/tests/fixtures/slug_vectors.json" \
   "$T3B/npm/tests/fixtures/slug_vectors.json"
cp "$ROOT_DIR/pypi/tests/fixtures/slug_vectors.json" \
   "$T3B/pypi/tests/fixtures/slug_vectors.json"

python3 - "$T3B/internal/integrations/assets/catalog.json" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
catalog = json.loads(path.read_text(encoding="utf-8"))
for target in catalog["targets"]:
    if target["id"] == "codex":
        target["surfaces"].append({
            "id": "experimental",
            "name": "Codex Experimental",
            "scopes": ["project"],
            "capabilities": {
                "agents": {
                    "support_level": "native",
                    "representation": "custom-agent-toml",
                },
                "skills": {
                    "support_level": "unsupported",
                    "representation": "none",
                },
            },
            "paths": {
                "agents": [
                    {
                        "scope": "project",
                        "path": ".codex-experimental/agents/trackfw-{{id}}.toml",
                        "extension": ".toml",
                    }
                ],
                "skills": [],
            },
        })
        break
else:
    raise SystemExit("codex target not found")
path.write_text(json.dumps(catalog, ensure_ascii=False, separators=(",", ":")), encoding="utf-8")
PY

assert_fails_with "identity-parity/catalog-target-missing" \
  "catalog-derived target/surface is not accepted by the Go CLI" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T3B/scripts/check-identity-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 4 — check-validate-parity.sh: npm sem regra wip_has_req → contrato diverge
#
# Usa npm/$T4 e pypi/$T4; Go é o binário real (GO_BIN explícito, absoluto).
# Remover a chamada applyRule('wip_has_req'…) do npm faz Go e Python reportarem
# wip_has_req mas npm não → "validate JSON contract differs between runtimes".
#
# GO_BIN explícito e absoluto (ROADMAP-2026-08-20-gates-para-os-tres-contratos-
# de-maior-risco, ML-2A): check-validate-parity.sh ganhou suporte a GO_BIN neste
# ML (antes sempre auto-compilava, ignorando a variável) — sem override aqui, a
# cópia herdaria o GO_BIN="bin/trackfw" (relativo) que `make parity` exporta só
# para a linha do check-gates-falsify.sh no Makefile, resolvido contra o ROOT_DIR
# ERRADO ($T4, não o repo real) por check-validate-parity.sh, mesma convenção já
# usada pelos Cenários 42/78 ao copiar outros scripts GO_BIN-aware.
# ---------------------------------------------------------------------------
T4="$WORK/s4"
mkdir -p "$T4/scripts"
setup_npm_tree "$T4"
ln -s "$ROOT_DIR/pypi" "$T4/pypi"
cp "$ROOT_DIR/scripts/check-validate-parity.sh" "$T4/scripts/"
# Corromper: remover applyRule de wip_has_req do validator npm
sed "s/applyRule('wip_has_req'.*$/\/\/ [falsified] wip_has_req removed/" \
  "$ROOT_DIR/npm/src/validator/index.js" > "$T4/npm/src/validator/index.js"

assert_fails_with "validate-parity/rule-removed" \
  "validate JSON contract differs between runtimes" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" \
  bash "$T4/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 5 — check-cli-parity.sh: npm sem comando 'note' → missing command
#
# O gate deriva os comandos do Go CLI e verifica se npm e Python os têm.
# Remover program.addCommand(require('./note')) do npm faz check_help falhar
# antes de check-integration-cli-parity.sh ser invocado.
# ---------------------------------------------------------------------------
T5="$WORK/s5"
mkdir -p "$T5/scripts" "$T5/bin"
setup_npm_tree "$T5"
ln -s "$ROOT_DIR/pypi" "$T5/pypi"
# Scripts necessários (check-cli-parity.sh chama check-integration-cli-parity.sh)
cp "$ROOT_DIR/scripts/check-cli-parity.sh" "$T5/scripts/"
ln -s "$ROOT_DIR/scripts/check-integration-cli-parity.sh" "$T5/scripts/check-integration-cli-parity.sh"
ln -s "$ROOT_DIR/internal" "$T5/internal"
# Corromper: remover registro do comando 'note' do npm
grep -v "require('./note')" "$ROOT_DIR/npm/src/commands/index.js" \
  > "$T5/npm/src/commands/index.js"

assert_fails_with "cli-parity/missing-command" \
  "node: missing command 'note'" \
  bash "$T5/scripts/check-cli-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 6 — check-integration-cli-parity.sh: npm sem comando 'agents' →
#              root help missing agents
#
# O gate roda assert_help_contract por runtime em ordem go→node→python.
# Go passa; ao chegar em node, grep falha com "node: root help missing agents".
# GO_BIN pré-compilado é passado explicitamente para evitar rebuild.
# ---------------------------------------------------------------------------
T6="$WORK/s6"
mkdir -p "$T6/scripts" "$T6/bin"
setup_npm_tree "$T6"
ln -s "$ROOT_DIR/pypi" "$T6/pypi"
ln -s "$ROOT_DIR/internal" "$T6/internal"
cp "$ROOT_DIR/scripts/check-integration-cli-parity.sh" "$T6/scripts/"
# Corromper: remover registro do comando 'agents' do npm
grep -v "require('./agents')" "$ROOT_DIR/npm/src/commands/index.js" \
  > "$T6/npm/src/commands/index.js"

assert_fails_with "integration-cli-parity/missing-agents" \
  "node: root help missing agents" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T6/scripts/check-integration-cli-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 7 — check-artifact-parity.sh: drift de conteúdo em req do npm →
#              gate detecta divergência byte-a-byte (go vs node)
#
# Objetivo (P4): provar que o gate REPROVA quando um template gera conteúdo
# diferente do esperado — sem isso, um gate que nunca falha não é um gate,
# é um ritual.
#
# Estratégia: copiar npm/src via setup_npm_tree, corromper req.js para emitir
# "status: OPEN" em vez de "status: Open" no frontmatter do artefato.
# Go gerará "status: Open"; Node gerará "status: OPEN" → diff detecta → exit 1.
#
# Guard de corrupção: cmp -s confirma que o sed realmente alterou o arquivo;
# se não alterar (padrão não encontrado), a prova P4 seria inválida — o gate
# passaria e assert_fails_with reportaria "saiu com 0, esperava != 0", o que
# confundiria diagnóstico do gate com falha na montagem do cenário.
# ---------------------------------------------------------------------------
T7="$WORK/s7"
mkdir -p "$T7/scripts"
setup_npm_tree "$T7"
ln -s "$ROOT_DIR/pypi" "$T7/pypi"
cp "$ROOT_DIR/scripts/check-artifact-parity.sh" "$T7/scripts/"

# Corromper: trocar "status: Open" por "status: OPEN" no gerador de req do npm.
sed "s/status: Open/status: OPEN/" \
  "$ROOT_DIR/npm/src/generators/req.js" > "$T7/npm/src/generators/req.js"

# Guard: garantir que a corrupção foi aplicada antes de rodar o gate.
if cmp -s "$ROOT_DIR/npm/src/generators/req.js" "$T7/npm/src/generators/req.js"; then
  echo "FAIL [falsify/setup-s7]: sed não alterou req.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "artifact-parity/req-content-drift" \
  "artifact parity drift: req (go vs node)" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T7/scripts/check-artifact-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 8 — check-artifact-parity.sh: drift de NOME de arquivo em req do Go →
#              gate detecta ausência de arquivo com nome esperado (vacuity guard)
#
# Objetivo (P4): provar que o gate REPROVA quando o nome do arquivo gerado
# diverge entre runtimes — o caminho de comparação de nome é independente
# do caminho de comparação de conteúdo e exige prova separada.
#
# Estratégia: compilar um binário Go isolado que use o prefixo "RREQ-" em vez
# de "REQ-" no gerador de req. O gate espera "REQ-<data>-<slug>.md"; o binário
# gera "RREQ-<data>-<slug>.md" → vacuity guard falha com "arquivo ausente",
# diagnóstico distinto do drift de conteúdo (Cenário 7).
#
# O binário isolado é compilado num GOPATH temporário para não contaminar
# o working tree do projeto.
# ---------------------------------------------------------------------------
T8="$WORK/s8"
mkdir -p "$T8/scripts"
ln -s "$ROOT_DIR/pypi" "$T8/pypi"
setup_npm_tree "$T8"
cp "$ROOT_DIR/scripts/check-artifact-parity.sh" "$T8/scripts/"

# Criar cópia isolada do módulo Go com o gerador de req corrompido
T8_MOD="$WORK/s8-mod"
cp -r "$ROOT_DIR/." "$T8_MOD"

# Corromper: trocar "REQ-" por "RREQ-" no nome do arquivo gerado (req.go).
# O padrão que ocorre no arquivo é: /REQ-%s-%s.md
sed 's|/REQ-%s-%s\.md|/RREQ-%s-%s.md|' \
  "$ROOT_DIR/internal/generators/req.go" > "$T8_MOD/internal/generators/req.go"

# Guard: garantir que a corrupção foi aplicada.
if cmp -s "$ROOT_DIR/internal/generators/req.go" "$T8_MOD/internal/generators/req.go"; then
  echo "FAIL [falsify/setup-s8]: sed não alterou req.go — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

# Compilar binário corrompido
T8_BIN="$WORK/s8-bin/trackfw"
mkdir -p "$(dirname "$T8_BIN")"
build_go_or_fail "setup-s8-build" "$T8_MOD" "$T8_BIN"

assert_fails_with "artifact-parity/req-name-drift" \
  "arquivo ausente" \
  env GO_BIN="$T8_BIN" bash "$T8/scripts/check-artifact-parity.sh"

# ---------------------------------------------------------------------------
# ---------------------------------------------------------------------------
# Cenário 9 — check-artifact-parity.sh: drift de conteúdo no slash-command
#              roadmap do npm → gate detecta divergência byte-a-byte.
#
# Objetivo (P4): provar que a comparação de artefatos também cobre o
# slash-command `/trackfw:roadmap`, não apenas os artefatos criados por
# comandos como `req new` e `roadmap new`.
# ---------------------------------------------------------------------------
T9="$WORK/s9"
mkdir -p "$T9/scripts"
setup_npm_tree "$T9"
ln -s "$ROOT_DIR/pypi" "$T9/pypi"
cp "$ROOT_DIR/scripts/check-artifact-parity.sh" "$T9/scripts/"

# Corromper: trocar o status canônico do slash-command no gerador de init npm.
sed "s/status: backlog/status: backlogged/" \
  "$ROOT_DIR/npm/src/generators/init.js" > "$T9/npm/src/generators/init.js"

# Guard: garantir que a corrupção foi aplicada antes de rodar o gate.
if cmp -s "$ROOT_DIR/npm/src/generators/init.js" "$T9/npm/src/generators/init.js"; then
  echo "FAIL [falsify/setup-s9]: sed não alterou init.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "artifact-parity/slash-roadmap-content-drift" \
  "artifact parity drift: slash_roadmap (go vs node)" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T9/scripts/check-artifact-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 10 — check-cli-parity.sh: Python sem --from-req em roadmap new →
#               gate detecta drift de flags do subcomando.
#
# Objetivo (P4): provar que o gate de CLI não verifica só comandos de topo;
# ele também reprova se uma flag pública obrigatória de `roadmap new` sumir
# em qualquer runtime.
# ---------------------------------------------------------------------------
T10="$WORK/s10"
mkdir -p "$T10/scripts" "$T10/bin"
setup_npm_tree "$T10"
cp -r "$ROOT_DIR/pypi" "$T10/pypi"
ln -s "$ROOT_DIR/internal" "$T10/internal"
cp "$ROOT_DIR/scripts/check-cli-parity.sh" "$T10/scripts/"
ln -s "$ROOT_DIR/scripts/check-integration-cli-parity.sh" "$T10/scripts/check-integration-cli-parity.sh"

# Corromper: remover apenas o registro de --from-req do argparse Python.
python3 - "$ROOT_DIR/pypi/trackfw/commands/roadmap.py" "$T10/pypi/trackfw/commands/roadmap.py" <<'PY'
import pathlib
import sys

source = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
old = '''    new_p.add_argument(
        "--from-req",
        default=None,
        help="Generate roadmap with ML stubs from REQ acceptance criteria",
    )
'''
if old not in source:
    raise SystemExit("pattern not found")
pathlib.Path(sys.argv[2]).write_text(source.replace(old, ""), encoding="utf-8")
PY

assert_fails_with "cli-parity/roadmap-new-flag-drift" \
  "python: roadmap new help missing --from-req" \
  bash "$T10/scripts/check-cli-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 11 — check-artifact-parity.sh: log by_agent sem prefixo de agente →
#               gate detecta drift na trilha de transição.
#
# Objetivo (P4): provar que o ciclo E2E do gate verifica a atribuição de agente
# no `.trackfw-log`, não apenas a movimentação física do arquivo.
# ---------------------------------------------------------------------------
T11="$WORK/s11"
mkdir -p "$T11/scripts"
setup_npm_tree "$T11"
ln -s "$ROOT_DIR/pypi" "$T11/pypi"
cp "$ROOT_DIR/scripts/check-artifact-parity.sh" "$T11/scripts/"

# Corromper: remover prefixo agent/ do log by_agent no runtime Node.
python3 - "$ROOT_DIR/npm/src/generators/roadmap.js" "$T11/npm/src/generators/roadmap.js" <<'PY'
import pathlib
import sys

source = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
old = "    logBasename = agent + '/' + basename\n"
new = "    logBasename = basename\n"
if old not in source:
    raise SystemExit("pattern not found")
pathlib.Path(sys.argv[2]).write_text(source.replace(old, new), encoding="utf-8")
PY

assert_fails_with "artifact-parity/by-agent-log-drift" \
  ".trackfw-log não registrou backlog → analyzing" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T11/scripts/check-artifact-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 12 — check-referential-integrity.sh: REQ com roadmap quebrado →
#              gate detecta referência inexistente no frontmatter.
#
# Objetivo (P4): provar que o gate de integridade referencial reprova uma
# referência canônica quebrada sem deixar resíduo no workspace real.
# ---------------------------------------------------------------------------
T12="$WORK/s12"
mkdir -p "$T12/scripts" "$T12/docs"
cp "$ROOT_DIR/scripts/check-referential-integrity.sh" "$T12/scripts/"
cp -r "$ROOT_DIR/docs/req" "$T12/docs/req"
cp -r "$ROOT_DIR/docs/roadmaps" "$T12/docs/roadmaps"
cp -r "$ROOT_DIR/docs/adr" "$T12/docs/adr"

# Corromper: quebrar uma referência existente em cópia temporária.
cat > "$T12/docs/req/REQ-adr-wizard-e-list-2026-06-11.md" <<'EOF'
---
status: Done
adr: ""
roadmap: "docs/roadmaps/done/MISSING-roadmap-adr-wizard-e-list-2026-06-11.md"
---

# REQ quebrada para prova P4
EOF

assert_fails_with "referential-integrity/missing-roadmap" \
  "referential integrity failed" \
  bash "$T12/scripts/check-referential-integrity.sh"

# ---------------------------------------------------------------------------
# Cenário 13 — check-barrier.sh: a própria prova E2E da barrier é falsificável.
#
# Objetivo (P4): check-barrier.sh (ML-4A) não implementa `trackfw barrier` — ele
# delega aos três runtimes. Falsificar seu conteúdo não é corromper a
# implementação (isso é escopo do ML-2A/2B/2C), mas provar que a asserção do
# próprio harness ("Wave 2 continua bloqueada antes da correção") tem poder de
# reprovação. BARRIER_SELFTEST_BREAK=1 é um seam dedicado (documentado no
# cabeçalho de check-barrier.sh) que corrompe deliberadamente a fixture da
# Wave 2 do cenário 1 para já vir ✅ — reproduzindo a classe exata de defeito
# que a checagem `mls_complete` deveria capturar — e o script deve reportar
# essa reprovação com diagnóstico explícito em vez de sair verde.
# ---------------------------------------------------------------------------
assert_fails_with "barrier/blocked-not-detected" \
  "FAIL [barrier/two-wave-flow/wave2-blocked]: expected exit 1 for Wave 2, got 0" \
  env BARRIER_SELFTEST_BREAK=1 GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenário 14 — check-slash-parity.sh: drift de conteúdo em status.md do npm →
#              gate detecta divergência byte-a-byte entre runtimes, nomeando
#              o arquivo específico.
#
# Objetivo (P4, ML-5D): provar que check-slash-parity.sh REPROVA quando um
# comando slash diverge em conteúdo entre runtimes, e que o diagnóstico nomeia
# o arquivo e o par de runtimes divergentes — não apenas "algo diverge".
#
# Nota: HEAD já tem drift pré-existente conhecido em move.md e architect.md
# (ver vault/notes/, reportado fora do escopo do ML-5D). Por isso o padrão
# de falsificação abaixo usa status.md — um arquivo hoje idêntico nos três
# runtimes — para que a reprovação observada seja inequivocamente a
# corrupção deste cenário, não o ruído pré-existente.
# ---------------------------------------------------------------------------
T14="$WORK/s14"
mkdir -p "$T14/scripts"
setup_npm_tree "$T14"
ln -s "$ROOT_DIR/pypi" "$T14/pypi"
cp "$ROOT_DIR/scripts/check-slash-parity.sh" "$T14/scripts/"

# Corromper: alterar o texto do comando executado por status.md no gerador npm.
# O literal na fonte é um template string com backticks escapados
# (Execute o seguinte comando bash: \`trackfw status\`); o padrão do sed
# precisa incluir as barras invertidas para casar com o texto real.
sed 's/Execute o seguinte comando bash: \\`trackfw status\\`/Execute o seguinte comando bash: \\`trackfw statuz\\`/' \
  "$ROOT_DIR/npm/src/generators/init.js" > "$T14/npm/src/generators/init.js"

# Guard: garantir que a corrupção foi aplicada antes de rodar o gate.
if cmp -s "$ROOT_DIR/npm/src/generators/init.js" "$T14/npm/src/generators/init.js"; then
  echo "FAIL [falsify/setup-s14]: sed não alterou init.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "slash-parity/status-content-drift" \
  "slash parity drift: status.md (go vs node)" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T14/scripts/check-slash-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 15 — check-slash-parity.sh: comando removido/renomeado do npm →
#              gate detecta drift de NOME (vacuity guard), independente do
#              caminho de comparação de conteúdo (Cenário 14).
#
# Objetivo (P4, ML-5D): provar que a prova de não-vacuidade do gate cobre os
# dois critérios de aceite separadamente — nome do conjunto de comandos E
# conteúdo — e não apenas o conteúdo (Cenário 14 já cobre esse). Renomear a
# chave 'status.md' para 'status-renamed.md' no mapa CLAUDE_COMMANDS do npm
# faz o Node.js instalar 9 arquivos (contagem correta) mas sem 'status.md'
# — o vacuity guard por-nome-de-arquivo deve reprovar antes de qualquer diff
# de conteúdo ser calculado, com diagnóstico distinto do Cenário 14.
# ---------------------------------------------------------------------------
T15="$WORK/s15"
mkdir -p "$T15/scripts"
setup_npm_tree "$T15"
ln -s "$ROOT_DIR/pypi" "$T15/pypi"
cp "$ROOT_DIR/scripts/check-slash-parity.sh" "$T15/scripts/"

sed "s/'status.md': \`Execute/'status-renamed.md': \`Execute/" \
  "$ROOT_DIR/npm/src/generators/init.js" > "$T15/npm/src/generators/init.js"

# Guard: garantir que a corrupção foi aplicada antes de rodar o gate.
if cmp -s "$ROOT_DIR/npm/src/generators/init.js" "$T15/npm/src/generators/init.js"; then
  echo "FAIL [falsify/setup-s15]: sed não alterou init.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "slash-parity/status-name-drift" \
  "slash parity drift: status.md missing (node) — vacuity guard failed" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T15/scripts/check-slash-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 16 — check-rules-parity.sh: drift de conteúdo no bloco de regras do
#              npm (omitindo o estado `analyzing`) → gate detecta divergência
#              byte-a-byte entre runtimes num dos 4 arquivos auxiliares.
#
# Objetivo (ML-5G): provar que check-rules-parity.sh REPROVA quando o texto
# do bloco de regras (trackfwRulesBlock/_trackfw_rules_block) diverge entre
# runtimes — o próprio defeito que motivou este gate (Go omitia `analyzing`
# e o item de ciclo de vida de ML antes desta ML). Corrompe a linha de
# estados no gerador npm; como os 4 arquivos auxiliares recebem o mesmo
# bloco, qualquer um deles evidencia a reprovação.
# ---------------------------------------------------------------------------
T16="$WORK/s16"
mkdir -p "$T16/scripts"
setup_npm_tree "$T16"
ln -s "$ROOT_DIR/pypi" "$T16/pypi"
cp "$ROOT_DIR/scripts/check-rules-parity.sh" "$T16/scripts/"

# Corromper: remover o estado `analyzing` da chain de estados injetada pelo
# bloco de regras do npm.
sed "s/backlog \/ analyzing \/ wip \/ blocked \/ done \/ abandoned/backlog \/ wip \/ blocked \/ done \/ abandoned/" \
  "$ROOT_DIR/npm/src/generators/init.js" > "$T16/npm/src/generators/init.js"

# Guard: garantir que a corrupção foi aplicada antes de rodar o gate.
if cmp -s "$ROOT_DIR/npm/src/generators/init.js" "$T16/npm/src/generators/init.js"; then
  echo "FAIL [falsify/setup-s16]: sed não alterou init.js — padrão não encontrado; prova de falsificação inválida" >&2
  exit 1
fi

assert_fails_with "rules-parity/content-drift" \
  "rules parity drift: GEMINI.md differs between go and node" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T16/scripts/check-rules-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 17 — check-update-parity.sh: `update harness --dry-run` do Node.js
#              deixa de honrar o guard de dry-run em um alvo → o gate detecta
#              a escrita real no disco que --dry-run deveria suprimir.
#
# Objetivo (ML-6G): provar que a asserção "zero escritas sob --dry-run" do
# novo gate tem poder de reprovação, não apenas de leitura de JSON. O
# fixture do próprio check-update-parity.sh (cenário 4) já semeia um
# claude-skill "stale" (precisa de rewrite) especificamente para que este
# guard tenha algo real a suprimir — sem isso a prova seria vácua (o guard
# passaria mesmo com a corrupção, porque não haveria escrita pendente para
# revelar a ausência do early-return).
#
# Corrompe `claudeSkillTarget` em npm/src/commands/update-harness.js,
# removendo o único `if (dryRun) return ...` que impede a escrita real do
# arquivo de skill legado durante --dry-run.
# ---------------------------------------------------------------------------
T17="$WORK/s17"
mkdir -p "$T17/scripts"
setup_npm_tree "$T17"
ln -s "$ROOT_DIR/pypi" "$T17/pypi"
cp "$ROOT_DIR/scripts/check-update-parity.sh" "$T17/scripts/"

sed "s/    if (dryRun) return { id, state: 'updated', path: displayPath }/    \/\/ [falsified] dry-run guard removed — write proceeds unconditionally/" \
  "$ROOT_DIR/npm/src/commands/update-harness.js" > "$T17/npm/src/commands/update-harness.js"

# Guard: garantir que a corrupção foi aplicada antes de rodar o gate.
if cmp -s "$ROOT_DIR/npm/src/commands/update-harness.js" "$T17/npm/src/commands/update-harness.js"; then
  echo "FAIL [falsify/setup-s17]: sed não alterou update-harness.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "update-parity/dry-run-write-leak" \
  "filesystem tree under HOME changed during --dry-run" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T17/scripts/check-update-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 18 — não-mutação: os gates que invocam CLIs reais (agents install,
# init, update, barrier) não alteram a árvore de trabalho do repositório
# quando rodados a partir da raiz — exatamente como `make quality`/`make
# parity` já fazem.
#
# Objetivo (ML-6I): o bug corrigido em install_claude_agents() de
# check-update-parity.sh (ver
# vault/notes/update-parity-gate-writes-real-claude-md-2026-07-29.md) fazia
# o gate passar — `exit 0`, todas as scenarios "OK" — enquanto injetava o
# bloco trackfw:rules no CLAUDE.md do próprio repositório. "Gate verde" não
# provava "repositório intocado"; esta prova fecha esse buraco de forma
# automática em vez de depender de um agente lembrar de rodar `git status`
# manualmente. Captura `git status --porcelain` antes/depois de rodar,
# a partir de ROOT_DIR, os gates que exercitam CLIs reais (não os que operam
# só sobre cópias isoladas em $WORK) e reprova se houver qualquer diferença.
# ---------------------------------------------------------------------------
GATES_MUTATION_CHECK=(
  scripts/check-update-parity.sh
  scripts/check-barrier.sh
  scripts/check-slash-parity.sh
  scripts/check-rules-parity.sh
  scripts/check-roadmap-move-parity.sh
)

before_status=$(cd "$ROOT_DIR" && git status --porcelain)
for gate in "${GATES_MUTATION_CHECK[@]}"; do
  if ! (cd "$ROOT_DIR" && GO_BIN="$ROOT_DIR/bin/trackfw" bash "$gate") >"$WORK/mutation-check.$(basename "$gate").log" 2>&1; then
    echo "FAIL [falsify/no-repo-mutation]: $gate saiu != 0 rodando limpo (não corrompido) — não é possível provar não-mutação" >&2
    sed 's/^/    /' "$WORK/mutation-check.$(basename "$gate").log" >&2
    exit 1
  fi
done
after_status=$(cd "$ROOT_DIR" && git status --porcelain)

if [[ "$before_status" != "$after_status" ]]; then
  echo "FAIL [falsify/no-repo-mutation]: rodar os gates a partir da raiz alterou a árvore de trabalho do repositório" >&2
  diff <(echo "$before_status") <(echo "$after_status") >&2 || true
  exit 1
fi
echo "OK   [falsify/no-repo-mutation]"

# ---------------------------------------------------------------------------
# Cenário 19 — check-barrier.sh: o gate de heading-malformada-after-target
# (Cenário 9) é falsificável com respeito à classe de bug early-break.
#
# Objetivo (ML-3A, ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis):
# O Cenário 9 de check-barrier.sh cobre a posição "depois da wave alvo" —
# a posição crítica que uma implementação com early-break NÃO detecta.
# Sem esta prova, o cenário seria vacuoso: mesmo que todos os runtimes
# tivessem o bug de early-break (voltando exit 0), o cenário passaria
# verde (cli-parity.md §detection-is-a-full-pre-pass, regra "both positions").
#
# BARRIER_BIS_SELFTEST_BREAK=1 ativa o seam dedicado em check-barrier.sh:
# o Cenário 9 escreve uma fixture válida (sem o heading malformado), fazendo
# todos os runtimes retornar exit 0. A asserção espera exit 2 → falha com o
# diagnóstico explícito abaixo — provando que o cenário tem poder de reprovação
# sobre a classe de defeito de early-break.
#
# Nota: o seam corrompe a FIXTURE, nunca a asserção (mesmo padrão que
# BARRIER_SELFTEST_BREAK do Cenário 13) — não é uma mudança tautológica.
# ---------------------------------------------------------------------------
assert_fails_with "barrier/early-break-after-target-not-detected" \
  'FAIL [barrier/wave-label/malformed-after-target/go]: expected exit 2 for after-position malformed heading, got 0' \
  env BARRIER_BIS_SELFTEST_BREAK=1 GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenário 20 — check-roadmap-move-parity.sh: ordenação por caminho completo no
#              Node.js em vez de basename → gate detecta divergência na fixture
#              discriminante (apolo/REQ-zzz + zeus/REQ-aaa → aaa, zzz esperado).
#
# Objetivo (ML-3A, ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada):
# A fixture discriminante (Cenário 3) inverte os basenames entre os agentes —
# `apolo/done/REQ-zzz.md` e `zeus/backlog/REQ-aaa.md` — de modo que ordenação
# por caminho completo (apolo < zeus) produz `zzz, aaa` (ERRADO) enquanto
# ordenação por basename produz `aaa, zzz` (CORRETO). Uma implementação que usa
# o caminho completo como chave de sort concorda com a fixture COINCIDENTE
# (`apolo/REQ-aaa` + `zeus/REQ-zzz`) mas diverge aqui — e sem esta prova o
# cenário 3 seria vacuoso com respeito a essa classe de regressão.
#
# Seam: sed troca `path.basename(a)` → `a` e `path.basename(b)` → `b` no
# comparador de `syncReqReferences` em npm/src/generators/roadmap.js.
# Corrompe a IMPLEMENTAÇÃO (fixture do gate), nunca a asserção do gate —
# mesmo padrão dos Cenários 14/16/17.
# ---------------------------------------------------------------------------
T20="$WORK/s20"
mkdir -p "$T20/scripts"
setup_npm_tree "$T20"
ln -s "$ROOT_DIR/pypi" "$T20/pypi"
cp "$ROOT_DIR/scripts/check-roadmap-move-parity.sh" "$T20/scripts/"

# Corromper: sort por caminho completo em vez de basename
sed -e 's/const ba = path\.basename(a)/const ba = a/' \
    -e 's/const bb = path\.basename(b)/const bb = b/' \
    "$ROOT_DIR/npm/src/generators/roadmap.js" > "$T20/npm/src/generators/roadmap.js"

# Guard: garantir que a corrupção foi aplicada
if cmp -s "$ROOT_DIR/npm/src/generators/roadmap.js" "$T20/npm/src/generators/roadmap.js"; then
  echo "FAIL [falsify/setup-s20]: sed não alterou roadmap.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "roadmap-move-parity/discriminant-wrong-order-not-detected" \
  "roadmap-move-parity/by_agent-discriminant/node" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T20/scripts/check-roadmap-move-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 21 — check-cli-parity.sh: Node.js version subcommand reintroduz o
#              prefixo `v` → gate detecta formato inválido (prova do braço
#              de asserção de formato — regex arm).
#
# Objetivo (ML-3A, ROADMAP-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis):
# A asserção unificada ('^trackfw [0-9]+\.[0-9]+\.[0-9]+$') deve reprovar quando
# um runtime imprime 'trackfw v5.0.0' em vez de 'trackfw 5.0.0'. A asserção
# anterior ('^trackfw .+') aceitava os dois formatos, tornando o gate vacuoso
# com respeito ao prefixo `v`. Corrupção na implementação, nunca na asserção.
# ---------------------------------------------------------------------------
T21="$WORK/s21"
mkdir -p "$T21/scripts" "$T21/bin"
setup_npm_tree "$T21"
ln -s "$ROOT_DIR/pypi" "$T21/pypi"
ln -s "$ROOT_DIR/internal" "$T21/internal"
cp "$ROOT_DIR/scripts/check-cli-parity.sh" "$T21/scripts/"
ln -s "$ROOT_DIR/scripts/check-integration-cli-parity.sh" "$T21/scripts/check-integration-cli-parity.sh"

# Corromper: reintroduzir o prefixo `v` no subcomando `version` do Node.js.
sed 's/`trackfw ${version}`/`trackfw v${version}`/' \
  "$ROOT_DIR/npm/src/commands/version.js" > "$T21/npm/src/commands/version.js"

# Guard: garantir que a corrupção foi aplicada.
if cmp -s "$ROOT_DIR/npm/src/commands/version.js" "$T21/npm/src/commands/version.js"; then
  echo "FAIL [falsify/setup-s21]: sed não alterou version.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "cli-parity/version-v-prefix" \
  "node version format invalid" \
  bash "$T21/scripts/check-cli-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 22 — check-cli-parity.sh: versão do npm/package.json diverge dos
#              demais runtimes → gate detecta mismatch byte-a-byte (prova do
#              braço de comparação — byte-comparison arm).
#
# Objetivo (ML-3A): a comparação byte-a-byte não pode ser provada pelo Cenário 21
# (que falha antes, no braço de formato). Este cenário corrompe package.json para
# 9.9.9 — Node imprime 'trackfw 9.9.9', Go e Python continuam em 5.0.0.
# Formato sintaticamente correto para todos os seis; apenas a comparação
# byte-a-byte detecta a divergência. Corrupção na implementação, nunca na asserção.
# ---------------------------------------------------------------------------
T22="$WORK/s22"
mkdir -p "$T22/scripts" "$T22/bin"
setup_npm_tree "$T22"
ln -s "$ROOT_DIR/pypi" "$T22/pypi"
ln -s "$ROOT_DIR/internal" "$T22/internal"
cp "$ROOT_DIR/scripts/check-cli-parity.sh" "$T22/scripts/"
ln -s "$ROOT_DIR/scripts/check-integration-cli-parity.sh" "$T22/scripts/check-integration-cli-parity.sh"

# Corromper: substituir a versão do npm/package.json por 9.9.9.
# Node.js lê a versão de package.json (via require('../../package.json')) em
# ambas as superfícies (version subcommand e --version flag); Go e Python
# permanecem em 5.0.0. Formato passa a regex; comparação byte-a-byte reprova.
sed 's/"version": "[^"]*"/"version": "9.9.9"/' \
  "$ROOT_DIR/npm/package.json" > "$T22/npm/package.json"

# Guard: garantir que a corrupção foi aplicada.
if cmp -s "$ROOT_DIR/npm/package.json" "$T22/npm/package.json"; then
  echo "FAIL [falsify/setup-s22]: sed não alterou package.json — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "cli-parity/version-byte-mismatch" \
  "version byte mismatch — go vs node/version" \
  bash "$T22/scripts/check-cli-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 23 — check-cli-parity.sh: Go reintroduz -v como atalho de --version
#              (seam: remoção da pré-declaração de "version" sem shorthand em
#              internal/commands/root.go) → gate detecta que -v exita 0.
#
# Objetivo (ML-3A, ROADMAP-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go):
# Sem `root.Flags().Bool("version", false, "version for trackfw")`, cobra executa
# InitDefaultVersionFlag e registra --version com shorthand v, fazendo `trackfw -v`
# sair com exit 0 e imprimir a versão. O gate do ML-3A detecta isso com o
# diagnóstico "go -v exited 0 — -v must be rejected (non-zero exit)".
#
# Seam: APENAS Go é falsificado — é o único runtime que carregava o defeito
# (cobra InitDefaultVersionFlag). Node.js e Python já rejeitavam -v antes do
# ML-2A; adicionar seams neles estaria fora do escopo negativo do roadmap.
#
# Guarda de padrão (sed): confirma que o sed encontrou e alterou o alvo antes
# de construir o binário — se o padrão mudou de nome, a prova é inválida.
# Guarda de vivacidade: constrói e executa o binário corrompido para confirmar
# que -v é aceito antes de rodar o gate — distingue "seam inativo" de "gate
# não reprova".
# ---------------------------------------------------------------------------
T23="$WORK/s23"
mkdir -p "$T23/scripts"
# Go: cópia real (não symlink) para isolar a corrupção em internal/commands/root.go.
mkdir -p "$T23/cmd" "$T23/internal"
cp -r "$ROOT_DIR/cmd/." "$T23/cmd/"
cp -r "$ROOT_DIR/internal/." "$T23/internal/"
cp "$ROOT_DIR/go.mod" "$T23/go.mod"
cp "$ROOT_DIR/go.sum"  "$T23/go.sum"
# Node.js e Python: symlinks (não modificados — seam é Go-only).
ln -s "$ROOT_DIR/npm"  "$T23/npm"
ln -s "$ROOT_DIR/pypi" "$T23/pypi"
# Scripts: copiar o gate; symlink para check-integration (lido como ROOT_DIR=$T23).
cp "$ROOT_DIR/scripts/check-cli-parity.sh" "$T23/scripts/"
ln -s "$ROOT_DIR/scripts/check-integration-cli-parity.sh" \
      "$T23/scripts/check-integration-cli-parity.sh"

# Corromper: remover a pré-declaração que impede o cobra de registrar -v.
# Sem esta linha, cobra.InitDefaultVersionFlag registra --version com shorthand v.
sed 's/root\.Flags()\.Bool("version", false, "version for trackfw")/\/\/ [falsified] root.Flags().Bool("version", false, "version for trackfw") — removed/' \
  "$ROOT_DIR/internal/commands/root.go" > "$T23/internal/commands/root.go"

# Guarda de padrão: garantir que o sed encontrou e alterou o alvo.
if cmp -s "$ROOT_DIR/internal/commands/root.go" "$T23/internal/commands/root.go"; then
  echo "FAIL [falsify/setup-s23]: sed não alterou root.go — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

# Guarda de vivacidade: compilar e exercitar o binário corrompido antes de rodar o gate.
# Separa "seam inativo" (buildou mas -v continua rejeitado) de "gate não reprova".
T23_BIN="$WORK/s23-bin/trackfw"
mkdir -p "$(dirname "$T23_BIN")"
build_go_or_fail "setup-s23-liveness-build" "$T23" "$T23_BIN"

set +e
_S23_V_OUT=$("$T23_BIN" -v 2>&1)
_S23_V_EXIT=$?
set -e

if [[ $_S23_V_EXIT -ne 0 ]]; then
  echo "FAIL [falsify/setup-s23-liveness]: seam inativo — binário corrompido ainda rejeita -v (exit $_S23_V_EXIT; got: '$_S23_V_OUT')" >&2
  exit 1
fi
if ! grep -Eq '^trackfw [0-9]+\.[0-9]+\.[0-9]+$' <<<"$_S23_V_OUT"; then
  echo "FAIL [falsify/setup-s23-liveness]: seam ativo mas -v não imprimiu versão no formato esperado (exit $_S23_V_EXIT; got: '$_S23_V_OUT')" >&2
  exit 1
fi

# Rodar o gate a partir do módulo corrompido: `cd T23` faz `go build ./cmd/trackfw`
# compilar a partir do internal/ corrompido (cobra RegisterVersion com shorthand v).
assert_fails_with "cli-parity/v-flag-accepted" \
  "go -v exited 0 — -v must be rejected" \
  bash -c 'cd "$1" && bash scripts/check-cli-parity.sh' _ "$T23"

# ---------------------------------------------------------------------------
# Cenário 24 — ciclo `roadmap new` → `roadmap move ... wip` → `validate`:
#              gerador de roadmap sem o heading `## Acceptance Criteria` faz
#              o roadmap movido para wip reprovar em `validate` com o
#              diagnóstico `wip_acceptance`
#              (ROADMAP-2026-07-31-alinhar-marcador-de-criterios-de-aceite-do-gerador-de-roadmap).
#
# Objetivo (ML-2A): nenhum gate de paridade existente detecta a remoção
# COORDENADA do heading nos três geradores — check-artifact-parity.sh só
# compara os runtimes ENTRE si (byte-a-byte), nunca contra o contrato do
# validador. Sem esta prova, os três geradores poderiam voltar a perder o
# heading simultaneamente (o defeito original, contornado manualmente em
# três ciclos consecutivos — ver Wave 1 do roadmap) e `make quality`
# continuaria verde. Reproduz o ciclo real (`init` → `roadmap new` →
# `roadmap move ... wip` → `validate`) num sandbox isolado por runtime, e
# exige que `validate` reprove com o diagnóstico exato do validador — texto
# idêntico nos três (internal/validator/validator.go:989,
# npm/src/validator/index.js:415, pypi/trackfw/validator.py:669).
#
# Cobre os DOIS caminhos de geração por CLI (AC3 da REQ: "vale também para
# roadmap new --from-req") — não apenas o template simples. O heading ocorre
# 2x, byte-idêntico, em cada gerador (simples e --from-req); sem cobrir os
# dois, alguém poderia remover só o bloco do --from-req nos três CLIs e este
# cenário continuaria verde.
#
# Nota sobre o caminho --from-req (HISTÓRICA — obsoleta a partir da Wave 1 do
# ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req):
# até essa Wave 1, o ciclo com REQ NUNCA reprovava "limpo" — NewRoadmapFromREQ
# gravava `req: "<basename>"` no frontmatter e `ref_targets_exist` sempre
# reprovava essa referência co-ocorrendo com wip_acceptance. Isso NÃO
# invalidava a prova daquele momento: com o gerador correto (pré-Wave 1) a
# violação de wip_acceptance estava ausente da saída (só aparecia a de
# ref_targets_exist); com o gerador corrompido as duas apareciam juntas. O
# padrão buscado por assert_fails_with é o diagnóstico específico de
# wip_acceptance, não a ausência de outras violações — a prova de vivacidade
# abaixo confirmava isso empiricamente.
#
# A partir da Wave 1, o `req:` do frontmatter passou a gravar o caminho
# relativo completo (não mais o basename), então o ciclo `--from-req` AGORA
# reprova limpo sem `ref_targets_exist` co-ocorrente — o Cenário 25 (braço
# de linha de base `*/from-req-baseline`, via assert_lacks_pattern) prova
# isso diretamente. Este parágrafo permanece para explicar por que o
# Cenário 24 nunca precisou de um braço de linha de base equivalente: quando
# foi escrito, o ciclo `--from-req` nunca vinha "limpo" e a ausência de
# `wip_acceptance` já bastava como sinal.
#
# Corrompe a IMPLEMENTAÇÃO (gerador), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21. Cobre os três CLIs: cada runtime tem seu próprio
# gerador disjunto e portanto sua própria prova de vivacidade.
# ---------------------------------------------------------------------------

# Fixture de REQ válida (ADR/Roadmap preenchidos) reaproveitada nos sandboxes
# — evita disparar wip_has_req/req_has_adr/req_has_roadmap, que reprovariam
# por motivo diferente do heading e confundiriam o diagnóstico.
write_roadmap_acceptance_req_fixture() {
  local dest=$1
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<'REQEOF'
---
status: Open
date: 2026-08-01
adr: ""
roadmap: ""
---

# REQ: Flag Source

## Acceptance Criteria
- [ ] Something

## Linked ADR
ADR: none

## Linked Roadmap
Roadmap: none
REQEOF
}

# Remove a n-ésima ocorrência (0-based) do bloco de heading consolidado.
# O bloco é byte-idêntico nas 2 ocorrências (template simples e --from-req)
# em Go/Node/Python — só o texto QUE SEGUE difere — então localizamos por
# índice de ocorrência em vez de âncora de sufixo (frágil e específica por
# linguagem).
remove_roadmap_acceptance_heading() {
  local src_file=$1
  local dest_file=$2
  local occurrence=$3   # 0 = template simples, 1 = --from-req
  local label=$4
  python3 - "$src_file" "$dest_file" "$occurrence" <<'PY'
import pathlib
import sys

src_path, dest_path, occurrence = sys.argv[1], sys.argv[2], int(sys.argv[3])
source = pathlib.Path(src_path).read_text(encoding="utf-8")
block = ("## Acceptance Criteria\n"
         "<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->\n"
         "- [ ]\n- [ ]\n\n")
positions = [i for i in range(len(source)) if source.startswith(block, i)]
if len(positions) != 2:
    raise SystemExit(f"expected 2 occurrences of the heading block, got {len(positions)}")
start = positions[occurrence]
end = start + len(block)
pathlib.Path(dest_path).write_text(source[:start] + source[end:], encoding="utf-8")
PY
  if cmp -s "$src_file" "$dest_file"; then
    echo "FAIL [falsify/setup-s24-$label]: heading não removido — prova P4 inválida" >&2
    exit 1
  fi
}

# Executa o ciclo completo init → roadmap new (simples ou --from-req) →
# roadmap move wip → validate contra um binário/runtime já preparado no
# sandbox $1, e imprime a saída de `validate` (stdout+stderr) preservando o
# exit code — para ser usado dentro de assert_fails_with. $1 é o workdir; o
# restante dos argumentos ("$@" após o shift) é o comando do runtime como
# argv (ex: "$T24G_BIN", ou "node" "npm/bin/trackfw") — sem eval, mesmo
# idioma de invocação direta usado no resto do script (Cenário 23).
ROADMAP_CYCLE_SCRIPT_SIMPLE='
  set -e
  cd "$1"
  shift
  "$@" init >/dev/null
  "$@" roadmap new --title "Falsify Test" --req docs/req/REQ-flag-source.md >/dev/null
  name=$(basename "$(find docs/roadmaps/backlog -name "*.md")")
  "$@" roadmap move "$name" wip >/dev/null
  exec "$@" validate
'
ROADMAP_CYCLE_SCRIPT_FROM_REQ='
  set -e
  cd "$1"
  shift
  "$@" init >/dev/null
  "$@" roadmap new --from-req docs/req/REQ-flag-source.md >/dev/null
  name=$(basename "$(find docs/roadmaps/backlog -name "*.md")")
  "$@" roadmap move "$name" wip >/dev/null
  exec "$@" validate
'

# --- Go -------------------------------------------------------------------
# Cópia enxuta do módulo (cmd/ + internal/ + go.mod/go.sum), não o repo
# inteiro — mesmo padrão do Cenário 23; evita I/O desnecessário e não
# arrasta node_modules/pypi/build para dentro do sandbox de compilação.
for occ_label in "0:simple" "1:from-req"; do
  occ="${occ_label%%:*}"
  path_name="${occ_label##*:}"

  T24G_MOD="$WORK/s24-go-mod-$path_name"
  mkdir -p "$T24G_MOD/cmd" "$T24G_MOD/internal"
  cp -r "$ROOT_DIR/cmd/." "$T24G_MOD/cmd/"
  cp -r "$ROOT_DIR/internal/." "$T24G_MOD/internal/"
  cp "$ROOT_DIR/go.mod" "$T24G_MOD/go.mod"
  cp "$ROOT_DIR/go.sum" "$T24G_MOD/go.sum"
  remove_roadmap_acceptance_heading \
    "$ROOT_DIR/internal/generators/roadmap.go" "$T24G_MOD/internal/generators/roadmap.go" \
    "$occ" "go-$path_name"

  T24G_BIN="$WORK/s24-go-bin-$path_name/trackfw"
  mkdir -p "$(dirname "$T24G_BIN")"
  build_go_or_fail "setup-s24-go-$path_name-build" "$T24G_MOD" "$T24G_BIN"

  T24G="$WORK/s24-go-$path_name"
  mkdir -p "$T24G"
  write_roadmap_acceptance_req_fixture "$T24G/docs/req/REQ-flag-source.md"

  script_var="ROADMAP_CYCLE_SCRIPT_SIMPLE"
  [[ "$path_name" == "from-req" ]] && script_var="ROADMAP_CYCLE_SCRIPT_FROM_REQ"

  assert_fails_with "roadmap-acceptance-heading/go/$path_name" \
    "is in wip but has no acceptance criteria block" \
    bash -c "${!script_var}" _ "$T24G" "$T24G_BIN"
done

# --- Node -------------------------------------------------------------------
for occ_label in "0:simple" "1:from-req"; do
  occ="${occ_label%%:*}"
  path_name="${occ_label##*:}"

  T24N="$WORK/s24-node-$path_name"
  mkdir -p "$T24N"
  setup_npm_tree "$T24N"
  remove_roadmap_acceptance_heading \
    "$ROOT_DIR/npm/src/generators/roadmap.js" "$T24N/npm/src/generators/roadmap.js" \
    "$occ" "node-$path_name"

  write_roadmap_acceptance_req_fixture "$T24N/docs/req/REQ-flag-source.md"

  script_var="ROADMAP_CYCLE_SCRIPT_SIMPLE"
  [[ "$path_name" == "from-req" ]] && script_var="ROADMAP_CYCLE_SCRIPT_FROM_REQ"

  assert_fails_with "roadmap-acceptance-heading/node/$path_name" \
    "is in wip but has no acceptance criteria block" \
    bash -c "${!script_var}" _ "$T24N" node npm/bin/trackfw
done

# --- Python -------------------------------------------------------------------
for occ_label in "0:simple" "1:from-req"; do
  occ="${occ_label%%:*}"
  path_name="${occ_label##*:}"

  T24P="$WORK/s24-python-$path_name"
  mkdir -p "$T24P"
  cp -r "$ROOT_DIR/pypi" "$T24P/pypi"
  remove_roadmap_acceptance_heading \
    "$ROOT_DIR/pypi/trackfw/generators/roadmap.py" "$T24P/pypi/trackfw/generators/roadmap.py" \
    "$occ" "python-$path_name"

  write_roadmap_acceptance_req_fixture "$T24P/docs/req/REQ-flag-source.md"

  script_var="ROADMAP_CYCLE_SCRIPT_SIMPLE"
  [[ "$path_name" == "from-req" ]] && script_var="ROADMAP_CYCLE_SCRIPT_FROM_REQ"

  assert_fails_with "roadmap-acceptance-heading/python/$path_name" \
    "is in wip but has no acceptance criteria block" \
    bash -c "${!script_var}" _ "$T24P" env "PYTHONPATH=$T24P/pypi" python3 -m trackfw
done

# ---------------------------------------------------------------------------
# Helpers reused pelos Cenários 25 e 26 abaixo.
# ---------------------------------------------------------------------------

# Substitui a única ocorrência de `old` por `new` em todo o arquivo. Falha se
# a contagem de ocorrências não for exatamente 1 — evita corromper o alvo
# errado silenciosamente e evita "passar" sem corromper nada.
corrupt_literal() {
  local src=$1 dest=$2 old=$3 new=$4 label=$5
  python3 - "$src" "$dest" "$old" "$new" "$label" <<'PY'
import pathlib
import sys

src, dest, old, new, label = sys.argv[1:6]
source = pathlib.Path(src).read_text(encoding="utf-8")
count = source.count(old)
if count != 1:
    raise SystemExit(f"[{label}] expected exactly 1 occurrence of pattern, got {count}")
pathlib.Path(dest).write_text(source.replace(old, new, 1), encoding="utf-8")
PY
}

# Substitui a primeira ocorrência de `old` por `new`, restrita ao corpo de
# `func_name` (de `def func_name(` até o próximo `\ndef ` ou fim de arquivo).
# Necessário no Python: o literal `req: "{req_path}"` ocorre IDÊNTICO em duas
# funções distintas (_roadmap_template para --req simples,
# generate_roadmap_from_req para --from-req) — sem escopo de função, corromper
# uma corromperia as duas ao mesmo tempo.
corrupt_python_func_literal() {
  local src=$1 dest=$2 func_name=$3 old=$4 new=$5
  python3 - "$src" "$dest" "$func_name" "$old" "$new" <<'PY'
import pathlib
import re
import sys

src, dest, func_name, old, new = sys.argv[1:6]
source = pathlib.Path(src).read_text(encoding="utf-8")
marker = f"def {func_name}("
start = source.index(marker)
tail = source[start + 1:]
next_def = re.search(r"\ndef ", tail)
end = start + 1 + next_def.start() if next_def else len(source)
segment = source[start:end]
if segment.count(old) != 1:
    raise SystemExit(f"[{func_name}] expected exactly 1 occurrence of pattern, got {segment.count(old)}")
new_segment = segment.replace(old, new, 1)
pathlib.Path(dest).write_text(source[:start] + new_segment + source[end:], encoding="utf-8")
PY
}

# Helper: assert que o comando retorna exit 0 E a saída NÃO contém `pattern`.
# Usado para provar que o ciclo LIMPO (código correto, sem corrupção) não
# emite o diagnóstico da corrupção — sem esta prova, o braço de detecção
# (assert_fails_with) sozinho não descarta a hipótese de que o ciclo já
# reprovaria por qualquer outro motivo (seam inativo mascarado por ruído
# alheio à corrupção).
assert_lacks_pattern() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: ciclo limpo saiu com $status, esperava 0" >&2
    echo "  output: $out" >&2
    exit 1
  fi
  if grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label]: seam inativo — o ciclo LIMPO já emite '$pattern'; o cenário de corrupção passaria mesmo sem a corrupção" >&2
    echo "  output: $out" >&2
    exit 1
  fi
  echo "OK   [falsify/$label]"
}

# ---------------------------------------------------------------------------
# Cenário 25 — ciclo `roadmap new --from-req` → `roadmap move ... wip` →
# `validate`: revertendo os 3 geradores para gravar `filepath.Base`/`basename`
# (em vez do caminho relativo completo) no campo `req:` do frontmatter — o
# bug corrigido por
# ADR-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-parametro-roots-morto
# (ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req)
# — o ciclo deve reprovar em `validate` com `ref_targets_exist`.
#
# Objetivo (ML-2A): nenhum gate de paridade existente cobre o CONTRATO
# gerador→validador para o campo `req:` — check-artifact-parity.sh só compara
# os runtimes ENTRE si (byte-a-byte), nunca contra o `os.Stat`/
# `referenceExists` do validador. Sem esta prova, os três geradores poderiam
# voltar a gravar basename simultaneamente (o defeito original deste
# roadmap, "a ferramenta reprova o que ela mesma gerou" pela terceira vez) e
# `make quality` continuaria verde.
#
# Reusa write_roadmap_acceptance_req_fixture e ROADMAP_CYCLE_SCRIPT_FROM_REQ
# (definidos no Cenário 24) — mesma fixture de REQ válida, mesmo idioma de
# ciclo E2E. O diagnóstico esperado ("which does not exist") é a substring
# estática da mensagem de ref_targets_exist nos três runtimes (`roadmap "%s"
# links to REQ "%s" which does not exist` / equivalentes) quando o `req:` do
# frontmatter aponta para um caminho que a validação estrita (sem `roots`,
# conforme o ADR) não resolve — exatamente o que acontece quando o campo
# grava só o basename em vez do caminho relativo completo docs/req/....
#
# Corrompe a IMPLEMENTAÇÃO (gerador), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21/24. Cobre os três CLIs.
# ---------------------------------------------------------------------------

# Diagnóstico estático e discriminante: com a fixture REQ-flag-source.md, o
# ref corrompido é sempre filepath.Base("docs/req/REQ-flag-source.md") =
# "REQ-flag-source.md" — mensagem byte-idêntica nos 3 runtimes
# (validator.go:1463, index.js:758, validator.py:940). Mais específico que
# "which does not exist" isolado, que também casa com as mensagens de
# req→ADR e req→Roadmap ausentes (não aplicáveis aqui, mas indistinguíveis
# por um grep genérico).
S25_PATTERN='links to REQ "REQ-flag-source.md" which does not exist'

# --- Go -----------------------------------------------------------------
# Braço de linha de base (ciclo LIMPO, sem corrupção): prova que o gerador
# correto (pós-Wave 1) não emite mais o diagnóstico — sem isto, o braço de
# detecção abaixo não descartaria "o ciclo já reprovava por outro motivo".
T25G_BASE_BIN="$WORK/s25-go-base-bin/trackfw"
mkdir -p "$(dirname "$T25G_BASE_BIN")"
build_go_or_fail "setup-s25-go-baseline-build" "$ROOT_DIR" "$T25G_BASE_BIN"

T25G_BASE="$WORK/s25-go-base"
mkdir -p "$T25G_BASE"
write_roadmap_acceptance_req_fixture "$T25G_BASE/docs/req/REQ-flag-source.md"

assert_lacks_pattern "roadmap-req-frontmatter-path/go/from-req-baseline" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25G_BASE" "$T25G_BASE_BIN"

# Braço de detecção: gerador revertido para gravar basename.
T25G_MOD="$WORK/s25-go-mod"
mkdir -p "$T25G_MOD/cmd" "$T25G_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T25G_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T25G_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T25G_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T25G_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/roadmap.go" "$T25G_MOD/internal/generators/roadmap.go" \
  'date, reqPath, title, date, filepath.Base(reqPath), reqPath, adrRef, mlSection.String())' \
  'date, filepath.Base(reqPath), title, date, filepath.Base(reqPath), reqPath, adrRef, mlSection.String())' \
  "s25-go"

T25G_BIN="$WORK/s25-go-bin/trackfw"
mkdir -p "$(dirname "$T25G_BIN")"
build_go_or_fail "setup-s25-go-build" "$T25G_MOD" "$T25G_BIN"

T25G="$WORK/s25-go"
mkdir -p "$T25G"
write_roadmap_acceptance_req_fixture "$T25G/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/go/from-req" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25G" "$T25G_BIN"

# --- Node -----------------------------------------------------------------
# Braço de linha de base.
T25N_BASE="$WORK/s25-node-base"
mkdir -p "$T25N_BASE"
setup_npm_tree "$T25N_BASE"
write_roadmap_acceptance_req_fixture "$T25N_BASE/docs/req/REQ-flag-source.md"

assert_lacks_pattern "roadmap-req-frontmatter-path/node/from-req-baseline" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25N_BASE" node npm/bin/trackfw

# Braço de detecção.
T25N="$WORK/s25-node"
mkdir -p "$T25N"
setup_npm_tree "$T25N"
corrupt_literal \
  "$ROOT_DIR/npm/src/generators/roadmap.js" "$T25N/npm/src/generators/roadmap.js" \
  'req: "${reqPath}"' \
  'req: "${basename}"' \
  "s25-node"

write_roadmap_acceptance_req_fixture "$T25N/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/node/from-req" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25N" node npm/bin/trackfw

# --- Python -----------------------------------------------------------------
# Braço de linha de base.
T25P_BASE="$WORK/s25-python-base"
mkdir -p "$T25P_BASE"
cp -r "$ROOT_DIR/pypi" "$T25P_BASE/pypi"
write_roadmap_acceptance_req_fixture "$T25P_BASE/docs/req/REQ-flag-source.md"

assert_lacks_pattern "roadmap-req-frontmatter-path/python/from-req-baseline" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25P_BASE" env "PYTHONPATH=$T25P_BASE/pypi" python3 -m trackfw

# Braço de detecção.
T25P="$WORK/s25-python"
mkdir -p "$T25P"
cp -r "$ROOT_DIR/pypi" "$T25P/pypi"
corrupt_python_func_literal \
  "$ROOT_DIR/pypi/trackfw/generators/roadmap.py" "$T25P/pypi/trackfw/generators/roadmap.py" \
  "generate_roadmap_from_req" \
  'req: "{req_path}"' \
  'req: "{basename}"'

write_roadmap_acceptance_req_fixture "$T25P/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/python/from-req" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25P" env "PYTHONPATH=$T25P/pypi" python3 -m trackfw

# ---------------------------------------------------------------------------
# Cenário 26 — AC2b: o caminho SIMPLES (`roadmap new --title <t> --req
# <path>`) também deve gravar o caminho completo no `req:` do frontmatter.
#
# Diferente do Cenário 25, uma regressão aqui NÃO produz uma violação de
# `validate` — `extractRefPath` tem early-return para valor vazio, então
# `req: ""` é um falso-NEGATIVO silencioso (documentado no roadmap como "bug
# irmão AC2b": este próprio ciclo de trabalho foi gerado com `--req` e saiu
# com `req: ""` antes da Wave 1). `assert_fails_with` não serve para provar
# a REGRESSÃO em si — validate não reprova nem antes nem depois — então este
# cenário inspeciona o artefato gerado diretamente:
#   1. prova positiva: com o gerador correto, o campo `req:` sai não-vazio
#      nos 3 CLIs (regressão NÃO presente);
#   2. prova de detecção: revertendo o gerador para gravar `req: ""` sempre
#      (o defeito original), a MESMA checagem reprova com diagnóstico
#      explícito — provando que a checagem tem poder de reprovação, não é
#      vácua.
# Sem o passo 2, o passo 1 sozinho não provaria nada: um `grep` que sempre
# retorna "ok" também "passaria" o passo 1.
#
# Corrompe a IMPLEMENTAÇÃO (gerador), nunca a asserção. Cobre os três CLIs.
# ---------------------------------------------------------------------------

# Ciclo simples (--req) num sandbox $1 usando o runtime dado em "$@": roda
# `init` + `roadmap new --req`, localiza o arquivo gerado e extrai o valor
# do campo `req:` do frontmatter. Compara contra o caminho EXATO passado a
# --req (não apenas "não-vazio") — uma regressão que gravasse o basename em
# vez do caminho completo no caminho simples (a mesma classe de defeito do
# Cenário 25, só que aqui) passaria despercebida por um teste de
# não-vazio. Sai com exit 1 e diagnóstico explícito se o campo divergir —
# usado tanto para a prova positiva (código correto, chamado diretamente)
# quanto para a prova de detecção (código corrompido, via assert_fails_with).
SIMPLE_REQ_FIELD_SCRIPT='
  set -e
  cd "$1"
  shift
  "$@" init >/dev/null
  "$@" roadmap new --title "AC2b Flag Source" --req docs/req/REQ-flag-source.md >/dev/null
  name=$(basename "$(find docs/roadmaps/backlog -name "*.md")")
  value=$(grep -m1 "^req: " "docs/roadmaps/backlog/$name" | sed -E "s/^req: \"?([^\"]*)\"?\$/\1/")
  if [[ "$value" != "docs/req/REQ-flag-source.md" ]]; then
    echo "req: field mismatch in roadmap generated via --req simple path (AC2b regression — expected docs/req/REQ-flag-source.md, got $value; validate does not flag this silently)"
    exit 1
  fi
  echo "req: field = $value (matches --req path, AC2b holds)"
'

# Helper: assert que o comando retorna exit 0 (prova positiva). Espelha
# assert_fails_with, mas na direção inversa — necessário porque o
# Cenário 26 primeiro precisa provar "código correto não regride" antes de
# provar "código corrompido é detectado".
assert_succeeds() {
  local label=$1
  shift
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: saiu com $status, esperava 0" >&2
    echo "  output: $out" >&2
    exit 1
  fi
  echo "OK   [falsify/$label]: $out"
}

# --- Go: prova positiva --------------------------------------------------
# Binário isolado (não $ROOT_DIR/bin/trackfw): a prova não pode depender de
# `make build` já ter rodado antes deste script — mesmo padrão de
# auto-suficiência do braço de detecção logo abaixo.
T26_BASE_GO_BIN="$WORK/s26-base-go-bin/trackfw"
mkdir -p "$(dirname "$T26_BASE_GO_BIN")"
build_go_or_fail "setup-s26-go-baseline-build" "$ROOT_DIR" "$T26_BASE_GO_BIN"

T26_BASE_GO="$WORK/s26-base-go"
mkdir -p "$T26_BASE_GO"
write_roadmap_acceptance_req_fixture "$T26_BASE_GO/docs/req/REQ-flag-source.md"
assert_succeeds "roadmap-req-frontmatter-path/go/simple-baseline" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26_BASE_GO" "$T26_BASE_GO_BIN"

# --- Go: prova de detecção (gerador corrompido para req: "" sempre) -------
T26C_GO_MOD="$WORK/s26-corrupt-go-mod"
mkdir -p "$T26C_GO_MOD/cmd" "$T26C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T26C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T26C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T26C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T26C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/roadmap.go" "$T26C_GO_MOD/internal/generators/roadmap.go" \
  ', date, content.REQPath, content.Title, date, content.REQPath)' \
  ', date, "", content.Title, date, content.REQPath)' \
  "s26-go"

T26C_GO_BIN="$WORK/s26-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T26C_GO_BIN")"
build_go_or_fail "setup-s26-go-build" "$T26C_GO_MOD" "$T26C_GO_BIN"

T26C_GO="$WORK/s26-corrupt-go"
mkdir -p "$T26C_GO"
write_roadmap_acceptance_req_fixture "$T26C_GO/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/go/simple-detects-regression" \
  "AC2b regression" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26C_GO" "$T26C_GO_BIN"

# --- Node: prova positiva --------------------------------------------------
T26_BASE_N="$WORK/s26-base-node"
mkdir -p "$T26_BASE_N"
setup_npm_tree "$T26_BASE_N"
write_roadmap_acceptance_req_fixture "$T26_BASE_N/docs/req/REQ-flag-source.md"
assert_succeeds "roadmap-req-frontmatter-path/node/simple-baseline" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26_BASE_N" node npm/bin/trackfw

# --- Node: prova de detecção ------------------------------------------------
T26C_N="$WORK/s26-corrupt-node"
mkdir -p "$T26C_N"
setup_npm_tree "$T26C_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/generators/roadmap.js" "$T26C_N/npm/src/generators/roadmap.js" \
  "const reqField = reqPath ? \`\"\${reqPath}\"\` : '\"\"'" \
  "const reqField = '\"\"'" \
  "s26-node"

write_roadmap_acceptance_req_fixture "$T26C_N/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/node/simple-detects-regression" \
  "AC2b regression" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26C_N" node npm/bin/trackfw

# --- Python: prova positiva -------------------------------------------------
T26_BASE_P="$WORK/s26-base-python"
mkdir -p "$T26_BASE_P"
cp -r "$ROOT_DIR/pypi" "$T26_BASE_P/pypi"
write_roadmap_acceptance_req_fixture "$T26_BASE_P/docs/req/REQ-flag-source.md"
assert_succeeds "roadmap-req-frontmatter-path/python/simple-baseline" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26_BASE_P" env "PYTHONPATH=$T26_BASE_P/pypi" python3 -m trackfw

# --- Python: prova de detecção ----------------------------------------------
T26C_P="$WORK/s26-corrupt-python"
mkdir -p "$T26C_P"
cp -r "$ROOT_DIR/pypi" "$T26C_P/pypi"
corrupt_python_func_literal \
  "$ROOT_DIR/pypi/trackfw/generators/roadmap.py" "$T26C_P/pypi/trackfw/generators/roadmap.py" \
  "_roadmap_template" \
  'req: "{req_path}"' \
  'req: ""'

write_roadmap_acceptance_req_fixture "$T26C_P/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/python/simple-detects-regression" \
  "AC2b regression" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26C_P" env "PYTHONPATH=$T26C_P/pypi" python3 -m trackfw

# ---------------------------------------------------------------------------
# Cenário 27 — validate: adr_accepted_when_req_done + blocked_by_draft_adr
# (ROADMAP-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida,
# ML-2A). Sem este cenário, `check-validate-parity.sh` passava vacuamente
# neste repositório — nenhum artefato aqui viola as regras novas, então um
# gate "verde" não discriminava a existência das regras de sua ausência. O
# mesmo valia para a correção da cegueira de `blocked_by_draft_adr` a
# `Status: Proposed` (o caminho normal de `adr new`) — nenhuma REQ Open deste
# repositório é bloqueada por ADR Proposed.
#
# Cobre as DUAS regras × os TRÊS CLIs, com dois braços por CLI:
#   - baseline: projeto-fixture com ADR Proposed + REQ Done referenciando-o
#     (deve violar adr_accepted_when_req_done) e REQ Open bloqueada pelo
#     mesmo ADR (deve violar blocked_by_draft_adr) — código correto,
#     assert_fails_with nos dois diagnósticos; e um segundo projeto com ADR
#     Superseded (aceito por exclusão) + REQ Done referenciando-o — não deve
#     violar, assert_succeeds.
#   - detecção: neutraliza o helper de resolução de status do ADR
#     (resolveAdrStatus/resolveAdrStatus/_extract_adr_status→_adr_not_accepted,
#     conforme o CLI) para sempre resolver "aceito"; roda validate contra o
#     MESMO projeto-fixture violador e prova, via assert_lacks_pattern (exige
#     exit 0 E ausência do diagnóstico), que as duas violações desaparecem —
#     a checagem tem poder de reprovação, não é vácua.
#
# Corrompe a IMPLEMENTAÇÃO (validador), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21/24/26.
# ---------------------------------------------------------------------------

# Scaffold mínimo de projeto trackfw (docs/adr, docs/req, docs/roadmaps/*,
# trackfw.yaml) — mesma estrutura de check-validate-parity.sh.
scaffold_adr_req_project() {
  local dest=$1
  mkdir -p "$dest/docs/adr" "$dest/docs/req" \
    "$dest/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}
  cat > "$dest/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF
}

# ADR fixture com status alinhado entre frontmatter e cabeçalho (caso
# canônico bem formado) — mesmo padrão de adrFixtureContent (validator_test.go).
write_adr_status_fixture() {
  local dest=$1 status=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: $status
date: 2026-08-01
author: ""
---

# ADR: fixture

> Date: 2026-08-01 | Status: $status

## Context
ctx

## Decision
decision
EOF
}

# REQ Done referenciando o ADR via frontmatter \`adr:\` e via a seção
# "## Linked ADR" — mesmo padrão de reqDoneFixtureContent (validator_test.go).
write_req_done_fixture() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Done
date: 2026-08-01
author: ""
adr: "$adr_rel"
roadmap: ""
---

# REQ: fixture

> Date: 2026-08-01 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: $adr_rel

## Linked Roadmap
Roadmap:
EOF
}

# REQ Open bloqueada pelo ADR via a seção "## Blocked by ADRs" — mesmo padrão
# do fixture de TestBlockedByDraftADR_REQOpen_ProposedADR_Violates.
write_req_open_blocked_fixture() {
  local dest=$1 adr_basename=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
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
- $adr_basename (Proposed)

## Linked Roadmap
Roadmap:
EOF
}

S27_MSG_ACCEPTED='is not accepted (status: Proposed)'
S27_MSG_BLOCKED='is blocked by not-accepted ADR: ADR-2026-08-01-proposed-fixture.md'

# --- Go: prova positiva (projeto violador + projeto não-violador) ---------
T27_GO_BIN="$WORK/s27-go-bin/trackfw"
mkdir -p "$(dirname "$T27_GO_BIN")"
build_go_or_fail "setup-s27-go-baseline-build" "$ROOT_DIR" "$T27_GO_BIN"

T27_GO_VIOLATING="$WORK/s27-go-violating"
scaffold_adr_req_project "$T27_GO_VIOLATING"
write_adr_status_fixture "$T27_GO_VIOLATING/docs/adr/ADR-2026-08-01-proposed-fixture.md" "Proposed"
write_req_done_fixture "$T27_GO_VIOLATING/docs/req/REQ-2026-08-01-done-fixture.md" \
  "docs/adr/ADR-2026-08-01-proposed-fixture.md"
write_req_open_blocked_fixture "$T27_GO_VIOLATING/docs/req/REQ-2026-08-01-blocked-fixture.md" \
  "ADR-2026-08-01-proposed-fixture.md"

assert_fails_with "adr-not-accepted/go/adr_accepted_when_req_done-baseline" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27_GO_BIN' validate"
assert_fails_with "adr-not-accepted/go/blocked_by_draft_adr-baseline" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27_GO_BIN' validate"

T27_GO_CLEAN="$WORK/s27-go-clean"
scaffold_adr_req_project "$T27_GO_CLEAN"
write_adr_status_fixture "$T27_GO_CLEAN/docs/adr/ADR-2026-08-01-superseded-fixture.md" "Superseded"
write_req_done_fixture "$T27_GO_CLEAN/docs/req/REQ-2026-08-01-done-superseded-fixture.md" \
  "docs/adr/ADR-2026-08-01-superseded-fixture.md"

assert_succeeds "adr-not-accepted/go/superseded-not-a-violation-baseline" \
  bash -c "cd '$T27_GO_CLEAN' && exec '$T27_GO_BIN' validate"

# --- Go: prova de detecção (resolveAdrStatus neutralizado) -----------------
T27C_GO_MOD="$WORK/s27-corrupt-go-mod"
mkdir -p "$T27C_GO_MOD/cmd" "$T27C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T27C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T27C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T27C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T27C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T27C_GO_MOD/internal/validator/validator.go" \
  'func resolveAdrStatus(content string) string {
	if status := extractFrontmatterField(content, "status"); status != "" {
		return status
	}' \
  'func resolveAdrStatus(content string) string {
	return "Accepted"
	if status := extractFrontmatterField(content, "status"); status != "" {
		return status
	}' \
  "s27-go"

T27C_GO_BIN="$WORK/s27-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T27C_GO_BIN")"
build_go_or_fail "setup-s27-go-build" "$T27C_GO_MOD" "$T27C_GO_BIN"

assert_lacks_pattern "adr-not-accepted/go/adr_accepted_when_req_done-detects-regression" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27C_GO_BIN' validate"
assert_lacks_pattern "adr-not-accepted/go/blocked_by_draft_adr-detects-regression" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27C_GO_BIN' validate"

# --- Node: prova positiva ---------------------------------------------------
T27_N_VIOLATING="$WORK/s27-node-violating"
setup_npm_tree "$T27_N_VIOLATING"
scaffold_adr_req_project "$T27_N_VIOLATING"
write_adr_status_fixture "$T27_N_VIOLATING/docs/adr/ADR-2026-08-01-proposed-fixture.md" "Proposed"
write_req_done_fixture "$T27_N_VIOLATING/docs/req/REQ-2026-08-01-done-fixture.md" \
  "docs/adr/ADR-2026-08-01-proposed-fixture.md"
write_req_open_blocked_fixture "$T27_N_VIOLATING/docs/req/REQ-2026-08-01-blocked-fixture.md" \
  "ADR-2026-08-01-proposed-fixture.md"

assert_fails_with "adr-not-accepted/node/adr_accepted_when_req_done-baseline" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_N_VIOLATING' && exec node npm/bin/trackfw validate"
assert_fails_with "adr-not-accepted/node/blocked_by_draft_adr-baseline" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_N_VIOLATING' && exec node npm/bin/trackfw validate"

T27_N_CLEAN="$WORK/s27-node-clean"
setup_npm_tree "$T27_N_CLEAN"
scaffold_adr_req_project "$T27_N_CLEAN"
write_adr_status_fixture "$T27_N_CLEAN/docs/adr/ADR-2026-08-01-superseded-fixture.md" "Superseded"
write_req_done_fixture "$T27_N_CLEAN/docs/req/REQ-2026-08-01-done-superseded-fixture.md" \
  "docs/adr/ADR-2026-08-01-superseded-fixture.md"

assert_succeeds "adr-not-accepted/node/superseded-not-a-violation-baseline" \
  bash -c "cd '$T27_N_CLEAN' && exec node npm/bin/trackfw validate"

# --- Node: prova de detecção (adrNotAcceptedStatusForRule neutralizado) ----
T27C_N="$WORK/s27-corrupt-node"
setup_npm_tree "$T27C_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$T27C_N/npm/src/validator/index.js" \
  "  const notAccepted = status.toLowerCase() === 'draft' || status.toLowerCase() === 'proposed'
" \
  "  const notAccepted = false
" \
  "s27-node"

assert_lacks_pattern "adr-not-accepted/node/adr_accepted_when_req_done-detects-regression" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_N_VIOLATING' && exec node '$T27C_N/npm/bin/trackfw' validate"
assert_lacks_pattern "adr-not-accepted/node/blocked_by_draft_adr-detects-regression" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_N_VIOLATING' && exec node '$T27C_N/npm/bin/trackfw' validate"

# --- Python: prova positiva -------------------------------------------------
T27_P_VIOLATING="$WORK/s27-python-violating"
mkdir -p "$T27_P_VIOLATING"
cp -r "$ROOT_DIR/pypi" "$T27_P_VIOLATING/pypi"
scaffold_adr_req_project "$T27_P_VIOLATING"
write_adr_status_fixture "$T27_P_VIOLATING/docs/adr/ADR-2026-08-01-proposed-fixture.md" "Proposed"
write_req_done_fixture "$T27_P_VIOLATING/docs/req/REQ-2026-08-01-done-fixture.md" \
  "docs/adr/ADR-2026-08-01-proposed-fixture.md"
write_req_open_blocked_fixture "$T27_P_VIOLATING/docs/req/REQ-2026-08-01-blocked-fixture.md" \
  "ADR-2026-08-01-proposed-fixture.md"

assert_fails_with "adr-not-accepted/python/adr_accepted_when_req_done-baseline" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_P_VIOLATING' && exec env PYTHONPATH='$T27_P_VIOLATING/pypi' python3 -m trackfw validate"
assert_fails_with "adr-not-accepted/python/blocked_by_draft_adr-baseline" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_P_VIOLATING' && exec env PYTHONPATH='$T27_P_VIOLATING/pypi' python3 -m trackfw validate"

T27_P_CLEAN="$WORK/s27-python-clean"
mkdir -p "$T27_P_CLEAN"
cp -r "$ROOT_DIR/pypi" "$T27_P_CLEAN/pypi"
scaffold_adr_req_project "$T27_P_CLEAN"
write_adr_status_fixture "$T27_P_CLEAN/docs/adr/ADR-2026-08-01-superseded-fixture.md" "Superseded"
write_req_done_fixture "$T27_P_CLEAN/docs/req/REQ-2026-08-01-done-superseded-fixture.md" \
  "docs/adr/ADR-2026-08-01-superseded-fixture.md"

assert_succeeds "adr-not-accepted/python/superseded-not-a-violation-baseline" \
  bash -c "cd '$T27_P_CLEAN' && exec env PYTHONPATH='$T27_P_CLEAN/pypi' python3 -m trackfw validate"

# --- Python: prova de detecção (_adr_not_accepted neutralizado) ------------
T27C_P="$WORK/s27-corrupt-python"
mkdir -p "$T27C_P"
cp -r "$ROOT_DIR/pypi" "$T27C_P/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/validator.py" "$T27C_P/pypi/trackfw/validator.py" \
  '    return _extract_adr_status(content).strip().lower() in ("draft", "proposed")
' \
  '    return False
' \
  "s27-python"

assert_lacks_pattern "adr-not-accepted/python/adr_accepted_when_req_done-detects-regression" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_P_VIOLATING' && exec env PYTHONPATH='$T27C_P/pypi' python3 -m trackfw validate"
assert_lacks_pattern "adr-not-accepted/python/blocked_by_draft_adr-detects-regression" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_P_VIOLATING' && exec env PYTHONPATH='$T27C_P/pypi' python3 -m trackfw validate"

# ---------------------------------------------------------------------------
# Cenário 28 — extractRefPath (e equivalentes) removem backtick da referência
# (REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-
# validate-no-python)
#
# `` ADR: `docs/adr/X.md` (prosa) `` é a forma real usada em REQs do próprio
# repositório. SEM remoção de backtick, o token extraído é "`docs/adr/X.md`"
# — não termina em ".md" — e a referência fica invisível EM SILÊNCIO: nenhuma
# regra que use extractRefPath a alcança. É especialmente grave quando a REQ
# NÃO tem `adr:` no frontmatter (só a prosa do corpo referencia o ADR) — o
# cenário aqui reproduz exatamente essa forma, sem fixture com backtick a
# checagem seria vácua (vault/notes/deteccao-de-status-de-adr-divergencias-
# entre-clis-2026-08-01.md).
#
# Cobre os TRÊS CLIs, dois braços cada:
#   - baseline: REQ Done SEM `adr:` no frontmatter, referenciando o ADR só via
#     `` ADR: `docs/adr/X.md` (prosa) `` na seção "## Linked ADR"; ADR alvo
#     Proposed. Código correto → assert_fails_with adr_accepted_when_req_done.
#   - detecção: reverte a remoção do backtick no extrator do CLI (mesmo ponto
#     de código alterado pela Wave 1, revertido ao estado anterior) e roda
#     validate contra o MESMO projeto-fixture violador — prova, via
#     assert_lacks_pattern, que a violação desaparece (a referência volta a
#     ficar invisível), confirmando que a checagem tem poder de reprovação.
#
# Corrompe a IMPLEMENTAÇÃO (extrator), nunca a asserção — mesmo padrão do
# Cenário 27.
# ---------------------------------------------------------------------------

# REQ Done SEM `adr:` no frontmatter, referenciando o ADR só via backtick na
# seção "## Linked ADR" — a forma real usada em REQs do repositório.
write_req_done_fixture_backtick_body_only() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Done
date: 2026-08-02
author: ""
adr: ""
roadmap: ""
---

# REQ: fixture com backtick

> Date: 2026-08-02 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: \`$adr_rel\` (prosa)

## Linked Roadmap
Roadmap:
EOF
}

S28_MSG_ACCEPTED='is not accepted (status: Proposed)'

# --- Go: prova positiva -----------------------------------------------------
# Reusa T27_GO_BIN (binário Go limpo, construído a partir do ROOT_DIR sem
# corrupção) — não precisa recompilar. Se o Cenário 27 for removido, mova a
# compilação para cá.
T28_GO_VIOLATING="$WORK/s28-go-violating"
scaffold_adr_req_project "$T28_GO_VIOLATING"
write_adr_status_fixture "$T28_GO_VIOLATING/docs/adr/ADR-2026-08-02-proposed-fixture.md" "Proposed"
write_req_done_fixture_backtick_body_only "$T28_GO_VIOLATING/docs/req/REQ-2026-08-02-backtick-fixture.md" \
  "docs/adr/ADR-2026-08-02-proposed-fixture.md"

assert_fails_with "backtick-ref/go/adr_accepted_when_req_done-baseline" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_GO_VIOLATING' && exec '$T27_GO_BIN' validate"

# --- Go: prova de detecção (backtick reintroduzido em extractRefPath) ------
T28C_GO_MOD="$WORK/s28-corrupt-go-mod"
mkdir -p "$T28C_GO_MOD/cmd" "$T28C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T28C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T28C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T28C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T28C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T28C_GO_MOD/internal/validator/validator.go" \
  'v := strings.Trim(fields[0], "\"'"'"'`")' \
  'v := strings.Trim(fields[0], "\"'"'"'")' \
  "s28-go"

T28C_GO_BIN="$WORK/s28-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T28C_GO_BIN")"
build_go_or_fail "setup-s28-go-build" "$T28C_GO_MOD" "$T28C_GO_BIN"

assert_lacks_pattern "backtick-ref/go/adr_accepted_when_req_done-detects-regression" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_GO_VIOLATING' && exec '$T28C_GO_BIN' validate"

# --- Node: prova positiva ---------------------------------------------------
T28_N_VIOLATING="$WORK/s28-node-violating"
setup_npm_tree "$T28_N_VIOLATING"
scaffold_adr_req_project "$T28_N_VIOLATING"
write_adr_status_fixture "$T28_N_VIOLATING/docs/adr/ADR-2026-08-02-proposed-fixture.md" "Proposed"
write_req_done_fixture_backtick_body_only "$T28_N_VIOLATING/docs/req/REQ-2026-08-02-backtick-fixture.md" \
  "docs/adr/ADR-2026-08-02-proposed-fixture.md"

assert_fails_with "backtick-ref/node/adr_accepted_when_req_done-baseline" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_N_VIOLATING' && exec node npm/bin/trackfw validate"

# --- Node: prova de detecção (backtick reintroduzido em extractRefPath) ----
T28C_N="$WORK/s28-corrupt-node"
setup_npm_tree "$T28C_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$T28C_N/npm/src/validator/index.js" \
  "      val = val.replace(/^[\"'\`]|[\"'\`]\$/g, '')
" \
  "      val = val.replace(/^[\"']|[\"']\$/g, '')
" \
  "s28-node"

assert_lacks_pattern "backtick-ref/node/adr_accepted_when_req_done-detects-regression" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_N_VIOLATING' && exec node '$T28C_N/npm/bin/trackfw' validate"

# --- Python: prova positiva -------------------------------------------------
T28_P_VIOLATING="$WORK/s28-python-violating"
mkdir -p "$T28_P_VIOLATING"
cp -r "$ROOT_DIR/pypi" "$T28_P_VIOLATING/pypi"
scaffold_adr_req_project "$T28_P_VIOLATING"
write_adr_status_fixture "$T28_P_VIOLATING/docs/adr/ADR-2026-08-02-proposed-fixture.md" "Proposed"
write_req_done_fixture_backtick_body_only "$T28_P_VIOLATING/docs/req/REQ-2026-08-02-backtick-fixture.md" \
  "docs/adr/ADR-2026-08-02-proposed-fixture.md"

assert_fails_with "backtick-ref/python/adr_accepted_when_req_done-baseline" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_P_VIOLATING' && exec env PYTHONPATH='$T28_P_VIOLATING/pypi' python3 -m trackfw validate"

# --- Python: prova de detecção (backtick reintroduzido em _extract_ref_path)
#
# Nota (ML-2A, ROADMAP-2026-08-02-fechar-as-duas-divergencias-de-parsing-
# remanescentes-no-python): a extração de referência foi refatorada em
# 588b9b8 (item 1 deste roadmap — delimitador não pareado) para
# _strip_ref_delimiters()/_REF_DELIMITERS; o literal original deste cenário
# (normalize_yaml_flat_value + par casado de backtick) não existe mais em
# validator.py e corrupt_literal reprovaria com "got 0 occurrences". Alvo
# ajustado para remover só o backtick de _REF_DELIMITERS — corrupção mínima
# equivalente (isola o item 1, que segue coberto pelo Cenário 32 abaixo, do
# suporte a backtick que este cenário prova).
T28C_P="$WORK/s28-corrupt-python"
mkdir -p "$T28C_P"
cp -r "$ROOT_DIR/pypi" "$T28C_P/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/validator.py" "$T28C_P/pypi/trackfw/validator.py" \
  '_REF_DELIMITERS = ("\"", "'"'"'", "`")' \
  '_REF_DELIMITERS = ("\"", "'"'"'")' \
  "s28-python"

assert_lacks_pattern "backtick-ref/python/adr_accepted_when_req_done-detects-regression" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_P_VIOLATING' && exec env PYTHONPATH='$T28C_P/pypi' python3 -m trackfw validate"

# ---------------------------------------------------------------------------
# Cenário 29 — os 3 CLIs imprimem a MESMA mensagem de sucesso do `validate`
# sem violações (REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-
# de-sucesso-do-validate-no-python, ponto 3)
#
# Nada em CI garantia isto até agora — foi exatamente por não haver gate que
# o Python ficou meses imprimindo o literal hardcoded "✓ Governance OK" em
# vez da chave `validate.ok` do i18n (que os 3 CLIs compartilham e que os
# outros dois já usavam). Um diff três-a-três puro (sem pin) passaria mesmo
# se os 3 imprimissem a mesma coisa errada, ou nada — por isso o baseline
# também compara contra o literal esperado pinado, não só entre si.
#
#   - baseline: projeto-fixture sem nenhum arquivo em docs/adr, docs/req ou
#     docs/roadmaps/* (zero violações) — os 3 CLIs devem imprimir,
#     byte-a-byte, exatamente "✓ No violations found." E os três devem ser
#     idênticos entre si.
#   - detecção: reverte SÓ o Python para o literal hardcoded antigo
#     ("✓ Governance OK") no ponto exato onde a Wave 1 trocou pela chave
#     `validate.ok` (commands/validate.py) — prova que a comparação
#     byte-a-byte reprova a regressão que viveu meses sem detecção.
#
# Corrompe a IMPLEMENTAÇÃO (mensagem do Python), nunca a asserção.
# ---------------------------------------------------------------------------

# A mensagem esperada é resolvida via i18n_t("validate.ok") pelos 3 CLIs, que
# depende do locale ativo do processo. Fixamos LANG/LC_ALL=en_US.UTF-8 nas
# chamadas comparadas para que o cenário seja determinístico independente do
# locale da máquina onde o gate roda (ADR-2026-08-04-make-quality-forca-
# locale-fixo-no-gate-de-falsificacao-em-vez-de-pin-em-ingles) — em vez de
# ler a expectativa dinamicamente, o que enfraqueceria a prova de detecção
# de regressão abaixo.
S29_EXPECTED=$'\xe2\x9c\x93 No violations found.\n'

T29_PROJECT="$WORK/s29-clean-project"
scaffold_adr_req_project "$T29_PROJECT"

s29_go_out=$(cd "$T29_PROJECT" && env LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 "$T27_GO_BIN" validate)$'\n'
s29_node_out=$(cd "$T29_PROJECT" && env LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 node "$ROOT_DIR/npm/bin/trackfw" validate)$'\n'
s29_python_out=$(cd "$T29_PROJECT" && env LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate)$'\n'

if [[ "$s29_go_out" == "$S29_EXPECTED" && "$s29_node_out" == "$S29_EXPECTED" && "$s29_python_out" == "$S29_EXPECTED" ]]; then
  echo "OK   [falsify/validate-ok-message/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/validate-ok-message/baseline-byte-identical-and-pinned]: esperava '$S29_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s29_go_out")" >&2
  echo "  node:   $(printf '%q' "$s29_node_out")" >&2
  echo "  python: $(printf '%q' "$s29_python_out")" >&2
  exit 1
fi

# --- Python: prova de detecção (literal hardcoded antigo reintroduzido) ----
T29C_P="$WORK/s29-corrupt-python"
mkdir -p "$T29C_P"
cp -r "$ROOT_DIR/pypi" "$T29C_P/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/validate.py" "$T29C_P/pypi/trackfw/commands/validate.py" \
  'print(_green(i18n_t("validate.ok")))' \
  'print(_green("✓ Governance OK"))' \
  "s29-python"

s29c_python_out=$(cd "$T29_PROJECT" && env LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 PYTHONPATH="$T29C_P/pypi" python3 -m trackfw validate)$'\n'
if [[ "$s29c_python_out" != "$S29_EXPECTED" ]]; then
  echo "OK   [falsify/validate-ok-message/python-detects-regression]"
else
  echo "FAIL [falsify/validate-ok-message/python-detects-regression]: literal hardcoded reintroduzido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 30 — `trackfw status`: bloco 📊 Inventory byte-idêntico nos 3 CLIs
# no modo flat, com fixture DISCRIMINANTE (roadmap em analyzing/ + REQs
# Open/Done/Closed) — prova ROADMAP-2026-08-02-convergir-o-comando-status-
# dos-tres-clis-num-formato-unico (ML-3A), AC2 (analyzing contado, antes
# omitido em 5 de 6 pontos de enumeração no Python) e AC3 (REQs
# discriminadas por status real, antes Done/Closed agrupados no Python).
#
# O repositório real deste projeto tem "analyzing 0" — não exercitaria a
# correção principal. A fixture PRECISA ter >=1 roadmap em analyzing/ e as
# 3 combinações de status de REQ, senão o cenário não discrimina nada.
#
# Mesmo padrão dos Cenários 28/29: compara contra um LITERAL PINADO, não os
# 3 CLIs entre si — um diff três-a-três passaria se todos derivassem juntos
# do mesmo bug (ex: todos omitindo analyzing), ou se todos imprimissem
# vazio.
#
# Corrompe a IMPLEMENTAÇÃO (Go: a lista de estados enumerados em
# inventoryBlock), nunca a asserção — mesmo padrão dos Cenários
# 14/16/17/20/21/24/25/26/27/28/29.
# ---------------------------------------------------------------------------

# REQ mínima com status controlado — só o frontmatter importa para o bloco
# Inventory (contagem por status), mas o corpo segue o mesmo esqueleto das
# demais fixtures de REQ do harness (write_req_done_fixture etc.).
write_req_status_fixture() {
  local dest=$1 status=$2 title=$3
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: $status
date: 2026-08-02
author: ""
adr: ""
roadmap: ""
---

# REQ: $title

> Date: 2026-08-02 | Status: $status

## Motivation
motivo

## Acceptance Criteria
- [ ] item

## Linked ADR
ADR:

## Linked Roadmap
Roadmap:
EOF
}

# Roadmap mínimo com status controlado, para popular um estado específico
# (ex: analyzing/) na contagem do bloco Inventory.
write_roadmap_state_fixture() {
  local dest=$1 status=$2 title=$3
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: $status
date: 2026-08-02
---

# Roadmap: $title

> Status: $status
EOF
}

S30_PROJECT="$WORK/s30-status-project"
scaffold_adr_req_project "$S30_PROJECT"
write_req_status_fixture "$S30_PROJECT/docs/req/REQ-open.md" "Open" "open fixture"
write_req_status_fixture "$S30_PROJECT/docs/req/REQ-done.md" "Done" "done fixture"
write_req_status_fixture "$S30_PROJECT/docs/req/REQ-closed.md" "Closed" "closed fixture"
write_roadmap_state_fixture "$S30_PROJECT/docs/roadmaps/analyzing/ROADMAP-analyzing.md" "analyzing" "analyzing fixture"

S30_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        3  (1 Open · 1 Done · 1 Closed)\n   Roadmaps    1\n     backlog 0 · analyzing 1 · wip 0\n     blocked 0 · done 0 · abandoned 0\n\n🔄 WIP (0)\n\n❌ Blocked (0)\n\n✅ Done (last 5)\n\n────────────────────────────────────────\n'

# --- prova positiva: os 3 CLIs, contra o literal pinado ---------------------
s30_go_out=$(cd "$S30_PROJECT" && "$T27_GO_BIN" status)$'\n'
s30_node_out=$(cd "$S30_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" status)$'\n'
s30_python_out=$(cd "$S30_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw status)$'\n'

if [[ "$s30_go_out" == "$S30_EXPECTED" && "$s30_node_out" == "$S30_EXPECTED" && "$s30_python_out" == "$S30_EXPECTED" ]]; then
  echo "OK   [falsify/status-inventory/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/status-inventory/baseline-byte-identical-and-pinned]: esperava '$S30_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s30_go_out")" >&2
  echo "  node:   $(printf '%q' "$s30_node_out")" >&2
  echo "  python: $(printf '%q' "$s30_python_out")" >&2
  exit 1
fi

# --- braço de detecção: Go reverte a enumeração de analyzing (5 de 6 -------
# estados) — reproduz o defeito histórico do Python pré-Wave-1 num CLI
# concreto e prova que a comparação byte-a-byte reprova a omissão.
T30C_GO_MOD="$WORK/s30-corrupt-go"
mkdir -p "$T30C_GO_MOD/cmd" "$T30C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T30C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T30C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T30C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T30C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T30C_GO_MOD/internal/validator/validator.go" \
  'states := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}' \
  'states := []string{"backlog", "wip", "blocked", "done", "abandoned"}' \
  "s30-go"

T30C_GO_BIN="$WORK/s30-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T30C_GO_BIN")"
build_go_or_fail "setup-s30-go-corrupt-build" "$T30C_GO_MOD" "$T30C_GO_BIN"

s30c_go_out=$(cd "$S30_PROJECT" && "$T30C_GO_BIN" status)$'\n'
if [[ "$s30c_go_out" != "$S30_EXPECTED" ]]; then
  echo "OK   [falsify/status-inventory/go-detects-analyzing-omission]"
else
  echo "FAIL [falsify/status-inventory/go-detects-analyzing-omission]: enumeração de analyzing revertida mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 31 — `trackfw status` no modo `by_agent`: bloco 📊 Inventory +
# "⚙ WIP by Agent" byte-idênticos nos 3 CLIs, contra literal pinado.
#
# Foi exatamente no by_agent que o Python divergiu historicamente — a seção
# listava os NOMES DE ESTADO (backlog/wip/blocked/...) como se fossem
# agentes, em vez de agregar por agente configurado. check-artifact-parity.sh
# e check-validate-parity.sh não cobrem `status`; nenhum gate de paridade
# existente comparava os 3 CLIs nesse modo — por isso este cenário próprio.
#
# `agents:` em lista de BLOCO (não flow-style `[apolo, zeus]`) — o parser
# YAML leve do Python (pypi/trackfw/config.py) não trata flow-style, e isso
# é um defeito PRÉ-EXISTENTE e distinto do `status` (não corrigido aqui, já
# reportado para fila própria). Lista em bloco evita acoplar este cenário a
# esse defeito conhecido.
# ---------------------------------------------------------------------------

S31_PROJECT="$WORK/s31-status-by-agent-project"
mkdir -p "$S31_PROJECT/docs/adr" "$S31_PROJECT/docs/req"
mkdir -p "$S31_PROJECT/docs/roadmaps/apolo"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S31_PROJECT/docs/roadmaps/zeus"/{backlog,analyzing,wip,blocked,done,abandoned}
cat > "$S31_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
- apolo
- zeus
EOF
# zeus tem roadmap em analyzing/ mas NENHUM em wip/ — discriminante
# deliberado: prova que o bloco Inventory agrega através de TODOS os
# agentes (Roadmaps total = 2, analyzing 1 · wip 1), enquanto a seção
# "⚙ WIP by Agent" só lista quem tem wip não-vazio (só [apolo] aparece).
write_roadmap_state_fixture "$S31_PROJECT/docs/roadmaps/apolo/wip/ROADMAP-apolo-wip.md" "wip" "apolo wip fixture"
write_roadmap_state_fixture "$S31_PROJECT/docs/roadmaps/zeus/analyzing/ROADMAP-zeus-analyzing.md" "analyzing" "zeus analyzing fixture"

S31_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    2\n     backlog 0 · analyzing 1 · wip 1\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [apolo] WIP (1)\n    ROADMAP-apolo-wip.md\n\n────────────────────────────────────────\n'

s31_go_out=$(cd "$S31_PROJECT" && "$T27_GO_BIN" status)$'\n'
s31_node_out=$(cd "$S31_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" status)$'\n'
s31_python_out=$(cd "$S31_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw status)$'\n'

if [[ "$s31_go_out" == "$S31_EXPECTED" && "$s31_node_out" == "$S31_EXPECTED" && "$s31_python_out" == "$S31_EXPECTED" ]]; then
  echo "OK   [falsify/status-inventory-by-agent/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/status-inventory-by-agent/baseline-byte-identical-and-pinned]: esperava '$S31_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s31_go_out")" >&2
  echo "  node:   $(printf '%q' "$s31_node_out")" >&2
  echo "  python: $(printf '%q' "$s31_python_out")" >&2
  exit 1
fi

# --- braço de detecção: Python corrompe SÓ o subdiretório lido pelo loop de
# listagem por agente (wip → backlog), deixando a agregação do Inventory
# (_roadmap_counts_by_agent/totals, função separada) intacta. Isolado desta
# forma porque um swap de `agents` inteiro (ex: por _ROADMAP_STATES, o bug
# histórico literal) já derruba o bloco Inventory sozinho — a asserção
# passaria mesmo que o corpo da seção "⚙ WIP by Agent" nunca fosse
# comparado byte-a-byte, mascarando exatamente o que este cenário promete
# cobrir. Com o Inventory permanecendo idêntico ao pinado, a única forma da
# reprovação abaixo passar é a comparação byte-a-byte ter de fato pego a
# divergência na seção "⚙ WIP by Agent" (a linha "[apolo] WIP (1)" some).
T31C_PY="$WORK/s31-corrupt-python"
mkdir -p "$T31C_PY"
cp -r "$ROOT_DIR/pypi" "$T31C_PY/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/status.py" "$T31C_PY/pypi/trackfw/commands/status.py" \
  '            agent_wip = _list_files(os.path.join(roadmap_dir, agent, "wip"))' \
  '            agent_wip = _list_files(os.path.join(roadmap_dir, agent, "backlog"))' \
  "s31-python"

s31c_python_out=$(cd "$S31_PROJECT" && env PYTHONPATH="$T31C_PY/pypi" python3 -m trackfw status)$'\n'
if [[ "$s31c_python_out" != "$S31_EXPECTED" ]]; then
  echo "OK   [falsify/status-inventory-by-agent/python-detects-wip-by-agent-body-drift]"
else
  echo "FAIL [falsify/status-inventory-by-agent/python-detects-wip-by-agent-body-drift]: subdiretório do loop por agente trocado mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 32 — item 1 do ROADMAP-2026-08-02-fechar-as-duas-divergencias-de-
# parsing-remanescentes-no-python: _extract_ref_path (Python) remove um
# delimitador NÃO PAREADO (aspas/backtick só de um lado) tão bem quanto
# Go/Node — antes (pré-588b9b8) devolvia "" e a referência ficava invisível
# em silêncio.
#
# Fixture discriminante: `ADR: "docs/adr/X.md'` no corpo (aspa dupla
# abrindo, aspa simples fechando — delimitadores MISTOS, não pareados). Os
# Cenários 27/28 já existentes usam aspas pareadas ou backtick pareado —
# nenhum dos dois exercita esta classe. Sem a correção, val termina em "'"
# (não ".md") e _extract_ref_path descarta a referência silenciosamente — a
# violação adr_accepted_when_req_done nunca dispara.
#
#   - baseline: os 3 CLIs, contra o MESMO projeto-fixture, reprovam com o
#     diagnóstico de ADR não aceito (S27_MSG_ACCEPTED) — prova que os 3
#     concordam (Go/Node já eram a referência; Python alinhado por 588b9b8).
#   - detecção: reverte SÓ o Python — o call site exato alterado por
#     588b9b8 (_strip_ref_delimiters → normalize_yaml_flat_value, que exige
#     par casado) — e prova, via assert_lacks_pattern, que a violação
#     desaparece na MESMA fixture.
#
# Corrompe a IMPLEMENTAÇÃO (o ponto exato revertido por 588b9b8), nunca a
# asserção — mesmo padrão dos Cenários 27/28/29/30/31. Reusa T27_GO_BIN
# (binário Go limpo) e S27_MSG_ACCEPTED.
# ---------------------------------------------------------------------------

write_req_done_fixture_unpaired_delimiter_body_only() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Done
date: 2026-08-02
author: ""
adr: ""
roadmap: ""
---

# REQ: fixture com delimitador não pareado

> Date: 2026-08-02 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: "$adr_rel'

## Linked Roadmap
Roadmap:
EOF
}

T32_PROJECT="$WORK/s32-unpaired-delimiter-project"
scaffold_adr_req_project "$T32_PROJECT"
write_adr_status_fixture "$T32_PROJECT/docs/adr/ADR-2026-08-02-proposed-fixture.md" "Proposed"
write_req_done_fixture_unpaired_delimiter_body_only \
  "$T32_PROJECT/docs/req/REQ-2026-08-02-unpaired-delimiter-fixture.md" \
  "docs/adr/ADR-2026-08-02-proposed-fixture.md"

assert_fails_with "unpaired-delimiter/go/adr_accepted_when_req_done-baseline" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T32_PROJECT' && exec '$T27_GO_BIN' validate"
assert_fails_with "unpaired-delimiter/node/adr_accepted_when_req_done-baseline" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T32_PROJECT' && exec node '$ROOT_DIR/npm/bin/trackfw' validate"
assert_fails_with "unpaired-delimiter/python/adr_accepted_when_req_done-baseline" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T32_PROJECT' && exec env PYTHONPATH='$ROOT_DIR/pypi' python3 -m trackfw validate"

# --- Python: prova de detecção (delimitador não pareado deixa de ser -------
# removido — reverte exatamente o call site alterado por 588b9b8)
T32C_P="$WORK/s32-corrupt-python"
mkdir -p "$T32C_P"
cp -r "$ROOT_DIR/pypi" "$T32C_P/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/validator.py" "$T32C_P/pypi/trackfw/validator.py" \
  '            val = _strip_ref_delimiters(val)
' \
  '            val = normalize_yaml_flat_value(val)
' \
  "s32-python"

assert_lacks_pattern "unpaired-delimiter/python/adr_accepted_when_req_done-detects-regression" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T32_PROJECT' && exec env PYTHONPATH='$T32C_P/pypi' python3 -m trackfw validate"

# ---------------------------------------------------------------------------
# Cenário 33 — item 2 do ROADMAP-2026-08-02-fechar-as-duas-divergencias-de-
# parsing-remanescentes-no-python: `trackfw status` no fallback de agentes
# (by_agent SEM `agents:` configurado) lista os subdiretórios em ordem
# ALFABÉTICA nos 3 CLIs — antes (pré-588b9b8) o Python (_list_dirs, pypi/
# trackfw/commands/status.py) devolvia a ordem crua do filesystem,
# divergindo de Go (os.ReadDir, ordenado por contrato da stdlib) e Node
# (fs.readdirSync, ordenado neste filesystem).
#
# Fixture discriminante: `by_agent` SEM `agents:` no trackfw.yaml (força o
# fallback) — o Cenário 31 já existente configura `agents:` em lista de
# bloco e por isso NÃO passa pelo fallback; não cobre este item. Os
# subdiretórios são criados FORA de ordem alfabética (zeus antes de apolo) —
# documenta a intenção do defeito histórico (a ordem devolvida dependia da
# ordem de criação no filesystem), mesmo que o braço de detecção abaixo não
# dependa dela para reprovar (ver nota). AMBOS os agentes têm roadmap em
# wip/ — com só um WIP não-vazio (padrão do Cenário 31), a ordenação não
# apareceria na saída e a checagem seria vácua.
#
#   - baseline: os 3 CLIs, contra o literal PINADO (capturado rodando os 3
#     CLIs reais contra a fixture, não construído à mão), byte-idênticos —
#     [apolo] antes de [zeus] em "⚙ WIP by Agent" e no total do Inventory.
#   - detecção: Python reverte `_list_dirs` para `sorted(..., reverse=True)`
#     — mesma linha alterada por 588b9b8, mesmo ponto de código — e prova,
#     por asserção POSITIVA (não só "!= esperado", que também passaria por
#     um crash ou saída vazia), que [zeus] passa a aparecer ANTES de
#     [apolo] na saída corrompida.
#
#     NOTA: o braço de detecção usa `reverse=True`, não `os.listdir` cru
#     (sem sorted nenhum) como o defeito histórico literal. `os.listdir`
#     cru depende da ordem de readdir() do filesystem — em APFS (macOS,
#     testado aqui) ela preserva ordem de criação e o cenário reprova; em
#     ext4 com dir_index (Linux, comum em CI) readdir devolve ordem hash,
#     que para {apolo, zeus} pode sair alfabética por coincidência,
#     tornando o braço inerte (falso "checagem vácua") nessa máquina.
#     `reverse=True` é determinística em qualquer filesystem — mais forte
#     que o defeito original (que era não-determinístico), mas prova
#     exatamente o que o cenário promete: que a comparação byte-a-byte tem
#     poder de reprovação sobre a ORDEM dos agentes, não apenas sobre o
#     conjunto.
#
# Corrompe a IMPLEMENTAÇÃO, nunca a asserção — mesmo padrão dos Cenários
# 14/16/17/20/21/24/25/26/27/28/29/30/31/32.
# ---------------------------------------------------------------------------

S33_PROJECT="$WORK/s33-status-by-agent-fallback-order-project"
mkdir -p "$S33_PROJECT/docs/adr" "$S33_PROJECT/docs/req"
mkdir -p "$S33_PROJECT/docs/roadmaps/zeus"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S33_PROJECT/docs/roadmaps/apolo"/{backlog,analyzing,wip,blocked,done,abandoned}
cat > "$S33_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
EOF
# zeus criado ANTES de apolo — sem sorted(), listdir cru preserva esta ordem
# de criação neste filesystem, tornando a fixture discriminante.
write_roadmap_state_fixture "$S33_PROJECT/docs/roadmaps/zeus/wip/ROADMAP-zeus-wip.md" "wip" "zeus wip fixture"
write_roadmap_state_fixture "$S33_PROJECT/docs/roadmaps/apolo/wip/ROADMAP-apolo-wip.md" "wip" "apolo wip fixture"

S33_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    2\n     backlog 0 · analyzing 0 · wip 2\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [apolo] WIP (1)\n    ROADMAP-apolo-wip.md\n  [zeus] WIP (1)\n    ROADMAP-zeus-wip.md\n\n────────────────────────────────────────\n'

s33_go_out=$(cd "$S33_PROJECT" && "$T27_GO_BIN" status)$'\n'
s33_node_out=$(cd "$S33_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" status)$'\n'
s33_python_out=$(cd "$S33_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw status)$'\n'

if [[ "$s33_go_out" == "$S33_EXPECTED" && "$s33_node_out" == "$S33_EXPECTED" && "$s33_python_out" == "$S33_EXPECTED" ]]; then
  echo "OK   [falsify/status-by-agent-fallback-order/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/status-by-agent-fallback-order/baseline-byte-identical-and-pinned]: esperava '$S33_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s33_go_out")" >&2
  echo "  node:   $(printf '%q' "$s33_node_out")" >&2
  echo "  python: $(printf '%q' "$s33_python_out")" >&2
  exit 1
fi

# --- braço de detecção: Python reverte a ordenação do resolvedor canônico
# (config.resolve_agent_namespaces) para ordem determinística invertida.
# RETARGET (ML-1A, REQ-2026-08-29): _get_agents (status.py) parou de chamar
# _list_dirs diretamente e passou a delegar em validator.resolve_agent_namespaces
# (re-export de config.resolve_agent_namespaces) — o resolvedor canônico único
# desta REQ. Corromper _list_dirs não afeta mais a saída de `status`, tornando
# a checagem vácua; o ponto que de fato ordena a lista de agentes agora é o
# `sorted(...)` dentro de resolve_agent_namespaces em config.py.
T33C_PY="$WORK/s33-corrupt-python"
mkdir -p "$T33C_PY"
cp -r "$ROOT_DIR/pypi" "$T33C_PY/pypi"
# Literal atualizado pelo ML-2A (REQ-2026-08-29): resolveAgentNamespaces passou a filtrar
# entradas comprovadamente infra (isInfraDirName — nomes iniciando com "." e "node_modules") antes
# de ordenar; o bloco `sorted(...)` corrompido abaixo precisa casar com essa forma nova.
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$T33C_PY/pypi/trackfw/config.py" \
  '            from_disk = sorted(
                e.name for e in it
                if e.is_dir(follow_symlinks=False)  # symlinks retornam False — nunca seguidos
                and not is_infra_dir_name(e.name)  # ML-2A: nunca vira namespace, ver comentário abaixo
            )
' \
  '            from_disk = sorted(
                (e.name for e in it
                 if e.is_dir(follow_symlinks=False)  # symlinks retornam False — nunca seguidos
                 and not is_infra_dir_name(e.name)),  # ML-2A: nunca vira namespace, ver comentário abaixo
                reverse=True,
            )
' \
  "s33-python"

s33c_python_out=$(cd "$S33_PROJECT" && env PYTHONPATH="$T33C_PY/pypi" python3 -m trackfw status)$'\n'
if [[ "$s33c_python_out" == "$S33_EXPECTED" ]]; then
  echo "FAIL [falsify/status-by-agent-fallback-order/python-detects-order-regression]: resolve_agent_namespaces revertido para ordem invertida mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF $'[zeus] WIP (1)\n    ROADMAP-zeus-wip.md\n  [apolo]' <<<"$s33c_python_out"; then
  echo "OK   [falsify/status-by-agent-fallback-order/python-detects-order-regression]"
else
  echo "FAIL [falsify/status-by-agent-fallback-order/python-detects-order-regression]: saída corrompida diverge do pinado, mas não pela ordem esperada (zeus antes de apolo) — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s33c_python_out")" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 34 — item 3 do ROADMAP-2026-08-02-fechar-as-duas-divergencias-de-
# parsing-remanescentes-no-python: Go e Node aceitam sequência YAML em bloco
# NÃO indentada ("agents:\n- apolo") — antes (pré-d208971) tratavam a linha
# "- apolo" como top-level (por falta de indentação) e DESCARTAVAM a lista
# em silêncio, caindo no fallback de varrer subdiretórios. O Python já lia
# corretamente (referência).
#
# RETARGET (ML-2A, 2026-08-02, Ártemis): a Wave 1 do ROADMAP-substituir-os-
# parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis substituiu o
# scanner linha-a-linha do Go e do Node por gopkg.in/yaml.v3 / `yaml` 2.x —
# os literais originais (`isListItem`/`continuesOpenList`) não existem mais;
# rodar os 82 cenários herdados ANTES de editar (exigido pelo ML-2A) falhou
# no setup deste cenário com "expected exactly 1 occurrence of pattern, got
# 0", o sintoma descrito em vault/notes/cenarios-de-falsificacao-quebram-em-
# refactor-do-alvo-2026-08-02.md. Diferença deste caso para o do vault: ali
# havia um ponto de código NOVO equivalente para retargetar a corrupção
# preservando a MESMA propriedade (suporte a backtick). Aqui não há mais
# nenhum código que trate "sequência não-indentada" como caso especial — uma
# biblioteca YAML de verdade não tem esse conceito, a fixture não-indentada é
# só YAML válido comum. A corrupção foi retargetada para o ponto genérico
# mais próximo que ainda preserva a intenção operacional do cenário — "a
# lista `agents:` é lida, não descartada" — corrompendo a atribuição de
# `cfg.Agents`/`cfg.agents` a partir do valor já parseado. Isso já NÃO prova
# mais nada sobre indentação especificamente (yaml.v3/`yaml` tratam bloco
# indentado e não-indentado de forma idêntica — não há mais um branch de
# código que só dispara para um dos dois); prova que a leitura da chave
# `agents:` (SEQUÊNCIA em bloco, incluindo a forma não-indentada que a
# fixture já usa) ainda popula `cfg.Agents`, e que removê-la reproduz o
# MESMO sintoma observável de antes (fallback devolve `zeus` extra) — a
# fixture não-indentada permanece no cenário como o vestígio do caso
# histórico original, mas o BRAÇO de detecção não é mais seletivo por
# indentação.
# Fixture discriminante: `agents:` em lista de bloco NÃO indentada,
# configurando SÓ `zeus`, enquanto `docs/roadmaps/` no disco tem `apolo`
# **e** `zeus` (ambos com roadmap em wip/). Os cenários existentes (ex. 31)
# usam forma indentada — não exercitam o defeito.
#
# RETARGET 2 (ML-1A, REQ-2026-08-29, apolo-tf): resolveAgentNamespaces/
# resolve_agent_namespaces (o resolvedor canônico desta REQ) passou a devolver
# a UNIÃO entre `agents:` e o disco — não mais a substituição. Com isso,
# `zeus` deixou de ficar invisível quando `agents: [apolo]` e `zeus/` existe
# só em disco (é exatamente o comportamento que a REQ corrige), e a
# divergência "zeus aparece ou não" que discriminava este cenário até
# REQ-2026-08-29 ficou vácua: zeus agora aparece nos dois braços (parser
# correto OU quebrado), porque o resolvedor sempre lê o disco também.
# RETARGET 3 (ML-3A, REQ-2026-08-29, artemis-tf): a Wave 2 (apolo-tf) somou a
# violação `agent_namespace_undeclared` sobre a mesma união — e essa violação
# devolve um discriminante mais forte que ORDEM, sobrevivendo à mesma
# corrupção testada aqui sem depender de posição relativa na saída (uma
# propriedade de apresentação que qualquer refatoração de UI do `status`
# poderia mudar sem tocar no parsing). `agent_namespace_undeclared` é gated
# por DECLARAÇÃO, não pela união: `zeus`, declarado em `agents: [zeus]`, NUNCA
# aparece como "não declarado" enquanto a lista for lida corretamente — só
# `apolo` (só-disco) aparece. Se `cfg.Agents`/`cfg.agents` for corrompido
# (lista descartada, fica vazia), o resolvedor cai para união vazia + disco:
# TODOS os namespaces em disco — `zeus` INCLUÍDO — viram "não declarados".
# Violação em massa citando `zeus` por nome é o sinal, e ele não pode ocorrer
# por acidente de disco (diferente da presença de um item na saída, `zeus`
# só entra na lista de violação se `agents:` genuinamente falhou ao lê-lo).
# Reproduzido ao vivo nos 3 CLIs (Go/Node — Python fora de escopo deste
# cenário, ver nota abaixo) antes de codar este gate: baseline `validate`
# emite só a violação de `apolo`; corrompido emite as de `apolo` E `zeus`.
#
#   - baseline: `trackfw validate` roda sobre `$S34_PROJECT` com os binários
#     LIMPOS (Go real, Node real) — a violação `agent namespace "zeus" ...
#     is not declared` está AUSENTE (zeus é declarado); a de `apolo` está
#     presente (não é o alvo deste cenário, só ruído esperado).
#   - detecção: Go e Node revertem o mesmo ponto de corrupção genérico já
#     documentado acima (`cfg.Agents = items` / `cfg.agents = items` — a
#     atribuição final a partir do valor já parseado) e provam, por
#     asserção POSITIVA, que a violação de `zeus` PASSA A APARECER na saída
#     corrompida — a ausência no baseline e a presença no corrompido são as
#     duas metades da prova de não-vacuidade (sem a metade "ausente no
#     baseline", uma implementação que sempre acusasse `zeus` passaria por
#     acidente).
#
# Corrompe a IMPLEMENTAÇÃO (o mesmo ponto genérico do braço original, nunca
# reintroduzido — ver RETARGET acima), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21/24/25/26/27/28/29/30/31/32/33. Não toca em pypi/ —
# o Python já está correto neste ponto (item fora do escopo negativo deste
# roadmap, decisão herdada do Cenário 34 original).
# ---------------------------------------------------------------------------

S34_PROJECT="$WORK/s34-config-unindented-agents-project"
mkdir -p "$S34_PROJECT/docs/adr" "$S34_PROJECT/docs/req"
mkdir -p "$S34_PROJECT/docs/roadmaps/zeus"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S34_PROJECT/docs/roadmaps/apolo"/{backlog,analyzing,wip,blocked,done,abandoned}
cat > "$S34_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
- zeus
EOF
write_roadmap_state_fixture "$S34_PROJECT/docs/roadmaps/zeus/wip/ROADMAP-zeus-wip.md" "wip" "zeus wip fixture (declarado em agents:)"
write_roadmap_state_fixture "$S34_PROJECT/docs/roadmaps/apolo/wip/ROADMAP-apolo-wip.md" "wip" "apolo wip fixture (só em disco)"

S34_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    2\n     backlog 0 · analyzing 0 · wip 2\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [zeus] WIP (1)\n    ROADMAP-zeus-wip.md\n  [apolo] WIP (1)\n    ROADMAP-apolo-wip.md\n\n────────────────────────────────────────\n'

s34_go_out=$(cd "$S34_PROJECT" && "$T27_GO_BIN" status)$'\n'
s34_node_out=$(cd "$S34_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" status)$'\n'
s34_python_out=$(cd "$S34_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw status)$'\n'

if [[ "$s34_go_out" == "$S34_EXPECTED" && "$s34_node_out" == "$S34_EXPECTED" && "$s34_python_out" == "$S34_EXPECTED" ]]; then
  echo "OK   [falsify/config-unindented-agents/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/config-unindented-agents/baseline-byte-identical-and-pinned]: esperava '$S34_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s34_go_out")" >&2
  echo "  node:   $(printf '%q' "$s34_node_out")" >&2
  echo "  python: $(printf '%q' "$s34_python_out")" >&2
  exit 1
fi

# Diagnóstico estático e discriminante: byte-idêntico à mensagem da regra
# `agent_namespace_undeclared` (validator.go:1157/index.js/config.py) —
# only ocorre se o resolvedor genuinamente perder `zeus` de `agents:`.
S34_ZEUS_UNDECLARED='agent namespace "zeus" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'
S34_APOLO_UNDECLARED='agent namespace "apolo" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'

# --- braço de baseline: com os binários LIMPOS, zeus (declarado) nunca
# aparece como não-declarado — só apolo (só-disco). Prova de não-vacuidade EM
# DUAS PONTAS: a ausência de zeus sozinha não distingue "validate rodou e a
# regra corretamente poupou zeus" de "validate não rodou nada" (binário
# quebrado, `cd` engolido pelo `; true`, saída vazia) — por isso a asserção
# de apolo PRESENTE é obrigatória aqui: prova que o validate rodou, varreu o
# disco e a regra disparou de verdade, só não para o namespace declarado.
s34_validate_go_out=$(cd "$S34_PROJECT" && "$T27_GO_BIN" validate 2>&1; true)
if ! grep -qF "$S34_APOLO_UNDECLARED" <<<"$s34_validate_go_out"; then
  echo "FAIL [falsify/config-unindented-agents/go/agent-namespace-undeclared-baseline]: apolo (só-disco) deveria estar 'não declarado' no ciclo LIMPO e não está — validate pode não ter rodado (cenário vácuo)" >&2
  echo "  output: $(printf '%q' "$s34_validate_go_out")" >&2
  exit 1
fi
if grep -qF "$S34_ZEUS_UNDECLARED" <<<"$s34_validate_go_out"; then
  echo "FAIL [falsify/config-unindented-agents/go/agent-namespace-undeclared-baseline]: zeus (declarado em agents:) já aparece como não-declarado no ciclo LIMPO — o cenário seria vácuo" >&2
  echo "  output: $(printf '%q' "$s34_validate_go_out")" >&2
  exit 1
fi
echo "OK   [falsify/config-unindented-agents/go/agent-namespace-undeclared-baseline]"

s34_validate_node_out=$(cd "$S34_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" validate 2>&1; true)
if ! grep -qF "$S34_APOLO_UNDECLARED" <<<"$s34_validate_node_out"; then
  echo "FAIL [falsify/config-unindented-agents/node/agent-namespace-undeclared-baseline]: apolo (só-disco) deveria estar 'não declarado' no ciclo LIMPO e não está — validate pode não ter rodado (cenário vácuo)" >&2
  echo "  output: $(printf '%q' "$s34_validate_node_out")" >&2
  exit 1
fi
if grep -qF "$S34_ZEUS_UNDECLARED" <<<"$s34_validate_node_out"; then
  echo "FAIL [falsify/config-unindented-agents/node/agent-namespace-undeclared-baseline]: zeus (declarado em agents:) já aparece como não-declarado no ciclo LIMPO — o cenário seria vácuo" >&2
  echo "  output: $(printf '%q' "$s34_validate_node_out")" >&2
  exit 1
fi
echo "OK   [falsify/config-unindented-agents/node/agent-namespace-undeclared-baseline]"

# --- braço de detecção: Go deixa de atribuir cfg.Agents a partir da lista --
# lida (RETARGET — ver comentário no topo do Cenário 34: isListItem/
# continuesOpenList não existem mais pós-yaml.v3)
T34C_GO_MOD="$WORK/s34-corrupt-go"
mkdir -p "$T34C_GO_MOD/cmd" "$T34C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T34C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T34C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T34C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T34C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T34C_GO_MOD/internal/config/config.go" \
  $'\t\t\tcfg.Agents = items\n' \
  $'\t\t\t_ = items\n' \
  "s34-go"

T34C_GO_BIN="$WORK/s34-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T34C_GO_BIN")"
build_go_or_fail "setup-s34-go-corrupt-build" "$T34C_GO_MOD" "$T34C_GO_BIN"

s34c_validate_go_out=$(cd "$S34_PROJECT" && "$T34C_GO_BIN" validate 2>&1; true)
if grep -qF "$S34_ZEUS_UNDECLARED" <<<"$s34c_validate_go_out"; then
  echo "OK   [falsify/config-unindented-agents/go-detects-list-discarded]"
else
  echo "FAIL [falsify/config-unindented-agents/go-detects-list-discarded]: cfg.Agents descartado, mas zeus não virou 'não declarado' na violação agent_namespace_undeclared — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s34c_validate_go_out")" >&2
  exit 1
fi

# --- braço de detecção: Node deixa de atribuir cfg.agents a partir da lista
# lida (RETARGET — mesmo motivo do braço Go acima)
T34C_N="$WORK/s34-corrupt-node"
setup_npm_tree "$T34C_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/config/index.js" "$T34C_N/npm/src/config/index.js" \
  $'    if (items) cfg.agents = items;\n' \
  $'    if (items) { /* no-op */ }\n' \
  "s34-node"

s34c_validate_node_out=$(cd "$S34_PROJECT" && node "$T34C_N/npm/bin/trackfw" validate 2>&1; true)
if grep -qF "$S34_ZEUS_UNDECLARED" <<<"$s34c_validate_node_out"; then
  echo "OK   [falsify/config-unindented-agents/node-detects-list-discarded]"
else
  echo "FAIL [falsify/config-unindented-agents/node-detects-list-discarded]: cfg.agents descartado, mas zeus não virou 'não declarado' na violação agent_namespace_undeclared — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s34c_validate_node_out")" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 35 — ROADMAP-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-
# config-dos-tres-clis (ML-2A): `agents:` em lista YAML INLINE cujo item
# contém vírgula DENTRO de aspas ("caso 8" do contrato — `["a, b", "c"]` são
# DOIS itens, não três) precisa ser preservado como um único nome de agente
# nos 3 CLIs.
#
# Nenhum cenário existente exercita este caso: o 34 cobre lista em BLOCO não
# indentada (defeito de outro parser, já corrigido antes desta Wave); os
# Cenários 30/31/33 usam `agents:` em bloco, sem flow-style. A tabela de 9
# casos do ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-
# dos-tres-clis foi verificada por teste unitário em cada CLI (ML-1A), mas
# nenhum gate de PARIDADE cross-CLI cobria o caso 8 especificamente — e é o
# único dos nove que uma separação ingênua por vírgula quebra.
#
# Fixture discriminante: agente real chamado `ka, tsu` — diretório em disco
# `docs/roadmaps/ka, tsu/` (vírgula+espaço é caractere válido em nome de
# diretório Unix) contendo um roadmap em wip/. `trackfw.yaml` configura
# `agents: ["ka, tsu", "obi"]` (flow-style, item citado com vírgula
# embutida). Escolhido deliberadamente para não ser vácuo por acidente: um
# parser que separa a vírgula ingenuamente (fora de aspas) produz os
# fragmentos "ka" e "tsu" como agentes SEPARADOS — nenhum dos dois casa com
# o diretório real `ka, tsu` no disco, então o roadmap correspondente
# desaparece INTEIRO da saída (não apenas o nome do agente muda formatação
# — a seção "⚙ WIP by Agent" fica vazia e o Inventory some a contagem).
# Uma fixture só com `[a, b]` (sem vírgula em item) não teria essa
# propriedade: qualquer separação, ingênua ou correta, produziria os mesmos
# dois nomes.
#
# Segunda camada de discriminação, decisiva contra reversão TOTAL do suporte
# inline (não só o ramo de aspas): `docs/roadmaps/zeta/` também existe no
# disco, com wip roadmap PRÓPRIO, mas `zeta` NÃO está na lista configurada.
# Com `agents:` corretamente parseado (inline, não-vazio), `resolveStateDirs`
# itera só os agentes configurados — `zeta` nunca entra na conta, igual ao
# papel de `zeus` no Cenário 34. Se alguém revertesse `isInlineList` por
# inteiro (não só o scanner de aspas), `agents: [...]` cairia no modo bloco,
# não encontraria `- item` nas linhas seguintes, produziria `cfg.Agents`
# vazio, e o CÓDIGO cairia no fallback de varrer `docs/roadmaps/*` — que
# encontraria `ka, tsu`, `obi` E `zeta`. Sem `zeta` no disco, esse fallback
# reproduziria por acidente a MESMA saída do parser correto (mesmo conjunto
# efetivo de agentes com wip), e os três braços de detecção abaixo
# morreriam no setup com "expected exactly 1 occurrence... got 0" — o
# defeito descrito em vault/notes/cenarios-de-falsificacao-quebram-em-
# refactor-do-alvo-2026-08-02.md, aqui por reversão total em vez de
# refactor. Com `zeta` presente e fora da lista, o fallback reintroduziria
# `[zeta] WIP (1)` na saída — divergência inequívoca do pinado.
#
#   - baseline: os 3 CLIs, contra o literal PINADO (capturado rodando os 3
#     CLIs reais contra a fixture), byte-idênticos — `[ka, tsu] WIP (1)`
#     aparece com o roadmap listado, Inventory Roadmaps total 1 (wip 1).
#   - detecção: originalmente revertia, em cada CLI, o ramo de detecção de
#     aspas de `splitTopLevelCommas`/`_split_top_level_commas` (`case r ==
#     '"' || r == '\''`/`ch === '"' || ch === "'"`/`ch in ('"', "'")`).
#
# RETARGET (ML-2A, 2026-08-02, Ártemis): rodar os 82 cenários herdados ANTES
# de editar (exigido pelo ML-2A) — depois de consertado o Cenário 34 (ver
# comentário no topo dele) — revelou o MESMO sintoma aqui: a Wave 1 desta
# REQ eliminou `splitTopLevelCommas`/`_split_top_level_commas` por inteiro
# nos 3 CLIs (`grep -rn splitTopLevelCommas internal/ npm/src/ pypi/` não
# encontra mais nada) — uma biblioteca YAML de verdade faz o parsing de
# sequência em fluxo (incluindo vírgula dentro de aspas) nativamente, sem
# precisar de scanner próprio. Não há mais um "ramo de detecção de aspas"
# para reverter seletivamente — é a MESMA classe de obsolescência do
# Cenário 34 (vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-
# alvo-2026-08-02.md), desta vez mascarada porque `set -euo pipefail` fazia
# o script abortar no Cenário 34 antes de alcançar este. Retargetado para o
# mesmo ponto genérico usado no Cenário 34 (`cfg.Agents = items` /
# `cfg.agents = items` / `cfg["agents"] = items` — a atribuição final a
# partir do valor já parseado pela biblioteca). A fixture (vírgula dentro de
# aspas) continua no cenário como vestígio do caso histórico original, mas o
# braço de detecção deixou de ser seletivo por aspas — prova "a chave
# `agents:` inline é lida", não mais "vírgula dentro de aspas
# especificamente" (que agora é responsabilidade estrutural da biblioteca,
# sem código próprio para corromper).
#
# Corrompe a IMPLEMENTAÇÃO (o ponto de atribuição final de `agents:`, não
# mais `splitTopLevelCommas` — ver RETARGET acima), nunca a asserção — mesmo
# padrão dos Cenários 14/16/17/20/21/24/25/26/27/28/29/30/31/32/33/34. Não
# amplia o suporte YAML (mapas inline, listas aninhadas) — fora de escopo,
# registrado no ADR.
#
# RETARGET 2 (ML-1A, REQ-2026-08-29, apolo-tf): o resolvedor canônico
# (resolveAgentNamespaces) passou a devolver a UNIÃO entre `agents:` e o
# disco. Isso desarma a asserção "presença/ausência de [zeta]" da mesma forma
# que no Cenário 34 (zeta agora aparece nos dois braços) — E piora aqui
# especificamente: o diretório físico `docs/roadmaps/ka, tsu/` já existe em
# disco com esse nome LITERAL, então a união encontra "ka, tsu" via
# varredura de disco mesmo que a atribuição de `cfg.Agents`/`cfg.agents`
# seja corrompida por completo — a fixture original ficou incapaz de provar
# qualquer coisa por presença/ausência. A propriedade usada neste retarget
# (ML-1A) foi ORDEM — ver RETARGET 3 abaixo para o motivo de ter sido
# substituída.
#
# RETARGET 3 (ML-3A, REQ-2026-08-29, artemis-tf): a violação
# `agent_namespace_undeclared` (Wave 2, apolo-tf) devolve um sinal mais forte
# que ORDEM — a mesma razão do Cenário 34, ver o comentário RETARGET 3 lá
# para a justificativa completa. Aqui a fixture (`agents: ["obi", "ka,
# tsu"]`, `zeta` só-disco) já é exatamente o desenho que a violação precisa:
# com o parser correto, `obi` e `ka, tsu` são DECLARADOS — nunca aparecem
# como "não declarados"; só `zeta` aparece. Se `cfg.Agents`/`cfg.agents`/
# `cfg["agents"]` for corrompido (lista descartada), os três — `obi`, `ka,
# tsu` E `zeta` — viram "não declarados". A violação citando `ka, tsu` por
# nome (com a vírgula preservada, vinda do nome do diretório em disco, não
# do parsing de `agents:` — a união sempre soletra o nome como está no
# disco) prova que o parser perdeu a declaração, sem depender de posição na
# saída. Cobre os 3 CLIs (diferente do Cenário 34, que exclui Python por
# decisão herdada — aqui o Python já tinha arm próprio e o mantém).
# ---------------------------------------------------------------------------

S35_PROJECT="$WORK/s35-config-inline-comma-in-quotes-project"
mkdir -p "$S35_PROJECT/docs/adr" "$S35_PROJECT/docs/req"
mkdir -p "$S35_PROJECT/docs/roadmaps/ka, tsu"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S35_PROJECT/docs/roadmaps/obi"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S35_PROJECT/docs/roadmaps/zeta"/{backlog,analyzing,wip,blocked,done,abandoned}
cat > "$S35_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents: ["obi", "ka, tsu"]
EOF
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/ka, tsu/wip/ROADMAP-ka-tsu-wip.md" "wip" "ka tsu wip fixture (caso 8)"
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/obi/wip/ROADMAP-obi-wip.md" "wip" "obi wip fixture (declarado primeiro, discrimina ordem)"
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/zeta/wip/ROADMAP-zeta-wip.md" "wip" "zeta wip fixture (fora da lista configurada — só disco, union)"

S35_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    3\n     backlog 0 · analyzing 0 · wip 3\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [obi] WIP (1)\n    ROADMAP-obi-wip.md\n  [ka, tsu] WIP (1)\n    ROADMAP-ka-tsu-wip.md\n  [zeta] WIP (1)\n    ROADMAP-zeta-wip.md\n\n────────────────────────────────────────\n'

s35_go_out=$(cd "$S35_PROJECT" && "$T27_GO_BIN" status)$'\n'
s35_node_out=$(cd "$S35_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" status)$'\n'
s35_python_out=$(cd "$S35_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw status)$'\n'

if [[ "$s35_go_out" == "$S35_EXPECTED" && "$s35_node_out" == "$S35_EXPECTED" && "$s35_python_out" == "$S35_EXPECTED" ]]; then
  echo "OK   [falsify/config-inline-comma-in-quotes/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/baseline-byte-identical-and-pinned]: esperava '$S35_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s35_go_out")" >&2
  echo "  node:   $(printf '%q' "$s35_node_out")" >&2
  echo "  python: $(printf '%q' "$s35_python_out")" >&2
  exit 1
fi

# Diagnóstico estático e discriminante: byte-idêntico à mensagem da regra
# `agent_namespace_undeclared`, com a vírgula do nome do diretório em disco
# preservada — vem da UNIÃO (varredura de disco), não do parsing de `agents:`.
S35_KATSU_UNDECLARED='agent namespace "ka, tsu" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'
S35_OBI_UNDECLARED='agent namespace "obi" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'
S35_ZETA_UNDECLARED='agent namespace "zeta" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'

# --- braço de baseline: com os binários LIMPOS, `obi` e `ka, tsu`
# (declarados) nunca aparecem como não-declarados — só `zeta` (só-disco).
# Prova de não-vacuidade para os dois alvos, nos 3 CLIs: a ausência de
# obi/"ka, tsu" sozinha não distingue "validate rodou e a regra corretamente
# poupou os declarados" de "validate não rodou nada" (saída vazia engolida
# pelo `; true`) — a asserção de zeta PRESENTE é obrigatória, prova que o
# validate rodou, varreu o disco e a regra disparou de verdade.
s35_validate_go_out=$(cd "$S35_PROJECT" && "$T27_GO_BIN" validate 2>&1; true)
s35_validate_node_out=$(cd "$S35_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" validate 2>&1; true)
s35_validate_python_out=$(cd "$S35_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate 2>&1; true)
for pair in "go:$s35_validate_go_out" "node:$s35_validate_node_out" "python:$s35_validate_python_out"; do
  runtime="${pair%%:*}"
  out="${pair#*:}"
  if ! grep -qF "$S35_ZETA_UNDECLARED" <<<"$out"; then
    echo "FAIL [falsify/config-inline-comma-in-quotes/$runtime/agent-namespace-undeclared-baseline]: zeta (só-disco) deveria estar 'não declarado' no ciclo LIMPO e não está — validate pode não ter rodado (cenário vácuo)" >&2
    echo "  output: $(printf '%q' "$out")" >&2
    exit 1
  fi
  if grep -qF "$S35_KATSU_UNDECLARED" <<<"$out" || grep -qF "$S35_OBI_UNDECLARED" <<<"$out"; then
    echo "FAIL [falsify/config-inline-comma-in-quotes/$runtime/agent-namespace-undeclared-baseline]: obi ou 'ka, tsu' (declarados em agents:) já aparecem como não-declarados no ciclo LIMPO — o cenário seria vácuo" >&2
    echo "  output: $(printf '%q' "$out")" >&2
    exit 1
  fi
  echo "OK   [falsify/config-inline-comma-in-quotes/$runtime/agent-namespace-undeclared-baseline]"
done

# --- braço de detecção: Go deixa de atribuir cfg.Agents a partir da lista --
# lida (RETARGET — ver comentário no topo do Cenário 35: splitTopLevelCommas
# não existe mais pós-yaml.v3; mesmo ponto usado no Cenário 34)
T35C_GO_MOD="$WORK/s35-corrupt-go"
mkdir -p "$T35C_GO_MOD/cmd" "$T35C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T35C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T35C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T35C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T35C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T35C_GO_MOD/internal/config/config.go" \
  $'\t\t\tcfg.Agents = items\n' \
  $'\t\t\t_ = items\n' \
  "s35-go"

T35C_GO_BIN="$WORK/s35-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T35C_GO_BIN")"
build_go_or_fail "setup-s35-go-corrupt-build" "$T35C_GO_MOD" "$T35C_GO_BIN"

s35c_validate_go_out=$(cd "$S35_PROJECT" && "$T35C_GO_BIN" validate 2>&1; true)
if grep -qF "$S35_KATSU_UNDECLARED" <<<"$s35c_validate_go_out" && grep -qF "$S35_OBI_UNDECLARED" <<<"$s35c_validate_go_out"; then
  echo "OK   [falsify/config-inline-comma-in-quotes/go-detects-agents-discarded]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/go-detects-agents-discarded]: cfg.Agents descartado, mas obi e/ou 'ka, tsu' não viraram 'não declarados' na violação agent_namespace_undeclared — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s35c_validate_go_out")" >&2
  exit 1
fi

# --- braço de detecção: Node deixa de atribuir cfg.agents a partir da
# lista lida (RETARGET — mesmo motivo do braço Go acima)
T35C_N="$WORK/s35-corrupt-node"
setup_npm_tree "$T35C_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/config/index.js" "$T35C_N/npm/src/config/index.js" \
  $'    if (items) cfg.agents = items;\n' \
  $'    if (items) { /* no-op */ }\n' \
  "s35-node"

s35c_validate_node_out=$(cd "$S35_PROJECT" && node "$T35C_N/npm/bin/trackfw" validate 2>&1; true)
if grep -qF "$S35_KATSU_UNDECLARED" <<<"$s35c_validate_node_out" && grep -qF "$S35_OBI_UNDECLARED" <<<"$s35c_validate_node_out"; then
  echo "OK   [falsify/config-inline-comma-in-quotes/node-detects-agents-discarded]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/node-detects-agents-discarded]: cfg.agents descartado, mas obi e/ou 'ka, tsu' não viraram 'não declarados' na violação agent_namespace_undeclared — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s35c_validate_node_out")" >&2
  exit 1
fi

# --- braço de detecção: Python deixa de atribuir cfg["agents"] a partir da
# lista lida (RETARGET — mesmo motivo dos braços Go/Node acima)
T35C_P="$WORK/s35-corrupt-python"
mkdir -p "$T35C_P"
cp -r "$ROOT_DIR/pypi" "$T35C_P/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$T35C_P/pypi/trackfw/config.py" \
  $'    if "agents" in m:\n        items = _string_list(m["agents"])\n        if items is not None:\n            cfg["agents"] = items\n' \
  $'    if "agents" in m:\n        items = _string_list(m["agents"])\n' \
  "s35-python"

s35c_validate_python_out=$(cd "$S35_PROJECT" && env PYTHONPATH="$T35C_P/pypi" python3 -m trackfw validate 2>&1; true)
if grep -qF "$S35_KATSU_UNDECLARED" <<<"$s35c_validate_python_out" && grep -qF "$S35_OBI_UNDECLARED" <<<"$s35c_validate_python_out"; then
  echo "OK   [falsify/config-inline-comma-in-quotes/python-detects-agents-discarded]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/python-detects-agents-discarded]: cfg[\"agents\"] descartado, mas obi e/ou 'ka, tsu' não viraram 'não declarados' na violação agent_namespace_undeclared — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s35c_validate_python_out")" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 36 — ML-2A do ROADMAP-2026-08-02-substituir-os-parsers-artesanais-
# de-config-por-biblioteca-yaml-nos-tres-clis: fidelidade textual de escalar
# YAML (AC3 do roadmap) — a normalização para string na fronteira, feita em
# normalizeNode (Go) / normalizeNode (Node) / _normalize_node (Python), não
# pode regredir para o valor JÁ TIPADO que cada biblioteca resolveria por
# padrão (int/bool/date), ou os 3 CLIs voltam a divergir por schema — a
# MESMA divergência de 3 vias medida no ML-0A (ver Wave 0 do roadmap):
#
#   entrada            Go (typed)      Node (typed)    Python (typed)
#   "010"              int 8           number 10       int 8
#   "2026-08-02"       time.Time       string (nao      date
#                                       converte)
#   "yes"              string (nao     string (nao      bool True
#                       coage)          coage)
#
# Os TRÊS valores do contrato (octal, data nua, "yes") são necessários na
# MESMA fixture — cada um cobre um CLI diferente, e nenhum par prova os 3:
#   - octal ("010") é o ÚNICO dos três que produz tipo não-string em Node
#     (number). Sem ele, uma regressão de normalização em Node passaria
#     despercebida pela fixture inteira (nem a data nem "yes" mudam de tipo
#     em Node — ver tabela acima).
#   - data nua ("2026-08-02") produz tipo não-string em Go E Python
#     (time.Time / date) — mas em Node o valor já chega como string mesmo
#     sem normalização (Node não tem resolver de data), então sozinha ela
#     não prova nada sobre Node.
#   - "yes" produz tipo não-string SÓ em Python (bool True, resolução YAML
#     1.1) — Go e `yaml` (Node) seguem o núcleo YAML 1.2 e não coagem
#     yes/no para booleano, então "yes" chega como string nos dois mesmo
#     sem normalização.
#
# Cada CLI usa o guard de tipo já existente (stringVal: `v.(string)` em Go,
# `typeof v === 'string'` em Node, `isinstance(v, str)` em Python) para só
# aceitar escalares vindos como string. Quando a normalização é removida (a
# corrupção abaixo faz normalizeNode devolver o valor TIPADO em vez do texto
# bruto), um valor que se tipifica como não-string faz o guard reprovar
# SILENCIOSAMENTE — a chave é descartada e o default do campo prevalece, sem
# erro. É exatamente esse efeito observável (queda para o default) que os
# três braços de detecção abaixo verificam.
#
# NÃO usa wip_limit/governance_mode/lenient_until (os campos citados
# literalmente no ADR): esses três são lidos, para o propósito de
# `trackfw validate`, por um leitor artesanal linha-a-linha SEPARADO
# (readWIPConfig/readGovernanceMode em internal/validator/validator.go, e os
# gêmeos em npm/src/validator/index.js e pypi/trackfw/validator.py) que a
# Wave 1 desta REQ NÃO tocou — ProjectConfig.WipLimit/GovernanceMode/
# LenientUntil (o caminho que passa pela biblioteca YAML) fica sombreado e
# nunca chega ao `validate` real. Uma fixture nessas chaves seria vácua para
# este cenário porque não exercitaria normalizeNode nenhuma. Ver achado
# registrado em vault/notes/config-legacy-line-reader-sombreia-yaml-lib-no-
# validate-2026-08-02.md. Em vez disso, a fixture usa `roadmap_dir`,
# `req_dir` e `adr_dirs` — os três são lidos via config.Load() (o caminho
# real da biblioteca YAML) e aparecem, cada um, no bloco Inventory de
# `trackfw status`, dando um sinal visível e determinístico por campo.
#
# Fixture: `roadmap_dir: 010`, `req_dir: 2026-08-02`, `adr_dirs: [yes]` —
# cada um aponta para um diretório NO DISCO com nome literal igual ao valor
# bruto ("010/", "2026-08-02/", "yes/"), cada um com exatamente 1 item
# (roadmap wip, REQ, ADR). Os caminhos DEFAULT (docs/roadmaps, docs/req,
# docs/adr) também têm conteúdo — mas em quantidade DIFERENTE (2 roadmaps,
# 2 REQs, 2 ADRs) — para que, se a corrupção fizer o parser cair no default,
# a divergência apareça como um número POSITIVO diferente do pinado, nunca
# como zero-por-coincidência (mesma lição do Cenário 35 e de
# vault/notes/falsificacao-fixture-vacua-contra-reversao-total-vs-parcial-
# 2026-08-02.md).
#
# Medido empiricamente (não apenas deduzido) contra os binários reais antes
# de fechar o cenário: a matriz de divergência por CLI corrompido é
#   - Go corrompido:     REQs 1→2, Roadmaps 1→2 (backlog 0→1, wip 1→1);
#                        ADRs PERMANECE 1 (Go não diverge em "yes")
#   - Node corrompido:   Roadmaps 1→2 (backlog 0→1); ADRs e REQs PERMANECEM
#                        1 (Node não diverge em data nua nem em "yes")
#   - Python corrompido: ADRs 1→0 (adr_dirs vira lista vazia, não cai no
#     default — stringList filtra o item não-string e devolve [] "presente
#     e vazio", contrato herdado do fix de lista inline), REQs 1→2,
#     Roadmaps 1→2
# Isso prova, por CLI, exatamente a matriz de discriminação do contrato:
# Node só diverge por causa do octal; Go e Python divergem por causa da
# data; só Python diverge por causa do "yes".
#
# Corrompe a IMPLEMENTAÇÃO (o branch de escalar de normalizeNode/
# _normalize_node), nunca a asserção — mesmo padrão dos cenários anteriores.
# ---------------------------------------------------------------------------

S36_PROJECT="$WORK/s36-config-schema-discriminant-project"
mkdir -p "$S36_PROJECT/docs/adr" "$S36_PROJECT/docs/req"
mkdir -p "$S36_PROJECT/docs/roadmaps"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S36_PROJECT/010"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S36_PROJECT/2026-08-02"
mkdir -p "$S36_PROJECT/yes"
cat > "$S36_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
roadmap_dir: 010
req_dir: 2026-08-02
adr_dirs:
  - yes
EOF
write_roadmap_state_fixture "$S36_PROJECT/010/wip/ROADMAP-s36-custom-wip.md" "wip" "s36 custom wip fixture"
write_req_status_fixture "$S36_PROJECT/2026-08-02/REQ-s36-custom.md" "Open" "s36 custom req fixture"
write_adr_status_fixture "$S36_PROJECT/yes/ADR-s36-custom.md" "Accepted"

# Conteúdo diferente (em quantidade) nos caminhos DEFAULT — ver comentário
# acima sobre por que isso é necessário para não mascarar a corrupção.
write_roadmap_state_fixture "$S36_PROJECT/docs/roadmaps/wip/ROADMAP-s36-default-wip.md" "wip" "s36 default wip fixture"
write_roadmap_state_fixture "$S36_PROJECT/docs/roadmaps/backlog/ROADMAP-s36-default-backlog.md" "backlog" "s36 default backlog fixture"
write_req_status_fixture "$S36_PROJECT/docs/req/REQ-s36-default-1.md" "Open" "s36 default req 1"
write_req_status_fixture "$S36_PROJECT/docs/req/REQ-s36-default-2.md" "Open" "s36 default req 2"
write_adr_status_fixture "$S36_PROJECT/docs/adr/ADR-s36-default-1.md" "Accepted"
write_adr_status_fixture "$S36_PROJECT/docs/adr/ADR-s36-default-2.md" "Accepted"

S36_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        1\n   REQs        1  (1 Open · 0 Done · 0 Closed)\n   Roadmaps    1\n     backlog 0 · analyzing 0 · wip 1\n     blocked 0 · done 0 · abandoned 0\n\n🔄 WIP (1)\n   ROADMAP-s36-custom-wip.md\n\n❌ Blocked (0)\n\n✅ Done (last 5)\n\n────────────────────────────────────────\n'

s36_go_out=$(cd "$S36_PROJECT" && "$T27_GO_BIN" status)$'\n'
s36_node_out=$(cd "$S36_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" status)$'\n'
s36_python_out=$(cd "$S36_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw status)$'\n'

if [[ "$s36_go_out" == "$S36_EXPECTED" && "$s36_node_out" == "$S36_EXPECTED" && "$s36_python_out" == "$S36_EXPECTED" ]]; then
  echo "OK   [falsify/config-schema-discriminant/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/config-schema-discriminant/baseline-byte-identical-and-pinned]: esperava '$S36_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s36_go_out")" >&2
  echo "  node:   $(printf '%q' "$s36_node_out")" >&2
  echo "  python: $(printf '%q' "$s36_python_out")" >&2
  exit 1
fi

# --- braço de detecção: Go devolve o valor TIPADO em vez do texto bruto ----
# (octal "010" -> int 8: roadmap_dir cai no default; data nua -> time.Time:
# req_dir cai no default; "yes" -> string "yes": adr_dirs NÃO diverge — Go
# não coage yes/no para booleano)
T36C_GO_MOD="$WORK/s36-corrupt-go"
mkdir -p "$T36C_GO_MOD/cmd" "$T36C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T36C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T36C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T36C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T36C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T36C_GO_MOD/internal/config/config.go" \
  $'\tcase yaml.ScalarNode:\n\t\treturn n.Value\n' \
  $'\tcase yaml.ScalarNode:\n\t\tvar typed interface{}\n\t\tn.Decode(&typed)\n\t\treturn typed\n' \
  "s36-go"

T36C_GO_BIN="$WORK/s36-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T36C_GO_BIN")"
build_go_or_fail "setup-s36-go-corrupt-build" "$T36C_GO_MOD" "$T36C_GO_BIN"

s36c_go_out=$(cd "$S36_PROJECT" && "$T36C_GO_BIN" status)$'\n'
if [[ "$s36c_go_out" == "$S36_EXPECTED" ]]; then
  echo "FAIL [falsify/config-schema-discriminant/go-detects-typed-scalar-regression]: normalizeNode revertido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "ADRs        1" <<<"$s36c_go_out" && grep -qF "REQs        2" <<<"$s36c_go_out" && grep -qF "backlog 1" <<<"$s36c_go_out"; then
  echo "OK   [falsify/config-schema-discriminant/go-detects-typed-scalar-regression]"
else
  echo "FAIL [falsify/config-schema-discriminant/go-detects-typed-scalar-regression]: saída corrompida diverge do pinado, mas não no padrão esperado (ADRs deveria permanecer 1; REQs e Roadmaps deveriam cair para o default) — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s36c_go_out")" >&2
  exit 1
fi

# --- braço de detecção: Node devolve o valor TIPADO em vez do texto bruto --
# (octal "010" -> number 10: roadmap_dir cai no default; data nua e "yes"
# chegam como string mesmo tipadas em Node -> NÃO divergem)
T36C_N="$WORK/s36-corrupt-node"
setup_npm_tree "$T36C_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/config/index.js" "$T36C_N/npm/src/config/index.js" \
  $'    return n.source != null ? n.source : (n.value == null ? \'\' : String(n.value));\n' \
  $'    return n.value;\n' \
  "s36-node"

s36c_node_out=$(cd "$S36_PROJECT" && node "$T36C_N/npm/bin/trackfw" status)$'\n'
if [[ "$s36c_node_out" == "$S36_EXPECTED" ]]; then
  echo "FAIL [falsify/config-schema-discriminant/node-detects-typed-scalar-regression]: normalizeNode revertido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "ADRs        1" <<<"$s36c_node_out" && grep -qF "REQs        1" <<<"$s36c_node_out" && grep -qF "backlog 1" <<<"$s36c_node_out"; then
  echo "OK   [falsify/config-schema-discriminant/node-detects-typed-scalar-regression]"
else
  echo "FAIL [falsify/config-schema-discriminant/node-detects-typed-scalar-regression]: saída corrompida diverge do pinado, mas não no padrão esperado (ADRs e REQs deveriam permanecer inalterados; só Roadmaps deveria cair para o default) — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s36c_node_out")" >&2
  exit 1
fi

# --- braço de detecção: Python devolve o valor CONSTRUÍDO (via
# SafeConstructor) em vez do texto bruto do nó — único dos 3 em que "yes"
# também diverge (bool True -> filtrado de adr_dirs -> lista vazia, não
# default)
T36C_P="$WORK/s36-corrupt-python"
mkdir -p "$T36C_P"
cp -r "$ROOT_DIR/pypi" "$T36C_P/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$T36C_P/pypi/trackfw/config.py" \
  $'    if isinstance(node, yaml.ScalarNode):\n        return node.value\n' \
  $'    if isinstance(node, yaml.ScalarNode):\n        return yaml.constructor.SafeConstructor().construct_object(node, deep=True)\n' \
  "s36-python"

s36c_python_out=$(cd "$S36_PROJECT" && env PYTHONPATH="$T36C_P/pypi" python3 -m trackfw status)$'\n'
if [[ "$s36c_python_out" == "$S36_EXPECTED" ]]; then
  echo "FAIL [falsify/config-schema-discriminant/python-detects-typed-scalar-regression]: _normalize_node revertido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "ADRs        0" <<<"$s36c_python_out" && grep -qF "REQs        2" <<<"$s36c_python_out" && grep -qF "backlog 1" <<<"$s36c_python_out"; then
  echo "OK   [falsify/config-schema-discriminant/python-detects-typed-scalar-regression]"
else
  echo "FAIL [falsify/config-schema-discriminant/python-detects-typed-scalar-regression]: saída corrompida diverge do pinado, mas não no padrão esperado (ADRs deveria cair para 0 — adr_dirs vazio, não default; REQs e Roadmaps deveriam cair para o default) — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s36c_python_out")" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 37 — ML-2A: caminho de erro de config malformada — os 3 CLIs
# imprimem a MESMA mensagem em stderr (MalformedConfigMessage /
# MALFORMED_CONFIG_MESSAGE) e saem com o MESMO exit code (1) quando
# trackfw.yaml existe mas não é YAML válido. Comportamento NOVO desta REQ
# (ML-1B, addendum ao ML-1A) — nada em CI garantia isso antes deste cenário;
# as suítes unitárias por CLI provam a mensagem isoladamente, mas nenhum
# gate cruzava os 3 binários reais contra a MESMA fixture malformada.
#
# Fixture: sequência de fluxo (`[...]`) aberta e nunca fechada — inválida
# nas 3 bibliotecas (confirmado empiricamente: gopkg.in/yaml.v3, `yaml` 2.x
# e PyYAML rejeitam as 3, cada uma com sua própria mensagem nativa
# diferente — exatamente por isso a mensagem trackfw é estática, não
# derivada do erro da biblioteca, ver comentário de MalformedConfigMessage
# em internal/config/config.go).
#
# Braço de detecção: só Go (a checagem de erro de sintaxe do Go, além de ser
# a mais recente/complexa das 3 — soma o probe de yaml.Unmarshal com
# hasMultipleDocuments — é também o ponto onde uma regressão de "parei de
# tratar erro de sintaxe como fatal" é mais fácil de introduzir sem querer
# ao mexer no probe). Não repete o braço nos 3 CLIs: o mecanismo (parse ->
# erro -> flag "malformed" -> stderr fatal + exit 1) é estruturalmente
# idêntico nos 3 (mesmo comentário-fonte, ver MALFORMED_CONFIG_MESSAGE nos 3
# arquivos), e o objetivo deste cenário é provar que o gate cruzado existe e
# pega uma regressão — não re-provar a suíte unitária de cada CLI.
# ---------------------------------------------------------------------------

S37_PROJECT="$WORK/s37-config-malformed-project"
mkdir -p "$S37_PROJECT/docs/adr" "$S37_PROJECT/docs/req" \
  "$S37_PROJECT/docs/roadmaps"/{backlog,analyzing,wip,blocked,done,abandoned}
printf 'agents: [a, b\ngovernance_mode: strict\n' > "$S37_PROJECT/trackfw.yaml"

S37_EXPECTED_STDERR='trackfw: erro ao carregar "trackfw.yaml": YAML malformado. Corrija a sintaxe do arquivo antes de continuar.'

set +e
s37_go_out=$(cd "$S37_PROJECT" && "$T27_GO_BIN" status 2>&1)
s37_go_status=$?
s37_node_out=$(cd "$S37_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" status 2>&1)
s37_node_status=$?
s37_python_out=$(cd "$S37_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw status 2>&1)
s37_python_status=$?
set -e

if [[ "$s37_go_status" -eq 1 && "$s37_node_status" -eq 1 && "$s37_python_status" -eq 1 \
      && "$s37_go_out" == "$S37_EXPECTED_STDERR" && "$s37_node_out" == "$S37_EXPECTED_STDERR" \
      && "$s37_python_out" == "$S37_EXPECTED_STDERR" ]]; then
  echo "OK   [falsify/config-malformed-error-path/baseline-byte-identical-exit-1-3-clis]"
else
  echo "FAIL [falsify/config-malformed-error-path/baseline-byte-identical-exit-1-3-clis]: esperava stderr '$S37_EXPECTED_STDERR' e exit 1 nos 3 CLIs" >&2
  echo "  go:     status=$s37_go_status out=$(printf '%q' "$s37_go_out")" >&2
  echo "  node:   status=$s37_node_status out=$(printf '%q' "$s37_node_out")" >&2
  echo "  python: status=$s37_python_status out=$(printf '%q' "$s37_python_out")" >&2
  exit 1
fi

# --- braço de detecção: Go deixa de tratar erro de sintaxe/multi-documento
# como fatal (probe sempre "ok") ------------------------------------------
T37C_GO_MOD="$WORK/s37-corrupt-go"
mkdir -p "$T37C_GO_MOD/cmd" "$T37C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T37C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T37C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T37C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T37C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T37C_GO_MOD/internal/config/config.go" \
  $'\t\tif err := yaml.Unmarshal(data, &probe); err != nil || hasMultipleDocuments(data) {\n' \
  $'\t\tif err := yaml.Unmarshal(data, &probe); err == nil && false {\n' \
  "s37-go"

T37C_GO_BIN="$WORK/s37-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T37C_GO_BIN")"
build_go_or_fail "setup-s37-go-corrupt-build" "$T37C_GO_MOD" "$T37C_GO_BIN"

set +e
s37c_go_out=$(cd "$S37_PROJECT" && "$T37C_GO_BIN" status 2>&1)
s37c_go_status=$?
set -e
if [[ "$s37c_go_status" -eq 1 && "$s37c_go_out" == "$S37_EXPECTED_STDERR" ]]; then
  echo "FAIL [falsify/config-malformed-error-path/go-detects-fatal-check-removed]: checagem de erro de sintaxe revertida mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if [[ "$s37c_go_status" -eq 0 ]]; then
  echo "OK   [falsify/config-malformed-error-path/go-detects-fatal-check-removed]"
else
  echo "FAIL [falsify/config-malformed-error-path/go-detects-fatal-check-removed]: saída corrompida diverge do pinado, mas o exit não caiu para 0 — diagnóstico pelo motivo errado" >&2
  echo "  status=$s37c_go_status output: $(printf '%q' "$s37c_go_out")" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 38 — wipConfigFrom (e equivalentes _wip_config_from/JS) volta a ler
# trackfw.yaml artesanalmente em vez de consumir o cfg já normalizado por
# config.Load() — regressão descoberta na auditoria do ML-3A (elimina os
# leitores readWIPConfig/readGovernanceMode em 74d70ee). Nenhum cenário deste
# harness fixava essa regressão: o teste unitário existente
# (TestValidateWIPLimit_Global_HighLimit) usa `wip_limit: 3` SEM aspas — valor
# que um leitor artesanal (Sscanf %d / parseInt / int()) lê corretamente,
# igual à biblioteca YAML. Sem aspas o cenário é vácuo: os dois caminhos
# concordam e nenhuma regressão é detectada.
#
# Fixture discriminante: `wip_limit: "3"` — COM aspas. Um leitor artesanal
# falha ao interpretar o valor citado (Sscanf/parseInt/int() encontram `"3"`,
# não `3`) e cai no default 1; a biblioteca YAML resolve o escalar tipado
# normalmente para 3.
#
#   - baseline: projeto com wip_limit: "3" (citado) e 4 roadmaps em wip/ → os
#     3 CLIs devem reportar o warning "4 roadmaps in wip/ (limit: 3) —
#     consider focusing" (cfg.WipLimit == 3, valor lido pela biblioteca).
#   - detecção: reintroduz, em cada CLI, exatamente o padrão do
#     readWIPConfig/wipConfigFrom eliminado por 74d70ee — releitura artesanal
#     de trackfw.yaml em vez de consumir o cfg já carregado — e prova que a
#     saída volta a "(limit: 1)" nos 3 CLIs.
#
# Corrompe a IMPLEMENTAÇÃO (wipConfigFrom/_wip_config_from e equivalente
# Node), nunca a asserção — mesmo padrão dos cenários anteriores.
# ---------------------------------------------------------------------------

write_wip_roadmap_fixture() {
  local dest=$1 title=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
# Roadmap: $title

REQ: REQ-001

## Acceptance Criteria
- [ ] item
EOF
}

S38_PROJECT="$WORK/s38-wip-limit-quoted-project"
scaffold_adr_req_project "$S38_PROJECT"
cat > "$S38_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
wip_limit: "3"
wip_by_squad: false
EOF
for n in 1 2 3 4; do
  write_wip_roadmap_fixture "$S38_PROJECT/docs/roadmaps/wip/ROADMAP-wip-$n.md" "wip fixture $n"
done

S38_EXPECTED_WARNING='4 roadmaps in wip/ (limit: 3) — consider focusing'
S38_REGRESSED_WARNING='4 roadmaps in wip/ (limit: 1) — consider focusing'

# --- prova positiva: os 3 CLIs, com a fixture citada -------------------------
set +e
s38_go_out=$(cd "$S38_PROJECT" && "$T27_GO_BIN" validate 2>&1)
s38_node_out=$(cd "$S38_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" validate 2>&1)
s38_python_out=$(cd "$S38_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate 2>&1)
set -e

if grep -qF "$S38_EXPECTED_WARNING" <<<"$s38_go_out" \
    && grep -qF "$S38_EXPECTED_WARNING" <<<"$s38_node_out" \
    && grep -qF "$S38_EXPECTED_WARNING" <<<"$s38_python_out"; then
  echo "OK   [falsify/wip-limit-quoted/baseline-3-clis]"
else
  echo "FAIL [falsify/wip-limit-quoted/baseline-3-clis]: esperava '$S38_EXPECTED_WARNING' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s38_go_out")" >&2
  echo "  node:   $(printf '%q' "$s38_node_out")" >&2
  echo "  python: $(printf '%q' "$s38_python_out")" >&2
  exit 1
fi

# --- Go: prova de detecção (wipConfigFrom volta a ler trackfw.yaml direto) --
GO_S38_OLD=$'func wipConfigFrom(cfg config.ProjectConfig) WIPConfig {\n\treturn WIPConfig{Limit: cfg.WipLimit, BySquad: cfg.WipBySquad}\n}'
GO_S38_NEW=$'func wipConfigFrom(cfg config.ProjectConfig) WIPConfig {\n\twc := WIPConfig{Limit: 1, BySquad: cfg.WipBySquad}\n\tcontent, err := os.ReadFile("trackfw.yaml")\n\tif err != nil {\n\t\treturn wc\n\t}\n\tfor _, line := range strings.Split(string(content), "\\n") {\n\t\tline = strings.TrimSpace(line)\n\t\tif strings.HasPrefix(line, "wip_limit:") {\n\t\t\tval := strings.TrimSpace(strings.TrimPrefix(line, "wip_limit:"))\n\t\t\tfields := strings.Fields(val)\n\t\t\tif len(fields) > 0 {\n\t\t\t\tvar n int\n\t\t\t\tif _, err := fmt.Sscanf(fields[0], "%d", &n); err == nil && n > 0 {\n\t\t\t\t\twc.Limit = n\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\treturn wc\n}'

T38C_GO_MOD="$WORK/s38-corrupt-go"
mkdir -p "$T38C_GO_MOD/cmd" "$T38C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T38C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T38C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T38C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T38C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T38C_GO_MOD/internal/validator/validator.go" \
  "$GO_S38_OLD" "$GO_S38_NEW" "s38-go"

T38C_GO_BIN="$WORK/s38-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T38C_GO_BIN")"
build_go_or_fail "setup-s38-go-corrupt-build" "$T38C_GO_MOD" "$T38C_GO_BIN"

set +e
s38c_go_out=$(cd "$S38_PROJECT" && "$T38C_GO_BIN" validate 2>&1)
set -e
if grep -qF "$S38_REGRESSED_WARNING" <<<"$s38c_go_out" && ! grep -qF "$S38_EXPECTED_WARNING" <<<"$s38c_go_out"; then
  echo "OK   [falsify/wip-limit-quoted/go-detects-artisanal-reader-reintroduced]"
else
  echo "FAIL [falsify/wip-limit-quoted/go-detects-artisanal-reader-reintroduced]: leitor artesanal reintroduzido mas a saída não voltou a '(limit: 1)' — checagem vácua" >&2
  echo "  output: $(printf '%q' "$s38c_go_out")" >&2
  exit 1
fi

# --- Node: prova de detecção -------------------------------------------------
NODE_S38_OLD=$'function wipConfigFrom(cfg) {\n  return { limit: cfg.wipLimit > 0 ? cfg.wipLimit : 1, bySquad: !!cfg.wipBySquad }\n}'
NODE_S38_NEW=$'function wipConfigFrom(cfg) {\n  let limit = 1\n  try {\n    const content = fs.readFileSync(\'trackfw.yaml\', \'utf8\')\n    for (const line of content.split(\'\\n\')) {\n      const t = line.trim()\n      if (t.startsWith(\'wip_limit:\')) {\n        const val = t.slice(\'wip_limit:\'.length).trim().split(/\\s+/)[0]\n        const n = parseInt(val, 10)\n        if (!isNaN(n) && n > 0) limit = n\n      }\n    }\n  } catch (_) {}\n  return { limit, bySquad: !!cfg.wipBySquad }\n}'

T38C_NODE="$WORK/s38-corrupt-node"
setup_npm_tree "$T38C_NODE"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$T38C_NODE/npm/src/validator/index.js" \
  "$NODE_S38_OLD" "$NODE_S38_NEW" "s38-node"

set +e
s38c_node_out=$(cd "$S38_PROJECT" && node "$T38C_NODE/npm/bin/trackfw" validate 2>&1)
set -e
if grep -qF "$S38_REGRESSED_WARNING" <<<"$s38c_node_out" && ! grep -qF "$S38_EXPECTED_WARNING" <<<"$s38c_node_out"; then
  echo "OK   [falsify/wip-limit-quoted/node-detects-artisanal-reader-reintroduced]"
else
  echo "FAIL [falsify/wip-limit-quoted/node-detects-artisanal-reader-reintroduced]: leitor artesanal reintroduzido mas a saída não voltou a '(limit: 1)' — checagem vácua" >&2
  echo "  output: $(printf '%q' "$s38c_node_out")" >&2
  exit 1
fi

# --- Python: prova de detecção ------------------------------------------------
PY_S38_OLD=$'def _wip_config_from(cfg: dict) -> dict:\n    """\n    Deriva {"limit": int, "by_squad": bool} a partir do dict de config já normalizado por\n    _config.load() — nenhuma releitura de trackfw.yaml acontece aqui.\n    """\n    limit = cfg.get("wip_limit", 1)\n    if not isinstance(limit, int) or limit <= 0:\n        limit = 1\n    return {"limit": limit, "by_squad": bool(cfg.get("wip_by_squad", False))}'
PY_S38_NEW=$'def _wip_config_from(cfg: dict) -> dict:\n    limit = 1\n    try:\n        with open("trackfw.yaml", "r", encoding="utf-8") as f:\n            content = f.read()\n        for line in content.split("\\n"):\n            t = line.strip()\n            if t.startswith("wip_limit:"):\n                fields = t[len("wip_limit:"):].strip().split()\n                if fields:\n                    try:\n                        n = int(fields[0])\n                        if n > 0:\n                            limit = n\n                    except ValueError:\n                        pass\n    except OSError:\n        pass\n    return {"limit": limit, "by_squad": bool(cfg.get("wip_by_squad", False))}'

T38C_PYTHON="$WORK/s38-corrupt-python"
mkdir -p "$T38C_PYTHON"
cp -r "$ROOT_DIR/pypi" "$T38C_PYTHON/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/validator.py" "$T38C_PYTHON/pypi/trackfw/validator.py" \
  "$PY_S38_OLD" "$PY_S38_NEW" "s38-python"

set +e
s38c_python_out=$(cd "$S38_PROJECT" && env PYTHONPATH="$T38C_PYTHON/pypi" python3 -m trackfw validate 2>&1)
set -e
if grep -qF "$S38_REGRESSED_WARNING" <<<"$s38c_python_out" && ! grep -qF "$S38_EXPECTED_WARNING" <<<"$s38c_python_out"; then
  echo "OK   [falsify/wip-limit-quoted/python-detects-artisanal-reader-reintroduced]"
else
  echo "FAIL [falsify/wip-limit-quoted/python-detects-artisanal-reader-reintroduced]: leitor artesanal reintroduzido mas a saída não voltou a '(limit: 1)' — checagem vácua" >&2
  echo "  output: $(printf '%q' "$s38c_python_out")" >&2
  exit 1
fi


# ---------------------------------------------------------------------------
# Cenários 39/40/41 — ML-3A (REQ-2026-08-02-unificar-a-leitura-do-trackfw-
# yaml-em-um-unico-carregador-nos-tres-clis): `trackfw update` volta a ler
# hooks/ci/backend/frontend/pkg_manager com um scanner artesanal em vez de
# consumir o namespace `Update` já resolvido pelo carregador único
# (config.Load()/projectConfig.load()/project_config.load() — ver
# internal/generators/update.go:loadUpdateConfig,
# npm/src/commands/update.js:loadUpdateConfig,
# pypi/trackfw/commands/update.py:_load_update_config).
#
# Fixture discriminante (AC4/AC7 — chave aninhada homônima, o candidato mais
# forte da REQ): `hooks: lefthook` na raiz do YAML e uma seção NÃO relacionada
# com uma chave `hooks:` homônima aninhada por baixo dela.
#
#   hooks: lefthook
#   legacy_project_settings:
#     hooks: husky
#
# O carregador único respeita a estrutura do mapeamento — só a chave `hooks`
# da RAIZ é lida (ProjectConfig.Update.Hooks == "lefthook"). Um scanner
# artesanal reintroduzido (mesmo padrão eliminado pelo ML-2A: itera linha a
# linha, casa o prefixo `hooks:` em QUALQUER indentação, ignora nesting)
# sobrescreve o valor a cada ocorrência — a última linha que casa vence — e
# termina com "husky" em vez de "lefthook".
#
# Guarda de vivacidade: o efeito não é só "o arquivo lido mudou" — é
# observável no comportamento de `updateHooksSurgical`/`_update_hooks_surgical`.
# Go e Python GRAVAM o arquivo incondicionalmente e imprimem "✓ <arquivo> —
# trackfw[-]validate injetado" com hooks=lefthook (correto) ou hooks=husky
# (regredido). Node.js, na invocação bare (sem --install-missing), reporta o
# alvo `git-hooks` como `missing` — a escrita real fica atrás de
# --install-missing (runFileTarget não chama `apply` quando o arquivo ainda
# não existe e installMissing é false) — mas o CAMPO `path` do relatório
# ainda diverge (`lefthook.yml` vs `.husky/pre-commit`), então o sinal
# continua genuíno e não-vácuo: é o mesmo `cfg.hooks` resolvido pelo scanner
# que decide qual nome aparece, escrito ou não. Os três braços verificam qual
# dos dois nomes aparece na saída de `trackfw update` bare (sem flags — ver
# constraint da barreira ML-2A/Hefesto para o braço Python, que possui um
# segundo caminho, `_run_project`, atrás de --dry-run/--json/--targets/
# --install-missing, que NUNCA chama o carregador — fora do escopo desta REQ).
#
# Duas provas foram feitas para cada CLI, complementares: (1) corrupção de
# uma CÓPIA isolada em $WORK (os braços de detecção abaixo, que rodam sempre
# dentro da suíte) e (2) corrupção do ARQUIVO REAL do working tree, rodada
# manualmente uma única vez durante o desenvolvimento deste ML para confirmar
# que os braços de baseline (que consomem `$T27_GO_BIN`/`$ROOT_DIR/npm/bin/
# trackfw`/`PYTHONPATH=$ROOT_DIR/pypi` — código real, não corrompido) de fato
# flipam se alguém regredir o código real — não só a cópia. Revertida
# (`git checkout --`) e confirmada limpa (`git status --porcelain`) em
# seguida; não faz parte da execução normal do gate (custaria 3 rebuilds/
# reverts a cada corrida). Uma corrupção real também dispara um segundo
# mecanismo independente do `corrupt_literal`/`assert_fails_with` normal: se
# o literal-alvo mudar de forma (refactor), `corrupt_literal` falha primeiro
# com "expected exactly 1 occurrence… got 0" — sintoma de setup, não de
# veredito do gate (ver vault/notes/cenarios-de-falsificacao-quebram-em-
# refactor-do-alvo-2026-08-02.md).
#
# Corrompe a IMPLEMENTAÇÃO (loadUpdateConfig/_load_update_config), nunca a
# asserção — mesmo padrão dos cenários anteriores.
# ---------------------------------------------------------------------------

write_update_hooks_discriminant_fixture() {
  local dest=$1
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<'FIXEOF'
hooks: lefthook
legacy_project_settings:
  hooks: husky
FIXEOF
}

S39_EXPECTED_MSG='✓ lefthook.yml — trackfw-validate injetado'
S39_REGRESSED_MSG='✓ .husky/pre-commit — trackfw validate injetado'

# --- Cenário 39 — Go --------------------------------------------------------

S39_BASE="$WORK/s39-go-baseline"
mkdir -p "$S39_BASE"
write_update_hooks_discriminant_fixture "$S39_BASE/trackfw.yaml"
set +e
s39_base_out=$(cd "$S39_BASE" && "$T27_GO_BIN" update 2>&1)
s39_base_status=$?
set -e
if [[ $s39_base_status -eq 0 ]] \
    && grep -qF "$S39_EXPECTED_MSG" <<<"$s39_base_out" \
    && ! grep -qF "$S39_REGRESSED_MSG" <<<"$s39_base_out"; then
  echo "OK   [falsify/update-config-loader/go-baseline]"
else
  echo "FAIL [falsify/update-config-loader/go-baseline]: esperava exit 0 e '$S39_EXPECTED_MSG'" >&2
  echo "  status: $s39_base_status" >&2
  echo "  output: $(printf '%q' "$s39_base_out")" >&2
  exit 1
fi

GO_S39_OLD=$'func loadUpdateConfig() Config {\n\tu := config.Load().Update\n\treturn Config{\n\t\tHooks:      u.Hooks,\n\t\tCI:         u.CI,\n\t\tBackend:    u.Backend,\n\t\tFrontend:   u.Frontend,\n\t\tPkgManager: u.PkgManager,\n\t}\n}'
GO_S39_NEW=$'func loadUpdateConfig() Config {\n\t// [falsified] artisanal line-by-line scanner reintroduced — matches the "hooks:" prefix at\n\t// ANY indentation and keeps overwriting, so the LAST matching line wins regardless of nesting.\n\t// config.Load() is still invoked (kept referenced) but its Update namespace is discarded.\n\t_ = config.Load()\n\tdata, err := os.ReadFile("trackfw.yaml")\n\tif err != nil {\n\t\treturn Config{}\n\t}\n\tcfg := Config{}\n\tfor _, line := range strings.Split(string(data), "\\n") {\n\t\tline = strings.TrimSpace(line)\n\t\tif strings.HasPrefix(line, "#") {\n\t\t\tcontinue\n\t\t}\n\t\tidx := strings.Index(line, ":")\n\t\tif idx < 0 {\n\t\t\tcontinue\n\t\t}\n\t\tkey := strings.TrimSpace(line[:idx])\n\t\tval := strings.TrimSpace(line[idx+1:])\n\t\tswitch key {\n\t\tcase "hooks":\n\t\t\tcfg.Hooks = val\n\t\tcase "ci":\n\t\t\tcfg.CI = val\n\t\tcase "backend":\n\t\t\tcfg.Backend = val\n\t\tcase "frontend":\n\t\t\tcfg.Frontend = val\n\t\tcase "pkg_manager":\n\t\t\tcfg.PkgManager = val\n\t\t}\n\t}\n\treturn cfg\n}'

T39C_GO_MOD="$WORK/s39-corrupt-go"
mkdir -p "$T39C_GO_MOD/cmd" "$T39C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T39C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T39C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T39C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T39C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/update.go" "$T39C_GO_MOD/internal/generators/update.go" \
  "$GO_S39_OLD" "$GO_S39_NEW" "s39-go"

T39C_GO_BIN="$WORK/s39-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T39C_GO_BIN")"
build_go_or_fail "setup-s39-go-corrupt-build" "$T39C_GO_MOD" "$T39C_GO_BIN"

S39C="$WORK/s39-go-corrupt"
mkdir -p "$S39C"
write_update_hooks_discriminant_fixture "$S39C/trackfw.yaml"
set +e
s39c_out=$(cd "$S39C" && "$T39C_GO_BIN" update 2>&1)
set -e
if grep -qF "$S39_REGRESSED_MSG" <<<"$s39c_out" && ! grep -qF "$S39_EXPECTED_MSG" <<<"$s39c_out"; then
  echo "OK   [falsify/update-config-loader/go-detects-artisanal-scanner-reintroduced]"
else
  echo "FAIL [falsify/update-config-loader/go-detects-artisanal-scanner-reintroduced]: scanner artesanal reintroduzido mas a saída não regrediu para hooks=husky — checagem vácua" >&2
  echo "  output: $(printf '%q' "$s39c_out")" >&2
  exit 1
fi

# --- Cenário 40 — Node.js ----------------------------------------------------

S40_BASE="$WORK/s40-node-baseline"
mkdir -p "$S40_BASE"
write_update_hooks_discriminant_fixture "$S40_BASE/trackfw.yaml"
set +e
s40_base_out=$(cd "$S40_BASE" && node "$ROOT_DIR/npm/bin/trackfw" update 2>&1)
s40_base_status=$?
set -e
if [[ $s40_base_status -eq 0 ]] \
    && grep -qF "lefthook.yml" <<<"$s40_base_out" \
    && ! grep -qF ".husky/pre-commit" <<<"$s40_base_out"; then
  echo "OK   [falsify/update-config-loader/node-baseline]"
else
  echo "FAIL [falsify/update-config-loader/node-baseline]: esperava exit 0 e 'lefthook.yml' no relatório de git-hooks" >&2
  echo "  status: $s40_base_status" >&2
  echo "  output: $(printf '%q' "$s40_base_out")" >&2
  exit 1
fi

NODE_S40_OLD=$'function loadUpdateConfig(rootDir) {\n  const u = projectConfig.load(rootDir).update;\n  return {\n    hooks: u.hooks,\n    ci: u.ci,\n    backend: u.backend,\n    frontend: u.frontend,\n    pkg_manager: u.pkgManager,\n  };\n}'
NODE_S40_NEW=$'function loadUpdateConfig(rootDir) {\n  // [falsified] artisanal line-by-line scanner reintroduced — matches the "hooks:" prefix at\n  // ANY indentation and keeps overwriting cfg[key], so the LAST matching line wins regardless\n  // of nesting. projectConfig.load() is no longer consulted at all.\n  const yamlPath = path.join(rootDir, \'trackfw.yaml\');\n  if (!fs.existsSync(yamlPath)) return {};\n  const lines = fs.readFileSync(yamlPath, \'utf8\').split(\'\\n\');\n  const cfg = {};\n  for (const line of lines) {\n    const trimmed = line.trim();\n    if (trimmed.startsWith(\'#\')) continue;\n    const idx = trimmed.indexOf(\':\');\n    if (idx < 0) continue;\n    const key = trimmed.slice(0, idx).trim();\n    let val = trimmed.slice(idx + 1).trim();\n    cfg[key] = val;\n  }\n  return {\n    hooks: cfg.hooks,\n    ci: cfg.ci,\n    backend: cfg.backend,\n    frontend: cfg.frontend,\n    pkg_manager: cfg.pkg_manager,\n  };\n}'

T40C_NODE="$WORK/s40-corrupt-node"
setup_npm_tree "$T40C_NODE"
corrupt_literal \
  "$ROOT_DIR/npm/src/commands/update.js" "$T40C_NODE/npm/src/commands/update.js" \
  "$NODE_S40_OLD" "$NODE_S40_NEW" "s40-node"

S40C="$WORK/s40-node-corrupt"
mkdir -p "$S40C"
write_update_hooks_discriminant_fixture "$S40C/trackfw.yaml"
set +e
s40c_out=$(cd "$S40C" && node "$T40C_NODE/npm/bin/trackfw" update 2>&1)
set -e
if grep -qF ".husky/pre-commit" <<<"$s40c_out" && ! grep -qF "lefthook.yml" <<<"$s40c_out"; then
  echo "OK   [falsify/update-config-loader/node-detects-artisanal-scanner-reintroduced]"
else
  echo "FAIL [falsify/update-config-loader/node-detects-artisanal-scanner-reintroduced]: scanner artesanal reintroduzido mas a saída não regrediu para hooks=husky — checagem vácua" >&2
  echo "  output: $(printf '%q' "$s40c_out")" >&2
  exit 1
fi

# --- Cenário 41 — Python -----------------------------------------------------
#
# Constraint da barreira (Hefesto): o braço Python precisa exercitar
# `trackfw update` BARE (sem --dry-run/--json/--targets/--install-missing) —
# esse é o único caminho (_run, via _load_update_config) que satisfaz AC6.
# Os quatro flags caem em _run_project, que nunca chama o carregador de
# config e por isso tornaria o cenário vácuo (passaria idêntico com o
# scanner artesanal reintroduzido). Verificado empiricamente abaixo.

S41_BASE="$WORK/s41-python-baseline"
mkdir -p "$S41_BASE"
write_update_hooks_discriminant_fixture "$S41_BASE/trackfw.yaml"
set +e
s41_base_out=$(cd "$S41_BASE" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw update 2>&1)
s41_base_status=$?
set -e
if [[ $s41_base_status -eq 0 ]] \
    && grep -qF "$S39_EXPECTED_MSG" <<<"$s41_base_out" \
    && ! grep -qF "$S39_REGRESSED_MSG" <<<"$s41_base_out"; then
  echo "OK   [falsify/update-config-loader/python-baseline]"
else
  echo "FAIL [falsify/update-config-loader/python-baseline]: esperava exit 0 e '$S39_EXPECTED_MSG' via 'trackfw update' bare" >&2
  echo "  status: $s41_base_status" >&2
  echo "  output: $(printf '%q' "$s41_base_out")" >&2
  exit 1
fi

PY_S41_OLD=$'def _load_update_config(cwd: str) -> dict[str, str]:\n    """Reads the 5 fields `trackfw update` cares about via the single config loader\n    (trackfw.config, see ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-\n    tipados.md) instead of a second, artisanal read of trackfw.yaml. config.load() reads\n    relative to the given cwd (unlike Go\'s process-cwd-only Load()), so no chdir is required."""\n    return dict(project_config.load(cwd)["update"])'
PY_S41_NEW=$'def _load_update_config(cwd: str) -> dict[str, str]:\n    # [falsified] artisanal line-by-line scanner reintroduced — matches the "hooks:" prefix at\n    # ANY indentation and keeps overwriting cfg[key], so the LAST matching line wins regardless\n    # of nesting. project_config.load() is no longer consulted at all.\n    yaml_path = os.path.join(cwd, "trackfw.yaml")\n    try:\n        with open(yaml_path, "r", encoding="utf-8") as f:\n            content = f.read()\n    except OSError:\n        return {}\n    cfg: dict[str, str] = {}\n    for line in content.split("\\n"):\n        trimmed = line.strip()\n        if trimmed.startswith("#"):\n            continue\n        idx = trimmed.find(":")\n        if idx < 0:\n            continue\n        key = trimmed[:idx].strip()\n        val = trimmed[idx + 1:].strip()\n        cfg[key] = val\n    return cfg'

T41C_PYTHON="$WORK/s41-corrupt-python"
mkdir -p "$T41C_PYTHON"
cp -r "$ROOT_DIR/pypi" "$T41C_PYTHON/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/update.py" "$T41C_PYTHON/pypi/trackfw/commands/update.py" \
  "$PY_S41_OLD" "$PY_S41_NEW" "s41-python"

S41C="$WORK/s41-python-corrupt"
mkdir -p "$S41C"
write_update_hooks_discriminant_fixture "$S41C/trackfw.yaml"
set +e
s41c_out=$(cd "$S41C" && env PYTHONPATH="$T41C_PYTHON/pypi" python3 -m trackfw update 2>&1)
set -e
if grep -qF "$S39_REGRESSED_MSG" <<<"$s41c_out" && ! grep -qF "$S39_EXPECTED_MSG" <<<"$s41c_out"; then
  echo "OK   [falsify/update-config-loader/python-detects-artisanal-scanner-reintroduced]"
else
  echo "FAIL [falsify/update-config-loader/python-detects-artisanal-scanner-reintroduced]: scanner artesanal reintroduzido mas a saída não regrediu para hooks=husky — checagem vácua (verifique se a invocação bare de fato passou por _run/_load_update_config)" >&2
  echo "  output: $(printf '%q' "$s41c_out")" >&2
  exit 1
fi

# Guarda de não-vacuidade adicional (constraint da barreira): confirma que o
# caminho --dry-run (_run_project) do Python, mesmo com o carregador corrompido,
# NÃO reproduz o diagnóstico de detecção acima — provando que o cenário
# realmente depende da invocação bare (_run) e não passaria por acidente com
# qualquer flag.
set +e
s41c_dryrun_out=$(cd "$S41C" && env PYTHONPATH="$T41C_PYTHON/pypi" python3 -m trackfw update --dry-run 2>&1)
set -e
if ! grep -qF "$S39_REGRESSED_MSG" <<<"$s41c_dryrun_out" && ! grep -qF "$S39_EXPECTED_MSG" <<<"$s41c_dryrun_out"; then
  echo "OK   [falsify/update-config-loader/python-dry-run-path-confirmed-blind]"
else
  echo "FAIL [falsify/update-config-loader/python-dry-run-path-confirmed-blind]: --dry-run inesperadamente emitiu uma das mensagens de hooks — a constraint 'bare only' pode estar desatualizada" >&2
  echo "  output: $(printf '%q' "$s41c_dryrun_out")" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 42 — check-branch-new-parity.sh: Node.js reformata a mensagem
#              "blocked: ..." de stderr → gate detecta divergência.
#
# Objetivo (ML-3A, ROADMAP-2026-08-04-comando-trackfw-branch-new-para-bloquear-
# criacao-de-branch-sem-req-roadmap-em-wip): o contrato de `trackfw branch new`
# promete que a linha `blocked: no matching roadmap in wip/ nor done/ for "..."`
# escrita em stderr é idêntica nos 3 runtimes (Go: root.go Execute() imprime o
# erro retornado; Node.js: branch/runner.js's writeErr; Python: run_branch_new's
# err_out.write). Corrompe o texto emitido pelo Node.js — o cenário (a) do gate
# (no-match) deve detectar a divergência de stderr.
#
# Seam: sed troca a string `blocked: no matching roadmap in wip/ nor done/ for`
# por uma paráfrase em npm/src/branch/runner.js — corrompe a IMPLEMENTAÇÃO
# (fixture do gate), nunca a asserção do gate — mesmo padrão dos Cenários
# 14/16/17/20.
# ---------------------------------------------------------------------------
T42="$WORK/s42"
mkdir -p "$T42/scripts"
setup_npm_tree "$T42"
ln -s "$ROOT_DIR/pypi" "$T42/pypi"
cp "$ROOT_DIR/scripts/check-branch-new-parity.sh" "$T42/scripts/"

sed 's/blocked: no matching roadmap in wip\/ nor done\/ for/blocked: roadmap not found for branch/' \
  "$ROOT_DIR/npm/src/branch/runner.js" > "$T42/npm/src/branch/runner.js"

# Guard: garantir que a corrupção foi aplicada
if cmp -s "$ROOT_DIR/npm/src/branch/runner.js" "$T42/npm/src/branch/runner.js"; then
  echo "FAIL [falsify/setup-s42]: sed não alterou branch/runner.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "branch-new-parity/no-match/go-vs-node/err-message-reformatted-not-detected" \
  "branch-new-parity/no-match/go-vs-node/err" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T42/scripts/check-branch-new-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 43 — check-attention-scripts-parity.sh: Python literal do texto
#              "no-op fora da raiz" diverge de Go/Node.js → gate detecta.
#
# Objetivo (ROADMAP-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-
# entre-go-node-e-python-sem-gate-de-paridade): os dois scripts de attention
# hooks (signal e cleanup) são embutidos como literal-fonte independente em
# cada runtime — nada além deste gate garante que ficam byte-idênticos. Esta
# é exatamente a classe de regressão que motivou a REQ (o comentário já
# divergiu em PT/EN/PT-diferente sem nenhum gate notar). Corrompe apenas o
# literal `_ATTENTION_CLEANUP_SH` do Python numa cópia isolada de pypi/ — o
# gate deve reprovar com o diff explícito entre Go e Python.
#
# Seam: corrupt_literal com contexto estendido até "ROADMAP_DIR=$(grep" —
# a mesma frase de comentário aparece IDÊNTICA em _ATTENTION_SIGNAL_SH (que
# tem "if command -v jq" logo depois, não "ROADMAP_DIR=$(grep"), então o
# contexto extra restringe a substituição à única ocorrência do script de
# cleanup — sem isso corrupt_literal aborta com "expected exactly 1
# occurrence" (mesmo padrão de escopo do Cenário 34/corrupt_python_func_literal).
#
# Reaproveita o padrão dos Cenários 36/42: o gate roda a partir de sua própria
# cópia (T43/scripts/), cujo ROOT_DIR relativo aponta para o fixture — NODE_CLI
# vem de setup_npm_tree (não corrompido aqui) e PY_ROOT (default
# $ROOT_DIR/pypi dentro do gate) aponta para a cópia corrompida de pypi/.
# ---------------------------------------------------------------------------
T43="$WORK/s43"
mkdir -p "$T43/scripts"
setup_npm_tree "$T43"
cp -r "$ROOT_DIR/pypi" "$T43/pypi"
cp "$ROOT_DIR/scripts/check-attention-scripts-parity.sh" "$T43/scripts/"

corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/generators/init_gen.py" "$T43/pypi/trackfw/generators/init_gen.py" \
  $'# Script is intentionally a no-op when executed outside the project root\n[ -f "trackfw.yaml" ] || exit 0\n\nROADMAP_DIR=$(grep' \
  $'# Script disables itself outside the trackfw project root\n[ -f "trackfw.yaml" ] || exit 0\n\nROADMAP_DIR=$(grep' \
  "s43-python-cleanup-comment"

assert_fails_with "attention-scripts-parity/trackfw-attention-cleanup.sh/go-vs-py-comment-drift-not-detected" \
  "attention-scripts-parity/trackfw-attention-cleanup.sh/go-vs-py" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T43/scripts/check-attention-scripts-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 44 — check-agent-hooks-parity.sh: Node.js muda o `matcher` de
#              trackfw-credential-guard-post no wiring do Kiro (.kiro/hooks/
#              trackfw-attention.json) → gate detecta o drift estrutural.
#
# Objetivo (ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-
# credenciais-reais-por-subagentes, ML-3A): InjectXHooks (Go
# internal/generators/agentfiles.go), injectXHooks (Node npm/src/generators/
# hooks.js) e inject_x_hooks (Python pypi/trackfw/generators/hooks.py) para
# cada um dos 6 CLIs da wave nativa são 3 implementações independentes que só
# precisam concordar em ESTRUTURA (chaves/valores relevantes), não em
# byte-a-byte — nada além deste gate garante que ficam em paridade estrutural
# entre si. Corrompe apenas o literal `injectKiroHooks` do Node.js (troca o
# matcher de 'shell' para 'execute_bash' na entrada
# 'trackfw-credential-guard-post') numa cópia isolada de npm/ — o gate deve
# reprovar com o path JSON divergente ($.hooks[3].matcher) no diagnóstico.
#
# Seam: corrupt_literal com o campo `matcher: 'shell'` isolado por contexto de
# `name: 'trackfw-credential-guard-post'` na linha anterior — a mesma string
# `matcher: 'shell'` aparece 2x em injectKiroHooks (entradas -pre e -post),
# então o contexto do `name` vizinho restringe a substituição à única
# ocorrência da entrada -post (mesmo padrão de escopo do Cenário 34/
# corrupt_python_func_literal e do Cenário 43).
#
# Reaproveita o padrão do Cenário 43: o gate roda a partir de sua própria
# cópia (T44/scripts/), cujo ROOT_DIR relativo aponta para o fixture —
# NODE_CLI (não sobrepunível por env em check-agent-hooks-parity.sh, ao
# contrário de GO_BIN/PY_ROOT) resolve para a árvore corrompida via
# setup_npm_tree; GO_BIN e PY_ROOT apontam para o binário/pypi reais e
# não-corrompidos do repositório (só o Node precisa estar isolado aqui).
# ---------------------------------------------------------------------------
T44="$WORK/s44"
mkdir -p "$T44/scripts"
setup_npm_tree "$T44"
cp "$ROOT_DIR/scripts/check-agent-hooks-parity.sh" "$T44/scripts/"

corrupt_literal \
  "$ROOT_DIR/npm/src/generators/hooks.js" "$T44/npm/src/generators/hooks.js" \
  $'name: \'trackfw-credential-guard-post\',\n        description: \'Warns on possible plaintext credential materialization after a shell command executes\',\n        trigger: \'PostToolUse\',\n        matcher: \'shell\',' \
  $'name: \'trackfw-credential-guard-post\',\n        description: \'Warns on possible plaintext credential materialization after a shell command executes\',\n        trigger: \'PostToolUse\',\n        matcher: \'execute_bash\',' \
  "s44-node-kiro-guard-post-matcher"

assert_fails_with "agent-hooks-parity/kiro/go-vs-node-matcher-drift-not-detected" \
  "agent-hooks-parity/kiro/go-vs-node" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" PY_ROOT="$ROOT_DIR/pypi" bash "$T44/scripts/check-agent-hooks-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 45 — check-harness-hooks-parity.sh: Python muda o `matcher` de
#              trackfw-credential-guard-global-post no wiring GLOBAL do Kiro
#              (~/.kiro/hooks/trackfw-credential-guard.json) → gate detecta o
#              drift estrutural.
#
# Objetivo (ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-
# cross-project-via-trackfw-update-harness, ML-4A): harnessCredentialGuard-
# TargetKiro (Go internal/generators/update.go), credentialGuardTargetKiro
# (Node npm/src/commands/update-harness.js) e _credential_guard_kiro_result
# (Python pypi/trackfw/commands/update_harness.py) são 3 implementações
# independentes que só precisam concordar em ESTRUTURA, não em byte-a-byte —
# nada além deste gate garante que ficam em paridade estrutural entre si.
# Corrompe apenas o literal `_credential_guard_kiro_result` do Python (troca
# o matcher da entrada "trackfw-credential-guard-global-post" de 'shell'
# para 'execute_bash') numa cópia isolada de pypi/ — o gate deve reprovar com
# o path JSON divergente ($.hooks[1].matcher) no diagnóstico.
#
# Seam: corrupt_literal com contexto estendido até a linha `"trigger":
# "PostToolUse",` — a mesma string `"matcher": "shell",` aparece 2x em
# _credential_guard_kiro_result (entradas -global-pre e -global-post), então
# o contexto do `"trigger": "PostToolUse"` na linha anterior restringe a
# substituição à única ocorrência da entrada -global-post (mesmo padrão de
# escopo do Cenário 44/corrupt_literal em injectKiroHooks).
#
# Reaproveita o padrão do Cenário 44: o gate roda a partir de sua própria
# cópia (T45/scripts/), cujo ROOT_DIR relativo aponta para o fixture —
# NODE_CLI (não sobrepunível por env em check-harness-hooks-parity.sh, ao
# contrário de GO_BIN/PY_ROOT) resolve para a árvore real via setup_npm_tree
# (não corrompida aqui); GO_BIN aponta para o binário real do repositório;
# PY_ROOT aponta para a cópia corrompida de pypi/ (só o Python precisa estar
# isolado neste cenário).
# ---------------------------------------------------------------------------
T45="$WORK/s45"
mkdir -p "$T45/scripts"
setup_npm_tree "$T45"
cp -r "$ROOT_DIR/pypi" "$T45/pypi"
cp "$ROOT_DIR/scripts/check-harness-hooks-parity.sh" "$T45/scripts/"

corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/update_harness.py" "$T45/pypi/trackfw/commands/update_harness.py" \
  $'                "name": "trackfw-credential-guard-global-post",\n                "description": "Warns on possible plaintext credential materialization after a shell command executes (global, all projects)",\n                "trigger": "PostToolUse",\n                "matcher": "shell",' \
  $'                "name": "trackfw-credential-guard-global-post",\n                "description": "Warns on possible plaintext credential materialization after a shell command executes (global, all projects)",\n                "trigger": "PostToolUse",\n                "matcher": "execute_bash",' \
  "s45-python-kiro-guard-post-matcher"

assert_fails_with "harness-hooks-parity/kiro/go-vs-py-matcher-drift-not-detected" \
  "harness-hooks-parity/kiro/go-vs-py" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" PY_ROOT="$T45/pypi" bash "$T45/scripts/check-harness-hooks-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 46 — check-agent-hooks-parity.sh: os 3 stacks param de emitir a
#              entrada de project-scope credential-guard para o Claude de
#              forma IDÊNTICA (não um drift entre stacks) → o guard de
#              vacuidade `credential-guard-present` (linhas ~189-202 do gate)
#              reprova, e não o comparador estrutural (compare_json).
#
# Objetivo (REQ-2026-08-11-prova-negativa-dedicada-para-o-guard-de-vacuidade-
# credential-guard-present-do-check-agent-hooks-parity): check-agent-hooks-
# parity.sh tem DUAS camadas — um guard de vacuidade (P2, grep por
# "trackfw-credential-guard.sh" no arquivo gerado) e um comparador estrutural
# Go×Node/Go×Python. O Cenário 44 falsifica só o comparador. Sem prova
# própria, o guard de vacuidade poderia parar de funcionar sem que nenhum
# cenário acusasse — exatamente o vetor que ele existe para pegar: os 3
# stacks removendo a entrada de credential-guard de forma idêntica, o que o
# comparador cross-stack, sozinho, não detecta (os 3 lados continuam iguais
# entre si).
#
# ARMADILHA (ver ROADMAP-2026-08-12, seção "A armadilha que define o desenho
# do cenário"): "arquivo de hook sem entrada de credential-guard" é um estado
# LEGÍTIMO quando o credential-guard GLOBAL já está instalado — sabotar
# apagando a entrada do ARQUIVO GERADO não funciona, porque o injector
# regenera o arquivo a cada execução do gate e a sabotagem some. A sabotagem
# tem de estar na EMISSÃO, nos 3 geradores, de forma idêntica.
#
# Seam escolhido: as 3 funções de dedup globalCredentialGuardInstalledClaude
# (Go internal/generators/agentfiles.go:1206, Node npm/src/generators/
# hooks.js:570) e _global_credential_guard_installed_claude (Python
# pypi/trackfw/generators/hooks.py:133) são substituídas por um corpo que
# sempre retorna true/True — nas 3 cópias isoladas do source, cada função
# reescrita para o corpo mínimo `return true`/`return True`. Isso simula
# exatamente a classe de bug de 2026-08-08 (dedup lendo "global instalado"
# quando não deveria), só que como REGRESSÃO DE CÓDIGO, não de ambiente:
# InjectClaudeHooks/injectClaudeHooks/inject_claude_hooks (linhas ~248/276 Go,
# ~678/686 Node, ~314 Python) então pulam TODA a emissão de
# `$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh` para o Claude, nos
# 3 stacks igualmente — confirmado por leitura das 3 funções injetoras: a
# emissão do credential-guard para Claude está inteiramente contida dentro
# desses `if !globalCredentialGuardInstalledClaude()` (não há caminho
# alternativo de emissão fora deles), então forçar a função a sempre retornar
# true é suficiente para eliminar a entrada por completo, sem tocar em
# nenhuma outra entrada (attention-signal/cleanup) nem em nenhum outro CLI —
# o comparador estrutural Go×Node/Go×Python para o Claude continua batendo
# (os 3 lados ficam igualmente sem a entrada). Claude é o alvo porque é o
# primeiro item de $CLIS no gate, tornando o label de FAIL
# "agent-hooks-parity/claude/<runtime>/credential-guard-present"
# deterministicamente o primeiro assert possível na saída.
#
# RETARGET: se o mecanismo de dedup migrar para algo table-driven (uma única
# função genérica parametrizada por CLI, em vez de uma função por CLI),
# reaponte a sabotagem para o que quer que suprima a emissão de
# project-scope só para o Claude — o texto exato acima (as 3 assinaturas de
# função) é a âncora de manutenção.
#
# $HOME permanece isolado: run_discover_init (dentro do próprio
# check-agent-hooks-parity.sh, copiado para o fixture) já isola HOME por
# runtime — este cenário não muda esse mecanismo, só o resultado da função de
# dedup.
#
# Mecânica: reaproveita setup_npm_tree + cópia do próprio gate para o
# fixture (padrão do Cenário 44, necessário porque NODE_CLI não é
# sobrepunível por env em check-agent-hooks-parity.sh); GO_BIN e PY_ROOT são
# sobrepuníveis, então o Go corrompido é compilado numa cópia isolada de
# cmd/+internal/ (padrão build_go_or_fail dos Cenários 34/35/etc.) e o Python
# corrompido numa cópia isolada de pypi/ (padrão do Cenário 45).
#
# Braço baseline: mesma árvore (Go/Node/Python) sem sabotagem — o gate deve
# sair com 0.
# Braço detecção: sai != 0, a saída contém o FAIL de
# agent-hooks-parity/claude/{go,node,py}/credential-guard-present, e NÃO
# contém "go-vs-node" nem "go-vs-py" — o gate sai logo após o guard de
# vacuidade reprovar (linha ~204-208 do gate), antes mesmo do comparador
# estrutural rodar, então nenhuma referência a go-vs-node/go-vs-py aparece na
# saída: prova de que a falha não veio do comparador.
#
# ML-1B (ROADMAP-2026-08-12): o braço de detecção também prova que a causa do
# FAIL foi a SABOTAGEM, não um `$HOME` vazado/não-isolado lendo o guard
# global real da máquina (o modo de falha ambiental de 2026-08-08 — ver
# vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-
# 2026-08-08.md). A sabotagem é um `return true` LITERAL que não lê `$HOME`
# em nenhuma das 3 linguagens; as outras 5 funções de dedup continuam lendo
# `$HOME` de verdade e não foram tocadas. O braço planta um `$HOME`
# SINTÉTICO (controlado por este script, não pelo ambiente real do
# executor) com o guard global do Codex — só do Codex, nenhum Claude — e o
# passa como `HOME` externo na invocação do gate. Em operação correta (com o
# isolamento por runtime de run_discover_init intacto) esse `HOME` externo é
# ignorado e os 5 CLIs não-sabotados passam; se o isolamento regredisse, o
# Codex passaria a ler o `$HOME` sintético e falharia também, mudando a
# assinatura de "só claude" para "claude + codex" — o que a asserção de
# exclusividade abaixo (nenhum dos 5 CLIs não-sabotados em FAIL) captura,
# sem depender do que está instalado no `$HOME` real de quem roda o gate.
# ---------------------------------------------------------------------------

# --- braço baseline: árvore íntegra, gate deve passar --------------------
T46B="$WORK/s46-baseline"
mkdir -p "$T46B/scripts"
setup_npm_tree "$T46B"
cp "$ROOT_DIR/scripts/check-agent-hooks-parity.sh" "$T46B/scripts/"

set +e
s46b_out=$(env GO_BIN="$ROOT_DIR/bin/trackfw" PY_ROOT="$ROOT_DIR/pypi" bash "$T46B/scripts/check-agent-hooks-parity.sh" 2>&1)
s46b_status=$?
set -e
if [[ $s46b_status -ne 0 ]]; then
  echo "FAIL [falsify/agent-hooks-parity/credential-guard-present-vacuity/baseline]: árvore íntegra deveria passar, saiu com $s46b_status" >&2
  echo "  output: $s46b_out" >&2
  exit 1
fi
echo "OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/baseline]"

# --- braço detecção: dedup sempre "instalado" nos 3 stacks -----------------
T46="$WORK/s46"
mkdir -p "$T46/scripts"
setup_npm_tree "$T46"
cp "$ROOT_DIR/scripts/check-agent-hooks-parity.sh" "$T46/scripts/"

corrupt_literal \
  "$ROOT_DIR/npm/src/generators/hooks.js" "$T46/npm/src/generators/hooks.js" \
  $'function globalCredentialGuardInstalledClaude() {\n  const scriptPath = globalCredentialGuardScriptPath()\n  if (!scriptPath) return false\n  const root = readGlobalHookJSON(\'.claude\', \'settings.json\')\n  if (!root || !root.hooks) return false\n  return hookArrayHasCommand(root.hooks.PreToolUse, \'Bash\', scriptPath)\n}' \
  $'function globalCredentialGuardInstalledClaude() {\n  return true\n}' \
  "s46-node-claude-dedup-always-true"

T46_GO_MOD="$WORK/s46-corrupt-go"
mkdir -p "$T46_GO_MOD/cmd" "$T46_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T46_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T46_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T46_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T46_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/agentfiles.go" "$T46_GO_MOD/internal/generators/agentfiles.go" \
  $'func globalCredentialGuardInstalledClaude() bool {\n\tscriptPath, ok := globalCredentialGuardScriptPath()\n\tif !ok {\n\t\treturn false\n\t}\n\troot, ok := readGlobalHookJSON(".claude", "settings.json")\n\tif !ok {\n\t\treturn false\n\t}\n\thooks, _ := root["hooks"].(map[string]interface{})\n\treturn hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath)\n}' \
  $'func globalCredentialGuardInstalledClaude() bool {\n\treturn true\n}' \
  "s46-go-claude-dedup-always-true"
T46_GO_BIN="$WORK/s46-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T46_GO_BIN")"
build_go_or_fail "setup-s46-go-corrupt-build" "$T46_GO_MOD" "$T46_GO_BIN"

T46_PY="$WORK/s46-corrupt-py"
mkdir -p "$T46_PY"
cp -r "$ROOT_DIR/pypi" "$T46_PY/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/generators/hooks.py" "$T46_PY/pypi/trackfw/generators/hooks.py" \
  $'def _global_credential_guard_installed_claude() -> bool:\n    script_path = _global_credential_guard_script_path()\n    if not script_path:\n        return False\n    root = _read_global_hook_json(\'.claude\', \'settings.json\')\n    if not root:\n        return False\n    hooks = root.get(\'hooks\')\n    if not isinstance(hooks, dict):\n        return False\n    return _hook_array_has_command(hooks.get(\'PreToolUse\'), \'Bash\', script_path)' \
  $'def _global_credential_guard_installed_claude() -> bool:\n    return True' \
  "s46-python-claude-dedup-always-true"

# ML-1B (ROADMAP-2026-08-12): torna este braço autodiscriminante — não basta
# ver o FAIL do Claude, é preciso provar que a causa foi a SABOTAGEM (a
# função de dedup do Claude hardcoded para `return true`) e não um `$HOME`
# vazado/não-isolado lendo o guard global real da máquina (o modo de falha
# ambiental de 2026-08-08 — ver vault/notes/
# check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md).
#
# Discriminante escolhido: a sabotagem acima é um `return true` LITERAL — não
# lê `$HOME` em nenhuma das 3 linguagens. As outras 5 funções de dedup
# (codex/gemini/cursor/copilot/kiro) continuam lendo `$HOME` de verdade e não
# foram tocadas por esta sabotagem. Isso dá uma assimetria observável: sob
# sabotagem real (com `$HOME` isolado, como o gate copiado já faz via
# run_discover_init), SÓ o Claude fica sem a entrada de project-scope; sob um
# vazamento de `$HOME` (isolamento removido/quebrado), QUALQUER CLI cujo
# guard global esteja presente no `$HOME` que vazou perde a entrada também —
# não só o Claude.
#
# Para que essa segunda condição seja verificável sem depender do que está
# instalado no `$HOME` REAL desta máquina (o problema do discriminante
# descartado no roadmap: "assertar que codex não falha" só funciona se o
# guard global do codex estiver instalado na máquina que roda o gate), este
# braço planta um `$HOME` SINTÉTICO — controlado por este script, não pelo
# ambiente — com o guard global do Codex instalado (e nenhum outro CLI,
# principalmente não o Claude) e o passa como `HOME` externo na invocação do
# gate copiado. Como o gate copiado isola `$HOME` por runtime dentro de
# run_discover_init (mecanismo intocado por esta sabotagem), esse `HOME`
# externo é ignorado em operação correta — os 5 CLIs não-sabotados continuam
# vendo um `$HOME` isolado vazio e, portanto, PASSAM. Se o isolamento
# regredisse (achado hipotético coberto pela prova exigida no roadmap), o
# Codex passaria a ler este `$HOME` sintético, "veria" seu guard global
# plantado e falharia também — mudando a assinatura de FAIL de "só claude"
# para "claude + codex", o que a asserção de exclusividade abaixo capturaria
# independente de qualquer coisa instalada no `$HOME` real do executor.
T46_FAKE_HOME="$WORK/s46-fake-global-home"
mkdir -p "$T46_FAKE_HOME/.codex"
# "type":"command" (ROADMAP-2026-08-17 ML-4B): must match what the real writer emits so this
# fixture stays a valid discriminant if $HOME isolation ever regressed — hookArrayHasCommand now
# requires the sibling "type" field, so an entry missing it would no longer read as "installed"
# even in the hypothetical leak this fixture defends against.
cat >"$T46_FAKE_HOME/.codex/hooks.json" <<EOF
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$T46_FAKE_HOME/.trackfw/scripts/trackfw-credential-guard.sh"}]}]}}
EOF

set +e
s46_out=$(env HOME="$T46_FAKE_HOME" GO_BIN="$T46_GO_BIN" PY_ROOT="$T46_PY/pypi" bash "$T46/scripts/check-agent-hooks-parity.sh" 2>&1)
s46_status=$?
set -e

if [[ $s46_status -eq 0 ]]; then
  echo "FAIL [falsify/agent-hooks-parity/credential-guard-present-vacuity/detected]: saiu com 0, esperava != 0" >&2
  echo "  output: $s46_out" >&2
  exit 1
fi

for s46_label in \
  "agent-hooks-parity/claude/go/credential-guard-present" \
  "agent-hooks-parity/claude/node/credential-guard-present" \
  "agent-hooks-parity/claude/py/credential-guard-present"
do
  if ! grep -qF "$s46_label" <<<"$s46_out"; then
    echo "FAIL [falsify/agent-hooks-parity/credential-guard-present-vacuity/detected]: saída não contém '$s46_label'" >&2
    echo "  output: $s46_out" >&2
    exit 1
  fi
done
echo "OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/detected]"

# Discriminante autodiscriminante: nenhum dos 5 CLIs não-sabotados pode
# aparecer como credential-guard-present FAIL — nem o Codex, cujo guard
# global sintético foi plantado acima exatamente para tornar esta asserção
# sensível a um vazamento de isolamento de `$HOME`, e não só à sabotagem.
for s46_clean_cli in codex gemini copilot cursor kiro; do
  for s46_clean_runtime in go node py; do
    s46_clean_label="agent-hooks-parity/$s46_clean_cli/$s46_clean_runtime/credential-guard-present"
    if grep -qF "$s46_clean_label" <<<"$s46_out"; then
      echo "FAIL [falsify/agent-hooks-parity/credential-guard-present-vacuity/discriminant]: saída contém '$s46_clean_label' — a falha não está isolada ao Claude sabotado, sinal de vazamento de \$HOME (ou de outra causa) em vez de sabotagem de código" >&2
      echo "  output: $s46_out" >&2
      exit 1
    fi
  done
done
echo "OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/discriminant]"

if grep -qE "go-vs-node|go-vs-py" <<<"$s46_out"; then
  echo "FAIL [falsify/agent-hooks-parity/credential-guard-present-vacuity/structural-comparator-not-reached]: saída contém referência ao comparador estrutural (go-vs-node/go-vs-py) — o cenário está testando o Cenário 44, não o guard de vacuidade" >&2
  echo "  output: $s46_out" >&2
  exit 1
fi
echo "OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/structural-comparator-not-reached]"

# ---------------------------------------------------------------------------
# Cenário 47 — internal/validator: prova de não-vacuidade da regra
#              "credential_guard_hook_resolvable" (ROADMAP-2026-08-12-
#              mitigacao-do-fail-open-do-credential-guard-..., ML-1A, Apolo;
#              este cenário é o ML-2A, Ártemis) — a regra ACUSA quando existe
#              hook de credential-guard de PROJETO registrado (aqui,
#              .claude/settings.json) e o script referenciado não existe, e
#              NÃO acusa quando o script está presente e executável.
#
# ÂNCORA DE MANUTENÇÃO / RETARGET: $S47_MSG_MISSING abaixo é um TRECHO do
# literal exato emitido por validateCredentialGuardHookResolvable em
# internal/validator/validator_credential_guard.go:163-166 ("... but the
# script does not exist — run `trackfw update` to regenerate it"). Se essa
# mensagem mudar de forma (wording, ordem dos campos, ou deixar de citar
# "trackfw update"), reaponte $S47_MSG_MISSING para o novo literal — os
# equivalentes Node (npm/src/validator/index.js, função
# validateCredentialGuardHookResolvable, ~linha 1231) e Python
# (pypi/trackfw/validator.py:1447, validate_credential_guard_hook_resolvable)
# precisam mudar a mensagem junto, por regra de paridade (ADR-2026-08-05) —
# mas este cenário, por desenho, exercita só o CLI Go (ver abaixo).
#
# Por que só o CLI Go: o roadmap (ML-2A) permite testar um subconjunto dos 3
# stacks quando o cenário não precisa exercitar os outros para provar
# não-vacuidade. Os testes unitários dos 3 stacks já cobrem paridade de
# comportamento (internal/validator/validator_credential_guard_test.go;
# pypi/tests/test_validator.py:1001-1122; equivalente em npm/src/validator
# via npm test) — este cenário é a prova P4 (black-box, via `trackfw
# validate` de verdade) de que a regra Go não é vácua, o que já é suficiente
# para satisfazer o critério de aceite do ML.
#
# Por que não precisa isolar $HOME (ao contrário do Cenário 46): esta regra
# só lê arquivos de hook de PROJETO sob a raiz do projeto corrente
# (os.Getwd(), ver validateCredentialGuardHookResolvable) — nunca consulta
# $HOME ou o guard GLOBAL. Não existe vetor de vazamento ambiental a
# discriminar aqui, diferente do dedup globalCredentialGuardInstalled*() que
# o Cenário 46 testa.
#
# Braço autodiscriminante: a asserção de detecção usa assert_fails_with com
# o literal EXATO da mensagem desta regra, não um "saiu != 0" genérico.
# `grep -rn` em internal/validator/*.go confirma que nenhuma outra regra
# emite essa frase — então este braço só pode ser satisfeito pela regra sob
# teste disparando, nunca por uma violação incidental de outra regra. Reforço
# adicional: o fixture (scaffold_adr_req_project) é um projeto vazio sem
# docs/adr, docs/req ou docs/roadmaps/* — o mesmo fixture que o Cenário 29
# prova imprimir "✓ No violations found." byte-a-byte quando íntegro — então
# nenhuma OUTRA regra tem material para disparar neste projeto além da
# sabotagem deliberada (script ausente) que este cenário introduz.
#
# O que prova que a SABOTAGEM (não um acidente de fixture) é a causa: os dois
# braços usam s47_write_claude_guard_hook — o MESMO gerador de fixture — com
# um único delta entre eles (scripts/trackfw-credential-guard.sh criado e
# marcado +x no baseline, omitido na detecção). Isso encadeia os dois braços
# um no outro: o braço de detecção passando prova que a cadeia inteira está
# viva até o ponto de falha (JSON parseado → marcador
# "trackfw-credential-guard.sh" encontrado → prefixo $CLAUDE_PROJECT_DIR/
# resolvido → os.Stat alcançado e retornando "não existe") — se qualquer elo
# dessa cadeia estivesse quebrado (ex: typo no prefixo, marcador não
# reconhecido), a regra pularia o arquivo em silêncio e a DETECÇÃO teria
# falhado, não o contrário. O braço baseline então prova que o mesmo caminho
# de código, com o único delta "script presente e executável", fica em
# silêncio — atribuindo a diferença de resultado ao os.Stat, não a alguma
# outra causa incidental no fixture.
#
# Limite de cobertura conhecido (fora do escopo deste ML): a regra tem 2
# pontos de wiring em internal/validator/validator.go — applyRule (:418,
# usado por Validate(), o caminho de texto exercitado aqui) e
# applyRuleTagged (:604, usado por ValidateTagged()/`validate --json`). Este
# cenário e a prova de não-vacuidade abaixo cobrem só :418; uma regressão que
# remova a chamada em :604 sem tocar em :418 passaria por este gate em
# silêncio. Reportado a Zeus para decisão (ML novo ou aceitar o gap).
# ---------------------------------------------------------------------------

# ROADMAP-2026-08-17 ML-4B: the fixture must carry "type":"command" like the
# real writer (mergeClaudeHookArray, internal/generators/agentfiles.go) always
# emits -- credential_guard_hook_resolvable now treats a matched command
# WITHOUT that sibling field as a structurally malformed entry (see
# hookArrayHasCommand's ML-4B doc comment), not a validly-wired one, so a
# fixture missing it would make this scenario's own baseline arm fail for the
# wrong reason (masking the "does the script exist" check this cenário exists
# to prove) instead of exercising it.
s47_write_claude_guard_hook() {
  local dest=$1
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"}]}]}}
EOF
}

S47_MSG_MISSING='but the script does not exist — run `trackfw update` to regenerate it'

# --- braço baseline: script presente e executável -> validate passa --------
T47_OK="$WORK/s47-script-present"
scaffold_adr_req_project "$T47_OK"
s47_write_claude_guard_hook "$T47_OK/.claude/settings.json"
mkdir -p "$T47_OK/scripts"
cat > "$T47_OK/scripts/trackfw-credential-guard.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$T47_OK/scripts/trackfw-credential-guard.sh"

set +e
s47ok_out=$(cd "$T47_OK" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s47ok_status=$?
set -e
if [[ $s47ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-hook-resolvable/baseline]: árvore íntegra (script presente e executável) deveria passar, saiu com $s47ok_status" >&2
  echo "  output: $s47ok_out" >&2
  exit 1
fi
# Nota: o modo texto do `validate` (exercitado aqui) nunca imprime o nome
# interno da regra ("credential_guard_hook_resolvable") — só a mensagem. Só
# `validate --json` exporia o rule key (ver RuleItem.Rule em
# internal/validator/result.go), caminho não coberto por este cenário (ver
# comentário "Limite de cobertura conhecido" acima). A asserção aqui checa
# apenas a AUSÊNCIA da mensagem desta regra, que é o que o modo texto pode
# provar.
if grep -qF "$S47_MSG_MISSING" <<<"$s47ok_out"; then
  echo "FAIL [falsify/credential-guard-hook-resolvable/baseline]: script presente e executável mas a regra disparou mesmo assim" >&2
  echo "  output: $s47ok_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-hook-resolvable/baseline]"

# --- braço detecção: script ausente -> validate acusa esta regra -----------
T47_MISSING="$WORK/s47-script-missing"
scaffold_adr_req_project "$T47_MISSING"
s47_write_claude_guard_hook "$T47_MISSING/.claude/settings.json"
# scripts/trackfw-credential-guard.sh deliberadamente OMITIDO — é a sabotagem.

assert_fails_with "credential-guard-hook-resolvable/detected" \
  "$S47_MSG_MISSING" \
  bash -c "cd '$T47_MISSING' && exec '$ROOT_DIR/bin/trackfw' validate"

# ---------------------------------------------------------------------------
# Cenário 48 — check-attention-scripts-parity.sh: Node.js reordena a
#              concatenação de CREDENTIAL_GUARD_SCRIPT (CG_HEADER +
#              CG_PROJECT_GUARD + CG_DETECTION_CORE + CG_PROJECT_TAIL) →
#              trackfw-credential-guard.sh emitido por `discover --init`
#              diverge de Go/Python, e o gate (agora estendido por ML-0B do
#              ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-
#              regra-de-validate para incluir esse terceiro script) detecta.
#
# Objetivo: provar que a extensão do gate (adicionar trackfw-credential-guard.sh
# ao loop de trackfw-attention-signal.sh/trackfw-attention-cleanup.sh) é
# não-vácua — e, mais importante, provar que ela cobre uma classe de
# regressão que o teste Go pré-existente (TestCredentialGuardScript_
# ParityAcrossStacks, internal/generators/credential_guard_test.go) NÃO
# cobre: esse teste reconstrói o script Node/Python via regex-scraping dos
# literais CG_*/_CG_* de dentro do texto-fonte, concatenando-os na ORDEM que
# o próprio teste Go assume — nunca executa `discover --init` nos 3 runtimes
# nem lê a linha de composição real do Node
# (`const CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_PROJECT_GUARD + ...`).
# Corromper só essa linha de composição (sem tocar nos literais CG_* em si)
# muda o script REALMENTE EMITIDO pelo Node.js — o guard de no-op de projeto
# passa a rodar depois do núcleo de detecção em vez de antes, quebrando a
# garantia de no-op fora da raiz — enquanto `go test -run
# ParityAcrossStacks` continua verde (verificado manualmente durante a
# auditoria deste ML: a mesma sabotagem abaixo não move nenhum caso desse
# teste Go de PASS para FAIL). Este cenário prova que o gate SHELL, que
# executa `discover --init` de verdade nos 3 runtimes e diffa o arquivo
# emitido, pega o que o teste Go estrutural não pega.
#
# Seam: corrupt_literal na linha de composição de npm/src/generators/hooks.js
# — troca a ORDEM de CG_PROJECT_GUARD e CG_DETECTION_CORE, nunca o conteúdo
# de nenhum bloco CG_* (que são idênticos entre si nos dois arranjos — só a
# ORDEM da concatenação final muda).
# ---------------------------------------------------------------------------
T48="$WORK/s48"
mkdir -p "$T48/scripts"
setup_npm_tree "$T48"
ln -s "$ROOT_DIR/pypi" "$T48/pypi"
cp "$ROOT_DIR/scripts/check-attention-scripts-parity.sh" "$T48/scripts/"

corrupt_literal \
  "$ROOT_DIR/npm/src/generators/hooks.js" "$T48/npm/src/generators/hooks.js" \
  'const CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_PROJECT_GUARD + CG_DETECTION_CORE + CG_PROJECT_TAIL' \
  'const CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_DETECTION_CORE + CG_PROJECT_GUARD + CG_PROJECT_TAIL' \
  "s48-node-credential-guard-composition-order"

assert_fails_with "attention-scripts-parity/trackfw-credential-guard.sh/go-vs-node-composition-reordered-not-detected" \
  "attention-scripts-parity/trackfw-credential-guard.sh/go-vs-node" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T48/scripts/check-attention-scripts-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 49 — internal/validator: prova de não-vacuidade da regra
#              "credential_guard_script_integrity" (ROADMAP-2026-08-12-
#              deteccao-de-adulteracao-do-credential-guard-regra-de-validate,
#              ML-1A, Apolo; este cenário é o ML-2A, Ártemis) — a regra ACUSA
#              quando scripts/trackfw-credential-guard.sh diverge do template
#              que ESTE binário trackfw geraria, e NÃO acusa quando o
#              conteúdo é byte-idêntico ao template.
#
# ÂNCORA DE MANUTENÇÃO / RETARGET: $S49_MSG abaixo é um TRECHO do literal
# exato emitido por validateCredentialGuardScriptIntegrity em
# internal/validator/validator_credential_guard_integrity.go:53-57 ("%s
# content diverges from the template this version of trackfw generates — if
# you did not edit this file by hand, run `trackfw update` to regenerate
# it"). Se essa mensagem mudar de forma (wording, ordem dos campos, ou
# deixar de citar "content diverges from the template"), reaponte $S49_MSG
# para o novo literal — os equivalentes Node (npm/src/validator/index.js) e
# Python (pypi/trackfw/validator.py) precisam mudar junto, por regra de
# paridade, mas este cenário testa só o CLI Go (mesma justificativa do
# Cenário 47: testes unitários dos 3 stacks já cobrem paridade de
# comportamento; este cenário é a prova P4 black-box de que a regra Go não
# é vácua).
#
# Severidade: o default é "warning" (ADR-2026-08-12 Emenda 3 — o script não
# carrega marcador de versão, então a regra não discrimina drift legítimo de
# adulteração), e "warning" NÃO derruba o exit code de `trackfw validate`
# (internal/commands/validate.go: só violations viram Errorf; warnings só
# imprimem "⚠"). Os fixtures abaixo fixam `rules:
# credential_guard_script_integrity: error` para poder usar
# assert_fails_with (que exige exit != 0) — recomendação de Apolo/Zeus
# repassada no despacho deste ML; registrado aqui para não parecer
# inconsistente com o default real.
#
# Braço autodiscriminante: s49_write_fixture é o MESMO gerador para os dois
# fixtures principais (T49_OK/T49_BAD) — a única diferença entre eles é uma
# linha "# tampered..." apensada ao script DEPOIS de copiado do template
# real (gerado por uma execução isolada de `bin/trackfw discover --init` no
# início deste bloco, então byte-idêntico ao que ESTE binário produziria —
# não um literal reconstruído à mão que poderia divergir do binário sob
# teste). O braço baseline prova que o mesmo caminho de código, com o script
# intocado, fica em silêncio; o braço de detecção prova que o MESMO fixture
# com essa única linha a mais dispara — atribuindo a diferença de resultado
# ao conteúdo do script, não a alguma outra causa incidental: o fixture não
# tem .claude/settings.json, então credential_guard_hook_resolvable nunca
# tem material para disparar aqui, e nenhuma outra regra tem material no
# mesmo scaffold_adr_req_project que o Cenário 29 prova imprimir "✓ No
# violations found." byte-a-byte quando íntegro.
#
# Prova de não-vacuidade: T49_OFF é o MESMO fixture corrompido de T49_BAD,
# mas com `rules: credential_guard_script_integrity: off` em vez de `error`
# — o único delta é a severidade configurada, não o conteúdo do script.
# assert_would_now_fail roda EXATAMENTE o mesmo comando/critério que
# assert_fails_with usaria no braço de detecção (exit != 0 E mensagem
# presente) e exige que ele NÃO seja atendido aqui — provando que, com a
# regra desligada, o braço de detecção acima FALHARIA (não apenas que a
# mensagem some, que por si só provaria só que o knob `rules:` funciona, sem
# dizer nada sobre a asserção de detecção depender da regra). Não é preciso
# reconstruir bin/trackfw aqui: a sabotagem é inteiramente por config de
# fixture (`rules:`), nunca por edição de internal/validator/*.go — este ML
# não tem permissão de tocar internal/ (ver "Arquivos permitidos" no
# despacho).
#
# Limite de cobertura conhecido (mesmo do Cenário 47): a regra tem 2 pontos
# de wiring em internal/validator/validator.go — applyRule (usado por
# Validate(), o caminho de texto exercitado aqui) e applyRuleTagged (usado
# por ValidateTagged()/`validate --json`). Este cenário cobre só o primeiro;
# uma regressão isolada no segundo passaria por este gate em silêncio.
# ---------------------------------------------------------------------------
S49_MSG='content diverges from the template this version of trackfw generates'

S49_REF_DIR="$WORK/s49-ref"
mkdir -p "$S49_REF_DIR"
(cd "$S49_REF_DIR" && "$ROOT_DIR/bin/trackfw" discover --init </dev/null >/dev/null 2>&1)
S49_REF_SCRIPT="$S49_REF_DIR/scripts/trackfw-credential-guard.sh"
if [[ ! -s "$S49_REF_SCRIPT" ]]; then
  echo "FAIL [falsify/credential-guard-script-integrity/setup]: 'trackfw discover --init' não gerou scripts/trackfw-credential-guard.sh" >&2
  exit 1
fi

s49_write_fixture() {
  local dest=$1 severity=$2
  scaffold_adr_req_project "$dest"
  cat >> "$dest/trackfw.yaml" <<EOF
rules:
  credential_guard_script_integrity: $severity
EOF
  mkdir -p "$dest/scripts"
  cp "$S49_REF_SCRIPT" "$dest/scripts/trackfw-credential-guard.sh"
  chmod +x "$dest/scripts/trackfw-credential-guard.sh"
}

# --- braço baseline: script byte-idêntico ao template -> validate passa ----
T49_OK="$WORK/s49-script-identical"
s49_write_fixture "$T49_OK" error

set +e
s49ok_out=$(cd "$T49_OK" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s49ok_status=$?
set -e
if [[ $s49ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-script-integrity/baseline]: árvore íntegra (script byte-idêntico ao template) deveria passar, saiu com $s49ok_status" >&2
  echo "  output: $s49ok_out" >&2
  exit 1
fi
if grep -qF "$S49_MSG" <<<"$s49ok_out"; then
  echo "FAIL [falsify/credential-guard-script-integrity/baseline]: script íntegro mas a regra disparou mesmo assim" >&2
  echo "  output: $s49ok_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-script-integrity/baseline]"

# --- braço detecção: script corrompido -> validate acusa esta regra --------
T49_BAD="$WORK/s49-script-corrupted"
s49_write_fixture "$T49_BAD" error
printf '# tampered by check-gates-falsify.sh Cenario 49\n' >> "$T49_BAD/scripts/trackfw-credential-guard.sh"

assert_fails_with "credential-guard-script-integrity/detected" \
  "$S49_MSG" \
  bash -c "cd '$T49_BAD' && exec '$ROOT_DIR/bin/trackfw' validate"

# --- prova de não-vacuidade: mesma corrupção, regra desligada -> o braço de
# detecção FALHARIA (assert_would_now_fail exige exit==0 OU mensagem
# ausente; se o critério de assert_fails_with fosse satisfeito mesmo assim,
# reprova aqui) --------------------------------------------------------------
T49_OFF="$WORK/s49-script-corrupted-rule-off"
s49_write_fixture "$T49_OFF" off
printf '# tampered by check-gates-falsify.sh Cenario 49\n' >> "$T49_OFF/scripts/trackfw-credential-guard.sh"

assert_would_now_fail "credential-guard-script-integrity" \
  "$S49_MSG" \
  bash -c "cd '$T49_OFF' && exec '$ROOT_DIR/bin/trackfw' validate"

# ---------------------------------------------------------------------------
# Cenário 50 — internal/validator: prova de não-vacuidade da regra
#              "credential_guard_mode_downgrade" (ROADMAP-2026-08-12-
#              deteccao-de-adulteracao-do-credential-guard-regra-de-validate,
#              ML-1A, Apolo; este cenário é o ML-2A, Ártemis) — a regra ACUSA
#              quando credential_guard.mode era "block" no commit HEAD do
#              git e o trackfw.yaml em disco não resolve mais para "block",
#              e NÃO acusa quando disco e HEAD concordam.
#
# ÂNCORA DE MANUTENÇÃO / RETARGET: $S50_MSG abaixo é um TRECHO do literal
# exato emitido por credentialGuardModeDowngradeMessage em
# internal/validator/validator_credential_guard_integrity.go:174-177. Se
# essa mensagem mudar de forma (wording, ou deixar de citar "does not
# resolve to block"), reaponte $S50_MSG para o novo literal — Node
# (npm/src/validator/index.js) e Python (pypi/trackfw/validator.py) precisam
# mudar junto, por regra de paridade, mas este cenário testa só o CLI Go
# (mesma justificativa do Cenário 47/49: prova P4 black-box de
# não-vacuidade; paridade de comportamento já coberta pelos testes unitários
# dos 3 stacks). $S50_FULL_MSG é o literal COMPLETO (não só o trecho),
# usado pelo Cenário 52 abaixo como entrada de .trackfw-baseline.json — o
# filtro de baseline compara a mensagem inteira (validator.go:527), não uma
# substring.
#
# Severidade: o default já é "error" (credential_guard_mode_downgrade está
# deliberadamente AUSENTE de ruleDefaults em internal/validator/validator.go
# — cai no default de ruleSeverity) — diferente do Cenário 49, não precisa
# de override em `rules:` para usar assert_fails_with.
#
# Encanamento novo exigido por este cenário (achado de Apolo, repassado por
# Zeus no despacho): nenhum fixture existente em check-gates-falsify.sh
# fazia `git init`/commit antes deste — s50_commit_fixture é o primeiro a
# criar um repo git de verdade dentro de $WORK por cenário, necessário
# porque esta regra só lê `git show HEAD:./trackfw.yaml` (sem HEAD, fica em
# silêncio por desenho — não há como disparar sem essa âncora). Generalizado
# aqui (ML-2A) para aceitar o conteúdo do trackfw.yaml commitado como
# parâmetro — os Cenários 51/52/53 abaixo reusam o mesmo helper com HEADs
# diferentes.
#
# Braço autodiscriminante: s50_commit_fixture é o MESMO gerador para os dois
# fixtures principais (T50_OK/T50_BAD) — ambos commitam
# credential_guard.mode: block no HEAD via o MESMO conteúdo de trackfw.yaml
# (s50_yaml_content block). A ÚNICA diferença entre os dois braços é uma
# reescrita NÃO commitada do trackfw.yaml em disco depois do commit: o braço
# baseline não toca o disco (disco == HEAD, mode: block); o braço de
# detecção sobrescreve só o disco para mode: warn, sem novo commit (git
# status ficaria "dirty" — é exatamente o "relaxamento legítimo não
# commitado" que o ADR (Emenda 3) trata como o único falso positivo
# aceitável, e que esta mensagem converte no próprio rastro auditável: "if
# this was intentional, commit the change"). Isso atribui a diferença de
# resultado à divergência disco-vs-HEAD, não a alguma outra causa
# incidental — nenhuma outra regra tem material neste
# scaffold_adr_req_project além do credential_guard.mode adicionado (mesma
# garantia do Cenário 47/49: fixture base é o que o Cenário 29 prova "✓ No
# violations found." byte-a-byte).
#
# 🔴 ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard,
# ML-2A (Ártemis) — MECANISMO DE NÃO-VACUIDADE SUBSTITUÍDO (ADR Emenda 2):
# a prova original sabotava a regra escrevendo `rules:
# credential_guard_mode_downgrade: off` em disco SEM commit — exatamente o
# comportamento PRÉ-ADR (auto-silenciamento sem rastro) que o M4 (ML-1A)
# fecha. Com M4 em produção esse sabotage NÃO tem mais efeito nenhum: o
# HEAD (sem `rules:`) resolve para o default "error", que vence a
# comparação direcional "mais estrita entre HEAD e disco"
# (credentialGuardRuleSeverity, validator_credential_guard_integrity.go:252)
# mesmo com "off" em disco — então o braço de detecção continuaria
# disparando, e o braço antigo ficaria PERMANENTEMENTE vermelho. Isso não é
# regressão: é o gate provando o próprio bug que este ADR corrige (ver
# vault/notes/scenario-50-non-vacuity-obsoleta-pelo-anchoring-no-head-2026-08-12.md).
#
# Substituído por um sabotage que CONTINUA funcionando por desenho: `rules:
# credential_guard_mode_downgrade: off` COMMITADO junto com mode: block no
# MESMO commit de HEAD — o "desligamento legítimo" do ADR §Decision point 5
# (mesmo padrão de
# TestCredentialGuardModeDowngrade_ConfiguravelViaRules/off_commitado em
# validator_credential_guard_integrity_test.go:356-371: HEAD e disco
# concordam em "off", então "mais estrita entre HEAD e disco" resolve para
# "off" e a regra silencia de verdade). T50_OFF commita mode: block +
# rules: ...: off juntos e depois baixa SÓ o mode para "warn" em disco (sem
# novo commit, rules: off permanece em disco também — nenhuma outra
# variável muda). Este ML não tem permissão de editar
# internal/validator/*.go (ver "Arquivos permitidos" no despacho), então
# `_ = credentialGuardModeMsgs` (o outro sabotage sugerido, usado por Zeus
# em auditoria) não é uma opção AQUI — só o "rules: off commitado" fica
# disponível dentro do escopo deste ML.
#
# Limite de cobertura conhecido (mesmo do Cenário 47/49): cobre só o wiring
# applyRule (Validate()/texto), não applyRuleTagged (ValidateTagged()/
# `validate --json`) em internal/validator/validator.go.
# ---------------------------------------------------------------------------
S50_MSG='current file does not resolve to block'
S50_FULL_MSG='trackfw.yaml sets credential_guard.mode: block at the git HEAD commit, but the current file does not resolve to block — if this was intentional, commit the change; otherwise investigate before treating the credential guard as active'

# rules_severity vazio (padrão) omite o bloco `rules:` inteiro — usado pelos
# braços que não commitam/escrevem nenhum override de severidade.
s50_yaml_content() {
  local mode=$1 rules_severity=${2:-}
  cat <<EOF
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
credential_guard:
  mode: $mode
EOF
  if [[ -n "$rules_severity" ]]; then
    printf 'rules:\n  credential_guard_mode_downgrade: %s\n' "$rules_severity"
  fi
}

# Generalizado (ML-2A) para aceitar o conteúdo do trackfw.yaml commitado —
# antes só commitava s50_yaml_content block; os Cenários 51/52/53 precisam
# commitar HEADs diferentes (com/sem rules: off junto, com/sem
# credential_guard nenhum).
s50_commit_fixture() {
  local dest=$1 yaml_content=$2 commit_msg=$3
  scaffold_adr_req_project "$dest"
  printf '%s' "$yaml_content" > "$dest/trackfw.yaml"
  (
    cd "$dest"
    git init -q
    git config user.email "falsify@trackfw.test"
    git config user.name "trackfw falsify"
    # Isolamento contra config global ambiente do executor (vault/notes/
    # check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md,
    # mesma classe de problema): sem isto, um `commit.gpgsign=true` global
    # falharia sem chave disponível, e um `core.hooksPath` global rodaria
    # hooks do usuário dentro deste fixture descartável.
    git config commit.gpgsign false
    git config core.hooksPath /dev/null
    git add -A
    git commit -q -m "$commit_msg"
  )
}

# --- braço baseline: disco concorda com HEAD (mode: block) -> validate passa
T50_OK="$WORK/s50-mode-matches-head"
s50_commit_fixture "$T50_OK" "$(s50_yaml_content block)" \
  "trackfw.yaml with credential_guard.mode: block"

set +e
s50ok_out=$(cd "$T50_OK" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s50ok_status=$?
set -e
if [[ $s50ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-mode-downgrade/baseline]: disco == HEAD (mode: block) deveria passar, saiu com $s50ok_status" >&2
  echo "  output: $s50ok_out" >&2
  exit 1
fi
if grep -qF "$S50_MSG" <<<"$s50ok_out"; then
  echo "FAIL [falsify/credential-guard-mode-downgrade/baseline]: disco == HEAD mas a regra disparou mesmo assim" >&2
  echo "  output: $s50ok_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-mode-downgrade/baseline]"

# --- braço detecção: disco diverge do HEAD (mode: warn, não commitado) -----
T50_BAD="$WORK/s50-mode-downgraded"
s50_commit_fixture "$T50_BAD" "$(s50_yaml_content block)" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn > "$T50_BAD/trackfw.yaml"

assert_fails_with "credential-guard-mode-downgrade/detected" \
  "$S50_MSG" \
  bash -c "cd '$T50_BAD' && exec '$ROOT_DIR/bin/trackfw' validate"

# --- prova de não-vacuidade (mecanismo NOVO, ML-2A): mesma divergência de
# mode, mas HEAD commita `rules: credential_guard_mode_downgrade: off`
# JUNTO com mode: block — "mais estrita entre HEAD e disco" resolve para
# "off" (ambos concordam) e a regra silencia de verdade. Isso prova que o
# braço de detecção acima (T50_BAD, HEAD SEM rules: off) depende
# genuinamente da regra estar ativa — não apenas que o knob `rules:`
# funciona (esse era o furo do mecanismo antigo). Helper trocado de
# assert_would_now_fail (usado pelo mecanismo antigo, e ainda usado pelo
# Cenário 49) para assert_lacks_pattern: aqui o fixture representa
# comportamento CORRETO de produção (desligamento legítimo commitado
# silenciando de verdade), não uma árvore sabotada por config — não faz
# sentido pedir que o critério de assert_fails_with "não seja atendido"
# (disjunção: exit==0 OU mensagem ausente) quando o esperado é exit==0 E
# mensagem ausente ao mesmo tempo. assert_lacks_pattern exige as DUAS
# (conjunção) — critério estritamente mais forte, não uma divergência de
# padrão sem motivo. NÃO reverter para assert_would_now_fail aqui: o
# `rules: ...: off` deste braço está COMMITADO (ver acima), diferente do
# uso de assert_would_now_fail no Cenário 49, onde o `off` é só em disco. -
T50_OFF="$WORK/s50-mode-downgraded-rule-off-commitado"
s50_commit_fixture "$T50_OFF" "$(s50_yaml_content block off)" \
  "trackfw.yaml with credential_guard.mode: block and rules: credential_guard_mode_downgrade: off"
s50_yaml_content warn off > "$T50_OFF/trackfw.yaml"

assert_lacks_pattern "credential-guard-mode-downgrade/non-vacuity" \
  "$S50_MSG" \
  bash -c "cd '$T50_OFF' && exec '$ROOT_DIR/bin/trackfw' validate"

# ---------------------------------------------------------------------------
# Cenário 51 — internal/validator: o CENÁRIO DECISIVO do M4
#              (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-
#              credential-guard, ML-1A/ML-2A) — a edição COMBINADA e NÃO
#              commitada (`credential_guard.mode: warn` + `rules:
#              credential_guard_mode_downgrade: off`, ambos só em disco)
#              CONTINUA sendo reportada. É exatamente o furo que o
#              ML-0A/Barreira B0 descreveram: antes do M4, essa única edição
#              rebaixava o controle E desligava quem avisaria, sem deixar
#              rastro (ver Context do roadmap).
#
# Braço de detecção: T51_BAD commita SÓ mode: block no HEAD (sem rules: —
# o caso comum, "nenhuma decisão tomada sobre a severidade desta regra").
# Em disco, SEM novo commit, o ataque combinado sobrescreve tanto mode
# quanto rules: na MESMA edição — s50_yaml_content warn off produz as duas
# chaves de uma vez, então não há como a edição ser parcial. A mensagem
# TEM que aparecer: se HEAD-informed severity não vencesse aqui, o disco
# "off" desligaria a regra que deveria denunciar a própria mudança de
# disco — o auto-silenciamento que o ADR fecha.
S51_BAD_HEAD="$(s50_yaml_content block)"

T51_BAD="$WORK/s51-combined-uncommitted"
s50_commit_fixture "$T51_BAD" "$S51_BAD_HEAD" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn off > "$T51_BAD/trackfw.yaml"

assert_fails_with "credential-guard-anchoring-combined-edit/detected" \
  "$S50_MSG" \
  bash -c "cd '$T51_BAD' && exec '$ROOT_DIR/bin/trackfw' validate"

# Braço autodiscriminante/contraste: T51_OFF_COMMITTED aplica o MESMO
# ataque de disco (mode: warn + rules: off) — mas desta vez `rules:
# credential_guard_mode_downgrade: off` também está COMMITADO no HEAD
# (junto com mode: block, no mesmo commit — desligamento legítimo, ADR
# §Decision point 5, mesmíssima construção do braço de não-vacuidade do
# Cenário 50 acima). A ÚNICA variável entre T51_BAD e T51_OFF_COMMITTED é
# se o "off" estava commitado — o resultado muda de "reportado" para
# "silenciado" SÓ por causa dessa variável, isolando exatamente o que o M4
# promete: desligar continua possível, mas só via commit (rastro
# auditável), nunca por edição de disco sozinha. -------------------------
T51_OFF_COMMITTED="$WORK/s51-combined-off-committed"
s50_commit_fixture "$T51_OFF_COMMITTED" "$(s50_yaml_content block off)" \
  "trackfw.yaml with credential_guard.mode: block and rules: credential_guard_mode_downgrade: off"
s50_yaml_content warn off > "$T51_OFF_COMMITTED/trackfw.yaml"

assert_lacks_pattern "credential-guard-anchoring-combined-edit/legitimate-committed-off-silences" \
  "$S50_MSG" \
  bash -c "cd '$T51_OFF_COMMITTED' && exec '$ROOT_DIR/bin/trackfw' validate"

# 🔴 Prova de não-vacuidade do M4 em si (não apenas do knob `rules:`, já
# provado acima): sabotagem TEMPORÁRIA e NÃO commitada de
# internal/validator/validator_credential_guard_integrity.go — dentro de
# credentialGuardRuleSeverity, trocar `return
# credentialGuardStricterSeverity(headSeverity, diskSeverity)` por `return
# diskSeverity` (i.e., voltar ao comportamento pré-ADR, disco vence
# sempre), reconstruir bin/trackfw
# (vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12.md
# — go build ./... sozinho NÃO regenera o binário) e rodar T51_BAD de novo:
# o braço de detecção acima DEVE falhar (mensagem ausente, exit 0), porque
# o disco "off" venceria. Restaurar o arquivo e reconstruir antes de
# prosseguir — este ML não tem permissão de deixar internal/ tocado no
# diff final; a sabotagem é só para a prova de auditoria, nunca commitada.
# Saída colada no relatório final desta execução.
#
# Limite de cobertura conhecido (mesmo do Cenário 50): cobre só applyRule
# (Validate()/texto), não applyRuleTagged (ValidateTagged()/`validate
# --json`).
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 52 — internal/validator: o CARVE-OUT do .trackfw-baseline.json
#              (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-
#              credential-guard, Barreira B0/ML-1A/ML-2A) — uma violação de
#              regra de credential-guard listada em .trackfw-baseline.json
#              CONTINUA sendo reportada (filterBaselineTagged,
#              validator.go:500-511: as 3 regras em
#              credentialGuardAnchoredRules nunca são toleradas por
#              baseline, "regardless of what .trackfw-baseline.json contains
#              for it"). O canal do baseline é diferente do canal `rules:`
#              fechado pelos Cenários 50/51 — o arquivo é .gitignore'd
#              DELIBERADAMENTE (.gitignore:14-15), então "exigir commit" não
#              se aplica; o fechamento é excluir as 3 regras da elegibilidade
#              do ratchet, não comparar HEAD-vs-disco.
#
# Formato do .trackfw-baseline.json verificado contra a implementação (não
# escrito de improviso a partir da prosa do ADR — armadilha "prova que não
# prova" de outra forma): BaselineFile{Created, Violations []string,
# Warnings []string} (validator.go:18-23), e filterBaselineTagged compara
# CADA violação pelo texto INTEIRO da mensagem (v.Msg, validator.go:527) —
# não uma tag de regra, não um hash. $S50_FULL_MSG é esse literal completo
# para a regra de guarda; a chave para o carve-out entrar em jogo é o NOME
# da regra (credentialGuardAnchoredRules[v.Rule]), não o conteúdo da
# mensagem — mas o filtro só reconhece a mensagem se o texto bater
# EXATAMENTE, por isso a mensagem completa (não $S50_MSG, que é só um
# trecho) é o que entra no JSON.
#
# Braço autodiscriminante (mesmo fixture, uma única execução de `validate`,
# controle embutido em vez de scenario separado): T52 tem DUAS violações
# reais simultâneas — a de credential-guard (mode: warn não commitado,
# igual ao T50_BAD/T51_BAD) e uma de filename_uniqueness (regra NÃO-guard,
# "docs/roadmaps/backlog/dup.md" e "docs/roadmaps/done/dup.md" — mesmo nome
# (backlog+done, não wip/blocked: essas duas têm regras extras — "roadmap X is
# in wip but has no linked REQ/acceptance criteria block" — que poluiriam o exit
# code deste cenário sem relação com filename_uniqueness)
# em dois estados, validator.go:1938-2012) — e .trackfw-baseline.json lista
# as DUAS pelo texto completo. Se o carve-out funcionar: a de
# credential-guard continua aparecendo (não tolerada), a de
# filename_uniqueness some (tolerada normalmente). Isso prova as DUAS
# metades na mesma prova: (a) o formato do baseline realmente suprime
# quando a regra NÃO é de credential-guard — sem isso, a violação de guarda
# "aparecer" não provaria nada, porque o baseline poderia estar
# simplesmente mal-formado e não suprimir NADA; (b) o carve-out é
# ESPECÍFICO da regra de guarda, não uma falha geral do mecanismo de
# baseline. Sem o braço de filename_uniqueness, este cenário seria a
# "prova que não prova" documentada em
# vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12.md.
#
# 🔴 Prova de não-vacuidade: sabotagem TEMPORÁRIA e NÃO commitada de
# filterBaselineTagged (validator.go) — remover a condição
# `&& !credentialGuardAnchoredRules[v.Rule]` (deixando só `if tolerated {
# continue }`, i.e., o carve-out nunca existiu), reconstruir bin/trackfw e
# rodar T52 de novo: a violação de credential-guard deve DESAPARECER
# também (as duas ficariam suprimidas, exit 0) — provando que este cenário
# depende genuinamente do carve-out, não de alguma outra causa. Restaurar
# o arquivo e reconstruir antes de prosseguir. Saída colada no relatório.
# ---------------------------------------------------------------------------
S52_FILENAME_MSG='roadmap "dup.md" appears in multiple states: [backlog done]'

T52="$WORK/s52-baseline-carveout"
s50_commit_fixture "$T52" "$(s50_yaml_content block)" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn > "$T52/trackfw.yaml"
printf '# dup\n' > "$T52/docs/roadmaps/backlog/dup.md"
printf '# dup\n' > "$T52/docs/roadmaps/done/dup.md"
cat > "$T52/.trackfw-baseline.json" <<EOF
{
  "created": "2026-08-12T00:00:00Z",
  "violations": [
    "$S50_FULL_MSG",
    "roadmap \"dup.md\" appears in multiple states: [backlog done]"
  ],
  "warnings": []
}
EOF

set +e
s52_out=$(cd "$T52" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s52_status=$?
set -e
if [[ $s52_status -eq 0 ]]; then
  echo "FAIL [falsify/credential-guard-baseline-carveout]: baseline listando a violação de credential-guard deveria continuar reprovando (carve-out), saiu com 0" >&2
  echo "  output: $s52_out" >&2
  exit 1
fi
if ! grep -qF "$S50_MSG" <<<"$s52_out"; then
  echo "FAIL [falsify/credential-guard-baseline-carveout]: violação de credential-guard listada no baseline foi suprimida — carve-out não está funcionando" >&2
  echo "  output: $s52_out" >&2
  exit 1
fi
if grep -qF "$S52_FILENAME_MSG" <<<"$s52_out"; then
  echo "FAIL [falsify/credential-guard-baseline-carveout]: violação NÃO-guard (filename_uniqueness) listada no MESMO baseline não foi suprimida — o formato do baseline não está funcionando neste fixture (prova vácua: a linha acima passaria mesmo com um baseline mal-formado)" >&2
  echo "  output: $s52_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-baseline-carveout]: guarda reportada apesar do baseline, não-guarda suprimida pelo MESMO baseline"

# ---------------------------------------------------------------------------
# Cenário 53 — internal/validator: NÃO-REGRESSÃO — a regra "zero delta" do
#              ADR-2026-08-12 (Decision point, "zero delta para as outras
#              ~38 regras") não vazou para regras que NÃO são de
#              credential-guard. Uma regra comum (filename_uniqueness, não
#              listada em credentialGuardAnchoredRules) continua sendo
#              desligável por `rules: <nome>: off` NÃO commitado — o mesmo
#              comportamento de SEMPRE, disco-only (diskRuleSeverity), sem
#              nenhuma consulta a HEAD. Este é o cenário mais importante
#              para a confiança no M4: sem ele, uma regressão que ampliasse
#              o âncoramento por engano (ex.: alguém adiciona
#              filename_uniqueness a credentialGuardAnchoredRules "só por
#              via das dúvidas") passaria por TODOS os outros cenários deste
#              arquivo em silêncio.
#
# HEAD real é obrigatório aqui, mesmo esta regra nunca consultando HEAD
# hoje: sem HEAD, credentialGuardRuleSeverity cairia direto em
# diskSeverity mesmo que a regra FOSSE (por engano) adicionada ao mapa
# âncorado — mascarando exatamente o vazamento que este cenário existe
# para pegar (headTrackfwYAML retornando ok=false é um dos "sem âncora,
# cai no disco" do ADR §Decision point 4 — ver credentialGuardRuleSeverity,
# validator_credential_guard_integrity.go:252-270). Por isso T53 commita um
# trackfw.yaml (sem `rules:`, sem credential_guard — irrelevante para esta
# regra) via o MESMO s50_commit_fixture usado acima.
#
# Braço baseline (violação DEVE aparecer, sem override): T53_BASE só
# commita o scaffold padrão e cria os mesmos dois "dup.md" do Cenário 52 —
# filename_uniqueness não tem entrada em ruleDefaults (validator.go:101-109
# — só note_orphan e credential_guard_script_integrity estão lá), então o
# default é "error" e a violação derruba o exit code sem precisar de
# `rules:` no fixture (diferente do Cenário 49 — ver aviso da armadilha #2
# no despacho).
S53_BASE_HEAD="$(cat <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF
)"

T53_BASE="$WORK/s53-non-guard-baseline"
s50_commit_fixture "$T53_BASE" "$S53_BASE_HEAD" \
  "trackfw.yaml without any rules: override"
printf '# dup\n' > "$T53_BASE/docs/roadmaps/backlog/dup.md"
printf '# dup\n' > "$T53_BASE/docs/roadmaps/done/dup.md"

assert_fails_with "credential-guard-anchoring-non-regression/filename-uniqueness-baseline" \
  "$S52_FILENAME_MSG" \
  bash -c "cd '$T53_BASE' && exec '$ROOT_DIR/bin/trackfw' validate"

# Braço de detecção (silêncio esperado): MESMO HEAD (sem rules: commitado),
# MESMA divergência de dup.md — mas agora o disco acrescenta, SEM commit,
# `rules: filename_uniqueness: off`. Como esta regra NÃO está em
# credentialGuardAnchoredRules, ruleSeverity() (validator.go:111-128) usa
# diskRuleSeverity() puro — o mesmo caminho de SEMPRE, alheio a HEAD — e a
# violação deve sumir por completo (exit 0), provando que o M4 não alterou
# este caminho. -----------------------------------------------------------
T53_OFF="$WORK/s53-non-guard-off-uncommitted"
s50_commit_fixture "$T53_OFF" "$S53_BASE_HEAD" \
  "trackfw.yaml without any rules: override"
printf '# dup\n' > "$T53_OFF/docs/roadmaps/backlog/dup.md"
printf '# dup\n' > "$T53_OFF/docs/roadmaps/done/dup.md"
{
  printf '%s\n' "$S53_BASE_HEAD"
  printf 'rules:\n  filename_uniqueness: off\n'
} > "$T53_OFF/trackfw.yaml"

assert_lacks_pattern "credential-guard-anchoring-non-regression/filename-uniqueness-off-uncommitted-still-silences" \
  "$S52_FILENAME_MSG" \
  bash -c "cd '$T53_OFF' && exec '$ROOT_DIR/bin/trackfw' validate"

# 🔴 Prova de não-vacuidade: sabotagem TEMPORÁRIA e NÃO commitada de
# internal/validator/validator_credential_guard_integrity.go — acrescentar
# "filename_uniqueness": true ao mapa credentialGuardAnchoredRules
# (simulando um vazamento de escopo do M4 para uma regra comum),
# reconstruir bin/trackfw e rodar T53_OFF de novo: o braço acima DEVE
# passar a FALHAR (a mensagem volta a aparecer, exit != 0), porque
# credentialGuardRuleSeverity entraria em jogo — HEAD (sem `rules:` para
# filename_uniqueness) resolveria para o default "error", que venceria o
# "off" do disco pela comparação "mais estrita", exatamente o vazamento que
# este cenário existe para detectar. Restaurar o arquivo e reconstruir
# antes de prosseguir. Saída colada no relatório final desta execução.
#
# Escolha da regra: filename_uniqueness é a mesma usada como controle no
# Cenário 52 (reaproveita $S52_FILENAME_MSG e o fixture "dup.md"), tem
# default "error" (assert_fails_with exige exit != 0 — armadilha #2 do
# despacho, evitada de propósito: um default "warning" não derrubaria o
# exit code e a prova de baseline seria vácua).
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 54 — internal/validator: o bypass por variáveis de ambiente GIT_*
#              (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-
#              credential-guard, ML-1B/ML-2B) CONTINUA fechado — nenhum dos
#              dois vetores achados pelo ML-3B (Hades) e reproduzidos por
#              Zeus consegue derrotar o M4 herdando GIT_* do processo pai.
#
# Os dois vetores (vault/notes/validador-git-env-bypass-filtre-por-prefixo-
# 2026-08-12.md): (1) REDIRECIONAMENTO — GIT_DIR/GIT_WORK_TREE apontando
# para outro repositório git, fazendo headTrackfwYAML() ler o HEAD ERRADO;
# (2) FALHA INDUZIDA — GIT_CONFIG_COUNT=abc, que não redireciona nada, só
# faz o subprocesso git sair 128 por config malformada em linha de comando
# — e headTrackfwYAML() trata QUALQUER falha do git como "sem âncora,
# silêncio", então basta fazer o git falhar por QUALQUER motivo. Cobrir só
# o vetor (1) deixaria uma regressão de volta para uma denylist enumerada
# (o erro de enquadramento original que não fechava o problema) passar
# despercebida — ver a nota acima, "O erro de enquadramento".
#
# RETARGET: se gitCommand()/cleanGitEnv() (internal/validator/
# validator_git_exec.go) mudarem de arquivo, assinatura ou deixarem de ser
# o único ponto de invocação de git deste pacote, reaponte este cenário — o
# invariante que importa é "nenhuma chamada de git deste pacote herda
# GIT_* do processo pai", não a função específica.
#
# Fixture: MESMO ataque combinado do Cenário 51 (T51_BAD) — HEAD commita
# SÓ credential_guard.mode: block (sem rules:), disco (sem novo commit)
# sobrescreve para mode: warn + rules: credential_guard_mode_downgrade: off
# — mas agora rodado com as variáveis de ambiente GIT_* do adversário
# injetadas no processo. Se cleanGitEnv() regredisse (parasse de filtrar,
# ou voltasse a ser denylist enumerada), qualquer um dos dois braços
# abaixo silenciaria a regra por completo — reabrindo o furo que este
# roadmap inteiro combate.
#
# Cobertura da tabela de reprodução manual de Zeus (despacho deste ML — 4
# linhas: "sem manipulação", "GIT_DIR + GIT_WORK_TREE", "GIT_CONFIG_COUNT=abc",
# "GIT_CEILING_DIRECTORIES", todas reportando violação POST-fix, ou seja: a
# tabela de Zeus é verificação de que a defesa segura vale nas 4 condições,
# não uma lista de vetores pré-fix): "sem manipulação" já é o braço de
# detecção do Cenário 51 (T51_BAD, fixture idêntico, sem env injetado) — não
# duplicado aqui de propósito. GIT_DIR/GIT_WORK_TREE e GIT_CONFIG_COUNT são
# os dois braços abaixo. GIT_CEILING_DIRECTORIES foi investigado e
# DELIBERADAMENTE NÃO virou um terceiro braço de detecção: testado cru
# contra `git -C <dir>` com <dir> já sendo a raiz do repo (a MESMA forma que
# gitCommand(".", ...) sempre usa neste código-base — cwd já é a raiz do
# projeto/repo, nunca um subdiretório) — o ceiling só bloqueia caminhada
# PARA CIMA na descoberta, e uma descoberta que já começa num diretório com
# `.git` não caminha para lugar nenhum. Confirmado empiricamente com/sem
# `cleanGitEnv()` sabotado, ceiling = o próprio <dir> e ceiling = o pai de
# <dir>: as duas vezes `git show HEAD:./f.yaml` teve sucesso normal, exit 0.
# Incluir um assert_fails_with para essa variável seria vácuo — o próprio
# controle autodiscriminante embutido (braço "-is-real" abaixo, aplicado ao
# mesmo padrão) rejeitaria a inclusão. Reportado a Zeus; se a tabela dele
# reproduziu um bypass real via GIT_CEILING_DIRECTORIES, foi contra uma
# forma de invocação diferente desta (ex.: cwd num subdiretório do repo),
# que não é como este código-base invoca git hoje.
#
# Limite de escopo: este cenário exercita só a limpeza de ambiente
# (cleanGitEnv() removendo GIT_*), NÃO o ancoramento `-C dir` em si — contra
# ESTE fixture (cwd == $T54 == raiz do repo), remover `-C` deixaria o
# cenário verde do mesmo jeito, porque o processo já está no diretório
# certo por padrão. O ancoramento `-C` é validado por leitura de código
# (gitCommand() sempre passa `-C dir` explícito, nunca confia só em cwd),
# não por um braço de falsificação dedicado neste arquivo.
S54_HEAD="$(s50_yaml_content block)"

T54="$WORK/s54-git-env-bypass"
s50_commit_fixture "$T54" "$S54_HEAD" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn off > "$T54/trackfw.yaml"

# Repositório-isca para o vetor de redirecionamento: um segundo repo git,
# em outro diretório, SEM trackfw.yaml commitado — se GIT_DIR/GIT_WORK_TREE
# vencessem o `-C $T54` explícito de gitCommand(), `git show
# HEAD:./trackfw.yaml` passaria a mirar este repo, onde o path não existe
# em HEAD, e headTrackfwYAML() retornaria ok=false — caindo no MESMO
# fallback disco-only que o Cenário 51 usa para provar o desligamento
# legítimo (mode: warn + rules: off, ambos só em disco aqui): a severidade
# silenciaria por inteiro.
T54_DECOY="$WORK/s54-decoy-repo"
mkdir -p "$T54_DECOY"
(
  cd "$T54_DECOY"
  git init -q
  git config user.email "falsify@trackfw.test"
  git config user.name "trackfw falsify"
  git config commit.gpgsign false
  git config core.hooksPath /dev/null
  printf 'decoy: true\n' > not-trackfw.yaml
  git add -A
  git commit -q -m "decoy repo without trackfw.yaml"
)

# Braço autodiscriminante embutido: prova, ANTES de testar o binário do
# trackfw, que o repositório-isca é um vetor de ataque genuíno contra um
# `git -C` cru — sem isto, um braço de detecção "verde" abaixo não
# provaria que cleanGitEnv()/`-C` é quem defende; provaria só que o
# ataque nunca funcionou contra nada.
set +e
s54_raw_out=$(GIT_DIR="$T54_DECOY/.git" GIT_WORK_TREE="$T54_DECOY" git -C "$T54" show HEAD:./trackfw.yaml 2>&1)
s54_raw_status=$?
set -e
if [[ $s54_raw_status -eq 0 ]] && grep -qF "mode: block" <<<"$s54_raw_out"; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/attack-inert]: GIT_DIR/GIT_WORK_TREE NÃO desviaram um \`git -C\` cru para o repositório-isca — o vetor de ataque em si está inerte neste ambiente, a prova abaixo não provaria nada" >&2
  echo "  output: $s54_raw_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-git-env-bypass/redirect-attack-is-real]: GIT_DIR/GIT_WORK_TREE realmente desviam um \`git -C\` cru (saiu $s54_raw_status, sem 'mode: block' do HEAD real) — confirma que o vetor é genuíno, não teatro"

set +e
s54_rawcfg_out=$(GIT_CONFIG_COUNT=abc git -C "$T54" rev-parse --is-inside-work-tree 2>&1)
s54_rawcfg_status=$?
set -e
if [[ $s54_rawcfg_status -eq 0 ]]; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/config-attack-inert]: GIT_CONFIG_COUNT=abc NÃO derrubou um \`git -C\` cru — o vetor de falha induzida está inerte neste ambiente, a prova abaixo não provaria nada" >&2
  echo "  output: $s54_rawcfg_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-git-env-bypass/config-attack-is-real]: GIT_CONFIG_COUNT=abc realmente derruba um \`git -C\` cru (saiu $s54_rawcfg_status) — confirma que o vetor é genuíno"

# Braço de detecção 1/2 — REDIRECIONAMENTO: mesmo GIT_DIR/GIT_WORK_TREE do
# repositório-isca acima, agora contra o binário trackfw. gitCommand()
# ancora com `-C $T54` e limpa GIT_* do ambiente do processo filho — se
# isso continuar funcionando, `git show HEAD:./trackfw.yaml` de
# headTrackfwYAML() lê o HEAD de $T54 (mode: block), "mais estrita entre
# HEAD e disco" vence sobre o disco (warn + off), e $S50_MSG aparece.
assert_fails_with "credential-guard-git-env-bypass/redirect-detected" \
  "$S50_MSG" \
  bash -c "cd '$T54' && GIT_DIR='$T54_DECOY/.git' GIT_WORK_TREE='$T54_DECOY' exec '$ROOT_DIR/bin/trackfw' validate"

# Braço de detecção 2/2 — FALHA INDUZIDA: GIT_CONFIG_COUNT=abc não
# redireciona nada, só faz o subprocesso git sair 128 por config malformada
# em linha de comando — o vetor que quebrou a denylist original de 8 nomes.
# cleanGitEnv() precisa removê-la do ambiente do processo filho pelo MESMO
# mecanismo (prefixo GIT_), sem tratamento especial por nome de variável.
assert_fails_with "credential-guard-git-env-bypass/config-count-detected" \
  "$S50_MSG" \
  bash -c "cd '$T54' && GIT_CONFIG_COUNT=abc exec '$ROOT_DIR/bin/trackfw' validate"

# Braço "worktree legítimo continua funcionando" — git worktree add cria
# uma segunda working tree cujo .git é um ARQUIVO (não diretório) apontando
# de volta para o gitdir principal; o comentário de validator_git_exec.go
# promete que isso continua resolvendo o MESMO repositório porque a
# descoberta normal a partir de `-C dir` já chega lá sem depender de nada
# herdado. Prova aqui SEM injetar GIT_* manualmente (o cenário decisivo de
# bypass já está provado acima) — só confirma que `-C` para dentro de uma
# worktree vinculada funciona no caminho feliz, nos dois sentidos (baseline
# silenciosa e detecção).
T54_WT_MAIN="$WORK/s54-worktree-main"
s50_commit_fixture "$T54_WT_MAIN" "$S54_HEAD" \
  "trackfw.yaml with credential_guard.mode: block"
T54_WT_LINKED="$WORK/s54-worktree-linked"
(cd "$T54_WT_MAIN" && git worktree add -q -b s54-wt-branch "$T54_WT_LINKED")

set +e
s54wt_ok_out=$(cd "$T54_WT_LINKED" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s54wt_ok_status=$?
set -e
if [[ $s54wt_ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/worktree-baseline]: worktree vinculada com disco == HEAD (mode: block) deveria passar, saiu com $s54wt_ok_status" >&2
  echo "  output: $s54wt_ok_out" >&2
  exit 1
fi
if grep -qF "$S50_MSG" <<<"$s54wt_ok_out"; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/worktree-baseline]: worktree vinculada com disco == HEAD, mas a regra disparou mesmo assim" >&2
  echo "  output: $s54wt_ok_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-git-env-bypass/worktree-legitimate-baseline]"

s50_yaml_content warn > "$T54_WT_LINKED/trackfw.yaml"
assert_fails_with "credential-guard-git-env-bypass/worktree-legitimate-detection" \
  "$S50_MSG" \
  bash -c "cd '$T54_WT_LINKED' && exec '$ROOT_DIR/bin/trackfw' validate"

# 🔴 Prova de não-vacuidade: sabotagem TEMPORÁRIA e NÃO commitada de
# internal/validator/validator_git_exec.go — dentro de cleanGitEnv(),
# trocar o corpo por `return os.Environ()` (i.e., voltar a herdar GIT_* do
# processo pai sem filtro nenhum, o comportamento PRÉ-ML-1B), reconstruir
# bin/trackfw (vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-
# falsify-2026-08-12.md — go build ./... sozinho NÃO regenera esse
# binário) e rodar os dois braços de detecção acima de novo: AMBOS devem
# passar a FALHAR (mensagem ausente, exit 0) — o redirecionamento faz
# headTrackfwYAML() ler o repositório-isca (sem trackfw.yaml em HEAD,
# ok=false) e a falha induzida faz `git show HEAD:./trackfw.yaml` sair 128
# (ok=false também) — os dois caem no fallback disco-only, onde `rules:
# credential_guard_mode_downgrade: off` (só em disco, não commitado)
# silencia a regra por inteiro. Restaurar o arquivo e reconstruir antes de
# prosseguir — este ML não tem permissão de deixar internal/ tocado no
# diff final; a sabotagem é só para a prova de auditoria, nunca commitada.
# Saída colada no relatório final desta execução.
#
# Limite de cobertura conhecido (mesmo do Cenário 50/51): cobre só o wiring
# applyRule (Validate()/texto), não applyRuleTagged (ValidateTagged()/
# `validate --json`) em internal/validator/validator.go.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenários 55/56/57 — check-unknown-command-parity.sh (criado pelo ML-2A,
# ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw, sem prova de
# falsificação — lacuna reportada pelo próprio ML-2A) — cada cenário sabota UM
# CLI por vez, prova que o gate reprova (braço de detecção) e prova que o
# ciclo LIMPO passa (braço de linha de base), fechando o P4 de
# docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md que
# faltava para este gate.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 55 — divergência de TEXTO: Python remove o sufixo ` for
#              "{cmd_path}"` da mensagem canônica (format_unknown_command_error,
#              pypi/trackfw/unknown_command.py) → o próprio guard de
#              vacuidade do gate no cenário (a) "no-suggestion" ("canonical
#              message missing") detecta a divergência, antes mesmo de chegar
#              em assert_three_way.
#
# Seam: corrupt_literal na IMPLEMENTAÇÃO (unknown_command.py), nunca na
# asserção do gate — mesmo padrão dos Cenários 14/16/17/20/25/42.
# ---------------------------------------------------------------------------
T55_BASE="$WORK/s55-base"
mkdir -p "$T55_BASE/scripts"
setup_npm_tree "$T55_BASE"
cp -r "$ROOT_DIR/pypi" "$T55_BASE/pypi"
cp "$ROOT_DIR/scripts/check-unknown-command-parity.sh" "$T55_BASE/scripts/"

assert_succeeds "unknown-command-parity/text-drift/python-baseline" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T55_BASE/scripts/check-unknown-command-parity.sh"

T55="$WORK/s55"
mkdir -p "$T55/scripts"
setup_npm_tree "$T55"
cp -r "$ROOT_DIR/pypi" "$T55/pypi"
cp "$ROOT_DIR/scripts/check-unknown-command-parity.sh" "$T55/scripts/"

corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/unknown_command.py" "$T55/pypi/trackfw/unknown_command.py" \
  "lines = [f'Error: unknown command \"{typed}\" for \"{cmd_path}\"']" \
  "lines = [f'Error: unknown command \"{typed}\"']" \
  "s55-python-text-drift"

assert_fails_with "unknown-command-parity/text-drift/python-detects-regression" \
  "canonical message missing" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T55/scripts/check-unknown-command-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 56 — divergência de EXIT CODE: Node.js troca `process.exit(1)` por
#              `process.exit(3)` no listener `command:*`
#              (npm/src/commands/index.js) → assert_three_way's exit-code
#              check ("exit codes diverge") detecta a divergência no primeiro
#              cenário exercitado ("no-suggestion").
#
# Seam: corrupt_literal na IMPLEMENTAÇÃO (index.js), nunca na asserção do
# gate.
# ---------------------------------------------------------------------------
T56_BASE="$WORK/s56-base"
mkdir -p "$T56_BASE/scripts"
setup_npm_tree "$T56_BASE"
cp -r "$ROOT_DIR/pypi" "$T56_BASE/pypi"
cp "$ROOT_DIR/scripts/check-unknown-command-parity.sh" "$T56_BASE/scripts/"

assert_succeeds "unknown-command-parity/exit-code-drift/node-baseline" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T56_BASE/scripts/check-unknown-command-parity.sh"

T56="$WORK/s56"
mkdir -p "$T56/scripts"
setup_npm_tree "$T56"
cp -r "$ROOT_DIR/pypi" "$T56/pypi"
cp "$ROOT_DIR/scripts/check-unknown-command-parity.sh" "$T56/scripts/"

corrupt_literal \
  "$ROOT_DIR/npm/src/commands/index.js" "$T56/npm/src/commands/index.js" \
  'process.exit(1)' \
  'process.exit(3)' \
  "s56-node-exit-code-drift"

assert_fails_with "unknown-command-parity/exit-code-drift/node-detects-regression" \
  "exit codes diverge" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T56/scripts/check-unknown-command-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 57 — SUGESTÃO ausente: Go deixa de emitir a linha `Did you mean
#              "..."?` — a condição `found` de formatUnknownCommandError
#              (internal/commands/root.go) é forçada a nunca disparar
#              (`found && false`, mantendo `found` referenciado para não
#              quebrar `go build` com "declared and not used") → o guard de
#              vacuidade do gate no cenário (b) "with-suggestion" ("vacuity
#              guard: expected suggestion 'validate' missing") detecta a
#              divergência para o runtime go.
#
# Requer rebuild de um binário Go isolado (mesmo padrão dos Cenários 25/26):
# a prova não pode depender de `make build` já ter rodado com a corrupção.
# Seam: corrupt_literal na IMPLEMENTAÇÃO (root.go), nunca na asserção do gate.
# ---------------------------------------------------------------------------
T57G_BASE_BIN="$WORK/s57-go-base-bin/trackfw"
mkdir -p "$(dirname "$T57G_BASE_BIN")"
build_go_or_fail "setup-s57-go-baseline-build" "$ROOT_DIR" "$T57G_BASE_BIN"

T57_BASE="$WORK/s57-base"
mkdir -p "$T57_BASE/scripts"
setup_npm_tree "$T57_BASE"
cp -r "$ROOT_DIR/pypi" "$T57_BASE/pypi"
cp "$ROOT_DIR/scripts/check-unknown-command-parity.sh" "$T57_BASE/scripts/"

assert_succeeds "unknown-command-parity/missing-suggestion/go-baseline" \
  env GO_BIN="$T57G_BASE_BIN" bash "$T57_BASE/scripts/check-unknown-command-parity.sh"

T57G_MOD="$WORK/s57-go-mod"
mkdir -p "$T57G_MOD/cmd" "$T57G_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T57G_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T57G_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T57G_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T57G_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/commands/root.go" "$T57G_MOD/internal/commands/root.go" \
  'unknownCommandCandidates(root)); found {' \
  'unknownCommandCandidates(root)); found && false {' \
  "s57-go-suggestion-suppressed"

T57G_BIN="$WORK/s57-go-bin/trackfw"
mkdir -p "$(dirname "$T57G_BIN")"
build_go_or_fail "setup-s57-go-build" "$T57G_MOD" "$T57G_BIN"

T57="$WORK/s57"
mkdir -p "$T57/scripts"
setup_npm_tree "$T57"
cp -r "$ROOT_DIR/pypi" "$T57/pypi"
cp "$ROOT_DIR/scripts/check-unknown-command-parity.sh" "$T57/scripts/"

assert_fails_with "unknown-command-parity/missing-suggestion/go-detects-regression" \
  "vacuity guard: expected suggestion 'validate' missing" \
  env GO_BIN="$T57G_BIN" bash "$T57/scripts/check-unknown-command-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 58 — ML-1A da REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-
# stack-trace-caminhos-absolutos-e-versao-do-runtime: o handler global de
# erro no entrypoint Node (npm/bin/trackfw) e no Python (trackfw.cli:main)
# realmente impede que um erro não tratado vaze stack trace, caminho
# absoluto de instalação e versão do runtime — e que remover o handler faz
# o vazamento reaparecer, provando que o handler (não uma coincidência de
# mensagem já limpa) é o mecanismo que fecha o gap.
#
# Node: fixture REAL — `trackfw agents install` seguido de adulteração do
# manifesto + do artefato instalado, depois `trackfw agents update --force`,
# que faz IntegrationManager.preflight lançar `Unmanaged artifact does not
# match a trackfw template` (manager.js:189) sem NENHUM try/catch no
# caminho — exatamente o vazamento medido na REQ.
#   - baseline: bin/trackfw atual (parseAsync().catch(reportFatalError)) →
#     stderr limpo, sem frames de stack, sem "npm/src/", sem "Node.js vX".
#   - detecção: cópia isolada com bin/trackfw REVERTIDO para a forma
#     pré-fix (`require(...).parseAsync(process.argv)`, sem .catch) →
#     stderr volta a vazar a stack completa e a versão do runtime.
#
# Python: fixture SINTÉTICA — `roadmap list` não tinha, e continua sem ter,
# nenhum try/except cobrindo cfg_module.load()/os.path.isdir
# (commands/roadmap.py:_cmd_list); a REQ documenta que hoje nenhum caminho
# Python vaza NOS CAMINHOS TESTADOS, então este cenário injeta um raise
# determinístico em _cmd_list para provar a defesa em profundidade em
# QUALQUER caminho, testado ou não. roadmap.py é corrompido IGUAL nos dois
# braços — o único delta entre baseline e detecção é o handler em cli.py.
#   - baseline: cli.py atual (try/except em torno de args.func) → stderr
#     limpo "trackfw roadmap: ...", sem "Traceback".
#   - detecção: cópia isolada com cli.py REVERTIDO para a forma pré-fix
#     (chamada direta a args.func(args), sem try/except) → "Traceback"
#     reaparece.
#
# Seam: corrupt_literal na IMPLEMENTAÇÃO (bin/trackfw, cli.py), nunca na
# asserção.
# ---------------------------------------------------------------------------

# --- Node: baseline via o bin/trackfw REAL (é o próprio fix sob teste) -----
S58N_BASE_ROOT="$WORK/s58-node-base"
S58N_BASE_PROJECT="$S58N_BASE_ROOT/project"
S58N_BASE_HOME="$S58N_BASE_ROOT/home"
mkdir -p "$S58N_BASE_PROJECT" "$S58N_BASE_HOME"
(cd "$S58N_BASE_PROJECT" && HOME="$S58N_BASE_HOME" node "$ROOT_DIR/npm/bin/trackfw" \
  agents install --scope project --targets codex --items iac >/dev/null 2>&1)
python3 - "$S58N_BASE_PROJECT" <<'PY'
import json, sys, pathlib
project = pathlib.Path(sys.argv[1])
manifest_path = project / ".trackfw/integrations-manifest.json"
manifest = json.loads(manifest_path.read_text())
key = next(k for k in manifest["artifacts"] if "iac" in k)
del manifest["artifacts"][key]
manifest_path.write_text(json.dumps(manifest))
PY
printf '\n# tampered\n' >> "$S58N_BASE_PROJECT/.codex/agents/trackfw-iac.toml"

set +e
s58n_base_out=$(cd "$S58N_BASE_PROJECT" && HOME="$S58N_BASE_HOME" node "$ROOT_DIR/npm/bin/trackfw" \
  agents update --force --scope project --targets codex --items iac 2>&1)
s58n_base_status=$?
set -e

if [[ "$s58n_base_status" -eq 0 ]]; then
  echo "FAIL [falsify/fatal-error-handler/node-baseline]: exit 0 inesperado — fixture não disparou o erro esperado" >&2
  echo "  output: $(printf '%q' "$s58n_base_out")" >&2
  exit 1
fi
if grep -qF "    at " <<<"$s58n_base_out" || grep -qF "npm/src/" <<<"$s58n_base_out" || grep -Eq 'Node\.js v[0-9]' <<<"$s58n_base_out"; then
  echo "FAIL [falsify/fatal-error-handler/node-baseline]: stderr do bin/trackfw REAL ainda vaza stack/caminho de instalação/versão do runtime" >&2
  echo "  output: $(printf '%q' "$s58n_base_out")" >&2
  exit 1
fi
echo "OK   [falsify/fatal-error-handler/node-baseline]"

# --- Node: detecção — bin/trackfw revertido para a forma pré-fix -----------
# single-delta: o `new` abaixo reverte SÓ o handler global (installGlobalHandlers
# + .catch/reportFatalError); a interceptação de zero-argumento do ML-1C
# (ROADMAP-2026-08-16-higiene-...) é preservada tal qual está no bin/trackfw
# atual — senão este cenário provaria duas regressões ao mesmo tempo em vez de
# isolar o handler de erro como única variável.
S58N_MOD_ROOT="$WORK/s58-node-mod"
setup_npm_tree "$S58N_MOD_ROOT"
corrupt_literal \
  "$ROOT_DIR/npm/bin/trackfw" "$S58N_MOD_ROOT/npm/bin/trackfw" \
  $'#!/usr/bin/env node\n\'use strict\'\n\nconst { reportFatalError, installGlobalHandlers } = require(\'../src/lib/fatal-error\')\n\n// Registered before any command runs — see fatal-error.js for why this is a\n// single entrypoint-level handler instead of a try/catch per command.\ninstallGlobalHandlers()\n\nconst program = require(\'../src/commands/index\').createProgram()\n\n// trackfw sem argumento é uso legítimo (pedir ajuda), não erro — decisão do\n// arquiteto no ML-1C (ROADMAP-2026-08-16-higiene-...). Sem intervenção,\n// commander trata "nenhum operando" como "provavelmente faltou um\n// subcomando" (Command._parseCommand: `this.commands.length && this.args\n// .length === 0 && !this._actionHandler` → `this.help({error: true})`) —\n// help em stderr, exit 1. Interceptar SÓ o caso de zero argumentos aqui,\n// antes de entrar no parser, reusa o MESMO texto de ajuda (outputHelp) na\n// saída canônica (stdout, exit 0) sem tocar no parsing normal — em\n// particular sem registrar uma .action() no root, que reclassificaria um\n// comando desconhecido (ex.: "trackfw naoexiste") como argumento posicional\n// da action em vez de disparar o listener \'command:*\' (regressão descartada\n// em favor desta abordagem: ver ROADMAP ML-1C).\n//\n// Fica ANTES do parseAsync e DEPOIS de installGlobalHandlers(): o caminho de\n// zero argumentos é sucesso e nunca deve passar pelo reportFatalError abaixo.\nif (process.argv.length <= 2) {\n  program.outputHelp()\n  process.exit(0)\n}\n\nprogram\n  .parseAsync(process.argv)\n  .catch((err) => {\n    // The primary path: an action (sync or async) threw, and since the\n    // program is driven by parseAsync() that surfaces here as a rejected\n    // promise — not as an uncaughtException/unhandledRejection, which is\n    // why this .catch exists in addition to installGlobalHandlers() above.\n    reportFatalError(err)\n    process.exitCode = 1\n  })\n' \
  $'#!/usr/bin/env node\n\'use strict\'\n\nconst program = require(\'../src/commands/index\').createProgram()\n\n// trackfw sem argumento é uso legítimo (pedir ajuda), não erro — decisão do\n// arquiteto no ML-1C (ROADMAP-2026-08-16-higiene-...). Sem intervenção,\n// commander trata "nenhum operando" como "provavelmente faltou um\n// subcomando" (Command._parseCommand: `this.commands.length && this.args\n// .length === 0 && !this._actionHandler` → `this.help({error: true})`) —\n// help em stderr, exit 1. Interceptar SÓ o caso de zero argumentos aqui,\n// antes de entrar no parser, reusa o MESMO texto de ajuda (outputHelp) na\n// saída canônica (stdout, exit 0) sem tocar no parsing normal — em\n// particular sem registrar uma .action() no root, que reclassificaria um\n// comando desconhecido (ex.: "trackfw naoexiste") como argumento posicional\n// da action em vez de disparar o listener \'command:*\' (regressão descartada\n// em favor desta abordagem: ver ROADMAP ML-1C).\n//\n// Fica ANTES do parseAsync e DEPOIS de installGlobalHandlers(): o caminho de\n// zero argumentos é sucesso e nunca deve passar pelo reportFatalError abaixo.\nif (process.argv.length <= 2) {\n  program.outputHelp()\n  process.exit(0)\n}\n\nprogram.parseAsync(process.argv)\n' \
  "s58-node-revert-fatal-handler"

S58N_MOD_PROJECT="$WORK/s58-node-mod-project"
S58N_MOD_HOME="$WORK/s58-node-mod-home"
mkdir -p "$S58N_MOD_PROJECT" "$S58N_MOD_HOME"
(cd "$S58N_MOD_PROJECT" && HOME="$S58N_MOD_HOME" node "$S58N_MOD_ROOT/npm/bin/trackfw" \
  agents install --scope project --targets codex --items iac >/dev/null 2>&1)
python3 - "$S58N_MOD_PROJECT" <<'PY'
import json, sys, pathlib
project = pathlib.Path(sys.argv[1])
manifest_path = project / ".trackfw/integrations-manifest.json"
manifest = json.loads(manifest_path.read_text())
key = next(k for k in manifest["artifacts"] if "iac" in k)
del manifest["artifacts"][key]
manifest_path.write_text(json.dumps(manifest))
PY
printf '\n# tampered\n' >> "$S58N_MOD_PROJECT/.codex/agents/trackfw-iac.toml"

set +e
s58n_mod_out=$(cd "$S58N_MOD_PROJECT" && HOME="$S58N_MOD_HOME" node "$S58N_MOD_ROOT/npm/bin/trackfw" \
  agents update --force --scope project --targets codex --items iac 2>&1)
s58n_mod_status=$?
set -e

if grep -qF "    at " <<<"$s58n_mod_out" && grep -Eq 'Node\.js v[0-9]' <<<"$s58n_mod_out"; then
  echo "OK   [falsify/fatal-error-handler/node-detects-regression]"
else
  echo "FAIL [falsify/fatal-error-handler/node-detects-regression]: bin/trackfw revertido não vazou stack/versão do runtime — braço de detecção vácuo" >&2
  echo "  status=$s58n_mod_status output: $(printf '%q' "$s58n_mod_out")" >&2
  exit 1
fi

# --- Go: baseline via o binário isolado já construído para os Cenários
# 27+ ($T27_GO_BIN) — a REQ mede Go como "não vaza" (cobra trata o erro sem
# handler dedicado), mas nenhum gate travava isso; sem um braço aqui, uma
# mudança futura em cobra/SilenceErrors poderia reintroduzir o vazamento na
# terceira linguagem sem que "nos 3 CLIs" da REQ/roadmap fosse honrado. Sem
# braço de detecção: nenhum código Go foi tocado por este ML, não há
# regressão own-code para provar — só o baseline precisa ficar travado.
S58G_PROJECT="$WORK/s58-go-project"
S58G_HOME="$WORK/s58-go-home"
mkdir -p "$S58G_PROJECT" "$S58G_HOME"
(cd "$S58G_PROJECT" && HOME="$S58G_HOME" "$T27_GO_BIN" \
  agents install --scope project --targets codex --items iac >/dev/null 2>&1)
python3 - "$S58G_PROJECT" <<'PY'
import json, sys, pathlib
project = pathlib.Path(sys.argv[1])
manifest_path = project / ".trackfw/integrations-manifest.json"
manifest = json.loads(manifest_path.read_text())
key = next(k for k in manifest["artifacts"] if "iac" in k)
del manifest["artifacts"][key]
manifest_path.write_text(json.dumps(manifest))
PY
printf '\n# tampered\n' >> "$S58G_PROJECT/.codex/agents/trackfw-iac.toml"

set +e
s58g_out=$(cd "$S58G_PROJECT" && HOME="$S58G_HOME" "$T27_GO_BIN" \
  agents update --force --scope project --targets codex --items iac 2>&1)
s58g_status=$?
set -e

if [[ "$s58g_status" -eq 0 ]]; then
  echo "FAIL [falsify/fatal-error-handler/go-baseline]: exit 0 inesperado — fixture não disparou o erro esperado" >&2
  echo "  output: $(printf '%q' "$s58g_out")" >&2
  exit 1
fi
if grep -qF "panic:" <<<"$s58g_out" || grep -qF "goroutine " <<<"$s58g_out" || grep -Eq '\.go:[0-9]+' <<<"$s58g_out"; then
  echo "FAIL [falsify/fatal-error-handler/go-baseline]: stderr do binário Go vaza panic/goroutine/linha de fonte .go:N" >&2
  echo "  output: $(printf '%q' "$s58g_out")" >&2
  exit 1
fi
echo "OK   [falsify/fatal-error-handler/go-baseline]"

# --- Python: fixture comum — _cmd_list corrompido para lançar sem try/except
S58P_ROADMAP_OLD='def _cmd_list(args):
    cfg = cfg_module.load()'
S58P_ROADMAP_NEW='def _cmd_list(args):
    raise RuntimeError(f"synthetic uncaught error at {__file__}")
    cfg = cfg_module.load()'

# --- Python: baseline — cli.py atual (com o try/except) --------------------
S58P_BASE_PYPI="$WORK/s58-py-base/pypi"
mkdir -p "$S58P_BASE_PYPI"
cp -r "$ROOT_DIR/pypi/." "$S58P_BASE_PYPI/"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/roadmap.py" "$S58P_BASE_PYPI/trackfw/commands/roadmap.py" \
  "$S58P_ROADMAP_OLD" "$S58P_ROADMAP_NEW" "s58-py-base-roadmap-list-raise"

set +e
s58p_base_out=$(env PYTHONPATH="$S58P_BASE_PYPI" python3 -m trackfw roadmap list 2>&1)
s58p_base_status=$?
set -e

if [[ "$s58p_base_status" -eq 0 ]]; then
  echo "FAIL [falsify/fatal-error-handler/python-baseline]: exit 0 inesperado — fixture não disparou o erro esperado" >&2
  echo "  output: $(printf '%q' "$s58p_base_out")" >&2
  exit 1
fi
if [[ "$s58p_base_out" != "trackfw roadmap: synthetic uncaught error at "* ]] || grep -qF 'Traceback' <<<"$s58p_base_out"; then
  echo "FAIL [falsify/fatal-error-handler/python-baseline]: cli.py atual ainda vaza traceback, ou a mensagem limpa mudou de forma" >&2
  echo "  output: $(printf '%q' "$s58p_base_out")" >&2
  exit 1
fi
echo "OK   [falsify/fatal-error-handler/python-baseline]"

# --- Python: detecção — cli.py revertido para a forma pré-fix --------------
S58P_MOD_PYPI="$WORK/s58-py-mod/pypi"
mkdir -p "$S58P_MOD_PYPI"
cp -r "$ROOT_DIR/pypi/." "$S58P_MOD_PYPI/"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/roadmap.py" "$S58P_MOD_PYPI/trackfw/commands/roadmap.py" \
  "$S58P_ROADMAP_OLD" "$S58P_ROADMAP_NEW" "s58-py-mod-roadmap-list-raise"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/cli.py" "$S58P_MOD_PYPI/trackfw/cli.py" \
  $'    if hasattr(args, "func"):\n        # Global backstop (defense in depth, see fatal_error.py): commands that\n        # already catch their own domain errors (e.g. agents/skills install \xe2\x80\x94\n        # IntegrationError/OSError/ValueError in integrations/command.py:run())\n        # print their own clean message and raise SystemExit, which is a\n        # BaseException and therefore NOT caught here \xe2\x80\x94 it propagates\n        # unchanged. This except only catches what nothing else caught.\n        try:\n            args.func(args)\n        except Exception as error:  # noqa: BLE001 \xe2\x80\x94 intentional catch-all backstop\n            report_fatal_error(error, command=args.command)\n            sys.exit(1)\n    else:' \
  $'    if hasattr(args, "func"):\n        args.func(args)\n    else:' \
  "s58-py-revert-fatal-handler"

set +e
s58p_mod_out=$(env PYTHONPATH="$S58P_MOD_PYPI" python3 -m trackfw roadmap list 2>&1)
s58p_mod_status=$?
set -e

if [[ "$s58p_mod_status" -ne 0 ]] && grep -qF 'Traceback (most recent call last)' <<<"$s58p_mod_out"; then
  echo "OK   [falsify/fatal-error-handler/python-detects-regression]"
else
  echo "FAIL [falsify/fatal-error-handler/python-detects-regression]: cli.py revertido não vazou traceback — braço de detecção vácuo" >&2
  echo "  status=$s58p_mod_status output: $(printf '%q' "$s58p_mod_out")" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenário 59 — ML-1C da ROADMAP-2026-08-16-serve-amarra-em-loopback-por-
# padrao-com-opt-in-explicito-para-exposicao (REQ-2026-08-16-trackfw-serve-
# escuta-em-todas-as-interfaces-sem-autenticacao-expondo-a-cadeia-de-
# governanca-na-rede, AC5): check-serve-address-parity.sh (novo, criado por
# este ML) realmente reprova se o bind padrão voltar a wildcard — a própria
# regressão de segurança original desta REQ.
#
# Sabotagem: pypi/trackfw/commands/serve.py — `server_cls((host, port), ...)`
# vira `server_cls(("", port), ...)`, ou seja, o Python volta a ignorar
# completamente o `host` resolvido e escutar em todas as interfaces
# (INADDR_ANY), qualquer que seja `--host`. O braço "default-bind/py" do gate
# (que afirma `lsof` mostrando 127.0.0.1:PORT, nunca `*:PORT`) é quem detecta.
#
# Seam: corrupt_literal na IMPLEMENTAÇÃO (serve.py), nunca na asserção do
# gate — mesmo padrão dos Cenários 14/16/17/20/25/42/55/56.
# ---------------------------------------------------------------------------
T59_BASE="$WORK/s59-base"
mkdir -p "$T59_BASE/scripts"
setup_npm_tree "$T59_BASE"
cp -r "$ROOT_DIR/pypi" "$T59_BASE/pypi"
cp "$ROOT_DIR/scripts/check-serve-address-parity.sh" "$T59_BASE/scripts/"

assert_succeeds "serve-address-parity/wildcard-bind-regression/python-baseline" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T59_BASE/scripts/check-serve-address-parity.sh"

T59="$WORK/s59"
mkdir -p "$T59/scripts"
setup_npm_tree "$T59"
cp -r "$ROOT_DIR/pypi" "$T59/pypi"
cp "$ROOT_DIR/scripts/check-serve-address-parity.sh" "$T59/scripts/"

corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/serve.py" "$T59/pypi/trackfw/commands/serve.py" \
  'server = server_cls((host, port), handler_class)' \
  'server = server_cls(("", port), handler_class)' \
  "s59-python-wildcard-bind-regression"

assert_fails_with "serve-address-parity/wildcard-bind-regression/python-detects-regression" \
  "expected lsof to show 127.0.0.1:" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T59/scripts/check-serve-address-parity.sh"

# Cenários 60/61 — scripts/trackfw-git-branch-guard.sh (ML-1A,
# ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-
# da-release-7-0-0.md, itens 1 e 2). Cada cenário monta baseline (código
# LIMPO) + detecção (UM literal corrompido em internal/generators/scaffold.go
# via corrupt_literal, nunca na asserção) — mesmo padrão dos Cenários 55/56/57.
#
# ML-1A da ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-
# e-integridade-independente-de-fiacao.md acrescentou um no-op: o guard só
# bloqueia DENTRO de um projeto trackfw (trackfw.yaml em algum ancestral do
# cwd). assert_guard_exit invoca `bash "$script"` com o cwd AMBIENTE do
# processo do gate — antes desta ML isso não importava (o guard sempre
# bloqueava, em qualquer diretório). Sem tratar isso aqui, os Cenários 60-63
# ficariam silenciosamente dependentes do cwd de quem chama
# check-gates-falsify.sh: hoje (make quality roda da raiz do repo, que TEM
# trackfw.yaml) os braços baseline continuam passando pelo motivo certo, mas
# rodar o gate de outro diretório sem trackfw.yaml ancestral faria os braços
# baseline (que esperam exit 2) falhar porque o guard vira no-op — e pior,
# os braços de detecção (que também esperam exit 2 na forma corrompida)
# passariam de qualquer forma, mascarando uma regressão real do matcher.
# Fix: entra num diretório de fixture COM trackfw.yaml para toda a extensão
# dos Cenários 60-63 (todo assert_guard_exit deste bloco herda esse cwd via
# variável de shell, sem precisar mudar a assinatura do helper nem os ~30
# call sites) — cwd original restaurado logo antes do Cenário 64.
GBG_FIXTURE_DIR="$WORK/gbg-fixture-with-trackfw-yaml"
mkdir -p "$GBG_FIXTURE_DIR"
echo "project_name: gbg-fixture" > "$GBG_FIXTURE_DIR/trackfw.yaml"
GBG_ORIGINAL_PWD="$(pwd)"
cd "$GBG_FIXTURE_DIR"
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 60 — item 2 (brecha de contorno): `git switch -c` (forma
# alternativa a `checkout -b` para criar branch) deve ser bloqueado. Braço
# baseline prova que o guard LIMPO bloqueia; braço de detecção corrompe o
# case "-c|-C|--create|--create=*|--force-create|--force-create=*)" do
# gerador Go para um padrão que nunca casa, reconstrói o script a partir de
# um módulo Go isolado, e prova que SEM essa linha o mesmo comando escapa
# (exit 0 em vez de 2) — não-vacuidade: a detecção depende exatamente do
# literal que o ML-1A adicionou, não de qualquer outro efeito colateral.
# ---------------------------------------------------------------------------
T60_BASE_MOD="$WORK/s60-base-mod"
mkdir -p "$T60_BASE_MOD/cmd" "$T60_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T60_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T60_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T60_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T60_BASE_MOD/go.sum"

T60_BASE_OUT="$WORK/s60-base-out"
run_go_guard_dump "setup-s60-go-baseline-build" "$T60_BASE_MOD" "$T60_BASE_OUT"

assert_guard_exit "git-branch-guard/switch-c/baseline-blocks" \
  "$T60_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git switch -c feat/x"}}' \
  2

T60_MOD="$WORK/s60-mod"
mkdir -p "$T60_MOD/cmd" "$T60_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T60_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T60_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T60_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T60_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T60_MOD/internal/generators/scaffold.go" \
  '-c|-C|--create|--create=*|--force-create|--force-create=*)' \
  '--never-matches-anything-s58)' \
  "s60-go-switch-c-detection-removed"

T60_OUT="$WORK/s60-out"
run_go_guard_dump "setup-s60-go-corrupted-build" "$T60_MOD" "$T60_OUT"

assert_guard_exit "git-branch-guard/switch-c/detection-catches-bypass" \
  "$T60_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git switch -c feat/x"}}' \
  0

# ---------------------------------------------------------------------------
# Cenário 61 — item 1 (falso-positivo por prosa): uma mensagem de commit cuja
# linha COMEÇA com "git checkout -b" (reprodução literal do
# vault/notes/git-branch-guard-falso-positivo-em-linha-de-mensagem-de-commit-
# 2026-08-16.md) não deve bloquear. Braço baseline prova que o guard LIMPO
# permite; braço de detecção corrompe match_subcommand() para voltar a usar o
# `sed` cego antigo (em vez de strip_heredoc_bodies + quote_aware_split),
# reconstrói o script isolado, e prova que SEM a correção o mesmo comando
# volta a ser bloqueado — não-vacuidade: a permissividade depende exatamente
# do pré-processamento quote-aware introduzido por este ML, não de qualquer
# outro efeito colateral. A prova de não-regressão complementar (comandos
# reais encadeados atrás de um `-m` corretamente fechado continuam
# bloqueados) está em internal/generators/git_branch_guard_test.go
# (TestGitBranchGuard_ChainedCommand_SecondGitBlocked e as novas
# TestGitBranchGuard_QuotedMessageThenRealChainedCommand_StillBlocks/
# TestGitBranchGuard_SwitchDashC_Blocks/TestGitBranchGuard_SwitchWithoutCreateFlag_Allows).
# ---------------------------------------------------------------------------
# Reprodução literal do incidente real (vault note): `-m "$(cat <<'EOF' ...
# EOF)"` (convenção de mensagem de commit multi-linha do próprio CLAUDE.md
# deste repositório) — a quebra de linha REAL logo após `<<'EOF'` faz a
# primeira linha do corpo (`  git checkout -b ...`) virar, depois de
# stripar espaço à esquerda, seu PRÓPRIO segmento com "git" como primeiro
# token. Um `-m "..."` sem heredoc, sem quebra de linha antes de "git", não
# reproduz o bug (a linha inteira permanece grudada no segmento que começa
# com "bin/trackfw").
PROSE_PAYLOAD=$(python3 -c '
import json
cmd = ("bin/trackfw commit -m \"$(cat <<'"'"'EOF'"'"'\n"
       "  git checkout -b            -> bloqueado pelo guard\n"
       "  trackfw branch new chore/  -> recusado\n"
       "EOF\n"
       ")\"")
print(json.dumps({"tool_input": {"command": cmd}}))
')

T61_BASE_MOD="$WORK/s61-base-mod"
mkdir -p "$T61_BASE_MOD/cmd" "$T61_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T61_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T61_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T61_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T61_BASE_MOD/go.sum"

T61_BASE_OUT="$WORK/s61-base-out"
run_go_guard_dump "setup-s61-go-baseline-build" "$T61_BASE_MOD" "$T61_BASE_OUT"

assert_guard_exit "git-branch-guard/prose-in-message/baseline-allows" \
  "$T61_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  "$PROSE_PAYLOAD" \
  0

T61_MOD="$WORK/s61-mod"
mkdir -p "$T61_MOD/cmd" "$T61_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T61_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T61_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T61_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T61_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T61_MOD/internal/generators/scaffold.go" \
  'normalized=$(strip_heredoc_bodies "$1")
  normalized=$(quote_aware_split "$normalized")' \
  'normalized=$(printf '"'"'%s'"'"' "$1" | sed -e '"'"'s/&&/\n/g'"'"' -e '"'"'s/||/\n/g'"'"' -e '"'"'s/[;|]/\n/g'"'"')' \
  "s61-go-quote-aware-split-reverted"

T61_OUT="$WORK/s61-out"
run_go_guard_dump "setup-s61-go-corrupted-build" "$T61_MOD" "$T61_OUT"

assert_guard_exit "git-branch-guard/prose-in-message/detection-catches-regression" \
  "$T61_OUT/scripts/trackfw-git-branch-guard.sh" \
  "$PROSE_PAYLOAD" \
  2

# ---------------------------------------------------------------------------
# Cenário 62 — scripts/trackfw-git-branch-guard.sh (ML-4B,
# ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-
# da-release-7-0-0.md), corretivo do veredito BLOQUEAR do hades-tf
# (docs/seguranca/2026-08-16-revisao-do-git-branch-guard.md): duas evasões
# reproduzidas e fechadas nesse ML — prefixo `env`/`command` antes de `git`, e
# flag do `checkout -b` fora da primeira posição de token (`-q -b`,
# `--no-track -b`). Mesmo padrão baseline+detecção dos Cenários 60/61: braço
# baseline prova que o guard LIMPO bloqueia; braço de detecção corrompe UM
# literal isolado do gerador Go, reconstrói o script a partir de um módulo Go
# isolado, e prova que sem essa linha o mesmo comando escapa (exit 0 em vez
# de 2) — não-vacuidade: a detecção depende exatamente do literal que o
# ML-4B adicionou, não de qualquer outro efeito colateral.
# ---------------------------------------------------------------------------

# --- 62a — prefixo env/command antes de git ---------------------------------
T62A_BASE_MOD="$WORK/s62a-base-mod"
mkdir -p "$T62A_BASE_MOD/cmd" "$T62A_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62A_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62A_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62A_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62A_BASE_MOD/go.sum"

T62A_BASE_OUT="$WORK/s62a-base-out"
run_go_guard_dump "setup-s62a-go-baseline-build" "$T62A_BASE_MOD" "$T62A_BASE_OUT"

assert_guard_exit "git-branch-guard/env-command-prefix/baseline-blocks-env" \
  "$T62A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env git commit -m \"x\""}}' \
  2

assert_guard_exit "git-branch-guard/env-command-prefix/baseline-blocks-command" \
  "$T62A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"command git push"}}' \
  2

T62A_MOD="$WORK/s62a-mod"
mkdir -p "$T62A_MOD/cmd" "$T62A_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62A_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62A_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62A_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62A_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T62A_MOD/internal/generators/scaffold.go" \
  'while [ "$base" = "env" ] || [ "$base" = "command" ]; do' \
  'while [ "$base" = "__never-matches-s62a__" ]; do' \
  "s62a-go-env-command-prefix-stripping-removed"

T62A_OUT="$WORK/s62a-out"
run_go_guard_dump "setup-s62a-go-corrupted-build" "$T62A_MOD" "$T62A_OUT"

assert_guard_exit "git-branch-guard/env-command-prefix/detection-catches-bypass-env" \
  "$T62A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env git commit -m \"x\""}}' \
  0

assert_guard_exit "git-branch-guard/env-command-prefix/detection-catches-bypass-command" \
  "$T62A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"command git push"}}' \
  0

# Auto-discriminação: a corrupção acima é um `while` que nunca casa nenhum
# `base` — se ela silenciasse o guard inteiro (em vez de só o stripping de
# env/command), as duas asserções acima "provariam" bypass por um motivo
# errado. `git push` puro (sem prefixo) contra o MESMO build corrompido
# precisa continuar bloqueado — isola a corrupção ao stripping de
# env/command, não a uma quebra geral do matcher.
assert_guard_exit "git-branch-guard/env-command-prefix/detection-does-not-break-plain-push" \
  "$T62A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git push"}}' \
  2

# --- 62b — flag do checkout -b fora da primeira posição de token -----------
T62B_BASE_MOD="$WORK/s62b-base-mod"
mkdir -p "$T62B_BASE_MOD/cmd" "$T62B_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62B_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62B_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62B_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62B_BASE_MOD/go.sum"

T62B_BASE_OUT="$WORK/s62b-base-out"
run_go_guard_dump "setup-s62b-go-baseline-build" "$T62B_BASE_MOD" "$T62B_BASE_OUT"

assert_guard_exit "git-branch-guard/checkout-flag-position/baseline-blocks-q-b" \
  "$T62B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -q -b nova"}}' \
  2

assert_guard_exit "git-branch-guard/checkout-flag-position/baseline-blocks-no-track" \
  "$T62B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout --no-track -b nova"}}' \
  2

T62B_MOD="$WORK/s62b-mod"
mkdir -p "$T62B_MOD/cmd" "$T62B_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62B_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62B_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62B_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62B_MOD/go.sum"

# Corrupção retargetada para a forma EXATA pré-ML-4B (não um pattern
# "nunca casa" genérico): reverte o for-loop de varredura de tokens para o
# `if [ "${1:-}" = "-b" ]; then` original, que só olha o token IMEDIATAMENTE
# seguinte a `checkout`. Isso preserva a detecção de `git checkout -b nova`
# (a flag na primeira posição) e derruba só `-q -b`/`--no-track -b` — o
# discriminante preciso do que o ML-4B mudou, não uma corrupção que também
# apagaria a detecção de `checkout -b` simples (o que tornaria o cenário
# indistinguível de "checkout detection sumiu inteira").
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T62B_MOD/internal/generators/scaffold.go" \
  '      checkout)
        for tok2 in "$@"; do
          case "$tok2" in
            -b|-B|--orphan|--orphan=*)
              echo "checkout-b"
              return 0
              ;;
          esac
        done
' \
  '      checkout)
        if [ "${1:-}" = "-b" ]; then
          echo "checkout-b"
          return 0
        fi
' \
  "s62b-go-checkout-token-scan-reverted-to-pre-ml4b"

T62B_OUT="$WORK/s62b-out"
run_go_guard_dump "setup-s62b-go-corrupted-build" "$T62B_MOD" "$T62B_OUT"

assert_guard_exit "git-branch-guard/checkout-flag-position/detection-catches-bypass-q-b" \
  "$T62B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -q -b nova"}}' \
  0

assert_guard_exit "git-branch-guard/checkout-flag-position/detection-catches-bypass-no-track" \
  "$T62B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout --no-track -b nova"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido (revertido para a
# forma pré-ML-4B), `git checkout -b nova` — flag na primeira posição —
# precisa continuar bloqueado. Prova que a corrupção isola exatamente o
# token-scan que o ML-4B acrescentou, não a detecção de checkout -b como um
# todo.
assert_guard_exit "git-branch-guard/checkout-flag-position/detection-does-not-break-plain-checkout-b" \
  "$T62B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -b nova"}}' \
  2

# ---------------------------------------------------------------------------
# Cenário 63 — scripts/trackfw-git-branch-guard.sh (ML-4C,
# ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-
# da-release-7-0-0.md), corretivo da reverificação do hades-tf após levantar
# o bloqueio do ML-4A: três evasões apontadas e fechadas neste ML —
# `git branch <nome>` (e -c/-C/-m/-M), `git worktree add -b`, e
# `env CHAVE=valor git ...`. Mesmo padrão baseline+detecção dos Cenários
# 60/61/62: braço baseline prova que o guard LIMPO bloqueia; braço de
# detecção corrompe UM literal isolado do gerador Go, reconstrói o script a
# partir de um módulo Go isolado, e prova que sem essa linha o mesmo comando
# escapa (exit 0 em vez de 2) — não-vacuidade: a detecção depende exatamente
# do literal que o ML-4C adicionou, não de qualquer outro efeito colateral.
# ---------------------------------------------------------------------------

# --- 63a — git branch <nome> (argumento posicional puro cria a branch) -----
T63A_BASE_MOD="$WORK/s63a-base-mod"
mkdir -p "$T63A_BASE_MOD/cmd" "$T63A_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63A_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63A_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63A_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63A_BASE_MOD/go.sum"

T63A_BASE_OUT="$WORK/s63a-base-out"
run_go_guard_dump "setup-s63a-go-baseline-build" "$T63A_BASE_MOD" "$T63A_BASE_OUT"

assert_guard_exit "git-branch-guard/branch-create/baseline-blocks-positional" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch nova"}}' \
  2

assert_guard_exit "git-branch-guard/branch-create/baseline-blocks-dash-c" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -c origem nova"}}' \
  2

assert_guard_exit "git-branch-guard/branch-create/baseline-allows-list" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -a"}}' \
  0

assert_guard_exit "git-branch-guard/branch-create/baseline-allows-delete" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -d nome"}}' \
  0

T63A_MOD="$WORK/s63a-mod"
mkdir -p "$T63A_MOD/cmd" "$T63A_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63A_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63A_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63A_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63A_MOD/go.sum"

# Corrupção isolada: neutraliza SÓ a marcação de "argumento posicional puro
# cria branch" (saw_positional=1 -> saw_positional=0), sem tocar
# branch_action (-c/-C/-m/-M) nem has_delete — prova que a detecção depende
# exatamente desse literal, e não derruba -c/-C/-m/-M nem -d/-D (asserção de
# auto-discriminação abaixo).
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T63A_MOD/internal/generators/scaffold.go" \
  '            *)
              saw_positional=1
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then' \
  '            *)
              saw_positional=0
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then' \
  "s63a-go-branch-positional-detection-removed"

T63A_OUT="$WORK/s63a-out"
run_go_guard_dump "setup-s63a-go-corrupted-build" "$T63A_MOD" "$T63A_OUT"

assert_guard_exit "git-branch-guard/branch-create/detection-catches-bypass-positional" \
  "$T63A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch nova"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido, `git branch -c origem
# nova` (via branch_action, não saw_positional) e `git branch -d nome`
# (leitura/delete, nunca deveria bloquear) precisam se comportar
# exatamente como antes — prova que a corrupção isola só o caminho de
# argumento posicional puro.
assert_guard_exit "git-branch-guard/branch-create/detection-does-not-break-dash-c" \
  "$T63A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -c origem nova"}}' \
  2

assert_guard_exit "git-branch-guard/branch-create/detection-does-not-break-delete" \
  "$T63A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -d nome"}}' \
  0

# --- 63b — git worktree add -b (forma direta de criar branch) --------------
T63B_BASE_MOD="$WORK/s63b-base-mod"
mkdir -p "$T63B_BASE_MOD/cmd" "$T63B_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63B_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63B_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63B_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63B_BASE_MOD/go.sum"

T63B_BASE_OUT="$WORK/s63b-base-out"
run_go_guard_dump "setup-s63b-go-baseline-build" "$T63B_BASE_MOD" "$T63B_BASE_OUT"

assert_guard_exit "git-branch-guard/worktree-add-b/baseline-blocks" \
  "$T63B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git worktree add -b nova ../nova"}}' \
  2

assert_guard_exit "git-branch-guard/worktree-add-b/baseline-allows-without-b" \
  "$T63B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git worktree add ../nova existing-branch"}}' \
  0

T63B_MOD="$WORK/s63b-mod"
mkdir -p "$T63B_MOD/cmd" "$T63B_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63B_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63B_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63B_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63B_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T63B_MOD/internal/generators/scaffold.go" \
  '      worktree)
        if [ "${1:-}" = "add" ]; then' \
  '      worktree)
        if [ "${1:-}" = "__never-matches-s63b__" ]; then' \
  "s63b-go-worktree-add-detection-removed"

T63B_OUT="$WORK/s63b-out"
run_go_guard_dump "setup-s63b-go-corrupted-build" "$T63B_MOD" "$T63B_OUT"

assert_guard_exit "git-branch-guard/worktree-add-b/detection-catches-bypass" \
  "$T63B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git worktree add -b nova ../nova"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido, `git push` puro
# continua bloqueado — isola a corrupção ao worktree add -b, não uma
# quebra geral do matcher.
assert_guard_exit "git-branch-guard/worktree-add-b/detection-does-not-break-plain-push" \
  "$T63B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git push"}}' \
  2

# --- 63c — env CHAVE=valor git ... (stripping de atribuição de variável) ---
T63C_BASE_MOD="$WORK/s63c-base-mod"
mkdir -p "$T63C_BASE_MOD/cmd" "$T63C_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63C_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63C_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63C_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63C_BASE_MOD/go.sum"

T63C_BASE_OUT="$WORK/s63c-base-out"
run_go_guard_dump "setup-s63c-go-baseline-build" "$T63C_BASE_MOD" "$T63C_BASE_OUT"

assert_guard_exit "git-branch-guard/env-var-assignment/baseline-blocks-single" \
  "$T63C_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar git push"}}' \
  2

assert_guard_exit "git-branch-guard/env-var-assignment/baseline-blocks-multiple" \
  "$T63C_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar BAZ=qux git commit -m x"}}' \
  2

assert_guard_exit "git-branch-guard/env-var-assignment/baseline-still-evades-flag-form" \
  "$T63C_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env -i git push"}}' \
  0

T63C_MOD="$WORK/s63c-mod"
mkdir -p "$T63C_MOD/cmd" "$T63C_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63C_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63C_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63C_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63C_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T63C_MOD/internal/generators/scaffold.go" \
  '      if [ "$is_env" = "env" ]; then' \
  '      if [ "$is_env" = "__never-matches-s63c__" ]; then' \
  "s63c-go-env-var-assignment-stripping-removed"

T63C_OUT="$WORK/s63c-out"
run_go_guard_dump "setup-s63c-go-corrupted-build" "$T63C_MOD" "$T63C_OUT"

assert_guard_exit "git-branch-guard/env-var-assignment/detection-catches-bypass-single" \
  "$T63C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar git push"}}' \
  0

assert_guard_exit "git-branch-guard/env-var-assignment/detection-catches-bypass-multiple" \
  "$T63C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar BAZ=qux git commit -m x"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido, a forma NUA (sem
# atribuição de variável) `env git push`, fechada pelo ML-4B, precisa
# continuar bloqueada — isola a corrupção ao stripping de CHAVE=valor, não
# ao stripping de env/command como um todo.
assert_guard_exit "git-branch-guard/env-var-assignment/detection-does-not-break-bare-env-prefix" \
  "$T63C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env git push"}}' \
  2

# Restaura o cwd original — o resto do gate (se algum cenário futuro for
# acrescentado depois deste ponto) não deve herdar o cwd de fixture dos
# Cenários 60-63.
cd "$GBG_ORIGINAL_PWD"

# ---------------------------------------------------------------------------
# Cenário 64 — scripts/trackfw-git-branch-guard.sh (ML-1A,
# ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
# integridade-independente-de-fiacao.md): o guard vira no-op (exit 0) fora
# de projeto trackfw (sem trackfw.yaml em nenhum ancestral do cwd) — decisão
# de ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md,
# pré-requisito para cabear o guard em escopo global (Wave 2 do mesmo
# roadmap) sem quebrar git commit/push em toda a máquina. Quatro braços,
# mesmo padrão dos Cenários 62/63: baseline sem trackfw.yaml prova o no-op;
# baseline COM trackfw.yaml prova reverse-vacuity (o 0 acima veio do no-op,
# não de um build quebrado); detecção corrompe o literal isolado do
# gerador Go que fecha o probe (`[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0`) e
# prova que sem essa linha o guard bloqueia INCONDICIONALMENTE mesmo fora de
# projeto trackfw; auto-discriminação prova que, contra o MESMO build
# corrompido, dentro de projeto trackfw o comportamento de bloqueio
# permanece — isola a corrupção ao probe do no-op, não ao matcher inteiro.
# ---------------------------------------------------------------------------
T64_NO_YAML_DIR="$WORK/s64-no-trackfw-yaml"
mkdir -p "$T64_NO_YAML_DIR"
T64_WITH_YAML_DIR="$WORK/s64-with-trackfw-yaml"
mkdir -p "$T64_WITH_YAML_DIR"
echo "project_name: s64-fixture" > "$T64_WITH_YAML_DIR/trackfw.yaml"

T64_BASE_MOD="$WORK/s64-base-mod"
mkdir -p "$T64_BASE_MOD/cmd" "$T64_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T64_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T64_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T64_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T64_BASE_MOD/go.sum"

T64_BASE_OUT="$WORK/s64-base-out"
run_go_guard_dump "setup-s64-go-baseline-build" "$T64_BASE_MOD" "$T64_BASE_OUT"

(
  cd "$T64_NO_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/baseline-noop-without-trackfw-yaml" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    0
)

(
  cd "$T64_WITH_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/baseline-blocks-with-trackfw-yaml" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    2
)

T64_MOD="$WORK/s64-mod"
mkdir -p "$T64_MOD/cmd" "$T64_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T64_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T64_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T64_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T64_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T64_MOD/internal/generators/scaffold.go" \
  '[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0' \
  '[ "$_TRACKFW_FOUND" -eq 1 ] || true' \
  "s64-go-noop-probe-removed"

T64_OUT="$WORK/s64-out"
run_go_guard_dump "setup-s64-go-corrupted-build" "$T64_MOD" "$T64_OUT"

(
  cd "$T64_NO_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/detection-catches-bypass-without-trackfw-yaml" \
    "$T64_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    2
)

# Auto-discriminação: contra o MESMO build corrompido, DENTRO de projeto
# trackfw o guard precisa continuar bloqueando exatamente como antes —
# isola a corrupção ao probe do no-op, não a uma quebra geral do matcher.
(
  cd "$T64_WITH_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/detection-does-not-break-inside-project" \
    "$T64_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    2
)

# ---------------------------------------------------------------------------
# Cenário 65 — scripts/trackfw-git-branch-guard.sh (ML-1B, mesma ROADMAP-
# 2026-08-17): a auditoria do arquiteto reprovou o ML-1A porque o no-op
# saía com exit 0 ANTES de ler o stdin -- quem escreve o payload JSON no
# pipe (o próprio harness) recebia EPIPE, reprodutível em 100% das chamadas
# fora de projeto trackfw. Três braços: baseline com o build ATUAL (já
# corrigido pelo ML-1B) prova que o escritor termina limpo; baseline com
# payload GRANDE (>64KB, estoura o buffer do pipe) prova que o dreno
# funciona mesmo sob pressão de buffer; detecção corrompe o literal isolado
# do dreno de stdin (`[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)`
# -> `_TRACKFW_STDIN=""`, mantendo a checagem `-t 0` mas neutralizando a
# leitura real) e prova que, sem o dreno, o EPIPE volta -- isolando a
# regressão ao dreno em si, não a uma mudança geral no probe do no-op.
# ---------------------------------------------------------------------------
T65_NO_YAML_DIR="$WORK/s65-no-trackfw-yaml"
mkdir -p "$T65_NO_YAML_DIR"

(
  cd "$T65_NO_YAML_DIR" && assert_writer_no_epipe \
    "git-branch-guard/stdin-drain-before-noop/baseline-writer-clean" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    0 1
)

T65_BIG_PAYLOAD=$(python3 -c "import json; print(json.dumps({'tool_input':{'command':'git push','pad':'x'*200000}}))")
(
  cd "$T65_NO_YAML_DIR" && assert_writer_no_epipe \
    "git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    "$T65_BIG_PAYLOAD" \
    0 1
)

T65_MOD="$WORK/s65-mod"
mkdir -p "$T65_MOD/cmd" "$T65_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T65_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T65_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T65_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T65_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T65_MOD/internal/generators/scaffold.go" \
  '[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)' \
  '_TRACKFW_STDIN=""' \
  "s65-go-stdin-drain-removed"

T65_OUT="$WORK/s65-out"
run_go_guard_dump "setup-s65-go-corrupted-build" "$T65_MOD" "$T65_OUT"

(
  cd "$T65_NO_YAML_DIR" && assert_writer_no_epipe \
    "git-branch-guard/stdin-drain-before-noop/detection-catches-epipe-regression" \
    "$T65_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    0 0
)

# ---------------------------------------------------------------------------
# Cenário 66 — check-harness-hooks-parity.sh: Python muda o `matcher` de
#              trackfw-git-branch-guard-global-post no wiring GLOBAL do Kiro
#              (~/.kiro/hooks/trackfw-git-branch-guard.json) → gate detecta o
#              drift estrutural.
#
# Objetivo (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
# projeto-e-integridade-independente-de-fiacao, Wave 2/ML-2A): mesmo padrão
# do Cenário 45, agora para o par irmão git-branch-guard —
# harnessGitBranchGuardTargetKiro (Go internal/generators/update.go),
# gitBranchGuardTargetKiro (Node npm/src/commands/update-harness.js) e
# _git_branch_guard_kiro_result (Python pypi/trackfw/commands/
# update_harness.py) são 3 implementações independentes que só precisam
# concordar em ESTRUTURA — nada além deste gate garante paridade estrutural
# entre elas para o par git-branch-guard especificamente (o Cenário 45 só
# provou não-vacuidade para credential-guard; sem este cenário, uma
# divergência introduzida só no par git-branch-guard poderia passar
# despercebida indefinidamente, já que os dois pares vivem em arquivos
# Kiro distintos — ver header do gate).
#
# Corrompe apenas o literal `_git_branch_guard_kiro_result` do Python (troca
# o matcher da entrada "trackfw-git-branch-guard-global-post" de 'shell'
# para 'execute_bash') numa cópia isolada de pypi/ — o gate deve reprovar com
# o path JSON divergente ($.hooks[1].matcher) no diagnóstico, sob o label
# NOVO "harness-hooks-parity/kiro/git-branch-guard/go-vs-py" (o label
# original "harness-hooks-parity/kiro/go-vs-py", ainda usado pelo Cenário 45
# para o arquivo de credential-guard, permanece intocado — prova que os dois
# arquivos do Kiro são comparados de forma independente).
#
# Seam: mesmo escopo do Cenário 45 — contexto estendido até a linha
# `"trigger": "PostToolUse",` restringe a substituição à única ocorrência da
# entrada -global-post dentro de `_git_branch_guard_kiro_result` (a mesma
# string `"matcher": "shell",` também aparece em
# `_credential_guard_kiro_result`, uma função textualmente distinta).
# ---------------------------------------------------------------------------
T66="$WORK/s66"
mkdir -p "$T66/scripts"
setup_npm_tree "$T66"
cp -r "$ROOT_DIR/pypi" "$T66/pypi"
cp "$ROOT_DIR/scripts/check-harness-hooks-parity.sh" "$T66/scripts/"

corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/update_harness.py" "$T66/pypi/trackfw/commands/update_harness.py" \
  $'                "trigger": "PostToolUse",\n                "matcher": "shell",\n                "action": {"type": "command", "command": script_path},\n            },\n        ],\n    }\n    desired = (json.dumps(desired_doc, indent=2) + "\\n").encode("utf-8")\n\n    try:\n        existing = Path(path).read_bytes()\n    except FileNotFoundError:\n        if not install_missing:\n            return {"id": target_id, "state": STATE_MISSING, "path": display_path}\n        if dry_run:\n            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n        try:\n            Path(path).parent.mkdir(parents=True, exist_ok=True)\n            Path(path).write_bytes(desired)\n        except OSError as error:\n            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}\n        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n    except OSError as error:\n        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}\n\n    if existing == desired:\n        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}\n    if dry_run:\n        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n    try:\n        Path(path).write_bytes(desired)\n    except OSError as error:\n        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}\n    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n\n\ndef _catalog_group_result(' \
  $'                "trigger": "PostToolUse",\n                "matcher": "execute_bash",\n                "action": {"type": "command", "command": script_path},\n            },\n        ],\n    }\n    desired = (json.dumps(desired_doc, indent=2) + "\\n").encode("utf-8")\n\n    try:\n        existing = Path(path).read_bytes()\n    except FileNotFoundError:\n        if not install_missing:\n            return {"id": target_id, "state": STATE_MISSING, "path": display_path}\n        if dry_run:\n            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n        try:\n            Path(path).parent.mkdir(parents=True, exist_ok=True)\n            Path(path).write_bytes(desired)\n        except OSError as error:\n            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}\n        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n    except OSError as error:\n        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}\n\n    if existing == desired:\n        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}\n    if dry_run:\n        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n    try:\n        Path(path).write_bytes(desired)\n    except OSError as error:\n        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}\n    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}\n\n\ndef _catalog_group_result(' \
  "s66-python-kiro-git-branch-guard-post-matcher"

assert_fails_with "harness-hooks-parity/kiro/git-branch-guard/go-vs-py-matcher-drift-not-detected" \
  "harness-hooks-parity/kiro/git-branch-guard/go-vs-py" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" PY_ROOT="$T66/pypi" bash "$T66/scripts/check-harness-hooks-parity.sh"

# Non-regression: the ORIGINAL credential-guard label (Cenário 45) must
# still pass against this same corrupted tree — proving the two Kiro files
# (and their two comparison labels) are independent, not one being a proxy
# for the other. The overall gate run above already exits non-zero (the
# git-branch-guard comparison fails), so this checks the specific OK line
# for the credential-guard label is still present in that same run's output.
set +e
T66_GATE_OUT=$(env GO_BIN="$ROOT_DIR/bin/trackfw" PY_ROOT="$T66/pypi" bash "$T66/scripts/check-harness-hooks-parity.sh" 2>&1)
set -e
if ! grep -qF "OK   [harness-hooks-parity/kiro/go-vs-py]" <<<"$T66_GATE_OUT"; then
  echo "FAIL [falsify/harness-hooks-parity/kiro/go-vs-py-credential-guard-unaffected-by-git-branch-guard-corruption]: expected the credential-guard label to still pass while only git-branch-guard is corrupted" >&2
  echo "  output: $T66_GATE_OUT" >&2
  exit 1
fi
echo "OK   [falsify/harness-hooks-parity/kiro/go-vs-py-credential-guard-unaffected-by-git-branch-guard-corruption]"

# ---------------------------------------------------------------------------
# Cenário 67 — dedup projeto+global para o `git-branch-guard`
#              (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
#              projeto-e-integridade-independente-de-fiacao.md, Wave 2/ML-2B):
#              com a fiação global do git-branch-guard instalada para o
#              Claude, `trackfw discover --init` NÃO deve escrever a entrada
#              de projeto ($CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-
#              guard.sh) — senão o guard roda duas vezes por chamada Bash e
#              a mensagem de bloqueio dobra (o sintoma medido que abriu esta
#              ML). globalGitBranchGuardInstalledClaude() (Go internal/
#              generators/agentfiles.go) é o seam.
#
# Por que Go sozinho: mesmo precedente dos Cenários 57 (Go)/59 (Python) —
# sabotagem de um único stack quando a paridade estrutural entre os 3 já é
# garantida em outro lugar (make quality roda `go test`/`node --test`/
# `pytest` nos 3, cada um com seus próprios testes de dedup do
# git-branch-guard criados por este ML — internal/generators/
# git_branch_guard_dedup_test.go, npm/tests/git_branch_guard_dedup.test.js,
# pypi/tests/test_git_branch_guard_dedup.py — que já provam a mesma
# propriedade nos 3 stacks via unidade). Este cenário prova o mecanismo
# fim-a-fim contra o BINÁRIO REAL (`discover --init`), o que nenhum teste de
# unidade cobre (eles chamam os injetores diretamente, nunca o comando CLI).
#
# Três braços:
#   1. Baseline — $HOME sintético com ~/.claude/settings.json apontando
#      PreToolUse[Bash] para o caminho EXATO que
#      globalGitBranchGuardScriptPath() resolveria
#      ($HOME/.trackfw/scripts/trackfw-git-branch-guard.sh); roda
#      `discover --init` de verdade contra um fixture de projeto (marcador
#      CLAUDE.md) e prova que .claude/settings.json do PROJETO não contém a
#      entrada de git-branch-guard, mas CONTÉM a de credential-guard — prova
#      que o skip é específico do guard, não um apagão geral de PreToolUse.
#   2. Reverse-vacuity — mesmo fixture de projeto, $HOME vazio (nenhuma
#      fiação global) → a entrada de projeto do git-branch-guard aparece
#      normalmente, provando que a ausência no braço 1 veio da fiação
#      global, não de alguma quebra geral do injector.
#   3. Detecção — corrompe o CORPO INTEIRO de globalGitBranchGuardInstalledClaude
#      (não só um trecho — o texto `hookArrayHasCommand(hooks["PreToolUse"],
#      "Bash", scriptPath)` também aparece, idêntico, dentro de
#      globalCredentialGuardInstalledClaude, então corrupt_literal com esse
#      recorte sozinho falharia a asserção de ocorrência única) para
#      `return false` incondicional, numa cópia isolada de internal/+cmd/
#      (mesmo padrão do Cenário 46); reconstrói e roda o MESMO fixture do
#      braço 1 (mesmo $HOME sintético com a fiação global instalada) contra
#      o binário corrompido — a entrada de projeto REAPARECE, reproduzindo o
#      sintoma exato de mensagem duplicada que motivou esta ML.
# ---------------------------------------------------------------------------
T67_PROJECT_DIR="$WORK/s67-project"
mkdir -p "$T67_PROJECT_DIR"
: > "$T67_PROJECT_DIR/CLAUDE.md"

# T67_FAKE_HOME is built from a slash-collapsed form of $WORK (WORK67_CLEAN),
# not $WORK itself -- $TMPDIR ends in "/" on macOS, so $WORK carries an
# embedded "//" that used to make this baseline arm's outcome depend on the
# exact bug ML-2C fixes (ROADMAP-2026-08-17, "Diagnostico do arquiteto"). The
# // tolerance itself is now exercised EXPLICITLY by braco 4 below with a
# deliberately corrupted HOME, so this baseline stays deterministic across
# platforms instead of accidentally depending on TMPDIR's shape.
WORK67_CLEAN=$(printf '%s' "$WORK" | sed 's#//*#/#g')
T67_FAKE_HOME="$WORK67_CLEAN/s67-fake-home-installed"
mkdir -p "$T67_FAKE_HOME/.claude"
cat >"$T67_FAKE_HOME/.claude/settings.json" <<EOF
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$T67_FAKE_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"}]}]}}
EOF

set +e
(cd "$T67_PROJECT_DIR" && HOME="$T67_FAKE_HOME" "$ROOT_DIR/bin/trackfw" discover --init) >"$WORK/s67-baseline-discover.log" 2>&1
s67b_status=$?
set -e
if [[ $s67b_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/baseline-setup]: discover --init saiu com $s67b_status" >&2
  cat "$WORK/s67-baseline-discover.log" >&2
  exit 1
fi

s67b_settings="$T67_PROJECT_DIR/.claude/settings.json"
if grep -qF 'trackfw-git-branch-guard.sh' "$s67b_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/baseline-skips-project-entry]: entrada de git-branch-guard presente em $s67b_settings com a fiação global instalada" >&2
  cat "$s67b_settings" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-dedup/baseline-skips-project-entry]"

if ! grep -qF 'trackfw-credential-guard.sh' "$s67b_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/baseline-credential-guard-unaffected]: entrada de credential-guard ausente — o skip não deveria afetar o outro guard" >&2
  cat "$s67b_settings" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-dedup/baseline-credential-guard-unaffected]"

# --- braço 2: reverse-vacuity, $HOME vazio -> entrada de projeto normal ---
T67_PROJECT_DIR_RV="$WORK/s67-project-reverse-vacuity"
mkdir -p "$T67_PROJECT_DIR_RV"
: > "$T67_PROJECT_DIR_RV/CLAUDE.md"
T67_EMPTY_HOME="$WORK/s67-empty-home"
mkdir -p "$T67_EMPTY_HOME"

set +e
(cd "$T67_PROJECT_DIR_RV" && HOME="$T67_EMPTY_HOME" "$ROOT_DIR/bin/trackfw" discover --init) >"$WORK/s67-rv-discover.log" 2>&1
s67rv_status=$?
set -e
if [[ $s67rv_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/reverse-vacuity-setup]: discover --init saiu com $s67rv_status" >&2
  cat "$WORK/s67-rv-discover.log" >&2
  exit 1
fi

s67rv_settings="$T67_PROJECT_DIR_RV/.claude/settings.json"
if ! grep -qF 'trackfw-git-branch-guard.sh' "$s67rv_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/reverse-vacuity]: entrada de git-branch-guard ausente com \$HOME vazio (sem fiação global) — o skip não deveria acontecer aqui" >&2
  cat "$s67rv_settings" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-dedup/reverse-vacuity]"

# --- braço 3: detecção — dedup neutralizado, entrada de projeto reaparece ---
T67_MOD="$WORK/s67-corrupt-go"
mkdir -p "$T67_MOD/cmd" "$T67_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T67_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T67_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T67_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T67_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/agentfiles.go" "$T67_MOD/internal/generators/agentfiles.go" \
  $'func globalGitBranchGuardInstalledClaude() bool {\n\tscriptPath, ok := globalGitBranchGuardScriptPath()\n\tif !ok {\n\t\treturn false\n\t}\n\troot, ok := readGlobalHookJSON(".claude", "settings.json")\n\tif !ok {\n\t\treturn false\n\t}\n\thooks, _ := root["hooks"].(map[string]interface{})\n\treturn hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath)\n}' \
  $'func globalGitBranchGuardInstalledClaude() bool {\n\treturn false\n}' \
  "s67-go-claude-git-branch-guard-dedup-always-false"

T67_BIN="$WORK/s67-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T67_BIN")"
build_go_or_fail "setup-s67-go-corrupt-build" "$T67_MOD" "$T67_BIN"

T67_PROJECT_DIR_DET="$WORK/s67-project-detection"
mkdir -p "$T67_PROJECT_DIR_DET"
: > "$T67_PROJECT_DIR_DET/CLAUDE.md"

set +e
(cd "$T67_PROJECT_DIR_DET" && HOME="$T67_FAKE_HOME" "$T67_BIN" discover --init) >"$WORK/s67-detection-discover.log" 2>&1
s67d_status=$?
set -e
if [[ $s67d_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/detection-setup]: discover --init (binário corrompido) saiu com $s67d_status" >&2
  cat "$WORK/s67-detection-discover.log" >&2
  exit 1
fi

s67d_settings="$T67_PROJECT_DIR_DET/.claude/settings.json"
if ! grep -qF 'trackfw-git-branch-guard.sh' "$s67d_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/detection-catches-regression]: com o dedup neutralizado (sempre 'não instalado'), a entrada de projeto deveria REAPARECER mesmo com a fiação global instalada — não reapareceu" >&2
  cat "$s67d_settings" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-dedup/detection-catches-regression]"

# --- braço 4 (ML-2C) — tolerância a "//" no comando gravado no config global ---
# Constrói um HOME sintético com barra dupla EMBUTIDA no meio do caminho
# (reproduzindo exatamente a forma que $TMPDIR/$WORK assume no macOS) e grava
# o comando do PreToolUse[Bash] usando essa forma crua, sem passar por
# filepath.Join — como um config editado à mão, ou capturado quando $HOME
# tinha barra dupla, produziria. globalGitBranchGuardScriptPath() computa o
# caminho já normalizado a partir do MESMO HOME; antes do ML-2C a comparação
# de string crua entre os dois falhava e o dedup não disparava (refs=1); a
# correção normaliza os dois lados antes de comparar (refs=0 esperado).
T67_FAKE_HOME_SLASH="${WORK67_CLEAN}//s67-fake-home-installed-slash"
mkdir -p "$T67_FAKE_HOME_SLASH/.claude"
cat >"$T67_FAKE_HOME_SLASH/.claude/settings.json" <<EOF
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$T67_FAKE_HOME_SLASH/.trackfw/scripts/trackfw-git-branch-guard.sh"}]}]}}
EOF

T67_PROJECT_DIR_SLASH="$WORK67_CLEAN/s67-project-double-slash"
mkdir -p "$T67_PROJECT_DIR_SLASH"
: > "$T67_PROJECT_DIR_SLASH/CLAUDE.md"

set +e
(cd "$T67_PROJECT_DIR_SLASH" && HOME="$T67_FAKE_HOME_SLASH" "$ROOT_DIR/bin/trackfw" discover --init) >"$WORK/s67-slash-discover.log" 2>&1
s67s_status=$?
set -e
if [[ $s67s_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/double-slash-tolerance-setup]: discover --init saiu com $s67s_status" >&2
  cat "$WORK/s67-slash-discover.log" >&2
  exit 1
fi

s67s_settings="$T67_PROJECT_DIR_SLASH/.claude/settings.json"
if grep -qF 'trackfw-git-branch-guard.sh' "$s67s_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/double-slash-tolerance]: entrada de git-branch-guard presente em $s67s_settings mesmo com // no comando gravado do HOME global — a comparação deveria normalizar antes de comparar" >&2
  cat "$s67s_settings" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-dedup/double-slash-tolerance]"

# ---------------------------------------------------------------------------
# Cenário 68 — internal/validator: "git_branch_guard_script_integrity" (e,
#              não-regressão, "credential_guard_script_integrity") em ESCOPO
#              GLOBAL disparam pela EXISTÊNCIA do artefato em
#              ~/.trackfw/scripts/, não pela fiação (ROADMAP-2026-08-17-
#              guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-
#              independente-de-fiacao, ML-3A).
#
# Ponto cego fechado: antes deste ML, validateGuardGlobalScriptIntegrity só
# avaliava os 6 arquivos de config global QUE REFERENCIAM scriptMarker — sem
# fiação, o laço nunca entrava e a regra nunca rodava. Medido na máquina de
# KG (motivação da REQ): o script global do git-branch-guard ficou 3
# versões atrasado (123 linhas vs 369) com `validate` verde o tempo todo,
# porque nada o cabeava.
#
# $HOME é sempre um diretório sintético isolado (t.TempDir/$WORK-equivalente
# em shell), nunca o real — precedente de vazamento de ambiente é o
# Cenário 46.
# ---------------------------------------------------------------------------
S68_MSG='content diverges from the template this version of trackfw generates'

s68_write_project() {
  local dest=$1 rule=$2 severity=$3
  scaffold_adr_req_project "$dest"
  cat >> "$dest/trackfw.yaml" <<EOF
rules:
  $rule: $severity
EOF
}

# --- fixture: $HOME sintético e vazio, 'trackfw update harness' (SEM
# --targets, SEM --install-missing) escreve os dois scripts globais
# incondicionalmente (internal/generators/update.go:489-496) mas não
# instala NENHUM alvo — sem nada já instalado e sem --install-missing, todo
# target reporta "missing" e fica intocado. Resultado: os scripts existem em
# $T68_HOME/.trackfw/scripts/trackfw-{credential,git-branch}-guard.sh e ZERO
# arquivo de config nesse $HOME referencia nenhum dos dois — exatamente o
# estado que fazia a regra antiga nunca disparar. -----------------------------
T68_HOME="$WORK/s68-fake-home"
mkdir -p "$T68_HOME"

(HOME="$T68_HOME" "$ROOT_DIR/bin/trackfw" update harness) >"$WORK/s68-update-harness.log" 2>&1
T68_GBG_SCRIPT="$T68_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
if [[ ! -s "$T68_GBG_SCRIPT" ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/setup]: 'trackfw update harness' não escreveu $T68_GBG_SCRIPT" >&2
  cat "$WORK/s68-update-harness.log" >&2
  exit 1
fi

for cfg in .claude/settings.json .codex/hooks.json .gemini/settings.json \
           .cursor/hooks.json .copilot/settings.json .kiro/hooks/trackfw-git-branch-guard.json; do
  if [[ -e "$T68_HOME/$cfg" ]]; then
    echo "FAIL [falsify/git-branch-guard-global-script-integrity/setup]: $T68_HOME/$cfg existe — fixture deveria ter ZERO fiação para provar independência da fiação" >&2
    exit 1
  fi
done

# --- braço baseline: script global íntegro, ZERO fiação -> validate passa --
T68_OK="$WORK/s68-project-ok"
s68_write_project "$T68_OK" git_branch_guard_script_integrity error

set +e
s68ok_out=$(cd "$T68_OK" && HOME="$T68_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
s68ok_status=$?
set -e
if [[ $s68ok_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/baseline]: script global íntegro e SEM fiação deveria passar, saiu com $s68ok_status" >&2
  echo "  output: $s68ok_out" >&2
  exit 1
fi
if grep -qF "$S68_MSG" <<<"$s68ok_out"; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/baseline]: script global íntegro mas a regra disparou mesmo assim" >&2
  echo "  output: $s68ok_out" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-global-script-integrity/baseline]"

# --- braço de ausência: $HOME onde NENHUM script foi instalado -> silêncio -
# (não ter rodado 'trackfw update harness' nesse $HOME é estado legítimo, não
# é erro — falso-positivo aqui afetaria todo usuário que nunca instalou o
# harness global) -------------------------------------------------------------
T68_ABSENT_HOME="$WORK/s68-absent-home"
mkdir -p "$T68_ABSENT_HOME"
T68_ABSENT="$WORK/s68-project-absent"
s68_write_project "$T68_ABSENT" git_branch_guard_script_integrity error

set +e
s68absent_out=$(cd "$T68_ABSENT" && HOME="$T68_ABSENT_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
s68absent_status=$?
set -e
if [[ $s68absent_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/absent-is-not-a-violation]: script global nunca instalado ($T68_ABSENT_HOME) não pode reprovar validate, saiu com $s68absent_status" >&2
  echo "  output: $s68absent_out" >&2
  exit 1
fi
if grep -qF "$S68_MSG" <<<"$s68absent_out"; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/absent-is-not-a-violation]: script global nunca instalado, mas a regra disparou (falso-positivo de ausência)" >&2
  echo "  output: $s68absent_out" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-global-script-integrity/absent-is-not-a-violation]"

# --- braço de detecção: script global corrompido, ZERO config referenciando
# ele -> validate acusa mesmo assim — o discriminante central deste ML -----
T68_BAD="$WORK/s68-project-bad"
s68_write_project "$T68_BAD" git_branch_guard_script_integrity error
printf '# tampered by check-gates-falsify.sh Cenario 68\n' >> "$T68_GBG_SCRIPT"

assert_fails_with "git-branch-guard-global-script-integrity/detected-without-wiring" \
  "$S68_MSG" \
  bash -c "cd '$T68_BAD' && exec env HOME='$T68_HOME' '$ROOT_DIR/bin/trackfw' validate"

# --- prova de não-vacuidade: mesma árvore corrompida ($T68_HOME já ficou
# corrompido pelo braço acima), regra desligada -> o braço de detecção
# FALHARIA -------------------------------------------------------------------
T68_OFF="$WORK/s68-project-off"
s68_write_project "$T68_OFF" git_branch_guard_script_integrity off

assert_would_now_fail "git-branch-guard-global-script-integrity" \
  "$S68_MSG" \
  bash -c "cd '$T68_OFF' && exec env HOME='$T68_HOME' '$ROOT_DIR/bin/trackfw' validate"

# --- braço de não-duplicação (git-branch-guard): o MESMO script referenciado
# por 2 configs de CLI diferentes (Claude + Codex, via 'update harness
# --install-missing') -> exatamente 1 mensagem, nunca 2. Prova central de
# "sem dupla emissão" agora que git-branch-guard tem DOIS caminhos possíveis
# de disparo (existência do artefato E fiação, desde a Wave 2 deste
# roadmap) -------------------------------------------------------------------
T68_DUP_HOME="$WORK/s68-dup-home-gbg"
mkdir -p "$T68_DUP_HOME"
set +e
(HOME="$T68_DUP_HOME" "$ROOT_DIR/bin/trackfw" update harness \
  --targets claude-git-branch-guard,codex-git-branch-guard --install-missing) \
  >"$WORK/s68-dup-update-harness-gbg.log" 2>&1
s68dupgbg_setup_status=$?
set -e
if [[ $s68dupgbg_setup_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/no-double-report-setup]: update harness saiu com $s68dupgbg_setup_status" >&2
  cat "$WORK/s68-dup-update-harness-gbg.log" >&2
  exit 1
fi
T68_DUP_SCRIPT="$T68_DUP_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
if ! grep -qF 'trackfw-git-branch-guard.sh' "$T68_DUP_HOME/.claude/settings.json" || \
   ! grep -qF 'trackfw-git-branch-guard.sh' "$T68_DUP_HOME/.codex/hooks.json"; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/no-double-report-setup]: fiação em Claude E Codex não foi instalada — não é o fixture de 2 configs pretendido" >&2
  exit 1
fi
printf '# tampered by check-gates-falsify.sh Cenario 68 (dup gbg)\n' >> "$T68_DUP_SCRIPT"

T68_DUP="$WORK/s68-project-dup-gbg"
s68_write_project "$T68_DUP" git_branch_guard_script_integrity error

set +e
s68dup_out=$(cd "$T68_DUP" && HOME="$T68_DUP_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
set -e
s68dup_count=$(grep -oF "$S68_MSG" <<<"$s68dup_out" | wc -l | tr -d ' ')
if [[ "$s68dup_count" -ne 1 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/no-double-report]: esperado exatamente 1 ocorrência da mensagem de integridade (2 configs referenciam o MESMO script), obteve $s68dup_count" >&2
  echo "  output: $s68dup_out" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-global-script-integrity/no-double-report]"

# --- braço de não-regressão + não-duplicação (credential-guard): mesmo
# padrão acima, mas para o guard que HOJE já é verificado via fiação — prova
# que ele continua sendo verificado (AC5, não-regressão) e que, mesmo
# referenciado por 2 CLIs, não passa a duplicar agora que a regra checa o
# artefato uma única vez por script em vez de uma vez por config ------------
T68_DUP_HOME_CG="$WORK/s68-dup-home-cg"
mkdir -p "$T68_DUP_HOME_CG"
set +e
(HOME="$T68_DUP_HOME_CG" "$ROOT_DIR/bin/trackfw" update harness \
  --targets claude-credential-guard,codex-credential-guard --install-missing) \
  >"$WORK/s68-dup-update-harness-cg.log" 2>&1
s68dupcg_setup_status=$?
set -e
if [[ $s68dupcg_setup_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-global-script-integrity/no-double-report-setup]: update harness saiu com $s68dupcg_setup_status" >&2
  cat "$WORK/s68-dup-update-harness-cg.log" >&2
  exit 1
fi
T68_DUP_SCRIPT_CG="$T68_DUP_HOME_CG/.trackfw/scripts/trackfw-credential-guard.sh"
if ! grep -qF 'trackfw-credential-guard.sh' "$T68_DUP_HOME_CG/.claude/settings.json" || \
   ! grep -qF 'trackfw-credential-guard.sh' "$T68_DUP_HOME_CG/.codex/hooks.json"; then
  echo "FAIL [falsify/credential-guard-global-script-integrity/no-double-report-setup]: fiação em Claude E Codex não foi instalada — não é o fixture de 2 configs pretendido" >&2
  exit 1
fi
printf '# tampered by check-gates-falsify.sh Cenario 68 (dup cg)\n' >> "$T68_DUP_SCRIPT_CG"

T68_DUP_CG="$WORK/s68-project-dup-cg"
s68_write_project "$T68_DUP_CG" credential_guard_script_integrity error

set +e
s68dupcg_out=$(cd "$T68_DUP_CG" && HOME="$T68_DUP_HOME_CG" "$ROOT_DIR/bin/trackfw" validate 2>&1)
set -e
s68dupcg_count=$(grep -oF "$S68_MSG" <<<"$s68dupcg_out" | wc -l | tr -d ' ')
if [[ "$s68dupcg_count" -ne 1 ]]; then
  echo "FAIL [falsify/credential-guard-global-script-integrity/no-double-report]: esperado exatamente 1 ocorrência (não-regressão + sem duplicar), obteve $s68dupcg_count" >&2
  echo "  output: $s68dupcg_out" >&2
  exit 1
fi
echo "OK   [falsify/credential-guard-global-script-integrity/no-double-report]"

# ---------------------------------------------------------------------------
# Cenário 69 — internal/validator: "git_branch_guard_hook_resolvable" em
#              ESCOPO GLOBAL passa a inspecionar o arquivo DEDICADO do Kiro
#              (~/.kiro/hooks/trackfw-git-branch-guard.json), não apenas
#              ~/.kiro/hooks/trackfw-credential-guard.json
#              (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
#              projeto-e-integridade-independente-de-fiacao, ML-3B).
#
# Ponto cego fechado: antes deste ML, globalGuardConfigFiles (validador, 3
# stacks) só apontava o Kiro para trackfw-credential-guard.json, mesmo para
# a checagem do git-branch-guard — então o arquivo dedicado que a Wave 2
# (ML-2A) passou a escrever para o git-branch-guard do Kiro nunca era lido
# por nenhuma checagem de resolvibilidade: um hook Kiro apontando para
# script ausente/não-executável passava limpo, o mesmo defeito ("instalado
# e não verificado") que esta REQ inteira existe para corrigir.
#
# $HOME é sempre um diretório sintético isolado, nunca o real — precedente
# de vazamento de ambiente é o Cenário 46.
# ---------------------------------------------------------------------------
T69_HOME="$WORK/s69-fake-home"
mkdir -p "$T69_HOME"

set +e
(HOME="$T69_HOME" "$ROOT_DIR/bin/trackfw" update harness \
  --targets kiro-git-branch-guard,kiro-credential-guard --install-missing) \
  >"$WORK/s69-update-harness.log" 2>&1
s69setup_status=$?
set -e
if [[ $s69setup_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/setup]: update harness saiu com $s69setup_status" >&2
  cat "$WORK/s69-update-harness.log" >&2
  exit 1
fi

T69_GBG_HOOKS="$T69_HOME/.kiro/hooks/trackfw-git-branch-guard.json"
T69_CG_HOOKS="$T69_HOME/.kiro/hooks/trackfw-credential-guard.json"
T69_GBG_SCRIPT="$T69_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
if [[ ! -s "$T69_GBG_HOOKS" ]] || [[ ! -s "$T69_CG_HOOKS" ]] || [[ ! -x "$T69_GBG_SCRIPT" ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/setup]: fixture incompleta — esperava $T69_GBG_HOOKS, $T69_CG_HOOKS e $T69_GBG_SCRIPT (executável)" >&2
  exit 1
fi

# --- braço baseline: os dois arquivos dedicados do Kiro, ambos apontando
# para scripts presentes e executáveis -> validate passa em silêncio ------
T69_OK="$WORK/s69-project-ok"
scaffold_adr_req_project "$T69_OK"

set +e
s69ok_out=$(cd "$T69_OK" && HOME="$T69_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
s69ok_status=$?
set -e
if [[ $s69ok_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline]: fiação Kiro íntegra deveria passar, saiu com $s69ok_status" >&2
  echo "  output: $s69ok_out" >&2
  exit 1
fi
if grep -qF 'trackfw-git-branch-guard.json' <<<"$s69ok_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline]: fiação Kiro íntegra mas a regra disparou mesmo assim" >&2
  echo "  output: $s69ok_out" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline]"

# --- braço de detecção: script referenciado pelo arquivo DEDICADO do Kiro
# some do disco -> validate deve acusar, citando o arquivo do Kiro — o
# discriminante central deste ML (antes dele, esta checagem nunca lia esse
# arquivo, então nenhuma acusação era possível aqui) -----------------------
rm -f "$T69_GBG_SCRIPT"

T69_BAD="$WORK/s69-project-bad"
scaffold_adr_req_project "$T69_BAD"

assert_fails_with "git-branch-guard-global-hook-resolvable/kiro-dedicated-file/detected" \
  "does not exist" \
  bash -c "cd '$T69_BAD' && exec env HOME='$T69_HOME' '$ROOT_DIR/bin/trackfw' validate"

s69bad_out=$(cd "$T69_BAD" && HOME="$T69_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1 || true)
if ! grep -qF 'trackfw-git-branch-guard.json' <<<"$s69bad_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/detected]: mensagem não cita o arquivo dedicado do Kiro (trackfw-git-branch-guard.json)" >&2
  echo "  output: $s69bad_out" >&2
  exit 1
fi
if ! grep -qF 'Kiro' <<<"$s69bad_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/detected]: mensagem não cita o CLI (Kiro)" >&2
  echo "  output: $s69bad_out" >&2
  exit 1
fi

# --- não-regressão + não-duplicação: com o script do git-branch-guard
# ausente, o credential-guard do Kiro (arquivo separado, script intacto)
# continua em silêncio, E a violation do git-branch-guard aparece exatamente
# 1 vez (não uma vez por arquivo/guard) -------------------------------------
s69bad_gbg_count=$(grep -oF 'trackfw-git-branch-guard.json' <<<"$s69bad_out" | wc -l | tr -d ' ')
if [[ "$s69bad_gbg_count" -ne 1 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/no-double-report]: esperado exatamente 1 ocorrência da violation do Kiro, obteve $s69bad_gbg_count" >&2
  echo "  output: $s69bad_out" >&2
  exit 1
fi
if grep -qF 'trackfw-credential-guard.json' <<<"$s69bad_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/no-regression]: credential-guard do Kiro (arquivo intacto) não deveria disparar, mas apareceu na saída" >&2
  echo "  output: $s69bad_out" >&2
  exit 1
fi
echo "OK   [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/no-double-report-and-no-regression]"

# ---------------------------------------------------------------------------
# Cenário 70 — check-ship-parity.sh: a nova cena "squash-merge-warning" (AC8,
# ROADMAP-2026-08-18-branch-prune-com-dry-run-por-padrao-e-heuristica-de-arquivos-tocados, ML-2B)
# é falsificável — sabota o `detectPendingSquashMerges` do Node.js reintroduzindo o falso
# positivo que a heurística de arquivos-tocados (evaluateBranchIntegration) existe para
# eliminar: a condição `evalResult.decision === BRANCH_PRUNE_DECISION.PENDING_WORK` vira `true`
# incondicional, fazendo o Node avisar sobre TODA branch remota não-ancestral de origin/main —
# inclusive a que foi squash-mergeada e está apenas defasada (feat/a do fixture da cena e).
# Go e Python não são tocados. Prova que o gate de paridade novo reprova quando um runtime
# diverge nesse comportamento específico, não apenas na formatação textual.
# ---------------------------------------------------------------------------
T70="$WORK/s70"
mkdir -p "$T70/scripts"
setup_npm_tree "$T70"
ln -s "$ROOT_DIR/pypi" "$T70/pypi"
cp "$ROOT_DIR/scripts/check-ship-parity.sh" "$T70/scripts/"

sed 's/if (evalResult.decision === BRANCH_PRUNE_DECISION.PENDING_WORK) {/if (true) { \/\/ [falsified] pending-work gate removed/' \
  "$ROOT_DIR/npm/src/ship/runner.js" > "$T70/npm/src/ship/runner.js"

# Guard: garantir que a corrupção foi aplicada antes de rodar o gate.
if cmp -s "$ROOT_DIR/npm/src/ship/runner.js" "$T70/npm/src/ship/runner.js"; then
  echo "FAIL [falsify/setup-s70]: sed não alterou runner.js — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

assert_fails_with "ship-parity/squash-merge-warning-false-positive" \
  "stale-but-integrated feat/a must never appear in a warning" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T70/scripts/check-ship-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 71 — check-doctor-parity.sh (ML-2B, ROADMAP-2026-08-18-doctor-detecta-artefato-fora-
# do-manifesto-e-inverte-a-ordem-de-persistencia) is falsifiable against the exact near-miss the
# ML-2A audit trail flagged before it ever shipped: ClassifyDoctor's `!inspection.Registered`
# discriminant (internal/integrations/doctor.go) reverted to `!inspection.Managed` — the
# unregistered-write branch keying off "this claim owns the manifest entry" instead of "some
# entry exists at all". A destination legitimately registered under a DIFFERENT claim then reads
# Managed=false while State stays Current, and the sabotaged binary reports it as an
# "unregistered write" — precisely the dominant false positive `doctor` exists to avoid,
# reproduced end to end through the real `doctor` command via check-doctor-parity.sh's own
# scenario (e) "registered-under-different-claim" fixture (manifest claim retargeted to a
# different item, sha256/bytes untouched). Baseline arm proves the gate passes clean against the
# unmodified Go binary before the paired detection arm proves the single-literal corruption
# makes it fail — single-delta design (only the one case-clause identifier changes, nothing in
# the gate's own assertions).
# ---------------------------------------------------------------------------
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-doctor-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s71]: check-doctor-parity.sh failed against the UNMODIFIED Go binary — baseline must be green before the detection arm means anything" >&2
  exit 1
fi
echo "OK   [falsify/doctor-parity/registered-under-different-claim/baseline-clean]"

T71C_GO_MOD="$WORK/s71-corrupt-go"
mkdir -p "$T71C_GO_MOD/cmd" "$T71C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T71C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T71C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T71C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T71C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/integrations/doctor.go" "$T71C_GO_MOD/internal/integrations/doctor.go" \
  'case !inspection.Registered && inspection.State == StateCurrent:' \
  'case !inspection.Managed && inspection.State == StateCurrent:' \
  "s71-go"

T71C_GO_BIN="$WORK/s71-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T71C_GO_BIN")"
build_go_or_fail "setup-s71-go-corrupt-build" "$T71C_GO_MOD" "$T71C_GO_BIN"

assert_fails_with "doctor-parity/registered-under-different-claim-false-positive" \
  "doctor-parity/registered-under-different-claim" \
  env GO_BIN="$T71C_GO_BIN" bash "$ROOT_DIR/scripts/check-doctor-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 72 — check-doctor-parity.sh (ML-2C, ROADMAP-2026-08-18-doctor-detecta-artefato-fora-
# do-manifesto-e-inverte-a-ordem-de-persistencia) falsifies the analogous near-miss for
# ClassifyDoctor's new unknown-content class: `!inspection.Registered` reverted to
# `!inspection.Managed` in the unknown-content case-clause. A destination registered under a
# DIFFERENT claim, whose content ALSO drifted (Managed=false, Registered=true, State=Modified),
# then satisfies the sabotaged `!Managed` condition and gets misreported as unknown-content —
# exactly the state that must stay silent (it belongs to the other claim, not this one).
# Cenário 71's own scenario (e) cannot discriminate this corruption: its State stays Current, so
# it never reaches the unknown-content case regardless of which discriminant is used — this is
# why check-doctor-parity.sh's scenario (f) "registered-under-different-claim-content-drifted"
# exists (added by ML-2C specifically to give this scenario something non-vacuous to falsify
# against). Baseline arm proves the gate passes clean against the unmodified Go binary before the
# paired detection arm proves the single-literal corruption makes it fail — single-delta design,
# same shape as Cenário 71.
# ---------------------------------------------------------------------------
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-doctor-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s72]: check-doctor-parity.sh failed against the UNMODIFIED Go binary — baseline must be green before the detection arm means anything" >&2
  exit 1
fi
echo "OK   [falsify/doctor-parity/registered-under-different-claim-content-drifted/baseline-clean]"

T72C_GO_MOD="$WORK/s72-corrupt-go"
mkdir -p "$T72C_GO_MOD/cmd" "$T72C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T72C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T72C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T72C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T72C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/integrations/doctor.go" "$T72C_GO_MOD/internal/integrations/doctor.go" \
  'case !inspection.Registered && inspection.State == StateModified:' \
  'case !inspection.Managed && inspection.State == StateModified:' \
  "s72-go"

T72C_GO_BIN="$WORK/s72-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T72C_GO_BIN")"
build_go_or_fail "setup-s72-go-corrupt-build" "$T72C_GO_MOD" "$T72C_GO_BIN"

assert_fails_with "doctor-parity/registered-under-different-claim-content-drifted-false-positive" \
  "doctor-parity/registered-under-different-claim-content-drifted" \
  env GO_BIN="$T72C_GO_BIN" bash "$ROOT_DIR/scripts/check-doctor-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 73 — check-ship-force-parity.sh (ML-1B, ROADMAP-2026-08-19-caminho-governado-para-
# push-forcado-e-tag-de-release.md) is falsifiable against the exact regression the ADR exists to
# prevent: `--force-with-lease` silently degrading to raw `--force` in the push-arg construction
# (internal/commands/ship.go's single `"--force-with-lease"` string literal). A single-literal
# corruption in an isolated Go copy, exercised end to end through the real `ship` command by the
# gate's own scenario (e) "remote-advanced-lease-mismatch" fixture: a second clone pushes a
# legitimate commit to the same branch, our clone's remote-tracking ref is pinned stale on
# purpose (fetch refspec restricted to main only), so the correct `--force-with-lease` refuses
# (stale lease — real git safety semantics) while a raw `--force` pushes through regardless and
# destroys the other clone's commit. This is the semantic discriminant the ADR calls for — string-
# inspecting the push argv would not catch a runtime that computes the SAME argv through a
# different, equally-wrong code path, but this scenario observes the actual git push OUTCOME
# (exit code + whether the other party's commit survives on the remote), so it does. Baseline arm
# proves check-ship-force-parity.sh passes clean against the unmodified Go binary before the
# paired detection arm proves the single-literal corruption makes it fail — same single-delta
# design as Cenários 71/72.
# ---------------------------------------------------------------------------
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-ship-force-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s73]: check-ship-force-parity.sh failed against the UNMODIFIED Go binary — baseline must be green before the detection arm means anything" >&2
  exit 1
fi
echo "OK   [falsify/ship-force-parity/remote-advanced-lease-mismatch/baseline-clean]"

T73C_GO_MOD="$WORK/s73-corrupt-go"
mkdir -p "$T73C_GO_MOD/cmd" "$T73C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T73C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T73C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T73C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T73C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/commands/ship.go" "$T73C_GO_MOD/internal/commands/ship.go" \
  '"--force-with-lease"' \
  '"--force"' \
  "s73-go"

T73C_GO_BIN="$WORK/s73-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T73C_GO_BIN")"
build_go_or_fail "setup-s73-go-corrupt-build" "$T73C_GO_MOD" "$T73C_GO_BIN"

assert_fails_with "ship-force-parity/remote-advanced-lease-mismatch-raw-force-false-negative" \
  "must refuse when the remote advances past the recorded lease" \
  env GO_BIN="$T73C_GO_BIN" bash "$ROOT_DIR/scripts/check-ship-force-parity.sh"


# ---------------------------------------------------------------------------
# Cenário 74 — scripts/trackfw-git-branch-guard.sh (ML-3A, ROADMAP-2026-08-19-
# caminho-governado-para-push-forcado-e-tag-de-release.md / REQ-2026-08-19-
# guard-nao-bloqueia-comandos-destrutivos-de-working-tree-em-repo-compartilhado-
# por-agentes.md) — a nova classe de comandos destrutivos de working tree
# (stash/reset --hard/clean -f|-x/restore <path>/checkout -- <path>|checkout .)
# ganha um par baseline+detecção POR COMANDO, cobrindo as DUAS direções que
# a REQ nomeia como risco: (a) o comando FICA bloqueado — corrompe o rótulo
# do case que reconhece o subcomando, provando que o bloqueio depende do
# literal novo, não de coincidência; (b) o comando LIBERADO continua livre —
# corrompe o discriminante que separa a forma segura da perigosa (o próprio
# risco DOMINANTE que a REQ nomeia: super-bloquear é pior que sub-bloquear),
# provando que a liberação também depende de um literal específico, não de
# um match "solto" que por acaso deixa passar. Mesmo padrão baseline+detecção
# dos Cenários 60-65: braço baseline prova que o guard LIMPO se comporta como
# esperado nos dois sentidos; braço de detecção corrompe UM literal isolado
# do gerador Go, reconstrói o script a partir de um módulo Go isolado, e
# prova que a mudança de comportamento é exatamente a esperada.
# ---------------------------------------------------------------------------

T74_BASE_MOD="$WORK/s74-base-mod"
mkdir -p "$T74_BASE_MOD/cmd" "$T74_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74_BASE_MOD/go.sum"

T74_BASE_OUT="$WORK/s74-base-out"
run_go_guard_dump "setup-s74-go-baseline-build" "$T74_BASE_MOD" "$T74_BASE_OUT"
T74_BASE_SCRIPT="$T74_BASE_OUT/scripts/trackfw-git-branch-guard.sh"

# --- 74a — git stash: bare/push/save/clear/drop bloqueiam, list/show livres ---
assert_guard_exit "git-branch-guard/stash/baseline-blocks-bare" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash"}}' 2
assert_guard_exit "git-branch-guard/stash/baseline-blocks-drop" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash drop"}}' 2
assert_guard_exit "git-branch-guard/stash/baseline-frees-list" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash list"}}' 0
assert_guard_exit "git-branch-guard/stash/baseline-frees-show" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash show"}}' 0

T74A_MOD="$WORK/s74a-mod"
mkdir -p "$T74A_MOD/cmd" "$T74A_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74A_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74A_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74A_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74A_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74A_MOD/internal/generators/scaffold.go" \
  '      stash)
' \
  '      __never_matches_s74a__)
' \
  "s74a-go-stash-case-label-removed"
T74A_OUT="$WORK/s74a-out"
run_go_guard_dump "setup-s74a-go-corrupted-build" "$T74A_MOD" "$T74A_OUT"
assert_guard_exit "git-branch-guard/stash/detection-catches-bypass" \
  "$T74A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git stash"}}' 0

T74B_MOD="$WORK/s74b-mod"
mkdir -p "$T74B_MOD/cmd" "$T74B_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74B_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74B_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74B_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74B_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74B_MOD/internal/generators/scaffold.go" \
  '          list|show)
' \
  '          __never_matches_s74b__)
' \
  "s74b-go-stash-allowlist-removed"
T74B_OUT="$WORK/s74b-out"
run_go_guard_dump "setup-s74b-go-corrupted-build" "$T74B_MOD" "$T74B_OUT"
assert_guard_exit "git-branch-guard/stash/detection-catches-overblock-list" \
  "$T74B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git stash list"}}' 2

# --- 74c — git reset --hard bloqueia; --soft/--mixed/sem flag livres --------
assert_guard_exit "git-branch-guard/reset-hard/baseline-blocks" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git reset --hard"}}' 2
assert_guard_exit "git-branch-guard/reset-hard/baseline-frees-soft" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git reset --soft HEAD~1"}}' 0
assert_guard_exit "git-branch-guard/reset-hard/baseline-frees-bare" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git reset"}}' 0

T74C_MOD="$WORK/s74c-mod"
mkdir -p "$T74C_MOD/cmd" "$T74C_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74C_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74C_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74C_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74C_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74C_MOD/internal/generators/scaffold.go" \
  '      reset)
' \
  '      __never_matches_s74c__)
' \
  "s74c-go-reset-case-label-removed"
T74C_OUT="$WORK/s74c-out"
run_go_guard_dump "setup-s74c-go-corrupted-build" "$T74C_MOD" "$T74C_OUT"
assert_guard_exit "git-branch-guard/reset-hard/detection-catches-bypass" \
  "$T74C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git reset --hard"}}' 0

T74D_MOD="$WORK/s74d-mod"
mkdir -p "$T74D_MOD/cmd" "$T74D_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74D_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74D_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74D_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74D_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74D_MOD/internal/generators/scaffold.go" \
  '            --hard)
' \
  '            *)
' \
  "s74d-go-reset-hard-discriminant-widened"
T74D_OUT="$WORK/s74d-out"
run_go_guard_dump "setup-s74d-go-corrupted-build" "$T74D_MOD" "$T74D_OUT"
assert_guard_exit "git-branch-guard/reset-hard/detection-catches-overblock-soft" \
  "$T74D_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git reset --soft HEAD~1"}}' 2

# --- 74e — git clean -f/-x bloqueia; -n/--dry-run livre (inclusive quando -n
# aparece JUNTO com -f: -n vence, git nunca apaga nada com --dry-run presente) --
assert_guard_exit "git-branch-guard/clean-force/baseline-blocks-f" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git clean -fd"}}' 2
assert_guard_exit "git-branch-guard/clean-force/baseline-frees-dry-run" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git clean -n"}}' 0
assert_guard_exit "git-branch-guard/clean-force/baseline-frees-dry-run-plus-force" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git clean -n -f"}}' 0

T74E_MOD="$WORK/s74e-mod"
mkdir -p "$T74E_MOD/cmd" "$T74E_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74E_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74E_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74E_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74E_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74E_MOD/internal/generators/scaffold.go" \
  '      clean)
' \
  '      __never_matches_s74e__)
' \
  "s74e-go-clean-case-label-removed"
T74E_OUT="$WORK/s74e-out"
run_go_guard_dump "setup-s74e-go-corrupted-build" "$T74E_MOD" "$T74E_OUT"
assert_guard_exit "git-branch-guard/clean-force/detection-catches-bypass" \
  "$T74E_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git clean -fd"}}' 0

T74F_MOD="$WORK/s74f-mod"
mkdir -p "$T74F_MOD/cmd" "$T74F_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74F_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74F_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74F_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74F_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74F_MOD/internal/generators/scaffold.go" \
  '            -n|--dry-run)
' \
  '            __never_matches_s74f__)
' \
  "s74f-go-clean-dry-run-guard-removed"
T74F_OUT="$WORK/s74f-out"
run_go_guard_dump "setup-s74f-go-corrupted-build" "$T74F_MOD" "$T74F_OUT"
assert_guard_exit "git-branch-guard/clean-force/detection-catches-overblock-dry-run-plus-force" \
  "$T74F_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git clean -n -f"}}' 2

# --- 74g — git restore <path> bloqueia; --staged livre ----------------------
assert_guard_exit "git-branch-guard/restore-path/baseline-blocks" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git restore foo.txt"}}' 2
assert_guard_exit "git-branch-guard/restore-path/baseline-frees-staged" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git restore --staged foo.txt"}}' 0

T74G_MOD="$WORK/s74g-mod"
mkdir -p "$T74G_MOD/cmd" "$T74G_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74G_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74G_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74G_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74G_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74G_MOD/internal/generators/scaffold.go" \
  '      restore)
' \
  '      __never_matches_s74g__)
' \
  "s74g-go-restore-case-label-removed"
T74G_OUT="$WORK/s74g-out"
run_go_guard_dump "setup-s74g-go-corrupted-build" "$T74G_MOD" "$T74G_OUT"
assert_guard_exit "git-branch-guard/restore-path/detection-catches-bypass" \
  "$T74G_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore foo.txt"}}' 0

T74H_MOD="$WORK/s74h-mod"
mkdir -p "$T74H_MOD/cmd" "$T74H_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74H_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74H_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74H_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74H_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74H_MOD/internal/generators/scaffold.go" \
  '            --staged)
' \
  '            __never_matches_s74h__)
' \
  "s74h-go-restore-staged-guard-removed"
T74H_OUT="$WORK/s74h-out"
run_go_guard_dump "setup-s74h-go-corrupted-build" "$T74H_MOD" "$T74H_OUT"
assert_guard_exit "git-branch-guard/restore-path/detection-catches-overblock-staged" \
  "$T74H_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore --staged foo.txt"}}' 2

# --- 74h-bis — --staged NUNCA basta sozinho para liberar quando --worktree/-W
# também aparece: "--staged --worktree" restaura os DOIS (afeta o working
# tree), então deve continuar bloqueado mesmo com --staged presente — achado
# do arquiteto (hades-tf/advisor): "git restore --staged" sozinho é o único
# caso liberado pela REQ, não "qualquer --staged".
assert_guard_exit "git-branch-guard/restore-path/baseline-blocks-staged-plus-worktree" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git restore --staged --worktree foo.txt"}}' 2

T74H2_MOD="$WORK/s74h2-mod"
mkdir -p "$T74H2_MOD/cmd" "$T74H2_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74H2_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74H2_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74H2_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74H2_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74H2_MOD/internal/generators/scaffold.go" \
  '            --worktree|-W)
' \
  '            __never_matches_s74h2__)
' \
  "s74h2-go-restore-worktree-discriminant-removed"
T74H2_OUT="$WORK/s74h2-out"
run_go_guard_dump "setup-s74h2-go-corrupted-build" "$T74H2_MOD" "$T74H2_OUT"
assert_guard_exit "git-branch-guard/restore-path/detection-catches-underblock-staged-plus-worktree" \
  "$T74H2_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore --staged --worktree foo.txt"}}' 0

# Auto-discriminação: contra o MESMO build corrompido, "--staged" sozinho
# (sem --worktree) precisa continuar livre — prova que a corrupção isola só o
# discriminante --worktree/-W, não a liberação de --staged como um todo.
assert_guard_exit "git-branch-guard/restore-path/detection-does-not-break-staged-alone" \
  "$T74H2_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore --staged foo.txt"}}' 0

# --- 74i — git checkout -- <path> | checkout . bloqueia; checkout <branch> livre
assert_guard_exit "git-branch-guard/checkout-path/baseline-blocks-dashdash" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git checkout -- foo.txt"}}' 2
assert_guard_exit "git-branch-guard/checkout-path/baseline-blocks-dot" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git checkout ."}}' 2
assert_guard_exit "git-branch-guard/checkout-path/baseline-frees-branch" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git checkout main"}}' 0

T74I_MOD="$WORK/s74i-mod"
mkdir -p "$T74I_MOD/cmd" "$T74I_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74I_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74I_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74I_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74I_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74I_MOD/internal/generators/scaffold.go" \
  '            --|.)
' \
  '            __never_matches_s74i__)
' \
  "s74i-go-checkout-path-discriminant-removed"
T74I_OUT="$WORK/s74i-out"
run_go_guard_dump "setup-s74i-go-corrupted-build" "$T74I_MOD" "$T74I_OUT"
assert_guard_exit "git-branch-guard/checkout-path/detection-catches-bypass" \
  "$T74I_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -- foo.txt"}}' 0

T74J_MOD="$WORK/s74j-mod"
mkdir -p "$T74J_MOD/cmd" "$T74J_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74J_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74J_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74J_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74J_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74J_MOD/internal/generators/scaffold.go" \
  '            --|.)
' \
  '            *)
' \
  "s74j-go-checkout-path-discriminant-widened"
T74J_OUT="$WORK/s74j-out"
run_go_guard_dump "setup-s74j-go-corrupted-build" "$T74J_MOD" "$T74J_OUT"
assert_guard_exit "git-branch-guard/checkout-path/detection-catches-overblock-branch" \
  "$T74J_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout main"}}' 2

# ---------------------------------------------------------------------------
# Cenário 75 — check-release-tag-parity.sh (ML-2B, ROADMAP-2026-08-19-caminho-governado-para-
# push-forcado-e-tag-de-release.md) is falsifiable against the exact regression the ADR exists to
# prevent: an annotated tag silently degrading to a LIGHTWEIGHT tag — the ref pointing straight
# at the commit instead of at the tag object created by the first `gh api` call
# (internal/commands/release.go's `refPayload` marshal, the single `SHA: tagObj.SHA` field). A
# single-literal corruption in an isolated Go copy (`SHA: tagObj.SHA` → `SHA: objectSHA`),
# exercised end to end through the real `release tag` command by the gate's own scenario
# "success" fixture: the gate's `gh` stub returns a tag-object sha deliberately different from
# the fixture's commit sha, so the ref-creation payload's `sha` field is the discriminant — the
# correct binary links it to the tag-object sha, the sabotaged binary links it to the commit sha
# directly (a lightweight tag wearing an annotated tag's success message: `git describe`/`git tag
# -l` would still find it, and the loss is invisible until someone looks for the message on the
# tag object). Baseline arm proves check-release-tag-parity.sh passes clean against the
# unmodified Go binary before the paired detection arm proves the single-literal corruption makes
# it fail — same single-delta design as Cenário 73.
# ---------------------------------------------------------------------------
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s75]: check-release-tag-parity.sh failed against the UNMODIFIED Go binary — baseline must be green before the detection arm means anything" >&2
  exit 1
fi
echo "OK   [falsify/release-tag-parity/success/baseline-clean]"

T75C_GO_MOD="$WORK/s75-corrupt-go"
mkdir -p "$T75C_GO_MOD/cmd" "$T75C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T75C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T75C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T75C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T75C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" "$T75C_GO_MOD/internal/commands/release.go" \
  '}{Ref: "refs/tags/" + tagName, SHA: tagObj.SHA})' \
  '}{Ref: "refs/tags/" + tagName, SHA: objectSHA})' \
  "s75-go"

T75C_GO_BIN="$WORK/s75-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T75C_GO_BIN")"
build_go_or_fail "setup-s75-go-corrupt-build" "$T75C_GO_MOD" "$T75C_GO_BIN"

assert_fails_with "release-tag-parity/success-lightweight-tag-false-negative" \
  "LIGHTWEIGHT-TAG REGRESSION: ref payload 'sha' must equal the tag-object sha" \
  env GO_BIN="$T75C_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 76 — check-release-tag-parity.sh's Scenarios 11-13 (ML-4B, ROADMAP-2026-08-19-
# caminho-governado-para-push-forcado-e-tag-de-release.md, Emenda 1 do ADR) are falsifiable
# against the exact regression Emenda 1 exists to prevent: the commit-target divergence check
# silently disabled, so `release tag` falls back to trusting a LOCAL (forged/stale) ref instead
# of refusing when it disagrees with the forge. A single-literal corruption in an isolated Go
# copy — internal/commands/release.go's `if forgeLocalSHA != "" && forgeLocalSHA != commitObj.SHA {`
# guarded with a `false &&` prefix that never evaluates true — neuters the divergence check
# without touching the forge GET calls or the objectSHA assignment themselves, so the corrupted
# binary still resolves and even PRINTS the forge's sha correctly; only the refusal is gone.
# Exercised end to end through the real `release tag` command by the gate's own Scenario 12
# fixture (origin/main forged via `git update-ref` under a narrowed refspec, refs/heads/main
# reset to match it so the pre-existing local-branch-staleness check cannot discriminate this
# corruption either) — baseline arm proves check-release-tag-parity.sh passes clean against the
# unmodified Go binary before the paired detection arm proves the single-literal corruption
# makes it fail. Same single-delta design as Cenários 73/75.
# ---------------------------------------------------------------------------
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s76]: check-release-tag-parity.sh failed against the UNMODIFIED Go binary — baseline must be green before the detection arm means anything" >&2
  exit 1
fi
echo "OK   [falsify/release-tag-parity/forge-commit-diverges-update-ref/baseline-clean]"

T76_GO_MOD="$WORK/s76-corrupt-go"
mkdir -p "$T76_GO_MOD/cmd" "$T76_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T76_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T76_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T76_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T76_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" "$T76_GO_MOD/internal/commands/release.go" \
  'if forgeLocalSHA != "" && forgeLocalSHA != commitObj.SHA {' \
  'if false && forgeLocalSHA != "" && forgeLocalSHA != commitObj.SHA {' \
  "s76-go"

T76_GO_BIN="$WORK/s76-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T76_GO_BIN")"
build_go_or_fail "setup-s76-go-corrupt-build" "$T76_GO_MOD" "$T76_GO_BIN"

assert_fails_with "release-tag-parity/forge-commit-diverges-false-negative" \
  "expected non-zero exit when origin/main is forged via update-ref" \
  env GO_BIN="$T76_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 77 — scripts/check-parity-contract-coverage.sh (ROADMAP-2026-08-20-
# contrato-pinado-no-cli-parity-sem-gate-nomeado.md, ML-1B) — gate created by
# this ML without its own falsification scenario, closing the reported gap:
# a meta-checker without a P4 scenario would be the exact irony the REQ
# exists to prevent one level up ("gate sem cenário de falsificação é gate
# não-verificado").
#
# Baseline fixture covers all 3 heading levels (##/###/####) and all 4 valid
# annotation shapes (gate=, gate=+partial=, gap reason=, none reason=) plus
# one unannotated section, proving the report-mode counts are honest (the
# real docs/cli-parity.md today has zero `none` sections, so that counting
# path is otherwise unexercised). Six single-delta corruptions off that same
# baseline prove each of the 5 documented failure classes plus the unknown-
# key/malformed-prefix parsing rule from the ADR's Emenda 1/Nota de parsing.
# Non-vacuity: neutering the gate-existence check on an isolated copy proves
# the "gate nomeado inexistente" detection arm actually depends on it.
# ---------------------------------------------------------------------------
T77="$WORK/s77"
mkdir -p "$T77/scripts"
cp -r "$ROOT_DIR/scripts/." "$T77/scripts/"

write_s77_fixture() {
  local dest=$1 gate_line=$2 partial_line=$3 gap_line=$4 none_line=$5
  cat > "$dest" <<EOF
# Fixture cli-parity

## Gate section

$gate_line

Prosa qualquer da seção com gate pleno.

### Gate section com partial

$partial_line

Prosa qualquer da seção com cobertura parcial.

#### Gap section

$gap_line

Prosa qualquer da seção sem gate.

## None section

$none_line

Prosa qualquer da seção que não é contrato.
EOF
}

# --- 77a — baseline: as 4 formas válidas anotadas, exit 0 -------------------
# ML-3A (2026-08-20): a 5ª seção ("Unannotated section") que existia aqui até
# a triagem fechar foi REMOVIDA do fixture compartilhado — ela agora reprova
# (ver 77p), e um baseline que se propõe a passar com exit 0 não pode conter
# uma seção que o próprio ML-3A tornou reprovável, ou o baseline de 77b-77o
# passaria a testar "reprova, mas pelo motivo errado" sem que ninguém notasse.
# A cobertura de "seção sem anotação" ganhou fixture dedicado em 77p.
T77A="$T77/s77a-baseline.md"
write_s77_fixture "$T77A" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=cobre so a mecanica -->" \
  "<!-- trackfw-contract: gap reason=nada protege isto ainda -->" \
  "<!-- trackfw-contract: none reason=e so prosa de contexto -->"

T77A_OUT="$WORK/s77a-out"
set +e
T77A_STDOUT=$(bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77A" 2>"$T77A_OUT")
T77A_STATUS=$?
set -e
if [[ $T77A_STATUS -ne 0 ]]; then
  echo "FAIL [falsify/parity-contract-coverage/baseline]: saiu com $T77A_STATUS, esperava 0" >&2
  echo "  stdout: $T77A_STDOUT" >&2
  echo "  stderr: $(cat "$T77A_OUT")" >&2
  exit 1
fi
for expected in \
  "total de seções reais (##/###/####), fora de fences: 4  (## 2 · ### 1 · #### 1)" \
  "gate= (cobertura plena):  1" \
  "gate= com partial=:       1" \
  "gap (contrato SEM gate):  1" \
  "none (não-contrato):      1" \
  "sem anotação:             0" \
  "anotação inválida:        0"; do
  if ! grep -qF "$expected" <<<"$T77A_STDOUT"; then
    echo "FAIL [falsify/parity-contract-coverage/baseline]: relatório não contém '$expected'" >&2
    echo "  stdout: $T77A_STDOUT" >&2
    exit 1
  fi
done
echo "OK   [falsify/parity-contract-coverage/baseline]: 3 níveis de título + 4 estados válidos, todas anotadas, contagens corretas"

# --- 77b — gate= sem caminho nomeado (vazio) — regra GERAL da Emenda 2:
#           chave PRESENTE com valor vazio reprova, mensagem nomeia a chave --
T77B="$T77/s77b-gate-empty.md"
write_s77_fixture "$T77B" \
  "<!-- trackfw-contract: gate= -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/gate-empty" \
  "gate= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77B"

# --- 77c — gate nomeado que não existe no disco — aponta para o vazio -----
T77C="$T77/s77c-gate-missing.md"
write_s77_fixture "$T77C" \
  "<!-- trackfw-contract: gate=scripts/does-not-exist-anywhere.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/gate-missing-on-disk" \
  "gate nomeado não existe no disco: scripts/does-not-exist-anywhere.sh" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77C"

# --- 77d — gap sem reason= --------------------------------------------------
T77D="$T77/s77d-gap-no-reason.md"
write_s77_fixture "$T77D" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/gap-no-reason" \
  "gap sem reason= (motivo obrigatório)" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77D"

# --- 77e — none sem reason= -------------------------------------------------
T77E="$T77/s77e-none-no-reason.md"
write_s77_fixture "$T77E" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none -->"
assert_fails_with "parity-contract-coverage/none-no-reason" \
  "none sem reason= (motivo obrigatório)" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77E"

# --- 77f — chave desconhecida (Nota de parsing da Emenda 1: `reson=` não
#           vira parte do valor anterior em silêncio) -----------------------
T77F="$T77/s77f-unknown-key.md"
write_s77_fixture "$T77F" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap reson=motivo com erro de digitação -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/unknown-key" \
  "chave desconhecida na anotação" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77F"

# --- 77g — anotação malformada: prefixo trackfw-contract: sem estado
#           reconhecido (nem gate=, nem gap, nem none) ---------------------
T77G="$T77/s77g-malformed.md"
write_s77_fixture "$T77G" \
  "<!-- trackfw-contract: estado-nao-reconhecido foo -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/malformed-state" \
  "prefixo trackfw-contract sem estado reconhecido" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77G"

# --- 77h — não-vacuidade: neutraliza a checagem de existência do gate numa
#           cópia isolada do checker e prova que o caso 77c fica mudo -------
T77H_SCRIPT="$T77/scripts/check-parity-contract-coverage-neutered.sh"
corrupt_literal \
  "$T77/scripts/check-parity-contract-coverage.sh" "$T77H_SCRIPT" \
  'if not os.path.isfile(os.path.join(ROOT, path)):' \
  'if False:' \
  "s77h-neuter-gate-existence-check"
chmod +x "$T77H_SCRIPT"
assert_succeeds "parity-contract-coverage/gate-missing/non-vacuity" \
  bash "$T77H_SCRIPT" "$T77C"

# --- 77i — partial= presente com valor vazio (ADR Emenda 2, 6º caso de
#           reprovação): a regra é GERAL — não é um `if` dedicado a
#           `partial`, é o mesmo laço "toda chave presente exige valor
#           não-vazio" que 77b já exercita para `gate=`. Este cenário prova
#           que a MESMA regra também pega `partial=` sem precisar de código
#           novo por chave — o que 77b sozinho não provaria. -----------------
T77I="$T77/s77i-partial-empty.md"
write_s77_fixture "$T77I" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial= -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/partial-empty" \
  "partial= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77I"

# --- 77j — reason= presente com valor vazio no estado `gap`, distinto de
#           reason= AUSENTE (77d testa a chave nem escrita; aqui a chave
#           está escrita e vazia) — single-delta: só o `gap_line` muda em
#           relação ao baseline 77a. -----------------------------------------
T77J="$T77/s77j-reason-empty-gap.md"
write_s77_fixture "$T77J" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason= -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/reason-empty/gap" \
  "reason= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77J"

# --- 77k — mesmo caso, estado `none` (77e testa a chave AUSENTE; este
#           fixture prova que o laço geral também cobre o outro dos dois
#           estados que compartilham o branch `state in ("gap", "none")`,
#           não só `gap`) — single-delta relativo ao baseline. --------------
T77K="$T77/s77k-reason-empty-none.md"
write_s77_fixture "$T77K" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason= -->"
assert_fails_with "parity-contract-coverage/reason-empty/none" \
  "reason= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77K"

# --- 77l — não-vacuidade da regra GERAL da Emenda 2: neutraliza o laço
#           "toda chave presente exige valor não-vazio" numa cópia isolada
#           do checker (o mesmo laço que 77i/77j/77k dependem, não um `if`
#           por chave) e prova que 77i fica mudo — isolando que é o LAÇO,
#           não os checks específicos de gate=/reason= já removidos deste
#           checker, quem sustenta a detecção. --------------------------------
T77L_SCRIPT="$T77/scripts/check-parity-contract-coverage-empty-neutered.sh"
corrupt_literal \
  "$T77/scripts/check-parity-contract-coverage.sh" "$T77L_SCRIPT" \
  'for key in sorted(kv):' \
  'for key in []:' \
  "s77l-neuter-general-empty-check"
chmod +x "$T77L_SCRIPT"
assert_succeeds "parity-contract-coverage/empty-value/non-vacuity" \
  bash "$T77L_SCRIPT" "$T77I"

# --- 77m — chave desconhecida DEPOIS de uma chave real, dentro do valor
#           dela (ML-1B-ter, conformidade sobre 77f: aquele fixture só prova
#           chave desconhecida ANTES da primeira chave real — 'reson=' logo
#           após 'gap ', sem nenhuma chave reconhecida vindo antes dele no
#           corpo. Aqui 'reson=' vem DEPOIS de 'reason=', posição que
#           extract_kv() engolia em silêncio como parte do valor de
#           'reason=' antes deste ML — exatamente o cenário que a Nota de
#           parsing do ADR descreve como o motivo de existir a regra.) ------
T77M="$T77/s77m-unknown-key-positional.md"
write_s77_fixture "$T77M" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=motivo qualquer reson=erro de digitacao -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/unknown-key/positional-after-known-key" \
  "chave desconhecida na anotação: 'reson='" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77M"

# --- 77n — controle de não-regressão do heurístico do 77m: texto livre
#           contendo '=' que NÃO é typo de gate=/partial=/reason= não pode
#           reprovar — 'LANG=' (maiúsculo, nunca bate no heurístico) e
#           '--flag=' (o 'f' de 'flag' é precedido por '-', não por espaço,
#           então nunca é considerado candidato a chave). Ambos citados no
#           handoff do ML como o risco central deste heurístico: reprovar um
#           motivo legítimo é pior que deixar passar um typo raro. ---------
T77N="$T77/s77n-freetext-equals-ok.md"
write_s77_fixture "$T77N" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=o comportamento sob LANG=pt_BR roda com --flag=valor e nao e comparado entre os 3 CLIs -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_succeeds "parity-contract-coverage/unknown-key/freetext-equals-non-regression" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77N"

# --- 77o — não-vacuidade do 77m: neutraliza o laço find_unknown_key_typos()
#           numa cópia isolada do checker (o mecanismo que 77m depende, e
#           que 77f sozinho NÃO exercitaria, já que 77f é pego pelo
#           `leading` antigo) e prova que 77m fica mudo. -------------------
T77O_SCRIPT="$T77/scripts/check-parity-contract-coverage-positional-neutered.sh"
corrupt_literal \
  "$T77/scripts/check-parity-contract-coverage.sh" "$T77O_SCRIPT" \
  'for token in sorted(set(find_unknown_key_typos(body, positions))):' \
  'for token in sorted(set([])):' \
  "s77o-neuter-positional-unknown-key-check"
chmod +x "$T77O_SCRIPT"
assert_succeeds "parity-contract-coverage/unknown-key/positional-non-vacuity" \
  bash "$T77O_SCRIPT" "$T77M"

# --- 77p — ML-3A: seção nova SEM anotação agora reprova (a triagem fechou,
#           177/177 — deixou de ser "ainda não triada" e passou a ser
#           regressão). Single-delta contra o baseline 77a: acrescenta UM
#           cabeçalho a mais, sem anotação nenhuma, e prova que só ele muda
#           o resultado de exit 0 para exit 1. Braço de baseline embutido:
#           77a acima já prova que as 4 formas válidas, sozinhas, passam —
#           este cenário isola que é especificamente a AUSÊNCIA de anotação
#           na 5ª seção que derruba o exit code, não alguma outra diferença
#           entre os fixtures. -------------------------------------------
T77P="$T77/s77p-new-section-unannotated.md"
cat "$T77A" > "$T77P"
cat >> "$T77P" <<'EOF'

## Seção nova sem passar pela triagem

Prosa qualquer — ninguém anotou esta seção. Antes do ML-3A isso só entrava
no relatório; agora é regressão e precisa reprovar.
EOF
assert_fails_with "parity-contract-coverage/unannotated-section/blocks-since-ml3a" \
  "seção sem anotação trackfw-contract" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77P"

# ---------------------------------------------------------------------------
# Cenário 78 — check-agent-hooks-parity.sh: Node.js muda o default `tools` do
#              custom agent Amazon Q (.amazonq/cli-agents/q_cli_default.json)
#              de `['*']` para `['read']` → gate detecta o drift estrutural.
#
# Objetivo (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco,
# ML-1B): windsurf e amazonq foram adicionados a CLIS/marker_for/hookfile_for
# em check-agent-hooks-parity.sh neste ML — sem prova própria, a extensão
# poderia estar presente só de nome (ex.: um marker_for/hookfile_for errado
# fazendo o loop silenciosamente comparar um arquivo vazio/inexistente dos
# dois lados, "passando" por vacuidade mútua) sem o comparador estrutural
# jamais ser realmente exercitado para os dois CLIs novos. Corrompe apenas o
# literal `tools: ['*']` de `injectAmazonQHooks` (Node) numa cópia isolada de
# npm/ — único no arquivo (confirmado por grep antes de escrever este
# cenário) — o gate deve reprovar com o path JSON divergente ($.tools[0]) no
# diagnóstico.
#
# Reaproveita o padrão do Cenário 44: o gate roda a partir de sua própria
# cópia (T78/scripts/), cujo ROOT_DIR relativo aponta para o fixture —
# NODE_CLI (não sobrepunível por env em check-agent-hooks-parity.sh, ao
# contrário de GO_BIN/PY_ROOT) resolve para a árvore corrompida via
# setup_npm_tree; GO_BIN e PY_ROOT apontam para o binário/pypi reais e
# não-corrompidos do repositório (só o Node precisa estar isolado aqui).
# ---------------------------------------------------------------------------
T78="$WORK/s78"
mkdir -p "$T78/scripts"
setup_npm_tree "$T78"
cp "$ROOT_DIR/scripts/check-agent-hooks-parity.sh" "$T78/scripts/"

corrupt_literal \
  "$ROOT_DIR/npm/src/generators/hooks.js" "$T78/npm/src/generators/hooks.js" \
  $'    tools: [\'*\'],' \
  $'    tools: [\'read\'],' \
  "s78-node-amazonq-default-tools"

assert_fails_with "agent-hooks-parity/amazonq/go-vs-node-tools-drift-not-detected" \
  "agent-hooks-parity/amazonq/go-vs-node" \
  env GO_BIN="$ROOT_DIR/bin/trackfw" PY_ROOT="$ROOT_DIR/pypi" bash "$T78/scripts/check-agent-hooks-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 79 — check-validate-parity.sh: `branch_has_wip_roadmap` deixa de
#              aceitar roadmap correspondente em done/ (BranchSlugMatchesRoadmap
#              passa a ignorar doneDirs) → o novo bloco branch_has_wip_roadmap
#              done/ acceptance (caso "match") reprova.
#
# Objetivo (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco,
# ML-2A): o comportamento central desta seção — aceitar roadmap em done/, não só
# wip/, desde a REQ-2026-07-26 — nunca tinha sido exercitado cross-CLI antes deste
# ML. Sem este cenário, o novo bloco de check-validate-parity.sh poderia estar
# presente só de nome (ex.: uma fixture que não chega a exercitar o matching real)
# sem jamais provar que reprovaria se a aceitação de done/ quebrasse.
#
# Seam: internal/validator/validator.go's BranchSlugMatchesRoadmap concatena
# wipDirs+doneDirs num único slice `dirs` ANTES de varrer candidatos — tanto o
# match quanto a lista `candidates` usada na mensagem de orientação dependem dele.
# Delta de literal único: `append(append([]string{}, wipDirs...), doneDirs...)`
# vira `append([]string{}, wipDirs...)`, dropando doneDirs inteiramente. Como
# Go é a implementação de referência que Node.js/Python espelham (não uma
# reimplementação paralela), esta é a única sabotagem necessária: um roadmap em
# done/ com slug correspondente deixa de ser encontrado (candidates também fica
# vazio, já que é construído do mesmo `dirs`), reproduzindo exatamente o sintoma
# "branch bloqueada mesmo com o roadmap já em done/" que a REQ-2026-07-26 existe
# para evitar.
#
# GO_BIN override (adicionado a check-validate-parity.sh neste mesmo ML) aponta
# o gate REAL, sem cópia, para o binário sabotado — só Go é isolado; Node.js e
# Python seguem os reais e corretos do repositório, o que é exatamente o ponto:
# prova que o novo bloco detecta uma regressão em QUALQUER um dos 3 runtimes,
# não só quando todos quebram juntos.
# ---------------------------------------------------------------------------
T79="$WORK/s79"
mkdir -p "$T79/cmd" "$T79/internal"
cp -r "$ROOT_DIR/cmd/." "$T79/cmd/"
cp -r "$ROOT_DIR/internal/." "$T79/internal/"
cp "$ROOT_DIR/go.mod" "$T79/go.mod"
cp "$ROOT_DIR/go.sum" "$T79/go.sum"

sed 's/dirs := append(append(\[\]string{}, wipDirs\.\.\.), doneDirs\.\.\.)/dirs := append([]string{}, wipDirs...) \/\/ [falsified] doneDirs dropped/' \
  "$ROOT_DIR/internal/validator/validator.go" > "$T79/internal/validator/validator.go"

# Guarda de padrão: garantir que o sed encontrou e alterou o alvo.
if cmp -s "$ROOT_DIR/internal/validator/validator.go" "$T79/internal/validator/validator.go"; then
  echo "FAIL [falsify/setup-s79]: sed não alterou validator.go — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

T79_BIN="$WORK/s79-bin/trackfw"
mkdir -p "$(dirname "$T79_BIN")"
build_go_or_fail "setup-s79-liveness-build" "$T79" "$T79_BIN"

# Braço de baseline: o binário REAL (bin/trackfw, não corrompido) precisa passar
# limpo antes de provar que o corrompido reprova — sem isto, um bloco novo que
# ficasse permanentemente vermelho por um bug de fixture (não pela sabotagem)
# passaria no braço de detecção abaixo pelo motivo errado. GO_BIN absoluto
# explícito, mesma razão do fix do Cenário 4 (make parity exporta
# GO_BIN=bin/trackfw relativo só para a linha deste script no Makefile).
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s79-baseline]: check-validate-parity.sh já reprova com o binário real — prova P4 inválida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/branch-has-wip-roadmap-done-acceptance-baseline]"

assert_fails_with "validate-parity/branch-has-wip-roadmap-done-acceptance-not-detected" \
  "roadmap correspondente em done/ deveria ser aceito" \
  env GO_BIN="$T79_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 80 — check-validate-parity.sh: `credential_guard_hook_resolvable`
#              deixa de detectar script ausente (credentialGuardScriptMarker
#              desativado) → o novo bloco do ML-3A reprova.
#
# Objetivo (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco,
# ML-3A): o novo bloco de check-validate-parity.sh (4 casos: claude-absent,
# claude-present, cursor-absent, cursor-present) seria vacuo sem este cenário P4
# — ele poderia estar verde por acidente sem jamais provar que reprovaria se a
# detecção do script ausente quebrasse.
#
# Seam: internal/validator/validator_credential_guard.go's
# credentialGuardScriptMarker é o único parâmetro que distingue "estou
# buscando uma entrada de credential-guard" de "não estou buscando nada". Delta
# de literal único: o valor "trackfw-credential-guard.sh" vira
# "trackfw-credential-guard-DISABLED.sh", fazendo collectCommandsWithMarker não
# casar nenhum comando nos fixtures de hook — a regra fica muda para Go enquanto
# Node.js/Python ainda a disparam, expondo a divergência no bloco do ML-3A.
#
# GO_BIN override aponta check-validate-parity.sh para o binário sabotado —
# só Go é isolado; Node.js e Python seguem os reais do repositório, exatamente
# o ponto: prova que o novo bloco detecta uma regressão específica de Go sem
# exigir que os 3 CLIs quebrem juntos.
# ---------------------------------------------------------------------------
T80="$WORK/s80"
mkdir -p "$T80/cmd" "$T80/internal"
cp -r "$ROOT_DIR/cmd/." "$T80/cmd/"
cp -r "$ROOT_DIR/internal/." "$T80/internal/"
cp "$ROOT_DIR/go.mod" "$T80/go.mod"
cp "$ROOT_DIR/go.sum" "$T80/go.sum"

sed 's/const credentialGuardScriptMarker = "trackfw-credential-guard.sh"/const credentialGuardScriptMarker = "trackfw-credential-guard-DISABLED.sh" \/\/ [falsified]/' \
  "$ROOT_DIR/internal/validator/validator_credential_guard.go" > "$T80/internal/validator/validator_credential_guard.go"

# Guarda de padrão: garantir que o sed encontrou e alterou o alvo.
if cmp -s "$ROOT_DIR/internal/validator/validator_credential_guard.go" "$T80/internal/validator/validator_credential_guard.go"; then
  echo "FAIL [falsify/setup-s80]: sed não alterou validator_credential_guard.go — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

T80_BIN="$WORK/s80-bin/trackfw"
mkdir -p "$(dirname "$T80_BIN")"
build_go_or_fail "setup-s80-liveness-build" "$T80" "$T80_BIN"

# Braço de baseline: o binário REAL (bin/trackfw, não corrompido) precisa passar
# limpo antes de provar que o corrompido reprova — sem isto, um bloco novo que
# ficasse permanentemente vermelho por um bug de fixture passaria no braço de
# detecção abaixo pelo motivo errado. GO_BIN absoluto explícito, mesma razão
# do Cenário 79.
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s80-baseline]: check-validate-parity.sh já reprova com o binário real — prova P4 inválida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/credential-guard-hook-resolvable-cross-cli-baseline]"

assert_fails_with "validate-parity/credential-guard-hook-resolvable-not-detected" \
  "credential_guard_hook_resolvable parity (claude-absent/go): expected violation" \
  env GO_BIN="$T80_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 81 — check-validate-parity.sh: `credential_guard_hook_resolvable`
#              não detecta script não-executável (exec-bit check invertido) →
#              o bloco ML-4B (cg-claude-noexec) reprova.
#
# Objetivo (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco,
# ML-4B/A-1): o novo caso cg-claude-noexec de check-validate-parity.sh seria
# vacuo sem este cenário P4 — ele poderia estar verde por acidente sem jamais
# provar que reprovaria se a detecção do exec-bit quebrasse.
#
# Seam: internal/validator/validator_credential_guard.go, condição
# `info.Mode()&0111 == 0:` dentro de `case CurrentGOOS != "windows" && info.Mode()&0111 == 0:`
# — o sed mira o SUBSTRING da checagem de modo, não a cláusula `case` inteira,
# porque o port do #222 Grupo B (ROADMAP-2026-08-31-portar-as-correcoes-do-
# reporter-da-issue-216, ML-1A) prefixou a condição com o guard `CurrentGOOS !=
# "windows" &&` — mesmo precedente do Cenário 179 (`execBit &&` em
# scaffold_doctor.go), que também mira o substring em vez da cláusula
# completa para sobreviver a um guard de plataforma adicionado depois.
# RETARGETED 2026-08-31 ML-1A: âncora era `case info\.Mode()&0111 == 0:`
# (casava a cláusula inteira); virou `info\.Mode()&0111 == 0:` (substring) —
# ver vault/notes/falsify-cenario-pina-linha-de-fonte-por-sed-guard-de-
# plataforma-quebra-2026-08-31.md.
#
# GO_BIN override (mesma convenção dos Cenários 79/80) aponta
# check-validate-parity.sh para o binário sabotado.
# ---------------------------------------------------------------------------
T81="$WORK/s81"
mkdir -p "$T81/cmd" "$T81/internal"
cp -r "$ROOT_DIR/cmd/." "$T81/cmd/"
cp -r "$ROOT_DIR/internal/." "$T81/internal/"
cp "$ROOT_DIR/go.mod" "$T81/go.mod"
cp "$ROOT_DIR/go.sum" "$T81/go.sum"

sed 's/info\.Mode()&0111 == 0:/false \&\& info.Mode()\&0111 == 0: \/\/ [falsified]/' \
  "$ROOT_DIR/internal/validator/validator_credential_guard.go" > "$T81/internal/validator/validator_credential_guard.go"

if cmp -s "$ROOT_DIR/internal/validator/validator_credential_guard.go" "$T81/internal/validator/validator_credential_guard.go"; then
  echo "FAIL [falsify/setup-s81]: sed não alterou validator_credential_guard.go — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

T81_BIN="$WORK/s81-bin/trackfw"
mkdir -p "$(dirname "$T81_BIN")"
build_go_or_fail "setup-s81-liveness-build" "$T81" "$T81_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s81-baseline]: check-validate-parity.sh já reprova com o binário real — prova P4 inválida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/credential-guard-noexec-baseline]"

assert_fails_with "validate-parity/credential-guard-noexec-not-detected" \
  "credential_guard_hook_resolvable parity (claude-noexec" \
  env GO_BIN="$T81_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 82 — check-validate-parity.sh: `credential_guard_hook_resolvable`
#              não detecta hook sem "type":"command" (type check invertido) →
#              o bloco ML-4B (cg-claude-notype) reprova.
#
# Objetivo (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco,
# ML-4B/A-1): o novo caso cg-claude-notype de check-validate-parity.sh seria
# vacuo sem este cenário P4 — ele poderia estar verde por acidente sem jamais
# provar que reprovaria se a detecção do missing-type quebrasse.
#
# Seam: internal/validator/validator_credential_guard.go linha
# `if hf.requiresCommandType && !m.typeIsCommand {` — condição que detecta
# entrada sem o campo obrigatório "type":"command". Delta de literal único:
# `!m.typeIsCommand` vira `m.typeIsCommand` (inversão — agora dispara apenas
# quando o tipo É "command", nunca quando está ausente). Go fica cego ao
# caminho notype; Node.js/Python ficam reais e corretos, expondo a divergência
# no caso cg-claude-notype.
# ---------------------------------------------------------------------------
T82="$WORK/s82"
mkdir -p "$T82/cmd" "$T82/internal"
cp -r "$ROOT_DIR/cmd/." "$T82/cmd/"
cp -r "$ROOT_DIR/internal/." "$T82/internal/"
cp "$ROOT_DIR/go.mod" "$T82/go.mod"
cp "$ROOT_DIR/go.sum" "$T82/go.sum"

sed 's/if hf\.requiresCommandType \&\& !m\.typeIsCommand {/if false \&\& hf.requiresCommandType \&\& !m.typeIsCommand { \/\/ [falsified]/' \
  "$ROOT_DIR/internal/validator/validator_credential_guard.go" > "$T82/internal/validator/validator_credential_guard.go"

if cmp -s "$ROOT_DIR/internal/validator/validator_credential_guard.go" "$T82/internal/validator/validator_credential_guard.go"; then
  echo "FAIL [falsify/setup-s82]: sed não alterou validator_credential_guard.go — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

T82_BIN="$WORK/s82-bin/trackfw"
mkdir -p "$(dirname "$T82_BIN")"
build_go_or_fail "setup-s82-liveness-build" "$T82" "$T82_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s82-baseline]: check-validate-parity.sh já reprova com o binário real — prova P4 inválida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/credential-guard-notype-baseline]"

assert_fails_with "validate-parity/credential-guard-notype-not-detected" \
  "credential_guard_hook_resolvable parity (claude-notype" \
  env GO_BIN="$T82_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 83 — check-agent-hooks-parity.sh: Amazon Q `deniedCommands` removido
#              correladamente dos 3 stacks → o guard P3 (ML-4B/B-1) reprova.
#
# Objetivo (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco,
# ML-4B/B-1): um drop correlacionado de deniedCommands nos 3 stacks passaria
# compare_json (ambos os lados escrevem o campo com o mesmo valor errado) e
# o P2 guard (string trackfw-git-branch-guard.sh ainda presente). Sem o guard
# P3, a regressão seria silenciosa. Este cenário prova que o guard a detecta.
#
# Estratégia tri-stack: sufixar " DISABLED" ao padrão literal em cada stack —
# os 3 CLIs ainda escrevem deniedCommands, mas com padrão diferente. grep -F
# para '^git (commit|push|checkout -b)' falha nos 3; compare_json passa.
#   Go:     const gitDenyPattern em agentfiles.go  (GO_BIN override)
#   Node.js: const GBG_DENIED_COMMANDS_PATTERN em hooks.js  (setup_npm_tree
#            + script copiado para T83/scripts/ → NODE_CLI resolve via ROOT_DIR)
#   Python: _GIT_GUARD_DENIED_COMMANDS_PATTERN em hooks.py  (PY_ROOT override)
# ---------------------------------------------------------------------------
T83="$WORK/s83"
mkdir -p "$T83/cmd" "$T83/internal" "$T83/scripts"
cp -r "$ROOT_DIR/cmd/." "$T83/cmd/"
cp -r "$ROOT_DIR/internal/." "$T83/internal/"
cp "$ROOT_DIR/go.mod" "$T83/go.mod"
cp "$ROOT_DIR/go.sum" "$T83/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/agentfiles.go" "$T83/internal/generators/agentfiles.go" \
  'const gitDenyPattern = `^git (commit|push|checkout -b)`' \
  'const gitDenyPattern = `DENIED_COMMANDS_REMOVED` // [falsified]' \
  "s83-go-denied-commands-pattern"

T83_BIN="$WORK/s83-bin/trackfw"
mkdir -p "$(dirname "$T83_BIN")"
build_go_or_fail "setup-s83-liveness-build" "$T83" "$T83_BIN"

setup_npm_tree "$T83"
corrupt_literal \
  "$ROOT_DIR/npm/src/generators/hooks.js" "$T83/npm/src/generators/hooks.js" \
  "const GBG_DENIED_COMMANDS_PATTERN = '^git (commit|push|checkout -b)'" \
  "const GBG_DENIED_COMMANDS_PATTERN = 'DENIED_COMMANDS_REMOVED' // [falsified]" \
  "s83-node-denied-commands-pattern"

mkdir -p "$T83/pypi"
cp -r "$ROOT_DIR/pypi/." "$T83/pypi/"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/generators/hooks.py" "$T83/pypi/trackfw/generators/hooks.py" \
  "_GIT_GUARD_DENIED_COMMANDS_PATTERN = '^git (commit|push|checkout -b)'" \
  "_GIT_GUARD_DENIED_COMMANDS_PATTERN = 'DENIED_COMMANDS_REMOVED'  # [falsified]" \
  "s83-python-denied-commands-pattern"

cp "$ROOT_DIR/scripts/check-agent-hooks-parity.sh" "$T83/scripts/"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" PY_ROOT="$ROOT_DIR/pypi" bash "$ROOT_DIR/scripts/check-agent-hooks-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s83-baseline]: check-agent-hooks-parity.sh já reprova com os runtimes reais — prova P4 inválida" >&2
  exit 1
fi
echo "OK   [falsify/agent-hooks-parity/amazonq-denied-commands-vacuity-baseline]"

assert_fails_with "agent-hooks-parity/amazonq/denied-commands-not-detected" \
  "agent-hooks-parity/amazonq/go/denied-commands-present" \
  env GO_BIN="$T83_BIN" PY_ROOT="$T83/pypi" bash "$T83/scripts/check-agent-hooks-parity.sh"

# Cenário 84 — check-artifact-parity.sh: seção ## Architect responses ausente no Node.js detectada
# (ML-1A, ROADMAP-2026-08-21-regra-de-verbosidade-no-asset-do-arquiteto-e-nas-regras-semeadas)
#
# Objetivo (P4): provar que check-artifact-parity.sh REPROVA quando o gerador Node.js do
# CLAUDE.md remove a seção ## Architect responses enquanto Go e Python permanecem corretos.
#
# Estratégia: corromper npm/src/generators/init.js trocando o header da seção pelo nome
# ## VERBOSITY_SECTION_REMOVED. O awk de extração não encontra ## Architect responses no
# CLAUDE.md gerado pelo Node.js → vacuity guard dispara a mensagem de erro esperada.
# Go (bin/trackfw real via GO_BIN) e Python (pypi/ copiado) ficam íntegros.
# Baseline prova que o ciclo limpo passa antes da detecção.
# ---------------------------------------------------------------------------
T84="$WORK/s84"
mkdir -p "$T84/scripts"

setup_npm_tree "$T84"
corrupt_literal \
  "$ROOT_DIR/npm/src/generators/init.js" "$T84/npm/src/generators/init.js" \
  "content += '\\n## Architect responses\\n\\n'" \
  "content += '\\n## VERBOSITY_SECTION_REMOVED\\n\\n' // [falsified]" \
  's84-node-architect-responses-header'

mkdir -p "$T84/pypi"
cp -r "$ROOT_DIR/pypi/." "$T84/pypi/"

cp "$ROOT_DIR/scripts/check-artifact-parity.sh" "$T84/scripts/"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-artifact-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s84-baseline]: check-artifact-parity.sh já reprova com os runtimes reais — prova P4 inválida' >&2
  exit 1
fi
echo 'OK   [falsify/artifact-parity/claude-md-architect-responses-vacuity-baseline]'

assert_fails_with 'artifact-parity/claude-md-architect-responses-section-node' \
  'CLAUDE.md ## Architect responses missing or empty (node)' \
  env GO_BIN="$ROOT_DIR/bin/trackfw" bash "$T84/scripts/check-artifact-parity.sh"

# Cenário 85 — nil map em ProjectConfig.AgentModels: parse() sem initConfigMaps()
#              causa panic com "assignment to entry in nil map" quando
#              ParseRulesFromContent é chamado com conteúdo contendo agent_models:.
#
# Objetivo (P4, ML-2C ROADMAP-2026-08-21-versao-do-modelo-por-tier-com-
# composicao-por-alvo): provar que a chamada initConfigMaps(cfg) no início de
# parse() é indispensável. Sem ela, ParseRulesFromContent cria um
# ProjectConfig{Rules: make(...)} com AgentModels nil; parse() então escreve
# cfg.AgentModels[k] = s → panic "assignment to entry in nil map".
#
# Estratégia: corromper config.go removendo a chamada initConfigMaps(cfg), e
# executar o próprio TestParseRulesFromContentWithAgentModels_NoPanic na cópia
# corrompida via `go test`. Ao contrário de execução via CLI + git HEAD
# (que depende de git subprocess), o `go test` exerce a função diretamente —
# sem dependência de ambiente.
#
# Seam: internal/config/config.go — remover a chamada initConfigMaps(cfg)
# na primeira linha de parse(). A função initConfigMaps permanece (sem erro
# "declared and not used" para reflect), mas não é chamada — suficiente para
# restaurar o nil map.
# ---------------------------------------------------------------------------
T85="$WORK/s85"
mkdir -p "$T85/cmd" "$T85/internal"
cp -r "$ROOT_DIR/cmd/." "$T85/cmd/"
cp -r "$ROOT_DIR/internal/." "$T85/internal/"
cp "$ROOT_DIR/go.mod" "$T85/go.mod"
cp "$ROOT_DIR/go.sum" "$T85/go.sum"

sed 's/\tinitConfigMaps(cfg) \/\/ guarantee: all map fields are non-nil before any write/\t\/\/ [falsified] initConfigMaps(cfg) removed — nil map panic restored/' \
  "$ROOT_DIR/internal/config/config.go" > "$T85/internal/config/config.go"

if cmp -s "$ROOT_DIR/internal/config/config.go" "$T85/internal/config/config.go"; then
  echo "FAIL [falsify/setup-s85]: sed não alterou config.go — padrão não encontrado; prova P4 inválida" >&2
  exit 1
fi

# Liveness check: o módulo corrompido ainda compila (initConfigMaps existe mas
# não é chamada — reflect continua usado).
T85_BIN="$WORK/s85-bin/trackfw"
mkdir -p "$(dirname "$T85_BIN")"
build_go_or_fail "setup-s85-liveness-build" "$T85" "$T85_BIN"

# Braço de baseline: go test passes no código real
if ! (cd "$ROOT_DIR" && env GOCACHE="$WORK/go-build-cache" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/config/ -run TestParseRulesFromContentWithAgentModels_NoPanic) >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s85-baseline]: go test falhou no código real — prova P4 inválida" >&2
  exit 1
fi
echo "OK   [falsify/nil-map-init/parse-with-agent-models-nopanic-baseline]"

# Braço de detecção: go test panica na cópia corrompida
assert_fails_with "nil-map-init/parse-missing-causes-panic-on-agent-models" \
  "assignment to entry in nil map" \
  bash -c "cd \"$T85\" && env GOCACHE=\"$WORK/go-build-cache\" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/config/ -run TestParseRulesFromContentWithAgentModels_NoPanic"

# Cenário 86 — namespace leak via remoção da guarda de allowlist em render.go:
#              a condição `targetID == "claude" && len(agentModels) > 0` protege
#              todos os targets não-Claude de receberem model IDs compostos quando
#              agent_models está configurado.  Sem ela, targets como Gemini (que
#              usam o case `default:` do switch de representação) recebem o valor
#              composto (ex.: "model: claude-sonnet-4-6") em vez do alias canônico
#              (ex.: "model: sonnet"), quebrando o agente em produção.
#
# Objetivo (P4, ML-3A ROADMAP-2026-08-21-versao-do-modelo-por-tier-com-
# composicao-por-alvo): provar que a guarda `targetID == "claude" &&` em
# internal/integrations/render.go é o load-bearing literal que impede o
# vazamento de namespace.  A fronteira de namespace é um GATE (ADR-2026-08-21
# §4), não um cuidado — este cenário prova que o gate a detecta.
#
# Alvo de sabotagem escolhido: não-vazamento (Gemini), não composição.
# Um P4 que só provasse que a composição deixou de funcionar deixaria o
# vazamento sem falsificação — e vazamento é o defeito caro: o usuário só
# descobre quando o agente não sobe, com a causa a duas camadas de distância.
#
# Por que Gemini e não Codex:
#   - Codex usa case "custom-agent-toml" (retorna cedo do switch de
#     representação) — a guarda no case `default:` não o alcança de forma
#     alguma.  Sabotá-la não causaria leak no Codex, tornando o P4 inválido.
#   - Gemini usa case `default:` (representation "agent-markdown") sem
#     proteção específica por targetID — é exatamente o alvo que a guarda
#     protege e que vazaria se ela fosse removida.
#
# Estratégia: copiar a árvore Go para $T86, remover `targetID == "claude" && `
# da condição em render.go com sed, compilar um binário isolado e executar
# check-agent-models-parity.sh apontando GO_BIN para ele.  O braco de detecção
# espera a mensagem "namespace leak" no stderr/stdout.  Verificação de
# vivacidade: confirmar que o arquivo foi de fato modificado (sed não foi
# no-op) e que o binário corrompido ainda compila (a guarda é um predicado,
# não uma declaração — sem ela o código ainda é Go válido).
#
# Seam: internal/integrations/render.go, condição no case `default:`:
#   } else if targetID == "claude" && len(agentModels) > 0 {
# →  } else if len(agentModels) > 0 {
# ---------------------------------------------------------------------------
T86="$WORK/s86"
mkdir -p "$T86/cmd" "$T86/internal" "$T86/bin" "$T86/scripts"
cp -r "$ROOT_DIR/cmd/." "$T86/cmd/"
cp -r "$ROOT_DIR/internal/." "$T86/internal/"
cp "$ROOT_DIR/go.mod" "$T86/go.mod"
cp "$ROOT_DIR/go.sum" "$T86/go.sum"
cp "$ROOT_DIR/scripts/check-agent-models-parity.sh" "$T86/scripts/"

# Aplicar patch: remover a guarda de allowlist claude
sed 's/} else if targetID == "claude" \&\& len(agentModels) > 0 {/} else if len(agentModels) > 0 {/' \
  "$ROOT_DIR/internal/integrations/render.go" > "$T86/internal/integrations/render.go"

# Verificação de vivacidade: confirmar que o patch foi aplicado
if cmp -s "$ROOT_DIR/internal/integrations/render.go" "$T86/internal/integrations/render.go"; then
  echo 'FAIL [falsify/setup-s86-liveness]: sed nao modificou render.go — seam pode ter mudado' >&2
  exit 1
fi
echo 'OK   [falsify/agent-models-parity/namespace-guard-liveness]'

# Compilar binário corrompido — a guarda é um predicado puro; sem ela o código
# continua válido Go e compila normalmente.
if ! (cd "$T86" && env GOCACHE="$WORK/go-build-cache" go build -o "$T86/bin/trackfw" ./cmd/trackfw) >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s86-build]: binário corrompido nao compilou — verificar o patch' >&2
  exit 1
fi
echo 'OK   [falsify/agent-models-parity/namespace-guard-sabotaged-build]'

# Braco de baseline: gate deve PASSAR com o binário real
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s86-baseline]: check-agent-models-parity.sh ja reprova com binario real — prova P4 invalida' >&2
  exit 1
fi
echo 'OK   [falsify/agent-models-parity/namespace-guard-baseline]'

# Braco de detecção: gate deve FALHAR com o binário corrompido,
# especificamente relatando "namespace leak" no output.
#
# Nota: usamos $ROOT_DIR/scripts/check-agent-models-parity.sh (não a cópia
# em $T86/scripts/).  A cópia T86 existe para que o T86 seja uma árvore Go
# válida (go build usa o ROOT_DIR dos arquivos Go do check-agent-models-
# parity.sh para descobrir o pacote — se o script não estiver em T86 a
# compilação do binário corrompido não depende dele).  Mas ao EXECUTAR o
# gate, ROOT_DIR deve resolver para o projeto real, pois NODE_CLI e PY_ROOT
# são derivados de ROOT_DIR no script e npm/pypi não existem em $T86.
# Resultado: GO_BIN aponta para o binário corrompido (que vaza namespace);
# Node.js e Python usam os CLIs reais e ficam limpos — como esperado, dado
# que o leak é exclusivo da guarda removida do Go.  set -euo pipefail faria
# o script morrer antes de atingir a mensagem "namespace leak" se o Node
# tentasse chamar um binário inexistente em $T86/npm/.
assert_fails_with "agent-models-parity/namespace-guard-removed-causes-gemini-leak" \
  "namespace leak" \
  env GO_BIN="$T86/bin/trackfw" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh"

echo '# ---------------------------------------------------------------------------'
# Cenário 87 — check-release-tag-parity.sh Scenario 16 (content-from-commit-
#              provenance): CHANGELOG read bypassed from forge commit back to
#              local HEAD — false-negative proof.
#
# Alvo de sabotagem:
#   deps.readCommittedFile(objectSHA, "CHANGELOG.md")
#   → deps.readCommittedFile("HEAD", "CHANGELOG.md")
#   in internal/commands/release.go.
#
# Por que este literal:
#   - readFile foi REMOVIDO do struct releaseDeps (ML-2A); substituir por
#     deps.readFile() não compila. O único fallback possível é passar um
#     argumento sha diferente a readCommittedFile.
#   - "HEAD" é sintaticamente válido (git show HEAD:<path> resolve para o tip
#     local) e compila sem alterações adicionais.
#   - P3 (leitura dos version files) ainda usa objectSHA → ainda passa (9.9.9).
#   - P4 (leitura do CHANGELOG) agora lê de HEAD → ## [9.9.9] encontrado em
#     HEAD também → P4 ainda passa → exit 0.
#   - MAS message = conteúdo de HEAD's CHANGELOG ("head-only"), não do forge
#     commit ("forge-only") → asserção de proveniência do Scenario 16 dispara.
#   - O literal aparece EXATAMENTE UMA VEZ em release.go (verificado por grep):
#     corrupt_literal reprova se count ≠ 1.
#
# Não-vacuidade: braço de baseline prova que o gate passa com o binário real
# antes de qualquer sabotagem. Braço de detecção prova que o binário corrompido
# faz o gate falhar com a mensagem exata da asserção de proveniência.
#
# Seam (único em release.go):
#   changelogContent, err := deps.readCommittedFile(objectSHA, "CHANGELOG.md")
# →  changelogContent, err := deps.readCommittedFile("HEAD", "CHANGELOG.md")
# ---------------------------------------------------------------------------
T87="$WORK/s87"
T87C_GO_MOD="$T87/corrupt-go"
T87C_GO_BIN="$T87/corrupt-go-bin/trackfw"
mkdir -p "$T87C_GO_MOD/cmd" "$T87C_GO_MOD/internal" "$T87/corrupt-go-bin"
cp -r "$ROOT_DIR/cmd/." "$T87C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T87C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T87C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T87C_GO_MOD/go.sum"

# Baseline: gate deve PASSAR com o binário real
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s87-baseline]: check-release-tag-parity.sh ja reprova com binario real — prova P4 invalida' >&2
  exit 1
fi
echo 'OK   [falsify/release-tag-parity/content-from-commit-baseline]'

# Aplicar sabotagem: substituir objectSHA por "HEAD" na leitura do CHANGELOG
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" \
  "$T87C_GO_MOD/internal/commands/release.go" \
  'deps.readCommittedFile(objectSHA, "CHANGELOG.md")' \
  'deps.readCommittedFile("HEAD", "CHANGELOG.md")' \
  'setup-s87-changelog-read-corrupt'

build_go_or_fail "setup-s87-go-corrupt-build" "$T87C_GO_MOD" "$T87C_GO_BIN"

# Detecção: gate deve FALHAR com a mensagem exata da asserção de proveniência
assert_fails_with "release-tag-parity/content-from-commit-false-negative" \
  "provenance: tag message must contain 'forge-only'" \
  env GO_BIN="$T87C_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

echo '# ---------------------------------------------------------------------------'
# Cenário 158 — check-release-tag-parity.sh Scenario 17 (refs-replace-bypass):
#               --no-replace-objects flag removed from git show call in
#               defaultReleaseReadCommittedFile — false-negative proof (P4).
#
# Alvo de sabotagem:
#   exec.Command("git", "--no-replace-objects", "show", sha+":"+path)
#   → exec.Command("git", "show", sha+":"+path)
#   (removes the flag that blocks refs/replace/ object-identity substitution)
#
# Mecanismo do falso-negativo:
#   - Sem --no-replace-objects, git show $FORGE_SHA:CHANGELOG.md segue a
#     refs/replace/ ref e lê o commit do atacante ("refs-replace-forged").
#   - P3 (version files) ainda lê de objectSHA → 9.9.9 ✓ → exit 0.
#   - P4 (CHANGELOG) também retorna ## [9.9.9] section (do commit atacante,
#     que herda a estrutura do forge commit via s17-attacker branch) → P4 ✓.
#   - Mas a mensagem da tag = "refs-replace-forged" (conteúdo do atacante),
#     não "forge-only" (conteúdo do forge commit) → asserção de proveniência
#     do Scenario 17 dispara: 'provenance: tag message must contain forge-only'.
#   - A asserção por runtime é o que torna o scenario resistente a revert
#     correlacionado: todos os 3 stacks revertem → cada runtime dispara
#     individualmente; assert_three_way pega o revert de stack único.
#
# Seam (único em release.go — verificado por grep):
#   "--no-replace-objects", "show"  →  "show"
# ---------------------------------------------------------------------------
T88="$WORK/s158"
T88C_GO_MOD="$T88/corrupt-go"
T88C_GO_BIN="$T88/corrupt-go-bin/trackfw"
mkdir -p "$T88C_GO_MOD/cmd" "$T88C_GO_MOD/internal" "$T88/corrupt-go-bin"
cp -r "$ROOT_DIR/cmd/." "$T88C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T88C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T88C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T88C_GO_MOD/go.sum"

# Baseline: gate deve PASSAR com o binário real (independente do braço S87)
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s158-baseline]: check-release-tag-parity.sh ja reprova com binario real — prova P4 invalida' >&2
  exit 1
fi
echo 'OK   [falsify/release-tag-parity/refs-replace-bypass-baseline]'

# Aplicar sabotagem: remover --no-replace-objects da chamada git show
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" \
  "$T88C_GO_MOD/internal/commands/release.go" \
  '"--no-replace-objects", "show"' \
  '"show"' \
  'setup-s158-no-replace-objects-removed'

build_go_or_fail "setup-s158-go-corrupt-build" "$T88C_GO_MOD" "$T88C_GO_BIN"

# Detecção: gate deve FALHAR com a mensagem exata da asserção de proveniência
# do Scenario 17. O binário corrompido segue refs/replace/ e lê o CHANGELOG
# do commit do atacante ("refs-replace-forged") em vez do forge commit
# ("forge-only") — Scenario 17's provenance assertion fires.
assert_fails_with "release-tag-parity/refs-replace-bypass-false-negative" \
  "provenance: tag message must contain 'forge-only'" \
  env GO_BIN="$T88C_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 159 — check-validate-parity.sh: `credential_guard_hook_resolvable`
#               deixa de detectar a forma relativa pura de Claude (cláusula
#               bare-relative da classe 2 desativada por `false &&`) →
#               o caso cg-claude-relativo reprova.
#
# Objetivo (ROADMAP-2026-08-21-validate-detecta-hook-de-guard-na-forma-
# relativa-antiga, ML-2A — P4 direção-A): prova que o caso cg-claude-relativo
# de check-validate-parity.sh seria vácuo sem este cenário — ele poderia estar
# verde por acidente sem jamais provar que reprovaria se a detecção da forma
# relativa pura quebrasse.
#
# Seam: internal/validator/validator_credential_guard.go —
# RETARGETED 2026-08-22 ML-2A (ADR-2026-08-22): isRelativePureForGuard foi
# removida e substituída por classifyHookAnchorage. O seam agora é a cláusula
# bare-relative dentro da classe 2:
#   `(!strings.HasPrefix(rawStripped, "$") && !filepath.IsAbs(rawStripped))`
# Delta de literal único: `!strings.HasPrefix(rawStripped, "$")` → `false`,
# tornando a cláusula `false && !filepath.IsAbs(rawStripped)` morta.
# Efeito: caminho relativo puro (scripts/…) cai na classe 3 → silêncio em Go;
# $PWD/ ainda atinge a cláusula de prefixo anterior e permanece acusado.
# Node.js/Python ficam reais → P2 vacuity guard: Go não reporta violação
# em cg-claude-relativo mas deveria.
#
# GO_BIN override (mesma convenção dos Cenários 80-82) aponta
# check-validate-parity.sh para o binário sabotado.
# ---------------------------------------------------------------------------
T89="$WORK/s159"
mkdir -p "$T89/cmd" "$T89/internal"
cp -r "$ROOT_DIR/cmd/." "$T89/cmd/"
cp -r "$ROOT_DIR/internal/." "$T89/internal/"
cp "$ROOT_DIR/go.mod" "$T89/go.mod"
cp "$ROOT_DIR/go.sum" "$T89/go.sum"

sed 's/!strings\.HasPrefix(rawStripped, "\$")/false/g' \
  "$ROOT_DIR/internal/validator/validator_credential_guard.go" > "$T89/internal/validator/validator_credential_guard.go"

if cmp -s "$ROOT_DIR/internal/validator/validator_credential_guard.go" "$T89/internal/validator/validator_credential_guard.go"; then
  echo "FAIL [falsify/setup-s159]: sed nao alterou validator_credential_guard.go — padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T89_BIN="$WORK/s159-bin/trackfw"
mkdir -p "$(dirname "$T89_BIN")"
build_go_or_fail "setup-s159-liveness-build" "$T89" "$T89_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s159-baseline]: check-validate-parity.sh ja reprova com o binario real — prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/credential-guard-bare-relative-suppression-baseline]"

assert_fails_with "validate-parity/credential-guard-bare-relative-not-detected" \
  "claude-relativo/go): expected violation" \
  env GO_BIN="$T89_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 160 — check-validate-parity.sh: `credential_guard_hook_resolvable`
#               acusa Copilot falso-positivamente (requiresVarOrShellPrefix
#               flipado para Copilot de false→true) → o caso
#               cg-copilot-relativo-present reprova.
#
# Objetivo (ROADMAP-2026-08-21-validate-detecta-hook-de-guard-na-forma-
# relativa-antiga, ML-2A — P4 direção-B): prova a SEGUNDA direção de falha:
# o discriminante de falso-positivo (requiresVarOrShellPrefix=false para
# Copilot) elimina a acusação errada de Copilot. Sem este cenário, uma
# regressão que flipe esse flag para true passaria despercebida — o caso
# cg-copilot-relativo-present ficaria permanentemente silencioso por razão
# errada (não testável sem corromper o flag).
#
# Seam: internal/validator/validator_credential_guard.go linha
# `{".github/hooks/trackfw-attention.json", "GitHub Copilot CLI", true, false},`
# Delta de literal único: o último `false` vira `true` — Copilot passa a ser
# tratado como Claude/Codex/Gemini, acusando caminho relativo puro como forma
# antiga/errada. Go reporta violação em cg-copilot-relativo-present (expect=False)
# → Python analysis reprova com "nenhuma violacao da regra esperada".
# Node.js/Python ficam reais e corretos.
# ---------------------------------------------------------------------------
T90="$WORK/s160"
mkdir -p "$T90/cmd" "$T90/internal"
cp -r "$ROOT_DIR/cmd/." "$T90/cmd/"
cp -r "$ROOT_DIR/internal/." "$T90/internal/"
cp "$ROOT_DIR/go.mod" "$T90/go.mod"
cp "$ROOT_DIR/go.sum" "$T90/go.sum"

sed 's/"GitHub Copilot CLI", true, false},/"GitHub Copilot CLI", true, true}, \/\/ [falsified]/' \
  "$ROOT_DIR/internal/validator/validator_credential_guard.go" > "$T90/internal/validator/validator_credential_guard.go"

if cmp -s "$ROOT_DIR/internal/validator/validator_credential_guard.go" "$T90/internal/validator/validator_credential_guard.go"; then
  echo "FAIL [falsify/setup-s160]: sed nao alterou validator_credential_guard.go — padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T90_BIN="$WORK/s160-bin/trackfw"
mkdir -p "$(dirname "$T90_BIN")"
build_go_or_fail "setup-s160-liveness-build" "$T90" "$T90_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s160-baseline]: check-validate-parity.sh ja reprova com o binario real — prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/credential-guard-copilot-false-positive-baseline]"

assert_fails_with "validate-parity/credential-guard-copilot-false-positive-detected" \
  "copilot-relativo-present" \
  env GO_BIN="$T90_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 161 — check-push-parity.sh: governance gate removida para feat/
#               (isGatedShipBranch sempre retorna false em push.go) → feat/
#               sem roadmap sai 0 em Go, mas Node.js/Python saem 1.
#
# Objetivo (ROADMAP-2026-08-22-trackfw-push-comando-proprio-para-empurrar-
# commits-ja-criados, ML-2A — P4 direção-A): prova a PRIMEIRA direção de
# falha: a guarda de governança em push.go (Step 2) é o discriminante real.
# Sem este cenário, uma regressão que neutralize isGatedShipBranch passaria
# despercebida — feat/ sem roadmap não bloquearia mais.
#
# Seam: internal/commands/push.go linha
# `	if !isGatedShipBranch(branch) {`
# Delta de literal único: `!isGatedShipBranch(branch)` → `true || !isGatedShipBranch(branch)`.
# O branch morto faz feat/ sempre cair no caminho de chore/docs (skip).
# Go reporta "Governance: skipped (chore/docs branch)" para feat/ e sai 0,
# Node.js/Python reportam "Governance check failed" e saem 1.
# check-push-parity.sh reprova em push-parity/feat-governance-blocked/exit-code.
# ---------------------------------------------------------------------------
T91="$WORK/s161"
mkdir -p "$T91/cmd" "$T91/internal"
cp -r "$ROOT_DIR/cmd/." "$T91/cmd/"
cp -r "$ROOT_DIR/internal/." "$T91/internal/"
cp "$ROOT_DIR/go.mod" "$T91/go.mod"
cp "$ROOT_DIR/go.sum" "$T91/go.sum"

sed 's/if !isGatedShipBranch(branch) {/if true || !isGatedShipBranch(branch) {/' \
  "$ROOT_DIR/internal/commands/push.go" > "$T91/internal/commands/push.go"

if cmp -s "$ROOT_DIR/internal/commands/push.go" "$T91/internal/commands/push.go"; then
  echo "FAIL [falsify/setup-s161]: sed nao alterou push.go — padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T91_BIN="$WORK/s161-bin/trackfw"
mkdir -p "$(dirname "$T91_BIN")"
build_go_or_fail "setup-s161-liveness-build" "$T91" "$T91_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-push-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s161-baseline]: check-push-parity.sh ja reprova com o binario real — prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/push-parity/governance-gate-removed-baseline]"

assert_fails_with "push-parity/feat-governance-blocked/exit-code" \
  "exit codes diverge" \
  env GO_BIN="$T91_BIN" bash "$ROOT_DIR/scripts/check-push-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 162 — check-push-parity.sh: push emite texto "Opening pull request"
#               após "Governance: OK" em Go (format string adulterado) →
#               guard de vacuidade de Direção B dispara e assert_three_way
#               detecta divergência go-vs-node.
#
# Objetivo (ROADMAP-2026-08-22-trackfw-push-comando-proprio-para-empurrar-
# commits-ja-criados, ML-2A — P4 direção-B): prova a SEGUNDA direção de
# falha: o discriminante "push nunca abre PR/MR" é testável e detectável.
# Sem este cenário, uma regressão que adicionasse abertura de PR ao push
# passaria despercebida se aplicada uniformemente nos 3 CLIs (assert_three_way
# ficaria silencioso); o guard de vacuidade no cenário (a) fecha essa brecha
# para regressões Go-only.
#
# Seam: internal/commands/push.go linha
# `		fmt.Fprintf(deps.out, "Governance: OK\n")`
# Delta de literal único: `"Governance: OK\n"` →
# `"Governance: OK\nOpening pull request for branch...\n"`.
# Go passa a emitir "Opening pull request for branch..." após Governance: OK;
# check-push-parity.sh cenário (a) reprova no guard de vacuidade "push must
# not open a PR" antes mesmo de assert_three_way (que também divergiria).
#
# Nota de paridade parcial: se a regressão for aplicada de forma idêntica nos
# 3 CLIs, assert_three_way não detecta (os 3 concordam com o valor errado).
# O guard de vacuidade fecha apenas a vertente Go-only. Uma corrupção
# tri-stack coordenada é considerada fora do modelo de ameaça deste gate
# (exigiria comprometimento simultâneo das 3 codebases).
# ---------------------------------------------------------------------------
T92="$WORK/s162"
mkdir -p "$T92/cmd" "$T92/internal"
cp -r "$ROOT_DIR/cmd/." "$T92/cmd/"
cp -r "$ROOT_DIR/internal/." "$T92/internal/"
cp "$ROOT_DIR/go.mod" "$T92/go.mod"
cp "$ROOT_DIR/go.sum" "$T92/go.sum"

sed 's/"Governance: OK\\n"/"Governance: OK\\nOpening pull request for branch...\\n"/' \
  "$ROOT_DIR/internal/commands/push.go" > "$T92/internal/commands/push.go"

if cmp -s "$ROOT_DIR/internal/commands/push.go" "$T92/internal/commands/push.go"; then
  echo "FAIL [falsify/setup-s162]: sed nao alterou push.go — padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T92_BIN="$WORK/s162-bin/trackfw"
mkdir -p "$(dirname "$T92_BIN")"
build_go_or_fail "setup-s162-liveness-build" "$T92" "$T92_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-push-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s162-baseline]: check-push-parity.sh ja reprova com o binario real — prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/push-parity/pr-text-emitted-baseline]"

assert_fails_with "push-parity/feat-governance-ok-no-upstream/go" \
  "push must not open a PR" \
  env GO_BIN="$T92_BIN" bash "$ROOT_DIR/scripts/check-push-parity.sh"


# ---------------------------------------------------------------------------
# Cenário 163 — sabotagem do gate force-with-lease de push: open=true força PR check sempre passar
#
# Delta: uma linha em internal/commands/push.go, linha 202:
#   -		if !open {
#   (insere open = true antes, forçando open sempre true)
# Efeito: a verificação de PR aberto deixa de recusar, o push --force-with-lease
# acontece mesmo sem PR aberto (violação da propriedade de segurança P4-push).
# Detecção: check-push-force-parity.sh cenário (b) (forge-zero-pr) reprova porque
# o stub retorna [] mas o binário sabotado não recusa mais.
# ---------------------------------------------------------------------------
T93="$WORK/s163"
mkdir -p "$T93/cmd" "$T93/internal"
cp -r "$ROOT_DIR/cmd/." "$T93/cmd/"
cp -r "$ROOT_DIR/internal/." "$T93/internal/"
cp "$ROOT_DIR/go.mod" "$T93/go.mod"
cp "$ROOT_DIR/go.sum" "$T93/go.sum"

sed 's/\t\tif !open {/\t\topen = true \/\/ sabotaged: PR check bypassed\n\t\tif !open {/' \
  "$ROOT_DIR/internal/commands/push.go" > "$T93/internal/commands/push.go"

if cmp -s "$ROOT_DIR/internal/commands/push.go" "$T93/internal/commands/push.go"; then
  echo "FAIL [falsify/setup-s163]: sed nao alterou push.go — padrao nao encontrado; prova P4-push invalida" >&2
  exit 1
fi

T93_BIN="$WORK/s163-bin/trackfw"
mkdir -p "$(dirname "$T93_BIN")"
build_go_or_fail "setup-s163-liveness-build" "$T93" "$T93_BIN"

# Baseline arm: check-push-force-parity.sh deve passar com o binario real.
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-push-force-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s163-baseline]: check-push-force-parity.sh ja reprova com o binario real — prova P4-push invalida" >&2
  exit 1
fi
echo "OK   [falsify/push-force-parity/pr-open-gate-baseline]"

# Detection arm: check-push-force-parity.sh deve reprovar com o binario sabotado.
assert_fails_with "push-force-parity/pr-open-gate-removed/go" \
  "forge-zero-pr" \
  env GO_BIN="$T93_BIN" bash "$ROOT_DIR/scripts/check-push-force-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 164 — check-validate-parity.sh: `credential_guard_hook_resolvable`
#               deixa de detectar `$PWD/…` (acusação suprimida pela renomeia
#               do prefixo de `$PWD/` para `$PWD_DEAD/` no classificador) →
#               o caso cg-claude-pwd reprova.
#
# Objetivo (ROADMAP-2026-08-22-validate-detecta-hook-com-pwd-que-falha-fora-
# da-raiz, ML-2A — P4 direção-A): prova que o caso cg-claude-pwd de
# check-validate-parity.sh seria vácuo sem este cenário — ele poderia estar
# verde por acidente sem jamais provar que reprovaria se a detecção de $PWD
# quebrasse.
#
# Seam: internal/validator/validator_credential_guard.go linha
# `if strings.HasPrefix(rawStripped, "$PWD/") ||`
# Delta de literal único: prefixo `"$PWD/"` → `"$PWD_DEAD/"`.
# Efeito: `$PWD/…` não casa mais com a cláusula de prefixo → cai na cláusula
# bare-relative? Não: começa com `$` → última cláusula (`!$`) é false → classe
# 3 → silêncio em Go. Node.js/Python ficam reais → P2 vacuity guard: Go não
# reporta violação em cg-claude-pwd mas deveria.
#
# GO_BIN override aponta check-validate-parity.sh para o binário sabotado.
# ---------------------------------------------------------------------------
T94="$WORK/s164"
mkdir -p "$T94/cmd" "$T94/internal"
cp -r "$ROOT_DIR/cmd/." "$T94/cmd/"
cp -r "$ROOT_DIR/internal/." "$T94/internal/"
cp "$ROOT_DIR/go.mod" "$T94/go.mod"
cp "$ROOT_DIR/go.sum" "$T94/go.sum"

sed 's/strings\.HasPrefix(rawStripped, "\$PWD\/")/strings.HasPrefix(rawStripped, "\$PWD_DEAD\/")/g' \
  "$ROOT_DIR/internal/validator/validator_credential_guard.go" > "$T94/internal/validator/validator_credential_guard.go"

if cmp -s "$ROOT_DIR/internal/validator/validator_credential_guard.go" "$T94/internal/validator/validator_credential_guard.go"; then
  echo "FAIL [falsify/setup-s164]: sed nao alterou validator_credential_guard.go — padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T94_BIN="$WORK/s164-bin/trackfw"
mkdir -p "$(dirname "$T94_BIN")"
build_go_or_fail "setup-s164-liveness-build" "$T94" "$T94_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s164-baseline]: check-validate-parity.sh ja reprova com o binario real — prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/credential-guard-pwd-suppression-baseline]"

assert_fails_with "validate-parity/credential-guard-pwd-not-detected" \
  "claude-pwd/go): expected violation" \
  env GO_BIN="$T94_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 165 — check-validate-parity.sh: `credential_guard_hook_resolvable`
#               passa a acusar caminho absoluto (falso-positivo introduzido pela
#               substituição de `filepath.IsAbs(rawStripped)` → `false` no
#               classificador) → o caso cg-claude-absoluto reprova.
#
# Objetivo (ROADMAP-2026-08-22-validate-detecta-hook-com-pwd-que-falha-fora-
# da-raiz, ML-2A — P4 direção-B): prova a SEGUNDA direção de falha — o caso
# cg-claude-absoluto de check-validate-parity.sh seria vácuo sem este cenário.
# A direção B é a que protege o defeito caro desta entrega: acusar caminho
# absoluto é o falso-positivo que reprova a entrega.
#
# Seam: internal/validator/validator_credential_guard.go
# `filepath.IsAbs(rawStripped)` — aparece em DOIS lugares (classe 1 e classe 2).
# Delta: ambas as ocorrências substituídas por `false` com /g.
# Efeito: linha 105 (classe 1) não captura mais absolutos → linha 112
# (`!false` = true) → absoluto cai na cláusula bare-relative → classe 2 →
# acusado. Go reporta violação em cg-claude-absoluto (expect=False) →
# Python reprova com "nenhuma violacao da regra esperada".
# Nota: linha 175 usa `raw` (não `rawStripped`) → resolveCredentialGuardHookPath
# não é afetada. Compila porque `false` é expressão bool válida e as variáveis
# declaradas continuam referenciadas.
#
# GO_BIN override aponta check-validate-parity.sh para o binário sabotado.
# ---------------------------------------------------------------------------
T95="$WORK/s165"
mkdir -p "$T95/cmd" "$T95/internal"
cp -r "$ROOT_DIR/cmd/." "$T95/cmd/"
cp -r "$ROOT_DIR/internal/." "$T95/internal/"
cp "$ROOT_DIR/go.mod" "$T95/go.mod"
cp "$ROOT_DIR/go.sum" "$T95/go.sum"

sed 's/filepath\.IsAbs(rawStripped)/false/g' \
  "$ROOT_DIR/internal/validator/validator_credential_guard.go" > "$T95/internal/validator/validator_credential_guard.go"

if cmp -s "$ROOT_DIR/internal/validator/validator_credential_guard.go" "$T95/internal/validator/validator_credential_guard.go"; then
  echo "FAIL [falsify/setup-s165]: sed nao alterou validator_credential_guard.go — padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T95_BIN="$WORK/s165-bin/trackfw"
mkdir -p "$(dirname "$T95_BIN")"
build_go_or_fail "setup-s165-liveness-build" "$T95" "$T95_BIN"

if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-validate-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s165-baseline]: check-validate-parity.sh ja reprova com o binario real — prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/validate-parity/credential-guard-absolute-false-positive-baseline]"

assert_fails_with "validate-parity/credential-guard-absolute-path-accused" \
  "claude-absoluto/go): nenhuma violação" \
  env GO_BIN="$T95_BIN" bash "$ROOT_DIR/scripts/check-validate-parity.sh"


# ---------------------------------------------------------------------------
# Cenario 166 -- Direcao A (AC9/AC14, ROADMAP-2026-08-22-wave-0-de-modelo-de-
#                ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push,
#                ML-2A): gerador de roadmap deixa de emitir "## Wave 0 --
#                Threat Model" nos 3 stacks SINCRONIZADAMENTE (mesmo texto
#                trocado, mesma hora, nos 3 geradores) -- prova que a
#                assercao de conteudo esperado acrescentada a
#                check-artifact-parity.sh (AC14) e load-bearing. Sem ela,
#                as 3 saidas identicas-mas-erradas passariam limpas no diff
#                cross-stack existente (achado do modelo de ameaca, docs/
#                seguranca/2026-08-22-modelo-de-ameaca-da-wave-0-no-harness.md,
#                Sec3 F1/F4: "uma regressao sincronizada que remove Wave 0
#                dos 3 stacks passa em silencio").
#
# GO_BIN sozinho nao basta: check-artifact-parity.sh hardcoda
# node "$ROOT_DIR/npm/bin/trackfw" e PYTHONPATH="$ROOT_DIR/pypi" -- ROOT_DIR
# deriva do proprio BASH_SOURCE do script. A unica forma de sincronizar a
# sabotagem nos 3 stacks e copiar a arvore inteira (cmd/, internal/, npm/,
# pypi/, go.mod, go.sum) e invocar a COPIA do gate, nao o do ROOT_DIR real
# -- mesmo com GO_BIN apontando para o real, o node/python continuariam
# limpos e a asimetria denunciaria a prova (2 stacks limpos, 1 sabotado
# passaria pelo diff cross-stack antigo mesmo sem a assercao nova).
# ---------------------------------------------------------------------------
setup_s166_tree() {
  local dest=$1
  mkdir -p "$dest/cmd" "$dest/internal" "$dest/scripts" \
           "$dest/npm/bin" "$dest/npm/src" "$dest/pypi"
  cp -r "$ROOT_DIR/cmd/." "$dest/cmd/"
  cp -r "$ROOT_DIR/internal/." "$dest/internal/"
  cp "$ROOT_DIR/go.mod" "$dest/go.mod"
  cp "$ROOT_DIR/go.sum" "$dest/go.sum"
  cp "$ROOT_DIR/scripts/check-artifact-parity.sh" "$dest/scripts/check-artifact-parity.sh"
  cp "$ROOT_DIR/npm/bin/trackfw" "$dest/npm/bin/trackfw"
  cp -r "$ROOT_DIR/npm/src/." "$dest/npm/src/"
  ln -s "$ROOT_DIR/npm/node_modules" "$dest/npm/node_modules"
  cp "$ROOT_DIR/npm/package.json" "$dest/npm/package.json"
  cp -r "$ROOT_DIR/pypi/trackfw" "$dest/pypi/trackfw"
}

# Baseline -- copia LIMPA (sem sabotagem): o gate contra a copia tem que
# passar tao limpo quanto passa contra o ROOT_DIR real. Sem isto, a copia
# em si poderia estar quebrada por um motivo alheio a sabotagem, e a prova
# P4 seria invalida (mesmo padrao dos Cenarios 164/165).
T96_BASE="$WORK/s166-baseline"
setup_s166_tree "$T96_BASE"
T96_BASE_BIN="$T96_BASE/bin/trackfw"
mkdir -p "$T96_BASE/bin"
build_go_or_fail "setup-s166-baseline-build" "$T96_BASE" "$T96_BASE_BIN"
if ! GO_BIN="$T96_BASE_BIN" bash "$T96_BASE/scripts/check-artifact-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s166-baseline]: check-artifact-parity.sh ja reprova contra a copia limpa -- prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/artifact-parity/wave0-removed-synced-baseline]"

# Deteccao -- copia SABOTADA: mesma string trocada nos 3 geradores.
T96="$WORK/s166-detection"
setup_s166_tree "$T96"
sed -i.bak 's/## Wave 0 — Threat Model/## Wave 9 — Not Threat Model/' \
  "$T96/internal/generators/roadmap.go"
sed -i.bak 's/## Wave 0 — Threat Model/## Wave 9 — Not Threat Model/' \
  "$T96/npm/src/generators/roadmap.js"
sed -i.bak 's/## Wave 0 — Threat Model/## Wave 9 — Not Threat Model/' \
  "$T96/pypi/trackfw/generators/roadmap.py"
rm -f "$T96/internal/generators/roadmap.go.bak" \
      "$T96/npm/src/generators/roadmap.js.bak" \
      "$T96/pypi/trackfw/generators/roadmap.py.bak"
for pair in \
  "internal/generators/roadmap.go" \
  "npm/src/generators/roadmap.js" \
  "pypi/trackfw/generators/roadmap.py"; do
  if cmp -s "$ROOT_DIR/$pair" "$T96/$pair"; then
    echo "FAIL [falsify/setup-s166]: sed nao alterou $pair -- padrao nao encontrado; prova P4 invalida" >&2
    exit 1
  fi
done
T96_BIN="$T96/bin/trackfw"
mkdir -p "$T96/bin"
build_go_or_fail "setup-s166-build" "$T96" "$T96_BIN"

assert_fails_with "artifact-parity/wave0-removed-synced-detected" \
  "artifact content drift: roadmap (go)" \
  env GO_BIN="$T96_BIN" bash "$T96/scripts/check-artifact-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 167 -- Direcao B (AC9, mesmo roadmap, ML-2A): `barrier --wave 0`
#                volta a ser recusado (regressao do lower bound de
#                parseWaves, internal/commands/barrier.go, mesma classe
#                revertida pelo ML-1A -- "0" volta a contar como malformado).
#                Prova que o cenario 11 invertido de check-barrier.sh
#                (barrier/wave-label/wave-zero-accepted) e load-bearing: sem
#                a correcao de ML-1A, o fixture com "## Wave 0" reprova.
#
# Sabotagem Go-only (mesmo padrao dos Cenarios 164/165): so parseWaves e
# exercitado pelo fixture do Cenario 11 (a validacao do FLAG em
# newBarrierCmd nao entra em jogo porque o fixture chama `--wave 0`, que
# passa a validacao do flag e so tropeca ao ler o cabecalho do roadmap).
# ---------------------------------------------------------------------------
T97="$WORK/s167"
mkdir -p "$T97/cmd" "$T97/internal"
cp -r "$ROOT_DIR/cmd/." "$T97/cmd/"
cp -r "$ROOT_DIR/internal/." "$T97/internal/"
cp "$ROOT_DIR/go.mod" "$T97/go.mod"
cp "$ROOT_DIR/go.sum" "$T97/go.sum"

sed 's/intVal < 0 {/intVal < 1 {/' \
  "$ROOT_DIR/internal/commands/barrier.go" > "$T97/internal/commands/barrier.go"

if cmp -s "$ROOT_DIR/internal/commands/barrier.go" "$T97/internal/commands/barrier.go"; then
  echo "FAIL [falsify/setup-s167]: sed nao alterou barrier.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T97_BIN="$WORK/s167-bin/trackfw"
mkdir -p "$(dirname "$T97_BIN")"
build_go_or_fail "setup-s167-build" "$T97" "$T97_BIN"

# Baseline -- binario REAL contra o fixture real de check-barrier.sh (BIS_SELFTEST_BREAK
# desligado): a suite inteira precisa passar antes de provar a deteccao.
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-barrier.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s167-baseline]: check-barrier.sh ja reprova com o binario real -- prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/barrier/wave-zero-rejected-again-baseline]"

assert_fails_with "barrier/wave-zero-rejected-again-detected" \
  "expected exit 0 or 1 (never 2" \
  env GO_BIN="$T97_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenario 168 -- Direcao B, SEGUNDO guarda (AC9, mesmo roadmap, ML-2A):
#                complementa o Cenario 167. ML-1A's audit finding #1 (docs/
#                agents-working-context.md, hades-tf FIM: ML-0A) e o modelo
#                de ameaca (Sec3 F2) sao explicitos: "barrier tem DOIS
#                guardas contra --wave 0, nao um" -- a validacao do FLAG em
#                newBarrierCmd (linha ~92, waveInt < 0) e o guarda dentro de
#                parseWaves (linha ~208, intVal < 0, coberto pelo Cenario
#                167). Uma regressao que reverta SO o guarda do flag nunca
#                chega a ler o roadmap -- sai em "trackfw barrier: invalid
#                --wave \"0\" -- not a valid wave label" (mensagem distinta
#                da de parseWaves, "malformed wave heading..."), mas ainda
#                assim exit 2, e ainda assim capturado pelo mesmo assert do
#                Cenario 11 invertido ("expected exit 0 or 1 (never 2"). Sem
#                este cenario, o segundo guarda ficaria sem prova de
#                falsificacao dedicada -- um dos dois pontos do achado
#                ficaria fechado so por inspecao, nao por gate.
# ---------------------------------------------------------------------------
T99="$WORK/s168"
mkdir -p "$T99/cmd" "$T99/internal"
cp -r "$ROOT_DIR/cmd/." "$T99/cmd/"
cp -r "$ROOT_DIR/internal/." "$T99/internal/"
cp "$ROOT_DIR/go.mod" "$T99/go.mod"
cp "$ROOT_DIR/go.sum" "$T99/go.sum"

sed 's/waveInt < 0 {/waveInt < 1 {/' \
  "$ROOT_DIR/internal/commands/barrier.go" > "$T99/internal/commands/barrier.go"

if cmp -s "$ROOT_DIR/internal/commands/barrier.go" "$T99/internal/commands/barrier.go"; then
  echo "FAIL [falsify/setup-s168]: sed nao alterou barrier.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T99_BIN="$WORK/s168-bin/trackfw"
mkdir -p "$(dirname "$T99_BIN")"
build_go_or_fail "setup-s168-build" "$T99" "$T99_BIN"

# Baseline ja provado pelo Cenario 167 (mesmo binario real, mesmo
# check-barrier.sh) -- reexecutar aqui seria redundante; a garantia de
# nao-vacuidade do braco de deteccao vem do assert_fails_with abaixo.
echo "OK   [falsify/barrier/wave-zero-flag-guard-rejected-again-baseline]: reaproveita a baseline do Cenario 167 (mesmo binario real, mesmo check-barrier.sh)"

assert_fails_with "barrier/wave-zero-flag-guard-rejected-again-detected" \
  "expected exit 0 or 1 (never 2" \
  env GO_BIN="$T99_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"


# ---------------------------------------------------------------------------
# Cenario 169 -- Direcao A: escopo global volta a ler do cwd em vez de
#                ~/.trackfw/trackfw.yaml (seam: integrations_flags.go:225,
#                config.ResolveAgentModels(opts.scope, ...) substituido por
#                config.Load().AgentModels, "").
#                Deteccao: check-agent-models-parity.sh Case 6 (dois cwds com
#                global pin identico mas cwd-b com sonnet:9.9 distinto --
#                se o escopo global le o cwd, cwd-a produz model: sonnet e
#                cwd-b produz claude-sonnet-9-9; a comparacao cross-cwd reprova
#                com "global scope reads cwd instead of global config").
# ---------------------------------------------------------------------------
T169="$WORK/s169"
mkdir -p "$T169/cmd" "$T169/internal"
cp -r "$ROOT_DIR/cmd/." "$T169/cmd/"
cp -r "$ROOT_DIR/internal/." "$T169/internal/"
cp "$ROOT_DIR/go.mod" "$T169/go.mod"
cp "$ROOT_DIR/go.sum" "$T169/go.sum"

sed 's/config\.ResolveAgentModels(opts\.scope, manager\.HomeDir, manager\.ProjectRoot)/config.Load().AgentModels, ""/' \
  "$ROOT_DIR/internal/commands/integrations_flags.go" > "$T169/internal/commands/integrations_flags.go"

if cmp -s "$ROOT_DIR/internal/commands/integrations_flags.go" "$T169/internal/commands/integrations_flags.go"; then
  echo "FAIL [falsify/setup-s169]: sed nao alterou integrations_flags.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T169_BIN="$WORK/s169-bin/trackfw"
mkdir -p "$(dirname "$T169_BIN")"
build_go_or_fail "setup-s169-build" "$T169" "$T169_BIN"

# Baseline -- binario REAL: check-agent-models-parity.sh deve passar antes da deteccao
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s169-baseline]: check-agent-models-parity.sh ja reprova com o binario real -- prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/global-scope/direction-a-reads-cwd-baseline]"

assert_fails_with "global-scope/direction-a-reads-cwd-detected" \
  "from global pin" \
  env GO_BIN="$T169_BIN" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 170 -- Direcao B: escopo de projeto passa a ler do global em vez
#                do projeto (seam: integrations_flags.go:225, opts.scope
#                substituido por "global" em config.ResolveAgentModels).
#                Deteccao: check-agent-models-parity.sh Case 9 (projeto com
#                sonnet:9.9, global com sonnet:4.6 -- se o escopo de projeto
#                ler do global, o modelo seria claude-sonnet-4-6 em vez de
#                claude-sonnet-9-9; a vacuity guard reprova com
#                "claude-sonnet-9-9' (project pin)" — aspas simples faz parte
#                da mensagem diag: "missing 'model: claude-sonnet-9-9' (project pin)").
#                Baseline reaproveita do Cenario 169 (mesmo binario real,
#                mesmo check-agent-models-parity.sh).
# ---------------------------------------------------------------------------
T170="$WORK/s170"
mkdir -p "$T170/cmd" "$T170/internal"
cp -r "$ROOT_DIR/cmd/." "$T170/cmd/"
cp -r "$ROOT_DIR/internal/." "$T170/internal/"
cp "$ROOT_DIR/go.mod" "$T170/go.mod"
cp "$ROOT_DIR/go.sum" "$T170/go.sum"

sed 's/config\.ResolveAgentModels(opts\.scope,/config.ResolveAgentModels("global",/' \
  "$ROOT_DIR/internal/commands/integrations_flags.go" > "$T170/internal/commands/integrations_flags.go"

if cmp -s "$ROOT_DIR/internal/commands/integrations_flags.go" "$T170/internal/commands/integrations_flags.go"; then
  echo "FAIL [falsify/setup-s170]: sed nao alterou integrations_flags.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T170_BIN="$WORK/s170-bin/trackfw"
mkdir -p "$(dirname "$T170_BIN")"
build_go_or_fail "setup-s170-build" "$T170" "$T170_BIN"

# Baseline reaproveita do Cenario 169 (mesmo binario real, mesmo gate)
echo "OK   [falsify/global-scope/direction-b-reads-global-baseline]: reaproveita baseline do Cenario 169"

assert_fails_with "global-scope/direction-b-reads-global-detected" \
  "claude-sonnet-9-9' (project pin)" \
  env GO_BIN="$T170_BIN" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh"


# ---------------------------------------------------------------------------
# Cenario 171 -- Direcao A: sanitizacao do titulo removida do roadmap.go
#                (ML-1A da REQ-2026-08-23) faz o Cenario 13 do check-barrier.sh
#                falhar. Sabotagem Go-only: strings.ContainsAny(content.Title,
#                "\n\r") substituido por false -- o bloco de sanitizacao vira dead
#                code e roadmap new com titulo forjado passa a retornar exit 0.
#                Deteccao: check-barrier.sh Scenario 13 reprova com
#                "expected 'roadmap title must be a single line'".
# ---------------------------------------------------------------------------
T171="$WORK/s171"
mkdir -p "$T171/cmd" "$T171/internal"
cp -r "$ROOT_DIR/cmd/." "$T171/cmd/"
cp -r "$ROOT_DIR/internal/." "$T171/internal/"
cp "$ROOT_DIR/go.mod" "$T171/go.mod"
cp "$ROOT_DIR/go.sum" "$T171/go.sum"

sed 's/strings\.ContainsAny(content\.Title, "\\n\\r")/false/' \
  "$ROOT_DIR/internal/generators/roadmap.go" > "$T171/internal/generators/roadmap.go"

if cmp -s "$ROOT_DIR/internal/generators/roadmap.go" "$T171/internal/generators/roadmap.go"; then
  echo "FAIL [falsify/setup-s171]: sed nao alterou roadmap.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T171_BIN="$WORK/s171-bin/trackfw"
mkdir -p "$(dirname "$T171_BIN")"
build_go_or_fail "setup-s171-build" "$T171" "$T171_BIN"

# Baseline -- binario REAL: check-barrier.sh deve passar antes da deteccao
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-barrier.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s171-baseline]: check-barrier.sh ja reprova com o binario real -- prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/ac2-sanitization/direction-a-baseline]"

assert_fails_with "ac2-sanitization/direction-a-detected" \
  "expected exit non-0 for forged title" \
  env GO_BIN="$T171_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenario 172 -- Direcao B: verificacao de confianca removida do barrier.go
#                (ML-2A da REQ-2026-08-23) faz o Cenario 14 do check-barrier.sh
#                falhar. Sabotagem Go-only: `if !verdict.trusted {` substituido
#                por `if false {` -- o caminho not_evaluated nunca e tomado e o
#                gate sempre executa, mesmo sem --trust-local-gates.
#                Deteccao: check-barrier.sh Scenario 14 reprova com
#                "hostile gate EXECUTED -- sentinel was created" (AC14: a prova
#                e pela ausencia do arquivo, nao pelo exit code).
#                Baseline reaproveita do Cenario 171 (mesmo binario real,
#                mesmo check-barrier.sh).
# ---------------------------------------------------------------------------
T172="$WORK/s172"
mkdir -p "$T172/cmd" "$T172/internal"
cp -r "$ROOT_DIR/cmd/." "$T172/cmd/"
cp -r "$ROOT_DIR/internal/." "$T172/internal/"
cp "$ROOT_DIR/go.mod" "$T172/go.mod"
cp "$ROOT_DIR/go.sum" "$T172/go.sum"

sed 's/if !verdict\.trusted {/if false {/' \
  "$ROOT_DIR/internal/commands/barrier.go" > "$T172/internal/commands/barrier.go"

if cmp -s "$ROOT_DIR/internal/commands/barrier.go" "$T172/internal/commands/barrier.go"; then
  echo "FAIL [falsify/setup-s172]: sed nao alterou barrier.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T172_BIN="$WORK/s172-bin/trackfw"
mkdir -p "$(dirname "$T172_BIN")"
build_go_or_fail "setup-s172-build" "$T172" "$T172_BIN"

# Baseline reaproveita do Cenario 171 (mesmo binario real, mesmo gate)
echo "OK   [falsify/trust-check/direction-b-baseline]: reaproveita baseline do Cenario 171"

assert_fails_with "trust-check/direction-b-detected" \
  "hostile gate EXECUTED" \
  env GO_BIN="$T172_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenario 173 -- Direcao A: digest constante em auditsurface.go (sha256.Sum256
#                substituido por hash de nil) faz o Cenario FN-2 do
#                check-audit-surface.sh falhar: digest igual entre os dois refs
#                (wiring inalterado, script diferente) onde deveria diferir.
#                Seam: AUDIT_SURFACE_SELFTEST_BREAK=A instrui o gate a construir
#                um binario Go sabotado internamente, sem precisar passar GO_BIN.
#                Deteccao: check-audit-surface.sh reprova com
#                "audit-surface/fn-2/digest-changes-when-script-changes".
# ---------------------------------------------------------------------------
# Baseline -- gate passa com o binario real antes da deteccao
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-audit-surface.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s173-baseline]: check-audit-surface.sh ja reprova com o binario real -- prova invalida" >&2
  exit 1
fi
echo "OK   [falsify/audit-surface/direction-a-baseline]"

assert_fails_with "audit-surface/direction-a-detected" \
  "audit-surface/fn-2/digest-changes-when-script-changes" \
  env AUDIT_SURFACE_SELFTEST_BREAK=A bash "$ROOT_DIR/scripts/check-audit-surface.sh"

# ---------------------------------------------------------------------------
# Cenario 174 -- Direcao B: "docs/cli-parity.md" inserido em
#                instructionFilePaths em auditsurface.go faz o Cenario FP-1 do
#                check-audit-surface.sh falhar: o arquivo aparece no output
#                (instruction [present] docs/cli-parity.md) quando o gate
#                asserta que nao deve aparecer (AC16).
#                Seam: AUDIT_SURFACE_SELFTEST_BREAK=B.
#                Baseline reaproveita do Cenario 173 (mesmo gate, mesmo binario real).
#                Deteccao: check-audit-surface.sh reprova com
#                "audit-surface/fp-1/cli-parity-absent".
# ---------------------------------------------------------------------------
echo "OK   [falsify/audit-surface/direction-b-baseline]: reaproveita baseline do Cenario 173"

assert_fails_with "audit-surface/direction-b-detected" \
  "audit-surface/fp-1/cli-parity-absent" \
  env AUDIT_SURFACE_SELFTEST_BREAK=B bash "$ROOT_DIR/scripts/check-audit-surface.sh"

# ---------------------------------------------------------------------------
# Cenario 175 -- Direcao A: add("trackfw.yaml") removido de
#                buildSandboxInclusion (internal/generators/update.go) faz o
#                Cenario 11 do check-update-parity.sh falhar: dry-run reporta
#                skipped onde run real reporta updated para fixture com
#                agent_conventions (trackfw.yaml ausente do sandbox =>
#                ReadAgentConventions retorna vazio => hash de CLAUDE.md difere
#                do run real).
#                Deteccao: check-update-parity.sh reprova com
#                "sandbox/gap-e/dry-vs-real/go".
# ---------------------------------------------------------------------------
T175="$WORK/s175"
mkdir -p "$T175/cmd" "$T175/internal"
cp -r "$ROOT_DIR/cmd/." "$T175/cmd/"
cp -r "$ROOT_DIR/internal/." "$T175/internal/"
cp "$ROOT_DIR/go.mod" "$T175/go.mod"
cp "$ROOT_DIR/go.sum" "$T175/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/update.go" \
  "$T175/internal/generators/update.go" \
  'add("trackfw.yaml")' \
  '_ = "trackfw.yaml" // SABOTAGE-S175: trackfw.yaml removed from sandbox' \
  'setup-s175'

T175_BIN="$WORK/s175-bin/trackfw"
mkdir -p "$(dirname "$T175_BIN")"
build_go_or_fail "setup-s175-build" "$T175" "$T175_BIN"

# Baseline -- binario REAL: check-update-parity.sh deve passar antes da deteccao
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-update-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s175-baseline]: check-update-parity.sh ja reprova com o binario real -- prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/sandbox-gap-e/direction-a-baseline]"

assert_fails_with "sandbox-gap-e/direction-a-detected" \
  "sandbox/gap-e/dry-vs-real/go" \
  env GO_BIN="$T175_BIN" bash "$ROOT_DIR/scripts/check-update-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 176 -- Direcao B: corpo de copyProjectTree substituido por
#                filepath.WalkDir + os.ReadFile (reintroduzindo a travessia da
#                arvore inteira que aborta em symlinks pendurados fora do
#                conjunto declarado — o incidente do CMDB do KG).
#                Baseline reaproveita do Cenario 175 (mesmo gate, mesmo binario real).
#                Deteccao: check-update-parity.sh reprova com
#                "sandbox/dangling-outside-set/exit-zero/go".
# ---------------------------------------------------------------------------
T176="$WORK/s176"
mkdir -p "$T176/cmd" "$T176/internal"
cp -r "$ROOT_DIR/cmd/." "$T176/cmd/"
cp -r "$ROOT_DIR/internal/." "$T176/internal/"
cp "$ROOT_DIR/go.mod" "$T176/go.mod"
cp "$ROOT_DIR/go.sum" "$T176/go.sum"

python3 - "$ROOT_DIR/internal/generators/update.go" "$T176/internal/generators/update.go" <<'PY'
import pathlib, sys

src_path, dest_path = sys.argv[1], sys.argv[2]
text = pathlib.Path(src_path).read_text(encoding="utf-8")

old = (
    "\tfor _, rel := range paths {\n"
    "\t\tif err := copyPath(filepath.Join(src, rel), filepath.Join(dst, rel)); err != nil {\n"
    "\t\t\treturn fmt.Errorf(\"sandbox: copying %s: %w\", rel, err)\n"
    "\t\t}\n"
    "\t}\n"
    "\treturn nil"
)

new = (
    "\treturn filepath.WalkDir(src, func(fpath string, d fs.DirEntry, err error) error {\n"
    "\t\tif err != nil {\n"
    "\t\t\treturn err\n"
    "\t\t}\n"
    "\t\trel, _ := filepath.Rel(src, fpath)\n"
    "\t\tif d.IsDir() {\n"
    "\t\t\treturn os.MkdirAll(filepath.Join(dst, rel), 0o755)\n"
    "\t\t}\n"
    "\t\tdata, err := os.ReadFile(fpath)\n"
    "\t\tif err != nil {\n"
    "\t\t\treturn err\n"
    "\t\t}\n"
    "\t\treturn os.WriteFile(filepath.Join(dst, rel), data, 0o644)\n"
    "\t})"
)

count = text.count(old)
if count != 1:
    raise SystemExit(f"[setup-s176] expected exactly 1 occurrence of loop body, got {count}")
pathlib.Path(dest_path).write_text(text.replace(old, new, 1), encoding="utf-8")
PY

if cmp -s "$ROOT_DIR/internal/generators/update.go" "$T176/internal/generators/update.go"; then
  echo "FAIL [falsify/setup-s176]: python3 nao alterou update.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T176_BIN="$WORK/s176-bin/trackfw"
mkdir -p "$(dirname "$T176_BIN")"
build_go_or_fail "setup-s176-build" "$T176" "$T176_BIN"

echo "OK   [falsify/sandbox-walkdir-reintroduced/direction-b-baseline]: reaproveita baseline do Cenario 175"

assert_fails_with "sandbox-walkdir-reintroduced/direction-b-detected" \
  "sandbox/dangling-outside-set/exit-zero/go" \
  env GO_BIN="$T176_BIN" bash "$ROOT_DIR/scripts/check-update-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 177 -- Direcao A: checkScaffoldArtifact silenciado (nunca reporta
#                divergencia de scaffold). A funcao e corrompida substituindo
#                `bytes.Equal(actual, expected)` por `bytes.Equal(actual, actual)`
#                (comparacao do conteudo contra si mesmo — sempre verdadeira),
#                fazendo a funcao retornar nil para qualquer artefato e
#                ocultando toda divergencia de scaffold. O gate
#                check-doctor-parity.sh detecta via vacuity guard do cenario
#                (h) (scaffold-attention-signal-divergent): o binario sabotado
#                reporta "no mismatches found" onde o Go real reportaria
#                [scaffold-divergent], e o label
#                "scaffold-attention-signal-divergent-text/go" aparece na linha
#                FAIL.
#                ROADMAP: ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold
#                ML-2A, AC8 direcao A.
# ---------------------------------------------------------------------------
T177="$WORK/s177"
mkdir -p "$T177/cmd" "$T177/internal"
cp -r "$ROOT_DIR/cmd/." "$T177/cmd/"
cp -r "$ROOT_DIR/internal/." "$T177/internal/"
cp "$ROOT_DIR/go.mod" "$T177/go.mod"
cp "$ROOT_DIR/go.sum" "$T177/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold_doctor.go" \
  "$T177/internal/generators/scaffold_doctor.go" \
  'bytes.Equal(actual, expected)' \
  'bytes.Equal(actual, actual)' \
  'setup-s177'

T177_BIN="$WORK/s177-bin/trackfw"
mkdir -p "$(dirname "$T177_BIN")"
build_go_or_fail "setup-s177-build" "$T177" "$T177_BIN"

# Baseline -- binario REAL: check-doctor-parity.sh deve passar antes da deteccao
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-doctor-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s177-baseline]: check-doctor-parity.sh ja reprova com o binario real -- prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/scaffold-divergent-silenced/direction-a-baseline]"

assert_fails_with "scaffold-divergent-silenced/direction-a-detected" \
  "scaffold-attention-signal-divergent-text/go" \
  env GO_BIN="$T177_BIN" bash "$ROOT_DIR/scripts/check-doctor-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 178 -- Direcao B: artefato integro acusado como divergente.
#                A funcao checkScaffoldArtifact e corrompida substituindo
#                `bytes.Equal(actual, expected)` por
#                `!bytes.Equal(actual, expected)` (inversao da guarda de
#                igualdade): quando o conteudo bate com o template (projeto
#                integro), a condicao nao se satisfaz e a funcao continua ate
#                retornar um finding scaffold-divergent — falso positivo. O
#                gate check-doctor-parity.sh detecta via vacuity guard do
#                cenario (g) (scaffold-baseline-clean): o binario sabotado
#                reporta [scaffold-divergent] onde o Go real reportaria
#                "no mismatches found", e o label
#                "scaffold-baseline-clean-text/go" aparece na linha FAIL.
#                Baseline reaproveita do Cenario 177 (mesmo gate, mesmo
#                binario real).
#                ROADMAP: ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold
#                ML-2A, AC8 direcao B.
# ---------------------------------------------------------------------------
T178="$WORK/s178"
mkdir -p "$T178/cmd" "$T178/internal"
cp -r "$ROOT_DIR/cmd/." "$T178/cmd/"
cp -r "$ROOT_DIR/internal/." "$T178/internal/"
cp "$ROOT_DIR/go.mod" "$T178/go.mod"
cp "$ROOT_DIR/go.sum" "$T178/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold_doctor.go" \
  "$T178/internal/generators/scaffold_doctor.go" \
  'bytes.Equal(actual, expected)' \
  '!bytes.Equal(actual, expected)' \
  'setup-s178'

T178_BIN="$WORK/s178-bin/trackfw"
mkdir -p "$(dirname "$T178_BIN")"
build_go_or_fail "setup-s178-build" "$T178" "$T178_BIN"

echo "OK   [falsify/scaffold-intact-accused/direction-b-baseline]: reaproveita baseline do Cenario 177"

assert_fails_with "scaffold-intact-accused/direction-b-detected" \
  "scaffold-baseline-clean-text/go" \
  env GO_BIN="$T178_BIN" bash "$ROOT_DIR/scripts/check-doctor-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 179 -- Direcao A: guarda execBit silenciada (o modo nao e verificado
#                para nenhum artefato). checkScaffoldArtifact corrompida:
#                `execBit &&` → `false  &&` faz a condicao da verificacao de
#                modo ser sempre falsa; artefatos com bit de execucao ausente
#                (como o script do cenario (p)) passam sem ser acusados. O
#                gate check-doctor-parity.sh detecta via vacuity guard do
#                cenario (p) (scaffold-wrong-mode-detected): o binario sabotado
#                nao reporta [scaffold-wrong-mode] onde o Go real reportaria, e
#                o label "scaffold-wrong-mode-detected-text/go" aparece na
#                linha FAIL.
#                Nota: a sabotagem atinge a condicao em scaffold_doctor.go:324
#                (`if execBit && CurrentGOOS != "windows" && !execBitPresent`);
#                a verificacao checkValidateScriptArtifact (~linha 269), que
#                nao tem parametro execBit, nao e coberta por este cenario.
#                ROADMAP: ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-
#                dos-artefatos-de-scaffold, ML-2A, AC7/AC8 direcao A.
# ---------------------------------------------------------------------------
T179="$WORK/s179"
mkdir -p "$T179/cmd" "$T179/internal"
cp -r "$ROOT_DIR/cmd/." "$T179/cmd/"
cp -r "$ROOT_DIR/internal/." "$T179/internal/"
cp "$ROOT_DIR/go.mod" "$T179/go.mod"
cp "$ROOT_DIR/go.sum" "$T179/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold_doctor.go" \
  "$T179/internal/generators/scaffold_doctor.go" \
  'execBit &&' \
  'false  &&' \
  'setup-s179'

T179_BIN="$WORK/s179-bin/trackfw"
mkdir -p "$(dirname "$T179_BIN")"
build_go_or_fail "setup-s179-build" "$T179" "$T179_BIN"

# Baseline -- binario REAL: check-doctor-parity.sh deve passar antes da deteccao
if ! GO_BIN="$ROOT_DIR/bin/trackfw" bash "$ROOT_DIR/scripts/check-doctor-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s179-baseline]: check-doctor-parity.sh ja reprova com o binario real -- prova P4 invalida" >&2
  exit 1
fi
echo "OK   [falsify/scaffold-mode-check-silenced/direction-a-baseline]"

assert_fails_with "scaffold-mode-check-silenced/direction-a-detected" \
  "scaffold-wrong-mode-detected-text/go" \
  env GO_BIN="$T179_BIN" bash "$ROOT_DIR/scripts/check-doctor-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 180 -- Direcao B: discriminante execBit silenciado (o doctor verifica
#                o bit de execucao em TODOS os artefatos, incluindo os 0644).
#                checkScaffoldArtifact corrompida: `execBit &&` → `true   &&`
#                faz a condicao ser sempre verdadeira; artefatos 0644 (slash
#                commands .claude/commands/trackfw/*.md) sao acusados de
#                scaffold-wrong-mode mesmo sem ter execBit=true no descritor.
#                O gate e invocado diretamente (Go only, nao check-doctor-
#                parity.sh) porque Python emite uma linha extra de progresso
#                ao varrer .claude/commands/trackfw/, producindo divergencia
#                de stdout que nada tem a ver com o modo de execucao.
#                Baseline: binario real nao acusa nenhum slash command de
#                scaffold-wrong-mode. Deteccao: binario sabotado acusa os 9
#                slash commands de scaffold-wrong-mode (falso positivo, AC4/
#                AC11 violados).
#                ROADMAP: ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-
#                dos-artefatos-de-scaffold, ML-2A, AC7/AC8 direcao B.
# ---------------------------------------------------------------------------
T180="$WORK/s180"
mkdir -p "$T180/cmd" "$T180/internal"
cp -r "$ROOT_DIR/cmd/." "$T180/cmd/"
cp -r "$ROOT_DIR/internal/." "$T180/internal/"
cp "$ROOT_DIR/go.mod" "$T180/go.mod"
cp "$ROOT_DIR/go.sum" "$T180/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold_doctor.go" \
  "$T180/internal/generators/scaffold_doctor.go" \
  'execBit &&' \
  'true   &&' \
  'setup-s180'

T180_BIN="$WORK/s180-bin/trackfw"
mkdir -p "$(dirname "$T180_BIN")"
build_go_or_fail "setup-s180-build" "$T180" "$T180_BIN"

# Fixture com slash commands -- artefatos 0644 que nunca devem ser acusados
T180_PROJ="$WORK/s180-fixture/project"
T180_HOME="$WORK/s180-fixture/home"
mkdir -p "$T180_PROJ" "$T180_HOME/.trackfw"
printf '%s\n' '{"schema_version":1,"user_nickname":"KG","agents":{"backend":{"display_name":"Apolo","slug":"apolo"}}}' \
  >"$T180_HOME/.trackfw/identity.json"
printf 'governance_mode: lenient\nadr_dirs:\n  - docs/adr\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\n' \
  >"$T180_PROJ/trackfw.yaml"
(cd "$T180_PROJ" && HOME="$T180_HOME" "$ROOT_DIR/bin/trackfw" \
  update --install-missing --targets validate-script,agent-hooks,claude-commands) >/dev/null

# Baseline: binario REAL nao deve acusar slash commands de scaffold-wrong-mode
t180_baseline_out=$(cd "$T180_PROJ" && HOME="$T180_HOME" "$ROOT_DIR/bin/trackfw" doctor 2>&1)
if printf '%s\n' "$t180_baseline_out" | grep -F "[scaffold-wrong-mode]" | grep -qF "commands/trackfw"; then
  echo "FAIL [falsify/setup-s180-baseline]: binario real acusou scaffold-wrong-mode em slash command -- baseline invalida" >&2
  exit 1
fi
echo "OK   [falsify/scaffold-execbit-discriminant-silenced/direction-b-baseline]"

# Deteccao: binario sabotado deve acusar pelo menos um slash command de scaffold-wrong-mode
t180_det_out=$(cd "$T180_PROJ" && HOME="$T180_HOME" "$T180_BIN" doctor 2>&1)
if printf '%s\n' "$t180_det_out" | grep -F "[scaffold-wrong-mode]" | grep -qF "commands/trackfw"; then
  echo "OK   [falsify/scaffold-execbit-discriminant-silenced/direction-b-detected]"
else
  echo "FAIL [falsify/scaffold-execbit-discriminant-silenced/direction-b-detected]: binario sabotado (execBit && -> true &&) nao acusou scaffold-wrong-mode em nenhum slash command 0644" >&2
  echo "  doctor output:" >&2
  printf '%s\n' "$t180_det_out" | sed 's/^/    /' >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Cenario 181 -- Direcao C: os.Chmod removido de generateValidateScript
#                (scaffold.go). Sem o Chmod, trackfw update reescreve o
#                conteudo (apply() roda via runFileTarget para arquivo
#                existente) mas nao restaura o bit de execucao — o ciclo
#                "doctor acusa → update nao remedia → doctor acusa de novo"
#                reaparece (AC9). O cenario verifica o EFEITO: o bit foi
#                restaurado no braco de baseline, e permanece ausente no
#                braco de deteccao APOS o update reescrever o conteudo.
#                Braco de baseline: baixa o bit → binario REAL update
#                --targets validate-script → test -x confirma o bit voltou.
#                Braco de deteccao: corrompe conteudo + baixa o bit → binario
#                sabotado update --targets validate-script → cmp -s confirma
#                conteudo restaurado (apply() rodou) → test ! -x confirma bit
#                ainda ausente (Chmod nao rodou).
#                ROADMAP: ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-
#                dos-artefatos-de-scaffold, ML-2A, AC7/AC8 direcao C.
# ---------------------------------------------------------------------------
T181="$WORK/s181"
mkdir -p "$T181/cmd" "$T181/internal"
cp -r "$ROOT_DIR/cmd/." "$T181/cmd/"
cp -r "$ROOT_DIR/internal/." "$T181/internal/"
cp "$ROOT_DIR/go.mod" "$T181/go.mod"
cp "$ROOT_DIR/go.sum" "$T181/go.sum"

python3 - "$ROOT_DIR/internal/generators/scaffold.go" \
          "$T181/internal/generators/scaffold.go" <<'PY'
import pathlib, sys

src_path, dest_path = sys.argv[1], sys.argv[2]
text = pathlib.Path(src_path).read_text(encoding="utf-8")

old = (
    '\tif err := os.Chmod(path, 0755); err != nil {\n'
    '\t\treturn fmt.Errorf("setting execute bit on validate script: %w", err)\n'
    '\t}\n'
)
new = '\t// falsify: os.Chmod removed (AC9 regression probe -- Direction C)\n'

count = text.count(old)
if count != 1:
    raise SystemExit(f"[setup-s181] expected exactly 1 occurrence of Chmod block, got {count}")
pathlib.Path(dest_path).write_text(text.replace(old, new, 1), encoding="utf-8")
PY

if cmp -s "$ROOT_DIR/internal/generators/scaffold.go" "$T181/internal/generators/scaffold.go"; then
  echo "FAIL [falsify/setup-s181]: python3 nao alterou scaffold.go -- padrao nao encontrado; prova P4 invalida" >&2
  exit 1
fi

T181_BIN="$WORK/s181-bin/trackfw"
mkdir -p "$(dirname "$T181_BIN")"
build_go_or_fail "setup-s181-build" "$T181" "$T181_BIN"

_S181_ID='{"schema_version":1,"user_nickname":"KG","agents":{"backend":{"display_name":"Apolo","slug":"apolo"}}}'
_S181_CFG='governance_mode: lenient
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
'

# Braco de baseline: REAL binary restaura o bit apos ser baixado
T181_BASE_PROJ="$WORK/s181-base/project"
T181_BASE_HOME="$WORK/s181-base/home"
mkdir -p "$T181_BASE_PROJ" "$T181_BASE_HOME/.trackfw"
printf '%s\n' "$_S181_ID" >"$T181_BASE_HOME/.trackfw/identity.json"
printf '%s' "$_S181_CFG" >"$T181_BASE_PROJ/trackfw.yaml"
(cd "$T181_BASE_PROJ" && HOME="$T181_BASE_HOME" "$ROOT_DIR/bin/trackfw" \
  update --install-missing --targets validate-script) >/dev/null
chmod 0644 "$T181_BASE_PROJ/scripts/trackfw-validate.sh"
(cd "$T181_BASE_PROJ" && HOME="$T181_BASE_HOME" "$ROOT_DIR/bin/trackfw" \
  update --targets validate-script) >/dev/null
if test -x "$T181_BASE_PROJ/scripts/trackfw-validate.sh"; then
  echo "OK   [falsify/scaffold-update-chmod-removed/direction-c-baseline]"
else
  echo "FAIL [falsify/scaffold-update-chmod-removed/direction-c-baseline]: binario real nao restaurou o bit de execucao apos update" >&2
  ls -la "$T181_BASE_PROJ/scripts/trackfw-validate.sh" >&2
  exit 1
fi

# Braco de deteccao: sabotaged binary restaura o conteudo mas NAO o bit
T181_DET_PROJ="$WORK/s181-det/project"
T181_DET_HOME="$WORK/s181-det/home"
mkdir -p "$T181_DET_PROJ" "$T181_DET_HOME/.trackfw"
printf '%s\n' "$_S181_ID" >"$T181_DET_HOME/.trackfw/identity.json"
printf '%s' "$_S181_CFG" >"$T181_DET_PROJ/trackfw.yaml"
(cd "$T181_DET_PROJ" && HOME="$T181_DET_HOME" "$ROOT_DIR/bin/trackfw" \
  update --install-missing --targets validate-script) >/dev/null
T181_SCRIPT="$T181_DET_PROJ/scripts/trackfw-validate.sh"
# Salva conteudo canonico antes de corromper
cp "$T181_SCRIPT" "$WORK/s181-canonical.sh"
# Corrompe conteudo (apply() deve detectar e restaurar) e baixa o bit
printf 'X' >>"$T181_SCRIPT"
chmod 0644 "$T181_SCRIPT"
# Roda binario sabotado
(cd "$T181_DET_PROJ" && HOME="$T181_DET_HOME" "$T181_BIN" \
  update --targets validate-script) >/dev/null
# Verifica: conteudo restaurado (apply() rodou)
if ! cmp -s "$WORK/s181-canonical.sh" "$T181_SCRIPT"; then
  echo "FAIL [falsify/scaffold-update-chmod-removed/direction-c-detected]: binario sabotado nao restaurou o conteudo -- apply() nao rodou" >&2
  exit 1
fi
# Verifica: bit ainda ausente (Chmod nao rodou)
if test -x "$T181_SCRIPT"; then
  echo "FAIL [falsify/scaffold-update-chmod-removed/direction-c-detected]: binario sabotado restaurou o bit de execucao -- os.Chmod nao foi removido" >&2
  ls -la "$T181_SCRIPT" >&2
  exit 1
fi
echo "OK   [falsify/scaffold-update-chmod-removed/direction-c-detected]"

echo "Falsification checks passed (all 181 scenarios, 23 gates + 11 generator/validator contracts — roadmap acceptance heading (24), req frontmatter --from-req path (25, baseline + detection) and --req simple path AC2b (26, baseline + detection), adr_accepted_when_req_done + blocked_by_draft_adr (27, baseline + baseline-negative + detection, 2 rules x 3 CLIs), backtick-wrapped ADR reference without frontmatter adr: field (28, baseline + detection, 3 CLIs), validate success message pinned + byte-identical across 3 CLIs (29, baseline + detection), status Inventory block flat mode pinned + byte-identical with analyzing/REQ-status discriminant fixture (30, baseline + Go analyzing-omission detection), status Inventory + WIP by Agent block by_agent mode pinned + byte-identical (31, baseline + Python WIP-by-Agent body-drift detection), unpaired reference delimiter in adr_accepted_when_req_done fixture — Python-only regression (32, baseline 3 CLIs + Python detection), status by_agent fallback order without agents: configured — Python-only regression (33, baseline 3 CLIs pinned + Python detection with positional assertion), config parser unindented block sequence for agents: — Go+Node-only regression (34, baseline 3 CLIs pinned + Go and Node detection via agent_namespace_undeclared violation presence, RETARGETED 2026-08-02 for the yaml.v3/yaml-2.x migration — original literal removed by ML-1A — RETARGETED AGAIN 2026-08-29 ML-3A from positional/order assertion to violation-message presence after REQ-2026-08-29's union made ordering the weaker discriminant), config parser inline list item with comma-inside-quotes for agents: — 3 CLIs regression (35, baseline 3 CLIs pinned + Go/Node/Python detection via agent_namespace_undeclared violation presence, RETARGETED 2026-08-02 for the yaml.v3/yaml-2.x migration — original splitTopLevelCommas literal removed by ML-1A — RETARGETED AGAIN 2026-08-29 ML-3A from positional/order assertion to violation-message presence after REQ-2026-08-29's union made ordering the weaker discriminant), config scalar schema-fidelity (octal/bare-date/yes) via roadmap_dir+req_dir+adr_dirs — normalizeNode typed-scalar regression, each CLI diverges only on the case the ADR predicts (36, baseline 3 CLIs pinned + Go/Node/Python detection each isolating its own discriminant), malformed trackfw.yaml error path — stderr message + exit 1 byte-identical across 3 CLIs (37, baseline 3 CLIs + Go fatal-check-removed detection) — proved non-vacuous, wip_limit quoted-scalar regression via wipConfigFrom/_wip_config_from — validate() bypassing config.Load() with an artisanal trackfw.yaml re-read discriminated only by a quoted \"3\" scalar (38, baseline 3 CLIs pinned + Go/Node/Python detection reintroducing the readWIPConfig pattern eliminated by 74d70ee), \`trackfw update\` hooks/ci/backend/frontend/pkg_manager scanner regression via loadUpdateConfig/_load_update_config — nested homonym key discriminant (\`hooks: lefthook\` at root vs nested \`hooks: husky\`) reintroducing the ML-2A-eliminated any-indentation last-match-wins scanner, one cenario per CLI (39 Go, 40 Node.js, 41 Python — each baseline + detection; Python's braço exercises the bare \`trackfw update\` invocation per the ML-2A/Hefesto barrier constraint and adds a --dry-run blindness guard proving _run_project never reaches the loader), \`trackfw branch new\` no-match stderr message (\`blocked: no matching roadmap in wip/ nor done/ for ...\`) reformatted by Node.js — check-branch-new-parity.sh's go-vs-node stderr diff detects the divergence (42), attention-hook scripts (signal/cleanup) byte-identity across Go/Node.js/Python — Python's \"no-op fora da raiz\" comment corrupted in the cleanup script literal — check-attention-scripts-parity.sh's go-vs-py diff detects the divergence (43), per-CLI agent hook files (.claude/settings.json, .codex/hooks.json, .gemini/settings.json, .github/hooks/trackfw-attention.json, .cursor/hooks.json, .kiro/hooks/trackfw-attention.json) structural parity across Go/Node.js/Python for all 6 native-wave CLIs — Node.js's Kiro credential-guard-post matcher corrupted from 'shell' to 'execute_bash' — check-agent-hooks-parity.sh's go-vs-node structural diff detects the divergence at \$.hooks[3].matcher (44), global-scope credential-guard hook files (~/.claude/settings.json, ~/.codex/hooks.json, ~/.gemini/settings.json, ~/.cursor/hooks.json, ~/.copilot/settings.json, ~/.kiro/hooks/trackfw-credential-guard.json) written by \`trackfw update harness --targets <tool>-credential-guard --install-missing\` structural parity across Go/Node.js/Python for all 6 native-wave CLIs — Python's Kiro credential-guard-global-post matcher corrupted from 'shell' to 'execute_bash' — check-harness-hooks-parity.sh's go-vs-py structural diff detects the divergence at \$.hooks[1].matcher (45), check-agent-hooks-parity.sh's credential-guard-present vacuity guard (P2) — Go/Node.js/Python's globalCredentialGuardInstalledClaude/_global_credential_guard_installed_claude dedup forced to always report \"installed\" in 3 isolated source copies, dropping the project-scope credential-guard entry for Claude identically across all 3 stacks (structural comparator stays satisfied, never even reached — gate exits at the vacuity guard first) — proved non-vacuous against a neutered guard and proved the failure key is credential-guard-present, not go-vs-node/go-vs-py; detection arm made self-discriminating (ML-1B, ROADMAP-2026-08-12) against the 2026-08-08 environmental-leak failure mode via a test-controlled synthetic \$HOME (Codex-only global guard, no Claude) plus an exclusivity assertion that none of the 5 non-sabotaged CLIs may appear in the FAIL set — proved against a leak-only (no sabotage) adversarial variant that the pre-ML-1B assertion set was satisfiable by pure environmental leak and the new exclusivity check rejects it (46), \`trackfw validate\`'s credential_guard_hook_resolvable rule (ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A/ML-2A) — a registered project-scope Claude credential-guard hook (.claude/settings.json) whose referenced script is missing must be flagged, and must stay silent when the script is present and executable, exercised end-to-end via the real Go binary against an otherwise-empty scaffold_adr_req_project fixture (the same fixture Scenario 29 pins to zero violations, so no other rule has material to fire) — detection arm asserts the exact validator diagnostic literal (unique across internal/validator/*.go per grep) rather than a generic non-zero exit, proved non-vacuous, no \$HOME dependency by design since the rule never reads outside the project root (47), check-attention-scripts-parity.sh extended (ML-0B, ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate) to cover scripts/trackfw-credential-guard.sh (project scope) alongside the two attention scripts — Node.js's CREDENTIAL_GUARD_SCRIPT composition line reordered (CG_PROJECT_GUARD and CG_DETECTION_CORE swapped, no CG_* block content touched) so the script actually emitted by \`discover --init\` diverges from Go/Python while the pre-existing Go-only TestCredentialGuardScript_ParityAcrossStacks (which reconstructs the script by regex-scraping and Go-hardcoded-order-concatenating the CG_*/_CG_* literals, never executing Node/Python) stays green — proves the shell gate closes a real coverage gap the structural unit test cannot see (48), \`trackfw validate\`'s credential_guard_script_integrity rule (ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A/ML-2A) — scripts/trackfw-credential-guard.sh diverging from the template this trackfw binary would generate (via a real, isolated \`discover --init\` run, then a single tampered line appended) must be flagged with \`rules: credential_guard_script_integrity: error\` fixed in the fixture (default severity is warning, which does not flip validate's exit code), and must stay silent when the script is byte-identical to that binary's own template — detection arm asserts the exact validator diagnostic literal, proved non-vacuous via assert_would_now_fail (same exit!=0-and-message-present criterion as assert_fails_with, required to NOT hold against a config-only \`rules: ...: off\` neutering of the same corrupted fixture) rather than a message-absence-only check, single-delta design isolates the corruption (baseline vs. detection) and the severity override (detection vs. non-vacuity) as the only variables, applyRuleTagged/--json path left uncovered same as Scenario 47 (49), \`trackfw validate\`'s credential_guard_mode_downgrade rule (ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A/ML-2A) — credential_guard.mode: block committed at git HEAD followed by an uncommitted on-disk downgrade to mode: warn must be flagged (first check-gates-falsify.sh scenario to git-init/commit a real fixture repo, closing the gap Apolo found — no prior fixture had a HEAD for this rule to anchor against), and must stay silent when disk matches HEAD — non-vacuity mechanism REPLACED by ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard/ML-2A (ADR Emenda 2): the old \`rules: ...: off\` uncommitted neutering stopped proving anything once M4 anchored severity at HEAD, so it now commits \`rules: credential_guard_mode_downgrade: off\` TOGETHER with mode: block at HEAD (the ADR's legitimate-committed-disable path) instead, single-delta design isolates the uncommitted downgrade (baseline vs. detection) and the committed-off HEAD (detection vs. non-vacuity) as the only variables, applyRuleTagged/--json path left uncovered same as Scenario 47 (50), the M4 mechanism itself (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1A/ML-2A) — the decisive scenario: the COMBINED uncommitted edit (\`credential_guard.mode: warn\` + \`rules: credential_guard_mode_downgrade: off\`, both disk-only, HEAD only ever committing mode: block) must still be reported, self-discriminating against a contrast fixture where the SAME disk-side attack is applied but \`rules: ...: off\` is committed at HEAD alongside mode: block (legitimate, auditable) and is silenced — isolating commit-status of the off as the only variable; non-vacuity proved by temporarily reverting credentialGuardRuleSeverity to disk-only resolution (pre-ADR behavior), rebuilding bin/trackfw, confirming the detection arm goes red, then restoring and rebuilding (51), the .trackfw-baseline.json carve-out (Barreira B0/ML-1A/ML-2A) — a credential-guard violation listed in .trackfw-baseline.json by its full literal message continues to be reported, verified against the real BaselineFile{Violations,Warnings} shape and exact-message-match semantics in filterBaselineTagged (validator.go) rather than assumed from ADR prose, self-discriminating within a single fixture/single \`validate\` run: a filename_uniqueness (non-guard) violation listed in the SAME baseline by the same mechanism IS suppressed, proving the carve-out is specific to the 3 guard rules rather than the baseline format being broken outright (which would make the guard violation \"surviving\" prove nothing) — non-vacuity proved by temporarily dropping the \`&& !credentialGuardAnchoredRules[v.Rule]\` guard in filterBaselineTagged, rebuilding, confirming both violations get suppressed, then restoring and rebuilding (52), non-regression for non-guard rules (the most important scenario for confidence in M4, closing the \"blast radius\" question) — filename_uniqueness (not in credentialGuardAnchoredRules, default severity error) with \`rules: filename_uniqueness: off\` set disk-only and never committed continues to fully silence the rule exactly as before this ADR, proving diskRuleSeverity's disk-only path for the other ~38 rules received zero delta from M4; fixture carries a real git HEAD (committing a trackfw.yaml with no rules: block) specifically so the non-vacuity proof is meaningful — without a HEAD, credentialGuardRuleSeverity would fall back to disk-only regardless of anchoring, masking exactly the scope-leak this scenario exists to catch; non-vacuity proved by temporarily adding filename_uniqueness to credentialGuardAnchoredRules (simulating an M4 scope leak), rebuilding, confirming the silenced arm goes red (HEAD's absent rules: entry now resolves to the stricter default and wins over disk's off), then restoring and rebuilding (53), the GIT_* environment-variable bypass of the M4 anchoring (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1B/ML-2B) stays closed against both vectors found by ML-3B — GIT_DIR/GIT_WORK_TREE redirection to a decoy repository without a committed trackfw.yaml, and GIT_CONFIG_COUNT=abc failure-induction (unrelated to redirection, just makes the git subprocess exit 128) — each embedded with a raw \`git -C\` control proving the vector is a genuine attack (not inert) before testing the trackfw binary, plus a legitimate git-worktree control (worktree add/-C anchoring) proving normal worktree usage is unaffected; non-vacuity proved by temporarily reverting cleanGitEnv() (validator_git_exec.go) to return unfiltered os.Environ(), rebuilding bin/trackfw, confirming both detection arms go silent, then restoring and rebuilding (54), check-unknown-command-parity.sh (ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw, ML-2B) — gate created by ML-2A without a falsification scenario, closing the reported gap: canonical-message text drift via Python's format_unknown_command_error dropping the \` for \"{cmd_path}\"\` suffix, caught by the gate's own \"no-suggestion\" vacuity guard (55, baseline + detection), unknown-command exit code drift via Node's \`process.exit(1)\` changed to \`process.exit(3)\`, caught by assert_three_way's exit-code check (56, baseline + detection), and the \"Did you mean\" suggestion suppressed in Go by neutering formatUnknownCommandError's \`found\` branch (\`found && false\`, keeping \`found\` referenced so \`go build\` still succeeds), caught by the gate's \"with-suggestion\" vacuity guard for the go runtime, rebuilt via an isolated Go binary per Cenário 25/26 convention (57, baseline + detection) — one CLI sabotaged per discriminant, each baseline arm proving the clean cycle passes before its paired detection arm proves the corrupted cycle fails), the global fatal-error handler in the Node and Python entrypoints, PLUS a Go baseline arm locking the third CLI's already-clean behavior against future regression (REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-stack-trace-caminhos-absolutos-e-versao-do-runtime, ML-1A) — Node baseline reproduces the REAL unmanaged-artifact production bug end to end (agents install + manifest/artifact tamper + agents update --force) against the unmodified bin/trackfw and proves stderr carries no stack frame, no npm/src/ install path and no \"Node.js vX\"; Node detection reverts bin/trackfw's parseAsync().catch(reportFatalError) to its pre-fix bare parseAsync() call in an isolated copy against the SAME repro and proves the leak reappears; Go baseline runs the SAME repro against the isolated Go binary already built for Cenários 27+ and proves stderr carries no panic:/goroutine/.go:N line — no detection arm, since no Go code was touched by this ML and there is no own-code regression to prove, only \"nos 3 CLIs\" (REQ AC1/roadmap action 4) to lock; Python baseline/detection hold a synthetic corrupted commands/roadmap.py:_cmd_list (unconditional raise, since the REQ found no Python path leaks today) IDENTICAL across both arms and vary only cli.py's try/except around args.func — present it prints \"trackfw roadmap: ...\" with no Traceback, reverted to the pre-fix bare call it prints a full Traceback (58), check-serve-address-parity.sh (ROADMAP-2026-08-16-serve-amarra-em-loopback-por-padrao-com-opt-in-explicito-para-exposicao, ML-1C) — gate created by ML-1C without a falsification scenario, closing the reported gap: pypi/trackfw/commands/serve.py's server_cls((host, port), ...) reverted to server_cls((\"\", port), ...), reintroducing the exact wildcard-bind regression this REQ exists to fix, caught by the gate's own default-bind/py assertion (\"expected lsof to show 127.0.0.1:...\") — baseline arm proves the clean cycle passes across all 4 sub-checks (default loopback bind, ::1 bind, wildcard exposure warning, printed URL) before the paired detection arm proves the corrupted cycle fails (59, git-branch-guard: brecha de contorno via 'git switch -c' (60, baseline + deteccao) e falso-positivo por prosa em linha de mensagem de commit (61, baseline + deteccao) — ML-1A da ROADMAP-2026-08-16-higiene-sete-debitos, prefixo env/command antes de git e flag do checkout -b fora da primeira posicao de token (62, baseline + deteccao para cada sub-caso) — ML-4B corretivo do veredito BLOQUEAR do hades-tf, mesma ROADMAP, git branch <nome>/-c/-C/-m/-M, git worktree add -b, e env CHAVE=valor git ... (63, baseline + deteccao + auto-discriminacao para cada um dos 3 sub-casos) — ML-4C corretivo da reverificacao do hades-tf apos levantar o bloqueio, mesma ROADMAP, guard vira no-op fora de projeto trackfw (64, baseline sem trackfw.yaml + baseline com trackfw.yaml/reverse-vacuity + deteccao + auto-discriminacao dentro de projeto) — ML-1A da ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao, pre-requisito para cabear o guard em escopo global sem quebrar git commit/push em toda a maquina, guard drena o stdin ANTES do no-op para o escritor externo nao receber EPIPE (65, baseline com payload normal + baseline com payload grande >64KB estourando o buffer do pipe + deteccao corrompendo o literal isolado do dreno de stdin, escritor real via subprocesso python3 nao via here-string do bash, non-vacuity provada isolando o EPIPE apenas no braco de deteccao) — ML-1B corretivo da auditoria do arquiteto que reprovou o ML-1A, mesma ROADMAP), check-harness-hooks-parity.sh estendido para o par git-branch-guard global (66, Python muda o matcher de trackfw-git-branch-guard-global-post no arquivo dedicado do Kiro, gate reprova sob o label novo harness-hooks-parity/kiro/git-branch-guard/go-vs-py, e prova de independencia: o label original harness-hooks-parity/kiro/go-vs-py (credential-guard) continua passando na MESMA arvore corrompida) — ML-2A da mesma ROADMAP-2026-08-17, fiacao global do git-branch-guard nos mesmos 6 CLIs do credential-guard), dedup projeto+global do git-branch-guard via \`trackfw discover --init\` real contra um \$HOME sintetico (67, baseline prova que a fiacao global instalada suprime a entrada de projeto do git-branch-guard sem afetar a de credential-guard + reverse-vacuity com \$HOME vazio prova que a ausencia veio da fiacao global, nao de uma quebra geral + deteccao neutraliza globalGitBranchGuardInstalledClaude para 'return false' incondicional numa copia isolada e prova que a entrada de projeto REAPARECE, reproduzindo o sintoma de mensagem duplicada + braco 4 (ML-2C) prova que a comparacao normalizada tolera \"//\" no comando gravado no config global, HOME sintetico deliberadamente com barra dupla embutida) — ML-2B/ML-2C da mesma ROADMAP-2026-08-17, global-scope git_branch_guard_script_integrity/credential_guard_script_integrity trigger on ARTIFACT EXISTENCE at ~/.trackfw/scripts/, not on config wiring (68, baseline com script global integro e ZERO fiacao + ausencia do script nunca instalado + deteccao do script corrompido sem NENHUM config referenciando-o (discriminante central) + prova de nao-vacuidade via rules:...off + nao-duplicacao com o MESMO script referenciado por 2 CLIs (Claude+Codex) para git-branch-guard e, nao-regressao, para credential-guard, contando ocorrencias exatas da mensagem no output real) — ML-3A da mesma ROADMAP-2026-08-17, git_branch_guard_hook_resolvable em escopo GLOBAL passa a inspecionar o arquivo dedicado do Kiro (~/.kiro/hooks/trackfw-git-branch-guard.json), nao so trackfw-credential-guard.json (69, baseline com os dois arquivos dedicados do Kiro integros + deteccao removendo o script referenciado pelo arquivo dedicado do git-branch-guard, mensagem citando o arquivo e o CLI Kiro + nao-duplicacao (exatamente 1 ocorrencia) + nao-regressao (credential-guard do Kiro, arquivo separado e intacto, permanece em silencio)) — ML-3B da mesma ROADMAP-2026-08-17, check-doctor-parity.sh (ROADMAP-2026-08-18-doctor-detecta-artefato-fora-do-manifesto-e-inverte-a-ordem-de-persistencia, ML-2B) — gate created by this ML without its own falsification scenario, closing the reported gap by targeting the exact near-miss the ML-2A audit trail flagged before shipping: ClassifyDoctor's \`!inspection.Registered\` discriminant (internal/integrations/doctor.go) reverted to \`!inspection.Managed\` in an isolated Go binary, so a destination legitimately registered under a DIFFERENT claim (Managed=false, Registered=true, State=Current) is misreported as an unregistered-write false positive, caught end to end through the real \`doctor\` command by the gate's own scenario (e) \"registered-under-different-claim\" fixture — baseline arm proves check-doctor-parity.sh passes clean against the unmodified Go binary before the paired detection arm proves the single-literal corruption makes it fail (71), ML-2C's unknown-content class (ROADMAP-2026-08-18-doctor-detecta-artefato-fora-do-manifesto-e-inverte-a-ordem-de-persistencia, ML-2C) — the analogous near-miss for the third case ClassifyDoctor gained to close the ML-3A audit finding (docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md): \`!inspection.Registered\` reverted to \`!inspection.Managed\` in the unknown-content case-clause, caught end to end by check-doctor-parity.sh's NEW scenario (f) \"registered-under-different-claim-content-drifted\" fixture (retargeted claim item + a byte appended to on-disk content, so State=Modified where scenario (e) alone stays State=Current and cannot discriminate this corruption) — baseline arm proves check-doctor-parity.sh passes clean against the unmodified Go binary before the paired detection arm proves the single-literal corruption makes it fail (72), check-ship-force-parity.sh (ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md, ML-1B) — gate created by this ML without its own falsification scenario, closing the reported gap: internal/commands/ship.go's single \`\"--force-with-lease\"\` push-arg literal reverted to raw \`\"--force\"\` in an isolated Go binary, caught end to end through the real \`ship\` command by the gate's own scenario (e) \"remote-advanced-lease-mismatch\" fixture — a second clone pushes a legitimate commit to the same branch while our clone's remote-tracking ref is pinned stale on purpose, so the correct flag refuses (stale lease) and the sabotaged raw --force pushes through and destroys the other party's commit; the semantic outcome (push exit code + commit survival on the remote), not argv string-inspection, is what discriminates — baseline arm proves check-ship-force-parity.sh passes clean against the unmodified Go binary before the paired detection arm proves the single-literal corruption makes it fail (73), git-branch-guard extended to the destructive working-tree class (stash/reset --hard/clean -f|-x/restore <path>/checkout -- <path>|checkout .) per REQ-2026-08-19-guard-nao-bloqueia-comandos-destrutivos-de-working-tree-em-repo-compartilhado-por-agentes.md ML-3A -- one baseline+detection pair PER COMMAND, covering BOTH directions the REQ names as risk: the blocked form escaping (case-label corrupted away) and the freed form being wrongly caught (allow-list/discriminant corrupted away or widened to match everything, e.g. --hard turned into a bare wildcard so git reset --soft would wrongly block) -- git reset --soft/--mixed, git stash list/show, git clean -n, git restore --staged and git checkout <branch> all proven free both before and after corruption isolates the exact literal each depends on (74), check-release-tag-parity.sh (ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md, ML-2B) — gate created by this ML without its own falsification scenario, closing the reported gap: internal/commands/release.go's single \`SHA: tagObj.SHA\` ref-payload literal reverted to \`SHA: objectSHA\` in an isolated Go binary, degrading the published tag from annotated to lightweight (the ref points straight at the commit instead of at the tag object the first gh api call created) — caught end to end through the real \`release tag\` command by the gate's own \"success\" fixture, whose gh stub deliberately returns a tag-object sha different from the commit sha so the ref payload's sha field discriminates the two outcomes; baseline arm proves check-release-tag-parity.sh passes clean against the unmodified Go binary before the paired detection arm proves the single-literal corruption makes it fail (75), the commit-target divergence check itself (ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md, Emenda 1, ML-4B corretivo do veredito BLOQUEAR do hades-tf, mesma ROADMAP) — internal/commands/release.go's single \`if forgeLocalSHA != \\\"\\\" && forgeLocalSHA != commitObj.SHA {\` guard neutered with a \`false &&\` prefix in an isolated Go binary, silently reverting the commit-target back to trusting a local ref instead of refusing on divergence from the forge, caught end to end through the real \`release tag\` command by the gate's own Scenario 12 fixture (origin/main forged via \`git update-ref\` under a narrowed remote.origin.fetch refspec, refs/heads/main reset to match so the pre-existing local-branch-staleness check cannot discriminate this corruption either) — baseline arm proves check-release-tag-parity.sh passes clean against the unmodified Go binary before the paired detection arm proves the single-literal corruption makes it fail (76), scripts/check-parity-contract-coverage.sh (ROADMAP-2026-08-20-contrato-pinado-no-cli-parity-sem-gate-nomeado.md, ML-1B) — gate created by this ML without its own falsification scenario, closing the reported gap: baseline fixture covering all 3 heading levels (##/###/####) and all 4 valid trackfw-contract annotation shapes (gate=, gate=+partial=, gap reason=, none reason=) proves the counts are honest, then 6 single-delta corruptions off that same baseline prove each documented failure class (gate= empty, gate= naming a path missing on disk, gap without reason=, none without reason=, unknown key reson= per the ADR Emenda 1 parsing note, and a malformed trackfw-contract prefix naming no recognized state) each reprove with the exact diagnostic literal, plus non-vacuity neutering the gate-existence check on an isolated copy of the checker to prove the gate-missing-on-disk detection arm actually depends on it (77), the ADR's Emenda 2 general rule (ROADMAP-2026-08-20-contrato-pinado-no-cli-parity-sem-gate-nomeado.md, ML-1B-bis) — 'toda chave presente exige valor não-vazio' implemented as a single loop over every key present in the parsed annotation (not a per-key if) that replaced the old 77b-only gate=-empty special case, proved single-delta against the same baseline by THREE arms exercising keys that never had a dedicated empty check before this ML: gate=scripts/check-cli-parity.sh partial= (77i, the 'gate= com partial=' cell silently under-counting a section with an undeclared gap is exactly the failure Emenda 2 exists to close), gap reason= (77j, distinct from the pre-existing 77d which tests the key never being written at all — here the key IS written and empty, a case that scenario structurally cannot reach since its fixture omits the key), and none reason= (77k, same distinction against 77e, isolating that the shared state in (\"gap\", \"none\") branch enforces the rule for BOTH states, not just gap) — each asserting the generic 'CHAVE= presente com valor vazio' diagnostic naming the empty key rather than a bespoke per-key message, plus non-vacuity (77l) neutering the general empty-value loop itself (not a specific gate=/reason= check, since those were removed as dead code by this ML) on an isolated copy and proving 77i's detection goes silent, isolating that the LOOP is what's load-bearing (142), the ADR Nota de parsing's positional gap this ML closes (ROADMAP-2026-08-20-contrato-pinado-no-cli-parity-sem-gate-nomeado.md, ML-1B-ter) — chave desconhecida na anotação agora reprova em QUALQUER posição, não só antes da primeira chave real: find_unknown_key_typos() varre o corpo inteiro (inclusive dentro do valor já fatiado de reason=/partial=) por token alfabético minúsculo imediatamente seguido de '=' a distância de edição <=1 de gate/partial/reason, proved by 77m ('reson=' escrito DEPOIS de um reason= real, o caso que 77f não alcança já que o typo de 77f fica antes de qualquer chave real e é pego pelo \`leading\` antigo), non-regressed against the heuristic's own false-positive risk by 77n (LANG=pt_BR e --flag=valor dentro de um reason= legítimo não podem reprovar — chave maiúscula e flag precedida de '-' nunca batem no formato tocado-por-espaço-minúsculo que o heurístico procura), and proved non-vacuous by 77o neutering find_unknown_key_typos()'s call site to an empty set in an isolated copy and confirming 77m goes silent (145), the ML-3A blocking-mode flip itself (ROADMAP-2026-08-20-contrato-pinado-no-cli-parity-sem-gate-nomeado.md, ML-3A) -- the Wave 2 triage having closed 177/177, an unannotated section now reproves instead of only being counted -- proved single-delta (77p) by appending ONE extra unannotated heading on top of the same 77a baseline (whose 4 fully-annotated sections alone still pass, isolating that the new heading's absent annotation is the only variable) and asserting the exact 'seção sem anotação trackfw-contract' diagnostic (146), check-agent-hooks-parity.sh extended to windsurf and amazonq (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md, ML-1B) — the generic structural comparator needed zero changes (CLIS/marker_for/hookfile_for gained two entries each, and the credential-guard vacuity guard's marker string is now per-CLI since windsurf/amazonq only wire git-branch-guard, not credential-guard) — Node.js's injectAmazonQHooks default \`tools: ['*']\` corrupted to \`tools: ['read']\` in an isolated copy of npm/, caught end to end through the real \`discover --init\` entry points by the gate's own structural diff at \$.tools[0], proving the extension actually exercises cross-stack comparison for these two CLIs and not just vacuous same-marker equality (147), check-validate-parity.sh's branch_has_wip_roadmap done/ acceptance block (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md, ML-2A) — the rule accepting a roadmap in done/ (not just wip/) since REQ-2026-07-26 had never been exercised cross-CLI (check-branch-new-parity.sh's fixtures literally said \"wip/ and done/ deliberately left empty\", check-validate-parity.sh had zero occurrences of the rule) — closed via TRACKFW_BRANCH (supported identically by the 3 CLIs, no real git checkout needed) across 3 cases: roadmap in done/ with matching slug accepted (the untested central case), no roadmap anywhere still blocks (non-regression), and roadmap in done/ with a DIFFERENT slug still blocks (the discriminant that keeps the gate from accidentally passing for any roadmap in done/) — while assembling the fixture, found and PINNED (not fixed) a genuine pre-existing Python-only divergence: pypi/trackfw/validator.py's validate_branch_has_wip_roadmap returns plain strings instead of the {\"message\":...} dict shape _enrich_items expects, so Python's validate --json tags this one rule with \"rule\": null/\"file\": null while Go/Node.js correctly tag \"branch_has_wip_roadmap\" — message text stays byte-identical across all 3, so the gate filters by message substring and separately asserts the rule-tag divergence explicitly so it cannot silently drift further; GO_BIN override added to check-validate-parity.sh (previously always self-built, unlike every sibling check-*-parity.sh script) to make P4 possible without a full script copy — proved by Cenário 79, single-literal corruption of internal/validator/validator.go's BranchSlugMatchesRoadmap dropping doneDirs from the scanned directory set, caught end to end through the real check-validate-parity.sh pointed at the isolated sabotaged Go binary via GO_BIN, while Node.js/Python stay real and correct (148, baseline + detecção), check-validate-parity.sh's credential_guard_hook_resolvable cross-CLI block (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md, ML-3A) — the rule exercised end-to-end in all 3 CLIs for the first time (Cenário 47 only covered Go), via 4 fixture cases: claude-absent (detection: .claude/settings.json with \$CLAUDE_PROJECT_DIR/… and script absent fires in all 3), claude-present (baseline: same hook with script present stays silent), cursor-absent (relative-path branch live — discriminant that prevents a vacuous ok=false-for-all-relatives implementation from passing the cursor-present arm alone), cursor-present (false-positive guard: legitimate Cursor relative path not accused when script is present) — proved non-vacuous by Cenário 80, single-literal corruption of credentialGuardScriptMarker in validator_credential_guard.go from \"trackfw-credential-guard.sh\" to \"trackfw-credential-guard-DISABLED.sh\", making Go blind to all credential-guard hook entries while Node.js/Python stay real and correct; the 4-case block correctly detects the divergence at the claude-absent/go vacuity check (149, baseline + detecção), check-validate-parity.sh's credential_guard_hook_resolvable exec-bit detection (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md, ML-4B/A-1) — new cg-claude-noexec case: script present but chmod 644 must fire \"not executable\" in all 3 CLIs; proved non-vacuous by Cenário 81, single-literal \`false &&\` prefix to \`case info.Mode()&0111 == 0:\` in validator_credential_guard.go, making the exec-bit case unreachable in Go (no false-positive on cg-claude-present, discriminates only the non-executable path) while Node.js/Python stay real and correct (150, baseline + detecção), check-validate-parity.sh's credential_guard_hook_resolvable missing-type detection (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md, ML-4B/A-1) — new cg-claude-notype case: hook without \"type\":\"command\" must fire 'missing \"type\":\"command\"' in all 3 CLIs (check fires BEFORE existence check, per ROADMAP-2026-08-17 ML-4B); proved non-vacuous by Cenário 82, single-literal \`false &&\` prefix to \`if hf.requiresCommandType && !m.typeIsCommand {\` in validator_credential_guard.go, making the type check unreachable in Go (no false-positive on cg-claude-present or cg-claude-absent, discriminates only the notype path) while Node.js/Python stay real and correct (151, baseline + detecção), check-agent-hooks-parity.sh's deniedCommands P3 vacuity guard (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md, ML-4B/B-1) — a correlated drop of Amazon Q deniedCommands from all 3 stacks passes compare_json (both sides write the key with the same wrong value) and the P2 guard (git-branch-guard script string still present); the new P3 guard catches it via grep -F for the exact deny pattern '^git (commit|push|checkout -b)'; proved non-vacuous by Cenário 83, tri-stack sabotage replacing gitDenyPattern/GBG_DENIED_COMMANDS_PATTERN/_GIT_GUARD_DENIED_COMMANDS_PATTERN with \'DENIED_COMMANDS_REMOVED\' in isolated Go/npm/pypi copies (NODE_CLI via T83/scripts/ ROOT_DIR, PY_ROOT override, GO_BIN override), all 3 stacks write deniedCommands with the wrong literal, compare_json passes, P3 guard fires at agent-hooks-parity/amazonq/go/denied-commands-present (152, baseline + detecção), check-artifact-parity.sh's CLAUDE.md ## Architect responses vacuity guard (ML-1A, ROADMAP-2026-08-21-regra-de-verbosidade-no-asset-do-arquiteto-e-nas-regras-semeadas) — Node.js's init.js section header corrupted from '## Architect responses' to '## VERBOSITY_SECTION_REMOVED' in an isolated npm copy (setup_npm_tree + corrupt_literal, script copied to T84/scripts/ so ROOT_DIR resolves to T84 and node uses the corrupted init.js while GO_BIN stays real via env override and Python uses a real pypi copy); awk extraction finds no '## Architect responses' heading in the Node.js CLAUDE.md → vacuity guard fires at 'CLAUDE.md ## Architect responses missing or empty (node)'; baseline arm proves check-artifact-parity.sh passes clean against unmodified runtimes before the paired detection arm proves the corrupted cycle fails (153, baseline + detecção), nil map em ProjectConfig.AgentModels: initConfigMaps(cfg) removido de parse() em config.go restaura o nil map — ParseRulesFromContent cria ProjectConfig{Rules: make(...)} sem inicializar AgentModels; parse() escreve cfg.AgentModels[k] = s → panic 'assignment to entry in nil map'; o fix usa reflexão para inicializar todos os campos de mapa de ProjectConfig antes de qualquer escrita, tornando parse() seguro independentemente da construção do caller; provado por trackfw validate em fixture git com agent_models: em HEAD que chega a ParseRulesFromContent via credentialGuardRuleSeverity, usando binário com initConfigMaps(cfg) comentado de parse() (155, baseline + detecção), namespace leak via remoção da guarda \`targetID == \"claude\" && len(agentModels) > 0\` de internal/integrations/render.go — targets que usam o case default: do switch de representação (ex.: Gemini via \"agent-markdown\") recebem model ID composto (\"claude-sonnet-4-6\") em vez do alias canônico (\"sonnet\") quando agent_models está configurado; check-agent-models-parity.sh reprova com 'namespace leak'; braco de baseline prova que o binário real passa antes da detecção; seam: literal isolado em cópia de árvore Go compilada em $T86 (156, baseline + detecção), check-release-tag-parity.sh content-anchorage bypass via readCommittedFile(objectSHA) → readCommittedFile(\"HEAD\") on the CHANGELOG read (ROADMAP-2026-08-21-release-tag-ancora-versao-e-mensagem-no-forge, ML-2B) — readFile was REMOVED from releaseDeps struct (ML-2A) so no fallback to working-tree disk read can compile; the only viable regression is passing a different sha argument to readCommittedFile; \"HEAD\" compiles and resolves to the local tip; P3 still reads version files from objectSHA (9.9.9 ✓); P4 reads CHANGELOG from HEAD which ALSO has ## [9.9.9] section → still passes → exit 0; but message = \"head-only\" (HEAD's CHANGELOG body), not \"forge-only\" (forge commit's CHANGELOG body) → Scenario 16's provenance assertion fires: 'provenance: tag message must contain forge-only'; two-axis fixture (HEAD at 9.9.7/head-only, decoy at 9.9.9/forge-only) makes both anchored reads independently falsifiable; baseline arm proves check-release-tag-parity.sh passes clean with real binary; detection arm proves corrupted binary makes it fail with the exact provenance message; seam: literal isolated in corrupt-go copy in $T87 (157, baseline + detecção), check-release-tag-parity.sh refs-replace-bypass detection arm (ROADMAP-2026-08-21-release-tag-ancora-versao-e-mensagem-no-forge, ML-4A) — internal/commands/release.go's single '\"--no-replace-objects\", \"show\"' literal removed (reverting git show to follow refs/replace/ object-identity redirect); git show \$FORGE_SHA:CHANGELOG.md follows the replace ref written by the attacker as a raw file write and returns 'refs-replace-forged' instead of 'forge-only'; P3 still reads version files from objectSHA (9.9.9 matches) → exit 0; but message = 'refs-replace-forged' → Scenario 17's per-runtime provenance assertion fires: 'provenance: tag message must contain forge-only'; three-axis fixture (HEAD at 9.9.7/head-only, forge commit at 9.9.9/forge-only, attacker commit LOCAL-ONLY at 9.9.9/refs-replace-forged with replace ref as file write) independently falsifiable per runtime; assert_three_way catches single-stack revert; baseline arm proves check-release-tag-parity.sh passes clean with real binary before paired detection arm proves corrupted binary makes it fail; seam isolated in corrupt-go copy in $T88 (158, baseline + detecção), check-validate-parity.sh credential_guard_hook_resolvable bare-relative-suppression (ROADMAP-2026-08-21 ML-2A, RETARGETED 2026-08-22 ML-2A para classifyHookAnchorage — seam deslocado de isRelativePureForGuard para a cláusula bare-relative da classe 2: !strings.HasPrefix(rawStripped, \"$\") → false; bare-relative cai na classe 3 e é silenciado em Go; \$PWD/… permanece acusado via cláusula de prefixo anterior) (159, baseline + detecção), check-validate-parity.sh credential_guard_hook_resolvable \$PWD-suppression direção-A (ROADMAP-2026-08-22 ML-2A): prefixo \"\$PWD/\" → \"\$PWD_DEAD/\" faz \$PWD/… cair na classe 3 em Go enquanto Node.js/Python ficam reais; P2 vacuity guard reprova no caso cg-claude-pwd (164, baseline + detecção), check-validate-parity.sh credential_guard_hook_resolvable absolute-path-accused direção-B (ROADMAP-2026-08-22 ML-2A): filepath.IsAbs(rawStripped) → false nas linhas 105 e 112 faz caminhos absolutos cair na classe 2 e serem acusados em Go; o case cg-claude-absoluto (expect silence) reporta mensagem inesperada; protege o defeito caro desta entrega (165, baseline + detecção) — synced Wave 0 removal across the 3 roadmap generators, caught by check-artifact-parity.sh expected-content assertion, not by cross-stack diff (166, baseline + detection), and barrier lower-bound regression on --wave 0 reintroducing the pre-ML-1A rejection, caught by the inverted Scenario 11 of check-barrier.sh (167, baseline + detection) and Direction B's second guard — the flag-level validation in newBarrierCmd (waveInt), distinct from parseWaves — reverted independently, also caught by the same inverted Scenario 11 assertion (168, detection only, baseline shared with 167), check-audit-surface.sh falsificado nas duas direcoes exigidas por AC9: digest constante via sha256.Sum256(nil) em auditsurface.go faz FN-2 ver mesmo digest entre dois refs onde o script diferiu — gate reprova em audit-surface/fn-2/digest-changes-when-script-changes (173, baseline + deteccao via AUDIT_SURFACE_SELFTEST_BREAK=A), e caminho de instrucao docs/cli-parity.md inserido em instructionFilePaths faz FP-1 encontrar o arquivo no output onde nao deveria aparecer — gate reprova em audit-surface/fp-1/cli-parity-absent (174, baseline compartilhada com 173 + deteccao via AUDIT_SURFACE_SELFTEST_BREAK=B), check-doctor-parity.sh's scaffold findings in both directions (ML-2A, ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template): checkScaffoldArtifact silenced by replacing bytes.Equal(actual, expected) with bytes.Equal(actual, actual) in an isolated Go binary (always equal, never reports divergence), caught by the gate's own scenario (h) scaffold-attention-signal-divergent vacuity guard (177, baseline + detecção), and checkScaffoldArtifact's equal guard inverted to !bytes.Equal(actual, expected) so intact scaffold files are falsely reported as scaffold-divergent, caught by the gate's own scenario (g) scaffold-baseline-clean vacuity guard (178, baseline shared with 177 + detecção), check-doctor-parity.sh's execute-bit check in three directions (ML-2A, ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-dos-artefatos-de-scaffold): Direction A — execBit && silenced via execBit && → false && in checkScaffoldArtifact (scaffold_doctor.go:324) so the mode check never fires, the binary reports no wrong-mode finding even when scripts/trackfw-validate.sh is at 0644, caught by check-doctor-parity.sh's scenario (p) scaffold-wrong-mode-detected vacuity guard (179, baseline + detecção), Direction B — execBit discriminant silenced via execBit && → true && so checkScaffoldArtifact checks the execute bit on every artifact regardless of descriptor, producing false scaffold-wrong-mode findings on slash commands (.claude/commands/trackfw/*.md, 0644) that carry execBit=false; the gate runs Go only (not 3-way parity because Python emits an extra progress line for the slash commands directory) against a fixture built with --targets validate-script,agent-hooks,claude-commands (180, baseline + detecção), Direction C — os.Chmod removed from generateValidateScript (scaffold.go AC9): the sabotaged update rewrites the content (apply() still runs via runFileTarget for existing files, proved by cmp -s) but does not restore the execute bit; baseline arm proves the real binary restores the bit after chmod 0644; detection arm proves the sabotaged binary restores only the content (bit still absent after update), isolating the Chmod call as the load-bearing step (181, baseline + detecção)\""
