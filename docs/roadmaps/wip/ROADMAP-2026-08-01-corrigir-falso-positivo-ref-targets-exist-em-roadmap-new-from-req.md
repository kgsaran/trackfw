---
status: wip
date: 2026-08-01
req: "docs/req/REQ-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req.md"
squad: ""
---

# Roadmap: Corrigir falso-positivo ref_targets_exist em roadmap new --from-req

> Created: 2026-08-01 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req.md
ADR: docs/adr/ADR-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-parametro-roots-morto.md

`roadmap new --from-req` gera roadmap que o `validate` reprova mesmo com a REQ existindo.
Reproduzido em diretório limpo em 2026-08-01: frontmatter recebe o **basename**, corpo recebe o
caminho completo, validador lê o frontmatter, `os.Stat` falha.

Terceiro caso seguido de "a ferramenta reprova o que ela mesma gerou".

### Os quatro pontos de código (verificados em 2026-08-01)

| | Gerador `--from-req` | Gerador simples | `referenceExists` | Chamadores |
|---|---|---|---|---|
| Go | `internal/generators/roadmap.go:175` | `internal/generators/roadmap.go` ~95 | `internal/validator/validator.go:1491` | `:1462`, `:1478`, `:1483` |
| Node | `npm/src/generators/roadmap.js:517` | `npm/src/generators/roadmap.js:425` | `npm/src/validator/index.js:781` | `:757`, `:769`, `:773` |
| Python | `pypi/trackfw/generators/roadmap.py:337` | `pypi/trackfw/generators/roadmap.py:192` | `pypi/trackfw/validator.py:968` | `:937`, `:951`, `:957` |

### Bug irmão incorporado (AC2b)

Descoberto durante o setup deste próprio ciclo: `roadmap new --title <t> --req <path>` grava
`req: ""` **vazio** no frontmatter. Como `extractRefPath` tem early-return para valor vazio,
**nenhuma** violação dispara — é um falso-**negativo**, complementar ao falso-positivo do
`--from-req`. Mesmo campo, mesmos arquivos: incorporado em vez de virar ciclo separado.

Este roadmap é a prova viva: foi gerado com `--req` e saiu com `req: ""`.

### Decisão do ADR

Contrato = **caminho relativo completo**. O parâmetro `roots` de `referenceExists` é
**removido** (não implementado) nos 3 CLIs, com os 3 chamadores de cada ajustados.
A validação segue **estrita**. `extractRefPath` **não** muda.

### Dependências e paralelismo

Wave 1 tem **3 MLs em paralelo** — cada CLI tem gerador e validador próprios, arquivos disjuntos.

`make parity` e `make quality` **falham** até os três estarem prontos
(`check-artifact-parity.sh` compara artefatos entre CLIs). Por isso **nenhum ML da Wave 1 tem
`parity` nos critérios**; a paridade é a Wave 2, que age como barreira.

### Risco herdado a confirmar na barreira

O cenário `roadmap-acceptance-heading/*/from-req` de `scripts/check-gates-falsify.sh` roda hoje
com `ref_targets_exist` co-ocorrendo. O `assert_fails_with` casa a substring de `wip_acceptance`,
então a previsão é que **não** quebre — mas isso é **confirmado empiricamente na Wave 2**, nunca
presumido.

## Acceptance Criteria

Consolidados da REQ (AC1–AC10). Detalhamento por microlote abaixo.

