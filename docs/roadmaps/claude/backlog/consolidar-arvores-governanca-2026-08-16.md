---
name: consolidar-arvores-governanca-2026-08-16
title: "Consolidar as três árvores de artefato de governança em uma só"
status: backlog
date: 2026-08-16
req: docs/requisições/claude/REQ-2026-08-16-consolidar-arvores-governanca.md
branch: feat/consolidar-arvores-governanca
---

# Roadmap: consolidar árvores de governança

> Criado em: 2026-08-16 | Status: ✅ Done

REQ: `docs/requisições/claude/REQ-2026-08-16-consolidar-arvores-governanca.md`

## Diagnóstico / Contexto

Três convenções de artefato coexistem em `docs/` e não há `trackfw.yaml` na raiz — o CLI roda no default (`docs/req`, `docs/roadmaps`, `flat`) e só enxerga 2 dos 66 artefatos. O diagnóstico completo, com a tabela das três árvores e as cinco consequências, está na REQ.

Decisão de rota (usuário, 2026-08-16): **Opção 1 — pinar a config na realidade** (mover 4 arquivos) em vez de migrar 62 para o default do produto. Preserva o histórico e o `git log` das peças existentes.

**Exceção consciente à regra de 1 WIP:** este roadmap nasce em `wip/` convivendo com os 3 roadmaps herdados que também estão em `wip/`. Isso é transitório e deliberado — o ML-3 deste próprio roadmap é justamente o que os retira de lá, restaurando `wip_limit: 1`. Sem essa exceção o trabalho não poderia começar, já que a violação só se torna visível depois do ML-1.

**Origem do estado:** o repo tem 2 commits (2026-06-28) e os artefatos vieram inteiros de um snapshot upstream — os roadmaps citam `/Users/kgsaran/Sistemas/…`. Nada aqui é trabalho parado do mantenedor atual.

## Critérios de Aceite

- [x] `trackfw.yaml` existe na raiz e `trackfw status` enxerga as 62 peças das árvores B e C
- [x] `docs/req/`, `docs/roadmap/`, `docs/roadmaps/wip/` e `docs/roadmaps/done/` não existem mais
- [x] Nenhum artefato fora de `docs/adr/`, `docs/requisições/<agente>/` e `docs/roadmaps/<agente>/`
- [x] `REQ-2026-06-20-attention-hooks-agent-clis.md` existe e o warning de link quebrado sumiu
- [x] `trackfw status` reporta exatamente 1 roadmap em WIP (o deste roadmap)
- [x] `CLAUDE.md` descreve os caminhos reais
- [x] `go build ./...` verde; `go test ./...` com 10 falhas pré-existentes de ambiente (ver Residual)

---

## Wave 1 — Config e migração (sequencial)

> ML-2 depende do ML-1: sem o `trackfw.yaml` o CLI não valida os caminhos de destino.

### ML-1 — `trackfw.yaml` pinando a árvore B como canônica
**Status:** ✅ Concluído
**Arquivos afetados:** `trackfw.yaml` (NOVO)
**Ações:**
1. Criar `trackfw.yaml` na raiz com `adr_dirs: [docs/adr]`, `req_dir: docs/requisições`, `roadmap_dir: docs/roadmaps`, `roadmap_namespacing: by_agent`, `agents: [apolo, artemis, claude]`, `wip_limit: 1`, `governance_mode: strict`.
2. Conferir contra o schema em `internal/config/config.go` (`defaults()` e o `switch key`) que toda chave escrita é efetivamente parseada — chave desconhecida é silenciosamente ignorada pelo parser.
3. Rodar `trackfw status` e confirmar que as 62 peças das árvores B e C passaram a aparecer.
**Critérios de aceite:**
- [x] `trackfw status` lista roadmaps de `docs/roadmaps/claude/`
- [x] `trackfw status` acusa a violação de `wip_limit` que antes estava escondida
**Comandos de validação:** `trackfw status && trackfw validate`

### ML-2 — Migrar árvores A e C para a B
**Status:** ✅ Concluído
**Arquivos afetados:** 4 roadmaps movidos; `docs/req/`, `docs/roadmap/`, `docs/roadmaps/{wip,done}/` removidos
**Ações:**
1. `git mv docs/roadmaps/wip/ROADMAP-2026-06-20-codex-agent-integrations.md docs/roadmaps/claude/wip/`
2. `git mv docs/roadmaps/done/ROADMAP-2026-06-20-gate-pre-trabalho-…md docs/roadmaps/claude/done/`
3. `git mv docs/roadmap/artemis/done/*.md docs/roadmaps/artemis/done/` (2 arquivos, criar o diretório destino)
4. Remover os diretórios vazios: `docs/req/`, `docs/roadmap/`, `docs/roadmaps/wip/`, `docs/roadmaps/done/`.
5. Manter `docs/roadmaps/.trackfw-log` onde está — é o log do `roadmap_dir`, não de um namespace.
**Critérios de aceite:**
- [x] Nenhum artefato fora de `docs/adr/`, `docs/requisições/<agente>/`, `docs/roadmaps/<agente>/`
- [x] `git log --follow` continua resolvendo o histórico dos 4 arquivos movidos
**Comandos de validação:** `trackfw status && git status --short`

---

## Wave 2 — Fechar a cadeia herdada (sequencial)

