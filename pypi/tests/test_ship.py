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
import subprocess
import tempfile
import pytest

from trackfw.ship.runner import (
    run_ship,
    is_ship_branch,
    is_gated_ship_branch,
    is_git_write_cmd,
    normalize_branch_slug,
    _resolve_roadmap_dir,
    GIT_WRITE_COMMANDS,
    _first_line,
    _build_forge_create_args,
    _all_doc_only,
    _default_base_branch,
    _git_commits_since,
    build_pr_body,
    COMMIT_MESSAGE_SEP,
    _detect_pending_squash_merges,
)
from trackfw import config as _trackfw_config
from trackfw.forge.adapter import forge_adapter
from trackfw.commands import ship as ship_cmd


# ────────────────────────────────────────────────────────────────────────────
# helpers
# ────────────────────────────────────────────────────────────────────────────

class MockGit:
    """Captures calls and returns configured responses."""

    def __init__(self, branch='feat/default', staged='file.py', remote_url='',
                 base_ref='', commit_log=''):
        self.branch = branch
        self.staged = staged
        self.remote_url = remote_url
        self.base_ref = base_ref
        self.commit_log = commit_log
        self.calls = []

    def exec(self, args):
        self.calls.append(list(args))
        joined = ' '.join(args)

        if joined.startswith('symbolic-ref --short'):
            if not self.branch:
                return ('', 'not a git repo')
            return (self.branch, None)

        if joined.startswith('symbolic-ref refs/remotes/origin/HEAD'):
            if not self.base_ref:
                return ('', None)
            return (self.base_ref, None)

        if joined.startswith('diff --cached --name-only'):
            return (self.staged, None)

        if joined.startswith('log '):
            return (self.commit_log, None)

        if joined.startswith('remote get-url'):
            if self.remote_url:
                return (self.remote_url, None)
            return ('', 'no remote')

        if '@{u}' in joined:
            return ('', 'no upstream')

        if joined.startswith('fetch'):
            return ('', 'offline')

        return ('', None)


def make_deps(branch='feat/my-feature', staged='file.py', violations=None,
              config_forge='', repo_dir='', avail_fn=None, exec_forge_cli=None,
              remote_url='', check_pr_open=None):
    """Builds a dict of injectable dependencies.

    'lines' (stdout, via writeln) and 'err_lines' (stderr, via write_err) are kept as separate
    buffers so ML-1B stream-routing can be tested directly; the combined text returned by run()
    still concatenates both so the many pre-existing substring assertions below stay meaningful
    regardless of which stream a given message lands on.
    """
    git = MockGit(branch=branch, staged=staged, remote_url=remote_url)
    lines = []
    err_lines = []
    cli_calls = []

    def _noop_forge_cli(name, args):
        cli_calls.append({'name': name, 'args': args})
        return None

    return {
        'git': git,
        'lines': lines,
        'err_lines': err_lines,
        'cli_calls': cli_calls,
        'exec_git': git.exec,
        'check_governance': lambda: violations if violations is not None else [],
        'writeln': lambda s: lines.append(s),
        'write_err': lambda s: err_lines.append(f'Error: {s}'),
        'config_forge': config_forge,
        'repo_dir': repo_dir,
        # Step 7 safe defaults: no CLI invoked, no filesystem access.
        'avail_fn': avail_fn or (lambda name: False),
        'exec_forge_cli': exec_forge_cli or _noop_forge_cli,
        'check_pr_open': check_pr_open,
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
        write_err=d['write_err'],
        avail_fn=d['avail_fn'],
        exec_forge_cli=d['exec_forge_cli'],
    )
    return code, '\n'.join(d['lines'] + d['err_lines']), d['git']


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


@pytest.mark.parametrize('branch', ['feature/foo', 'hotfix/bar', 'chores/typo', 'mybranch'])
def test_ship_wrong_pattern_aborts(branch):
    code, out, _ = run(branch=branch)
    assert code == 1
    assert 'does not match the required pattern' in out


@pytest.mark.parametrize('branch', ['feat/my-feature', 'fix/bug-123', 'refactor/clean-up', 'chore/release-x.y.z', 'docs/update-readme'])
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
    assert 'lenient' in out, "output must mention lenient mode so users understand why validate passes but ship aborts"


# ────────────────────────────────────────────────────────────────────────────
# ML-1B — error stream/prefix parity: the final one-line summary of every abort path goes to
# stderr with the "Error: " prefix (mirrors Go's cobra/root.go behavior exactly); everything
# printed before it (violation lists, remediation hints) stays on stdout with no lowercase
# "error: " leaking in. Byte-level cross-runtime parity for these scenarios is proven by
# scripts/check-ship-parity.sh.
# ────────────────────────────────────────────────────────────────────────────

def test_ship_governance_violation_summary_on_stderr_with_prefix():
    v = ['branch "feat/foo" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/']
    d = make_deps(branch='feat/foo', staged='file.py', violations=v)
    code = run_ship(
        message='feat: test',
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
        write_err=d['write_err'],
    )
    assert code == 1
    assert d['err_lines'] == ['Error: governance check failed: 1 violation(s)']
    stdout = '\n'.join(d['lines'])
    assert 'governance check failed' not in stdout, "the summary must NOT also appear on stdout"
    for line in d['lines']:
        assert not line.startswith('error: '), f"stdout line must not start with lowercase 'error: ': {line}"


def test_ship_branch_pattern_mismatch_summary_on_stderr_with_prefix():
    d = make_deps(branch='hotfix/whatever', staged='file.py')
    code = run_ship(
        message='fix: test',
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
        write_err=d['write_err'],
    )
    assert code == 1
    assert len(d['err_lines']) == 1
    assert d['err_lines'][0].startswith('Error: branch "hotfix/whatever" does not match the required pattern')
    assert d['lines'] == [], "nothing should have been written to stdout before this abort"


