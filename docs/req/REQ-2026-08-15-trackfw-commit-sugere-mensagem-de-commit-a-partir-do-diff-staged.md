---
status: Done
date: 2026-08-15
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md"
---

# REQ: trackfw commit sugere mensagem de commit a partir do diff staged

> Date: 2026-08-15 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Confirmado no código (`internal/commands/commit.go`): `trackfw commit -m "<mensagem>"`
exige a mensagem completa como argumento obrigatório e só a repassa para
`git commit -m <message>` depois da checagem de governança — não analisa o diff staged,
não sugere tipo/escopo, não monta corpo. Quem escreve a mensagem inteira é sempre quem
chama o comando.

Achado relacionado à REQ irmã `REQ-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-...`
(corpo de PR pobre por não agregar histórico) — mesma classe de lacuna, um degrau antes:
nem a mensagem de commit individual tem qualquer ajuda automática.

**Escopo deliberadamente conservador — sem chamada a LLM.** `trackfw` é um binário
Go/Node/Python sem integração com API de LLM (Anthropic/OpenAI) hoje, e não deveria
ganhar uma nas linguagens dos 3 CLIs só para isso — implicaria API key, chamada de rede,
custo e resultado não-determinístico, quebrando a paridade byte-a-byte que o projeto já
audita entre os 3 stacks. `trackfw` é a ferramenta que um agente de IA (Claude Code,
Codex etc.) invoca, não uma ferramenta que invoca um agente de IA. Esta REQ propõe uma
**sugestão heurística/estrutural** (tipo Conventional Commits detectado por padrão de
arquivos alterados + lista de arquivos + resumo mecânico do diff), não geração de prosa
natural — a prosa continua sendo escrita por quem chama o comando (humano ou agente).

## Acceptance Criteria
- [x] Nova flag `trackfw commit --suggest` (sem `-m`) imprime um esqueleto de mensagem no
      stdout e sai sem commitar — não executa `git commit`.
- [x] O esqueleto inclui: (a) tipo Conventional Commits sugerido por heurística simples
      sobre os arquivos staged (ex.: só arquivos em `*_test.go`/`*.test.js`/`test_*.py` →
      `test`; só `docs/`/`*.md` → `docs`; presença de arquivo novo em `internal/commands/`
      ou equivalente → `feat`; caso contrário → `fix`/`chore`, decisão de fallback a
      documentar no ML); (b) lista dos arquivos staged agrupados por status
      (`git diff --cached --name-status`); (c) um placeholder de corpo para preencher.
- [x] Documentar explicitamente na saída do `--suggest` que é uma heurística de
      apoio, não uma mensagem pronta — não fingir certeza sobre o tipo/escopo detectado.
- [x] Comportamento idêntico nos 3 CLIs (mesma heurística, mesmo formato de saída).
- [x] `trackfw commit -m "..."` (uso normal, sem `--suggest`) continua funcionando
      exatamente como hoje — este ML não muda o caminho padrão, só adiciona o novo modo.
- [x] `make quality` passa sem novas divergências de paridade.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md
