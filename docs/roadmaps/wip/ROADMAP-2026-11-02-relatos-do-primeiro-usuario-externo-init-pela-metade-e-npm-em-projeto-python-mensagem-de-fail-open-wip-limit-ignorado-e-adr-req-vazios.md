---
status: wip
date: 2026-11-02
req: "docs/req/REQ-2026-11-02-relatos-do-primeiro-usuario-externo-init-pela-metade-e-npm-em-projeto-python-mensagem-de-fail-open-wip-limit-ignorado-e-adr-req-vazios.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Relatos do primeiro usuário externo: init pela metade e npm em projeto Python, mensagem de fail open, wip_limit ignorado e ADR/REQ vazios

> Created: 2026-11-02 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-02-relatos-do-primeiro-usuario-externo-init-pela-metade-e-npm-em-projeto-python-mensagem-de-fail-open-wip-limit-ignorado-e-adr-req-vazios.md
Issues: #657, #658, #659, #660, #661

As causas estão medidas e escritas na REQ, uma por AC. A varredura de issues abertos em 2026-11-02 achou só estes cinco.

🔴 **Todo teste novo que chame `validate` depois de `init`** precisa ser rodado com o PATH sem `trackfw` antes do push (lição do #655).

## Acceptance Criteria
- [ ] AC1–AC6 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-02-wave0-relatos-primeiro-usuario-externo.md`
**Actions:**
1. Completude da enumeração, por causa:
   - todos os sítios que emitem "fail open" e o que cada um descreve de fato;
   - todos os consumidores do `wip_limit` (validate, status, serve, barrier, docs, testes);
   - todos os pontos do `init` que gravam `pkg_manager` e as consequências do valor (scripts, hooks gerados);
   - todos os pontos de escrita do `init` no projeto e no global, e a ordem em que acontecem;
   - os modelos de ADR e REQ (gerados por `adr new` e `req new`, e o fallback sem assistente) e o que conta como "só modelo".
   
   Liste também os testes e documentos que afirmam o comportamento atual.
2. Modelo de ameaça:
   - a regra do AC5 pode ser satisfeita com conteúdo vazio de outro jeito, por exemplo com um caractere?
   - a pré-checagem do AC4 abre uma janela TOCTOU (time-of-check to time-of-use) entre checar e escrever?
   - o `--scope project` pode escrever fora da raiz?
   - a mudança do padrão do `wip_limit` muda o resultado do `barrier`, do `commit` ou do `ship`?
3. Alvos de falsificação nos dois sentidos, do AC1 ao AC5.
4. Residual declarado.
5. `## Prescrições`, incluindo o nome da regra do AC5 e a lista de arquivos de cada ML, sem arquivo em comum entre o ML-1A e o ML-1B.
**Acceptance criteria:**
- [ ] As cinco seções respondidas com evidência (comando e resultado)
- [ ] Prescrições em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-02-wave0-relatos-primeiro-usuario-externo.md
grep -q '^## Prescrições' docs/seguranca/2026-11-02-wave0-relatos-primeiro-usuario-externo.md
```

## Wave 1 — Implementação (2 executores; arquivos definidos pela Wave 0)
> Dependencies: Wave 0 auditada. O ML-1A cuida do validator e da config (AC1, AC2 e AC5); o ML-1B cuida do `init` (AC3 e AC4). O `docs/cli-parity.md` fica só com o ML-1A, e o texto do ML-1B entra por ele ou pelo arquiteto.

### ML-1A — Validator: mensagem de fail closed, wip_limit configurável, regra de modelo vazio (apolo-tf) — AC1, AC2, AC5
**Status:** ⬜ Pendente
**Files affected:** definidos pela Wave 0.
**Acceptance criteria:**
- [ ] pacotes tocados verdes, inclusive com PATH sem trackfw
- [ ] contra-braço por AC

### ML-1B — Init: pré-checagem do destino, `--scope`, pkg_manager detectado (apolo-tf) — AC3, AC4
**Status:** ⬜ Pendente
**Files affected:** definidos pela Wave 0.
**Acceptance criteria:**
- [ ] `go test ./internal/commands/ -count=1` verde com PATH sem trackfw
- [ ] contra-braço por AC

**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ -count=1
go test ./internal/commands/ -count=1
```

## Wave 2 — Red team (hades-tf)
> Dependencies: Wave 1 auditada.

### ML-2A — Red team (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-02-redteam-relatos-primeiro-usuario-externo.md`
**Actions:**
1. Reimplementar, a partir da leitura, os alvos da Wave 0 contra o diff.
2. Refazer as reproduções dos cinco issues num repositório limpo com o binário da branch.
**Acceptance criteria:**
- [ ] Parecer com achados classificados e evidência

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-02-redteam-relatos-primeiro-usuario-externo.md
```
