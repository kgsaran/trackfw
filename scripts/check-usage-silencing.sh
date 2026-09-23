#!/usr/bin/env bash
# check-usage-silencing.sh — o bloco `Usage:` sai em erro de USO e NAO sai em erro de RUNTIME.
#
# Fecha a lacuna declarada em docs/cli-parity.md, secao "Usage silencing":
#   "nenhum gate verifica que a saida de uso (usage/help) e suprimida em erros de runtime"
#
# O discriminante e a ORDEM do cobra: ParseFlags e ValidateArgs rodam ANTES dos hooks
# Persistent*, RunE depois. Entao erro de linha de comando mostra o usage (o usuario
# digitou errado) e erro de execucao nao (a linha estava certa; o que falhou foi o mundo).
#
# DUAS DIRECOES, sempre. Um gate que so afirmasse "runtime nao imprime usage" passaria
# num binario que nunca imprime usage — inclusive quando o usuario erra a linha.
#
# --self-test exercita o proprio gate contra dois binarios sinteticos, cada um quebrando
# uma direcao: um que SEMPRE imprime usage e um que NUNCA imprime. Os dois tem de reprovar.
set -u

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
GO_BIN="${GO_BIN:-$ROOT_DIR/bin/trackfw}"

FAIL=0
EXEC_RUNTIME=0
EXEC_USO=0

tem_usage() { case "$1" in *"Usage:"*) return 0 ;; *) return 1 ;; esac; }

# projeto descartavel: os comandos precisam de um trackfw.yaml para chegar ao RunE
novo_projeto() {
  local d; d=$(mktemp -d)
  ( cd "$d" \
    && git init -q -b main . >/dev/null 2>&1 \
    && git config user.email "gate@localhost" \
    && git config user.name "gate" \
    && "$BIN_SOB_TESTE" init --ai-tools claude >/dev/null 2>&1 ) || true
  printf '%s' "$d"
}

# caso_runtime <rotulo> <args...> — erro de execucao: rc != 0 e SEM usage
caso_runtime() {
  local rot="$1"; shift
  local d out rc
  d=$(novo_projeto)
  out=$( cd "$d" && "$BIN_SOB_TESTE" "$@" 2>&1 ); rc=$?
  rm -rf "$d"
  EXEC_RUNTIME=$((EXEC_RUNTIME+1))
  if [ "$rc" -eq 0 ]; then
    echo "FAIL [runtime/$rot]: esperava rc != 0, veio 0 — o cenario nao produz erro de runtime"
    FAIL=1; return
  fi
  # 🔴 rc=127 e "comando nao encontrado", nao erro de runtime do CLI. Sem esta guarda o
  # braco de runtime fica VERDE quando o binario nem roda — erro sem usage, tecnicamente.
  if [ "$rc" -eq 127 ]; then
    echo "FAIL [runtime/$rot]: rc=127 — o binario nao foi encontrado, o cenario nao rodou"
    FAIL=1; return
  fi
  if tem_usage "$out"; then
    echo "FAIL [runtime/$rot]: bloco Usage: impresso em erro de runtime (rc=$rc)"
    FAIL=1; return
  fi
  echo "OK   [runtime/$rot] rc=$rc, sem usage"
}

# caso_uso <rotulo> <args...> — erro de uso: rc != 0 e COM usage
caso_uso() {
  local rot="$1"; shift
  local d out rc
  d=$(novo_projeto)
  out=$( cd "$d" && "$BIN_SOB_TESTE" "$@" 2>&1 ); rc=$?
  rm -rf "$d"
  EXEC_USO=$((EXEC_USO+1))
  if [ "$rc" -eq 0 ]; then
    echo "FAIL [uso/$rot]: esperava rc != 0, veio 0 — o cenario nao produz erro de uso"
    FAIL=1; return
  fi
  if ! tem_usage "$out"; then
    echo "FAIL [uso/$rot]: erro de uso SEM bloco Usage: (rc=$rc) — o usuario nao sabe a forma certa"
    FAIL=1; return
  fi
  echo "OK   [uso/$rot] rc=$rc, com usage"
}

