"""
push/runner.py — Core implementation of `trackfw push`.

Empurra commits já criados, sem commitar e sem abrir PR/MR.
Reusa os helpers de trackfw.ship.runner; nunca reimplementa a lógica de governança,
detecção de squash-merge ou construção de push args.

Via Python para símbolos privados: importação explícita dos símbolos `_build_push_args` e
`_detect_pending_squash_merges` de ship/runner.py, sem renomear — zero edições em ship/runner.py
e zero risco de regressão em check-ship-parity.sh e check-ship-force-parity.sh. O linter do
projeto é apenas `go vet ./...`, sem linter Python configurado, portanto acesso cross-módulo a
símbolo com prefixo `_` não é sinalizado.

See ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md.
"""

import sys

from trackfw.ship.runner import (
    is_ship_branch,
    is_gated_ship_branch,
    is_git_write_cmd,
    default_exec_git,
    check_ship_governance,
    default_check_pr_open,
    # Private symbols imported explicitly — preferred over renaming to avoid any
    # diff in ship/runner.py (which would risk triggering check-ship-parity.sh).
    _detect_pending_squash_merges,
    _build_push_args,
)

from trackfw.forge.resolve import resolve as forge_resolve
from trackfw.forge.adapter import forge_adapter


# push --force-with-lease refusal messages — "trackfw push" (not "ship") because this command
# closes the commit→push cycle without opening a PR. See
# ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md.
# Defined here (not imported from ship/runner.py) to keep push's user-visible messages
# independent of ship's contract string.
PUSH_FORCE_LEASE_NO_FORGE_CLI_MSG = (
    "trackfw push --force-with-lease requires a forge CLI (gh, glab, or az) to confirm "
    "an open pull/merge request before rewriting remote history. No forge CLI is "
    "available for this repository — install and authenticate it, or push without "
    "--force-with-lease."
)


def push_force_lease_no_pr_open_msg(branch):
    return (
        f'trackfw push --force-with-lease refuses to run: branch "{branch}" has no open '
        "pull/merge request. Open the PR/MR first (trackfw ship without "
        "--force-with-lease, or your forge's web UI), then retry."
    )


def push_force_lease_cannot_verify_msg(branch, cli_name, err_message):
    return (
        f'trackfw push --force-with-lease could not verify whether branch "{branch}" has '
        f"an open pull/merge request ({cli_name} CLI error: {err_message}). Refusing "
        f"rather than risking a force push without a verified PR — check your "
        f"{cli_name} CLI authentication and retry."
    )


