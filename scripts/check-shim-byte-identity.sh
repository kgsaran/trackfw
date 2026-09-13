#!/usr/bin/env bash
# check-shim-byte-identity.sh — gate permanente AC9 (ML-1D, ROADMAP-2026-09-12-v8).
#
# Porta a Pergunta 15 do windows-probe.yml para Linux/macOS: prova que o shim
# npm/bin/trackfw.js NÃO modifica nenhum byte nem nenhum exit code ao delegar
# para o binário nativo trackfw (Go), para os 5 comandos exercitados.
#
# RELAÇÃO COM O SHIM:
#   npm/bin/trackfw.js é o shim v8 (ported de prototype/packages/trackfw-shim/,
#   validado em darwin/arm64, win32/arm64, win32/x64 — ML-1B). Usa spawnSync
#   com stdio:'inherit', o que implica zero bytes alterados em teoria. Este gate
#   confirma empiricamente em Linux/macOS.
#   npm/bin/trackfw (sem .js) é o CLI Node.js legado — NÃO é testado aqui.
#
# COMANDOS EXERCITADOS (e a conclusão que cada braço afirma — CLAUDE.md regra
# dura de reconciliação):
#
#   C1. version
#       Afirma: o shim repassa stdout de `version` byte-identicamente.
#
#   C2. validate --json
#       Afirma: o shim repassa stdout de `validate --json` byte-identicamente
#       (comparação mais substantiva: ~30 kB de JSON em stdout).
#
#   C3. status
#       Afirma: o shim repassa stdout de `status` byte-identicamente.
#
#   C4. context --json
#       Afirma: o shim repassa stderr de `context --json` byte-identicamente
#       (cobra responde com erro de flag desconhecida em stderr; braço prova
#       que o erro é transparente ao shim, não mascarado por ele).
#
#   C5. version --nonexistent
#       Afirma: o shim repassa exit code não-zero de `version --nonexistent`
#       identicamente ao binário nativo — prova de transparência de exit code
#       de violação, sem modificação pelo shim.
#
# GUARDA DE VACUIDADE — piso de substantividade:
#   Cada comparação é classificada como:
#     SUBSTANTIVE   — pelo menos um stream (stdout ou stderr) tinha bytes
#                     em ambos os lados e foram comparados byte-a-byte.
#     VACUOUS_EMPTY — ambos os lados zeraram todos os streams; só prova
#                     exit code, não transparência de bytes.
#   Se SUBSTANTIVE < SUBSTANTIVE_MIN (4), reprova com mensagem explicando
#   quantas comparações foram substantivas vs. vacuosas.
#   Se "vazio contra vazio" passa sem falhar, o gate não prova nada sobre
#   bytes — apenas sobre exit code.
#
# RUÍDO DE PISO (noise floor):
#   Antes de comparar shim vs. nativo, compara nativo vs. nativo (duas
#   invocações idênticas). Se o nativo não for idempotente, a comparação
#   shim vs. nativo seria não-confiável — o braço é marcado NOISE e
#   ignorado (não conta para SUBSTANTIVE).
#
# AMBIENTE INCOMPLETO vs. DEFEITO:
#   go, node ausentes no PATH → SKIP nomeado, exit 0 (ambiente, não defeito).
#   Falha em go build, setup do staging, comparação → FAIL nomeado, exit 1
#   (defeito de produto).
#
# STAGING (sem npm install):
#   O shim npm/bin/trackfw.js usa apenas módulos core do Node.js mais
#   require.resolve('@trackfw-bin/<platform>/package.json'). O staging monta:
#     $WORK/env/bin/trackfw.js          ← cópia do shim
#     $WORK/env/node_modules/@trackfw-bin/<slug>/package.json ← manifest gerado
#     $WORK/env/node_modules/@trackfw-bin/<slug>/bin/trackfw  ← binário compilado
#   Sem npm install — rápido e hermético.
#
# CI:
#   - Linux/macOS: este script em parity-rest do Makefile e no job
#     shim-byte-identity do quality.yml (linux/darwin x64 runners).
#   - Windows: Pergunta 15 do windows-probe.yml (PowerShell, mantido separado).
set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0
SUBSTANTIVE=0
TOTAL_CMDS=5
SUBSTANTIVE_MIN=4  # Piso: pelo menos 4 de 5 comparações devem ter bytes

