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

Estamos em macOS; nenhum dos 6 CLIs está instalado no runner de CI. Este documento separa,
CLI a CLI, o que foi **medido** (execução real observada), o que é **documentado pelo fornecedor**
(fonte primária lida hoje, 2026-09-05, com citação literal) e o que é **inferido** — e nomeia o
experimento mínimo para fechar cada lacuna.

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
| Cursor | Não documentado; fornecedor fechado, sem acesso a código | **Indeterminado** | Indeterminado |
| GitHub Copilot CLI | Depende de qual campo do JSON está populado: `bash` (Unix) vs `powershell` (Windows) vs `command` (fallback cross-platform, copiado para os dois) | **Documentado pelo fornecedor** — e a config atual do trackfw (`InjectCopilotHooks`, só popula `bash`) está **confirmada por leitura de código**, não por doc | **Não, e por um motivo diferente dos outros**: o hook nem é lido no Windows local (ver achado 1) |
| Kiro | Não documentado; fornecedor fechado, sem acesso a código | **Indeterminado** | Indeterminado |

Achado que se repete em 3 dos 6 CLIs (Claude Code sem Git Bash, Codex no caminho comum, Gemini
sempre): **PowerShell é o shell padrão de fato no Windows para hooks de agente**, não `cmd.exe` —
e nenhum dos três interpreta shebang de `.sh`. `cmd.exe` aparece só como fallback de borda do
Codex, não como caminho comum de nenhum CLI.

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

### 5. Cursor e Kiro — genuinamente indeterminado, fornecedores fechados

Nenhuma das duas docs oficiais lidas hoje (`cursor.com/docs/hooks`; `kiro.dev/docs/hooks/actions/`,
`kiro.dev/docs/hooks/types/`) menciona shell de execução no Windows, `cmd.exe`, PowerShell, ou Git
Bash em nenhum lugar. Buscado por essas palavras nas duas — zero ocorrência. Diferente de Codex e
Gemini, **não há acesso a código-fonte**: os dois CLIs são produtos fechados (Cursor é proprietário;
Kiro é produto AWS fechado), então não há um `command_runner.rs`/`shell-utils.ts` equivalente para
ler. Isto é **exatamente** o mesmo veredito `INDETERMINADO` que
`docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` já registrou para Kiro na pergunta de
cwd — mas agora aplicado a uma pergunta diferente (shell), e desta vez também para Cursor, que
naquela pesquisa tinha veredito `OK` só para cwd (pergunta distinta, resolvida por doc).

Não escrevo "provavelmente bash" para nenhum dos dois — não há evidência primária para essa
inferência, e o handoff pede explicitamente para não fazer isso.

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
| **Cursor** | Cursor no Windows + Sysinternals Process Monitor (Procmon) | Mesmo procedimento, com `.cursor/hooks.json` do trackfw, Procmon filtrado por evento "Process Create" durante o disparo do hook | Ler a coluna "Command Line" do evento de criação de processo capturado — nome do executável (`bash.exe`, `sh.exe`, `powershell.exe`, `cmd.exe`) e argumentos resolvem a pergunta de shell diretamente |
| **Kiro** | Kiro no Windows + Procmon | Mesmo procedimento, com `.kiro/hooks/trackfw-attention.json` | Mesmo método do item Cursor |

Para Cursor e Kiro, Procmon filtrado em "Process Create" é o experimento mais barato que resolve a
pergunta sem exigir acesso a código-fonte: diferente de `Get-Process`/Gerenciador de Tarefas
(que só amostra processos vivos no instante da consulta), Procmon captura o evento de criação em si
— necessário porque um hook de guard tipicamente roda e sai em poucos milissegundos, tempo menor
que o intervalo de amostragem de uma consulta manual.

## Fecho

**O `.ps1` (ou script nativo equivalente) é necessário para Codex CLI e Gemini CLI** — medido em
código-fonte do fornecedor, não inferência: os dois rodam hooks via PowerShell no caminho comum do
Windows, que não interpreta shebang de `.sh`; "não fazer nada" não é opção para nenhum dos dois.
**Para Claude Code, é necessário condicionalmente** — Git Bash cobre o caminho feliz e é
pré-condição já aceita para Codex/git; documentar a dependência pode bastar, mas só se o experimento
mínimo confirmar que o fallback sem Git Bash é raro o suficiente para não tratar. **Para Cursor,
Kiro, e a interação `command_windows`+`trust_level` do Codex, é indeterminado** até a medição em
Windows real (tabela de experimentos acima) — nenhum dos dois tem doc ou código acessível, e o
achado de Copilot (campo `"bash"` sem `"command"`/`"powershell"`) não depende de `.ps1` nenhum,
é troca de chave no gerador.
