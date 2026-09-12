# Python `abspath` não resolve symlink — macOS `/var` → `/private/var` (ML-3C, 2026-09-12)

## Problema

`_agent_from_req_path` no Python CLI comparava dois caminhos para derivar o agente de uma REQ:

- `abs_req_dir` = resultado de `os.path.abspath(cfg["req_dir"])` → passa por `os.getcwd()` internamente, que retorna forma **canônica** após `cd` em symlink
- `abs_req` = resultado de `os.path.abspath(req_path)` onde `req_path` é um caminho absoluto não-canônico passado pelo usuário

No macOS, `/tmp` e `/var` são symlinks para `/private/tmp` e `/private/var`.
`os.getcwd()` retorna `/private/var/folders/...` (canônico), mas um caminho `/var/folders/...`
passado explicitamente fica **inalterado** por `abspath` — ele normaliza `.`/`..` mas não resolve symlinks.

A comparação `req_grandparent != abs_req_dir` falhava → caía em `resolve_write_agent` → erro de ambiguidade.

## Causa raiz técnica

```python
# ERRADO — um lado é canônico, o outro não é:
abs_req_dir = os.path.abspath(cfg["req_dir"])   # canônico via getcwd()
abs_req = os.path.abspath(req_path)             # não-canônico se req_path = /var/...

# CORRETO — ambos os lados canonicalizados com realpath:
abs_req_dir = os.path.realpath(os.path.abspath(req_dir))
abs_req     = os.path.realpath(os.path.abspath(req_path))
```

## Fix aplicado

`_agent_from_req_path` em `pypi/trackfw/generators/roadmap.py` passou a receber `req_dir` como segundo
argumento e aplica `os.path.realpath(os.path.abspath(...))` nos **dois lados** antes de `os.path.relpath`.

O guard em `_cmd_new` (`pypi/trackfw/commands/roadmap.py`) foi simplificado para delegar a detecção
inteiramente a `_agent_from_req_path`.

## Padrão equivalente nos outros runtimes

| Runtime | Função | Resolução de symlink |
|---------|--------|---------------------|
| Go | `agentFromPath` em `internal/generators/roadmap.go` | `filepath.EvalSymlinks` nos dois lados |
| Node | `agentFromPath` em `npm/src/generators/roadmap.js` | `fs.realpathSync` nos dois lados |
| Python | `_agent_from_req_path` em `pypi/trackfw/generators/roadmap.py` | `os.path.realpath(os.path.abspath(...))` nos dois lados (agora) |

## Padrão relacionado já documentado

Ver [Windows 8.3 short names quebram `path.relative` em fixtures que usam git](windows-8dot3-short-name-quebra-path-relative-em-fixture-git-2026-09-10.md) —
mesma família de problema: string comparison de paths que o OS trata como equivalentes mas que
têm representações textuais diferentes.

## Sítios varridos (varredura AC2 do ML-3C)

- `pypi/trackfw/validator.py:564-565` (`_is_subpath`): usa `abspath` nos dois lados, MAS o único
  chamador (linha 1309) já passa `os.path.realpath(...)` — não é defect, não foi tocado.
- Go `internal/generators/roadmap.go:204`: `filepath.Abs(cfg.REQDir)` sem EvalSymlinks, MAS resultado
  vai para `os.Stat` (transparente a symlinks), não a comparação de string — não é defect.
- Node `agentFromPath`: já usava `realpathSync` nos dois lados — correto.

## Teste pattern

Para testar caminho não-canônico sem usar `pwd -P` (que quebraria a fixture):

```python
# Criar symlink para o tmp_path e passar o caminho via symlink
link = tmp_path.parent / ("symlink_ml3c_" + tmp_path.name[-6:])
symlink_or_skip(link, tmp_path)  # skip no Windows sem Developer Mode
req_via_link = str(link / "req" / "beta" / req_basename)
# req_via_link é /var/... mas tmp_path é /private/var/...
agent = _agent_from_req_path(req_via_link, str(tmp_path / "req"))
assert agent == "beta"
```
