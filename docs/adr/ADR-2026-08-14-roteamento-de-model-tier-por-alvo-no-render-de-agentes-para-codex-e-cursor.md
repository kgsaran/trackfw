---
status: Accepted
date: 2026-08-14
author: "Zeus"
---

# ADR: roteamento de model tier por alvo no render de agentes para codex e cursor

> Date: 2026-08-14 | Status: Accepted

## Context
<!-- What is the situation that motivates this decision? -->

O catálogo canônico de agentes trackfw (`internal/integrations/assets/agents/*.md`) já
declara um tier abstrato de custo por agente no frontmatter — `model: opus` para
`architect`, `model: sonnet` para os demais 9 especialistas — usando o vocabulário de
alias da Claude Code. `Render()` (`internal/integrations/render.go`) consome esse campo
e hoje só o mapeia para um alvo: `agent-directory` (Antigravity CLI, surface `current`),
via `mapModel()` (`opus→pro`, `sonnet→flash`), decisão registrada em
ADR-2026-07-19-antigravity-agent-tools.md após confirmação empírica de que o `agy`
rejeitava silenciosamente `model: opus|sonnet` verbatim.

Levantamento da documentação oficial de Codex CLI e Cursor (2026-08-14) mostra o mesmo
padrão de risco nos dois alvos restantes que já suportam seleção de modelo por agente:

- **Codex CLI** (`representation: custom-agent-toml`, único alvo dessa representação):
  o branch `case "custom-agent-toml"` de `Render()` emite `name`, `description` e
  `developer_instructions` no TOML — **nunca `model`**. O agente customizado do Codex
  roda sempre no modelo default da sessão do orquestrador, independentemente do tier
  declarado no catálogo. Não é um bug de rejeição (como o do Antigravity), é uma
  omissão: a capacidade de tiering existe no runtime (`.codex/agents/<nome>.toml`
  aceita `model = "..."` e `model_reasoning_effort = "..."`) mas o render não a usa.

