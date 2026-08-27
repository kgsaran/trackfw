"""
audit_surface.py — Comando `trackfw audit-surface` (Python CLI).

AC16 invariant (no false positives, by construction): this module NEVER reads
file content looking for hook-path strings. It ONLY opens the 8 exact wiring-file
paths defined in RUNTIME_WIRING_PATH. Files like docs/cli-parity.md and
internal/generators/agentfiles.go happen to mention those paths as strings but are
never opened here — they live at paths not in RUNTIME_WIRING_PATH.

Mirrors internal/auditsurface/auditsurface.go and npm/src/commands/audit-surface.js.
"""

import hashlib
import json
import posixpath
import subprocess
import sys


# Runtime names in canonical order (matches check-agent-hooks-parity.sh CLIS).
CANONICAL_RUNTIMES = ["claude", "codex", "gemini", "copilot", "cursor", "kiro", "windsurf", "amazonq"]

RUNTIME_WIRING_PATH = {
    "claude": ".claude/settings.json",
    "codex": ".codex/hooks.json",
    "gemini": ".gemini/settings.json",
    "copilot": ".github/hooks/trackfw-attention.json",
    "cursor": ".cursor/hooks.json",
    "kiro": ".kiro/hooks/trackfw-attention.json",
    "windsurf": ".windsurf/hooks.json",
    "amazonq": ".amazonq/cli-agents/q_cli_default.json",
}

INSTRUCTION_FILE_PATHS = [
    "CLAUDE.md",
    "AGENTS.md",
    "GEMINI.md",
    ".windsurfrules",
    ".github/copilot-instructions.md",
    ".amazonq/developer/guidelines.md",
    ".cursor/rules/trackfw.mdc",
]


def git_show(ref: str, file_path: str, git_root: str) -> bytes | None:
    """Read a file at a given ref via `git show <ref>:<path>`. Returns None if absent."""
    result = subprocess.run(
        ["git", "show", f"{ref}:{file_path}"],
        cwd=git_root,
        capture_output=True,
    )
    if result.returncode != 0:
        return None
    return result.stdout


def git_ls_tree(ref: str, directory: str, git_root: str) -> list[str]:
    """List files under a directory at a given ref."""
    result = subprocess.run(
        ["git", "ls-tree", "-r", "--name-only", ref, "--", directory],
        cwd=git_root,
        capture_output=True,
    )
    if result.returncode != 0:
        return []
    lines = [line for line in result.stdout.decode().rstrip("\n").split("\n") if line]
    return sorted(lines)


def find_git_root(cwd: str | None = None) -> str | None:
    """Return the git repository root."""
    result = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        cwd=cwd,
        capture_output=True,
    )
    if result.returncode != 0:
        return None
    return result.stdout.decode().strip()


def normalize_command(raw_cmd: str) -> str:
    """Strip known project-root env-var prefixes to get a repo-relative script path.
    Returns '' if genuinely unresolvable (inline pipeline, -c string, builtin, etc.).

    Resolved forms:
      bare path:           "scripts/hook.sh"
      path with args:      "scripts/hook.sh --strict" -> "scripts/hook.sh"
      interpreter prefix:  "bash scripts/hook.sh"     -> "scripts/hook.sh"
      project-root prefix: "$CLAUDE_PROJECT_DIR/scripts/hook.sh" -> "scripts/hook.sh"

    Recognised script extensions: .sh .bash .zsh .py .js .rb .pl .fish
    """
    cmd = raw_cmd.strip()
    # Strip surrounding double-quotes (Codex format).
    if cmd.startswith('"') and cmd.endswith('"'):
        cmd = cmd[1:-1]

    prefixes = [
        "$CLAUDE_PROJECT_DIR/",
        "$GEMINI_PROJECT_DIR/",
        "$(git rev-parse --show-toplevel)/",
    ]

    def strip_prefix(s: str) -> str:
        for p in prefixes:
            if s.startswith(p):
                return s[len(p):]
        return s

    cmd = strip_prefix(cmd)

    script_exts = (".sh", ".bash", ".zsh", ".py", ".js", ".rb", ".pl", ".fish")

    # No spaces: bare path — accept if it has a recognised script extension.
    if " " not in cmd:
        return cmd if cmd.endswith(script_exts) else ""

    # Command has spaces: interpreter prefix or path-with-arguments.
    interpreters = {
        "bash", "sh", "dash", "zsh",
        "python", "python3", "python2",
        "node", "nodejs",
        "ruby", "perl",
    }
    tokens = cmd.split()
    candidate = ""

    if tokens[0] in interpreters:
        # Interpreter prefix form: skip flags to find the script argument.
        for tok in tokens[1:]:
            if tok == "-c":
                return ""  # inline script string, not a file path
            if tok.startswith("-"):
                continue  # flags: -e, -u, -x, etc.
            candidate = tok
            break
    else:
        # No interpreter: first token is the script path, rest are arguments.
        candidate = tokens[0]

    if not candidate:
        return ""

    # Strip project-root prefixes from candidate too.
    candidate = strip_prefix(candidate)

    return candidate if candidate.endswith(script_exts) else ""


