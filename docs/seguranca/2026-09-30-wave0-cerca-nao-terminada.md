---
date: 2026-09-30
roadmap: ROADMAP-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md
wave: 0
ml: ML-0A
author: hades-tf
---

# Wave 0 — Parecer de Segurança: cerca não terminada

## Contrato, superfície, medição, veredito

---

## Premissa 1 — O PR #475/#470 não fecha isto

**Afirmação do KG:** o PR #475 (que faz `ParseWaves` consultar a máscara de cerca) não fecha este defeito.

**Verificação por mecanismo:** `ParseWaves` recebe a máscara como parâmetro (`roadmapdoc.go:402`
e signature de `ParseWaves`). Se a máscara sai errada (mascarando até EOF em vez de parar na
cerca fechada), `ParseWaves` opera sobre entrada corrompida. Corrigir a consulta da máscara não
conserta a máscara em si — é aritmética, não medição.

**Verificação por sonda:** o binário compilado do branch atual já contém o #475 (`make build`
passou). Fixture com cerca não fechada e ML pendente dentro da cauda:

```
braço C (cerca fecha)  → mls_complete: blocked | ML-1B: not complete (status: ⬜ Pendente)
                          acceptance_evidence: blocked | ML-1B: 1 unmet acceptance criteria
braço D (cerca NÃO fecha) → mls_complete: passed | acceptance_evidence: passed
```

Medido com `./bin/trackfw barrier fixture_fence_d.md --wave 1 --trust-local-gates --json`.
O rc nos dois braços foi 1 porque o projeto de fixture carrega 6 violations de validate — o
que muda é `mls_complete` e `acceptance_evidence`, não o rc final. A diferença chave é que braço D
NÃO emite a failure `"ML-1B: not complete (status: ⬜ Pendente)"` que braço C emite.

**Veredito:** PREMISSA SUSTENTADA. Causa diferente, separação autorizada pela Regra Dura.

---

## Premissa 2 — São 2 arquivos em 235

**Verificação independente:** reimplementação da regra de cerca em Python a partir do contrato
(não do Go), varrendo `os.walk`:

```
docs/roadmaps       236 arquivos .md   2 com cerca aberta
scripts/testdata/roadmap-barrier-corpus-snapshot   144 arquivos .md   1 com cerca aberta
```

Os dois em `docs/roadmaps`:
- `done/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md` — linha 460
- `done/ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-do-barrier.md` — linha 1044

O corpus snapshot tem o mesmo `ROADMAP-2026-08-22` (mesmo arquivo, cópia congelada) — terceiro
arquivo, fora do escopo do AC5 (que cobre `docs/roadmaps`), mas a Wave 1 vai fazer exit 2 ao
processar o corpus, o que vai quebrar CI se não for corrigido no mesmo PR. Isso precisa
estar no escopo da Wave 1.

**Discrepância de 1:** KG mediu 235; minha contagem é 236. Diferença: o roadmap desta REQ foi
criado em `docs/roadmaps/wip/` entre as duas medições. Contagem confirmada.

**Veredito:** PREMISSA SUSTENTADA (2 arquivos afetados em `docs/roadmaps`; 1 arquivo adicional
em corpus snapshot que a Wave 1 deve cobrir).

---

## Premissa 3 — A cauda mascarada é só prosa

**Verificação com checklist estendido:** além de `### ML-`, `**Status:**`, `- [ ]`, `## Wave `,
medi também `**Critérios`, `**Gates`, `**Owner`, `- [x]`, `**Acceptance`.

```
Arquivo 1 (wave-0, 8 linhas mascaradas):  ZERO marcadores governantes
Arquivo 2 (dialeto, 54 linhas mascaradas): ZERO marcadores governantes
```

Conteúdo real das caudas:
- Arquivo 1: `**Entrega completa.**`, `---`, `## Notas`, 2 linhas de bullet points
- Arquivo 2: análise de defeito de Node vs Go, recomendação de correção mínima, lista de residuais

**Veredito:** PREMISSA SUSTENTADA. O defeito existe e o mecanismo está ativo, mas nos dois
casos hoje ele não esconde trabalho de ninguém. Severidade atual: mecanismo pronto, dano diferido.

---

## Premissa 4 — Leniência não se aplica

**Verificação:** `--lenient` existe em `validate` e é passado como `Lenient bool` para o
validator. Nenhum trecho de código aplica leniência a erros de parsing (FenceMask, ParseGates,
ParseWaves, ParseMLs). A REQ afirma: "O modo lenient deste projeto existe para violations do
validate, não para usage error de entrada malformada."

Nenhum precedente de leniência para entrada malformada foi encontrado. A flag `--lenient` opera
exclusivamente sobre violations que o validador produz, não sobre erros de leitura de documento.

