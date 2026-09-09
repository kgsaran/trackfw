---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-09-09-gates-de-paridade-provam-concordancia-mas-nao-completude.md"
---

# REQ: Gate anti-divergência prova que as cópias concordam, mas não que a lista está completa

> Date: 2026-09-01 | Status: Open

## Motivation

Achado do `hades-tf` na barreira final da `REQ-2026-09-01-os-fchmod-...`.

O `scripts/check-atomic-write-anti-divergence.sh` compara três cópias de `_atomic_write` e reprova se
divergirem. Ele faz isso bem — falsificado em quatro direções. **Mas a lista de arquivos é fixa:**

```bash
FILES=(pypi/trackfw/identity/__init__.py
       pypi/trackfw/thirdparty/quarantine.py
       pypi/trackfw/integrations/manager.py)
```

**Uma quarta cópia futura de `_atomic_write` passaria silenciosamente para sempre.** O gate provaria,
com toda a confiança, que as três que ele conhece concordam — enquanto a quarta, invisível, faz o que
quiser.

### É a mesma classe que atravessou esta sessão inteira, um nível acima

Os casos anteriores eram **controles inertes**: gate desligado, `VERDICT=ABSENT` por vacuidade,
`success()` implícito, `assert_has` passando com 1 de 2 ocorrências. Este é diferente e mais sutil:
**o controle roda, mede corretamente, e é completo sobre o que conhece — mas o que ele conhece está
congelado.**

Precedente no vault: `global-guard-dedup-and-hook-resolvable-never-validate-hook-structure-2026-08-18`.

### Por que importa mais do que parece

O veredito de **não extrair** a triplicação (ML-0A) foi tomado **assumindo que o gate garante a
não-divergência**. Se o gate não enxerga cópias novas, a premissa daquela decisão enfraquece com o
tempo — e ninguém é avisado.

## Acceptance Criteria

- [ ] **AC1** — O gate **descobre** as cópias em vez de recebê-las por lista, ou **afirma a contagem
      esperada** e reprova se a varredura encontrar um número diferente.
- [ ] **AC2** — 🔴 **Falsificação:** acrescentar uma quarta cópia de `_atomic_write` numa fixture faz
      o gate **reprovar**, nomeando o arquivo novo. Sem esta prova, a correção é decorativa.
- [ ] **AC3** — **Controle:** renomear ou mover uma das três existentes **também** reprova — não
      basta detectar adição.
- [ ] **AC4** — 🔴 **A mesma pergunta aplicada aos outros gates de lista fixa.** Este defeito não é do
      `check-atomic-write-anti-divergence`; é da **forma**. Enumerar quais gates de `scripts/` operam
      sobre lista fixa de arquivos e dizer, para cada um, se a lista congelada é aceitável ou é o
      mesmo buraco. **Esta AC vale mais que as três primeiras.**
- [ ] **AC5** — Contrato em `docs/cli-parity.md`: um gate que opera sobre conjunto de arquivos
      **declara se o conjunto é descoberto ou fixo**, e no segundo caso justifica.
- [ ] **AC6** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** reabrir o veredito de não extrair a triplicação — esta REQ **fortalece** a premissa dele.
- **Não** reescrever a lógica de comparação, que está correta e falsificada.

## Linked ADR

ADR: <!-- avaliar na análise: se a AC4 mostrar que a lista fixa é padrão disseminado, a decisão sobre
descoberta-vs-lista vira postura de projeto e merece ADR. -->

## Linked Roadmap

Roadmap:


## 🔴 Sítio de mesma causa — issue #298, 2026-09-09

`scripts/check-cli-parity.sh` **compara só comandos de primeiro nível**. Subcomando divergente passa
em silêncio: **9** (`adr` 2, `req` 3, `roadmap` 4) sem gate entre os 3 runtimes.

**É a mesma causa desta REQ, com outro sujeito:** aqui é `_atomic_write` com lista fixa de arquivos;
lá é `check-cli-parity` com escopo fixo no primeiro nível. Nos dois, o gate **prova concordância do
que conhece e nada sobre completude**.

Por isso entra como ML nesta REQ — **não como REQ nova**. Regra dura do `CLAUDE.md`.

🔴 **E já nos mordeu:** `req move` faltou nos três e `req list` faltou no Python, sem nenhum gate
avisar. Não é hipótese.

**Nota de medição:** o autor do issue reportou 12 subcomandos; medi **9**. A diferença provável é o
`help` que o cobra injeta em cada grupo — e excluí-lo é requisito do ML, não detalhe: contar o `help`
faria o gate exigir paridade de algo que o framework gera.