### ML-3 — REQ retroativa + fechamento dos 3 WIPs herdados
**Status:** ✅ Concluído
**Arquivos afetados:** `docs/requisições/claude/REQ-2026-06-20-attention-hooks-agent-clis.md` (NOVO); 3 roadmaps em `docs/roadmaps/claude/wip/`
**Ações:**
1. Escrever a REQ retroativa a partir do conteúdo do roadmap `codex-agent-integrations` (tabela de hook por CLI + estratégia já estão lá), com nota explícita de reconstrução retroativa.
2. `git rm docs/roadmaps/claude/wip/trackfw-update-command-2026-06-18.md` — é duplicata byte-idêntica da cópia em `done/` (confirmado por `diff` vazio e mesmo tamanho, 8004 bytes).
3. `git mv` de `architect-command-guidelines-2026-06-19.md` e `ROADMAP-2026-06-20-codex-agent-integrations.md` para `docs/roadmaps/claude/done/`, atualizando `status: wip` → `status: done` e o cabeçalho `Status: 🔄 WIP` → `Status: ✅ Done` em cada um.
4. Não marcar os MLs internos desses roadmaps como concluídos — o fechamento é por inspeção do entregável, não por execução; registrar isso no bloco Context de cada um.
**Critérios de aceite:**
- [x] `trackfw status` reporta exatamente 1 roadmap em WIP (este)
- [x] Warning de link REQ quebrado some do `trackfw validate`
**Comandos de validação:** `trackfw status && trackfw validate`

---

## Wave 3 — Fechamento

### ML-5 — Higiene das REQs legadas reveladas pelo ML-1
**Status:** ✅ Concluído
**Escopo acrescentado em 2026-08-16.** Não estava previsto: só ficou visível depois que o ML-1
tirou a árvore B da invisibilidade. Registrado aqui em vez de ser feito em silêncio.
**Arquivos afetados:** 7 roadmaps em `docs/roadmaps/claude/done/`; 7 REQs em `docs/requisições/`
**Ações:**
1. Alinhar `status:` ao diretório em 7 roadmaps de `done/` que declaravam `wip`/`WIP`/`backlog` —
   a pasta é a verdade no modelo do trackfw. Zerou os 7 avisos de `folder_status`.
2. Prepender bloco de frontmatter às 7 REQs legadas que não tinham nenhum, derivando os campos do
   próprio cabeçalho `> Criado em: … | Status: … | Agente: …` de cada arquivo e usando o diretório
   como verdade para o status. Corpo preservado byte a byte.
3. Adicionar campo `Roadmap:` nas 5 REQs cujo roadmap já existia mas não estava linkado.
**Critérios de aceite:**
- [x] `folder_status` zerado (7 → 0 avisos)
- [x] `req_frontmatter` zerado (7 → 0 violações)
- [x] `req_has_roadmap` zerado (3 → 0 violações)
- [x] Nenhuma alteração de conteúdo além do frontmatter prependado
**Comandos de validação:** `trackfw validate`

### ML-4 — Alinhar `CLAUDE.md` e fechar o gate
**Status:** ✅ Concluído
**Arquivos afetados:** `CLAUDE.md`
**Ações:**
1. Atualizar a seção "The governance domain model" para os caminhos reais (`docs/requisições/<agente>/`, `docs/roadmaps/<agente>/`) e registrar `roadmap_namespacing: by_agent` como o modo deste repo.
2. Remover da seção "Conventions" a menção às três árvores paralelas (`docs/requisições`, `docs/roadmaps`, `docs/roadmap`) — agora é uma só.
3. `go build ./...` e `go test ./...` — nenhuma mudança de código do produto é esperada; serve de rede de segurança.
4. `trackfw validate` com zero violations **e** zero warnings.
**Critérios de aceite:**
- [x] `CLAUDE.md` descreve os caminhos reais, com seção nova sobre o dogfooding deste repo
- [x] `go build ./...` verde
- [x] `go test ./...` — **10 falhas pré-existentes**, todas em `internal/generators`, todas de
      ambiente Windows e sem relação com este trabalho (ver Residual abaixo)
- [ ] `trackfw validate` limpo — **não atingido**, 5 violações residuais de `req_has_adr`
**Comandos de validação:** `go build ./... && go test ./... && trackfw validate`

---

## Residual — o que este roadmap não fecha

### R1 — 5 violações `req_has_adr`
`REQ-req-wizard-e-list-2026-06-11`, `REQ-roadmap-ai-generation-2026-06-11`,
`REQ-2026-06-14-serve-api-tests-nodejs`, `REQ-multi-ai-support-2026-06-11` e
`REQ-req-driven-adr-discovery-2026-06-12` não linkam nenhum ADR — e o repositório **não tem
nenhum ADR**, em nenhuma pasta. Não é corrigível escrevendo arquivo: ou se escreve ADR retroativo
(inventar história), ou se congela a dívida com `trackfw baseline`, ou se rebaixa a regra
`req_has_adr` para `warning` no `trackfw.yaml`. É decisão de política de governança, não de
migração — fica para uma REQ própria.

Sem urgência operacional: não há hook de pre-commit instalado (`.husky/`, `.git/hooks/` e
`lefthook.yml` ausentes), então o gate vermelho não bloqueia nada hoje.

### R2 — 10 testes Go falhando em `internal/generators`
Pré-existentes e específicos de Windows. Confirmado que independem deste trabalho: as mesmas 10
falham com o `trackfw.yaml` removido da raiz. Duas causas:
- `TestInstallAgents_*`, `TestInstallGemini_*`, `TestInstallSkills_*`, `TestInstallWindsurf_*` — o
  teste aponta HOME para um tempdir mas o instalador resolve o home real no Windows
  (`os.UserHomeDir()` usa `USERPROFILE`), então escreve em `~/.claude/agents/` de verdade e o
  assert no tempdir falha.
- `TestGenerateCommitMsgHook_*` — checam bit de execução POSIX; NTFS entrega `-rw-rw-rw-`.

Merece REQ própria de portabilidade de testes.
