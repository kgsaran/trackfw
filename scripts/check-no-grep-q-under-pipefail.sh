#!/usr/bin/env bash
# check-no-grep-q-under-pipefail.sh — no pipe-grep-q em scripts que usam pipefail,
# e no pipe-grep-q em arquivos de workflow do GitHub Actions (passos com shell: bash
# rodam sob pipefail; acrescentar shell: bash a um passo sem ele ativa o 141 sem aviso).
# Linked to: make parity-rest (ADR-2026-11-06 D5)
#
# WHY THIS EXISTS — D5 (ADR-2026-11-06):
#   Under 'set -o pipefail', pipe-grep-q exits 141 (SIGPIPE) intermittently:
#   grep exits on first match while the producer is still writing.
#   The fix: '| grep PATTERN >/dev/null' — grep reads all input, same exit codes.
#   Measured: 141 141 0 0 0 0 141 141 in 8 runs of the same pipeline.
#   This gate prevents the pattern from returning.
#
# B3 (ML-2B): extendido para varrer .github/workflows/*.yml. Passos com shell: bash
# rodam sob pipefail (bash --noprofile --norc -eo pipefail {0}); acrescentar shell: bash
# a um passo sem shell ativa o 141 sem aviso. Por isso a regra e uniforme em workflow:
# todo | grep -q e reprovado independentemente de o passo declarar shell: ou nao.
#
# USAGE
#   bash scripts/check-no-grep-q-under-pipefail.sh           # check scripts/ + .github/workflows/
#   bash scripts/check-no-grep-q-under-pipefail.sh --self-test
#   bash scripts/check-no-grep-q-under-pipefail.sh --scan-root /path/to/dir
#   # --scan-root DIR: scans DIR/*.sh AND DIR/*.yml (flat, used by --self-test)
#   #   Both must have at least one file; the gate fails (rc=2) if either set is empty.

set -euo pipefail

SELF=$(basename "${BASH_SOURCE[0]}")
SCRIPTS_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
MODE=scan
SCAN_DIR="$SCRIPTS_DIR"
# WORKFLOW_DIR: for default scan, the actual .github/workflows directory.
# For --scan-root, set to the same dir as SCAN_DIR (flat: yml files live directly there).
WORKFLOW_DIR="$(cd "$SCRIPTS_DIR/.." && pwd)/.github/workflows"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --self-test)  MODE=self-test; shift ;;
    --scan-root)  SCAN_DIR="$2"; WORKFLOW_DIR="$2"; shift 2 ;;
    *) printf '%s: unknown argument: %s\n' "$SELF" "$1" >&2; exit 2 ;;
  esac
done

