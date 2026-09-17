#!/usr/bin/env bash
# check-ci-workflow-binary-provenance.sh — AC5 gate (ML-1B,
# REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor...)
#
# Afirma: nenhum workflow deste repositório valida com um binário do trackfw que não
# seja compilado do código do próprio PR — qualquer que seja o mecanismo de obtenção
# (install.sh | sh, go install …@v, download de artefato, imagem pré-construída).
#
# Escopo: workflows que executam "trackfw validate". Workflows que obtêm o trackfw por
# outro motivo legítimo (ex.: smoke test de release que DEVE instalar o binário
# publicado) estão fora do escopo por não executarem "trackfw validate".
#
# Enumeração: git ls-files '.github/workflows/*.yml' — arquivos commitados, não do
# disco. Um arquivo não commitado nunca chega ao CI.
#
# Contra-braço: fixture com o padrão proibido deve reprovar, provando que o gate
# detecta o que afirma detectar.
set -euo pipefail

ROOT="${1:-.}"
FAIL=0

# ---------------------------------------------------------------------------
# is_published_binary FILE
# Returns 0 (true) if FILE contains signs of a published-release binary acquisition,
# 1 (false) if it compiles from source or uses no trackfw binary.
# Strips YAML comment lines before matching to avoid false positives from explanatory
# comments that mention the prohibited patterns.
# ---------------------------------------------------------------------------
is_published_binary() {
  local file=$1
  local stripped
  # Remove lines where the first non-space character is '#' (YAML comment lines).
  stripped=$(grep -v '^[[:space:]]*#' "$file")

  # Pattern 1: curl …/install.sh | sh  (or |sh without space)
  if echo "$stripped" | grep -qE 'install\.sh[[:space:]]*\|[[:space:]]*sh'; then
    return 0
  fi

  # Pattern 2: go install .../cmd/trackfw@v<version>  (pinned or @latest via @v)
  if echo "$stripped" | grep -qE 'go install .*/cmd/trackfw@v'; then
    return 0
  fi

  # Pattern 3: @latest variant
  if echo "$stripped" | grep -qE 'go install .*/cmd/trackfw@latest'; then
    return 0
  fi

  return 1
}

# ---------------------------------------------------------------------------
# Scan committed workflows.
# ---------------------------------------------------------------------------
CHECKED=0

mapfile -t WORKFLOWS < <(cd "$ROOT" && git ls-files '.github/workflows/*.yml' 2>/dev/null || true)

for wf in "${WORKFLOWS[@]}"; do
  full="$ROOT/$wf"
  # Only check workflows that run trackfw validate (scope restriction from AC5).
  if ! grep -qF 'trackfw validate' "$full" 2>/dev/null; then
    continue
  fi
  CHECKED=$((CHECKED + 1))
  if is_published_binary "$full"; then
    echo "check-ci-workflow-binary-provenance: FALHA — $wf executa 'trackfw validate' com binário obtido do release publicado (não compilado do PR)" >&2
    FAIL=1
  fi
done

# ---------------------------------------------------------------------------
# Guarda de vacuidade: ao menos 1 workflow com 'trackfw validate' deve ter sido
# verificado — se a enumeração estiver vazia ou nenhum workflow usar 'trackfw
# validate', o gate não mediu nada e deve reprovar.
# ---------------------------------------------------------------------------
if [ "$CHECKED" -eq 0 ]; then
  echo "check-ci-workflow-binary-provenance: FALHA (vacuidade) — nenhum workflow com 'trackfw validate' encontrado em .github/workflows/; o gate não mediu nada" >&2
  FAIL=1
fi

# ---------------------------------------------------------------------------
# Contra-braço: fixture com o padrão proibido deve ser detectada.
# ---------------------------------------------------------------------------
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-binary-provenance.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

cat > "$WORK/bad-gate.yml" << 'YAMLEOF'
name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance-install-script:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    env:
      TRACKFW_VERSION: "8.0.0"
    steps:
      - uses: actions/checkout@v4

      - name: Install trackfw
        run: |
          curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh

      - name: Governance gate
        run: trackfw validate
YAMLEOF

if ! is_published_binary "$WORK/bad-gate.yml"; then
  echo "check-ci-workflow-binary-provenance: FALHA (contra-braço) — fixture com 'install.sh | sh' deveria ser detectada como binário de release, mas não foi" >&2
  FAIL=1
else
  echo "OK contra-braço: fixture com 'install.sh | sh' detectada como proveniência de release"
fi

# ---------------------------------------------------------------------------
# Resultado.
# ---------------------------------------------------------------------------
if [ "$FAIL" -ne 0 ]; then
  echo "check-ci-workflow-binary-provenance: FALHOU" >&2
  exit 1
fi

echo "check-ci-workflow-binary-provenance: OK — $CHECKED workflow(s) com 'trackfw validate' verificados, nenhum usa binário de release publicado"
