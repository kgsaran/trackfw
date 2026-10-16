# p20-hex-analyze.py — le go test -json do stdin, extrai primeira divergencia hex.
#
# Pergunta 20 do windows-probe.yml (ML-W3a, REQ-2026-09-03).
#
# Saida grep-friendly:
#   PROBE w3 hex_diverge test=<nome> byte=<n> got_len=<n> want_len=<n>
#   PROBE w3 hex_got  <40-byte hex>
#   PROBE w3 hex_want <40-byte hex>
#   PROBE w3 hex_got_repr  <repr>
#   PROBE w3 hex_want_repr <repr>
#   PROBE w3 hex_diverge_crlf byte=<n> lf_len=<n> crlf_len=<n>
#   PROBE w3 hex_lf    <hex>
#   PROBE w3 hex_crlf  <hex>
#   PROBE w3 manifest_key_form=<posix_slash|native_json_escaped|unknown>
#   PROBE w3 manifest_raw_hex <hex>
#   P20_FAIL_MSG: <msg>
#
# Uso: go test -count=1 -json -run '^TestFoo$' ./pkg/ | python p20-hex-analyze.py <testname>

import sys
import json
import re

test_name = sys.argv[1] if len(sys.argv) > 1 else "unknown"

all_output = []
for line in sys.stdin:
    try:
        obj = json.loads(line.rstrip("\n"))
        if obj.get("Action") == "output":
            all_output.append(obj["Output"])
    except Exception:
        pass

full = "".join(all_output)

# First failure message (first assert line)
fail_m = re.search(r"[a-z_]+_test\.go:\d+: (.{0,400})", full, re.DOTALL)
if fail_m:
    msg = fail_m.group(1)[:350].replace("\n", " | ").replace("\r", "[CR]")
    print("P20_FAIL_MSG: " + msg)

# --- got --- / --- want --- (frozen golden tests)
m = re.search(r"--- got ---\r?\n(.*?)--- want ---\r?\n(.*?)(?:\r?\nFAIL|\Z)", full, re.DOTALL)
if m:
    got_b = m.group(1).encode("utf-8", "replace")
    want_b = m.group(2).encode("utf-8", "replace")
    for i in range(min(len(got_b), len(want_b))):
        if got_b[i] != want_b[i]:
            s = max(0, i - 2)
            eg = min(len(got_b), i + 40)
            ew = min(len(want_b), i + 40)
            print("PROBE w3 hex_diverge test={} byte={} got_len={} want_len={}".format(
                test_name, i, len(got_b), len(want_b)))
            print("PROBE w3 hex_got  " + got_b[s:eg].hex())
            print("PROBE w3 hex_want " + want_b[s:ew].hex())
            print("PROBE w3 hex_got_repr  " + repr(got_b[s:eg]))
            print("PROBE w3 hex_want_repr " + repr(want_b[s:ew]))
            break
    else:
        if len(got_b) != len(want_b):
            print("PROBE w3 hex_no_early_diverge test={} got_len={} want_len={}".format(
                test_name, len(got_b), len(want_b)))

# LF: / CRLF: block (CRLF injection tests)
m2 = re.search(r"LF:\r?\n(.*?)\r?\nCRLF:\r?\n(.*?)(?:\r?\n---|\Z)", full, re.DOTALL)
if m2:
    lf_b = m2.group(1).encode("utf-8", "replace")
    crlf_b = m2.group(2).encode("utf-8", "replace")
    for i in range(min(len(lf_b), len(crlf_b))):
        if lf_b[i] != crlf_b[i]:
            s = max(0, i - 2)
            el = min(len(lf_b), i + 40)
            ec = min(len(crlf_b), i + 40)
            print("PROBE w3 hex_diverge_crlf test={} byte={} lf_len={} crlf_len={}".format(
                test_name, i, len(lf_b), len(crlf_b)))
            print("PROBE w3 hex_lf    " + lf_b[s:el].hex())
            print("PROBE w3 hex_crlf  " + crlf_b[s:ec].hex())
            print("PROBE w3 hex_lf_repr   " + repr(lf_b[s:el]))
            print("PROBE w3 hex_crlf_repr " + repr(crlf_b[s:ec]))
            break
    else:
        if len(lf_b) != len(crlf_b):
            print("PROBE w3 hex_crlf_no_early_diverge test={} lf_len={} crlf_len={}".format(
                test_name, len(lf_b), len(crlf_b)))

# Manifest key form (TestUpdateMigratesKnownCodexAndPreservesUnknown)
man_m = re.search(r"unexpected Codex ownership manifest:\r?\n\s*(.*?)(?:\r?\n---|\Z)", full, re.DOTALL)
if man_m:
    raw = man_m.group(1)[:1200]
    raw_bytes = raw.encode("utf-8", "replace")
    # Detect key form: JSON-escaped backslash appears as \\ in the raw string
    if b"\\\\" in raw_bytes:
        form = "native_json_escaped"
    elif b":/" in raw_bytes:
        form = "posix_slash"
    else:
        form = "unknown"
    print("PROBE w3 manifest_key_form=" + form)
    print("PROBE w3 manifest_raw_hex " + raw_bytes[:200].hex())

sys.exit(0)
