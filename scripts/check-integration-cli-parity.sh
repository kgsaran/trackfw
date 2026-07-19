#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
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
  local runtime=$1 project=$2 home=$3 kind action root_output kind_output
  root_output=$(run_cli "$runtime" "$project" "$home" --help)
  for kind in agents skills; do
    grep -Eq "(^|[[:space:]])${kind}([[:space:]]|$)" <<<"$root_output" || {
      echo "$runtime: root help missing $kind" >&2; return 1;
    }
    kind_output=$(run_cli "$runtime" "$project" "$home" "$kind" --help)
    for action in list install uninstall update; do
      grep -Eq "(^|[[:space:]])${action}([[:space:]]|$)" <<<"$kind_output" || {
        echo "$runtime: $kind help missing $action" >&2; return 1;
      }
    done
  done
}

assert_json() {
  local filename=$1 expected_kind=$2 expected_state=${3:-} expected_target=${4:-} expected_surface=${5:-}
  python3 - "$filename" "$expected_kind" "$expected_state" "$expected_target" "$expected_surface" <<'PY'
import json, sys
filename, kind, state, target, surface = sys.argv[1:]
with open(filename, encoding="utf-8") as stream:
    payload = json.load(stream)
assert set(payload) == {"kind", "catalog_version", "items", "deployments"}, payload.keys()
assert payload["kind"] == kind
assert payload["catalog_version"]
assert len(payload["items"]) == (10 if kind == "agents" else 5)
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
import json, sys
expected = {"claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "kiro"}
with open(sys.argv[1], encoding="utf-8") as stream:
    payload = json.load(stream)
actual = {row["target"] for row in payload["deployments"]}
assert actual == expected, (actual, expected)
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