ok()   { echo "ok  : $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1" >&2; FAIL=$((FAIL + 1)); }
note() { echo "NOTE: $1"; }

# ── Guarda de dependências de ambiente ──────────────────────────────────────

if ! command -v go >/dev/null 2>&1; then
  echo "SKIP: go não encontrado no PATH — ambiente incompleto, não defeito de shim"
  echo ""
  echo "shim-byte-identity: SKIP (go ausente)"
  exit 0
fi

if ! command -v node >/dev/null 2>&1; then
  echo "SKIP: node não encontrado no PATH — ambiente incompleto, não defeito de shim"
  echo ""
  echo "shim-byte-identity: SKIP (node ausente)"
  exit 0
fi

SHIM_JS="$REPO_ROOT/npm/bin/trackfw.js"
if [[ ! -f "$SHIM_JS" ]]; then
  fail "npm/bin/trackfw.js não encontrado — ML-1B não foi executado?"
  echo ""
  echo "shim-byte-identity: $PASS passed, $FAIL failed"
  exit 1
fi

# ── Detecção de plataforma ──────────────────────────────────────────────────

case "$(uname -s)" in
  Darwin) PLAT_OS="darwin" ;;
  Linux)  PLAT_OS="linux"  ;;
  *)
    echo "SKIP: plataforma $(uname -s) não suportada (Windows usa P15 do windows-probe.yml)"
    echo ""
    echo "shim-byte-identity: SKIP (plataforma não suportada)"
    exit 0
    ;;
esac

case "$(uname -m)" in
  x86_64)        PLAT_ARCH="x64"   ;;
  aarch64|arm64) PLAT_ARCH="arm64" ;;
  *)             PLAT_ARCH="x64"   ;;
esac

PLAT_SLUG="${PLAT_OS}-${PLAT_ARCH}"

# ── Diretório de trabalho ────────────────────────────────────────────────────

WORK=$(mktemp -d)
WORK="$(cd "$WORK" && pwd -P)"
trap 'rm -rf "$WORK"' EXIT

WORK_NATIVE="$WORK/native"
WORK_ENV="$WORK/env"
WORK_COMPARE="$WORK/compare"

mkdir -p "$WORK_NATIVE" "$WORK_COMPARE"

# ── Passo 1: Compilar binário nativo ─────────────────────────────────────────

echo "→ compilando binário nativo ($PLAT_OS/$PLAT_ARCH)..."
NATIVE_BIN="$WORK_NATIVE/trackfw"
if ! (cd "$REPO_ROOT" && go build -o "$NATIVE_BIN" ./cmd/trackfw 2>&1); then
  fail "go build falhou — defeito de compilação, não defeito de shim"
  echo ""
  echo "shim-byte-identity: $PASS passed, $FAIL failed"
  exit 1
fi
if [[ ! -x "$NATIVE_BIN" ]]; then
  fail "binário nativo não executável após go build: $NATIVE_BIN"
  echo ""
  echo "shim-byte-identity: $PASS passed, $FAIL failed"
  exit 1
fi
ok "go build: binário nativo em $WORK_NATIVE/trackfw"

# ── Passo 2: Gerar manifests de plataforma ───────────────────────────────────

echo "→ gerando manifests de plataforma..."
if ! "$SCRIPT_DIR/gen-platform-manifests.sh" >/dev/null 2>&1; then
  fail "gen-platform-manifests.sh falhou"
  echo ""
  echo "shim-byte-identity: $PASS passed, $FAIL failed"
  exit 1
fi

PLATFORM_SRC="$REPO_ROOT/build/npm-platform/@trackfw-bin/$PLAT_SLUG"
if [[ ! -d "$PLATFORM_SRC" ]]; then
  fail "diretório de plataforma ausente após geração: build/npm-platform/@trackfw-bin/$PLAT_SLUG"
  echo ""
  echo "shim-byte-identity: $PASS passed, $FAIL failed"
  exit 1
fi
ok "manifests gerados para $PLAT_SLUG"

