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
- [x] AC1–AC4 da REQ
- [x] `make quality` EXIT=0

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
**Status:** ✅ Concluído
**Squad:** hades-tf
- [x] Parecer
      Veredito do hades-tf (`docs/seguranca/2026-10-09-red-team-verificacao-canais.md`): aprova com ressalvas. A1 (médio)
      sha256 da JSON API não conferido após o download; A2 (baixo) lista `urls` parcial reprova sem retry; A3 (baixo)
      variáveis de injeção ativas fora do self-test; e etiqueta de wheel inesperada só avisa. Decisão do arquiteto (regra
      do KG: defeito conhecido se corrige agora): todos entram no ML-2B; nenhum fica como residual.

### ML-2B — Corretivo do red-team
**Status:** ✅ Concluído
**Squad:** ares-tf
- [x] A1: sha256 de cada wheel conferido contra `digests.sha256` da JSON API; host do download restrito a `files.pythonhosted.org` (inclusive após redirect)
- [x] A2: lista `urls` com menos wheels que o esperado entra no retry (não reprova cedo)
- [x] A3: `PYPI_JSON_CMD`/`FETCH_CMD`/`NPM_PACK_CMD`/relógio injetável só valem no `--self-test`; no `--published` são ignorados (e avisados)
- [x] Etiqueta inesperada reprova (a lista e o release têm de concordar)
- [x] Braços de self-test para cada um; falsificação; `make quality` (arquiteto)
      Auditoria (2026-10-09): self-test 14/14; `--published 9.4.1` real 8 de 8 com sha256 conferido. O relatório
      declarou falsificações sem executá-las — executada pelo arquiteto a do sha256 (conferência desligada → braço 10
      reprova). `make quality` (arquiteto) EXIT=0, 347 OK.

