# Wave 0 — Threat Model: scope global não toca o projeto

> REQ: docs/req/REQ-2026-11-17-agents-install-com-scope-global-reescreve-arquivos-de-regras-e-hooks-do-projeto-no-cwd.md
> ADR: docs/adr/ADR-2026-11-21-agents-e-skills-install-com-scope-global-nao-escrevem-no-projeto.md
> Autor: hades-tf | Data: 2026-11-21 | ML: ML-0A

---

## 1. Enumeração — completude dos sítios de escrita

### 1.1 Metodologia

Grep inicial de todos os chamadores de produção dos injetores; depois leitura das funções alcançadas
por `agents|skills install|update|uninstall` e `third-party install`; verificação do fecho transitivo
até o ponto de entrada cobra; depois medição com binário real (bin/trackfw HEAD) via testes Go isolados
com `integrationCommandFixture` (HOME, CODEX_HOME, USERPROFILE isolados em TempDir, cwd = projeto
temporário, integrationsStdinIsTTY = false).

### 1.2 Grep de sítios diretos

Comando rodado:

```
grep -rn "InjectRulesForTool\|InjectRulesDetected\|InjectHooksForTools\|InjectHooksDetected\|InjectWindsurfHooks" \
  internal/ | grep -v "_test.go" | grep -v "testdata"
```

Resultado (linhas selecionadas relevantes ao escopo; comentários e strings de Fatalf omitidos):

```
internal/commands/integrations_flags.go:280  generators.InjectRulesForTool(target, manager.ProjectRoot)
internal/commands/integrations_flags.go:288  generators.InjectHooksForTools(opts.targets, manager.ProjectRoot)
internal/commands/discover.go:152            discover.InstallGates(r, cwd, out)
internal/commands/init.go:656               generators.InjectRulesForTool(tool, cwd)
internal/commands/init.go:672               generators.InjectHooksForTools(aiTools, cwd)
internal/discover/discover.go:89            generators.InjectHooksDetected(rootDir)
internal/generators/update.go:75            InjectRulesDetected(cwd)
internal/generators/update.go:82            InjectHooksDetected(cwd)
internal/generators/update.go:3414          InjectRulesDetected(r)
internal/generators/update.go:3436          InjectHooksDetected(r)
internal/generators/scaffold.go:302         InjectHooksDetected(cwd)
internal/generators/agentfiles.go:243       InjectRulesForTool(tool, cwd)        [dentro de InjectRulesDetected]
internal/generators/agentfiles.go:252       InjectRulesForTool(tool, cwd)        [dentro de InjectRulesDetected]
internal/generators/agentfiles.go:1613      InjectRulesForTool("windsurf", ...)  [dentro de InjectWindsurfHooks]
```

### 1.3 Fecho transitivo — sítios intermediários não alcançados por agents|skills install

Verificação por leitura com evidência de grep de chamadores. Dois comandos rodados:

```
grep -n "update\.\|discover\.\|ForceGenerate\|InstallGates\|runProjectTarget\|scaffold\." \
  internal/commands/integrations_flags.go
# → zero resultados

grep -n "update\.\|discover\.\|ForceGenerate\|InstallGates\|runProjectTarget\|scaffold\." \
  internal/commands/integrations_thirdparty.go
# → zero resultados
```

| Sítio | Único chamador de produção | Ponto cobra | Reachable por agents/skills install? |
|---|---|---|---|
| `discover.go:152` → `InstallGates` | `internal/commands/discover.go` | `trackfw discover --init` | Não |
| `generators/update.go:75,82` | `internal/commands/update.go` | `trackfw update` | Não |
| `generators/update.go:3414,3436` → `runProjectTarget` | `internal/generators/update.go` (interno) | `trackfw update` | Não |
| `scaffold.go:302` | chamado por `init.go` | `trackfw init` | Não |

**Lista fechada:** `trackfw discover --init`, `trackfw update`, `trackfw init` são comandos de projeto
por contrato (ADR D5); fora do escopo desta REQ.

