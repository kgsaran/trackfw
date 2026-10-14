# Revisão de Qualidade — Wave 2, ML-2A
## Branch: fix/guard-global-segue-o-home-que-o-agente-le
## REQ-2026-09-01 · ADR-2026-10-14 · 2026-10-14

Revisor: hefesto-tf

---

## Escopo

Revisão dos artefatos entregues pelos MLs 1A, 1B, 1C e 1D da branch
`fix/guard-global-segue-o-home-que-o-agente-le` (REQ-2026-09-01). Normativo:
ADR-2026-10-14 (D1–D6, A1–A8), `docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md`,
roadmap `ROADMAP-2026-09-22-*.md`. Ferramental: `go vet`, `go test` por pacote, mutações
manuais, sentinelas de diretório, reprodução de cenários compostos.

---

## Q1 — D3: resolvedor único por CLI

**Varredura dos 7 sítios de fiação do Claude (escritor e auditor) e dos sítios restantes.**

| Sítio | Resolvedor | Status D3 |
|---|---|---|
| `update.go:UpdateHarness` — resolve `claudeDir` uma vez, passa para todos os alvos | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `update.go:harnessCredentialGuardTargetClaude` | recebe `claudeDir` do `UpdateHarness` | CONFORME |
| `update.go:harnessGitBranchGuardTargetClaude` | recebe `claudeDir` do `UpdateHarness` | CONFORME |
| `update.go:harnessCatalogTarget` (target="claude") | injeta `claudeDir` via `manager.ClaudeDir` | CONFORME |
| `validator_git_branch_guard.go:validateGuardGlobalHookResolvable` (linha 171) | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `validator_credential_guard.go:validateClaudeGuardHookMatcherGlobalWarnings` (linha 1180) | `agentconfig.ClaudeConfigDir(DefaultResolver)` | CONFORME |
| `agentfiles.go:globalCredentialGuardInstalledClaude` (linha 2386) | `readClaudeSettingsJSON()` → `agentconfig.ClaudeConfigDir` | CONFORME |
| `agentfiles.go:globalBranchGuardInstalledClaude` (linha 2585) | `readClaudeSettingsJSON()` → `agentconfig.ClaudeConfigDir` | CONFORME |
| `validator_credential_guard.go:credentialGuardGlobalInstalledWindsurf` (linha 822) | `os.UserHomeDir()` | RESÍDUO A8 (declarado) |

**Sítio ausente do fechamento A2 — bloqueante (ver Q2-B1):**

`update.go:harnessClaudeSkillTarget` (linha 889) recebe `home` (`homedir.Dir()` = HOME) e
chama `GlobalClaudeSkillPath(home)` = `HOME/.claude/skills/trackfw/SKILL.md`.
No Windows com HOME≠USERPROFILE, escreve o SKILL.md no lugar errado.
O A2 lista `update.go:575` (início de `UpdateHarness`) mas não este alvo.
Causa idêntica à REQ: D1 não foi aplicado ao alvo legacy do skill.

```
update.go:889:  func harnessClaudeSkillTarget(home string, opts UpdateOptions) TargetResult {
update.go:893:      path := GlobalClaudeSkillPath(home)   // home = homedir.Dir() = HOME
scaffold.go:287: func GlobalClaudeSkillPath(home string) string {
scaffold.go:288:     return filepath.Join(home, ".claude", "skills", "trackfw", "SKILL.md")
```

Evidência de conformidade do auditor: nenhum validador lê o SKILL.md — writer e
auditor são a mesma função, ambos usam HOME. D3 interno está íntegro, mas D1 foi violado.

---

## Q2 — Regra Dura de Reconciliação (8 testes + 3 mutações)

### 8 testes verificados

| Teste | Conclusão declarada | Reconciliação |
|---|---|---|
| `TestOSCLIHomeResolver_WindowsBranch_Claude` | `HomeFor(CLIClaude)` retorna USERPROFILE no Windows simulado | CONFORME — falha sem o ramo Windows em `OSCLIHomeResolver.HomeFor` |
| `TestOSCLIHomeResolver_NonClaudeCLIs_Unchanged` | CLIs não-Claude usam HOME mesmo no Windows | AJUSTE — assertiva é `got != userprofileDir`, não `got == homeDir`; não pin positivo |
| `TestClaudeConfigDir_CLAUDE_CONFIG_DIR_Honoured` | `CLAUDE_CONFIG_DIR` é retornado como root (sem `.claude/`) | CONFORME |
| `TestClaudeConfigDir_Default_JoinsHomeDotClaude` | padrão = HOME+`.claude` fora do Windows | CONFORME |
| `TestClaudeConfigDir_Windows_UsesUserprofile` | USERPROFILE+`.claude` no Windows sem `CLAUDE_CONFIG_DIR` | CONFORME |
| `TestSetPlatformForTest_Restore` | restore reverte override | CONFORME |
| `TestHomeDivergenceWindows_Divergent` | HOME≠USERPROFILE → 1 aviso nomeando ambos | AJUSTE — "measured" é substring de "not measured"; a asserção passa para ambas as strings; path de escrita não é verificado |
| `TestManagerClaudeDir_OtherCLI_Unchanged` | plano Codex não usa ClaudeDir | CONFORME — falha se ClaudeDir vazar para não-claude |

