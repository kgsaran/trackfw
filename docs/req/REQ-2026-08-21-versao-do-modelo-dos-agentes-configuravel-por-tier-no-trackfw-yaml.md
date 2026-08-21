---
status: Open
date: 2026-08-21
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md"
---

# REQ: versão do modelo dos agentes configurável por tier no `trackfw.yaml`

> Date: 2026-08-21 | Status: Open

## Motivação

O catálogo fixa **tiers canônicos** (`opus`, `sonnet`) nos assets de agente, e cada alvo mapeia o
tier para o seu universo (`sonnet` → `flash` no Antigravity, `gpt-5.4-mini` no Codex,
`composer-2.5[fast=true]` no Cursor, `sonnet` passando direto no Claude).

**O usuário não tem como escolher a versão.** Medido em 2026-08-21 na máquina de KG: os 11
implementadores rodavam no alias `sonnet`, que resolve para o Sonnet corrente. Pinar exigiu editar
`~/.claude/agents/*.md` à mão — e esses arquivos **são gerados** a partir de
`internal/integrations/assets/agents/*.md`, então o próximo `trackfw agents update` reverte tudo,
**sem aviso**.

O conflito já é concreto no repositório: a regra de verbosidade entregue no PR #198 só chega ao
arquivo local via `agents update`, e rodar o update hoje desfaz o pin de modelo. **Hoje só dá para
ter um dos dois.**

### O motivo é cota, não custo — e a distinção precisa ficar escrita

Validado na documentação oficial (`platform.claude.com/docs/en/about-claude/models/overview`):

| modelo | 1M tokens equivalem a | preço in/out |
|---|---|---|
| Sonnet 4.6 | ~750k palavras | $3 / $15 |
| Sonnet 5 | ~555k palavras | $2 / $10 |

A doc explica a causa: *"uses the tokenizer introduced with Claude Opus 4.7; compared to models
before Claude Opus 4.7, the same text produces roughly **30% more tokens**"*.

Quem otimiza **cota de tokens** quer o modelo pré-4.7. Quem otimiza **custo em dólar** quer o
oposto — o Sonnet 5 é um terço mais barato por token, o que mais que compensa os 30%. **São
decisões contrárias a partir do mesmo dado**, e sem o motivo registrado um leitor futuro conclui que
a escolha está errada.

## Escopo

### 1. Configuração por tier, guardando **só a versão**

```yaml
agent_models:
  opus:   "5"      # -> claude-opus-5
  sonnet: "4.6"    # -> claude-sonnet-4-6
```

**Guardar a versão e compor o ID, em vez de guardar o ID pronto** — decisão de KG, e o argumento
mais forte é a composição por alvo. O mesmo modelo tem forma diferente por destino:

```
Claude      claude-sonnet-4-6
OpenCode    anthropic/claude-sonnet-4-6      (docs/cli-parity.md:419)
Bedrock     anthropic.claude-sonnet-4-6
```

Um ID cru obrigaria o render a **desmontar a string para remontar** noutro formato — frágil e
adivinhatório. Guardando a versão, cada alvo compõe a sua própria forma, que é exatamente o que a
camada de render já faz com os tiers.

### 2. Três regras de composição, todas da documentação oficial

1. **Ponto vira traço:** `4.6` → `4-6`.
2. **Versão maior omite o minor:** `5` → `claude-sonnet-5`, nunca `claude-sonnet-5-0`.
3. **Antes da 4.6 há data no ID** (`claude-sonnet-4-5-20250929`); o ID sem data é alias.

### 3. Escape hatch: ID completo aceito literalmente

Por causa da regra 3, e porque o formato **já mudou uma vez**. Se o valor não parecer versão, usar
como está.

### 4. Comando que lista a resolução efetiva

Por agente e por alvo. Sem isso o usuário configura e **não tem como confirmar que pegou** — que foi
exatamente a situação em que KG perguntou qual modelo os agentes usavam e nem ele nem eu sabíamos
responder sem medir.

### 5. O catálogo passa a pinar as versões escolhidas

Para que `agents update` **reforce** o pin em vez de desfazê-lo.

## O que **não** é escopo

- Trocar o tier de um agente (quem é `opus`, quem é `sonnet`). É outra decisão, com outro critério.
- Mudar os mapeamentos de Codex, Cursor e Antigravity. Eles têm universo próprio e ficam como estão.
- Escolher modelo por agente individual. O tier é a unidade; granularidade maior é escopo futuro se
  alguém pedir.

## 🔴 O risco dominante: vazamento entre namespaces

A sobrescrita **só pode valer onde o namespace é Claude**. Se `claude-sonnet-4-6` vazar para o
mapeamento do Codex ou do Cursor, os três alvos quebram — e quebram no artefato gerado, não no
`trackfw`, então o usuário descobre quando o agente não sobe.

Isto precisa ser gate, não cuidado.

## Acceptance Criteria

- [x] AC1 — `agent_models` por tier no `trackfw.yaml`, guardando **versão**, não ID.
- [x] AC2 — Composição correta nas três regras: ponto→traço, maior sem minor, e alvo compondo a
      própria forma (Claude / OpenCode / Bedrock).
- [x] AC3 — Escape hatch: valor que não é versão é usado **literalmente**.
- [x] AC4 — **Nenhum vazamento de namespace** — provado por cenário: Codex, Cursor e Antigravity
      seguem com os próprios valores quando `agent_models` está configurado.
- [x] AC5 — Comando lista a **resolução efetiva** por agente e por alvo.
- [x] AC6 — Catálogo pina as versões escolhidas; `agents update` **reforça** o pin.
- [x] AC7 — Paridade nos 3 CLIs, com **gate comparando saídas reais**.
- [x] AC8 — Cenário P4 com baseline e detecção.
- [x] AC9 — Anotação `trackfw-contract` da seção (o checker de cobertura é bloqueante).
- [x] AC10 — O motivo (**cota, não custo**) registrado no `cli-parity.md` ou em ADR.
- [ ] AC11 — `make quality` verde **e CI verde**. (local verde; **aguardando CI**)

## Riscos para quem executar

- **Config ausente não pode quebrar nada.** Sem `agent_models`, o comportamento é o de hoje — tier
  canônico. Regressão aqui atinge todo usuário do trackfw.
- **Não presumir que todo usuário tem acesso ao modelo pinado.** Plano e região variam; a resolução
  não deve falhar de forma obscura.
- **`make install` está proibido** neste ambiente (o CLI vem do Homebrew); use `make build` e
  `./bin/trackfw`. O binário do `PATH` está desatualizado e `--version` **não** distingue o build.
- Invocação CI-exata: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`.

## Linked ADR
ADR: <!-- a criar: versao por tier, composicao por alvo, escape hatch -->

## Linked Roadmap
Roadmap: <!-- a criar -->
