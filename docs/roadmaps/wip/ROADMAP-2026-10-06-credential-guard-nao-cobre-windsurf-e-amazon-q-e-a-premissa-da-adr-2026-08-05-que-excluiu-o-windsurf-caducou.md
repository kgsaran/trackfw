---
status: wip
date: 2026-10-06
req: "docs/req/REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
squad: ""
---

# Roadmap: credential guard nao cobre Windsurf e Amazon Q, e a premissa da ADR-2026-08-05 que excluiu o Windsurf caducou

> Created: 2026-10-06 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md -->
REQ: docs/req/REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] AC1 da REQ — eventos de hook do Windsurf e do Amazon Q remedidos com fonte oficial e data (ML-0B)
- [ ] AC2 da REQ — adendo à ADR-2026-08-05 com a decisão por CLI (ML-1A)
- [ ] AC3 da REQ — `init`/`update`/`update harness` emitem o credential guard para cada CLI decidido "instalar"; `validate` deixa de silenciar; teste nas duas direções (ML-1B)
- [ ] AC4 da REQ — prova de disparo real ou impossibilidade declarada com motivo (ML-2A)
- [ ] `make quality` verde (arquiteto, sem `~/.local/bin` no PATH)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model e remedição (2 MLs, mesmo despacho do hades-tf)
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** 🔄 Em andamento
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md` (novo)
**Actions:**
1. Enumeration completeness — todos os sítios que emitem, migram ou auditam o credential guard por CLI (gerador de projeto, `update`, `update harness`/`buildHarnessTargetIDs`, `harnessCatalogTargetOrder`, `validate`, detectores globais, `check-*` scripts, `docs/cli-parity.md`). Fechar a lista por grep do literal `trackfw guard credential` e de `credential-guard`, não só pelos arquivos da REQ.
2. Threat model — quem esvazia esta Wave 0 sem quebrar regra escrita, e como.
3. Falsification targets in both directions — por superfície: o que quebra se o guard deixar de ser emitido, e o que quebra se for emitido num evento que não bloqueia (falsa sensação de proteção) ou que bloqueia tudo (deny-all).
4. Declared residual.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

### ML-0B — Remedição dos eventos de hook (AC1 da REQ)
**Status:** 🔄 Em andamento
**Squad:** hades-tf
**Files affected:** mesmo documento do ML-0A, seção "Remedição"
**Actions:** por CLI (Windsurf, Amazon Q CLI), com URL da documentação oficial e data de acesso: quais eventos existem antes de ler arquivo, escrever arquivo e executar comando; que payload chega no stdin (comando? caminho? conteúdo escrito?); exit code que bloqueia vs só avisa; se há escopo global (arquivo de usuário) além do de projeto. Confrontar com o que o gerador emite hoje.
**Acceptance criteria:**
- [ ] Tabela por CLI × evento com fonte e data; cada célula "bloqueia/avisa/não existe" tem citação
- [ ] Recomendação por CLI: instalar (em qual evento e com qual nome de guard) ou manter fora (motivo medido)

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md
grep -q "Veredito" docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md
```

## Wave 1 — Decisão e implementação
> Dependencies: Wave 0 auditada. O detalhe dos MLs é fechado a partir do parecer da Wave 0.

### ML-1A — Adendo à ADR-2026-08-05 (AC2)
**Status:** ⬜ Pendente
**Squad:** zeus-tf
**Files affected:** `docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
**Acceptance criteria:**
- [ ] Adendo datado revendo a premissa do Windsurf e avaliando o Amazon Q, decisão por CLI citando o ML-0B

### ML-1B — Gerador, update, harness e validate (AC3)
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:** a fechar após Wave 0 (`internal/generators/agentfiles.go`, `internal/generators/update.go`, `internal/validator/validator_credential_guard.go` e testes)
**Acceptance criteria:**
- [ ] Teste: `init`/`update` emitem a linha D11 revista do credential guard no evento decidido, por CLI decidido
- [ ] Teste: `validate` acusa o arquivo sem o credential guard (antes silenciado) e não acusa o arquivo correto
- [ ] Falsificação nas duas direções, com a frase de reconciliação por teste novo
- [ ] Configs deste repo e `docs/cli-parity.md` atualizados

## Wave 2 — Prova e red-team
> Dependencies: Wave 1 auditada.

### ML-2A — Prova de disparo real (AC4)
**Status:** ⬜ Pendente
**Squad:** zeus-tf
**Acceptance criteria:**
- [ ] Disparo real medido em ao menos um CLI, ou impossibilidade declarada (sem conta de Windsurf/Amazon Q, decisão do usuário)

### ML-2B — Red-team do diff
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Acceptance criteria:**
- [ ] Parecer sobre o diff da Wave 1 contra o threat model do ML-0A; achados corrigidos nesta REQ
