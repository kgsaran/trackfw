# p20-eol-count.py — conta bytes CRLF/LF em assets e goldens relevantes para o ML-W3a.
#
# Pergunta 20 do windows-probe.yml (ML-W3a, REQ-2026-09-03).
#
# Saida grep-friendly:
#   PROBE w3 eol <arquivo> crlf=<n> lf=<n>
#
# Uso: python p20-eol-count.py
# Deve ser executado na raiz do workspace (GITHUB_WORKSPACE).

import sys

FILES = [
    "internal/integrations/assets/agents/architect.md",
    "internal/integrations/assets/agents/backend.md",
    "internal/integrations/assets/agents/frontend.md",
    "internal/integrations/assets/agents/opencode.md",
    "internal/integrations/testdata/architect.subagent.golden.md",
    "internal/integrations/testdata/backend.agent-directory.golden.md",
]

for f in FILES:
    try:
        data = open(f, "rb").read()
        crlf = data.count(b"\r\n")
        lf_only = data.count(b"\n") - crlf
        print("PROBE w3 eol {} crlf={} lf={}".format(f.split("/")[-1], crlf, lf_only))
    except Exception as e:
        print("PROBE w3 eol {} ERROR={}".format(f.split("/")[-1], e))

sys.exit(0)
