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
    _first_line,
    _build_forge_create_args,
)
from trackfw.forge.adapter import forge_adapter


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


def make_deps(branch='feat/my-feature', staged='file.py', violations=None,
              config_forge='', repo_dir='', avail_fn=None, exec_forge_cli=None):
    """Builds a dict of injectable dependencies."""
    git = MockGit(branch=branch, staged=staged)
    lines = []
    cli_calls = []

    def _noop_forge_cli(name, args):
        cli_calls.append({'name': name, 'args': args})
        return None

    return {
        'git': git,
        'lines': lines,
        'cli_calls': cli_calls,
        'exec_git': git.exec,
        'check_governance': lambda: violations if violations is not None else [],
        'writeln': lambda s: lines.append(s),
        'config_forge': config_forge,
        'repo_dir': repo_dir,
        # Step 7 safe defaults: no CLI invoked, no filesystem access.
        'avail_fn': avail_fn or (lambda name: False),
        'exec_forge_cli': exec_forge_cli or _noop_forge_cli,
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
        avail_fn=d['avail_fn'],
        exec_forge_cli=d['exec_forge_cli'],
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
    assert 'lenient' in out, "output must mention lenient mode so users understand why validate passes but ship aborts"


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
