---
status: done
date: 2026-08-16
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-16-trackfw-branch-new-aceita-tipos-chore-e-docs-sem-gate-de-roadmap.md"
---

# REQ: trackfw branch new aceita tipos chore e docs sem gate de roadmap

> Date: 2026-08-16 | Status: done
| Linear Issue: 
| Jira Issue: 


## Motivation

**O protocolo de release do projeto ficou inexecutável pelos caminhos sancionados** — efeito
colateral não previsto do `git-branch-guard` (PR #169).

Estado medido em 2026-08-16:

- `git checkout -b` é **bloqueado** pelo guard (`scripts/trackfw-git-branch-guard.sh:108`);
- `trackfw branch new` **recusa** qualquer tipo fora de `feat|fix|refactor`
  (`internal/commands/branch.go:16,157`);
- mas o **protocolo de release exige** branch `chore/release-x.y.z` — foi assim na 6.10.0 (PR #168),
  criada **antes** de o guard existir.

Resultado: não há caminho legítimo para criar a branch de uma release. O guard **não** cobre
`git switch -c`, então existe brecha — e usá-la seria evadir em silêncio um controle que o próprio
projeto criou, corroendo o valor dele.

A vocabulário restrito do `branch new` foi espelhado do `trackfw ship`, mas os dois **já divergem**
na prática: `trackfw ship` e `trackfw commit` tratam branches fora de `feat|fix|refactor` como
**housekeeping permitido sem roadmap** (regra 3 do `--help` de ambos). O `branch new` é o único que
recusa de todo.

## Acceptance Criteria

- [ ] **AC1** — `trackfw branch new chore/<slug>` e `trackfw branch new docs/<slug>` criam a branch,
      **sem** exigir roadmap em `wip/`/`done/` — coerente com a regra 3 já existente em
      `trackfw ship` e `trackfw commit`.
- [ ] **AC2** — `feat`, `fix` e `refactor` **continuam** exigindo roadmap correspondente. O gate de
      governança não é afrouxado para os tipos que o têm hoje.
- [ ] **AC3** — Tipo inválido (ex.: `banana/x`) continua recusado, com mensagem atualizada listando
      o vocabulário novo.
- [ ] **AC4** — Comportamento idêntico nos 3 CLIs; `scripts/check-branch-new-parity.sh` cobre os
      casos novos.
- [ ] **AC5** — `make quality` verde.

## Escopo negativo

- **Não** afrouxa o gate de roadmap para `feat|fix|refactor`.
- **Não** mexe no `git-branch-guard` nem na brecha do `git switch -c` — vira REQ própria.
- **Não** altera `trackfw ship`/`trackfw commit`, que já tratam esses tipos corretamente.

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
