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
  ', date, content.REQPath, content.Title, date, content.REQPath, content.Title)' \
  ', date, "", content.Title, date, content.REQPath, content.Title)' \
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

S29_EXPECTED=$'\xe2\x9c\x93 No violations found.\n'

T29_PROJECT="$WORK/s29-clean-project"
scaffold_adr_req_project "$T29_PROJECT"

s29_go_out=$(cd "$T29_PROJECT" && "$T27_GO_BIN" validate)$'\n'
s29_node_out=$(cd "$T29_PROJECT" && node "$ROOT_DIR/npm/bin/trackfw" validate)$'\n'
s29_python_out=$(cd "$T29_PROJECT" && env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate)$'\n'

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

s29c_python_out=$(cd "$T29_PROJECT" && env PYTHONPATH="$T29C_P/pypi" python3 -m trackfw validate)$'\n'
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

# --- braço de detecção: Python reverte _list_dirs para ordem determinística
# invertida (ver nota acima sobre por que não usar os.listdir cru) --------
T33C_PY="$WORK/s33-corrupt-python"
mkdir -p "$T33C_PY"
cp -r "$ROOT_DIR/pypi" "$T33C_PY/pypi"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/commands/status.py" "$T33C_PY/pypi/trackfw/commands/status.py" \
  '            name for name in sorted(os.listdir(path))
' \
  '            name for name in sorted(os.listdir(path), reverse=True)
' \
  "s33-python"

s33c_python_out=$(cd "$S33_PROJECT" && env PYTHONPATH="$T33C_PY/pypi" python3 -m trackfw status)$'\n'
if [[ "$s33c_python_out" == "$S33_EXPECTED" ]]; then
  echo "FAIL [falsify/status-by-agent-fallback-order/python-detects-order-regression]: _list_dirs revertido para ordem invertida mas a comparação continuou passando (checagem vácua)" >&2
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
# configurando SÓ `apolo`, enquanto `docs/roadmaps/` no disco tem `apolo`
# **e** `zeus` (zeus com roadmap em wip/). Os cenários existentes (ex. 31)
# usam forma indentada — não exercitam o defeito. Se a fixture configurasse
# o MESMO conjunto de agentes que existe no disco, reverter o parser cairia
# no fallback e reproduziria a MESMA saída (fallback enumera exatamente
# [apolo, zeus] = o que o parser corrigido já devolvia) — o braço de
# detecção seria vácuo. Com `agents: [apolo]` configurado e `zeus/` extra no
# disco, a divergência é inequívoca: parser correto → só apolo aparece
# (zeus invisível); parser quebrado → cai no fallback → zeus reaparece.
#
#   - baseline: os 3 CLIs, contra o literal PINADO (capturado rodando os 3
#     CLIs reais contra a fixture), byte-idênticos — só [apolo] aparece,
#     Inventory Roadmaps total 1 (wip 1); zeus invisível nos 3.
#   - detecção: Go e Node revertem o ponto exato alterado por d208971
#     (continuesOpenList sempre false — a lista não-indentada volta a ser
#     descartada) e provam, por asserção POSITIVA, que zeus reaparece na
#     saída corrompida.
#
# Corrompe a IMPLEMENTAÇÃO (o ponto exato revertido por d208971), nunca a
# asserção — mesmo padrão dos Cenários 14/16/17/20/21/24/25/26/27/28/29/30/
# 31/32/33. Não toca em pypi/ — o Python já está correto (item fora do
# escopo negativo deste roadmap).
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
- apolo
EOF
write_roadmap_state_fixture "$S34_PROJECT/docs/roadmaps/zeus/wip/ROADMAP-zeus-wip.md" "wip" "zeus wip fixture (fora da lista configurada)"
write_roadmap_state_fixture "$S34_PROJECT/docs/roadmaps/apolo/wip/ROADMAP-apolo-wip.md" "wip" "apolo wip fixture"

S34_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    1\n     backlog 0 · analyzing 0 · wip 1\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [apolo] WIP (1)\n    ROADMAP-apolo-wip.md\n\n────────────────────────────────────────\n'

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

s34c_go_out=$(cd "$S34_PROJECT" && "$T34C_GO_BIN" status)$'\n'
if [[ "$s34c_go_out" == "$S34_EXPECTED" ]]; then
  echo "FAIL [falsify/config-unindented-agents/go-detects-list-discarded]: cfg.Agents deixou de ser atribuido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "[zeus]" <<<"$s34c_go_out"; then
  echo "OK   [falsify/config-unindented-agents/go-detects-list-discarded]"
else
  echo "FAIL [falsify/config-unindented-agents/go-detects-list-discarded]: saída corrompida diverge do pinado, mas zeus não reapareceu — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s34c_go_out")" >&2
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

s34c_node_out=$(cd "$S34_PROJECT" && node "$T34C_N/npm/bin/trackfw" status)$'\n'
if [[ "$s34c_node_out" == "$S34_EXPECTED" ]]; then
  echo "FAIL [falsify/config-unindented-agents/node-detects-list-discarded]: cfg.agents deixou de ser atribuido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "[zeus]" <<<"$s34c_node_out"; then
  echo "OK   [falsify/config-unindented-agents/node-detects-list-discarded]"
