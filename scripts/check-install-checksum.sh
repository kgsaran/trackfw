#!/usr/bin/env bash
# check-install-checksum.sh — Falsifica AC1-AC6 da
# REQ-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica
#
# Sete cenarios exercitados, com frase de reconciliacao (CLAUDE.md Regra Dura de Reconciliacao):
#
#   C1 — tarball integro + checksums.txt valido → instala com sucesso (AC4 contra-braco)
#         Afirma: verificador integro nao bloqueia instalacao correta.
#         Afirma (AC1 sub): URL de checksums.txt contem a mesma tag que o tarball.
#
#   C2 — tarball com 1 byte trocado + checksums.txt do original → falha, SEM binario (AC3)
#         Afirma: adulteracao do tarball produz falha antes da extracao; ausencia provada pela
#         inexistencia do arquivo no destino, nao pelo exit code.
#
#   C3 — checksums.txt sem entrada para FILENAME → falha com mensagem "ausente" (AC2 est. 1)
#         Afirma: quando FILENAME nao esta em checksums.txt a mensagem contem "ausente".
#
#   C4 — checksums.txt com entrada duplicada → falha com mensagem "duplicado" (AC2 est. 2)
#         Afirma: quando FILENAME aparece mais de uma vez a mensagem contem "duplicado".
#
#   C5 — checksums.txt com hash errado → falha com mensagem "divergente" (AC2 est. 3)
#         Afirma: quando o hash calculado nao bate o publicado a mensagem contem "divergente".
#
#   C6 — PATH sem sha256sum e sem shasum → falha fechada (AC5)
#         Afirma: ausencia de ferramenta de hash produz falha explicita com mensagem
#         "nem sha256sum nem shasum"; binario nao e instalado.
#
#   C7 — PATH sem curl, somente wget → instala normalmente (rama wget do install.sh)
#         Afirma: o caminho wget do install.sh funciona corretamente quando curl esta ausente.
#
# Este gate e em si o AC6 da REQ: reprova quando verificacao e removida do install.sh.
# Secao "AC6 falsificacao do gate": remove o bloco de verificacao do install.sh em copia
# temporaria e confirma que tarball substituido e instalado (binario reporta "MALICIOUS"),
# provando que o gate nao e vacuo. Inclui guarda de strip: copia stripped deve NAO conter
# "Checksum OK" e DEVE conter "tar -xzf" — evita falso-OK por truncamento de sed.
#
# Hermeticidade: nenhum cenario dispara rede real. curl e wget sao substituidos por stubs
# que servem arquivos de um diretorio de fixtures local. Os stubs usam #!/bin/sh para
# funcionar com PATH restrito (sem dependencia do bash no PATH). Os stubs registram
# URLs em $STUB_URL_LOG quando definido — usado para verificar AC1 "da mesma tag".

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

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SH="$ROOT/scripts/install.sh"

WORK=$(mktemp -d)
# pwd -P: evita divergencia /var vs /private/var no macOS
WORK="$(cd "$WORK" && pwd -P)"
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Detectar ferramenta de hash disponivel no sistema (para criar fixtures)
# ---------------------------------------------------------------------------
hash_file() {
  local f="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$f" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$f" | awk '{print $1}'
  else
    echo "FATAL: nem sha256sum nem shasum disponiveis para criar fixtures" >&2
    exit 2
  fi
}

# ---------------------------------------------------------------------------
# Determinar OS/ARCH (idem install.sh)
# ---------------------------------------------------------------------------
case "$(uname -s)" in
  Darwin) T_OS="darwin" ;;
  *)      T_OS="linux"  ;;
esac
case "$(uname -m)" in
  x86_64)        T_ARCH="amd64" ;;
  aarch64|arm64) T_ARCH="arm64" ;;
  *)             T_ARCH="amd64" ;;
esac
VERSION="7.3.0"
FILENAME="trackfw_${VERSION}_${T_OS}_${T_ARCH}.tar.gz"

