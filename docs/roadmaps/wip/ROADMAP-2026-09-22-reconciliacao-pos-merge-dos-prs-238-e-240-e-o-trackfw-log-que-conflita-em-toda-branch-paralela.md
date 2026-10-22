---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md"
squad: "hades-tf, apolo-tf, ares-tf, prometeu-tf, hefesto-tf"
---

# Roadmap: Reconciliação pós-merge dos PRs #238 e #240, e o `.trackfw-log` que conflita em toda branch paralela

> Created: 2026-09-22 | Status: wip | Reescrito contra a main 75642ec5 em 2026-10-22

## Context
REQ: docs/req/REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md
ADR: docs/adr/ADR-2026-10-22-logs-append-only-compartilhados-viram-um-arquivo-por-entrada-porque-o-github-ignora-merge-union.md

### Medido em 2026-10-22

- O `merge=union` está em `.gitattributes:6` (#251) e é gerado por `generateGitAttributes` (`scaffold.go:2052-2102`).
  **O GitHub o ignora:** no #595, ele acusou conflito no `docs/roadmaps/.trackfw-log`, e o `git merge origin/main`
  local resolveu sozinho. O `docs/agents-working-context.md` (47.413 linhas) conflitou no #595 e no #597.
- Escritores do log: `generators/roadmap.go:264` e `generators/req.go:525` (o de REQ vai para `<req_dir>/.trackfw-log`).
  Leitores: `metrics/metrics.go:47`, `serve/metrics_log.go:25`, `serve/api_metrics.go:40`, `validator/validator.go:2873`,
  `commands/log.go:27`, `commands/metrics.go:20`, `discover/discover.go:543,710` e `commands/discover.go`.
  São **dois parsers duplicados** (metrics e serve).
- O contexto de trabalho é instruído em `generators/agentfiles.go:92-93` e na seção `## Working context` dos 12 assets em
  `internal/integrations/assets/agents/`. Os goldens `architect.*.golden.md` e `backend.agent-directory.golden.md` acompanham.
- AC1–AC4 (triagem de 2026-10-17) continuam vivos: `check-roadmap-barrier-contract.sh:612` (`errors="replace"`) e
  `check-output-encoding-declared.sh:113-127` (allowlist obsoleta) e `:163-168` (o `DECL_RE` não enxerga `sys.stdout.reconfigure`).
- Não há issue aberta sobre o mecanismo (varredura de 2026-10-22).

## Acceptance Criteria
- [ ] AC1–AC4 da REQ (gates de encoding), na Wave 1, ML-1B.
- [x] AC5: política decidida pelo KG (o mantenedor move depois do merge); os roadmaps do #238 e do #240 já estão em done.
- [ ] AC6 (reescrito): log com um arquivo por entrada, lido por um ponto único, com o merge sem conflito provado sem `.gitattributes`.
- [ ] AC7: as entradas dos dois lados sobrevivem, por igualdade de conjunto.
- [ ] AC8: o contexto de trabalho grava um arquivo por entrada, e as instruções geradas acompanham.
- [ ] `make quality` e o CI ficam verdes.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model e enumeração completa
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-22-wave0-log-por-entrada.md` (novo, único arquivo)
**Actions:**
1. Enumerar todo escritor e leitor dos dois arquivos, além do grep em `internal/*.go`: `scripts/` (a falsificação e os contratos),
   `docs/cli-parity.md`, os testes que fixam o arquivo único, `discover --bootstrap-log`, o `serve` e os goldens.
2. Listar todo código que percorre `roadmap_dir`/`req_dir` e diria o que faz com um diretório `.trackfw-log-entries/`: se ele vira
   estado, REQ ou lixo. Ponto de partida: `roadmap.go:1040-1046`.
3. O corpus pinado `internal/roadmapdoc/testdata/corpus` e os snapshots em `scripts/testdata/` têm `.trackfw-log`? A leitura do
   legado mantém o `CORPUS_HASH` válido? O escopo negativo proíbe mexer no corpus.
4. Nome do arquivo (ADR D1/D2): colisão no mesmo segundo, Windows (caracteres e tamanho de caminho), symlink no diretório,
   arquivo malformado ou de terceiro dentro do `.d/`, e ordenação com fuso.
5. Contexto de trabalho: recomendar o caminho de leitura (as N últimas entradas na instrução, ou um subcomando) com base no que já
   existe (`trackfw context`).
6. Alvos de falsificação nas duas direções e o resíduo, incluindo a janela de transição das instruções já instaladas.
**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (arquivo:linha ou comando e saída)
- [x] A lista final de arquivos de cada ML da Wave 1 e da Wave 2
- [x] Nenhuma linha de implementação

**Auditoria do arquiteto (2026-10-22):** aprovada, com os três ajustes incorporados ao adendo da ADR: extensão `.entry` em vez de negação no `.gitignore`, `isInfraDirName` e D2 sem hash. A lista de arquivos de cada ML está na seção "Lista de arquivos" do parecer, e os handoffs a seguem.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-22-wave0-log-por-entrada.md
```

## Wave 1 — Implementação do log e dos gates de encoding (2 MLs em paralelo, arquivos disjuntos)
> Dependencies: Wave 0 auditada.

### ML-1A — Log de transições com um arquivo por entrada (ADR D1, AC6, AC7)
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** escritores (`generators/roadmap.go`, `generators/req.go`), um pacote leitor único, todos os leitores listados no
Context, `generators/scaffold.go` (o comentário do `gitAttributesBlock`), `docs/cli-parity.md` e os testes. A lista final vem da Wave 0.
**Acceptance criteria:**
- [x] Uma transição grava `<dir>/.trackfw-log-entries/<AAAAMMDDTHHMMSSZ>-<h8>.entry` com uma linha. Não há mais append no arquivo.
- [x] Sobra um único parser. Todos os leitores leem o legado e o diretório. Nenhum leitor lê só o arquivo, e um teste prova isso.
- [x] Teste em `t.TempDir()` **sem `.gitattributes`**: duas branches movem roadmaps pelo código real, e o `git merge` sai limpo, com
      igualdade de conjunto das entradas. O braço contrário, com o formato antigo, conflita no mesmo teste.
- [x] A travessia de `roadmap_dir`/`req_dir` ignora `.trackfw-log-entries/`.
**Auditoria do arquiteto:** a 1ª entrega foi reprovada. O `TestDiscoverBootstrapLog` reprovava, porque `internal/commands` não foi rodado; o `check-write-containment` reprovava no `translog`; faltavam o teste do `by_agent` e a mordida. No corretivo: os 7 pacotes ficaram verdes; `wc=0`, com os 3 chamadores conferidos passando pelo `pathguard` antes do `WriteEntry`; e as 3 mordidas reprovam como esperado (`CONFLICT (add/add)` no append legado). Depois, as anotações de contrato no `cli-parity.md` e os pins B1/B3 de `check-agent-namespace-union.sh` foram atualizados para o `isInfraDirName` novo. `make parity-rest` com rc=0, medido pelo arquiteto.

### ML-1B — Gates de encoding (AC1–AC4 da REQ)
**Status:** ✅ Concluído
**Agente:** ares-tf
**Files affected:** `scripts/check-roadmap-barrier-contract.sh`, `scripts/check-output-encoding-declared.sh` e o autoteste/fixture deles
**Acceptance criteria:**
- [x] AC1: `errors="replace"` vira `strict`, ou o motivo fica escrito com medição. Falsificação com surrogate solto.
- [x] AC2/AC3: a allowlist é removida ou reescrita, e o aviso de obsoleta passa a reconhecer `sys.stdout.reconfigure`. Falsificado
      nas duas direções.
- [x] AC4: exposição a CRLF medida. Zero encerra o item.
- [x] Para script novo ou alterado, `make parity-rest` no handoff.

**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ ./internal/metrics/ ./internal/serve/ ./internal/validator/ ./internal/discover/ ./internal/commands/
```
**Auditoria do arquiteto:** `strict` com medição de surrogate (o `CORPUS_HASH` não mudou), allowlist vazia, `RECONFIGURE_RE` só no aviso de allowlist obsoleta (estreitamento justificado: o `reconfigure` cobre só um bloco), AC4 = 0 de 10 sítios expostos. O `--self-test` foi ligado ao `parity-rest` no Makefile, num ajuste posterior.

### ML-1C — Diretório sem sufixo `.d`
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Origem:** o arquiteto deixou passar o achado B2 da Wave 0 (`*.d` no `.gitignore` ignora o diretório; o `C++.gitignore` do GitHub abre com `*.d`).
**Acceptance criteria:**
- [x] `.trackfw-log.d` virou `.trackfw-log-entries`, com a constante única `translog.EntriesDirSuffix`.
- [x] `TestEntriesDirNotIgnoredByCommonGitignorePatterns`: com `*.d`, `*.log`, `*.tmp`, `*.bak` e `*.swp`, o caminho novo não é ignorado; o antigo é ignorado no braço contrário.
- [x] Junto, foi corrigido `TestReadAll_DotDSymlinkRefused`, que usava `os.Symlink` com `t.Skip` (proibido pelo `check-symlink-privilege-guard`). Medido pelo arquiteto: os 7 pacotes verdes e `spg=0`.

## Wave 2 — Contexto de trabalho com um arquivo por entrada (ADR D2, AC8)
> Dependencies: Wave 1 auditada. É sequencial ao ML-1A porque compartilha o pacote `internal/generators`.

### ML-2A — Instruções geradas e o arquivo legado
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Files affected:** `internal/generators/agentfiles.go`, os 12 `internal/integrations/assets/agents/*.md`, os goldens, o cabeçalho de
`docs/agents-working-context.md` e o `CLAUDE.md` do projeto (as linhas que mandam ler e atualizar o contexto)
**Acceptance criteria:**
- [x] Toda instrução gerada manda escrever um arquivo em `docs/agents-working-context/` e ler as entradas mais recentes. Um teste
      sobre todas as representações do catálogo tem braço negativo (instrução antiga reprova).
- [x] O `.md` legado ganha o cabeçalho de redirecionamento e não recebe mais entradas.
- [x] Os goldens acompanham, e a mudança aparece no diff.

## Wave 3 — Revisão
> Dependencies: Wave 2 auditada.

### ML-3A — Qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/seguranca/2026-10-22-quality-log-por-entrada.md`

### ML-3B — Red team
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-22-red-team-log-por-entrada.md`

## Wave 4 — Fechamento
> Dependencies: Wave 3 aprovada.

### ML-4A — make quality, CI e a transição
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Acceptance criteria:**
- [ ] `make quality` com rc=0, com a máquina ociosa e o HOME isolado
- [ ] CI do PR verde (o PR abre como draft até aqui)
- [ ] Lista, para o KG, das linhas do `~/.claude/CLAUDE.md` e dos agentes instalados que mudam depois do `agents update`

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Mudou de forma**: `merge=union` feito (#251) e roadmaps em done; vivos AC1–AC4 — `errors="replace"` sem decisão, allowlist obsoleta e `DECL_RE` cego a `sys.stdout.reconfigure` (o gate diz OK).

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