**Veredito:** PREMISSA SUSTENTADA. Leniência não muda o design.

---

## Regra 6 — estado de implementação por cláusula

Rule 6 (`cli-parity.md`, §Roadmap parsing rules): "A wave heading whose number is not parseable,
an ML whose body cannot be delimited, or an unterminated fence is a usage error (exit 2) with an
explicit message naming the offending line number — never a silent pass."

| Cláusula | Implementada? | Evidência |
|---|---|---|
| 1 — wave heading não parseável | SIM | `ParseWaves` → `MalformedWave`; barrier checa `wave_headings` |
| 2 — ML body cannot be delimited | LETRA MORTA | `ParseMLs` nunca retorna erro; sempre produz `MLBlock{Start, End}` |
| 3 — unterminated fence | PARCIAL | `ParseGates` detecta cerca aberta no bloco gates; `FenceMask` mascara em silêncio fora desse bloco |

A afirmação da REQ "cobre um caminho de dois" é tecnicamente precisa: o caminho coberto é o
da cerca de gates. A cláusula 2 não é implementada mas também não é acionável pelo parser atual
(não há cenário em que ParseMLs falhe em delimitar). Escopo desta REQ: cláusula 3, caminho geral.

---

## Pergunta 2 — Menor corte que cumpre a regra 6 sem quebrar os 10 sítios

### Os 10 call sites e seus callers

| # | Arquivo:linha | Função/contexto | Caller CLI |
|---|---|---|---|
| 1 | `barrier.go:163-164` | shim `fenceMask` (wrapper local) | `barrier` |
| 2 | `barrier.go:438` | uso do shim em `runBarrier` | `barrier` |
| 3 | `generators/roadmap.go:617` | `pendingMLsForDone` | `roadmap show` (done) |
| 4 | `generators/roadmap_show_json.go:128` | `buildRoadmapShowDoc` | `roadmap show --json` |
| 5 | `serve/api_board.go:168` | `parseMLProgressFull` | `trackfw serve` (HTTP) |
| 6 | `roadmapdoc.go:748` | `Wave0GateDiagnosis` | `validate` |
| 7 | `roadmapdoc.go:798` | `hasAnyNonPendingML` | `validate` (via Wave0GateDiagnosis) |
| 8 | `roadmapdoc.go:842` | `HasWave0` | `validate` |
| 9 | `roadmapdoc.go:866` | `DuplicateWaveOrMLLabels` | `validate` |
| 10 | `roadmapdoc.go:909` | `HasUnfinishedMLs` | `validate` |

### Menor corte proposto

**Não mudar a assinatura de `FenceMask`.** Mudá-la toca todos os 10 sítios de uma vez e obriga
a Wave 1 a lidar com cascata de erros em funções que hoje nunca falham.

**Adicionar `FenceMaskCheck(lines []string) (int, error)` em roadmapdoc** — mesma lógica de
`FenceMask`, mas retorna `(openLine 1-based, nil)` se a cerca fechou, ou `(0, error)` com a
mensagem no formato da regra 6 quando não fechou. `FenceMask` permanece inalterada.

Precedente de projeto para este padrão: `ParseGates` já faz exatamente isso — não muda a
assinatura dos callers, retorna `([]string, error)`.

Callers que precisam de mudança (5 sítios lógicos, não 10):

1. **`barrier.go`** — chamar `FenceMaskCheck` antes da avaliação; se erro, retornar
   `barrierUsageError` com o texto (que já sai como exit 2)
2. **`generators/roadmap.go`** (`pendingMLsForDone`) — chamar `FenceMaskCheck`; se erro,
   retornar erro ao caller de `roadmap show`, que sai exit 2
3. **`generators/roadmap_show_json.go`** (`buildRoadmapShowDoc`) — chamar `FenceMaskCheck`;
   o projeto já usa o padrão de colocar malformed waves no documento com
   "Fail-safe do lado de quem lê" (`malformed_waves` field) — o mesmo padrão se aplica aqui:
   adicionar um campo `unclosed_fence` ao JSON e retornar exit 2
4. **`serve/api_board.go`** (`parseMLProgressFull`) — chamar `FenceMaskCheck`; se erro, NÃO
   crashar — incrementar `MalformedFence` (novo campo no item) ou `MalformedWaves`, logar,
   continuar (ver tabela abaixo)
5. **`roadmapdoc.go` (funções internas 6-10)** — o padrão já existente em
   `hasAnyNonPendingML` e `HasUnfinishedMLs` é `if len(malformed) > 0 { return true }` —
   fail-closed em uma linha. Para cercas abertas, o mesmo: chamar `FenceMaskCheck` e retornar
   a posição conservadora (que já é o que cada função faz para malformed waves). Alternativamente,
   o `validate` pode chamar `FenceMaskCheck` uma vez por arquivo e gerar uma violation —
   mais limpo, porque isola a detecção da lógica de cada predicado.

