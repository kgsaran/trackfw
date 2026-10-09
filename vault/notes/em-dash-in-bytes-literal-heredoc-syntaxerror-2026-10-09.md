# em dash in bytes literal inside heredoc → Python SyntaxError

**Date:** 2026-10-09  
**Component:** scripts/check-channels-content.sh (arm8-fetch.py stub, --self-test)  
**REQ:** REQ-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md

## Root cause

`arm8-fetch.py` was written via `cat > ... << 'PYEOF'` heredoc and contained:

```python
z.writestr(f'{name}/__init__.py', b'# injected .py — arm8 bad-content stub\n')
```

The `—` is an em dash (U+2014), a non-ASCII character. Python 3 **bytes literals
(`b'...'`) may only contain ASCII characters** (ord ≤ 127); non-ASCII raises
`SyntaxError: bytes can only contain ASCII literal characters` at parse time,
before the script runs.

## Symptom

The self-test arm 8 (bad wheel content → expect exit 1, no retry) was failing
with `clock=30` instead of `clock=0`. The fetch stub silently raised SyntaxError
on every call, so `_do_fetch` returned non-zero every time, `all_dl_ok=0`,
`attempt_ok=0`, and the retry loop kept firing until the fake deadline (30 s)
was exhausted.

The retry loop did not "leak" into content inspection — the content inspection
was never reached. The fix was trivial: replace `—` with `--` in the comment.

## Why it was hard to spot

1. The subprocess captures `2>&1`, so the SyntaxError was swallowed into
   `arm8_out` but NOT printed by the test (the failure branch only printed
   a one-line summary).
2. Running the subprocess manually (from outside the self-test) worked because
   those manual tests used ASCII-only stubs written directly, not from the
   heredoc.
3. The `[[ -n "$url" ]] || continue` fix (the previous debugging session's
   hypothesis) was correct and necessary for a different reason — but Arm 8
   was still failing for a completely different cause.

## Fix

`scripts/check-channels-content.sh` line ~719: change

```python
z.writestr(f'{name}/__init__.py', b'# injected .py — arm8 bad-content stub\n')
```
to
```python
z.writestr(f'{name}/__init__.py', b'# injected .py -- arm8 bad-content stub\n')
```

## Rule of thumb

**Never put non-ASCII characters inside Python bytes literals (`b'...'`) in
heredoc-generated stub scripts.** Non-ASCII in a normal string (`'...'`) is fine
with `# -*- coding: utf-8 -*-` or Python 3 default UTF-8 source — but `b'...'`
is unconditionally ASCII-only regardless of source encoding.

Also added: `| strip_cr` to the `wheel_urls` python3 capture in
`_fetch_pypi_wheels_with_retry` (required by `check-crlf-normalize-capture`
gate, pre-existing missing normalization).
