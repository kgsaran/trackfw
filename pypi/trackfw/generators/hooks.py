"""
generators/hooks.py — Injeção de attention hooks para CLIs de IA.

Detecta CLIs presentes no projeto e configura hooks PreToolUse/PostToolUse
para sinalizar o board do `trackfw serve` automaticamente.
"""

import json
import os
from pathlib import Path


# ---------------------------------------------------------------------------
# Helpers de I/O
# ---------------------------------------------------------------------------

def _read_json(file_path: str) -> dict:
    """Lê JSON de arquivo; retorna {} se não existir ou inválido."""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def _write_json(file_path: str, data: dict) -> None:
    """Escreve JSON com indent 2."""
    os.makedirs(os.path.dirname(os.path.abspath(file_path)), exist_ok=True)
    with open(file_path, 'w', encoding='utf-8') as f:
        json.dump(data, f, indent=2)
        f.write('\n')


def _has_entry(lst: list, field: str, value: str) -> bool:
    """Verifica se lista tem dict com field==value."""
    return any(isinstance(e, dict) and e.get(field) == value for e in (lst or []))


def _merge_simple_command_array(hook_list: list, command: str) -> None:
    """Garante (idempotente) que hook_list tenha uma entrada {"command": ...}.

    Mirrors internal/generators/agentfiles.go:mergeSimpleCommandArray — para
    arrays de hooks "simples" tipo Cursor (hooks.beforeShellExecution/
    afterShellExecution): cada entry é um objeto plano {"command": "..."},
    sem matcher, sem {type, hooks:[...]} aninhado como Claude/Codex/Gemini.
    """
    if not _has_entry(hook_list, 'command', command):
        hook_list.append({'command': command})


def _merge_copilot_hook_array(hook_list: list, script_path: str) -> None:
    """Garante (idempotente) que hook_list tenha uma entrada
    {"type":"command","matcher":"bash","bash":script_path,"cwd":".","timeoutSec":10}.

    Mirrors internal/generators/update.go:mergeCredentialGuardCopilotHooks —
    for GitHub Copilot's command-hook entry shape (hooks.preToolUse/
    postToolUse), matched on the "bash" field (not "command", Cursor's flat
    shape, nor a nested {"matcher","hooks":[...]}, Claude/Codex/Gemini's
    shape). See that Go function's doc comment for the full
    ~/.copilot/settings.json format investigation (ROADMAP-2026-08-06 Wave 2
    /ML-2E).
    """
    if not _has_entry(hook_list, 'bash', script_path):
        hook_list.append({
            'type': 'command',
            'matcher': 'bash',
            'bash': script_path,
            'cwd': '.',
            'timeoutSec': 10,
        })


# ---------------------------------------------------------------------------
# Global credential-guard dedup (ROADMAP-2026-08-06 Wave 3/ML-3A)
# ---------------------------------------------------------------------------
# inject_claude_hooks/inject_codex_hooks/inject_gemini_hooks/inject_cursor_hooks/
# inject_copilot_hooks/inject_kiro_hooks each check, read-only, whether the
# user already has the global-scope credential-guard wiring installed for
# that CLI (via `trackfw update harness --targets <tool>-credential-guard`,
# pypi/trackfw/commands/update_harness.py) before adding the project-scope
# credential-guard entry. If the global entry is already present, the
# project-scope entry is skipped entirely -- attention-signal/cleanup are
# unaffected (inherently project-scoped, ADR-2026-08-06 Decision #4).
#
# Fail-open is mandatory: any failure to resolve $HOME, read the global
# file, or parse its JSON is treated as "not installed globally" -- this
# section never writes to the global file (read-only by construction).
# ---------------------------------------------------------------------------

def _global_credential_guard_script_path() -> str | None:
    """Mirrors Go's globalCredentialGuardScriptPath / Node's
    globalCredentialGuardScriptPath."""
    home = os.path.expanduser('~')
    if not home or home == '~':
        return None
    return os.path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')


