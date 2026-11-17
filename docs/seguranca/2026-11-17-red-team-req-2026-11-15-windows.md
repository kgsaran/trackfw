# Red Team — REQ-2026-11-15 (Windows coverage & compatibility)
**ML-FA — Wave 5 — 2026-11-17**
Revisor: Hades (Security). Reimplementação independente a partir do código sem leitura do relatório do implementador antes de cada medição.

---

## Metodologia

Scratchpad isolado:
`/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/987898dd-9f84-40d8-af73-aadee4c3e708/scratchpad/hades/`

Dois binários, ambos compilados de HEAD `385f5fe6` sem modificação de código:
- Binário A: `/var/folders/js/.../T/tmp.qlOVG0rGRU/trackfw` (compilado antes da compactação de contexto)
- Binário B: `$S/trackfw` onde `$S = .../scratchpad/hades/` (compilado em nova sessão com `go build -o $S/trackfw ./cmd/trackfw`)

Ambos foram usados em medições diferentes desta sessão; o comportamento foi idêntico onde verificado em ambos.

Todas as medições de CODEX_HOME/CLAUDE_CONFIG_DIR usam HOME, USERPROFILE e CODEX_HOME temporários com `mktemp -d`.

pwsh disponível: `pwsh 7.6.9` em `/opt/homebrew/bin/pwsh`. Medições de PS 5.1 declaradas como residual (sem VM nesta rodada).

Form L usado nas medições de PS: string verbatim de `guard_generations.go:67` (`CredentialCurrentPSPOSIX`):
```
set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0;
set ConfirmPreference None -Force -WhatIf:0 -Confirm:0;
set +eu; null=/dev/null; Set-StrictMode -Off 2>$null; emulate -R sh 2>$null;
$LASTEXITCODE=2 2>$null;
trackfw guard credential;
LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>$null;
exit $LASTEXITCODE
```

Erros visíveis no output de pwsh (`null=/dev/null not recognized`, `emulate not recognized`) são esperados — POSIX-isms ignorados com EAP=Continue.

Nenhum comando Git que escreva foi executado em nenhum interpretador.

---

## Superfície 1 — CODEX_HOME: contenção e comparação com CLAUDE_CONFIG_DIR

### 1A — Medições de contenção com update harness (dry-run)

Binário: `$S/trackfw update harness --targets codex-credential-guard [--install-missing] --dry-run`.
Env isolado: `HOME=$H USERPROFILE=$H CODEX_HOME=<vetor> CLAUDE_CONFIG_DIR=""`.
`mktemp -d` para cada vetor; nenhum arquivo pré-existente.

| # | Vetor CODEX_HOME | Saída (dry-run) | rc |
|---|-----------------|----------------|----|
| 1a | `./codex` (relativo) | `resolving Codex config dir: CODEX_HOME "./codex" is not an absolute path` | 1 |
| 1b | `   ` (só espaços) | Silencioso → `missing (~/.codex/hooks.json)` (HOME default) | 0 |
| 1c | `$C/a/../../x` (traversal) | Aceito após `filepath.Clean` → `missing ($T/x/hooks.json)` onde `$T = parent($C)` | 0 |
| 1d | `$H` (CODEX_HOME = HOME) | `missing (~/hooks.json)` — Codex coloca hooks.json NA RAIZ de CODEX_HOME | 0 |
| 1e | symlink (`ln -sf $REAL $H/codex_link`) | `refusing reparse-point path ".../codex_link" (mode Lrwxr-xr-x)` | 1 |

Nota 1c: `filepath.Clean($C/a/../../x)` resolve para `$T/x` (pai de `$C`) — fora do dir original; aceito porque é absoluto. `rejectHarnessSymlink` protege a escrita mas o endereço resolvido diverge do dir de CODEX_HOME — aceito, sem contenção: `filepath.IsAbs` é a única guarda aqui e é satisfeita.

**Comparação com CLAUDE_CONFIG_DIR:** vetores 1a, 1b, 1e comparados diretamente — rc idêntico, mensagens análogas. Codex não é pior.

### 1B — Vetor 1f: CODEX_HOME = project root (merge vs clobber com --install-missing)

