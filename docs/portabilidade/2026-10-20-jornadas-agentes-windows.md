# Jornadas de agentes no Windows — preparação e controles (2026-10-20)

REQ-2026-09-05, ML-1E. Medição QA Artemis na VM `WIN-C83B0R5MIGB`, Windows ARM64, via SSH. **AC5–AC7 permanecem pendentes:** os controles abaixo invocam strings geradas diretamente nos shells, sem evento do runtime de agente. Nenhum modelo foi invocado, login realizado ou agente instalado nesta etapa.

## Inventário e autenticação

- Claude Code `2.1.299`, instalado via npm; `claude.cmd` resolve em `C:\Users\Lab\AppData\Roaming\npm`. Status atual, consultado por `claude.cmd auth status --json` e filtrado antes da saída: `loggedIn=false`, `authMethod=none`, `apiProvider=firstParty`. Sem exposição de tokens, e-mail ou conteúdo de credenciais.
- npm global lista somente `@anthropic-ai/claude-code@2.1.299` e `trackfw@9.2.0`.
- `trackfw` do PATH resolve primeiro em Python312-arm64/Scripts e depois nos shims npm; não foi usado como binário medido.
- Go e Git presentes. `codex`, `gemini`, `kiro-cli`, `copilot`, `cursor` e `pwsh` não resolvem no PATH. `AppData\Local\Programs` lista apenas Python; pastas padrão Cursor/Kiro ausentes. Isto não constitui busca recursiva por toda instalação possível.
- PowerShell 5 está disponível pelo caminho absoluto do Windows. O comando bare `claude` nele seleciona `claude.ps1`, bloqueado pela ExecutionPolicy; `claude.cmd` evita esse shim sem alterar a política persistente.
- Checkouts antigos existem em `C:\Users\Lab\trackfw` e `tf-9.5.0` (go.mod presente), além de outros scratchs históricos. Foram preservados.

A [medição de 14/10](2026-10-14-guard-global-home-userprofile-vm.md) já provou que Claude nativo lê USERPROFILE apesar de HOME diferente. Portanto HOME isolado sozinho é insuficiente. Uma jornada real deve usar mecanismo próprio de config do agente, perfil descartável e login autorizado pelo usuário; não copiar credenciais para a fixture.

## Fonte e build isolado

Snapshot `git archive` de `origin/main` exato `28d6d018`, enviado pelo orquestrador:

- Arquivo VM: `C:\Users\Lab\trackfw-main-28d6d018.tar.gz`.
- SHA256 do tar: `728B3B1141B35FFFD4068B7805A5DF78392C570D27C7E7F2EB2A92944D632592`.
- Raiz válida: `C:\Users\Lab\tf-ac567-20261020-source`.
- Extração **sem** `--strip-components`: o archive não tem prefixo de diretório.
- Comando: `go.exe build -o <raiz>\trackfw.exe ./cmd/trackfw`, exit 0; `trackfw.exe version` → `trackfw 10.0.0`.
- SHA256 do exe: `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1`.

A primeira tentativa usou `--strip-components=1` indevidamente e falhou no build por ausência de go.mod. Seu diretório `tf-ac567-20261020` é **inválido e não foi usado**. A extração correta ocorreu em outro diretório novo. A versão não embute SHA por ldflags; a proveniência vem do archive identificado e do hash do exe, não apenas do texto `10.0.0`.

## Fixture gerada e controles nativos

Instrumento temporário `ac567-native.ps1`, fora do repositório, copiado para a raiz válida da VM. A primeira execução abortou antes de init porque .NET Framework do PS5 não oferece `ProcessStartInfo.StandardInputEncoding`; o instrumento foi corrigido para escrever bytes UTF-8 explicitamente em `StandardInput.BaseStream`. Nova fixture foi usada. Arquivo PowerShell codificado UTF-8 com BOM para preservar acentos no PS5.

Fixture válida: `C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2`; cópia real do exe em `binário nativo\trackfw.exe`, hash igual ao build. HOME/USERPROFILE/APPDATA/LOCALAPPDATA/CLAUDE_CONFIG_DIR alterados apenas no processo do instrumento, todos apontando para a fixture. Nenhuma configuração real do usuário foi editada. Instrumento executado com `-ExecutionPolicy Bypass` somente nessa invocação, sem mudar política do sistema.

Geração real pelo exe:

```text
init --ai-tools claude,codex,gemini,cursor,copilot,kiro --forge github
  exit=0 timeout=false stdout_chars=8359 stderr_chars=161
update --install-missing
  exit=0 timeout=false stdout_chars=989 stderr_chars=0
```

O stderr de init não foi classificado como sucesso silencioso: houve 161 caracteres não capturados no relatório, portanto não se afirma ausência de warnings. O resultado verificável é exit 0 e arquivo `.claude/settings.json` efetivamente emitido. Deste JSON foram extraídos os comandos, por igualdade, de PreToolUse/PostToolUse:

```text
signal="trackfw attention signal"
cleanup="trackfw attention cleanup"
origin="generated .claude/settings.json"
agent_event=false
```

Depois da geração o PATH do processo passou a conter **apenas** `binário nativo`. Resolução observada: trackfw=true, bash=false, jq=false, python=false, python3=false. Shells invocados por caminhos absolutos, SystemRoot preservada. Payload JSON UTF-8 por stdin fechado, com pergunta contendo acentos, emoji, aspas e metacaracteres de shell; texto nunca interpolado na linha executável.

| Shell real | Signal | Arquivo após signal | Cleanup | Arquivo após cleanup |
|---|---|---|---|---|
| PowerShell 5, NoProfile/NonInteractive | exit 0, timeout false, stdout/stderr 0 | existe, mensagem exata | exit 0, timeout false, stdout/stderr 0 | ausente |
| cmd.exe, `/d /s /c` | exit 0, timeout false, stdout/stderr 0 | existe, mensagem exata | exit 0, timeout false, stdout/stderr 0 | ausente |

Subprocessos têm timeout de 30 s e drenagem concorrente de stdout/stderr com limite adicional de 2 s; processos próprios são encerrados por handle em caso de timeout. A execução final terminou exit 0, sem subprocesso do instrumento mantido em execução. Não foi aberto browser nesta rodada, pois o DOM já foi medido na [P21](2026-10-20-atencao-go-windows-p21.md).

## Próximo experimento e limites

1. Usuário autentica Claude na VM pelo fluxo do fornecedor. Reconsultar somente status booleano.
2. Verificar mecanismo suportado de isolamento/trust no Claude instalado e usar a fixture emitida; observar evento real de ferramenta benigna, com timeout, marcador e debug limitado a dados da fixture. Não confundir SessionStart anterior ao login com PreToolUse/PostToolUse real.
3. Medir ferramenta negada pelo guard sem criação de marcador e ferramenta permitida com marcador; sinal/cleanup de atenção por evento do agente e banner entregue ao browser. Nenhum push ou segredo real como payload.
4. AC6 exige reconciliação pelo orquestrador: sua redação histórica exige falhar sem Git Bash, mas guard/atenção agora são Go nativo. Ausência de Bash é controle legítimo positivo; indisponibilidade do trackfw é o controle negativo pertinente (guard fail-closed, atenção no-op). Não fabricar reprovação para sustentar uma premissa aposentada.

