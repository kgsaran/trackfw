---
status: Open
date: 2026-07-31
author: "Zeus"
adr: "docs/adr/ADR-2026-07-31-gerador-de-roadmap-emite-heading-consolidado-de-criterios-de-aceite.md"
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-07-31-alinhar-marcador-de-criterios-de-aceite-do-gerador-de-roadmap.md"
---

# REQ: Alinhar marcador de criterios de aceite gerado por roadmap new com o validator

> Date: 2026-07-31 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

**O trackfw gera um roadmap que o próprio trackfw rejeita.** Sequência reproduzível, sem nenhuma
edição manual:

```bash
trackfw roadmap new --title "X" --req docs/req/REQ-....md
trackfw roadmap move <nome> wip
trackfw validate
# ✗ roadmap "ROADMAP-....md" is in wip but has no acceptance criteria block
```

### Causa raiz

Divergência entre o marcador **emitido** pelo gerador e o marcador **procurado** pelo validador:

| Componente | Valor |
|---|---|
| Gerador emite | `**Acceptance criteria:**` (negrito, por microlote) |
| Validador procura | `## Acceptance Criteria` ou `## Critérios de Aceite` (heading nível 2) |

Referências verificadas:
- `internal/config/config.go:83` — `AcceptanceMarkers: []string{"## Acceptance Criteria", "## Critérios de Aceite"}`
- `internal/validator/validator.go:989` — `validateWIPHasAcceptanceCriteria()` usa
  `contentHasMarker(s, cfg.AcceptanceMarkers)`
- Gerador Go: `internal/generators/roadmap.go:115` e `:156`
- Gerador Node: `npm/src/generators/roadmap.js:444` e `:500`
- Gerador Python: `pypi/trackfw/generators/roadmap.py:211` e `:321`

`**Acceptance criteria:**` nunca casa com `## Acceptance Criteria`. Os três CLIs divergem do
validador de forma idêntica — é defeito de contrato, não drift de paridade.

Note que `internal/generators/req.go:93` emite corretamente `## Acceptance Criteria`. A
divergência é específica do gerador de roadmap.

### Impacto

Todo agente ou pessoa que siga o caminho oficial (`roadmap new` → `roadmap move ... wip`) bate
numa violação de `validate` que não é culpa sua, e gasta tempo investigando o tooling em vez do
trabalho. Aconteceu nesta sessão em 2026-07-31 e exigiu contorno manual. É especialmente danoso
porque a regra `branch_has_wip_roadmap` empurra o usuário para exatamente essa sequência.

Nota de vault: `vault/notes/roadmap-new-gera-marcador-de-aceite-invalido-2026-07-31.md`

## Acceptance Criteria

- [ ] **AC1** — Um roadmap recém-criado por `trackfw roadmap new` e movido para `wip` passa em
      `trackfw validate` **sem nenhuma edição manual**, nos três CLIs.
- [ ] **AC2** — O gerador emite um heading consolidado que casa com `cfg.AcceptanceMarkers`,
      **sem remover** os blocos `**Acceptance criteria:**` por microlote, que continuam sendo a
      unidade operacional de cada ML.
- [ ] **AC3** — Paridade: os três geradores produzem o **mesmo** artefato byte-a-byte para a
      mesma entrada. `scripts/check-artifact-parity.sh` passa.
- [ ] **AC4** — Vale também para `roadmap new --from-req`, que gera stubs de ML a partir dos
      critérios de aceite da REQ.
- [ ] **AC5** — Teste de falsificação no padrão `scripts/check-gates-falsify.sh`: ao reverter o
      gerador para emitir apenas `**Acceptance criteria:**`, o teste deve **falhar** com a
      violação `wip_acceptance`. Provar o efeito de ponta a ponta (gerar → mover → validar),
      não apenas a presença da string no template.
- [ ] **AC6** — Roadmaps **já existentes** em `docs/roadmaps/` não são invalidados pela mudança.
      `trackfw validate` continua verde no repositório.
- [ ] **AC7** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não** adicionar `"**Acceptance criteria:**"` a `AcceptanceMarkers` no default do config.
  Isso mascara o defeito, enfraquece o validador e aceita como válido um marcador que não é
  heading. A correção é no gerador.
- Não alterar o gerador de REQ (`req.go` e equivalentes) — já está correto.
- Não alterar a lógica de `validateWIPHasAcceptanceCriteria` nem de `contentHasMarker`.
- Não alterar o comportamento de `roadmap move` nem a regra `branch_has_wip_roadmap`.
- Não reescrever roadmaps existentes no repositório em massa — a mudança é no template de
  artefatos novos.
- Não mexer em `pypi/build/lib/` — artefato de build.
- Não alterar `docs/roadmaps/.trackfw-log`.

## Notas de implementação

Regra dura de paridade: a mudança é obrigatória nos **três** geradores, e eles produzem artefato
comparado byte-a-byte por `scripts/check-artifact-parity.sh`. Os três MLs tocam arquivos
disjuntos (`internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py`), portanto **podem rodar em paralelo** — mas o gate de
paridade só fecha depois dos três, então precisa de barreira antes da validação final.

**Decisões já tomadas no ADR pareado:** o heading consolidado fica **após a seção de contexto e
antes da primeira wave**, e é gerado como **placeholder a preencher** — não como agregação
automática dos critérios dos MLs, que criaria duas fontes de verdade divergentes na primeira
edição manual. Os blocos `**Acceptance criteria:**` por ML permanecem intactos.

## Linked ADR

ADR: docs/adr/ADR-2026-07-31-gerador-de-roadmap-emite-heading-consolidado-de-criterios-de-aceite.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/backlog/ROADMAP-2026-07-31-alinhar-marcador-de-criterios-de-aceite-do-gerador-de-roadmap.md
