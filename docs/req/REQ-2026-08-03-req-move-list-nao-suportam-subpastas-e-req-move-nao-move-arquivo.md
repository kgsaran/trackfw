---
status: Done
date: 2026-08-03
author: "Zeus"
adr: "docs/adr/ADR-2026-08-04-req-move-list-reusar-roadmap-namespacing-para-req-e-mover-fisicamente-o-arquivo.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md"
---

# REQ: req move/list não enxergam REQDir com subpastas, e req move não move o arquivo

> Date: 2026-08-03 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Sessão real no projeto CMDB (`trackfw.yaml`: `req_dir: docs/requisições`,
`roadmap_namespacing: by_agent`) expôs dois defeitos em `trackfw req move`/`trackfw req list` ao tentar
promover uma REQ de `backlog` para `wip` e depois para `done`. Os dois comandos falharam repetidamente com
`REQ "..." not found in docs/requisições`, mesmo com o arquivo existindo em
`docs/requisições/claude/backlog/REQ-2026-07-30-....md` — forçando edição manual do frontmatter + `git mv`
em substituição ao comando, 4 vezes na mesma sessão.

### Causa raiz 1 — `findREQ`/`ListREQs` não descem em subpastas

`internal/generators/req.go`:

- `ListREQs` (linha ~118): `filepath.Glob(filepath.Join(dir, "*.md"))` — glob de um nível só sobre
  `cfg.REQDir`.
- `findREQ` (linha 265, usada por `MoveREQ`): `os.ReadDir(dir)`, também de um nível só, e
  **explicitamente pula diretórios** (`if e.IsDir() { continue }`).

Nenhum dos dois desce em `REQDir/<agente>/<estado>/` (namespacing `by_agent`) nem em
`REQDir/<estado>/` (REQs organizadas por estado sem namespacing por agente — layout que o próprio
projeto reconhece como válido: `internal/validator/validator_traceid_test.go:79-82` cria fixtures em
`docs/req/done/REQ-001-done.md` e o validador as processa normalmente).

Resultado: em qualquer projeto que organize REQs em subpastas — por agente ou por estado —, `req
list` sempre reporta `No REQs found` e `req move` sempre falha com `not found`, mesmo que
`trackfw context`/`trackfw validate` (que usam um scanner diferente, recursivo) enxerguem os mesmos
arquivos perfeitamente. A discrepância entre comandos é o que tornou o defeito difícil de diagnosticar em
campo — cada comando do trackfw parecia ter uma visão diferente do mesmo `docs/requisições`.

### Causa raiz 2 — `MoveREQ` nunca move o arquivo, só reescreve `status:`

`internal/generators/req.go`, `MoveREQ` (linha ~241): lê o arquivo, chama `rewriteREQStatus`, e grava de
volta **no mesmo caminho** (`os.WriteFile(path, updated, 0644)`). Não há `os.Rename`/`os.MkdirAll` para
nenhum diretório de estado-alvo. Confirmado como comportamento **intencional e testado** —
`TestMoveREQ_RewritesStatusInPlace` (`internal/generators/req_test.go:268`) nomeia essa exata semântica.

Isso diverge de `MoveRoadmap` (`internal/generators/roadmap.go:337`), que:

1. Resolve o diretório-alvo considerando `RoadmapNamespacing` (`agentStateDir`/`stateDir`).
2. Fisicamente move o arquivo via `os.Rename` (linha ~373).
3. Sincroniza o `status:` no destino.
4. Registra a transição em `.trackfw-log`.
5. Sincroniza a referência `roadmap:` em toda REQ pareada (`syncREQReferences`).

`MoveREQ` não faz nenhum dos 5 passos além do #3. O resultado prático, em qualquer projeto cujo modelo
de governança declare "pasta é a fonte de verdade do estado" (o próprio README/docs do trackfw promove
esse princípio para roadmaps) — REQs nunca conseguem seguir esse princípio via CLI: `trackfw req move
<nome> done` deixa o arquivo fisicamente em `backlog/` com `status: done` no frontmatter, uma
divergência pasta-vs-status que o `validate`/gate depois reporta como erro (regra R8 do Kanban
Governance Gate, testemunhada na mesma sessão) — o próprio comando que deveria manter a consistência é
quem a quebra.

### Por que isso importa

