"""
ship/runner.py — Core implementation of `trackfw ship`.

All git write operations are injectable for testability.
Never passes "add ." or "add -A" to any git executor.
"""

import json
import os
import re
import subprocess
import sys

from trackfw import config as _config
from trackfw import validator as _validator
from trackfw.forge.resolve import resolve as forge_resolve
from trackfw.forge.adapter import forge_adapter


# Git subcommands that modify local or remote state.
# In --dry-run mode these are printed but not executed.
GIT_WRITE_COMMANDS = {"commit", "push", "fetch"}

# force-with-lease refusal messages. Named constants so the ML-1B parity gate has a single
# place to compare byte-for-byte across the 3 CLIs. Byte-identical to Go's
# forceLease*Msg/Fmt constants (internal/commands/ship.go) and Node's FORCE_LEASE_* /
# forceLease*Msg() (npm/src/ship/runner.js). Never expose "--force" (raw) as a flag — see
# ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.
FORCE_LEASE_NO_FORGE_CLI_MSG = (
    "trackfw ship --force-with-lease requires a forge CLI (gh, glab, or az) to confirm "
    "an open pull/merge request before rewriting remote history. No forge CLI is "
    "available for this repository — install and authenticate it, or push without "
    "--force-with-lease."
)


def force_lease_no_pr_open_msg(branch):
    return (
        f'trackfw ship --force-with-lease refuses to run: branch "{branch}" has no open '
        "pull/merge request. Open the PR/MR first (trackfw ship without "
        "--force-with-lease, or your forge's web UI), then retry."
    )


def force_lease_cannot_verify_msg(branch, cli_name, err_message):
    return (
        f'trackfw ship --force-with-lease could not verify whether branch "{branch}" has '
        f"an open pull/merge request ({cli_name} CLI error: {err_message}). Refusing "
        f"rather than risking a force push without a verified PR — check your "
        f"{cli_name} CLI authentication and retry."
    )


def default_check_pr_open(adapter, branch):
    """
    Queries the resolved forge CLI for an open PR/MR whose source branch is `branch`, using
    the same list-based query shape for every forge: empty result means "no PR" (exit 0),
    any non-zero exit or unparseable output means "cannot verify" (raised as an exception —
    never conflated with "no PR"). bitbucket and "manual" never reach here: run_ship only
    calls check_pr_open when adapter.available is True, and bitbucket's adapter is always
    available=False.
    """
    if adapter.forge == "github":
        args = ["pr", "list", "--head", branch, "--state", "open", "--json", "number"]
    elif adapter.forge == "gitlab":
        # glab mr list: --source-branch filters by source branch, --state opened matches
        # gh's "open" state, -F json requests machine-readable output (glab's own flag,
        # not an external jq/GNU dependency).
        args = ["mr", "list", "--source-branch", branch, "--state", "opened", "-F", "json"]
    elif adapter.forge == "azure":
        # az defaults to --output json; passed explicitly here for clarity, not reliance
        # on the ambient default.
        args = ["repos", "pr", "list", "--source-branch", branch, "--status", "active", "--output", "json"]
    else:
        raise RuntimeError(f'no PR/MR query defined for forge "{adapter.forge}"')

    try:
        result = subprocess.run([adapter.cli_name] + args, capture_output=True, text=True)
    except FileNotFoundError:
        raise RuntimeError(f"{adapter.cli_name} not found in PATH")

    if result.returncode != 0:
        msg = (result.stderr or "").strip() or f"{adapter.cli_name} exited with {result.returncode}"
        raise RuntimeError(msg)

    try:
        parsed = json.loads(result.stdout or "[]")
    except json.JSONDecodeError as e:
        raise RuntimeError(f"could not parse {adapter.cli_name} output: {e}")

    return isinstance(parsed, list) and len(parsed) > 0


def is_git_write_cmd(args):
    """Returns True when the first arg is a write-mode git subcommand."""
    return len(args) > 0 and args[0] in GIT_WRITE_COMMANDS


