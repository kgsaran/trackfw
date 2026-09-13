#!/usr/bin/env bash
# check-wheel-filename.sh — gate que valida nome e conteúdo de wheel gerada por
# build_wheel.py com versão de pré-lançamento (semver 8.0.0-rc1).
#
# Problema coberto (D6-write, auditoria 2026-09-13):
#   build_wheel.py recebia a versão crua do goreleaser (ex: 8.0.0-rc1) e usava
#   essa string diretamente no nome do arquivo e nos campos internos da wheel.
#   O parser oficial (packaging.utils.parse_wheel_filename) rejeita "8.0.0-rc1"
#   porque o hífen introduz um campo de build-tag, que deve começar com dígito.
#   A publicação no PyPI falharia DEPOIS que o npm já teria publicado — espelhando
#   o acidente da v7.6.0.
#
# Modo normal (parity-rest):
#   Constrói uma wheel com --version 8.0.0-rc1 e valida o nome produzido via
#   parse_wheel_filename.  O gate passa se e somente se o nome é aceito.
#
# Falsificação (check-gates-falsify.sh):
#   Direção 1 (--falsify-raw): valida um nome com versão não-normalizada
#     → parse_wheel_filename deve FALHAR (confirma que o defeito seria detectado).
#   Direção 2 (--falsify-normalized): valida um nome com versão normalizada
#     → parse_wheel_filename deve PASSAR (confirma que a correção é suficiente).
#
# Regra Dura de Reconciliação: cada teste afirma uma conclusão.
#   Gate normal:       build_wheel.py produz nome PEP 440 válido a partir de semver
#                      de pré-lançamento.
#   --falsify-raw:     parse_wheel_filename rejeita nome com versão semver crua —
#                      a gate detectaria a regressão se a normalização fosse removida.
#   --falsify-normalized: parse_wheel_filename aceita nome com versão PEP 440 —
#                      confirma que a forma normalizada é a corretta.
#
# Nota: o gate usa versão de pré-lançamento (8.0.0-rc1) porque "8.0.0" é
# idêntico em semver e PEP 440 — um gate com versão limpa passaria com o
# defeito intacto.
#
# ML-1A-D6write, ROADMAP-2026-09-12-v8-um-binario-muitos-canais.

set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_WHEEL="$REPO_ROOT/pypi/scripts/build_wheel.py"

WORK=""
cleanup() { [[ -n "$WORK" ]] && rm -rf "$WORK" || true; }
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Modo --falsify-raw: nome não-normalizado deve ser rejeitado
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--falsify-raw" ]]; then
  RAW_NAME="trackfw-8.0.0-rc1-py3-none-manylinux_2_17_x86_64.whl"
  result=$(python3 - <<PYEOF
import sys
try:
    from packaging.utils import parse_wheel_filename, InvalidWheelFilename
    try:
        parse_wheel_filename("$RAW_NAME")
        print("PASS")
    except InvalidWheelFilename as e:
        print(f"FAIL: {e}")
except ImportError:
    print("SKIP: packaging indisponivel")
PYEOF
)
  if [[ "$result" == FAIL:* ]]; then
    echo "OK   [falsify/wheel-filename/raw]: nome não-normalizado rejeitado — $result"
    exit 0
  elif [[ "$result" == SKIP:* ]]; then
    if [[ -n "${CI:-}" ]]; then
      echo "FAIL [falsify/wheel-filename/raw]: 'packaging' ausente no runner (CI=true) — instalar no job: pip install packaging" >&2
      exit 1
    else
      echo "SKIP [falsify/wheel-filename/raw]: packaging indisponivel localmente — instale com: pip install packaging"
      exit 0
    fi
  else
    echo "FAIL [falsify/wheel-filename/raw]: esperava rejeição, obteve: $result" >&2
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Modo --falsify-normalized: nome normalizado deve ser aceito
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--falsify-normalized" ]]; then
  NORM_NAME="trackfw-8.0.0rc1-py3-none-manylinux_2_17_x86_64.whl"
  result=$(python3 - <<PYEOF
import sys
try:
    from packaging.utils import parse_wheel_filename, InvalidWheelFilename
    try:
        parse_wheel_filename("$NORM_NAME")
        print("PASS")
    except InvalidWheelFilename as e:
        print(f"FAIL: {e}")
except ImportError:
    print("SKIP: packaging indisponivel")
PYEOF
)
  if [[ "$result" == "PASS" ]]; then
    echo "OK   [falsify/wheel-filename/normalized]: nome normalizado aceito"
    exit 0
  elif [[ "$result" == SKIP:* ]]; then
    if [[ -n "${CI:-}" ]]; then
      echo "FAIL [falsify/wheel-filename/normalized]: 'packaging' ausente no runner (CI=true) — instalar no job: pip install packaging" >&2
      exit 1
    else
      echo "SKIP [falsify/wheel-filename/normalized]: packaging indisponivel localmente — instale com: pip install packaging"
      exit 0
    fi
  else
    echo "FAIL [falsify/wheel-filename/normalized]: esperava aceitação, obteve: $result" >&2
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Modo normal: constrói wheel com versão semver de pré-lançamento e valida
# ---------------------------------------------------------------------------
# Checks cobertos:
#   1. Nome aceito por parse_wheel_filename (PEP 440 válido)
#   2. Versão parseada = Version("8.0.0-rc1") — guarda contra normalização parcial
#      que produza versão diferente (ex.: strip do rc → 8.0.0)
#   3. Prefixo dist-info interno = trackfw-8.0.0rc1.dist-info (mesmo defeito no zip)
#   4. Campo Version: no METADATA = 8.0.0rc1 (quarto sítio que o defeito original corrompía)
#
# Regra Dura de Reconciliação — conclusão do relatório que cada check afirma:
#   Check 1: a versão semver 8.0.0-rc1 no nome do arquivo é inválida (InvalidWheelFilename
#            com "Invalid build number: rc1") quando a normalização está ausente.
#   Check 2: a normalização produz exatamente 8.0.0rc1, não outra grafia (ex.: strip do rc).
#   Checks 3+4: o mesmo defeito que corrompe o nome de arquivo corrompe também o prefixo
#               dist-info e o campo Version: no METADATA — os quatro sítios são corrigidos
#               ou nenhum é (são derivados de dist_name, que usa a versão normalizada).
#
# Nota adicional declarada pelo handoff: nenhum dos 885 checks do `make quality` construía
# uma wheel com versão de pré-lançamento — por isso o gate ficou verde com este defeito
# dentro. Os checks estão corretos sobre o que afirmam; nenhum afirmava isto.

