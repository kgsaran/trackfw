---
name: project-roadmap-title-newline-barrier-rce-unfixed
description: roadmap new "<title with embedded newline>" can forge a whole ## Wave N section with its own gate block, and trackfw barrier --wave N executes it — live shell-command execution, not yet fixed.
metadata:
  type: project
  status: open
---

Found 2026-08-23 on ML-3A of `ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md`.

`internal/generators/roadmap.go:150` (`NewRoadmapFromContent`) interpolates the `roadmap new` title
into `# Roadmap: %s` with no newline stripping. A title containing
`"\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n\`\`\`bash\n<cmd>\n\`\`\`\n"` plants a forged
wave section, complete with its own gate block, ahead of the real template-generated section in the
same file. `internal/commands/barrier.go` (`parseWaves`/`parseGates`) resolves the **first**
occurrence of a `## Wave N` heading in the file and executes whatever gate command sits inside it via
`sh -c` — reproduced with `barrier --wave 0` (Go, Node — `/tmp/*_PWNED` files materialized) and
independently reproduced with `--wave 1` against the same mechanism, proving it predates the Wave-0
REQ (gates feature dates to ADR-2026-07-26-principios-de-design-de-gates-verificaveis).

**Why:** the Wave-0 REQ's AC13 ("gate literal do template não interpolado") only guarded the
template's own gate string against title/REQ interpolation — a different, narrower guarantee than
"the title can't inject an entirely separate Markdown section with its own gate." The two are easy
to conflate when reviewing; testing only `$(...)`/backtick vectors (shell-string evaluation) will
say AC13 is clean while this structural-injection path remains wide open.

**How to apply:** do not treat this as closed by AC13. If asked to review `roadmap new`,
`barrier`, or anything touching title/REQ-content interpolation into Markdown, retest this
specific vector (embedded newline forging a `## Wave N` + `**Gates da wave:**` block) — it was not
fixed as of 2026-08-23, only reported (`docs/seguranca/2026-08-23-barreira-da-wave-0-no-harness.md`
§2-bis, vault note
`vault/notes/roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23.md`). If a
later REQ claims to have fixed it, verify by re-running the exact reproduction (title with embedded
`\n\n## Wave N ...` + gate fence, then `barrier --wave N --json`, check `commands[]` for the injected
payload and whether the file it targets actually materializes) before trusting the claim.
