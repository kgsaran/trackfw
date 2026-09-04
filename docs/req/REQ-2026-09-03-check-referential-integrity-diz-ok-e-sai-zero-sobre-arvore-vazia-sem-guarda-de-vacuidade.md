---
status: Open
date: 2026-09-03
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `check-referential-integrity.sh` diz `OK` e sai 0 sobre árvore vazia — quinto gate vácuo, e está dentro do `parity`

> Date: 2026-09-03 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Suspeitado pelo `ares-tf` ao cronometrar os 46 gates do `parity` — **0,04 s**, o único gate rápido
sem contador nem guarda de vacuidade. **Verificado pelo arquiteto, por execução:**

```
arvore com docs/ vazio:
  Referential integrity OK
  rc=0
```

O corpo é `for req in docs/req/*.md; do [[ -f "$req" ]] || continue`. **Se o glob não casar, o gate
imprime `OK` e sai 0 sem checar nada.**

### É o quinto, e a diferença importa

A auditoria de backlog desta sprint mediu **4 gates vácuos** —
`check-shell-posix-portability.sh`, `check-atomic-write-anti-divergence.sh`,
`check-tty-detection.sh` e os checks da camada 2. Os quatro estavam **fora** do
`check-gates-falsify.sh`, e essa foi a causa raiz identificada: *a cobertura de falsificação não
acompanha os gates novos*.

**Este é o quinto, e está dentro do alvo `parity`** — roda em todo PR, e reporta saúde sem medir.

🔴 **O modo de falha é o pior possível:** não é "o gate não pega o defeito X". É **"o gate reporta
verde quando não olhou para nada"** — mesma classe do contrato de paridade que documentava cobertura
inexistente, e do índice de vault que dizia `NÃO CORRIGIDO` sobre algo corrigido.

### Como isto se torna real, e não teórico

O gate resolve `docs/req/` **relativo ao cwd**. Basta um projeto com `req_dir` configurado diferente,
uma execução de outro diretório, ou uma renomeação de pasta — e ele passa a aprovar tudo, para
sempre, em silêncio.

## Acceptance Criteria

- [ ] **AC1** — Guarda de vacuidade: varredura que enumere **zero** itens **falha** ou reporta
      `not_evaluated`. **Nunca** `OK` com exit 0.
- [ ] **AC2** — 🔴 **Falsificação nas duas direções:** árvore vazia → **reprova**; árvore íntegra →
      passa. Um gate que passa nos dois casos não mede nada.
- [ ] **AC3** — O gate emite **contagem** do que verificou. Os demais gates ≤0,11 s têm guarda ou
      emitem contagem — este é o único que não tem nenhuma das duas.
- [ ] **AC4** — 🔴 **Cenário em `scripts/check-gates-falsify.sh`.** Esta é a correção da **classe**,
      não da instância: os 4 vácuos anteriores estavam fora daquele harness, e é por isso que
      sobreviveram ao `make quality` verde. Sem o cenário, o sexto nasce vácuo.
- [ ] **AC5** — O gate resolve `req_dir`/`adr_dirs`/`roadmap_dir` do `trackfw.yaml` em vez de
      hardcodar `docs/req/` — **ou** declara explicitamente que só serve para este repositório.
      Decidir e escrever.
- [ ] **AC6** — `make quality` verde e `trackfw validate` exit 0.

## Negative Scope

- **Não** varrer os outros 45 gates atrás de vacuidade nesta REQ. O `ares-tf` mediu que os demais
  ≤0,11 s **têm** guarda ou contagem, e não procurou fora do `parity` — **os 3 vácuos restantes da
  auditoria anterior seguem em REQ própria.**
- **Não** acelerar nem mexer no `check-gates-falsify.sh` aqui — é REQ própria (610 dos 780 s).
- **Não** relaxar o que o gate verifica para "compensar" a guarda nova.

## Linked ADR
<!-- Correção de gate; sem decisão de arquitetura. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
