---
title: Remedição do schema de hook dos 8 CLIs de agente — ADR-2026-10-04
date: 2026-10-04
author: prometeu-tf
status: investigação concluída
---

# Remedição do schema de hook dos CLIs de agente

> 🔴 Documento de investigação pura. Nenhum código foi alterado, nenhum hook foi regenerado, nenhuma
> operação de git foi executada. ML-0C do
> `ROADMAP-2026-09-22-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md`.

## Objetivo

Responder, para cada um dos 8 CLIs emitidos por `internal/generators/agentfiles.go`, as perguntas da
ADR-2026-10-04:

- **(a)** Com que shell o `command` do hook roda no Windows?
- **(b)** Existe campo por plataforma ou por shell no schema?
- **(c)** Qual exit code bloqueia, e exit ≠ 2 (p. ex., exit 1 quando o binário não existe) = permitir ou bloquear?
- **(d)** Qual é o cwd do processo do hook? O guard precisa achar `trackfw.yaml`.
- **(e)** A string nua `trackfw guard git-branch` é aceita como `command`?

E comparar com a medição de 2026-09-05/06
(`docs/portabilidade/2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md`).

## Tabela resumida — 8 CLIs, questões (a)–(e)

| CLI | (a) Shell Windows | (b) Campo por plataforma/shell | (c) Exit que bloqueia (PreToolUse) | (d) cwd do hook | (e) String nua aceita? |
|---|---|---|---|---|---|
| Claude Code | Git Bash (se instalado), PowerShell (fallback) | SIM — campo `"shell": "bash"\|"powershell"` | exit 2 apenas; exit ≠ 2 = **fail-open** | dinâmico (segue `cd` do agente); `$CLAUDE_PROJECT_DIR` = raiz | SIM — shell form aceita qualquer comando no PATH |
| Codex | PowerShell (`pwsh.exe` → `powershell.exe`) | NÃO — `command` único, interpretado pelo shell detectado | exit 2 (ADR); exit ≠ 2 = **fail-open** | cwd da sessão no momento do hook | SIM — `pwsh -NoProfile -Command "trackfw guard git-branch"` |
| Gemini | PowerShell (`ComSpec` → `pwsh.exe` → `powershell.exe`) | NÃO — `command` único; expansão de vars em JS antes de invocar o shell | exit 2; PS wrapper propaga LASTEXITCODE; exit ≠ 2 = **fail-open** | `input.cwd` = raiz do projeto (passado explicitamente no spawn) | SIM — `powershell -Command "trackfw guard git-branch; if ($LASTEXITCODE -ne 0)…"` |
| Kiro | `cmd.exe` (default do Node.js `child_process`) | NÃO — campo `command` único | **QUALQUER exit ≠ 0 bloqueia** (fail-closed) | `t.cwd` = raiz do workspace | SIM — `cmd.exe /d /s /c trackfw guard git-branch` |
| Copilot | PowerShell (campo `powershell`) / bash (campo `bash`) / ambos (campo `command`) | SIM — `bash` (Unix), `powershell` (Windows), `command` (fallback cross-platform) | **QUALQUER exit ≠ 0 bloqueia** para preToolUse (exceto timeout) — **MUDANÇA desde 2026-09-05** | campo `cwd` na entrada do hook; trackfw emite `"cwd": "."` = raiz do repo | SIM — via campo `command` |
| Cursor | PowerShell (`pwsh` → `powershell.exe` → caminho fixo; lança exceção se nenhum existir) | NÃO — campo `command` único em `.cursor/hooks.json` | `beforeShellExecution`: exit 2 apenas; exit ≠ 2 = **fail-open** | não determinado (processo herda cwd do Cursor; confirmação pendente) | SIM — `$input \| trackfw guard git-branch` no wrapper PS |
| Windsurf | PowerShell via `powershell -Command` (campo `command` como fallback; campo `powershell` se definido) | SIM — `command` (macOS/Linux via `bash -c`, Windows via `powershell -Command`) e `powershell` (Windows via `powershell -Command`) — **NÃO investigado em 2026-09-05** | `pre_run_command`: exit 2 apenas; exit ≠ 2 = **fail-open** | raiz do repo (doc: "default is the root of the repo currently being worked on") | SIM — `powershell -Command "trackfw guard git-branch"` |
| Amazon Q | `cmd.exe /C` (Rust: `Command::new("cmd").arg("/C")`) — **NÃO investigado em 2026-09-05** | NÃO — campo `command` único | exit 2 apenas; exit ≠ 2 = **fail-open** | não determinado (Rust não chama `.current_dir()`; herda cwd do processo AmazonQ) | SIM — `cmd.exe /C trackfw guard git-branch` (resolve .exe/.cmd do PATH) |

