#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
ML-2A ratchet + ML-2B removal-note enforcement.

ADR: docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-
     tipo-de-evento-nunca-por-contagem.md

D1  — fails if a test name appears that is NOT in the known list (new failure).
D2  — vacuity guard: list empty or missing -> exit 1, naming the cause.
D4  — removal_note enforcement (ML-2B):
        - entries removed from 'entries' must appear in the 'removed' section
          with a 'removal_note' field (baseline diff via --baseline).
        - removed entries without 'removal_note' -> exit 1.
        - removal_note='corrected': test must appear in PASS output of the
          corresponding runtime (Go: '--- PASS:', Node: 'ok N -', Python: 'PASSED').
        - removal_note='renamed': 'renamed_to' field must be present and the new
          name must exist in the active entries for the same runtime.
        - removal_note='no-longer-runs': no further verification (cannot distinguish
          from 'corrected' definitively without collection data).
Warn — known entry not observed (fixed/renamed) -> ::warning::, never exit 1.

Discriminant measurement (ML-2B, 2026-09-10, macOS arm64):
  Go   (-v)     : '--- PASS: TestFoo (0.01s)' appears for passing tests.
  Node (TAP)    : 'ok N - test name' at column 0 (vs 'not ok N - test name').
  Python (-q -rA): 'PASSED pypi/tests/...' in short summary section.
  All three runtimes have a discriminant (corrected vs no-longer-runs).
  Python requires '-rA' flag (not present in -q alone); if pass set is vacuous
  (no PASSED lines found), checker warns and skips 'corrected' verification.

Artifact guard (observation-side vacuity): missing output artifact -> exit 1.
Without it, empty observation sets produce ~38 spurious removal warnings and
exit 0 — the exact D3 failure mode (suite did not run) displaced onto the
observation side.

Usage:
  # Self-test (run by `make quality` and in CI on non-windows jobs):
  check-windows-known-failures.py --self-test

  # Normal check (run by the verifier step in windows-full-suites):
  check-windows-known-failures.py \\
      --list .github/windows-known-failures.json \\
      --go-out  <path/to/go-suite-out.txt> \\
      --node-tap <path/to/node-suite.tap> \\
      --python-out <path/to/python-suite-out.txt> \\
      [--baseline <path/to/baseline-known-failures.json>]

Exit codes:
  0 — no new failures detected (warnings may have been emitted)
  1 — new failure(s) detected, or vacuity/artifact guard triggered,
      or removal_note violation found