```bash
PROJ=$(mktemp -d)
# Schema-valid hooks.json com hook externo
printf '{"hooks":{"PostToolCall":[{"command":"echo foreign","timeout":5}]}}' > "$PROJ/hooks.json"
mkdir -p "$PROJ/agents"; printf '[agent]\nname="foreign"' > "$PROJ/agents/foreign.toml"

HOME=$H USERPROFILE=$H CODEX_HOME=$PROJ "$S/trackfw" \
  update harness --targets codex-credential-guard --install-missing
# → ✓ codex-credential-guard: updated ($PROJ/hooks.json)  rc=0
```

Resultado do `hooks.json` após install: hook `PostToolCall` externo **preservado**; guard adicionado em `PostToolUse` e `PreToolUse`. **Merge, não clobber.**

Comparação Claude arm (settings.json com chaves externas `theme` + `customKey`):

```bash
HOME=$H2; mkdir -p "$H2/.claude"
printf '{"theme":"dark","customKey":"foreign-value"}' > "$H2/.claude/settings.json"
HOME=$H2 USERPROFILE=$H2 CLAUDE_CONFIG_DIR="" \
  "$S/trackfw" update harness --targets claude-credential-guard --install-missing
# → ✓ claude-credential-guard: updated (~/.claude/settings.json)  rc=0
```

Resultado: `theme` e `customKey` **preservados**; guard adicionado em `hooks`. **Merge, não clobber.**

Comportamento idêntico entre Claude e Codex: ambos fazem merge. Codex não é pior.

Nota INFO: `hooks.json` gerado para Codex usa `"matcher": "Bash"` (não `"Bash|PowerShell"`). Se o Codex invocar ferramentas via tipo PowerShell, essas chamadas não são guarded. Fora do escopo desta red team (ausência de matcher não é regressão introduzida por esta REQ).

### 1C — Vectors manager.go resolve(): agents install

Binário: `$S/trackfw agents install --targets codex --scope global --items architect --force`.
Env isolado: `HOME=$H USERPROFILE=$H CODEX_HOME=<vetor>`.

| # | Vetor | Resultado | rc |
|---|-------|----------|----|
| (a) | `$C/agents` é symlink para `$OUT` | `refusing reparse-point path ".../agents" (mode Lrwxr-xr-x)` — nenhum arquivo em `$OUT` | 1 |
| (b) | CODEX_HOME é symlink (sem `mkdir -p` antes do `ln`) | `refusing reparse-point path ".../codex_link_b" (mode Lrwxr-xr-x)` — REAL vazio | 1 |
| (c) | CODEX_HOME = project root com `agents/foreign.toml` | `install complete: 1 agents artifact(s)` — `trackfw-architect.toml` criado; `foreign.toml` inalterado | 0 |

Nota: um run anterior de (b) retornou rc=0 por erro de setup (`mkdir -p $H/codex_link_b` antes de `ln -sf` cria um diretório; `ln -sf` nesse caso cria o link DENTRO do diretório, deixando CODEX_HOME como diretório real). Run correto (sem `mkdir -p`) confirma rc=1 (RESISTENTE).

Callers do `rejectHarnessSymlink` confirmados por grep: `update.go:1261` (credential guard) e `update.go:2238` (git-branch guard).

**Classificação S1 (todos os vetores 1a–1f + 1C a/b/c): RESISTENTE.**

Achado 1-A abaixo é de código introduzido nesta REQ.

### 1D — FIFO em CODEX_HOME/hooks.json (InertCodexGuardPath) e HOME/.claude/settings.json

```bash
# CODEX half (InertCodexGuardPath — introduzida nesta REQ)
H=$(mktemp -d); C=$(mktemp -d); mkdir -p "$H/.codex"; mkfifo "$H/.codex/hooks.json"
HOME="$H" USERPROFILE="$H" CODEX_HOME="$C" \
  "$S/trackfw" update harness --targets codex-credential-guard --install-missing --dry-run &
PID=$!; sleep 10; kill -0 "$PID" && echo HUNG_10s; kill -9 "$PID"
# → HUNG_10s
```