### 3 mutações executadas

**M1 — `ClaudeConfigDir` ignorando `CLAUDE_CONFIG_DIR` vazio em vez de não definido**

Mutação: substituir `os.Getenv("CLAUDE_CONFIG_DIR"); dir != ""` por
`os.LookupEnv("CLAUDE_CONFIG_DIR"); ok2 {` (honra string vazia como definida).

```
# W=$(copy); sed LookupEnv
(cd $W && env -u CLAUDE_CONFIG_DIR go test ./internal/{agentconfig,integrations,generators,validator,commands}/... -count=1 | grep -E "^(--- FAIL|FAIL|ok )")
ok  github.com/kgsaran/trackfw/internal/agentconfig
ok  github.com/kgsaran/trackfw/internal/integrations
ok  github.com/kgsaran/trackfw/internal/generators
FAIL github.com/kgsaran/trackfw/internal/validator   (TestScratch — adicionado por esta revisão)
ok  github.com/kgsaran/trackfw/internal/commands
```

Resultado: nenhum teste de produção pega M1. A mutação sobrevive.
Causa: todos os testes que chamam `os.Unsetenv("CLAUDE_CONFIG_DIR")` removem a variável do processo,
tornando `LookupEnv` retornar `("", false)` — o caminho mutado nunca é exercido.
Os scripts de gate usam `CLAUDE_CONFIG_DIR=` (string vazia) mas Go tests usam `os.Unsetenv`,
de forma que o cenário `t.Setenv("CLAUDE_CONFIG_DIR", "")` nunca é testado.
**Lacuna de teste — severidade: ajuste** (ver Q2-A2).

