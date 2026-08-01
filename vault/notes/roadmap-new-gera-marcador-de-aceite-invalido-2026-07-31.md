# `trackfw roadmap new` gera roadmap que falha no próprio `validate`

> Data: 2026-07-31 | Autor: Zeus | Domínio: generators / validator

## Sintoma

Roadmap recém-criado com `trackfw roadmap new`, movido para `wip/` com
`trackfw roadmap move <nome> wip`, falha imediatamente no `trackfw validate`:

```
✗ roadmap "ROADMAP-....md" is in wip but has no acceptance criteria block
```

Nenhuma edição manual foi feita. O artefato saiu do gerador oficial e é rejeitado pelo
validador oficial na primeira transição de estado.

## Causa raiz

Divergência entre o marcador **gerado** e o marcador **procurado**.

- `internal/generators/roadmap.go:115` e `:156` emitem `**Acceptance criteria:**` —
  texto em negrito, por microlote.
- `internal/config/config.go:83` define
  `AcceptanceMarkers: []string{"## Acceptance Criteria", "## Critérios de Aceite"}` —
  **headings de nível 2**.
- `internal/validator/validator.go:989` (`validateWIPHasAcceptanceCriteria`) usa
  `contentHasMarker(s, cfg.AcceptanceMarkers)`, que exige o heading.

`**Acceptance criteria:**` nunca casa com `## Acceptance Criteria`. O gerador e o validador
discordam sobre o que é um "bloco de critérios de aceite".

Note que `internal/generators/req.go:93` emite corretamente `## Acceptance Criteria` — a
divergência é específica do gerador de roadmap.

## Contorno

Acrescentar manualmente um heading `## Critérios de Aceite` (ou `## Acceptance Criteria`)
no roadmap, além dos blocos `**Critérios de aceite:**` por microlote. O heading consolidado
no topo satisfaz o validador; os blocos por ML continuam sendo a fonte operacional.

Alternativa: declarar `acceptance_markers:` no `trackfw.yaml` incluindo
`"**Acceptance criteria:**"`. Não recomendado — mascara o defeito em vez de corrigi-lo.

## Armadilha relacionada: slug de branch

Na mesma sessão, o `validate` também acusou:

```
✗ branch "feat/dashboard-adr-req-list-views" has no matching roadmap in wip/ nor done/
```

`internal/validator/validator.go:1737-1746` extrai o slug após a primeira `/` do nome da
branch e exige `strings.Contains(normalizeBranchSlug(filename), branchSlug)`. Ou seja:
**o slug da branch precisa ser substring do nome do arquivo do roadmap**, não o contrário.
Uma branch com nome mais curto e diferente do título do roadmap falha.

Prática: nomear a branch a partir do slug do roadmap gerado, não inventar um nome curto.
No caso, `feat/views-de-lista-para-adrs-e-reqs-no-dashboard` para o roadmap
`ROADMAP-2026-07-31-views-de-lista-para-adrs-e-reqs-no-dashboard.md`.

## Impacto

Todo agente que criar um roadmap pelo caminho oficial e mover para `wip` vai bater nessas
duas violações e perder tempo investigando um "erro" que é do próprio tooling. Os dois
casos custam facilmente 10–20 min sem esta nota.

## ✅ CORRIGIDO em 2026-08-01

Entregue no **PR #96** (`ROADMAP-2026-07-31-alinhar-marcador-de-criterios-de-aceite-do-gerador-de-roadmap`,
ADR `ADR-2026-07-31-gerador-de-roadmap-emite-heading-consolidado-...`).

Os três geradores passaram a emitir, além dos blocos `**Acceptance criteria:**` por microlote, um
heading consolidado `## Acceptance Criteria` após o contexto e antes da primeira wave — nos dois
caminhos (`roadmap new` e `roadmap new --from-req`). `roadmap new` → `move wip` → `validate` agora
passa **sem edição manual**.

Rejeitado explicitamente: relaxar `AcceptanceMarkers` para aceitar o marcador em negrito. Isso
mascararia o defeito e esvaziaria a regra do validador.

**Proteção de CI:** cenários `roadmap-acceptance-heading/{go,node,python}/{simple,from-req}` em
`scripts/check-gates-falsify.sh`. Contador de cenários subiu 24 → 30.

**A armadilha do slug de branch, abaixo, CONTINUA VÁLIDA** — não foi alterada por aquele PR.

## Correção sugerida na época (histórico)

Alinhar `internal/generators/roadmap.go` para emitir também um heading
`## Acceptance Criteria` consolidado — **nos três CLIs** (Go, npm, pypi), pela regra dura
de paridade. Não corrigido nesta sessão para não expandir o escopo da REQ em andamento.
