# `merge=union` preserva todas as linhas mas **não** a ordem — e o único leitor que depende de posição é o cycle time do `metrics`

> 2026-09-02 · ML-1A do `ROADMAP-2026-09-02-gitattributes-com-merge-union-para-o-trackfw-log-nos-3-clis`
> Arquivos: `.gitattributes` (raiz), `internal/metrics/metrics.go`, `internal/generators/{scaffold,roadmap,req}.go`

## O mecanismo, em uma frase

`merge=union` **nunca conflita**, e é exatamente por isso que ele precisa de controle: ausência de
conflito parece sucesso, mas o driver concatena o bloco de `ours` inteiro **antes** do bloco de
`theirs` — num arquivo cujas linhas começam com timestamp, o resultado passa a estar fora de ordem
cronológica sem que nada avise.

Medido em repositório de rascunho (base `09:00`, `09:10`; `main` acrescenta `10:45` e `11:21`;
branch acrescenta `10:46`):

```
09:00 · 09:10 · 10:45 · 11:21 · 10:46      <- ordem resultante, exit 0, sem conflito
```

Sem o `.gitattributes`, o mesmo cenário dá `CONFLICT (content)` e `UU` — falsificado nas duas
direções.

## O que NÃO acontece (e é contraintuitivo)

- **Não duplica** quando os dois lados acrescentam a **mesma** linha: o xdiff trata a adição idêntica
  como mudança comum e o driver de união nem é chamado. Medido.
- **Não duplica** em sobreposição parcial (`ours` = `L1,L2`, `theirs` = `L1`) — resultado `L0,L1,L2`.
- **Não perde** linha em nenhum dos casos: igualdade de conjunto verificada, não só exit code.

O medo natural ("union engole ou duplica conteúdo") não se confirmou. O efeito real é **só** a ordem.

## Por que é caro descobrir isso amanhã

Quem for auditar a decisão vai querer saber quais leitores do `.trackfw-log` dependem de **posição**
e quais dependem de **timestamp**. A varredura já foi feita:

| Leitor | Depende da ordem do arquivo? |
|---|---|
| `trackfw log --tail N` | Apresentação (últimas N linhas por posição) — cosmético |
| `validator` `stale_wip` (`latestWIPTransitionTime`) | **Não** — `timestamp.After(latest)` |
| `metrics` throughput | **Não** — min/max de timestamp |
| `metrics` cycle time e WIP age (`Calculate`) | **SIM** — primeira entrada `backlog`/`wip` e última `done`/`wip` **na ordem do slice**, que é a ordem do arquivo |
| `serve` `api_metrics` | Mesmo parser posicional do `metrics` |

A exposição do `Calculate` é estreita: só é atingida por um roadmap com transições registradas nos
**dois** lados do merge. E a alternativa é pior — o `.trackfw-log` deste repositório carrega hoje uma
linha **duplicada** (`2026-09-02 10:46 … gate-do-barrier`) produzida por uma **resolução manual** de
conflito. `merge=union` não teria duplicado. O trade-off é: ordem embaralhada em caso raro (union)
contra linha perdida ou duplicada em todo merge (resolução manual, medida).

## A parte do `.gitattributes` que se erra na primeira tentativa

O padrão é `.trackfw-log` **sem barra**, e isso é decisão, não descuido: padrão sem `/` casa o
basename em **qualquer** diretório. Há **dois** logs — `roadmap_dir/.trackfw-log` e
`req_dir/.trackfw-log` (`req.go` `appendREQTransitionLog`) — e os dois diretórios são configuráveis
por projeto no `trackfw.yaml`. Um padrão com caminho fixo nasce quebrado em qualquer projeto que
tenha configurado outro diretório, e deixa o log de REQ descoberto mesmo no layout padrão.
Confirmado com `git check-attr merge` em `docs/roadmaps/`, `docs/req/` e um `custom/rm/` arbitrário:
`merge: union` nos três.

## Não coberto (declarado, não corrigido)

- `trackfw update` **não** emite o arquivo — só projeto inicializado depois desta versão recebe a
  regra. Projeto existente precisa acrescentar à mão (ou rodar `init` de novo: o append é idempotente).
- A falsificação é sobre `git merge` **local**. Se o merge do lado do servidor da forge honra o
  atributo não foi medido aqui.
