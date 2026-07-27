---
status: done
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md"
squad: ""
---

# Roadmap: integridade das referencias e ciclo de vida da REQ

> Created: 2026-07-27 | Status: done

## Context

REQ: `docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`
ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`

**38 de 48 REQs (79%)** têm no frontmatter `roadmap:` um caminho inexistente, e `trackfw validate`
está **verde**. E nada fecha a REQ quando o roadmap conclui — 6 estão `Open` com roadmap em `done/`,
o que produz falso positivo numa regra de severidade `error`.

Os três escapes que mantêm o `validate` verde (cada um suficiente sozinho):

| # | Escape | Onde |
|:-:|---|---|
| 1 | frontmatter `roadmap:` nunca é lido — extrator busca `Roadmap:` no corpo | `validator.go:1291-1311` |
| 2 | `referenceExists` faz fallback por **basename recursivo** | `validator.go:1356-1377` |
| 3 | severidade `warning` | `config.go:88` |

**37 das 38 referências apontam para arquivo existente.** É ausência de formato canônico, não
rastreabilidade perdida — o template grava `roadmap: ""` e nunca se definiu como preencher.

### Ordem das waves

```
Wave 1 (1A) ─ barrier ─> Wave 2 (2A ‖ 2B) ─ barrier ─> Wave 3 (3A)
  expõe os 3 escapes      contrato + ciclo de vida     normaliza dados + gate
