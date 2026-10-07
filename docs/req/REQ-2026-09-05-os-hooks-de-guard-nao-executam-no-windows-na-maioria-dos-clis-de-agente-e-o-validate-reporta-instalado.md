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

- [x] **AC1** — **Copilot:** o hook passa a ser lido no Windows. A entrada emitida usa o campo
      `command` com `trackfw guard <nome>`, e não só `bash`. Falsificação: com só `bash`, um teste de
      schema afirma que o campo `powershell`/`command` está ausente; com o novo, presente.
      ✅ Evidência: Copilot emite o campo `command` com `trackfw guard <nome>; exit $LASTEXITCODE` e não emite `bash`/`powershell` (testes do `InjectCopilotHooks`, ML-2A; PR #527).
- [x] **AC2** — **Um guard, em Go:** `trackfw guard git-branch` e `trackfw guard credential` existem
      e reproduzem o comportamento do `.sh` (payload pelo stdin, exit 2 + stderr no bloqueio, saída
      `hookSpecificOutput` por CLI, no-op fora de projeto, falha fechada da ADR-2026-10-02).
      ✅ Evidência: `trackfw guard git-branch` e `trackfw guard credential` em `internal/guard/` (ML-1A/1B); paridade com o `.sh` congelado pela suíte do ML-1C; D7 (todo erro sob `guard` sai 2) corrigida no ML-4C.
- [ ] **AC3** — 🔴 **A prova é o guard DISPARANDO e BLOQUEANDO**, não o arquivo existindo. Medido na
      VM Windows, nas cadeias de shell que os CLIs usam (`powershell -NoProfile -Command`, `pwsh`,
      `cmd /c`, Git Bash): `git push` bruto bloqueado com exit 2, comando inofensivo liberado. E nos
      CLIs de agente instalados na VM, pelo menos em um de PowerShell e no Kiro (`cmd.exe`).
      ⚠️ **Parcial.** Shells medidos na VM (PowerShell 5, `cmd`, Git Bash: 2/0; ML-3A). CLI real: **Claude Code 2.1.292** (ML-5B, rodada B: `PreToolUse:PowerShell` negou o `git push` com a REASON do guard; `git status` executou) e **Codex CLI 0.160.1** (ML-5C, teste interativo do KG: "Blocked by hook"; `git status` executou). **Kiro NÃO MEDIDO** — sem CLI para Windows ARM64 e o IDE exige conta AWS Builder ID; decisão do KG em 2026-10-07. Copilot e Amazon Q não verificados (sem conta).
- [x] **AC4** — 🔴 **Paridade comportamental `.sh` ↔ Go, por gate.** O corpus de testes que hoje
      exercita o `.sh` passa a exercitar também o `trackfw guard`, com o mesmo veredito em todo
      cenário. Divergência é falha, não nota.
      ✅ Evidência: o corpus roda nos dois braços contra a fixture congelada (`internal/generators/testdata/guard-sh-reference/`); trocar a REASON no Go reprova. Divergências deliberadas, todas no sentido seguro e nomeadas: N09 no Windows (`filepath.Base` com `\`), C2 (`&` separa comando), D10 (aspas na palavra de comando).
- [x] **AC5** — 🔴 **Controle POSIX:** Linux e macOS sem regressão, medidos antes e depois (testes do
      guard e `make quality`).
      ✅ Evidência: `make quality` EXIT=0 pelo arquiteto, inclusive sem `~/.local/bin` no PATH; CI do PR #527 e do PR #528 verdes (20/20).
- [x] **AC6** — **Todos os CLIs que o `agentfiles.go` emite**, não só os 6 da tabela de 2026-09-05:
      inclui Windsurf e Amazon Q. Cursor e Kiro foram medidos no adendo de 2026-09-06 e recebem a
      mesma string.
      ✅ Evidência: os 8 `Inject*Hooks` emitem a linha da D2 revista (ML-2A); o `validate` lê também Windsurf e Amazon Q (ML-2C); teste de concordância gerador↔validator por CLI (ML-4C).
- [ ] **AC7** — 🔴 **O `validate` relata se o hook pode executar.** Config apontando para `.sh` num
      contexto em que ele não executa, ou `trackfw` resolvido sem o subcomando `guard`, não é
      "instalado".
      ✅ Evidência: linha exata por família, sonda de `trackfw guard --help`, `.ps1` sob `Restricted`, `trackfw.exe/.cmd/.bat` na raiz (ML-2B); matcher sem `PowerShell` no Claude Code (ML-5A); `trackfw` sem `guard` resolvido pelo Git Bash de login — medido na VM: 2 violations (ML-5D).
      🔴 Reaberto em 2026-10-07 pela issue #530: a regra de matcher do ML-5A avisa nos grupos `Read` e `Write|Edit` que o próprio `init` escreve, e o `trackfw update` não limpa o aviso. Corrigido no ML-5F: a regra só avalia grupos de shell (matcher com `Bash`); a saída canônica do `init` não gera aviso (PR #533, fecha #530; lançado na 9.3.1).
      🔴 Reaberto em 2026-10-07 pela issue #535: com o `trackfw` ausente do PATH de quem executa o hook (app GUI, máquina nova), a linha sai 127 — erro não bloqueante — e o guard desliga sem aviso. O `validate` roda no PATH do terminal e não vê esse caso. Wave 6.
- [x] **AC8** — **Os `.sh` viram invólucro** (`exec trackfw guard <nome> "$@"`) nas 4 cópias
      (projeto, global, template do `scaffold.go` e o que mais a Wave 0 enumerar). O `trackfw update`
      migra as configs existentes para a string nova.
      ✅ Evidência: os `.sh` são invólucros que falham fechado (sem `trackfw` ou sem `guard` → 2) e fazem `exec trackfw guard`; `trackfw update` migra as formas antigas e o matcher `Bash` → `Bash|PowerShell` (ML-2A, ML-5A).
- [x] **AC9** — 🔴 **Os riscos da D5 da ADR-2026-10-04 medidos antes da implementação:** shim do npm
      sob `ExecutionPolicy Restricted`, binário sem o subcomando (falha aberta) e `trackfw` fora do
      `PATH`. Cada um tem veredito escrito e, se for o caso, mitigação no roadmap.
      ✅ Evidência: Wave 0 — `docs/seguranca/2026-10-04-wave0-guard-em-go.md` (ML-0A) e `docs/portabilidade/2026-10-04-…-por-canal.md` (ML-0B), com veredito por risco e mitigação no roadmap.

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
