# Pesquisa: semântica de cwd e placeholders de caminho nos 6 CLIs de agente

> ML-0A do roadmap `ROADMAP-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd.md`.
> Toda afirmação abaixo é sustentada por URL + citação literal da documentação primária do
> fornecedor, retirada em 2026-08-11. Onde a doc não responde, a célula é `INDETERMINADO` com o
> registro do que foi procurado — nunca inferência por analogia com outro CLI.

## Tabela 6×4

### 1. Claude Code

Fonte: <https://code.claude.com/docs/en/hooks>

| Pergunta | Resposta | Citação literal |
|---|---|---|
| (a) cwd de execução | O diretório corrente do processo Claude Code no momento em que o hook dispara. | "Handlers run in the current directory with Claude Code's environment." |
| (b) fixo ou dinâmico | Dinâmico — acompanha o cwd do processo, não é fixado na raiz do projeto. | O placeholder de projeto existe explicitamente "regardless of the working directory when the hook runs" — frase que só faz sentido se o cwd puder variar. |
| (c) placeholders | `${CLAUDE_PROJECT_DIR}` (forma `$VAR`/`${VAR}` conforme shell), também exportado como env var `CLAUDE_PROJECT_DIR` no processo filho; existe também `${CLAUDE_PLUGIN_ROOT}`/`${CLAUDE_PLUGIN_DATA}` para hooks de plugin. Expandido no campo `command` (forma exec, cada elemento de `args`) e também disponível como env var lida pelo próprio script. Em PowerShell shell-form requer `$env:CLAUDE_PROJECT_DIR` (a partir da v2.1.198 Claude Code reescreve `${CLAUDE_PROJECT_DIR}` automaticamente para a sintaxe PowerShell). | "Use these placeholders to reference hook scripts relative to the project or plugin root, regardless of the working directory when the hook runs: `${CLAUDE_PROJECT_DIR}`: the project root." · "both export them as the environment variables `CLAUDE_PROJECT_DIR`, `CLAUDE_PLUGIN_ROOT`, and `CLAUDE_PLUGIN_DATA` on the spawned process" · "where `${CLAUDE_PROJECT_DIR}` is substituted in each `args` element regardless of where the hook is defined." |
| (d) relativo resolve contra | Contra o cwd dinâmico do processo (não contra o arquivo de settings) — é exatamente por isso que a doc recomenda usar o placeholder em vez de caminho relativo puro. | "Use absolute paths: specify full paths for scripts. In exec form, use `${CLAUDE_PROJECT_DIR}` and the path needs no quoting." (a doc nunca recomenda caminho relativo puro para scripts) |

**Veredito: QUEBRADO** por padrão para caminho relativo puro (é o bug já corrigido em produção para o
credential-guard do Claude, commit `0c66ecb`). Mecanismo de correção disponível na própria doc:
`${CLAUDE_PROJECT_DIR}/...` (não é caminho absoluto hardcoded — é um placeholder expandido em
runtime, portanto não viola a restrição "sem caminho absoluto em arquivo versionado").

---

### 2. Codex CLI

Fontes: <https://developers.openai.com/codex/hooks> e <https://developers.openai.com/codex/config-advanced>

