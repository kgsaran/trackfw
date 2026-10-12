---
revisor: hefesto-tf
branch: fix/operacoes-de-rede-sem-prazo-e-branch-prune-remoto
data: 2026-10-12
escopo: Wave 2 (ML-2A) — REQ-2026-10-12 (prazo de rede) + REQ-2026-10-10 (branch prune --remote)
---

# Parecer de Qualidade — Wave 2 (ML-2A)

## Suite completa

```
go test ./internal/commands/ ./internal/netdeadline/ ./internal/sync/ ./internal/guard/ -count=1
ok  github.com/kgsaran/trackfw/internal/commands     47.405s
ok  github.com/kgsaran/trackfw/internal/netdeadline   2.418s
ok  github.com/kgsaran/trackfw/internal/sync          7.112s
ok  github.com/kgsaran/trackfw/internal/guard         4.683s

GOOS=windows go vet ./internal/...  → limpo
```

---

## Q1 — Mutation 2: `resolveNetworkErr` com ramos invertidos não é capturada pelos testes unitários

**Severidade:** ajuste

**Evidência:**

Em cópia scratch (`mktemp -d`), inverteu-se a ordem dos ramos de `resolveNetworkErr`
(verifica `exec.ErrWaitDelay+success` antes de `ctx.Err()`). Resultado:
`TestWaitDelay_ResolveNetworkErr` **PASSOU** com a mutação. Os quatro casos testados
são independentes (ctx expirado OU ErrWaitDelay — nunca os dois simultaneamente). O caso
em que ambos coexistem (processo sai 0 imediatamente antes de o contexto expirar, e
WaitDelay depois dispara com `ctx.Err() != nil` já verdadeiro) não está coberto.

**Impacto semântico:** com a ordem invertida esse edge case classificaria como
`isWaitDelaySuccess=true` em vez de `isTimeout=true`, violando o ADR D1. Na prática é
improvável (`cmdWaitDelay=5s` muito menor que timeout=60s), mas a especificação é clara.

**Correção proposta:** acrescentar Case 5 a `TestWaitDelay_ResolveNetworkErr` em
`internal/commands/netexec_test.go`:

```go
// Case 5: ErrWaitDelay + success ProcessState + ctx already expired → (true, false)
cancelledCtx, cancelFn := context.WithCancel(bg)
cancelFn()
fakeSuccessCmd := &exec.Cmd{ProcessState: successState}
isTimeout, isSuccess = resolveNetworkErr(exec.ErrWaitDelay, cancelledCtx, fakeSuccessCmd)
if !isTimeout || isSuccess {
    t.Errorf("ErrWaitDelay+success+expired ctx: want (true,false), got (%v,%v)", isTimeout, isSuccess)
}
```

---

## Q2 — T4 (gate de executor direto) declarado no Wave 0 e não implementado

**Severidade:** ajuste

**Evidência:**

`docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md`, seção 2.5, ameaça T4:

> "O gate real é um teste que varre `internal/` procurando literais de subcomandos de
> rede ('fetch', 'ls-remote', 'push') em chamadas diretas a exec.Command (sem passar por
> defaultGitExec). Falha se qualquer match for encontrado."

O teste de varredura não foi entregue. O único sinal hoje de que um `exec.Command("git",
"fetch", ...)` direto quebraria algo é o `TestGitExec_AC2_WaitDelayOnly`, que testa um
sítio específico — não sítios futuros.

**Correção proposta:** acrescentar teste de varredura em `internal/commands/` ou
`internal/netdeadline/` que percorra `internal/**/*.go` e falhe se `exec.Command` ou
`exec.CommandContext` aparecer com argumento literal `"fetch"`, `"ls-remote"` ou `"push"`
fora dos arquivos `netexec*.go`.

---

## Q3 — `runNetworkCmd` constrói mensagem de erro inline; `defaultGitExec` usa `buildGitError`

**Severidade:** dívida aceitável (observação)

`internal/commands/netexec.go`: `runNetworkCmd` duplica inline a lógica de `buildGitError`
(5 linhas). A diferença de assinatura (`[]string` vs variáveis separadas) torna a
unificação não trivial. Custo < benefício. **Dívida aceitável.**

---

## Q4 — `parseHostOwnerRepo` chamado em 3 sítios independentes

**Severidade:** dívida aceitável (observação)

`forgeDefaultBranch` (`branch_prune_forge.go:74`), `queryForgePRs`
(`branch_prune_forge.go:171`) e `queryProtectedBranches` (`branch_prune_remote.go:272`)
chamam `gitExec("remote", "get-url", "origin")` + `parseHostOwnerRepo` separadamente.
Operação local (leitura de config git), custo negligível. **Dívida aceitável.**

---

## Q5 — `detectPendingSquashMerges` faz duas chamadas ao forge para o mesmo repo

**Severidade:** dívida aceitável (observação)

`ship.go:765`: `resolveDefaultBranch` aciona `ghExec("api", "repos/ownerRepo")`. Linhas
depois, `queryForgePRs` aciona `ghExec("pr", "list", ...)`. Endpoints distintos;
não podem ser fundidos. `gitExec("remote", "get-url", "origin")` é chamado 2× (local).
**Dívida aceitável.**

---

## Q6 — Margem superior do `TestGitExec_AC2_WaitDelayOnly` pode falhar em CI Windows lento

**Severidade:** ajuste

**Evidência:**

`internal/commands/netexec_test.go`:

```go
maxExpected := 3*time.Second + cmdWaitDelay + 1*time.Second  // = 9s
```

Resultado medido (macOS): 8.11s — 0.89s de margem antes do limite de 9s. Em
`windows-latest` do GitHub Actions o overhead de spawn de processo pode acrescentar
~1-2s, colocando o elapsed acima de 9s e disparando o check.