### 1.4 check-gates-falsify.sh e outros scripts

Dois greps rodados:

```
grep -n "agents install\|skills install" scripts/check-gates-falsify.sh
# → zero resultados (arquivo existe: 9212 linhas, verificado com wc -l)
```

```
grep -rn "agents install\|skills install" scripts/check-update-parity.sh \
  scripts/check-agent-models-parity.sh | grep -v "scope project\|scope=project\|#"
# → hits apenas nas funções install_claude_agents() e run_install_global() (ver §3)
```

`check-gates-falsify.sh` não contém nenhuma invocação de `agents install` ou `skills install`.
Nenhum cenário neste arquivo precisa ser atualizado.

### 1.5 Bloco incriminado — único sítio de escrita em cwd com scope global

**`integrations_flags.go:278–296`** — único sítio dentro de `executeIntegrationMutation` que escreve
sob `manager.ProjectRoot`. Guard é apenas `if operation == "install"`:

```go
if operation == "install" {
    for _, target := range opts.targets {                                                   // :279
        if err := generators.InjectRulesForTool(target, manager.ProjectRoot); err != nil { // :280
            return fmt.Errorf("install %s auxiliary rules: %w", target, err)
        }
    }
    if err := generators.InjectHooksForTools(opts.targets, manager.ProjectRoot); err != nil { // :288
        fmt.Fprintf(os.Stderr, "  ⚠ agent hooks: %v\n", err)
    }
    if containsAntigravityTarget(opts.targets) {
        for _, msg := range validator.ValidateAntigravityIDEGlobalGuardWarningsForDoctor(manager.ProjectRoot) { // :294
            fmt.Fprintf(os.Stderr, "  ⚠ %s\n", msg)
        }
    }
}
```

**Arquivos escritos no cwd por target** (somente o target passado — não há adição implícita de outros
targets; loop de linha 279 itera apenas sobre `opts.targets`):

| Target | `InjectRulesForTool(target, ...)` → arquivo | `InjectHooksForTools([target], ...)` → arquivo |
|---|---|---|
| `claude` | `CLAUDE.md` | `.claude/settings.json` |
| `codex` | `AGENTS.md` | `.codex/hooks.json` |
| `gemini` | `GEMINI.md` | `.gemini/settings.json` |
| `windsurf` | `.windsurfrules` (loop :280 + InjectWindsurfHooks:1613) | `.windsurf/hooks.json` |
| `copilot` | `.github/copilot-instructions.md` | — (sem entry em cliDetectors) |
| `amazonq` | `.amazonq/developer/guidelines.md` | `.amazonq/cli-agents/q_cli_default.json` |
| `cursor` | `.cursor/rules/trackfw.mdc` | `.cursor/hooks.json` |
| `kiro` | — (no-op: não está em agentFiles) | `.kiro/hooks/trackfw-attention.json` |
| `antigravity` | — (no-op: não está em agentFiles) | `.github/hooks/trackfw-attention.json` |
| `opencode` | — (no-op: não está em agentFiles) | `.opencode/plugins/trackfw-guard.js` |

**Acoplamento adicional:** `InjectWindsurfHooks` chama `InjectRulesForTool("windsurf", guardRoot)`
internamente (agentfiles.go:1613). Uma correção que gate apenas `InjectRulesForTool` no loop:280
sem gatar `InjectHooksForTools`:288 ainda escreve `.windsurfrules` via windsurf. O único fechamento
completo é gatar o bloco inteiro 278–298.

**manager.ProjectRoot = os.Getwd()** — `integrationsManager()` linha 445. Não é a raiz git/trackfw.

### 1.6 Agents/skills update e uninstall NÃO são afetados

O bloco 278–298 é guardado por `if operation == "install"`. O switch-case logo acima:
```go
case "update":    err = manager.Update(plans, opts.force)   // :257
case "uninstall": err = manager.Uninstall(plans, opts.force) // :260
```
Para `update` e `uninstall`, o fluxo retorna sem entrar no bloco 278.