| Pergunta | Resposta | Citação literal |
|---|---|---|
| (a) cwd de execução | O `cwd` da sessão Codex. | "Commands run with the session `cwd` as their working directory." |
| (b) fixo ou dinâmico | Dinâmico em relação à raiz do repositório — Codex pode ser iniciado a partir de um subdiretório, então o cwd da sessão não é garantidamente a raiz do projeto. (A doc não afirma explicitamente que um `cd` do agente durante a sessão altera o cwd do hook; a evidência direta é sobre variação por diretório de início.) | "Codex may be started from a subdirectory, and a git-root-based path keeps the hook location stable." |
| (c) placeholders | **Não existe** variável de ambiente equivalente a `CLAUDE_PROJECT_DIR`/`GEMINI_PROJECT_DIR` para hooks de repositório comuns. A doc só documenta `PLUGIN_ROOT`/`PLUGIN_DATA` (e os aliases `CLAUDE_PLUGIN_ROOT`/`CLAUDE_PLUGIN_DATA`) para hooks de **plugin**, não para `hooks.json`/`[hooks]` de repositório. O mecanismo que a própria doc recomenda para hooks de repositório é substituição de shell via `$(git rev-parse --show-toplevel)` dentro do campo `command`. | "Plugin hook commands receive these environment variables: `PLUGIN_ROOT` is a Codex-specific extension..." (restrito a plugins) · exemplo TOML: `command = '/usr/bin/python3 "$(git rev-parse --show-toplevel)/.codex/hooks/pre_tool_use_policy.py"'` · "For repo-local hooks, prefer resolving from the git root instead of using a relative path such as `.codex/hooks/...`." |
| (d) relativo resolve contra | Contra o cwd da sessão — daí a recomendação de não usar caminho relativo e sim resolução via git root. Nota separada (config-advanced): para *config* de projeto (ex.: `model_instructions_file`), caminhos relativos resolvem contra a pasta `.codex/` — mas essa regra é documentada para chaves de `config.toml`, não para o campo `command` de hooks; não deve ser extrapolada. | "For repo-local hooks, prefer resolving from the git root instead of using a relative path such as `.codex/hooks/...`." · (config.toml, escopo distinto) "Relative paths inside a project config (for example, `model_instructions_file`) are resolved relative to the `.codex/` folder that con[tains it]" |

**Veredito: QUEBRADO.** Confirmado pela própria doc, que já recomenda evitar caminho relativo.
Mecanismo de correção disponível: `$(git rev-parse --show-toplevel)` embutido no campo `command`
(substituição de shell resolvida em runtime — não é caminho absoluto hardcoded no arquivo).
Restrição: depende de o hook rodar dentro de um repositório git (aceitável para o caso de uso do
trackfw).

---

### 3. Gemini CLI

Fontes: <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/index.md>,
<https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md>,
<https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/best-practices.md>

| Pergunta | Resposta | Citação literal |
|---|---|---|
| (a) cwd de execução | Não há frase única e explícita do tipo "hooks executam com cwd = X". Evidência direta e primária, mas indireta: o payload de entrada de todo hook traz um campo `cwd` distinto do env var `GEMINI_PROJECT_DIR`, e a lista de env vars documenta separadamente `GEMINI_CWD` (`"The current working directory"`) e `GEMINI_PROJECT_DIR` (`"The absolute path to the project root"`) como duas variáveis diferentes — o que só faz sentido de existir se puderem divergir. | reference.md: `"cwd": string, // Current working directory` (campo de input, `hooks/reference.md`) · index.md, seção "Environment variables": "`GEMINI_PROJECT_DIR`: The absolute path to the project root." / "`GEMINI_CWD`: The current working directory." |
| (b) fixo ou dinâmico | **INDETERMINADO** de forma explícita — a doc não afirma literalmente que o cwd acompanha os `cd` do agente durante a sessão. A evidência de (a) aponta nessa direção mas não é uma afirmação direta sobre dinamismo intra-sessão. Procurado em `reference.md`, `index.md`, `writing-hooks.md`, `best-practices.md`: nenhuma frase equivalente à de Claude ("regardless of the working directory when the hook runs") ou de Codex ("Codex may be started from a subdirectory"). | — (busca sem resultado; ver acima os arquivos consultados) |
| (c) placeholders | `$GEMINI_PROJECT_DIR` — forma `$VAR` em shell, expandido no campo `command`. Alias documentado: `CLAUDE_PROJECT_DIR` também é setado "for compatibility". Todo exemplo de hook script na doc usa `$GEMINI_PROJECT_DIR/...`, nunca caminho relativo puro. | index.md, exemplo de config: `"command": "$GEMINI_PROJECT_DIR/.gemini/hooks/security.sh"` · "`CLAUDE_PROJECT_DIR`: (Alias) Provided for compatibility." · best-practices.md, troubleshooting: `echo "$GEMINI_PROJECT_DIR/.gemini/hooks/my-hook.sh"` |
| (d) relativo resolve contra | **INDETERMINADO** de forma explícita — nenhuma frase primária diz contra o quê um caminho relativo puro no campo `command` é resolvido. Procurado nos mesmos 4 arquivos por "relative path"/"resolved"/"portable"/"hardcode": sem ocorrência. | — (busca sem resultado) |

