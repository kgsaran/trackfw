# OpenCode — Contrato do plugin (fonte primária)

> Data: 2026-11-19
> REQ: REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md
> Autor: prometeu-tf — ML-5A (Wave 5, investigação documental; nenhum código alterado)
> Branch: feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli

---

## URLs efetivamente buscadas (com status)

| URL | Status | Observação |
|---|---|---|
| `https://opencode.ai/llms.txt` | **200** | Índice de produtos e docs; confirma V2 atual |
| `https://opencode.ai/docs/index.md` | **200** | Intro + canais de instalação |
| `https://opencode.ai/docs/llms.txt` | **200** | Índice completo de páginas de docs V2 |
| `https://opencode.ai/docs/plugins.md` | **200** | Configure plugins (global/project) |
| `https://opencode.ai/docs/build/plugins.md` | **200** | Visão geral da API de plugin |
| `https://opencode.ai/docs/build/plugins/server.md` | **200** | Server plugin — API completa incluindo hooks |
| `https://opencode.ai/docs/build/plugins/migrate-v1.md` | **200** | Migração V1→V2; inclui tabela de hooks |
| `https://opencode.ai/docs/config.md` | **200** | Localizações de config por plataforma |
| `https://opencode.ai/changelog.md` | **200** | Changelog; confirma V2.3.x e V1.19.x |
| `https://opencode.ai/docs/tools.md` | **200** | Nomes e schemas dos tools do modelo |
| `npm view @opencode/cli` | **200** | registry npm; homepage/repo confirmados |
| `npm view opencode-ai` | **200** | registry npm; confirma V1 sem homepage |
| GitHub `anomalyco/opencode` API | **200** | homepage, default_branch=dev |
| `brew info opencode` | — | instalado localmente; aponta para anomalyco/opencode |
| `opencode --version` | — | binário local: 1.19.5 |
| Fonte primária dos tipos (GitHub raw, V2 tag `v2.3.0` sha `a6b15d33daf6`): | | |
| `packages/plugin/src/effect/tool.ts` | **200** | ToolHooks, ToolFailures — V2 Effect |
| `packages/plugin/src/promise/tool.ts` | **200** | ToolHooks — V2 Promise |
| `packages/plugin/src/promise/adapter.ts` | **200** | toolError; adaptação execute.before |
| `packages/plugin/src/runtime.ts` | **200** | `typeof Bun === "undefined"` → confirma runtime Bun |
| `packages/plugin/src/source.bun.ts` | **200** | `Bun.Transpiler`, `Bun.plugin` — runtime Bun |
| `packages/plugin/src/source.node.ts` | **200** | Node.js fallback loader |
| `packages/core/src/plugin/hooks.ts` | **200** | Domains, ToolFailures, trigger/register |
| `packages/core/src/config/discovery.ts` | **200** | systemDirectory; paths por plataforma |
| `packages/util/src/global-roots.ts` | **200** | XDG config path (global dir) |
| `packages/util/src/global.ts` | **200** | `OPENCODE_CONFIG_DIR` override |
| `packages/core/src/tool/plugin/shell.ts` | **200** | Schema do tool shell V2: command, workdir |
| `packages/core/src/tool/plugin/write.ts` | **200** | Schema do tool write V2: path, content |
| `packages/core/src/tool/plugin/edit.ts` | **200** | Schema do tool edit V2: path, oldString, newString |
| `packages/core/src/tool/plugin/patch.ts` | **200** | V2 patch tool: ID `"patch"` confirmado (`export const name = "patch"`) |
| GitHub `anomalyco/opencode/tags` | **200** | v1.19.5 e v2.3.x confirmados no mesmo repo |
| Fonte primária dos tipos (GitHub raw, V1 tag `v1.19.5`): | | |
| `packages/plugin/src/index.ts` | **200** | Hooks V1: `tool.execute.before`, `tool.execute.after`, `PluginInput.worktree` + `.directory` |
| `packages/opencode/src/config/plugin.ts` | **200** | V1 plugin discovery glob real (não `external.ts`) |
| `packages/core/src/config/plugin/external.ts` | **200** | V2 loader no monorepo v1.19.5 (NÃO fiado pelo binário V1) |
| `packages/opencode/src/session/tools.ts` | **200** | trigger site em V1: `plugin.trigger("tool.execute.before", ...)` |
| `packages/opencode/src/plugin/index.ts` | **200** | Implementação de `trigger` em V1 (Effect.promise + mutação de output) |
| `packages/opencode/src/tool/shell/prompt.ts` | **200** | Schema de args do shell V1: `{command, workdir?, timeout?}` |
| `packages/opencode/src/tool/shell/id.ts` | **200** | `ToolID = "bash"` — ID do tool shell em V1 |
| `packages/opencode/src/tool/write.ts` | **200** | V1 write: ID `"write"`, campo `filePath` (não `path` como V2) |
| `packages/opencode/src/tool/read.ts` | **200** | V1 read: ID `"read"`, campo `filePath` (não `path` como V2) |
| `packages/opencode/src/tool/edit.ts` | **200** | V1 edit: ID `"edit"`, campo `filePath` (não `path` como V2) |
| `packages/opencode/src/tool/apply_patch.ts` | **200** | V1 apply_patch: ID `"apply_patch"` (diferente de V2 `"patch"`) |
| `packages/opencode/src/config/v2-compat.ts` | **200** | Downgrader V2→V1: `plugins` key → "unsupported" |
| `packages/opencode/src/tool/code-mode.ts` | **200** | Chama `plugin.trigger("tool.execute.before")` — guard cobre code-mode paths |
| Fonte primária dos schemas (GitHub raw, V2 tag `v2.3.0` — complementar): | | |
| `packages/schema/src/location.ts` | **200** | V2 `Location.Info.project.directory` = raiz do projeto (tipo `AbsolutePath`) |
| `packages/plugin/src/promise/plugin.ts` (Context) | **200** | V2 `Context` interface: `readonly location: Location.Info` — confirma `ctx.location` acessível no `setup` |

> **Nota sobre o repositório:** `github.com/sst/opencode` retorna HTTP 301 → `github.com/anomalyco/opencode`
> (migração de organização; mesmo repositório, ID 975734319). Todas as URLs de fonte primária neste documento
> usam `anomalyco/opencode`.

---

## 1. Locais do plugin e registro

### 1.1 V2 (versão atual — `@opencode/cli` 2.3.x)

**Auto-descoberta por diretório:**

| Nível | Caminho |
|---|---|
| Projeto | `.opencode/plugins/*.ts` · `.opencode/plugins/*.js` · `.opencode/plugins/<dir>/` |
| Global | `~/.config/opencode/plugins/*.ts` · `.../plugins/*.js` · `.../plugins/<dir>/` |

Citação literal da doc (`opencode.ai/docs/plugins.md`):
> "OpenCode also loads direct `.ts` and `.js` files and immediate plugin package directories from every discovered `.opencode/plugins/` directory."
> "Global plugins use the same discovery layout under the OpenCode config directory."

```text
~/.config/opencode/plugins/
    ├── concise.ts          ← arquivo .ts solo — auto-carregado
    ├── reviewer.js         ← arquivo .js solo — auto-carregado
    └── acme-package/       ← diretório com package.json — auto-carregado
```

**Via `plugins` em `opencode.json(c)` (complementar à auto-descoberta):**

```jsonc
// opencode.jsonc — projeto (.opencode/opencode.jsonc, opencode.jsonc, ou .opencode/)
// ou global (~/.config/opencode/opencode.jsonc)
{
  "$schema": "https://opencode.ai/config.json",
  "plugins": [
    "opencode-acme-plugin",        // pacote npm publicado
    "opencode-acme-plugin@1.2.0",  // versão fixa
    "./plugins/local",             // diretório local relativo ao opencode.jsonc
    "/absolute/path/plugin",       // caminho absoluto
    {
      "package": "./plugins/company",
      "options": { "strict": true }
    }
  ]
}
```

Citação (`opencode.ai/docs/plugins.md`):
> "Plugin arrays from applicable config files are applied from lowest to highest precedence instead of replacing one another."

Isso significa que plugins de **todos** os níveis são carregados (são acumulados, não substituídos). Config
settings seguem precedência de override; a lista de plugins é **concatenada**.

