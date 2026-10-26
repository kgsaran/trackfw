# Wave 0 — Modelo de Ameaça: aviso de presença do credential guard nos CLIs sem regra de presença

> REQ: REQ-2026-10-25 · ADR D17 · Issue #619 · Agente: hades-tf · Data: 2026-10-25
> Medições: binário `trackfw 11.0.2` em diretórios isolados (`mktemp -d`, HOME isolado)

---

## 1. Completude da enumeração

### 1.1 Superfícies declaradas no roadmap

O roadmap declara 8 CLIs com arquivos de hook de projeto. A enumeração é fechada ou faltam superfícies?

| CLI | Arquivo de hook de projeto | Na `credentialGuardHookFiles` | No gerador (init medido) |
|-----|---------------------------|-------------------------------|--------------------------|
| Claude Code | `.claude/settings.json` | linha 75 | sim |
| Codex CLI | `.codex/hooks.json` | linha 76 | sim |
| Gemini CLI | `.gemini/settings.json` | linha 77 | sim |
| Cursor | `.cursor/hooks.json` | linha 78 | sim |
| GitHub Copilot CLI | `.github/hooks/trackfw-attention.json` | linha 79 | sim |
| Kiro | `.kiro/hooks/trackfw-attention.json` | linha 80 | sim |
| Windsurf | `.windsurf/hooks.json` | linha 100 | sim |
| Amazon Q | `.amazonq/cli-agents/q_cli_default.json` | linha 108 | sim |

Fonte: `internal/validator/validator_credential_guard.go:72–109`.

A lista em `credentialGuardHookFiles` é a lista fechada canônica do produto (comentário na linha 65). Os 8 CLIs estão cobertos. A enumeração é fechada.

### 1.2 Pares (evento, matcher) bloqueantes por CLI

Medidos com `trackfw 11.0.2 init` e `trackfw update` em diretórios isolados (`mktemp -d`, HOME isolado). "Matcher" = string exata como o gerador escreve no JSON. "Bloqueante" = o CLI interrompe a ferramenta se o hook retorna exit diferente de 0.

#### Claude Code (`.claude/settings.json`) — gerador linhas 389, 391, 392

Schema: `hookCmdSchemaNested` (`validator_guard_wiring.go:91`). Struct: `requiresCommandType=true`, `family=PSPosix`. `claudeShellMatcher = "Bash|PowerShell"` definido em `agentfiles.go:509`.

| Evento | Matcher (string exata) | Bloqueante | Gerador |
|--------|------------------------|------------|---------|
| PreToolUse | `Bash|PowerShell` | sim (exit 2) | linha 389 |
| PreToolUse | `Read` | sim | linha 391 |
| PreToolUse | `Write|Edit` | sim | linha 392 |
| PostToolUse | `Bash|PowerShell` | nao | linha 402 |
| PostToolUse | `Read` | nao | linha 403 |
| PostToolUse | `Write|Edit` | nao | linha 404 |

Confirmado no `.claude/settings.json` deste repositorio: PreToolUse + `Bash|PowerShell`, `Read`, `Write|Edit` todos presentes.

#### Codex CLI (`.codex/hooks.json`) — gerador linhas 679, 680

Schema: `hookCmdSchemaNested` (`validator_guard_wiring.go:92`). Struct: `requiresCommandType=true`, `family=PSPosix`. Nota: Codex usa `"Bash"` (nao `"Bash|PowerShell"`) — so bloqueia Bash.

| Evento | Matcher | Bloqueante | Gerador |
|--------|---------|------------|---------|
| PreToolUse | `Bash` | sim (exit 2) | linha 679 |
| PreToolUse | `apply_patch` | sim | linha 680 |
| PostToolUse | `Bash` | nao | linha 687 |
| PostToolUse | `apply_patch` | nao | linha 688 |

Confirmado no `.codex/hooks.json` deste repositorio: PreToolUse + `Bash`, `apply_patch` presentes.

