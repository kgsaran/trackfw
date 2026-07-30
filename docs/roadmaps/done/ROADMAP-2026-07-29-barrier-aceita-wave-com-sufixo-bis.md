---
status: done
date: 2026-07-29
req: "REQ-2026-07-29-barrier-aceita-wave-com-sufixo-bis"
squad: ""
---

# Roadmap: barrier aceita wave com sufixo bis

> Created: 2026-07-29 | Status: done

## Contexto

REQ: `docs/req/REQ-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`

`trackfw barrier` rejeita `## Wave 2-bis` com `malformed wave heading`, e o erro **aborta as quatro
waves** do documento, não só a malformada: o parser varre todas as headings procurando a wave alvo e
levanta o erro antes de decidir se aquela heading interessa.

**Escopo negativo explícito:** este roadmap **não** relaxa a rigidez do parser. Heading fora da
gramática continua abortando o documento inteiro — ignorar silenciosamente uma heading malformada
deixaria seus MLs sem auditoria e produziria barrier verde sobre trabalho não verificado. Ver a seção
"Decisão de design que mudou durante a análise" na REQ.

## Critérios de Aceite

- [x] Gramática `<inteiro>[-<sufixo>]` com sufixo `[a-z0-9]+` minúsculo — regex pinada nos três, tabela
      de inválidos (`X`, `2-BIS`, `-bis`, `2-`, `2-bis-ter`, `0`) coberta em teste unitário nos três.
- [x] `--wave 2-bis` funciona; `--wave 2` não casa com `Wave 2-bis` — cenário
      `barrier/wave-label/bis-identity/{go,node,py}`, com vacuity-guard pelo campo `wave` do JSON, não
      apenas pelo exit code.
- [x] Ordenação: `2-bis` após `2`, antes de `3` — regra normativa no contrato; **sem call site em
      runtime nenhum**, portanto o comparador é opcional. Go tem `compareWaveLabels` coberto por
      testes; Node e Python declinaram corretamente em vez de shipar código morto. Registrado no
      contrato para ninguém "corrigir" a assimetria em nenhuma das duas direções.
- [x] Heading inválida continua abortando o documento — cenários
      `barrier/wave-label/malformed-{before,after}-target/{go,node,py}`. A posição **depois** é a que
      importa: era a célula que escondia o early-break do Python.
- [x] Mensagens de exit-2 pinadas e byte-idênticas nos três — a **terceira** (heading malformada) e a
      **quarta** (argumento `--wave` inválido), esta última descoberta despinada durante a Wave 2.
- [x] `make quality` exit 0 (34 cenários de barrier, 20 de falsificação, 13 gates não-vacuosos) e
      `bin/trackfw validate --json` 0 violações. Barrier das quatro waves: `passed`.
- [x] **Não-vacuidade provada por execução:** `BARRIER_BIS_SELFTEST_BREAK=1` faz o gate reprovar com
      `FAIL [barrier/wave-label/malformed-after-target/go]: expected exit 2 ..., got 0`. O seam corrompe
      a **fixture**, nunca a asserção — verificado pelo orquestrador.

## Mapa de dependências

```
Wave 1 — ML-1A (emenda do ADR + contrato, orquestrador)
   ↓ barrier — os três runtimes implementam contra o contrato congelado
Wave 2 — ML-2A (Go) ‖ ML-2B (Node.js) ‖ ML-2C (Python)   ← spawn simultâneo, arquivos disjuntos
   ↓ barrier — exige os três concluídos
Wave 3 — ML-3A (auditoria de paridade byte-a-byte)
```

Lição incorporada do roadmap anterior: o contrato do ML-1A lá pinou os **nomes** dos parâmetros e não
seus **valores**, e custou uma wave corretiva inteira. Aqui o ML-1A pina a gramática, a ordenação **e**
o texto literal da mensagem antes de qualquer implementação.

---

## Wave 1 — Emendar o ADR e congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Emendar o ADR e pinar a gramática de rótulo de wave
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Agente:** orquestrador (`trackfw_architect`) — autoria exclusiva, como no ML-6A do roadmap da barrier
**Arquivos afetados:**
- `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md` — emenda
- `docs/cli-parity.md` — seção `## trackfw barrier`: linha `--wave` da tabela de command surface,
  regra 1 de "Roadmap parsing rules", bloco de mensagens de exit-2 pinadas

