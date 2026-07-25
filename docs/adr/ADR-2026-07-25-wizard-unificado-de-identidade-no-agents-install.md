---
status: Proposed
date: 2026-07-25
author: "Zeus (Principal Software Architect)"
---

# ADR: Wizard unificado de identidade no agents install

> Date: 2026-07-25 | Status: Proposed

## Context

O ADR-2026-07-25-identidade-personalizavel-de-agentes (PR #64) entregou a
identidade personalizavel de agentes. O wizard interativo, porem, ficou
**exclusivamente em `trackfw init`**, o comando de bootstrap de projeto.

Isso produziu tres lacunas, identificadas em uso:

### L1 — Descoberta

`trackfw agents install` e o caminho natural para instalar agentes em um
projeto ja inicializado. Hoje ele **le** `~/.trackfw/identity.json` (via
`identity.Load` em `integrations_flags.go`) mas **nunca oferece** configurar a
identidade. Quem nao roda `init` novamente so descobre a feature lendo o
README. Uma feature de personalizacao invisivel no comando que a consome e,
na pratica, uma feature desligada.

### L2 — Modo `custom` expoe o id tecnico, nao a especialidade

O prompt atual do modo livre e:

```
Nome de exibicao para o agente (architect)
```

O usuario ve `architect`, `dba`, `code-quality` — identificadores internos. O
catalogo (`assets/catalog.json`) ja carrega, para cada agente, um `name`
legivel e uma `description` da especialidade:

```
architect  | Architect | Architecture, ADRs and governed coordination
dba        | DBA       | Database modeling, performance and recovery
```

Esses campos existem e nao sao usados no wizard.

### L3 — Presets sao escolhidos as cegas

Escolher `tolkien` nao revela que o agente de seguranca virara "Boromir" e o
de banco de dados, "Elrond". O usuario so descobre o mapeamento **depois** de
os artefatos terem sido escritos em disco. Nao ha etapa de confirmacao.

### Restricao herdada

O wizard **nunca** pode bloquear em caminho nao-interativo (`!IsTerminal`) —
regra ja estabelecida no ADR anterior (D9) e coberta por testes. Qualquer
prompt novo herda essa restricao.

## Decision

### D1 — Componente unico de wizard, consumido por dois comandos

Extrair o wizard de identidade para um componente reutilizavel, consumido por
`init` **e** por `agents install`. Duas implementacoes divergiriam com o
tempo e multiplicariam por 6 (2 comandos x 3 CLIs) o custo de cada ajuste de
UX.

### D2 — Regra de acionamento (evita virar incomodo)

O passo de identidade em `agents install` e exibido **somente** quando **todas**
as condicoes valem:

1. `kind == agents` (ver D5);
2. stdin e TTY;
3. **nao existe** `~/.trackfw/identity.json`, **ou** o usuario passou `--identity`
   para reconfigurar explicitamente.

Com identidade ja configurada e sem `--identity`, o comando **nao pergunta
nada** e apenas informa qual identidade esta em uso. Um wizard que reaparece a
cada instalacao seria pior do que a ausencia dele.

Para uso nao-interativo, `agents install` passa a aceitar `--identity-preset`,
com a mesma semantica ja definida em `init`.

### D3 — Tela de confirmacao com especialidade → nome

Antes de persistir, exibir os 10 pares e pedir confirmacao:

```
── Confirmação ──────────────────────────────
  Architecture, ADRs and governed coordination   →  Gandalf
  Backend APIs, domain logic and integrations    →  Aragorn
  Threat analysis and DevSecOps controls         →  Boromir
  Database modeling, performance and recovery    →  Elrond
  ...
  Como você será chamado:                           chefe

? Confirmar? (S/n)
```

Recusar retorna a selecao de preset, sem gravar nada. Vale para preset **e**
para o modo `custom`.

### D4 — Rotulos derivados do catalogo, nao do id

No modo `custom`, cada campo usa `Item.Name` e `Item.Description` do catalogo:

```
Architect — Architecture, ADRs and governed coordination
> _
```

O `id` deixa de aparecer no rotulo. Fonte unica: `catalog.json`, ja embedado
nos 3 CLIs — nenhuma tabela nova de textos a manter sincronizada.

### D5 — `skills install` nunca pergunta identidade

Skills nao tem identidade (`Render` ja faz short-circuit em `KindSkills`).
`newIntegrationsLifecycleCmd` e compartilhado entre `agents` e `skills`, entao
o gate por `kind` e obrigatorio — sem ele, `trackfw skills install` exibiria um
wizard sem efeito algum.

### D6 — Ordem do fluxo

```
1. Target CLIs            (existente)
2. Agentes a gerenciar    (existente)
3. Superficie ambigua     (existente, condicional)
4. Apelido do usuario      NOVO ── condicional por D2
5. Preset ou custom        NOVO ── condicional por D2
6. Nomes livres            NOVO ── so no modo custom
7. Confirmacao             NOVO ── condicional por D2
8. Instalacao             (existente)
```

A identidade vem **depois** da selecao de alvos porque alvo e o dado que o
usuario veio fornecer; identidade e configuracao acessoria. Interromper antes
do objetivo principal aumenta abandono.

### D7 — Paridade nos 3 CLIs

Regra dura do projeto. A ordem das etapas, os rotulos, a regra de acionamento
e o conteudo da tela de confirmacao devem ser equivalentes em Go, Node e
Python.

## Consequences

### Positivas
- Feature deixa de depender de leitura de README para ser descoberta.
- Usuario ve o mapeamento antes de escrever em disco (D3), eliminando o
  "instalei e nao era o que eu esperava".
- Rotulos passam a comunicar especialidade em vez de identificador interno.
- Uma unica implementacao de wizard por CLI reduz divergencia futura.

### Negativas / custos
- `agents install` ganha um caminho interativo novo — mais superficie de teste,
  em especial para garantir que o ramo nao-TTY siga intocado.
- A regra de acionamento (D2) e sutil: errar para o lado permissivo transforma
  o wizard em incomodo recorrente. Precisa de teste explicito para o caso
  "identidade ja existe → nao pergunta".
- 3 implementacoes + i18n em 9 arquivos de locale.

### Neutras
- Nenhuma mudanca no formato de `identity.json`, no schema, no contrato de
  slug ou nos artefatos gerados. Esta REQ e **exclusivamente de UX de CLI**;
  o gate `check-identity-parity.sh` existente continua valendo sem alteracao.

## Alternatives Considered

- **Duplicar o wizard em `agents install`.** Rejeitado: 6 copias (2 comandos x
  3 CLIs) divergiriam ao primeiro ajuste de UX.
- **Perguntar identidade em toda execucao de `agents install`.** Rejeitado:
  vira incomodo e leva o usuario a automatizar o "pular", esvaziando a feature.
- **Manter o wizard so em `init` e resolver a descoberta com documentacao.**
  Rejeitado: e a situacao atual, e foi ela que produziu a lacuna L1.
- **Tela de confirmacao apenas para presets.** Rejeitado: o modo `custom` tem
  10 campos digitados e e onde erro de digitacao e mais provavel — e o caso que
  mais se beneficia da revisao.
- **Criar tabela propria de rotulos de especialidade.** Rejeitado: `catalog.json`
  ja tem `name` e `description` embedados nos 3 CLIs; uma tabela paralela seria
  mais uma fonte a manter sincronizada.
