'use strict'

// unknown-command.js — canonical cross-CLI "unknown command" message and the
// shared suggestion algorithm.
//
// ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-
// terceiro.md (D3): argument dispatch to an external `trackfw-<name>` binary was
// removed together with the plugin subsystem. An unrecognized top-level command
// now always produces the message below instead — in all three CLIs.
//
// The plain (no-transposition) Levenshtein distance and the suggestion-picking
// rule here are reimplemented IDENTICALLY in Go (internal/commands/root.go,
// suggestCommand/levenshteinDistance) and Python (pypi/trackfw/cli.py,
// _suggest_command/_levenshtein_distance) — deliberately NOT delegated to each
// framework's own suggestion helper (cobra's Command.SuggestionsFor, commander's
// suggestSimilar), which use different distance functions/thresholds and would
// make the three CLIs disagree on whether/what to suggest for the same typo.
// Parity is enforced by scripts/check-unknown-command-parity.sh.

function levenshteinDistance(a, b) {
  const la = a.length
  const lb = b.length
  const d = []
  for (let i = 0; i <= la; i++) {
    d.push(new Array(lb + 1).fill(0))
    d[i][0] = i
  }
  for (let j = 0; j <= lb; j++) {
    d[0][j] = j
  }
  for (let i = 1; i <= la; i++) {
    for (let j = 1; j <= lb; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1
      d[i][j] = Math.min(
        d[i - 1][j] + 1, // deletion
        d[i][j - 1] + 1, // insertion
        d[i - 1][j - 1] + cost, // substitution
      )
    }
  }
  return d[la][lb]
}

// suggestCommand — a candidate is eligible when its case-insensitive
// Levenshtein distance to `typed` is <= 2, OR it is a case-insensitive prefix
// match (candidate starts with the typed text). Among eligible candidates the
// winner is the one with the lowest distance, alphabetical tie-break, so the
// pick is deterministic and single — matching Go/Python exactly.
function suggestCommand(typed, candidates) {
  const lowerTyped = typed.toLowerCase()
  let bestDist = -1
  let best = ''
  for (const candidate of candidates) {
    const lowerC = candidate.toLowerCase()
    const dist = levenshteinDistance(lowerTyped, lowerC)
    const prefixMatch = lowerTyped.length > 0 && lowerC.startsWith(lowerTyped)
    if (dist > 2 && !prefixMatch) continue
    if (bestDist === -1 || dist < bestDist || (dist === bestDist && candidate < best)) {
      bestDist = dist
      best = candidate
    }
  }
  return bestDist === -1 ? null : best
}

// formatUnknownCommandError — canonical message, byte-identical across the
// three CLIs modulo the typed name and the suggestion (present only when
// suggestCommand finds an eligible candidate):
//   Error: unknown command "x" for "trackfw"
//   Did you mean "validate"?
//   Run 'trackfw --help' for usage.
function formatUnknownCommandError(typed, candidates, cmdPath) {
  const lines = [`Error: unknown command "${typed}" for "${cmdPath}"`]
  const suggestion = suggestCommand(typed, candidates)
  if (suggestion) {
    lines.push(`Did you mean "${suggestion}"?`)
  }
  lines.push(`Run '${cmdPath} --help' for usage.`)
  return lines.join('\n')
}

module.exports = { levenshteinDistance, suggestCommand, formatUnknownCommandError }