def _read_global_hook_json(*rel_parts: str) -> dict | None:
    """Reads+parses JSON at $HOME/<...rel_parts>; returns None on any failure
    (fail-open)."""
    home = os.path.expanduser('~')
    if not home or home == '~':
        return None
    try:
        with open(os.path.join(home, *rel_parts), 'r', encoding='utf-8') as f:
            return json.load(f)
    except (OSError, json.JSONDecodeError):
        return None


def _hook_array_has_command(existing, matcher: str, command: str) -> bool:
    """Read-only counterpart of _merge_claude_hook_array."""
    if not isinstance(existing, list):
        return False
    for entry in existing:
        if not isinstance(entry, dict) or entry.get('matcher') != matcher:
            continue
        inner = entry.get('hooks')
        if isinstance(inner, list) and _has_entry(inner, 'command', command):
            return True
    return False


def _simple_array_has_value(existing, field: str, value: str) -> bool:
    """Read-only check for flat arrays (Cursor's {"command":...} / Copilot's
    {"bash":...} shape)."""
    if not isinstance(existing, list):
        return False
    return _has_entry(existing, field, value)


def _global_credential_guard_installed_claude() -> bool:
    script_path = _global_credential_guard_script_path()
    if not script_path:
        return False
    root = _read_global_hook_json('.claude', 'settings.json')
    if not root:
        return False
    hooks = root.get('hooks')
    if not isinstance(hooks, dict):
        return False
    return _hook_array_has_command(hooks.get('PreToolUse'), 'Bash', script_path)


def _global_credential_guard_installed_codex() -> bool:
    script_path = _global_credential_guard_script_path()
    if not script_path:
        return False
    root = _read_global_hook_json('.codex', 'hooks.json')
    if not root:
        return False
    hooks = root.get('hooks')
    if not isinstance(hooks, dict):
        return False
    return _hook_array_has_command(hooks.get('PreToolUse'), 'Bash', script_path)


def _global_credential_guard_installed_gemini() -> bool:
    script_path = _global_credential_guard_script_path()
    if not script_path:
        return False
    root = _read_global_hook_json('.gemini', 'settings.json')
    if not root:
        return False
    hooks = root.get('hooks')
    if not isinstance(hooks, dict):
        return False
    return _hook_array_has_command(hooks.get('BeforeTool'), 'run_shell_command', script_path)


def _global_credential_guard_installed_cursor() -> bool:
    script_path = _global_credential_guard_script_path()
    if not script_path:
        return False
    root = _read_global_hook_json('.cursor', 'hooks.json')
    if not root:
        return False
    hooks = root.get('hooks')
    if not isinstance(hooks, dict):
        return False
    return _simple_array_has_value(hooks.get('beforeShellExecution'), 'command', script_path)


def _global_credential_guard_installed_copilot() -> bool:
    script_path = _global_credential_guard_script_path()
    if not script_path:
        return False
    root = _read_global_hook_json('.copilot', 'settings.json')
    if not root:
        return False
    hooks = root.get('hooks')
    if not isinstance(hooks, dict):
        return False
    return _simple_array_has_value(hooks.get('preToolUse'), 'bash', script_path)


def _global_credential_guard_installed_kiro() -> bool:
    """~/.kiro/hooks/trackfw-credential-guard.json is 100% dedicated to the
    global credential-guard wiring (overwritten wholesale, never merged), so
    presence + non-empty content is sufficient -- matches the roadmap's
    explicit instruction for Kiro."""
    home = os.path.expanduser('~')
    if not home or home == '~':
        return False
    path = os.path.join(home, '.kiro', 'hooks', 'trackfw-credential-guard.json')
    try:
        return os.path.getsize(path) > 0
    except OSError:
        return False


