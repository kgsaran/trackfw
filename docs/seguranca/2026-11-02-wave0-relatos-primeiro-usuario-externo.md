# Wave 0 — Threat Model: Relatos do Primeiro Usuário Externo

> Hades (hades-tf) | 2026-11-02 | REQ-2026-11-02 | issues #657–661

---

## 1. Completude da enumeração

### AC1 — Sítios que dizem "will fail open" e o que cada um descreve de fato

**Medição:**

```
$ grep -rn "will fail open\|the hook will fail open" internal/validator/
```

Seis ocorrências. A coluna "Comportamento real" descreve a forma atual e a D2-legacy separadamente:

| Linha | Função | Condição | Comportamento real |
|---|---|---|---|
| `validator_guard_binary_probe.go:308` | `guardBinaryProbeOnce` | binário ausente do PATH | Forma atual PS/POSIX: exit 2 (bloqueia). D2-legacy PS/POSIX: exit 127 (fail-open). Mensagem errada somente quando TODOS os hooks PS/POSIX estão na forma atual e nenhum hook cmd.exe-family está presente. |
| `validator_guard_binary_probe.go:321` | `guardBinaryProbeOnce` | binário sem subcomando `guard` | Forma atual PS/POSIX: exit não-2 → normalizado para 2 (bloqueia). D2-legacy: exit não-2 (fail-open). Mesmo predicado que 308. |
| `validator_guard_binary_probe.go:428` | `guardCheckPS1RestrictedPolicy` | política Restricted (.ps1 sai 0) | Exit 0 (fail-open) — mensagem correta. |
| `validator_guard_binary_probe.go:435` | `guardCheckPS1RestrictedPolicy` | política ilegível | Incerto — mensagem correta (condicional). |
| `validator_guard_binary_probe.go:511` | `guardGitBashBinaryProbe` | trackfw ausente no PATH de login do Git Bash | Forma atual Git Bash: exit 2 (bloqueia). D2-legacy PS/POSIX em bash: exit 127 (fail-open). Mensagem errada somente quando TODOS os hooks Git Bash estão na forma atual. |
| `validator_guard_binary_probe.go:543` | `guardGitBashBinaryProbe` | binário Git Bash sem subcomando `guard` | Mesma lógica de 511. |

**Contexto: estes são hooks de ferramenta de agente (settings.json), não git hooks.**

As linhas de hook geridas pelo probe são escritas em `.claude/settings.json` como `PreToolUse`/`PostToolUse` para os matchers `Bash|PowerShell`, `Read` e `Write|Edit` (lido — `agentfiles.go:389–404`, `claudeShellMatcher = "Bash|PowerShell"`). Para hooks de agente Claude Code, somente exit 2 bloqueia a operação; qualquer outro código (incluindo 0, 1, 127) é tratado como erro não-bloqueante. Diferente de git hooks, onde qualquer não-zero bloqueia.

**Forma atual PS/POSIX (`CredentialCurrentPSPOSIX`, lido — `guardforms/guard_generations.go:23`):**

```
emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential;
LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE
```

`trackfw` ausente → bash exit 127 → `LASTEXITCODE=$((2*!!127))` = 2 → `exit $LASTEXITCODE` → exit 2 (BLOQUEANTE). Medido:

```
$ bash -c 'no_such_command_xyz 2>/dev/null; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE'; echo "exit: $?"
exit: 2
```

