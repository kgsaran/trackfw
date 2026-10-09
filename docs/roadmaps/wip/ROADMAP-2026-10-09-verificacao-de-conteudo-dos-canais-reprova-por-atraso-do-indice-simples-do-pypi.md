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
**Status:** ✅ Concluído
**Squad:** hades-tf
- [x] Medição dos endpoints do PyPI (JSON API vs `/simple/`) e do npm, com a 9.4.1 publicada e o histórico dos dois runs
- [x] Enumeração de todo passo pós-publicação com leitura sem retry (release.yml, scripts/verify-*, check-channels-content)
- [x] Threat model (o retry pode esconder publicação quebrada? prazo?), residual, Veredito
      Parecer (`docs/seguranca/2026-10-09-wave0-verificacao-canais-indice-pypi.md`): `/simple/` servido pelo Fastly com
      `max-age=600`; falhas a 165 s (9.3.3) e 193 s (9.4.1) do upload. Decisões do arquiteto na auditoria:
      (1) prazo de 300 s proposto é menor que o `max-age` de 600 s medido → prazo 900 s;
      (2) achado da auditoria, mesma causa (o passo D7 não lê o que foi publicado pelo que é): `pip download` sem
      `--platform` baixa SÓ a wheel da plataforma do runner — o D7 inspeciona 1 de 8 wheels. Correção: listar as wheels
      pela JSON API (com retry) e baixar cada URL de `files.pythonhosted.org` (não depende do `/simple/`), exigindo as 8;
      (3) `npm pack` recebe o mesmo retry (residual do parecer não fica).

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-09-wave0-verificacao-canais-indice-pypi.md
grep -q Veredito docs/seguranca/2026-10-09-wave0-verificacao-canais-indice-pypi.md
```

## Wave 1 — Correção
### ML-1A — Retry com prazo na verificação de conteúdo (AC2–AC4)
**Status:** ✅ Concluído
**Squad:** ares-tf
- [x] Retry com backoff e prazo para `pip download` e `npm pack`; self-test nas duas direções; comentário do workflow
      Auditoria (2026-10-09): o relatório omitiu a execução real e as falsificações pedidas — feitas pelo arquiteto:
      `--published 9.4.1` real → 8 de 8 wheels inspecionadas (10 OK); cópia exigindo uma 9ª etiqueta inexistente →
      `FAIL: missing expected wheel tag: linux_riscv64`. Self-test 9/9 (ligado ao `make quality` pelo Makefile:177).
      `make quality` (arquiteto) EXIT=0, 347 OK.

## Wave 2 — Red-team
### ML-2A — Red-team do diff
**Status:** 🔄 Em andamento
**Squad:** hades-tf
- [ ] Parecer