def _merge_claude_hook_array(hook_list: list, matcher: str, command: str) -> None:
    """Garante (idempotente) que hook_list tenha uma entrada matcher→command.

    Se já existir uma entrada com o matcher dado, apenas garante que o
    command esteja presente nela (sem duplicar). Caso contrário, cria uma
    nova entrada — preservando quaisquer outras entradas já presentes
    (ex.: matcher diferente injetado por uma execução anterior).
    """
    for entry in hook_list:
        if isinstance(entry, dict) and entry.get('matcher') == matcher:
            inner = entry.setdefault('hooks', [])
            if not _has_entry(inner, 'command', command):
                inner.append({'type': 'command', 'command': command})
            return

    hook_list.append({
        'matcher': matcher,
        'hooks': [
            {'type': 'command', 'command': command}
        ],
    })


# ---------------------------------------------------------------------------
# Claude Code — .claude/settings.json
# ---------------------------------------------------------------------------

def inject_claude_hooks(cwd: str) -> None:
    """Injeta hooks PreToolUse/PostToolUse no .claude/settings.json."""
    file_path = os.path.join(cwd, '.claude', 'settings.json')
    data = _read_json(file_path)

    hooks = data.setdefault('hooks', {})

    # PreToolUse — AskUserQuestion matcher → signal; Bash matcher → credential guard
    pre_hooks = hooks.setdefault('PreToolUse', [])
    _merge_claude_hook_array(pre_hooks, 'AskUserQuestion', 'scripts/trackfw-attention-signal.sh')
    # Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A): skip project-scope
    # credential-guard when the global one is already installed.
    skip_cg = _global_credential_guard_installed_claude()
    if not skip_cg:
        _merge_claude_hook_array(pre_hooks, 'Bash', 'scripts/trackfw-credential-guard.sh')

    # PostToolUse — AskUserQuestion matcher → cleanup; Bash matcher → credential guard
    post_hooks = hooks.setdefault('PostToolUse', [])
    _merge_claude_hook_array(post_hooks, 'AskUserQuestion', 'scripts/trackfw-attention-cleanup.sh')
    if not skip_cg:
        _merge_claude_hook_array(post_hooks, 'Bash', 'scripts/trackfw-credential-guard.sh')

    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Codex — .codex/hooks.json
#
# Two independent hook events: PermissionRequest (matcher ".*") for the existing
# attention-signal -- only fires when Codex is about to prompt for approval, not
# for every command -- and PreToolUse/PostToolUse (matcher "Bash") for
# credential-guard, which fires for every Bash tool call regardless of approval.
# Confirmed against https://developers.openai.com/codex/hooks (2026-08-05): hooks
# are enabled by default (no `[features] hooks = true`/`codex_hooks` opt-in
# needed -- that flag exists only to turn hooks OFF), and PreToolUse blocking
# uses exit code 2 + stderr (matching trackfw-credential-guard.sh's "block" mode).
# ---------------------------------------------------------------------------

def _merge_codex_hook_entry(entries: list, matcher: str, command: str) -> None:
    """Garante (idempotente) que `entries` (um array PreToolUse/PostToolUse/etc.
    do formato Codex) tenha uma entrada `matcher` contendo `command`.

    Mirrors `_merge_claude_hook_array`: if an entry with the given matcher
    already exists (e.g. a third-party hook, or a previous trackfw run), the
    new command is merged into its `hooks` array instead of appending a
    duplicate `{"matcher": ...}` block.

    No `timeout`/`statusMessage` decoration: this function used to accept
    `**extra_fields` and always passed `timeout=10` (+ a per-hook
    `statusMessage`) when creating a new entry -- fields Go's
    InjectCodexHooks and Node's injectCodexHooks never wrote and that
    `docs/cli-parity.md`'s "Codex wiring (ML-2B)" section never documents as
    functional (undocumented on
    https://developers.openai.com/codex/hooks, no test in
    pypi/tests/test_generators_init.py or pypi/tests/test_codex.py depends
    on them). check-agent-hooks-parity.sh (ML-3A) caught the resulting
    Python-only .codex/hooks.json structural drift; removed here to align
    Python with Go/Node, mirroring the ML-2C precedent that dropped
    Python-only `name`/`timeout: 10000` decoration from the Gemini hooks
    entries for the same reason.
    """
    for entry in entries:
        if not isinstance(entry, dict) or entry.get('matcher') != matcher:
            continue
        inner = entry.setdefault('hooks', [])
        if not _has_entry(inner, 'command', command):
            inner.append({'type': 'command', 'command': command})
        return

    entries.append({
        'matcher': matcher,
        'hooks': [{'type': 'command', 'command': command}],
    })


