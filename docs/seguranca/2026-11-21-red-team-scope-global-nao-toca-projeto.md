# Red Team — ML-2B: scope global não toca o projeto

> REQ: docs/req/REQ-2026-11-17-agents-install-com-scope-global-reescreve-arquivos-de-regras-e-hooks-do-projeto-no-cwd.md
> ADR: docs/adr/ADR-2026-11-21-agents-e-skills-install-com-scope-global-nao-escrevem-no-projeto.md
> Fix auditado: `internal/commands/integrations_flags.go:278–316` (commit ML-1A, branch fix/scope-global-nao-escreve-no-projeto)
> Autor: hades-tf | Data: 2026-11-21 | ML: ML-2B (Wave 2)

---

## VEREDITO: APROVA COM RESSALVAS

O contrato D1–D5 está implementado corretamente. Nenhum vetor testado produziu escrita de regras ou
hooks sob `manager.ProjectRoot` com escopo global. O D2 hint vai ao canal correto (`cmd.ErrOrStderr()`),
não contamina o stdout em modo `--json`, e não cita caminhos absolutos. O escopo project (D3) continua
escrevendo normalmente. Um residual pré-existente (R1) é nomeado e classificado.

---

## Metodologia

**Ponto de partida:** contrato (ADR D1–D5), não o diff. Cada vetor foi reimplementado a partir da
leitura do contrato e do código de produção — não do conjunto de testes existente ou do relatório do ML-1A.

**Oracle:** `rtSnapshot(dir)` captura SHA-256 de cada arquivo regular sob o diretório do projeto
(antes e depois de cada arm). Qualquer arquivo adicionado ou modificado no project dir durante um
arm de escopo global é reportado como violação de D1. Oracle escolhido em vez de lista explícita de
arquivos porque D1 diz "nada sob ProjectRoot" — não "apenas estes arquivos conhecidos".

**Isolamento:** `integrationCommandFixture` (HOME, USERPROFILE, CODEX_HOME, cwd em TempDir separados;
`integrationsStdinIsTTY = false`). `OPENCODE_CONFIG_DIR` e `XDG_CONFIG_HOME` isolados adicionalmente
nos arms que tocam opencode.

**Rota de medição:** testes Go com fixture (sem binário, sem shell, sem make quality).
Arquivo: `internal/commands/zz_redteam_scope_global_test.go`

```
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -count=1 -timeout 5m -run 'TestRedTeam' -v ./internal/commands/
```

Todos os testes listados abaixo passaram. Resultado final: `PASS ok github.com/kgsaran/trackfw/internal/commands 1.340s`

---

## Tentativas e resultados

### T1 — `agents install --scope global --targets <alvo> --items backend` (10 alvos)

**Teste:** `TestRedTeamSweep_AllTargets_AgentsInstall_GlobalScope`

Varreu todos os 10 alvos do catálogo (claude, codex, gemini, antigravity, cursor, copilot, windsurf,
amazonq, opencode, kiro). Para cada alvo: snapshot antes → install global → snapshot depois →
`rtDiff`. Arquivos stale (CLAUDE.md, AGENTS.md, GEMINI.md, .windsurfrules sem bloco trackfw:rules)
pré-semeados no project dir para que o oracle detecte "modificado" além de "criado".

**Contra-arm (D3) embutido:** mesmo alvo com `--scope project` em fixture separado; `assertHasProjectWrites`
confirma que o oracle não é vacuoso — o escopo project produz diff não-vazio para todos os 10 alvos.

**Resultado:** PASS. Diff vazio em todos os 10 arms globais. Diff não-vazio em todos os 10 arms de projeto.

Verificação de catalog: todos os 10 alvos têm caminhos globais com prefixo `~/`; nenhuma superfície
carece do escopo `global`. BuildPlans não usa o caminho do projeto para escopo global em nenhum alvo.

### T2 — sem `--scope`, sem TTY → resolve para global → sem escrita no projeto

**Teste:** `TestRedTeamT2_NonTTY_DefaultGlobal_NoProjectWrites`

Rodou `agents install --targets claude --items backend` sem `--scope` em ambiente não-TTY
(`integrationsStdinIsTTY = false`, já configurado pelo fixture). CLAUDE.md stale pré-semeado.
Oracle: hash idêntico antes/depois.

**Resultado:** PASS. CLAUDE.md não foi modificado. `resolveScope` setou `opts.scope = "global"` pelo
ramo non-TTY (linha 561–563 de integrations_flags.go), e o guard `opts.scope == "project"` bloqueou
a injeção corretamente.

### T3 — TTY + prompt retorna "global" → sem escrita no projeto + D2 hint no stderr

**Teste:** `TestRedTeamT3_TTYPromptGlobal_NoProjectWrites_HintInStderr`