# ────────────────────────────────────────────────────────────────────────────
# Doc-only exception — Steps 1 & 2 skip branch-pattern and governance checks
# ────────────────────────────────────────────────────────────────────────────

def test_ship_doc_only_branch_non_conforming_name_allowed():
    # "docs/foo" does not match feat|fix|refactor/<slug> — normally rejected by
    # is_ship_branch, but every staged file is doc-only, so Step 1's branch-pattern
    # check must be skipped.
    def _should_never_be_called():
        raise AssertionError('check_governance must not be called for a doc-only change')

    git = MockGit(branch='docs/foo', staged='docs/some-note.md')
    lines = []
    code = run_ship(
        message='docs: update note',
        dry_run=True,
        exec_git=git.exec,
        check_governance=_should_never_be_called,
        writeln=lambda s: lines.append(s),
    )
    out = '\n'.join(lines)
    assert code == 0, f"doc-only change on non-conforming branch name should not be blocked: {out}"
    assert 'does not match the required pattern' not in out


def test_ship_doc_only_branch_missing_roadmap_governance_skipped():
    # feat/<slug> is a correctly named branch, but governance would fail — doc-only
    # staged content must skip governance entirely, never calling check_governance.
    called = {'value': False}

    def _tracking_governance():
        called['value'] = True
        return ['no matching roadmap in wip/ nor done/']

    git = MockGit(branch='feat/doc-fix', staged='docs/req/REQ-x.md\nvault/notes/note.md')
    lines = []
    code = run_ship(
        message='docs: fix req',
        dry_run=True,
        exec_git=git.exec,
        check_governance=_tracking_governance,
        writeln=lambda s: lines.append(s),
    )
    out = '\n'.join(lines)
    assert code == 0, f"doc-only change must not be blocked by governance: {out}"
    assert not called['value'], 'check_governance must not be called at all for a doc-only change'
    assert 'Governance: skipped (doc-only change)' in out


def test_ship_mixed_doc_and_code_still_blocked_by_governance():
    # One non-doc file staged alongside doc files must NOT trigger the doc-only
    # exception — governance runs exactly as it does today, and the configured
    # violation still blocks.
    v = ['branch "feat/mixed" is a feat/fix/refactor branch but no roadmap is in wip/']
    code, out, _ = run(branch='feat/mixed', staged='docs/note.md\ntrackfw/ship/runner.py', violations=v)
    assert code == 1
    assert 'governance check failed' in out
    assert 'skipped (doc-only change)' not in out


def test_ship_mixed_doc_and_code_non_conforming_branch_still_blocked():
    # Same mixed-content guarantee, but on a branch name outside the ship vocabulary entirely
    # (feat/fix/refactor/chore/docs) — must still fail Step 1's branch-pattern check.
    code, out, _ = run(branch='hotfix/mixed', staged='docs/note.md\ntrackfw/ship/runner.py')
    assert code == 1
    assert 'does not match the required pattern' in out


# ────────────────────────────────────────────────────────────────────────────
# chore/docs branch-type exception — Step 2 skips governance regardless of staged content
# ────────────────────────────────────────────────────────────────────────────

def test_ship_chore_branch_mixed_content_governance_skipped():
    # "chore/release-x.y.z" carries a non-doc file staged (not doc-only) — proves the skip is
    # keyed on branch type, not on the pre-existing doc-only staged-content exception.
    called = {'value': False}

    def _tracking_governance():
        called['value'] = True
        return ['should never be called']

    git = MockGit(branch='chore/release-x.y.z', staged='trackfw/ship/runner.py')
    lines = []
    code = run_ship(
        message='chore: release x.y.z',
        dry_run=True,
        exec_git=git.exec,
        check_governance=_tracking_governance,
        writeln=lambda s: lines.append(s),
    )
    out = '\n'.join(lines)
    assert code == 0, f"chore branch must not be blocked by governance: {out}"
    assert not called['value'], 'check_governance must not be called at all for a chore/docs branch'
    assert 'Governance: skipped (chore/docs branch)' in out


def test_ship_docs_branch_mixed_content_governance_skipped():
    called = {'value': False}

    def _tracking_governance():
        called['value'] = True
        return ['should never be called']

    git = MockGit(branch='docs/update-readme', staged='docs/note.md\ntrackfw/ship/runner.py')
    lines = []
    code = run_ship(
        message='docs: update readme',
        dry_run=True,
        exec_git=git.exec,
        check_governance=_tracking_governance,
        writeln=lambda s: lines.append(s),
    )
    out = '\n'.join(lines)
    assert code == 0, f"docs branch must not be blocked by governance: {out}"
    assert not called['value'], 'check_governance must not be called at all for a chore/docs branch'
    assert 'Governance: skipped (chore/docs branch)' in out


def test_ship_feat_branch_no_roadmap_non_regression():
    # Non-regression: feat/fix/refactor branches must still be hard-gated on governance —
    # loosening the gate for chore/docs must not loosen it for feat/fix/refactor.
    v = ['branch "feat/no-roadmap" is a feat/fix/refactor branch but no roadmap is in wip/']
    code, out, _ = run(branch='feat/no-roadmap', dry_run=True, violations=v)
    assert code == 1, "expected governance error — feat/fix/refactor must still be gated"
    assert 'governance check failed' in out
    assert 'Governance: skipped' not in out


# ────────────────────────────────────────────────────────────────────────────
# _all_doc_only unit tests
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.parametrize('files', [
    ['docs/req/REQ-x.md'],
    ['vault/notes/note.md'],
    ['README.md'],
    ['docs/req/REQ-x.md', 'vault/notes/note.md', 'CHANGELOG.md'],
])
def test_all_doc_only_true(files):
    assert _all_doc_only(files), f"_all_doc_only({files}) should be True"


