# Red-Team — Wave 2, ML-2A: guard global segue o home que o agente lê

> Data: 2026-10-14 | Revisor: hades-tf | Branch: fix/guard-global-segue-o-home-que-o-agente-le
> REQ: REQ-2026-09-01 | ADR: ADR-2026-10-14 | HEAD: 1b02fb03

---

## Escopo

Reimplementação independente a partir da leitura do diff, seguida de testes descartáveis em cópia
da árvore (`rsync -a --exclude .git`). Binary compilado em `mktemp` separado. Nenhum teste escreve
em `HOME`, `USERPROFILE` ou `CLAUDE_CONFIG_DIR` reais — verificado com diretórios sentinela (0
arquivos ao final). Normativo: ADR-2026-10-14 (D1–D6, A1–A8), Wave 0
(`docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md`) e revisão de qualidade do
hefesto-tf (`docs/qualidade/2026-10-14-revisao-guard-global-home-por-cli.md`).

Alvos: `internal/agentconfig/`, `Manager.ClaudeDir` (`internal/integrations/manager.go`),
`harness*TargetClaude`, `harnessClaudeSkillTarget`, `withD5Notice` (`internal/generators/update.go`),
`installGlobalSkillInner` (`internal/generators/scaffold.go`), regras do validador.

---

## Alvo 1 — `CLAUDE_CONFIG_DIR` como vetor de escrita

### Testes executados na cópia (cópia descartável + `pathguard.RejectAndReport` + binary)

| Valor | `claudeDir` pós-Clean | guardRoot | guard | resultado |
|-------|----------------------|-----------|-------|-----------|
| `""` (vazio) | fallback → HOME/.claude | HOME | n/a | caminho correto |
| `..` | `..` | `.` | **REJEITA** — path escapes root | fechado |
| `../evil` | `../evil` | `..` | **PASSA** | **R-1** |
| `../../evil` | `../../evil` | `../..` | **PASSA** | **R-1** |
| `relative/path` | `relative/path` | `relative` | **PASSA** | **R-1** |
| `/../etc` | `/etc` | `/` | **REJEITA** no macOS (`/etc` é symlink) | fechado no macOS |
| `/tmp/claude-rt` | `/tmp/claude-rt` | `/tmp` | **REJEITA** no macOS (`/tmp` é symlink) | fechado no macOS |
| `link/a/b/claude-cfg` (link=simlink acima do root) | mesmo | `link/a/b` | **PASSA** — simlink acima do root | **R-2** |

**Raíz do R-1 (ajuste):** `ClaudeConfigDir` retorna `filepath.Clean(dir)`. `filepath.Clean` não
converte caminhos relativos em absolutos. Com `dir = "../evil"`, `claudeDir = "../evil"`. O guard
root vira `filepath.Dir("../evil") = ".."`. `Beneath("..", "../evil/settings.json")` chama
`filepath.Rel("..", "../evil/settings.json")` = `"evil/settings.json"` — não começa com `..` →
retorna true. Guard passa. Escrita vai para `<CWD>/../evil/settings.json`.

Por que `..` sozinho é rejeitado mas `../evil` não: com `..`, o root é `"."` e o target é
`"../settings.json"`. `Beneath(".", "../settings.json")` = `filepath.Rel(".", "../settings.json")` =
`"../settings.json"` → começa com `..` → false → escapa root → guard rejeita. Com `../evil`, o root
é `".."` e `"../evil/settings.json"` está DENTRO de `".."` — um nível acima do CWD, mas dentro
do root declarado.

Correção proposta (R-1): `internal/agentconfig/agentconfig.go:257` — rejeitar CLAUDE_CONFIG_DIR
não-absoluto como erro A7, em vez de aceitar silenciosamente:
```go
if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
    if !filepath.IsAbs(filepath.Clean(dir)) {
        return "", fmt.Errorf("CLAUDE_CONFIG_DIR %q is not an absolute path", dir)
    }
    return filepath.Clean(dir), nil
}
```
Isso converte o caso relativo em A7-failure (emite finding, sem escrita silenciosa), e também rejeita
strings de apenas espaços (`filepath.Clean("   ")` = `"   "`, que não é absoluto).

**Raíz do R-2 (resíduo):** o guard root é `filepath.Dir(claudeDir)`. Um simlink em qualquer
ancestral ACIMA de `filepath.Dir(claudeDir)` não é verificado. Medido: `CLAUDE_CONFIG_DIR=link/a/b/claude-cfg`
onde `link → real` — a escrita ocorre em `real/a/b/claude-cfg/settings.json` sem que o guard
detecte `link`. Este é o mesmo residual T5 do pathguard que existe desde a versão original: o root
jamais foi EvalSymlinks-verificado em `rejectHarnessSymlink`. A diferença é que o root era HOME
(fixo) e agora é `parent(CLAUDE_CONFIG_DIR)` (variável por env var). Pré-existente na classe,
novo no ponto de entrada. Declarar no ADR.

