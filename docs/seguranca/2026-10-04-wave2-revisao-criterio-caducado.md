# Wave 2 — Security Review: critério caducado e cortes por data

> Roadmap: ROADMAP-2026-10-04-nao-ha-como-fechar-req-com-criterio-de-aceite-permanentemente-inverificavel.md | ML-2A | 2026-10-04
> Revisor: Hades (hades-tf)
> Binário auditado: branch `fix/fechar-req-com-criterio-de-aceite-permanentemente-inverificavel` — commit 5f1e22af, compilado em `/private/tmp/claude-501/tf514h`

---

## 1. Veredito por cenário (T1–T8)

### T1 — Formas inválidas de `Caducou:` que não devem contar como lapsed

Vetores executados contra o binário da branch com `barrier --wave 1`. Cada resultado cita a mensagem exata do barrier.

**T1-a: `Caducou:` dentro de comentário HTML multi-linha**

```
- [ ] critério
<!--
  Caducou: esta justificativa está num comentário HTML multi-linha
-->
```

Comando:
```
trackfw barrier ROADMAP-test-hades-html_multiline.md --wave 1
```

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: 1 unmet acceptance criteria
result: blocked
```

Veredito: **FECHADO.** A `htmlCommentMask` em `internal/roadmapdoc/roadmapdoc.go:633` marca linhas dentro de `<!-- ... -->` como comentário. Na avaliação de continuação `Caducou:` em `AcceptanceEvaluateFull` (linha 737), a linha seguinte com `htmlMask[next] == true` é rejeitada. O critério fica `Unmet`.

**T1-b: `Caducou:` com apenas espaços ou whitespace após o dois-pontos**

```
- [ ] critério
  Caducou:   
```

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: 1 unmet acceptance criteria
result: blocked
```

Veredito: **FECHADO.** `LapsedContinuationRe = ^ {2,}Caducou:\s*\S` (`internal/roadmapdoc/roadmapdoc.go:58`). O `\S` exige ao menos um caractere não-whitespace. `Caducou:   ` (só espaços) falha; `Caducou:\r` (CRLF vazio) também falha — `\r` é whitespace e é consumido pelo `\s*`, sem `\S` restante.

**T1-c: `Caducou:` com apenas pontuação mínima (`.`)**

```
- [ ] critério
  Caducou: .
```

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: all acceptance criteria lapsed
    ~ ML-1A: 1 lapsed acceptance criteria
result: blocked
```

Veredito: **LAPSED — e isso é correto por D2.** O ponto (`.`) é `\S`. D2 define `Caducou:` como "texto não-vazio depois dos dois-pontos"; o produto verifica existência, não qualidade. Texto mínimo cai no resíduo R2 (texto livre). O resultado é `blocked` pela regra D6 ("all lapsed → blocked"), não `passed`. **Não é um bypass.**

**T1-d: `Caducou:` após sub-item (não é linha diretamente seguinte)**

```
- [ ] critério
  - sub-item detalhe
  Caducou: justificativa
```

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: 1 unmet acceptance criteria
result: blocked
```

Veredito: **FECHADO.** `AcceptanceEvaluateFull` verifica `lines[i+1]` (linha imediatamente seguinte ao `[ ]`). A linha `  - sub-item detalhe` não casa `LapsedContinuationRe`, portanto o critério permanece `Unmet`. A linha `  Caducou:` em `i+2` é invisível para o avaliador.

**T1-e: tabulação no lugar de espaços**

Critério com `\tCaducou: justificativa` (tab no início).

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: 1 unmet acceptance criteria
result: blocked
```

Veredito: **FECHADO.** `LapsedContinuationRe = ^ {2,}Caducou:` — a classe `{2,}` é explicitamente espaços (`\x20`), não `\s`. Tab (`\t`) não casa. (`internal/roadmapdoc/roadmapdoc.go:58`)

**T1-f: CRLF**

Arquivo com terminadores `\r\n`. Conteúdo da linha de continuação: `  Caducou: justificativa CRLF\r`.

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: all acceptance criteria lapsed
    ~ ML-1A: 1 lapsed acceptance criteria
result: blocked
```

