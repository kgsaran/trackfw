---
status: wip
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md"
squad: ""
---

# Roadmap: integridade das referencias e ciclo de vida da REQ

> Created: 2026-07-27 | Status: wip

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

**Status:** pending
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
- [ ] Comando de fechamento da REQ nos 3 CLIs, sincronizando frontmatter **e** header
- [ ] `trackfw log` grava no mesmo caminho nos 3, respeitando `roadmap_dir`
- [ ] `forge` e `trace_id_field` com strip de aspas no Go
- [ ] Teste do Defeito 2 reativado e passando

---

## Wave 3 — Normalização dos dados e gate (agente único)

> Dependências: **barrier** — Wave 2 concluída. O formato canônico precisa existir antes de normalizar.

### ML-3A — Normalizar as 38 referências e proteger com gate

**Status:** pending
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
- [ ] 38 referências normalizadas; zero apontando para arquivo inexistente
- [ ] 6 REQs fechadas **pelo comando**, não manualmente
- [ ] `ref_targets_exist` elevado para `error` nos 3 CLIs, DEPOIS da normalização
- [ ] Gate com prova negativa, rodando em `make quality`
- [ ] `trackfw validate` verde; `git status` limpo após os testes

## Acceptance Criteria

- [ ] As 3 waves concluídas, na ordem
- [ ] Os três escapes eliminados, cada um com teste que provou a cegueira antes
- [ ] Formato canônico documentado e aplicado nos 3 CLIs
- [ ] REQ fecha por comando, sincronizando os dois lugares de status
- [ ] `make quality` verde, sem variável auxiliar
- [ ] Escopo negativo da REQ respeitado — os 5 grupos ficam registrados, não corrigidos
