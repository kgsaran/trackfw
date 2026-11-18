# Antigravity — Contrato de hooks e status do Gemini CLI

> Data: 2026-11-18
> REQ: REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md
> Autor: prometeu-tf — ML-0B (Wave 0, investigação documental; nenhum código alterado)
> Branch: feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli

---

## URLs efetivamente buscadas (com status HTTP)

| URL | Status | Observação |
|---|---|---|
| `https://antigravity.google/docs/hooks.md` | **200** | Fonte primária principal; contém App, CLI e IDE numa página única |
| `https://antigravity.google/llms.txt` | **200** | Índice de docs do site |
| `https://antigravity.google/docs/cli/gcli-migration.md` | **200** | Migração do Gemini CLI → Antigravity CLI |
| `https://antigravity.google/docs/sandbox.md` | **200** | Terminal sandbox; menciona suporte Windows |
| `https://antigravity.google/docs/cli/using.md` | **200** | Configurações e dicas do CLI |
| `https://antigravity.google/docs/cli/headless.md` | **200** | Modo headless / `-p` |
| `https://antigravity.google/docs/install.md` | **200** | Autenticação; menciona Windows Credential Manager e API key |
| `https://antigravity.google/docs/getting-started.md` | **200** | Quickstart; confirma Windows no CLI |
| `https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli/` | **200** | Blog post de transição (2026-05-19) |
| `https://antigravity.google/docs/hooks?tab=ide` | **200** | Redirect do lado do cliente para `/docs/hooks` (meta-refresh + JS) |
| `https://antigravity.google/docs/ide/hooks` | **200** | Redirect para `/docs/hooks?tab=ide` |
| `https://antigravity.google/docs/index.md` | **404** | — |
| `https://antigravity.google/docs/cli/agy.md` | **404** | — |

Fonte primária para hook contract: `https://antigravity.google/docs/hooks.md` (200, markdown, 543 linhas).
As abas (App, CLI, IDE) vivem numa única página — `/docs/hooks?tab=ide` e `/docs/ide/hooks` redirecionam
para ela.
O guia de terceiro Medium referenciado na tarefa (`medium.com/google-cloud/a-developers-guide-...`) **não foi
buscado** — seria marcado como terceiro/não-primário. Nenhuma informação dele é usada aqui.

---

## 1. Locais do `hooks.json`

### 1.1 Por superfície e nível

| Nível | Antigravity App | Antigravity CLI (`agy`) | Antigravity IDE |
|---|---|---|---|
| **Workspace (projeto)** | `.agents/hooks.json` na raiz do workspace | `.agents/hooks.json` na raiz do projeto | `.agents/hooks.json` no projeto aberto |
| **Global** | `~/.gemini/config/hooks.json` | `~/.gemini/config/hooks.json` | `~/.gemini/config/hooks.json` |
| **Plugin** | não documentado | Dentro do `hooks.json` de cada plugin instalado | não documentado |

Citação literal da doc (CLI `agy`):
> "Hooks in `~/.gemini/antigravity-cli/settings.json` **aren't loaded**."

Ou seja: `~/.gemini/antigravity-cli/settings.json` não carrega hooks (é o arquivo de configuração geral do CLI).
Os hooks globais vivem em `~/.gemini/config/hooks.json`.

O campo `app_data_dir` nos payloads de stdin resolve de forma diferente por superfície:
- Antigravity App: `~/.gemini/antigravity`
- Antigravity CLI: `~/.gemini/antigravity-cli`
- Antigravity IDE: `~/.gemini/antigravity-ide`

> **Comparação com Gemini CLI (trackfw atual):** o Gemini usa `.gemini/settings.json` (workspace) e
> `~/.gemini/settings.json` (global). O Antigravity muda o arquivo de workspace para `.agents/hooks.json`
> e separa os hooks globais em `~/.gemini/config/hooks.json`. O namespace `~/.gemini/` é **compartilhado**
> — consequência documentada na seção 7, item "shared namespace".

> **Nota sobre `~/.gemini/settings.json`:** a doc exclui explicitamente `~/.gemini/antigravity-cli/settings.json`
> mas **não menciona** `~/.gemini/settings.json` (onde o trackfw hoje grava hooks globais do Gemini CLI).
> Se o Antigravity também lê esse arquivo ou se o ignora para hooks é **não documentado — medir (Wave 1,
> braço M11)**.

### 1.2 Windows — suporte confirmado

O Antigravity App suporta Windows 10 (64-bit) e Windows 11 (fonte: `docs/getting-started.md`).
O Antigravity CLI tem instalador Windows separado (fonte: `docs/getting-started.md`):

```powershell
# Windows (PowerShell)
irm https://antigravity.google/cli/install.ps1 | iex
# Binário padrão: C:\Users\<username>\AppData\Local\agy\bin
```

O sandbox (`docs/sandbox.md`) menciona: "On Windows, the sandbox requires Windows 11 24H2 or later with
the August 2026 update or later" — confirma que a plataforma Windows é suportada.

