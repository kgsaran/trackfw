---
title: Contrato de execução do "command" de hook por CLI de agente no Windows — qual shell interpreta o `.sh`
date: 2026-09-05
author: prometeu-tf (investigação pura — nenhuma correção aplicada)
status: investigação concluída
---

# Contrato de execução de hook por CLI de agente no Windows

> 🔴 Documento de investigação pura. Nenhum código foi alterado, nenhum hook foi regenerado,
> nenhuma operação de git foi executada. `internal/generators/agentfiles.go` e os geradores
> Node/Python não foram tocados.

## A pergunta e por que ela bifurca o trabalho

O trackfw emite, em cada config de hook de projeto, um `command` que referencia
`scripts/trackfw-credential-guard.sh` (ou `trackfw-git-branch-guard.sh`, `trackfw-attention-*.sh`) —
um script **bash**. Quem interpreta essa string é o CLI do agente, não o trackfw. A pergunta:
**no Windows, por qual shell cada CLI passa o `command`, por padrão, sem configuração adicional do
usuário?** Se for `sh`/Git Bash, o `.sh` roda. Se não for, o guard nunca dispara.

O runner de CI deste repositório é macOS; nenhum dos 6 CLIs está instalado nele. Este documento
separa, CLI a CLI, o que foi **medido** (execução real observada ou leitura direta do
código-fonte/bundle instalado), o que é **documentado pelo fornecedor** (fonte primária lida hoje,
com citação literal) e o que é **inferido** — e nomeia o experimento mínimo para fechar cada lacuna.

> **Atualização de 2026-09-06**: as duas lacunas que restavam (Cursor, Kiro) foram fechadas por
> leitura do bundle Electron/Node instalado numa VM Windows ARM64 (build `10.0.26200`) provisionada
> só para esta investigação, sem autenticar nenhuma conta dos dois produtos — ver seções 5 e 6 e o
> "Fecho" atualizado. Nenhum dos 6 CLIs permanece indeterminado nesta pergunta.

## Enumeração dos 6 CLIs (a partir do código, não de memória)

`internal/validator/validator_credential_guard.go`, `credentialGuardHookFiles` — lista fechada dos
arquivos de hook de projeto que o trackfw gera e que o validador entende:

| CLI | Arquivo de hook | `requiresVarOrShellPrefix` |
|---|---|---|
| Claude Code | `.claude/settings.json` | true |
| Codex CLI | `.codex/hooks.json` | true |
| Gemini CLI | `.gemini/settings.json` | true |
| Cursor | `.cursor/hooks.json` | false |
| GitHub Copilot CLI | `.github/hooks/trackfw-attention.json` | false |
| Kiro | `.kiro/hooks/trackfw-attention.json` | false |

`requiresVarOrShellPrefix=false` marca os CLIs cujo caminho relativo puro já é a forma correta
(cwd fixo na raiz do projeto, `docs/cli-parity.md` §"Mecanismo de resolução...") — isso é ortogonal
à pergunta deste documento: mesmo um caminho corretamente ancorado ainda precisa de um shell
POSIX-capaz para interpretar o shebang `#!/bin/bash`/`#!/usr/bin/env bash` do `.sh`.

## Tabela por CLI — grau de certeza explícito

| CLI | Shell no Windows (sem config extra) | Grau de certeza | `.sh` do trackfw roda? |
|---|---|---|---|
| Claude Code | Git Bash, se instalado; senão PowerShell (`powershell.exe`/`pwsh.exe`) | **Documentado pelo fornecedor** | Sim, se Git Bash presente. Não, se ausente — dupla falha documentada: `$CLAUDE_PROJECT_DIR` sem chaves não é reescrito para `$env:`, **e** PowerShell não interpreta shebang |
| Codex CLI | PowerShell (`pwsh.exe`/`powershell.exe`) no caminho comum (sessão com `TurnEnvironment` já resolvido — cobre `PreToolUse`/`PostToolUse`, onde os guards disparam); `cmd.exe` só num fallback de borda (sem `TurnEnvironment`/shell resolvido, ex. hooks muito cedo no ciclo de vida) | **Medido no código-fonte**, 3 arquivos (`session/mod.rs`, `shell.rs`, `shell_detect.rs`, branch `main`, lidos hoje) — corrigido após revisão: leitura inicial só de `command_runner.rs` generalizava o fallback de borda como caminho padrão | **Não** — mesma classe de falha do Gemini: a sintaxe `$(...)` provavelmente resolve (coincide com PowerShell), mas PowerShell não interpreta o shebang do `.sh` |
| Gemini CLI | PowerShell (`pwsh.exe` se no PATH, senão `powershell.exe`) — nunca `cmd`/bash | **Medido no código-fonte** (`packages/core/src/utils/shell-utils.ts`, branch `main`, lido hoje) | **Não** — PowerShell não interpreta shebang; a substituição de `$GEMINI_PROJECT_DIR` funciona (é feita por regex no próprio Gemini antes de invocar o shell), mas o `.sh` em si nunca executa |
| Cursor | `pwsh.exe` se presente no PATH, senão `powershell.exe` — nunca `cmd.exe`, nunca bash. O `command` do hook é injetado dentro de um script PowerShell gerado internamente (`$OutputEncoding=[...UTF8]; Get-Content ... \| & { $input \| <command> }`), executado com `-NoProfile -NonInteractive -ExecutionPolicy Bypass` | **Medido no bundle instalado** (Cursor 3.19.7 arm64 para Windows, `resources\app\out\vs\workbench\workbench.desktop.main.js` + `resources\app\out\vs\workbench\api\node\extensionHostProcess.js`, lidos hoje via VM Windows real, binário oficial via winget `Anysphere.Cursor`) | **Não** — mesma classe de falha de Codex/Gemini: PowerShell não interpreta shebang, e como o `command` do trackfw é o caminho bare `scripts/trackfw-credential-guard.sh` (sem prefixo `bash`), o operador `&` do PowerShell tenta invocar o `.sh` diretamente e falha (extensão sem cmdlet/executável associado) |
| GitHub Copilot CLI | Depende de qual campo do JSON está populado: `bash` (Unix) vs `powershell` (Windows) vs `command` (fallback cross-platform, copiado para os dois) | **Documentado pelo fornecedor** — e a config atual do trackfw (`InjectCopilotHooks`, só popula `bash`) está **confirmada por leitura de código**, não por doc | **Não, e por um motivo diferente dos outros**: o hook nem é lido no Windows local (ver achado 1) |
| Kiro | `cmd.exe` (via `process.env.ComSpec`, default do Node.js quando `shell: true` sem override explícito) — nunca PowerShell, nunca bash | **Medido no bundle instalado** (Kiro 1.0.437 / extensão `kiro.kiro-agent` v1.0.794, `resources\app\extensions\kiro.kiro-agent\dist\extension.js`, lido hoje via VM Windows real, binário oficial via winget `Amazon.Kiro`) | **Não** — `cmd.exe` não interpreta shebang nem tem associação de arquivo para `.sh` por padrão; o `command` do trackfw (`scripts/trackfw-credential-guard.sh`, sem prefixo) falha com "não é reconhecido como um comando interno" |