```

A Wave 1 escreve os testes negativos **antes** de qualquer correção — mesma disciplina do ciclo
anterior. Corrigir primeiro faria as regras passarem por efeito colateral e perderíamos a prova de
que estavam cegas.

---

## Wave 1 — Expor os escapes (agente único)

> Dependências: nenhuma.

### ML-1A — Testes negativos que provam a cegueira

**Status:** done
**Files affected:** testes do validator nos 3 CLIs

**Actions:**
1. **Teste do escape 1**: REQ com `roadmap:` no frontmatter apontando para caminho inexistente, e
   **sem** a linha `Roadmap:` no corpo → deve haver violação. Hoje não há.
2. **Teste do escape 2**: REQ cujo corpo tem `Roadmap: docs/roadmaps/wip/X.md` enquanto o arquivo está
   em `docs/roadmaps/done/X.md` → deve haver violação. Hoje o fallback por basename valida.
3. **Teste do escape 3**: confirmar que `ref_targets_exist` com severidade `error` reprova o gate.
4. **Teste do Defeito 2**: REQ `Open` cujo roadmap está em `done/` → deve ser sinalizada.
5. Todos devem **falhar** neste estado. Capture a saída e registre — é o entregável.
6. Marque como esperando falha de forma **strict** nos 3 runtimes (`xfail(strict=True)` no Python,
   helper `testSkip` já existente no Node). ⚠️ **Go: não use `t.Skip`** — ele não executa o corpo e
   ficaria pulado para sempre quando a Wave 2 corrigir. Use um mecanismo que **avise no XPASS**, como
   o `testSkip` do Node faz. Foi a assimetria que encontrei no ciclo passado.
7. `make quality` verde.

**Acceptance criteria:**
- [x] 4 cenários cobertos nos 3 CLIs, todos falhando contra o código atual
- [x] Saída das falhas registrada no relatório
- [x] Marcação strict nos 3 — nenhum runtime cala se o defeito for corrigido
- [x] `make quality` verde

**Relatório ML-1A — Artemis — 2026-07-27:**

Arquivos alterados:
- `internal/validator/validator_integrity_xfail_test.go`
- `npm/tests/validator.test.js`
- `pypi/tests/test_validator.py`

Semântica strict por runtime:
- Go: helper `xfailExpect` executa o corpo e emite `t.Errorf` em XPASS; não usa `t.Skip`.
- Node.js: helper `testSkip` executa o corpo e incrementa `failed` em XPASS.
- Python: `pytest.mark.xfail(strict=True)`.

Evidência das falhas esperadas:
- `go test ./internal/validator -run 'TestXFail' -v` → 4/4 `PASS` com logs `[xfail esperado]`
  para Escape 1, Escape 2, Escape 3 e Defeito 2.
- `npm test -- --runInBand --test-name-pattern=validator` → `37 passed, 0 failed, 4 xfail`
  no `tests/validator.test.js`.
- `python3 -m pytest pypi/tests/test_validator.py -q -rxX -k ml1a` → `59 deselected, 4 xfailed`.

Validação final:
- `python3 -m pytest pypi/tests/test_validator.py -q -rxX` no sandbox falhou fora do ML-1A por
  `PermissionError` ao tentar criar diretórios temporários em `~/`; reexecutado como parte do
  `make quality` fora do sandbox.
- `make quality` → verde: Go `ok` incluindo `internal/validator`; Node `261 pass` e validator
  `37 passed, 0 failed, 4 xfail`; Python `604 passed, 4 xfailed`; `go vet`, build, parity,
  static/integration assets, identity parity, artifact parity e falsification gates passaram.

---

## Wave 2 — Contrato e ciclo de vida (2 MLs paralelos)

> Dependências: **barrier** — ML-1A concluído. Diretórios disjuntos: validator × geradores/comandos.

### ML-2A — Formato canônico e validação real do link

**Status:** done
**Files affected:** `internal/validator/validator.go`, `npm/src/validator/index.js`,
`pypi/trackfw/validator.py`, `internal/config/config.go` e equivalentes, `docs/cli-parity.md`

**Actions:**
1. **Formato canônico — decidido, não reabrir:** `roadmap:` e `adr:` no frontmatter usam **caminho
   relativo completo a partir da raiz do projeto, com `.md`** (ex.:
   `docs/roadmaps/done/ROADMAP-....md`).
   **Verificado pelo orquestrador:** `internal/serve/api_chain.go` monta o nó com `ID: path`, onde
   `path` vem de `filepath.WalkDir(cfg.RoadmapDir, ...)` — caminho relativo completo — e a aresta é
   `chainEdge{From: path, To: val}` com `val` sendo o valor cru do frontmatter. Qualquer outro formato
   (basename, caminho parcial) gera **aresta órfã** no grafo do `serve`. Documentar em
   `docs/cli-parity.md` como contrato.
2. **Validar o campo do frontmatter** por caminho, nos 3 CLIs. Hoje nenhuma regra o lê.
3. **Remover o fallback por basename** de `referenceExists` (`validator.go:1356-1377` e equivalentes).
   **Decidido: remover, não tornar opt-in.** Um permissivo que aceita o caminho errado não é
   validação, e 32 das 38 referências inválidas vivem exatamente dele — mantê-lo como opção significa
   que ninguém liga e nada muda.
4. **Corrigir `blocked` namespace-aware**: hoje é `cfg.RoadmapDir + "/blocked"` hardcoded
   (`validator.go:1319`) na mesma função onde `wip` passa por `resolveStateDirs`. Em `by_agent`,
   roadmaps blocked nunca são varridos.
5. Reativar os testes do ML-1A referentes aos escapes 1 e 2.

> ⚠️ **A elevação de `ref_targets_exist` para `error` NÃO é deste ML — é do ML-3A.** Com `error` e as
> 38 referências ainda não normalizadas, `make quality` ficaria **vermelho** entre a Wave 2 e a Wave 3
> e este ML não conseguiria fechar com a barrier verde. A severidade sobe **depois** que os dados
> estiverem limpos.

**Acceptance criteria:**
- [x] Formato canônico documentado em `docs/cli-parity.md`
- [x] Link do frontmatter validado por caminho nos 3 CLIs
- [x] Fallback por basename **removido** dos 3 CLIs
- [x] `blocked` usa `resolveStateDirs` nos 3 CLIs
- [x] Testes do ML-1A (escapes 1 e 2) reativados e passando
- [x] `make quality` verde — a severidade ainda é `warning`, então as 38 referências pendentes não
      reprovam o gate neste ponto

**Relatório ML-2A — Apolo — 2026-07-27:**

Arquivos alterados:
- `internal/validator/validator.go`
- `internal/validator/validator_integrity_xfail_test.go`
- `internal/validator/validator_improvements_test.go`
- `internal/validator/validator_namespacing_test.go`
- `internal/validator/validator_test.go`
- `npm/src/validator/index.js`
- `npm/tests/validator.test.js`
- `npm/tests/namespacing.test.js`
- `pypi/trackfw/validator.py`
- `pypi/tests/test_validator.py`
- `pypi/tests/test_namespacing.py`
- `docs/cli-parity.md`

Entregue:
- `extractRefPath`/`_extract_ref_path`/`extractRefPath` agora leem `adr:` e `roadmap:` em
  frontmatter de forma case-insensitive e removem aspas simples/duplas do valor antes de validar.
- `referenceExists`/`_reference_exists` valida somente o caminho literal expandido (`~/` incluso),
  sem fallback por basename recursivo.
- `validateBlockedHasREQ` e `validateRefTargetsExist` usam `resolveStateDirs(..., "blocked")` nos
  três runtimes.
- Escape 1 e Escape 2 foram reativados nos três runtimes; Escape 3 permanece xfail para ML-3A e
  Defeito 2 permanece xfail para ML-2B.
- `docs/cli-parity.md` documenta o formato canônico: caminho relativo completo desde a raiz do
  projeto, com `.md`, sem basename permissivo.

Validação:
- `go build ./...` → exit 0; o sandbox emitiu aviso não bloqueante ao tentar escrever cache em
  `/Users/kgsaran/go/pkg/mod/cache`.
- `go test ./...` → verde.
- `npm test` na raiz → falhou por ausência esperada de `package.json`; reexecutado em `npm/`.
- `(cd npm && npm test)` → `261 pass`, `0 fail`.
- `python3 -m pytest pypi/tests -q -rxX` → `607 passed, 2 xfailed`.
- `bin/trackfw validate` → exit 0, expondo 41 warnings de referências ainda não normalizadas
  (mantidas para ML-3A).
- `make quality` → verde: Go, Node, Python, `go vet`, build, CLI/validate parity, static/integration
  assets, identity parity, artifact parity e falsification gates passaram.

### ML-2B — Fechamento da REQ e higiene de paridade

**Status:** done
**Files affected:** `internal/commands/req.go`, `internal/generators/req.go`,
`npm/src/commands/{req,log}.js`, `pypi/trackfw/commands/{req,log}.py`, `internal/commands/log.go`,
`internal/config/config.go`

**Actions:**
1. **Comando que fecha a REQ**, nos 3 CLIs: `req move <nome> <status>` (simetria com `roadmap move`).
   Reescreve **frontmatter `status:` E header `> Date: … | Status: …`** — os dois, sempre juntos.
   Espelhe a semântica de `rewriteRoadmapStatus` (`internal/generators/roadmap.go:239-252`), que já
   resolveu esse problema para o roadmap: escopo estrito ao bloco de frontmatter, demais linhas byte a
   byte, não inventa chave.
   ⚠️ **`req move` NÃO move arquivo.** Diferente do roadmap, a REQ não tem estado-pasta — vive flat em
   `docs/req/`. Espelhar `MoveRoadmap` literalmente faria o comando tentar criar `docs/req/done/`.
   É **só reescrita dos dois campos de status**, no lugar onde o arquivo já está.
2. **`trackfw log` grava no mesmo arquivo nos 3 CLIs.** Hoje: Go usa `<roadmap_dir>/.trackfw-log`
   (respeita config), Node usa `docs/roadmaps/.trackfw-log` **hardcoded** (`npm/src/commands/log.js:12`),
   Python usa a **raiz do projeto** (`pypi/trackfw/commands/log.py:25`). Canônico: o do Go — respeita
   `roadmap_dir`. É o log que alimenta as métricas.
3. **Strip de aspas em `forge` e `trace_id_field` no Go** (`internal/config/config.go:287-292`). Node
   (`.replace(/^["']|["']$/g,'')`) e Python (`.strip("\"'")`) já removem. `forge: "github"` produz
   valores diferentes entre runtimes hoje.
4. Reativar o teste do ML-1A referente ao Defeito 2.

**Acceptance criteria:**
- [x] Comando de fechamento da REQ nos 3 CLIs, sincronizando frontmatter **e** header
- [x] `trackfw log` grava no mesmo caminho nos 3, respeitando `roadmap_dir`
- [x] `forge` e `trace_id_field` com strip de aspas no Go
- [x] Teste do Defeito 2 reativado e passando

**Relatório ML-2B — Apolo — 2026-07-27:**

Arquivos alterados:
- `internal/commands/req.go`
- `internal/commands/log_test.go`
- `internal/config/config_test.go`
- `internal/generators/req.go`
- `internal/generators/req_test.go`
- `internal/validator/validator.go`
- `internal/validator/validator_integrity_xfail_test.go`
- `npm/src/commands/log.js`
- `npm/src/commands/req.js`
- `npm/src/generators/req.js`
- `npm/src/validator/index.js`
- `npm/tests/config.test.js`
- `npm/tests/log_path.test.js`
- `npm/tests/req_move.test.js`
- `npm/tests/validator.test.js`
- `pypi/trackfw/commands/log.py`
- `pypi/trackfw/commands/req.py`
- `pypi/trackfw/generators/req.py`
- `pypi/trackfw/validator.py`
- `pypi/tests/test_commands_basic.py`
- `pypi/tests/test_config.py`
- `pypi/tests/test_generators_req.py`
- `pypi/tests/test_log_command.py`
- `pypi/tests/test_validator.py`

Entregue:
- `req move <nome> <status>` nos 3 CLIs, reescrevendo somente `status:` do frontmatter e o primeiro
  `| Status: ...` no header antes de seções `##`, sem mover a REQ flat e preservando ocorrências no corpo.
- `trackfw log` em Node e Python convergido para o contrato do Go: leitura de
  `<roadmap_dir>/.trackfw-log` com `--tail`; testes legados Python atualizados para essa semântica.
- Teste Go travando strip de aspas em `forge` e `trace_id_field` via `splitKV`, alinhado a Node/Python.
- Regra de ciclo de vida `req_roadmap_lifecycle` nos 3 validadores: REQ `Open` com roadmap canônico em
  `done/` gera warning. O teste do Defeito 2 foi reativado nos 3 runtimes; Escape 3 permanece para ML-3A.

Validação:
- `go build ./...` → exit 0; aviso não bloqueante de cache Go fora do workspace no sandbox.
- `go test ./...` → verde.
- `(cd npm && npm test)` → `263 pass`, `0 fail`.
- `python3 -m pytest pypi/tests -q -rxX` → `611 passed, 1 xfailed`.
- `git diff --check` → verde.
- `make quality` → verde: Go, Node, Python, `go vet`, build, CLI/validate parity, static/integration
  assets, identity parity, artifact parity e falsification gates passaram.

---

## Wave 3 — Normalização dos dados e gate (agente único)

> Dependências: **barrier** — Wave 2 concluída. O formato canônico precisa existir antes de normalizar.

### ML-3A — Normalizar as 38 referências e proteger com gate

**Status:** done
**Files affected:** `docs/req/*.md`, `scripts/`, `Makefile`

**Actions:**
1. **Normalizar as 38 referências inválidas** para o formato canônico. 37 apontam para arquivo que
   existe — resolva por basename contra `docs/roadmaps/**` e `docs/adr/**`. A que não existe
   (`ROADMAP-2026-07-25-escopo-...`, sem `.md`) precisa de investigação individual.
   **Não invente referência**: se não houver correspondência confiável, deixe vazio e registre.
2. **Sincronizar as 6 REQs `Open`** cujo roadmap está em `done/`, usando o comando criado no ML-2B —
   não à mão. É a prova de que o comando funciona.
3. **Elevar `ref_targets_exist` para `error`** (`internal/config/config.go:88` e equivalentes nos 3
   CLIs). **A ordem importa:** só depois dos itens 1 e 2, com os dados já limpos. Elevar antes deixaria
   `make quality` vermelho durante toda a Wave 2. O default de um gate de integridade deve reprovar.
4. **Gate de integridade referencial**: script que verifica que todo `roadmap:`/`adr:` de frontmatter
   aponta para arquivo existente, com **prova negativa** (P4) — quebrar uma referência
   propositalmente e afirmar que o gate reprova. Integrar ao `make quality`, sem variável auxiliar,
   sem resíduo.
5. Reativar o que restar dos testes do ML-1A (escape 3).

**Acceptance criteria:**
- [x] 38 referências normalizadas; zero apontando para arquivo inexistente
- [x] 6 REQs fechadas **pelo comando**, não manualmente
- [x] `ref_targets_exist` elevado para `error` nos 3 CLIs, DEPOIS da normalização
- [x] Gate com prova negativa, rodando em `make quality`
- [x] `trackfw validate` verde; `git status` limpo após os testes

**Relatório ML-3A — Apolo — 2026-07-27:**

Arquivos alterados:
- 38 REQs em `docs/req/*.md`
- `internal/config/config.go`
- `internal/validator/validator_integrity_xfail_test.go`
- `npm/src/config/index.js`
- `npm/tests/validator.test.js`
- `pypi/trackfw/config.py`
- `pypi/tests/test_validator.py`
- `scripts/check-referential-integrity.sh`
- `scripts/check-gates-falsify.sh`
- `Makefile`

Reconciliação das contagens:
- Medição inicial com `bin/trackfw validate --json`: 41 warnings `ref_targets_exist`.
- A divergência 41 × 38 ocorre porque os 41 eram itens de validação, não campos `roadmap:` únicos:
  a saída incluía warnings de `adr:` não canônicos e não cobria o caso `ROADMAP-2026-07-25-escopo-...`
  sem `.md`, ignorado pelo extrator.
- A reconciliação estática do frontmatter confirmou 38 campos `roadmap:` não canônicos. A investigação
  individual do caso `escopo` encontrou correspondência única em
  `docs/roadmaps/done/ROADMAP-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`
  e ADR única em `docs/adr/ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`.
- Normalização aplicada em 38 arquivos: 38 campos `roadmap:` e 6 campos `adr:` passaram para caminho
  relativo completo, com `.md`, sempre com correspondência única por basename. Nenhuma referência foi
  inventada.

Fechamento de REQs:
- 6 REQs `Open` com roadmap em `done/` foram fechadas via `bin/trackfw req move ... Done`:
  `REQ-2026-06-13-traceid-bidirecional.md`,
  `REQ-2026-06-13-v2.4-config-evolution.md`,
  `REQ-2026-06-20-gate-pre-trabalho-branch-wip-roadmap-e-fallback-husky-node.md`,
  `REQ-2026-07-19-corrigir-render-antigravity-com-tools-validos-e-model-tier-do-agy.md`,
  `REQ-2026-07-19-global-adrs-governance.md`,
  `REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade.md`.
- Resultado pós-normalização e fechamento, antes de elevar severidade: `bin/trackfw validate --json`
  retornou 0 violations e 0 warnings.

Gate e severidade:
- `ref_targets_exist` elevado de `warning` para `error` nos defaults dos 3 CLIs.
- Escape 3 reativado nos 3 runtimes: Go, Node.js e Python agora provam que referência quebrada vira
  violation com a configuração padrão.
- Novo gate `scripts/check-referential-integrity.sh` verifica `adr:` e `roadmap:` de frontmatter das
  REQs por existência literal do arquivo.
- `Makefile` integra o gate em `make quality`, sem variável auxiliar.
- `scripts/check-gates-falsify.sh` ganhou P4 `referential-integrity/missing-roadmap`, com fixture
  temporário quebrado e sem resíduo no workspace. O cenário 8 existente passou a usar `GOCACHE` local
  ao diretório temporário para funcionar no sandbox.

Validação:
- `go build ./...` → verde; aviso não bloqueante de cache Go fora do workspace no sandbox.
- `go test ./...` → verde.
- `(cd npm && npm test)` → `263 pass`, `0 fail`.
- `python3 -m pytest pypi/tests -q -rxX` → `612 passed`.
- `scripts/check-referential-integrity.sh` → `Referential integrity OK`.
- `scripts/check-gates-falsify.sh` → `Falsification checks passed (all 9 scenarios, 8 gates proved non-vacuous)`.
- `bin/trackfw validate --json` → 0 violations, 0 warnings.
- `make quality` → verde: Go, Node, Python, `go vet`, build, CLI/validate parity, integridade referencial,
  static/integration assets, identity parity, artifact parity e falsification gates passaram.

Correção pós-auditoria:
- Auditoria reprovou a primeira entrega porque `scripts/check-gates-falsify.sh` podia abortar no
  `go build` isolado do cenário 8 com stderr suprimido, encerrando após
  `artifact-parity/req-content-drift` sem rodar `artifact-parity/req-name-drift`,
  `referential-integrity/missing-roadmap` ou o resumo final.
- O cenário 8 compila uma cópia temporária do módulo Go com `internal/generators/req.go` corrompido
  para gerar `RREQ-...`; nesta sessão a compilação completou, mas a falha era opaca quando o build
  retornava erro.
- `scripts/check-gates-falsify.sh` agora usa `build_go_or_fail`, que mantém `GOCACHE` temporário e,
  em falha, imprime `FAIL [falsify/setup-s8-build]`, o comando exato e stdout/stderr capturados.
- Validação pós-correção: `scripts/check-gates-falsify.sh` executou os 9 cenários e finalizou com
  `Falsification checks passed (all 9 scenarios, 8 gates proved non-vacuous)`; `make quality` verde.

## Acceptance Criteria

- [x] As 3 waves concluídas, na ordem
- [x] Os três escapes eliminados, cada um com teste que provou a cegueira antes
- [x] Formato canônico documentado e aplicado nos 3 CLIs
- [x] REQ fecha por comando, sincronizando os dois lugares de status
- [x] `make quality` verde, sem variável auxiliar
- [x] Escopo negativo da REQ respeitado — os 5 grupos ficam registrados, não corrigidos
