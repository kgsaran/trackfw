#!/usr/bin/env sh
set -e

REPO="kgsaran/trackfw"
BIN="trackfw"
INSTALL_DIR="${TRACKFW_INSTALL_DIR:-/usr/local/bin}"

# --- Detectar OS ---
RAW_OS=$(uname -s)
case "$RAW_OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)
    echo "Sistema operacional nao suportado: $RAW_OS" >&2
    echo "Plataformas suportadas: macOS (Darwin), Linux" >&2
    exit 1
    ;;
esac

# --- Detectar ARCH ---
RAW_ARCH=$(uname -m)
case "$RAW_ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)
    echo "Arquitetura nao suportada: $RAW_ARCH" >&2
    echo "Arquiteturas suportadas: x86_64 (amd64), aarch64/arm64" >&2
    exit 1
    ;;
esac

# --- Honrar TRACKFW_VERSION, se definida (pin explicito) ---
# Se ausente ou vazia, o fluxo abaixo (resolucao via API) fica intocado.
#
# A validacao usa `case`, NUNCA `grep -E`: `case` ancora nas duas pontas do
# BUFFER inteiro do parametro do shell; `grep -E '^...$'` ancora por LINHA.
# Um valor com newline embutido e conteudo depois dela (ex.: "v7.3.0\nFOO")
# casaria a primeira linha isolada com `grep -qE` e o valor completo (com a
# segunda linha) seguiria adiante sem ser truncado nem rejeitado. Ver
# vault/notes/bash-grep-F-embedded-newline-vacuous-match-2026-08-16.md para a
# mesma familia de bug (ali era o PADRAO de grep -F com \n; aqui seria o DADO
# de entrada de grep -E com \n — mecanismo diferente, mesma causa raiz: as
# ancoras de grep sao por linha, nao por buffer).
#
# VERSION entra em duas interpolacoes depois deste bloco: URL (linha do
# download) e FILENAME (via VERSION_BARE), que por sua vez alimenta o `-o` do
# curl — o alvo real de um path traversal nao e a URL remota (o GitHub
# normaliza), e sim esse `-o`, que grava em disco sob controle do valor.
VERSION=""
if [ -n "${TRACKFW_VERSION:-}" ]; then
  _tv_raw="$TRACKFW_VERSION"
  case "$_tv_raw" in
    v*) _tv_body="${_tv_raw#v}" ;;
    *)  _tv_body="$_tv_raw" ;;
  esac
  _tv_valid=1
  case "$_tv_body" in
    *[!0-9.]*|.*|*.|*..*|"")
      _tv_valid=0
      ;;
  esac
  if [ "$_tv_valid" = "1" ]; then
    _tv_dots=$(printf '%s' "$_tv_body" | tr -cd '.' | wc -c | tr -d ' ')
    [ "$_tv_dots" = "2" ] || _tv_valid=0
  fi
  if [ "$_tv_valid" != "1" ]; then
    echo "Erro: TRACKFW_VERSION invalida: '${_tv_raw}'" >&2
    echo "Formato esperado: v?MAJOR.MINOR.PATCH (ex.: 7.3.0 ou v7.3.0)" >&2
    exit 1
  fi
  VERSION="v${_tv_body}"
fi

# --- Obter versao mais recente via API do GitHub (pulado se ja pinada acima) ---
if [ -z "$VERSION" ]; then
  if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"([^"]+)".*/\1/')
  elif command -v wget >/dev/null 2>&1; then
    VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"([^"]+)".*/\1/')
  else
    echo "Erro: curl ou wget sao necessarios para a instalacao." >&2
    exit 1
  fi
fi

if [ -z "$VERSION" ]; then
  echo "Erro: nao foi possivel determinar a versao mais recente." >&2
  exit 1
fi

# Remover prefixo 'v' para o nome do arquivo (GoReleaser usa a versao sem 'v' no nome do tar)
VERSION_BARE="${VERSION#v}"

FILENAME="${BIN}_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
TMP_DIR=$(mktemp -d)
# Garantir limpeza do diretorio temporario em qualquer saida (normal ou por erro).
# Signal 0 equivale a EXIT em sh POSIX.
trap 'rm -rf "${TMP_DIR}"' 0

echo "Instalando trackfw ${VERSION} (${OS}/${ARCH})..."
echo "URL: ${URL}"

# --- Seam de teste: imprime URL/destino e sai antes de qualquer rede ---
# Usado pelo gate scripts/check-install-version-pin.sh para nunca disparar
# download real. O destino impresso e exatamente o argumento do `-o` do
# curl abaixo (o alvo real de um path traversal via VERSION_BARE/FILENAME).
if [ -n "${TRACKFW_INSTALL_DRYRUN:-}" ]; then
  echo "DEST: ${TMP_DIR}/${FILENAME}"
  rm -rf "${TMP_DIR}"
  exit 0
fi

# --- Download ---
if command -v curl >/dev/null 2>&1; then
  curl -sSfL "${URL}" -o "${TMP_DIR}/${FILENAME}"
else
  wget -qO "${TMP_DIR}/${FILENAME}" "${URL}"
fi

