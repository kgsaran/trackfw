# Wave 2 — Red Team: `check-needs-success.sh` e troca da guarda do `parity`

> Data: 2026-11-12 | ML-2A | Autor: hades-tf
> REQ: REQ-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins-e-deixa-passar-resultado-de-needs-nao-documentado.md
> ADR: ADR-2026-11-12-agregador-de-ci-exige-success-dos-needs-em-vez-de-reprovar-lista-de-valores-ruins.md

---

## 1. Vetores derivados do contrato (antes de ler o código)

Derivados exclusivamente do texto do ADR (D2) e da seção ML-1A do roadmap, antes de abrir
`scripts/check-needs-success.sh`.

| ID | Superfície | Hipótese de bypass ou FP |
|----|-----------|--------------------------|
| V-J1 | JSON `result` | Homóglifo em `result` (ex.: ѕuccess com Cirílico U+0455) |
| V-J2 | JSON `result` | NFC/NFD — string "success" com combinação normalizada diferente |
| V-J3 | JSON `result` | Número ou booleano no lugar de string (`1`, `true`) |
| V-J4 | JSON `result` | `null` como valor de `result` |
| V-J5 | JSON `result` | String vazia ou espaços (` success`, `success `) |
| V-J6 | JSON parsing | Chave duplicada top-level: primeiro `failure`, depois `success` |
| V-J6b | JSON parsing | Chave duplicada aninhada: `{"result":"failure","result":"success"}` |
| V-J7 | JSON parsing | BOM UTF-8 no início do JSON |
| V-J8 | JSON parsing | NaN/Infinity no JSON (Python `json.loads` aceita por padrão?) |
| V-J9 | NEEDS_JSON | Variável com só espaços |
| V-J10 | JSON `result` | Job sem chave `result`, ou valor do job não é dict |
| V-J11 | JSON escape | `success` = "success" após decode — legítimo ou vetor? |
| V-J12 | JSON `result` | `result` com espaço (` success`, `success `) |
| V-E1 | Ambiente | `NEEDS_JSON` não definida vs. definida-mas-vazia |
| V-E2 | Ambiente | cwd com `json.py` hostil — `python3 -` importa do cwd? |
| V-E2b | Ambiente | PYTHONPATH com `json.py` hostil |
| V-E3 | Ambiente | `python3` no PATH substituído por outro binário |
| V-E4 | Ambiente | `PYTHONSTARTUP`/`PYTHONHOME` hostis |
| V-E5 | Ambiente | `BASH_ENV` apontando para arquivo que faz `set +e` |
| V-E5b | Ambiente | `BASH_ENV` definindo `python3()` como função shell que retorna 0 |
| V-B1 | Bash flow | `--self-test` passado como primeiro argumento posicional |
| V-B2 | Bash flow | `--` como argumento |
| V-B3 | Bash flow | `set -e` + heredoc python3 — rc do python3 se propaga sem `set -e`? |
| V-B4 | Bash flow | `MODE=self-test` setado no environment antes de chamada em run mode |
| V-W1 | Workflow | Se `checkout` ou `setup-python` falham, o step do guard é pulado; job pode ficar verde? |
| V-W2 | Workflow | Step sem `if:` explícito tem `success()` implícito — semântica de pular ≠ falhar? |
| M1 | Self-test | Mutar allowlist para denylist que preserva happy path (`is str and r in (...)`) |
| M2 | Self-test | Remover checagem de conjunto `actual - expected` |
| M3i | Self-test | Todos os escapes de `esc()` removidos |
| M3ii | Self-test | Remover apenas `.replace('\r', '%0D')` |
| M3iii | Self-test | Remover apenas `.replace('%', '%25')` |
| M4 | Self-test | Substituir bloco `isinstance(needs, dict)` por `pass` |
| M5 | Self-test | Remover `exit 1` do check de `$# -eq 0` (mantendo o printf) |
| M6 | Self-test | Remover `-e` de `set -euo pipefail` |

---

## 2. Tabela de vetores medidos

Todos os comandos foram executados em `bash` explícito, com arquivos temporários em `mktemp -d`
com `trap 'rm -rf "$d"' EXIT`. O repositório não foi modificado.

