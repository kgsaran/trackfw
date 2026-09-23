#!/usr/bin/env bash
# lib-crlf-normalize.sh — ponto único de normalização de CRLF no stdout de python3.
#
# REQ: REQ-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md
# ML:  ML-1A
#
# No Windows, python3 traduz \n → \r\n no stdout (modo texto). Bash que consome
# essa saída recebe valores com \r invisível. A função strip_cr() remove esse \r
# terminal de cada linha.
#
# O que esta lib FAZ:
#   strip_cr() é um filtro de pipeline que opera sobre stdout capturado — remove
#   \r terminal (artefato do modo texto) usando sed com CR literal (ANSI-C quote
#   $'\r') para portabilidade entre BSD sed (macOS) e GNU sed (Linux).
#
# O que esta lib NÃO faz:
#   Não toca conteúdo de arquivo. A função write_fixture_crlf() em
#   check-roadmap-barrier-contract.sh escreve fixtures com CRLF intencional
#   para o issue #216 — essa função usa python3 em modo binário, escreve em
#   arquivo (não em stdout capturado por bash), e está fora da população desta lib.
#
# Uso:
#   . "$SCRIPTS_DIR/lib-crlf-normalize.sh"   # ou . "$ROOT_DIR/scripts/lib-crlf-normalize.sh"
#
#   # stdout do Python como filtro (captura para arquivo):
#   python3 -c "..." | strip_cr > output.txt
#   $PY_BIN "$GEN" ... | strip_cr > manifest.txt
#
#   # captura em variável:
#   VAR=$(python3 -c "..." | strip_cr)
#   VAR=$(python3 -c "..." 2>/dev/null | strip_cr || echo "FALLBACK")
#
#   # pipeline com heredoc:
#   VAR=$(python3 - <<'PYEOF' | strip_cr
#   import sys
#   print("result")
#   PYEOF
#   )

strip_cr() { sed $'s/\r$//'; }
