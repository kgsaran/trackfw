# Revisão de Qualidade — Desacoplamento da Árvore de Governança (REQ #396)

> Revisor: hefesto-tf | Data: 2026-09-22 | Branch: fix/teste-e-gate-leem-a-arvore-de-governanca

## Veredito

**Aprovado com ressalvas.** Um achado deve ser corrigido neste PR (Q2). Dois achados devem ser
registrados como issues (Q1, Q4).

---

## Q1 — O `TestCorpusMeasurement_ReportOnly` ainda vale a pena existir?

**Posição: é um relatório com entrega parcialmente quebrada. Não é um teste no sentido útil.**

O CI principal roda `go test -timeout 2m ./...` sem `-v` (quality.yml step do job `go`, linha 31;
Makefile alvo `test`, linha 11). Em Go, `t.Logf` emite apenas quando o teste falha ou quando `-v`
está ativo. Este teste nunca falha.

**Medição neste repositório (layout flat, `done/` presente):**

```
$ go test -run 'TestCorpusMeasurement_ReportOnly' -v ./internal/roadmapdoc/
=== RUN   TestCorpusMeasurement_ReportOnly
    roadmapdoc_test.go:299: done/ corpus: total=194, unfinished(StatusIsComplete)=26, unfinished(HasUnfinishedMLs)=25
    roadmapdoc_test.go:301: Corpus measurement (REQ #396): count is logged for CI visibility only; ...
--- PASS: TestCorpusMeasurement_ReportOnly (0.07s)
RC=0
```

Com `-v` o output existe e é útil. Sem `-v` (caminho normal de CI), é descartado silenciosamente.

Os jobs que usam `-v` (Windows: linhas 375, 397, 530 do workflow) executam em layout `by_agent` sem
`docs/roadmaps/done/` flat — o teste faz `t.Skip` antes de medir qualquer coisa.

**Resultado:** o AC11 ("surfacing the count in CI logs for CI visibility") não é atendido no run
obrigatório. O teste não prejudica o produto (skip declarado no consumidor), mas também não cumpre
o que afirma.

**Ação recomendada:** registrar como issue. Opções: (a) converter para um step explícito em
`parity-other-gates` com `-run TestCorpusMeasurement_ReportOnly -v`, ou (b) deletar, visto que a
medição de baseline foi feita e documentada. Não bloqueia este PR.

---

## Q2 — A fixture do `validator_test.go` preserva o que importa?

**O discriminante da regressão original está presente e funcional. Mas a asserção é fraca — corrigir
neste PR.**

**Parte 1: o discriminante da regressão original (backtick vs. apenas aspas)**

A fixture preserva `adr: ""` com aspas duplas (não bare) e ADR exclusivamente via backtick no corpo.
Medição com Python do comportamento do trim antigo vs. novo sobre `fields[0]` dos três fixtures:

```
old_trim (sem backtick): "`docs/adr/ADR-2026-07-26-...verificaveis.md`"  → ends .md: False
new_trim (com backtick): "docs/adr/ADR-2026-07-26-...verificaveis.md"    → ends .md: True
```

Com o código antigo, `extractRefPath` encontra `adr: ""` no frontmatter (val strips para `""`, sem
sufixo `.md`, continua), depois encontra `ADR:` no corpo mas `fields[0]` retém os backticks e não
passa no `HasSuffix(".md")` → retorna `""`. O `t.Fatalf("extractRefPath não resolveu...")` dispara.
A regressão original é capturada.

**Parte 2: a asserção é fraca**

As asserções em `TestExtractRefPath_TresREQsReaisDoRepositorio`:

```go
if got == "" { t.Fatalf("extractRefPath não resolveu o ADR de %q", fix.name) }
if !strings.HasSuffix(got, ".md") { t.Errorf("ADR resolvido deveria terminar em .md") }
```

