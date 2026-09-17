# chdir+RemoveAll não força os.Getwd() a falhar no Windows

**Data:** 2026-09-17
**Contexto:** ML-1A-quater · PR #382 · `fix/sync-enumera-req`

## Fato

`TestSyncToProvider_GetWdFails_FailClosed_ML1Abis` usava a seguinte técnica para
forçar `os.Getwd()` a falhar:

```go
os.Chdir(dir)      // entra no dir
os.RemoveAll(dir)  // remove o CWD
// os.Getwd() falha a partir daqui (no Linux/macOS)
```

No **Windows**, o SO bloqueia a remoção de qualquer diretório que esteja em uso como
CWD de algum processo. `os.RemoveAll` retorna:

```
unlinkat C:\...\001: The process cannot access the file because it is being used by another process.
```

O teste chamava `t.Fatalf` nesse ponto e foi relatado pelo ratchet como **falha nova**
(`+1 NOVO`), não como skip.

## Solução correta

Tornar `os.Getwd` injetável via variável de pacote exportada no pacote `validator`:

```go
// internal/validator/validator.go
var GetwdFn func() (string, error) = os.Getwd
```

O teste injeta a falha diretamente, sem depender de truque de sistema de arquivos:

```go
validator.GetwdFn = func() (string, error) {
    return "", errors.New("injected: getwd unavailable")
}
```

O mesmo seam permite exercitar o segundo ramo fail-closed (`EvalSymlinks` falha):
injetar `GetwdFn` retornando um caminho inexistente faz `filepath.EvalSymlinks`
falhar deterministicamente em **todas as plataformas**.

## Por que `export_test.go` não resolve

`export_test.go` com `package validator` só expõe símbolos para testes compilados
DENTRO do pacote `validator`. Testes em `package sync` importam `validator` como
dependência comum — os símbolos de `export_test.go` não são incluídos nessa
compilação. O seam precisa ser exportado no código de produção (não em `_test.go`)
para ser visível a outros pacotes de teste.

## Invariante que garantiu a portabilidade

`filepath.EvalSymlinks(caminho-inexistente)` falha em todas as plataformas (Linux,
macOS, Windows). Não requer truque de filesystem, permissão especial nem symlink.
É a técnica correta para testar o ramo "EvalSymlinks falha" de forma portável.

## Plataformas afetadas

| Plataforma | Comportamento de `os.RemoveAll(CWD)` |
|---|---|
| Linux | Sucesso — o inode permanece acessível via `/proc/self/cwd` |
| macOS | Sucesso — o diretório é desvinculado mas o fd permanece aberto |
| Windows | **Falha** — SO bloqueia enquanto algum processo usa o diretório |

## Residual conhecido

`TestSyncToProvider_REQDirSymlink_AC7` usa `symlinkOrSkip` (guarda nomeada). No
Windows sem Developer Mode, o teste **pula** (symlink exige privilégio). No runner
do GitHub Actions usado em `windows-full-suites`, o teste **passou** (não pulou),
então a garantia AC7 está exercitada naquele ambiente. Se o runner mudar para um sem
Developer Mode, o teste voltará a pular — aceitável, pois é um `t.Skip` nomeado,
não um falso vermelho.
