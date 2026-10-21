# hook subprocess — PATH não herda do processo pai no Windows (Claude Code 2.1.304)

**Data:** 2026-10-21  
**Contexto:** ML-1E, REQ-2026-09-05, AC5–AC7 jornada real, VM WIN-C83B0R5MIGB Windows ARM64

## Achado

Claude Code 2.1.304 executa subprocessos de hook com um PATH que inclui instalações globais de trackfw (npm/Python), **independentemente do PATH do processo pai** que o lançou.

Medido: processo PowerShell definiu `$env:PATH = "binário nativo;Git\cmd;Git\bin;System32;SystemRoot"` antes de lançar `claude.exe` com `UseShellExecute=$false`. Hooks encontraram `trackfw 9.2.0` (Python/npm) em todas as 5 chamadas — incluindo a chamada em que `binário nativo` não estava no PATH. Controle: `trackfw version` e `where.exe trackfw` no mesmo PS5 com o mesmo PATH restrito resolveram para `trackfw 10.0.0` em `binário nativo`.

Evidência literal (stream-json `hook_response`):
```
HOOK hook_response PreToolUse:Bash exit=2
TOOL_RESULT content=PreToolUse:Bash hook error: [...trackfw guard git-branch...]:
  Error: unknown command "guard" for "trackfw"
```
`trackfw 10.0.0` tem `guard`; `trackfw 9.2.0` não tem. O erro prova qual binário rodou.

## Comportamento observado com o polyglot atual (gerado por `trackfw init` 10.0.0)

O polyglot de hook atual — ao contrário da linha simples descrita em `guard-fallback-old-binary-exit1-fail-open-6-clis-2026-10-06` — converte exit 1 do binário antigo em exit 2:

```
$LASTEXITCODE=2 2>${...}; trackfw guard credential; ...; $LASTEXITCODE=2*!!$LASTEXITCODE 2>${...}; exit $LASTEXITCODE
```

Trace em PowerShell 5 com `trackfw 9.2.0` (exit 1, "unknown command"):
1. `trackfw guard credential` → exit 1 → `$LASTEXITCODE=1`
2. `$LASTEXITCODE=2*!!$LASTEXITCODE` → `!!1`=`$true` → `2*$true=2` → `$LASTEXITCODE=2`
3. `exit $LASTEXITCODE` → exit 2

Resultado: **Claude Code bloqueia (exit 2 = critério)** — mas bloqueia TODA ação Bash (deny-all), inclusive ações que seriam permitidas com `trackfw 10.0.0`. Isso é diferente de fail-open — é deny-all operacional.

Contraste com a nota anterior: a nota de 2026-10-06 descreve o comportamento SEM o `$LASTEXITCODE=2*!!$LASTEXITCODE` no polyglot; naquele caso exit 1 → fail-open em Claude Code. A linha atual corrigiu o fail-open, mas à custa de deny-all quando o binário errado está no PATH.

## Consequência prática

Com `trackfw 9.2.0` no PATH efetivo dos hooks (qualquer versão sem `guard`):
- Todos os Bash/PowerShell/Read/Write ficam bloqueados
- O modelo só consegue responder com texto; nenhuma ferramenta de filesystem funciona
- A instalação parece "travada" sem mensagem de erro sobre versão

## Causa raiz (hipótese não confirmada)

Claude Code (Node.js) provavelmente acrescenta os caminhos do npm global (`%APPDATA%\Roaming\npm`) ao PATH do subprocesso de hook na inicialização, ou lê o PATH do registro do Windows em vez do ambiente do processo herdado. Para confirmar: inspecionar `@anthropic-ai/claude-code/dist/` na VM ou usar `--debug` com filtro de processo de hook.

## Correção para medir AC5/AC6 corretamente

Antes de reexecutar a jornada real:
1. Atualizar `trackfw` na instalação Python/npm global da VM para 10.0.0 (`pip install --upgrade trackfw` / `npm install -g trackfw@latest`) — ou —
2. Remover todas as instalações globais de `trackfw` e garantir que `binário nativo\trackfw.exe` seja o único binário no PATH que Claude Code alcança.

`trackfw validate` com a regra `hook_guard_binary_version` pode não detectar a divergência se o PATH que ele usa difere do PATH de hook do Claude Code (ver nota anterior, residual da sonda do validate).

## Links

- Nota anterior sobre binário antigo: [[guard-fallback-old-binary-exit1-fail-open-6-clis-2026-10-06]]
- Relatório completo: `docs/portabilidade/2026-10-20-jornadas-agentes-windows.md` (seção "Jornada real — 2026-10-21")
