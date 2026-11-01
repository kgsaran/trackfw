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
- [x] AC1–AC6 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model dos débitos de higiene (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-31. P-1 a P-3 aceitas. AC1: o lockfile tem dois campos de versão (raiz e `packages[""]`); o extrator confere os dois. AC2: `writeValidateScript` sobrescreve sempre, sem `Chmod`; a enumeração achou os dois hooks do husky com a mesma causa (AC2b, decisão do arquiteto pela Regra Dura); lefthook e `O_APPEND` ficam fora, justificados. AC3: o aviso reproduzido é a mensagem de `agent_models` global ausente, emitida uma vez por destino do catálogo em `harnessCatalogTarget` (`internal/generators/update.go`), não o `WARN:` de `agents_models.go`; o relatório em texto já vai para o stdout. Decisão do arquiteto no R-5: emitir uma vez por execução, só com destino do catálogo presente.
**Files affected:** `docs/seguranca/2026-10-31-wave0-debitos-de-higiene.md`, `vault/notes/update-harness-advisory-por-catalog-target-nao-warn-2026-10-31.md`, `vault/notes/index.md`
**Actions:**
1. Enumeração: todos os pontos que gravam script executável (`os.WriteFile` com modo de execução) e quais já aplicam `os.Chmod`; todos os arquivos que carregam a versão do projeto e quais o `release tag` confere. Lista fechada com o comando.
2. AC1: o que o `release tag` pode passar a recusar indevidamente (lockfile sem `packages[""]`, formato antigo); ordem das verificações.
3. AC2: `os.Chmod` sobre symlink ou arquivo que não é do usuário; Windows.
4. AC3: em que condição o aviso de `agent_models` aparece (valores, camadas de configuração, com e sem `--dry-run`); se o relatório no stderr é contrato documentado.
5. Alvos de falsificação e residual; prescrições numeradas.
**Acceptance criteria:**
- [x] As cinco seções respondidas com evidência (comando e resultado)
- [x] Prescrições escritas em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-wave0-debitos-de-higiene.md
grep -q 'Prescrições' docs/seguranca/2026-10-31-wave0-debitos-de-higiene.md
```

---

## Wave 1 — Implementação (ML-1A, ML-1B e ML-1C sequenciais: mesmo executor e `docs/cli-parity.md` em comum; ML-1D em paralelo, só documentação)
> Dependencies: Wave 0 auditada.

### ML-1A — `release tag` confere o `package-lock` (apolo-tf) — AC1
**Status:** ✅ Concluído — auditado em 2026-10-31. Duas entradas novas em `releaseVersionFiles` (raiz e `packages[""]` do lockfile); lockfile sem `packages[""]` falha fechado. São 4 arquivos e 5 conferências (corrigido depois do red team; o texto anterior desta nota dizia 5 e 7). A fixture de `scripts/check-release-tag-parity.sh` ganhou o lockfile (arquivo acrescentado ao escopo); o script passa (rodado pelo arquiteto). 3 subtestes novos conferidos pelo nome; contra-braço: sem as entradas, reprovam.
**Files affected:** `internal/commands/release.go`, `internal/commands/release_test.go`, `scripts/check-release-tag-parity.sh`, `docs/cli-parity.md`
**Actions:**
1. `releaseVersionFiles` ganha `npm/package-lock.json` (raiz e `packages[""]`), com extrator próprio.
2. A ajuda do `release tag` diz o número real de arquivos.
3. Prescrições da Wave 0.
**Acceptance criteria:**
- [x] AC1 da REQ; contra-braço: sem a entrada nova, o teste de lockfile divergente reprova
- [x] pacote `internal/commands` verde

### ML-1B — `InstallGates` e hooks do husky aplicam o bit de execução (apolo-tf) — AC2, AC2b
**Status:** ✅ Concluído — auditado em 2026-10-31. `os.Chmod(…, 0755)` depois da escrita em `writeValidateScript` e nos dois hooks do husky. 3 testes novos conferidos pelo nome; contra-braço por sítio: sem o `Chmod`, o teste correspondente vê `0644`.
**Files affected:** `internal/discover/discover.go`, `internal/discover/discover_test.go`, `internal/generators/scaffold.go`, `internal/generators/scaffold_test.go`
**Actions:**
1. Depois de gravar `scripts/trackfw-validate.sh`, aplicar o modo como os outros pontos de escrita (mesmo helper, se existir).
2. O mesmo nos dois hooks do husky em `scaffold.go` (AC2b).
3. Testes: arquivo pré-existente com `0644` fica executável, nos três sítios; no Windows, os testes pulam.
**Acceptance criteria:**
- [x] AC2 e AC2b da REQ; contra-braço: sem o `Chmod`, os testes reprovam
- [x] pacotes `internal/discover` e `internal/generators` verdes

### ML-1C — `agent_models` e o relatório do `update harness` (apolo-tf) — AC3
**Status:** ✅ Concluído — auditado em 2026-10-31. Pré-varredura dos ids em `UpdateHarness`: com destino do catálogo, `ResolveAgentModels` roda uma vez e a mensagem sai uma vez por `UpdateOptions.ErrWriter` (ligado a `cmd.ErrOrStderr()`); `harnessCatalogTarget` recebe os modelos por parâmetro. 2 testes novos conferidos pelo nome. Contra-braços: sem a pré-varredura, o de N destinos vê 0 mensagens; forçando a varredura verdadeira (medido pelo arquiteto, a mutação do executor não compilava), o de destinos fora do catálogo reprova.
**Files affected:** `internal/generators/update.go`, `internal/generators/update_test.go`, `internal/commands/update_harness.go`, `internal/commands/update_harness_test.go`, `docs/cli-parity.md`
**Actions:**
1. A mensagem de `agent_models` global sai uma vez por execução do `update harness`, antes do laço, só se houver destino do catálogo (pré-varredura dos ids, P-3 do parecer); `harnessCatalogTarget` recebe os modelos resolvidos por parâmetro.
2. `UpdateOptions` ganha escritor de erro, ligado ao `cmd.ErrOrStderr()` no comando, para o teste capturar a mensagem.
3. Testes: N destinos do catálogo → 1 mensagem; só destinos fora do catálogo → 0.
**Acceptance criteria:**
- [x] AC3 da REQ
- [x] pacote `internal/commands` verde

### ML-1D — Documentação do #633 e notas de vault (zeus-tf) — AC4
**Status:** ✅ Concluído — 2026-10-31. Nota do arquiteto no fim do parecer do #633 (R-5 corrigido no ML-1A do #633; P-5 sem issue, pela medição). Notas de vault `git-terminal-prompt-sem-terminal-falha-na-hora-2026-10-31` e `fixture-follow-remote-head-never-neutraliza-a-forge-2026-10-31`, linkadas no índice; nomes de função e de teste citados conferidos no código.
**Files affected:** `docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md`, `vault/notes/index.md`, `vault/notes/fixture-follow-remote-head-never-neutraliza-a-forge-2026-10-31.md`, `vault/notes/git-terminal-prompt-sem-terminal-falha-na-hora-2026-10-31.md`
**Actions:**
1. Nota do arquiteto no fim do parecer: o R-5 foi corrigido no ML-1A do roadmap do #633.
2. As duas notas de vault, linkadas no índice.
**Acceptance criteria:**
- [x] AC4 da REQ

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/ ./internal/discover/ ./internal/generators/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ ./internal/discover/ ./internal/generators/ -count=1
```

---

## Wave 2 — Red team
> Dependencies: Wave 1 auditada.

### ML-2A — Red team dos débitos de higiene (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-31. Aprova com ressalvas; P-1 a P-3 cumpridas. Quatro achados LOW e o residual R-4 viram a Wave 3 (defeito conhecido se corrige no mesmo PR): A-1 `cli-parity.md` ainda lista `pypi/trackfw/__init__.py` e diz 5 arquivos / 7 conferências (são 4 e 5; a nota de auditoria do ML-1A repetiu o número errado do relatório sem conferir, erro do arquiteto); A-2 extratores JSON aceitam `Version` por correspondência sem caixa; A-3 teste de 0 mensagens só procura uma das mensagens; A-4 nenhum teste prova que os modelos resolvidos chegam aos agentes gerados (mutação com `nil` sobrevive); R-4 `WriteFile`+`Chmod` seguem symlink trocado na corrida (Chmod pelo symlink em 171 de 5000 iterações) e hardlink.
**Files affected:** `docs/seguranca/2026-10-31-redteam-debitos-de-higiene.md`
**Actions:**
1. Tentar fazer o `release tag` aceitar lockfile divergente e o `discover --init` deixar o script sem bit.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [x] cada prescrição com veredito e evidência
- [x] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-redteam-debitos-de-higiene.md
```

---

## Wave 3 — Corretivos do red team (1 ML)
> Dependencies: Wave 2 auditada.

### ML-3A — Achados A-1 a A-4 e R-4 do red team (apolo-tf) — AC6
**Status:** ✅ Concluído — auditado em 2026-10-31. A-1: `cli-parity.md` e comentários do script com 4 arquivos e 5 conferências. A-2: extratores JSON por `map[string]json.RawMessage` e chave exata. A-3: stderr vazio exigido, e o cenário de configuração só do projeto emite uma mensagem. A-4: teste prova que o `model:` resolvido chega ao agente gerado (com `nil`, reprova). R-4: os três pontos de escrita usam `pathguard.GuardedWrite`; contra-braço medido pelo arquiteto: com `os.WriteFile` de volta em `discover.go`, os testes de symlink e de hardlink reprovam. 9 testes novos conferidos pelo nome em `-v`; `check-release-tag-parity.sh` e `check-write-containment.sh` verdes (executor).
**Files affected:** `docs/cli-parity.md`, `scripts/check-release-tag-parity.sh`, `internal/commands/release.go`, `internal/commands/release_test.go`, `internal/generators/update_test.go`, `internal/discover/discover.go`, `internal/discover/discover_test.go`, `internal/generators/scaffold.go`, `internal/generators/scaffold_test.go`
**Actions:**
1. A-1: `cli-parity.md` (pré-condição 3 do `release tag`) e comentários do script descrevem 4 arquivos e 5 conferências.
2. A-2: extratores de versão de JSON (`package.json` e os dois do lockfile) leem a chave exata.
3. A-3: o teste de destinos fora do catálogo exige stderr vazio.
4. A-4: teste que prova que os modelos resolvidos chegam aos agentes gerados.
5. R-4: os três pontos de escrita do AC2/AC2b passam a `pathguard.GuardedWrite`, mantendo os testes do bit de execução.
**Acceptance criteria:**
- [x] AC6 da REQ; contra-braço por item (A-2 com chave `Version` diferente; A-3 com a mensagem de configuração só do projeto; A-4 com `nil`; R-4 com symlink plantado no lugar do arquivo)
- [x] pacotes `internal/commands`, `internal/discover`, `internal/generators` e `internal/pathguard` verdes

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/ ./internal/discover/ ./internal/generators/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ ./internal/discover/ ./internal/generators/ ./internal/pathguard/ -count=1
make quality
```