# ---------------------------------------------------------------------------
# Criar fake binary (responde a qualquer invocacao, incluindo --version)
# ---------------------------------------------------------------------------
printf '#!/bin/sh\necho "trackfw v0.0.1-test"\n' > "$WORK/trackfw"
chmod +x "$WORK/trackfw"

# ---------------------------------------------------------------------------
# Criar fixtures
# ---------------------------------------------------------------------------
FIXTURES="$WORK/fixtures"
mkdir -p "$FIXTURES"

# Tarball valido (contem 'trackfw' na raiz, como goreleaser produziria)
tar -czf "$FIXTURES/$FILENAME" -C "$WORK" "trackfw"

# Hash real do tarball valido
REAL_HASH=$(hash_file "$FIXTURES/$FILENAME")

# checksums.txt valido (formato goreleaser: "<hash>  <filename>")
printf '%s  %s\n' "$REAL_HASH"       "$FILENAME"                                  > "$FIXTURES/checksums.txt"
printf '%s  %s\n' "deadbeefcafe0000" "trackfw_${VERSION}_linux_arm64.tar.gz"      >> "$FIXTURES/checksums.txt"

# Cenario C2: tarball adulterado (1 byte trocado), checksums aponta para hash original.
mkdir -p "$WORK/scenarios/tampered"
TAMPERED="$WORK/scenarios/tampered/$FILENAME"
cp "$FIXTURES/$FILENAME" "$TAMPERED"

# Offset derivado do tamanho real do arquivo. O tamanho do .tar.gz depende da versao do
# tar/gzip e do nome do arquivo — o indice fixo anterior (200) estourava IndexError no
# Linux do CI, onde o tarball da fixture e menor que no macOS. A fixture presumia um
# tamanho que ela nao controla.
# Falha fechada se pequeno demais: "nao consegui adulterar" != "adulterei".
# Garantia de nao-vacuidade e a assercao de hash abaixo, nao um raciocinio sobre
# onde o byte caiu na estrutura gzip.
TAMPER_HASH_BEFORE=$(hash_file "$TAMPERED")
python3 -c '
import sys
p = sys.argv[1]
data = bytearray(open(p, "rb").read())
if len(data) < 32:
    sys.stderr.write("FATAL [C2/setup]: tarball de fixture com %d bytes -- pequeno demais para adulterar\n" % len(data))
    sys.exit(2)
idx = len(data) // 2
data[idx] ^= 0xFF
open(p, "wb").write(data)
sys.stderr.write("C2/setup: byte %d de %d invertido\n" % (idx, len(data)))
' "$TAMPERED" || { echo "FATAL [C2/setup]: adulteracao do tarball falhou" >&2; exit 2; }

TAMPER_HASH_AFTER=$(hash_file "$TAMPERED")
if [ "$TAMPER_HASH_BEFORE" = "$TAMPER_HASH_AFTER" ]; then
  echo "FATAL [C2/setup]: adulteracao nao alterou o hash -- C2 seria vacuo" >&2
  exit 2
fi
echo "C2/nao-vacuidade: hash antes=${TAMPER_HASH_BEFORE}  depois=${TAMPER_HASH_AFTER}  (diferem)"
cp "$FIXTURES/checksums.txt" "$WORK/scenarios/tampered/checksums.txt"

# Cenario AC6/falsificacao: tarball VALIDO estruturalmente mas conteudo diferente.
printf '#!/bin/sh\necho "trackfw v0.0.1-MALICIOUS"\n' > "$WORK/trackfw-evil"
chmod +x "$WORK/trackfw-evil"
mkdir -p "$WORK/scenarios/substituted"
cp "$WORK/trackfw-evil" "$WORK/trackfw"
tar -czf "$WORK/scenarios/substituted/$FILENAME" -C "$WORK" "trackfw"
# Restaurar binario original
printf '#!/bin/sh\necho "trackfw v0.0.1-test"\n' > "$WORK/trackfw"
chmod +x "$WORK/trackfw"
# checksums.txt aponta para hash do tarball original (nao do substituto)
cp "$FIXTURES/checksums.txt" "$WORK/scenarios/substituted/checksums.txt"

