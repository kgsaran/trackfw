# Pesquisa: semântica de falha de hook (fail-open vs fail-closed) — varredura documental em 5 CLIs

> ML-1B do roadmap
> `docs/roadmaps/wip/ROADMAP-2026-08-12-semantica-de-falha-de-hook-fail-open-vs-fail-closed-nucleo-empirico-no-codex.md`.
> Escopo: **Claude Code · Gemini CLI · Cursor · GitHub Copilot CLI · Kiro** — varredura **documental**
> contra documentação primária do fornecedor. O Codex está fora (prova **empírica**, ML-1A, agente
> Ártemis, escreve em `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md` — arquivo não
> tocado por este ML).
>
> Toda afirmação abaixo é sustentada por URL + citação literal, retirada em 2026-08-12. Onde a doc
> não responde, a célula é `INDETERMINADO` com registro do que foi procurado — sem inferência por
> analogia entre CLIs (mesma regra de evidência da pesquisa ML-0A,
> `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`).

## A pergunta

O `scripts/trackfw-credential-guard.sh` é um controle de negação. Um controle de negação só vale se
falhar **fechado**. Duas perguntas, tratadas separadamente porque podem ter semânticas diferentes:

- **Caso A** — o `command` do hook **não resolve** (script ausente / caminho inválido): a ferramenta
  prossegue ou é bloqueada?
- **Caso B** — o hook **roda e sai com código != 0**: prossegue ou é bloqueado? Há distinção entre
  `exit 1` e `exit 2`?

---

## Tabela 5×2

| CLI | Caso A (comando não resolve) | Caso B (hook roda, exit != 0) |
|---|---|---|
| **Claude Code** | **Fail-open.** Cai no mesmo "non-blocking bucket" de qualquer falha de start. | **Distingue exit 1 de exit 2.** `exit 1` (ou qualquer não-2 sem JSON válido) = fail-open. `exit 2` = fail-closed (bloqueia). |
| **Gemini CLI** | `INDETERMINADO` — doc não afirma o que ocorre quando o comando não consegue nem iniciar. | **Não distingue exit 1 de outros não-2** — qualquer código fora de `{0,2}` cai no bucket `Other` = fail-open (warning, ação prossegue). `exit 2` = fail-closed (System Block). |
| **Cursor** | **Fail-open por padrão** (com a mesma ressalva de interpretação do Copilot) — "crash" está explicitamente listado ao lado de "timeout, invalid JSON" no mesmo enunciado de fail-open padrão; a doc não usa literalmente "script not found"; `failClosed: true` inverte por hook. | **Fail-open por padrão** para qualquer código != 2; `exit 2` = fail-closed (bloqueia). Não distingue `exit 1` de outros não-2/não-timeout — todos caem no mesmo bucket "Other exit codes". Opt-in `failClosed: true` inverte o padrão para o hook. |
| **GitHub Copilot CLI** | **Fail-closed para `preToolUse`** — "crash" está explicitamente listado ao lado de "non-zero exit" no enunciado de fail-closed (exceção: timeout continua fail-open). Ressalva: a doc não usa literalmente "script not found"/"comando não resolve"; usa "crash", que é o guarda-chuva sob o qual um shell não conseguindo iniciar o processo cairia — ver nota de interpretação abaixo. | **Fail-closed para `preToolUse`** — `exit 2` e "qualquer outro exit não-zero (exceto timeout)" bloqueiam igualmente; a doc não distingue `exit 1` de `exit 2` em termos de resultado (ambos negam), só na origem da mensagem (`exit 2` sempre nega mesmo se o JSON disser `permissionDecision: "allow"`). Para a maioria dos outros eventos (não-`preToolUse`), o padrão é fail-open. |
| **Kiro** | `INDETERMINADO` — nem a aba IDE nem a aba CLI de `hooks/actions/` discutem o caso em que o **comando em si não consegue rodar** (script ausente, caminho inválido). | **Depende da superfície: IDE é fail-closed sem distinguir `exit 1`/`exit 2`; CLI distingue `exit 1` de `exit 2` como Claude — só `exit 2` bloqueia, `exit 1`/outros são fail-open.** A página tem abas "IDE"/"CLI" com textos **diferentes** para a mesma seção; achado que não estava na pesquisa ML-0A de 2026-08-11 (aquela pesquisa não abriu essa distinção). |