WORK=$(mktemp -d)
FAKEBIN="$WORK/fakebin"
OUTDIR="$WORK/wheels"

# Fake ELF header — suficiente para build_wheel.py aceitar como binário
printf '\x7fELF' > "$FAKEBIN"

# Constrói a wheel com a versão semver crua — build_wheel.py deve normalizar
python3 "$BUILD_WHEEL" \
  --binary "$FAKEBIN" \
  --version "8.0.0-rc1" \
  --platform "manylinux_2_17_x86_64" \
  --output "$OUTDIR" > /dev/null

WHL=$(ls "$OUTDIR"/*.whl 2>/dev/null | head -1)
if [[ -z "$WHL" ]]; then
  echo "FAIL [wheel-filename]: build_wheel.py não produziu nenhuma .whl — guarda de vacuidade" >&2
  exit 1
fi

BASENAME=$(basename "$WHL")

# Valida nome + internos com o parser oficial
result=$(python3 - "$BASENAME" "$WHL" <<PYEOF
import sys, zipfile
fname = sys.argv[1]
whl_path = sys.argv[2]

try:
    from packaging.utils import parse_wheel_filename, InvalidWheelFilename
    from packaging.version import Version
except ImportError:
    # Nota: este bloco é código morto em modo normal. build_wheel.py (linha acima)
    # já falha com SystemExit se 'packaging' estiver ausente — o shell encerra
    # antes de chegar aqui (set -euo pipefail). Mantido por defensividade: se o
    # ambiente divergir (ex: venv partido com packaging parcialmente instalado),
    # emite uma mensagem clara em vez de traceback.
    import os as _os
    if _os.environ.get("CI"):
        print("FAIL: 'packaging' ausente no runner (CI=true) — instalar no job: pip install packaging")
    else:
        print("SKIP: packaging indisponivel (instale com: pip install packaging)")
    sys.exit(0)

errors = []

# Check 1: nome aceito pelo parser oficial
try:
    parsed_name, parsed_ver, parsed_build, parsed_tags = parse_wheel_filename(fname)
except InvalidWheelFilename as e:
    errors.append(f"filename rejected: {e}")
    parsed_ver = None

# Check 2: versão parseada bate com Version("8.0.0-rc1") = 8.0.0rc1
if parsed_ver is not None:
    expected_ver = Version("8.0.0-rc1")
    if parsed_ver != expected_ver:
        errors.append(f"parsed version mismatch: expected {expected_ver}, got {parsed_ver}")

# Checks 3+4: internos da wheel
expected_pep440 = str(Version("8.0.0-rc1"))  # "8.0.0rc1"
expected_dist_info = f"trackfw-{expected_pep440}.dist-info"
try:
    with zipfile.ZipFile(whl_path) as zf:
        names = zf.namelist()
        # Check 3: prefixo dist-info
        meta_paths = [n for n in names if n.endswith("/METADATA") and ".dist-info/" in n]
        if not meta_paths:
            errors.append("no METADATA found inside wheel zip")
        else:
            actual_prefix = meta_paths[0].split("/")[0]
            if actual_prefix != expected_dist_info:
                errors.append(f"dist-info prefix: expected '{expected_dist_info}', got '{actual_prefix}'")
            # Check 4: campo Version: no METADATA
            meta_bytes = zf.read(meta_paths[0]).decode("utf-8", errors="replace")
            version_lines = [l for l in meta_bytes.splitlines() if l.startswith("Version:")]
            if not version_lines:
                errors.append("METADATA has no Version: field")
            else:
                actual_version = version_lines[0].split(":", 1)[1].strip()
                if actual_version != expected_pep440:
                    errors.append(f"METADATA Version: expected '{expected_pep440}', got '{actual_version}'")
except Exception as e:
    errors.append(f"zip inspection failed: {e}")

if errors:
    print("FAIL: " + "; ".join(errors))
else:
    print("PASS")
PYEOF
)

case "$result" in
  PASS)
    echo "OK   [wheel-filename]: '$BASENAME' — nome PEP 440 válido, versão=8.0.0rc1, internos consistentes"
    ;;
  SKIP:*)
    # Código morto em modo normal (build_wheel.py hard-fail vem primeiro).
    # Emitido apenas em caso de venv partido; em CI vira FAIL via o bloco
    # except ImportError acima — este braço só é atingido localmente.
    echo "SKIP [wheel-filename]: packaging indisponível — ${result#SKIP: }"
    ;;
  FAIL:*)
    echo "FAIL [wheel-filename]: ${result#FAIL: }" >&2
    exit 1
    ;;
  *)
    echo "FAIL [wheel-filename]: resposta inesperada: $result" >&2
    exit 1
    ;;
esac
