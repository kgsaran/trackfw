# Wave 0 — Threat Model: critério caducado e cortes por data

> Roadmap: ROADMAP-2026-10-04-nao-ha-como-fechar-req…md | ML-0A | 2026-10-04

---

## 1. Completude — enumeração de parsers de caixa de critério

A varredura cobriu `internal/` e `scripts/`, excluindo `testdata/` e `_test.go`.

```
$ grep -rn '\[ \]' internal scripts | grep -v testdata | grep -v _test.go
$ grep -rn 'UnmetCriterionRe|CriterionLineRe|CriteriaHeaderRe|AcceptanceEvaluate' internal | grep -v _test.go
$ grep -rn 'wip_acceptance|no acceptance criteria block' internal | grep -v _test.go
```

### Parser P1 — `internal/roadmapdoc/roadmapdoc.go:615` (`AcceptanceEvaluate`)

O único parser que **conta** critérios para fins de governança. Usado por `barrier` (via P2)
e indiretamente invocado pelo `validate` por meio da regra `wip_acceptance` (P5).

```go
UnmetCriterionRe = regexp.MustCompile(`^- \[ \]`)   // linha 45
CriterionLineRe  = regexp.MustCompile(`^- \[.\]`)   // linha 46
```

Comportamento hoje com `- [ ]`: conta como `unmet`. Linhas de continuação (indentadas com
espaços, ex.: `      Caducou: texto`) não casam nenhum dos dois regexes e são **ignoradas** —
silenciosamente. Logo, o `Caducou:` indentado não muda nenhum contador hoje.

**Veredito: PRECISA reconhecer `Caducou:` (D2/D3).** O helper novo em `roadmapdoc` é o
único sítio a alterar; todos os chamadores herdam o comportamento corrigido.

### Parser P2 — `internal/commands/barrier.go:191` (`acceptanceEvaluate`, wrapper)

Delega integralmente para `roadmapdoc.AcceptanceEvaluate`. Adiciona a lógica de bloqueio e
o texto de saída (`"X unmet acceptance criteria"` em texto e JSON).

**Veredito: PRECISA reconhecer `Caducou:` — herda a correção do P1. Além disso, o texto e
o JSON precisam de campo novo (`lapsed`) conforme D3.** O contrato pinado em
`docs/cli-parity.md:2631-2632` e em `scripts/check-roadmap-barrier-contract.sh:984`
precisarão de atualização (ML-1A).

### Parser P3 — `internal/generators/roadmap.go:484` (`parseREQForRoadmap`)

Lê os critérios de um arquivo REQ apenas para **pré-preencher o wizard** ao criar um roadmap
`--from-req`. Extrai o texto do item `- [ ]` ou `- [x]` da mesma linha; a continuação
`      Caducou: …` (linha seguinte, indentada) não casa o prefixo `- [ ]` e é ignorada
silenciosamente.

**Veredito: NÃO precisa reconhecer `Caducou:`.** O comportamento atual é correto: o texto do
critério na linha `- [ ]` é copiado como template; a linha de continuação não vira um critério
autônomo. Nenhuma contagem de governança acontece aqui.

### Parser P4 — `internal/validator/validator_roadmap_gates.go:84` (`HasWave0` + `Wave0GateDiagnosis`)

Não conta critérios. Chama `roadmapdoc.HasWave0` para a regra `roadmap_wave0_required`
(apenas `wip/`) e, após o teste de presença, chama `Wave0GateDiagnosis` para a regra
`roadmap_gate_coverage`. Quando `HasWave0` retorna `false`, `Wave0GateDiagnosis` retorna
`Wave0GateOK` (linha 869: "Wave 0 nao encontrada — dominio do AC7-bis") — não há
duplo-reporte.

