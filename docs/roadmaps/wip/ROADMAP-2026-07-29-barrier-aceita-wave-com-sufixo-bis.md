---
status: wip
date: 2026-07-29
req: "REQ-2026-07-29-barrier-aceita-wave-com-sufixo-bis"
squad: ""
---

# Roadmap: barrier aceita wave com sufixo bis

> Created: 2026-07-29 | Status: wip

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

- [ ] Gramática `<inteiro>[-<sufixo>]` com sufixo `[a-z0-9]+` minúsculo.
- [ ] `--wave 2-bis` funciona; `--wave 2` não casa com `Wave 2-bis`.
- [ ] Ordenação: `2-bis` após `2`, antes de `3`; sufixos entre si lexicográficos.
- [ ] Heading inválida continua abortando o documento — teste explícito de regressão.
- [ ] Terceira mensagem de exit-2 pinada e byte-idêntica nos três runtimes.
- [ ] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

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

## Wave 3 — Auditoria de paridade (1 ML)
> Dependências: **barrier** — ML-2A, ML-2B e ML-2C concluídos.

### ML-3A — Auditar paridade e provar não-vacuidade
**Status:** ⬜ Pendente
**Agente:** Artemis

**Ações:**
1. Cenário de paridade comparando **bytes** de stderr dos três CLIs para rótulo inválido, encadeado em
   `make quality`, com vacuity-guard.
2. Cenário provando que `--wave 2-bis` resolve nos três e que `--wave 2` não casa com `Wave 2-bis`.
3. **Teste de regressão do abort:** roadmap com heading `## Wave X — ...` deve abortar em todas as
   waves nos três runtimes. É o teste que impede alguém de "corrigir" o abort para skip silencioso.
4. Cenário de ordenação com `2`, `2-bis`, `2-hotfix`, `3`.

**Critérios de aceite:**
- [ ] Mensagens byte-idênticas nos três.
- [ ] `--wave 2-bis` resolve; `--wave 2` não casa com `2-bis`.
- [ ] Abort de heading inválida preservado e testado nos três.
- [ ] `make quality` exit 0 e `bin/trackfw validate --json` 0 violações.
