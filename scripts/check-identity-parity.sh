#!/usr/bin/env bash
# Gate de paridade cross-CLI da identidade personalizável de agentes.
#
# Prova que os três CLIs (Go, Node.js, Python) produzem BYTE A BYTE os mesmos
# artefatos de agente quando recebem o mesmo ~/.trackfw/identity.json, para
# todos os alvos de integração — com identidade configurada e sem identidade
# (não-regressão).
#
# Motivação: o manifest de integrações indexa artefatos por sha256 do conteúdo.
# Qualquer divergência de renderização entre CLIs faz `agents list` reportar
# falso `modified` quando o usuário instala por um CLI e lista por outro.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
# run_install faz `cd` para o diretório do projeto — GO_BIN precisa ser absoluto.
case "$GO_BIN" in /*) ;; *) GO_BIN="$ROOT_DIR/$GO_BIN" ;; esac

load_catalog_targets() {
  local catalog="$ROOT_DIR/internal/integrations/assets/catalog.json"
  local specs="$WORK_DIR/identity-catalog-targets.txt"

  if [[ ! -f "$catalog" ]]; then
    echo "Identity parity: missing canonical catalog ${catalog#$ROOT_DIR/}" >&2
    exit 1
  fi

  python3 - "$catalog" >"$specs" <<'PY'
import json
import sys

catalog_path = sys.argv[1]
with open(catalog_path, "r", encoding="utf-8") as stream:
    catalog = json.load(stream)

required = []
for target in catalog.get("targets", []):
    supported = [
        surface
        for surface in target.get("surfaces", [])
        if surface.get("capabilities", {})
        .get("agents", {})
        .get("support_level") != "unsupported"
    ]
    if not supported:
        continue
    # O CLI usa a primeira superfície suportada como default. Emitimos "target"
    # para ela e "target=surface" para as demais. Isso mantém a semântica de
    # usuário e ainda exercita superfícies não-default como antigravity=legacy-cli
    # e kiro=cli, hoje necessárias para cobrir a representação agent-json.
    default_surface = supported[0].get("id")
    for surface in supported:
        surface_id = surface.get("id")
        if not surface_id:
            continue
        if surface_id == default_surface:
            required.append(target["id"])
        else:
            required.append(f"{target['id']}={surface_id}")

for spec in sorted(set(required)):
    print(spec)
PY

  if [[ ! -s "$specs" ]]; then
    echo "Identity parity: canonical catalog has no agent-capable target/surface" >&2
    exit 1
  fi

  TARGETS=()
  while IFS= read -r spec; do
    [[ -n "$spec" ]] && TARGETS+=("$spec")
  done <"$specs"
}

assert_catalog_targets_supported_by_go_cli() {
  local home="$WORK_DIR/home-catalog-preflight"
  local project="$WORK_DIR/project-catalog-preflight"
  mkdir -p "$home" "$project"

  local spec target
  for spec in "${TARGETS[@]}"; do
    target=${spec%%=*}
    local args=(agents list --targets "$target" --scope project --json)
    if [[ "$spec" == *=* ]]; then
      args+=(--surface "$spec")
    fi
    if ! (cd "$project" && HOME="$home" "$GO_BIN" "${args[@]}") >"$WORK_DIR/catalog-preflight.log" 2>&1; then
      echo "Identity parity: catalog-derived target/surface is not accepted by the Go CLI: $spec" >&2
      cat "$WORK_DIR/catalog-preflight.log" >&2
      exit 1
    fi
  done
}

# Nem todo ambiente tem as duas ferramentas: macOS traz shasum, Linux traz
# sha256sum. Escolhido uma vez para manter o formato de saída estável.
if command -v sha256sum >/dev/null 2>&1; then
  HASH_CMD=(sha256sum)
else
  HASH_CMD=(shasum -a 256)
fi

# Diretório-raiz único e isolado para tudo que este script cria. Nada é escrito
# no $HOME real nem na raiz do repositório. Criado ANTES do trap para que uma
# falha em qualquer ponto seguinte ainda limpe.
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-identity-parity.XXXXXX")
cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

if [[ ! -x "$GO_BIN" ]]; then
  mkdir -p "$(dirname "$GO_BIN")"
  (cd "$ROOT_DIR" && GOCACHE=${GOCACHE:-/tmp/trackfw-go-cache} go build -o "$GO_BIN" ./cmd/trackfw)
fi

# ---------------------------------------------------------------------------
# 1. Fixtures de vetores de slug byte-idênticas nos 3 CLIs
# ---------------------------------------------------------------------------
CANONICAL_VECTORS="$ROOT_DIR/internal/identity/testdata/slug_vectors.json"
for mirror in "$ROOT_DIR/npm/tests/fixtures/slug_vectors.json" "$ROOT_DIR/pypi/tests/fixtures/slug_vectors.json"; do
  if [[ ! -f "$mirror" ]]; then
    echo "Identity parity: missing slug vectors fixture ${mirror#$ROOT_DIR/}" >&2
    exit 1
  fi
  if ! cmp -s "$CANONICAL_VECTORS" "$mirror"; then
    echo "Identity parity: slug vectors drift in ${mirror#$ROOT_DIR/}" >&2
    echo "Canonical source: internal/identity/testdata/slug_vectors.json" >&2
    exit 1
  fi
done

load_catalog_targets
assert_catalog_targets_supported_by_go_cli

# ---------------------------------------------------------------------------
# 2. Helpers
# ---------------------------------------------------------------------------

# run_install <cli> <home> <project-dir> <spec>
# spec é "target" ou "target=surface".
run_install() {
  local cli=$1 home=$2 project=$3 spec=$4
  local target=${spec%%=*}
  local args=(agents install --targets "$target" --scope project)
  if [[ "$spec" == *=* ]]; then
    args+=(--surface "$spec")
  fi
  mkdir -p "$project"
  case "$cli" in
    go)     (cd "$project" && HOME="$home" "$GO_BIN" "${args[@]}") ;;
    node)   (cd "$project" && HOME="$home" node "$ROOT_DIR/npm/bin/trackfw" "${args[@]}") ;;
    python) (cd "$project" && HOME="$home" PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw "${args[@]}") ;;
    *) echo "Identity parity: unknown cli '$cli'" >&2; return 1 ;;
  esac
}

# snapshot <project-dir> — lista "<sha256>  <caminho relativo>" de todos os
# artefatos instalados, em ordem estável.
#
# .trackfw/integrations-manifest.json é EXCLUÍDO de propósito: ele armazena
# caminhos absolutos do diretório temporário, logo diverge sempre e por um
# motivo que nada tem a ver com paridade de renderização.
snapshot() {
  local project=$1
  (
    cd "$project"
    find . -type f ! -path './.trackfw/*' | LC_ALL=C sort | while IFS= read -r file; do
      printf '%s  %s\n' "$("${HASH_CMD[@]}" "$file" | cut -d' ' -f1)" "$file"
    done
  )
}

# compare_target <label> <home> <spec>
# Instala o alvo pelos 3 CLIs em projetos separados e exige snapshots idênticos.
compare_target() {
  local label=$1 home=$2 target=$3
  local base="$WORK_DIR/projects/$label/${target//=/--}"
  rm -rf "$base"
  mkdir -p "$base"
  local cli
  for cli in go node python; do
    if ! run_install "$cli" "$home" "$base/$cli" "$target" >"$base-$cli.log" 2>&1; then
      echo "Identity parity [$label] target '$target': ${cli} CLI install failed" >&2
      cat "$base-$cli.log" >&2
      return 1
    fi
    snapshot "$base/$cli" >"$WORK_DIR/snap-$cli.txt"
  done

  # Guarda contra aprovação vazia: comparar dois conjuntos vazios "passa".
  local count_go count_node count_python
  count_go=$(wc -l <"$WORK_DIR/snap-go.txt" | tr -d ' ')
  count_node=$(wc -l <"$WORK_DIR/snap-node.txt" | tr -d ' ')
  count_python=$(wc -l <"$WORK_DIR/snap-python.txt" | tr -d ' ')
  if [[ "$count_go" -lt 1 ]]; then
    echo "Identity parity [$label] target '$target': go CLI produced no artifacts (vacuous check)" >&2
    return 1
  fi
  if [[ "$count_go" != "$count_node" || "$count_go" != "$count_python" ]]; then
    echo "Identity parity [$label] target '$target': artifact count mismatch (go=$count_go node=$count_node python=$count_python)" >&2
    return 1
  fi

  local diverged=()
  cmp -s "$WORK_DIR/snap-go.txt" "$WORK_DIR/snap-node.txt" || diverged+=("node")
  cmp -s "$WORK_DIR/snap-go.txt" "$WORK_DIR/snap-python.txt" || diverged+=("python")
  if [[ ${#diverged[@]} -gt 0 ]]; then
    echo "Identity parity [$label] target '$target': artifacts diverge from the Go CLI in: ${diverged[*]}" >&2
    local cli
    for cli in "${diverged[@]}"; do
      echo "--- go vs ${cli} (sha256  path) ---" >&2
      diff "$WORK_DIR/snap-go.txt" "$WORK_DIR/snap-$cli.txt" >&2 || true
      local first
      first=$(diff "$WORK_DIR/snap-go.txt" "$WORK_DIR/snap-$cli.txt" | awk '/^[<>]/ {print $3; exit}')
      if [[ -n "${first:-}" && -f "$base/go/$first" && -f "$base/$cli/$first" ]]; then
        echo "--- first diverging artifact: $first ---" >&2
        diff "$base/go/$first" "$base/$cli/$first" >&2 || true
      fi
    done
    return 1
  fi
  return 0
}

# ---------------------------------------------------------------------------
# 3. HOMEs isolados: com identidade (preset greek + apelido) e sem identidade
# ---------------------------------------------------------------------------
HOME_WITH="$WORK_DIR/home-with-identity"
HOME_WITHOUT="$WORK_DIR/home-without-identity"
mkdir -p "$HOME_WITH/.trackfw" "$HOME_WITHOUT"

# Preset "greek" completo + apelido. Inclui display names não-ASCII (Ártemis,
# Métis) de propósito: encoding divergente entre CLIs é uma fonte real de drift.
cat >"$HOME_WITH/.trackfw/identity.json" <<'JSON'
{
  "schema_version": 1,
  "user_nickname": "Kleber",
  "agents": {
    "architect": {"display_name": "Zeus", "slug": "zeus"},
    "backend": {"display_name": "Apolo", "slug": "apolo"},
    "frontend": {"display_name": "Afrodite", "slug": "afrodite"},
    "qa": {"display_name": "Ártemis", "slug": "artemis"},
    "infra": {"display_name": "Ares", "slug": "ares"},
    "security": {"display_name": "Hades", "slug": "hades"},
    "dba": {"display_name": "Poseidon", "slug": "poseidon"},
    "ux": {"display_name": "Atena", "slug": "atena"},
    "code-quality": {"display_name": "Hefesto", "slug": "hefesto"},
    "data": {"display_name": "Métis", "slug": "metis"}
  }
}
JSON

failures=0
for target in "${TARGETS[@]}"; do
  compare_target "with-identity" "$HOME_WITH" "$target" || failures=$((failures + 1))
  compare_target "no-identity" "$HOME_WITHOUT" "$target" || failures=$((failures + 1))
done

if [[ "$failures" -gt 0 ]]; then
  echo "Identity parity: ${failures} check(s) failed" >&2
  exit 1
fi

echo "Identity parity verified across Go/Node/Python for ${#TARGETS[@]} target/surface combinations (with and without identity)"
