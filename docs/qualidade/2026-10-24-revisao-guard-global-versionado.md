# Parecer de Qualidade — ML-6A: D15 guard global versionado

**Data:** 2026-10-24  
**Agente:** hefesto-tf  
**Escopo:** `git diff origin/main..HEAD -- internal/ docs/cli-parity.md` (branch `fix/update-nao-remove-geracoes-guard-global-versionado`, HEAD ccf8730b)  
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md, Waves 4–5  

---

## Veredito: REPROVADO

Seis achados bloqueantes (B1–B6) violam a Regra Dura de Reconciliação: o nome ou o comentário de reconciliação declara o contrato oposto ao que o corpo do teste afirma.

---

## Achados Bloqueantes

### B1 — Reconciliação de `TestNestedD14Arms_Codex` declara "Bash ausente" mas o corpo afirma "Bash presente"

**Arquivo:** `internal/generators/nested_guard_migrate_dedup_test.go:503–505`

```go
// TestNestedD14Arms_Codex mirrors TestNestedD14Arms_Claude for Codex.
//
// Reconciliação: afirma que apply_patch está sempre presente, Bash ausente quando global.
func TestNestedD14Arms_Codex(t *testing.T) {
```

O comentário de reconciliação (L505) diz "**Bash ausente quando global**" — contrato D14. O corpo do único subteste (L543–545) afirma:

```go
// D15: Bash PRESENT (always emitted regardless of global).
if n := countNestedEntries(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX); n != 1 {
    t.Errorf("ArmB/Codex(D15): expected 1 cred in PreToolUse/Bash, got %d", n)
}
```

A declaração formal contradiz a asserção. Qualquer leitor que ler apenas a reconciliação vai concluir que Bash está ausente com global — o oposto do que D15 exige e do que o teste verifica.

**Correção:** substituir a reconciliação por "Reconciliação: afirma que Bash e apply_patch estão sempre presentes (D15 — wiring de projeto não depende do guard global)."

---

### B2 — Reconciliação de `TestNestedD14Arms_Gemini` declara "run_shell_command ausente" mas o corpo afirma "presente"

**Arquivo:** `internal/generators/nested_guard_migrate_dedup_test.go:554–557`

```go
// TestNestedD14Arms_Gemini mirrors TestNestedD14Arms_Claude for Gemini.
//
// Reconciliação: afirma que read/write pairs estão sempre presentes,
// run_shell_command ausente quando global.
func TestNestedD14Arms_Gemini(t *testing.T) {
```

O comentário declara "**run_shell_command ausente quando global**" — contrato D14. O corpo (L595–597) afirma:

```go
// D15: run_shell_command PRESENT (always emitted regardless of global).
if n := countNestedEntries(data, "BeforeTool", "run_shell_command", guardCredentialCmdPSPOSIX); n != 1 {
    t.Errorf("ArmB/Gemini(D15): expected 1 cred in BeforeTool/run_shell_command, got %d", n)
}
```

**Correção:** substituir por "Reconciliação: afirma que run_shell_command, read e write pairs estão sempre presentes (D15)."

---

### B3 — `TestGBGDedup_Cursor_BothGloballyInstalled_KeyAbsentNotEmpty` afirma chave PRESENTE

**Arquivo:** `internal/generators/git_branch_guard_dedup_test.go:158`

```go
func TestGBGDedup_Cursor_BothGloballyInstalled_KeyAbsentNotEmpty(t *testing.T) {
```

O nome diz "KeyAbsent" (chave ausente). O corpo (L179–183) diz:

```go
// D15: beforeShellExecution key must be PRESENT with both entries.
if _, present := hooks["beforeShellExecution"]; !present {
    t.Error("beforeShellExecution key must be present (D15: both guards always emitted)")
}
```

Nome e asserção são opostos. Confirmado: `go test -run TestGBGDedup_Cursor_BothGloballyInstalled_KeyAbsentNotEmpty -v` reporta `--- PASS` com rc=0 — o teste passa afirmando presença enquanto o nome declara ausência.

