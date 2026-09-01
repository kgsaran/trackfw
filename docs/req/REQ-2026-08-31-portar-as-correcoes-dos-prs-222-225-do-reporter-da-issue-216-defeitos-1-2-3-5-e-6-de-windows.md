---
status: Done
date: 2026-08-31
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-31-portar-as-correcoes-do-reporter-da-issue-216.md"
---

# REQ: Portar as correções dos PRs #222–#225 do reporter da issue #216 — defeitos 1, 2, 3, 5 e 6 de Windows

> Date: 2026-08-31 | Status: Open

## Motivation

`lourivalgarciajunior` reportou o issue **#216** com 11 defeitos de Windows, e depois enviou **quatro
PRs com as correções**. KG fechou os quatro — não por qualidade, mas porque já estávamos num ciclo de
correção com governança própria e quatro PRs de terceiro nos mesmos arquivos garantiriam conflito.
O autor foi avisado antes.

**A análise técnica concluiu que os quatro valem inteiros** (`docs/analises/2026-08-31-aproveitamento-dos-prs-222-225.md`):
diagnóstico batendo com a linha de base **medida em runner Windows real**, correção **na origem** e
não força bruta, paridade respeitada, e testes que falsificam **nas duas direções**.

Esta REQ porta esse trabalho para dentro da nossa governança.

### Atribuição — requisito, não cortesia

**Todo commit portado carrega `Co-Authored-By: lourivalgarciajunior <lourival.garcia@gmail.com>`**
(e-mail obtido de `gh pr view 222 --json commits`, não presumido). Ele encontrou o defeito, escreveu
a correção e a correção é boa. O log tem de dizer isso.

### Portar fielmente — não "melhorar" em trânsito

Estes diffs foram **revisados como estão escritos**. Se um especialista "melhorar" uma correção
enquanto porta, perdemos a revisão que justificou aceitá-la inteira. **Porte fiel; se achar que algo
deveria mudar, pare e avise — não corrija em voo.**

### Mapeamento

| PR | item(ns) #216 | mecanismo |
|---|---|---|
| #222 Grupo A | 2 | `$HOME` ignorado nos 3 runtimes |
| #222 Grupo B | 3 | `info.Mode()&0111` sempre 0 em NTFS (no **validator**, distinto da `REQ-2026-08-28`, que cobriu só `scaffold_doctor.go`) |
| #223 | 1 | `→` (U+2192) na `description=` do parser raiz mata `print_help()` em console cp1252 |
| #224 | 6 | `isatty()` mente `True` para `NUL` e derruba o `init` |
| #225 | 5 | geradores Python escrevem CRLF e divergem dos outros dois runtimes |

## Acceptance Criteria

- [x] **AC1** — Os cinco defeitos (1, 2, 3, 5, 6) corrigidos, portados **fielmente** dos PRs
      #222–#225, com os testes que os acompanham.
- [x] **AC2** — 🔴 **Atribuição:** todo commit de port carrega
      `Co-Authored-By: lourivalgarciajunior <lourival.garcia@gmail.com>`.
- [ ] **AC3** — ❌ **FALSIFICADA, não reescrita.** Previa a contagem caindo para **3** na camada 2;
      o CI mediu **5**. O achado não é que duas correções falharam — é que **os checks dos itens 2 e
      3 medem a plataforma, não o produto** (`node -e "os.homedir()"`, `bit0111=0` em NTFS), e por
      isso são estruturalmente incapazes de ir a `ABSENT`. O item 3 chega a declarar isso no próprio
      comentário: *"confirmatório; primário = camada 1"*. **Eu peguei o número previsto na análise e
      o transformei em critério sem verificar o que aqueles dois checks medem.**
      A evidência de que as correções funcionaram está na camada 1 (**293→145 falhas, 1265→1422
      passes**), no `check-homedir-parity.sh` (item 2, no nível do produto) e na falsificação nas
      duas direções feita pelo `hades-tf` (item 3). **Não troco "3" por "5"** — reescrever o alvo
      para casar com o resultado é mover a trave. Retarget dos dois checks vira REQ própria, com o
      requisito de ser falsificada revertendo temporariamente a correção.
