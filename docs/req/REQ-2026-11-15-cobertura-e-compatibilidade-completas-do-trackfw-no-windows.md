---
status: Open
date: 2026-11-15
author: "zeus-tf"
adr: "ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-em-vez-de-exigir-git-bash"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md"
---

# REQ: Cobertura e compatibilidade completas do trackfw no Windows

> Date: 2026-11-15 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Pedido do KG em 2026-11-14: *"o que falta para termos uma cobertura e compatibilidade completa no windows pois
nem todos os usuários irão rodar o trackfw no macOS."* Esta REQ foi aberta depois do merge do D18
(REQ-2026-11-13, PR #687), que tocava os mesmos arquivos de teste.

O que se mediu em 2026-11-14 no run 43423202733 da `main`, job `windows-full-suites`, com `go test -v`:

- O log tem 102 linhas `--- SKIP` (71 testes únicos pulados) e 5 `--- FAIL`. As 5 falhas são, por nome, as de
  `.github/windows-known-failures.json`.
- 41 dos 71 testes pulados são `TestMLScope*` (`barrier_ml_scope_test.go`), com a justificativa "windows path
  handling tested separately". Não existe esse teste separado: os 30 testes de parsing passam no Windows, e os 41
  que usam git nunca rodaram lá. O skip vem do PR #606 (2026-10-24).
- `TestBarrierReentry_T1/T2/T3/T7/T7e` pulam com "Windows coberto por windows-known-failures.json", mas a lista
  não tem nenhuma entrada de reentry. É a segunda justificativa falsa.
- Vários skips alegam "shells POSIX não disponíveis em Windows". O `windows-latest` tem Git Bash, e o
  consumidor com Git for Windows também. A justificativa nunca foi medida.
- `TestPush_ForceWithLease_NoPROpen_Blocks` declara "no end-to-end coverage".
- No consumidor:
  - `scripts/trackfw-attention-signal.sh`, `attention-cleanup.sh` e `trackfw-validate.sh` são gerados só
    em `.sh`.
  - A ADR-2026-09-05 (hook nativo por CLI) declarou Cursor e Kiro sem medição no Windows.
- Na distribuição, o `install.ps1` tem teste. Não há smoke de `npm install` nem de `pip install` no
  `windows-latest`, e o binário windows/arm64 é compilado e nunca executado.
- O D18 deixou um resíduo: o prompt de Confirm que drena o stdin do guard (forma K, corrigida na forma L) só foi
  medido no `pwsh` 7. Não foi medido no PS 5.1 sem `-NonInteractive` nem nos outros CLIs. O braço via perfil do
  `TestGuardLineShellMatrix_PSInheritedState` retorna cedo no Windows (XDG).

## Definição de "completo" (mensurável)

1. **Testes:** todo skip de Windows e toda known-failure recebe uma de três classificações: (a) asserção
   equivalente no Windows, (b) corrigido, ou (c) impossível na plataforma, com prova escrita. Nenhuma
   justificativa falsa fica de pé, e um gate impede skip novo sem classificação.
2. **Compatibilidade do consumidor:** cada artefato gerado e cada hook por CLI (Claude, Codex, Gemini, Copilot,
   Cursor, Kiro, Windsurf, Amazon Q) executa no Windows sem Git Bash, ou o `validate`/`doctor` declara que não
   executa.
3. **Distribuição:** os 3 canais (npm, PyPI, GitHub/`install.ps1`) têm smoke no `windows-latest`; o arm64 é
   executado se houver runner.
4. **PowerShell 5.1 e 7 cobertos**, inclusive via perfil e sem `-NonInteractive`.

## Escopo negativo

- Tooling de contribuidor no Windows (`make quality`, `scripts/*.sh` de desenvolvimento).
- O D18 (REQ-2026-11-13), que está mergeado e encerrado. Se um resíduo dele tiver a mesma causa de um achado
  desta REQ, entra aqui como ML.
- Feature nova.
- Mudança de comportamento em macOS/Linux além do necessário para a asserção equivalente.

## Acceptance Criteria
- [ ] AC1 — inventário da Wave 0: cada skip e cada known-failure de Windows, cada artefato gerado e cada hook por CLI, e cada canal de distribuição, com a classificação (a)/(b)/(c) e a evidência
- [ ] AC2 — zero skip de Windows com justificativa falsa; `TestMLScope*` e `TestBarrierReentry_*` rodam no Windows ou têm prova (c)
- [ ] AC3 — gate no CI reprova um skip novo de Windows sem classificação
- [ ] AC4 — cada artefato gerado e cada hook por CLI executa no Windows sem Git Bash, ou o `validate`/`doctor` declara que não
- [ ] AC5 — smoke de `npm install -g` e `pip install` no `windows-latest` executando `trackfw --version` e `trackfw validate`
- [ ] AC6 — guard via perfil e sem `-NonInteractive` medido no PS 5.1 e no `pwsh` 7 do Windows
- [ ] AC7 — `windows-full-suites` verde; `make quality` verde

## Linked ADR
ADR: docs/adr/ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-em-vez-de-exigir-git-bash.md
Relacionadas: ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem,
ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell,
ADR-2026-10-14-no-windows-o-guard-global-de-cada-cli-de-agente-e-gravado-e-auditado-no-home-que-aquele-cli-le

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md`
