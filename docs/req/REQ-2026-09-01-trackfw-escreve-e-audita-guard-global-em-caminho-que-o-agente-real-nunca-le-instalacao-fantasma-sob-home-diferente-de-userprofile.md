---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: `trackfw` escreve e audita guard global em caminho que o agente real nunca lê — instalação fantasma sob `$HOME` ≠ `%USERPROFILE%`

> Date: 2026-09-01 | Status: Open

## Motivation

Achado do `hades-tf` na Wave 0 da `REQ-2026-08-31` (`docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md`),
**deliberadamente mantido fora daquela REQ** porque corrigi-lo ali seria *feature* e não *port*.

Depois do port do `homedir`, o `UpdateHarness` (escritor) e o `validateGuardGlobalHookResolvable`
(auditor) usam **a mesma** `homedir.Dir()` — consistentes entre si. Mas o **CLI de agente real**
(Claude Code, Codex, Gemini) é **binário de terceiro** e continua resolvendo home pelo mecanismo
dele, tipicamente `%USERPROFILE%`-first.

Se `$HOME` e `%USERPROFILE%` divergirem — **plausível sob Git Bash, que é o ambiente do próprio
reporter do issue #216** — o `trackfw` escreve **e audita** um guard global saudável **num caminho
que o agente nunca lê**.

**Isso é pior que silêncio: é falso positivo de saúde.** O `credential-guard` e o `git-branch-guard`
são **controles de negação**. Um controle que se reporta saudável enquanto está inerte é a forma de
falha mais cara que existe — e é a mesma que apareceu cinco vezes durante a REQ anterior (o
`success()` implícito do Actions, o `VERDICT=ABSENT` por vacuidade, o `tail` engolindo o exit code,
a base `origin/main` num gate, e dois gates que nunca rodavam).

## Acceptance Criteria

- [ ] **AC1** — 🔴 **A medição certa não é comparar `$HOME` com `%USERPROFILE%` entre si.** É medir
      **onde o consumidor real lê o `settings.json`**, a partir de cada terminal (cmd.exe, Git Bash,
      PowerShell, CI). Comparar as duas env vars só diz que divergem; não diz qual delas o agente usa.
- [ ] **AC2** — Medição feita no runner Windows real — a sonda `windows-probe.yml` existe para
      exatamente este tipo de pergunta e já está na branch default.
- [ ] **AC3** — Se houver divergência, `trackfw validate` emite finding **informativo, não
      bloqueante**, nomeando os dois caminhos. Bloquear seria pior que o defeito: quebraria ambiente
      legítimo por uma condição que o `trackfw` não controla.
- [ ] **AC4** — 🔴 **Falsificação nas duas direções.** (a) com `$HOME` ≠ `%USERPROFILE%`, o finding
      dispara; (b) **controle**: com os dois iguais — o caso comum —, **não** dispara. Um aviso que
      aparece sempre é ruído, e ruído é ignorado.
- [ ] **AC5** — Paridade nos 3 CLIs.

## Negative Scope

- **Não** reverte nem altera o port do `homedir`. Ele está correto e foi aprovado na barreira.
- **Não** tenta adivinhar o mecanismo de cada agente de terceiro por leitura de documentação: **mede**.
- **Não** bloqueia operação por divergência.

## Linked ADR

ADR: <!-- avaliar durante a análise: se a conclusão for que o trackfw deve seguir o mecanismo
nativo do consumidor em vez do próprio, isso é decisão arquitetural e precisa de ADR. -->

## Linked Roadmap

Roadmap:
