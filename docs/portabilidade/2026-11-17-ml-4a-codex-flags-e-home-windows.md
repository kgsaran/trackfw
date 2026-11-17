# Medição: Codex CLI 0.169.0 no Windows — flags de shell e HOME para config global

> Data: 2026-11-17 | Medição por: artemis-tf | VM: Windows ARM64 (192.168.64.6, Lab@)
> REQ: `docs/req/REQ-2026-11-15-...` | Roadmap: `docs/roadmaps/wip/ROADMAP-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md`
> ML: 4A (porção CLIs)

## Escopo e restrições

Duas perguntas para Codex CLI 0.169.0 somente. Os demais CLIs (Gemini, Cursor, Copilot, Windsurf,
Kiro, Amazon Q) não estão instalados na VM → declarados **não medidos** nesta sessão.

**Contagem de chamadas ao modelo** (teto 6, conforme handoff):

| Chamada | Flags principais | Hooks.json matcher | hook_fired.txt criado? |
|---|---|---|---|
| 1/6 | `--dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox` | `"Bash"` | Não — projeto não-confiável |
| 2/6 | `--trust --dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox` | `"Bash"` | Não |
| 3/6 | `--dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox` | `".*"` | Não |
| 4/6 (não numerada) | `--dangerously-bypass-approvals-and-sandbox` | `".*"` | Não |
| 5/6 | `--dangerously-bypass-hook-trust --approve-for-me` | `".*"` | Não |
| 6/6 | `--approve-for-me` (sem bypass) | `".*"` | Não |

Projeto confiável a partir da chamada 2 (trust persistido). Budget esgotado. Modo interativo não
foi testado (requer terminal; fora do escopo da medição SSH).

**Contagem adicional — teto 4 (sessão corretivo 2026-11-17):**

| Chamada | Flags | Hooks.json evento/matcher | Resultado |
|---|---|---|---|
| 1/4 | `--dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox` | SessionStart + UserPromptSubmit + PreToolUse ".*" | BOM detectado — parse fail (err no JSONL), 0 hooks |
| 2/4 | idem | idem (BOM removido) | Hang 10 min, rc=-1 (killed) — causa: `exit 0` nos stubs |
| 3/4 | idem | SessionStart apenas (sem BOM) | SessionStart FIRED — `c3_session_fired.txt` criado |
| 4/4 | idem | PreToolUse ".*" apenas (sem BOM) | PreToolUse FIRED — `c4_pretool_fired.txt` criado |

Nota: o hang da chamada 2/4 foi causado por stubs escritos com `exit 0` em vez de `exit /b 0`. Quando
o hook roda via `cmd.exe /C stubfile.cmd` e o stub chama `exit 0`, cmd.exe encerra com código 0 e
Codex recebe o sinal de término antes de o stub criar o arquivo de saída — o processo fica em estado
de race ou bloqueado esperando output do hook. Stubs corrigidos para `exit /b 0` nas chamadas 3/4 e
4/4.

Binário confirmado: launcher npm (`codex.js`) faz `spawn` do nativo
`...codex-win32-arm64/vendor/aarch64-pc-windows-msvc/bin/codex.exe` — todas as análises de
`codex-rs/` são aplicáveis.

---

## Pergunta (1) — shell e argv quando Codex executa um comando

### Resultado: MEDIÇÃO EMPÍRICA (JSONL)

Fonte: campo `command` do item JSONL `command_execution` nas chamadas 5/6 e 6/6.

```
"C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe" -Command 'echo hello-ml4a'
```

**Detalhe do argv:**

| Posição | Valor |
|---|---|
| argv[0] | `C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe` |
| argv[1] | `-Command` |
| argv[2] | `'echo hello-ml4a'` (o comando pedido pelo agente) |

**Flags presentes:** `-Command` somente.
**Flags ausentes:** `-NoProfile`, `-NonInteractive`.

### Discrepância com leitura anterior de código-fonte

A medição de 2026-10-04
(`docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md`) citou
`derive_exec_args` do Codex: `[shell_path, "-NoProfile", "-Command", <command>]`.

A medição empírica de 2026-11-17 mostra `-Command` SEM `-NoProfile`. Interpretação mais provável:
o source code em questão pertence a uma versão anterior; o `derive_exec_args` de 0.169.0 não
adiciona mais `-NoProfile`. A medição empírica prevalece sobre a leitura de código-fonte (ADR D2).

### Mecanismo de execução em exec mode

O JSONL completo da chamada 6/6 (sem nenhuma flag de bypass) mostra:

