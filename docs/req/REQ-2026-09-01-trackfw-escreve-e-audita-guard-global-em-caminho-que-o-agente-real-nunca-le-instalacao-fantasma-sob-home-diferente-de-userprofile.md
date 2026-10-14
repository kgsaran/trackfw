---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-14-no-windows-o-guard-global-de-cada-cli-de-agente-e-gravado-e-auditado-no-home-que-aquele-cli-le.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-22-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-userprofile.md"
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

## Medição — 2026-10-14

Confirmado na VM (Claude Code 2.1.299, três terminais): com `HOME` ≠ `USERPROFILE` o trackfw grava e
audita sob `HOME`, o Claude Code lê `USERPROFILE`, e o `validate` não acusa nada
(`docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md`). **Decisão do usuário:** seguir
o home que cada CLI lê, mais o aviso (ADR-2026-10-14). Os critérios abaixo substituem os originais;
AC5 original (paridade nos 3 CLIs) caiu com a v8.

## Acceptance Criteria

- [x] **AC0** — 🔴 **Wave 0 (`hades-tf`):** o mapa CLI → mecanismo de home no Windows, com evidência
      por CLI (Claude Code medido; Codex, Gemini, Cursor, Copilot, Windsurf, Kiro). CLI sem evidência fica
      declarado e não muda de comportamento. Inclui a lista de testes que isolam só o `HOME`.
      ✅ Evidência: `docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md` (aprova com ressalvas; A1–A8 no ADR D6)
- [x] **AC1** — Medir onde o consumidor real lê, por terminal.
      ✅ Evidência: medição de 2026-10-14 (Claude Code; Git Bash, PowerShell, cmd).
- [x] **AC2** — Número do CI: o `windows-probe.yml` (runner x64) reproduz a divergência antes da correção
      e mostra que ela fechou depois.
      ✅ Evidência: run `38618567001` (x64): `main` grava sob `HOME`, branch sob `USERPROFILE`
- [x] **AC3** — `update harness` grava, e o `validate` audita, a fiação global de cada CLI medido no home
      que **aquele CLI** lê, por um resolvedor único por CLI usado pelos dois (ADR D1–D3).
      ✅ Evidência: `internal/agentconfig`; `TestGuardGlobalConcordance_*`; `Manager.ClaudeDir` (ML-1A, ML-1D)
- [x] **AC4** — Aviso informativo, não bloqueante, quando `HOME` ≠ home do SO no Windows, nomeando os dois
      caminhos, qual cada CLI lê e o resto inerte sob `HOME` (ADR D4–D5). Com os dois iguais: silêncio.
      ✅ Evidência: regra `home_divergence_windows`; `withD5Notice` (ML-1B)
- [x] **AC5** — 🔴 Falsificação nas duas direções: (a) com divergência, o guard é gravado e auditado no
      home do CLI, e o aviso dispara; (b) controle: sem divergência e fora do Windows, a saída é idêntica
      à de hoje. O teste reprova sem a correção **sem depender de rodar no Windows** (resolvedor
      injetável).
      ✅ Evidência: mordidas do ML-1A, ML-1B e ML-1D, todas no macOS
- [x] **AC6** — `docs/cli-parity.md` e a seção Windows do README atualizados.
      ✅ Evidência: `docs/cli-parity.md` e README (ML-1C)
- [x] **AC7** — Cada teste novo declara a conclusão que afirma e reprova sem a correção.
      ✅ Evidência: frase + mordida por teste em cada ML
- [ ] **AC8** — `make quality` EXIT=0 e CI verde, inclusive `windows-full-suites` (sem entrada nova na
      lista conhecida).

## Negative Scope

- **Não** reverte o port do `homedir`: `homedir.Dir()` continua valendo para os artefatos do trackfw.
- **Não** muda o comportamento fora do Windows nem para CLI sem evidência medida — **exceto** `CLAUDE_CONFIG_DIR`, honrado em qualquer SO por ser a mesma causa (ADR D6/A1).
- **Windsurf:** resíduo declarado até ser medido (ADR D6/A8).
- **Não** apaga o guard já gravado sob o `HOME` divergente — relata (ADR D5).
- **Não** bloqueia por divergência.

## Linked ADR
ADR: docs/adr/ADR-2026-10-14-no-windows-o-guard-global-de-cada-cli-de-agente-e-gravado-e-auditado-no-home-que-aquele-cli-le.md

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-userprofile.md`


---

## 🔴 Achado da auditoria externa (2026-09-05) — esta REQ se contradiz

Registro: `docs/portabilidade/2026-09-05-auditoria-externa-astra-achados-e-verificacao.md`, item 11.

**A `AC1` e a `AC4` exigem coisas diferentes, e a REQ trata as duas como se fossem a mesma:**

| | condição |
|---|---|
| AC1 | **medir o consumidor real** — o agente lê aquele caminho? |
| AC4 | **avisar pela simples divergência** entre `$HOME` e `%USERPROFILE%` |

Divergência de variáveis **não implica** que o agente não leia o arquivo, e ausência de divergência
**não garante** que leia. São predicados distintos, e um aviso emitido pelo segundo critério vai
disparar em ambiente correto e calar em ambiente quebrado.

🔴 **E há um limite que a REQ não declara:** avisar que os caminhos divergem **não faz o hook
funcionar**. A mitigação por aviso é aceitável como primeiro passo, mas precisa estar escrito que ela
é **mitigação, não correção** — senão o ML que a implementar será marcado como concluído tendo
entregue um aviso onde o objetivo era um guard que executa.

**Reconciliação exigida antes de qualquer implementação:** decidir qual dos dois predicados governa a
regra, e o que o outro vira — evidência auxiliar, diagnóstico, ou nada. Enquanto isso não estiver
escrito, **esta REQ não é implementável**: um agente honesto entregaria AC1 e AC4 em conflito.

### Reconciliação — 2026-10-14

Resolvida pelo ADR-2026-10-14: **governa o predicado da AC1 original** (o que o CLI lê de fato, por
um mapa medido por CLI). A divergência `HOME` × home do SO vira **diagnóstico auxiliar** (aviso
informativo). A correção é gravar e auditar no home do CLI (AC3) — o aviso **não** conta como
correção.