Achado que se repete em 4 dos 6 CLIs (Claude Code sem Git Bash, Codex no caminho comum, Gemini
sempre, Cursor sempre): **PowerShell é o shell mais comum no Windows para hooks de agente**, não
`cmd.exe` — e nenhum dos quatro interpreta shebang de `.sh`. Kiro é o único dos 6 medido/documentado
que usa `cmd.exe` como caminho comum (via default do Node.js `child_process`, não por escolha
explícita do fornecedor) — e `cmd.exe` também não interpreta shebang nem tem associação de arquivo
para `.sh`. Resultado prático: dos 6 CLIs, **nenhum** roda o `.sh` do trackfw no Windows sem alguma
mudança — a única variável é a classe de falha (PowerShell sem shebang vs `cmd.exe` sem shebang vs
campo de config ausente no Copilot).

## Achados por CLI (evidência + citação)

### 1. GitHub Copilot CLI — o achado mais grave: campo errado, não shell errado

Fonte: <https://docs.github.com/en/copilot/reference/hooks-reference>, lida hoje (2026-09-05).
Citação literal do schema de um "Command hook":

> `bash` string | One of `bash`, `powershell`, or `command`, unless `exec` is specified | **Shell
> command for Unix.**
> `powershell` string | ... | **Shell command for Windows.**
> `command` string | ... | **Cross-platform fallback. Copied to both `bash` and `powershell` when
> those fields are absent; explicit `bash` or `powershell` entries take precedence on their
> respective platforms.**

`internal/generators/agentfiles.go` (`InjectCopilotHooks`, linhas 852–930, confirmado por leitura de
código, não por doc) emite **só** o campo `"bash"` em toda entrada, nunca `"command"` nem
`"powershell"`:

```go
preToolUse := []interface{}{
    map[string]interface{}{
        "type":       "command",
        "bash":       "scripts/trackfw-attention-signal.sh",
        "cwd":        ".",
        "timeoutSec": 10,
    },
}
...
preToolUse = append(preToolUse, map[string]interface{}{
    "type":       "command",
    "matcher":    "bash",
    "bash":       "scripts/trackfw-credential-guard.sh",
    "cwd":        ".",
    "timeoutSec": 10,
})
```

Confirmado para as 8 entradas do arquivo (signal, cleanup, credential-guard × 3 matchers ×
pre/post, git-branch-guard): nenhuma carrega `"powershell"` ou `"command"` como chave irmã de
`"bash"`.

🔴 Pela própria semântica documentada do schema, isso significa que **no Copilot CLI local no
Windows, este hook nunca é lido** — o CLI olha o campo `powershell`, que está ausente, e não há
`command` de fallback para copiar. Isso independe de qual shell o Windows tem instalado: **é
vácuo de configuração, não incompatibilidade de shell**. A correção óbvia — trocar `"bash"` por
`"command"` — sai do escopo desta investigação (é implementação), mas é o achado mais acionável
do documento inteiro, porque não depende de nenhuma medição em Windows real para ser verdadeiro:
é uma leitura direta de schema documentado contra uma leitura direta de código gerador.

Nota à parte, sem impacto na conclusão acima: a seção de cloud agent do mesmo doc ("Cloud agent
execution environment") diz que lá "Only the `bash` field on command hooks is honored; `powershell`
entries are ignored" — mas cloud agent roda num sandbox Linux efêmero, fora do escopo Windows
deste documento; citado só para não confundir as duas superfícies.

### 2. Codex CLI — medido no código-fonte: PowerShell no caso comum, `cmd.exe` só num fallback de
borda — **achado corrigido após revisão do advisor** (a primeira leitura, só de
`command_runner.rs`, generalizava um fallback de borda como se fosse o caminho padrão)

Fonte: `github.com/openai/codex`, branch `main`, lido hoje via `raw.githubusercontent.com`
(repositório público, licença aberta — não é doc do fornecedor, é leitura de implementação).
Três arquivos, seguindo a cadeia de construção real:

**a) `codex-rs/core/src/session/mod.rs`, `build_hooks_config`** — o shell de hook **não** é sempre
o fallback de `command_runner.rs`; ele vem primeiro do `TurnEnvironment` da sessão em curso, se
existir:

