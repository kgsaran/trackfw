---
status: Open
date: 2026-07-25
author: "Zeus"
adr: "ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills"
roadmap: "ROADMAP-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills"
---

# REQ: Escopo de instalação selecionável para agents e skills

> Date: 2026-07-25 | Status: Open

## Motivation

`trackfw agents install` e `trackfw skills install` gravam os artefatos silenciosamente no
projeto atual (`.claude/agents/`, `.claude/skills/`), sem consultar o usuário. A flag
`--scope` tem default fixo `"project"` nos três CLIs e não existe nenhum prompt de escopo —
nem mesmo no caso mais comum (`--targets claude`), que hoje não passa por prompt algum.

O comportamento esperado é que agentes e skills sejam instalados na **pasta do usuário**
(`~/.claude/...`) por padrão, ou que o usuário seja **perguntado** sobre onde instalar.

## Acceptance Criteria

- [ ] Em TTY, sem `--scope`, os comandos `agents|skills install|update|uninstall` perguntam
      o escopo, com `global` pré-selecionado
- [ ] O prompt de escopo dispara mesmo quando `--targets` é informado (gate independente)
- [ ] Sem TTY e sem `--scope`, o escopo resolvido é `global` (`~/.claude/...`)
- [ ] `--scope project` e `--scope global` explícitos são respeitados e **não** disparam prompt
- [ ] A detecção de "escopo não informado" usa *flag-set*, não comparação de valor
- [ ] `trackfw init` também pergunta o escopo no wizard; sem TTY → `global`
- [ ] Os caminhos de destino resolvidos são impressos antes da gravação (fora do modo `--json`)
- [ ] `agents|skills list` adota o mesmo default `global` (sem perguntar)
- [ ] Comportamento idêntico nos 3 CLIs (Go, Node.js, Python) — `make quality` passa
- [ ] Breaking change documentado no `CHANGELOG.md` e em `docs/cli-parity.md`

## Linked ADR

ADR: `docs/adr/ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`