**Contenção por design (ADR A1/D6):** `CLAUDE_CONFIG_DIR` absoluto fora de HOME aceita escrita
fora de HOME. No macOS, a maioria dos caminhos externos a HOME passa por simlinks (`/tmp`, `/var`,
`/etc`), que são rejeitados pelo guard. No Linux esses simlinks não existem — `CLAUDE_CONFIG_DIR=/tmp/x`
escreve em `/tmp/x/settings.json`. Comportamento intencional; documentar limite de plataforma no ADR.

---

## Alvo 2 — Falso positivo de saúde

### 2a — `withD5Notice` declara guard ativo como "inert" (BLOQUEANTE — R-3)

**Medido com o binary da cópia:**

```
# Setup: instalar guard normalmente em H/.claude/settings.json
HOME="$H" USERPROFILE="$H" ./trackfw update harness --install-missing
# → claude-credential-guard: updated (~/.claude/settings.json)

# Rodar novamente com CLAUDE_CONFIG_DIR diferente
HOME="$H" USERPROFILE="$H" CLAUDE_CONFIG_DIR="$H/cc-custom" ./trackfw update harness --install-missing
# → ✓ claude-credential-guard: updated (~/cc-custom/settings.json)
#       - old guard under HOME is inert — Claude Code does not read
#         H/.claude/settings.json; remove it by hand if you want     ← ERRADO
```

`withD5Notice` chama `InertClaudeGuardPath(claudeDir)` que usa `homedir.Dir()` para o "old guard"
e não tem gate de SO. No macOS (e no Linux), quando CLAUDE_CONFIG_DIR está definido só no processo
do trackfw mas não no processo do Claude Code, o arquivo `H/.claude/settings.json` **é o guard
ativo**. O trackfw mente ao dizer que Claude Code não lê esse arquivo.

O ADR D5 destina-se ao cenário Windows com HOME≠USERPROFILE: o "old guard" inerte é aquele gravado
em HOME quando o agente lê de USERPROFILE. `withD5Notice` está implementado sem o gate
`agentconfig.HomeDivergencePaths()` e dispara em qualquer SO sempre que `claudeDir ≠ HOME/.claude`.

O vazamento de D5 para o harness também replica o B1 do hefesto em outra superfície: a mensagem
na saída do `update harness` instrui o usuário a apagar o guard ativo. A diferença com o B1 é que
o B1 era no `validate` (mesma sessão, mesma env var), enquanto aqui a divergência vem de envs de
processo distintos — tornando o falso positivo ainda mais difícil de detectar.

**Correção proposta:** `internal/generators/update.go:withD5Notice` — adicionar gate de SO:
```go
func withD5Notice(result TargetResult, claudeDir string) TargetResult {
    if result.State != TargetUpdated && result.State != TargetSkipped {
        return result
    }
    // D5 (ADR-2026-10-14): only relevant on Windows with HOME≠USERPROFILE.
    // Firing without this gate incorrectly labels the live guard as "inert"
    // when CLAUDE_CONFIG_DIR is set only in the trackfw process, not in
    // Claude Code's process.
    _, _, divergent := agentconfig.HomeDivergencePaths()
    if !divergent {
        return result
    }
    inertPath, present := agentconfig.InertClaudeGuardPath(claudeDir)
    if !present {
        return result
    }
    result.Message = fmt.Sprintf(
        "old guard under HOME is inert — Claude Code does not read %s; remove it by hand if you want",
        inertPath,
    )
    return result
}
```

### 2b — `validateHomeDivergenceWindows` — D5 embutido no aviso D4 (ajuste)

O D5 embutido em `validateHomeDivergenceWindows` (validator_home_divergence_rule.go) dispararia
sob as mesmas condições se a função rodasse fora do Windows. No entanto, ela já tem o gate
`agentconfig.HomeDivergencePaths()` (que retorna false fora do Windows). **Correto — sem ação aqui.**

### 2c — Mensagem D4 imprecisa quando CLAUDE_CONFIG_DIR definido (ajuste — R-4)