Não há `got != adrRef`. Um parser diferentemente-quebrado que retornasse qualquer caminho `.md`
passaria silenciosamente. Cada fixture tem um segundo caminho `.md` no frontmatter
(`roadmap: "docs/roadmaps/done/...md"`) — a chave `roadmap` não é `ADR`, então a implementação
atual não retorna esse valor. Mas a asserção não expressa isso; ela só verifica sufixo, não
identidade.

Mesmo gap em `TestExtractRefPath_CorpusBacktickREF`:
```go
if got == "" { t.Errorf("CORPUS REGRESSION: ...") }
```

**Correção requerida neste PR** (não modifica código de produto — modifica teste, que é escopo do
agente implementador):

```go
// em TestExtractRefPath_TresREQsReaisDoRepositorio (dentro do t.Run):
const adrRef = "docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md"
if got != adrRef {
    t.Errorf("extractRefPath(%q) = %q, want %q", fix.name, got, adrRef)
}

// em TestExtractRefPath_CorpusBacktickREF (dentro do t.Run):
// adicionar verificação de igualdade contra o valor esperado
```

**Sobre `TestExtractRefPath_CorpusBacktickREF`:** complementa sem duplicar — modos de falha
distintos (fixture pega regressão de código; corpus pega drift de arquivo real). Manter.

---

## Q3 — As três políticas do gate são necessárias, ou há formulação única?

**As três são necessárias.**

| Política | Variáveis | Invariante |
|---|---|---|
| `VARS_PIN_ALL` | `HASH_CMD_BIN`, `PYTHON_BIN` | Todo call site que invoca o consumidor deve pinar |
| `VARS_PIN_ANY` | `TRACKFW_SELF_GOVERNED` | Ao menos um call site deve pinar (design dual-site) |
| `VARS_FORBIDDEN_IN_QUALITY` | `TRACKFW_SELF_GOVERNED` | Nenhum call site em `make quality` pode pinar |

A terceira é o complemento negativo da segunda. A entrada "ML-1B-bis aprovado" no working context
documenta a medição: com apenas `VARS_PIN_ANY`, o pin reintroduzido em `parity-rest` saiu RC=0.
`VARS_FORBIDDEN_IN_QUALITY` é o que produziu RC=1. Sem ela, a regressão do ML-1B-bis passa
invisível.

Uma formulação unificada "exactly-one-upstream-site" exigiria `make -n self-governance` E
`make -n quality` com comparação de conjuntos — mais complexidade, com modos de falha menos
legíveis. As três políticas separadas têm mensagens de erro distintas que nomeiam o invariante
violado.

**Frase para o cabeçalho do script** (ausente hoje, recomendada para o próximo colaborador):
```
# PIN_ALL = pin em todo call site; PIN_ANY+FORBIDDEN = pin no upstream, ausente da cadeia quality
# (sem FORBIDDEN, pin-any passa mesmo com pin regredido para parity-rest — medido 2026-09-22).
```

**Sem ação necessária além da observação cosmética acima.**

---

## Q4 — O alvo `self-governance` está descoberto?

**Sim. O step pode ser removido do workflow sem que nada reprove.**

Verificação de todos os pontos de referência a `self-governance` fora de `docs/`:

```
$ grep -rn "self-governance" --include=*.sh --include=*.py --include=*.yml --include=Makefile . \
  | grep -v '^./docs/'

Makefile:5:    ... self-governance ...        ← declaração .PHONY
Makefile:144:self-governance: build           ← definição do alvo
scripts/check-parity-call-site-pins.sh       ← apenas comentários (7 ocorrências)
scripts/check-gates-falsify.sh               ← apenas comentário (1 ocorrência)
.github/workflows/quality.yml:834-835        ← único step que o invoca
RC=0
```

`check-parity-call-site-pins.sh` referencia `self-governance` **somente em comentários**. A lógica
funcional encontra o pin de `TRACKFW_SELF_GOVERNED` **dinamicamente** (varredura de recipe lines do
Makefile) — não verifica se o step do workflow existe. `check-orphan-gates.sh` também não detectaria
a remoção: `check-roadmap-barrier-contract.sh` continua sendo chamado por `parity-rest` (sem o pin),
e qualquer script com ao menos um call site não é "orphan".