**Fontes:**

| CLI | Fonte principal | URL | Data de leitura |
|---|---|---|---|
| Claude Code | Documentação oficial | https://docs.anthropic.com/en/docs/claude-code/hooks | 2026-10-04 |
| Codex | Código-fonte (`shell_detect.rs`, `shell.rs`, `session/mod.rs`) | https://github.com/openai/codex (branch `main`) | 2026-10-04 |
| Gemini | Código-fonte (`shell-utils.ts`, `hookRunner.ts`) | https://github.com/google-gemini/gemini-cli (branch `main`) | 2026-10-04 |
| Kiro | Bundle instalado (2026-09-06) + doc `kiro.dev/docs/hooks/actions/` | https://kiro.dev/docs/hooks/actions/ | 2026-10-04 (confirmação doc) |
| Copilot | Documentação oficial | https://docs.github.com/en/copilot/reference/hooks-reference | 2026-10-04 |
| Cursor | Bundle instalado Cursor 3.19.7 arm64 (2026-09-06) | — (bundle fechado) | 2026-09-06 (sem remedição disponível) |
| Windsurf | **Documentação oficial** — medida pela primeira vez aqui | https://docs.devin.ai/desktop/cascade/hooks | 2026-10-04 |
| Amazon Q | **Código-fonte Rust** — medido pela primeira vez aqui | https://github.com/aws/amazon-q-developer-cli (branch `main`) | 2026-10-04 |

---

## Seções por CLI

### 1. Claude Code

**Fonte:** https://docs.anthropic.com/en/docs/claude-code/hooks (2026-10-04)

**(a) Shell no Windows:**
Citação literal da doc:

> "Shell form runs when `args` is absent. The command string is passed to a shell: `sh -c` on macOS
> and Linux, **Git Bash on Windows**, or **PowerShell when Git Bash isn't installed.** Set the
> `shell` field to choose explicitly."

**(b) Campo por plataforma/shell:**
Campo `"shell"` disponível na entrada do hook:

> "`shell` no | Shell to use for this hook. Accepts `"bash"` or `"powershell"`. **Defaults to
> `"bash"`, or to `"powershell"` on Windows when Git Bash isn't installed.**"

`mergeClaudeHookArray` (`internal/generators/agentfiles.go`) emite apenas `{"type":"command","command":"..."}` — sem `"shell"` nem `"args"`. Logo o hook roda sempre em shell form com o default acima.

**(c) Exit code:**
Doc: "for most hook events, exit code 2 is the only exit code that blocks through the code alone. Without valid JSON on stdout, Claude Code treats **exit code 1 as a non-blocking error** and proceeds with the action." Exit ≠ 2 = **fail-open**.

**(d) cwd:**
Doc: "Handlers run in the **current directory** with Claude Code's environment. If the current directory no longer exists... Claude Code runs command hooks from the first of these that still exists: the directory the session started in, the project root, your home directory, or the system temp directory."

cwd é dinâmico, segue os `cd`s do agente. Para o guard encontrar `trackfw.yaml`, precisa subir a partir do cwd. `$CLAUDE_PROJECT_DIR` (env) está ancorado à raiz do projeto independentemente dos `cd`s.

**(e) String nua `trackfw guard git-branch`:**
Shell form interpreta a string como comando no shell do agente. `trackfw` precisa estar no PATH do processo. Aceita — sem aspas, sem caminho, sem variável.

**Comparação com 2026-09-05:** Sem mudança estrutural. O campo `"shell"` já existia. A mensagem de cwd dinâmico já estava documentada. A doc agora usa o domínio `docs.anthropic.com` em vez de `code.claude.com`, mas o conteúdo é o mesmo.

