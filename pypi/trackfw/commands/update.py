"""
commands/update.py — trackfw update (Python CLI).

Scope: the current repository only (docs/cli-parity.md, "`trackfw update`
vs `trackfw update harness`"). This command never mutates global state —
every write below is rooted at `cwd`, and the Codex integration block below
plans/applies with `scope="project"` explicitly. `trackfw update harness`
(trackfw/commands/update_harness.py) is the counterpart that refreshes the
user's global harness (`~/.claude`, `~/.codex`, etc.) and runs from
anywhere, without a `trackfw.yaml`.

Escopo reduzido: atualiza somente as regras de agente (blocos marker-delimited),
os agent hooks e a integração Codex de projeto já instalada. Gates (hooks/CI)
e Claude commands requerem o CLI Go ou Node.js — ver docs/cli-parity.md.

--dry-run, --json, --targets and --install-missing (added for
REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador, ML-6F) report
the same four-state model (updated/skipped/missing/failed) as `trackfw update
harness`, over a "scope": "project" JSON document — see docs/cli-parity.md.
The declared PROJECT_TARGET_IDS below is this runtime's reduced surface; it is
NOT required to match Go/Node.js's project target list byte-for-byte — only
the states, flags and JSON document shape are shared (see the docstring on
"Escopo reduzido" above, which the Go/Node.js CLIs do not implement).
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import tempfile
from pathlib import Path
from typing import Any

from trackfw.commands import update_harness
from trackfw.commands.update_harness import (
    STATE_FAILED,
    STATE_MISSING,
    STATE_SKIPPED,
    STATE_UPDATED,
)

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
]

CODEX_PROJECT_AGENTS_DISPLAY_PATH = ".codex/agents, .agents/skills"

# PROJECT_TARGET_IDS — this runtime's declared, fixed-order project-scope
# target list (see module docstring: reduced scope, documented exception).
PROJECT_TARGET_IDS = ["agent-rules", "agent-hooks", "codex-project-agents"]


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

    print("trackfw update — atualizando regras de agente...\n")

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

    if os.path.exists(os.path.join(cwd, "AGENTS.md")) or os.path.isdir(os.path.join(cwd, ".codex")):
        from trackfw import identity
        from trackfw.identity import IdentityError

        # Identity errors must abort the command — never fall back silently
        # to the neutral default, which would revert the user's identity.
        try:
            ident = identity.load(os.path.expanduser("~"))
        except IdentityError as e:
            print(f"update: identidade invalida: {e}")
            raise SystemExit(2) from e

        try:
            from trackfw.integrations.catalog import plan_deployments
            from trackfw.integrations.manager import IntegrationManager
            manager = IntegrationManager(cwd)
            _, plans = plan_deployments("agents", target_ids=["codex"], scope="project", identity_cfg=ident)
            plans = [plan for plan, status in zip(plans, manager.list(plans)) if status["state"] != "not-installed"]
            manager.update(plans)
            _, plans = plan_deployments("skills", target_ids=["codex"], scope="project", identity_cfg=ident)
            plans = [plan for plan, status in zip(plans, manager.list(plans)) if status["state"] != "not-installed"]
            manager.update(plans)
        except Exception as e:
            print(f"  ⚠ Codex integration: {e}")

    print()
    print("  Nota: este CLI Python atualiza apenas as regras de agente.")
    print("  Para atualizar gates (hooks/CI) e Claude commands, use:")
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
            ident = identity.load(os.path.expanduser("~"))
        except IdentityError as error:
            raise RuntimeError(f"identidade invalida: {error}") from error

        manager = IntegrationManager(root)
        wrote_any = False
        for kind in ("agents", "skills"):
            _, plans = plan_deployments(kind, target_ids=["codex"], scope="project", identity_cfg=ident)
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


def _resolve_project_targets(raw: str | None) -> list[str]:
    if not raw:
        return list(PROJECT_TARGET_IDS)
    requested = [value.strip() for value in raw.split(",") if value.strip()]
    unknown = [value for value in requested if value not in PROJECT_TARGET_IDS]
    if unknown:
        print(f"trackfw update: unknown target id(s): {', '.join(unknown)}")
        raise SystemExit(2)
    selected = set(requested)
    return [target_id for target_id in PROJECT_TARGET_IDS if target_id in selected]


def _copy_project_tree(src: str, dst: str) -> None:
    """Copies src into dst, skipping .git and node_modules, for use as a
    --dry-run sandbox that the real project tree is never written through."""
    def _ignore(directory: str, names: list[str]) -> set[str]:
        return {name for name in names if name in (".git", "node_modules")}

    for name in os.listdir(src):
        source_path = os.path.join(src, name)
        dest_path = os.path.join(dst, name)
        if name in (".git", "node_modules"):
            continue
        if os.path.isdir(source_path):
            shutil.copytree(source_path, dest_path, ignore=_ignore)
        else:
            shutil.copy2(source_path, dest_path)


def _run_project(args: argparse.Namespace) -> None:
    cwd = os.getcwd()
    if not os.path.exists(os.path.join(cwd, "trackfw.yaml")):
        print("Erro: trackfw.yaml não encontrado — execute trackfw init primeiro")
        raise SystemExit(1)

    target_ids = _resolve_project_targets(args.targets)
    dry_run = bool(args.dry_run)
    install_missing = bool(args.install_missing)

    apply_root = cwd
    tmp_dir = None
    if dry_run:
        tmp_dir = tempfile.mkdtemp(prefix="trackfw-update-")
        _copy_project_tree(cwd, tmp_dir)
        apply_root = tmp_dir

    try:
        from trackfw.generators.init_gen import inject_rules_detected
        from trackfw.generators.hooks import inject_hooks_detected

        targets: list[dict[str, Any]] = []
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
                        ", ".join(AGENT_HOOKS_RELATIVE_PATHS),
                        apply_root,
                        AGENT_HOOKS_RELATIVE_PATHS,
                        inject_hooks_detected,
                        dry_run,
                        install_missing,
                    )
                )
            elif target_id == "codex-project-agents":
                targets.append(_codex_project_agents_target(apply_root, dry_run, install_missing))
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
