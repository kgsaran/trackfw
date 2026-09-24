#!/usr/bin/env bash
# check-install-version-pin.sh — falsifica o suporte de scripts/install.sh a TRACKFW_VERSION
# (REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar.md,
# AC1-AC5; roadmap ML-1A). Modelo de ameaça em
# docs/roadmaps/wip/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-
# trackfw-version.md, secao "Resultado do ML-0A", secoes 2 e 3.
#
# Vetor central: install.sh valida TRACKFW_VERSION com `case`, nao `grep -E`. `grep -E '^...$'`
# ancora por LINHA, nao pelo buffer inteiro — um valor "v7.3.0\nFOO" casaria a primeira linha
# isolada e o valor completo (com a segunda linha) seguiria para URL/FILENAME sem ser rejeitado.
# `case` ancora nas duas pontas do parametro inteiro por construcao. Ver
# vault/notes/bash-grep-F-embedded-newline-vacuous-match-2026-08-16.md (mesma familia de bug:
# ali era o PADRAO de grep -F com \n embutido; aqui seria o DADO de grep -E com \n embutido —
# mecanismo diferente, mesma causa raiz).
#
# O alvo real de um path traversal nao e a URL remota (o GitHub normaliza o path do lado do
# servidor) — e o argumento `-o` do curl de download, que grava em TMP_DIR/FILENAME. Por isso
# este gate nao le apenas a URL impressa: le tambem o DEST (destino do -o) impresso pelo seam
# TRACKFW_INSTALL_DRYRUN.
#
# Hermeticidade: nenhum cenario aqui dispara rede de verdade. Dois mecanismos independentes:
#   1. TRACKFW_INSTALL_DRYRUN=1 faz install.sh sair com 0 ANTES do curl/wget de download.
#   2. curl/wget sao interceptados por stubs no inicio do PATH — mesmo a chamada de resolucao
#      via API (releases/latest), que roda ANTES do ponto de saida do dryrun quando
#      TRACKFW_VERSION esta ausente/vazia, bate no stub, nao na rede real. O stub tambem serve
#      de prova positiva de AC1 ("pula a API quando pinada"): quando TRACKFW_VERSION é valida,
#      o log de chamadas do stub deve ficar vazio.
set -euo pipefail

# Codificacao de saida (ML-4I-bis): forca UTF-8 no stdio de todo python3 deste gate.
# Sob console cp1252 (Windows) o Python herda a codepage e um print() de caractere fora
# do cp1252 estoura UnicodeEncodeError — o gate reprova por motivo alheio ao que mede.
# Declarado aqui, e nao no Makefile, para valer em invocacoes diretas e em CI.
export PYTHONIOENCODING=utf-8

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SH="$ROOT/scripts/install.sh"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

STUB_BIN="$WORK/stubbin"
mkdir -p "$STUB_BIN"
CURL_LOG="$WORK/curl.log"
export CURL_LOG
: > "$CURL_LOG"

# Stub de curl: registra toda chamada em CURL_LOG. Se a URL for a API de releases/latest,
# responde com um tag_name valido (para o fluxo nao-pinado seguir ate o ponto de saida do
# dryrun). Qualquer outra chamada (download real) indica que o script tentou ultrapassar o
# seam de dryrun — falha ruidosa, nunca rede de verdade.
cat > "$STUB_BIN/curl" <<'EOF'
#!/usr/bin/env bash
echo "curl $*" >> "$CURL_LOG"
for a in "$@"; do
  case "$a" in
    https://api.github.com/*)
      echo '{"tag_name": "v7.3.0"}'
      exit 0
      ;;
  esac
done
echo "STUB curl: chamada inesperada fora da API de releases/latest — o dryrun deveria ter saido antes" >&2
exit 1
EOF
chmod +x "$STUB_BIN/curl"

cat > "$STUB_BIN/wget" <<'EOF'
#!/usr/bin/env bash
echo "wget $*" >> "$CURL_LOG"
echo "STUB wget: chamada inesperada — este gate so exercita o caminho via curl" >&2
exit 1
EOF
chmod +x "$STUB_BIN/wget"

OUT=""
EC=0