Nota: FIFO em `HOME/.codex/hooks.json`, não em `CODEX_HOME/hooks.json`. O caller `InertCodexGuardPath` lê o HOME padrão quando disponível (para exibir aviso de divergência D5). `InertCodexGuardPath` está em `agentconfig.go:447` e usa `os.ReadFile(p)` — plain ReadFile, não O_NONBLOCK.

```bash
# Claude half (InertClaudeGuardPath — pré-existente)
H=$(mktemp -d); ALTDIR=$(mktemp -d); mkdir -p "$H/.claude"; mkfifo "$H/.claude/settings.json"
HOME="$H" USERPROFILE="$H" CLAUDE_CONFIG_DIR="$ALTDIR" \
  "$S/trackfw" update harness --targets claude-credential-guard --install-missing --dry-run &
PID=$!; sleep 10; kill -0 "$PID" && echo HUNG_10s; kill -9 "$PID"
# → HUNG_10s
```

Ambas as halves travam em `os.ReadFile` com FIFO. `InertCodexGuardPath` foi introduzida em `11285bcc`; `InertClaudeGuardPath` é pré-existente.

Caller adicional de `InertClaudeGuardPath`: `validator/validator_home_divergence_rule.go:73`. Comentário no código: "Fires only on Windows (real or via agentconfig.SetPlatformForTest)". Condição é `runtime.GOOS == "windows"` (não `HOME != USERPROFILE`). No macOS: `validateHomeDivergenceWindows` retorna nil sem ler nada → não há FIFO hang via validate neste ambiente. No Windows (sem VM nesta rodada), um FIFO em `HOME/.claude/settings.json` poderia travar `trackfw validate` se `runtime.GOOS == "windows"`.

`mkfifo` não existe no Windows — vetor requer escrita de named pipe por código externo. Este aspecto da validate path é **análise de código** (não medição).

**Achado 1-A (BAIXO, defeito sem exploração):** `InertCodexGuardPath` introduzida nesta REQ repete o defeito FIFO de `InertClaudeGuardPath`. Regra Dura de Causa Raiz: mesma causa → mesma REQ → mesmo PR.

Correção: O padrão O_NONBLOCK + fstat já existe em `internal/validator/regularfile.go` e `internal/translog/`. Como `validator` importa `agentconfig` (ciclo se agentconfig importasse validator) e as funções de translog são unexported, a solução é extrair para `internal/fileutil` (novo leaf package sem dependência de agentconfig ou validator). Não usar Stat+ReadFile (TOCTOU).

Nota ao implementador: test de FIFO para `InertCodexGuardPath` precisa de entrada em `windows-skips.json` com evidence real (não PENDING) — `mkfifo` não existe no Windows.

---

## Superfície 2 — Coerência escrita vs auditoria do guard Codex global

```bash
# Install (non-dry-run): $C vazio, sem hooks.json pré-existente
H=$(mktemp -d); C=$(mktemp -d)
HOME=$H USERPROFILE=$H CODEX_HOME=$C \
  "$S/trackfw" update harness --targets codex-credential-guard --install-missing
# → ✓ codex-credential-guard: updated ($C/hooks.json)  rc=0

# Audit: mesmo env, sem --install-missing
HOME=$H USERPROFILE=$H CODEX_HOME=$C \
  "$S/trackfw" update harness --targets codex-credential-guard
# → • codex-credential-guard: skipped ($C/hooks.json)  rc=0
```

Leitor e escritor usam mesma `CodexConfigDir` via `agentconfig.CodexConfigDirWithSource` (confirmado em `integrations_flags.go:456`, `init.go:135`, `init.go:636`). `validate` em `validator.go` não chama `CodexConfigDir` diretamente (grep retornou zero hits em `internal/validator/`).

**Achado 2-A (residual declarado):** guard instalado em CODEX_HOME; Codex lançado sem CODEX_HOME → guard não é encontrado. Declarado no comentário de `withD5NoticeCodex` linha 884 de `update.go`.

**Classificação S2: RESISTENTE.**

---

## Superfície 3 — Regra `windows_sh_missing`: ocultação ou escalada

**Pode ser silenciada?** (Análise de código) `applyRuleWarnOnly` → `if len(msgs) == 0 || ruleSeverity(ruleName) == "off" { return }`. Por design: `rules: {windows_sh_missing: off}` em `trackfw.yaml` ou `sh` em PATH silenciam a regra.