```rust
let (hook_shell_program, hook_shell_argv) = environment
    .and_then(|environment| environment.shell.as_ref())
    .map(|shell| {
        let mut argv = shell.derive_exec_args("", /*use_login_shell*/ false);
        let program = argv.remove(0);
        let _ = argv.pop();
        (Some(program), argv)
    })
    .unwrap_or_default();
```

**b) `codex-rs/core/src/shell.rs`, `derive_exec_args`** — para `ShellType::PowerShell`, monta
`[shell_path, "-NoProfile", "-Command", <command>]`; para `ShellType::Cmd`, `[shell_path, "/c",
<command>]`. Qual dos dois é o `environment.shell` da sessão vem de `default_user_shell()`.

**c) `codex-rs/shell-command/src/shell_detect.rs`, `default_user_shell_from_path`** — a resposta
para "qual shell a sessão usa por padrão no Windows":

```rust
pub fn default_user_shell_from_path(user_shell_path: Option<PathBuf>) -> DetectedShell {
    if cfg!(windows) {
        get_shell(ShellType::PowerShell).unwrap_or_else(ultimate_fallback_shell)
    }
    ...
}
```

**No Windows, o shell padrão de uma sessão Codex é PowerShell** (`pwsh.exe` se presente, senão
`powershell.exe`) — não `cmd.exe`. `ultimate_fallback_shell()` só cai para `cmd.exe` se a própria
detecção de PowerShell falhar totalmente (nenhum `pwsh`/`powershell` resolvível), o que é o caso de
borda, não o caminho comum. O `cmd.exe`/`/C` de `command_runner.rs::default_shell_command` (achado
da leitura anterior deste documento) só é alcançado quando `shell.program` está vazio, isto é,
quando `environment` é `None` ou `environment.shell` é `None` — plausível para hooks muito cedo no
ciclo de vida (ex.: `SessionStart` antes de qualquer `TurnEnvironment` resolver), mas **não** para
`PreToolUse`/`PostToolUse`, que é onde `credential-guard`/`git-branch-guard` disparam — esses eventos
ocorrem em turno já em curso, com `TurnEnvironment` necessariamente presente.

Consequência para o `command` que o trackfw emite —
`"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"` — passado como argumento
de `-Command` a `pwsh.exe`/`powershell.exe`:

- A sintaxe `$(...)` **coincide** entre bash e PowerShell (subexpression operator, também expande
  dentro de string entre aspas duplas) — diferente do caso de `$VAR` puro (que diverge:
  `$env:VAR` vs `$VAR`), aqui a substituição de comando provavelmente resolve corretamente **se**
  `git` estiver no `PATH` do processo — isto não foi confirmado empiricamente, só é plausível pela
  sintaxe.
- Mesmo com o caminho resolvendo para o `.sh` correto, **PowerShell não interpreta shebang** —
  mesma classe de falha final do Gemini CLI (seção 3), não a classe "sintaxe de shell errada desde
  o primeiro caractere" que a leitura anterior (só `command_runner.rs`) atribuía ao Codex.

Grau de certeza: **medido no código-fonte atual do fornecedor**, mas com uma lacuna empírica que a
leitura de código não fecha: não foi confirmado que `environment.shell` está sempre `Some` no
momento em que `PreToolUse`/`PostToolUse` disparam (só que a arquitetura do código torna isso
altamente provável). O experimento mínimo abaixo fecha essa lacuna.

### 3. Gemini CLI — medido no código-fonte: sempre PowerShell, nunca bash/cmd

Fonte: `github.com/google-gemini/gemini-cli`, branch `main`,
`packages/core/src/utils/shell-utils.ts` (`getShellConfiguration`) e
`packages/core/src/hooks/hookRunner.ts` (`executeCommandHook`, `expandCommand`), lidos hoje.

```ts
export function getShellConfiguration(): ShellConfiguration {
  if (isWindows()) {
    // ... prefere pwsh.exe, cai para powershell.exe ...
    return { executable: pwshOuPowershellExe, argsPrefix: [...], shell: 'powershell' };
  }
  return { executable: 'bash', argsPrefix: ['-c'], shell: 'bash' };
}
```

`executeCommandHook` chama `spawn(shellConfig.executable, [...shellConfig.argsPrefix, command], {
shell: false, ... })` — ou seja, **nunca há `cmd.exe` na trajetória do Gemini CLI**: é bash em
Unix, é sempre PowerShell (`pwsh.exe` se estiver no `PATH`, senão `powershell.exe`) no Windows, sem
opção de fallback para `cmd`.

Achado secundário relevante: **a substituição de `$GEMINI_PROJECT_DIR` não depende do shell** —
`expandCommand` faz `command.replace(/\$GEMINI_PROJECT_DIR/g, () => escapedCwd)` em JavaScript,
**antes** de passar a string para o `spawn`. Isso significa que a preocupação de
`docs/cli-parity.md` sobre `$GEMINI_PROJECT_DIR` não expandir dentro de uma sintaxe PowerShell
(`$VAR` vs `$env:VAR`) **não se aplica** — o Gemini já resolveu isso substituindo a string
literalmente antes do shell nem ver o placeholder. O que resta quebrado é só o `.sh` em si:
PowerShell não interpreta shebang (`#!/bin/bash`), então mesmo com o caminho corretamente resolvido
para `C:\...\scripts\trackfw-credential-guard.sh`, `powershell.exe -Command "C:\...\trackfw-credential-guard.sh"`
falha (arquivo não reconhecido como cmdlet/script executável nativamente, a menos que o usuário
tenha uma associação de extensão `.sh` configurada — não é o padrão de nenhuma instalação Windows).

