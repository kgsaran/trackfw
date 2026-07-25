---
status: Accepted
date: 2026-07-25
author: "Zeus"
---

# ADR: Escopo de instalação selecionável para agents e skills

> Date: 2026-07-25 | Status: Accepted

## Context

Os comandos `trackfw agents install|update|uninstall` e `trackfw skills install|update|uninstall`
gravam os artefatos **silenciosamente no projeto atual** (`.claude/agents/`, `.claude/skills/`).
O usuário nunca é consultado sobre onde instalar, e o comportamento contradiz a expectativa
natural de que agentes e skills sejam recursos do **ambiente do usuário**, reaproveitáveis
entre projetos.

A causa é um default fixo `"project"` na flag `--scope`, replicado nos três CLIs:

| CLI | Arquivo:linha |
|---|---|
| Go | `internal/commands/integrations_flags.go:105` |
| Node | `npm/src/commands/integrations.js:50` |
| Python | `pypi/trackfw/integrations/command.py:94` (+ `catalog.py:59`) |
| Go (`init`) | `internal/commands/init.go:358` — `Scope: "project"` hardcoded |

Nenhum dos CLIs possui prompt de escopo. O único prompt existente
(`promptIntegrationSelection`, Go `integrations_flags.go:343`) pergunta apenas CLIs alvo e
itens, e **só dispara quando `--targets` está vazio** — ou seja, o caso mais comum
(`trackfw agents install --targets claude`) não passa por prompt algum.

Todos os 11 surfaces do catálogo (`internal/integrations/assets/catalog.json`) declaram
`"scopes": ["global", "project"]` — não há restrição técnica por target.

## Decision

O escopo de instalação passa a ser uma **escolha explícita do usuário**, com `global`
(`~/.claude/...`) como padrão.

- **D1 — Não-interativo (sem TTY) e sem `--scope`:** default `global`.
  É um *breaking change* em relação ao comportamento atual (`project`).
- **D2 — Interativo (TTY) e sem `--scope`:** perguntar o escopo, com `global`
  pré-selecionado. O prompt é um **gate independente** da seleção de targets — deve
  disparar mesmo quando `--targets` for passado.
- **D3 — `--scope` explícito:** sempre respeitado, nunca pergunta. A detecção de
  "usuário não escolheu" **deve** usar *flag-set* (`cmd.Flags().Changed("scope")` no Go,
  `undefined` no Node, `default=None` no Python) — comparar contra o valor `"project"`
  é incorreto, pois não distingue um `--scope project` explícito do default.
- **D4 — `trackfw init`:** o wizard também pergunta o escopo; sem TTY → `global`.
- **D5 — Transparência:** sem confirmação adicional, mas os **caminhos de destino
  resolvidos** são impressos antes da gravação.
- **D6 — `list`:** não pergunta (comando de leitura), porém adota o mesmo default `global`,
  para não reportar deployments divergentes dos que o `install` gravou.
- **D7 — Paridade:** implementado de forma idêntica nos três CLIs, conforme a regra dura
  de paridade (`docs/cli-parity.md`).
- **D8 — `uninstall` não herda o default `global` em modo não-interativo.** D1 foi decidido
  no enquadramento "onde **instalar**". Aplicá-lo uniformemente a `uninstall` produziria
  uma consequência não aprovada: um script de CI executando
  `trackfw agents uninstall --targets claude` — que hoje remove `.claude/agents/trackfw-*.md`
  do repositório — passaria a **apagar arquivos do diretório home do usuário**.
  Verificado empiricamente após o ML-1A: os `destination` resolvidos vinham como
  `~/.claude/agents/trackfw-*.md`.
  Portanto, em `uninstall` **sem TTY e sem `--scope`**, o comando **falha** exigindo
  `--scope` explícito, reutilizando o precedente já existente no código
  (`"requires --targets in non-interactive mode"`). Em TTY, `uninstall` pergunta
  normalmente como `install`/`update` (o usuário vê a escolha antes de destruir).
  Justificativa: para operações destrutivas, o argumento de "degradação suave" que sustenta
  D1 se inverte — errar o escopo em `install` cria arquivos no lugar errado; errar em
  `uninstall` destrói arquivos do usuário.

## Consequences

**Positivas**
- Elimina instalação surpresa no repositório de trabalho do usuário.
- Agentes e skills instalados uma vez passam a valer para todos os projetos, que é o
  modelo mental correto para um catálogo de agentes.
- Os caminhos impressos antes da gravação tornam o efeito do comando auditável.

**Negativas / riscos**
- **Breaking change:** pipelines de CI que executam `agents install --targets X` sem
  `--scope` passam a gravar em `~/.claude/` em vez de `.claude/`. Mitigação: documentar no
  `CHANGELOG.md` e orientar o uso explícito de `--scope project`.
- Um passo adicional no fluxo interativo de `init`.
- Risco de regressão sutil se a detecção por *flag-set* for implementada como comparação de
  valor — codificado como critério de aceite testável em cada ML.

**Fora de escopo**
- `internal/generators/agents.go:16` (`InstallAgents`) — hardcoda `~/.claude/agents/`,
  sem callers de produção. Caminho morto, mantido como está.
- `internal/generators/scaffold.go:95` (`ForceInstallSkills`, via `update.go:122`) e
  `npm/src/generators/init.js:1043` (`installSkillsForce`) — instalam a skill do **próprio
  trackfw** em `~/.claude/skills/trackfw/`, global por natureza. Não alterados.
- Aliases deprecados (`copilot`, `cursor`, `gemini`, `windsurf`, `amazonq`) mantêm os
  scopes fixos que já passam hoje.

## Alternatives Considered

- **Manter `project` como default e apenas adicionar o prompt em TTY** — rejeitado:
  mantém a assimetria entre uso interativo e automatizado, e o comportamento não-interativo
  continua sendo exatamente o que o usuário reportou como bug.
- **Erro exigindo `--scope` em modo não-interativo** (precedente do
  `"requires --targets in non-interactive mode"`) — rejeitado pelo usuário: quebra toda
  automação existente de imediato, enquanto o default `global` degrada de forma mais suave.
- **Confirmação extra quando o escopo for `project`** — rejeitado: atrito desnecessário;
  a impressão dos destinos (D5) já entrega a transparência pretendida.
- **Tornar o escopo configurável por `.trackfw.yml`** — adiado: adiciona superfície de
  configuração sem resolver o problema reportado, que é a ausência de escolha no ponto de uso.

## Linked REQ

REQ: `docs/req/REQ-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`