**Ações:**
1. Emendar o ADR: `--wave` deixa de ser "Integer ≥ 1" e passa a aceitar rótulo.
2. Pinar a gramática: `^## Wave (\d+(?:-[a-z0-9]+)?) ` — inteiro, sufixo opcional minúsculo após
   hífen, seguido de espaço. A exigência do espaço final é preservada da regra 1 atual.
3. Pinar a ordenação: comparar primeiro o inteiro; em empate, rótulo sem sufixo precede rótulo com
   sufixo, e sufixos entre si comparam lexicograficamente.
4. Pinar a identidade: `--wave 2` casa **apenas** com `Wave 2`, nunca com `Wave 2-bis`.
5. Pinar literalmente a terceira mensagem de exit-2, escolhendo o texto do Go como base por já nomear
   a causa (Node hoje despeja a linha inteira sem motivo):
   ```
   trackfw barrier: malformed wave heading at line <n>: "<token>" is not a valid wave label
   ```
   `<token>` é o rótulo capturado, não a linha inteira. Registrar que o texto muda de
   `wave number` para `wave label`, e que essa é uma mudança observável de mensagem.
6. Registrar no contrato que abortar o documento inteiro é **intencional**, com a justificativa da
   vacuidade, para que nenhuma implementação futura o relaxe.

**Critérios de aceite:**
- [x] Gramática, ordenação, identidade e mensagem pinadas literalmente — seção
      `### Wave label grammar` em `docs/cli-parity.md`, com regex
      `^## Wave (\d+(?:-[a-z0-9]+)?) `, tabela de válidos/inválidos e regra de ordenação em 3 passos.
- [x] ADR emendado antes de qualquer implementação — decisões **15** (rótulo em vez de inteiro) e
      **16** (abort do documento é feature) em
      `ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`.
- [x] O caráter intencional do abort documentado, com o vínculo à decisão 13 (não-vacuidade).
- [x] Terceira mensagem de exit-2 pinada, com registro de que estava **despinada** e divergia nos três
      runtimes.

---

## Wave 2 — Implementar nos três runtimes (3 MLs em paralelo)
> Dependências: ML-1A completo. Arquivos disjuntos — **spawn simultâneo**.

### ML-2A — Go
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `internal/commands/barrier.go` (`waveHeadingRe` linha ~146, validação de
`--wave` linhas ~78-88, mensagem linha ~183), testes correspondentes

**Critérios de aceite:**
- [x] `--wave 2-bis` resolve `## Wave 2-bis`; `--wave 2` não.
- [x] Rótulo inválido aborta o documento, com a mensagem pinada.
- [x] `go build ./...`, `go test ./...`, `go vet ./...` passam.

**Evidência:**
- `go build ./...` — limpo (sem saída).
- `go test ./... ` — todos os 15 pacotes verdes, `internal/commands` em 8.8 s.
- `go vet ./...` — limpo (sem saída).
- `TestParseWaves_BisSuffix` PASS — label "2-bis" aceito.
- `TestParseWaves_LabelIdentityDistinct` PASS — "2" e "2-bis" são blocos distintos.
- `TestParseWaves_MalformedHeadingAbortsEntireDocument_Regression` PASS — `--wave 1`
  em documento com `## Wave X — ...` na linha 16 retorna exit 2, stderr byte-exato:
  `trackfw barrier: malformed wave heading at line 16: "X" is not a valid wave label`.
- `TestWaveLabelOrdering` PASS — incluindo caso discriminante 10 > 2 (numérico).
- Commit `751180b` na branch `feat/barrier-aceita-wave-com-sufixo-bis`.

**Observações reportadas ao orquestrador:**
1. `barrierResult.Wave` virou `string` → JSON emite `"wave":"1"` em vez de `"wave":1`.
   O exemplo `"wave": 2` em `docs/cli-parity.md` (seção `### JSON document`) diverge; ML-1A
   não atualizou esse bloco. Precisa de correção no contrato pelo orquestrador.
2. A mensagem de `--wave` inválido (`invalid --wave %q — not a valid wave label`) está despinada.
   Node emite `invalid --wave value: "…" (must be an integer >= 1)`, Python emite
   `malformed --wave value: …`. Todas três divergem. Requer pinagem antes de ML-3A.
3. `compareWaveLabels` foi implementado mas não é usado no fluxo de barrier (barrier não
   lista/ordena waves). Está disponível para uso futuro ou listagem de waves.

### ML-2B — Node.js
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `npm/src/commands/barrier.js` (`findWave` refatorado, `WAVE_SCAN_RE`,
`WAVE_LABEL_RE`, `isValidWaveLabel`), `npm/tests/barrier.test.js`