Confirmado por medição: `TestWave0ARM6AgentUpdateGlobalDoesNotRunInjectBlock`
(zz_wave0_arm_verify_test.go) — PASS. Rodado `agents update --scope global --targets claude` com
fixture stale CLAUDE.md; hash idêntico antes e depois.

### 1.7 manager.resolve() — sem escrita extra sob ProjectRoot em escopo global

`manager.resolve()` (manager.go:703–896): para escopo global, `root = m.HomeDir` (linha 711–712).
O manifest também vai para `HomeDir`. Nenhum dos três métodos escreve sob `ProjectRoot` para planos
de escopo global — confirmado por leitura; ARM 7 abaixo confirma que `.trackfw/integrations-manifest.json`
só aparece no cwd para project scope.

### 1.8 scopeExplicit — campo morto

`integrationOptions.scopeExplicit` é declarado (linha 29) e checado em `resolveScope` (linha 537),
mas **nunca definido como `true` em nenhum ponto de produção** do codebase:

```
grep -rn "\.scopeExplicit\s*=" internal/ | grep -v "_test.go"
# → zero resultados
```

O campo é dead code — nunca altera o comportamento atual.

### 1.9 Medição com binário HEAD (via go test)

Testes em `zz_wave0_arm_verify_test.go` usam `integrationCommandFixture` (HOME, CODEX_HOME,
USERPROFILE em TempDir, cwd = projeto temporário, integrationsStdinIsTTY = false). Hash SHA-256
capturado antes e depois do ARM para verificação de mudança.

```
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m -run "TestWave0ARM" ./internal/commands/ -v
```

Saída completa:

```
=== RUN   TestWave0ARM2GeminiGlobalDoesNotModifyCLAUDE
    ARM-2 confirmed: CLAUDE.md hash unchanged
    (e8b25bc5d5e9f8900362e50759bd2279d2068b25475ef4c77c503ab85adb23db);
    GEMINI.md written by inject (expected bug)
    ARM-2: GEMINI.md created in project (confirms inject bug for --scope global)
--- PASS: TestWave0ARM2GeminiGlobalDoesNotModifyCLAUDE (0.01s)

=== RUN   TestWave0ARM3CodexGlobalDoesNotModifyCLAUDE
    ARM-3 confirmed: CLAUDE.md hash unchanged
    (e8b25bc5d5e9f8900362e50759bd2279d2068b25475ef4c77c503ab85adb23db)
--- PASS: TestWave0ARM3CodexGlobalDoesNotModifyCLAUDE (0.01s)

=== RUN   TestWave0ARM6AgentUpdateGlobalDoesNotRunInjectBlock
    §1.6 confirmed: agents update --scope global did NOT modify CLAUDE.md
    (e8b25bc5d5e9f8900362e50759bd2279d2068b25475ef4c77c503ab85adb23db)
    §1.6 note: .claude/settings.json exists (written by install step, not update)
--- PASS: TestWave0ARM6AgentUpdateGlobalDoesNotRunInjectBlock (0.02s)

PASS
ok  github.com/kgsaran/trackfw/internal/commands  0.561s
```

**ARM 1 — `agents install --scope global --targets claude` (medido em sessão anterior):**
```
Destination (global):  FAKE_HOME/.claude/agents/trackfw-*.md  (12 arquivos)
Escrito no cwd (PROJETO):
  CLAUDE.md               ← InjectRulesForTool("claude", …) — hash ALTERADO (bloco injetado)
  .claude/settings.json   ← InjectClaudeHooks via InjectHooksForTools(["claude"], …)
```

**ARM 2 — `agents install --scope global --targets gemini` (medido por Go test, PASS):**
```
Destination (global): FAKE_HOME/.gemini/agents/trackfw-*.md
Escrito no cwd (PROJETO):
  GEMINI.md               ← InjectRulesForTool("gemini", …) — criado (não existia)
  .gemini/settings.json   ← InjectGeminiHooks via InjectHooksForTools(["gemini"], …)
  CLAUDE.md               — PRESENTE no cwd (fixture stale), hash INALTERADO (confirmado por Go test)
```