---

### 2. Codex CLI

**Fonte:** `github.com/openai/codex` branch `main` — `codex-rs/shell-command/src/shell_detect.rs`,
`codex-rs/core/src/shell.rs`, `codex-rs/core/src/session/mod.rs` — lidos em 2026-10-04.

**(a) Shell no Windows:**
`shell_detect.rs`, função `default_user_shell_from_path`:

```rust
pub fn default_user_shell_from_path(user_shell_path: Option<PathBuf>) -> DetectedShell {
    if cfg!(windows) {
        get_shell(ShellType::PowerShell).unwrap_or_else(ultimate_fallback_shell)
    }
    ...
}
```

Onde `get_shell(ShellType::PowerShell)` tenta `pwsh.exe` (PATH + caminho fixo
`C:\Program Files\PowerShell\7\pwsh.exe`), depois `powershell.exe` (PATH + caminho fixo
`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`). `ultimate_fallback_shell` só cai em
`cmd.exe` se nenhum PowerShell for encontrado — caso de borda.

**(b) Campo por plataforma:**
NÃO. `.codex/hooks.json` tem campo único `command`. O shell é determinado pela sessão.

**(c) Exit code:**
ADR-2026-09-05 adendo D7 confirma exit 2 = bloqueia. Código-fonte não adiciona lógica especial; a
semântica é a do bloco `build_hooks_config` que propaga o exit ao chamador. Exit ≠ 2 = **fail-open**.

**(d) cwd:**
`session/mod.rs`: `hook_shell_program` vem do `TurnEnvironment`; o spawn usa a cwd da sessão em
curso. O cwd exato no momento do hook (dinâmico vs. raiz) não foi determinado por leitura de código
— experimento mínimo: registrar `pwd` no stderr de um hook de teste em PowerShell.

**(e) String nua:**
PowerShell `-Command "trackfw guard git-branch"` resolve `trackfw` no PATH e passa `guard`
`git-branch` como argumentos posicionais. Aceita.

**Comparação com 2026-09-05:** Sem mudança de mecanismo. Confirmado que o caminho comum é
PowerShell, não `cmd.exe`. Único achado novo: `shell_detect.rs` tem constantes de caminhos fixos
(`PWSH_FALLBACK_PATHS`, `POWERSHELL_FALLBACK_PATHS`) que antes não haviam sido citadas — não mudam
a conclusão.

---

### 3. Gemini CLI

**Fonte:** `github.com/google-gemini/gemini-cli` branch `main` — `packages/core/src/utils/shell-utils.ts`
e `packages/core/src/hooks/hookRunner.ts` — lidos em 2026-10-04.

**(a) Shell no Windows:**
`getShellConfiguration()` atualizado desde 2026-09-05:

```ts
export function getShellConfiguration(): ShellConfiguration {
  if (isWindows()) {
    const powershellArgsPrefix = ['-NoProfile', '-NonInteractive', '-Command'];
    const comSpec = process.env['ComSpec'];
    if (comSpec) {
      const executable = comSpec.toLowerCase();
      if (executable.endsWith('powershell.exe') || executable.endsWith('pwsh.exe')) {
        return { executable: comSpec, argsPrefix: powershellArgsPrefix, shell: 'powershell' };
      }
    }
    const pwshPath = resolveExecutable('pwsh.exe');
    if (pwshPath) {
      return { executable: pwshPath, argsPrefix: ['-NoProfile', '-Command'], shell: 'powershell' };
    }
    return { executable: 'powershell.exe', argsPrefix: powershellArgsPrefix, shell: 'powershell' };
  }
  return { executable: 'bash', argsPrefix: ['-c'], shell: 'bash' };
}
```

Novo: verificação de `ComSpec` (se já apontar para PowerShell, usa diretamente). Resultado final
inalterado: sempre PowerShell no Windows.

**(b) Campo por plataforma:**
NÃO. `.gemini/settings.json` tem campo `command` único. Expansão de `$GEMINI_PROJECT_DIR` e
`$GEMINI_CWD` é feita em JavaScript pelo próprio runner **antes** de invocar o shell.

