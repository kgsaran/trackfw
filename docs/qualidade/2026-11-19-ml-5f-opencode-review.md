# ML-5F — Revisão de Qualidade: Guard no OpenCode via plugin (REQ-2026-11-17)

**Revisor:** hefesto-tf  
**Data:** 2026-11-19  
**Branch:** feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli  
**Commit:** `f7f0c8a3`  
**Veredito:** REPROVA — A7 bloqueia merge; veja remediação proposta

---

## Gates executados

```
go build ./...                                                      → ok
go vet ./internal/opencodeplugin/ ./internal/guard/ \
        ./internal/generators/ ./internal/validator/ \
        ./internal/agentconfig/                                     → limpo
go test ./internal/opencodeplugin/ -count=1 -timeout 120s          → ok (1.023s)
go test ./internal/guard/ -count=1 -timeout 60s                    → ok (3.107s)
go test ./internal/generators/ -count=1 -timeout 120s              → ok (84.240s)
go test ./internal/validator/ -count=1 -timeout 120s               → ok (38.660s)
go test ./internal/agentconfig/ -count=1 -timeout 60s              → ok (0.210s)
TestOpenCodePlugin_JSRuntime (5 braços: arm1–arm5)                  → PASS (0.70s)
scratch D4/patch — controle + braço patch (ver A7)                  → ver abaixo
git diff --cached                                                    → vazio
```

---

## Escopo do diff

32 arquivos, 2905 inserções, 37 remoções. Pacotes novos/expandidos:

| Pacote | Papel |
|---|---|
| `internal/opencodeplugin/` | asset `trackfw-guard.js` (go:embed) + `Render(scope)` |
| `internal/generators/opencode_plugin.go` | `InjectOpenCodePlugin` |
| `internal/generators/update.go` | target `opencode-guard` em `harnessOpenCodeGuardTarget` |
| `internal/guard/credential.go` | `credOpenCodeReadScan`, `isOpenCodePayload` |
| `internal/guard/payload.go` | `isOpenCodePayload` |
| `internal/guard/writescope.go` | `.opencode/plugins/`, `opencode.json`, `opencode.jsonc` |
| `internal/agentconfig/agentconfig.go` | `OpenCodeConfigDirWithSource`, `OpenCodePluginDisplayPath` |
| `internal/validator/validator_opencode_plugin.go` | D5/D6 checker |
| `internal/auditsurface/auditsurface.go` | `opencode` em `canonicalRuntimes` |
| `internal/commands/doctor.go` | `DoctorOpenCodePluginGlobalAbsent` |
| `scripts/check-update-parity.sh` | cenários 5c, 5c-2, 5c-3 |

---

## Aderência ao ADR D1–D6

**D1 — Plugin único com marcador.**
`trackfw-guard.js` linha 1: `// trackfw:opencode-guard`. `Render("global")` substitui
`const SCOPE = "project"` por `const SCOPE = "global"` (uma vez, invariante verificada).
`Render("project")` retorna `bytes.Clone` para prevenir mutação de slice compartilhado.
Verificado em `TestRender_MarkerOnFirstLine`, `TestRender_ScopeDiffers`, `TestRender_ProjectIsMutable`. ✓

**D2 — spawnSync com argv, fail-closed.**
`spawnSync("trackfw", ["guard", guardCmd].concat(extraFlags), {input, encoding, timeout:30000, cwd})` —
sem `shell: true`. `throw` em `result.error || result.status !== 0`. Cobertura: braços arm1 (block),
arm2 (pass), arm3 (ENOENT). ✓

**D3 — Payload `{hook_source:"opencode", tool_name, tool_input, cwd}` e cobertura de ferramentas.**
Mapeamento implementado:

| Ferramenta | Guard |
|---|---|
| `shell`, `bash` | credential + git-branch |
| `write`, `edit`, `patch`, `apply_patch` | credential |
| `read` | credential |

Go-side: `credOpenCodeReadScan` ativa apenas quando `hook_source=="opencode" && tool_name=="read"`,
lendo `tool_input.file_path` (absoluto obrigatório). `isOpenCodePayload` usa schema check explícita
(lição F1). ✓

