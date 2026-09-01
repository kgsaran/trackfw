"""
commands/update_harness.py — trackfw update harness (Python CLI).

Refreshes trackfw-managed artifacts already installed in the user's global
harness (``~/.claude``, ``~/.codex``, ``~/.gemini`` and equivalents for the
other catalog targets). Never touches the current project and never
requires ``trackfw.yaml`` or a project cwd — this command runs from
anywhere. Contract: docs/cli-parity.md, section "`trackfw update` vs
`trackfw update harness`".

Declared harness targets (fixed order — this is the order `targets` in the
JSON document follows, not filesystem order):

  1. ``claude-skill``            — legacy global compatibility skill,
                                    ``~/.claude/skills/trackfw/SKILL.md``.
     For every catalog target ``<tool>`` (in catalog.json declaration
     order: claude, codex, gemini, antigravity, cursor, copilot, windsurf,
     amazonq, opencode, kiro), two targets follow:
  N. ``<tool>-agents``           — every catalog *agent* item already
                                    deployed for ``<tool>`` at global scope.
  N. ``<tool>-skills``           — every catalog *skill* item already
                                    deployed for ``<tool>`` at global scope.

``<tool>-agents``/``<tool>-skills`` are a roll-up over every catalog item
for that (tool, kind) pair — mirroring the one directory-level example row
in the contract (``codex-agents`` / ``~/.codex/agents``), rather than one
row per catalog item (which `trackfw agents update --targets <tool>`
already reports at that granularity). Roll-up precedence when items
disagree: any item ``failed`` wins over ``updated``, which wins over
``skipped``. A group where every catalog item is not-installed reports
``missing``; a group with at least one installed item never reports
``missing`` for the itself (see the ambiguity note in the ML-6D report —
this precedence is a documented assumption, not part of the pinned
contract).
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any

from trackfw.identity import IdentityError, load as load_identity
from trackfw import config as trackfw_config
from trackfw.integrations.catalog import global_group_path, load_catalog, plan_deployments
from trackfw.integrations.manager import IntegrationError, IntegrationManager
from trackfw.generators.hooks import _merge_claude_hook_array, _merge_simple_command_array, _merge_copilot_hook_array
from trackfw.generators.init_gen import (
    generate_global_credential_guard_script,
    generate_global_git_branch_guard_script,
)
from trackfw.homedir import home_dir

STATE_UPDATED = "updated"
STATE_SKIPPED = "skipped"
STATE_MISSING = "missing"
STATE_FAILED = "failed"

_CATALOG_TARGET_ORDER = [
    "claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "opencode", "kiro",
]
_CATALOG_KIND_ORDER = ["agents", "skills"]

LEGACY_SKILL_RELATIVE = os.path.join(".claude", "skills", "trackfw", "SKILL.md")

LEGACY_SKILL_CONTENT = (
    "---\n"
    "name: trackfw\n"
    'description: "trackfw — Governed Software Delivery: ADR → REQ → ROADMAP → kanban"\n'
    'signature: "\U0001F4E6 trackfw - Governed Delivery"\n'
    "---\n\n"
    "# trackfw — Modo de Operação\n\n"
    "Você está operando com o **trackfw**, um framework de governança de entrega de software.\n"
    "Cadeia: **ADR → REQ → ROADMAP** · Estados: `backlog / wip / blocked / done / abandoned`\n\n"
    "## Comandos principais\n\n"
    "- `trackfw context` — contexto de trabalho atual (sempre execute primeiro)\n"
    "- `trackfw status` — todos os artefatos e estados\n"
    "- `trackfw validate` — valida consistência de governança\n"
    "- `trackfw roadmap move <nome> <estado>` — transição de estado\n"
    "- `trackfw serve` — board Kanban em http://localhost:4080\n\n"
    "## Protocolo de agente\n\n"
    "1. Antes de iniciar: `trackfw context` + ler `docs/agents-working-context.md`\n"
    "2. Após concluir: atualizar `docs/agents-working-context.md`\n"
    "3. Antes de PR: `trackfw validate` deve passar com zero violations\n"
)


def _tildeify(home: str, absolute: str) -> str:
    """Renders an absolute path rooted at `home` back into its `~/`-relative
    form for display — the contract's JSON example shows global paths
    abbreviated as `~/...` (docs/cli-parity.md, "Declared harness targets —
    pinned list": "path is rendered tilde-abbreviated ... never as an
    absolute path"). Mirrors npm/src/lib/update-engine.js:tildeify and
    internal/generators/update.go's harness display paths."""
    normalized_home = os.path.normpath(home)
    normalized = os.path.normpath(absolute)
    if normalized == normalized_home:
        return "~"
    prefix = normalized_home + os.sep
    if normalized.startswith(prefix):
        return "~/" + normalized[len(prefix):]
    return normalized


def declared_target_ids() -> list[str]:
    # "codex-credential-guard"/"codex-git-branch-guard",
    # "gemini-credential-guard"/"gemini-git-branch-guard",
    # "cursor-credential-guard"/"cursor-git-branch-guard",
    # "copilot-credential-guard"/"copilot-git-branch-guard",
    # "kiro-credential-guard"/"kiro-git-branch-guard" are each inserted
    # immediately BEFORE their tool's "-agents"/"-skills" pair — same
    # relative position as claude-credential-guard/claude-git-branch-guard,
    # which precede claude-agents/claude-skills, with credential-guard
    # always preceding git-branch-guard within a tool. See
    # internal/generators/update.go:buildHarnessTargetIDs for the full
    # rationale (ROADMAP-2026-08-06 Wave 2/ML-2B, ML-2C, ML-2D, ML-2E,
    # ML-2F; ROADMAP-2026-08-17 Wave 2/ML-2A for the git-branch-guard ids).
    ids = ["claude-skill", "claude-credential-guard", "claude-git-branch-guard"]
    for tool in _CATALOG_TARGET_ORDER:
        if tool == "codex":
            ids.append("codex-credential-guard")
            ids.append("codex-git-branch-guard")
        if tool == "gemini":
            ids.append("gemini-credential-guard")
            ids.append("gemini-git-branch-guard")
        if tool == "cursor":
            ids.append("cursor-credential-guard")
            ids.append("cursor-git-branch-guard")
        if tool == "copilot":
            ids.append("copilot-credential-guard")
            ids.append("copilot-git-branch-guard")
        if tool == "kiro":
            ids.append("kiro-credential-guard")
            ids.append("kiro-git-branch-guard")
        for kind in _CATALOG_KIND_ORDER:
            ids.append(f"{tool}-{kind}")
    return ids


def register(update_actions) -> None:
    parser = update_actions.add_parser(
        "harness",
        help="Update trackfw rules, agents and skills already installed in the user's global harness",
    )
    parser.add_argument("--dry-run", action="store_true", help="Compute and report states without writing anything")
    parser.add_argument("--json", action="store_true", help="Emit the result document instead of the text report")
    parser.add_argument("--targets", help="Comma-separated subset of harness target ids")
    parser.add_argument(
        "--install-missing",
        action="store_true",
        help="Allow missing targets to be installed instead of merely reported",
    )
    parser.set_defaults(func=_run)


def _resolve_targets(raw: str | None) -> list[str]:
    declared = declared_target_ids()
    if not raw:
        return declared
    requested = [value.strip() for value in raw.split(",") if value.strip()]
    unknown = [value for value in requested if value not in declared]
    if unknown:
        print(
            f"trackfw update harness: unknown target id(s): {', '.join(unknown)}",
            )
        raise SystemExit(2)
    # Preserve declared order, not the order the user typed --targets in —
    # consistent with "targets follows the declared target order" (contract).
    selected = set(requested)
    return [target_id for target_id in declared if target_id in selected]


def _legacy_skill_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    path = os.path.join(home, LEGACY_SKILL_RELATIVE)
    display_path = _tildeify(home, path)
    desired = LEGACY_SKILL_CONTENT.encode("utf-8")
    try:
        existing = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": "claude-skill", "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": "claude-skill", "state": STATE_UPDATED, "path": display_path}
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_bytes(desired)
        except OSError as error:
            return {"id": "claude-skill", "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": "claude-skill", "state": STATE_UPDATED, "path": display_path}
    if existing == desired:
        return {"id": "claude-skill", "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": "claude-skill", "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_bytes(desired)
    except OSError as error:
        return {"id": "claude-skill", "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": "claude-skill", "state": STATE_UPDATED, "path": display_path}


def _credential_guard_claude_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    credential-guard hook wiring for Claude Code: PreToolUse[matcher:"Bash"]/
    PostToolUse[matcher:"Bash"] entries in ~/.claude/settings.json pointing
    at the ABSOLUTE path of ~/.trackfw/scripts/trackfw-credential-guard.sh
    (a global hook can fire from any project's cwd). Mirrors
    internal/generators/update.go:harnessCredentialGuardTargetClaude —
    including deliberately NOT generating the script file itself here (out
    of this target's scope, same as the Go implementation). Reuses
    `_merge_claude_hook_array` (generators/hooks.py) so any pre-existing
    content in ~/.claude/settings.json (other hooks, user config) is
    preserved — only the credential-guard entry is added/merged. Does not
    reuse `_read_json`/`_write_json`: those swallow parse errors as `{}`,
    which would silently clobber unreadable/corrupt user config instead of
    reporting `failed`.
    """
    target_id = "claude-credential-guard"
    path = os.path.join(home, ".claude", "settings.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
        _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
    _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _credential_guard_codex_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    credential-guard hook wiring for Codex CLI: PreToolUse[matcher:"Bash"]/
    PostToolUse[matcher:"Bash"] entries in ~/.codex/hooks.json pointing at
    the ABSOLUTE path of ~/.trackfw/scripts/trackfw-credential-guard.sh (a
    global hook can fire from any project's cwd). Mirrors
    `_credential_guard_claude_result` exactly (same 4-state contract, same
    idempotent merge via `_merge_claude_hook_array`) and
    internal/generators/update.go:harnessCredentialGuardTargetCodex.

    Investigation (ROADMAP-2026-08-06 Wave 2/ML-2B, confirmed 2026-08-06
    against https://developers.openai.com/codex/hooks): "Hooks are enabled
    by default. To turn them off in config.toml, set: [features] hooks =
    false. Use hooks as the canonical feature key. codex_hooks still works
    as a deprecated alias." No `[features] codex_hooks = true` opt-in is
    required — the flag exists only to turn hooks OFF and is a deprecated
    alias for the canonical `hooks` key. https://developers.openai.com/codex
    /config-advanced (also fetched 2026-08-06) has no conflicting
    requirement.
    """
    target_id = "codex-credential-guard"
    path = os.path.join(home, ".codex", "hooks.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
        _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
    _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _credential_guard_gemini_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    credential-guard hook wiring for Gemini CLI:
    BeforeTool[matcher:"run_shell_command"]/AfterTool[matcher:"run_shell_command"]
    entries in ~/.gemini/settings.json pointing at the ABSOLUTE path of
    ~/.trackfw/scripts/trackfw-credential-guard.sh (a global hook can fire
    from any project's cwd). Mirrors `_credential_guard_claude_result`/
    `_credential_guard_codex_result` exactly (same 4-state contract, same
    idempotent merge via `_merge_claude_hook_array`) and
    internal/generators/update.go:harnessCredentialGuardTargetGemini — only
    the event key names differ (BeforeTool/AfterTool instead of
    PreToolUse/PostToolUse) and the matcher is "run_shell_command" instead of
    "Bash", since Gemini CLI uses a different hook vocabulary than
    Claude/Codex (confirmed against
    https://geminicli.com/docs/hooks/reference).
    """
    target_id = "gemini-credential-guard"
    path = os.path.join(home, ".gemini", "settings.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_claude_hook_array(hooks.setdefault("BeforeTool", []), "run_shell_command", script_path)
        _merge_claude_hook_array(hooks.setdefault("AfterTool", []), "run_shell_command", script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_claude_hook_array(hooks.setdefault("BeforeTool", []), "run_shell_command", script_path)
    _merge_claude_hook_array(hooks.setdefault("AfterTool", []), "run_shell_command", script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _credential_guard_cursor_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    credential-guard hook wiring for Cursor:
    hooks.beforeShellExecution/hooks.afterShellExecution entries in
    ~/.cursor/hooks.json pointing at the ABSOLUTE path of
    ~/.trackfw/scripts/trackfw-credential-guard.sh (a global hook can fire
    from any project's cwd). Mirrors `_credential_guard_claude_result`/
    `_credential_guard_codex_result`/`_credential_guard_gemini_result`'s
    4-state contract and internal/generators/update.go:
    harnessCredentialGuardTargetCursor, but via `_merge_simple_command_array`
    (generators/hooks.py) instead of `_merge_claude_hook_array`: Cursor's
    hooks.json schema (`{"version":1,"hooks":{"<event>":[{"command":"..."}]}}`,
    confirmed by generators/hooks.py:inject_cursor_hooks) is structurally
    different from Claude/Codex/Gemini's — each event array holds flat
    {"command":"..."} entries, no per-entry "matcher", no nested
    {"type","hooks":[...]}.
    """
    target_id = "cursor-credential-guard"
    path = os.path.join(home, ".cursor", "hooks.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        root.setdefault("version", 1)
        hooks = root.setdefault("hooks", {})
        _merge_simple_command_array(hooks.setdefault("beforeShellExecution", []), script_path)
        _merge_simple_command_array(hooks.setdefault("afterShellExecution", []), script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    root.setdefault("version", 1)
    hooks = root.setdefault("hooks", {})
    _merge_simple_command_array(hooks.setdefault("beforeShellExecution", []), script_path)
    _merge_simple_command_array(hooks.setdefault("afterShellExecution", []), script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _credential_guard_copilot_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    credential-guard hook wiring for GitHub Copilot:
    hooks.preToolUse/hooks.postToolUse[matcher:"bash"] entries in
    ~/.copilot/settings.json pointing at the ABSOLUTE path of
    ~/.trackfw/scripts/trackfw-credential-guard.sh (a global hook can fire
    from any project's cwd). Mirrors `_credential_guard_claude_result`/
    `_credential_guard_codex_result`/`_credential_guard_gemini_result`/
    `_credential_guard_cursor_result`'s 4-state contract and
    internal/generators/update.go:harnessCredentialGuardTargetCopilot, but
    via `_merge_copilot_hook_array` (generators/hooks.py) since Copilot's
    command-hook entry shape is its own.

    Investigation (ROADMAP-2026-08-06 Wave 2/ML-2E, confirmed 2026-08-06
    against https://docs.github.com/en/copilot/reference/hooks-reference,
    section "Hooks locations"): the user/global scope offers two mechanisms —
    a dedicated directory of standalone hook files (`~/.copilot/hooks/`, by
    default) and an "Inline hooks block in user-level config — the hooks
    field at the top level of ~/.copilot/settings.json". This follows the
    roadmap's explicit instruction and targets the latter. The doc confirms
    settings.json is NOT a dedicated hooks file (it is Copilot CLI's general
    user config file), so this merges into root["hooks"] only, preserving
    every other top-level key — same discipline as the Claude/Codex/Gemini
    targets' own general settings files. Entry shape ("Hook configuration
    files use JSON format with version 1", no exception carved out for the
    inline hooks field) is identical to `inject_copilot_hooks`'s project-scope
    entries; no top-level "version" key is added here since no example in the
    doc shows one on settings.json itself (only on dedicated hook files).
    """
    target_id = "copilot-credential-guard"
    path = os.path.join(home, ".copilot", "settings.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_copilot_hook_array(hooks.setdefault("preToolUse", []), script_path)
        _merge_copilot_hook_array(hooks.setdefault("postToolUse", []), script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_copilot_hook_array(hooks.setdefault("preToolUse", []), script_path)
    _merge_copilot_hook_array(hooks.setdefault("postToolUse", []), script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _credential_guard_kiro_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    credential-guard hook wiring for Kiro: a DEDICATED file at
    ~/.kiro/hooks/trackfw-credential-guard.json — unlike
    `_credential_guard_claude_result`/`..._codex_result`/`..._gemini_result`/
    `..._cursor_result`/`..._copilot_result` above (which merge into a
    shared, general settings file), ~/.kiro/hooks/ is a directory of
    one-file-per-hook, confirmed by
    generators/hooks.py:inject_kiro_hooks's own investigation and by
    kiro.dev/changelog/cli/2-13/: "Hooks placed in ~/.kiro/hooks/ now fire
    in every workspace automatically ... Workspace-level hooks continue to
    work alongside global ones". Same schema as inject_kiro_hooks (project
    scope): top-level {"version":"v1","hooks":[...]}, each entry
    {"name","description","trigger","matcher","action":{"type":"command",
    "command":<absolute path>}} — but the command here is the ABSOLUTE path
    of ~/.trackfw/scripts/trackfw-credential-guard.sh (a global hook can
    fire from any project's cwd), and the two hook names are
    "trackfw-credential-guard-global-pre"/"-global-post" — deliberately
    DISTINCT from the project-scope names ("trackfw-credential-guard-pre"/
    "-post") since this writes an entirely different file and nothing
    documents whether Kiro deduplicates same-named hooks across
    scopes/files; ML-3A's future project-scope dedup will match on the
    script path, not the hook name. Mirrors
    internal/generators/update.go:harnessCredentialGuardTargetKiro.

    Kiro v3 caveat (ROADMAP-2026-08-06 Wave 2/ML-2F, confirmed 2026-08-06
    against kiro.dev/changelog/cli/2-13/): global hooks are "Available in
    V3 (`kiro-cli --v3`)". `--v3` is a LAUNCH-MODE flag on the same
    installed binary, not a value any `--version`-style command reports —
    there is no documented `kiro`/`kiro-cli --version` output format
    anywhere in the fetched sources, and no persistent installed-version
    fact to probe from a separate process (trackfw never invokes Kiro
    itself). This target does NOT attempt a subprocess version probe, and
    does NOT put the caveat in the JSON "message" field either (pinned
    contract: "message" is failure-only — see docs/cli-parity.md). The v3
    prerequisite is documented here and in docs/cli-parity.md's own "Kiro
    global-scope wiring (ML-2F)" section instead.
    """
    target_id = "kiro-credential-guard"
    path = os.path.join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

    desired_doc = {
        "version": "v1",
        "hooks": [
            {
                "name": "trackfw-credential-guard-global-pre",
                "description": "Blocks/warns on possible plaintext credential materialization before a shell command executes (global, all projects)",
                "trigger": "PreToolUse",
                "matcher": "shell",
                "action": {"type": "command", "command": script_path},
            },
            {
                "name": "trackfw-credential-guard-global-post",
                "description": "Warns on possible plaintext credential materialization after a shell command executes (global, all projects)",
                "trigger": "PostToolUse",
                "matcher": "shell",
                "action": {"type": "command", "command": script_path},
            },
        ],
    }
    desired = (json.dumps(desired_doc, indent=2) + "\n").encode("utf-8")

    try:
        existing = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_bytes(desired)
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if existing == desired:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_bytes(desired)
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _git_branch_guard_claude_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    git-branch-guard hook wiring for Claude Code: mirrors
    `_credential_guard_claude_result` exactly (same file,
    ~/.claude/settings.json, same hooks.PreToolUse/PostToolUse[matcher:"Bash"]
    arrays, same `_merge_claude_hook_array` reuse), only the referenced
    script differs (trackfw-git-branch-guard.sh instead of
    trackfw-credential-guard.sh) — `_merge_claude_hook_array` appends a
    second, distinct command entry into the SAME matcher's inner array
    rather than overwriting the first, so both guards coexist. Mirrors
    internal/generators/update.go:harnessGitBranchGuardTargetClaude
    (ROADMAP-2026-08-17 Wave 2/ML-2A).
    """
    target_id = "claude-git-branch-guard"
    path = os.path.join(home, ".claude", "settings.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
        _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
    _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _git_branch_guard_codex_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Mirrors `_git_branch_guard_claude_result` for Codex CLI
    (~/.codex/hooks.json), same as `_credential_guard_codex_result`'s own
    relationship to `_credential_guard_claude_result`.
    """
    target_id = "codex-git-branch-guard"
    path = os.path.join(home, ".codex", "hooks.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
        _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_claude_hook_array(hooks.setdefault("PreToolUse", []), "Bash", script_path)
    _merge_claude_hook_array(hooks.setdefault("PostToolUse", []), "Bash", script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _git_branch_guard_gemini_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Mirrors `_git_branch_guard_claude_result` for Gemini CLI
    (~/.gemini/settings.json, BeforeTool/AfterTool[matcher:"run_shell_command"]),
    same as `_credential_guard_gemini_result`'s own relationship to
    `_credential_guard_claude_result`.
    """
    target_id = "gemini-git-branch-guard"
    path = os.path.join(home, ".gemini", "settings.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_claude_hook_array(hooks.setdefault("BeforeTool", []), "run_shell_command", script_path)
        _merge_claude_hook_array(hooks.setdefault("AfterTool", []), "run_shell_command", script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_claude_hook_array(hooks.setdefault("BeforeTool", []), "run_shell_command", script_path)
    _merge_claude_hook_array(hooks.setdefault("AfterTool", []), "run_shell_command", script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _git_branch_guard_cursor_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Mirrors `_git_branch_guard_claude_result` for Cursor
    (~/.cursor/hooks.json, hooks.beforeShellExecution/afterShellExecution via
    `_merge_simple_command_array`), same as `_credential_guard_cursor_result`'s
    own relationship to `_credential_guard_claude_result`.
    """
    target_id = "cursor-git-branch-guard"
    path = os.path.join(home, ".cursor", "hooks.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        root.setdefault("version", 1)
        hooks = root.setdefault("hooks", {})
        _merge_simple_command_array(hooks.setdefault("beforeShellExecution", []), script_path)
        _merge_simple_command_array(hooks.setdefault("afterShellExecution", []), script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    root.setdefault("version", 1)
    hooks = root.setdefault("hooks", {})
    _merge_simple_command_array(hooks.setdefault("beforeShellExecution", []), script_path)
    _merge_simple_command_array(hooks.setdefault("afterShellExecution", []), script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _git_branch_guard_copilot_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Mirrors `_git_branch_guard_claude_result` for GitHub Copilot
    (~/.copilot/settings.json, hooks.preToolUse/postToolUse[matcher:"bash"]
    via `_merge_copilot_hook_array`), same as
    `_credential_guard_copilot_result`'s own relationship to
    `_credential_guard_claude_result`.
    """
    target_id = "copilot-git-branch-guard"
    path = os.path.join(home, ".copilot", "settings.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

    try:
        raw = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        root: dict[str, Any] = {}
        hooks = root.setdefault("hooks", {})
        _merge_copilot_hook_array(hooks.setdefault("preToolUse", []), script_path)
        _merge_copilot_hook_array(hooks.setdefault("postToolUse", []), script_path)
        desired = json.dumps(root, indent=2) + "\n"
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(desired, encoding="utf-8", newline="\n")
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    try:
        root = json.loads(raw) if raw else {}
    except json.JSONDecodeError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if not isinstance(root, dict):
        root = {}
    hooks = root.setdefault("hooks", {})
    _merge_copilot_hook_array(hooks.setdefault("preToolUse", []), script_path)
    _merge_copilot_hook_array(hooks.setdefault("postToolUse", []), script_path)
    desired = json.dumps(root, indent=2) + "\n"
    if desired.encode("utf-8") == raw:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_text(desired, encoding="utf-8", newline="\n")
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _git_branch_guard_kiro_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    """Evaluates (and, unless dry_run, applies) the global-scope
    git-branch-guard hook wiring for Kiro: a DEDICATED file at
    ~/.kiro/hooks/trackfw-git-branch-guard.json — deliberately NOT the same
    file `_credential_guard_kiro_result` writes
    (~/.kiro/hooks/trackfw-credential-guard.json). That writer rewrites its
    document WHOLESALE every run (never merges), so two wholesale writers
    sharing one file would each overwrite the other's entries every run —
    both targets would report "updated" forever, a hard idempotency
    failure. Same schema as `_credential_guard_kiro_result`:
    {"version":"v1","hooks":[...]}, but hook names
    "trackfw-git-branch-guard-global-pre"/"-global-post". Mirrors
    internal/generators/update.go:harnessGitBranchGuardTargetKiro
    (ROADMAP-2026-08-17 Wave 2/ML-2A).
    """
    target_id = "kiro-git-branch-guard"
    path = os.path.join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json")
    display_path = _tildeify(home, path)
    script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

    desired_doc = {
        "version": "v1",
        "hooks": [
            {
                "name": "trackfw-git-branch-guard-global-pre",
                "description": "Blocks branch-creation git subcommands issued outside trackfw branch new (global, all trackfw projects)",
                "trigger": "PreToolUse",
                "matcher": "shell",
                "action": {"type": "command", "command": script_path},
            },
            {
                "name": "trackfw-git-branch-guard-global-post",
                "description": "Warns on branch-creation git subcommands issued outside trackfw branch new (global, all trackfw projects)",
                "trigger": "PostToolUse",
                "matcher": "shell",
                "action": {"type": "command", "command": script_path},
            },
        ],
    }
    desired = (json.dumps(desired_doc, indent=2) + "\n").encode("utf-8")

    try:
        existing = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": target_id, "state": STATE_MISSING, "path": display_path}
        if dry_run:
            return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_bytes(desired)
        except OSError as error:
            return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}

    if existing == desired:
        return {"id": target_id, "state": STATE_SKIPPED, "path": display_path}
    if dry_run:
        return {"id": target_id, "state": STATE_UPDATED, "path": display_path}
    try:
        Path(path).write_bytes(desired)
    except OSError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": display_path, "message": str(error)}
    return {"id": target_id, "state": STATE_UPDATED, "path": display_path}


def _catalog_group_result(
    tool: str,
    kind: str,
    home: str,
    manager: IntegrationManager,
    identity_cfg,
    dry_run: bool,
    install_missing: bool,
) -> dict[str, Any]:
    target_id = f"{tool}-{kind}"
    try:
        directory = global_group_path(load_catalog(), tool, kind)
    except ValueError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": "", "message": str(error)}

    try:
        harness_agent_models, harness_warn_msg = trackfw_config.resolve_agent_models("global", home, os.getcwd())
        if harness_warn_msg:
            print(harness_warn_msg, file=sys.stderr)
        _, plans = plan_deployments(kind, target_ids=[tool], scope="global", identity_cfg=identity_cfg, agent_models=harness_agent_models)
    except ValueError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": directory, "message": str(error)}

    if not plans:
        return {"id": target_id, "state": STATE_MISSING, "path": directory}

    inspections = manager.list(plans)

    installed = [(plan, inspection) for plan, inspection in zip(plans, inspections) if inspection["state"] != "not-installed"]
    not_installed = [plan for plan, inspection in zip(plans, inspections) if inspection["state"] == "not-installed"]

    results: list[tuple[str, str | None]] = []

    for plan, inspection in installed:
        pre_state = inspection["state"]
        if pre_state == "current":
            results.append((STATE_SKIPPED, None))
            continue
        if dry_run:
            if pre_state == "modified":
                results.append((STATE_FAILED, f"artifact {plan['destination']} is modified; use force"))
            else:
                results.append((STATE_UPDATED, None))
            continue
        try:
            manager.update([plan])
            results.append((STATE_UPDATED, None))
        except IntegrationError as error:
            results.append((STATE_FAILED, str(error)))

    if install_missing and not_installed:
        for plan in not_installed:
            if dry_run:
                results.append((STATE_UPDATED, None))
                continue
            try:
                manager.install([plan])
                results.append((STATE_UPDATED, None))
            except IntegrationError as error:
                results.append((STATE_FAILED, str(error)))

    if not results:
        # Nothing installed for this (tool, kind) at all, and --install-missing
        # was not requested — "missing never installs" (contract).
        return {"id": target_id, "state": STATE_MISSING, "path": directory}

    states = [state for state, _ in results]
    if STATE_FAILED in states:
        message = next(message for state, message in results if state == STATE_FAILED)
        return {"id": target_id, "state": STATE_FAILED, "path": directory, "message": message}
    if STATE_UPDATED in states:
        return {"id": target_id, "state": STATE_UPDATED, "path": directory}
    return {"id": target_id, "state": STATE_SKIPPED, "path": directory}


def _run(args: argparse.Namespace) -> None:
    home = home_dir()

    try:
        identity_cfg = load_identity(home)
    except IdentityError as error:
        print(f"update harness: identidade invalida: {error}")
        raise SystemExit(2) from error

    # The per-CLI *-credential-guard targets below only wire hook entries
    # that point at ~/.trackfw/scripts/trackfw-credential-guard.sh — none of
    # them write the script itself (ADR-2026-08-06, decision #2/#3). Without
    # this call the wiring is installed but every hook invocation fails with
    # "No such file or directory" because the script never exists.
    if not args.dry_run:
        generate_global_credential_guard_script(home)
        # ML-3C (ROADMAP-2026-08-14): mirrors Go's UpdateHarness
        # (internal/generators/update.go, GenerateGlobalGitBranchGuardScript call next to
        # GenerateGlobalCredentialGuardScript). Only writes the script itself -- no
        # per-CLI *-git-branch-guard target/hook wiring exists yet in any of the 3 stacks
        # (tracked separately; project-scope wiring is done in generators/hooks.py via
        # inject_hooks_detected).
        generate_global_git_branch_guard_script(home)

    target_ids = _resolve_targets(args.targets)
    manager = IntegrationManager(project_root=os.getcwd(), home_dir=home)

    targets: list[dict[str, Any]] = []
    for target_id in target_ids:
        if target_id == "claude-skill":
            targets.append(_legacy_skill_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "claude-credential-guard":
            targets.append(_credential_guard_claude_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "claude-git-branch-guard":
            targets.append(_git_branch_guard_claude_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "codex-credential-guard":
            targets.append(_credential_guard_codex_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "codex-git-branch-guard":
            targets.append(_git_branch_guard_codex_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "gemini-credential-guard":
            targets.append(_credential_guard_gemini_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "gemini-git-branch-guard":
            targets.append(_git_branch_guard_gemini_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "cursor-credential-guard":
            targets.append(_credential_guard_cursor_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "cursor-git-branch-guard":
            targets.append(_git_branch_guard_cursor_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "copilot-credential-guard":
            targets.append(_credential_guard_copilot_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "copilot-git-branch-guard":
            targets.append(_git_branch_guard_copilot_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "kiro-credential-guard":
            targets.append(_credential_guard_kiro_result(home, args.dry_run, args.install_missing))
            continue
        if target_id == "kiro-git-branch-guard":
            targets.append(_git_branch_guard_kiro_result(home, args.dry_run, args.install_missing))
            continue
        tool, kind = target_id.rsplit("-", 1)
        targets.append(_catalog_group_result(tool, kind, home, manager, identity_cfg, args.dry_run, args.install_missing))

    summary = {
        STATE_UPDATED: sum(1 for target in targets if target["state"] == STATE_UPDATED),
        STATE_SKIPPED: sum(1 for target in targets if target["state"] == STATE_SKIPPED),
        STATE_MISSING: sum(1 for target in targets if target["state"] == STATE_MISSING),
        STATE_FAILED: sum(1 for target in targets if target["state"] == STATE_FAILED),
    }

    payload = {
        "scope": "harness",
        "dry_run": bool(args.dry_run),
        "targets": targets,
        "summary": summary,
    }

    if args.json:
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    else:
        print("trackfw update harness — atualizando harness global...\n")
        for target in targets:
            suffix = f" — {target['message']}" if target["state"] == STATE_FAILED and "message" in target else ""
            print(f"  {target['id']:<16} {target['state']:<8} ({target['path']}){suffix}")
        print(
            f"\nupdated={summary[STATE_UPDATED]} skipped={summary[STATE_SKIPPED]} "
            f"missing={summary[STATE_MISSING]} failed={summary[STATE_FAILED]}"
        )

    if summary[STATE_FAILED] > 0:
        raise SystemExit(1)
