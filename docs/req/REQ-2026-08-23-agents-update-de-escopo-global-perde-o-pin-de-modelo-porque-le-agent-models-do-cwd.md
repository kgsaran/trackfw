---
status: Open
date: 2026-08-23
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-23-agents-update-de-escopo-global-resolve-o-pin-de-modelo-da-config-global.md"
---

# REQ: `agents update` de escopo global perde o pin de modelo porque lê `agent_models` do cwd

> Date: 2026-08-23 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

**Bug reportado por KG em uso real (2026-08-23), com impacto diário de cota.**

Depois de `trackfw update`, `update harness` e `agents update --force` com a 7.2.0 instalada:

```
~/.claude/agents/*.md   ->  model: opus (1) · model: sonnet (11)     <- alias, nao pin
~/.trackfw/             ->  identity.json, integrations-manifest.json, scripts/
                            NAO existe ~/.trackfw/trackfw.yaml
```

Rodando o **mesmo comando** de dentro do repositório do trackfw, que tem `agent_models`:

```
$ ./bin/trackfw agents update --force --scope global --targets claude
model: claude-opus-5 (1) · model: claude-sonnet-4-6 (11)             <- pin aplicado
```

**O código funciona.** `config.Load()` (`internal/config/config.go:125`) lê `trackfw.yaml` do
**diretório corrente**, sem fallback global — então artefato de escopo **global** é renderizado com a
config do diretório onde o comando foi rodado. De qualquer outro lugar, cai no tier canônico, **em
silêncio**.

**Isto contradiz o AC6 da `REQ-2026-08-21`**, marcado `[x]`: *"Catálogo pina as versões escolhidas;
`agents update` **reforça** o pin."* Para o alvo que motivou aquela REQ — o Claude Code — a promessa
não se cumpre.

Decisão de desenho em
`ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md`.

## Acceptance Criteria

- [ ] **AC1** — Instalação/atualização de artefato de **escopo global** resolve `agent_models` a
      partir de `~/.trackfw/trackfw.yaml`, **independente do diretório de invocação** — provado
      rodando de pelo menos dois cwd diferentes, um deles sem `trackfw.yaml`.
- [ ] **AC2** — Precedência explícita e documentada quando a chave existe nos dois lugares (global e
      projeto), com o motivo registrado. **A Wave 0 ataca essa escolha antes da implementação.**
- [ ] **AC3** — Escopo de **projeto** continua lendo o `trackfw.yaml` do projeto — sem regressão.
- [ ] **AC4** — Instalação de escopo global **sem** `agent_models` resolvido emite aviso visível na
      saída, distinguindo "não configurado" de "configurado em lugar que não vale para escopo
      global". Silêncio deixa de ser resposta.
- [ ] **AC5** — `trackfw agents models` mostra a **origem** da resolução (qual arquivo forneceu a
      versão, ou a ausência dela).
- [ ] **AC6** — Ausência de config global **não quebra nada**: comportamento continua sendo o tier
      canônico, como hoje.
- [ ] **AC7** — Paridade nos **3 CLIs**, com gate comparando **saídas reais**.
- [ ] **AC8** — Falsificação em **duas direções**: (a) escopo global voltando a ler do cwd é
      detectado; (b) escopo de projeto passando a ler do global é detectado.
- [ ] **AC9** — `docs/cli-parity.md` com o contrato de resolução por escopo e anotação
      `trackfw-contract`; checker de cobertura exit 0.
- [ ] **AC10** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0, com **exit code medido**.

## Negative scope

- **Não** generaliza o carregador de config global para outras chaves. Começa por `agent_models`; a
  extensão vem quando houver caso.
- **Não** muda os mapeamentos de Codex, Cursor, Antigravity ou qualquer outro alvo.
- **Não** muda quem é `opus` e quem é `sonnet` — tier por agente é outra decisão.
- **Não** bloqueia `agents update --scope global` na ausência de config global (o ADR rejeita).
- **Não** mexe em `identity.json` nem no manifesto de integrações.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-23-agents-update-de-escopo-global-resolve-o-pin-de-modelo-da-config-global.md`