| ID | Comando exato (simplificado) | rc | Saída relevante | Veredito |
|----|------------------------------|----|-----------------|---------|
| V-J1 | `NEEDS_JSON='{"a":{"result":"ѕuccess"}}' bash script a` | 1 | `result=ѕuccess (expected: success)` | Resistiu |
| V-J2 | `python3: unicodedata.normalize('{NFC,NFD}', 'success') == 'success'` | — | `True` (ASCII puro, sem forma especial) | Resistiu |
| V-J3 bool | `NEEDS_JSON='{"a":{"result":true}}' bash script a` | 1 | `result=True` | Resistiu |
| V-J3 num | `NEEDS_JSON='{"a":{"result":1}}' bash script a` | 1 | `result=1` | Resistiu |
| V-J4 | Coberto por arm 16 (null-result); `type(None) is not str` | 1 | — | Resistiu |
| V-J5 esp | `NEEDS_JSON='{"a":{"result":"success "}}' bash script a` | 1 | `result=success ` | Resistiu |
| V-J6 top | `NEEDS_JSON='{"a":{"result":"failure"},"a":{"result":"success"}}' bash script a` | 1 | `duplicate key in JSON object: a` | Resistiu |
| V-J6b aninhada | `NEEDS_JSON='{"a":{"result":"failure","result":"success"}}' bash script a` | 1 | `duplicate key in JSON object: result` | Resistiu — `reject_dupes` aplica-se recursivamente a todos os níveis |
| V-J7 BOM | `NEEDS_JSON=$'\xef\xbb\xbf{"a":…}' bash script a` | 1 | `Unexpected UTF-8 BOM` | Resistiu — fail-closed |
| V-J8 NaN `result` | `NEEDS_JSON='{"a":{"result":NaN}}' bash script a` | 1 | `result=nan (expected: success)` | Resistiu — Python aceita NaN; `type(r) is not str` captura; sem braço no self-test |
| V-J8 toplevel | `NEEDS_JSON='NaN' bash script a` | 1 | `top-level is not a JSON object (got: float)` | Resistiu |
| V-J9 esp | `NEEDS_JSON='   ' bash script a` | 1 | `not valid JSON: … col 4` | Resistiu |
| V-J10 no-dict | `NEEDS_JSON='{"a":null}' bash script a` | 1 | `result=None` | Resistiu |
| V-J11 escape | `NEEDS_JSON='{"a":{"result":"success"}}' bash script a` | 0 | `OK  a  result=success` | Legítimo — `s` = 's'; aprovação correta pela spec JSON |
| V-J12 espaco | `NEEDS_JSON='{"a":{"result":" success"}}' bash script a` | 1 | `result= success` | Resistiu |
| V-E1 unset | `(unset NEEDS_JSON; bash script a)` | 1 | `NEEDS_JSON is not set` | Resistiu |
| V-E1 vazia | `NEEDS_JSON='' bash script a` | 1 | `not valid JSON` | Resistiu |
| V-E2 cwd | `cd $d_com_json_py && NEEDS_JSON='{…abandoned…}' bash script …` | **0** | `OK … result=success` | **Bypass** — ver NF-1 |
| V-E2b PYTHONPATH | `PYTHONPATH=$d_com_json_py NEEDS_JSON='{…abandoned…}' bash script …` | **0** | `OK … result=success` | **Bypass** — ver NF-1 |
| V-E3 PATH | `PATH=$d_com_py3_falso NEEDS_JSON='{…abandoned…}' bash script …` | **0** | (sem saída) | Bypass R5 — ver §6 |
| V-E4 PYSTART | `PYTHONSTARTUP=$startup NEEDS_JSON='{…abandoned…}' bash script …` | 1 | `result=abandoned` | Resistiu — PYTHONSTARTUP não roda em modo não-interativo (stdin é pipe) |
| V-E4 PYHOME | `PYTHONHOME=$d_vazio NEEDS_JSON='{…abandoned…}' bash script …` | 1 | `Fatal: No module named 'encodings'` | Resistiu — crash do python3, fail-closed |
| V-E5 `set +e` | `BASH_ENV=$d/sete.sh NEEDS_JSON='{…abandoned…}' bash script a` | 1 | `result=abandoned` | Resistiu — script reaplica `set -euo pipefail` após BASH_ENV |
| V-E5b func | `BASH_ENV=$d/func.sh NEEDS_JSON='{…abandoned…}' bash script …` | **0** | (sem saída) | Bypass R5 — ver §6 |
| V-B1 arg | `NEEDS_JSON='{…abandoned…}' bash script --self-test parity-…` | 0 | `PASS all-success-1job …` | Entra em self-test mode; ignora NEEDS_JSON — ver NF-4 |
| V-B2 `--` | `NEEDS_JSON='{"a":…}' bash script -- a` | 1 | `job -- expected but missing` | Resistiu — `--` tratado como job name |
| V-B3 sem-e | Cópia com `set -uo pipefail`; `NEEDS_JSON='{…abandoned…}' bash cópia a` | **0** | `::error::…` em stderr, rc=0 | Confirmado — sem `-e`, rc do python3 é descartado; self-test pega (ver M6) |
| V-B4 MODE env | `MODE=self-test NEEDS_JSON='{"a":{"result":"abandoned"}}' bash script a` | 1 | `result=abandoned` | Resistiu — script redefine `MODE=run` internamente |
| V-trailing-sp-key | `NEEDS_JSON='{"a ":{"result":"success"}}' bash script a` | 1 | `job a expected but missing…job a  present…` | Resistiu — set comparison captura |
| V-W1/W2 | Leitura do quality.yml: checkout e setup-python sem `continue-on-error` | — | Se falham, job falha (não verde) | Leitura — sem bypass; veja §3 |
| Deep 1500 | `NEEDS_JSON='{"a":{"x":{"x":…1500x{"result":"abandoned"}…}}}' bash script a` | 1 | `result=None` | Resistiu — `r = data.get("result")` = None em nível errado |
| Deep 10000 | Idem com 10000 níveis; json.loads com C extension, sem RecursionError | 1 | `result=None` | Resistiu — C ext ignora `sys.setrecursionlimit`; nenhum RecursionError possível via json.loads |
| Huge 150KB | `NEEDS_JSON` com 150215 bytes (outputs grandes); 3 jobs `success` | 0 | `OK …` | Resistiu — sem E2BIG local |