# ── Passo 3: Montar staging do shim ──────────────────────────────────────────
# O shim npm/bin/trackfw.js usa require.resolve('@trackfw-bin/<platform>/package.json').
# O staging monta um node_modules limpo relativo à cópia do shim, sem npm install.

SHIM_COPY="$WORK_ENV/bin/trackfw.js"
PLATFORM_NM="$WORK_ENV/node_modules/@trackfw-bin/$PLAT_SLUG"

mkdir -p "$(dirname "$SHIM_COPY")"
cp "$SHIM_JS" "$SHIM_COPY"

mkdir -p "$PLATFORM_NM/bin"
cp "$PLATFORM_SRC/package.json" "$PLATFORM_NM/package.json"
cp "$NATIVE_BIN" "$PLATFORM_NM/bin/trackfw"
chmod +x "$PLATFORM_NM/bin/trackfw"

ok "staging: shim + plataforma em $WORK_ENV"

# Verificar que o binário resolvido pelo staging é o compilado
STAGED_BIN="$PLATFORM_NM/bin/trackfw"
if [[ ! -x "$STAGED_BIN" ]]; then
  fail "binário não executável no staging: $STAGED_BIN"
  echo ""
  echo "shim-byte-identity: $PASS passed, $FAIL failed"
  exit 1
fi

# ── Passo 4: Testar que o shim resolve corretamente ──────────────────────────

echo "→ testando resolução do shim..."
RESOLVE_OUT="$WORK/shim-resolve.txt"
if ! node "$SHIM_COPY" version >"$RESOLVE_OUT" 2>&1; then
  fail "shim falhou em invocação básica (node trackfw.js version): $(cat "$RESOLVE_OUT")"
  echo ""
  echo "shim-byte-identity: $PASS passed, $FAIL failed"
  exit 1
fi
ok "shim resolve e delega corretamente para o binário nativo"

# ── Passo 5: Ruído de piso (noise floor) ─────────────────────────────────────
# Dois comandos com saída determinística esperada (version, validate --json).
# Se o nativo não for idempotente, a comparação shim vs. nativo seria enganosa.

# NONDETERMINISTIC é uma lista de labels separados por espaços simples.
# Não usa declare -A (bash 4+) para manter compatibilidade com bash 3.2 (macOS padrão).
# Acesso: [[ " $NONDETERMINISTIC " == *" $label "* ]]
NONDETERMINISTIC=""

_noise_check() {
  local label="$1"; shift
  local out1="$WORK_COMPARE/noise-${label}-1-out" err1="$WORK_COMPARE/noise-${label}-1-err"
  local out2="$WORK_COMPARE/noise-${label}-2-out" err2="$WORK_COMPARE/noise-${label}-2-err"
  (cd "$REPO_ROOT" && "$NATIVE_BIN" "$@" >"$out1" 2>"$err1") || true
  (cd "$REPO_ROOT" && "$NATIVE_BIN" "$@" >"$out2" 2>"$err2") || true
  if ! cmp -s "$out1" "$out2" || ! cmp -s "$err1" "$err2"; then
    NONDETERMINISTIC="${NONDETERMINISTIC} ${label}"
    note "NOISE: $label — nativo não-determinístico (nativo vs. nativo diverge), comparação shim ignorada"
  fi
}

echo ""
echo "→ medindo ruído de piso..."
_noise_check "version" version
_noise_check "validate-json" validate --json
_noise_check "status" status

# ── Passo 6: Comparações shim vs. nativo ─────────────────────────────────────

