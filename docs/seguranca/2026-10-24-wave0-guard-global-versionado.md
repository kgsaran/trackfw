# Wave 0 — D15: a fiação de projeto não depende do guard global

**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md  
**ML:** ML-4A  
**Data:** 2026-10-24  
**Agente:** hades-tf

---

## 1 — Completude da enumeração

### 1.1 Método e alcance

Grep de encerramento executado contra **toda** a árvore `internal/` e `cmd/`, excluindo `_test.go`, cobrindo os seguintes símbolos: `globalCredentialGuardInstalled`, `globalGitBranchGuardInstalled`, `credentialGuardGlobalInstalled`, `homedir\.Dir`, `UserHomeDir`, `ClaudeConfigDir`, `readGlobalHookJSON`, `readClaudeSettingsJSON`, `USERPROFILE`, `CLAUDE_CONFIG_DIR`, `Getenv("HOME")`, `LookupEnv("HOME")`. Busca separada para `globalCredentialGuardScriptPath` e `globalGitBranchGuardScriptPath`.

Resultado: nenhum sítio de guard de projeto foi encontrado fora de `internal/generators/agentfiles.go` e `internal/validator/validator_credential_guard.go`. Os detectores de CLI (`cliDetectors` em `internal/generators/hooks.go:19`) leem apenas `cwd` via `os.Stat`; nenhum lê HOME.

`ensureGlobalADRDirRegistered` (`update.go:308`) lê HOME para registrar `~/.trackfw/adr` no `trackfw.yaml` — escopo de ADR, fora do escopo de guard. Registrado para completude.

### 1.2 Sítios de projeto que consultam o global (G1–G15) — todos a remover pela D15

| # | Função | Arquivo:linha | Mecanismo | Tipo D15 |
|---|--------|---------------|-----------|----------|
| G1 | `InjectClaudeHooks` | `agentfiles.go:393` | `skipClaudeCG := globalCredentialGuardInstalledClaude()` — `if !skipClaudeCG` emite; `else` remove PreToolUse/Bash + PostToolUse credential | PULA/REMOVE |
| G2 | `InjectClaudeHooks` | `agentfiles.go:406` | `if !globalGitBranchGuardInstalledClaude()` — emite; `else` remove PreToolUse/Bash git-branch | PULA/REMOVE |
| G3 | `InjectClaudeHooks` | `agentfiles.go:416` | `if !skipClaudeCG` PostToolUse credential — `else` remove | PULA/REMOVE |
| G4 | `InjectCodexHooks` | `agentfiles.go:697` | `skipCodexCG := globalCredentialGuardInstalledCodex()` — emite / `else` remove | PULA/REMOVE |
| G5 | `InjectCodexHooks` | `agentfiles.go:707` | `if !globalGitBranchGuardInstalledCodex()` — emite; `else` remove | PULA/REMOVE |
| G6 | `InjectCodexHooks` | `agentfiles.go:715` | `if !skipCodexCG` PostToolUse credential — `else` remove | PULA/REMOVE |
| G7 | `InjectGeminiHooks` | `agentfiles.go:862` | `skipGeminiCG := globalCredentialGuardInstalledGemini()` — emite / `else` remove | PULA/REMOVE |
| G8 | `InjectGeminiHooks` | `agentfiles.go:873` | `if !globalGitBranchGuardInstalledGemini()` — emite; `else` remove | PULA/REMOVE |
| G9 | `InjectGeminiHooks` | `agentfiles.go:881` | `if !skipGeminiCG` AfterTool credential — `else` remove | PULA/REMOVE |
| G10 | `InjectKiroHooks` | `agentfiles.go:996` | `if !globalCredentialGuardInstalledKiro()` — emite; sem `else` (overwrite-semântica) | PULA |
| G11 | `InjectCopilotHooks` | `agentfiles.go:1165` | `if !globalCredentialGuardInstalledCopilot()` — emite; sem `else` | PULA |
| G12 | `InjectCopilotHooks` | `agentfiles.go:1226` | `if !globalGitBranchGuardInstalledCopilot()` — emite; sem `else` | PULA |
| G13 | `InjectCursorHooks` | `agentfiles.go:1390` | `if !globalCredentialGuardInstalledCursor()` — emite; `else` L1397 remove beforeShellExecution + afterShellExecution credential | PULA/REMOVE |
| G14 | `InjectCursorHooks` | `agentfiles.go:1420` | `if !globalGitBranchGuardInstalledCursor()` — emite; `else` L1425 remove beforeShellExecution git-branch | PULA/REMOVE |
| G15 | `InjectWindsurfHooks` | `agentfiles.go:1726` | `if !globalCredentialGuardInstalledWindsurf()` — emite; `else` L1738 remove pre_run_command + L1739 pre_write_code credential | PULA/REMOVE |

