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
    _merge_claude_hook_array(pre_hooks, 'Bash', 'scripts/trackfw-credential-guard.sh')

    # PostToolUse — AskUserQuestion matcher → cleanup; Bash matcher → credential guard
    post_hooks = hooks.setdefault('PostToolUse', [])
    _merge_claude_hook_array(post_hooks, 'AskUserQuestion', 'scripts/trackfw-attention-cleanup.sh')
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

def _merge_codex_hook_entry(entries: list, matcher: str, command: str, **extra_fields) -> None:
    """Garante (idempotente) que `entries` (um array PreToolUse/PostToolUse/etc.
    do formato Codex) tenha uma entrada `matcher` contendo `command`.

    Mirrors `_merge_claude_hook_array`: if an entry with the given matcher
    already exists (e.g. a third-party hook, or a previous trackfw run), the
    new command is merged into its `hooks` array instead of appending a
    duplicate `{"matcher": ...}` block. `extra_fields` (timeout,
    statusMessage, ...) are only applied when creating a brand-new hook
    entry, matching the fields Codex hooks commonly carry.
    """
    for entry in entries:
        if not isinstance(entry, dict) or entry.get('matcher') != matcher:
            continue
        inner = entry.setdefault('hooks', [])
        if not _has_entry(inner, 'command', command):
            inner.append({'type': 'command', 'command': command, **extra_fields})
        return

    entries.append({
        'matcher': matcher,
        'hooks': [{'type': 'command', 'command': command, **extra_fields}],
    })


