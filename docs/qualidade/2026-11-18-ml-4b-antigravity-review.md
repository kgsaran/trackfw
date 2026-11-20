# ML-4B — Revisão de Qualidade: Guard no Google Antigravity (REQ-2026-11-17)

**Revisor:** hefesto-tf  
**Data:** 2026-11-18  
**Branch:** feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli  
**Veredito:** APROVA COM NOTAS — merge condicionado ao ML corretivo de A1

---

## Gates executados

```
go test ./internal/guard/         → ok  (3.133s)
go test ./internal/generators/    → ok  (86.189s)
go test ./internal/validator/     → ok  (37.463s)
go test ./internal/auditsurface/  → ok  (0.353s)
go vet ./internal/guard/ ./internal/generators/ ./internal/validator/ ./internal/auditsurface/
  → limpo
git status --short                → ?? docs/qualidade/2026-11-18-ml-4b-antigravity-review.md
```

---

## Resumo de escopo

Diff cobre: `internal/guard/payload.go`, `internal/guard/credential.go`,
`internal/guard/antigravity_test.go`, `internal/generators/agentfiles.go`,
`internal/generators/hooks.go`, `internal/generators/antigravity_hooks_test.go`,
`internal/generators/guard_registry_sites_ac4_test.go`,
`internal/auditsurface/auditsurface.go`, `internal/auditsurface/antigravity_test.go`,
`internal/validator/validator.go`, `internal/validator/validator_gemini_cli_legacy.go`,
`internal/validator/validator_credential_guard.go`,
`internal/validator/validator_guard_hook_concordance_external_test.go`,
`internal/validator/validator_guard_wiring.go`, `internal/validator/export_test.go`,
`internal/integrations/doctor.go`, `internal/commands/doctor.go`,
`internal/commands/init_guard_hooks_test.go`.

MLs cobertos: 3A (payload), 3B (hooks do projeto), 3B corretivo (matcher do git-branch),
3C (audit/aviso de legado), 3D (docs — fora do escopo interno).

---

## Achados

### A1 — `credViewFileScan` passa path JSON-decodificado por limpeza de tokens shell (Bug · Médio — bloqueia merge)

**Arquivo:linha:** `internal/guard/credential.go:803-804`

```go
resolved := credResolveArg(absPath, shellCwd, toolName)   // linha 803
if typ, cleanedPath := credScanFile(resolved); typ != "" { // linha 804
```

`credResolveArg` (linha 945) usa `strings.Map` para remover **todos** os caracteres `'` e `"` do
argumento — remoção interior, não só nas bordas. `credScanFile` (linha 596) repete o mesmo
`strings.Map`. Para `toolCall.args.AbsolutePath`, o path chega JSON-decodificado (sem aspas
residuais). A remoção produz um path corrupto para nomes de arquivo com `'` literal, como
`/projeto/it's.env` → `/projeto/its.env`.

**Medido com o binário** (`go build -o <scratchpad>/trackfw ./cmd/trackfw`),
em diretório temporário com `trackfw.yaml` em modo `block`, HOME/USERPROFILE/CODEX_HOME
temporários:

```
Payload controle (plain.env com credencial):
  trackfw-credential-guard: blocked - ... plain.env ...
  exit=2  (correto)

Payload teste (it's.env com credencial, its.env inexistente):
  exit=0  ← falso negativo confirmado
```

O guard abre `its.env`, não encontra o arquivo, e libera o `view_file`. A credencial em
`it's.env` não é detectada.

Essa é uma regressão introduzida por ML-3A: `credScanFile` existia antes e nunca recebeu paths
JSON-decodificados. `credViewFileScan` (linha 790) é código novo desta branch e é o único sítio
onde um path JSON-decodificado passa pelo pipeline de cleanup de tokens shell.

**Recomendação (Regra Dura de Causa Raiz — mesma REQ, mesmo PR):** adicionar ML corretivo na
REQ-2026-11-17. Correção: em `credViewFileScan`, resolver o caminho sem `credResolveArg` —
apenas `filepath.Clean` + `credNormalizeWindowsPath` para Windows — antes de chamar
`credScanFile`. Alternativamente, criar `credScanFilePath(path string)` que pule o `strings.Map`.
O PR permanece aberto até este ML entrar.