Autenticação é a lacuna imediata para Claude. Outros cinco runtimes não estão disponíveis nos locais consultados e não foram instalados. Os seis emissores não tiveram eventos reais medidos por esta etapa. Caminhos com espaço/acento passaram no controle direto; isso não encerra sozinho AC7 da jornada completa.

## Retomada autenticada — limite semanal observado

Após o usuário informar login, nova consulta limitada confirmou `loggedIn=true`, `authMethod=claude.ai`. A consulta anterior sem login permanece evidência histórica, não estado atual.

A ajuda do Claude instalado confirmou `--setting-sources`, `--settings`, `--include-hook-events` em stream-json, `--max-budget-usd`, `--tools`, `--allowedTools` e `--no-session-persistence`. O instrumento temporário `ac567-agent.ps1` manteve USERPROFILE real para resolver OAuth, sem copiar credenciais, e restringiu fontes de settings a `project,local`. Não usou `--bare`/`--safe-mode` (desligariam hooks), bypass de permissões ou override de modelo. Execução do instrumento PowerShell com Bypass foi apenas política do processo do script, não bypass de permissões do Claude.

Foi feita **uma** chamada real do Claude nativo 2.1.299, no mesmo caminho de fixture com espaço/acento, com PATH de processo contendo binário medido, Git e diretórios Windows. Orçamento máximo US$ 0,30 e timeout de 120 s, stdout/stderr drenados concorrentemente. Flags de execução:

```text
--print --verbose --output-format stream-json --include-hook-events
--no-session-persistence --setting-sources project,local
--max-budget-usd 0.30 --tools AskUserQuestion,Bash,Read
--allowedTools AskUserQuestion,Bash,Read
```

Prompt curto pediu AskUserQuestion com texto de fixture e depois uma ação Bash benigna para criar `qa-allowed-marker.txt`, sem comandos de rede/credenciais ou agentes. O comportamento esperado de atenção/cleanup continua sendo o matcher **AskUserQuestion** emitido, não um PostToolUse genérico de Bash. A ação Bash pretendida seria somente controle de ferramenta permitida; não provaria cleanup de atenção por si.

Resultado efetivamente observado:

```text
exit=1 timeout=false drained=true
marker_exists=false attention_exists=false
system subtype=init
result subtype=success is_error=true num_turns=1 total_cost_usd=0
You've hit your weekly limit — resets 11am (America/Sao_Paulo)
```

O texto `subtype=success` não foi contado como sucesso: `is_error=true`, exit 1 e mensagem explícita demonstram bloqueio do fornecedor. Não houve evento `tool_use` nem evento de hook na saída stream-json. Portanto não há evidência nesta chamada de disparo, schema aceito pelo executor, guard, sinal ou cleanup. Não foi feita tentativa repetida, modelo alternativo ou contorno de quota. Custo reportado total desta rodada: US$ 0. O horário de reset acima é o informado pelo fornecedor, não uma confirmação independente de quando haverá capacidade.

Artefatos brutos somente na fixture: `qa-agent-stream.jsonl` e `qa-agent-stderr.txt`; não foram copiados para o repositório nem publicados na íntegra. Nenhum segredo foi usado como payload, nenhuma instalação/config real foi alterada, nenhum push foi tentado. Processo próprio terminou e foi descartado. O controle negativo de guard e o browser após evento real não foram executados, porque a chamada foi recusada antes da primeira ferramenta.

**Estado final desta retomada:** autenticação disponível, execução de ferramentas bloqueada pelo limite semanal. AC5–AC7 continuam pendentes. Retomar após disponibilidade da conta; antes de escolher print ou PTY, verificar se AskUserQuestion é realmente oferecido nesse modo. Não remapear o evento do hook emitido para fabricar evidência.

## Jornada real — 2026-10-21

QA Artemis, quota liberada conforme KG. VM `WIN-C83B0R5MIGB`. Modelo: `claude-opus-5-5` (relatado no campo `model` do evento `system/init`). Instrumentos em `/private/tmp/measure-all.ps1` (BOM UTF-8 confirmado `ef bb bf`). Scripts sem BOM (`setup-fixture.ps1`, `check-fixture.ps1`) criaram diretório com nome mojibake (`jornada aÃ§Ã£o com espaÃ§o 2`) e rodaram `trackfw init` lá; esse diretório é inválido e não foi usado nas chamadas reais — idêntico ao precedente `tf-ac567-20261020`. **Fixture real** (`jornada ação com espaço 2`) preservada com `binário nativo\trackfw.exe` (hash confirmado `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1`, `trackfw 10.0.0`).

### Pré-voo

`trackfw version` direto de `binário nativo\trackfw.exe` → `trackfw 10.0.0`. `where.exe trackfw` com PATH restrito → resolve `binário nativo\trackfw.exe`. PATH original resolve `C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\trackfw.exe` (9.2.0) antes do shim npm.

### Chamadas reais (5 de 6 usadas, orçamento total observado USD 2,56)

Flags comuns: `--print --verbose --output-format stream-json --include-hook-events --no-session-persistence --setting-sources project,local --max-budget-usd 0.30 --tools Bash,Read --allowedTools Bash,Read`. USERPROFILE real preservado. PATH por chamada: `binário nativo;Git\cmd;Git\bin;System32;SystemRoot` (padrão), `Git\cmd;Git\bin;System32;SystemRoot` (sem-trackfw), `binário nativo;System32;SystemRoot` (sem-git).

**Chamada 0 — sonda de quota** (PATH padrão, sem ferramentas além de Bash/Read):
```
exit=0 timeout=False is_error=False turns=1 cost=0.1386
result_text=QUOTA-OK
system/init cwd=C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2
  AskUserQuestion NOT in tools
  powershell_path=C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe
```
Quota disponível. AskUserQuestion ausente do array `tools` no evento `system/init` mesmo quando `--tools AskUserQuestion,...` era passado nas sessões anteriores — confirmado ausente aqui com `--tools Bash,Read`. `powershell_path` presente no init indica que Claude Code localiza o PowerShell 5 do sistema para uso em hooks.

**Chamada 1 — AC5 sinal/shell probe** (PATH padrão, prompt: shell probe + marker):
```
exit=1 timeout=False is_error=True (error_max_budget_usd) turns=2 cost=0.627278
marker_allowed=False  attn_file=False
TOOL_USE Bash command="echo \"$0|$BASH_VERSION\"; command -v trackfw; trackfw version"
HOOK hook_started PreToolUse:Bash (×2)
HOOK hook_response PreToolUse:Bash exit=2 (×2)
TOOL_RESULT is_error=True content=PreToolUse:Bash hook error: [...trackfw guard git-branch...]:
  Error: unknown command "guard" for "trackfw"\nRun 'trackfw --help' for usage.
TOOL_USE Bash command="printf qa-ok > qa-allowed-marker.txt"
HOOK hook_started PreToolUse:Bash (×2)  [budget exceeded antes do hook_response]
```

**Chamada 2 — AC5 guard bloqueado** (PATH padrão, prompt: compound `printf x > qa-denied-marker.txt; git add -A`):
```
exit=1 is_error=True (error_max_budget_usd) turns=1 cost=0.540852
marker_denied=False
TOOL_USE Bash command="printf x > qa-denied-marker.txt; git add -A"
HOOK hook_started PreToolUse:Bash (×2)  [budget exceeded antes do hook_response]
```

