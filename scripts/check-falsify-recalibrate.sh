#!/usr/bin/env bash
# check-falsify-recalibrate.sh — autoteste de falsify-recalibrate.sh
# (ML-1A parte 2, REQ-2026-10-03 / issue #403).
#
# O que cada braço afirma:
#   Arm 1 (completo) — run completo (4 shards com timing válidos) produz exit 0
#     e o JSON de destino contém o rótulo r0 (em weights.r0).
#   Arm 2 (shard-ausente) — shard 2 sem timing_2.log: exit≠0, mensagem contém
#     "2", destino intocado (checksum antes == checksum depois).
#   Arm 3 (fork) — head_repository=fork/trackfw: exit≠0 antes de chamar
#     `run download` (o gh falso registra cada chamada; nenhuma chamada a
#     "run download" pode aparecer no log).
#   Arm 4 (ts-invalido) — ts=1.2.3 num shard: exit≠0 (gen-falsify-scenario-weights.py
#     rejeita o timestamp malformado).
#
# Prova de mordida:
#   S1: desativa a verificação do head_repo (passo a) e mostra que o braço do
#       fork REPROVA (o script aceita fork → a guarda do passo (a) era o que bloqueava).
#   S2: desativa a verificação de timing ausente (passo d) e mostra que o braço
#       do shard faltando REPROVA (o script aceita shard ausente → era o passo (d)).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RECALIBRATE="$SCRIPT_DIR/falsify-recalibrate.sh"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
SCRATCH=$(mktemp -d "${TMPDIR:-/tmp}/check-falsify-recalibrate.XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
TOTAL=0

_ok() {
  PASS=$((PASS + 1))
  TOTAL=$((TOTAL + 1))
  echo "PASS [falsify-recalibrate/$1] $2"
}

_fail() {
  TOTAL=$((TOTAL + 1))
  echo "FAIL [falsify-recalibrate/$1] $2" >&2
}

# Número de shards efetivo (lido do workflow, mesmo mecanismo do script real).
WF="$REPO_ROOT/.github/workflows/quality.yml"
SHARD_COUNT=$(grep -m1 'FALSIFY_SHARD_COUNT:' "$WF" | sed -E 's/.*FALSIFY_SHARD_COUNT:[[:space:]]*"?([0-9]+)"?.*/\1/') || true
if [[ -z "$SHARD_COUNT" || ! "$SHARD_COUNT" =~ ^[0-9]+$ ]]; then
  echo "check-falsify-recalibrate: nao foi possivel ler FALSIFY_SHARD_COUNT de $WF" >&2
  exit 1
fi

# _make_gh_fake <dir> <head_repo> [missing_shard] [bad_ts_shard]
# Escreve um script `gh` falso em <dir>/gh que:
#   - registra cada chamada em <dir>/gh-calls.log
#   - responde ao `api ...` com head_repo
#   - no `run download`, cria falsify-shard-N/timing_N.log para todos os
#     shards EXCETO missing_shard (se definido)
#   - se bad_ts_shard definido, escreve ts=1.2.3 nesse shard
_make_gh_fake() {
  local dir="$1" head_repo="$2" missing_shard="${3:-}" bad_ts_shard="${4:-}"
  mkdir -p "$dir"
  local gh_script="$dir/gh"
  # Expandir variaveis antes de gravar o heredoc
  local count="$SHARD_COUNT"
  cat > "$gh_script" << GHEOF
#!/usr/bin/env bash
echo "\$*" >> "$dir/gh-calls.log"
CMD="\$1"; shift || true
if [[ "\$CMD" == "api" ]]; then
  echo "$head_repo"
  exit 0
fi
if [[ "\$CMD" == "run" && "\${1:-}" == "download" ]]; then
  DLDIR=""
  while [[ \$# -gt 0 ]]; do
    [[ "\$1" == "--dir" ]] && { shift; DLDIR="\$1"; shift; continue; }
    shift
  done
  [[ -z "\$DLDIR" ]] && { echo "gh-fake: --dir nao encontrado" >&2; exit 1; }
  for n in \$(seq 0 $((count - 1))); do
    [[ -n "$missing_shard" && "\$n" == "$missing_shard" ]] && continue
    shard_dir="\$DLDIR/falsify-shard-\$n"
    mkdir -p "\$shard_dir"
    ts_start="1.0"
    [[ -n "$bad_ts_shard" && "\$n" == "$bad_ts_shard" ]] && ts_start="1.2.3"
    printf 'FALSIFY_TIMING phase=start block=b%s labels=r%s ts=%s\n' "\$n" "\$n" "\$ts_start" > "\$shard_dir/timing_\$n.log"
    printf 'FALSIFY_TIMING phase=end block=b%s labels=r%s ts=2.5\n' "\$n" "\$n" >> "\$shard_dir/timing_\$n.log"
  done
  exit 0
fi
echo "gh-fake: comando desconhecido: \$CMD \$*" >&2; exit 1
GHEOF
  chmod +x "$gh_script"
}

# _run_recalibrate <script> <gh_dir> <dest> [extra args...]
# Roda <script> com GH_BIN=<gh_dir>/gh e FALSIFY_WEIGHTS_OUT=<dest>.
# Retorna o exit code em RC_OUT (global).
RC_OUT=0
_run_recalibrate() {
  local script="$1" gh_dir="$2" dest="$3"
  RC_OUT=0
  GH_BIN="$gh_dir/gh" \
    FALSIFY_WEIGHTS_OUT="$dest" \
    bash "$script" "99999" >"$SCRATCH/_run_out.txt" 2>&1 || RC_OUT=$?
}

echo "=== check-falsify-recalibrate: iniciando autoteste ==="
echo ""

# ---------------------------------------------------------------------------
# Arm 1 — completo: 4 shards com timing validos → exit 0 e JSON contem r0
# Afirma: run completo produz calibracao bem-sucedida e JSON com rótulo r0.
# ---------------------------------------------------------------------------
echo "=== [arm 1] completo: 4 shards validos → exit 0 e JSON contem weights.r0 ==="
arm1_dir="$SCRATCH/arm1"
_make_gh_fake "$arm1_dir/fakegh" "kgsaran/trackfw" "" ""
DEST1="$arm1_dir/weights.json"
_run_recalibrate "$RECALIBRATE" "$arm1_dir/fakegh" "$DEST1"
if [[ "$RC_OUT" -ne 0 ]]; then
  _fail "completo/rc" "esperava exit 0, obteve $RC_OUT; output: $(cat "$SCRATCH/_run_out.txt")"
elif [[ ! -f "$DEST1" ]]; then
  _fail "completo/json-exists" "JSON de destino nao foi criado"
elif ! python3 -c "
import json, sys
d = json.load(open('$DEST1'))
w = d.get('weights', d)
sys.exit(0 if 'r0' in w else 1)
" 2>/dev/null; then
  _fail "completo/json-r0" "JSON nao contem chave r0 em weights: $(cat "$DEST1" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(list(d.get("weights",d).keys()))' 2>/dev/null || cat "$DEST1")"
else
  _ok "completo" "exit 0 e JSON contem weights.r0"
fi

# ---------------------------------------------------------------------------
# Arm 2 — shard-ausente: shard 2 sem timing_2.log → exit≠0, msg "2", intocado
# Afirma: shard faltando impede escrita do destino (integridade da calibração).
# ---------------------------------------------------------------------------
echo ""
echo "=== [arm 2] shard-ausente: timing_2.log ausente → exit≠0, msg '2', destino intocado ==="
arm2_dir="$SCRATCH/arm2"
_make_gh_fake "$arm2_dir/fakegh" "kgsaran/trackfw" "2" ""
DEST2="$arm2_dir/weights.json"
echo '{"preexistente": 1}' > "$DEST2"
CKSUM_BEFORE=$(shasum -a 256 "$DEST2" | awk '{print $1}')
_run_recalibrate "$RECALIBRATE" "$arm2_dir/fakegh" "$DEST2"
if [[ "$RC_OUT" -eq 0 ]]; then
  _fail "shard-ausente/rc" "esperava exit≠0, obteve 0"
else
  CKSUM_AFTER=$(shasum -a 256 "$DEST2" | awk '{print $1}')
  if ! grep -q "2" "$SCRATCH/_run_out.txt"; then
    _fail "shard-ausente/msg" "mensagem de erro nao contem '2': $(cat "$SCRATCH/_run_out.txt")"
  elif [[ "$CKSUM_BEFORE" != "$CKSUM_AFTER" ]]; then
    _fail "shard-ausente/intocado" "destino foi modificado (cksum antes=$CKSUM_BEFORE depois=$CKSUM_AFTER)"
  else
    _ok "shard-ausente" "exit≠0, mensagem contem '2', destino intocado"
  fi
fi

# ---------------------------------------------------------------------------
# Arm 3 — fork: head_repository=fork/trackfw → exit≠0, run download nao chamado
# Afirma: verificação do repo de origem bloqueia antes de baixar artefatos.
# ---------------------------------------------------------------------------
echo ""
echo "=== [arm 3] fork: head_repository=fork/trackfw → exit≠0, run download nao chamado ==="
arm3_dir="$SCRATCH/arm3"
_make_gh_fake "$arm3_dir/fakegh" "fork/trackfw" "" ""
DEST3="$arm3_dir/weights.json"
_run_recalibrate "$RECALIBRATE" "$arm3_dir/fakegh" "$DEST3"
if [[ "$RC_OUT" -eq 0 ]]; then
  _fail "fork/rc" "esperava exit≠0, obteve 0"
elif grep -q "run download" "$arm3_dir/fakegh/gh-calls.log" 2>/dev/null; then
  _fail "fork/no-download" "run download foi chamado mesmo com fork: $(cat "$arm3_dir/fakegh/gh-calls.log")"
else
  _ok "fork" "exit≠0 e run download nao foi chamado"
fi

# ---------------------------------------------------------------------------
# Arm 4 — ts-invalido: ts=1.2.3 num shard → exit≠0
# Afirma: timestamp malformado aborta a calibração.
# ---------------------------------------------------------------------------
echo ""
echo "=== [arm 4] ts-invalido: ts=1.2.3 no shard 0 → exit≠0 ==="
arm4_dir="$SCRATCH/arm4"
_make_gh_fake "$arm4_dir/fakegh" "kgsaran/trackfw" "" "0"
DEST4="$arm4_dir/weights.json"
_run_recalibrate "$RECALIBRATE" "$arm4_dir/fakegh" "$DEST4"
if [[ "$RC_OUT" -eq 0 ]]; then
  _fail "ts-invalido" "esperava exit≠0 com ts=1.2.3, obteve 0"
else
  _ok "ts-invalido" "exit≠0 com timestamp malformado (ts=1.2.3)"
fi

# ---------------------------------------------------------------------------
# Prova de mordida S1: desativa passo (a) → braço do fork deve retornar exit 0
# (o script aceita o fork, confirmando que era o passo (a) que bloqueava).
# ---------------------------------------------------------------------------
echo ""
echo "=== [S1] prova de mordida: desativar passo (a) → fork retorna exit 0 ==="

# Criar script sabotado com o bloco de verificação de head_repo desativado.
# Gravado em SCRATCH (não em SCRIPT_DIR) para não sujar scripts/ se abortado.
# FALSIFY_REPO_ROOT é passado na execução para que o sabotado resolva o workflow
# e o gerador Python a partir da raiz real do repo, não de SCRATCH.
SABOTADO="$SCRATCH/falsify-recalibrate-sabotado.sh"

# Construir script sabotado: copia o original mas substitui a verificação do fork
# por um no-op (a linha 'if [[ ... != ... ]]; then' vira 'if false; then').
awk '
  /if \[\[ "\$head_repo" != "kgsaran\/trackfw" \]\]/ {
    print "if false; then  # SABOTADO-S1: verificacao de fork desativada"
    next
  }
  { print }
' "$RECALIBRATE" > "$SABOTADO"
chmod +x "$SABOTADO"

s1_dir="$SCRATCH/s1"
_make_gh_fake "$s1_dir/fakegh" "fork/trackfw" "" ""
DEST_S1="$s1_dir/weights.json"
s1_rc=0
GH_BIN="$s1_dir/fakegh/gh" \
  FALSIFY_WEIGHTS_OUT="$DEST_S1" \
  FALSIFY_REPO_ROOT="$REPO_ROOT" \
  bash "$SABOTADO" "99999" >"$SCRATCH/_s1_out.txt" 2>&1 || s1_rc=$?

if [[ "$s1_rc" -eq 0 ]]; then
  _ok "mordida-S1" "passo (a) desativado: fork retornou exit 0 (confirma que era o passo (a) que bloqueava)"
else
  _fail "mordida-S1" "passo (a) desativado mas fork ainda retornou exit≠0 ($s1_rc); awk nao fez a substituicao? output: $(cat "$SCRATCH/_s1_out.txt" | tail -3)"
fi
rm -f "$SABOTADO"

# ---------------------------------------------------------------------------
# Prova de mordida S2: desativa passo (d) → shard ausente retorna exit 0
# (o script aceita shard faltando, confirmando que era o passo (d) que bloqueava).
# ---------------------------------------------------------------------------
echo ""
echo "=== [S2] prova de mordida: desativar passo (d) → shard ausente retorna exit 0 ==="

awk '
  /if \[\[ -z "\$timing_file" \|\| ! -s "\$timing_file" \]\]/ {
    print "if false; then  # SABOTADO-S2: verificacao de shard desativada"
    next
  }
  { print }
' "$RECALIBRATE" > "$SABOTADO"
chmod +x "$SABOTADO"

s2_dir="$SCRATCH/s2"
_make_gh_fake "$s2_dir/fakegh" "kgsaran/trackfw" "2" ""
DEST_S2="$s2_dir/weights.json"
s2_rc=0
GH_BIN="$s2_dir/fakegh/gh" \
  FALSIFY_WEIGHTS_OUT="$DEST_S2" \
  FALSIFY_REPO_ROOT="$REPO_ROOT" \
  bash "$SABOTADO" "99999" >"$SCRATCH/_s2_out.txt" 2>&1 || s2_rc=$?

if [[ "$s2_rc" -eq 0 ]]; then
  _ok "mordida-S2" "passo (d) desativado: shard ausente retornou exit 0 (confirma que era o passo (d) que bloqueava)"
else
  _fail "mordida-S2" "passo (d) desativado mas shard ausente ainda retornou exit≠0 ($s2_rc); awk nao fez a substituicao? output: $(cat "$SCRATCH/_s2_out.txt" | tail -3)"
fi
rm -f "$SABOTADO"

# ---------------------------------------------------------------------------
# Resultado final
# ---------------------------------------------------------------------------
echo ""
echo "=== check-falsify-recalibrate: $PASS/$TOTAL bracos passaram ==="
if [[ "$PASS" -ne "$TOTAL" ]]; then
  echo "check-falsify-recalibrate: FAIL -- $((TOTAL - PASS)) braco(s) reprovaram" >&2
  exit 1
fi
echo "check-falsify-recalibrate: OK"
exit 0
