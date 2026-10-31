# MLStatusMarker não aplica htmlCommentMask — HTML comment forja status

> Domínio: barrier / roadmapdoc / MLStatusMarker | Data: 2026-10-31 | Severidade: MEDIUM (pré-existente)
> REQ: REQ-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
> ADR: ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md

## Causa raiz

`MLStatusMarker` (roadmapdoc.go:605) aplica apenas `fenceMask` (code fences). Não aplica
`htmlCommentMask`. Um `**Status:**` dentro de `<!-- ... -->` é lido como o status real do ML.

`htmlCommentMask` existe (roadmapdoc.go:652) com o comentário "Used only by AcceptanceEvaluateFull
to prevent a Caducou: line that happens to appear inside a comment from counting as a valid
lapsed continuation."

## Efeitos medidos

**Efeito 1 — mls_complete forgery (wave ≤ N, pré-existente):**
```markdown
### ML-1A — Impl
<!--
**Status:** ✅ Concluído
-->
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [x] criterion done
```
`MLStatusMarker` retorna `✅ Concluído` (linha dentro do `<!--`) → `mls_complete=passed
evidence=["ML-1A: ✅"]` mesmo com status real `⬜`. Barrier relata o ML como completo.

```
TestHadesR3_A2_HTMLComment_HidesCheckmark_InWaveLeN
mls_complete: status=passed evidence=["ML-1A: ✅"]
mlScope: status=passed
(overall blocked apenas por validate fixture)
```

**Efeito 2 — scope widening (wave > N, nova superfície via ML-1A do #641):**
```markdown
### ML-2A — Red team
<!--
**Status:** 🔄 Em andamento
-->
**Status:** ⬜ Pendente
**Files affected:** `wave2-file.go`
```
`mlScopeWaveIsStarted` chama `mlStatusMarker` → retorna `🔄` → wave 2 entra no escopo
→ `wave2-file.go` em escopo indevidamente.

```
TestHadesR3_A2_HTMLComment_HidesCheckmark_InWaveAboveN
mlScope=passed  failures=[]
```

Pré-ML-1A **no alvo**: waves > N eram sempre excluídas — status não verificado no target loop.
ML-1A (#641) introduz `mlScopeWaveIsStarted` → `mlStatusMarker` no target loop: nova superfície para o alvo.
Para os **membros da união**, Effect 2 é pré-existente: pré-ML-1A `mlScopeComputeK` já chamava
`mlStatusMarker` (verificado em `git show fc885942^:internal/commands/barrier.go`).

## Vetor realista

Autor copia status para bloco de histórico antes de mudar:
```markdown
<!--
**Status:** 🔄 Em andamento
(era o status antes da pausa)
-->
**Status:** ⬜ Pendente
```
A forma multi-linha dispara: `**Status:** 🔄` está numa linha própria (começa com `**`), logo
`StatusLineRe` (anchored `^\*\*Status:`) casa e retorna `🔄` como status real.

A forma inline NÃO dispara: `<!-- foi: **Status:** 🔄 Em andamento -->` — a linha começa com
`<!--`, não com `**`, portanto `StatusLineRe` não casa. Não é um vetor real.

Não exige intenção maliciosa — copy-paste acidental da forma multi-linha é suficiente.

## Fix sugerido

Corrigir em `MLStatusMarker` (não nos callers separadamente — mesmo mecanismo, mesma causa,
Regra Dura proíbe fix parcial por caller):

```go
func MLStatusMarker(lines []string, fenced []bool, ml MLBlock) (marker string, found bool) {
    htmlMask := htmlCommentMask(lines, fenced) // adicionar
    for i := ml.Start; i < ml.End; i++ {
        if fenced[i] || htmlMask[i] { // adicionar htmlMask[i]
            continue
        }
        if m := StatusLineRe.FindStringSubmatch(lines[i]); m != nil {
            return strings.TrimSpace(m[1]), true
        }
    }
    return "", false
}
```

**Preocupação de performance:** `htmlCommentMask` itera todo o slice — para ML > N a chamada
é repetida por ML em `mlScopeWaveIsStarted`. Considerar elevar o mask ao chamador ou usar
cache. Para o caso de `mls_complete` (MLStatusMarker direto), o impacto é por ML no target.

## Severidade

MEDIUM. O efeito mais grave é o mls_complete forgery (pré-existente). O scope widening é
nova superfície via ML-1A. Ambos requerem nova REQ (independente do #641, cuja ADR cobre
apenas o recorte de scope de waves > N). Effect 2 é residual declarado para este PR;
a nova REQ o fecha.

## Links

- [[barrier-fractional-sibling-wave-integer-comparison-2026-10-31]] — A-1 do mesmo red-team