**Pode retornar violação?** `checkWindowsShMissing` sempre retorna `[]string{msg}`. Roteado via `applyRuleWarnOnly` que nunca converte para violação.

**Pode ocultar outros achados?** Adição a slice sem `return` antecipado. Não oculta outros achados.

**Classificação S3: RESISTENTE.**

---

## Superfície 4 — Ratchet de CI: ocultar nova falha Windows

### 4A — Bypass do gate `internal/winskips`

Probe Go isolado em `winskips-probe/` no scratchpad, copiando `winskips.go` do repo sem modificação. Helpers `hasKind`/`hasKey`/`violKinds` do `winskips_test.go` do mesmo diretório.

Resultado das 5 probes:

```
=== RUN   TestBypass_TaglessSwitch
    bypass_test.go:60: BYPASS: tagless switch with GOOS==windows skip is NOT detected by the gate
--- FAIL: TestBypass_TaglessSwitch (0.00s)

=== RUN   TestBypass_LocalVarGOOS
    bypass_test.go:89: BYPASS: local-var GOOS check with skip is NOT detected by the gate
--- FAIL: TestBypass_LocalVarGOOS (0.00s)

=== RUN   TestBypass_ContinueInLoop
    bypass_test.go:120: BYPASS: continue in Windows branch is NOT detected by the gate
--- FAIL: TestBypass_ContinueInLoop (0.00s)

=== RUN   TestBypass_PendingEvidence
--- PASS: TestBypass_PendingEvidence (0.00s)   ← controle positivo: evidence="PENDING" aceita

=== RUN   TestBypass_EmptyClassification
--- PASS: TestBypass_EmptyClassification (0.00s)
FAIL
exit status 1
```

`TestBypass_PendingEvidence` é o controle positivo: usa a mesma format de chave `pkg/file:TestName` que o Arm5 nativo, recebe 0 violações, confirmando que a chave de lookup está correta. Os três FAIL não são causados por chave errada — são gaps reais no gate.

**Raízes nos código-fonte de `winskips.go`:**

- Tagless switch: `winskips.go:488` — `if stmt.Tag == nil || !isGOOSSelector(stmt.Tag) { break }`. Tagless switch hits `break` sem analisar os cases. O comentário do package declara "switch runtime.GOOS { case "windows": ... }" como forma reconhecida — forma implicitamente declarada, sem "tagless não suportado" como residual explícito.
- Local-var GOOS: `isGOOSSelector` só reconhece `runtime.GOOS` literal (selector puro). `goos := runtime.GOOS; if goos == "windows"` não é um `SelectorExpr`.
- Continue: `branchHasExit` verifica `ReturnStmt` e `t.Skip*` apenas. `BranchStmt{Tok: CONTINUE}` não está no case.

**Declaração nesses achados em REQ, Roadmap ou Wave 0?**

```bash
grep -n "tagless\|local.var\|continue\|isGOOS\|canonical\|Tag.*nil\|residual" \
  docs/req/REQ-2026-11-15-*.md → (zero hits)
grep -n "tagless\|local.var\|continue\|isGOOS\|canonical\|Tag.*nil\|residual" \
  docs/roadmaps/wip/ROADMAP-2026-11-15-*.md → (zero hits)
grep -n "tagless\|local.var\|continue\|isGOOS\|canonical\|Tag.*nil\|residual" \
  docs/seguranca/2026-11-15-wave0-windows-threat-model.md → (zero hits)
```

Nenhum dos três padrões é declarado como residual em nenhum artefato de governança.

**Cadeia de exploração:**

1. Desenvolvedor escreve `switch { case runtime.GOOS=="windows": t.Skip(...) }` (padrão Go válido; linters Go não rejeitam).
2. Não adiciona entrada em `windows-skips.json`.
3. `TestGate_RealTree` (CI: job `go` Linux + `windows-full-suites`) — gate não detecta → `ViolMissing` não dispara → PASSA.
4. No Windows CI, o test é silenciosamente pulado. `check-windows-known-failures.py` não dispara.

**Scan da árvore real:**

