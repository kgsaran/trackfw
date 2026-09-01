"""
scaffold_doctor.py — scaffold artifact coverage for `trackfw doctor`
(ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-
com-propriedade-dada-pelo-caminho.md).

Mirrors internal/generators/scaffold_doctor.go (Go, canonical source of truth) and
npm/src/integrations/scaffold_doctor.js (Node.js).

All three runtimes cover the same artifact classes. What varies is described below.

  scripts/trackfw-validate.sh — SET-MEMBERSHIP (architect decision 2026-08-27)
    The three runtimes emit different bytes for this file: Go/Node produce a
    #!/usr/bin/env sh form with cfg-dependent build steps; Python produces a simpler
    #!/usr/bin/env bash form (_VALIDATE_SCRIPT_CONTENT). The divergence is pre-existing,
    intentional, and documented (docs/cli-parity.md,
    "validate.sh — pertencimento a conjunto (set-membership, escopado)").

    For this artifact ONLY, the doctor uses set-membership: the file is accepted if it
    matches ANY of the known runtime templates (Go/Node form for the project's cfg, OR
    Python's form). A file that matches NONE of the known forms continues to be accused —
    AC3 coverage is preserved. See _check_validate_script_artifact.

  CI workflow (.github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml)
    COVERED (ML-2C, REQ-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-
    honrando-trackfw-version). Python's `update` now includes `ci-workflow` in
    PROJECT_TARGET_IDS (pypi/trackfw/commands/update.py) whenever trackfw.yaml declares
    `ci: github-actions` or `ci: gitlab-ci`, so the doctor's remedy of `trackfw update` is
    now accurate for this runtime too. Only the path the project's `ci:` config selects is
    checked — the other one is never expected to exist. Absent `ci:` (or an unrecognized
    value) means neither path is expected, so nothing is checked (mirrors Go's/Node's own
    doctor: a project that never opted into a CI system is never accused of missing one).
    The former exclusion is gone from docs/cli-parity.md as of Wave 3 of the same REQ.

  Execute-bit checking (REQ-2026-08-28, AC2–AC5, AC10, AC11):
    The five scripts the generator writes with mode 0o755 are additionally checked for
    the owner-execute bit. The check uses (stat.st_mode & 0o100) != 0 (not == 0o755)
    so umask-narrowed modes (0o750, 0o700) are also accepted (AC10). Non-executable
    artifacts carry exec_bit=False and are never mode-checked (AC4/AC11). Content
    divergence takes precedence over mode (at most one finding per artifact). Content
    correct + bit missing → SCAFFOLD_WRONG_MODE (AC3 distinct state). On Windows
    (sys.platform == "win32") the execute bit is not representable on NTFS, so the
    mode check is suppressed entirely — AC5.

    Python's generator was already correct (open(...,'w', newline="\n") + os.chmod(0o755) is
    unconditional and ignores umask). No change needed to the Python write path.

  _current_platform is seeded from sys.platform at import time and can be overridden
  in tests via _set_platform_for_test to exercise the Windows guard (AC5 testability).
"""

from __future__ import annotations

import os
import sys
import tempfile
from typing import Any

try:
    import yaml as _yaml
    _YAML_AVAILABLE = True
except ImportError:
    _YAML_AVAILABLE = False

# Import the private constants directly — intentional: they are the single source of
# truth for each script's content, same principle as Go's exported constants in scaffold.go.
# Accessing private names (prefixed _) from the same package is acceptable here because
# this module is a sibling in the same generators/integrations boundary, not an external
# consumer; the alternative would be duplicating multi-kilobyte constants and accepting drift.
from trackfw.generators.init_gen import (
    _ATTENTION_SIGNAL_SH,
    _ATTENTION_CLEANUP_SH,
    _CREDENTIAL_GUARD_SH,
    _GIT_BRANCH_GUARD_SH,
    _VALIDATE_SCRIPT_CONTENT,
    GITHUB_ACTIONS_WORKFLOW_PATH,
    GITLAB_CI_WORKFLOW_PATH,
    build_github_actions_workflow_content,
    build_gitlab_ci_workflow_content,
    generate_claude_commands,
)
from trackfw.integrations.doctor import SCAFFOLD_DIVERGENT, SCAFFOLD_MISSING, SCAFFOLD_WRONG_MODE
from trackfw.commands.discover import (
    DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
    build_discover_github_actions_workflow_content,
)

