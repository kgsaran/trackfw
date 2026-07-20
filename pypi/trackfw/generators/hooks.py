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


# ---------------------------------------------------------------------------
# Claude Code — .claude/settings.json
# ---------------------------------------------------------------------------

def inject_claude_hooks(cwd: str) -> None:
    """Injeta hooks PreToolUse/PostToolUse no .claude/settings.json."""
    file_path = os.path.join(cwd, '.claude', 'settings.json')
    data = _read_json(file_path)

    hooks = data.setdefault('hooks', {})

    # PreToolUse — AskUserQuestion matcher → signal
    pre_hooks = hooks.setdefault('PreToolUse', [])
    if not _has_entry(pre_hooks, 'matcher', 'AskUserQuestion'):
        pre_hooks.append({
            'matcher': 'AskUserQuestion',
            'hooks': [
                {'type': 'command', 'command': 'scripts/trackfw-attention-signal.sh'}
            ],
        })
    else:
        # garante que o command está presente na entrada existente
        for entry in pre_hooks:
            if isinstance(entry, dict) and entry.get('matcher') == 'AskUserQuestion':
                inner = entry.setdefault('hooks', [])
                if not _has_entry(inner, 'command', 'scripts/trackfw-attention-signal.sh'):
                    inner.append({'type': 'command', 'command': 'scripts/trackfw-attention-signal.sh'})

    # PostToolUse — AskUserQuestion matcher → cleanup
    post_hooks = hooks.setdefault('PostToolUse', [])
    if not _has_entry(post_hooks, 'matcher', 'AskUserQuestion'):
        post_hooks.append({
            'matcher': 'AskUserQuestion',
            'hooks': [
                {'type': 'command', 'command': 'scripts/trackfw-attention-cleanup.sh'}
            ],
        })
    else:
        for entry in post_hooks:
            if isinstance(entry, dict) and entry.get('matcher') == 'AskUserQuestion':
                inner = entry.setdefault('hooks', [])
                if not _has_entry(inner, 'command', 'scripts/trackfw-attention-cleanup.sh'):
                    inner.append({'type': 'command', 'command': 'scripts/trackfw-attention-cleanup.sh'})

    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Codex — .codex/hooks.json
# ---------------------------------------------------------------------------

def inject_codex_hooks(cwd: str) -> None:
    """Injeta hooks PermissionRequest/PostToolUse no .codex/hooks.json."""
    file_path = os.path.join(cwd, '.codex', 'hooks.json')
    data = _read_json(file_path)

    hooks = data.setdefault('hooks', {})

    def has_nested_command(entries, command):
        return any(
            any(h.get('command') == command for h in entry.get('hooks', []))
            for entry in entries
        )

    pre_hooks = hooks.setdefault('PermissionRequest', [])
    if not has_nested_command(pre_hooks, 'scripts/trackfw-attention-signal.sh'):
        pre_hooks.append({
            'matcher': '.*',
            'hooks': [{
                'type': 'command',
                'command': 'scripts/trackfw-attention-signal.sh',
                'timeout': 10,
                'statusMessage': 'Waiting for approval',
            }],
        })

    post_hooks = hooks.setdefault('PostToolUse', [])
    if not has_nested_command(post_hooks, 'scripts/trackfw-attention-cleanup.sh'):
        post_hooks.append({
            'matcher': '.*',
            'hooks': [{
                'type': 'command',
                'command': 'scripts/trackfw-attention-cleanup.sh',
                'timeout': 10,
            }],
        })

    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Gemini — .gemini/settings.json
# ---------------------------------------------------------------------------

def inject_gemini_hooks(cwd: str) -> None:
    """Injeta hooks Notification/AfterTool no .gemini/settings.json."""
    file_path = os.path.join(cwd, '.gemini', 'settings.json')
    data = _read_json(file_path)

    hooks = data.setdefault('hooks', {})

    notifications = hooks.setdefault('Notification', [])
    if not any(
        entry.get('matcher') == 'ToolPermission'
        and any(
            hook.get('command') == 'scripts/trackfw-attention-signal.sh'
            for hook in entry.get('hooks', [])
        )
        for entry in notifications
    ):
        notifications.append({
            'matcher': 'ToolPermission',
            'hooks': [{
                'name': 'trackfw-attention-signal',
                'type': 'command',
                'command': 'scripts/trackfw-attention-signal.sh',
                'timeout': 10000,
            }],
        })

    after = hooks.setdefault('AfterTool', [])
    if not any(
        any(
            hook.get('command') == 'scripts/trackfw-attention-cleanup.sh'
            for hook in entry.get('hooks', [])
        )
        for entry in after
    ):
        after.append({
            'matcher': '*',
            'hooks': [{
                'name': 'trackfw-attention-cleanup',
                'type': 'command',
                'command': 'scripts/trackfw-attention-cleanup.sh',
                'timeout': 10000,
            }],
        })

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
# ---------------------------------------------------------------------------

def inject_copilot_hooks(cwd: str) -> None:
    """Cria/sobrescreve .github/hooks/trackfw-attention.json."""
    file_path = os.path.join(cwd, '.github', 'hooks', 'trackfw-attention.json')
    data = {
        'version': 1,
        'hooks': {
            'preToolUse': [{
                'type': 'command',
                'bash': 'scripts/trackfw-attention-signal.sh',
                'cwd': '.',
                'timeoutSec': 10,
            }],
            'postToolUse': [{
                'type': 'command',
                'bash': 'scripts/trackfw-attention-cleanup.sh',
                'cwd': '.',
                'timeoutSec': 10,
            }],
        },
    }
    _write_json(file_path, data)


# ---------------------------------------------------------------------------
# Cursor — .cursor/hooks.json
# ---------------------------------------------------------------------------

def inject_cursor_hooks(cwd: str) -> None:
    """Injeta hooks preToolUse/postToolUse no .cursor/hooks.json."""
    file_path = os.path.join(cwd, '.cursor', 'hooks.json')
    data = _read_json(file_path)

    pre = data.setdefault('preToolUse', [])
    if not _has_entry(pre, 'command', 'scripts/trackfw-attention-signal.sh'):
        pre.append({'command': 'scripts/trackfw-attention-signal.sh'})

    post = data.setdefault('postToolUse', [])
    if not _has_entry(post, 'command', 'scripts/trackfw-attention-cleanup.sh'):
        post.append({'command': 'scripts/trackfw-attention-cleanup.sh'})

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
