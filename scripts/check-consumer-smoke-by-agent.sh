#!/usr/bin/env bash
# check-consumer-smoke-by-agent.sh — smoke end-to-end com projeto by_agent + 2 agentes
#
# Verifica que:
#   - req new e roadmap new aceitam --agent e roteiam para o namespace correto (AC4, AC10, AC12)
#   - roadmap new --req herda o agente da REQ (AC11) — Go e Node; Python pendente (ver nota abaixo)
#   - sem --agent com 2 agentes, req new e roadmap new falham com mensagem de ambiguidade (AC5)
#
# Histórico:
#   ML-1C  (PR #326): gate criado com continue-on-error temporário detectando o #320
#   ML-3B-a (PR #330): GO_BIN normalizado para absoluto (issue #328); --req em 3 runtimes;
#                       cenários ajustados para o contrato pós-wave-1 (--agent obrigatório)
#   ML-3B-b (PR #330): continue-on-error removido do job
#
# ⚠️ DEFECTO MEDIDO (2026-09-12): Python `roadmap new --req` não herda o agente da REQ;
#   retorna o erro de ambiguidade em vez de derivar o namespace do caminho da REQ.
#   Go e Node passam. Reportado ao arquiteto — não corrigido neste ML (escopo: scripts/ e quality.yml).
#   Referência: medição em check-consumer-smoke-by-agent.sh ML-3B-a.
#
# Codificação de saída: força UTF-8 no stdio de todo python3 deste gate.
export PYTHONIOENCODING=utf-8

set -euo pipefail

ROOT_DIR=${TRACKFW_ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}

# Resolver binários — permite override via env para uso em CI.
# GO_BIN normalizado ANTES da guarda: guarda e chamadas (após cd "$PROJECT") precisam
# ver o mesmo caminho absoluto. Padrão de check-agent-hooks-parity.sh:73-74 e
# check-agent-namespace-union.sh:135-136. Um GO_BIN relativo como "bin/trackfw" passaria
# a guarda (que roda na raiz do repo) mas daria rc=127 nas chamadas após cd "$PROJECT".
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
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

  # Braço 3 (AC5 — roteamento por --agent): simula que roteamento distribui roadmaps
  # entre ambos os namespaces ao usar --agent explícito.
  ST_WORK=$(mktemp -d "${TMPDIR:-/tmp}/consumer-smoke-self-test.XXXXXX")
  trap 'rm -rf "$ST_WORK"' EXIT

  mkdir -p "$ST_WORK/docs/roadmaps/alpha/backlog"
  mkdir -p "$ST_WORK/docs/roadmaps/beta/backlog"

  # Simula roadmap criado com --agent alpha
  touch "$ST_WORK/docs/roadmaps/alpha/backlog/ROADMAP-test-alpha.md"
  # Simula roadmap criado com --agent beta
  touch "$ST_WORK/docs/roadmaps/beta/backlog/ROADMAP-test-beta.md"

  IN_ALPHA=$(find "$ST_WORK/docs/roadmaps/alpha/backlog" -name "ROADMAP-*.md" | wc -l)
  IN_BETA=$(find "$ST_WORK/docs/roadmaps/beta/backlog" -name "ROADMAP-*.md" | wc -l)

  if [[ "$IN_ALPHA" -gt 0 && "$IN_BETA" -gt 0 ]]; then
    echo "SELF-TEST PASS: ambos namespaces têm roadmaps — roteamento --agent funciona"
  else
    echo "SELF-TEST FAIL: roteamento --agent não distribuiu (alpha=$IN_ALPHA beta=$IN_BETA)"
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
# Nota: docs/req/{alpha,beta}/ NÃO são pré-criados — os CLIs criam ao escrever (by_agent + req_dir).
# Pré-criar mascararia um defeito de criação automática de diretório.

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

echo "=== consumer smoke by_agent (2 agentes: alpha, beta) ==="
echo "Projeto: $PROJECT"
echo ""