#### Gemini CLI (`.gemini/settings.json`) — gerador linhas 828, 829, 830

Schema: `hookCmdSchemaNested` (`validator_guard_wiring.go:93`). Struct: `requiresCommandType=true`, `family=PSPosix`.

| Evento | Matcher | Bloqueante | Gerador |
|--------|---------|------------|---------|
| BeforeTool | `run_shell_command` | sim (exit 2) | linha 828 |
| BeforeTool | `read_file|read_many_files` | sim | linha 829 |
| BeforeTool | `write_file|replace` | sim | linha 830 |
| AfterTool | `run_shell_command` | nao | linha 837 |
| AfterTool | `read_file|read_many_files` | nao | linha 838 |
| AfterTool | `write_file|replace` | nao | linha 839 |

Confirmado no `.gemini/settings.json` deste repositorio: BeforeTool + os tres matchers presentes.

#### GitHub Copilot CLI (`.github/hooks/trackfw-attention.json`) — gerador linhas 1105, 1120, 1127

Schema: `hookCmdSchemaFlat` (`validator_guard_wiring.go:95`). DIFERENCA CRITICA: o campo executavel e `entry["command"]` (nao `entry["hooks"][i]["command"]`). O schema `hookCmdSchemaFlatBash` (`validator_guard_wiring.go:79`) usa `entry["bash"]` e e EXCLUSIVO do arquivo GLOBAL do Copilot (`~/.copilot/settings.json`). O arquivo de projeto usa `hookCmdSchemaFlat` com `entry["command"]`. Struct: `requiresCommandType=true`, `family=PSPosix`.

| Evento | Matcher | Bloqueante | Gerador |
|--------|---------|------------|---------|
| preToolUse | `bash` | sim (exit 2 documentado — R1) | linha 1105 |
| preToolUse | `view` | sim | linha 1120 |
| preToolUse | `create|edit` | sim | linha 1127 |
| postToolUse | `bash` | nao | linha 1112 |
| postToolUse | `view` | nao | linha 1134 |
| postToolUse | `create|edit` | nao | linha 1141 |

Medido (update em fixture attention-only): `preToolUse + bash`, `view`, `create|edit` confirmados escritos.

#### Kiro (`.kiro/hooks/trackfw-attention.json`) — gerador linhas 940–988

Schema: `hookCmdSchemaKiro` (`validator_guard_wiring.go:98`). Estrutura: array raiz de objetos, com o comando em `entry["action"]["command"]`. Struct: `requiresCommandType=true`, `family=CmdExe`.

| Evento (trigger) | Matcher | Bloqueante | Gerador |
|------------------|---------|------------|---------|
| PreToolUse | `shell` | sim | linha 946 |
| PreToolUse | `read` | sim | linha 965 |
| PreToolUse | `write` | sim | linha 979 |
| PostToolUse | `shell` | nao | linha 953 |
| PostToolUse | `read` | nao | linha 972 |
| PostToolUse | `write` | nao | linha 986 |

Nota: Kiro armazena o comando em `entry["action"]["command"]`, nao em `entry["command"]`. O extrator de presenca deve usar o ramo `hookCmdSchemaKiro`, nao flat nem nested.

#### Cursor (`.cursor/hooks.json`) — gerador linhas 1321–1322, 1332–1335

Schema: `hookCmdSchemaFlat` (`validator_guard_wiring.go:96`). Struct: `requiresCommandType=false`, `family=PSPosix`.

| Evento | Matcher | Bloqueante | Gerador |
|--------|---------|------------|---------|
| beforeShellExecution | (nenhum) | sim (exit 2 = deny) | linha 1321–1322 |
| preToolUse | `Read` | incerto — R2 | linha 1332–1333 |
| preToolUse | `Write` | incerto — R2 | linha 1334–1335 |
| afterShellExecution | (nenhum) | nao | linha 1323–1324 |
| postToolUse | `Read` | nao | linha 1336–1337 |
| postToolUse | `Write` | nao | linha 1338–1339 |

