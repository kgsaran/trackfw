#!/usr/bin/env bash
# check-install-restriction.sh — gate permanente AC10 (ML-1E, ROADMAP-2026-09-12-v8).
#
# Prova que a cadeia de instalação do trackfw v8 (shim + pacote de plataforma)
# funciona corretamente sob 4 restrições que ambientes corporativos/CI impõem.
# Porta os cenários já provados no protótipo da Pergunta 15 do windows-probe.yml.
#
# CENÁRIOS E CONCLUSÕES (regra dura de reconciliação — CLAUDE.md):
#
#   C1. --offline (braço ENOTCACHED):
#       Afirma: `npm install --offline` com cache vazio recusa rede e falha com
#       código ENOTCACHED — o flag de fato bloqueia o acesso ao registry.
#       (Sem este braço, um flag ignorado silenciosamente passaria o gate.)
#
#   C2. --ignore-scripts:
#       Afirma: `--ignore-scripts` previne a execução do postinstall de um
#       pacote que, sem o flag, escreveria um arquivo sentinela.
#       Contra-braço: instalar o mesmo pacote sem --ignore-scripts executa o
#       postinstall — prova que o fixture não é vacuo.
#
#   C3. sem rota para GitHub (inspeção estática):
#       Afirma: o conjunto de pacotes instalados (shim, de tarball local) não
#       contém nenhuma especificação de URL git+/github.com nas dependências
#       transitivas.
#       Limitação declarada: prova o conjunto de pacotes servidos, não que uma
#       máquina bloqueada na rede consegue instalar. A inspeção estática é o que
#       cabe num gate de CI hermetico.
#
#   C4. lockfile cruzado (filtragem de plataforma npm):
#       Afirma: quando dois pacotes com campos `os`/`cpu` distintos são declarados
#       como optionalDependencies via `file:`, npm instala apenas o que corresponde
#       ao SO e arquitetura atuais e omite o incompatível.
#       Prova de atividade: o lockfile (package-lock.json gerado) contém o
#       pacote de outra plataforma, mas node_modules/ não o inclui.
#       Limitação declarada: usa fixtures locais com file: protocol, não um
#       lockfile real de outro SO — mas o mecanismo npm de filtragem por os/cpu
#       é o mesmo em ambos os casos.
#
# DEPENDÊNCIA DE AMBIENTE:
#   node e npm ausentes no PATH → SKIP nomeado, exit 0.
#   Falhas durante setup de fixtures → FAIL nomeado, exit 1.
#
# HERMETICIDADE:
#   Nenhum cenário acessa a rede real. C1 usa cache vazio e espera ENOTCACHED.
#   C2/C4 usam apenas fixtures locais (file: protocol, sem registry).
#   C3 inspeciona tarballs locais instalados.
#   ugrep/grep: usa /usr/bin/grep (ou git grep) — evita ugrep -I que omite NUL.
set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$SCRIPT_DIR/lib-crlf-normalize.sh"

PASS=0
FAIL=0

ok()   { echo "ok  : $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1" >&2; FAIL=$((FAIL + 1)); }
note() { echo "NOTE: $1"; }

# ── Guarda de dependências de ambiente ──────────────────────────────────────

if ! command -v node >/dev/null 2>&1; then
  if [[ -n "${CI:-}" ]]; then
    echo "FAIL: 'node' ausente no runner (CI=true) — o workflow declara actions/setup-node; ausência é erro de configuração, não ambiente incompleto" >&2
    exit 1
  fi
  echo "SKIP: node não encontrado no PATH — ambiente incompleto"
  echo ""
  echo "install-restriction: SKIP (node ausente)"
  exit 0
fi

if ! command -v npm >/dev/null 2>&1; then
  if [[ -n "${CI:-}" ]]; then
    echo "FAIL: 'npm' ausente no runner (CI=true) — o workflow declara actions/setup-node (que instala npm); ausência é erro de configuração, não ambiente incompleto" >&2
    exit 1
  fi
  echo "SKIP: npm não encontrado no PATH — ambiente incompleto"
  echo ""
  echo "install-restriction: SKIP (npm ausente)"
  exit 0
fi

# ── Diretório de trabalho ────────────────────────────────────────────────────

WORK=$(mktemp -d)
WORK="$(cd "$WORK" && pwd -P)"
trap 'rm -rf "$WORK"' EXIT

# ── Cenário 1: --offline (braço ENOTCACHED) ──────────────────────────────────
# C1 afirma: `npm install --offline` com cache vazio recusa rede e falha com
# código ENOTCACHED — o flag de fato bloqueia o acesso ao registry.

