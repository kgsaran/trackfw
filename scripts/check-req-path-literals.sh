#!/usr/bin/env bash
# check-req-path-literals.sh — gate AC5 (REQ-2026-09-17-sync-enumera-req-por-caminho-literal-...)
#
# Garante que nenhum código em internal/** (fora do resolvedor) e scripts/*.sh enumera
# REQs ou roadmaps por caminho literal hardcoded — padrão que ignora req_dir e roadmap_dir.
#
# DEFEITO PREVENIDO: internal/sync/sync.go usava filepath.Glob("docs/req/*.md"), ignorando
# a configuração req_dir inteiramente. Num projeto com req_dir: docs/requisições, o sync
# encontrava 0 REQs e podia criar issues a partir de arquivos residuais.
#
# PADRÃO DETECTADO:
#   Em *.go: literais de glob que enumeram REQs — "docs/req/*.md" ou variações.
#            Mais especificamente: a sequência `"docs/req/` seguida de `*.md"`.
#   Em *.sh: o padrão `docs/req/*.md` fora de comentários.
#
# ALLOWLIST EXPLÍCITA (sítios legítimos com justificativa):
#   internal/config/config.go      — default da struct ProjectConfig (REQDir: "docs/req")
#   internal/commands/configure.go — valor padrão exibido no wizard de setup
#   internal/commands/help.go      — texto de ajuda (Default: "docs/req")
#   internal/generators/scaffold.go — scaffold de diretório e template de comentário
#   internal/discover/discover.go  — lista de candidatos para descoberta automática de config
#
# EXCLUSÕES:
#   *_test.go                      — fixtures de teste criam o diretório "docs/req" por necessidade
#   scripts/check-req-path-literals.sh (este arquivo) — contém o padrão como alvo de busca
#   scripts/testdata/              — corpus frozen, nunca executado como código
#   scripts/check-gates-falsify.sh — corpus de falsificação, não código de produto

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Allowlist de arquivos legítimos em internal/ que contêm "docs/req" como
# valor de configuração ou texto de ajuda — NÃO como enumerador de arquivos.
# Adicionar novo sítio EXIGE comentário de justificativa.
# ---------------------------------------------------------------------------
ALLOWED_GO=(
  "internal/config/config.go"           # default REQDir da struct ProjectConfig
  "internal/commands/configure.go"      # valor padrão no wizard de setup interativo
  "internal/commands/help.go"           # texto de ajuda do CLI (Default: "docs/req")
  "internal/generators/scaffold.go"     # scaffold de diretório e template de comentário
  "internal/discover/discover.go"       # candidatos para descoberta automática de config
)

# ---------------------------------------------------------------------------
# Busca em *.go sob internal/ (excluindo _test.go e os sítios da allowlist).
# Padrão: "docs/req/*.md" como literal de glob (o defeito exato).
# ---------------------------------------------------------------------------
FAIL=0
FOUND=0

echo "=== check-req-path-literals: scanning internal/**/*.go (non-test) ==="

# Constrói o argumento de exclusão da allowlist para grep --exclude
EXCLUDE_ARGS=()
for f in "${ALLOWED_GO[@]}"; do
  EXCLUDE_ARGS+=(--exclude="$(basename "$f")")
done

# Scan de Go: procura o padrão de glob literal em arquivos não-teste
while IFS= read -r match; do
  [[ -z "$match" ]] && continue
  FOUND=1
  echo "FAIL $match"
  FAIL=1
done < <(
  grep -rn '"docs/req/\*\.md"' \
    --include="*.go" \
    --exclude="*_test.go" \
    "${EXCLUDE_ARGS[@]}" \
    "$REPO_ROOT/internal/" 2>/dev/null \
    | grep -v "^Binary"
)

# ---------------------------------------------------------------------------
# Busca em scripts/*.sh (excluindo este próprio script, testdata e falsify).
# Padrão: docs/req/*.md fora de linha de comentário (#).
# ---------------------------------------------------------------------------
echo "=== check-req-path-literals: scanning scripts/*.sh ==="

while IFS= read -r match; do
  [[ -z "$match" ]] && continue
  FOUND=1
  echo "FAIL $match"
  FAIL=1
done < <(
  find "$REPO_ROOT/scripts" -maxdepth 1 -name "*.sh" \
    ! -name "check-req-path-literals.sh" \
    ! -name "check-gates-falsify.sh" \
    2>/dev/null \
  | while IFS= read -r script; do
      grep -n 'docs/req/\*\.md' "$script" 2>/dev/null \
        | grep -v '^\s*#' \
        | sed "s|^|${script}:|" || true
    done
)

echo ""
if [[ "$FOUND" -eq 0 ]]; then
  echo "check-req-path-literals: OK — no literal REQ glob patterns found outside the resolver"
  exit 0
fi

if [[ "$FAIL" -ne 0 ]]; then
  echo ""
  echo "check-req-path-literals: FAIL — literal REQ glob patterns detected outside allowed sítios"
  echo "Fix: use validator.ResolveREQFiles(cfg) or resolveREQFiles(cfg) instead of a literal glob."
  exit 1
fi

echo "check-req-path-literals: OK"
exit 0