Medido (init e update em fixture attention-only): `migrateDedupGuardInFlatArray` (`guard_generations.go:43`) sempre adiciona a entrada canonica se ausente (Pass 3, linhas 71–77). Todos os tres pares (beforeShellExecution, preToolUse+Read, preToolUse+Write) sao escritos. Medicao anterior que reportou apenas `beforeShellExecution` estava incorreta (parser nao detectava entradas flat com matcher). A lista de presenca para Cursor deve incluir os 3 pares. R2 refere-se ao comportamento de bloqueio de preToolUse+Read/Write, nao a presenca.

#### Windsurf (`.windsurf/hooks.json`)

Ja coberto por `credentialGuardRequiredEntries` com violation.

#### Amazon Q (`.amazonq/cli-agents/q_cli_default.json`)

Ja coberto por `credentialGuardRequiredEntries` com violation.

### 1.3 Como a regra de violation atual escolhe os eventos

`validateCredentialGuardPresenceRequired()` (`validator_credential_guard.go:870`) itera `credentialGuardRequiredEntries` (linha 802). Para cada `(hookFile, events)`, chama `extractCredentialGuardCommandsFromEvent` (linha 824) com dois marcadores (`credentialGuardScriptMarker = "trackfw-credential-guard.sh"` linha 18; `credentialGuardSubcmdMarker = "trackfw guard credential"` linha 28). Eventos sem nenhum dos dois formam violation.

`extractCredentialGuardCommandsFromEvent` trata:
- `amazonQMatcher == ""` — flat sem matcher (Windsurf): `hooks[event]` flat array, `entry["command"]`.
- `amazonQMatcher != ""` — nested + exact-string matcher (Amazon Q): localiza por `"matcher"` e le `entry["hooks"][i]["command"]`.

Para os 6 novos CLIs, esse extrator nao funciona:
- Claude/Codex/Gemini: `hookCmdSchemaNested` — ramo inexistente.
- Copilot: `hookCmdSchemaFlat` com matcher — o ramo flat existe mas nao filtra por matcher.
- Kiro: `hookCmdSchemaKiro` com `action.command` — ramo inexistente.
- Cursor `beforeShellExecution`: flat sem matcher — funciona (mesmo caso Windsurf).
- Cursor `preToolUse + Read/Write`: flat com matcher — ramo flat existe mas nao filtra por matcher.

Semantica de matcher: string exata gerada. `"Bash|PowerShell"` e diferente de `"Bash"`. Um arquivo com matcher `"Bash"` nao satisfaz o par `Bash|PowerShell` — aviso dispara, `trackfw update` migra para `"Bash|PowerShell"`, aviso desaparece.

Extensoes necessarias para o ponto unico:

1. Campo `warnOnly bool` em `credentialGuardRequiredEntry` — false para Windsurf/Amazon Q, true para os 6.
2. Campo `matcher string` em `credentialGuardRequiredEvent` — string exata do gerador.
3. Campo `schema hookCmdSchemaKind` em `credentialGuardRequiredEntry`.
4. Extensao de `extractCredentialGuardCommandsFromEvent`: ramo nested+string-matcher (generalizar Amazon Q para Claude/Codex/Gemini), flat+string-matcher (Copilot, Cursor preToolUse), Kiro `action.command`.
5. `validateCredentialGuardPresenceRequired` usa `applyRuleWarnOnly` para `warnOnly=true`.

Sobre `applyRuleWarnOnly` (`validator.go:592`): quando `ruleSeverity == "off"` silencia; em qualquer outra severidade, sempre vai para warnings, nunca para violations. Ver §2.1 e R5.

### 1.4 Medicao: init/update por CLI, com e sem harness global

#### Pares escritos por init/update (6 CLIs a cobrir)