**Nota Windsurf git-branch:** não existe `globalGitBranchGuardInstalledWindsurf`; o git-branch do Windsurf é sempre emitido (`agentfiles.go:1719–1721`). Nenhum sítio a remover.

**Nota Amazon Q:** sem dedup; credential e git-branch sempre emitidos. Nenhum sítio a remover.

**Nota Cursor Read/Write pairs (L1407–L1413):** o comentário "ALWAYS present" já os torna incondicionais independentes do global. D15 não os toca.

**Nota Kiro e Copilot (G10–G12):** sem `else remove`. O único sítio é o `if !global` que pula a emissão.

### 1.3 Funções de suporte — mortas após remoção dos G1–G15

Grep de callers (`grep -rn 'globalCredentialGuardScriptPath\|globalGitBranchGuardScriptPath' internal/ --include='*.go' | grep -v _test`) confirma que ambas as funções (`agentfiles.go:2065` e `agentfiles.go:2576`) são chamadas exclusivamente pelos 12 `global*Installed*` que serão removidos.

`readGlobalHookJSON` (`agentfiles.go:2077`) e `readClaudeSettingsJSON` (`agentfiles.go:2098`) são chamadas exclusivamente pelos mesmos 12 (L2374, L2409, L2435, L2460, L2485, L2534 para `readGlobalHookJSON`; L2374, L2594 para `readClaudeSettingsJSON`). Sem callers remanescentes — código morto — adicionar à lista de remoção.

Funções a remover de `agentfiles.go`:

| Função | Linha |
|--------|-------|
| `globalCredentialGuardScriptPath` | 2065 |
| `readGlobalHookJSON` | 2077 |
| `readClaudeSettingsJSON` | 2098 |
| `globalCredentialGuardInstalledClaude` | 2368 |
| `globalCredentialGuardInstalledCodex` | 2404 |
| `globalCredentialGuardInstalledGemini` | 2430 |
| `globalCredentialGuardInstalledCursor` | 2455 |
| `globalCredentialGuardInstalledCopilot` | 2480 |
| `globalCredentialGuardInstalledKiro` | 2508 |
| `globalCredentialGuardInstalledWindsurf` | 2533 |
| `globalGitBranchGuardScriptPath` | 2576 |
| `globalGitBranchGuardInstalledClaude` | 2588 |
| `globalGitBranchGuardInstalledCodex` | 2620 |
| `globalGitBranchGuardInstalledGemini` | 2645 |
| `globalGitBranchGuardInstalledCursor` | 2670 |
| `globalGitBranchGuardInstalledCopilot` | 2695 |

### 1.4 Sítio de validador — o único ACEITA AUSÊNCIA a remover

| Sítio | Arquivo:linha | Efeito atual | Efeito após D15 |
|-------|---------------|-------------|-----------------|
| `globalInstalled: credentialGuardGlobalInstalledWindsurf` | `validator_credential_guard.go:748` | short-circuit na L901 suprime `guard_wiring_removed` para Windsurf quando global totalmente instalado | campo deve ser `nil`; `credentialGuardGlobalInstalledWindsurf` (L829) removida |

### 1.5 Sítios de escopo GLOBAL (mantidos pela D15 — não são sítios de projeto)

| Sítio | Arquivo | O que faz |
|-------|---------|-----------|
| `validateGuardGlobalHookResolvable` | `validator_git_branch_guard.go:163` | valida se o arquivo global existe e é resolvível — escopo global |
| `UpdateHarness` / `harness*Target*` | `update.go:583+` | escreve os arquivos globais — escopo global |
| `validateGuardWiringRemoved` | `validator_guard_wiring.go` | compara tuplas origin/main vs disco — não consulta global state, continua intacto |

Status: **concluído** — grep sobre `internal/ cmd/` excluindo `_test.go` com 16 padrões mais os 2 scriptPath não revelou sítio adicional além dos G1–G15 e do sítio de validador.

---

## 2 — Modelo de ameaça

### 2.1 Quem esvazia esta Wave 0 sem violar nenhuma regra escrita

O adversário é o implementador apressado e o arquiteto otimista.