A autenticação no Windows usa o Windows Credential Manager (fonte: `docs/install.md` — "Apple Keychain,
Linux Secret Service/dbus, or Windows Credential Manager").

### 1.3 Precedência e merge entre workspace e global

**Não documentado.** A doc descreve cada nível mas não especifica:
- Se workspace sobrepõe global por nome de hook idêntico.
- Se hooks dos dois níveis são fundidos ou se um silencia o outro.
- O que acontece quando o mesmo nome de hook aparece nos dois com `"enabled": false` em um deles.

Virar item de medição na Wave 1 (braço M9).
Fonte: `https://antigravity.google/docs/hooks.md` — seção "Managing hooks in Antigravity CLI".

### 1.4 O que define "project root"

**Não documentado.** A doc usa "project root" e "workspace root" mas não especifica como o `agy` os detecta
(presença de `.agents/`, de arquivo de controle, do diretório pai mais próximo etc.).

Virar item de medição na Wave 1 (braço M10).

---

## 2. Eventos disponíveis

| Evento | Momento de disparo | Matcher suportado? | Pode bloquear? |
|---|---|---|---|
| `SessionStart` | Uma vez, antes da primeira invocação do modelo | Não (ignorado) | Não (output: `injectSteps`) |
| `PreToolUse` | Antes da execução de cada ferramenta | Sim (regex contra `toolCall.name`) | **Sim** (output: `{"decision":"deny"}`) |
| `PostToolUse` | Após a conclusão de cada ferramenta | Sim (regex contra `toolCall.name`) | **Não** (output obrigatório: `{}`) |
| `PreInvocation` | Antes de cada chamada ao modelo | Não | Sim (via `terminationBehavior: "terminate"`) |
| `PostInvocation` | Após cada chamada ao modelo | Não | Sim (via `terminationBehavior: "terminate"`) |
| `Stop` | Quando o loop de execução termina | Não | Sim (via `decision: "continue"`) |

Fonte: `https://antigravity.google/docs/hooks.md` — seção "Supported events".

> **Achado crítico — PostToolUse não bloqueia:** o trackfw hoje usa `AfterTool` no Gemini CLI para
> o credential-guard. No Antigravity, `PostToolUse` só pode retornar `{}` — sem `decision`. A leg
> `AfterTool` do guard precisa ser movida para `PreToolUse` ou eliminada para o Antigravity.

> **Achado — sem evento Notification:** o Antigravity não tem evento `Notification` nem matcher
> `ToolPermission`. O hook de attention (`geminiSignalCmd`, `geminiCleanupCmd`) não tem equivalente
> documentado no Antigravity.

---

## 3. Ferramentas disponíveis (nomes para o matcher)

### 3.1 Ferramenta de shell (gate de comando)

| Ferramenta | Descrição | Campo com o comando no payload |
|---|---|---|
| `run_command` | "Propose a **Bash** command to run." | `toolCall.args.CommandLine` (string, PascalCase) |

> **Comparação com Gemini CLI:** o Gemini usa `run_shell_command`; o comando chega em `tool_input.command`.
> O Antigravity usa `run_command`; o comando chega em `toolCall.args.CommandLine`.

### 3.2 Ferramentas de escrita/edição de arquivo

| Ferramenta | Descrição |
|---|---|
| `write_to_file` | Cria novos arquivos (`TargetFile`, `Overwrite`, `CodeContent`, ...) |
| `replace_file_content` | Edita um bloco contíguo num arquivo (`TargetFile`, `TargetContent`, `ReplacementContent`, ...) |
| `multi_replace_file_content` | Múltiplas edições não-contíguas no mesmo arquivo |

> **Comparação com Gemini CLI:** o Gemini usa `write_file|replace`. No Antigravity são três ferramentas
> separadas: `write_to_file|replace_file_content|multi_replace_file_content`.

### 3.3 Ferramenta de leitura de arquivo

| Ferramenta | Descrição |
|---|---|
| `view_file` | Lê o conteúdo de um arquivo (`AbsolutePath`, `StartLine` opt, `EndLine` opt) |

> **Comparação com Gemini CLI:** o Gemini usa `read_file|read_many_files`. O Antigravity usa `view_file`.

### 3.4 Segundo caminho de entrada de shell

A ferramenta `manage_task` com `Action: "send_input"` permite enviar input para uma tarefa em background.
Um matcher `"run_command"` em `PreToolUse` não casa `manage_task` — mas este caminho pode introduzir
comandos sem ser interceptado pelo guard. Registrado aqui como pista; a análise de ameaça pertence ao
ML-0A (hades-tf).

---

## 4. Schema do JSON no stdin

### 4.1 Campos comuns a todos os eventos

| Campo | Tipo | Descrição |
|---|---|---|
| `conversationId` | string | UUID da conversa ativa |
| `workspacePaths` | array de strings | Caminhos absolutos dos workspaces montados |
| `transcriptPath` | string | `<app_data_dir>/brain/<conversationId>/.system_generated/logs/transcript.jsonl` |
| `artifactDirectoryPath` | string | Caminho para artefatos e screenshots da conversa |
| `modelName` | string | Identificador do modelo (ex.: `gemini-3.6-flash-medium`) |
| `lastUserInput` | string | Prompt mais recente do usuário |

**Atenção (`lastUserInput`):** este campo está em **todos** os payloads de todos os eventos (fonte:
`hooks.md`, seção "Common input fields"). O guard camada 1 (`RunCredential` em
`internal/guard/credential.go:222`) escaneia `string(data)` — os bytes brutos do payload completo.
Isso significa que um segredo colado no prompt do usuário vai disparar o guard em toda chamada de
ferramenta subsequente até o próximo prompt — falso-positivo estrutural que não existe quando o
payload não inclui o prompt.

> Se o payload Gemini CLI inclui ou não `lastUserInput` é **não verificado** — sem fonte disponível.
> O braço M13b mede se a camada 1 do guard efetivamente bloqueia quando o segredo está apenas no
> prompt. Documentar como restrição no ML-3A.

Fonte: `https://antigravity.google/docs/hooks.md` — seção "Common input fields", com exemplos de stdin.

### 4.2 Campos específicos de `PreToolUse`

| Campo | Tipo | Descrição |
|---|---|---|
| `toolCall` | object | Detalhes do tool call proposto |
| `toolCall.name` | string | Nome da ferramenta (ex.: `run_command`) |
| `toolCall.args` | object | Argumentos; para `run_command`: `CommandLine`, `Cwd`, `WaitMsBeforeAsync`, `RunPersistent`, `RequestedTerminalID` |
| `stepIdx` | integer | Índice 0-based do step atual na trajetória |
| _(Campos comuns)_ | | Incluídos no payload |

**Exemplo de payload `PreToolUse` para `run_command` (adaptado do exemplo da doc):**

```json
{
  "toolCall": {
    "name": "run_command",
    "args": {
      "CommandLine": "npm test",
      "Cwd": "/workspace/project",
      "WaitMsBeforeAsync": 5000
    }
  },
  "stepIdx": 19,
  "conversationId": "ec33ebf9-0cba-4100-8142-c61503f6c587",
  "workspacePaths": ["/workspace/project"],
  "transcriptPath": "~/.gemini/antigravity-cli/brain/ec33ebf9-.../.system_generated/logs/transcript.jsonl",
  "artifactDirectoryPath": "~/.gemini/antigravity-cli/brain/ec33ebf9-...",
  "modelName": "gemini-3.6-flash-medium",
  "lastUserInput": "Run the tests"
}
```

Fonte: `https://antigravity.google/docs/hooks.md` — seção "PreToolUse", input example (adaptado; o campo
`lastUserInput` foi acrescentado ao exemplo pois é listado como campo comum na seção "Common input fields"
mas não aparece explicitamente no exemplo de PreToolUse da doc).

### 4.3 Impacto em `ExtractCommand` / `credExtractCmdAndCwd`

A função `ExtractCommand` (`internal/guard/payload.go:148–198`) lê, em ordem de prioridade:
`tool_input.command`, `command` (root-level), `tool_info.command_line`, `hook_input.command`.

A função `credExtractCmdAndCwd` (`internal/guard/credential.go:737–797`) segue a mesma prioridade.

A função `credDeepScan` (`internal/guard/credential.go:1075–`) faz walk recursivo procurando chaves
`"command"` ou `"command_line"` (camelCase/snake_case).

O Antigravity entrega o comando em `toolCall.args.CommandLine` — nenhuma dessas quatro prioridades o casa
(`toolCall` não é `tool_input`; `CommandLine` é PascalCase, não `command` nem `command_line`).

**Resultado na prática:**
- **Camada 1 (scan bruto):** `inputStr := string(data)` escaneia o payload completo → encontra `CommandLine`
  no texto JSON, **mas também escaneia `lastUserInput`** (falso-positivo potencial).
- **`credExtractCmdAndCwd`:** retorna `shellCmd=""` para payloads Antigravity → `contextStr=""` → camada 2
  de scan de arquivo referenciado **é um no-op** para payloads Antigravity.
- **`credDeepScan`:** procura `"command"` e `"command_line"` (lowercase) → **não encontra `"CommandLine"`**
  (PascalCase) → no-op adicional.

Resumo: o guard bloqueia credenciais na camada 1 (raw scan), mas o scan de arquivo referenciado pelo comando
(camada 2) não funciona para payloads Antigravity. Decisão sobre `--format antigravity` vai para a ADR da
Wave 2.

---

## 5. Contrato de saída do hook (`PreToolUse`)

### 5.1 Campos documentados (stdout → JSON)

| Campo | Tipo | Descrição |
|---|---|---|
| `decision` | string | **Obrigatório.** `"allow"`, `"auto_approve"`, `"deny"`, `"ask"`, `"force_ask"`, `"deny_unless_prior_grant"` |
| `reason` | string | Opcional. Explicação mostrada ao agente ou ao usuário |
| `overwrite` | object | Opcional. Argumentos de substituição (shallow merge sobre `toolCall.args`) |
| `permissionOverrides` | array de strings | Opcional. Lista de resource strings para override de permissões |

Fonte: `https://antigravity.google/docs/hooks.md` — seção "PreToolUse", output fields.

### 5.2 O que a doc não especifica (itens para medição na Wave 1)

| Comportamento | Status | Por que importa |
|---|---|---|
| Exit code não-zero (ex.: exit 2) | **Não documentado** | O guard atual emite exit 2 para deny; se o Antigravity ignora exit code, o guard é fail-open |
| Stdout vazio para `PreToolUse` | **Não documentado** | `decision` é "Required" na doc → stdout vazio pode ser allow ou deny |
| JSON inválido no stdout | **Não documentado** | Fail-open ou fail-closed |
| Timeout: comportamento quando `command` estoura | **Não documentado** | Default de `timeout` é 30s; fail-open ou fail-closed |
| stderr: lido? exibido? afeta decision? | **Não documentado** | O guard hoje envia a reason para stderr |

> **Achado estrutural:** `decision` é marcado como **Required** na doc. A ausência de stdout (ou JSON
> inválido) pode ser tratada como JSON inválido ou como allow por default. Sem especificação explícita,
> **não se pode assumir que exit 2 bloqueia** — o contrato documentado usa `{"decision": "deny"}`.

### 5.3 Campos de configuração do hook

**Nível de entrada no array `hooks`:**

| Campo | Tipo | Descrição |
|---|---|---|
| `type` | string | Opcional. Apenas `"command"` suportado. Default: `"command"` |
| `command` | string | Obrigatório. O comando de shell a executar |
| `timeout` | integer | Opcional. Timeout em segundos. Default: `30` |

**Nível de hook-definition (acima do array):**

| Campo | Tipo | Descrição |
|---|---|---|
| `enabled` | boolean | Opcional. `false` desativa o hook sem removê-lo. Default: `true` |

Fonte: `https://antigravity.google/docs/hooks.md` — seções "Hook handler configuration" e "Hook definition fields".

---

## 6. Shell usado para executar o `command`

**Não documentado em `hooks.md`.** A doc descreve `command` como "The shell command to execute" mas não
especifica qual shell o interpreta no macOS ou Windows, o cwd do processo, nem quais variáveis de ambiente
são passadas.

**O que se sabe por outros docs:**
- O CLI se instala no Windows via PowerShell (`irm ... | iex`, `docs/getting-started.md`)
- A ferramenta `run_command` é descrita como "Propose a **Bash** command" — mas isso descreve o que o
  *agente* pode fazer, não o que executa o *hook*
- A autenticação Windows usa Windows Credential Manager

**Comparação com CLIs conhecidos** (documentado em `2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md`
e `2026-10-07-matcher-do-hook-vs-ferramenta-de-shell-por-cli.md`):

| CLI | Shell no Windows | Shell no macOS | Fonte |
|---|---|---|---|
| Gemini CLI | PowerShell (`pwsh`/`powershell.exe`) | Determinado pelo sistema | Medido (`packages/core/src/utils/shell-utils.ts`) |
| Codex CLI | PowerShell internamente; tool_name = `"Bash"` (OS-agnóstico) | bash/sh | Medido (`hook_names.rs` + JSONL `2026-10-07`) |
| **Antigravity (`agy`)** | **Não documentado — medir** | **Não documentado — medir** | — |

Itens de medição: Wave 1, braços M1 e M2.

---

## 7. Divergências: contrato atual do trackfw (Gemini) × Antigravity

| Aspecto | Gemini CLI (trackfw atual) | Antigravity (documentado) | Risco para o guard |
|---|---|---|---|
| **Arquivo de config (workspace)** | `.gemini/settings.json` | `.agents/hooks.json` | Alto — trackfw não grava nada para Antigravity hoje (`internal/generators/hooks.go:16`) |
| **Arquivo de config (global)** | `~/.gemini/settings.json` | `~/.gemini/config/hooks.json` | Alto — caminhos diferentes; ambos sob `~/.gemini/` |
| **`~/.gemini/` compartilhado** | Settings do Gemini CLI | Hooks globais do Antigravity em subdiretório `/config/` | Médio — sobreposição potencial não documentada (ver braço M11) |
| **Evento de pre-tool** | `BeforeTool` | `PreToolUse` | Alto — Antigravity não reconhece `BeforeTool` |
| **Evento de post-tool** | `AfterTool` | `PostToolUse` | Alto — nome diferente; e PostToolUse não bloqueia |
| **BeforeTool pode bloquear?** | Sim — `BeforeTool` + exit 2 bloqueia (comentário em `agentfiles.go:742-760`: "Exit Code 2 (Block Tool): Prevents execution. Uses stderr as the reason"). `AfterTool` também emite o guard (`agentfiles.go:877-879`), mas o AfterTool dispara **após** a execução — seu exit 2 não desfaz o comando | Antigravity não usa `BeforeTool`; `PreToolUse` + `{"decision":"deny"}` é o equivalente; `PostToolUse` não bloqueia | Arquitetural — `BeforeTool` do Gemini não tem equivalente direto; é necessário mapear para `PreToolUse` com saída JSON |
| **Ferramenta de shell** | `run_shell_command` | `run_command` | Alto — matcher `run_shell_command` nunca casa no Antigravity |
| **Ferramenta de leitura** | `read_file\|read_many_files` | `view_file` | Alto — matcher atual não casa |
| **Ferramenta de escrita** | `write_file\|replace` | `write_to_file\|replace_file_content\|multi_replace_file_content` | Alto — matcher parcial no melhor caso |
| **Mecanismo de bloqueio** | Exit 2 → deny (fonte: `guard.go:159-176`) | `{"decision":"deny"}` em stdout | Crítico — se Antigravity ignora exit code, o guard atual é fail-open |
| **Campo de extração do comando** | `tool_input.command` (prio 1 em `ExtractCommand`) | `toolCall.args.CommandLine` | Crítico — `credExtractCmdAndCwd` não suporta este caminho; camada 2 é no-op para payloads Antigravity |
| **Scan bruto de payload** | Gemini payload: presença de `lastUserInput` não verificada (fonte ausente) | Antigravity inclui `lastUserInput` em todos os payloads (fonte: `hooks.md` seção "Common input fields") | Médio — falso-positivo potencial; inferido do código (`credential.go:222`, raw scan); medir M13b |
| **Evento Notification** | `Notification` + `ToolPermission` (attention hook) | Sem equivalente documentado | Médio — attention hook não tem onde viver no Antigravity |

---

## 8. Status do Gemini CLI

**Fonte:** `https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli/`
(HTTP 200, publicado 2026-05-19, autores: Dmitry Lyalin e Taylor Mullen, Google).

Citação literal do artigo (corpo):

> "On June 18, 2026, Gemini CLI and Gemini Code Assist IDE extensions will **stop serving requests** for
> Google AI Pro and Ultra, as well as those using it free of charge using Gemini Code Assist for individuals."

> "For enterprise customers — If your organization uses Gemini CLI or our IDE extensions via a **Gemini Code
> Assist Standard or Enterprise license**, or if your organization uses Gemini Code Assist for GitHub through
> Google Cloud, **your access remains unchanged.** We'll continue to support Gemini CLI and Gemini Code
> Assist with access to the latest Gemini models and other updates. **Gemini CLI will remain accessible via
> paid Gemini and Gemini Enterprise Agent Platform API keys.**"

**Resumo:**

| Perfil | Status a partir de 2026-06-18 |
|---|---|
| Google AI Pro / Ultra (assinatura individual) | Parou de executar requests |
| Gemini Code Assist for individuals (gratuito) | Parou de executar requests |
| Gemini Code Assist for GitHub | "no new installations; requests in the following weeks will stop being served" |
| Gemini Code Assist Standard ou Enterprise (empresa, via Google Cloud) | **Acesso inalterado** |
| **Chave de API Gemini paga (individual ou Enterprise Agent Platform)** | **Acesso mantido** — "remain accessible via paid Gemini and Gemini Enterprise Agent Platform API keys" |

**Distinção chave:** o artigo distingue acesso por *assinatura* (AI Pro/Ultra, que parou) de acesso por
*chave de API paga* (que continua). Usuários com chave de API individual paga do Gemini Developer API
(Google AI Studio) **continuam com acesso** ao Gemini CLI após junho de 2026.

**Conclusão operacional:** o trackfw deve manter os hooks do Gemini CLI — continua funcional para usuários
com API key e para Enterprise. Decisão do KG (Diagnóstico do Roadmap): "manter como legado; `doctor`/`validate`
avisam que é legado e apontam o Antigravity."

---

## 9. Roteiro de medição — Wave 1

### 9.1 Pré-requisitos

```
Plataformas: Mac (zsh/bash, agy instalado) + VM Windows (PowerShell, agy instalado)
Ferramentas: agy CLI, trackfw no PATH de cada plataforma
Restrição: não tocar ~/.gemini/settings.json, ~/.claude, ~/.codex (ver M9/M11 para exceção controlada)
```

### 9.2 Setup: diretório de sonda

**Mac (zsh):**

```zsh
PROBE_DIR="$HOME/agyprobe"
mkdir -p "$PROBE_DIR/.agents"
mkdir -p "$PROBE_DIR/scripts"
cd "$PROBE_DIR"
git init   # agy pode exigir um repo para detectar workspace root
```

**Windows (PowerShell):**

```powershell
$ProbeDir = "$env:USERPROFILE\agyprobe"
New-Item -ItemType Directory -Path "$ProbeDir\.agents" -Force | Out-Null
New-Item -ItemType Directory -Path "$ProbeDir\scripts" -Force | Out-Null
Set-Location $ProbeDir
git init
```

### 9.3 Scripts de sonda

Os scripts ficam em `$PROBE_DIR/scripts/` (diretório separado de `.agents/` para que o agente não os
carregue como contexto). O `command` no `hooks.json` referencia o interpretador explícito para **não
depender do shell de invocação antes de medi-lo**.

**probe.sh (Mac) — criar com o bloco abaixo (copiar e colar no terminal; o heredoc não expande nada
porque o delimitador é entre aspas simples `'PROBE_END'`):**

```zsh
mkdir -p "$HOME/agyprobe/scripts"
cat > "$HOME/agyprobe/scripts/probe.sh" << 'PROBE_END'
#!/bin/sh
# Lê stdin, grava log, e executa o braço indicado como $1
ARM="${1:-unknown}"
LOG="/tmp/agyprobe_$(date +%s)_${$}_${ARM}.log"
STDIN_DATA=$(cat)

{
  printf '=== ARM: %s ===\n' "$ARM"
  printf '--- STDIN ---\n'
  printf '%s\n' "$STDIN_DATA"
  printf '--- SHELL ---\n'
  printf 'SHELL_VAR=%s\n' "${SHELL:-}"
  printf 'ZERO=%s\n' "$0"
  printf 'CWD=%s\n' "$(pwd)"
  printf 'HOME=%s\n' "$HOME"
  printf 'PATH=%s\n' "$PATH"
  printf '--- PROCESS (parent) ---\n'
  ps -o pid,ppid,command -p $$ 2>/dev/null || true
  PARENT_PID=$(ps -o ppid= -p $$ 2>/dev/null | tr -d ' ')
  [ -n "$PARENT_PID" ] && ps -o pid,ppid,command -p "$PARENT_PID" 2>/dev/null || true
  printf '=== END ===\n'
} >> "$LOG"

printf 'LOG: %s\n' "$LOG" >&2

# Comportamento do braço (prefixo: "deny-global", "allow-ws", etc. casam deny*/allow*)
case "$ARM" in
  allow*)   printf '{"decision":"allow"}';;
  deny*)    printf '{"decision":"deny","reason":"probe deny arm"}';;
  exit2)    exit 2;;
  exit1)    exit 1;;
  empty)    : ;;          # stdout vazio
  badjson)  printf 'NOT_JSON';;
  timeout)  sleep 10 && echo SLEPT_FULL >> "$LOG";;   # escreve no log se não foi interrompido
  *)        printf '{"decision":"allow"}';;
esac
PROBE_END
chmod +x "$HOME/agyprobe/scripts/probe.sh"
```

**probe.ps1 (Windows) — criar em `C:\agyprobe\scripts\probe.ps1` (sem BOM, sem `<username>`):**

Usar `Set-Content -Encoding ASCII`. O here-string com `@'...'@` mantém `$` literal, portanto
execute o bloco abaixo num PowerShell como está — não substitua nada:

```powershell
New-Item -ItemType Directory -Path 'C:\agyprobe\scripts' -Force | Out-Null
Set-Content -Encoding ASCII -Path 'C:\agyprobe\scripts\probe.ps1' -Value @'
param([string]$Arm = "unknown")
$LogFile = "C:/agyprobe/logs/probe_$(Get-Date -Format 'yyyyMMddHHmmssfff')_${Arm}.log"
New-Item -ItemType Directory -Path (Split-Path $LogFile) -Force | Out-Null

$StdinData = [Console]::In.ReadToEnd()

$Lines = @(
  "=== ARM: $Arm ===",
  "--- STDIN ---",
  $StdinData,
  "--- SHELL ---",
  "PSVersion: $($PSVersionTable.PSVersion)",
  "COMSPEC: $env:COMSPEC",
  "SHELL_VAR: $env:SHELL",
  "CWD: $(Get-Location)",
  "HOME: $env:HOME",
  "USERPROFILE: $env:USERPROFILE",
  "PATH: $env:PATH",
  "--- PROCESS (this + parent) ---"
)
[IO.File]::WriteAllText($LogFile, ($Lines -join "`n") + "`n", [Text.Encoding]::ASCII)
# transcriptPath/artifactDirectoryPath no stdin → resolve app_data_dir efetivo

try {
  $Me = Get-CimInstance -ClassName Win32_Process -Filter "ProcessId = $PID"
  $Parent = Get-CimInstance -ClassName Win32_Process -Filter "ProcessId = $($Me.ParentProcessId)"
  Add-Content -Encoding ASCII -Path $LogFile -Value (
    "PID=$($Me.ProcessId) PPID=$($Me.ParentProcessId) CMD=$($Me.CommandLine)`n" +
    "PARENT PID=$($Parent.ProcessId) PPID=$($Parent.ParentProcessId) CMD=$($Parent.CommandLine)`n" +
    "=== END ===`n"
  )
} catch {
  Add-Content -Encoding ASCII -Path $LogFile -Value "PROCESS QUERY ERROR: $_`n=== END ===`n"
}