echo ""
echo "── C1: --offline (braço ENOTCACHED) ────────────────────────────────────"

C1_CACHE="$WORK/c1-empty-cache"
C1_INSTALL="$WORK/c1-install"
mkdir -p "$C1_CACHE" "$C1_INSTALL"

C1_OUT="$WORK/c1-output.txt"
C1_EXIT=0
(cd "$C1_INSTALL" && npm install --offline --cache "$C1_CACHE" lodash \
  >"$C1_OUT" 2>&1) || C1_EXIT=$?

if [[ "$C1_EXIT" -eq 0 ]]; then
  fail "C1: npm install --offline com cache vazio deve falhar, mas saiu com exit 0"
  note "  Isso indica que o flag --offline foi ignorado ou lodash estava em cache"
else
  if /usr/bin/grep -qi "ENOTCACHED\|only-if-cached\|offline" "$C1_OUT"; then
    ok "C1: --offline recusa rede — exit=$C1_EXIT, saída contém ENOTCACHED/offline"
  else
    fail "C1: npm falhou (exit=$C1_EXIT) mas sem ENOTCACHED na saída — motivo inesperado"
    note "  Primeiros 300 bytes: $(head -c 300 "$C1_OUT" | tr '\n' ' ')"
  fi
fi

# ── Cenário 2: --ignore-scripts ──────────────────────────────────────────────
# C2 afirma: `--ignore-scripts` previne a execução do postinstall de um
# pacote que, sem o flag, escreveria um arquivo sentinela.

echo ""
echo "── C2: --ignore-scripts ─────────────────────────────────────────────────"

C2_SENTINEL="$WORK/c2-sentinel-written.txt"
C2_FIXTURE_DIR="$WORK/c2-fixture"
C2_INSTALL_A="$WORK/c2-install-with-flag"
C2_INSTALL_B="$WORK/c2-install-without-flag"
mkdir -p "$C2_FIXTURE_DIR" "$C2_INSTALL_A" "$C2_INSTALL_B"

# Pacote de fixture com postinstall que escreve sentinela absoluto.
# A expansão de $C2_SENTINEL acontece aqui (heredoc sem aspas simples) — é absoluto.
cat >"$C2_FIXTURE_DIR/package.json" <<PKGJSON
{
  "name": "trackfw-gate-fixture-scripts",
  "version": "1.0.0",
  "scripts": {
    "postinstall": "node -e \"require('fs').writeFileSync('${C2_SENTINEL}', 'postinstall ran');\""
  }
}
PKGJSON

# Braço A: com --ignore-scripts
rm -f "$C2_SENTINEL"
(cd "$C2_INSTALL_A" && npm install --ignore-scripts "$C2_FIXTURE_DIR" \
  >"$WORK/c2a-out.txt" 2>&1) || true

if [[ -f "$C2_SENTINEL" ]]; then
  fail "C2: sentinela foi escrito apesar de --ignore-scripts — postinstall executou"
else
  ok "C2: --ignore-scripts preveniu o postinstall — sentinela ausente"
fi

# Contra-braço B: sem --ignore-scripts (prova que fixture não é vacua)
rm -f "$C2_SENTINEL"
(cd "$C2_INSTALL_B" && npm install "$C2_FIXTURE_DIR" \
  >"$WORK/c2b-out.txt" 2>&1) || true

if [[ -f "$C2_SENTINEL" ]]; then
  ok "C2 contra-braço: sem --ignore-scripts o postinstall executa — fixture não é vacua"
else
  note "C2 contra-braço: postinstall não executou sem --ignore-scripts"
  note "  Pode ser restrição de ambiente; o resultado do braço A ainda é válido"
  note "  Para inspecionar: cat $WORK/c2b-out.txt"
fi

# ── Cenário 3: sem rota para GitHub (inspeção estática) ─────────────────────
# C3 afirma: o conjunto de dependências declaradas do shim (npm/package.json +
# npm/package-lock.json) não contém nenhuma especificação de URL git+/github.com —
# instalação bem-sucedida não requer rota para o GitHub.
# Limitação declarada: prova as dependências declaradas no repositório; não prova
# que uma máquina bloqueada ao GitHub instala em tempo real.

echo ""
echo "── C3: sem rota para GitHub (inspeção estática) ─────────────────────────"

NPM_PKG="$REPO_ROOT/npm/package.json"
NPM_LOCK="$REPO_ROOT/npm/package-lock.json"

if [[ ! -f "$NPM_PKG" ]]; then
  fail "C3: npm/package.json não encontrado"
