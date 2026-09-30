---
status: Done
date: 2026-09-30
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md"
---

# REQ: gate de wave que reentra no barrier recursa sem limite e um roadmap vira fork bomb

> Date: 2026-09-30 | Status: Done
| Linear Issue: 
| Jira Issue: 
| GitHub Issue: #485

## Motivation

O `trackfw barrier` **executa** os comandos do bloco `**Gates da wave:**` via `sh -c`
(`internal/commands/barrier.go`, `runGateCommand`). Se o gate for o próprio `trackfw barrier` sobre
a mesma wave, cada instância invoca a seguinte: **não há detecção de reentrada nem limite de
profundidade**. Medido em 2026-09-30: **3469 processos em ~6 min**, cadeia de PPID estritamente
linear, load 12.9.

O gate ofensor era um comando **benigno e natural** — exatamente o que um autor de roadmap escreve
ao pensar "o gate desta wave é passar na barreira". Foi escrito por mim, num roadmap real, e havia
uma segunda mina igual armada na `main` desde 2026-09-22.

### 🔴 O que este defeito É e o que NÃO é

O #485 enquadra o problema como *"DoS local acionável por um arquivo de texto"*. **Esse
enquadramento não se sustenta como propriedade de segurança, e esta REQ não o promete:**

- o gate roda `sh -c` arbitrário **por desenho**. Um roadmap hostil não precisa de recursão:
  `:(){ :|:& };:` basta, e nenhum limite de profundidade o detém;
- o trust check (`origin/main`) não protege um clone hostil, porque o `origin/main` dele já contém
  o roadmap.

**O defeito real é de robustez:** um gate **benigno e bem-intencionado** se amplifica sem limite
até o teto de PIDs da máquina, e a contenção óbvia (`pkill -f 'bin/trackfw barrier'`) não funciona.
É isso que esta REQ corrige.

## Acceptance Criteria

- [x] **AC1** — gate que é `trackfw barrier` sobre a **mesma** wave do mesmo roadmap é recusado
  **antes de executar**, com mensagem que nomeia a reentrada, e o processo termina sem multiplicar
  ✅ Evidência: T1 (`TestBarrierReentry_T1_DirectReentrance`)
- [x] **AC2** — reentrada por **indireção** (script, `make`, `sh -c` aninhado) é contida — não
  multiplica, termina com erro nomeado
  ✅ Evidência: T2 (indireção via `reenter.sh`)
- [x] **AC3** — 🔴 **gate legítimo continua rodando**, inclusive gate que aninha `barrier` sobre
  **outro** roadmap (ex.: `go test ./internal/commands/...`, `scripts/check-barrier.sh`, `make quality`).
  O comportamento do acervo real é medido antes e depois
  ✅ Evidência: T3 + barrier real sobre a Wave 1 (roda `go test`, que aninha barrier) `passed`
- [x] **AC4** — `barrier` invocado duas vezes **em sequência** (não aninhado) continua funcionando
  ✅ Evidência: T4
- [x] **AC5** — todo outro caminho do produto que execute gates de roadmap (enumerado na Wave 0) tem
  a mesma contenção, ou a ausência de risco é demonstrada
  ✅ Evidência: enumeração da Wave 0: `barrier.go` `runGateCommand` é o único executor
- [x] **AC6** — a mina do acervo (`done/ROADMAP-2026-09-22-...:73`) está desarmada neste PR, e o
  acervo tem **0** gates que reentram na própria wave (medido com extração do primeiro bloco cercado
  após o marcador, não `grep` de janela)
  ✅ Evidência: gate da Wave 0: 0 no acervo; mina do ROADMAP-2026-09-22 desarmada
- [x] **AC7** — `docs/cli-parity.md` (§ `trackfw barrier`) descreve a contenção, o exit code e a
  mensagem; o código de saída segue a convenção existente (usage error ≠ `blocked`)
  ✅ Evidência: `docs/cli-parity.md` § Reentrance detection
- [x] **AC8** — `make quality` e CI verdes
  ✅ Evidência: CI do PR #486 20/20 `pass`; `make quality` 347 OK / 0 FAIL

## Negative scope

- ❌ **Sandbox, limite de recursos ou isolamento** para comandos de gate arbitrários. Gate é código
  executado com a confiança do usuário; um roadmap hostil segue podendo fazer qualquer coisa.
- ❌ O #476 (cerca não terminada no `FenceMask`) — branch própria, **bloqueada atrás desta** por
  compartilhar `barrier.go`.
- ❌ O defeito do gate de `trackfw commit` que impede commitar a transição `wip → blocked` a partir
  da branch que a produz — observado no #476, vira issue própria.
- ❌ Mudar o trust check (`origin/main` / `--trust-local-gates`).

## Linked ADR
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md