Em `validateHomeDivergenceWindows`, a mensagem diz "Claude Code reads the OS home (measured)"
mas quando CLAUDE_CONFIG_DIR está definido no Windows, o Claude lê de CLAUDE_CONFIG_DIR, não do
"OS home". A mensagem é gerada apenas no Windows com HOME≠USERPROFILE; o campo `newClaudeDir`
(terceiro argumento: "trackfw writes and audits the Claude guard at %q") está correto. A primeira
frase fica imprecisa mas não silencia nem apaga nada.

**Correção sugerida (ajuste):** condicionar a frase ao caso padrão (sem CLAUDE_CONFIG_DIR):
`"Claude Code reads %q (measured)"` com `newClaudeDir` como argumento — precisa sem hardcodar
"OS home".

### 2d — B1 do hefesto (bloqueante anteriormente): validate declarava guard ativo como inerte

Corrigido no ML-2B: `validateHomeDivergenceWindows` agora chama
`agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)` para `newClaudeDir`. Verificado:
com CLAUDE_CONFIG_DIR=h/.claude (= HOME/.claude), `InertClaudeGuardPath("h/.claude")` compara
com `oldDir = homedir.Dir() + "/.claude" = "h/.claude"` → iguais → retorna false → D5 não dispara.
**B1 FECHADO.**

---

## Alvo 3 — Aviso `home_divergence_windows` e D5

### 3a — `containsTrackfwGuardMarker` — marcadores sobrevivem à codificação JSON?

Os comandos do guard (de `internal/generators/agentfiles.go:503-564`) contêm as strings:
- `trackfw guard git-branch` — sem caracteres especiais JSON
- `trackfw guard credential` — idem
- Formas legacy: `trackfw-credential-guard.sh`, `trackfw-git-branch-guard.sh` — idem

Em JSON, essas strings são codificadas como-estão (sem escape). `strings.Contains(s, "trackfw guard")`
nos bytes raw do JSON encontra `"trackfw guard"` dentro da string JSON. **Marcadores sobrevivem.
FECHADO.**

Nota: o comando cmd.exe (`"trackfw guard git-branch || exit 2"`) também contém `trackfw guard`.
Forma Windows com `.exe` não é usada nos comandos gerados — o binário em PATH é chamado como
`trackfw` em todas as plataformas.

### 3b — D5 no aviso D4 — correto quando CLAUDE_CONFIG_DIR + HOME≠USERPROFILE no Windows

Com CLAUDE_CONFIG_DIR=/custom e HOME≠USERPROFILE no Windows:
- `newClaudeDir = /custom` (de ClaudeConfigDir)
- `InertClaudeGuardPath("/custom")` → `oldDir = HOME/.claude` ≠ `/custom` → lê HOME/.claude/settings.json
- Se tiver guard → D5 dispara: "Old guard at HOME/.claude/settings.json is inert"

Correto: Claude lê `/custom` (CLAUDE_CONFIG_DIR), não HOME/.claude. **FECHADO.**

---

## Alvo 4 — Escritor × auditor: restam divergências?

### Grep de todos os sítios de Claude Code em produção (excluindo testes)

**`homedir.Dir()` no código de produção** (grep executado na cópia):
- Sítios de Claude Code: 0 — todos migrados para `agentconfig.ClaudeConfigDir`
- Sítios de outros CLIs (Codex, Gemini, Cursor, Copilot, Kiro, Windsurf escritor): inalterados
- Artefatos trackfw (`~/.trackfw/`): inalterados — correto per ADR D1

**`os.UserHomeDir()` no código de produção:**
- `validator_credential_guard.go:822` (Windsurf auditor) — resíduo A8
- `agentconfig/agentconfig.go:95` (OSCLIHomeResolver.HomeFor para Windows Claude) — correto

**Sítios de Claude Code — D3:**

| Sítio | Resolvedor | D3 |
|-------|------------|----|
| `update.go:UpdateHarness` (claudeDir) | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `update.go:harnessCredentialGuardTargetClaude` | recebe claudeDir | CONFORME |
| `update.go:harnessGitBranchGuardTargetClaude` | recebe claudeDir | CONFORME |
| `update.go:harnessClaudeSkillTarget` (ML-2C) | recebe claudeDir | CONFORME |
| `update.go:harnessCatalogTarget` | recebe claudeDir via `manager.ClaudeDir` | CONFORME |
| `scaffold.go:installGlobalSkillInner` (ML-2C) | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `agentfiles.go:readClaudeSettingsJSON` (ML-1A) | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `agentfiles.go:globalCredentialGuardInstalledClaude` | via `readClaudeSettingsJSON` | CONFORME |
| `agentfiles.go:globalBranchGuardInstalledClaude` | via `readClaudeSettingsJSON` | CONFORME |
| `validator_git_branch_guard.go:validateGuardGlobalHookResolvable` | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `validator_git_branch_guard.go:validateGuardGlobalHookShLegacyWarnings` | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `validator_git_branch_guard.go:validateGuardGlobalHookD2InlineWarnings` | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `validator_git_branch_guard.go:validateGuardGlobalHookD11LegacyWarnings` | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `validator_credential_guard.go:validateClaudeGuardHookMatcherGlobalWarnings` | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `validator_credential_guard.go:credentialGuardGlobalInstalledWindsurf` | `os.UserHomeDir()` | RESÍDUO A8 |