- **Cursor** (`representation: agent-markdown`, **compartilhada** com `gemini` e
  `kiro` surface `ide`): não há `case` dedicado em `Render()` para `agent-markdown`,
  então cai no branch `default`, que devolve o frontmatter de origem verbatim (ou,
  com identidade customizada, reescreve apenas `name`/`description` via
  `rewriteFrontmatterFields`, preservando `model:` intocado). O resultado é
  `model: opus` / `model: sonnet` literal no arquivo `.cursor/agents/trackfw-*.md`.
  A documentação oficial da Cursor (`cursor.com/docs/subagents`) não lista `opus`/
  `sonnet` como aliases aceitos — os valores documentados são `inherit`, IDs
  completos (`claude-opus-5`, `composer-2.5`) opcionalmente com parâmetros entre
  colchetes (`[effort=high]`, `[fast=true]`). O mesmo padrão de falha silenciosa do
  Antigravity é plausível aqui, mas **não foi confirmado empiricamente** nesta
  sessão (sem instância local do Cursor para testar `— este ADR registra risco
  documentado, não bug comprovado, e o roadmap correspondente inclui verificação
  antes de fechar o critério de aceite).

Uma complicação estrutural: como `agent-markdown` é uma representação **compartilhada**
por `cursor`, `gemini` e `kiro` (surface `ide`), e `Render()` hoje despacha somente por
`capability.Representation` — sem receber o `target.ID` do chamador —, não é possível
tratar `cursor` de forma diferente de `gemini`/`kiro` sem alterar a assinatura de
`Render()`. Aplicar um mapeamento de modelo ao branch `default` inteiro afetaria os
três alvos por igual, incluindo dois (`gemini`, `kiro`) cuja sintaxe de modelo aceita
não foi pesquisada nem confirmada nesta sessão — o que violaria a diretriz do projeto
de nunca simular suporte não confirmado (mesmo princípio que já rege a omissão
deliberada de `model:` no branch `opencode-agent`).

## Decision
<!-- What was decided? -->

1. **Codex**: estender o branch `case "custom-agent-toml"` de `Render()` para emitir
   `model = "<valor mapeado>"` no TOML, resolvido a partir do `model` canônico
   (`opus`/`sonnet`) por uma nova função `mapModelCodex()`, espelhando o padrão já
   existente de `mapModel()` para Antigravity. Diferença relevante a documentar: o
   Antigravity usa tiers estáveis (`pro`/`flash`) que não mudam com o ciclo de release
   de modelo; o Codex exige um **ID de modelo versionado** (ex.: `gpt-5.4`), então o
   mapeamento fica sujeito a ficar desatualizado quando a OpenAI depreciar/renomear
   modelos — aceito como débito conhecido, não bloqueador, e centralizado em um único
   ponto do código para minimizar o custo de atualização futura.
2. **Cursor**: estender a assinatura de `Render()` para receber o `target.ID` (ou
   equivalente) do chamador, permitindo diferenciar `cursor` de `gemini`/`kiro` dentro
   da representação `agent-markdown` compartilhada. Introduzir um `case` (ou sub-branch
   condicionado a `target.ID == "cursor"`) que aplica `mapModelCursor()` — mapeando
   `opus→claude-opus-5[effort=high]`, `sonnet→composer-2.5[fast=true]` (valores a
   confirmar/ajustar na Wave de implementação) — e reconstrói apenas a linha `model:`
   do frontmatter, preservando todo o restante byte-a-byte (mesmo contrato de
   `rewriteFrontmatterFields`). `gemini` e `kiro` continuam recebendo o passthrough
   atual, inalterado, até que sua sintaxe de modelo seja pesquisada e confirmada em
   REQ futura — não fazem parte do escopo desta decisão.
3. Mudança de assinatura de `Render()` é uma mudança de contrato entre os 3 CLIs
   (Go fonte canônica, Node.js, Python) — precisa de paridade e dos mesmos testes de
   contrato que já cobrem `render_test.go`.
4. Onde não houver como confirmar empiricamente a sintaxe aceita pelo runtime alvo
   (Cursor não está instalado localmente neste ambiente), o critério de aceite da REQ
   exige verificação documental cruzada (documentação oficial já citada) e, se
   possível, teste manual do usuário antes de mover a REQ para `Done` — não apenas
   testes de contrato Go/Node/Python, que só garantem paridade de output, não
   corretude contra o runtime real.

## Consequences
<!-- What are the positive and negative consequences of this decision? -->

- Codex e Cursor passam a herdar o tiering de custo já declarado no catálogo
  canônico, hoje efetivo apenas para Claude Code (nativo, sem tradução) e Antigravity.
- `Render()` ganha um parâmetro (`target.ID`) que hoje não existe — todos os 9
  call-sites (3 CLIs × representações) precisam ser auditados para paridade.
- O mapeamento Codex introduz acoplamento a IDs de modelo versionados da OpenAI,
  exigindo manutenção periódica — mitigado por centralizar o mapeamento em uma única
  função, mas não eliminado.
- `gemini` e `kiro` permanecem com o mesmo passthrough não verificado que têm hoje —
  esta decisão não piora nem resolve esse risco pré-existente, apenas não o expande.

## Alternatives Considered
<!-- What other options were evaluated and why were they rejected? -->

- **Aplicar o mesmo mapeamento de modelo ao branch `default` inteiro (afetando
  cursor, gemini e kiro de uma vez):** rejeitado — simularia suporte não confirmado
  para gemini/kiro, violando a diretriz do projeto de nunca inventar comportamento
  não documentado/verificado (mesmo racional que already levou à omissão deliberada
  de `model:` no OpenCode).
- **Não tratar Codex separadamente e aguardar disponibilidade de teste local:**
  rejeitado — a omissão de `model` no TOML do Codex é um gap confirmado por leitura
  direta do código (não uma hipótese), sem custo de espera adicional.
- **Usar tiers abstratos também no Codex (ex.: inventar um valor `"heavy"/"light"` no
  `config.toml` do usuário) em vez de IDs versionados:** rejeitado — o Codex CLI não
  documenta suporte a apelidos abstratos de modelo em `[agents]`/perfis; exigiria o
  usuário manter esse mapeamento manualmente fora do trackfw, o que é pior do que
  aceitar o débito de manutenção de um ID versionado centralizado no código.