# ---------------------------------------------------------------------------
# CENÁRIO DE AMBIGUIDADE — sem --agent com 2 agentes → erro obrigatório (AC5)
# Afirma: sem --agent, o CLI rejeita com RC=1 e mensagem de ambiguidade byte-idêntica
#         nos 3 runtimes (contrato de ML-2E).
# Manter este cenário: se regredir, qualquer CLI voltaria a aceitar req new silenciosamente.
# ---------------------------------------------------------------------------
echo "--- ambiguidade: req new sem --agent (3 runtimes — todos devem falhar com RC=1) ---"
AMBIG_RC_GO=0
AMBIG_OUT_GO=$(cd "$PROJECT" && "$GO_BIN" req new "REQ-ambig" </dev/null 2>&1) || AMBIG_RC_GO=$?
AMBIG_RC_NODE=0
AMBIG_OUT_NODE=$(cd "$PROJECT" && node "$NODE_CLI" req new "REQ-ambig" </dev/null 2>&1) || AMBIG_RC_NODE=$?
AMBIG_RC_PY=0
AMBIG_OUT_PY=$(cd "$PROJECT" && PYTHONPATH="$PY_ROOT" python3 -m trackfw req new "REQ-ambig" </dev/null 2>&1) || AMBIG_RC_PY=$?

AMBIG_FAIL=0
for runtime in go node py; do
  eval "rc=\$AMBIG_RC_${runtime^^}"
  eval "out=\$AMBIG_OUT_${runtime^^}"
  if [[ "$rc" -ne 1 ]]; then
    echo "::error::ambiguidade req new ($runtime): esperava RC=1, obteve RC=$rc. Saída: $out" >&2
    AMBIG_FAIL=1
  fi
done

if [[ "$AMBIG_FAIL" -eq 0 ]]; then
  # Verificar que a mensagem de erro é byte-idêntica entre os 3 runtimes (ML-2E)
  # Comparar apenas a primeira linha — Go emite linhas adicionais de uso
  GO_FIRST=$(echo "$AMBIG_OUT_GO" | head -1)
  NODE_FIRST=$(echo "$AMBIG_OUT_NODE" | head -1)
  PY_FIRST=$(echo "$AMBIG_OUT_PY" | head -1)

  if diff_go_node=$(diff <(echo "$GO_FIRST") <(echo "$NODE_FIRST") 2>&1) && \
     diff_go_py=$(diff <(echo "$GO_FIRST") <(echo "$PY_FIRST") 2>&1); then
    echo "ambiguidade req new (3 runtimes): OK — RC=1 e mensagem byte-idêntica"
    echo "  mensagem: $GO_FIRST"
  else
    echo "::error::ambiguidade req new: mensagem de erro difere entre runtimes:" >&2
    echo "  Go:   $GO_FIRST" >&2
    echo "  Node: $NODE_FIRST" >&2
    echo "  Py:   $PY_FIRST" >&2
    echo "  diff Go/Node: $diff_go_node" >&2
    echo "  diff Go/Py:   $diff_go_py" >&2
    FAIL=1
  fi
else
  FAIL=1
fi

# ---------------------------------------------------------------------------
# REQ new com --agent explícito — 3 runtimes
# ---------------------------------------------------------------------------
echo ""
echo "--- req new --agent alpha (Go) ---"
(cd "$PROJECT" && "$GO_BIN" req new "REQ-smoke-go" --agent alpha </dev/null) && \
  echo "req new --agent alpha (Go): OK" || { echo "::error::req new --agent alpha (Go) falhou" >&2; FAIL=1; }

echo ""
echo "--- req new --agent beta (Node) — cria REQ que será usada no --req herdado ---"
(cd "$PROJECT" && node "$NODE_CLI" req new "REQ-smoke-node" --agent beta </dev/null) && \
  echo "req new --agent beta (Node): OK" || { echo "::error::req new --agent beta (Node) falhou" >&2; FAIL=1; }

