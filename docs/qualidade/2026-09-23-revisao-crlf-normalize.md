# Revisão de qualidade — Barreira final da REQ do CRLF

> Hefesto (Code Quality) · 2026-09-23
> Branch: `fix/bash-consome-stdout-de-python3-sem-normalizar-crlf`

---

## Veredito

**BLOQUEIA — não como nova REQ, mas como ML pendente nesta REQ.**

O roadmap marca `- [x] Todo (a) corrigido por ponto único`. Existem 9 sítios
categoria (a) confirmados ainda sem `strip_cr`: 8 em `check-barrier.sh` e 1 em
`check-update-parity.sh`. Todos em capturas de funções helper que chamam python3
internamente — mesmo mecanismo, mesmo arquivo, mesmo padrão de `doc_status` e
`get_wave_field` que JÁ foram corrigidos no diff.

O AC não está satisfeito. Pela Regra Dura, esses sítios entram como ML-1C nesta
REQ — não como REQ nova — e o PR permanece aberto até que todos entrem.

---

## Q1 — 20 scripts sourceiam um lib. Vale o que custa? Alternativa Python?

**Vale. A alternativa Python-side não está documentada como rejeitada.**

O custo medido: um único corretivo (ML-1A-bis), causado pela resolução do lib via
`ROOT_DIR` mutável no `--self-test`. Após migrar para `SCRIPT_DIR`, nenhum outro
corretivo foi necessário.

O lib é 1 linha funcional:

```bash
# scripts/lib-crlf-normalize.sh:40
strip_cr() { sed $'s/\r$//'; }
```

Portável (BSD sed + GNU sed). A alternativa Python-side (`sys.stdout = open(...,
newline='\n')` ou PYTHONIOENCODING sem tradução de newline) exigiria tocar os ~19
scripts Python inline com lógicas diferentes. O "ponto único" ficaria no lado
errado da fronteira.

**Gap de documentação:** o roadmap (linha 76) cita "binary mode no Python" como
opção mas não documenta a rejeição. Não bloqueia; registrar como lacuna de ADR se
o rastreio importa.

---

## Q2 — Resolução por SCRIPT_DIR está uniforme?

**Três nomes de variável, funcionalmente corretos, cosméticamente inconsistentes.**

```bash
# Verificação de convenções nos 19 sítios:
# grep -n 'SCRIPT_DIR\|ROOT_DIR\|_SELF_DIR' scripts/*.sh | grep -v '#' | head -60

SCRIPT_DIR  → 9 scripts (padrão dominante pós-ML-1A-bis)
ROOT_DIR    → 9 scripts (resolve repo-root → scripts/lib)
_SELF_DIR   → 1 script (trackfw-attention-signal.sh, funciona igual a SCRIPT_DIR)
```

O `ROOT_DIR` via `TRACKFW_ROOT_DIR` em `check-gates-falsify.sh` e
`check-git-branch-guard-hook-schema.sh` é intencional: chunks da falsify são
copiados para `/tmp` e o override é o mecanismo documentado. A falsify copia o
lib junto (linha 7119 de check-gates-falsify.sh). Não é problema.

`_SELF_DIR` em trackfw-attention-signal.sh é ruído cosmético — funciona. Não
bloqueia.

---

## Q3 — O gate novo é legível?

**Sim, acima da média do repositório em dois aspectos, abaixo em um.**

Pontos fortes (P1, P2 de docs/gate-design-principles.md):

- `MIN_CAPTURES` documentado com proveniência: data, comando exato, resultado (66),
  fórmula (≈75%). Segue P1.