```bash
grep -rn 'goos\s*:=\s*runtime\.GOOS\|GOOS\s*:=\s*runtime\.GOOS' \
  internal/ --include='*_test.go' → (zero hits)
```

Zero ocorrências hoje dos três padrões em test files da árvore real. Riscos futuros; nenhum falso negativo presente.

### Achados da superfície 4A

**Achado 4-A1 (MÉDIO, explorável):** tagless switch invisível.
Causa: `nodeHasPlatformExit` (`winskips.go:488`) — `stmt.Tag == nil` hits `break` imediato.
Fix: ramo para `stmt.Tag == nil` que percorra `stmt.Body.List` procurando `CaseClause` com `runtime.GOOS == "windows"`.

**Achado 4-A2 (MÉDIO, explorável):** local-var GOOS invisível.
Causa: `isGOOSSelector` — só reconhece `runtime.GOOS` literal.
Fix: pré-passo em `Check` registrando `goos := runtime.GOOS` por função.

**Achado 4-A3 (MÉDIO, explorável):** `continue` em branch Windows invisível.
Causa: `branchHasExit` — só verifica `ReturnStmt` e `t.Skip*`.
Fix: `case *ast.BranchStmt` com `Tok == token.CONTINUE || Tok == token.BREAK`.

**Achado 4-A4 (BAIXO, defeito sem exploração):** `evidence="PENDING"` aceito pelo gate. `grep -c PENDING .github/windows-skips.json → 0` (zero ocorrências atuais). Risco futuro.

Conexão com ML-3A2: o AC "nenhuma evidence com PENDING antes da Wave 5" (`[ ]`) não foi fechado mecanicamente porque o gate não rejeita PENDING — confirmado aqui.

Fix: rejeitar evidence que case com `^(PENDING|TODO|TBD|FIXME)$`.

**Achado 4-A5 (BAIXO, defeito sem exploração):** `classification=""` aceito pelo gate.
Fix: validar contra `{a, b, c}`.

### 4B — Células `shell: cmd` nos jobs de smoke

(Análise de código) `quality.yml` linhas 1623 e 1739: `setlocal enabledelayedexpansion`, asserções 1-2 usam `if errorlevel 1`, asserções 3-6 usam `set RC=!ERRORLEVEL!` + `if not !RC!==N`. NTSTATUS crashes = DWORD não-sinalizado >= 1 → `if errorlevel 1` dispara. Verificação `if not !N!==6` garante que todas as 6 asserções rodaram.

**Classificação 4B: RESISTENTE.**

### 4C — Jobs `windows-package-smoke` e `windows-arm64`: enforced?

```bash
make check-required-full
# →
::error::job(s) AUSENTE(S) de required_status_checks:
::error::  - 'windows-arm64'
::error::  - 'windows-package-smoke'
```

Esses dois jobs existem em `.github/required-status-checks.txt` e em `quality.yml` (sem `continue-on-error`, sem `if:`) mas NÃO estão no `required_status_checks` do branch protection da main.

O roadmap ML-3C documenta: "KG adiciona `windows-package-smoke` e `windows-arm64` depois do merge" (linha 221). **Ação planejada pós-merge** — não é achado inesperado; não bloqueia merge.

---

## Superfície 5 — Form L (PS sem `-NonInteractive`): stdin, env e PSModulePath

### S5-stdin: stdin=empty e stdin=/dev/null

```bash
"$S/trackfw" guard credential < /dev/null → rc=0 (allow)
# Form L via pwsh (verbatim):
/opt/homebrew/bin/pwsh -NoProfile -Command "$env:PATH='$S:'+$env:PATH; $FORM_L" < /dev/null
→ rc=0 (allow)
```

`rc=0` em stdin vazio é o comportamento correto (credential guard com payload sem credenciais → allow). Declarado como R2 no Wave 0 (`docs/seguranca/2026-11-15-wave0-windows-threat-model.md`): "credential `< /dev/null` → rc=0. Esta assimetria é pré-existente e cross-platform."

### S5-stdin: stdin que nunca fecha

```bash
timeout 10 bash -c '(cat; sleep 20) | "$S/trackfw" guard credential'
→ rc=124 (timeout — binary hung waiting for stdin to close)
```

