# classifyHookAnchorage acusa `~/` como bare relative — falso-positivo em todos os 3 CLIs

**Data:** 2026-08-22  
**Descoberto por:** hades-tf (ML-3A do ROADMAP-2026-08-22-validate-detecta-hook-com-pwd-que-falha-fora-da-raiz)

## Problema

`classifyHookAnchorage` trata `~/scripts/...` como classe 2 (CwdDependent) porque:

```go
filepath.IsAbs("~/scripts/...") // retorna false — Go nao expande tilde
```

O tilde nao satisfaz nenhuma condicao da classe 1 (nao e `$CLAUDE_PROJECT_DIR/`, nao e absoluto via
`filepath.IsAbs`), e satisfaz a clausula catch-all da classe 2:

```go
!strings.HasPrefix(rawStripped, "$") && !filepath.IsAbs(rawStripped)
// "~/..." nao comeca com "$" e filepath.IsAbs retorna false → classe 2
```

## Por que e um falso-positivo

`~` expande para `$HOME` em qualquer shell POSIX. `~/scripts/foo.sh` resolve para
`/Users/user/scripts/foo.sh` independentemente do cwd — e semanticamente ancorado.

A mensagem emitida e factualmente errada:

> "this command only resolves from the project root and will silently fail when the agent's cwd is
> a subdirectory"

`~/scripts/...` nao falha quando o cwd e um subdirectorio — falha apenas se o arquivo nao existir
ou nao for executavel.

## Impacto critico: `~/.trackfw/scripts/trackfw-credential-guard.sh`

Este e o path que o trackfw usa para o **harness global** (criado por `trackfw update harness`). Se
um usuario configura esse path como hook global (para aplicar o guard em todos os projetos), o
`trackfw validate` acusa incorretamente. O usuario e instruido a rodar `trackfw update`, que
sobrescreve o hook por `$CLAUDE_PROJECT_DIR/scripts/...` — um path de escopo de projeto que nao
funciona como hook global.

Isso e exatamente o risco dominante identificado na REQ: falso-positivo faz o usuario desfazer
wiring intencional.

## Reproducao (todos os 3 CLIs)

```bash
# fixture
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '~/.trackfw/scripts/trackfw-credential-guard.sh'}]}]}}
print(json.dumps(d))
" > /tmp/test/.claude/settings.json

./bin/trackfw validate          # Go:  ACUSADO (bare relative path)
node npm/bin/trackfw validate   # Node: ACUSADO (bare relative path)  
PYTHONPATH=pypi python3 -m trackfw validate  # Python: ACUSADO (bare relative path)
```

## Correcao esperada

Adicionar `strings.HasPrefix(rawStripped, "~/")` como condicao de classe 1 em
`classifyHookAnchorage` — nos tres stacks:

- `internal/validator/validator_credential_guard.go`
- `npm/src/validator/index.js`
- `pypi/trackfw/validator.py`

Apos a correcao, adicionar caso parity em `scripts/check-validate-parity.sh`:
`cg-claude-tilde` (expected: SILENT) e `cg-claude-tilde-trackfw-harness` (expected: SILENT).

## Notas de instrumento

A barreira que descobriu isso (ML-3A) teve uma primeira sessao com medicao errada: Node.js foi
invocado via `node npm/src/index.js` (modulo nao encontrado) e Python via
`pypi/trackfw/cli.py` sem PYTHONPATH (ModuleNotFoundError). Ambos retornaram silencio falso,
levando a um veredito inicial errado (APROVADO COM RESSALVAS). A segunda sessao corrigiu os paths
e re-mediu — os tres stacks acusam `~/` identicamente.

**Licao:** ao verificar paridade de 3 CLIs, confirmar que cada invocacao produz saida real (nao
erro de modulo silenciado por grep) antes de registrar SILENT.
