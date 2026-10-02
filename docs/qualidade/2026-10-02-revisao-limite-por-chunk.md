# Parecer de Qualidade — ML-2A: limite de tempo por chunk no driver de falsificação

**Revisor:** hefesto-tf  
**Data:** 2026-10-02  
**Roadmap:** ROADMAP-2026-10-02-driver-de-falsificacao-paralelo-espera-para-sempre-por-chunk-que-nao-termina-limite-de-tempo-por-chunk-com-fail-nomeado.md  
**Diff base:** `git diff 0bf66679..HEAD -- scripts/ Makefile`  
**make quality:** EXIT=0

---

## 1. Conformidade com as 9 ações do ML-1A

| # | Ação | Arquivo / Linha | Status |
|---|------|-----------------|--------|
| 1 | `set -m` antes do launch; `PGID=$!` imediatamente após `&` | `run-gates-falsify-parallel.sh` — bloco de launch | OK |
| 2 | `</dev/null` explícito na linha de launch sob `set -m` | mesma linha: `( ... bash "$chunk" ) >"$log" 2>&1 </dev/null &` | OK |
| 3 | Timeout: árvore antes do kill, SIGTERM → 2 s → SIGKILL, varredura pós-wait | coleção com polling + bloco timed_out | OK |
| 4 | `trap` INT TERM HUP no driver, **não** EXIT | instalado antes do loop de launch; trap EXIT preexistente preservado | OK |
| 5 | Nenhum `exit` antes da guarda de conjunto | bloco timeout seta FAILED=1 e continua; guarda roda incondicionalmente | OK |
| 6 | `TRACKFW_FALSIFY_CHUNK_TIMEOUT` validado contra `^[1-9][0-9]*$` | validação com regex + mensagem de erro explícita | OK |
| 7 | Declarar bash 4+ no cabeçalho do driver | comentário no topo de `run-gates-falsify-parallel.sh` | OK |
| 8 | Chunk sintético com `sleep 999999` / loop (sem `sleep infinity`) | `check-falsify-chunk-timeout.sh`: usa `sleep 9981743` + `while :; do sleep 9981743; done` | OK |
| 9 | Autoteste: macOS+Linux OK; Git Bash/MSYS skip declarado; override relatado no stderr | plataforma detectada via `uname`; skip com `exit 0`; override declarado no stderr | OK |

Todos os 9 ajustes da Wave 0 e ações do ML-1A estão presentes e corretamente posicionados.

---

## 2. Conformidade com o parecer Wave 0

### 2.1 Mecanismo de kill

O parecer recomendou `set -m` + `kill -TERM -- -PGID; sleep 2; kill -9 -- -PGID`. Implementado conforme recomendação, com varredura não-bloqueante adicional após o `wait`, cobrindo grandchildren que chamaram `setsid()` (resíduo R3 declarado). Aderência total.

### 2.2 Limite de 1200 s

Mantido como padrão (`DEFAULT_CHUNK_TIMEOUT=1200`). Deadline absoluto a partir do primeiro launch (`LAUNCH_TS`): se N chunks estão travados, todos disparam no mesmo instante após T, não N×T. Correto e documentado no comentário.

### 2.3 Wiring somente em parity-rest (ubuntu-latest)

`Makefile`: o `check-falsify-chunk-timeout.sh` está ligado exclusivamente ao target `parity-rest`, que roda em `ubuntu-latest`. Não foi ligado a nenhum target que o `windows-full-suites` execute. Aderência ao ajuste 5 da Wave 0.

---

## 3. Achado: braço 5 skip conta como PASS

**Arquivo:** `scripts/check-falsify-chunk-timeout.sh`, **linha 330**

```bash
_ok "contra-braco/skip" "commit $OLD_COMMIT nao disponivel -- skip declarado"
```

**Problema:** `_ok` incrementa PASS+1 e TOTAL+1. No CI (`parity-rest`, `actions/checkout@v7` sem `fetch-depth: 0` — clone raso de profundidade 1), o commit `0bf66679` não está disponível; o braço 5 sempre salta. O resumo final reporta `10/10 bracos passaram`, mas um dos 10 não executou nada. O `check-channels-content.sh` (mesmo repositório) usa convenção diferente: função `skip()` com contador próprio `$SKIP`, reportado separadamente como "X passed, Y failed, Z skipped".

