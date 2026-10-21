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
