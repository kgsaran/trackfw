---
status: wip
date: 2026-09-18
squad: "hades-tf, apolo-tf, afrodite-tf"
---

# Roadmap: conclusão de microlote reimplementada por consumidor, e o scaffold placeholder chega a done sem gate que reprove

> Created: 2026-09-18 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-18-conclusao-de-microlote-reimplementada-por-consumidor-e-o-scaffold-placeholder-chega-a-done-sem-gate-que-reprove.md
ADR: docs/adr/ADR-2026-09-18-conclusao-de-microlote-tem-uma-implementacao-unica-num-pacote-folha-e-o-contorno-fecha-na-transicao-e-na-cobertura-do-gate.md

**Issue:** #392

O scaffold do `roadmap new` sobrevive por baixo do conteúdo e chega a `done/`. A causa não é a
ausência de um check: **"este ML está concluído?" é reimplementada por consumidor**, e o parser bom
está preso em `package commands`, inalcançável pelo `validator` por ciclo de import.

### Medições de 2026-09-18 (corpus real, não estimativa)

| medida | valor |
|---|---|
| roadmaps em `done/` | 192 |
| ...com ML não concluído (`StatusIsComplete`, canônico) | **27 (14%)** → base da rejeição da direção 1 |
| ...rejeitados por `ParseWaves` (rótulo `3-Py`, maiúscula) | **4** → total com fail-safe: 31 |
| ...com `## Wave 0` | **38** → por isso a exigência de Wave 0 NÃO retroage |
| ...com gate placeholder `exit 1` intacto | **4** |
| ...com rótulo de Wave/ML duplicado | **4** (+1 em `blocked/`) |
| união dos dois últimos em `done/` | **6** → corrigidos nesta REQ |
| roadmaps em `backlog/` com placeholder | **6** → legítimos, não cobrar |

> 🔴 **Nota de método.** A contagem de rótulo duplicado deu **0 em todos os estados** na primeira
> tentativa: o `awk` do macOS não suporta `ENDFILE` e a regra nunca disparou. Unanimidade num corpus é
> sinal de instrumento quebrado. Refeita com laço em `bash`, deu 4+1 e foi confirmada à mão em
> `ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal`, que tem o scaffold **inteiro** duplicado
> (linhas 13-60 e 78-103).

### Os quatro dialetos de "ML concluído?"

| sítio | como decide |
|---|---|
| `barrier` — `statusIsComplete` (`internal/commands/barrier.go:300`) | primeiro token, vocabulário fechado, fence-aware — **correto** |
| `serve` — `parseMLProgress` (`internal/serve/api_board.go:137-168`) | `Contains("✅")`, sem máscara de cerca — 🔴 o bug que a `ADR-2026-08-29` decidiu contra |
| `validate` — `contentHasMarker` (`validator.go:2312`) | só existência de heading — não responde |
| `scaffold` — prosa do slash command (`internal/generators/scaffold.go:350-384`) | **escreve** `**Status:** pending`, que não está no `statusVocabulary` |

**Arquivos afetados — só Go.** A v8 tem implementação única: `npm/src` não existe e
`pypi/trackfw/generators` tem zero `.py` versionado (medido). ADRs de jul/ago citam esses caminhos e
são **pré-v8** — não copiar delas.

## Acceptance Criteria

- [ ] AC1 pacote folha sem ciclo · AC2 identidade byte-a-byte do `barrier` · AC3 predicado reconciliado **27**/161 · **AC3-ter `WaveLabelRe` insensível a caixa**
- [ ] AC4 `serve` converge · AC5 `scaffold` converge **antes** do gate · AC6 `move done` recusa nomeando
- [ ] AC7 gate por **perda de cobertura** · **AC7-bis tier 1b (renomear heading)** · AC8 rótulo duplicado · AC8-bis não cobra em `backlog`
- [ ] AC8-ter os 6 sítios de `done/` corrigidos · AC9 `os.WriteFile` propaga erro · AC10 gates verdes · AC11 reconciliação

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat model
> Dependências: nenhuma. **Bloqueia toda wave de implementação.**

### ML-0A — quem esvazia este gate sem quebrar nenhuma regra escrita
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18) · **Papel:** `hades-tf`
**Files affected:** parecer em `docs/portabilidade/2026-09-18-threat-model-scaffold-residual.md` (somente leitura no restante do repo)
**Actions:**
1. **Completude da enumeração.** A lista de sítios que respondem "ML concluído?" está fechada em
   quatro (`barrier`, `serve`, `validate`, `scaffold`)? Não se limite aos arquivos nomeados: faça
   `grep` pelo literal `✅`, por `**Status:**` e por `Wave ` em todo `internal/` e `scripts/` antes de
   declarar a lista fechada. Nomeie o que faltar.
2. **Modelo de ameaça do gate novo.** Quem move um roadmap para `done` com ML pendente **sem quebrar
   nenhuma regra escrita**? Enumere ao menos: `mv`/`git mv` direto, editar o arquivo depois de mover,
   `roadmap move` para `abandoned` e de lá para `done`, e status com marcador de conclusão seguido de
   texto que o nega.
3. **Falsificação nas duas direções, por superfície.** Para cada gate proposto (AC6, AC7, AC8):
   o que quebra quando o comportamento regride, e o que quebra quando regride **ao contrário**
   (gate reprovando artefato legítimo)? 🔴 A direção do falso-positivo é a que o `ADR-2026-08-17`
   nomeia como a que faz o usuário desligar o guard.