Grau de certeza: **medido no código-fonte atual do fornecedor**, mesmo patamar do Codex.

### 4. Claude Code — o único caso documentado pelo próprio fornecedor com a granularidade certa

Fonte: <https://code.claude.com/docs/en/hooks>, lida hoje. Duas citações decisivas:

> "Shell form runs when `args` is absent. The command string is passed to a shell: **`sh -c` on
> macOS and Linux, Git Bash on Windows, or PowerShell when Git Bash isn't installed.** Set the
> `shell` field to choose explicitly."

> "`shell` no | Shell to use for this hook. Accepts `"bash"` or `"powershell"`. **Defaults to
> `"bash"`, or to `"powershell"` on Windows when Git Bash isn't installed.**"

`internal/generators/agentfiles.go` (`mergeClaudeHookArray`, confirmado por leitura de código) só
emite `{"type": "command", "command": "..."}` — nunca `"args"`, nunca `"shell"`. Logo o hook do
trackfw roda sempre em **shell form**, e o comportamento no Windows é exatamente o default
documentado acima: **Git Bash, se instalado; PowerShell, se não**.

Isso é o inverso de Codex e Gemini: Claude Code é o **único dos 6** cuja doc primária confirma, com
frase explícita e recente (a doc cita comportamento de versão ≥ 2.1.198, então é atual), que o
caminho feliz no Windows passa por um shell POSIX-capaz — **condicionado a Git Bash estar
instalado**. Sem Git Bash, cai no mesmo problema de Gemini/Codex: PowerShell não interpreta o
shebang do `.sh`.

O que parecia um residual de doc some ao olhar o que o trackfw realmente escreve. A doc tem uma
segunda frase, sobre a forma **sem chaves** (`$CLAUDE_PROJECT_DIR`, não `${CLAUDE_PROJECT_DIR}`):

> "Don't write the bare `$CLAUDE_PROJECT_DIR` spelling in a PowerShell hook. PowerShell parses it
> as an undefined local variable and resolves it to `$null`, which leaves the script path without
> its project-root prefix. **Claude Code doesn't rewrite that form; it logs a warning in the debug
> log instead.**"

`mergeClaudeHookArray`/`claudeGitGuardCmd` (`internal/generators/agentfiles.go`, confirmado por
leitura de código) emitem exatamente essa forma **sem chaves**: `"$CLAUDE_PROJECT_DIR/scripts/..."`,
nunca `"${CLAUDE_PROJECT_DIR}/scripts/..."`. A pergunta sobre reescrita condicionada a `"shell":
"powershell"` explícito fica **moot** — não se aplica ao formato que o trackfw gera, doc ou não. No
fallback implícito (Git Bash ausente → PowerShell automático), a falha é **dupla e documentada**,
sem ambiguidade: (1) `$CLAUDE_PROJECT_DIR` sem chaves não é reescrito, resolve a `$null`, o caminho
perde o prefixo da raiz do projeto; e (2), mesmo que o caminho estivesse correto, PowerShell não
interpreta o shebang do `.sh`. Grau de certeza: **documentado pelo fornecedor**, sem residual —
rebaixado de "ambiguidade residual" após conferir contra a string exata que o gerador emite.

### 5. Cursor — medido no bundle instalado (Electron/Node): sempre PowerShell, nunca `cmd`/bash

`cursor.com/docs/hooks`, lido hoje, não menciona shell de execução no Windows em lugar nenhum
(buscado por "cmd.exe", "PowerShell", "Git Bash" — zero ocorrência). Cursor é fornecedor fechado,
sem repositório público equivalente ao do Codex/Gemini. A lacuna foi fechada **sem doc e sem
autenticação**, lendo o artefato instalado: Cursor é um fork de VS Code (Electron), e a pergunta
"qual shell dispara o `command` do hook" é respondida diretamente pelo bundle JS que o instalador
grava em disco — o mesmo raciocínio que resolveu Gemini/Codex por código-fonte, aplicado a um
binário fechado em vez de um repositório aberto.

**Procedimento**: `winget install --id Anysphere.Cursor --silent --accept-package-agreements
--accept-source-agreements` numa VM Windows ARM64 limpa (build `10.0.26200`), sem login/conta —
instala **Cursor 3.19.7 (arm64)**. Nenhuma sessão autenticada foi aberta; a leitura é só do bundle
em disco (`resources\app\out\...`, texto legível, não minificado ao ponto de perder strings
literais). Dois arquivos, seguindo a cadeia de execução real:

**a) `resources\app\out\vs\workbench\workbench.desktop.main.js`** (renderer, DI container) —
a classe do serviço de hooks (`CBo`, registrada como implementação do símbolo `E7`) resolve o
comando e delega a um `shellExecService` injetado:

```js
async _executeCommandHookScript(e,t,n,i,r,s,o,a,c,l,u){
  ...
  const v=await this._getBackendOS(),_=JSON.stringify(r);let y,C,k;
  v===1?(C=_,y=f,k="windows_temp_file"):(C=_,y=f,k="stdin");
  ...
  const P=await this.shellExecService.executeHookDirect(y,p,v===1,x,I,C),
  ...
```