@pytest.mark.parametrize('files', [
    None,
    [],
    ['trackfw/ship/runner.py'],
    ['docs/req/REQ-x.md', 'trackfw/ship/runner.py'],
    ['setup.py'],
])
def test_all_doc_only_false(files):
    assert not _all_doc_only(files), f"_all_doc_only({files}) should be False"


# ────────────────────────────────────────────────────────────────────────────
# _default_base_branch unit tests
# ────────────────────────────────────────────────────────────────────────────

def test_default_base_branch_symbolic_ref_succeeds():
    exec_git = lambda args: ('refs/remotes/origin/develop', None)  # noqa: E731
    assert _default_base_branch(exec_git) == 'develop'


def test_default_base_branch_symbolic_ref_fails_falls_back_to_main():
    exec_git = lambda args: ('', 'no remote-tracking HEAD')  # noqa: E731
    assert _default_base_branch(exec_git) == 'main'


def test_default_base_branch_empty_output_falls_back_to_main():
    exec_git = lambda args: ('', None)  # noqa: E731
    assert _default_base_branch(exec_git) == 'main'


# ────────────────────────────────────────────────────────────────────────────
# build_pr_body unit tests
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.parametrize('commits', [[], ['feat: single commit']])
def test_build_pr_body_zero_or_one_commit_minimal_body(commits):
    body = build_pr_body('feat/my-feature', commits)
    assert body == 'Branch: feat/my-feature\n\nCreated by trackfw ship.'


def test_build_pr_body_multiple_commits_aggregates_history():
    commits = [
        'feat(ship): add doc-only exception\n\nSkips governance for docs/vault/md-only staged files.',
        'fix(ship): correct base branch fallback',
        'docs: update roadmap status',
    ]
    body = build_pr_body('feat/my-feature', commits)

    assert '## Commits' in body
    for subject in [
        '- feat(ship): add doc-only exception',
        '- fix(ship): correct base branch fallback',
        '- docs: update roadmap status',
    ]:
        assert subject in body
    assert '## Detalhes' in body
    assert 'Skips governance for docs/vault/md-only staged files.' in body
    assert '---\nBranch: feat/my-feature' in body

    # Exact-equality pin: this is what internal/commands/ship.go's buildPRBody
    # produces byte-for-byte for the same input — substring checks alone would not
    # catch a whitespace divergence that the cross-CLI parity audit would flag.
    assert body == (
        '## Commits\n\n'
        '- feat(ship): add doc-only exception\n'
        '- fix(ship): correct base branch fallback\n'
        '- docs: update roadmap status\n'
        '\n## Detalhes\n\n'
        '**feat(ship): add doc-only exception**\n\n'
        'Skips governance for docs/vault/md-only staged files.\n'
        '\n---\nBranch: feat/my-feature\n'
    )


# ────────────────────────────────────────────────────────────────────────────
# _git_commits_since unit tests
# ────────────────────────────────────────────────────────────────────────────

def test_git_commits_since_parses_separated_commits():
    log = 'feat: first' + COMMIT_MESSAGE_SEP + 'fix: second\n\nwith a body' + COMMIT_MESSAGE_SEP
    git = MockGit(branch='feat/my-feature', commit_log=log)
    commits = _git_commits_since('main', git.exec)
    assert len(commits) == 2
    assert commits[0] == 'feat: first'
    assert commits[1] == 'fix: second\n\nwith a body'


def test_git_commits_since_empty_range_returns_empty():
    git = MockGit(branch='feat/my-feature', commit_log='')
    commits = _git_commits_since('main', git.exec)
    assert commits == []


# ────────────────────────────────────────────────────────────────────────────
# End-to-end: --dry-run PR body reflects real branch commit history
# ────────────────────────────────────────────────────────────────────────────

def test_ship_dry_run_pr_body_aggregates_commit_history():
    git = MockGit(
        branch='feat/my-feature',
        staged='file.py',
        remote_url='https://github.com/org/repo.git',
        base_ref='refs/remotes/origin/main',
        commit_log='feat(x): third commit' + COMMIT_MESSAGE_SEP + 'feat(x): second commit' + COMMIT_MESSAGE_SEP,
    )
    lines = []
    code = run_ship(
        message='feat(x): first commit (this ship call)',
        dry_run=True,
        config_forge='github',
        exec_git=git.exec,
        check_governance=lambda: [],
        writeln=lambda s: lines.append(s),
        avail_fn=lambda name: False,
        exec_forge_cli=lambda name, args: None,
    )
    out = '\n'.join(lines)
    assert code == 0
    assert '[dry-run] Title: feat(x): first commit (this ship call)' in out
    assert '## Commits' in out
    assert 'feat(x): third commit' in out


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

@pytest.mark.parametrize('branch', ['feat/foo', 'feat/a-very-long-slug', 'fix/123', 'refactor/clean-up', 'chore/x', 'docs/x'])
def test_is_ship_branch_valid(branch):
    assert is_ship_branch(branch), f"{branch} should be valid"


@pytest.mark.parametrize('branch', ['main', 'master', 'feature/foo', 'hotfix/bar', 'feat/', 'refactor/', 'chore/', 'docs/'])
def test_is_ship_branch_invalid(branch):
    assert not is_ship_branch(branch), f"{branch} should be invalid"


@pytest.mark.parametrize('branch', ['feat/foo', 'fix/123', 'refactor/clean-up'])
def test_is_gated_ship_branch_true(branch):
    assert is_gated_ship_branch(branch), f"{branch} should be gated"


