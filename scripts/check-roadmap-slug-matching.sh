#!/usr/bin/env bash
# check-roadmap-slug-matching.sh — the branch↔roadmap matcher's calibration becomes a contract.
#
# ML-3C (AC14) of ROADMAP-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-
# comandos-e-o-segundo-se-esquece.md, implementing
# ADR-2026-09-26-precisao-do-vinculo-branch-roadmap-escrever-em-vez-de-inferir.md (D2/D4 + emenda).
#
# WHAT IT PINS
# ML-3A replaced `strings.Contains` with an ADDITIVE relation (substring OR token overlap) and
# CALIBRATED the threshold instead of choosing it: 205 governed branches of this repository
# measured against the 201 roadmaps then in wip/+done/, at four thresholds. The measurement is
# what authorised the change. This gate is that measurement, frozen:
#
#   205 branches × 201 roadmaps  →  190 accept · 15 block
#                                   of the 190: 176 by the historical substring arm,
#                                               14 ONLY by the new token arm (the repair)
#   +1 external calibration pair (issue #273) that forces the CEILING of the threshold
#
# A change to the matcher that moves ANY branch's verdict fails this gate BY NAME. The count
# alone is not the assertion — "191 became 190" tells the next developer to go guessing, and in
# this campaign guessing has been wrong every time.
#
# 🔴 WHY THE CORPUS IS A VERSIONED FIXTURE AND NOT `gh pr list` / `ls docs/roadmaps/*`
# Lesson of ML-7C of REQ-2026-08-31: there the pre-fix corpus was reachable only through a
# single local ref — on a branch `trackfw branch prune --apply` would classify as "safe to
# delete". A corpus the CI cannot reach is a corpus that does not exist. Neither `gh` (no
# credential in a fork's CI) nor the reflog (not cloned) nor the current content of
# docs/roadmaps/ (moves every time a roadmap transitions) can hold a verdict still. The corpus
# lives in scripts/testdata/branch-roadmap-slug-corpus/ and moves only by deliberate act.
#
# HOW THE VERDICT IS OBTAINED
# Through `trackfw branch new --dry-run <type>/<slug>`, which calls
# validator.BranchSlugMatchesRoadmap directly — the single implementation of the relation
# (D3). ⚠️ That path exercises the INFERENCE arm only, never ResolveBranchRoadmap: the written
# link of D1 lives in a gitignored per-checkout file, never exists in CI, and by design cannot
# be a fixture. What this gate pins is therefore the inference — which is precisely the arm
# every CI, clone and fork actually uses.
#
# ⚠️ `--dry-run` exits 0 for BOTH verdicts (it is a report, not a gate), so the discriminant is
# stdout, and any output that is neither "would create" nor "would block" is a FAIL with the
# literal text — never silently bucketed as a block.
#
# SELF-TEST (--self-test): 9 arms, four of which rebuild the matcher with a mutated relation in
# an isolated module copy (never in the real tree) and require this gate to fail NAMING the
# branches that flipped. The remaining arms falsify the non-vacuity guards: absent corpus, empty
# population, pin mismatch and a materialisation that silently loses a file.
set -uo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEFAULT_CORPUS_DIR="$ROOT_DIR/scripts/testdata/branch-roadmap-slug-corpus"
CORPUS_DIR="${TRACKFW_SLUG_CORPUS_DIR:-$DEFAULT_CORPUS_DIR}"
VALIDATOR_GO="$ROOT_DIR/internal/validator/validator.go"

# Trace of the corpus override, same reasoning as the TRACKFW_FALSIFY_* trace policy
# (check-parity-call-site-pins.sh): an override forgotten in the environment must leave a mark,
# never point the gate at another corpus in silence. The Makefile call site also unsets it.
if [[ -n "${TRACKFW_SLUG_CORPUS_DIR:-}" ]]; then
  echo "check-roadmap-slug-matching: TRACKFW_SLUG_CORPUS_DIR=$TRACKFW_SLUG_CORPUS_DIR (override ativo)" >&2
