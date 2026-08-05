---
status: Done
date: 2026-08-04
author: "kg.saran@gmail.com"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy.md"
---

# REQ: json.MarshalIndent do Go escapa HTML e diverge de Node/Python em 3 targets do catalogo (kiro amazonq antigravity-legacy)

> Date: 2026-08-04 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Achado durante a auditoria da Wave 3 da REQ-2026-08-04-comando-trackfw-branch-new (ML-3A). Rodando
`scripts/check-identity-parity.sh` (parte de `make quality` → alvo `parity`), 6 checks falham,
reproduzidos de forma independente e confirmados no código:

```
Identity parity [with-identity] target 'amazonq': artifacts diverge from the Go CLI in: node python
```
(mesmo padrão para `antigravity=legacy-cli` e `kiro=cli`, cada um em modo `with-identity` e
`no-identity` — 3 targets × 2 modos = 6 falhas)

**Causa raiz confirmada**: `internal/integrations/render.go:57`, dentro do `case "cli-agent-json",
"agent-json":` do switch de `Render()`, usa `json.MarshalIndent(map[string]string{...}, "", "  ")` —
a stdlib `encoding/json` do Go faz HTML-escaping por padrão (`<`, `>`, `&` viram `<`, `>`,
`&`), a menos que o encoder use `SetEscapeHTML(false)`. Nenhum dos outros dois runtimes escapa
esses caracteres por padrão (`JSON.stringify` no Node.js, `json.dumps` no Python).

O texto do prompt do papel Architect contém o placeholder literal `<slug>` (na seção "Dispatch
contract" adicionada pela REQ-2026-08-04-corrigir-dispatch-sem-subagent-type... — "sempre
`<slug>-tf`, onde `<slug>` depende..."), o que expõe a divergência: o Go produz `<slug>`
no JSON, Node.js/Python produzem `<slug>` literal. Byte-a-byte diferente, mesmo texto renderizado.

Os 3 targets afetados usam a representação `agent-json`/`cli-agent-json` (que passa por essa linha):

| Target | Surface | Representação |
|---|---|---|
| `kiro` | `cli` | `agent-json` |
| `amazonq` | `cli` | `cli-agent-json` |
| `antigravity` | `legacy-cli` | `agent-json` |

**Não bloqueia CI hoje**: `.github/workflows/quality.yml`, job `parity`, só roda
`check-cli-parity.sh`, `check-validate-parity.sh`, `check-static-assets.sh` e
`check-integration-assets.sh` — não roda `check-identity-parity.sh` nem o `make parity` completo. O
bug só é visível rodando `make quality` localmente. (A lacuna de cobertura do CI em si é uma
observação separada, potencialmente objeto de outra REQ — não faz parte do escopo deste REQ.)

## Acceptance Criteria
- [x] `internal/integrations/render.go:57` para de usar `json.MarshalIndent` puro; usa um
      `json.Encoder` com `SetEscapeHTML(false)` (mantendo a mesma indentação de 2 espaços e, se
      necessário, removendo o `\n` extra que `Encoder.Encode` adiciona, já que `MarshalIndent` não
      adiciona — conferir o `append(data, '\n')` logo após, hoje presente no código)
- [x] `scripts/check-identity-parity.sh` passa com 0 falhas (hoje: 6)
- [x] Auditoria (não necessariamente fix, a menos que confirmado divergente) dos demais
      `json.Marshal`/`json.MarshalIndent`/`json.NewEncoder` em `internal/` cujo output é comparado
      byte-a-byte com Node.js/Python por algum gate de paridade existente — especificamente
      `internal/generators/agentfiles.go` (6 call sites, geram `.claude/settings.json` e
      equivalentes) e a saída `--json` de `trackfw validate`/`barrier`/`update`
      (`internal/commands/validate.go`, `barrier.go`, `update.go`, `update_harness.go`) — confirmar
      se algum deles já diverge na prática (conteúdo real contendo `<`/`>`/`&`) ou se é só risco
      teórico sem gate cobrindo. Chamadas Go-only sem equivalente cross-runtime (servidor HTTP em
      `internal/serve/`, payloads de saída para Jira/Linear em `internal/sync/`) estão fora — não há
      contrato de paridade byte-a-byte para elas
- [x] Teste de regressão: golden/fixture com um caractere `<`, `>` ou `&` no conteúdo de origem,
      confirmando que a saída JSON do Go não escapa (byte-idêntica ao Node/Python para o mesmo
      cenário) — evita reintrodução silenciosa
- [x] `make quality` verde (incluindo `check-identity-parity.sh`, hoje não coberto pela suíte que
      falha silenciosamente por não estar no CI — mas continua sendo o critério local de "pronto")
- [x] `trackfw validate` sem violações

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: N/A — correção de bug de serialização, sem decisão arquitetural nova.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy.md`