rodar_cenarios() {
  caso_runtime "roadmap-move-inexistente"  roadmap move inexistente wip
  caso_runtime "req-move-inexistente"      req move inexistente done
  caso_uso     "req-new-sem-titulo"        req new
  caso_uso     "roadmap-move-sem-args"     roadmap move
  caso_uso     "flag-desconhecida"         validate --nao-existe-esta-flag
}

# --------------------------------------------------------------------------
# self-test: dois binarios sinteticos, cada um quebrando UMA direcao
# --------------------------------------------------------------------------
if [ "${1:-}" = "--self-test" ]; then
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  falhas_esperadas=0

  cat > "$tmp/sempre-usage" <<'STUB'
#!/usr/bin/env bash
case "${1:-}" in init) exit 0 ;; esac
echo "Error: algo" >&2
echo "Usage:" >&2
exit 1
STUB
  cat > "$tmp/nunca-usage" <<'STUB'
#!/usr/bin/env bash
case "${1:-}" in init) exit 0 ;; esac
echo "Error: algo" >&2
exit 1
STUB
  chmod +x "$tmp/sempre-usage" "$tmp/nunca-usage"

  for braco in sempre-usage nunca-usage; do
    BIN_SOB_TESTE="$tmp/$braco"
    saida=$(FAIL=0; EXEC_RUNTIME=0; EXEC_USO=0; rodar_cenarios 2>&1; echo "FAIL=$FAIL")
    if printf '%s' "$saida" | grep -q "FAIL \["; then
      echo "OK   [self-test/$braco] o gate REPROVA como deveria"
    else
      echo "FAIL [self-test/$braco] o gate passou num binario que quebra esta direcao"
      falhas_esperadas=1
    fi
  done

  if [ "$falhas_esperadas" -ne 0 ]; then
    echo "check-usage-silencing: self-test REPROVOU"
    exit 1
  fi
  echo "check-usage-silencing: self-test 2/2 OK"
  exit 0
fi

# --------------------------------------------------------------------------
# execucao normal
# --------------------------------------------------------------------------
if [ ! -x "$GO_BIN" ]; then
  echo "check-usage-silencing: binario ausente em $GO_BIN — rode 'make build' antes." >&2
  exit 1
fi

# 🔴 ABSOLUTO, sempre. O Makefile passa GO_BIN=$(BUILD_DIR)/$(BINARY), que e RELATIVO, e
# cada cenario roda dentro de um projeto descartavel (cd). Com caminho relativo o binario
# nao existe la, todo cenario sai rc=127, e o braco de runtime passaria dizendo
# "erro sem usage" — verde pelo motivo errado. Medido no CI do PR #413.
case "$GO_BIN" in
  /*|[A-Za-z]:*) BIN_SOB_TESTE="$GO_BIN" ;;
  *)             BIN_SOB_TESTE="$(CDPATH= cd -- "$(dirname -- "$GO_BIN")" && pwd)/$(basename -- "$GO_BIN")" ;;
esac

# Guarda de execucao: o binario tem de RODAR. Sem isto, um caminho valido mas quebrado
# (arquitetura errada, dependencia ausente) reproduziria o mesmo verde falso.
if ! "$BIN_SOB_TESTE" --version >/dev/null 2>&1; then
  echo "check-usage-silencing: '$BIN_SOB_TESTE --version' nao executou — binario invalido." >&2
  exit 1
fi

rodar_cenarios

# Guarda de vacuidade nas DUAS classes: zero cenario de qualquer lado e gate que
# nao mede, e sairia verde dizendo nada.
if [ "$EXEC_RUNTIME" -eq 0 ] || [ "$EXEC_USO" -eq 0 ]; then
  echo "check-usage-silencing: VACUO — runtime=$EXEC_RUNTIME uso=$EXEC_USO (as duas classes precisam de cenario)" >&2
  exit 1
fi

if [ "$FAIL" -ne 0 ]; then
  echo "check-usage-silencing: REPROVOU ($EXEC_RUNTIME de runtime, $EXEC_USO de uso)"
  exit 1
fi
echo "check-usage-silencing: OK — $EXEC_RUNTIME cenario(s) de runtime sem usage, $EXEC_USO de uso com usage"
