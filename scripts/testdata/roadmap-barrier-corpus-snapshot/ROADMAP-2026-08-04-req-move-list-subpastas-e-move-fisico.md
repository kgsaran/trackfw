---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-03-req-move-list-nao-suportam-subpastas-e-req-move-nao-move-arquivo.md"
squad: ""
---

# Roadmap: req move/list não enxergam REQDir com subpastas, e req move não move o arquivo

> Created: 2026-08-04 | Status: done

REQ: REQ-2026-08-03-req-move-list-nao-suportam-subpastas-e-req-move-nao-move-arquivo.md

## Diagnóstico / Contexto

`REQ-2026-08-03-req-move-list-nao-suportam-subpastas-e-req-move-nao-move-arquivo.md` documenta dois
defeitos reais, confirmados nos três CLIs (Go, Node.js, Python):

1. **`req list`/`req move` não descem em subpastas de `REQDir`.** `ListREQs`/`findREQ` (Go,
   `internal/generators/req.go:118,265`), `listREQs`/`findREQ` (Node,
   `npm/src/generators/req.js:11,108`) e `find_req` (Python,
   `pypi/trackfw/generators/req.py:154`) fazem apenas um nível de listagem/leitura do diretório —
   nenhum desce em `REQDir/<estado>/` (por-estado) nem `REQDir/<agente>/<estado>/` (by_agent).
   **Python nem sequer expõe `req list` hoje** (`pypi/trackfw/commands/req.py` só registra `new` e
   `move`) — lacuna adicional a fechar nesta correção para viabilizar o AC5 (paridade observável).
2. **`req move` nunca move fisicamente o arquivo**, só reescreve `status:` no mesmo caminho —
   `MoveREQ` (Go, linha 241/258), `moveREQ` (Node, linha 121/130), `move_req` (Python, linha
   167/183-184) — em todas, o `WriteFile`/`writeFileSync`/`open(...).write` grava de volta no
   caminho de origem. Diverge do padrão já testado em `MoveRoadmap`/`moveRoadmap`/`move_roadmap`, que
   resolve diretório-alvo, faz rename físico, sincroniza `status:` e loga a transição.

`ADR-2026-08-04-req-move-list-reusar-roadmap-namespacing-para-req-e-mover-fisicamente-o-arquivo.md`
já decidiu as duas questões de design em aberto:

- **D1** — REQ reusa `RoadmapNamespacing`/`roadmapNamespacing`/`roadmap_namespacing` do config; sem
  campo `req_namespacing` novo.
- **D3** — `req move` só move fisicamente quando o arquivo foi encontrado dentro de uma subpasta de
  estado reconhecida (`backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`), determinado pela
  **estrutura de diretórios do próprio arquivo encontrado** — não por uma flag de config isolada.
  Assim uma REQ solta em `REQDir/` continua em modo in-place (comportamento atual, sem migração
  forçada), e uma REQ já organizada em `REQDir/<estado>/` ou `REQDir/<agente>/<estado>/` passa a ser
  movida de verdade.

### Algoritmo de referência (mesmo nos 3 CLIs)

**Descoberta recursiva** (`listREQFiles`/equivalente) — para um dado `cfg`:
1. Glob `REQDir/*.md` (flat legado).
2. Glob `REQDir/<estado>/*.md` para cada estado em
   `["backlog","analyzing","wip","blocked","done","abandoned"]` (por-estado, sem agente).
3. Se `roadmap_namespacing == "by_agent"`: glob `REQDir/<agente>/<estado>/*.md` para cada agente
   (`cfg.agents`, ou subpastas de primeiro nível de `REQDir` se `agents` vazio) × cada estado.
4. Concatena os três conjuntos — não são mutuamente exclusivos, um projeto pode ter REQs em mais de um
   layout simultaneamente (ex.: migração em andamento). `req list` mostra todos; `req move`/`findREQ`
   usa o primeiro que casar o nome (mesma ordem: flat → por-estado → by_agent).

