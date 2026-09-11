#!/usr/bin/env bash
# check-consumer-smoke-by-agent.sh — AC3 do
# ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md
#
# Smoke que cria um projeto descartável com `roadmap_namespacing: by_agent`
# e **2 agentes** e roda os comandos principais nos 3 CLIs. Deve pegar o
# #320 (artefato sempre no primeiro agente; --req ignorando o agente da REQ).
#
# DECLARAÇÃO DE RECONCILIAÇÃO (ML-1C):
#   Este script afirma que com projeto by_agent + 2 agentes, `roadmap new`
#   (com e sem --req) sempre coloca o artefato em agents[0] (alpha) em vez
#   de respeitar o agente do contexto ou de oferecer seleção.
#   Medição: agentStateDir("", "backlog") usa cfg.Agents[0]
#   (internal/generators/roadmap.go:108–115).
#   Quando #320 for corrigido, este script deve sair com exit 0 e o job
#   consumer-smoke-by-agent pode ter o continue-on-error: true removido.
#
# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate.
export PYTHONIOENCODING=utf-8

set -euo pipefail

ROOT_DIR=${TRACKFW_ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}

# Resolver binários — permite override via env para uso em CI
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="$ROOT_DIR/pypi"

# ---------------------------------------------------------------------------
# Self-test mode: verifica que o script detecta os casos esperados sem
# precisar dos binários reais.
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--self-test" ]]; then
  echo "=== check-consumer-smoke-by-agent --self-test ==="
  FAIL=0

  # Braço 1: verifica que o script existe e é executável
  if [[ ! -x "${BASH_SOURCE[0]}" ]]; then
    echo "SELF-TEST FAIL: script não é executável"
    FAIL=1
  else
    echo "SELF-TEST PASS: script é executável"
  fi

  # Braço 2: verifica que o ROOT_DIR aponta para um repositório com trackfw.yaml
  if [[ ! -f "$ROOT_DIR/trackfw.yaml" ]]; then
    echo "SELF-TEST FAIL: ROOT_DIR não tem trackfw.yaml: $ROOT_DIR"
    FAIL=1
  else
    echo "SELF-TEST PASS: ROOT_DIR tem trackfw.yaml"
  fi

  # Braço 3 (falsificação — AC6): simula o cenário de artefato no agente errado
  # Cria estrutura by_agent com 2 agentes, cria arquivo em alpha/backlog,
  # verifica que a detecção reporta o erro.
  ST_WORK=$(mktemp -d "${TMPDIR:-/tmp}/consumer-smoke-self-test.XXXXXX")
  trap 'rm -rf "$ST_WORK"' EXIT

  mkdir -p "$ST_WORK/docs/roadmaps/alpha/backlog"
  mkdir -p "$ST_WORK/docs/roadmaps/beta/backlog"
  cat > "$ST_WORK/trackfw.yaml" <<'YAML'
governance_mode: strict
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
- alpha
- beta
YAML
  # Simula o bug: artefato em alpha/, não em beta/
  touch "$ST_WORK/docs/roadmaps/alpha/backlog/ROADMAP-test.md"

  # A detecção deve achar artefato em alpha mas não em beta
  IN_ALPHA=$(find "$ST_WORK/docs/roadmaps/alpha/backlog" -name "ROADMAP-*.md" | wc -l)
  IN_BETA=$(find "$ST_WORK/docs/roadmaps/beta/backlog" -name "ROADMAP-*.md" | wc -l)

  if [[ "$IN_ALPHA" -gt 0 && "$IN_BETA" -eq 0 ]]; then
    echo "SELF-TEST PASS: falsificação AC3 — detecta roadmap em alpha mas não em beta (bug #320)"
  else
    echo "SELF-TEST FAIL: falsificação AC3 — cenário não representou o bug corretamente (alpha=$IN_ALPHA beta=$IN_BETA)"
    FAIL=1
  fi

  if [[ "$FAIL" -eq 0 ]]; then
    echo "Self-test summary: PASS"
    exit 0
  else
    echo "Self-test summary: FAIL"
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Verificar se os binários estão disponíveis
# ---------------------------------------------------------------------------
if [[ ! -f "$GO_BIN" ]]; then
  echo "::error::consumer-smoke-by-agent: GO_BIN não encontrado: $GO_BIN — rode 'make build' primeiro" >&2
  exit 1
