---
date: 2026-10-01
reviewer: hefesto-tf
roadmap: ROADMAP-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md
branch: fix/estado-que-governa-a-branch
base: e104a7f7
head: 57aed5b4
---

# Revisão de Qualidade — ML-3B

## 1. make quality

```
$ make quality > /tmp/quality.log 2>&1; echo "EXIT=$?"
EXIT=0
```

35 pacotes Go: todos `ok`. Suite falsify: 347 OK, 0 FAIL. Saída completa inspecionada — todos
os `FAIL` no log estão dentro de braços de auto-teste (fixtures sintéticas injetadas para
provar que a gate detecta violações), nunca no produto.

## 2. D6 intacto — relação de casamento não tocada

```
$ git diff e104a7f7..HEAD -- internal/validator/validator.go \
    | grep -nE "^[-+].*(MatchRoadmapsForBranchSlug|branchRoadmapTokens|roadmapContentSlug|sharedTokenCount)|^[-+].*branchRoadmapMin"
(saída vazia)
```

Nenhuma das cinco funções/constantes protegidas pelo D6 foi modificada.

## 3. Gate do corpus (AC14 — ADR-2026-09-26)

```
$ GO_BIN=bin/trackfw bash scripts/check-roadmap-slug-matching.sh
OK   [manifest/coherence (205 = 190 accept (176 substring + 14 tokens) + 15 block)]
OK   [pin/min_shared_tokens (2)]
OK   [pin/min_token_len (3)]
check-roadmap-slug-matching: binário de medição = bin/trackfw
OK   [census/materialize (201 roadmaps, 205 branches)]
OK   [census/verdicts (205 branches, 190 accept / 15 block, zero divergência)]
OK   [calibration/materialize (1 roadmaps, 1 branches)]
OK   [calibration/verdicts (1 branches, 1 accept / 0 block, zero divergência)]
check-roadmap-slug-matching: OK (7 verificações; 205 branches × 201 roadmaps + 1 caso de calibração)
```

Nenhuma mudança de veredito: corpus congelado intacto, relação de casamento preservada.

## 4. Fonte única — resolução não reimplementada em commands/

```
$ grep -n "matchSlug\|ResolveBranchRoadmap\|wip.*done\|done.*wip" \
    internal/commands/commit.go internal/commands/branch.go \
    internal/commands/push.go internal/commands/ship.go \
    | grep -v _test
```

Resultado relevante:

- `branch.go:112` — `matchSlug: validator.BranchSlugMatchesRoadmap` (injeção, não reimplementação)
- `branch.go:172` — `deps.matchSlug(normalizedSlug, wipDirs, nil)` (delega)
- `commit.go:37–57` — `defaultResolveRoadmap` delega para `validator.ResolveBranchRoadmapForExisting`
- `commit.go:353` — chama `deps.resolveRoadmap(cfg, branch)` (função injetada)

`push.go`/`ship.go` passam por `validator.CheckShipGovernance` → `validateBranchHasWIPRoadmap`
(confirmado no diff: nenhum `matchSlug` direto nesses arquivos).

Nenhuma resolução branch↔roadmap reimplementada em commands/.

## 5. Mensagens de governança — fonte única

```
$ grep -n "BranchGovernanceOrientation\|BranchNoMatchingRoadmapMessage" \
    internal/commands/branch.go internal/commands/commit.go \
    | grep -v _test
```

Resultado:
- `branch.go:182` — `validator.BranchGovernanceOrientationForCreation`
- `branch.go:184` — `validator.BranchNoMatchingRoadmapMessageForCreation`
- `commit.go:376` — `validator.BranchGovernanceOrientationForExisting`
- `commit.go:378` — `validator.BranchNoMatchingRoadmapMessageForExisting`

Todas as mensagens delegam ao validator. Nenhum literal de mensagem de governança duplicado em
commands/.

Funções sem sufixo (`BranchGovernanceOrientation`, `BranchNoMatchingRoadmapMessage`) não existem
mais no validator.go (confirmado: grep vazio). Substituídas pelas variantes parametrizadas.