**(c) Exit code:**
Para PowerShell, `executeCommandHook` anexa ao comando:
```ts
command = `${command}; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }`;
```
Isso garante que o exit code do processo filho do comando seja propagado mesmo dentro do
`-Command`. Exit 2 = bloqueia. Exit ≠ 2 = **fail-open**.

Novo achado 2026-10-04: Gemini seta `CLAUDE_PROJECT_DIR: input.cwd` no env para compatibilidade.

**(d) cwd:**
`hookRunner.ts`, spawn:
```ts
const child = spawn(shellConfig.executable, [...shellConfig.argsPrefix, command], {
  env,
  cwd: input.cwd,   // ← EXPLICITAMENTE raiz do projeto
  ...
});
```
O processo do hook roda com cwd = `input.cwd` = raiz do projeto. `trackfw.yaml` é encontrado
imediatamente sem necessidade de subir.

**(e) String nua:**
`powershell -Command "trackfw guard git-branch; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }"`
— `trackfw` é resolúvel no PATH. Aceita.

**Comparação com 2026-09-05:** Adição da checagem de `ComSpec` — sem mudança de resultado. Novo
achado: `CLAUDE_PROJECT_DIR` setado no env. cwd explicitamente = raiz (confirmado no código, não
inferido).

---

### 4. Kiro

**Fonte:** Bundle instalado `kiro.kiro-agent` v1.0.794 (2026-09-06) — lido em sessão anterior.
Confirmação via documentação oficial `kiro.dev/docs/hooks/actions/` (2026-10-04).

**(a) Shell no Windows:**
`NodeProcessRunner.spawn` em `resources\app\extensions\kiro.kiro-agent\dist\extension.js`:
```js
n=(0,DIo.spawn)(t.command,{cwd:t.cwd,env:{...process.env,...t.env},shell:!0,...})
```
`child_process.spawn` com `{shell:true}` sem override explícito de shell → Node.js usa
`process.env.ComSpec` no Windows → `cmd.exe` por padrão.

**(b) Campo por plataforma:**
NÃO. Campo `command` único no schema `.kiro/hooks/*.json`.

**(c) Exit code:**
Documentação `kiro.dev/docs/hooks/actions/` (2026-10-04):

> "If the command returns any other exit code, the stderr output of the command is sent to the agent,
> and the agent is notified that the hook returned an error. Additionally, in the case of the Pre
> Tool Use hook, the tool invocation is blocked."

**QUALQUER exit ≠ 0 bloqueia para PreToolUse — fail-closed.** Esta era a conclusão de 2026-09-05
e é confirmada. Exit 1 (binário não encontrado) = **bloqueia**, não permite. Isso é **melhor para
segurança** que os CLIs que são fail-open.

**(d) cwd:**
`NodeProcessRunner.spawn`: `{cwd:t.cwd}` onde `t.cwd` vem do contexto de execução = raiz do
workspace. cwd setado explicitamente = raiz. `trackfw.yaml` encontrado imediatamente.

**(e) String nua:**
`cmd.exe /d /s /c trackfw guard git-branch` — `cmd.exe` resolve `trackfw.exe` ou `trackfw.cmd` no
PATH, com `guard git-branch` como argumentos. Aceita.

**Comparação com 2026-09-05:** Sem mudança de mecanismo. Achado (c) confirmado via doc oficial.

---

### 5. GitHub Copilot CLI

**Fonte:** https://docs.github.com/en/copilot/reference/hooks-reference (2026-10-04)

**(a) Shell no Windows:**
Campo `powershell` → `powershell -Command` (Windows).
Campo `bash` → bash (Unix).
Campo `command` → `bash -c` (Unix), `powershell -Command` (Windows, fallback).

Tabela da doc:

| `bash` set | `powershell` set | Windows | macOS/Linux |
|---|---|---|---|
| ✓ | ✗ | **hook ausente** (só `bash` populado, Windows lê `powershell` ou `command`) | bash |
| ✗ | ✓ | `powershell -Command` | hook silenciosamente pulado |
| ✓ | ✓ | `powershell -Command` | bash |
| ✗ | ✗ | `command` via `powershell -Command` | `command` via `bash -c` |

