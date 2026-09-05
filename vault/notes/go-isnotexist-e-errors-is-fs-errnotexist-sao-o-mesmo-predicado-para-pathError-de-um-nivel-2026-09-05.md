# `os.IsNotExist` e `errors.Is(err, fs.ErrNotExist)` são o mesmo predicado para `*PathError` de um nível — a troca do #276/ML-1C é modernização de idioma, não fix de ENOTDIR

> 2026-09-05 · apolo-tf · ML-1C, `ROADMAP-2026-09-05-fechar-os-tres-defeitos-mecanicos-dos-issues-do-consumidor-externo.md`

## O que a issue #276 afirmava

Que `os.IsNotExist(err)` classifica `ENOTDIR` como ausência (`true`) enquanto
`errors.Is(err, fs.ErrNotExist)`/`errors.Is(err, syscall.ENOENT)` distinguiria
corretamente — e que essa era a causa do sexto sítio de predicado de
plataforma (`internal/integrations/manager.go:477`, e por extensão os 5 já
"corrigidos" em `internal/validator/validator.go`).

## O que a leitura do GOROOT mostra (não é opinião, é o fonte instalado)

```
os.IsNotExist(err) = underlyingErrorIs(err, ErrNotExist)
                     = underlyingError(err) (desembrulha 1 nível: *PathError/*LinkError/*SyscallError → .Err)
                       → err == target || err.(syscall.Errno).Is(target)

errors.Is(err, fs.ErrNotExist) = desembrulha via Unwrap() recursivamente
                                  → mesmo err.(syscall.Errno).Is(fs.ErrNotExist) no fundo
```

Para um erro de **um nível só** — exatamente o que `os.ReadDir`/`os.Stat`/
`os.Open`/`os.ReadFile` devolvem sem wrapping adicional (`*PathError`
contendo `syscall.Errno`) — os dois predicados chamam o **mesmo método**
(`syscall.Errno.Is`) com o **mesmo argumento**. Não podem divergir, em
nenhuma plataforma, para essa forma de erro. Medido com `go run` contra um
ENOTDIR real em macOS: `os.IsNotExist == errors.Is(fs.ErrNotExist) == false`
para ambos.

E em `src/syscall/zerrors_windows.go`:

```go
ENOENT  Errno = ERROR_FILE_NOT_FOUND
ENOTDIR Errno = ERROR_PATH_NOT_FOUND
```

`Errno.Is(oserror.ErrNotExist)` no Windows (`syscall_windows.go`) inclui
`ERROR_PATH_NOT_FOUND` explicitamente — ou seja, no Windows o próprio Go
mapeia `ENOTDIR` (que ali *é* `ERROR_PATH_NOT_FOUND`) para "não existe", de
propósito, porque o Windows não tem um código de erro distinto para
"componente do caminho não é diretório" separado de "caminho não encontrado"
— os dois casos colidem no mesmo `ERROR_PATH_NOT_FOUND` na API do SO. Isso
vale tanto para `os.IsNotExist` quanto para `errors.Is(err, fs.ErrNotExist)`.

## A alternativa que a própria issue propôs também não serve

A issue #276 sugere `errors.Is(err, syscall.ENOENT)` como o predicado
"correto". Mas no Windows `ENOENT = ERROR_FILE_NOT_FOUND` é um código
**diferente** de `ENOTDIR = ERROR_PATH_NOT_FOUND` — e um diretório-pai
genuinamente ausente (o caso legítimo que a supressão precisa preservar,
p.ex. `~/.claude/agents/` ainda não criado num install limpo) tipicamente
também sai como `ERROR_PATH_NOT_FOUND` no Windows, não `ERROR_FILE_NOT_FOUND`.
Trocar para `errors.Is(err, syscall.ENOENT)` pararia de suprimir o caso
legítimo no Windows — regressão disfarçada de fix, não medida em CI real.

## Onde a troca `os.IsNotExist` → `errors.Is(err, fs.ErrNotExist)` FAZ diferença

Só quando o erro está embrulhado em **mais de um nível** antes do check —
ex. `fmt.Errorf("lendo %s: %w", path, err)` seguido de outro `%w` mais
adiante, e só então testado. `underlyingError` desembrulha exatamente um
nível (`*PathError`/`*LinkError`/`*SyscallError`); `errors.Is` desembrulha
recursivamente via `Unwrap()` até o fundo. Varredura feita nesta sessão
(`grep -B8 os.IsNotExist` cruzado com `fmt.Errorf.*%w` nos 8 linhas
anteriores) não achou nenhum sítio no código de produção (não-teste) do
repositório com esse padrão — todos os ~40 sítios de `os.IsNotExist` são
checks diretos sobre o retorno de `os.ReadFile`/`os.Open`/`os.Stat`/
`os.ReadDir`/`os.Lstat`, sem wrapping prévio.

## Consequência prática

- A troca em `manager.go:477` (ML-1C) foi mantida — é o idioma recomendado
  pela própria doc do Go ("New code should use errors.Is(err,
  fs.ErrNotExist)") e comprovadamente sem risco de regressão — mas rotulada
  como **modernização de idioma**, não como fechamento do mecanismo do
  #276.
- **O #276 não deve ser fechado** com base neste diff: o mecanismo alegado
  (ENOTDIR classificado como ausência por causa do predicado) está refutado
  para este sítio e, pelo mesmo argumento estrutural, para os 5 sítios já
  tocados em `internal/validator/` nesta campanha, se usaram o mesmo
  raciocínio.
- Uma correção real (se necessária) exigiria inspecionar o componente do
  caminho ofensivo diretamente (ex. `os.Stat` em cada segmento até achar o
  que não é diretório), não trocar o predicado de classificação do erro —
  decisão de produto fora do escopo deste ML.

## Addendum (2026-09-05, ML-1A) — a medição de Windows agora é um dado real, não só leitura de GOROOT

A leitura de GOROOT acima (`ENOTDIR Errno = ERROR_PATH_NOT_FOUND` em `zerrors_windows.go`) previa o
comportamento, mas o ML-1C não tinha uma execução real em Windows para confirmá-la — o teste que
essa mesma sessão escreveu (`manager_collision_enotdir_test.go`) afirmava o oposto e só foi
falsificado depois, pelo CI de Windows (run `33991655271`):

```
manager_collision_enotdir_test.go:58: detectNameCollision(ENOTDIR) = nil,
    want a reported error (ENOTDIR must not be classified as absence)
```

`err == nil` nessa mensagem é o dado empírico: em `windows-latest`, `detectNameCollision` suprime a
condição ENOTDIR construída pelo teste (arquivo real onde um componente de diretório era esperado),
exatamente como suprimiria um diretório genuinamente ausente. Isso confirma, por execução real, a
previsão feita só por leitura de GOROOT.

Reconciliado no ML-1A (`ROADMAP-2026-09-05-reconciliar-o-que-declaramos-com-o-que-medimos-apos-a-auditoria-externa.md`):
o teste passou a afirmar comportamento por plataforma (`runtime.GOOS`) em vez de uma expectativa
única — em POSIX, `err != nil`; no Windows, `err == nil`, mas só depois de confirmar via
`os.ReadDir` bruto que `errors.Is(rawErr, fs.ErrNotExist)` é verdadeiro no runner real (afirma o
mecanismo, não só o resultado). Nenhum `t.Skip` foi usado.

## Ligado a

- [[dedup-guard-path-cego-a-backslash-no-windows-2026-09-05]] — outro caso
  desta campanha onde a plataforma (Windows) conflacionava dois conceitos
  que o POSIX distingue.