# Cenario C3: checksums.txt sem entrada para FILENAME (ausente)
mkdir -p "$WORK/scenarios/absent"
cp "$FIXTURES/$FILENAME" "$WORK/scenarios/absent/$FILENAME"
printf '%s  %s\n' "$REAL_HASH" "trackfw_${VERSION}_windows_amd64.zip" > "$WORK/scenarios/absent/checksums.txt"

# Cenario C4: checksums.txt com entrada duplicada
mkdir -p "$WORK/scenarios/duplicate"
cp "$FIXTURES/$FILENAME" "$WORK/scenarios/duplicate/$FILENAME"
printf '%s  %s\n' "$REAL_HASH" "$FILENAME" > "$WORK/scenarios/duplicate/checksums.txt"
printf '%s  %s\n' "$REAL_HASH" "$FILENAME" >> "$WORK/scenarios/duplicate/checksums.txt"

# Cenario C5: checksums.txt com hash errado (divergente)
mkdir -p "$WORK/scenarios/divergent"
cp "$FIXTURES/$FILENAME" "$WORK/scenarios/divergent/$FILENAME"
printf '%s  %s\n' "0000000000000000000000000000000000000000000000000000000000000000" "$FILENAME" > "$WORK/scenarios/divergent/checksums.txt"

# ---------------------------------------------------------------------------
# Stubs de curl e wget em POSIX sh (sem dependencia de bash no PATH)
#
# Quando $STUB_URL_LOG esta definido, registram cada URL — usado por C1 para
# verificar que checksums.txt vem da mesma tag (AC1 "da mesma tag").
# Forma curl: -sSfL <url> -o <dest>
# Forma wget: -qO <dest> <url>
# ---------------------------------------------------------------------------
STUB_BIN="$WORK/stubbin"
mkdir -p "$STUB_BIN"

cat > "$STUB_BIN/curl" << 'CURLEOF'
#!/bin/sh
URL=""
DEST=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o)
      shift
      DEST="$1"
      ;;
    https://*|http://*)
      URL="$1"
      ;;
    *) ;;
  esac
  shift
done
if [ -n "${STUB_URL_LOG:-}" ] && [ -n "$URL" ]; then
  echo "$URL" >> "${STUB_URL_LOG}"
fi
BNAME="${URL##*/}"
SRC="${STUB_SERVE_DIR}/${BNAME}"
if [ -z "$DEST" ]; then
  if [ -f "$SRC" ]; then cat "$SRC"; else echo "stub-curl: nao encontrado: $SRC" >&2; exit 22; fi
else
  if [ -f "$SRC" ]; then cp "$SRC" "$DEST"; else echo "stub-curl: nao encontrado: $SRC" >&2; exit 22; fi
fi
CURLEOF
chmod +x "$STUB_BIN/curl"

cat > "$STUB_BIN/wget" << 'WGETEOF'
#!/bin/sh
URL=""
DEST=""
while [ $# -gt 0 ]; do
  case "$1" in
    -O|-qO)
      shift
      DEST="$1"
      ;;
    https://*|http://*)
      URL="$1"
      ;;
    *) ;;
  esac
  shift
done
if [ -n "${STUB_URL_LOG:-}" ] && [ -n "$URL" ]; then
  echo "$URL" >> "${STUB_URL_LOG}"
fi
BNAME="${URL##*/}"
SRC="${STUB_SERVE_DIR}/${BNAME}"
if [ "$DEST" = "-" ] || [ -z "$DEST" ]; then
  if [ -f "$SRC" ]; then cat "$SRC"; else echo "stub-wget: nao encontrado: $SRC" >&2; exit 1; fi
else
  if [ -f "$SRC" ]; then cp "$SRC" "$DEST"; else echo "stub-wget: nao encontrado: $SRC" >&2; exit 1; fi