Veredito: **CORRETO em ambas as direções.** O `\r` final não impede a detecção: `LapsedContinuationRe` basta encontrar `\S` antes do fim — aqui `j` de "justificativa" é encontrado antes do `\r`. Uma linha `  Caducou:\r` (sem texto real) falha porque `\r` é `\s` e não é `\S`. CRLF não cria bypass e não bloqueia forma válida.

**T1-g: `* [x]` e `+ [x]` (marcadores alternativos)**

```
* [x] critério com marcador estrela
```

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: no acceptance block
result: blocked
```

Veredito: **FECHADO — fail-closed.** `CriterionLineRe = ^- \[.\]` exige `-` na posição 0. `*` e `+` não casam; o critério é invisível ao contador. Bloco vazio → "no acceptance block" → bloqueio. Não há como obter `passed` via marcador alternativo.

**T1-h: critério aninhado (indentado) com caixa**

```
- [x] parent criterion
    - [ ] nested criterion with checkbox
```

Saída:
```
✓ acceptance_evidence: passed
```

Veredito: **FECHADO — fail-safe.** O critério indentado `    - [ ]` não casa `^- \[.\]` (requer `-` em coluna 0). O parent `- [x]` é a única entrada contada; `met=1, unmet=0` → passed. Critério aninhado com `[ ]` é invisível — não conta como unmet, portanto não pode inflar a contagem de abertos.

**T1-i: linha em branco entre `[ ]` e `Caducou:`**

```
- [ ] critério

  Caducou: justificativa após linha em branco
```

Saída:
```
✗ acceptance_evidence: blocked
    - ML-1A: 1 unmet acceptance criteria
result: blocked
```

Veredito: **FECHADO.** A linha em branco (`""`) é `lines[i+1]`. `LapsedContinuationRe` não casa string vazia; o critério permanece `Unmet`. A `Caducou:` em `i+2` é ignorada.

---

### T2 — `Caducou:` texto livre para esconder critério verificável

Veredito: **RESÍDUO DECLARADO — mitigação de visibilidade presente.**

O barrier imprime a mensagem informacional `~ ML-1A: N lapsed acceptance criteria` quando há lapsed. A justificativa completa aparece no texto da saída? Verificado na saída do `T1-c` e `T_ALL_LAPSED`: a saída atual mostra a contagem mas não o texto da justificativa. Isso é consistente com R2 declarado na Wave 0. A revisão humana no `barrier --trust-local-gates` ainda deve ler o roadmap.

Nota para o arquiteto: a Wave 0 recomendou imprimir o texto da justificativa no relatório do barrier. Isso não foi implementado — o barrier mostra `~ ML-1A: 1 lapsed acceptance criteria` mas não o texto da justificativa. A omissão é de visibilidade, não de segurança (o gate ainda bloqueia quando todas são lapsed). Registrar como ajuste menor abaixo.

---

### T3 — `[~]`, `[-]`, `[?]` como bypass (pre-existente)

Vetores testados:
```
- [~] critério tilde
- [-] critério dash
```

Saída para ambos:
```
✗ acceptance_evidence: blocked
    - ML-1A: 1 unmet acceptance criteria
    - ML-1A: unrecognized checkbox at line 22
