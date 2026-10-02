#!/usr/bin/env bash
# check-falsify-chunk-timeout.sh — autoteste do limite de tempo por chunk
# implementado em run-gates-falsify-parallel.sh (ML-1A,
# REQ-2026-10-02-driver-de-falsificacao-paralelo-espera-para-sempre-por-chunk-
# que-nao-termina-limite-de-tempo-por-chunk-com-fail-nomeado.md).
#
# O que cada braço afirma:
#   Arm 1 (invalid-timeout) — TRACKFW_FALSIFY_CHUNK_TIMEOUT inválido aborta o
#     driver antes de criar qualquer processo, rc≠0 com mensagem.
#   Arm 2 (normal-completion) — chunks sintéticos que terminam normalmente:
#     rc=0 e nenhuma linha FAIL timeout no output.
#   Arm 3 (hung-chunk) — chunk que dorme para sempre com neto em background:
#     rc≠0 em até T+10 s, linha FAIL [falsify-driver/chunk-timeout], árvore
#     impressa, e 0 processos do marcador sobreviventes após a saída do driver.
#   Arm 4 (stdin-isolation) — chunk lançado sob set -m com </dev/null explícito
#     não lê bytes do stdin do driver (ajuste 8 da Wave 0).
#   Arm 5 (contra-braco) — o driver de 0bf66679 (sem timeout) NÃO termina
#     dentro de um teto externo com o mesmo chunk sintético travado.
#
# Plataforma: macOS e Linux. Em Git Bash / MSYS (uname com MINGW ou MSYS),
# kill -- -PGID não existe — skipa com mensagem declarada (resíduo R4, Wave 0).
#
# Nota AC5: o driver requer bash 4+ (mapfile). Este autoteste usa o bash do
# PATH (#!/usr/bin/env bash), que é o mesmo que o make usa (bash 5.x no macOS
# com Homebrew). /bin/bash no macOS é bash 3.2 e não pode rodar o driver.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DRIVER="$SCRIPT_DIR/run-gates-falsify-parallel.sh"