@pytest.mark.parametrize('branch', ['chore/x', 'docs/x', 'main', 'feature/foo', 'chore/', 'docs/'])
def test_is_gated_ship_branch_false(branch):
    assert not is_gated_ship_branch(branch), f"{branch} should not be gated"


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


# ────────────────────────────────────────────────────────────────────────────
# Step 7 — forge resolution and PR/MR opening
# ────────────────────────────────────────────────────────────────────────────

def _make_step7(config_forge='', forge_flag='', avail_fn=None):
    """Returns (deps, cli_calls, opts_kwargs) ready to reach Step 7."""
    cli_calls = []

    def mock_exec_forge_cli(name, args):
        cli_calls.append({'name': name, 'args': args})
        return None

    d = make_deps(
        branch='feat/my-feature',
        staged='file.py',
        config_forge=config_forge,
        repo_dir='',
        avail_fn=avail_fn or (lambda name: False),
        exec_forge_cli=mock_exec_forge_cli,
    )
    d['cli_calls_ref'] = cli_calls
    kwargs = dict(
        message='feat(x): test step7',
        dry_run=False,
        no_pr=False,
        forge_flag=forge_flag,
        config_forge=config_forge,
        repo_dir='',
        avail_fn=d['avail_fn'],
        exec_forge_cli=mock_exec_forge_cli,
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
    )
    return d, cli_calls, kwargs


def test_ship_step7_gitlab_says_merge_request():
    d, cli_calls, kwargs = _make_step7(config_forge='gitlab')
    code = run_ship(**kwargs)
    out = '\n'.join(d['lines'])
    assert code == 0
    assert 'Merge Request' in out, f"expected Merge Request, got: {out}"


def test_ship_step7_github_says_pull_request():
    d, cli_calls, kwargs = _make_step7(config_forge='github')
    code = run_ship(**kwargs)
    out = '\n'.join(d['lines'])
    assert code == 0
    assert 'Pull Request' in out, f"expected Pull Request, got: {out}"


def test_ship_step7_cli_unavailable_exit0_with_url():
    cli_calls = []

    def mock_exec_forge_cli(name, args):
        cli_calls.append({'name': name, 'args': args})
        return None

    git = MockGit(branch='feat/my-feature', staged='file.py')
    orig_exec = git.exec

    def exec_git_with_remote(args):
        if args[:3] == ['remote', 'get-url', 'origin']:
            return ('https://github.com/org/repo.git', None)
        return orig_exec(args)

    lines = []
    code = run_ship(
        message='feat(x): test',
        dry_run=False,
        no_pr=False,
        forge_flag='',
        config_forge='github',
        repo_dir='',
        avail_fn=lambda name: False,
        exec_forge_cli=mock_exec_forge_cli,
        exec_git=exec_git_with_remote,
        check_governance=lambda: [],
        writeln=lambda s: lines.append(s),
    )
    out = '\n'.join(lines)
    assert code == 0
    assert len(cli_calls) == 0, 'exec_forge_cli must not be called when CLI unavailable'
    assert 'github.com' in out, f"expected fallback URL, got: {out}"


def test_ship_step7_manual_forge_exit0():
    d, cli_calls, kwargs = _make_step7(config_forge='', forge_flag='')
    code = run_ship(**kwargs)
    out = '\n'.join(d['lines'])
    assert code == 0
    assert len(cli_calls) == 0, 'exec_forge_cli must not be called for manual forge'
    assert 'ship complete' in out


def test_ship_step7_no_pr_skips_step7():
    d, cli_calls, kwargs = _make_step7(config_forge='github', avail_fn=lambda name: True)
    kwargs['no_pr'] = True
    code = run_ship(**kwargs)
    out = '\n'.join(d['lines'])
    assert code == 0
    assert len(cli_calls) == 0, 'exec_forge_cli must not be called with --no-pr'
    assert '--no-pr' in out
    assert 'ship complete' in out


def test_ship_step7_forge_flag_overrides():
    d, cli_calls, kwargs = _make_step7(config_forge='', forge_flag='github')
    code = run_ship(**kwargs)
    out = '\n'.join(d['lines'])
    assert code == 0
    assert 'github (source: flag)' in out, f"expected source: flag, got: {out}"


def test_ship_step7_dry_run_no_forge_cli():
    d, cli_calls, kwargs = _make_step7(config_forge='github', avail_fn=lambda name: True)
    kwargs['dry_run'] = True
    code = run_ship(**kwargs)
    out = '\n'.join(d['lines'])
    assert code == 0
    assert len(cli_calls) == 0, 'exec_forge_cli must not be called in dry-run mode'
    assert '[dry-run]' in out or 'would open' in out, f"expected dry-run marker: {out}"


def test_ship_step7_source_in_output():
    d, cli_calls, kwargs = _make_step7(config_forge='gitlab')
    run_ship(**kwargs)
    out = '\n'.join(d['lines'])
    assert 'source: config' in out, f"expected source: config, got: {out}"


def test_ship_step7_cli_available_invokes_exec():
    d, cli_calls, kwargs = _make_step7(config_forge='github', avail_fn=lambda name: True)
    code = run_ship(**kwargs)
    assert code == 0
    assert len(cli_calls) == 1, f"expected 1 CLI call, got {len(cli_calls)}"
    assert cli_calls[0]['name'] == 'gh'
    assert '--title' in cli_calls[0]['args'], f"expected --title in args: {cli_calls[0]['args']}"


# ────────────────────────────────────────────────────────────────────────────
# _build_forge_create_args unit tests
# ────────────────────────────────────────────────────────────────────────────

def test_build_forge_create_args_github_uses_body():
    adapter = forge_adapter('github', lambda name: False)
    args = _build_forge_create_args(adapter, 'my title', 'my body')
    assert args == ['pr', 'create', '--title', 'my title', '--body', 'my body']


