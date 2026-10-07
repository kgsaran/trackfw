---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-22-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md"
---

# REQ: os hooks de guard nao executam no Windows na maioria dos CLIs de agente e o validate reporta instalado

> Date: 2026-09-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Governada por
`docs/adr/ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-em-vez-de-exigir-git-bash.md`.
Medição completa em
`docs/portabilidade/2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md`.

**Medido, não inferido** — leitura do código-fonte dos fornecedores e documentação oficial:

| CLI | shell no Windows | `.sh` dispara? |
|---|---|---|
| Gemini CLI | PowerShell, sempre | **não** |
| Codex CLI | PowerShell no caminho comum | **não** |
| GitHub Copilot CLI | — | **não** — populamos `"bash"`, ele lê outro campo |
| Claude Code | Git Bash se instalado | **condicional** |
| Cursor · Kiro | — | **indeterminado** |

🔴 **O `trackfw validate` reporta esses hooks como instalados.** O controle que existe para impedir
`git push` bruto por subagente reporta saúde sobre o que nunca inspecionou — na plataforma inteira.

**Contexto de negócio, do usuário:** *"hoje só atendemos usuários de Linux e macOS. Se algum usuário
corporativo tentar usar o trackfw vai se deparar com essa enxurrada de erros e vai desistir do
framework."* Um produto de governança que não governa no Windows não tem segunda chance com esse
usuário.

## Revisão de 2026-10-04 — o desenho mudou antes do código

As configs de hook são **versionadas**, e a mesma string é lida pelo shell que cada CLI de agente
escolhe, em cada máquina do time. Escolher `.sh`/`.ps1` no `init` ou no hook quebra o time misto.
A ADR-2026-10-04 decide: o guard vira **`trackfw guard <nome>`** (Go), e a linha de hook é a mesma
string em `sh`, Git Bash, PowerShell e `cmd.exe`. Os ACs abaixo foram reescritos para esse desenho.
Os originais pediam um `.ps1` (AC2) e paridade nos 3 CLIs (AC8), que deixaram de existir na v8.

## Acceptance Criteria

- [ ] **AC1** — **Copilot:** o hook passa a ser lido no Windows. A entrada emitida usa o campo
      `command` com `trackfw guard <nome>`, e não só `bash`. Falsificação: com só `bash`, um teste de
      schema afirma que o campo `powershell`/`command` está ausente; com o novo, presente.
- [ ] **AC2** — **Um guard, em Go:** `trackfw guard git-branch` e `trackfw guard credential` existem
      e reproduzem o comportamento do `.sh` (payload pelo stdin, exit 2 + stderr no bloqueio, saída
      `hookSpecificOutput` por CLI, no-op fora de projeto, falha fechada da ADR-2026-10-02).
- [ ] **AC3** — 🔴 **A prova é o guard DISPARANDO e BLOQUEANDO**, não o arquivo existindo. Medido na
      VM Windows, nas cadeias de shell que os CLIs usam (`powershell -NoProfile -Command`, `pwsh`,
      `cmd /c`, Git Bash): `git push` bruto bloqueado com exit 2, comando inofensivo liberado. E nos
      CLIs de agente instalados na VM, pelo menos em um de PowerShell e no Kiro (`cmd.exe`).
- [ ] **AC4** — 🔴 **Paridade comportamental `.sh` ↔ Go, por gate.** O corpus de testes que hoje
      exercita o `.sh` passa a exercitar também o `trackfw guard`, com o mesmo veredito em todo
      cenário. Divergência é falha, não nota.
- [ ] **AC5** — 🔴 **Controle POSIX:** Linux e macOS sem regressão, medidos antes e depois (testes do
      guard e `make quality`).
- [ ] **AC6** — **Todos os CLIs que o `agentfiles.go` emite**, não só os 6 da tabela de 2026-09-05:
      inclui Windsurf e Amazon Q. Cursor e Kiro foram medidos no adendo de 2026-09-06 e recebem a
      mesma string.
- [ ] **AC7** — 🔴 **O `validate` relata se o hook pode executar.** Config apontando para `.sh` num
      contexto em que ele não executa, ou `trackfw` resolvido sem o subcomando `guard`, não é
      "instalado".
- [ ] **AC8** — **Os `.sh` viram invólucro** (`exec trackfw guard <nome> "$@"`) nas 4 cópias
      (projeto, global, template do `scaffold.go` e o que mais a Wave 0 enumerar). O `trackfw update`
      migra as configs existentes para a string nova.
- [ ] **AC9** — 🔴 **Os riscos da D5 da ADR-2026-10-04 medidos antes da implementação:** shim do npm
      sob `ExecutionPolicy Restricted`, binário sem o subcomando (falha aberta) e `trackfw` fora do
      `PATH`. Cada um tem veredito escrito e, se for o caso, mitigação no roadmap.

## Negative Scope

- ❌ **Não** exigir Git Bash como remédio — D1 da ADR-2026-09-05.
- ❌ **Não** escrever guard em PowerShell (`.ps1`) — D7 da ADR-2026-09-05, substituída.
- ❌ **Não** portar `attention-signal`/`attention-cleanup` nesta REQ. São conveniência (D4 da ADR
  anterior); continuam `.sh`, e o `validate` os declara como não executáveis no Windows.
- ❌ **Não** melhorar a lógica dos guards ao portar. Comportamento **igual** é o que torna a paridade
  verificável; as evasões declaradas no cabeçalho do `.sh` continuam declaradas.
- ❌ **Não** tratar aqui a jornada de instalação em Windows (`install.sh`, README, ARM64) —
  REQ-2026-09-05 da instalação, própria.
- ❌ **Não** tratar aqui o local da config global (`$HOME` ≠ `%USERPROFILE%`) — REQ-2026-09-01. Teste
  de causa: o guard em Go elimina o **caminho do script**, mas não o **lugar onde a config global é
  escrita**, que é o defeito daquela REQ.

## Linked ADR
ADR: docs/adr/ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-em-vez-de-exigir-git-bash.md
ADR: docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md`