**D4 — Autoproteção. ⚠ DIVERGÊNCIA para `patch`/`apply_patch` (ver A7).**
`selfProtect` protege `__filename`, `globalDir/plugins/trackfw-guard.js` e
`projectDir/.opencode/plugins/trackfw-guard.js` para `write/edit/patch/apply_patch`.
Para `write` com `args.path` explícito: confirmado por scratch (ver A7). Para `patch` com alvo
em `patchText` (sem `args.path`/`args.filePath`): selfProtect retorna sem proteção (M17 residual).
ADR D4 lista `patch`/`apply_patch` como cobertos; a limitação M17 não está no ADR como residual
explícito. DIVERGÊNCIA.

**D5 — Geração e auditoria de projeto.**
`cliDetectors["opencode"]` detecta `.opencode/` como diretório; `InjectOpenCodePlugin` cria
`.opencode/plugins/trackfw-guard.js` com `MkdirAll` + `pathguard`. Idempotente por comparação
de conteúdo normalizado CRLF. Validator checa ausência/marker/staleness via
`applyRuleWarnOnly("credential_guard_presence_warning")` em AMBOS os sítios. ✓ com nota A9.

**D6 — Aviso de global ausente (doctor only).**
`harnessOpenCodeGuardTarget` em `update.go` escreve global plugin com `SCOPE = "global"`.
`state=missing` sem `--install-missing`. `validateOpenCodePluginGlobalAbsent` para o doctor
quando projeto tem plugin mas global está ausente. ✓

---

## Achados

### A1 — RETIRADO

A1 original ("V1 export usa `hookOutput.args`") era achado falso. A assinatura V1 de
`"tool.execute.before"` é `async (input, output) =>` onde `output.args` é o campo correto
dos args da ferramenta — confirmado em `docs/portabilidade/2026-11-19-opencode-contrato-de-plugin.md`
linha 475.

---

### A2 — Racional R-1 factualmente errado + gap limitado de D4 para globalDir relativo (Baixo — não bloqueia)

**Arquivo:linha:** `internal/opencodeplugin/trackfw-guard.js:53-55`

```js
// R-1: a relative value is accepted here because it is the same value that
// Go used when writing the plugin.
```

**Por que o racional é falso:** `OpenCodeConfigDirWithSource` em `agentconfig.go` REJEITA
`OPENCODE_CONFIG_DIR` relativo com erro — Go nunca escreve o plugin global a partir de um
path relativo. A afirmação "mesmo valor que o Go usou" é incorreta.

**D4 gap (limitado):** quando `OPENCODE_CONFIG_DIR` tem valor relativo no ambiente de execução,
`globalOpenCodeConfigDir()` retorna um path relativo. `pluginProtected[1]` =
`nodePath.join(relativeGlobalDir, "plugins", "trackfw-guard.js")` = path relativo.
`eqPath` só faz comparação de string (sem resolve); `target` (line 92) é sempre absoluto.
O check falha silenciosamente.

```js
function eqPath(a, b) {   // internal/opencodeplugin/trackfw-guard.js:44
  if (process.platform === "win32" || process.platform === "darwin") {
    return a.toLowerCase() === b.toLowerCase()
  }
  return a === b
}
```

**Mitigação por `__filename`:** `pluginProtected[0] = __filename` é sempre absoluto. Quando a
instância em execução é o plugin global, `__filename` O protege independentemente de `globalDir`.
O gap só afeta a instância do plugin de PROJETO tentando proteger o plugin GLOBAL — cenário
raro (OPENCODE_CONFIG_DIR relativo é configuração inválida conforme XDG spec).

A divergência para `XDG_CONFIG_HOME` é idêntica.

**Recomendação:** corrigir o comentário R-1; adicionar `nodePath.resolve()` na saída de
`globalOpenCodeConfigDir()` para espelhar o comportamento Go.

---

### A3 — `CODEX_HOME` isolado em testes de OpenCode (Ruído cosmético — não bloqueia)

