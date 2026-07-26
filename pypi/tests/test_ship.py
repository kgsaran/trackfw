"""
test_ship.py — Testes para trackfw.ship.runner

Cobre os mesmos casos que Go e Node.js:
  - main / master: aborta com exit ≠ 0
  - Branch fora do padrão: aborta
  - Sem roadmap em wip: aborta com comandos de correção
  - Nada staged: aborta
  - Sem -m: aborta
  - --dry-run: nenhum comando de escrita vai para exec_git
  - Garantia de source: git add . / git add -A não aparecem em runner.py
  - is_ship_branch, is_git_write_cmd, normalize_branch_slug
"""

import os
import re
import pytest

from trackfw.ship.runner import (
    run_ship,
    is_ship_branch,
    is_git_write_cmd,
    normalize_branch_slug,
    GIT_WRITE_COMMANDS,
)


# ────────────────────────────────────────────────────────────────────────────
# helpers
# ────────────────────────────────────────────────────────────────────────────

class MockGit:
    """Captures calls and returns configured responses."""

    def __init__(self, branch='feat/default', staged='file.py'):
        self.branch = branch
        self.staged = staged
        self.calls = []

    def exec(self, args):
        self.calls.append(list(args))
        joined = ' '.join(args)

        if joined.startswith('symbolic-ref --short'):
            if not self.branch:
                return ('', 'not a git repo')
            return (self.branch, None)

        if joined.startswith('diff --cached --name-only'):
            return (self.staged, None)

        if '@{u}' in joined:
            return ('', 'no upstream')

        if joined.startswith('fetch'):
            return ('', 'offline')

        return ('', None)


def make_deps(branch='feat/my-feature', staged='file.py', violations=None):
    """Builds a dict of injectable dependencies."""
    git = MockGit(branch=branch, staged=staged)
    lines = []

    return {
        'git': git,
        'lines': lines,
        'exec_git': git.exec,
        'check_governance': lambda: violations if violations is not None else [],
        'writeln': lambda s: lines.append(s),
    }


def run(branch='feat/my-feature', staged='file.py', message='feat: test',
        dry_run=False, violations=None):
    """Convenience wrapper that calls run_ship with mock deps."""
    d = make_deps(branch=branch, staged=staged, violations=violations)
    code = run_ship(
        message=message,
        dry_run=dry_run,
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
    )
    return code, '\n'.join(d['lines']), d['git']


# ────────────────────────────────────────────────────────────────────────────
# Step 1 — Branch validation
# ────────────────────────────────────────────────────────────────────────────

def test_ship_main_branch_aborts():
    code, out, _ = run(branch='main')
    assert code == 1
    assert 'cannot run on' in out


def test_ship_master_branch_aborts():
    code, out, _ = run(branch='master')
    assert code == 1
    assert 'cannot run on' in out


@pytest.mark.parametrize('branch', ['feature/foo', 'hotfix/bar', 'docs/update', 'mybranch'])
def test_ship_wrong_pattern_aborts(branch):
    code, out, _ = run(branch=branch)
    assert code == 1
    assert 'does not match the required pattern' in out


@pytest.mark.parametrize('branch', ['feat/my-feature', 'fix/bug-123', 'refactor/clean-up'])
def test_ship_valid_branch_not_rejected_at_step1(branch):
    code, out, _ = run(branch=branch)
    assert 'does not match the required pattern' not in out
    assert 'cannot run on' not in out


# ────────────────────────────────────────────────────────────────────────────
# Step 2 — Governance
# ────────────────────────────────────────────────────────────────────────────

def test_ship_no_wip_roadmap_aborts_with_remediation():
    v = ['branch "feat/foo" is a feat/fix/refactor branch but no roadmap is in wip/']
    code, out, _ = run(branch='feat/foo', violations=v)
    assert code == 1
    assert 'governance check failed' in out
    assert 'trackfw req new' in out
    assert 'trackfw roadmap new' in out
    assert 'trackfw roadmap move' in out


# ────────────────────────────────────────────────────────────────────────────
# Step 4 — Nothing staged
# ────────────────────────────────────────────────────────────────────────────

def test_ship_nothing_staged_aborts():
    code, out, _ = run(staged='')
    assert code == 1
    assert 'nothing is staged' in out