# --- Verificar checksum SHA-256 (antes de extrair) ---
#
# O checksums.txt e publicado pelo GoReleaser na mesma tag do release.
# Baixar, localizar a entrada exata de FILENAME (match por igualdade de campo,
# nunca substring — ver comentario das linhas 35-43 sobre ancoragem de grep)
# e comparar com o hash calculado localmente antes de qualquer extracao.
#
# Tres estados de falha, cada um com mensagem propria:
#   ausente   — FILENAME nao encontrado em checksums.txt
#   duplicado — FILENAME aparece mais de uma vez (arquivo corrompido/adulterado)
#   divergente— hash calculado nao bate o hash publicado
#
# Ferramenta de hash: sha256sum (Linux/GNU) ou shasum -a 256 (macOS/BSD).
# Ausencia das duas e falha fechada — "nao consegui verificar" nao e
# "verificado" e nunca pode ser tratado como tal.

CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
CHECKSUMS_FILE="${TMP_DIR}/checksums.txt"

# Baixar checksums.txt da mesma tag
if command -v curl >/dev/null 2>&1; then
  if ! curl -sSfL "${CHECKSUMS_URL}" -o "${CHECKSUMS_FILE}"; then
    echo "Erro: nao foi possivel baixar checksums.txt de ${CHECKSUMS_URL}" >&2
    echo "Instalacao abortada — verificacao de integridade impossivel." >&2
    exit 1
  fi
elif command -v wget >/dev/null 2>&1; then
  if ! wget -qO "${CHECKSUMS_FILE}" "${CHECKSUMS_URL}"; then
    echo "Erro: nao foi possivel baixar checksums.txt de ${CHECKSUMS_URL}" >&2
    echo "Instalacao abortada — verificacao de integridade impossivel." >&2
    exit 1
  fi
fi

# Detectar ferramenta de hash — sha256sum (Linux/GNU) ou shasum (macOS/BSD)
SHA256_CMD=""
if command -v sha256sum >/dev/null 2>&1; then
  SHA256_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA256_CMD="shasum"
else
  echo "Erro: nem sha256sum nem shasum encontrados neste sistema." >&2
  echo "Instalacao abortada — verificacao de checksum nao pode ser realizada." >&2
  exit 1
fi

# Parsear checksums.txt: contar entradas exatas para FILENAME e capturar o hash.
# Formato goreleaser: "<hash>  <filename>" (dois espacos como separador).
# Usa awk para separar campos — nao usa grep para evitar match de substrings
# (ex: 'trackfw_1.0.0_linux_amd64.tar.gz' nao deve casar 'trackfw_1.0.0_linux_amd64.tar.gz.sig').
EXPECTED_HASH=""
MATCH_COUNT=0
while IFS= read -r CKLINE || [ -n "$CKLINE" ]; do
  CKLINE_HASH=$(printf '%s' "$CKLINE" | awk '{print $1}')
  CKLINE_FILE=$(printf '%s' "$CKLINE" | awk '{print $2}')
  if [ "$CKLINE_FILE" = "$FILENAME" ]; then
    MATCH_COUNT=$((MATCH_COUNT + 1))
    EXPECTED_HASH="$CKLINE_HASH"
  fi
done < "${CHECKSUMS_FILE}"

if [ "$MATCH_COUNT" -eq 0 ]; then
  echo "Erro: checksum ausente — '${FILENAME}' nao encontrado em checksums.txt." >&2
  echo "Instalacao abortada." >&2
  exit 1
elif [ "$MATCH_COUNT" -gt 1 ]; then
  echo "Erro: checksum duplicado — '${FILENAME}' aparece ${MATCH_COUNT} vezes em checksums.txt." >&2
  echo "Instalacao abortada." >&2
  exit 1
fi

# Calcular hash do tarball baixado e comparar
if [ "$SHA256_CMD" = "sha256sum" ]; then
  ACTUAL_HASH=$(sha256sum "${TMP_DIR}/${FILENAME}" | awk '{print $1}')
else
  ACTUAL_HASH=$(shasum -a 256 "${TMP_DIR}/${FILENAME}" | awk '{print $1}')
fi

if [ "$ACTUAL_HASH" != "$EXPECTED_HASH" ]; then
  echo "Erro: checksum divergente — o tarball recebido nao corresponde ao hash publicado." >&2
  echo "  Esperado:  ${EXPECTED_HASH}" >&2
  echo "  Calculado: ${ACTUAL_HASH}" >&2
  echo "Instalacao abortada — o tarball pode estar corrompido ou adulterado." >&2
  exit 1
fi

echo "Checksum OK: ${ACTUAL_HASH}"

# --- Extrair ---
tar -xzf "${TMP_DIR}/${FILENAME}" -C "${TMP_DIR}"

# --- Instalar (idempotente: sobrescreve binario existente) ---
if [ ! -w "${INSTALL_DIR}" ]; then
  echo "Permissao negada em ${INSTALL_DIR}. Tentando com sudo..."
  sudo mv "${TMP_DIR}/${BIN}" "${INSTALL_DIR}/${BIN}"
  sudo chmod +x "${INSTALL_DIR}/${BIN}"
else
  mv "${TMP_DIR}/${BIN}" "${INSTALL_DIR}/${BIN}"
  chmod +x "${INSTALL_DIR}/${BIN}"
fi

# --- Limpeza --- (realizada automaticamente pelo trap EXIT configurado acima)

# --- Verificar PATH ---
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    ;;
  *)
    echo ""
    echo "Atencao: ${INSTALL_DIR} nao esta no seu PATH."
    echo "Adicione ao seu shell profile:"
    echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
    ;;
esac

# --- Sucesso ---
echo ""
echo "trackfw ${VERSION} instalado com sucesso em ${INSTALL_DIR}/${BIN}"
"${INSTALL_DIR}/${BIN}" --version