fi
WGETEOF
chmod +x "$STUB_BIN/wget"

# ---------------------------------------------------------------------------
# Helpers de cenario
# ---------------------------------------------------------------------------
SCENARIOS_RUN=0
GATE_FAIL=0
INSTALL_DIR_TMP="$WORK/installdir"
mkdir -p "$INSTALL_DIR_TMP"
OUT=""
RC=0
URL_LOG="$WORK/last-url-log.txt"

run_install() {
  # run_install <serve_dir> [extra_env=val ...]
  local serve_dir="$1"; shift
  rm -f "$INSTALL_DIR_TMP/trackfw"
  rm -f "$URL_LOG"
  set +e
  OUT=$(env \
    TRACKFW_VERSION="v${VERSION}" \
    TRACKFW_INSTALL_DIR="$INSTALL_DIR_TMP" \
    STUB_SERVE_DIR="$serve_dir" \
    STUB_URL_LOG="$URL_LOG" \
    PATH="$STUB_BIN:$PATH" \
    "$@" \
    sh "$INSTALL_SH" 2>&1)
  RC=$?
  set -e
}

assert_pass() {
  local label="$1"
  if [ "$RC" -ne 0 ]; then
    echo "FAIL [$label]: esperava exit 0, saiu com $RC"
    echo "  output: $OUT"
    GATE_FAIL=1; return
  fi
  echo "OK   [$label]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
}

assert_fail_with() {
  local label="$1" pattern="$2"
  if [ "$RC" -eq 0 ]; then
    echo "FAIL [$label]: esperava falha (exit != 0), saiu com 0"
    echo "  output: $OUT"
    GATE_FAIL=1; return
  fi
  if ! echo "$OUT" | grep -qi "$pattern"; then
    echo "FAIL [$label]: saiu com $RC mas mensagem nao contem '$pattern'"
    echo "  output: $OUT"
    GATE_FAIL=1; return
  fi
  echo "OK   [$label]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
}

assert_no_binary() {
  local label="$1"
  if [ -f "$INSTALL_DIR_TMP/trackfw" ]; then
    echo "FAIL [$label]: binario presente em $INSTALL_DIR_TMP/trackfw — deveria estar ausente"
    GATE_FAIL=1; return
  fi
  echo "OK   [$label] (ausencia do binario confirmada)"
}

assert_binary_present() {
  local label="$1"
  if [ ! -f "$INSTALL_DIR_TMP/trackfw" ]; then
    echo "FAIL [$label]: binario ausente em $INSTALL_DIR_TMP/trackfw — deveria ter sido instalado"
    GATE_FAIL=1; return
  fi
  echo "OK   [$label] (presenca do binario confirmada)"
}

# ---------------------------------------------------------------------------
# C1 — tarball integro instala normalmente (AC4 contra-braco)
# Reconciliacao: afirma que verificador integro nao bloqueia instalacao correta.
# Reconciliacao (AC1 sub): afirma que a URL de checksums.txt contem a tag correta.
# ---------------------------------------------------------------------------
run_install "$FIXTURES"
assert_pass           "C1/integro-instala"
assert_binary_present "C1/binario-presente"

# AC1 "da mesma tag": URL de checksums.txt deve conter /v<VERSION>/
if ! grep -q "/v${VERSION}/" "$URL_LOG" 2>/dev/null; then
  echo "FAIL [C1/ac1-mesma-tag]: URL de checksums.txt nao contem /v${VERSION}/"
  echo "  URLs registradas: $(cat "$URL_LOG" 2>/dev/null || echo '(log vazio)')"
  GATE_FAIL=1
else
  echo "OK   [C1/ac1-mesma-tag]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
fi

# ---------------------------------------------------------------------------
# C2 — 1 byte adulterado → falha + ausencia do binario (AC3)
# Reconciliacao: afirma que adulteracao do tarball impede instalacao; ausencia
# do binario no destino prova que nenhum artefato foi extraido.
# ---------------------------------------------------------------------------
run_install "$WORK/scenarios/tampered"
assert_fail_with "C2/adulterado-falha"      "divergente"
assert_no_binary "C2/adulterado-sem-binario"

