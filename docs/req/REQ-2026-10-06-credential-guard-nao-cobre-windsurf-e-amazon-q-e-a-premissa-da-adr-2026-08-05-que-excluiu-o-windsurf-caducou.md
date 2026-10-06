---
status: Open
date: 2026-10-06
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
---

# REQ: credential guard nao cobre Windsurf e Amazon Q, e a premissa da ADR-2026-08-05 que excluiu o Windsurf caducou

> Date: 2026-10-06 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Achado do ML-2C da REQ-2026-09-05 (2026-10-06): o gerador instala o **git-branch guard** para Windsurf
(`.windsurf/hooks.json`, evento `pre_run_command`) e Amazon Q (`.amazonq/cli-agents/q_cli_default.json`,
`preToolUse`), mas **nunca o credential guard**. Para esses dois CLIs, um subagente pode materializar uma
credencial real sem que nenhum hook a veja.

A exclusão do Windsurf foi deliberada na ADR-2026-08-05 (Alternatives Considered), com a premissa de que
o Windsurf **não tinha hook pré-execução por tool call** (só `pre_user_prompt`/`post_write_code`/
`post_cascade_response`). A premissa caducou: o próprio trackfw já emite um hook `pre_run_command` para o
Windsurf. A ADR-2026-08-06 (escopo global) herdou a exclusão sem rever a premissa. O Amazon Q entrou no
gerador depois das duas ADRs e não foi avaliado para o credential guard.

Não é a causa da REQ-2026-09-05 (lá: o hook não executa no Windows). Aqui: o hook nunca é instalado.
Por isso, REQ própria.

**Escopo negativo:** não muda o credential guard em si (padrões, modo block, cwd); não muda o git-branch
guard; não trata outros CLIs; não reabre a decisão de modo avisador por padrão da ADR-2026-08-05.

## Acceptance Criteria
- [ ] AC1 — Remedição, com fonte oficial e data: quais eventos de hook do Windsurf e do Amazon Q recebem o payload da tool call (comando e/ou conteúdo escrito) e se um exit code bloqueia ou só avisa
- [ ] AC2 — Adendo à ADR-2026-08-05 revendo a premissa do Windsurf, com a decisão por CLI (instalar, ou manter fora com o motivo medido)
- [ ] AC3 — Para cada CLI decidido "instalar": `trackfw init`/`update` emitem o credential guard na forma da D2 revista da ADR-2026-10-04, e o `validate` deixa de silenciar o arquivo (teste nas duas direções)
- [ ] AC4 — Prova de disparo real em ao menos um dos dois CLIs, ou a impossibilidade de prova declarada com o motivo

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: `docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md` (premissa a rever);
`docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md` (forma da linha de hook).
Depende da REQ-2026-09-05 (a linha `trackfw guard credential` precisa existir).

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md