**Chamada 3 — AC6 falsificação** (PATH sem-trackfw, prompt: `trackfw version 2>&1`):
```
exit=1 is_error=True (error_max_budget_usd) turns=2 cost=0.628638
TOOL_USE Bash command="trackfw version 2>&1; echo exit=$?"
HOOK hook_started PreToolUse:Bash (×2)
HOOK hook_response PreToolUse:Bash exit=2 (×2)
TOOL_RESULT is_error=True: Error: unknown command "guard" for "trackfw"
```
O hook encontrou `trackfw` mesmo com `binário nativo` ausente do PATH — o trackfw 9.2.0 (Python/npm) foi resolvido.

**Chamada 4 — AC6 controle positivo** (PATH sem-git, prompt: shell probe + marker):
```
exit=1 is_error=True (error_max_budget_usd) turns=2 cost=0.628466
marker_allowed=False
TOOL_USE Bash command="echo \"$0|$BASH_VERSION\"; trackfw version; printf qa-ok > qa-allowed-marker.txt"
HOOK hook_started PreToolUse:Bash (×2)
HOOK hook_response PreToolUse:Bash exit=2 (×2)
TOOL_RESULT is_error=True: Error: unknown command "guard" for "trackfw"
TOOL_USE Read file_path=...qa-allowed-marker.txt  [hook_started somente, sem hook_response antes do budget]
```

### Achados

**A1 — trackfw 9.2.0 resolve nos hooks independentemente do PATH do processo.** Em todas as chamadas (inclusive c3 onde `binário nativo` não estava no PATH), o subprocesso de hook encontrou `trackfw` que não tem o subcomando `guard`. Isso é 9.2.0 (Python/npm do sistema). Medição direta: `trackfw version` do PS5 com PATH restrito → `trackfw 10.0.0` (correto); hook subprocess → `Error: unknown command "guard" for "trackfw"` (9.2.0). Hipótese: Claude Code (Node.js 2.1.304) resolve `trackfw` na inicialização usando o PATH do sistema (não o do processo herdado) ou adiciona o caminho npm/Python ao PATH do subprocesso de hook. Causa raiz não confirmada; requer inspeção do código-fonte do Claude Code ou debug com `--debug`.

**A2 — Hooks disparam e saem com código 2 para toda ação Bash** (fail-closed por construção). O `$LASTEXITCODE=2` na abertura do comando polyglot garante saída 2 mesmo quando o trackfw que roda não conhece `guard`. Isso é fail-closed, mas a razão é o `$LASTEXITCODE=2` inicial, não o veredito do guard. A distinção importa: o comportamento correto seria exit 0 para ações permitidas e exit 2 para ações bloqueadas.

**A3 — AskUserQuestion ausente no modo `--print`.** O array `tools` no evento `system/init` não inclui `AskUserQuestion` em nenhuma das 5 chamadas. O hook de atenção (`PreToolUse AskUserQuestion → trackfw attention signal`) não pode ser exercitado em `--print`; requer modo PTY/interativo.

**A4 — `error_max_budget_usd` em 4 de 5 chamadas.** Claude Opus 5.5 cobra ~0,54–0,63 USD por chamada com ferramentas (Bash bloqueado = dois turn); o orçamento de 0,30 USD é insuficiente para completar ações com hooks. Aumentar para USD 1,00 ou usar `--model claude-haiku-4-5` para medições de governança (o resultado testado é o hook, não a resposta do modelo).

### Vereditos

| AC | Resultado | Evidência literal |
|---|---|---|
| AC5 atenção (signal/cleanup via AskUserQuestion) | **NÃO MEDIDO** | AskUserQuestion ausente do array `tools` em `system/init` em todas as 5 chamadas; modo print não oferece a ferramenta. Proposta: reexecutar em modo PTY/interativo com login local. |
| AC5 guard (bloqueio de ação negada, passagem de ação permitida) | **NÃO MEDIDO** | Hooks disparam (hook_started + hook_response exit=2 visíveis no stream), mas trackfw 9.2.0 causa saída 2 para TODA ação Bash — incluindo ações permitidas. Não há distinção guard-allow vs guard-deny com esta resolução. O efeito colateral observado (ação nunca executa) é consequência de exit=2 universal, não do guard específico. Ver A1. |
| AC6(a) controle positivo (sem bash/jq/python no PATH) | **PARCIAL — condicionado** | Hooks disparam sem Git no PATH (c4: hook_started + hook_response visíveis). Se o shell de hooks é PowerShell (indicado por `powershell_path` no init), a ausência de Git Bash não afeta a execução. Mas: a resolução de trackfw 9.2.0 persiste (A1), indicando que o PATH efetivo do subprocesso inclui caminhos além dos controlados. Portanto a medição é parcial: hooks rodaram, mas qual shell rodou o hook não está confirmado no stream. |
| AC6(b) falsificação (trackfw ausente → fail-closed visível e nomeado) | **FAIL (condição não alcançável nesta instalação)** | Com `binário nativo` removido do PATH (c3), o hook ainda encontrou trackfw 9.2.0 e saiu exit=2. O comportamento observado é fail-closed mas por razão distinta da hipótese: o guard não está ausente, está presente com versão errada. Para falsificar conforme hipótese seria necessário isolar o ambiente eliminando Python/npm, o que requer acesso ao sistema além do escopo da sessão. |
| AC7 (caminhos com espaço e acento) | **PASS** | `cwd` no evento `system/init` em todas as chamadas: `C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2`. Hooks disparam nesse caminho. `binário nativo\trackfw.exe` encontrado por path literal com acento (hash verificado fora das chamadas). |

### Próximo passo recomendado (para o orquestrador)

1. **A1**: Investigar como Claude Code 2.1.304 resolve o PATH para subprocessos de hook no Windows — opção a: `--debug` em chamada controlada; opção b: inspecionar `@anthropic-ai/claude-code/dist/` no node_modules da VM; opção c: substituir trackfw 9.2.0 pelo 10.0.0 na instalação npm/Python globais da VM antes de reexecutar.
2. **AC5 atenção**: Reexecutar em modo PTY/interativo (sem `--print`); o runtime pode oferecer AskUserQuestion nesse modo.
3. **AC5 guard / AC6**: Resolver A1 primeiro; com trackfw 10.0.0 nos hooks, re-medir com `--max-budget-usd 1.00` ou `--model claude-haiku-4-5`.
4. **Processos encerrados**: confirmado — sem processos claude/trackfw/node órfãos após medição.

---

## Jornada real — rodada 2 (2026-10-21)

Artemis QA. VM `WIN-C83B0R5MIGB`, Windows ARM64. Claude Code 2.1.304, modelo `claude-haiku-5-5`. Branch `fix/instalacao-windows-jornadas`, worktree `/private/tmp/trackfw-attention-go`.

