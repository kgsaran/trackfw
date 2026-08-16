"""unknown_command.py — canonical cross-CLI "unknown command" message and the
shared suggestion algorithm.

ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-
terceiro.md (D3): argument dispatch to an external `trackfw-<name>` binary was
removed together with the plugin subsystem. An unrecognized top-level command
now always produces the message below instead — in all three CLIs.

The plain (no-transposition) Levenshtein distance and the suggestion-picking
rule here are reimplemented IDENTICALLY in Go (internal/commands/root.go,
suggestCommand/levenshteinDistance) and Node.js (npm/src/lib/unknown-command.js,
suggestCommand/levenshteinDistance) — deliberately not delegated to argparse's
own "invalid choice" formatting, which has neither a distance-based suggestion
nor the exit code (1) this contract requires. Parity is enforced by
scripts/check-unknown-command-parity.sh.
"""


def levenshtein_distance(a: str, b: str) -> int:
    la, lb = len(a), len(b)
    d = [[0] * (lb + 1) for _ in range(la + 1)]
    for i in range(la + 1):
        d[i][0] = i
    for j in range(lb + 1):
        d[0][j] = j
    for i in range(1, la + 1):
        for j in range(1, lb + 1):
            cost = 0 if a[i - 1] == b[j - 1] else 1
            d[i][j] = min(
                d[i - 1][j] + 1,  # deletion
                d[i][j - 1] + 1,  # insertion
                d[i - 1][j - 1] + cost,  # substitution
            )
    return d[la][lb]


def suggest_command(typed: str, candidates):
    """A candidate is eligible when its case-insensitive Levenshtein distance
    to `typed` is <= 2, OR it is a case-insensitive prefix match. Among
    eligible candidates the winner is the one with the lowest distance,
    alphabetical tie-break — deterministic and single, matching Go/Node.js.
    """
    lower_typed = typed.lower()
    best_dist = -1
    best = None
    for candidate in candidates:
        lower_c = candidate.lower()
        dist = levenshtein_distance(lower_typed, lower_c)
        prefix_match = bool(lower_typed) and lower_c.startswith(lower_typed)
        if dist > 2 and not prefix_match:
            continue
        if best_dist == -1 or dist < best_dist or (dist == best_dist and candidate < best):
            best_dist = dist
            best = candidate
    return best


def format_unknown_command_error(typed: str, candidates, cmd_path: str) -> str:
    """Canonical message, byte-identical across the three CLIs modulo the
    typed name and the suggestion:

        Error: unknown command "x" for "trackfw"
        Did you mean "validate"?
        Run 'trackfw --help' for usage.
    """
    lines = [f'Error: unknown command "{typed}" for "{cmd_path}"']
    suggestion = suggest_command(typed, candidates)
    if suggestion:
        lines.append(f'Did you mean "{suggestion}"?')
    lines.append(f"Run '{cmd_path} --help' for usage.")
    return "\n".join(lines)