- Strings de diagnóstico explícitas para `assert_fails_with`.
- `LOOKAHEAD_LINES=20` com justificativa ("All known multi-line patterns close
  within 12 lines"). _Nota: o valor (20) e a afirmação (12) não coincidem — a
  direção é segura mas a documentação é inconsistente._
- `export PYTHONIOENCODING=utf-8` acompanhado de comentário explicando a menção
  morta (sem invocar python3).

Ponto abaixo do padrão (vs. `check-raw-read-ban.sh`):

- **Mensagem de FAIL não diz o que fazer:**
  ```
  FAIL [$fname:$lineno] python3 capture without strip_cr: $line
  ```
  Diz onde e o quê, não como. A mensagem ideal acrescentaria
  `"— pipe through | strip_cr before closing )"`. Cosmético; não bloqueia.

---

## Q4 — 18 menções a python3 e PYTHONIOENCODING declarado sem invocar Python. Sintoma de quê?

**Trade-off documentado e correto. O discriminante do gate de encoding é
conservador por construção — o CRLF gate é o primeiro caso concreto do
comportamento esperado.**

O `check-output-encoding-declared.sh` usa discriminante por arquivo: qualquer
`check-*.sh` com `python3` em linha não-comentário entra na população. O trade-off
é documentado no próprio gate (linhas ~207-212):

```
Trade-off assumido, na direcao segura: uma mencao MORTA a `python3` dentro
de corpo de heredoc passa a colocar o arquivo na populacao e a exigir dele
a declaracao. Isso e falso positivo, ruidoso e FECHADO — reprova pedindo
uma linha inofensiva —, ao contrario do falso negativo que substitui.
Ocorre desde 2026-09-23: check-crlf-normalize-capture.sh menciona
`python3` em ERE e comentarios (sem invocar), caiu na populacao e resolveu
exatamente como o trade-off previu.
```

O gate poderia usar `population_lines` (já implementado) para distinguir menção
real de invocação. Mudança fora do escopo desta REQ; trade-off atual documentado
e aceito.

---

## Q5 — Cobertura. strip_cr no lugar errado? Caso que vai doer depois?

**strip_cr está no lugar certo nos 66 candidatos varridos. Mas o gate tem um
ponto cego estrutural para capturas indiretas — 9 sítios (a) confirmados,
já presentes na mesma entrega que corrigiu o padrão idêntico em dois outros
sítios do mesmo arquivo.**

### 5a — strip_cr no lugar errado

```bash
grep -rn 'strip_cr' scripts/*.sh | grep -v 'lib-crlf-normalize\|check-crlf-normalize' | wc -l
# 24
```

Todos os 24 usos estão dentro do pipeline do python3, antes do fechamento do
`$()` ou do redirecionamento para arquivo. Braço "strip_cr fora do contexto"
não se materializou.

### 5b — Ponto cego estrutural: capturas indiretas através de função

O gate detecta `$([^\)]*python3` mas não `$(funcao_que_chama_python3 ...)`.

Em `check-barrier.sh` existem dois padrões corrigidos no diff:

```diff
# doc_status — função helper com python3 internamente
-  python3 -c "...print(json.loads(sys.argv[1])['status'])" "$doc"
+  python3 -c "...print(json.loads(sys.argv[1])['status'])" "$doc" | strip_cr

# get_wave_field — idem
-  python3 -c "...print(json.loads(sys.argv[1])['wave'])" "$1"
+  python3 -c "...print(json.loads(sys.argv[1])['wave'])" "$1" | strip_cr
```

Mesma função `check_field_json` (linha 142), mesmo padrão, não corrigida:

```bash
check_field_json() {
  local doc=$1 name=$2 field=$3
  python3 -c "
import json, sys
...
print(json.dumps(c.get(field)))
...
" "$doc" "$name" "$field"
# sem strip_cr
}
```

#### Sítios de captura confirmados sem strip_cr

**check-barrier.sh** — 7 capturas + 1 pipe-to-file:

```bash
# Verificado: git show main:scripts/check-barrier.sh | grep -n 'check_field_json' | grep '=\$('
# linha 524
CMDS=$(check_field_json "$BARRIER_STDOUT" "gates" "commands")
[[ "$CMDS" == "[]" ]]                        # bash string compare — quebra com \r

# linhas 964-978
WH_STATUS=$(check_field_json "$BARRIER_STDOUT" wave_headings status)
[[ "$WH_STATUS" == '"passed"' ]]             # quebra: "passed"\r != "passed"
MLS_STATUS=$(check_field_json ...)
MLS_EVIDENCE=$(check_field_json ...)
GATES_STATUS=$(check_field_json ...)
GATES_COMMANDS=$(check_field_json ...)
[[ "$GATES_COMMANDS" == '["exit 0"]' ]]      # quebra
GATES_EVIDENCE=$(check_field_json ...)

# linha 625 — normalize_barrier_json
echo "$BARRIER_STDOUT" | normalize_barrier_json >"$WORK/go.norm.json"
# normalize_barrier_json chama python3 com json.dump(..., indent=2)
# sem strip_cr → arquivo tem \r\n → diff pode falhar
```

**check-update-parity.sh** — 1 captura:

```bash
# linha 336
ids_go=$(target_ids_json "$S1_GO_OUT" 2>/dev/null || echo "PARSE_ERROR")
[[ "$ids_go" == "PARSE_ERROR" || -z "$ids_go" || "$ids_go" == "[]" ]]
# "[]"\r != "[]" → braço de vacuidade nunca dispara
```

**check-roadmap-barrier-contract.sh** — 4 capturas:

```bash
# linhas 144, 163, 792, 830
arr=$(doc_check_json "$doc" "$name" "$field")
mls_failures=$(doc_check_json "$CLI_STDOUT" "mls_complete" "failures")
```

Esses 4 passam o valor para `python3 -c "json.loads(sys.argv[1])"` que é
leniente com `\r` (whitespace no JSON decoder). Risco menor que os de
check-barrier.sh, mas mesma causa.

#### Por que o gate não os viu

O padrão do gate `\$\([^\)]*python3` não casa com `$(check_field_json ...)`.
A vacuity guard (66 >= 50) confirma que o corpus principal está coberto, mas
os 9 sítios acima ficaram fora da população por construção.

---

## Q6 — O que bloquearia

### Corrigir neste PR (ML-1C desta REQ — Regra Dura)

**9 sítios, mesma causa, mesmo mecanismo, mesmo arquivo dos sítios já corrigidos.**

1. Adicionar `| strip_cr` dentro de `check_field_json` em `check-barrier.sh`
   (linha 142) — corrigi a função e os 7 `$(check_field_json ...)` ficam cobertos
   automaticamente.
2. Adicionar `| strip_cr` dentro de `normalize_barrier_json` em
   `check-barrier.sh` (linha 614) — `json.dump(..., sys.stdout)` → pipe → strip_cr
   antes de fechar a função.
3. Adicionar `| strip_cr` dentro de `target_ids_json` em
   `check-update-parity.sh` (linha 91) — `print(json.dumps(...))` → pipe →
   strip_cr.
4. Adicionar `| strip_cr` dentro de `doc_check_json` em
   `check-roadmap-barrier-contract.sh` (linha 113) — mesmo padrão.
5. **Ampliar o gate:** adicionar um padrão de scan para `python3`/`$PY_BIN`
   no corpo de funções helper cujo stdout é capturado, OU mover o `strip_cr`
   para dentro das funções (o que o gate já detecta via LOOKAHEAD_LINES). A
   segunda opção é mais simples — resolve automaticamente sem mudar o padrão
   do gate.

### Registrar como issue separada (não bloqueia)

6. Discriminante de `check-output-encoding-declared.sh` poderia distinguir
   menção de invocação. Trade-off aceito e documentado; melhoria ergonômica
   independente.
7. Mensagem de FAIL do gate não diz como corrigir. PR de follow-up.
8. Documentação da rejeição da alternativa Python-side no roadmap/ADR.
9. Inconsistência cosmética: `LOOKAHEAD_LINES=20` mas comentário diz
   "within 12 lines". Correção de doc, direção segura.

---

## Evidências

```bash
# Gate RC e corpus
bash scripts/check-crlf-normalize-capture.sh 2>&1 | tail -8
# Candidates: 66 | Exempt: 6 | MIN: 50 | RC=0

# check_field_json sem strip_cr em main (pré-existente, não introduzido por esta branch)
git show main:scripts/check-barrier.sh | grep -n 'check_field_json' | grep '=\$('
# 524, 964, 967, 969, 972, 974, 976

# doc_status e get_wave_field FORAM corrigidos nesta branch
git diff main...HEAD -- scripts/check-barrier.sh | grep 'strip_cr' | head -5
# +  python3 -c "...print(json.loads(sys.argv[1])['status'])..." "$doc" | strip_cr
# +  python3 -c "...print(json.loads(sys.argv[1])['wave'])..." "$1" | strip_cr

# check_field_json NÃO foi corrigida (confirma gap)
sed -n '140,155p' scripts/check-barrier.sh | grep -c 'strip_cr'
# 0

# check-update-parity.sh: target_ids_json sem strip_cr
sed -n '88,95p' scripts/check-update-parity.sh
# target_ids_json() { python3 -c "...print(json.dumps(...))"...; } — sem strip_cr
grep -n 'target_ids_json' scripts/check-update-parity.sh | grep '=\$('
# 336: ids_go=$(target_ids_json ...) → [[ "$ids_go" == "[]" ]]
```
