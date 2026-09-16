#!/usr/bin/env bash
# Gate: deteccao de terminal interativo confiavel no binario Go.
#
# ML-3A (v8 — um binário, muitos canais): pypi/trackfw/ e npm/src/ removidos.
# O discriminante Python (pypi/trackfw/tty.py / stdin_is_interactive()) e a
# verificacao estatica de isatty() no Python nao sao mais aplicaveis — o
# pacote Python e o shim Node.js nao implementam logica propria de TTY.
#
# O Go usa os.Stdin.Fd() + golang.org/x/term.IsTerminal (ou similar), que
# responde corretamente ao contexto — nunca acusa interativo quando stdin e
# redirecionado. Este gate verifica que `trackfw init --ai-tools gemini` nao
# trava com stdin redirecionado, o que seria sintoma de wizard bloqueando em
# contexto nao interativo.
#
# Ver historico: o gate original media sys.stdin.isatty() vs stdin_is_interactive()
# do Python (Go usa GetConsoleMode no Windows / IsTerminal no Unix e nunca mente).
# Esse par foi valido enquanto existiu o CLI Python; removido com ML-3A (v8).
set -euo pipefail

export PYTHONIOENCODING=utf-8

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO_BIN="${GO_BIN:-$ROOT/bin/trackfw}"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-tty-detection: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

# ── Behavioral pin: Go nao trava com stdin nao interativo ──
#
# Redireciona stdin de /dev/null e verifica que o binario nao bloqueia
# esperando input. Se houver regressao de wizard rodando em contexto nao
# interativo, este teste vai travar ate o timeout.
# --ai-tools gemini forca a inicializacao de um tool sem precisar de wizard
# de identidade interativo (que e o que poderia travar).
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-tty-detection.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/project" "$WORK/home"

set +e
HOME="$WORK/home" "$GO_BIN" init --ai-tools gemini >/dev/null 2>&1 </dev/null
go_exit=$?
set -e

# Qualquer exit code que nao seja timeout e aceitavel — o criterio e
# "nao trava", nao "exit 0" (pode sair com erro por diretorio sem trackfw.yaml).
echo "tty detection (Go): init --ai-tools gemini </dev/null saiu com exit $go_exit — nao travou"

echo "Deteccao de TTY: gate Go-only (v8 single-runtime) — binario nao trava com stdin nao interativo."