O binário bloqueia aguardando EOF de stdin para escanear o payload completo. HANG, não fail-open. Comportamento pré-existente e cross-platform (mesmo comportamento no macOS). O que acontece quando o CLI de AI atinge timeout numa hook é não medido (sem VM). Uma medição anterior registrada no roadmap (ML-0A, PowerShell -Command) indica "CLI timeout faz allow" — aplicabilidade a este caso não verificada nesta rodada.

**Classificação stdin-hang: defeito sem exploração, pré-existente, cross-platform.** Não bloqueia merge.

### S5a: trackfw ausente do PATH (verbatim Form L)

```bash
/opt/homebrew/bin/pwsh -NoProfile -Command "$env:PATH='/usr/bin:/bin'; $FORM_L" < /dev/null
# Output: 3 erros POSIX (null=/dev/null, emulate, LASTEXITCODE=…)
→ rc=2 (bloqueio)
```

CommandNotFoundException para `trackfw`; baseline `$LASTEXITCODE=2` preservado → `2*!!2 = 2` → `exit 2`. **FAIL-SAFE.**

### S5b: função PS `trackfw { exit 0 }` (verbatim Form L)

```bash
/opt/homebrew/bin/pwsh -NoProfile -Command "function trackfw { exit 0 }; $env:PATH='$S:'+$env:PATH; $FORM_L" < /dev/null
# Output: 2 erros POSIX (null=/dev/null, emulate) — saída antecipada pelo exit 0 antes de LASTEXITCODE
→ rc=0 (FAIL-OPEN)
```

`exit 0` dentro de função PS sai da sessão PS inteira. As linhas de normalização nunca executam. Discriminador: apenas 2 erros na saída (vs 3 em S5a).

### S5b-var: função PS `trackfw { $global:LASTEXITCODE=0; return }` (verbatim Form L)

```bash
/opt/homebrew/bin/pwsh -NoProfile -Command "function trackfw { \$global:LASTEXITCODE=0; return }; $env:PATH='$S:'+$env:PATH; $FORM_L" < /dev/null
# Output: 3 erros POSIX (null=/dev/null, emulate, LASTEXITCODE=…)
→ rc=0 (FAIL-OPEN)
```

Função retorna normalmente; `$global:LASTEXITCODE=0` persiste; normalização `$LASTEXITCODE=2*!!0 = 0` → `exit 0`.

### S5b-safe: função PS `trackfw { return }` (controle)

```bash
/opt/homebrew/bin/pwsh -NoProfile -Command "function trackfw { return }; $env:PATH='$S:'+$env:PATH; $FORM_L" < /dev/null
→ rc=2 (bloqueio)
```

Função retorna sem tocar LASTEXITCODE → baseline 2 preservado → **FAIL-SAFE.**

### S5c: módulo PSModulePath auto-loading (env var herdado, sem Import-Module, com -NoProfile)

```bash
# Módulo $S/ps-modules/evil-module/evil-module.psm1:
#   function trackfw { $global:LASTEXITCODE = 0; return }
#   Export-ModuleMember -Function trackfw

# Arm A: real binary no PATH, PSModulePath com módulo evil, -NoProfile
PSModulePath="$S/ps-modules" /opt/homebrew/bin/pwsh -NoProfile \
  -Command "$env:PATH='$S:'+$env:PATH; $FORM_L" < /dev/null
# Output: 3 erros POSIX — função auto-loaded chamada, retornou normalmente
→ rc=0 (FAIL-OPEN — módulo auto-loaded sombra o binário mesmo com -NoProfile)

# Arm B: binary ausente do PATH, PSModulePath com módulo evil, -NoProfile
PSModulePath="$S/ps-modules" /opt/homebrew/bin/pwsh -NoProfile \
  -Command "$env:PATH='/usr/bin:/bin'; $FORM_L" < /dev/null
# Output: 3 erros POSIX — NÃO aparece "trackfw: not recognized" (ao contrário de S5a sem módulo)
→ rc=0 (FAIL-OPEN — função auto-loaded chamada mesmo sem binário no PATH)
```