# run_install ENVASSIGN...  — roda install.sh com as atribuicoes de env dadas, mais
# TRACKFW_INSTALL_DRYRUN=1 e PATH apontando para os stubs. Preenche OUT/EC.
run_install() {
  : > "$CURL_LOG"
  set +e
  OUT=$(env "$@" TRACKFW_INSTALL_DRYRUN=1 PATH="$STUB_BIN:$PATH" sh "$INSTALL_SH" 2>&1)
  EC=$?
  set -e
}

# run_install_unset — mesma coisa, mas com TRACKFW_VERSION explicitamente ausente do ambiente
# (nao apenas vazia), para cobrir o caso "variavel nunca setada" (AC2) separado do caso
# "variavel setada como string vazia".
run_install_unset() {
  : > "$CURL_LOG"
  set +e
  OUT=$(env -u TRACKFW_VERSION TRACKFW_INSTALL_DRYRUN=1 PATH="$STUB_BIN:$PATH" sh "$INSTALL_SH" 2>&1)
  EC=$?
  set -e
}

SCENARIOS_RUN=0

pass_pinned() {
  # label, TRACKFW_VERSION value, expected version substring in URL (ja normalizada com 'v'),
  # expected VERSION_BARE (sem 'v') para checar o DEST — o alvo real de um path traversal
  # (o argumento `-o` do curl de download, nao a URL remota).
  local label="$1" value="$2" expect="$3" expect_bare="$4"
  run_install "TRACKFW_VERSION=$value"
  if [ "$EC" -ne 0 ]; then
    echo "FAIL [install-version-pin/$label]: esperava exit 0, saiu com $EC" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  if ! grep -qF "$expect" <<<"$OUT"; then
    echo "FAIL [install-version-pin/$label]: URL nao contem '$expect'" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  local dest
  dest=$(sed -n 's/^DEST: //p' <<<"$OUT")
  if [ -z "$dest" ]; then
    echo "FAIL [install-version-pin/$label]: nenhuma linha DEST impressa pelo seam de dryrun" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  # O alvo real do traversal e o `-o` do curl (grava DEST em disco). O basename de DEST tem
  # que ser exatamente "trackfw_<bare>_<os>_<arch>.tar.gz" — sem "/" nem ".." vazando do
  # VERSION_BARE para dentro do nome do arquivo ou para fora de TMP_DIR.
  case "$dest" in
    *..*)
      echo "FAIL [install-version-pin/$label]: DEST contem '..' — path traversal no destino do -o" >&2
      echo "  DEST: $dest" >&2
      exit 1
      ;;
  esac
  local base="${dest##*/}"
  case "$base" in
    trackfw_"${expect_bare}"_*.tar.gz) : ;;
    *)
      echo "FAIL [install-version-pin/$label]: basename de DEST nao bate 'trackfw_${expect_bare}_<os>_<arch>.tar.gz'" >&2
      echo "  DEST: $dest" >&2
      exit 1
      ;;
  esac
  if [ -s "$CURL_LOG" ]; then
    echo "FAIL [install-version-pin/$label]: AC1 violado — houve chamada a curl/wget com TRACKFW_VERSION pinada (deveria pular a API)" >&2
    echo "  curl log: $(cat "$CURL_LOG")" >&2
    exit 1
  fi
  echo "OK   [install-version-pin/$label]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
}

pass_api_resolved() {
  # label, funcao de invocacao (run_install_unset ou run_install com valor vazio)
  local label="$1"
  if [ "$EC" -ne 0 ]; then
    echo "FAIL [install-version-pin/$label]: esperava exit 0 (fluxo AC2 intocado), saiu com $EC" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  if ! grep -qF "api.github.com" "$CURL_LOG"; then
    echo "FAIL [install-version-pin/$label]: AC2 violado — TRACKFW_VERSION ausente/vazia deveria consultar a API de releases/latest e nao consultou" >&2
    echo "  curl log: $(cat "$CURL_LOG")" >&2
    exit 1
  fi
  echo "OK   [install-version-pin/$label]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
}

assert_fails_with() {
  local label="$1" pattern="$2" value="$3"
  run_install "TRACKFW_VERSION=$value"
  if [ "$EC" -eq 0 ]; then
    echo "FAIL [install-version-pin/$label]: saiu com 0, esperava != 0 para TRACKFW_VERSION='$value'" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  if ! grep -qF "$pattern" <<<"$OUT"; then
    echo "FAIL [install-version-pin/$label]: saiu com $EC mas falta diagnostico '$pattern'" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  if [ -s "$CURL_LOG" ]; then
    echo "FAIL [install-version-pin/$label]: valor invalido nao deveria ter composto URL nem chamado curl/wget" >&2
    echo "  curl log: $(cat "$CURL_LOG")" >&2
    exit 1
  fi
  if grep -qF "DEST: " <<<"$OUT"; then
    echo "FAIL [install-version-pin/$label]: valor invalido chegou a compor o alvo do -o (DEST impresso) — a rejeicao tem que ocorrer ANTES da composicao de URL/FILENAME/DEST" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  echo "OK   [install-version-pin/$label]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
}

