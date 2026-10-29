---
status: wip
date: 2026-10-29
req: "docs/req/REQ-2026-10-29-barrier-ml-scope-le-a-declaracao-de-arquivos-afetados-em-formato-de-lista.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: barrier ml_scope lê a declaração de arquivos afetados em formato de lista

> Created: 2026-10-29 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-29-barrier-ml-scope-le-a-declaracao-de-arquivos-afetados-em-formato-de-lista.md
ADR: docs/adr/ADR-2026-10-29-barrier-ml-scope-le-a-declaracao-em-lista-e-une-os-roadmaps-tocados-pela-branch.md

Esta é a segunda REQ da branch `feat/documentacao-reflete-produto-11-windows-install-ps1`, e o mesmo PR (#627) leva as
duas. A primeira é a de documentação 11.x
(`docs/roadmaps/wip/ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md`).

**Dependências com o roadmap de documentação:**
- A Wave 1 daquele roadmap fica bloqueada no `ml_scope` até o ML-1A daqui terminar. As outras checagens já passaram:
  `gates: passed`, `validate: passed`.
- O ML-2B de lá reescreve `docs/cli-parity.md` inteiro, e o ML-1A daqui edita a seção `ml_scope` do mesmo arquivo. Por
  isso são **sequenciais**: o ML-2B só começa depois do ML-1A daqui commitado.

**Barrier com o próprio defeito:** enquanto o ML-1A não existir, o barrier deste roadmap também reprova o `ml_scope`,
porque a branch toca os arquivos do roadmap de documentação. Até lá, o veredito da Wave 0 se apoia em `gates` e
`validate` verdes mais auditoria manual do diff, e isso fica registrado. Depois do ML-1A, os barriers das duas REQs rodam
de novo com `bin/trackfw`, o binário da branch.

## Acceptance Criteria
- [ ] AC1 a AC9 da REQ atendidos, com evidência no ML que os entrega

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do ml_scope em lista e em união (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-29. 12 prescrições em §4.1 do parecer. Dois achados entraram na REQ como AC10 (glob sem segmento literal) e AC11 (`req:` e `adr:` sem checagem de prefixo; `req: "go.mod"` concedia `go.mod`, conferido no código). O `ml_scope` do barrier desta wave reprova pelo próprio defeito, como o Context já previa.
**Files affected:** `docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md`
**Actions:**
1. Completude da enumeração. Liste todo leitor de `Files affected` / `Arquivos afetados` (código Go, scripts,
   `docs/cli-parity.md`, testes). Feche a lista com o comando usado, sem `| head`.
2. Regra de crases por item. Escolha entre "só a primeira crase do item" e "todas as crases", com o caso real do ML-1B
   do roadmap de documentação: o item que diz que `pypi/pyproject.toml` **fica** e cita quatro consumidores. Decida
   também se sub-itens indentados contam.
3. Modelo de ameaça. Quem amplia o escopo sem quebrar regra escrita, e como? Inclua pelo menos:
   - glob largo (`**`, `*`, `/`) num roadmap qualquer da união;
   - roadmap novo commitado só para ampliar escopo;
   - roadmap tocado por um rename trivial;
   - roadmap em `abandoned/` ou `backlog/`;
   - lista que "vaza" para a lista de critérios de aceite.
4. Recorte de waves dos outros roadmaps da união. Hoje o alvo contribui com as waves 0..N; os outros, com todas, ou só
   com as waves que já têm ML iniciado? Decida com o caso desta branch.
5. Alvos de falsificação nas duas direções, para cada regra. O que quebra se regredir para "lista vazia", e o que quebra
   se regredir para "lista engole o resto da seção". O mesmo vale para a união: perder roadmap, e ganhar roadmap que não
   deveria contar.
6. Residual declarado.
**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (comando e resultado), não com afirmação de uma linha
- [x] Decisão de crases e de recorte de waves escrita como prescrição para o ML-1A
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md
grep -q 'Prescrições' docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Parser em lista e escopo por união (apolo-tf) — AC1 a AC11
**Status:** ✅ Concluído — auditado em 2026-10-29, depois de um corretivo. O contra-braço de T11/T12 fixava o contrato antigo (roadmap novo em `done/` fora de escopo) e agora usa um caminho apagado na branch. A mensagem de padrão largo passou a nomear roadmap, ML e token. A seção `ml_scope` do `cli-parity.md` foi escrita. Medido pelo arquiteto: `go test ./internal/commands/` rc=0, e os 23 nomes de MLScope conferidos em `-v`. `check-barrier.sh` rc=0, `check-parity-contract-coverage.sh` rc=0, `go test ./internal/pathguard/` rc=0. `bin/trackfw barrier` da Wave 1 da documentação 11.x: 6/6 passed.
**Files affected:** `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Actions:**
1. `parseMLScopeFilesAffected`: aceitar os itens de lista depois do rótulo, com a regra de crases prescrita pela Wave 0.
2. `checkMLScope`: montar o escopo como união dos roadmaps tocados pela branch (ADR-2026-10-29, decisão 2), com o
   recorte de waves prescrito pela Wave 0. A orquestração de cada roadmap da união entra no conjunto permitido.
3. Testes em `barrier_ml_scope_test.go` para AC1 a AC6. Os de repositório git usam `t.TempDir()`, nunca a árvore real.
4. `docs/cli-parity.md`: só a seção `ml_scope` (gramática e entradas), com os `trackfw-contract` apontando para os testes
   novos. Nenhuma outra seção.
5. Seguir as 12 prescrições de `docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md` §4.1. A p.10 diz que `scripts/check-barrier.sh` não muda.
**Acceptance criteria:**
- [x] AC1 a AC6 da REQ, cada um com o nome do teste conferido por `go test -run <nome> -v`
- [x] Sabotagem medida e revertida: voltar o `break` em `- ` reprova o teste de AC1; remover a união reprova o de AC4
- [x] AC8: `go build -o bin/trackfw ./cmd/trackfw` e `bin/trackfw barrier <roadmap de documentação> --wave 1
  --trust-local-gates` com `ml_scope: passed`
- [x] `go test ./internal/commands/` e `sh scripts/check-barrier.sh` verdes
- [x] Relatório com uma frase por teste novo, dizendo que conclusão do ML ele afirma

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -run 'MLScope' -count=1
```

## Wave 2 — Red team
> Dependencies: Wave 1 auditada.

### ML-2A — Red team do ml_scope (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-29-redteam-barrier-ml-scope.md`
**Actions:**
1. Reimplementar a regra a partir da leitura do contrato, sem conferir o diff, e confrontar com o binário da branch.
2. Tentar ampliar o escopo pelos vetores da Wave 0 seção 3, em repositório descartável.
**Acceptance criteria:**
- [ ] Parecer aprovado, ou corretivo despachado e reauditado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-29-redteam-barrier-ml-scope.md
```