**Remoção parcial — deixar o `if` após apagar o `else` (risco mais provável):**
Para G1–G9 e G13–G15, é possível remover apenas o corpo do `else remove` e deixar o `if !global` no lugar. Resultado: guard não emitido com global instalado. Nenhum build falha; o único gate que captura é o teste no braço `with-global` que afirma explicitamente **presença** do par. Para G10–G12 (Kiro, Copilot — sem `else`), basta remover o corpo do `if` e deixar a guarda — guard não emitido com global instalado, mas também sem remoção.

**Inversão incompleta do validador:**
Remover a guarda no gerador mas manter `entry.globalInstalled: credentialGuardGlobalInstalledWindsurf` (`validator_credential_guard.go:748`). Resultado: `update` passa a emitir o credential no Windsurf, mas o `validate` aceita a ausência quando global está instalado — mascara o defeito no CI local do mantenedor (que tem global). AC9 fecha isso.

**HOME falso gerado à mão no AC9:**
Se o JSON do HOME falso não bater com a forma exata que `credentialGuardGlobalInstalledWindsurf` reconhece, a função retorna `false` no código `origin/main` e o contra-braço falha por razão errada (forma desconhecida, não ausência de projeto). Mitigação: gerar via `UpdateHarness` real.

**Vazamento de env no AC9:**
`internal/homedir.Dir()` (`homedir.go:32`) usa `os.Getenv("HOME")` diretamente — **sem cache** (não usa mitchellh/go-homedir; verificado: `DisableCache` não existe no pacote). `t.Setenv("HOME", fakeHome)` isola corretamente. Para `readClaudeSettingsJSON` (usa `agentconfig.ClaudeConfigDir`): `CLAUDE_CONFIG_DIR=""` cai para OS home (`agentconfig.go:279`: entra no `if` somente se `raw != ""`); recomendar `t.Setenv("CLAUDE_CONFIG_DIR", fakeHome+"/.claude")` para apontar explicitamente ao HOME falso.

### 2.2 Fail-open introduzido pela D15

Nenhum fail-open novo. A D15 remove a condição de pular: o guard sempre dispara. Os `global*Installed*` (fail-open: erro → `false` → emit) tornam-se código morto — o fail-open desaparece.

### 2.3 Duplicação de disparo (global + projeto no mesmo evento)

**Credential guard — idempotência medida por código-fonte:**
- Projeto (padrão `warn`, rc=0): `os.WriteFile` (overwrite) em `roadmap_dir/.trackfw-credential-guard.json`
- Global (padrão `block`, rc=2): `os.WriteFile` em `docs/roadmaps/.trackfw-credential-guard.json` (fixo)
- Se `roadmap_dir == "docs/roadmaps"` (padrão): segundo overwrite sobrescreve o primeiro; conteúdo idêntico exceto `timestamp`. Resultado final indistinguível de disparo único.
- Se `roadmap_dir` diverge: dois arquivos; `trackfw serve` monitora apenas o de projeto (residual R2).
- Se global em `block`: rc=2; ferramenta bloqueada.

**Git-branch guard:**
- Grep de `permissionDecision` em `internal/ cmd/` produção: **um único sítio**, `internal/guard/gitbranch.go:193`, valor fixo `"deny"`. Nenhuma emissão de `allow`. Sem race entre allow e deny.
- Com dois hooks: dois `deny` no stdout + dois rc=2; agente bloqueia. Mensagem dobrada aceita pela D15 (`TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled_NonVacuous`, `git_branch_guard_dedup_test.go:565`).

**Ordenação hook global × projeto:** `sequential` field ordena dentro de um grupo de matcher (`agentfiles.go:759–763`); ordem entre `~/.claude/settings.json` e `./.claude/settings.json` não documentada. Irrelevante: qualquer rc=2 bloqueia; sem emissão de allow → sem race.

Status: **concluído** — grep de `permissionDecision` confirma `"deny"` único; idempotência derivada de `os.WriteFile`; homedir sem cache verificado em `homedir.go:32–35`; UpdateHarness signature verificada: `func UpdateHarness(opts UpdateOptions) (UpdateReport, error)` (`update.go:583`).

---

## 3 — Alvos de falsificação em ambas as direções

### 3.1 Falso negativo (D15 não entregue — guard ausente quando deveria estar)