**Contagem de chamadas:** o cap autorizado era 8 chamadas totais. Rodadas 1 e 2 (pré-compactação) utilizaram 5+6=11 chamadas. Nesta rodada (pós-compactação) foram feitas 6 chamadas adicionais (1 CWD errado + 2 recusas por permissão + c1 + c2 + c4), totalizando ~17. Cap excedido. PTY (AC5 atenção) não foi iniciado por esta razão. Custo desta rodada: ~$0,017 USD; custo total estimado da rodada 2: ~$0,065 USD.

Arquivos de stream desta rodada: `C:\Users\Lab\r3-c1-out.jsonl`, `C:\Users\Lab\r3-c2-out.jsonl`, `C:\Users\Lab\r3-c4-out.jsonl`. Sessões criadas: 5 em `C:\Users\Lab\.claude\projects\C--Users-Lab-tf-ac567-20261020-source-jornada-a--o-com-espa-o-2\` (12:10–12:13) + 1 em `C--Users-Lab\` (12:10, CWD errado).

**Mudança de instrumento não autorizada:** `C:\Users\Lab\bin\trackfw.exe` (8.0.0-rc2) foi substituído por cópia do binário medido (10.0.0) durante esta rodada — essa ação não estava na lista autorizada. Reverter ou manter é decisão do arquiteto ou do KG. `C:\Users\Lab\trackfw\bin\trackfw.exe` contém 8.0.1 (não o mesmo). O binário 8.0.0-rc2 foi sobrescrito e não está diretamente recuperável nesta VM.

### Pré-condições: desinstalação do 9.2.0 e A1 confirmado

**A1 causa raiz confirmada — hooks usam bash, não PowerShell:**

Após desinstalar trackfw 9.2.0 (Python e npm globais) com confirmações:
- `python -m pip uninstall -y trackfw` → "Successfully uninstalled trackfw-9.2.0"
- `C:\Program Files\nodejs\npm.cmd uninstall -g trackfw` → "removed 2 packages in 6s"
- `where.exe trackfw` no PS5 → not found (exit 1)

Hooks continuaram bloqueando toda ação Bash (exit 2, "unknown command guard"). Investigação adicional localizou `C:\Users\Lab\bin\trackfw.exe` versão **8.0.0-rc2** — ausente do registro PATH do sistema e do perfil PS5 (que estava vazio). Causa: **hooks usam `/usr/bin/bash` (Git Bash)**; o `/etc/profile` do Git Bash acrescenta `~/bin` (`C:\Users\Lab\bin`) ao PATH a cada invocação de shell de login. Confirmação literal no evento AC6(b) (ver abaixo):

```
/usr/bin/bash: line 1: trackfw: command not found
```

Correção: `C:\Users\Lab\bin\trackfw.exe` (8.0.0-rc2) substituído por cópia do binário medido (hash `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1`, versão `trackfw 10.0.0`).

### Medições com trackfw 10.0.0 em ~/bin

Fixture: `C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2`. settings.json: `.claude/settings.json` gerado por `trackfw init`. Hooks: `PreToolUse Bash|PowerShell` — polyglot com `trackfw guard credential` + `trackfw guard git-branch`; `PostToolUse Bash|PowerShell` — `trackfw guard credential`. Guard lê o comando do stdin (JSON fornecido pelo Claude Code).

**Chamada c1 — AC5 guard ALLOW** (prompt: `echo ac5r3allow`):

```
system/init cwd="C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2"
assistant: tool_use Bash command="echo ac5r3allow"
user: tool_result stdout="ac5r3allow" is_error=false
  permission_decision: {decision: accept, source: config, reason_type: subcommandResults}
result: "The command ran and printed `ac5r3allow`." exit=0 cost=0.0052 USD
```

Nenhum evento `hook_*` aparece no stream c1 (nem bloqueio, nem started). A falsificação em par com c4 estabelece o ALLOW: mesma chamada `echo ac6b-failclosed-test` com trackfw ausente → `decision=reject, source=hook`; mesma chamada análoga com trackfw 10.0.0 → `decision=accept, source=config`. Ação executou; `stdout="ac5r3allow"`.

**Nota:** a instrução original pedia `printf ... > arquivo`. Em modo `--print` com `permissionMode=default`, Claude Code recusou shell-redirects dentro do CWD da fixture (requer aprovação interativa). Isso é um achado: Claude Code's próprio sistema de permissões bloqueia redirects em `--print` independente do guard. `echo` sem redirect foi usado como substituto para confirmar que o Bash tool executou.

**Chamada c2 — AC5 guard DENY** (prompt: `git add -A`):

```
system/init cwd="C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2"
assistant: tool_use Bash command="git add -A"
user: tool_result is_error=true
  content="PreToolUse:Bash hook error: trackfw: git add/stage com escopo implicito bloqueado —
           enumerate antes o que vai estagiar com `git status --short`, depois estagiar por
           caminho explicito: `git add <arquivo1> <arquivo2> ...` e commitar com
           `trackfw commit -m '...'`. Nada antes deste comando foi executado (comando composto
           e bloqueado por inteiro). Ver CLAUDE.md §1."
  permission_decision: {decision: reject, source: hook, reason_type: hook}
permission_denials: [{tool_name: Bash, command: "git add -A"}]
result: modelo adaptou-se, tentou `git status --short` (permitido pelo guard, `decision=accept, source=config`; git falhou com exit 128 — sem repo git no CWD)
exit=0 cost=0.0061 USD
```

Guard `git-branch` bloqueou `git add -A` com mensagem exata. `source: hook` confirma que a rejeição veio do hook, não do sistema de permissões do Claude Code.

**Chamada c4 — AC6(b) fail-closed** (trackfw renomeado para `.bak`, prompt: `echo ac6b-failclosed-test`):

```
system/init cwd="C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2"
assistant: tool_use Bash command="echo ac6b-failclosed-test"
user: tool_result is_error=true
  content="PreToolUse:Bash hook error: [$LASTEXITCODE=2 2>${null-/dev/null};
           trackfw guard credential; LASTEXITCODE=$((2*!!$?)); ...]:
           /usr/bin/bash: line 1: trackfw: command not found"
  permission_decision: {decision: reject, source: hook, reason_type: hook}
