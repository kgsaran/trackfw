---
status: wip
date: 2026-08-05
req: "docs/req/REQ-2026-08-05-atualiza-protocolo-de-criacao-de-branch-do-architect-para-usar-trackfw-branch-new.md"
squad: "prometeu-tf"
---

# Roadmap: atualiza protocolo de criação de branch do Architect para usar trackfw branch new

> Created: 2026-08-05 | Status: wip

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-05-atualiza-protocolo-de-criacao-de-branch-do-architect-para-usar-trackfw-branch-new.md`

O template canônico do agente Architect ainda instrui `git checkout -b` cru no parágrafo de Git
authority, apesar de `trackfw branch new <type>/<slug>` (v6.4.0) existir exatamente para mover o
gate `branch_has_wip_roadmap` para antes da criação da branch.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] `internal/integrations/assets/agents/architect.md` instrui `trackfw branch new` como forma
      preferencial, com fallback documentado para `git checkout -b`
- [ ] Mudança sincronizada nos espelhos `npm/`/`pypi/` via `scripts/sync-integration-assets.sh`
- [ ] `make quality`/`make parity` verdes, `trackfw validate` sem violações

## Wave 1 — Atualizar protocolo de Git authority (1 ML)
> Dependencies: none

### ML-1A — Instruir `trackfw branch new` no parágrafo de Git authority
**Status:** pending
**Files affected:** `internal/integrations/assets/agents/architect.md` (canônico),
`npm/src/integrations/assets/agents/architect.md`, `pypi/trackfw/integrations/assets/agents/architect.md`
(sincronizados via `scripts/sync-integration-assets.sh`)
**Actions:**
1. Reescrever o parágrafo "Git authority" (linha ~27) para instruir: preferir `trackfw branch new
   <type>/<slug>` para criar a branch; se o comando não existir ou retornar erro de "comando
   desconhecido" (binário `trackfw` desatualizado ou ausente), cair para `git checkout -b` cru como
   fallback documentado — nunca travar o fluxo do orquestrador por falta do comando.
2. Rodar `scripts/sync-integration-assets.sh` para propagar a mudança aos espelhos npm/pypi.
**Acceptance criteria:**
- [ ] build passes (`go build ./...`)
- [ ] tests green (`go test ./...`, `npm test`, `python3 -m pytest`)
- [ ] `bash scripts/check-integration-assets.sh` verde
- [ ] `trackfw validate` sem violações
