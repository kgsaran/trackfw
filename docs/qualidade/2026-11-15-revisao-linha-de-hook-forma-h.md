# Revisão de Qualidade — Linha de Hook Forma H (ML-2A, REQ-2026-11-13)

**Revisor:** hefesto-tf  
**Data:** 2026-11-15  
**Branch:** fix/linha-de-hook-do-guard-sai-127-sob-errexit-herdado  
**PR:** #687 (draft)  
**Escopo do diff:** `git diff origin/main...HEAD`

---

## Veredito

**BLOQUEADO.** Dois achados bloqueantes. Cinco notas não bloqueantes.

---

## Achados Bloqueantes

### BLOQUEIO-1 — Reconciliação falsa em três testes de migração D18 (A1-shape, Regra Dura de Reconciliação)

**Arquivos:** `internal/generators/guard_d18_migration_test.go`, linhas 107, 247, 292.

Os três testes declaram que o loop explícito de migração D18 é o caminho load-bearing. As medições de mutação mostram o oposto.

**Medição 1 — Claude (InjectClaudeHooks, linhas 376-383):**

Código do loop removido e substituído por `_ = legacyD18...` (3 declarações de supressão). Resultado:

```
go test ./internal/generators/ -run "TestD18Migration" -v -count=1
--- PASS: TestD18Migration_Claude_CredentialRead (0.00s)   ← devia falhar segundo reconciliação
--- PASS: TestD18Migration_Claude_GitBranch (0.00s)
--- PASS: TestD18Migration_Claude_WriteScope (0.00s)
...
ok  github.com/kgsaran/trackfw/internal/generators  1.388s
```

Restauração: `git diff --stat -- internal/generators/agentfiles.go` → vazio.

**Medição 2 — Codex (InjectCodexHooks, linhas 703-708):**

Loop Codex removido. Resultado:

```
go test ./internal/generators/ -run "TestD18Migration_Codex" -v -count=1
--- PASS: TestD18Migration_Codex_CredentialApplyPatch (0.00s)   ← devia falhar segundo reconciliação
ok  github.com/kgsaran/trackfw/internal/generators  1.260s
```

Restauração: `git diff --stat` → vazio.

**Medição 3 — Gemini (InjectGeminiHooks, linhas 861-866):**

Loop Gemini removido. Resultado:

```
go test ./internal/generators/ -run "TestD18Migration_Gemini" -v -count=1
--- PASS: TestD18Migration_Gemini_CredentialReadFile (0.00s)   ← devia falhar segundo reconciliação
ok  github.com/kgsaran/trackfw/internal/generators  1.185s
```

Restauração: `git diff --stat` → vazio.

**Causa:** O dedup do registry cobre todos os três casos:
- Claude: `agentfiles.go:403-406` — dedup cobre `{claudeShellMatcher, "Bash", "Read", "Write|Edit"}` com `CredentialLegacyPSPOSIX` (que inclui `LegacyD18CredentialPSPOSIX`).
- Codex: `agentfiles.go:714-716` — dedup cobre `{"Bash", "apply_patch"}` com `CredentialLegacyPSPOSIX`.
- Gemini: `agentfiles.go:871-873` — dedup cobre `{"run_shell_command", "read_file|read_many_files", "write_file|replace"}` com `CredentialLegacyPSPOSIX`.

As reconciliações declaradas são **falsas**: os testes passam mesmo quando o código que pretendem cobrir é removido. Isso é exatamente o padrão A1 da Regra Dura de Reconciliação (CLAUDE.md):

> "medimos X, e mesmo assim declaramos não-X"

**Microlote corretivo:**  
Para cada um dos três testes, reescrever a linha de reconciliação de:  
`"if the explicit D18 … loop … is removed, this test fails"`  
para:  
`"D18 legacy in [matcher] migrates to current via the dedup backstop ([linha] in agentfiles.go, CredentialLegacyPSPOSIX); the explicit loop is defense-in-depth, not load-bearing — removing it does not break this test"`  
E opcionalmente remover os loops explícitos (simplificação, em ML separado).

---

### BLOQUEIO-2 — Write-scope D18 legado produz violação enganosa, sem aviso de migração

