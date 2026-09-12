---
type: bug
date: 2026-09-12
pr: "330"
ml: ML-3B-a
status: open — reportado ao arquiteto, não corrigido
---

# Python `roadmap new --req` não herda o agente da REQ (AC11)

## Causa raiz medida

`roadmap new --req <caminho-em-beta/>` no Python retorna o erro de ambiguidade
(`by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one`) em
vez de derivar o namespace do caminho da REQ.

Go e Node implementam a herança corretamente: o caminho `docs/req/beta/REQ-*.md` faz o roadmap
ir para `docs/roadmaps/beta/backlog/` sem `--agent` explícito.

## Como reproduzir

```bash
TMP=$(mktemp -d)
cat > "$TMP/trackfw.yaml" <<'YAML'
governance_mode: strict
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
- alpha
- beta
YAML
mkdir -p "$TMP/docs/req" "$TMP/docs/roadmaps"/{alpha,beta}/backlog

BETA_REQ=$(cd "$TMP" && <go-bin> req new "REQ-beta" --agent beta | awk '{print $2}')

# Go — OK (RC=0, cria em beta/backlog)
cd "$TMP" && <go-bin> roadmap new "ROADMAP-go" --req "$BETA_REQ"

# Node — OK (RC=0, cria em beta/backlog)
cd "$TMP" && node <node-cli> roadmap new "ROADMAP-node" --req "$BETA_REQ"

# Python — FALHA (RC=1, "multiple agent namespaces")
cd "$TMP" && PYTHONPATH=<pypi> python3 -m trackfw roadmap new "ROADMAP-py" --req "$BETA_REQ"
```

## Contexto da descoberta

Descoberto em ML-3B-a ao exercitar o Go de verdade pela primeira vez no smoke test
`check-consumer-smoke-by-agent.sh` (issue #328: `GO_BIN` era relativo, dava rc=127 após
`cd "$PROJECT"`). Com o Go funcionando, o teste `--req` em 3 runtimes foi adicionado ao smoke,
e o Python falhou.

ML-1C declarou AC11 como Concluído, mas a auditoria usou `--req` sem medir o Python
explicitamente. O defecto sobreviveu.

## Impacto

- AC11 do roadmap `ROADMAP-2026-09-11-by-agent-req-new-e-roadmap-new-...` não está satisfeito
  no Python.
- O smoke `check-consumer-smoke-by-agent.sh` falha no CI (::error::AC11 Python).
- Mesma causa que o B1 da Wave 1 no Go (ordem errada: `ResolveWriteAgent` antes de derivar
  agente do `--req`). Presumível que Python tem a mesma inversão de ordem.

## Onde investigar

- `pypi/trackfw/generators/roadmap.py` — função equivalente a `NewRoadmapFromContent` do Go.
  Verificar se `resolve_write_agent` (ou equivalente) roda antes de extrair o agente do
  `req_path`.
- Compare com `pypi/trackfw/generators/roadmap.py` função de `roadmap new --from-req`
  (se existir) — pode ter a ordem certa, como acontecia no Go antes do B1.

## Decisão

Não corrigido neste ML (escopo: somente `scripts/check-consumer-smoke-by-agent.sh` e
`.github/workflows/quality.yml`). Reportado ao arquiteto. O gate detecta a falha.