**Consequência:** um refactor de workflow que deleta o step `make self-governance` faz `parity-other-
gates` passar, `parity` (required check) passar, e `check-parity-call-site-pins.sh` passar. A
tripwire de disco deixa de rodar em CI sem sinal de alerta.

**Como os outros gates críticos se protegem:** estão direto em `parity-rest`, que é call site
obrigatório da cadeia `parity → required check`. `self-governance` fica fora dessa cadeia por design
(é a feature), mas não tem proteção equivalente.

**Ação recomendada:** registrar como issue. Opção técnica: adicionar um self-test a
`check-roadmap-barrier-contract.sh` que seja invocado por `parity-rest` (sem o pin de ambiente),
verificando que a lógica da tripwire compile e execute — não seria a tripwire real, mas seria o sinal
de que a tripwire foi removida do CI.

---

## Q5 — Duplicação e deriva

**Um risco de manutenção identificado; nenhum duplicado do tipo que originou a REQ.**

Os loops `VARS_PIN_ALL` e `VARS_PIN_ANY` em `check-parity-call-site-pins.sh` são quase idênticos
(~30 linhas cada), diferindo apenas na condição final: `all invocations` vs. `pinned_count > 0`. A
distinção é semanticamente necessária (Q3), mas o próximo colaborador que alterar a lógica de
`find_consuming_scripts` terá de editar dois lugares. Risco: cópia que deriva silenciosamente — a
mesma classe do defeito original.

A REQ não introduziu duplicação do tipo helper-copiado: `check-parity-call-site-pins.sh` cobre um
invariante novo (relação call-site↔pin no Makefile) sem precedente no corpus de scripts existente.

O padrão "script + alvo Makefile + step no workflow" já existia antes desta REQ. O que esta REQ
acrescenta é um segundo alvo (`self-governance`) reutilizando o mesmo script com env var diferente —
não cria nova instância de duplicação no padrão.

**Sem ação necessária neste PR.** Orientação: se um quarto conjunto de variáveis for adicionado com
semântica `PIN_ALL`, considerar fatorar os dois loops idênticos antes da terceira instância.

---

## Q6 — O que eu bloquearia

**Um achado requer correção neste PR:**

- **Q2 — Asserção fraca na fixture:** `got != adrRef` está ausente em ambos os testes. A regressão
  original é capturada corretamente; uma regressão diferente (retorna `.md`-sufixo errado) não seria.
  Fix é uma linha por teste — escopo do agente implementador.

**Dois achados para registrar como issues:**

- Q1 — `TestCorpusMeasurement_ReportOnly` não emite em CI normal. Baixa severidade.
- Q4 — Step `self-governance` sem proteção contra remoção silenciosa. Severidade moderada.

---

## Resumo: corrigir neste PR vs registrar como issue

### Corrigir neste PR
1. **Q2 — Adicionar `got != adrRef` em `TestExtractRefPath_TresREQsReaisDoRepositorio`** (e
   asserção equivalente em `TestExtractRefPath_CorpusBacktickREF`). O teste entregue por este PR
   não verifica a identidade do resultado, apenas seu sufixo — o gap é real, o fix é uma linha,
   o PR já está aberto.

### Registrar como issue
2. **Q1 — `TestCorpusMeasurement_ReportOnly` com AC quebrado:** `t.Logf` suprimido no CI normal
   (`go test` sem `-v`). Converter para step com `-v` ou deletar.
3. **Q4 — Step `self-governance` sem proteção:** deletável em refactor de workflow sem reprovar
   nenhum gate. Opção: self-test invocado por `parity-rest`.

### Orientações (sem issue obrigatória)
- **Q3 cosmético:** adicionar no cabeçalho de `check-parity-call-site-pins.sh` a frase que explica
  por que FORBIDDEN não é redundante com PIN_ANY.
- **Q5 estrutural:** se quarto conjunto de variáveis for adicionado, fatorar os dois loops antes da
  terceira instância.
