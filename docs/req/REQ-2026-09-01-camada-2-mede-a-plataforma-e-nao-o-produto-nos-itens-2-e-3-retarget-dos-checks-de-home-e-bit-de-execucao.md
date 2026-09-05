---
status: Done
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-05-retarget-dos-checks-de-camada-2-que-medem-a-plataforma-e-nao-o-produto.md"
---

# REQ: Camada 2 mede a plataforma e não o produto nos itens 2, 3, 4 e 7 — retarget dos checks

> Date: 2026-09-01 | Status: Done

## Motivation

Ao portar as correções do reporter do issue #216 (`REQ-2026-08-31-portar-as-correcoes-...`), a AC
previa a camada 2 caindo de **8** para **3** `REPRODUCED`. O CI mediu **5**, e o diagnóstico é que
**dois checks medem o sistema operacional, não o `trackfw`**:

```powershell
# item 2 — scripts/windows-repro/run.ps1:112
node   -e "console.log(require('os').homedir())"
python -c "import os; print(os.path.expanduser('~'))"

# item 3 — run.ps1:134
if ($r3.Stdout -match "bit0111=0") { "REPRODUCED" }
```

O `os.homedir()` do Node vai ignorar `$HOME` no Windows para sempre, e o bit `0111` vai ser 0 em
NTFS para sempre. As correções **desviam o `trackfw`** dessas propriedades — via `homedir.Dir()` e
via guarda de plataforma no validator — mas **não mudam a plataforma**. Logo os dois checks são
**estruturalmente incapazes** de ir a `ABSENT`, e não servem como regressão do que corrigimos.

O item 3 chega a declarar isso no próprio comentário: *"confirmatório; primário = camada 1"*. Estava
escrito antes de a AC ser fixada.

**As correções funcionaram** — a evidência está em outro lugar:

| medição | resultado |
|---|---|
| camada 1 | `293 failed, 1265 passed` → **`145 failed, 1422 passed`** |
| item 2, nível de produto | `scripts/check-homedir-parity.sh` passa em `parity` no CI |
| item 3, nível de produto | `hades-tf` reverteu o guard e confirmou que o teste *"não dispara no Windows"* quebra sem ele |

## Terceira instância — o item 7 (acrescentado em 2026-09-01)

O mesmo padrão apareceu no **item 7**, medido no PR #236: a correção foi verificada por execução dos
3 binários reais, e **a camada 2 não a enxergou** (contagem ficou em 4, esperava 3).

```go
// scripts/windows-repro/go/checks.go:135 — cmdGateQuote
c := exec.Command("sh", "-c", gateQuoteCommand)   // réplica, não o barrier
```

O check roda **réplicas dentro do harness** (`checks.go`/`checks.js`/`checks.py`), cada uma com sua
própria invocação de shell. A correção mudou `barrier.js`/`barrier.py`.

**Com três instâncias, isto deixa de ser acidente e vira propriedade do instrumento:** os checks
2, 3 e 7 medem **substitutos** — a plataforma, ou uma réplica — em vez do produto.

**Consequência para as ACs:** o retarget passa a cobrir **três** itens. E a AC3 (falsificar
revertendo a correção) vale igualmente para o 7: retargetado, ele iria a `ABSENT` imediatamente,
porque a correção já está aplicada.

**Por que não abri REQ nova:** é a mesma causa raiz. Fragmentar em três REQs esconderia que o
instrumento tem um **padrão**, e cada uma pareceria um acidente isolado.

## Quarta instância — o item 4 (acrescentado em 2026-09-05)

O parecer de fechamento do issue #216
(`docs/portabilidade/2026-09-05-parecer-fechamento-issue-216.md`) mediu uma **quarta** instância do
mesmo padrão, por caminho independente:

> item 4 — CORRIGIDO em 2026-09-02, **mas a sonda de Windows real nunca foi atualizada e continua
> testando o mecanismo pré-fix**.

O check invoca o **mecanismo cru** (o helper Python lendo o markdown) em vez do `.sh` real que a
correção mudou. É a mesma forma dos itens 2, 3 e 7: **o instrumento mede um substituto.**