def inject_codex_hooks(cwd: str) -> None:
    """Injeta hooks PermissionRequest/PreToolUse/PostToolUse no .codex/hooks.json."""
    file_path = os.path.join(cwd, '.codex', 'hooks.json')
    data = _read_json(file_path)

    hooks = data.setdefault('hooks', {})

    pre_permission_hooks = hooks.setdefault('PermissionRequest', [])
    _merge_codex_hook_entry(
        pre_permission_hooks, '.*', 'scripts/trackfw-attention-signal.sh',
    )

    # Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A): skip project-scope
    # credential-guard when the global one is already installed.
    skip_cg = _global_credential_guard_installed_codex()

    pre_tool_hooks = hooks.setdefault('PreToolUse', [])
    if not skip_cg:
        _merge_codex_hook_entry(
            pre_tool_hooks, 'Bash', 'scripts/trackfw-credential-guard.sh',
        )

    post_hooks = hooks.setdefault('PostToolUse', [])
    _merge_codex_hook_entry(
        post_hooks, '.*', 'scripts/trackfw-attention-cleanup.sh',
    )
    if not skip_cg:
        _merge_codex_hook_entry(
            post_hooks, 'Bash', 'scripts/trackfw-credential-guard.sh',
        )

    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Gemini — .gemini/settings.json
#
# Three independent hook events: Notification (matcher "ToolPermission") for the
# existing attention-signal -- only fires when Gemini CLI is about to prompt for
# permission, not for every tool call -- and BeforeTool/AfterTool (matcher
# "run_shell_command") for credential-guard, which fires for every shell tool call
# regardless of whether a permission prompt is needed. Confirmed against
# https://geminicli.com/docs/hooks/reference (retrieved 2026-08-05): BeforeTool
# "Fires before a tool is invoked. Used for argument validation, security checks,
# and parameter rewriting" and supports "Exit Code 2 (Block Tool): Prevents
# execution. Uses stderr as the reason" -- matching trackfw-credential-guard.sh's
# existing "block" mode. The shell tool's canonical name is "run_shell_command"
# (doc: "you can match any built-in tool (for example, read_file,
# run_shell_command)"); matcher is a regex evaluated against tool_name. AfterTool
# (matcher "*") is the pre-existing attention-cleanup wiring, unrelated to the new
# credential-guard entry added as a separate array entry (different matcher) in the
# same event.
#
# Design note (ML-2C): rewritten to use the shared `_merge_claude_hook_array`
# helper -- already used by `inject_claude_hooks` -- instead of the bespoke
# "does any entry contain this command" checks the previous version of this
# function used. That inline pattern would append a *second* group with the same
# matcher when a third-party group already existed for it, the exact divergence
# ML-2A fixed in Go's `mergeClaudeHookArray` and ML-2B fixed in Python's
# `_merge_codex_hook_entry`. As a side effect, the `name`/`timeout: 10000` fields
# this function used to write for Gemini entries (which Go/Node never wrote) are
# dropped here to match Go/Node/`_merge_claude_hook_array` output shape byte-for-
# byte -- structural cross-stack parity (ML-3A's gate) takes precedence over
# preserving those two informational-only fields.
# ---------------------------------------------------------------------------