**Veredito: QUEBRADO** (por evidência primária indireta, não por analogia com outro CLI): a própria
doc distingue `GEMINI_CWD` de `GEMINI_PROJECT_DIR` como dois valores potencialmente diferentes, e
100% dos exemplos de hook script na doc usam `$GEMINI_PROJECT_DIR` em vez de caminho relativo —
padrão de uso consistente com "caminho relativo não é confiável". (a)/(b)/(d) ficam
**parcialmente INDETERMINADO** quanto à frase explícita de dinamismo; isso não muda o veredito
porque o mecanismo de correção (c) está plenamente confirmado e documentado com exemplos.
Mecanismo de correção: `$GEMINI_PROJECT_DIR/...`.

---

### 4. Cursor

Fonte: <https://cursor.com/docs/hooks>

| Pergunta | Resposta | Citação literal |
|---|---|---|
| (a) cwd de execução | Depende da origem do hook. Para hooks de projeto (`.cursor/hooks.json` no repositório, que é o caso do trackfw): a raiz do projeto. | "Project hooks (`.cursor/hooks.json` in a repository): Run from the **project root**" |
| (b) fixo ou dinâmico | **Fixo** na raiz do projeto para hooks de projeto — não descrito como acompanhando `cd`s do agente. | mesma citação acima; a doc lista 4 origens de hook (Project/User/Enterprise/Team), cada uma com working directory fixo e documentado, não dinâmico por sessão. |
| (c) placeholders | `CURSOR_PROJECT_DIR` — env var, "Workspace root directory", sempre presente ("Always Present": Yes). | tabela de env vars: `CURSOR_PROJECT_DIR` \| "Workspace root directory" \| "Yes" |
| (d) relativo resolve contra | Contra a raiz do projeto (para hooks de projeto) — a doc dá o exemplo exato do erro a evitar. | "For project hooks, use paths like `.cursor/hooks/script.sh` (relative to project root), not `./hooks/script.sh` (which would look for `<project>/hooks/script.sh`)." |

**Veredito: OK.** cwd fixo na raiz do projeto para hooks de projeto; caminho relativo já resolve
corretamente contra a raiz. Nenhuma mudança necessária no mecanismo de resolução (o `check-agent-hooks-parity` e os geradores continuam livres para emitir caminho relativo puro aqui).

---

### 5. Kiro

Fontes: <https://kiro.dev/docs/hooks/>, <https://kiro.dev/docs/hooks/types/>,
<https://kiro.dev/docs/hooks/actions/>, <https://kiro.dev/docs/hooks/troubleshooting/>

| Pergunta | Resposta | Citação literal |
|---|---|---|
| (a) cwd de execução | **INDETERMINADO.** A doc de `hooks/actions/` (seção "Shell Command action", que documenta o campo `command` da action de hook) não menciona em nenhum momento o diretório de trabalho do processo executado — só descreve o contrato de exit code/stdout/stderr e timeout. | "With this action, you can define a shell command that is executed each time the hook is triggered." · "If the command returns an exit code of '0' indicating success, the *stdout* output of the command is added to the agent's context." (nenhuma menção a cwd na mesma seção) |
| (b) fixo ou dinâmico | **INDETERMINADO** — mesma ausência acima. `hooks/types/` só documenta um campo `cwd` no **payload de stdin JSON** recebido pelo hook (`"cwd": "/current/working/directory"` — placeholder de exemplo, não um valor real), que é informativo (o hook pode ler onde o Kiro estava quando disparou), não uma afirmação sobre onde o comando `action.command` é executado. | `hooks/types/`, exemplos de payload `agentSpawn`/`userPromptSubmit`/`stop`/`postToolUse`: `{"hook_event_name": "...", "cwd": "/current/working/directory", ...}` |
| (c) placeholders | **INDETERMINADO** — nenhuma variável de ambiente do tipo `KIRO_PROJECT_DIR`/`${workspaceFolder}` foi encontrada em `hooks/`, `hooks/types/`, `hooks/actions/` ou `hooks/troubleshooting/`. Buscado por: `PROJECT_DIR`, `workspaceFolder`, `KIRO_`, `${`. Nenhuma ocorrência fora de código de infraestrutura da própria página (CSS/JS). | — (busca sem resultado nas 4 páginas) |
| (d) relativo resolve contra | **INDETERMINADO** — nenhuma citação sobre contra o que um caminho relativo no campo `command` da action é resolvido. Buscado por "relative", "resolved", "working directory" nas 4 páginas; a única ocorrência de "relative to" nas páginas consultadas é de CSS (`position: relative`), não de documentação textual sobre paths. | — (busca sem resultado) |