**Nota sobre E2BIG:** os 3 needs de `parity` (`parity-falsify-shard`, `parity-other-gates`,
`check-required-checks`) têm `outputs: null` confirmado por yaml.safe_load — não há dados de usuário
em `toJSON(needs)`. O JSON real em CI é < 100 bytes por job. E2BIG é inatingível em produção.

---

## 3. Análise de workflow (V-W1/V-W2) — leitura, não medição

O job `parity` tem a seguinte sequência de steps (quality.yml:1010–1018):

```
1. actions/checkout@v7       (sem if:, sem continue-on-error)
2. actions/setup-python@v7   (sem if:, sem continue-on-error)
3. check-needs-success.sh    (sem if:, sem continue-on-error) ← guard
4. steps seguintes           (sem if: explícito)
```

Steps sem `if:` explícito usam `success()` implícito no nível de step. Se o step 1 (checkout)
falhar, os steps 2–4 são pulados e o job termina com `result = "failure"`. O guard não roda, mas o
job também não aprova — fail-closed.

Não há caminho em que `parity` reporte `success` com o guard tendo sido pulado sem
`continue-on-error: true` em checkout ou setup-python. Nenhum dos dois tem esse atributo
(confirmado nas linhas 1011–1014 do quality.yml).

---

## 4. Resultado das mutações

| Mutação | Descrição | Bypass direto medido | Self-test detecta? | Braços que falharam |
|---------|-----------|---------------------|-------------------|---------------------|
| M1 (denylist correto) | `type(r) is str and r in ("failure","cancelled","skipped")` | Sim — `abandoned` rc=0, `success` rc=0 | **Sim** | 4 (abandoned), 8 (invented-value), 15 (missing-result-key), 16 (null-result), 17–19, 21; arm 27 FAIL por literal mismatch (a linha já é denylist, sed não casa) |
| M2 (sem set comparison) | Remoção dos dois for-loops de `expected - actual` / `actual - expected` | Sim — `NEEDS_JSON='{}'` com job esperado rc=0 | **Sim** | 9 (empty-object), 24 (missing-expected-job) |
| M3i (sem todos os escapes) | Todos os `.replace(...)` de `esc()` removidos via sed | — | **Sim** | 21 (jobname-injection-newline) |
| M3ii (sem `\r` escape) | `.replace('\r', '%0D')` removido via sed | — | **Sobrevive** — ver NF-2 |
| M3iii (sem `%` escape) | `.replace('%', '%25')` removido via sed | — | **Sobrevive** — ver NF-2 |
| M4 (isinstance → pass) | Bloco `isinstance(needs, dict)` substituído por `pass` | Não — null/array causam `AttributeError: 'NoneType'/'list' has no attribute 'keys'`, rc=1 | **Sim** | 12 (null-toplevel), 13 (array-toplevel), 14 (string-toplevel) — pegou por mensagem errada (traceback em vez de "not a JSON object"), não por rc |
| M5 (exit 1 → true, mantendo printf) | `exit 1` do bloco `$# -eq 0` substituído por `true` | Sim — `NEEDS_JSON='{}'` sem args → rc=0, imprime mensagem de erro mas não sai | **Sobrevive** — ver NF-3; self-test 27/27 passa porque arm 26 usa NEEDS_JSON não-vazio, que faz o set check reprovar e ainda imprime "no job names" |
| M6 (sem `-e`) | `set -uo pipefail` (sem e); rc do python3 descartado | Sim — `abandoned` rc=0 | **Sim** | 4 (abandoned), 5 (failure), 6 (cancelled), 7 (skipped), 8 (invented-value) |