4. **Curva de custo do atacante para o AC7**, no formato dos três tiers do ML-2C do #387: deixar
   `exit 1`, apagar o bloco de gates, e o que vier depois.
5. **Residual declarado** — o que este desenho aceita não cobrir.
**Acceptance criteria:**
- [ ] As cinco seções respondidas com evidência medida, não asserção de uma linha
- [ ] Nenhuma linha de implementação escrita neste ML
- [ ] O parecer nomeia explicitamente se algum sítio faltou na enumeração de quatro

**Gates da wave:**
```bash
test -f docs/portabilidade/2026-09-18-threat-model-scaffold-residual.md || { echo "parecer da Wave 0 ausente"; exit 1; }
grep -q "Residual declarado" docs/portabilidade/2026-09-18-threat-model-scaffold-residual.md || { echo "parecer sem secao de residual"; exit 1; }
```

---

## Wave 1 — Pacote folha (precede tudo, por decisão da ADR)
> Dependências: Wave 0 auditada. **ML-1A é sequencial consigo mesmo: o baseline vem antes da edição.**

### ML-1A — extrair o parser para `internal/roadmapdoc`, provando não-regressão
**Status:** ✅ Concluído **na extração**, mas com corretivo obrigatório no ML-1B — o teste de baseline lê o corpus vivo · **Papel:** `apolo-tf`
**Files affected:**
- **cria** `internal/roadmapdoc/roadmapdoc.go` + `internal/roadmapdoc/roadmapdoc_test.go`
- `internal/commands/barrier.go` (passa a importar; remove as funções movidas)
- `scripts/` — script de captura/comparação do baseline
**Actions:**
1. 🔴 **ANTES de editar qualquer linha**, capturar o baseline: para cada roadmap de `done/` + `wip/`
   e cada wave presente nele, rodar `trackfw barrier <path> --wave <n> --json --trust-local-gates` e
   gravar a saída. Commitar o baseline **neste passo**, antes da extração.
2. Mover para `internal/roadmapdoc`, sem alterar comportamento: `splitRoadmapLines` (barrier.go:172),
   `detectFenceMarker` (:321), `fenceMask` (:350), `waveBlock` (:392), `mlBlock` (:399), `parseWaves`
   (:408), `splitWaveLabel` (:447), `compareWaveLabels` (:464), `parseMLs` (:494), `mlStatusMarker`
   (:523), `statusIsComplete` (:300) e o `statusVocabulary` (~:216), `stripVS16` (:234),
   `hasDisallowedCombiningMark` (:263), `normalizeStatusToken` (:276), `acceptanceEvaluate` (:543),
   `parseGates` (:592), e os regexes de barrier.go:180-203.
3. Exportar o mínimo necessário; `internal/roadmapdoc` **não** pode importar `internal/commands` nem
   `internal/validator`.
4. `barrier.go` passa a chamar o pacote. Nenhuma mudança de mensagem, ordem ou exit code.
5. Re-rodar a captura e **comparar byte a byte** com o baseline.
**Acceptance criteria:**
- [ ] `go build ./...` RC=0, sem ciclo de import
- [ ] 🔴 Saída `--json` do `barrier` **byte-idêntica** ao baseline em 100% dos pares (roadmap, wave)
- [ ] `grep -rn "trackfw/internal/commands\|trackfw/internal/validator" internal/roadmapdoc/` → vazio
- [ ] `make test` RC=0 · `make quality` RC=0
- [ ] **Uma frase por teste novo** declarando qual conclusão deste ML ele afirma (AC11)
**Comandos de validação:** `go build ./... && make test && make quality`

### ML-1B — corretivo do ML-1A: o teste de baseline não pode ler o corpus vivo
**Status:** ✅ Concluído **no corretivo**, mas `make quality` REPROVA — o pin de corpus não foi atualizado; fecha com o ML-1C
**Files affected:** `internal/roadmapdoc/compare_baseline_test.go`, `internal/roadmapdoc/testdata/`,
`internal/roadmapdoc/roadmapdoc.go` (AC3-ter), `internal/roadmapdoc/roadmapdoc_test.go`
**Contexto da reprovação parcial:** a extração está correta e **fica** — auditei o AC1 (zero import de
`commands`/`validator`; o pacote só importa stdlib e `golang.org/x/text`), o AC2 (ver abaixo) e o AC3.
O defeito é o `TestParsingMatchesBaseline`: ele compara um baseline **congelado** contra o **corpus
vivo**. Eu editei uma linha de status do próprio roadmap desta REQ e o `make test` passou de RC=0 a
RC=2 em minutos. O ML-3A e o ML-4A tocam roadmaps por definição — o teste quebraria neles também.
🔴 É o mesmo defeito que o AC3 já evitava com fixture congelado; o congelamento foi aplicado aos
fixtures do predicado e **não** ao teste de baseline.
**Actions:**
1. Congelar em `testdata/` as cópias dos roadmaps que o baseline cobre, **ou** gravar no baseline o
   hash de cada arquivo e pular — **contando e reportando** — os que divergirem. Nunca pular em
   silêncio: um teste que ignora o que mudou não afirma nada.