fi

FAIL=0
CHECKED=0
ok()   { echo "OK   [$1]"; CHECKED=$((CHECKED + 1)); }
bad()  { echo "FAIL [$1]: $2" >&2; FAIL=1; CHECKED=$((CHECKED + 1)); }
die()  { echo "check-roadmap-slug-matching: $1" >&2; exit 1; }

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-slug-matching.XXXXXX") || die "mktemp falhou"
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# resolve_go_bin — prefer the pinned GO_BIN from the Makefile call site (already built by the
# `build` prerequisite); fall back to bin/trackfw, then to a local build, so the gate is also
# runnable standalone. GOCACHE is deliberately NOT redirected: the fallback build reuses the
# real module cache (~1s) instead of paying a cold cache, and this gate never runs under the
# synthetic HOME that forces the falsify harness to pin it.
# ---------------------------------------------------------------------------
resolve_go_bin() {
  local candidate="${GO_BIN:-}"
  if [[ -n "$candidate" ]]; then
    [[ "$candidate" == /* ]] || candidate="$ROOT_DIR/$candidate"
    [[ -x "$candidate" ]] || die "GO_BIN=$candidate não é executável"
    echo "$candidate"
    return 0
  fi
  if [[ -x "$ROOT_DIR/bin/trackfw" ]]; then
    echo "$ROOT_DIR/bin/trackfw"
    return 0
  fi
  local built="$WORK/trackfw"
  ( cd "$ROOT_DIR" && go build -o "$built" ./cmd/trackfw ) >"$WORK/build.log" 2>&1 \
    || { sed 's/^/    /' "$WORK/build.log" >&2; die "go build do binário de medição falhou"; }
  echo "$built"
}

# data_lines FILE — the fixture payload: no comments, no blank lines.
data_lines() { grep -v '^[[:space:]]*#' "$1" | grep -v '^[[:space:]]*$'; }

# manifest_value KEY — reads key=value from the manifest, failing loudly when absent/empty.
manifest_value() {
  local key=$1 val
  val=$(data_lines "$CORPUS_DIR/manifest.txt" | sed -n "s/^${key}=//p" | head -1)
  [[ -n "$val" ]] || die "manifest.txt sem a chave obrigatória '${key}' (ou valor vazio)"
  [[ "$val" =~ ^[0-9]+$ ]] || die "manifest.txt: ${key}='${val}' não é numérico"
  echo "$val"
}

# ---------------------------------------------------------------------------
# run_corpus LABEL ROADMAPS_FILE VERDICTS_FILE EXPECTED_ROADMAPS EXPECTED_BRANCHES
#
# Materialises a synthetic project from the frozen roadmap list and asks the binary for the
# verdict of every branch in the frozen verdict list.
#
# 🔴 The materialisation identity guard is not decoration: while building this fixture, a shell
# artefact produced 402 lines for 201 files and 202 files on disk for 201 names — a corpus that
# silently shrinks or grows is a gate that keeps printing OK about a population nobody chose.
# So the number of .md files actually created is compared to the number of fixture lines, and
# both numbers are printed on failure.
# ---------------------------------------------------------------------------
run_corpus() {
  local label=$1 roadmaps_file=$2 verdicts_file=$3 exp_roadmaps=$4 exp_branches=$5

  local -a roadmaps=() rows=()
  mapfile -t roadmaps < <(data_lines "$roadmaps_file")
  mapfile -t rows < <(data_lines "$verdicts_file")

  # Non-vacuity, before anything else: an empty population must FAIL, never pass in silence.
  if [[ ${#roadmaps[@]} -eq 0 ]]; then
    bad "$label/population" "corpus de roadmaps VAZIO em $roadmaps_file — gate vácuo"
    return 0
  fi
  if [[ ${#rows[@]} -eq 0 ]]; then
    bad "$label/population" "corpus de branches VAZIO em $verdicts_file — gate vácuo"
    return 0
  fi
  if [[ ${#roadmaps[@]} -ne $exp_roadmaps ]]; then
    bad "$label/population" "roadmaps no fixture: ${#roadmaps[@]}, manifest declara $exp_roadmaps (contagem EXATA, não piso)"
    return 0
  fi
  if [[ ${#rows[@]} -ne $exp_branches ]]; then
    bad "$label/population" "branches no fixture: ${#rows[@]}, manifest declara $exp_branches (contagem EXATA, não piso)"
    return 0
  fi

  local proj="$WORK/$label-proj"
  rm -rf "$proj"
  mkdir -p "$proj/docs/roadmaps/wip" || die "mkdir do projeto sintético falhou"
  cat > "$proj/trackfw.yaml" <<'YAMLEOF'
governance_mode: lenient
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
YAMLEOF
  local name
  for name in "${roadmaps[@]}"; do
    case "$name" in
      */*) bad "$label/materialize" "nome de roadmap com separador de caminho: '$name'"; return 0 ;;
      *.md) : ;;
      *) bad "$label/materialize" "nome de roadmap sem sufixo .md: '$name'"; return 0 ;;
    esac
    : > "$proj/docs/roadmaps/wip/$name" || die "não consegui materializar '$name'"
  done

  local on_disk
  on_disk=$(find "$proj/docs/roadmaps/wip" -maxdepth 1 -name '*.md' | wc -l | tr -d ' ')
  if [[ "$on_disk" != "${#roadmaps[@]}" ]]; then
    bad "$label/materialize" "identidade fixture↔disco quebrada: ${#roadmaps[@]} linhas no fixture, $on_disk arquivos .md no disco (nome duplicado? caractere perdido?)"
    return 0
  fi
  ok "$label/materialize (${#roadmaps[@]} roadmaps, ${#rows[@]} branches)"

  local row branch expected arm out verdict mismatches=0
  local n_accept=0 n_block=0 n_substring=0 n_tokens=0
  for row in "${rows[@]}"; do
    branch=$(cut -f1 <<<"$row")
    expected=$(cut -f2 <<<"$row")
    arm=$(cut -f3 <<<"$row")
    if [[ -z "$branch" || -z "$expected" || -z "$arm" ]]; then
      bad "$label/format" "linha de verdito malformada (esperado <branch>TAB<verdict>TAB<arm>): '$row'"
      return 0
    fi
    case "$expected" in
      accept) n_accept=$((n_accept + 1)) ;;
      block)  n_block=$((n_block + 1)) ;;
      *) bad "$label/format" "verdito desconhecido '$expected' para '$branch' (use accept|block)"; return 0 ;;
    esac
    case "$arm" in
      substring) n_substring=$((n_substring + 1)) ;;
      tokens)    n_tokens=$((n_tokens + 1)) ;;
      none)      : ;;
      *) bad "$label/format" "braço desconhecido '$arm' para '$branch' (use substring|tokens|none)"; return 0 ;;
    esac

    out=$( cd "$proj" && "$MEASURE_BIN" branch new --dry-run "$branch" 2>&1 )
    case "$out" in
      *"would create"*) verdict=accept ;;
      *"would block"*)  verdict=block ;;
      *)
        # rc de --dry-run é 0 nas duas direções, então saída inesperada NUNCA é tratada como
        # "block por omissão" — é falha com o texto literal.
        bad "$label/$branch" "saída não reconhecida do binário (nem 'would create' nem 'would block'): <<<$out>>>"
        mismatches=$((mismatches + 1))
        continue
        ;;
    esac
    if [[ "$verdict" != "$expected" ]]; then
      bad "$label/$branch" "veredito mudou — esperado $expected (arm=$arm), obtido $verdict"
      mismatches=$((mismatches + 1))
    fi
  done

  # Both verdict classes present: a regeneration that flattened the corpus to all-accept would
  # otherwise keep passing while pinning nothing.
  if [[ "$label" == census ]]; then
    [[ $n_accept -gt 0 ]] || bad "$label/classes" "nenhuma branch com verdito 'accept' no fixture — corpus degenerado"
    [[ $n_block  -gt 0 ]] || bad "$label/classes" "nenhuma branch com verdito 'block' no fixture — corpus degenerado"
    [[ $n_accept -eq $MF_ACCEPT ]] || bad "$label/classes" "accept no fixture: $n_accept, manifest declara $MF_ACCEPT"
    [[ $n_block  -eq $MF_BLOCK  ]] || bad "$label/classes" "block no fixture: $n_block, manifest declara $MF_BLOCK"
    [[ $n_substring -eq $MF_SUBSTRING ]] || bad "$label/classes" "arm=substring no fixture: $n_substring, manifest declara $MF_SUBSTRING"
    [[ $n_tokens -eq $MF_TOKENS ]] || bad "$label/classes" "arm=tokens no fixture: $n_tokens, manifest declara $MF_TOKENS"
  fi

  if [[ $mismatches -eq 0 ]]; then
    ok "$label/verdicts (${#rows[@]} branches, $n_accept accept / $n_block block, zero divergência)"
  else
    echo "  → $mismatches de ${#rows[@]} branches divergiram do corpus congelado em '$label'." >&2
    echo "  → Se a mudança no matcher é intencional, REGENERE o corpus e o manifest no mesmo PR," >&2
    echo "    citando o roadmap: a calibração é contrato, não configuração." >&2
  fi
}