`f` é `t.command` (o `command` bruto do `hooks.json`, sem transformação — só troca de placeholders
`${CLAUDE_PLUGIN_ROOT}`, que o trackfw não usa). `v===1` é o discriminante Windows: confirmado pelo
mesmo enum usado em `getEnterpriseConfigDirectory`/`resolveEnterpriseConfigDirectoryAndPath` no
mesmo arquivo (`case 1:` → `"C:\\ProgramData\\Cursor"`, `case 2:` → macOS, `case 3:` → Linux).

**b) `resources\app\out\vs\workbench\api\node\extensionHostProcess.js`** (processo Node, onde o
proxy `executeHookDirect` realmente executa) — o método `$executeHookDirect`:

```js
async $executeHookDirect(e,n,t,a,s=6e4,o){
  ...
  const c=t?{shell:sL(),shellArgs:pbt}:{},
        d=new oL(n,{...c,env:pL()}),
  ...
  t&&o!==void 0&&(h=Ao.join(st.tmpdir(),`cursor-hook-payload-${process.pid}-...json`),
                   await bo.promises.writeFile(h,o,"utf8"), _=gbt(h,e));
  ...
```

`t` é o terceiro parâmetro — `v===1` vindo do renderer, ou seja, `t===true` no Windows. Quando
`t` é verdadeiro, a sessão de shell é criada com `shell: sL()` e `shellArgs: pbt`:

```js
function sL(){
  if(process.platform!=="win32"){
    const n=process.env.SHELL;
    if(n?.includes("pwsh")||n?.includes("powershell"))return n
  }
  let e=Gr("pwsh",[]).cmd;
  if(e!=="pwsh"||(e=Gr("powershell",[]).cmd,e!=="powershell")||process.platform==="win32"&&
    (e=rL.join(process.env.SYSTEMROOT,"System32","WindowsPowerShell","v1.0","powershell.exe"),
     yIt(e)))return e;
  throw new Error("Neither 'pwsh' (PowerShell Core) nor 'powershell' (Windows PowerShell) found in PATH")
}
```

```js
pbt=["-NoProfile","-NonInteractive","-ExecutionPolicy","Bypass"];
```

Lida até o fim (a leitura inicial deste documento tinha cortado a função no meio, no mesmo padrão
de erro que a seção do Codex já havia corrigido uma vez — generalizar a partir de um trecho
truncado): `sL()` resolve, em ordem, `pwsh` (PowerShell 7+) no PATH; senão `powershell` no PATH;
senão o caminho fixo `%SYSTEMROOT%\System32\WindowsPowerShell\v1.0\powershell.exe` (PowerShell 5.1,
que faz parte de toda instalação padrão do Windows desde o Windows 7/Server 2008 R2) **se esse
arquivo existir** (`yIt(e)`, checagem de existência). Só se as três tentativas falharem — cenário
essencially inatingível numa instalação Windows de fábrica, já que a 3ª é um caminho fixo do
sistema operacional, não uma busca em PATH — a função **lança uma exceção** (`throw new Error(...)`)
em vez de cair para `cmd.exe` ou bash. **Não existe branch de `cmd.exe`/bash em `sL()`** — confirmado
lendo a função inteira até o `throw` final, não só o trecho inicial. `oL` (`NaiveTerminalExecutor`,
classe que efetivamente spawna o processo) foi lida também: seu construtor consome
`this.options?.shell` e `this.options?.shellArgs` diretamente no `spawn` (`Wi(this.options?.shell||
process.env.SHELL||"/bin/sh",[...this.options?.shellArgs??[],"-c",p],...)`), confirmando que o
`shell`/`shellArgs` calculados por `$executeHookDirect` (`sL()`/`pbt`) são de fato os que chegam ao
processo filho, não apenas nomes de opção que coincidem por acaso.

O `command` bruto não é passado como argumento de `-Command`; em vez disso, é escrito no payload de
um arquivo temporário JSON e injetado dentro de um mini-script PowerShell gerado por `gbt`:

```js
pbt já cobre os flags fixos; gbt(tempFilePath, command) monta:
`$OutputEncoding = [System.Text.Encoding]::UTF8; Get-Content -LiteralPath '<tempFilePath>' -Raw | & { $input | <command> }`
```

ou seja, o `command` do `hooks.json` é interpolado **literalmente** dentro de um bloco de script
(`{ $input | <command> }`) invocado pelo operador `&`, recebendo o payload JSON via stdin (`$input`).
O `&` aqui invoca o **bloco de script** inteiro, não o `command` isoladamente — a função auxiliar
`_bt` só prefixaria um `& ` extra na frente do `command` se ele começasse com aspas (`'`/`"`); para o
`command` que o trackfw emite hoje — `scripts/trackfw-credential-guard.sh` (caminho relativo bare,
sem prefixo `bash` nem aspas, ver `internal/generators/agentfiles.go`,
`mergeCredentialGuardCursorHooks`/`mergeCursorGuardMatcherEntry`) — `_bt` devolve a string sem
modificação. O script final vira, na prática, `... | & { $input |
scripts/trackfw-credential-guard.sh }`: dentro do bloco, é a resolução de comando comum do
PowerShell (não um operador `&` extra) que tenta localizar `scripts/trackfw-credential-guard.sh`
como cmdlet/executável e falha — mesmo resultado final (o `.sh` não roda), mecanismo levemente
diferente do que uma leitura apressada do trecho sugeriria.

**Grau de certeza: medido no bundle instalado**, versão 3.19.7 (arm64), dois arquivos, cadeia
completa renderer→node. Não é doc do fornecedor (Cursor não documenta isso publicamente) nem
inferência — é leitura direta do artefato que roda de fato na máquina do usuário.

