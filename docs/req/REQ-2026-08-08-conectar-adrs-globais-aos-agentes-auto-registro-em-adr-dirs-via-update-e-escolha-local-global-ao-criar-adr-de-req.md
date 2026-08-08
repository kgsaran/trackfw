---
status: Done
date: 2026-08-08
author: ""
adr: ""
roadmap: ""
---

# REQ: conectar ADRs globais aos agentes: auto-registro em adr_dirs via update e escolha local-global ao criar ADR de REQ

> Date: 2026-08-08 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
`trackfw adr new/list --scope global` (REQ-2026-08-08-adr-new-com-escopo-global-...,
já implementado) permite criar/listar ADRs em `~/.trackfw/adr/`, mas isso é
inerte hoje: nenhum projeto tem `~/.trackfw/adr` no `adr_dirs` do seu
`trackfw.yaml`, então (a) `trackfw context` nunca lista esses ADRs, (b) a
diretiva já injetada em todo `CLAUDE.md`/`AGENTS.md` ("Obrigatório:
Inspecione e respeite todos os ADRs globais nos diretórios listados em
adr_dirs...") não tem nada para apontar, e (c) os subagentes especialistas
(`~/.claude/agents/*.md` — Apolo, Hefesto, etc.) não têm NENHUMA menção a
ADRs/`adr_dirs`, então só enxergam ADRs se o orquestrador colar isso
manualmente no prompt de delegação — o que não vinha acontecendo. Escrever
ADRs globais sem nenhum desses três elos é trabalho perdido.

Escopo desta REQ, por decisão do usuário: fechar os dois primeiros elos
(auto-registro em `adr_dirs` + escolha de escopo no momento de criação via
REQ). O terceiro elo (garantir que os subagentes especialistas recebam
contexto de ADR ao serem delegados) é comportamento do orquestrador
(Zeus/Claude), não uma mudança de código do trackfw — fica registrado aqui
como responsabilidade operacional, não como ML.

## Acceptance Criteria
- [ ] `trackfw update` (escopo projeto, os 3 CLIs) adiciona `~/.trackfw/adr`
      ao `adr_dirs` do `trackfw.yaml` do projeto **se e somente se** esse
      diretório existir no `$HOME` do usuário **e** contiver pelo menos um
      `ADR-*.md` — nunca cria a entrada "no escuro" apontando para um
      diretório vazio/inexistente
- [ ] A escrita é idempotente e cirúrgica: nunca duplica a entrada se já
      presente, nunca remove/reordena entradas existentes do usuário,
      preserva comentários e demais linhas do `trackfw.yaml` — mesma
      categoria de merge já usada por `updateHooksSurgical`
      (`internal/generators/update.go`), não uma recriação do arquivo
      (diferente do wizard `trackfw configure`, que recria do zero)
- [ ] `trackfw req new` (fluxo interativo, os 3 CLIs), ao gerar um ADR draft
      a partir de uma probe detectada, pergunta (só em TTY) se aquele ADR é
      `local` (default) ou `global` — um único prompt por sessão de REQ,
      aplicado a todos os ADR drafts gerados nela, não por-pergunta
- [ ] Modo não-interativo (sem TTY, ex.: CI/scripts) preserva o
      comportamento atual sem nenhuma mudança: ADRs sempre `local`, sem
      prompt novo
- [ ] Testes com `$HOME`/cwd de fixture (nunca o `$HOME` real) verdes nos 3 stacks
- [ ] `docs/cli-parity.md` atualizado
- [ ] `trackfw validate` sem violações

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-07-19-global-adrs-governance.md (decisão #4 dessa ADR já previa a diretiva de prompt para IA; esta REQ fecha a lacuna de que a diretiva hoje não tem dados para apontar)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-08-conectar-adrs-globais-aos-agentes-auto-registro-em-adr-dirs-via-update-e-escolha-local-global-ao-criar-adr-de-req.md
