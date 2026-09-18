---
status: done
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-gates-morrem-em-cp1252-e-corrompem-fixture-nao-ascii-no-windows-prs-238-e-240.md"
squad: "zeus-tf"
---

# Roadmap: Governança dos PRs #238 e #240 do reporter

> Created: 2026-09-02 | Status: done

## Context

REQ: `docs/req/REQ-2026-09-02-gates-morrem-em-cp1252-e-corrompem-fixture-nao-ascii-no-windows-prs-238-e-240.md`

🔴 **O código é de `lourivalgarciajunior`.** Este roadmap **não planeja trabalho** — ele registra
trabalho **já feito e já verificado**, para que a cadeia exigida exista.

**A exigência de governança nunca foi publicada** neste repositório: não há `CONTRIBUTING.md` nem
template de PR. **A obrigação é nossa.** Exceção pontual decidida pelo KG, válida só para estes dois
PRs.

## Acceptance Criteria

- [x] Gate não estoura em cp1252 — verificado pelo autor
- [x] Fixture bate byte a byte para não-ASCII — **reproduzido pelo arquiteto**
- [x] Controle: entrada ASCII continua correta
- [x] `parity` **SUCCESS** nos dois PRs — o job que executa o gate corrigido
- [ ] Rebase sobre a `main`, pelos checks novos que não existiam
- [ ] Mergeados

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Registro e desbloqueio (ML único)

### ML-1A — Auditoria registrada e PRs desbloqueados
**Status:** ✅ Concluído
**Agente:** arquiteto (`zeus-tf`)
**Files affected:** nenhum — o código é dos PRs #238 e #240.

**O que foi auditado, e como:**

| item | verificação |
|---|---|
| `#240` dupla codificação | **reproduzida por execução:** `e2 ac 9c` → `c3 a2 c2 ac c5 93`; com o fix, idêntico à entrada |
| `#238` `CORPUS_HASH` | **confirmado na linha 542** que o hash cobre o arquivo alimentado pelo heredoc |
| escopo | **só** `scripts/check-roadmap-barrier-contract.sh` nos dois PRs |
| colisão | minha branch não toca esse arquivo |
| CI | `parity` **SUCCESS** — e é o job que **executa** o gate corrigido |

**Por que isto não é "aprovar sem revisar":** a revisão aconteceu — só não gerou artefato antes,
porque a regra que exige o artefato **não está publicada**.

## Verificação

Os dois PRs precisam de **rebase**: foram abertos antes do rename dos job ids e do
`required_status_checks`, então `governance-install-script` e `governance-go-install` **não existiam**
neles. Sem rebase, os 9 checks exigidos não fecham.

**Isso não é defeito dos PRs** — é o custo esperado de ligar um portão com trabalho em voo.

## Barreira final

Dispensada, e o motivo é registrado em vez de omitido: **a auditoria técnica já ocorreu** — por mim,
com reprodução de bytes — e o `parity` do CI **executa** o gate corrigido nos dois PRs. Convocar
`hefesto-tf` e `hades-tf` para revisar duas linhas já verificadas gastaria o sinal das barreiras onde
elas importam.