Correção em relação à listagem inicial desta sessão: CLAUDE.md foi listado como "escrito" na sessão
anterior. Era vacuidade — a fixture stale estava presente mas não foi modificada. `InjectRulesForTool`
itera apenas sobre `opts.targets = ["gemini"]`; `injectOrUpdateRules` é chamada com
`filePath = filepath.Join(guardRoot, "GEMINI.md")` (agentfiles.go:229), não CLAUDE.md. Medido.

**ARM 3 — `agents install --scope global --targets codex` (medido por Go test, PASS):**
```
Destination (global): FAKE_HOME/.codex/agents/trackfw-*.toml
Escrito no cwd (PROJETO):
  AGENTS.md               ← InjectRulesForTool("codex", …) — criado
  .codex/hooks.json       ← InjectCodexHooks via InjectHooksForTools(["codex"], …)
  CLAUDE.md               — PRESENTE no cwd (fixture stale), hash INALTERADO (confirmado por Go test)
```

**ARM 4 — `agents install --scope global --targets windsurf` (medido em sessão anterior):**
```
Destination (global): FAKE_HOME/.codeium/windsurf/skills/…
Escrito no cwd (PROJETO):
  .windsurfrules           ← InjectRulesForTool("windsurf", …) VIA LOOP :280
                            + InjectRulesForTool("windsurf", …) VIA InjectWindsurfHooks:1613
  .windsurf/hooks.json     ← InjectWindsurfHooks via InjectHooksForTools(["windsurf"], …)
  CLAUDE.md                — PRESENTE no cwd (fixture stale), hash INALTERADO (mesmo mecanismo ARM 2/3)
```

**ARM 5 — `skills install --scope global --targets claude` (medido em sessão anterior):**
```
Destination (global): FAKE_HOME/.claude/skills/trackfw-*.md  (17 arquivos)
Escrito no cwd (PROJETO):
  CLAUDE.md               ← InjectRulesForTool("claude", …) — hash ALTERADO
  .claude/settings.json   ← InjectClaudeHooks (mesma causa via InjectHooksForTools)
```

**ARM 6 — `agents install --targets claude` (sem --scope, não-TTY → default global):**
```
Mesmo resultado do ARM 1. resolveScope() define opts.scope="global" (linha 547) quando
stdin não é TTY (confirmado por TestInstallWithoutScopeOutsideTTYDefaultsToGlobal).
```

**ARM 7 — `agents install --scope project --targets claude` (D3 baseline):**
```
Escrito no cwd (PROJETO):
  .claude/agents/trackfw-*.md  (12 arquivos via manager.Install)
  .claude/settings.json         ← InjectClaudeHooks
  .trackfw/integrations-manifest.json  ← manifest (project scope)
  CLAUDE.md                     ← InjectRulesForTool("claude", …)
```

**ARM update — `agents update --scope global --targets claude` (medido por Go test, PASS):**
```
CLAUDE.md: hash INALTERADO. .claude/settings.json não criado pelo update.
Confirma §1.6: inject block bloqueado por if operation == "install".
```

**Verificação pós-medição (todos os ARMs):** `git -C /…/trackfw status --short` → vazio (clean).

### 1.10 Go test sweep — install sem --scope project

```
grep -n "SetArgs.*install" \
  internal/commands/agents_skills_test.go \
  internal/commands/agentfiles_catalog_install_test.go \
  internal/commands/init_guard_hooks_test.go | grep -v '"--scope", "project"'
```