**Ordem de carregamento dos arquivos de config** (do mais baixo para o mais alto, citação de `config.md`):
```
~/.config/opencode/opencode.json      ← global
./opencode.json                        ← projeto (raiz)
./.opencode/opencode.json              ← .opencode override de projeto
```

A config em `.opencode/` overrides a config direta; a do projeto overrides a global.

### 1.2 V1 (versão local KG — `opencode` 1.19.5)

A descoberta usa glob `{plugin,plugins}/*.{ts,js}` em cada diretório de config ativo
(fonte: `packages/opencode/src/config/plugin.ts@v1.19.5` — o módulo de config V1 nativo):

```ts
// packages/opencode/src/config/plugin.ts@v1.19.5 (módulo de config V1)
for (const item of await Glob.scan("{plugin,plugins}/*.{ts,js}", {
  cwd: dir,          // caminho de cada config dir
  absolute: true,
  dot: true,
  symlink: true,
})) {
  plugins.push(pathToFileURL(item).href)
}
```

> **Nota sobre `external.ts`:** `packages/core/src/config/plugin/external.ts@v1.19.5` existe no monorepo
> mas pertence ao V2 core e NÃO é fiado pelo binário V1 1.19.5 — a descoberta V1 é feita por
> `packages/opencode/src/config/plugin.ts` acima, que usa `ConfigPluginV1.Spec` e alimenta
> `PluginLoader.loadExternal` (que chama apenas `readV1Plugin` → `server()`). Confirmar em ML-5B.

Ou seja, V1 carrega de `.opencode/plugin/*.ts` e `.opencode/plugins/*.ts` (ambos), e o mesmo
no diretório global `~/.config/opencode/`.

A chave de config em V1 é `plugin` (não `plugins`):
```jsonc
// V1 — opencode.json
{ "plugin": ["opencode-example-plugin", ["./plugin/local.ts", { "enabled": true }]] }
```

> **Nota:** V2 lê também `.opencode/plugin/` para compatibilidade retroativa com V1 (citação de
> `migrate-v1.md`: "V2 discovers local plugins from both `.opencode/plugin/` and `.opencode/plugins/`").

### 1.3 Windows — caminho global

A doc (`opencode.ai/docs/config.md`) não exibe o path global Windows explicitamente.
O código-fonte revela o mecanismo real (`packages/util/src/global-roots.ts@v2.3.0`):

```ts
const config = process.env.XDG_CONFIG_HOME || (home ? path.join(home, ".config") : undefined)
```