REASON="TRACKFW_VERSION invalida"

# --- Cenarios que PASSAM ---------------------------------------------------

pass_pinned "pinned-bare"              "7.3.0"    "v7.3.0"    "7.3.0"
pass_pinned "pinned-v-prefixed"        "v7.3.0"   "v7.3.0"    "7.3.0"
pass_pinned "pinned-multi-digit-minor" "v7.30.0"  "v7.30.0"   "7.30.0"
pass_pinned "pinned-multi-digit-major" "v10.0.0"  "v10.0.0"   "10.0.0"
pass_pinned "pinned-pre-1.0-no-v"      "0.9.1"    "v0.9.1"    "0.9.1"

# AC5: "7.3.0" e "v7.3.0" tem que baixar o mesmo asset — URL E DEST byte-identicos. O DEST e
# o alvo real do -o do curl (o argumento de escrita em disco), nao so a URL remota.
# ML-2I: o rc do grep SOZINHO (sem cano) mata o script sob `set -e` — nao depende de
# pipefail. Antes desta guarda, um seam de dryrun que parasse de emitir a linha "URL: "
# matava o gate MUDO, sem rotulo nenhum. A guarda de rc abaixo tira a morte muda; a
# guarda de nao-vacuidade logo depois e obrigatoria porque, sem ela, duas URLs vazias
# se comparariam IGUAIS e o cenario emitiria "OK" sobre medicao nenhuma.
run_install "TRACKFW_VERSION=7.3.0"
URL_BARE=$({ grep '^URL: ' <<<"$OUT" || true; })
DEST_BARE=$(sed -n 's/^DEST: //p' <<<"$OUT")
run_install "TRACKFW_VERSION=v7.3.0"
URL_PREFIXED=$({ grep '^URL: ' <<<"$OUT" || true; })
DEST_PREFIXED=$(sed -n 's/^DEST: //p' <<<"$OUT")
if [ -z "$URL_BARE" ] || [ -z "$URL_PREFIXED" ]; then
  echo "FAIL [install-version-pin/ac5-same-asset]: o seam de dryrun nao emitiu linha 'URL: ' — medicao vacua, nao comparacao" >&2
  echo "  bare:      [$URL_BARE]" >&2
  echo "  prefixed:  [$URL_PREFIXED]" >&2
  echo "  output do ultimo run_install: $OUT" >&2
  exit 1
fi
if [ "$URL_BARE" != "$URL_PREFIXED" ]; then
  echo "FAIL [install-version-pin/ac5-same-asset]: AC5 violado — '7.3.0' e 'v7.3.0' compuseram URLs diferentes" >&2
  echo "  bare:      $URL_BARE" >&2
  echo "  prefixed:  $URL_PREFIXED" >&2
  exit 1
fi
# ML-2J: `sed -n` sem casar sai 0 — nao ha rc para propagar, logo a guarda de rc do ML-2I
# e inutil AQUI por construcao; o unico discriminante possivel e o CONTEUDO. Sem esta
# guarda, um seam de dryrun que parasse de emitir a linha "DEST: " deixaria as duas
# capturas VAZIAS, elas comparariam IGUAIS e o cenario emitiria "OK" sobre medicao
# nenhuma. A guarda incide sobre o BASENAME — a expressao efetivamente comparada abaixo —
# e nao sobre a captura crua: um DEST terminado em "/" tem captura nao-vazia e basename
# vazio, e e o basename que decide o veredito.
DEST_BARE_BASE="${DEST_BARE##*/}"
DEST_PREFIXED_BASE="${DEST_PREFIXED##*/}"
if [ -z "$DEST_BARE_BASE" ] || [ -z "$DEST_PREFIXED_BASE" ]; then
  echo "FAIL [install-version-pin/ac5-same-asset]: o seam de dryrun nao emitiu basename utilizavel na linha 'DEST: ' — medicao vacua, nao comparacao" >&2
  echo "  bare:      [$DEST_BARE] basename [$DEST_BARE_BASE]" >&2
  echo "  prefixed:  [$DEST_PREFIXED] basename [$DEST_PREFIXED_BASE]" >&2
  echo "  output do ultimo run_install: $OUT" >&2
  exit 1
