#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

mkdir -p \
  "$TMP_DIR/project/docs/adr" \
  "$TMP_DIR/project/docs/req" \
  "$TMP_DIR/project/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}

cat >"$TMP_DIR/project/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

cat >"$TMP_DIR/project/docs/roadmaps/wip/RM.md" <<'EOF'
---
status: WIP
---
# Roadmap without required governance links
EOF

GOCACHE=${GOCACHE:-/tmp/trackfw-go-cache} go build -o "$TMP_DIR/trackfw-go" ./cmd/trackfw

run_validator() {
  local output=$1
  shift
  set +e
  (
    cd "$TMP_DIR/project"
    "$@"
  ) >"$output" 2>"$output.stderr"
  local status=$?
  set -e
  if [[ $status -ne 1 ]]; then
    echo "Expected validation exit code 1, got $status for $*" >&2
    return 1
  fi
}

run_validator "$TMP_DIR/go.json" "$TMP_DIR/trackfw-go" validate --json
run_validator "$TMP_DIR/node.json" node "$ROOT_DIR/npm/bin/trackfw" validate --json
run_validator "$TMP_DIR/python.json" env PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw validate --json

python3 - "$TMP_DIR/go.json" "$TMP_DIR/node.json" "$TMP_DIR/python.json" <<'PY'
import json
import sys

def contract(path):
    with open(path, encoding="utf-8") as stream:
        payload = json.load(stream)
    return {
        "summary": payload["summary"],
        "violations": sorted(
            (item.get("rule"), item.get("file")) for item in payload["violations"]
        ),
        "warnings": sorted(
            (item.get("rule"), item.get("file")) for item in payload["warnings"]
        ),
    }

contracts = [contract(path) for path in sys.argv[1:]]
if contracts[1:] != contracts[:-1]:
    for path, value in zip(sys.argv[1:], contracts):
        print(path, json.dumps(value, indent=2), file=sys.stderr)
    raise SystemExit("validate JSON contract differs between runtimes")
PY

echo "Validate JSON parity checks passed"