**Consequência no Windows:** se `XDG_CONFIG_HOME` não estiver definida (caso padrão do Windows),
o global é `C:\Users\<username>\.config\opencode\` — **NÃO** `%APPDATA%\opencode`.

| Nível | Windows |
|---|---|
| Global (config) | `C:\Users\<username>\.config\opencode\opencode.jsonc` |
| Global (plugins) | `C:\Users\<username>\.config\opencode\plugins\` |
| System | `%ProgramData%\opencode` (fonte: `config.md` tabela de plataformas) |

A variável de ambiente `OPENCODE_CONFIG_DIR` permite sobrescrever o global em qualquer plataforma
(fonte: `packages/util/src/global.ts@v2.3.0`):
```ts
Layer.effect(Service, Effect.suspend(() => acquire({ config: process.env.OPENCODE_CONFIG_DIR ?? Path.config })))
```

---

## 2. Assinatura do hook

### 2.1 V2 — `ctx.tool.hook("execute.before", ...)`

Fonte primária: `packages/plugin/src/promise/tool.ts@v2.3.0` e `packages/plugin/src/effect/tool.ts@v2.3.0`.

```ts
// packages/plugin/src/effect/tool.ts@v2.3.0 e packages/plugin/src/promise/tool.ts@v2.3.0
interface ToolHooks {
  readonly "execute.before": {
    tool: string              // nome do tool (mutable — pode ser alterado pelo hook)
    readonly sessionID: Session.ID
    readonly agent: Agent.ID
    readonly messageID: SessionMessage.ID
    readonly id: Tool.CallID
    input: unknown            // args do tool (mutable — pode ser alterado pelo hook)
  }
}
```

> **Nota:** o `dev` branch adiciona campos `info?` e `codemode` que NÃO estão em `v2.3.0`. O contrato
> aqui é o de `v2.3.0` (a versão instalável via npm). Os campos de `dev` podem estar disponíveis em
> versões futuras; a medir em ML-5B com o binário real.

Registro:
```ts
export default Plugin.define({
  id: "trackfw.guard",
  async setup(ctx) {
    await ctx.tool.hook("execute.before", (event) => {
      // event.tool  — nome: "shell", "write", "edit", "read", etc. (IDs V2)
      // event.input — shape depende do tool (ver §2.3)
    })
  },
})
```

**Não há `cwd` diretamente no evento.** Para o credential guard, o cwd precisa apontar para a
raiz do projeto (onde `trackfw.yaml` está):
- Use `ctx.location.project.directory` — a raiz do projeto (campo confirmado em `packages/schema/src/location.ts@v2.3.0`; valor runtime a confirmar no ML-5B, M19).
- NÃO use `ctx.location.directory` — é o workspace/cwd de lançamento, pode ser subdiretório.

O working directory de um comando shell está em `event.input.workdir` (ver §2.3) — para guards
de branch git, use `event.input.workdir` como cwd do spawnSync (FindProjectRoot faz walk-up).

### 2.2 V1 — `"tool.execute.before"`

Fonte primária: `packages/plugin/src/index.ts@v1.19.5` (tipo do hook) e
`packages/opencode/src/session/tools.ts@v1.19.5` (trigger site).

```ts
// Tipo declarado em packages/plugin/src/index.ts@v1.19.5
"tool.execute.before"?: (
  input: { tool: string; sessionID: string; callID: string },
  output: { args: any },      // mutable — shape depende do tool
) => Promise<void>
```

**Implementação do trigger** (`packages/opencode/src/plugin/index.ts@v1.19.5`):
```ts
yield* Effect.promise(async () => fn(input, output))
```
`output` é passado por referência; a callback pode **mutar** `output.args` para alterar os args do
tool. Se a callback lançar (ou rejeitar a promise), o `Effect.promise` falha e o tool call é bloqueado.

**Call site** (`packages/opencode/src/session/tools.ts@v1.19.5`):
```ts
yield* plugin.trigger(
  "tool.execute.before",
  { tool: item.id, sessionID: ctx.sessionID, callID: ctx.callID },
  { args },
)
```

O projeto está em `worktree` do `PluginInput` (passado na construção do plugin):
```ts
export const GuardPlugin: Plugin = async ({ directory, worktree }) => ({
  "tool.execute.before": async (input, output) => {
    // input.tool  — nome: "bash" para shell (V1); "write", "edit", etc. para outros
    // output.args — shape depende do tool (ver §2.3)
    // worktree    — raiz do projeto/git worktree (onde trackfw.yaml está) — usar para cwd do guard
    // directory   — workspace cwd (pode ser subdiretório; NÃO usar para cwd do credential guard)
  },
})
```

> **Atenção — ID do tool shell difere entre versões:**
> - V1: o shell tool tem ID `"bash"` (fonte: `packages/opencode/src/tool/shell/id.ts@v1.19.5`,
>   `export const ToolID = "bash"` — comentário: "Keep as 'bash' for compatibility... Rename with opencode 2.0")
> - V2: renomeado para `"shell"` na série 2.x
>
> Um guard dual-export deve checar `input.tool === "bash"` em V1 e `event.tool === "shell"` em V2.

### 2.3 Shapes de args dos tools relevantes

**V2** (fonte: `packages/core/src/tool/plugin/shell.ts@v2.3.0`, `write.ts@v2.3.0`, `edit.ts@v2.3.0`):

**`shell` (ID V2: `"shell"`):**
```ts
{
  command: string        // string do comando shell
  workdir?: string       // diretório de trabalho; default = cwd do servidor
  timeout?: number       // ms; default = 120 000
  background?: boolean   // default = false
}
```

**`write`:**
```ts
{
  path: string           // caminho do arquivo (relativo ou absoluto)
  content: string        // conteúdo a escrever
}
```

**`edit`:**
```ts
{
  path: string           // arquivo a editar
  oldString: string      // texto exato a localizar
  newString: string      // substituto
  replaceAll?: boolean   // default = false
}
```

**V1** (fonte: `packages/opencode/src/tool/shell/prompt.ts@v1.19.5` e `packages/opencode/src/tool/{read,write,edit}.ts@v1.19.5`):

**`bash` (ID V1: `"bash"`):**
```ts
{
  command: string        // string do comando shell
  workdir?: string       // diretório de trabalho
  timeout?: number       // ms
  // NÃO tem campo `background` em V1
}
```

**`write` (ID V1: `"write"`):** campo de caminho é `filePath` (não `path` como em V2)
```ts
{
  filePath: string       // caminho absoluto do arquivo — confirmado em write.ts@v1.19.5
  content: string        // conteúdo a escrever
}
```

**`read` (ID V1: `"read"`):** campo de caminho é `filePath` (não `path` como em V2)
```ts
{
  filePath: string       // caminho absoluto do arquivo ou diretório
  offset?: number        // linha inicial (1-indexed)
  limit?: number         // número máximo de linhas
}
```

**`edit` (ID V1: `"edit"`):** campo de caminho é `filePath` (não `path` como em V2)
```ts
{
  filePath: string       // caminho absoluto do arquivo — confirmado em edit.ts@v1.19.5
  oldString: string
  newString: string
  replaceAll?: boolean
}
```

> **Diferença crítica V1 × V2:**
> - Campo de caminho: V1 usa `filePath`; V2 usa `path`
> - IDs de file tools: mesmos nas duas versões (`"write"`, `"read"`, `"edit"`)
> - Shell: `"bash"` em V1; `"shell"` em V2
> - Patch/apply_patch: V1 usa `"apply_patch"`; V2 usa `"patch"` (IDs DIFERENTES)
>
> V1 `apply_patch` e V2 `patch` — IDs confirmados por fonte; shapes dos args não inspecionados (M17 aberto).

**Medição empírica — comportamento do guard por tipo de payload (`--global`):**

Padrão: AWS access key `AKIA[0-9A-Z]{16}`. Arquivo temp real com conteúdo `secretkey: AKIA…` para T-D5/T-RC/T-RV1/T-RV2.

| ID | Payload | Exit 11.5.0¹ | Exit 11.7.0² | O que o guard detecta |
|---|---|---|---|---|
| **T-S** | `{"command":"echo AKIA…"}` | **2** | **2** | Layer 1: AWS key em raw bytes |
| **T-WV1** | `{"filePath":"/…","content":"key = AKIA…"}` | **2** | **2** | Layer 1: AWS key em `content` |
| **T-WV2** | `{"path":"/…","content":"key = AKIA…"}` | **2** | **2** | Layer 1: AWS key em `content` |
| **T-D5** | Antigravity `view_file` + `AbsolutePath` → arquivo real com AWS key | **0** | **2** | D5: guard lê o arquivo (ver nota) |
| **T-RC** | `{"tool_name":"Read","tool_input":{"file_path":"…"}}` | **0** | **0** | sem extrator para `tool_input.file_path` |
| **T-RV1** | `{"filePath":"…"}` (path-only) | **0** | **0** | sem extrator para `filePath` |
| **T-RV2** | `{"path":"…"}` (path-only) | **0** | **0** | sem extrator para `path` |

¹ Homebrew `trackfw` em PATH no Mac KG (`/opt/homebrew/bin/trackfw`). Medido nesta sessão.
² `bin/trackfw` local (HEAD da branch `feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli`). Medido nesta sessão.

> **T-D5 diferença:** `credViewFileScan` foi introduzido no commit `cafa1952` (ML-3A, 2026-11-18),
> que está nesta branch mas **ainda não foi tagueado** (v11.7.0 é de 2026-11-11, antes do commit).
> T-D5=0 em 11.5.0 é comportamento esperado de versão sem D5 — não é regressão.

**Conclusão:**
1. `write` e `edit`: guard detecta credencial no **conteúdo escrito** — cobre V1 (`filePath`) e V2 (`path`) igualmente via Layer 1.
2. `read` path-only: guard é no-op em pré. O guard não extrai `filePath`/`path` para escanear o arquivo destino. D5 faz isso para Antigravity `view_file` — mas não há extrator equivalente para OpenCode `read`.
3. **Gap pré e redaction pós:** OpenCode `read` não tem cobertura pré (guard é no-op para payload path-only). Post: V2 `execute.after` não pode bloquear via throw (`never`), mas `result` é mutável — se mutar o resultado alterar o que o modelo vê, redaction é viável (M20; verificação runtime pendente). Os outros CLIs têm post-hook bloqueante; o gap real é a ausência de scan pré do arquivo lido. Um extrator análogo ao D5 para `read` OpenCode seria Go product code em `internal/guard`; é candidato a ML na REQ-2026-11-17 (mesma causa, Regra Dura). Decisão do arquiteto.

> **`patch` (V2) / `apply_patch` (V1):** se o payload incluir diff text como campo, Layer 1 provavelmente varre e detecta credencial no diff. Para payloads path-only, mesmo comportamento que `read` (exit 0). Shape dos args não inspecionado em nenhuma versão — confirmar em ML-5B (M17).

Nomes de outros tools do modelo V2 (fonte: `opencode.ai/docs/tools.md`):
`read`, `glob`, `grep`, `patch`, `webfetch`, `websearch`

---

## 3. Como bloquear — throw e modos de falha

### 3.1 V2 — três modos de falha (fail-closed vs fail-open)

Fonte: `packages/plugin/src/promise/adapter.ts@v2.3.0` (função `toolError` e `adaptToolHook`):

```ts
// O adapter trata qualquer erro lançado de execute.before como Tool.Error:
const toolError = (error: unknown) =>
  error instanceof Tool.Error
    ? error
    : new Tool.Error({ message: error instanceof Error ? error.message : String(error), error })