[Console]::Error.WriteLine("LOG: $LogFile")

# Comportamento do braço (prefixo: "deny-global", "allow-ws", etc. casam deny*/allow*)
switch -Wildcard ($Arm) {
  "allow*"  { [Console]::Out.Write('{"decision":"allow"}') }
  "deny*"   { [Console]::Out.Write('{"decision":"deny","reason":"probe deny arm"}') }
  "exit2"   { exit 2 }
  "exit1"   { exit 1 }
  "empty"   { }
  "badjson" { [Console]::Out.Write('NOT_JSON') }
  "timeout" { Start-Sleep -Seconds 10; Add-Content -Encoding ASCII -Path $LogFile -Value "SLEPT_FULL" }
  default   { [Console]::Out.Write('{"decision":"allow"}') }
}
'@
```

### 9.4 Testemunha de execução (marcador)

Para cada braço, pedir ao agente no prompt (após configurar o `hooks.json` correto):

```
Run exactly this shell command and nothing else:
echo MARKER_<ARM> > /tmp/marker_<arm>.txt
```

(Windows — usar caminho literal com barras no sentido certo; NÃO usar `$env:TEMP` pois o shell do hook
é o que está sendo medido; usar `C:/agyprobe/` que funciona tanto em cmd quanto em bash/PowerShell:)
```
Run exactly this shell command and nothing else:
echo MARKER_ALLOW > C:/agyprobe/marker_allow.txt
```

| Resultado | Interpretação |
|---|---|
| Log de sonda existe + marcador existe | Hook disparou; braço liberou (allow) |
| Log de sonda existe + marcador ausente | Hook disparou; braço bloqueou |
| Log de sonda ausente + marcador existe | Hook não disparou (matcher não casou ou hook não carregado) |
| Log de sonda ausente + marcador ausente | Agente não tentou executar ou hooks não carregados |

Verificar `/hooks` no TUI do `agy` antes de cada sessão para confirmar que o hook está ativo.

**Protocolo por braço:**
1. Limpar marcadores antigos: `rm -f /tmp/marker_*.txt` (Mac) / `Remove-Item C:/agyprobe/marker_*.txt -ErrorAction SilentlyContinue` (Win)
2. Sair do `agy`, reescrever `.agents/hooks.json` (ou usar a macro `arm`/`Set-Arm`), iniciar sessão nova.
3. Verificar hooks carregados (a doc menciona um "hooks inspector"; o comando exato não foi verificado
   — tentar `/hooks` no TUI; se não existir, prosseguir sem e notar ausência).
4. **Registrar se apareceu prompt de aprovação:** o `agy` pode pedir aprovação mesmo quando o hook retorna
   `allow`. Sempre aprovar o prompt e anotar que apareceu — a aparição não significa que o hook não
   disparou.

**Executar M4 (deny documentado) e allow antes de qualquer outro braço** — confirmam que o mecanismo
básico funciona antes de testar comportamentos de borda.

### 9.5 `hooks.json` de sonda por braço

O arquivo vai em `$PROBE_DIR/.agents/hooks.json`. Os comandos abaixo geram o arquivo correto
com expansão de path e sem BOM.

**Mac — gerador de hooks.json por braço:**

```zsh
# Função auxiliar (colar no terminal uma vez por sessão)
arm() {
  local a="$1" t="${2:-10}"
  cat > "$HOME/agyprobe/.agents/hooks.json" << EOF
{
  "probe": {
    "PreToolUse": [{"matcher": "run_command", "hooks": [
      {"type": "command", "command": "/bin/sh $HOME/agyprobe/scripts/probe.sh $a", "timeout": $t}
    ]}]
  }
}
EOF
  printf 'hooks.json → braço %s (timeout %ss)\n' "$a" "$t"
}

