---
status: Accepted
date: 2026-08-02
author: "Zeus"
---

# ADR: Python alinha delimitador nao pareado e ordenacao do fallback de agentes

> Date: 2026-08-02 | Status: Accepted

## Context

Dois itens ficaram na fila, ambos **medidos** em ciclos anteriores e nenhum com caso real no
repositório. KG pediu que fossem fechados **antes** de gerar a tag, para não versionar defeito
conhecido.

Ao investigar, o segundo item mudou de natureza.

### Item 1 — delimitador não pareado

`ADR: "docs/adr/X.md'` — abre com aspa dupla, fecha com simples. Medido no PR #104:

| CLI | Mecanismo | Resultado |
|---|---|---|
| Go | `strings.Trim(v, "\"'`")` — remove qualquer char do conjunto | `docs/adr/X.md` |
| Node | `replace(/^["'`]\|["'`]$/g, '')` — uma ocorrência por ponta | `docs/adr/X.md` |
| Python | `normalize_yaml_flat_value` — só remove **par casado** | `''` |

### Item 2 — NÃO era o que eu reportei

Eu havia registrado como "o parser YAML do Python não trata lista inline". Ao medir:

**Nenhum dos três trata lista inline.** Go (`internal/config/config.go:190`), Node
(`npm/src/config/index.js:135`) e Python só reconhecem lista em bloco (`- item`). Com
`agents: [zeus, apolo]`, os três devolvem lista vazia e caem no fallback de varrer subdiretórios.

A divergência real está **no fallback**:

```
fixture: docs/roadmaps/{zeus,apolo}/wip/  (criados nessa ordem)

Go     → [apolo] … [zeus]     (ordenado)
Node   → [apolo] … [zeus]     (ordenado)
Python → [zeus]  … [apolo]    (ordem do filesystem)
```

Causa: `_list_dirs` (`pypi/trackfw/commands/status.py`) **não ordena**, enquanto a função irmã
`_list_files`, no mesmo arquivo, ordena. Go e Node ordenam (`sort.Strings`, `.sort()`).

### Achado novo — item 2b, fora do escopo

Que os três **silenciosamente ignorem** `agents: [a, b]` é defeito próprio: o usuário escreve
configuração válida em YAML e nada acontece, sem aviso. Mas é **consistente entre os CLIs**,
portanto não é problema de paridade — e resolvê-lo exige decisão de produto (suportar inline, ou
avisar que foi ignorado). Fica para a fila.

### Restrição que não se aplica

Compatibilidade retroativa. KG confirmou que o trackfw ainda não tem usuários externos, então
"quebraria quem depende do comportamento atual" não é argumento — em nenhum dos dois itens.

## Decision

**O Python alinha-se a Go e Node nos dois casos.** Em ambos, os outros dois concordam entre si e
o Python é o desviante.

1. **Delimitador não pareado:** o Python passa a removê-lo, como Go e Node. Concretamente, a
   remoção deixa de exigir par casado **no caminho da extração de referência**.
2. **Fallback de agentes:** `_list_dirs` passa a ordenar, como `_list_files` já faz no mesmo
   arquivo e como Go e Node fazem.
3. **Inline não é suportado em lugar nenhum** — permanece assim. Fica registrado como item de
   fila com decisão de produto pendente.

**Escopo cirúrgico:** as duas correções são **só no Python**. Go e Node não são tocados.

### Sobre o item 1: por que alinhar ao permissivo

Considerei o inverso — tornar Go e Node estritos como o Python. Um valor `"X.md'` é malformado, e
recusá-lo é defensável. Mas **nenhum dos dois comportamentos emite erro**: o estrito simplesmente
não encontra a referência, tão silenciosamente quanto o permissivo a encontra. Entre dois
silêncios, o que **acha** o arquivo é menos danoso que o que o ignora — e é a mesma lógica que
justificou aceitar backtick no PR #104.

## Consequences

**Positivas**

- Fecha as duas últimas divergências medidas, permitindo tagear sem defeito conhecido — que era
  o pedido de KG.
- O fallback de agentes passa a ser determinístico no Python. Ordenação instável dependia da
  ordem de criação no filesystem, o que torna teste e comparação entre CLIs frágeis.
- `_list_dirs` fica coerente com `_list_files`, sua irmã no mesmo arquivo.

**Negativas / aceitas**

- O Python fica **mais permissivo** com valor malformado. Aceito: entre dois silêncios, achar é
  melhor que ignorar.
- A ordenação muda a saída do Python em `by_agent` quando `agents:` não está configurado. Sem
  impacto real — não há usuários, e a ordem anterior era não determinística de qualquer forma.
- O item 2b (inline ignorado em silêncio) **permanece aberto** nos três CLIs.

## Alternatives Considered

**Tornar Go e Node estritos, alinhando ao Python** (item 1) — também produziria paridade.
**Rejeitado:** mudaria dois CLIs em vez de um, e o comportamento estrito não é melhor — ambos
falham em silêncio, e o permissivo ao menos encontra o arquivo.

**Suportar lista inline nos três parsers agora** (item 2b) — fecharia o achado novo junto.
**Rejeitado:** é feature nos três CLIs, exige decisão de produto (suportar versus avisar) e não
é divergência de paridade — os três se comportam igual. Misturar com este ciclo, cujo objetivo é
esvaziar a fila antes da tag, ampliaria o escopo sem necessidade.

**Fazer `_list_dirs` ordenar em todos os call sites de uma vez** — mais abrangente.
**Rejeitado como enquadramento:** `_list_dirs` já é a função a corrigir, e ordenar é seu
comportamento correto em qualquer uso. Não há "todos os call sites" a decidir — há uma função com
comportamento inconsistente em relação à irmã.