**Evidência do CI:** o job `parity-rest` em `.github/workflows/quality.yml` usa `actions/checkout@v7` sem `with: fetch-depth:`, portanto `fetch-depth` padrão = 1. Commit `0bf66679` está quatro commits atrás do HEAD atual da branch; o clone raso não o contém.

**Impacto:** os 4 braços críticos (invalid-timeout, normal-completion, hung-chunk, stdin-isolation) continuam executando em CI sem nenhum problema. O braço 5 é prova de regressão do driver antigo — útil, mas não é o caminho crítico. O risco real é que o CI nunca valide que o driver antigo trava, e o rótulo de sucesso mascara esse gap no resumo.

**Severidade:** média.

**Correção sugerida (não bloqueante para aprovação, mas obrigatória antes do merge):**

Adicionar função `_skip` com contador separado:

```bash
SKIP=0
_skip() {
  SKIP=$((SKIP + 1))
  TOTAL=$((TOTAL + 1))
  echo "SKIP [falsify-driver/$1] $2"
}
```

Substituir a chamada de linha 330:
```bash
_skip "contra-braco" "commit $OLD_COMMIT nao disponivel -- skip declarado"
```

Atualizar o bloco de resultado final:
```bash
echo "=== check-falsify-chunk-timeout: $PASS/$((TOTAL - SKIP)) bracos passaram ($SKIP skip) ==="
if [[ "$PASS" -ne "$((TOTAL - SKIP))" ]]; then
  echo "check-falsify-chunk-timeout: FAIL -- $((TOTAL - SKIP - PASS)) braco(s) reprovaram" >&2
  exit 1
fi
```

Assim, em CI o resumo seria `9/9 bracos passaram (1 skip)` — explícito e não-enganoso.

---

## 4. Resultado do `make quality`

```
EXIT=0
```

Linhas citadas do log:

```
run-gates-falsify-parallel: suite completa -- 8 chunks, 347 OK, 0 FAIL, guarda de conjunto OK (nenhum rotulo esperado ausente)
```

Autoteste (localmente, com `0bf66679` disponível, braço 5 executou):

```
OK   [falsify-driver/invalid-timeout] driver abortou rc=1 com mensagem de invalido
OK   [falsify-driver/normal-completion] rc=0, sem linha de timeout; OK [falsify/synthetic-ok] encontrado no output
OK   [falsify-driver/hung-chunk/rc] rc=1≠0
OK   [falsify-driver/hung-chunk/time] terminou em 7s (≤ T+10=15s)
OK   [falsify-driver/hung-chunk/fail-line] linha FAIL [falsify-driver/chunk-timeout] presente em stderr
OK   [falsify-driver/hung-chunk/tree] arvore de processos impressa (marker 9981743 encontrado)
OK   [falsify-driver/hung-chunk/guard] guarda de conjunto relatou ausencia de CHUNK_COMPLETE
OK   [falsify-driver/hung-chunk/survivors] 0 processos sobreviventes (sleep 9981743)
OK   [falsify-driver/stdin-isolation] chunk nao leu stdin do driver (STDIN_ISOLATED confirmado)
OK   [falsify-driver/contra-braco] driver antigo (0bf66679) ainda vivo em 15s: prova que bloqueia em wait sem timeout
=== check-falsify-chunk-timeout: 10/10 bracos passaram ===
check-falsify-chunk-timeout: OK
```

---

## Veredito: APROVA COM AJUSTES

**Ajuste obrigatório antes do merge:**

| # | Arquivo | Linha | Ajuste |
|---|---------|-------|--------|
| A1 | `scripts/check-falsify-chunk-timeout.sh` | 330 | Substituir `_ok "contra-braco/skip"` por `_skip "contra-braco"` com contador separado; atualizar resumo final para reportar "N/N bracos passaram (M skip)" |

As 9 ações do ML-1A estão implementadas corretamente. O `make quality` passou com EXIT=0, suite 347 OK / 0 FAIL.
