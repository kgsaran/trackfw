---
status: wip
date: 2026-10-25
req: "docs/req/REQ-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: validate avisa quando o arquivo de hook de projeto não tem o credential guard, nos CLIs sem regra de presença

> Created: 2026-10-25 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md
ADR: ADR-2026-10-04, D17. Issue: #619. Decisão do KG: opção (b), aviso numa minor.

As REQs anteriores (#613 e #618) mostraram que a enumeração precisa cobrir `scripts/check-gates-falsify.sh` e os testes que
afirmam o contrato atual, e não só o Go de produção.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ, cada um com a evidência no ML que o fecha.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e eventos esperados por CLI
> Dependencies: nenhuma. Bloqueia toda implementação.

### ML-0A — Eventos bloqueantes por CLI, sítios e ruído
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-wave0-aviso-presenca-credential-guard.md` (novo; o único arquivo que o ML escreve)
**Acceptance criteria:**
- [ ] Por CLI (os 8): os pares (evento, matcher) de credential guard que o gerador emite no projeto, com arquivo:linha do gerador, e quais deles bloqueiam a execução
- [ ] Como a regra de violation do Windsurf e do Amazon Q (`validateCredentialGuardPresenceRequired`) escolhe os eventos hoje, e se essa lista pode virar o ponto único dos 8
- [ ] Ruído: medir com o binário 11.0.2, em `mktemp -d`, que `init`/`update` de cada CLI, com e sem global, geram todos os pares, e que o aviso não dispararia
- [ ] Testes e cenários de `scripts/` que afirmam "ausência de guard nesses 6 CLIs não é problema" e vão mudar
- [ ] Ameaças: aviso que some sem a fiação existir, ou aviso que o `update` não resolve
- [ ] Recomendação de `Files affected` para a Wave 1

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-25-wave0-aviso-presenca-credential-guard.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Aviso de presença nos 6 CLIs, com lista única de eventos
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** a definir pela Wave 0 (no mínimo `internal/validator/validator_credential_guard.go`, os testes novos e `docs/cli-parity.md`)
**Acceptance criteria:**
- [ ] AC1–AC5 da REQ, com contra-braço em `origin/main`

## Wave 2 — Revisões independentes
> Dependencies: Wave 1 auditada.

### ML-2A — Revisão de qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-25-revisao-aviso-presenca-credential-guard.md`

### ML-2B — Red-team
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-redteam-aviso-presenca-credential-guard.md`

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality, CI e PR
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** nenhum arquivo de produto; só medição
**Acceptance criteria:**
- [ ] `make quality` rc=0 com a máquina ociosa e o HOME isolado
- [ ] Neste repositório, o `validate` do binário da branch não emite o aviso novo (a fiação está completa)
- [ ] CI verde; PR com `Closes #619`
