---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-corrigir-dispatch-sem-subagent-type-no-template-do-architect.md"
squad: "prometeu-tf"
---

# Roadmap: corrigir dispatch sem subagent_type no template do Architect

> Created: 2026-08-04 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-corrigir-dispatch-sem-subagent-type-no-template-do-architect.md`

Zeus (`zeus-tf`) nomeou especialistas na prosa/`squad:` sem passar `subagent_type` na Agent tool; o
harness caiu no default `general-purpose`. O template canônico do Architect não instrui o dispatch
técnico correto. A tabela de mapeamento nome→subagent_type NÃO pode ser hardcoded (identidade é
configurável por preset grego/nórdico/HP/custom — ver `internal/identity/preset.go` e
`docs/cli-parity.md` § Agent identity); a instrução deve ser identity-agnostic: ler o `name:` do
frontmatter do agente instalado do role-alvo.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Seção "Dispatch contract" adicionada a `internal/integrations/assets/agents/architect.md`
- [x] `npm/src/integrations/assets/agents/architect.md` e `pypi/trackfw/integrations/assets/agents/architect.md`
      byte-idênticos ao canônico (`scripts/sync-integration-assets.sh`)
- [x] Goldens Go atualizados e `go test ./internal/integrations/...` verde
- [x] `bash scripts/check-integration-assets.sh` verde
- [x] Nenhuma menção a `subagent_type` em templates de superfícies não-Claude-Code

## Wave 1 — Corrigir template do Architect (1 ML)
> Dependencies: none

### ML-1A — Adicionar dispatch contract identity-agnostic ao template do Architect
**Status:** ✅ Concluído
**Files affected:**
- `internal/integrations/assets/agents/architect.md` (canônico)
- `npm/src/integrations/assets/agents/architect.md` (espelho, via sync script)
- `pypi/trackfw/integrations/assets/agents/architect.md` (espelho, via sync script)
- `internal/integrations/testdata/architect.subagent.golden.md`
- `internal/integrations/testdata/architect.agent-directory.golden.md`
- Qualquer fixture Node/Python que compare o corpo completo de `architect.md` (buscar antes de editar)

**Actions:**
1. No arquivo canônico, adicionar uma nova seção (ex: entre "## Workflow" e "## Post-microbatch audit")
   com o conteúdo: (a) nomear um especialista na prosa/`squad:` não roteia a chamada da Agent tool;
   (b) todo dispatch DEVE passar `subagent_type` explícito, sob pena de cair no default `general-purpose`
   silenciosamente; (c) o valor correto é o `name:` do frontmatter do agente instalado daquele role
   (sempre `<slug>-tf`, `<slug>` depende da identidade do usuário — nunca assumir nome fixo); se
   desconhecido, ler o arquivo do agente instalado antes de despachar, em vez de adivinhar.
2. Rodar `scripts/sync-integration-assets.sh` a partir da raiz para propagar aos espelhos npm/pypi
   byte-a-byte. Não editar os espelhos manualmente. Não tocar `pypi/build/lib/...` (artefato de build).
3. Atualizar os dois goldens Go listados acima para refletir a nova seção.
4. `grep -rn "Post-microbatch audit\|## Workflow" npm/tests pypi/tests internal/integrations` e ajustar
   qualquer fixture que compare o corpo completo do arquivo.
5. Não propagar `subagent_type` para templates Gemini/Copilot/Windsurf/Codex — parâmetro exclusivo do
   Claude Code.

**Acceptance criteria:**
- [x] `go build ./...` sem erros
- [x] `go test ./internal/integrations/...` verde
- [x] `bash scripts/check-integration-assets.sh` verde
- [x] `trackfw validate` sem novas violações