**Move condicional** (`MoveREQ`/equivalente), a partir do `path` retornado por `findREQ`:
1. `parentDir = dirname(path)`; `grandparentDir = dirname(parentDir)`.
2. Se `parentDir == REQDir` (arquivo solto) → **modo in-place**: comportamento atual — reescreve
   `status:`, grava no mesmo `path`, não move nem cria pastas. Fim.
3. Senão, se `grandparentDir == REQDir` e `basename(parentDir)` é um dos 6 estados válidos → **layout
   por-estado**: `targetDir = REQDir/<novo-estado>/`.
4. Senão, se `basename(parentDir)` é um dos 6 estados válidos e `basename(grandparentDir)` é uma
   subpasta de primeiro nível de `REQDir` → **layout by_agent**: `agent = basename(grandparentDir)`,
   `targetDir = REQDir/<agent>/<novo-estado>/`.
5. Nos casos 3/4: `MkdirAll(targetDir)`, reescreve `status:` no conteúdo, escreve em
   `targetDir/basename(path)`, remove o arquivo original (rename), registra a transição em
   `REQDir/.trackfw-log` no mesmo formato de `appendTransitionLog` do roadmap (linha:
   `<timestamp>  <basename ou agente/basename>  <estado-origem> → <estado-destino>`).

## Acceptance Criteria

Consolidado dos AC1-AC7 da REQ — critérios mensuráveis por ML nas seções de Wave abaixo:

- [x] AC1 — `req list` recursivo nos 3 layouts (flat, por-estado, by_agent), sem flag adicional
- [x] AC2 — `req move`/`findREQ` encontram REQs em subpastas nos 3 layouts
- [x] AC3 — `req move` move fisicamente o arquivo quando já há subpasta de estado; permanece in-place
      para REQs soltas em `REQDir/`
- [x] AC4 — transição de REQ registrada em `.trackfw-log`, mesmo formato do roadmap
- [x] AC5 — paridade comprovada nos 3 CLIs (Go, Node.js, Python) contra o mesmo fixture
- [x] AC6 — testes de regressão cobrindo os 2 novos layouts, preservando a cobertura do modo legado
- [x] AC7 — README/docs atualizados sobre namespacing de REQ e comportamento condicional do move

## Wave 1 — Correção nos 3 CLIs (3 MLs em paralelo)
> Dependências: Independente (arquivos/linguagens distintas, sem sobreposição)

### ML-1A — Go: recursão + move físico condicional em `internal/generators/req.go`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/req.go` (`ListREQs`, `findREQ`, `MoveREQ` — linhas 118, 265, 241)
- `internal/commands/req.go` (linha ~153, call site de `ListREQs`)
- `internal/generators/req_test.go` (preservar `TestMoveREQ_RewritesStatusInPlace`, linha 268; adicionar
  novos testes)

**Ações:**
1. Adicionar `listREQFiles(cfg config.ProjectConfig) []string` em `req.go`, implementando o algoritmo
   de descoberta recursiva descrito acima. Reaproveitar `roadmapStateOrder` e `roadmapValidStateNames`
   já definidos em `internal/generators/roadmap.go` (mesmo package `generators` — sem import extra).
2. Reescrever `ListREQs` para chamar `config.Load()` internamente e iterar `listREQFiles(cfg)` (mudar
   assinatura para `func ListREQs() error`, sem parâmetro `dir`); imprimir `No REQs found in
   %s\n"` com `cfg.REQDir` quando vazio, preservando o formato de saída (`%-60s %s`) e `parseREQMeta`.
   Atualizar `internal/commands/req.go:153` para `return generators.ListREQs()`.
