# hook subprocess — PATH não herda do processo pai no Windows (Claude Code 2.1.304)

**Data:** 2026-10-21  
**Contexto:** ML-1E, REQ-2026-09-05, AC5–AC7 jornada real, VM WIN-C83B0R5MIGB Windows ARM64

## Achado

**Atualizado 2026-10-21 — causa raiz confirmada.** A hipótese original ("npm/Python global no PATH") estava errada. Ver seção abaixo.

Rodadas 1 e 2 (pré-confirmação): hooks encontraram `trackfw 8.0.0-rc2` em `C:\Users\Lab\bin\trackfw.exe` via PATH bash de login (`~/bin`), gerando `Error: unknown command "guard" for "trackfw"`. Com 8.0.0-rc2 em bash, o polyglot exibia:

```
Trace em bash com trackfw 8.0.0-rc2 (exit 1 para "unknown command"):
1. trackfw guard credential → exit 1 → $?=1
2. LASTEXITCODE=$((2*!!1)) = $((2*1)) = 2
3. exit $LASTEXITCODE → exit 2
```

Resultado: **deny-all** em vez de distinguir ALLOW/DENY pelo guard. O binário certo estava no PATH do processo pai mas não era encontrado pelo hook.

## Comportamento observado com o polyglot atual (gerado por `trackfw init` 10.0.0)

O polyglot é um script polyglot bash+PowerShell:

```
$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE
```

Em bash (executor confirmado via `/usr/bin/bash: line 1: ...`):
1. `$LASTEXITCODE=2 2>/dev/null` — tenta rodar comando `=2`, falha silenciosa
2. `trackfw guard credential` — lê stdin JSON com o comando a auditar, retorna exit 0 (allow) ou exit 2 (deny)
3. `LASTEXITCODE=$((2*!!$?))` — captura: `2*!!0=0` (allow) ou `2*!!2=2` (deny)
4. `$LASTEXITCODE=2*!!$LASTEXITCODE 2>/dev/null` — redundante em bash (var LASTEXITCODE não é `$?`)
5. `exit $LASTEXITCODE` → exit 0 ou exit 2

Com trackfw **ausente** em bash: trackfw não encontrado → exit 127 → `LASTEXITCODE=$((2*!!127))=2` → exit 2 → **fail-closed**.

## Consequência prática

Com `~/bin/trackfw.exe` na versão errada (sem `guard`):
- Todos os Bash/PowerShell/Read/Write ficam bloqueados (deny-all)
- O modelo só consegue responder com texto; nenhuma ferramenta de filesystem funciona
- A instalação parece "travada" sem mensagem de erro sobre versão

## Causa raiz — CONFIRMADA (2026-10-21)

**Hooks usam `/usr/bin/bash` (Git Bash), não PowerShell.** Confirmação literal na chamada AC6(b):

```
PreToolUse:Bash hook error: [...]: /usr/bin/bash: line 1: trackfw: command not found
```

O bash do Git Bash lê `/etc/profile` a cada invocação de shell de login, que adiciona `~/bin` (`C:\Users\Lab\bin`) ao PATH. Este caminho **não** aparece no registro do Windows (`HKCU` ou `HKLM`) nem no perfil do PowerShell 5 (`$PROFILE` vazio). Portanto `where.exe trackfw` no PS5 e qualquer inspeção de PATH por variável de ambiente do processo pai não detectam este caminho.

O binário encontrado era `C:\Users\Lab\bin\trackfw.exe` versão **8.0.0-rc2** — sem o subcomando `guard`. Substituição por 10.0.0 (hash `696B63EAD8A5EB20B4DF41B8F7A427A0DBA5083D66506D6B248DBE61070062F1`) resolveu o problema.

## Comportamento com 10.0.0 em ~/bin (confirmado 2026-10-21)