# Path constants — mirror Go's exported constants and Node's module-level strings.
CLAUDE_COMMANDS_DIR_PATH = '.claude/commands/trackfw'

# _current_platform is seeded from sys.platform at import time.
# Tests override it via _set_platform_for_test to exercise the Windows guard (AC5).
_current_platform: str = sys.platform


def _set_platform_for_test(platform: str) -> Any:
    """Override _current_platform for unit tests. Returns a restore callable.

    Usage::

        restore = _set_platform_for_test('win32')
        try:
            ...assertions...
        finally:
            restore()
    """
    global _current_platform
    prev = _current_platform
    _current_platform = platform

    def _restore() -> None:
        global _current_platform
        _current_platform = prev

    return _restore


def _exec_bit_present(path: str) -> bool:
    """Return True if the file at path has the owner-execute bit set
    (stat.st_mode & 0o100 != 0). Returns False on any stat error.

    Uses bit mask rather than equality to 0o755 so that umask-narrowed modes
    like 0o750 or 0o700 are also accepted — AC10 of REQ-2026-08-28.
    """
    try:
        return bool(os.stat(path).st_mode & 0o100)
    except OSError:
        return False


def _load_project_config(project_root: str) -> dict[str, str | None]:
    """Read trackfw.yaml from project_root and return a cfg dict with the keys that
    _build_go_node_validate_script expects. Missing keys default to None.

    Mirrors Node's loadProjectConfig() and Go's loadUpdateConfig(). Uses the yaml
    library when available; falls back to a simple grep-based parser for environments
    where PyYAML/ruamel is not installed.
    """
    yaml_path = os.path.join(project_root, 'trackfw.yaml')
    try:
        with open(yaml_path, 'r', encoding='utf-8') as f:
            raw = f.read()
    except OSError:
        return {'backend': None, 'frontend': None, 'pkg_manager': None, 'ci': None}

    if _YAML_AVAILABLE:
        try:
            parsed = _yaml.safe_load(raw) or {}
        except Exception:
            parsed = {}
    else:
        # Minimal single-key grep fallback (covers the common scalar case).
        parsed = {}
        for line in raw.splitlines():
            for key in ('backend', 'frontend', 'pkg_manager', 'ci'):
                if line.strip().startswith(key + ':'):
                    val = line.split(':', 1)[1].strip().strip('"\'')
                    if val:
                        parsed[key] = val

    return {
        'backend': parsed.get('backend') or None,
        'frontend': parsed.get('frontend') or None,
        'pkg_manager': parsed.get('pkg_manager') or None,
        'ci': parsed.get('ci') or None,
    }


def _build_go_node_validate_script(cfg: dict[str, str | None]) -> str:
    """Return the content that Go's/Node's buildValidateScript(cfg) produces for the
    given config. This is a Python mirror of the Go/Node function — maintained here so
    the set-membership check in _check_validate_script_artifact can identify files
    generated by those runtimes without shelling out.

    IMPORTANT: this mirror is a parity liability. If Go's buildValidateScript changes,
    this function must be updated in the same commit. A unit test in each runtime
    (including this one) asserts that both known forms are accepted and a near-miss is
    refused, keeping drift detectable.

    Mirrors Go internal/generators/scaffold.go:buildValidateScript and
    Node npm/src/generators/init.js:buildValidateScript exactly.
    """
    backend = (cfg.get('backend') or '').strip()
    frontend = (cfg.get('frontend') or '').strip()
    pkg_manager = (cfg.get('pkg_manager') or '').strip()

    base = (
        '#!/usr/bin/env sh\n'
        '# trackfw governance gate — generated by trackfw init\n'
        'set -e\n'
        '\n'
        'echo "→ trackfw: validating governance..."\n'
        'trackfw validate\n'
        '\n'
    )

    if backend == 'go':
        base += 'echo "→ build check (go)..."\ngo build ./...\n'
    elif backend == 'java':
        base += 'echo "→ build check (maven)..."\nmvn compile -q\n'
    elif backend == 'node':
        pm = pkg_manager if pkg_manager else 'npm'
        base += f'echo "→ build check (node)..."\n{pm} run build\n'
    elif backend == 'python':
        base += (
            'echo "→ build check (python)..."\n'
            "python3 -c \"import pathlib, py_compile; "
            "[py_compile.compile(str(p), doraise=True) for p in pathlib.Path('.').rglob('*.py') "
            "if '.venv' not in p.parts and 'venv' not in p.parts]\"\n"
        )

    if frontend in ('react', 'vue', 'angular'):
        pm = pkg_manager if pkg_manager and pkg_manager != 'none' else 'npm'
        base += f'echo "→ frontend build check..."\n{pm} run build\n'

    base += '\necho "✓ all checks passed."\n'
    return base