---

### A2 — Testes do git-branch guard não cobrem o CWD real de produção (Nota · Médio)

**Arquivo:linha:** `internal/guard/antigravity_test.go:93` e `:110`

`TestGitBranchAntigravityPayload_Push` e `TestGitBranchAntigravityPayload_AllowLs` chamam
`runWith(t, makeProjectDir(t), payload, "")` — CWD do teste é a raiz do projeto.

Em produção, o Antigravity executa o hook com CWD = `<projeto>/.agents/`. `RunGitBranch`
(`gitbranch.go:107`) usa `os.Getwd()` (linha 113) + `FindProjectRoot` (linha 122, definida em
224), que percorre subindo. De `.agents/` ele percorre até o pai e encontra `trackfw.yaml` —
correto. Mas nenhum teste exercita esse caminho. Se `FindProjectRoot` for modificado para não
percorrer, o guard silencia para o Antigravity sem que nenhum teste existente quebre.

**Recomendação:** adicionar variante em `TestGitBranchAntigravityPayload_Push` onde CWD =
`agentsDir` (subdir do projeto) e payload tem `git push` → espera rc=2. Sem isso, o A.C.
"git-branch: payload Antigravity com `git push` → exit 2" foi atestado com CWD diferente do
real.

---

### A3 — Três parsers independentes para o schema Antigravity (Duplicação · Médio)

**Arquivo:linha:**
- `internal/auditsurface/auditsurface.go:723` — `extractAntigravitySchema`
- `internal/validator/validator_credential_guard.go:366` — `collectCommandsWithMarkerBySchema` (branch `hookCmdSchemaAntigravity`)
- `internal/validator/validator_guard_wiring.go:329` — `guardWiringExtractTuples` (branch `hookCmdSchemaAntigravity`)

Os três implementam independentemente o mesmo algoritmo de 3 níveis:
`group → eventMap → []entry → hooks[].command`. Código duplicado literal — mesmo padrão de range,
mesma asserção de tipo `map[string]interface{}` / `map[string]any`, mesmas guardas. Se o formato
do `.agents/hooks.json` mudar (ex.: adição de subnível ou renomeação de chave), os três devem
ser atualizados em sincronia. Uma mudança em apenas dois dos três produz comportamento divergente
sem erro de compilação.

**Recomendação:** extrair a travessia do schema para uma função auxiliar
`walkAntigravityEntries(parsed, fn func(event, matcher, cmd string))` em `validator_guard_wiring.go`
ou num arquivo de suporte. Os três chamadores passam callbacks distintos — o algoritmo de
travessia em si pode ser único.

Não bloqueia merge. Deve ser planejado como ML de refactoring na REQ vigente ou na próxima.

---

### A4 — `payloadCwd` não é validado como absoluto antes de uso como `projectCwd` (Nota · Médio)

**Arquivo:linha:** `internal/guard/credential.go:227-230`

```go
if isAntig && payloadCwd != "" {
    projectCwd = payloadCwd
}
if _, err := os.Stat(filepath.Join(projectCwd, "trackfw.yaml")); err != nil {
    return 0
}
```

Sem `filepath.IsAbs(payloadCwd)`, um payload com `"cwd": "../../../tmp"` faz `projectCwd` virar
caminho relativo ao CWD do processo (`.agents/`). Medição §11 do contrato de hooks confirma que
o runtime do Antigravity sempre entrega caminho absoluto, logo não há exploração operacional.
Hardening defensivo recomendado antes do merge global.

---

### A5 — `os.Stat` segue symlinks em `credScanFile` (correção factual, sem bug)

**Arquivo:linha:** `internal/guard/credential.go:609-611`

```go
fi, err := os.Stat(path)
if err != nil || !fi.Mode().IsRegular() {
```

