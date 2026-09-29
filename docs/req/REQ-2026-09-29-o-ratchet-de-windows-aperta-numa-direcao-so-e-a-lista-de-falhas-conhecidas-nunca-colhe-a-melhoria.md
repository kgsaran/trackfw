---
status: Done
date: 2026-09-29
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-nunca-colhe-a-melhoria.md"
---

# REQ: o ratchet de Windows aperta numa direção só, e a lista nunca colhe a melhoria

> Date: 2026-09-29 | Status: Done
| Linear Issue:
| Jira Issue:

Origem: **#364**, recapturado do **#329** antes de fechá-lo — o objeto do #329 sumiu com a v8, mas a
observação declarada acionável não.

## Motivation

O ratchet reporta `[-1 resolvido]` quando uma falha declarada passa. Esse sinal **não reprova, não
fecha nada, e persiste** — no #329 foram **4 corridas seguidas** com o mesmo `-1`.

🔴 **O ratchet aperta numa direção só:** bloqueia regressão nova e **nunca colhe a melhoria**. A lista
só cresce, e uma entrada obsoleta fica **indistinguível** de uma legítima.

### A medição que o issue pedia como primeiro passo — feita

O #364 declara: *"sem medição de quantas entradas hoje são obsoletas, nenhuma das três opções se
justifica sozinha"*. Medido em **2026-09-29**, extraindo do log do `windows-full-suites` de **dois**
runs da `main` (18/09 e 29/09) — **sem gastar run novo**, porque o job já executa tudo:

```
1677 testes PASS
  14 testes FAIL  →  e são EXATAMENTE as 14 entradas da lista
   0 falhas fora da lista     (nenhuma regressão)
   0 entradas obsoletas       (nenhuma já passa)
```

🔴 **A lista está exata: zero passivo.** E isso **decide a política** — ver **D6** da ADR: a opção
"reprovar quando resolve" é a única que fica **mais barata quanto mais cedo** for adotada, e está no
ponto mais barato **agora**.

⚠️ **Correção de uma suposição minha:** eu supunha que parte das 14 já passava após as correções
recentes de Windows (CRLF, junção, contenção, shim MSYS do #466). **Falso, nos dois runs.** O ratchet
não colher melhoria **não implica** que haja melhoria a colher — inferi a segunda coisa da primeira.

### O segundo achado, que sustenta o primeiro

Uma entrada é hoje `{name, runtime, class}` — **nenhuma diz por que falha**. Sem isso, o **D6** não
se sustenta: quando o job reprovar dizendo *"esta entrada resolveu"*, quem não souber por que ela
falhava **não consegue julgar** se resolveu de verdade ou se o ambiente mudou.

## Acceptance Criteria

- [x] 🔴 **D6 — falha declarada que resolve REPROVA o job**, nomeando a entrada e apontando o
      protocolo do **D4** (mover para `removed[]` com `removal_note`)
- [x] 🔴 **Falsificação nas duas direções:** entrada que resolve → **reprova** · lista fiel ao
      observado → **passa**. E o contra-braço que importa: **regressão nova continua reprovando**
      (o D6 não pode ter desligado o D1)
- [x] 🔴 **D7 — as 14 entradas ganham a razão da falha**, e o esquema aceita o campo sem quebrar o
      checker existente
- [x] **Triagem por causa-raiz:** as 14 agrupadas por mecanismo (permissão POSIX · CRLF · `bash`
      ausente · outro), com a razão **medida**, não presumida
- [x] **O checker valida o campo novo** — entrada sem razão reprova, senão o D7 degrada em opcional
- [x] `make quality` e **CI** verdes, **incluindo o `windows-full-suites`** — é o job que este
      trabalho altera

### Evidência (PR #479, mergeado em 2026-09-29)

| AC | evidência |
|---|---|
| D6 reprova | `ML-D6: Go 'TestB1' is in the known list but PASSED -- move to removed[] with removal_note (D4)` |
| falsificação nas 2 direções | B1 → reprova · lista fiel → **RC=0** · regressão nova → `[+1 NOVO]`, o D1 não foi desligado |
| D7 nas 14 | 14/14 com `reason`, zero não-ASCII, **9 `(MEASURED)` / 5 `INFER`**, conferível por grep estrito |
| triagem por causa-raiz | Wave 0 (`hades-tf`), grupos A=4 · B=4 · C=2 · D=4, **soma 14** — parecer em `docs/seguranca/2026-09-29-wave0-ratchet.md` |
| checker valida o campo | `validate_active_entries()`; ausência → reprova; `removed[]` isento |
| CI verde | **20 checks, 0 falhas**, incluindo `windows-full-suites` e `windows-gates-cp1252` |

🔴 **O D6 nasceu com dois baldes e a Wave 0 bloqueou o roadmap** exigindo o terceiro: entrada que
**sumiu sem passar** (skip, `panic`, rename) não é resolução. Sem ele, o gate mandaria retirar de
`entries[]` dívida que apenas deixou de ser medida.

🔴 **Achado extra, mesma causa, mesmo PR (Wave 3):** a Wave 2 aprovou o D6 por reimplementação — e na
auditoria pós-ML a linha de **resumo** rotulava o balde 2 como `resolvido`, duas linhas abaixo da
mensagem de erro que diz `Not a resolution`. `_cls_label` usava `known - obs`, a união dos baldes 1 e
2. Partido pelo conjunto de PASS em `resolvido` vs `ausente`. Colhido no runner Windows real:
`Go 1/2 [-1 ausente]`.

## Negative scope — o que esta REQ NÃO faz

- **Não** corrige nenhuma das 14 falhas. Esta REQ conserta o **mecanismo de colheita** e **documenta**
  as causas; corrigir é trabalho que a triagem vai **habilitar**, não substituir.
- **Não** adota remoção automática — rejeitada na **Emenda 1** da ADR: um teste instável sairia da
  lista numa corrida e voltaria a reprovar na seguinte, **em silêncio**.
- **Não** mexe no **D1** (bloqueio por conjunto de nomes) nem no **D5** (nomes instáveis como limite
  declarado). O D6 se **soma** a eles.
- **Não** trata o **#421** (fixture inconstruível em NTFS `noacl`) nem o **#403** (pesos de timing).
  Mecanismos distintos; o #421 pode **aparecer** na triagem do D7 como a razão de alguma entrada — e
  se aparecer, isso é **insumo** para ele, não absorção.

## Linked ADR
ADR: docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-nunca-colhe-a-melhoria.md
