# Revisão de Qualidade — Recalibração dos Pesos do Falsify

> REQ-2026-10-03 / #403 | ML-3A | Hefesto-tf | 2026-10-04
> Branch: `fix/pesos-do-falsify-recalibrados-do-ci`
> Escopo: `scripts/falsify-recalibrate.sh` (novo), `scripts/check-falsify-recalibrate.sh` (novo),
> `scripts/gen-falsify-chunks.py`, `scripts/gen-falsify-scenario-weights.py`,
> `scripts/falsify-scenario-weights.json`, `Makefile`, `.github/workflows/quality.yml`
> Especificação: ADR-2026-10-03, decisões D1–D5

---

## 1. Gate Completo

**Pré-condição de carga (antes do gate):**
```
10:03  up 2 days, 18:38, 4 users, load averages: 1.84 3.33 3.68
PID  %CPU  COMM
1666  19.3  Stats.app
2108  14.9  claude
47843 11.6  ClaudeBar
610   10.5  WindowServer
13150  6.1  WebKit.WebContent
31628  4.8  claude
1961   2.6  OrbStack Helper
```

**Comando executado:**
```bash
cd /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw
make quality > /tmp/claude-501/mq403.log 2>&1; echo EXIT=$? >> /tmp/claude-501/mq403.log
```

**Log:** `/tmp/claude-501/mq403.log`
**Linhas:** 853
**Última linha:** `EXIT=2`
**EXIT:** 2

**Resultado por gate relevante:**

| Gate | Resultado |
|---|---|
| `check-crlf-normalize-capture` | OK (linha 489) |
| `check-interpolated-path-in-python` | **FAIL** (linha 851) |
| `check-falsify-recalibrate` (autoteste) | Não executou — `make` abortou na linha 852 antes de chegar no step |

**Autoteste standalone** (`bash scripts/check-falsify-recalibrate.sh`): 6/6 PASS — o autoteste passa quando executado diretamente, mas o gate estático `check-interpolated-path-in-python` reprovaria o passo 218 do `parity-rest` antes de o script ser invocado.

---

## 2. Revisão

### 2.1 falsify-recalibrate.sh

- `set -euo pipefail` presente. Quoting de variáveis correto em todo o script.
- `FALSIFY_REPO_ROOT` documentado e funcional: permite que o autoteste grave o script sabotado em `$SCRATCH` e ainda resolva `quality.yml` e `gen-falsify-scenario-weights.py` no repo real.
- `TMP=$(mktemp -d ...)` com `trap 'rm -rf "$TMP"' EXIT`. Limpeza garantida.
- Passo (a): verificação de `head_repository` com `gh api` antes de qualquer download. Condizente com AJ-T4.
- Passo (b): leitura de `FALSIFY_SHARD_COUNT` do `quality.yml` — sem duplicação da constante.
- Passo (d): exige `timing_${n}.log` não vazio para cada shard. Não toca o destino se falta algum. Condizente com AJ-T7.
- Passo (e): concatenação ordenada (seq 0..N-1), roda `gen-falsify-scenario-weights.py`.
- Sem problemas estruturais.

### 2.2 check-falsify-recalibrate.sh

- `set -euo pipefail` presente.
- Provas de mordida (S1, S2): o script sabotado é gravado em `$SCRATCH`, não em `scripts/`. `FALSIFY_REPO_ROOT="$REPO_ROOT"` passado para que o sabotado resolva os caminhos do repo real. `rm -f "$SABOTADO"` executado após cada prova. Sem escrita fora do diretório temporário.
- Heredoc do `gh` falso: delimitador `GHEOF` sem aspas (expansão intencional de `$dir`); `\$*` e `\$CMD` escapados corretamente para serem literais no script gravado.
- **DEFEITO:** linha 131 — Python inline com caminho interpolado:
  ```bash
  elif ! python3 -c "
  import json, sys
  d = json.load(open('$DEST1'))
  ```
  O gate `check-interpolated-path-in-python` reprovará este padrão. No MSYS/Git-Bash a conversão POSIX→Windows ocorre apenas em `argv`, não dentro de strings de código. O caminho deve ser passado como argumento.

### 2.3 gen-falsify-scenario-weights.py