# ===========================================================================
# Modo normal
# ===========================================================================
main_check() {
  [[ -d "$CORPUS_DIR" ]] || die "corpus ausente: $CORPUS_DIR (fixture versionada obrigatória)"
  local f
  for f in manifest.txt census-roadmaps.txt census-verdicts.tsv calibration-roadmaps.txt calibration-verdicts.tsv; do
    [[ -s "$CORPUS_DIR/$f" ]] || die "arquivo do corpus ausente ou vazio: $CORPUS_DIR/$f"
  done
  [[ -s "$VALIDATOR_GO" ]] || die "fonte do matcher ausente: $VALIDATOR_GO"

  MF_MIN_TOKENS=$(manifest_value min_shared_tokens) || exit 1
  MF_MIN_LEN=$(manifest_value min_token_len) || exit 1
  MF_ROADMAPS=$(manifest_value census_roadmaps) || exit 1
  MF_BRANCHES=$(manifest_value census_branches) || exit 1
  MF_ACCEPT=$(manifest_value census_accept) || exit 1
  MF_BLOCK=$(manifest_value census_block) || exit 1
  MF_SUBSTRING=$(manifest_value census_arm_substring) || exit 1
  MF_TOKENS=$(manifest_value census_arm_tokens) || exit 1
  MF_CAL_ROADMAPS=$(manifest_value calibration_roadmaps) || exit 1
  MF_CAL_BRANCHES=$(manifest_value calibration_branches) || exit 1

  if [[ $((MF_ACCEPT + MF_BLOCK)) -ne $MF_BRANCHES ]]; then
    bad "manifest/coherence" "census_accept($MF_ACCEPT) + census_block($MF_BLOCK) != census_branches($MF_BRANCHES)"
  elif [[ $((MF_SUBSTRING + MF_TOKENS)) -ne $MF_ACCEPT ]]; then
    bad "manifest/coherence" "census_arm_substring($MF_SUBSTRING) + census_arm_tokens($MF_TOKENS) != census_accept($MF_ACCEPT)"
  else
    ok "manifest/coherence ($MF_BRANCHES = $MF_ACCEPT accept ($MF_SUBSTRING substring + $MF_TOKENS tokens) + $MF_BLOCK block)"
  fi

  # --- Os dois pins de calibração --------------------------------------------
  # Literal declaration, on purpose: a refactor to `var`, to a config field or to a flag would
  # turn a calibrated value back into a runtime choice, and this gate's corpus would keep
  # passing while the number it pins stopped being the number in force.
  local pin_tokens="const branchRoadmapMinSharedTokens = $MF_MIN_TOKENS"
  local pin_len="const branchRoadmapMinTokenLen = $MF_MIN_LEN"
  if grep -qxF "$pin_tokens" "$VALIDATOR_GO"; then
    ok "pin/min_shared_tokens ($MF_MIN_TOKENS)"
  else
    bad "pin/min_shared_tokens" "não achei a declaração literal '$pin_tokens' em $VALIDATOR_GO — o limiar CALIBRADO pelo ML-3A mudou (ou deixou de ser const). Atualize o Go E regenere o corpus/manifest no mesmo PR: $(grep -n 'branchRoadmapMinSharedTokens = ' "$VALIDATOR_GO" | head -1)"
  fi
  if grep -qxF "$pin_len" "$VALIDATOR_GO"; then
    ok "pin/min_token_len ($MF_MIN_LEN)"
  else
    bad "pin/min_token_len" "não achei a declaração literal '$pin_len' em $VALIDATOR_GO — o corte de tamanho de token também é calibração do ML-3A: $(grep -n 'branchRoadmapMinTokenLen = ' "$VALIDATOR_GO" | head -1)"
  fi

  MEASURE_BIN=$(resolve_go_bin) || exit 1
  echo "check-roadmap-slug-matching: binário de medição = $MEASURE_BIN"

  run_corpus census "$CORPUS_DIR/census-roadmaps.txt" "$CORPUS_DIR/census-verdicts.tsv" \
    "$MF_ROADMAPS" "$MF_BRANCHES"
  run_corpus calibration "$CORPUS_DIR/calibration-roadmaps.txt" "$CORPUS_DIR/calibration-verdicts.tsv" \
    "$MF_CAL_ROADMAPS" "$MF_CAL_BRANCHES"

  if [[ $FAIL -ne 0 ]]; then
    echo "check-roadmap-slug-matching: REPROVOU ($CHECKED verificações)" >&2
    exit 1
  fi
  echo "check-roadmap-slug-matching: OK ($CHECKED verificações; $MF_BRANCHES branches × $MF_ROADMAPS roadmaps + $MF_CAL_BRANCHES caso de calibração)"
}

