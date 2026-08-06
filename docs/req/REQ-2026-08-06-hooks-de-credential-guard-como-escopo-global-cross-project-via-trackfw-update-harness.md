---
status: Open
date: 2026-08-06
author: "kg.saran@gmail.com"
adr: ""
roadmap: ""
---

# REQ: hooks de credential-guard como escopo global cross-project via trackfw update harness

> Date: 2026-08-06 | Status: Open
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
- [ ] Confirmado, por CLI dos 6 da wave nativa (Claude Code, Codex, Gemini CLI, GitHub Copilot,
      Cursor, Kiro), se existe um nível de configuração de hooks em escopo de usuário/global que é
      **mesclado** (não sobrescrito) com o nível de projeto — não assumir a partir da investigação
      preliminar (só Claude Code e Cursor foram confirmados informalmente nesta sessão, os outros 4
      não)
- [ ] Para os CLIs sem suporte a hooks globais: decisão documentada — ficam de fora do escopo global
      (mantêm só o wiring por-projeto já existente), não é bloqueante para os que suportam
- [ ] Caminho do script resolvido para um local estável fora do repositório (ex.: `~/.trackfw/scripts/`
      ou equivalente) — hoje `scripts/trackfw-credential-guard.sh` é relativo à raiz do projeto, não
      funciona referenciado por um hook global
- [ ] `trackfw update harness` ganha alvo(s) novo(s) para o credential-guard nos CLIs confirmados,
      seguindo o mesmo padrão de `--targets`/`--install-missing`/`--dry-run`/relatório já existente
      (`generators.UpdateReport`)
- [ ] Decisão registrada sobre convivência entre wiring global (novo) e wiring por-projeto (já
      existente, PR #141) — evitar duplicidade (hook disparando duas vezes) quando um projeto tem
      ambos
- [ ] Paridade Go/Node/Python mantida desde o primeiro commit
- [ ] `trackfw validate`/`make quality` sem regressão

## Linked ADR
ADR: `docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md`
— decide instalação opt-in (só via `trackfw update harness`, sem mudança de comportamento em
`init`/`update`), script global em `~/.trackfw/scripts/`, novos alvos por CLI em `HarnessTargetIDs`,
e dedup por leitura (projeto detecta wiring global e pula o wiring local do credential-guard).

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`
