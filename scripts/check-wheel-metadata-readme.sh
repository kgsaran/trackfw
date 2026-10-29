#!/usr/bin/env bash
# check-wheel-metadata-readme.sh — gate que verifica que a wheel emitida por
# build_wheel.py contém Description-Content-Type e o corpo do README.md no METADATA.
#
# Problema coberto (ML-1B, ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x):
#   A página do PyPI do trackfw exibia descrição vazia porque build_wheel.py escrevia
#   apenas os campos mínimos de cabeçalho no METADATA — sem Description-Content-Type
#   nem corpo. O campo `description` da PyPI API retornava string vazia.
#   Correção: Metadata-Version 2.1 (já emitida) suporta Description-Content-Type e
#   o corpo após linha em branco (PEP 566). O script passa o README.md como --readme.
#
# Modo normal (parity-rest):
#   Copia o README.md raiz para temp, constrói uma wheel linux amd64 com binário dummy,
#   descompacta o METADATA e assere:
#     (a) cabeçalho "Description-Content-Type: text/markdown" presente
#     (b) corpo do METADATA byte-idêntico ao README.md
#
# Contra-braços (self-falsifying — incluídos neste script):
#   Arm C:  --readme aponta para arquivo inexistente → build_wheel falha (exit ≠ 0),
#           nenhuma .whl produzida.
#   Arm C2: --readme aponta para arquivo vazio → idem.
#   Arm D1: cópia de build_wheel.py sem a linha Description-Content-Type (sed) →
#           wheel é produzida (exit 0), mas asserção (a) falha.
#   Arm D2: cópia de build_wheel.py com corpo neutralizado (body = b"") →
#           wheel é produzida (exit 0), mas asserção (b) falha.
#
# Regra Dura de Reconciliação — uma frase por teste/braço:
#   Normal:   build_wheel.py grava Description-Content-Type e o README como corpo do
#             METADATA quando o README existe e não está vazio.
#   Arm C:    build_wheel.py recusa (exit ≠ 0) e não cria .whl quando o README
#             está ausente — nunca produz uma wheel com descrição vazia silenciosamente.
#   Arm C2:   build_wheel.py recusa (exit ≠ 0) e não cria .whl quando o README
#             está vazio — mesma garantia para o caso de arquivo vazio.
#   Arm D1:   a asserção (a) detecta a ausência de Description-Content-Type — se a
#             linha fosse removida de build_wheel.py a gate travaria o merge.
#   Arm D2:   a asserção (b) detecta corpo METADATA vazio — se a escrita do corpo fosse
#             suprimida em build_wheel.py a gate travaria o merge.
#
# ML-1B, ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x.

set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$SCRIPT_DIR/lib-crlf-normalize.sh"
BUILD_WHEEL="$REPO_ROOT/pypi/scripts/build_wheel.py"
README_SRC="$REPO_ROOT/README.md"

WORK=""
cleanup() { [ -n "$WORK" ] && rm -rf "$WORK" || true; }
trap cleanup EXIT
WORK="$(mktemp -d)"

FAKEBIN="$WORK/fakebin"
# Fake ELF header — suficiente para build_wheel.py aceitar como binário
printf '\x7fELF' > "$FAKEBIN"

# ---------------------------------------------------------------------------
# Modo normal: constrói wheel e verifica (a) Content-Type e (b) corpo
# ---------------------------------------------------------------------------
OUTDIR="$WORK/wheels"
mkdir -p "$OUTDIR"

python3 "$BUILD_WHEEL" \
    --binary  "$FAKEBIN" \
    --version "11.1.0" \
    --platform "manylinux_2_17_x86_64" \
    --output  "$OUTDIR" \
    --readme  "$README_SRC" \
    > /dev/null

