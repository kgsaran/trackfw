---
status: backlog
date: 2026-09-06
squad: ares-tf
req: "docs/req/REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-e-nao-distingue-suite-que-nao-carregou-de-teste-que-reprovou.md"
---

# Roadmap: Ratchet por nome, e classe própria para suíte que não carrega

> Criado em: 2026-09-06 | Status: backlog

## Context

REQ: `docs/req/REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-e-nao-distingue-suite-que-nao-carregou-de-teste-que-reprovou.md`
ADR: `docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md` (`Accepted`)
Fecha: **#275** e **#274**

## Diagnóstico

`windows-full-suites` roda com `continue-on-error: true` — **nenhuma regressão de Windows reprova um
PR**. E a contagem esconde regressão: medido 3x pelo consumidor externo, e **uma 4ª vez conosco**, no
PR #285, que baixou o total e introduziu 6 falhas.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — O discriminante, antes do ratchet
> Sequencial. A D3 da ADR exige medir antes de escrever.

### ML-1A — Distinguir "suíte não carregou" de "teste reprovou"
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
🔴 **Medir o discriminante nos DOIS cenários antes de escrevê-lo.** Foi pular esse passo que produziu
a nossa afirmação pública errada no `#274` — `pass 0 / fail 1` é idêntico nos dois casos.
Cobrir também `tests == 0`. **Falha de classe própria**, não linha na lista de nomes.

## Wave 2 — O ratchet
> Dependências: Wave 1. Sem o discriminante, um estado sem nomes escapa do ratchet por construção.

### ML-2A — Lista versionada de vermelhos, por nome
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
🔴 **A lista nasce de um run do CI**, nunca de máquina — o autor do `#275` declara que o Windows dele
não é o runner. Reprova nome fora da lista; **avisa** quando um nome da lista deixa de falhar.
🔴 **Guarda de não-vacuidade:** com a lista vazia e a dívida atual, o job **tem** de reprovar.

### ML-2B — Remoção de nome exige justificativa
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
Corrigido, **renomeado** ou **deixou de executar** — o ratchet não distingue sozinho. Sem isto a
lista vira cemitério, que é a única forma de ele fracassar em silêncio.
**Falsificação obrigatória:** renomear um teste da lista **sem corrigi-lo** não pode virar verde.

## Wave 3 — Tirar a rede
> Dependências: Waves 1 e 2 fechadas e verdes.

### ML-3A — Remover `continue-on-error` do `windows-full-suites`
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
🔴 **Só aqui.** Remover antes tornaria a `main` imergível com a dívida atual — o ponto do ratchet é
bloquear regressão **sem** exigir zero primeiro.
