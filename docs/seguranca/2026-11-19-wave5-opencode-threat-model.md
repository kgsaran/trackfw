# Threat Model — Guard de Credencial no OpenCode via Plugin JS/TS
> Wave 5 · ML-5-0 · hades-tf · 2026-11-19
> REQ: REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md

---

## 1. Completude — Enumeração das Superfícies

**Método de enumeração:** grep recursivo por `opencode`, `\.opencode`, `tool.execute`, `plugin` em
`internal/` (excluindo `_test.go`) + leitura de `internal/generators/hooks.go`,
`internal/guard/writescope.go`, `internal/guard/payload.go`,
`internal/integrations/assets/catalog.json`, `internal/generators/update.go`,
`internal/auditsurface/auditsurface.go`, `internal/agentconfig/agentconfig.go`,
`internal/commands/doctor.go`. Fontes de contrato do OpenCode: `docs/portabilidade/2026-11-19-opencode-contrato-de-plugin.md` (prometeu-tf, ML-5A) como fonte primária de API.

**Desvio de escopo:** o despacho autorizou `opencode --version` e `opencode --help`. Os subcomandos
`opencode debug paths`, `debug config`, `debug v2`, `debug skill` e `debug agent` também foram
executados (opencode 1.19.5) como leitura pura sem chamada de modelo. Nenhum arquivo em
`~/.config/opencode/` foi modificado (timestamps: Jul 3–4, confirmados após a sessão). O KG deve
saber do desvio antes de aprovar o threat model.

---

### 1.1 `cliDetectors` — Dispatcher de injeção de hooks

**Arquivo:** `internal/generators/hooks.go:16`

```go
// CLIs without an Inject*Hooks implementation (opencode) are intentionally
// absent — callers that pass unknown names get a silent skip.
```

O OpenCode está **explicitamente ausente** do mapa `cliDetectors`. Qualquer chamada a
`InjectHooksForTools(["opencode"], ...)` é um silent skip. Confirmado pelo teste
`contra_opencode_still_skips` (`internal/generators/antigravity_hooks_test.go:310`) —
invariante com asserção, não comentário. A Wave 5 é quem muda esse teste.

---

### 1.2 Harness targets — `HarnessTargetIDs` / `harnessCatalogTargetOrder`

**Arquivo:** `internal/generators/update.go:511`

O OpenCode aparece na ordem do catálogo para agents/skills mas **não existem targets
`opencode-credential-guard` nem `opencode-git-branch-guard`**. O `update harness` não instala
guard global para o OpenCode.

---

### 1.3 Catalog — caminhos de agentes/skills, sem hook

**Arquivo:** `internal/integrations/assets/catalog.json` (entrada `"id":"opencode"`)

Caminhos definidos para `agents` e `skills`. **Nenhuma entrada de hook, credential-guard ou
git-branch-guard.** O catálogo confirma que o OpenCode foi adicionado apenas para agents/skills
(REQ-2026-08-04, status: Done).

---

### 1.4 `writeScopeSubagentFrozenRelPaths` — Proteção de escrita via write-scope guard

**Arquivo:** `internal/guard/writescope.go:61–76`

Nenhum caminho OpenCode está na lista de caminhos congelados. Especificamente ausentes:
- `.opencode/plugins/` e `.opencode/plugin/` — diretórios de auto-carregamento (V2 descobre de ambos)
- `opencode.json` / `opencode.jsonc` a nível de projeto (raiz e `.opencode/`)
- `.opencode/opencode.json` / `.opencode/opencode.jsonc` — override de projeto (fonte: prometeu §1.1)
- `~/.config/opencode/opencode.json` — global (fora da raiz do projeto por definição)

---

### 1.5 Validators — Auditoria de hooks instalados

**Arquivos:** `internal/validator/validator_credential_guard.go`,
`internal/validator/validator_git_branch_guard.go`

Nenhuma verificação de caminho de hook do OpenCode. `trackfw validate` passa verde mesmo sem
qualquer guard do OpenCode instalado (medido nesta sessão: rc=0, 0 violations).

---

### 1.6 Mecanismo de plugin do OpenCode — gap arquitetural central

**Fonte primária:** `docs/portabilidade/2026-11-19-opencode-contrato-de-plugin.md` §2 e §3.

O mecanismo de hook do OpenCode é **fundamentalmente diferente** dos 9 outros CLIs suportados:

| Aspecto | Claude/Codex/Gemini/Antigravity (outros 9) | OpenCode V1 (1.19.x) | OpenCode V2 (2.x) |
|---|---|---|---|
| Formato de hook | JSON declarativo + script shell | Plugin JS/TS — `"tool.execute.before"` | Plugin JS/TS — `ctx.tool.hook("execute.before", ...)` |
| API incompatível | — | `server()` + `Hooks` object | `Plugin.define({ id, setup })` |
| Mecanismo de bloqueio | `exit 2` (fail-closed) | `throw` (fail-closed via Effect.promise) | `throw` (fail-closed via Effect.tryPromise) |
| ID do tool shell | — | `"bash"` | `"shell"` (renomeado em 2.0) |
| `agent` no hook | — | ❌ ausente no `tool.execute.before` V1 | ✅ `event.agent: Agent.ID` |
| Registro global | arquivo JSON | `plugin: [...]` em opencode.json | `plugins: [...]` + auto-descoberta |
| Bypass por flag | — | `--pure` / `OPENCODE_PURE=1` | idem |

**Mecanismo de bloqueio (confirmado por fonte primária):**

V2 — `packages/plugin/src/promise/adapter.ts@v2.3.0`:
```ts
const toolError = (error: unknown) =>
  error instanceof Tool.Error ? error
  : new Tool.Error({ message: error instanceof Error ? error.message : String(error), error })
```
O callback é envolvido em `Effect.tryPromise({ try: ..., catch: toolError })`. Lançar **qualquer**
valor bloqueia o tool call.

V1 — `packages/opencode/src/plugin/index.ts@v1.19.5`:
```ts
yield* Effect.promise(async () => fn(input, output))
```
Lançar da promise bloqueia o tool call. Exemplo literal de `migrate-v1.md`:
```ts
"tool.execute.before": async (input, output) => {
  if (input.tool === "read" && output.args.filePath.includes(".env")) {
    throw new Error("Do not read .env files")
  }
},
```

O risco **não** é "throw não bloqueia". O risco está nos **três casos de falha** (prometeu §3):
- **Caso A** — throw bloqueia ✅ (correto, fail-closed)
- **Caso B** — plugin não carrega → hook nunca registrado → fail-open ❌
- **Caso C** — binário guard ausente + verificação ingênua → fail-open ❌

**Post-execution hook:** Prometeu §4 e §7 M20 documentam `execute.after` como existente
em V1 e V2, mas com canal de falha `never` — NÃO pode bloquear. Equivalente ao `PostToolUse` em
Claude Code, que registra o credential guard nos mesmos 3 matchers (`agentfiles.go:427–429`). O
OpenCode plugin para `execute.after` só pode fazer auditoria (warn-only) após a execução. Ausência
de bloqueio em `execute.after` é por construção (prometeu §4, confirmado por `hooks.ts@v2.3.0`).
Para o threat model, a proteção real é em `execute.before` (pre); `execute.after` é belt-and-
suspenders. (Canal `never` atribuído a prometeu §4; não lido diretamente em `hooks.ts`.) Declarado em R9.

---

### 1.7 `trackfw.ts` em `~/.config/opencode/tools/`

**Arquivo:** `~/.config/opencode/tools/trackfw.ts` (user-authored — **não entregue pelo produto
trackfw**; grep em `internal/` e `scripts/` não encontrou referência; timestamps: Jul 3–4, não
modificado nesta sessão)

Expõe o CLI `trackfw` como custom tool do OpenCode. **Não é um guard.** Contém shell injection
(`sh -c "trackfw ${cmd}"` com `cmd` controlado pelo modelo) e fail-open quando `trackfw` não está
no PATH — ver R4.

---