def inject_codex_hooks(cwd: str) -> None:
    """Injeta hooks PermissionRequest/PreToolUse/PostToolUse no .codex/hooks.json."""
    file_path = os.path.join(cwd, '.codex', 'hooks.json')
    data = _read_json(file_path)

    hooks = data.setdefault('hooks', {})

    pre_permission_hooks = hooks.setdefault('PermissionRequest', [])
    _merge_codex_hook_entry(
        pre_permission_hooks, '.*', 'scripts/trackfw-attention-signal.sh',
        timeout=10, statusMessage='Waiting for approval',
    )

    pre_tool_hooks = hooks.setdefault('PreToolUse', [])
    _merge_codex_hook_entry(
        pre_tool_hooks, 'Bash', 'scripts/trackfw-credential-guard.sh',
        timeout=10, statusMessage='Scanning command for credentials',
    )

    post_hooks = hooks.setdefault('PostToolUse', [])
    _merge_codex_hook_entry(
        post_hooks, '.*', 'scripts/trackfw-attention-cleanup.sh',
        timeout=10,
    )
    _merge_codex_hook_entry(
        post_hooks, 'Bash', 'scripts/trackfw-credential-guard.sh',
        timeout=10, statusMessage='Scanning command output for credentials',
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

    before = hooks.setdefault('BeforeTool', [])
    _merge_claude_hook_array(before, 'run_shell_command', 'scripts/trackfw-credential-guard.sh')

    after = hooks.setdefault('AfterTool', [])
    _merge_claude_hook_array(after, '*', 'scripts/trackfw-attention-cleanup.sh')
    _merge_claude_hook_array(after, 'run_shell_command', 'scripts/trackfw-credential-guard.sh')

    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Kiro — .kiro/hooks/trackfw-attention.json (arquivo dedicado, overwrite seguro)
# ---------------------------------------------------------------------------

def inject_kiro_hooks(cwd: str) -> None:
    """Cria/sobrescreve .kiro/hooks/trackfw-attention.json."""
    file_path = os.path.join(cwd, '.kiro', 'hooks', 'trackfw-attention.json')
    data = {
        'hooks': [
            {
                'name': 'trackfw-attention-signal',
                'event': 'PreToolUse',
                'matcher': {'tool_name': '.*'},
                'action': {'type': 'command', 'command': 'scripts/trackfw-attention-signal.sh'},
            },
            {
                'name': 'trackfw-attention-cleanup',
                'event': 'PostToolUse',
                'matcher': {'tool_name': '.*'},
                'action': {'type': 'command', 'command': 'scripts/trackfw-attention-cleanup.sh'},
            },
        ]
    }
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
    data = {
        'version': 1,
        'hooks': {
            'preToolUse': [
                {
                    'type': 'command',
                    'bash': 'scripts/trackfw-attention-signal.sh',
                    'cwd': '.',
                    'timeoutSec': 10,
                },
                {
                    'type': 'command',
                    'matcher': 'bash',
                    'bash': 'scripts/trackfw-credential-guard.sh',
                    'cwd': '.',
                    'timeoutSec': 10,
                },
            ],
            'postToolUse': [
                {
                    'type': 'command',
                    'bash': 'scripts/trackfw-attention-cleanup.sh',
                    'cwd': '.',
                    'timeoutSec': 10,
                },
                {
                    'type': 'command',
                    'matcher': 'bash',
                    'bash': 'scripts/trackfw-credential-guard.sh',
                    'cwd': '.',
                    'timeoutSec': 10,
                },
            ],
        },
    }
    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Cursor — .cursor/hooks.json
#
# Two independent things are wired here:
#   - Top-level preToolUse/postToolUse (existing attention-signal/cleanup) -- kept as-is,
#     NOT migrated by this function. These keys do not match any event documented at
#     https://cursor.com/docs/agent/hooks (retrieved 2026-08-05): the real Cursor hook
#     config is `{"version": 1, "hooks": {"<eventName>": [...] }}`, and the documented
#     event names are sessionStart/sessionEnd/beforeShellExecution/beforeMCPExecution/
#     afterShellExecution/afterMCPExecution/beforeReadFile/afterFileEdit/
#     beforeSubmitPrompt/preCompact/stop/beforeTabFileRead/afterTabFileEdit -- there is no
#     generic preToolUse/postToolUse event at all. Re-scoping the legacy attention hooks
#     to a real event is out of scope for this ML; tracked as a follow-up (see
#     docs/cli-parity.md, "Cursor wiring (ML-2E)").
#   - hooks.beforeShellExecution + hooks.afterShellExecution (new, this ML) --
#     credential-guard. beforeShellExecution is the real, Bash-specific, pre-execution
#     event: input is `{"command","cwd","sandbox"}`, response (stdout JSON, only read on
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
#     PostToolUse wiring already used for the other CLIs in this wave. Per-event `matcher`
#     (regex against the command string itself, not a tool name -- the event is already
#     shell-specific) is optional and intentionally omitted: the guard must see every
#     shell command, not a filtered subset. Concurrency between hooks registered on the
#     same event was not documented on the page retrieved for this investigation (unlike
#     Codex, which explicitly documents concurrent execution); not assumed either way --
#     not a blocker here since this event array only ever contains the single
#     credential-guard entry added by trackfw.
# ---------------------------------------------------------------------------

def inject_cursor_hooks(cwd: str) -> None:
    """Injeta hooks preToolUse/postToolUse e hooks.beforeShellExecution/afterShellExecution
    (credential-guard) no .cursor/hooks.json."""
    file_path = os.path.join(cwd, '.cursor', 'hooks.json')
    data = _read_json(file_path)

    pre = data.setdefault('preToolUse', [])
    if not _has_entry(pre, 'command', 'scripts/trackfw-attention-signal.sh'):
        pre.append({'command': 'scripts/trackfw-attention-signal.sh'})

    post = data.setdefault('postToolUse', [])
    if not _has_entry(post, 'command', 'scripts/trackfw-attention-cleanup.sh'):
        post.append({'command': 'scripts/trackfw-attention-cleanup.sh'})

    if 'version' not in data:
        data['version'] = 1
    hooks = data.get('hooks')
    if not isinstance(hooks, dict):
        hooks = {}
        data['hooks'] = hooks

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
