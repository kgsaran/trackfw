# Windows: EscapeArg troca `"` por `\` em argumento sem espaço — gate malformado vira verde

**Data:** 2026-10-01 | **REQ:** #491 (Wave 4, ML-4A) | **Crédito da medição:** Lourival Garcia Junior (comentário #491, PR #495)

## Mecanismo

Quando o texto de um gate passa por argv (`exec.Command("sh", "-n", "-c", texto)` ou
`exec.Command("sh", "-c", texto)`), o Go aplica `EscapeArg` a cada elemento. Em Windows + MSYS,
um argumento **sem espaço** que contém `"` recebe a `"` escapada como `\"` (sem envoltória em
aspas externas). O MSYS repassa os argumentos ao processo filho reinterpretando `\"` como `\`,
produzindo:

| enviado ao Go      | recebido pelo sh no Windows |
|--------------------|----------------------------|
| `esperado="scaffold.go` | `esperado=\scaffold.go` |
| `x="ab`            | `x=\ab`                    |
| `echo "abre`       | `echo "abre` (com espaço: ida e volta fiel) |
| `a="x" b="y"`      | `a="x" b="y"` (com espaços: fiel) |

**Discriminante:** presença de espaço no argumento. Sem espaço, EscapeArg não envolve em aspas e
escapa o `"` embutido. Com espaço, envolve em aspas e a ida e volta é fiel.

## Consequência

`esperado=\scaffold.go` é uma **atribuição válida** em sh — o `\` antes de `s` é tratado como
escape de caractere literal `s`, resultando na string `scaffold.go`. O comando sai 0. Portanto:

- `sh -n` (fragment check, Wave 2 desta REQ): sai 0 → gate malformado **aprovado silenciosamente**
- `sh -c` (execução do gate, `runGateCommand`): também sai 0 → gate executa e **aprova como verde**

## Por que só apareceu de fora

Todas as medições da REQ #491 (Waves 0–3) rodaram em macOS (bash 3.2, `bash --posix`, dash). Em
macOS o argv não é mangledado — `esperado="scaffold.go` chega intacto ao `sh`. A Wave 0 declarou
"zero instâncias" num ambiente onde a forma não existe.

A medição foi feita por Lourival Garcia Junior em Windows 11, MINGW64, bash 5.2.26, Go 1.25.2,
chamando o binário compilado da branch `1290231b`. O achado entrou como ML corretivo desta REQ
pela Regra Dura de Causa Raiz (mesma causa: texto do gate passa por argv).

## Tabela de paridade macOS: stdin vs argv vs env-eval (exit + stdout)

Medida com `go run` em macOS sobre 12 vetores para `runGateCommand`. Colunas: `exit/stdout`.

| vetor | argv `sh -c` | stdin `sh` | env-eval | argv~stdin | argv~env |
|-------|-------------|-----------|---------|-----------|---------|
| `esperado="scaffold.go` | 2/"" | 2/"" | 1/"" | = | DIFF |
| `x="ab` | 2/"" | 2/"" | 1/"" | = | DIFF |
| `echo "abre` | 2/"" | 2/"" | 1/"" | = | DIFF |
| `a="x" b="y"` | 0/"" | 0/"" | 0/"" | = | = |
| `esperado="scaffold go` | 2/"" | 2/"" | 1/"" | = | DIFF |
| `false` | 1/"" | 1/"" | 1/"" | = | = |
| `true` | 0/"" | 0/"" | 0/"" | = | = |
| `exit 7` | 7/"" | 7/"" | 7/"" | = | = |
| `nosuchtool-xyz` | 127/"" | 127/"" | 127/"" | = | = |
| `cat` (stdin=devnull) | 0/"" | 0/"" | 0/"" | = | = |
| `read x; test -z "$x"` | 0/"" | 0/"" | 0/"" | = | = |
| `echo $0` | 0/"sh" | 0/"sh" | 0/"sh" | = | = |

**Veredito:** stdin tem paridade total com argv em macOS (todos os 12 vetores — exit code e stdout
idênticos). env-eval diverge em 5 vetores (exit 1 vs exit 2 para fragmentos com aspas). **Stdin
escolhido.**

Para `sh -n`: argv e stdin concordam nos 12 vetores em macOS. Em Windows, argv transforma
`esperado="scaffold.go` em 0 (FN); stdin preserva o fragmento e retorna 2 (TP).

## Correção aplicada (ML-4A)

Dois sítios em `internal/commands/barrier.go`:

```go
// checkGateFragments — antes:
c := exec.Command("sh", "-n", "-c", gc.Text)

// depois:
c := exec.Command("sh", "-n")
c.Stdin = strings.NewReader(gc.Text)

// runGateCommand — antes:
c := exec.Command("sh", "-c", command)

// depois:
c := exec.Command("sh")
c.Stdin = strings.NewReader(command)
```

Stdin é opaco ao `EscapeArg` — o texto chega ao processo byte-idêntico em todos os OSes.

## Teste de regressão

`TestBarrierFragment_TransportNoArgvMangling` (`internal/commands/barrier_fragment_test.go`):
cria gate `esperado="scaffold.go`, verifica que o barrier reporta `gates: blocked` com
`incomplete command` e que o sentinel não executa. No macOS passa porque o stdin transport é
usado; em Windows (CI `windows-full-suites`) prova que o argv mangling está fechado.
