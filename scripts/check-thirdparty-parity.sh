#!/usr/bin/env bash
# check-thirdparty-parity.sh — cross-CLI parity for the third-party artifact
# gate (ADR-2026-08-15-gate-de-duas-fases-...): Go, Node.js, Python.
#
# ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas, ML-3A.
#
# Scope, deliberately network-free (D6: this gate never fetches from the
# network — `third-party fetch` is the only subcommand that does, and its
# network policy is already covered per-stack by internal/thirdparty,
# npm/tests/thirdparty.test.js and pypi/tests/test_thirdparty.py; a genuine
# end-to-end HTTPS round trip across 3 languages would require a self-signed
# CA trusted by Go's TLS client, which root_darwin.go does not honor via
# SSL_CERT_FILE — see docs/roadmaps/.trackfw-attention.json history for this
# ML). What this script DOES cover:
#
#   A. The D3 marker-corpus test cases the roadmap calls out by name exist in
#      all 3 stacks' own test suites (a real parity gate: it fails if a stack
#      silently drops an edge case), not re-executed here — CheckMarkers is
#      already exercised, per stack, by `make quality`'s test/test-node/
#      test-python targets.
#   B. The D9 three-schema round trip: hand-author identical quarantine +
#      provenance records (this IS the real flow — D10.2 established the
#      approver writes the provenance entry directly, no command does it),
#      run `third-party install --apply-to` in all 3 CLIs, and compare
#      stdout, the installed skill file, integrations-manifest.json (D11
#      origin field, semantically — map keys are absolute per-runtime tmp
#      paths, so compared after normalizing the project prefix) and
#      thirdparty-references.json (byte-for-byte — its destination field is
#      the project-relative string, never an absolute path).
#   C. D2 branch (i) violation message byte-parity via `trackfw validate
#      --json`, after normalizing the project prefix.
#   D. D10.1 --apply-to-in-diverging-scope refusal message byte-parity.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
case "$GO_BIN" in
  /*) ;;
  *) GO_BIN="$ROOT_DIR/${GO_BIN#./}" ;;
esac
if [[ ! -x "$GO_BIN" ]]; then
  echo "check-thirdparty-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-thirdparty-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT INT TERM

export GOCACHE="$WORK/go-cache"
export npm_config_cache="$WORK/npm-cache"
export PYTHONDONTWRITEBYTECODE=1
export TRACKFW_ORCHESTRATOR_SESSION=1

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

fail=0
ok() { echo "OK: $1"; }
bad() { echo "FAIL: $1" >&2; fail=1; }

# ── PART A — marker corpus coverage: the tricky cases must exist per stack ──
# The roadmap names these specific cases (heading H1/H6, backtick fence,
# tilde fence, UNCLOSED fence, CLOSER SHORTER THAN OPENER, INDENTED fence,
# fullwidth, cyrillic-passes, HTML comment, prose, multiple spaces, fence
# followed by a real heading) as the set where a naive regex would diverge
# from the CommonMark-style line-scanner (D3 amendment). A missing case in
# any one stack is a real parity gap, not a style nit.
# Renamed/inverted by ML-4C (D3-ter): fence_unclosed and fence_short_close
# used to assert the OPPOSITE (no match — an unclosed/never-properly-closed
# fence swallowed the rest of the document); D3-ter(a) made both cases now
# CATCH the marker instead. html_comment likewise flipped (D3-ter(b):
# neutralize, not remove) and is now covered by a case-per-name in all 3
# stacks, not just Go. casefold and security_doc are new cases added by
# ML-4C (D3-ter(c) unification, and the non-regression falsification test
# named by the ML-4C AC, respectively).
declare -A corpus_grep=(
  [heading_h1]='EachMarkerRefusedAsH1|matches_literal_heading|matches a literal H1'
  [heading_h6]='EachMarkerRefusedAsH6|matches_h6_heading|heading level H6'
  [fence_backtick]='MarkerInsideFencedBlockAccepted|ignores_marker_inside_fenced_block|quoted inside a fenced code block'
  [fence_tilde]='MarkerInsideTildeFencedBlockAccepted|tilde_fence_also_removed|tilde-fenced code block'
  [fence_unclosed]='UnclosedFenceNoLongerGrantsImmunity|unclosed_fence_no_longer_grants_immunity|unclosed fence no longer grants immunity'
  [fence_short_close]='CloserShorterThanOpenerDoesNotCloseButStillCaught|closer_shorter_than_opener_does_not_close_but_still_caught|closer shorter than opener'
  [fence_indented]='IndentedFenceStillRecognized|indented_fence_still_recognized|indented fence is still recognized'
  [fence_then_heading]='HeadingAfterClosedFenceStillMatches|heading_after_closed_fence_still_matches|heading after a closed fence'
  [fullwidth]='FullwidthCompatibilityCharsRefused|full_width_heading_is_refused|refuses fullwidth'
  [cyrillic_pass]='CyrillicHomoglyphPasses|cyrillic_homoglyph_passes|cyrillic homoglyph'
  [html_comment]='HTMLCommentNeutralizedContentStillMatches|html_comment_neutralized_content_still_matches|neutralized HTML comment'
  [casefold]='CasefoldIsSimpleLowercaseNotFullCasefold|casefold_is_simple_lowercase_not_full_casefold|casefold is simple lowercase'
  [security_doc_nonregression]='SecurityOpinionDocumentDoesNotRefuseItself|security_opinion_document_does_not_refuse_itself|security opinion document does not refuse itself'
  [benign]='BenignContentAccepted'
)
declare -A corpus_files=(
  [go]="$ROOT_DIR/internal/thirdparty/markers_test.go"
  [node]="$ROOT_DIR/npm/tests/thirdparty.test.js"
  [python]="$ROOT_DIR/pypi/tests/test_thirdparty.py"
)
for case_name in "${!corpus_grep[@]}"; do
  pattern=${corpus_grep[$case_name]}
  for runtime in go node python; do
    file=${corpus_files[$runtime]}
    if [[ "$case_name" == "benign" ]] && [[ "$runtime" != "go" ]]; then
      # "benign content accepted" is exercised indirectly by every other
      # case in Node/Python (checkMarkers is pure and the benign-pass-through
      # path has no per-language branch) — only Go has a case-per-name for
      # it; not a gap.
      continue
    fi
    if ! grep -Eq "$pattern" "$file"; then
      bad "marker corpus case '$case_name' not found in $runtime ($file)"
    fi
  done
done
[[ "$fail" -eq 0 ]] && ok "marker corpus cases present in all 3 stacks"

# ── PART B — D9 three-schema round trip (network-free) ─────────────────────
# CONTENT is DELIBERATELY NOT canonical (trailing blank line after the last
# sentence) — the AC named case ("instalação legítima com conteúdo
# não-canônico não gera violação") is only genuinely exercised end-to-end if
# checksum_sha256 (raw domain) and installed_sha256 (normalized domain)
# actually DIFFER for this fixture. Canonical content would make
# INSTALLED_SHA256 == CHECKSUM, and the checks below would pass identically
# whether install correctly writes the normalized-domain hash or incorrectly
# writes the raw-domain hash (the exact bug D2-bis exists to prevent) —
# discovered during this ML's own review when an earlier canonical-content
# version of this fixture silently failed to discriminate the two.
CONTENT='# Example Third-Party Skill

Some helpful, benign content for the agent to consume.

'
CHECKSUM=$(printf '%s' "$CONTENT" | python3 -c 'import sys,hashlib; sys.stdout.write(hashlib.sha256(sys.stdin.buffer.read()).hexdigest())')
CONTENT_B64=$(printf '%s' "$CONTENT" | python3 -c 'import sys,base64; sys.stdout.write(base64.b64encode(sys.stdin.buffer.read()).decode())')

# NOT computed via an intermediate `$(...)` capture of the normalized
# bytes — bash command substitution strips trailing newlines, which would
# silently corrupt the very thing being hashed (normalize_third_party_content
# always ends in exactly one "\n"). Compute the hash directly from $CONTENT
# in a single Python process instead, asserting (not just commenting) that
# normalize_third_party_content(CONTENT) != CONTENT — i.e. that this fixture
# actually discriminates raw-domain from normalized-domain hashing, which is
# the entire point of making CONTENT non-canonical above.
INSTALLED_SHA256=$(printf '%s' "$CONTENT" | python3 -c '
import sys, hashlib
raw = sys.stdin.buffer.read()
normalized = raw.strip() + b"\n"
assert normalized != raw, "fixture CONTENT must NOT be canonical, or installed_sha256 cannot be distinguished from checksum_sha256 in this gate"
sys.stdout.write(hashlib.sha256(normalized).hexdigest())
')
if [[ "$INSTALLED_SHA256" == "$CHECKSUM" ]]; then
  bad "fixture setup: INSTALLED_SHA256 must differ from CHECKSUM (raw vs normalized), got the same value"
fi

# write_quarantine_and_provenance seeds the quarantine record (as `fetch`
# would write it) and the EXTERNAL APPROVER's half of the provenance entry
# (D10.2 — no subcommand writes provenance; a human/hades-tf writes it
# directly, keyed by checksum_sha256 the approver reviewed). Deliberately
# does NOT set installed_sha256: per ADR-2026-08-15 D2-bis, that field is
# only known — and only written — by `third-party install` itself, at
# install time, from the actual normalized bytes it writes to disk. The
# fixture mirrors that: the approver-authored entry below is schema_version
# 2 but installed_sha256-less, and each CLI's install command under test
# must add it below (verified after the install loop, Part B).
write_quarantine_and_provenance() {
  local project=$1
  mkdir -p "$project/.trackfw/thirdparty-quarantine"
  cat >"$project/.trackfw/thirdparty-quarantine/$CHECKSUM.json" <<EOF
{
  "schema_version": 1,
  "url": "https://example.com/skills/my-skill.md",
  "checksum_sha256": "$CHECKSUM",
  "fetched_at": "2026-08-15T00:00:00Z",
  "content_base64": "$CONTENT_B64",
  "marker_check": {"result": "pass", "matched_markers": []},
  "kind": "skill",
  "requested_targets": ["claude"]
}
EOF
  cat >"$project/.trackfw/thirdparty-provenance.json" <<EOF
{
  "schema_version": 2,
  "entries": {
    ".claude/skills/thirdparty/my-skill.md": {
      "url": "https://example.com/skills/my-skill.md",
      "checksum_sha256": "$CHECKSUM",
      "installed_at": "2026-08-15T00:00:00Z",
      "approved_by": "hades-tf",
      "review_reference": "docs/seguranca/example.md",
      "scope": "project",
      "marker_override": false
    }
  }
}
EOF
}

runtimes=(go node python)
for runtime in "${runtimes[@]}"; do
  project="$WORK/$runtime/project"
  home="$WORK/$runtime/home"
  mkdir -p "$project" "$home"
  # Resolve symlinks NOW (macOS: /var -> /private/var, /tmp -> /private/tmp)
  # so the $project value this script uses to build fixtures and normalize
  # output matches the PHYSICAL path each CLI's own cwd resolution reports
  # internally (Node/Python's cwd resolution is always physical; Go's is
  # physical only after an explicit EvalSymlinks, which internal/commands'
  # own project-root resolution does not perform — this is the exact
  # /tmp-vs-/private/tmp divergence already documented in
  # validator_credential_guard.go, hit here because this script is the
  # first to introspect manifest.json's absolute destination paths).
  project=$(cd "$project" && pwd -P)
  home=$(cd "$home" && pwd -P)

  run_cli "$runtime" "$project" "$home" agents install --targets claude --items backend --scope project \
    >"$WORK/$runtime-agent-install.out" 2>&1

  write_quarantine_and_provenance "$project"

  set +e
  run_cli "$runtime" "$project" "$home" skills third-party install \
    --checksum "$CHECKSUM" --targets claude --apply-to backend --yes-i-trust-this-source \
    >"$WORK/$runtime-install.out" 2>&1
  install_status=$?
  set -e
  if [[ $install_status -ne 0 ]]; then
    bad "$runtime: third-party install failed (exit $install_status):"
    cat "$WORK/$runtime-install.out" >&2
    continue
  fi

  # Normalize the per-runtime absolute project prefix out of stdout, as a
  # defensive measure — install's stdout is documented to only ever print
  # project-relative destinations, but this keeps the diff meaningful even
  # if that ever regresses instead of silently comparing two paths that
  # merely happen to differ only in tmp-dir naming.
  sed "s#$project#<PROJECT>#g" "$WORK/$runtime-install.out" >"$WORK/$runtime-install.normalized"

  cp "$project/.claude/skills/thirdparty/my-skill.md" "$WORK/$runtime-skill.md" 2>/dev/null \
    || bad "$runtime: installed skill file not found"
  cp "$project/.trackfw/integrations-manifest.json" "$WORK/$runtime-manifest.json" 2>/dev/null \
    || bad "$runtime: integrations-manifest.json not found"
  cp "$project/.trackfw/thirdparty-references.json" "$WORK/$runtime-references.json" 2>/dev/null \
    || bad "$runtime: thirdparty-references.json not found"
  cp "$project/.trackfw/thirdparty-provenance.json" "$WORK/$runtime-provenance.json" 2>/dev/null \
    || bad "$runtime: thirdparty-provenance.json not found"

  echo "$project" >"$WORK/$runtime-project-path"
done

# D2-bis (ML-3B) — installed_sha256 must be (a) present, (b) equal to
# sha256 of the NORMALIZED content actually installed (not checksum_sha256,
# which stays the raw-bytes D8c approval anchor untouched by install), and
# (c) byte-identical across the 3 CLIs. thirdparty-provenance.json has no
# absolute-path fields (its entry key is already project-relative, per the
# "Nota de paridade crítica" in docs/cli-parity.md), so a plain byte diff —
# unlike integrations-manifest.json above — is meaningful without
# normalization.
for runtime in "${runtimes[@]}"; do
  installed_sha256=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['entries']['.claude/skills/thirdparty/my-skill.md']['installed_sha256'])" "$WORK/$runtime-provenance.json" 2>/dev/null || true)
  if [[ "$installed_sha256" != "$INSTALLED_SHA256" ]]; then
    bad "$runtime: thirdparty-provenance.json installed_sha256 = '$installed_sha256', want '$INSTALLED_SHA256' (D2-bis)"
  fi
done
if diff -u "$WORK/go-provenance.json" "$WORK/node-provenance.json" >/dev/null \
  && diff -u "$WORK/go-provenance.json" "$WORK/python-provenance.json" >/dev/null; then
  ok "thirdparty-provenance.json (including installed_sha256, D2-bis) is byte-identical across the 3 CLIs"
else
  bad "thirdparty-provenance.json differs across CLIs"
  diff -u "$WORK/go-provenance.json" "$WORK/node-provenance.json" >&2 || true
  diff -u "$WORK/go-provenance.json" "$WORK/python-provenance.json" >&2 || true
fi

if diff -u "$WORK/go-install.normalized" "$WORK/node-install.normalized" >/dev/null \
  && diff -u "$WORK/go-install.normalized" "$WORK/python-install.normalized" >/dev/null; then
  ok "third-party install stdout is byte-identical across the 3 CLIs (normalized)"
else
  bad "third-party install stdout differs across CLIs"
  diff -u "$WORK/go-install.normalized" "$WORK/node-install.normalized" >&2 || true
  diff -u "$WORK/go-install.normalized" "$WORK/python-install.normalized" >&2 || true
fi

if diff -u "$WORK/go-skill.md" "$WORK/node-skill.md" >/dev/null \
  && diff -u "$WORK/go-skill.md" "$WORK/python-skill.md" >/dev/null; then
  ok "installed skill file content is byte-identical across the 3 CLIs"
else
  bad "installed skill file content differs across CLIs"
fi

if diff -u "$WORK/go-references.json" "$WORK/node-references.json" >/dev/null \
  && diff -u "$WORK/go-references.json" "$WORK/python-references.json" >/dev/null; then
  ok "thirdparty-references.json is byte-identical across the 3 CLIs (D9 schema 3)"
else
  bad "thirdparty-references.json differs across CLIs"
  diff -u "$WORK/go-references.json" "$WORK/node-references.json" >&2 || true
  diff -u "$WORK/go-references.json" "$WORK/python-references.json" >&2 || true
fi

# integrations-manifest.json has absolute per-runtime tmp paths as map keys
# and in the "destination" field — normalize each runtime's own project
# prefix to a fixed placeholder before comparing SEMANTICALLY (not
# byte-for-byte: Python's manifest writer serializes with sort_keys=True,
# a pre-existing, pre-ML-3A divergence from Go/Node's declared-field-order
# output that is out of this ML's scope to fix — see this ML's delivery
# report). Semantic comparison is exactly what
# scripts/check-integration-cli-parity.sh's compare_json already does for
# the rest of the integrations subsystem.
compare_manifest_claims() {
  python3 - "$@" <<'PY'
import json, sys
paths = sys.argv[1:4]
projects = sys.argv[4:7]
docs = []
for path, project in zip(paths, projects):
    with open(path, encoding="utf-8") as fh:
        raw = fh.read()
    raw = raw.replace(project, "<PROJECT>")
    docs.append(json.loads(raw))
first = docs[0]
for i, doc in enumerate(docs[1:], 2):
    assert doc == first, f"manifest semantic drift in document {i}:\n{json.dumps(doc, indent=2, sort_keys=True)}\nvs\n{json.dumps(first, indent=2, sort_keys=True)}"
dest = "<PROJECT>/.claude/skills/thirdparty/my-skill.md"
artifact = first["artifacts"].get(dest)
assert artifact is not None, f"expected artifact at {dest}, got keys {list(first['artifacts'])}"
claims = artifact["claims"]
assert any(c.get("origin") == "thirdparty" for c in claims), claims
print("manifest claim semantics match (origin=thirdparty present, D11)")
PY
}
if compare_manifest_claims \
  "$WORK/go-manifest.json" "$WORK/node-manifest.json" "$WORK/python-manifest.json" \
  "$(cat "$WORK/go-project-path")" "$(cat "$WORK/node-project-path")" "$(cat "$WORK/python-project-path")"; then
  ok "integrations-manifest.json claim (origin=thirdparty) is semantically identical across the 3 CLIs (D11)"
else
  bad "integrations-manifest.json claim semantics differ across CLIs"
fi

# D2-bis end-to-end, before Part C strips provenance below: a genuinely
# legitimate install of the NON-CANONICAL $CONTENT fixture (see the comment
# above CONTENT's definition) must produce ZERO
# thirdparty_artifact_has_provenance violations in all 3 CLIs. This is the
# actual AC ("instalação legítima com conteúdo não-canônico não é
# falso-positivo") exercised end-to-end through the real `install` command
# in all 3 stacks, not just through the validator unit tests (which
# hand-author the provenance entry and so only exercise the reader half of
# D2-bis, never the writer).
for runtime in "${runtimes[@]}"; do
  project=$(cd "$WORK/$runtime/project" && pwd -P)
  set +e
  run_cli "$runtime" "$project" "$WORK/$runtime/home" validate --json >"$WORK/$runtime-clean-validate.json" 2>"$WORK/$runtime-clean-validate.stderr"
  set -e
  violation_count=$(python3 -c "
import json, sys
with open(sys.argv[1], encoding='utf-8') as fh:
    payload = json.load(fh)
print(sum(1 for v in payload.get('violations', []) if v.get('rule') == 'thirdparty_artifact_has_provenance'))
" "$WORK/$runtime-clean-validate.json")
  if [[ "$violation_count" != "0" ]]; then
    bad "$runtime: legitimate non-canonical install produced $violation_count thirdparty_artifact_has_provenance violation(s), want 0 (D2-bis end-to-end)"
    cat "$WORK/$runtime-clean-validate.json" >&2
  fi
done
[[ "$fail" -eq 0 ]] && ok "D2-bis: legitimate install of non-canonical content produces zero validate violations end-to-end, all 3 CLIs"

# ── PART C — D2 branch (i) violation message byte-parity ───────────────────
# Reuse the 3 projects above but strip the provenance entry, so the manifest
# still carries the origin=thirdparty claim (from Part B's install) with no
# matching approval — the exact branch (i) trigger.
for runtime in "${runtimes[@]}"; do
  # Must be the SAME resolved physical path used in Part B (pwd -P) — the
  # violation message embeds the CLI's own resolved cwd, and Part B already
  # proved the raw (non-realpath'd) $project value diverges from it on
  # macOS ("/var/..." vs "/private/var/..."); reusing the unresolved value
  # here would make the normalization below a no-op and the byte-parity
  # check meaningless.
  project=$(cd "$WORK/$runtime/project" && pwd -P)
  rm -f "$project/.trackfw/thirdparty-provenance.json"
  set +e
  # stdout only — `validate` prints its JSON to stdout but Execute() in
  # internal/commands/root.go writes the plain-text error summary
  # ("N violation(s) found") to STDERR regardless of --json/SilenceErrors,
  # so stderr must not be merged into the file this script parses as JSON.
  run_cli "$runtime" "$project" "$WORK/$runtime/home" validate --json >"$WORK/$runtime-validate.json" 2>"$WORK/$runtime-validate.stderr"
  set -e
  python3 - "$WORK/$runtime-validate.json" "$project" "$WORK/$runtime-branch-i.msg" <<'PY'
import json, sys
path, project, out_path = sys.argv[1:4]
with open(path, encoding="utf-8") as fh:
    payload = json.load(fh)
msgs = [
    item["message"].replace(project, "<PROJECT>")
    for item in payload.get("violations", [])
    if item.get("rule") == "thirdparty_artifact_has_provenance"
]
with open(out_path, "w", encoding="utf-8") as fh:
    fh.write("\n".join(sorted(msgs)) + "\n")
PY
done
if [[ -s "$WORK/go-branch-i.msg" ]] \
  && diff -u "$WORK/go-branch-i.msg" "$WORK/node-branch-i.msg" >/dev/null \
  && diff -u "$WORK/go-branch-i.msg" "$WORK/python-branch-i.msg" >/dev/null; then
  ok "D2 branch (i) violation message is byte-identical across the 3 CLIs (normalized)"
else
  bad "D2 branch (i) violation message differs across CLIs, or is empty"
  cat "$WORK/go-branch-i.msg" >&2 || true
  diff -u "$WORK/go-branch-i.msg" "$WORK/node-branch-i.msg" >&2 || true
  diff -u "$WORK/go-branch-i.msg" "$WORK/python-branch-i.msg" >&2 || true
fi

# ── PART D — D10.1 --apply-to scope-mismatch refusal message parity ────────
# Fresh projects, no agent installed at all: --apply-to must refuse with the
# exact remediation command, before any write.
for runtime in "${runtimes[@]}"; do
  project="$WORK/d10/$runtime/project"
  home="$WORK/d10/$runtime/home"
  mkdir -p "$project" "$home"
  project=$(cd "$project" && pwd -P)
  home=$(cd "$home" && pwd -P)
  write_quarantine_and_provenance "$project"
  set +e
  run_cli "$runtime" "$project" "$home" skills third-party install \
    --checksum "$CHECKSUM" --targets claude --apply-to backend --yes-i-trust-this-source \
    >"$WORK/$runtime-d10.out" 2>&1
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    bad "$runtime: expected D10.1 refusal (agent not installed) but install succeeded"
  fi
  # Extract ONLY the "cannot attach reference: ..." remediation line, not
  # the full raw output. The 3 CLIs wrap this identical message differently
  # at the process level: Go's cobra prints it plus a Usage/Flags block;
  # Node.js and Python let the underlying Error/IntegrationError propagate
  # UNCAUGHT out of the CLI entrypoint (npm/bin/trackfw has no top-level
  # try/catch; pypi/trackfw/cli.py likewise), which is why their output is
  # a raw stack trace ending in "Node.js vNN.N.N" / a Python traceback
  # instead of a clean one-line error. That crash-vs-clean-error divergence
  # is REAL and worth fixing, but it is a project-wide characteristic of
  # every Node/Python command (reproduced here with a plain `agents install
  # --targets nonexistent`, unrelated to third-party) — out of this ML's
  # boundary, which only implements the NEW thirdparty_artifact_has_provenance
  # rule and its own commands' messages, not the CLIs' general top-level
  # error handling. See this ML's delivery report. What this gate DOES
  # assert, matching AC's literal wording ("a mensagem traga o comando
  # exato de remediação... idêntica nos 3 CLIs"), is that the remediation
  # message text itself — the part D10.1 actually specifies — is identical.
  grep -o 'cannot attach reference:.*$' "$WORK/$runtime-d10.out" | head -1 \
    | sed "s#$project#<PROJECT>#g" >"$WORK/$runtime-d10.normalized"
done
if [[ -s "$WORK/go-d10.normalized" ]] \
  && diff -u "$WORK/go-d10.normalized" "$WORK/node-d10.normalized" >/dev/null \
  && diff -u "$WORK/go-d10.normalized" "$WORK/python-d10.normalized" >/dev/null; then
  ok "D10.1 --apply-to scope-mismatch remediation message is byte-identical across the 3 CLIs"
else
  bad "D10.1 remediation message differs across CLIs, or is empty"
  diff -u "$WORK/go-d10.normalized" "$WORK/node-d10.normalized" >&2 || true
  diff -u "$WORK/go-d10.normalized" "$WORK/python-d10.normalized" >&2 || true
fi

if [[ "$fail" -ne 0 ]]; then
  echo "check-thirdparty-parity: FAILED" >&2
  exit 1
fi
echo "Third-party artifact gate parity checks passed (D9 schemas, D2 branch i, D10.1, D3 corpus coverage)"