**Arquivo:linha:** `internal/generators/opencode_plugin_test.go:22`, `:57`, `:90`

`t.Setenv("CODEX_HOME", "")` é irrelevante para `InjectOpenCodePlugin` — copy-paste de outro
teste. Harmless.

---

### A4 — String literal de Marker e path duplicados sem necessidade (Baixo — não bloqueia)

**Locais onde `.opencode/plugins/trackfw-guard.js` está declarado como literal:**

| Arquivo | Forma |
|---|---|
| `internal/generators/opencode_plugin.go:14` | `const openCodePluginRelPath` |
| `internal/validator/validator_opencode_plugin.go:44` | `const openCodePluginRelPath` |
| `internal/auditsurface/auditsurface.go:51` | valor no map `runtimeWiringPath` |

**Locais onde `"// trackfw:opencode-guard"` está declarado:**

| Arquivo | Forma |
|---|---|
| `internal/opencodeplugin/opencodeplugin.go` | `Marker` (exportado) |
| `internal/auditsurface/auditsurface.go:901` | `const openCodePluginMarker` (local, sem test pin) |

**Por que o fix é simples:** `internal/opencodeplugin` só importa stdlib
(`bytes`, `embed`, `fmt`, `strings`) — verificado com `go list -f '{{join .Imports "\n"}}' ./internal/opencodeplugin`.
Tanto `generators` quanto `validator` já importam `opencodeplugin`. Sem ciclo. `auditsurface`
também pode importar `opencodeplugin` livremente.

O `go:embed` embute `trackfw-guard.js` a partir de `internal/opencodeplugin/`, não valida a
string de destino `.opencode/plugins/...` — drift na string de destino não é detectado em
compile time.

**Recomendação:** exportar `PluginRelPath` e usar `opencodeplugin.Marker` nos três pacotes.

---

### A5 — arm1 não asserta "operation blocked" (Reconciliação · Baixo — não bloqueia)

**Arquivo:linha:** `internal/opencodeplugin/opencodeplugin_jsruntime_test.go:274,311`

O cabeçalho do arm1 e a conclusão inline declaram `stderr contains "operation blocked"`.
O corpo do teste verifica apenas:

```go
if !strings.Contains(stderr, "THREW:") { ... }   // throw genérico
if strings.Contains(stderr, "not found") { ... }  // exclui ENOENT
```

A frase "operation blocked" nunca é asserta — violação da Regra Dura de Reconciliação.
O texto real emitido (quando fake trackfw sai rc=2 com stderr vazio) é
`"trackfw guard credential: operation blocked"` (JS linha 198).

**Recomendação:** adicionar `strings.Contains(stderr, "operation blocked")` ao arm1.

---

### A6 — `cli-parity.md` cita `execa` e nega nova extraction path (Doc inaccuracy · Baixo — não bloqueia)

**Arquivo:linha:** `docs/cli-parity.md:4428` e `:4896`

Linha 4428: "plugin calls `execa`" e "No new payload extraction path is needed".
Linha 4896: "executado pelo plugin via `execa`".

**Evidências:**
- D2 proíbe `execa` (npm package externo, risco T11); `trackfw-guard.js` usa `spawnSync`.
- `credOpenCodeReadScan` em `internal/guard/credential.go` É uma nova extraction path Go-side.

**Recomendação:** `execa` → `spawnSync`; "No new payload extraction path" → "Go-side adds `credOpenCodeReadScan` for `tool_name=read`".

---

### A7 — D4 não protege `patch`/`apply_patch` quando alvo está no patchText — BLOQUEANTE (Médio — BLOQUEIA)

**Arquivo:linha:** `internal/opencodeplugin/trackfw-guard.js:86-91`

```js
var rawTarget = args.path || args.filePath || null
if (!rawTarget) {
  // M17 residual: patch/apply_patch without an explicit path cannot be verified.
  return          // ← selfProtect retorna sem verificar
}
```