- [x] **AC4** — 🔴 **Cada item que sai de `REPRODUCED` é explicado no roadmap, citando o run que
      mediu a transição.** É a regra estabelecida ao fechar a `REQ-2026-08-30` (PR #228): contador
      decrescente **com histórico**, não alvo fixo.
- [x] **AC5** — Falsificação preservada nas duas direções. Em especial o controle: a operação
      legítima continua funcionando. Não basta o defeito sumir.
- [x] **AC6** — `docs/cli-parity.md` ganha os **dois contratos** que estas correções passam a impor e
      que hoje **não estão escritos em lugar nenhum**: *"os 3 runtimes escrevem artefato em LF"* e
      *"os 3 runtimes escrevem UTF-8 na saída, independente da codepage do console"*. O #225
      introduz um gate (`check-python-writes-lf.sh`) que **impõe** o primeiro — gate sem contrato
      escrito é exatamente o drift que aquele documento existe para impedir.
- [x] **AC7** — Wave 0 dedicada à **mudança de âncora de confiança** do #222 Grupo A:
      `validateGuardGlobalHookResolvable` passa de API nativa do SO para `$HOME` env-var-first. É
      caminho de segurança; `hades-tf` nomeou como merecedor de revisão dedicada.
- [x] **AC8** — `make quality` verde **e CI verde**. Verde local não é conclusão —
      `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.

## Negative Scope — o que esta REQ NÃO faz

- 🔴 **NÃO corrige o item 4**, e isto precisa ser dito alto porque o título fala em cp1252. O #223
  corrige o item 1 via `_force_utf8_output()` dentro de `main()`. **O item 4 é um `print()` cru em
  `scripts/check-parity-contract-coverage.sh`, que nunca entra em `main()`** — outro mecanismo, outro
  caminho, nenhum dos quatro PRs o toca. Quem ler "cp1252 corrigido" vai supor que o item 4 veio
  junto. Não veio.
  > Ironia funcional que vale registrar: o item 4 **é** o gate que audita a cobertura de
  > `docs/cli-parity.md`, morrendo em cp1252 ao ler esse mesmo arquivo. A AC6 acrescenta contrato a
  > um documento cujo gate de cobertura está, ele próprio, cego no Windows.
- **NÃO corrige os itens 7 e 10** — mecanismos não cobertos por nenhum dos quatro PRs.
- **NÃO toca os itens 8, 9 e 11** — fora de escopo declarado desde a Wave 0 da `REQ-2026-08-30`.
- **NÃO mexe na guarda de ancestral** (`REQ-2026-08-31-guarda-de-folha-...`, em `analyzing/`).
- **NÃO reabre** a `REQ-2026-08-30`, já `Done` com a linha de base 8/11 congelada.

## Nota sobre ordem — corrigindo o que eu mesmo disse

Eu havia afirmado que o **item 1 bloqueia a medição dos itens 5 e 6** e que por isso o #223 viria
primeiro. **Não constrange a ordem de port:** o instrumento **já neutraliza** o item 1 via
`PYTHONIOENCODING` justamente para conseguir medir 5 e 6 (ML-1C da `REQ-2026-08-30`). A única
restrição real de ordem é **#224 depois de #223**, porque o diff do #224 **contém** o do #223
(branch empilhada) — portar fora de ordem obriga a separar os conjuntos à mão.

## REQ fechada em 2026-09-01 — com a AC3 falsificada e registrada como tal

**Cinco defeitos de Windows corrigidos** (itens 1, 2, 3, 5 e 6 do issue #216), portados dos PRs
#222–#225 de `lourivalgarciajunior`, mergeados no **PR #229**. Ambas as barreiras aprovaram sem
bloqueante.

| medição | antes | depois |
|---|---|---|
| camada 2 (`windows-defect-reproduction`) | 8 `REPRODUCED` | **5** |
| camada 1 (`windows-full-suites`) | `293 failed, 1265 passed` | **`145 failed, 1422 passed`** |

**A AC3 previa 3 e o CI mediu 5. Fica FALSIFICADA, não reescrita** — os itens 2 e 3 medem a
plataforma (`os.homedir()` do Node, bit `0111` em NTFS) e não o produto, então são estruturalmente
incapazes de ir a `ABSENT`. Retarget em `REQ-2026-09-01-camada-2-mede-a-plataforma-e-nao-o-produto-...`.

**Por que não fecho como plenamente satisfeita e mesmo assim fecho:** o trabalho foi entregue,
revisado e mergeado; o que falhou foi **minha previsão sobre o instrumento**, não a correção. Manter
a REQ aberta por isso repetiria o erro que o `hefesto-tf` apontou na `REQ-2026-08-30` — um critério
que, mantido, torna impossível registrar progresso.

**Três REQs de acompanhamento abertas** com o que esta REQ deliberadamente não fez: retarget dos
checks 2 e 3; detecção de instalação fantasma (barrada da Wave 2 por ser *feature*, não *port*); e
teste unitário para o `tty.py`.

**Restam os itens 4, 7 e 10** para fechar o issue #216 por inteiro.

**Crédito:** `lourivalgarciajunior` reportou os 11 defeitos, escreveu as correções de cinco deles, e
as correções passaram por duas barreiras sem bloqueante. Quatro dos cinco commits carregam
`Co-Authored-By` dele.

## Linked ADR

ADR: <!-- nenhum. Ports de correções já revisadas, com tratamento por runtime justificado pelo
defeito ser single-runtime onde é o caso (#223, #224, #225 são Python-only porque o defeito é
Python-only — Go e Node já escrevem UTF-8 sem consultar codepage). Não há decisão arquitetural nova. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-31-portar-as-correcoes-do-reporter-da-issue-216.md`
