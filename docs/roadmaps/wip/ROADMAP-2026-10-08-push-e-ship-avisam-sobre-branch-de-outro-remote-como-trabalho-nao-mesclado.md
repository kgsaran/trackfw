---
status: wip
date: 2026-10-08
req: "docs/req/REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md"
squad: ""
---

# Roadmap: push e ship avisam sobre branch de outro remote como trabalho nao mesclado

> Created: 2026-10-08 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md -->
REQ: docs/req/REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md

## Acceptance Criteria
- [ ] AC1–AC4 da REQ
- [ ] `make quality` (arquiteto)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e enumeração
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model e sítios multi-remote (AC1)
**Status:** 🔄 Em andamento
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-08-wave0-push-ship-outro-remote.md`
- [ ] Enumeração fechada por grep (`branch", "-r"`, `"origin/"`, `--no-merged`, `TrimPrefix`) em internal/
- [ ] Medição com repositório temporário de dois remotes, nas duas direções, com o binário da main
- [ ] Threat model (o recorte esconde trabalho nosso? remote chamado diferente de `origin`?), residual, Veredito

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-08-wave0-push-ship-outro-remote.md
grep -q Veredito docs/seguranca/2026-10-08-wave0-push-ship-outro-remote.md
```

## Wave 1 — Correção
> Dependencies: Wave 0 auditada.

### ML-1A — Recorte por remote e fixture multi-remote (AC2, AC3)
**Status:** ⬜ Pendente
**Squad:** apolo-tf
- [ ] Testes nas duas direções com dois remotes; falsificação; frase de reconciliação

## Wave 2 — Red-team
### ML-2A — Red-team do diff
**Status:** ⬜ Pendente
**Squad:** hades-tf
- [ ] Parecer sobre o diff
