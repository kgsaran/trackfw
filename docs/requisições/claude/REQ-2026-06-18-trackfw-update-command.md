---
name: REQ-2026-06-18-trackfw-update-command
title: "trackfw update — atualizar regras, gates e artefatos gerenciados"
status: Proposed
date: 2026-06-18
author: Zeus (orquestrador)
---

# REQ-2026-06-18: trackfw update

## Contexto

Após `trackfw init` ou `trackfw discover --init`, o projeto fica com um conjunto de artefatos gerados: regras nos arquivos de agente, gates (hooks e CI), slash commands Claude e skill global. Quando o usuário atualiza o binário do trackfw para uma versão mais nova, esses artefatos ficam desatualizados — as novas regras, templates de CI e slash commands permanecem na versão antiga.

## Necessidade

Precisamos de um comando `trackfw update` que re-aplique todos os templates atuais (embutidos no binário) sobre um projeto já inicializado, sem destruir customizações do usuário.

## Critérios de Aceite

1. `trackfw update` (nos 3 CLIs) atualiza o bloco `<!-- trackfw:rules:start/end -->` em todos os arquivos de agente detectados.
2. Atualiza `scripts/trackfw-validate.sh` com o template atual (trackfw-owned).
3. Atualiza o workflow CI (`.github/workflows/trackfw-gate.yml` ou `.gitlab-ci-trackfw.yml`) com o template atual (trackfw-owned).
4. Atualiza `.claude/commands/trackfw/*.md` — o diretório inteiro é trackfw-owned.
5. Atualiza `~/.claude/skills/trackfw/SKILL.md` — o arquivo é trackfw-owned.
6. Para git hooks (`.husky/pre-commit`, `lefthook.yml`) — comportamento **cirúrgico**: garantir que `trackfw validate` está presente, sem sobrescrever conteúdo do usuário.
7. Exibe resumo do que foi atualizado vs. já estava atual.
8. Falhas parciais (ex.: arquivo sem permissão) são logadas como avisos sem abortar o comando.
9. Paridade nos 3 CLIs: Go (completo), Node.js (completo), Python (escopo reduzido: apenas regras de agente + validação da ausência de trackfw.yaml).

## Escopo Explícito

**Inclui:**
- Inject-or-update de regras em arquivos de agente
- Regeneração de arquivos trackfw-owned (validate script, CI workflow, Claude commands, skills)
- Atualização cirúrgica de git hooks (inject idempotente)
- Leitura de `trackfw.yaml` para reconstruir `Config` (hooks, ci, backend, frontend, pkg_manager)

**Exclui:**
- Atualização de `trackfw.yaml` em si (contém paths/config do projeto)
- Migração de schema de `trackfw.yaml` (campo novo → adicionar, nunca remover)
- Tracking de versão instalada (nice-to-have, não neste ciclo)
- Dry-run / `--what` flags (nice-to-have, não neste ciclo)

## Restrição Python

O CLI Python tem `init_gen.py` parcialmente implementado (sem geração de gates/hooks completa). O `trackfw update` Python faz apenas: inject de regras de agente + avisa que gates e commands precisam do CLI Go/Node.