**Correção proposta:**
```go
maxExpected := 3*time.Second + cmdWaitDelay + 3*time.Second  // 11s; margem 4s
```

---

## Q7 — `linear.go`: verifica `ctx.Err()` em vez de `errors.Is(err, ...)` (cosmético)

**Severidade:** observação

`internal/sync/linear.go`: `errors.Is(ctx.Err(), context.DeadlineExceeded)` em vez de
`errors.Is(err, context.DeadlineExceeded)`. Para contexto criado localmente de
`context.Background()` os dois são equivalentes. `TestLinearCreateIssue_Timeout` passa.
Inconsistência cosmética com o padrão dos outros executores.

---

## Q8 — Reconciliação dos 8 testes amostrados

| # | Teste | Conclusão declarada | Mutação correspondente | Resultado |
|---|-------|---------------------|------------------------|-----------|
| 1 | `TestClassifyGitArgs_Push` | "classificador pula -c+valor, retorna 'push'" | M1: `-c` como flag de 1 token | **CAPTURADA** — `got "k=v"` |
| 2 | `TestClassifyGitArgs_LSRemote` | "pula equals-form, retorna 'ls-remote'" | M1 não afeta (usa --git-dir=) | — |
| 3 | `TestWaitDelay_ResolveNetworkErr` | "(true,false) em expiry; (false,true) ErrWaitDelay+success; (false,false) outros" | M2: ramos invertidos | **NÃO CAPTURADA** (ver Q1) |
| 4 | `TestNetworkCmd_WaitDelay_ExitZeroWithHangingGrandchild` | "runNetworkCmd retorna nil quando filho sai 0 e neto prende pipe" | M2 capturaria em integração, mas não testa caso misto | — |
| 5 | `TestRelease_LSRemoteError_FailsClosed` | "erro em ls-remote → falha fechada; nunca 'tag já existe'" | original `_ :=` descartava erro | **CAPTURARIA** |
| 6 | `TestRelease_LSRemoteTimeout_FailsClosed` | "timeout em ls-remote → falha fechada + nomeia TRACKFW_NETWORK_TIMEOUT" | idem | **CAPTURARIA** |
| 7 | `TestRemotePrune_MergedPRContainedTip_Deletes` | "merged_pr + apply → deleteRemoteBranch com SHA + --force-with-lease" | M3 não afeta (testa decisão, não refspec) | — |
| 8 | `TestRunGitBranch_PushDelete_BranchDeleteForms/M_tag_refspec` | "`:refs/tags/v1.0` → reasonPush, não branch-delete" | M3: `isPushBranchDelete` aceita refs/tags/ na passagem 2 | **CAPTURADA** — M_tag_refspec FAIL |

**Resultado geral:** 7/8 testes afirmam corretamente a conclusão declarada e a mutação
correspondente captura a regressão. Exceção: teste #3 (ver Q1).

---

## Q9 — Contrato de erro: sem descarte de erro de rede no diff

**Severidade:** OK

```
git --no-pager diff origin/main...HEAD -- internal/ | grep "^+" | grep "_ :="
```

Nenhum `_ :=` novo em operação de rede. `release.go:310` e `ship.go:801` corrigidos
conforme os testes `TestRelease_LSRemote*` e `TestShip_DetectSquashMerges_GhTimeout*`.
Os `_ :=` remanescentes são todos operações locais documentadas em wave 0, seção 1.3.

---

## Q10 — Portabilidade

**Severidade:** OK (com ressalva de margem → Q6)

- Build tags `netexec_unix.go` (`!windows`) e `netexec_windows.go` (`windows`): corretos.
- `GOOS=windows go vet ./internal/...`: limpo (EXIT=0).
- `TestGitExec_AC2_Bite_NoWaitDelay` e `TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild`
  com skip no Windows: justificados. O caminho WaitDelay-only é coberto por
  `TestGitExec_AC2_WaitDelayOnly` (não skip, força `hasControllingTTY=true`).
- Gap Windows: WaitDelay-success do `defaultGitExec` via hook não tem prova no Windows.
  Aceitável: `applyGroupDeadline` é no-op no Windows e o path cai no mesmo `resolveNetworkErr`.

---

## Q11 — Idioma misto no pacote `netdeadline`

**Severidade:** observação

`TimeoutError.Error()`: `"%s excedeu %s (%s)"` (português).
`Parse()` warnings: `"trackfw: %s=%q is invalid (%v)"` (inglês). Cosmético.

---

## Q12 — `resolveDefaultBranch` × `resolveDefaultBranchWithSource`

**Severidade:** OK

Thin wrapper que descarta o terceiro retorno. Sem duplicação de lógica.

---

## Veredito

**APROVADO COM AJUSTES**

Nenhum achado bloqueante. Três ajustes não bloqueantes corrigíveis na mesma REQ ou como
ML adicional:

- **Q1** `netexec_test.go`: adicionar Case 5 ao `TestWaitDelay_ResolveNetworkErr` (ctx expirado + ErrWaitDelay+success → isTimeout=true)
- **Q2** (novo ML): implementar teste de varredura T4 — declarado no wave 0 seção 2.5 como gate obrigatório e não entregue
- **Q6** `netexec_test.go`: ampliar margem de `TestGitExec_AC2_WaitDelayOnly` de 1s para 3s

Observações sem ação obrigatória: Q3 (duplicação inline de buildGitError), Q4 (parseHostOwnerRepo 3×), Q5 (2 chamadas forge em detectPendingSquashMerges), Q7 (ctx.Err() vs errors.Is cosmético), Q11 (idioma misto em netdeadline).