def run_push(
    dry_run=False,
    force_with_lease=False,
    config_forge="",
    repo_dir=".",
    # Injectable dependencies for testing.
    exec_git=None,
    check_governance=None,
    check_pr_open=None,
    avail_fn=None,
    out=None,
    err_out=None,
):
    """
    Implements the push sequence: branch validation, governance, force-with-lease gate,
    squash-merge detection, and push. Never commits and never opens a PR/MR.

    Returns int exit code (0 = success).
    """
    if exec_git is None:
        exec_git = default_exec_git
    if check_governance is None:
        check_governance = check_ship_governance
    if out is None:
        out = sys.stdout
    if err_out is None:
        err_out = sys.stderr

    def writeln(s):
        print(s, file=out)

    # "Error: " prefix — mirrors Go exactly: push.go returns messages as errors, and
    # root.go's ExecuteC path emits `cmd.ErrPrefix() + " " + err.Error() + "\n"` —
    # cmd.ErrPrefix() is "Error:", so the combined line is "Error: <message>\n".
    def write_err(s):
        print(f'Error: {s}', file=err_out)  # noqa: E731

    # Inner wrapper: skips write commands in --dry-run mode.
    def git(args):
        if dry_run and is_git_write_cmd(args):
            writeln(f"[dry-run] git {' '.join(args)}")
            return ('', None)
        return exec_git(args)

    # ─── Step 1: Branch validation ────────────────────────────────────────────
    stdout, err = exec_git(['symbolic-ref', '--short', 'HEAD'])
    if err:
        write_err(f'could not determine current branch (are you in a git repo?): {err}')
        return 1
    branch = stdout.strip()

    # main/master is blocked unconditionally.
    if branch in ('main', 'master'):
        write_err(
            f'trackfw push cannot run on "{branch}" — use a feature branch:\n'
            '  git checkout -b feat/<slug>'
        )
        return 1

    if not is_ship_branch(branch):
        write_err(
            f'branch "{branch}" does not match the required pattern feat|fix|refactor|chore|docs/<slug>\n'
            'Rename your branch or create a new one:\n  git checkout -b feat/<slug>'
        )
        return 1

    writeln(f'Branch: {branch}')

    # ─── Step 2: Governance ───────────────────────────────────────────────────
    # push never reads the index — no doc-only exception. Governance is either
    # skipped (chore/docs) or enforced (feat/fix/refactor), nothing in between.
    if not is_gated_ship_branch(branch):
        # chore/docs: housekeeping types already exempted from this gate by
        # `trackfw branch new` and `trackfw commit` — push without it too.
        writeln('Governance: skipped (chore/docs branch)')
    else:
        violations = check_governance()
        if violations:
            writeln('\nGovernance check failed:')
            for v in violations:
                writeln(f'  {v}')
            writeln('\nCreate the required artifacts before running push:')
            writeln('  trackfw req new "<title>"')
            writeln('  trackfw roadmap new "<title>"')
            writeln('  trackfw roadmap move <name> wip')
            writeln("\nNote: this governance check is a hard gate — it is not affected by lenient")
            writeln("mode or per-rule severity configured in trackfw.yaml. If 'trackfw validate'")
            writeln("passes but 'trackfw push' aborts here, you likely have lenient mode")
            writeln("configured — push always requires REQ + roadmap in wip/.")
            write_err(f'governance check failed: {len(violations)} violation(s)')
            return 1

        writeln('Governance: OK')

    # ─── Step 2.5: force-with-lease gate ─────────────────────────────────────
    # Runs before any write (push) — a refusal here never leaves the caller unable to push.
    # Read-only, so it runs in --dry-run too.
    if force_with_lease:
        gate_remote_url_out, _ = exec_git(['remote', 'get-url', 'origin'])
        gate_remote_url = (gate_remote_url_out or '').strip()

        try:
            resolution = forge_resolve(
                flag_forge='',
                config_forge=config_forge,
                remote_url=gate_remote_url,
                repo_dir=repo_dir,
            )
        except ValueError as res_err:
            write_err(str(res_err))
            return 1

        adapter = forge_adapter(resolution.forge, avail_fn)
        if resolution.forge == 'manual' or not adapter.available:
            write_err(PUSH_FORCE_LEASE_NO_FORGE_CLI_MSG)
            return 1

        check_pr_open_fn = check_pr_open or default_check_pr_open
        try:
            open_pr = check_pr_open_fn(adapter, branch)
        except Exception as pr_err:  # noqa: BLE001 — any failure here means "cannot verify"
            write_err(push_force_lease_cannot_verify_msg(branch, adapter.cli_name, pr_err))
            return 1
        if not open_pr:
            write_err(push_force_lease_no_pr_open_msg(branch))
            return 1

        writeln(f'force-with-lease: open {adapter.noun} confirmed for "{branch}" ({resolution.forge}).')

    # ─── Step 3: Squash-merge detection ──────────────────────────────────────
    if dry_run:
        writeln('[dry-run] git fetch origin --prune')
    else:
        _, fetch_err = exec_git(['fetch', 'origin', '--prune'])
        if fetch_err:
            writeln('Warning: could not fetch origin (offline or no remote); skipping squash-merge check.')
        else:
            _detect_pending_squash_merges(branch, exec_git, writeln)

    # ─── Step 4: Push ─────────────────────────────────────────────────────────
    push_args = _build_push_args(branch, exec_git)
    if force_with_lease:
        # Fixed position: push --force-with-lease [-u] origin <branch> — identical
        # across the 3 CLIs (the parity gate compares this literally).
        push_args = [push_args[0], '--force-with-lease'] + push_args[1:]
    _, push_err = git(push_args)
    if push_err:
        write_err(f'git push failed: {push_err}')
        return 1

    if not dry_run:
        writeln(f'Pushed: {branch} → origin/{branch}')

    return 0