else
  echo "FAIL [falsify/config-unindented-agents/node-detects-list-discarded]: saída corrompida diverge do pinado, mas zeus não reapareceu — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s34c_node_out")" >&2
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
agents: ["ka, tsu", "obi"]
EOF
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/ka, tsu/wip/ROADMAP-ka-tsu-wip.md" "wip" "ka tsu wip fixture (caso 8)"
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/zeta/wip/ROADMAP-zeta-wip.md" "wip" "zeta wip fixture (fora da lista configurada)"

S35_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    1\n     backlog 0 · analyzing 0 · wip 1\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [ka, tsu] WIP (1)\n    ROADMAP-ka-tsu-wip.md\n\n────────────────────────────────────────\n'

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

s35c_go_out=$(cd "$S35_PROJECT" && "$T35C_GO_BIN" status)$'\n'
if [[ "$s35c_go_out" == "$S35_EXPECTED" ]]; then
  echo "FAIL [falsify/config-inline-comma-in-quotes/go-detects-agents-discarded]: cfg.Agents deixou de ser atribuido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "[zeta]" <<<"$s35c_go_out"; then
  echo "OK   [falsify/config-inline-comma-in-quotes/go-detects-agents-discarded]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/go-detects-agents-discarded]: saída corrompida diverge do pinado, mas zeta não reapareceu — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s35c_go_out")" >&2
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

s35c_node_out=$(cd "$S35_PROJECT" && node "$T35C_N/npm/bin/trackfw" status)$'\n'
if [[ "$s35c_node_out" == "$S35_EXPECTED" ]]; then
  echo "FAIL [falsify/config-inline-comma-in-quotes/node-detects-agents-discarded]: cfg.agents deixou de ser atribuido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "[zeta]" <<<"$s35c_node_out"; then
  echo "OK   [falsify/config-inline-comma-in-quotes/node-detects-agents-discarded]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/node-detects-agents-discarded]: saída corrompida diverge do pinado, mas zeta não reapareceu — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s35c_node_out")" >&2
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

s35c_python_out=$(cd "$S35_PROJECT" && env PYTHONPATH="$T35C_P/pypi" python3 -m trackfw status)$'\n'
if [[ "$s35c_python_out" == "$S35_EXPECTED" ]]; then
  echo "FAIL [falsify/config-inline-comma-in-quotes/python-detects-agents-discarded]: cfg[\"agents\"] deixou de ser atribuido mas a comparação continuou passando (checagem vácua)" >&2
  exit 1
fi
if grep -qF "[zeta]" <<<"$s35c_python_out"; then
  echo "OK   [falsify/config-inline-comma-in-quotes/python-detects-agents-discarded]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/python-detects-agents-discarded]: saída corrompida diverge do pinado, mas zeta não reapareceu — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s35c_python_out")" >&2
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

echo "Falsification checks passed (all 92 scenarios, 14 gates + 11 generator/validator contracts — roadmap acceptance heading (24), req frontmatter --from-req path (25, baseline + detection) and --req simple path AC2b (26, baseline + detection), adr_accepted_when_req_done + blocked_by_draft_adr (27, baseline + baseline-negative + detection, 2 rules x 3 CLIs), backtick-wrapped ADR reference without frontmatter adr: field (28, baseline + detection, 3 CLIs), validate success message pinned + byte-identical across 3 CLIs (29, baseline + detection), status Inventory block flat mode pinned + byte-identical with analyzing/REQ-status discriminant fixture (30, baseline + Go analyzing-omission detection), status Inventory + WIP by Agent block by_agent mode pinned + byte-identical (31, baseline + Python WIP-by-Agent body-drift detection), unpaired reference delimiter in adr_accepted_when_req_done fixture — Python-only regression (32, baseline 3 CLIs + Python detection), status by_agent fallback order without agents: configured — Python-only regression (33, baseline 3 CLIs pinned + Python detection with positional assertion), config parser unindented block sequence for agents: — Go+Node-only regression (34, baseline 3 CLIs pinned + Go and Node detection with positional assertion, RETARGETED 2026-08-02 for the yaml.v3/yaml-2.x migration — original literal removed by ML-1A), config parser inline list item with comma-inside-quotes for agents: — 3 CLIs regression (35, baseline 3 CLIs pinned + Go/Node/Python detection with positional assertion, RETARGETED 2026-08-02 for the yaml.v3/yaml-2.x migration — original splitTopLevelCommas literal removed by ML-1A), config scalar schema-fidelity (octal/bare-date/yes) via roadmap_dir+req_dir+adr_dirs — normalizeNode typed-scalar regression, each CLI diverges only on the case the ADR predicts (36, baseline 3 CLIs pinned + Go/Node/Python detection each isolating its own discriminant), malformed trackfw.yaml error path — stderr message + exit 1 byte-identical across 3 CLIs (37, baseline 3 CLIs + Go fatal-check-removed detection) — proved non-vacuous, wip_limit quoted-scalar regression via wipConfigFrom/_wip_config_from — validate() bypassing config.Load() with an artisanal trackfw.yaml re-read discriminated only by a quoted \"3\" scalar (38, baseline 3 CLIs pinned + Go/Node/Python detection reintroducing the readWIPConfig pattern eliminated by 74d70ee))"