---

## Pergunta 1 — Comportamento por superfície (tabela das 10)

### Medições

**barrier (sítios 1-2) — comportamento hoje:**
```
cerca aberta + ML pendente na cauda → mls_complete: passed, acceptance_evidence: passed
```
O ML é apagado da análise. O veredito é `passed` para aquele check.

**serve (sítio 5) — comportamento hoje, medido:**
```
fixture_fence_a.md (cerca fecha):    ml_total: 2, ml_done: 1, next_ml: "Wave 1 — test · ML-1B — segundo, PENDENTE"
fixture_fence_d.md (cerca NÃO fecha): ml_total: 1, ml_done: 1, next_ml: ""
```
O serve sub-reporta: 1 ML em vez de 2, mostra o roadmap como completo (1/1) quando há 1 ML
pendente escondido. Medido com `curl http://localhost:14902/api/board`.

**roadmap show --json (sítio 4) — comportamento hoje:**
```
malformed_waves: []  (vazio — não detecta a cerca aberta)
wave count: correto para os MLs ANTES da cerca
```
Nenhum sinal de malformação no output.

**validate (sítios 6-10) — comportamento hoje:**
```
trackfw validate → 0 violations, 2 warnings (warnings são de REQ sem ADR — não relacionados)
```
Validate não detecta cercas abertas nos 2 arquivos afetados.

### Tabela de comportamento proposto

| Sítio | Comando | Hoje | Proposto | Motivo |
|---|---|---|---|---|
| 1-2 | `barrier` | silent pass — ML mascarado desaparece | **exit 2** com mensagem nomeando a linha de abertura | Rule 6; barrier é a porta de governança; um ML invisível é pior que um erro de arquivo |
| 3 | `roadmap show` (done check) | silencia MLs mascarados | **exit 2** com mensagem | Rule 6; leitura de CLI, deve ser fiel |
| 4 | `roadmap show --json` | `malformed_waves: []`, contagem incompleta | **exit 2** + campo `unclosed_fence: {line: N}` no JSON | Rule 6 + precedente do projeto: `malformed_waves` já usa esse padrão para fail-safe em consumers JSON |
| 5 | `serve` (HTTP) | sub-reporta contagem de MLs, `ml_total` incorreto | **NÃO exit 2** — adicionar campo `malformed_fence: true` no item do board, logar, continuar | Servidor HTTP não pode crashar por um roadmap malformado no acervo; board é informativo, não é gate |
| 6 | `Wave0GateDiagnosis` → `validate` | mascara conteúdo após a cerca | fail-closed: checar antes e retornar `Wave0GateOK` FALSO (conservativo) | Validate é o lugar certo para surface a violation |
| 7 | `hasAnyNonPendingML` → `validate` | pode retornar false mesmo com MLs pendentes mascarados | retornar true (já é o padrão de `hasAnyNonPendingML` para `len(malformed) > 0`) | Fail-closed em uma linha; precedente imediato |
| 8 | `HasWave0` → `validate` | pode retornar false se Wave 0 vier após a cerca | retornar false conservativo; validate adiciona violation | HasWave0 já usa `ParseWaves` que ignora linhas mascaradas |
| 9 | `DuplicateWaveOrMLLabels` → `validate` | pula labels mascarados silenciosamente | checar e retornar violation de cerca aberta ao invés | A cerca aberta é ela mesma a violation mais séria |
| 10 | `HasUnfinishedMLs` → `validate` | pode retornar false para MLs mascarados | retornar true (mesmo padrão de `len(malformed) > 0`) | Fail-closed; precedente imediato em `HasUnfinishedMLs` |

### Decisão de produto aberta

O comportamento do `serve` (sítio 5) exige decisão sobre o nome e formato do campo de sinal:
`malformed_fence`, `malformed_waves` (reutilizar), ou `parsing_errors`. O parecer recomenda
reutilizar `malformed_waves` com um tipo de entrada diferente (ou um contador separado), porque
o consumer JavaScript do board já sabe lidar com `malformed_waves > 0`. Mas isso é decisão de
produto, não de segurança.

---

## Pergunta 3 — Conteúdo real das caudas mascaradas

### Arquivo 1: `done/ROADMAP-2026-08-22-wave-0-...ensina-trackfw-push.md`
Cerca abre na linha 460. Arquivo tem 468 linhas. 8 linhas mascaradas:

```
461: (vazio)
462: **Entrega completa.**
463: (vazio)
464: ---
465: (vazio)
466: ## Notas
467: - **Fora de escopo:** tudo listado no *Negative scope* da REQ.
468: - Commits, branch e PR são exclusivos do `trackfw_architect`.
```