**Veredito: INDETERMINADO.** Resultado esperado pelo roadmap (§ "Default para INDETERMINADO").
Ação: nenhuma mudança de mecanismo para Kiro; registrar em `docs/cli-parity.md` como "mecanismo de
resolução não verificável em doc primária — mantido relativo", com data 2026-08-11 e as 4 URLs acima
como o que foi consultado.

---

### 6. GitHub Copilot CLI

Fonte: <https://docs.github.com/en/copilot/reference/hooks-reference>

| Pergunta | Resposta | Citação literal |
|---|---|---|
| (a) cwd de execução | Configurável por hook via o campo opcional `cwd` do próprio JSON do hook. Se omitido, a doc não declara explicitamente o padrão (não localizado). | Tabela "Command hooks" (campos do objeto de comando): `cwd` \| `string` \| `No` \| "Working directory for the command (relative to repository root or absolute)." |
| (b) fixo ou dinâmico | **Fixo, se o campo `cwd` for setado** (é um campo de configuração estática do JSON versionado, não deriva do cwd do processo agente). Sem o campo, o comportamento padrão não está documentado explicitamente (INDETERMINADO só para esse sub-caso). | mesma citação acima |
| (c) placeholders | Não é um placeholder de string (`${VAR}`) — é um **campo estruturado dedicado** `cwd` que aceita valor relativo à raiz do repositório ou absoluto. Exemplo oficial de schema mostra `"cwd": "OPTIONAL/WORKING/DIRECTORY"` ao lado de `bash`/`powershell`/`command`. | exemplo de `preToolUse` no schema oficial: `{"type": "command", "bash": "YOUR_BASH_COMMAND", "powershell": "YOUR_POWERSHELL_COMMAND", "cwd": "OPTIONAL/WORKING/DIRECTORY", "env": {...}, "timeoutSec": 30}` |
| (d) relativo resolve contra | Um valor relativo no campo `cwd` resolve contra a **raiz do repositório**, conforme a própria descrição do campo. O campo `bash`/`command` (o script a rodar) passa a ser executado dentro desse `cwd`, então um caminho relativo dentro de `bash`/`command` (ex.: `scripts/foo.sh`) resolve, por semântica padrão de shell, contra o `cwd` já pinado. | "Working directory for the command (relative to repository root or absolute)." |

**Veredito: OK — já corrigido no código atual do trackfw.** `InjectCopilotHooks`
(`internal/generators/agentfiles.go:687`) já emite `"cwd": "."` em **todas** as entradas de hook do
Copilot (signal, cleanup, credential-guard bash/view/create-edit), o que — pela própria semântica
documentada do campo — pina a execução na raiz do repositório independentemente de onde o agente
tenha feito `cd`. Nenhuma mudança de mecanismo necessária para este CLI; o comando dentro de `bash`
(ex.: `scripts/trackfw-attention-signal.sh`) já resolve corretamente contra a raiz do repo graças ao
`cwd: "."` que já está presente. (Achado que contraria uma premissa implícita do roadmap: nem todos
os 6 CLIs precisam de correção — Copilot já está correto por já usar o campo `cwd` nativo do
fornecedor, não por acidente.)

---

## Resumo de veredito por CLI