```

#### Caso A — throw do callback → bloqueia (fail-closed)

- O callback é envolvido em `Effect.tryPromise({ try: ..., catch: toolError })`.
- Lançar **qualquer** valor — `throw new Error("...")` ou `throw new Tool.Error(...)` — bloqueia o tool call.
- O modelo recebe o `message` do `Tool.Error`.
- `execute.after` **não** pode bloquear — seu canal de falha é `never`.

```ts
await ctx.tool.hook("execute.before", (event) => {
  if (event.tool === "shell") {
    const input = event.input as { command: string; workdir?: string }
    if (/git\s+(commit|push)/.test(input.command)) {
      throw new Error("trackfw: git commit/push bloqueado pelo guard de credencial")
    }
  }
})
```

#### Caso B — plugin falha ao carregar → hook nunca registrado (fail-open)

Se o plugin não carregar (npm install falha, entry não encontrado, erro de sintaxe), o hook
`execute.before` **nunca é registrado**. Nenhum bloqueio ocorre; o tool executa normalmente.
Este comportamento é **fail-open** — a segurança do guard depende do plugin estar carregado.

#### Caso C — binário guard ausente ou crash → spawnSync retorna null (fail-open por default)

```ts
const result = spawnSync("trackfw-guard", [...], { ... })
// result.error   — definido se o processo não pôde ser criado (binário ausente, ENOENT)
// result.status  — null se o processo foi encerrado por sinal (crash)
//               — número (0, 1, 2...) se o processo terminou normalmente
```

**Código ingênuo — fail-open quando binário está ausente:**
```ts
if (result.status === 2) throw new Error("bloqueado")  // ← ERRADO: status null se binário ausente
```

**Código correto — fail-closed para erro do guard:**
```ts
if (result.error || result.status !== 0) {
  throw new Error(
    result.error
      ? `trackfw não encontrado: ${result.error.message}`
      : result.stderr?.trim() || "trackfw: operação bloqueada pelo guard"
  )
}
```

Com este padrão, binário ausente → throw → tool bloqueado (fail-closed). Ajuste conforme a política
desejada (fail-open vs fail-closed quando o guard não está instalado).

### 3.2 V1

Fonte: `packages/opencode/src/plugin/index.ts@v1.19.5`:
```ts
yield* Effect.promise(async () => fn(input, output))
```

Lançar da promise faz o `Effect.promise` falhar como um **defect** (erro não tipado), não como um
typed `Tool.Error` (que é o caso V2 via `tryPromise`). O tool call é bloqueado em ambos — mas a
mensagem que o modelo vê e a sobrevivência da sessão podem diferir entre V1 e V2. Os três modos
estruturais de falha são os mesmos:
- Caso A: throw bloqueia.
- Caso B: plugin não carregado → hook ausente → fail-open.
- Caso C: binário ausente → `spawnSync` retorna `result.error` → fail-open se verificação for `=== 2`.

Citação de `migrate-v1.md` sobre bloqueio:
```ts
"tool.execute.before": async (input, output) => {
  if (input.tool === "read" && output.args.filePath.includes(".env")) {
    throw new Error("Do not read .env files")
  }
},
```

---

## 4. Runtime e chamada de binário externo

### 4.1 Runtime: Bun (confirmado por código-fonte)

Fonte: `packages/plugin/src/runtime.ts@v2.3.0`:
```ts
export function ensurePluginRuntime() {
  if (typeof Bun === "undefined") return {}   // ← plugins não carregam fora do Bun
  if (installed) return installed
  Bun.plugin({ ... })
  ...
}
```

Fonte: `packages/plugin/src/source.bun.ts@v2.3.0`:
```ts
new Bun.Transpiler({ ... }).scan(...)   // ← usa APIs exclusivas do Bun
Bun.resolveSync(...)
```

O arquivo `source.node.ts` existe (Node hook para reloading em dev), mas o runtime de execução
dos plugins é **Bun**. O V1 também usava Bun (a `PluginInput` incluía `$: BunShell`).

### 4.2 Chamar binário externo com JSON no stdin

**Contrato do guard** (fonte: `internal/commands/guard.go`, `internal/guard/payload.go`,
`internal/guardforms/guard_generations.go`):

| Guard | Comando | Exit 0 | Exit 2 |
|---|---|---|---|
| Credencial | `trackfw guard credential` | liberado | bloqueado |
| Git branch | `trackfw guard git-branch` | liberado | bloqueado |

O guard lê stdin como JSON. Para o **git-branch guard**, o campo `command` é extraído por prioridade
(fonte: `internal/guard/payload.go` `ExtractCommand`):
1. `tool_input.command`
2. `command` (raiz)
3. `tool_info.command_line`
4. `hook_input.command`
5. `toolCall.args.CommandLine` (somente payload Antigravity)

Para o **credential guard**, todo o payload JSON é varrido como bytes brutos procurando padrões de
credencial (JWT, AWS key, etc.) — a estrutura de chaves não importa.

**Payload recomendado** — spread de `event.input` + campo `cwd` para Layer 2 (glob de paths relativos):

O campo `cwd` no raiz do JSON é lido por `credExtractCmdAndCwd` (D4 fallback) e repassado a
`credGlobToken` como `baseCwd` para resolver glob patterns relativos nos redirects do comando
(fonte: `credential.go` linhas 472–496). Sem `cwd`, o guard usa o processo cwd (que é `projectDir`),
não o `workdir` do shell — podendo errar para redirects relativos como `> ./logs/out.txt`.

```json
{
  "command": "git commit -am 'wip'",
  "workdir": "/repo",
  "cwd": "/repo/src"
}
```

No plugin: `{ ...shellInput, cwd: path.resolve(projectDir, shellInput.workdir ?? ".") }`

Bun é compatível com as APIs de `node:child_process`.

Os dois guards diferem em como usam o `cwd` do processo (fonte: `internal/guard/credential.go` e
`internal/guard/gitbranch.go`):
- **`credential`**: chama `os.Getwd()` e procura `trackfw.yaml` **diretamente** nesse diretório
  (`filepath.Join(cwd, "trackfw.yaml")`) — sem walk-up. Se o `cwd` passado for um subdiretório
  do projeto, o guard não encontra `trackfw.yaml` e retorna exit 0 (no-op). Passe **sempre a raiz
  do projeto** (`projectDir`).
- **`git-branch`**: chama `FindProjectRoot(os.Getwd())` — faz walk-up. Qualquer `cwd` dentro da
  árvore do projeto funciona; usar `workdir` do shell ou `projectDir` como fallback é seguro.

Opção A — `node:child_process` (síncrono, portável Node/Bun):
```ts
import { spawnSync } from "node:child_process"
import { resolve as resolvePath } from "node:path"

// Spread event.input para colocar "command" na raiz (priority 2 de ExtractCommand)
const shellInput = event.input as { command?: string; workdir?: string }
// Resolve workdir antes de usar como cwd (pode ser relativo — M18 aberto)
const resolvedWorkdir = shellInput.workdir ? resolvePath(projectDir, shellInput.workdir) : undefined
// Inclui "cwd" no payload para Layer 2 do credential guard (resolve globs relativos)
const payload = { ...shellInput, cwd: resolvedWorkdir ?? projectDir }

// credential: cwd = projectDir (checa trackfw.yaml diretamente, sem walk-up)
// projectDir = ctx.location.project.directory (V2) ou worktree (V1)
const credResult = spawnSync("trackfw", ["guard", "credential"], {
  input: JSON.stringify(payload),
  encoding: "utf8",
  timeout: 5000,
  cwd: projectDir,                              // ← sempre a raiz do projeto
})

// git-branch: cwd = workdir do shell ou raiz (FindProjectRoot faz walk-up)
const branchResult = spawnSync("trackfw", ["guard", "git-branch"], {
  input: JSON.stringify(payload),
  encoding: "utf8",
  timeout: 5000,
  cwd: resolvedWorkdir ?? projectDir,           // ← workdir resolvido
})
// result.status === 0 → liberado; !== 0 → bloqueado; null → crash/sinal/timeout
```

Opção B — `Bun.spawnSync` (síncrono, Bun nativo):
```ts
import { resolve as resolvePath } from "node:path"

const shellInput = event.input as { command?: string; workdir?: string }
const resolvedWorkdir = shellInput.workdir ? resolvePath(projectDir, shellInput.workdir) : undefined
const payload = new TextEncoder().encode(JSON.stringify({ ...shellInput, cwd: resolvedWorkdir ?? projectDir }))