def inject_gemini_hooks(cwd: str) -> None:
    """Injeta hooks Notification/BeforeTool/AfterTool no .gemini/settings.json."""
    file_path = os.path.join(cwd, '.gemini', 'settings.json')
    data = _read_json(file_path)

    hooks = data.setdefault('hooks', {})

    notifications = hooks.setdefault('Notification', [])
    _merge_claude_hook_array(notifications, 'ToolPermission', 'scripts/trackfw-attention-signal.sh')

    # Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A): skip project-scope
    # credential-guard when the global one is already installed.
    skip_cg = _global_credential_guard_installed_gemini()

    before = hooks.setdefault('BeforeTool', [])
    if not skip_cg:
        _merge_claude_hook_array(before, 'run_shell_command', 'scripts/trackfw-credential-guard.sh')

    after = hooks.setdefault('AfterTool', [])
    _merge_claude_hook_array(after, '*', 'scripts/trackfw-attention-cleanup.sh')
    if not skip_cg:
        _merge_claude_hook_array(after, 'run_shell_command', 'scripts/trackfw-credential-guard.sh')

    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Kiro — .kiro/hooks/trackfw-attention.json (arquivo dedicado, overwrite seguro)
# ---------------------------------------------------------------------------

# Kiro — .kiro/hooks/trackfw-attention.json (dedicated file, safe overwrite)
#
# Format confirmed against https://kiro.dev/docs/hooks/ , https://kiro.dev/docs/hooks/types and
# https://kiro.dev/docs/hooks/actions/ (retrieved 2026-08-05). Top level is {"version": "v1", "hooks":
# [...]} ("version" is the string "v1"), each entry {"name", "description"?, "trigger", "matcher"?,
# "action", ...}. The field is "trigger" (NOT "event" as previously emitted here and in the Go/Node
# siblings -- "event" does not exist in the documented schema). "matcher" is a plain regex string
# matched against tool name for PreToolUse/PostToolUse (NOT an object like {"tool_name": ".*"} as
# previously emitted) -- "*" is the documented wildcard for "all tools"; ".*" is not a documented
# matcher value. PreToolUse ("Before a tool is about to execute", Can block: Yes) is confirmed distinct
# from PostFileSave/file-save events, resolving the ADR's open question about Kiro intercepting shell
# commands pre-execution. Blocking contract: any non-zero exit from a PreToolUse command hook blocks
# the tool invocation (stricter than the exit-code-2-specific contract of Claude Code/Codex/Gemini);
# trackfw-credential-guard.sh only ever exits 0 or 2 on its normal-operation paths (ML-1A), so this is
# safe. Shell tool matcher uses the documented alias "shell" ("all built-in shell command-related
# tools"), broader than the single canonical tool id "execute_bash". This file is fully
# generated/overwritten by trackfw (not merged with user content), so the legacy attention-signal/
# cleanup entries are realigned to the correct schema here too rather than left in the old, never-valid
# shape (same situation as the GitHub Copilot fix in ML-2D).
def inject_kiro_hooks(cwd: str) -> None:
    """Cria/sobrescreve .kiro/hooks/trackfw-attention.json."""
    file_path = os.path.join(cwd, '.kiro', 'hooks', 'trackfw-attention.json')
    hooks = [
        {
            'name': 'trackfw-attention-signal',
            'description': 'Signals trackfw board when agent executes a tool',
            'trigger': 'PreToolUse',
            'matcher': '*',
            'action': {'type': 'command', 'command': 'scripts/trackfw-attention-signal.sh'},
        },
        {
            'name': 'trackfw-attention-cleanup',
            'description': 'Clears trackfw board attention after tool completes',
            'trigger': 'PostToolUse',
            'matcher': '*',
            'action': {'type': 'command', 'command': 'scripts/trackfw-attention-cleanup.sh'},
        },
    ]

    # Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A): skip project-scope
    # credential-guard entries when the global one is already installed.
    if not _global_credential_guard_installed_kiro():
        hooks.append({
            'name': 'trackfw-credential-guard-pre',
            'description': 'Blocks/warns on possible plaintext credential materialization before a shell command executes',
            'trigger': 'PreToolUse',
            'matcher': 'shell',
            'action': {'type': 'command', 'command': 'scripts/trackfw-credential-guard.sh'},
        })
        hooks.append({
            'name': 'trackfw-credential-guard-post',
            'description': 'Warns on possible plaintext credential materialization after a shell command executes',
            'trigger': 'PostToolUse',
            'matcher': 'shell',
            'action': {'type': 'command', 'command': 'scripts/trackfw-credential-guard.sh'},
        })

    data = {'version': 'v1', 'hooks': hooks}
    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Copilot — .github/hooks/trackfw-attention.json (arquivo dedicado, overwrite seguro)