def _scaffold_remedy(action: str, rel_path: str) -> str:
    """Returns a ready-to-copy remedy command for a scaffold finding.
    The message is neutral about blame direction (AC16): binary version stated
    but direction (project stale vs binary stale) left to the user to determine.
    """
    try:
        import importlib.metadata
        ver = importlib.metadata.version('trackfw')
    except Exception:
        ver = 'unknown'
    return (
        f"trackfw update   # {action} {rel_path}: content differs from the template "
        f"trackfw v{ver} generates; if this project was initialized with a newer binary, "
        f"update the binary instead"
    )


def _scaffold_wrong_mode_remedy(rel_path: str) -> str:
    """Returns a remedy command for the scaffold-wrong-mode finding.
    Names the missing execute bit explicitly to distinguish it from content divergence
    (AC3 of REQ-2026-08-28). The message is runtime-neutral so the parity check can
    diff Go, Node, and Python outputs byte-for-byte on this finding kind.
    """
    return (
        f"trackfw update   # restore execute bit on {rel_path}: content is correct but "
        f"the owner-execute bit is missing (mode 0755 required); trackfw update now "
        f"restores the mode unconditionally on existing files"
    )


def _check_validate_script_artifact(
    project_root: str,
    cfg: dict[str, str | None],
) -> dict[str, Any] | None:
    """Check scripts/trackfw-validate.sh using set-membership.

    Accepted if the on-disk content matches EITHER Go/Node's cfg-rendered form OR
    Python's fixed form (_VALIDATE_SCRIPT_CONTENT). A file that matches NONE of the
    known forms is reported as scaffold-divergent. A missing file is scaffold-missing.

    After content membership passes, the execute bit is checked (AC2/AC3) unless on
    Windows (AC5). Content divergence always takes precedence over mode.

    This is the ONLY artifact with set-membership; all others use single-template
    equality via _check_scaffold_artifact. See module docstring.
    """
    rel_path = 'scripts/trackfw-validate.sh'
    abs_path = os.path.join(project_root, rel_path)
    try:
        with open(abs_path, 'r', encoding='utf-8') as f:
            actual = f.read()
    except FileNotFoundError:
        return {
            'finding': SCAFFOLD_MISSING,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_remedy('restore', rel_path),
        }
    except OSError:
        return {
            'finding': SCAFFOLD_DIVERGENT,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_remedy('resync', rel_path),
        }

    go_node_form = _build_go_node_validate_script(cfg)
    if actual != _VALIDATE_SCRIPT_CONTENT and actual != go_node_form:
        # Content diverges — scaffold-divergent takes precedence.
        return {
            'finding': SCAFFOLD_DIVERGENT,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_remedy('resync', rel_path),
        }

    # Content accepted. Check the execute bit (AC2/AC3).
    # Suppressed on Windows where the bit is not representable (AC5).
    if _current_platform != 'win32' and not _exec_bit_present(abs_path):
        return {
            'finding': SCAFFOLD_WRONG_MODE,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_wrong_mode_remedy(rel_path),
        }

    return None