WHL=$(ls "$OUTDIR"/*.whl 2>/dev/null | head -1)
if [ -z "$WHL" ]; then
    echo "FAIL [wheel-metadata-readme]: build_wheel.py não produziu nenhuma .whl — guarda de vacuidade" >&2
    exit 1
fi

result=$(python3 - "$WHL" "$README_SRC" <<'PYEOF' | strip_cr
import sys, zipfile

whl_path = sys.argv[1]
readme_path = sys.argv[2]

errors = []

with open(readme_path, "rb") as fh:
    expected_body = fh.read()

with zipfile.ZipFile(whl_path) as zf:
    meta_paths = [n for n in zf.namelist() if n.endswith("/METADATA") and ".dist-info/" in n]
    if not meta_paths:
        print("FAIL: nenhum METADATA encontrado na wheel")
        sys.exit(0)
    raw = zf.read(meta_paths[0])

    # Core Metadata 2.1+: cabeçalhos + linha em branco + corpo de descrição
    if b"\n\n" in raw:
        headers_part, body = raw.split(b"\n\n", 1)
    else:
        headers_part = raw
        body = b""

    headers_text = headers_part.decode("utf-8", errors="replace")

    # Asserção (a): Description-Content-Type: text/markdown
    if "Description-Content-Type: text/markdown" not in headers_text:
        errors.append("ASSERT-A-FAIL: Description-Content-Type: text/markdown ausente dos cabeçalhos")

    # Asserção (b): corpo byte-idêntico ao README.md
    if body != expected_body:
        errors.append(
            f"ASSERT-B-FAIL: corpo diverge do README.md "
            f"(esperado {len(expected_body)} bytes, obtido {len(body)} bytes)"
        )

if errors:
    print("FAIL: " + "; ".join(errors))
else:
    print("PASS")
PYEOF
)

case "$result" in
    PASS)
        echo "OK   [wheel-metadata-readme/normal]: Description-Content-Type presente e corpo igual ao README.md"
        ;;
    FAIL:*)
        echo "FAIL [wheel-metadata-readme/normal]: ${result#FAIL: }" >&2
        exit 1
        ;;
    *)
        echo "FAIL [wheel-metadata-readme/normal]: resposta inesperada: $result" >&2
        exit 1
        ;;
esac

# ---------------------------------------------------------------------------
# Arm C: README ausente → build falha, nenhuma .whl produzida
# ---------------------------------------------------------------------------
ARM_C_OUT="$WORK/arm-c-wheels"
mkdir -p "$ARM_C_OUT"
arm_c_rc=0
python3 "$BUILD_WHEEL" \
    --binary  "$FAKEBIN" \
    --version "11.1.0" \
    --platform "manylinux_2_17_x86_64" \
    --output  "$ARM_C_OUT" \
    --readme  "$WORK/nao-existe.md" \
    > /dev/null 2>&1 || arm_c_rc=$?
if [ "$arm_c_rc" -eq 0 ]; then
    echo "FAIL [wheel-metadata-readme/arm-c]: build_wheel.py deveria sair com erro para README ausente, mas saiu 0" >&2
    exit 1
fi
if ls "$ARM_C_OUT"/*.whl 2>/dev/null | head -1 | grep -q .; then
    echo "FAIL [wheel-metadata-readme/arm-c]: build_wheel.py produziu .whl mesmo com README ausente" >&2
    exit 1
fi
echo "OK   [wheel-metadata-readme/arm-c]: build_wheel.py recusou (exit $arm_c_rc) com README ausente, sem .whl"

# ---------------------------------------------------------------------------
# Arm C2: README vazio → build falha, nenhuma .whl produzida
# ---------------------------------------------------------------------------
ARM_C2_README="$WORK/empty-readme.md"
ARM_C2_OUT="$WORK/arm-c2-wheels"
mkdir -p "$ARM_C2_OUT"
printf '' > "$ARM_C2_README"
arm_c2_rc=0
python3 "$BUILD_WHEEL" \
    --binary  "$FAKEBIN" \
    --version "11.1.0" \
    --platform "manylinux_2_17_x86_64" \
    --output  "$ARM_C2_OUT" \
    --readme  "$ARM_C2_README" \
    > /dev/null 2>&1 || arm_c2_rc=$?
if [ "$arm_c2_rc" -eq 0 ]; then
    echo "FAIL [wheel-metadata-readme/arm-c2]: build_wheel.py deveria sair com erro para README vazio, mas saiu 0" >&2
    exit 1
fi
if ls "$ARM_C2_OUT"/*.whl 2>/dev/null | head -1 | grep -q .; then
    echo "FAIL [wheel-metadata-readme/arm-c2]: build_wheel.py produziu .whl mesmo com README vazio" >&2
    exit 1
fi
echo "OK   [wheel-metadata-readme/arm-c2]: build_wheel.py recusou (exit $arm_c2_rc) com README vazio, sem .whl"

# ---------------------------------------------------------------------------
# Arm D1: sabotagem — linha Description-Content-Type removida
#   Expectativa: wheel produzida (exit 0), asserção (a) detecta ausência
# ---------------------------------------------------------------------------
ARM_D1_SCRIPT="$WORK/arm-d1-build_wheel.py"
ARM_D1_OUT="$WORK/arm-d1-wheels"
mkdir -p "$ARM_D1_OUT"

# Gera cópia sabotada: remove a linha com Description-Content-Type
sed '/Description-Content-Type/d' "$BUILD_WHEEL" > "$ARM_D1_SCRIPT"

# Verifica que a cópia sabotada difere do original
if cmp -s "$BUILD_WHEEL" "$ARM_D1_SCRIPT"; then
    echo "FAIL [wheel-metadata-readme/arm-d1]: sed não produziu diferença na cópia sabotada" >&2
    exit 1
fi

# A cópia sabotada deve construir a wheel com sucesso (Python válido)
arm_d1_build_rc=0
python3 "$ARM_D1_SCRIPT" \
    --binary  "$FAKEBIN" \
    --version "11.1.0" \
    --platform "manylinux_2_17_x86_64" \
    --output  "$ARM_D1_OUT" \
    --readme  "$README_SRC" \
    > /dev/null 2>&1 || arm_d1_build_rc=$?
if [ "$arm_d1_build_rc" -ne 0 ]; then
    echo "FAIL [wheel-metadata-readme/arm-d1]: script sabotado falhou na construção (rc=$arm_d1_build_rc) — a sabotagem deve preservar Python válido" >&2
    exit 1
fi
D1_WHL=$(ls "$ARM_D1_OUT"/*.whl 2>/dev/null | head -1)
if [ -z "$D1_WHL" ]; then
    echo "FAIL [wheel-metadata-readme/arm-d1]: script sabotado não produziu .whl" >&2
    exit 1
fi

# Verifica que a asserção (a) falha para esta wheel
d1_result=$(python3 - "$D1_WHL" "$README_SRC" <<'PYEOF' | strip_cr
import sys, zipfile

whl_path = sys.argv[1]
readme_path = sys.argv[2]
with open(readme_path, "rb") as fh:
    expected_body = fh.read()
with zipfile.ZipFile(whl_path) as zf:
    meta_paths = [n for n in zf.namelist() if n.endswith("/METADATA") and ".dist-info/" in n]
    if not meta_paths:
        print("INDETERMINATE: no METADATA")
        sys.exit(0)
    raw = zf.read(meta_paths[0])
    headers_part = raw.split(b"\n\n", 1)[0] if b"\n\n" in raw else raw
    headers_text = headers_part.decode("utf-8", errors="replace")
    if "Description-Content-Type: text/markdown" in headers_text:
        print("PASS-UNEXPECTED: Content-Type ainda presente — sabotagem não funcionou")
    else:
        print("FAIL-AS-EXPECTED: Description-Content-Type ausente")
PYEOF
)
if echo "$d1_result" | grep -q "FAIL-AS-EXPECTED"; then
    echo "OK   [wheel-metadata-readme/arm-d1]: sabotagem (Content-Type removido) detectada pela asserção (a)"
else
    echo "FAIL [wheel-metadata-readme/arm-d1]: sabotagem não detectada; resultado: $d1_result" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Arm D2: sabotagem — corpo neutralizado com body = b""
#   Expectativa: wheel produzida (exit 0), asserção (b) detecta corpo vazio
# ---------------------------------------------------------------------------
ARM_D2_SCRIPT="$WORK/arm-d2-build_wheel.py"
ARM_D2_OUT="$WORK/arm-d2-wheels"
mkdir -p "$ARM_D2_OUT"

# Gera cópia sabotada: neutraliza a linha "body = readme_bytes" → "body = b"""""
# A linha em build_wheel.py tem exatamente esta forma (4 espaços de indentação):
#   "    body = readme_bytes  # SABOTAGE-D2: ..."
sed 's/^    body = readme_bytes.*/    body = b""/' "$BUILD_WHEEL" > "$ARM_D2_SCRIPT"

# Verifica que a cópia sabotada difere do original
if cmp -s "$BUILD_WHEEL" "$ARM_D2_SCRIPT"; then
    echo "FAIL [wheel-metadata-readme/arm-d2]: sed não produziu diferença na cópia sabotada" >&2
    exit 1
fi

# A cópia sabotada deve construir a wheel com sucesso
arm_d2_build_rc=0
python3 "$ARM_D2_SCRIPT" \
    --binary  "$FAKEBIN" \
    --version "11.1.0" \
    --platform "manylinux_2_17_x86_64" \
    --output  "$ARM_D2_OUT" \
    --readme  "$README_SRC" \
    > /dev/null 2>&1 || arm_d2_build_rc=$?
if [ "$arm_d2_build_rc" -ne 0 ]; then
    echo "FAIL [wheel-metadata-readme/arm-d2]: script sabotado falhou na construção (rc=$arm_d2_build_rc)" >&2
    exit 1
fi
D2_WHL=$(ls "$ARM_D2_OUT"/*.whl 2>/dev/null | head -1)
if [ -z "$D2_WHL" ]; then
    echo "FAIL [wheel-metadata-readme/arm-d2]: script sabotado não produziu .whl" >&2
    exit 1
fi

# Verifica que a asserção (b) falha para esta wheel
d2_result=$(python3 - "$D2_WHL" "$README_SRC" <<'PYEOF' | strip_cr
import sys, zipfile

whl_path = sys.argv[1]
readme_path = sys.argv[2]
with open(readme_path, "rb") as fh:
    expected_body = fh.read()
with zipfile.ZipFile(whl_path) as zf:
    meta_paths = [n for n in zf.namelist() if n.endswith("/METADATA") and ".dist-info/" in n]
    if not meta_paths:
        print("INDETERMINATE: no METADATA")
        sys.exit(0)
    raw = zf.read(meta_paths[0])
    body = raw.split(b"\n\n", 1)[1] if b"\n\n" in raw else b""
    if body != expected_body:
        print(
            f"FAIL-AS-EXPECTED: corpo diverge "
            f"(esperado {len(expected_body)} bytes, obtido {len(body)} bytes)"
        )
    else:
        print("PASS-UNEXPECTED: corpo coincide — sabotagem não funcionou")
PYEOF
)
if echo "$d2_result" | grep -q "FAIL-AS-EXPECTED"; then
    echo "OK   [wheel-metadata-readme/arm-d2]: sabotagem (corpo neutralizado) detectada pela asserção (b)"
else
    echo "FAIL [wheel-metadata-readme/arm-d2]: sabotagem não detectada; resultado: $d2_result" >&2
    exit 1
fi

echo "OK   [wheel-metadata-readme]: todos os cenários passaram (normal + C + C2 + D1 + D2)"