**Sobreviventes confirmados: M3ii, M3iii, M5.**

**Nota sobre M4 e o critério "bypass":** M4 não produz bypass. null/array em top-level causam
`AttributeError` (crash Python) com rc=1. O self-test pega M4 porque o arm verifica a substring
"not a JSON object" e recebe um traceback em vez disso — correto, mas por critério de mensagem,
não de rc.

**Nota sobre arm 27 (counter-arm-denylist) no M1 correto:** o arm 27 muta o script instalado de
allowlist para denylist via `sed`. Quando o script já É uma denylist (M1 aplicado), o `sed` busca
a literal da linha-allowlist, não casa → `cmp -s` detecta → arm 27 reporta FAIL por motivo
correto ("sabotage has no effect — line literal mismatch"). Comportamento esperado.

---

## 5. Arms 28 e 29 propostos — cobertura dos sobreviventes M3ii e M3iii

Medidos contra original, M3ii e M3iii. Descrição de arm no estilo do ML-1A.

**Arm 28: jobname-injection-cr**

```bash
# Arm 28: jobname-injection-cr
# Affirms: a \r in a job key is escaped (%0D) -- forged CR cannot create a new line in CI output
ARMS=$((ARMS + 1))
job_with_cr=$'x\r::error::forged'
rc28=0
out28=$(NEEDS_JSON='{"x\r::error::forged":{"result":"abandoned"}}' \
  "$BASH" "$SCRIPT" "$job_with_cr" 2>&1) || rc28=$?
arm28_ok=1
if [ "$rc28" -eq 0 ]; then
  printf 'FAIL jobname-injection-cr: expected rc!=0, got rc=0\n'
  arm28_ok=0
fi
if printf '%s\n' "$out28" | grep -qF '%0D::error::forged'; then
  :  # escaped form found -- correct
else
  printf 'FAIL jobname-injection-cr: escaped form %%0D::error::forged not found\n'
  arm28_ok=0
fi
[ "$arm28_ok" = "1" ] && { printf 'PASS jobname-injection-cr\n'; PASS=$((PASS + 1)); } \
  || FAIL=$((FAIL + 1))
```

**Arm 29: jobname-pct-injection**

```bash
# Arm 29: jobname-pct-injection
# Affirms: % in a job key is escaped (%25) -- %0A injection requires % to be double-encoded first
rc29=0
out29=$(NEEDS_JSON='{"a%0Aforged":{"result":"abandoned"}}' \
  "$BASH" "$SCRIPT" "a%0Aforged" 2>&1) || rc29=$?
check_arm "jobname-pct-injection" 0 "$rc29" "$out29" "" "a%250Aforged"
```

**Arm adicional M5b: no-args-empty-object**

```bash
# Arm 30: no-args-empty-object
# Affirms: calling with no args AND empty NEEDS_JSON is rejected -- guards against vacuous approval
# (M5 survivor: without exit 1, NEEDS_JSON='{}'+no-args exits rc=0 silently)
rc30=0
out30=$(NEEDS_JSON='{}' "$BASH" "$SCRIPT" 2>&1) || rc30=$?
check_arm "no-args-empty-object" 0 "$rc30" "$out30" "" "no job names"
```

Atualizar `EXPECTED_ARMS=30`.