**Correção:** renomear para `TestGBGDedup_Cursor_BothGloballyInstalled_BothKeysPresent` ou similar.

---

### B4 — `TestKiroD14_ShellRemovalRequiresLegacySeed` afirma shell SEMPRE PRESENTE

**Arquivo:** `internal/generators/copilot_kiro_d14_ml2c_test.go:197`

```go
func TestKiroD14_ShellRemovalRequiresLegacySeed(t *testing.T) {
```

O nome diz "ShellRemoval" (shell é removido). O corpo (L215–219) diz:

```go
// D15: shell must be PRESENT (always emitted regardless of global).
for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
    n := kiroD14CountPair(hooksRaw, trigger, "shell", guardCredentialCmdCmdExe)
    if n != 1 {
        t.Errorf("%s×shell: count=%d, want 1 (D15: always emitted)", trigger, n)
    }
}
```

Confirmado: `go test -run TestKiroD14_ShellRemovalRequiresLegacySeed -v` — `--- PASS` com rc=0, afirmando shell presente, nome dizendo remoção.

**Correção:** renomear para `TestKiroD15_ShellAlwaysPresent` e atualizar comentário de reconciliação.

---

### B5 — `TestNestedD14Arms_Claude` doc declara "shell absent" mas subteste e corpo afirmam "ShellPresent"

**Arquivo:** `internal/generators/nested_guard_migrate_dedup_test.go:356–365`

```go
// TestNestedD14Arms_Claude runs the three D14-revised arms for Claude:
//   ArmA: no global → shell+Read+Write present, each exactly once.
//   ArmB: global CG installed → shell absent; Read+Write still present (D14 revised).
//   ArmC: global GBG installed → git-branch shell absent; cred unaffected.
//
// ...
// Reconciliação: afirma que a detecção do guard global aciona os três ramos de D14
// revisado e que os pares não cobertos pelo global permanecem presentes.
func TestNestedD14Arms_Claude(t *testing.T) {
```

A doc diz "**ArmB: global CG installed → shell absent**" (D14 contract). A reconciliação diz "D14 revisado". O subteste é `ArmB_GlobalCGInstalled_ShellPresent_ReadWritePresent` (nome diz "ShellPresent"). O corpo em `TestDedup_Claude_D14Revised_ReadWriteAlwaysPresent` (L726–728) e nos `ArmB` asserts de nested correspondentes confirmam "D15: shell pair must be PRESENT".

Dois vetores opostos na mesma função: o doc block diz "absent" (D14), o subtest name e o corpo dizem "present" (D15).

**Correção:** atualizar a doc de `TestNestedD14Arms_Claude` para "ArmB: global CG installed → shell PRESENT (D15)" e alterar a reconciliação para "afirma que todos os pares estão presentes independentemente do guard global (D15)."

---

### B6 — `TestCursorGuardMigrateDedup_D14Arms` cabeçalho e doc declaram ausência mas ArmB/ArmC afirmam presença

**Arquivo:** `internal/generators/cursor_guard_migrate_dedup_test.go:10–22, 230–244`

O cabeçalho do arquivo (L12–14) descreve o contrato D14:

```
//   AC6 — three D14-revised arms:
//          B) global credential guard installed → shell events for cred removed,
//          C) global git-branch guard installed → git-branch removed from shell
```

A doc de `TestCursorGuardMigrateDedup_D14Arms` (L233–238):

```
//   - After inject: NO cred guard in beforeShellExecution (removed, D14); user hook survives;
//   - After inject: NO git guard in shell events (removed); cred present once;
```

Reconciliação (L241–242): "afirma que a detecção do guard global aciona os três ramos do D14 revisado com dados pré-existentes".

O corpo de ArmB (L357–360) afirma:

```go
// D15: cred guard PRESENT in beforeShellExecution (always emitted).
if n := countFlatEntries(before, guardCredentialCmdPSPOSIX, ""); n != 1 {
    t.Errorf("ArmB(D15): beforeShellExecution cred guard count want 1, got %d", n)
}
```

