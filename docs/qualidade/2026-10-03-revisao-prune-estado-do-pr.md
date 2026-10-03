# Revisão de Qualidade — ML-2B: branch prune consulta o estado do PR (#481)

**Data:** 2026-10-03
**Branch:** `fix/branch-prune-consulta-o-estado-do-pr`
**Revisor:** hefesto-tf
**Escopo:** `git diff origin/main...HEAD -- internal/ docs/cli-parity.md`

---

## Pré-condição: carga do sistema

```
uptime: 17:17  up 2 days, 1:52, load averages: 5.24 4.85 3.75
processo mais pesado: webpack (155 %CPU) — não é go test nem make
nenhum go test ou make em execução antes do gate
```

---

## 1. Manutenibilidade

### 1.1 Tamanho e legibilidade de `evaluateBranchWithForge`

Arquivo: `internal/commands/branch_prune_forge.go`, linhas 270–450 (181 linhas).

A função segue a ordem D1 do ADR com seções numeradas e comentadas (case 0, 1, 2/2b, 3, 4, 5). O único trecho genuinamente denso é o laço interno sobre `mergedValidBase` (linhas 330–387), que implementa a máquina de prioridade `mergedOutcomeNone < HeadAbsent < Diverged < Commits < delete-imediato`. A escolha de declarar `mergedOutcome` como tipo local dentro da função é idiomática em Go para máquinas de estado de curto alcance e não prejudica testabilidade (os resultados são expostos via `branchPruneEvaluation`). O comentário sobre `mergedOutcomeNone` no switch final (L409-410: "shouldn't happen since len(mergedValidBase) > 0") documenta o invariante e não exige código adicional.

**Avaliação:** legível e justificado; 181 linhas é o limite superior aceitável dado que a complexidade é intrínseca à ordem D1, não artificial. Nenhum refatoramento necessário.

### 1.2 Nomes e duplicação

- `upstreamFor`, `objectExists`, `isNotAncestorError`, `evaluateBranchWithForge`, `queryForgePRs`, `parseHostOwnerRepo`: nomes precisos, responsabilidade única.
- `isReviewDecision` (branch_prune.go) elimina a repetição do OR triplo (`review_doc_config || closed_pr || merged_head_absent`) em `runBranchPrune`. Decisão correta.
- As constantes de decisão (`branchPruneDecisionMergedPR`, `branchPruneDecisionOpenPR`, etc.) seguem o padrão existente.
- Sem duplicação observada no diff.

### 1.3 `docs/cli-parity.md` — descrição do comportamento entregue

**O que está correto:**

- A descrição da ordem D1 está presente inline na seção `branch prune` (linhas 1954-1958): "open PR → keep; MERGED PR containing tip → delete; MERGED PR with commits after → keep; MERGED PR diverged → keep; closed PR → review; no PR no upstream → keep; no PR with upstream → content heuristic".
- Degradação descrita: "On degradation (no `gh`, non-GitHub remote, error, or truncated response) the content heuristic below applies and one line naming the cause is printed." Corresponde à linha `Note: forge PR signal not available — using content heuristic only. Cause: X` emitida por `runBranchPrune`.
- Categorias de review atualizadas (linha 2064-2066): inclui `closed_pr` e `merged_head_absent` além de `review_doc_config`.
- Comportamento de `detectPendingSquashMerges` para ship/push atualizado (linhas 2072-2084): lista as três decisões que disparam aviso e as que ficam silenciosas.

**Gap menor (A1):**

A descrição inline da ordem D1 diz `"no PR no upstream → keep"`. O comportamento real — conforme o código (branch_prune_forge.go L424-441) e o teste `TestAJ1_NeverPushed_MergedPRInNonMainBase_NoPRNeverPushed` — é: **"no upstream → keep"**, independentemente de haver PR com base != main. O AJ1 corrigiu exatamente esse caso: uma branch com PR MERGED para base != main (fora de `mergedValidBase`) e `upstream == ""` deve ser mantida porque pode ser a única cópia do trabalho. A palavra "no PR" na descrição inline induz a leitura de que a guarda só dispara quando não há PR nenhum — o que é impreciso. O comportamento em código e teste está correto; apenas a descrição pública é levemente enganosa.

Ajuste executável: na linha 1956 de `docs/cli-parity.md`, substituir `"no PR no upstream → keep"` por `"no upstream → keep (even with PR to non-main base — AJ1)"`.

**Nota sobre `ship`/`push` e degradação silenciosa:**

