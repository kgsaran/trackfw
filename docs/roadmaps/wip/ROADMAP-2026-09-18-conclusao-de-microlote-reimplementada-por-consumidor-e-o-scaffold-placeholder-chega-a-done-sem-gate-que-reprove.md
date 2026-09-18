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
**Status:** 🔄 Em andamento · **Papel:** `apolo-tf`
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

---

## Wave 2 — Consumidores convergem (2 MLs em paralelo)
> Dependências: ML-1A auditado. Arquivos disjuntos entre os dois MLs.

### ML-2A — `serve` deixa de decidir por substring
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
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
**Status:** ⬜ Pendente · **Papel:** `afrodite-tf`
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
> Dependências: **Wave 2 completa** — o AC5 exige que o `scaffold` convirja antes do gate, sob pena de
> todo roadmap autorado pelo slash command nascer bloqueado na transição.

### ML-3A — `roadmap move ... done` recusa ML pendente, nomeando
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
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
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
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

### ML-4A — corrigir os 6 roadmaps de `done/` com scaffold residual
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
**Acceptance criteria:**
- [ ] Predicados do ML-3B retornam **zero** em `done/` e em `blocked/`
- [ ] `git diff` mostra **apenas** remoção de scaffold e substituição de gate — nenhum `⬜ → ✅`
- [ ] `trackfw validate` sem violação nova · `make quality` RC=0
- [ ] Uma frase por teste novo, se houver (AC11)