| Superfície | Onde a sabotagem entra | Gate que deve capturar |
|-----------|----------------------|------------------------|
| G10/G11/G12 — `if !global` sem `else` | `if !global` com corpo vazio | Teste `with-global` afirmando **presença** de "shell"/"bash"/git-branch com global instalado |
| G1–G9, G13–G15 — `else remove` | `else` removido mas `if !global` mantido | Mesmos testes invertidos para cada agent |
| G15 Windsurf + validador | `credentialGuardGlobalInstalledWindsurf` mantida no validador | AC9: `update` → `validate` rc=0 com HOME falso gerado por `UpdateHarness` |
| Validador L748 | `globalInstalled` field não zerado | `TestCredentialGuardPresenceRequired_Windsurf_GlobalCompleto_SemViolation` (L1153) invertido: deve virar violation |

### 3.2 Falso positivo (aperto do validador quebra repositório anteriormente verde)

**Condição exata:** repositório com `.windsurf/hooks.json` sem credential guard E guard global Windsurf totalmente instalado. Antes de D15: sem violation. Após D15: `guard_wiring_removed`.

**Quem está nessa condição:** consumidores que rodaram `trackfw update` na v11.0.0. Após PR da D15, `trackfw update` reemite e repara. CI já emite a violation hoje (sem global). O aperto alinha o mantenedor com o CI.

**Nota para a decisão de versão (cabe ao arquiteto):** o aperto é breaking change de comportamento do `validate` para consumidores Windsurf em estado específico. Único sítio: `validator_credential_guard.go:748` (um campo zerado).

### 3.3 Falsificação do contra-braço da AC9

**Risco:** HOME falso à mão pode usar forma não reconhecida por `credentialGuardGlobalInstalledWindsurf` em `origin/main`.

**Receita verificada para o AC9:**
```go
t.Setenv("HOME", fakeHome)
t.Setenv("CLAUDE_CONFIG_DIR", fakeHome+"/.claude")
// Gerar o HOME falso com o harness real:
_, err := generators.UpdateHarness(generators.UpdateOptions{
    DryRun:         false,
    Targets:        []string{"windsurf-credential-guard"},
    InstallMissing: true,
})
// ... então chamar InjectWindsurfHooks sobre projeto vazio e validate
```

`internal/homedir.Dir()` usa `os.Getenv("HOME")` sem cache — `t.Setenv` suficiente. `CLAUDE_CONFIG_DIR` explícito evita dependência do fallback de `agentconfig.ClaudeConfigDir`.

### 3.4 Testes que afirmam D14 e devem inverter para D15

| Teste | Arquivo:linha | Afirma (D14) | Deve afirmar (D15) |
|-------|---------------|-------------|-------------------|
| `TestDedup_Claude_SkipsProjectEntryWhenGlobalInstalled` | `credential_guard_dedup_test.go:35` | com global, credential ausente | **presente** |
| `TestDedup_Codex_SkipsProjectEntryWhenGlobalInstalled` | L69 | idem | **presente** |
| `TestDedup_Gemini_SkipsProjectEntryWhenGlobalInstalled` | L103 | idem | **presente** |
| `TestDedup_Cursor_SkipsProjectEntryWhenGlobalInstalled` | L137 | beforeShellExecution credential ausente | **presente** |
| `TestDedup_Copilot_SkipsProjectEntryWhenGlobalInstalled` | L207 | bash credential ausente | **presente** |
| `TestDedup_Kiro_SkipsProjectEntryWhenGlobalInstalled` | L292 | shell credential ausente | **presente** |
| `TestDedup_Windsurf_SkipsProjectEntryWhenGlobalInstalled` | L415 | pre_run_command/pre_write_code ausentes | **ambos presentes** |
| `TestGBGDedup_Claude_SkipsProjectEntryWhenGlobalInstalled` | `git_branch_guard_dedup_test.go:27` | PreToolUse/Bash git-branch ausente | **presente** |
| `TestGBGDedup_Codex_SkipsProjectEntryWhenGlobalInstalled` | L60 | idem | **presente** |
| `TestGBGDedup_Gemini_SkipsProjectEntryWhenGlobalInstalled` | L88 | idem | **presente** |
| `TestGBGDedup_Cursor_SkipsProjectEntryWhenGlobalInstalled` | L116 | beforeShellExecution git-branch ausente | **presente** |
| `TestGBGDedup_Cursor_BothGloballyInstalled_KeyAbsentNotEmpty` | L145 | chave ausente (não empty) com ambos globais | chave presente com ambas as entradas |
| `TestGBGDedup_Copilot_SkipsProjectEntryWhenGlobalInstalled` | L176 | bash git-branch ausente | **presente** |
| `TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled` | L478 | dedup → 1 entrada combinada | 2 entradas independentes |
| `TestCopilotD14_ViewCreateEditAlwaysPresent/with-global` | `copilot_kiro_d14_ml2c_test.go:112` | bash ausente com global | **presente** |
| `TestKiroD14_ReadWriteAlwaysPresent/with-global` | L168 | shell ausente com global | **presente** |
| `TestKiroD14_ShellRemovalRequiresLegacySeed` | L230 | overwrite → shell ausente | overwrite → shell **presente** |
| `TestWindsurfCredential_D14GlobalInstalledRemovesProject` | `guard_harness_dedup_test.go:689` | credential removido quando global | **presente** |
| `TestCredentialGuardPresenceRequired_Windsurf_GlobalCompleto_SemViolation` | `validator_credential_guard_test.go:1153` | sem violation com global e projeto sem credential | **violation** |

