---
status: wip
date: 2026-10-08
req: ""
squad: ""
---

# Roadmap: req_done_open_criteria decompoe o numero agregado e mostra quantas REQs isentas sao herdadas de outro repositorio

> Created: 2026-10-08 | Status: wip

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: 

## Acceptance Criteria
- [ ] AC1–AC5 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e discriminante
> Dependencies: PR #543 mergeado (ok, 2026-10-08). Blocks all implementation.

### ML-0A — Threat model e escolha do discriminante (AC1)
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/<data>-wave0-req-done-open-criteria-herdadas.md`
**Actions:**
1. Enumeration completeness — todos os sítios que emitem a linha agregada (texto e `--json`), e quem a lê (gates, scripts, `check-validate-rule-pins.sh`, docs).
2. Threat model — quem usa o recorte "herdada" para esconder dívida própria (ex.: REQ local com o mesmo basename de uma do upstream; remote `upstream` apontando para o próprio repo).
3. Falsification targets nas duas direções.
4. Declared residual.
5. Medir custo e semântica do discriminante derivado (`git ls-tree` no ramo padrão de `upstream`, ref ausente, fetch velho, shallow clone) vs declarado (`upstream_origin`).
**Acceptance criteria:**
- [x] Seções respondidas com evidência e recomendação do discriminante

**Gates da wave:**
```bash
test -n "$(ls docs/seguranca/*wave0-req-done-open-criteria-herdadas.md 2>/dev/null)"
```

      Auditoria (2026-10-08): veredito do hades-tf aceito — discriminante derivado (basename da REQ presente no `req_dir`
      do próprio upstream, lido de `<ref>:trackfw.yaml`), ~37 ms medido no fork do relator, concordância 28/28 com o
      `upstream_origin` dele. Decisões do arquiteto sobre §5.4: o baseline já é instável a qualquer mudança de contagem desta
      linha; a parentética só acrescenta a dependência do fetch — documentar, não normalizar. Variante "ref unresolvable"
      aceita (diz por que o recorte falta, em vez de silenciar).

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Recorte na linha agregada (AC2–AC4)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/validator/validator_req_done_criteria.go`, testes, `docs/cli-parity.md` se documenta a linha
**Acceptance criteria:**
- [x] Sem remote `upstream`: saída byte-idêntica (teste)
- [x] Com `upstream`: recorte correto nas duas direções (teste com repo temporário)
- [x] Falsificação e frase de reconciliação por teste novo
- [x] `make quality` (arquiteto)
      Auditoria (2026-10-08): 5 testes conferidos por nome. Artefato real: clone do fork `lourivalgarciajunior/trackfw`
      com o binário desta branch — sem `upstream`: linha idêntica à atual (22 exempt, 77 scanned); com `upstream` = este
      repo + fetch: `22 ... (22 inherited from upstream/main)`, o mesmo número da issue. `--json` leva o recorte em
      `warnings[].message` (sem campo novo — documentado no cli-parity). `make quality` EXIT=0 (executor), 347 OK / 0 FAIL.

## Wave 2 — Red-team e fechamento
### ML-2A — Red-team do diff
**Status:** ✅ Concluído
**Squad:** hades-tf
- [x] Parecer sobre o diff contra o threat model
      Veredito do hades-tf (`docs/seguranca/2026-10-08-red-team-req-done-open-criteria-herdadas.md`): **não libera**.
      F1 (médio): nome de ramo com `"` vindo de `refs/remotes/upstream/HEAD` entra na parentética e o `--json` extrai um
      `warnings[].file` falso (viola §6.2 da Wave 0). F2 (info, residual): `trackfw.yaml` de 47 MB no upstream custa
      510 ms. Byte-identidade sem upstream, injeção por req_dir/pathspec, GIT_* e decisões da regra: medidos, sem achado.

### ML-2B — Corretivo F1
**Status:** 🔄 Em andamento
**Squad:** apolo-tf
- [ ] O nome do ramo na parentética só aceita caracteres seguros; fora disso, cai para a variante sem nome
- [ ] Teste com ramo contendo `"`: `warnings[].file` não é afetado no `--json`; falsificação
- [ ] `go test ./internal/validator/`; `make quality` (arquiteto)