- `config.RoadmapNamespacing` existe (`internal/config/config_namespacing_test.go:29`); não há campo
  equivalente para REQ — o schema de config não modela namespacing de REQ, mas o `req_dir` é o mesmo
  `trackfw.yaml` que já declara `roadmap_namespacing: by_agent`, e nada impede (nem a documentação
  avisa) um usuário organizar REQs no mesmo padrão do roadmap — é o que aconteceu no CMDB.
- Mesmo sem namespacing por agente, layout `REQDir/<estado>/` (testado no validador) já quebra `req
  list`/`req move` hoje.
- O único workaround funcional é edição manual + `git mv`, que é exatamente o tipo de trabalho que o
  trackfw existe para eliminar.

## Acceptance Criteria

- [ ] **AC1 — `req list` recursivo.** `trackfw req list` encontra REQs em `REQDir/*.md` (layout flat
      atual), `REQDir/<estado>/*.md` (por-estado) e `REQDir/<agente>/<estado>/*.md` (by_agent) sem
      flag adicional — mesmo alcance que `trackfw context` já tem hoje para REQs.
- [ ] **AC2 — `req move` encontra REQs em subpastas.** `findREQ` ganha um modo de busca recursiva
      (ou reaproveita a lógica de `findRoadmap`, adaptada), nos três layouts do AC1.
- [ ] **AC3 — `req move` move fisicamente o arquivo.** Ao mudar o estado, o arquivo é relocado para
      `REQDir/<estado>/` (flat) ou `REQDir/<agente>/<estado>/` (by_agent) via rename real — não apenas
      reescrita de `status:` no lugar. Mantém compatibilidade: se o projeto usa REQs soltas direto em
      `REQDir/` sem subpastas de estado (layout legado, sem diretórios de estado), `req move` continua
      reescrevendo `status:` in place e não cria estrutura de pastas nova por conta própria — só passa a
      mover fisicamente quando o projeto já tem `REQDir/<estado>/` (ou `<agente>/<estado>/`) como
      estrutura existente. Critério de decisão análogo ao que `findRoadmap`/`MoveRoadmap` já usam.
- [ ] **AC4 — Sincronização paralela ao roadmap.** Ao mover um REQ, `roadmap:` no REQ movido não muda
      de valor (isso é responsabilidade de quem move o roadmap, já coberto por `syncREQReferences`) —
      mas o transition log (`.trackfw-log`) registra a transição do REQ, no mesmo formato usado por
      `MoveRoadmap`.
- [ ] **AC5 — Paridade nos 3 CLIs.** Node (`npm/src/`) e Python (`pypi/trackfw/`) recebem a mesma
      correção, com o mesmo comportamento observável — provado executando os três binários contra o
      mesmo fixture (mesmo padrão de prova usado em REQs anteriores de paridade, ex.
      `REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-...md` AC3).
- [ ] **AC6 — Testes de regressão.** Fixture com REQ em `docs/req/backlog/` (por-estado, sem agente) e
      outra em `docs/req/claude/backlog/` (by_agent) — `req list` e `req move` funcionam nos dois.
      `TestMoveREQ_RewritesStatusInPlace` é preservado ou explicitamente substituído/ajustado para
      cobrir o novo comportamento de move físico, sem remover cobertura do modo legado in-place.
- [ ] **AC7 — Documentação.** README/docs do trackfw esclarecem se REQs suportam
      `roadmap_namespacing: by_agent` (implícito hoje, nunca declarado) e o comportamento de `req move`
      quanto a mover ou não o arquivo físico.

## Negative Scope — o que esta REQ NÃO faz

- **Não introduz um campo `req_namespacing` novo no schema de config.** Se a correção decidir que REQ
  deve reusar `roadmap_namespacing` (mesmo valor, sem campo próprio), documentar essa decisão; se
  decidir criar campo próprio, é decisão do time/ADR sucessor — esta REQ só levanta o problema.
- **Não altera `MoveRoadmap`/`findRoadmap`**, que já funcionam corretamente e servem de referência de
  design.
- **Não resolve a divergência entre `trackfw context` (que já enxerga REQs em subpastas) e os demais
  comandos** além de trazer `req list`/`req move` à paridade com `context` — não propõe unificar os
  scanners internos além do necessário para essa paridade.

## Linked ADR

ADR: `ADR-2026-08-04-req-move-list-reusar-roadmap-namespacing-para-req-e-mover-fisicamente-o-arquivo` —
decide reusar `roadmap_namespacing` (sem campo `req_namespacing` novo) e define o critério de move
físico condicional (só quando `REQDir/<estado>/` ou `<agente>/<estado>/` já existe como estrutura
corrente).

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md`
