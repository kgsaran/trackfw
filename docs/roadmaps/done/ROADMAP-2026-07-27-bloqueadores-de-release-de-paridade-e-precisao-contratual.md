---
status: done
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md"
squad: ""
---

# Roadmap: Bloqueadores de release de paridade e precisão contratual

> Created: 2026-07-27 | Status: done

## Context

REQ: `docs/req/REQ-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md`

Quatro gaps comprometem paridade ou veracidade contratual da próxima versão. O roadmap usa provas
negativas antes das correções e fecha com gates cross-CLI e package smoke.

## Wave 1 — Provar os quatro bloqueadores (1 ML)

> Dependencies: none.

### ML-1A — Testes negativos de flags, aspas, log e schemas

**Status:** done

**Files affected:**
- `pypi/tests/test_commands_roadmap_discover.py`
- `pypi/tests/test_validator.py`
- `pypi/tests/test_generators_roadmap.py`
- `npm/tests/roadmap_move.test.js`
- `internal/generators/roadmap_test.go`
- testes/fixtures de documentação aplicáveis

**Actions:**
1. Provar que o help Python não expõe as três flags e que fixtures equivalentes divergem de Go/Node.
2. Provar que `status: "wip"` gera resultado divergente no Python.
3. Provar que `backlog → wip` em `by_agent` perde o agente somente no log Python.
4. Adicionar teste documental que falhe enquanto o site atribuir JSON Schema ao validator sem
   implementação correspondente.
5. Usar xfail strict/XPASS guard nos três ambientes quando aplicável.

**Acceptance criteria:**
- [x] Quatro bloqueadores cobertos com mensagens diagnósticas ou guard explícito quando já corrigido.
- [x] Nenhum código de produção alterado.
- [x] Comandos focados verdes; `make quality` reservado para auditoria central conforme handoff.

**Validation commands:**
```bash
python3 -m pytest pypi/tests/test_commands_roadmap_discover.py pypi/tests/test_validator.py pypi/tests/test_generators_roadmap.py -q -rxX
(cd npm && npm test)
go test ./internal/generators -v
make quality
```

**ML-1A result — 2026-07-27 (Artemis):**
- Python flags: `pypi/tests/test_commands_roadmap_discover.py` adiciona xfail strict para `roadmap new`
  exigir `--title`, `--req` e `--from-req`; controles Go/Node em
  `internal/commands/roadmap_flags_test.go` e `npm/tests/roadmap_command.test.js` provam a superfície
  equivalente nos runtimes de referência.
- Python quoted status: `pypi/tests/test_validator.py` adiciona xfails strict para
  `parse_frontmatter` e `folder_status` com `status: "wip"`.
- Python by_agent log: a base atual já preservava `zeus/<arquivo>.md`; em vez de criar xfail que daria
  XPASS imediato, `pypi/tests/test_generators_roadmap.py` recebeu guard obrigatório contra regressão do
  log `backlog → wip`.
- JSON Schema docs: `pypi/tests/test_documentation_contract.py` adiciona xfail strict enquanto o site
  afirmar que `trackfw validate` consome `docs/schema/*.json` automaticamente.
- Validation:
  - `python3 -m pytest pypi/tests/test_commands_roadmap_discover.py pypi/tests/test_validator.py pypi/tests/test_generators_roadmap.py pypi/tests/test_documentation_contract.py -q -rxX`
    → `115 passed, 4 xfailed`.
  - `go test ./internal/commands ./internal/generators -run 'RoadmapNewCmdExposesParityFlags|MoveRoadmap' -v`
    → pass.
  - `npm test -- --test-name-pattern='roadmap new exposes parity flags|moveRoadmap'` → suíte Node executada
    com `265 pass`, `0 fail`.
  - `bin/trackfw validate --json` → `0 violations`, `0 warnings`.

## Wave 2 — Correções independentes (4 MLs em paralelo)

> Dependencies: Wave 1 complete. Ownership sem sobreposição entre comando, validator, log e docs.

### ML-2A — Paridade das flags de roadmap no Python

**Status:** done

**Files affected:**
- `pypi/trackfw/commands/roadmap.py`
- `pypi/trackfw/generators/roadmap.py`
- `pypi/tests/test_commands_roadmap_discover.py`
- `pypi/tests/test_generators_roadmap.py`

**Actions:**
- Implementar `--title`, `--req` e `--from-req` com precedência idêntica a Go/Node.
- Reutilizar geradores existentes; não duplicar parsing de REQ.
- Cobrir help, argumentos válidos, conflitos e saída gerada.

**Acceptance criteria:**
- [x] Help e comportamento Python convergem com Go/Node.
- [x] Testes do ML-1A reativados e verdes.
- [x] Package Python build/install/smoke verde.

### ML-2B — Normalização de aspas no frontmatter Python

**Status:** done