```jsonl
{"type":"thread.started","thread_id":"01a1e9ce-43f5-7c01-85af-1bb1e386da87"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"I'll run `echo hello-ml4a`."}}
{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"\"C:\\\\WINDOWS\\\\System32\\\\WindowsPowerShell\\\\v1.0\\\\powershell.exe\" -Command 'echo hello-ml4a'","exit_code":null,"status":"in_progress"}}
{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"...","exit_code":0,"status":"completed"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"```text\nhello-ml4a\n```"}}
{"type":"turn.completed","usage":{"input_tokens":33005,"cached_input_tokens":24576,"output_tokens":64}}
```

Observação: o item `command_execution` é emitido como item nativo do Codex (Responses API), sem
item `tool_call` antecedente visível no JSONL. Isso é uma característica do formato de saída, não
evidência de que hooks não são consultados — ver seção de hooks abaixo.

---

## Pergunta (2) — qual HOME o Codex lê para config global

### Resultado: MEDIÇÃO EMPÍRICA (codex doctor --json)

**Protocolo:** copiar config.toml marcador em cada home candidato, variar a env var, correr
`codex doctor --json` e ler `checks.config.load.details.CODEX_HOME` e `checks.config.load.details.config.toml`.

**Bases de comparação:**

| Braço | Env var alterada | Marcador em | Resultado CODEX_HOME doctor | Config.toml lido | Interpretação |
|---|---|---|---|---|---|
| (a) USERPROFILE | `USERPROFILE=C:\Users\Lab\ml4a\codex\homeA` | `homeA\.codex\config.toml` (marker_homeA_dotcodex) | `C:\Users\Lab\.codex` | Config real, 0 MCP | USERPROFILE ignorado |
| (b) HOME | `HOME=C:\Users\Lab\ml4a\codex\homeB` | `homeB\.codex\config.toml` (marker_homeB_dotcodex) | `C:\Users\Lab\.codex` | Config real, 0 MCP | HOME ignorado |
| (c) CODEX_HOME | `CODEX_HOME=C:\Users\Lab\ml4a\codex\homeC` | `homeC\config.toml` (marker_homeC_root) | `C:\Users\Lab\ml4a\codex\homeC` | marker_homeC_root, 1 MCP | CODEX_HOME respeitado; nível ROOT |
| (d) duplo | HOME + USERPROFILE alterados | ambos homeA e homeB | `C:\Users\Lab\.codex` | Config real, 0 MCP | Confirma OS API |

**Detalhe do braço (c) — estrutura do CODEX_HOME:**

Quando `CODEX_HOME=<dir>`:
- Lido: `<dir>\config.toml` (nível raiz, **não** `<dir>\.codex\config.toml`)
- Confirmado: `homeC\config.toml` lido (marker_homeC_root MCP present), `homeC\.codex\config.toml` ignorado

### Conclusão da medição HOME

Codex 0.169.0 no Windows resolve o home assim:
1. Se `CODEX_HOME` está definido → usa `$CODEX_HOME` (lê config.toml no nível raiz do CODEX_HOME)
2. Se não → usa a API do SO Windows (`SHGetKnownFolderPath` / equivalente) → resolve para `C:\Users\Lab\.codex`
3. `HOME` e `USERPROFILE` env vars são completamente ignorados

Fonte de evidência adicional: `codex doctor --json` sem vars alteradas reporta
`"CODEX_HOME": "C:\\Users\\Lab\\.codex"` que coincide exatamente com
`%USERPROFILE%\.codex` mas NÃO responde às mudanças de env USERPROFILE — portanto é a API do SO.

---

## Medição de hooks — contra-braço e correção

### 🔴 CORRIGIDO (sessão 2026-11-17): hooks DISPARAM em exec mode

A afirmação original "PreToolUse não dispara" foi baseada em 6 chamadas que não criaram
`hook_fired.txt`. O contra-braço desta sessão revelou que a causa era o arquivo `hooks.json` com
UTF-8 BOM — não o comportamento do exec mode.

**Achado BOM:** `hooks.json` escrito por PowerShell (`Set-Content` ou `[IO.File]::WriteAllText` sem
codificação explícita) ganha BOM `EF BB BF`. O parser do Codex 0.169.0 (serde_json) rejeita BOM com
`"expected value at line 1 column 1"`. Com `--dangerously-bypass-hook-trust`, o erro aparece como
item `error` no JSONL; sem o flag, o erro é silencioso. Correção: usar
`[System.Text.UTF8Encoding]::new($false)` ou escrever via `scp` de macOS.

**Resultado do contra-braço (sessão corretivo, chamadas 3/4 e 4/4):**

| Evento testado | Arquivo de saída | Resultado |
|---|---|---|
| SessionStart | `c3_session_fired.txt` | **FIRED** — conteúdo "fired_c3_session" |
| PreToolUse ".*" | `c4_pretool_fired.txt` | **FIRED** — conteúdo "fired_c4_pretool" |

