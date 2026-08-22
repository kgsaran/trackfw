"""
test_push.py — Unit tests for trackfw push (Python)

Covers the same 5 cases as Go (internal/commands/push_test.go):
  1. No upstream → -u is present in push args
  2. With upstream → -u is absent from push args
  3. Branch `main` blocked
  4. Governance absent in feat/
  5. Exemption in chore/

Follows the style and dependency-injection mechanisms of test_ship.py.
"""

import io
import pytest

from trackfw.push.runner import run_push


# ────────────────────────────────────────────────────────────────────────────
# helpers
# ────────────────────────────────────────────────────────────────────────────


class MockPushGit:
    """Captures calls and returns configured responses for push tests."""

    def __init__(self, branch='feat/my-feature', has_upstream=False):
        self.branch = branch
        self.has_upstream = has_upstream
        self.calls = []

    def exec(self, args):
        self.calls.append(list(args))
        joined = ' '.join(args)

        if joined.startswith('symbolic-ref --short'):
            if not self.branch:
                return ('', 'not a git repo')
            return (self.branch, None)

        if '@{u}' in joined:
            if self.has_upstream:
                return (f'origin/{self.branch}', None)
            return ('', 'no upstream')

        if joined.startswith('remote get-url'):
            return ('', 'no remote')

        if joined.startswith('fetch'):
            # Non-blocking — squash-merge step skips on error.
            return ('', 'offline')

        if joined.startswith('branch -r'):
            return ('', None)

        if joined.startswith('push'):
            return ('', None)

        return ('', None)


def make_deps(branch='feat/my-feature', has_upstream=False, violations=None):
    """Builds injectable dependencies for run_push tests."""
    git = MockPushGit(branch=branch, has_upstream=has_upstream)
    out = io.StringIO()
    err_out = io.StringIO()

    return {
        'git': git,
        'exec_git': git.exec,
        'check_governance': lambda: violations if violations is not None else [],
        'out': out,
        'err_out': err_out,
    }


def run(branch='feat/my-feature', has_upstream=False, violations=None):
    """Convenience wrapper: calls run_push and returns (exit_code, combined_output, git_mock)."""
    d = make_deps(branch=branch, has_upstream=has_upstream, violations=violations)
    code = run_push(
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        out=d['out'],
        err_out=d['err_out'],
    )
    combined = d['out'].getvalue() + d['err_out'].getvalue()
    return code, combined, d['git']


# ────────────────────────────────────────────────────────────────────────────
# Case 3: main/master branch blocked unconditionally
# ────────────────────────────────────────────────────────────────────────────


def test_push_main_branch_aborts():
    code, out, _ = run(branch='main')
    assert code == 1
    assert 'cannot run on' in out


def test_push_master_branch_aborts():
    code, out, _ = run(branch='master')
    assert code == 1
    assert 'cannot run on' in out


# ────────────────────────────────────────────────────────────────────────────
# Case 4: governance absent in feat/ → blocked
# ────────────────────────────────────────────────────────────────────────────


def test_push_feat_branch_no_roadmap_aborts():
    violations = ['branch "feat/my-feature" is a feat/fix/refactor branch but no roadmap is in wip/']
    code, out, _ = run(branch='feat/my-feature', violations=violations)
    assert code == 1
    assert 'governance check failed' in out
    assert 'trackfw req new' in out
    assert 'lenient' in out, 'output must mention lenient mode so users understand why validate passes but push aborts'


# ────────────────────────────────────────────────────────────────────────────
# Case 5: chore/ and docs/ branches exempt from governance
# ────────────────────────────────────────────────────────────────────────────


def test_push_chore_branch_governance_skipped():
    # violations would fail if governance were checked
    violations = ['would fail if checked']
    code, out, _ = run(branch='chore/update-deps', violations=violations)
    assert code == 0, 'chore branch must succeed despite governance violations being present'
    assert 'governance check failed' not in out, 'governance must not be checked for chore/ branch'


def test_push_docs_branch_governance_skipped():
    violations = ['would fail if checked']
    code, out, _ = run(branch='docs/update-readme', violations=violations)
    assert code == 0, 'docs branch must succeed despite governance violations being present'
    assert 'governance check failed' not in out, 'governance must not be checked for docs/ branch'


# ────────────────────────────────────────────────────────────────────────────
# Case 1: no upstream → push args include -u
# ────────────────────────────────────────────────────────────────────────────


def test_push_no_upstream_adds_dash_u():
    # has_upstream=False → _build_push_args returns ['push', '-u', 'origin', branch]
    code, _, git = run(branch='feat/my-feature', has_upstream=False)
    assert code == 0
    push_call = next((c for c in git.calls if c and c[0] == 'push'), None)
    assert push_call is not None, 'a push call must have been made'
    assert '-u' in push_call, f'push args must include -u when no upstream is set; got: {push_call}'
    assert 'origin' in push_call, 'push args must include origin'
    assert 'feat/my-feature' in push_call, 'push args must include branch name'


# ────────────────────────────────────────────────────────────────────────────
# Case 2: with upstream → push args do NOT include -u
# ────────────────────────────────────────────────────────────────────────────


def test_push_with_upstream_no_dash_u():
    # has_upstream=True → _build_push_args returns ['push', 'origin', branch]
    code, _, git = run(branch='feat/my-feature', has_upstream=True)
    assert code == 0
    push_call = next((c for c in git.calls if c and c[0] == 'push'), None)
    assert push_call is not None, 'a push call must have been made'
    assert '-u' not in push_call, f'push args must NOT include -u when upstream exists; got: {push_call}'
    assert 'origin' in push_call, 'push args must include origin'
    assert 'feat/my-feature' in push_call, 'push args must include branch name'


# ────────────────────────────────────────────────────────────────────────────
# NeverCommits — push must never call git commit (ML-4A)
# ────────────────────────────────────────────────────────────────────────────


def test_push_never_calls_git_commit():
    # chore/ branch: no governance check, dry-run keeps it safe
    code, _, git = run(branch='chore/update-deps')
    assert code == 0
    commit_calls = [c for c in git.calls if c and c[0] == 'commit']
    assert commit_calls == [], f'push must never call git commit, but found: {commit_calls}'


# ────────────────────────────────────────────────────────────────────────────
# GovernanceMessage — says "trackfw push", not "trackfw ship" (ML-4A)
# ────────────────────────────────────────────────────────────────────────────


def test_push_governance_message_says_push_not_ship():
    violations = ['no roadmap found in wip/ nor done/']
    code, out, _ = run(branch='feat/orphan', violations=violations)
    assert 'trackfw push' in out, f'governance message must contain "trackfw push"; got: {out!r}'
    assert 'trackfw ship' not in out, f'governance message must NOT contain "trackfw ship"; got: {out!r}'