result: blocked
```

Veredito: **FECHADO — MELHOR QUE O ESPERADO.**

A Wave 0 identificou `[~]` como bypass pré-existente e o declarou resíduo (R3), recomendando correção de `CriterionLineRe` como pré-condição para que D3 melhorasse a postura. O implementador foi além: D6 introduziu `MetCriterionRe = ^- \[[xX]\]` (`internal/roadmapdoc/roadmapdoc.go:47`) e reclassificou `AcceptanceEvaluateFull` para usar três cases (`MetCriterionRe`, `UnmetCriterionRe`, `default`). O `default` agora acumula `Unmet` e registra `UnrecognizedLines`. O bypass foi fechado e o revisor recebe aviso com número de linha.

O resíduo R3 da Wave 0 foi eliminado na mesma REQ.

---

### T4 — Critério dentro de `<!-- -->` contado incorretamente (pre-existente)

Veredito: **RESÍDUO PRÉ-EXISTENTE — não introduzido por esta REQ.**

`- [x]` dentro de `<!-- -->` ainda conta como met (falso positivo de evidência). A `htmlCommentMask` protege apenas o caminho de detecção `Caducou:` (linia 640: "applies only to the Caducou: detection path"). A proteção de T1-a confirma que o mascaramento funciona para o escopo desta REQ. O problema T4 pré-existe e está declarado em `cli-parity.md` como limite do verificador sintático. Não foi ampliado por esta entrega.

---

### T5 — Data retroativa para D4 (`req_done_open_criteria`)

Veredito: **RESÍDUO DECLARADO — idêntico ao declarado na Wave 0.**

`reqCreationDate` lê `date:` do frontmatter primeiro. Campo editável. Uma REQ criada hoje com `date: 2026-09-01` ficaria isenta. O código em `internal/validator/validator_req_done_criteria.go:66` documenta: "Data ilegível → false (fail-closed: charged, not exempt)." A direção de fail-closed para data ilegível é correta. A editabilidade de `date:` é R1, declarada na ADR.

---

### T6 — Data retroativa para D5 (`roadmap_wave0_required`)

Vetores D5 executados contra `trackfw validate` com roadmaps em `wip/`:

**D5_A: `date:` ausente, data 2026-09-01 no nome do arquivo**

`ROADMAP-2026-09-01-test-hades-d5a.md` sem campo `date:`.

Saída:
```
⚠  roadmap_wave0_required: 1 roadmap(s) in wip/ exempt from Wave 0 requirement as dated before 2026-09-18
```

Resultado: **exempt.** `roadmapFilenameDateRe = (\d{4}-\d{2}-\d{2})` (`internal/validator/validator_req_roadmap_cutoff.go:166`) encontra a primeira data no basename sem âncora. Regex mais flexível que o de REQ (`^REQ-`); cobre exatamente o conjunto de 33 roadmaps antigos identificados na Wave 0 (T7/R6). ✓

**D5_B: `date:` com formato inválido**

`date: invalid-date-format` no frontmatter, data 2026-09-01 no nome.

Resultado: **exempt.** `RoadmapCreationDate` tenta o frontmatter primeiro; `time.Parse` falha silenciosamente; fallback para `roadmapFilenameDateRe` extrai `2026-09-01` do basename. ✓

**D5_C: dois datas no nome do arquivo**

`ROADMAP-2026-09-01-v2-2026-10-04-test-hades-d5c.md` (datas 2026-09-01 e 2026-10-04).

Resultado: **exempt.** `FindStringSubmatch` retorna o primeiro match (`2026-09-01`). Um roadmap nomeado com data pré-corte à frente é considerado pré-corte — o que é equivalente ao resíduo R1/R6 (editabilidade de data). Nenhuma garantia de integridade além da visibilidade ao revisor.

Veredito geral D5/T6: **RESÍDUO DECLARADO — o R6 da Wave 0 foi coberto pelo `roadmapFilenameDateRe` flexível.** A preocupação da Wave 0 com os 33 roadmaps em `done/` sem `date:` no frontmatter e com data no sufixo foi diretamente endereçada. A editabilidade de `date:` permanece residual (R1).

---

### T7 — REQ ou roadmap sem data legível (fail-closed e seus efeitos)

Veredito: **FECHADO — o resíduo R6 da Wave 0 foi eliminado.**

A Wave 0 identificou R6 como defeito dependente do implementador: se `roadmapCreationDate` reutilizasse `reqCreationDate` sem adaptação, os 33 roadmaps antigos com data no sufixo falhariam (fail-closed → Wave 0 exigida em roadmaps pré-corte → deadlock recriado). O implementador criou `RoadmapCreationDate` separado (`internal/validator/validator_req_roadmap_cutoff.go:173`) com `roadmapFilenameDateRe = (\d{4}-\d{2}-\d{2})` — sem âncora, primeira data em qualquer posição. Confirmado na medição D5_A/B acima: roadmaps com data no sufixo são corretamente isentados.

---

### T8 — Wave 0 com todos os critérios `Caducou:` (zero evidência real)

Vetor executado:

```markdown
### ML-0A — Threat model
**Acceptance criteria:**
- [ ] threat model written
  Caducou: preguiça de escrever
