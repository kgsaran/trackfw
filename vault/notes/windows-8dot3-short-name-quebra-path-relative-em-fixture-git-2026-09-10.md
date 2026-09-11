# Windows 8.3 short names quebram `path.relative` em fixtures que usam git

> 2026-09-10 · descoberto ao corrigir ML-W3B da `ROADMAP-2026-09-10-barrier-executa-gate-de-roadmap-nao-confiavel`

## O sintoma

Dois testes F3 sentinel falham **só no CI do Windows** (`runneradmin`), não no Mac nem no VM de dev
(usuário `Lab`):
- `F3 sentinel: local content differs from origin/main prevents gate execution` (Node.js)
- `test_f3_sentinel_content_differs_from_origin_prevents_gate_execution` (Python)

O test runner reporta que `gates.failures` contém `"roadmap is not committed in origin/main"` em vez
de `"roadmap content differs from origin/main"`.

## A causa raiz

No GitHub Actions runner, o usuário é `runneradmin`. O Windows abrevia nomes longos para o formato 8.3:
`runneradmin` → `RUNNER~1`.

```
TEMP = C:\Users\RUNNER~1\AppData\Local\Temp   ← 8.3 short form
```

Dentro do fixture, `fs.mkdtempSync` / `tempfile.mkdtemp` herdam este `TEMP`. O caminho do clone fica
em `C:\Users\RUNNER~1\...\clone`.

Na produção, `git rev-parse --show-toplevel` **expande** o nome curto para o nome longo:

```
git output: C:/Users/runneradmin/AppData/Local/Temp/tw-trust-sentinel-xxx/clone
```

`path.relative` (Node.js) e `os.path.relpath` (Python) fazem comparação textual. Quando `root` tem
o nome longo e `absRoadmap` tem o nome curto (ou vice-versa), o resultado é garbage:

```
// Medido com probe3.js no Windows ARM64 VM:
path.relative(
  'C:/Users/RUNNER~1/.../clone',       // root (short)
  'C:\\Users\\runneradmin\\...\\clone\\docs\\roadmaps\\wip\\ROADMAP.md'  // abs (long)
)
→ '..\..\..\..\..\..\runneradmin\AppData\...'   // GARBAGE, não o caminho relativo esperado
```

`git cat-file -e refs/remotes/origin/main:<garbage-path>` → exit 128 → função retorna "not
committed" → teste espera "content differs" → **FALHA**.

## Por que Go passa

`makeTrustGitFixture` (Go) chama `filepath.EvalSymlinks(base)` imediatamente após `t.TempDir()`.
O comentário diz "macOS symlinks", mas **no Windows esta chamada também expande nomes 8.3 para nomes
longos**. Com `base` em nome longo, todos os caminhos derivados também ficam em nome longo, e
`filepath.Rel(topLevel, absRoadmap)` funciona corretamente.

## Por que o VM de dev não reproduz

O usuário é `Lab` (3 caracteres). Não há abreviação 8.3. `TEMP` e git retornam o mesmo caminho
longo. `path.relative` funciona. O teste passa.

Para forçar a reprodução no VM: criar diretório de nome longo, obter 8.3 via `GetShortPathName`,
setar `TEMP`/`TMP` para o caminho curto, rodar o teste. (8.3 names devem estar habilitados no volume;
no ARM64 VM de dev eles estavam desabilitados — `NtfsDisable8dot3NameCreation`.)

## A correção

No **fixture de teste** (não na produção), adicionar normalização após `mkdtemp`:

### Node.js (`npm/tests/barrier.test.js`)
```js
let base = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-trust-sentinel-'))
try { base = fs.realpathSync.native(base) } catch (_) { /* best-effort */ }
```

**ATENÇÃO: `fs.realpathSync` (sem `.native`) NÃO expande 8.3 no Windows.**
Medido em 2026-09-11 no Windows ARM64 VM:

| API | Input | Output | Expande 8.3? |
|-----|-------|--------|-------------|
| `fs.realpathSync` | `C:\Users\Lab\TW-MEA~2` | `C:\Users\Lab\TW-MEA~2` | **NÃO** |
| `fs.realpathSync.native` | `C:\Users\Lab\TW-MEA~2` | `C:\Users\Lab\tw-measure-longname-test-...` | **SIM** |
| `path.resolve` | `C:\Users\Lab\TW-MEA~2` | `C:\Users\Lab\TW-MEA~2` | NÃO |
| `path.win32.resolve` | `C:\Users\Lab\TW-MEA~2` | `C:\Users\Lab\TW-MEA~2` | NÃO |

`fs.realpathSync` usa uma implementação JS (lstat/readlink loop) que resolve symlinks mas
**não chama `GetFinalPathNameByHandleW`** e não expande nomes 8.3.
`fs.realpathSync.native` chama `uv_fs_realpath` → `GetFinalPathNameByHandleW`, que retorna
o nome longo canônico.

Isso espelha o comportamento Go: `filepath.EvalSymlinks` usa `GetFinalPathNameByHandleW`
internamente e expande 8.3. O análogo correto em Node é `.native`, não a versão JS.

### Python (`pypi/tests/test_barrier.py`)
```python
base = Path(os.path.realpath(tempfile.mkdtemp(prefix="tw-trust-sentinel-")))
```

`os.path.realpath` no Windows chama `GetFinalPathNameByHandleW` e expande 8.3 — análogo ao
`fs.realpathSync.native` do Node e ao `filepath.EvalSymlinks` do Go.

## Residual declarado na produção

O mesmo sítio existe na **produção** (`roadmapTrustForGates`). Se um usuário real tiver o repo
sob um caminho com 8.3 short names, a função retorna "not committed" em vez de "content differs".
**Fail-closed** — gates não executam. O usuário pode passar `--trust-local-gates` como workaround.
Corrigir a produção exigiria normalizar `absRoadmap` (derivado de `path.resolve`/`os.path.abspath`
sobre o caminho passado pelo chamador, que pode estar em TEMP com nome curto) com
`fs.realpathSync.native` (Node) / `os.path.realpath` (Python) / `filepath.EvalSymlinks` (Go)
antes de computar o `relPath` — não feito neste ML. (`topLevel` já vem
canônico do `git rev-parse --show-toplevel` e não precisa de normalização.)

## Padrão generalizável

> Toda fixture que (a) cria diretórios temporários via `TEMP` **e** (b) compara caminhos contra
> saída de `git rev-parse --show-toplevel` deve normalizar o base path antes de construir qualquer
> caminho derivado.

Git sempre retorna nomes longos. `TEMP` no Windows pode retornar nomes curtos. A divergência
quebra qualquer comparação textual de caminhos.