# ---------------------------------------------------------------------------
# C3 — checksums.txt sem entrada para FILENAME (AC2 estado 1)
# Reconciliacao: afirma que FILENAME ausente em checksums.txt produz mensagem "ausente".
# ---------------------------------------------------------------------------
run_install "$WORK/scenarios/absent"
assert_fail_with "C3/checksum-ausente"  "ausente"
assert_no_binary "C3/ausente-sem-binario"

# ---------------------------------------------------------------------------
# C4 — entrada duplicada para FILENAME (AC2 estado 2)
# Reconciliacao: afirma que entrada duplicada produz mensagem "duplicado".
# ---------------------------------------------------------------------------
run_install "$WORK/scenarios/duplicate"
assert_fail_with "C4/checksum-duplicado"  "duplicado"
assert_no_binary "C4/duplicado-sem-binario"

# ---------------------------------------------------------------------------
# C5 — hash errado em checksums.txt (AC2 estado 3)
# Reconciliacao: afirma que hash errado produz mensagem "divergente".
# ---------------------------------------------------------------------------
run_install "$WORK/scenarios/divergent"
assert_fail_with "C5/hash-divergente"  "divergente"
assert_no_binary "C5/divergente-sem-binario"

# ---------------------------------------------------------------------------
# C6 — sem sha256sum e sem shasum no PATH → falha fechada (AC5)
#
# PATH minimo com utilitarios essenciais mas SEM sha256sum/shasum.
# Os stubs curl/wget usam #!/bin/sh (funciona sem bash no PATH).
# Reconciliacao: afirma que ausencia de ferramenta de hash produz falha
# com mensagem exata "nem sha256sum nem shasum"; binario nao e instalado.
# ---------------------------------------------------------------------------
MINPATH_DIR="$WORK/minpath"
mkdir -p "$MINPATH_DIR"
ln -sf "$STUB_BIN/curl" "$MINPATH_DIR/curl"
ln -sf "$STUB_BIN/wget" "$MINPATH_DIR/wget"

for util in sh env awk sed uname tr wc mktemp mv chmod tar cp rm printf python3; do
  bin_path="$(command -v "$util" 2>/dev/null)" || true
  if [ -z "$bin_path" ]; then
    echo "FAIL [C6/setup]: utilitario essencial ausente no sistema: $util" >&2
    GATE_FAIL=1
    continue
  fi
  if [ "$(basename "$bin_path")" != "sha256sum" ] && [ "$(basename "$bin_path")" != "shasum" ]; then
    ln -sf "$bin_path" "$MINPATH_DIR/$(basename "$bin_path")" 2>/dev/null || true
  fi
done

if [ -e "$MINPATH_DIR/sha256sum" ] || [ -e "$MINPATH_DIR/shasum" ]; then
  echo "FAIL [C6/setup]: minpath contem sha256sum ou shasum — cenario invalido" >&2
  GATE_FAIL=1
fi

rm -f "$INSTALL_DIR_TMP/trackfw"
set +e
OUT_C6=$(env \
  TRACKFW_VERSION="v${VERSION}" \
  TRACKFW_INSTALL_DIR="$INSTALL_DIR_TMP" \
  STUB_SERVE_DIR="$FIXTURES" \
  PATH="$MINPATH_DIR" \
  sh "$INSTALL_SH" 2>&1)
RC_C6=$?
set -e

if [ "$RC_C6" -eq 0 ]; then
  echo "FAIL [C6/sem-ferramenta-hash]: esperava falha, saiu com 0"
  echo "  output: $OUT_C6"
  GATE_FAIL=1
