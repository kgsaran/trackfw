"""
commands/update.py — trackfw update (Python CLI).

Scope: the current repository only (docs/cli-parity.md, "`trackfw update`
vs `trackfw update harness`"). This command never mutates global state —
every write below is rooted at `cwd`, and the Codex integration block below
plans/applies with `scope="project"` explicitly. `trackfw update harness`
(trackfw/commands/update_harness.py) is the counterpart that refreshes the
user's global harness (`~/.claude`, `~/.codex`, etc.) and runs from
anywhere, without a `trackfw.yaml`.

--dry-run, --json, --targets and --install-missing (added for
REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador, ML-6F/ML-6H)
report the same four-state model (updated/skipped/missing/failed) as
`trackfw update harness`, over a "scope": "project" JSON document — see
docs/cli-parity.md.

PROJECT_TARGET_IDS declares this runtime's base project-scope target order:
`agent-rules`, `agent-hooks`, `codex-project-agents`, `validate-script`,
`claude-commands` — the same 5 ids, same order, as Go's and Node.js's
declared list minus the config-conditional entries (docs/cli-parity.md,
"Declared project targets — pinned list").

`ci-workflow` (ML-2C, REQ-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-
install-sh-honrando-trackfw-version) closes what was previously a Go/Node.js-
only gap: this runtime now generates, and `update` now manages, the pinned
CI workflow (see trackfw/generators/init_gen.py's
build_github_actions_workflow_content/build_gitlab_ci_workflow_content and
generate_ci_workflow) whenever the project's trackfw.yaml declares
`ci: github-actions` or `ci: gitlab-ci` — same condition, same relative
position (right after `validate-script`), as Go's ProjectTargetIDs and
Node's PROJECT_TARGET_IDS. `project_target_ids(cfg)` below computes the
per-invocation declared list; PROJECT_TARGET_IDS is the ci-workflow-less
base used when no cfg is available yet (e.g. validating --targets before
trackfw.yaml is read).

`git-hooks` remains Go/Node.js-only: this runtime still has no CLI surface
to configure a git-hooks framework at `init` time (no --hooks flag — see
trackfw/commands/init.py), so there is nothing for that target to manage
here; it is correctly absent, not silently shortened. Per contract, a
runtime that cannot manage a target still declares it and reports an honest
state rather than omitting it — `validate-script` and `claude-commands` are
both fully implementable in this runtime (see
trackfw/generators/init_gen.py's generate_validate_script and
generate_claude_commands, the same generators `trackfw init` uses) and are
implemented below.
"""

from __future__ import annotations

import argparse
import contextlib
import glob
import hashlib
import json
import os
import shutil
import sys
import tempfile
from pathlib import Path
from typing import Any

from trackfw import config as project_config
from trackfw.commands import update_harness
from trackfw.commands.update_harness import (
    STATE_FAILED,
    STATE_MISSING,
    STATE_SKIPPED,
    STATE_UPDATED,
)
from trackfw.generators.adr import global_adr_dir
from trackfw.homedir import home_dir

AGENT_RULES_RELATIVE_PATHS = [
    "CLAUDE.md",
    "AGENTS.md",
    "GEMINI.md",
    os.path.join(".github", "copilot-instructions.md"),
    ".windsurfrules",
    os.path.join(".amazonq", "developer", "guidelines.md"),
    os.path.join(".cursor", "rules", "trackfw.mdc"),
]

AGENT_HOOKS_RELATIVE_PATHS = [
    os.path.join(".claude", "settings.json"),
    os.path.join(".codex", "hooks.json"),
    os.path.join(".gemini", "settings.json"),
    os.path.join(".kiro", "hooks", "trackfw-attention.json"),
    os.path.join(".github", "hooks", "trackfw-attention.json"),
    os.path.join(".cursor", "hooks.json"),
    os.path.join("scripts", "trackfw-attention-signal.sh"),
    os.path.join("scripts", "trackfw-attention-cleanup.sh"),
    os.path.join("scripts", "trackfw-credential-guard.sh"),
    os.path.join("scripts", "trackfw-git-branch-guard.sh"),       # Gap F: was missing
    os.path.join(".windsurf", "hooks.json"),                       # Gap A: InjectWindsurfHooks writes this
    os.path.join(".amazonq", "cli-agents", "q_cli_default.json"), # Gap B: InjectAmazonQHooks writes this
]