2. **AC3-ter:** `WaveLabelRe` passa a aceitar o sufixo **insensível a caixa**, e o erro de `ParseWaves`
   deixa de ser silencioso. Os 4 roadmaps com `## Wave 3-Py` passam a parsear.
   Contra-braço: `## Wave abc` continua rejeitado.
3. Erro de `ParseWaves` **conta como pendência** (fail-safe) em qualquer predicado de gate. Nunca
   falhe aberto.
**Acceptance criteria:**
- [ ] Editar a linha de status de um roadmap qualquer do corpus **não** quebra `make test`
- [ ] Os 4 roadmaps de `## Wave 3-Py` parseiam; `## Wave abc` continua rejeitado (contra-braço)
- [ ] O teste **reporta** quantos arquivos pulou e por quê — zero pulos silenciosos
- [ ] `make test` RC=0 · `make quality` RC=0
- [ ] Uma frase por teste novo declarando o que ele afirma (AC11)

### ML-1C — corretivo do ML-1B: o pin de vereditos do corpus não foi atualizado
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18: `make quality` **640 OK / 0 FAIL**, diff do pin **append puro** 10/0 de um único arquivo)
**Files affected:** `scripts/testdata/roadmap-barrier-corpus-verdicts.tsv`, e o script que o pina
(`scripts/check-roadmap-barrier-contract.sh` ou equivalente — localize; não presuma o nome)
**Contexto da reprovação:** o ML-1B está correto no que entregou — auditei o congelamento do corpus
(sonda com `-count=1` num roadmap real: editar não quebra mais), os skips nomeados com piso de 450, e
o pin de exit-2 do `barrier`, que foi **preservado e não afrouxado** (`2-BIS` migrou para válido e
`abc` entrou como o inválido genuíno). O defeito é outro: `make quality` **reprova em 4 cenários**:

```
FAIL [corpus/exit2-count]: waves malformadas (exit 2): 10, pinado 14
FAIL [corpus/mls-complete-verdict-counts]: evidence=639 failure=118, pinado failure=113
FAIL [corpus/acceptance-evidence-verdict-counts]: evidence=314 failure=439, pinado failure=434
FAIL [corpus/non-reclassification]: hash da tabela mudou; 10 linhas novas (1300a1301,1310)
```

🔴 **O gate funcionou exatamente como projetado.** `corpus/non-reclassification` existe para pegar
mudança de comportamento não declarada no corpus, e pegou a nossa.

**Causa, medida por mim e consistente em todos os quatro números:** os 4 roadmaps de `## Wave 3-Py`
estavam **ausentes do pin** (`grep -c` → **0 linhas cada**, e **0** linhas com wave `3-Py`), porque o
`ParseWaves` os rejeitava por inteiro. Com o AC3-ter eles passam a parsear e geram vereditos: **+10
linhas**, **+5** failures em `mls_complete`, **+5** em `acceptance_evidence`, e as waves malformadas
caem de **14 para 10**. É o efeito **intencional** da decisão 11 da ADR.

**Por que o ML-1B não viu:** o `make quality` dele morreu antes, em `check-serve-address-parity.sh`
(`declare -A` sob bash 3.2 — falha **ambiental** do PATH dele, não do repositório; o shebang é
`/usr/bin/env bash` e o bash desta máquina é 5.3). 🔴 **Uma falha ambiental precoce mascarou uma
falha real posterior** — motivo pelo qual o arquiteto roda o gate por conta própria.
**Actions:**
1. Regenerar o pin `scripts/testdata/roadmap-barrier-corpus-verdicts.tsv`.
2. 🔴 **Auditar o delta linha a linha ANTES de aceitar**: as **10** linhas novas devem pertencer
   **exclusivamente** aos 4 roadmaps nomeados acima. Qualquer linha que venha de outro arquivo é
   reclassificação **não** explicada pelo AC3-ter e **reprova** — investigue, não re-gere.
3. Atualizar os três contadores pinados (exit2=10, mls_complete failure=118,
   acceptance_evidence failure=439) e o hash, **no mesmo commit** e com o motivo escrito.
4. Registrar no cabeçalho do `.tsv` (ou no script) **por que** o pin mudou, citando a decisão 11 da
   ADR — um pin que muda sem rastro é um pin que qualquer um re-gera na próxima falha.
**Acceptance criteria:**
- [ ] As 10 linhas novas pertencem **só** aos 4 roadmaps `3-Py` — demonstrado com o diff nomeado
- [ ] `make quality` RC=0, executado **até o fim** (não aborte na primeira falha ambiental)
- [ ] Se `check-serve-address-parity.sh` falhar por `declare -A`, **relate como ambiental e siga** —
      não altere esse script, está fora de escopo desta REQ
- [ ] O motivo da mudança do pin está escrito no artefato, não só no commit
- [ ] `make test` RC=0 · Uma frase por teste novo (AC11)

---

## Wave 2 — Consumidores convergem (2 MLs em paralelo)
> Dependências: Wave 1 (ML-1A a ML-1E) auditada. Arquivos disjuntos entre os dois MLs.
> 🔴 **Nenhum dos dois roda `make quality`.** Medido: `quality → parity → build` escreve
> `bin/trackfw`, e duas execuções simultâneas na mesma árvore corrompem o binário uma da outra.
> Cada ML roda `go build ./...` e `go test` do seu pacote; **o arquiteto roda `make quality` uma vez
> como barreira da wave**, depois dos dois.
> 🔴 **Nenhum dos dois edita arquivo transversal** (`docs/agents-working-context.md`,
> `.claude/agent-memory/`) nem executa `git add` — dois agentes na mesma árvore se atropelam, e um
> `git add -A` já commitou lixo nesta REQ.

