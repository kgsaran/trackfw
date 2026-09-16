#!/usr/bin/env bash
# Wave 0 gate — a superficie de slug de artefato esta fechada.
# Falha se aparecer implementacao nova, ou sumir uma das declaradas.
#
# Ate 2026-09-16 eram 9 implementacoes: 1 em Go, 4 em Node (npm/src/generators)
# e 4 em Python (pypi/trackfw/generators). A v8.0.0 do upstream (#365) removeu as
# duas reimplementacoes; sobra a do Go. O gate continua: uma segunda
# implementacao de slug em internal/generators e exatamente o que ele pega.
# Ver REQ-2026-09-16-gates-so-nossos-depois-da-v8.
set -euo pipefail

expected=$(printf '%s\n' \
  'internal/generators/adr.go:toSlug' | sort)

# `|| true`: sem ele, a implementacao SUMIDA faz o grep sair 1, o pipefail mata o
# script dentro da atribuicao, e o gate reprova sem dizer o que sumiu.
actual=$(
  { grep -rlE '^func toSlug' internal/generators --include='*.go' || true; } \
    | sed 's/$/:toSlug/' | sort -u)

if [ "$actual" != "$expected" ]; then
  echo "Wave 0: inventario de slug mudou."
  diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") || true
  exit 1
fi
echo "Wave 0: inventario de slug fechado — 1 implementacao declarada (Go; Node e Python removidos na v8)."