#
# Format confirmed against https://docs.github.com/en/copilot/reference/hooks-reference (retrieved
# 2026-08-05): repository-level hook files live at .github/hooks/*.json, using the schema
# {"version": 1, "hooks": {"<event>": [<command entry>, ...]}}, where a command entry is
# {"type": "command", "bash": "...", "cwd": "...", "timeoutSec": N}. This is the format this function
# already used before this ML -- Go and Node previously emitted a different, undocumented
# {"hooks": [{"event", "run"}]} shape and were aligned to this one (Python was correct).
#
# Matcher: the doc's matcher-filtering table lists `preToolUse -> toolName` and `postToolUse ->
# toolName` (a regex, anchored `^(?:PATTERN)$`), and shows a worked `"matcher"` field inline on a
# postToolUse command entry. With camelCase event names (preToolUse/postToolUse, used here), toolName
# carries the runtime tool name, and the shell tool's runtime name is "bash" (lowercase) -- distinct
# from PascalCase events, which report the Claude-mapped name "Bash". trackfw-credential-guard.sh
# scans the raw JSON payload for JWT/AWS-key patterns regardless of field names (ML-1A), so it works
# under either payload shape; the matcher below is a scope-narrowing optimization only.
#
# Concurrency: "If multiple hooks of the same type are configured, they execute in order" (same
# section) -- Copilot hooks run serially, in configured order, for the same event, unlike Codex's
# confirmed-concurrent or Gemini's undocumented cross-group model. The ML-1A fix (credential-guard's
# "warn" mode writes to its own dedicated $ROADMAP_DIR/.trackfw-credential-guard.json, never touching
# the shared .trackfw-attention.json that trackfw-attention-cleanup.sh deletes) makes ordering moot
# regardless.
# ---------------------------------------------------------------------------

def inject_copilot_hooks(cwd: str) -> None:
    """Cria/sobrescreve .github/hooks/trackfw-attention.json."""
    file_path = os.path.join(cwd, '.github', 'hooks', 'trackfw-attention.json')

    pre_tool_use = [
        {
            'type': 'command',
            'bash': 'scripts/trackfw-attention-signal.sh',
            'cwd': '.',
            'timeoutSec': 10,
        },
    ]
    post_tool_use = [
        {
            'type': 'command',
            'bash': 'scripts/trackfw-attention-cleanup.sh',
            'cwd': '.',
            'timeoutSec': 10,
        },
    ]

    # Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A): skip project-scope
    # credential-guard entries when the global one is already installed.
    if not _global_credential_guard_installed_copilot():
        guard_entry = {
            'type': 'command',
            'matcher': 'bash',
            'bash': 'scripts/trackfw-credential-guard.sh',
            'cwd': '.',
            'timeoutSec': 10,
        }
        pre_tool_use.append(dict(guard_entry))
        post_tool_use.append(dict(guard_entry))

    data = {
        'version': 1,
        'hooks': {
            'preToolUse': pre_tool_use,
            'postToolUse': post_tool_use,
        },
    }
    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Cursor — .cursor/hooks.json