**Resultados de medição:**
- Arm 28 contra original: **PASS**
- Arm 28 contra M3ii: **FAIL** (captura o sobrevivente)
- Arm 29 contra original: **PASS**
- Arm 29 contra M3iii: **FAIL** (captura o sobrevivente)
- Arm 30 contra original: rc=1 (**PASS**)
- Arm 30 contra M5 (exit 1 removido): `NEEDS_JSON='{}'` sem args → rc=0 (**FAIL**, captura o sobrevivente)

---

## 6. Achados classificados

### NF-1 — `python3 -` importa do cwd e do PYTHONPATH (recomendar como novo ML)

**Superfície:** V-E2 (cwd) e V-E2b (PYTHONPATH).

**Evidência:** medido com `json.py` que retorna os 3 jobs com `result: success`. Com o cwd ou
PYTHONPATH apontando para um diretório contendo esse arquivo:

```bash
# V-E2: cd para dir com json.py hostil
cd $d_com_json_py
NEEDS_JSON='{"parity-falsify-shard":{"result":"abandoned"},...}' \
  bash scripts/check-needs-success.sh parity-falsify-shard parity-other-gates check-required-checks
# rc=0  out="OK  check-required-checks  result=success ..."
```

**Mecanismo:** `python3 -` coloca `''` em `sys.path[0]` (cwd). `PYTHONPATH` é respeitado.
`import json` importa o módulo hostil em vez do stdlib.

**Shadow no root do repo:** `git ls-files | grep -E '^(json\.py|json/__init__\.py|os\.py)$'` →
`(nenhum)`. O bypass não está ativo hoje. Se fosse ativo, seria bloqueante.

**Classe de ataque:** quem pode introduzir `json.py` no repositório pode também modificar
`check-needs-success.sh` diretamente. A diferença é que `json.py` é mais covert — um revisor
pode não perceber que o arquivo é relevante para o guard. A Regra Dura de Causa Raiz se aplica:
mesma causa (python3 sem isolamento), mesma REQ.

**Fix medido:** trocar `python3 - "$@"` por `python3 -I -X utf8 - "$@"`. `-I` (isolated mode)
implica `-E` (ignora `PYTHONPATH`, `PYTHONSTARTUP`, `PYTHONHOME`) e remove `''` de `sys.path`.
`-X utf8` (PEP 540) garante encoding UTF-8 independentemente do locale, compensando o fato de
`-I` ignorar `PYTHONIOENCODING`.

Resultados medidos com a correção (`sed 's/python3 - "\$@"/python3 -I -X utf8 - "$@"/'`):
- Self-test 27/27 PASS ✓
- V-E2 (cwd + json.py hostil): rc=1 ✓
- V-E2b (PYTHONPATH + json.py hostil): rc=1 ✓
- `PYTHONIOENCODING=cp1252 python3 -I -X utf8`: stdout.encoding = `utf-8` ✓

**Nota sobre `export PYTHONIOENCODING=utf-8` no script:** com `-I`, este export torna-se no-op
em runtime (ignorado por `-E`). A linha deve ser mantida porque `check-output-encoding-declared`
(gate do `parity-rest`) valida que o script exporta a variável — a remoção quebraria esse gate.
O encoding real em runtime é garantido por `-X utf8`.

**Correção exata (em `scripts/check-needs-success.sh`, linha 60):**
```
-  python3 - "$@" <<'PY'
+  python3 -I -X utf8 - "$@" <<'PY'
```

**Recomendação:** abrir novo ML nesta REQ (Regra Dura de Causa Raiz) — mesma superfície de módulo
externo, mesma fix de uma linha.

---

### NF-2 — Escapes `\r` e `%` não testados pelo self-test (gap de cobertura)

**Superfície:** M3ii (sobrevivente), M3iii (sobrevivente).

**Evidência:**
- M3ii (sem `.replace('\r', '%0D')`): self-test 27/27 passa — nenhum braço testa `\r` em job name
- M3iii (sem `.replace('%', '%25')`): self-test 27/27 passa — nenhum braço testa `%` em job name
- Arms 28 e 29 propostos: capturam M3ii e M3iii respectivamente (medido)

**Impacto em produção:** IDs de job do GitHub Actions são restritos a `[a-zA-Z_][a-zA-Z0-9_-]*`.
`\r` e `%` não podem aparecer em nomes de job reais. O vetor de injeção é bloqueado pela sintaxe
do YAML antes de chegar ao script.

**Gap de origem:** o ML-0A §3.2 nomeou V14 (`jobname-injection-cr`) como braço obrigatório, mas
ele não entrou na tabela §3.4 nem nos 27 braços do ML-1A. A ausência de `%` não foi listada
em §3.4.