**Arquivo:** `internal/validator/validator_write_scope_wiring.go:108`.

A função `settingsHasWriteScopeHook` reconhece **apenas** `guardforms.WriteScopeCurrentPSPOSIX`:

```go
// linha 108
if strings.TrimSpace(cmd) == guardforms.WriteScopeCurrentPSPOSIX {
    return true
}
```

Se um hook contém `guardforms.LegacyD18WriteScopePSPOSIX`, a função retorna `false`, e o validator emite a violação `write_scope_not_wired` com a mensagem:

> `write_scope_not_wired: .claude/settings.json PreToolUse[Write|Edit|NotebookEdit] missing trackfw guard write-scope (run trackfw init or trackfw update)`

Isso é **assimétrico** em relação a credential e git-branch:
- Credential D18 legado → warning de migração D18 (mensagem específica: "run `trackfw update`")
- Git-branch D18 legado → warning de migração D18 (mensagem específica: "run `trackfw update`")
- Write-scope D18 legado → violação `write_scope_not_wired` (mensagem enganosa: "guard not installed")

Além disso, não há nenhum teste no diff que verifique o comportamento do validator com write-scope D18 legado.

Sob a perspectiva de segurança: um hook write-scope com a forma D18 legada sai 127 sob `errexit` herdado (fail-open). A violação `write_scope_not_wired` pelo menos bloqueia o validate, mas a mensagem não orienta o usuário para a causa real (forma desatualizada, não ausência do guard).

**Microlote corretivo:**  
1. Atualizar `settingsHasWriteScopeHook` para reconhecer `LegacyD18WriteScopePSPOSIX` como forma válida (evitar falsa violação), OU adicionar aviso D18 específico para write-scope via `validateGuardHookD18LegacyWarnings` (alinhado com credential/git-branch).  
2. Adicionar teste ao `validator_guard_d18_legacy_test.go` cobrindo: write-scope D18 legado → comportamento esperado do validator.

---

## Achados por Nota (não bloqueantes)

### NOTA-1 — Loops explícitos D18 são inteiramente redundantes com o dedup do registry

Ver medições no BLOQUEIO-1. Os loops em Claude (`agentfiles.go:376-383`), Codex (`703-708`) e Gemini (`861-866`) são defense-in-depth. O código é executado mas sem efeito observável. Não é dead code perigoso — é código inerte.

**Recomendação:** remover em ML de limpeza separado, com comment explicando que o dedup é o caminho efetivo para cada caso.

---

### NOTA-2 — Quinta cópia literal da forma H em `guardExpectedLine` sem pino de teste

**Arquivo:** `internal/validator/validator_guard_binary_probe.go:94`.

```go
return `set +eu; set ErrorActionPreference Continue; null=/dev/null; Set-StrictMode -Off 2>$null; emulate -R sh 2>$null; $LASTEXITCODE=2 2>$null; trackfw guard ` + subcmdName + `; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>$null; exit $LASTEXITCODE`
```

Esta string é construída independentemente das constantes `guardforms.*CurrentPSPOSIX`. Os quatro constantes em `guardforms/guard_generations.go` são as outras quatro cópias.

**Ausência de teste de paridade:** Nenhum teste pina `guardExpectedLine("credential", guardShellFamilyPSPosix) == guardforms.CredentialCurrentPSPOSIX`. Se a forma H mudar em `guardforms`, o validator continuará reconhecendo a forma antiga e falhará em detectar a nova — sem erro de compilação, sem teste falhando.

**Recomendação:** Adicionar um teste de paridade em `validator_guard_binary_probe_test.go` (ou equivalente) que afirme `guardExpectedLine(subcmd, PSPosix) == guardforms.*CurrentPSPOSIX` para todos os quatro subcomandos. Ou refatorar `guardExpectedLine` para usar as constantes de guardforms via concatenação de partes (preamble + subcmd + sufixo), eliminando a cópia.

---

### NOTA-3 — Comentário órfão no braço de perfil de `TestGuardLineShellMatrix_PSInheritedState`

**Arquivo:** `internal/generators/guard_line_shell_test.go:2129-2131`.