`integrationsStdinIsTTY = true`, `promptInstallScopeRunner` stub retorna `"global"` (caminho de
pré-seleção, Enter sem escolha). Rodou `agents install --targets claude --items backend`.

**Vetor T2 (ameaça):** um guard keyed em `cmd.Flags().Changed("scope")` falharia aqui — o usuário
não passou `--scope`, então `Changed("scope") = false`, e o guard deixaria o bloco rodar mesmo com
`opts.scope = "global"`. O fix usa `opts.scope == "project"`, avaliado DEPOIS de `resolveScope`,
o que é correto.

**Resultado:** PASS. Snapshot diff vazio. D2 hint ("not modified") presente no stderr capturado via
`cmd.SetErr`.

### T4 — `skills install --scope global` → sem escrita no projeto

**Teste:** `TestRedTeamT4_SkillsInstall_GlobalScope_NoProjectWrites`

`newSkillsCmd()`, `--targets claude --items governance --scope global`. CLAUDE.md stale pré-semeado.
Verificação adicional: D2 hint para skills cmd diz "skills install" (não "agents install").

**Vetor T4 (ameaça):** um guard keyed em `KindAgents` deixaria skills install escapar. O fix usa
`opts.scope == "project"` sem checar `kind` — correto, pois `executeIntegrationMutation` é o ponto
único compartilhado por agents e skills.

**Resultado:** PASS. Snapshot diff vazio. Hint correto ("skills install --scope project").

### T5 — `--targets windsurf --scope global` → sem `.windsurfrules` nem `.windsurf/hooks.json`

**Teste:** `TestRedTeamT5_Windsurf_GlobalScope_NoRulesOrHooks`

`agents install --targets windsurf --items backend --scope global`. Checks explícitos para `.windsurfrules`
(que seria escrito por `InjectRulesForTool` via loop :280 E por `InjectWindsurfHooks:1613` internamente)
e `.windsurf/hooks.json`.

**Vetor T5 (ameaça — fix parcial):** um fix que gateasse apenas `InjectRulesForTool` (loop :280)
mas não `InjectHooksForTools` (:288) ainda escreveria `.windsurfrules` via `InjectWindsurfHooks:1613`
(que chama `InjectRulesForTool("windsurf", ...)` internamente, agentfiles.go:1613). O fix correto
gateou o bloco inteiro 278–316.

**Resultado:** PASS. Nenhum dos dois arquivos criado. Oracle e checks explícitos ambos passaram.

### T6 — `--json --scope global` → stdout JSON limpo, D2 hint apenas no stderr

**Teste:** `TestRedTeamT6_JSON_GlobalScope_CleanStdout_HintInStderr`

`agents install --targets claude --items backend --scope global --json`. `cmd.SetOut(&stdout)` e
`cmd.SetErr(&stderr)` explicitamente configurados (Cobra usa `ErrOrStderr()` para o D2 hint;
sem `cmd.SetErr`, o hint iria para o stderr real do processo, não para o buffer do teste).

**Vetor T6 (ameaça):** D2 hint impresso em `cmd.OutOrStdout()` em vez de `cmd.ErrOrStderr()` corromperia
o JSON. Verificação: `json.Unmarshal(stdout.Bytes(), &out)` não deve falhar.

**Resultado:** PASS. stdout parseável como `lifecycleOutput`. Hint ausente do stdout. Hint presente
no stderr.

### T7 — D2 hint não cita o cwd nem nenhum caminho absoluto

**Teste:** `TestRedTeamT7_D2Hint_NoCwdOrHomePath`

Capturou o hint via `cmd.SetErr`. Verificou ausência do project path, do home path, e de qualquer
token com prefixo `/`.

**Vetor T7 (ameaça):** um implementador que formatasse a dica como
`"rules not written to <cwd>"` violaria ADR D2 ("não cita o caminho do cwd").

**Resultado:** PASS. Hint capturado: `"Installed globally. Project rules and hooks were not modified. To update the project, run: trackfw agents install --scope project  or  trackfw update\n"`. Nenhum caminho absoluto.

### T8 — `--scope project` não imprime o D2 hint

**Teste:** `TestRedTeamT8_ProjectScope_NoD2Hint`

`agents install --targets claude --items backend --scope project`. Verificou ausência de "not modified"
no stderr capturado.

**Vetor T8 (ameaça — falso positivo de D3):** um implementador que imprimisse o hint incondicionalmente
enganaria o usuário de que o projeto não foi alterado quando foi.

**Resultado:** PASS. Hint ausente do stderr para escopo project.

### T9 — subdiretório: install global a partir de subdir não escreve no subdir

**Teste:** `TestRedTeamT9_Subdir_GlobalScope_NoSubdirWrites`

Criou `project/src/`, fez `os.Chdir(src)`. `manager.ProjectRoot = os.Getwd() = src`. Rodou
`agents install --targets claude --items backend --scope global`. Verificou snapshot de `src` e do
`project` (parent).