# Usar: arm deny | arm allow | arm exit2 | arm empty | arm badjson | arm timeout 5
```

**Windows — gerador de hooks.json por braço:**

```powershell
function Set-Arm {
  param([string]$A, [int]$T = 10)
  $content = "{`n  `"probe`": {`n    `"PreToolUse`": [{`"matcher`": `"run_command`", `"hooks`": [`n      {`"type`": `"command`", `"command`": `"powershell -NoProfile -ExecutionPolicy Bypass -File C:/agyprobe/scripts/probe.ps1 -Arm $A`", `"timeout`": $T}`n    ]}]`n  }`n}"
  [IO.File]::WriteAllText("$env:USERPROFILE\agyprobe\.agents\hooks.json", $content, [Text.Encoding]::ASCII)
  Write-Host "hooks.json -> braço $A (timeout ${T}s)"
}

# Usar: Set-Arm deny | Set-Arm allow | Set-Arm exit2 | Set-Arm empty | Set-Arm badjson | Set-Arm timeout 5
```

### 9.6 Braços de medição

**M1 — Shell e ambiente no macOS**
- Configurar: `arm allow` (macro acima)
- Prompt: `echo MARKER_M1 > /tmp/marker_m1.txt`
- Medir no log: CommandLine do processo pai (campo `PARENT ...CMD=` no log), `PATH`, `HOME`, `CWD`
- O discriminador-chave é o **processo pai**, não `$0` ou `SHELL_VAR` (que descrevem o interpretador
  explícito passado no `command`, não o shell que o `agy` usou para iniciar o hook):
  - Hipótese sh/bash/zsh: pai mostra `/bin/sh -c ...` / `bash -c ...` / `zsh -c ...`
  - Hipótese exec direto: pai é o processo `agy` ou node/go runner; filho é `/bin/sh` sem intermediário