`os.Stat` segue symlinks. `fi.Mode().IsRegular()` retorna `true` para symlink apontando para
arquivo regular. Portanto `credViewFileScan` lê através de symlinks — comportamento correto
(o guard deve detectar credenciais em arquivos referenciados por symlink). FIFOs e sockets
retornam `IsRegular()=false` e são rejeitados sem bloquear. O único risco TOCTOU
(regular → FIFO entre `Stat` e `ReadFile`) preexiste ao branch; `internal/fileutil.ReadRegularFile`
oferece proteção real via O_NONBLOCK mas não é usado por `internal/guard` — padrão anterior,
não regressão.

---

### A6 — `countAntigravityGroup` descarta o parâmetro `event` (Nota · Baixo)

**Arquivo:linha:** `internal/generators/guard_registry_sites_ac4_test.go:73`

```go
func countAntigravityGroup(root map[string]interface{}, _, matcher, cmd string) int {
```

Conta `(matcher, cmd)` em todos os eventos — não verifica que a entrada está sob `PreToolUse`.
Para o uso corrente de AC4 (dedup/migração) é suficiente.
`TestGuardHookConcordance_D17_Antigravity_PresenceWarning` cobre a verificação de evento no
validator. Lacuna de cobertura não crítica.

---

### A7 — Mensagem de `gemini_cli_legacy` em português; sibling em inglês (Nota · Baixo)

**Arquivo:linha:** `internal/validator/validator_gemini_cli_legacy.go:46`

A mensagem começa com `"Gemini CLI é legado para contas individuais..."`. As mensagens
equivalentes de presença no `validator_credential_guard.go:1061` estão em inglês
(`"is present but has no credential guard entry in event %s — ..."`). Inconsistência de idioma
na saída do `validate`/`doctor`. Não bloqueia; pode ser padronizado num ML de polish.

---

## Verificações das questões específicas do handoff

### Q1 — Prioridade 5 e `isAntigravityPayload` alteram veredito para Claude/Codex/Gemini?

Não. P5 (`toolCall.args.CommandLine`, adicionada em `payload.go:201`) fica após P4
(`hook_input.command`). Schemas Claude/Codex/Gemini usam `tool_input.command` (P1), `command`
root (P2) ou `tool_info.command_line` (P3). `isAntigravityPayload` requer `toolCall` como objeto
JSON e `hookEventName` presentes simultaneamente; Claude Code usa `tool_name` + `tool_input`,
sem `hookEventName`. O teste `TestExtractCommand_Priority5_DoesNotOverrideHigherPriority` afirma
isso explicitamente. `extractNested3` está definida em `payload.go:219` como extensão de 3 níveis
de `extractNested` (2 níveis, `payload.go:277`) — não é duplicação, é extensão documentada.

### Q2 — D4: brecha com toolCall não-objeto, chaves duplicadas ou payload misto?

**toolCall não-objeto:** `isAntigravityPayload` verifica `trimmedTC[0] == '{'`. String ou número
→ false → payload completo escaneado pela camada 1. ✓

**hookEventName presente, toolCall null/ausente:** `isJSONNull` detecta e retorna false. ✓

**Payload com toolCall E tool_input:** layer 1 é estreitada a `toolCall` (D4 — decisão explícita
do KG: `tool_input` não é escaneado). `tool_input.command` ainda é extraído pela extração de
comando (P1). Lacuna real e intencional conforme ADR: um segredo embutido em
`tool_input.argumento_arbitrário` (não em `command`) não seria detectado pela camada 1. O modelo
controla o conteúdo de `tool_input`, não um atacante.

**Chaves duplicadas JSON:** `json.Unmarshal` do Go toma o último valor — sem vulnerabilidade
específica ao Antigravity.

### Q3 — Substituição do CWD do processo pelo `cwd` do payload é segura?