3. Reescrever `findREQ(name string, cfg config.ProjectConfig) (string, error)` para iterar
   `listREQFiles(cfg)` e retornar o primeiro cujo `filepath.Base` contém `name` (case-insensitive, via
   `containsIgnoreCase` já existente em `roadmap.go`). Atualizar `MoveREQ` para chamar
   `findREQ(name, cfg)` (removendo o antigo parâmetro `dir string`).
4. Em `MoveREQ`, após obter `path` e `updated` (conteúdo já com `status:` reescrito), implementar o
   passo "Move condicional" do algoritmo de referência: se `path` está solto em `cfg.REQDir`, gravar
   in-place (comportamento atual, sem alteração de mensagem `✓ updated ...`); se está em subpasta de
   estado reconhecida (por-estado ou by_agent), resolver `targetDir` via `stateDir`/`agentStateDir` (já
   existentes em `roadmap.go`), `os.MkdirAll`, escrever em `targetDir/basename(path)`, `os.Remove(path)`
   se o destino for diferente da origem, e imprimir `✓ moved %s → %s\n` (mesmo padrão de
   `MoveRoadmap`). Registrar a transição em `cfg.REQDir + "/.trackfw-log"` com uma função
   `appendREQTransitionLog(basename, fromState, toState string)` local a `req.go` (mesmo formato de
   `appendTransitionLog` em `roadmap.go:456`, arquivo de log diferente).
5. Testes novos em `req_test.go`: `TestListREQs_ByState` (fixture `docs/req/backlog/REQ-x.md`),
   `TestListREQs_ByAgent` (fixture `docs/req/claude/wip/REQ-y.md` com `RoadmapNamespacing:
   "by_agent"`), `TestFindREQ_RecursesSubfolders`, `TestMoveREQ_PhysicallyMovesInStateLayout`,
   `TestMoveREQ_PhysicallyMovesInByAgentLayout`, `TestMoveREQ_LogsTransition`. Não remover nem alterar
   `TestMoveREQ_RewritesStatusInPlace` (deve continuar passando sem mudança de assert, comprovando que
   o modo legado é preservado).

**Critérios de aceite:**
- [x] `go build ./...` sem erros
- [x] `go test ./internal/generators/... ./internal/commands/...` verde, incluindo
      `TestMoveREQ_RewritesStatusInPlace` inalterado e os 6 testes novos
- [x] `go vet ./...` sem avisos

**Comandos de validação:** `go build ./... && go test ./internal/... && go vet ./...`

---

### ML-1B — Node.js: recursão + move físico condicional em `npm/src/generators/req.js`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/generators/req.js` (`listREQs`, `findREQ`, `moveREQ` — linhas 11, 108, 121)
- `npm/src/commands/req.js` (linha ~64-67, call site de `listREQs`)
- `npm/tests/req_move.test.js` (preservar teste in-place existente, linha 15; adicionar novos)

**Ações:** mesmo algoritmo do ML-1A, adaptado para Node:
1. Adicionar `listREQFiles(cfg)` em `req.js`, reaproveitando `stateDir`/`agentStateDir`/
   `STATE_ORDER`/`roadmapValidStateNames`-equivalente já existentes em `npm/src/generators/roadmap.js`
   (`require('./roadmap')` se não exportados, ou exportar as constantes/funções necessárias de
   `roadmap.js` — ver `module.exports` ao final do arquivo, linha ~666).
2. Reescrever `listREQs(cfg)` para receber `cfg` (não mais `dir` string) e iterar `listREQFiles(cfg)`;
   atualizar `npm/src/commands/req.js` para passar `require('../config').load()` inteiro em vez de só
   `.reqDir`.
3. Reescrever `findREQ(name, cfg)` para iterar `listREQFiles(cfg)`; atualizar `moveREQ` para usar a
   nova assinatura.
4. Em `moveREQ`, implementar o move condicional: `fs.mkdirSync(targetDir, {recursive:true})`,
   `fs.writeFileSync(dst, ...)`, `fs.unlinkSync(src)` se `dst !== src`, log de transição em
   `<reqDir>/.trackfw-log` via função local `appendREQTransitionLog`, mesmo formato de
   `appendTransitionLog` em `roadmap.js`.