# AGENT_HOOKS_DISPLAY_PATH — the reported `path` string for the agent-hooks
# target. Pinned to match Go's and Node.js's rendering exactly: the two
# per-file attention scripts are collapsed into a single glob
# ("scripts/trackfw-attention-*.sh"), not spelled out individually.
# Rewritten as an explicit string (previously used a positional [:-3] slice
# that broke whenever a path was inserted into AGENT_HOOKS_RELATIVE_PATHS).
AGENT_HOOKS_DISPLAY_PATH = (
    ".claude/settings.json, .codex/hooks.json, .gemini/settings.json"
    ", .kiro/hooks/trackfw-attention.json, .github/hooks/trackfw-attention.json"
    ", .cursor/hooks.json, scripts/trackfw-attention-*.sh"
    ", scripts/trackfw-credential-guard.sh, scripts/trackfw-git-branch-guard.sh"
    ", .windsurf/hooks.json, .amazonq/cli-agents/q_cli_default.json"
)

CODEX_PROJECT_AGENTS_DISPLAY_PATH = ".codex/agents, .agents/skills"

VALIDATE_SCRIPT_RELATIVE_PATH = os.path.join("scripts", "trackfw-validate.sh")
CLAUDE_COMMANDS_RELATIVE_PATH = os.path.join(".claude", "commands", "trackfw")

# ci-workflow (ML-2C) — both possible destinations are declared as relPaths;
# generate_ci_workflow only ever writes the one matching cfg["ci"], so the
# other stays absent (hashed as None on both sides, contributing nothing to
# the before/after diff) — mirrors Go's runFileTarget relPaths for
# "ci-workflow" (internal/generators/update.go), which declares both paths
# for the same reason.
#
# ML-2G (AC17) added a THIRD relPath: .github/workflows/trackfw-validate.yml
# (DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH, trackfw.commands.discover) — a
# different file, owned by `trackfw discover --init`, that `update` now
# refreshes-only-if-present (AC17(a)/(b)) alongside the trackfw-gate.yml/
# .gitlab-ci-trackfw.yml pair above.
CI_WORKFLOW_RELATIVE_PATHS = [
    os.path.join(".github", "workflows", "trackfw-gate.yml"),
    ".gitlab-ci-trackfw.yml",
    os.path.join(".github", "workflows", "trackfw-validate.yml"),
]
CI_WORKFLOW_DISPLAY_PATH = (
    ".github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml, .github/workflows/trackfw-validate.yml"
)

# PROJECT_TARGET_IDS — this runtime's base declared project-scope target
# list, without the config-conditional "ci-workflow" entry. See module
# docstring and project_target_ids() below, which is what callers with a
# loaded cfg should use — this constant remains for callers that only need
# the ci-workflow-less base (e.g. the "unknown target id" message shown
# before trackfw.yaml is read).
PROJECT_TARGET_IDS = [
    "agent-rules",
    "agent-hooks",
    "codex-project-agents",
    "validate-script",
    "claude-commands",
]


def project_target_ids(cfg: dict[str, str] | None, discover_workflow_present: bool = False) -> list[str]:
    """Returns the declared project-scope target ids for this invocation,
    inserting "ci-workflow" right after "validate-script" — same relative
    position as Go's ProjectTargetIDs (internal/generators/update.go) and
    Node's PROJECT_TARGET_IDS (npm/src/commands/update.js) — when cfg["ci"]
    is "github-actions" or "gitlab-ci" OR (AC17(c), REQ-2026-08-28, ML-2G)
    trackfw-validate.yml (written by `trackfw discover --init`, an
    independent install mechanism — DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH in
    trackfw.commands.discover) already exists on disk — that second clause
    lets `update` manage a discover-installed workflow even in a `ci: none`
    project, closing the gap where that file was otherwise outside any
    command's management and the doctor's `trackfw update` remedy for it was
    inert. `git-hooks` is never added: this runtime has no `init`-time
    surface to configure a hooks framework."""
    ids = ["agent-rules", "agent-hooks", "codex-project-agents", "validate-script"]
    if (cfg and cfg.get("ci") in ("github-actions", "gitlab-ci")) or discover_workflow_present:
        ids.append("ci-workflow")
    ids.append("claude-commands")
    return ids


