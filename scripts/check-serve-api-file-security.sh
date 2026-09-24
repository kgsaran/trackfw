#!/usr/bin/env bash
# check-serve-api-file-security.sh — prova que `trackfw serve` usa defesas de
# path traversal/symlink escape no handler de arquivo estático.
#
# ML-3A (v8 — um binário, muitos canais): Node.js e Python removidos.
# Este gate agora verifica apenas o binário Go:
#   - AC7: serve.go usa embed.FS (nenhum ReadFile de disco)
#   - AC6 Go falsificação: sem filePathAllowed o teste SymlinkEscape FALHA
#   - AC6 Go corrigido: TestFileHandler_SymlinkEscape PASSA
#   - AC4 contra-braço: TestFileHandler_SymlinkInsideRoot PASSA
#
# Cenários Python e Node.js removidos por ML-3A (npm/src/ e pypi/trackfw/ deletados).
#
# Nota: python3 ainda é usado para a falsificação via overlay.json — não como
# runtime do CLI, apenas como ferramenta de manipulação de texto.
#
# Convenções (herdadas do gate original):
#   - python3 sempre, nunca python.

set -euo pipefail
export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd -P)"
GO_API_FILE="$ROOT_DIR/internal/serve/api_file.go"

PASS=0
FAIL=0

ok() {
  PASS=$((PASS + 1))
  printf '  ok  %s\n' "$1"
}

fail() {
  FAIL=$((FAIL + 1))
  printf '  FAIL %s\n' "$1" >&2
  if [[ -n "${2:-}" ]]; then
    printf '       %s\n' "$2" >&2
  fi
}

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-serve-api-security.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Pré-requisitos
# ---------------------------------------------------------------------------
echo ""
echo "── Pré-requisitos ────────────────────────────────────────────────────────"

if [[ ! -f "$GO_API_FILE" ]]; then
  printf '  ERRO: %s não encontrado\n' "$GO_API_FILE" >&2
  exit 1
fi
printf '  --  internal/serve/api_file.go encontrado\n'

# ---------------------------------------------------------------------------
# AC7: Varredura — lista de sítios
# ---------------------------------------------------------------------------
echo ""
echo "── AC7: Varredura — lista de sítios ──────────────────────────────────────"
echo "  Resultado da varredura (v8 single-runtime):"
echo "    Go — 1 sítio: internal/serve/api_file.go (?path=)"
echo "       Go serve assets via embed.FS (nenhum ReadFile de disco em serve.go)"
echo "  (npm/src/ e pypi/trackfw/ removidos por ML-3A)"

# Go: embed.FS — sem ReadFile de disco em serve.go
if grep -q "embed\.FS\|//go:embed" "$ROOT_DIR/internal/serve/serve.go" 2>/dev/null; then
  ok "Go serve.go usa embed.FS (sem ReadFile de disco para assets estáticos)"
elif ! grep -q "ReadFile\|os\.Open" "$ROOT_DIR/internal/serve/serve.go" 2>/dev/null; then
  ok "Go serve.go não faz ReadFile de disco (assets estáticos não expostos)"
else
  fail "Go serve.go pode ter ReadFile de disco inesperado — revisar AC7" \
       "$(grep -n 'ReadFile\|os\.Open' "$ROOT_DIR/internal/serve/serve.go" 2>/dev/null | head -5)"
fi

# ---------------------------------------------------------------------------
# AC6 falsificação: Go dinâmico (go test -overlay)
# ---------------------------------------------------------------------------
echo ""
echo "── AC6 falsificação: Go dinâmico (go test -overlay) ─────────────────────"