### 3.5 Testes que se tornam vacuosos ou devem ser deletados

| Teste | Arquivo:linha | Razão |
|-------|---------------|-------|
| `TestDedup_FailOpen_NoGlobalFile` | `credential_guard_dedup_test.go:350` | afirma "emite" — vacuoso (D15 sempre emite) |
| `TestDedup_FailOpen_CorruptedGlobalFile` | L364 | idem |
| `TestDedup_FailOpen_UnreadableGlobalFile` | L385 | idem |
| `TestDedup_Windsurf_FailOpenWhenGlobalMissing` | L464 | idem |
| `TestGBGDedup_FailOpen_NoGlobalFile` | `git_branch_guard_dedup_test.go:386` | idem |
| `TestGBGDedup_FailOpen_CorruptedGlobalFile` | L400 | idem |
| `TestGBGDedup_MalformedGlobalEntry_ProjectStillProtects` | L330 | idem |
| `TestGBGDedup_Claude_ReWiresProjectEntryWhenGlobalEntryMissingType` | L259 | rewiring irrelevante sem dedup |
| `TestGBGDedup_Copilot_ReWiresProjectEntryWhenGlobalEntryMissingType` | L286 | idem |
| `TestGlobalCredentialGuardInstalledWindsurf_BothEvents_ReturnsTrue` | `credential_guard_windsurf_amazonq_ml1b_test.go:372` | função sendo deletada |
| `TestGlobalCredentialGuardInstalledWindsurf_OnlyPreRunCommand_ReturnsFalse` | L389 | idem |
| `TestInjectWindsurfHooks_ML1D_SoPre_RunCommand_GlobalInstala` | L408 | comportamento que D15 inverte |
| `TestCredentialGuardGlobalInstalled_*` (L1264, L1279, L1297) | `validator_credential_guard_test.go` | função `credentialGuardGlobalInstalledWindsurf` sendo removida |
| `TestCredentialGuardPresenceRequired_Windsurf_GlobalD2Legacy_Violation` | L1310 | comportamento pré-D15 |

### 3.6 Divisão de ML (recomendação para o arquiteto)

`agentfiles.go` e `validator_credential_guard.go` são disjuntos — possível paralelizar. Contudo, `update_validate_global_guard_test.go` (AC9) cruza ambos e só pode ser escrito no commit que fecha os dois lados. Qualquer divisão entregaria commit intermediário com AC9 vermelho.

Recomendação: ML-5A único editando ambos. Se o arquiteto optar por paralelizar, AC9 vai no commit que fecha o último lado.

Status: **concluído** — 19 testes que afirmam D14 com arquivo:linha; 13 vacuosos ou a deletar; AC9 recipe com tipos verificados; falsificações com evidências concretas.

---

## 4 — Residual declarado

**R1 — Duplicação de mensagem no git-branch com ambos os escopos ativos:**
Aceito explicitamente pela D15. Não é falha: qualquer rc=2 bloqueia, independente da ordem. Coberto por `TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled_NonVacuous`.

**R2 — Dois arquivos de atenção quando `roadmap_dir` diverge de `docs/roadmaps`:**
Projeto em `roadmap_dir/`; global em `docs/roadmaps/`. O arquivo global fica fora do monitoramento do `trackfw serve`. Ruído de housekeeping; não é falha de segurança.

**R3 — Ordenação de hooks global × projeto não documentada em Claude Code, Gemini, Cursor, Windsurf:**
Irrelevante: grep de `permissionDecision` em toda a árvore de produção confirma que nenhuma emissão de `allow` existe (`internal/guard/gitbranch.go:193` — único sítio, valor sempre `"deny"`). O veredito é determinado pelo rc, não pela ordem.

**R4 — `readGlobalHookJSON` e `readClaudeSettingsJSON` tornam-se código morto:**
Estão na lista de remoção de ML-5A. Sem callers remanescentes após remoção dos G1–G15.
