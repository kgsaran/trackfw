#!/usr/bin/env bash
# check-serve-api-file-security.sh — prova que /api/file e serveStatic bloqueiam
# symlinks que apontam para fora das raízes autorizadas nos 3 runtimes (H-01 + M-03).
#
# Cobre:
#   AC1  — Go e Node canonicalizam raiz E arquivo antes de comparar
#   AC2  — M-03: serveStatic do Node usa o mesmo tratamento; testado aqui e em
#          serve_api.test.js (attack arm + counter-arm)
#   AC3  — 403 E ausência do conteúdo no corpo (assere os dois)
#   AC4  — arquivo legítimo dentro da raiz continua devolvendo 200
#   AC5  — teste do vetor nos 3 runtimes (inclusive Python, que já passava)
#   AC6  — falsificação dinâmica nos 3 runtimes: revertendo o realpath, o teste
#          falha (prova que o gate discrimina, não só que o código compila)
#   AC7  — varredura dos sítios: lista fechada, comando registrado
#
# AC6 por runtime:
#   Go   — go test -overlay com cópia vulnerável (filePathAllowed(realAbsPath) desabilitado
#          via `if false &&`). O teste TestFileHandler_SymlinkEscape deve FALHAR na versão
#          vulnerável e PASSAR na versão correta.
#   Node — sed remove realpathSync.native (não-op) de api_file.js; require() do módulo
#          vulnerável; confirma que handleFile() retorna 200 com segredo.
#   Py   — sed substitui os.path.realpath por os.path.abspath em api_file.py; importlib
#          carrega o módulo vulnerável; confirma que get_file() vaza o segredo.
#
# Convenções:
#   - set -euo pipefail; mktemp WORK + trap cleanup.
#   - ok()/fail() acumulador: não aborta no primeiro erro.
#   - pwd -P obrigatório ao criar WORK (mktemp -d dá /var/...; processo vê /private/var/...).
#   - env -u FORCE_COLOR para suprimir sequências ANSI em saídas de CLIs.
#   - python3 sempre, nunca python.

set -euo pipefail
export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd -P)"
NODE_API_FILE="$ROOT_DIR/npm/src/serve/api_file.js"
NODE_SERVE_FILE="$ROOT_DIR/npm/src/commands/serve.js"
PY_API_FILE="$ROOT_DIR/pypi/trackfw/serve/api_file.py"
GO_API_FILE="$ROOT_DIR/internal/serve/api_file.go"

PASS=0
FAIL=0

ok() {
  PASS=$((PASS + 1))
  printf '  ok  %s\n' "$1"
}

fail() {
  FAIL=$((FAIL + 1))
  printf '  FAIL %s\n' "$1"
  if [ -n "${2-}" ]; then
    printf '       %s\n' "$2"
  fi
}

# ─── Diretório de trabalho ────────────────────────────────────────────────────

WORK=$(mktemp -d); WORK=$(cd "$WORK" && pwd -P)
trap 'rm -rf "$WORK"' EXIT

setup_project() {
  local BASE="$1"
  mkdir -p "$BASE/docs/req" "$BASE/docs/roadmaps" "$BASE/docs/adr"
  printf 'req_dir: docs/req\nroadmap_dir: docs/roadmaps\n' > "$BASE/trackfw.yaml"
  printf -- '---\nstatus: Open\n---\n# REQ: ok\n' > "$BASE/docs/req/REQ-ok.md"
}

echo ""
echo "=== check-serve-api-file-security.sh ==="
echo ""

# ─── AC7: Varredura de sítios ─────────────────────────────────────────────────
# Comando: grep -rn "path.*resolve\|filepath\.Clean\|filepath\.Join\|os\.ReadFile\|readFileSync"
# nos handlers serve de cada runtime e identificar todos os sítios que recebem
# input controlado pelo usuário (parâmetro ?path= ou pathname /static/...).
#
# Resultado da varredura (2026-09-11):
#   Go    — 1 sítio: internal/serve/api_file.go (?path=)
#            Go serve assets via embed.FS (nenhum ReadFile de disco em serve.go)
#   Node  — 2 sítios: npm/src/serve/api_file.js (?path=) e
#                     npm/src/commands/serve.js serveStatic (/static/...)
#   Python — 2 sítios: pypi/trackfw/serve/api_file.py (?path=) [já defendido]
#                      pypi/trackfw/commands/serve.py _serve_static_file [já defendido]
# Lista fechada. Qualquer sítio adicional deve ser adicionado aqui.