**Situação atual:** `InjectCopilotHooks` emite apenas `"bash": "scripts/..."` — **nenhuma entrada de
hook é lida no Windows** (campo `powershell` ausente, sem `command` de fallback). Achado idêntico ao
de 2026-09-05.

**ADR-2026-10-04 D4** resolve isso: usar campo `command` (cross-platform fallback). Com
`"command": "trackfw guard git-branch"`, Copilot roda via `bash -c` no macOS/Linux e via
`powershell -Command` no Windows — um campo cobre os dois.

**(b) Campo por plataforma:**
SIM — `bash`/`powershell`/`command` e também `exec`/`args` (exec form, novo em relação a 2026-09-05).

**(c) Exit code:**
**MUDANÇA SIGNIFICATIVA desde 2026-09-05:**

Citação doc (2026-10-04):

> "preToolUse command hooks are fail-closed on exit 2 and on non-timeout errors — **exit 2, a crash,
> or any other non-zero exit (other than a timeout) denies the tool call**, even if the hook's stdout
> JSON reports permissionDecision: 'allow'. Timeouts are always fail-open."

Em 2026-09-05, o único exit code de bloqueio documentado era exit 2. **Agora: QUALQUER exit ≠ 0
bloqueia para preToolUse (exceto timeout).** Esta é a mesma semântica do Kiro — fail-closed. Para
o risco D5 da ADR (binário velho → exit ≠ 2 → fail-open), **o Copilot está imune**: exit 1 de
binário sem o subcomando `guard` = bloqueia, não permite.

**(d) cwd:**
`InjectCopilotHooks` emite `"cwd": "."` em todas as entradas. A doc confirma que `cwd` é relativo
à raiz do repositório. cwd = raiz do repo. `trackfw.yaml` encontrado imediatamente.

**(e) String nua:**
Campo `command` com `"command": "trackfw guard git-branch"` → bash `trackfw guard git-branch`
(macOS/Linux) ou `powershell -Command "trackfw guard git-branch"` (Windows). Aceita.

**Comparação com 2026-09-05:** **EXIT CODE É A MAIOR MUDANÇA.** Anteriormente documentado como "exit
2 = block"; agora "qualquer exit não-zero = block para preToolUse". Isso não é regressão — é
fortalecimento da semântica de falha. O campo `exec`/`args` é novo; o achado do campo `bash`
ausente no Windows é idêntico.

---

### 6. Cursor

**Fonte:** Bundle instalado Cursor 3.19.7 arm64 (2026-09-06). Sem remedição disponível
(Cursor é fechado, sem repositório público). A medição anterior está documentada em
`docs/portabilidade/2026-09-05-...` seção 5.

**(a) Shell no Windows:**
`sL()` no bundle resolve em ordem: `pwsh` no PATH, `powershell` no PATH, caminho fixo
`%SYSTEMROOT%\System32\WindowsPowerShell\v1.0\powershell.exe`. Se nenhum existir, lança exceção
(`throw new Error("Neither 'pwsh' nor 'powershell' found")`). Nunca `cmd.exe`, nunca bash.

**(b) Campo por plataforma:**
NÃO. `.cursor/hooks.json` tem campo `command` único (além de eventos diferentes por tipo de hook:
`beforeShellExecution`, `afterShellExecution`, `preToolUse`, `postToolUse`).

**(c) Exit code:**
`beforeShellExecution` (hook de execução de shell):

> "exit 0 uses the JSON output (or defaults to allow if stdout has none); exit 2 blocks the action
> ('equivalent to returning permission: \"deny\"'); any other exit code fail-opens (hook failed,
> action proceeds)."

Exit 2 = bloqueia. Exit ≠ 2 = **fail-open**. Para ADR D5: binário velho → exit ≠ 2 → **fail-open**.

**(d) cwd:**
**Não determinado.** `NaiveTerminalExecutor` (bundle) não passa `.cwd()` explicitamente ao spawn.
O processo herda o cwd do processo Cursor (normalmente a raiz do projeto onde o Cursor foi aberto).
Experimento mínimo para fechar: registrar `pwd` em um hook de teste no Cursor em Windows.