O corpo de ArmC (L426–428) afirma:

```go
// D15: git-branch guard PRESENT (always emitted); cred guard also present.
if n := countFlatEntries(before, guardGitBranchCmdPSPOSIX, ""); n != 1 {
    t.Errorf("ArmC(D15): beforeShellExecution git guard count want 1, got %d", n)
}
```

Os três textos (cabeçalho do arquivo, doc do teste e reconciliação) declaram remoção D14; os corpos de ArmB e ArmC afirmam presença D15.

**Correção:** atualizar o cabeçalho do arquivo, a doc de `TestCursorGuardMigrateDedup_D14Arms` e a reconciliação para descrever o contrato D15 (PRESENT, não removed).

---

## Achados de Recomendação

### R1 — Stale comments em `validator_git_branch_guard.go` referenciam funções removidas

**Arquivo:** `internal/validator/validator_git_branch_guard.go:146`

```go
// Global entries are written by internal/generators (harnessCredentialGuardTarget*, agentfiles.go)
// as fully resolved absolute paths (via globalCredentialGuardScriptPath — filepath.Join(home,
```

`globalCredentialGuardScriptPath` foi removida de `agentfiles.go` em D15. O grep confirma: `grep -rn 'func globalCredentialGuardScriptPath' internal/generators/agentfiles.go` retorna vazio. O mecanismo descrito (caminhos absolutos) ainda é correto, mas a referência ao nome da função é stale.

**Também em L649:** `// see globalCredentialGuardScriptPath/globalGitBranchGuardScriptPath in internal/generators/agentfiles.go` — mesma situação.

**Também em L134–141:** comentário de `validateGuardGlobalHookResolvable` ainda descreve o comportamento D14 ("dedup means the project file has NO entry at all"). Sob D15, o projeto sempre tem a entrada.

**Correção:** substituir as referências às funções removidas pelo nome atual dos helpers de harness em `update.go`, e atualizar a frase de L140–141 para refletir D15.

---

### R2 — `docs/cli-parity.md:4508` diz "Os 6 InjectXHooks" mas são 7

**Arquivo:** `docs/cli-parity.md:4508–4509`

```
6. **D15 (ADR-2026-10-04 adendo 2026-10-24): wiring por-projeto sempre emitido.** Os 6
   `InjectXHooks` de projeto emitem a entrada de credential-guard incondicionalmente
```

Após D15, 7 funções emitem guard de credential incondicionalmente: `InjectClaudeHooks`, `InjectCodexHooks`, `InjectGeminiHooks`, `InjectKiroHooks`, `InjectCopilotHooks`, `InjectCursorHooks`, `InjectWindsurfHooks`. `InjectAmazonQHooks` nunca teve dedup (não entra na contagem). O "6" refere-se ao estado pré-Windsurf (Windsurf foi o 7º CLI, adicionado em ML-1B antes do ML-3A que introduziu o count "6"), e não foi atualizado quando Windsurf entrou.

Evidência: `grep -n 'func Inject.*Hooks' internal/generators/agentfiles.go` lista 8 funções; `grep -n 'func TestDedup_.*EmitsProject' internal/generators/credential_guard_dedup_test.go` lista 7 CLIs.

**Correção:** substituir "Os 6" por "Os 7".

---

### R3 — `TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled` nome ambíguo para D15

**Arquivo:** `internal/generators/git_branch_guard_dedup_test.go:346`

O nome implica "uma mensagem" (D14: dedup → 1 entrada combinada). O corpo (L391–393) afirma:

```go
if gitGuardEntries != 2 {
    t.Fatalf("D15: expected exactly 2 git-branch-guard entries across project+global ...")
}
```

D15 produz 2 entradas independentes (projeto + global), não 1. O nome é enganoso para um leitor que conhece D14: "MessageAppearsOnce" sugere dedup.

**Correção:** renomear para `TestGBGDedup_BothScopesFireIndependently_TwoMessages` e adicionar reconciliação explícita: "afirma que projeto + global emitem 2 entradas independentes (custo aceito por D15)."

