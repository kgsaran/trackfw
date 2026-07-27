"""
ship/runner.py — Core implementation of `trackfw ship`.

All git write operations are injectable for testability.
Never passes "add ." or "add -A" to any git executor.
"""

import os
import re
import subprocess
import sys

from trackfw import config as _config
from trackfw.forge.resolve import resolve as forge_resolve
from trackfw.forge.adapter import forge_adapter


# Git subcommands that modify local or remote state.
# In --dry-run mode these are printed but not executed.
GIT_WRITE_COMMANDS = {"commit", "push", "fetch"}


def is_git_write_cmd(args):
    """Returns True when the first arg is a write-mode git subcommand."""
    return len(args) > 0 and args[0] in GIT_WRITE_COMMANDS


def is_ship_branch(branch):
    """Returns True when branch matches feat|fix|refactor/<slug>."""
    return bool(re.match(r'^(feat|fix|refactor)/.+', branch))


def normalize_branch_slug(value):
    """
    Converts a string to a lowercase dash-only slug.
    Identical algorithm to Go normalizeBranchSlug and JS normalizeBranchSlug.
    """
    out = []
    last_dash = False
    for ch in value.lower():
        if re.match(r'[a-z0-9]', ch):
            out.append(ch)
            last_dash = False
        elif not last_dash:
            out.append('-')
            last_dash = True
    return ''.join(out).strip('-')