def test_build_forge_create_args_azure_uses_description():
    adapter = forge_adapter('azure', lambda name: False)
    args = _build_forge_create_args(adapter, 'my title', 'my body')
    assert '--body' not in args, 'azure must not use --body'
    assert '--description' in args, 'azure must use --description'


def test_build_forge_create_args_never_mutates():
    adapter = forge_adapter('gitlab', lambda name: False)
    original = list(adapter.cli_args)
    _build_forge_create_args(adapter, 't1', 'b1')
    _build_forge_create_args(adapter, 't2', 'b2')
    assert adapter.cli_args == original, 'adapter.cli_args must not be mutated'


def test_first_line_multiline():
    assert _first_line('feat(x): title\n\nbody') == 'feat(x): title'


def test_first_line_no_newline():
    assert _first_line('no newline') == 'no newline'


def test_first_line_empty():
    assert _first_line('') == ''


# ────────────────────────────────────────────────────────────────────────────
# Forge matrix — 4 forges × 2 avail states × 2 host types (16 cells)
# All cells run with dry_run=True to skip real push.
# ────────────────────────────────────────────────────────────────────────────

KNOWN_URLS = {
    'github':    'https://github.com/org/repo.git',
    'gitlab':    'https://gitlab.com/org/repo.git',
    'bitbucket': 'https://bitbucket.org/org/repo.git',
    'azure':     'https://dev.azure.com/org/proj/_git/repo',
}
SELF_HOSTED_URL = 'https://git.mycompany.com/org/repo.git'

_FORGE_MATRIX = [
    # forge, cli_present, remote_url, noun, expected_substring
    # github × known host
    ('github',    True,  KNOWN_URLS['github'],    'Pull Request',  '[dry-run] would open Pull Request via github CLI'),
    ('github',    False, KNOWN_URLS['github'],    'Pull Request',  'github.com'),
    # github × self-hosted
    ('github',    True,  SELF_HOSTED_URL,         'Pull Request',  '[dry-run] would open Pull Request via github CLI'),
    ('github',    False, SELF_HOSTED_URL,         'Pull Request',  'mycompany.com'),
    # gitlab × known host
    ('gitlab',    True,  KNOWN_URLS['gitlab'],    'Merge Request', '[dry-run] would open Merge Request via gitlab CLI'),
    ('gitlab',    False, KNOWN_URLS['gitlab'],    'Merge Request', 'gitlab.com'),
    # gitlab × self-hosted
    ('gitlab',    True,  SELF_HOSTED_URL,         'Merge Request', '[dry-run] would open Merge Request via gitlab CLI'),
    ('gitlab',    False, SELF_HOSTED_URL,         'Merge Request', 'mycompany.com'),
    # bitbucket × known host (bitbucket has no CLI — always absent regardless of avail_fn)
    ('bitbucket', True,  KNOWN_URLS['bitbucket'], 'Pull Request',  'bitbucket.org'),
    ('bitbucket', False, KNOWN_URLS['bitbucket'], 'Pull Request',  'bitbucket.org'),
    # bitbucket × self-hosted
    ('bitbucket', True,  SELF_HOSTED_URL,         'Pull Request',  'mycompany.com'),
    ('bitbucket', False, SELF_HOSTED_URL,         'Pull Request',  'mycompany.com'),
    # azure × known host
    ('azure',     True,  KNOWN_URLS['azure'],     'Pull Request',  '[dry-run] would open Pull Request via azure CLI'),
    ('azure',     False, KNOWN_URLS['azure'],     'Pull Request',  'dev.azure.com'),
    # azure × self-hosted
    ('azure',     True,  SELF_HOSTED_URL,         'Pull Request',  '[dry-run] would open Pull Request via azure CLI'),
    ('azure',     False, SELF_HOSTED_URL,         'Pull Request',  'mycompany.com'),
]


@pytest.mark.parametrize(
    'forge,cli_present,remote_url,noun,expected_sub',
    _FORGE_MATRIX,
    ids=[
        f"{f}×{'cli-present' if c else 'cli-absent'}×{'self-hosted' if 'mycompany' in r else 'known-host'}"
        for f, c, r, *_ in _FORGE_MATRIX
    ],
)
def test_forge_matrix(forge, cli_present, remote_url, noun, expected_sub):
    """Verifies dry-run output for every cell of the forge × avail × host matrix."""
    cli_calls = []

    def mock_exec_forge_cli(name, args):
        cli_calls.append({'name': name, 'args': args})
        return None

    d = make_deps(
        branch='feat/my-feature',
        staged='file.py',
        config_forge=forge,
        remote_url=remote_url,
        avail_fn=lambda name: cli_present,
        exec_forge_cli=mock_exec_forge_cli,
    )

    code = run_ship(
        message='feat(x): matrix test',
        dry_run=True,
        no_pr=False,
        forge_flag='',
        config_forge=forge,
        repo_dir='',
        avail_fn=d['avail_fn'],
        exec_forge_cli=mock_exec_forge_cli,
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
    )
    out = '\n'.join(d['lines'])

    assert code == 0, f"expected exit 0, got {code}\noutput: {out}"
    assert f'Forge:     {forge} (source: config)' in out, \
        f'expected forge line with source: config, got: {out}'
    assert noun in out, f'expected noun "{noun}" in output, got: {out}'
    assert expected_sub in out, f'expected "{expected_sub}" in output, got: {out}'
    assert len(cli_calls) == 0, \
        f'dry-run must not invoke exec_forge_cli, got {len(cli_calls)} calls'


# ────────────────────────────────────────────────────────────────────────────
# Silence usage — runtime errors must NOT show "usage" text
# ────────────────────────────────────────────────────────────────────────────