## 6. Código morto — confirmado removido

```
$ grep -n "splitNewlines\|TestGitLsTree_AccentedFilename_OldBehavior" \
    internal/auditsurface/gitlstree_test.go
(saída vazia)

$ grep -n "TestBranchStateE2E_InformativeMeasure217Branches" \
    internal/commands/branch_state_e2e_test.go
(saída vazia)
```

Os três artefatos apontados nos ML-1B/ML-1C/ML-2B foram removidos.

## 7. Comentários desatualizados — ENCONTRADOS

```
$ grep -n "nor done/\|wip/ or done" \
    internal/commands/branch.go internal/commands/commit.go \
    | grep -v _test
```

Resultado:

```
internal/commands/branch.go:16:  // gated on a matching REQ + roadmap already in wip/ or done/
internal/commands/branch.go:28:  // already in wip/ or done/ before the branch is created.
internal/commands/branch.go:80:  Short: "...gated on a matching REQ + roadmap already in wip/ or done/"
internal/commands/commit.go:23:  // wip/ or done/ before a commit is allowed
internal/commands/commit.go:80:  wip/ or done/ — the exact matching logic [em Long:]
```

**Análise:**

| linha | tipo | defeito |
|---|---|---|
| `branch.go:16` | doc comment | D1 é só `wip/`; comentário diz `wip/ or done/` |
| `branch.go:28` | doc comment | idem |
| `branch.go:80` | `Short:` (user-visible no `--help`) | idem; **único visível ao usuário** |
| `commit.go:23` | doc comment | D2 inclui `blocked/`; comentário omite |
| `commit.go:80` | `Long:` (user-visible) | idem |

A lógica está correta: `branch.go:172` chama `matchSlug(normalizedSlug, wipDirs, nil)` (wip/ only, D1);
`commit.go:353` chama `deps.resolveRoadmap` → `ResolveBranchRoadmapForExisting` (wip/+blocked/+done/, D2).
Os comentários mentem sobre os dois contratos.

O `Short:` de `branch.go` (linha 80) e o `Long:` de `commit.go` (linha 80) são saída de `--help` —
não são comentários internos. Um usuário que lê `trackfw branch new --help` vê "wip/ or done/" para
um gate que é wip/-only.

## 8. Checagem do `nor done/` em commands/ (AC do ML-1B)

```
$ grep -rn "nor done/" internal/commands/*.go | grep -v _test.go
internal/commands/barrier.go:132: ...not found in wip/ nor done/...
```

Único resultado: `barrier.go`, que está fora do escopo desta REQ. Confirma AC do ML-1B.

---

## Veredito: APROVA COM AJUSTES

Gates: EXIT=0. D6 intacto. Corpus sem divergência. Fonte única preservada. Código morto removido.

**Ajustes (cosméticos, sem impacto em lógica ou testes):**

1. `internal/commands/branch.go:16` — substituir `wip/ or done/` por `wip/ only (D1 of ADR-2026-10-01)` no doc comment de `branchValidTypes`.
2. `internal/commands/branch.go:28` — substituir `already in wip/ or done/ before the branch is created` por `already in wip/ before the branch is created (D1 of ADR-2026-10-01)` no doc comment de `branchGatedTypes`.
3. `internal/commands/branch.go:80` (`Short:`, user-visible) — substituir `gated on a matching REQ + roadmap already in wip/ or done/` por `gated on a matching REQ + roadmap in wip/ (done/ roadmaps named in error message)`.
4. `internal/commands/commit.go:23` — substituir `wip/ or done/ before a commit is allowed` por `wip/, blocked/, or done/ before a commit is allowed (D2 of ADR-2026-10-01)`.
5. `internal/commands/commit.go:80` (`Long:`, user-visible) — substituir `wip/ or done/ — the exact matching logic` por `wip/, blocked/, or done/ — the exact matching logic`.