const credResult = Bun.spawnSync(["trackfw", "guard", "credential"], {
  stdin: payload,
  cwd: projectDir,                              // ← sempre a raiz do projeto
  // Nota: Bun.spawnSync e timeout — confirmar M16 no ML-5B
})
const branchResult = Bun.spawnSync(["trackfw", "guard", "git-branch"], {
  stdin: payload,
  cwd: resolvedWorkdir ?? projectDir,           // ← workdir resolvido
})
// result.exitCode === 0 → liberado; !== 0 → bloqueado
```

**Timeout:** `node:child_process.spawnSync` suporta a opção `timeout` nativamente. Quando o processo
excede o tempo: `result.error.code = "ETIMEDOUT"`, `result.status = null`, `result.signal = "SIGTERM"`.
O padrão `result.error || result.status !== 0` falha fechado.
Verificado empiricamente: `spawnSync('sleep', ['10'], {timeout: 100})` → `{error:{code:'ETIMEDOUT'}, status:null}`.
Adicionar `timeout: 5000` (5 s) ao objeto de opções. A confirmar no ML-5B se `Bun.spawnSync` honra
a mesma opção ao chamar `node:child_process.spawnSync` (M16; §4 e §8 usam `node:child_process`, não `Bun.spawnSync`).

> **Tools cobertos pelo credential guard** (fonte: `internal/generators/agentfiles.go`:
> `InjectClaudeHooks` linhas 414–429, `InjectCodexHooks` linhas 716–725, `InjectGeminiHooks`
> linhas 869–880, `InjectKiroHooks` linhas 983–1035):
>
> | CLI | Shell | File tools |
> |---|---|---|
> | Claude Code | `Bash\|PowerShell` | `Read`, `Write\|Edit` |
> | Codex | `Bash`, `apply_patch` | — |
> | Gemini CLI | `run_shell_command` | `read_file\|read_many_files`, `write_file\|replace` |
> | Kiro | `shell` | `read`, `write` |
>
> Todos os CLIs têm **pre + post** (before/after): o credential guard também roda **após** o
> tool executar — onde o conteúdo do arquivo lido fica disponível. OpenCode V2 não suporta
> bloqueio em `execute.after` (canal de falha é `never` — confirmado por hooks.ts@v2.3.0).
> O plugin cobre apenas **pre**; **post é gap de paridade com todos os outros CLIs**.
>
> O git-branch guard cobre **apenas shell** (não file tools).
>
> **Para o plugin OpenCode cobrir o escopo pre equivalente aos outros CLIs, o credential guard
> deve ser chamado para `shell`, `write`, `edit` e `read` (V2). Em V1, os IDs são os mesmos
> (`"write"`, `"read"`, `"edit"` confirmados por fonte), mas o campo de caminho é `filePath`
> (não `path`) — ver §2.3.**
>
> O guard scana os bytes brutos do payload (Layer 1) — `content` de um `write` ou `newString`
> de um `edit` são varridos automaticamente. `tool_name` não precisa ser declarado explicitamente
> para o caso base; o guard usa `tool_name` para três funções distintas (fonte: `credential.go`):
> (a) normalização de caminhos PowerShell/Windows (`credIsPowerShellTool`);
> (b) isenção do `fs_write` (Amazon Q) — quando `tool_name == "fs_write"`, `command` é tratado como enum tag, não comando shell;
> (c) roteamento do scan D5 (Antigravity) — quando `tool_name == "view_file"`, o guard lê o arquivo em `toolCall.args.AbsolutePath`.
> Para payloads OpenCode sem `tool_name`, essas três rotas são no-op inerte (não bloqueiam indevidamente).
>
> **Windows — `.cmd` shim:** `spawnSync("trackfw", ...)` em Bun/Node pode não resolver o shim npm
> `trackfw.cmd` em `PATH`. Se o binário não for encontrado, `result.error` conterá `ENOENT`.
> A política fail-closed do código acima bloqueará o tool call. A medir em ML-5B (M14).
>
> **Guard global vs projeto:** `trackfw guard credential --global` existe e usa mode=block mesmo fora
> de um projeto trackfw. Um plugin instalado em `~/.config/opencode/plugins/` pode querer `--global`.
> Essa é uma decisão arquitetural para o arquiteto; não definida aqui.
>
> **`spawnSync` bloqueia o event loop:** V1 usa `Effect.promise` e V2 usa `tryPromise` — ambos
> aguardam callbacks assíncronos. É viável usar `spawn` assíncrono + `await` em vez de `spawnSync`.
> Para o MVP, `spawnSync` é simples e suficiente se o guard for rápido (< 5 s com timeout).

---

## 5. Plugin `.js`/`.ts` local sem npm

**Sim, funciona** nos dois V1 e V2.

Basta criar `~/.config/opencode/plugins/trackfw-guard.ts` (global) ou
`.opencode/plugins/trackfw-guard.ts` (projeto). O OpenCode carrega automaticamente.

Nenhum `package.json`, `npm install` ou publicação é necessário para arquivo solo.

Exemplo mínimo de plugin V2 sem dependência extra além de `@opencode/plugin`
(que é provida pelo host — não precisa estar em `node_modules` do plugin):

```ts
// .opencode/plugins/trackfw-guard.ts  — funciona SOMENTE em V2
// Plugin.define é no-op (fonte: packages/plugin/src/promise/plugin.ts@v2.3.0).
// V2 loader detecta {id, setup} por shape (fonte: module.ts@v2.3.0 Schema.Union).
import { Plugin } from "@opencode/plugin"
import { spawnSync } from "node:child_process"
import { resolve as resolvePath } from "node:path"

function runGuard(cmd: string, payload: unknown, cwd: string): void {
  // timeout: spawnSync opção nativa — no timeout: result.error.code=ETIMEDOUT, status=null
  // → result.error || status !== 0 falha fechado. Verificado empiricamente em Node.js.
  const result = spawnSync("trackfw", ["guard", cmd], {
    input: JSON.stringify(payload),
    encoding: "utf8",
    timeout: 5000,
    cwd,
  })
  if (result.error || result.status !== 0) {
    throw new Error(
      result.error
        ? `trackfw não encontrado: ${result.error.message}`
        : result.stderr?.trim() || `trackfw guard ${cmd}: operação bloqueada`
    )
  }
}

export default Plugin.define({
  id: "trackfw.guard",
  async setup(ctx) {
    // ctx.location.project.directory = raiz do projeto (onde trackfw.yaml está)
    // confirmado no tipo Context em packages/plugin/src/promise/plugin.ts@v2.3.0:
    //   readonly location: Location.Info   (Location.Info.project.directory: AbsolutePath)
    //   fonte de Location.Info: packages/schema/src/location.ts@v2.3.0
    const projectDir = ctx.location.project.directory
    await ctx.tool.hook("execute.before", (event) => {
      const inp = event.input as { command?: string; workdir?: string }
      // Resolve workdir antes de usar como cwd — pode ser relativo (M18 aberto)
      const resolvedWorkdir = inp.workdir ? resolvePath(projectDir, inp.workdir) : undefined
      // Inclui "cwd" no payload para Layer 2 do credential guard (resolve globs relativos)
      const payload = { ...inp, cwd: resolvedWorkdir ?? projectDir }
      if (event.tool === "shell") {
        // Shell: credential (cwd=projectDir) + git-branch (cwd=workdir, walk-up)
        runGuard("credential", payload, projectDir)
        runGuard("git-branch", payload, resolvedWorkdir ?? projectDir)
      } else if (["write", "edit", "read", "patch"].includes(event.tool)) {
        // File tools: credential only — Layer 1 varre path/content/oldString como raw bytes
        runGuard("credential", payload, projectDir)
      }
    })
  },
})
```

> **Fail-closed vs fail-open:** o código acima lança quando `result.error` (binário ausente) OU
> `result.status !== 0`. Se a política for fail-open quando o guard não está instalado, substitua
> `result.error || result.status !== 0` por `result.status === 2`.
>
> **Os demais CLIs (Claude, Codex, Gemini) são fail-closed:** o comando gerado por `guardforms.go`
> é `trackfw guard credential; LASTEXITCODE=$((2*!!$?))` — se o binário não for encontrado a shell
> retorna exit 127, `2*!!127 = 2`, e o hook bloqueia o tool call. Um plugin com `status === 2`
> divergiria desse comportamento.
>
> **Guard de credencial vs branch:** `trackfw guard credential` escaneia o payload por padrões de
> credencial (JWT, chaves AWS, etc.). `trackfw guard git-branch` usa `ExtractCommand` para extrair
> o `command` e verificar se ele tenta operar em branch proibida. Instale os dois em sequência para
> cobertura completa.

Para V1 (opencode 1.19.5), o formato é diferente — ver §2.2 e §8 (dual-export).

---

## 6. `opencode` 1.19.5 (Homebrew, Mac KG) × `@opencode/cli` 2.3.0 (npm, Windows)

**Veredito: mesmo produto, versões diferentes (V1 × V2) do mesmo repositório.**

Provas por fonte primária:

| Prova | Valor |
|---|---|
| `brew info opencode` → "From" | `github.com/anomalyco/opencode` (homepage na fórmula) |
| `npm view @opencode/cli` → `repository.url` | `git+https://github.com/anomalyco/opencode.git` |
| `curl api.github.com/repos/anomalyco/opencode` → `homepage` | `https://opencode.ai` |
| `opencode --version` local | `1.19.5` |
| Releases no GitHub (`/releases`) | `v1.19.6` (V1 latest) e `v2.3.2` (V2 latest) — no mesmo repo |
| `@opencode/cli` latest em npm | `2.3.2` |
| `opencode-ai` latest em npm | `1.19.6` |
| `github.com/sst/opencode` | HTTP 301 → `github.com/anomalyco/opencode` (migração de org) |

**Conclusão:**
- `opencode` 1.19.5 (fórmula `anomalyco/tap/opencode`) = V1 (série `1.x`) do `anomalyco/opencode`
- `@opencode/cli` 2.3.0 (npm) = V2 (série `2.x`) do mesmo repo
- `opencode-ai` no npm = V1 distribuído via npm (série `1.x`; `packages/opencode/package.json@v1.19.5`
  tem `name: opencode` — a distribuição como `opencode-ai` é feita separadamente, não pelo nome nativo)