# ────────────────────────────────────────────────────────────────────────────
# Step 5 — Missing commit message
# ────────────────────────────────────────────────────────────────────────────

def test_ship_no_message_aborts():
    code, out, _ = run(message='')
    assert code == 1
    assert 'commit message is required' in out


# ────────────────────────────────────────────────────────────────────────────
# --dry-run: no write commands sent to exec_git
# ────────────────────────────────────────────────────────────────────────────

def test_ship_dry_run_no_write_commands():
    d = make_deps(branch='feat/dry-run', staged='file.py')
    code = run_ship(
        message='feat(scope): dry run test',
        dry_run=True,
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
    )
    assert code == 0, f"dry-run should succeed, got {code}"

    for call in d['git'].calls:
        if call and call[0] in GIT_WRITE_COMMANDS:
            pytest.fail(f"dry-run must not send write command to exec_git: git {' '.join(call)}")

    out = '\n'.join(d['lines'])
    assert '[dry-run]' in out, "dry-run output must contain [dry-run] markers"


# ────────────────────────────────────────────────────────────────────────────
# Source-level guarantee: git add . / git add -A must not appear in runner.py
# ────────────────────────────────────────────────────────────────────────────

def test_ship_source_has_no_git_add_all():
    runner_path = os.path.join(os.path.dirname(__file__), '../trackfw/ship/runner.py')
    with open(runner_path) as f:
        src = f.read()

    # Check for argument patterns that would indicate a real git add call.
    # Single-quoted doc strings like 'git add .' are not matched.
    forbidden = ["'add', '.'", "'add', '-A'", '"add", "."', '"add", "-A"']
    for bad in forbidden:
        assert bad not in src, f"runner.py must not contain {bad}"


# ────────────────────────────────────────────────────────────────────────────
# Runtime guarantee: exec_git never receives add . or add -A
# ────────────────────────────────────────────────────────────────────────────

def test_ship_exec_never_receives_git_add_all():
    d = make_deps(branch='feat/safe', staged='file.py')
    run_ship(
        message='feat: safe',
        dry_run=True,
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
    )

    for call in d['git'].calls:
        if len(call) >= 2 and call[0] == 'add' and call[1] in ('.', '-A'):
            pytest.fail(f"exec_git received forbidden call: git {' '.join(call)}")


# ────────────────────────────────────────────────────────────────────────────
# is_ship_branch unit tests
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.parametrize('branch', ['feat/foo', 'feat/a-very-long-slug', 'fix/123', 'refactor/clean-up'])
def test_is_ship_branch_valid(branch):
    assert is_ship_branch(branch), f"{branch} should be valid"


@pytest.mark.parametrize('branch', ['main', 'master', 'feature/foo', 'hotfix/bar', 'feat/', 'refactor/'])
def test_is_ship_branch_invalid(branch):
    assert not is_ship_branch(branch), f"{branch} should be invalid"


# ────────────────────────────────────────────────────────────────────────────
# is_git_write_cmd unit tests
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.parametrize('args', [
    ['commit', '-m', 'msg'],
    ['push', 'origin', 'feat/foo'],
    ['push', '-u', 'origin', 'feat/foo'],
    ['fetch', 'origin', '--prune'],
])
def test_is_git_write_cmd_writes(args):
    assert is_git_write_cmd(args), f"{args} should be a write command"


@pytest.mark.parametrize('args', [
    ['status', '--short'],
    ['diff', '--cached', '--stat'],
    ['branch', '-r', '--no-merged'],
    ['symbolic-ref', '--short', 'HEAD'],
    ['log', '-1'],
])
def test_is_git_write_cmd_reads(args):
    assert not is_git_write_cmd(args), f"{args} should be read-only"


# ────────────────────────────────────────────────────────────────────────────
# normalize_branch_slug unit tests
# ────────────────────────────────────────────────────────────────────────────

def test_normalize_branch_slug():
    assert normalize_branch_slug('my-feature') == 'my-feature'
    assert normalize_branch_slug('My Feature') == 'my-feature'
    assert normalize_branch_slug('foo_bar.baz') == 'foo-bar-baz'
    assert normalize_branch_slug('ABC123') == 'abc123'
