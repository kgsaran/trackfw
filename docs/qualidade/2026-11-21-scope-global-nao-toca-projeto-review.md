# Code Quality Review — ML-1A (Wave 1)
# scope global não escreve no projeto

> Revisor: hefesto-tf | Data: 2026-11-21
> Escopo: `git diff 6dda75b1^..49be92c9` (commit `fix(commands): agents/skills install com scope global...`)
> Contrato: ADR-2026-11-21 D1–D4 | Threat model: Wave 0 T1–T9

---

## Veredito: APROVA

Nenhum achado bloqueante. 8 achados não bloqueantes listados abaixo.

**Evidência de base:**

```
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -count=1 -timeout 5m \
  -run 'TestGlobalInstallDoesNotWriteClaudeRulesOrHooks|TestGlobalInstallDefaultNonTTYDoesNotWriteRulesOrHooks|TestSkillsInstallGlobalDoesNotWriteClaudeRulesOrHooks|TestProjectInstallWritesClaudeRulesAndHooksAndNoHint|TestGlobalInstallJsonStdoutIsValid|TestGlobalInstallWindsurfDoesNotWriteWindsurfFiles|TestGlobalInstallHintDoesNotCiteCwd|TestTTYScopeGlobalDoesNotWriteRulesOrHooks|TestTTYScopeProjectWritesRulesAndHooks' \
  ./internal/commands/
ok  github.com/kgsaran/trackfw/internal/commands  0.833s
```

```
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -count=1 -timeout 5m \
  -run 'Install|Skills|Agents|Hook|Rules|Scope|Windsurf|Codex|Antigravity|Identity' \
  ./internal/commands/
ok  github.com/kgsaran/trackfw/internal/commands  21.809s
```

```
go build ./internal/commands/
# (zero output — sem erro de compilação)
go vet ./internal/commands/
# (zero output)
git status --short
# (clean — zero arquivos modificados durante a revisão)
```

---

## Checagem de pré-condição

`trackfw validate` passa (zero violations); warnings pré-existentes de REQs sem ADR são anteriores a
este ML. `zz_wave0_arm_verify_test.go` não está rastreado pelo git e não existe no disco — o cleanup
da wave 0 foi concluído.

---

## Revisão do gate (D1–D4)

**Localização do guard:** `internal/commands/integrations_flags.go`, linha 286.

```go
if operation == "install" && opts.scope == "project" {
    // InjectRulesForTool ... InjectHooksForTools ... ValidateAntigravity (D4)
} else if operation == "install" {
    // D2: hint para stderr via cmd.ErrOrStderr()
}
```

O guard é avaliado DEPOIS de `resolveScope()` (linha 162), como exigido pelo threat model T1/T2.
`resolveScope()` garante que `opts.scope` é `"project"` ou `"global"` ao sair: explícito via flag →
validado; non-TTY → `"global"`; TTY → retorna o valor do prompt (que oferece só "project" / "global").
Não há rota para `opts.scope == ""` nem para valor inválido ao chegar no guard. **Correto.**

D1 — global não escreve no projeto: guard positivo em `"project"`, fail-closed. Verificado.
D2 — hint para stderr via `cmd.ErrOrStderr()`; sem path de cwd; `%s` expande `kind` ("agents" /
"skills") corretamente (`type ItemKind string`). Verificado.
D3 — project scope inalterado: o bloco de inject não foi movido, apenas o guard foi estreitado.
Verificado.
D4 — D8/Antigravity gated pelo mesmo `if`: correto em termos de lógica; ver achado NB-3.

`check-update-parity.sh`: as funções `install_claude_agents()` e `install_agent_global()` já usavam
scratch dirs isolados; nenhum downstream do script espera `CLAUDE.md` ou `.claude/settings.json`
nesses diretórios. Limpeza do comentário coerente com o fix. Sem impacto no CI.

`docs/cli-parity.md`: seção nova adicionada. Seção do Antigravity atualizada com qualificador D4.
Nenhuma seção preexistente afirma que `agents install` / `skills install` escrevem regras ou hooks
sem qualifier de escopo (pesquisa por `InjectRulesForTool`, `InjectHooksForTools`, "auxiliary rules",
"writes rules" sem termos de negação ou escopo). Verificado.

---

## Achados não bloqueantes

