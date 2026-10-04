# Revisão de qualidade — critério de aceite caducado e cortes por data (ML-2B, REQ #514)

> Data: 2026-10-04 | Branch: `fix/fechar-req-com-criterio-de-aceite-permanentemente-inverificavel`
> Revisor: hefesto-tf | Veredito: **APROVA**

Escopo: `git diff origin/main...HEAD -- internal/ docs/cli-parity.md`
ADR de referência: `ADR-2026-10-04-criterio-de-aceite-caducado-…` (D1–D6)

---

## Ambiente no início

```
13:53  up 2 days, 22:29, 4 users, load averages: 2.93 3.10 2.89
 1961  99.5  OrbStack Helper
48186  13.3  chrome-headless-shell
 2108   8.7  claude
47843   8.6  ClaudeBar
22673   2.5  QEMULauncher
48114   2.3  node
 1260   1.8  ghostty
  610   1.7  WindowServer
```

---

## Gate completo

**Arquivo de log:** `/tmp/claude-501/mq514.log`
**Linhas:** 2933
**Última linha:** `EXIT=0`
**Suite de falsificação:** `run-gates-falsify-parallel: suite completa — 8 chunks, 347 OK, 0 FAIL, guarda de conjunto OK (nenhum rótulo esperado ausente)`

Pacotes Go:

```
ok  github.com/kgsaran/trackfw/internal/auditsurface  (cached)
ok  github.com/kgsaran/trackfw/internal/changelog     (cached)
ok  github.com/kgsaran/trackfw/internal/commands      14.454s
ok  github.com/kgsaran/trackfw/internal/config        (cached)
ok  github.com/kgsaran/trackfw/internal/discover      0.685s
ok  github.com/kgsaran/trackfw/internal/forge         (cached)
ok  github.com/kgsaran/trackfw/internal/generators    33.124s
ok  github.com/kgsaran/trackfw/internal/i18n          (cached)
ok  github.com/kgsaran/trackfw/internal/identity      (cached)
ok  github.com/kgsaran/trackfw/internal/integrations  (cached)
ok  github.com/kgsaran/trackfw/internal/metrics       0.194s
ok  github.com/kgsaran/trackfw/internal/pathanchor    (cached)
ok  github.com/kgsaran/trackfw/internal/pathguard     0.868s
ok  github.com/kgsaran/trackfw/internal/roadmapdoc    0.343s
ok  github.com/kgsaran/trackfw/internal/serve         0.241s
ok  github.com/kgsaran/trackfw/internal/sync          0.245s
ok  github.com/kgsaran/trackfw/internal/thirdparty    (cached)
ok  github.com/kgsaran/trackfw/internal/validator     13.625s
```

18/18 packages OK. Os `FAIL` no log pertencem exclusivamente a braços de falsificação (fixtures negativas criadas para provar que os gates detectam regressões) — cada um foi seguido de um `OK [falsify/...]` confirmando o comportamento esperado.

---

## Revisão por arquivo

### `internal/roadmapdoc/roadmapdoc.go`

**Contagem em três classes (D2/D3/D6/T3):**
- Refactoring de `AcceptanceEvaluate` inline → `AcceptanceEvaluateFull` + wrapper backward-compat é correto. O wrapper soma `Unmet + Lapsed` para preservar a semântica original (`d.Unmet + d.Lapsed`), o que é o comportamento correto: "toda contagem existente continua verdadeira" (D2).
- `acceptanceHeader` extraído como função nomeada; reduz a complexidade ciclomática do loop principal.
- `htmlCommentMask`: adição focada, O(n), impede que um `Caducou:` dentro de comentário HTML conte como linha de continuação válida.
- `extractLapsedReason`: sítio único de parsing do texto da justificativa, com truncamento a 120 runes e `…`.
- O `switch/case` em `AcceptanceEvaluateFull` é legível: (1) `[x]`/`[X]` → Met; (2) `[ ]` com próxima linha `Caducou:` válida → Lapsed; (3) `[ ]` sem continuação → Unmet; (4) outro caractere → Unmet + `UnrecognizedLines`.
- `UnrecognizedLines: []int{}` inicializado como slice vazio (não nil) para garantir `[]` no JSON — opção defensiva e documentada.

**`LapsedReason` / `LapsedReasons`:**
- `LapsedReasons []LapsedReason` em `AcceptanceDetail`: nil quando Lapsed == 0 (omitido do JSON por omitempty no barrier). A distinção nil vs. slice vazio é respeitada.

**Duração/complexidade:** nenhuma hot path nova; todas as funções novas são O(n) no número de linhas do documento.

### `internal/commands/barrier.go`