### 1.8 `debug paths` / `debug config` — carregamento confirmado

**Fonte:** `opencode debug paths 2>&1`, `opencode debug config 2>&1` (opencode 1.19.5):

- Diretório de config global: `/Users/kgsaran/.config/opencode`
- `plugin: []` — nenhum plugin registrado via `opencode.json`
- `tools/trackfw.ts` não aparece em `plugin: []` (mecanismo de `tools/` é separado)

---

### 1.9 `auditsurface.go` — `canonicalRuntimes` não inclui OpenCode

**Arquivo:** `internal/auditsurface/auditsurface.go:31–50`

```go
var canonicalRuntimes = []string{
    "claude", "codex", "gemini", "copilot", "cursor", "kiro", "windsurf", "amazonq", "antigravity",
}
// opencode: ausente
```

OpenCode está ausente dos 9 runtimes canônicos. O audit surface não tem wiring path definido para
o OpenCode. `trackfw validate` não verifica integridade de hooks do OpenCode via esta surface.

---

### 1.10 `agentconfig.go` — sem constante `CLIOpenCode`

**Arquivo:** `internal/agentconfig/agentconfig.go:41–44`

```go
CLIClaude = "claude"
// CLICodex, CLIGemini, CLIAntigravity...
// CLIOpenCode: ausente
```