def get_symlink_target(ref: str, script_path: str, git_root: str) -> tuple[str, bool]:
    """Check whether path at ref is a git symlink (mode 120000).
    Returns (target, True) if symlink, ('', False) otherwise."""
    result = subprocess.run(
        ["git", "ls-tree", ref, "--", script_path],
        cwd=git_root,
        capture_output=True,
    )
    if result.returncode != 0 or not result.stdout:
        return "", False
    line = result.stdout.decode()
    tab_idx = line.find("\t")
    if tab_idx < 0:
        return "", False
    fields = line[:tab_idx].split()
    if not fields or fields[0] != "120000":
        return "", False
    # Read the symlink target string.
    target_bytes = git_show(ref, script_path, git_root)
    if target_bytes is None:
        return "", False
    return target_bytes.decode().strip(), True


def resolve_script_digest(ref: str, script_path: str, git_root: str) -> str:
    """Follow the symlink chain (if any) at script_path with cycle detection and
    a depth limit, then return the digest string.

    Marker format: "symlink-><first_target>|<outcome>" for symlink chains;
    plain "sha256:<hex>" or "not-found" for regular files.

    Outcomes for symlink chains:
      sha256:<hex>            — resolved to real content
      not-found               — final target absent at this ref
      not-supported           — absolute symlink target
      circular-not-supported  — cycle detected
      chain-not-supported     — depth limit (8 hops) exceeded
    """
    max_hops = 8
    visited: set[str] = {script_path}
    current = script_path
    first_target = ""  # first symlink target, for the "symlink->" prefix

    for _ in range(max_hops):
        target, is_link = get_symlink_target(ref, current, git_root)
        if not is_link:
            # current is a regular file (or absent).
            if not first_target:
                # No symlink in the chain: plain digest.
                data = git_show(ref, current, git_root)
                return sha256hex(data) if data is not None else "not-found"
            # End of symlink chain.
            data = git_show(ref, current, git_root)
            if data is not None:
                return f"symlink->{first_target}|" + sha256hex(data)
            return f"symlink->{first_target}|not-found"

        if not first_target:
            first_target = target

        # Absolute target — cannot follow without checkout.
        if target.startswith("/"):
            return f"symlink->{first_target}|not-supported"

        # Resolve relative to current's directory, normalising ".." segments.
        link_dir = posixpath.dirname(current)
        resolved = posixpath.normpath(posixpath.join(link_dir, target))

        # Cycle detection.
        if resolved in visited:
            return f"symlink->{first_target}|circular-not-supported"
        visited.add(resolved)
        current = resolved

    # Depth limit exceeded.
    return f"symlink->{first_target}|chain-not-supported"


