---
status: Done
date: 2026-07-30
author: "trackfw_architect"
adr: "docs/adr/ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md"
---

# REQ: roadmap move sincroniza a referencia da REQ pareada

> Date: 2026-07-30 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

`trackfw roadmap move` sincroniza a pasta **e** o `status:` do frontmatter do roadmap, mas **não**
atualiza a referência `roadmap:` da REQ pareada, que continua apontando para o estado anterior. O
comando criado para cumprir a governança **produz um estado que o próprio validador reprova**:

```
ref_targets_exist: req "REQ-....md" links to Roadmap
  "docs/roadmaps/backlog/ROADMAP-....md" which does not exist
```

Constatado empiricamente **quatro vezes em duas sessões consecutivas** (2026-07-29 e 2026-07-30), nos
dois roadmaps executados: ao mover para `wip` e ao mover para `done`, foi necessário corrigir a
referência da REQ à mão em todas as transições. Não é caso de borda — acontece em **toda** transição de
roadmap com REQ pareada, isto é, em toda demanda não trivial.

A correção do `status:` do frontmatter do roadmap já foi entregue em
`REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato`. Esta REQ fecha a metade que ficou de
fora: a referência do lado da REQ.

## Fatos verificados no código — pinar antes de implementar

1. **A referência normativa é a do frontmatter, não a do corpo.** `extractRefPath`
   (`internal/validator/validator.go:1426`) percorre as linhas, casa a chave `Roadmap` sem distinguir
   maiúsculas e retorna o **primeiro** valor terminado em `.md`. Faz `strings.Trim(fields[0], "\"'")` —
   apenas aspas, **não** backticks. Logo a forma do corpo
   (`` Roadmap: `docs/roadmaps/wip/X.md` ``) termina em backtick, não casa `.md` e é **ignorada** pelo
   validador. Só o frontmatter (`roadmap: "docs/..."`) é avaliado. Implementar acreditando que o corpo
   é normativo levaria a "corrigir" o lugar errado.
2. **O `req:` do roadmap não serve para descobrir o par.** `trackfw roadmap new` grava `req: ""`, e os
   roadmaps existentes carregam ali um slug sem caminho e sem `.md`. A descoberta confiável é a
   inversa: varrer o `req_dir` procurando REQs cujo `roadmap:` tenha o **basename** do roadmap movido.
3. **Formato canônico da referência** (`docs/cli-parity.md`, § "Canonical governance references"):
   caminho completo a partir da raiz do projeto, incluindo `.md`. O validador compara literalmente, sem
   fallback por basename — `wip/X.md` quando o arquivo está em `done/` é inválido.
4. **`req_dir` pode ser namespaced por agente.** O validador já cobre isso
   (`internal/validator/validator.go:840-862`, varrendo `reqDir/<agente>/<estado>/*.md` além de
   `reqDir/*.md`). A varredura desta feature deve cobrir o mesmo terreno, senão a REQ pareada não é
   encontrada em projetos `by_agent`.
5. **O move já reescreve conteúdo com segurança.** `MoveRoadmap`
   (`internal/generators/roadmap.go:326`) faz `os.Rename` e então `rewriteRoadmapStatus` sobre o
   destino. A escrita da REQ deve ocorrer **após** o rename bem-sucedido, no mesmo ponto do fluxo.

## Acceptance Criteria

- [x] `roadmap move` reescreve o `roadmap:` do frontmatter de **toda** REQ que aponte para o roadmap
      movido, com o caminho completo do novo estado, incluindo `.md`.
- [x] A linha `Roadmap:` do **corpo** da REQ também é atualizada, preservando a formatação existente
      (inclusive backticks). Não é lida pelo validador, mas divergir do frontmatter engana o leitor.
- [x] Descoberta do par por varredura do `req_dir` casando o **basename** do roadmap, cobrindo layout
      flat **e** `by_agent`.
- [x] **Zero** REQs apontando para o roadmap → no-op silencioso, exit 0. Roadmap sem REQ é legítimo.
- [x] **Múltiplas** REQs apontando para o mesmo roadmap → todas atualizadas, uma linha de saída por REQ.
- [x] REQ cujo `roadmap:` aponta para **outro** roadmap → não tocada.
- [x] Referência já correta → nenhuma escrita (idempotente); mover duas vezes seguidas não altera bytes.
- [x] Saída informa cada REQ atualizada; nenhuma REQ atualizada não imprime ruído.
- [x] Falha ao reescrever uma REQ **não** desfaz o move e **não** falha em silêncio: diagnóstico
      nomeando a REQ e exit não-zero.
- [x] Paridade nos três CLIs, com cenário de comparação byte-a-byte encadeado em `make quality`.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Escopo negativo

- **Não** alterar o campo `req:` do roadmap. A sincronização é unidirecional: o move conhece o novo
  caminho do roadmap e corrige quem aponta para ele.
- **Não** criar REQ ausente, nem inferir pareamento por semelhança de slug. Só referências explícitas
  são atualizadas.
- **Não** mudar o formato canônico da referência.

## Linked ADR
ADR: `docs/adr/ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md`

Não altera decisão arquitetural: completa a implementação da cadeia `REQ ↔ ROADMAP` que o ADR já
estabelece. Se a análise revelar necessidade de decisão nova, o ML-1A abre emenda.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`
