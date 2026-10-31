---
status: Done
date: 2026-10-30
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink-e-reprova-em-conta-windows-sem-privilegio.md"
---

# REQ: teste do invólucro de atencao materializa o binario por symlink e reprova em conta Windows sem privilegio

> Date: 2026-10-30 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Origem

Issue #634, de `lourivalgarciajunior`: `TestAttentionScripts_FallbackWithoutJQ` reprova 3 de 3 numa
conta Windows sem `SeCreateSymbolicLinkPrivilege`. O controle positivo do autor (binário alcançável no
PATH do filho → payload escrito) está no issue.

## Motivation

`internal/generators/scaffold_test.go:398–416` põe o `trackfw` compilado da árvore
(`compiledGuardBinary`) no `fakeBinDir` por `os.Symlink`. Sem privilégio, o erro é classificado e
ignorado, o `fakeBinDir` fica vazio, e o invólucro sai 0 sem escrever nada
(`command -v trackfw || exit 0`). O teste reprova duas asserções depois. O CI não vê porque o
`windows-latest` tem privilégio de symlink.

A entrada em `.github/windows-known-failures.json` diz `removal_note: "corrected"` pelo ML-1H do
`ROADMAP-2026-09-22-a-instalacao-em-windows-promete-o-que-nao-entrega-...`. Aquele ML tirou a
dependência de `jq` e do ajudante falso; a dependência de symlink (Group C) ficou. **Decisão:** REQ
nova citando a anterior, não reabertura: a REQ antiga (instalação no Windows) está fechada há mais de
um mês, e o resíduo é um único sítio de teste.

**Enumeração da mesma causa** (`grep -rn -B4 "os.Symlink(" internal --include='*_test.go'`):
só dois testes materializam binário por symlink. `git_branch_guard_test.go:932` já cai para cópia
(`copyExecutableFile`) sem privilégio. Os demais ~20 `os.Symlink` testam o próprio symlink e pulam
legitimamente sem privilégio.

## Acceptance Criteria

- [ ] **AC1** — o teste não usa symlink para pôr o binário no PATH do filho (diretório do binário
      compilado no PATH, ou cópia com `filepath.Base` do binário, que preserva o `.exe`).
- [ ] **AC2** — o PATH do filho continua sem `jq` (o teste ainda afirma o modo sem jq).
- [ ] **AC3** — falta do binário no PATH do filho reprova no setup com mensagem que nomeia a causa,
      não duas asserções depois.
- [ ] **AC4** — medido numa conta Windows sem privilégio de symlink (VM), antes e depois; se a conta
      tiver o privilégio, isso fica escrito e a prova é o teste sem symlink mais o contra-braço.

## Negative Scope

- Não muda o invólucro gerado nem o `trackfw attention`.
- Não mexe nos testes cujo sujeito é o próprio symlink.

## Linked ADR
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink-e-reprova-em-conta-windows-sem-privilegio.md