- `anomalyco/tap/opencode-v2` = fórmula Homebrew do V2

As duas versões **não são compatíveis no nível de plugins**:
- V1 chama `server()` e usa os hooks retornados (`"tool.execute.before": async (input, output) => ...`)
- V2 chama `setup()` e registra hooks via `ctx.tool.hook("execute.before", ...)`
- V1 não carrega V2 plugins registrados via a chave `plugins` em `opencode.jsonc` (chave marcada "unsupported" por v2-compat.ts); V2 ignora `server()` e chama `setup()`
- Um arquivo autodescoberto com `{id, setup, server}` funciona em ambos (ver §8 dual-export)
- Um pacote só com `setup` não funciona em V1 (V1 lança erro se `server` estiver ausente)

Citação de `migrate-v1.md`:
> "V1 plugin implementations do not run in V2. Moving a file or renaming its config entry is not enough."

Prova adicional de que V1 não carrega plugins V2 (`packages/opencode/src/config/v2-compat.ts@v1.19.5`,
linhas 117–118):
```ts
// V1's V2-compat downgrader: "plugins" key is explicitly marked as unsupported
for (const key of ["plugins", "providers", "websearch", "warming"])
  if (Object.hasOwn(parsed.value, key)) unsupported([key], diagnostics)
```
A chave `plugins` (a do V2) é marcada como `"unsupported"` pelo downgrader de config V1 — o V1
não apenas ignora a chave: reporta-a como inválida. Isso confirma que um plugin registrado via a
chave `plugins` no `opencode.jsonc` V2 **nunca carrega** em hosts V1 (a chave é rejeitada).

> **Nota:** isso não impede que V1 carregue um arquivo `{id, setup, server}` autodescoberto via
> glob `{plugin,plugins}/*.{ts,js}` — V1 carrega o arquivo e chama `server()`. O `setup()` é
> ignorado não porque V1 rejeita a chave, mas porque `readV1Plugin("detect")` só chama `server()`.
> Se `server` estiver ausente e `id` estiver presente, V1 lança erro (confirmado por fonte).
> Um arquivo dual-export com `{id, setup, server}` funciona no V1 porque `server` está presente.

---

## 7. Itens para o ML-5B medir (no binário real)

Estes itens requerem o binário instalado e não podem ser derivados só da documentação.

