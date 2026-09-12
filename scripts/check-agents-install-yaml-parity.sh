#!/usr/bin/env bash
# check-agents-install-yaml-parity.sh — verifica que os 3 CLIs (Go, Node.js, Python)
# produzem o MESMO trackfw.yaml depois de `agents install --scope project` em 4 cenários:
#
#   S1: arquivo sem chave agents:   → bloco ANEXADO AO FIM; comentários/ordem preservados
#   S2: bloco agents: no MEIO       → item entra no bloco; bloco NÃO muda de lugar
#   S3: agents: [flow, inline]      → arquivo BYTE-IDÊNTICO à entrada; aviso no stderr
#   S4: roadmap_namespacing: flat   → chave agents: NÃO é criada (arquivo byte-idêntico)
#
# Além disso verifica idempotência: segunda instalação não duplica (S1, S2).
#
# Por que este gate existe: as três divergências da Wave 2 (posição do bloco, flow
# inline, mensagem) só apareceram porque o arquiteto rodou os três binários lado a lado
# à mão. O check-artifact-parity.sh compara artefatos GERADOS; o trackfw.yaml MODIFICADO
# não estava na lista dele. Este gate fecha esse buraco.
#
# Linked to: make parity-rest (ML-2D, ROADMAP-2026-09-11-by-agent-req-new-e-roadmap-new-
# agents-install-nao-registra-e-escreve-sempre-no-primeiro.md)
#
# Invocação:
#   GO_BIN=bin/trackfw scripts/check-agents-install-yaml-parity.sh
#   GO_BIN=bin/trackfw scripts/check-agents-install-yaml-parity.sh --self-test
#
# --self-test: cria um "mutant pypi" sem a guarda de flow-inline e prova que o gate
# reprova NOMEANDO "python"; em seguida prova que com os três alinhados o gate PASSA.
# Propósito: falsificação embutida — se alguém remover a guarda de inline-flow no
# Python, este arm prova que o gate detectaria, não que "o self-test passou".
set -euo pipefail

export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb
unset FORCE_COLOR 2>/dev/null || true

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)

# ── Resolução dos binários ────────────────────────────────────────────────────
# Padrão de check-roadmap-move-parity.sh: GO_BIN ausente → compila throwaway.
if [[ -z "${GO_BIN:-}" ]]; then
  _GO_TMP=$(cd "$(mktemp -d)" && pwd -P)
  GO_BIN="$_GO_TMP/trackfw-go"
  (cd "$ROOT_DIR" && GOCACHE="$_GO_TMP/cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
# pwd -P garante que o caminho sobrevive a cd em subshells (armadilha /var vs /private/var).
GO_BIN=$(cd "$(dirname "$GO_BIN")" && pwd -P)/$(basename "$GO_BIN")

NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
# PY_ROOT é override-ável para a falsificação com mutant.
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"
PY_ROOT=$(cd "$PY_ROOT" && pwd -P)

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-agents-install-yaml-parity: Go binary not found/executable at $GO_BIN" >&2; exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-agents-install-yaml-parity: Node CLI not found at $NODE_CLI" >&2; exit 1
fi

WORK=$(cd "$(mktemp -d "${TMPDIR:-/tmp}/trackfw-agents-install-yaml-parity.XXXXXX")" && pwd -P)
trap 'rm -rf "$WORK"' EXIT

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ── Invocação de agents install ───────────────────────────────────────────────
# run_install RUNTIME DIR → popula DIR/out DIR/err; devolve rc em INSTALL_RC.
# Usa $HOME sintético para que o catálogo de descoberta não leia o HOME real.
run_install() {
  local rt=$1 dir=$2
  local home_dir="$WORK/home_${rt}_$$"
  mkdir -p "$home_dir"
  local out="$WORK/out_${rt}_$$" err="$WORK/err_${rt}_$$"
  set +e
  case "$rt" in
    go)
      (cd "$dir" && HOME="$home_dir" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 \
         "$GO_BIN" agents install --targets claude --items architect --scope project) \
         >"$out" 2>"$err"
      ;;
    node)
      (cd "$dir" && HOME="$home_dir" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 \
         node "$NODE_CLI" agents install --targets claude --items architect --scope project) \
         >"$out" 2>"$err"
      ;;
    py)
      (cd "$dir" && HOME="$home_dir" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 \
         PYTHONPATH="$PY_ROOT" python3 -m trackfw agents install \
         --targets claude --items architect --scope project) \
         >"$out" 2>"$err"
      ;;
    *)
      echo "run_install: unknown runtime '$rt'" >&2; exit 1 ;;
  esac
  INSTALL_RC=$?
  INSTALL_STDOUT=$(cat "$out")
  INSTALL_STDERR=$(cat "$err")
  rm -f "$out" "$err"
  set -e
}