**Medido (§11 do contrato de hooks):** o runtime do Antigravity entrega no campo `cwd` a raiz
do workspace — não um subdiretório. O guard de credencial faz
`os.Stat(filepath.Join(payloadCwd, "trackfw.yaml"))` (linha 230) — verificação direta, sem
walk-up. O guard de git-branch usa `FindProjectRoot(os.Getwd())` onde `os.Getwd()` retorna
`.agents/` (walk-up). Ambos chegam ao mesmo `trackfw.yaml` por caminhos distintos. Quando não
encontrado: ambos retornam 0 (fail-open consistente). Sem assimetria de comportamento para o
caso medido. Hardening para cwd relativo: ver A4.

### Q4 — `credViewFileScan` reutiliza as proteções de leitura da camada 2?

Parcialmente. `credViewFileScan` (linha 790) chama `credResolveArg` (linha 803) + `credScanFile`
(linha 804) — a mesma sequência da camada 2 de leitura via tokens shell. `credScanFile` usa
`os.Stat` + `fi.Mode().IsRegular()` + limite de 1 MiB + `os.ReadFile`.

Diferença em relação a `fileutil.ReadRegularFile`: janela TOCTOU entre `Stat` e `ReadFile`
(ausente em `fileutil` via O_NONBLOCK). Essa limitação preexiste ao branch; `internal/guard`
nunca usou `fileutil`. Nenhuma regressão introduzida por essa diferença.

O desvio identificado em A1 (stripping de aspas em path JSON-decodificado) representa uma
divergência real: a camada 2 recebe tokens de shell onde aspas são ruído legítimo; aqui o path
está pré-decodificado pelo JSON e não carrega aspas. Duplicação de código combinada com semântica
diferente do input — essa combinação produziu o bug.

**Duplicação de parsers de schema:** ver A3.

### Q5 — `InjectAntigravityHooks`: coesão, dedup/migração, terceiros?

**Coesão:** segue o padrão de `InjectAmazonQHooks` — `pathguard.ResolveRoot`,
`pathguard.RejectAndReport`, `os.MkdirAll`, read→parse→merge→write. ✓

**Dedup:** `migrateDedupGuardInNestedArray` normaliza formas PSPOSIX e CmdExe legadas.
`removeGuardFromNestedArrayInHooks` remove o git-branch da posição errada antes de reinserí-lo
no matcher correto (`run_command`).
`gitBranchLegacyAll` inclui `GitBranchCurrentPSPOSIX` como forma legacy — correto, PSPOSIX é
família inválida para Antigravity. ✓

**Preservação de terceiros:** leitura via `map[string]interface{}` preserva chaves desconhecidas.
`appendAntigravityHookEntry` opera só dentro de `root["trackfw"]`.
`TestInjectAntigravityHooks_PreservesThirdParty` confirma. ✓

**Idempotência:** `TestInjectAntigravityHooks_Idempotent` confirma saída byte-a-byte igual na
segunda execução. ✓

### Q6 — Validator: `hookCmdSchemaAntigravity` nos dois sítios espelho; `gemini_cli_legacy` sempre warning?

**Dois sítios:** `ValidateUnfiltered` (linha 1355) e `validateUnfilteredTagged` (linha 1919)
ambos chamam `applyRuleWarnOnly`/`applyRuleWarnOnlyTagged`. ✓

**Severidade fixa:** `applyRuleWarnOnly` (linha 620-625):

```go
func applyRuleWarnOnly(ruleName string, msgs []string, warnings *[]string) {
    if len(msgs) == 0 || ruleSeverity(ruleName) == "off" {
        return
    }
    *warnings = append(*warnings, msgs...)  // sempre warnings, nunca violations
}
```

Mesmo com `rules: { gemini_cli_legacy: error }` no `trackfw.yaml`, a função insere em
`*warnings` e nunca em violations. A regra pode ser silenciada com `"off"` mas não pode ser
promovida a violation. O teste `TestGeminiCliLegacy_StrictMode_StillWarning` confirma via
`rules: error`. ✓

**`hookCmdSchemaAntigravity`** registrada em todos os sítios obrigatórios:
`guardWiringHookLayout` (`validator_guard_wiring.go:110`),
`credentialGuardHookFiles`, `credentialGuardRequiredEntries`,
`collectCommandsWithMarkerBySchema` (linha 366), `extractPresenceCommandsD17`,
`guardWiringExtractTuples` (linha 329), `guardWiringExtractCommandsFromEntry`,
`export_test.go`. ✓