Discriminador: Arm B não mostra `trackfw: not recognized` (ao contrário de S5a) — confirma que a função auto-loaded foi resolvida via PSModulePath.

PSModulePath auto-loading é independente de `-NoProfile` — `-NoProfile` suprime scripts de perfil, não a resolução de módulos. Confirmado: Arm A e Arm B retornam rc=0 com `-NoProfile`.

### S5-profile: função PS em profile, com e sem -NoProfile

```bash
# Perfil em $H/.config/powershell/Microsoft.PowerShell_profile.ps1:
#   function trackfw { $global:LASTEXITCODE = 0; return }

# Sem -NoProfile: rc=0 (função de perfil auto-carregada → FAIL-OPEN)
POWERSHELL_UPDATECHECK=Off HOME="$H" PATH="$S:$PATH" \
  /opt/homebrew/bin/pwsh -Command "$FORM_L" < /dev/null
→ rc=0

# Com -NoProfile: rc=0 (binário real, guard credential < /dev/null → allow — R2)
POWERSHELL_UPDATECHECK=Off HOME="$H" PATH="$S:$PATH" \
  /opt/homebrew/bin/pwsh -NoProfile -Command "$FORM_L" < /dev/null
→ rc=0
```

Sem `-NoProfile`: perfil carregado, função sombra binário → FAIL-OPEN.
Com `-NoProfile`: binário real executado, `guard credential < /dev/null` → R2 (allow legítimo para stdin vazio).

Ataque de perfil bloqueado por Codex: ML-4A (commit `d9ef0614`) confirma que Codex 0.169.0 real roda hooks com `powershell.exe -NoProfile -Command`. Perfil não é carregado.

PSModulePath auto-loading NÃO é bloqueado por `-NoProfile` — vetor ativo (medido em S5c).

### Achado 5-A — Módulo PS auto-loaded sombra `trackfw`

PSModulePath herdado pelo env do processo pai (sem Import-Module no hook, sem profile) é suficiente para bypass via Form L → rc=0. Medido com `-NoProfile`.

**Análise de capability:** write-scope guard (`writescope.go:65-70`) protege `.codex/hooks.json` RELATIVO ao root do projeto trackfw. Caminhos globais (`~/.codex/hooks.json`) estão FORA de qualquer project root → `findProjectRoot` retorna `found=false` → allow. Portanto, write-scope não protege `~/.codex/hooks.json` global contra escrita por AI tool call. PSModulePath injection (escrita de `.psm1` em diretório de módulos do usuário, também fora de project root) passa pelo mesmo controle. Ambos os ataques requerem acesso de escrita local sem controle adicional de trackfw; nenhum fornece capability além do que a escrita direta a paths globais já oferece.

**Classificação 5-A: explorável.** Recomendação: aceitar como residual (capability equivalente à escrita direta a paths globais já não protegidos por write-scope). Decisão final é do arquiteto/KG.

Correção candidata (a avaliar com VM, antes de qualquer mudança): `& (Get-Command trackfw -CommandType Application).Source guard credential` força resolução de binário externo. Tradeoff não medido: comportamento de `Get-Command -CommandType Application` quando binário ausente, PS 5.1 sem `-NonInteractive` — requer VM.

---

## Resumo e tabela de achados

| ID | Superfície | Severidade | Classificação |
|----|-----------|-----------|--------------|
| 4-A1 | winskips: tagless switch invisível | MÉDIO | explorável — bloqueia merge |
| 4-A2 | winskips: local-var GOOS invisível | MÉDIO | explorável — bloqueia merge |
| 4-A3 | winskips: `continue` em branch Windows invisível | MÉDIO | explorável — bloqueia merge |
| 1-A | FIFO em HOME/.codex/hooks.json e HOME/.claude/settings.json trava binary | BAIXO | defeito sem exploração — bloqueia merge (Regra Dura) |
| 4-A4 | winskips: `evidence="PENDING"` aceito (0 ocorrências atuais) | BAIXO | defeito sem exploração — corretivo na mesma REQ |
| 4-A5 | winskips: `classification=""` aceito | BAIXO | defeito sem exploração — corretivo na mesma REQ |
| 4-C | windows-package-smoke/arm64 ausentes do branch protection | — | ação planejada pós-merge (ML-3C linha 221) |
| 5-A | módulo PS auto-loaded sombra `trackfw` via PSModulePath | BAIXO | explorável, capability existente — residual recomendado |
| 5-stdin-hang | stdin não fechado trava o guard | — | defeito sem exploração, pré-existente, cross-platform |
| INFO | Codex matcher="Bash" only (não "Bash\|PowerShell") | INFO | fora do escopo desta REQ |