def default_exec_git(args):
    """
    Production git executor.
    Returns (stdout_str, error_str_or_None).
    """
    try:
        result = subprocess.run(
            ['git'] + args,
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            return ('', result.stderr.strip() or f"git {' '.join(args)} failed")
        return (result.stdout.strip(), None)
    except FileNotFoundError:
        return ('', 'git not found in PATH')
    except Exception as e:
        return ('', str(e))


def _resolve_roadmap_dir(cwd=None):
    """
    Delegates to config.load() — single source of truth for roadmap_dir.
    Accepts an optional cwd for testability (passed through to config.load).
    Default when no trackfw.yaml is present: docs/roadmaps.
    """
    return _config.load(cwd)["roadmap_dir"]


def check_ship_governance():
    """
    Hard gate (bypasses config/baseline/lenient).
    Checks:
      1. Current branch has a matching roadmap in wip/
      2. WIP roadmaps have a linked REQ
    Returns list of violation messages (empty = pass).
    """
    violations = []

    # Resolve via config module — single source of truth (default: docs/roadmaps)
    roadmap_dir = _resolve_roadmap_dir()
    wip_dir = os.path.join(roadmap_dir, 'wip')

    stdout, err = default_exec_git(['symbolic-ref', '--short', 'HEAD'])
    branch = stdout.strip() if not err else ''

    if branch and is_ship_branch(branch):
        slug = normalize_branch_slug('/'.join(branch.split('/')[1:]))
        wip_files = []
        has_match = False

        if os.path.isdir(wip_dir):
            wip_files = [f for f in os.listdir(wip_dir) if f.endswith('.md')]
            for f in wip_files:
                if slug in normalize_branch_slug(f):
                    has_match = True
                    break

        if not wip_files:
            violations.append(
                f'branch "{branch}" is a feat/fix/refactor branch but no roadmap is in wip/ — '
                'create governance artifacts first:\n'
                '  trackfw req new "<title>"\n'
                '  trackfw roadmap new "<title>"\n'
                '  trackfw roadmap move <name> wip'
            )
        elif not has_match:
            violations.append(
                f'branch "{branch}" has no matching roadmap in wip/ '
                f'(found: {", ".join(wip_files)}) — '
                'include the branch slug in the roadmap filename'
            )

        if has_match and os.path.isdir(wip_dir):
            for f in wip_files:
                fpath = os.path.join(wip_dir, f)
                try:
                    content = open(fpath).read()
                    if 'REQ:' not in content and 'req:' not in content:
                        violations.append(f'roadmap "{f}" is in wip but has no linked REQ')
                except OSError:
                    pass

    return violations


def _detect_pending_squash_merges(current_branch, exec_git, writeln):
    """Warns about remote branches with non-empty diffs vs origin/main. Non-blocking."""
    stdout, err = exec_git(['branch', '-r', '--no-merged', 'origin/main'])
    if err or not stdout.strip():
        return

    for raw in stdout.split('\n'):
        candidate = raw.strip()
        if not candidate or 'HEAD' in candidate:
            continue
        short_name = re.sub(r'^origin/', '', candidate)
        if short_name == current_branch:
            continue
        diff_out, derr = exec_git(['diff', 'origin/main', candidate, '--stat'])
        if derr:
            continue
        if diff_out.strip():
            writeln(f'Warning: branch "{short_name}" appears to have unmerged changes vs origin/main.')


def _build_push_args(branch, exec_git):
    """Returns the push args, adding -u if no upstream is configured."""
    _, err = exec_git(['rev-parse', '--abbrev-ref', '--symbolic-full-name', '@{u}'])
    if err:
        return ['push', '-u', 'origin', branch]
    return ['push', 'origin', branch]


def _first_line(s):
    """Returns only the first line of s."""
    idx = s.find('\n')
    return s[:idx] if idx >= 0 else s


def _build_forge_create_args(adapter, title, body):
    """Builds CLI args for PR/MR creation. Never mutates adapter.cli_args."""
    args = list(adapter.cli_args) + ['--title', title]
    if adapter.forge == 'azure':
        args += ['--description', body]
    else:
        args += ['--body', body]
    return args


def _default_exec_forge_cli(name, args):
    """Runs the forge CLI inheriting stdin/stdout/stderr. Returns error string or None."""
    try:
        result = subprocess.run([name] + args)
        if result.returncode != 0:
            return f"{name} exited with {result.returncode}"
        return None
    except FileNotFoundError:
        return f"{name} not found in PATH"
    except Exception as e:
        return str(e)


def run_ship(
    message='',
    dry_run=False,
    no_pr=False,
    forge_flag='',
    config_forge='',
    repo_dir='',
    avail_fn=None,
    exec_forge_cli=None,
    exec_git=None,
    check_governance=None,
    writeln=None,
):
    """
    Executes the seven-step ship sequence.

    Parameters
    ----------
    message : str
        Commit message (-m). Required; abort if empty.
    dry_run : bool
        Print what would be done without executing write commands.
    no_pr : bool
        Skip PR/MR creation after push.
    forge_flag : str
        Explicit forge override (highest precedence).
    config_forge : str
        Forge value from trackfw.yaml (injected; production: config['forge']).
    repo_dir : str
        Repo root for CI file detection ("" skips CI detection — safe for tests).
    avail_fn : callable(str) -> bool | None
        CLI availability check for forge_adapter. None uses production default.
    exec_forge_cli : callable(str, list[str]) -> str|None
        Runs the forge CLI. Returns error string or None. None uses production default.
    exec_git : callable(list[str]) -> (str, str|None)
        Injected git executor. Returns (stdout, error_or_None).
    check_governance : callable() -> list[str]
        Injected governance check. Returns violation messages.
    writeln : callable(str) -> None
        Injected output writer.

    Returns
    -------
    int
        Exit code: 0 = success, 1 = failure.
    """
    if exec_git is None:
        exec_git = default_exec_git
    if check_governance is None:
        check_governance = check_ship_governance
    if writeln is None:
        writeln = lambda s: print(s)  # noqa: E731
    if exec_forge_cli is None:
        exec_forge_cli = _default_exec_forge_cli

    # Inner git wrapper: skips write commands in dry-run mode.
    def git(args):
        if dry_run and is_git_write_cmd(args):
            writeln(f"[dry-run] git {' '.join(args)}")
            return ('', None)
        return exec_git(args)

    # ─── Step 1: Branch validation ─────────────────────────────────────────
    stdout, err = exec_git(['symbolic-ref', '--short', 'HEAD'])
    if err:
        writeln(f'error: could not determine current branch (are you in a git repo?): {err}')
        return 1
    branch = stdout.strip()

    if branch in ('main', 'master'):
        writeln(
            f'error: trackfw ship cannot run on "{branch}" — use a feature branch:\n'
            '  git checkout -b feat/<slug>'
        )
        return 1

    if not is_ship_branch(branch):
        writeln(
            f'error: branch "{branch}" does not match the required pattern feat|fix|refactor/<slug>\n'
            'Rename your branch or create a new one:\n  git checkout -b feat/<slug>'
        )
        return 1

    writeln(f'Branch: {branch}')

    # ─── Step 2: Governance ────────────────────────────────────────────────
    violations = check_governance()
    if violations:
        writeln('\nGovernance check failed:')
        for v in violations:
            writeln(f'  {v}')
        writeln('\nCreate the required artifacts before running ship:')
        writeln('  trackfw req new "<title>"')
        writeln('  trackfw roadmap new "<title>"')
        writeln('  trackfw roadmap move <name> wip')
        writeln("\nNote: this governance check is a hard gate — it is not affected by lenient")
        writeln("mode or per-rule severity configured in trackfw.yaml. If 'trackfw validate'")
        writeln("passes but 'trackfw ship' aborts here, you likely have lenient mode")
        writeln("configured — ship always requires REQ + roadmap in wip/.")
        writeln(f'\nerror: governance check failed: {len(violations)} violation(s)')
        return 1

    writeln('Governance: OK')

    # ─── Step 3: Squash-merge detection ────────────────────────────────────
    if dry_run:
        writeln('[dry-run] git fetch origin --prune')
    else:
        _, fetch_err = exec_git(['fetch', 'origin', '--prune'])
        if fetch_err:
            writeln('Warning: could not fetch origin (offline or no remote); skipping squash-merge check.')
        else:
            _detect_pending_squash_merges(branch, exec_git, writeln)

    # ─── Step 4: Review staged ─────────────────────────────────────────────
    status_out, _ = exec_git(['status', '--short'])
    diff_stat_out, _ = exec_git(['diff', '--cached', '--stat'])

    writeln('\n── Staged changes ──────────────────────────────────────')
    if status_out:
        writeln(status_out)
    if diff_stat_out:
        writeln(diff_stat_out)
    writeln('────────────────────────────────────────────────────────\n')

    cached_files, _ = exec_git(['diff', '--cached', '--name-only'])
    if not cached_files.strip():
        writeln(
            'error: nothing is staged — stage your files explicitly before running ship:\n'
            '  git add <file1> <file2> ...\n'
            "Never use 'git add .' or 'git add -A'"
        )
        return 1

    # ─── Step 5: Commit ────────────────────────────────────────────────────
    if not message:
        writeln(
            'error: commit message is required — use -m:\n'
            '  trackfw ship -m "feat(<scope>): <description>"'
        )
        return 1

    _, commit_err = git(['commit', '-m', message])
    if commit_err:
        writeln(f'error: git commit failed: {commit_err}')
        return 1

    if not dry_run:
        writeln(f'Committed: {message}')

    # ─── Step 6: Push ──────────────────────────────────────────────────────
    push_args = _build_push_args(branch, exec_git)
    _, push_err = git(push_args)
    if push_err:
        writeln(f'error: git push failed: {push_err}')
        return 1

    if not dry_run:
        writeln(f'Pushed:    {branch} → origin/{branch}')

    # ─── Step 7: Open PR/MR ────────────────────────────────────────────────
    # Resolve forge: flag → config → remote URL → CI files → manual.
    remote_url_out, _ = exec_git(['remote', 'get-url', 'origin'])
    remote_url = (remote_url_out or '').strip()

    try:
        resolution = forge_resolve(
            flag_forge=forge_flag,
            config_forge=config_forge,
            remote_url=remote_url,
            repo_dir=repo_dir,
        )
    except ValueError as res_err:
        writeln(f'Warning: forge resolution error: {res_err} — open PR/MR manually.')
        writeln('\nship complete.')
        return 0

    adapter = forge_adapter(resolution.forge, avail_fn)
    writeln(f'Forge:     {resolution.forge} (source: {resolution.source})')

    if no_pr:
        writeln(f'--no-pr: skipping {adapter.noun} creation.')
        writeln('\nship complete.')
        return 0

    if dry_run:
        if not adapter.available and resolution.forge != 'manual':
            url = adapter.fallback_url(remote_url, branch)
            if url:
                writeln(f'[dry-run] {adapter.noun} CLI ({adapter.cli_name}) not available — would open in browser:\n  {url}')
            else:
                writeln(f'[dry-run] {adapter.noun} CLI ({adapter.cli_name}) not available — would open {adapter.noun} manually')
        else:
            writeln(f'[dry-run] would open {adapter.noun} via {resolution.forge} CLI')
        return 0

    if resolution.forge == 'manual':
        writeln(f'\nOpen your {adapter.noun} manually at:\n  {remote_url}')
        writeln('\nship complete.')
        return 0

    if not adapter.available:
        url = adapter.fallback_url(remote_url, branch)
        if url:
            writeln(f'{adapter.noun} CLI ({adapter.cli_name}) not available — open in browser:\n  {url}')
        else:
            writeln(f'{adapter.noun} CLI ({adapter.cli_name}) not available — open {adapter.noun} manually.')
        writeln('\nship complete.')
        return 0

    # CLI available — invoke it.
    title = _first_line(message)
    body = f'Branch: {branch}\n\nCreated by trackfw ship.'
    cli_args = _build_forge_create_args(adapter, title, body)
    cli_err = exec_forge_cli(adapter.cli_name, cli_args)
    if cli_err:
        url = adapter.fallback_url(remote_url, branch)
        writeln(f'Warning: {adapter.noun} CLI failed ({cli_err}).')
        if url:
            writeln(f'Open in browser:\n  {url}')
    else:
        writeln(f'{adapter.noun} created.')

    writeln('\nship complete.')
    return 0
