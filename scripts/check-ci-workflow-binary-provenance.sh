#!/usr/bin/env bash
# check-ci-workflow-binary-provenance.sh — AC5 gate (ML-1C corretivo do ML-1B,
# REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor...)
#
# Afirma: todo workflow commitado deste repositório que execute "trackfw validate"
# compila o binário do trackfw do código do próprio PR — prova positiva de um
# passo "go build .../cmd/trackfw" no arquivo. Qualquer mecanismo de obtenção
# de binário publicado (npm, pip, brew, install.sh | sh, download-artifact,
# imagem pré-construída, ou outro mecanismo ainda desconhecido) reprova por
# ausência da prova positiva, não por reconhecimento do mecanismo. Isto fecha
# a classe de bypasses por omissão que a versão de lista proibida (ML-1B)
# deixava aberta.
#
# Escopo: workflows commitados em .github/workflows/*.yml e
# .github/workflows/*.yaml que executam "trackfw validate".
# Um workflow que obtenha o trackfw por razão legítima diferente (ex.: smoke
# test de release que DEVE instalar o binário publicado) está fora do escopo
# por não executar "trackfw validate".
#
# Decisão de escopo — residual declarado:
#   Um workflow que rode "trackfw doctor" ou "trackfw status" com binário
#   publicado está fora do escopo (não executa "trackfw validate"). Esta é
#   uma decisão explícita, não um esquecimento — se tal workflow existir, a
#   garantia deste gate não o cobre; um gate dedicado seria necessário.
#
# GitLab CI — residual declarado:
#   buildGitLabCIWorkflowContent (scaffold.go:2021) emite "install.sh | sh"
#   sem braço de produtor. O caminho que o builder escreve é
#   ".gitlab-ci-trackfw.yml" (constante GitLabCIWorkflowPath, scaffold.go:1947).
#   Esse arquivo não está no escopo de ".github/workflows/"; medição em
#   2026-09-17 confirma que não há ".gitlab-ci*" commitado neste repositório
#   (git ls-files '.gitlab-ci*' retorna vazio). O gate não cobre GitLab CI;
#   cobertura de template é responsabilidade do check-ci-workflow-pin-parity.sh
#   ou de um gate dedicado se um arquivo GitLab CI vier a ser commitado.
#   Nota: o handoff do ML-1C mencionava ".gitlab-ci.yml" mas a constante no
#   código é ".gitlab-ci-trackfw.yml" — a medição prevalece sobre o handoff.
#
# Exceções legítimas:
#   Para adicionar uma exceção nomeada, inclua o basename em ALLOWED_EXCEPTIONS
#   com o motivo. NÃO alargue o padrão de prova positiva.
#   Hoje: nenhuma exceção — todos os workflows deste repositório que executam
#   "trackfw validate" compilam do fonte.
#
# Enumeração: git ls-files — apenas arquivos commitados; GitHub Actions aceita
# .yml e .yaml; ambas as extensões são enumeradas. Arquivos com outras extensões
# em .github/workflows/ geram aviso.
#
# SKIP_COUNTER_ARMS=1: pula a seção de contra-braço (usado internamente).
set -euo pipefail

ROOT="${1:-.}"

# ---------------------------------------------------------------------------
# ALLOWED_EXCEPTIONS: basename → motivo. Vazio — nenhuma exceção hoje.
# ---------------------------------------------------------------------------
declare -A ALLOWED_EXCEPTIONS=()

# ---------------------------------------------------------------------------
# runs_trackfw_validate FILE
# Returns 0 if FILE (comment-stripped) contains "trackfw validate".
# ---------------------------------------------------------------------------
runs_trackfw_validate() {
  local file=$1
  grep -v '^[[:space:]]*#' "$file" | grep -qF 'trackfw validate'
}

# ---------------------------------------------------------------------------
# has_source_build FILE
# Returns 0 if FILE (comment-stripped) contains a step that compiles trackfw
# from the PR's own source code: a line matching "go build .*/cmd/trackfw".
# Comment stripping prevents a comment like "# go build ./cmd/trackfw" from
# satisfying the proof requirement.
# ---------------------------------------------------------------------------
has_source_build() {
  local file=$1
  grep -v '^[[:space:]]*#' "$file" \
    | grep -qE 'go build[[:space:]].*[[:space:]](\.\/|[a-zA-Z0-9_./-]+\/)cmd\/trackfw([[:space:]]|$)'
}

# ---------------------------------------------------------------------------
# Main scan.
# ---------------------------------------------------------------------------
MAIN_FAIL=0
MAIN_CHECKED=0