**M2 — `normHomePath` sem normalizar `\` para `/`**

Mutação: remover `p = strings.ReplaceAll(p, \`\\\`, \`/\`)` de `normHomePath`.

```
(cd $W && go test ./internal/agentconfig/... ./internal/validator/... -run "TestHomeDivergence" | grep -E "^(--- FAIL|FAIL|ok )")
--- FAIL: TestHomeDivergenceWindows_CaseAndTrailingSlash
FAIL github.com/kgsaran/trackfw/internal/validator
```

Capturado por: `TestHomeDivergenceWindows_CaseAndTrailingSlash`.
Conclusão afirmada pelo teste: caminhos que diferem apenas em case ou barra final
são tratados iguais (A3 — Git Bash MSYS2 converte HOME POSIX para forma Windows).
CONFORME.

**M3 — `Manager` aplicando `ClaudeDir` a escopo de projeto (removendo só a guarda de escopo)**

Mutação: remover `plan.Claim.Scope == "global" &&` da condição `manager.go:738`,
mantendo a guarda de target.

```
(cd $W && env -u CLAUDE_CONFIG_DIR go test ./internal/.../... | grep -E "^(--- FAIL|FAIL|ok )")
--- FAIL: TestInstallExplicitScopeProjectDoesNotPrompt
--- FAIL: TestDoctorReportsNoFindingsOnEmptyProject
... (18 testes no pacote commands)
FAIL github.com/kgsaran/trackfw/internal/commands
ok  github.com/kgsaran/trackfw/internal/integrations
```

Capturado por: 18 testes no pacote `commands`. Nenhum teste em `manager_claude_test.go`
cobre project-scope com `ClaudeDir` definido — a cobertura vem de testes de integração
em `commands`. CONFORME (captura existe, embora não no pacote dedicado).

---

## Q3 — Isolamento dos testes

Método: criar diretórios-sentinela vazios, rodar os pacotes tocados com
`USERPROFILE=$SENTINEL CLAUDE_CONFIG_DIR=$SENTINEL2`, verificar com `find -type f`.

```bash
SENTINEL_UP=$HOME/sentinel-up  SENTINEL_CC=$HOME/sentinel-cc
mkdir -p $SENTINEL_UP $SENTINEL_CC
USERPROFILE=$SENTINEL_UP CLAUDE_CONFIG_DIR=$SENTINEL_CC \
  go test ./internal/{agentconfig,integrations,generators,validator,commands}/... -count=1
find $SENTINEL_UP -type f   # (vazio)
find $SENTINEL_CC -type f   # (vazio)
```

Todos os quatro pacotes tocados retornam os sentinelas vazios.
**Nenhum teste escreve no USERPROFILE ou CLAUDE_CONFIG_DIR reais.** CONFORME.

Nota: em macOS, `os.UserHomeDir()` lê HOME (não USERPROFILE), de modo que o sentinel
de USERPROFILE é informativo apenas — o sentinela de CLAUDE_CONFIG_DIR é o probe
real. Para CI Windows, o `agentconfig.SetPlatformForTest` isola o caminho corretamente.

---

## Q4 — Duplicação e coesão

**`readClaudeSettingsJSON` × `readGlobalHookJSON`**

Separação intencional e bem-documentada (`agentfiles.go:2105-2109`):
- `readGlobalHookJSON` usa `homedir.Dir()` (HOME) — para todos os CLIs não-Claude.
- `readClaudeSettingsJSON` usa `agentconfig.ClaudeConfigDir` — exclusiva para Claude.
Nenhum chamador usa `readGlobalHookJSON` para caminhos `.claude` na versão atual
(o corpus `pathguard/testdata/corpus-pre-fix` é a versão antiga, não código ativo).
CONFORME.

**`HomeDivergencePaths` × `InertClaudeGuardPath`**

Funções complementares, não duplicadas:
- `HomeDivergencePaths` — diagnóstico de divergência HOME vs. USERPROFILE.
- `InertClaudeGuardPath` — detecta resíduo D5 em `HOME/.claude/settings.json`.
Ambas chamadas em `validateHomeDivergenceWindows`. CONFORME.

**`destRoot` no `Manager` — ramo `strings.HasPrefix(destination, "~/.claude/")`**

Quando `ClaudeDir` retorna o próprio `HomeDir` (`CLAUDE_CONFIG_DIR=HOME`, sem subdir),
`destRoot == root`, logo `destRoot != root` é falso e o prefix `~/.claude/` não é
removido. O agente aterra em `HOME/.claude/agents/...` em vez de `HOME/agents/...`.

Evidência (cópia em W, `internal/integrations/claudedir_home_edge_test.go`):
```
Written to HOME/.claude/agents: true   ← path errado
Written to HOME/agents:         false  ← path esperado (CLAUDE_CONFIG_DIR=HOME → root=HOME)
```

Severidade: **ajuste**. `CLAUDE_CONFIG_DIR=HOME` é atípico (a medição documental usa
`/custom/path`), a guarda `beneath` rejeita corretamente qualquer saída da arvore,
e nenhum script ou ADR cita este cenário. Entretanto, o comportamento está errado e
não há teste. O fix seria capturar um bool na atribuição de `destRoot` em vez de
retestar `destRoot != root`.

Caminho afetado: `internal/integrations/manager.go:763`.

---

## Q5 — Mensagens

**Idioma e precisão do aviso `home_divergence_windows`**

O aviso (`validator_home_divergence_rule.go:53-60`) está em inglês, consistente com as
vizinhas. O terceiro argumento `%q` de "trackfw writes and audits the Claude guard at %q"
é `osHome` (USERPROFILE), não o path completo `osHome/.claude/settings.json`.
Imprecisão leve mas aceitável em isolamento.

**Bug reproduzido: CLAUDE_CONFIG_DIR + HOME≠USERPROFILE — bloqueante** (ver Q2-B2 acima)

Cenário: Windows simulado, HOME=`h`, USERPROFILE=`up`, `CLAUDE_CONFIG_DIR=h/.claude`.
`agentconfig.ClaudeConfigDir` retorna `h/.claude` (A1). O guard é escrito em `h/.claude/settings.json`.

O `validateHomeDivergenceWindows` (`validator_home_divergence_rule.go:45`):
```go
newClaudeDir := filepath.Join(osHome, ".claude")  // = up/.claude (USERPROFILE)
// ClaudeConfigDir não é consultado aqui
if inertPath, present := agentconfig.InertClaudeGuardPath(newClaudeDir); present {
```

`InertClaudeGuardPath(up/.claude)` compara `h/.claude` (HOME/.claude) com `up/.claude` —
eles diferem, então lê `h/.claude/settings.json`, encontra o guard ativo, e o declara
**inerte**.

Saída reproduzida:
```
HOME="h" differs from the OS home (USERPROFILE) "up" — Claude Code reads the OS home (measured);
trackfw writes and audits the Claude guard at "up".   ← ERRADO: é h/.claude, não up
...
Old guard at "h/.claude/settings.json" is inert — Claude Code does not read this file;
remove it by hand if you want.   ← ERRADO: este arquivo é o guard ativo
```

Dois defeitos encadeados:
1. A mensagem aponta USERPROFILE como destino quando o destino é `CLAUDE_CONFIG_DIR`.
2. O D5 chama de "inert" o guard que `ClaudeConfigDir` aponta — instrui o usuário a
   apagar um controle de negação ativo.

Teste de reprodução: `internal/validator/claudeconfig_divergence_test.go` (no W, removido após revisão).
```
--- FAIL: TestScratch_ClaudeConfigDir_With_Divergence
    D5 inert notice fired for a LIVE guard — would instruct user to delete active protection
```

**Severidade: bloqueante.** Fix no `validateHomeDivergenceWindows`: consultar
`agentconfig.ClaudeConfigDir(DefaultResolver)` para obter `newClaudeDir` real; emitir
finding (A7) se a resolução falhar; usar `newClaudeDir` real na mensagem e em
`InertClaudeGuardPath`.

---

## Q6 — Portabilidade

```bash
GOOS=windows go vet ./internal/...
# (saída vazia — sem erros)
echo rc=$?
# rc=0
```

Novos arquivos no diff: nenhum com sufixo `_windows`, `_linux`, `_darwin` ou GOARCH acidental.
O novo `internal/agentconfig/agentconfig.go` usa `effectiveGOOS()` para ramificar em Windows —
abordagem correta e testável.

Testes com `t.Skip` ou `runtime.GOOS`: nenhum encontrado nos pacotes tocados.

---

## Resumo dos achados

| ID | Severidade | Sítio | Descrição |
|---|---|---|---|
| **B1** | **Bloqueante** | `validator_home_divergence_rule.go:45–60` | D5 declara o guard ativo em `CLAUDE_CONFIG_DIR` como "inert" quando HOME≠USERPROFILE; instrui apagar controle de negação ativo |
| **B2** | **Bloqueante** | `update.go:893`, `scaffold.go:287` | `harnessClaudeSkillTarget` usa `home` (HOME), não `ClaudeConfigDir`; SKILL.md vai para `HOME/.claude` no Windows — ghost install D1 omitido de A2 |
| **A1** | Ajuste | `manager.go:763` | `destRoot == root` quando `CLAUDE_CONFIG_DIR=HOME`; agente aterra em `HOME/.claude/agents/...` em vez de `HOME/agents/...`; sem teste |
| **A2** | Ajuste | `agentconfig_test.go` | Lacuna de teste para M1: nenhum teste usa `t.Setenv("CLAUDE_CONFIG_DIR","")` — `LookupEnv` sobrevive sem detecção |
| **A3** | Ajuste | `validator_home_divergence_rule_test.go:53` | `TestHomeDivergenceWindows_Divergent`: "measured" é substring de "not measured"; assertiva de path de escrita ausente |
| **O1** | Observação | `manager_claude_test.go:196` | `TestManagerClaudeDir_OtherCLI_Unchanged` assertiva negativa (`!= userprofileDir`); seria mais forte com pin positivo (`== homeDir`) |
| **O2** | Observação | `manager.go:763` | Claude global com destino `~/` sem prefix `~/.claude/` falha `beneath` — fail-closed correto, mas sem teste de cobertura |
| **O3** | Observação | `manager_claude_test.go:39-81` | Nome `WindowsDivergence_AgentLandsUnderCLAUDE_DIR` é enganoso: ClaudeDir é injetado via closure; USERPROFILE/CLAUDE_CONFIG_DIR são configurados mas não lidos pelo código testado |

---

## Veredito

**BLOQUEADO.**

Dois defeitos bloqueantes antes de mergear:

- **B1**: `validateHomeDivergenceWindows` chama `InertClaudeGuardPath` com `osHome/.claude`
  (hardcoded) sem consultar `ClaudeConfigDir`. Quando `CLAUDE_CONFIG_DIR=HOME/.claude` com
  HOME≠USERPROFILE, o guard ativo é declarado inert e o usuário é instruído a apagá-lo.
  Reproduzido. Novo ML em REQ-2026-09-01; PR mantido aberto.

- **B2**: `harnessClaudeSkillTarget` escreve o SKILL.md usando `home` (HOME), não
  `agentconfig.ClaudeConfigDir`. No Windows com HOME≠USERPROFILE, o SKILL.md vai para
  `HOME/.claude/...` — ghost install, mesma causa raiz da REQ original, sítio omitido
  do fechamento A2. Novo ML em REQ-2026-09-01; PR mantido aberto.

Os achados A1–A3 e O1–O3 não bloqueiam; podem ser tratados no mesmo PR ou em ML
subsequente, conforme prioridade do arquiteto.
