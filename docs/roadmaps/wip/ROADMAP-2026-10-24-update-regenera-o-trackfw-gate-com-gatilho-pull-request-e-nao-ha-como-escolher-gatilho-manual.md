---
status: wip
date: 2026-10-24
req: "docs/req/REQ-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: gatilho configurável dos workflows gerados (ci_trigger)

> Created: 2026-10-24 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md
Issue: #607. ADR-2026-08-28, adendo de 2026-10-24.

## Acceptance Criteria
- [ ] AC1–AC5 da REQ
- [ ] `make quality` e o CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do gatilho configurável
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-24-wave0-ci-trigger.md`
**Actions:**
1. **Enumeração fechada:** procure por grep todo sítio que emite, compara ou valida um workflow gerado:
   - `pull_request`, `merge_requests` e `on:` em `internal/`;
   - os três templates, o scaffold doctor e as regras do validate sobre o workflow de governança (ADR-2026-09-29);
   - os testes que fixam os templates byte a byte.

   Cruze todas as dimensões: template × produtor/consumidor × GitHub/GitLab × `init`/`update`/`discover` × doctor/validate.
2. **Ameaças:**
   - `ci_trigger: manual` desliga o gate de PR em silêncio: quem vê isso, e o `validate` precisa avisar?;
   - um valor com YAML injetado no template;
   - um valor desconhecido;
   - a regra de "um workflow por projeto" (ADR-2026-09-29) com o workflow manual;
   - `required_status_checks` apontando para um job que nunca roda.
3. **Alvos de falsificação** nos dois sentidos e **resíduo declarado**.
**Acceptance criteria:**
- [ ] A matriz com as fontes (arquivo:linha)
- [ ] As ameaças respondidas com evidência
- [ ] Nenhuma linha de produto
**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-24-wave0-ci-trigger.md
git diff --quiet origin/main -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada. Detalhada depois da Wave 0.

### ML-1A — `ci_trigger` nos templates, no doctor e no validate
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** a definir pela Wave 0
**Acceptance criteria:**
- [ ] AC1–AC4 da REQ

## Wave 2 — Fechamento
> Dependencies: Wave 1 auditada.

### ML-2A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] `make quality` rc=0
- [ ] CI do PR verde, com `Closes #607`
