#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
ML-2A ratchet: verifies that Windows CI failures match the known-failures list.

ADR: docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-
     tipo-de-evento-nunca-por-contagem.md

D1  — fails if a test name appears that is NOT in the known list (new failure).
D2  — vacuity guard: list empty or missing -> exit 1, naming the cause.
D4  — removal note field documented; format validated by ML-2B.
Warn — known entry not observed (fixed/renamed) -> ::warning::, never exit 1.

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
      --python-out <path/to/python-suite-out.txt>

Exit codes:
  0 — no new failures detected (warnings may have been emitted)
  1 — new failure(s) detected, or vacuity/artifact guard triggered
"""

from __future__ import annotations

import argparse
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

def load_known_list(path: str) -> list[dict]:
    """Load and validate the known-failures list.

    Raises SystemExit(1) on vacuity (missing file or zero entries).
    A missing or empty list is not 'no known failures' — it is a broken
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
            f"ML-2A vacuity guard: known-failures list at '{path}' has zero entries. "
            "An empty list passes every observation. "
            "With active Windows debt this is the worst failure mode."
        )
        raise SystemExit(1)
    return entries


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
# Extractors
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
# Ratchet check
# ---------------------------------------------------------------------------

def run_check(
    list_path: str,
    go_out: str,
    node_tap: str,
    python_out: str,
) -> int:
    """Run the ratchet check. Returns 0 (pass) or 1 (new failure(s) detected)."""

    # 1. Load known list (vacuity guard inside — raises SystemExit(1) if broken)
    known = load_known_list(list_path)

    # Build lookup sets by runtime+class
    known_go_assert   = {e['name'] for e in known if e['runtime'] == 'go'     and e['class'] == 'assertion'}
    known_node_assert = {e['name'] for e in known if e['runtime'] == 'node'   and e['class'] == 'assertion'}
    known_node_load   = {e['name'] for e in known if e['runtime'] == 'node'   and e['class'] == 'suite-load-failure'}
    known_py_assert   = {e['name'] for e in known if e['runtime'] == 'python' and e['class'] == 'assertion'}

    # 2. Require artifacts (observation-side vacuity guard)
    go_path  = require_artifact(go_out,     'go-suite-out.txt')
    tap_path = require_artifact(node_tap,   'node-suite.tap')
    py_path  = require_artifact(python_out, 'python-suite-out.txt')

    # 3. Extract observed failures
    obs_go = extract_go_failures(go_path)
    obs_node_assert, obs_node_load = extract_node_failures(tap_path)
    obs_py = extract_python_failures(py_path)

    has_new = False

    # 4. New failures NOT in the known list -> ::error:: + exit 1
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

    # 5. Known entries NOT observed (fixed/renamed) -> ::warning:: only, never exit 1
    #    Fixing a test must not break CI (that would make the ratchet a trap).
    for name in sorted(known_go_assert - obs_go):
        _warn(
            f"ML-2A ratchet: Go assertion '{name}' is in the known list but did NOT fail. "
            "Please remove from .github/windows-known-failures.json with a 'removal_note' "
            "field (ML-2B: corrected | renamed | no-longer-runs)."
        )

    for name in sorted(known_node_assert - obs_node_assert):
        _warn(
            f"ML-2A ratchet: Node.js assertion '{name}' is in the known list but did NOT fail. "
            "Please remove from .github/windows-known-failures.json with a 'removal_note' (ML-2B)."
        )

    for name in sorted(known_node_load - obs_node_load):
        _warn(
            f"ML-2A ratchet: Node.js suite-load-failure '{name}' is in the known list but did NOT fail. "
            "Please remove from .github/windows-known-failures.json with a 'removal_note' (ML-2B)."
        )

    for name in sorted(known_py_assert - obs_py):
        _warn(
            f"ML-2A ratchet: Python assertion '{name}' is in the known list but did NOT fail. "
            "Please remove from .github/windows-known-failures.json with a 'removal_note' (ML-2B)."
        )

    # 6. Informational summary — counts only (never used for decisions per ADR D1)
    total_obs   = len(obs_go) + len(obs_node_assert) + len(obs_node_load) + len(obs_py)
    total_known = len(known)
    print(f"ML-2A: {total_obs} observed / {total_known} known. "
          f"Go {len(obs_go)}/{len(known_go_assert)}, "
          f"Node-assert {len(obs_node_assert)}/{len(known_node_assert)}, "
          f"Node-load {len(obs_node_load)}/{len(known_node_load)}, "
          f"Python {len(obs_py)}/{len(known_py_assert)}.", flush=True)

    return 1 if has_new else 0


# ---------------------------------------------------------------------------
# Self-test (falsification in both directions + vacuity guards)
# ---------------------------------------------------------------------------

def run_self_test() -> int:
    """Run built-in falsification suite.

    Reconciliation (per project rule: each test/guard declares in one sentence
    what conclusion of this ML it asserts, confronted against the measurement):

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
        list_path  = os.path.join(td, "known.json")
        go_path    = os.path.join(td, "go-suite.txt")
        tap_path   = os.path.join(td, "node.tap")
        py_path    = os.path.join(td, "python.txt")

        # Canonical sample content matching real CI output format
        GO_FAIL    = "--- FAIL: TestFoo (0.01s)\n"
        GO_FAIL_2  = "--- FAIL: TestBar (0.01s)\n"
        TAP_ASSERT = (
            "not ok 1 - sample assertion test\n"
            "  ---\n"
            "  failureType: 'testCodeFailure'\n"
            "  code: 'ERR_ASSERTION'\n"
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

        BASE_ENTRIES = [
            {"name": "TestFoo",               "runtime": "go",     "class": "assertion"},
            {"name": "sample assertion test",  "runtime": "node",   "class": "assertion"},
            {"name": "broken.test.js",         "runtime": "node",   "class": "suite-load-failure"},
            {"name": "test_foo.py::test_bar",  "runtime": "python", "class": "assertion"},
        ]

        def write_list(entries: list[dict]) -> None:
            data = {"_meta": {"source": {"run_id": "self-test"}}, "entries": entries}
            Path(list_path).write_text(
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
            '{"entries": []}', encoding="utf-8"
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

    print(f"\nSelf-test summary: {n_pass} PASS, {n_fail} FAIL", flush=True)
    return 0 if n_fail == 0 else 1


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(
        description="ML-2A ratchet: Windows known-failures verifier."
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

    return run_check(args.list, args.go_out, args.node_tap, args.python_out)


if __name__ == "__main__":
    sys.exit(main())