elif ! echo "$OUT_C6" | grep -qF "nem sha256sum nem shasum"; then
  echo "FAIL [C6/sem-ferramenta-hash]: saiu com $RC_C6 mas mensagem nao contem 'nem sha256sum nem shasum'"
  echo "  output: $OUT_C6"
  GATE_FAIL=1
else
  echo "OK   [C6/sem-ferramenta-hash]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
fi

if [ ! -f "$INSTALL_DIR_TMP/trackfw" ]; then
  echo "OK   [C6/sem-ferramenta-sem-binario] (ausencia do binario confirmada)"
else
  echo "FAIL [C6/sem-ferramenta-sem-binario]: binario presente apesar de falha"
  GATE_FAIL=1
fi

# ---------------------------------------------------------------------------
# C7 — PATH sem curl, somente wget → instala normalmente
#
# Exercita o caminho elif de install.sh (wget) que nunca executa quando curl
# esta presente no PATH. O WGET_ONLY_BIN contem o stub wget, utilitarios
# essenciais (inclusive gzip — necessario porque GNU tar faz fork de gzip para
# descomprimir .gz no Linux) e exclui curl explicitamente.
# Reconciliacao: afirma que o caminho wget funciona corretamente quando curl ausente.
# Guarda de vacuidade: utilitario essencial ausente no sistema nomeia o culpado
# em vez de montar PATH incompleto e reprovar com atribuicao errada.
# gunzip: ligado se existir como binario separado; opcional (pode ser wrapper/
# link de gzip em algumas distribuicoes — ausencia nao e defeito do cenario).
# ---------------------------------------------------------------------------
WGET_ONLY_BIN="$WORK/wget-only-bin"
mkdir -p "$WGET_ONLY_BIN"
ln -sf "$STUB_BIN/wget" "$WGET_ONLY_BIN/wget"

for util in sh env awk sed uname tr wc mktemp mv chmod tar cp rm printf grep gzip; do
  _p="$(command -v "$util" 2>/dev/null)" || true
  if [ -z "$_p" ]; then
    echo "FAIL [C7/setup]: utilitario essencial ausente no sistema: $util" >&2
    GATE_FAIL=1
    continue
  fi
  ln -sf "$_p" "$WGET_ONLY_BIN/$(basename "$_p")" 2>/dev/null || true
done
_gunzip_p="$(command -v gunzip 2>/dev/null)" || true
[ -n "$_gunzip_p" ] && ln -sf "$_gunzip_p" "$WGET_ONLY_BIN/gunzip" 2>/dev/null || true
if command -v sha256sum >/dev/null 2>&1; then
  ln -sf "$(command -v sha256sum)" "$WGET_ONLY_BIN/sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  ln -sf "$(command -v shasum)" "$WGET_ONLY_BIN/shasum"
fi

if [ -e "$WGET_ONLY_BIN/curl" ]; then
  echo "FAIL [C7/setup]: wget-only-bin contem curl — cenario invalido" >&2
  GATE_FAIL=1
fi

rm -f "$INSTALL_DIR_TMP/trackfw"
rm -f "$URL_LOG"
set +e
OUT_C7=$(env \
  TRACKFW_VERSION="v${VERSION}" \
  TRACKFW_INSTALL_DIR="$INSTALL_DIR_TMP" \
  STUB_SERVE_DIR="$FIXTURES" \
  STUB_URL_LOG="$URL_LOG" \
  PATH="$WGET_ONLY_BIN" \
  sh "$INSTALL_SH" 2>&1)
RC_C7=$?
set -e

if [ "$RC_C7" -ne 0 ]; then
  echo "FAIL [C7/wget-only-instala]: esperava exit 0, saiu com $RC_C7"
  echo "  output: $OUT_C7"
  GATE_FAIL=1
else
  echo "OK   [C7/wget-only-instala]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
fi

if [ -f "$INSTALL_DIR_TMP/trackfw" ]; then
  echo "OK   [C7/wget-only-binario] (presenca do binario confirmada)"
else
  echo "FAIL [C7/wget-only-binario]: binario ausente apos instalacao via wget"
  GATE_FAIL=1