# ── Nomeador de divergência ───────────────────────────────────────────────────
# name_offender GO_FILE NODE_FILE PY_FILE — imprime qual runtime difere.
name_offender() {
  local gf=$1 nf=$2 pf=$3
  local gn=0 gp=0 np=0
  cmp -s "$gf" "$nf" && gn=1 || true
  cmp -s "$gf" "$pf" && gp=1 || true
  cmp -s "$nf" "$pf" && np=1 || true
  if   [[ $gn -eq 1 && $gp -eq 0 ]]; then echo "python"
  elif [[ $gn -eq 1 && $np -eq 0 ]]; then echo "python"   # redundante, mesmo caso
  elif [[ $gp -eq 1 && $gn -eq 0 ]]; then echo "node"
  elif [[ $np -eq 1 && $gn -eq 0 ]]; then echo "go"
  else                                      echo "all-three"
  fi
}

# ── Scaffolding de fixture ────────────────────────────────────────────────────
# make_project DIR YAML_CONTENT — cria projeto mínimo e escreve trackfw.yaml.
make_project() {
  local dir=$1; shift
  mkdir -p "$dir/docs/roadmaps" "$dir/docs/req"
  printf '%s' "$*" > "$dir/trackfw.yaml"
}

# ── Comparador de três vias ───────────────────────────────────────────────────
# compare_three SCENARIO GO_FILE NODE_FILE PY_FILE [REF_FILE]
# Se REF_FILE for fornecido, cada runtime é comparado contra ele (detecta
# "todos três iguais entre si mas errados"). Sempre faz também three-way.
compare_three() {
  local scenario=$1 gf=$2 nf=$3 pf=$4 ref="${5:-}"

  # Se arquivo de referência externo: cada runtime vs. referência.
  if [[ -n "$ref" ]]; then
    if ! cmp -s "$gf" "$ref"; then
      fail "$scenario/go" "resultado difere da referência esperada"$'\n'"$(diff "$ref" "$gf" || true)"
    fi
    if ! cmp -s "$nf" "$ref"; then
      fail "$scenario/node" "resultado difere da referência esperada"$'\n'"$(diff "$ref" "$nf" || true)"
    fi
    if ! cmp -s "$pf" "$ref"; then
      fail "$scenario/py" "resultado difere da referência esperada"$'\n'"$(diff "$ref" "$pf" || true)"
    fi
  fi

  # Three-way: se divergem, nomear o offender.
  if ! cmp -s "$gf" "$nf" || ! cmp -s "$gf" "$pf"; then
    local offender
    offender=$(name_offender "$gf" "$nf" "$pf")
    fail "$scenario/three-way" "runtimes divergem — offender: $offender"$'\n'\
"--- go ---"$'\n'"$(cat "$gf")"$'\n'\
"--- node ---"$'\n'"$(cat "$nf")"$'\n'\
"--- py ---"$'\n'"$(cat "$pf")"
  else
    ok "$scenario/three-way"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# CENÁRIO 1 — sem chave agents:
# Esperado: bloco anexado ao fim; comentários e ordem preservados.
# ─────────────────────────────────────────────────────────────────────────────
run_scenario_1() {
  local prefix="${1:-S1}"
  local input
  input='# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
wip_limit: 3
'
  # Esperado: bloco ao fim (depois de wip_limit:), exatamente assim.
  local expected
  expected='# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
wip_limit: 3
agents:
  - architect
'

  for rt in go node py; do
    local proj="$WORK/${prefix}_${rt}"
    make_project "$proj" "$input"
    run_install "$rt" "$proj"
    if [[ $INSTALL_RC -ne 0 ]]; then
      fail "$prefix/$rt" "agents install falhou com rc=$INSTALL_RC"$'\n'"stderr: $INSTALL_STDERR"
    fi
    # Vacuidade: o arquivo DEVE ter mudado.
    if cmp -s "$proj/trackfw.yaml" <(printf '%s' "$input"); then
      fail "$prefix/$rt/vacuity" "arquivo não foi alterado — install foi no-op inesperado"
    fi
    cp "$proj/trackfw.yaml" "$WORK/${prefix}_result_${rt}"
  done

  # Escrever referência esperada.
  printf '%s' "$expected" > "$WORK/${prefix}_ref"

  compare_three "$prefix" \
    "$WORK/${prefix}_result_go" "$WORK/${prefix}_result_node" "$WORK/${prefix}_result_py" \
    "$WORK/${prefix}_ref"

  if cmp -s "$WORK/${prefix}_result_go" "$WORK/${prefix}_ref" 2>/dev/null; then
    ok "$prefix/reference"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# CENÁRIO 2 — bloco agents: no meio do arquivo
# Esperado: item entra no bloco; bloco NÃO muda de lugar.
# ─────────────────────────────────────────────────────────────────────────────
run_scenario_2() {
  local prefix="${1:-S2}"
  local input
  input='# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
agents:
  - alpha
wip_limit: 3
'
  local expected
  expected='# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
agents:
  - alpha
  - architect
wip_limit: 3
'

  for rt in go node py; do
    local proj="$WORK/${prefix}_${rt}"
    make_project "$proj" "$input"
    run_install "$rt" "$proj"
    if [[ $INSTALL_RC -ne 0 ]]; then
      fail "$prefix/$rt" "agents install falhou com rc=$INSTALL_RC"$'\n'"stderr: $INSTALL_STDERR"
    fi
    # Vacuidade: o arquivo DEVE ter mudado.
    if cmp -s "$proj/trackfw.yaml" <(printf '%s' "$input"); then
      fail "$prefix/$rt/vacuity" "arquivo não foi alterado — install foi no-op inesperado"
    fi
    cp "$proj/trackfw.yaml" "$WORK/${prefix}_result_${rt}"
  done

  printf '%s' "$expected" > "$WORK/${prefix}_ref"

  compare_three "$prefix" \
    "$WORK/${prefix}_result_go" "$WORK/${prefix}_result_node" "$WORK/${prefix}_result_py" \
    "$WORK/${prefix}_ref"

  if cmp -s "$WORK/${prefix}_result_go" "$WORK/${prefix}_ref" 2>/dev/null; then
    ok "$prefix/reference"
  fi

  # Verificar que o bloco não mudou de lugar: "wip_limit:" deve continuar APÓS agents: block.
  for rt in go node py; do
    local f="$WORK/${prefix}_result_${rt}"
    # Encontra linha de "agents:" e "wip_limit:" — agents deve vir antes de wip_limit.
    local agents_line wip_line
    agents_line=$(grep -n "^agents:" "$f" | head -1 | cut -d: -f1)
    wip_line=$(grep -n "^wip_limit:" "$f" | head -1 | cut -d: -f1)
    if [[ -n "$agents_line" && -n "$wip_line" && "$agents_line" -gt "$wip_line" ]]; then
      fail "$prefix/$rt/block-position" "bloco agents: foi movido para depois de wip_limit:"
    else
      ok "$prefix/$rt/block-position"
    fi
  done
}

# ─────────────────────────────────────────────────────────────────────────────
# CENÁRIO 3 — agents: [flow inline]
# Esperado: arquivo BYTE-IDÊNTICO à entrada; aviso no stderr nomeando arquivo e item.
# ─────────────────────────────────────────────────────────────────────────────
run_scenario_3() {
  local prefix="${1:-S3}"
  local input
  input='# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
agents: [alpha, beta]
wip_limit: 3
'

  for rt in go node py; do
    local proj="$WORK/${prefix}_${rt}"
    make_project "$proj" "$input"
    run_install "$rt" "$proj"
    # rc pode ser 0 (install "completo" mesmo sem registrar) — não falhar por rc.
    # Verificar que o arquivo está BYTE-IDÊNTICO à entrada.
    if ! cmp -s "$proj/trackfw.yaml" <(printf '%s' "$input"); then
      fail "$prefix/$rt/byte-identical" \
        "arquivo foi modificado — esperado byte-idêntico à entrada"$'\n'"diff:"$'\n'"$(diff <(printf '%s' "$input") "$proj/trackfw.yaml" || true)"
    else
      ok "$prefix/$rt/byte-identical"
    fi
    # Stderr deve mencionar o arquivo (caminho), o item e o formato inline.
    if ! echo "$INSTALL_STDERR" | grep -qF "$proj/trackfw.yaml"; then
      fail "$prefix/$rt/stderr-path" "stderr não nomeia o arquivo trackfw.yaml; got: $INSTALL_STDERR"
    else
      ok "$prefix/$rt/stderr-path"
    fi
    if ! echo "$INSTALL_STDERR" | grep -q "architect"; then
      fail "$prefix/$rt/stderr-item" "stderr não menciona o item 'architect'; got: $INSTALL_STDERR"
    else
      ok "$prefix/$rt/stderr-item"
    fi
    if ! echo "$INSTALL_STDERR" | grep -q "inline-flow\|inline_flow\|inline flow"; then
      fail "$prefix/$rt/stderr-inline" "stderr não menciona formato inline; got: $INSTALL_STDERR"
    else
      ok "$prefix/$rt/stderr-inline"
    fi
    cp "$proj/trackfw.yaml" "$WORK/${prefix}_result_${rt}"
  done

  # Three-way: todos devem ser idênticos (= input).
  compare_three "$prefix" \
    "$WORK/${prefix}_result_go" "$WORK/${prefix}_result_node" "$WORK/${prefix}_result_py"
}

# ─────────────────────────────────────────────────────────────────────────────
# CENÁRIO 4 — roadmap_namespacing: flat
# Esperado: chave agents: NÃO é criada; arquivo byte-idêntico à entrada.
# ─────────────────────────────────────────────────────────────────────────────
run_scenario_4() {
  local prefix="${1:-S4}"
  local input
  input='# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: flat
wip_limit: 3
'

  for rt in go node py; do
    local proj="$WORK/${prefix}_${rt}"
    # flat: precisa de subdir backlog para o manager encontrar o projeto
    make_project "$proj" "$input"
    mkdir -p "$proj/docs/roadmaps/backlog"
    run_install "$rt" "$proj"
    # S4: rc deve ser 0 (namespacing flat → retorna sem erro, apenas sem registrar).
    if [[ $INSTALL_RC -ne 0 ]]; then
      fail "$prefix/$rt/rc" "agents install falhou com rc=$INSTALL_RC; stderr: $INSTALL_STDERR"
    fi
    # Vacuidade positiva: install deve ter produzido o artefato de agente
    # (mesmo em flat, os arquivos do agente para --targets claude são escritos).
    if [[ ! -f "$proj/.claude/agents/trackfw-architect.md" ]]; then
      fail "$prefix/$rt/vacuity" "artefato .claude/agents/trackfw-architect.md ausente — install não executou"
    else
      ok "$prefix/$rt/vacuity"
    fi
    # Arquivo deve ser byte-idêntico.
    if ! cmp -s "$proj/trackfw.yaml" <(printf '%s' "$input"); then
      fail "$prefix/$rt/byte-identical" \
        "arquivo foi modificado — esperado byte-idêntico à entrada"$'\n'"diff:"$'\n'"$(diff <(printf '%s' "$input") "$proj/trackfw.yaml" || true)"
    else
      ok "$prefix/$rt/byte-identical"
    fi
    # Não deve ter criado chave agents:.
    if grep -q "^agents:" "$proj/trackfw.yaml"; then
      fail "$prefix/$rt/no-agents-key" "chave agents: criada em projeto flat"
    else
      ok "$prefix/$rt/no-agents-key"
    fi
    cp "$proj/trackfw.yaml" "$WORK/${prefix}_result_${rt}"
  done

  compare_three "$prefix" \
    "$WORK/${prefix}_result_go" "$WORK/${prefix}_result_node" "$WORK/${prefix}_result_py"
}

# ─────────────────────────────────────────────────────────────────────────────
# IDEMPOTÊNCIA — segunda instalação não duplica (S1 e S2)
# ─────────────────────────────────────────────────────────────────────────────
run_idempotence() {
  local scenario=$1 input=$2
  local prefix="idempotence_${scenario}"

  for rt in go node py; do
    local proj="$WORK/${prefix}_${rt}"
    make_project "$proj" "$input"
    # Primeira instalação.
    run_install "$rt" "$proj"
    local after_first="$WORK/${prefix}_first_${rt}"
    cp "$proj/trackfw.yaml" "$after_first"
    # Segunda instalação.
    run_install "$rt" "$proj"
    # Arquivo deve ser byte-idêntico ao pós-primeira.
    if ! cmp -s "$proj/trackfw.yaml" "$after_first"; then
      fail "$prefix/$rt" "segunda instalação alterou o arquivo — não é idempotente"$'\n'"diff:"$'\n'"$(diff "$after_first" "$proj/trackfw.yaml" || true)"
    else
      ok "$prefix/$rt"
    fi
  done
}

# ─────────────────────────────────────────────────────────────────────────────
# MODO --self-test: falsificação embutida
# Cria um mutant-pypi sem a guarda de inline-flow e verifica que o gate reprova
# nomeando "python"; depois verifica que com os três alinhados o gate PASSA.
# ─────────────────────────────────────────────────────────────────────────────
run_self_test() {
  echo "=== --self-test: criando mutant-pypi ==="

  # 1. Criar cópia mutante do pypi.
  local mutant_py="$WORK/mutant-pypi"
  cp -R "$ROOT_DIR/pypi" "$mutant_py"
  local cfg="$mutant_py/trackfw/config.py"

  # Remover as linhas que formam a guarda de inline-flow:
  #   if stripped.startswith("agents:") and not stripped.lstrip().startswith("#"):
  #       sys.stderr.write(...)
  #       return  # installation itself succeeds; only registration is skipped
  # A forma portável: usar Python para editar a cópia.
  python3 - "$cfg" <<'PYEOF'
import sys, re
path = sys.argv[1]
src = open(path).read()
# Remove the inline-flow guard block (three-line pattern).
# Match: the if-condition + the sys.stderr.write block + the return line.
patched = re.sub(
    r'        # Detect flow-inline format.*?'
    r'            return  # installation itself succeeds.*?\n',
    '',
    src,
    flags=re.DOTALL,
)
if patched == src:
    sys.exit("self-test: patch não localizou a guarda de inline-flow em config.py")
open(path, 'w').write(patched)
print("mutant-pypi: guarda de inline-flow removida de", path)
PYEOF

  # 2. Rodar cenário S3 (inline-flow) com PY_ROOT apontando para o mutante.
  echo ""
  echo "=== --self-test: rodando S3 com mutant-pypi (espera FAIL nomeando python) ==="
  local input_s3='# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
agents: [alpha, beta]
wip_limit: 3
'
  # Rodar cada runtime na mesma fixture (projeto separado por runtime).
  local st_go="$WORK/st_go" st_node="$WORK/st_node" st_py="$WORK/st_py"
  for proj in "$st_go" "$st_node" "$st_py"; do
    make_project "$proj" "$input_s3"
  done

  # Go e Node: comportamento real.
  run_install go "$st_go"
  cp "$st_go/trackfw.yaml" "$WORK/st_result_go"
  run_install node "$st_node"
  cp "$st_node/trackfw.yaml" "$WORK/st_result_node"

  # Python mutante.
  local home_mutant="$WORK/home_mutant"
  mkdir -p "$home_mutant"
  set +e
  (cd "$st_py" && HOME="$home_mutant" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 \
     PYTHONPATH="$mutant_py" python3 -m trackfw agents install \
     --targets claude --items architect --scope project \
     >"$WORK/st_py_stdout" 2>"$WORK/st_py_stderr")
  local py_rc=$?
  set -e
  cp "$st_py/trackfw.yaml" "$WORK/st_result_py"

  echo "mutant-python stdout: $(cat "$WORK/st_py_stdout")"
  echo "mutant-python stderr: $(cat "$WORK/st_py_stderr")"
  echo "mutant-python yaml result:"
  cat "$WORK/st_result_py"

  # Verificar: o arquivo do Python mutante NÃO é byte-idêntico à entrada → ele escreveu.
  if cmp -s "$WORK/st_result_py" <(printf '%s' "$input_s3"); then
    echo "FAIL [self-test/mutation]: mutant-python deixou o arquivo inalterado — a mutação não produziu regressão esperada" >&2
    exit 1
  fi
  echo "OK   [self-test/mutation-confirmed]: mutant-python modificou o arquivo (regressão detectável)"

  # Verificar: three-way detecta o offender e o nomeia como "python".
  if cmp -s "$WORK/st_result_go" "$WORK/st_result_node" && \
     ! cmp -s "$WORK/st_result_go" "$WORK/st_result_py"; then
    local offender
    offender=$(name_offender "$WORK/st_result_go" "$WORK/st_result_node" "$WORK/st_result_py")
    echo "OK   [self-test/offender-named]: offender identificado como: $offender"
    if [[ "$offender" != "python" ]]; then
      echo "FAIL [self-test/offender-named]: esperado 'python', got '$offender'" >&2
      exit 1
    fi
    echo "OK   [self-test/offender-is-python]: gate nomearia python como divergente"
  else
    echo "FAIL [self-test/three-way]: go/node não concordam, ou py coincidentemente igual — reexaminar" >&2
    exit 1
  fi

  # 3. Contra-braço: com PY_ROOT real (runtimes alinhados), S3 PASSA.
  echo ""
  echo "=== --self-test: contra-braço — runtimes alinhados (espera PASS) ==="
  local real_go="$WORK/st_real_go" real_node="$WORK/st_real_node" real_py="$WORK/st_real_py"
  for proj in "$real_go" "$real_node" "$real_py"; do
    make_project "$proj" "$input_s3"
  done
  run_install go   "$real_go";   cp "$real_go/trackfw.yaml"   "$WORK/st_real_result_go"
  run_install node "$real_node"; cp "$real_node/trackfw.yaml" "$WORK/st_real_result_node"
  # Python real (PY_ROOT padrão, não mutante).
  local home_real="$WORK/home_real"
  mkdir -p "$home_real"
  set +e
  (cd "$real_py" && HOME="$home_real" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 \
     PYTHONPATH="$PY_ROOT" python3 -m trackfw agents install \
     --targets claude --items architect --scope project \
     >"$WORK/st_real_stdout" 2>"$WORK/st_real_stderr")
  set -e
  cp "$real_py/trackfw.yaml" "$WORK/st_real_result_py"

  if cmp -s "$WORK/st_real_result_go"   <(printf '%s' "$input_s3") && \
     cmp -s "$WORK/st_real_result_node" <(printf '%s' "$input_s3") && \
     cmp -s "$WORK/st_real_result_py"   <(printf '%s' "$input_s3"); then
    echo "OK   [self-test/contra-arm]: todos três byte-idênticos à entrada — gate passaria"
  else
    echo "FAIL [self-test/contra-arm]: um ou mais runtimes modificaram o arquivo no cenário inline" >&2
    diff <(printf '%s' "$input_s3") "$WORK/st_real_result_py" || true
    exit 1
  fi

  echo ""
  echo "=== --self-test: OK — falsificação provada nos dois braços ==="
}

# ─────────────────────────────────────────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--self-test" ]]; then
  run_self_test
  exit 0
fi

echo "check-agents-install-yaml-parity: GO_BIN=$GO_BIN  PY_ROOT=$PY_ROOT"

run_scenario_1
run_scenario_2
run_scenario_3
run_scenario_4

# Idempotência em S1 (sem chave) e S2 (bloco existente).
run_idempotence S1 '# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
wip_limit: 3
'
run_idempotence S2 '# my comment
roadmap_dir: docs/roadmaps
req_dir: docs/req
roadmap_namespacing: by_agent
agents:
  - alpha
wip_limit: 3
'

if [[ $FAIL -eq 0 ]]; then
  echo "check-agents-install-yaml-parity: all scenarios OK"
else
  echo "check-agents-install-yaml-parity: FAILED — see errors above" >&2
  exit 1
fi
