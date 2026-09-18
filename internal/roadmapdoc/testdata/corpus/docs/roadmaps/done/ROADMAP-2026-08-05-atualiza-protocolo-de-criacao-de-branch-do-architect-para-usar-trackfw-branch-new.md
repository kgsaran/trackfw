---
status: done
date: 2026-08-05
req: "docs/req/REQ-2026-08-05-atualiza-protocolo-de-criacao-de-branch-do-architect-para-usar-trackfw-branch-new.md"
squad: "prometeu-tf"
---

# Roadmap: atualiza protocolo de criação de branch do Architect para usar trackfw branch new

> Created: 2026-08-05 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-05-atualiza-protocolo-de-criacao-de-branch-do-architect-para-usar-trackfw-branch-new.md`

O template canônico do agente Architect ainda instrui `git checkout -b` cru no parágrafo de Git
authority, apesar de `trackfw branch new <type>/<slug>` (v6.4.0) existir exatamente para mover o
gate `branch_has_wip_roadmap` para antes da criação da branch.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] `internal/integrations/assets/agents/architect.md` instrui `trackfw branch new` como forma
      preferencial, com fallback documentado para `git checkout -b`
- [x] Mudança sincronizada nos espelhos `npm/`/`pypi/` via `scripts/sync-integration-assets.sh`
- [x] `make quality`/`make parity` verdes, `trackfw validate` sem violações

## Wave 1 — Atualizar protocolo de Git authority (1 ML)
> Dependencies: none

### ML-1A — Instruir `trackfw branch new` no parágrafo de Git authority
**Status:** ✅ Concluído
**Files affected:** `internal/integrations/assets/agents/architect.md` (canônico),
`npm/src/integrations/assets/agents/architect.md`, `pypi/trackfw/integrations/assets/agents/architect.md`
(sincronizados via `scripts/sync-integration-assets.sh`); adicionalmente
`internal/integrations/testdata/architect.agent-directory.golden.md`,
`internal/integrations/testdata/architect.subagent.golden.md` e
`npm/tests/agents-skills.test.js` (fixtures que fixavam o texto literal do parágrafo antigo e
quebravam com a mudança de conteúdo — atualizados para o novo texto).
**Actions:**
1. Reescrever o parágrafo "Git authority" (linha ~27) para instruir: preferir `trackfw branch new
   <type>/<slug>` para criar a branch; se o comando não existir ou retornar erro de "comando
   desconhecido" (binário `trackfw` desatualizado ou ausente), cair para `git checkout -b` cru como
   fallback documentado — nunca travar o fluxo do orquestrador por falta do comando.
2. Rodar `scripts/sync-integration-assets.sh` para propagar a mudança aos espelhos npm/pypi.
3. Atualizar os golden fixtures de Go (`internal/integrations/testdata/architect.*.golden.md`) e o
   literal esperado em `npm/tests/agents-skills.test.js` para o novo texto do parágrafo, já que
   fixavam o texto antigo byte-a-byte.
**Acceptance criteria:**
- [x] build passes (`go build ./...`)
- [x] tests green (`go test ./...`, `npm test`, `python3 -m pytest`)
- [x] `bash scripts/check-integration-assets.sh` verde
- [x] `trackfw validate` sem violações