5. Testes novos em `npm/tests/`: espelhar os 6 testes do ML-1A (nomes de arquivo livres, ex.
   `req_list_subfolders.test.js`, `req_move_physical.test.js`). Não alterar o teste existente
   `'moveREQ rewrites frontmatter and header status without moving file'`
   (`npm/tests/req_move.test.js:15`) — deve continuar passando.

**Critérios de aceite:**
- [x] `npm test` (workspace `npm/`) verde, incluindo o teste in-place existente e os novos
- [x] Nenhum warning de lint (se `npm run lint` existir no workspace)

**Comandos de validação:** `npm --prefix npm test`

---

### ML-1C — Python: recursão + `req list` novo + move físico condicional em `pypi/trackfw/`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/generators/req.py` (`find_req`, `move_req` — linhas 154, 167)
- `pypi/trackfw/commands/req.py` (adicionar subcomando `list`, hoje só `new`/`move` — linhas 9-48)
- `pypi/tests/test_generators_req.py` (preservar `test_move_req_rewrites_status_in_place`, linha 107;
  adicionar novos)

**Ações:**
1. **Adicionar `list_reqs(cfg: dict) -> None`** em `req.py` (função nova — hoje não existe nenhum
   equivalente Python de `listREQs`). Formato de saída idêntico ao Go/Node: `f"{filename:<60} {status}"`
   por linha, `"No REQs found in {req_dir}"` se vazio. Reaproveitar `parse_req_status`-equivalente (se
   não existir em Python, adicionar extraindo de `"| Status: "` como em `parseREQStatus`/
   `parseREQMeta`).
2. Adicionar `list_req_files(cfg: dict) -> list[str]` em `req.py`, mesmo algoritmo dos MLs 1A/1B,
   reaproveitando `_state_dir`/`_agent_state_dir`/`STATE_ORDER`-equivalente de
   `pypi/trackfw/generators/roadmap.py` (linhas 109, 116) — importar de lá, não duplicar.
3. Reescrever `find_req(name, cfg)` (troca o parâmetro `req_dir: str` por `cfg: dict`) para iterar
   `list_req_files(cfg)`. Atualizar `move_req` para a nova assinatura.
4. Em `move_req`, implementar o move condicional: `os.makedirs(target_dir, exist_ok=True)`, escreve no
   destino, `os.remove(src)` se destino != origem (mesmo padrão usado em `move_roadmap`,
   `pypi/trackfw/generators/roadmap.py:522`), log de transição em `<req_dir>/.trackfw-log` via função
   local `_append_req_transition_log`, mesmo formato de `_append_transition_log` do roadmap.
5. Em `pypi/trackfw/commands/req.py`: registrar `req_sub.add_parser("list", ...)` (sem argumentos
   posicionais — mesmo padrão do Go/Node `req list` sem filtro), despachar em `_dispatch` para
   `_cmd_list`, que chama `list_reqs(cfg)`. Atualizar help text `"Commands: new, move"` →
   `"Commands: new, move, list"`.
6. Testes novos em `pypi/tests/test_generators_req.py` (ou arquivo dedicado
   `test_req_list_move_subfolders.py`): espelhar os 6 casos do ML-1A, incluindo um teste de CLI para o
   novo `req list` (`test_cli_req_list_by_state`, `test_cli_req_list_by_agent`). Não alterar
   `test_move_req_rewrites_status_in_place` — deve continuar passando.

**Critérios de aceite:**
- [x] `pytest pypi/tests/` verde, incluindo o teste in-place existente e os novos
- [x] `trackfw req list` funcional via CLI Python (antes inexistente) — provado por teste de CLI, não
      só de função

**Comandos de validação:** `cd pypi && python -m pytest tests/`