| CLI | Veredito | Mecanismo de correção (se QUEBRADO) |
|---|---|---|
| Claude Code | QUEBRADO (por padrão; credential-guard já corrigido) | `${CLAUDE_PROJECT_DIR}/...` |
| Codex CLI | QUEBRADO | `$(git rev-parse --show-toplevel)` embutido no campo `command` |
| Gemini CLI | QUEBRADO (evidência primária indireta — ver ressalva na seção 3) | `$GEMINI_PROJECT_DIR/...` |
| Cursor | OK | — (relativo já resolve contra a raiz do projeto) |
| Kiro | INDETERMINADO | — (mantido relativo; ver nota `docs/cli-parity.md`) |
| GitHub Copilot CLI | OK (já corrigido no código atual) | — (`cwd: "."` já emitido por `InjectCopilotHooks`) |

## Achados que contrariam ou refinam a hipótese do roadmap

1. **Copilot não precisa de Wave de correção.** A hipótese do roadmap tratava os 6 CLIs como
   candidatos a mudança; a auditoria deste ML mostra que o Copilot já está `OK` no código atual —
   `InjectCopilotHooks` já usa o campo nativo `cwd: "."`, documentado pelo próprio GitHub como
   relativo à raiz do repositório. Não há string a trocar nem migração a generalizar para este CLI.
2. **Gemini tem mecanismo confirmado (c) mas não uma frase explícita de dinamismo (a/b/d).** A
   evidência é primária e direta (duas env vars distintas documentadas, 100% dos exemplos de hook
   script usando `$GEMINI_PROJECT_DIR`), mas não uma sentença única do tipo "hooks run in the
   current directory". Zeus deve decidir se isso é evidência suficiente para tratar Gemini como
   `QUEBRADO` na Wave de emissão, ou se prefere marcar como cautelarmente `INDETERMINADO` até haver
   confirmação empírica (teste reproduzível), conforme a cláusula de "default para INDETERMINADO"
   do roadmap permite sobrepor por CLI com evidência direta.
3. **Codex não tem env var de project-dir para hooks comuns** — confirma o indício preliminar do
   roadmap. O único mecanismo documentado é a substituição de shell via `git rev-parse
   --show-toplevel`, o que implica uma dependência (repositório git) que não existe para os outros
   CLIs corrigidos via env var.
4. **Kiro caiu em `INDETERMINADO` como esperado** — a doc de hooks (`hooks/`, `hooks/types/`,
   `hooks/actions/`, `hooks/troubleshooting/`) nunca discute cwd de execução do comando nem
   variáveis de ambiente de raiz de projeto para a "Shell Command action".

## Fontes consultadas

- <https://code.claude.com/docs/en/hooks> — respondeu (a)–(d).
- <https://developers.openai.com/codex/hooks> — respondeu (a)–(d).
- <https://developers.openai.com/codex/config-advanced> — contexto adicional sobre resolução de
  caminhos relativos em `.codex/config.toml` (escopo de config, não de hooks — citado com a ressalva
  explícita de não ser extrapolável).
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/index.md> — respondeu (c);
  parcial para (a)/(b)/(d).
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md> — consultado para
  (a)/(b)/(d); sem resultado direto além do campo de input `cwd`.
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/writing-hooks.md> — consultado
  para (a)/(b)/(d); sem resultado.
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/best-practices.md> — consultado
  para (a)/(b)/(d); sem resultado direto; confirmou padrão de uso de `$GEMINI_PROJECT_DIR` em
  troubleshooting.
- <https://cursor.com/docs/hooks> — respondeu (a)–(d).
- <https://kiro.dev/docs/hooks/> — consultado; não respondeu (a)/(b)/(c)/(d) para a Shell Command
  action.
- <https://kiro.dev/docs/hooks/types/> — consultado; só documenta o schema de payload de stdin
  (inclui um campo `cwd` informativo do lado do *input*, não do processo executado), sem resposta a
  (a)/(b)/(c)/(d).
- <https://kiro.dev/docs/hooks/actions/> — consultado; documenta a Shell Command action (contrato de
  exit code/stdout/stderr/timeout) sem mencionar cwd de execução nem placeholders — sem resposta a
  (a)/(b)/(c)/(d).
- <https://kiro.dev/docs/hooks/troubleshooting/> — consultado; sem resultado para (a)/(b)/(c)/(d).
- <https://docs.github.com/en/copilot/reference/hooks-reference> — respondeu (a)–(d) via o campo
  estruturado `cwd`.
