---
status: Open
date: 2026-11-17
author: ""
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-11-17-agents-install-com-scope-global-reescreve-arquivos-de-regras-e-hooks-do-projeto-no-cwd.md"
---

# REQ: agents install com scope global reescreve arquivos de regras e hooks do projeto no cwd

> Date: 2026-11-17 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

`trackfw agents install --scope global` (e `skills install`; `global` é o default de `--scope`) instala o catálogo no
home do usuário, mas em seguida chama, sem olhar o escopo, `generators.InjectRulesForTool(target, manager.ProjectRoot)`
e `generators.InjectHooksForTools(opts.targets, manager.ProjectRoot)` (`internal/commands/integrations_flags.go:277-289`).
Resultado: os arquivos de regras (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, ...) e os hooks do **projeto em que o
comando foi rodado** são reescritos, mesmo quando o usuário pediu escopo global, e mesmo quando o cwd não é o projeto
que ele quis tocar.

**Medido em 2026-11-17** (red team ML-FA da REQ-2026-11-15): o hades isolou `HOME`, `USERPROFILE` e `CODEX_HOME` em
`mktemp -d`, rodou `agents install --targets codex --scope global --items architect --force` com o cwd na raiz do
repositório, e o `CLAUDE.md` e o `AGENTS.md` do trackfw foram reescritos (3 linhas do bloco de governança). Reproduzido
pelo arquiteto num projeto temporário com o binário do HEAD: rc=0, `AGENTS.md` idêntico ao da árvore alterada.

O comentário do código diz que o comportamento restaura o dos aliases removidos (ML-5E do
ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador). A pergunta desta REQ é se isso vale para o escopo
global — que, pelo nome, promete não tocar no projeto.

## Escopo negativo
- Não muda o que `--scope project` escreve.
- Não muda `trackfw init` nem `trackfw update` (eles são, por contrato, comandos de projeto).
- Não muda o conteúdo do bloco de regras gerado, só **quando** ele é escrito.

## Acceptance Criteria
- [ ] Contrato decidido e escrito em `docs/cli-parity.md`: com `--scope global`, o install escreve ou não as regras e os
  hooks do projeto; se escreve, o comando avisa qual projeto tocou
- [ ] Teste com braço e contra-braço: `agents install --scope global` num cwd com `CLAUDE.md`/`AGENTS.md` versionados
  produz exatamente o efeito do contrato; `--scope project` mantém o comportamento atual
- [ ] Varredura dos outros comandos de escopo global que chamam `InjectRulesForTool`/`InjectHooksForTools`/
  `InjectRulesDetected` com o cwd, com o mesmo contrato aplicado (mesma causa, mesma REQ)

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-11-17-agents-install-com-scope-global-reescreve-arquivos-de-regras-e-hooks-do-projeto-no-cwd.md