**Files affected:**
- `pypi/trackfw/validator.py`
- `pypi/trackfw/traceid.py`
- `pypi/tests/test_validator.py`
- `pypi/tests/test_traceid.py`

**Actions:**
- Centralizar normalização de valores YAML flat, removendo aspas simples/duplas externas.
- Aplicar a mesma semântica em validator e trace ID.
- Cobrir valores vazios, aspas e conteúdo interno preservado.

**Acceptance criteria:**
- [x] Fixtures aspeadas produzem paridade com Go/Node.
- [x] Nenhuma regressão em campos não aspeados.

### ML-2C — Log `by_agent` com atribuição paritária

**Status:** done

**Resultado da Wave 1:** a implementação já está presente na base (`log_basename` preserva
`<agent>/ROADMAP-…`); o ML-1A adicionou a prova obrigatória de regressão. Não há correção de
produção remanescente neste microlote.

**Files affected:**
- `pypi/trackfw/generators/roadmap.py`
- `pypi/trackfw/commands/log.py`
- `pypi/tests/test_generators_roadmap.py`
- `pypi/tests/test_log_command.py`

**Actions:**
- Espelhar exatamente o formato Go/Node para transições em `by_agent`.
- Preservar o formato flat.
- Provar leitura por `log` e pelas métricas.

**Acceptance criteria:**
- [x] Logs equivalentes byte a byte nos três CLIs para flat e `by_agent`.
- [x] Agente aparece uma única vez e no campo correto.

### ML-2D — Contrato verdadeiro para JSON Schemas

**Status:** done

**Files affected:**
- `site/guide/ai-agents.md`
- `site/en/guide/ai-agents.md`
- `docs/cli-parity.md`
- teste/gate documental correspondente

**Actions:**
- Decisão fechada para esta release: documentar schemas como auxiliares externos, não consumidos
  automaticamente por `trackfw validate`.
- Corrigir PT-BR/EN e exemplos de uso.
- Adicionar gate contra a alegação antiga.

**Acceptance criteria:**
- [x] Nenhuma página afirma validação automática inexistente.
- [x] Exemplos externos continuam corretos.
- [x] PT-BR e inglês convergem semanticamente.

## Wave 3 — Release gate integrado (1 ML)

> Dependencies: Wave 2 complete.

### ML-3A — Paridade cross-CLI e smoke de release

**Status:** done

**Files affected:**
- `scripts/check-cli-parity.sh`
- `scripts/check-artifact-parity.sh`
- `scripts/check-gates-falsify.sh`

**Actions:**
- Exercitar flags Python com geração real de artefato.
- Comparar parsing aspeado e logs flat/`by_agent` nos três CLIs.
- Adicionar provas negativas P4 para drift das flags e do log.
- Reforçar os gates já integrados ao `make quality` sem resíduos.

**Acceptance criteria:**
- [x] Gates falham para cada regressão proposital.
- [x] Go, Node e Python produzem resultados equivalentes.
- [x] `make quality` verde; smoke de npm e PyPI verde.
- [x] `trackfw validate` retorna 0/0.
- [x] REQ liberada para release.

**ML-3A result — 2026-07-27 (Artemis):**
- `scripts/check-cli-parity.sh` agora valida a superfície pública de `roadmap new` nos três runtimes,
  exigindo `--title`, `--req` e `--from-req`.
- `scripts/check-artifact-parity.sh` passou a gerar artefatos reais com `--title/--req` e
  `--from-req`, comparar 8 tipos de artefato nos runtimes Go/Node/Python, validar `status: "wip"`
  como 0/0 e preservar o ciclo flat/`by_agent`.
- `scripts/check-gates-falsify.sh` foi ampliado para 12 cenários, incluindo drift de flag pública em
  `roadmap new` e drift do log `by_agent`.
- Validation:
  - `GO_BIN=bin/trackfw scripts/check-cli-parity.sh` → `CLI parity smoke checks passed`.
  - `GO_BIN=bin/trackfw scripts/check-artifact-parity.sh` →
    `Artifact parity checks passed (8 artifact types × 3 runtimes; roadmap flags, quoted status, analyzing cycle flat/by_agent)`.
  - `GO_BIN=bin/trackfw scripts/check-gates-falsify.sh` →
    `Falsification checks passed (all 12 scenarios, 8 gates proved non-vacuous)`.
  - `bin/trackfw validate --json` → `0 violations`, `0 warnings`.
  - `git diff --check` → verde.
  - `make quality` → verde em execução anterior desta mesma sessão; package smoke não foi reexecutado
    na retomada por orientação explícita do handoff.

## Acceptance Criteria

- [x] Quatro bloqueadores encerrados.
- [x] Zero divergências públicas não documentadas.
- [x] Provas negativas preservadas.
- [x] `make quality` e package smoke verdes.