def _check_ci_workflow_artifact(
    project_root: str,
    cfg: dict[str, str | None],
) -> dict[str, Any] | None:
    """Checks the CI workflow declared by cfg["ci"] (ML-2C, REQ-2026-08-28).

    Only the path matching cfg["ci"] is checked — "github-actions" checks
    GITHUB_ACTIONS_WORKFLOW_PATH, "gitlab-ci" checks GITLAB_CI_WORKFLOW_PATH.
    Absent or unrecognized cfg["ci"] means this runtime expects neither path
    to exist, so no finding is produced either way (a project that never
    opted into a CI system is never accused of missing one — same principle
    as Go's and Node's own scaffold doctor). Content divergence takes
    precedence over the execute bit, but this artifact carries no execute
    bit expectation (workflow files are 0o644, exec_bit=False), matching
    _check_scaffold_artifact's default.
    """
    ci = cfg.get('ci')
    if ci == 'github-actions':
        rel_path = GITHUB_ACTIONS_WORKFLOW_PATH
        expected = build_github_actions_workflow_content(cfg)
    elif ci == 'gitlab-ci':
        rel_path = GITLAB_CI_WORKFLOW_PATH
        expected = build_gitlab_ci_workflow_content(cfg)
    else:
        return None

    return _check_scaffold_artifact(
        os.path.join(project_root, rel_path), rel_path, expected, True, False
    )


def _check_scaffold_artifact(
    abs_path: str,
    rel_path: str,
    expected: str,
    report_missing: bool,
    exec_bit: bool = False,
) -> dict[str, Any] | None:
    """Compare on-disk content at abs_path against expected.
    Returns a finding dict if divergent or (when report_missing=True) absent; else None.

    exec_bit controls whether the owner-execute bit is checked after content passes:
      True  → artifact written with 0o755; bit absence → SCAFFOLD_WRONG_MODE (AC2/AC3).
              Suppressed on Windows (AC5).
      False → artifact expected 0o644; never mode-checked (AC4/AC11).

    Content divergence takes precedence over mode (at most one finding per artifact).
    """
    try:
        with open(abs_path, 'r', encoding='utf-8') as f:
            actual = f.read()
    except FileNotFoundError:
        if not report_missing:
            return None
        return {
            'finding': SCAFFOLD_MISSING,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_remedy('restore', rel_path),
        }
    except OSError:
        # Unreadable artifact: report as divergent so the user is informed.
        return {
            'finding': SCAFFOLD_DIVERGENT,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_remedy('resync', rel_path),
        }
    if actual != expected:
        # Content diverges — takes precedence over any mode issue.
        return {
            'finding': SCAFFOLD_DIVERGENT,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_remedy('resync', rel_path),
        }

    # Content matches. Check the execute bit when required (AC2/AC3).
    # Suppressed on Windows where the bit is not representable (AC5).
    if exec_bit and _current_platform != 'win32' and not _exec_bit_present(abs_path):
        return {
            'finding': SCAFFOLD_WRONG_MODE,
            'claim': {'kind': '', 'item': '', 'target': '', 'surface': '', 'scope': ''},
            'destination': rel_path,
            'remedy': _scaffold_wrong_mode_remedy(rel_path),
        }

    return None