---

### R4 — `roadmapAllStates` em `barrier.go:899` adiciona cópia a lista já duplicada em ~14 sítios

**Arquivo:** `internal/commands/barrier.go:899`

```go
var roadmapAllStates = []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}
```

Medição (grep por `[]string{"backlog"`, `[]string{"wip"`, `map[string]bool{"backlog"` excluindo `_test.go`): ~14 sítios com os 6 estados em `internal/validator/validator.go` (4×), `validator_traceid.go` (4×), `validator_roadmap_gates.go` (1×), `generators/roadmap.go` (3×), `serve/api_board.go` (1×), `commands/barrier.go` (1× — novo). O roadmap estimava "~9 sítios de 4 pacotes" — o real é ~14 sítios em 6 pacotes.

Adicionalmente, `discover/discover.go:534` e `:541` listam 5 estados (omitem "analyzing"), o que é um defeito preexistente — roadmaps em `analyzing/` não são contados pelo discover.

**Avaliação:** a causa da duplicação (ausência de constante compartilhada) é diferente da causa D15. Centralizar exige um pacote folha (como `internal/guardforms`) porque `validator` não pode importar `generators`, tocando ~14 sítios em 6 pacotes — escopo de uma REQ separada. O `discover.go` (5 estados, missing `analyzing`) é mesma causa de duplicação, deve entrar na mesma REQ de centralização (não em REQ separada — Regra Dura de Causa Raiz).

**Ação mínima neste PR:** o comentário `// Keep in sync` em L896 é suficiente por ora. A REQ de centralização deve ser aberta pelo arquiteto.

---

### R5 — Comentários stale de D14 em `agentfiles.go`

**Arquivo 1:** `internal/generators/agentfiles.go:1158–1159`

```go
// that only means there is no MIGRATION concern (nothing stale to leave
// behind) — it does not exempt this entry from the dedup-against-global
// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope git-branch guard
```

Sentença truncada: "it does not exempt this entry from the dedup-against-global" não tem objeto — a frase foi cortada quando o comentário D14 foi substituído pelo D15. O texto ainda é legível no conjunto, mas a quebra de frase é um artefato.

**Arquivo 2:** `internal/generators/agentfiles.go:1326`

```go
// Credential guard — Read/Write pairs (D14 revised: ALWAYS present).
```

"D14 revised" foi substituído por D15. O comportamento descrito ("ALWAYS present") está correto, mas a referência à decisão caducada é stale.

**Correção:** completar a sentença truncada e substituir "D14 revised" por "D15".

---

### R6 — `TestD15_GlobalOnlyDoesNotSatisfyProjectScope` não verifica a regra específica

**Arquivo:** `internal/commands/update_validate_global_guard_test.go:351`

```go
_, rc = d15RunTFW(t, workDir, fakeHome, "validate")
if rc == 0 {
    t.Fatal("D15 ArmB: validate should fail ...")
}
```

O teste verifica apenas `rc != 0`. Qualquer violação não relacionada (por exemplo, `branch_has_wip_roadmap`, `req_adr_linked`) satisfaz a asserção sem que `credential_guard_presence_required` tenha disparado. O ArmB perderia a regressão de D15 se a regra de presença fosse acidentalmente removida e outra violação passasse a dominar.

**Correção:** usar `--json` e verificar explicitamente a presença de uma violação com `rule == "credential_guard_presence_required"`.

---

### R7 — `validator_git_branch_guard.go:86` diz "6 global harness targets" mas credential guard harness tem 7

**Arquivo:** `internal/validator/validator_git_branch_guard.go:86`

```go
// Paths and CLI labels correspond to the 6 global harness targets
// in internal/generators/update.go (the globalCredentialGuardInstalled{…} functions were removed
```

