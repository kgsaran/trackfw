---
status: done
date: 2026-08-01
req: "docs/req/REQ-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req.md"
squad: ""
---

# Roadmap: Corrigir falso-positivo ref_targets_exist em roadmap new --from-req

> Created: 2026-08-01 | Status: done

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

- [x] `--from-req` grava caminho completo no `req:` do frontmatter, nos 3 CLIs
- [x] `--req` no caminho simples também grava o caminho completo (AC2b)
- [x] `roots` removido da assinatura e dos 3 chamadores, nos 3 CLIs
- [x] Validação segue estrita: `req:` com basename continua reprovando, coberto por teste
- [x] `extractRefPath` intocado
- [x] `validate` verde no repositório; `check-artifact-parity.sh` passa
- [x] Cenário `roadmap-acceptance-heading/*/from-req` continua passando (confirmado empiricamente)
- [x] Cenário de falsificação novo para esta correção — na verdade dois, contador 30 → 42
- [x] `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes

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
**Status:** ✅ concluído (auditado 2026-08-01)
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
- [x] `check-artifact-parity.sh` passa; `make quality` exit 0; `validate` verde
- [x] 6 cenários herdados confirmados passando
- [x] Cenário novo adicionado; contador **30 → 42**
- [x] Cenário novo provado não vacuoso
- [x] `git status --porcelain` sem resíduo de teste

**Entrega acima do pedido.** Foram criados **dois** cenários, cada um com braço de *baseline* e
braço de *detecção*, nos 3 CLIs:

- **Cenário 25 — `--from-req`:** casa o texto completo
  `links to REQ "REQ-flag-source.md" which does not exist`, e não o genérico `does not exist`
  que o handoff sugeriu. Correção dela, e correta: o padrão genérico casaria também as violações
  irmãs de ADR/Roadmap ausentes. Efeito colateral útil — a corrupção deixa o caminho completo no
  corpo e o basename só no frontmatter, o que **pina a precedência frontmatter-sobre-corpo** do
  `extractRefPath`.
- **Cenário 26 — `--req` simples (AC2b):** aqui a regressão **não** produz violação
  (`extractRefPath` tem early-return para vazio — é silêncio, não erro), então `assert_fails_with`
  não serviria. Ela inspeciona o artefato e compara o campo `req:` contra o caminho exato passado
  a `--req` — não apenas "não-vazio", o que também pega uma regressão para basename.

**Falsificação independente de Zeus:** reverti o gerador Go para `filepath.Base(reqPath)` no
`--from-req` e rodei o gate:

```
EXIT=1
FAIL [falsify/roadmap-req-frontmatter-path/go/from-req-baseline]: ciclo limpo saiu com 1, esperava 0
```

O braço de baseline é o que detecta — comprova que a proteção é real, não decorativa.

**Comentário obsoleto corrigido por ela:** o Cenário 24 afirmava que "o ciclo com REQ nunca reprova
limpo" — verdade antes da Wave 1, falsa depois, e contradita pelo braço de baseline novo no mesmo
arquivo. Reescrito. Dois comentários adjacentes afirmando fatos opostos seria armadilha garantida.

---

## Fechamento

Concluído e auditado em 2026-08-01. `make quality` exit 0; falsificação **42/42**.

**Entrega:** os 3 CLIs gravam caminho completo no campo `req:` (nos dois caminhos de geração), o
parâmetro `roots` morto foi removido de `referenceExists` com os 3 chamadores de cada CLI
ajustados, e a validação segue **estrita** — com teste em cada CLI garantindo que basename
continua reprovando.

**Proteção de CI:** contador de cenários de falsificação **30 → 42**. Dois cenários novos, cada um
com baseline e detecção, cobrindo os 3 CLIs e os dois caminhos de geração.

**Quatro defeitos fechados neste ciclo:** o falso-positivo do `--from-req` (basename), o
falso-**negativo** do `--req` simples (`req: ""` vazio, descoberto durante o próprio setup), o
parâmetro `roots` morto que enganava três chamadores em cada CLI, e um comentário obsoleto no
gate que afirmaria o oposto do cenário recém-adicionado.