D3 FECHADO para Claude Code. Windsurf: resíduo A8.

### `Manager.ClaudeDir` — hefesto A1 (CLAUDE_CONFIG_DIR=HOME)

ML-2B corrigiu usando `claudeDirSet` em vez de `destRoot != root`
(`internal/integrations/manager.go:748`). Com CLAUDE_CONFIG_DIR=HOME:
- OLD: `destRoot == root` → false → `else` branch → `filepath.Join(HOME, ".claude/agents/...")` = `HOME/.claude/agents/...`
- NEW: `claudeDirSet=true` + `strings.HasPrefix(destination, "~/.claude/")` → strip `.claude/` prefix → `filepath.Join(HOME, "agents/...")` = `HOME/agents/...`

Correto: quando CLAUDE_CONFIG_DIR=HOME, settings.json fica em `HOME/settings.json` (raiz), não em
`HOME/.claude/`. Agentes/skills em `HOME/agents/`, `HOME/skills/`. **Hefesto A1 FECHADO.**

### `installGlobalSkillInner` — assimetria `rejectHarnessSymlink` vs `ResolveRoot`

`harnessClaudeSkillTarget` em update.go usa `filepath.Dir(claudeDir)` como root diretamente
(sem `ResolveRoot`). `installGlobalSkillInner` em scaffold.go usa `pathguard.ResolveRoot(filepath.Dir(claudeDir))`.
No macOS com simlinks acima do home (`/var → /private/var`), o harness pode recusar uma escrita que
o scaffold aceita para o mesmo arquivo (root não-resolvido vs root-resolvido). Fail-closed, não
fail-open. Pre-existing class (T5 residual do pathguard, documentado no código).

---

## Alvo 5 — Regressão fora do Windows

**Cinco pacotes rodados na cópia com sentinelas:**

```
USERPROFILE=$S1 CLAUDE_CONFIG_DIR=$S2 HOME=$H go test
  ./internal/agentconfig/... ./internal/integrations/...
  ./internal/generators/... ./internal/validator/... ./internal/commands/...
  -count=1
→ ok agentconfig, integrations, generators, validator, commands
→ find $S1 -type f: 0 files  (USERPROFILE sentinela vazio)
→ find $S2 -type f: 0 files  (CLAUDE_CONFIG_DIR sentinela vazio)
```

`ClaudeConfigDir` sem CLAUDE_CONFIG_DIR no macOS: `"/Users/kgsaran/.claude"` (HOME + `"/.claude"`).
Byte-idêntico ao comportamento anterior. **Regressão macOS/Linux: FECHADA.**

---

## Alvo 6 — Windsurf e CLIs inferidos

Nenhuma mudança nos sítios de Windsurf ou dos CLIs inferidos. Divergência escritor/auditor do
Windsurf mantida como resíduo A8 declarado. **Sem regressão; conforme ADR A8.**

---

## Achados