| Ação | Comportamento | Evidência |
|---|---|---|
| `echo ac5r3allow` | ALLOW — bash retorna exit 0 → polyglot exit 0 | `stdout="ac5r3allow"`, sem bloqueio |
| `git add -A` | DENY — guard retorna exit 2 com mensagem | `permission_decision.decision=reject, source=hook` |
| trackfw ausente | DENY (fail-closed) — bash exit 127 → polyglot exit 2 | `/usr/bin/bash: trackfw: command not found` |

## Correção aplicada (decisão do arquiteto/KG necessária)

`C:\Users\Lab\bin\trackfw.exe` foi substituído por cópia do binário 10.0.0 medido durante a rodada 2 de medição QA. Esta ação **não estava na lista autorizada** — reverter ou manter é decisão do arquiteto ou do KG. O binário original 8.0.0-rc2 foi sobrescrito e não está diretamente recuperável. `C:\Users\Lab\trackfw\bin\trackfw.exe` contém 8.0.1 (não o mesmo).

Se mantida: o `~/bin` do Git Bash continuará sendo o PATH de hook; qualquer atualização futura do trackfw deve sobrescrever `~/bin\trackfw.exe` com a versão compatível com o `settings.json` gerado pelo `trackfw init` vigente.

## Correção ao título da nota (2026-10-21, rodada 3)

O título "PATH não herda do processo pai" estava incorreto. O comportamento real, confirmado na rodada 3 call 8:

**Bash SIM herda o PATH do processo pai (claude.exe).** O `/etc/profile` do Git Bash PREPENDE `~/bin` ao PATH existente — não substitui. Portanto o PATH efetivo do bash é:

```
~/bin : <PATH herdado de claude.exe>
```

**Consequência prática:**
- Se `binNative` (com trackfw 10.0.0) estiver no PATH do processo pai, bash o encontra mesmo com `~/bin/trackfw.exe` ausente.
- Fail-closed (exit 127) só ocorre quando trackfw está ausente de **TODO** o PATH efetivo do bash (tanto `~/bin` quanto o PATH herdado).
- Evidência: call 8 — `~/bin/trackfw.exe` renomeado para `.bak`, PATH do processo tinha `binNative` → hooks exit=0 (permitiram). Rodada 2 c4 — `~/bin/trackfw.exe` renomeado, `binNative` FORA do PATH → hooks exit=127 (bloquearam).

## Achado adicional (rodada 3): hook_started/hook_response requerem --debug=hooks

`--include-hook-events` sozinho não emite `hook_started`/`hook_response` para hooks ALLOW (exit=0). Esses eventos só aparecem no stream-json com `--include-hook-events` E `--debug=hooks` combinados. Hooks DENY aparecem via `tool_result` com `is_error=true` e `permission_decision.source=hook` mesmo sem `--debug=hooks`.

## Achado adicional (rodada 3): caminhos hardcoded para bash em claude.exe

Claude Code 2.1.304 (binário `claude.exe`, 277MB) contém no executável:
- Primário: `C:\Program Files\Git\bin\bash.exe`
- Secundário: `C:\Program Files (x86)\Git\bin\bash.exe`
- Override via `CLAUDE_CODE_GIT_BASH_PATH` (env var)
- Mensagem se não encontrado: `"Git Bash not found; BashTool will be unavailable"`

Isso significa: Git ausente do PATH do processo lançador **não impede** Claude Code de encontrar bash — ele usa os caminhos hardcoded diretamente.

## Achado adicional (rodada 3): PostToolUse só aparece quando is_error=False

O hook PostToolUse:Bash só é emitido no stream quando a ferramenta Bash termina com `is_error=False` (exit 0). Se o comando Bash retorna exit não-zero (ex: git exit 128 → is_error=True), o PostToolUse não aparece no stream com `--debug=hooks`.

## Links

- Nota anterior sobre binário antigo: [[guard-fallback-old-binary-exit1-fail-open-6-clis-2026-10-06]]
- Relatório completo: `docs/portabilidade/2026-10-20-jornadas-agentes-windows.md` (seções "Jornada real — 2026-10-21" e "Jornada real — rodada 3 (2026-10-21)")