def test_silence_usage_runtime_error_no_usage_text():
    """A runtime error (branch validation) must not print usage/help text."""
    code, out, _ = run(branch='main')
    assert code == 1
    assert 'usage' not in out.lower(), \
        f"runtime error must not print usage text, got: {out}"


def test_silence_usage_parse_error_shows_usage():
    """An argparse parse error (unknown flag) must print usage."""
    import subprocess
    import sys
    pypi_root = os.path.join(os.path.dirname(__file__), '..')
    result = subprocess.run(
        [sys.executable, '-m', 'trackfw', 'ship', '--unknown-flag-xyz'],
        capture_output=True,
        text=True,
        env={**os.environ, 'PYTHONPATH': pypi_root},
    )
    out = result.stdout + result.stderr
    assert result.returncode != 0
    assert 'usage' in out.lower() or 'error' in out.lower(), \
        f"expected usage/error on unknown flag, got: {out}"


# ────────────────────────────────────────────────────────────────────────────
# Parity test — _resolve_roadmap_dir default must be docs/roadmaps (not docs/roadmaps/claude)
# Locks the default across all runtimes: Go, Node.js, Python all use docs/roadmaps.
# ────────────────────────────────────────────────────────────────────────────

def test_resolve_roadmap_dir_default_is_docs_roadmaps():
    """
    Without a trackfw.yaml, _resolve_roadmap_dir() must return 'docs/roadmaps'.
    This is the parity lock: Go, Node.js and Python must agree on the same default.
    """
    import tempfile
    with tempfile.TemporaryDirectory() as tmpdir:
        _trackfw_config.reset()
        try:
            result = _resolve_roadmap_dir(tmpdir)
            assert result == 'docs/roadmaps', (
                f'default roadmap_dir must be "docs/roadmaps", got "{result}" '
                '— parity lock violated'
            )
        finally:
            _trackfw_config.reset()  # clean singleton so subsequent tests are unaffected


# ────────────────────────────────────────────────────────────────────────────
# Integration test — real Python binary with clean PATH (no gh/glab/az)
# ────────────────────────────────────────────────────────────────────────────

def test_ship_integration_graceful_degradation_clean_path():
    """Runs the real Python CLI with PATH containing only git.
    Verifies exit 0 and fallback URL in output when forge CLI is absent.
    """
    import subprocess
    import sys
    import shutil
    import tempfile

    tmp_dir = tempfile.mkdtemp(prefix='trackfw-ship-py-')
    try:
        # Create tmpBin with only git
        tmp_bin = os.path.join(tmp_dir, 'bin')
        os.makedirs(tmp_bin)
        git_path = shutil.which('git')
        assert git_path, 'git must be installed to run integration test'
        os.symlink(git_path, os.path.join(tmp_bin, 'git'))

        # Create git repo
        repo_dir = os.path.join(tmp_dir, 'repo')
        os.makedirs(repo_dir)

        def git(*args):
            return subprocess.run(['git', *args], cwd=repo_dir, capture_output=True, text=True)

        git('init')
        git('config', 'user.email', 'test@example.com')
        git('config', 'user.name', 'Test')
        # Set HEAD to feat/my-feature without committing
        subprocess.run(
            ['git', 'symbolic-ref', 'HEAD', 'refs/heads/feat/my-feature'],
            cwd=repo_dir, capture_output=True, text=True,
        )
        git('remote', 'add', 'origin', 'https://github.com/org/repo.git')

        # Stage a file
        with open(os.path.join(repo_dir, 'staged.txt'), 'w') as f:
            f.write('content\n')
        git('add', 'staged.txt')

        # Create governance: wip roadmap with branch slug and REQ
        wip_dir = os.path.join(repo_dir, 'docs', 'roadmaps', 'wip')
        os.makedirs(wip_dir)
        with open(os.path.join(wip_dir, 'ROADMAP-my-feature-integration-test.md'), 'w') as f:
            f.write('REQ: REQ-ship-integration-test\n\n# Roadmap: Integration Test\n\n'
                    'Test roadmap for graceful degradation proof.\n')

        # Run Python CLI with absolute interpreter path and clean PATH (no gh/glab/az)
        pypi_root = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
        result = subprocess.run(
            [sys.executable, '-m', 'trackfw', 'ship',
             '--dry-run', '--forge', 'github', '-m', 'feat: integration test'],
            cwd=repo_dir,
            capture_output=True,
            text=True,
            env={
                'PATH': tmp_bin,
                'HOME': tmp_dir,
                'PYTHONPATH': pypi_root,
                'GIT_AUTHOR_NAME': 'Test',
                'GIT_AUTHOR_EMAIL': 'test@example.com',
                'GIT_COMMITTER_NAME': 'Test',
                'GIT_COMMITTER_EMAIL': 'test@example.com',
            },
        )

        out = result.stdout + result.stderr
        assert result.returncode == 0, \
            f"expected exit 0, got {result.returncode}\noutput: {out}"
        assert 'github.com' in out, \
            f"expected github.com URL in output, got: {out}"

    finally:
        shutil.rmtree(tmp_dir, ignore_errors=True)


# ────────────────────────────────────────────────────────────────────────────
# ML-2A — _detect_pending_squash_merges reuses evaluate_branch_integration
# ────────────────────────────────────────────────────────────────────────────

def _git_available():
    try:
        subprocess.run(["git", "--version"], capture_output=True, check=True)
        return True
    except Exception:
        return False


