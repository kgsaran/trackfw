---
status: wip
date: 2026-10-02
req: "docs/req/REQ-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md"
squad: "hades-tf, apolo-tf, artemis-tf, hefesto-tf"
---

# Roadmap: qualquer .md em adr_dirs é contado como ADR — o critério passa a ser o prefixo ADR-

> Created: 2026-10-02 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md
ADR: docs/adr/ADR-2026-10-02-o-criterio-de-identificacao-de-adr-e-o-prefixo-adr-no-nome-do-arquivo-aplicado-no-primitivo-unico-de-enumeracao.md
Issue: #471 (o PR fecha). Base: `main` em `44718ffc`.

Mapa dos sítios (lido em `44718ffc`):

| sítio | hoje | depois |
|---|---|---|
| `internal/validator/validator.go` `walkADRFilePathsForRule` (~:3107) | `strings.HasSuffix(path, ".md")` | basename com prefixo `ADR-` (sem distinção de maiúsculas) **e** `.md` (D1/D2) |
| consumidores do primitivo: `ResolveADRFiles`, `WalkADRFilePaths` (`adr list`, `NewADRDraft`), `walkADRFiles`, regras do `validate` | herdam | herdam (sem mudança própria) |
| `internal/validator/validator.go` `findADRFile` (~:3140) | varredura própria por basename | **não muda**: resolve referência explícita (decisão da Wave 0, ADR D2) |
| `internal/validator/validator.go` `mdBasenamesOnDisk` (scope-redirect, ~:342/454) | todo `.md` | **não muda**: detecção de artefato perdido precisa ser larga |
| `internal/serve/api_chain.go` `scanChainDir` (~:83), tipo `adr` | `WalkDir` próprio, `HasSuffix(".md")` | nós de ADR pelo primitivo (D3) |
| `internal/discover/discover.go` sonda de fallback (~:486 subpastas e ~:491 plano) | `countMDFiles(docs/adr…)` | contagem pelo primitivo nos **dois** chamadores (D3) |
| regra nova `adr_file_without_prefix` | — | warning (D4), ligada nos **dois** caminhos de aplicação de regra do `validate` (`applyRule` ~:856 e `applyRuleTagged` ~:1211) |

## Acceptance Criteria
- [x] AC1 — Wave 0 auditada
      ✅ `docs/seguranca/2026-10-02-wave0-criterio-de-adr.md`: APROVA COM AJUSTES (A1–A5 absorvidos no ADR e nos ML-1A/1B)
- [x] AC2 — fixture de três braços: `NOTAS.md` ≡ vazio em status/context/discover/adr list/validate
      ✅ `TestADRPrefixE2E_AC2_ThreeArms` (binário): reprova em `44718ffc`
- [x] AC3 — `adr-001-x.md` minúsculo conta
      ✅ `TestADRPrefixE2E_AC3_LowercasePrefixCounts` + unitário `TestWalkADRFilePathsForRule_LowercaseADREnumerated`
- [x] AC4 — `/api/chain` sem nó para `NOTAS.md`
      ✅ `TestChainHandler_ADRPrefixFilter_NoNotasNodeButADRReqRoadmapPresent`
- [x] AC5 — sonda do `discover` sem crédito para `NOTAS.md`
      ✅ `TestADRPrefixE2E_AC5_DiscoverFallbackIgnoresNOTAS` + `TestScan_Fallback{Flat,Subdir}_NotasNotCounted`
- [x] AC6 — `adr_file_without_prefix` dispara só no caso certo
      ✅ `TestADRPrefixE2E_AC6_*` + 5 unitários `TestADRFileWithoutPrefix_*`
- [x] AC7 — `adr new` não é afetado pelo critério (premissa original errada: o nome vem da data, não de numeração)
      ✅ `TestADRPrefixE2E_AC7_AdrNewUsesDateSlug` (premissa corrigida: o nome vem da data, não de contador)
- [x] AC8 — `cli-parity.md` e pinos de conjunto de regras atualizados
      ✅ pinos 32/32 (pin28/pin29 novos); seção nova no `cli-parity.md`
- [x] AC9 — teste novo declara o que afirma e reprova no critério antigo
      ✅ prova de mordida por overlay em todos os ML; uma frase por teste