| Arquivo | Linha | Test | Args | Scope resolvido | Afirma que cwd não foi modificado? |
|---|---|---|---|---|---|
| agents_skills_test.go | 60 | TestInstallRequiresTargetsOutsideTTY | `install` (sem targets) | — (erro antes do inject) | n/a |
| agents_skills_test.go | 350 | TestInstallWithoutScopeOutsideTTYDefaultsToGlobal | `install --targets claude --items backend` | non-TTY → global | **Não** — afirma apenas `~/.claude/agents/` |
| agents_skills_test.go | 383 | TestInstallWithTargetsButNoScopeStillPromptsInTTY | `install --targets claude --items backend` | TTY + prompt → "project" | n/a — scope é project |
| agents_skills_test.go | 428 | TestListWithoutScopeReportsGlobalDestinations | `install --targets claude --items backend` | non-TTY → global | **Não** — afirma apenas destino global e home path |

```
grep -rn "ValidateAntigravity\|antigravity.*install\|install.*antigravity" \
  internal/commands/ | grep "_test.go"
# → zero resultados
```

Nenhum teste Go existente asserta o contrato antigo ("global escreve CLAUDE.md no cwd"). Nenhum
teste Go existente asserta o contrato novo ("global NÃO escreve CLAUDE.md no cwd"). O fix não quebra
nenhum teste Go existente pelo caminho das rules/hooks.

**Gaps a fechar no ML-1A:**
- `TestInstallWithoutScopeOutsideTTYDefaultsToGlobal` (linha 350): estender para afirmar que CLAUDE.md
  e `.claude/settings.json` não foram criados no cwd.
- `TestListWithoutScopeReportsGlobalDestinations` (linha 428): mesmo gap.
- `TestInstallGlobalCodexRespectsCodexHome` (linha 457): afirma destino em CODEX_HOME; gap: ausência
  de AGENTS.md e `.codex/hooks.json` no cwd.

### 1.11 Lacuna de vacuidade (§1.9)

`injectOrUpdateRules` (agentfiles.go:159–212): só altera o arquivo se (a) não existe (cria) ou (b)
existe sem o bloco `<!-- trackfw:rules:start -->` (append). Para o teste de ML-1A: usar fixture
stale (sem bloco) para ambos os braços, global e project. `os.Stat` não distingue arquivo criado de
pré-existente; não é suficiente. O oracle correto: hash SHA-256 antes e depois para pré-existentes;
verificação de não-existência pós-ARM para novos.

### 1.12 third-party install

`executeThirdPartyInstall` **não** chama `InjectRulesForTool` nem `InjectHooksForTools` (lido;
zero ocorrências em `integrations_thirdparty.go`). No entanto, escreve dois arquivos sob
`manager.ProjectRoot` independente do escopo:

| Arquivo | Chamada | Quando |
|---|---|---|
| `.trackfw/thirdparty-provenance.json` | `thirdparty.UpsertProvenanceEntry(manager.ProjectRoot, ...)` | Sempre, incluindo `--scope global` (via `--yes-global-scope-unverified`) |
| `.trackfw/thirdparty-references.json` | `integrations.UpsertThirdPartyReference(manager.ProjectRoot, ...)` | Apenas quando `--apply-to` é passado |

Veja §R1 para a classificação.

### 1.13 Lista fechada

A enumeração está fechada. Nenhum outro sítio sob `agents|skills install|update|uninstall` ou
`third-party install` escreve rules/hooks no cwd com escopo global, além do bloco
`integrations_flags.go:278–296`.

---

## 2. Threat Model — como D1 pode ser esvaziado

**Adversário:** o implementador apressado ou o arquiteto otimista que faz um fix parcial ou constrói
o guard com a condição errada. Não é um atacante externo.

### T1 — Guard `if opts.scope == "project"` colocado antes de resolveScope

