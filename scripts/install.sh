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