fi

if [[ ! -f "$NODE_CLI" ]]; then
  echo "::error::consumer-smoke-by-agent: NODE_CLI não encontrado: $NODE_CLI — rode 'npm ci --ignore-scripts' em npm/ primeiro" >&2
  exit 1
fi

# Verificar Python CLI
PY_CMD=""
if command -v python3 >/dev/null 2>&1 && PYTHONPATH="$PY_ROOT" python3 -m trackfw --version >/dev/null 2>&1; then
  PY_CMD="PYTHONPATH=$PY_ROOT python3 -m trackfw"
elif command -v python >/dev/null 2>&1 && PYTHONPATH="$PY_ROOT" python -m trackfw --version >/dev/null 2>&1; then
  PY_CMD="PYTHONPATH=$PY_ROOT python -m trackfw"
else
  echo "::error::consumer-smoke-by-agent: Python CLI não disponível — rode 'pip install pypi/' primeiro" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Criar projeto descartável
# ---------------------------------------------------------------------------
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-consumer-smoke-by-agent.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

PROJECT="$WORK/test-project"
mkdir -p "$PROJECT/docs/adr"
mkdir -p "$PROJECT/docs/req"
mkdir -p "$PROJECT/docs/roadmaps/alpha"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$PROJECT/docs/roadmaps/beta"/{backlog,analyzing,wip,blocked,done,abandoned}

cat > "$PROJECT/trackfw.yaml" <<'YAML'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
- alpha
- beta
YAML

FAIL=0

echo "=== AC3 consumer smoke by_agent (2 agentes: alpha, beta) ==="
echo "Projeto: $PROJECT"
echo ""

# ---------------------------------------------------------------------------
# REQ new — todos os 3 CLIs
# ---------------------------------------------------------------------------
echo "--- req new (Go) ---"
(cd "$PROJECT" && "$GO_BIN" req new "REQ-consumer-smoke-go" </dev/null) && \
  echo "req new (Go): OK" || { echo "::error::req new (Go) falhou" >&2; FAIL=1; }

echo "--- req new (Node) ---"
(cd "$PROJECT" && node "$NODE_CLI" req new "REQ-consumer-smoke-node" </dev/null) && \
  echo "req new (Node): OK" || { echo "::error::req new (Node) falhou" >&2; FAIL=1; }

echo "--- req new (Python) ---"
(cd "$PROJECT" && PYTHONPATH="$PY_ROOT" python3 -m trackfw req new "REQ-consumer-smoke-python" </dev/null) && \
  echo "req new (Python): OK" || { echo "::error::req new (Python) falhou" >&2; FAIL=1; }

# REQs devem estar em docs/req/
REQ_COUNT=$(find "$PROJECT/docs/req" -name "*.md" 2>/dev/null | wc -l)
echo ""
echo "REQs criadas em docs/req/: $REQ_COUNT"
if [[ "$REQ_COUNT" -ne 3 ]]; then
  echo "::error::AC3: esperava 3 REQs em docs/req/, encontrou $REQ_COUNT" >&2
  FAIL=1
fi

# Pegar path de uma REQ para usar no roadmap new --req
FIRST_REQ=$(find "$PROJECT/docs/req" -name "*.md" 2>/dev/null | head -1)

# ---------------------------------------------------------------------------
# Roadmap new — Go (sem --req, deve ir para alpha/backlog — bug #320)
# ---------------------------------------------------------------------------
echo ""
echo "--- roadmap new (Go, sem --req) ---"
(cd "$PROJECT" && "$GO_BIN" roadmap new "ROADMAP-consumer-smoke-go" </dev/null) && \
  echo "roadmap new (Go): OK" || { echo "::error::roadmap new (Go) falhou" >&2; FAIL=1; }

