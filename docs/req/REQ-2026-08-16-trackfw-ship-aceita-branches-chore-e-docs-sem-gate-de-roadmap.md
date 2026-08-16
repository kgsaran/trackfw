---
status: Open
date: 2026-08-16
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-16-trackfw-ship-aceita-branches-chore-e-docs-sem-gate-de-roadmap.md"
---

# REQ: trackfw ship aceita branches chore e docs sem gate de roadmap

> Date: 2026-08-16 | Status: Open
| Linear Issue: 
| Jira Issue: 


## Motivation

Par do `REQ-2026-08-16-trackfw-branch-new-aceita-tipos-chore-e-docs-sem-gate-de-roadmap` (mergeada
em #177). Aquele fix destravou a **criação** da branch de release; falta a **publicação**.

Estado medido em 2026-08-16, com a release 7.0.0 já commitada e travada:

| Comando | Branch `chore/` | Onde |
|---|---|---|
| `trackfw branch new` | ✅ aceita (após #177) | — |
| `trackfw commit` | ✅ aceita, sem exigir roadmap | regra 3 do `--help` |
| `trackfw ship` | ❌ **recusa** | regra 1; `internal/commands/ship.go:171`, `:513` |
| push cru | ❌ bloqueado pelo guard | — |

Resultado: a release fica **commitada e impublicável**.

**Erro de leitura que o arquiteto cometeu e registra aqui:** na REQ anterior foi afirmado que
"`ship` e `commit` já tratam branches fora de `feat|fix|refactor` como housekeeping permitido". Isso
vale **apenas para o `commit`**. O `ship` é explícito na regra 1 — *"Validates branch name — must
match `feat|fix|refactor/<slug>`"*. A regra 3 lida pertencia ao `commit`, e foi atribuída aos dois.
Consequência: o fix anterior resolveu metade do caminho, e a outra metade só apareceu ao tentar
publicar.

## Acceptance Criteria

- [ ] **AC1** — `trackfw ship` aceita branches `chore/<slug>` e `docs/<slug>`, **sem** exigir REQ +
      roadmap em `wip/` — coerente com o que `trackfw commit` já faz.
- [ ] **AC2** — `feat`/`fix`/`refactor` **continuam** exigindo governança. O gate duro da regra 2
      **não** é afrouxado para os tipos que o têm hoje.
- [ ] **AC3** — Branch fora do vocabulário (ex.: `banana/x`) continua recusada, com mensagem
      atualizada, **byte-idêntica nos 3 CLIs**.
- [ ] **AC4** — Texto do `--help` das regras 1 e 2 atualizado nos 3 CLIs para refletir o
      comportamento real.
- [ ] **AC5** — `make quality` verde; paridade cobre os casos novos.

## Escopo negativo

- **Não** afrouxa o gate de governança para `feat|fix|refactor`.
- **Não** mexe no `git-branch-guard` (débitos próprios já registrados: brecha de contorno e
  falso-positivo por prosa).
- **Não** altera `trackfw commit` nem `trackfw branch new`, que já estão corretos.

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