# --- Plataforma: skip em Git Bash / MSYS -----------------------------------
_uname=$(uname -s 2>/dev/null || true)
if [[ "$_uname" == MINGW* || "$_uname" == MSYS* ]]; then
  echo "check-falsify-chunk-timeout: SKIP -- Git Bash / MSYS detectado (uname='$_uname'): kill -- -PGID nao existe nessa plataforma (residuo R4, Wave 0 ML-0A)" >&2
  exit 0
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
SCRATCH=$(mktemp -d "${TMPDIR:-/tmp}/check-falsify-chunk-timeout.XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
TOTAL=0

_ok() {
  PASS=$((PASS + 1))
  TOTAL=$((TOTAL + 1))
  echo "OK   [falsify-driver/$1] $2"
}

_fail() {
  TOTAL=$((TOTAL + 1))
  echo "FAIL [falsify-driver/$1] $2" >&2
}

# gen_normal_gen <outdir>: escreve o gerador Python para chunks normais.
# Cada chunk emite OK [falsify/synthetic-ok] e CHUNK_COMPLETE N.
_write_gen_normal() {
  local out="$1"
  cat > "$out" << 'PYEOF'
#!/usr/bin/env python3
import sys, os, stat
outdir = sys.argv[2]
njobs = int(sys.argv[3])
n = max(njobs, 2)
for i in range(n):
    p = os.path.join(outdir, f"chunk_{i}.sh")
    with open(p, 'w') as f:
        f.write(f"""#!/usr/bin/env bash
set -euo pipefail
echo "OK [falsify/synthetic-ok] chunk_{i} completed normally"
echo "CHUNK_COMPLETE {i}"
""")
    os.chmod(p, 0o755)
    print(f"chunk={i} label=synthetic-ok")
PYEOF
  chmod +x "$out"
}

# _write_gen_hung <outdir> <marker> <n_hung>: gerador com n_hung chunks que
# dormem para sempre (com neto em background) e (n_hung + 1) chunks normais.
# marker é um número usado em "sleep <marker>" para identificar sobreviventes.
_write_gen_hung() {
  local out="$1" marker="$2"
  cat > "$out" << PYEOF
#!/usr/bin/env python3
import sys, os, stat
outdir = sys.argv[2]
njobs = int(sys.argv[3])
n_chunks = max(njobs, 2)
marker = $marker

for i in range(n_chunks):
    p = os.path.join(outdir, f"chunk_{i}.sh")
    if i < n_chunks - 1:
        # chunks travados: neto em background + loop infinito
        with open(p, 'w') as f:
            f.write(f"""#!/usr/bin/env bash
# synthetic hung chunk -- marker={marker}
sleep {marker} &
while :; do sleep {marker}; done
echo "CHUNK_COMPLETE {i}"
""")
        os.chmod(p, 0o755)
        print(f"chunk={i} label=synthetic-ok")
    else:
        # último chunk: termina normalmente
        with open(p, 'w') as f:
            f.write(f"""#!/usr/bin/env bash
set -euo pipefail
echo "OK [falsify/synthetic-ok] chunk_{i} completed normally"
echo "CHUNK_COMPLETE {i}"
""")
        os.chmod(p, 0o755)
        print(f"chunk={i} label=synthetic-ok")
PYEOF
  chmod +x "$out"
}

# _write_gen_stdin_probe <outdir>: gerador com um chunk que sonda stdin
# emitindo STDIN_RECEIVED:<line> se leu algo, STDIN_ISOLATED se não leu.
_write_gen_stdin_probe() {
  local out="$1"
  cat > "$out" << 'PYEOF'
#!/usr/bin/env python3
import sys, os, stat
outdir = sys.argv[2]
for i in range(2):
    p = os.path.join(outdir, f"chunk_{i}.sh")
    if i == 0:
        # probe: read with 1s timeout from stdin
        with open(p, 'w') as f:
            f.write(f"""#!/usr/bin/env bash
if read -t 1 -r _stdin_line <&0 2>/dev/null; then
  echo "STDIN_RECEIVED:$_stdin_line"
else
  echo "STDIN_ISOLATED"
fi
echo "OK [falsify/synthetic-stdin] stdin probe done"
echo "CHUNK_COMPLETE {i}"
""")
    else:
        with open(p, 'w') as f:
            f.write(f"""#!/usr/bin/env bash
set -euo pipefail
echo "OK [falsify/synthetic-ok] chunk_{i} ok"
echo "CHUNK_COMPLETE {i}"
""")
    os.chmod(p, 0o755)
    print(f"chunk={i} label=synthetic-stdin" if i == 0 else f"chunk={i} label=synthetic-ok")
PYEOF
  chmod +x "$out"
}

echo "=== check-falsify-chunk-timeout: iniciando autoteste ==="
echo ""

# ---------------------------------------------------------------------------
# Arm 1 — invalid timeout: driver aborta com mensagem, rc≠0
# Afirma: TRACKFW_FALSIFY_CHUNK_TIMEOUT inválido é rejeitado antes de qualquer
# execução de chunk.
# ---------------------------------------------------------------------------
echo "=== [arm 1] invalid-timeout: TRACKFW_FALSIFY_CHUNK_TIMEOUT=abc → rc≠0 ==="
arm1_dir="$SCRATCH/arm1"
mkdir -p "$arm1_dir"
_write_gen_normal "$arm1_dir/gen.py"
arm1_rc=0
arm1_stderr="$arm1_dir/err.txt"
TRACKFW_FALSIFY_GEN="$arm1_dir/gen.py" \
  TRACKFW_FALSIFY_JOBS=2 \
  TRACKFW_FALSIFY_CHUNK_TIMEOUT=abc \
  bash "$DRIVER" >/dev/null 2>"$arm1_stderr" || arm1_rc=$?
if [[ "$arm1_rc" -eq 0 ]]; then
  _fail "invalid-timeout" "driver deveria ter saido rc≠0 com CHUNK_TIMEOUT=abc, mas saiu 0"
  exit 1
fi
if ! grep -qF "invalido" "$arm1_stderr"; then
  _fail "invalid-timeout" "mensagem de erro esperada nao encontrada em stderr (arquivo: $arm1_stderr)"
  exit 1
fi
_ok "invalid-timeout" "driver abortou rc=$arm1_rc com mensagem de invalido"

# ---------------------------------------------------------------------------
# Arm 2 — normal completion: chunks rapidos → rc=0, sem linha FAIL timeout
# Afirma: o mecanismo de timeout nao interfere em chunks que terminam normalmente.
# ---------------------------------------------------------------------------
echo ""
echo "=== [arm 2] normal-completion: chunks sinteticos rapidos → rc=0 ==="
arm2_dir="$SCRATCH/arm2"
mkdir -p "$arm2_dir"
_write_gen_normal "$arm2_dir/gen.py"
arm2_rc=0
arm2_out="$arm2_dir/out.txt"
arm2_err="$arm2_dir/err.txt"
TRACKFW_FALSIFY_GEN="$arm2_dir/gen.py" \
  TRACKFW_FALSIFY_JOBS=2 \
  TRACKFW_FALSIFY_CHUNK_TIMEOUT=30 \
  bash "$DRIVER" >"$arm2_out" 2>"$arm2_err" || arm2_rc=$?
if [[ "$arm2_rc" -ne 0 ]]; then
  _fail "normal-completion" "driver reprovou (rc=$arm2_rc) com chunks normais; stderr: $(cat "$arm2_err")"
  exit 1
fi
if grep -qF "FAIL [falsify-driver/chunk-timeout]" "$arm2_err" 2>/dev/null; then
  _fail "normal-completion" "linha FAIL timeout apareceu mesmo com chunks normais"
  exit 1
fi
_ok "normal-completion" "rc=0, sem linha de timeout; OK [falsify/synthetic-ok] encontrado no output"

# ---------------------------------------------------------------------------
# Arm 3 — hung-chunk: chunk que dorme para sempre com neto em background.
# Marker 9981743 identifica os processos sobreviventes em ps.
# Afirma: rc≠0 em T+10 s, linha FAIL nomeada, arvore impressa, 0 sobreviventes.
# ---------------------------------------------------------------------------
echo ""
echo "=== [arm 3] hung-chunk: chunk com sleep 9981743 + neto, T=5 → rc≠0, FAIL, arvore, 0 sobreviventes ==="
arm3_dir="$SCRATCH/arm3"
mkdir -p "$arm3_dir"
_write_gen_hung "$arm3_dir/gen.py" 9981743
arm3_rc=0
arm3_err="$arm3_dir/err.txt"
arm3_start=$(date +%s)
TRACKFW_FALSIFY_GEN="$arm3_dir/gen.py" \
  TRACKFW_FALSIFY_JOBS=2 \
  TRACKFW_FALSIFY_CHUNK_TIMEOUT=5 \
  bash "$DRIVER" >/dev/null 2>"$arm3_err" || arm3_rc=$?
arm3_end=$(date +%s)
arm3_elapsed=$((arm3_end - arm3_start))

if [[ "$arm3_rc" -eq 0 ]]; then
  _fail "hung-chunk/rc" "driver deveria ter saido rc≠0 com chunk travado, mas saiu 0"
  exit 1
fi

# Verificar que terminou em T+10 s (T=5, ceiling=15)
if [[ "$arm3_elapsed" -gt 15 ]]; then
  _fail "hung-chunk/time" "driver demorou ${arm3_elapsed}s (ceiling T+10=15s excedido)"
  exit 1
fi

# Verificar linha FAIL nomeada
if ! grep -qF "FAIL [falsify-driver/chunk-timeout]" "$arm3_err"; then
  _fail "hung-chunk/fail-line" "linha 'FAIL [falsify-driver/chunk-timeout]' nao encontrada em stderr"
  cat "$arm3_err" >&2
  exit 1
fi

# Verificar que a arvore foi impressa (deve conter o marker)
if ! grep -q "9981743" "$arm3_err"; then
  _fail "hung-chunk/tree" "arvore de processos nao impressa (marker 9981743 nao encontrado em stderr)"
  cat "$arm3_err" >&2
  exit 1
fi

# Verificar guarda de conjunto rodou (chunk morto → sentinela ausente)
if ! grep -qF "GUARDA -- chunk_" "$arm3_err"; then
  _fail "hung-chunk/guard" "guarda de conjunto nao relatou ausencia do sentinela CHUNK_COMPLETE"
  cat "$arm3_err" >&2
  exit 1
fi

# Verificar 0 sobreviventes (dar 1 s para o SO limpar).
# Usa comm= + args= para filtrar somente processos cujo executavel é "sleep",
# evitando falso-positivo onde a linha de comando do shell pai (que contém o
# padrão "sleep 9981743" como parte do texto do script) casa contra si mesma.
sleep 1
_ps_snap3="$SCRATCH/ps_snap3.txt"
ps -A -o comm= -o args= > "$_ps_snap3" 2>/dev/null || true
arm3_survivors=$( awk '$1=="sleep" && $0~/9981743/{c++} END{print c+0}' "$_ps_snap3" )
if [[ "$arm3_survivors" -ne 0 ]]; then
  _fail "hung-chunk/survivors" "$arm3_survivors processo(s) sleep 9981743 sobreviventes apos saida do driver"
  exit 1
fi

_ok "hung-chunk/rc" "rc=$arm3_rc≠0"
_ok "hung-chunk/time" "terminou em ${arm3_elapsed}s (≤ T+10=15s)"
_ok "hung-chunk/fail-line" "linha FAIL [falsify-driver/chunk-timeout] presente em stderr"
_ok "hung-chunk/tree" "arvore de processos impressa (marker 9981743 encontrado)"
_ok "hung-chunk/guard" "guarda de conjunto relatou ausencia de CHUNK_COMPLETE"
_ok "hung-chunk/survivors" "0 processos sobreviventes (sleep 9981743)"

# ---------------------------------------------------------------------------
# Arm 4 — stdin isolation: chunk lançado com set -m + </dev/null nao le stdin
# Afirma: set -m + </dev/null explícito isola stdin do driver (ajuste 8 Wave 0).
# ---------------------------------------------------------------------------
echo ""
echo "=== [arm 4] stdin-isolation: chunk lançado sob set -m nao le bytes do stdin do driver ==="
arm4_dir="$SCRATCH/arm4"
mkdir -p "$arm4_dir"
_write_gen_stdin_probe "$arm4_dir/gen.py"
arm4_rc=0
arm4_out="$arm4_dir/out.txt"
arm4_err="$arm4_dir/err.txt"
# Pipe some bytes into driver stdin -- the chunk must NOT receive them
echo "SENTINEL_STDIN_DATA_9981745" | \
  TRACKFW_FALSIFY_GEN="$arm4_dir/gen.py" \
  TRACKFW_FALSIFY_JOBS=2 \
  TRACKFW_FALSIFY_CHUNK_TIMEOUT=30 \
  bash "$DRIVER" >"$arm4_out" 2>"$arm4_err" || arm4_rc=$?
if [[ "$arm4_rc" -ne 0 ]]; then
  _fail "stdin-isolation/rc" "driver reprovou (rc=$arm4_rc); stderr: $(cat "$arm4_err")"
  exit 1
fi
if grep -qF "STDIN_RECEIVED" "$arm4_out"; then
  _fail "stdin-isolation/probe" "chunk leu bytes do stdin do driver (STDIN_RECEIVED encontrado no log)"
  exit 1
fi
if ! grep -qF "STDIN_ISOLATED" "$arm4_out"; then
  _fail "stdin-isolation/probe" "STDIN_ISOLATED nao encontrado (chunk nao confirmou isolamento)"
  exit 1
fi
_ok "stdin-isolation" "chunk nao leu stdin do driver (STDIN_ISOLATED confirmado)"

# ---------------------------------------------------------------------------
# Arm 5 — contra-braço: driver de 0bf66679 NAO termina com chunk travado
# dentro do teto externo.
# Afirma: sem o mecanismo de timeout, wait "$pid" bloqueia indefinidamente;
# a prova e que o driver continua vivo no teto enquanto o novo driver ja saiu.
# ---------------------------------------------------------------------------
echo ""
echo "=== [arm 5] contra-braco: driver de 0bf66679 nao termina em teto externo ==="

OLD_COMMIT="0bf66679"
_git_ok=0
git -C "$REPO_ROOT" cat-file -e "${OLD_COMMIT}^{commit}" 2>/dev/null && _git_ok=1 || true
if [[ "$_git_ok" -eq 0 ]]; then
  echo "check-falsify-chunk-timeout: contra-braco SKIP -- commit $OLD_COMMIT nao disponivel no objeto git local (shallow clone?)" >&2
  _ok "contra-braco/skip" "commit $OLD_COMMIT nao disponivel -- skip declarado"
else
  # Montar copia do driver antigo em diretorio que respeita ROOT_DIR
  arm5_dir="$SCRATCH/arm5/scripts"
  mkdir -p "$arm5_dir"
  git -C "$REPO_ROOT" show "${OLD_COMMIT}:scripts/run-gates-falsify-parallel.sh" > "$arm5_dir/run-gates-falsify-parallel.sh"
  git -C "$REPO_ROOT" show "${OLD_COMMIT}:scripts/lib-crlf-normalize.sh" > "$arm5_dir/lib-crlf-normalize.sh"
  OLD_DRIVER="$arm5_dir/run-gates-falsify-parallel.sh"

  # Gerador para o contra-braco (marker diferente: 9981744)
  arm5_gen="$SCRATCH/arm5/gen.py"
  cat > "$arm5_gen" << 'PYEOF'
#!/usr/bin/env python3
import sys, os, stat
outdir = sys.argv[2]
njobs = max(int(sys.argv[3]), 2)
for i in range(njobs):
    p = os.path.join(outdir, f"chunk_{i}.sh")
    if i == 0:
        with open(p, 'w') as f:
            f.write("""#!/usr/bin/env bash
sleep 9981744 &
while :; do sleep 9981744; done
echo "CHUNK_COMPLETE 0"
""")
    else:
        with open(p, 'w') as f:
            f.write(f"""#!/usr/bin/env bash
set -euo pipefail
echo "OK [falsify/synthetic-ok] chunk_{i} ok"
echo "CHUNK_COMPLETE {i}"
""")
    os.chmod(p, 0o755)
    print(f"chunk={i} label=synthetic-ok")
PYEOF
  chmod +x "$arm5_gen"

  # Teto externo: T=5 (novo driver termina em ~7s), ceiling=15s.
  # O driver antigo deveria estar AINDA VIVO ao atingir o teto.
  CONTRA_CEILING=15
  arm5_old_rc=0
  arm5_old_pid=""
  arm5_old_pgid=""

  # Lançar o driver antigo com monitor mode para ter PGID
  set -m
  TRACKFW_FALSIFY_GEN="$arm5_gen" \
    TRACKFW_FALSIFY_JOBS=2 \
    bash "$OLD_DRIVER" >/dev/null 2>/dev/null &
  arm5_old_pid=$!
  set +m
  arm5_old_pgid="$arm5_old_pid"

  # Aguardar o teto externo
  sleep "$CONTRA_CEILING"

  # Verificar que o driver antigo AINDA está vivo (travado em wait "$pid")
  arm5_still_alive=0
  kill -0 "$arm5_old_pid" 2>/dev/null && arm5_still_alive=1 || true

  # Matar o driver antigo e seus chunks
  kill -9 -- "-$arm5_old_pgid" 2>/dev/null || true
  wait "$arm5_old_pid" 2>/dev/null || true
  sleep 0.5
  # Limpar sleep 9981744 caso tenham sobrevivido (grupo kill pode ter perdido setsid).
  # Usa comm= para evitar que o shell pai (cujo args= contém o padrão) case contra si.
  _ps_snap5="$SCRATCH/ps_snap5.txt"
  ps -A -o comm= -o pid= -o args= > "$_ps_snap5" 2>/dev/null || true
  _old_surv=$( awk '$1=="sleep" && $0~/9981744/{print $2}' "$_ps_snap5" || true )
  for _p in $_old_surv; do kill -9 "$_p" 2>/dev/null || true; done

  if [[ "$arm5_still_alive" -eq 0 ]]; then
    _fail "contra-braco" "driver antigo ($OLD_COMMIT) terminou antes do teto de ${CONTRA_CEILING}s -- contra-braco falhou (driver antigo pode ter mudado ou o chunk sintetico nao travou)"
    exit 1
  fi
  _ok "contra-braco" "driver antigo ($OLD_COMMIT) ainda vivo em ${CONTRA_CEILING}s: prova que bloqueia em wait sem timeout"
fi

# ---------------------------------------------------------------------------
# Resultado final
# ---------------------------------------------------------------------------
echo ""
echo "=== check-falsify-chunk-timeout: $PASS/$TOTAL bracos passaram ==="
if [[ "$PASS" -ne "$TOTAL" ]]; then
  echo "check-falsify-chunk-timeout: FAIL -- $((TOTAL - PASS)) braco(s) reprovaram" >&2
  exit 1
fi
echo "check-falsify-chunk-timeout: OK"
exit 0