**`.sh` dispara?** Não — mesma classe de falha final de Codex/Gemini (PowerShell não interpreta
shebang `#!/bin/bash`), com um agravante específico do Cursor: como o `command` é usado sem
`bash`/`sh` explícito na frente, mesmo que o usuário tivesse Git Bash instalado e no PATH, o
PowerShell ainda tentaria resolver `scripts/trackfw-credential-guard.sh` como um **comando/cmdlet**
(não encontraria — `.sh` não está em `PATHEXT` nem tem handler de extensão registrado por padrão),
não como "rode isto com bash". O caminho de correção mínimo teria que emitir algo como
`bash scripts/trackfw-credential-guard.sh` (ou script `.ps1` nativo) no campo `command`, não o
caminho bare atual — decisão de implementação, fora do escopo desta investigação.

### 6. Kiro — medido no bundle instalado (extensão Node): `cmd.exe` via default do Node.js, nunca
PowerShell/bash

`kiro.dev/docs/hooks/actions/` e `kiro.dev/docs/hooks/types/`, lidos hoje, também não mencionam
shell de execução no Windows (mesma busca vazia). Kiro também é fornecedor fechado (produto AWS).
Mesmo procedimento do Cursor: `winget install --id Amazon.Kiro --silent --accept-package-agreements
--accept-source-agreements` na mesma VM, sem login — instala **Kiro 1.0.437**. Kiro também é um
fork de VS Code, mas sua feature de hooks vive numa extensão bundlada, não no core do workbench:
`resources\app\extensions\kiro.kiro-agent\dist\extension.js` (bundle único, 12.9M caracteres,
`package.json` da extensão confirma `"name":"kiroAgent"`, `"version":"1.0.794"`,
`"engines":{"node":"^22.21.1"}`).

Cadeia de execução, três pontos no mesmo arquivo:

**a) Parse do `action` do hook** (`$Io`) — o `command` é lido direto do JSON do hook sem
transformação de shell:

```js
let o=r.action.type==="command"?{kind:jh.Command,command:r.action.command}:{kind:jh.Agent,prompt:r.action.prompt},
```

**b) `CommandAction.execute` (classe `kor`)** — único placeholder suportado é `${WORKSPACE_ROOT}`
(o trackfw não usa esse placeholder, então o `command` chega intacto):

```js
kor=class{ ... async execute(t,e,r){
  ...
  let l=t.action.command.replace(/\$\{WORKSPACE_ROOT\}/g,r.cwd);
  try{
    let u=await this.deps.processRunner.spawn({command:l,cwd:r.cwd,env:{},stdin:o,signal:r.signal,timeoutMs:a});
    ...
```

**c) `NodeProcessRunner.spawn` (classe `Vor`, exportada como `NodeProcessRunner` no módulo de
hooks)** — a implementação real, chamando o `child_process` nativo do Node:

```js
DIo=require("child_process"),
...
Vor=class{ ... spawn(t){
  return new Promise(e=>{
    let r=process.platform==="win32",
        n=(0,DIo.spawn)(t.command,{cwd:t.cwd,env:{...process.env,...t.env},shell:!0,detached:!r,stdio:["pipe","pipe","pipe"]});
    ...
```

`t.command` é passado como **string única** (não array de argv) para `child_process.spawn`, com
`{shell:true}` e **sem** a opção `shell:` apontando para um executável específico. Pelo contrato
documentado do próprio Node.js (`child_process.spawn(command[, args][, options])`, opção
`options.shell`): *"If true, runs command inside of a shell... On Windows, the default shell is
specified by `process.env.ComSpec`"* — que é `cmd.exe` numa instalação padrão do Windows (o
`ComSpec` só aponta para outro shell se o usuário o alterar manualmente, o que não é o comportamento
padrão de nenhuma instalação Windows). Kiro não sobrescreve `ComSpec` nem passa `shell: "pwsh.exe"`
em lugar nenhum desse trecho — é o único ponto de execução de comando de hook no bundle (buscado por
todas as ocorrências de `spawn(t){` no arquivo, não só a primeira: **1 ocorrência**, no offset
2678696).

A dúvida seguinte — se o `processRunner` que `CommandAction.execute` de fato usa em produção é esta
implementação (`Vor`/`NodeProcessRunner`) e não alguma outra injetada por fora — foi fechada
buscando todas as ocorrências de `processRunner:` e `new Vor` no arquivo (4 ocorrências de
`processRunner:`, todas fazendo apenas *forwarding* posicional do parâmetro entre construtores; **1**
ocorrência de `new Vor`, na inicialização do módulo `v2Hooks` da extensão):

```js
if(u.v2===!0)this.v2HooksCache=new Jor({fs:new PFe,processRunner:new Vor,clock:new Hor,
  telemetry:Kor(),logger:P,homeDir:this.homeDir,cloudConfigBase:...})
```

`Jor` é `HooksModuleCache` (a classe que gerencia os módulos de hooks por workspace, vista na seção
de parse acima — `featureFlags:{v2Hooks:!0}`), e é exatamente essa cadeia que resolve
`PreToolUse`/`PostToolUse` para `.kiro/hooks/*.json`. `NodeProcessRunner` (`Vor`) é, portanto, a
única implementação de `processRunner` instanciada em todo o arquivo — não apenas "a única definida
no arquivo", mas a que é concretamente conectada na inicialização do subsistema de hooks. O fato de
Kiro também expor `execute_pwsh` como ferramenta distinta de `execute_bash` (achado inicial, tabela
de alias de ferramentas) não afeta esta conclusão: esse é um mapeamento de nomes de **ferramenta do
agente** (o que o modelo pode invocar como ação), não do **executor de hook** — são dois subsistemas
diferentes no mesmo bundle, e só o segundo (`NodeProcessRunner`) processa `action.command` de
`hooks.json`.