### ML-2A — `serve` deixa de decidir por substring
**Status:** ✅ Concluído **com o corretivo ML-2C** — o AC4 fecha com as duas entregas somadas
**Files affected:** `internal/serve/api_board.go` (137-168) + teste
**Actions:**
1. `parseMLProgress` passa a usar `internal/roadmapdoc`: `fenceMask` + `parseMLs` + `mlStatusMarker`
   + `statusIsComplete`. Remover `strings.HasPrefix(trimmed, "### ML-")` e
   `strings.Contains(trimmed, "✅")`.
2. Teste que **falsifica o defeito atual**: roadmap com `✅` dentro de cerca de código e ML realmente
   pendente — hoje conta como concluído, depois não pode contar.
**Acceptance criteria:**
- [ ] O teste do item 2 **falha** contra o código atual e passa depois (demonstrar as duas execuções)
- [ ] `make test` RC=0 · `make quality` RC=0
- [ ] Uma frase por teste novo declarando o que ele afirma (AC11)

### ML-2B — `scaffold` ensina o vocabulário que o verificador aceita
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18: `pending` zerado em `scaffold.go` **e** em `.claude/commands/trackfw/roadmap.md`)
**Files affected:** `internal/generators/scaffold.go` (350-384) + teste
**Actions:**
1. Trocar `**Status:** pending` (scaffold.go:354) pelo valor que o gerador escreve —
   `⬜ Pendente` (`internal/generators/roadmap.go:73`).
2. Varrer o restante da prosa do scaffold por qualquer outro valor de status fora do
   `statusVocabulary` e convergir.
3. Teste de contrato: **todo** valor de `**Status:**` que o `scaffold` ensina satisfaz
   `roadmapdoc.StatusIsComplete` ou é um marcador de pendência do `Status Legend`.
**Acceptance criteria:**
- [ ] `grep -n "Status:\*\* pending" internal/generators/scaffold.go` → vazio
- [ ] Teste de contrato do item 3 verde
- [ ] `make test` RC=0 · `make quality` RC=0 · `trackfw doctor` sem mismatch
- [ ] Uma frase por teste novo declarando o que ele afirma (AC11)

**Gates da wave:**
```bash
grep -rn "strings.Contains(trimmed, \"✅\")" internal/serve/ && { echo "serve ainda decide por substring"; exit 1; }
grep -n 'Status:\*\* pending' internal/generators/scaffold.go && { echo "scaffold ainda ensina pending"; exit 1; }
exit 0
```

---

## Wave 3 — Gates (2 MLs em paralelo)
> Dependências: **Wave 2 completa e auditada** (barreira `make quality` 640 OK / 0 FAIL) — o AC5 exige
> que o `scaffold` convirja antes do gate, sob pena de todo roadmap autorado pelo slash command nascer
> bloqueado na transição.
> 🔴 **Nenhum dos dois roda `make quality`** (corromperiam `bin/trackfw` mutuamente); o arquiteto roda
> uma vez como barreira. **ML-3A não altera `internal/roadmapdoc/`** — consome o que já existe; se
> precisar de predicado novo, **para e relata**, porque o ML-3B é o dono do pacote nesta wave.

### ML-3A — `roadmap move ... done` recusa ML pendente, nomeando
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18: sonda ao vivo — o gate recusa o próprio roadmap desta REQ nomeando 4 MLs com linha)
**Files affected:** `internal/generators/roadmap.go` (`MoveRoadmap`, 524-605) + teste
**Actions:**
1. Antes do `os.Rename` (linha 572), quando o destino é `done`: parsear com `roadmapdoc` e coletar os
   MLs cujo status não satisfaz `StatusIsComplete`.
2. Havendo algum, **recusar** com erro que lista cada ML pendente por **rótulo e número de linha**.
3. **AC9:** trocar `_ = os.WriteFile` (581-585) por propagação do erro.
4. Testes: (a) roadmap com ML `⬜` → recusa nomeando; (b) **contra-braço** — roadmap legitimamente
   concluído → move normalmente; (c) falha de escrita na sincronização → erro propagado.
**Acceptance criteria:**
- [ ] Os três testes verdes, o (b) explicitamente nomeado como contra-braço
- [ ] A mensagem de recusa contém rótulo **e** linha de cada ML pendente
- [ ] `grep -n "_ = os.WriteFile" internal/generators/roadmap.go` → vazio
- [ ] `make test` RC=0 · `make quality` RC=0
- [ ] Uma frase por teste novo declarando o que ele afirma (AC11)

### ML-3B — cobertura de gate e rótulo duplicado, sensíveis ao estado
**Status:** ✅ Concluído **com o corretivo ML-3C** — as 3 regras ficam; a leitura crua foi trocada pela rota existente. AC7/AC7-bis/AC8/AC8-bis fecham com as duas entregas somadas
**Files affected:** `internal/roadmapdoc/` (predicados novos), `internal/validator/validator.go`
(**dois** sítios espelhados: `ValidateUnfiltered` ~758-946 **e** `validateUnfilteredTagged`
~1093-1289), `internal/validator/validator_test.go`
**Actions:**
1. Predicado **de perda de cobertura** em `roadmapdoc`: *a wave perdeu o gate que o template lhe deu*.
   🔴 **Não** implementar como "o texto do placeholder está presente" — `parseGates` trata wave sem
   `**Gates da wave:**` como zero gates, o que é legal, e o discriminante ingênuo liberaria o contorno
   de uma linha. É a lição literal do ML-2B do #387.