"""

from __future__ import annotations

import argparse
import contextlib
import io
import json
import os
import re
import sys
import tempfile
from pathlib import Path


# ---------------------------------------------------------------------------
# GitHub Actions annotation helpers
# ---------------------------------------------------------------------------

# _ANNOTATION_SINK is where _err() and _warn() write ::error:: / ::warning::
# tokens.  Default: sys.stdout — the GitHub Actions runner reads every line
# on stdout and turns ::error:: into a check-run annotation (#319).
#
# During run_self_test(), each fixture call that exercises an error path uses
# _capture_annotations() to swap the sink for an in-process io.StringIO.
# The caller then asserts on ann.getvalue() to prove the error arm was reached
# (AC5) without leaking any ::error:: to the real stdout.
#
# Production mode never touches this variable — _err()/_warn() keep writing to
# sys.stdout so real ratchet failures still create runner annotations (AC4).
_ANNOTATION_SINK = sys.stdout


@contextlib.contextmanager
def _capture_annotations():
    """Redirect _err/_warn to a buffer for one self-test fixture call.

    Usage::

        with _capture_annotations() as ann:
            rc = run_check(...)
        assert "::error::" in ann.getvalue()   # AC5: error arm was reached

    Restores the previous sink on exit; safe for nesting.
    """
    global _ANNOTATION_SINK
    buf = io.StringIO()
    old = _ANNOTATION_SINK
    _ANNOTATION_SINK = buf
    try:
        yield buf
    finally:
        _ANNOTATION_SINK = old


def _st_print(msg: str, *, stream=None) -> None:
    """Write a self-test diagnostic line safely on any console encoding.

    Replaces characters not representable in the console charset with '?'
    instead of raising UnicodeEncodeError — e.g. U+2192 (→) in cp1252 (#314).

    Survives contextlib.redirect_stdout(io.StringIO()): StringIO has no
    .encoding attribute, so the fallback is 'utf-8' (encode/decode is a no-op).
    """
    s = stream or sys.stdout
    enc = getattr(s, "encoding", None) or "utf-8"
    print(msg.encode(enc, errors="replace").decode(enc, errors="replace"), file=s, flush=True)


def _err(msg: str) -> None:
    print(f"::error::{msg}", file=_ANNOTATION_SINK, flush=True)


def _warn(msg: str) -> None:
    print(f"::warning::{msg}", file=_ANNOTATION_SINK, flush=True)


# ---------------------------------------------------------------------------
# List loader (vacuity guard inside)
# ---------------------------------------------------------------------------

def load_known_data(path: str) -> dict:
    """Load the full known-failures JSON. Returns the raw dict.

    Raises SystemExit(1) on vacuity (missing file or zero active entries).
    A missing or empty active list is not 'no known failures' — it is a broken
    ratchet that passes silently over active debt.
    """
    p = Path(path)
    if not p.exists():
        _err(
            f"ML-2A vacuity guard: known-failures list not found at '{path}'. "
            "A missing list with active debt lets every failure pass silently. "
            "Create the list from a CI run (ADR D2) before enabling the ratchet."
        )
        raise SystemExit(1)
    with p.open(encoding="utf-8") as f:
        data = json.load(f)
    entries = data.get("entries", [])
    if not entries:
        _err(
            f"ML-2A vacuity guard: known-failures list at '{path}' has zero active entries. "
            "An empty list passes every observation. "
            "With active Windows debt this is the worst failure mode."
        )
        raise SystemExit(1)
    return data


def load_known_list(path: str) -> list[dict]:
    """Backwards-compatible: return active entries only."""
    return load_known_data(path).get("entries", [])


# ---------------------------------------------------------------------------
# Artifact guard
# ---------------------------------------------------------------------------

def require_artifact(path: str, label: str) -> Path:
    """Return Path if it exists; emit ::error:: and raise SystemExit(1) otherwise.

    If a suite step crashed before writing, the observation set is empty.
    An empty observation set causes every known entry to warn 'please remove'
    and the verifier exits 0 — the observation-side analogue of ADR D3 vacuity.
    """
    p = Path(path)
    if not p.exists():
        _err(
            f"ML-2A artifact guard: '{label}' not found at '{path}'. "
            "An empty observation set would make every known entry warn 'please remove' "
            "and exit 0 — the same vacuity failure mode as an empty list, displaced onto "
            "the observation side. The suite step may have crashed before writing the file."
        )
        raise SystemExit(1)
    return p


# ---------------------------------------------------------------------------
# Failure extractors
# ---------------------------------------------------------------------------

def extract_go_failures(go_out_path: Path) -> set[str]:
    """Extract top-level Go test failure names.

    D5 decision: strip subtest path (everything after first '/').
    Subtest names are table entries and may change when test parameters change.

    Pattern: '--- FAIL: TestName' anywhere in the line (the log written by
    go test -v always has '--- FAIL: TestName (duration)' without leading spaces
    for top-level tests; subtests have 4-space indent, but since we match
    *anywhere* in the line and strip after '/', both resolve to the top-level name).
    """
    pattern = re.compile(r'--- FAIL: (Test\w+(?:/\S*)?)')
    failures: set[str] = set()
    with go_out_path.open(encoding="utf-8", errors="replace") as f:
        for line in f:
            m = pattern.search(line)
            if m:
                raw = m.group(1)
                top_level = raw.split('/')[0]
                failures.add(top_level)
    return failures


def _classify_node_block(
    name: str,
    has_exit_code: bool,
    assertions: set[str],
    suite_load_failures: set[str],
) -> None:
    """Classify a single TAP 'not ok' block into assertion or suite-load-failure.

    D3-bis (ADR): presence of 'exitCode:' in the YAML block of a 'not ok' entry
    means the file process died before running tests -> suite-load-failure.
    Absence means a test assertion failed inside a running suite.

    D5 decision for suite-load-failure: use file basename only.
    The full runner path (D:\\a\\trackfw\\trackfw\\...) is unstable across runs.
    """
    if has_exit_code:
        # suite-load-failure: subject is the file path — use basename
        basename = os.path.basename(name.replace('\\', '/'))
        suite_load_failures.add(basename)
    else:
        assertions.add(name)


def extract_node_failures(tap_path: Path) -> tuple[set[str], set[str]]:
    """Extract Node.js assertion failures and suite-load-failures from TAP output.

    Returns (assertion_names, suite_load_failure_basenames).

    The TAP file is written directly by Node's --test-reporter-destination, so
    it has no GitHub Actions line prefix. Top-level 'not ok' entries are at
    column 0; YAML block fields are indented with 2 spaces.
    """
    assertions: set[str] = set()
    suite_load_failures: set[str] = set()

    not_ok_re = re.compile(r'^not ok \d+ - (.+)$')
    exit_code_re = re.compile(r'^\s+exitCode:\s+\d+')

    # Read with explicit UTF-8 to preserve non-ASCII test names (em-dashes, accents)
    content = tap_path.read_text(encoding="utf-8", errors="replace")

    current_name: str | None = None
    has_exit_code = False

    for raw_line in content.splitlines():
        line = raw_line.rstrip('\r')  # handle CRLF from Windows runner

        m = not_ok_re.match(line)
        if m:
            # Flush previous block before opening new one
            if current_name is not None:
                _classify_node_block(
                    current_name, has_exit_code, assertions, suite_load_failures
                )
            current_name = m.group(1).strip()
            has_exit_code = False
        elif current_name is not None and exit_code_re.match(line):
            has_exit_code = True

    # Flush last block
    if current_name is not None:
        _classify_node_block(
            current_name, has_exit_code, assertions, suite_load_failures
        )

    return assertions, suite_load_failures


def extract_python_failures(python_out_path: Path) -> set[str]:
    """Extract Python test failure IDs.

    D5 decision: full pytest node ID relative to pypi/tests/ including class
    qualifier (e.g. 'test_foo.py::TestClass::test_method'). Function names alone
    are not unique across classes.

    Normalization: strip 'pypi[/\\]tests[/\\]' prefix, replace backslashes with
    forward slashes. This handles both POSIX ('pypi/tests/...') and Windows
    ('pypi\\tests\\...') paths in the pytest output.
    """
    # Pattern: 'FAILED pypi/tests/test_foo.py::...' or 'FAILED pypi\tests\test_foo.py::...'
    pattern = re.compile(
        r'FAILED\s+(pypi[/\\]tests[/\\][^\s]+?)(?:\s+-\s+|$)'
    )
    prefix_re = re.compile(r'^pypi[/\\]tests[/\\]')

    failures: set[str] = set()
    with python_out_path.open(encoding="utf-8", errors="replace") as f:
        for line in f:
            m = pattern.search(line)
            if m:
                raw_id = m.group(1).strip()
                # Normalize path separators
                normalized = raw_id.replace('\\', '/')
                # Strip 'pypi/tests/' prefix to get the relative node ID
                normalized = prefix_re.sub('', normalized)
                failures.add(normalized)
    return failures


# ---------------------------------------------------------------------------
# Pass extractors (ML-2B: discriminant for corrected vs no-longer-runs)
# ---------------------------------------------------------------------------

def extract_go_passes(go_out_path: Path) -> tuple[set[str], bool]:
    """Extract top-level Go test pass names from -v output.

    Returns (passes, is_vacuous).

    is_vacuous: True if the artifact has zero test result lines (neither PASS
    nor FAIL), which means the suite did not produce output — skip 'corrected'
    verification in that case to avoid false positives.

    Measured (2026-09-10, macOS arm64, go test -v):
        passing test  → '--- PASS: TestFoo (0.01s)' present
        deleted test  → neither '--- PASS: TestFoo' nor '--- FAIL: TestFoo'
    D5: strip subtests (same as failure extraction).
    """
    pass_pattern = re.compile(r'--- PASS: (Test\w+(?:/\S*)?)')
    fail_pattern = re.compile(r'--- FAIL:')
    passes: set[str] = set()
    has_any_result = False

    with go_out_path.open(encoding="utf-8", errors="replace") as f:
        for line in f:
            mp = pass_pattern.search(line)
            if mp:
                raw = mp.group(1)
                top_level = raw.split('/')[0]
                passes.add(top_level)
                has_any_result = True
            elif fail_pattern.search(line):
                has_any_result = True

    return passes, not has_any_result


def extract_node_passes(tap_path: Path) -> tuple[set[str], bool]:
    """Extract Node.js passing test names from TAP output.

    Returns (passes, is_vacuous).

    TAP format (measured 2026-09-10, Node 26.8.1/macOS arm64):
        passing test  → 'ok N - test name' at column 0
        failing test  → 'not ok N - test name' at column 0
        deleted test  → neither line appears

    is_vacuous: True if no 'ok' or 'not ok' lines found at column 0.
    """
    ok_re = re.compile(r'^ok \d+ - (.+)$')
    not_ok_re = re.compile(r'^not ok \d+')
    passes: set[str] = set()
    has_any_result = False

    content = tap_path.read_text(encoding="utf-8", errors="replace")

    for raw_line in content.splitlines():
        line = raw_line.rstrip('\r')
        if not_ok_re.match(line):
            has_any_result = True
        else:
            m = ok_re.match(line)
            if m:
                passes.add(m.group(1).strip())
                has_any_result = True

    return passes, not has_any_result


def extract_python_passes(python_out_path: Path) -> tuple[set[str], bool]:
    """Extract Python test pass IDs from pytest -q -rA output.

    Returns (passes, is_vacuous).

    pytest -rA adds 'PASSED pypi/tests/...' to the short summary section.
    Measured (2026-09-10, Python 3.14.7/macOS arm64):
        'PASSED pypi/tests/test_foo.py::TestClass::method'

    Without -rA (or with old pytest), no PASSED lines appear.
    is_vacuous: True if neither PASSED nor FAILED lines found; in that case
    the checker warns and skips 'corrected' verification for Python.

    Normalization: same as extract_python_failures (strip prefix, normalize slashes).
    """
    passed_pattern = re.compile(
        r'PASSED\s+(pypi[/\\]tests[/\\][^\s]+?)(?:\s+-\s+|$)'
    )
    failed_pattern = re.compile(r'FAILED\s+pypi[/\\]tests[/\\]')
    prefix_re = re.compile(r'^pypi[/\\]tests[/\\]')

    passes: set[str] = set()
    has_any_result = False

    with python_out_path.open(encoding="utf-8", errors="replace") as f:
        for line in f:
            mp = passed_pattern.search(line)
            if mp:
                raw_id = mp.group(1).strip()
                normalized = raw_id.replace('\\', '/')
                normalized = prefix_re.sub('', normalized)
                passes.add(normalized)
                has_any_result = True
            elif failed_pattern.search(line):
                has_any_result = True

    return passes, not has_any_result


# ---------------------------------------------------------------------------
# ML-3A: Go suite-load-failure name parser
# ---------------------------------------------------------------------------

def _parse_go_load_names(content: str) -> set[str]:
    """Extract package paths from Go suite-load-failure marker content.

    Workflow writes one line per failing package:
        'FAIL\\tgithub.com/kgsaran/trackfw/internal/badpkg [setup failed]'
    (Measured: T16 fixture format; run 34478752778 pattern.)

    Returns the stable package path (e.g. 'github.com/kgsaran/trackfw/internal/badpkg').
    Returns empty set if no matching lines are found.
    """
    pattern = re.compile(r'^FAIL\s+(\S+)\s+\[setup failed\]', re.MULTILINE)
    return set(pattern.findall(content))


# ---------------------------------------------------------------------------
# ML-2B: removed section validation (D4)
# ---------------------------------------------------------------------------

_VALID_REMOVAL_NOTES = {"corrected", "renamed", "no-longer-runs"}


def validate_removed(
    removed: list[dict],
    active_entries: list[dict],
    go_passes: set[str],
    node_passes: set[str],
    py_passes: set[str],
    go_pass_vacuous: bool,
    node_pass_vacuous: bool,
    py_pass_vacuous: bool,
) -> bool:
    """Validate entries in the 'removed' section (ADR D4).

    Returns True if all removed entries are valid, False on any violation.

    Arm 1 (no removal_note)      — detected here: missing 'removal_note' field.
    Arm 2 (corrected claim false) — detected here for Go/Node/Python if pass
                                    set is not vacuous.
    Arm 3 (renamed without entry) — detected here: 'renamed_to' not in active.
    Arm 4 (valid removal)         — returns True.
    """
    # Build active-entry lookup by (name, runtime) for 'renamed' check
    active_keys: set[tuple[str, str]] = {
        (e["name"], e["runtime"]) for e in active_entries
    }

    ok = True
    for entry in removed:
        name = entry.get("name", "<unknown>")
        runtime = entry.get("runtime", "<unknown>")
        note = entry.get("removal_note")

        # Check 1: removal_note must be present
        if not note:
            _err(
                f"ML-2B D4: removed entry '{name}' (runtime: {runtime}) has no "
                "'removal_note'. Specify corrected | renamed | no-longer-runs. "
                "ADR D4: every retirement must declare which case applies so the "
                "list does not become a silent cemetery."
            )
            ok = False
            continue

        # Check 2: removal_note must be a known value
        if note not in _VALID_REMOVAL_NOTES:
            _err(
                f"ML-2B D4: removed entry '{name}' (runtime: {runtime}) has unknown "
                f"removal_note '{note}'. Valid values: corrected | renamed | no-longer-runs."
            )
            ok = False
            continue

        # Check 3: corrected → test must appear in passes
        if note == "corrected":
            if runtime == "go":
                if go_pass_vacuous:
                    _warn(
                        f"ML-2B D4: cannot verify removal_note='corrected' for '{name}' "
                        "(runtime: go) — pass observation set is vacuous (no '--- PASS:' "
                        "lines found). Suite may not have produced output. Skipping check."
                    )
                elif name not in go_passes:
                    _err(
                        f"ML-2B D4: removed entry '{name}' (runtime: go) has "
                        "removal_note='corrected' but the test does not appear in "
                        "'--- PASS:' output. It may be 'no-longer-runs' instead, or "
                        "the test is still failing."
                    )
                    ok = False
            elif runtime == "node":
                if node_pass_vacuous:
                    _warn(
                        f"ML-2B D4: cannot verify removal_note='corrected' for '{name}' "
                        "(runtime: node) — pass observation set is vacuous (no 'ok N -' "
                        "lines found in TAP). Skipping check."
                    )
                elif name not in node_passes:
                    _err(
                        f"ML-2B D4: removed entry '{name}' (runtime: node) has "
                        "removal_note='corrected' but the test does not appear in "
                        "TAP 'ok N - <name>' output. It may be 'no-longer-runs' instead, "
                        "or the test is still failing."
                    )
                    ok = False
            elif runtime == "python":
                if py_pass_vacuous:
                    _warn(
                        f"ML-2B D4: cannot verify removal_note='corrected' for '{name}' "
                        "(runtime: python) — pass observation set is vacuous (no 'PASSED' "
                        "lines found). Ensure pytest is run with -rA flag. Skipping check."
                    )
                elif name not in py_passes:
                    _err(
                        f"ML-2B D4: removed entry '{name}' (runtime: python) has "
                        "removal_note='corrected' but the test does not appear in "
                        "'PASSED' output. It may be 'no-longer-runs' instead, or "
                        "the test is still failing. (Requires pytest -rA flag.)"
                    )
                    ok = False

        # Check 4: renamed → renamed_to must be present and in active entries
        elif note == "renamed":
            renamed_to = entry.get("renamed_to")
            if not renamed_to:
                _err(
                    f"ML-2B D4: removed entry '{name}' (runtime: {runtime}) has "
                    "removal_note='renamed' but no 'renamed_to' field. Specify the "
                    "new test name. If the renamed test was also fixed, use "
                    "removal_note='corrected' instead."
                )
                ok = False
            elif (renamed_to, runtime) not in active_keys:
                _err(
                    f"ML-2B D4: removed entry '{name}' (runtime: {runtime}) has "
                    f"removal_note='renamed' with renamed_to='{renamed_to}', but "
                    f"'{renamed_to}' is not in the active entries for runtime '{runtime}'. "
                    "If the renamed test was also fixed, use removal_note='corrected'. "
                    "If it still fails, add it to the active entries."
                )
                ok = False

        # 'no-longer-runs': no further verification — cannot distinguish from
        # 'corrected' without collection data. The human declaring 'no-longer-runs'
        # is responsible for confirming the test no longer exists.

    return ok


def check_baseline_deletions(
    baseline_path: str,
    current_entries: list[dict],
    removed_entries: list[dict],
) -> bool:
    """Detect entries deleted from 'entries' without a corresponding 'removed' record.

    Arm 1 (baseline variant): entry in baseline, absent from current entries AND
    from the 'removed' section → exit 1. Every retirement must go through 'removed'.

    Returns True if all deletions are accounted for, False on any violation.
    Skips silently if baseline_path is empty or the file does not exist.
    """
    if not baseline_path:
        return True

    bp = Path(baseline_path)
    if not bp.exists():
        _warn(
            f"ML-2B: baseline file '{baseline_path}' not found; "
            "baseline deletion check disabled. "
            "(Expected: git show origin/main:.github/windows-known-failures.json)"
        )
        return True

    try:
        with bp.open(encoding="utf-8") as f:
            baseline_data = json.load(f)
    except Exception as e:
        _warn(f"ML-2B: could not parse baseline file '{baseline_path}': {e}. Skipping baseline check.")
        return True

    baseline_entries = baseline_data.get("entries", [])

    # Build key sets
    current_keys: set[tuple[str, str, str]] = {
        (e["name"], e["runtime"], e.get("class", ""))
        for e in current_entries
    }
    removed_keys: set[tuple[str, str, str]] = {
        (e["name"], e["runtime"], e.get("class", ""))
        for e in removed_entries
    }

    ok = True
    for entry in baseline_entries:
        key = (entry["name"], entry["runtime"], entry.get("class", ""))
        if key not in current_keys and key not in removed_keys:
            _err(
                f"ML-2B D4 (baseline): entry '{entry['name']}' (runtime: {entry['runtime']}) "
                "was in the baseline but is absent from both 'entries' and 'removed'. "
                "Move it to the 'removed' section with a removal_note "
                "(corrected | renamed | no-longer-runs) before removing from 'entries'."
            )
            ok = False

    # Positive confirmation — makes "baseline ran and found nothing" observable.
    # Two-states-one-observable: without this line, "clean" and "skipped" are
    # indistinguishable in the log (same silence). T15 asserts this line appears
    # when a baseline is given and is absent when baseline_path is empty.
    if ok:
        print(
            f"ML-2B D4 (baseline): {len(baseline_entries)} entrada(s) comparadas — "
            "nenhuma deleção silenciosa.",
            flush=True,
        )

    return ok


# ---------------------------------------------------------------------------
# Ratchet check
# ---------------------------------------------------------------------------

def run_check(
    list_path: str,
    go_out: str,
    node_tap: str,
    python_out: str,
    baseline_path: str = "",
    load_markers_dir: str = "",
) -> int:
    """Run the ratchet check. Returns 0 (pass) or 1 (new failure(s) detected)."""

    # 1. Load known list (vacuity guard inside — raises SystemExit(1) if broken)
    known_data = load_known_data(list_path)
    known = known_data.get("entries", [])
    removed = known_data.get("removed", [])

    # Build lookup sets by runtime+class
    known_go_assert   = {e['name'] for e in known if e['runtime'] == 'go'     and e['class'] == 'assertion'}
    known_go_load     = {e['name'] for e in known if e['runtime'] == 'go'     and e['class'] == 'suite-load-failure'}
    known_node_assert = {e['name'] for e in known if e['runtime'] == 'node'   and e['class'] == 'assertion'}
    known_node_load   = {e['name'] for e in known if e['runtime'] == 'node'   and e['class'] == 'suite-load-failure'}
    known_py_assert   = {e['name'] for e in known if e['runtime'] == 'python' and e['class'] == 'assertion'}

    # 2. Require artifacts (observation-side vacuity guard — file-existence)
    go_path  = require_artifact(go_out,     'go-suite-out.txt')
    tap_path = require_artifact(node_tap,   'node-suite.tap')
    py_path  = require_artifact(python_out, 'python-suite-out.txt')

    # 3. ML-3A: early marker check — markers where no name is extractable (row 3 only).
    #    Suite steps write marker files BEFORE exiting; continue-on-error absorbs the exit
    #    code but not the marker. The ratchet reads markers here and in step 3b.
    #    "Não consegui procurar → fatal, nunca aviso." (vault note: dois-estados-um-observable)
    #
    #    Row classification by marker (measured, 2026-09-10):
    #      suite-load-failure.python.txt — content: fixed string 'python-suite-load-failure
    #                                      (exit code $rc)'. No extractable name → row 3.
    #      zero-test-failure.node.txt    — content: fixed string 'zero-test'. No test name
    #                                      by construction ('# tests 0') → row 3.
    #      zero-test-failure.python.txt  — content: fixed string 'python-zero-test-failure
    #                                      (exit code 5)'. No test name → row 3.
    #    Go and Node suite-load-failure markers carry names; handled in step 3b (after
    #    extraction) so the known list can be consulted. See classification there.
    if load_markers_dir:
        early_fail = False
        early_specs = [
            ("suite-load-failure.python.txt", "Python suite-load-failure"),
            ("zero-test-failure.node.txt",    "Node.js zero-test-failure"),
            ("zero-test-failure.python.txt",  "Python zero-test-failure"),
        ]
        for fname, label in early_specs:
            marker = Path(load_markers_dir) / fname
            if marker.exists():
                content = marker.read_text(encoding="utf-8").strip()
                _err(
                    f"ML-3A: {label} — sem nome extraível (row 3: o pior modo, a contagem some "
                    f"sem rastro). Detalhe: {content}"
                )
                early_fail = True
        if early_fail:
            return 1

    # 4. Extract observed failures
    obs_go = extract_go_failures(go_path)
    obs_node_assert, obs_node_load = extract_node_failures(tap_path)
    obs_py = extract_python_failures(py_path)

    # 5. Extract observed passes (ML-2B: discriminant for corrected vs no-longer-runs)
    go_passes,   go_pass_vac   = extract_go_passes(go_path)
    node_passes, node_pass_vac = extract_node_passes(tap_path)
    py_passes,   py_pass_vac   = extract_python_passes(py_path)

    # 5b. ML-3A: results-present vacuity guard.
    #     If an artifact exists but contains NO test result lines, that is
    #     "não consegui procurar" — not "procurei e não achei" (vault note).
    #     Fatal, not a warning. is_vacuous=True means no FAIL/PASS lines at all.
    if go_pass_vac and not obs_go:
        _err(
            "ML-3A vacuity (results-present): go-suite-out.txt exists but contains no "
            "'--- FAIL:' or '--- PASS:' lines. Suite may not have produced results — "
            "'não consegui procurar' → fatal (not a warning)."
        )
        return 1
    if node_pass_vac and not obs_node_assert and not obs_node_load:
        _err(
            "ML-3A vacuity (results-present): node-suite.tap exists but contains no "
            "'ok'/'not ok' lines. Suite may not have produced results — "
            "'não consegui procurar' → fatal (not a warning)."
        )
        return 1
    if py_pass_vac and not obs_py:
        _err(
            "ML-3A vacuity (results-present): python-suite-out.txt exists but contains no "
            "'FAILED' or 'PASSED' lines. Suite may not have produced results — "
            "'não consegui procurar' → fatal (not a warning)."
        )
        return 1

    has_new = False

    # 3b. ML-3A: late marker check — Go and Node suite-load-failure (names available).
    #     Done after extraction so the known list can be consulted.
    #
    #     Row classification (measured, 2026-09-10):
    #       suite-load-failure.go.txt  — content: 'FAIL\t<pkg> [setup failed]' lines written
    #                                    by the workflow. Package path is stable. Parsed by
    #                                    _parse_go_load_names(). Consult known_go_load:
    #                                    row 1 (name in list) | row 2 (name not in list,
    #                                    new failure) | row 3 (content unparseable, no name).
    #       suite-load-failure.node.txt — content: 'exitCode:' YAML lines from TAP — these
    #                                    are NOT names. Names come from extract_node_failures()
    #                                    → obs_node_load. Marker is a signal only.
    #                                    row 1/2: obs_node_load non-empty → step 6 ratchet
    #                                             handles by name (known → pass, new → fail).
    #                                    row 3: marker exists AND obs_node_load empty → load
    #                                           failure produced no name in TAP → fatal.
    if load_markers_dir:
        # Go suite-load-failure: extract names from marker content, consult list
        go_load_marker = Path(load_markers_dir) / "suite-load-failure.go.txt"
        if go_load_marker.exists():
            go_marker_content = go_load_marker.read_text(encoding="utf-8").strip()
            go_load_names = _parse_go_load_names(go_marker_content)
            if not go_load_names:
                # Row 3: marker exists but no package name parseable
                _err(
                    f"ML-3A: Go suite-load-failure sem nome extraível (row 3 — o pior modo, "
                    f"a contagem some sem rastro). Detalhe: {go_marker_content}"
                )
                has_new = True
            else:
                for pkg in sorted(go_load_names):
                    if pkg not in known_go_load:
                        # Row 2: new Go load failure, package not in known list
                        _err(
                            f"ML-3A: NEW Go suite-load-failure not in known list: '{pkg}'. "
                            "Fix the compilation error. "
                            "If this is inherited Windows-only debt that pre-dates this PR, "
                            "add to .github/windows-known-failures.json with a source run id. "
                            "(ADR D3: suite-load-failure has its own class — not a test assertion.)"
                        )
                        has_new = True
                    # Row 1: pkg in known_go_load — known debt, pass silently

        # Node.js suite-load-failure: marker is a signal; names come from obs_node_load
        node_load_marker = Path(load_markers_dir) / "suite-load-failure.node.txt"
        if node_load_marker.exists():
            if not obs_node_load:
                # Row 3: marker signals a load failure but TAP produced no name
                node_marker_content = node_load_marker.read_text(encoding="utf-8").strip()
                _err(
                    f"ML-3A: Node.js suite-load-failure sem nome no TAP (row 3 — o pior modo: "
                    f"suíte falhou sem produzir nome rastreável no TAP). "
                    f"Detalhe do marcador: {node_marker_content}"
                )
                has_new = True
            # Rows 1 and 2: obs_node_load non-empty — step 6 ratchet handles by name

    # 6. New failures NOT in the known list -> ::error:: + exit 1
    for name in sorted(obs_go - known_go_assert):
        _err(
            f"ML-2A ratchet: NEW Go assertion failure not in known list: '{name}'. "
            "Fix the test. "
            "If this is inherited Windows-only debt that pre-dates this PR, "
            "add to .github/windows-known-failures.json with a source run id."
        )
        has_new = True

    for name in sorted(obs_node_assert - known_node_assert):
        _err(
            f"ML-2A ratchet: NEW Node.js assertion failure not in known list: '{name}'. "
            "Fix the test. "
            "If this is inherited Windows-only debt that pre-dates this PR, "
            "add to .github/windows-known-failures.json with a source run id."
        )
        has_new = True

    for name in sorted(obs_node_load - known_node_load):
        _err(
            f"ML-2A ratchet: NEW Node.js suite-load-failure not in known list: '{name}'. "
            "Fix the suite. "
            "If this is inherited Windows-only debt that pre-dates this PR, "
            "add to .github/windows-known-failures.json with a source run id."
        )
        has_new = True

    for name in sorted(obs_py - known_py_assert):
        _err(
            f"ML-2A ratchet: NEW Python assertion failure not in known list: '{name}'. "
            "Fix the test. "
            "If this is inherited Windows-only debt that pre-dates this PR, "
            "add to .github/windows-known-failures.json with a source run id."
        )
        has_new = True

    # 7. Known entries NOT observed (fixed/renamed) -> ::warning:: only, never exit 1
    #    Fixing a test must not break CI (that would make the ratchet a trap).
    for name in sorted(known_go_assert - obs_go):
        _warn(
            f"ML-2A ratchet: Go assertion '{name}' is in the known list but did NOT fail. "
            "Please move to the 'removed' section in .github/windows-known-failures.json "
            "with a 'removal_note' field (ML-2B: corrected | renamed | no-longer-runs)."
        )

    for name in sorted(known_node_assert - obs_node_assert):
        _warn(
            f"ML-2A ratchet: Node.js assertion '{name}' is in the known list but did NOT fail. "
            "Please move to the 'removed' section with a 'removal_note' (ML-2B)."
        )

    for name in sorted(known_node_load - obs_node_load):
        _warn(
            f"ML-2A ratchet: Node.js suite-load-failure '{name}' is in the known list but did NOT fail. "
            "Please move to the 'removed' section with a 'removal_note' (ML-2B)."
        )

    for name in sorted(known_py_assert - obs_py):
        _warn(
            f"ML-2A ratchet: Python assertion '{name}' is in the known list but did NOT fail. "
            "Please move to the 'removed' section with a 'removal_note' (ML-2B)."
        )

    # 8. ML-2B: baseline deletion check (D4 — silent deletion via git diff)
    if not check_baseline_deletions(baseline_path, known, removed):
        has_new = True

    # 9. ML-2B: validate removed section entries (D4)
    if not validate_removed(
        removed, known,
        go_passes, node_passes, py_passes,
        go_pass_vac, node_pass_vac, py_pass_vac,
    ):
        has_new = True

    # 10. Informational summary — never used for decisions (ADR D1: discriminant is the name set).
    #     The summary is derived from the same set differences as steps 6 and 7, so the
    #     invariant holds: a failing gate (has_new=True, meaning obs-known is non-empty for some
    #     class) always produces at least one [+N NOVO] on the first and only summary line.
    #
    #     Defect corrected (2026-09-11, PR #316): when one class gained failures (+1) and another
    #     lost them (-1) by the same count, the total headline "38 observed / 38 active" looked
    #     clean while the gate failed — CI run 34547480139: Node-assert 11/10, Python 12/13.
    #     Root cause: the previous code printed the total without per-class direction; a reader
    #     seeing only that line reached the most reasonable wrong conclusion.
    #
    #     Fix: imbalance in ANY class is visible on the same first line, with direction shown.
    #     Uses set difference (not count) so equal counts with different names are also caught.
    total_obs     = len(obs_go) + len(obs_node_assert) + len(obs_node_load) + len(obs_py)
    total_known   = len(known)
    total_removed = len(removed)

    def _cls_label(obs_set: set, known_set: set, prefix: str) -> tuple:
        """Build per-class label with direction indicator using set difference.

        Returns (label_string, has_imbalance).
        Invariant: if steps 6/7 fire for this class, has_imbalance is True and the
        label contains a direction tag — same set difference, same result.
        """
        surplus  = obs_set - known_set   # new failures in this class (step 6 direction)
        resolved = known_set - obs_set   # debt paid in this class  (step 7 direction)
        label = f"{prefix} {len(obs_set)}/{len(known_set)}"
        if surplus and resolved:
            label += f" [+{len(surplus)} NOVO, -{len(resolved)} resolvido]"
        elif surplus:
            label += f" [+{len(surplus)} NOVO]"
        elif resolved:
            label += f" [-{len(resolved)} resolvido]"
        return label, bool(surplus or resolved)

    go_lbl, go_imb = _cls_label(obs_go,         known_go_assert,  "Go")
    na_lbl, na_imb = _cls_label(obs_node_assert, known_node_assert, "Node-assert")
    nl_lbl, nl_imb = _cls_label(obs_node_load,   known_node_load,  "Node-load")
    py_lbl, py_imb = _cls_label(obs_py,          known_py_assert,  "Python")

    headline = (
        " — DESEQUILÍBRIO POR CLASSE"
        if (go_imb or na_imb or nl_imb or py_imb)
        else ""
    )
    print(
        f"ML-2A/2B: {total_obs} observed / {total_known} active / {total_removed} removed"
        f"{headline}. "
        f"{go_lbl}, {na_lbl}, {nl_lbl}, {py_lbl}.",
        flush=True
    )

    return 1 if has_new else 0


# ---------------------------------------------------------------------------
# Self-test (ML-2A T1-T9 + ML-2B T10-T14)
# ---------------------------------------------------------------------------

def run_self_test() -> int:
    """Run built-in falsification suite.

    Reconciliation (per project rule: each test/guard declares in one sentence
    what conclusion of this ML it asserts, confronted against the measurement):

    ── ML-2A (T1–T9) ──────────────────────────────────────────────────────────

    T1 — 'observed == known on all 4 sets' -> exit 0
         Asserts: when every list entry fails and no new name appears, verifier exits clean.

    T2 — 'new Go name not in list' -> exit 1
         Asserts: ADR D1 — a name outside the known set causes verifier to fail.

    T3 — 'known entry missing from observed' -> exit 0 (warning only)
         Asserts: fixing a test must not break CI; disappearance is a warning, not a block.

    T4 — 'empty list' -> vacuity guard -> SystemExit(1)
         Asserts: an empty list with active debt passes everything silently — the guard
         must fire before any comparison is attempted.

    T5 — 'list file missing' -> vacuity guard -> SystemExit(1)
         Asserts: a missing file is indistinguishable from an empty list; same guard.

    T6 — 'artifact missing (go-out)' -> observation-side vacuity -> SystemExit(1)
         Asserts: missing artifact -> empty observation set -> 38 spurious removal warnings
         + exit 0; the guard must fire before the comparison.

    T7 — 'Node suite-load-failure with full Windows runner path' -> basename match
         Asserts: D5 decision — full path is unstable; basename is the canonical name.
         Measured: run 34478752778, not-ok 79 subject = 'D:\\a\\trackfw\\...\\validator.test.js'.

    T8 — 'Python class method with Windows backslash path' -> normalized match
         Asserts: D5 decision — pypi\\tests\\TestClass::method normalized to
         'test_file.py::TestClass::method', which is the form stored in the list.

    T9 — 'non-ASCII name round-trip (em-dash, accented chars)' -> no encoding loss
         Asserts: 4 Node names with non-ASCII chars (em-dash, accents) survive
         JSON -> file -> extract cycle. Encoding mismatch looks identical to a new failure.

    ── ML-2B (T10–T14) ─────────────────────────────────────────────────────────

    T10 — 'baseline entry deleted without removed record' -> exit 1
          Asserts: ADR D4 — entries deleted from 'entries' without a 'removed' record
          are caught by the --baseline diff. Measured: Go pass output contains '--- PASS:
          TestFoo' (test was corrected), but no removed record exists.

    T11 — 'removed entry without removal_note' -> exit 1
          Asserts: ADR D4 — every entry in 'removed' must have a 'removal_note' field.
          An entry without a note is an unexplained disappearance.

    T12 — 'removal_note=corrected but test not in PASS output (Go)' -> exit 1
          Asserts: measured discriminant (Go -v: '--- PASS:' present for corrected,
          absent for no-longer-runs) — if pass observation is non-vacuous and the test
          is absent, the 'corrected' claim is wrong.

    T13 — 'removal_note=renamed but renamed_to not in active entries' -> exit 1
          Asserts: ADR D4 — a renamed test that still fails must be in the active list;
          if renamed_to is absent from entries, either the rename was also a fix (use
          'corrected') or the new name was forgotten.

    T14 — 'valid removal (corrected + test in PASS output)' -> exit 0
          Asserts: vacuity guard — the ratchet must not reprove all removals;
          a well-formed 'corrected' entry with the test in passes exits clean.

    T15 — 'baseline positive confirmation line' -> emitted with baseline, absent without
          Asserts: check_baseline_deletions emits 'ML-2B D4 (baseline): N entrada(s)
          comparadas' when a baseline is provided and all entries are accounted for.
          When baseline_path is empty the line is absent (two-states-one-observable
          fix: "clean" and "skipped" were previously indistinguishable in the log).

    ── ML-3A (T16–T22) ─────────────────────────────────────────────────────────

    T16 — 'Go suite-load-failure marker, package not in known_go_load' -> exit 1 (row 2)
          Asserts: Go load-failure marker content 'FAIL\t<pkg> [setup failed]' yields a
          package path; _parse_go_load_names extracts it; package compared against
          known_go_load; 'github.com/kgsaran/trackfw/internal/badpkg' is not in the empty
          known_go_load → row 2 (name produced, not in list) → exit 1 with package named.
          Measurement: marker content format from run 34478752778; workflow line ~603.
          [UPDATED: previously tested blanket check; now exercises step-3b "consult list"
           path. Behavior unchanged (no Go load entries → always row 2 today) but
           mechanism now satisfiable by adding an entry.]

    T17 — 'no load-failure markers, only known failures' -> exit 0 (counter-arm)
          Asserts: when markers_dir is empty (no marker files), neither step-3 early check
          nor step-3b late check fires; known names pass step 6 → exit 0.
          Without this arm, a guard that always fails would look correct.
          [SYNTHETIC: markers_dir exists but is empty]

    T18 — 'go-suite-out.txt present but vacuous (no FAIL/PASS lines)' -> exit 1
          Asserts: ML-3A results-present vacuity guard (vault note: "não consegui
          procurar → fatal, nunca aviso") — go-suite-out.txt containing only a
          '[setup failed]' line (no test result lines) means the suite did not produce
          results; the guard fires before the ratchet compares names and emits spurious
          "not observed" warnings for all 14 known Go entries.
          [SYNTHETIC: go-suite-out.txt written with only a [setup failed] line]

    T19 — 'Node.js load-failure marker + name in known_node_load' -> exit 0 (row 1)
          Asserts: when a Node suite-load-failure marker exists but all extracted names
          (from TAP) are in known_node_load (e.g. 'broken.test.js'), the ratchet exits
          clean — known debt does not block CI. This is the arm whose absence shipped the
          original defect: 'validator.test.js' was in the known list but the blanket marker
          check failed regardless, making the job permanently un-green.
          [SYNTHETIC: marker file present; TAP has broken.test.js with exitCode → row 1]

    T20 — 'Node.js load-failure marker + name NOT in known_node_load' -> exit 1, named (row 2)
          Asserts: 'new_broken.test.js' extracted from TAP (exitCode block present), not
          in known_node_load → step-6 ratchet fires naming the file → exit 1.
          Marker exists but row-1/2 decision belongs to step-6, not step-3b (step-3b only
          handles the row-3 no-name case). Message contains 'new_broken.test.js'.
          [SYNTHETIC: marker present; TAP has new_broken.test.js with exitCode]

    T21 — 'Node.js load-failure marker + obs_node_load empty' -> exit 1, "sem nome" (row 3)
          Asserts: marker exists (signal: load failure occurred) but TAP has no exitCode
          block → obs_node_load empty → step-3b detects row-3 (D3: worst mode, contagem
          some sem rastro) → exit 1 with "sem nome" in message.
          [SYNTHETIC: marker present; TAP has only assertion failures, no exitCode blocks]

    T22 — 'Node.js zero-test-failure marker' -> exit 1 (row 3, early check)
          Asserts: zero-test produces no test name by construction ('# tests 0' + exit 0)
          → row 3 → step-3 early check fires immediately → exit 1.
          [SYNTHETIC: zero-test-failure.node.txt present with fixed-string content]

    ── Sumário (T23–T26) — corretivo 2026-09-11, PR #316 ───────────────────────

    T23 — 'cancel case: Node-assert +1 NOVO + Python -1 resolvido, total balanced'
          -> first line shows 'DESEQUILÍBRIO POR CLASSE' and direction tags
          Asserts: set-based imbalance detection exposes the cancellation that count-based
          total masks. CI case: run 34547480139, Node-assert 11/10, Python 12/13,
          "38 observed / 38 active" looked clean while gate failed.
          Measurement: without the fix, a reader seeing only the first line reached the
          most reasonable wrong conclusion — "a catraca foi desligada".
          [SYNTHETIC: 2 Node-assert obs (known + new_name), 0 Python obs, 1 Python known]

    T24 — 'all classes balanced' -> no 'DESEQUILÍBRIO' on first line (counter-arm)
          Asserts: when every observed name matches a known name and vice versa, the
          headline is clean and no direction tags appear. Without this arm, a guard that
          always flags would appear to work.
          [REUSES: BASE_ENTRIES + default write_artifacts — same fixture as T1]

    T25 — 'one class surplus, others balanced' -> first line shows '[+1 NOVO]' (single-class arm)
          Asserts: imbalance in a single class is flagged independently of other classes.
          [SYNTHETIC: Node-assert 2 obs (1 known + 1 new), Go/Node-load/Python matched]

    T26 — 'equal count but different names in a class' -> first line shows '[+1 NOVO]'
          Asserts: the fix uses set difference, not count comparison. When obs and known have
          the same count but different names (one name replaced), count-based logic would
          print a clean headline while set-based detection exposes the surplus. This arm
          separates the correct fix from the plausible-wrong count-based alternative.
          Measurement: Node-assert known={A,B}, obs={A,C} → count 2/2 but surplus={C},
          resolved={B} → '+1 NOVO, -1 resolvido' tag appears despite equal count.
          [SYNTHETIC: Node-assert with two known entries, one replaced in observation]
    """
    n_pass = 0
    n_fail = 0

    def check(cond: bool, description: str) -> None:
        nonlocal n_pass, n_fail
        if cond:
            _st_print(f"  SELF-TEST PASS: {description}")
            n_pass += 1
        else:
            _st_print(f"  SELF-TEST FAIL: {description}", stream=sys.stderr)
            n_fail += 1

    with tempfile.TemporaryDirectory() as td:
        list_path      = os.path.join(td, "known.json")
        baseline_path  = os.path.join(td, "baseline.json")
        go_path        = os.path.join(td, "go-suite.txt")
        tap_path       = os.path.join(td, "node.tap")
        py_path        = os.path.join(td, "python.txt")

        # Canonical sample content matching real CI output format
        GO_FAIL    = "--- FAIL: TestFoo (0.01s)\n"
        GO_FAIL_2  = "--- FAIL: TestBar (0.01s)\n"
        GO_PASS    = "--- PASS: TestFoo (0.01s)\n"
        GO_OTHER_PASS = "--- PASS: TestOther (0.01s)\n"
        TAP_ASSERT = (
            "not ok 1 - sample assertion test\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  code: 'ERR_ASSERTION'\n"
            "  ...\n"
        )
        TAP_PASS = (
            "ok 1 - sample assertion test\n"
            "  ---\n"
            "  duration_ms: 0.5\n"
            "  ...\n"
        )
        TAP_LOAD = (
            "not ok 2 - /runner/work/trackfw/tests/broken.test.js\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  exitCode: 1\n"
            "  ...\n"
        )
        PY_FAIL = (
            "FAILED pypi/tests/test_foo.py::test_bar - AssertionError: x\n"
        )
        PY_PASS = (
            "PASSED pypi/tests/test_foo.py::test_bar\n"
        )

        BASE_ENTRIES = [
            {"name": "TestFoo",               "runtime": "go",     "class": "assertion"},
            {"name": "sample assertion test",  "runtime": "node",   "class": "assertion"},
            {"name": "broken.test.js",         "runtime": "node",   "class": "suite-load-failure"},
            {"name": "test_foo.py::test_bar",  "runtime": "python", "class": "assertion"},
        ]

        def write_list(entries: list[dict], removed: list[dict] | None = None) -> None:
            data = {
                "_meta": {"source": {"run_id": "self-test"}},
                "entries": entries,
                "removed": removed if removed is not None else [],
            }
            Path(list_path).write_text(
                json.dumps(data, ensure_ascii=False), encoding="utf-8"
            )

        def write_baseline(entries: list[dict]) -> None:
            data = {
                "_meta": {"source": {"run_id": "self-test-baseline"}},
                "entries": entries,
                "removed": [],
            }
            Path(baseline_path).write_text(
                json.dumps(data, ensure_ascii=False), encoding="utf-8"
            )

        def write_artifacts(
            go: str = GO_FAIL,
            tap: str = TAP_ASSERT + TAP_LOAD,
            py: str = PY_FAIL,
        ) -> None:
            Path(go_path).write_text(go, encoding="utf-8")
            Path(tap_path).write_text(tap, encoding="utf-8")
            Path(py_path).write_text(py, encoding="utf-8")

        # ── T1: all observed match known ─────────────────────────────────────
        _st_print("=== T1: all observed match known -> exit 0 ===")
        write_list(BASE_ENTRIES)
        write_artifacts()
        with _capture_annotations():
            rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T1: all known observed -> exit 0")

        # ── T2: new Go failure not in list ────────────────────────────────────
        _st_print("=== T2: new Go failure not in list -> exit 1 ===")
        write_list(BASE_ENTRIES)
        write_artifacts(go=GO_FAIL + GO_FAIL_2)  # TestBar is not in list
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 1, "T2: new Go failure -> exit 1")
        check("::error::" in ann.getvalue(), "T2 AC5: _err() called (error arm executed)")

        # ── T3: known entry not observed -> warning, exit 0 ──────────────────
        _st_print("=== T3: known entry missing from observed -> warning, exit 0 ===")
        extra = BASE_ENTRIES + [
            {"name": "TestKnownButFixed", "runtime": "go", "class": "assertion"}
        ]
        write_list(extra)
        write_artifacts()  # TestKnownButFixed absent from go output
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T3: known entry not observed -> warning only, exit 0")
        check("::warning::" in ann.getvalue(), "T3 AC5: _warn() called (warning arm executed)")

        # ── T4: empty list -> vacuity guard -> SystemExit(1) ─────────────────
        _st_print("=== T4: empty list -> vacuity guard -> SystemExit(1) ===")
        Path(list_path).write_text(
            '{"entries": [], "removed": []}', encoding="utf-8"
        )
        write_artifacts()
        with _capture_annotations() as ann:
            fired = False
            try:
                run_check(list_path, go_path, tap_path, py_path)
            except SystemExit as e:
                fired = (e.code == 1)
        check(fired, "T4: empty list -> SystemExit(1)")
        check("::error::" in ann.getvalue(), "T4 AC5: vacuity guard called _err()")

        # ── T5: list file missing -> vacuity guard -> SystemExit(1) ──────────
        _st_print("=== T5: missing list file -> vacuity guard -> SystemExit(1) ===")
        if os.path.exists(list_path):
            os.unlink(list_path)
        write_artifacts()
        with _capture_annotations() as ann:
            fired = False
            try:
                run_check(list_path, go_path, tap_path, py_path)
            except SystemExit as e:
                fired = (e.code == 1)
        check(fired, "T5: missing list -> SystemExit(1)")
        check("::error::" in ann.getvalue(), "T5 AC5: vacuity guard called _err()")

        # ── T6: artifact missing -> observation-side vacuity -> SystemExit(1) ─
        _st_print("=== T6: missing go artifact -> SystemExit(1) ===")
        write_list(BASE_ENTRIES)
        write_artifacts()
        os.unlink(go_path)
        with _capture_annotations() as ann:
            fired = False
            try:
                run_check(list_path, go_path, tap_path, py_path)
            except SystemExit as e:
                fired = (e.code == 1)
        check(fired, "T6: missing artifact -> SystemExit(1)")
        check("::error::" in ann.getvalue(), "T6 AC5: artifact guard called _err()")

        # ── T7: Node suite-load-failure with Windows full path ────────────────
        _st_print("=== T7: Node suite-load with Windows path -> basename match ===")
        write_list(BASE_ENTRIES)
        win_tap = (
            "not ok 1 - sample assertion test\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  code: 'ERR_ASSERTION'\n"
            "  ...\n"
            "not ok 2 - D:\\a\\trackfw\\trackfw\\npm\\tests\\broken.test.js\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  exitCode: 1\n"
            "  ...\n"
        )
        write_artifacts(tap=win_tap)
        with _capture_annotations():
            rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T7: Windows path in TAP -> basename 'broken.test.js' matches known entry")

        # ── T8: Python class method with Windows backslash path ───────────────
        _st_print("=== T8: Python class method + Windows backslash -> normalized ===")
        entries_with_class = BASE_ENTRIES + [
            {
                "name": "test_commands_basic.py::TestRealCommands::test_status_uses_real_handler",
                "runtime": "python",
                "class": "assertion",
            }
        ]
        write_list(entries_with_class)
        py_win = (
            "FAILED pypi\\tests\\test_foo.py::test_bar - AssertionError\n"
            "FAILED pypi\\tests\\test_commands_basic.py::TestRealCommands::test_status_uses_real_handler - TypeError\n"
        )
        write_artifacts(py=py_win)
        with _capture_annotations():
            rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T8: Windows backslash Python path normalized -> class method matches")

        # ── T9: non-ASCII name round-trip ─────────────────────────────────────
        _st_print("=== T9: non-ASCII name (em-dash, accents) round-trip ===")
        non_ascii_name = "sem identidade \u2014 sa\u00edda id\u00eantica ao comportamento pr\u00e9-existente (n\u00e3o-regress\u00e3o)"
        entries_utf8 = [
            {"name": "TestFoo",              "runtime": "go",     "class": "assertion"},
            {"name": non_ascii_name,          "runtime": "node",   "class": "assertion"},
            {"name": "broken.test.js",        "runtime": "node",   "class": "suite-load-failure"},
            {"name": "test_foo.py::test_bar", "runtime": "python", "class": "assertion"},
        ]
        write_list(entries_utf8)
        non_ascii_tap = (
            f"not ok 1 - {non_ascii_name}\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  ...\n"
            "not ok 2 - D:\\broken.test.js\n"
            "  ---\n"
            "  exitCode: 1\n"
            "  ...\n"
        )
        write_artifacts(tap=non_ascii_tap)
        with _capture_annotations():
            rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T9: non-ASCII test name survives JSON->file->extract round-trip")

        # ── ML-2B T10: baseline entry deleted without removed record -> exit 1 ─
        # Asserts: entry in baseline, absent from current entries, NOT in removed
        # → baseline check fires. The test passes (appears as '--- PASS: TestFoo')
        # so ML-2A does NOT fire for a new failure; only baseline check fires.
        _st_print("=== T10: baseline entry deleted, no removed record -> exit 1 ===")
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        write_list(active_without_testfoo, [])  # TestFoo gone, removed is empty
        write_baseline(BASE_ENTRIES)             # TestFoo was in baseline
        write_artifacts(go=GO_PASS)              # TestFoo passes now (not in fail output)
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T10: baseline entry deleted without removed record -> exit 1")
        check("::error::" in ann.getvalue(), "T10 AC5: baseline check called _err()")

        # ── ML-2B T11: removed entry without removal_note -> exit 1 ──────────
        # Asserts: an entry in 'removed' without 'removal_note' must cause exit 1.
        # This is the intra-file validation arm of D4.
        _st_print("=== T11: removed entry without removal_note -> exit 1 ===")
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_no_note = [{"name": "TestFoo", "runtime": "go", "class": "assertion"}]
        write_list(active_without_testfoo, removed_no_note)
        write_baseline(BASE_ENTRIES)
        write_artifacts(go=GO_PASS)
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T11: removed entry without removal_note -> exit 1")
        check("::error::" in ann.getvalue(), "T11 AC5: removal validation called _err()")

        # ── ML-2B T12: corrected claim but test not in PASS output -> exit 1 ──
        # Asserts: measured discriminant (Go -v '--- PASS:' present for corrected,
        # absent for no-longer-runs) — non-vacuous pass set without test name means
        # the corrected claim cannot be verified.
        _st_print("=== T12: removal_note=corrected but test not in PASS output -> exit 1 ===")
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_corrected = [{
            "name": "TestFoo", "runtime": "go", "class": "assertion",
            "removal_note": "corrected",
        }]
        write_list(active_without_testfoo, removed_corrected)
        write_baseline(BASE_ENTRIES)
        # GO_OTHER_PASS: non-vacuous (has a PASS line) but TestFoo is NOT there
        write_artifacts(go=GO_OTHER_PASS)
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T12: corrected claim without TestFoo in PASS output -> exit 1")
        check("::error::" in ann.getvalue(), "T12 AC5: corrected-claim check called _err()")

        # ── ML-2B T13: renamed without renamed_to in active entries -> exit 1 ─
        # Asserts: if removal_note=renamed and renamed_to is absent from active entries,
        # the checker fires. The fixture has TestFooRenamed passing (not in fails),
        # so ML-2A cannot be the cause — only the renamed_to validation fires.
        _st_print("=== T13: removal_note=renamed, renamed_to not in entries -> exit 1 ===")
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_renamed = [{
            "name": "TestFoo", "runtime": "go", "class": "assertion",
            "removal_note": "renamed", "renamed_to": "TestFooRenamed",
        }]
        write_list(active_without_testfoo, removed_renamed)
        write_baseline(BASE_ENTRIES)
        # TestFooRenamed passes (not failing), TestFoo passes — neither in fail output
        write_artifacts(go="--- PASS: TestFooRenamed (0.01s)\n")
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T13: renamed_to='TestFooRenamed' not in active entries -> exit 1")
        check("::error::" in ann.getvalue(), "T13 AC5: renamed_to check called _err()")

        # ── ML-2B T14: valid removal (corrected + TestFoo in PASS output) -> exit 0
        # Asserts: the ratchet must not block all removals — a well-formed 'corrected'
        # entry with the test appearing in pass output exits cleanly (vacuity guard
        # for the removal mechanism itself).
        _st_print("=== T14: valid removal (corrected + test in PASS output) -> exit 0 ===")
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_valid = [{
            "name": "TestFoo", "runtime": "go", "class": "assertion",
            "removal_note": "corrected",
        }]
        write_list(active_without_testfoo, removed_valid)
        write_baseline(BASE_ENTRIES)
        write_artifacts(go=GO_PASS)  # TestFoo appears as PASS
        with _capture_annotations():
            rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 0, "T14: valid corrected removal with TestFoo in PASS output -> exit 0")

        # ── ML-2B T15: baseline confirmation line emitted with baseline, absent without
        # Asserts: check_baseline_deletions emits 'ML-2B D4 (baseline): N entrada(s)
        # comparadas' when baseline is provided; line is absent when baseline_path=''.
        # Fixes two-states-one-observable: "clean" and "skipped" were indistinguishable.
        _st_print("=== T15: baseline positive confirmation line -> present with baseline, absent without ===")
        write_list(BASE_ENTRIES, [])   # all entries in active, none removed
        write_baseline(BASE_ENTRIES)   # baseline matches current entries exactly

        # T15a: baseline provided → confirmation line appears
        # check_baseline_deletions() uses print() not _err()/_warn(), so
        # redirect_stdout captures its output directly (no annotation sink needed).
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            result = check_baseline_deletions(baseline_path, BASE_ENTRIES, [])
        out = buf.getvalue()
        check(
            result is True and "ML-2B D4 (baseline):" in out and "comparadas" in out,
            "T15a: baseline clean -> 'ML-2B D4 (baseline): N entrada(s) comparadas' emitted",
        )

        # T15b: empty baseline_path → confirmation line absent (baseline skipped)
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            result = check_baseline_deletions("", BASE_ENTRIES, [])
        out = buf.getvalue()
        check(
            result is True and "comparadas" not in out,
            "T15b: baseline skipped (empty path) -> no confirmation line",
        )

        # ── ML-3A T16: Go suite-load-failure marker present -> exit 1 ──────────
        _st_print("=== T16: Go suite-load-failure marker present -> exit 1 ===")
        markers_dir = os.path.join(td, "markers")
        os.makedirs(markers_dir, exist_ok=True)
        write_list(BASE_ENTRIES)
        write_artifacts()  # normal artifacts — no new test names outside known list
        Path(os.path.join(markers_dir, "suite-load-failure.go.txt")).write_text(
            "FAIL\tgithub.com/kgsaran/trackfw/internal/badpkg [setup failed]",
            encoding="utf-8",
        )
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_dir)
        check(rc == 1, "T16: Go suite-load-failure marker -> exit 1")
        check("::error::" in ann.getvalue(), "T16 AC5: load-failure ratchet called _err()")
        # Remove marker for T17
        os.remove(os.path.join(markers_dir, "suite-load-failure.go.txt"))

        # ── ML-3A T17: no markers + known-only failures -> exit 0 (row 4 counter-arm)
        _st_print("=== T17: no load-failure markers -> exit 0 (row 4 counter-arm) ===")
        write_list(BASE_ENTRIES)
        write_artifacts()
        with _capture_annotations():
            rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_dir)
        check(rc == 0, "T17: no markers, only known failures -> exit 0")

        # ── ML-3A T18: go-suite-out.txt vacuous -> results-present guard -> exit 1
        _st_print("=== T18: go-suite-out.txt vacuous -> results-present guard -> exit 1 ===")
        write_list(BASE_ENTRIES)
        # go artifact has only [setup failed] — no --- FAIL: or --- PASS: lines
        write_artifacts(go="FAIL\tgithub.com/kgsaran/trackfw/internal/badpkg [setup failed]\n")
        # No marker: testing the vacuity guard path (independent of marker path)
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 1, "T18: go-suite-out.txt vacuous (no FAIL/PASS lines) -> exit 1")
        check("::error::" in ann.getvalue(), "T18 AC5: results-present guard called _err()")

        # ── ML-3A T19: Node.js load-failure marker + name IN list -> exit 0 (row 1) ──
        # broken.test.js is in BASE_ENTRIES as suite-load-failure; TAP_LOAD produces it
        # in obs_node_load. Marker is present but step-6 ratchet sees no new names.
        _st_print("=== T19: Node.js load-failure marker + name in list -> exit 0 (row 1) ===")
        markers_t19 = os.path.join(td, "markers_t19")
        os.makedirs(markers_t19, exist_ok=True)
        write_list(BASE_ENTRIES)
        write_artifacts()  # default TAP_ASSERT + TAP_LOAD; TAP_LOAD yields broken.test.js
        Path(os.path.join(markers_t19, "suite-load-failure.node.txt")).write_text(
            "  exitCode: 1", encoding="utf-8"
        )
        with _capture_annotations():
            rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_t19)
        check(rc == 0, "T19: Node load-failure marker, 'broken.test.js' in known list -> exit 0 (row 1)")

        # ── ML-3A T20: Node.js load-failure marker + name NOT in list -> exit 1, named ─
        # new_broken.test.js extracted from TAP (exitCode block), not in known_node_load
        # → step-6 ratchet fires naming it via _err(). With the sink, _err() goes to
        # the annotation capture buffer, not stdout. The assertion checks ann.getvalue().
        _st_print("=== T20: Node.js load-failure marker + name not in list -> exit 1, named (row 2) ===")
        markers_t20 = os.path.join(td, "markers_t20")
        os.makedirs(markers_t20, exist_ok=True)
        new_load_tap = (
            TAP_ASSERT
            + "not ok 2 - /runner/work/trackfw/tests/new_broken.test.js\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  exitCode: 1\n"
            "  ...\n"
        )
        write_list(BASE_ENTRIES)
        write_artifacts(tap=new_load_tap)
        Path(os.path.join(markers_t20, "suite-load-failure.node.txt")).write_text(
            "  exitCode: 1", encoding="utf-8"
        )
        with _capture_annotations() as ann_t20:
            with contextlib.redirect_stdout(io.StringIO()):
                rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_t20)
        check(
            rc == 1 and "new_broken.test.js" in ann_t20.getvalue(),
            "T20: Node load-failure 'new_broken.test.js' not in list -> exit 1, named in annotation (row 2)"
        )
        check("::error::" in ann_t20.getvalue(), "T20 AC5: step-6 ratchet called _err() with file name")

        # ── ML-3A T21: Node.js load-failure marker + obs_node_load empty -> exit 1, "sem nome"
        # TAP has only assertion failures (no exitCode block) → obs_node_load empty.
        # Marker exists → step-3b detects row-3 (D3 worst mode) → exit 1 with "sem nome".
        # TAP_ASSERT alone: has 'not ok' lines (so 5b vacuity doesn't fire) but no exitCode.
        _st_print("=== T21: Node.js load-failure marker + no name in TAP -> exit 1 (row 3) ===")
        markers_t21 = os.path.join(td, "markers_t21")
        os.makedirs(markers_t21, exist_ok=True)
        write_list(BASE_ENTRIES)
        write_artifacts(tap=TAP_ASSERT)  # assertion-only TAP; no exitCode block → obs_node_load = {}
        Path(os.path.join(markers_t21, "suite-load-failure.node.txt")).write_text(
            "  exitCode: 1", encoding="utf-8"
        )
        with _capture_annotations() as ann_t21:
            with contextlib.redirect_stdout(io.StringIO()):
                rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_t21)
        check(
            rc == 1 and "sem nome" in ann_t21.getvalue(),
            "T21: Node load-failure marker, obs_node_load empty -> exit 1, 'sem nome' in annotation (row 3)"
        )
        check("::error::" in ann_t21.getvalue(), "T21 AC5: step-3b row-3 called _err()")

        # ── ML-3A T22: Node.js zero-test-failure marker -> exit 1 (row 3, early check) ──
        # zero-test marker has no test name by construction ('# tests 0' + exit 0) → row 3
        # → step-3 early check fires immediately → exit 1.
        _st_print("=== T22: Node.js zero-test-failure marker -> exit 1 (row 3) ===")
        markers_t22 = os.path.join(td, "markers_t22")
        os.makedirs(markers_t22, exist_ok=True)
        write_list(BASE_ENTRIES)
        write_artifacts()
        Path(os.path.join(markers_t22, "zero-test-failure.node.txt")).write_text(
            "zero-test", encoding="utf-8"
        )
        with _capture_annotations() as ann:
            rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_t22)
        check(rc == 1, "T22: Node.js zero-test-failure marker -> exit 1 (row 3, early check)")
        check("::error::" in ann.getvalue(), "T22 AC5: zero-test early check called _err()")

        # ── Sumário T23: cancel case — one class +1 NOVO, another -1 resolvido ──────────
        # Asserts: set-based detection exposes the cancellation that the count-based total
        # masks. CI case: run 34547480139, Node-assert 11/10, Python 12/13, total 38/38.
        # Fixture: 2 Node-assert obs (known + new_name), 0 Python obs, 1 Python known,
        # Go and Node-load balanced. Total obs == total known — the cancellation is exact.
        _st_print("=== T23: cancel case (Node +1 NOVO, Python -1 resolvido, total balanced) "
                  "-> DESEQUILIBRIO on first line ===")
        entries_t23 = [
            {"name": "TestFoo",              "runtime": "go",     "class": "assertion"},
            {"name": "known_node_assert",    "runtime": "node",   "class": "assertion"},
            {"name": "broken.test.js",       "runtime": "node",   "class": "suite-load-failure"},
            {"name": "test_foo.py::test_bar","runtime": "python", "class": "assertion"},
        ]
        write_list(entries_t23)
        cancel_tap = (
            # known_node_assert still fails (remains in list)
            "not ok 1 - known_node_assert\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  ...\n"
            # new_node_assert is NEW — not in list
            "not ok 2 - new_node_assert\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  ...\n"
            # broken.test.js still load-fails (suite-load-failure, still in list)
            "not ok 3 - /runner/tests/broken.test.js\n"
            "  ---\n"
            "  exitCode: 1\n"
            "  ...\n"
        )
        # Python: 0 failures observed → test_foo.py::test_bar "resolved" (warning only)
        write_artifacts(tap=cancel_tap, py="PASSED pypi/tests/test_foo.py::test_bar\n")
        buf = io.StringIO()
        with _capture_annotations(), contextlib.redirect_stdout(buf):
            run_check(list_path, go_path, tap_path, py_path)
        out_t23 = buf.getvalue()
        summary_line_t23 = next(
            (l for l in out_t23.splitlines() if l.startswith("ML-2A/2B:")), ""
        )
        check(
            "DESEQUIL" in summary_line_t23
            and "NOVO" in summary_line_t23
            and "resolvido" in summary_line_t23,
            "T23: cancel case -> 'DESEQUILIBRIO POR CLASSE' + NOVO + resolvido on first summary line",
        )

        # ── Sumário T24: all balanced -> no imbalance markers (counter-arm) ───────────
        # Asserts: when every observed name matches a known name and vice versa, the
        # first summary line is clean. Uses T1's fixture (BASE_ENTRIES + default artifacts).
        # Negative assertion kept live by the positive arm of T23/T25 running in the same
        # session (same fixture writer / same redirect pattern) — the channel is proven live.
        _st_print("=== T24: all balanced -> no DESEQUILIBRIO on summary line (counter-arm) ===")
        write_list(BASE_ENTRIES)
        write_artifacts()
        buf = io.StringIO()
        with _capture_annotations(), contextlib.redirect_stdout(buf):
            run_check(list_path, go_path, tap_path, py_path)
        out_t24 = buf.getvalue()
        summary_line_t24 = next(
            (l for l in out_t24.splitlines() if l.startswith("ML-2A/2B:")), ""
        )
        check(
            "DESEQUIL" not in summary_line_t24
            and "NOVO" not in summary_line_t24
            and "resolvido" not in summary_line_t24,
            "T24: all classes balanced -> no 'DESEQUILIBRIO', 'NOVO', or 'resolvido' on summary line",
        )

        # ── Sumário T25: one class surplus, others balanced ───────────────────────────
        # Asserts: imbalance in a single class is flagged independently of other classes.
        # Node-assert: 2 obs (known + new_one), 1 known → surplus = {new_one}.
        # Others: balanced.
        _st_print("=== T25: one class surplus, others balanced -> [+1 NOVO] on summary line ===")
        write_list(BASE_ENTRIES)
        single_surplus_tap = (
            # sample assertion test still fails (in list)
            "not ok 1 - sample assertion test\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  ...\n"
            # extra_new_assertion is NEW — not in list
            "not ok 2 - extra_new_assertion\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  ...\n"
            # broken.test.js still load-fails
            "not ok 3 - /runner/tests/broken.test.js\n"
            "  ---\n"
            "  exitCode: 1\n"
            "  ...\n"
        )
        write_artifacts(tap=single_surplus_tap)
        buf = io.StringIO()
        with _capture_annotations(), contextlib.redirect_stdout(buf):
            run_check(list_path, go_path, tap_path, py_path)
        out_t25 = buf.getvalue()
        summary_line_t25 = next(
            (l for l in out_t25.splitlines() if l.startswith("ML-2A/2B:")), ""
        )
        check(
            "DESEQUIL" in summary_line_t25
            and "[+1 NOVO]" in summary_line_t25,
            "T25: one class surplus -> 'DESEQUILIBRIO POR CLASSE' and '[+1 NOVO]' on summary line",
        )

        # ── Sumário T26: equal count but different names in one class ─────────────────
        # Asserts: set-based detection catches one-name replacement (surplus + resolved in
        # same class) even when obs count == known count. Count-based logic would see 2/2
        # and print clean; set-based sees surplus={NodeC} → '[+1 NOVO, -1 resolvido]' tag.
        # This arm separates the correct (set-based) fix from the plausible-wrong (count-based).
        _st_print("=== T26: equal count, different names in a class -> [+1 NOVO] on summary line ===")
        entries_t26 = [
            {"name": "TestFoo",              "runtime": "go",     "class": "assertion"},
            {"name": "NodeA",                "runtime": "node",   "class": "assertion"},
            {"name": "NodeB",                "runtime": "node",   "class": "assertion"},
            {"name": "broken.test.js",       "runtime": "node",   "class": "suite-load-failure"},
            {"name": "test_foo.py::test_bar","runtime": "python", "class": "assertion"},
        ]
        write_list(entries_t26)
        # obs: NodeA (known) + NodeC (new, replaces NodeB) → obs count = known count = 2
        replacement_tap = (
            "not ok 1 - NodeA\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  ...\n"
            "not ok 2 - NodeC\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  ...\n"
            "not ok 3 - /runner/tests/broken.test.js\n"
            "  ---\n"
            "  exitCode: 1\n"
            "  ...\n"
        )
        write_artifacts(tap=replacement_tap)
        buf = io.StringIO()
        with _capture_annotations(), contextlib.redirect_stdout(buf):
            run_check(list_path, go_path, tap_path, py_path)
        out_t26 = buf.getvalue()
        summary_line_t26 = next(
            (l for l in out_t26.splitlines() if l.startswith("ML-2A/2B:")), ""
        )
        check(
            "DESEQUIL" in summary_line_t26
            and "NOVO" in summary_line_t26,
            "T26: equal count (2/2) but different names -> set-based detection fires "
            "'DESEQUILIBRIO POR CLASSE' and 'NOVO' despite equal count (count-based would miss this)",
        )

        # ── AC4: production sink emits ::error:: (falsification in both directions) ──────
        # Direction 1 (fixture -> not annotate): proven structurally — all fixture
        # run_check() calls above are wrapped in _capture_annotations(), so no ::error::
        # can reach the real stdout during self-test (demonstrated by AC3 check below).
        #
        # Direction 2 (production -> annotates): _err() must route through _ANNOTATION_SINK
        # and _ANNOTATION_SINK must default to sys.stdout.
        #
        # Note: redirect_stdout(buf) replaces sys.stdout but NOT _ANNOTATION_SINK, which
        # was bound to the original sys.stdout at import time. To test _err() routing,
        # we swap _ANNOTATION_SINK directly — same mechanism as _capture_annotations().
        # T27a checks the default binding; T27b checks _err() routes through the sink.
        _st_print("=== T27: AC4 falsification (production direction) ===")

        # T27a: _ANNOTATION_SINK is sys.stdout in default state (outside any capture context)
        check(
            _ANNOTATION_SINK is sys.stdout,
            "T27a AC4: _ANNOTATION_SINK is sys.stdout in default state (no active capture)",
        )

        # T27b: _err() writes to _ANNOTATION_SINK, not a hardcoded stream
        # (if _err() ignored the sink and wrote directly to sys.stdout, this would still
        # pass — but T27a + T27b together prove the chain: sink=stdout + _err uses sink)
        with _capture_annotations() as ac4_buf:
            _err("sentinel-ac4")
        check(
            "::error::sentinel-ac4" in ac4_buf.getvalue(),
            "T27b AC4: _err() routes through _ANNOTATION_SINK -> '::error::sentinel-ac4' in sink",
        )

    _st_print(f"\nSelf-test summary: {n_pass} PASS, {n_fail} FAIL")
    return 0 if n_fail == 0 else 1


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(
        description="ML-2A ratchet + ML-2B removal-note enforcement + ML-3A load-failure verdict."
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run built-in falsification suite (no CI artifacts required).",
    )
    parser.add_argument("--list",       default=".github/windows-known-failures.json")
    parser.add_argument("--go-out",     default="")
    parser.add_argument("--node-tap",   default="")
    parser.add_argument("--python-out", default="")
    parser.add_argument(
        "--baseline",
        default="",
        help=(
            "Path to baseline known-failures JSON (e.g. from "
            "'git show origin/main:.github/windows-known-failures.json'). "
            "When provided, entries deleted from 'entries' without a corresponding "
            "'removed' record cause exit 1 (ML-2B D4 baseline check)."
        ),
    )
    parser.add_argument(
        "--load-markers-dir",
        default="",
        help=(
            "Directory where suite steps write marker files for suite-load-failure and "
            "zero-test events (ML-3A). Typically RUNNER_TEMP on Windows CI. If any "
            "marker file is found, ratchet exits 1 immediately (classe própria, ML-1A). "
            "Also enables the results-present vacuity guard."
        ),
    )
    args = parser.parse_args()

    if args.self_test:
        return run_self_test()

    if not args.go_out or not args.node_tap or not args.python_out:
        print(
            "error: --go-out, --node-tap and --python-out are required "
            "when not running --self-test.",
            file=sys.stderr,
        )
        return 2

    return run_check(
        args.list, args.go_out, args.node_tap, args.python_out,
        baseline_path=args.baseline,
        load_markers_dir=args.load_markers_dir,
    )


if __name__ == "__main__":
    sys.exit(main())