fi

# ---------------------------------------------------------------------------
# AC6 — falsificacao do gate
#
# Remove o bloco de verificacao do install.sh em copia temporaria e confirma
# que tarball substituido (estruturalmente valido, binario diferente) e instalado
# silenciosamente — provando que o gate nao e vacuo.
#
# Guarda de strip: copia stripped deve NAO conter "Checksum OK" (bloco removido)
# e DEVE conter "tar -xzf" (resto do script intacto). Se o sed truncar ate EOF,
# "tar -xzf" desaparece e a guarda reprova antes de qualquer falso-OK.
#
# Afirmacao principal: binario instalado (sem verificacao) reporta "MALICIOUS".
# ---------------------------------------------------------------------------
INSTALL_SH_STRIPPED="$WORK/install-stripped.sh"
sed '/^# --- Verificar checksum SHA-256/,/echo "Checksum OK:.*$/d' "$INSTALL_SH" > "$INSTALL_SH_STRIPPED"
chmod +x "$INSTALL_SH_STRIPPED"

STRIP_OK=1
if grep -q "Checksum OK" "$INSTALL_SH_STRIPPED"; then
  echo "FAIL [AC6/strip-sanidade]: script stripped ainda contem 'Checksum OK' — sed nao removeu o bloco"
  GATE_FAIL=1
  STRIP_OK=0
fi
if ! grep -q "tar -xzf" "$INSTALL_SH_STRIPPED"; then
  echo "FAIL [AC6/strip-sanidade]: script stripped nao contem 'tar -xzf' — sed apagou demais (possivel truncamento ate EOF)"
  GATE_FAIL=1
  STRIP_OK=0
fi

if [ "$STRIP_OK" = "1" ]; then
  rm -f "$INSTALL_DIR_TMP/trackfw"
  set +e
  OUT_STRIPPED=$(env \
    TRACKFW_VERSION="v${VERSION}" \
    TRACKFW_INSTALL_DIR="$INSTALL_DIR_TMP" \
    STUB_SERVE_DIR="$WORK/scenarios/substituted" \
    PATH="$STUB_BIN:$PATH" \
    sh "$INSTALL_SH_STRIPPED" 2>&1)
  RC_STRIPPED=$?
  set -e

  MALICIOUS_INSTALLED=0
  if [ -f "$INSTALL_DIR_TMP/trackfw" ] && "$INSTALL_DIR_TMP/trackfw" 2>&1 | grep -q "MALICIOUS"; then
    MALICIOUS_INSTALLED=1
  fi

  if [ "$MALICIOUS_INSTALLED" = "1" ]; then
    echo "OK   [AC6/falsificacao-gate]: binario MALICIOUS instalado sem verificacao — gate e load-bearing"
    SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
  else
    echo "FAIL [AC6/falsificacao-gate]: install.sh sem verificacao deveria instalar o binario MALICIOUS"
    echo "  binario presente: $([ -f "$INSTALL_DIR_TMP/trackfw" ] && echo sim || echo nao)"
    if [ -f "$INSTALL_DIR_TMP/trackfw" ]; then
      echo "  saida do binario: $("$INSTALL_DIR_TMP/trackfw" 2>&1 || true)"
    fi
    echo "  rc stripped: $RC_STRIPPED"
    echo "  output: $OUT_STRIPPED"
    GATE_FAIL=1
  fi
fi

# ---------------------------------------------------------------------------
# Guarda de vacuidade
# ---------------------------------------------------------------------------
if [ "$SCENARIOS_RUN" -eq 0 ]; then
  echo "FAIL [vacuidade]: nenhum cenario executado" >&2
  exit 1
fi

echo ""
if [ "$GATE_FAIL" -ne 0 ]; then
  echo "check-install-checksum: FAIL — $SCENARIOS_RUN cenarios OK, um ou mais falharam"
  exit 1
fi
echo "check-install-checksum: OK — $SCENARIOS_RUN cenarios passaram"