# ===========================================================================
# --self-test
# ===========================================================================
# Frozen flip set for the N=2→3 mutation (measured 2026-09-26). Hardcoded instead of derived:
# deriving the expectation from the mutated build under test is the self-referential defect
# hades-tf found in ML-2D — sabotaging the target would erase the requirement with it.
SELFTEST_FLIPS_N3=(
  "feat/v2.2-cli-parity"
  "feat/v2.3-ai-agent-rail"
  "feat/v2.4-docs-site"
  "fix/falha-de-windows-por-causa-raiz"
  "fix/fecha-req-validator-improvements"
  "fix/fechar-os-grupos-de-falha-de-windows-por-causa-raiz"
  "fix/grupos-de-falha-de-windows-por-causa-raiz"
  "fix/kanban-next-ml-progress"
  "fix/req-linked-adr-roadmap-select"
)

st_ok()   { echo "OK   [self-test/$1]"; ST_CHECKED=$((ST_CHECKED + 1)); }
st_bad()  { echo "FAIL [self-test/$1]: $2" >&2; ST_FAIL=1; ST_CHECKED=$((ST_CHECKED + 1)); }

# build_mutant NAME SED_EXPR — isolated module copy with ONE constant changed, then rebuilt.
# The real tree is never touched. The substitution is verified by grep afterwards: a sed that
# matched nothing exits 0 and would leave an unmutated binary, turning a falsification arm into
# a vacuous pass.
build_mutant() {
  local name=$1 sed_expr=$2 expect_line=$3
  local dir="$WORK/mutant-$name"
  rm -rf "$dir"; mkdir -p "$dir"
  cp -R "$ROOT_DIR/cmd" "$ROOT_DIR/internal" "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$dir/" \
    || { st_bad "$name/copy" "cópia do módulo falhou"; return 1; }
  sed -i.bak "$sed_expr" "$dir/internal/validator/validator.go" \
    || { st_bad "$name/sed" "sed falhou"; return 1; }
  rm -f "$dir/internal/validator/validator.go.bak"
  if ! grep -qxF "$expect_line" "$dir/internal/validator/validator.go"; then
    st_bad "$name/sed" "mutação NÃO aplicada — esperava a linha '$expect_line' na cópia (sed no-op produziria braço vácuo)"
    return 1
  fi
  if ! ( cd "$dir" && go build -o "$WORK/trackfw-$name" ./cmd/trackfw ) >"$WORK/build-$name.log" 2>&1; then
    sed 's/^/    /' "$WORK/build-$name.log" >&2
    st_bad "$name/build" "go build da cópia mutada falhou"
    return 1
  fi
  echo "$WORK/trackfw-$name"
}