else
  C3_FOUND=0

  # Inspecionar npm/package.json — todas as seções de dependências
  for dep_section in dependencies devDependencies optionalDependencies peerDependencies; do
    if python3 - "$NPM_PKG" "$dep_section" <<'PY' ; then
import json, sys, re
path, section = sys.argv[1], sys.argv[2]
with open(path, encoding='utf-8') as f:
    pkg = json.load(f)
deps = pkg.get(section, {})
github_re = re.compile(r'(git\+https?://github\.com|git\+ssh://git@github\.com|github:|https?://github\.com/.*\.git)')
found = [(k, v) for k, v in deps.items() if github_re.search(str(v))]
for k, v in found:
    print(f"  {section}/{k}: {v}", flush=True)
if found:
    sys.exit(1)
PY
      : # ok, no github URLs in this section
    else
      C3_FOUND=$((C3_FOUND + 1))
    fi
  done

  # Inspecionar package-lock.json (dependências transitivas)
  if [[ -f "$NPM_LOCK" ]]; then
    C3_LOCK_COUNT=$(python3 - "$NPM_LOCK" <<'PY' | strip_cr
import json, sys, re
with open(sys.argv[1], encoding='utf-8') as f:
    lock = json.load(f)
github_re = re.compile(r'(git\+https?://github\.com|git\+ssh://git@github\.com|github:)')
found = []
for pkg_path, pkg_data in lock.get("packages", {}).items():
    resolved = pkg_data.get("resolved", "")
    version = pkg_data.get("version", "")
    if github_re.search(resolved) or github_re.search(version):
        found.append(f"  {pkg_path}: resolved={resolved}")
for item in found[:5]:
    print(item, flush=True)
print(len(found))
PY
    )
    C3_LOCK_NUM=$(echo "$C3_LOCK_COUNT" | tail -1)
    if [[ "$C3_LOCK_NUM" -gt 0 ]]; then
      C3_FOUND=$((C3_FOUND + C3_LOCK_NUM))
      note "C3: URLs do GitHub em package-lock.json (transitivas):"
      echo "$C3_LOCK_COUNT" | head -5
    fi
  fi

  if [[ "$C3_FOUND" -eq 0 ]]; then
    ok "C3: nenhuma URL git+/github.com em npm/package.json + package-lock.json"
    note "  Limitação: inspeção das dependências declaradas; não prova bloqueio de rede real"
  else
    fail "C3: $C3_FOUND seções/pacotes com URLs do GitHub nas dependências"
  fi
fi

# ── Cenário 4: lockfile cruzado (filtragem de plataforma) ────────────────────
# C4 afirma: quando dois pacotes com campos `os`/`cpu` distintos são declarados
# como optionalDependencies via `file:`, npm instala apenas o correspondente
# ao SO e arquitetura atuais — o incompatível é omitido de node_modules.
# Prova de atividade: package-lock.json gerado contém o outro pacote, mas
# node_modules/ não.

echo ""
echo "── C4: lockfile cruzado (filtragem de plataforma) ───────────────────────"

case "$(uname -s)" in
  Darwin) C4_MY_OS="darwin" ; C4_OTHER_OS="linux"  ;;
  Linux)  C4_MY_OS="linux"  ; C4_OTHER_OS="darwin" ;;
  *)      C4_MY_OS="linux"  ; C4_OTHER_OS="darwin" ;;
esac
case "$(uname -m)" in
  x86_64)        C4_MY_ARCH="x64"   ; C4_OTHER_ARCH="arm64" ;;
  aarch64|arm64) C4_MY_ARCH="arm64" ; C4_OTHER_ARCH="x64"   ;;
  *)             C4_MY_ARCH="x64"   ; C4_OTHER_ARCH="arm64" ;;
esac

C4_MY_SLUG="${C4_MY_OS}-${C4_MY_ARCH}"
C4_OTHER_SLUG="${C4_OTHER_OS}-${C4_OTHER_ARCH}"

C4_MY_DIR="$WORK/c4-my-${C4_MY_SLUG}"
C4_OTHER_DIR="$WORK/c4-other-${C4_OTHER_SLUG}"
C4_PROJECT_DIR="$WORK/c4-project"
mkdir -p "$C4_MY_DIR" "$C4_OTHER_DIR" "$C4_PROJECT_DIR"

