---
status: Proposed
date: 2026-07-25
author: "Zeus (Principal Software Architect)"
---

# ADR: Identidade personalizavel de agentes

> Date: 2026-07-25 | Status: Proposed

## Context

Os 10 agentes distribuidos pelo trackfw usam identidade neutra e funcional
(`trackfw-architect`, `trackfw-backend`, ...). A interacao e impessoal: nao ha
como o agente se apresentar por um nome proprio, nem como o usuario ser tratado
por um apelido.

A investigacao levantou os seguintes fatos, todos verificados no codigo e na
documentacao oficial do Claude Code:

1. **`Render()` (`internal/integrations/render.go`) e o funil unico** por onde
   todo agente passa antes de ser escrito, nas superficies suportadas
   (`markdown`, `custom-agent-toml`, `cli-agent-json`, `agent-json`,
   `agent-directory`). Personalizacao aplicada ali alcanca todos os alvos.

2. **A selecao de subagent usa apenas `name` + `description`.** Documentacao
   oficial (`sub-agents.md`): *"Claude uses each subagent's `description` to
   decide when to delegate tasks"* e *"identity comes only from the `name`
   frontmatter field"*. O corpo do agente e carregado somente **apos** a
   selecao. Logo:
   - apelido no `description` -> habilita roteamento por linguagem natural;
   - apelido no corpo -> habilita autoidentificacao ("quem e voce?");
   - `name` -> unico identificador aceito em mencao explicita (`@agent-<name>`).

3. **`name` nao precisa coincidir com o nome do arquivo.** Documentacao
   oficial: *"The filename doesn't have to match"*. O destino de instalacao e
   derivado de `{{id}}` no catalogo (`trackfw-{{id}}`), nunca de `name`.
   Portanto alterar `name` **nao** altera o path, **nao** invalida chaves do
   manifest e **nao** gera artefatos orfaos.

4. **Nao ha suporte nativo a alias.** O identificador tem de ser o proprio
   `name`, restrito a minusculas e hifens.

5. **Colisao de `name` e silenciosa e nao-deterministica.** Documentacao
   oficial: dois arquivos no mesmo diretorio com o mesmo `name` fazem o Claude
   Code carregar *"only one of them, chosen by filesystem read order rather
   than a documented precedence"*. Entre niveis, project vence user-level.
   Verificacao na maquina de referencia mostrou que `~/.claude/agents/` ja
   contem `zeus.md`, `apolo.md`, `afrodite.md`, `ares.md`, `artemis.md`,
   `athena.md`, `hades.md`, `hephaestus.md`, `metis.md` e `poseidon.md` —
   um preset mitologico grego "puro" colidiria com os 10.

6. **O modelo de hash do manifest suporta personalizacao sem drift.**
   `applyMutation` (`manager.go`) calcula `desiredHash` a partir de
   `plan.Content` em tempo de plano e grava `entry.Hash = actualHash`. Um
   artefato personalizado permanece `current` **desde que a configuracao de
   identidade seja duravel e lida antes de `BuildPlans()`**. `legacy.go`
   governa apenas a adocao de bytes default previamente publicados.

7. **`agentTools(name)` decide SET_ARCH vs SET_IMPL por
   `strings.HasSuffix(name, "architect")`.** Se `name` passar a ser derivado de
   entrada do usuario, o agente arquiteto perde silenciosamente 4 ferramentas
   de orquestracao.

## Decision

Adotar **identidade personalizavel materializada em tempo de instalacao**, com
tres campos distintos e um contrato de slug compartilhado pelos 3 CLIs.

### D1 — Modelo de identidade

| Campo | Destino no artefato | Funcao |
|---|---|---|
| `display_name` | `description` (prefixo) + corpo | Autoidentificacao e roteamento natural |
| `slug` | `name` do frontmatter | Mencao explicita `@agent-<slug>` |
| `user_nickname` (global, unico) | corpo apenas | Como o agente trata o usuario |

O `id` canonico do catalogo (`architect`, `backend`, ...) e o path de
instalacao (`trackfw-{{id}}`) **permanecem imutaveis**.

### D2 — Sufixo distintivo obrigatorio no slug

O `name` gerado recebe o sufixo fixo `-tf`:

```
display_name "Zeus"  ->  name: zeus-tf
```

Elimina por construcao a colisao com agentes pessoais homonimos, preservando
`@agent-zeus-tf` funcional e `display_name` limpo ("Zeus") no `description` e
no corpo. O sufixo e detalhe do identificador tecnico e nunca aparece na forma
como o agente se apresenta.

### D3 — Contrato de slug (identico nos 3 CLIs)

1. Slugs do preset sao **hardcoded na tabela do preset**, nunca derivados por
   normalizacao em runtime (`Artemis -> artemis`, `Metis -> metis`,
   `Hefesto -> hefesto`). Remove dependencia de diferencas de normalizacao
   Unicode entre Go, JavaScript e Python.
2. Texto livre passa por: NFD + remocao de diacriticos (ASCII-fold),
   lowercase, espacos e underscores -> hifen, remocao de caracteres fora de
   `[a-z0-9-]`, colapso de hifens repetidos, trim de hifens nas pontas.