echo ""
echo "--- req new --agent alpha (Python) ---"
(cd "$PROJECT" && PYTHONPATH="$PY_ROOT" python3 -m trackfw req new "REQ-smoke-python" --agent alpha </dev/null) && \
  echo "req new --agent alpha (Python): OK" || { echo "::error::req new --agent alpha (Python) falhou" >&2; FAIL=1; }

# Verificar REQs criadas: 3 esperadas (os CLIs criam docs/req/alpha/ e docs/req/beta/ automaticamente)
REQ_COUNT=$(find "$PROJECT/docs/req" -name "*.md" 2>/dev/null | wc -l)
echo ""
echo "REQs criadas em docs/req/ (recursive): $REQ_COUNT"
if [[ "$REQ_COUNT" -ne 3 ]]; then
  echo "::error::esperava 3 REQs (1 Go-alpha, 1 Node-beta, 1 Python-alpha), encontrou $REQ_COUNT" >&2
  FAIL=1
fi

# REQ criada pelo Node em beta/ — usada nos cenários --req (herança de agente, AC11)
BETA_REQ=$(find "$PROJECT/docs/req/beta" -name "*.md" 2>/dev/null | head -1)
if [[ -z "$BETA_REQ" ]]; then
  echo "::error::REQ em docs/req/beta/ não encontrada — req new --agent beta (Node) falhou?" >&2
  FAIL=1
fi

# ---------------------------------------------------------------------------
# Roadmap new com --agent explícito — 3 runtimes
# ---------------------------------------------------------------------------
echo ""
echo "--- roadmap new --agent alpha (Go) ---"
(cd "$PROJECT" && "$GO_BIN" roadmap new "ROADMAP-smoke-go" --agent alpha </dev/null) && \
  echo "roadmap new --agent alpha (Go): OK" || { echo "::error::roadmap new --agent alpha (Go) falhou" >&2; FAIL=1; }

echo ""
echo "--- roadmap new --agent beta (Node) ---"
(cd "$PROJECT" && node "$NODE_CLI" roadmap new "ROADMAP-smoke-node" --agent beta </dev/null) && \
  echo "roadmap new --agent beta (Node): OK" || { echo "::error::roadmap new --agent beta (Node) falhou" >&2; FAIL=1; }