2. Predicado de **rótulo duplicado** de Wave/ML.
3. Registrar as regras nos **dois** sítios do validator (esquecer o `Tagged` faz a regra sumir do
   `--json`).
4. **Sensibilidade ao estado (AC8-bis):** cobra a partir de `wip`; **não** cobra em `backlog` nem
   `analyzing` — 6 roadmaps de `backlog/` carregam o placeholder legitimamente.
5. **AC7-bis (tier 1b):** roadmap em `wip`/`blocked` sem `## Wave 0` é violação; a exigência **não
   retroage a `done/`** — 154 dos 192 acenderiam. Teste: renomear `## Wave 0 — X` para `## X` num
   roadmap de `wip` → reprova; o mesmo em `done/` → não reprova.
6. Testes de falsificação do AC7, os **três** braços: (a) `exit 1` intacto → reprova;
   (b) **bloco de gates inteiramente apagado** → reprova; (c) gate substituído por comando real → não
   reprova. E do AC8: o fixture com duas `## Wave 0` passa hoje (`barrier.go:877-882` faz `break` no
   primeiro rótulo) e reprova depois.
**Acceptance criteria:**
- [ ] Os três braços do AC7 e os dois do AC8 verdes, cada um nomeado
- [ ] Regras presentes em `trackfw validate` **e** em `trackfw validate --json`
- [ ] Roadmap em `backlog` com `exit 1` não reprova; o mesmo arquivo em `wip` reprova
- [ ] `make test` RC=0 · `make quality` RC=0
- [ ] Uma frase por teste novo declarando o que ele afirma (AC11)

**Gates da wave:**
```bash
go build ./... || exit 1
grep -q "roadmapdoc" internal/validator/validator.go || { echo "validator nao usa o pacote folha"; exit 1; }
exit 0
```

---

## Wave 4 — Sanear os sítios medidos
> Dependências: Wave 3 completa — sem os predicados não há como provar que chegou a zero.

### ML-3C — corretivo do ML-3B: usar a rota de leitura que já existe
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18: `check-raw-read-ban` **OK, 0 raw sites, 91 linhas escaneadas** — não vácuo)
**Files affected:** `internal/validator/validator_roadmap_gates.go` — **só este**
**Contexto:** as três regras do ML-3B estão **corretas e ficam** — auditei os três braços do AC7 (o
braço (b), bloco de gates apagado, é o que mata o discriminante ingênuo e reprova), o AC7-bis com
`done/` fora, o AC8 com as linhas das duplicatas nomeadas (704/727, 709/754), o AC8-bis, e o registro
espelhado **29/29** em `applyRule`/`applyRuleTagged`.

🔴 **A barreira reprovou**, e a suíte abortou em 135 de 640:
```
FAIL [go] unjustified raw read at internal/validator/validator_roadmap_gates.go:56:
  raw, err := os.ReadFile(path)
```
`scripts/check-raw-read-ban.sh` existe desde a REQ do
`ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio`.

**A causa não é falta de comentário.** `internal/validator/validator.go:76` já tem
`readFileForRule(rule, path string, msgs *[]string) ([]byte, bool)`, que faz **exatamente** o que o
ML-3B escreveu à mão — lê e emite `inspectionDiagnostic` no erro — só que através de
`readRegularFile`, que **trata arquivo não-regular**. O `os.ReadFile` cru não trata.
Ou seja: a reimplementação é **pior**, não apenas não-conforme. É o erro que o `CLAUDE.md` global
nomeia — *reimplementar uma capacidade do produto pior do que ela é*.
**Actions:**
1. Trocar o `os.ReadFile` + tratamento manual por `readFileForRule("roadmap_gate_coverage", path, &gateMsgs)`.
2. Conferir se há **outra** leitura crua no arquivo — não pare na linha 56.
3. ❌ **Não** adicione `raw-read-allowed:` para calar o gate. O marcador existe para casos em que a
   rota não serve; aqui ela serve.
**Acceptance criteria:**
- [ ] `/usr/bin/grep -n "os.ReadFile" internal/validator/validator_roadmap_gates.go` → **vazio**
- [ ] Nenhum `raw-read-allowed:` acrescentado
- [ ] `go build ./...` RC=0 · `go test ./internal/validator/` RC=0
- [ ] Os 7 testes de integração do ML-3B continuam verdes (não-regressão)
- [ ] Uma frase por teste novo, se houver (AC11)

### ML-4A — sanear os sítios medidos E promover a severidade a `error`
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
**Files affected:** em `docs/roadmaps/done/` — `ROADMAP-2026-08-18-doctor-detecta-artefato-fora-do-manifesto...`,
`ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness...`,
`ROADMAP-2026-09-10-barrier-executa-gate-de-roadmap-nao-confiavel...`,
`ROADMAP-2026-09-11-serve-interpola-host-em-string-de-shell...`,
`ROADMAP-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado...`,
`ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal...`
(e `docs/roadmaps/blocked/ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz.md`)
**Actions:**
1. Em cada um: remover o **bloco de scaffold residual** duplicado, preservando integralmente o
   conteúdo autorado. 🔴 Na dúvida entre duas cópias, preservar a que tem conteúdo real e remover a
   que tem os placeholders do template — **nunca** o contrário.