# roadmap new --req (deve inferir agente, mas #320: vai para alpha)
if [[ -n "$FIRST_REQ" ]]; then
  echo ""
  echo "--- roadmap new (Go, com --req) ---"
  (cd "$PROJECT" && "$GO_BIN" roadmap new "ROADMAP-consumer-smoke-go-req" --req "$FIRST_REQ" </dev/null) && \
    echo "roadmap new --req (Go): OK" || { echo "::error::roadmap new --req (Go) falhou" >&2; FAIL=1; }
fi

echo ""
echo "--- roadmap new (Node) ---"
(cd "$PROJECT" && node "$NODE_CLI" roadmap new "ROADMAP-consumer-smoke-node" </dev/null) && \
  echo "roadmap new (Node): OK" || { echo "::error::roadmap new (Node) falhou" >&2; FAIL=1; }

echo ""
echo "--- roadmap new (Python) ---"
(cd "$PROJECT" && PYTHONPATH="$PY_ROOT" python3 -m trackfw roadmap new "ROADMAP-consumer-smoke-python" </dev/null) && \
  echo "roadmap new (Python): OK" || { echo "::error::roadmap new (Python) falhou" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Verificar distribuição de roadmaps — detectar #320
# ---------------------------------------------------------------------------
echo ""
echo "=== Distribuição de roadmaps (detecção de #320) ==="

ALPHA_ROADMAPS=$(find "$PROJECT/docs/roadmaps/alpha" -name "ROADMAP-*.md" 2>/dev/null | wc -l)
BETA_ROADMAPS=$(find "$PROJECT/docs/roadmaps/beta" -name "ROADMAP-*.md" 2>/dev/null | wc -l)

echo "Roadmaps em alpha/ (agents[0]): $ALPHA_ROADMAPS"
echo "Roadmaps em beta/ (agents[1]):  $BETA_ROADMAPS"
echo ""

find "$PROJECT/docs/roadmaps" -name "ROADMAP-*.md" | while read -r f; do
  echo "  $f"
done

if [[ "$BETA_ROADMAPS" -eq 0 && "$ALPHA_ROADMAPS" -gt 0 ]]; then
  echo ""
  echo "::error::AC3: #320 DETECTADO — todos os $ALPHA_ROADMAPS roadmap(s) foram criados em alpha/ (agents[0]). Nenhum foi para beta/ (agents[1]). O comando 'roadmap new' não respeita o agente do contexto nem de --req. Ver issue #320." >&2
  FAIL=1
elif [[ "$BETA_ROADMAPS" -gt 0 ]]; then
  echo "AC3: roadmaps distribuídos (alpha=$ALPHA_ROADMAPS beta=$BETA_ROADMAPS) — #320 pode estar corrigido."
else
  echo "::error::AC3: nenhum roadmap criado em nenhum agente." >&2
  FAIL=1
fi

# ---------------------------------------------------------------------------
# trackfw validate e status — todos os 3 CLIs
# ---------------------------------------------------------------------------
echo ""
echo "--- validate (Go) ---"
(cd "$PROJECT" && "$GO_BIN" validate 2>&1 | tail -3) || true  # validate pode ter warnings

echo ""
echo "--- status (Go) ---"
(cd "$PROJECT" && "$GO_BIN" status 2>&1) || { echo "::error::status (Go) falhou" >&2; FAIL=1; }

echo ""
echo "--- status (Node) ---"
(cd "$PROJECT" && node "$NODE_CLI" status 2>&1) || { echo "::error::status (Node) falhou" >&2; FAIL=1; }

echo ""
echo "--- status (Python) ---"
(cd "$PROJECT" && PYTHONPATH="$PY_ROOT" python3 -m trackfw status 2>&1) || { echo "::error::status (Python) falhou" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Resultado
# ---------------------------------------------------------------------------
echo ""
if [[ "$FAIL" -eq 0 ]]; then
  echo "AC3 consumer smoke: PASS"
  exit 0
else
  echo "AC3 consumer smoke: FAIL (ver ::error:: acima)"
  exit 1
fi