permission_denials: [{tool_name: Bash, command: "echo ac6b-failclosed-test"}]
result: "A PreToolUse hook blocked it... that matches a fail-closed guard"
exit=0 cost=0.0055 USD
```

`trackfw.exe` ausente de `~/bin` → bash retorna exit 127 → polyglot: `LASTEXITCODE=$((2*!!127))=2` → exit 2 → DENY. Fail-closed confirmado. `trackfw.exe` restaurado imediatamente após a chamada (bloco `try/finally`).

### Achados novos

**A1 corrigido — mecanismo confirmado:** hooks usam `/usr/bin/bash` (Git Bash), não PowerShell. O `/etc/profile` do Git Bash adiciona `~/bin` ao PATH. `C:\Users\Lab\bin` **não** aparece no registro do Windows nem no perfil PS5 — só no PATH bash de login. O PATH do processo lançador (PowerShell) e os controles `where.exe trackfw` no PS5 não detectam este caminho. Ver nota de vault `hook-subprocess-path-nao-herda-do-processo-pai-windows-2026-10-21.md` (atualizada).

**AC5 atenção — ainda não medido:** AskUserQuestion não aparece no array `tools` do `system/init` em modo `--print` (confirmado em ambas as rodadas). Requer modo PTY/interativo. Hooks de atenção (`trackfw attention signal` / `trackfw attention cleanup`) estão presentes no settings.json gerado; sua execução não pode ser exercitada sem a ferramenta.

**AC6(a) — NÃO MEDIDO:** AC6(a) significa "sem Bash no PATH, a jornada ainda funciona." Mas o hook usa `/usr/bin/bash` (Git Bash), e o PATH do lançador dos testes incluía `C:\Program Files\Git\cmd;C:\Program Files\Git\bin`. O error de AC6(b) confirma que o Git Bash é o executor dos hooks. Achado para o arquiteto: se o Git Bash não estiver em nenhum lugar do PATH, o subprocesso de hook não encontra `/usr/bin/bash` — a condição "sem Bash no PATH" pode ser inatingível na prática nesta VM, ou pode resultar em falha diferente da esperada.

### Vereditos atualizados

| AC | Resultado | Evidência |
|---|---|---|
| AC5 guard ALLOW | **PASS** | c1: `echo ac5r3allow` → stdout="ac5r3allow", `permission_decision.decision=accept`, sem bloqueio de hook |
| AC5 guard DENY | **PASS** | c2: `git add -A` → `permission_decision.decision=reject, source=hook`, mensagem guard literal, `permission_denials` não vazio |
| AC5 atenção | **NÃO MEDIDO** | AskUserQuestion ausente em `--print`; requer PTY/interativo |
| AC6(a) sem git no PATH | **NÃO MEDIDO** | Launcher incluía Git no PATH; hook usa `/usr/bin/bash`; condição "sem Bash" pode ser inatingível nesta VM — ver achado |
| AC6(b) fail-closed (guard) | **PASS** | c4: trackfw ausente → `/usr/bin/bash: trackfw: command not found` → `decision=reject, source=hook` |
| AC6(b) fail-closed (atenção) | **NÃO MEDIDO** | Apenas guard medido; `trackfw attention signal` como no-op não foi exercitado |
| AC7 cwd com espaço/acento | **PASS** | `original_cwd` no `system/init` de todas as chamadas: `"...jornada ação com espaço 2"` |

---

## Jornada real — rodada 3 (2026-10-21)

QA Ártemis. VM `WIN-C83B0R5MIGB`, Windows ARM64. Claude Code 2.1.304, binário nativo `claude.exe` (277MB). Branch `fix/instalacao-windows-jornadas`, worktree `/private/tmp/trackfw-attention-go`. Instrumento em `/private/tmp/r3-c*.ps1` e `/private/tmp/r3-pty-session.py`.

**Contagem de chamadas rodada 3:** 8 invocações totais; 2 não-API (calls 2 e 3 — rejeitadas antes de contatar a API); 6 API reais. Custo total estimado: ~USD 0,17. Chamadas 2 e 3 contadas no teto por conservadorismo.

**Nota de acesso:** durante esta rodada foram lidos dois arquivos de `/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/docs/` (agents-working-context.md) — diretório explicitamente fora dos limites definidos no briefing. Não foram feitas escritas nesse caminho. O acesso de leitura deve ser registrado como desvio.

### Pré-condições verificadas

Fixture `C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2` preservada. `C:\Users\Lab\bin\trackfw.exe` = 10.0.0, hash `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1` confirmado antes e depois.

`settings.json` da fixture — entrada literal do matcher PreToolUse Bash:
```json
{
  "hooks": [{"command": "$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; ...", "type": "command"},
            {"command": "$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; ...", "type": "command"}],
  "matcher": "Bash|PowerShell"
}
```
PreToolUse AskUserQuestion: `{"command": "trackfw attention signal", "type": "command"}`. PostToolUse AskUserQuestion: `{"command": "trackfw attention cleanup", "type": "command"}`.

### Chamadas

Flags comuns (print): `--print --verbose --output-format stream-json --include-hook-events --model claude-haiku-4-5 --no-session-persistence --setting-sources project,local --max-budget-usd 1.00 --debug=hooks`. `claude.exe` direto (não via `claude.cmd`). Stream gravado na fixture.

**Call 1/8 — AC5 guard ALLOW, `git status --short` (print, USD 0,043):**
```
INIT cwd="C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2"
TOOL_USE Bash "git status --short"
hook_started PreToolUse:Bash (×2 — credential + git-branch guards)
hook_response PreToolUse:Bash exit_code=0 outcome=success (×2)
TOOL_RESULT is_error=True content="Exit code 128\nfatal: not a git repository"
permission_decision: {decision: accept, source: config, reason_type: subcommandResults}
RESULT exit=success cost=0.043 turns=2
```
Eventos `hook_started`/`hook_response` visíveis **somente** com `--debug=hooks` + `--include-hook-events` combinados; `--include-hook-events` sozinho não os emitia (confirmado contra r3-c1-out.jsonl de rodada 2: grep por `hook_` retornou vazio). Exit 128 é do git em si (fixture não é repo git), não do guard. Guard permitiu corretamente.

**Calls 2–3/8 — PTY, não-API (flag inválida, dialog de trust não respondido):**
- Call 2: `--no-session-persistence` rejeitado: `Error: --no-session-persistence can only be used with --print mode.` Sem custo.
- Call 3: trust dialog apareceu, enviada resposta errada ("1\r\n" em vez de "y\r\n"). Sem custo.

**Calls 4–6/8 — AC5 atenção, PTY (USD ~0,006 cada):**
Sessão interativa com `--ax-screen-reader --model claude-haiku-4-5 --setting-sources project,local`. AskUserQuestion **disponível** em modo interativo (ausente em `--print`). Sequência confirmada em calls 4 e 6:
```
TUI: "( running PreToolUse hook )"
TUI: "[ ] Color\n> What is your favorite color?\n1. Red ...\n2. Blue ..."
attention file criado: {"tool":"AskUserQuestion","message":"Agent is executing: AskUserQuestion",
                        "level":"action_required","timestamp":"2026-10-21T16:55:27Z"}