2. Substituir o gate `exit 1` placeholder pelo gate real da wave, quando houver um; se o roadmap já
   está concluído e não houve gate, registrar isso explicitamente no bloco em vez de apagá-lo.
3. Não alterar status de ML nem marcar nada como concluído — este ML **remove duplicata**, não
   conclui trabalho.

#### 🔴 Decisão do arquiteto sobre a severidade (2026-09-18)

O ML-3B registrou as três regras novas com default **`warning`**, e escreveu o motivo no código:
*"three blocked/ roadmaps pre-date this rule and would fire immediately as violations (breaking AC10
before ML-4A cleans them)"*.

**Escolher a severidade que não reprova é funcionalmente um carve-out**, e a REQ do #387 inteira foi
sobre isso: severidade e leniência são a superfície de ataque. O precedente é desta mesma sessão —
`req_roadmap_sync` tem default `warning`, e foi **exatamente** por isso que a seção
`## Linked Roadmap` ausente da REQ do #387 passou despercebida até eu tropeçar nela. Registrei ali
que era *"lacuna de severidade, não de detecção"*. Repetir o padrão sabendo disso seria pior.

**Decidido:** as três regras vão a **`error`**, e os sítios são **corrigidos pelo conteúdo** neste ML
— não afrouxados. É o que o ML-3A da REQ do #387 fez com as 8 contradições ativas.

**Ordem obrigatória dentro deste ML:** sanear **primeiro**, promover **depois**, e entregar
`trackfw validate` **RC=0**. Promover antes deixaria o repositório reprovando no meio do ML.

#### Sítios a sanear — 9 roadmaps

**Em `done/` (6), scaffold residual / gate placeholder:** `ROADMAP-2026-08-18-doctor-detecta-artefato-fora-do-manifesto...`,
`ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness...`,
`ROADMAP-2026-09-10-barrier-executa-gate-de-roadmap-nao-confiavel...`,
`ROADMAP-2026-09-11-serve-interpola-host-em-string-de-shell...`,
`ROADMAP-2026-09-17-jira-base-url-...`, `ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal...`

**Em `blocked/` (3), medidos pelas regras novas com o binário já compilado:**
- `ROADMAP-2026-09-12-triagem-medida-das-reqs-de-paridade...` → sem `## Wave 0`
- `ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz` → Wave 0 sem bloco de gates **e** `ML-4A` duplicado (linhas 704/727) **e** `ML-4B` duplicado (709/754)
- `ROADMAP-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap...` → Wave 0 sem bloco de gates

**Também:** os 2 roadmaps com `## Wave reaberta` (`ROADMAP-2026-09-01-caminho-dentro-de-artefato-versionado...`
e o `fechar-os-grupos-de-falha-de-windows`) — renomear o rótulo para `<n>-reaberta`. É a metade de
**conteúdo** do AC3-ter, cuja metade de **código** o ML-1D já entregou.

**Acceptance criteria:**
- [ ] 🔴 Saneamento **antes** da promoção; `trackfw validate` **RC=0** ao final, com as 3 regras em `error`
- [ ] Predicados do ML-3B retornam **zero** em `done/` e em `blocked/`
- [ ] `git diff` mostra **apenas** remoção de scaffold e substituição de gate — nenhum `⬜ → ✅`
- [ ] `trackfw validate` sem violação nova · `make quality` RC=0
- [ ] Uma frase por teste novo, se houver (AC11)

### ML-1D — a gramática de rótulo é estreita demais, e o erro cascateia para o documento inteiro
**Status:** ✅ Concluído **com o corretivo ML-1E** — a gramática sem hífen, `SplitWaveLabel` e a equivalência de lookup **ficam**; a cascata foi **reprovada** por contrariar a `ADR-2026-07-29` decisão 16, e o AC7-bis fecha com as duas entregas somadas
**Files affected:** `internal/roadmapdoc/roadmapdoc.go`, `internal/roadmapdoc/roadmapdoc_test.go`,
`internal/commands/barrier.go` (lookup do rótulo, ~879), `internal/commands/barrier_test.go`,
`scripts/testdata/roadmap-barrier-corpus-verdicts.tsv`, `scripts/check-roadmap-barrier-contract.sh`

**Origem:** achado do ML-1C, **ampliado e verificado por mim**. Mesma causa do AC3-ter (gramática de
rótulo que não cobre o uso real), logo **mesma REQ** pela Regra Dura de Causa Raiz.

🔴 **Defeito 1 — cascata, e é o mais grave.** Medido por mim, não inferido:
```
$ trackfw barrier ROADMAP-2026-07-26-convergencia-do-harness... --wave 2
trackfw barrier: malformed wave heading at line 58: "1b" is not a valid wave label
EXIT=2
```
A wave `2` é **válida**. Um rótulo malformado em **outra** wave cega o `barrier` sobre o **documento
inteiro**. Não são "10 acertos diretos"; são 2 arquivos invisíveis por completo.

🔴 **Defeito 2 — gramática estreita demais.** Varredura minha em todo o corpus (não só no snapshot de
144 do gate): **4 arquivos**, em **duas formas**:

| forma | arquivos | natureza | onde corrige |
|---|---|---|---|
| `1b` (sufixo sem hífen) | `ROADMAP-2026-07-26-convergencia-do-harness...`, `ROADMAP-2026-08-16-serve-amarra-em-loopback...` | **gramática** | aqui, em código |
| `reaberta` (sem dígito inicial) | `ROADMAP-2026-09-01-caminho-dentro-de-artefato...`, `blocked/ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows...` | **conteúdo** | ML-4A, renomeando para `<n>-reaberta` |

**Precedente que governa:** `ADR-2026-08-22` (postura do validate diante de formas não reconhecidas)
decidiu que *"é uma regra, não uma lista de literais"* e nomeia **"condição estreita demais"** como
padrão recorrente deste projeto. `WaveLabelRe` já foi alargada uma vez hoje (caixa) e ainda não cobre
`1b` — é o mesmo padrão. A mesma ADR dá o princípio da cascata: forma não reconhecida se **isola e
nomeia**, não derruba o resto.

**Actions:**
1. **Parar a cascata.** Rótulo inválido invalida **aquela** wave, não o documento. As demais
   continuam avaliadas; a inválida é reportada **nomeadamente, com arquivo e linha**.
2. **Alargar a gramática** para aceitar sufixo alfanumérico **com ou sem hífen** (`1b`, `3-Py`,
   `10-a2`). 🔴 **Contra-braço inegociável:** rótulo **sem dígito inicial** (`abc`, `reaberta`)
   **continua inválido**. Não alargue para legitimar 2 arquivos — eles se corrigem como conteúdo.
3. **`SplitWaveLabel` sem hífen:** verificar que `SplitWaveLabel("1b")` devolve `(1, "b")` e não
   `(0, "")` nem a string inteira — senão `CompareWaveLabels` ordena `1b` antes de `1`.
4. **Lookup por igualdade exata** (`barrier.go:~879`: `waves[i].label == waveLabel`): com `1b` e
   `1-b` ambos válidos e **iguais** sob `CompareWaveLabels`, `--wave 1-b` contra `## Wave 1b` daria
   "wave not found". **Normalize no lookup ou declare como residual nomeado** — não deixe implícito.
5. **Atualizar o pin** com a mesma disciplina do ML-1C. Forma esperada **pré-declarada**: `exit2` cai
   de 10 rumo a 0; linhas são **acrescentadas** para as waves recém-parseáveis de
   `convergencia-harness` e `serve-amarra`; **zero** linhas preexistentes removidas ou alteradas.
   🔴 Remoção ou alteração significa roadmap já pinado reclassificado — **reprovaria o ML-1A
   retroativamente**. Nesse caso **pare e relate**.
**Acceptance criteria:**
- [ ] `barrier <convergencia-harness> --wave 2` avalia a wave 2 e **não** sai 2 por causa do `1b`
- [ ] A wave inválida é reportada nomeadamente, com arquivo e linha
- [ ] `1b`, `3-Py`, `10-a2` válidos; **`abc` e `reaberta` continuam inválidos** (contra-braço)
- [ ] `SplitWaveLabel("1b")` = `(1, "b")`; ordenação de `1` antes de `1b` demonstrada em teste
- [ ] O caso `--wave 1-b` vs `## Wave 1b` está resolvido **ou** declarado como residual nomeado
- [ ] Diff do pin é **append puro**; zero remoções ou alterações
- [ ] `go build ./...` RC=0 · `make test` RC=0 · `make quality` RC=0 até o fim
- [ ] Uma frase por teste novo declarando o que ele afirma (AC11)

### ML-1E — corretivo do ML-1D: heading malformada não pode virar warning de stderr
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18: vacuidade falsificada com os dois binários; `make quality` **640 OK / 0 FAIL**, conjunto de rótulos idêntico)
**Files affected:** `internal/commands/barrier.go` (bloco ~439-448 e a montagem dos checks),
`internal/commands/barrier_test.go`, `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`,
`docs/adr/ADR-2026-09-18-...md`, `docs/cli-parity.md`,
`scripts/testdata/roadmap-barrier-corpus-verdicts.tsv`, `scripts/check-roadmap-barrier-contract.sh`

🔴 **A falha de origem é do arquiteto, não do executor.** Eu afirmei, no handoff do ML-1D, que *"não
existe ADR anterior pinando a gramática de rótulo de wave — verificado"*. Verifiquei a **gramática** e
**não a cascata**. A `ADR-2026-07-29` tem **duas** decisões vivas sobre isto:

- **Decisão 15** pina a gramática: *"sufixo `[a-z0-9]+`"* — que o **ML-1B** alargou para `[a-zA-Z0-9]`
  **sem emendá-la**, e eu aprovei.
- **Decisão 16** pina a cascata, e a rejeição do que eu mandei fazer está **escrita lá**:
  > *"Durante a análise da emenda 15 considerou-se escopar o erro à wave solicitada, tornando as demais
  > headings malformadas inócuas. **Rejeitado.** Ignorar silenciosamente uma heading malformada faria
  > os MLs contidos nela deixarem de ser auditados: um typo (`## Wave X — ...`) produziria barrier
  > verde sobre trabalho não verificado. É a mesma vacuidade que a decisão 13 proíbe."*

O executor **fez o certo** ao declarar a revogação em vez de executá-la em silêncio — foi assim que eu
descobri.