def _get_expected_claude_commands() -> dict[str, str]:
    """Return {filename: content} for all slash commands by generating them to a temp dir.

    Uses generate_claude_commands (which is idempotent/skip-if-exists — safe in a fresh
    temp dir) to derive the expected content from the same write path, ensuring structural
    impossibility of drift between the comparison and the generator.
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        generate_claude_commands(tmpdir)
        cmd_dir = os.path.join(tmpdir, CLAUDE_COMMANDS_DIR_PATH)
        result = {}
        try:
            for fname in os.listdir(cmd_dir):
                fpath = os.path.join(cmd_dir, fname)
                if os.path.isfile(fpath):
                    with open(fpath, 'r', encoding='utf-8') as f:
                        result[fname] = f.read()
        except OSError:
            pass
        return result


def run_scaffold_doctor(project_root: str | None = None) -> list[dict[str, Any]]:
    """Compare scaffold artifacts on disk against the templates the current Python CLI
    binary would generate, and return findings for any artifact that is divergent or missing.

    Scope: validate script, attention scripts, guards, slash commands (AC14 eligibility gate),
    and the CI workflow declared by cfg["ci"] (ML-2C, REQ-2026-08-28 — see
    _check_ci_workflow_artifact).

    Execute-bit checking (REQ-2026-08-28): the five executable scripts carry exec_bit=True;
    slash commands carry exec_bit=False — never accused of missing execute bit (AC11).
    Platform guard: mode check suppressed on Windows (AC5).

    @param project_root: absolute path to the project root (default: os.getcwd())
    @returns: list of finding dicts with keys finding, claim, destination, remedy
    """
    project_root = project_root or os.getcwd()

    # Eligibility: trackfw.yaml must exist. Without it there is no evidence this
    # is a trackfw project — return empty findings rather than flooding a non-trackfw repo.
    if not os.path.isfile(os.path.join(project_root, 'trackfw.yaml')):
        return []

    # Load project config for the validate.sh set-membership check.
    cfg = _load_project_config(project_root)

    findings: list[dict[str, Any]] = []

    # --- Scripts (always in scope when trackfw.yaml is present) ---
    # scripts/trackfw-validate.sh uses set-membership (Go/Node form OR Python form) —
    # see _check_validate_script_artifact and module docstring.
    # The four remaining scripts use single-template equality via _check_scaffold_artifact.
    # All five have exec_bit=True: the generator writes them with mode 0o755 (AC11).
    # The .lstrip('\n') mirrors the write path in _generate_attention_scripts and
    # _generate_credential_guard_script, which call .lstrip('\n') before writing.
    f = _check_validate_script_artifact(project_root, cfg)
    if f is not None:
        findings.append(f)

    static_scripts = [
        ('scripts/trackfw-attention-signal.sh', _ATTENTION_SIGNAL_SH.lstrip('\n'), True),
        ('scripts/trackfw-attention-cleanup.sh', _ATTENTION_CLEANUP_SH.lstrip('\n'), True),
        ('scripts/trackfw-credential-guard.sh', _CREDENTIAL_GUARD_SH.lstrip('\n'), True),
        ('scripts/trackfw-git-branch-guard.sh', _GIT_BRANCH_GUARD_SH.lstrip('\n'), True),
    ]
    for rel_path, expected, exec_bit in static_scripts:
        f = _check_scaffold_artifact(
            os.path.join(project_root, rel_path), rel_path, expected, True, exec_bit
        )
        if f is not None:
            findings.append(f)

    # --- CI workflow (ML-2C, REQ-2026-08-28) — only checked when cfg["ci"] is
    # "github-actions" or "gitlab-ci"; a no-op otherwise (see
    # _check_ci_workflow_artifact).
    f = _check_ci_workflow_artifact(project_root, cfg)
    if f is not None:
        findings.append(f)

    # --- Discover CI workflow (second, independent install mechanism, ML-2F) ---
    #
    # .github/workflows/trackfw-validate.yml (written by `trackfw discover --init`,
    # install_gates) is a separate artifact from GITHUB_ACTIONS_WORKFLOW_PATH above —
    # both can coexist in the same project (ADR-2026-08-28). Only checked when the file
    # is already present, mirroring the "conditional artifact" treatment above but using
    # presence-on-disk instead of cfg["ci"], because install_gates decides on its own
    # discovery signal (github-actions detection), not on trackfw.yaml's `ci:` key — a
    # project can have discover's workflow without cfg["ci"] ever being set.
    discover_workflow_path = os.path.join(project_root, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
    if os.path.isfile(discover_workflow_path):
        f = _check_scaffold_artifact(
            discover_workflow_path,
            DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
            build_discover_github_actions_workflow_content(),
            True,
            False,
        )
        if f is not None:
            findings.append(f)

    # --- Slash commands (AC14: only when the directory already exists) ---
    # The directory's presence is the eligibility signal: a project initialized via
    # `trackfw discover --init` (which does NOT write slash commands) will not have
    # this directory, so we report no missing commands. A project initialized via
    # `trackfw init` or `trackfw update` will have it, and any absent file is a finding.
    # Slash commands are markdown files (0o644) — exec_bit=False (AC11).
    claude_dir = os.path.join(project_root, CLAUDE_COMMANDS_DIR_PATH)
    if os.path.isdir(claude_dir):
        for filename, content in _get_expected_claude_commands().items():
            rel_path = f'{CLAUDE_COMMANDS_DIR_PATH}/{filename}'
            f = _check_scaffold_artifact(
                os.path.join(project_root, rel_path), rel_path, content, True, False
            )
            if f is not None:
                findings.append(f)

    # Deterministic output (AC7): sort by destination.
    findings.sort(key=lambda f: f['destination'])

    return findings
