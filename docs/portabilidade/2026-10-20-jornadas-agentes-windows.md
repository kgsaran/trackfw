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