O cobra `StringVar` para `--scope` (linha ~364 de `integrations_flags.go`) preenche `opts.scope`
antes de `RunE`. Um flag `--scope project` explícito já está em `opts.scope = "project"` quando
`RunE` inicia. O risco aqui não é o flag explícito — é o **prompt path**: quando o usuário está em
TTY e não passa `--scope`, `resolveScope` (linha 162) chama `promptInstallScopeRunner`, que pode
retornar "project". Se o guard para o bloco de inject for posicionado antes da linha 162 e ler
`opts.scope` nesse ponto, ele lê `""` (flag não alterado), não "project". O bloco então roda com
scope = "" para todos os casos não-TTY — inclusive global default — porque o guard com `""` nunca
iguala "project". D3 regressão: braço `TestInstallWithTargetsButNoScopeStillPromptsInTTY` estendido
para afirmar CLAUDE.md e `.claude/settings.json` escritos pega isso.

**Como capturar:** braço `TestInstallWithTargetsButNoScopeStillPromptsInTTY` estendido para afirmar
que CLAUDE.md e `.claude/settings.json` foram criados (D3 regressão).

### T2 — Guard keyed em `cmd.Flags().Changed("scope")` em vez de `opts.scope`

A sabotagem concreta: `if cmd.Flags().Changed("scope") && opts.scope == "global" { skip }`.
Isso skipa a escrita quando o usuário explicitamente passou `--scope global`, mas NÃO quando o scope
veio do prompt TTY (retornou "global") ou do default non-TTY ("global"). Ambos os casos não setam
`Changed("scope")`. Um `agents install --targets claude` em CI (sem --scope, sem TTY) continuaria
escrevendo no projeto.

**Como capturar:** braço "sem --scope, sem TTY" com `integrationsStdinIsTTY = false` que afirma
hash CLAUDE.md idêntico antes e depois (ou ausência do arquivo).

### T3 — Prompt TTY retorna "global" (pré-seleção do select)

`promptInstallScopeRunner` pré-seleciona "global" no `huh.Select` (confirmado por
`TestInstallScopeSelectDefaultsToGlobal`, agents_skills_test.go:409). Em TTY com bare Enter, a
seleção retorna "global". Um fix keyed em `cmd.Flags().Changed("scope")` falha aqui: o usuário
pressionou Enter, o scope foi "escolhido" pelo prompt, mas `Changed("scope") = false`. Resulta em
escrita no cwd mesmo com scope = "global".

**Como capturar:** stub `promptInstallScopeRunner` retornando "global" + `integrationsStdinIsTTY = true`;
afirmar hash idêntico de CLAUDE.md antes e depois e hint D2 no stderr.

### T4 — Guard keyed apenas em KindAgents, omite KindSkills

A linha 314 gate corretamente o registro no trackfw.yaml para agents-only. Um implementador que
copiasse esse padrão para o bloco de inject deixaria `skills install --scope global --targets claude`
continuando a escrever no cwd (ARM 5 mediu). D5 da ADR diz "mesma causa, mesmo contrato" — o guard
correto é baseado em `opts.scope`, não em `kind`.

**Como capturar:** braço `newSkillsCmd() install --scope global --targets claude` afirmando hash
idêntico de CLAUDE.md antes e depois; contra-braço `--scope project` afirmando hash ALTERADO.

### T5 — Fix parcial: gata InjectRulesForTool mas não InjectHooksForTools (ou vice-versa)

`InjectWindsurfHooks` chama `InjectRulesForTool("windsurf", ...)` internamente (agentfiles.go:1613).
Se o fix gata apenas `InjectRulesForTool`, `.windsurf/hooks.json` ainda será escrito; se gata apenas
`InjectHooksForTools`, `.windsurfrules` ainda será escrito via InjectWindsurfHooks:1613. O único
fechamento completo é gatar o bloco inteiro 278–298.

**Como capturar:** braço `--targets windsurf --scope global` que afirma ausência de `.windsurfrules`
**e** ausência de `.windsurf/hooks.json` no cwd.

### T6 — D2 hint impresso no stdout com --json

A ADR D2 exige que o hint seja impresso depois do install com escopo global. Se a dica for para
`cmd.OutOrStdout()` em vez de `cmd.ErrOrStderr()`, um chamador que parseia `--json` receberá lixo.