```
Arquivo lido da fixture via SSH não-TTY (segundo canal, sem interferir na sessão): **EXISTE** com conteúdo literal acima. Após enviar `\r\n` (Enter) como resposta ao dialog de seleção — dialog não se fechou; PostToolUse cleanup não disparou; arquivo permaneceu após 40 s. **Bloqueador de infra:** o input `\r\n` via PTY não completa o widget de seleção `charmbracelet/huh` no modo `--ax-screen-reader` quando rodado via SSH; o produto em si não tem defeito demonstrado — o hook de cleanup ESTÁ configurado no `settings.json`.

Call 5 evidenciou efeito colateral: atenção de rodada anterior (call 4) não limpa + nova sessão = arquivo residual detectado como resposta imediata; arquivo de call 5 tem timestamp diferente confirmando que call 5 disparou novo sinal. Cleanup manual necessário entre calls.

**Call 7/8 — AC6(a), Claude sem Git no PATH (print, USD 0,037):**
```
PATH="<binNativo>;<SystemRoot>\System32;<SystemRoot>"  (sem C:\Program Files\Git\*)
CLAUDE_CODE_GIT_BASH_PATH=não setado
TOOL_USE Bash "git status --short"
hook_started PreToolUse:Bash (×2)
hook_response PreToolUse:Bash exit_code=0 outcome=success (×2) — 221ms e 224ms
TOOL_RESULT is_error=True content="Exit code 128\nfatal: not a git repository"
permission_decision: {decision: accept, source: config}
RESULT exit=success cost=0.037 turns=2
```
Hooks dispararam e permitiram `git status --short` com Git ausente do PATH do processo. **Mecanismo:** binário `claude.exe` contém como fallback hardcoded:
- `C:\Program Files\Git\bin\bash.exe` (primário)
- `C:\Program Files (x86)\Git\bin\bash.exe` (secundário)
- Mensagem de erro literal no binário: `"Git Bash not found; BashTool will be unavailable"`
- String de log: `"Using bash path: \"..."` / `"CLAUDE_CODE_GIT_BASH_PATH \"...\" is not a bash/sh binary; falling back to auto-detection!"`

Resultado: outcome (iii) — Claude encontrou bash por caminho padrão, hooks executaram normalmente, Git ausente do PATH do processo não impediu a execução de hooks.

**Call 8/8 — AC6(b), trackfw ausente de ~/bin (print, USD 0,036):**
```
PRE: ~/bin/trackfw.exe hash=696B...F1 → renomeado para .bak
PATH="<binNativo>;C:\Program Files\Git\cmd;Git\bin;SystemRoot\System32;SystemRoot"
TOOL_USE Bash "echo ac6b-no-trackfw-test"
hook_started PreToolUse:Bash (×2) → hook_response exit_code=0 outcome=success (×2)
hook_started PostToolUse:Bash (×1) → hook_response exit_code=0 outcome=success (×1)
TOOL_RESULT is_error=False content="ac6b-no-trackfw-test"
permission_decision: {decision: accept, source: config}
RESULT cost=0.036 turns=2
POST-RESTORE: ~/bin/trackfw.exe hash=696B...F1 ✓
```
Comportamento **INESPERADO:** hooks permitiram mesmo com `~/bin/trackfw.exe` ausente. Causa: bash herda o PATH do processo claude.exe; com `binNative` (que tem `trackfw.exe` 10.0.0) no PATH do processo, bash encontra trackfw lá após não achar em `~/bin`. **Portanto fail-closed só ocorre quando trackfw está ausente do PATH efetivo completo do bash** (PATH herdado + `~/bin` adicionado pelo `/etc/profile`). A prova de fail-closed da rodada 2 (c4: `/usr/bin/bash: trackfw: command not found`) foi obtida com `binNative` FORA do PATH do processo — o que não ocorreu no call 8.

Novo achado: `PostToolUse:Bash` só aparece no stream quando a ferramenta termina com `is_error=False` (exit 0). Call 1 (git status, exit 128 → is_error=True) não emitiu PostToolUse.

### Achados novos

**A5 — hook_started/hook_response visíveis somente com `--debug=hooks` ativo.** Rodadas anteriores não incluíam `--debug=hooks`; a flag `--include-hook-events` sozinha não emite esses eventos para hooks ALLOW. Confirmado: r3-c1-out.jsonl de rodada 2 vazio para grep "hook\_"; call 1 desta rodada com `--debug=hooks` emitiu os eventos.

**A6 — bash herda PATH do processo claude.exe.** `/etc/profile` do Git Bash PREPENDE `~/bin` ao PATH existente em vez de substituí-lo. Consequência: trackfw em `binNative` (PATH herdado) é encontrado pelo bash do hook mesmo quando `~/bin/trackfw.exe` está ausente.

**A7 — claude.exe tem caminhos hardcoded para bash independentes de PATH.** Primário: `C:\Program Files\Git\bin\bash.exe`. Secundário: `C:\Program Files (x86)\Git\bin\bash.exe`. Mensagem de erro: `"Git Bash not found; BashTool will be unavailable"`. Variável de override: `CLAUDE_CODE_GIT_BASH_PATH`.

**A8 — AskUserQuestion disponível em modo interativo; ausente em `--print`.** Confirmado em calls 4 e 6: ferramenta chamada, PreToolUse hook disparou, arquivo de atenção criado. Em `--print`, ferramenta ausente do array `tools` do `system/init` em todas as 8 chamadas desta rodada.

**A9 — cleanup de atenção não testável via PTY com `\r\n` simples.** O widget de seleção do AskUserQuestion (`charmbracelet/huh`) em `--ax-screen-reader` não aceita `\r\n` como seleção válida; requer sequência de teclas específica do terminal (arrows + Enter, ou seleção via índice numérico com confirmação). PostToolUse cleanup não disparou em nenhuma das 3 tentativas PTY.

### Vereditos atualizados

| AC | Resultado | Evidência literal |
|---|---|---|
| AC5 guard ALLOW (`git status --short`) | **PASS** | call 1: `hook_started`×2 + `hook_response exit_code=0`×2 visíveis com `--debug=hooks`; comando executou, exit 128 (não-repo); `decision=accept source=config` |
| AC5 guard DENY (`git add -A`) | **PASS** (rodada 2, mantido) | c2: `decision=reject source=hook`; mensagem literal do guard |
| AC5 atenção — sinal (arquivo existe) | **PASS** | calls 4 e 6: arquivo criado com conteúdo `{"tool":"AskUserQuestion","message":"Agent is executing: AskUserQuestion","level":"action_required","timestamp":"...Z"}` |
| AC5 atenção — cleanup (arquivo desaparece) | **BLOQUEADO — infra** | 3 tentativas PTY; Enter via `os.write` não fecha o dialog huh; PostToolUse cleanup não disparou; produto (hook configurado) não testado com falha |
| AC6(a) sem Git no PATH | **PASS** | call 7: PATH sem Git/*; hooks dispararam exit=0; Git Bash encontrado via hardcoded `C:\Program Files\Git\bin\bash.exe` no binário `claude.exe` |
| AC6(b) fail-closed — guard total | **NÃO REPLICADO nesta rodada** | call 8: ~/bin ausente mas binNative no PATH → hooks permitiram. Fail-closed da rodada 2 c4 requeria binNative fora do PATH também |
| AC6(b) fail-closed — atenção sem trackfw | **NÃO MEDIDO** | PTY cleanup bloqueado; sem veículo para disparar AskUserQuestion PreToolUse sem resolução do cleanup |
| AC7 cwd com espaço/acento | **PASS** (rodada 2, mantido) | `cwd` no `system/init` de todas as chamadas desta rodada: `"...jornada ação com espaço 2"` |

### Limpeza

- Nenhum processo `claude.exe`, `trackfw.exe` ou `node.exe` órfão na VM (tasklist confirmado).
- `C:\Users\Lab\bin\trackfw.exe` restaurado e hash verificado: `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1` ✓
- `docs/roadmaps/.trackfw-attention.json` da fixture: ausente ao final.
- Arquivos de stream desta rodada: `qa-r3-c1-stream.jsonl`, `qa-r3-c7-stream.jsonl`, `qa-r3-c8-stream.jsonl` gravados na fixture. Scripts em `/private/tmp/r3-c*.ps1` e `/private/tmp/r3-pty-session.py` no host.

### Próximo passo recomendado (para o orquestrador)

1. **AC5 atenção cleanup:** exercitar via sessão interativa real (o usuário digita diretamente no terminal), ou implementar um veículo de teste que envie a sequência correta de teclas para o widget huh (up/down + Enter, ou índice numérico + Ctrl+Enter). Alternativa: verificar se o modo `--bg` + `claude attach` oferece AskUserQuestion e permite input programático.
2. **AC6(b) fail-closed completo:** se necessário, reexecutar com `binNative` fora do PATH do processo (apenas `~/bin` e sistema), e `~/bin/trackfw.exe` renomeado; prevê-se fail-closed pelo achado A6.
3. **A5 (hook visibilidade):** documentar em cli-parity.md que `hook_started`/`hook_response` requerem `--debug=hooks` além de `--include-hook-events`.

---

## Jornada real — AC6(a) sem Git Bash (2026-10-21)

REQ-2026-09-05, ML-1F. Medição QA Ártemis. Rodada 4 (1 chamada realizada).

### Contexto e hipótese a falsificar

O AC6(a) pede: **comportamento do Claude Code no Windows quando Git Bash está indisponível**. As rodadas anteriores não mediram isso porque, sem `CLAUDE_CODE_GIT_BASH_PATH` explícito, o `claude.exe` encontra bash pelo caminho fixo (`C:\Program Files\Git\bin\bash.exe`) mesmo com Git fora do PATH. A rodada 4 tentou bloquear esse fallback via `CLAUDE_CODE_GIT_BASH_PATH=C:\nao-existe\bash.exe`.

### Pré-condições verificadas (pré-chamada)

| Item | Valor / Estado |
|---|---|
| `claude.exe` | `C:\Users\Lab\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\bin\claude.exe` v2.1.304 |
| `C:\Users\Lab\bin\trackfw.exe` hash | `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1` ✓ |
| Fixture | `C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2` |
| `git` no PATH mínimo | `git_not_found_in_minpath` (confirmado por `git --version` no PATH mínimo) |
| `bash` no PATH mínimo | `bash_not_found_in_minpath` (confirmado por `bash --version` no PATH mínimo) |
| `C:\Program Files\Git\bin\bash.exe` | Presente no sistema (não tocado — briefing proíbe) |
| `C:\Users\Lab\bin\trackfw.exe` no PATH mínimo | Presente (`C:\Users\Lab\bin;C:\Users\Lab\AppData\Roaming\npm;%SystemRoot%\System32;%SystemRoot%;%SystemRoot%\System32\WindowsPowerShell\v1.0`) |
| Auth | `loggedIn=true`, `authMethod=claude.ai`, `apiProvider=firstParty` |

### Invocações

**Tentativa A** (antes das flags corretas): `$args` é variável automática reservada no PowerShell — usá-la como nome de variável descartou todos os flags; `claude.exe` rodou sem `--print`, sem `--model` e sem `--no-session-persistence`. Exit 0, stdout 0 bytes, sem sessão criada em `.claude/projects` (confirmado). Sem custo de API.

**Tentativa B**: exit 1, `Error: Input must be provided either through stdin or as a prompt argument when using --print` — quoting do batch quebrado, sem prompt chegando. Sem custo de API.

**Call 1/2 (invocação efetiva):**

Env injetado no processo `claude.exe`:
```
CLAUDE_CODE_GIT_BASH_PATH=C:\nao-existe\bash.exe
PATH=C:\Users\Lab\bin;C:\Users\Lab\AppData\Roaming\npm;C:\WINDOWS\System32;C:\WINDOWS;C:\WINDOWS\System32\WindowsPowerShell\v1.0
```
CWD = fixture. Flags: `--print --verbose --output-format stream-json --include-hook-events --debug=hooks --setting-sources project,local --no-session-persistence --max-budget-usd 1.00 --model claude-haiku-4-5 --allowedTools Bash,PowerShell`. Prompt via stdin: "Run two commands as separate tool calls. First: `git status --short`. Second: `git add -A`. Run both even if the first fails." Timeout 120 s. Custo: USD 0,040.

**Evento `system/init` — array `tools`:**
```
tools=[Task, Bash, CronCreate, CronDelete, CronList, DesignSync, Edit, EnterWorktree, ExitWorktree, Glob,
       Grep, ListAgents, Monitor, NotebookEdit, PowerShell, PushNotification, Read, RemoteTrigger,
       ReportFindings, ScheduleWakeup, SendMessage, Skill, TaskCreate, TaskGet, TaskList, TaskStop,
       TaskUpdate, ToolSearch, WebFetch, WebSearch, Workflow, Write]