Metodo: `mktemp -d`, HOME isolado, `trackfw 11.0.2 init --ai-tools <cli>` e `trackfw update` em fixture attention-only. Parser por schema (nested: `entry["hooks"][i]["command"]`; flat: `entry["command"]`; Kiro: `entry["action"]["command"]`).

| CLI | Pares PreToolUse (bloqueantes) apos update | Confirmado por |
|-----|---------------------------------------------|----------------|
| Claude | PreToolUse + `Bash|PowerShell`, `Read`, `Write|Edit` | init + `.claude/settings.json` deste repo |
| Codex | PreToolUse + `Bash`, `apply_patch` | init + `.codex/hooks.json` deste repo |
| Gemini | BeforeTool + `run_shell_command`, `read_file|read_many_files`, `write_file|replace` | init + `.gemini/settings.json` deste repo |
| Cursor | `beforeShellExecution` (sem matcher), `preToolUse + Read`, `preToolUse + Write` | init + update medidos em fixture attention-only |
| Copilot | `preToolUse + bash`, `view`, `create|edit` | update medido em fixture attention-only |
| Kiro | PreToolUse + `shell`, `read`, `write` | init medido (linhas do gerador 946/965/979) |

#### D15 — project-scope sempre emitido, independente do harness global

O gerador contem o seguinte comentario explicito para Claude (`agentfiles.go:385–388`): "project-scope wiring is always emitted regardless of global guard state. No generator consults global to skip or remove project-scope entries." Comentarios equivalentes existem para cada CLI. `migrateDedupGuardInFlatArray` sempre adiciona a entrada canonica (Pass 3, `guard_generations.go:71–77`) mesmo quando o array de entrada esta vazio.

O novo aviso nao existe no binario 11.0.2, portanto o comportamento "com global instalado" para AC3 nao pode ser medido contra o binario atual. ML-1A deve verificar que `init`/`update` com harness global instalado nao dispara o aviso de presenca (AC3 — ver R6).

### 1.5 Testes e cenarios que afirmam "ausencia de guard nesses 6 CLIs = sem problema" e vao mudar

Metodo: grep mecanico por `.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`, `.cursor/hooks.json`, `.github/hooks/trackfw-attention.json`, `.kiro/hooks/trackfw-attention.json` nos arquivos `*_test.go` e `*.sh`, cruzado com assercoes de `len(msgs/warnings)==0`, `rc==0` ou ausencia de mensagem de aviso.

Arquivos com hits: `validator_credential_guard_test.go`, `validator_guard_d16_wiring_external_test.go`, `cursor_guard_migrate_dedup_external_test.go`, `validator_guard_hook_concordance_external_test.go` (extensao necessaria), `scripts/check-gates-falsify.sh`.

