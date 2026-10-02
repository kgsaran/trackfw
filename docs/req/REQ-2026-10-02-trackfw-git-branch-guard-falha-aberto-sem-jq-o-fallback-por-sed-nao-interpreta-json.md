---
status: Open
date: 2026-10-02
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-02-o-guard-de-branch-extrai-o-comando-do-payload-por-um-parser-json-de-verdade-e-falha-fechado-quando-nao-consegue.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md"
---

# REQ: trackfw-git-branch-guard falha aberto sem jq — o fallback por sed não interpreta JSON

> Date: 2026-10-02 | Status: Open
| GitHub Issue: #507

## Motivation

Sem `jq`, o guard extrai o comando do payload JSON com `sed`, que não desescapa `\n` e para no primeiro
`\"`. Um `git push`, `git commit` ou `git checkout -b` na segunda linha de um comando multilinha passa
sem bloqueio. Medido na #507 em Windows 11 sem `jq`, com controles nas duas direções. No CI o caminho
nunca roda, porque os runners têm `jq`.

Decisão (ADR ligado): extrator JSON em `awk` como fallback, que falha fechado quando a chave existe e o
valor é indecodificável. Toda a tabela de testes do guard passa a rodar com e sem `jq`.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Wave 0:** threat model do extrator (payloads hostis: aspas escapadas,
  contrabarra no fim, `\u0000`, `\u000a`, chave duplicada, chave em objeto aninhado errado, payload de
  200 KB); confirmação por efeito da forma 2 (truncagem no `\"`); e custo do `awk` com payload grande.
  Parecer em `docs/seguranca/`.
- [ ] **AC2** — Sem `jq` (`PATH` curado): os 3 casos multilinha da #507 (`push`, `commit`,
  `checkout -b` na 2ª linha) → rc=2; `echo \"a\"; git push origin main` → rc=2; `git status` → rc=0.
  Reprova no script de `3b2eff09`.
- [ ] **AC3** — `git commit -m "linha 1\nlinha 2"` com contrabarra literal (JSON `\\n`) **não** é
  fatiado: o resultado é igual com e sem `jq`.
- [ ] **AC4** — Toda a tabela de testes existente do guard roda com e sem `jq`, com o mesmo veredito
  em cada caso.
- [ ] **AC5** — Chave do comando presente e indecodificável → rc=2 com mensagem que nomeia a causa;
  chave ausente → comportamento de hoje.
- [ ] **AC6** — As 4 cópias iguais: `TestGitBranchGuardScriptReference_MatchesGenerator` verde,
  `trackfw validate` sem `git_branch_guard_script_integrity` neste repositório, e o cenário de
  falsificação que sabota o guard segue provando a sabotagem (`corrupt_literal` atualizado).
- [ ] **AC5-bis** — NUL no comando decodificado (`git push\u0000origin main`) → rc=2 **com e sem `jq`**.
  Hoje passa nos dois caminhos (Wave 0).
- [ ] **AC7** — Cada teste novo declara a conclusão que afirma.
- [ ] **AC8** — `make quality` EXIT=0 (máquina ociosa) e CI verde, inclusive `windows-full-suites`.

## Negative scope

- **`trackfw-attention-signal.sh`** (fallback por `python3`): outro script e outra função; registrado
  como observação, não corrigido aqui.
- **`credential-guard`:** não tem fallback de extração (medido na #507).
- **Atualização dos consumidores:** chega por `trackfw update`; não forçamos.
- **#502:** outra causa (teste lendo o layout do repositório real).

## Linked ADR
ADR: docs/adr/ADR-2026-10-02-o-guard-de-branch-extrai-o-comando-do-payload-por-um-parser-json-de-verdade-e-falha-fechado-quando-nao-consegue.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md
