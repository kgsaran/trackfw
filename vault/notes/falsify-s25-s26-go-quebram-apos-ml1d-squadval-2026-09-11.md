# Cenários s25-go e s26-go quebram após ML-1A adicionar squadVal a roadmap.go

> Data: 2026-09-11 | Autor: Hefesto | Domínio: gates / falsificação
> Padrão: [cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02](cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md)

## Sintoma

`make parity-falsify` reporta:

```
run-gates-falsify-parallel: GUARDA -- chunk_6 nao chegou ao sentinela CHUNK_COMPLETE
  (ultima linha do log: '[s25-go] expected exactly 1 occurrence of pattern, got 0')
run-gates-falsify-parallel: GUARDA -- chunk_4 nao chegou ao sentinela CHUNK_COMPLETE
  (ultima linha do log: '[s26-go] expected exactly 1 occurrence of pattern, got 0')
```

Seguido por dezenas de rótulos AUSENTE em cascata (incluindo todos os rótulos do
`roadmap-ref-stale-state/python/` que foram o sintoma original reportado).

## Causa

O **ML-1A** (entrou na árvore com o commit `9d042b8e` da branch
`fix/by-agent-req-new-e-roadmap-new`) adicionou o
parâmetro `squadVal` à chamada `fmt.Sprintf` em `internal/generators/roadmap.go`.

### s25-go

Antes do ML-1A (`origin/main`):
```go
date, reqPath, title, date, filepath.Base(reqPath), reqPath, adrRef, mlSection.String())
```

Depois do ML-1A (branch):
```go
date, reqPath, squadVal, title, date, filepath.Base(reqPath), reqPath, adrRef, mlSection.String())
```

O `corrupt_literal` de s25-go mirava o literal antigo → count=0 → SystemExit → set -e mata chunk_6.

### s26-go

Antes do ML-1A (`origin/main`):
```go
, date, content.REQPath, content.Title, date, content.REQPath)
```

Depois do ML-1A (branch):
```go
, date, content.REQPath, squadVal, content.Title, date, content.REQPath)
```

O `corrupt_literal` de s26-go mirava o literal antigo → count=0 → SystemExit → set -e mata chunk_4.

## Relação com o sintoma original

O usuário reportou apenas 4 rótulos Python ausentes (`roadmap-ref-stale-state/python/*detects*`).
Esses rótulos estão em chunk_6, depois de s25. Como chunk_6 morre em s25, TODOS os rótulos
após s25 ficam ausentes — incluindo os 4 Python. O sintoma reportado é subconjunto do efeito.

## Diagnóstico A/B

- `origin/main`: count=1 para ambos os literais → chunks correm normalmente.
- Branch: count=0 para ambos → os dois chunks morrem no setup.

Confirmado por execução direta de `chunk_6.sh` com `TRACKFW_ROOT_DIR=...`.

## Correção aplicada

Retarget dos dois `corrupt_literal` em `scripts/check-gates-falsify.sh`:

| Cenário | Arg 1 (old→new) | Arg 2 (replacement, ajuste mínimo) |
|---|---|---|
| s25-go | adiciona `squadVal,` após `reqPath,` | idem na posição 2 (corruption mantém) |
| s26-go | adiciona `squadVal,` após `content.REQPath,` | idem na posição 3 |

A intenção do cenário é preservada: a corrupção continua substituindo `reqPath`/`content.REQPath`
por uma versão truncada no argumento de título, o que o validator detecta.

Verificação: `make parity-falsify` → RC=0, 414 OK, 0 FAIL, guarda OK.

## Regra de prevenção

Toda vez que um ML modifica a assinatura de um `fmt.Sprintf` em `internal/generators/roadmap.go`
(ou em qualquer arquivo que seja alvo de `corrupt_literal`), verificar o count dos literais
antes de commitar:

```bash
grep -c '<literal-alvo-do-cenario>' internal/generators/roadmap.go
```

Se count != 1, retargetar o `corrupt_literal` correspondente **no mesmo commit** antes de fechar o ML.

Referência: [cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02](cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md)

---

## Correção de atribuição (arquiteto, 2026-09-11)

O relatório original atribuiu o `squadVal` ao **ML-1D** (`274dce77`, título posicional). Medido com a
régua certa:

```bash
$ git log --oneline origin/main..HEAD -S"squadVal" -- internal/generators/roadmap.go
9d042b8e chore(governance): barreira da Wave 1 BLOQUEIA — ...
```

Foi o **ML-1A**, que entrou na árvore junto com aquele commit de governança. A causa, a medição e a
correção estavam certas; **só o culpado estava errado** — e um `git log -S` desfaz a dúvida em um
comando.

⚠️ Vale como método: **atribuir por proximidade temporal é chute.** `-S` busca por quem introduziu o
próprio texto.

## 🔴 O sintoma apontou o cenário ERRADO

O `make quality` acusou **quatro rótulos do s193** (`roadmap-ref-stale-state/python/*`) ausentes. Gastei
tempo investigando o s193 — conferi os dois literais de mutação dele, os dois presentes e únicos, em
branch e main.

**O s193 nunca teve problema.** O chunk morria em `set -e` no **s25**, muito antes de chegar nele. Os
rótulos ausentes eram **tudo que vinha depois do ponto de morte** — não o defeito.

**Regra:** num runner que aborta por `set -e`, "rótulo ausente" nomeia **onde o script parou de
imprimir**, não onde ele quebrou. O primeiro rótulo ausente da ordem de execução é a pista; os demais
são consequência. A guarda de sentinela (`CHUNK_COMPLETE`) é o que distingue as duas coisas — e foi ela
que resolveu.