| Arquivo | Linha | Teste / cenario | O que afirma | Quebra apos ML-1A? |
|---------|-------|-----------------|--------------|-------------------|
| `validator_credential_guard_test.go` | 88 | `NaoDisparaSemEntrada` | Claude com attention-only, sem guard, len(msgs)==0 | SIM — 3 warnings de presenca disparam |
| `validator_credential_guard_test.go` | 119 | `NaoDisparaFormatoDesconhecido` | Claude PreToolUse+Bash + $SOME_OTHER_VAR, len(msgs)==0 | SIM — "Bash" diferente de "Bash|PowerShell", mais Read/Write ausentes |
| `validator_credential_guard_test.go` | 306 | `KiroNaoGeraFalsoPositivoComCamposVizinhos` | Kiro com so PreToolUse+shell (sem read/write), len(msgs)==0 | SIM — warnings para read e write ausentes |
| `validator_credential_guard_test.go` | 627 | `CaminhoAbsolutoSilencioso` | Claude PreToolUse+Bash, caminho absoluto, len(msgs)==0 | SIM — "Bash" diferente de "Bash|PowerShell", mais 2 pares ausentes |
| `validator_credential_guard_test.go` | 645 | `OutraVarSilenciosa` | Claude PreToolUse+Bash, $MY_CUSTOM_DIR, len(msgs)==0 | SIM — mesma razao |
| `validator_credential_guard_test.go` | 663 | `CodexFormaSilenciosa` | Codex PreToolUse + .* (wildcard), len(msgs)==0 | SIM — ".*" diferente de "Bash" e de "apply_patch" |
| `validator_guard_d16_wiring_external_test.go` | 785 | `TestCredentialGuardPresenceRequired_Kiro_NaoTemRegraDePresencaDisco` | Ausencia de msg com ".kiro" + "has no credential guard entry" | SIM — nova mensagem contera essas strings |
| `cursor_guard_migrate_dedup_external_test.go` | 159 | AC1 apos InjectCursorHooks | len(warnings)==0 | NAO QUEBRA se InjectCursorHooks escreve beforeShellExecution + preToolUse+Read/Write com guard — verificar antes de ML-1A (R3) |
| `validator_credential_guard_test.go` | 924 | `NaoDisparaFormaRelativaEmCursor` | Cursor com beforeShellExecution, script presente, len(msgs)==0 | NAO QUEBRA se fixture inclui preToolUse+Read/Write com guard; QUEBRA se nao inclui (R3) |
| `scripts/check-gates-falsify.sh` | 3988–4024 | Cenario 47 baseline | rc==0 E ausencia de S47_MSG_MISSING | NAO QUEBRA sintaticamente (rc=0, nova msg diferente de S47_MSG_MISSING). Deriva semantica: fixture legado ("Bash", 1 par) deixa de representar guard correto |

Os testes que assertam uma **violation especifica** (nao zero msgs) nao quebram — a violation continua presente, os novos warnings so adicionam mensagens. Ex.: `CaminhoResolvidoEhFisicoNaoSimlink:353` passa.

Acao recomendada em ML-1A: atualizar `s47_write_claude_guard_hook` (`scripts/check-gates-falsify.sh:1383–1389`) para escrever os 3 pares modernos. Verificar `InjectCursorHooks` antes de ML-1A.

---

## 2. Modelo de ameaca

O adversario aqui nao e um atacante externo: e o implementador apressado que entrega AC2 (rc nao muda) sem verificar se o aviso chega ao usuario nas configuracoes relevantes, e o arquiteto otimista que le "AC3 satisfeito — ruido zero no caso gerado" sem medir o `update` ou sem confrontar cada par do gerador com a lista de presenca.

### 2.1 Silenciamento via `rules: off` — aviso que nao chega ao usuario

`applyRuleWarnOnly` (`validator.go:592`) silencia quando `ruleSeverity(ruleName) == "off"` (linha 593). Um projeto que configura `rules: <nome-da-regra>: off` no `trackfw.yaml` nao recebe o aviso.

`lenientCarveoutRules` (`validator.go:617`) e o conjunto de violations que sobrevivem ao modo lenient. Isso nao se aplica a `applyRuleWarnOnly` — mensagens ja em warnings nao sao afetadas por `lenientCarveoutRules`. Nao ha mecanismo no produto para tornar um aviso insilenciavel via configuracao.

Precedente existente: `validateGuardHookShLegacyWarnings` ja usa `applyRuleWarnOnly("credential_guard_hook_resolvable", ...)` (`validator.go:1066`). Pode ser silenciado com `rules: credential_guard_hook_resolvable: off`. O novo aviso de presenca, usando o mesmo ou nome proprio, herda esse comportamento. Aceito como residual R5.

### 2.2 Extrator aceita guard em evento errado (falso negativo)

Se `extractCredentialGuardCommandsFromEvent` nao filtra pelo nome do evento, um hook com guard apenas em PostToolUse seria detectado como presente em PreToolUse, e o aviso nao dispararia.

