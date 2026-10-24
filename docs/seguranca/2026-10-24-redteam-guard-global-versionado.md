# Parecer de Segurança — Red-Team D15 (guard global versionado)

> Gerado em: 2026-10-24 | Agente: hades-tf | ML: 6B
> Branch: `fix/update-nao-remove-geracoes-guard-global-versionado`
> HEAD: `ccf8730b`

---

## 1. Escopo

Red-team da implementação da D15 (ADR-2026-10-04, adendo 2026-10-24): remoção de toda consulta ao
guard global nas generators de escopo de projeto. Antes (D14): `if !globalXInstalledCLI() { emit }
else { remove }`. Depois (D15): emit incondicional para todos os CLIs.

Arquivos auditados (diff `origin/main..HEAD -- internal/`):

- `internal/commands/barrier.go` (ML-5B)
- `internal/commands/barrier_ml_scope_test.go`
- `internal/commands/update_validate_global_guard_test.go` (AC9)
- `internal/generators/agentfiles.go`
- `internal/generators/copilot_kiro_d14_ml2c_test.go`
- `internal/generators/credential_guard_dedup_test.go`
- `internal/generators/credential_guard_windsurf_amazonq_ml1b_test.go`
- `internal/generators/cursor_guard_migrate_dedup_test.go`
- `internal/generators/git_branch_guard_dedup_test.go`
- `internal/generators/guard_harness_dedup_test.go`
- `internal/generators/nested_guard_migrate_dedup_test.go`
- `internal/validator/export_test.go`
- `internal/validator/validator_credential_guard.go`
- `internal/validator/validator_credential_guard_test.go`
- `internal/validator/validator_git_branch_guard.go`

Método: archive `git archive HEAD | tar -x -C $T` em dir temporário. Binário de linha de base
compilado no archive (`trackfw_baseline`). Para falsificação de generator: reverse-patch com
`git diff origin/main..HEAD -- internal/generators/agentfiles.go | patch -R -p1`, binário
compilado separado (`trackfw_rp`). Para falsificação de validador: cópia isolada com stub de
retorno imediato. Nenhum arquivo do repositório foi modificado. Proibições respeitadas.

---

## 2. Vetores testados

### Baseline — HEAD (unsabotaged)

```
go test ./internal/generators/ -run 'TestDedup_.*EmitsProjectEntry' -timeout 30s
ok  github.com/kgsaran/trackfw/internal/generators  0.600s  rc=0

go test ./internal/commands/ -run 'TestD15_UpdatePreservesProjectGuardWithGlobalInstalled' -timeout 60s
ok  github.com/kgsaran/trackfw/internal/commands  5.038s  rc=0

go test ./internal/validator/ -run 'TestCredentialGuardPresenceRequired_Windsurf_GlobalCompleto_EmitsViolation' -timeout 30s
ok  github.com/kgsaran/trackfw/internal/validator  0.271s  rc=0
```

Todos passam no HEAD.

---

### S1-S3 — Reverse-patch: restauração completa do D14 generator

**Sabotagem (fiel):** `git diff origin/main..HEAD -- internal/generators/agentfiles.go | patch -R -p1`
no archive. Isso restaura o código real do D14 — detectors leem HOME e retornam verdadeiro quando o
harness está instalado. O validador permanece no HEAD (D15). Binário recompilado: `trackfw_rp`.

**Resultado (dedup unit tests — todos os 7 CLIs falham):**