- [ ] `--from-req` grava caminho completo no `req:` do frontmatter, nos 3 CLIs
- [ ] `--req` no caminho simples também grava o caminho completo (AC2b)
- [ ] `roots` removido da assinatura e dos 3 chamadores, nos 3 CLIs
- [ ] Validação segue estrita: `req:` com basename continua reprovando, coberto por teste
- [ ] `extractRefPath` intocado
- [ ] `validate` verde no repositório; `check-artifact-parity.sh` passa
- [ ] Cenário `roadmap-acceptance-heading/*/from-req` continua passando (confirmado, não presumido)
- [ ] Cenário de falsificação novo para esta correção
- [ ] `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes

---

## Wave 1 — Geradores e validadores (3 MLs EM PARALELO)
> Dependências: nenhuma. Arquivos disjuntos por CLI.

### ML-1A — CLI Go
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `internal/generators/roadmap.go`, `internal/validator/validator.go` + testes Go

**Acceptance criteria:**
- [ ] `--from-req` grava `reqPath` completo (não `filepath.Base`) no `req:` do frontmatter
- [ ] Caminho simples com `--req` grava o caminho completo; **sem** `--req` mantém `req: ""`
- [ ] `referenceExists` perde o parâmetro `roots`; os 3 chamadores ajustados
- [ ] Teste garantindo que `req:` com basename **continua** reprovando (validação estrita)
- [ ] `make build`, `make lint`, `go test ./...` verdes
- [ ] Ciclo em tmp: `--from-req` → `wip` → `validate` sem `ref_targets_exist`
- [ ] Não tocar em `npm/`, `pypi/`, nem em `extractRefPath`

### ML-1B — CLI Node
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `npm/src/generators/roadmap.js`, `npm/src/validator/index.js` + testes Node

**Acceptance criteria:** equivalentes ao ML-1A, no CLI Node (`npm test` verde).
- [ ] Não tocar em `internal/`, `pypi/`

### ML-1C — CLI Python
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/generators/roadmap.py`, `pypi/trackfw/validator.py` + testes Python

**Acceptance criteria:** equivalentes ao ML-1A, no CLI Python.
- [ ] Não tocar em `internal/`, `npm/`, `pypi/build/lib/`

---

### Auditoria da Wave 1 (Zeus, 2026-08-01)

Verificação independente, não por relato:

- `roots` removido da assinatura nos três: `referenceExists(ref string)`,
  `referenceExists(ref)`, `_reference_exists(ref: str)`. **Nenhum chamador** ainda passa o
  segundo argumento — varredura limpa nos três arquivos.
- **Ciclo real nos 3 CLIs**, em diretórios temporários, com os binários/módulos de verdade:

  | CLI | `req:` gerado | violações `does not exist` |
  |---|---|---|
  | Go | `docs/req/REQ-2026-08-01-fonte-go.md` | **0** |
  | Node | `docs/req/REQ-2026-08-01-fonte-node.md` | **0** |
  | Python | `docs/req/REQ-2026-08-01-fonte-python.md` | **0** |

- **Risco herdado resolvido:** os 6 cenários `roadmap-acceptance-heading/*` do PR #96 **continuam
  passando** (30/30, exit 0). A previsão da nota de vault se confirmou — o `assert_fails_with`
  casa a substring de `wip_acceptance`, então perder a violação co-ocorrente não afeta. Confirmado
  empiricamente, como o roadmap exigia.
- `check-artifact-parity.sh` passa; `make quality` exit 0; `validate` verde.

**Convergência independente:** os três agentes, sem se comunicarem, decidiram manter o basename no
comentário `<!-- Derived from REQ: -->` — texto de leitura humana, não campo validado. Paridade
textual preservada sem coordenação explícita, o que valida a precisão do handoff.

**Cobertura de validação estrita:** os três adicionaram (ou confirmaram) teste provando que um
`req:` com basename **continua** reprovando, e os três demonstraram que o teste é capaz de falhar.
É o que impede alguém de "consertar" um sintoma futuro afrouxando o validador.

---

## Wave 2 — Barreira: paridade e falsificação (1 ML)
> Dependências: **ML-1A, ML-1B e ML-1C completos e auditados**

### ML-2A — Paridade, regressão do gate herdado e seam novo
**Status:** pending
**Agente:** Ártemis

**Ações:**
1. `scripts/check-artifact-parity.sh` passa; `make quality` exit 0; `validate` verde.
2. **Confirmar empiricamente** que os 6 cenários `roadmap-acceptance-heading/*` continuam
   passando — em especial os `from-req`, que perdem a violação co-ocorrente.
3. Cenário permanente novo: revertendo o gerador para gravar basename, o ciclo `--from-req` →
   `wip` → `validate` deve **falhar** com `ref_targets_exist`. Seguir o idioma dos cenários
   existentes, cobrindo os 3 CLIs se viável.
4. Provar que o cenário novo é capaz de falhar.

**Acceptance criteria:**
- [ ] `check-artifact-parity.sh` passa; `make quality` exit 0; `validate` verde
- [ ] 6 cenários herdados confirmados passando
- [ ] Cenário novo adicionado, contador atualizado na linha final
- [ ] Cenário novo provado não vacuoso
- [ ] `git status --porcelain` sem resíduo de teste