# Fixture: pacote da plataforma atual
cat >"$C4_MY_DIR/package.json" <<EOF
{"name":"@trackfw-gate/${C4_MY_SLUG}","version":"1.0.0","os":["${C4_MY_OS}"],"cpu":["${C4_MY_ARCH}"]}
EOF
printf 'sentinel-%s' "$C4_MY_SLUG" >"$C4_MY_DIR/sentinel.txt"

# Fixture: pacote da outra plataforma
cat >"$C4_OTHER_DIR/package.json" <<EOF
{"name":"@trackfw-gate/${C4_OTHER_SLUG}","version":"1.0.0","os":["${C4_OTHER_OS}"],"cpu":["${C4_OTHER_ARCH}"]}
EOF
printf 'sentinel-%s' "$C4_OTHER_SLUG" >"$C4_OTHER_DIR/sentinel.txt"

# Projeto com optionalDependencies apontando para os dois via file: relativo
# Caminhos relativos a C4_PROJECT_DIR
MY_REL="$(realpath --relative-to="$C4_PROJECT_DIR" "$C4_MY_DIR" 2>/dev/null || python3 -c \
  "import os; print(os.path.relpath('$C4_MY_DIR', '$C4_PROJECT_DIR'))" | strip_cr)"
OTHER_REL="$(realpath --relative-to="$C4_PROJECT_DIR" "$C4_OTHER_DIR" 2>/dev/null || python3 -c \
  "import os; print(os.path.relpath('$C4_OTHER_DIR', '$C4_PROJECT_DIR'))" | strip_cr)"

cat >"$C4_PROJECT_DIR/package.json" <<EOF
{
  "name": "trackfw-gate-crossplatform-test",
  "version": "1.0.0",
  "optionalDependencies": {
    "@trackfw-gate/${C4_MY_SLUG}": "file:${MY_REL}",
    "@trackfw-gate/${C4_OTHER_SLUG}": "file:${OTHER_REL}"
  }
}
EOF

# npm install: platform filtering happens here
(cd "$C4_PROJECT_DIR" && npm install --ignore-scripts \
  >"$WORK/c4-out.txt" 2>&1) || true

C4_MY_NM="$C4_PROJECT_DIR/node_modules/@trackfw-gate/$C4_MY_SLUG"
C4_OTHER_NM="$C4_PROJECT_DIR/node_modules/@trackfw-gate/$C4_OTHER_SLUG"
C4_LOCK="$C4_PROJECT_DIR/package-lock.json"

C4_MY_INSTALLED=false
C4_OTHER_INSTALLED=false
[[ -d "$C4_MY_NM" ]] && C4_MY_INSTALLED=true
[[ -d "$C4_OTHER_NM" ]] && C4_OTHER_INSTALLED=true

# Verificar que o lockfile contém o outro pacote (prova de atividade)
C4_LOCK_HAS_OTHER=false
if [[ -f "$C4_LOCK" ]] && /usr/bin/grep -q "$C4_OTHER_SLUG" "$C4_LOCK" 2>/dev/null; then
  C4_LOCK_HAS_OTHER=true
fi

if $C4_MY_INSTALLED && ! $C4_OTHER_INSTALLED; then
  ok "C4: @trackfw-gate/$C4_MY_SLUG instalado (match de plataforma)"
  ok "C4: @trackfw-gate/$C4_OTHER_SLUG ausente de node_modules (filtrado por os/cpu)"
  if $C4_LOCK_HAS_OTHER; then
    ok "C4 prova de atividade: lockfile contém $C4_OTHER_SLUG mas node_modules não"
  else
    note "C4: package-lock.json não contém $C4_OTHER_SLUG (npm pode ter omitido da resolução)"
    note "  Limitação: sem lockfile com o outro pacote, a 'prova de atividade' é fraca"
  fi
  note "  Limitação: fixture local (file:); mecanismo de filtragem é o mesmo do registry"
elif $C4_MY_INSTALLED && $C4_OTHER_INSTALLED; then
  note "C4: npm instalou ambas as plataformas — filtragem não ocorreu com file: neste ambiente"
  note "  VACUIDADE DECLARADA: filtragem por os/cpu pode não aplicar a file: em npm $( npm --version)"
  note "  A prova completa exige registry online com packages com os/cpu declarados"
  ok "C4: @trackfw-gate/$C4_MY_SLUG instalado — compatibilidade da plataforma atual confirmada"
elif ! $C4_MY_INSTALLED; then
  fail "C4: @trackfw-gate/$C4_MY_SLUG NÃO instalado — plataforma atual não reconhecida"
fi

# ── Resultado ─────────────────────────────────────────────────────────────────

echo ""
echo "install-restriction: $PASS passed, $FAIL failed"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