3. Slug vazio apos normalizacao, ou com mais de 40 caracteres, e **rejeitado
   com erro** — nunca silenciosamente corrigido.
4. Unicidade validada entre os 10 agentes: dois `display_name` que produzam o
   mesmo slug sao rejeitados.
5. Uma **tabela de vetores de teste** unica, replicada nas tres suites
   (acentos, maiusculas, espacos, emoji, string vazia, colisao).

### D4 — Deteccao de colisao no install

Antes de escrever, o instalador varre o diretorio de destino em busca de
outros arquivos declarando o mesmo `name`. Colisao gera **aviso explicito** e
exige `--force`. O sufixo `-tf` torna o caso raro, mas a verificacao permanece
porque a falha silenciosa e indetectavel pelo usuario.

### D5 — Persistencia: config global unica

A identidade vive em **`~/.trackfw/identity.json`**, valida para os escopos
`global` e `project`. Optou-se deliberadamente por **nao** espelhar a regra de
raiz-por-escopo do manifest: identidade e preferencia do usuario, nao estado do
projeto, e duplica-la por projeto geraria nomes divergentes entre repositorios
do mesmo usuario.

```json
{
  "schema_version": 1,
  "user_nickname": "chefe",
  "agents": {
    "architect": { "display_name": "Zeus", "slug": "zeus-tf" }
  }
}
```

Ausencia do arquivo, `agents` vazio ou entrada ausente para um `id` ->
comportamento default atual, byte a byte.

### D6 — Materializacao em build time, nunca em runtime

A identidade e injetada por `Render()` e gravada no artefato. **O agente nunca
le a configuracao**: nenhuma tool call, nenhuma instrucao de leitura no corpo,
nenhum custo por interacao. O `description` sofre substituicao (nao acrescimo);
o corpo cresce em ordem de dezenas de tokens, carregados apenas apos a selecao.

### D7 — Cobertura de todos os pontos de re-render

Todo caller de `BuildPlans` deve resolver a identidade, sob pena de reverter a
personalizacao no proximo update:

| CLI | Callers |
|---|---|
| Go | `internal/commands/integrations_flags.go` (mutation + list), `internal/commands/init.go`, `internal/generators/update.go` |
| Node.js | `npm/src/integrations/index.js`, `npm/src/commands/update.js` |
| Python | equivalentes em `pypi/trackfw/integrations/` |

### D8 — Remocao da heuristica por nome em `agentTools`

`agentTools` passa a decidir por `item.ID == "architect"`. `Render()` ja recebe
`item`, entao a decisao deixa de depender de entrada do usuario.

### D9 — Caminho nao-interativo preservado

`init` ganha `--identity-preset greek|neutral|none`. O ramo `!IsTerminal`
existente nunca bloqueia em prompt, e re-executar `init` reutiliza a identidade
persistida em vez de re-perguntar.

## Consequences

### Positivas
- Interacao humanizada sem custo de token relevante.
- `@agent-zeus-tf` funcional, com agentes pessoais do usuario intactos.
- `agentTools` deixa de depender de string de nome — remove fragilidade
  preexistente.
- Path, manifest e ciclo `install/update/uninstall` inalterados.

### Negativas / custos
- Atravessa a regra dura de paridade: 3 implementacoes + vetores replicados.
- `description` personalizado pode ficar semanticamente proximo de um agente
  pessoal homonimo, degradando roteamento natural. Mitigado mantendo o sufixo
  de papel ("Zeus — Principal Software Architect").
- Quem ja instalou com defaults e depois habilita identidade ve estado
  `outdated` ate rodar `update` — transicao esperada, nao regressao.
- Novo arquivo de configuracao global a versionar e migrar.

### Neutras
- `legacy.go` nao requer manutencao: governa apenas bytes default publicados.

## Alternatives Considered

- **Alterar apenas `description` + corpo, mantendo `name`.** Menor risco, mas
  `@agent-<apelido>` nao funcionaria. Rejeitado por nao entregar o objetivo
  central de invocacao pelo nome humanizado.
- **Renomear tambem o arquivo instalado.** Rejeitado: quebraria chaves do
  manifest (indexado por path absoluto), exigindo migracao explicita e deixando
  artefatos orfaos em caso de falha.
- **Preset grego sem sufixo.** Rejeitado: colide com os 10 agentes pessoais ja
  presentes em `~/.claude/agents/`, produzindo shadowing silencioso.
- **Preset alternativo (nordico) sem sufixo.** Rejeitado: apenas adia a colisao
  para outros usuarios e ainda exigiria a varredura do D4.
- **Agente ler a configuracao em runtime.** Rejeitado: instrucao permanente no
  prompt + tool call por sessao + latencia, sem beneficio sobre materializar na
  instalacao.
- **Identidade por escopo (global e project separados).** Rejeitado: gera nomes
  divergentes entre projetos do mesmo usuario, contrariando a natureza de
  preferencia pessoal da feature.
