---
status: Open
date: 2026-08-14
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md"
---

# REQ: bloqueio tecnico de comandos git brutos por subagente via deny/hooks nos 7 runtimes suportados

> Date: 2026-08-14 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Hoje o fluxo `trackfw req new` → `roadmap new` → `roadmap move wip` → `branch new` só é
enforced por convenção: os subagentes têm `Bash` liberado sem restrição e nada impede
tecnicamente que um agente especialista rode `git commit`/`git push`/`git checkout -b`
brutos, contornando o gate `branch_has_wip_roadmap` do `trackfw validate`. A única
barreira técnica real hoje é a branch protection do GitHub (bloqueia push direto na
`main`), que não cobre o momento de criação de branch nem substitui o roteamento pelos
comandos `trackfw branch new`/`trackfw ship`.

Levantamento confirmado contra a documentação oficial dos 7 runtimes que o trackfw
suporta (`internal/generators/agentfiles.go`: claude, codex, gemini, copilot, windsurf,
amazonq, cursor) mostra que cada um resolve isolamento por-agente de forma diferente:

- **Claude Code**: `permissions.deny` existe mas é sempre global ao projeto/usuário —
  não há escopo por subagente sem hook. Único mecanismo condicional por agente: hook
  `PreToolUse` inspecionando `subagent_name`.
- **Codex CLI**: Rules (`prefix_rule(decision="forbidden")`) bloqueiam por prefixo de
  comando de forma granular; hooks existem mas são experimentais e só cobrem shell.
- **Gemini CLI**: `tools.exclude` só bloqueia por prefixo bruto (nega `git` inteiro, não
  isola `git commit` de `git status`); hooks `PreToolUse` (exit code 2) dão controle
  fino; suporta subagentes com toolset restrito nativo.
- **GitHub Copilot**: `--deny-tool='shell(git push)'` é granular e tem precedência sobre
  allow; não há hooks documentados; custom agents (`.agent.md`) têm `tools` próprio.
- **Cursor**: `Shell(git:commit)` é granular; hook `beforeShellExecution` é o mecanismo
  mais robusto (allow/deny/ask condicional); doc oficial adverte que a allowlist é
  "best-effort, not a security boundary".
- **Windsurf**: deny list do Cascade tem precedência sobre allow; hook
  `pre_run_command` (exit code 2) bloqueia condicionalmente.
- **Amazon Q Developer**: `deniedCommands` com regex, avaliado antes do allow; hook
  `preToolUse` confirmado; custom agents com `tools`/`allowedTools` próprio.

Nenhum runtime tem garantia hermética documentada (Cursor e Amazon Q alertam
explicitamente sobre bypass via prompt injection / command chaining), então o objetivo
desta REQ é elevar o piso de enforcement de "convenção em markdown" para "bloqueio
técnico configurado", não prometer impossibilidade de bypass.

## Acceptance Criteria
- [ ] Cada um dos 7 geradores de agente (`internal/generators/agentfiles.go` e
      equivalentes em `npm/src/` e `pypi/trackfw/`, respeitando a regra de paridade de
      3 CLIs) passa a emitir, além do bloco de instrução textual já existente, a
      configuração técnica de deny/hook nativa daquele runtime para os comandos
      `git commit`, `git push` e `git checkout -b` brutos.
- [x] **Decisão registrada em 2026-08-14 (usuário, via AskUserQuestion pós-Wave 3):**
      isolamento arquiteto-livre/especialista-bloqueado fica como débito técnico
      documentado, não implementado nesta REQ. Nenhum dos 7 runtimes recebeu a
      diferenciação por `subagent_name`/subagente nativo nesta implementação — o deny é
      **global para todos os agentes, inclusive o arquiteto (Zeus/equivalente)**, em
      todos os 7 runtimes, sem exceção. Justificativa aceita: coerente com a regra já
      existente no CLAUDE.md de que o arquiteto não deveria commitar código de
      implementação mesmo (§3) — o arquiteto passa a usar `trackfw commit`/
      `trackfw branch new`/`trackfw ship` como qualquer outro agente. Se o isolamento
      por subagente vier a ser necessário no futuro, é uma REQ nova (hook
      `PreToolUse`/`pre_run_command` condicional por `subagent_name` em
      Claude/Codex/Windsurf + geradores de subagente nativo, hoje inexistentes no
      trackfw, para Gemini/Amazon Q).
- [ ] `make quality` (contratos de paridade Go/Node/Python) passa sem novas
      divergências entre os 3 CLIs.
- [ ] Testado manualmente em pelo menos Claude Code (ambiente disponível nesta sessão)
      confirmando que `git commit`/`git push`/`git checkout -b` brutos são bloqueados
      para um subagente especialista e que `trackfw branch new`/`trackfw ship`
      continuam funcionando.
- [ ] Documentação em `docs/cli-parity.md` ou equivalente registra o comportamento
      esperado por runtime (tabela de suporte), para que divergências futuras sejam
      reconhecidas como decisão consciente, não regressão.
- [ ] Novo comando `trackfw commit` (Go/Node/Python, paridade completa) recusa commit
      direto em `main`/branch protegida e recusa commit em branch `feat/fix/refactor`
      sem roadmap correspondente em `wip/`. Motivação direta: nesta sessão, um commit de
      artefatos de governança (REQ+roadmap) foi feito acidentalmente direto na `main`
      via `git commit` bruto (commit `cda74cd`, revertido antes do push) — nada no
      trackfw impediu isso no momento do commit, só depois manualmente. O deny/hook por
      runtime (critério acima) deve incluir `git commit` bruto, não só `checkout -b`/
      `push`, e orientar para `trackfw commit`.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md
