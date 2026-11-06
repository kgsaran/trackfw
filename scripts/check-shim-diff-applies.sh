#!/usr/bin/env bash
# check-shim-diff-applies.sh — classifica se o diff de um PR toca apenas caminhos
# inertes para o shim-byte-identity (ADR-2026-11-06 D1/D2).
#
# Conjunto inerte (critério ADR D1: não altera o que shim-byte-identity compara):
#   docs/*                  — documentação de projeto (raiz, prefixo ancorado)
#   vault/*                 — notas de knowledge vault
#   .claude/agent-memory/*  — memória de agentes
#   *.md sem /              — Markdown na raiz do repositório
#
# Modos de invocação:
#   --classify   lê env EVENT_NAME, BASE_REF, DEFAULT_BRANCH, BASE_SHA, HEAD_SHA
#                escreve diff_is_inert=<true|false> em $GITHUB_OUTPUT ou stdout
#                retorna 0 sempre (D2: falha no diff → false + job sucede)
#   --self-test  verifica os casos de falsificação e sai com rc != 0 se algum falhar
#
# ADR-2026-11-06: D1 (conjunto inerte), D2 (fail-closed), ADR-2026-09-17 (versão BASE).
set -euo pipefail

export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# is_inert <path>
# Função pura: retorna 0 (inerte) ou 1 (aplicável).
# Prefixo ANCORADO — nunca substring. Consultar T1/T2 do threat model.
# ---------------------------------------------------------------------------
is_inert() {
  local f="$1"
  [[ "$f" == docs/*                 ]] && return 0
  [[ "$f" == vault/*                ]] && return 0
  [[ "$f" == .claude/agent-memory/* ]] && return 0
  [[ "$f" == *.md && "$f" != */*    ]] && return 0
  return 1
}

# ---------------------------------------------------------------------------
# fetch_diff_to_file <tree_a> <tree_b> <outfile> [gitdir]
# Executa git diff -z --name-only --no-renames e grava resultado em outfile.
# --no-renames: um rename lista AMBOS os caminhos (origem e destino) —
# defesa contra T3 (rename de não-inerte para inerte: origem aparece).
# -z: NUL-delimitado, compatível com read -d '' (defesa contra R4).
# Retorna o exit code do git: 0 em sucesso, != 0 em falha (SHA inexistente,
# clone raso, etc.).
# ---------------------------------------------------------------------------
fetch_diff_to_file() {
  local tree_a="$1" tree_b="$2" outfile="$3"
  local gitdir="${4:-$REPO_ROOT}"
  git -C "$gitdir" diff -z --name-only --no-renames \
      "$tree_a" "$tree_b" > "$outfile" 2>/dev/null || return $?
  return 0
}

# ---------------------------------------------------------------------------
# emit_result <true|false> <mensagem>
# Escreve diff_is_inert= em $GITHUB_OUTPUT (se definido) ou em stdout.
# ---------------------------------------------------------------------------
emit_result() {
  local verdict="$1" message="$2"
  echo "$message"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "diff_is_inert=$verdict" >> "${GITHUB_OUTPUT}"
  else
    echo "diff_is_inert=$verdict"
  fi
}

# ---------------------------------------------------------------------------
# classify_tmpfile <tmpfile>
# Lê caminhos NUL-delimitados de tmpfile e emite o veredito via emit_result.
# Retorna 0 sempre.
# ---------------------------------------------------------------------------
classify_tmpfile() {
  local tmpfile="$1"
  local count=0 non_inert="" f inert_list=""
  while IFS= read -r -d '' f; do
    [[ -z "$f" ]] && continue
    count=$((count + 1))
    if ! is_inert "$f"; then
      non_inert="$f"
      break
    fi
    inert_list="${inert_list}  $f"$'\n'
  done < "$tmpfile"

  if [[ "$count" -eq 0 ]]; then
    emit_result "false" "diff vazio → aplicável (fail-closed)"
  elif [[ -n "$non_inert" ]]; then
    emit_result "false" "nao-inerte: $non_inert → aplicável"
  else
    echo "todos $count arquivo(s) são inertes (lista no log do job classify):"
    printf '%s' "$inert_list"
    emit_result "true" "diff inerte → não aplicável"
  fi
  return 0
}

