---
status: wip
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md"
squad: ""
---

# Roadmap: Débitos técnicos pós-release de robustez e manutenibilidade

> Created: 2026-07-27 | Status: wip

## Context

REQ: `docs/req/REQ-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md`

Roadmap não bloqueante, destinado a eliminar três fontes de degradação silenciosa ou manutenção
manual depois da próxima release.

## Wave 1 — Decisões e provas negativas (2 MLs)

> Dependencies: none. MLs paralelos, sem código de produção.

### ML-1A — Definir contrato de idade e erros de inspeção

**Status:** done

**Files affected:**
- novo ADR ou adendo explícito ao ADR de gates verificáveis
- `docs/cli-parity.md`
- fixtures/testes negativos dos validators

**Actions:**
- Decidir se idade significa último commit, entrada em WIP ou transição registrada.
- Definir fallback fora de repositório Git.
- Classificar ENOENT, permissão, arquivo inválido e erro de walk em warning/violation.
- Registrar compatibilidade e defaults.

**Acceptance criteria:**
- [x] Decisões fechadas antes de implementação.
- [x] Casos negativos reproduzidos nos três runtimes.
- [x] Nenhuma alteração de produção.

**ML-1A result — 2026-07-27 (Apolo):**
- Contrato registrado como adendo em
  `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md` e em
  `docs/cli-parity.md`.
- Decisão temporal: idade de `stale_wip` é a entrada mais recente do roadmap em `wip/` no
  `.trackfw-log`; fallback retrocompatível fora de log é `mtime`; `git log` deixa de ser fonte
  contratual de idade.
- Política de inspeção definida: `ENOENT` de diretório de estado opcional não gera finding;
  permissão negada, `ENOTDIR`/erro de walk, arquivo esperado ilegível e arquivo/linha de log inválidos
  devem emitir diagnóstico da regra, com severidade configurável da própria regra.
- Provas negativas adicionadas nos três runtimes para a lacuna temporal e para erro de walk em `wip/`:
  `internal/validator/validator_stale_wip_contract_xfail_test.go`,
  `npm/tests/validator.test.js` e `pypi/tests/test_validator.py`.
- Nenhum arquivo de produção foi alterado neste ML.

**Validation:**
```bash
go test ./internal/validator -run 'StaleWIP' -v
(cd npm && npm test -- --test-name-pattern='stale_wip')
python3 -m pytest pypi/tests/test_validator.py -q -rxX
```

### ML-1B — Provar lacuna do catálogo no gate de identidade

**Status:** done

**Files affected:**
- `scripts/check-identity-parity.sh`
- `scripts/check-gates-falsify.sh`
- fixtures temporárias do catálogo

**Actions:**
- Inserir alvo/superfície temporário no catálogo e provar que o gate atual não o exercita.
- Criar expectativa de falha estrita sem resíduo.

**Acceptance criteria:**
- [x] Gate atual demonstrado como incompleto.
- [x] Prova negativa identifica o alvo ausente.

**ML-1B result — 2026-07-27 (Apolo):**
- `scripts/check-identity-parity.sh` passou a validar que `TARGETS` cobre todas as superfícies de
  agentes suportadas no catálogo canônico. A lista continua hardcoded; este ML apenas impede que um
  alvo/superfície novo entre no catálogo sem prova no gate.
- `scripts/check-gates-falsify.sh` ganhou o cenário P4
  `identity-parity/catalog-target-missing`, que injeta temporariamente a superfície
  `codex=experimental` numa cópia do catálogo e exige falha por
  `catalog target/surface not covered by TARGETS`.
- A fixture temporária fica sob o diretório `mktemp` do harness e é removida pelo `trap`; nenhum
  catálogo real é alterado.
- Validation:
  - `scripts/check-identity-parity.sh` →
    `Identity parity verified across Go/Node/Python for 11 target/surface combinations (with and without identity)`.
  - `scripts/check-gates-falsify.sh` →
    `Falsification checks passed (all 13 scenarios, 8 gates proved non-vacuous)`.

## Wave 2 — Implementações independentes (3 MLs em paralelo)

> Dependencies: Wave 1 complete.

### ML-2A — `stale_wip` configurável e determinístico

**Status:** pending

**Files affected:**
- `internal/config/config.go`
- `internal/validator/validator.go`
- `npm/src/config/index.js`
- `npm/src/validator/index.js`
- `pypi/trackfw/config.py`
- `pypi/trackfw/validator.py`
- testes equivalentes dos três runtimes

**Actions:**
- Adicionar configuração canônica conforme decisão do ML-1A.
- Preservar default retrocompatível.
- Injetar relógio/fonte temporal em testes para remover dependência do horário real.

**Acceptance criteria:**
- [ ] Paridade dos três CLIs.
- [ ] Testes determinísticos para limite, boundary e fallback.

### ML-2B — Política explícita de erros de I/O

**Status:** pending

**Files affected:**
- validators Go/Node/Python
- testes de filesystem dos três runtimes

**Actions:**
- Refatorar helpers de walk/read para distinguir ausência esperada de falha de inspeção.
- Acumular diagnósticos sem abortar no primeiro arquivo.
- Aplicar inicialmente às regras `adr_orphan`, `blocked_by_draft_adr`, `blocked_has_req`,
  `ref_targets_exist` e demais sites inventariados no ML-1A.

**Acceptance criteria:**
- [ ] Nenhuma falha de permissão vira sucesso silencioso.
- [ ] Diagnóstico contém regra e arquivo/diretório.
- [ ] ENOENT opcional mantém comportamento documentado.

### ML-2C — Gate de identidade derivado do catálogo

**Status:** pending

**Files affected:**
- `scripts/check-identity-parity.sh`
- assets/fixtures do catálogo, se necessário
- `scripts/check-gates-falsify.sh`

**Actions:**
- Derivar alvos e superfícies do catálogo canônico.
- Manter exceções não-default explícitas e justificadas.
- Reativar a prova negativa do ML-1B.

**Acceptance criteria:**
- [ ] Todo alvo/superfície aplicável é exercitado.
- [ ] Alvo novo entra no gate sem edição manual da lista.
- [ ] Gate continua isolado do HOME real.

## Wave 3 — Consolidação e compatibilidade (1 ML)

> Dependencies: Wave 2 complete.

### ML-3A — Paridade, documentação e regressão

**Status:** pending

**Files affected:**
- `docs/cli-parity.md`
- `site/guide/commands.md`
- `site/en/guide/commands.md`
- `Makefile`
- gates de falsificação

**Actions:**
- Documentar configuração de stale e política de I/O.
- Executar matriz Go/Node/Python e provas P4.
- Confirmar que projetos sem novas chaves preservam o comportamento anterior.

**Acceptance criteria:**
- [ ] Paridade dos três runtimes.
- [ ] Compatibilidade retroativa comprovada.
- [ ] `make quality` e `trackfw validate` verdes.
- [ ] Gates negativos falham pelos motivos esperados.

## Acceptance Criteria

- [ ] Três débitos eliminados.
- [ ] Nenhuma degradação silenciosa nos casos cobertos.
- [ ] Catálogo e gate permanecem sincronizados automaticamente.
- [ ] Trabalho concluído sem bloquear indevidamente a release anterior.