**Critérios de aceite:**
- [x] Comportamento equivalente ao Go.
- [x] Mensagem passa a nomear a causa e o token, não a linha inteira.
- [x] `cd npm && npm test` passa.

**Evidência:** `npm test` → 338 passed, 0 failed. Novos testes:
- `isValidWaveLabel`: tabela completa de válidos/inválidos.
- `findWave: resolves wave by label including suffix (2-bis)` — `--wave 2-bis` resolve.
- `findWave: --wave 2 does not match ## Wave 2-bis` — identidades distintas.
- `findWave: malformed error message contains the token` — token, não linha inteira.
- `findWave: REGRESSION — malformed heading aborts entire document` — decisão 16.
- CLI: `barrier regression: --wave 2-bis resolves ## Wave 2-bis heading at CLI level`.
- CLI: `barrier regression: --wave 2 does NOT match ## Wave 2-bis at CLI level`.
- CLI: `barrier regression: ABORT — malformed wave heading aborts entire document for every --wave value`.

### ML-2C — Python
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/commands/barrier.py` (`_WAVE_HEADING_RE`, validação de token
linha ~115, mensagem linha ~116), testes correspondentes

**Critérios de aceite:**
- [x] `--wave 2-bis` resolve `## Wave 2-bis`; `--wave 2` **não** casa com `Wave 2-bis`
- [x] Rótulo/heading inválido aborta o documento inteiro, com a mensagem pinada e exit 2
- [x] Aspas duplas na mensagem, não as aspas simples do `!r`
- [x] Teste de regressão do abort presente (`test_wave_heading_malformada_aborta_documento_inteiro`)
- [x] Suíte Python passa: 699/699 (`cd pypi && python3 -m pytest`)

**Evidência:**
- `_WAVE_HEADING_RE = re.compile(r"^## Wave (\d+(?:-[a-z0-9]+)?) ")` — gramática pinada
- `_ANY_WAVE_H2_RE = re.compile(r"^## Wave (\S+) ")` — detector de headings malformadas
- `_parse_wave_int` substituído por `_parse_wave_label` com `re.fullmatch` (previne aceitar `2-bis-ter`)
- Mensagem usa f-string com aspas duplas explícitas: `"{token}" is not a valid wave label`
- `doc["wave"]` agora é `str` em vez de `int` — nenhum teste da suíte assertava no tipo; `check-barrier.sh` não grepou o campo — mudança sem impacto observável externo
- 6 novos testes adicionados em `pypi/tests/test_barrier.py`

---

## Wave 3 — Convergir Python e alinhar mensagens (2 MLs em paralelo, corretivo)
> Dependências: Wave 2 completa. Emenda do contrato feita. Arquivos disjuntos — **spawn simultâneo**.

**Origem:** auditoria do orquestrador **executando os três CLIs**, não lendo relatórios. Um dos três
relatórios afirmava preservar o abort e não preservava.

**Nota de nomenclatura:** esta wave corretiva **não** se chama `Wave 2-bis`, apesar de ser exatamente
o caso de uso da feature. Motivo: o Python ainda não trata o rótulo corretamente, e `make quality`
executa `check-barrier.sh` nos três runtimes — batizar a wave de `2-bis` codificaria o defeito
não-corrigido dentro do artefato de governança que controla a correção. Dogfooding fica para um
roadmap posterior, depois que o ML-3A provar que os três concordam.

### Divergências medidas empiricamente

**1. Python não aborta quando a heading malformada vem DEPOIS da wave alvo.**

| Posição da heading malformada | Esperado | Go | Node.js | Python |
|---|---|---|---|---|
| Antes da wave alvo | exit 2 | exit 2 | exit 2 | exit 2 |
| **Depois** da wave alvo | **exit 2** | exit 2 | exit 2 | **exit 1 `blocked`** |

Causa: `_find_wave` sai do laço ao encontrar a wave pedida, então a heading posterior nunca é
visitada. É o mesmo early-break que o Node corrigiu no próprio ML-2B. **Viola duas decisões do ADR**:
a 16 (abort é feature) e a 12 (roadmap malformado nunca deve ser lido como "wave reprovada", porque
mascara o defeito real).

O teste de regressão do ML-2C cobre **apenas** a posição "antes" — passa enquanto o bug sobrevive.
Vacuidade parcial: o teste é real, a cobertura é incompleta.

**2. Mensagem de `--wave` inválido divergia nos três:**

