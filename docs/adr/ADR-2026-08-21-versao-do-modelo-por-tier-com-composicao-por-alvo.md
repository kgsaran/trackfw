---
status: Accepted
date: 2026-08-21
author: "Zeus (Arquiteto)"
---

# ADR: versão do modelo por tier, com composição por alvo

> Date: 2026-08-21 | Status: Accepted

## Context

Os assets de agente fixam **tiers canônicos** (`opus`, `sonnet`), e `internal/integrations/render.go`
mapeia cada tier para o universo do alvo — `mapModel` (Antigravity), `mapModelCodex`,
`mapModelCursor`, e passagem direta no Claude. O usuário **não escolhe a versão**.

Medido em 2026-08-21: pinar exigiu editar `~/.claude/agents/*.md` à mão, e esses arquivos são
**gerados** a partir de `internal/integrations/assets/agents/*.md`. O próximo `trackfw agents update`
reverte, **sem aviso** — o pior modo de falha para uma configuração de custo, porque o sintoma é
consumo subindo, não erro.

O conflito já é concreto: a regra de verbosidade do PR #198 só chega ao arquivo local via
`agents update`, e o update desfaz o pin. **Hoje só dá para ter um dos dois.**

## Decision

### 1. Configurar a **versão**, não o ID

```yaml
agent_models:
  opus:   "5"      # -> claude-opus-5
  sonnet: "4.6"    # -> claude-sonnet-4-6
```

Guardar o ID pronto **quebraria os alvos não-Anthropic**. O mesmo modelo tem forma diferente por
destino:

```
Claude      claude-sonnet-4-6
OpenCode    anthropic/claude-sonnet-4-6      (docs/cli-parity.md:419)
Bedrock     anthropic.claude-sonnet-4-6
```

Com o ID cru, o render teria que **desmontar a string para remontar** noutro formato — frágil e
adivinhatório. Com a versão, cada alvo compõe a própria forma, que é **exatamente o que a camada de
render já faz** com os tiers. A sugestão é de KG; o argumento de composição por alvo é o que a
sustenta.

### 2. Três regras de composição, da documentação oficial

1. **Ponto vira traço:** `4.6` → `4-6`.
2. **Versão maior omite o minor:** `5` → `claude-sonnet-5`, nunca `claude-sonnet-5-0`.
3. **Antes da 4.6 há data no ID** (`claude-sonnet-4-5-20250929`); o ID sem data é alias.

### 3. Escape hatch: valor que não é versão é usado literalmente

Por causa da regra 3, e porque **o formato já mudou uma vez**. Amarrar o produto ao formato atual
repetiria o erro que a própria doc documenta.

### 4. 🔴 A sobrescrita só vale onde o namespace é Claude

Codex, Cursor e Antigravity têm mapeamento próprio e **não são afetados**. Um ID Claude vazando para
lá quebra os três — e quebra no **artefato gerado**, não no `trackfw`, então o usuário descobre
quando o agente não sobe.

**Isto é gate, não cuidado.**

### 5. Config ausente não muda nada

Sem `agent_models`, comportamento idêntico ao de hoje: tier canônico. O default precisa ser correto
sem intervenção.

## O motivo é cota, não custo — e a distinção fica registrada

Validado em `platform.claude.com/docs/en/about-claude/models/overview`:

| modelo | 1M tokens equivalem a | preço in/out |
|---|---|---|
| Sonnet 4.6 | ~750k palavras | $3 / $15 |
| Sonnet 5 | ~555k palavras | $2 / $10 |

Causa, na própria doc: *"uses the tokenizer introduced with Claude Opus 4.7; compared to models
before Claude Opus 4.7, the same text produces roughly **30% more tokens**"*.

Quem otimiza **cota** quer o modelo pré-4.7. Quem otimiza **dólar** quer o oposto — o Sonnet 5 é um
terço mais barato por token, o que mais que compensa os 30%. **São decisões contrárias a partir do
mesmo dado.** Sem isto escrito, um leitor futuro "corrige" a escolha para o lado errado.

## Consequences

**Positivas**
- O pin sobrevive ao `agents update` — hoje não sobrevive.
- Quem tem restrição de cota decide sem editar arquivo gerado.
- A resolução efetiva fica **inspecionável**: sem comando que a liste, configurar é apostar.

**Negativas / riscos**
- Superfície de config nova nos 3 CLIs, com custo de paridade.
- **Pinar cria manutenção:** modelo tem cronograma de depreciação próprio, e um pin envelhece em
  silêncio. Aceito — o alias envelhece igual, só que sem o usuário saber qual está usando.
- Nem todo usuário tem acesso ao modelo pinado; plano e região variam. A resolução não pode falhar
  de forma obscura.

## Alternatives Considered

- **Guardar o ID completo** — rejeitada: quebra OpenCode e Bedrock, ou obriga o render a desmontar
  string. Foi a minha proposta inicial; a de KG é melhor.
- **Editar o asset e pronto** — rejeitada: some no `agents update`, que é o defeito que originou a REQ.
- **Botão de "usar versão anterior"** — rejeitada pelo mesmo motivo do estado `none` na
  `REQ-2026-08-18`: opção binária é ajustada uma vez e esquecida, e não sobrevive à próxima geração
  de modelos.
- **Modelo por agente individual** — adiada, não rejeitada. O tier é a unidade certa hoje;
  granularidade maior se alguém pedir.