**Grau de certeza: medido no bundle instalado**, extensão `kiro.kiro-agent` v1.0.794 (Kiro 1.0.437),
três funções na mesma cadeia de chamada, mais o comportamento de default do Node.js documentado
oficialmente (não é suposição própria — é a doc do runtime que o próprio bundle roda em cima).

**`.sh` dispara?** Não — por um motivo diferente de Cursor/Codex/Gemini: aqui não é PowerShell sem
suporte a shebang, é `cmd.exe` sem suporte a shebang **e** sem associação de extensão `.sh` por
padrão. Para o `command` que o trackfw emite (`scripts/trackfw-credential-guard.sh`, mesmo caminho
bare, ver `InjectKiroHooks` em `internal/generators/agentfiles.go`), `cmd.exe /d /s /c
"scripts/trackfw-credential-guard.sh"` falha com `"scripts/trackfw-credential-guard.sh" não é
reconhecido como um comando interno ou externo` — a mesma classe de falha final (arquivo não
executável para o shell escolhido), só que via `cmd.exe` em vez de PowerShell.

Nenhuma inferência foi necessária para nenhum dos dois — a leitura do bundle fechou a pergunta sem
precisar do Procmon planejado no handoff original (ver tabela de experimentos abaixo, mantida como
registro do que teria sido feito se a leitura de bundle não tivesse bastado).

## O que este handoff presumia e que a leitura derrubou