`detectPendingSquashMerges` em ship.go (L801) descarta o motivo de degradação com `snapshot, _ := queryForgePRs(...)`. Isso é intencional — o caminho é advisory-only, não destrutivo — mas a cli-parity não menciona explicitamente que ship NÃO imprime a linha `Note:` em caso de degradação (ao contrário do `branch prune`). Baixo impacto; não é bloqueante.

### 1.4 Acoplamento `isNotAncestorError` / `defaultGitExec`

`isNotAncestorError` (branch_prune_forge.go, L243-248) usa `strings.HasSuffix(err.Error(), "exited with 1")` para distinguir exit 1 (não-ancestral) de exit 128 (erro fatal). Esse predicado depende do formato de mensagem produzido por `defaultGitExec` (ship.go, L174-175): `fmt.Sprintf("git %s exited with %d", strings.Join(args, " "), exitErr.ExitCode())`.

O acoplamento é aceito e gerenciado:

1. O comentário em `isNotAncestorError` documenta a dependência e explica por que `HasSuffix` é obrigatório ("`exited with 128` contains `exited with 1` as substring").
2. `TestIsNotAncestorError` (branch_prune_forge_test.go, L665) verifica os três casos de borda (exit 1, exit 128, erro sem código).
3. `TestL4_DefaultGitExec_IsAncestorFormat` (branch_prune_forge_test.go, L1311) cria um repositório git real, chama `defaultGitExec` com `merge-base --is-ancestor` para dois commits não-ancestrais, e afirma que `isNotAncestorError` retorna `true` para exit 1 e `false` para exit 128 (OID inválido). Qualquer mudança no format string de `defaultGitExec` quebra esse teste imediatamente.

**Avaliação:** o acoplamento é aceitável. O `TestL4` é o amarre correto — ele testa a junção dos dois lados, não cada lado isolado. Refatorar para erro tipado exigiria mudar a assinatura de `gitExec func(...string) (string, error)` em toda a base, custo injustificado pelo benefício marginal.

---

## 2. Gate completo

### 2.1 Execução

```
comando: cd /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw && \
         make quality > <log> 2>&1; echo EXIT=$? >> <log>
```

### 2.2 Resultados

| Métrica | Valor |
|---------|-------|
| EXIT | 0 |
| Linhas no log | 3001 |
| Linhas `^(FAIL\|--- FAIL)` | 14 |
| Suíte de falsificação completa | sim |

### 2.3 Os 14 `FAIL` são esperados

Todos os 14 aparecem dentro de seções com cabeçalho "gate must FAIL" na saída da suíte de falsificação — são os arms injetando violações sintéticas para provar que o gate as detecta. Nenhum é falha real. Exemplo característico (linha 1024 do log):

```
=== Arm 1 (braço 1): 3 unmarked consumer writes — gate must FAIL naming each site ===
FAIL [trackfw.yaml] line 7 in writeTrackfwConfig — unconditional write ...
```

### 2.4 Resultado de cada pacote Go

```
ok   github.com/kgsaran/trackfw/internal/commands    15.621s   ← não cached; exercita os novos testes
ok   github.com/kgsaran/trackfw/internal/generators  35.429s
ok   github.com/kgsaran/trackfw/internal/validator   14.952s
(demais: cached, todos ok)
```

### 2.5 Suíte de falsificação

```
run-gates-falsify-parallel: suite completa -- 8 chunks, 347 OK, 0 FAIL,
guarda de conjunto OK (nenhum rotulo esperado ausente)
```

A suíte rodou até o fim — confirmado na última linha do log, não no resumo de um chunk intermediário.

---

## 3. Veredito

**APROVA COM AJUSTES**

### A1 — Baixo: imprecisão na descrição inline de case 4 em `docs/cli-parity.md`

Na linha 1956 de `docs/cli-parity.md`, substituir `"no PR no upstream → keep"` por `"no upstream → keep (even with PR to non-main base — AJ1)"`.

Justificativa: o comportamento real é `upstream == ""` dispara o guarda independentemente de existir PR para base != main. A expressão "no PR" na descrição atual induz leitura incorreta para o caso AJ1, que é exatamente o cenário que a correção protege. O código e o teste estão corretos; apenas a documentação pública precisa de ajuste.

---

*Nenhum outro ajuste necessário. O gate está verde, a cobertura do novo código é adequada, e o acoplamento L4 está devidamente amestrado pelo TestL4.*