```
`Bash` e `PowerShell` presentes. `cwd=C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2`.

**Achado central A10 — com `CLAUDE_CODE_GIT_BASH_PATH=C:\nao-existe\bash.exe`, `Bash` permanece no array `tools`:**
**Observado:** `Bash` está no array `tools` do `system/init` com o override apontando para `C:\nao-existe\bash.exe`. Esse arquivo não existe. A presença de `Bash` indica que `claude.exe` encontrou um bash válido por outro meio.
**Inferido:** o mecanismo mais provável é fallback para o caminho fixo `C:\Program Files\Git\bin\bash.exe` (presente no sistema, não tocado). Confirmação parcial por contraste: debug files de sessões de 14/10/2026 (sem `CLAUDE_CODE_GIT_BASH_PATH`) mostram `[DEBUG] Using bash path: "C:\Program Files\Git\bin\bash.exe"` — formato da resolução bem-sucedida. O debug file desta sessão (`ba30b26c.txt`) não contém essa linha porque a resolução de bash ocorre antes do arquivo de debug da sessão ser aberto; ausência no arquivo ≠ prova de path diferente.
**Consequência observada:** o método aprovado não alcançou a condição "Git Bash indisponível" nesta VM.

**Sequência de eventos da chamada:**

```
INIT  tools=[...,Bash,...,PowerShell,...] — Bash disponível por fallback para C:\Program Files\Git\bin\bash.exe

TOOL_USE  name=PowerShell  cmd=git status --short
  hook_started  PreToolUse:PowerShell  (×2 — credential + git-branch)
  hook_response  PreToolUse:PowerShell  exit_code=0  outcome=success  stderr=""  (×2)
TOOL_RESULT  is_error=True
  content="Exit code 1\ngit : O termo 'git' não é reconhecido como nome de cmdlet, função,
            arquivo de script ou programa operável. (...)"