@pytest.mark.skipif(not _git_available(), reason="git not available in PATH")
def test_detect_pending_squash_merges_real_git_repo_stale_vs_pending():
    """P4 falsification scenario for ML-2A: reproduces the PR #181/#182 incident in a real,
    disposable git repository. origin/feat/a is squash-merged into origin/main, which then
    advances further (origin/feat/b), leaving origin/feat/a's naive bidirectional diff non-empty
    even though every file it touched is already on main. origin/feat/pending never merges
    anywhere and must still warn.
    """
    with tempfile.TemporaryDirectory(prefix="trackfw-ship-squash-py-") as work:
        bare_dir = os.path.join(work, "origin.git")
        clone_dir = os.path.join(work, "clone")
        empty_gitconfig = os.path.join(work, "empty-gitconfig")
        with open(empty_gitconfig, "w") as f:
            f.write("")

        env = dict(os.environ)
        env.update({
            "GIT_CONFIG_GLOBAL": empty_gitconfig,
            "GIT_CONFIG_SYSTEM": "/dev/null",
            "GIT_TERMINAL_PROMPT": "0",
            "HOME": work,
        })

        def run(cwd, args):
            result = subprocess.run(
                ["git"] + args, cwd=cwd, env=env, capture_output=True, text=True
            )
            if result.returncode != 0:
                raise AssertionError(
                    f"git {args} (cwd={cwd}) failed: {result.stderr}\n{result.stdout}"
                )
            return result.stdout

        os.makedirs(bare_dir, exist_ok=True)
        run(bare_dir, ["init", "-q", "--bare", "-b", "main"])

        os.makedirs(clone_dir, exist_ok=True)
        run(work, ["clone", "-q", bare_dir, clone_dir])
        run(clone_dir, ["config", "user.email", "falsify@trackfw.test"])
        run(clone_dir, ["config", "user.name", "trackfw falsify"])
        run(clone_dir, ["config", "commit.gpgsign", "false"])
        run(clone_dir, ["config", "core.hooksPath", "/dev/null"])

        def write_file(name, content):
            with open(os.path.join(clone_dir, name), "w") as f:
                f.write(content)

        write_file("base.txt", "base\n")
        run(clone_dir, ["add", "base.txt"])
        run(clone_dir, ["commit", "-q", "-m", "base commit"])
        run(clone_dir, ["push", "-q", "origin", "main"])

        # feat/a — pushed to origin, squash-merged into main (ancestry never records the merge).
        run(clone_dir, ["checkout", "-q", "-b", "feat/a"])
        write_file("a.txt", "a\n")
        run(clone_dir, ["add", "a.txt"])
        run(clone_dir, ["commit", "-q", "-m", "feat/a work"])
        run(clone_dir, ["push", "-q", "origin", "feat/a"])
        run(clone_dir, ["checkout", "-q", "main"])
        run(clone_dir, ["merge", "-q", "--squash", "feat/a"])
        run(clone_dir, ["commit", "-q", "-m", "squash-merge feat/a (PR #181)"])

        # main advances further (PR #182) — makes the naive bidirectional diff non-empty for the
        # already-integrated feat/a.
        write_file("b.txt", "b\n")
        run(clone_dir, ["add", "b.txt"])
        run(clone_dir, ["commit", "-q", "-m", "unrelated follow-up (PR #182)"])
        run(clone_dir, ["push", "-q", "origin", "main"])

        # feat/pending — pushed to origin, genuinely never merged anywhere.
        run(clone_dir, ["checkout", "-q", "-b", "feat/pending"])
        write_file("c.txt", "c\n")
        run(clone_dir, ["add", "c.txt"])
        run(clone_dir, ["commit", "-q", "-m", "feat/pending work, never merged"])
        run(clone_dir, ["push", "-q", "origin", "feat/pending"])

        run(clone_dir, ["checkout", "-q", "main"])
        run(clone_dir, ["fetch", "-q", "origin"])

        def exec_git(args):
            result = subprocess.run(
                ["git"] + args, cwd=clone_dir, env=env, capture_output=True, text=True
            )
            if result.returncode != 0:
                return ("", result.stderr.strip() or f"git {' '.join(args)} failed")
            return (result.stdout.strip(), None)

        # P4 baseline: the naive bidirectional check IS non-empty for origin/feat/a — reproduces
        # the exact PR #181/#182 false positive.
        naive_out, naive_err = exec_git(["diff", "origin/main", "origin/feat/a", "--stat"])
        assert naive_err is None
        assert naive_out.strip() != "", (
            "fixture inválida: diff ingênuo deve ser não-vazio para reproduzir o falso "
            "positivo do #181/#182"
        )

        # P4 detection: the fixed _detect_pending_squash_merges must not warn about feat/a and
        # must still warn about feat/pending.
        lines = []
        _detect_pending_squash_merges("main", exec_git, lines.append)
        got = "\n".join(lines)

        assert '"feat/a"' not in got, (
            f"must NOT warn about feat/a (stale-but-integrated). Output:\n{got}"
        )
        assert '"feat/pending"' in got, (
            f"must still warn about feat/pending (genuinely unmerged). Output:\n{got}"
        )


def test_detect_pending_squash_merges_delegates_to_evaluate_branch_integration():
    """A raw bidirectional-diff implementation would call `diff origin/main <candidate> --stat`
    directly; the shared evaluate_branch_integration only reaches its own -z diffs after a
    successful merge-base, which never happens here (merge-base fails) — so no warning should
    fire and the raw diff call should never happen.
    """
    calls = {}

    def exec_git(args):
        key = " ".join(args)
        calls[key] = calls.get(key, 0) + 1
        if key == "branch -r --no-merged origin/main":
            return ("  origin/feat/unrelated-history\n", None)
        if key.startswith("merge-base origin/main"):
            return ("", "fatal: no merge base")
        if key.startswith("diff origin/main"):
            return ("some.file | 1 +\n", None)
        return ("", None)

    lines = []
    _detect_pending_squash_merges("main", exec_git, lines.append)

    assert lines == [], f"expected no warning when merge-base fails, got: {lines}"
    assert "diff origin/main origin/feat/unrelated-history --stat" not in calls, (
        "_detect_pending_squash_merges must not run its own bidirectional diff --stat anymore"
    )