VULN_GO="$WORK/api_file_vuln.go"
OVERLAY_JSON="$WORK/overlay.json"
# O overlay.json é escrito pelo PRÓPRIO Python, com json.dumps sobre os caminhos
# recebidos por argv (mesma forma de .github/workflows/quality.yml:1172, que roda
# em windows-latest). Duas razões, nenhuma delas condicional por plataforma:
#   1. grafia — no Git Bash as variáveis do shell são caminhos POSIX-MSYS
#      (/c/Users/...); o MSYS converte argv de processo nativo, mas NUNCA o
#      conteúdo de um arquivo. Um overlay com chave POSIX é ignorado EM SILÊNCIO
#      pelo go.exe (não é erro), o teste roda contra o fonte correto e o braço de
#      falsificação conclui "passou na versão vulnerável";
#   2. escape — interpolar caminho cru dentro de JSON via printf é injeção em
#      formato estruturado, independente de plataforma: qualquer '\\' ou '"' no
#      caminho quebra o JSON. `cygpath -m` corrigiria só a grafia (e escapa do
#      problema por acidente, por emitir '/'); `cygpath -w` emitiria C:\Users\...
#      e produziria JSON inválido. json.dumps fecha os dois por construção.
# O assert fecha o segundo canal de vacuidade das mesmas linhas: se o needle
# mudar, o "vulnerável" seria byte a byte igual ao correto e o teste passaria —
# mesmo sintoma, outro mecanismo.
python3 -c "
import json, os, sys
src, vuln, overlay = (os.path.abspath(a) for a in sys.argv[1:4])
content = open(src, encoding='utf-8').read()
needle = '\tif !filePathAllowed(realAbsPath, physicalAllowedDirs) {'
count = content.count(needle)
assert count >= 1, 'AC6: padrao de injecao nao encontrado em %s (count=%d)' % (src, count)
open(vuln, 'w', encoding='utf-8').write(
    content.replace(needle, '\tif false && !filePathAllowed(realAbsPath, physicalAllowedDirs) {')
)
open(overlay, 'w', encoding='utf-8').write(json.dumps({'Replace': {src: vuln}}))
" "$GO_API_FILE" "$VULN_GO" "$OVERLAY_JSON"

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

CORR_GO_OUT=$(cd "$ROOT_DIR" && go test ./internal/serve/... \
    -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s 2>&1 || true)
if echo "$CORR_GO_OUT" | grep -q "^ok"; then
  ok "AC6 Go corrigido: TestFileHandler_SymlinkEscape PASSA na versão correta"
else
  fail "AC6 Go corrigido: TestFileHandler_SymlinkEscape falhou na versão correta" \
       "$CORR_GO_OUT"
fi

# ---------------------------------------------------------------------------
# AC3: symlink escape — TestFileHandler_SymlinkEscape (ataque → 403)
# ---------------------------------------------------------------------------
echo ""
echo "── AC3: symlink escape — ataque → 403 ───────────────────────────────────"
echo "   Go:"
if (cd "$ROOT_DIR" && go test ./internal/serve/... -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s 2>&1) | grep -q "^ok"; then
  ok "Go TestFileHandler_SymlinkEscape — 403 sem corpo"
else
  fail "Go TestFileHandler_SymlinkEscape falhou"
fi

# ---------------------------------------------------------------------------
# AC4 (contra-braço): symlink legítimo → 200
# ---------------------------------------------------------------------------
echo ""
echo "── AC4 (contra-braço): symlink legítimo → 200 ───────────────────────────"
echo "   Go:"
if (cd "$ROOT_DIR" && go test ./internal/serve/... -run "TestFileHandler_SymlinkInsideRoot" -count=1 -timeout 30s 2>&1) | grep -q "^ok"; then
  ok "Go TestFileHandler_SymlinkInsideRoot — 200 com conteúdo"
else
  fail "Go TestFileHandler_SymlinkInsideRoot falhou"
fi

# ---------------------------------------------------------------------------
# Resultado final
# ---------------------------------------------------------------------------
echo ""
echo "── Resultado final ───────────────────────────────────────────────────────"
echo "  $PASS ok, $FAIL falhou"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi

EXPECTED_PASS=5
if [ "$PASS" -ne "$EXPECTED_PASS" ]; then
  printf '  ERRO: esperados %d ok, obtidos %d — cenário silenciado\n' "$EXPECTED_PASS" "$PASS"
  exit 1
fi
exit 0