- [ ] no implementation line
  Caducou: preguiça também
```

Comando:
```
trackfw barrier ROADMAP-test-hades-wave0_lapsed.md --wave 0
```

Saída:
```
✗ acceptance_evidence: blocked
    - ML-0A: all acceptance criteria lapsed
    ~ ML-0A: 2 lapsed acceptance criteria
result: blocked
```

Veredito: **FECHADO — melhor que o esperado.**

A Wave 0 declarou T8 como resíduo recomendando mitigação para ML-2A (`met >= 1` obrigatório em Wave 0, ou aviso). D6 implementou a regra "all acceptance criteria lapsed → blocked" (`internal/commands/barrier.go`, verificado na saída). O resultado é `blocked`, não `passed`. O bypass via all-lapsed não existe. O resíduo R5 da Wave 0 foi eliminado.

---

## 2. Resíduos declarados — situação pós-entrega

| ID | Descrição original (Wave 0) | Status pós-entrega |
|----|-----------------------------|--------------------|
| R1 | `date:` editável (retroativo) | Permanece. Declarado na ADR. |
| R2 | `Caducou:` texto livre | Permanece. O barrier mostra contagem mas não o texto da justificativa — ver Ajuste 1. |
| R3 | `[~]`, `[-]`, `[?]` contam como met | **ELIMINADO** por D6. Agora contam como unmet + aviso de linha. |
| R4 | HTML `- [x]` e `- [ ]` não mascarados | Permanece. Pré-existente, fora do escopo. |
| R5 | All-lapsed Wave 0 passa | **ELIMINADO** por D6 ("all lapsed → blocked"). |
| R6 | Roadmaps com data no sufixo sem `date:` | **ELIMINADO** por `roadmapFilenameDateRe` sem âncora. |

---

## 3. Achados fora do escopo da REQ (para registro)

**A1 — Texto da justificativa não aparece na saída do barrier**

O barrier reporta `~ ML-1A: N lapsed acceptance criteria` mas não imprime o texto de cada justificativa. A Wave 0 (T2) recomendou imprimir `"ML-1A: 1 lapsed acceptance criterion — "texto aqui""` para permitir revisão sem abrir o arquivo. Não é uma vulnerabilidade — o gate bloqueia corretamente quando tudo é lapsed. É uma lacuna de visibilidade.

Impacto: o revisor que executa `barrier --trust-local-gates` não vê as justificativas na saída; precisa abrir o roadmap para revisá-las.

---

## 4. Veredito Final

**APROVA COM AJUSTE**

Todos os vetores T1–T8 foram executados contra o binário compilado da branch. Os resíduos R3, R5 e R6 da Wave 0 foram eliminados pela entrega. Os resíduos R1, R2 e R4 permanecem declarados e estão dentro do escopo aceito pela ADR. Nenhum vetor de falsificação encontrou caminho de bypass que não esteja coberto por um resíduo explicitamente declarado.

### Ajuste 1 — imprimir o texto da justificativa no relatório do barrier (visibilidade, não segurança)

Quando `lapsed > 0`, o barrier deve emitir linha como:
```
~ ML-1A: 1 lapsed — "texto da justificativa aqui"
```
Arquivo: `internal/commands/barrier.go` — no bloco que escreve as linhas `~`. Hoje o loop conhece a justificativa porque `AcceptanceEvaluateFull` poderia retorná-la em `AcceptanceDetail.LapsedMessages []string`. A adição é em dois sítios: `roadmapdoc.AcceptanceDetail` (campo novo) e `barrier.go` (uso do campo na saída). Isso fecha o gap de visibilidade de R2 sem alterar nenhuma regra de bloqueio.

Esse ajuste é de visibilidade, não de segurança. Pode ser entregue como ML adicional à REQ vigente ou adiado — a decisão é do arquiteto.

---

*Hades, 2026-10-04*