Porta de entrada: o ramo para `hookCmdSchemaNested` deve acessar `hooks[event.hookEvent]` primeiro, depois filtrar por `entry["matcher"] == event.matcher`, depois ler `entry["hooks"][i]["command"]`. Omitir o filtro por evento e o erro.

Mitigacao: AC4 — teste de "guard so em PostToolUse produz aviso" (minimo um por CLI).

### 2.3 Lista de presenca diverge do gerador (warning que update nao resolve)

Se `credentialGuardRequiredEntries` declara um par com matcher diferente do que o gerador escreve (ex.: `"Bash|PowerShell"` na lista mas gerador escreve `"Bash"` para Codex), `trackfw update` nao resolve o aviso — warning permanente.

Mitigacao: AC3 por CLI — `update` em fixture attention-only + `validate`, zero warnings de presenca. Os matchers na lista de presenca devem ser copiados da tabela em §1.2, nao inferidos.

### 2.4 Extrator Kiro usa ramo errado (`action.command` vs. `command`)

Kiro armazena o comando em `entry["action"]["command"]`, nao em `entry["command"]` (schemaFlat) nem em `entry["hooks"][i]["command"]` (schemaNested). Um extrator que usa o ramo errado retorna nil para todos os arquivos Kiro — falso positivo permanente.

Falso negativo (Kiro): um adversario pode colocar o marcador em `entry["command"]` (nivel raiz do objeto, fora de `action`) — campo que o Kiro nao executa. Se o extrator le `entry["command"]` em vez de `entry["action"]["command"]`, o guard e reportado como presente mas nunca executa. AC4 deve incluir: marcador em `entry["command"]` (sem `action`) produz aviso.

Mitigacao falso positivo: AC3 para Kiro — `init` + `validate` sem warnings.

### 2.5 Extrator Copilot usa `hookCmdSchemaFlatBash` em vez de `hookCmdSchemaFlat`

O arquivo de projeto Copilot usa `hookCmdSchemaFlat` — comando em `entry["command"]`. O arquivo GLOBAL do Copilot usa `hookCmdSchemaFlatBash` — comando em `entry["bash"]`. Se o extrator de presenca usar `bash` em vez de `command` para o projeto, o guard nunca e encontrado — falso positivo para todo projeto Copilot.

Mitigacao: AC3 para Copilot. `collectCommandsFromCommandKeyArray` (`validator_credential_guard.go:320`) ja le `entry["command"]` corretamente para `hookCmdSchemaFlat`.

### 2.6 Deriva semantica do Cenario 47

O braco baseline do Cenario 47 passa sintaticamente apos ML-1A mesmo com o fixture legado. A proxima auditoria pode presumir incorretamente que o fixture representa "guard correto moderno".

Mitigacao: atualizar `s47_write_claude_guard_hook` em ML-1A (ver §1.5).

### 2.7 Falso negativo de group["command"] — nivel de grupo vs. nivel interno

Para `hookCmdSchemaNested`, o comando executavel esta em `group["hooks"][j]["command"]` (nivel interno). Um campo `group["command"]` no nivel do grupo e ignorado pelo Claude SDK. Se o extrator le `group["command"]` (errado), um adversario pode colocar o marcador la (onde nao executa) e o check reporta guard presente — falso negativo.

Mitigacao: AC4 deve incluir "marker em group['command'] (nivel de grupo), nao em hooks[j]['command']" para schemaNested. O `collectCommandsWithMarkerBySchema` existente ja le `innerObj["command"]` na linha 409 — o extrator de presenca deve usar o mesmo caminho.

---

## 3. Alvos de falsificacao nas duas direcoes

### Superfície A — Presenca do aviso por par (CLI, evento, matcher)

Falso negativo (aviso deveria disparar, nao dispara):
- Guard em PostToolUse mas nao em PreToolUse — AC4 detecta.
- Guard em `group["command"]` nivel de grupo (nested), ou `entry["command"]` fora de `action` (Kiro) — AC4 caso especial.
- Implementador apressado: escreve AC3 (no noise) sem AC4 (positive case). Deteccao: Regra Dura de Reconciliacao — relatorio ML-1A declara frase do teste por AC4.