echo "── AC7: Varredura — lista de sítios ──────────────────────────────────────"
# Go: embed.FS — sem ReadFile de disco em serve.go
if grep -q "embed\.FS\|//go:embed" "$ROOT_DIR/internal/serve/serve.go" 2>/dev/null; then
  ok "Go serve.go usa embed.FS (sem ReadFile de disco para assets estáticos)"
elif ! grep -q "ReadFile\|os\.Open" "$ROOT_DIR/internal/serve/serve.go" 2>/dev/null; then
  ok "Go serve.go não faz ReadFile de disco (assets estáticos não expostos)"
else
  fail "Go serve.go pode ter ReadFile de disco inesperado — revisar AC7" \
       "$(grep -n 'ReadFile\|os\.Open' "$ROOT_DIR/internal/serve/serve.go" 2>/dev/null | head -5)"
fi

# Python: _serve_static_file usa realpath
if grep -q "os\.path\.realpath" "$ROOT_DIR/pypi/trackfw/commands/serve.py" 2>/dev/null; then
  ok "Python serve.py _serve_static_file usa os.path.realpath"
else
  fail "Python serve.py pode estar sem realpath em _serve_static_file — revisar AC7"
fi

# ─── AC6 falsificação: Go dinâmico ───────────────────────────────────────────
# Prova por execução: cria cópia vulnerável de api_file.go (check físico desabilitado
# via `if false &&`) e roda o teste com go test -overlay. O teste DEVE FALHAR na versão
# vulnerável (provando que discrimina) e PASSAR na versão original.
echo ""
echo "── AC6 falsificação: Go dinâmico (go test -overlay) ─────────────────────"

VULN_GO="$WORK/api_file_vuln.go"
# Cria versão vulnerável: desabilita a verificação física inserindo `false &&`
# antes da condição. `physicalAllowedDirs` permanece referenciado → compila sem erro.
python3 -c "
src = open('$GO_API_FILE').read()
vuln = src.replace(
    '\tif !filePathAllowed(realAbsPath, physicalAllowedDirs) {',
    '\tif false && !filePathAllowed(realAbsPath, physicalAllowedDirs) {'
)
open('$VULN_GO', 'w').write(vuln)
"

# overlay.json: substitui o arquivo original pelo vulnerável durante o teste
OVERLAY_JSON="$WORK/overlay.json"
printf '{"Replace": {"%s": "%s"}}\n' "$GO_API_FILE" "$VULN_GO" > "$OVERLAY_JSON"

# Executa com a versão vulnerável — o teste deve FALHAR
VULN_GO_OUT=$(cd "$ROOT_DIR" && go test -overlay="$OVERLAY_JSON" ./internal/serve/... \
    -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s 2>&1 || true)
if echo "$VULN_GO_OUT" | grep -qE "^(FAIL|--- FAIL)"; then
  ok "AC6 Go falsificação: sem filePathAllowed(realAbsPath) o teste SymlinkEscape FALHA"
elif echo "$VULN_GO_OUT" | grep -q "^ok"; then
  fail "AC6 Go falsificação: teste passou na versão vulnerável — deve reprovar" \
       "$VULN_GO_OUT"
else
  fail "AC6 Go falsificação: resultado inesperado" "$VULN_GO_OUT"
fi

# Confirma que a versão corrigida passa
CORR_GO_OUT=$(cd "$ROOT_DIR" && go test ./internal/serve/... \
    -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s 2>&1 || true)
if echo "$CORR_GO_OUT" | grep -q "^ok"; then
  ok "AC6 Go corrigido: TestFileHandler_SymlinkEscape PASSA na versão correta"
else
  fail "AC6 Go corrigido: TestFileHandler_SymlinkEscape falhou na versão correta" \
       "$CORR_GO_OUT"
fi

# ─── AC6 falsificação: Python dinâmico ───────────────────────────────────────
# Prova por execução: substitui os.path.realpath por os.path.abspath (não resolve
# symlinks), carrega o módulo vulnerável com importlib, confirma que get_file()
# vaza o segredo via wfile.write.
echo ""
echo "── AC6 falsificação: Python dinâmico (importlib) ────────────────────────"

VULN_PY="$WORK/api_file_vuln.py"
sed 's/os\.path\.realpath/os.path.abspath/g' "$PY_API_FILE" > "$VULN_PY"