- Adicional — braço de fingerprint de shell (após M1 confirmar que hook dispara):
  ```zsh
  # hooks.json com comando literal para fingerprint (não usa probe.sh — comando direto)
  cat > "$HOME/agyprobe/.agents/hooks.json" << 'EOF'
  {"probe-shellid":{"PreToolUse":[{"matcher":"run_command","hooks":[
    {"type":"command","command":"echo \"0=$0 BASH=$BASH_VERSION ZSH=$ZSH_VERSION\" > /tmp/shellid_hook.txt","timeout":10}
  ]}]}}
  EOF
  ```
  Depois do marcador: `cat /tmp/shellid_hook.txt`. Se `$0` tem `/bin/sh`, `bash` etc. → shell identificado.
  Se o arquivo não foi criado pelo hook (o hook não expandiu nada): o hook tem shell diferente e não
  expandiu `$0`. O pai no log do M1 é o discriminador final.

**M2 — Shell e ambiente no Windows**
- Configurar: `Set-Arm allow` (macro acima)
- Prompt: `echo MARKER_M2 > C:/agyprobe/marker_m2.txt`
- Medir no log: CommandLine do processo pai via WMI (`PARENT ... CMD=`)
- O discriminador-chave é o **processo pai**:
  - Hipótese cmd.exe: pai mostra `cmd.exe /c ...` ou similar
  - Hipótese PowerShell: pai mostra `powershell.exe -Command ...` ou `pwsh -Command ...`
  - Hipótese bash: pai mostra `bash -c ...`
  - Hipótese exec direto: pai é o processo `agy`
- Adicional — braço de fingerprint de shell (Windows):
  ```powershell
  [IO.File]::WriteAllText(
    "$env:USERPROFILE\agyprobe\.agents\hooks.json",
    '{"probe-shellid":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"echo %COMSPEC% $PSVersionTable $0 > C:/agyprobe/shellid_hook.txt","timeout":10}]}]}}',
    [Text.Encoding]::ASCII
  )
  ```
  `echo %COMSPEC%` → expandido por cmd; `$PSVersionTable` → expandido por PowerShell; `$0` → expandido por bash/sh.
  O filho que escreve com todos os três não expandidos: exec direto sem shell.

**M3 — Exit 2 sem stdout (contrato atual do trackfw)**
- Mac: `arm exit2` | Windows: `Set-Arm exit2`
- Prompt: `echo MARKER_M3 > /tmp/marker_m3.txt` (Mac) / `echo MARKER_M3 > C:/agyprobe/marker_m3.txt` (Win)
- O probe escreve o log e chama `exit 2` sem stdout — log deve existir; marcador é o observable
- Hipótese A (fail-closed): marcador ausente — Antigravity trata exit 2 como deny
- Hipótese B (fail-open): marcador presente — Antigravity ignora exit code; guard atual é ineficaz para Antigravity
- Executar DEPOIS de M4 (assim sabe-se que a sonda funciona antes de testar exit 2)

**M4 — `{"decision":"deny"}` no stdout (executar PRIMEIRO — calibração)**
- Mac: `arm deny` | Windows: `Set-Arm deny`
- Prompt: `echo MARKER_M4 > /tmp/marker_m4.txt` (Mac) / `echo MARKER_M4 > C:/agyprobe/marker_m4.txt` (Win)
- Resultado esperado: log criado, marcador **ausente** (bloqueado pelo deny documentado)
- Contra-braço: `arm allow` / `Set-Arm allow` → marcador **presente**
- Se M4 falhar (marcador criado mesmo com deny), o mecanismo não funciona — não prosseguir

