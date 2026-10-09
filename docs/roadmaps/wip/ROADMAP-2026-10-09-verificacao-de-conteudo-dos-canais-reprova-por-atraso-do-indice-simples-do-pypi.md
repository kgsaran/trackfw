---
status: wip
date: 2026-10-09
req: "docs/req/REQ-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md"
squad: ""
---

# Roadmap: verificacao de conteudo dos canais reprova por atraso do indice simples do PyPI

> Created: 2026-10-09 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md -->
REQ: docs/req/REQ-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md

## Acceptance Criteria
- [ ] AC1–AC4 da REQ
- [ ] `make quality` (arquiteto)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e medição
### ML-0A — Endpoints e enumeração (AC1)
**Status:** 🔄 Em andamento
**Squad:** hades-tf
- [ ] Medição dos endpoints do PyPI (JSON API vs `/simple/`) e do npm, com a 9.4.1 publicada e o histórico dos dois runs
- [ ] Enumeração de todo passo pós-publicação com leitura sem retry (release.yml, scripts/verify-*, check-channels-content)
- [ ] Threat model (o retry pode esconder publicação quebrada? prazo?), residual, Veredito

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-09-wave0-verificacao-canais-indice-pypi.md
grep -q Veredito docs/seguranca/2026-10-09-wave0-verificacao-canais-indice-pypi.md
```

## Wave 1 — Correção
### ML-1A — Retry com prazo na verificação de conteúdo (AC2–AC4)
**Status:** ⬜ Pendente
**Squad:** ares-tf
- [ ] Retry com backoff e prazo para `pip download` e `npm pack`; self-test nas duas direções; comentário do workflow

## Wave 2 — Red-team
### ML-2A — Red-team do diff
**Status:** ⬜ Pendente
**Squad:** hades-tf
- [ ] Parecer