O comment refere-se aos credential guard harness targets em `update.go`. Esta diff adicionou `harnessCredentialGuardTargetWindsurf` (Windsurf, ML-5A), elevando o total para 7 targets de CLI. A lista `globalGuardConfigFiles` definida abaixo tem 6 entradas (Claude, Codex, Gemini, Cursor, Copilot, Kiro) — se Windsurf não deve entrar nessa lista, o comentário deve explicar a exclusão; se deve entrar, a omissão é um gap de validação.

**Correção mínima:** corrigir "6" para "7" (ou indicar que Windsurf foi excluído intencionalmente com justificativa).

---

### R8 — `cursor_guard_migrate_dedup_test.go:21` cita "D14-revised code paths" em reconciliação file-level

**Arquivo:** `internal/generators/cursor_guard_migrate_dedup_test.go:21`

```go
//   AC6 asserts: global guard detection gates the three D14-revised code paths.
```

D15 remove a detecção de guard global dos generators. A reconciliação de AC6 deveria dizer que os três arms provam que o generator emite incondicionalmente — não que a detecção do guard global "gates" os caminhos.

**Correção:** substituir por "AC6 asserts: InjectCursorHooks always emits all entries regardless of global guard state (D15 — global detection removed)."

---

### R9 — Prefixo "Dedup" nos testes renomeados é stale

**Arquivos:** `internal/generators/credential_guard_dedup_test.go`, `internal/generators/git_branch_guard_dedup_test.go`

Os testes foram renomeados de `TestDedup_*_SkipsProjectEntryWhenGlobalInstalled` para `TestDedup_*_EmitsProjectEntryEvenWhenGlobalInstalled` e de `TestGBGDedup_*_Skips*` para `TestGBGDedup_*_EmitsProjectEntry*`. O prefixo "Dedup" / "GBGDedup" permanece, sugerindo que os testes verificam comportamento de deduplicação — o oposto do que D15 afirma (dedup removido). Os nomes agora implicam que o arquivo/fixture envolve dedup, mas a asserção é de ausência de dedup.

**Correção:** renomear os prefixos para `TestProjectWiring_*` e `TestGBGProjectWiring_*`, ou ao menos atualizar o cabeçalho dos arquivos para explicar que "Dedup" no nome é histórico e que os testes agora afirmam emissão incondicional.

---

## Positivos Verificados

**Dead code:** `go vet ./internal/generators/ ./internal/validator/ ./internal/commands/` passou com rc=0. As 12 funções `global*Installed*` e as 4 helpers (`globalCredentialGuardScriptPath`, `readGlobalHookJSON`, `readClaudeSettingsJSON`, `globalGitBranchGuardScriptPath`) foram removidas de `agentfiles.go`. O campo `globalInstalled` e as funções `credentialGuardGlobalInstalled*` foram removidos de `validator_credential_guard.go`. `credentialGuardGlobalSubcmdMarker` removido. `homedir` e `agentconfig` removidos das imports de `agentfiles.go`.

**Testes vacuosos deletados:** `TestDedup_FailOpen_*`, `TestGBGDedup_FailOpen_*`, `TestGBGDedup_MalformedGlobalEntry_*`, `TestGBGDedup_Claude_ReWires*`, `TestGBGDedup_Copilot_ReWires*`, `TestGlobalCredentialGuardInstalledWindsurf_*`, `TestInjectWindsurfHooks_ML1D_*`, `TestCredentialGuardPresenceRequired_Windsurf_GlobalCompleto_SemViolation` — todos ausentes no grep. `TestCredentialGuardPresenceRequired_Windsurf_GlobalCompleto_EmitsViolation` invertido corretamente: teste passa com rc=0.

**Não-vacuidade dos testes renomeados:** `TestDedup_Claude_EmitsProjectEntryEvenWhenGlobalInstalled` e equivalentes semeiam o arquivo global (via `helperWriteJSON(home/.claude/settings.json, ...)` com `dedupScriptPath`) antes de chamar `InjectClaudeHooks`. A fixture é idêntica à da versão D14; só a asserção foi invertida. Na origin/main, o D14 code skipparia o project entry → o teste falharia → não-vacuo por construção.

