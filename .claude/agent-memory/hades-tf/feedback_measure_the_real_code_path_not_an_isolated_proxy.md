---
name: feedback-measure-the-real-code-path-not-an-isolated-proxy
description: when comparing 3-CLI parity of a code path, run the actual read+decode+parse pipeline through each real binary, not just the shared primitive (e.g. the JSON parser) in isolation
metadata:
  type: feedback
---

Testing `encoding/json`/`JSON.parse`/`json.loads` directly with identical raw bytes is not the same as
testing the validator's actual behavior, even when the diff under review is entirely inside the parse
step. There can be a layer BEFORE the shared primitive (file read, then text decode) that differs between
the 3 runtimes and is invisible if you only feed bytes to the parser standalone.

**Why:** during the ML-1A barrier for ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel
(2026-09-06), I claimed "paridade total" for a corpus (BOM, JSONC, UTF-16, etc.) after running raw bytes
against `encoding/json`/`JSON.parse`/`json.loads` in isolation — all 3 agreed. The advisor pointed out the
real validator code doesn't reach the parser that way: Go reads raw `[]byte`, Node decodes lossily
(`fs.readFileSync(path, 'utf8')`, invalid bytes -> U+FFFD, never throws), and Python decodes STRICTLY
(`open(path, "r", encoding="utf-8")`), raising `UnicodeDecodeError` — a `ValueError` subclass, not an
`OSError` — before `json.loads` is ever called. My proxy script couldn't see this because it skipped the
decode step entirely. Re-running the 2 relevant fixtures against the 3 real binaries found Python crashing
uncaught where Go/Node both handled the case cleanly. See [[project_guard_hook_read_ioerror_fail_open_node_python]].

**How to apply:** for cross-CLI parity claims specifically, always execute the full pipeline (read → decode
→ parse → business logic) through the actual 3 binaries (`go build` output, `node <entry>`,
`PYTHONPATH=... python3 -m <pkg>`) with the same fixture file on disk, not just the narrowest shared
primitive in a throwaway script. A proxy test of one function is fine for a first pass / narrowing search,
but the parity verdict itself must come from the real path — this is the same principle as
[[feedback_verify_by_execution]] and [[feedback_medir_decode_e_encode_separadamente]], specialized further:
even "execute it" isn't enough if what you execute is a stand-in for the code rather than the code.