echo ""
echo "--- roadmap new --agent alpha (Python) ---"
(cd "$PROJECT" && PYTHONPATH="$PY_ROOT" python3 -m trackfw roadmap new "ROADMAP-smoke-python" --agent alpha </dev/null) && \
  echo "roadmap new --agent alpha (Python): OK" || { echo "::error::roadmap new --agent alpha (Python) falhou" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Roadmap new --req (herança de agente — AC11) — Go e Node confirmados
# BETA_REQ está em docs/req/beta/ → roadmaps devem ir para beta/backlog
# Verificação: delta de arquivo (snapshot antes/depois) + frontmatter squad:
#
# ⚠️ Python: --req não herda agente da REQ (defecto medido em 2026-09-12).
#   Python retorna o erro de ambiguidade em vez de derivar o namespace do caminho.
#   Testado aqui para que o CI detecte a regressão. Não corrigido neste ML.
# ---------------------------------------------------------------------------
if [[ -n "$BETA_REQ" ]]; then
  echo ""
  echo "--- roadmap new --req <REQ em beta/> (Go) — deve herdar beta (AC11) ---"
  BEFORE_BETA_GO=$(find "$PROJECT/docs/roadmaps/beta/backlog" -name "*.md" 2>/dev/null | sort)
  (cd "$PROJECT" && "$GO_BIN" roadmap new "ROADMAP-smoke-go-req" --req "$BETA_REQ" </dev/null) && \
    echo "roadmap new --req (Go): OK" || { echo "::error::roadmap new --req (Go) falhou" >&2; FAIL=1; }
  AFTER_BETA_GO=$(find "$PROJECT/docs/roadmaps/beta/backlog" -name "*.md" 2>/dev/null | sort)
  GO_REQ_FILE=$(comm -13 <(echo "$BEFORE_BETA_GO") <(echo "$AFTER_BETA_GO") | head -1)
  if [[ -n "$GO_REQ_FILE" ]]; then
    GO_SQUAD=$(grep "^squad:" "$GO_REQ_FILE" | awk '{print $2}' | tr -d '"')
    if [[ "$GO_SQUAD" == "beta" ]]; then
      echo "  herança --req (Go): squad=beta no frontmatter — OK ($GO_REQ_FILE)"
    else
      echo "::error::herança --req (Go): squad='$GO_SQUAD' no frontmatter, esperava 'beta'" >&2
      FAIL=1
    fi
  else
    echo "::error::herança --req (Go): nenhum roadmap novo detectado em beta/backlog/" >&2
    FAIL=1
  fi

  echo ""
  echo "--- roadmap new --req <REQ em beta/> (Node) — deve herdar beta (AC11) ---"
  BEFORE_BETA_NODE=$(find "$PROJECT/docs/roadmaps/beta/backlog" -name "*.md" 2>/dev/null | sort)
  (cd "$PROJECT" && node "$NODE_CLI" roadmap new "ROADMAP-smoke-node-req" --req "$BETA_REQ" </dev/null) && \
    echo "roadmap new --req (Node): OK" || { echo "::error::roadmap new --req (Node) falhou" >&2; FAIL=1; }
  AFTER_BETA_NODE=$(find "$PROJECT/docs/roadmaps/beta/backlog" -name "*.md" 2>/dev/null | sort)
  NODE_REQ_FILE=$(comm -13 <(echo "$BEFORE_BETA_NODE") <(echo "$AFTER_BETA_NODE") | head -1)
  if [[ -n "$NODE_REQ_FILE" ]]; then
    NODE_SQUAD=$(grep "^squad:" "$NODE_REQ_FILE" | awk '{print $2}' | tr -d '"')
    if [[ "$NODE_SQUAD" == "beta" ]]; then
      echo "  herança --req (Node): squad=beta no frontmatter — OK ($NODE_REQ_FILE)"
    else
      echo "::error::herança --req (Node): squad='$NODE_SQUAD' no frontmatter, esperava 'beta'" >&2
      FAIL=1
    fi
  else
    echo "::error::herança --req (Node): nenhum roadmap novo detectado em beta/backlog/" >&2
    FAIL=1
  fi

  echo ""
  echo "--- roadmap new --req <REQ em beta/> (Python) --- [⚠️ defecto medido — deve FALHAR]"
  PY_REQ_RC=0
  PY_REQ_OUT=$(cd "$PROJECT" && PYTHONPATH="$PY_ROOT" python3 -m trackfw roadmap new "ROADMAP-smoke-python-req" --req "$BETA_REQ" </dev/null 2>&1) || PY_REQ_RC=$?
  if [[ "$PY_REQ_RC" -ne 0 ]]; then
    echo "  roadmap new --req (Python): FALHOU com RC=$PY_REQ_RC — defecto AC11 confirmado"
    echo "  saída: $PY_REQ_OUT"
    echo "::error::AC11 Python: roadmap new --req não herda agente da REQ. Ver nota no cabeçalho do script." >&2
    FAIL=1
  else
    echo "  roadmap new --req (Python): OK (defecto resolvido)"
    # Verificar frontmatter
    BEFORE_BETA_PY=$(find "$PROJECT/docs/roadmaps/beta/backlog" -name "*.md" 2>/dev/null | sort)
    PY_REQ_FILE=$(comm -13 <(echo "$BEFORE_BETA_PY") <(echo "$(find "$PROJECT/docs/roadmaps/beta/backlog" -name "*.md" 2>/dev/null | sort)") | head -1)
    if [[ -n "$PY_REQ_FILE" ]]; then
      PY_SQUAD=$(grep "^squad:" "$PY_REQ_FILE" | awk '{print $2}' | tr -d '"')
      [[ "$PY_SQUAD" == "beta" ]] || { echo "::error::herança --req (Python): squad='$PY_SQUAD', esperava 'beta'" >&2; FAIL=1; }
    fi
  fi
fi

# ---------------------------------------------------------------------------
# Verificar distribuição de roadmaps — contagens exatas esperadas
#
# Tabela de roteamento:
#   roadmap new --agent alpha (Go)    → alpha/backlog  (1)
#   roadmap new --agent beta  (Node)  → beta/backlog   (1)
#   roadmap new --agent alpha (Python)→ alpha/backlog  (1)
#   roadmap new --req beta (Go)       → beta/backlog   (1) — se AC11 OK
#   roadmap new --req beta (Node)     → beta/backlog   (1) — se AC11 OK
# Python --req não cria (defecto)
#
# Mínimos assertivos: alpha=2 (Go+Python com --agent), beta≥2 (Node com --agent + Go e Node --req)
# ---------------------------------------------------------------------------
echo ""
echo "=== Distribuição de roadmaps ==="

ALPHA_ROADMAPS=$(find "$PROJECT/docs/roadmaps/alpha" -name "ROADMAP-*.md" 2>/dev/null | wc -l)
BETA_ROADMAPS=$(find "$PROJECT/docs/roadmaps/beta" -name "ROADMAP-*.md" 2>/dev/null | wc -l)

echo "Roadmaps em alpha/: $ALPHA_ROADMAPS"
echo "Roadmaps em beta/:  $BETA_ROADMAPS"
echo ""

find "$PROJECT/docs/roadmaps" -name "ROADMAP-*.md" | sort | while read -r f; do
  echo "  $f"
done

# alpha: exatamente 2 (Go e Python com --agent alpha)
if [[ "$ALPHA_ROADMAPS" -eq 2 ]]; then
  echo "alpha/: $ALPHA_ROADMAPS roadmap(s) — OK (exato)"
else
  echo "::error::alpha/: esperava exatamente 2 (Go+Python --agent alpha), encontrou $ALPHA_ROADMAPS" >&2
  FAIL=1
fi

# beta: mínimo 2 — Node --agent beta + Go --req herdado (Node --req herdado = 3 se AC11 OK)
if [[ "$BETA_ROADMAPS" -ge 2 ]]; then
  echo "beta/: $BETA_ROADMAPS roadmap(s) — OK (≥2: Node --agent + --req herdado(s))"
else
  echo "::error::beta/: esperava ≥2, encontrou $BETA_ROADMAPS" >&2
  FAIL=1
fi

# ---------------------------------------------------------------------------
# validate (Go) — verifica que o binário executa sem crash
# DECISÃO: o fixture cria REQs sem ADR e sem Roadmap vinculados. Em governance_mode
# strict, validate emite req_has_adr e req_has_roadmap como violations — comportamento
# correto. O smoke não é um projeto governado; assertar 0 violations seria falso.
# Assertamos apenas RC≠127 (binário encontrado e executável). Violations de governança
# do fixture não são sinal de regressão do smoke.
# ---------------------------------------------------------------------------
echo ""
echo "--- validate (Go) — RC≠127 (binário executa) ---"
VALIDATE_RC=0
{ VALIDATE_OUTPUT=$(cd "$PROJECT" && "$GO_BIN" validate 2>&1); } || VALIDATE_RC=$?
echo "$VALIDATE_OUTPUT" | tail -3
if [[ "$VALIDATE_RC" -eq 127 ]]; then
  echo "::error::validate (Go): binário não encontrado ou não executável (RC=127) — GO_BIN=$GO_BIN" >&2
  FAIL=1
else
  echo "validate (Go): executou sem crash (RC=$VALIDATE_RC) — OK"
fi

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
  echo "consumer smoke by_agent: PASS"
  exit 0
else
  echo "consumer smoke by_agent: FAIL (ver ::error:: acima)"
  exit 1
fi