- [ ] AC10 — `make quality` EXIT=0 e CI verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Completude dos sítios e modelo de ameaça do critério
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-02-wave0-criterio-de-adr.md` (único arquivo escrito)
**Actions:**
1. **Completude:** procure todo sítio que enumera, conta, lista ou resolve ADR (`grep -rn "ADRDirs\|adr_dirs\|walkADR\|ResolveADRFiles\|findADRFile" internal/ scripts/`, e geradores de contexto/CLAUDE.md). Diga se a tabela do Context está fechada. Decida `findADRFile`: uma REQ que referencia um `.md` sem prefixo deve continuar resolvendo o vínculo ou não?
2. **Ameaça:** quem faz o produto contar ADR que não existe, ou esconder ADR que existe, sem quebrar regra escrita? Mínimo: nome `ADR-` em diretório não-ADR alcançado por symlink; `ADR-.md` (prefixo sem corpo); maiúsculas mistas; arquivo `ADR-x.md` que é diretório; Unicode parecido com `A`/`D`/`R` (homoglifo).
3. **Falsificação nas duas direções** por sítio: frouxo (volta a contar `NOTAS.md`) e restrito (deixa de contar `adr-001.md` ou `ADR-…` legítimo).
4. **Resíduo declarado.**
**Acceptance criteria:**
- [x] As quatro seções com evidência (comando + saída)
- [x] Veredito explícito
- [x] Nenhuma linha de implementação

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-02-wave0-criterio-de-adr.md
grep -q "Veredito" docs/seguranca/2026-10-02-wave0-criterio-de-adr.md
```

## Wave 1 — Implementação (2 MLs em paralelo: arquivos disjuntos)
> Dependencies: Wave 0 auditada