**O defeito, confirmado no código:** `malformed` é impresso em `cmd.ErrOrStderr()` e **não entra em
nenhum check**. O veredito continua sendo função de `mls_complete`/`acceptance_evidence`/`gates`/
`validate`. O comentário do próprio executor admite: *"MLs inside malformed waves are **unreachable**
by wave-scoped barrier calls... Named residual."* Um typo `## Wave X` esconde MLs pendentes e o
`barrier` pode sair **passed**. É literalmente o cenário da decisão 16.

**O que FICA do ML-1D** (auditado e aprovado por mim): a gramática sem hífen (`1b`), o
`SplitWaveLabel("1b") = (1,"b")`, a equivalência `CompareWaveLabels("1b","1-b") == 0` resolvendo o
lookup, e o pin — **append puro, 33/0**, só dos 2 arquivos esperados.

**Síntese decidida por mim:** a decisão 16 está **certa no princípio** (*"deve reprovar alto"*,
nunca verde sobre trabalho não auditado) e **errada no remédio** (abortar o documento inteiro, que
cega o `barrier` em 2 roadmaps reais). O remédio correto satisfaz os dois:

**Actions:**
1. Heading malformada vira um **check próprio do `barrier`** — `wave_headings` — que entra no
   veredito e **bloqueia**. Nunca warning só em stderr.
2. As waves válidas **continuam sendo avaliadas** (a cegueira não volta). O `barrier` avalia tudo,
   reporta tudo e **não sai verde** enquanto houver heading malformada no documento.
3. O check nomeia **cada** heading malformada com linha e token, na mesma forma dos outros checks
   (`evidence`/`failures`), e aparece no `--json`.
4. **Emendar formalmente a `ADR-2026-07-29`**, nas decisões **15** (gramática alargada pelo ML-1B) e
   **16** (remédio revisto, princípio preservado), citando #392. 🔴 Contrariar ADR viva sem emenda é
   o defeito que originou este ML — não o repita ao corrigi-lo.
5. Atualizar o pin com a disciplina do ML-1C: delta **pré-declarado**, append puro, zero remoções ou
   alterações; se houver remoção, **pare e relate**.
**Acceptance criteria:**
- [ ] 🔴 **Falsificação da vacuidade:** roadmap com `## Wave 1` sadia **e** `## Wave X` escondendo um ML `⬜` → `barrier --wave 1` **NÃO sai passed**. Este teste tem de **falhar** contra o código atual do ML-1D — demonstre as duas execuções.
- [ ] Contra-braço: roadmap **sem** heading malformada e com tudo concluído → `passed` (o check não introduz falso-positivo)
- [ ] `barrier <convergencia-harness> --wave 2` **avalia** a wave 2 (a cegueira não volta)
- [ ] `wave_headings` aparece no `--json` com linha e token de cada heading malformada
- [ ] `ADR-2026-07-29` emendada nas decisões 15 **e** 16
- [ ] Diff do pin append puro; zero remoções
- [ ] `go build ./...` RC=0 · `make test` RC=0 · `make quality` RC=0 até o fim
- [ ] Uma frase por teste novo (AC11)

### ML-2C — corretivo do ML-2A: ML terminado infla o `total` do board
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18: sonda própria confirma 2/2 e o contra-braço 1/2)
**Files affected:** `internal/serve/api_board.go`, `internal/serve/api_board_test.go` — **só estes**
**Contexto:** a refatoração do ML-2A está **aprovada e fica** — os dois defeitos da `ADR-2026-08-29`
saíram, `roadmapdoc` é usado, e o teste de cerca falsifica o comportamento antigo (`done=1` → `done=0`).
O executor ainda declarou residuais que eu quero ver escritos: assimetria de VS16, "primeiro
`**Status:**` vence", e MLs fora de wave medidos por estado (**wip=0**, que é o que importa ao board).

🔴 **O defeito, medido por sonda minha:** `StatusTerminated` não incrementa `done`, mas o ML **continua
contando em `total`** (`api_board.go:179`).
```
roadmap com ML-1A ✅ Concluído + ML-1B ABANDONADO
→ total=2 done=1 → o board mostra 1/2, para sempre
```
Nunca chega a 100%, e exibe como pendente algo **encerrado por decisão**. Contradiz a **decisão 9 da
`ADR-2026-09-18`**, que eu mesmo escrevi: *"encerrado sem conclusão → **libera**... encerramento
explícito é uma decisão registrada, não um esquecimento"*.
**Actions:**
1. Escolher e justificar: **(a)** ML terminado sai do `total`; ou **(b)** conta como resolvido em
   `done`. Decidir **lendo o consumidor do JSON** — se o board exibe `done/total` como contagem de MLs
   reais, (b) preserva o registro de que o ML existiu; se o total é só denominador de progresso, (a)
   basta.
2. Implementar, com o motivo em comentário no código.
**Acceptance criteria:**
- [ ] 🔴 Teste que **falsifica o atual**: `1 ✅ + 1 ABANDONADO` hoje dá `1/2`; depois dá progresso
      completo. Demonstrar as **duas execuções**.
- [ ] Contra-braço: **`❌ Bloqueado` continua pendente** — é pendência, não encerramento (decisão 9).
      Roadmap com ML bloqueado **não** pode aparecer completo.
- [ ] `go build ./...` RC=0 · `go test ./internal/serve/` RC=0
- [ ] Uma frase por teste novo (AC11)