**(e) String nua:**
No `beforeShellExecution`, o `command` é interpolado em um wrapper PowerShell:
`$OutputEncoding = [...UTF8]; Get-Content -LiteralPath '<tmpfile>' -Raw | & { $input | <command> }`

Para `trackfw guard git-branch`:
`... | & { $input | trackfw guard git-branch }`

PowerShell resolve `trackfw` como executável no PATH e passa `guard git-branch` como argumentos. Aceita.

**Comparação com 2026-09-05:** Sem mudança (bundle não atualizado desde 2026-09-06). cwd ainda não
determinado — único residual.

---

### 7. Windsurf

**Fonte:** https://docs.devin.ai/desktop/cascade/hooks (2026-10-04) — **medido pela primeira vez
neste documento**. Windsurf não estava na tabela de 6 CLIs de 2026-09-05.

**(a) Shell no Windows:**
Tabela "Cross-Platform Behavior" da doc:

| `command` set | `powershell` set | Windows | macOS/Linux |
|---|---|---|---|
| ✓ | ✗ | `command` via `powershell -Command` | `command` via `bash -c` |
| ✗ | ✓ | `powershell` via `powershell -Command` | hook silenciosamente pulado |
| ✓ | ✓ | `powershell` via `powershell -Command` | `command` via `bash -c` |
| ✗ | ✗ | erro de validação | erro de validação |

**Situação atual:** `InjectWindsurfHooks` emite
`{"command": "bash scripts/trackfw-git-branch-guard.sh", "show_output": true}` — apenas campo
`command`. No Windows, cai no caso "✓ ✗": `powershell -Command "bash scripts/trackfw-git-branch-guard.sh"`.
O `bash` só existe se Git Bash estiver no PATH — mesmo condicional do Claude Code.

**(b) Campo por plataforma:**
SIM — `command` (cross-platform, `bash -c` no macOS/Linux, `powershell -Command` no Windows como
fallback) e `powershell` (Windows explícito). Windsurf tem o mesmo esquema de dois campos que o
Copilot — diferença: Windsurf não tem campo `bash` separado (usa `command` para Unix).

**(c) Exit code:**
Doc:

> "For pre-hooks (executed before an action), your script can **block the action by exiting with
> exit code 2**."

Exit 2 = bloqueia. Exit ≠ 2 = **fail-open** (não documentado como bloqueante). Para ADR D5: fail-open.

**(d) cwd:**
Doc:

> "About the working_directory parameter: In multi-repo workspaces, **the default is the root of
> the repo currently being worked on**. Relative paths resolve from the default location (workspace
> or repo root)."

O JSON de entrada de `pre_run_command` inclui `tool_info.cwd` com o cwd explícito. cwd = raiz do
repo. `trackfw.yaml` encontrado imediatamente.

**(e) String nua:**
`"command": "trackfw guard git-branch"` → no Windows, `powershell -Command "trackfw guard git-branch"`.
`trackfw` resolvido no PATH. Aceita. Sem caminho, sem variável.

**Comparação com 2026-09-05:** Windsurf não estava na medição anterior. Este é o primeiro contrato
documentado. Achados: schema `command`/`powershell` (como Copilot); exit 2 = block; cwd = raiz do repo.
A string nua `trackfw guard git-branch` funciona via campo `command` cross-platform.

---

### 8. Amazon Q Developer CLI

**Fonte:** `github.com/aws/amazon-q-developer-cli` branch `main` —
`crates/chat-cli/src/cli/agent/hook.rs`,
`crates/chat-cli/src/cli/chat/cli/hooks.rs`,
`docs/hooks.md` — lidos em 2026-10-04. **Medido pela primeira vez neste documento.**

**(a) Shell no Windows:**
`hooks.rs`, função `run_hook`:

```rust
#[cfg(unix)]
let mut cmd = tokio::process::Command::new("bash");
#[cfg(unix)]
let cmd = cmd.arg("-c").arg(command)...;

#[cfg(windows)]
let mut cmd = tokio::process::Command::new("cmd");
#[cfg(windows)]
let cmd = cmd.arg("/C").arg(command)...;
```