| Runtime | Texto emitido |
|---|---|
| Go | `trackfw barrier: invalid --wave "2-BIS" — not a valid wave label` |
| Node.js | `trackfw barrier: invalid --wave value: "2-BIS" (must be a valid wave label, e.g. 1, 2-bis)` |
| Python | `trackfw barrier: malformed --wave value: "2-BIS" is not a valid wave label` |

Falha do contrato: pinei a mensagem de *heading* malformada e esqueci a de *argumento* inválido.
Agora pinada como a quarta mensagem de exit-2, adotando o texto do Go.

### Convergências que não exigem ação

- **Campo `wave` do JSON:** os três emitem string (`"wave": "1"`). A diferença de espaçamento
  (Go compacto vs Node/Python espaçado) é **normalizada** por `check-barrier.sh`, que não faz
  `sort_keys`. Não mexer.
- **Detector amplo + validador estrito:** os três convergiram espontaneamente para a estrutura de dois
  regexes. Agora pinada no contrato, incluindo a exigência de pré-passo completo.
- **Comparador de ordenação:** não há call site em runtime nenhum. Go tem `compareWaveLabels` coberto
  por testes; Node e Python declinaram corretamente em vez de criar código morto. Pinado como opcional.

### ML-2D — Corrigir o early-break do Python e alinhar sua mensagem
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/commands/barrier.py`, `pypi/tests/test_barrier.py`

**Ações:**
1. Mover a validação de heading para **pré-passo completo**: visitar todas as headings do documento
   antes de resolver o rótulo pedido, sem `break` antecipado.
2. Alinhar a mensagem de `--wave` inválido ao texto canônico do Go.
3. Teste cobrindo a heading malformada nas **duas** posições — antes e depois da wave alvo.

**Critérios de aceite:**
- [x] Heading malformada **depois** da wave alvo aborta com exit 2 e a mensagem pinada.
- [x] Heading malformada antes continua abortando (não regredir).
- [x] Teste cobre ambas as posições.
- [x] Mensagem de `--wave` inválido byte-idêntica ao Go.
- [x] Suíte Python passa — 701/701.

**Evidência empírica (duas posições):**

| Posição da heading malformada | Exit code | Stderr (byte-exato) |
|---|---|---|
| ANTES da wave alvo (linha 5) | 2 | `trackfw barrier: malformed wave heading at line 5: "X" is not a valid wave label` |
| DEPOIS da wave alvo (linha 13) | 2 | `trackfw barrier: malformed wave heading at line 13: "X" is not a valid wave label` |

**Mensagem `--wave` inválido:**
`trackfw barrier: invalid --wave "2-BIS" — not a valid wave label` (U+2014, byte-idêntico ao Go)

**Alterações em `barrier.py`:**
- Novo helper `_is_valid_wave_label(token)` — regra `fullmatch(\d+(?:-[a-z0-9]+)?) AND int >= 1` compartilhada por heading pre-pass e `_parse_wave_label`.
- `_find_wave` reescrito em dois passos: pré-passo completo (Fase 1) coleta todas as headings e aborta na primeira inválida; busca por label exata (Fase 2) sem break antecipado.
- `_parse_wave_label` usa `_is_valid_wave_label` e emite mensagem canônica com travessão U+2014.

### ML-2E — Alinhar a mensagem de `--wave` inválido no Node.js
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `npm/src/commands/barrier.js`, `npm/tests/barrier.test.js`

**Ações:** trocado `invalid --wave value: "<v>" (must be a valid wave label, e.g. 1, 2-bis)` pelo texto
canônico `invalid --wave "<v>" — not a valid wave label`. Separador U+2014 (`\xe2\x80\x94`), não hífen.
Adicionado teste `barrier regression: invalid --wave message is pinned literally (fourth exit-2 message)`
que verifica o texto byte-exato ao rodar o CLI com `--wave 2-BIS`.

**Critérios de aceite:**
- [x] Node.js emite `trackfw barrier: invalid --wave "<value>" — not a valid wave label`.
- [x] Go inalterado (já era o texto canônico, não foi tocado).
- [x] `npm test` passa — 339 passed, 0 failed.

**Evidência:**
- Node.js: `trackfw barrier: invalid --wave "2-BIS" — not a valid wave label`
- Go:      `trackfw barrier: invalid --wave "2-BIS" — not a valid wave label`
- Comparação xxd byte-a-byte: BYTE-IDÊNTICO (`\xe2\x80\x94` U+2014 em ambos).
- `npm test`: 339 passed, 0 failed — commit `b55393d`.

**Disjunção:** ML-2D toca só `pypi/`, ML-2E toca só `npm/`. Paralelizáveis. A mudança de mensagem do
Python foi absorvida pelo ML-2D justamente para evitar dois agentes no mesmo arquivo.

## Wave 4 — Auditoria de paridade (1 ML)
> Dependências: **barrier** — Waves 2 e 3 completas (ML-2A a ML-2E).

### ML-3A — Auditar paridade e provar não-vacuidade
**Status:** ✅ Concluído (2026-07-30)
**Agente:** Artemis

**Ações:**
1. Cenário de paridade comparando **bytes** de stderr dos três CLIs para rótulo inválido, encadeado em
   `make quality`, com vacuity-guard.
2. Cenário provando que `--wave 2-bis` resolve nos três e que `--wave 2` não casa com `Wave 2-bis`.
3. **Teste de regressão do abort:** roadmap com heading `## Wave X — ...` deve abortar em todas as
   waves nos três runtimes. É o teste que impede alguém de "corrigir" o abort para skip silencioso.
