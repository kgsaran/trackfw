---
name: proibir-make-quality-cega-o-executor
description: Proibir `make quality` no handoff é certo com frente paralela, mas cega o executor para regressão em gate vizinho — autorize no ML final
metadata:
  type: feedback
---

**Proibir `make quality` nos handoffs é defensável — mas cega o executor para regressão em gate
vizinho, e o custo volta inteiro no fim.** Autorize `make quality` explicitamente **no último ML da
wave**, quando não houver frente paralela.

**Why:** em 2026-09-28 (#445) proibi `make quality` em **todos** os handoffs (custo de CPU e medição
sobre árvore em movimento — ver [[paralelismo-tem-custo-de-cpu]]). Pedi a remoção do marcador
`consumer-config-merge-allowed:` de um sítio; o executor removeu o **comentário inteiro**, levando
junto o `// write-containment-allowed:` — marcador de **outro gate** (`check-write-containment`), que
existia na `main`.

Nenhum executor podia ver: os comandos que autorizei (`go build`, `go test`, o gate específico do ML)
**todos passavam**. Só apareceu quando **eu** rodei `make quality` no fim, e custou mais um ciclo de
ML.

**How to apply:**
- MLs em paralelo → proibido `make quality` (a razão original continua válida)
- **último ML da wave, sem frente paralela → autorize, e diga que é o AC principal**
- Ao pedir remoção de comentário/marcador, **enumere no handoff os marcadores que devem PERMANECER**
  e por quê. Marcadores de gates diferentes moram lado a lado e parecem a mesma coisa.

Relacionado: [[paralelismo-tem-custo-de-cpu]], [[mudar-contrato-auditar-quem-afirma-o-antigo]].
