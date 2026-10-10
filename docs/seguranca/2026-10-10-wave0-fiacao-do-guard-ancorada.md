# Wave 0 — Threat Model: Fiação do Guard Ancorada no origin/main

> Reviewer: `hades-tf` | Data: 2026-10-10
> REQ: `docs/req/REQ-2026-09-02-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md`
> ADR em análise: `docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md`

---

## 1. Completude da população (ADR D3)

**Objetivo:** confirmar que a lista de arquivos de hook de projeto com guard é fechada e coincide com o que o gerador escreve.

### 1.1 Lista normativa (`credentialGuardHookFiles`)

Fonte: `internal/validator/validator_credential_guard.go:76-113`.

| # | Arquivo (relativo à raiz do projeto) | CLI | requiresCommandType | family |
|---|---|---|---|---|
| 1 | `.claude/settings.json` | Claude Code | true | PS/POSIX |
| 2 | `.codex/hooks.json` | Codex CLI | true | PS/POSIX |
| 3 | `.gemini/settings.json` | Gemini CLI | true | PS/POSIX |
| 4 | `.cursor/hooks.json` | Cursor | false | PS/POSIX |
| 5 | `.github/hooks/trackfw-attention.json` | GitHub Copilot CLI | true | PS/POSIX |
| 6 | `.kiro/hooks/trackfw-attention.json` | Kiro | true | cmd.exe |
| 7 | `.windsurf/hooks.json` | Windsurf | false | PS/POSIX |
| 8 | `.amazonq/cli-agents/q_cli_default.json` | Amazon Q | false | cmd.exe |

Os dois guards são colocados nas 8 entradas: `credential` e `git-branch`. Ambos usam
`validateGuardHookResolvable` com a mesma lista (`credentialGuardHookFiles`).

### 1.2 Verificação contra o gerador

**Comandos canônicos** (fonte: `internal/generators/agentfiles.go:502-507`):

```
guardGitBranchCmdPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
guardCredentialCmdPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
guardGitBranchCmdCmdExe   = "trackfw guard git-branch || exit 2"
guardCredentialCmdCmdExe  = "trackfw guard credential || exit 2"
```

Linha esperada calculada por `guardExpectedLine()` (`validator_guard_binary_probe.go:49-54`).

Buscando `guardGitBranchCmdPSPOSIX` e `guardCredentialCmdPSPOSIX` nos `InjectXXXHooks`:

| Inject function | credential | git-branch | Observação |
|---|---|---|---|
| `InjectClaudeHooks` (L249) | sim | sim | dedup global |
| `InjectCodexHooks` (L586) | sim | sim | dedup global |
| `InjectGeminiHooks` (L756) | sim | sim | dedup global |
| `InjectCursorHooks` (L1303) | sim | sim | dedup global; usa `mergeCursorGuardMatcherEntry` |
| `InjectCopilotHooks` (L1111) | sim | sim | dedup global |
| `InjectKiroHooks` (L954) | sim | sim | reescreve arquivo inteiro; arquivos separados p/ global |
| `InjectWindsurfHooks` (L1655) | sim (`pre_run_command`+`pre_write_code`) | sim (`pre_run_command`) | dedup global via `credentialGuardGlobalInstalledWindsurf` |
| `InjectAmazonQHooks` (L1832) | sim (`execute_bash`+`fs_write`) | sim (`execute_bash`) | sem dedup global (R4 declarado) |

**Resultado:** a lista de 8 arquivos em `credentialGuardHookFiles` está fechada — cada arquivo é
escrito por exatamente um `InjectXXX` que inclui ambos os guards. Nenhum `InjectXXX` escreve guard
em arquivo fora desta lista (busca de `guardGitBranchCmdPSPOSIX`/`guardCredentialCmdPSPOSIX` em
`agentfiles.go` não retornou outros arquivos de destino).

### 1.3 Tuples do origin/main desta instância

Medido com:
```bash
git show origin/main:.claude/settings.json | python3 -I -c "
import sys, json
d = json.load(sys.stdin)
h = d.get('hooks', {})
for ev in h:
    for e in (h[ev] if isinstance(h[ev], list) else []):
        m = e.get('matcher','')
        for inner in e.get('hooks', []):
            cmd = inner.get('command','')
            if 'guard' in cmd.lower() or 'trackfw' in cmd.lower():
                print(f'{ev}|{m}|{cmd[:80]}')
"
```

Resultado (9 tuples):