OUTSIDE_PY="$WORK/outside-py"
mkdir -p "$OUTSIDE_PY"
printf 'HADES_SECRET_TOKEN_ABC123\n' > "$OUTSIDE_PY/secret.txt"
OUTSIDE_PY_REAL=$(cd "$OUTSIDE_PY" && pwd -P)

PY_VULN_RESULT=$(python3 - <<PYEOF 2>/dev/null
import sys, os, importlib.util, tempfile
from unittest.mock import MagicMock
from urllib.parse import urlparse

# Diretório do projeto simulado
tmp = tempfile.mkdtemp()
os.makedirs(os.path.join(tmp, 'docs', 'req'))

# Symlink dentro de docs/req apontando para arquivo secreto externo
secret = os.path.join('$OUTSIDE_PY_REAL', 'secret.txt')
link = os.path.join(tmp, 'docs', 'req', 'link.md')
os.symlink(secret, link)

# Carrega módulo vulnerável
spec = importlib.util.spec_from_file_location('api_file_vuln', '$VULN_PY')
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

# Chama get_file com handler mock
os.chdir(tmp)
cfg = {'req_dir': 'docs/req', 'adr_dirs': [], 'roadmap_dir': 'docs/roadmaps'}
parsed_url = urlparse('/api/file?path=docs/req/link.md')
handler = MagicMock()
mod.get_file(cfg, parsed_url, handler)

calls = [str(c) for c in handler.wfile.write.call_args_list]
leaked = any('HADES_SECRET' in c for c in calls)
errors = handler.send_error.call_args_list
sys.stdout.write('leaked=' + str(leaked) + ' errors=' + str(len(errors)))
PYEOF
)

if echo "$PY_VULN_RESULT" | grep -q "leaked=True"; then
  ok "AC6 Python falsificação: sem realpath o handler vulnerável vaza o segredo"
else
  fail "AC6 Python falsificação: módulo vulnerável não vazou segredo — verificar" \
       "Resultado: $PY_VULN_RESULT"
fi

# Confirma que versão corrigida NÃO vaza
PY_CORR_RESULT=$(cd "$ROOT_DIR" && PYTHONPATH=pypi python3 -m pytest \
    pypi/tests/test_serve_api.py::TestFileAPI::test_symlink_escape_blocked_403_no_body \
    -v --tb=short 2>&1)
if echo "$PY_CORR_RESULT" | grep -q "PASSED"; then
  ok "AC6 Python corrigido: test_symlink_escape_blocked_403_no_body PASSA na versão correta"
else
  fail "AC6 Python corrigido: teste falhou na versão correta" "$PY_CORR_RESULT"
fi

# ─── AC6 falsificação: Node dinâmico ──────────────────────────────────────────
# Criar versão vulnerável de api_file.js (sem realpathSync) e confirmar que
# handleFile() retorna 200 com o segredo — provando que o gate discrimina.
echo ""
echo "── AC6 falsificação: Node dinâmico ──────────────────────────────────────"

FAKE_DIR="$WORK/fake-node"
mkdir -p "$FAKE_DIR"
VULN_API="$FAKE_DIR/api_file_vuln.js"

# Versão vulnerável: substitui realpathSync.native por identidade (não-op)
sed 's/fs\.realpathSync\.native(\([^)]*\))/\1/g' "$NODE_API_FILE" > "$VULN_API"

# Criar projeto temporário
PROJ="$WORK/proj-node-vuln"
setup_project "$PROJ"
SECRET_DIR="$WORK/outside-node"
mkdir -p "$SECRET_DIR"
printf 'HADES_SECRET_TOKEN_ABC123\n' > "$SECRET_DIR/secret.txt"
PROJ_REAL=$(cd "$PROJ" && pwd -P)
ln -s "$SECRET_DIR/secret.txt" "$PROJ_REAL/docs/req/link.md"