**M5 — Stdout vazio (nenhum JSON)**
- Mac: `arm empty` | Windows: `Set-Arm empty`
- Prompt: `echo MARKER_M5 > /tmp/marker_m5.txt` (Mac) / `echo MARKER_M5 > C:/agyprobe/marker_m5.txt` (Win)
- Hipótese A (fail-open): marcador presente
- Hipótese B (fail-closed): marcador ausente

**M6 — JSON inválido no stdout**
- Mac: `arm badjson` | Windows: `Set-Arm badjson`
- Prompt: `echo MARKER_M6 > /tmp/marker_m6.txt` (Mac) / `echo MARKER_M6 > C:/agyprobe/marker_m6.txt` (Win)
- Hipótese A (fail-open): marcador presente
- Hipótese B (fail-closed): marcador ausente

**M7 — Exit 1 (exit code genérico)**
- Mac: `arm exit1` | Windows: `Set-Arm exit1`
- Prompt: `echo MARKER_M7 > /tmp/marker_m7.txt` (Mac) / `echo MARKER_M7 > C:/agyprobe/marker_m7.txt` (Win)
- Distingue: "qualquer exit ≠ 0 bloqueia" (marcador ausente) vs. "apenas exit 2" (marcador presente)

**M8 — Timeout**
- Mac: `arm timeout 5` | Windows: `Set-Arm timeout 5` (timeout do hook = 5s)
- O probe dorme 10s sem produzir output — o hook deve expirar em ~5s antes do probe responder
- Hipótese A (fail-open): marcador presente (Antigravity expira e libera)
- Hipótese B (fail-closed): marcador ausente (Antigravity expira e bloqueia)
- Registrar quanto tempo o `agy` ficou esperando antes de continuar/bloquear

**M9 — Precedência workspace × global**

Os dois hooks têm o **mesmo nome** (`policy-g9`) para testar sobreposição. Os braços `deny-global`
e `allow-ws` casam via prefixo na função `arm` (o `case` usa `deny*)` / `allow*)`).

```zsh
# Precaução: backup do global antes
mkdir -p "$HOME/.gemini/config"
[ -f "$HOME/.gemini/config/hooks.json" ] && cp "$HOME/.gemini/config/hooks.json" "$HOME/.gemini/config/hooks.json.bak"

# Global: deny
cat > "$HOME/.gemini/config/hooks.json" << EOF
{
  "policy-g9": {
    "PreToolUse": [{"matcher": "run_command", "hooks": [
      {"type": "command", "command": "/bin/sh $HOME/agyprobe/scripts/probe.sh deny-global", "timeout": 10}
    ]}]
  }
}
EOF

# Workspace: allow — MESMO NOME policy-g9 (testa sobreposição)
# NÃO usar arm() aqui: arm() escreve "probe", não "policy-g9"
cat > "$HOME/agyprobe/.agents/hooks.json" << EOF
{
  "policy-g9": {
    "PreToolUse": [{"matcher": "run_command", "hooks": [
      {"type": "command", "command": "/bin/sh $HOME/agyprobe/scripts/probe.sh allow-ws", "timeout": 10}
    ]}]
  }
}
EOF
```