# ---------------------------------------------------------------------------
# --self-test: run fixture-based tests in a temp directory and exit
# ---------------------------------------------------------------------------
if [[ "$MODE" == "self-test" ]]; then
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  PASS=0
  FAIL=0

  run_test() {
    local name="$1" script_content="$2" expect_pass="$3" ext="${4:-sh}"
    local f="$tmpdir/${name}.${ext}"
    printf '%s\n' "$script_content" > "$f"
    local out rc=0
    out=$(bash "${BASH_SOURCE[0]}" --scan-root "$tmpdir" 2>&1) || rc=$?
    if [[ "$expect_pass" == "pass" ]]; then
      if [[ $rc -eq 0 ]]; then
        echo "  PASS  $name"
        (( PASS++ )) || true
      else
        echo "  FAIL  $name  (expected rc=0, got rc=$rc)"
        echo "        output: $out"
        (( FAIL++ )) || true
      fi
    else
      if [[ $rc -ne 0 ]]; then
        echo "  PASS  $name"
        (( PASS++ )) || true
      else
        echo "  FAIL  $name  (expected rc!=0, got rc=0)"
        echo "        output: $out"
        (( FAIL++ )) || true
      fi
    fi
    # Remove created file so each test starts clean
    rm -f "$f"
  }

  echo "--- self-test: check-no-grep-q-under-pipefail ---"

  # Persistent benign .yml so the yml anti-vacuity guard does not trip on .sh tests.
  printf '# benign workflow\n' > "$tmpdir/__benign__.yml"

  # Flags built via variables so the script source contains no literal pipe-grep-q.
  _q=q
  _Eq=Eq
  _E_q="E -q"

  # T1: script com pipefail + pipe com flag -q → REPROVA
  # Reconciliação: o gate detecta o padrão proibido que causa 141 sob pipefail
  run_test "t1-pipefail-grep-q" \
    "$(printf '#!/usr/bin/env bash\nset -euo pipefail\ngrep -v '"'"'^\s*#'"'"' file.sh | grep -%s pattern' "$_q")" \
    "fail"

  # T2: script com pipefail + flags combinadas -Eq (q em posição não inicial) → REPROVA
  # Reconciliação: o gate detecta -q em qualquer posição dentro do grupo de flags
  run_test "t2-pipefail-grep-Eq" \
    "$(printf '#!/usr/bin/env bash\nset -euo pipefail\necho test | grep -%s foo' "$_Eq")" \
    "fail"

  # T3: script SEM pipefail + pipe com grep-q → PASSA
  # Reconciliação: sem pipefail o padrão não causa 141 e não precisa ser bloqueado
  run_test "t3-no-pipefail-grep-q" \
    "$(printf '#!/usr/bin/env bash\necho test | grep -%s foo' "$_q")" \
    "pass"

  # T4: script com pipefail mas grep lendo arquivo (sem pipe antes) → PASSA
  # Reconciliação: grep lendo arquivo diretamente não causa SIGPIPE e não deve ser bloqueado
  run_test "t4-pipefail-grep-q-file" \
    "$(printf '#!/usr/bin/env bash\nset -euo pipefail\ngrep -%s pattern file.txt' "$_q")" \
    "pass"

  # T5: linha comentada com pipe-grep-q → PASSA
  # Reconciliação: linhas comentadas são ignoradas pelo gate (não são código executado)
  run_test "t5-commented-grep-q" \
    "$(printf '#!/usr/bin/env bash\nset -euo pipefail\n# echo test | grep -%s foo' "$_q")" \
    "pass"

  # T6: | grep PATTERN >/dev/null (forma correta) → PASSA
  # Reconciliação: a forma correta (>/dev/null em vez de -q) passa o gate
  run_test "t6-grep-devnull" \
    "$(printf '#!/usr/bin/env bash\nset -euo pipefail\necho test | grep foo >/dev/null')" \
    "pass"

  # T7: flags separadas (| grep -E -q) → REPROVA
  # Reconciliação: -q como grupo de flag separado de -E também é detectado
  run_test "t7-pipefail-grep-E-dash-q" \
    "$(printf '#!/usr/bin/env bash\nset -euo pipefail\necho test | grep -%s foo' "$_E_q")" \
    "fail"

  # T8: variante egrep (| egrep -q) → REPROVA
  # Reconciliação: o gate detecta a variante egrep além de grep
  run_test "t8-pipefail-egrep-q" \
    "$(printf '#!/usr/bin/env bash\nset -euo pipefail\necho test | egrep -%s foo' "$_q")" \
    "fail"

  # --- Casos YAML: B3 (ML-2B) ---
  # Arquivo .sh benigno no tmpdir para satisfazer a guarda de vacuidade do scan de .sh
  printf '#!/usr/bin/env bash\necho ok\n' > "$tmpdir/__benign__.sh"

  # T_yml_fail: workflow yml com | grep -q → REPROVA
  # Reconciliação: o gate detecta | grep -q em workflow YAML pois passos com shell: bash
  # rodam sob pipefail; a regra e uniforme para evitar que shell: bash seja adicionado depois
  run_test "ty1-yml-grep-q" \
    "$(printf 'steps:\n  - run: echo x | grep -%s y' "$_q")" \
    "fail" \
    "yml"

  # T_yml_pass: workflow yml com | grep y >/dev/null → PASSA
  # Reconciliação: a forma correta (>/dev/null) não é bloqueada em arquivos yml
  run_test "ty2-yml-devnull" \
    "$(printf 'steps:\n  - run: echo x | grep y >/dev/null')" \
    "pass" \
    "yml"

  rm -f "$tmpdir/__benign__.sh"
  rm -f "$tmpdir/__benign__.yml"

  # T_no_yml: diretório sem *.yml/*.yaml mas com *.sh válido → REPROVA (guarda de vacuidade yml)
  # Reconciliação: com .sh presente e nenhum .yml no --scan-root, o gate sai com rc!=0;
  # o contra-braço (cópia sem a guarda) mostra que, sem ela, o scan sairia rc=0 no mesmo cenário.
  run_test "t-no-yml-vacuity" \
    "$(printf '#!/usr/bin/env bash\necho ok')" \
    "fail"

  echo "---"
  echo "Results: $PASS passed, $FAIL failed"
  if [[ $FAIL -gt 0 ]]; then
    exit 1
  fi
  exit 0
fi

# ---------------------------------------------------------------------------
# Main: scan scripts/*.sh for pipe-grep-q under pipefail
# ---------------------------------------------------------------------------