1. O handoff assumia implicitamente que a bifurcação era binária ("`sh`/Git Bash" vs "`cmd.exe`
   ou PowerShell"). A leitura mostrou que **3 dos 6 CLIs caem em categorias diferentes entre si**
   dentro do lado "não é bash": Codex e Gemini convergem em PowerShell como caminho comum (variável
   resolve, mas PowerShell não roda `.sh`), Copilot nem chega a essa pergunta — quebra por campo de
   config ausente, não por shell — e `cmd.exe` só aparece como fallback de borda do Codex, nunca
   como caminho padrão de CLI nenhum.
2. O handoff pedia para verificar se "quem executa o `command` é o CLI do agente" — confirmado, mas
   com uma variante que o handoff não previa: **Gemini CLI faz sua própria expansão de variável em
   JavaScript antes de invocar qualquer shell**, então o mecanismo de ancoragem do ADR-2026-08-11
   (`$GEMINI_PROJECT_DIR/...`) já funciona hoje mesmo no PowerShell — o único elo que falta lá é o
   shell não saber rodar `.sh`, não a variável não expandir.
3. Claude Code, que o `docs/cli-parity.md` existente já tratava como o caso mais estudado (por ter
   o bug de cwd corrigido em produção), acabou sendo o único dos 6 com resposta documentada pelo
   fornecedor **na granularidade exata desta pergunta** ("shell form... Git Bash no Windows, ou
   PowerShell quando Git Bash não está instalado") — nenhuma pesquisa anterior do repositório havia
   citado essa frase, porque a pesquisa de 2026-08-11 respondia (a)-(d) sobre cwd/placeholders, não
   sobre shell.
4. O handoff presumia que Cursor e Kiro exigiriam Procmon numa VM Windows autenticada para resolver
   — "fornecedor fechado" foi tratado como sinônimo de "sem evidência de código acessível". A leitura
   mostrou que isso vale para o repositório-fonte, não para o artefato instalado: os dois são forks
   de Electron/VS Code com bundle JS legível em disco (não ofuscado a ponto de perder strings
   literais), e a pergunta foi respondida por leitura estática do bundle, sem autenticar nenhuma
   conta e sem precisar do Procmon planejado — o mesmo padrão de "ler o artefato" que fechou
   Gemini/Codex por repositório público, adaptado a um binário fechado. Achado extra não previsto:
   **Kiro é o único dos 6 CLIs medidos que usa `cmd.exe`**, não PowerShell — por herdar o default do
   Node.js (`child_process.spawn` com `shell:true` sem override), não por escolha deliberada
   documentada em lugar nenhum do produto.

## Experimento mínimo por lacuna

Nenhum dos 6 CLIs está instalável no runner de CI atual (macOS, sem contas/licenças dos fornecedores
— Claude Code, Codex CLI, Gemini CLI, Cursor, Copilot CLI e Kiro exigem autenticação/instalação que
o ambiente de CI deste repositório não tem). O braço `run.ps1` de
`scripts/windows-repro/` já resolve isto para scripts **do próprio trackfw** (bash/PowerShell
puros); os 6 CLIs de agente são produtos externos de terceiros, fora do que esse harness pode
instalar automaticamente. Os experimentos abaixo são para o contribuidor externo com **Windows 11
real**, um a um, cada um independente dos outros:

| CLI | O que instalar | O que rodar | O que observar |
|---|---|---|---|
| **Claude Code** (confirmar o log de debug, não mais "resolver ambiguidade" — a doc já fecha o mecanismo) | Claude Code no Windows, com e sem Git Bash no PATH | `trackfw init` num repo de teste; disparar uma chamada Bash/Read/Edit para acionar o `PreToolUse` do credential-guard, uma vez **com** Git Bash instalado e uma vez **sem** (renomear/remover temporariamente o Git Bash do PATH) | Caso 1 (Git Bash presente): o script `.sh` deve rodar e produzir sua saída normal. Caso 2 (Git Bash ausente): confirmar que o log de debug do Claude Code mostra o warning documentado de `$CLAUDE_PROJECT_DIR` não reescrito, e que o hook falha (silenciosa ou visivelmente) |
| **Codex CLI** | Codex CLI no Windows, projeto marcado `trusted` (`~/.codex/config.toml`, `trust_level = "trusted"` — pré-condição já documentada em `docs/cli-parity.md`) | `trackfw init`; acionar uma chamada de shell dentro do Codex para disparar `PreToolUse` (turno já em curso — é o caso que a leitura de código prediz como comum) | Confirmar a predição desta pesquisa: o processo filho gerado é `pwsh.exe`/`powershell.exe` (não `cmd.exe`), e se a substituição `$(git rev-parse --show-toplevel)` resolve corretamente dentro do `-Command` do PowerShell antes de falhar em rodar o `.sh`. Também vale disparar um hook de `SessionStart` (antes de qualquer turno) para checar se **esse** caso cai no fallback `cmd.exe` de borda, confirmando a distinção entre os dois caminhos que a leitura de código previu. Se confirmado, a correção mínima é gerar `command_windows` (`bash.exe -lc "..."` ou script `.ps1` nativo), nunca reescrever `command` (quebraria macOS/Linux) |
| **Gemini CLI** | Gemini CLI no Windows | Mesmo procedimento | Confirmar que `$GEMINI_PROJECT_DIR` é substituído corretamente na string final (visível no log/debug do Gemini se houver) e que a falha, se houver, é "arquivo `.sh` não reconhecido pelo PowerShell", não "variável não expandida" — distingue as duas causas possíveis |
| **GitHub Copilot CLI** | Copilot CLI no Windows | `trackfw init`; acionar `preToolUse`/`postToolUse` para o hook de credential-guard | Confirmar que o hook **nunca dispara** (nenhuma saída, nenhum log de execução) — evidência independente do achado de leitura de schema da seção 1, sem precisar de shell nenhum: já é vácuo de campo |
| ~~**Cursor**~~ **RESOLVIDO por leitura de bundle (seção 5)** | ~~Cursor no Windows + Sysinternals Process Monitor (Procmon)~~ Não foi necessário — bundle instalado leu de forma definitiva | — | Confirmação opcional com Procmon (`.cursor/hooks.json` do trackfw, filtro "Process Create") continua válida como segunda fonte independente se algum dia o comportamento do bundle for questionado, mas não é mais bloqueante |
| ~~**Kiro**~~ **RESOLVIDO por leitura de bundle (seção 6)** | ~~Kiro no Windows + Procmon~~ Não foi necessário — bundle instalado leu de forma definitiva | — | Mesma nota do item Cursor, com `.kiro/hooks/trackfw-attention.json` |

Para Cursor e Kiro, Procmon filtrado em "Process Create" teria sido o experimento mais barato que
resolveria a pergunta sem exigir acesso a código-fonte (diferente de `Get-Process`/Gerenciador de
Tarefas, que só amostra processos vivos no instante da consulta, Procmon captura o evento de criação
em si) — mas a leitura estática do bundle instalado (Electron/Node, texto legível, strings literais
preservadas) chegou à mesma resposta sem precisar rodar o hook de verdade nem instrumentar o SO.

## Fecho

**Atualização de 2026-09-06**: as duas lacunas que fechavam este documento como "investigação
concluída, com 2 CLIs indeterminados" foram fechadas por leitura de bundle instalado numa VM Windows
ARM64 real (build `10.0.26200`, acesso via SSH, sem autenticar nenhuma conta) — ver seções 5 e 6.
**Nenhum dos 6 CLIs é mais indeterminado.**

**O `.ps1` (ou script nativo equivalente) é necessário para Codex CLI, Gemini CLI e Cursor** —
medido em código-fonte (Codex/Gemini) ou bundle instalado (Cursor), não inferência: os três rodam
hooks via PowerShell no caminho comum do Windows (`pwsh.exe` se presente, senão `powershell.exe`),
que não interpreta shebang de `.sh`; "não fazer nada" não é opção para nenhum dos três. **Um
equivalente nativo do Windows (`.bat`/`.cmd`, ou de novo o mesmo `.ps1`) é necessário para Kiro** —
medido no bundle instalado: Kiro roda hooks via `cmd.exe` (default do Node.js `child_process` sem
override explícito de shell), que também não interpreta shebang nem tem associação de arquivo para
`.sh`. **Para Claude Code, é necessário condicionalmente** — Git Bash cobre o caminho feliz e é
pré-condição já aceita para Codex/git; documentar a dependência pode bastar, mas só se o experimento
mínimo confirmar que o fallback sem Git Bash é raro o suficiente para não tratar. **Para a interação
`command_windows`+`trust_level` do Codex, ainda é indeterminado** até a medição em Windows real
(único item restante na tabela de experimentos) — o achado de Copilot (campo `"bash"` sem
`"command"`/`"powershell"`) não depende de `.ps1` nenhum, é troca de chave no gerador.

**Consequência prática para o campo `command` que o trackfw emite hoje** (caminho relativo bare,
`scripts/trackfw-credential-guard.sh`, sem prefixo `bash`/`sh`, idêntico nos geradores de Cursor e
Kiro): em nenhum dos 6 CLIs esse `command` dispara o `.sh` no Windows sem intervenção — a única
variável entre eles é a classe exata de falha (PowerShell sem shebang em 4 CLIs, `cmd.exe` sem
shebang em 1, campo de config ausente em 1). Isso não é mais "falta de dado" para nenhum CLI: é
"dado que aponta para a mesma conclusão por 5 caminhos técnicos distintos, mais um vácuo de
configuração" — a decisão de implementação (que mecanismo Windows-nativo emitir, e onde) é a próxima
REQ, fora do escopo desta investigação pura.