def sha256hex(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def extract_tuples(runtime: str, content: bytes) -> list[dict]:
    """Parse wiring-file JSON and return hook tuples per runtime schema."""
    try:
        root = json.loads(content.decode())
    except Exception as e:
        return [{"event": "parse-error", "matcher": "", "raw_command": str(e),
                 "script_path": "", "script_digest": "unresolvable"}]

    if runtime in ("claude", "codex", "amazonq", "gemini"):
        return extract_claude_schema(root)
    elif runtime == "kiro":
        return extract_kiro_schema(root)
    elif runtime == "copilot":
        return extract_copilot_schema(root)
    elif runtime == "cursor":
        return extract_cursor_schema(root)
    elif runtime == "windsurf":
        return extract_windsurf_schema(root)
    return []


def extract_claude_schema(root: dict) -> list[dict]:
    """{"hooks": {"EVENT": [{"matcher":"...","hooks":[{"command":"...","type":"command"}]}]}}"""
    hooks = root.get("hooks") or {}
    tuples = []
    for event in sorted(hooks.keys()):
        entries = sorted(hooks[event] or [], key=lambda e: e.get("matcher") or "")
        for entry in entries:
            for h in entry.get("hooks") or []:
                if h.get("type") != "command":
                    continue
                cmd = h.get("command") or ""
                tuples.append({
                    "event": event,
                    "matcher": entry.get("matcher") or "",
                    "raw_command": cmd,
                    "script_path": normalize_command(cmd),
                    "script_digest": "",
                })
    return tuples


def extract_kiro_schema(root: dict) -> list[dict]:
    """{"version":"v1","hooks":[{"trigger":"...","matcher":"...","action":{"type":"command","command":"..."}}]}"""
    hooks = sorted(root.get("hooks") or [],
                   key=lambda h: (h.get("trigger") or "", h.get("matcher") or ""))
    tuples = []
    for h in hooks:
        action = h.get("action") or {}
        if action.get("type") != "command":
            continue
        cmd = action.get("command") or ""
        tuples.append({
            "event": h.get("trigger") or "",
            "matcher": h.get("matcher") or "",
            "raw_command": cmd,
            "script_path": normalize_command(cmd),
            "script_digest": "",
        })
    return tuples


def extract_copilot_schema(root: dict) -> list[dict]:
    """{"version":1,"hooks":{"preToolUse":[{"type":"command","bash":"...","matcher":"..."}],...}}"""
    hooks = root.get("hooks") or {}
    tuples = []
    for event in sorted(hooks.keys()):
        entries = sorted(hooks[event] or [],
                         key=lambda e: (e.get("matcher") or "", e.get("bash") or ""))
        for entry in entries:
            if entry.get("type") != "command":
                continue
            cmd = entry.get("bash") or ""
            tuples.append({
                "event": event,
                "matcher": entry.get("matcher") or "",
                "raw_command": cmd,
                "script_path": normalize_command(cmd),
                "script_digest": "",
            })
    return tuples


def extract_cursor_schema(root: dict) -> list[dict]:
    """{"hooks":{"preToolUse":[{"command":"...","matcher":"..."}],...}}"""
    hooks = root.get("hooks") or {}
    tuples = []
    for event in sorted(hooks.keys()):
        entries = sorted(hooks[event] or [], key=lambda e: e.get("command") or "")
        for entry in entries:
            cmd = entry.get("command") or ""
            if not cmd:
                continue
            tuples.append({
                "event": event,
                "matcher": entry.get("matcher") or "",
                "raw_command": cmd,
                "script_path": normalize_command(cmd),
                "script_digest": "",
            })
    return tuples


def extract_windsurf_schema(root: dict) -> list[dict]:
    """{"hooks":{"pre_run_command":[{"command":"...","show_output":true}]}}"""
    hooks = root.get("hooks") or {}
    tuples = []
    for event in sorted(hooks.keys()):
        entries = sorted(hooks[event] or [], key=lambda e: e.get("command") or "")
        for entry in entries:
            cmd = entry.get("command") or ""
            if not cmd:
                continue
            tuples.append({
                "event": event,
                "matcher": "*",
                "raw_command": cmd,
                "script_path": normalize_command(cmd),
                "script_digest": "",
            })
    return tuples


def audit_lifecycle_hooks(ref: str, git_root: str) -> list[dict]:
    """Check npm lifecycle hooks, .husky/pre-commit, and .vscode/tasks.json."""
    hooks = []

    # npm lifecycle hooks: discover package.json by trying candidate paths in order.
    npm_candidates = ["package.json", "npm/package.json"]
    npm_found = None
    npm_content = None
    for c in npm_candidates:
        content = git_show(ref, c, git_root)
        if content is not None:
            npm_found = c
            npm_content = content
            break

    if npm_found is not None:
        try:
            pkg = json.loads(npm_content.decode())
        except Exception:
            pkg = {}
        scripts = pkg.get("scripts") or {}
        for key in ["preinstall", "postinstall", "prepare"]:
            if key in scripts:
                hooks.append({"file": npm_found, "key": key, "command": scripts[key], "present": True})
            else:
                hooks.append({"file": npm_found, "key": key, "present": False})
    else:
        for key in ["preinstall", "postinstall", "prepare"]:
            hooks.append({"file": "package.json", "key": key, "present": False})

    husky_content = git_show(ref, ".husky/pre-commit", git_root)
    if husky_content is not None:
        cmd = extract_husky_command(husky_content.decode())
        hooks.append({"file": ".husky/pre-commit", "key": "pre-commit", "command": cmd, "present": True})
    else:
        hooks.append({"file": ".husky/pre-commit", "key": "pre-commit", "present": False})

    # .vscode/tasks.json — presence/absence only (AC13 pattern: absence is information).
    vs_content = git_show(ref, ".vscode/tasks.json", git_root)
    hooks.append({"file": ".vscode/tasks.json", "key": "tasks", "present": vs_content is not None})
    return hooks


def extract_husky_command(content: str) -> str:
    for line in content.split("\n"):
        t = line.strip()
        if not t or t.startswith("#"):
            continue
        return t
    return ""


def run_audit_surface(ref: str, base: str, git_root: str) -> dict:
    """Perform the full audit and return a report dict."""
    report: dict = {"ref": ref, "hook_wiring": [], "instruction_files": [], "lifecycle_hooks": []}
    if base:
        report["base"] = base

    # 1. Hook wiring — 8 runtimes in canonical order.
    for runtime in CANONICAL_RUNTIMES:
        wiring_file = RUNTIME_WIRING_PATH[runtime]
        content = git_show(ref, wiring_file, git_root)
        if content is None:
            report["hook_wiring"].append({
                "runtime": runtime, "wiring_file": wiring_file, "present": False, "tuples": []
            })
            continue
        tuples = extract_tuples(runtime, content)
        # Compute digests, following symlink chains.
        for t in tuples:
            t["script_digest"] = (
                resolve_script_digest(ref, t["script_path"], git_root)
                if t["script_path"]
                else "unresolvable"
            )
        report["hook_wiring"].append({
            "runtime": runtime, "wiring_file": wiring_file, "present": True, "tuples": tuples
        })

    # 2. Instruction files (agent-config kind).
    for p in INSTRUCTION_FILE_PATHS:
        content = git_show(ref, p, git_root)
        report["instruction_files"].append({"path": p, "kind": "agent-config", "present": content is not None})

    # 3. Slash commands (.claude/commands/**/*.md).
    slash_files = [f for f in git_ls_tree(ref, ".claude/commands", git_root) if f.endswith(".md")]
    for f in slash_files:
        report["instruction_files"].append({"path": f, "kind": "slash-command", "present": True})

    # 4. Lifecycle hooks.
    report["lifecycle_hooks"] = audit_lifecycle_hooks(ref, git_root)

    return report


def tuple_count(report: dict) -> int:
    return sum(len(rr["tuples"]) for rr in report["hook_wiring"])


def format_text(report: dict) -> str:
    """Format the human-readable text report.
    Format is byte-identical to Go and Node.js implementations."""
    n = tuple_count(report)
    lines = [f"trackfw audit-surface: {n} hook tuple(s) at {report['ref']}", ""]

    for rr in report["hook_wiring"]:
        if not rr["present"]:
            lines.append(f"absent [{rr['runtime']}] {rr['wiring_file']}")
            continue
        if not rr["tuples"]:
            lines.append(f"no_hooks [{rr['runtime']}] {rr['wiring_file']}")
            continue
        for t in rr["tuples"]:
            matcher = t["matcher"] or "*"
            lines.append(f"hook [{rr['runtime']}] {rr['wiring_file']} {t['event']}/{matcher} {t['raw_command']} {t['script_digest']}")

    for f in report["instruction_files"]:
        if f["kind"] == "slash-command":
            if f["present"]:
                lines.append(f"slash-command {f['path']}")
        else:
            status = "present" if f["present"] else "absent"
            lines.append(f"instruction [{status}] {f['path']}")

    for lh in report["lifecycle_hooks"]:
        if lh["present"]:
            line = f"lifecycle [present] {lh['file']} {lh['key']}"
            if lh.get("command"):
                line += f" {lh['command']}"
            lines.append(line)
        else:
            lines.append(f"lifecycle [absent] {lh['file']} {lh['key']}")

    # If nothing after the blank line, drop the blank line.
    if len(lines) > 1 and lines[1] == "" and len(lines) == 2:
        lines.pop(1)
        return "\n".join(lines) + "\n"

    return "\n".join(lines) + "\n"


def run(args) -> int:
    ref = args.ref
    git_root = find_git_root()
    if not git_root:
        sys.stderr.write("audit-surface: not inside a git repository\n")
        sys.exit(1)

    # Validate ref resolves to a commit object in this repository (F3 fix: ^{commit} rejects
    # a 40-hex SHA from another repo that git would otherwise accept without ^{commit}).
    result = subprocess.run(
        ["git", "rev-parse", "--verify", ref + "^{commit}"],
        cwd=git_root,
        capture_output=True,
    )
    if result.returncode != 0:
        sys.stderr.write(f'audit-surface: ref "{ref}" does not resolve\n')
        sys.exit(1)

    base = getattr(args, "base", None) or ""
    report = run_audit_surface(ref, base, git_root)

    if getattr(args, "json", False):
        print(json.dumps(report, indent=2))
        return 0

    print(format_text(report), end="")
    return 0


def register(subparsers):
    """Register the 'audit-surface' subcommand."""
    parser = subparsers.add_parser(
        "audit-surface",
        help="Report the executable surface of a git ref without checking it out",
    )
    parser.add_argument("ref", help="git ref to audit (e.g. FETCH_HEAD, a commit hash, a branch name)")
    parser.add_argument("--json", action="store_true", help="Emit report as JSON instead of text")
    parser.add_argument("--base", default="", help="Base ref for Makefile/CI diff (optional)")
    parser.set_defaults(func=run)
    return parser