### Q7 — Regra de Reconciliação: cada teste novo afirma algo do ML?

Todos os testes têm cabeçalho `// Conclusion asserted:` explícito. Verificação cruzada:

| Teste | ML | Conclusão declarada | Consistente? |
|---|---|---|---|
| `TestGitBranchAntigravityPayload_Push` | 3A | P5 fecha T2 silencioso | ✓ (CWD errado — ver A2) |
| `TestGitBranchAntigravityPayload_AllowLs` | 3A | contra-braço ls → 0 | ✓ (CWD errado — ver A2) |
| `TestCredentialAntigravityPayload_SecretInCommandLine` | 3A | layer-1 de toolCall detecta | ✓ |
| `TestCredentialAntigravityPayload_SecretOnlyInLastUserInput` | 3A | D4 não escaneia lastUserInput | ✓ |
| `TestCredentialAntigravityPayload_Layer2Cat` | 3A | layer-2b via P5 | ✓ |
| `TestCredentialAntigravityPayload_ViewFileWithSecret` | 3A | credViewFileScan ativo | ✓ mas não cobre path com `'` (gap A1) |
| `TestCredentialAntigravityPayload_ViewFileInnocuous` | 3A | contra-braço view_file | ✓ |
| `TestGuardStdoutEmpty_D2Invariant` | 3A | D2: stdout vazio em exit 0 | ✓ |
| `TestExtractTuplesAntigravity` | 3B corretivo | matcher git-branch = "run_command" | ✓ |
| `TestInjectAntigravityHooks_Create` | 3B | 4 entradas D5 com matchers corretos | ✓ |
| `TestInjectAntigravityHooks_MigratesGitBranchMatcher` | 3B corretivo | migração do matcher errado | ✓ |
| `TestGuardHookConcordance_Antigravity_Credential` | 3C | gerador ↔ validator concordam | ✓ |
| `TestGeminiCliLegacy_GeminiGuardNoAntigravity_Warns` | 3C | regra D7 dispara no cenário correto | ✓ |
| `TestGeminiCliLegacy_StrictMode_StillWarning` | 3C | D7 sempre warning, nunca violation | ✓ |

---

## Itens fora do escopo desta revisão

- ML-3B2 (guard global do Antigravity) — pendente de ML-1B; não está no diff.
- Validação end-to-end com `agy` real (ML-4A, pendente do KG).
- `docs/cli-parity.md` e `catalog.json` (ML-3D, fora de `internal/`).

---

## Resultado

Todos os gates passaram. Achados:

| ID | Severidade | Natureza | Bloqueia merge? |
|---|---|---|---|
| A1 | Médio | Bug confirmado com binário: strip de aspas em path JSON-decodificado → falso negativo para arquivos com `'` no nome | **Sim — ML corretivo na REQ-2026-11-17 antes do merge** |
| A2 | Médio | Lacuna de teste: CWD real de produção (`.agents/`) não coberto pelos testes do git-branch guard | Não — código correto; testes cobrem o assert com CWD diferente |
| A3 | Médio | Duplicação: 3 parsers independentes para o schema Antigravity | Não — risco de drift futuro |
| A4 | Médio | `filepath.IsAbs` ausente em `payloadCwd` antes de substituir `projectCwd` | Não — sem impacto no caso real medido |
| A5 | — | Correção factual: `os.Stat` segue symlinks; comportamento correto | — |
| A6 | Baixo | `countAntigravityGroup` descarta evento no AC4 | Não |
| A7 | Baixo | Mensagem de `gemini_cli_legacy` em PT; siblings em EN | Não |

**A1 bloqueia o merge.** O resto são notas de melhoria.

O código está arquiteturalmente correto, coeso com os padrões do repositório, e os testes
reconciliam as conclusões dos MLs (com a ressalva de A2 sobre o CWD dos testes do git-branch).
**APROVA COM NOTAS — merge condicionado ao ML corretivo de A1.**
