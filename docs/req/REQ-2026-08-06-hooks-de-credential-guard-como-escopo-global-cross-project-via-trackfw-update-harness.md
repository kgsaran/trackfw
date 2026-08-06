---
status: Done
date: 2026-08-06
author: "kg.saran@gmail.com"
adr: "docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md"
---

# REQ: hooks de credential-guard como escopo global cross-project via trackfw update harness

> Date: 2026-08-06 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

O hook `trackfw-credential-guard.sh` (PR #141) foi construído reaproveitando o pipeline
`InjectXHooks`/`InjectHooksDetected` já existente para o mecanismo de attention-signal — que é
inerentemente **por-projeto**, porque o sinal que ele escreve (`.trackfw-attention.json`) pertence a
um roadmap/repo específico do `trackfw serve`. O credential-guard herdou esse escopo sem isso ter
sido uma decisão própria: hoje ele só é gerado/conectado quando alguém roda `trackfw init`/
`discover --init`/`update` **dentro de um projeto que já tem `trackfw.yaml`**.

Isso é uma lacuna real para o propósito do hook: o risco que ele mitiga (subagente materializando
credencial real em texto plano) existe em **qualquer projeto** que o usuário abra com um CLI de IA
com hooks (Claude Code, Cursor, etc.), com ou sem `trackfw.yaml`, com ou sem `trackfw init` já
rodado ali. Um usuário com dezenas de repositórios só fica protegido nos que ele lembrou de
inicializar com o trackfw — o oposto do que faz sentido para uma proteção de segurança.

O projeto já tem um comando de escopo explicitamente global para esse tipo de situação:
`trackfw update harness` (`internal/commands/update_harness.go`, `docs/cli-parity.md` seção
"`trackfw update` vs `trackfw update harness`") — hoje ele só cobre `claude-skill` e deployments de
agents/skills do catálogo de integrações (`internal/generators/update.go:HarnessTargetIDs`), nunca
hooks. Investigar se dá para estender esse comando (ou criar um alvo novo dentro dele) para instalar
o credential-guard em nível de usuário (`~/.claude/settings.json`, `~/.cursor/hooks.json`, etc.),
agindo cross-project sem depender de `trackfw.yaml` em cada repo.

## Acceptance Criteria
- [x] Confirmado, por CLI dos 6 da wave nativa, que existe nível de hooks global mesclado com o de
      projeto — os 6 (não só Claude Code e Cursor) suportam: Codex (`~/.codex/hooks.json`, prioridade
      2), Gemini CLI (`~/.gemini/settings.json`, hierarquia System→Workspace→User), GitHub Copilot
      (bloco `hooks` inline em `~/.copilot/settings.json`), Kiro (`~/.kiro/hooks/`, só na v3)
- [x] N/A — todos os 6 CLIs confirmados suportam escopo global; nenhum precisou ser excluído
- [x] Script global em `~/.trackfw/scripts/trackfw-credential-guard.sh` (ML-1A), reusando o núcleo de
      detecção do script de projeto sem duplicação
- [x] `trackfw update harness` ganha 6 alvos novos (`<tool>-credential-guard`, ML-2A-2F), mesmo
      contrato de 4 estados/`--targets`/`--install-missing`/`--dry-run`
- [x] Dedup por leitura implementado (ML-3A): `InjectXHooks` detecta wiring global e pula a entrada
      de credential-guard por-projeto, fail-open em qualquer erro de leitura, confirmado end-to-end
- [x] Paridade Go/Node/Python mantida — com um incidente corrigido no ML-2A (autoria do roadmap só
      listou Go inicialmente, corrigido com follow-up antes do commit; ver
      `feedback_roadmap_deve_listar_3_stacks` na memória do orquestrador)
- [x] `trackfw validate`/`make quality` sem regressão — `make quality` confirmado passando de ponta a
      ponta (103/103 cenários de falsificação, incluindo o gate estrutural novo do ML-4A)

## Linked ADR
ADR: `docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md`
— decide instalação opt-in (só via `trackfw update harness`, sem mudança de comportamento em
`init`/`update`), script global em `~/.trackfw/scripts/`, novos alvos por CLI em `HarnessTargetIDs`,
e dedup por leitura (projeto detecta wiring global e pula o wiring local do credential-guard).

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`