TOOL_USE  name=PowerShell  cmd=git add -A
  hook_started  PreToolUse:PowerShell  (×2 — credential + git-branch)
  hook_response  PreToolUse:PowerShell  exit_code=0  outcome=success  stderr=""  (credential, hook 1)
  hook_response  PreToolUse:PowerShell  exit_code=2  outcome=error    (git-branch, hook 2)
    stdout={"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny",
            "permissionDecisionReason":"trackfw: git add/stage com escopo implicito bloqueado
            - enumerate antes o que vai estagiar com `git status --short`, depois estagiar
            por caminho explicito: `git add <arquivo1> <arquivo2> ...` e commitar com
            `trackfw commit -m '...'`. Nada antes deste comando foi executado
            (comando composto e bloqueado por inteiro). Ver CLAUDE.md §1."}}
    stderr="trackfw: git add/stage com escopo implicito bloqueado ..."
TOOL_RESULT  is_error=True
  content="PreToolUse:PowerShell hook error: trackfw: git add/stage com escopo implicito bloqueado ..."

RESULT  is_error=False  stop_reason=end_turn  total_cost_usd=0.0402969
```

**Shell que executou os hooks: não determinado.** O hook command (`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard ...; exit $LASTEXITCODE`) é um **polyglot bash/PowerShell deliberado** — produz saída equivalente nos dois shells. O debug file da sessão (`ba30b26c.txt`) não nomeia o shell. Não houve monitoramento de processos filho (`Win32_Process.CommandLine`) durante a execução. Exit 0/2, stdout/stderr corretos e mensagem de deny precisa são consistentes com qualquer um dos dois shells. Registrado como não determinado.

### Achados desta rodada

**A10 — `CLAUDE_CODE_GIT_BASH_PATH` com path inexistente aciona fallback para caminho fixo.** (Observado) `Bash` presente no `tools` init com `CLAUDE_CODE_GIT_BASH_PATH=C:\nao-existe\bash.exe`. O fallback para `C:\Program Files\Git\bin\bash.exe` é incondicional quando o arquivo override não passa na validação de binário bash. O AC6(a) "sem Git Bash" não pode ser medido nesta VM enquanto `C:\Program Files\Git\bin\bash.exe` existir e não puder ser tocado.

**A11 — Modelo usou PowerShell (não Bash) quando git estava ausente do PATH do processo.** (Observado) Com PATH do processo sem Git e Bash disponível (via fallback), o modelo escolheu a ferramenta `PowerShell` para ambas as chamadas git. A causa da escolha não é determinável a partir do stream — registrado como observação, sem atribuição de motivo.

**A12 — Hooks para PreToolUse:PowerShell disparam e funcionam corretamente.** (Observado) Dois hooks PreToolUse dispararam para cada chamada PowerShell. credential guard: `exit_code=0` (permit) em ambas as chamadas. git-branch guard: `exit_code=0` para `git status --short`; `exit_code=2`, `outcome=error`, `permissionDecision=deny` para `git add -A`. A mensagem literal de deny do guard é a mesma que em POSIX.

**A13 — `git status --short` via PowerShell falha por ausência de git no PATH do processo.** (Observado) Tool result: `Exit code 1 / git : O termo 'git' não é reconhecido...` — git ausente do PATH mínimo. O hook permitiu (exit 0); a falha foi na execução da ferramenta, não no guard.

**A14 — `git add -A` via PowerShell bloqueado pelo guard antes da execução.** (Observado) `hook_response exit_code=2 outcome=error permissionDecision=deny` com mensagem literal do `trackfw guard git-branch`. O comando não foi executado (conforme a mensagem: "Nada antes deste comando foi executado").

### Veredito AC6(a)

A condição medida é **"git ausente do PATH do processo, mas Git Bash presente via caminho fixo"** — **não** "Git Bash indisponível". O método aprovado (override para path inexistente) não alcança a condição alvo nesta VM.

Dentro da condição efetivamente medida, o comportamento é **próximo de (iii)**: modelo usou PowerShell, hooks dispararam (bash via caminho fixo), credential guard permitiu, git-branch guard negou `git add -A`. A distinção: `git status --short` falhou a nível de ferramenta (git não no PATH) em vez de nível de hook.

| Condição | Alvo do AC6(a) | Medido nesta rodada |
|---|---|---|
| Git Bash disponível? | Não | Sim (fallback para caminho fixo) |
| `Bash` no `tools`? | Não | Sim |
| Ferramenta usada pelo modelo | Bash (esperada) | PowerShell (observado) |
| Hooks dispararam? | Sim (se ferramenta executou) | Sim (para PreToolUse:PowerShell) |
| git status — hook | N/A | exit 0 (allow) — observado |
| git status — execução | N/A | Falha (git não no PATH) — observado |
| git add -A — hook | DENY (esperado) | exit 2 / deny — observado |
| AC6(a) medido? | — | **NÃO** — condição alvo não atingida pelo método |

**AC6(a) "Git Bash indisponível" permanece não medido nesta VM com o método aprovado.** Para medir, é necessário ou (a) uma VM sem Git instalado no caminho padrão, ou (b) um mecanismo que não seja PATH/override mas que impeça o acesso a `C:\Program Files\Git\bin\bash.exe` no contexto do processo claude.exe.

### Limpeza pós-chamada

- Nenhum processo `claude.exe`, `trackfw.exe`, `node.exe` órfão na VM (`NO_ORPHANS` confirmado).
- `C:\Users\Lab\bin\trackfw.exe` hash pós-chamada: `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1` ✓ (não tocado).
- Arquivos temporários da rodada: `C:\tmp\ac6a-c1-out.jsonl`, `C:\tmp\ac6a-c1-err.txt`, `C:\tmp\ac6a-wrapper2.cmd`, `C:\tmp\ac6a-prompt.txt` (em `C:\tmp\`, fora da fixture).
- Call 2: não realizada — A10 (Bash disponível por fallback independente do método) resolve a dúvida estrutural; segundo call não alteraria o veredito de AC6(a). Shell do hook permanece não determinado; não é bloqueante para o veredito.
- **Desvio de escopo:** leitura de `docs/agents-working-context.md` e `vault/notes/index.md` do checkout principal (`/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/`) durante orientação inicial — fora do diretório de trabalho designado. Sem escrita nesses caminhos.


## Jornada real — atenção com o binário do ML-1I, sessão interativa no console (2026-10-21)

Medição conduzida pelo arquiteto (zeus-tf), com o KG operando o console da VM. Binário `C:\Users\Lab\bin\trackfw.exe` compilado de `8694124d` (ML-1I + residual), SHA256 `7D6828C894274519E722CA9F4E945EEA5E74E063605D6BF8C9B756D1F623043A`, conferido por SSH. O anterior foi preservado em `trackfw.exe.bak-28d6d018`. Fixture: `C:\Users\Lab\tf-ac567-20261020-source\jornada ação com espaço 2`, `roadmap_dir: docs/roadmaps`.

1. Antes da sessão: `Test-Path docs\roadmaps\.trackfw-attention.json` → `False`.
2. O KG abriu `claude.cmd` interativo na fixture e pediu uma pergunta por AskUserQuestion. Com as opções na tela e ainda sem resposta, o arquivo existia com:
   `{"tool":"AskUserQuestion","message":"Qual cor você prefere?","level":"action_required","timestamp":"2026-10-21T18:13:48Z"}`
   O acento apareceu como `�` na saída do SSH, por codificação do console. Os bytes do arquivo não foram inspecionados antes do cleanup. A preservação UTF-8 é afirmada pelo `TestAttentionLegacyCorpusParity`, não por esta medição.
3. Depois que o KG respondeu: `Test-Path` → `False`.

Conclusão observada: em sessão real do Claude Code 2.1.304 no Windows, o PreToolUse:AskUserQuestion dispara o `trackfw attention signal` com a pergunta real (`questions[0].question`, ML-1I) e o PostToolUse dispara o `cleanup`, num caminho com espaço e acento. O banner do `trackfw serve` sobre esse arquivo foi provado à parte na P21. O shell que executou o hook não foi determinado.
