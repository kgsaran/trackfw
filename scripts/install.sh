#!/usr/bin/env sh
set -e

REPO="kgsaran/trackfw"
BIN="trackfw"
INSTALL_DIR="/usr/local/bin"

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

# --- Obter versao mais recente via API do GitHub ---
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

if [ -z "$VERSION" ]; then
  echo "Erro: nao foi possivel determinar a versao mais recente." >&2
  exit 1
fi

# Remover prefixo 'v' para o nome do arquivo (GoReleaser usa a versao sem 'v' no nome do tar)
VERSION_BARE="${VERSION#v}"

FILENAME="${BIN}_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
TMP_DIR=$(mktemp -d)

echo "Instalando trackfw ${VERSION} (${OS}/${ARCH})..."
echo "URL: ${URL}"

# --- Download ---
if command -v curl >/dev/null 2>&1; then
  curl -sSfL "${URL}" -o "${TMP_DIR}/${FILENAME}"
else
  wget -qO "${TMP_DIR}/${FILENAME}" "${URL}"
fi

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

# --- Limpeza ---
rm -rf "${TMP_DIR}"

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
