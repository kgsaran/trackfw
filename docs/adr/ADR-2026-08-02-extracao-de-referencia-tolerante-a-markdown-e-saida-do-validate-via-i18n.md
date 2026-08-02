---
status: Accepted
date: 2026-08-02
author: "Zeus"
---

# ADR: Extracao de referencia tolerante a markdown e saida do validate via i18n

> Date: 2026-08-02 | Status: Accepted

## Context

Dois defeitos de paridade da superfície do validador, reportados como achados colaterais no PR #103
e verificados por reprodução em 2026-08-02.

### Defeito A — backticks tornam a referência invisível

`extractRefPath` extrai o primeiro token após `ADR:` / `REQ:` / `Roadmap:` e remove aspas antes de
testar o sufixo `.md`. **Não remove backticks.** Uma linha escrita como

```
ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md` (P1–P4; esta REQ é ...)
```

produz o token `` `docs/adr/....md` ``, que não termina em `.md` — então o extrator **não encontra
referência alguma** e segue em silêncio.

Reproduzido: **13 REQs** do repositório usam backticks no campo `ADR:` do corpo. Dez delas têm o
frontmatter `adr:` populado, que é lido primeiro e salva o caso. **Três não têm**, e ficam
inalcançáveis por qualquer regra que dependa desse extrator — inclusive a
`adr_accepted_when_req_done` entregue no PR #103:

- `REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md`
- `REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`
- `REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md`

Hoje as três apontam para um ADR `Accepted`, então **não há falso-negativo real** — mas o buraco é
estrutural, e é silencioso por natureza: nada avisa que a referência não foi lida.

Verificado que **não** há um segundo defeito: `adr: ""` no frontmatter **não** causa early-return.
O valor `""` é reduzido a string vazia, falha o teste de `.md` e o laço **continua** corretamente
até a linha do corpo. A causa é única — o backtick.

### Defeito B — Python ignora a própria chave de i18n

Os três CLIs têm a chave `validate.ok` com o valor `"✓ No violations found."` nos respectivos
`i18n/locales/en-US.json`. Go e Node a usam. **O Python não**:
`pypi/trackfw/commands/validate.py:104` imprime `"✓ Governance OK"` **hardcoded**.

Não é divergência de tradução — é código ignorando o recurso que o próprio pacote já carrega.

### Divergência estrutural observada nos três extratores

Os três tokenizam igual (primeiro token separado por espaço), mas removem delimitadores de formas
diferentes:

| CLI | Mecanismo | Comportamento em delimitador não pareado (`"x.md'`) |
|---|---|---|
| Go | `strings.Trim(v, "\"'")` | remove ambos |
| Node | `replace(/^["']\|["']$/g, '')` | remove ambos |
| Python | `normalize_yaml_flat_value` — só se **par casado** | **não remove** |

É divergência **pré-existente**, sem caso real no repositório.

## Decision

1. **`extractRefPath` e equivalentes passam a remover backtick** além de aspas simples e duplas,
   nos 3 CLIs.
2. **Cada CLI mantém o próprio mecanismo de remoção** — apenas o backtick é acrescentado ao
   conjunto. **Não** unificar os mecanismos neste ciclo: fazê-lo mudaria o comportamento em
   delimitador não pareado, que ninguém pediu e nenhum caso real exercita.
3. **Mas a divergência passa a ser medida, não presumida:** um critério de aceite exige que os três
   produzam saída **idêntica** para uma tabela compartilhada de entradas — incluindo delimitador
   não pareado. Se divergirem, o executor **reporta** em vez de escolher sozinho.
4. **O Python passa a usar a chave `validate.ok`** do próprio i18n, como Go e Node. O literal
   `"✓ Governance OK"` é removido.
5. **Nenhuma outra mudança de comportamento.** Em particular, o caminho de early-return para valor
   vazio **não** é alterado — está correto.

## Consequences

**Positivas**

- Fecha um falso-negativo silencioso: referências escritas com a formatação markdown natural
  (código inline) passam a ser lidas. Três REQs do repositório saem da invisibilidade.
- A mensagem de sucesso do `validate` fica idêntica nos 3 CLIs, e passa a respeitar i18n no Python
  — hoje ela ignoraria qualquer tradução configurada.
- A divergência dos mecanismos de strip deixa de ser folclore e passa a ter medição.

**Negativas / aceitas**

- **Mudança de saída observável no Python:** quem depende do texto `✓ Governance OK` — script,
  pipeline — quebra. É correção de paridade, mas precisa constar do CHANGELOG.
- A divergência de delimitador não pareado **permanece**. Aceito: sem caso real, e unificar teria
  custo/risco desproporcional. Fica medida e documentada.
- Referências ficam mais permissivas: uma linha com backtick que antes era ignorada agora é lida e
  pode produzir violação que antes não existia. É o objetivo, mas pode surpreender projetos
  downstream — daí a exigência de que `validate` siga verde neste repositório.

## Alternatives Considered

**Unificar os três mecanismos de strip agora** — eliminaria a divergência de vez.
**Rejeitado:** mudaria comportamento em delimitador não pareado sem nenhum caso real que o exija,
misturando correção de bug com refatoração de contrato. Fica medido por critério de aceite; se a
medição acusar divergência relevante, vira REQ própria.

**Corrigir o frontmatter das 3 REQs em vez do extrator** — resolveria o sintoma hoje.
**Rejeitado:** trata os dados e não o mecanismo. Qualquer REQ nova escrita com backtick — a forma
natural em markdown para um caminho de arquivo — voltaria a ser invisível.

**Manter `"✓ Governance OK"` e alinhar Go e Node a ele** — também produziria paridade.
**Rejeitado:** os três já carregam `validate.ok` = `"✓ No violations found."` no i18n. O Python é
o desviante, e a correção é fazê-lo usar o recurso que já tem — não propagar o hardcode.

**Tolerar também `*`, `_` e colchetes de link markdown** — cobertura maior.
**Rejeitado por ora:** backtick é a forma efetivamente usada nos 13 casos reais. Ampliar sem caso
concreto é especulação; o extrator ficaria mais permissivo sem necessidade demonstrada.