fi
if [ "$DEST_BARE_BASE" != "$DEST_PREFIXED_BASE" ]; then
  echo "FAIL [install-version-pin/ac5-same-asset]: AC5 violado — '7.3.0' e 'v7.3.0' compuseram basenames de DEST diferentes" >&2
  echo "  bare:      $DEST_BARE" >&2
  echo "  prefixed:  $DEST_PREFIXED" >&2
  exit 1
fi
echo "OK   [install-version-pin/ac5-same-asset]"
SCENARIOS_RUN=$((SCENARIOS_RUN + 1))

run_install_unset
pass_api_resolved "unset-resolves-via-api"

run_install "TRACKFW_VERSION="
pass_api_resolved "empty-resolves-via-api"

# --- Pre-release versions (-rcN / -betaN): admitted since ML-4I (install.sh Windows support) ---
# Afirma: TRACKFW_VERSION com sufixo -rcN ou -betaN e aceito; URL e DEST usam a versao completa
# incluindo o sufixo. O VERSION_BARE resultante (ex.: 8.0.0-rc2) nao contem "/" nem ".." —
# o charset do sufixo (so digitos) garante que o alvo do -o do curl permanece dentro de TMP_DIR.
pass_pinned "pinned-prerelease-rc-bare"      "8.0.0-rc2"   "v8.0.0-rc2"   "8.0.0-rc2"
pass_pinned "pinned-prerelease-rc-v"         "v8.0.0-rc2"  "v8.0.0-rc2"   "8.0.0-rc2"
pass_pinned "pinned-prerelease-beta"         "v8.0.0-beta1" "v8.0.0-beta1" "8.0.0-beta1"

# AC5-pre: bare e prefixed com prerelease devem baixar o mesmo asset
# ML-2I: mesma causa e mesmo par de guardas do cenario ac5-same-asset acima.
run_install "TRACKFW_VERSION=8.0.0-rc2"
URL_PRE_BARE=$({ grep '^URL: ' <<<"$OUT" || true; })
DEST_PRE_BARE=$(sed -n 's/^DEST: //p' <<<"$OUT")
run_install "TRACKFW_VERSION=v8.0.0-rc2"
URL_PRE_PREFIXED=$({ grep '^URL: ' <<<"$OUT" || true; })
DEST_PRE_PREFIXED=$(sed -n 's/^DEST: //p' <<<"$OUT")
if [ -z "$URL_PRE_BARE" ] || [ -z "$URL_PRE_PREFIXED" ]; then
  echo "FAIL [install-version-pin/ac5-prerelease-same-asset]: o seam de dryrun nao emitiu linha 'URL: ' — medicao vacua, nao comparacao" >&2
  echo "  bare:      [$URL_PRE_BARE]" >&2
  echo "  prefixed:  [$URL_PRE_PREFIXED]" >&2
  echo "  output do ultimo run_install: $OUT" >&2
  exit 1
fi
if [ "$URL_PRE_BARE" != "$URL_PRE_PREFIXED" ]; then
  echo "FAIL [install-version-pin/ac5-prerelease-same-asset]: '8.0.0-rc2' e 'v8.0.0-rc2' compuseram URLs diferentes" >&2
  echo "  bare:      $URL_PRE_BARE" >&2
  echo "  prefixed:  $URL_PRE_PREFIXED" >&2
  exit 1
fi
# ML-2J: mesma causa e mesma guarda do cenario ac5-same-asset acima — o `sed -n` nao tem
# rc para propagar e dois DEST vazios comparariam iguais, emitindo "OK" vacuo.
DEST_PRE_BARE_BASE="${DEST_PRE_BARE##*/}"
DEST_PRE_PREFIXED_BASE="${DEST_PRE_PREFIXED##*/}"
if [ -z "$DEST_PRE_BARE_BASE" ] || [ -z "$DEST_PRE_PREFIXED_BASE" ]; then
  echo "FAIL [install-version-pin/ac5-prerelease-same-asset]: o seam de dryrun nao emitiu basename utilizavel na linha 'DEST: ' — medicao vacua, nao comparacao" >&2
  echo "  bare:      [$DEST_PRE_BARE] basename [$DEST_PRE_BARE_BASE]" >&2
  echo "  prefixed:  [$DEST_PRE_PREFIXED] basename [$DEST_PRE_PREFIXED_BASE]" >&2
  echo "  output do ultimo run_install: $OUT" >&2
  exit 1