**ML-5B barrier.go:** `TestMLScope_RoadmapMovedStatePasses_Flat` e `TestMLScope_RoadmapMovedStatePasses_ByAgent` passam com rc=0. O conjunto de orquestração inclui corretamente todos os estados do `roadmap_dir` para o mesmo basename.

**TestD15 (7 subtestes):** `go test ./internal/commands/ -run 'TestD15_' -v -count=1` — todos os subtestes passam; a linha `ok` confirma rc=0. Isolamento correto: `d15CleanEnv` strips `CLAUDE_CONFIG_DIR` e reatribui a `fakeHome/.claude`; `HOME` e `USERPROFILE` apontam para `fakeHome`. `harnessCredentialGuardTargetWindsurf` usa `filepath.Join(home, ".codeium", "windsurf", "hooks.json")` onde `home = homedir.Dir()` lê `$HOME` — nenhum escape para o home real de KG. Subtestes: claude 3.01s, codex 1.59s, gemini 1.70s, cursor 0.89s, copilot 0.85s, kiro 0.68s, windsurf 0.75s; total 9.47s.

**cli-parity.md:** as alterações descrevem corretamente o comportamento D15 (generators incondicional, validator sem short-circuit), exceto pelo "6" documentado em R2.

---

## Resumo de Achados

| # | Tipo | Arquivo:linha | Descrição |
|---|------|---------------|-----------|
| B1 | Bloqueante | `nested_guard_migrate_dedup_test.go:505` | Reconciliação Codex diz "Bash ausente", corpo diz "Bash presente" |
| B2 | Bloqueante | `nested_guard_migrate_dedup_test.go:556` | Reconciliação Gemini diz "run_shell_command ausente", corpo diz "presente" |
| B3 | Bloqueante | `git_branch_guard_dedup_test.go:158` | Nome `KeyAbsentNotEmpty` afirma ausência, corpo afirma presença |
| B4 | Bloqueante | `copilot_kiro_d14_ml2c_test.go:197` | Nome `ShellRemovalRequiresLegacySeed` afirma remoção, corpo afirma presença |
| B5 | Bloqueante | `nested_guard_migrate_dedup_test.go:358,364` | Doc `TestNestedD14Arms_Claude` diz "shell absent / D14 revisado", subteste e corpo dizem "ShellPresent" |
| B6 | Bloqueante | `cursor_guard_migrate_dedup_test.go:12–13,233–242` | Cabeçalho do arquivo e doc do teste declaram "removed, D14"; ArmB e ArmC afirmam "D15: PRESENT" |
| R1 | Recomendação | `validator_git_branch_guard.go:146,649,134` | Funções removidas (D15) ainda citadas em comentários; L140–141 descreve D14 |
| R2 | Recomendação | `docs/cli-parity.md:4508` | "Os 6 InjectXHooks" — correto é 7 (Windsurf é o 7º) |
| R3 | Recomendação | `git_branch_guard_dedup_test.go:346` | Nome `MessageAppearsOnce` ambíguo para D15 (2 entradas independentes) |
| R4 | Recomendação | `barrier.go:899` | Nova cópia da lista de estados (~14 sítios existentes); `discover.go` faltando "analyzing" é mesma causa — REQ única de centralização |
| R5 | Recomendação | `agentfiles.go:1159,1326` | Sentença truncada e label "D14 revised" stale |
| R6 | Recomendação | `update_validate_global_guard_test.go:351` | ArmB verifica só `rc != 0`, não `credential_guard_presence_required` |
| R7 | Recomendação | `validator_git_branch_guard.go:86` | Diz "6 global harness targets" mas credential harness tem 7 (Windsurf adicionado neste diff) |
| R8 | Recomendação | `cursor_guard_migrate_dedup_test.go:21` | Reconciliação file-level diz "D14-revised code paths"; D15 removeu a detecção |
| R9 | Recomendação | `credential_guard_dedup_test.go`, `git_branch_guard_dedup_test.go` | Prefixo "Dedup" / "GBGDedup" nos testes renomeados implica dedup behavior — stale após D15 |
