---
name: project-guard-hook-read-ioerror-fail-open-node-python
description: credential/git-branch guard hook config read errors (permission denied, dir-not-file) still silently pass validate in Node+Python even after ML-1A closed the JSON-parse fail-open
metadata:
  type: project
---

`validateGuardHookResolvable`/`validateGuardGlobalHookResolvable` (Go: `internal/validator/validator_credential_guard.go`,
`validator_git_branch_guard.go`; Node: `npm/src/validator/index.js`; Python: `pypi/trackfw/validator.py`) read one
of 12 known guard-config files (6 project-scope + 6 global-scope) and used to `continue` silently on
BOTH "invalid JSON" and "can't read the file for any reason". ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-
config-ilegivel-deixa-de-ser-silencio / ML-1A (2026-09-06) closed only the invalid-JSON half, in all 3 CLIs,
with confirmed byte-identical parity (tested live against the 3 native JSON parsers with identical raw
bytes: BOM, JSONC, trailing comma, empty, whitespace-only, UTF-16, single-quoted — all 3 reject identically).

**Not fixed**: the read-I/O-error half (EACCES/permission-denied, EISDIR/directory-instead-of-file, ELOOP).
Reproduced live with `chmod 000` and with a directory in place of the file on `.claude/settings.json`:
- Go: `os.ReadFile` error that isn't `os.IsNotExist` returns a hard Go `error` (pre-existing to this ML,
  not new) — this **crashes the entire `trackfw validate` invocation** (exit 1, empty stdout, no other
  rule reported either).
- Node: `npm/src/validator/index.js` catch block is `if (e.code === 'ENOENT') continue; continue` — a dead
  conditional, EVERY read error (not just ENOENT) falls through to the same silent `continue`. Live test:
  `chmod 000` or dir-not-file on `.claude/settings.json` → `node npm/bin/trackfw validate --json` exits 0,
  zero mention of the file.
- Python: `pypi/trackfw/validator.py` has `except OSError: continue` — `OSError` is the parent class of
  `PermissionError`/`IsADirectoryError`/`FileNotFoundError` in Python, so it collapses all of them into one
  silent skip. Live test: same chmod-000/dir-not-file cases → exit 0, silent pass.

Net effect: today, in the released Node and Python CLIs, a credential-guard or git-branch-guard hook file
that is completely unreadable (bad permissions, replaced by a directory, etc.) makes `trackfw validate`
report clean success — exactly the "control reports health about wiring it never inspected" defect the
whole REQ (REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-...) exists to close, still alive for
this sub-case. [[project_provenance_key_filepath_rel_read_mismatch]]-style pattern: writer/one-CLI already
correct (Go crashes loud), the other 2 runtimes independently reimplement the same read+parse logic and
never converged on the narrower exception.

**Fix direction** (not yet a REQ/roadmap as of 2026-09-06): Node should distinguish `e.code === 'ENOENT'`
(silent, legitimate) from anything else (violation, mirroring Go's *intent* but as a reported violation
instead of a process crash); Python should catch `FileNotFoundError` specifically instead of blanket
`OSError`, with a violation branch for any other `OSError`. Go's own behavior (hard crash of the whole
`validate` run on one unreadable guard file) is also worth softening to a scoped violation message instead
of aborting the entire command — that part predates ML-1A and was out of its stated scope.

**Second, related gap found on re-measurement (advisor caught that my first pass measured a proxy, not the
real path — see [[feedback_measure_the_real_code_path_not_an_isolated_proxy]]):** below the read step is a
DECODE step, and it diverges too. Go's `json.Unmarshal` operates on raw `[]byte` (never crashes on bad
UTF-8, either fails the parse or silently coerces). Node's `fs.readFileSync(path, 'utf8')` decodes lossily
(invalid bytes -> U+FFFD, never throws). Python's `open(path, "r", encoding="utf-8")` + `.read()` decodes
STRICTLY and raises `UnicodeDecodeError` — which is a `ValueError`/`UnicodeError` subclass, NOT an
`OSError`, so it is not caught by the `except OSError: continue` guarding that read, nor by anything else
in the loop. Live-tested two fixtures against the real 3 binaries: (a) whole file saved as UTF-16 (classic
Windows Notepad "Save as Unicode" footgun) — Go/Node both emit a clean "not valid JSON" violation (exit 1,
structured), Python crashes with a raw uncaught-exception message on stderr (exit 1, no JSON output); (b) a
single invalid UTF-8 byte inside an otherwise well-formed JSON string — Go/Node silently accept it (coerced),
Python crashes the same way. Same family as the read-I/O gap above (an except/catch clause narrower than
the exceptions actually reachable at that point), one layer lower (decode, not read). Fix must touch the
decode step specifically, not just widen the `OSError` branch — e.g. read as bytes and decode with
`errors="replace"` to match Node's lossy behavior, or explicitly catch `UnicodeDecodeError` and emit a
violation, matching whichever behavior the architect picks as canonical.

Full write-up: `~/.trackfw/rascunhos/2026-09-06-parecer-hades-fail-open-config-ilegivel.md` (hades-tf
barrier verdict: APROVA COM RESSALVAS on ML-1A itself — this finding is a named residual, not a block,
since it's pre-existing and out of the ML's declared JSON-parse-only scope).
