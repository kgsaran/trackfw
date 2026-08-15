# Go `regexp` (RE2) não suporta backreferences — falha em runtime, não em `go build`

> 2026-08-15 · ML-1A do `ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas`
> (`internal/thirdparty/markers.go`, `CheckMarkers` — critério D3)

## O erro de enquadramento

D3 (`docs/adr/ADR-2026-08-15-gate-de-duas-fases-...md`) descreve remover blocos cercados (``` e
~~~) como se fosse um regex único: algo como

```
(?ms)^(```|~~~).*?^\1[^\n]*$
```

Isso é válido em PCRE (JS, Python `re`, Ruby...), mas **não compila como regex válido em Go** — na
verdade compila silenciosamente no sentido de que `go build` **não acusa nada**, porque o padrão é
uma string comum até ser passada para `regexp.MustCompile` em **tempo de execução**. O panic só
aparece ao rodar o binário/teste:

```
panic: regexp: Compile("..."): error parsing regexp: invalid escape sequence: `\1`
```

`go vet` também não pega isso — não avalia o conteúdo de strings passadas para `regexp.Compile`.

## Por que isso importa aqui

`internal/thirdparty/markers.go` é código canônico Go que a Wave 2 deste roadmap (ML-2A Node, ML-2B
Python) vai portar 1:1. Em Node (`RegExp`, motor V8/Irregexp) e em Python (`re`), backreferences
**funcionam normalmente** — então um porte literal do texto do ADR funcionaria nesses dois CLIs e
falharia (ou pior, precisaria de outra reescrita) só no Go, se alguém tentasse "simplificar" o Go
para bater com o texto do ADR depois do fato.

## A regra

**Nunca usar backreference (`\1`, `\2`...) num `regexp.MustCompile` em Go.** Se o algoritmo do ADR/
parecer pede correspondência entre abertura e fechamento de um delimitador variável (aqui: ``` vs
~~~, e a contagem de repetições do CommonMark), implemente como **scanner linha a linha em Go**
(estado explícito de "dentro/fora de fence" + comparação de string), não como regex único. Foi o que
`removeFencedBlocks` em `internal/thirdparty/markers.go` faz.

## Como reconhecer que você caiu nisso

- `go build ./...` passa limpo.
- `go vet ./...` passa limpo.
- **Qualquer** `go test` ou execução do binário que exercite o caminho de código com o
  `regexp.MustCompile` panica com `invalid escape sequence` (se a var é de pacote, o panic ocorre
  na inicialização do pacote — atinge **qualquer** teste do pacote, não só o que exercitaria a
  função).

## Relacionado

- `internal/thirdparty/markers.go` — `removeFencedBlocks`, `fencePrefixPattern`.
- `docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md`
  D3 — a emenda do arquiteto que introduziu a remoção de blocos cercados.
- Para quem for portar em ML-2A/ML-2B: pode implementar com backreference nativo (JS/Python `re`)
  se preferir — só **não replique o scanner linha a linha do Go por engano**, achando que é
  exigência do ADR; é só a forma como o Go contorna a limitação do RE2.