O ADR D4 (linha 68) afirma: "O plugin nega `write`/`edit`/`patch`/`apply_patch` cujo alvo
resolvido seja [os paths protegidos]." O residual do ADR (linha 74) exclui explicitamente
apenas `rm` via `shell`. Linha 104 ("write, edit e patch não foram medidos no binário") é
sobre o disparo do hook, não sobre onde o alvo está.

**Scratch evidence (isolado; realpath, PATH sem trackfw):**

```
# Controle — write com args.path explícito
env HOME=/tmp/t OPENCODE_CONFIG_DIR="" PATH=/tmp/t/bin:/usr/bin:/bin \
  node driver.js plugin.js plugin.js "control"
→ THREW: trackfw-guard: blocked — write to opencode plugin file is not permitted (D4 self-protection)
   exit: 1   ✓ mock dispatches corretamente em selfProtect

# Braço patch — patchText com alvo embarcado
env HOME=/tmp/t OPENCODE_CONFIG_DIR="" PATH=/tmp/t/bin:/usr/bin:/bin \
  node driver.js plugin.js plugin.js "patch"
→ THREW: trackfw not found or timed out: spawnSync trackfw ENOENT
   exit: 1   ← selfProtect passou sem throw; spawnSync alcançado → confirma bypass de D4
```

O controle prova que o mock funciona. O braço patch prova que `selfProtect` não foi acionado
e `runGuard` foi chamado. Um agente que use `patch {"patchText": "*** Update File: .opencode/plugins/trackfw-guard.js..."}` bypassa D4 e modifica o plugin.

A declaração patchText como chave é baseada no nome mais plausível (OpenCode V2 `patch` tool);
o resultado se mantém para qualquer shape sem `path`/`filePath`.

**Remediação (escolha do arquiteto):**

| Opção | Descrição | Trade-off |
|---|---|---|
| **B — Amend ADR** | Adicionar ao residual D4: "patch/apply_patch sem campo path/filePath explícito (alvo no texto do patch — M17 aberto)" | Rápido; gap permanece |
| **A — Fix implementation** | Extrair cabeçalhos de arquivo de todos os args string-valued: `*** Update/Add/Delete/Move File:` e `+++`/`---` lines; resolver contra projectDir e aplicar eqPath | Elimina o gap; requer ML dedicado |

Sob a Regra Dura de Causa Raiz, o fix vai como ML novo na REQ-2026-11-17 vigente.
A7 sobrepõe o escopo do relatório de red team (hades-tf) — arquiteto deve reconciliar.

---

### A8 — `update.go`: comentário invertido e re-leitura de env (Baixo — não bloqueia)

**Arquivo:linha:** `internal/generators/update.go:3049-3052` e `:3062`

**Item 1 — Comentário:** "Use `filepath.Dir(openCodeDir)` (the OPENCODE_CONFIG_DIR value itself,
one level up from `plugins/`)" — `filepath.Dir(openCodeDir)` é o PARENTE de OPENCODE_CONFIG_DIR,
não o valor em si. O parentético descreve `openCodeDir` mas está atribuído a `filepath.Dir(openCodeDir)`.

**Item 2 — Re-leitura:** `OpenCodeConfigDirWithSource` já retorna `(dir, source, err)`. Para
decidir o `symlinkRoot`, o código ignora `source` e usa `os.LookupEnv("OPENCODE_CONFIG_DIR")`
diretamente. Funcional (env não muda mid-process), mas acoplamento desnecessário ao env.

**Recomendação:** corrigir comentário; passar `source` como parâmetro para eliminar a
re-leitura.

---

### A9 — D5 severity não escalável via `rules:` (Nota de design — não bloqueia)

`applyRuleWarnOnly("credential_guard_presence_warning")` significa que ausência ou staleness do
OpenCode plugin não pode ser escalada para violation via `rules:`. Em contraste,
`credential_guard_script_integrity` usa `applyRule` e pode ser escalado.

O ADR D5 não menciona o nome da regra nem a severidade imutável. Design choice razoável para
primeira implementação; sem ação necessária neste PR.

---

## Nota: CI — job `go` sem `setup-node`; Windows com `setup-node@v7`