### NB-1 — Condição morta em `TestGlobalInstallHintDoesNotCiteCwd`
**Severidade:** não bloqueante
**Arquivo:linha:** `internal/commands/integrations_scope_project_writes_test.go:328–330`

```go
if strings.Contains(hintLine, "/") && strings.Contains(hintLine, project[:min(len(project), 5)]) {
    t.Errorf("D2 hint appears to contain a filesystem path; hint: %q", hintLine)
}
```

O texto do hint não contém o caractere `/`; a primeira condição é permanentemente `false` e o `&&`
nunca ativa. O teste não verifica o que declara ("no absolute path"). A proteção real é a asserção
da linha 324 (`strings.Contains(hintLine, project)`), que é suficiente mas não detecta paths com
separador Windows.

**Correção sugerida:** substituir por `strings.ContainsAny(hintLine, "/\\")` (ou
`strings.Contains(hintLine, string(filepath.Separator))` para portabilidade).

---

### NB-2 — `func min` sombra o built-in do Go 1.21+; comentário factualmente errado
**Severidade:** não bloqueante, mas risco de compilação com `zz_redteam_*_test.go` do hades
**Arquivo:linha:** `internal/commands/integrations_scope_project_writes_test.go:333–339`

`go.mod` declara `go 1.25.2`; `min` é built-in desde Go 1.21. O helper local é desnecessário. O
comentário "// min is a helper for older Go versions without built-in min for int." é falso para
este módulo. Hades está escrevendo `zz_redteam_*_test.go` no mesmo package; se qualquer um desses
arquivos importar ou definir `min` com assinatura diferente, o build quebra por declaração
duplicada.

**Correção sugerida:** remover o helper e o comentário; usar o built-in `min(len(project), 5)`.

---

### NB-3 — D4 não é testável via `cmd.SetErr`; marker de gate overclaims
**Severidade:** não bloqueante
**Arquivo:linha:** `docs/cli-parity.md:1290` (marker) e `internal/commands/integrations_flags.go:298–303`

O D8/Antigravity warning usa `fmt.Fprintf(os.Stderr, ...)` (linha 298), não
`fmt.Fprintf(cmd.ErrOrStderr(), ...)`. O marker `<!-- trackfw-contract: gate=... -->` não declara
que D4 não é alcançado pelos 9 testes. Os testes que usam `cmd.SetErr(&stderr)` não podem capturar
essa saída.

O comportamento de D4 está correto (o bloco inteiro 286–317 é gateado; quando `scope != "project"`,
o ValidateAntigravity não roda). Mas não há teste que capture a saída D8 para verificar que ela NÃO
aparece em scope global.

**Correção sugerida (não-produto):** adicionar `partial=D4-sem-teste-capturavel (D8-warning usa
os.Stderr, não cmd.ErrOrStderr; verificado por leitura do gate, não por asserção)` ao marker.
Roteamento de `os.Stderr` → `cmd.ErrOrStderr()` nas linhas 298 e 303 é mudança de produto — fora
do escopo desta revisão.

---

### NB-4 — Número de linha "line 162" em comentário de código e em cli-parity.md
**Severidade:** não bloqueante
**Arquivo:linha:** `internal/commands/integrations_flags.go:280` e `docs/cli-parity.md:1321`

Ambas as referências a "line 162" são precisas agora. Tornam-se obsoletas se linhas forem
adicionadas acima em `integrations_flags.go`.

**Correção sugerida:** substituir por referência simbólica: "opts.scope is read AFTER `resolveScope()`
completes" / "after the `resolveScope()` call".

---

### NB-5 — Atribuição morta `_ = project2`
**Severidade:** não bloqueante
**Arquivo:linha:** `internal/commands/integrations_scope_project_writes_test.go:279`

`project2` é usado nas linhas 287 e 290. A atribuição `_ = project2` na linha 279 é ruído que
sugere intenção de suprimir "unused variable" antes de o autor adicionar os usos, mas não foi
removida.

**Correção sugerida:** deletar a linha 279.

---

### NB-6 — `TestGlobalInstallWindsurfDoesNotWriteWindsurfFiles` declara uma conclusão para dois braços
**Severidade:** não bloqueante
**Arquivo:linha:** `internal/commands/integrations_scope_project_writes_test.go:28–32`

