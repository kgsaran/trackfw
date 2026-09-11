---
name: project-python-ipaddress-zone-id-metachar-windows-injection
description: Python ipaddress.ip_address accepts zone IDs with shell metacharacters (&, ;, |); list2cmdline does NOT quote them without spaces; cmd.exe splits at & — new command injection in Python Windows serve path
metadata:
  type: project
---

Found 2026-09-11, REQ-2026-09-01-serve-interpola-host, BLOQUEIA verdict.

**Mechanism:** Python 3.9+ `ipaddress.ip_address()` accepts zone IDs (the part after `%` in scoped
IPv6 addresses like `fe80::1%eth0`) with arbitrary characters including `&`, `;`, `|`, and space.
Python's `_is_valid_host()` passes such hosts as valid. The URL becomes
`http://[fe80::1%eth0&calc.exe&echo]:4080`. `subprocess.list2cmdline()` only quotes arguments with
space/tab/quotes — so `&` without preceding space is NOT quoted. `cmd.exe` interprets unquoted `&`
as command separator → `calc.exe` executes.

**Contrasts with Node.js**: `net.isIPv6('fe80::1%eth0&id')` → false (Node rejects zone IDs with
metacharacters — only alphanumeric zone IDs accepted).
**Contrasts with Go**: `net.ParseIP('fe80::1%eth0')` → nil (Go rejects ALL scoped addresses).

**Safe variant**: `fe80::1%eth0 & calc.exe` (space before `&`) → `list2cmdline` QUOTES the whole
arg → `&` inside quotes is literal in cmd.exe → safe. Attacker avoids spaces trivially.

**Fix (Python only)**: In `_is_valid_host()`, after `ipaddress.ip_address()` accepts a scoped address:
```python
if '%' in host:
    zone_id = host.split('%', 1)[1]
    if not re.match(r'^[a-zA-Z0-9._-]+$', zone_id):
        return False
```

**Why:** REQ fix correctly replaced `shell=True` with `Popen(argv, shell=False)`. But the validator
became the only gate, and it did not validate zone IDs. The injection path changed (from explicit
shell to argv→list2cmdline→cmd.exe metachar), but the injection still exists.

**How to apply:** When reviewing any IPv6 validator in Python that uses `ipaddress`, check zone ID
handling explicitly. When reviewing Windows process invocation with URLs, always check `list2cmdline`
output for unquoted metacharacters.

Vault note: `vault/notes/python-ipaddress-zone-id-metachar-windows-injection-2026-09-11.md`