O job `go` (`.github/workflows/quality.yml:32-46`, ubuntu-latest) roda `go test -timeout 2m ./...`
sem `setup-node`. `TestOpenCodePlugin_JSRuntime` não honra `TRACKFW_DISABLE_EXTERNAL_COMMANDS` —
pula apenas se `findNodeRuntime()` não encontrar runtime JS. Ubuntu-latest tem Node pré-instalado;
os testes rodam.

`windows-full-suites` (linha 169) tem `setup-node@v7` explícito e roda
`go test -v -p 1 -parallel 1 -timeout 2m ./...` (linhas 361, 383) cobrindo `./internal/opencodeplugin`.
Sem `-short` ou `-run` filter. Ambos os jobs confirmados como executando os testes JS.

---

## Duplicação e padrões existentes

A resolução de config dir vive em dois lugares por necessidade (plugin roda em Bun/Node). A
divergência R-1 está documentada mas o racional é incorreto — ver A2.

A autoproteção para `patch`/`apply_patch` sem path explícito é documentada como M17 residual no
portability doc (linha 816). O ADR D4 não qualifica essa limitação — ver A7 (bloqueante).

---

## Regra de Reconciliação — verificação cruzada

| Teste | Conclusão declarada | Consistente com ML? |
|---|---|---|
| `TestRender_MarkerOnFirstLine` | Marker na primeira linha de project e global | ✓ D1 |
| `TestRender_ScopeDiffers` | project ≠ global em exatamente SCOPE | ✓ D1 |
| `TestRender_ProjectIsMutable` | bytes.Clone previne mutação do asset | ✓ D1 |
| `TestRender_UnknownScopeReturnsError` | scope inválido → erro | ✓ D1 |
| `TestOpenCodeRead_SecretInFile_Blocked` | credOpenCodeReadScan lê file_path → exit 2 | ✓ D3 |
| `TestOpenCodeRead_NoHookSource_FileNotRead` | sem hook_source → arquivo não lido | ✓ D3 |
| `TestOpenCodeRead_WriteToolWithSecretFile_FileNotRead` | write tool → arquivo não lido | ✓ D3 |
| `TestOpenCodeRead_RelativePath_NotRead` | path relativo → não lido | ✓ D3 |
| `TestOpenCodeRead_DirectoryPath_NotRead` | diretório → não lido | ✓ D3 |
| `TestOpenCodeRead_InnocentFile_Allowed` | arquivo inocente → exit 0 | ✓ D3 |
| `TestOpenCodeShell_SecretInCommand_Blocked` | shell com segredo → exit 2 | ✓ D3 |
| `TestOpenCodeRead_GlobalScope_SecretInFile_Blocked` | --global lê file_path → exit 2 | ✓ D3 |
| `TestWriteScope_OpenCode_PluginsDir_SubagentDenied` | .opencode/plugins/ congelado | ✓ D4 |
| `TestWriteScope_OpenCode_OpencodeJson_SubagentDenied` | opencode.json congelado | ✓ D4 |
| `TestWriteScope_OpenCode_OpencodeJsonc_SubagentDenied` | opencode.jsonc congelado | ✓ D4 |
| `TestWriteScope_OpenCode_OutsidePluginsDir_Allowed` | fora de plugins/ → permitido | ✓ D4 |
| `TestWriteScope_OpenCode_MainSession_Allowed` | sessão principal → permitido | ✓ D4 |
| JS `arm1_shell_secret_throws` | shell + segredo → throw rc≠0, não ENOENT | ✓ D2/D3 — mas conclusão declara "operation blocked" não asserta; ver A5 |
| JS `arm2_innocuous_passes` | comando inocente → exit 0, stdout vazio | ✓ D2 |
| JS `arm3_enoent_throws` | sem trackfw no PATH → ENOENT → throw | ✓ D2 |
| JS `arm4_write_self_throws` | write ao plugin via path explícito → D4 antes do spawnSync | ✓ D4 (escopo: args.path; M17 aberto para patchText — ver A7) |
| JS `arm5_read_payload_structure` | payload tem hook_source:"opencode" + file_path absoluto | ✓ D3 |
| `TestOpenCodePlugin_NoOpenCodeDir_Silent` | sem .opencode/ → silencioso | ✓ D5 |
| `TestOpenCodePlugin_PluginAbsent_Warns` | plugin ausente → warn "trackfw update" | ✓ D5 |
| `TestOpenCodePlugin_Stale_Warns` | plugin stale → warn | ✓ D5 |
| `TestOpenCodePlugin_MissingMarker_Warns` | sem marker → warn | ✓ D5 |
| `TestOpenCodePlugin_Canonical_Silent` | canonical → silencioso | ✓ D5 |
| `TestOpenCodePlugin_GlobalAbsent_Warns` | projeto sim, global não → advisory | ✓ D6 |
| `TestOpenCodePlugin_GlobalPresent_Silent` | global presente com marker → silencioso | ✓ D6 |
| `TestOpenCodePlugin_ProjectAbsent_D6Silent` | projeto ausente → D6 silencioso | ✓ D6 |
| `TestOpenCodePlugin_ValidateForTest_Absent_IsWarning` | D5 via applyRuleWarnOnly | ✓ D5 |
| `TestOpenCodeConfigDir_*` (8 testes) | precedência OPENCODE_CONFIG_DIR > XDG > home | ✓ R-1 |
| `TestOSCLIHomeResolver_WindowsBranch_OpenCode` | USERPROFILE no Windows | ✓ D5 |
| `TestInjectOpenCodePlugin_Create` | Marker + SCOPE="project" ao criar | ✓ D1/D5 |
| `TestInjectOpenCodePlugin_Idempotent` | segunda chamada = no-op | ✓ D5 |
| `TestInjectOpenCodePlugin_Overwrites` | stale → sobrescreve | ✓ D5 |
| check-update-parity 5c | opencode-guard em HarnessTargetIDs | ✓ D6 |
| check-update-parity 5c-2 | state=missing sem --install-missing | ✓ D6 |
| check-update-parity 5c-3 | --install-missing grava SCOPE="global" | ✓ D6 |