```go
// Recriar marcador para verificação posterior por sub-teste.
if rmErr := os.Remove(markerPath); rmErr != nil {
    t.Fatalf(...)
}
```

Nenhum sub-teste verifica o marcador. O comentário promete uma verificação que não ocorre. Embora o perfil seja carregado (confirmado: `runGuardLinePSNoProfile` chama `execPSCommandOpts` com `noProfile=false`, que não insere `-NoProfile`), não há asserção por sub-teste de que o perfil foi efetivamente ativo durante a execução do guard.

**Verificação de PASS vs SKIP:** Executado com pwsh disponível (`/opt/homebrew/bin/pwsh`). Contagem de sub-testes PS: 130 PASS, 0 SKIP. Os braços de perfil executaram de verdade.

**Recomendação:** remover o comentário ou adicionar verificação de marcador em pelo menos um sub-teste (RC0-allow ou RC2-deny).

---

### NOTA-4 — Claim não verificável de cobertura Windows no braço de perfil

**Arquivo:** `internal/generators/guard_line_shell_test.go:2095-2099`.

```go
if runtime.GOOS == "windows" {
    // Perfil via XDG_CONFIG_HOME não funciona no Windows; exercitado no CI windows-latest
    // com HOME alternativo quando disponível.
    t.Log("perfil via XDG_CONFIG_HOME pulado no Windows")
    return
}
```

O texto "exercitado no CI windows-latest com HOME alternativo quando disponível" não aponta para nenhum teste específico. Grep em `internal/` não encontrou nenhum teste que implemente a variante Windows com HOME alternativo.

Diferente do padrão `barrier_ml_scope_test.go` (citado no handoff), este usa `t.Log` em vez de `t.Skip`, então não reporta falsa cobertura — mas a afirmação no comentário é aspiracional. Baixo impacto: a CI Windows cobre outros aspectos do guard.

---

### NOTA-5 — gofmt pré-existente em três arquivos de main

**Verificação de pré-existência (counter-arm primeiro):**

```
git show origin/main:internal/guardforms/guard_generations.go | gofmt -l /dev/stdin → (vazio — método funciona)
git show origin/main:internal/generators/update.go | gofmt -l /dev/stdin            → /dev/stdin
git show origin/main:internal/validator/validator_credential_guard.go | ...          → /dev/stdin
git show origin/main:internal/validator/validator.go | ...                            → /dev/stdin
```

As três violações (`Targets []string  //` com dois espaços em `update.go`; linha em branco de comentário em `validator_credential_guard.go`; formatação de bloco de documentação em `validator.go`) preexistem em main. O branch não introduziu novas violações.

**Verificação das linhas adicionadas:** `gofmt -d` em todos os arquivos tocados (incluindo `guard_generations.go`, `agentfiles.go`, `guard_binary_probe.go`, `guard_wiring.go`, `guard_git_branch_guard.go`, `guard_d18_migration_test.go`, `validator_guard_d18_legacy_test.go`) retornou vazio — as linhas adicionadas estão formatadas corretamente.

---

### NOTA-6 — Leve imprecisão de contagem de linhas PS entre README.md e cli-parity.md

**README.md:** "emits **three** `CommandNotFound` error blocks to stderr (one for `null=/dev/null`, one for `emulate`, one for the `$LASTEXITCODE=2*...` segment)"

**cli-parity.md:** "`null=/dev/null` → +2 linhas no stderr PS por execução"; "`emulate -R sh 2>$null` → uma linha no stderr por execução"

A afirmação de "três blocos" no README é consistente com cli-parity.md se "bloco" = "error record" (1 bloco = ≥1 linhas). Mas o leitor que soma "+2 linhas + 1 linha + ?" não chega a "três blocos" sem a chave "1 bloco = N linhas". Um terceiro bloco não é descrito em cli-parity.md.

Medição com pwsh disponível (`/opt/homebrew/bin/pwsh`): os testes PS passaram 130/130 sem SKIPs, confirmando a forma H funciona. Verificação direta do stderr PS com stub não foi realizada nesta sessão; o claim de "três blocos" vem do commit c14712f4 (medição do ML-1D na VM Windows 11 ARM64).