```
go test ./internal/generators/ -run 'TestDedup_.*EmitsProjectEntry' -timeout 30s
--- FAIL: TestDedup_Claude_EmitsProjectEntryEvenWhenGlobalInstalled (0.00s)
    credential_guard_dedup_test.go:58: project-scope credential-guard PreToolUse must be present
      even when global is installed (D15)
    credential_guard_dedup_test.go:61: project-scope credential-guard PostToolUse must be present
      even when global is installed (D15)
--- FAIL: TestDedup_Codex_EmitsProjectEntryEvenWhenGlobalInstalled (0.00s)
    credential_guard_dedup_test.go:93: project-scope credential-guard PreToolUse must be present
      even when global is installed (D15)
    credential_guard_dedup_test.go:96: project-scope credential-guard PostToolUse must be present
      even when global is installed (D15)
--- FAIL: TestDedup_Gemini_EmitsProjectEntryEvenWhenGlobalInstalled (0.00s)
    credential_guard_dedup_test.go:128: project-scope credential-guard BeforeTool must be present
      even when global is installed (D15)
    credential_guard_dedup_test.go:131: project-scope credential-guard AfterTool must be present
      even when global is installed (D15)
--- FAIL: TestDedup_Cursor_EmitsProjectEntryEvenWhenGlobalInstalled (0.00s)
    credential_guard_dedup_test.go:177: credential-guard must be present in beforeShellExecution
      even when global is installed (D15)
    credential_guard_dedup_test.go:191: credential-guard must be present in afterShellExecution (D15)
--- FAIL: TestDedup_Copilot_EmitsProjectEntryEvenWhenGlobalInstalled (0.00s)
    credential_guard_dedup_test.go:237: bash credential-guard must be present in preToolUse
      even when global is installed (D15)
    credential_guard_dedup_test.go:249: bash credential-guard must be present in postToolUse
      even when global is installed (D15)
--- FAIL: TestDedup_Kiro_EmitsProjectEntryEvenWhenGlobalInstalled (0.00s)
    credential_guard_dedup_test.go:300: expected 9 hooks (D15: shell credential-guard present), got 7
--- FAIL: TestDedup_Windsurf_EmitsProjectEntryEvenWhenGlobalInstalled (0.00s)
    credential_guard_dedup_test.go:366: project-scope credential-guard must be present in
      pre_run_command even when global is installed (D15)
    credential_guard_dedup_test.go:366: project-scope credential-guard must be present in
      pre_write_code even when global is installed (D15)
FAIL  github.com/kgsaran/trackfw/internal/generators  0.598s  rc=1
```

Todos os 7 CLIs capturados pelos testes unitários de generator. ✓

**Resultado (AC9 e2e — todos os 7 CLIs falham):**

```
go test ./internal/commands/ -run 'TestD15_UpdatePreservesProjectGuardWithGlobalInstalled' -timeout 120s
--- FAIL: TestD15_UpdatePreservesProjectGuardWithGlobalInstalled/claude
    update_validate_global_guard_test.go:279: D15 violation for claude:
      guard_wiring_removed fired after update with global guard installed
      guard_wiring_removed: .claude/settings.json (Claude Code): guard "credential" wiring
        (event="PostToolUse" matcher="Bash|PowerShell") was present in origin/main and is absent
        from disk — this is a security control; investigate before proceeding
      guard_wiring_removed: .claude/settings.json (Claude Code): guard "credential" wiring
        (event="PreToolUse" matcher="Bash|PowerShell") was present in origin/main and is absent
        from disk — this is a security control; investigate before proceeding
      guard_wiring_removed: .claude/settings.json (Claude Code): guard "git-branch" wiring
        (event="PreToolUse" matcher="Bash|PowerShell") was present in origin/main and is absent
        from disk — this is a security control; investigate before proceeding
--- FAIL: TestD15_UpdatePreservesProjectGuardWithGlobalInstalled/codex (3 violations)
--- FAIL: TestD15_UpdatePreservesProjectGuardWithGlobalInstalled/gemini (3 violations)
--- FAIL: TestD15_UpdatePreservesProjectGuardWithGlobalInstalled/cursor (3 violations)
--- FAIL: TestD15_UpdatePreservesProjectGuardWithGlobalInstalled/copilot (3 violations)
--- FAIL: TestD15_UpdatePreservesProjectGuardWithGlobalInstalled/kiro (2 violations)
--- FAIL: TestD15_UpdatePreservesProjectGuardWithGlobalInstalled/windsurf (4 violations)
FAIL  github.com/kgsaran/trackfw/internal/commands  5.241s  rc=1
```

AC9 captura corretamente todos os 7 CLIs pela via `guard_wiring_removed`. ✓

**Mecanismo confirmado:** o comportamento temporal de AC9 é: (1) `init` sem harness global instalado
→ origin/main recebe o guard; (2) install do harness global; (3) `update` com D14 restaurado →
detectors retornam verdadeiro → guard removido → `guard_wiring_removed` dispara. Dedup tests e AC9
medem aspectos complementares: dedup tests verificam a saída do generator isolado; AC9 verifica o
ciclo completo `init → install-global → update → validate`.

---

### S4 — Windsurf: restauração do short-circuit no validador

**Sabotagem:** cópia isolada `s4_windsurf_validator/internal/validator/`. Arquivo `zz_s4_sabotage.go`
define `credentialGuardGlobalInstalledWindsurf_S4() bool { return true }`. `validator_credential_guard.go`
restaurado para incluir o campo `globalInstalled func() bool` na struct
`credentialGuardRequiredEntry` e o `if entry.globalInstalled != nil && entry.globalInstalled() { continue }`
em `validateCredentialGuardPresenceRequired`.

**Resultado (validator unit test):**
```
go test ./internal/validator/ -run TestCredentialGuardPresenceRequired_Windsurf_GlobalCompleto_EmitsViolation
--- FAIL (0.00s)
    validator_credential_guard_test.go:1169: D15: expected violation for .windsurf/hooks.json with
      no project credential guard, even with global fully installed, got: []
rc=1
```
Capturado. ✓