**Veredito sobre caixas: NÃO precisa reconhecer `Caducou:`.** Mas **PRECISA do corte D5**
(2026-09-18): um roadmap datado antes do corte que volte a `wip/` pela Regra Dura deve ser
isento da regra `roadmap_wave0_required` e da `roadmap_gate_coverage`.

### Parser P5 — `internal/validator/validator.go:2537` (`validateWIPHasAcceptanceCriteria`, regra `wip_acceptance`)

Verifica apenas se o roadmap tem o cabeçalho de critérios de aceite (`AcceptanceMarkers`). Não
conta critérios individuais, não chama `AcceptanceEvaluate`.

**Veredito: NÃO precisa reconhecer `Caducou:`.** Estrutural, não semântico.

### Parser P6 — `internal/generators/roadmap.go:755` (`pendingMLsForDone` + `HasWave0`)

Verifica o status dos MLs (não caixas de critério) e chama `HasWave0` como bloqueador do
`move … done`.

**Veredito sobre caixas: NÃO precisa reconhecer `Caducou:`.** O `barrier` (P2) é o gate
de caixas; o `move … done` faz gate por status de ML. Mas **PRECISA do corte D5** em `HasWave0`,
assim como P4.

### Parser P7 — `internal/serve/api_board.go:145` (`parseMLProgressFull`)

Conta MLs por status (`MLStatusMarker`) para o painel Kanban. Não chama `AcceptanceEvaluate`.
A renderização visual do markdown (`app.js:renderMarkdownSafe` via marked.js + DOMPurify) é
para exibição apenas — sem contagem de governança.

**Veredito: NÃO precisa reconhecer `Caducou:`.** O painel exibe o que o arquivo diz; a
contagem é de MLs, não de critérios.

### Scripts (check-barrier.sh, check-roadmap-barrier-contract.sh, check-gates-falsify.sh, check-validate-rule-pins.sh)

Nenhum faz parsing independente de caixas de critério. Todos delegam ao binário e fazem
asserções sobre o texto de saída. O `check-roadmap-barrier-contract.sh:984` pina a string
`"1 unmet acceptance criteria"` — precisará de atualização quando o `barrier` passar a
reportar `lapsed` separadamente (ML-1A).

**Veredito: NÃO precisam reconhecer `Caducou:` diretamente. O script `check-roadmap-barrier-contract.sh` precisará de atualização de fixtures em ML-1A para o novo formato de saída.**

### Lista fechada de parsers

| ID | Arquivo | Função | Conta critérios? | Precisa reconhecer `Caducou:`? |
|----|---------|--------|-----------------|-------------------------------|
| P1 | `internal/roadmapdoc/roadmapdoc.go:615` | `AcceptanceEvaluate` | Sim | **Sim (D2/D3)** |
| P2 | `internal/commands/barrier.go:191` | wrapper + saída | Via P1 | **Sim — herda P1 + texto/JSON** |
| P3 | `internal/generators/roadmap.go:484` | `parseREQForRoadmap` (wizard) | Não | Não |
| P4 | `internal/validator/validator_roadmap_gates.go:84` | `HasWave0`/Wave0Gate | Não | **Sim — precisa corte D5** |
| P5 | `internal/validator/validator.go:2537` | `wip_acceptance` | Não (estrutural) | Não |
| P6 | `internal/generators/roadmap.go:755` | `pendingMLsForDone`+`HasWave0` | Não | **Sim — precisa corte D5** |
| P7 | `internal/serve/api_board.go:145` | `parseMLProgressFull` | Não | Não |

A lista está fechada. Os dois chamadores de `HasWave0` que precisam do corte D5 são P4 e P6.

---

## 2. Threat Model

### T1 — `Caducou:` inválido que não deve contar (formas que o parser deve rejeitar)

A D2 exige linha de continuação **indentada** com **texto não-vazio** depois dos dois-pontos.
As formas abaixo foram medidas com o binário da `main` (`a481da42`). Todas hoje bloqueiam
como `unmet`. O comportamento correto após D3 está indicado na coluna "esperado pós-D3".

