#!/usr/bin/env bash
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
case "$GO_BIN" in
  /*) ;;
  *) GO_BIN="$ROOT_DIR/${GO_BIN#./}" ;;
esac
if [[ ! -x "$GO_BIN" ]]; then
  echo "Go trackfw binary is not executable: $GO_BIN" >&2
  exit 1
fi

# Derive expected item counts from the canonical catalog — never hardcode.
CATALOG_FILE="$ROOT_DIR/internal/integrations/assets/catalog.json"
if [[ ! -f "$CATALOG_FILE" ]]; then
  echo "ERROR: catalog not found: $CATALOG_FILE — cannot derive expected item counts; aborting." >&2
  exit 1
fi
_catalog_counts=$(python3 - "$CATALOG_FILE" <<'PY'
import json, sys
try:
    with open(sys.argv[1], encoding="utf-8") as f:
        cat = json.load(f)
    print(len(cat["agents"]), len(cat["skills"]))
except Exception as exc:
    print(f"catalog parse error: {exc}", file=sys.stderr)
    sys.exit(1)
PY
) || { echo "ERROR: failed to parse catalog at $CATALOG_FILE" >&2; exit 1; }
read -r EXPECTED_AGENTS_COUNT EXPECTED_SKILLS_COUNT <<<"$_catalog_counts"
export CATALOG_FILE EXPECTED_AGENTS_COUNT EXPECTED_SKILLS_COUNT

TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-integration-parity.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT INT TERM

export GOCACHE="$TMP_ROOT/go-cache"
export npm_config_cache="$TMP_ROOT/npm-cache"
export PYTHONDONTWRITEBYTECODE=1

run_cli() {
  local runtime=$1 project=$2 home=$3
  shift 3
  case "$runtime" in
    go)     (cd "$project" && HOME="$home" "$GO_BIN" "$@") ;;
    node)   (cd "$project" && HOME="$home" node "$ROOT_DIR/npm/bin/trackfw" "$@") ;;
    python) (cd "$project" && HOME="$home" PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw "$@") ;;
    *) echo "unknown runtime: $runtime" >&2; return 2 ;;
  esac
}

assert_help_contract() {
  local runtime=$1 project=$2 home=$3 kind action root_output stripped_root kind_output stripped_kind
  root_output=$(run_cli "$runtime" "$project" "$home" --help)
  # Strip ANSI escapes before grep — Python 3.13+ colourises argparse help and
  # NO_COLOR may not propagate into nested invocations through run_cli.
  stripped_root=$(printf '%s' "$root_output" | sed 's/\x1b\[[0-9;]*m//g')
  for kind in agents skills; do
    grep -Eq "(^|[[:space:]])${kind}([[:space:]]|$)" <<<"$stripped_root" || {
      echo "$runtime: root help missing $kind" >&2; return 1;
    }
    kind_output=$(run_cli "$runtime" "$project" "$home" "$kind" --help)
    stripped_kind=$(printf '%s' "$kind_output" | sed 's/\x1b\[[0-9;]*m//g')
    for action in list install uninstall update; do
      grep -Eq "(^|[[:space:]])${action}([[:space:]]|$)" <<<"$stripped_kind" || {
        echo "$runtime: $kind help missing $action" >&2; return 1;
      }
    done
  done
}

assert_json() {
  local filename=$1 expected_kind=$2 expected_state=${3:-} expected_target=${4:-} expected_surface=${5:-}
  python3 - "$filename" "$expected_kind" "$expected_state" "$expected_target" "$expected_surface" <<'PY'
import json, os, sys
filename, kind, state, target, surface = sys.argv[1:]
with open(filename, encoding="utf-8") as stream:
    payload = json.load(stream)
assert set(payload) == {"kind", "catalog_version", "items", "deployments"}, payload.keys()
assert payload["kind"] == kind
assert payload["catalog_version"]
_expected = int(os.environ["EXPECTED_AGENTS_COUNT"]) if kind == "agents" else int(os.environ["EXPECTED_SKILLS_COUNT"])
_actual = len(payload["items"])
assert _actual == _expected, f"item count mismatch for {kind}: expected {_expected}, got {_actual}"
assert all(set(item) == {"id", "name", "description"} for item in payload["items"])
required = {"target", "surface", "scope", "item", "support_level", "representation", "destination", "state", "managed"}
assert all(set(row) == required for row in payload["deployments"])
if state:
    assert len(payload["deployments"]) == 1, payload["deployments"]
    row = payload["deployments"][0]
    assert row["state"] == state, row
    assert row["target"] == target, row
    assert row["surface"] == surface, row
    assert row["managed"] is (state != "not-installed"), row
PY
}

assert_catalog_targets() {
  local filename=$1
  python3 - "$filename" <<'PY'
# P1: derive the expected target set from the catalog (CATALOG_FILE env var) so
# the gate stays accurate automatically when new targets are added, without
# requiring a manual edit to a hardcoded constant in this script.
import json, sys, os
catalog_path = os.environ.get("CATALOG_FILE", "")
if not catalog_path:
    print("assert_catalog_targets: CATALOG_FILE env var not set", file=sys.stderr)
    sys.exit(1)
try:
    with open(catalog_path, encoding="utf-8") as f:
        cat = json.load(f)
    expected = {t["id"] for t in cat["targets"]}
except Exception as exc:
    print(f"assert_catalog_targets: failed to read catalog: {exc}", file=sys.stderr)
    sys.exit(1)
with open(sys.argv[1], encoding="utf-8") as stream:
    payload = json.load(stream)
actual = {row["target"] for row in payload["deployments"]}
assert actual == expected, (
    f"target set mismatch\n  got:      {sorted(actual)}\n  expected: {sorted(expected)}"
)
rows = [(row["target"], row["surface"], row["item"]) for row in payload["deployments"]]
assert rows == sorted(rows), rows
PY
}

compare_json() {
  python3 - "$@" <<'PY'
import json, sys
documents = []
for filename in sys.argv[1:]:
    with open(filename, encoding="utf-8") as stream:
        documents.append(json.load(stream))
first = documents[0]
for index, document in enumerate(documents[1:], 2):
    assert document == first, f"JSON semantic drift in document {index}"
PY
}

runtimes=(go node python)
for runtime in "${runtimes[@]}"; do
  project="$TMP_ROOT/$runtime/project"
  home="$TMP_ROOT/$runtime/home"
  mkdir -p "$project" "$home"
  assert_help_contract "$runtime" "$project" "$home"

  # Unfiltered list proves the complete target matrix and deterministic JSON.
  run_cli "$runtime" "$project" "$home" agents list --items backend --scope project --json >"$TMP_ROOT/$runtime-catalog.json"
  assert_json "$TMP_ROOT/$runtime-catalog.json" agents
  assert_catalog_targets "$TMP_ROOT/$runtime-catalog.json"

  # Agent lifecycle: explicit legacy surface in project scope.
  common_agent=(--targets antigravity --items backend --scope project --surface antigravity=legacy-cli --json)
  run_cli "$runtime" "$project" "$home" agents install "${common_agent[@]}" >"$TMP_ROOT/$runtime-agent-install.json"
  assert_json "$TMP_ROOT/$runtime-agent-install.json" agents current antigravity legacy-cli
  run_cli "$runtime" "$project" "$home" agents list "${common_agent[@]}" >"$TMP_ROOT/$runtime-agent-list.json"
  assert_json "$TMP_ROOT/$runtime-agent-list.json" agents current antigravity legacy-cli
  run_cli "$runtime" "$project" "$home" agents update "${common_agent[@]}" >"$TMP_ROOT/$runtime-agent-update.json"
  assert_json "$TMP_ROOT/$runtime-agent-update.json" agents current antigravity legacy-cli
  run_cli "$runtime" "$project" "$home" agents uninstall "${common_agent[@]}" >"$TMP_ROOT/$runtime-agent-uninstall.json"
  assert_json "$TMP_ROOT/$runtime-agent-uninstall.json" agents not-installed antigravity legacy-cli

  # Skill lifecycle: global scope with HOME redirected to this runtime's tmp.
  common_skill=(--targets claude --items governance --scope global --json)
  run_cli "$runtime" "$project" "$home" skills install "${common_skill[@]}" >"$TMP_ROOT/$runtime-skill-install.json"
  assert_json "$TMP_ROOT/$runtime-skill-install.json" skills current claude cli
  run_cli "$runtime" "$project" "$home" skills list "${common_skill[@]}" >"$TMP_ROOT/$runtime-skill-list.json"
  assert_json "$TMP_ROOT/$runtime-skill-list.json" skills current claude cli
  run_cli "$runtime" "$project" "$home" skills update "${common_skill[@]}" >"$TMP_ROOT/$runtime-skill-update.json"
  assert_json "$TMP_ROOT/$runtime-skill-update.json" skills current claude cli
  run_cli "$runtime" "$project" "$home" skills uninstall "${common_skill[@]}" >"$TMP_ROOT/$runtime-skill-uninstall.json"
  assert_json "$TMP_ROOT/$runtime-skill-uninstall.json" skills not-installed claude cli
done

# Compare every canonical response semantically rather than relying on spacing.
for suffix in catalog agent-install agent-list agent-update agent-uninstall skill-install skill-list skill-update skill-uninstall; do
  compare_json "$TMP_ROOT/go-$suffix.json" "$TMP_ROOT/node-$suffix.json" "$TMP_ROOT/python-$suffix.json"
done

echo "Integration CLI parity lifecycle checks passed"
