---
status: wip
date: 2026-08-21
req: "docs/req/REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md"
adr: ""
squad: "apolo-tf, hades-tf"
---

# Roadmap: `validate` detecta hook de guard na forma relativa antiga

> Created: 2026-08-21 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md`

Hook de guard escrito na **forma relativa antiga** funciona quando o comando roda na raiz do
repositório e **falha silenciosamente** fora dela. O `validate` não detecta — e o script está lá,
então nada acusa.

Última REQ de segurança antes da **7.2.0**.

## 🔴 O risco dominante é o falso-positivo, e ele decide o desenho

**Cursor e Copilot usam caminho relativo como forma correta**, por decisão registrada. Acusá-los
seria pior que a lacuna: quebra `validate` de quem está certo, e — pelo `ADR-2026-08-17` — guard que
atrapalha é guard que o usuário desliga.

A regra precisa distinguir **relativo que falha** de **relativo que é a forma certa daquele CLI**.
Essa distinção é o trabalho; o resto é mecânica.

## Riscos que valem para todos os MLs

1. **Não invadir a fronteira do `credential_guard_hook_resolvable`**, que já tem gate cross-CLI desde
   o ML-3A da REQ dos três contratos. Estender, não duplicar.
2. **Gate comparando as saídas reais** — teste por stack não fecha. Nove divergências reais nesta
   série.
3. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`.
4. Anotação `trackfw-contract` atualizada — o checker de cobertura é bloqueante.

---

## Wave 1 — Repro e regra

### ML-1A — Reproduzir a falha antes de escrever a regra
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Lote de investigação.** Entrega um parecer curto no roadmap; **nenhuma linha de regra**.

**Perguntas a responder com medição:**
- Qual é exatamente a "forma relativa antiga", e em quais dos 6 CLIs ela é **errada**?
- Em quais ela é **correta** (Cursor, Copilot — confirmar, não presumir)?
- O hook na forma antiga **falha mesmo** fora da raiz? Reproduza.
- Qual sinal distingue as duas formas de modo **decidível**? Se não houver sinal limpo, diga —
  é resposta legítima, e muda o desenho.

**Critérios de aceite:**
- [ ] As quatro respostas, com evidência medida
- [ ] Nenhuma linha de regra escrita

### ML-1B — Implementar a regra
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-1A
**Critérios de aceite:** ver AC1–AC4 da REQ. Em especial o **AC3** — Cursor e Copilot com relativo
**continuam limpos**.

---

## Wave 2 — Gate

### ML-2A — Gate de paridade + P4
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-1B
**Estender o gate existente** do `credential_guard_hook_resolvable`, não criar paralelo.

**Critérios de aceite:**
- [ ] Forma antiga acusada nos 3 CLIs; forma correta silenciosa nos 6
- [ ] **Cursor e Copilot com relativo silenciosos** — o discriminante de falso-positivo
- [ ] Cenário P4 com baseline e detecção
- [ ] `cli-parity.md` nomeia o gate; checker de cobertura exit 0
- [ ] `make quality` verde · CI-exata verde

---

## Wave 3 — Barreira

### ML-3A — `hades-tf`
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-21-revisao-da-deteccao-de-hook-relativo.md`

A regra decide se um guard está ativo. Avaliar se a detecção pode ser **contornada** por uma forma
que ela não reconhece, e se o falso-positivo em Cursor/Copilot foi de fato evitado. **Veredito
explícito.**

---

## Notas
- **Fora de escopo:** mudar a decisão de qual forma cada CLI usa. A regra **detecta**, não redefine.
- Commits e branch são exclusivos do `trackfw_architect`.
