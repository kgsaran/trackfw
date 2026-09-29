# Parecer Wave 2 — ML-2A: auditoria independente por reimplementacao
**Roadmap:** ROADMAP-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-nunca-colhe-a-melhoria.md
**Data:** 2026-09-29 | **Auditor:** hades-tf

## Metodo

Nao li o diff do ML-1A. Li a ADR (incluindo Emenda 1 com D6/D7) e a REQ, derivei o
comportamento esperado e criei fixtures proprias no scratchpad. Exercitei o checker
(`scripts/check-windows-known-failures.py`) como caixa-preta.

Script de auditoria: `/private/tmp/.../scratchpad/hades-ml2a-audit.py`

---

## 1. Os tres baldes (D6 RECORTE) com fixtures proprias

**O que a ADR/REQ esperam:** D6 tem tres baldes, nao um. Baldes 1 e 2 ambos reprovam;
o que os distingue e a mensagem (atribuicao errada e o defeito que o D3 existe para evitar).

| balde | condicao | acao | mensagem |
|---|---|---|---|
| 1 (resolvido) | na lista E passou | reprova | "PASSED -- move to removed[]" |
| 2 (nao executou) | na lista, NEM passou NEM falhou | reprova | "neither failed nor passed -- skip, panic, or deleted" |
| 3 (ainda falha) | na lista E falhou | passa | silencioso |

### H1 — D5: teste renomeado (balde 2)

**Fixture:** `TestOld` na lista; go output mostra apenas `TestNew` (renomeado) e `TestAlpha`
(ainda falha). `TestOld` some das saidas sem aparecer em PASS nem em FAIL.

**Comando:**
```
run_check(LIST, GO_OUT, "", "")
# GO_OUT: "--- FAIL: TestAlpha (0.01s)\n--- PASS: TestNew (0.01s)\n"
# LIST: [TestOld, TestAlpha], ambos com reason ASCII
```

**Saida observada:**
```
ML-2A/2B: 1 observed / 2 active / 0 removed - DESEQUILIBRIO POR CLASSE. Go 1/2 [-1 resolvido], ...
::error::ML-D6: Go 'TestOld' neither failed nor passed -- skip, package panic, or deleted. Not a resolution. ...
```

**Veredito:** PASS. Balde 2 correto — nao diz "PASSED", diz "neither failed nor passed".
O renomear nao produz falso positivo de "resolveu".

### H2 — Python balde 1 (test passa)

**Fixture:** `test_foo.py::test_chmod` na lista Python; py output mostra `PASSED pypi/tests/test_foo.py::test_chmod`.

**Saida observada:**
```
::error::ML-D6: Python 'test_foo.py::test_chmod' is in the known list but PASSED -- move to removed[] ...
```

**Veredito:** PASS. Balde 1 Python correto, mensagem cita PASSED e removed[].

### H3 — Cenario misto: B1 + B3 + D1 simultaneos

**Fixture:** `TestResolvedNow` (passa), `TestStillFailing` (falha), `TestNewRegression` (nova).

**Saida observada:**
- `TestNewRegression` nomeado em D1 (::error::)
- `TestResolvedNow` nomeado em D6 B1 (::error:: com PASSED)
- `TestStillFailing` sem anotacao (balde 3, silencioso)

**Veredito:** PASS. Tres baldes segregados corretamente no mesmo run.

### H4 — Node suite-load-failure nao observada: sem discriminante de passou

**Fixture:** `broken.test.js` na lista como `suite-load-failure`; TAP sem nenhum bloco exitCode.

**Saida observada:**
```
::error::ML-D6: Node.js suite-load-failure 'broken.test.js' not observed -- file no longer fails
or package did not run. No pass discriminant for this class; investigate before removing.
(ADR D6 bucket 2: no pass set for suite-load-failure.)
```

**Veredito:** PASS. Sem discriminante, vai para balde 2. Nao diz PASSED.

---

## 2. Contra-braco do D1 — regressao nova continua reprovando

**H8 — D1 nao desativado por D6:**

**Fixture:** `TestKnown` na lista; go output mostra `TestKnown` PASSANDO e `TestBrandNewFailure` FALHANDO.

**Saida observada:**
```
::error::ML-D6: Go 'TestKnown' is in the known list but PASSED -- move to removed[] ...
::error::ML-2A ratchet: NEW Go assertion failure not in known list: 'TestBrandNewFailure'. ...
```
`rc = 1`

**Veredito:** PASS. D1 e D6 B1 coexistem. D6 nao desativou o D1.

---

## 3. D7 — campo `reason`

### H5 — reason nao-ASCII sob cp1252

**Fixture:** entry com `"reason": "Group B: CRLF normalizer — renderer path diverges"` (em-dash U+2014).