Falso positivo (aviso dispara quando guard esta presente):
- Extrator usa ramo errado por schema (Kiro: `command` em vez de `action.command`; Copilot: `bash` em vez de `command`) — AC3 detecta.
- Matcher wildcard `".*"` ou `""` no arquivo cobre todos os eventos, mas lista de presenca usa string exata — aviso dispara. Aceito: `update` corrige para a string canonica; warning resolvivel.

### Superfície B — Invariancia do rc (AC2)

Falso negativo (rc muda sem que o teste detecte):
- Aviso registrado via `applyRule` em vez de `applyRuleWarnOnly` — em severity default, vai para violations, rc=1.
- Gate: AC2 asserta `rc == 0` em modo strict (sem flag lenient).
- Implementador apressado: testa em modo lenient (onde violations nao-carveout tambem viram aviso e rc=0).

### Superfície C — Windsurf e Amazon Q mantem violation (AC5 — regressao)

Falso negativo (violation deixa de disparar):
- Ao acrescentar `warnOnly bool`, o campo default para `true`, ou os entries existentes sao reescritos com `warnOnly=true`.
- Gate: testes pre-existentes de Windsurf/Amazon Q violation passam sem modificacao de fixture. Contra-braco em `origin/main`.

### Superfície D — Lista compartilhada como ponto unico (AC5)

Falso negativo (CLI sem entrada na lista):
- 5 dos 6 CLIs adicionados; um omitido.
- Gate: teste de concordancia bidirecional — todo CLI de `credentialGuardHookFiles` (exceto Windsurf/Amazon Q) tem entrada `warnOnly=true` na lista, e vice-versa.

Falso positivo (lista declara par que gerador nao escreve):
- Divergencia de matcher string — AC3 por `update` pega.

### Superfície E — Schema correto por extrator

Falso positivo (guard presente, extrator nao encontra):
- Kiro sem ramo `action.command` — warning sempre.
- Copilot com `entry["bash"]` em vez de `entry["command"]` — warning sempre.
- Gate: AC3 por CLI.

Falso negativo (guard ausente, extrator reporta presente):
- `group["command"]` no nested: Ameaca 2.7. `entry["command"]` fora de `action` no Kiro: Ameaca 2.4.
- Gate: AC4 com casos especificos.

### Superfície F — `type:"command"` como discriminante

Para CLIs com `requiresCommandType=true` (Claude, Codex, Gemini, Copilot, Kiro), D16 exige `type:"command"` para o entry ser executavel. `collectCommandsFromCommandKeyArray` (`validator_credential_guard.go:320`) ja preserva `typeIsCommand` (linha 330). O ramo nested preserva `typeIsCommand` (linha 411). O extrator de presenca deve usar o mesmo criterio: entry sem `type:"command"` nao conta como guard presente.

Mitigacao: AC4 com caso "type omitido ou type:'approval' + marcador presente" — aviso dispara.

### Superfície G — Teste antigo e extrator com bug se reforcam (falso verde)

`NaoDisparaSemEntrada` nao quebra se o extrator de Claude nao funciona (aviso nao dispara, len(msgs)==0 continua verdadeiro).

Gate: AC1 — teste positivo explicito por CLI. Se o extrator nao funciona, AC1 falha.

---

## 4. Residual declarado, files affected e ML-3A pre-check

### 4.1 Residual declarado

R1 — Copilot exit code nao medido ao vivo. Documentacao cita exit 2 como rejeicao, nao medido com a ferramenta instalada. Aceito.

R2 — Cursor `preToolUse + Read/Write` — comportamento de bloqueio nao confirmado. O gerador escreve os pares (confirmado). Se bloqueiam, a lista de presenca ja os inclui. Se nao bloqueiam, o aviso e inocuo e resolvivel por `trackfw update`. Aceito.