# Enumerate .yml and .yaml (GitHub Actions accepts both).
WF_YML=(); WF_YAML=()
mapfile -t WF_YML  < <(cd "$ROOT" && git ls-files '.github/workflows/*.yml'  2>/dev/null || true)
mapfile -t WF_YAML < <(cd "$ROOT" && git ls-files '.github/workflows/*.yaml' 2>/dev/null || true)
ALL_WORKFLOWS=("${WF_YML[@]}" "${WF_YAML[@]}")

# Count all files in .github/workflows/ to detect unexpected extensions.
ALL_WF_FILES=()
mapfile -t ALL_WF_FILES < <(cd "$ROOT" && git ls-files '.github/workflows/' 2>/dev/null | grep -v '^$' || true)
TOTAL_ALL=${#ALL_WF_FILES[@]}
TOTAL_ENUM=${#ALL_WORKFLOWS[@]}
if [ "$TOTAL_ALL" -gt "$TOTAL_ENUM" ]; then
  SKIPPED=$(( TOTAL_ALL - TOTAL_ENUM ))
  echo "check-ci-workflow-binary-provenance: AVISO — $SKIPPED arquivo(s) em .github/workflows/ com extensao diferente de .yml/.yaml ignorados na enumeracao" >&2
fi

for WF in "${ALL_WORKFLOWS[@]}"; do
  FULL="$ROOT/$WF"
  if ! runs_trackfw_validate "$FULL"; then
    continue
  fi
  MAIN_CHECKED=$(( MAIN_CHECKED + 1 ))

  BN=$(basename "$WF")
  if [[ ${ALLOWED_EXCEPTIONS[$BN]+_} ]]; then
    echo "OK excecao nomeada: $WF — ${ALLOWED_EXCEPTIONS[$BN]}"
    continue
  fi

  if ! has_source_build "$FULL"; then
    echo "check-ci-workflow-binary-provenance: FALHA — $WF executa 'trackfw validate' sem compilar o trackfw do codigo do PR (nenhum passo 'go build .../cmd/trackfw' encontrado)" >&2
    MAIN_FAIL=1
  fi
done

# Vacuity guard.
if [ "$MAIN_CHECKED" -eq 0 ]; then
  echo "check-ci-workflow-binary-provenance: FALHA (vacuidade) — nenhum workflow com 'trackfw validate' encontrado em .github/workflows/; o gate nao mediu nada" >&2
  MAIN_FAIL=1
fi

# Early exit in counter-arm mode (avoids infinite recursion).
if [ "${SKIP_COUNTER_ARMS:-0}" = "1" ]; then
  if [ "$MAIN_FAIL" -ne 0 ]; then
    exit 1
  fi
  exit 0
fi

# ---------------------------------------------------------------------------
# Counter-arms: six fixtures in throwaway git repos, each tested in isolation
# by calling this script with SKIP_COUNTER_ARMS=1.
#   Five must FAIL  (npm, pip, brew, artifact, .yaml extension with install.sh)
#   One must PASS   (compiles from source)
# Each bad fixture is in its own dir so the failure output names the file.
# ---------------------------------------------------------------------------
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-binary-provenance.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

COUNTER_FAIL=0

# make_fixture_dir SUBDIR
# Creates $WORK/SUBDIR/.github/workflows/ and returns the subdir path.
make_fixture_dir() {
  local d="$WORK/$1/.github/workflows"
  mkdir -p "$d"
  echo "$WORK/$1"
}

init_git() {
  local d=$1
  (cd "$d" && git init -q && git add -A) 2>/dev/null
}

# assert_fails FIXTURE_DIR FIXTURE_FILENAME LABEL
# Verifies that running the scan on FIXTURE_DIR:
#   (a) exits non-zero, AND
#   (b) outputs a line containing FIXTURE_FILENAME.
assert_fails() {
  local fdir=$1 fname=$2 label=$3
  local output rc=0
  output=$(SKIP_COUNTER_ARMS=1 bash "$0" "$fdir" 2>&1) || rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "check-ci-workflow-binary-provenance: FALHA (contra-braco $label) — deveria retornar RC!=0 mas retornou RC=0" >&2
    COUNTER_FAIL=1
    return
  fi
  if ! echo "$output" | grep -qF "$fname"; then
    echo "check-ci-workflow-binary-provenance: FALHA (contra-braco $label) — output nao nomeia o arquivo '$fname'" >&2
    echo "  output capturado: $output" >&2
    COUNTER_FAIL=1
    return
  fi
  echo "OK contra-braco: $fname reprova nomeando o arquivo ($label)"
}

# assert_passes FIXTURE_DIR LABEL
# Verifies that running the scan on FIXTURE_DIR exits zero.
assert_passes() {
  local fdir=$1 label=$2
  local rc=0
  SKIP_COUNTER_ARMS=1 bash "$0" "$fdir" >/dev/null 2>&1 || rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "check-ci-workflow-binary-provenance: FALHA (contra-braco $label) — fixture com go build deveria aprovar (RC=0) mas retornou RC=$rc" >&2
    COUNTER_FAIL=1
    return
  fi
  echo "OK contra-braco: $label aprovado (gate nao e uniformemente vermelho)"
}

# ---------------------------------------------------------------------------
# bad-1: npm install -g trackfw
# Reconciliacao: o gate invertido reprova npm porque nenhum passo
# "go build .../cmd/trackfw" existe — npm e binario publicado.
# ---------------------------------------------------------------------------
D=$(make_fixture_dir bad-npm)
cat > "$D/.github/workflows/w-npm.yml" << 'YAMLEOF'
name: w-npm
on: [pull_request]
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm install -g trackfw
      - run: trackfw validate
YAMLEOF
init_git "$D"
assert_fails "$D" "w-npm.yml" "bad-npm"

# ---------------------------------------------------------------------------
# bad-2: pip install trackfw
# Reconciliacao: o gate invertido reprova pip porque nenhum passo
# "go build .../cmd/trackfw" existe — pip e binario publicado.
# ---------------------------------------------------------------------------
D=$(make_fixture_dir bad-pip)
cat > "$D/.github/workflows/w-pip.yml" << 'YAMLEOF'
name: w-pip
on: [pull_request]
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: pip install trackfw
      - run: trackfw validate
YAMLEOF
init_git "$D"
assert_fails "$D" "w-pip.yml" "bad-pip"

# ---------------------------------------------------------------------------
# bad-3: brew install trackfw
# Reconciliacao: o gate invertido reprova brew porque nenhum passo
# "go build .../cmd/trackfw" existe — brew e binario publicado.
# ---------------------------------------------------------------------------
D=$(make_fixture_dir bad-brew)
cat > "$D/.github/workflows/w-brew.yml" << 'YAMLEOF'
name: w-brew
on: [pull_request]
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: brew install kgsaran/tap/trackfw
      - run: trackfw validate
YAMLEOF
init_git "$D"
assert_fails "$D" "w-brew.yml" "bad-brew"

# ---------------------------------------------------------------------------
# bad-4: actions/download-artifact@v4
# Reconciliacao: o gate invertido reprova download-artifact porque nenhum passo
# "go build .../cmd/trackfw" existe — artefato pre-construido e binario publicado.
# ---------------------------------------------------------------------------
D=$(make_fixture_dir bad-artifact)
cat > "$D/.github/workflows/w-artifact.yml" << 'YAMLEOF'
name: w-artifact
on: [pull_request]
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          name: trackfw-binary
      - run: chmod +x trackfw && sudo mv trackfw /usr/local/bin/
      - run: trackfw validate
YAMLEOF
init_git "$D"
assert_fails "$D" "w-artifact.yml" "bad-artifact"

# ---------------------------------------------------------------------------
# bad-5: extensao .yaml com install.sh | sh
# Reconciliacao: a enumeracao agora cobre *.yaml; este fixture prova que um
# arquivo .yaml com binario publicado e detectado — o ML-1B ignorava .yaml,
# reduzindo a contagem sem aviso.
# ---------------------------------------------------------------------------
D=$(make_fixture_dir bad-yaml)
cat > "$D/.github/workflows/w-yaml.yaml" << 'YAMLEOF'
name: w-yaml
on: [pull_request]
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
      - run: trackfw validate
YAMLEOF
init_git "$D"
assert_fails "$D" "w-yaml.yaml" "bad-yaml"

# ---------------------------------------------------------------------------
# good-1: compila do fonte
# Reconciliacao: o gate invertido nao e uniformemente vermelho — um workflow
# que compila do fonte e verificado e nao sinalizado.
# ---------------------------------------------------------------------------
D=$(make_fixture_dir good-source)
cat > "$D/.github/workflows/w-source.yml" << 'YAMLEOF'
name: w-source
on: [pull_request]
jobs:
  governance-go-install:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: go build -o /usr/local/bin/trackfw ./cmd/trackfw
      - run: trackfw validate
YAMLEOF
init_git "$D"
assert_passes "$D" "good-source"

# ---------------------------------------------------------------------------
# Resultado final.
# ---------------------------------------------------------------------------
if [ "$MAIN_FAIL" -ne 0 ] || [ "$COUNTER_FAIL" -ne 0 ]; then
  echo "check-ci-workflow-binary-provenance: FALHOU" >&2
  exit 1
fi

echo "check-ci-workflow-binary-provenance: OK — $MAIN_CHECKED workflow(s) com 'trackfw validate' verificados, todos compilam do fonte do PR"