Nenhuma constante `CLIOpenCode` existe. `HomeFor("opencode")` cai no path genérico sem tratamento
Windows específico. Para o guard global (ML-5C), será necessário adicionar `CLIOpenCode` e tratar
o caminho XDG Windows (`C:\Users\<user>\.config\opencode\`, **não** `%APPDATA%`).

---

### 1.11 `doctor.go` — sem regra de diagnóstico para o OpenCode

**Arquivo:** `internal/commands/doctor.go:141–155, 372`

O `doctor` tem regras `D7` (gemini-cli-legacy) e `D8` (antigravity-ide-global-guard) mas nenhuma
regra `D*` para o OpenCode. `trackfw doctor` não avisa sobre ausência de guard no OpenCode.

---

### 1.12 `writeScopeWriteTools` — case sensitivity bloqueia apenas PascalCase

**Arquivo:** `internal/guard/writescope.go:45–48, 131–133`

```go
var writeScopeWriteTools = map[string]struct{}{
    "Write":        {},
    "Edit":         {},
    "NotebookEdit": {},
}
// Step 4: tool_name not in write tools → allow.
if _, isWrite := writeScopeWriteTools[toolName]; !isWrite {
    return 0 // allow
}
```

O guard usa **comparação exata** (case-sensitive). Os tool names do OpenCode são lowercase
(`"write"`, `"edit"`, `"shell"`). Como `"write"` ≠ `"Write"`, o Step 4 retorna `allow` imediatamente
sem verificar os frozen paths — mesmo que `.opencode/plugins/` seja adicionado a
`writeScopeSubagentFrozenRelPaths` no futuro.

**Medido nesta sessão com branch binary v11.7.0 (working-tree build em `1f8e82c0`, não lançado;**
**instalado 11.5.0 não tem a entrada ML-FA2 de `.agents/hooks.json`):**
```bash
# tool_name "Write" (PascalCase) + agent_id + .agents/hooks.json → BLOQUEADO
printf '{"tool_name":"Write","agent_id":"hades-tf","tool_input":{"file_path":"<tmpdir>/.agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=2 [BLOQUEADO — Step 10, frozen path]

# tool_name "write" (lowercase OpenCode) + agent_id + .agents/hooks.json → ALLOW
printf '{"tool_name":"write","agent_id":"hades-tf","tool_input":{"file_path":"<tmpdir>/.agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=0 [ALLOW — Step 4, lowercase não está em writeScopeWriteTools]
```
(O `trackfw.yaml` vazio foi criado em `<tmpdir>/` via `mktemp -d` para que `findProjectRoot`
encontrasse a raiz. Caminhos reais com `/private/tmp/...` abreviados como `<tmpdir>` acima.)

**Cinco pré-requisitos independentes para que write-scope proteja efetivamente caminhos OpenCode
(medidos nesta sessão):**

1. Adicionar `.opencode/plugins/` etc. a `writeScopeSubagentFrozenRelPaths` (ainda ausente, §1.4)
2. Aceitar tool names lowercase em `writeScopeWriteTools` OU o plugin mapear `write→Write` etc.
   (ainda ausente — Step 4 retorna allow para lowercase, medido acima)
3. Plugin encaminha `agent_id` somente em chamadas de subagente — `writeScopeSubagentFrozenRelPaths`
   só se aplica quando `isSubagent = payloadHasAgentID(root)` é true. Em V1, o hook
   `tool.execute.before` não expõe `agent` (prometeu §2.2). Em V2, `event.agent: Agent.ID` está
   presente **em toda chamada** (non-optional) — encaminhar incondicionalmente tornaria sessão
   principal "subagente", bloqueando `trackfw.yaml`. Discriminador a medir (ML-5B braço A2)
4. O valor do caminho deve ser **absoluto** — write-scope rejeita caminhos relativos com rc=2
   "not an absolute path". O plugin deve resolver `ctx.location.directory + path` para absoluto
   antes de enviar ao guard (medido nesta sessão)
5. Campo de caminho deve ser `file_path` — não `filePath` (V1) nem `path` (V2). Enviar com chave
   errada → rc=2 "tool_input.file_path absent" (bloqueia **toda** Write call, não apenas frozen
   paths — falso positivo, medido nesta sessão com `filePath` e `path`)

**Risco de falso positivo (pré-req 5):** um plugin que use `filePath` ou `path` sem mapear para
`file_path` bloqueará 100% das Write/Edit calls com "tool_input.file_path absent" — independente
do conteúdo ou destino. É fail-closed mas impede qualquer uso do OpenCode com escrita ativa.

---

### 1.13 Mapeamento de superfícies para o roadmap Wave 5

O roadmap define: ML-5A (contrato, prometeu), ML-5B (medição no binário real, KG+zeus), ML-5C (ADR),
ML-5D (implementação + red team, indefinido até após ML-5C).

| Superfície (§) | Coberta por | Observação |
|---|---|---|
| §1.1 cliDetectors ausente | ML-5D | InjectOpenCodeHooks a implementar |
| §1.2 Harness targets ausentes | ML-5D | targets `opencode-credential-guard`, `opencode-git-branch-guard` |
| §1.3 Catalog sem hook | ML-5D | entrada `hook` no catálogo |
| §1.4 writeScopeSubagentFrozenRelPaths | ML-5C (decisão ADR) + ML-5D | 5 pré-requisitos (R7) |
| §1.5 Validators ausentes | ML-5D | regras credential e git-branch para OpenCode |
| §1.6 Mecanismo de plugin (Casos A/B/C) | ML-5A + ML-5B + ML-5C + ML-5D | loop completo |
| §1.7 trackfw.ts user-authored | ML-5D (opcional, se ADR decidir) | shell injection, R4 |
| §1.8 debug paths data | ML-5A | confirmado |
| §1.9 canonicalRuntimes ausente | **não nomeado no roadmap** | ML-5D natural, declarar |
| §1.10 CLIOpenCode ausente | **não nomeado no roadmap** | ML-5D natural, declarar |
| §1.11 doctor rule ausente | **não nomeado no roadmap** | ML-5D natural; template: D8 |
| §1.12 writeScopeWriteTools lowercase | **não nomeado no roadmap** | mudança cross-CLI, precisa ML ou nota ADR |

**Superfícies não explicitamente cobertas pelo roadmap (a declarar ao arquiteto):**

1. `canonicalRuntimes` em `auditsurface.go` — adicionar `"opencode"` para fechar gap de audit surface
2. Constante `CLIOpenCode` em `agentconfig.go` — necessária para HomeFor e caminhos Windows
3. Regra `doctor` para OpenCode — template disponível: `D8` (`antigravity_ide_global_guard`)
4. `writeScopeWriteTools` aceitar lowercase — alternativa 1: mudança no `writescope.go` (cross-CLI, afeta mapa compartilhado); alternativa 2: o plugin mapeia para PascalCase antes de chamar o guard (dentro-do-plugin, sem mudança de código compartilhado). A alternativa 2 não requer mudança de código fora do plugin; a ADR ML-5C decide qual usar
5. V2 tool `patch` — pode escrever em frozen paths; não tem entrada em `writeScopeWriteTools` (nenhum PascalCase que cobrisse); o credential guard deve incluí-lo explicitamente (inferido — a confirmar no ML-5B braço A2)

Nenhuma dessas superfícies abre REQ nova — mesma causa (OpenCode sem guard), mesma REQ vigente.
São candidatos a MLs dentro de ML-5D ou notas na ADR ML-5C.

---

### Resumo de completude

| Superfície | Claude | Codex | Gemini | Antigravity | OpenCode |
|---|:---:|:---:|:---:|:---:|:---:|
| `cliDetectors` (hook injector) | ✅ | ✅ | ✅ | ✅ (ML-3B) | ❌ ausente |
| `Inject*Hooks` (project scope) | ✅ | ✅ | ✅ | ✅ (ML-3B) | ❌ ausente |
| Harness target global credential | ✅ | ✅ | ✅ | ✅ (`update.go:578` `antigravity-credential-guard`) | ❌ ausente |
| Harness target global git-branch | ✅ | ✅ | ✅ | ✅ (`update.go:578` `antigravity-git-branch-guard`) | ❌ ausente |
| `writeScopeSubagentFrozenRelPaths` hook config | ✅ `.claude/settings.json` | ✅ `.codex/hooks.json` | ✅ `.gemini/settings.json` | ✅ `.agents/hooks.json` | ❌ `.opencode/plugins/` ausente |
| `writeScopeWriteTools` wired para este CLI | ✅ `Write`/`Edit` (InjectClaudeHooks) | N/A não fiado | N/A não fiado | N/A não fiado | ❌ não fiado; lowercase bypassa Step 4 |
| `canonicalRuntimes` / `runtimeWiringPath` | ✅ | ✅ | ✅ | ✅ | ❌ ausente |
| Constante `CLI*` em `agentconfig.go` | ✅ | ✅ | ✅ | ✅ | ❌ ausente |
| Validator credential audit | ✅ | ✅ | ✅ | ❌ | ❌ ausente |
| Validator git-branch audit | ✅ | ✅ | ✅ | ❌ | ❌ ausente |
| `doctor` script presence (universal, `scaffold_doctor.go:321`) | ✅ verifica `scripts/trackfw-credential-guard.sh` | ✅ idem | ✅ idem | ✅ idem | ✅ verifica o mesmo script; nota: script não é chamado pelo plugin OpenCode — detecção universal, não prova wiring |
| `doctor` per-CLI advisory rule | ❌ | ❌ | ✅ D7 (advisory: Gemini guard sem Antigravity; migração, não wiring) | ✅ D8 (IDE sem guard global) | ❌ ausente |
| Mecanismo de bloqueio nativo | exit 2 | exit 2 | exit 2 | exit 2 | throw (Casos B+C fail-open) |
| Bypass por flag/variável | — | — | — | — | `--pure` / `OPENCODE_PURE=1` |

---

## 2. Modelo de Ameaças

O adversário aqui é o **implementador apressado** que copia `InjectAntigravityHooks`, injeta o
comando do shell como string em um arquivo `.ts` e verifica apenas `result.status === 2` para
decidir se bloqueia — sem tratar `result.error` (binário ausente, Caso C) nem Caso B (plugin não
carregou).

O segundo adversário é o **arquiteto otimista** que vê `throw` bloqueando em V2 e assume que a
geração do plugin é simples — esquecendo que V1 usa `server()` + `Hooks` object (API incompatível
com V2), que o tool ID do shell difere entre versões (`"bash"` vs `"shell"`), que `event.agent`
não existe em V1, e que o payload de `event.input` precisa ser mapeado para `tool_input.command`
antes de enviar ao guard.

---

### T1 — `tool.execute.before` bloqueia via throw; risco é fail-open nos casos de erro (ALTO)

**Mecanismo (confirmado por fonte primária — prometeu §3):**

Throw **bloqueia** em ambas as versões — fail-closed por construção:
- V2: `Effect.tryPromise({ try: ..., catch: toolError })` — qualquer lançamento → `Tool.Error` → tool bloqueado
- V1: `Effect.promise(async () => fn(input, output))` — throw da promise → Effect falha → tool bloqueado

O risco real está nos **casos de falha do próprio guard**:

**Caso B — plugin não carrega (fail-open):**
Se o plugin tiver erro de sintaxe, import não resolvido, ou entry point não encontrado, o hook
`execute.before` **nunca é registrado**. O OpenCode prossegue sem o plugin; nenhum bloqueio ocorre.

**Caso C — binário guard ausente + verificação ingênua (fail-open, medido):**
```ts
// ERRADO: fail-open quando binário está ausente
const result = spawnSync("trackfw", ["guard", "git-branch"], { input: payload, encoding: "utf8" })
if (result.status === 2) throw new Error("bloqueado")
```
Medido com `node:child_process.spawnSync` (nesta sessão): quando o binário não existe,
`result.status = null` (não 2) e `result.error.code = "ENOENT"`. A verificação `=== 2` retorna
`false` → nenhum throw → fail-open. (Prometeu §4.2 usa `node:child_process` que é compatível com
Bun; comportamento de Bun.spawnSync não medido por falta de Bun em PATH — a confirmar no ML-5B.)

**Padrão correto (prometeu §3.1 Caso C):**
```ts
if (result.error || result.status !== 0) {
  throw new Error(result.error
    ? `trackfw não encontrado: ${result.error.message}`
    : result.stdout?.trim() || "trackfw: operação bloqueada")
}
```

Este padrão é o equivalente JS/TS da política já estabelecida nas formas PSPOSIX em
`internal/guardforms/guard_generations.go:67` (`CredentialCurrentPSPOSIX`, excerto):
```bash
$LASTEXITCODE=2 2>$null; trackfw guard credential; LASTEXITCODE=$((2*!!$?))
```
A forma tem dois braços de fail-closed para binário ausente:
- **PowerShell:** `$LASTEXITCODE=2 2>$null` pre-define o código de saída como 2. Se `trackfw` não
  for encontrado (EAP=Continue, CommandNotFound não-terminante), `$LASTEXITCODE` permanece em 2.
- **POSIX:** `$?=127` para comando não encontrado → `LASTEXITCODE=$((2*!!127)) = $((2*1)) = 2` →
  fail-closed.
O plugin JS/TS deve replicar a mesma política: qualquer `result.error` (incluindo `ENOENT`) lança
exceção, bloqueando o tool call.

**Aviso de implementação ML-5D — não usar `execSync` ou `sh -c` para chamar o guard:**
Um implementador apressado pode copiar a string poliglota do guardforms e passar para `execSync` ou
`sh -c`. Isso importa o defeito D18: se o shell do processo OpenCode herdar `errexit`/`nounset` (via
`BASH_ENV`, `SHELLOPTS` ou envolvimento em script com `set -e`), a string poliglota aborta com 127
antes de chamar o guard — fail-open silencioso (REQ-2026-11-13, medido com `bash -e` nesta sessão).
O plugin deve usar **argv array direto** (`Bun.spawnSync(['trackfw', 'guard', 'credential'], ...)`
ou `spawnSync('trackfw', ['guard', 'credential'], ...)`) sem passar por nenhum shell.

**Nota sobre caminho absoluto vs. PATH:**
Usar caminho absoluto para `trackfw` (embutido pelo generator no `trackfw init`) é viável
**apenas para plugin de escopo global** — o arquivo vive em `~/.config/opencode/plugins/` e é
local a cada máquina. Para **plugin de escopo de projeto** (commitado no repositório), um caminho
absoluto de uma máquina quebra os colegas de equipe e o CI: o generator não pode embutir
`/Users/kgsaran/...` em um arquivo commitado. O plugin de projeto deve encontrar `trackfw` via PATH
e, com isso, seguir o mesmo padrão fail-closed para binário ausente — throw por ENOENT → toda
chamada bloqueada até o guard ser instalado. Essa tensão (proteção forte vs. usabilidade em CI)
é uma decisão arquitetural para a ADR (ML-5C).

**Falsificação:** ver Seção 3, Superfície A.

---

### T2 — `permission.ask` dispara apenas para permissões explícitas: cobertura incompleta (MÉDIO)

**Mecanismo:**
`permission.ask` permite `output.status = "deny"` mas só dispara quando uma permissão explícita
é solicitada pelo OpenCode. Se `opencode.json` tiver `"permission": { "bash": { "*": "allow" } }`
ou se o agente rodar em modo sem prompts, `permission.ask` **nunca dispara**.

A alternativa nativa (`"permission": { "bash": { ... } }` em `opencode.json`) usa padrões
estáticos — adequada para "nunca commitar na main" via glob, mas não para escaneamento dinâmico de
credenciais (Layer 1 do `guard credential`).

**Medir no ML-5B:** quais ferramentas (`bash`/`shell`, `edit`, `read`) disparam `permission.ask`
na configuração padrão?

---

### T3 — Plugin gerado: fail-open por verificação ingênua + mismatch V1/V2 (ALTO)

**Mecanismo:**

**Sub-risco 3a — Caso C com spawnSync (medido):**
O plugin gerado pelo trackfw chamará `Bun.spawnSync` ou `spawnSync` do `node:child_process`. Se
verificar apenas `result.status === 2`, será fail-open quando `trackfw` não estiver no PATH —
`result.status` é `null` (não 2) nesses casos (medido nesta sessão; ver T1 Caso C).

**Sub-risco 3b — Tool ID do shell difere entre V1 e V2 (fonte: prometeu §2.2):**
V1: ID do shell = `"bash"`. V2: ID do shell = `"shell"` (renomeado em 2.0).
Um guard que filtra apenas `event.tool === "shell"` não interceptará nada em V1.
O plugin gerado deve verificar os dois: `tool === "bash" || tool === "shell"`.

**Sub-risco 3c — API incompatível entre V1 e V2 (fonte: prometeu §2.1 e §2.2):**
V1: `export const GuardPlugin: Plugin = async ({ directory }) => ({ "tool.execute.before": ... })`.
V2: `export default Plugin.define({ id: "trackfw.guard", async setup(ctx) { ctx.tool.hook(...) } })`.
São **incompatíveis**. O plugin gerado deve ser um arquivo dual-export (técnica em `migrate-v1.md`)
ou o generator deve detectar a versão instalada e emitir o formato correto.

**Sub-risco 3d — write-scope tool name case mismatch (medido):**
O plugin deve mapear `"write"` → `"Write"` e `"edit"` → `"Edit"` antes de chamar
`trackfw guard write-scope`. Sem isso, write-scope retorna allow no Step 4 sem verificar frozen
paths (confirmado em 1.12).

**Sub-risco 3e — discriminador de subagente ausente em V1 e ambíguo em V2 (fonte: prometeu §2.2 + 1.12):**
Em V1, `input` do hook não expõe `agent` (`{ tool, sessionID, callID }`). O plugin V1 não pode
incluir `agent_id` no payload de write-scope, e portanto `writeScopeSubagentFrozenRelPaths` não
se aplica — o guard não bloqueia nem `.agents/hooks.json` nem `.opencode/plugins/` para sessões V1.
Em V2, `event.agent` está presente em toda chamada (non-optional); encaminhar incondicionalmente
bloquearia a sessão principal (ver T7, R7). A discriminação correta é a medir (ML-5B braço A2).

**Sub-risco 3f — write-scope path field mapping: `path`/`filePath` ≠ `file_path` → false positive (medido):**
`writescope.go` usa `tool_input.file_path` como único campo de caminho para tools Write/Edit.
V1 envia `filePath` (fonte: prometeu §2.3); V2 envia `path`. Medido nesta sessão com bin v11.7.0:
`{"tool_name":"Write","agent_id":"hades-tf","tool_input":{"filePath":"<frozen>"}}` → rc=2 com
mensagem "tool_input.file_path absent" (bloqueia TODA write, não apenas frozen paths).
Idem para `path`. O plugin deve mapear: `V1 filePath → file_path`, `V2 path → file_path` antes
de chamar o write-scope guard.

**Sub-risco 3g — credential guard não chamado para write/edit (lacuna de paridade):**
Claude Code registra o credential guard em três matchers: `Bash|PowerShell`, `Read`, `Write|Edit`
(`agentfiles.go:414–417`). O plugin OpenCode que chamar o credential guard apenas para tool
`shell`/`bash` perderá credenciais em `content` (Write) e `newString` (Edit). Confirmado pela
medição de `validate --json` (nesta sessão): 3 `credential_guard_presence_warning` separadas para
os matchers `Bash|PowerShell`, `Read`, e `Write|Edit`. O plugin gerado deve chamar o credential
guard para `shell`/`bash`, `read`, e `write`/`edit`. **Inferido (não medido):** V2 tem um tool
`patch` que escreve arquivos (prometeu §2.3); não há entrada PascalCase `Patch` em
`writeScopeWriteTools` — `patch` também não é coberto pelo write-scope; incluir no credential
guard por paridade (a confirmar no ML-5B braço A2 contra-braço de `write`).

---

### T4 — Payload schema: `event.input.command` ≠ `tool_input.command` → silent allow (ALTO)

**Mecanismo (medido):**
`ExtractCommand` em `internal/guard/payload.go:165` tem 5 prioridades; a 1ª é `tool_input.command`.
O campo que OpenCode passa é `event.input.command` (V2) ou `output.args.command` (V1) — **não**
`tool_input.command`. Um plugin que envia o payload sem mapear para `tool_input.command` produz
`shellCmd=""` → silent allow.

Medido nesta sessão com binário trackfw 11.5.0:
```bash
# tool_input.command (priority 1) → BLOQUEADO
printf '{"tool":"bash","tool_input":{"command":"git commit -m msg"}}' | trackfw guard git-branch → rc=2

# args.command (fora das 5 prioridades) → SILENT ALLOW
printf '{"tool":"bash","args":{"command":"git commit -m msg"}}' | trackfw guard git-branch → rc=0
```

**Mapeamento correto no plugin:**
```ts
// V2: event.input.command → tool_input.command
const payload = JSON.stringify({ tool_input: { command: (event.input as { command: string }).command } })
// V1: output.args.command → tool_input.command
const payload = JSON.stringify({ tool_input: { command: (output.args as { command: string }).command } })
```

---

### T5 — Payload: entrega por stdin vs argv; encoding; tamanho (MÉDIO)

**Mecanismo:**

**Sub-risco 5a — Delivery channel:**
O trackfw guard recebe o payload via stdin (padrão do PSPOSIX). Se o plugin usar argv
(`execFile("trackfw", ["guard", "git-branch", payload])`):
- Vaza o payload para `ps aux` (todos os processos no sistema veem os args)
- ARG_MAX (~2 MB no Linux) pode truncar payloads grandes

**Stdin é o canal correto** (confirmado pelo design do guard e por prometeu §4.2).

**Sub-risco 5b — Encoding:**
`DrainStdin` recebe bytes brutos. `payload.go:151` strips UTF-8 BOM antes do parse. `JSON.stringify`
em Bun produz UTF-8 sem BOM → correto. NUL bytes em `event.input.command` → `ErrNULInCommand`
(fail-closed, ADR D2-bis) — comportamento correto.

**Sub-risco 5c — Tamanho e escopo do payload por guard:**
`DrainStdin` usa janela de idle de 2s sem limite de tamanho. O plugin deve enviar apenas os campos
necessários para cada guard — mas o escopo correto difere por guard e por tool:
- **Guard git-branch** (apenas shell calls): `tool_input.command` suficiente.
- **Guard credential** (shell calls): `tool_input.command` suficiente.
- **Guard credential** (read calls): `tool_input.file_path` suficiente para Layer 1 (varre o
  caminho como string no payload). **Layer 2 é no-op para Read**: `credSecondLayer`
  (`credential.go:1020`) retorna `credFinding{}` vazio quando `shellCmd == ""` — não há
  varredura de arquivo referenciado. Incluir o campo de caminho ainda é correto para paridade de
  payload, mas a proteção real vem de Layer 1 (marginal) e da wiring de Write/Edit (abaixo).
- **Guard credential** (write/edit calls): payload completo incluindo `content`/`newString` —
  Layer 1 varre o conteúdo sendo escrito para detectar credenciais. Enviar apenas `command` cria
  **lacuna de paridade** com Claude Code, que registra o guard no matcher `Write|Edit`
  (`agentfiles.go:417`) exatamente para escanear `content`/`newString`.

Medido nesta sessão com `trackfw validate --json` (bin v11.7.0 em `1f8e82c0`, HOME isolado em
`mktemp -d`) em projeto minimal com `.claude/settings.json` vazio (rc=0, **4 warnings**: 3
`credential_guard_presence_warning` para os matchers `Bash|PowerShell`, `Read`, `Write|Edit` + 1
`write_scope_not_wired`): o validator confirma que os três matchers são requeridos para paridade com
Claude Code. Sem OpenCode guard: **0 warnings específicos a OpenCode** (ver braço E1-FN: projeto
com `.opencode/opencode.json` vazio → 1 `write_scope_not_wired`, 0 `credential_guard_presence_warning`).

---

### T6 — `--pure` / `OPENCODE_PURE=1` desabilita o plugin de guard globalmente (ALTO)

**Mecanismo (fonte: `opencode --help`, opencode 1.19.5):**
```
--pure    run without external plugins    [boolean]
```

Qualquer invocação com `--pure` ou `OPENCODE_PURE=1` desabilita **todos** os plugins externos,
incluindo o plugin de guard do trackfw. `validate` não detectaria a ausência; `update harness` não
reportaria nada (1.2, 1.5).

**Diferença dos outros 9 CLIs:** hooks JSON e shell são processados pelo runtime do CLI, não
desabilitados por flag. Para o OpenCode, o próprio mecanismo de entrega do guard é opt-out.

**Falsificação:** ver Seção 3, Superfície C.

---

### T7 — Agente pode desabilitar o plugin de guard editando `opencode.json` ou removendo o plugin (ALTO)

**Mecanismo:**
A configuração `plugin: [...]` / `plugins: [...]` em `opencode.json` é a chave de registro do plugin.
Write-scope não congela `opencode.json` nem `.opencode/plugins/` (confirmado em 1.4).
Mesmo que esses caminhos sejam adicionados, **cinco pré-requisitos independentes** são necessários
para que o write-scope efetivamente bloqueie (medidos em 1.12):
1. Adicionar os caminhos a `writeScopeSubagentFrozenRelPaths`
2. Aceitar tool names lowercase em `writeScopeWriteTools` (ou o plugin mapear para PascalCase)
3. Plugin encaminha `agent_id` somente em chamadas de subagente (a medir no ML-5B braço A2). Em V1,
   `tool.execute.before` não expõe `agent` — write-scope de frozen paths inaplicável em V1.
   Medido via D1: sem `agent_id` + PascalCase + frozen path → rc=0 (sessão principal = allow)
4. O valor do caminho deve ser absoluto (write-scope rejeita relativos com rc=2)
5. Campo de caminho deve ser `file_path` — não `filePath`/`path` (rc=2 "file_path absent")

**Residual pré-existente (canal shell):** mesmo com os cinco pré-requisitos atendidos, **tool `shell`**
(e qualquer tool não em `writeScopeWriteTools`) nunca passa por write-scope (Step 4 retorna allow).
Um agente pode usar `shell` para `rm` ou `sed` em `opencode.json` ou `.opencode/plugins/guard.ts`
sem que write-scope intervenha. Este canal shell bypass é pré-existente para todos os CLIs: Claude
Code tem o mesmo gap para o tool `Bash`. Ver R7.

**Cadeia (estado futuro após ML-5C instalar o guard):**
1. Agente usa tool `edit` (lowercase) em `opencode.json` ou tool `shell` para `rm`/`sed` em
   `opencode.json` ou `.opencode/plugins/guard.ts` → write-scope allow (Step 4, lowercase bypassa;
   `shell` nunca passa por write-scope). Para plugin de auto-descoberta, basta apagar o arquivo do
   diretório `.opencode/plugins/`.
2. Plugin removido da lista `plugins: []` (ou arquivo do plugin apagado)
3. Guard inativo na próxima sessão; `validate` passa verde (nenhum gate existe hoje — ver T6, E1)
4. `git commit` com credencial passa sem bloqueio

**Falsificação:** ver Seção 3, Superfície D.

---

### T8 — TOCTOU: `input` em `execute.before` é mutável por plugins posteriores (BAIXO)

**Mecanismo (inferido da API):**
Em V2, `event.input` é passado por referência e mutável. Um plugin carregado **depois** do plugin
de guard pode re-escrever `event.input` após o guard ter inspecionado os args originais. A ordem
de execução de múltiplos plugins não está documentada (a medir no ML-5B).

---

### T9 — MCP, subagentes (`task`), e tools de terceiro podem não disparar `execute.before` (ALTO)

**Mecanismo (inferido):**
`execute.before` pode ser invocado apenas para ferramentas nativas do OpenCode (bash/shell, write,
edit, read). MCP tools, subagentes via `task`, e tools de outros plugins podem criar contextos
separados sem os hooks do plugin de guard.

**A medir no ML-5B:** um MCP tool `bash` chamado pelo agente passa por `execute.before` do plugin?

---

### T10 — Windows: runtime Bun; `CLIOpenCode` ausente; XDG path não `%APPDATA%` (MÉDIO)

**Mecanismo:**
O runtime de plugins é **Bun** em ambas as versões (prometeu §4.1). No Windows (Lourival, opencode
2.3.0, issue #691):
- `HomeFor("opencode")` em `agentconfig.go` não tem especialização Windows (1.10)
- O caminho global é `C:\Users\<user>\.config\opencode\` (XDG), **não** `%APPDATA%\opencode`
- Comportamento de `Bun.spawnSync` com binário ausente não medido (Bun não disponível em PATH local)

**Nota de implementação crítica (ML-5D):** `node:child_process.spawnSync` RETORNA `{error:...}`
em ENOENT — não lança (medido nesta sessão com Node). O padrão correto (prometeu §3.1) verifica
`result.error` e então lança explicitamente. Se `Bun.spawnSync` LANÇAR ao invés de retornar `error`
(comportamento da API nativa Bun — a medir no ML-5B braço T10), o Effect o propaga automaticamente
como `Tool.Error` → fail-closed. Em qualquer caso: o ML-5D NÃO deve envolver o spawn em um bloco
`try/catch` que capture a exceção e retorne normalmente — qualquer wrapper de catch deve re-lançar.

**A medir no ML-5B (Lourival):**
Probe `bun -e` com `node:child_process.spawnSync` (a API que prometeu §4.2 usa):
```bash
bun -e "const {spawnSync}=require('node:child_process'); const r=spawnSync('trackfw-nao-existe',['guard','git-branch'],{input:'{}',encoding:'utf8'}); console.log(JSON.stringify({status:r.status,error_code:r.error?r.error.code:null,signal:r.signal}));"
```
Resultado esperado: `{"status":null,"error_code":"ENOENT","signal":null}` — igual ao medido em Mac
com Node (`node -e` nesta sessão). Contra-braço: mesma sonda com `Bun.spawnSync` (a API nativa do
Bun): `bun -e "const r=Bun.spawnSync(['trackfw-nao-existe'],{stdin:'pipe'}); console.log(r.exitCode, r.success)"`.
Se `bun` estiver ausente do PATH de Lourival, substituir pelo braço A1(V2) em OpenCode — se o
guard falha ao encontrar `trackfw`, o braço A1 cobre o comportamento do plugin no Caso C. Não
usar `node -e` como substituto: o plugin roda sob Bun, e a API nativa Bun pode divergir de Node.

---

### T11 — Supply chain: `@opencode-ai/plugin` como runtime dependency (MÉDIO)

**Mecanismo:**
Se o plugin gerado pelo trackfw tiver `import { ... } from "@opencode-ai/plugin"` em runtime (não
`import type`), precisará de `node_modules/` instalado. Se ausente → Caso B (fail-open).

**Mitigação (ML-5D):** o plugin deve ser um arquivo TS/JS sem `import` em runtime — tipos são
`import type` (compilados fora). Prometeu §4.2 mostra exemplos sem import de runtime.

---

### T12 — Credential guard Layer 2 não-funcional com schema desconhecido (MÉDIO)

**Mecanismo:**
`guard credential` Layer 1 (`credScanInput`) varre o payload raw como string — cobre campos
desconhecidos. Layer 2 analisa `shellCmd`. Se `shellCmd=""` (campo de comando não mapeado para
`tool_input.command`), Layer 2 é no-op. Vinculado a T4.

---

### T13 — write-scope: tool names do OpenCode em lowercase bypassam verificação de frozen paths (ALTO)

**Mecanismo (medido em 1.12 com branch binary v11.7.0):**
`writeScopeWriteTools` contém apenas PascalCase (`"Write"`, `"Edit"`, `"NotebookEdit"`). OpenCode
envia `"write"`, `"edit"`, `"shell"` (lowercase). Step 4 do write-scope retorna `allow` sem
atingir a verificação de frozen paths.

**Impacto:** adicionar caminhos OpenCode a `writeScopeSubagentFrozenRelPaths` **não é suficiente**
sem também resolver os outros quatro pré-requisitos (T7, totalização: 5): case mismatch, discriminador de
subagente, caminho absoluto, e campo `file_path`. Em V1, `agent_id` não está disponível no hook
→ frozen paths inaplicáveis em V1 independente dos outros pré-requisitos.

---

## 3. Alvos de Falsificação — Dois Sentidos por Superfície

**Nota sobre braços de medição:** os braços com modelo requerem `opencode run` (sessão com
chamada de modelo). Braços A1, A2, C1, F1, G1, H1 requerem sessão OpenCode com modelo.

---

### Superfície A — Mecanismo de bloqueio do `execute.before` (T1)

**Falso negativo (guard escreve throw, OpenCode ignora):** ferramenta com credencial executa sem
bloqueio.
Gate: **braço de bloqueio do ML-5B no binário real** — git commit com credencial fake deve ser
bloqueado. Testes unitários não substituem (não invocam o OpenCode real). A fonte primária
(prometeu §3) confirma fail-closed por construção; este braço valida no binário.

**Falso positivo (throw bloqueia todas as ferramentas):** qualquer ferramenta, incluindo `echo`,
é abortada pelo plugin.
Gate: **braço benígno do ML-5B** — `echo hello` deve passar sem bloqueio.

**Falso positivo (fail-closed bloqueia quando PATH difere do shell):** em IDE launch ou Windows
(T10), o PATH do processo OpenCode pode não incluir `trackfw`. Com o padrão correto
(`result.error || result.status !== 0`), uma ENOENT lança exceção → todo `shell` call bloqueado.
Gate:
- **Plugin de escopo global** (`~/.config/opencode/plugins/`): o generator pode emitir caminho
  absoluto (`which trackfw` detectado em tempo de `update harness`) — o arquivo é local à máquina
  de cada usuário, não commitado. Risco de staleness: upgrade move o binário; caminho desatualizado
  → fail-closed bloqueia todo `shell` call até re-executar `update harness`. O produto atual usa
  nome por PATH (não absoluto) em `CredentialCurrentPSPOSIX`. Uma ADR deve declarar se o plugin
  global emite path absoluto ou depende de PATH.
- **Plugin de escopo de projeto** (commitado no repositório): um caminho absoluto de uma máquina
  quebra todos os colegas de equipe e o CI. Deve usar `trackfw` via PATH, com o mesmo padrão
  fail-closed. Consequência aceita: se `trackfw` não estiver no PATH da máquina de um colega,
  todo `shell` call é bloqueado até ele instalar o guard. Este é o falso positivo aceito para
  o escopo de projeto — a ADR (ML-5C) decide qual escopo é o padrão entregue.

**Braço A1 (com modelo, N=1 Mac):** instalar um plugin com `throw` em `execute.before`, rodar
`opencode run "execute echo hello"`. Resultado esperado: fonte teórica confirma fail-closed;
o braço valida no binário real.

**Braço A2 (com modelo, N=1 Mac) — sonda de payload + discriminador subagente:** plugin que grava
`JSON.stringify({tool, input, agent, sessionID})` em arquivo temporário sem throw, executar uma
chamada normal e uma chamada via `task` (subagente). Resultado esperado: arquivo com
`{tool:"shell"/"bash", input:{command:"echo hello"}}`. O contraste entre `agent` na sessão
principal e no subagente determina o discriminador para T7 pré-requisito 3. Contra-braço adicional
para `write`: campo `path` identificado (informa pré-req 5; e se o valor de `path` é absoluto ou
relativo, informa pré-req 4 — se relativo, o plugin deve resolver com `ctx.location.directory`).
Contra-braço adicional para `patch` (V2): gravar o input shape de um tool `patch` para identificar
o campo de caminho e confirmar se o tool é `patch` ou `apply_patch` em V2 (inferido via prometeu
§2.3; medir no binário real do OpenCode 2.3.0 no Windows, braço A2(V2)). Contra-braço: sonda para `write` → campo `path` identificado.

---

### Superfície B — Payload mapping: `tool_input.command` ≠ payload real (T4)

**Falso negativo (campo errado → silent allow):**
`git commit` com credencial fake passa porque `input.command` não está em `tool_input.command`.
Gate: braço A2 confirma o campo; ajuste no plugin; re-testar braço A1.

**Falso positivo:** mapeamento extrai campo errado como `shellCmd`, bloqueia comando benígno.
Gate: braço benígno A1.

**Braço B1 (model-free, medido nesta sessão com binário 11.5.0):**
```bash
printf '{"tool":"bash","tool_input":{"command":"git commit -m msg"}}' | trackfw guard git-branch → rc=2 ✅
printf '{"tool":"bash","args":{"command":"git commit -m msg"}}' | trackfw guard git-branch → rc=0 ✅
```

---

### Superfície C — Bypass por `--pure` / `OPENCODE_PURE=1` (T6)

**Falso negativo (guard desabilitado por flag):** sessão com `--pure` executa qualquer ferramenta
sem o plugin de guard.
Gate: o `validate` deveria emitir aviso; o `doctor` deveria auditar a presença do plugin.
**Nenhum desses gates existe hoje** (confirmado em 1.5, 1.11).

**Falso positivo:** `--pure` é uso legítimo do OpenCode; proibir via `validate` seria excesso.
O único gate efetivo é documentação clara (ADR ML-5C).

**Braço C1 (com modelo, N=1 Mac):** `opencode --pure run "..."` com plugin de sonda instalado.
Resultado esperado: a sonda não escreve nada (plugin não carregado).

---

### Superfície D — Write-scope: agente edita `opencode.json` / remove plugin (T7, T13)

**Falso negativo (plugin removido sem alerta):** agente edita `opencode.json` para remover o
plugin de guard. Gate: cinco pré-requisitos devem ser cumpridos juntos (T7). Nenhum existe hoje.

**Falso positivo (`agent_id` forwarded incondicionalmente → sessão principal tratada como subagente):**
Se o plugin V2 encaminhar `event.agent` como `agent_id` em toda chamada, write-scope trata a sessão
principal como subagente, bloqueando `trackfw.yaml` e `.claude/settings.json` para o usuário.
Gate: encaminhar `agent_id` somente em chamadas de subagente. O discriminador é a medir (braço A2).

**Falso positivo (campo de caminho errado → `file_path absent` → bloqueia toda Write):**
Se o plugin usar o nome de campo nativo do OpenCode (`filePath` em V1 ou `path` em V2) sem mapear
para `file_path`, write-scope retorna rc=2 "tool_input.file_path absent" para TODAS as Write/Edit
calls — independente do conteúdo ou destino. É fail-closed mas impede qualquer uso do OpenCode
com escrita ativa. Gate: D1 contra-braço com `filePath`/`path` confirma o falso positivo; o plugin
deve mapear para `file_path` antes de enviar ao guard (pré-req 5, R7).

**Falso positivo (`.opencode/` inteiro congelado):** agente não consegue criar agentes/skills.
Gate: a proteção deve ser apenas para `.opencode/plugins/` e os arquivos de config que registram
o plugin, não para `.opencode/agents/` ou `.opencode/skills/`.

**Braço D1 (model-free, medido nesta sessão com branch binary v11.7.0, caminhos reais abreviados):**
```bash
# controle positivo: PascalCase + agent_id + already-frozen path (.agents/hooks.json, Antigravity)
# → BLOQUEADO. Confirma pré-req 2 (PascalCase) + pré-req 3 (agent_id) + pré-req 4 (absolute) + pré-req 5 (file_path)
# quando o path JÁ está na lista frozen. Pré-req 1 (adicionar .opencode/plugins/ à lista) é mudança de código,
# não pode ser confirmado sem a mudança. Este controle usa .agents/hooks.json como proxy de um frozen path.
printf '{"tool_name":"Write","agent_id":"hades-tf","tool_input":{"file_path":"<tmpdir>/.agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=2 [controle positivo ✅ — pré-req 2+3+4+5 atendidos]

# pré-req 2 (lowercase bypassa Step 4): tool_name "write" → ALLOW antes de verificar frozen paths
printf '{"tool_name":"write","agent_id":"hades-tf","tool_input":{"file_path":"<tmpdir>/.agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=0 [pré-req 2 ✅ confirma gap]

# pré-req 1 ausente: "Edit" + frozen path inexistente na lista → ALLOW (pré-req 1 ausente, correto)
printf '{"tool_name":"Edit","agent_id":"hades-tf","tool_input":{"file_path":"<tmpdir>/opencode.json"}}' \
  | bin/trackfw guard write-scope → rc=0 [pré-req 1 ✅ correto — opencode.json não frozen]

# pré-req 5: campo errado (filePath) → rc=2 "tool_input.file_path absent" (bloqueia TODA write)
printf '{"tool_name":"Write","agent_id":"hades-tf","tool_input":{"filePath":"<tmpdir>/.agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=2 "file_path absent" [pré-req 5 ✅ confirma falso positivo]

# pré-req 5: campo errado (path V2) → idem
printf '{"tool_name":"Write","agent_id":"hades-tf","tool_input":{"path":"<tmpdir>/.agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=2 "file_path absent" [pré-req 5 ✅ confirma V2]

# pré-req 4: caminho relativo → rc=2 "not an absolute path"
printf '{"tool_name":"Write","agent_id":"hades-tf","tool_input":{"file_path":".agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=2 "not absolute" [pré-req 4 ✅]

# pré-req 3: sem agent_id + PascalCase + frozen path → ALLOW (sessão principal não é subagente)
printf '{"tool_name":"Write","tool_input":{"file_path":"<tmpdir>/.agents/hooks.json"}}' \
  | bin/trackfw guard write-scope → rc=0 [pré-req 3 ✅ — sem agent_id = sessão principal = allow]
```
D1 cobre: pré-req 2 (lowercase bypassa), pré-req 3 (sem agent_id = sessão principal = allow —
evidência de que frozen paths só se aplicam em subagentes), pré-req 4 (relativo rejeita), pré-req
5 (filePath/path rejeita). Citado em T7 item 3 e R7(c).
O pré-req 1 (código → adicionar `.opencode/plugins/` à lista frozen) não é mensurável sem mudança
de código; o controle positivo confirma que a lista funciona quando o caminho está lá (`.agents/hooks.json`).
O discriminador V2 (pré-req 3: OpenCode distingue sessão principal vs subagente via `event.agent`) é
o que A2 mede — se `.agents/hooks.json` foi congelado por pré-req 1, e V2 `event.agent` está presente
em toda chamada (non-optional), o plugin deve enviar `agent_id` somente para subagentes.

---

### Superfície E — `validate` e `doctor` não auditam o OpenCode (T6, T9)

**Falso negativo (guard removido, `validate` passa verde):**
Gate: adicionar verificação do plugin de guard ao validator e ao `doctor`. Nenhum existe (1.5, 1.11).

**Falso positivo (validator audita caminho errado):** `validate` reporta ausente quando o plugin
está instalado globalmente mas o validator só olha o projeto.
Gate: o validator deve cobrir os dois escopos — depende da decisão da ADR (ML-5C). Template para
a regra de `doctor`: `D8` (`antigravity_ide_global_guard`) em `internal/commands/doctor.go:150`.

**Braço E1 (model-free, medido nesta sessão com bin v11.7.0 em `1f8e82c0`, HOME isolado em `mktemp -d`):**
Dois braços paralelos medidos via `trackfw validate --json` em projetos tmpdir:
- **Braço E1-FN (OpenCode sem guard):** projeto com `trackfw.yaml` + `.opencode/opencode.json` vazio
  (simula OpenCode em uso), HOME isolado → rc=0, **1 warning** total: `write_scope_not_wired`
  (genérico, não específico a OpenCode), 0 `credential_guard_presence_warning`. Gate inexistente
  confirmado ✅.
- **Braço E1-contra (Claude sem guard):** projeto com `trackfw.yaml` + `.claude/settings.json` vazio,
  HOME isolado → rc=0, **4 warnings** total: 3 `credential_guard_presence_warning` para
  `Bash|PowerShell`, `Read`, `Write|Edit` + 1 `write_scope_not_wired`. Confirma que o gate existe
  para Claude e é cego para OpenCode ✅.
Comparação justa: ambos os braços têm o arquivo de config da IDE presente. O resultado assimétrico
(1 vs 4 warnings; 0 vs 3 `credential_guard_presence_warning`) é a evidência do gap.

---

### Superfície F — Supply chain: plugin sem `node_modules` (T11)

**Falso negativo (plugin falha ao carregar, guard inativo):** plugin com `import` de runtime e sem
`node_modules/` → Caso B, fail-open (a medir).
Gate: o plugin gerado não deve ter `import` em runtime (só `import type`).

**Braço F1 (com modelo; requer opencode para observar se o hook disparou no carregamento falho):** escrever plugin
com `import { something } from "@opencode-ai/plugin"` sem `node_modules/`. Resultado esperado:
OpenCode reporta erro de carregamento (ou ignora silenciosamente). Contra-braço: com `import type`
→ carrega sem `node_modules/`.

---

### Tabela consolidada de braços de medição para o ML-5B

| Braço | Superfície | Com modelo? | Resultado esperado | Contra-braço |
|---|---|:---:|---|---|
| A1 — throw bloqueia `execute.before`? | A (T1) | Sim | fail-closed confirmado pelo binário | echo benígno → passa; PATH-only trackfw → PATH gap |
| A2 — sonda de payload + discriminador subagente | B (T4), D (T7) | Sim | `{tool:"shell"/"bash", input:{command:...}, agent:..., sessionID:...}` para sessão principal e via `task` | sonda para `write` → campo `path` (pré-req 5); para `patch` → field + absolute? (inferido); `agent` em sessão principal vs task (pré-req 3-V2) |
| B1 — `tool_input.command` bloqueia | B (T4) | Não (medido ✅) | rc=2; `args.command` → rc=0 | `args.command` → rc=0 (silent allow) |
| C1 — `--pure` desabilita sonda | C (T6) | Sim | sonda não executa | sem `--pure` → sonda executa |
| D1 — write-scope 5 pré-req | D (T7, T13) | Não (medido ✅) | `write` rc=0 (pré-req 2); `filePath`/`path`→rc=2 (pré-req 5); relativo→rc=2 (pré-req 4); sem agent_id→rc=0 (pré-req 3) | `Write`+PascalCase+frozen+agent_id→rc=2; A2 cobre pré-req 1+3-V2 |
| E1 — `validate` passa sem plugin | E (T6) | Não (medido ✅) | rc=0, 0 warnings OpenCode | contra-braço: Claude arm (3 `credential_guard_presence_warning`) ✅ |
| F1 — plugin sem `node_modules` → Caso B | F (T11) | Sim | Caso B (fail-open) a medir | `import type` → sem `node_modules` |
| G1 — `permission.ask` dispara para `shell`? | T2 | Sim | depende da config | config `{bash:"ask"}` → sempre dispara |
| H1 — MCP tool bypassa `execute.before`? | T9 | Sim | sonda não executada para MCP | ferramenta nativa → sonda executada |

**Braços com modelo por plataforma (lista fechada):**

Mac (opencode 1.19.5, V1 API): A1, A2, C1, F1, G1, H1 → **N=6 Mac**.
Windows (Lourival, opencode 2.3.0, V2 API): A1(V2), A2(V2), C1(V2), F1(V2), G1(V2), H1(V2) → **N=6 Windows**.
(G1 e H1 são braços incondicionais em Windows porque a API V2 é um code path diferente.)
T10 (Bun.spawnSync/node:child_process.spawnSync com binário ausente, Windows) — model-free: usar
as probes documentadas em T10 (`bun -e "..."` com `node:child_process.spawnSync` e contra-braço
com `Bun.spawnSync`). NÃO usar `node -e` como fallback direto — ver T10 para a justificativa.

Braços B1, D1, E1 são model-free e foram executados nesta sessão.
Total de braços com modelo: **12** (6 Mac + 6 Windows).

---

## 4. Residual Declarado

**R1 — Payload exato (campo e cwd) não medido no binário real.**
A fonte primária (prometeu §2.3) documenta os shapes de V1 e V2 e confirma `event.input.command`
(V2) e `output.args.command` (V1) como os campos do comando shell. O mapeamento para
`tool_input.command` (ExtractCommand prioridade 1) está especificado. O braço A2 do ML-5B valida
isso no binário real. Comportamento de `Bun.spawnSync` com binário ausente não medido (Bun não em
PATH local) — a confirmar no ML-5B braço T10.

**R2 — `--pure` / `OPENCODE_PURE=1` como bypass de design.**
Qualquer invocação com `--pure` desabilita o plugin de guard sem detecção por `validate` ou
`doctor`. Este bypass não existe nos outros 9 CLIs. Aceito como residual de design: documentar
claramente na ADR (ML-5C). Não é possível fechar sem mudança no OpenCode.

**R3 — Caminhos globais não são cobertos por write-scope.**
`~/.config/opencode/opencode.json` e `~/.config/opencode/plugins/` estão fora da raiz do projeto.
O write-scope não os cobre por limitação arquitetural declarada (mesma limitação pré-existente para
`~/.claude/settings.json` etc.). Aceito como residual pré-existente.

**R4 — `trackfw.ts` user-authored: shell injection e fail-open não são produto trackfw.**
O arquivo `~/.config/opencode/tools/trackfw.ts` não é entregue pelo trackfw. Expõe o CLI trackfw
como custom tool do OpenCode com shell injection risk (`sh -c "trackfw ${cmd}"`) e fail-open quando
`trackfw` está ausente do PATH (captura toda exceção e retorna string). Se o KG decidir que o
produto deve gerar esse arquivo com proteção adequada (quote dos args, fail-closed no erro), isso
é escopo do ML-5D.

**R5 — Credential guard project scope verifica apenas o cwd (ADR D9, R-cred-2).**
Herdado do mesmo residual declarado para o Antigravity. O guard invocado de um subdiretório não
encontra `trackfw.yaml` (sem walk-up) → no-op silencioso. O cwd efetivo do plugin no OpenCode é
`ctx.location.directory` (V2) ou closure `directory` (V1) — a confirmar no braço A2.

**R6 — Versão do OpenCode: 1.19.5 (Mac, V1 API) vs. 2.3.0 (Windows, V2 API).**
O plugin deve suportar V1 e V2 simultaneamente (dual-export) ou o generator deve detectar a versão.
O ML-5A (prometeu) documentou a incompatibilidade; o ML-5C (ADR) decide a estratégia. Aceito como
residual até que ML-5C seja concluído.

**R7 — write-scope: cinco pré-requisitos independentes; mais residual de canal shell.**
(a) Adicionar caminhos OpenCode a `writeScopeSubagentFrozenRelPaths`. (b) Aceitar tool names
lowercase em `writeScopeWriteTools` ou o plugin mapear para PascalCase. (c) Plugin encaminha
`agent_id` somente em chamadas de subagente — discriminador a medir (ML-5B braço A2); em V1 o
hook não expõe `agent`, tornando write-scope de frozen paths inaplicável em V1. Medido via D1:
sem `agent_id` + frozen path → rc=0 (sessão principal sempre allow). (d) Valor do
caminho deve ser absoluto — write-scope rejeita relativos com rc=2 "not an absolute path". (e)
Campo de caminho deve ser `file_path` — não `filePath`/`path`; chave errada → rc=2 "file_path
absent", bloqueando TODA Write call (falso positivo, medido nesta sessão com binário v11.7.0).
Os cinco pré-requisitos devem ser tratados na mesma ADR.

**Residual de canal shell + `patch` (pré-existente para todos os CLIs):** mesmo com (a)–(e) atendidos,
o tool `shell` (e qualquer tool não em `writeScopeWriteTools`) nunca passa por write-scope (Step 4
retorna allow). Um agente pode usar `shell` para `rm` ou `sed` em `opencode.json` sem que
write-scope intervenha. O mesmo vale para o V2 tool `patch` — pode escrever em frozen paths mas não tem entrada em
`writeScopeWriteTools`, portanto Step 4 retorna allow para `patch` mesmo com todos os outros
pré-requisitos atendidos. Este é o mesmo gap do tool `Bash` para Claude Code.

**R8 — Caso B: plugin-load failure é fail-open sem detecção (pré-existente ao guard).**
Se o plugin gerado tiver erro de sintaxe, import não resolvido (T11), ou for gerado em formato V2
numa instalação V1 (T3c), o hook `execute.before` nunca é registrado. O OpenCode prossegue
silenciosamente. Nenhum recurso do trackfw pode fazer o loader do OpenCode falhar-fechado — o guard
só pode detectar se o arquivo do plugin está presente (via `validate`/`doctor`), não se ele carregou
com sucesso. O ML-5D deve incluir um `validate` ou `doctor` que verifique presença do arquivo, e a
ADR (ML-5C) deve declarar esta limitação. O braço F1 do ML-5B confirma que o mecanismo do Caso B ocorre
(dois sub-braços: plugin com `import` de runtime sem `node_modules`; plugin V2-only em Mac 1.19.5).
A1 e A1(V2) devem usar o mesmo arquivo dual-export (prometeu §8) para validar que o dual-export
carrega em ambas as versões. Aceito como residual de design do mecanismo de plugin do OpenCode.

**R9 — `execute.after` não pode bloquear (PostToolUse é warn-only por construção).**
Prometeu §4 e §7 M20 documentam `execute.after` em V1 e V2 com canal de retorno `never`
— não pode bloquear. A proteção real é 100% pré-execução via `execute.before`. O equivalente
Claude Code PostToolUse (credential guard registrado em `agentfiles.go:427–429`) tem a mesma
assimetria: o PostToolUse roda após a execução, não pode desfazê-la. Para o plugin OpenCode,
`execute.after` é candidato a auditoria/logging pós-execução (ML-5D belt-and-suspenders), não
mecanismo de bloqueio. Aceito como residual de design do mecanismo de plugin do OpenCode.