_compare_cmd() {
  local label="$1"; shift
  local args=("$@")

  if [[ " ${NONDETERMINISTIC} " == *" ${label} "* ]]; then
    note "SKIP_NOISE: $label ignorado por não-determinismo no ruído de piso"
    return
  fi

  local n_out="$WORK_COMPARE/${label}-native-stdout"
  local n_err="$WORK_COMPARE/${label}-native-stderr"
  local s_out="$WORK_COMPARE/${label}-shim-stdout"
  local s_err="$WORK_COMPARE/${label}-shim-stderr"

  local n_exit=0 s_exit=0
  (cd "$REPO_ROOT" && "$NATIVE_BIN" "${args[@]}" >"$n_out" 2>"$n_err") || n_exit=$?
  (cd "$REPO_ROOT" && node "$SHIM_COPY" "${args[@]}" >"$s_out" 2>"$s_err") || s_exit=$?

  local n_stdout_bytes s_stdout_bytes n_stderr_bytes s_stderr_bytes
  n_stdout_bytes=$(wc -c <"$n_out")
  s_stdout_bytes=$(wc -c <"$s_out")
  n_stderr_bytes=$(wc -c <"$n_err")
  s_stderr_bytes=$(wc -c <"$s_err")

  local has_bytes=false stdout_ok=true stderr_ok=true exit_ok=true

  if [[ "$n_stdout_bytes" -gt 0 || "$s_stdout_bytes" -gt 0 ]]; then
    has_bytes=true
    if ! cmp -s "$n_out" "$s_out"; then
      stdout_ok=false
      fail "$label: stdout diverge (nativo ${n_stdout_bytes}B vs shim ${s_stdout_bytes}B)"
    fi
  fi

  if [[ "$n_stderr_bytes" -gt 0 || "$s_stderr_bytes" -gt 0 ]]; then
    has_bytes=true
    if ! cmp -s "$n_err" "$s_err"; then
      stderr_ok=false
      fail "$label: stderr diverge (nativo ${n_stderr_bytes}B vs shim ${s_stderr_bytes}B)"
    fi
  fi

  if [[ "$n_exit" -ne "$s_exit" ]]; then
    exit_ok=false
    fail "$label: exit code diverge (nativo=$n_exit, shim=$s_exit)"
  fi

  if $has_bytes; then
    SUBSTANTIVE=$((SUBSTANTIVE + 1))
    if $stdout_ok && $stderr_ok && $exit_ok; then
      ok "$label: BYTE_IDENTICAL + EXIT_MATCH($n_exit) [SUBSTANTIVE]"
    fi
  else
    note "$label: ambos lados produziram 0 bytes — prova exit code match apenas"
    note "  nativo=$n_exit, shim=$s_exit [VACUOUS_EMPTY]"
    if $exit_ok; then
      ok "$label: EXIT_MATCH($n_exit) [VACUOUS_EMPTY — não prova transparência de bytes]"
    fi
  fi
}

echo ""
echo "── Comparações shim vs. nativo ─────────────────────────────────────────"
echo ""

# C1: afirma que o shim repassa stdout de `version` byte-identicamente.
_compare_cmd "version" version

# C2: afirma que o shim repassa stdout de `validate --json` byte-identicamente.
_compare_cmd "validate-json" validate --json

# C3: afirma que o shim repassa stdout de `status` byte-identicamente.
_compare_cmd "status" status

# C4: afirma que o shim repassa stderr de `context --json` byte-identicamente
#     (cobra responde com erro de flag desconhecida em stderr).
_compare_cmd "context-json" context --json

# C5: afirma que o shim repassa exit code não-zero de `version --nonexistent`
#     identicamente ao binário nativo (prova de transparência de exit code de
#     violação, sem modificação pelo shim).
_compare_cmd "version-badflag" version --nonexistent

echo ""

# ── Passo 7: Guarda de vacuidade — piso de substantividade ──────────────────

if [[ "$SUBSTANTIVE" -lt "$SUBSTANTIVE_MIN" ]]; then
  fail "VACUITY: só $SUBSTANTIVE de $TOTAL_CMDS comparações foram substantivas (mínimo $SUBSTANTIVE_MIN)"
  fail "  Comparações substantivas provam transparência de bytes; vacuosas provam apenas exit code"
else
  ok "piso de substantividade: $SUBSTANTIVE/$TOTAL_CMDS comparações com bytes [mínimo $SUBSTANTIVE_MIN]"
fi

# ── Resultado ─────────────────────────────────────────────────────────────────

echo ""
echo "shim-byte-identity: $PASS passed, $FAIL failed"
echo "  (comparações substantivas: $SUBSTANTIVE/$TOTAL_CMDS; mínimo: $SUBSTANTIVE_MIN)"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