R3 — `InjectCursorHooks` pode nao escrever os 3 pares. Verificar antes de ML-1A se `InjectCursorHooks` escreve `preToolUse+Read` e `preToolUse+Write`. Se nao escrever, `NaoDisparaFormaRelativaEmCursor` quebra.

R4 — Amazon Q sem escopo global. Confirmado.

R5 — Silenciamento por `rules: off` e possivel e aceito. `applyRuleWarnOnly` silencia com `off`. `lenientCarveoutRules` aplica-se somente a violations, nao a aviso-only. Nao ha mecanismo de insilenciabilidade para warnings. O novo aviso compartilha esse comportamento com todos os avisos existentes (ex.: `credential_guard_hook_resolvable` legacy, `validator.go:1066`). Aceito — consistente com o produto.

R6 — AC3 com harness global nao medido contra binario atual. O aviso nao existe em 11.0.2. D15 (codigo do gerador, `agentfiles.go:385–388` e equivalentes) garante que project-scope e sempre emitido. ML-1A deve testar `init`/`update` com harness global instalado + `validate` sem aviso de presenca.

### 4.2 ML-3A pre-check — hook files neste repositorio

Verificado read-only (`ls -la`):

| Hook file | Presente neste repo? | Pares PreToolUse com guard (confirmado) |
|-----------|---------------------|-----------------------------------------|
| `.claude/settings.json` | sim | `Bash|PowerShell`, `Read`, `Write|Edit` — todos presentes |
| `.codex/hooks.json` | sim | `Bash`, `apply_patch` — ambos presentes |
| `.gemini/settings.json` | sim | `run_shell_command`, `read_file|read_many_files`, `write_file|replace` — todos presentes |
| `.cursor/hooks.json` | nao existe | — |
| `.github/hooks/trackfw-attention.json` | nao existe | — |
| `.kiro/hooks/trackfw-attention.json` | nao existe | — |

Apos ML-1A, ML-3A verifica `trackfw validate` neste repositorio (binario da branch) sem avisos de presenca para os 3 arquivos existentes. Os 3 estao completos — segundo criterio de aceite do ML-3A nao causara surpresa.

### 4.3 Files affected recomendados para Wave 1 (ML-1A)

| Arquivo | Tipo de alteracao |
|---------|-------------------|
| `internal/validator/validator_credential_guard.go` | Campos `warnOnly bool`, `matcher string`, `schema hookCmdSchemaKind` nas structs; extensao de `extractCredentialGuardCommandsFromEvent` com ramos para nested+string-matcher (generalizar Amazon Q para Claude/Codex/Gemini), flat+string-matcher (Copilot, Cursor preToolUse), Kiro; 6 novas entradas em `credentialGuardRequiredEntries`; emissao via `applyRuleWarnOnly` para `warnOnly=true` |
| `internal/validator/validator_credential_guard_test.go` | Atualizar 6 testes da §1.5 que quebram; AC1 (aviso por CLI), AC2 (rc=0 strict), AC4 (guard PostToolUse, group-level, type:command, Kiro outer-command), concordancia com gerador |
| `internal/validator/validator_guard_d16_wiring_external_test.go` | Atualizar `TestCredentialGuardPresenceRequired_Kiro_NaoTemRegraDePresencaDisco` — afirmar aviso, nao violation, rc=0 |
| `internal/validator/validator_guard_hook_concordance_external_test.go` | Concordancia bidirecional `credentialGuardHookFiles` ↔ `credentialGuardRequiredEntries`; verificar `warnOnly` por categoria |
| `docs/cli-parity.md` | Documentar aviso de presenca e que `rules: <nome>: off` pode silencia-lo (R5) |
| `scripts/check-gates-falsify.sh` | Atualizar `s47_write_claude_guard_hook` (linhas 1383–1389) para escrever 3 pares modernos |
