#!/usr/bin/env bash
# Gate: os tres runtimes resolvem a home para $HOME quando ela esta definida.
#
# os.homedir() (Node) e os.path.expanduser (Python) leem %USERPROFILE% no Windows e
# ignoram $HOME; os.UserHomeDir() (Go) faz o mesmo. Sem isto, todo teste que isola a
# home com HOME=<tempdir> continua lendo e escrevendo a home real do desenvolvedor.
#
# Cobre os TRES runtimes de proposito, nao so os dois corrigidos por ultimo: a
# falsificacao que ninguem vigia e o Go regredir, porque ele "ja esta certo".
#
# Ver REQ-2026-08-29-node-e-python-ignoram-home-no-windows.
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

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAKE="$(mktemp -d)"
trap 'rm -rf "$FAKE"' EXIT
mkdir -p "$FAKE/home"

fail=0

# ── Por efeito: com HOME apontado para um tempdir, os tres resolvem para ele ──
#
# A comparacao usa o nome unico do tempdir, nao o caminho inteiro: no Windows o
# mktemp do Git Bash devolve /tmp/tmp.XXXX enquanto os runtimes reportam
# C:/Users/.../Temp/tmp.XXXX. O mesmo diretorio, duas grafias — comparar o caminho
# completo reprovaria com o comportamento correto.
expected="$FAKE/home"
token="$(basename "$FAKE")"
for rt in go node python; do
  case "$rt" in
    go)     out=$(cd "$FAKE" && HOME="$expected" "$ROOT/bin/trackfw" adr list --scope global 2>&1 || true) ;;
    node)   out=$(cd "$FAKE" && HOME="$expected" node "$ROOT/npm/bin/trackfw" adr list --scope global 2>&1 || true) ;;
    python) out=$(cd "$FAKE" && HOME="$expected" PYTHONPATH="$ROOT/pypi" python3 -m trackfw adr list --scope global 2>&1 || true) ;;
  esac
  if ! printf '%s' "$out" | grep -qF "$token"; then
    echo "homedir parity: $rt nao resolveu para \$HOME"
    echo "  esperado conter o tempdir: $token"
    echo "  saida:                     $out"
    fail=1
  fi
done

# ── P2 vacuity guard: mesmos diretorios e filtros do scan estatico abaixo ──
#
# Se npm/src, pypi/trackfw ou internal+cmd forem movidos/renomeados, ou um
# filtro quebrar, o grep abaixo visitaria silenciosamente zero arquivos e nao
# encontraria nada para reportar — o gate passaria sem dizer se algo foi de
# fato verificado. Ancorado em $ROOT, o mesmo prefixo usado pelo grep logo
# abaixo. Mirrors o padrao de scripts/check-python-writes-lf.sh ("P2 vacuity
# guard").
scanned_node=$(find "$ROOT/npm/src" -name '*.js' -print 2>/dev/null || true)
scanned_py=$(find "$ROOT/pypi/trackfw" -name '*.py' -print 2>/dev/null || true)
scanned_go=$(find "$ROOT/internal" "$ROOT/cmd" -name '*.go' -print 2>/dev/null || true)
for pair in "npm/src:$scanned_node" "pypi/trackfw:$scanned_py" "internal+cmd:$scanned_go"; do
  dir="${pair%%:*}"; files="${pair#*:}"
  if [ -z "$files" ]; then
    echo "homedir parity: scan visited zero files under $dir — refusing to pass silently"
    fail=1
  fi
done

# ── Estatico: nenhuma forma crua fora do helper de cada runtime ──
raw_node=$(grep -rn 'os\.homedir()' "$ROOT/npm/src" --include='*.js' | grep -v 'src/homedir.js' || true)
raw_py=$(grep -rn 'os\.path\.expanduser\|Path\.home()' "$ROOT/pypi/trackfw" --include='*.py' | grep -v 'trackfw/homedir.py' || true)
raw_go=$(grep -rn 'os\.UserHomeDir()' "$ROOT/internal" "$ROOT/cmd" --include='*.go' \
         | grep -v '_test.go' | grep -v 'internal/homedir/' | grep -v '//' || true)

for pair in "node:$raw_node" "python:$raw_py" "go:$raw_go"; do
  rt="${pair%%:*}"; hits="${pair#*:}"
  if [ -n "$hits" ]; then
    echo "homedir parity: $rt tem resolucao de home fora do helper:"
    printf '%s\n' "$hits" | sed 's/^/  /'
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  echo
  echo "Use o helper de cada runtime: internal/homedir.Dir(), npm/src/homedir.js, trackfw.homedir."
  exit 1
fi
echo "Paridade de home: os 3 runtimes honram \$HOME, e nenhum resolve fora do helper."
