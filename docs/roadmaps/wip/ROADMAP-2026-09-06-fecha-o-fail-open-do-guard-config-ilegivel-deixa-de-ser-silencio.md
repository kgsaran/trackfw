---
status: wip
date: 2026-09-06
squad: apolo-tf
req: "docs/req/REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-integridade-do-script-e-da-config-controle-positivo-e-fail-closed-nativo.md"
---

# Roadmap: Fecha o fail-open do guard — config ilegível deixa de ser silêncio

> Criado em: 2026-09-06 | Status: wip

## Context

REQ (**reaberta**): `docs/req/REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-integridade-do-script-e-da-config-controle-positivo-e-fail-closed-nativo.md`

## Diagnóstico

A REQ estava `Done` com **os 4 critérios em branco**. Dois foram entregues de fato (controle positivo
e integridade de conteúdo); **dois não** — justamente os que cobrem **deleção** e **"não consegui
rodar"** no momento da invocação.

E o ML-6C mediu uma **quinta via**: JSON inválido em config de guard é engolido por `continue` mudo.
**O controle reporta saúde sobre o que não conseguiu ler.**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — A via medida, sozinha
> **WIP = 1.** Nada novo entra antes desta fechar.

### ML-1A — Config ilegível deixa de ser silêncio
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` + barreira `hades-tf`
**Sítios conhecidos:** `internal/validator/validator_git_branch_guard.go:151-154` e a função irmã do
credential-guard. 🔴 **Enumerar os demais antes de corrigir** — o ML-6C achou dois **por acaso**,
olhando outra coisa.

**Decisão em aberto, a tomar com medição:** acusar como violation, ou emitir diagnóstico próprio.
🔴 O `continue` mudo deixa de ser aceitável, **mas a escolha entre as duas tem consequência**: acusar
demais faz o usuário desligar a regra, e aí o controle vale zero. Escrever a razão da escolha.

**Critérios:** falsificação nas duas direções (config corrompido acusa; válido não acusa) ·
enumeração completa das leituras de config de guard, com veredito por sítio · paridade nos 3 CLIs ·
`make quality` e `validate` verdes.

## Fora desta wave, e declarado
Os **ACs 2 e 3** da REQ (o `failClosed` do Cursor e o wrapper) continuam não entregues. **Não entram
aqui** — o AC3 tem bloqueador declarado e não resolvido (o script é gerado, não vem no binário; um
clone fresco com hooks commitados antes do `init` teria toda chamada travada). Entram na wave
seguinte **da mesma REQ**, depois que esta fechar.