fi
if [ "$DEST_PRE_BARE_BASE" != "$DEST_PRE_PREFIXED_BASE" ]; then
  echo "FAIL [install-version-pin/ac5-prerelease-same-asset]: '8.0.0-rc2' e 'v8.0.0-rc2' compuseram basenames de DEST diferentes" >&2
  echo "  bare:      $DEST_PRE_BARE" >&2
  echo "  prefixed:  $DEST_PRE_PREFIXED" >&2
  exit 1
fi
echo "OK   [install-version-pin/ac5-prerelease-same-asset]"
SCENARIOS_RUN=$((SCENARIOS_RUN + 1))

# --- Cenarios que FALHAM, com a razao declarada pelo proprio install.sh ----

assert_fails_with "command-separator-semicolon" "$REASON" '7.3.0; rm -rf /'
assert_fails_with "command-substitution-dollar" "$REASON" '$(id)'
assert_fails_with "command-substitution-backtick" "$REASON" '`id`'
assert_fails_with "command-separator-and-pipe"  "$REASON" '7.3.0 && curl x | sh'
assert_fails_with "path-traversal"              "$REASON" '../../etc'
# Traversal com prefixo numerico valido — o alvo real deste vetor nao e a URL remota (o
# GitHub normaliza o path do lado do servidor), e sim o argumento `-o` do curl de download:
# se VERSION_BARE contivesse "/", FILENAME/DEST escreveria fora de TMP_DIR. O charset de
# `case` (so digitos e ponto apos o "v" opcional) ja rejeita "/" antes de compor DEST — este
# cenario nomeia explicitamente esse alvo, em vez de so testar a forma generica "../../etc".
assert_fails_with "path-traversal-targets-dash-o-dest" "$REASON" '7.3.0/../../tmp/evil'
assert_fails_with "whitespace-only"             "$REASON" '   '

# Newline embutida COM conteudo depois dela — o vetor central deste ML (secao 2 do ML-0A).
# Um cenario so com "v7.3.0\n" (newline final sem conteudo) nao discrimina uma implementacao
# correta de uma baseada em `grep -qE` sem `-z`: ambas passariam. O conteudo pos-newline e o
# que expoe o bug de ancoragem por-linha do grep.
NEWLINE_VALUE=$(printf 'v7.3.0\nFOO')
assert_fails_with "embedded-newline-with-trailing-content" "$REASON" "$NEWLINE_VALUE"

# Pre-release invalidos — sufixos fora do formato admitido devem falhar (ML-4I).
# Afirma: apenas -rcN e -betaN sao admitidos; outras formas sao rejeitadas como antes.
assert_fails_with "prerelease-alpha-rejected"   "$REASON" 'v8.0.0-alpha1'
assert_fails_with "prerelease-dash-only"        "$REASON" 'v8.0.0-'
assert_fails_with "prerelease-rc-no-digit"      "$REASON" 'v8.0.0-rc'
assert_fails_with "prerelease-rc-with-slash"    "$REASON" 'v8.0.0-rc1/evil'
assert_fails_with "prerelease-rc-newline"       "$REASON" "$(printf 'v8.0.0-rc1\nFOO')"

# --- Cenarios de deteccao de arquitetura no Windows -------------------------
#
# Falsifica que install.sh lê RAW_OS (uname -s) em vez de uname -m para detectar
# arquitetura no Windows. Mecanismo: stubs configuraveis via STUB_UNAME_S / STUB_UNAME_M.
#
# Medicao: uname -s = MINGW64_NT-10.0-26200-ARM64 no Windows 11 ARM64 Git Bash (2026-09-16).
# Limitacao: MSYS_NT e CYGWIN_NT nao medidos diretamente — mesma base NT, mesmo padrao.
# O cenario B (x64 simulado) e declarado como simulacao, nao medicao em VM real.
#
# Stub de uname em diretorio proprio (separado de $STUB_BIN) para nao interferir
# com `command -v uname` em cenarios existentes.
UNAME_STUB_BIN="$WORK/uname-stubbin"
mkdir -p "$UNAME_STUB_BIN"