---

## Mensagens de usuário

Todas as mensagens do `validator`, `doctor` e `auditsurface` introduzidas neste commit estão
em inglês. ✓

---

## Contratos antigos em scripts

`scripts/check-update-parity.sh` atualizado com cenários 5c/5c-2/5c-3. O contrato antigo
`contra_opencode_still_skips` foi corretamente invertido. Nenhum contrato obsoleto ativo. ✓

---

## Resultado por severidade

| ID | Severidade | Natureza | Bloqueia merge? |
|---|---|---|---|
| A1 | — | Retirado (finding falso) | — |
| A2 | Baixo | Racional R-1 falso; gap D4 para OPENCODE_CONFIG_DIR relativo (limitado — `__filename` protege instância global) | Não |
| A3 | Ruído | CODEX_HOME em testes OpenCode (copy-paste harmless) | Não |
| A4 | Baixo | openCodePluginRelPath em 3 locais; auditsurface.Marker local sem test pin; fix via export de opencodeplugin (stdlib only, sem ciclo) | Não |
| A5 | Baixo | arm1: "operation blocked" declarado mas não asserta — Regra Dura de Reconciliação | Não |
| A6 | Baixo | cli-parity.md cita `execa` (deve ser `spawnSync`) e nega `credOpenCodeReadScan` | Não |
| **A7** | **Médio** | **D4 patch/apply_patch: alvo em patchText bypassa selfProtect — confirmado empiricamente; ADR D4 lista as ferramentas como cobertas sem qualificar M17** | **SIM** |
| A8 | Baixo | update.go comentário invertido sobre filepath.Dir + re-leitura de env | Não |
| A9 | Nota | D5 warn-only não escalável (design choice não documentado no ADR) | Não |

**A7 bloqueia o merge.** Remediação: ML novo em REQ-2026-11-17 com Opção A (fix implementation)
ou Opção B (amend ADR D4 residual) — escolha do arquiteto.

**REPROVA.**