ZERO marcadores governantes (`### ML-`, `**Status:**`, `- [ ]`, `## Wave `, `**Critérios`,
`**Gates`, `**Owner`, `- [x]`, `**Acceptance`).

### Arquivo 2: `done/ROADMAP-2026-08-29-dialeto-canonico-...barrier.md`
Cerca abre na linha 1044. Arquivo tem 1098 linhas. 54 linhas mascaradas.
Conteúdo: análise de defeito de divergência Node vs Go/Python, recomendação de correção mínima,
confirmação de residuais, lista de passos corretivos.

ZERO marcadores governantes (mesma checklist).

Nota: a linha 949 (`### ML-1A — ML nao concluida, mas libera a wave`) está DENTRO de uma cerca
diferente (abre antes de 949, fecha antes de 1044) — é exemplo documentado, não ML real. A
cerca aberta na linha 1044 é a última do arquivo e é distinta.

**Severidade atual: mecanismo ativo, dano diferido.** O defeito já age: os 2 arquivos `done/`
têm contagens artificialmente corretas porque a cauda mascarada é prosa pura. No dia em que uma
cauda tiver conteúdo governante, o dano é silencioso.

---

## Pergunta 4 — O que mais o FenceMask mascara em silêncio?

**Escopo:** esta pergunta é sobre o `FenceMask` geral (não a gramática de cerca em si —
qual linha fecha, fences indentadas, tilde vs backtick — que o escopo negativo da REQ exclui).

**Resultado:** `FenceMask` mascara tudo entre um abridor de cerca e o fim do arquivo quando
o fechador não vem. Isso é o único "silêncio" do `FenceMask` que importa para rule 6. Outros
comportamentos conhecidos:

1. **Cerca dentro de cerca:** não se aplica — CommonMark não aninha fences; o `FenceMask`
   atual está correto para o caso de outer-fence aberta vs inner fence (a inner não fecha a
   outer, por regra de caractere e comprimento).

2. **Gates fence** (`ParseGates`): tem detecção própria e independente — fora do escopo desta
   análise.

3. **HTML comments:** fora do escopo de `FenceMask` (são tratados em `thirdparty/markers.go`).

4. **Regra 6 cláusula 2** ("ML whose body cannot be delimited"): `ParseMLs` nunca retorna erro —
   sempre produz `MLBlock{Start, End}`. Esta cláusula é atualmente letra morta: não há caminho
   no parser que dispare "corpo não delimitável". Não é silêncio do `FenceMask`; é uma promessa
   do contrato sem scenario real no parser atual. A implementação deveria documentar que esta
   cláusula não é acionável ou criar o scenario.

---

## Acervo: terceiro arquivo (corpus snapshot)

`scripts/testdata/roadmap-barrier-corpus-snapshot/ROADMAP-2026-08-22-wave-0-...ensina-trackfw-push.md`

Este arquivo tem a mesma cerca aberta (linha 460, mesma cauda de prosa). Ele é uma fixture
congelada, fora do escopo do AC5 (que cobre `docs/roadmaps`). Mas quando a Wave 1 implementar
o exit 2, qualquer teste de CI que processe o corpus vai receber exit 2 neste arquivo.

**Recomendação:** a Wave 1 deve fechar a cerca no corpus snapshot no mesmo PR que os 2 arquivos
de `docs/roadmaps`. São 3 linhas de adição no total.

---

## Critérios de aceite do ML-0A — estado

- [x] Tabela das 10 superfícies com o comportamento proposto e o motivo de cada um
- [x] Resposta à pergunta 2 com o menor corte identificado
- [x] Recontagem independente dos arquivos de cerca aberta, e o conteúdo real das caudas
- [x] Veredito sobre o #470: mesma causa ou não, com a medição
- [x] Se a medição refutar qualquer premissa da REQ: dito (nenhuma refutada)
- [x] Nenhuma linha de implementação escrita neste ML

---

## Declaração de residuais

1. **Cláusula 2 da rule 6** ("ML whose body cannot be delimited") não é acionável pelo parser
   atual. Documentar como letra morta ou criar scenario é trabalho de doc, fora desta REQ.

2. **Corpus snapshot** contém um terceiro arquivo com cerca aberta. O AC5 da REQ cobre
   `docs/roadmaps` (2 arquivos). O terceiro arquivo precisa ser incluído no mesmo PR pela
   Wave 1, ou o CI vai quebrar ao processar o corpus.

3. **`roadmap show` (sem --json)** (sítio 3, `pendingMLsForDone`): a função é chamada para
   detectar MLs pendentes em roadmaps `done/`. O comportamento proposto (exit 2) é correto
   para CLI, mas a Wave 1 deve verificar se `pendingMLsForDone` tem outros callers além do
   `roadmap show` que precisariam do mesmo tratamento.

---

*Hades, 2026-09-30 — ML-0A Wave 0 concluído*
