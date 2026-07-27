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
# Cenário 4 — check-validate-parity.sh: npm sem regra wip_has_req → contrato diverge
#
# O script compila Go a partir do CWD (ROOT_DIR), usa npm/$T4 e pypi/$T4.
# Remover a chamada applyRule('wip_has_req'…) do npm faz Go e Python reportarem
# wip_has_req mas npm não → "validate JSON contract differs between runtimes".
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
(cd "$T8_MOD" && env GOCACHE="$WORK/go-build-cache" go build -o "$T8_BIN" ./cmd/trackfw) 2>/dev/null

assert_fails_with "artifact-parity/req-name-drift" \
  "arquivo ausente" \
  env GO_BIN="$T8_BIN" bash "$T8/scripts/check-artifact-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 9 — check-referential-integrity.sh: REQ com roadmap quebrado →
#              gate detecta referência inexistente no frontmatter.
#
# Objetivo (P4): provar que o gate de integridade referencial reprova uma
# referência canônica quebrada sem deixar resíduo no workspace real.
# ---------------------------------------------------------------------------
T9="$WORK/s9"
mkdir -p "$T9/scripts" "$T9/docs"
cp "$ROOT_DIR/scripts/check-referential-integrity.sh" "$T9/scripts/"
cp -r "$ROOT_DIR/docs/req" "$T9/docs/req"
cp -r "$ROOT_DIR/docs/roadmaps" "$T9/docs/roadmaps"
cp -r "$ROOT_DIR/docs/adr" "$T9/docs/adr"

# Corromper: quebrar uma referência existente em cópia temporária.
cat > "$T9/docs/req/REQ-adr-wizard-e-list-2026-06-11.md" <<'EOF'
---
status: Done
adr: ""
roadmap: "docs/roadmaps/done/MISSING-roadmap-adr-wizard-e-list-2026-06-11.md"
---

# REQ quebrada para prova P4
EOF

assert_fails_with "referential-integrity/missing-roadmap" \
  "referential integrity failed" \
  bash "$T9/scripts/check-referential-integrity.sh"

echo "Falsification checks passed (all 9 scenarios, 8 gates proved non-vacuous)"