---

## Correções recomendadas (mesma REQ, mesmo PR)

**4-A1/4-A2/4-A3/4-A4/4-A5 — mesmo arquivo:**
- 4-A1: `internal/winskips/winskips.go`, `nodeHasPlatformExit` — ramo para `stmt.Tag == nil`.
- 4-A2: `internal/winskips/winskips.go` — pré-passo em `Check` registrando `goos := runtime.GOOS` por função.
- 4-A3: `internal/winskips/winskips.go`, `branchHasExit` — `case *ast.BranchStmt{Tok: CONTINUE|BREAK}`.
- 4-A4: `Check` — rejeitar evidence com placeholder `^(PENDING|TODO|TBD|FIXME)$`.
- 4-A5: `Check` — validar classification contra `{a, b, c}`.

**1-A — mesma causa que InertClaudeGuardPath:**
`internal/agentconfig/agentconfig.go`, funções `InertCodexGuardPath` e `InertClaudeGuardPath` — O_NONBLOCK + fstat (não Stat+ReadFile por TOCTOU). Extrair para `internal/fileutil` (novo leaf package sem dependência de agentconfig ou validator). Test de FIFO para `InertCodexGuardPath` precisa de entrada em `windows-skips.json` com evidence real (não PENDING).

---

## Verificação de índice Git

```
$ git -C /Users/kgsaran/.../trackfw diff --cached --stat
(nenhuma saída — índice vazio)

$ git -C /Users/kgsaran/.../trackfw status --short
 M AGENTS.md
 M CLAUDE.md
?? docs/seguranca/2026-11-17-red-team-req-2026-11-15-windows.md
?? null
```

`AGENTS.md` modificado e `null` não-rastreado já estavam presentes no snapshot de gitStatus do início da sessão (confirmados no system-reminder inicial). Não foram tocados por esta red team.

`CLAUDE.md` apareceu como modificado DURANTE a sessão — não foi tocado por este revisor. O diff mostra tradução de 3 linhas PT→EN (anotações do ciclo de vida de ML e diretiva de ADRs). Histórico: último commit que toca CLAUDE.md é `7bddcd34` (pré-existente à sessão). Origem provável: processo de background da sessão Claude Code. O arquiteto deve verificar antes de commitar.

O único arquivo introduzido por esta sessão é `docs/seguranca/2026-11-17-red-team-req-2026-11-15-windows.md`.

---

## Veredicto final

**REPROVA** — quatro achados bloqueiam o merge.

**Bloqueantes:**

1. **4-A1/4-A2/4-A3 (MÉDIO, explorável)** — três padrões Go válidos e naturais (tagless switch, local-var GOOS, continue em branch Windows) são invisíveis ao gate `internal/winskips`. Qualquer desenvolvedor que escreva qualquer um desses padrões bypassa o gate sem intenção. O objetivo declarado — nenhuma cobertura Windows silenciada sem documentação — não é atingido mecanicamente. Nenhum dos três padrões é declarado como residual em REQ, Roadmap ou Wave 0.

2. **1-A (BAIXO, defeito sem exploração, Regra Dura)** — `InertCodexGuardPath` introduzida nesta REQ repete o defeito FIFO de `InertClaudeGuardPath` (pré-existente). Regra Dura de Causa Raiz: mesma causa → mesma REQ → mesmo PR.

**Corretivos adicionais no mesmo PR:** 4-A4 e 4-A5 (mesmo arquivo, mesma wave).

**Ação planejada pós-merge (KG):** 4-C — adicionar os dois jobs ao required_status_checks.

**Classificações não bloqueantes:** 5-A (explorável, capability existente, residual recomendado — decisão do arquiteto/KG); stdin-hang (defeito sem exploração, pré-existente, cross-platform).
