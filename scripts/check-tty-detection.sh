#!/usr/bin/env bash
# Gate: deteccao de terminal interativo confiavel nos tres runtimes.
#
# sys.stdin.isatty() devolve True para NUL no Windows — NUL e um character device,
# e o Windows classifica character device como TTY. O resultado e o `init` do
# Python entrando no wizard de identidade em contexto nao interativo e morrendo
# com "EOF when reading a line". Go usa GetConsoleMode e Node usa o tipo de handle
# do libuv; so o Python precisava do ajuste.
#
# MEDE O DISCRIMINANTE, NAO O COMANDO. A primeira versao deste gate rodava
# `trackfw init </dev/null` e exigia exit 0 nos tres — e passava com e sem a
# correcao, porque o `init` sem --ai-tools nao chega a alcancar o wizard e com
# --ai-tools esbarra antes num os.fchmod que nao existe no Windows. Verde que nao
# significava nada. Agora pergunta direto ao predicado que a producao consulta.
#
# Ver o doc de pypi/trackfw/tty.py para o raciocinio completo.
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
fail=0

# ── Efeito: sob stdin nao interativo, nenhum runtime pode dizer "interativo" ──
#
# </dev/null e o NUL do Windows quando rodado sob Git Bash — exatamente o caso que
# mente para o isatty() do Python.
py_raw=$(PYTHONPATH="$ROOT/pypi" python3 -c 'import sys;print(sys.stdin.isatty())' </dev/null)
py_fix=$(PYTHONPATH="$ROOT/pypi" python3 -c 'from trackfw.tty import stdin_is_interactive;print(stdin_is_interactive())' </dev/null)
node_raw=$(node -e 'process.stdout.write(String(Boolean(process.stdin.isTTY)))' </dev/null)

if [ "$py_raw" != "True" ]; then
  # Nada a estreitar aqui: o isatty() ja responde a verdade. Nomeia a garantia que
  # deixou de ser exercitada em vez de passar em silencio. Deteccao pela CONDICAO,
  # nao por uname — num Windows que um dia conserte o isatty o gate volta sozinho.
  echo "tty detection: efeito NAO exercitado — neste sistema sys.stdin.isatty() ja"
  echo "  devolve $py_raw sob stdin nao interativo, entao nao ha mentira para estreitar."
else
  if [ "$py_fix" != "False" ]; then
    echo "tty detection: isatty() mente (True) e stdin_is_interactive() repetiu a mentira ($py_fix)"
    fail=1
  fi
fi

if [ "$node_raw" != "false" ]; then
  echo "tty detection: node considerou stdin interativo ($node_raw) sob </dev/null"
  fail=1
fi

# ── P2 vacuity guard: mesmo diretorio e filtro do scan estatico abaixo ──
#
# Se pypi/trackfw/ for movido/renomeado ou o filtro quebrar, o grep abaixo
# visitaria silenciosamente zero arquivos e nao encontraria nada para
# reportar — o gate passaria sem dizer se algo foi de fato verificado.
# Mirrors o padrao de scripts/check-python-writes-lf.sh ("P2 vacuity guard").
scanned=$(find "$ROOT/pypi/trackfw" -name '*.py' -print 2>/dev/null || true)
if [ -z "$scanned" ]; then
  echo "tty detection: scan visited zero .py files under pypi/trackfw/ — refusing to pass silently"
  fail=1
fi

# ── Estatico: nenhum isatty() cru fora do helper ──
raw=$(grep -rn 'isatty()' "$ROOT/pypi/trackfw" --include='*.py' \
      | grep -v 'trackfw/tty.py' \
      | grep -vE ':\s*#' | grep -v 'never blocks' || true)
if [ -n "$raw" ]; then
  echo "tty detection: python tem isatty() fora do helper:"
  printf '%s\n' "$raw" | sed 's/^/  /'
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo
  echo 'Use trackfw.tty.stdin_is_interactive() / stdout_is_interactive().'
  exit 1
fi
echo "Deteccao de TTY: o isatty() mente e o helper corrige; nenhum isatty() cru no Python."