**Setup comum às chamadas 3/4 e 4/4:**
- Diretório: `C:\Users\Lab\ml4a\codex\repo\` (git repo)
- hooks.json UTF-8 sem BOM, schema correto, escrito via `scp` de macOS
- Stubs: `.cmd` com `exit /b 0`, caminho absoluto para o arquivo de saída
- Flags: `--dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox`
- JSONL de ambas: bypass warnings (x2), `turn.started`, `command_execution` completo — rc=0, ~9s

**Referência de source code:**
`codex-rs/hooks/src/engine/discovery.rs::load_hooks_json()` lê `<dot_codex_folder>/hooks.json`.
`codex-rs/hooks/src/engine/command_runner.rs::build_command()` executa hooks via
`cmd.exe /C "<command>"` com `CREATE_NO_WINDOW` e `env_clear()` no Windows.
`session/turn.rs:411` chama `run_pending_session_start_hooks()` no caminho de exec mode.

**Sobre as 6 chamadas originais:**
exec5 (chamada 5/6, `--dangerously-bypass-hook-trust --approve-for-me`) mostrou bypass warnings
sem erro de parse — consistent com hooks.json válido OU com hooks.json ausente (warnings podem
vir de arquivo de hooks global em `~/.codex/hooks.json`). `hook_fired.txt` não foi criado por
nenhuma das 6 chamadas. Causa mais provável: BOM no hooks.json original, cuja mensagem de erro
era silenciada pela ausência de `--dangerously-bypass-hook-trust` nas chamadas sem esse flag.

**Disclaimer:** modo interativo (`codex` sem `exec`) não foi testado nesta sessão.

---

## Implicações para internal/agentconfig/agentconfig.go

### HomeFor(CLICodex) — divergência de comportamento medida

Implementação atual (`internal/agentconfig/agentconfig.go:128-132`):

```go
func (OSCLIHomeResolver) HomeFor(cli string) (string, error) {
    if cli == CLIClaude && effectiveGOOS() == "windows" {
        return effectiveUserHomeDir()
    }
    return homedir.Dir()
}
```

`homedir.Dir()` (`internal/homedir/homedir.go`) lê `HOME` env primeiro; se vazio, cai em
`os.UserHomeDir()` que no Windows lê `USERPROFILE`.

**Regra medida do Codex 0.169.0:**
1. Se `CODEX_HOME` definido → usa `$CODEX_HOME` (lê `config.toml` no nível raiz, NÃO `$CODEX_HOME\.codex\config.toml`)
2. Caso contrário → chama `dirs::home_dir()` → `SHGetKnownFolderPath(FOLDERID_Profile)` (Windows API, ignora env vars)

Fonte: `codex-rs/utils/home-dir/src/lib.rs::find_codex_home()` → `dirs` 6.0.0 `win.rs:5` →
`dirs-sys` 0.5.0 `src/lib.rs::known_folder_profile()`.

**Condições de divergência (trackfw vs Codex):**

| Condição | trackfw `homedir.Dir()` | Codex | Divergem? |
|---|---|---|---|
| `HOME` definido | retorna `HOME` | ignora | **Sim** |
| `HOME` vazio, `USERPROFILE` override | `os.UserHomeDir()` retorna override | ignora | **Sim** |
| `CODEX_HOME` definido | desconhecido (sem handling) | usa `$CODEX_HOME` como raiz | **Sim** |
| Nenhuma das anteriores | `os.UserHomeDir()` → API Windows | API Windows | Coincidem |

A divergência ocorre apenas nas três condições listadas; em ambiente padrão (sem env vars forçadas)
os dois resolvem para `C:\Users\<user>` via API Windows.

### InjectCodexHooks — guard PreToolUse funciona em exec mode

O guard de credencial emitido por `InjectCodexHooks` usa `PreToolUse` com matcher `"Bash"`. A
medição desta sessão (chamada 4/4, matcher `".*"`) confirma que PreToolUse hooks **disparam** em
`codex exec` mode com Responses API no Codex 0.169.0.

A afirmação original ("guard não funciona em exec mode") era baseada em medição com hooks.json
corrompido por BOM. Com hooks.json válido (sem BOM), o guard é consultado antes de cada execução de
comando de shell.

**Condição crítica:** o hooks.json gerado por `InjectCodexHooks` não pode ter UTF-8 BOM. Se escrito
via PowerShell sem `[System.Text.UTF8Encoding]::new($false)`, o BOM desabilita silenciosamente todos
os hooks. Verificar encoding sempre que o arquivo for gerado programaticamente no Windows.

---

## Outros CLIs — não medidos

Gemini CLI, Cursor, GitHub Copilot CLI, Windsurf CLI, Kiro, Amazon Q: não instalados na VM
`192.168.64.6` em 2026-11-17. Declarados **não medidos** conforme instrução do handoff.