# ────────────────────────────────────────────────────────────────────────────
# --force-with-lease — ML-1A
# ────────────────────────────────────────────────────────────────────────────

def _make_force_lease_deps(staged='file.py', check_pr_open=None):
    return make_deps(
        branch='fix/rebase-test',
        staged=staged,
        config_forge='github',
        avail_fn=lambda name: True,
        remote_url='https://github.com/org/repo.git',
        check_pr_open=check_pr_open,
    )


def _run_force_lease(d, message='fix: rebase', dry_run=False):
    code = run_ship(
        message=message,
        dry_run=dry_run,
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
        write_err=d['write_err'],
        avail_fn=d['avail_fn'],
        exec_forge_cli=d['exec_forge_cli'],
        config_forge=d['config_forge'],
        repo_dir=d['repo_dir'],
        force_with_lease=True,
        check_pr_open=d['check_pr_open'],
    )
    out = '\n'.join(d['lines'] + d['err_lines'])
    return code, out


def test_ship_force_lease_open_pr_succeeds():
    seen = {}

    def check_pr_open(adapter, branch):
        seen['branch'] = branch
        return True

    d = _make_force_lease_deps(check_pr_open=check_pr_open)
    code, out = _run_force_lease(d)
    assert code == 0, out
    assert seen['branch'] == 'fix/rebase-test'
    assert len(d['cli_calls']) == 0, 'exec_forge_cli must not be called to create a PR'

    push_call = next(c for c in d['git'].calls if c[0] == 'push')
    assert push_call == ['push', '--force-with-lease', '-u', 'origin', 'fix/rebase-test']
    assert 'already open — skipping creation (--force-with-lease)' in out


def test_ship_force_lease_push_only_when_nothing_staged():
    d = _make_force_lease_deps(staged='', check_pr_open=lambda a, b: True)
    code, out = _run_force_lease(d, message='')
    assert code == 0, out
    assert not any(c[0] == 'commit' for c in d['git'].calls), 'commit must not be called'
    assert 'pushes existing commits only' in out


def test_ship_nothing_staged_without_force_lease_still_aborts():
    d = make_deps(branch='fix/rebase-test', staged='')
    code = run_ship(
        message='fix: x',
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
        write_err=d['write_err'],
        avail_fn=d['avail_fn'],
        exec_forge_cli=d['exec_forge_cli'],
    )
    out = '\n'.join(d['lines'] + d['err_lines'])
    assert code == 1
    assert 'nothing is staged' in out


def test_ship_force_lease_no_open_pr_refuses():
    d = _make_force_lease_deps(check_pr_open=lambda a, b: False)
    code, out = _run_force_lease(d)
    assert code == 1
    assert 'no open pull/merge request' in out
    assert not any(c[0] in ('commit', 'push') for c in d['git'].calls), 'must not write before refusal'
    assert len(d['cli_calls']) == 0


def test_ship_force_lease_no_forge_cli_refuses_without_degrading():
    d = make_deps(
        branch='fix/rebase-test',
        staged='file.py',
        config_forge='github',
        avail_fn=lambda name: False,
        remote_url='https://github.com/org/repo.git',
    )
    code, out = _run_force_lease(d)
    assert code == 1
    assert 'requires a forge CLI' in out
    assert not any(c[0] in ('commit', 'push') for c in d['git'].calls)


def test_ship_force_lease_manual_forge_refuses():
    d = make_deps(branch='fix/rebase-test', staged='file.py', avail_fn=lambda name: True)
    code, out = _run_force_lease(d)
    assert code == 1
    assert 'requires a forge CLI' in out


def test_ship_force_lease_cannot_verify_refuses():
    def check_pr_open(adapter, branch):
        raise RuntimeError('gh: authentication required')

    d = _make_force_lease_deps(check_pr_open=check_pr_open)
    code, out = _run_force_lease(d)
    assert code == 1
    assert 'could not verify' in out
    assert not any(c[0] in ('commit', 'push') for c in d['git'].calls)


def test_ship_force_lease_dry_run_still_runs_gate():
    called = {'value': False}

    def check_pr_open(adapter, branch):
        called['value'] = True
        return False

    d = _make_force_lease_deps(check_pr_open=check_pr_open)
    code, out = _run_force_lease(d, dry_run=True)
    assert code == 1
    assert called['value'], 'check_pr_open must run in dry-run mode too'
    assert 'no open pull/merge request' in out


def test_ship_normal_push_unaffected_by_force_lease_path():
    d = make_deps(branch='fix/normal', staged='file.py')
    code = run_ship(
        message='fix: x',
        exec_git=d['exec_git'],
        check_governance=d['check_governance'],
        writeln=d['writeln'],
        write_err=d['write_err'],
        avail_fn=d['avail_fn'],
        exec_forge_cli=d['exec_forge_cli'],
        force_with_lease=False,
    )
    assert code == 0
    push_call = next(c for c in d['git'].calls if c[0] == 'push')
    assert push_call == ['push', '-u', 'origin', 'fix/normal']


def test_ship_cli_raw_force_flag_does_not_exist():
    import argparse

    parser = argparse.ArgumentParser(prog='trackfw')
    subparsers = parser.add_subparsers(dest='command')
    ship_cmd.register(subparsers)

    # --force-with-lease must parse.
    args = parser.parse_args(['ship', '-m', 'x', '--force-with-lease'])
    assert args.force_with_lease is True

    # Raw --force must NOT be accepted, not even as an abbreviation of
    # --force-with-lease (allow_abbrev=False on the ship subparser).
    with pytest.raises(SystemExit):
        parser.parse_args(['ship', '-m', 'x', '--force'])