**Recomendação:** Em cli-parity.md, descrever explicitamente que `$LASTEXITCODE=2*!!$LASTEXITCODE 2>$null` produz um bloco de CommandNotFound PS (similar ao `emulate`), completando a contagem de três.

---

### NOTA-7 — Quatro constantes `*CurrentPSPOSIX` com preamble verbatim

`guardforms.CredentialCurrentPSPOSIX`, `GitBranchCurrentPSPOSIX`, `WriteScopeCurrentPSPOSIX` e `CredentialGlobalCurrentPSPOSIX` têm os mesmos ~115 chars de preamble antes do subcomando. Os três comentários curtos `// D18 ML-1C2 (REQ-2026-11-13): four-segment preamble (see CredentialCurrentPSPOSIX)` mitigam mas não eliminam o risco de divergência.

Consolidação via `const preamble = "set +eu; ..."` e `const suffix = "; LASTEXITCODE=..."` manteria valores compile-time, compatível com D12 (igualdade exata na verificação, não na geração). Recomendado para ML de limpeza.

---

## Respostas às Perguntas do Escopo

### Q1 — Duplicação entre as quatro constantes `*CurrentPSPOSIX`

Duplicação existe (ver NOTA-7). Não bloqueia — há cinco cópias no total (a quinta em `guardExpectedLine`, ver NOTA-2). O risco de drift silencioso é real para a quinta cópia, que não tem teste de paridade.

### Q2 — Testes afirmam comportamento ou apenas igualdade de string?

Os testes afirmam comportamento (RC + stdin.out + stderr), não apenas igualdade de string. Os `t.Fatalf("constante não começa com ml1dPrefix")` são guards defensivos, não os asserts primários.

Exceção: as reconciliações declaradas em três testes de migração são **falsas** (ver BLOQUEIO-1) — os testes passam mesmo sem o código que pretendem verificar.

### Q3 — t.Skip sem motivo real ou cobertura falsa

Nenhum `t.Skip` com justificativa falsa do tipo "tested separately". Todos os novos skips são baseados em ausência de ambiente (shell/pwsh não encontrado, timeout).

Achado relacionado: `t.Log("perfil via XDG_CONFIG_HOME pulado no Windows")` com claim não verificável de cobertura CI (ver NOTA-4). Usa `t.Log`, não `t.Skip`, portanto não reporta falsa cobertura.

### Q4 — Código morto, comentários órfãos, formas nunca released

**CAND_F:** Apenas como constante local `candFCredentialPSPOSIX` no contra-braço de `TestGuardLineShellMatrix_PreambleNoStderr`. Bem documentada como nunca released. Não em código de produto.

**ML-1C em produto:** Referências a `ML-1C` em arquivos não-test pertencem a REQs diferentes (REQ-2026-10-24, REQ-2026-09-05). Nenhuma referência a `CAND_F` ou `ML-1C` de REQ-2026-11-13 fora de testes.

**Comentário órfão:** `guard_line_shell_test.go:2129` (ver NOTA-3).

**Loops redundantes:** `agentfiles.go:376-383, 703-708, 861-866` — código executado mas inerte. Verificado por mutação.

### Q5 — go vet e gofmt

```
go vet ./internal/...   → (vazio — clean)
```

Gofmt: violações pré-existentes em main (`update.go`, `validator_credential_guard.go`, `validator.go`). Método validado pelo counter-arm (guard_generations.go retornou vazio). Linhas adicionadas pelo branch estão formatadas corretamente.

---

## Cobertura dos Testes Novos — Reconciliação (nomes reais verificados)

```
grep -n '^func Test' internal/generators/guard_d18_migration_test.go \
    internal/validator/validator_guard_d18_legacy_test.go
```