- `acceptanceEvaluateDetail` é um wrapper fino sobre `AcceptanceEvaluateFull` — mantém o barrier testável sem acoplamento direto ao package `roadmapdoc`.
- `appendLapsedDetails` é o único sítio de mapeamento `roadmapdoc.LapsedReason` → `barrierLapsedDetail`. Princípio de ponto único respeitado.
- 5 ramos no switch de `runBarrier`: `!hasBlock` / `unmet > 0` / `met == 0 && lapsed > 0` (T8) / `default (met > 0)`. A visibilidade de critérios lapsed em ramo `unmet > 0` (T2) está correta: lapsed aparecem como informativo mesmo quando o ML está bloqueado.
- `printBarrierText` usa `~` para lapsed (distinto de `-` para failures) e indenta os detalhes 6 espaços: `      line N: Caducou: <text>`.

### `internal/validator/validator_req_done_criteria.go`

- `countREQOpenCriteria` constrói um slice sintético para reutilizar `AcceptanceEvaluateFull` sem duplicar o parsing de cabeçalho AC nem a lógica de fence-mask. Design correto.
- A direção do corte (forward, não amnistia) está comentada com `🔴` e documentada na parity.
- `reqDoneOpenCriteriaAlwaysWarn` retorna nil quando `scanned == 0`, prevenindo uma linha de aviso vazia/vacuosa.
- `validateREQDoneOpenCriteria` é chamada duas vezes em `validator.go` (unfiltered + tagged) — padrão já existente no codebase; o comentário no sítio tagged alerta explicitamente sobre o risco de esquecer.

### `validator_req_roadmap_cutoff.go` / `validator_roadmap_gates.go` / `generators/roadmap.go`

- `RoadmapCreationDate` + `RoadmapWave0CutoffDate` exportados corretamente para `generators/roadmap.go`.
- `validateRoadmapGatesCoverage` passou de 3 para 4 retornos (adicionando `wave0ExemptNotice`); ambos os call sites em `validator.go` foram atualizados (unfiltered + tagged). A isenção é roteada via `applyRuleWarnOnly` — não via `applyRule` — o que garante que seja sempre warning, independente da severidade configurada da regra.
- `MoveRoadmap` em `generators/roadmap.go`: o corte D5 é aplicado antes do teste `if len(blockers) > 0 || missingWave0` — ordem correta.

### Duplication — `reqCreationDate` vs. `RoadmapCreationDate`

Os dois funções partilham ~7 linhas de parsing de frontmatter idênticas. A diferença legítima é o regex de fallback de nome de arquivo:
- `reqFilenameDateRe` ancora em `^REQ-YYYY-MM-DD`
- `roadmapFilenameDateRe` casa a primeira `YYYY-MM-DD` em qualquer posição do basename

A extração de um helper `parseDateFromFrontmatter(content string) (time.Time, bool)` eliminaria a duplicação sem alterar comportamento. **Severidade: BAIXA** — o código é simples, os contextos são distintos, e a deduplicação não é bloqueante para esta PR.

### `docs/cli-parity.md`

Verificado contra a entrega:

| Item | Declarado? | Observação |
|---|---|---|
| Forma `Caducou:` (D2) | Sim | Regras do parser listadas: adjacência, ≥2 espaços, justificativa obrigatória, fence-mask |
| `lapsed` no JSON do `show --json` | Sim | Campo novo, separado de `unmet` |
| Mudança de semântica do `unmet` | **Sim** | "O campo `unmet` do JSON agora é estrito … Consumidores que dependiam do comportamento anterior (`unmet = unmet + lapsed`) devem somar `unmet + lapsed`" |
| `lapsed_details` no JSON do barrier (ML-1D) | Sim | Formato `{"line": N, "text": "…"}`, truncamento 120 runes, omitempty, saída textual `      line N: Caducou:` |
| `all acceptance criteria lapsed` (T8) | Sim | Formato do failure message documentado |
| `unrecognized checkbox` (D6/T3) | Sim | Texto `unrecognized checkbox at line N` |
| `req_done_open_criteria` (D4) | Sim | Severidade warning, corte 2026-10-04, direção oposta ao `req_has_roadmap`, notice agregado |
| Corte Wave 0 (D5) | Sim | Cutoff 2026-09-18, régua de data, fail-closed, notice agregado |
| Gate markers | Sim | Todos 3 blocos `<!-- trackfw-contract: gate=... -->` apontam para os arquivos corretos |

A mudança de semântica do `unmet` é a mais crítica para consumidores. Está declarada de forma explícita e com guidance de migração. **Nenhuma omissão encontrada.**

---

## Achados por severidade

**BLOQUEANTE:** nenhum.

**MÉDIO:** nenhum.

**BAIXO:**

1. `internal/validator/validator_req_roadmap_cutoff.go` — `reqCreationDate` e `RoadmapCreationDate` duplicam ~7 linhas de parsing de frontmatter. Extração de helper reduziria manutenção futura. Não bloqueia; pode ser tratado em PR de housekeeping separado.

---

## Veredito

**APROVA**

EXIT=0 · 18/18 pacotes Go · suite de falsificação 347 OK / 0 FAIL / suite completa · cli-parity.md declara todas as mudanças de contrato incluindo a mudança de semântica do `unmet` · nenhum achado bloqueante.
