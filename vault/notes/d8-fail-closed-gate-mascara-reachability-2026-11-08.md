# D8: `ParseGatesLines` fail-closed — reachability por superfície (2026-11-08)

## Achado

ML-2C (ADR-2026-11-08) adicionou dois checks em `ParseGatesLines` para garantir que anomalias de máscara que escondem o cabeçalho de gates disparam erro (fail-closed) em vez de retornar zero gates silenciosamente.

### Check 1 — cerca não terminada (`openFenceLine > 0 && openFenceLine-1 < waveEnd`)

**Irrechável via `barrier`** (`internal/commands/barrier.go:runBarrier`):
`runBarrier` chama `FenceMaskCheck(lines)` ANTES de chamar `ParseGatesLines`. Se houver cerca não terminada em qualquer parte do documento, `FenceMaskCheck` dispara primeiro com mensagem `"unterminated code fence starting at line N"` e exit 2 — `ParseGatesLines` nunca é chamado.

**Reachável via `validator/Wave0GateDiagnosis`** (`internal/validator/validator_roadmap_gates.go`):
`Wave0GateDiagnosisDetail` só chama `FenceMaskCheck` em modo fail-fast do próprio diagnóstico, mas o check ali é sobre o documento inteiro. O check D8 em `ParseGatesLines` é exercitado quando o chamador não fez `FenceMaskCheck` antes.

### Check 2 — comentário HTML não terminado (`HTMLCommentMaskCheck(lines, fenced, waveEnd)`)

**Reachável via `barrier`** (medido): `runBarrier` NÃO chama `HTMLCommentMaskCheck` antes de `ParseGatesLines`. Resultado medido: exit 2, stderr, sem JSON output (mesmo com `--json`).

**Reachável via `validator`**: `Wave0GateDiagnosisDetail` → `ParseGates` → `ParseGatesLines` → check D8 HTML.

### Residual E3 (open vector)

Uma cerca de 3 backticks que ABRE E FECHA dentro da wave, mascarando o cabeçalho de gates via CommonMark (a cerca ```` ```bash ```` com info string não fecha a cerca externa — só uma ` ``` ` sem conteúdo fecha), retorna `openFenceLine=0` em `scanFences` (cerca fechada). D8 não pode disparar. `ParseGatesLines` retorna zero gates sem erro (fail-open). Corrigir exige rework de `scanFences` para rastrear pares internos. Candidato a ML na REQ vigente.

### Testdata vs corpus live (ROADMAP-2026-08-22-wave-0-de-modelo...)

O arquivo `testdata/corpus/` usa 3 backticks nas linhas 223 e 236; o arquivo live usa 4 backticks. Com 3 backticks o balanceamento desloca, a cerca na linha 460 não fecha (openFenceLine=460), e D8 dispara para wave=2 do corpus congelado. No arquivo live a fence correta (4 backticks) fecha normalmente. `corrigidosPela470` em `compare_baseline_test.go` registra esta divergência esperada.

## Arquivos relevantes

- `internal/roadmapdoc/roadmapdoc.go` — `ParseGatesLines`, `HTMLCommentMaskCheck`, `htmlCommentMaskKernel`, `scanFences`
- `internal/commands/barrier.go` — `runBarrier` (chama `FenceMaskCheck`, NÃO `HTMLCommentMaskCheck`)
- `internal/validator/validator_roadmap_gates.go` — `Wave0GateDiagnosisDetail`
- `internal/roadmapdoc/gates_lines_test.go` — `TestParseGatesLines_D8_UnclosedFenceBeforeHeaderReturnsError`, `TestParseGatesLines_D1_UnclosedHTMLCommentReturnsError`, `TestParseGatesLines_E3_MaskedOnlyHeaderReturnsError`
- `internal/roadmapdoc/compare_baseline_test.go` — `corrigidosPela470` entrada `wave=2, check=gates`


**Atualização 2026-11-08 (ML-2F):** o residual E3 foi fechado. Uma wave sem cabeçalho de gates válido e com cabeçalho só mascarado (cerca balanceada ou comentário HTML) agora retorna erro. Contratos antigos que afirmavam "marcador mascarado sozinho = zero gates" (F1, AC11 e o teste do arquivo real `done/ROADMAP-2026-08-22-...` wave 2) foram invertidos de propósito. No acervo há um único caso, em `done/`, que não roda mais barrier.