**Resultado (AC9 ArmB):**
```
go test ./internal/commands/ -run TestD15_GlobalOnlyDoesNotSatisfyProjectScope
--- FAIL (0.57s)
    update_validate_global_guard_test.go:353: D15 ArmB: validate should fail when global is
      installed but .windsurf/hooks.json has no project credential guard
rc=1
```
Capturado. ✓

---

### Ataque 2 — HOME leakage no escopo de projeto após D15

Grep de `homedir.Dir()` e `agentconfig.ClaudeConfigDir` em `agentfiles.go` (HEAD): ambas as
importações removidas. As ocorrências remanescentes de `homedir.Dir()` em `update.go` (linhas
174, 309, 589, 771, 3015) e `agentconfig.ClaudeConfigDir` em `update.go` pertencem ao caminho de
harness global, não ao gerador de escopo de projeto. `validator_git_branch_guard.go` lê HOME
somente em `validateGuardGlobalHookResolvable` — escopo de validação global, correto.

Probe `TestHomeIsolation_Claude` e `TestHomeIsolation_Windsurf` (código de verificação compilado
contra HEAD): saída de `InjectClaudeHooks` e `InjectWindsurfHooks` em dir vazio é idêntica entre
HOME vazio e HOME com seed do credential guard global. `rc=0` em ambos.

**Resultado: sem HOME leakage no escopo de projeto após D15.** ✓

---

### Ataque 3 — Idempotência de injeção (herança de D14)

`TestInjectWindsurfHooks_ML1B_PreWriteCodeIdempotent` (pré-existente, HEAD archive): `rc=0`. ✓

D15 herda idempotência das funções de merge (`mergeClaudeHookArray`, `migrateDedupGuardInFlatArray`,
`mergeSimpleCommandArray`) — essas funções são no-ops quando a entrada já existe. O diff de D15
não alterou nenhuma dessas funções.

---

### Ataque 4 — ML-5B path traversal na expansão de siblings

`barrier.go:899` (ML-5B):
```go
// roadmapAllStates is the ordered list of valid roadmap state directory names
// used by `trackfw roadmap move`. Keep in sync with the states accepted by that
var roadmapAllStates = []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}
```

Probe `TestMLScopeOrchestration_PathCases` (compilado contra HEAD):

| entrada | sibling em conjunto? | esperado |
|---|---|---|
| `docs/roadmaps/wip/X.md` | `docs/roadmaps/done/X.md` | correto ✓ |
| `docs/roadmaps/apolo-tf/wip/X.md` | `apolo-tf/done/X.md` | correto ✓ |
| `docs/roadmaps/apolo-tf/wip/X.md` | `zeus-tf/done/X.md` | ausente ✓ |
| `X.md` (sem pai de estado) | nenhum sibling | guardado ✓ |
| `internal/wip/X.md` | `internal/done/X.md` | presente — ver R-2 |

`..` traversal: `relPath = filepath.ToSlash(filepath.Rel(topLevel, absRoadmap))` — para qualquer
roadmap dentro do worktree nunca produz componente `..`. ✓

`TestMLScope_RoadmapMovedStatePasses_ByAgent` (T12): `rc=0` — contra-braço by_agent corretamente
rejeita sibling de outro agente. ✓

**Entrada real de `buildMLScopeOrchestrationSet`:** a função recebe o caminho resolvido por
`resolveBarrierRoadmap`, que busca o basename apenas em `wip/` e `done/` sob `cfg.RoadmapDir`
(`barrier.go:141–150`). Um path como `internal/wip/X.md` só entra na expansão se `roadmap_dir`
for configurado como `internal` ou ancestral que inclua `internal/wip/`. Em configuração padrão
(`roadmap_dir: docs/roadmaps`), `resolveBarrierRoadmap` retornaria erro antes de chegar à expansão.

---

### Ataque 5 — AC9: dependência de PATH (sistema vs. branch)

`barrierBinary` compila via `go build ./cmd/trackfw` a partir do `projRoot` do archive — usa sempre
o binário da branch, nunca `/opt/homebrew/bin/trackfw`. `d15CleanEnv` desafixa `CLAUDE_CONFIG_DIR`,
pino em `fakeHome/.claude`, prepend `binDir` ao PATH. `e2eCleanEnv` fixa `HOME`, `USERPROFILE`,
`GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM=/dev/null`.

