---
status: Open
date: 2026-08-11
author: "Zeus (Arquiteto)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd.md"
---

# REQ: Resolucao de caminho dos hooks de agente independente do cwd — attention-signal/cleanup e os 5 CLIs nao-Claude

> Date: 2026-08-11 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

O commit `0c66ecb` (v6.7.1) corrigiu **um** dos comandos de hook que o trackfw injeta: o
`trackfw-credential-guard.sh` no wiring do Claude Code, que era emitido como caminho relativo puro
(`scripts/trackfw-credential-guard.sh`). O Claude Code resolve o `command` do hook contra o **cwd
dinâmico** do hook — que acompanha os `cd` feitos pelo agente durante a sessão, não a raiz do
projeto (doc primária: <https://code.claude.com/docs/en/hooks>, "Handlers run in the current
directory"). Em monorepos isso fazia o hook falhar com `No such file or directory`, bug reportado em
produção no projeto CMDB.

A correção foi deliberadamente estreita e o próprio commit registrou o resto como não corrigido.
Permanece emitido como **caminho relativo puro**:

1. `trackfw-attention-signal.sh` e `trackfw-attention-cleanup.sh` no wiring do Claude Code
   (`internal/generators/agentfiles.go:211` e `:265`, mais os equivalentes Node e Python).
2. **Todos** os comandos de hook (attention + credential-guard) no wiring dos outros 5 CLIs:
   Codex, Gemini, Kiro, Copilot e Cursor.

Os hooks de attention casam apenas o matcher `AskUserQuestion`, não `Bash|Read|Write|Edit` como o
credential-guard — o mecanismo de falha é o mesmo, a frequência de disparo é muito menor. É bug
real, não incidente aberto.

### Por que isto não é um port mecânico do `0c66ecb`

Duas restrições eliminam a solução óbvia ("emitir `$<CLI>_PROJECT_DIR/...` em todo lugar"):

- **Não há mecanismo uniforme entre os 6 CLIs.** Pesquisa preliminar indica que o Codex CLI não
  expõe env var de project-dir (o contexto chega por stdin JSON, campo `cwd`), e as fontes sobre o
  Cursor se contradizem entre "cwd = project root" e "caminho relativo ao `hooks.json`". Isso
  precisa ser verificado contra doc primária antes de qualquer linha de código.
- **Caminho absoluto resolvido na injeção não serve.** Os arquivos de settings dos CLIs
  (`.claude/settings.json` etc.) são versionados no repositório do usuário; gravar o caminho
  absoluto da máquina que rodou `trackfw init/update` quebraria o hook para qualquer outro
  desenvolvedor ou checkout. (O credential-guard de **escopo global** é caso distinto e já usa
  caminho absoluto legitimamente, pois vive fora do repo.)

Portanto o mecanismo de resolução é uma decisão de arquitetura, registrada em ADR **depois** da
verificação — o ADR documenta o que foi provado, não a hipótese.

## Acceptance Criteria

- [ ] Existe uma tabela de verificação, com **uma citação de doc primária por célula**, cobrindo os
      6 CLIs (Claude, Codex, Gemini, Kiro, Copilot, Cursor) e respondendo, para cada um: (a) qual é o
      cwd em que o comando do hook é executado; (b) esse cwd é estável na raiz do projeto ou
      dinâmico; (c) quais placeholders/env vars de raiz de projeto existem; (d) o caminho relativo é
      resolvido contra o cwd ou contra a localização do arquivo de settings.
- [ ] Existe um ADR aceito que decide o mecanismo de resolução **por CLI**, admitindo
      explicitamente mecanismos diferentes quando a verificação mostrar que não há um único, e que
      registra os CLIs em que nenhuma mudança é necessária.
- [ ] Todo CLI que a verificação provar quebrado passa a emitir comandos de hook que resolvem para a
      raiz do projeto independentemente do cwd, nos **3 CLIs do trackfw** (Go, Node.js, Python) —
      `docs/cli-parity.md` atualizado.
- [ ] Todo comando alterado tem migração in-place de entradas antigas (padrão
      `migrateClaudeHookCommand`), de modo que rodar `trackfw update` sobre um settings file gravado
      por versão anterior **reescreva** a entrada quebrada em vez de acrescentar uma segunda ao lado
      dela.
- [ ] Testes nos 3 stacks cobrem: comando novo emitido, migração de entrada antiga, e ausência de
      entrada duplicada após `update` repetido.
- [ ] `go test ./...`, `npm test`, `pytest` e `make quality` verdes; `trackfw validate` sem
      violações.

### Escopo negativo (explicitamente fora)

- **Não** altera o credential-guard do Claude Code (já corrigido em `0c66ecb`) além da migração já
  existente.
- **Não** altera o credential-guard de **escopo global** (`trackfw update harness`), que usa caminho
  absoluto por design e está fora do repo do usuário.
- **Não** altera o conteúdo dos scripts `scripts/trackfw-*.sh` — apenas como o **comando** é
  registrado nos arquivos de settings.
- **Não** adiciona matchers novos, nem muda quais eventos disparam quais hooks.
- **Não** endurece o guard de vacuidade P4 de `check-agent-hooks-parity.sh` (achado pendente
  registrado em 2026-08-08 no working context) — assunto separado.
- **Não** corrige o `settings.json` de projetos consumidores (ex.: CMDB); isso é resolvido pelo
  usuário rodando `trackfw update`.

## Linked ADR
<!-- ADR criado na Wave 1 (barreira), após a verificação da Wave 0 -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd.md