---

## 1. Claude Code

Fonte: <https://code.claude.com/docs/en/hooks>, seção "Exit code output" / "Exit code 0" / "Exit code 2".
Consultada em 2026-08-12 (mesma doc primária já usada na pesquisa ML-0A de 2026-08-11).

### Caso A — comando não resolve

> "A hook that can't start lands in the same non-blocking bucket. When the script doesn't exist or
> isn't executable, the shell exits with a code like 127 and you see the same notice with the
> interpreter's message, for example `Failed with non-blocking status code:
> /bin/sh: /path/to/hook.sh: No such file or directory`. For most hook events, the action proceeds."

**Veredito: fail-open.** A própria doc usa o exemplo canônico de "script ausente" (`No such file or
directory`) e afirma explicitamente que, para a maioria dos eventos de hook, a ação prossegue.

Citação adicional, mais específica de gate/controle de negação (a doc fala do próprio caso de uso
"policy hook", que é a categoria do credential-guard):

> "When you set up a policy hook, watch for this notice on its first run: a mistyped path in
> settings.json leaves the gate silently disabled."

Esta segunda citação é a mais diretamente aplicável ao roadmap: é o fornecedor descrevendo, nas
próprias palavras, que um **gate de negação** (a categoria de hook a que o credential-guard pertence)
fica **silenciosamente desabilitado** quando o caminho não resolve — não apenas um exemplo genérico
de erro, mas o cenário exato ("policy hook" com "mistyped path") que este roadmap investiga.

### Caso B — hook roda, exit != 0

> "For most hook events, exit code 2 is the only exit code that blocks through the code alone.
> Without valid JSON on stdout, Claude Code treats exit code 1 as a non-blocking error and proceeds
> with the action even though 1 is the conventional Unix failure code. If your hook is meant to
> enforce a policy, use exit 2."

> "Exit 2 means a blocking error. On events that can block, exit 2 blocks whether or not you print
> JSON: even a JSON permissionDecision of 'allow' can[not override it]."

Tabela "Exit code 2 behavior per event", linha específica de `PreToolUse` (o evento relevante para o
credential-guard, verificada nominalmente e não por analogia com outra linha da mesma tabela):

> "Hook event | Can block? | What happens on exit 2
> PreToolUse | Yes | Blocks the tool call
> PermissionRequest | No | Exit code 2 isn't honored for this event and the permission flow proceeds
> unchanged. Deny through the decision object instead"

**Veredito: distingue exit 1 de exit 2, e a linha de `PreToolUse` confirma bloqueio (não a linha
"No" — essa é `PermissionRequest`, um evento diferente).** `exit 1` = fail-open (a doc cita
literalmente esse código como exemplo do que **não** bloqueia). `exit 2` = fail-closed em
`PreToolUse` especificamente, e é blindado contra sobrescrita por JSON — é o único código com
garantia de bloqueio "through the code alone".

**Relevância direta para o `trackfw-credential-guard.sh`:** o gerador emite `exit 2` no modo block
(`internal/generators/scaffold.go`, comentário "bloquear (`credential_guard.mode: block`, exit 2)" —
lido diretamente do código-fonte do próprio trackfw, não inferido) — está no lado correto da
distinção que Claude documenta para o evento `PreToolUse`.

---

## 2. Gemini CLI

Fontes: <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/index.md>,
<https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md>,
<https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/best-practices.md>,
<https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/writing-hooks.md>.
Consultadas em 2026-08-12.

### Caso A — comando não resolve

**`INDETERMINADO`.** Buscado por "not found", "ENOENT", "no such file", "spawn", "command not
found", "invalid path" nos 4 arquivos acima — nenhuma ocorrência textual sobre o que acontece quando
o processo do hook **não consegue nem iniciar**. A seção de troubleshooting do `best-practices.md`
("Hook not executing") só orienta a **verificar** se o caminho resolve corretamente:

> "**Verify script path:** Ensure the path in `settings.json` resolves correctly."

— mas não declara o resultado (bloqueia ou prossegue) quando o caminho está errado. Isso é uma
lacuna documental real, não uma inferência recusada: a doc trata o tópico "exit codes" (que resolve o
Caso B) como algo distinto de "o comando não iniciou", e nunca junta os dois.

### Caso B — hook roda, exit != 0

`docs/hooks/reference.md`:

> "- **Exit codes**:
>   - `0`: Success. `stdout` is parsed as JSON. **Preferred for all logic.**
>   - `2`: System Block. The action is blocked; `stderr` is used as the rejection reason.
>   - `Other`: Warning. A non-fatal failure occurred; the CLI continues with a warning."

`docs/hooks/index.md`, tabela equivalente:

> "| **0** | **Success** | The `stdout` is parsed as JSON... |
> | **2** | **System Block** | **Critical Block**. The target action (tool, turn, or stop) is
> aborted. `stderr` is used as the rejection reason. High severity; used for security stops or
> script failures. |
> | **Other** | **Warning** | Non-fatal failure. A warning is shown, but the interaction proceeds
> using original parameters. |"

**Veredito: não distingue `exit 1` de outros não-2 — fail-open para qualquer coisa fora de `{0,2}`.**
Diferente de Claude (que cita `exit 1` nominalmente como exemplo do bucket fail-open), a doc do Gemini
usa só o rótulo genérico `Other`. Não há citação que trate `exit 1` de forma diferente de, por
exemplo, `exit 3` ou `exit 127` — todos caem no mesmo bucket "Warning / interaction proceeds".
`exit 2` é o único código com efeito de bloqueio documentado ("System Block... used for security
stops **or script failures**" — nota: essa frase da tabela do `index.md` sugere que até falhas de
script poderiam, em tese, ser sinalizadas via `exit 2` pelo autor do hook, mas isso é o hook **decidindo**
sair com 2, não o comportamento padrão do CLI diante de uma falha de execução — não deve ser
confundido com o Caso A).

---

## 3. Cursor

Fonte: <https://cursor.com/docs/hooks>. Consultada em 2026-08-12 (mesma doc primária da pesquisa
ML-0A).

### Caso A — comando não resolve

> "By default, hook failures (crash, timeout, invalid JSON) allow the action through (fail-open). Set
> `failClosed: true` on the hook definition to block the action on failure instead. This is
> recommended for security-critical `beforeMCPExecution` hooks."

> "`failClosed` | `boolean` | `false` | When `true`, hook failures (crash, timeout, invalid JSON)
> block the action instead of allowing it through. Useful for security-critical hooks."

**Veredito: fail-open por padrão.** Ressalva de interpretação (mesma reserva registrada para Copilot
abaixo, aplicada aqui pela consistência de critério deste ML): a doc usa literalmente "crash", não
"script not found"/"comando não resolve". A ponte entre os dois — um comando cujo caminho não
resolve tipicamente falha no próprio `spawn` do processo, o que se enquadraria em "crash" — é uma
inferência sobre o significado do termo do fornecedor, não uma citação literal do cenário exato do
Caso A. A doc agrupa "crash" explicitamente junto de "timeout" e "invalid JSON" no mesmo enunciado de
fail-open padrão, e não lista "path not found"/"script missing" como categoria própria e distinta. O
padrão é invertível por hook via `failClosed: true` — mecanismo nativo e documentado, não um flag
inferido.

Nota de matiz: para `beforeReadFile` especificamente, a doc usa redação um pouco diferente mas o
mesmo sentido:

> "By default, `beforeReadFile` hook failures (crash, timeout, invalid JSON) are logged and the read
> is allowed through. Set `failClosed: true` on the hook definition to block the read on failure
> instead."

### Caso B — hook roda, exit != 0

> "Exit code behavior: Exit code 0 - Hook succeeded, use the JSON output. Exit code 2 - Block the
> action (equivalent to returning `permission: "deny"`). Other exit codes - Hook failed, action
> proceeds (fail-open by default)."

**Veredito: fail-open por padrão para qualquer código != 2; não distingue `exit 1` de outros não-2.**
`exit 2` bloqueia; "Other exit codes" (bucket único, sem menção nominal a `exit 1`) faz a ação
prosseguir — mesmo padrão do Caso A, e mesmo mecanismo de opt-out (`failClosed: true`) documentado
para inverter.

---

## 4. GitHub Copilot CLI

Fonte: <https://docs.github.com/en/copilot/reference/hooks-reference>. Consultada em 2026-08-12
(mesma doc primária da pesquisa ML-0A).

### Caso A — comando não resolve

> "Important — Command vs HTTP fail behavior for `preToolUse`: Command `preToolUse` hooks are
> **fail-closed on errors**—a crash or non-zero exit (including exit 2) denies the tool call, even if
> the hook's stdout JSON reports `permissionDecision: 'allow'`."

> "For `preToolUse` command hooks, exit 2, crashes, and other non-zero exits all fail-closed and deny
> the tool call—exit 2 always denies, even if the hook's stdout JSON reports `permissionDecision:
> 'allow'`. ... Timeouts are always fail-open, including for `preToolUse` and admin-deployed policy
> hooks: a warning is surfaced and the tool call proceeds through the normal permission flow rather
> than being denied."

**Veredito: fail-closed para `preToolUse` (o evento relevante para o credential-guard).** Ressalva de
interpretação, registrada por honestidade de evidência: a doc usa literalmente a palavra "crash", não
"script not found" ou "command not found". Um comando cujo caminho não resolve (`ENOENT` no
`execve`) tipicamente produz uma falha de spawn do processo — que a doc trata sob o mesmo guarda-chuva
de "crash" usado ao lado de "non-zero exit" em todas as três citações acima, e distingue
explicitamente apenas o caso de **timeout** como exceção fail-open. Não há uma frase dedicada e
literal a "comando ausente/caminho inválido" — por isso o veredito é dado com essa ressalva, e não
como uma citação 1:1 ao enunciado do Caso A tal como perguntado.

### Caso B — hook roda, exit != 0

Tabela de exit codes da doc:

> "| **Meaning** |
> | `0` | Success. `stdout` is parsed as the hook output JSON if present. |
> | `2` | Treated as a warning by default. `stderr` is surfaced to the user but the run continues.
> For `permissionRequest` and `preToolUse`, exit 2 is treated as a deny: any stdout JSON is merged
> with the deny decision and the tool call is denied eve[n if permissionDecision reports allow]. |
> | `Other non-zero` | Logged as a hook failure. The run continues (fail-open). Exception:
> `preToolUse` is fail-closed—a non-zero exit (other than exit 2) denies the tool call... |"

**Veredito: para eventos em geral, `exit 2` é warning-only e `Other non-zero` é fail-open — exceto
`preToolUse`, onde ambos (exit 2 e qualquer outro não-zero) fail-closed.** Não há distinção de
resultado entre `exit 1` e `exit 2` dentro de `preToolUse`: os dois negam a chamada. A única distinção
de origem é que `exit 2` "always denies" mesmo que o JSON diga `allow`, enquanto para outros
não-zero a negação decorre do próprio código de saída não-zero (sem essa blindagem explícita contra
JSON contraditório citada para o caso genérico) — mas o resultado prático (bloqueio) é o mesmo.

---

## 5. Kiro

Fontes: <https://kiro.dev/docs/hooks/actions/>, <https://kiro.dev/docs/hooks/>,
<https://kiro.dev/docs/hooks/troubleshooting/>. Consultadas em 2026-08-12 (mesmas 3 das 4 URLs da
pesquisa ML-0A de 2026-08-11 — `hooks/types/` não trouxe achado novo sobre exit code e não foi
re-citada aqui).

### Caso A — comando não resolve

**`INDETERMINADO`.** A seção "Shell Command action" de `hooks/actions/` documenta o contrato de exit
code em detalhe (ver Caso B abaixo), mas nunca menciona o que acontece se o **comando em si não
conseguir rodar** (script ausente, caminho inválido, sem permissão de execução). A seção
"Troubleshooting hooks" (`hooks/troubleshooting/`) tem um item "Shell command errors" que orienta
diagnóstico, mas não declara o comportamento resultante:

> "Shell command errors — Verify the command works when run manually in a terminal. Check that all
> required environment variables are available. Ensure any external tools or dependencies are
> installed. Review STDERR output for error messages."

Buscado adicionalmente por "not found", "ENOENT", "no such file", "invalid path", "127" nas 3 páginas
— sem ocorrência textual sobre o resultado (bloqueia vs prossegue) desse cenário.

### Caso B — hook roda, exit != 0

**Achado com uma armadilha metodológica importante, corrigida nesta versão do documento**: a página
`hooks/actions/` renderiza duas abas de conteúdo, "IDE" e "CLI", para a mesma seção "Shell Command
action", com **textos diferentes**. A primeira leitura deste ML citou só o texto da aba IDE; abaixo
estão os dois, extraídos separadamente do HTML servido (confirmado por inspeção do payload React —
os dois blocos aparecem em painéis `tabpanel` distintos, `value:"ide"` e `value:"cli"`, na mesma
resposta HTTP).

**Aba IDE** (`hooks/actions/`, painel `tabpanel value="ide"`):

> "If the command returns an exit code of '0' indicating success, the stdout output of the command is
> added to the agent's context. If the command returns any other exit code, the stderr output of the
> command is sent to the agent, and the agent is notified that the hook returned an error.
> Additionally, in the case of the Pre Tool Use hook, the tool invocation is blocked, and for the
> Prompt Submit hook, the user prompt submission is blocked."

Veredito da aba IDE: **fail-closed para `PreToolUse`** (e para `Prompt Submit`). Sem distinção entre
`exit 1` e `exit 2` — "any other exit code" é categoria única; qualquer saída não-zero basta para
bloquear.

**Aba CLI** (mesma página, painel `tabpanel value="cli"`) — texto diferente, não capturado pela
leitura inicial deste ML:

> "Hooks receive hook event data in JSON format via STDIN. The output behavior depends on the exit
> code:
> - **Exit code 0**: Hook succeeded. STDOUT is added to context (SessionStart, UserPromptSubmit) or
>   ignored (others).
> - **Exit code 2**: Block execution (PreToolUse, UserPromptSubmit, PreTaskExec only). STDERR is
>   returned to the agent.
> - **Other exit codes**: Hook failed. STDERR is shown as warning to user and execution proceeds."

Veredito da aba CLI: **distingue `exit 1` de `exit 2`, no mesmo padrão de Claude/Gemini/Cursor.**
Só `exit 2` bloqueia `PreToolUse`; qualquer outro código não-zero (incluindo `exit 1`) é fail-open
("execution proceeds").

**Veredito consolidado do Caso B para Kiro: depende da superfície de execução, e as duas abas
discordam entre si.** Isso não é um erro de leitura resolvido para um único valor — é uma
característica real da doc: o fornecedor documenta comportamentos **diferentes** para Kiro IDE e Kiro
CLI na mesma página. Qual das duas se aplica ao trackfw depende de qual superfície do Kiro consome o
`hooks.json`/config gerado pelo trackfw (Kiro IDE vs Kiro CLI) — essa determinação está **fora do
escopo documental deste ML** (é uma pergunta sobre o próprio trackfw, não sobre a doc do fornecedor) e
fica registrada aqui como ambiguidade a resolver antes de qualquer decisão de correção para Kiro.

**Implicação para o `trackfw-credential-guard.sh`:** a hipótese cautelosa de tratar Kiro como "dívida
totalmente desconhecida" (`docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, Q3) permanece válida para
Kiro CLI (fail-open em `exit 1`, mesma classe de risco que motivou a atenção a Claude/Gemini/Cursor) e
fica **refutada** para Kiro IDE (fail-closed nativo para qualquer exit != 0, sem exigir configuração
extra). Não decidir qual API o trackfw usa é, por si só, um gap a fechar antes de qualquer mudança de
código — não deste ML documental.