# ---------------------------------------------------------------------------
# run_classify
# Ponto de entrada do modo --classify. Nunca falha: D2 garante que
# indisponibilidade do diff também produz diff_is_inert=false + exit 0.
# ---------------------------------------------------------------------------
run_classify() {
  # Repositório pelo cwd, nunca pela localização do script (D2).
  # O CI copia este script para $RUNNER_TEMP e roda-o com cwd = checkout.
  local repo_root
  if ! repo_root=$(git rev-parse --show-toplevel 2>/dev/null); then
    emit_result "false" "git rev-parse --show-toplevel falhou (cwd não é repositório git?) → aplicável (D2)"
    return 0
  fi

  local event="${EVENT_NAME:-}"
  local base_ref="${BASE_REF:-}"
  local default_branch="${DEFAULT_BRANCH:-}"
  local base_sha="${BASE_SHA:-}"
  local head_sha="${HEAD_SHA:-}"

  # Checagens de pré-condição (exit 0 — D2)
  if [[ "$event" != "pull_request" ]]; then
    emit_result "false" "event='$event' != pull_request → aplicável (D2)"
    return 0
  fi
  if [[ -z "$base_ref" ]]; then
    emit_result "false" "BASE_REF vazio → aplicável (D2)"
    return 0
  fi
  if [[ -z "$default_branch" ]]; then
    emit_result "false" "DEFAULT_BRANCH vazio → aplicável (D2)"
    return 0
  fi
  if [[ "$base_ref" != "$default_branch" ]]; then
    emit_result "false" "base='$base_ref' != default='$default_branch' → aplicável"
    return 0
  fi

  # Merge-base (falha → D2)
  local mb
  if ! mb=$(git -C "$repo_root" merge-base "$base_sha" "$head_sha" 2>/dev/null); then
    emit_result "false" "git merge-base falhou (SHA indisponível?) → aplicável (D2)"
    return 0
  fi

  # Diff (falha → D2)
  local tmpfile
  tmpfile=$(mktemp)
  if ! fetch_diff_to_file "$mb" "$head_sha" "$tmpfile" "$repo_root"; then
    rm -f "$tmpfile"
    emit_result "false" "git diff falhou → aplicável (D2)"
    return 0
  fi

  classify_tmpfile "$tmpfile"
  rm -f "$tmpfile"
  return 0
}