## Wave 2 — Paridade comprovada e documentação (2 MLs em paralelo)
> Dependências: Wave 1 completa (os 3 CLIs precisam estar corrigidos para comparar comportamento)

### ML-2A — AC5: prova de paridade entre os 3 binários
**Status:** ✅ Concluído
**Arquivos afetados:**
- Script/fixture temporário (não versionado) ou teste de integração dedicado, seguindo o mesmo padrão
  de prova usado em `REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-...md` AC3

**Ações:**
1. Montar um fixture único com `trackfw.yaml` (`req_dir: docs/req`, `roadmap_namespacing: by_agent`,
   `agents: [claude]`) e REQs nos 3 layouts (flat, por-estado, by_agent).
2. Executar `req list` e `req move <nome> done` nos três binários (`bin/trackfw`, `node npm/bin/...`,
   `python -m pypi.trackfw ...` ou equivalente instalado) contra o mesmo fixture, capturando
   stdout e o estado final do filesystem.
3. Comparar: mesmo conjunto de arquivos listados, mesmo destino físico após `move`, mesmo conteúdo de
   `.trackfw-log`. Documentar o resultado da prova (pode ser um teste de integração versionado em
   `internal/commands/` ou um script descartável cujo output vai no corpo do PR — decisão do
   implementador, mas o resultado da comparação deve ficar registrado no PR).

**Critérios de aceite:**
- [x] Os 3 CLIs produzem o mesmo conjunto de REQs listados e o mesmo destino físico pós-move no mesmo
      fixture
- [x] Evidência da comparação anexada ao PR (output ou teste versionado)

**Comandos de validação:** execução manual/scriptada dos 3 binários contra o fixture compartilhado

### ML-2B — AC7: documentação
**Status:** ✅ Concluído
**Arquivos afetados:**
- `README.md` (ou `docs/` — localizar seção de configuração/namespacing existente para
  `roadmap_namespacing`)
- `docs/cli-parity.md` (se aplicável, registrar a paridade alcançada nesta correção)

**Ações:**
1. Documentar explicitamente que REQ suporta `roadmap_namespacing: by_agent` (reuso do mesmo campo,
   decisão D1 do ADR) — hoje implícito, nunca declarado.
2. Documentar o comportamento de `req move`: in-place quando a REQ está solta em `REQDir/`; move físico
   quando já está em `REQDir/<estado>/` ou `REQDir/<agente>/<estado>/` — comportamento condicional, sem
   migração automática de layout.

**Critérios de aceite:**
- [x] README/docs atualizados refletindo o comportamento implementado na Wave 1
- [x] Nenhuma referência residual afirmando que `req move` "só reescreve status" sem mencionar o modo
      físico novo

**Comandos de validação:** revisão manual do diff de documentação contra o comportamento implementado

## Wave 3 — Auditoria final (1 ML)
> Dependências: Wave 1 e Wave 2 completas

### ML-3A — Regressão completa + fechamento da REQ
**Status:** ✅ Concluído
**Arquivos afetados:**
- Nenhum arquivo de produto — apenas execução de gates e atualização de metadados de governança

**Ações:**
1. Rodar a suíte completa dos 3 CLIs (`make quality` ou equivalente) para garantir que nada regrediu
   fora do escopo desta correção.
2. Rodar `trackfw validate` no próprio repositório trackfw para confirmar que a governança (REQ ↔ ADR ↔
   Roadmap) permanece consistente.
3. Marcar todos os ACs (AC1-AC7) da REQ como atendidos e mover a REQ para `Done` via
   `trackfw req move` (usando o próprio comando corrigido nesta implementação — dogfooding).

**Critérios de aceite:**
- [x] `make quality` (ou os 3 comandos equivalentes) verde
- [x] `trackfw validate` sem violações
- [x] REQ-2026-08-03 movida para status `Done`

**Comandos de validação:** `make quality && trackfw validate`