---

## Raciocínio registrado: por que Claude e Gemini não tiveram verificação empírica neste roadmap

Item obrigatório do prompt deste ML. Fonte: `docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, Q2.

A pergunta que motivaria uma verificação empírica seria: "se `$CLAUDE_PROJECT_DIR` ou
`$GEMINI_PROJECT_DIR` expandirem para vazio/indefinido, o comando resultante pode acidentalmente
apontar para um script **diferente e controlável por terceiro** (fail-to-wrong-script), tornando o
Caso A um vetor de bypass silencioso mais perigoso do que uma simples falha de execução?" A revisão de
segurança de 2026-08-11 (Q2) já respondeu essa pergunta especificamente, com evidência primária:

> "**Degradação sob variável indefinida**: se `$CLAUDE_PROJECT_DIR` ou `$GEMINI_PROJECT_DIR`
> expandirem para vazio, o comando vira `/scripts/trackfw-<script>.sh` — um caminho absoluto na raiz
> do **sistema de arquivos**, não do projeto. Isso não é 'algo perigoso' no sentido de apontar para um
> script controlável por terceiro: nenhuma parte não-privilegiada consegue plantar um arquivo em
> `/scripts/`. A degradação é sempre **fail-to-run** (arquivo não encontrado), nunca
> fail-to-wrong-script."

Ou seja: a única forma conhecida e documentada de o comando "não resolver" para Claude/Gemini
(variável de projeto indefinida) já foi provada — por análise estática do próprio texto do comando
gerado, não por analogia — como resultando em um caminho absoluto na raiz do sistema de arquivos
(`/scripts/...`), onde nenhum processo sem privilégio de root consegue plantar um arquivo. Isso
elimina a única razão que justificaria pagar o custo de uma prova empírica neste roadmap: não há
hipótese viável de "o guard roda um script errado, mas plausível, plantado por um atacante" a
confirmar — o único desfecho possível do Caso A nesses dois CLIs é "comando não encontrado", cujo
resultado (fail-open, conforme Caso A documentado acima para ambos) já está coberto pela varredura
documental deste ML, sem necessidade de reproduzir o CLI real. Isso é uma decisão **reavaliável**: se
no futuro surgir evidência de que a variável de projeto pode, sob alguma condição ainda não mapeada,
resolver para um caminho não-`/scripts/...` controlável, a verificação empírica volta a ser
justificada.

Nota lateral, não obrigatória mas relevante para o mesmo raciocínio: para Claude, o Caso A e o Caso B
com `exit 1` convergem para o **mesmo veredito** (fail-open) por caminhos documentais independentes —
o que reduz ainda mais a urgência de uma prova empírica adicional, já que dois pontos de evidência
primária concordam.

---

## Fontes consultadas

- <https://code.claude.com/docs/en/hooks> — respondeu Caso A e Caso B (Claude).
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/index.md> — respondeu Caso B
  (Gemini); consultado para Caso A sem resultado.
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md> — respondeu Caso B
  (Gemini); consultado para Caso A sem resultado.
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/best-practices.md> — consultado
  para Caso A (seção troubleshooting "Hook not executing"); sem resposta direta ao resultado
  (bloqueia/prossegue).
- <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/writing-hooks.md> — consultado
  para Caso A; sem resultado.
- <https://cursor.com/docs/hooks> — respondeu Caso A e Caso B (Cursor), incluindo o mecanismo de
  opt-out `failClosed`.
- <https://docs.github.com/en/copilot/reference/hooks-reference> — respondeu Caso A (com ressalva de
  interpretação sobre "crash") e Caso B (Copilot).
- <https://kiro.dev/docs/hooks/actions/> — respondeu Caso B (Kiro, achado novo), com respostas
  **diferentes** nas abas IDE e CLI da mesma seção (ver corpo do documento, seção 5); consultado para
  Caso A sem resultado em nenhuma das duas abas.
- <https://kiro.dev/docs/hooks/> — consultado para Caso A; sem resultado.
- <https://kiro.dev/docs/hooks/troubleshooting/> — consultado para Caso A (seção "Shell command
  errors"); sem resultado sobre o desfecho (bloqueia/prossegue).
- `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` (Q2) — fonte do raciocínio Claude/Gemini sobre
  ausência de verificação empírica (fail-to-run vs fail-to-wrong-script).
- `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` — ponto de partida (lista de URLs de
  hooks por CLI) e precedente metodológico (regra de evidência, formato de citação).