REAL_UNAME=$(command -v uname)
cat > "$UNAME_STUB_BIN/uname" << UNAME_STUB_HEREDOC
#!/bin/sh
if [ -n "\${STUB_UNAME_S:-}" ]; then
  case "\$1" in
    -s) printf '%s\n' "\${STUB_UNAME_S}" ;;
    -m) printf '%s\n' "\${STUB_UNAME_M:-x86_64}" ;;
    *)  "${REAL_UNAME}" "\$@" ;;
  esac
else
  "${REAL_UNAME}" "\$@"
fi
UNAME_STUB_HEREDOC
chmod +x "$UNAME_STUB_BIN/uname"

run_install_win() {
  # run_install_win ENV=val ...  — como run_install mas com stub de uname no PATH
  : > "$CURL_LOG"
  set +e
  OUT=$(env "$@" TRACKFW_INSTALL_DRYRUN=1 PATH="$UNAME_STUB_BIN:$STUB_BIN:$PATH" sh "$INSTALL_SH" 2>&1)
  EC=$?
  set -e
}

assert_url_contains() {
  # assert_url_contains label substring
  local label="$1" substr="$2"
  if ! grep -qF "$substr" <<<"$OUT"; then
    echo "FAIL [win-arch/$label]: URL/output nao contem '$substr'" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  echo "OK   [win-arch/$label]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
}

assert_win_fails_with() {
  local label="$1" pattern="$2"
  if [ "$EC" -eq 0 ]; then
    echo "FAIL [win-arch/$label]: esperava falha (exit != 0), saiu com 0" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  if ! grep -qF "$pattern" <<<"$OUT"; then
    echo "FAIL [win-arch/$label]: saiu com $EC mas mensagem nao contem '$pattern'" >&2
    echo "  output: $OUT" >&2
    exit 1
  fi
  echo "OK   [win-arch/$label]"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
}

# A — MINGW64 ARM64: uname -m diz x86_64 (mente), uname -s tem sufixo -ARM64
# Afirma: install.sh seleciona windows_arm64 lendo RAW_OS, nao uname -m.
# (Medido em VM real: ver braço A do relatorio ML-4I-bis.)
run_install_win "TRACKFW_VERSION=v7.3.0" \
                "STUB_UNAME_S=MINGW64_NT-10.0-26200-ARM64" \
                "STUB_UNAME_M=x86_64"
if [ "$EC" -ne 0 ]; then
  echo "FAIL [win-arch/mingw64-arm64]: esperava exit 0, saiu com $EC" >&2; echo "  output: $OUT" >&2; exit 1
fi
assert_url_contains "mingw64-arm64" "windows_arm64"

# B — MINGW64 x64 (SIMULADO): sem sufixo ARM64, string NT presente → amd64
# NOTA: e simulacao, nao medicao em VM x64 real. Declarado como tal.
# Afirma: ausencia do sufixo ARM64 com sinal NT positivo (*_NT-*) produz windows_amd64.
run_install_win "TRACKFW_VERSION=v7.3.0" \
                "STUB_UNAME_S=MINGW64_NT-10.0-19041" \
                "STUB_UNAME_M=x86_64"
if [ "$EC" -ne 0 ]; then
  echo "FAIL [win-arch/mingw64-x64-sim]: esperava exit 0, saiu com $EC (SIMULADO)" >&2; echo "  output: $OUT" >&2; exit 1
fi
assert_url_contains "mingw64-x64-sim" "windows_amd64"

# MSYS ARM64 — mesmo mecanismo, prefixo MSYS_NT
# Afirma: sufixo -ARM64 em MSYS_NT tambem seleciona windows_arm64.
run_install_win "TRACKFW_VERSION=v7.3.0" \
                "STUB_UNAME_S=MSYS_NT-10.0-26200-ARM64" \
                "STUB_UNAME_M=x86_64"
if [ "$EC" -ne 0 ]; then
  echo "FAIL [win-arch/msys-arm64]: esperava exit 0, saiu com $EC" >&2; echo "  output: $OUT" >&2; exit 1
fi
assert_url_contains "msys-arm64" "windows_arm64"