| Nome do teste (confirmado por grep) | Conclusão que afirma | Status |
|---|---|---|
| `TestD18Migration_Claude_CredentialRead` (linha 107) | D18 legado no Read → substituído pela atual | Reconciliação FALSA: o dedup cobre, não o loop explícito |
| `TestD18Migration_Claude_GitBranch` (linha 156) | D18 legado no Bash|PowerShell → substituído | Reconciliação parcialmente falsa (dedup + loop) |
| `TestD18Migration_Claude_WriteScope` (linha 201) | D18 legado no Write|Edit|NotebookEdit → substituído | Reconciliação parcialmente falsa (dedup + loop) |
| `TestD18Migration_Codex_CredentialApplyPatch` (linha 247) | D18 legado no apply_patch → substituído | Reconciliação FALSA: o dedup cobre, não o loop explícito |
| `TestD18Migration_Gemini_CredentialReadFile` (linha 292) | D18 legado no read_file → substituído | Reconciliação FALSA: o dedup cobre, não o loop explícito |
| `TestD18Migration_Cursor_CredentialBeforeShell` (linha 340) | D18 legado no Cursor beforeShellExecution → substituído via registry | Correta |
| `TestD18Migration_Copilot_ProjectScopeAlwaysCurrentForm` (linha 386) | Copilot recria com forma atual | Correta |
| `TestD18Migration_Claude_Idempotence` (linha 426) | Segunda execução não altera bytes | Correta |
| `TestD18Migration_D11RevToCurrentForm` (linha 489) | D11Rev migra para a atual, não para D18 legada | Correta |
| `TestD18Migration_MixtureCollapseToOne` (linha 544) | D18 legada + atual colapsa para uma entrada | Correta |
| `TestD18Migration_GlobalScope` (linha 597) | UpdateHarness migra forma global D18 | Correta |
| `TestD18LegacyWarning_D18LegacyFormEmitsWarning` (validator, linha 60) | D18 legada → warning D18 | Correta |
| `TestD18LegacyWarning_CurrentD18FormSilent` (validator, linha 91) | Forma atual → sem warning D18 | Correta |
| `TestD18LegacyWarning_D11RevFormEmitsD11RevNotD18` (validator, linha 119) | D11Rev → warning D11Rev, não D18 | Correta |
| `TestD18LegacyWarning_D18LegacyNotAViolation` (validator, linha 157) | D18 legada → não é violation | Correta |

---

## Comandos Executados

```
go vet ./internal/...                                     → vazio (clean)
gofmt -l <arquivos do diff>                               → update.go, validator_credential_guard.go, validator.go (pré-existentes)
gofmt -d internal/generators/update.go                   → Targets []string  // (pré-existente)
git show origin/main:internal/guardforms/guard_generations.go | gofmt -l /dev/stdin → vazio (counter-arm confirma método)
git show origin/main:internal/generators/update.go | gofmt -l /dev/stdin             → /dev/stdin (pré-existente)
gofmt -d internal/generators/guard_d18_migration_test.go  → vazio (clean)
gofmt -d internal/guardforms/guard_generations.go         → vazio (clean)
gofmt -d internal/validator/validator_guard_binary_probe.go → vazio (clean)
go test ./internal/generators/ -run "TestD18Migration" -v -count=1    → 11 PASS, 0 SKIP
go test ./internal/generators/ -run "TestGuardLineShellMatrix_PSInheritedState|TestGuardLineShellMatrix_D18NovaContraArm_Stop" -v -count=1 -timeout 300s → 130 PASS, 0 SKIP
go test ./internal/generators/ -run "TestGuardLineShellMatrix_PreambleNoStderr" -v -count=1 -timeout 300s → ok
go test ./internal/validator/ -run "TestD18Legacy" -v -count=1 → 4 PASS, 0 SKIP
go test ./internal/generators/ -run "TestGuardLineShellMatrix$" -count=1 → ok
go build ./internal/...                                    → vazio (clean)
which pwsh                                                 → /opt/homebrew/bin/pwsh
Mutação 1 (Claude loop): go test -run TestD18Migration → ok (1.388s); git diff --stat → vazio
Mutação 2 (Codex loop): go test -run TestD18Migration_Codex → ok (1.260s); git diff --stat → vazio
Mutação 3 (Gemini loop): go test -run TestD18Migration_Gemini → ok (1.185s); git diff --stat → vazio
```

---

## git status --short

```
?? docs/qualidade/2026-11-15-revisao-linha-de-hook-forma-h.md
```