# run_gate_with BIN [CORPUS_DIR] — runs THIS gate (normal mode) and captures output + rc.
# rc is read on a line of its own: a captured assignment merged with the notification has
# already reported "exit 0" over a real rc of 2 in this campaign.
run_gate_with() {
  local bin=$1 corpus=${2:-$DEFAULT_CORPUS_DIR}
  ST_OUT=$( GO_BIN="$bin" TRACKFW_SLUG_CORPUS_DIR="$corpus" bash "$ROOT_DIR/scripts/check-roadmap-slug-matching.sh" 2>&1 )
  ST_RC=$?
}

# expect_named_flips LABEL EXPECTED_RC NAMES... — the core of AC14: the gate must not only fail,
# it must NAME every branch whose verdict moved.
expect_named_flips() {
  local label=$1 want_rc=$2; shift 2
  if [[ "$ST_RC" -eq 0 ]]; then
    st_bad "$label" "gate PASSOU com matcher mutado (rc=0) — falsificação não detectada"
    return 0
  fi
  local missing=() b
  for b in "$@"; do
    grep -qF "FAIL [census/$b]" <<<"$ST_OUT" || grep -qF "FAIL [calibration/$b]" <<<"$ST_OUT" || missing+=("$b")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    st_bad "$label" "gate reprovou (rc=$ST_RC) mas NÃO nomeou ${#missing[@]} branch(es): ${missing[*]}"
    return 0
  fi
  local flip_count
  flip_count=$( { grep -cE '^FAIL \[(census|calibration)/[^]]+\]: veredito mudou' <<<"$ST_OUT" || true; } )
  if [[ "$flip_count" != "$#" ]]; then
    st_bad "$label" "gate nomeou $flip_count flips, esperava exatamente $# (lista congelada em 2026-09-26)"
    return 0
  fi
  st_ok "$label (rc=$ST_RC, $# branches nomeadas)"
}