**Resultado:** PASS. Comportamento pré-existente (mesmo mecanismo que project scope, que também usa
`os.Getwd()`) não regrediu. Não é um vetor de bypass; documentado como comportamento esperado.

### Additional — `--force --scope global` não bypassa o guard

**Teste:** `TestRedTeamForce_GlobalScope_NoProjectWrites`

CLAUDE.md stale pré-semeado. Rodou `agents install --targets claude --items backend --scope global --force`.

**Resultado:** PASS. `--force` afeta idempotência (overwrite de arquivos globais existentes), não o
roteamento de escopo. Guard mantido.

### Additional — último `--scope` vence (global), sem escrita no projeto

**Teste:** `TestRedTeamLastScopeWins_Global_NoProjectWrites`

`agents install --targets claude --items backend --scope project --scope global` (Cobra last-wins → global).

**Resultado:** PASS. opts.scope = "global" após double-flag. Guard correto.

### Additional — codex: AGENTS.md e .codex/hooks.json ausentes com escopo global

**Teste:** `TestRedTeamCodex_GlobalScope_NoRulesOrHooks`

**Resultado:** PASS.

### Additional — antigravity: .github/hooks/trackfw-attention.json ausente com escopo global (D4)

**Teste:** `TestRedTeamAntigravity_GlobalScope_NoProjectHooks`

`agents install --targets antigravity --items backend --scope global`. Verifica ausência de
`.github/hooks/trackfw-attention.json` (escrito por `InjectHooksForTools(["antigravity"], ...)`)
e confirma que `ValidateAntigravityIDEGlobalGuardWarningsForDoctor` não foi chamada (efeito indireto:
sem o aviso D8 no stderr do arm global, e sem arquivo de hooks no project dir).

**Resultado:** PASS. ADR D4 implementado: o aviso D8 e os hooks do Antigravity ficam dentro do bloco
`scope == "project"`.

### Additional — skills cmd D2 hint diz "skills install" (não "agents")

**Teste:** `TestRedTeamSkillsHintNamedCorrectly`

**Resultado:** PASS. O `%s` no formato usa `kind` ("skills"), não hardcoded.

---

## Residuais declarados

**R1 — `third-party install` com `--scope global` escreve `.trackfw/thirdparty-provenance.json` no cwd (pré-existente, fora de D1)**

`executeThirdPartyInstall` não chama `InjectRulesForTool` nem `InjectHooksForTools`. Mas escreve
`.trackfw/thirdparty-provenance.json` via `thirdparty.UpsertProvenanceEntry(manager.ProjectRoot, ...)`
independente do escopo, e `.trackfw/thirdparty-references.json` quando `--apply-to` é passado.
ADR D1 proíbe "nada sob ProjectRoot" com global scope; a cláusula lista "nem regras nem hooks".
`.trackfw/thirdparty-provenance.json` não é regras nem hooks — fora do escopo textual de D1.
Além disso, `third-party install` exige `--yes-global-scope-unverified` para escopo global e tem
default project. Classificação: fora de D1–D5 por leitura literal. Nomeado para decisão explícita
do arquiteto; não bloqueia o merge desta REQ.

**R2 — Wave 0 R2 (D4 antigravity warning) — FECHADO pelo ML-1A**

Wave 0 declarou como residual pendente de implementação. O fix de ML-1A incluiu
`ValidateAntigravityIDEGlobalGuardWarningsForDoctor` dentro do bloco `scope == "project"`, conforme
exigido por ADR D4. Medido e confirmado pelo arm `TestRedTeamAntigravity_GlobalScope_NoProjectHooks`.
R2 está fechado; não persiste como residual.

**R3 — `trackfw: agents global: agent_models not configured` vai para `os.Stderr` direto (pré-existente)**

As linhas de aviso de agent_models usam `fmt.Fprintln(os.Stderr, ...)` em vez de `cmd.ErrOrStderr()`.
Isso é pré-existente e não está no escopo desta REQ (não é um write em ProjectRoot). Classificação:
LOW, pré-existente, sem impacto no contrato D1–D5. Mencionado porque é visível na saída dos testes
(`trackfw: agents global: agent_models not configured in ~/.trackfw/trackfw.yaml`).

---

## Arquivos produzidos

- `internal/commands/zz_redteam_scope_global_test.go` — arquivo de testes. Deve ser removido pelo
  arquiteto antes do merge (artefato de red team, não de regressão; os guards de regressão estão
  nos testes estendidos do ML-1A).
- `docs/seguranca/2026-11-21-red-team-scope-global-nao-toca-projeto.md` — este documento.

---

## git status (pós-medição)

```
?? internal/commands/zz_redteam_scope_global_test.go
```

Nenhum staged change. Nenhum arquivo de produto modificado. Nenhuma operação git de escrita executada.
