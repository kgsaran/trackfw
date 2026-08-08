---
status: Done
date: 2026-08-08
author: ""
adr: ""
roadmap: ""
---

# REQ: adr new com escopo global (--scope global) escrevendo em ~/.trackfw/adr

> Date: 2026-08-08 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
`trackfw adr new` só sabe escrever no `adr_dirs` do projeto atual (`docs/adr` por
padrão) — decisões arquiteturais que são genuinamente cross-project (ex.:
ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md,
criado manualmente fora do fluxo do CLI porque não havia comando para isso)
não têm um caminho oficial. O usuário já usa a convenção `adr_dirs`
suportando caminhos `~/...` (expansão de til já implementada e testada —
`internal/config/config_paths_test.go:TestConfigTildeExpansionInAdrDirs`)
para dar visibilidade de ADRs globais a um projeto específico, mas hoje não
existe comando para *criar* esses ADRs num diretório fixo e estável fora de
qualquer projeto.

**Fora de escopo desta REQ (decisão explícita, evita scope creep):**
- `trackfw validate`/`status`/`context` continuam enxergando só os
  `adr_dirs` configurados no `trackfw.yaml` do projeto atual — nenhuma
  varredura implícita de `~/.trackfw/adr`. Se o usuário quiser que um
  projeto veja os ADRs globais, ele mesmo adiciona `~/.trackfw/adr` ao
  `adr_dirs` daquele projeto (capacidade já existente, inalterada).
- `trackfw req new --from-req`/o fluxo de ADR vinculado a REQ (`NewADRDraft`,
  usado por `req.go`) não ganha escopo global — um ADR draft nascido de uma
  REQ é inerentemente do projeto onde a REQ vive.
- Drift pré-existente entre os 3 CLIs (Python's `adr new` já tem `--dir`/
  `--status`, Go/Node não têm equivalente, `docs/cli-parity.md` não
  documenta isso como exceção sancionada) não é corrigido aqui — é
  ortogonal a esta REQ e fica registrado como observação, não como escopo.

## Acceptance Criteria
- [ ] `trackfw adr new "título" --scope global` (Go/Node.js/Python) escreve em
      `~/.trackfw/adr/ADR-<data>-<slug>.md`, sem exigir `trackfw.yaml`/raiz de
      projeto no cwd (mesmo padrão de `trackfw update harness`)
- [ ] `--scope project` (default, comportamento atual inalterado) continua
      escrevendo em `adr_dirs[0]` do `trackfw.yaml` do projeto
- [ ] `trackfw adr list --scope global` lista `~/.trackfw/adr/*.md`;
      `--scope project` (default) continua listando `adr_dirs[0]`
- [ ] Conteúdo/formato do ADR gerado idêntico entre os dois escopos (só o
      diretório de destino muda — reusa o mesmo template/wizard)
- [ ] Testes Go/Node.js/Python verdes, incluindo casos de `--scope global`
      com `$HOME` de fixture (nunca o `$HOME` real do ambiente de teste)
- [ ] `docs/cli-parity.md` atualizado com a nova flag `--scope` de `adr new`/
      `adr list`
- [ ] `trackfw validate` sem violações

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: <!-- nenhum — feature aditiva (novo flag opcional, default preserva comportamento atual), não requer novo ADR -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-08-adr-new-com-escopo-global-scope-global-escrevendo-em-trackfw-adr.md