shopt -s nullglob
mapfile -t SCRIPTS < <(find "$SCAN_DIR" -maxdepth 1 -name '*.sh' -type f | sort)

# Anti-vacuity guard: fail if no scripts found
if [[ ${#SCRIPTS[@]} -eq 0 ]]; then
  echo "$SELF: ERROR: no *.sh files found in $SCAN_DIR" >&2
  exit 2
fi

VIOLATIONS=0

for script in "${SCRIPTS[@]}"; do
  # Only check scripts that use pipefail
  if ! grep -q 'pipefail' "$script" 2>/dev/null; then
    continue
  fi

  # Scan for non-commented lines with piped grep where grep uses a quiet flag (-q,
  # -Eq, -E with -q in a separate group, egrep/fgrep variant, --quiet, --silent).
  # Herestrings (<<<) are stripped before the check to avoid false positives.
  # Comment lines are skipped.
  while IFS= read -r lineno_and_line; do
    lineno="${lineno_and_line%%:*}"
    content="${lineno_and_line#*:}"

    # Skip pure comment lines
    trimmed="${content#"${content%%[! ]*}"}"   # strip leading whitespace
    [[ "$trimmed" == \#* ]] && continue

    # Strip <<< portions (herestrings are not pipes)
    stripped_heredoc="${content//<<<*/}"

    # Detection: pipe-then-[e|f]grep with -q in any flag group, or --quiet/--silent.
    # Pattern 1: | [ef]grep with zero or more non-q flag groups then a q-containing group.
    # Pattern 2: | [ef]grep with --quiet or --silent anywhere on the line.
    if grep -E '\| *[ef]*grep([[:space:]]+-[a-zA-Z]+)*[[:space:]]+-[a-zA-Z]*q[a-zA-Z]*|\| *[ef]*grep[[:space:]].*--(quiet|silent)' \
        <<<"$stripped_heredoc" >/dev/null 2>&1; then
      echo "${script}:${lineno}: ${content}"
      (( VIOLATIONS++ )) || true
    fi
  done < <(grep -n '.' "$script" | grep -E '\| *[ef]*grep')
done

# ---------------------------------------------------------------------------
# B3 (ML-2B): varre .github/workflows/*.yml em busca de pipe-grep-q.
# Passos com shell: bash rodam sob pipefail (bash --noprofile --norc -eo pipefail {0});
# passos sem shell: (Linux/macOS) rodam sob bash -e {0}, sem pipefail. Porem acrescentar
# shell: bash a um passo sem ele ativa o 141 sem aviso. Por isso a regra e uniforme:
# todo | grep -q em workflow e reprovado independentemente do shell: declarado.
# ---------------------------------------------------------------------------
mapfile -t WORKFLOWS < <(find "$WORKFLOW_DIR" -maxdepth 1 \( -name '*.yml' -o -name '*.yaml' \) -type f 2>/dev/null | sort)

# Anti-vacuity guard: fail if no workflow files found (R12 — ML-2D)
if [[ ${#WORKFLOWS[@]} -eq 0 ]]; then
  echo "$SELF: ERROR: no *.yml/*.yaml files found in $WORKFLOW_DIR" >&2
  exit 2
fi

for wf in "${WORKFLOWS[@]}"; do
  while IFS= read -r lineno_and_line; do
    lineno="${lineno_and_line%%:*}"
    content="${lineno_and_line#*:}"

    # Skip pure comment lines
    trimmed="${content#"${content%%[! ]*}"}"
    [[ "$trimmed" == \#* ]] && continue

    # Strip <<< portions
    stripped_heredoc="${content//<<<*/}"

    # Same detection pattern as for .sh files
    if grep -E '\| *[ef]*grep([[:space:]]+-[a-zA-Z]+)*[[:space:]]+-[a-zA-Z]*q[a-zA-Z]*|\| *[ef]*grep[[:space:]].*--(quiet|silent)' \
        <<<"$stripped_heredoc" >/dev/null 2>&1; then
      echo "${wf}:${lineno}: ${content}"
      (( VIOLATIONS++ )) || true
    fi
  done < <(grep -n '.' "$wf" | grep -E '\| *[ef]*grep')
done

if [[ $VIOLATIONS -gt 0 ]]; then
  printf '%s: FAIL — %d occurrence(s) of pipe-grep-q under pipefail found\n' "$SELF" "$VIOLATIONS" >&2
  exit 1
fi

echo "$SELF: OK — no pipe-grep-q under pipefail found"
exit 0