4. Cenário de ordenação com `2`, `2-bis`, `2-hotfix`, `3`.

**Critérios de aceite:**
- [x] Cenário de paridade cobre a matriz completa, com as **duas** posições de heading malformada
      (`scripts/check-barrier.sh` Cenários 8 e 9, `OK` confirmado nos três runtimes).
- [x] Vacuity-guard presente nos cenários novos (stderr não-vazio antes de byte-diff; wave field
      verificada no JSON para identidade).
- [x] Guarda de falsificação prova que o gate detecta a regressão do early-break: `BARRIER_BIS_SELFTEST_BREAK=1`
      em `check-gates-falsify.sh` Cenário 19 → `OK [falsify/barrier/early-break-after-target-not-detected]`.
- [x] Gramática inválida coberta nos testes unitários: Go `TestWaveLabelGrammar_ValidAndInvalid` (6 inválidos + 5 válidos
      via `parseWaves`), Python `test_is_valid_wave_label_tabela_completa` (6 inválidos + 5 válidos via
      `_is_valid_wave_label`), Node.js já completo antes de ML-3A.
- [x] `make quality` exit 0 (34 OK em `check-barrier.sh`, 20 OK em `check-gates-falsify.sh`,
      `bin/trackfw validate --json` 0 violações, `git status` limpo).

**Evidência real (2026-07-30):**
- Go: `go test ./internal/commands/ -count=1` → ok (8.7s)
- Python: `pytest pypi/tests/` → 702 passed (28.5s)
- Node.js: `npm test --prefix npm` → 0 fail
- `check-barrier.sh`: 34 OK incluindo Cenários 8–12 novos
- `check-gates-falsify.sh`: `OK [falsify/barrier/early-break-after-target-not-detected]`
- `Falsification checks passed (all 20 scenarios, 13 gates proved non-vacuous)`
- `bin/trackfw validate --json`: violations: []

---

## Matriz de verificação empírica do orquestrador (Wave 3)

Executando os três CLIs, não lendo relatórios. Todas as células byte-idênticas, incluindo o número de
linha na mensagem.

**Heading malformada × posição** (`## Wave X`, `--wave 1`):

| Posição | Go | Node.js | Python |
|---|---|---|---|
| Antes da wave alvo | exit 2, linha 7 | exit 2, linha 7 | exit 2, linha 7 |
| Depois da wave alvo | exit 2, linha 12 | exit 2, linha 12 | exit 2, linha 12 |

**Rótulo com sufixo:**

| Cenário | Go | Node.js | Python |
|---|---|---|---|
| `--wave 2-bis` num roadmap com `Wave 2` e `Wave 2-bis` | resolve, `wave: "2-bis"` | idem | idem |
| `--wave 2` no mesmo roadmap | resolve `Wave 2`, `wave: "2"` | idem | idem |
| `## Wave 0` | exit 2, mensagem pinada | idem | idem |
| `--wave 2-BIS` | exit 2, quarta mensagem pinada | idem | idem |

A identidade distinta está provada: `--wave 2` resolve a `Wave 2` e **não** a `Wave 2-bis`, apesar de
`2` ser prefixo de `2-bis`.

**Suítes (pós-ML-3A):** `go build`/`test`/`vet` limpos · `npm test` 0 fail · `pytest` 702 passed ·
`make quality` exit 0 (inclui `barrier/parity/three-runtimes-identical`, `barrier/usage-error/{go,node,py}`,
Cenários 8–12 novos em `check-barrier.sh`, e os 20 cenários de falsificação) · árvore limpa.
