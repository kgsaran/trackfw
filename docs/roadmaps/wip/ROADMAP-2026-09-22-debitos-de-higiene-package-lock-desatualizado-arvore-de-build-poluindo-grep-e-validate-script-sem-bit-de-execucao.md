---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-08-30-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md"
squad: "hades-tf, apolo-tf, zeus-tf"
---

# Roadmap: Débitos de higiene — `package-lock` desatualizado, árvore de build poluindo `grep`, e validate script sem bit de execução

> Created: 2026-09-22 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-30-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md

Reescrito em 2026-10-31 a partir da medição na `main` (11.2.3); ver a tabela de revisão da REQ. Itens
vivos: `release tag` não confere o `package-lock` (AC1); `InstallGates` sem `os.Chmod` (AC2); aviso de
`agent_models` e destino do relatório do `update harness` (AC3); documentação do #633 e notas de vault
(AC4). `GIT_TERMINAL_PROMPT` sai do escopo pela medição (REQ, Motivation).

## Acceptance Criteria
- [ ] AC1–AC5 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model dos débitos de higiene (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-31-wave0-debitos-de-higiene.md`
**Actions:**
1. Enumeração: todos os pontos que gravam script executável (`os.WriteFile` com modo de execução) e quais já aplicam `os.Chmod`; todos os arquivos que carregam a versão do projeto e quais o `release tag` confere. Lista fechada com o comando.
2. AC1: o que o `release tag` pode passar a recusar indevidamente (lockfile sem `packages[""]`, formato antigo); ordem das verificações.
3. AC2: `os.Chmod` sobre symlink ou arquivo que não é do usuário; Windows.
4. AC3: em que condição o aviso de `agent_models` aparece (valores, camadas de configuração, com e sem `--dry-run`); se o relatório no stderr é contrato documentado.
5. Alvos de falsificação e residual; prescrições numeradas.
**Acceptance criteria:**
- [ ] As cinco seções respondidas com evidência (comando e resultado)
- [ ] Prescrições escritas em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-wave0-debitos-de-higiene.md
grep -q 'Prescrições' docs/seguranca/2026-10-31-wave0-debitos-de-higiene.md
```

---

## Wave 1 — Implementação (ML-1A, ML-1B e ML-1C sequenciais: mesmo executor e `docs/cli-parity.md` em comum; ML-1D em paralelo, só documentação)
> Dependencies: Wave 0 auditada.

### ML-1A — `release tag` confere o `package-lock` (apolo-tf) — AC1
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/release.go`, `internal/commands/release_test.go`, `docs/cli-parity.md`
**Actions:**
1. `releaseVersionFiles` ganha `npm/package-lock.json` (raiz e `packages[""]`), com extrator próprio.
2. A ajuda do `release tag` diz o número real de arquivos.
3. Prescrições da Wave 0.
**Acceptance criteria:**
- [ ] AC1 da REQ; contra-braço: sem a entrada nova, o teste de lockfile divergente reprova
- [ ] pacote `internal/commands` verde

### ML-1B — `InstallGates` aplica o bit de execução (apolo-tf) — AC2
**Status:** ⬜ Pendente
**Files affected:** `internal/discover/discover.go`, `internal/discover/discover_test.go`
**Actions:**
1. Depois de gravar `scripts/trackfw-validate.sh`, aplicar o modo como os outros pontos de escrita (mesmo helper, se existir).
2. Teste: arquivo pré-existente com `0644` fica executável; no Windows, o teste pula.
**Acceptance criteria:**
- [ ] AC2 da REQ; contra-braço: sem o `Chmod`, o teste reprova
- [ ] pacote `internal/discover` verde

### ML-1C — `agent_models` e o relatório do `update harness` (apolo-tf) — AC3
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/agents_models.go`, `internal/commands/update_harness.go`, `internal/commands/update_harness_test.go`, `docs/cli-parity.md`
**Actions:**
1. Reproduzir o aviso de 2026-10-30 a partir da condição medida na Wave 0; corrigir na causa ou registrar, com a medição, que não aparece mais.
2. Conferir o destino do relatório em texto contra `docs/cli-parity.md`; mover para o stdout só se não for contrato.
**Acceptance criteria:**
- [ ] AC3 da REQ
- [ ] pacote `internal/commands` verde

### ML-1D — Documentação do #633 e notas de vault (zeus-tf) — AC4
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md`, `vault/notes/index.md`, `vault/notes/fixture-follow-remote-head-never-neutraliza-a-forge-2026-10-31.md`, `vault/notes/git-terminal-prompt-sem-terminal-falha-na-hora-2026-10-31.md`
**Actions:**
1. Nota do arquiteto no fim do parecer: o R-5 foi corrigido no ML-1A do roadmap do #633.
2. As duas notas de vault, linkadas no índice.
**Acceptance criteria:**
- [ ] AC4 da REQ

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/ ./internal/discover/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ ./internal/discover/ -count=1
```

---

## Wave 2 — Red team
> Dependencies: Wave 1 auditada.

### ML-2A — Red team dos débitos de higiene (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-31-redteam-debitos-de-higiene.md`
**Actions:**
1. Tentar fazer o `release tag` aceitar lockfile divergente e o `discover --init` deixar o script sem bit.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [ ] cada prescrição com veredito e evidência
- [ ] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-redteam-debitos-de-higiene.md
make quality
```
