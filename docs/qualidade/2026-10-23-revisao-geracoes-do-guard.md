# Revisão de qualidade — gerações do guard (ML-2A)

> Agente: hefesto-tf | Data: 2026-10-23
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-...`
> Branch: `fix/update-nao-remove-geracoes-antigas-do-credential-guard`
> Diff revisado: `git diff origin/main...HEAD -- internal/`

## Veredito

**REPROVADO — 1 bloqueante (B1).**

O build (`go build ./...`) rc=0. Os testes relevantes passam. A estrutura do registro D12 e os
helpers D13 estão corretos. O único bloqueio é uma violação de D12 por literal cru que escapou do
saneamento da Wave 1, com AC2 passando porque o teste procura nomes de variável `legacy*Cmd*` e
não captura literais diretos.

---

## B — Bloqueantes

### B1 — D12 violada: literal cru `"scripts/trackfw-credential-guard.sh"` em `InjectGeminiHooks`

**Arquivo:** `internal/generators/agentfiles.go` linhas 808–819.

**Evidência:**

```
808  migrateHookCommand(hooks["BeforeTool"], "run_shell_command",  "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
810  migrateHookCommand(hooks["BeforeTool"], "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
812  migrateHookCommand(hooks["BeforeTool"], "write_file|replace", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
815  migrateHookCommand(hooks["AfterTool"],  "run_shell_command",  "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
817  migrateHookCommand(hooks["AfterTool"],  "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
819  migrateHookCommand(hooks["AfterTool"],  "write_file|replace", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
```

O alias `legacyBareCredGuardCmd = guardforms.LegacyBareCredGuard` está definido na mesma seção de
constantes da `agentfiles.go` (linha 535). Estas seis chamadas usam o literal em vez do alias.

**Por que é bloqueante:**

D12 (ADR-2026-10-04 adendo 2026-10-23) exige: "Nenhum sítio escreve a própria cópia." Estas
chamadas escrevem exatamente isso: uma cópia local do valor de `LegacyBareCredGuard`. Se o valor do
registro mudar, há dois sítios a atualizar — exatamente o que D12 proíbe.

**Por que AC2 não captura:**

AC2 testa com grep por `legacy*Cmd*` fora do registro. O literal `"scripts/trackfw-..."` não casa
com esse padrão. O teste passa enquanto a violação persiste.

**Agravante — redundância funcional:**

As linhas 847–850 de `InjectGeminiHooks` já passam `guardforms.CredentialLegacyPSPOSIX` (que inclui
`LegacyBareCredGuard`) para `migrateDedupGuardInNestedArray`. As seis `migrateHookCommand` com
literal cru são hoje duplamente desnecessárias: violam D12 e são redundantes.

**Corretivo esperado (apolo-tf):** substituir `"scripts/trackfw-credential-guard.sh"` por
`legacyBareCredGuardCmd` nas seis chamadas, ou remover as seis chamadas (dado que
`migrateDedupGuardInNestedArray` já cobre esse literal via registro).

---

## R — Recomendações

### R1 — Duplicação de código entre `dedupFlatGuardEntries` e `migrateDedupGuardInFlatArray`

**Arquivo:** `internal/generators/guard_generations.go` linhas 43–115 (`migrateDedupGuard`) e
367–428 (`dedupFlatGuardEntries`).

Os passes 1 e 2 (reescrita legada→atual e deduplicação) são idênticos nos dois helpers: ~55 linhas
de código duplicado. `dedupFlatGuardEntries` omite o passe 3 intencionalmente. O padrão mais limpo
seria `migrateDedupGuardInFlatArray` delegando para `dedupFlatGuardEntries` e adicionando o passe 3,
eliminando a duplicação.

### R2 — Nomes de teste atrelados a microlote (`_ML1C_`)

**Arquivos:** `internal/generators/guard_harness_dedup_ml1c_test.go` (todos os 9 subtestes
`TestHarness_ML1C_*`, `TestWindsurf_ML1C_*`, `TestAmazonQ_ML1C_*`).

Nomes como `TestHarness_ML1C_Codex_Credential_NLegaciesDedup` acoplam o nome ao microbatch de
implementação, não ao comportamento. Quando um próximo ML corrigir os mesmos sítios, o sufixo
ficará enganoso. Renomear para comportamento (ex.: `TestHarness_Codex_Credential_NLegaciesDedup`)
seria mais duradouro.

### R3 — Teste `TestNestedMigrate_Claude_AC4_EveryRegistryFormMigrates` subsumed por `TestAC4_RegistrySites`

**Arquivo:** `internal/generators/nested_guard_migrate_dedup_test.go` linhas 277–336.

Este teste verifica que cada forma de `CredentialLegacyPSPOSIX` migra para a forma canônica no par
`claudeShellMatcher` de Claude. `TestAC4_RegistrySites` (`guard_registry_sites_ac4_test.go:124`)
cobre o mesmo par — e mais. O teste mais antigo não acrescenta cobertura nem falsificação
adicional, tornando-o decorativo.

### R4 — Risco latente de aliasing de slice em `InjectAmazonQHooks`

**Arquivo:** `internal/generators/agentfiles.go` linha 1881.

```go
hooks["preToolUse"] = migrateDedupGuardInNestedArray(
    hooks["preToolUse"], "execute_bash", guardGitBranchCmdCmdExe,
    append(guardforms.GitBranchLegacyCmdExe, guardforms.GitBranchLegacyPSPOSIX...))
```

**Medição:** `guardforms.GitBranchLegacyCmdExe` tem `len=1`, `cap=1` na inicialização (variável de
pacote `var GitBranchLegacyCmdExe = []string{LegacyD2GitBranchCmdExe}`). Com `len == cap`, o
`append` aloca novo array — confirmado em `go run`. O registro original não é mutado.

**Risco latente:** se uma segunda forma for acrescentada a `GitBranchLegacyCmdExe`, o `cap` pode
ser 2 e o `append` mutará o array do registro em corridas onde a função é chamada. O padrão seguro
(conforme a linha 1880 logo acima) é:

```go
append(append([]string{}, guardforms.GitBranchLegacyCmdExe...), guardforms.GitBranchLegacyPSPOSIX...)
```

### R5 — Constantes marcador do validador desacopladas do registro

**Arquivo:** `internal/validator/validator_credential_guard.go` linhas 18 e 38.

```go
const credentialGuardScriptMarker = "trackfw-credential-guard.sh"
const gitBranchGuardSubcmdMarker  = "trackfw guard git-branch"
```

Estas constantes são usadas para `strings.Contains` (detecção de substring), não para comparação
exata de forma. Não são membros do registro D12. Mas `"trackfw-credential-guard.sh"` é o
`path.Base(guardforms.LegacyBareCredGuard)`, e `"trackfw guard git-branch"` é substring de
`guardforms.LegacyD2GitBranchPSPOSIX`. Se o nome do script ou o subcomando mudar, há dois sítios a
atualizar. Derivar essas constantes do registro eliminaria a divergência latente.

---

## Validação executada

```
go build ./...                                               rc=0
go test ./internal/generators/ ./internal/validator/ \
  -run 'TestCursorGuardMigrateDedup|TestNestedMigrate|TestDedup_|TestHarness_ML1C|TestWindsurf_ML1C|TestAmazonQ_ML1C|TestAC4_RegistrySites|TestAC2_' \
  -count=1                                                  ok (2 pacotes)
```

## git status --short (worktree /private/tmp/trackfw-toml-escape)

Verificado via `git diff origin/main...HEAD -- internal/ --stat`: sem arquivos novos de produto
neste ML. Apenas este arquivo de findings foi criado.
