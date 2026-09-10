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

def _err(msg: str) -> None:
    print(f"::error::{msg}", flush=True)


def _warn(msg: str) -> None:
    print(f"::warning::{msg}", flush=True)


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
    known_node_assert = {e['name'] for e in known if e['runtime'] == 'node'   and e['class'] == 'assertion'}
    known_node_load   = {e['name'] for e in known if e['runtime'] == 'node'   and e['class'] == 'suite-load-failure'}
    known_py_assert   = {e['name'] for e in known if e['runtime'] == 'python' and e['class'] == 'assertion'}

    # 2. Require artifacts (observation-side vacuity guard — file-existence)
    go_path  = require_artifact(go_out,     'go-suite-out.txt')
    tap_path = require_artifact(node_tap,   'node-suite.tap')
    py_path  = require_artifact(python_out, 'python-suite-out.txt')

    # 3. ML-3A: suite-load-failure and zero-test marker check (classe própria, ML-1A).
    #    Suite steps write marker files to load_markers_dir when they detect load failures
    #    or zero-test events BEFORE exiting. Step-level continue-on-error absorbs the exit
    #    code but not the marker file. The ratchet reads the markers here and exits 1 if any
    #    exist — two distinct states, two distinct messages (vault note: dois-estados-um-observable).
    #    "Não consegui procurar → fatal, nunca aviso."
    if load_markers_dir:
        load_fail = False
        marker_specs = [
            ("suite-load-failure.go.txt",     "Go suite-load-failure"),
            ("suite-load-failure.node.txt",   "Node.js suite-load-failure"),
            ("suite-load-failure.python.txt", "Python suite-load-failure"),
            ("zero-test-failure.node.txt",    "Node.js zero-test-failure"),
            ("zero-test-failure.python.txt",  "Python zero-test-failure"),
        ]
        for fname, label in marker_specs:
            marker = Path(load_markers_dir) / fname
            if marker.exists():
                content = marker.read_text(encoding="utf-8").strip()
                _err(
                    f"ML-3A: {label} — suíte não carregou ou zero testes (classe própria, ML-1A). "
                    f"O ratchet de nomes não captura este evento por nome; o árbitro reprova diretamente. "
                    f"Detalhe: {content}"
                )
                load_fail = True
        if load_fail:
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

    # 6. New failures NOT in the known list -> ::error:: + exit 1
    for name in sorted(obs_go - known_go_assert):
        _err(
            f"ML-2A ratchet: NEW Go assertion failure not in known list: '{name}'. "
            "Add to .github/windows-known-failures.json (with source run id) or fix the test."
        )
        has_new = True

    for name in sorted(obs_node_assert - known_node_assert):
        _err(
            f"ML-2A ratchet: NEW Node.js assertion failure not in known list: '{name}'. "
            "Add to .github/windows-known-failures.json or fix the test."
        )
        has_new = True

    for name in sorted(obs_node_load - known_node_load):
        _err(
            f"ML-2A ratchet: NEW Node.js suite-load-failure not in known list: '{name}'. "
            "Add to .github/windows-known-failures.json or fix the suite."
        )
        has_new = True

    for name in sorted(obs_py - known_py_assert):
        _err(
            f"ML-2A ratchet: NEW Python assertion failure not in known list: '{name}'. "
            "Add to .github/windows-known-failures.json or fix the test."
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

    # 10. Informational summary — counts only (never used for decisions per ADR D1)
    total_obs   = len(obs_go) + len(obs_node_assert) + len(obs_node_load) + len(obs_py)
    total_known = len(known)
    total_removed = len(removed)
    print(
        f"ML-2A/2B: {total_obs} observed / {total_known} active / {total_removed} removed. "
        f"Go {len(obs_go)}/{len(known_go_assert)}, "
        f"Node-assert {len(obs_node_assert)}/{len(known_node_assert)}, "
        f"Node-load {len(obs_node_load)}/{len(known_node_load)}, "
        f"Python {len(obs_py)}/{len(known_py_assert)}.",
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

    ── ML-3A (T16–T18) ─────────────────────────────────────────────────────────

    T16 — 'Go suite-load-failure marker present' -> exit 1 (row 4 "reprova" arm)
          Asserts: ML-3A row 4 — when a suite step writes a load-failure marker file
          (classe própria, ML-1A), the ratchet exits 1 even if all observed test names
          are in the known list. Step-level continue-on-error absorbs the exit code but
          not the marker; the marker is the signal that reaches the judge.
          [SYNTHETIC: marker file created in temp dir; no real Go compilation failure]

    T17 — 'no load-failure markers, only known failures' -> exit 0 (row 4 counter-arm)
          Asserts: the marker mechanism (ML-3A) does not block the normal path (row 1);
          the T16 guard fires only on marker presence, not always. Without this arm, a
          verdict that always fails looks identical to a correctly failing verdict.
          [SYNTHETIC: markers_dir exists but is empty]

    T18 — 'go-suite-out.txt present but vacuous (no FAIL/PASS lines)' -> exit 1
          Asserts: ML-3A results-present vacuity guard (vault note: "não consegui
          procurar → fatal, nunca aviso") — go-suite-out.txt containing only a
          '[setup failed]' line (no test result lines) means the suite did not produce
          results; the guard fires before the ratchet compares names and emits spurious
          "not observed" warnings for all 14 known Go entries.
          [SYNTHETIC: go-suite-out.txt written with only a [setup failed] line]
    """
    n_pass = 0
    n_fail = 0

    def check(cond: bool, description: str) -> None:
        nonlocal n_pass, n_fail
        if cond:
            print(f"  SELF-TEST PASS: {description}", flush=True)
            n_pass += 1
        else:
            print(f"  SELF-TEST FAIL: {description}", file=sys.stderr, flush=True)
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
        print("=== T1: all observed match known -> exit 0 ===", flush=True)
        write_list(BASE_ENTRIES)
        write_artifacts()
        rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T1: all known observed -> exit 0")

        # ── T2: new Go failure not in list ────────────────────────────────────
        print("=== T2: new Go failure not in list -> exit 1 ===", flush=True)
        write_list(BASE_ENTRIES)
        write_artifacts(go=GO_FAIL + GO_FAIL_2)  # TestBar is not in list
        rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 1, "T2: new Go failure -> exit 1")

        # ── T3: known entry not observed -> warning, exit 0 ──────────────────
        print("=== T3: known entry missing from observed -> warning, exit 0 ===", flush=True)
        extra = BASE_ENTRIES + [
            {"name": "TestKnownButFixed", "runtime": "go", "class": "assertion"}
        ]
        write_list(extra)
        write_artifacts()  # TestKnownButFixed absent from go output
        rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T3: known entry not observed -> warning only, exit 0")

        # ── T4: empty list -> vacuity guard -> SystemExit(1) ─────────────────
        print("=== T4: empty list -> vacuity guard -> SystemExit(1) ===", flush=True)
        Path(list_path).write_text(
            '{"entries": [], "removed": []}', encoding="utf-8"
        )
        write_artifacts()
        fired = False
        try:
            run_check(list_path, go_path, tap_path, py_path)
        except SystemExit as e:
            fired = (e.code == 1)
        check(fired, "T4: empty list -> SystemExit(1)")

        # ── T5: list file missing -> vacuity guard -> SystemExit(1) ──────────
        print("=== T5: missing list file -> vacuity guard -> SystemExit(1) ===", flush=True)
        if os.path.exists(list_path):
            os.unlink(list_path)
        write_artifacts()
        fired = False
        try:
            run_check(list_path, go_path, tap_path, py_path)
        except SystemExit as e:
            fired = (e.code == 1)
        check(fired, "T5: missing list -> SystemExit(1)")

        # ── T6: artifact missing -> observation-side vacuity -> SystemExit(1) ─
        print("=== T6: missing go artifact -> SystemExit(1) ===", flush=True)
        write_list(BASE_ENTRIES)
        write_artifacts()
        os.unlink(go_path)
        fired = False
        try:
            run_check(list_path, go_path, tap_path, py_path)
        except SystemExit as e:
            fired = (e.code == 1)
        check(fired, "T6: missing artifact -> SystemExit(1)")

        # ── T7: Node suite-load-failure with Windows full path ────────────────
        print("=== T7: Node suite-load with Windows path -> basename match ===", flush=True)
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
        rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T7: Windows path in TAP -> basename 'broken.test.js' matches known entry")

        # ── T8: Python class method with Windows backslash path ───────────────
        print("=== T8: Python class method + Windows backslash -> normalized ===", flush=True)
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
        rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T8: Windows backslash Python path normalized -> class method matches")

        # ── T9: non-ASCII name round-trip ─────────────────────────────────────
        print("=== T9: non-ASCII name (em-dash, accents) round-trip ===", flush=True)
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
        rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 0, "T9: non-ASCII test name survives JSON->file->extract round-trip")

        # ── ML-2B T10: baseline entry deleted without removed record -> exit 1 ─
        # Asserts: entry in baseline, absent from current entries, NOT in removed
        # → baseline check fires. The test passes (appears as '--- PASS: TestFoo')
        # so ML-2A does NOT fire for a new failure; only baseline check fires.
        print("=== T10: baseline entry deleted, no removed record -> exit 1 ===", flush=True)
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        write_list(active_without_testfoo, [])  # TestFoo gone, removed is empty
        write_baseline(BASE_ENTRIES)             # TestFoo was in baseline
        write_artifacts(go=GO_PASS)              # TestFoo passes now (not in fail output)
        rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T10: baseline entry deleted without removed record -> exit 1")

        # ── ML-2B T11: removed entry without removal_note -> exit 1 ──────────
        # Asserts: an entry in 'removed' without 'removal_note' must cause exit 1.
        # This is the intra-file validation arm of D4.
        print("=== T11: removed entry without removal_note -> exit 1 ===", flush=True)
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_no_note = [{"name": "TestFoo", "runtime": "go", "class": "assertion"}]
        write_list(active_without_testfoo, removed_no_note)
        write_baseline(BASE_ENTRIES)
        write_artifacts(go=GO_PASS)
        rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T11: removed entry without removal_note -> exit 1")

        # ── ML-2B T12: corrected claim but test not in PASS output -> exit 1 ──
        # Asserts: measured discriminant (Go -v '--- PASS:' present for corrected,
        # absent for no-longer-runs) — non-vacuous pass set without test name means
        # the corrected claim cannot be verified.
        print("=== T12: removal_note=corrected but test not in PASS output -> exit 1 ===", flush=True)
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_corrected = [{
            "name": "TestFoo", "runtime": "go", "class": "assertion",
            "removal_note": "corrected",
        }]
        write_list(active_without_testfoo, removed_corrected)
        write_baseline(BASE_ENTRIES)
        # GO_OTHER_PASS: non-vacuous (has a PASS line) but TestFoo is NOT there
        write_artifacts(go=GO_OTHER_PASS)
        rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T12: corrected claim without TestFoo in PASS output -> exit 1")

        # ── ML-2B T13: renamed without renamed_to in active entries -> exit 1 ─
        # Asserts: if removal_note=renamed and renamed_to is absent from active entries,
        # the checker fires. The fixture has TestFooRenamed passing (not in fails),
        # so ML-2A cannot be the cause — only the renamed_to validation fires.
        print("=== T13: removal_note=renamed, renamed_to not in entries -> exit 1 ===", flush=True)
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_renamed = [{
            "name": "TestFoo", "runtime": "go", "class": "assertion",
            "removal_note": "renamed", "renamed_to": "TestFooRenamed",
        }]
        write_list(active_without_testfoo, removed_renamed)
        write_baseline(BASE_ENTRIES)
        # TestFooRenamed passes (not failing), TestFoo passes — neither in fail output
        write_artifacts(go="--- PASS: TestFooRenamed (0.01s)\n")
        rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 1, "T13: renamed_to='TestFooRenamed' not in active entries -> exit 1")

        # ── ML-2B T14: valid removal (corrected + TestFoo in PASS output) -> exit 0
        # Asserts: the ratchet must not block all removals — a well-formed 'corrected'
        # entry with the test appearing in pass output exits cleanly (vacuity guard
        # for the removal mechanism itself).
        print("=== T14: valid removal (corrected + test in PASS output) -> exit 0 ===", flush=True)
        active_without_testfoo = [e for e in BASE_ENTRIES if e["name"] != "TestFoo"]
        removed_valid = [{
            "name": "TestFoo", "runtime": "go", "class": "assertion",
            "removal_note": "corrected",
        }]
        write_list(active_without_testfoo, removed_valid)
        write_baseline(BASE_ENTRIES)
        write_artifacts(go=GO_PASS)  # TestFoo appears as PASS
        rc = run_check(list_path, go_path, tap_path, py_path, baseline_path=baseline_path)
        check(rc == 0, "T14: valid corrected removal with TestFoo in PASS output -> exit 0")

        # ── ML-2B T15: baseline confirmation line emitted with baseline, absent without
        # Asserts: check_baseline_deletions emits 'ML-2B D4 (baseline): N entrada(s)
        # comparadas' when baseline is provided; line is absent when baseline_path=''.
        # Fixes two-states-one-observable: "clean" and "skipped" were indistinguishable.
        print("=== T15: baseline positive confirmation line -> present with baseline, absent without ===", flush=True)
        write_list(BASE_ENTRIES, [])   # all entries in active, none removed
        write_baseline(BASE_ENTRIES)   # baseline matches current entries exactly

        # T15a: baseline provided → confirmation line appears
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            result = check_baseline_deletions(baseline_path, BASE_ENTRIES, [])
        out = buf.getvalue()
        check(
            result is True and "ML-2B D4 (baseline):" in out and "comparadas" in out,
            "T15a: baseline clean → 'ML-2B D4 (baseline): N entrada(s) comparadas' emitted",
        )

        # T15b: empty baseline_path → confirmation line absent (baseline skipped)
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            result = check_baseline_deletions("", BASE_ENTRIES, [])
        out = buf.getvalue()
        check(
            result is True and "comparadas" not in out,
            "T15b: baseline skipped (empty path) → no confirmation line",
        )

        # ── ML-3A T16: Go suite-load-failure marker present -> exit 1 ──────────
        print("=== T16: Go suite-load-failure marker present -> exit 1 ===", flush=True)
        markers_dir = os.path.join(td, "markers")
        os.makedirs(markers_dir, exist_ok=True)
        write_list(BASE_ENTRIES)
        write_artifacts()  # normal artifacts — no new test names outside known list
        Path(os.path.join(markers_dir, "suite-load-failure.go.txt")).write_text(
            "FAIL\tgithub.com/kgsaran/trackfw/internal/badpkg [setup failed]",
            encoding="utf-8",
        )
        rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_dir)
        check(rc == 1, "T16: Go suite-load-failure marker -> exit 1")
        # Remove marker for T17
        os.remove(os.path.join(markers_dir, "suite-load-failure.go.txt"))

        # ── ML-3A T17: no markers + known-only failures -> exit 0 (row 4 counter-arm)
        print("=== T17: no load-failure markers -> exit 0 (row 4 counter-arm) ===", flush=True)
        write_list(BASE_ENTRIES)
        write_artifacts()
        rc = run_check(list_path, go_path, tap_path, py_path, load_markers_dir=markers_dir)
        check(rc == 0, "T17: no markers, only known failures -> exit 0")

        # ── ML-3A T18: go-suite-out.txt vacuous -> results-present guard -> exit 1
        print("=== T18: go-suite-out.txt vacuous -> results-present guard -> exit 1 ===", flush=True)
        write_list(BASE_ENTRIES)
        # go artifact has only [setup failed] — no --- FAIL: or --- PASS: lines
        write_artifacts(go="FAIL\tgithub.com/kgsaran/trackfw/internal/badpkg [setup failed]\n")
        # No marker: testing the vacuity guard path (independent of marker path)
        rc = run_check(list_path, go_path, tap_path, py_path)
        check(rc == 1, "T18: go-suite-out.txt vacuous (no FAIL/PASS lines) -> exit 1")

    print(f"\nSelf-test summary: {n_pass} PASS, {n_fail} FAIL", flush=True)
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