🔴 **Isto contradiz o Negative Scope original desta REQ**, que dizia "não toca os itens 4, 7 e 10" —
escrito antes de o item 7 virar a terceira instância (no próprio corpo desta REQ, em 2026-09-01) e
antes de o item 4 ser medido como a quarta. **O escopo negativo abaixo está corrigido:** só o item
10 fica de fora, por seguir genuinamente sem correção.

**Com quatro instâncias, a conclusão endurece:** não é um check mal escrito, é uma **propriedade do
instrumento**. Um harness de reprodução que replica o mecanismo em vez de invocar o produto mede a
si mesmo — e envelhece em silêncio a cada correção que o produto recebe.

**Consequência prática, e é a que justifica prioridade:** hoje o dashboard de Windows mostra
defeitos **corrigidos** como `REPRODUCED`. Quem abrir o painel — inclusive o contribuidor externo
que abriu o #216 — chega à conclusão errada. **Não é possível fechar o #216 com honestidade
enquanto isto não for corrigido**, mesmo com 6 dos 7 itens resolvidos no produto.

## Acceptance Criteria

- [ ] **AC1** — O check do item 2 mede o **`trackfw`** resolvendo home com `HOME` ≠ `USERPROFILE`,
      não `os.homedir()` do runtime.
- [ ] **AC2** — O check do item 3 mede o **validator** deixando de alarmar em Windows, não o bit em
      NTFS. Ou, se a conclusão for que o item 3 **deve** permanecer confirmatório, isso é declarado
      explicitamente e ele sai da contagem de `REPRODUCED` corrigíveis.
- [ ] **AC3** — 🔴 **Falsificação obrigatória revertendo a correção.** Retargetados, os dois checks
      vão a `ABSENT` **no instante em que forem escritos**, porque a correção já está aplicada. **Um
      check que passa ao nascer não prova nada.** Só valem se, **revertendo temporariamente**
      `homedir.Dir()` e a guarda de plataforma, eles voltarem a `REPRODUCED`. Sem essa prova, o
      retarget é decorativo.
- [ ] **AC4** — A contagem esperada da camada 2 é **recalculada e justificada item a item**, não
      herdada de previsão. O erro original foi transformar um número previsto em critério sem
      verificar o que os checks mediam.
- [ ] **AC6** — 🔴 **O check do item 4 invoca o `.sh` real**, não o mecanismo replicado. Falsificação:
      revertendo o fix de 2026-09-02, ele volta a `REPRODUCED`.
- [ ] **AC7** — 🔴 **Guarda contra a próxima instância.** Enumerar **todos** os checks das camadas do
      harness (`scripts/windows-repro/`) e classificar cada um: **invoca o produto** ou **replica o
      mecanismo / mede a plataforma**. Quatro instâncias apareceram uma a uma, por acidente; a quinta
      não deve depender de sorte. Um check que replica o mecanismo e não está declarado como
      confirmatório é defeito.
- [ ] **AC5** — A `AC3` da `REQ-2026-08-31` continua marcada como **FALSIFICADA**. Esta REQ **não**
      a reescreve — corrige o instrumento, não o histórico.

## Negative Scope

- **Não** altera nenhuma correção de produto. As cinco portadas ficam como estão.
- **Não** re-baselina a camada 2 junto com outra mudança: retarget de medição e correção de defeito
  no mesmo diff tornam impossível atribuir causa a uma mudança de contagem.
- **Não** toca o item **10**, que segue genuinamente sem correção.
- 🔴 **Corrigido em 2026-09-05:** a linha original excluía os itens **4 e 7**. Ambos foram depois
  medidos como instâncias do mesmo padrão (o 7 nesta própria REQ, o 4 no parecer do #216) e **entram
  no escopo**. Manter a exclusão faria a REQ corrigir três quartos de uma causa raiz.

## Linked ADR

ADR: <!-- nenhum. Correção de instrumento de medição; nenhuma decisão arquitetural nova. -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-09-05-retarget-dos-checks-de-camada-2-que-medem-a-plataforma-e-nao-o-produto.md


---

## Encerramento — 2026-09-05

Entregue nos PRs #280 e #281. Os 4 checks passam a invocar o produto; o item 3 saiu do contador de forma estrutural; os dois vazamentos de sinal fechados. Resultado no CI: Reproduzidos 0, Inconclusivos 0.