| # | Item | Binário necessário | Resultado esperado (fonte) |
|---|---|---|---|
| M1 | Confirmar que `~/.config/opencode/plugins/` é o global no Mac com `opencode` 1.19.5 (V1) | V1 1.19.5 | `~/.config/opencode/` (XDG sem `XDG_CONFIG_HOME` = `~/.config`) |
| M2 | Confirmar que `~/.config/opencode/plugins/` é o global no Windows com `@opencode/cli` 2.3.0 (V2) | V2 2.3.0 (Windows KG) | `C:\Users\<user>\.config\opencode\plugins\` |
| M3 | Confirmar que single `.ts` em `~/.config/opencode/plugins/` auto-carrega no V1 sem config extra | V1 1.19.5 | sim (glob `{plugin,plugins}/*.ts`) |
| M4 | Confirmar que single `.ts` em `~/.config/opencode/plugins/` auto-carrega no V2 | V2 | sim (autodiscovery) |
| M5 | Confirmar nome do tool shell no V1 — `"bash"` conforme ID no código-fonte | V1 1.19.5 | `"bash"` (**confirmado por fonte**: `packages/opencode/src/tool/shell/id.ts@v1.19.5` → `ToolID = "bash"`) |
| M6 | Confirmar shape de `output.args` para o tool `bash` no V1 (`command`, `workdir?`, `timeout?`) | V1 1.19.5 | `{ command, workdir?, timeout? }` (**confirmado por fonte**: `packages/opencode/src/tool/shell/prompt.ts@v1.19.5`; sem campo `background`) |
| M7 | Confirmar que `throw` de V1 `"tool.execute.before"` bloqueia o tool e que a mensagem chega ao modelo | V1 1.19.5 | bloqueia (fonte: `Effect.promise` falha se rejeitar → propaga no `gen`); mas V1 usa `Effect.promise` (defect), não `tryPromise` (typed `Tool.Error`) como V2 — a mensagem visível ao modelo e a sobrevivência da sessão devem ser medidas no binário |
| M8 | Confirmar que `Bun.spawnSync` / `node:child_process.spawnSync` está disponível no V1 plugin runtime | V1 1.19.5 | sim (Bun runtime desde V1; `$: BunShell` no PluginInput) |
| M9 | Confirmar que `output.args.workdir` (V1) / `event.input.workdir` (V2) está acessível no hook para identificar o cwd do shell — e que é diferente de `ctx.location.project.directory` / `worktree` (que é a raiz do projeto). NÃO é `ctx.location.directory` / `directory` que são o workspace cwd. | V1+V2 | `workdir` disponível por fonte; `ctx.location.directory` = workspace cwd, NÃO raiz do projeto |
| M10 | Verificar se a variável `OPENCODE_CONFIG_DIR` altera o diretório global no Mac com 1.19.5 | V1 1.19.5 | sim (código presente em `global.ts`; existia em V1?) |
| M11 | Medir se `opencode-ai` (npm V1) é funcionalmente idêntico ao `opencode` (Homebrew V1) | V1 instalado por npm | provável sim; confirmar `--version` |
| M12 | Confirmar que o field do workdir no tool shell V1 é `workdir` (não `cwd`) | V1 1.19.5 | `workdir` (**confirmado por fonte**: `Schema.optional(Schema.String)` como `workdir` em `prompt.ts@v1.19.5`) |
| M13 | Verificar se `external.ts@v1.19.5` é fiado pelo binário V1 (dual-load risk) | V1 1.19.5 | Não fiado por fonte (V1 usa `config/plugin.ts`, não `core/config/plugin/external.ts`); confirmar no binário |
| M14 | Windows: `spawnSync("trackfw", ...)` em Bun/Node resolve o shim npm `trackfw.cmd`? | V2 2.3.0 (Windows KG) | Não testado; se ENOENT, `result.error` está definido → fail-closed (bloqueia tudo) |
| M15 | Confirmar comportamento de falha de load no V2.3.0: erro silencioso (fail-open) ou aviso ao usuário? `external.ts@v1.19.5` usava `Effect.ignoreCause`; `module.ts@v2.3.0` retorna `LoadError` tipado — onde é capturado? | V2 2.3.0 | Desconhecido; `external.ts@v1.19.5` sugeria fail-open silencioso, mas `module.ts@v2.3.0` reestruturou o loader |
| M16 | Confirmar que `Bun.spawnSync` honra a opção `timeout` (Node.js: `child_process.spawnSync` a suporta nativamente; Bun pode divergir) | V2 2.3.0 (Bun runtime) | Node.js suporta (fonte: nodejs.org/api/child_process.html); Bun a confirmar |
| M17 | IDs dos tools de file: **FECHADO por fonte** — `"write"`, `"read"`, `"edit"` (mesmos IDs que V2; confirmados em `packages/opencode/src/tool/{write,read,edit}.ts@v1.19.5`). Campo de caminho é `filePath` em V1, `path` em V2. §8 cobre V1 file tools. **Shape dos args de `apply_patch` (V1) e `patch` (V2) não foram inspecionados** — confirmar em ML-5B (IDs dos dois confirmados; shapes abertos). | V1+V2 | IDs: Fechado; shapes de patch/apply_patch: abertos |
| M18 | `workdir` em V1/V2 é sempre absoluto? Para Layer 2 do credential guard (`credGlobToken`), o payload deve incluir `cwd = path.resolve(projectDir, workdir ?? ".")` para resolver glob patterns relativos. Confirmar se `workdir` já é absoluto (simplificaria) | V1+V2 | Não verificado; §4.2 inclui `cwd` no payload por precaução |
| M19 | `ctx.location.project.directory` (V2) e `worktree` (V1) são a raiz do projeto? **Confirmado em fonte:** `ctx.location: Location.Info` está no tipo `Context` (`packages/plugin/src/promise/plugin.ts@v2.3.0`); `Location.Info.project.directory: AbsolutePath` (`location.ts@v2.3.0`). V1 `worktree` = raiz git (derivado do projeto git); **edge case: diretório não-git** — `NotGitError` é possível (`worktree/index.ts@v1.19.5`). Runtime V2 (valor real) a confirmar ML-5B. | V1+V2 | `ctx.location` confirmado em tipo; valor runtime a confirmar. V1 não-git: edge case pendente |
| M20 | `execute.after` para warn-only parity: V2 canal `never` confirmado (não pode bloquear via throw). **V2 `result` é mutável** (campo `result: Tool.Result` SEM `readonly` em `ToolHooks["execute.after"]` — confirmado em `packages/plugin/src/effect/tool.ts@v2.3.0`). Se mutar `result` alterar o que o modelo recebe, **redaction** é viável em V2 sem throw. V1 `output.output: string` passado por referência (padrão análogo ao `args` do before-hook). Se mutá-lo alterar o contexto do modelo, redaction também é viável em V1. **Verificação runtime pendente:** a mutação de `result` (V2) e `output.output` (V1) chega ao modelo? Confirmar em ML-5B. Decisão arquitetural para o arquiteto. | V1+V2 | Mutabilidade de `result` confirmada por fonte; efeito runtime a confirmar |
| M21 | **PATH binary skew:** o plugin chama `spawnSync("trackfw")` que resolve pelo PATH. No Mac KG, PATH → Homebrew 11.5.0; `bin/trackfw` local é 11.7.0. T-S/T-WV1/T-WV2 medidos em ambos: mesmos resultados. T-D5 difere (D5 = feature não-tagueada em 11.5.0). **Ainda não medido:** root-`cwd` D4 fallback em 11.5.0, e Windows npm shim (`trackfw.cmd` resolve pelo PATH?). ML-5B deve registrar `trackfw --version` do PATH binary em cada máquina. | Mac + Windows KG | Mac 11.5.0 medido (T-S/T-WV1/T-WV2/T-D5/T-RC/T-RV1/T-RV2); Windows a medir |

---

## 8. Suporte simultâneo V1 e V2 (dual-export)

Para um único arquivo `.ts` funcionar tanto no Mac do KG (V1 1.19.5) quanto no Windows (V2 2.3.0):

Fonte: `docs/build/plugins/server.md` ("Support V1") e `docs/build/plugins/migrate-v1.md`.

> **Aviso de importação:** `import { Plugin } from "@opencode/plugin"` é o pacote V2.
> V1 provê `@opencode-ai/plugin`. Um arquivo que importe `@opencode/plugin` falha ao carregar
> em hosts V1 (módulo não encontrado → caso B de fail-open do §3). Para compatibilidade dual,
> use a forma zero-import abaixo.
>
> **V2 detecta {id, setup} por shape** (fonte: `packages/core/src/plugin/module.ts@v2.3.0`
> Schema.Union: `{id, effect} | {id, setup}` — confirmado; e
> `packages/plugin/src/promise/plugin.ts@v2.3.0` — `define` é no-op: `return plugin`).
>
> **Falha silenciosa no load (fail-open):** em `packages/core/src/config/plugin/external.ts@v1.19.5`
> (loader V2 que já estava no monorepo antes da série 2.x) o load termina em `.pipe(Effect.ignoreCause)`,
> o que suprime o erro sem notificar o usuário. Em `packages/core/src/plugin/module.ts@v2.3.0` o
> loader foi reestruturado e retorna `LoadError` tipado em vez de usar `ignoreCause` diretamente;
> onde esse `LoadError` é capturado na cadeia de chamada V2.3.0 não foi rastreado nesta investigação
> — o comportamento observável (erro silencioso vs. aviso ao usuário) deve ser confirmado no ML-5B.
> A afirmação "V2 é fail-open no load" é derivada de `external.ts@v1.19.5` e pode ter evoluído.

```ts
// .opencode/plugins/trackfw-guard.ts — dual-export V1 + V2
//
// Por que zero-import é seguro (ver §8 intro para detalhes):
//   - Plugin.define em V2 é `return plugin` (no-op); V2 detecta {id, setup} por shape
//     via Schema.Union (fonte: packages/core/src/plugin/module.ts@v2.3.0).
//     packages/plugin/src/promise/plugin.ts@v2.3.0 confirma: define = `return plugin`.
//   - V1 (1.19.5) chama server() via readV1Plugin("detect") e ignora setup().
//     Se server() estiver ausente e id presente, readV1Plugin("detect") lança erro (fonte: v1 plugin/index.ts).
//     A chave `plugins` (config V2) é marcada "unsupported" por v2-compat.ts@v1.19.5 (linhas 117–118).
//     (packages/core/src/config/plugin/external.ts = loader V2 no monorepo, NÃO fiado pelo binário V1.)
//   - Objeto com {id, setup, server}: V2 lê setup(); V1 lê server(). Sem duplo disparo.
import { spawnSync } from "node:child_process"
import { resolve as resolvePath } from "node:path"

function callGuard(
  guardCmd: string,
  cmdInput: { command?: string; workdir?: string },
  projectDir: string,  // V2: ctx.location.project.directory; V1: worktree
) {
  // Resolve workdir antes de usar como cwd — pode ser relativo (M18 aberto)
  const resolvedWorkdir = cmdInput.workdir ? resolvePath(projectDir, cmdInput.workdir) : undefined
  // Spread coloca "command" na raiz → ExtractCommand priority 2
  // Inclui "cwd" para Layer 2 resolve de glob patterns relativos (§4.2)
  const payload = {
    ...cmdInput,
    // "cwd" raiz → Layer 2 credGlobToken usa como baseCwd para resolve de globs relativos
    cwd: resolvedWorkdir ?? projectDir,
  }
  // process cwd difere por guard:
  //   credential → projectDir (checa trackfw.yaml diretamente, sem walk-up)
  //   git-branch → workdir resolvido ?? projectDir (FindProjectRoot faz walk-up)
  const guardCwd = guardCmd === "credential" ? projectDir : (resolvedWorkdir ?? projectDir)
  const result = spawnSync("trackfw", ["guard", guardCmd], {
    input: JSON.stringify(payload),
    encoding: "utf8",
    timeout: 5000,   // ms; result.error.code = ETIMEDOUT, status = null → fail-closed
    cwd: guardCwd,
  })
  if (result.error || result.status !== 0) {
    throw new Error(
      result.error
        ? `trackfw não encontrado: ${result.error.message}`
        : result.stderr?.trim() || `trackfw guard ${guardCmd}: operação bloqueada`
    )
  }
}

export default {
  id: "trackfw.guard",

  // V2 — chama setup(); ignora server()
  async setup(ctx: any) {
    // ctx.location: confirmado no tipo Context (packages/plugin/src/promise/plugin.ts@v2.3.0):
    //   export interface Context { readonly location: Location.Info; ... }
    // Location.Info.project.directory = raiz do projeto (AbsolutePath; schema: location.ts@v2.3.0)
    // Location.Info.directory = workspace/cwd de lançamento (pode ser subdiretório)
    // credential guard: os.Getwd() direto → usar project.directory, não directory
    const projectDir: string = ctx.location.project.directory
    await ctx.tool.hook("execute.before", (event: any) => {
      const inp = event.input as { command?: string; workdir?: string }
      if (event.tool === "shell") {           // V2: tool shell tem ID "shell"
        // Shell: credential (cwd=projectDir) + git-branch (cwd=workdir, walk-up)
        callGuard("credential", inp, projectDir)
        callGuard("git-branch", inp, projectDir)
      } else if (["write", "edit", "read", "patch"].includes(event.tool)) {
        // File tools: credential only (parity pre-tool com Claude/Gemini/Kiro)
        // Layer 1 varre raw bytes — path/content/oldString/newString escaneados automaticamente
        callGuard("credential", inp, projectDir)
      }
    })
  },

  // V1 — chama server(); ignora setup()
  async server({ directory, worktree }: { directory: string; worktree: string }) {
    // worktree = raiz do projeto (git worktree root; onde trackfw.yaml está)
    // directory = cwd do workspace (pode ser subdiretório)
    // fonte: packages/plugin/src/index.ts@v1.19.5 PluginInput.worktree + .directory
    return {
      "tool.execute.before": async (
        input: { tool: string },
        output: { args: { command?: string; workdir?: string; filePath?: string } },
      ) => {
        if (input.tool === "bash") {          // V1: tool shell tem ID "bash"
          // Shell: credential + git-branch guards
          callGuard("credential", output.args, worktree)
          callGuard("git-branch", output.args, worktree)
        } else if (["write", "edit", "read", "apply_patch"].includes(input.tool)) {
          // V1 file tools: mesmos IDs que V2 exceto apply_patch (V1) vs patch (V2)
          // campo de caminho é filePath (V1), não path (V2) — Layer 1 varre bytes brutos
          // apply_patch confirmado em packages/opencode/src/tool/apply_patch.ts@v1.19.5
          callGuard("credential", output.args, worktree)
        }
      },
    }
  },
}
```

Citação de `migrate-v1.md`:
> "V1 object entrypoints are supported in OpenCode `1.18.29`. Older V1 releases may expect function exports
> instead; test the installed package with the oldest V1 release you intend to support and with V2."

Como KG usa 1.19.5 (> 1.18.29), o formato objeto V1 está coberto.

---

## 9. Limitações desta investigação (não medido)

- **M1–M4, M8, M10, M11** (§7) são derivados de código-fonte; comportamento real não foi confirmado no binário.
- **M5, M6, M7, M9, M12** têm resultado esperado confirmado por fonte primária mas ainda não
  validado no binário real — podem divergir se V1 1.19.5 diferir do código lido.
- **M15** (§7): comportamento de falha de load no V2.3.0 — `external.ts@v1.19.5` usava `Effect.ignoreCause`;
  `module.ts@v2.3.0` retorna `LoadError` tipado. Onde é capturado não foi rastreado; medir no ML-5B.
- **M16** (§7): `Bun.spawnSync` honra a opção `timeout`? Node.js suporta; confirmar para Bun.
- **M17** (§7): IDs de file tools V1 **fechados por fonte** (`"write"`, `"read"`, `"edit"`, `"apply_patch"`). Shapes dos args de `apply_patch` (V1) e `patch` (V2) não inspecionados — abertos para ML-5B.
- **M18** (§7): `workdir` absoluto ou relativo? Não verificado; payload inclui `cwd: path.resolve(projectDir, workdir ?? ".")` como precaução para Layer 2.
- **M19** (§7): `ctx.location` accessor **confirmado em tipo** (`packages/plugin/src/promise/plugin.ts@v2.3.0` — `Context.location: Location.Info`; `Location.Info.project.directory: AbsolutePath` em `location.ts@v2.3.0`). Valor runtime a confirmar ML-5B. V1 `worktree` = git root por fonte; non-git dirs pendentes.
- **M20** (§7): V2 `result` é mutável (confirmado por `effect/tool.ts@v2.3.0` — sem `readonly`). V1 `output.output` por referência. Se mutar `result`/`output.output` alterar o que o modelo vê, redaction é viável sem throw em ambas versões. Verificação runtime pendente; decisões de wiring e redaction para o arquiteto.
- **Code-mode bypass (V1):** `code-mode.ts@v1.19.5` linha 141 chama `plugin.trigger("tool.execute.before", ...)` — o guard dispara nesse path. **Sem bypass.**
- **Layer 2 cwd:** `credGlobToken` usa `shellCwd` (do campo `cwd` raiz no payload) como baseCwd para resolve de glob patterns relativos. §4.2 inclui `cwd` no payload.
- O timeout do hook V2 (quanto tempo `execute.before` pode levar antes de o OpenCode interromper)
  não está documentado na doc pública — a medir no ML-5B.
- Interação entre o guard global (`~/.config/opencode/plugins/`) e um plugin de projeto
  (`.opencode/plugins/`) com o mesmo `id` — a doc diz que todos são carregados, mas não
  documenta o comportamento de duplicata de ID.
- O campo `background` do shell tool V2 não existe em V1 (confirmado por fonte); qualquer
  guard que inspecione `input.background` deve checar a versão antes.
- **Possíveis bypass em V1:** `acp/tool.ts@v1.19.5` também chama `plugin.trigger("tool.execute.before")` — o guard dispara nesse path. `cli/cmd/run/tool.ts@v1.19.5` não chama diretamente (confirmado por grep); `task.ts` cria subtasks que passam por `session/tools.ts`. A cobertura parece completa, mas a medir no ML-5B.
- **M21** (§7): PATH binary skew — Mac KG: 11.5.0 (PATH) vs 11.7.0 (bin/). T-S/T-WV1/T-WV2 idênticos em ambos. T-D5=0 em 11.5.0 (D5 não-tagueada, sem regressão). D4 root-`cwd` fallback em 11.5.0 e Windows a medir no ML-5B.
- **Contrato do guard** (fonte: `internal/commands/guard.go`, `internal/guardforms/guard_generations.go`, `internal/guard/payload.go`): confirmado por código local. Exit 0 = allow, exit 2 = block. Payload: `command` em raiz (priority 2 de `ExtractCommand`). Binário: `trackfw guard credential` / `trackfw guard git-branch`.

## 8. Medido no binário real (ML-5B, 2026-11-19, KG) — opencode v2.3.2, `anthropic/claude-haiku-5-5`

Plugin de sonda `opencode-sondas/guard-probe-plugin.js` (spawnSync com argv, sem shell; `throw` se
`result.error || status !== 0`), trackfw 11.7.0 da branch, `--standalone` em todos os braços, 6/6 chamadas por máquina.
macOS (Homebrew `opencode` 2.3.2) e Windows 11 ARM64 (npm `@opencode/cli` 2.3.2; shell da ferramenta = `cmd.exe`)
deram o **mesmo** resultado:

| Braço | Resultado (Mac = Windows) |
|---|---|
| C1 chave fake em `shell` | **bloqueado** — log do plugin `status=2`, marker ausente |
| C2 `echo hello` | liberado — credential e git-branch `status=0`, marker criado (prova que o hook dispara) |
| C3 `trackfw` fora do PATH | **bloqueado** — `error_code=ENOENT`, marker ausente (fail-closed) |
| C4 `read` de arquivo com a chave | liberado — `status=0`, args só com caminho (gap T-RV, mesma causa do D5 `view_file`) |
| C5 `OPENCODE_CONFIG_PROJECT_DISABLE=1` | o comando com a chave **executou**; plugin de projeto não carregou (residual T6) |
| C6 só plugin global | **bloqueado** — `status=2`; global em `~/.config/opencode/plugins/` (Windows: `%USERPROFILE%\.config\opencode\plugins\`) |

Nomes medidos das ferramentas em V2: `shell` e `read` (args `path`; a sonda acrescentou `file_path` absoluto).
Não medido: V1 (1.19.x) no binário (o run do Mac em 1.19.6 não chegou ao modelo), `edit`/`patch`, MCP e `task`.

### 8.1 Plugin do produto (ML-5H, 2026-11-20, KG) — opencode v2.3.2, `anthropic/claude-haiku-5-5`

Plugin `trackfw-guard.js` gerado pelo `trackfw update` (projeto) e pelo `update harness --targets opencode-guard` (global),
trackfw da branch em d3bcc446, 7/7 chamadas por máquina. macOS e Windows 11 ARM64 deram o **mesmo** resultado:

| Braço | Resultado (Mac = Windows) |
|---|---|
| P1 chave fake em `shell` | **bloqueado**: `trackfw-credential-guard: blocked`, marker ausente |
| P2 `echo hello` | liberado, marker criado |
| P3 `git commit --allow-empty` | **bloqueado**: `raw git commit blocked`, commits 1 → 1 (fecha o C7) |
| P4 `read` de arquivo com a chave | **bloqueado**: o guard lê o arquivo (fecha o gap do C4) |
| P5 `write` em `.opencode/plugins/evil.js` | **bloqueado** pela autoproteção D4, arquivo não criado |
| P6 `webfetch` com a chave na URL | **bloqueado** |
| P7 só plugin global | **bloqueado**; o harness reporta `updated=1`, e o global é removido ao final |