| ID | Severidade | Sítio | Descrição | Correção |
|----|-----------|-------|-----------|---------|
| **R-3** | **BLOQUEANTE** | `internal/generators/update.go:withD5Notice` | `withD5Notice` dispara em qualquer SO quando `CLAUDE_CONFIG_DIR ≠ HOME/.claude`. Diz "Claude Code does not read HOME/.claude/settings.json; remove it by hand" quando CLAUDE_CONFIG_DIR está só no processo do trackfw — o arquivo em HOME/.claude **é o guard ativo**. Reproduzido com o binary. | Adicionar gate `_, _, divergent := agentconfig.HomeDivergencePaths(); if !divergent { return result }` no início de `withD5Notice`. |
| **R-1** | Ajuste | `internal/agentconfig/agentconfig.go:257` | `ClaudeConfigDir` aceita CLAUDE_CONFIG_DIR relativo. `filepath.Clean("../evil")` = `"../evil"`. Guard root vira `".."`, `Beneath("..", "../evil/settings.json")` passa. Escrita vai para `<CWD>/../evil/settings.json`. Medido: `../evil` e `../../evil` passam; `..` sozinho é rejeitado. | Rejeitar CLAUDE_CONFIG_DIR não-absoluto como erro A7: `if !filepath.IsAbs(filepath.Clean(dir)) { return "", fmt.Errorf(...) }`. |
| **R-4** | Ajuste | `internal/validator/validator_home_divergence_rule.go:53` | "Claude Code reads the OS home (measured)" é falso quando CLAUDE_CONFIG_DIR está definido — Claude lê CLAUDE_CONFIG_DIR, não o OS home. Windows-only, mensagem de diagnóstico. | Substituir "the OS home" por `%q` com `newClaudeDir` como argumento. |
| **R-2** | Resíduo | Toda a cadeia claudeDir | Simlink em ancestral ACIMA de `parent(CLAUDE_CONFIG_DIR)` passa no guard. Medido: `link/a/b/claude-cfg` com `link → real` escreve em `real/a/b/claude-cfg/settings.json`. Pre-existing class (T5); root nunca foi EvalSymlinks-verificado em `rejectHarnessSymlink`. | Declarar no ADR como extensão do resíduo T5 para CLAUDE_CONFIG_DIR. |
| **R-5** | Resíduo | `withD5Notice` | CLAUDE_CONFIG_DIR definido só no Claude Code, não no trackfw: trackfw escreve em HOME/.claude, reporta saudável, mas Claude lê de CLAUDE_CONFIG_DIR → guard inerte. D3 mantido internamente; divergência vem de envs de processo distintos. Não mitigável em código. | Documentar no ADR. |

### Confirmações

| B1 (hefesto) | `validateHomeDivergenceWindows` usa `agentconfig.ClaudeConfigDir` | **FECHADO** |
| B2 (hefesto) | `harnessClaudeSkillTarget` e `installGlobalSkillInner` usam `claudeDir` | **FECHADO** |
| Hefesto A1 | `Manager.ClaudeDir` com `claudeDirSet` corrige CLAUDE_CONFIG_DIR=HOME | **FECHADO** |
| Regressão macOS/Linux | 5 pacotes PASS, sentinelas vazios, caminho byte-idêntico | **FECHADO** |
| A7 (auditor com erro emite finding) | `claudeDirErr != nil` → finding em todas as 4 funções do validador | **CONFORME** |
| D3 (escritor = auditor) | todos os 14 sítios de Claude Code usam `agentconfig.ClaudeConfigDir` | **CONFORME** |
| containsTrackfwGuardMarker | marcadores sobrevivem à codificação JSON em todas as formas | **CONFORME** |
| Windsurf resíduo A8 | escritor e auditor inalterados | **CONFORME** |

---

## Veredito

**BLOQUEADO.**

Um bloqueante novo além dos dois do hefesto (já corrigidos):

**R-3**: `withD5Notice` em `internal/generators/update.go` não tem gate de SO. Dispara em qualquer
plataforma quando CLAUDE_CONFIG_DIR está definido no processo do trackfw mas Claude Code roda sem
essa variável. Resultado: o harness diz ao usuário para apagar o guard ativo. Reproduzido com o
binary da cópia. Fix: adicionar gate `agentconfig.HomeDivergencePaths()` antes de
`InertClaudeGuardPath`. Uma linha mais um teste.

R-1 e R-4 são ajustes, não bloqueam merge mas devem entrar no mesmo PR (R-1: uma linha + teste;
R-4: um argumento de format string).

---

## Resíduos para o ADR

1. **`withD5Notice` sem gate de SO** (R-3, bloqueante): corrigido no mesmo PR via gate
   `HomeDivergencePaths()`.

2. **CLAUDE_CONFIG_DIR relativo** (R-1, ajuste): `filepath.IsAbs` check em `ClaudeConfigDir`
   converte em A7-failure explícito.

3. **Contenção expandida de HOME para `parent(CLAUDE_CONFIG_DIR)`** (R-2): por design (A1/D6);
   simlinks acima do root não são verificados — mesmo padrão T5 pré-existente, agora extensível via
   env var. Declarar explicitamente no ADR.

4. **CLAUDE_CONFIG_DIR divergente entre processos** (R-5): não mitigável em código; documentar como
   limitação.

5. **Mensagem D4 imprecisa** (R-4): correção pontual no format string.

6. **Windsurf resíduo A8** (pré-existente, confirmado).

7. **USERPROFILE ausente** (Wave 0 Seção 3.2, confirmado): A7 emite finding; HomeDivergencePaths
   retorna false (correto para diagnóstico D4).