# C — sinal ausente: MINGW sem string de versao NT (nao tem _NT-) → recusa nomeada
# Afirma: install.sh recusa nomeando em vez de assumir amd64 quando nao ha sinal confiavel.
run_install_win "TRACKFW_VERSION=v7.3.0" \
                "STUB_UNAME_S=MINGW64" \
                "STUB_UNAME_M=x86_64"
assert_win_fails_with "no-nt-string-fails" "Arquitetura Windows nao reconhecida"

# TRACKFW_ARCH override — forcando arm64 num host que seria detectado como amd64
# Afirma: override valido e aceito e produz a arquitetura declarada.
run_install_win "TRACKFW_VERSION=v7.3.0" \
                "TRACKFW_ARCH=arm64" \
                "STUB_UNAME_S=MINGW64_NT-10.0-19041" \
                "STUB_UNAME_M=x86_64"
if [ "$EC" -ne 0 ]; then
  echo "FAIL [win-arch/trackfw-arch-override]: esperava exit 0, saiu com $EC" >&2; echo "  output: $OUT" >&2; exit 1
fi
assert_url_contains "trackfw-arch-override" "windows_arm64"

# TRACKFW_ARCH invalido — valor fora de amd64|arm64 deve falhar
# Afirma: TRACKFW_ARCH com valor nao admitido produz falha com mensagem "TRACKFW_ARCH invalido".
run_install_win "TRACKFW_VERSION=v7.3.0" \
                "TRACKFW_ARCH=x86_64" \
                "STUB_UNAME_S=MINGW64_NT-10.0-19041" \
                "STUB_UNAME_M=x86_64"
assert_win_fails_with "trackfw-arch-invalid" "TRACKFW_ARCH invalido"

# Nao-vacuidade (contra-braco): install.sh sem o fix entrega windows_amd64 com
# o mesmo stub ARM64, provando que o cenario mingw64-arm64 e discriminante.
# Cria copia regredida com Python substituindo o bloco de arch Windows pelo
# comportamento antigo (uname -m).
INSTALL_SH_REGRESSED="$WORK/install-regressed.sh"
python3 - "$INSTALL_SH" "$INSTALL_SH_REGRESSED" <<'REGRESS_PYEOF'
import re, sys
src = open(sys.argv[1]).read()
new_src = re.sub(
    r'(# --- Detectar ARCH ---\n).*?(?=\n# --- Honrar)',
    r'\1RAW_ARCH=$(uname -m)\n'
     'case "$RAW_ARCH" in\n'
     '  x86_64)          ARCH="amd64" ;;\n'
     '  aarch64|arm64)   ARCH="arm64" ;;\n'
     '  *)\n'
     '    echo "Arquitetura nao suportada: $RAW_ARCH" >&2; exit 1 ;;\n'
     'esac',
    src, flags=re.DOTALL)
if new_src == src:
    sys.stderr.write("FATAL [regress]: bloco de arch detection nao encontrado em install.sh\n")
    sys.exit(2)
open(sys.argv[2], 'w').write(new_src)
REGRESS_PYEOF

: > "$CURL_LOG"
set +e
OUT_REG=$(env "TRACKFW_VERSION=v7.3.0" \
  "STUB_UNAME_S=MINGW64_NT-10.0-26200-ARM64" \
  "STUB_UNAME_M=x86_64" \
  TRACKFW_INSTALL_DRYRUN=1 \
  PATH="$UNAME_STUB_BIN:$STUB_BIN:$PATH" \
  sh "$INSTALL_SH_REGRESSED" 2>&1)
EC_REG=$?
set -e
if [ "$EC_REG" -eq 0 ] && grep -qF "windows_amd64" <<<"$OUT_REG"; then
  echo "OK   [win-arch/nonvacuity-regressed]: install.sh sem fix entrega windows_amd64 com stub ARM64 — cenario mingw64-arm64 e discriminante"
  SCENARIOS_RUN=$((SCENARIOS_RUN + 1))
else
  echo "FAIL [win-arch/nonvacuity-regressed]: install.sh regredido nao entregou windows_amd64 com stub ARM64 (EC=$EC_REG)" >&2
  echo "  output: $OUT_REG" >&2
  exit 1
fi

# --- Guarda de vacuidade ----------------------------------------------------
if [ "$SCENARIOS_RUN" -eq 0 ]; then
  echo "FAIL [install-version-pin]: guarda de vacuidade — nenhum cenario rodou" >&2
  exit 1
fi

echo "install-version-pin: $SCENARIOS_RUN cenarios OK"
