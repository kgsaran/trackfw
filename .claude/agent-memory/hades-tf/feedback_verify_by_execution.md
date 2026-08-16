---
name: feedback-verify-by-execution
description: for trackfw serve/CLI security reviews, spin up the process and hit it (curl, LAN IP) instead of only reading code — reading alone missed a real active vuln
metadata:
  type: feedback
---

Rule: when reviewing `trackfw serve` (or any long-running/network-facing command) across the 3
CLIs, actually start it and probe it (`curl 127.0.0.1:<port>`, `curl <LAN IP>:<port>`,
`lsof -iTCP:<port> -sTCP:LISTEN`) rather than concluding from reading the bind call alone.

**Why:** during the 2026-08-16 ML-2A security review, reading `internal/serve/serve.go` and
`pypi/trackfw/commands/serve.py` alone would have suggested "looks like a normal HTTP server" —
the advisor pushed to actually verify, and starting the process + curling the machine's LAN IP
confirmed both Go and Python bind to all interfaces (`0.0.0.0`), not just localhost like Node —
an active, unauthenticated network exposure of governance docs. This was the single highest-value
finding of that review and would have been missed by code-reading + grep alone. See
[[project-serve-binds-all-interfaces]].

Same pattern paid off for a second question that session: whether the new global error handler
(`installGlobalHandlers()` calling `process.exit(1)`) kills the `serve` process differently than
before. Reading suggested a possible regression; running a baseline copy (handler disabled) against
a fixed copy with an identical synthetic throw proved the crash-the-whole-process behavior is
Node's own default (pre-existing), not something the fix introduced — the fix only changed stderr
verbosity. Execution settled it either way, in less time than continued reading/reasoning would have.

**How to apply:** default to isolated-copy execution (never in the tracked repo) whenever a claim
is "does X leak/regress/crash" and a live process can answer it directly — especially for `serve`,
CLI entrypoints, and error-handling paths. Reading code is fine for ruling things in/out structurally
(e.g. confirming a catalog is `//go:embed`d and therefore not runtime-influenceable), but for
runtime behavior (bind address, crash-on-throw, response body content) run it.
