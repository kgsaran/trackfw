#!/usr/bin/env bash
# Gate: nenhuma escrita de texto em pypi/trackfw/ sem newline explicito.
#
# open(path,"w") do Python usa newline=None, que traduz \n para os.linesep — CRLF
# no Windows. Go e Node escrevem bytes direto. Sem isto os tres runtimes produzem
# artefato diferente byte a byte, e os scripts/*.sh gerados saem com CR no shebang,
# que falha em POSIX com "bad interpreter".
#
# Estatico de proposito: a CI do upstream e Linux e nunca ve o defeito, entao todo
# arquivo novo que vier de la chega sem newline. Este gate pega no merge.
#
# Ver docs/cli-parity.md.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

# P2 vacuity guard: derive, with `find`, the file list visited by the same
# criteria as the os.walk() below (pypi/trackfw/**/*.py, skipping
# __pycache__), against the SAME relative path and cwd the walk below uses
# (no ROOT_DIR/cd here on purpose — using an absolute anchor would let this
# guard pass from a cwd where the walk itself sees nothing, defeating the
# guard). If pypi/trackfw/ were moved, renamed, or a filter broke, the walk
# below would silently visit zero files and OFFENDERS would stay empty — the
# gate would pass, but say nothing about whether any file was actually
# checked. Mirrors the pattern in scripts/check-static-assets.sh ("P2
# vacuity guard").
SCANNED=$(find pypi/trackfw -name __pycache__ -prune -o -name '*.py' -print)
if [[ -z "$SCANNED" ]]; then
  echo "check-python-writes-lf: scan visited zero .py files under pypi/trackfw/ — refusing to pass silently" >&2
  exit 1
fi

OFFENDERS=$(python3 - <<'PY'
import io, os, re
NAMES = ('open(', '.write_text(')
def calls(s, name):
    out, i = [], 0
    while True:
        i = s.find(name, i)
        if i < 0: return out
        j, depth, instr = i + len(name), 1, None
        while j < len(s) and depth:
            c = s[j]
            if instr:
                if c == chr(92): j += 2; continue
                if c == instr: instr = None
            elif c in '"\'': instr = c
            elif c == '(': depth += 1
            elif c == ')': depth -= 1
            j += 1
        out.append((i, j)); i = j
bad = []
for root, dirs, fs in os.walk('pypi/trackfw'):
    if '__pycache__' in root: continue
    for f in sorted(fs):
        if not f.endswith('.py'): continue
        p = os.path.join(root, f)
        s = io.open(p, encoding='utf-8').read()
        for name in NAMES:
            for a, b in calls(s, name):
                call = s[a:b]
                if 'newline' in call: continue
                if name == 'open(':
                    if re.search(r'''["'][rwax]\+?b\+?["']''', call): continue
                    if not re.search(r'''["'][wa]\+?["']''', call): continue
                line = s.count(chr(10), 0, a) + 1
                bad.append('%s:%d' % (p.replace(chr(92), '/'), line))
print(chr(10).join(bad))
PY
)

if [ -n "$OFFENDERS" ]; then
  echo "escrita de texto sem newline explicito em pypi/trackfw/:"
  echo "$OFFENDERS" | sed 's/^/  /'
  echo
  echo 'Use newline="\n". Ver docs/cli-parity.md.'
  exit 1
fi
echo "Escrita em LF: nenhuma chamada sem newline explicito."