**Correção:** adicionar arms 28, 29, 30 ao `--self-test` e atualizar `EXPECTED_ARMS=30` (ver §5).
**Recomendação:** novo ML nesta REQ.

---

### NF-3 — Arm 26 (no-args) decorativo para M5 — M5 é sobrevivente parcial

**Superfície:** M5 — `exit 1` do bloco `$# -eq 0` substituído por `true`.

**Evidência:**
- Self-test com M5: 27/27 PASS — arm 26 usa `NEEDS_JSON='{"a":{"result":"success"}}'` (não-vazio),
  o set check força rc=1 independentemente do `exit 1`, e a mensagem "no job names" ainda é
  impressa
- Vetor de bypass: `NEEDS_JSON='{}'` sem args → rc=0 (imprime mensagem mas não sai)
  ```
  NEEDS_JSON='{}' bash mut_m5b.sh
  rc=0  out="mut_m5b.sh: ::error::no job names given -- usage: …"
  ```

**Significado:** arm 26 não discrimina o `exit 1` do bloco `$# -eq 0` porque (a) o rc é obtido
também pelo set check quando NEEDS_JSON é não-vazio, e (b) a mensagem "no job names" é impressa
mesmo sem o `exit 1`. O arm é decorativo para esta mutação específica.

**Impacto em produção:** a chamada em quality.yml:1018 tem 3 job names; `$# -eq 0` nunca é True.
Exploitar requer `NEEDS_JSON='{}'` + nenhum argumento, que viola o contrato declarado. Sem vetor
de bypass em produção.

**Correção:** adicionar arm 30 (`no-args-empty-object`) ao `--self-test` (ver §5), que testa
`NEEDS_JSON='{}'` sem args.

---

### NF-4 — `--self-test` como primeiro argumento posicional entra em self-test mode (design)

**Superfície:** V-B1.

**Evidência:**
```bash
NEEDS_JSON='{"parity-falsify-shard":{"result":"abandoned"},...}' \
  bash check-needs-success.sh --self-test parity-falsify-shard …
# rc=0, 27/27 PASS — ignorou completamente NEEDS_JSON e job names reais
```

Args extras após `--self-test` são silenciosamente ignorados (o `shift` remove apenas o primeiro).

**Por que não é exploitável em produção:** IDs de job do GitHub Actions não podem começar com `--`
(sintaxe YAML inválida). A chamada em quality.yml:1018 começa com `parity-falsify-shard`.

**Correção mínima (não urgente):** no início do bloco `if [ "$MODE" = "self-test" ]; then`:
```bash
if [ $# -gt 0 ]; then
  printf '%s: ::error::--self-test takes no arguments (got: %s)\n' "$SELF" "$*" >&2
  exit 1
fi
```
Medido em cópia: self-test 27/27 passa com esse check; `--self-test extra` retorna rc=1.

---

### R — Residuais declarados

**R-cwd-PYTHONPATH:** V-E2 e V-E2b. Fechados por `python3 -I -X utf8` (ver NF-1). Recomendado
como novo ML.

**R-M3ii/iii/M5:** gaps de cobertura no self-test. Sem exploitabilidade em produção. Fechados
pelos arms 28, 29 e 30 (ver NF-2 e NF-3). Recomendado como novo ML.

**R-V-E3 (PATH falso):** quem pode injetar um `python3` falso no PATH de um step do `parity` pode
modificar o próprio `quality.yml` — mesmo argumento da correção 2 do ML-0A ("quem pode trocar o
script também pode trocar o quality.yml"). Classificado como R5; `python3 -I` não fecha este vetor.

**R-V-E5b (BASH_ENV função):** quem pode setar `BASH_ENV` em um step anterior do `parity` pode
modificar o `quality.yml` diretamente. Mesmo argumento. Classificado como R5; `python3 -I` não
fecha (a função é encontrada pelo `command -v` antes do binário externo).

**R-NaN:** Python aceita NaN/Infinity como extensão não-padrão. Script reprova via `type(r) is not
str`. Sem bypass; gap apenas de cobertura (sem braço no self-test).

---

## 7. Verificações de integridade finais

```bash
bash scripts/check-needs-success.sh --self-test
# 27/27 PASS (medido durante este ML)

git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw status --short
# ?? docs/seguranca/2026-11-12-wave2-redteam-agregador-needs-success.md

git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw diff --cached --stat
# (vazio)
```