O cabeçalho do arquivo declara:
```
//   TestGlobalInstallWindsurfDoesNotWriteWindsurfFiles
//     Affirms T5: ...
```
O teste tem um contra-braço (`--scope project`) que afirma D3. A Regra Dura de Reconciliação exige
uma frase por conclusão afirmada.

**Correção sugerida:** adicionar ao cabeçalho:
```
//     Counter-arm affirms D3: --scope project creates .windsurfrules and
//     .windsurf/hooks.json (InjectWindsurfHooks is not blocked for project scope).
```

---

### NB-7 — `cli-parity.md` overclaim: "current working directory is not modified"
**Severidade:** não bloqueante
**Arquivo:linha:** `docs/cli-parity.md:1294`

```
Running `agents install --scope global` ... The current working directory is not modified.
```

O threat model R3 documenta que quando `cwd == HOME` (manager.ProjectRoot == manager.HomeDir), o
`manager.Install` global também escreve em cwd e o oracle de hash daria falso positivo. A afirmação
absoluta é incorreta para esse caso.

**Correção sugerida:** "Rules and hook files under `manager.ProjectRoot` are not written."

---

### NB-8 — Sentinel de hint dividido entre verificações positivas e negativas
**Severidade:** não bloqueante
**Arquivo:linha:** `internal/commands/integrations_scope_project_writes_test.go` — múltiplas linhas

Verificações positivas (hint deve estar presente) usam dois critérios distintos:
- `"trackfw agents install --scope project"` (testes 1, 2, 3)
- `"not modified"` (testes 5, 7, 8)

Verificações negativas (hint não deve aparecer) usam exclusivamente `"not modified"`.

Se o texto do hint for reformulado sem "not modified" (ex.: "Project files not changed"), as
verificações negativas passam silenciosamente sem verificar nada.

**Correção sugerida:** definir `const hintSentinel = "not modified"` e usar em todas as verificações
positivas e negativas.

---

## Cobertura de T1–T9 (threat model)

| Ameaça | Teste | Braço e contra-braço |
|--------|-------|----------------------|
| T1 guard antes de resolveScope | TestTTYScopeProjectWritesRulesAndHooks | prompt "project" → CLAUDE.md modificado |
| T2 guard keyed em Changed("scope") | TestGlobalInstallDefaultNonTTYDoesNotWriteRulesOrHooks | sem --scope, non-TTY → hash idêntico |
| T3 prompt TTY retorna "global" | TestTTYScopeGlobalDoesNotWriteRulesOrHooks | stub → hash idêntico |
| T4 guard só em KindAgents | TestSkillsInstallGlobalDoesNotWriteClaudeRulesOrHooks | skills --scope global → hash idêntico |
| T5 fix parcial (só InjectRules ou só InjectHooks) | TestGlobalInstallWindsurfDoesNotWriteWindsurfFiles | windsurf global → ambos ausentes |
| T6 D2 hint contamina stdout com --json | TestGlobalInstallJsonStdoutIsValid | stdout é JSON válido; hint em stderr |
| T7 hint cita cwd | TestGlobalInstallHintDoesNotCiteCwd | hint não contém project path (ver NB-1) |
| T8 hint aparece com --scope project | TestProjectInstallWritesClaudeRulesAndHooksAndNoHint | project → sem "not modified" |
| T9 ProjectRoot = os.Getwd() | coberto pelo fixture (os.Chdir) — sem gap |

Cobertura completa. NB-1 enfraquece T7 mas não o invalida: a asserção primária (linha 324) verifica
o path completo.

---

## Duplicação e helpers

`fileSHA256`, `assertFileAbsent`, `assertHashUnchanged` são helpers novos sem duplicata no pacote. A
unica sobreposição encontrada (`sha256.Sum256` em `attention_test.go:55`) é inline, não um helper
reutilizável.

`TestGlobalInstallDefaultNonTTYDoesNotWriteRulesOrHooks` sobrepõe parcialmente
`TestInstallWithoutScopeOutsideTTYDefaultsToGlobal` (agents_skills_test.go:350), que o threat model
§1.10 pedia que fosse estendido. A nova fixture é mais completa (hash CLAUDE.md + ausência de
settings.json + verificação do hint). Os dois coexistem sem conflito — a extensão do teste existente
era uma alternativa, não a única correção válida.

---

## git status

```
$ git status --short
(clean — zero arquivos modificados durante esta revisão)
```