def _discover_workflow_present(cwd: str) -> bool:
    """Reports whether .github/workflows/trackfw-validate.yml already exists
    under cwd AS A REGULAR FILE. Used both to decide whether "ci-workflow"
    is declared (AC17(c)) and, inside its apply, to decide whether to
    refresh it — existence is always checked against the real cwd, never
    the --dry-run sandbox, mirroring how cfg itself is read from the real
    cwd before the sandbox is built.

    Checks os.path.islink FIRST, before os.path.isfile: os.path.isfile
    follows symlinks, so a live symlink whose target resolves to a regular
    file would be reported "present" purely because the link happens to
    resolve — pulling "ci-workflow" into the declared target set on the
    strength of a link this command does not own. Symlinks are therefore
    treated as NOT present here: `update` will not declare/manage a target
    on their account, and
    _refresh_discover_github_actions_workflow_if_present below refuses to
    write through them regardless."""
    from trackfw.commands.discover import DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH

    dest = os.path.join(cwd, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
    if os.path.islink(dest):
        return False
    return os.path.isfile(dest)


def _refresh_discover_github_actions_workflow_if_present(root: str) -> None:
    """Refreshes .github/workflows/trackfw-validate.yml ONLY when it already
    exists under root as a REGULAR FILE — `update` never creates this file
    (AC17(b)): ownership of the install decision belongs to `trackfw
    discover --init`, not `update`. Writes the SAME builder scaffold doctor
    compares against (build_discover_github_actions_workflow_content,
    trackfw.commands.discover) so what `update` writes and what `doctor`
    expects can never drift apart by construction (REQ-2026-08-28 AC17).

    Checks os.path.islink FIRST: this path is the most sensitive one
    `update` can write to (it controls what runs in CI for anyone who
    checks the project out), so if it is a symlink — live or dangling —
    this function refuses to write through it. Refusing is loud (stderr),
    never silent, so "update didn't refresh my workflow" stays diagnosable
    instead of a silent no-op."""
    from trackfw.commands.discover import (
        DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
        build_discover_github_actions_workflow_content,
    )

    dest = os.path.join(root, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
    if os.path.islink(dest):
        print(
            f"aviso: {DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} é um symlink; "
            "trackfw update não escreve através de symlinks — arquivo não foi tocado",
            file=sys.stderr,
        )
        return
    if not os.path.isfile(dest):
        return  # not installed — update never creates it (AC17(b))
    with open(dest, "w", encoding="utf-8", newline="\n") as f:
        f.write(build_discover_github_actions_workflow_content())


# ---------------------------------------------------------------------------
# AC6 (REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis) —
# this runtime's `update` historically never read hooks/ci/backend/frontend/pkg_manager at all
# (grep -rn pkg_manager pypi/trackfw returned empty before this change). Closes that specific
# functional gap for `hooks`, mirroring Go's updateHooksSurgical (internal/generators/update.go)
# and Node's updateHooksSurgical (npm/src/commands/update.js) message-for-message: ensures
# "trackfw validate" is present in the detected hook framework's config without overwriting
# user content. `ci`/`backend`/`frontend`/`pkg_manager` are read (closing the "lê os cinco
# campos" half of AC6) but intentionally left unacted upon here — see the module docstring and
# docs/cli-parity.md, "Declared project targets — pinned list": this runtime's declared 5-id
# project-scope target set is a documented, intentional reduction versus Go/Node.js (no --ci/
# --hooks flags at `init` time), and `ci-workflow`/`git-hooks` are NOT part of that pinned list —
# adding them would be an undeclared expansion of the target contract, which is Wave 4's
# (docs/cli-parity.md) territory, not this microlote's.
# ---------------------------------------------------------------------------


def _load_update_config(cwd: str) -> dict[str, str]:
    """Reads the 5 fields `trackfw update` cares about via the single config loader
    (trackfw.config, see ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-
    tipados.md) instead of a second, artisanal read of trackfw.yaml. config.load() reads
    relative to the given cwd (unlike Go's process-cwd-only Load()), so no chdir is required."""
    return dict(project_config.load(cwd)["update"])


def _update_hooks_surgical(hooks: str, root_dir: str) -> None:
    """Ensures 'trackfw validate' is present in the detected hook framework's config without
    overwriting user content — mirrors Go's updateHooksSurgical and Node's updateHooksSurgical
    (same detection, same injected text, same printed messages)."""
    if hooks == "husky":
        hook_path = os.path.join(root_dir, ".husky", "pre-commit")
        content = ""
        if os.path.isfile(hook_path):
            with open(hook_path, "r", encoding="utf-8") as f:
                content = f.read()
        if "trackfw validate" in content:
            print("  ✓ .husky/pre-commit — trackfw validate já presente")
            return
        os.makedirs(os.path.join(root_dir, ".husky"), exist_ok=True)
        with open(hook_path, "a", encoding="utf-8", newline="\n") as f:
            f.write("\ntrackfw validate\n")
        try:
            os.chmod(hook_path, 0o755)
        except OSError:
            pass
        print("  ✓ .husky/pre-commit — trackfw validate injetado")
    elif hooks == "lefthook":
        lefthook_path = os.path.join(root_dir, "lefthook.yml")
        content = ""
        if os.path.isfile(lefthook_path):
            with open(lefthook_path, "r", encoding="utf-8") as f:
                content = f.read()
        if "trackfw-validate:" in content or "trackfw validate" in content:
            print("  ✓ lefthook.yml — trackfw já presente")
            return
        with open(lefthook_path, "a", encoding="utf-8", newline="\n") as f:
            f.write("\npre-commit:\n  commands:\n    trackfw-validate:\n      run: trackfw validate\n")
        print("  ✓ lefthook.yml — trackfw-validate injetado")


def _adr_dirs_entry_present(content: str, abs_global_dir: str) -> bool:
    """Reports whether content's adr_dirs block (if any) already has an item
    resolving to the global ADR dir — matching both the literal
    "~/.trackfw/adr" form and the expanded absolute-path form so the two
    textual spellings of the same entry are never treated as distinct.
    Mirrors Go's adrDirsEntryPresent and Node's resolvesToGlobal loop."""
    lines = content.split("\n")
    in_adr_dirs = False
    for line in lines:
        trimmed = line.rstrip(" \t")
        if trimmed.lstrip(" ").startswith("adr_dirs:"):
            in_adr_dirs = True
            continue
        if in_adr_dirs:
            item_line = trimmed.lstrip(" ")
            if not item_line.startswith("-"):
                break  # list ended
            value = item_line[1:].strip()
            if value == "~/.trackfw/adr" or value == abs_global_dir:
                return True
    return False


def _insert_global_adr_dir_entry(content: str) -> str:
    """Returns content with "  - ~/.trackfw/adr" inserted as the last item of
    the existing adr_dirs list, or — if content has no adr_dirs key at all
    (implying the loader's implicit "docs/adr" default) — with a new
    adr_dirs block appended at the end preserving that default explicitly
    alongside the new global entry. Mirrors Go's insertGlobalADRDirEntry and
    Node's equivalent splice."""
    lines = content.split("\n")
    adr_dirs_idx = -1
    for i, line in enumerate(lines):
        if line.rstrip(" \t").lstrip(" ").startswith("adr_dirs:"):
            adr_dirs_idx = i
            break

    if adr_dirs_idx == -1:
        if not content.endswith("\n"):
            content += "\n"
        if not content.endswith("\n\n") and content != "\n":
            content += "\n"
        content += "adr_dirs:\n  - docs/adr\n  - ~/.trackfw/adr\n"
        return content

    item_indent = "  "
    last_item_idx = adr_dirs_idx
    for i in range(adr_dirs_idx + 1, len(lines)):
        trimmed = lines[i].rstrip(" \t")
        left_trimmed = trimmed.lstrip(" ")
        if not left_trimmed.startswith("-"):
            break
        item_indent = trimmed[: len(trimmed) - len(left_trimmed)]
        last_item_idx = i

    new_line = item_indent + "- ~/.trackfw/adr"
    out = lines[: last_item_idx + 1] + [new_line] + lines[last_item_idx + 1 :]
    return "\n".join(out)


def _ensure_global_adr_dir_registered(cwd: str) -> None:
    """Registers ~/.trackfw/adr in the project's trackfw.yaml `adr_dirs`
    list, but ONLY when that directory exists AND contains at least one
    `ADR-*.md` file — an empty or absent global ADR dir is a no-op, never
    written. The edit is surgical (text-level splice, never
    config.load()+re-serialize, which would lose the user's
    comments/formatting) and idempotent. Mirrors Go's
    ensureGlobalADRDirRegistered (internal/generators/update.go) and Node's
    ensureGlobalAdrDirRegistered (npm/src/commands/update.js) message-for-
    message."""
    home = home_dir()
    global_dir = global_adr_dir(home)
    if not os.path.isdir(global_dir):
        return  # global ADR dir doesn't exist — no-op

    matches = glob.glob(os.path.join(global_dir, "ADR-*.md"))
    if not matches:
        return  # global ADR dir has no ADRs yet — no-op

    yaml_path = os.path.join(cwd, "trackfw.yaml")
    try:
        with open(yaml_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return

    abs_global_dir = os.path.join(home, ".trackfw", "adr")
    if _adr_dirs_entry_present(content, abs_global_dir):
        return  # already registered (literal "~/.trackfw/adr" or the expanded absolute path)

    updated = _insert_global_adr_dir_entry(content)
    with open(yaml_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(updated)
    print("  ✓ adr_dirs: ~/.trackfw/adr registrado")


def register(subparsers: argparse.ArgumentParser) -> None:
    parser = subparsers.add_parser(
        "update",
        help="Update trackfw rules in agent config files (agent rules only)",
    )
    parser.add_argument("--dry-run", action="store_true", help="Compute and report states without writing anything")
    parser.add_argument("--json", action="store_true", help="Emit the result document instead of the text report")
    parser.add_argument("--targets", help="Comma-separated subset of target ids")
    parser.add_argument(
        "--install-missing",
        action="store_true",
        help="Allow missing targets to be installed instead of merely reported",
    )
    # `update_action` is optional (required=False, the argparse default) so
    # bare `trackfw update` keeps running `_run` below via `set_defaults`.
    # Only when the user types `trackfw update harness` does the child
    # parser's own `set_defaults(func=...)` override it.
    update_actions = parser.add_subparsers(dest="update_action")
    update_harness.register(update_actions)
    parser.set_defaults(func=_dispatch)


def _dispatch(args: argparse.Namespace) -> None:
    if args.dry_run or args.json or args.targets or args.install_missing:
        _run_project(args)
        return
    _run(args)


def _run(args: argparse.Namespace) -> None:
    cwd = os.getcwd()
    yaml_path = os.path.join(cwd, "trackfw.yaml")

    if not os.path.exists(yaml_path):
        print("Erro: trackfw.yaml não encontrado — execute trackfw init primeiro")
        raise SystemExit(1)

    _ensure_global_adr_dir_registered(cwd)

    print("trackfw update — atualizando regras de agente...\n")

    update_cfg = _load_update_config(cwd)
    if update_cfg.get("hooks") in ("husky", "lefthook"):
        _update_hooks_surgical(update_cfg["hooks"], cwd)

    from trackfw.generators.init_gen import inject_rules_detected
    try:
        inject_rules_detected(cwd)
        print("  Regras de agente atualizadas (CLAUDE.md, GEMINI.md, etc.)")
    except Exception as e:
        print(f"  Aviso: falha ao atualizar regras: {e}")

    from trackfw.generators.hooks import inject_hooks_detected
    try:
        inject_hooks_detected(cwd)
        print('  ✓ agent hooks atualizados')
    except Exception as e:
        print(f'  ⚠ agent hooks: {e}')

    if update_cfg.get("ci") in ("github-actions", "gitlab-ci"):
        from trackfw.generators.init_gen import generate_ci_workflow
        try:
            generate_ci_workflow(cwd, update_cfg)
        except Exception as e:
            print(f'  ⚠ ci workflow: {e}')

    # discover-installed CI workflow (.github/workflows/trackfw-validate.yml),
    # present regardless of update_cfg["ci"] (AC17(c), REQ-2026-08-28) — same
    # shared writer _run_project's "ci-workflow" target uses, so the simple
    # `trackfw update` path and the `--targets ci-workflow` path can never
    # drift apart on what "refreshed" means. No-ops when the file isn't
    # present (AC17(b) — update never installs it, only `trackfw discover
    # --init` does).
    try:
        _refresh_discover_github_actions_workflow_if_present(cwd)
        if _discover_workflow_present(cwd):
            print('  ✓ CI workflow (discover) atualizado')
    except Exception as e:
        print(f'  ⚠ ci workflow (discover): {e}')

    if os.path.exists(os.path.join(cwd, "AGENTS.md")) or os.path.isdir(os.path.join(cwd, ".codex")):
        from trackfw import identity
        from trackfw.identity import IdentityError

        # Identity errors must abort the command — never fall back silently
        # to the neutral default, which would revert the user's identity.
        try:
            ident = identity.load(home_dir())
        except IdentityError as e:
            print(f"update: identidade invalida: {e}")
            raise SystemExit(2) from e

        try:
            from trackfw.integrations.catalog import plan_deployments
            from trackfw.integrations.manager import IntegrationManager
            manager = IntegrationManager(cwd)
            _am = project_config.load(cwd).get("agent_models", {})
            _, plans = plan_deployments("agents", target_ids=["codex"], scope="project", identity_cfg=ident, agent_models=_am)
            plans = [plan for plan, status in zip(plans, manager.list(plans)) if status["state"] != "not-installed"]
            manager.update(plans)
            _, plans = plan_deployments("skills", target_ids=["codex"], scope="project", identity_cfg=ident, agent_models=_am)
            plans = [plan for plan, status in zip(plans, manager.list(plans)) if status["state"] != "not-installed"]
            manager.update(plans)
        except Exception as e:
            print(f"  ⚠ Codex integration: {e}")

    print()
    print("  Nota: este CLI Python atualiza regras de agente, git hooks (husky/lefthook) e o")
    print("  workflow de CI pinado (github-actions/gitlab-ci), quando declarado em trackfw.yaml.")
    print("  Para atualizar Claude commands, use:")
    print("    trackfw update   (CLI Go)")
    print("    npx trackfw update   (CLI Node.js)")

    print("\ntrackfw update concluído")
    try:
        from trackfw.generators.init_gen import print_architect_next_steps
        print_architect_next_steps(cwd)
    except Exception:
        pass


# ---------------------------------------------------------------------------
# --dry-run / --json / --targets / --install-missing — four-state model,
# mirroring internal/generators/update.go's runFileTarget and
# npm/src/lib/update-engine.js's runFileTarget for this runtime's reduced
# project-scope target set.
# ---------------------------------------------------------------------------


def _hash_path(path: str) -> str | None:
    """Returns None when path does not exist, a content hash for a file, or
    a hash of the recursive (relative-path, content-hash) listing for a
    directory."""
    if not os.path.exists(path):
        return None
    if os.path.isfile(path):
        return hashlib.sha256(Path(path).read_bytes()).hexdigest()
    entries = []
    for root, _dirs, files in os.walk(path):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, path)
            entries.append(f"{rel}:{hashlib.sha256(Path(full).read_bytes()).hexdigest()}")
    entries.sort()
    return hashlib.sha256("\n".join(entries).encode("utf-8")).hexdigest()


def _hash_rel_paths(root: str, rel_paths: list[str]) -> list[str | None]:
    return [_hash_path(os.path.join(root, rel)) for rel in rel_paths]


def _all_missing(hashes: list[str | None]) -> bool:
    return all(h is None for h in hashes)


def _run_file_target(
    target_id: str,
    display_path: str,
    root: str,
    rel_paths: list[str],
    apply,
    dry_run: bool,
    install_missing: bool,
) -> dict[str, Any]:
    """Computes updated/skipped/missing/failed for a target whose only
    observable effect is writing under rel_paths (relative to root), by
    diffing content hashes before/after invoking apply(root). "missing"
    never installs: apply is never called when every rel_path is absent and
    install_missing is not set."""
    before = _hash_rel_paths(root, rel_paths)
    if _all_missing(before) and not install_missing:
        return {"id": target_id, "state": STATE_MISSING, "path": display_path}

    try:
        apply(root)
    except Exception as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    after = _hash_rel_paths(root, rel_paths)
    if _all_missing(before) and _all_missing(after):
        return {"id": target_id, "state": STATE_MISSING, "path": display_path}
    if before == after:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _codex_project_agents_target(root: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    detected = os.path.exists(os.path.join(root, "AGENTS.md")) or os.path.isdir(os.path.join(root, ".codex"))
    if not detected:
        return {"id": "codex-project-agents", "state": STATE_MISSING, "path": CODEX_PROJECT_AGENTS_DISPLAY_PATH}

    try:
        from trackfw import identity
        from trackfw.identity import IdentityError
        from trackfw.integrations.catalog import plan_deployments
        from trackfw.integrations.manager import IntegrationManager

        try:
            ident = identity.load(home_dir())
        except IdentityError as error:
            raise RuntimeError(f"identidade invalida: {error}") from error

        manager = IntegrationManager(root)
        wrote_any = False
        for kind in ("agents", "skills"):
            _, plans = plan_deployments(kind, target_ids=["codex"], scope="project", identity_cfg=ident, agent_models=project_config.load(root).get("agent_models", {}))
            statuses = manager.list(plans)
            to_write = [
                plan
                for plan, status in zip(plans, statuses)
                if status["state"] == "outdated" or (install_missing and status["state"] == "not-installed")
            ]
            if to_write:
                wrote_any = True
                manager.update(to_write)
        state = STATE_UPDATED if wrote_any else STATE_SKIPPED
        return {"id": "codex-project-agents", "state": state, "path": CODEX_PROJECT_AGENTS_DISPLAY_PATH}
    except Exception as error:
        return {
            "id": "codex-project-agents",
            "state": STATE_FAILED,
            "path": CODEX_PROJECT_AGENTS_DISPLAY_PATH,
            "message": str(error),
        }


def _resolve_project_targets(raw: str | None, declared: list[str]) -> list[str]:
    if not raw:
        return list(declared)
    requested = [value.strip() for value in raw.split(",") if value.strip()]
    unknown = [value for value in requested if value not in declared]
    if unknown:
        print(f"trackfw update: unknown target id(s): {', '.join(unknown)}")
        raise SystemExit(2)
    selected = set(requested)
    return [target_id for target_id in declared if target_id in selected]


@contextlib.contextmanager
def _silence_stdout(active: bool):
    """Redirects stdout to /dev/null for the duration of the context when
    `active` is True. `generate_validate_script`, `generate_claude_commands`,
    `inject_rules_detected`, etc. print human progress lines as a side
    effect of writing; with --json, stdout must carry only the result
    document. Mirrors Go's silenceStdout (internal/commands/update.go) and
    Node's silenceConsole (npm/src/lib/update-engine.js) — added in ML-6H
    when validate-script/claude-commands were wired into this runtime's
    `update` and their print()s broke `--json` output parsing."""
    if not active:
        yield
        return
    with open(os.devnull, "w", encoding="utf-8", newline="\n") as devnull:
        with contextlib.redirect_stdout(devnull):
            yield


def _copy_path(src: str, dst: str) -> None:
    """Copies src to dst using os.lstat (symlink-aware).

    Broken symlinks are silently skipped — the destination is simply absent,
    causing _hash_rel_paths to return '' (treated as 'missing'), matching
    Node.js's fs.existsSync behaviour (existsSync follows symlinks;
    broken symlink → false → copyPath returns without copying).

    Valid symlinks are copied as regular files (the content of the symlink
    target is written to dst, not the symlink itself), matching Node.js's
    fs.copyFileSync behaviour (R6 in the declared residual).

    Directories are recursed: each entry is copied via _copy_path, preserving
    symlink semantics throughout the subtree. The directory itself is created
    with os.makedirs before the loop so an empty declared directory materialises
    as present (not absent) in the sandbox.
    """
    try:
        st = os.lstat(src)
    except FileNotFoundError:
        return  # absent or broken symlink — skip
    import stat as _stat
    if _stat.S_ISLNK(st.st_mode):
        # Follow the symlink to verify it resolves.
        try:
            os.stat(src)  # follows symlinks; raises if broken
        except OSError:
            return  # broken symlink — skip
        # Valid symlink: copy content (open(src) follows symlinks).
        parent = os.path.dirname(dst)
        if parent:
            os.makedirs(parent, exist_ok=True)
        shutil.copy2(src, dst)  # copy2 follows symlinks — safe for valid ones
        return
    if _stat.S_ISDIR(st.st_mode):
        os.makedirs(dst, exist_ok=True)
        for name in os.listdir(src):
            _copy_path(os.path.join(src, name), os.path.join(dst, name))
        return
    # Regular file.
    parent = os.path.dirname(dst)
    if parent:
        os.makedirs(parent, exist_ok=True)
    shutil.copy2(src, dst)


def _build_sandbox_inclusion(selected: list[str], hooks: str | None = None) -> list[str]:
    """Returns the union of paths to copy into the --dry-run sandbox.

    Three categories:
    1. Each selected target's declared relPaths (written outputs that
       dry-run hashes before and after apply to detect changes).
    2. trackfw.yaml — always a prerequisite: agent-rules reads it for
       agent_conventions when generating CLAUDE.md (Gap E).
    3. Detection-signal files for agent-hooks (Gap C): inject_hooks_detected
       checks these to decide which hooks to inject; without them the sandbox
       silently skips hooks the real run would write.
    """
    seen: set[str] = set()

    # Gap E prerequisite: always copy trackfw.yaml.
    seen.add("trackfw.yaml")

    agent_hooks_selected = False
    for target_id in selected:
        if target_id == "agent-rules":
            for p in AGENT_RULES_RELATIVE_PATHS:
                seen.add(p)
        elif target_id == "agent-hooks":
            agent_hooks_selected = True
            for p in AGENT_HOOKS_RELATIVE_PATHS:
                seen.add(p)
        elif target_id == "validate-script":
            seen.add(VALIDATE_SCRIPT_RELATIVE_PATH)
        elif target_id == "ci-workflow":
            for p in CI_WORKFLOW_RELATIVE_PATHS:
                seen.add(p)
        elif target_id == "claude-commands":
            seen.add(CLAUDE_COMMANDS_RELATIVE_PATH)
        # codex-project-agents has no static relPaths (Gap D residual)

    # Gap C: detection-signal files that inject_hooks_detected checks.
    if agent_hooks_selected:
        for p in [
            "CLAUDE.md",
            "AGENTS.md",
            "GEMINI.md",
            os.path.join(".github", "copilot-instructions.md"),
            ".windsurfrules",
            os.path.join(".amazonq", "developer", "guidelines.md"),
        ]:
            seen.add(p)

    return sorted(seen)


def _copy_project_tree(src: str, dst: str, paths: list[str]) -> None:
    """Copies only the declared inclusion paths from src into dst.

    Replaces the former shutil.copytree-based copy that traversed the entire
    project tree, which aborted on broken symlinks outside the declared target
    set (KG's CMDB incident: .venv/bin/python → removed python3.13). With
    inclusion-based copying, anything not in paths is irrelevant.
    """
    for rel in paths:
        _copy_path(os.path.join(src, rel), os.path.join(dst, rel))


def _run_project(args: argparse.Namespace) -> None:
    cwd = os.getcwd()
    if not os.path.exists(os.path.join(cwd, "trackfw.yaml")):
        print("Erro: trackfw.yaml não encontrado — execute trackfw init primeiro")
        raise SystemExit(1)

    # ci-workflow's declared presence depends on cfg["ci"] (trackfw.yaml),
    # read from the real cwd — same as Go's ProjectTargetIDs(loadUpdateConfig())
    # and Node's cfg read before building PROJECT_TARGET_IDS' effective set —
    # OR (AC17(c), ML-2G) on trackfw-validate.yml already existing on disk,
    # also read from the real cwd, never the --dry-run sandbox.
    update_cfg = _load_update_config(cwd)
    declared_ids = project_target_ids(update_cfg, _discover_workflow_present(cwd))
    target_ids = _resolve_project_targets(args.targets, declared_ids)
    dry_run = bool(args.dry_run)
    install_missing = bool(args.install_missing)

    apply_root = cwd
    tmp_dir = None
    if dry_run:
        tmp_dir = tempfile.mkdtemp(prefix="trackfw-update-")
        _copy_project_tree(cwd, tmp_dir, _build_sandbox_inclusion(target_ids))
        apply_root = tmp_dir

    # Writes against apply_root (the tmp sandbox during --dry-run, cwd
    # otherwise) so this never mutates the real trackfw.yaml when dry_run is
    # set — same sandboxing contract as every other target below.
    _ensure_global_adr_dir_registered(apply_root)

    try:
        from trackfw.generators.init_gen import inject_rules_detected
        from trackfw.generators.hooks import inject_hooks_detected

        targets: list[dict[str, Any]] = []
        with _silence_stdout(bool(args.json)):
            for target_id in target_ids:
                if target_id == "agent-rules":
                    targets.append(
                        _run_file_target(
                            "agent-rules",
                            ", ".join(AGENT_RULES_RELATIVE_PATHS),
                            apply_root,
                            AGENT_RULES_RELATIVE_PATHS,
                            inject_rules_detected,
                            dry_run,
                            install_missing,
                        )
                    )
                elif target_id == "agent-hooks":
                    targets.append(
                        _run_file_target(
                            "agent-hooks",
                            AGENT_HOOKS_DISPLAY_PATH,
                            apply_root,
                            AGENT_HOOKS_RELATIVE_PATHS,
                            inject_hooks_detected,
                            dry_run,
                            install_missing,
                        )
                    )
                elif target_id == "codex-project-agents":
                    targets.append(_codex_project_agents_target(apply_root, dry_run, install_missing))
                elif target_id == "validate-script":
                    from trackfw.generators.init_gen import generate_validate_script

                    targets.append(
                        _run_file_target(
                            "validate-script",
                            VALIDATE_SCRIPT_RELATIVE_PATH,
                            apply_root,
                            [VALIDATE_SCRIPT_RELATIVE_PATH],
                            generate_validate_script,
                            dry_run,
                            install_missing,
                        )
                    )
                elif target_id == "ci-workflow":
                    from trackfw.generators.init_gen import generate_ci_workflow

                    def _apply_ci_workflow(root: str, _cfg: dict = update_cfg) -> None:
                        generate_ci_workflow(root, _cfg)
                        _refresh_discover_github_actions_workflow_if_present(root)

                    targets.append(
                        _run_file_target(
                            "ci-workflow",
                            CI_WORKFLOW_DISPLAY_PATH,
                            apply_root,
                            CI_WORKFLOW_RELATIVE_PATHS,
                            _apply_ci_workflow,
                            dry_run,
                            install_missing,
                        )
                    )
                elif target_id == "claude-commands":
                    from trackfw.generators.init_gen import generate_claude_commands

                    targets.append(
                        _run_file_target(
                            "claude-commands",
                            CLAUDE_COMMANDS_RELATIVE_PATH,
                            apply_root,
                            [CLAUDE_COMMANDS_RELATIVE_PATH],
                            generate_claude_commands,
                            dry_run,
                            install_missing,
                        )
                    )
    finally:
        if tmp_dir:
            shutil.rmtree(tmp_dir, ignore_errors=True)

    summary = {
        STATE_UPDATED: sum(1 for target in targets if target["state"] == STATE_UPDATED),
        STATE_SKIPPED: sum(1 for target in targets if target["state"] == STATE_SKIPPED),
        STATE_MISSING: sum(1 for target in targets if target["state"] == STATE_MISSING),
        STATE_FAILED: sum(1 for target in targets if target["state"] == STATE_FAILED),
    }

    payload = {
        "scope": "project",
        "dry_run": dry_run,
        "targets": targets,
        "summary": summary,
    }

    if args.json:
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    else:
        print("trackfw update — scope: project" + (" (dry-run)" if dry_run else ""))
        for target in targets:
            suffix = f" — {target['message']}" if target["state"] == STATE_FAILED and "message" in target else ""
            print(f"  {target['id']:<24} {target['state']:<8} ({target['path']}){suffix}")
        print(
            f"\nupdated={summary[STATE_UPDATED]} skipped={summary[STATE_SKIPPED]} "
            f"missing={summary[STATE_MISSING]} failed={summary[STATE_FAILED]}"
        )

    if summary[STATE_FAILED] > 0:
        raise SystemExit(1)