No Windows: `cmd.exe /C <command>`. No Unix: `bash -c <command>`.

**(b) Campo por plataforma:**
NÃO. `hook.rs`: schema do hook tem apenas `command` (string), `timeout_ms`, `max_output_size`,
`cache_ttl_seconds` e `matcher` (optional glob). Campo único sem bifurcação por plataforma.

**(c) Exit code:**
`docs/hooks.md`:

> "**Exit code 0**: Allow tool execution.
> **Exit code 2**: Block tool execution, return STDERR to LLM.
> **Other exit codes**: Hook failed. STDERR is shown as warning to user, **allow tool execution**."

Exit 2 = bloqueia. Exit ≠ 2 = **fail-open**. Para ADR D5: fail-open.

**(d) cwd:**
**Não determinado.** `run_hook` em `hooks.rs` não chama `.current_dir()` — o processo herda o cwd
do Amazon Q CLI no momento do lançamento. O campo `"cwd"` é enviado no JSON do stdin (para o script
ler), mas **NÃO é setado como o cwd do processo filho**. Isso significa que caminhos relativos no
`command` são resolvidos a partir do cwd de lançamento do Q CLI, não necessariamente da raiz do
projeto. Experimento mínimo: `cmd /C cd` em um hook de teste registra o cwd real do processo.

**(e) String nua:**
`cmd.exe /C trackfw guard git-branch` → `cmd.exe` resolve `trackfw.exe` ou `trackfw.cmd` no PATH
e passa `guard git-branch` como argumentos posicionais. Aceita.

**Comparação com 2026-09-05:** Amazon Q não estava na medição anterior. Achados: shell é `cmd.exe /C`
no Windows (mesmo mecanismo que o Kiro herdava do Node.js, mas aqui é uma escolha explícita em Rust).
cwd NÃO setado explicitamente no processo filho — residual a confirmar.

---

## Comparação sistemática com 2026-09-05/06

| CLI | Mudança desde 2026-09-05 | Fonte |
|---|---|---|
| Claude Code | Sem mudança de mecanismo. Domínio doc mudou de `code.claude.com` para `docs.anthropic.com`. | docs.anthropic.com/en/docs/claude-code/hooks |
| Codex | Sem mudança de mecanismo. Confirmadas constantes de fallback path (`PWSH_FALLBACK_PATHS`). | código-fonte |
| Gemini | Adição de checagem `ComSpec` antes de buscar PowerShell no PATH. Resultado final: sem mudança. Confirmado: cwd setado explicitamente = raiz. `CLAUDE_PROJECT_DIR` setado no env (novo). | código-fonte |
| Kiro | Sem mudança. exit code confirmado via doc oficial. | kiro.dev/docs/hooks/actions/ |
| Copilot | **MUDANÇA SIGNIFICATIVA: preToolUse agora é fail-closed para qualquer exit ≠ 0 (exceto timeout).** Antes: exit 2 apenas. Novo `exec`/`args` form documentado. | docs.github.com |
| Cursor | Sem remedição disponível (bundle fechado). cwd ainda não determinado. | — |
| Windsurf | **PRIMEIRA MEDIÇÃO.** Schema `command`/`powershell`; exit 2 = block; cwd = raiz. | docs.devin.ai |
| Amazon Q | **PRIMEIRA MEDIÇÃO.** `cmd.exe /C` no Windows; exit 2 = block; cwd não setado no processo. | código-fonte |

---

## Implicações para o desenho da ADR-2026-10-04

### Implicação 1 — Exit ≠ 2 = fail-open em 6 de 8 CLIs (D5 item 2 validado)

A ADR-2026-10-04 D5 item 2 identifica como risco: "binário velho → falha aberta". Medição confirma:

- **Fail-open** (exit ≠ 2 = permite): Claude Code, Codex, Gemini, Cursor, Windsurf, Amazon Q — 6 de 8.
- **Fail-closed** (qualquer exit ≠ 0 bloqueia para PreToolUse): Kiro, Copilot — 2 de 8.