**Medição do Bruno (#658):** testou a forma PS/POSIX. Git Bash e PowerShell 5.1 → exit 2 (bloqueia). cmd.exe com a forma PS/POSIX → exit 1. "Linha testada: emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?))..."

**Forma cmd.exe nativa (lido — `agentfiles.go:93`, usada por Kiro e Amazon Q):** `trackfw guard <cmd> || exit 2`. A semântica do `||` em cmd.exe implica exit 2 quando `trackfw` retorna não-zero. Este comportamento é código-lido, não medido em Windows. Kiro e Amazon Q usam esta família (comentário `agentfiles.go:939`). Não foi verificado se esses hook runners tratam exit 2 como bloqueante da mesma forma que Claude Code. Declarado como residual (ver R-2).

**Forma D2-legacy PS/POSIX (lido — comentário `validator_credential_guard.go:706`):**

```
trackfw guard credential; exit $LASTEXITCODE
```

Em bash: `$LASTEXITCODE` não é variável especial; string vazia → `exit` sem argumento → exits com `$?` corrente = 127. Medido:

```
$ bash -c 'no_such_cmd_xyz 2>/dev/null; exit $LASTEXITCODE'; echo "exit: $?"
exit: 127
```

Exit 127 é fail-OPEN para hooks de agente. O comentário em linha 706 está correto: "D2 revised form — fail-open quando trackfw ausente do PATH."

**`anySubcmdFormFound`, `hasPSPosixSubcmd` e distinção entre formas:**

`validator_credential_guard.go:680–740` (lido): TODAS as formas (atual, D11Rev, D11, D2-legacy, unrecognized) setam `anySubcmdFormFound = true`. Quando família é PS/POSIX, todas setam `hasPSPosixSubcmd = true`. O probe dispara para qualquer forma, incluindo D2-legacy.

**`validateGuardHookD2InlineWarnings` (lido — `validator_credential_guard.go:1221–1225`):**

Função existe. D2-legacy já recebe migration warnings via esta função. O fix de P-1 adiciona distinção para as mensagens do probe.

**Testes existentes que não prendem "fail open" para a forma atual:**

`validator_guard_binary_probe_test.go:311` — `strings.Contains(violations[0], "not found in PATH")`.
`validator_guard_binary_probe_test.go:332` — `strings.Contains(violations[0], "does not have the guard subcommand")`.
`validator_guard_binary_probe_gitbash_test.go:415,471` — mesmas asserções para o probe Git Bash.

Nenhum verifica ausência de "fail open". Regressão possível sem asserção negativa.

**Obs. — comentário stale em `agentfiles.go:73`:** diz "sem sufixo" para cmd.exe, mas o código na linha 93 retorna `|| exit 2`. Fora do escopo desta REQ; registrar para a arquiteta.

---

### AC2 — Consumidores do wip_limit

**Medição:** `grep -rn "wip_limit\|validateWIPLimit\|wipViolationsLimit" internal/`

| Caminho | Arquivo:Linha | Descrição | Bug presente |
|---|---|---|---|
| `validate` (unfiltered) | `validator.go:953–958` | `applyRule("wip_limit", wipViolationsLimit, ...)` | Sim — wipViolationsLimit sempre vazio |
| `barrier` (tagged) | `validator.go:1484–1490` | `applyRuleTagged("wip_limit", wipViolationsLimit, ...)` | Sim — mesmo bug |
| `status` | `validator.go:1962–1974` | Exibe `⚠` inline somente no bloco `if cfg.WIP.BySquad` | Informacional, sem severidade; somente em modo by_squad |
| `serve` | via `GetStatus()` | Herda exibição do status | Idem |
| `ship` | `validator.go:4745` | Não chama `validateWIPLimit` | Sem impacto |
| `commit` | sem chamada | Sem impacto | — |
| `scaffold.go:957` | template | Grava `wip_limit: %d` (inteiro) — campo diferente de `rules.wip_limit` | Sem impacto |
| `configure.go:60–132` | wizard | Lê/escreve `wip_limit:` (inteiro) | Sem impacto |
| `help.go:84,168` | doc | `rules.wip_limit` (Default: "error") | Linha 168 precisa mudar para "warning" |

**Três branches em validateWIPLimit, todas com o bug (lido):**

```go
// by_agent:
warnings = append(warnings, fmt.Sprintf("%d roadmaps in wip/ for agent %q ...", ...))
// flat (non-squad):
warnings = append(warnings, fmt.Sprintf("%d roadmaps in wip/ (limit: %d) ...", ...))
// flat (by_squad):
warnings = append(warnings, fmt.Sprintf("squad %q has %d roadmaps in wip/ ...", ...))
```

`wipViolationsLimit` sempre vazio → `applyRule("wip_limit", wipViolationsLimit, ...)` recebe slice vazio, não emite nada.

**Dois default sources — por que mudar só ruleDefaults tem efeito zero:**

`diskRuleSeverity(name)` verifica `cfg.Rules[name]` (preenchido por `defaults()` em config.go:402: `"wip_limit": "error"`) antes de `ruleDefaults`. O passo 1 sempre bate. Mudar somente `ruleDefaults` tem efeito zero.

**`wip_limit` NÃO está em `credentialGuardAnchoredRules` (lido — `validator_credential_guard_integrity.go:249`):**

```go
var credentialGuardAnchoredRules = map[string]bool{
    "credential_guard_hook_resolvable":  true,
    "credential_guard_script_integrity": true,
    "credential_guard_mode_downgrade":   true,
    "guard_wiring_removed":              true,
    "git_branch_guard_hook_resolvable":  true,
    "git_branch_guard_script_integrity": true,
}
```

`wip_limit` ausente. Portanto: não há gate de weakening para esta regra no anchored path. A mudança de default afeta somente `diskRuleSeverity`. A edição de `ruleDefaults` em validator.go é para consistência (fonte de verdade única), não para fechar um gap de segurança.

**Impacto observável do fix — quatro casos:**

| `rules.wip_limit` no projeto | Antes do fix | Depois do fix |
|---|---|---|
| ausente | warning (fixo, ignora config) | warning (default) |
| `off` | warning (ignora `off`) | sem mensagem |
| `warning` | warning (correto por acidente) | warning |
| `error` | warning (ignora `error`) | violation; rc=1 em strict mode |

`validate` e `barrier`: mudança observável para projetos com `rules.wip_limit: error` (violation em vez de warning) e `rules.wip_limit: off` (silêncio em vez de warning). `commit` e `ship` não chamam `validateWIPLimit` — sem mudança.

**Impacto neste repositório:** 1 roadmap em wip/, limite 1, `1 > 1 = false` — nenhuma mensagem de qualquer forma.

---

### AC3 — Pontos que gravam pkg_manager e consequências do valor vazio

**Medição:**

```
$ grep -rn "PkgManager\|pkg_manager" internal/ scripts/ --exclude="*.go"
```

Resultado: somente corpus de teste (`pathguard/testdata/corpus-pre-fix/*.txt`) e strings de falsificação em `scripts/check-gates-falsify.sh` — nenhum consome PkgManager em lógica ativa. Nenhum script de hook usa o valor.

**Pontos de escrita em arquivos `.go` (exceto testes):**

`config.go:125` (campo, default `""`), `init.go:130` (non-TTY: `PkgManager: "npm"` hardcoded), `claudemd.go:96–98,135–137,254–256` (fallback de `""` para `"npm"` quando `Frontend != ""` ou `Backend != ""`).

**Detecção de `package.json` no discover (lido — `discover.go:188`):**

`discover.go:188` verifica `package.json` internamente como parte da lógica de escolha de framework de hook. Resultado não exposto em `DiscoveryResult` (struct sem campo `PkgManager`). P-6 replica o mesmo padrão inline em `init.go`.

**Consequências de `PkgManager = ""` — medição com binário real:**

```
$ trackfw validate   # em dir com pkg_manager: 
1 warning(s) (write_scope_not_wired)   RC=0

$ trackfw validate   # em dir sem linha pkg_manager
1 warning(s) (write_scope_not_wired)   RC=0
```

`pkg_manager: ` (YAML null) não gera `config_value_type`. `normalizeNode` retorna `ScalarNode.Value = ""` para scalars nulos. `scaffold.go` NÃO precisa mudar.

---

### AC4 — Pontos de escrita do init e ordem

**Código lido (`init.go`):**

Non-TTY path (linha 134–148): `generators.Scaffold(cfg)` antes de `installAITools(aiTools, cwd, "global")`. TTY path: mesma ordem. Ambos têm a inversão.

**installAITools escreve em dois grupos (lido, `init.go:427–510`):**

Grupo 1 — `manager.Install`: destinos resolvidos por `BuildPlans`. Write-time guard com `pathguard.RejectAndReport` (lido — `manager.go:865`).

Grupo 2 — `InjectRulesForTool(tool, cwd)`: project-local em `cwd/<agentFiles[tool]>`. Pathguard próprio (lido — `agentfiles.go:144`). Chamado APÓS `manager.Install` ter sucesso.

**Write-time guard com paths não-existentes (lido — `pathguard.go:77`):** ENOENT silenciado. Detecta junctions em componentes intermediários mesmo quando leaf não existe. Sem falso positivo em máquina fresca.

**Reprodução (macOS, symlink como proxy):**

```
$ ln -sfn $REAL_CLAUDE $HOME_SYM/.claude
$ HOME=$HOME_SYM ./bin/trackfw init --ai-tools claude [non-TTY]
# resultado: [escreve 20 arquivos de projeto] ... refusing reparse-point path ".../.claude"
# rc=1, projeto escrito, installAITools falhou
```

**RC=1 para `agents install --scope project` com symlink (medido sem pipe):**

```
$ ln -sfn /real/claude $SCRATCH/myproject/.claude
$ ./bin/trackfw agents install --scope project --targets claude
Error: refusing write to .../.claude/agents/trackfw-architect.md: refusing reparse-point path ".../.claude"
RC=1
```

---

### AC5 — Templates de ADR e REQ, e o que conta como "só modelo"

**Templates ADR (lido — `generators/adr.go`):**

Template 1 (linhas 128–168), aceito/active: seções obrigatórias `## Context`, `## Decision`, `## Consequences`, `## Alternatives Considered`. Comentários-modelo: `<!-- What is the situation that motivates this decision? -->`, `<!-- What was decided? -->`, `<!-- What are the positive and negative consequences of this decision? -->`, `<!-- What other options were evaluated and why were they rejected? -->`.

Template 2 (linhas 340–358), draft/fallback: mesmas quatro seções, mesmos quatro comentários-modelo.

**Templates REQ (lido — `generators/req.go:95–176`):**

Seções para verificação: `## Motivation` → `<!-- Why is this requirement needed? What problem does it solve? -->`. `## Acceptance Criteria` → `- [ ]\n- [ ]` (bare checkboxes — conteúdo-modelo).

Seções opcionais (nunca disparar): `## Linked ADR`, `## Blocked by ADRs`, `## Linked Roadmap`, `## Negative scope`.

**Arquivos live com estas seções (comando e resultado):**

```
$ grep -rln '## Motivation\|## Context' internal/ | grep -v '_test.go'
internal/pathguard/testdata/corpus-pre-fix/internal/generators/adr.go.txt   (corpus snapshot)
internal/pathguard/testdata/corpus-pre-fix/internal/generators/req.go.txt   (corpus snapshot)
internal/generators/roadmap.go                                               (roadmap template, não ADR/REQ)
internal/generators/adr.go                                                   (template fonte)
internal/generators/req.go                                                   (template fonte)
internal/sync/sync.go                                                        (parsing, não template)
internal/roadmapdoc/testdata/...                                             (corpus fixtures)
```

Arquivos ativos com template de ADR ou REQ: somente `internal/generators/adr.go` e `internal/generators/req.go`. `sync.go` e `roadmap.go` não são templates de ADR/REQ. Corpus e testdata são fixtures, não código ativo.

**Convenção de nomes de regra (lido):**

Padrão dominante `_has_`: `wip_has_req`, `blocked_has_req`, `branch_has_wip_roadmap`, `req_has_adr`, `req_has_roadmap` — 5 ocorrências. A nova regra é `adr_req_has_content`.

**Config não valida nomes de regra (lido — `config.go:539–567`):** aceita qualquer string. Novas regras configuráveis sem mudança no parser.

---

## 2. Modelo de Ameaça

**O adversário é o implementador apressado e o arquiteto otimista.**

### T-1 (AC1): novo teste cobre 308/321 mas não 511/543

**Gate:** asserção negativa `!strings.Contains(...)` em `validator_guard_binary_probe_gitbash_test.go` para os dois sites quando forma atual presente e nenhuma legada.

### T-2 (AC1): nova mensagem overclaima "blocks" quando formas legadas presentes

(a) Mensagem sem condição overclaima quando hooks mistos. (b) `hasCurrentSubcmd` setado para família cmd.exe overclaima para Kiro/Amazon Q, cujo comportamento com exit 2 não foi medido.

**Gate:** mensagem "blocks every command" somente quando `hasCurrentSubcmd && !hasAnyUnverifiedSubcmd`, onde `hasCurrentSubcmd` é setado somente para família PS/POSIX na forma atual. Vetor crítico: hook PS/POSIX atual + hook D2-legacy → "fail open" presente.

**Gate adicional:** somente hook cmd.exe-family na forma atual → mensagem mantém texto conservador (não overclaima "blocks").

### T-3 (AC1): hasAnyUnverifiedSubcmd setado somente em branch D2-legacy, esquecendo D11/D11Rev

Teste end-to-end com fixture de arquivo real é o único que detecta isso.

**Gate:** vetor em `validator_guard_binary_probe_test.go` com hook D11 + forma atual PS/POSIX → "fail open" presente (PATH seam em `validator_git_exec_test.go` pode ser reutilizado).

### T-4 (AC2): mudar apenas ruleDefaults, sem mudar config.go

`diskRuleSeverity` bate em `cfg.Rules["wip_limit"] = "error"` antes de `ruleDefaults`. Efeito zero.

**Gate:** strict mode sem `rules:` no trackfw.yaml → `ruleSeverity("wip_limit") == "warning"`.

### T-5 (AC2): loops wipWarningsLimit permanecem após fix

**Gate:** dois sites de append de `wipWarningsLimit` deletados no diff.

### T-6 (AC2): cache da probe retorna mensagem errada em segunda chamada com flags diferentes

Se cache key não inclui novos flags, segunda chamada reutiliza entrada da primeira.

**Gate:** duas chamadas com combinações de flags diferentes na mesma run → mensagens diferentes.

### T-7 (AC4): pré-checagem cobre apenas BuildPlans, não InjectRulesForTool

**Gate:** pré-checagem cobre destinos de `InjectRulesForTool` além dos de `BuildPlans`.

### T-8 (AC4): pré-checagem só no non-TTY path

**Gate:** TTY com scope mockado e HOME com symlink → Scaffold não chamado.

### T-9 (AC4): TOCTOU — junction criada entre pré-checagem e escrita

Write-time guards continuam ativos. **Gate:** guards não removidos no diff.

### T-10 (AC5): comentário HTML não fechado consome seção seguinte

**Gate:** vetor com `<!--` não fechado ao final da seção.

### T-11 (AC5): CRLF no modelo mascara detecção

**Gate:** vetor com `<!-- model\r\n` → aviso emitido.

---

## 3. Alvos de Falsificação

### AC1

**Falso negativo (forma atual PS/POSIX diz "fail open" quando devia avisar):**
- Gate: `hasCurrentSubcmd && !hasAnyUnverifiedSubcmd` (hasCurrentSubcmd setado somente PS/POSIX) → "fail open" ausente; "exit 2" presente

**Falso positivo (overclaima "blocks" com formas legadas ou cmd.exe-family presentes):**
- Gate: hook PS/POSIX atual + D2-legacy → "fail open" presente
- Gate: somente hook cmd.exe-family atual → mensagem conservadora (não "blocks")
- Gate de cache: segunda chamada com flags diferentes → mensagem diferente

### AC2

**Falso negativo (ignora wip_limit: error):**
- Gate: `rules.wip_limit: error`, 2 roadmaps em wip/, strict mode → rc=1 com violation

**Falso negativo (wip_limit: off gera warning após o fix):**
- Gate: `rules.wip_limit: off`, limite estourado → sem mensagem

**Falso negativo (somente ruleDefaults corrigido, config.go não):**
- Gate: strict mode sem `rules:` → `ruleSeverity("wip_limit") == "warning"` (via diskRuleSeverity)

### AC3

**Falso negativo (npm em Python):**
- Gate: non-TTY sem `package.json` → `pkg_manager: ` vazio (RC=0 no validate)

**Falso positivo (vazio em Node.js):**
- Gate: non-TTY com `package.json` → `pkg_manager: npm`

### AC4

**Falso negativo (projeto half-init):**
- Gate: non-TTY com symlink em `HOME/.claude` → rc=1, nenhum arquivo de projeto escrito

**Falso positivo (máquina fresca recusada):**
- Gate: sem `HOME/.claude` → rc=0, init completo (ENOENT silenciado)

### AC5

**Falso negativo (modelo não detectado):**
- Gate: vetores (a) `- [ ]` bare, (b) `<!-- comment -->`, (c) multi-linha, (d) `<!--` não fechado, (e) CRLF no comentário

**Falso positivo (ADR preenchido com comentário interno):**
- Gate: `## Context` com `"CQRS (<!-- sigla -->)"` → sem aviso

**Falso positivo (seção opcional com modelo):**
- Gate: `## Linked ADR: <!-- ... -->` → sem aviso

---

## 4. Residual Declarado

**R-1 (AC1):** D2-legacy PS/POSIX em bash sai 127 — fail-OPEN para hooks de agente. Comentário no código correto. Para hooks mistos (PS/POSIX atual + legado), mensagem do probe mantém "fail open" por segurança conservadora. `validateGuardHookD2InlineWarnings` (lido — `validator_credential_guard.go:1221`) cobre migration warnings separadamente.

**R-2 (AC1):** cmd.exe com formulário PS/POSIX sai 1 (medido pelo Bruno em #658) — fail-OPEN. Limitação documentada. Forma cmd.exe nativa (`|| exit 2`, Kiro e Amazon Q) é afirmação de código-lido. Não verificado se os hook runners desses tools tratam exit 2 como bloqueante. A mensagem do probe para esses hooks mantém o texto conservador (não overclaima "blocks").

**R-3 (AC1, stale):** Comentário em `agentfiles.go:73` contradiz código em linha 93. Fora do escopo desta REQ.

**R-4 (AC3):** `DiscoveryResult` não expõe `PkgManager`. P-6 replica o padrão `os.Stat(filepath.Join(cwd, "package.json"))` de `discover.go:188` inline em `init.go`.

**R-5 (AC4):** `identity.Save(home)` escreve em `~/.trackfw/` antes de `Scaffold`. Fora do escopo.

**R-6 (AC4):** TOCTOU entre pré-checagem e escrita. Write-time guards permanecem. Aceito.

**R-7 (AC5):** Seção preenchida com um único caractere passa sem aviso. Tripwire, não auditoria de qualidade.

**R-8 (AC5):** ADRs legados com headings em pt-BR não disparam aviso. Correto.

**R-9 (AC5):** Deletar `## Context` silencia a regra. Fora do escopo.

---

## Prescrições

### P-1 (AC1) — Corrigir mensagens em guardBinaryProbeOnce (linhas 308 e 321), com predicado de forma atual PS/POSIX vs. outros

**Nota ao arquiteto — emenda a AC1:** AC1 da REQ é incondicional. P-1 torna a mensagem condicional — muda somente quando `hasCurrentSubcmd && !hasAnyUnverifiedSubcmd`. Solicitar emenda formal a AC1 antes de iniciar ML-1A. Texto proposto: "As mensagens de binário ausente e de binário sem `guard` dizem que o hook bloqueia todo comando (exit 2) quando somente hooks PS/POSIX na forma atual estão presentes. Com formas legadas ou hooks cmd.exe-family, mantém 'fail open'. Comportamento da forma cmd.exe nativa é afirmação de código-lido; Kiro e Amazon Q requerem medição própria."

Arquivo 1: `internal/validator/validator_credential_guard.go`

Adicionar dois booleans ao escopo da função de detecção:
- `hasCurrentSubcmd bool` — setado somente no branch `m.raw == expectedLine` **E `hf.family` é PS/POSIX** (emulate/LASTEXITCODE família). Hooks cmd.exe-family na forma atual NÃO setam este flag.
- `hasAnyUnverifiedSubcmd bool` — setado em TODOS os outros branches: D11Rev, D11, D2-legacy, unrecognized, E para qualquer hook cmd.exe-family (independentemente de ser forma atual ou não).

Renomear de `hasAnyLegacySubcmd` para `hasAnyUnverifiedSubcmd` para que ninguém o "corrija" de volta para cobrir somente legados.

Passar ambos para `guardBinaryProbeCached(hasPSPosixEntry, hasCurrentSubcmd, hasAnyUnverifiedSubcmd bool)`.

Arquivo 2: `internal/validator/validator_guard_binary_probe.go`

Atualizar assinaturas de `guardBinaryProbeOnce` e `guardBinaryProbeCached`. Expandir `guardBinaryProbeRunState` (atualmente `[2]*guardBinaryProbeCacheEntry`) para cobrir as três flags — sugestão: struct como chave de mapa.

Lógica nas linhas 308 e 321:
- `hasCurrentSubcmd && !hasAnyUnverifiedSubcmd`: substituir "the hook will fail open" por "the hook will block every command the agent runs through it (exit 2)". Cláusula de remediação existente permanece.
- Caso contrário: manter "fail open".

Testes em `validator_guard_binary_probe_test.go`:
- `hasCurrentSubcmd=true, hasAnyUnverifiedSubcmd=false` (somente PS/POSIX atual): "fail open" ausente; "exit 2" presente.
- `hasCurrentSubcmd=true, hasAnyUnverifiedSubcmd=true` (misto PS/POSIX atual + legado): "fail open" presente — vetor mais crítico.
- `hasCurrentSubcmd=false` (D2-legacy puro): "fail open" presente.
- `hasCurrentSubcmd=false, hasAnyUnverifiedSubcmd=true` (somente cmd.exe-family atual): "fail open" presente (comportamento não medido em Windows).
- Gate de cache: duas chamadas com flags diferentes na mesma run → mensagens diferentes.

### P-2 (AC1, Regra Dura) — Corrigir mensagens em guardGitBashBinaryProbe (linhas 511 e 543), mesmo predicado

Arquivo: `internal/validator/validator_guard_binary_probe.go`

Mesmo predicado `hasCurrentSubcmd && !hasAnyUnverifiedSubcmd` (recebido via parâmetros).

Linha 511: quando predicado verdadeiro, substituir "the hook will fail open (exit 127)" por "the hook will block every command the agent runs through it (exit 2)". "exit 127" era correto para D2-legacy em bash (medido); manter somente quando há hooks não-verificados.

Linha 543: mesmo tratamento.

Testes em `validator_guard_binary_probe_gitbash_test.go`:
- Mesmos quatro cenários de P-1.
- Asserção: "Git Bash" presente quando corrigido; "PowerShell" ausente (estas linhas são de `guardGitBashBinaryProbe`).

**Arquivo adicional — vetor end-to-end de hook misto:**

Adicionar vetor em `validator_guard_binary_probe_test.go` usando fixtures de hook real (hook D11 + hook PS/POSIX atual), passando pelo loop de detecção completo em `validateCredentialGuardHookResolvable`. O vetor precisa do PATH seam (binário ausente do PATH); consultar `validator_git_exec_test.go` (arquivo confirmado como tendo `t.Setenv("PATH", ...)`) para reutilizar o padrão. Este é o único vetor que captura implementador que esquece `hasAnyUnverifiedSubcmd` nos branches D11/D11Rev.

### P-3 (AC2) — validateWIPLimit: mover mensagens de warnings para violations

Arquivo: `internal/validator/validator.go`

Nas três branches: substituir `warnings = append(warnings, ...)` por `violations = append(violations, ...)`. Remover appends de `wipWarningsLimit` em linhas 958 e 1490.

### P-4 (AC2) — Mudar o default de wip_limit para warning nas DUAS fontes

Arquivo 1: `internal/config/config.go:402` — `"wip_limit": "error"` → `"wip_limit": "warning"`.

Arquivo 2: `internal/validator/validator.go:194` — adicionar `"wip_limit": "warning"` ao mapa `ruleDefaults` (por consistência — esta regra não está em `credentialGuardAnchoredRules`).

Teste obrigatório:

Gate 1 — path normal (`diskRuleSeverity`):
```
strict mode, sem rules: no trackfw.yaml, sem origin/main
→ ruleSeverity("wip_limit") == "warning"
```

Nota: não há gate para o anchored path porque `wip_limit` não está em `credentialGuardAnchoredRules` (verificado — mapa lido em `validator_credential_guard_integrity.go:249`).

### P-5 (AC2) — Atualizar documentação

`internal/commands/help.go:168`: `Default: '"error"'` → `Default: '"warning"'`. (ML-1A adicionará também entrada para `adr_req_has_content` onde a lista de regras está documentada.)

`README.md:480`: expandir "one at a time" para explicar `wip_limit` (inteiro) e `rules.wip_limit` (default "warning"). Projetos com `rules.wip_limit: error` passam a receber violation em strict mode.

### P-6 (AC3) — Detectar pkg_manager no non-TTY path do init

Arquivo: `internal/commands/init.go`

Substituir `PkgManager: "npm"` por check inline: `os.Stat(filepath.Join(cwd, "package.json"))` — mesmo padrão de `discover.go:188`. Retorna `"npm"` se existe, `""` caso contrário.

`scaffold.go` não precisa mudar: `pkg_manager: ` (string vazia) é YAML válido — medido com binário real, RC=0.

**Nota ao arquiteto:** `DiscoveryResult` não expõe `PkgManager`. "Usa a detecção existente" da REQ refere-se ao padrão de verificação de `package.json` que já existe em `discover.go:188`.

### P-7 (AC4) — Pré-checagem do destino antes de Scaffold em ambos os caminhos

Arquivo: `internal/commands/init.go`

**Spec:** antes de `Scaffold`, chamar `pathguard.RejectAndReport(destRoot, destination)` para cada destino dos dois grupos:

1. Grupo `BuildPlans`: destinos que `BuildPlans(aiTools, cwd, scope)` retornaria. O `destRoot` é o mesmo que `manager.Install` computa (lido — `manager.go:758–843`); não chamar `EvalSymlinks` sobre `~/.claude` diretamente.
2. Grupo `InjectRulesForTool`: para cada tool em `agentFiles`, destino `filepath.Join(cwd, agentFiles[tool])`.

`pathguard.RejectAndReport` percorre todos os componentes. ENOENT silenciado — sem falso positivo em máquina fresca.

**Ordem correta em AMBOS os caminhos:**

```
1. Resolver scope (TTY: promptInstallScopeRunner ANTES de Scaffold)
2. Pre-check: BuildPlans + InjectRulesForTool → pathguard.RejectAndReport → abortar se recusado
3. generators.Scaffold(cfg)
4. installAITools(aiTools, cwd, scope)   ← write-time guards continuam ativos (não remover)
```

**Saída de erro:** nomear o destino problemático; oferecer `--scope project` ou `trackfw agents install --scope project` como alternativa.

**Flag:** `--scope project|global`, padrão `global`.

**Documentação de `--scope` para init:** na usage string em `init.go` (ML-1B). `--scope` já está documentado em `help.go` para `agents install` (linhas 779–801). A mudança de contrato de `init --scope` vai em `docs/cli-parity.md` via ML-1A (seguindo a convenção do roadmap de que "o texto do ML-1B entra por ele ou pelo arquiteto") ou commit separado do arquiteto.

**TOCTOU:** pré-checagem não substitui guard de escrita. Ambos permanecem.

### P-8 (AC5) — Nova regra adr_req_has_content

**Nome da regra:** `adr_req_has_content`

**Default:** `"warning"` — adicionar a `ruleDefaults` em `validator.go` e documentar em `help.go` onde outras regras estão listadas.

**Seções mandatórias:**

ADR (em `adr_dirs`, prefixo `ADR-`): `## Context`, `## Decision`, `## Consequences`, `## Alternatives Considered` (ambos os templates em `generators/adr.go`).

REQ (em `req_dir`): `## Motivation`, `## Acceptance Criteria`.

**Seções opcionais (nunca disparar):** `## Linked ADR`, `## Blocked by ADRs`, `## Linked Roadmap`, `## Negative scope`.

**Modelo (conteúdo a ignorar):**
- Comentários HTML: `<!-- ... -->` single-line e multi-line, incluindo `<!--` não fechado
- Whitespace puro: espaços, tabs, CRLF (`\r\n`), LF, BOM, NBSP, U+200B, U+2060
- Bare checkboxes: linhas `^\s*-\s*\[\s*\]\s*$`

**Vetores de teste obrigatórios (lista fechada para ML-1A):**

| # | Conteúdo | Resultado esperado |
|---|---|---|
| a | `- [ ]` bare em `## Acceptance Criteria` | aviso |
| b | `<!-- comment -->` single-line em `## Context` | aviso |
| c | comentário multi-linha `<!-- ...\n... -->` | aviso |
| d | `<!--` não fechado ao final da seção | aviso |
| e | CRLF no comentário (`<!-- model\r\n`) | aviso |
| f | Parágrafo de texto em qualquer seção mandatória | sem aviso |
| g | `## Context` com `"CQRS (<!-- sigla -->)"` | sem aviso (texto além do comentário) |
| h | `## Linked ADR: <!-- ... -->` (seção opcional) | sem aviso |

---

## Lista de Arquivos por ML (sem sobreposição)

### ML-1A — Validator, config, docs (AC1, AC2, AC5)

Produção:
- `internal/validator/validator_credential_guard.go` (AC1: `hasCurrentSubcmd`, `hasAnyUnverifiedSubcmd`)
- `internal/validator/validator_guard_binary_probe.go` (AC1: linhas 308, 321, 511, 543; assinaturas e cache)
- `internal/validator/validator.go` (AC2: validateWIPLimit + applyRule sites, ruleDefaults; AC5: nova função + ruleDefaults)
- `internal/config/config.go` (AC2: defaults)
- `internal/commands/help.go` (AC2: linha 168; AC5: entrada para adr_req_has_content na lista de regras)
- `README.md` (AC2: wip_limit configurável)
- `docs/cli-parity.md` (AC2: defaults; AC5: nova regra; AC4: nota de contrato de `init --scope`)

Testes:
- `internal/validator/validator_guard_binary_probe_test.go` (AC1: quatro cenários + gate de cache + vetor end-to-end de hook misto)
- `internal/validator/validator_guard_binary_probe_gitbash_test.go` (AC1: quatro cenários)
- `internal/validator/validator_origin_main_severity_test.go` (AC2: Gate 1 de default)
- `internal/validator/validator_adr_req_has_content_test.go` (novo, AC5: vetores a–h)

### ML-1B — Init (AC3, AC4)

Produção:
- `internal/commands/init.go` (AC3: check inline package.json; AC4: pré-checagem + `--scope` + ordem)

Testes:
- `internal/commands/init_test.go` (AC3 e AC4)

**Sem arquivo em comum entre ML-1A e ML-1B.** `help.go` em ML-1A; documentação de `--scope` para init fica na usage string em `init.go` (ML-1B) e em `docs/cli-parity.md` via ML-1A.

**Aviso de compilação paralela:** ML-1B testa `./internal/commands/`. ML-1A edita `help.go` no mesmo pacote. Serializar ML-1A antes de ML-1B iniciar testes do pacote, ou usar worktrees separados.

---

## Para a arquiteta — registrar em vault

1. **`ruleDefaults` e `config.defaults()` são duas fontes com dois leitores independentes.** `diskRuleSeverity` usa `cfg.Rules` (de `defaults()`); `credentialGuardDefaultSeverity` usa apenas `ruleDefaults`. Para regras em `credentialGuardAnchoredRules`, inconsistência cria falsa violação de weakening. Para regras fora do mapa (como `wip_limit`), o risco é menor mas a inconsistência é confusa. Mudar ambas atomicamente a cada novo padrão de default.

2. **Hooks de agente Claude Code (settings.json) bloqueiam somente em exit 2.** `$((2*!!$?))` normaliza qualquer não-zero para 2 na forma atual PS/POSIX. D2-legacy PS/POSIX em bash sai 127 — fail-OPEN. O comentário "fail open quando trackfw ausente do PATH" estava correto para D2-legacy. Forma cmd.exe nativa (`|| exit 2`, Kiro/Amazon Q) é afirmação de código-lido. Comportamento desses tools com exit 2 requer medição própria em Windows.

3. **`normalizeNode` retorna `ScalarNode.Value` (string) para scalars YAML.** `pkg_manager: ` (YAML null) → `ScalarNode.Value = ""` → sem `config_value_type`. Medido com binário real: RC=0. Premissa "YAML null causa `config_value_type`" é falsa.