```
PostToolUse|AskUserQuestion|$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh
PostToolUse|Bash|PowerShell|$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; …
PostToolUse|Read|$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; …
PostToolUse|Write|Edit|$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; …
PreToolUse|AskUserQuestion|$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh
PreToolUse|Bash|PowerShell|$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; …
PreToolUse|Bash|PowerShell|$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; …
PreToolUse|Read|$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; …
PreToolUse|Write|Edit|$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; …
```

Observação: o guard de projeto está presente em `origin/main`. O dedup por global guard NÃO foi
ativado nesta instância (wiring de projeto gerado normalmente).

**Conclusão da seção 1:** a lista D3 está fechada. A AC0 deste tópico está satisfeita.

---

## 2. Threat Model

**Adversário:** agente com escrita irrestrita ao workspace (o adversário desta REQ, conforme
ADR-2026-08-12) ou implementador que remove a fiação por acidente. O adversário com permissão de
merge em `main` é o limite declarado do ADR-2026-08-12 e não é tratado aqui.

### 2a. Arquivos lidos pelo CLI e não rastreados / chaves de desligamento global

#### Claude Code — `.claude/settings.local.json`

**Achado:** A chave `disableAllHooks: true` em qualquer arquivo de settings do Claude Code
desabilita TODA execução de hooks. Fonte: `https://docs.anthropic.com/claude/docs/claude-code/settings`
(confirmado: "The `disableAllHooks` setting is a boolean that, when set to true, disables all hook
and statusLine execution").

O arquivo `.claude/settings.local.json` é git-ignored globalmente via `~/.config/git/ignore`.
Medido:
```bash
git check-ignore -v .claude/settings.local.json
# saída: /Users/kgsaran/.config/git/ignore:1:**/.claude/settings.local.json  .claude/settings.local.json
git status --short --ignored .claude/ | head -5
# saída: !! .claude/settings.local.json
```

**Vetor:** Adversário escreve `{"disableAllHooks": true}` em `.claude/settings.local.json`. O
arquivo rastreado `.claude/settings.json` continua intacto com todas as tuples. A regra proposta
(D2) não detecta este bypass porque compara apenas tuples no arquivo rastreado.

**Vetor alternativo (arquivo rastreado):** `disableAllHooks: true` pode também ser inserido no
próprio `.claude/settings.json` (arquivo rastreado). Neste caso, as tuples permanecem; a rule D2
não detecta a chave de desabilitação.

#### Claude Code — gestão pelo ADR

O ADR declara "contornos que não tocam o arquivo rastreado" como resíduo, mas não nomeia
`.claude/settings.local.json` nem `disableAllHooks` explicitamente. Necessário nomear (ver A1).

#### Gemini CLI — `hooksConfig.enabled`

**Achado:** Gemini CLI tem a chave `hooksConfig.enabled` (default `true`). Quando `false`, NENHUM
hook executa. Fonte: `https://geminicli.com/docs/hooks/reference/` (confirmado: "The 'Enable Hooks'
setting (hooksConfig.enabled) is the canonical toggle for the hooks system. When disabled, no hooks
will be executed. Default value is true").

O gerador (`InjectGeminiHooks`) NUNCA escreve a chave `hooksConfig`. Portanto, origin/main não
contém esta chave. Um adversário que adicione `"hooksConfig": {"enabled": false}` ao arquivo
`.gemini/settings.json` (rastreado) não alteraria nenhuma tuple — a rule D2 seria silenciosa.

**Vetor:** adversário adiciona `"hooksConfig": {"enabled": false}` ao arquivo rastreado
`.gemini/settings.json`. Tuples intactas. Rule silenciosa.

#### GitHub Copilot CLI — `disableAllHooks`

**Achado:** GitHub Copilot CLI tem a chave `disableAllHooks` no arquivo de hooks. Fonte:
`https://docs.github.com/en/copilot/reference/hooks-reference` (confirmado: "disableAllHooks: a
global hook disable toggle"). O arquivo rastreado é `.github/hooks/trackfw-attention.json`.

**Vetor:** adversário adiciona `"disableAllHooks": true` ao arquivo rastreado. Tuples intactas.
Rule D2 silenciosa.

#### Kiro — `"enabled": false` por entrada de hook

**Achado:** Kiro suporta o campo `"enabled": false` por entrada de hook (documentado em
`https://kiro.dev/docs/hooks/` — "The enabled field is optional and set to false to skip the hook
without deleting it"). O arquivo rastreado é `.kiro/hooks/trackfw-attention.json`, reescrito
integralmente pelo `InjectKiroHooks`.

**Vetor:** adversário edita o arquivo rastreado adicionando `"enabled": false` nos hooks de guard.
As tuples (evento, matcher, comando) permanecem. Rule D2 silenciosa — compara tuples, não o campo
`enabled`.

#### Codex CLI — `[features] hooks = false` em `config.toml`

**Achado:** Codex CLI desabilita todos os hooks via `[features] hooks = false` em `config.toml`
(fonte: `https://developers.openai.com/codex/hooks` — "To disable hooks in config.toml, set:
`[features] hooks = false`"). Este arquivo é `.codex/config.toml`, distinto de `.codex/hooks.json`
(o rastreado pela rule). Se `.codex/config.toml` estiver no repositório e rastreado, o bypass é
visível no diff e pode ser detectado por uma rule separada; se não for rastreado, é um vetor
residual.

**Vetor (parcialmente fora do alcance):** `.codex/config.toml` é um arquivo separado; sua presença
e rastreabilidade dependem de cada adotante. Residual nomeado (R3 — ver seção 4).

#### Windsurf, Amazon Q

Mecanismo de desabilitação global não encontrado na documentação pesquisada
(`windsurf` — DuckDuckGo/web inconcluso, `amazonq` — contexto hooks deprecated, agente hooks:
`/context hooks disable-all` é a API antiga). Amazon Q CLI foi marcado como depreciado em favor do
Kiro CLI em 2025-12-15 (fonte: docs.aws.amazon.com). `.amazonq/cli-agents/q_cli_default.json`
continua no `credentialGuardHookFiles` atual.

### 2b. Casamento do comando do guard

A rule proposta (D2) compara tuples. O validator já usa `guardExpectedLine()` para o formato
canônico. As formas reconhecidas pela lógica existente de `validateGuardHookResolvable` são:

| Forma | Status no validator atual |
|---|---|
| D11 revised (canônica) | OK — `anySubcmdFormFound = true` |
| D11 pré-ML-6C (legacy) | warning via `validateGuardHookD11LegacyWarnings` |
| D2 revised (legacy) | warning via `validateGuardHookD2InlineWarnings` |
| `echo trackfw guard credential` | não contém `guardCredentialSubcmdMarker` → ausência detectada |
| `true` | não contém o marcador → ausência detectada |

**Para a nova rule D2:** se a comparação de tuples usar a string exata de comando como 4ª componente,
uma fiação que existe em origin/main na forma D2-legacy e foi migrada para D11-revised no disco
seria reportada como AUSENTE → falso positivo. Mesmo raciocínio para migração de matcher
`"Bash"` → `"Bash|PowerShell"` (ML-5A, `migrateGuardHookMatcher`, agentfiles.go:357-360).

**Evidência da migração de matcher:** `InjectClaudeHooks` chama:
```go
// agentfiles.go:357-360
migrateGuardHookMatcher(hooks["PreToolUse"], "Bash", claudeShellMatcher,
    guardGitBranchCmdPSPOSIX, guardCredentialCmdPSPOSIX)
```
onde `claudeShellMatcher = "Bash|PowerShell"`. Esta chamada RENOMEIA o matcher `"Bash"` para
`"Bash|PowerShell"` em projetos que instalaram antes do ML-5A. Após `trackfw update`, o disco terá
`"Bash|PowerShell"`, mas origin/main (pré-ML-5A) ainda tem `"Bash"`. A tuple de origin/main
estaria AUSENTE do disco → falso positivo que persiste até a próxima PR.

**Conclusão:** a 4ª componente da tuple (comando) deve reconhecer formas equivalentes (D2, D11,
D11-revised). A componente matcher deve usar cobertura, não igualdade: `"Bash|PowerShell"` no
disco COBRE `"Bash"` em origin/main (ver A2).

### 2c. Os 4 estados do anchor aplicados a arquivos de hook

O discriminante de 4 estados em `originMainTrackfwYAML` (`validator_credential_guard_integrity.go:365`)
aplica-se por chamada `git show`. Para a nova rule, cada arquivo de hook usa uma chamada independente.

| Estado | Detecção | Comportamento proposto |
|---|---|---|
| `originAnchorNoGit` | não é worktree git | silêncio — não reachable via edição de arquivo |
| `originAnchorNoRemote` | sem remote `origin` | silêncio — **ver A3 abaixo** |
| `originAnchorRefUnreadable` | origin existe, ref ilegível | FAIL CLOSED |
| arquivo ausente em origin/main | ref ok, arquivo não existe | "nunca instalado" → silêncio |
| arquivo presente | ref ok, arquivo presente | compara tuples → violation se ausente |

**Working tree edit:** adversário remove tuple de `.claude/settings.json` no disco. origin/main
inalterado. Rule acusa. Sobrevive ao baseline (carve-out em `credentialGuardAnchoredRules`) e ao
lenient (carve-out em `lenientCarveoutRules`). **Correto** — se D5 for implementado.

**Branch (não mergeada):** mesmo que a remoção esteja commitada na branch do PR, origin/main
continua intacta → rule acusa no CI do PR. **Correto.**

**Estado `originAnchorNoRemote` — vetor de adversário com acesso a shell:**
O ADR-2026-09-17 afirma que os estados 1 e 2 são "not adversary-reachable via editing trackfw.yaml
alone". O adversário desta REQ, porém, é um agente com escrita irrestrita ao workspace —
incluindo `git remote remove origin` via Bash. Medido: este comando edita `.git/config`, que não
é um arquivo de hook rastreado. Após a remoção do remote, TODAS as rules ancoradas ficam silenciosas.
Este vetor existe e não é coberto pelo D1. Deve ser declarado como resíduo (ver R4, seção 4).

### 2d. Dedup do gerador com guard global instalado

`mergeClaudeHookArray` (agentfiles.go:1976-2015): itera o array existente; se encontra a
(matcher, command) pair, retorna sem modificar. Caso contrário, ACRESCENTA. **Nunca remove.**
Confirmado por leitura direta.

`migrateHookCommand` (agentfiles.go:1491-1506): altera o campo `command` de entradas existentes.
Se origin/main tem a forma D2/D11 e o `trackfw update` migra para D11-revised, a tuple de origin/main
(com o comando antigo) ficaria ausente do disco — potencial falso positivo (ver A2).

`migrateGuardHookMatcher` (agentfiles.go:1526+): renomeia o matcher ou extrai entradas de guard de
um bloco misto. Muda a 3ª componente da tuple — idêntico impacto ao item anterior.

**Conclusão:** `trackfw update` não remove fiação de forma silenciosa; mas migra formas de comando e
de matcher, tornando tuples de origin/main (pré-migração) ausentes do disco (pós-migração). Sem
tratamento especial na rule, isso produz falso positivo transitório (entre `trackfw update` local e
o próximo PR mergeado).

### 2e. `credentialGuardAnchoredRules` + `filterBaselineTagged` + `lenientCarveoutRules`

**Baseline carve-out:** `filterBaselineTagged` (validator.go:1194), linha 1210:
```go
if tolerated && !credentialGuardAnchoredRules[v.Rule] { ... }
```
Se a rule está em `credentialGuardAnchoredRules`, a baseline NÃO silencia. Confirmado.

**Lenient carve-out:** `applyLenientWithCarveout` (validator.go:607):
```go
if lenientCarveoutRules[v.Rule] { kept = append(kept, v) }
```
Se a rule está em `lenientCarveoutRules`, ela fica como violation mesmo em LENIENT. Confirmado.

**Estado atual:**
- `credentialGuardAnchoredRules` (linha 249): `credential_guard_hook_resolvable`,
  `credential_guard_script_integrity`, `credential_guard_mode_downgrade`. **Não inclui**
  `git_branch_guard_hook_resolvable` nem `git_branch_guard_script_integrity` (AC5 gap: confirmado).
- `lenientCarveoutRules` (linha 597): `req_roadmap_lifecycle`, `ref_targets_exist`. **Não inclui**
  nenhuma rule de guard.

**Com D5 implementado** (nova rule `guard_wiring_removed` adicionada a ambos os mapas): a violation
sobreviveria tanto ao baseline quanto ao lenient. D6 (adicionar as 2 rules de `git_branch_guard` ao
`credentialGuardAnchoredRules`) fecha o gap do AC5.

**Conclusão:** a combinação de carve-outs é suficiente SE e SOMENTE SE D5 e D6 forem implementados.
O estado atual NÃO basta.

---

## 3. Alvos de falsificação nas duas direções

### Direção 1 — Adulteração passa (false negative, detecção regride)

Por tupla:

| Vetor | Arquivo | Método | Gate esperado | Gating status |
|---|---|---|---|---|
| FN-1 | `.claude/settings.json` | tuple removida | rule acusa, sobrevive a lenient+baseline | OK (com D5) |
| FN-2 | `.claude/settings.json` | `disableAllHooks: true` adicionado (sem remover tuples) | rule **silenciosa** | **GAP** — ver A1 |
| FN-3 | `.claude/settings.local.json` | `disableAllHooks: true` (untracked) | rule silenciosa por construção | **RESÍDUO R1** |
| FN-4 | `.gemini/settings.json` | `hooksConfig.enabled: false` adicionado | rule **silenciosa** | **GAP** — ver A1 |
| FN-5 | `.kiro/hooks/trackfw-attention.json` | `"enabled": false` nas entradas de guard | rule **silenciosa** | **GAP** — ver A1 |
| FN-6 | `.github/hooks/trackfw-attention.json` | `disableAllHooks: true` adicionado | rule **silenciosa** | **GAP** — ver A1 |
| FN-7 | Qualquer hook file | `git remote remove origin` via shell | anchor silenciosa (estado 2) | **RESÍDUO R4** |
| FN-8 | `.codex/config.toml` | `[features] hooks = false` (arquivo separado) | rule só inspeciona `.codex/hooks.json` | **RESÍDUO R3** |

### Direção 2 — Projeto legítimo acusado (false positive, controle excessivo)

| Vetor | Origem | Mecanismo | Consequência |
|---|---|---|---|
| FP-1 | `trackfw update` pós ML-5A | matcher `"Bash"` → `"Bash|PowerShell"` no disco; origin/main tem `"Bash"` | violation até próximo PR mergeado |
| FP-2 | `trackfw update` pós ML-6B/6C | comando D2/D11-legacy → D11-revised no disco; origin/main tem forma antiga | violation até próximo PR mergeado |
| FP-3 | Projeto sem guard instalado | arquivo de hook ausente em origin/main (estado "file absent") | **silêncio correto** — D4 |
| FP-4 | `trackfw update` com global guard | global instalado → gerador não escreve project-scope; origin/main também não tem (desde a instalação do global) | **silêncio correto** — D4 |

FP-1 e FP-2 são transitórios (resolvem no próximo commit/PR) mas, como `guard_wiring_removed`
estará em `lenientCarveoutRules` e `credentialGuardAnchoredRules`, NÃO podem ser suprimidos por
baseline nem por lenient. Isso os torna bloqueantes para os adotantes durante a janela de migração.
**Ver A2.**

---

## 4. Resíduo declarado

O desenho aceita explicitamente não cobrir:

| ID | Resíduo | Justificativa |
|---|---|---|
| R1 | `.claude/settings.local.json` com `disableAllHooks: true` | git-ignored (confirmado: `~/.config/git/ignore:1`); sem âncora |
| R2 | Escopo global (`~/.claude/settings.json` etc.) | fora do git; já declarado no ADR |
| R3 | `[features] hooks = false` em `.codex/config.toml` | arquivo diferente de `.codex/hooks.json`; rastreabilidade dependente do adotante |
| R4 | `git remote remove origin` via shell | edita `.git/config`; não coberto pelo anchor de arquivo rastreado |
| R5 | Adversário com permissão de merge em `main` | limite declarado do ADR-2026-08-12 |
| R6 | Chaves de desabilitação nos arquivos RASTREADOS (`disableAllHooks`, `hooksConfig.enabled`, `"enabled": false`) | ver A1 — declarar como residual ou estender D2 |

**Nota sobre R4:** O ADR-2026-09-17 afirma que estados 1/2 do anchor não são "adversary-reachable".
Isso é verdade para a superfície de escrita de PR (editar o arquivo não altera o remote). Não é
verdade para o adversário desta REQ (agente com shell), que PODE executar
`git remote remove origin`. Este resíduo deve ser declarado no ADR porque o argumento original não
cobre este threat model.

**Nota sobre Amazon Q:** O Amazon Q CLI foi marcado como depreciado em favor do Kiro CLI em
2025-12-15 (fonte: docs.aws.amazon.com). O arquivo `.amazonq/cli-agents/q_cli_default.json` está
em `credentialGuardHookFiles`. O arquiteto deve decidir se mantém ou migra esta entrada.
Não é bloqueante para este Wave 0.

---

## 5. Veredito

**APROVADO COM AJUSTES**

O mecanismo central (âncora em `origin/main`, comparação por tupla, carve-outs de lenient e
baseline) é sólido para o caminho principal de adulteração (remoção direta de tuple no arquivo
rastreado). Os ajustes abaixo devem ser incorporados ao ADR pelo arquiteto antes do Wave 1.

---

**A1 — Chaves de desabilitação nos arquivos rastreados devem ser nomeadas como resíduo (ou
estendido o escopo da comparação).**

Os seguintes mecanismos permitem silenciar TODOS os hooks enquanto mantêm as tuples intactas,
nos arquivos RASTREADOS:

- `disableAllHooks: true` em `.claude/settings.json` (Claude Code) ou `.github/hooks/trackfw-attention.json` (Copilot)
  — fonte: docs.anthropic.com, docs.github.com
- `hooksConfig.enabled: false` em `.gemini/settings.json` (Gemini CLI) — fonte: geminicli.com
- `"enabled": false` por entrada em `.kiro/hooks/trackfw-attention.json` (Kiro) — fonte: kiro.dev

A ADR declara como resíduo "contornos que não tocam o arquivo rastreado" — mas estes TOCAM o
arquivo rastreado sem alterar as tuples. Devem ser nomeados explicitamente na seção
`Resíduos declarados` do ADR. Alternativa: estender D2 para comparar também estas chaves, mas isso
cria risco de falso positivo legítimo (desenvolvimento com `disableAllHooks` temporário).

**Recomendação:** declarar como resíduo nomeado em vez de estender D2, pois a habilitação/
desabilitação deliberada por um desenvolvedor humano é indistinguível de adulteração neste
mecanismo.

---

**A2 — D2's exact-tuple matching precisa de tratamento para formas equivalentes e migrações de
matcher.**

Quando origin/main tem uma tuple com forma D2-legacy ou matcher `"Bash"` (pré-ML-5A), e o disco
tem a forma D11-revised ou matcher `"Bash|PowerShell"` (pós-`trackfw update`), a rule acusaria
falso positivo. O mecanismo:

- `migrateHookCommand` (agentfiles.go:1491): migra D2→D11 no disco
- `migrateGuardHookMatcher` (agentfiles.go:1526): renomeia `"Bash"` → `"Bash|PowerShell"`

A rule D2 não pode usar igualdade de string simples para comando e matcher. O ML-1A deve
implementar:

1. **Para comando:** reconhecer as formas D2-legacy, D11-legacy e D11-revised como equivalentes
   para a mesma função de guard (usando `guardD2LegacyLine`, `guardD11LegacyLine`, `guardExpectedLine`
   por família).
2. **Para matcher:** um matcher no disco que CONTÉM o matcher de origin/main como subconjunto
   sintático deve ser tratado como presente (ex.: `"Bash|PowerShell"` ⊇ `"Bash"`). Formalmente:
   se `diskMatcher` contém `originMatcher` como alternativa, a tuple é considerada presente.

Sem este tratamento, todo adotante que rode `trackfw update` em um repositório antigo receberá uma
violation bloqueante que não pode ser suprimida por baseline ou lenient.

---

**A3 — O argumento "not adversary-reachable" do ADR deve ser refinado para distinguir superfície
de PR da superfície de agente com shell.**

O ADR-2026-09-17 e o ADR proposto afirmam que os estados 1 e 2 do anchor (sem git / sem remote)
não são atingíveis editando um arquivo rastreado. Isso é verdade para a superfície de escrita de PR.

O adversário desta REQ, porém, é um agente com `Bash` irrestrito — que pode executar
`git remote remove origin`, `git config --unset remote.origin.url` ou `rm -rf .git/remote` para
forçar o estado 2 (silêncio). O ADR deve declarar este vetor explicitamente como resíduo R4 ou
explicar por que considera o agent shell access fora do threat model desta REQ.

---

**A4 — A rule deve derivar o ref independentemente de `currentOriginMain.ref`.**

`loadOriginMainAnchor()` retorna `ref = ""` quando `trackfw.yaml` está ausente de origin/main
(ex.: repositório novo, ou estado `originAnchorFileAbsent`). Se a nova rule usar
`currentOriginMain.ref` para construir `git show <ref>:<hookfile>`, falharia silenciosamente quando
`trackfw.yaml` ainda não está em origin/main.

A implementação em ML-1A deve chamar `deriveOriginDefaultBranch()` independentemente para obter o
ref, ou tratar `ref == ""` com falha fechada nomeada.

---

**Verificação de gates do ML-0A (para o arquiteto):**

```bash
test -s docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md    # ✅
grep -q "Veredito" docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md   # ✅
git diff --quiet HEAD -- .claude/settings.json internal/               # verificar abaixo
```