O risco da ADR é real para a maioria dos CLIs. A mitigação proposta ("validate precisa denunciar a
versão incompatível") cobre todos os 8; o "avisar alto" é necessário especialmente para os 6
fail-open.

### Implicação 2 — A string nua `trackfw guard git-branch` é aceita por todos os 8 CLIs

Sem exceção. Nenhum CLI exige caminho, variável ou aspas. O único requisito é que `trackfw` esteja
no PATH do processo do hook. A D2 da ADR ("linha de hook é a mesma string em todo shell") é
verificável — nenhum CLI impede o uso da string nua.

### Implicação 3 — cwd para `trackfw.yaml`: determinado em 6, residual em 2

| Status | CLIs |
|---|---|
| cwd = raiz do projeto (determinado) | Gemini, Kiro, Windsurf, Copilot (`cwd: "."`) |
| cwd = dinâmico/raiz (funciona com walk-up) | Claude Code |
| cwd não determinado (processo filho herda cwd do CLI) | Cursor, Amazon Q |

Para Cursor e Amazon Q, a string nua `trackfw guard git-branch` ainda funciona (o guard encontra o
`trackfw` no PATH), mas o walk-up para encontrar `trackfw.yaml` depende de onde o CLI foi lançado.
Experimento mínimo: registrar `pwd` em um hook de teste nos dois CLIs para confirmar.

### Implicação 4 — Windsurf exige campo `command`, não `bash`

A emissão atual (`InjectWindsurfHooks`) usa `"command": "bash scripts/trackfw-git-branch-guard.sh"`.
Com o novo desenho, o valor muda para `"command": "trackfw guard git-branch"` — e a string funciona
cross-platform sem campo `powershell` separado (o campo `command` cobre macOS via `bash -c` e Windows
via `powershell -Command`).

A doc de Windsurf confirma: se `powershell` field estiver ausente e só `command` estiver setado, no
Windows o CLI usa `powershell -Command <command>`. Com `trackfw guard git-branch` como valor, isso
funciona se `trackfw` estiver no PATH — independente de Git Bash.

### Implicação 5 — Copilot preToolUse tornou-se fail-closed (novidade)

A mudança no Copilot (qualquer exit ≠ 0 = bloqueia para preToolUse) MELHORA a postura de segurança
sem exigir nenhuma mudança no guard. Um `trackfw` sem subcomando `guard` sai com exit 1 (não 2) →
bloqueia a tool call. Isso NÃO contradiz a ADR, apenas reduz o impacto do risco D5 item 2 para o
Copilot.

### Implicação 6 — Amazon Q usa `cmd.exe /C` (não PowerShell), não requer script `.ps1`

O guard `trackfw guard git-branch` executado via `cmd.exe /C trackfw guard git-branch` funciona
corretamente se `trackfw.exe` ou `trackfw.cmd` estiver no PATH. Não há necessidade de um shim
PowerShell. A ADR está correta ao não distinguir Amazon Q dos demais — a string nua funciona.

### Itens não contradizem a ADR

Nenhum dos 8 CLIs impede o uso da string `trackfw guard git-branch`. O desenho da ADR-2026-10-04 é
compatível com todos os contratos medidos. Os riscos D5 continuam abertos e são gerenciados pelas
mitigações propostas (validate, versão mínima).

---

## Residuais declarados

| Residual | Impacto | Experimento mínimo |
|---|---|---|
| cwd do processo de hook no Cursor | Se o Cursor for aberto de um diretório diferente da raiz do repo, o walk-up para `trackfw.yaml` pode falhar. | Registrar `pwd` (ou `%CD%` via `cmd.exe /C echo %CD%`) em hook de `beforeShellExecution` no Windows. |
| cwd do processo de hook no Amazon Q | Mesmo que o Cursor — Q CLI pode ter sido lançado de qualquer diretório. | Mesmo procedimento: `cmd /C cd` em hook de teste. |
| Cursor: sem remedição do bundle | Cursor 3.19.7 era a versão medida em 2026-09-06. Se houve atualização, o mecanismo pode ter mudado. | Instalar a versão atual do Cursor na VM e reler o bundle. |
| Copilot: exit code do `exec` form | O `exec`/`args` form foi mencionado na doc; seu comportamento de exit code não foi verificado. | Não relevante para o desenho atual (o trackfw não usa `exec` form). |