**Como capturar:** braço `--scope global --json` que verifica que stdout é JSON válido (sem a dica
inline) e que stderr contém a dica.

### T7 — D2 hint contém o cwd (viola ADR)

ADR D2 diz explicitamente: "A mensagem não cita o caminho do cwd: ela não depende de onde o comando
rodou." Um implementador que formata a dica como "rules not written to `<cwd>`" violaria D2.

**Como capturar:** braço que afirma que o stderr/hint não contém nenhum path absoluto.

### T8 — D2 hint impresso também com --scope project (D3 falso positivo)

Um implementador que imprime o hint sempre (sem checar escopo) quebraria project scope.

**Como capturar:** braço `--scope project` que afirma ausência da dica no stderr.

### T9 — ProjectRoot = os.Getwd(), não raiz git

`integrationsManager()` define `ProjectRoot = os.Getwd()` (linha 445). Se o usuário rodar em
subpasta (ex.: `cd src/ && trackfw agents install --scope global`), o inject block atual escreve sob
`src/`. Comportamento pré-existente ao fix e idêntico para project scope. O test oracle deve checar
o cwd do teste.

---

## 3. Alvos de falsificação — nas duas direções

### Direção A — global escreve no projeto (regressão de D1)

| Falsificação | Mecanismo | Qual teste/braço pega |
|---|---|---|
| Remover `if operation == "install"` ou adicionar `opts.scope` mas inverter condição | Inject volta a correr para scope global | Braço `--scope global` que afirma hash CLAUDE.md idêntico antes/depois com fixture stale |
| Guard colocado antes de resolveScope | opts.scope = "" nunca iguala "project" | Braço non-TTY sem --scope (resolve global via default) |
| Guard keyed em `Changed("scope") && scope=="global"` | prompt retorna "global" sem Changed | Braço TTY + stub returning "global" |
| Fix gata InjectRulesForTool mas não InjectHooksForTools | windsurf via InjectWindsurfHooks:1613 ainda escreve .windsurfrules | Braço `--targets windsurf --scope global`: .windsurfrules ausente E .windsurf/hooks.json ausente |
| Fix gata apenas KindAgents | skills install global continua escrevendo | Braço `newSkillsCmd() install --scope global --targets claude` |

**Sítio de entrada da sabotagem:** linha 278 de `integrations_flags.go`. O único guard correto:
`if operation == "install" && opts.scope == "project"`, avaliado DEPOIS de `resolveScope` (linha 162).

**Oracle correto:** hash SHA-256 de cada arquivo de rules/hooks antes e depois. Para arquivos
pré-existentes com conteúdo stale: afirmar hash idêntico. Para arquivos ausentes: afirmar
não-existência pós-ARM. `os.Stat` não é suficiente para fixtures pré-existentes.

### Direção B — project scope para de escrever (regressão de D3)

| Falsificação | Mecanismo | Qual teste/braço pega |
|---|---|---|
| Guard com condição invertida (`!= "project"`) | Project scope entra no branch "não escreve" | `--scope project --targets claude` afirmando CLAUDE.md hash ALTERADO e `.claude/settings.json` criado |
| Guard `if opts.scope == "project"` antes de resolveScope | opts.scope = "" para prompt-chosen project | TestInstallWithTargetsButNoScopeStillPromptsInTTY estendido com assert de CLAUDE.md criado |
| InjectHooksForTools chamado mas InjectRulesForTool omitido | CLAUDE.md não criado por install project | `--scope project --targets claude` verificando hash ALTERADO de CLAUDE.md |
| D2 hint impresso com --scope project | Usuário vê "regras não alteradas" mas foram alteradas | `--scope project` verificando ausência da dica no stderr |

**Testes existentes que já afirmam D3:**

- `TestAgentsInstallCreatesRulesFileFromEmptyProject` (agentfiles_catalog_install_test.go) — usa
  `--scope project`; afirma GEMINI.md, copilot-instructions.md, .windsurfrules, amazonq guidelines.