self_test() {
  ST_FAIL=0
  ST_CHECKED=0

  # --- Arm 1 — contra-braço: matcher correto ⇒ gate PASSA -------------------
  local base_bin="${GO_BIN:-}"
  if [[ -n "$base_bin" ]]; then
    [[ "$base_bin" == /* ]] || base_bin="$ROOT_DIR/$base_bin"
    [[ -x "$base_bin" ]] || die "self-test: GO_BIN=$base_bin não é executável"
  elif [[ -x "$ROOT_DIR/bin/trackfw" ]]; then
    base_bin="$ROOT_DIR/bin/trackfw"
  else
    base_bin="$WORK/trackfw-base"
    ( cd "$ROOT_DIR" && go build -o "$base_bin" ./cmd/trackfw ) >"$WORK/build-base.log" 2>&1 \
      || { sed 's/^/    /' "$WORK/build-base.log" >&2; die "self-test: go build do binário base falhou"; }
  fi
  run_gate_with "$base_bin"
  if [[ "$ST_RC" -eq 0 ]]; then
    st_ok "control/matcher-intacto (rc=0)"
  else
    echo "$ST_OUT" >&2
    st_bad "control/matcher-intacto" "gate reprovou com o matcher ÍNTEGRO (rc=$ST_RC) — corpus defasado ou fixture quebrado"
  fi

  # --- Arms 2/3/4 — mutação no matcher ⇒ reprova NOMEANDO --------------------
  local bin3 bin1 bin99
  if bin3=$(build_mutant n3 \
      's/^const branchRoadmapMinSharedTokens = 2$/const branchRoadmapMinSharedTokens = 3/' \
      'const branchRoadmapMinSharedTokens = 3'); then
    run_gate_with "$bin3"
    # Ceiling: the 9 historical branches AND the #273 calibration pair flip accept→block.
    expect_named_flips "mutation/N=3-teto" 1 \
      "${SELFTEST_FLIPS_N3[@]}" "feat/adrs-retroativas-da-divida-do-acervo"
  fi

  if bin1=$(build_mutant n1 \
      's/^const branchRoadmapMinSharedTokens = 2$/const branchRoadmapMinSharedTokens = 1/' \
      'const branchRoadmapMinSharedTokens = 1'); then
    run_gate_with "$bin1"
    # Floor: at N=1 the relation stops discriminating — the 15 block rows all flip to accept.
    local -a expect_block=()
    mapfile -t expect_block < <(data_lines "$DEFAULT_CORPUS_DIR/census-verdicts.tsv" | awk -F'\t' '$2=="block"{print $1}')
    if [[ ${#expect_block[@]} -eq 0 ]]; then
      st_bad "mutation/N=1-piso" "fixture não tem nenhuma linha 'block' — braço vácuo"
    else
      expect_named_flips "mutation/N=1-piso" 1 "${expect_block[@]}"
    fi
  fi

  if bin99=$(build_mutant n99 \
      's/^const branchRoadmapMinSharedTokens = 2$/const branchRoadmapMinSharedTokens = 99/' \
      'const branchRoadmapMinSharedTokens = 99'); then
    run_gate_with "$bin99"
    # Token arm effectively dead: exactly the rows marked arm=tokens (the repair of ML-3A) plus
    # the #273 pair must flip. This is what makes the third column of the fixture load-bearing.
    local -a expect_tokens=()
    mapfile -t expect_tokens < <(data_lines "$DEFAULT_CORPUS_DIR/census-verdicts.tsv" | awk -F'\t' '$3=="tokens"{print $1}')
    if [[ ${#expect_tokens[@]} -eq 0 ]]; then
      st_bad "mutation/braco-de-tokens-morto" "fixture não tem nenhuma linha 'arm=tokens' — braço vácuo"
    else
      expect_named_flips "mutation/braco-de-tokens-morto" 1 \
        "${expect_tokens[@]}" "feat/adrs-retroativas-da-divida-do-acervo"
    fi
  fi

  # --- Arm 5 — mutação no OUTRO braço: substring morto -----------------------
  # Os três arms acima movem a MESMA constante. AC14 fala de mutação no matcher, e
  # `strings.Contains(...) ||` é metade da relação — se nenhuma mutação a tocasse, o gate provaria
  # apenas o limiar.
  # 🔴 O resultado é medição própria e vale por si: com o braço substring morto, apenas DUAS das
  # 205 mudam de veredito — `feat/v2.0-gaps` e `fix/v8-um-binario`, ambas com menos de 2 tokens de
  # conteúdo de 3+ caracteres no slug. Ou seja: em 203 dos 205 casos a sobreposição de tokens já
  # SUBSUME o substring, e o raio de alcance da etapa RESTRITIVA da D4 (remover o que só o
  # substring aceita) é exatamente essas duas branches. Este arm é o instrumento que o ML-3C
  # prometeu preparar para essa etapa, não a etapa.
  local bin_sub
  if bin_sub=$(build_mutant sub \
      's/if strings\.Contains(normalizeBranchSlug(name), branchSlug) ||/if false \/* substring arm disabled *\/ ||/' \
      '			if false /* substring arm disabled */ ||'); then
    run_gate_with "$bin_sub"
    expect_named_flips "mutation/braco-substring-morto" 1 \
      "feat/v2.0-gaps" "fix/v8-um-binario"
  fi

  # --- Arm 6 — corpus ausente reprova (nunca passa em silêncio) -------------
  run_gate_with "$base_bin" "$WORK/corpus-inexistente"
  if [[ "$ST_RC" -ne 0 ]] && grep -qF "corpus ausente" <<<"$ST_OUT"; then
    st_ok "vacuity/corpus-ausente (rc=$ST_RC)"
  else
    st_bad "vacuity/corpus-ausente" "esperava reprovação nomeando 'corpus ausente' (rc=$ST_RC): $ST_OUT"
  fi

  # --- Arm 7 — população zero reprova --------------------------------------
  local empty_corpus="$WORK/corpus-vazio"
  cp -R "$DEFAULT_CORPUS_DIR" "$empty_corpus"
  grep '^[[:space:]]*#' "$DEFAULT_CORPUS_DIR/census-verdicts.tsv" > "$empty_corpus/census-verdicts.tsv"
  run_gate_with "$base_bin" "$empty_corpus"
  if [[ "$ST_RC" -ne 0 ]] && grep -qF "VAZIO" <<<"$ST_OUT"; then
    st_ok "vacuity/populacao-zero (rc=$ST_RC)"
  else
    st_bad "vacuity/populacao-zero" "esperava reprovação por população vazia (rc=$ST_RC): $ST_OUT"
  fi

  # --- Arm 8 — pin do limiar divergente reprova ----------------------------
  local pin_corpus="$WORK/corpus-pin"
  cp -R "$DEFAULT_CORPUS_DIR" "$pin_corpus"
  sed -i.bak 's/^min_shared_tokens=.*/min_shared_tokens=5/' "$pin_corpus/manifest.txt"
  rm -f "$pin_corpus/manifest.txt.bak"
  run_gate_with "$base_bin" "$pin_corpus"
  if [[ "$ST_RC" -ne 0 ]] && grep -qF "pin/min_shared_tokens" <<<"$ST_OUT"; then
    st_ok "pin/limiar-divergente (rc=$ST_RC)"
  else
    st_bad "pin/limiar-divergente" "esperava reprovação do pin do limiar (rc=$ST_RC): $ST_OUT"
  fi

  # --- Arm 9 — materialização que perde arquivo reprova --------------------
  # Duplicating a line makes the fixture declare 202 names that collapse to 201 files on disk —
  # the exact silent shrink measured while building this corpus.
  local dup_corpus="$WORK/corpus-dup"
  cp -R "$DEFAULT_CORPUS_DIR" "$dup_corpus"
  data_lines "$DEFAULT_CORPUS_DIR/census-roadmaps.txt" | head -1 >> "$dup_corpus/census-roadmaps.txt"
  sed -i.bak 's/^census_roadmaps=.*/census_roadmaps=202/' "$dup_corpus/manifest.txt"
  rm -f "$dup_corpus/manifest.txt.bak"
  run_gate_with "$base_bin" "$dup_corpus"
  if [[ "$ST_RC" -ne 0 ]] && grep -qF "identidade fixture↔disco quebrada" <<<"$ST_OUT"; then
    st_ok "vacuity/identidade-fixture-disco (rc=$ST_RC)"
  else
    st_bad "vacuity/identidade-fixture-disco" "esperava reprovação por identidade fixture↔disco (rc=$ST_RC): $ST_OUT"
  fi

  if [[ $ST_FAIL -ne 0 ]]; then
    echo "check-roadmap-slug-matching --self-test: REPROVOU ($ST_CHECKED braços)" >&2
    exit 1
  fi
  echo "check-roadmap-slug-matching --self-test: OK ($ST_CHECKED braços)"
}

case "${1:-}" in
  --self-test) self_test ;;
  "") main_check ;;
  *) die "uso: check-roadmap-slug-matching.sh [--self-test]" ;;
esac
