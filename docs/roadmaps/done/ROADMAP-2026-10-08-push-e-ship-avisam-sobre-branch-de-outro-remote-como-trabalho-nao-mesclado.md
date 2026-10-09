---
status: done
date: 2026-10-08
req: "docs/req/REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md"
squad: ""
---

# Roadmap: push e ship avisam sobre branch de outro remote como trabalho nao mesclado

> Created: 2026-10-08 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md -->
REQ: docs/req/REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md

## Acceptance Criteria
- [x] AC1–AC3 da REQ (AC4 — comentário na #547 — no PR)
- [x] `make quality` EXIT=0

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e enumeração
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model e sítios multi-remote (AC1)
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-08-wave0-push-ship-outro-remote.md`
- [x] Enumeração fechada por grep (`branch", "-r"`, `"origin/"`, `--no-merged`, `TrimPrefix`) em internal/
- [x] Medição com repositório temporário de dois remotes, nas duas direções, com o binário da main
- [x] Threat model (o recorte esconde trabalho nosso? remote chamado diferente de `origin`?), residual, Veredito
      Auditoria (2026-10-08): sítio único (`detectPendingSquashMerges`), defeito medido com dois remotes bare (aviso falso
      para `upstream/fix/...`, verdadeiro para `origin/feat/pending`, silêncio para `upstream/main`). Achado R3 no mesmo
      laço: `strings.Contains(candidate, "HEAD")` descarta `origin/fix/HEADER-parse` — mesma causa (população de
      candidatos recortada por predicado de string frouxo) → entra no ML-1A, não fica fora.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-08-wave0-push-ship-outro-remote.md
grep -q Veredito docs/seguranca/2026-10-08-wave0-push-ship-outro-remote.md
```

## Wave 1 — Correção
> Dependencies: Wave 0 auditada.

### ML-1A — Recorte por remote e fixture multi-remote (AC2, AC3)
**Status:** ✅ Concluído
**Squad:** apolo-tf
- [x] Testes nas duas direções com dois remotes; falsificação; frase de reconciliação
      Auditoria (2026-10-08): `TestDetectPendingSquashMerges_Issue547_MultiRemote` conferido por nome; diff lido (HasPrefix
      `origin/` + descarte exato de `<remote>/HEAD`); falsificações nas duas direções registradas pelo executor; `make
      quality` EXIT=0 (executor), 347 OK. Reprodução com repositório real de dois remotes pelo arquiteto: não feita — o
      guard do projeto bloqueia `git update-ref`/`git commit` crus, e não foi contornado; a medição ao vivo do defeito é a
      da Wave 0. Agente parado com TaskStop (ficou preso após o relatório).

## Wave 2 — Red-team
### ML-2A — Red-team do diff
**Status:** ✅ Concluído
**Squad:** hades-tf
- [x] Parecer sobre o diff
      Veredito do hades-tf (`docs/seguranca/2026-10-08-red-team-push-ship-outro-remote.md`): **aprova**. 8 casos de HEAD,
      prefixos `origin-mirror`/`originx`, callers reconfirmados (push.go e ship.go), as duas mutações reprovam o teste.
      Residual pré-existente: remote principal com nome diferente de `origin` (fora do escopo, saída 2 da issue).