```bash
$ bash /private/tmp/…/scratchpad/fx514.sh   # medição base
# == 2 barrier wave 1:
#   - ML-1A: 1 unmet acceptance criteria
# result: blocked
```

| Forma | Hoje (main) | Esperado pós-D3 | Motivo |
|-------|------------|-----------------|--------|
| `- [ ] x` (sem continuação) | unmet / blocked | unmet / blocked | sem linha `Caducou:` |
| `Caducou:` (vazio, sem texto) | unmet / blocked | unmet / blocked | D2 exige texto não-vazio |
| `Caducou:` com indentação correta e texto | unmet / blocked | **lapsed / passed** | forma legítima |
| `Caducou:` sem indentação (coluna 0) | unmet / blocked | unmet / blocked | não é continuação |
| `Caducou:` em bloco de código (` ``` `) | unmet / blocked | unmet / blocked | fenced → ignorado |
| `<!-- Caducou: … -->` (comentário HTML) | unmet / blocked | unmet / blocked | não há mascaramento HTML |
| `Caducou:` após linha em branco | unmet / blocked | unmet / blocked | não é continuação direta |
| `- [x] x / Caducou: …` (critério já atendido) | met / passed | met / passed | parent é `[x]`, não `[ ]` |
| `- [ ] a / Caducou: v8 / - [ ] b` | 2 unmet / blocked | 1 lapsed, 1 unmet / blocked | `b` permanece unmet |

Medição confirmada para as primeiras seis formas:

```
$ bash /tmp/test_criterion_variants.sh (adaptado ao scratchpad)
[caducou-empty]:      - ML-1A: 1 unmet acceptance criteria  result: blocked
[caducou-with-text]:  - ML-1A: 1 unmet acceptance criteria  result: blocked
[caducou-no-indent]:  - ML-1A: 1 unmet acceptance criteria  result: blocked
[caducou-in-code-fence]: - ML-1A: 1 unmet acceptance criteria  result: blocked
[caducou-in-html-comment]: - ML-1A: 1 unmet acceptance criteria  result: blocked
```

Todas as formas inválidas bloqueiam hoje. A implementação precisa garantir que
continuem bloqueando (regressão zero) enquanto a forma canônica passe a liberar.

### T2 — `Caducou:` usado para esconder critério verificável

A D2 define `Caducou:` como texto livre; o produto verifica que a justificativa existe, não
que ela é verdadeira. Um autor pode escrever `Caducou: prefiro não testar` para um critério
perfeitamente verificável.

**Mitigação barata disponível:** o `barrier` com D3 deve imprimir a justificativa no
relatório (`ML-1A: 1 lapsed acceptance criterion — "prefiro não testar"`). O texto fica
visível para o revisor humano, que pode aprovar ou recusar no `barrier --trust-local-gates`.

**Veredito: resíduo declarado, com mitigação de visibilidade.** A ADR já declara
explicitamente: "o produto verifica que a justificativa existe, não que ela é verdadeira".
A mitigação de imprimir o texto é barata e está dentro do escopo do ML-1A.

### T3 — `[~]`, `[-]`, `[?]` como bypass pré-existente (mais barato que `Caducou:`)

`CriterionLineRe = ^- \[.\]` casa **qualquer** caractere único entre colchetes, incluindo
`~`, `-`, `?`, `!`, etc. Esses tokens contam como **met** hoje:

```bash
$ bash /tmp/test_criterion_variants.sh
[tilde-bracket-[~]]: result: passed    # [~] → met=1, passed
[dash-bracket-[-]]:  result: passed    # [-] → met=1, passed
[question-mark-[?]]: result: passed    # [?] → met=1, passed
```

Isso é um bypass pré-existente, mais simples que `Caducou:`: basta trocar `- [ ]` por
`- [~]` para que o `barrier` leia o critério como atendido, sem justificativa alguma.

A ADR menciona `[~]` como token rejeitado por "nos parsers que não o aprendessem, contaria
em silêncio como nem atendido nem pendente". A medição mostra que **já conta como met**, não
como neutro. O risco do ADR existia na direção errada.

**Veredito: pré-existente, fora do escopo desta REQ. Reportar como issue separada.**
A `Caducou:` proposta pela D2 é MAIS restritiva que `[~]`: exige o cabeçalho e a justificativa.
Se o objetivo de segurança é impedir bypass silencioso, o `CriterionLineRe` precisa ser
restrito a `^- \[[ xX]\]` antes ou junto com ML-1A. Sem isso, a D3 não melhora a postura
de segurança: `[~]` continuará passando sem texto algum.

### T4 — Critério HTML-comentado contado incorretamente

Não há mascaramento de HTML em `roadmapdoc`. Uma linha `- [x] crit` dentro de um bloco
`<!-- … -->` é contada como met:

```bash
[html-comment-forged-met]:   result: passed   # ← falso positivo
[html-comment-forged-unmet]: - ML-1A: 1 unmet acceptance criteria  result: blocked
```

O `- [x]` dentro de `<!--` é contado como met (falso positivo de evidência), e o `- [ ]`
dentro de `<!--` é contado como unmet (bloqueio espúrio).

**Veredito: pré-existente, não introduzido por esta REQ. Resíduo declarado.** A cli-parity.md
já documenta que o `barrier` é verificador sintático, não semântico. Não corrigir aqui.

### T5 — Data retroativa para D4 (regra `req_done_open_criteria`)

A régua de data é `date:` do frontmatter primeiro, nome do arquivo como fallback (mesma que
`reqCreationDate`). O campo `date:` é editável pelo usuário.

Uma REQ criada hoje com `date: 2026-09-01` no frontmatter seria lida como pré-corte (2026-10-04)
e ficaria isenta da regra D4.

```bash
$ # medido: validate passa (1 violation de harness, não de req_done_open_criteria)
$ [no-fm-date-date-in-name]: 2 warning(s) (harness warning apenas)
```

**Veredito: resíduo declarado pela própria ADR.** A D4 já escreve:
"Resíduo declarado: `date:` é editável. Um roadmap novo com data retroativa escapa da D5, e uma REQ nova com data retroativa escapa da D4."

O git fornece a data real de criação, mas está fora do escopo. Não há mitigação nova a propor.

### T6 — Data retroativa para D5 (`HasWave0` em `move … done` e `roadmap_wave0_required`)

Mesmo mecanismo que T5, mas para roadmaps. Um roadmap criado hoje com `date: 2026-09-01` (antes do
corte 2026-09-18) seria isento da exigência de Wave 0.

**Veredito: resíduo declarado pela própria ADR.** Idêntico a T5.

### T7 — REQ ou roadmap sem `date:` e sem data legível no nome (fail-closed e seus efeitos)

`reqCreationDate` retorna `(zero, false)` quando nenhuma das duas réguas produz data. O
chamador trata isso como pós-corte (fail-closed). Para **D4**, isso significa que a REQ seria
cobrada pela regra `req_done_open_criteria` — warning, não erro. O efeito é noise, não deadlock.

Para **D5**, o problema é mais grave:

`reqFilenameDateRe = ^REQ-(\d{4}-\d{2}-\d{2})` — âncora `^` e prefixo `REQ-`. Não casa:
- `ROADMAP-YYYY-MM-DD-slug.md` (prefixo errado)
- `slug-YYYY-MM-DD.md` (data no sufixo, sem prefixo REQ-)

Medição no acervo real:

```bash
$ find docs/roadmaps/done -name "*.md" | wc -l
  218

$ # 33 roadmaps em done/ com data no sufixo (ex: "docs-site-vitepress-2026-06-13.md"),
$   sem date: no frontmatter — todos datados 2026-06-11 a 2026-09-09 (< corte 2026-09-18)
$ # 4 roadmaps não-done sem Wave 0: TODOS têm date: no frontmatter → seguros
```

Para esses 33 roadmaps que estão em `done/`:

- **Hoje**: sem problema — já estão em `done/`, os gates de `HasWave0` no `move … done` já
  passaram.
- **Amanhã (Regra Dura)**: se qualquer um desses 33 for reativado em `wip/`, o
  `roadmap_wave0_required` disparará. Com a D5 implementada via `reqCreationDate` reutilizado
  sem adaptação, a data seria ilegível → fail-closed → "pós-corte" → Wave 0 exigida → **deadlock
  recriado para esses roadmaps**, exatamente o problema que a #514 quis resolver.

**Veredito: requer ajuste.** O implementador (apolo-tf) deve criar uma função
`roadmapCreationDate` (separada de `reqCreationDate`) com um regex que cubra:

1. `date:` no frontmatter (primária, já funciona)
2. `ROADMAP-YYYY-MM-DD` prefixo (para novos roadmaps)
3. `*-YYYY-MM-DD.md` sufixo (para os 33 roadmaps antigos)

A alternativa mais simples: regex genérico `(\d{4}-\d{2}-\d{2})` que extrai a primeira data
encontrada no basename. **Nota para ML-1B:** declare o regex escolhido no docblock, com
o número de roadmaps cobertos por cada ramo.

### T8 — Wave 0 com todos os critérios `Caducou:` (zero evidência real)

Após D3, se todos os critérios de um ML — inclusive os do ML-0A (Wave 0) — forem marcados
`Caducou:`, o `barrier --wave 0` passaria com zero critérios realmente atendidos:

```bash
$ # Pre-D3 (main):
[all-lapsed-baseline]: - ML-1A: 2 unmet acceptance criteria  result: blocked
[wave0-all-lapsed]:    - ML-0A: 2 unmet acceptance criteria  result: blocked

$ # Esperado após D3 sem mitigação:
# ML-0A: 0 criteria met, 2 lapsed  →  passed  ← zero evidência
```

Os critérios do Wave 0 são:
```
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML
```

Marcar ambos com `Caducou:` liberaria o `barrier --wave 0` sem que o modelo de ameaça
tenha sido escrito. A governança é a justificativa visível (T2), mas um revisor distante
pode aceitar sem verificar.

**Veredito: resíduo declarado com mitigação recomendada para ML-1A.** Mitigação barata:
o `barrier` pode exigir `met >= 1` quando `lapsed > 0` em Wave 0, ou imprimir alerta de
"wave 0 sem nenhum critério atendido". Deixar como requisito de Wave 2 (ML-2A).

---

## 3. Alvos de falsificação nas duas direções

| Superfície | Sítio de entrada | Gate que deve pegar | Direção FN | Direção FP |
|-----------|-----------------|--------------------|-----------|----|
| **P1/P2 — `Caducou:` inválido conta como lapsed** | `roadmapdoc.AcceptanceEvaluate` | `barrier acceptance_evidence` | `Caducou:` sem texto, sem indentação, em cerca, em HTML → libera indevidamente | critério legítimo com `Caducou:` bem-formado → bloqueia sem razão |
| **P1/P2 — `[~]`, `[-]` já contam como met** | `CriterionLineRe = ^- \[.\]` | `barrier acceptance_evidence` | qualquer char em `[.]` → libera sem `Caducou:` (FN pré-existente) | — |
| **P4/P6 — corte D5 data ilegível** | `roadmapCreationDate` (não existe ainda) | `roadmap_wave0_required` / `move … done` | data no sufixo não lida → fail-closed → Wave 0 exigida em roadmap pré-corte (FN: nega exemção legítima) | data retroativa → exemção concedida indevidamente (FP de exemção) |
| **D4 — `req_done_open_criteria` com data ilegível** | `reqCreationDate` | `validate req_done_open_criteria` | data ausente → fail-closed → warning em REQ antiga (FN: warning espúrio) | `date:` retroativo → exemção concedida a REQ nova (FP de exemção) |
| **D3 — all-lapsed Wave 0** | `roadmapdoc.AcceptanceEvaluate` | `barrier --wave 0 acceptance_evidence` | todos critérios lapsed → passed com zero evidência real (FN de evidência) | — |
| **D3 — `Caducou:` em após linha em branco** | `roadmapdoc.AcceptanceEvaluate` | `barrier acceptance_evidence` | blank line before `Caducou:` → `- [ ]` não lapsado (FN: `Caducou:` ignorado) | — |
| **HTML `- [x]` não mascarado** | `roadmapdoc.AcceptanceEvaluate` | `barrier acceptance_evidence` | `- [x]` em `<!-- -->` conta como met → libera sem evidência real (FN de evidência) | `- [ ]` em `<!-- -->` bloqueia indevidamente (FP de bloqueio) |
| **cli-parity.md e check-roadmap-barrier-contract.sh** | `scripts/check-roadmap-barrier-contract.sh:984` | `make quality` | teste pina `"1 unmet acceptance criteria"` → falha quando `barrier` reporta `lapsed` (FP de CI após D3) | — |

---

## 4. Resíduo declarado

O design aceita não cobrir os seguintes pontos:

**R1 — `date:` editável.** Tanto a D4 quanto a D5 dependem do campo `date:` do frontmatter,
que qualquer autor pode retrodar. O git tem a data real mas está fora do escopo. Declarado
explicitamente na ADR, seção Consequences.

**R2 — `Caducou:` texto livre.** O produto verifica que a justificativa existe, não que ela
é verdadeira. Um autor pode escrever qualquer texto após os dois-pontos. A mitigação de
visibilidade (imprimir o texto no relatório do `barrier`) reduz mas não elimina o risco.
Declarado explicitamente na ADR.

**R3 — `[~]`, `[-]`, `[?]` contam como met.** `CriterionLineRe = ^- \[.\]` aceita qualquer
caractere único. Isso é um bypass mais barato que `Caducou:` e pré-existente. O issue deve
ser aberto separadamente: sem corrigir o regex para `^- \[[ xX]\]`, a D3 não melhora a
postura de segurança para o atacante que quer evitar a exibição da justificativa.

**R4 — HTML `- [x]` e `- [ ]` não mascarados.** Critérios dentro de `<!-- -->` são contados
como se fossem reais. Pré-existente, documentado em `cli-parity.md` como limite do verificador
sintático.

**R5 — All-lapsed Wave 0.** Após D3, um ML-0A com todos os critérios `Caducou:`
passa o `barrier --wave 0` com zero evidência real. A mitigação (`met >= 1` obrigatório em
Wave 0, ou aviso explícito) foi recomendada para ML-2A — não implementar no ML-1A para não
ampliar o escopo.

**R6 — Roadmaps com data no sufixo sem `date:` no frontmatter trazidos de volta a `wip/`.**
33 roadmaps em `done/` com formato `slug-YYYY-MM-DD.md` e sem `date:` no frontmatter não são
cobertos pelo regex `^REQ-(\d{4}-\d{2}-\d{2})`. Se reativados em `wip/` pela Regra Dura, a
isenção D5 falhará (fail-closed) e o `roadmap_wave0_required` disparará para roadmaps que
têm data real anterior ao corte. Isso recriaria o deadlock da #514 para esse subconjunto.
**Este resíduo DEPENDE do implementador** — se `roadmapCreationDate` cobrir sufixo, o resíduo
fecha. Se reutilizar `reqCreationDate` sem adaptação, o resíduo se torna defeito.

---

*Hades, 2026-10-04*