#
# Two independent things are wired here, both nested under the real Cursor
# hook config `{"version": 1, "hooks": {"<eventName>": [...] }}`:
#   - hooks.preToolUse + hooks.postToolUse (migrated by this ML) --
#     attention-signal/cleanup. Prior to this ML these were written to
#     top-level preToolUse/postToolUse arrays, which did not match any
#     documented Cursor event (confirmed 2026-08-05, see docs/cli-parity.md
#     "Cursor wiring (ML-2E)"). Re-fetching https://cursor.com/docs/hooks on
#     2026-08-06 (the /docs/agent/hooks URL now 308-redirects there) shows
#     Cursor's docs were updated in the interim to add three new generic
#     events: preToolUse/postToolUse/postToolUseFailure, "fires for all tool
#     types (Shell, Read, Write, MCP, Task, etc.)". preToolUse's documented
#     input is `{"tool_name","tool_input":{...},"tool_use_id","cwd",...}`
#     and postToolUse's is the same shape plus `tool_output`/`duration` --
#     structurally identical to Claude Code's PreToolUse/PostToolUse payload
#     (`tool_name`/`tool_input`), which is exactly the shape
#     scripts/trackfw-attention-signal.sh and trackfw-attention-cleanup.sh
#     already parse (`.tool_name`, `.tool_input.question // .tool_input.command`).
#     No script changes were needed. Per-hook `matcher` filters by tool type
#     (e.g. "Shell|Read|Write") and is optional; intentionally omitted here,
#     same reasoning as beforeShellExecution below -- the attention signal
#     must fire for every tool use, not a filtered subset.
#   - hooks.beforeShellExecution + hooks.afterShellExecution (ML-2E, prior
#     cycle) -- credential-guard. beforeShellExecution is the real,
#     Bash-specific, pre-execution event: input is
#     `{"command","cwd","sandbox"}`, response (stdout JSON, only read on
#     exit code 0) is `{"permission":"allow"|"deny"|"ask","user_message":"...",
#     "agent_message":"..."}`. Per the documented "Exit code behavior": exit 0 uses the
#     JSON output (or defaults to allow if stdout has none -- confirmed by the doc's own
#     minimal example hook, which exits 0 with no stdout at all), exit 2 blocks the
#     action ("equivalent to returning permission: \"deny\""), any other exit code
#     fail-opens (hook failed, action proceeds). This is already exactly
#     trackfw-credential-guard.sh's existing contract (block mode -> exit 2 + stderr, warn
#     mode -> exit 0), so no script changes were needed to wire Cursor. afterShellExecution
#     is a post-execution audit-only event (input adds "output"/"duration", no
#     allow/deny/ask response defined) -- added in parallel for symmetry with the
#     PostToolUse wiring already used for the other CLIs in this wave. Concurrency between
#     hooks registered on the same event was not documented on the page retrieved for this
#     investigation (unlike Codex, which explicitly documents concurrent execution); not
#     assumed either way -- not a blocker here since this event array only ever contains
#     the single credential-guard entry added by trackfw.
#
# Backward compatibility: a .cursor/hooks.json written by a pre-migration
# trackfw still has the legacy top-level preToolUse/postToolUse arrays. This
# function migrates known trackfw entries out of those top-level arrays into
# the nested hooks.preToolUse/hooks.postToolUse location, and drops the
# top-level key entirely once it is empty -- but never touches or deletes
# unrelated entries a user may have added there themselves (those keys are
# inert either way -- Cursor never read the top-level location -- so leaving
# them is harmless and avoids destroying unrelated user data on a guess).
# ---------------------------------------------------------------------------

def _remove_known_command_from_legacy_top_level_array(data: dict, key: str, command: str) -> None:
    arr = data.get(key)
    if not isinstance(arr, list):
        return
    kept = [item for item in arr if not (isinstance(item, dict) and item.get('command') == command)]
    if kept:
        data[key] = kept
    else:
        del data[key]