`CODEX_HOME`, `XDG_CONFIG_HOME`, `CODEIUM_HOME` não removidos por `e2eCleanEnv`. Verificação:
`git show origin/main:internal/generators/agentfiles.go | grep 'Getenv'` — nenhum `Getenv`
encontrado nos detectors. Os detectors do D14 usam `homedir.Dir()` (lê `HOME` via Go stdlib) e
`agentconfig.ClaudeConfigDir` (lê `CLAUDE_CONFIG_DIR` ou `~/.claude`). `HOME` está fixado por
`e2eCleanEnv`. CODEX/XDG/CODEIUM não são lidos por nenhum detector de D14.

**Resultado: AC9 testa o binário da branch; isolamento de HOME é suficiente.** ✓

---

## 3. Achados

### R-1 — RESIDUAL: `roadmapAllStates` requer sincronização manual

**Arquivo:** `internal/commands/barrier.go`, linha 895–899

O conjunto de estados é enumerado independentemente em quatro sítios sem uma constante canônica
compartilhada:

| sítio | estados listados |
|---|---|
| `barrier.go:899` | backlog, analyzing, wip, blocked, done, abandoned (6) |
| `validator.go:2099` | backlog, analyzing, wip, blocked, done, abandoned (6) |
| `validator.go:1864` | backlog, analyzing, wip, blocked, done, abandoned (6) |
| `discover.go:534` | backlog, wip, blocked, done, abandoned (5) — difere: "analyzing" ausente |

`barrier.go:899` está correto para D15. A divergência em `discover.go:534` é pré-existente ao diff
de D15 (não introduzida por este trabalho). O risco é que uma adição futura de estado em
`trackfw roadmap move` atualize apenas alguns dos quatro sítios.

**Não bloqueante para este merge.** Registrado para tratamento separado.

---

### R-2 — RESIDUAL: sibling expansion alcança caminhos fora do roadmap_dir em configurações não-padrão

**Arquivo:** `internal/commands/barrier.go`, `buildMLScopeOrchestrationSet` (ML-5B)

Medido em `TestMLScopeOrchestration_PathCases`: um roadmap em `internal/wip/X.md` (passado
diretamente à função) expande para `internal/done/X.md` etc. Qualquer diretório cujo pai tenha nome
que colida com um estado (ex.: `internal/wip/`) é tratado como sibling de roadmap.

Limite real da exposição: `buildMLScopeOrchestrationSet` só é chamado com o path retornado por
`resolveBarrierRoadmap` (`barrier.go:1252`), que busca o basename apenas em `wip/` e `done/` sob
`cfg.RoadmapDir`. Em configuração padrão, caminhos como `internal/wip/X.md` não existem em
`cfg.RoadmapDir` e `resolveBarrierRoadmap` retornaria erro sem chegar à expansão.

**Não bloqueante.** Contexto para revisão futura de hardening do barrier em projetos com
`roadmap_dir` não-convencional.

---

### R-3 — RESIDUAL: cross-whitelist de basename idêntico em modo flat

**Arquivo:** `internal/commands/barrier.go`, `buildMLScopeOrchestrationSet`

Em modo `roadmap_namespacing: flat`, dois roadmaps com o mesmo basename em estados diferentes
(ex.: `wip/refactor-auth.md` e `done/refactor-auth.md`) whitelist mutuamente os caminhos siblings
um do outro no ml_scope. ML-5B foi criado precisamente para este fim — o barrier deve aceitar o
roadmap em qualquer estado do roadmap_dir durante um ciclo de revisão.

**Não bloqueante.** Comportamento esperado e propósito do ML-5B.

---

## 4. Veredito

| achado | severidade | ação |
|---|---|---|
| R-1: `roadmapAllStates` sem constante canônica | residual aceito | issue ou ML futuro |
| R-2: sibling expansion para caminhos non-standard | residual aceito | revisão futura de barrier |
| R-3: cross-whitelist basename idêntico flat | residual aceito | comportamento esperado |

**Nenhum bloqueante encontrado.**

A D15 como implementada é correta e segura:

- Remoção dos 16 `global*Installed*` é completa — sem HOME leakage em escopo de projeto.
- Todos os vetores de regressão D14 são capturados pela suíte: dedup tests (`TestDedup_*_EmitsProjectEntryEvenWhenGlobalInstalled`) capturam a regressão de generator na camada unitária; AC9 (`TestD15_UpdatePreservesProjectGuardWithGlobalInstalled`) captura o mesmo vetor via ciclo e2e completo (`init → install-global → update → validate`); e2e ArmB (`TestD15_GlobalOnlyDoesNotSatisfyProjectScope`) captura a regressão de validador.
- Idempotência e isolamento de HOME mantidos.
- ML-5B (sibling expansion do barrier) funciona corretamente para os casos esperados.

**Decisão: APROVA.**
