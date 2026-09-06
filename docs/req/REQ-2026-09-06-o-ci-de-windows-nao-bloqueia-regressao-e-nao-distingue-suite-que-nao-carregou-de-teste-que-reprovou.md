---
status: Open
date: 2026-09-06
author: ""
adr: ""
roadmap: ""
---

# REQ: o CI de Windows nao bloqueia regressao e nao distingue suite que nao carregou de teste que reprovou

> Date: 2026-09-06 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Governada por
`docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md`
(`Accepted`). Fecha os issues **#275** e **#274**, que o consumidor externo abriu com medição própria.

### O gate de Windows não bloqueia nada

`windows-full-suites` roda com `continue-on-error: true`. **Enquanto isso valer, nenhuma regressão de
Windows reprova um PR** — a dívida é grande demais para exigir zero, então nada é exigido.

### E a contagem esconde regressão. Medido três vezes

Pelo consumidor externo, comparando conjuntos **por nome** em Windows 11 real:

```
#269 + #270    51 → 33    corrigidas 20 · NOVAS 0
#271           33 → 32    corrigidas  2 · NOVAS 1   ←
#272           32 → 12    corrigidas 20 · NOVAS 0
```

*"Uma queda de 1 pode conter 2 correções e 1 regressão."*

🔴 **E aconteceu de novo conosco, depois disso.** O PR #285 baixou a contagem geral e **introduziu 6
falhas de Python**; só apareceram porque o arquiteto **diffou os nomes** entre a `main` e a branch. A
proteção dependeu de alguém lembrar — o CI não obrigava.

### O instrumento da contagem também já falhou

O arquiteto reportou **69 onde havia 101**: o padrão de `grep` não casava o prefixo por linha do
`gh run view --log`, e saiu com exit 0 e número plausível.
`vault/notes/contagem-de-falhas-de-windows-do-go-medida-por-padrao-frouxo-2026-09-04.md`.

**Um ratchet por nome torna essa fragilidade irrelevante:** ele compara **conjuntos**, não números.

### E o discriminante que propusemos para o #274 não discriminava

Medido:

```
teste que roda e reprova   → # tests 1 · pass 0 · fail 1
import quebrado            → # tests 1 · pass 0 · fail 1
```

Comentamos publicamente que `pass 0 / fail 1` distingue "a suíte não carregou". **Não distingue** —
correção já publicada na própria issue.

## Acceptance Criteria

- [ ] **AC1** — Lista versionada de vermelhos conhecidos, **por nome de teste**. O job **reprova** se
      aparecer nome fora da lista.
- [ ] **AC2** — Nome da lista que **deixa de falhar** gera **aviso** pedindo remoção — não reprova.
- [ ] **AC3** — 🔴 **A lista nasce de um run do CI, nunca de uma máquina.** O autor do `#275` declara
      que o Windows dele **não é o runner**; a medição dele sustenta a **necessidade**, não o
      conteúdo.
- [ ] **AC4** — 🔴 **Suíte que não executa é falha de CLASSE PRÓPRIA.** Um estado **sem nomes** não é
      coberto por ratchet por nome — e é o pior modo de falha, porque a contagem **cai** e a queda
      parece progresso.
- [ ] **AC5** — 🔴 **O discriminante do AC4 é medido nos dois cenários ANTES de ser escrito.** Foi
      pular esse passo que produziu a nossa afirmação errada no `#274`.
- [ ] **AC6** — 🔴 **Remoção de nome exige justificativa explícita**: corrigido, **renomeado** num
      refactor, ou **deixou de executar**. O ratchet não distingue os três sozinho — e sem isso a
      lista vira cemitério, que é a única forma de ele fracassar em silêncio.
- [ ] **AC7** — 🔴 **Guarda de não-vacuidade:** com a lista vazia e a dívida atual, o job **tem** de
      reprovar. Ratchet que passa sobre qualquer entrada não é ratchet.
- [ ] **AC8** — **Falsificação nas duas direções:** teste novo que falha só em Windows, sem tocar a
      lista → reprova **nomeando o teste**; corrigir um da lista → **avisa**; removê-lo → verde.
- [ ] **AC9** — O caso do **rename** exercitado: renomear um teste da lista **sem corrigi-lo** não
      pode virar verde silencioso.
- [ ] **AC10** — O `continue-on-error: true` do `windows-full-suites` **sai** — 🔴 **só depois** de
      AC1, AC4, AC6 e AC7 estarem de pé. Removê-lo antes tornaria a `main` imergível.

## Negative Scope

- ❌ **Não** exigir que a dívida chegue a zero. O ponto do ratchet é **bloquear regressão sem zerar
  primeiro**.
- ❌ **Não** aplicar a outros jobs além dos de Windows nesta REQ.
- ❌ **Não** resolver os nomes instáveis de teste parametrizado inventando convenção nova — **declarar**
  o tratamento (D5 da ADR), e deixar escrito.
- ❌ **Não** corrigir nenhuma das 39 falhas restantes aqui. Esta REQ constrói o **instrumento**; as
  falhas têm caminho próprio.

## Linked ADR
ADR: docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-09-06-ratchet-por-nome-e-classe-propria-para-suite-que-nao-carrega.md