def inject_cursor_hooks(cwd: str) -> None:
    """Injeta hooks.preToolUse/postToolUse e hooks.beforeShellExecution/afterShellExecution
    (credential-guard) no .cursor/hooks.json, migrando entradas legadas de nível raiz."""
    file_path = os.path.join(cwd, '.cursor', 'hooks.json')
    data = _read_json(file_path)

    if 'version' not in data:
        data['version'] = 1
    hooks = data.get('hooks')
    if not isinstance(hooks, dict):
        hooks = {}
        data['hooks'] = hooks

    # Migra qualquer entrada legada de nível raiz (escrita pelo trackfw antes
    # deste ML) para o local real e aninhado.
    pre = hooks.setdefault('preToolUse', [])
    if not _has_entry(pre, 'command', 'scripts/trackfw-attention-signal.sh'):
        pre.append({'command': 'scripts/trackfw-attention-signal.sh'})
    _remove_known_command_from_legacy_top_level_array(data, 'preToolUse', 'scripts/trackfw-attention-signal.sh')

    post = hooks.setdefault('postToolUse', [])
    if not _has_entry(post, 'command', 'scripts/trackfw-attention-cleanup.sh'):
        post.append({'command': 'scripts/trackfw-attention-cleanup.sh'})
    _remove_known_command_from_legacy_top_level_array(data, 'postToolUse', 'scripts/trackfw-attention-cleanup.sh')

    # Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A): skip project-scope
    # credential-guard entries when the global one is already installed.
    if not _global_credential_guard_installed_cursor():
        before = hooks.setdefault('beforeShellExecution', [])
        if not _has_entry(before, 'command', 'scripts/trackfw-credential-guard.sh'):
            before.append({'command': 'scripts/trackfw-credential-guard.sh'})

        after = hooks.setdefault('afterShellExecution', [])
        if not _has_entry(after, 'command', 'scripts/trackfw-credential-guard.sh'):
            after.append({'command': 'scripts/trackfw-credential-guard.sh'})

    _write_json(file_path, data)


def inject_windsurf_hooks(cwd: str) -> None:
    """Atualiza .windsurfrules com a diretiva de regras do trackfw."""
    from trackfw.generators.init_gen import inject_rules_for_tool
    inject_rules_for_tool('windsurf', cwd)


# ---------------------------------------------------------------------------
# Ponto de entrada público — detecção automática
# ---------------------------------------------------------------------------

def inject_hooks_detected(cwd: str) -> None:
    """
    Detecta CLIs presentes no projeto e injeta hooks de atenção em cada um.
    Erros são não-fatais: reportados mas não interrompem o fluxo.
    """
    try:
        from trackfw.generators.init_gen import _generate_attention_scripts
        _generate_attention_scripts(cwd)
    except Exception as e:
        print(f'  ⚠ attention scripts: {e}')

    try:
        from trackfw.generators.init_gen import _generate_credential_guard_script
        _generate_credential_guard_script(cwd)
    except Exception as e:
        print(f'  ⚠ credential guard script: {e}')

    detections = {
        'claude': (
            lambda: os.path.isdir(os.path.join(cwd, '.claude')) or os.path.isfile(os.path.join(cwd, 'CLAUDE.md')),
            inject_claude_hooks,
        ),
        'codex': (
            lambda: os.path.isfile(os.path.join(cwd, 'AGENTS.md')) or os.path.isdir(os.path.join(cwd, '.codex')),
            inject_codex_hooks,
        ),
        'gemini': (
            lambda: os.path.isfile(os.path.join(cwd, 'GEMINI.md')) or os.path.isdir(os.path.join(cwd, '.gemini')),
            inject_gemini_hooks,
        ),
        'kiro': (
            lambda: os.path.isdir(os.path.join(cwd, '.kiro')),
            inject_kiro_hooks,
        ),
        'copilot': (
            lambda: os.path.isfile(os.path.join(cwd, '.github', 'copilot-instructions.md')),
            inject_copilot_hooks,
        ),
        'cursor': (
            lambda: os.path.isdir(os.path.join(cwd, '.cursor')),
            inject_cursor_hooks,
        ),
        'windsurf': (
            lambda: os.path.isfile(os.path.join(cwd, '.windsurfrules')),
            inject_windsurf_hooks,
        ),
    }

    for name, (check, fn) in detections.items():
        try:
            if check():
                fn(cwd)
        except Exception as e:
            print(f'  ⚠ {name} hooks: {e}')
