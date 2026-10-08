---
status: Open
date: 2026-10-06
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
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

**Escopo corrigido em 2026-10-08 (Wave 0, Regra Dura de Causa Raiz):** o guard não entendia o payload do Windsurf
(`command_line`), truncava argumento entre aspas e aplicava a isenção de `> /dev/null` ao payload de escrita inteiro —
medido: `Write`/`Edit` do Claude Code e `fs_write` do Amazon Q com JWT + `> /dev/null` no conteúdo saíam 0 em `block`.
Mesma causa (o guard não lê o payload de cada CLI pelo que ele é) → entra aqui como ML-1C.

**Escopo negativo:** não muda os padrões de detecção nem o modo block/warn do credential guard; não muda a regra de cwd; não muda o git-branch
guard; não trata outros CLIs; não reabre a decisão de modo avisador por padrão da ADR-2026-08-05.

## Acceptance Criteria
- [x] AC1 — Remedição, com fonte oficial e data: quais eventos de hook do Windsurf e do Amazon Q recebem o payload da tool call (comando e/ou conteúdo escrito) e se um exit code bloqueia ou só avisa
      ✅ Evidência: `docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md` (fontes: docs.devin.ai/desktop/cascade/hooks; aws/amazon-q-developer-cli docs/hooks.md e fs_write.rs, acesso 2026-10-08); 1º parecer reprovado por payload inventado, corrigido.
- [x] AC2 — Adendo à ADR-2026-08-05 revendo a premissa do Windsurf, com a decisão por CLI (instalar, ou manter fora com o motivo medido)
      ✅ Evidência: adendo e emenda de 2026-10-08 em `docs/adr/ADR-2026-08-05-…`: instalar em Windsurf pre_run_command/pre_write_code e Amazon Q execute_bash/fs_write; leituras fora (payload sem conteúdo).
- [x] AC3 — Para cada CLI decidido "instalar": `trackfw init`/`update` emitem o credential guard na forma da D2 revista da ADR-2026-10-04, e o `validate` deixa de silenciar o arquivo (teste nas duas direções)
      ✅ Evidência: PR #543 — `credential_guard_windsurf_amazonq_ml1b_test.go`, `validator_guard_binary_probe_windsurf_amazonq_test.go`, `TestCredentialGuardPresenceRequired_Windsurf_*`; medido com o binário: init instala, remover pre_write_code → validate acusa, update repõe.
- [x] AC5 — O guard lê `command`/`command_line` por JSON parse e a isenção de redirecionamento efêmero vale só para o comando de shell; teste com payload real de cada CLI nas duas direções
      ✅ Evidência: PR #543 — `credExtractCmdAndCwd`/`credDeepScan`/`credIsSimpleCmd` em `internal/guard/credential.go`; 32 combinações medidas com o binário (projeto/global × BOM); `windows-full-suites` verde após ML-2E.
- [x] AC4 — Prova de disparo real em ao menos um dos dois CLIs, ou a impossibilidade de prova declarada com o motivo
      ✅ Evidência: impossibilidade declarada (sem conta de Windsurf/Amazon Q); medido no lugar o binário com payload documentado e o artefato gerado (roadmap, ML-2A).
- [x] AC6 — Reaberto em 2026-10-08 pela issue #544: no Windows, a 2ª camada resolve o caminho na forma em que o CLI o escreve — Git Bash (`/c/Users/...`, `/C/...`), nativo (`C:\...`) e `C:/...` — tanto em argumento (`cat <caminho>`) quanto em alvo de redirecionamento (`> C:\...` era cortado no `:`); medido na VM e no CI de Windows lendo stderr, não só rc

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: `docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md` (premissa a rever);
`docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md` (forma da linha de hook).
Depende da REQ-2026-09-05 (a linha `trackfw guard credential` precisa existir).

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md