def is_ship_branch(branch):
    """
    Returns True when branch matches feat|fix|refactor|chore|docs/<slug> — the full vocabulary
    `trackfw ship` accepts on the branch name. feat/fix/refactor are gated on Step 2's
    branch_has_wip_roadmap governance check (a hard gate not affected by lenient mode);
    chore/docs are housekeeping types — already exempted from that gate by `trackfw branch new`
    and `trackfw commit` — and ship without it too.
    """
    return bool(re.match(r'^(feat|fix|refactor|chore|docs)/.+', branch))


def is_gated_ship_branch(branch):
    """
    Returns True when branch matches feat|fix|refactor/<slug> — the subset of is_ship_branch's
    vocabulary that requires Step 2's branch_has_wip_roadmap governance check. chore/docs
    branches satisfy is_ship_branch but return False here.
    """
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

    Delegates entirely to the shared validator functions already used by `trackfw validate`,
    `trackfw branch new` and `trackfw commit` — never reimplement this logic locally.
    Byte-identical to Go's CheckShipGovernance (internal/validator/validator.go), which has the
    same no-args shape and the same two checks:
      1. validate_branch_has_wip_roadmap — current branch (feat/fix/refactor only; re-derived
         internally from TRACKFW_BRANCH/git, same as Go) has a matching roadmap in wip/ OR done/
      2. validate_wip_has_req — every roadmap in wip/ has a linked REQ
    Before this ML, this function reimplemented both checks locally with its own wording (no
    "nor done/" clause, no done/ directory scan at all) — see
    vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md. Duplicating
    the check was the actual root cause of the drift, not just the wording.

    Returns list of violation messages (empty = pass).
    """
    cfg = _config.load()
    violations = list(_validator.validate_branch_has_wip_roadmap(cfg))
    violations += [v['message'] for v in _validator.validate_wip_has_req(cfg)]
    return violations


def _detect_pending_squash_merges(current_branch, exec_git, writeln):
    """Warns about remote branches with genuinely pending work vs origin/main. Non-blocking.

    Reuses evaluate_branch_integration (trackfw.commands.branch) — the same touched-files
    heuristic `trackfw branch prune` uses — instead of a naive bidirectional
    `git diff origin/main <branch> --stat`. The naive check false-positives on a branch that was
    squash-merged and is now merely stale (main advanced further afterwards): it always shows a
    non-empty diff even though nothing from the branch is actually missing from main. Only
    BRANCH_PRUNE_DECISION_PENDING_WORK — genuine, unintegrated work — surfaces this warning; every
    other decision (no_own_work, content_identical, review_doc_config, no_merge_base, eval_error)
    stays silent, the same posture the naive check had on error (skip, no warning).

    Imported lazily to mirror commands/branch.py's own late import of ship/runner.py (avoids an
    import-time cycle between the two modules).
    """
    from trackfw.commands.branch import (
        evaluate_branch_integration,
        BRANCH_PRUNE_DECISION_PENDING_WORK,
    )

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
        result = evaluate_branch_integration(candidate, exec_git)
        if result['decision'] == BRANCH_PRUNE_DECISION_PENDING_WORK:
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


# commit_message_sep delimits full commit messages (%B) in the output of
# _git_commits_since's `git log --format=%B<sep>`. Same non-printable control
# character used by the Go and Node.js implementations for byte-for-byte parity —
# it cannot appear in a real commit message and survives str.strip().
COMMIT_MESSAGE_SEP = '\x1e'


def _split_nonempty_lines(s):
    """Splits git output into a list of trimmed, non-empty lines. Returns [] for empty input."""
    s = (s or '').strip()
    if not s:
        return []
    return [line.strip() for line in s.split('\n') if line.strip()]


def _all_doc_only(files):
    """
    Returns True when there is at least one staged file and every staged file is
    doc-only: under docs/ or vault/ (path prefix), or has a .md extension. A single
    file outside that criterion makes it return False. Mirrors the doc-only exception
    documented in CLAUDE.md §7 ("Alteração doc-only (markdown, comentários)").
    """
    if not files:
        return False
    for f in files:
        if f.startswith('docs/') or f.startswith('vault/') or f.endswith('.md'):
            continue
        return False
    return True


# _GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX — see Go's ship.go gitSymbolicRefOriginHeadPrefix for the
# rationale: stripping this exact literal prefix (instead of cutting at the last '/') is what
# makes _default_base_branch correct for a default branch that itself contains a slash.
_GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX = 'refs/remotes/origin/'


def _default_base_branch(exec_git):
    """
    Resolves the repository's default branch for `git log <base>..HEAD`.
    Tries `git symbolic-ref refs/remotes/origin/HEAD` and falls back to "main" when that fails
    or yields nothing (e.g. shallow clone without a remote-tracking HEAD).
    """
    stdout, err = exec_git(['symbolic-ref', 'refs/remotes/origin/HEAD'])
    if err:
        return 'main'
    stdout = stdout.strip()
    if not stdout.startswith(_GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX):
        return 'main'
    name = stdout[len(_GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX):]
    return name if name else 'main'


def _git_commits_since(base, exec_git):
    """
    Returns the full message (subject + body) of every non-merge commit in
    base..HEAD, most-recent-first (git log's natural order). Returns [] on any git
    error or when the range is empty.
    """
    stdout, err = exec_git(['log', f'{base}..HEAD', '--no-merges', f'--format=%B{COMMIT_MESSAGE_SEP}'])
    if err:
        return []
    stdout = stdout.strip()
    if not stdout:
        return []
    commits = []
    for part in stdout.split(COMMIT_MESSAGE_SEP):
        part = part.strip('\n')
        if part.strip():
            commits.append(part)
    return commits


def build_pr_body(branch, commits):
    """
    Constructs the PR/MR body. With 0 or 1 non-merge commit on the branch (the
    trivial case — just the commit `ship` itself made), it keeps the original minimal
    body, not a regression. With 2+ commits, it aggregates the branch's commit
    history:

        ## Commits
        - <subject of commit 1>
        - <subject of commit 2>

        ## Detalhes
        <full body of each commit that has one, in blocks>

        ---
        Branch: <branch>
    """
    if len(commits) <= 1:
        return f'Branch: {branch}\n\nCreated by trackfw ship.'

    subjects = []
    details = []
    for c in commits:
        lines = c.split('\n', 1)
        subject = lines[0].strip()
        if not subject:
            continue
        subjects.append(subject)
        if len(lines) > 1:
            body_text = lines[1].strip()
            if body_text:
                details.append(f'**{subject}**\n\n{body_text}')

    out = ['## Commits\n\n']
    for s in subjects:
        out.append(f'- {s}\n')
    if details:
        out.append('\n## Detalhes\n\n')
        out.append('\n\n'.join(details))
        out.append('\n')
    out.append(f'\n---\nBranch: {branch}\n')
    return ''.join(out)


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
    write_err=None,
    force_with_lease=False,
    check_pr_open=None,
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
        Injected output writer (stdout).
    write_err : callable(str) -> None
        Injected error writer (stderr) for the terminal error of every abort path below. Mirrors
        Go exactly: ship.go returns these same messages as a bare exception, and
        internal/commands/root.go's Execute() wrapper prints them to stderr as
        `Error: <message>` (cmd.ErrPrefix() is "Error:", Fprintln inserts one space). Every
        multi-line detail printed BEFORE the abort (violation lists, remediation hints,
        "Note: ..." blocks) stays on stdout via writeln, same as Go's deps.out — only this final
        one-line (or one-block) summary moves to stderr.
    force_with_lease : bool
        --force-with-lease: governed force-push (git push --force-with-lease). Only runs
        when the branch already has an open PR/MR, verified via check_pr_open. When
        nothing is staged (e.g. after a rebase that already committed the result), skips
        the commit step entirely instead of requiring -m.
    check_pr_open : callable(adapter, str) -> bool | None
        Injected PR/MR-open check for the resolved forge. Raises on "cannot verify"
        (CLI error / unparseable output) — never returns False for that case. None uses
        default_check_pr_open.

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
    if write_err is None:
        write_err = lambda s: print(f'Error: {s}', file=sys.stderr)  # noqa: E731
    if exec_forge_cli is None:
        exec_forge_cli = _default_exec_forge_cli

    # Inner git wrapper: skips write commands in dry-run mode.
    def git(args):
        if dry_run and is_git_write_cmd(args):
            writeln(f"[dry-run] git {' '.join(args)}")
            return ('', None)
        return exec_git(args)

    # ─── Step 0: staged files ───────────────────────────────────────────────
    # Read once, up front, so Steps 1 and 2 can grant a doc-only exception before
    # they run — and so Step 4 below reuses the same read instead of querying git
    # twice.
    staged_out, _ = exec_git(['diff', '--cached', '--name-only'])
    staged_files = _split_nonempty_lines(staged_out)
    doc_only = _all_doc_only(staged_files)

    # ─── Step 1: Branch validation ─────────────────────────────────────────
    stdout, err = exec_git(['symbolic-ref', '--short', 'HEAD'])
    if err:
        write_err(f'could not determine current branch (are you in a git repo?): {err}')
        return 1
    branch = stdout.strip()

    # main/master is blocked unconditionally — the doc-only exception never applies here.
    if branch in ('main', 'master'):
        write_err(
            f'trackfw ship cannot run on "{branch}" — use a feature branch:\n'
            '  git checkout -b feat/<slug>'
        )
        return 1

    if not doc_only and not is_ship_branch(branch):
        write_err(
            f'branch "{branch}" does not match the required pattern feat|fix|refactor|chore|docs/<slug>\n'
            'Rename your branch or create a new one:\n  git checkout -b feat/<slug>'
        )
        return 1

    writeln(f'Branch: {branch}')

    # ─── Step 2: Governance ────────────────────────────────────────────────
    # Doc-only changes (all staged files under docs/, vault/, or *.md) are exempt
    # from REQ+roadmap governance — mirrors the CLAUDE.md §7 exception for doc-only
    # changes.
    if doc_only:
        writeln('Governance: skipped (doc-only change)')
    elif is_ship_branch(branch) and not is_gated_ship_branch(branch):
        # chore/docs: housekeeping types already exempted from this gate by
        # `trackfw branch new` and `trackfw commit` — ship without it too.
        writeln('Governance: skipped (chore/docs branch)')
    else:
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
            write_err(f'governance check failed: {len(violations)} violation(s)')
            return 1

        writeln('Governance: OK')

    # ─── Step 2.5: force-with-lease gate ────────────────────────────────────
    # Runs before any write (commit/push) — a refusal here must never leave a local
    # commit the caller cannot push. Read-only, so it runs in --dry-run too, same
    # posture as the read-only calls in Step 0 / Step 7.
    #
    # force_lease_adapter/force_lease_resolution are reused by Step 7 below to avoid a
    # second forge resolution and a duplicate "Forge: ..." line, and because Step 7
    # must skip PR/MR creation entirely once this gate has confirmed one is already
    # open — creating a second one would be a spurious failure on every successful
    # force push.
    force_lease_adapter = None
    force_lease_resolution = None
    if force_with_lease:
        gate_remote_url_out, _ = exec_git(['remote', 'get-url', 'origin'])
        gate_remote_url = (gate_remote_url_out or '').strip()

        try:
            resolution = forge_resolve(
                flag_forge=forge_flag,
                config_forge=config_forge,
                remote_url=gate_remote_url,
                repo_dir=repo_dir,
            )
        except ValueError as res_err:
            write_err(str(res_err))
            return 1

        adapter = forge_adapter(resolution.forge, avail_fn)
        if resolution.forge == 'manual' or not adapter.available:
            write_err(FORCE_LEASE_NO_FORGE_CLI_MSG)
            return 1

        check_pr_open_fn = check_pr_open or default_check_pr_open
        try:
            open_pr = check_pr_open_fn(adapter, branch)
        except Exception as pr_err:  # noqa: BLE001 — any failure here means "cannot verify"
            write_err(force_lease_cannot_verify_msg(branch, adapter.cli_name, pr_err))
            return 1
        if not open_pr:
            write_err(force_lease_no_pr_open_msg(branch))
            return 1

        writeln(f'force-with-lease: open {adapter.noun} confirmed for "{branch}" ({resolution.forge}).')
        force_lease_adapter = adapter
        force_lease_resolution = resolution

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

    # Reuses staged_files read at the top of the function (Step 0) — never re-query
    # git here.
    #
    # --force-with-lease push-only mode: a rebase that resolved conflicts already
    # committed the result (the index is clean afterwards) — there is nothing left to
    # stage or commit, only to push. Only --force-with-lease grants this exception;
    # without it, "nothing staged" still aborts exactly as before (non-regression).
    push_only = force_with_lease and not staged_files

    if not staged_files and not force_with_lease:
        write_err(
            'nothing is staged — stage your files explicitly before running ship:\n'
            '  git add <file1> <file2> ...\n'
            "Never use 'git add .' or 'git add -A'"
        )
        return 1

    # ─── Step 5: Commit ────────────────────────────────────────────────────
    if push_only:
        writeln('Nothing staged — --force-with-lease pushes existing commits only, no new commit.')
    else:
        if not message:
            write_err(
                'commit message is required — use -m:\n'
                '  trackfw ship -m "feat(<scope>): <description>"'
            )
            return 1

        _, commit_err = git(['commit', '-m', message])
        if commit_err:
            write_err(f'git commit failed: {commit_err}')
            return 1

        if not dry_run:
            writeln(f'Committed: {message}')

    # ─── Step 6: Push ──────────────────────────────────────────────────────
    push_args = _build_push_args(branch, exec_git)
    if force_with_lease:
        # Fixed position: push --force-with-lease [-u] origin <branch> — identical
        # across the 3 CLIs (ML-1B's parity gate compares this literally).
        push_args = [push_args[0], '--force-with-lease'] + push_args[1:]
    _, push_err = git(push_args)
    if push_err:
        write_err(f'git push failed: {push_err}')
        return 1

    if not dry_run:
        writeln(f'Pushed:    {branch} → origin/{branch}')

    # ─── Step 7: Open PR/MR ────────────────────────────────────────────────
    # --force-with-lease only ever reaches here after Step 2.5 confirmed a PR/MR is
    # already open on this branch — creating another one would be a spurious failure
    # on every successful force push. Reuses the adapter/resolution Step 2.5 already
    # computed instead of resolving the forge a second time.
    if force_with_lease:
        writeln(f'Forge:     {force_lease_resolution.forge} (source: {force_lease_resolution.source})')
        writeln(f'{force_lease_adapter.noun} already open — skipping creation (--force-with-lease).')
        writeln('\nship complete.')
        return 0

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

    # Title/body computed once for every remaining branch below (dry-run and real CLI
    # invocation alike). git log/diff are read-only — they run in --dry-run mode too,
    # same as the staged-files read in Step 0.
    #
    # Design decision (documented per roadmap ML-1A, ported from Go): the title is
    # always _first_line(message), the -m message passed to this very `ship` call,
    # even when the branch carries multiple prior commits. Deriving a distinct "PR
    # title" from N unrelated commit subjects would need a heuristic with no
    # unambiguous answer — the simplest, least surprising rule is that -m is the PR's
    # summary.
    base = _default_base_branch(exec_git)
    commits = _git_commits_since(base, exec_git)
    title = _first_line(message)
    body = build_pr_body(branch, commits)

    if dry_run:
        writeln(f'[dry-run] Title: {title}')
        writeln(f'[dry-run] Body:\n{body}')
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
