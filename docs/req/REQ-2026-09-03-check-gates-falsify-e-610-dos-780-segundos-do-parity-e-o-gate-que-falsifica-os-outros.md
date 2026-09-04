---
status: Open
date: 2026-09-03
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `check-gates-falsify.sh` é 610 dos 780 segundos do `parity` — e é o gate que falsifica os outros

> Date: 2026-09-03 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Medido no runner do CI (`docs/qualidade/2026-09-03-medicao-do-alvo-parity.md`):

```
job parity                780 s
  check-gates-falsify.sh  610 s   <- 78,4%
  os outros 43 gates      170 s
```

**Um gate é 78% do caminho crítico do CI.** A `REQ-2026-09-02-job-parity-...` foi encerrada pelo AC6
porque a matriz de shards ganharia só ~19%: **o piso é este gate sozinho**, e nenhuma distribuição o
divide. Sensibilidade medida: ele teria de cair a ~40% do job para o sharding render ~7 min; está em
75,9%.

O KG pediu urgência — os PRs esperam por isso, inclusive os do reporter externo. **É aqui, e só
aqui, que os segundos estão.**

## 🔴 Por que este NÃO é um alvo de otimização comum

Este é **o gate que prova que os outros gates reprovam**. Hoje tem **359 cenários**, e cada um
compila um binário para sabotar. Ele é a razão pela qual a auditoria desta sprint pôde afirmar que
**4 gates estavam vácuos** — os 4 estavam **fora** dele.

**Acelerá-lo perdendo cenário é desligar a rede que protege todo o resto.** E o efeito seria
invisível: um cenário removido não falha, ele simplesmente deixa de existir.

Nesta mesma sprint, três vezes: um controle que passa sobre o defeito que existe para pegar. Este
gate é a defesa contra essa classe. **Cortar cenário para ganhar minuto é o pior negócio disponível.**

## Acceptance Criteria

- [ ] **AC1** — Perfil **por cenário**: quais dos 359 dominam, e **quanto de cada um é compilação de
      binário** contra execução do cenário. Sem isso, qualquer mudança é chute — 🔴 e nesta sprint eu
      já errei duas estimativas por não medir antes (73 desmascaradas → 29 reais; `parity` "não é o
      gargalo" → é 90% dele).
- [ ] **AC2** — 🔴 **Cobertura idêntica, verificada por conjunto.** O conjunto de cenários executados
      depois é **igual** ao de antes. Comparação de conjunto, não de contagem. **Um cenário que
      desaparece não falha — ele deixa de existir**, e é por isso que contagem não basta.
- [ ] **AC3** — 🔴 **Falsificação do próprio harness:** para uma amostra de cenários, sabotar o alvo
      e provar que **ainda reprovam** depois da mudança. Um harness acelerado que parou de detectar
      é pior que um harness lento.
- [ ] **AC4** — Ganho **medido no CI**, em run comparável — não estimado da soma local. A medição
      registrou que a máquina local é **1,77x mais lenta** que o runner, então soma local não
      transporta.
- [ ] **AC5** — 🔴 **"Não vale" continua sendo resultado válido.** Se o custo dominante for
      irredutível sem perder cobertura, encerrar com o número é a resposta certa — foi assim que a
      REQ do `parity` fechou.
- [ ] **AC6** — Se a saída for **isolar o gate em job próprio** (em vez de acelerá-lo), a armadilha
      do check obrigatório é pré-condição: `parity` é exigido **por nome**, com `enforce_admins`.
      🔴 Confirmado ao vivo: `python (3.10)`/`python (3.12)` na mesma lista são **prova** de que
      matriz renomeia o check. Exige job agregador chamado **exatamente** `parity`, com `needs:`,
      `if: always()`, tratando `skipped`/`cancelled` como **reprovação**.

## Negative Scope

- 🔴 **Não remover cenário.** Nem "consolidar cenários equivalentes" sem provar equivalência por
  execução. Reduzir cobertura não é otimizar.
- 🔴 **Não tirar o gate do `parity`** para o job ficar rápido. Isso move o problema e perde o portão.
- **Não** paralelizar por dentro sem medir flake: a medição mostrou que **auto-concorrência** pega o
  que concorrência cruzada não pega — dois gates reprovaram **5/5** e **ambos passaram** no teste
  cruzado. Num gate, intermitência é pior que lentidão.
- **Não** mexer nos outros 43 gates aqui: somados são 170 s, e o ganho não paga o risco.

## Linked ADR
<!-- Se a saída for isolar em job próprio, o formato do job vira convenção do repositório e pode
     merecer ADR curta. Decidir na Wave 0. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