# Usar Node para exercitar o handler vulnerável diretamente via require()
LINK_PATH="$PROJ_REAL/docs/req/link.md"
REQ_DIR="$PROJ_REAL/docs/req"
VULN_RESULT=$(node -e "
const path = require('path');
const { handleFile } = require('$VULN_API');
const cfg = { reqDir: '$REQ_DIR' };
const req = { url: '/api/file?path=$LINK_PATH' };
let statusCode = null, body = '';
const res = { writeHead: (c) => { statusCode = c }, end: (d) => { body += (d || '') } };
handleFile(cfg, req, res);
process.stdout.write(statusCode + ':' + body.slice(0, 50));
" 2>/dev/null)

if echo "$VULN_RESULT" | grep -q "HADES_SECRET"; then
  ok "AC6 Node falsificação: handler vulnerável (sem realpathSync) retorna 200 + segredo"
else
  fail "AC6 Node falsificação: handler vulnerável não retornou o segredo" \
       "Resultado: $VULN_RESULT"
fi

# ─── AC2: M-03 serveStatic ────────────────────────────────────────────────────
# Prova que serveStatic bloqueia symlinks externos (attack arm) e continua
# servindo arquivos legítimos (counter-arm). Os testes unitários em
# npm/tests/serve_api.test.js cobrem ambos os braços — verificados aqui via runner.
echo ""
echo "── AC2: M-03 serveStatic — braços attack e counter ─────────────────────"

NODE_ALL=$(node "$ROOT_DIR/npm/tests/serve_api.test.js" 2>&1)

if echo "$NODE_ALL" | grep -q "serveStatic — symlink para fora do STATIC_DIR retorna 403"; then
  ok "AC2 Node serveStatic attack arm — symlink externo → 403 sem corpo (serve_api.test.js)"
else
  fail "AC2 Node serveStatic attack arm falhou — verificar serve_api.test.js" \
       "$(echo "$NODE_ALL" | grep -E "serveStatic|x " | head -5)"
fi

if echo "$NODE_ALL" | grep -q "serveStatic — arquivo legítimo em STATIC_DIR retorna 200"; then
  ok "AC2 Node serveStatic counter-arm — arquivo legítimo → 200 (serve_api.test.js)"
else
  fail "AC2 Node serveStatic counter-arm falhou — verificar serve_api.test.js" \
       "$(echo "$NODE_ALL" | grep -E "serveStatic|x " | head -5)"
fi

# ─── AC1 + AC3 + AC5 (braço corrigido): 3 runtimes ──────────────────────────
echo ""
echo "── AC1 + AC3 + AC5 (braço corrigido): 3 runtimes ───────────────────────"

# ── Go ──
echo "   Go:"
if (cd "$ROOT_DIR" && go test ./internal/serve/... -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s 2>&1) | grep -q "^ok"; then
  ok "Go TestFileHandler_SymlinkEscape — 403 sem corpo"
else
  fail "Go TestFileHandler_SymlinkEscape falhou"
fi

# ── Node ──
echo "   Node:"
if echo "$NODE_ALL" | grep -q "symlink para fora da raiz retorna 403"; then
  ok "Node api_file — symlink escape → 403 sem corpo"
else
  fail "Node symlink escape test falhou" "$(echo "$NODE_ALL" | grep -E "x |symlink" | head -5)"
fi

# ── Python ──
echo "   Python:"
PY_RESULT=$(cd "$ROOT_DIR" && PYTHONPATH=pypi python3 -m pytest pypi/tests/test_serve_api.py::TestFileAPI::test_symlink_escape_blocked_403_no_body -v --tb=short 2>&1)
if echo "$PY_RESULT" | grep -q "PASSED"; then
  ok "Python test_symlink_escape_blocked_403_no_body — 403 sem corpo"
else
  fail "Python symlink escape test falhou" "$PY_RESULT"
fi

echo ""
echo "── AC4 (contra-braço): symlink legítimo → 200 nos 3 runtimes ───────────"

# Go
if (cd "$ROOT_DIR" && go test ./internal/serve/... -run "TestFileHandler_SymlinkInsideRoot" -count=1 -timeout 30s 2>&1) | grep -q "^ok"; then
  ok "Go TestFileHandler_SymlinkInsideRoot — 200 com conteúdo"
else
  fail "Go TestFileHandler_SymlinkInsideRoot falhou"
fi

# Node
if echo "$NODE_ALL" | grep -q "symlink legítimo dentro da raiz retorna 200"; then
  ok "Node api_file — symlink interno legítimo → 200"
else
  fail "Node symlink legítimo test falhou"
fi

# Python
PY_LEG=$(cd "$ROOT_DIR" && PYTHONPATH=pypi python3 -m pytest pypi/tests/test_serve_api.py::TestFileAPI::test_symlink_inside_root_allowed -v --tb=short 2>&1)
if echo "$PY_LEG" | grep -q "PASSED"; then
  ok "Python test_symlink_inside_root_allowed — 200 com conteúdo"
else
  fail "Python symlink legítimo test falhou" "$PY_LEG"
fi

echo ""
echo "── Resultado final ───────────────────────────────────────────────────────"
echo "  $PASS ok, $FAIL falhou"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