- Regex `_TS_VALID = re.compile(r'^[0-9]+(\.[0-9]+)?$')` valida o `ts` antes de `float()`, com mensagem que nomeia arquivo e linha. Condizente com AJ-T6.
- Duração não finita e negativa causam `sys.exit` com mensagem. Duração zero eleva para 0,001 s com aviso em stderr. Condizente com AJ-T5.
- Escrita atômica: `mkstemp(dir=out_dir)` + `os.replace`. Remove o temporário em caso de exceção. Condizente com AJ-T5.

### 2.4 gen-falsify-chunks.py

- `load_weights` valida cada peso: NaN, Inf, negativo e não numérico causam `sys.exit` com o rótulo nomeado. Condizente com AJ-T5.
- Mesma validação para `_fallback_weight_for_unlabeled`.
- Linha de resumo final emitida em `stderr`: `N de M rotulos sem peso calibrado (X%)`. Condizente com D4 e AJ-T8.
- Exit code inalterado (a linha de resumo nunca reprova). Condizente com decisão do KG em D4.

### 2.5 Makefile

Comentário do target `parity-rest` (linhas 211-218):

```makefile
# ML-1A (REQ-2026-10-03, #403): autoteste da recalibracao dos pesos do falsify.
# Exercita 4 bracos (run completo, shard ausente, fork, ts invalido) e 2 provas
# de mordida (S1: guarda do fork; S2: guarda do shard ausente). Sem RUN real:
# usa gh falso em $SCRATCH, nao chama a API do GitHub.
```

Confirmado: o autoteste executa 4 bracos + 2 provas de mordida = 6 casos, todos PASS isoladamente. O uso de `gh` falso via `GH_BIN` é correto; nenhuma chamada de rede real. O comentário diz a verdade.

### 2.6 quality.yml

`FALSIFY_TIMING_FILE: ${{ github.workspace }}/falsify-shard-out/timing_${{ matrix.shard }}.log`
O diretório `falsify-shard-out/` é criado em `run-gates-falsify-shard.sh:96` antes do chunk. A variável chega ao processo do chunk porque não há `env -i` no caminho. Condizente com AJ-T3.

### 2.7 Resíduo do ML-2A

O roadmap registra: 3 blocos sem rótulo (linhas 1814, 4977 e 6925 de `check-gates-falsify.sh`), todos com fallback `_fallback_weight_for_unlabeled` = 69,11 s. Bloco 1814 realmente leva ~69 s; blocos 4977 e 6925 levam ~2 s cada. Com N=8, cada um recebe chunk próprio, deixando 2 workers com ~2 s de trabalho.

Confirmado pelo arquivo recalibrado:

```
_fallback_weight_for_unlabeled: 69.1122
len(weights): 204
```

O resíduo declarado no roadmap concorda com o arquivo e com a lógica do empacotador.

---

## 3. Achado

### A1 — REPROVADOR: caminho interpolado em corpo Python (check-falsify-recalibrate.sh:131)

**Severidade:** bloqueador de gate
**Evidência:** `/tmp/claude-501/mq403.log` linha 786:
```
FAIL [check-falsify-recalibrate.sh:131] caminho interpolado no texto do programa Python: d = json.load(open('$DEST1'))
     COMO CORRIGIR: passe o caminho por argumento, nunca dentro do texto do programa.
```
**Causa:** `python3 -c "... open('$DEST1') ..."` expande `$DEST1` dentro do código Python. Em MSYS/Git-Bash, a conversão POSIX→Windows ocorre em `argv`, não em strings dentro do código.

**Correção executável (uma frase por passo):**

No bloco do arm 1 (linhas 129–136 de `check-falsify-recalibrate.sh`), substitua:
```bash
elif ! python3 -c "
import json, sys
d = json.load(open('$DEST1'))
w = d.get('weights', d)
sys.exit(0 if 'r0' in w else 1)
" 2>/dev/null; then
```
por:
```bash
elif ! python3 -c "
import json, sys
d = json.load(open(sys.argv[1]))
w = d.get('weights', d)
sys.exit(0 if 'r0' in w else 1)
" "$DEST1" 2>/dev/null; then
```

O precedente nesta árvore está em `scripts/check-thirdparty-parity.sh:167` (`json.load(open(sys.argv[1]))`).

---

## 4. Veredito

**REPROVA**

EXIT=2. Um FAIL real no gate `check-interpolated-path-in-python` (A1 acima). O autoteste `check-falsify-recalibrate` passa 6/6 quando executado isoladamente, mas nunca chega a rodar no `make quality` porque o gate estático falha antes. Todo o restante do diff (scripts, Python, workflow, Makefile) está correto e condizente com ADR D1–D5. O único bloqueio é A1.