# ---------------------------------------------------------------------------
# run_self_test
# Verifica todos os casos de falsificação do AC2 da REQ.
# Sai com rc != 0 se algum caso falhar.
# ---------------------------------------------------------------------------
run_self_test() {
  local tmp
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT

  local fail=0
  local pass=0

  # Desliga GITHUB_OUTPUT para que emit_result escreva em stdout
  local saved_github_output="${GITHUB_OUTPUT:-}"
  unset GITHUB_OUTPUT

  # Restaura na saída (além do trap de cleanup)
  trap 'rm -rf "$tmp"; [[ -n "$saved_github_output" ]] && export GITHUB_OUTPUT="$saved_github_output"' EXIT

  # Helpers
  st_pass() { echo "OK   [$1]"; pass=$((pass + 1)); }
  st_fail() { echo "FAIL [$1]: $2"; fail=$((fail + 1)); }

  # -------------------------------------------------------------------------
  # Parte 1: função pura is_inert (sem git)
  # -------------------------------------------------------------------------

  # Caso 1a: docs/ inerte
  if is_inert "docs/CONTRIBUTING.md"; then
    st_pass "1a: docs/CONTRIBUTING.md → inerte"
  else
    st_fail "1a" "docs/CONTRIBUTING.md devia ser inerte"
  fi

  # Caso 1b: vault/ inerte
  if is_inert "vault/notes/index.md"; then
    st_pass "1b: vault/notes/index.md → inerte"
  else
    st_fail "1b" "vault/notes/index.md devia ser inerte"
  fi

  # Caso 1c: .claude/agent-memory/ inerte
  if is_inert ".claude/agent-memory/note.md"; then
    st_pass "1c: .claude/agent-memory/note.md → inerte"
  else
    st_fail "1c" ".claude/agent-memory/note.md devia ser inerte"
  fi

  # Caso 1d: *.md raiz (sem /) inerte
  if is_inert "README.md"; then
    st_pass "1d: README.md → inerte"
  else
    st_fail "1d" "README.md devia ser inerte"
  fi

  # Caso 1e: CHANGELOG.md inerte
  if is_inert "CHANGELOG.md"; then
    st_pass "1e: CHANGELOG.md → inerte"
  else
    st_fail "1e" "CHANGELOG.md devia ser inerte"
  fi

  # Caso 2: internal/ não inerte
  if ! is_inert "internal/main.go"; then
    st_pass "2: internal/main.go → não inerte"
  else
    st_fail "2" "internal/main.go NÃO devia ser inerte"
  fi

  # npm/ não inerte
  if ! is_inert "npm/x.js"; then
    st_pass "2b: npm/x.js → não inerte"
  else
    st_fail "2b" "npm/x.js NÃO devia ser inerte"
  fi

  # cmd/ não inerte
  if ! is_inert "cmd/main.go"; then
    st_pass "2c: cmd/main.go → não inerte"
  else
    st_fail "2c" "cmd/main.go NÃO devia ser inerte"
  fi

  # Caso 3: raiz não-.md não inerte
  if ! is_inert "Makefile"; then
    st_pass "3: Makefile → não inerte"
  else
    st_fail "3" "Makefile NÃO devia ser inerte"
  fi

  # Caso 4: caminho desconhecido não inerte
  if ! is_inert "mystery.bin"; then
    st_pass "4: mystery.bin → não inerte (fail-closed)"
  else
    st_fail "4" "mystery.bin NÃO devia ser inerte"
  fi

  # Caso 11: internal/x/docs/a.md → não inerte (prefixo ancorado)
  if ! is_inert "internal/x/docs/a.md"; then
    st_pass "11: internal/x/docs/a.md → não inerte (prefixo ancorado)"
  else
    st_fail "11" "internal/x/docs/a.md NÃO devia ser inerte — prefixo ancorado requerido"
  fi

  # Caso 12: npm/README.md → não inerte (tem / logo, não é raiz)
  if ! is_inert "npm/README.md"; then
    st_pass "12: npm/README.md → não inerte (contém /)"
  else
    st_fail "12" "npm/README.md NÃO devia ser inerte"
  fi

  # -------------------------------------------------------------------------
  # Parte 2: fetch_diff_to_file com git real
  # -------------------------------------------------------------------------
  local gitdir="$tmp/repo"
  mkdir -p "$gitdir"

  # Inicializa repositório (sem commits, sem refs de autoria)
  git -C "$gitdir" init -q
  # Desabilita globals de git que poderiam interferir
  git -C "$gitdir" config user.email "test@test.test"
  git -C "$gitdir" config user.name "Test"

  local diffout="$tmp/diff.out"

  # --- T1: inerte puro (apenas docs/) ---
  mkdir -p "$gitdir/docs"
  echo "a" > "$gitdir/docs/a.md"
  git -C "$gitdir" add docs/a.md
  local T1_docs
  T1_docs=$(git -C "$gitdir" write-tree)

  # Diff de empty-tree para T1_docs: apenas docs/a.md
  local empty_tree
  empty_tree=$(git -C "$gitdir" hash-object -t tree /dev/null)
  local diff_rc=0
  fetch_diff_to_file "$empty_tree" "$T1_docs" "$diffout" "$gitdir" || diff_rc=$?
  if [[ "$diff_rc" -ne 0 ]]; then
    st_fail "T1-git" "fetch_diff_to_file falhou inesperadamente (rc=$diff_rc)"
  else
    local out_classify
    out_classify=$(classify_tmpfile "$diffout" && true)
    if echo "$out_classify" | grep -q "diff_is_inert=true"; then
      st_pass "T1-git: inerte puro (docs/a.md) → true"
    else
      st_fail "T1-git" "esperava diff_is_inert=true, got: $out_classify"
    fi
  fi

  # --- T2: arquivo de internal/ não inerte ---
  mkdir -p "$gitdir/internal"
  echo "x" > "$gitdir/internal/x.go"
  git -C "$gitdir" add internal/x.go
  local T2_internal
  T2_internal=$(git -C "$gitdir" write-tree)

  diff_rc=0
  fetch_diff_to_file "$T1_docs" "$T2_internal" "$diffout" "$gitdir" || diff_rc=$?
  if [[ "$diff_rc" -ne 0 ]]; then
    st_fail "T2-git" "fetch_diff_to_file falhou (rc=$diff_rc)"
  else
    out_classify=$(classify_tmpfile "$diffout" && true)
    if echo "$out_classify" | grep -q "diff_is_inert=false"; then
      st_pass "T2-git: internal/x.go → false"
    else
      st_fail "T2-git" "esperava diff_is_inert=false, got: $out_classify"
    fi
  fi

  # --- T3: diff vazio (mesmo tree) ---
  diff_rc=0
  fetch_diff_to_file "$T2_internal" "$T2_internal" "$diffout" "$gitdir" || diff_rc=$?
  if [[ "$diff_rc" -ne 0 ]]; then
    st_fail "T3-git" "fetch_diff_to_file falhou (rc=$diff_rc)"
  else
    out_classify=$(classify_tmpfile "$diffout" && true)
    if echo "$out_classify" | grep -q "diff_is_inert=false"; then
      st_pass "T3-git: diff vazio → false (fail-closed)"
    else
      st_fail "T3-git" "esperava diff_is_inert=false para diff vazio, got: $out_classify"
    fi
  fi

  # --- T4: rename de inerte para fora (vault/x.md → src/y.go) ---
  # T_vault: só vault/x.md
  mkdir -p "$gitdir/vault"
  echo "v" > "$gitdir/vault/x.md"
  # Reset index para incluir apenas vault/x.md
  local idx_save="$tmp/index.bak"
  cp "$gitdir/.git/index" "$idx_save" 2>/dev/null || true
  git -C "$gitdir" read-tree --empty
  git -C "$gitdir" add "$gitdir/vault/x.md"
  local T_vault
  T_vault=$(git -C "$gitdir" write-tree)

  # T_src: só src/y.go (mesmo conteúdo, caminho diferente — simula rename)
  mkdir -p "$gitdir/src"
  echo "v" > "$gitdir/src/y.go"
  git -C "$gitdir" read-tree --empty
  git -C "$gitdir" add "$gitdir/src/y.go"
  local T_src
  T_src=$(git -C "$gitdir" write-tree)
  # Restaura index
  if [[ -f "$idx_save" ]]; then
    cp "$idx_save" "$gitdir/.git/index"
  fi

  diff_rc=0
  fetch_diff_to_file "$T_vault" "$T_src" "$diffout" "$gitdir" || diff_rc=$?
  if [[ "$diff_rc" -ne 0 ]]; then
    st_fail "T4-git" "fetch_diff_to_file falhou (rc=$diff_rc)"
  else
    # --no-renames mostra ambos; src/y.go é não-inerte → false
    out_classify=$(classify_tmpfile "$diffout" && true)
    if echo "$out_classify" | grep -q "diff_is_inert=false"; then
      st_pass "T4-git: rename vault/x.md→src/y.go, --no-renames mostra src/y.go → false"
    else
      st_fail "T4-git" "esperava diff_is_inert=false (src/y.go não inerte), got: $out_classify"
    fi
  fi

  # --- T5: rename de fora para inerte (internal/x.go → docs/x.go) ---
  # T_internal_only: só internal/x.go
  git -C "$gitdir" read-tree --empty
  echo "x" > "$gitdir/internal/xr.go"
  git -C "$gitdir" add "$gitdir/internal/xr.go"
  local T_int_only
  T_int_only=$(git -C "$gitdir" write-tree)

  # T_docs_only: só docs/x.go (mesmo conteúdo — simula rename)
  mkdir -p "$gitdir/docs"
  echo "x" > "$gitdir/docs/xr.go"
  git -C "$gitdir" read-tree --empty
  git -C "$gitdir" add "$gitdir/docs/xr.go"
  local T_docs_only
  T_docs_only=$(git -C "$gitdir" write-tree)

  diff_rc=0
  fetch_diff_to_file "$T_int_only" "$T_docs_only" "$diffout" "$gitdir" || diff_rc=$?
  if [[ "$diff_rc" -ne 0 ]]; then
    st_fail "T5-git" "fetch_diff_to_file falhou (rc=$diff_rc)"
  else
    # --no-renames mostra: internal/xr.go (delete) + docs/xr.go (add)
    # internal/xr.go não é inerte → false
    out_classify=$(classify_tmpfile "$diffout" && true)
    if echo "$out_classify" | grep -q "diff_is_inert=false"; then
      st_pass "T5-git: rename internal/xr.go→docs/xr.go, --no-renames mostra internal/xr.go → false"
    else
      st_fail "T5-git" "esperava diff_is_inert=false (internal/xr.go aparece), got: $out_classify"
    fi
  fi

  # --- T6: base ausente (SHA inexistente) ---
  diff_rc=0
  fetch_diff_to_file "0000000000000000000000000000000000000000" "$T2_internal" "$diffout" "$gitdir" || diff_rc=$?
  if [[ "$diff_rc" -ne 0 ]]; then
    st_pass "T6-git: base ausente → fetch_diff_to_file retorna rc=$diff_rc (não zero)"
  else
    st_fail "T6-git" "fetch_diff_to_file devia falhar com SHA inexistente, mas retornou 0"
  fi

  # -------------------------------------------------------------------------
  # Parte 3: casos de env (via subshell --classify)
  # -------------------------------------------------------------------------

  # Caso 8: base ≠ branch padrão → false
  # Usa "${BASH_SOURCE[0]}" para funcionar tanto do repositório quanto de cópia em diretório arbitrário.
  local classify_out
  classify_out=$(
    EVENT_NAME=pull_request \
    BASE_REF=feature/docs \
    DEFAULT_BRANCH=main \
    BASE_SHA=0000000000000000000000000000000000000000 \
    HEAD_SHA=0000000000000000000000000000000000000001 \
    GITHUB_OUTPUT="" \
    bash "${BASH_SOURCE[0]}" --classify 2>&1 || true
  )
  if echo "$classify_out" | grep -q "diff_is_inert=false"; then
    st_pass "8: base != default branch → false"
  else
    st_fail "8" "esperava diff_is_inert=false para base≠default, got: $classify_out"
  fi

  # Caso 9: evento != pull_request → false
  classify_out=$(
    EVENT_NAME=push \
    BASE_REF=main \
    DEFAULT_BRANCH=main \
    BASE_SHA=0000000000000000000000000000000000000000 \
    HEAD_SHA=0000000000000000000000000000000000000001 \
    GITHUB_OUTPUT="" \
    bash "${BASH_SOURCE[0]}" --classify 2>&1 || true
  )
  if echo "$classify_out" | grep -q "diff_is_inert=false"; then
    st_pass "9: event=push → false"
  else
    st_fail "9" "esperava diff_is_inert=false para evento!=pull_request, got: $classify_out"
  fi

  # Caso 7: base ausente em --classify (merge-base falha) → false
  classify_out=$(
    EVENT_NAME=pull_request \
    BASE_REF=main \
    DEFAULT_BRANCH=main \
    BASE_SHA=0000000000000000000000000000000000000000 \
    HEAD_SHA=0000000000000000000000000000000000000001 \
    GITHUB_OUTPUT="" \
    bash "${BASH_SOURCE[0]}" --classify 2>&1 || true
  )
  if echo "$classify_out" | grep -q "diff_is_inert=false"; then
    st_pass "7: base SHA inexistente em --classify → false (merge-base falha)"
  else
    st_fail "7" "esperava diff_is_inert=false para SHA inexistente, got: $classify_out"
  fi

  # -------------------------------------------------------------------------
  # Parte 4: script fora do repositório, cwd dentro (reproduz forma do CI)
  # Cria repositório git com dois commits via plumbing puro, copia o script
  # para diretório separado e roda com cwd = repositório temporário.
  # -------------------------------------------------------------------------
  local plumb_repo script_copy_dir
  plumb_repo=$(mktemp -d)
  script_copy_dir=$(mktemp -d)

  # Inicializa repositório
  git -C "$plumb_repo" init -q
  git -C "$plumb_repo" config user.email "test@test.test"
  git -C "$plumb_repo" config user.name "Test"

  # Commit base: internal/x.go
  mkdir -p "$plumb_repo/internal"
  printf 'x\n' > "$plumb_repo/internal/x.go"
  local blob_x
  blob_x=$(git -C "$plumb_repo" hash-object -w "$plumb_repo/internal/x.go")
  git -C "$plumb_repo" update-index --add --cacheinfo 100644,"$blob_x","internal/x.go"
  local tree_base
  tree_base=$(git -C "$plumb_repo" write-tree)
  local commit_base
  commit_base=$(
    GIT_AUTHOR_NAME="Test" GIT_AUTHOR_EMAIL="test@test.test" \
    GIT_COMMITTER_NAME="Test" GIT_COMMITTER_EMAIL="test@test.test" \
    git -C "$plumb_repo" commit-tree "$tree_base" -m "base"
  )

  # Commit head-inerte: acrescenta apenas docs/a.md (diff inerte)
  mkdir -p "$plumb_repo/docs"
  printf 'a\n' > "$plumb_repo/docs/a.md"
  local blob_adoc
  blob_adoc=$(git -C "$plumb_repo" hash-object -w "$plumb_repo/docs/a.md")
  git -C "$plumb_repo" read-tree "$tree_base"
  git -C "$plumb_repo" update-index --add --cacheinfo 100644,"$blob_adoc","docs/a.md"
  local tree_inert
  tree_inert=$(git -C "$plumb_repo" write-tree)
  local commit_inert
  commit_inert=$(
    GIT_AUTHOR_NAME="Test" GIT_AUTHOR_EMAIL="test@test.test" \
    GIT_COMMITTER_NAME="Test" GIT_COMMITTER_EMAIL="test@test.test" \
    git -C "$plumb_repo" commit-tree "$tree_inert" -p "$commit_base" -m "head-inert"
  )

  # Copia o script para diretório separado (fora do repositório)
  cp "${BASH_SOURCE[0]}" "$script_copy_dir/classifier-base.sh"

  # T10a: script fora do repo, cwd = repo, diff só em docs/ → true
  classify_out=$(
    cd "$plumb_repo" && \
    EVENT_NAME=pull_request \
    BASE_REF=main \
    DEFAULT_BRANCH=main \
    BASE_SHA="$commit_base" \
    HEAD_SHA="$commit_inert" \
    GITHUB_OUTPUT="" \
    bash "$script_copy_dir/classifier-base.sh" --classify 2>&1 || true
  )
  if echo "$classify_out" | grep -q "diff_is_inert=true"; then
    st_pass "T10a: script fora do repo, cwd=repo, diff inerte (docs/a.md) → true"
  else
    st_fail "T10a" "esperava diff_is_inert=true, got: $classify_out"
  fi

  # Commit head-não-inerte: acrescenta internal/y.go (diff não inerte)
  printf 'y\n' > "$plumb_repo/internal/y.go"
  local blob_y
  blob_y=$(git -C "$plumb_repo" hash-object -w "$plumb_repo/internal/y.go")
  git -C "$plumb_repo" read-tree "$tree_base"
  git -C "$plumb_repo" update-index --add --cacheinfo 100644,"$blob_y","internal/y.go"
  local tree_noninert
  tree_noninert=$(git -C "$plumb_repo" write-tree)
  local commit_noninert
  commit_noninert=$(
    GIT_AUTHOR_NAME="Test" GIT_AUTHOR_EMAIL="test@test.test" \
    GIT_COMMITTER_NAME="Test" GIT_COMMITTER_EMAIL="test@test.test" \
    git -C "$plumb_repo" commit-tree "$tree_noninert" -p "$commit_base" -m "head-noninert"
  )

  # T10b: script fora do repo, cwd = repo, diff em internal/ → false
  classify_out=$(
    cd "$plumb_repo" && \
    EVENT_NAME=pull_request \
    BASE_REF=main \
    DEFAULT_BRANCH=main \
    BASE_SHA="$commit_base" \
    HEAD_SHA="$commit_noninert" \
    GITHUB_OUTPUT="" \
    bash "$script_copy_dir/classifier-base.sh" --classify 2>&1 || true
  )
  if echo "$classify_out" | grep -q "diff_is_inert=false"; then
    st_pass "T10b: script fora do repo, cwd=repo, diff não inerte (internal/y.go) → false"
  else
    st_fail "T10b" "esperava diff_is_inert=false, got: $classify_out"
  fi

  rm -rf "$plumb_repo" "$script_copy_dir"

  # -------------------------------------------------------------------------
  # Resumo
  # -------------------------------------------------------------------------
  echo ""
  echo "self-test: $pass passou(aram), $fail falhou(aram)"
  if [[ "$fail" -ne 0 ]]; then
    exit 1
  fi
  exit 0
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------
case "${1:-}" in
  --classify)
    run_classify
    ;;
  --self-test)
    run_self_test
    ;;
  *)
    echo "uso: check-shim-diff-applies.sh --classify | --self-test" >&2
    exit 1
    ;;
esac