- `TestAgentsInstallInjectsGuardHooks` (init_guard_hooks_test.go) — usa `--scope project`; afirma
  `.gemini/settings.json`.
- `TestAgentsInstallRulesFileIdempotent` (agentfiles_catalog_install_test.go) — idempotência.
- `TestInstallWithTargetsButNoScopeStillPromptsInTTY` (agents_skills_test.go:365) — prompt retorna
  "project", afirma `.claude/agents/` no projeto.

Nenhum desses asserta o contrato antigo (global escreve no cwd). O fix não quebra nenhum.

**Principal gap a fechar no ML-1A:**

`TestInstallWithoutScopeOutsideTTYDefaultsToGlobal` (linha 350) e
`TestListWithoutScopeReportsGlobalDestinations` (linha 428): rodam install global (non-TTY default),
afirmam apenas destino global. Não afirmam que cwd não foi modificado. Esses dois precisam ser
estendidos para afirmar zero criação/modificação de rules/hooks no cwd.

**Testes e scripts que documentam o contrato antigo:**

`check-update-parity.sh:154–168` — `install_claude_agents()` usa scratch_dir isolado. Comentário:
"`agents install --scope global` writes CLAUDE.md into the *current project*." Após o fix, o
scratch_dir permanece como defesa em profundidade; o comentário deve ser atualizado. Único sítio
em scripts.

---

## 4. Residual declarado

**R1 — `third-party install` com `--scope global` escreve `.trackfw/thirdparty-provenance.json` no cwd.**

ADR D1 proíbe "nada sob ProjectRoot" com global scope; a cláusula colon define "nem regras
(`InjectRulesForTool`) nem hooks (`InjectHooksForTools`)". `.trackfw/thirdparty-provenance.json` não
é regras nem hooks — está fora do escopo textual de D1. ADR D5 diz "regras ou hooks" (mesma cláusula).
`third-party install` exige `--yes-global-scope-unverified` para global e tem default `project`. Por
leitura literal, este sítio está fora de D1. Nomeado para decisão explícita do arquiteto. Não bloqueia
ML-1A.

**R2 — `ValidateAntigravityIDEGlobalGuardWarningsForDoctor(manager.ProjectRoot)` com escopo global.**

ADR D4 diz que o aviso D8 de Antigravity só deve sair com `--scope project`. A função é read-only;
não escreve arquivos. Semântica incorreta com global scope. O fix de ML-1A deve gatar o bloco inteiro
(linhas 278–298 inclusive o ValidateAntigravity). Item de implementação; não bloqueia ML-1A.
Nenhum teste Go asserta este aviso para o caminho do install.

**R3 — Oracle de teste baseado em lista explícita de arquivos, não em snapshot.**

Se `manager.HomeDir == manager.ProjectRoot` (cwd = HOME), o manager.Install global também escreve
em cwd, e um oracle "nenhum novo arquivo" daria falso positivo. O oracle correto verifica a lista
explícita de arquivos de rules/hooks por hash SHA-256. Residual de test design.

**R4 — scopeExplicit é campo morto; nenhum bypass atual.**

O campo `integrationOptions.scopeExplicit` nunca é definido como `true` em produção. Se um futuro
contribuidor re-introduzir aliases que settam `scopeExplicit = true`, esses aliases herdarão o
comportamento do escopo resolvido. Risco futuro; zero impacto no presente.

---

## Estado do ML-0A

**Status:** ⬜ Pendente — a ser marcado ✅ pelo arquiteto após auditoria do conteúdo deste documento.
Os critérios de aceite no roadmap (`[ ] The four sections answered with evidence`) permanecem `[ ]`;
o arquiteto é a autoridade para flipá-los. Este documento não edita o roadmap.

Arquivo de medição: `internal/commands/zz_wave0_arm_verify_test.go` — pode ser removido após
auditoria do ML-0A, antes do merge do PR.