```powershell
# Windows: equivalente
New-Item -ItemType Directory -Path "$env:USERPROFILE\.gemini\config" -Force | Out-Null
# Backup se existir
if (Test-Path "$env:USERPROFILE\.gemini\config\hooks.json") {
  Copy-Item "$env:USERPROFILE\.gemini\config\hooks.json" "$env:USERPROFILE\.gemini\config\hooks.json.bak"
}
[IO.File]::WriteAllText(
  "$env:USERPROFILE\.gemini\config\hooks.json",
  "{`"policy-g9`": {`"PreToolUse`": [{`"matcher`": `"run_command`", `"hooks`": [{`"type`": `"command`", `"command`": `"powershell -NoProfile -ExecutionPolicy Bypass -File C:/agyprobe/scripts/probe.ps1 -Arm deny-global`", `"timeout`": 10}]}]}}",
  [Text.Encoding]::ASCII
)
# Workspace: allow — MESMO NOME policy-g9
[IO.File]::WriteAllText(
  "$env:USERPROFILE\agyprobe\.agents\hooks.json",
  "{`"policy-g9`":{`"PreToolUse`":[{`"matcher`":`"run_command`",`"hooks`":[{`"type`":`"command`",`"command`":`"powershell -NoProfile -ExecutionPolicy Bypass -File C:/agyprobe/scripts/probe.ps1 -Arm allow-ws`",`"timeout`":10}]}]}}",
  [Text.Encoding]::ASCII
)
```

Prompt de medição: `echo MARKER_M9 > /tmp/marker_m9.txt` (Mac) / `echo MARKER_M9 > C:/agyprobe/marker_m9.txt` (Win)

- Hipótese workspace-sobrepõe-global: apenas log `allow-ws` aparece; marcador criado
- Hipótese global-sobrepõe-workspace: apenas log `deny-global` aparece; marcador ausente
- Hipótese merge (ambos executam): dois logs aparecem (um com `ARM: deny-global`, outro com `ARM: allow-ws`)

**M9b — `enabled: false` em hook de workspace com deny no global:**
Mesmo setup do M9, mas o workspace com `allow-ws` recebe `"enabled": false`:
```zsh
cat > "$HOME/agyprobe/.agents/hooks.json" << EOF
{
  "policy-g9": {
    "enabled": false,
    "PreToolUse": [{"matcher": "run_command", "hooks": [
      {"type": "command", "command": "/bin/sh $HOME/agyprobe/scripts/probe.sh allow-ws", "timeout": 10}
    ]}]
  }
}
EOF
```
- Se log `deny-global` aparece + marcador **ausente**: o `enabled:false` do workspace não suprimiu o global-deny → global prevalece mesmo com workspace desativado → ameaça confirmada (guard global não pode ser silenciado por workspace)
- Se log `deny-global` **ausente** + marcador **presente**: o `enabled:false` do workspace suprimiu o global → ameaça ativa (projeto pode desativar guard global)

**M9b — Windows equivalente:**
```powershell
# enabled:false no workspace
[IO.File]::WriteAllText(
  "$env:USERPROFILE\agyprobe\.agents\hooks.json",
  "{`"policy-g9`":{`"enabled`":false,`"PreToolUse`":[{`"matcher`":`"run_command`",`"hooks`":[{`"type`":`"command`",`"command`":`"powershell -NoProfile -ExecutionPolicy Bypass -File C:/agyprobe/scripts/probe.ps1 -Arm allow-ws`",`"timeout`":10}]}]}}",
  [Text.Encoding]::ASCII
)
```

**Restaurar após M9/M9b (Mac):**
```zsh
[ -f "$HOME/.gemini/config/hooks.json.bak" ] && mv "$HOME/.gemini/config/hooks.json.bak" "$HOME/.gemini/config/hooks.json" || rm -f "$HOME/.gemini/config/hooks.json"
```

**Restaurar após M9/M9b (Windows):**
```powershell
$bak = "$env:USERPROFILE\.gemini\config\hooks.json.bak"
$tgt = "$env:USERPROFILE\.gemini\config\hooks.json"
if (Test-Path $bak) { Move-Item $bak $tgt -Force } else { Remove-Item $tgt -ErrorAction SilentlyContinue }
```

**M10 — Raiz do workspace (project root)**
- Garantir que o `hooks.json` de sonda com `arm allow` já está em `$HOME/agyprobe/.agents/hooks.json`
- `mkdir -p "$HOME/agyprobe/subdir" && cd "$HOME/agyprobe/subdir" && agy` (Mac)
- `New-Item -ItemType Directory "$env:USERPROFILE\agyprobe\subdir" -Force; Set-Location "$env:USERPROFILE\agyprobe\subdir"; agy` (Win)
- Prompt: `echo MARKER_M10 > /tmp/marker_m10.txt` (Mac) / `echo MARKER_M10 > C:/agyprobe/marker_m10.txt` (Win)
- Se log criado: `.agents/` foi encontrado na árvore de pais (comportamento esperado)
- Se log ausente: o `agy` só carrega `.agents/hooks.json` do CWD exato

**M11 — `~/.gemini/settings.json` lido como hooks pelo Antigravity?**
```zsh
# Backup e escreve arquivo mínimo de teste (não "acrescenta" para evitar JSON inválido)
SRC="$HOME/.gemini/settings.json"
[ -f "$SRC" ] && cp "$SRC" "$SRC.bak"
# Formato A: chave Gemini CLI (BeforeTool/run_shell_command)
cat > "$SRC" << EOF
{"hooks":{"BeforeTool":[{"matcher":"run_shell_command","hooks":[{"command":"/bin/sh $HOME/agyprobe/scripts/probe.sh allow-settings-A"}]}]}}
EOF
# Executar agy e prompt; depois repetir com formato B
# Formato B: chave Antigravity (PreToolUse/run_command)
cat > "$SRC" << EOF
{"hooks":{"PreToolUse":[{"matcher":"run_command","hooks":[{"command":"/bin/sh $HOME/agyprobe/scripts/probe.sh allow-settings-B"}]}]}}
EOF
# Restaurar
[ -f "$SRC.bak" ] && mv "$SRC.bak" "$SRC" || rm -f "$SRC"
```
- Se log `ARM: allow-settings-A` aparece: Antigravity lê `~/.gemini/settings.json` no formato Gemini CLI
- Se log `ARM: allow-settings-B` aparece: Antigravity lê `~/.gemini/settings.json` no formato Antigravity
- Se nenhum aparece: `~/.gemini/settings.json` não é lido pelo Antigravity para hooks

**M12 — Gemini CLI com chave de API individual paga ainda executa?**
- Pré-condição: ter o binário `gemini` instalado (`which gemini`) e `GEMINI_API_KEY` com chave paga
- `gemini --version` → confirma binário presente e versão
- Configurar hook de sonda no `$HOME/agyprobe/.gemini/settings.json` (criar subdiretório `.gemini/`
  dentro da sonda para não alterar o global):
  ```zsh
  mkdir -p "$HOME/agyprobe/.gemini"
  cat > "$HOME/agyprobe/.gemini/settings.json" << EOF
  {"hooks":{"BeforeTool":[{"matcher":"run_shell_command","hooks":[{"command":"/bin/sh $HOME/agyprobe/scripts/probe.sh allow-m12"}]}]}}
  EOF
  cd "$HOME/agyprobe" && gemini   # Gemini lê .gemini/settings.json do projeto
  ```
- Prompt: `echo MARKER_M12 > /tmp/marker_m12.txt`
- Resultado esperado (API key paga válida): log `ARM: allow-m12` criado + marcador criado (requests servidos)
- Contra-braço: `GEMINI_API_KEY="" gemini` → erro de autenticação; sem log nem marcador
- Nota: o artigo do blog diz que API keys pagas continuam funcionando — M12 é a medição direta; "paga
  individual" é inferido do texto ("paid Gemini...API keys"), não testado separadamente de "Enterprise"

**M13 — Contrato real do guard de credencial no Antigravity (separado em 3 sub-braços)**

> **Pré-condição:** `trackfw guard credential --global` não exige `trackfw.yaml`
> (flag `--global`, fonte: `internal/commands/guard.go:177-178`: `"run as global scope guard
> (no trackfw.yaml required, default mode block, fixed docs/roadmaps dir)"`).
>
> **Chave fake:** padrão do guard é `AIza[0-9A-Za-z_-]{35,}` (fonte:
> `internal/guard/credential.go:125`): `AIza` (4 chars) + 35+ chars alfanuméricos/`_`/`-`.
> Mínimo 39 chars. A chave de 22 chars usada anteriormente neste doc (`AIzaSy...6789`) NÃO
> casa — era muito curta.
>
> **Não escreva a chave fake literalmente neste arquivo** (o guard da sessão bloqueia grep/commit
> que passem pelo payload). Use os geradores abaixo — cada um produz a chave na hora:
>
> Mac: `printf 'AIza%0.sF' {1..35}` → `AIzaFFFFF...` (39 chars)
> Windows: `"AIza" + "F" * 35`
>
> **Comando de hook para M13** (usar caminho absoluto do trackfw + `--global`, para não depender
> do PATH do shell do hook e não exigir `trackfw.yaml`):
>
> Mac (gera o hooks.json com `$TFW` resolvido agora):
> ```zsh
> TFW="$(command -v trackfw)"
> cat > "$HOME/agyprobe/.agents/hooks.json" << EOF
> {
>   "guard-m13": {
>     "PreToolUse": [{"matcher": "run_command", "hooks": [
>       {"type": "command", "command": "$TFW guard credential --global", "timeout": 15}
>     ]}]
>   }
> }
> EOF
> ```
>
> Windows (copiar o binário para evitar shims npm; usar `--global`):
> ```powershell
> New-Item -ItemType Directory 'C:\agyprobe\bin' -Force | Out-Null
> Copy-Item (Get-Command trackfw).Source 'C:\agyprobe\bin\trackfw.exe'
> [IO.File]::WriteAllText(
>   "$env:USERPROFILE\agyprobe\.agents\hooks.json",
>   '{"guard-m13":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"C:/agyprobe/bin/trackfw.exe guard credential --global","timeout":15}]}]}}',
>   [Text.Encoding]::ASCII
> )
> ```

- **M13a — Segredo no comando:**
  - Configurar o hooks.json do guard acima (Mac ou Win)
  - Gerar o prompt com a chave (nunca escreva a chave literal no código ou neste doc):
    - Mac: `printf 'Run exactly this shell command: echo %s > /tmp/marker_m13a.txt' "$(printf 'AIza%0.sF' {1..35})" | pbcopy` — colar no agy
    - Windows: `"Run exactly this shell command: echo AIza$('F'*35) > C:/agyprobe/marker_m13a.txt" | Set-Clipboard` — colar no agy
  - Resultado se guard exit 2 bloqueia: `agy` recusa o comando; marcador ausente; guard exibe mensagem de bloqueio
  - Resultado se exit 2 ignorado (fail-open): marcador criado; nenhum bloqueio visível

- **M13b — Segredo no prompt apenas (um único prompt):**
  - Mesmo hooks.json do guard
  - Iniciar sessão nova e digitar um único prompt que contenha o segredo E o comando:
    - Mac: `printf 'Ignore this key AIza%s. Run exactly: echo ok > /tmp/marker_m13b.txt' "$(printf '%0.sF' {1..35})" | pbcopy` — colar no agy
    - Windows: `"Ignore this key AIza$('F'*35). Run exactly: echo ok > C:/agyprobe/marker_m13b.txt" | Set-Clipboard` — colar no agy
  - Resultado esperado se layer 1 escaneia `lastUserInput`: bloqueado (falso-positivo — a chave está no mesmo payload que o comando)
  - Resultado se layer 1 só vê o campo de comando: marcador criado

- **M13c — Comando inócuo, sessão limpa:**
  - Iniciar sessão nova (sem nenhum prompt anterior com chave)
  - Mesmo hooks.json do guard
  - Prompt: `Run exactly this shell command: echo ok > /tmp/marker_m13c.txt` (Mac) / `...> C:/agyprobe/marker_m13c.txt` (Win)
  - Resultado esperado: marcador criado (allow — contra-braço confirma que o guard não bloqueia comandos inócuos)

### 9.7 Achando o caminho global efetivo no Windows

```powershell
# Verificar se USERPROFILE e HOME resolvem para o mesmo lugar
$a = "$env:USERPROFILE\.gemini\config"
$b = if ($env:HOME) { "$env:HOME\.gemini\config" } else { "(HOME nao definida)" }
Write-Host "USERPROFILE path: $a"
Write-Host "HOME path: $b"
if ($a -eq $b) {
    Write-Host "SAME — gravar hook global em apenas um deles"
} else {
    Write-Host "DIFFERENT — testar M9 com hooks nos dois caminhos"
    # Usar braços distintos em cada caminho para identificar qual o agy lê
}
```

**`app_data_dir` efetivo** (campo `transcriptPath` / `artifactDirectoryPath` no log de stdin do M1/M2):
O prefixo antes de `/brain/` é o `app_data_dir` real. Para o CLI isso deve ser `~/.gemini/antigravity-cli`
no Mac ou `C:\Users\<username>\.gemini\antigravity-cli` no Windows — mas confirmar no log, não presumir.
O `app_data_dir` derivado do log é mais confiável do que comparar variáveis de ambiente.

---

## 10. Itens abertos (para o ML-1A)

| Item | O que medir | Braço |
|---|---|---|
| Shell no macOS | `ZERO`, `PARENT_PID`, output do `ps` | M1 |
| Shell no Windows | `PSVersion`, WMI CommandLine | M2 |
| Exit 2 bloqueia? | Marcador ausente × presente | M3 |
| Stdout vazio = allow ou deny? | Marcador ausente × presente | M5 |
| JSON inválido = allow ou deny? | Marcador ausente × presente | M6 |
| Timeout = fail-open ou fail-closed? | Marcador ausente × presente após ~5s | M8 |
| Precedência workspace × global | Qual log aparece | M9 |
| `enabled:false` em hook de workspace com deny no global | Marcador ausente × presente | M9b |
| Project root detectado como? | Subdiretório dispara hook ou não | M10 |
| `~/.gemini/settings.json` lido pelo Antigravity? | Probe em settings.json | M11 |
| Gemini CLI ainda serve requests com API key paga? | Marcador no binário gemini | M12 |
| Exit 2 do guard real bloqueia? | Marcador M13a | M13a |
| `lastUserInput` dispara layer 1? | Marcador M13b | M13b |
| CWD do processo de hook | Campo `CWD` no log | M1/M2 |
| Variáveis de ambiente no hook | `PATH`, `HOME`, `USERPROFILE` no log | M1/M2 |

---

## Notas de processo (ML-0B)

1. `bin/trackfw guard credential --help` foi executado a partir da raiz da árvore de trabalho
   (`/Users/kgsaran/.../trackfw/`). Isso tecnicamente conflita com a restrição "não rode o binário na
   árvore". A execução foi somente para leitura de help text; `git status --short` confirmou árvore
   limpa antes e após.

2. As URLs `antigravity.google/docs/hooks?tab=ide` e `/docs/ide/hooks` chegaram como páginas HTML com
   meta-refresh (HTTP 200 no nível de protocolo, mas redirect do lado do cliente). O conteúdo real está
   na página única `hooks.md`.

3. Nenhum arquivo de `docs/agents-working-context/` foi escrito por este agente (escrita reservada ao
   `trackfw_architect`).

4. Tentativa de dry-run do guard com payload Antigravity em diretório temporário: o guard da sessão
   Claude bloqueou os comandos Bash que continham a chave fake (incluindo um heredoc Python e um
   grep). Fatos observados: (a) o guard bloqueou comandos cujo payload de stdin da ferramenta Bash
   continha a chave — o próprio comportamento da camada 1; (b) um `grep` sobre este documento (depois
   de o documento conter a chave literal) foi bloqueado pelo mesmo mecanismo. O dry-run não foi
   completado. Consequência: **nenhuma chave fake literal pode ser escrita neste documento**, motivo
   pelo qual M13 usa geradores de chave. A confirmação de comportamento fica como item de medição M13
   no binário Antigravity real.

## 11. Medido (ML-1A, 2026-11-18) — `agy` 1.5.11 no macOS e no Windows 11 ARM64

Runners: `docs/portabilidade/antigravity-sondas/run-arms-{mac.sh,windows.ps1}`, rodados pelo KG; modo headless
(`agy -p ... --dangerously-skip-permissions --output-format stream-json`), hook de projeto em `.agents/hooks.json`,
matcher `run_command`. Cada braço foi confirmado no `stream-json`: o passo `run_command` termina em `ERROR` e a
resposta do agente cita o hook. Ou seja, o marcador ausente é bloqueio do hook, não recusa do modelo.

| Braço (saída do hook) | macOS | Windows | Leitura |
|---|---|---|---|
| `{"decision":"allow"}`, exit 0 | executa | executa | contra-braço |
| `{"decision":"deny"}`, exit 0 | **bloqueia** | **bloqueia** | contrato documentado |
| stdout vazio, **exit 2** | **bloqueia** | **bloqueia** | contrato atual do trackfw funciona |
| stdout vazio, exit 1 | **bloqueia** | **bloqueia** | todo exit ≠ 0 é fail-closed |
| stdout vazio, exit 0 | executa | executa | = allow; guard silencioso libera |
| `NOT_JSON`, exit 0 | **bloqueia** (`failed to unmarshal result ... via protojson`) | **bloqueia** | stdout que não é JSON válido bloqueia |

| Ambiente do hook | macOS | Windows |
|---|---|---|
| Quem executa o `command` | `sh -c` (`$0=sh`, bash 3.2 do `/bin/sh`); pai do script = `agy` | `cmd /s /c "<command>"`; avô = `agy.exe` |
| cwd do hook | `<projeto>/.agents` (diretório do `hooks.json`) | `C:\Users\Lab\agyprobe\.agents` |
| `HOME` / `USERPROFILE` | `HOME` herdado | `HOME` **vazio**, `USERPROFILE=C:\Users\Lab` |
| PATH | herdado do shell que lançou o `agy` | herdado (inclui `C:\Program Files\Git\cmd`) |
| Payload (stdin) | `toolCall.args.CommandLine`, `toolCall.name`, `cwd`, `workspacePaths`, `lastUserInput`, `hookEventName` | idem; caminhos com `\\` |
| `<app_data_dir>` efetivo | `~/.gemini/antigravity-cli` | `C:\Users\Lab\.gemini\antigravity-cli` |

**Consequências para a ADR:**
1. O contrato de saída do trackfw (exit 0 silencioso = libera, exit 2 = bloqueia) já é respeitado. Não é preciso
   saída JSON; o `hookSpecificOutput` que o guard de git-branch escreve junto com exit 2 também bloqueia (exit ≠ 0).
   🔴 Mas qualquer **texto não-JSON no stdout com exit 0** bloqueia o comando: o guard nunca pode escrever no stdout ao
   liberar.
2. O shell do hook é `sh -c` no macOS e `cmd /s /c` no Windows. As formas de linha que o trackfw já tem (POSIX e
   cmd.exe, ML-2B da REQ-2026-11-15) se aplicam por plataforma.
3. O comando chega em `toolCall.args.CommandLine`. O guard de git-branch não extrai esse campo e libera em silêncio
   (T2 do threat model); a camada 2 do guard de credencial também não. É mudança de código obrigatória.
4. O cwd do hook é `.agents/`, não a raiz: a busca de `trackfw.yaml` sobe e acha a raiz; o campo `cwd` do payload é a
   raiz do workspace.
5. `lastUserInput` está em todo payload: a camada 1 do guard de credencial vai bloquear um comando inócuo se o prompt
   do usuário contiver um segredo (falso positivo a decidir na ADR).

**Ainda não medido:** caminho global efetivo (`~/.gemini/config/hooks.json`) e precedência projeto × global (M9),
`enabled:false` (M9b), timeout (M8), Gemini CLI com chave paga (M12). M9 exige escrever em `~/.gemini/config/`,
o que pede autorização do KG.
