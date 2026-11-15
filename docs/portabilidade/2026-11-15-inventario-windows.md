# Inventário de Windows — trackfw

> Produzido por: artemis-tf, ML-0B  
> Fonte de CI: run 43766925360, job 132974038971, head 6b28e547 (inclui PR #687/D18), branch `main`  
> Contagem confirmada: 137 linhas `--- SKIP`, **71 nomes únicos** de topo, 5 `--- FAIL`, 5052 `--- PASS`  
> Corretivo ML-0B2 (2026-11-15): total corrigido de 74→71 (medido); MLs propostos atualizados; arm64 corrigido; seção de reconciliação com threat model adicionada.  
> Comando de verificação: `gh run view 43766925360 --job 132974038971 --log`

---

## 1. Known-failures (5 entradas em `.github/windows-known-failures.json`)

| Nome do teste | Arquivo:linha | Razão (do JSON, medida) | Classificação |
|---|---|---|---|
| TestChecksum_StableAndMatchesSHA256Sum | `internal/thirdparty/markers_test.go:225` | `shasum -a 256` em Windows retorna hash com prefixo de barra invertida; `Checksum()` retorna hex sem prefixo | **(c)** `shasum` não é ferramenta padrão do Windows; o formato de saída diverge por design |
| TestFilenameUniqueness_DiretorioNaoLegivel_P2 | `internal/validator/validator_test.go:1661` | `os.Chmod(0o000)` ignorado silenciosamente no NTFS sem ACL; diretório continua legível | **(c)** NTFS não aplica bits POSIX de permissão |
| TestFolderStatus_DiretorioNaoLegivel_P2 | `internal/validator/validator_test.go:1641` | mesma causa do anterior | **(c)** |
| TestShip_Integration_GracefulDegradation_RealBinary | `internal/commands/ship_test.go:1335` | `git.exe` falso no PATH aciona defesa anti-fork-bomb do runner Windows | **(c)** comportamento do runner Windows impede o padrão impostor de binário |
| TestStaleWIPReportsWIPWalkError | `internal/validator/validator_stale_wip_contract_xfail_test.go:113` | `ENOTDIR` = `ERROR_PATH_NOT_FOUND` no Windows; walk não produz o diagnóstico esperado quando arquivo está onde se espera diretório | **(c)** indistinguibilidade de `ENOTDIR`/`ENOENT` no Windows (medido em D16) |

Todos os 5 estão no JSON com `"class": "assertion"` e marcados `(MEASURED)`.

---

## 2. Skip inventory (71 nomes únicos, 137 linhas SKIP)

### Chave de classificação

- **(a)** asserção equivalente no Windows é possível — apenas precisa ser escrita
- **(b)** defeito a corrigir — o skip guarda um teste que PODE e DEVE rodar no Windows (justificativa falsa ou skip prematuro)
- **(c)** impossível na plataforma — o comportamento específico não pode ocorrer no Windows
- **[FALSO]** = a justificativa textual do `t.Skip` faz uma afirmação demonstravelmente errada

### 2.1 Grupo ML-scope — barrier_ml_scope_test.go (41 testes)

Todos os 41 testes (39 `TestMLScope_*` + 2 `TestMLScopeGlob*`) têm o mesmo guarda de skip:

```go
if runtime.GOOS == "windows" {
    t.Skip("ml_scope uses git commands; windows path handling tested separately")
}
```

Localização representativa: `internal/commands/barrier_ml_scope_test.go:269` (repetido em cada função de teste).

**Classificação: (b) [FALSO]**  
Prova: `ml_scope` usa `exec.Command("git")` diretamente, não via `sh`. O `git` está em PATH no `windows-latest` (Git for Windows). Não existe nenhum "Windows path handling tested separately" — confirmado inspecionando o repositório inteiro. Skip introduzido no PR #606 (2026-10-24).

| Teste | Arquivo:linha do skip |
|---|---|
| TestMLScope_AdrOutsideADRDirsDiscarded | barrier_ml_scope_test.go:269 |
| TestMLScope_BlockedStatusAboveN_InScope | barrier_ml_scope_test.go:319 |
| TestMLScope_BroadPatternInStartedWaveAboveN_Blocked | barrier_ml_scope_test.go:371 |
| TestMLScope_DeclaredFilePasses | barrier_ml_scope_test.go:420 |
| TestMLScope_DirectoryAndGlobMatch | barrier_ml_scope_test.go:479 |
| TestMLScope_DiskEditDoesNotAmplifyScope | barrier_ml_scope_test.go:530 |
| TestMLScope_DiskEditOfWaveStatusDoesNotAmplifyScope | barrier_ml_scope_test.go:587 |
| TestMLScope_FractionalWave_LowerWaveHasDeclFailure | barrier_ml_scope_test.go:631 |
| TestMLScope_FractionalWave_StartedInScope | barrier_ml_scope_test.go:833 |
| TestMLScope_FractionalWave_UnstartedNoDeclFailure | barrier_ml_scope_test.go:918 |
| TestMLScope_FractionalWave_UnstartedOutOfScope | barrier_ml_scope_test.go:1034 |
| TestMLScope_HTMLCommentedStatus_MLSCompleteBlocked | barrier_ml_scope_test.go:2283 |
| TestMLScope_HTMLCommentedStatus_WaveNotStarted | barrier_ml_scope_test.go:2394 |
| TestMLScope_NoOriginMain_Blocked | barrier_ml_scope_test.go:2514 |
| TestMLScope_OrchestrationSetPasses | barrier_ml_scope_test.go:2577 |
| TestMLScope_OutOfScopeFails | barrier_ml_scope_test.go:2648 |
| TestMLScope_PreCutoffSkipped | barrier_ml_scope_test.go:2711 |
| TestMLScope_ReqOutsideReqDirDiscarded | barrier_ml_scope_test.go:2794 |
| TestMLScope_RoadmapMovedStatePasses_ByAgent | barrier_ml_scope_test.go:2927 |
| TestMLScope_RoadmapMovedStatePasses_Flat | barrier_ml_scope_test.go:3038 |
| TestMLScope_StartedWaveAboveN_NoDeclFailureForMissingDeclaration | barrier_ml_scope_test.go:3165 |
| TestMLScope_StartedWaveAboveN_PassesWhenFilesCommitted | barrier_ml_scope_test.go:3240 |
| TestMLScope_StartedWaveAboveN_UndeclaredFileStillFails | barrier_ml_scope_test.go:3417 |
| TestMLScope_TargetRoadmapExcludedFromMemberLoop | barrier_ml_scope_test.go:3472 |
| TestMLScope_TerminatedStatusAboveN_InScope | barrier_ml_scope_test.go:3517 |
| TestMLScope_UnionAbandonedContributesOrchestrationOnly | barrier_ml_scope_test.go:3562 |
| TestMLScope_UnionDoesNotIncludeUncommittedRoadmap | barrier_ml_scope_test.go:3688 |
| TestMLScope_UnionDoneAtMergeBaseOrchestrationOnly | barrier_ml_scope_test.go:3736 |
| TestMLScope_UnionDoneMovedInBranchContributesPatterns | barrier_ml_scope_test.go:3794 |
| TestMLScope_UnionIgnoresWorkingTreeRoadmapEdit | barrier_ml_scope_test.go:3849 |
| TestMLScope_UnionMemberUnstartedWaveIsExcluded | barrier_ml_scope_test.go:2156 |
| TestMLScope_UnionMissingDateTreatedAsPostCutoff | barrier_ml_scope_test.go:2220 |
| TestMLScope_UnionPreCutoffMemberOrchestrationOnly | barrier_ml_scope_test.go:2092 |
| TestMLScope_UnionStartedWavesOnly_AllPendingContributesNothing | barrier_ml_scope_test.go:2022 |
| TestMLScope_UnionStartedWavesOnly_DoneWaveContributesPatterns | barrier_ml_scope_test.go:1935 |
| TestMLScope_UnionTwoRoadmapsFilesFromOtherRoadmapInScope | barrier_ml_scope_test.go:1873 |
| TestMLScope_UnstartedWaveAmidStartedWavesAboveN_Fails | barrier_ml_scope_test.go:1777 |
| TestMLScope_UntrackedOutOfScopeFails | barrier_ml_scope_test.go:1695 |
| TestMLScope_WipMissingDeclarationFails | barrier_ml_scope_test.go:1632 |
| TestMLScopeGlobBroadPatternBlocked | barrier_ml_scope_test.go:1508 |
| TestMLScopeGlobPrefixedPatternAccepted | barrier_ml_scope_test.go:1573 |

### 2.2 Grupo Reentry — barrier_reentry_test.go (5 testes)

| Teste | Arquivo:linha | Justificativa textual do t.Skip | Classificação |
|---|---|---|---|
| TestBarrierReentry_T1_DirectReentrance | barrier_reentry_test.go:187 | "shell gate com fusível requer sh POSIX; Windows coberto por windows-known-failures.json" | **(b) [FALSO]** — `sh` está em PATH no windows-latest (prova: §9); zero entradas de reentry no JSON |
| TestBarrierReentry_T2_IndirectReentrance | barrier_reentry_test.go:231 | mesma | **(b) [FALSO]** |
| TestBarrierReentry_T3_LegitimateNested | barrier_reentry_test.go:280 | "shell gate requer sh POSIX; Windows coberto por **windows-known-files.json**" | **(b) [FALSO]** — mesmo problema + typo no nome do arquivo ("files" em vez de "failures") |
| TestBarrierReentry_T7_Canonicalization | barrier_reentry_test.go:439 | "os.Symlink pode falhar no Windows sem permissão de admin" | **(b)** antipadrão — skip incondicional por GOOS; o padrão correto é `try os.Symlink; t.Skipf on err` (como em configure_guard_test.go:24) |
| TestBarrierReentry_T7e_APFSCaseInsensitive | barrier_reentry_test.go:710 | `t.Skip("T7e: EvalSymlinks normalizou case — skip (bypass não ocorreria)")` — skip dinâmico via probe do filesystem | **(c)** legítimo — `EvalSymlinks` normaliza case no NTFS; o comportamento APFS (preservação de case em FS case-insensitive) não é reproduzível no Windows |

### 2.3 Grupo shells POSIX — guard_line_shell_test.go (3 entradas no log de CI)

Justificativa recorrente: "shells POSIX não disponíveis em Windows" (e variante: "... medida separadamente na VM").

**Classificação: (b) [FALSO PARCIAL]** — afirmação falsa para `sh` (e provavelmente `bash`) que chegam via Git for Windows no `windows-latest`. Para `zsh` e `dash`, provavelmente verdade. A cláusula "medida separadamente na VM" descreve medição futura que ainda não foi realizada.

| Teste | Arquivo:linha do skip | Escopo do subconjunto afetado | No log de CI? |
|---|---|---|---|
| TestGuardLineShellMatrix | guard_line_shell_test.go:805 (nível de subtest) | PASS no topo (30.60s no D18); subtests POSIX (sh, bash, zsh, dash e variantes) SKIP; subtest pwsh PASSA — confirmado no CI D18 | **sim** (contado nos 71) |
| TestGuardLineShellMatrix_DefeitoAntigo | guard_line_shell_test.go:986 | skip ao entrar na função | **sim** (contado nos 71) |
| TestGuardLineShellMatrix_UnsetoptInsuficiente | guard_line_shell_test.go:1094 | skip ao entrar na função | **sim** (contado nos 71) |
| TestGuardLineShellMatrix_D18ContraArm | guard_line_shell_test.go:1231 | skip ao entrar na função | não — não gerou linha `--- SKIP` de topo no log de CI (não contado nos 71) |
| TestGuardLineShellMatrix_PreambleNoStderr | guard_line_shell_test.go:2671 | skip ao entrar na função | não — não gerou linha `--- SKIP` de topo no log de CI (não contado nos 71) |

As duas últimas entradas têm guarda de skip no código-fonte e são documentadas aqui como contexto, mas não aparecem no log de CI do run 43766925360 — não são contadas no total de 71.

**Lacuna adicional (não-skip, gap de cobertura):** `TestGuardLineShellMatrix_LContraArm_ConfirmLow` PASSA no CI D18 — apenas o braço via-command. O braço via-profile usa `t.Log(...); return` (linhas 2316 e 2976), não `t.Skip` — portanto não aparece no log de skips. O cenário de drain de stdin por `$ConfirmPreference` via perfil **não foi medido no Windows** (nem PS 5.1 nem pwsh 7 no Windows).

### 2.4 Grupo chmod POSIX — validator tests (4 testes)

Justificativa: "bits de permissão POSIX não se aplicam no Windows" — **VERDADEIRO**.  
O `os.Chmod(0o000)` não bloqueia leitura no NTFS sem ACL.

**Classificação: (c)**

| Teste | Arquivo:linha |
|---|---|
| TestCredentialGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar | validator/validator_credential_guard_integrity_test.go:25 |
| TestGitBranchGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar | validator/validator_git_branch_guard_test.go:234 |
| TestGuardGlobalScriptIntegrity_ScriptIlegivel_ViolationSemSilencio | validator/validator_git_branch_guard_test.go:694 |
| TestGuardHookResolvable_ProjectPermissionDenied_DisparaViolationSemCrash | validator/validator_git_branch_guard_test.go:1162 |

Asserção equivalente no Windows requer ACL (`icacls` / `SetFileSecurity`). Se implementada, seria **(a)** — reclassificar quando implementado.

### 2.5 Grupo chmod POSIX — json_exit_test.go (2 testes)

**Classificação: (c)**

| Teste | Arquivo:linha do skip indireto | Justificativa |
|---|---|---|
| TestJSONExit_UpdateJSON_TargetFailedSilentStderr | json_exit_test.go:79 (via setupUpdateFailFixtureDir) | "chmod 444 não bloqueia escrita no Windows" |
| TestJSONExit_UpdateHarnessJSON_TargetFailedSilentStderr | json_exit_test.go:122 (via setupHarnessFailFixtureDir) | "chmod 555 não bloqueia criação de subdiretório no Windows" |

### 2.6 Grupo restrições NTFS (8 testes)

NTFS não permite `:`, `*`, `\n`, `"` em nomes de arquivo/diretório. Os vetores de ataque dependem dessas construções.

**Classificação: (c)**

| Teste | Arquivo:linha | Justificativa |
|---|---|---|
| TestCredentialGuardScript_GeneratedScript_WindowsPathRedirectDetectsJWTInFile | generators/credential_guard_test.go:755 | "':' is not valid in a filename on Windows — fixture cannot be created" |
| TestCredGlobEscapeLiteral_UnitVectors | guard/credential_test.go:2176 | subtest "backslash then bracket" — `skip: "windows"` — "filesystem does not allow this path component" |
| TestCredResolveArg_GitBashPath_POSIXUnchanged | guard/credential_test.go:1473 | "POSIX test — skipped on Windows" — semântica de `/c/Users/...` difere por plataforma; Windows-only tests existem (linhas 1502–1711) |
| TestRunCredential_D1_ControlCharInFilename | guard/credential_test.go:1969 | "filename with newline is not supported on Windows" |
| TestRunCredential_A1_CwdStarSiblingNoFalsePositive | guard/credential_test.go:2244 | "NTFS does not allow '*' in directory names" |
| TestMatchSubcommand_BackslashPrefixed_PosixNotGit | guard/gitbranch_test.go:956 | "POSIX-only: on Windows filepath.Base('\\\\git') returns 'git' — divergence is intentional"; Windows-only test existe em gitbranch_test.go:1162 |
| TestReqDoneOpenCriteria_DoubleQuoteBranchName | validator/validator_req_done_criteria_test.go:528 | "NTFS não permite `\"` em nomes de arquivo; ataque não é montável nessa plataforma" |
| TestMdBasenamesInGitTree_LiteralPathspecDirName | validator/validator_literal_pathspecs_test.go:19 | "NTFS reserva ':' em nome de arquivo — o diretório ':(exclude)rm' não pode existir, então o vetor RN1 não existe nesta plataforma" |

### 2.7 Grupo D11/D18 — fail_closed e guard_line (2 testes no log de CI)

| Teste | Arquivo:linha | Justificativa textual | Classificação | No log de CI? |
|---|---|---|---|---|
| TestD11FailClosed_POSIX_TabelaRC | generators/fail_closed_d11_behavior_test.go:92 | "linha D11 PS/POSIX não se aplica diretamente ao cmd.exe — skip em Windows" | **(a)** — a linha D11 é um poliglota PS/POSIX; uma asserção Windows via `pwsh` é possível; o skip confunde cmd.exe com PowerShell | **sim** (contado nos 71) |
| TestD11FailClosed_PowerShell_TabelaRC | generators/fail_closed_d11_behavior_test.go:133 | "teste usa stub #!/bin/sh — não aplicável em cmd.exe/PS no Windows desta forma" | **(b)** — `sh` está disponível no windows-latest; o stub `#!/bin/sh` PODE executar via Git Bash | **sim** (contado nos 71) |
| TestD18ContraArm_ShAsDashSim | generators/guard_line_shell_test.go:3110 | "shells POSIX não disponíveis em Windows" | **(b) [FALSO]** para o braço sh; `dash` pode não estar disponível (medição pendente) | não — não gerou linha `--- SKIP` de topo no log de CI (não contado nos 71) |

### 2.8 Grupo netexec/process (2 testes)

| Teste | Arquivo:linha | Justificativa textual | Classificação |
|---|---|---|---|
| TestGitExec_AC2_Bite_NoWaitDelay | commands/netexec_test.go:483 | "bite test skipped on Windows: pipe hang behaviour not confirmed there" | **(b)** — "not confirmed" indica falta de medição, não impossibilidade |
| TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild | commands/netexec_test.go:697 | "pre-push hook test skipped on Windows: shell hooks not supported" | **(b) [FALSO]** — hooks de shell rodam no Windows com `sh` via Git Bash |

### 2.9 Grupo git_branch_guard_awk (1 teste)

| Teste | Arquivo:linha | Justificativa textual | Classificação |
|---|---|---|---|
| TestGitBranchGuardAwk_ProvaDeMordida | generators/git_branch_guard_test.go:1161 | `t.Skipf("git show 3b2eff09: não disponível (%v) — pulando prova de mordida", err)` — skip condicional | **(a)** — se o objeto git `3b2eff09` está disponível (clone completo), o teste deve rodar no Windows; o problema é de workflow (clone raso), não de plataforma |

### 2.10 Grupo doctor/scan (2 testes)

| Teste | Arquivo:linha | Justificativa textual | Classificação |
|---|---|---|---|
| TestScanStrayAgentMemory_PathsWithSpaceAndQuote | commands/doctor_test.go:530 | "sh -c not available on Windows" | **(b) [FALSO]** — `sh` está em PATH no windows-latest (§9) |
| TestScanStrayAgentMemory_UnreadableSubdir | commands/doctor_test.go:613 | "chmod 000 not effective on Windows" | **(c)** — NTFS não aplica bits POSIX |

### 2.11 Grupo plataforma-agnóstica (1 teste)

| Teste | Arquivo:linha | Justificativa textual | Classificação |
|---|---|---|---|
| TestPush_ForceWithLease_NoPROpen_Blocks | commands/push_test.go:261 | "force-with-lease+no-PR path has no end-to-end coverage; see docs/cli-parity.md partial= annotation" | NÃO é skip específico de Windows — skipa em todos os SOs. **(a)** — uma asserção é possível quando a lacuna de cobertura for fechada (ver docs/cli-parity.md) |

---

## 3. Contagem de classificações

Contagens geradas a partir da lista de 71 nomes em `skips71.txt` via script Python (ML-0B2):

```
grep -a -- '--- SKIP' wfs.log | sed -E 's/.*--- SKIP: ([^ /]+).*/\1/' | sort -u | wc -l
# → 71

grep -c 't\.Skip("ml_scope' internal/commands/barrier_ml_scope_test.go
# → 41
```

Cada nome foi atribuído a uma classificação derivada de §2.1–§2.11; a soma resultante confirma o total:

| Classificação | Testes | Composição |
|---|---|---|
| **(b) [FALSO]** | 49 | 41 ML-scope + T1/T2/T3 (3) + TestGitExec_WaitDelay + TestScanStrayAgentMemory_PathsWithSpaceAndQuote + 3 guard_line_shell_* (confirmadas no log) |
| **(b)** antipadrão ou medição pendente | 3 | T7 Canonicalization + TestGitExec_AC2 + TestD11FailClosed_PowerShell |
| **(a)** asserção Windows possível | 3 | TestD11FailClosed_POSIX + TestGitBranchGuardAwk + TestPush_ForceWithLease |
| **(c)** impossível na plataforma | 16 | T7e (1) + 8 NTFS + 4 chmod-validator + 2 json_exit + ScanStray_Unreadable (1) |
| **Total** | **71** | |

Nota: `TestGuardLineShellMatrix_D18ContraArm`, `TestGuardLineShellMatrix_PreambleNoStderr` e `TestD18ContraArm_ShAsDashSim` têm guardas de skip no código-fonte mas não geraram linha `--- SKIP` de topo no log do run 43766925360. São documentados em §2.3/§2.7 como contexto e excluídos do total de 71.

---

## 4. Artefatos do consumidor gerados pelo `trackfw init`

As chamadas reais de `WriteFile` em `internal/generators/scaffold.go` estão nas linhas 1163, 1226, 1245, 1292, 1421 e 1856. Os caminhos definidos como `const` (linhas 97–110) são referências — não são as chamadas de escrita.

| Artefato gerado | Requer shell para executar? | `sh` disponível sem Git Bash no Windows? | Evidência CI |
|---|---|---|---|
| `scripts/trackfw-validate.sh` | sim — shebang `#!/bin/sh` | Não — `sh` só existe via Git Bash ou WSL | Nenhuma evidência de execução consumer-side |
| `scripts/trackfw-attention-signal.sh` | sim | id. | Nenhuma — atenção funciona via `trackfw attention signal` (Go) |
| `scripts/trackfw-attention-cleanup.sh` | sim | id. | Nenhuma |
| `scripts/trackfw-credential-guard.sh` | sim | id. | Nenhuma — guard funciona via `trackfw guard credential` (Go) |
| `scripts/trackfw-git-branch-guard.sh` | sim | id. | Nenhuma — guard funciona via `trackfw guard git-branch` (Go) |
| `.lefthook/commit-msg/trackfw-req-check.sh` | sim (via lefthook) | id. | Nenhuma |
| Arquivos de config de agente (`.claude/CLAUDE.md`, `.cursor/rules`, etc.) | não — texto puro | n/a | n/a |
| Linha de hook nos arquivos de agente (Forma L / CmdExe) | sim — executada pelo shell do CLI | depende do CLI (ver §5) | ver §5 |

**Achado — gap AC4:** os 6 arquivos `.sh` gerados requerem `sh` para execução. No Windows, isso implica Git Bash ou WSL. O `trackfw validate` / `trackfw doctor` não declara explicitamente que esses scripts não executam sem Git Bash.

---

## 5. Hooks por CLI de agente

Forma L gerada por `internal/generators/agentfiles.go`:
```
set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0;
set ConfirmPreference None -Force -WhatIf:0 -Confirm:0;
set +eu; null=/dev/null; Set-StrictMode -Off 2>$null; emulate -R sh 2>$null;
$LASTEXITCODE=2 2>$null; trackfw guard <cmd>; ...
```
Kiro usa forma CmdExe (`CredentialCurrentCmdExe = "trackfw guard credential || exit 2"`).

| CLI | Forma gerada | Executa no Windows sem Git Bash? | PS passa -NoProfile/-NonInteractive? | Evidência |
|---|---|---|---|---|
| **Claude Code** | Forma L | **Sim** — `trackfw` é binário Go nativo | **-NoProfile -NonInteractive** confirmados por inspeção binária (offset 227832188 no Windows ARM64) | D18 doc `docs/portabilidade/2026-11-13-guard-errexit-windows.md` §4 |
| **Kiro** | CmdExe (`\|\| exit 2`) | **Sim** — Node.js spawna `%ComSpec%` (cmd.exe); sem envolvimento de PS | n/a (cmd.exe) | ADR-2026-09-05 Adendo; `agentfiles.go` constante `CredentialCurrentCmdExe` |
| **Cursor** | Forma L | **Sim** (binário Go nativo); forma L lida com EAP=Stop | **Não medido** na VM | ADR-2026-09-05 Adendo declara Cursor nativo no Windows |
| **Codex** | Forma L | **Sim** (binário Go) | **Não medido** | Não instalado na VM de D18 |
| **Gemini** | Forma L | **Sim** | **Não medido** | Não instalado na VM de D18 |
| **Copilot** | Forma L | **Sim** | **Não medido** | Não instalado na VM de D18 |
| **Windsurf** | Forma L | **Sim** | **Não medido** | Não instalado na VM de D18 |
| **Amazon Q** | Forma L | **Sim** | **Não medido** | Não instalado na VM de D18 |

**Lacuna AC6 (D18 residual):** todos os CLIs exceto Claude Code não têm evidência de que passam (ou não) `-NonInteractive` ao PS. Sem `-NonInteractive`, um perfil com `$ConfirmPreference = Low` drena o stdin do hook (Form K tinha esse defeito; Form L adiciona `set ConfirmPreference None`). O braço via-perfil do `TestGuardLineShellMatrix_LContraArm_ConfirmLow` não mede o PS 5.1 nem o pwsh 7 do Windows (retorna cedo com `t.Log`, não `t.Skip`, em guard_line_shell_test.go:2316 e 2976).

---

## 6. Canais de distribuição

| Canal | Smoke no windows-latest? | O que é testado | O que falta |
|---|---|---|---|
| **GitHub release / install.ps1** | **Sim** — job `windows-install-ps1` | Instala com PS 5.1 e pwsh 7; `scripts/check-install-ps1.ps1` inclui happy-path que instala binário localmente compilado (via `setup-go`) e roda `trackfw --version`; testa também erros de checksum e permissão | windows/amd64 é exercido. windows/arm64: compilado no release workflow mas sem runner Windows ARM64 disponível |
| **npm** (`npm install -g trackfw`) | **Não** — job `package-smoke` roda em ubuntu-latest | Instala e verifica binário no Linux | Smoke no windows-latest: `npm install -g` + `trackfw --version` + `trackfw validate` |
| **PyPI** (`pip install trackfw`) | **Não** — mesmo job, ubuntu-latest | id. | Smoke no windows-latest: `pip install` + `trackfw --version` + `trackfw validate` |
| **Homebrew** | N/A — fórmula é darwin-only | n/a | n/a |
| **arm64 Windows** | **Não** — sem job no quality.yml | windows/arm64 compilado pelo GoReleaser mas nunca executado em CI | Runner `windows-11-arm` **GA para repositórios públicos** desde 2025-08-07 (https://github.blog/changelog/2025-08-07-arm64-hosted-runners-for-public-repositories-are-now-generally-available/); repo `kgsaran/trackfw` é público (`gh repo view kgsaran/trackfw --json visibility` → `PUBLIC`); adicionar job `windows-arm64` ao `quality.yml` (ver REQ-IMP-7 e ML-arm64 proposto) |

---

## 7. Resumo de lacunas por causa raiz

| Causa | Instâncias | Classificação dominante |
|---|---|---|
| **C1** Skip falso "ml_scope git separado" | 41 TestMLScope_* | (b) false |
| **C2** Skip falso "sh POSIX / windows-known-failures" (reentry) | T1, T2, T3 | (b) false |
| **C3** Skip falso/parcial "POSIX shells não disponíveis" | T7 reentry antipadrão + 3 guard_line_shell (no log) + D11PS + netexec + ScanStray_sh | (b) — sh está disponível |
| **C4** NTFS não aplica chmod 000/444/555 | 2 known-failures + 4 validator + 2 json_exit + 1 doctor | (c) |
| **C5** NTFS não permite `:*"\n` em nomes de arquivo | 8 testes de vetores de ataque | (c) |
| **C6** ConfirmPreference via-perfil não medido no Windows | braço via-profile de LContraArm_ConfirmLow | lacuna de cobertura (não skip) |
| **C7** npm/PyPI sem smoke Windows | — | falta de workflow |
| **C8** CLIs exceto Claude Code sem evidência de -NonInteractive | — | medição pendente (VM) |

---

## 8. MLs propostos (insumo para Wave 1 e seguintes)

Agrupados por causa (Regra Dura de Causa Raiz). ML-1A e ML-1B já estão no roadmap.

### ML-1A — Remover skip falso C1: TestMLScope_* no Windows (41 testes)
**Arquivo:** `internal/commands/barrier_ml_scope_test.go`  
**Ação:** remover o guarda `runtime.GOOS == "windows"` em todas as 41 ocorrências (`grep -n 't\.Skip("ml_scope' barrier_ml_scope_test.go` lista todas; `grep -c` retorna 41). Confirmar que fixtures não usam caminhos POSIX literais como `/tmp/...` — se usarem, substituir por `t.TempDir()`.  
**Critério de aceite:** os 41 testes passam no `windows-full-suites`; CI verde.

### ML-1B — Remover skip falso C2: TestBarrierReentry T1/T2/T3 + antipadrão T7
**Arquivo:** `internal/commands/barrier_reentry_test.go`  
**Ação:** remover guarda falso em T1 (linha 187), T2 (linha 231), T3 (linha 280). T7 (linha 439): converter skip incondicional em probe (`try os.Symlink; t.Skipf on err`). T7e: skip dinâmico — manter.  
**Critério de aceite:** T1/T2/T3/T7 passam no Windows ou T7 entra em `windows-known-failures.json` com razão medida.  
**Nota:** T1/T2/T3 usam `runGateCommand` que chama `exec.Command("sh")` — `sh` está em PATH, mas confirmar no CI antes de dar done.

### ML-C3 — Remover skips falsos C3: guard_line_shell_* (sh e bash arms)
**Arquivos:** `internal/generators/guard_line_shell_test.go`, `internal/generators/fail_closed_d11_behavior_test.go`, `internal/commands/netexec_test.go`, `internal/commands/doctor_test.go`  
**Ação:** para cada skip com "shells POSIX não disponíveis em Windows" — separar por shell:
- Braço `sh` e `bash`: substituir skip por probe `exec.LookPath("sh")` + `t.Skipf on err`.
- Braços `zsh` / `dash`: manter skip com classificação `// (c)` confirmada por `exec.LookPath` retornando err.
**Critério de aceite:** subtests sh (e possivelmente bash) rodam no Windows; zero skips com justificativa falsa sem comentário de classificação.

### ML-C3-gate — Gate de CI contra skip novo sem classificação (AC3)
**Arquivo:** `.github/windows-known-failures.json` ou novo script de gate  
**Ação:** adicionar gate que percorre `*_test.go`, detecta `runtime.GOOS == "windows"` antes de `t.Skip` sem comentário `// (a)`, `// (b)` ou `// (c)`, e reprova o build.  
**Nota:** o gate NÃO detecta `t.Log; return` (como em guard_line_shell_test.go:2316 e 2976) — esses requerem inspeção manual no PR de D18 residual.

### ML-C6 — Medir ConfirmPreference via-perfil no PS 5.1 e pwsh 7 do Windows (AC6)
**Arquivo:** `internal/generators/guard_line_shell_test.go` (linhas 2316 e 2976)  
**Ação:** converter `t.Log("braço via-profile pulado..."); return` em `t.Skip(...)` adequado, ou implementar o braço de forma que rode no Windows com PS 5.1 (injetando `$ConfirmPreference = Low` no perfil antes do hook).  
**VM necessária:** PS 5.1 sem `-NonInteractive`; inserção de `$ConfirmPreference = Low` no perfil.  
**Critério de aceite:** braço via-profile de `TestGuardLineShellMatrix_LContraArm_ConfirmLow` PASSA no windows-latest ou tem justificativa (c) medida.

### ML-C7 — Smoke npm e PyPI no windows-latest (AC5)
**Arquivo:** `.github/workflows/quality.yml`  
**Ação:** adicionar job `package-smoke-windows` que roda em `windows-latest`, faz `npm install -g trackfw` e `pip install trackfw`, executa `trackfw --version` e `trackfw validate` em diretório de fixture mínimo.  
**Critério de aceite:** job verde em duas rodadas consecutivas.

### ML-C8 — Medir -NonInteractive por CLI (AC6 parcial)
**Dependência:** VM com cada CLI instalado (Codex, Cursor, Gemini, Copilot, Windsurf, Amazon Q)  
**Ação:** para cada CLI, verificar se PS é invocado com `-NonInteractive` (inspecionando processo spawner ou instrumentando `pwsh.exe` stub). Documentar resultado em `docs/portabilidade/`.  
**Critério de aceite:** para cada CLI: confirmado `-NonInteractive` OU defeito documentado com severidade.

---

## 9. Prova de que `sh` está em PATH no windows-latest

`TestBarrierFragment_TransportSingleLineReadContra` PASSA no CI D18 (run 43766925360, job 132974038971). Esse teste chama `runGateCommand("read x; test -z \"$x\"", nil)` internamente, que executa `exec.Command("sh", "-c", ...)` em `internal/commands/barrier.go:1462`. Se `sh` não estivesse em PATH, o comando falharia com "executable file not found" e o teste reportaria FAIL — ele reporta PASS. Essa prova falsifica a justificativa "shells POSIX não disponíveis em Windows" para todos os testes que dependem somente de `sh`.

---

## 10. Padrão `os.Getenv("GOOS")` — três testes com guarda quebrada

Três testes têm o guarda `os.Getenv("GOOS") == "windows"` em vez de `runtime.GOOS == "windows"`:
- `internal/discover/discover_test.go:1331` (`TestInstallGates_ValidateScript_HardlinkNotFollowed`)
- `internal/generators/scaffold_test.go:1175` (hardlink test)
- `internal/generators/scaffold_test.go:1224` (hardlink test)

A var de ambiente `GOOS` **nunca** é definida automaticamente no runtime — `runtime.GOOS` é a forma correta. Como resultado, esses três testes **nunca saltam no Windows** e **PASSAM** no CI D18 (confirmado no log). Isso é um falso positivo de cobertura — o skip nunca protege. Os três não aparecem na lista de 71 skips porque não saltam.

Classificação das asserções: **já passam no Windows**; o guarda quebrado não causa problema de cobertura hoje, mas é código enganoso que pode levar a futuros skip mal posicionados.

---

## 11. Reconciliação com o threat model (ML-0A)

O threat model (`docs/seguranca/2026-11-15-wave0-windows-threat-model.md`, ML-0A) elencou nove
requisitos de implementação (REQ-IMP-1..9). Esta seção mapeia cada um ao(s) ML(s) proposto(s) no
inventário, ou declara "sem ML" quando o requisito não tem cobertura ainda.

| REQ-IMP | Descrição (threat model §5) | ML proposto que cobre | Status |
|---|---|---|---|
| **REQ-IMP-1** | Gate AC3 AST-based (não grep); detecta `t.Log(...); return` além de `t.Skip`; exige arquivo de registro separado de known-failures | ML-C3-gate | Parcial — o ML atual descreve gate por grep/texto; precisa ser explicitamente AST-based e detectar o padrão `t.Log+return` (ver threat model T1 e §1.1b) |
| **REQ-IMP-2** | ML-1B evidência CI medida por teste (PASS/FAIL/known-failure com log de CI citado); adicionar ao `windows-known-failures.json` sem execução no Windows não satisfaz AC2 | ML-1B | Parcial — critério de aceite atual aceita "T7 entra em windows-known-failures.json com razão medida"; precisa exigir explicitamente evidência de execução (run URL) |
| **REQ-IMP-3** | Smoke AC5 inclui dois braços de stdin que detectam drop: `echo 'git branch ...' \| guard git-branch` → rc=2; `echo '{...write-scope payload...}' \| guard write-scope` → rc=0 | ML-C7 | Parcial — ML atual pede só `--version` + `validate`; braços de stdin precisam ser adicionados explicitamente |
| **REQ-IMP-4** | AC6 dois braços por versão PS: (a) `-Command` com PS 5.1 e pwsh 7; (b) braço "perfil" via `$PROFILE` real no runner; assertion por `stdin_bytes == len(payload)`, não só `os.Stat` | ML-C6 | Parcial — ML atual cobre braço via-perfil mas não distingue PS 5.1 de pwsh 7, não impõe `$PROFILE` real e não exige assertion de `stdin_bytes` |
| **REQ-IMP-5** | AC4 dois tipos de artefato: conveniência (`*.sh`, lefthook, husky) → `validate`/`doctor` pode declarar; guards (`credential`, `git-branch`, `write-scope`) → devem executar, declaração não satisfaz | **sem ML** | Novo ML necessário: ML-portabilidade-guards — verificar que formas cmd.exe (Kiro, Amazon Q) executam sob `cmd.exe` real no windows-latest |
| **REQ-IMP-6** | Teste em `windows-full-suites` que execute `cmd /c "trackfw guard git-branch \|\| exit 2"` sob `cmd.exe` real com payload deny e allow | **sem ML** | Novo ML necessário: ML-cmdexe-guard-test |
| **REQ-IMP-7** | Job `windows-arm64` no `quality.yml` usando label `windows-11-arm` (GA para repos públicos desde 2025-08-07); repo é PUBLIC | **sem ML** | Novo ML necessário: ML-arm64 — adicionar job `windows-arm64` com os dois braços de REQ-IMP-3 |
| **REQ-IMP-8** | Estender `OSCLIHomeResolver.HomeFor()` para Codex, Gemini, Cursor, Copilot, Windsurf, Kiro com evidência medida por CLI; Amazon Q sem global hooks — não precisa | **sem ML** | Novo ML necessário: ML-homeresolver-6clis |
| **REQ-IMP-9** | `validate` e `doctor` emitem aviso proativo de `sh` ausente no PATH (não erro; texto = `shMissingMsg` em barrier.go:486) | **sem ML** | Novo ML necessário: ML-validate-sh-probe |

**Resumo de cobertura:** 4 de 9 REQ-IMP têm ML proposto (parcialmente); 5 de 9 não têm ML ainda.
Os 5 novos MLs necessários são: ML-portabilidade-guards, ML-cmdexe-guard-test, ML-arm64, ML-homeresolver-6clis, ML-validate-sh-probe.