### ML-1A — Critério no primitivo + regra `adr_file_without_prefix`
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/validator/validator.go`, testes em `internal/validator/*_test.go`,
`docs/cli-parity.md`, `scripts/check-validate-rule-pins.sh` (se o conjunto de regras for pinado)
**Actions:** D1/D2 em `walkADRFilePathsForRule`, incluindo `d.Type().IsRegular()` (symlink de diretório
`ADR-x.md` deixa de contar; A3 da Wave 0); `findADRFile` **intocado**; regra D4 (warning em
`ruleDefaults`) usando `resolveAdrStatus` (A2), nos dois caminhos de aplicação; comentários que dizem
"sem filtro de prefixo" (ex.: `WalkADRFilePaths` ~:3068) corrigidos.
**Acceptance criteria:**
- [x] Testes: `NOTAS.md` não é enumerado; `adr-001-x.md` é; `ADR-…` é; symlink de diretório `ADR-x.md` não é; regra D4 nos quatro braços (frontmatter `status:`, cabeçalho `| Status:`, `README.md` sem status, `ADR-…`)
- [x] A1 da Wave 0: REQ com `blocked_by:` para `decisao.md` (sem prefixo, Draft) continua disparando `blocked_by_draft_adr`
- [x] Cada teste reprova com o critério antigo (prova por overlay); uma frase por teste
      ✅ Auditoria (arquiteto): 10 testes rodados por nome, PASS; prova de mordida A–F por overlay no relatório; pinos 32/32.
      ⚠️ `d.Type().IsRegular()` exclui também symlink de ARQUIVO, que antes contava e é legível por `readRegularFile` (segue o link): regressão na direção restrita → ML-1C.
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ ./internal/generators/ -count=1
```

### ML-1B — `serve` e sonda do `discover` pelo primitivo
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/serve/api_chain.go`, `internal/discover/discover.go`, testes dos dois pacotes,
`scripts/check-adr-enumeration-single-point.sh` (só o comentário de isenção)
**Paralelismo:** não toca `internal/validator/`; consome `validator.WalkADRFilePaths`/`ResolveADRFiles`, que já existem.
**Actions:** D3. Em `scanChainDir`, para `nodeType == "adr"`, enumerar pelo primitivo; REQ e roadmap
seguem como estão. No `discover`, a sonda de fallback conta ADR pelo primitivo nos dois chamadores
(subpastas e plano). Atualizar o comentário de isenção de `scripts/check-adr-enumeration-single-point.sh`
(~:88) que deixa de ser verdade (A4).
**Acceptance criteria:**
- [x] Testes: `/api/chain` sem nó para `NOTAS.md` **e** com os nós de REQ e roadmap intactos (A5); sonda do `discover` sem crédito no layout plano e no de subpastas; uma frase por teste
- [x] O gate de "ADR enumeration outside single point" do `check-gates-falsify.sh` segue verde
      ✅ Auditoria: os 3 testes reprovavam antes do ML-1A (prova natural de mordida) e passam com ele; `check-adr-enumeration-single-point.sh` e `check-ref-separator-portability.sh` verdes.
**Gates da wave:**
```bash
go build ./...
go test ./internal/serve/ ./internal/discover/ -count=1
```

### ML-1C — Corretivo: symlink de arquivo volta a contar; só symlink de diretório sai
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Origem:** auditoria do ML-1A. A3 da Wave 0 pedia excluir o symlink de **diretório** `ADR-x.md`. O
`d.Type().IsRegular()` (Lstat) exclui também o symlink de **arquivo** `ADR-x.md → ../shared/ADR-x.md`,
que contava antes e que `readRegularFile` lê (`f.Stat()` segue o link). É regressão na direção restrita.
**Files affected:** `internal/validator/validator.go` (`walkADRFilePathsForRule`), `internal/validator/validator_adr_prefix_test.go`
**Actions:** quando `d.Type()&fs.ModeSymlink != 0`, decidir por `os.Stat(path)` (segue o link) e exigir
`Mode().IsRegular()`; senão, `d.Type().IsRegular()`. Link quebrado não conta.
**Acceptance criteria:**
- [x] Teste: symlink de arquivo `ADR-x.md` é enumerado; symlink de diretório `ADR-y.md` não; link quebrado não (skip em Windows se o symlink não puder ser criado)
- [x] O teste do symlink de arquivo reprova com o código atual (prova por overlay); uma frase por teste
      ✅ Auditoria: 3 testes de symlink PASS por nome; o de arquivo reprova com a condição Lstat (overlay). O de link quebrado passa nos dois (guarda, não prova de mordida). Sítio irmão em `validateADRFilesWithoutPrefix` (~:3189) → ML-1D.
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ -count=1
```

### ML-1D — Corretivo: o mesmo teste de "arquivo ou link para arquivo" no aviso D4
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Origem:** relatório do ML-1C. `validateADRFilesWithoutPrefix` (~:3189) ainda usa `d.Type().IsRegular()`: um `.md` sem prefixo que é symlink para um arquivo com status não dispara o aviso.
**Files affected:** `internal/validator/validator.go`, `internal/validator/validator_adr_prefix_test.go`
**Actions:** extrair o teste de tipo do ML-1C numa função única (ex.: `isRegularOrLinkToRegular(path, d)`) e usá-la nos dois sítios.
**Acceptance criteria:**
- [x] Teste: symlink sem prefixo para arquivo com `status:` dispara o aviso; reprova com a condição atual (overlay)
- [x] Os dois sítios chamam a mesma função
      ✅ Prova refeita pelo arquiteto, sabotando SÓ a chamada do aviso (~:3202): FAIL; original PASS. (O sed do relatório trocava também a declaração da função, e o FAIL podia ser de compilação.)
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ -count=1
```

## Wave 2 — Ponta a ponta com o binário
> Dependencies: Wave 1 auditada

### ML-2A — Fixture de três braços da #471 com o binário real
**Status:** ✅ Concluído
**Squad:** artemis-tf
**Files affected:** teste novo em `internal/commands/` (reusar o harness `e2eBinary`/`TRACKFW_E2E_BIN` de `branch_state_e2e_test.go`)
**Actions:** AC2, AC3, AC5, AC6 e AC7 com o binário; contra-braço com o binário de `44718ffc`.
**Acceptance criteria:**
- [x] Cada cenário reprova no binário de `44718ffc` e passa na branch (provar)
- [x] Uma frase por teste
      ✅ Contra-braço rodado pelo arquiteto: binário de `44718ffc` reprova AC2, AC5, AC6 e AC7; a branch passa os 5. O AC3 passa nos dois porque o critério antigo (todo `.md`) já contava `adr-001-x.md`: é guarda da direção restrita.
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -count=1
```

## Wave 3 — Revisão e gate completo
> Dependencies: Wave 2 auditada

### ML-3A — Revisão de qualidade e `make quality`
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-02-revisao-criterio-de-adr.md`
**Actions:** ponto único sem sobra de varredura própria de ADR; comentários que mentem; `make quality`
completo (se o paralelo de 8 chunks pendurar, rodar a falsificação em grupos e declarar).
**Acceptance criteria:**
- [ ] `make quality` EXIT=0 (ou falsificação em grupos com 0 FAIL, declarada)
- [ ] Veredito explícito

**Gates da wave:**
```bash
test -s docs/qualidade/2026-10-02-revisao-criterio-de-adr.md
```