**Metodo:** substitui `_ANNOTATION_SINK` por um objeto com `.encoding = "cp1252"` e `encode(..., errors="strict")`.

**Saida observada:** `rc = 1`, `::error::` no sink com mensagem ASCII pura, sem `UnicodeEncodeError`.

**Veredito:** PASS. O _err() e ASCII-safe sob cp1252.

### H6 — D7 dispara antes de D6

**Fixture:** entry sem reason E que passou (seria B1 se D7 nao bloqueasse).

**Saida observada:** erro citando "D7"/"reason" — sem mencao a "PASSED" (D6 nao chegou a rodar).

**Veredito:** PASS. D7 tem early return em `run_check` antes das comparacoes do D6.

### H7 — removed[] sem reason nao dispara D7

**Fixture:** entries[] com reason, removed[] sem reason (forma das 24 entradas existentes).

**Saida observada:** `rc = 0`. Sem ::error::.

**Veredito:** PASS. D7 e opcional para removed[].

---

## 4. Verificacao do conteudo real das 14 entradas

### H9 — As 14 entradas todas tem reason ASCII

**Medido:**
- `len(entries) == 14`: PASS
- Nenhuma entrada sem reason: PASS
- Nenhuma reason nao-ASCII: PASS
- `len(removed) == 24`: PASS
- Nenhuma removed[] com reason (24 entradas legadas, sem campo): PASS

### H10 — MEASURED vs INFER: achado de inconsistencia de marcador

**O que a ADR afirma (Emenda 1):** "9 MEASURED / 5 INFER"

**Medido por contagem mecanica:**

| marcador | contagem |
|---|---|
| `(MEASURED)` estrito | 8 |
| `MEASURED` (incluindo variante) | 9 |
| `INFER` | 5 |

**Entrada com variante:**
```
TestStaleWIPReportsWIPWalkError:
  "Group A: ENOTDIR equals ERROR_PATH_NOT_FOUND on Windows ... (MEASURED in ADR-2026-09-05 Adendo)"
```

Esta entrada usa `(MEASURED in ADR-2026-09-05 Adendo)` em vez de `(MEASURED)`.

**Impacto no checker:** NENHUM. O checker so valida ASCII-only e presenca do campo.
Nao valida o formato do marcador. A entrada e aceita por D7 sem problema.

**Impacto no invariante do ADR:** A afirmacao "9 MEASURED / 5 INFER" e verdadeira pelo
sentido (evidencia existe na mensagem de falha ou no ADR citado), mas inconsistente pelo
marcador. Uma busca mecanica por `(MEASURED)` retorna 8.

**Severidade:** MENOR — documentacao, sem impacto funcional.

### H11 — Grupos A/B/C/D somam 14

**Medido:** A=4, B=4, C=2, D=4. Total=14. Concorda com ADR Emenda 1 (correcao de 13 para 14).

---

## 5. Self-test oficial

```
python3 scripts/check-windows-known-failures.py --self-test
Self-test summary: 53 PASS, 0 FAIL
```

T1-T32 (incluindo T28 D6 balde 1 e T29-T32 D7) todos PASS.

---

## 6. git diff trackfw.yaml

**Vazio.** Confirmado — nenhuma alteracao ao arquivo de configuracao da governanca.

---

## Veredito geral

**APROVA.** O D6 em tres baldes esta implementado corretamente:
- Balde 1 (resolveu) reprova dizendo PASSED — correto
- Balde 2 (nao executou: skip, panic, rename) reprova dizendo "neither failed nor passed" — correto
- Balde 3 (ainda falha) passa silenciosamente — correto
- D1 (regressao nova) continua reprovando simultaneamente com D6 — correto

O D7 (reason obrigatorio) esta implementado corretamente:
- Ausente reprova com mensagem ASCII-safe — correto
- Nao-ASCII reprova sem UnicodeEncodeError sob cp1252 — correto
- removed[] exempt de D7 — correto

**Achado menor (documentacao sem impacto funcional):** `TestStaleWIPReportsWIPWalkError`
usa `(MEASURED in ADR-2026-09-05 Adendo)` em vez de `(MEASURED)`. Contagem mecanica do
marcador estrito da 8, nao 9. A afirmacao da ADR "9 MEASURED / 5 INFER" e verdadeira
pelo sentido mas inconsistente pelo marcador. Nao requer correcao urgente.

**Sobrou caminho pelo qual uma melhoria passe despercebida?** NAO. O D6 agora reprova
em ambos os casos de "ausencia": tanto quando o teste passou (B1) quanto quando desapareceu
sem sinal de correcao (B2). A lista e autolimpante no sentido forte.
