#!/usr/bin/env bash
# check-no-literal-nul-in-source.sh — ML-1F
# Gate: nenhum arquivo-fonte rastreado pode conter byte NUL literal.
#
# ESCOPO: arquivos classificados como texto em .gitattributes (text=auto ou text=set).
#   Em 2026-09-13: 651 arquivos de texto de 1820 rastreados.
#   Binários legítimos (*.gif, etc.) têm text=unspecified e são pulados por design.
#   Se wave futura comitar fixture binário com NUL e ele tiver text=auto/.gitattributes
#   aponta para ele, a resposta é uma entrada declarada com razão, não filtro de extensão.
#
# 🔴 NÃO usa grep em lugar nenhum: grep (ugrep -I neste ambiente) pula silenciosamente
#    arquivos que contenham NUL, sendo derrotado pelo objeto medido.
#    Ferramenta correta: LC_ALL=C tr -d -c '\000' < arquivo | wc -c
#
# EXCEÇÃO DECLARADA (arquivo: scripts/nul-source-exceptions.txt):
#   Formato por linha: <caminho-relativo-à-raiz><TAB><contagem-de-NUL><TAB><razão>
#   O gate REPROVA em três modos de obsolescência:
#     (a) O arquivo declarado não existe mais       → Wave 3 o removeu; limpe a lista
#     (b) O arquivo existe mas NUL count é 0        → escape \0 foi aplicado; limpe a lista
#     (c) A contagem real diverge da declarada      → NUL novo ou remoção parcial
#   Nenhuma dessas condições é silenciosa — a exceção não pode se tornar permanente.
#   Não pinamos offsets na lista: eles mudam a cada edição não relacionada ao NUL.
#   Offsets são reportados apenas nas mensagens de falha, para localização.
#
# GUARDA DE VACUIDADE: examinados = 0 → reprova.
#   Arquivos pulados (não-regular, symlink, ilegível) são contados e nomeados
#   separadamente — vacuidade parcial é visível no output.
#
# ACHADO DE VARREDURA INICIAL (2026-09-13): 651 fontes texto examinados;
#   NUL encontrado exatamente nos dois declarados — sem terceiro fonte.
#
# ML-1F (ROADMAP-2026-09-12-v8-um-binario-muitos-canais)

set -euo pipefail
# Codificação de saída: força UTF-8 no stdio de todo python3 deste gate.
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$SCRIPT_DIR/lib-crlf-normalize.sh"
REPO_ROOT="${_CHECKNUL_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
EXCEPTIONS_FILE="${_CHECKNUL_EXCEPTIONS:-"$SCRIPT_DIR/nul-source-exceptions.txt"}"

RED='\033[0;31m'; GRN='\033[0;32m'; YEL='\033[1;33m'; NC='\033[0m'

# Extrai lista de arquivos classificados como texto pelo .gitattributes.
# Usa git ls-files --eol: coluna attr/ contém text= para fontes, vazio para binários.
# 🔴 NÃO usa grep: python3 faz o filtro.
# 🔴 NÃO usa heredoc em pipeline: o heredoc anularia o stdin do pipe;
#    o código Python fica no argumento -c.
_text_files_from_repo() {
  local root="$1"
  git -C "$root" ls-files --eol 2>/dev/null | python3 -c '
import sys
for line in sys.stdin:
    line = line.rstrip("\n")
    if "\t" not in line:
        continue
    attrs_part, path = line.rsplit("\t", 1)
    path = path.strip()
    if "text=" in attrs_part:
        print(path)
' | strip_cr
}

# ─────────────────────────────────────────────────────────────────────────────
# SELF-TEST: falsifica nas cinco direções sem mutar a árvore real.
# ─────────────────────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--self-test" ]]; then
  echo "=== check-no-literal-nul-in-source --self-test ==="
  FAIL=0
  WORK=$(mktemp -d "${TMPDIR:-/tmp}/checknul-st.XXXXXX")
  trap 'rm -rf "$WORK"' EXIT

  # Inicializar repositório git mínimo (identidade local)
  git -C "$WORK" init -q
  git -C "$WORK" config user.email "st@test"
  git -C "$WORK" config user.name "SelfTest"
  mkdir -p "$WORK/src"

  # .gitattributes: classifica *.js como texto para que _text_files_from_repo os inclua
  printf '*.js text=auto eol=lf\n' > "$WORK/.gitattributes"

  # Arquivo limpo (sem NUL) — reutilizado em múltiplos arms
  printf 'hello world\nno nul here\n' > "$WORK/src/clean.js"
  git -C "$WORK" add .

  # Exceções vazia (nenhuma exceção declarada)
  EMPTY_EXC=$(mktemp "${TMPDIR:-/tmp}/checknul-exc.XXXXXX")
  printf '' > "$EMPTY_EXC"

  # ── Arm 1: árvore limpa → deve passar ─────────────────────────────────────
  # Afirma: varredura com todos os fontes limpos → gate sai com rc=0.
  arm1_rc=0
  _CHECKNUL_ROOT="$WORK" _CHECKNUL_EXCEPTIONS="$EMPTY_EXC" \
    bash "${BASH_SOURCE[0]}" > /dev/null 2>&1 || arm1_rc=$?
  if [[ $arm1_rc -eq 0 ]]; then
    echo "Arm 1 PASS: árvore limpa → passou (rc=0)"
  else
    echo "Arm 1 FAIL: árvore limpa → deveria passar, saiu rc=$arm1_rc"
    FAIL=1
  fi

  # ── Arm 2: NUL não declarado → deve reprovar nomeando arquivo e offset ─────
  # Afirma: arquivo rastreado com NUL não declarado → gate sai com rc≠0
  #         e nomeia o arquivo na saída.
  printf 'antes\x00depois\n' > "$WORK/src/tainted.js"
  git -C "$WORK" add .
  arm2_out=""
  arm2_rc=0
  arm2_out=$(_CHECKNUL_ROOT="$WORK" _CHECKNUL_EXCEPTIONS="$EMPTY_EXC" \
    bash "${BASH_SOURCE[0]}" 2>&1) || arm2_rc=$?
  if [[ $arm2_rc -ne 0 ]]; then
    # Verificar que o arquivo é nomeado (python3 — sem grep)
    if python3 -c "import sys; sys.exit(0 if 'tainted.js' in sys.stdin.read() else 1)" \
        <<< "$arm2_out"; then
      echo "Arm 2 PASS: NUL não declarado → reprovou nomeando tainted.js"
    else
      echo "Arm 2 FAIL: reprovou mas não nomeou o arquivo na saída"
      printf '  saída: %s\n' "$arm2_out"
      FAIL=1
    fi
  else
    echo "Arm 2 FAIL: NUL não declarado → deveria reprovar (rc=$arm2_rc)"
    FAIL=1
  fi
  # Remover arquivo manchado antes do próximo arm
  rm -f "$WORK/src/tainted.js"
  git -C "$WORK" rm -qf "src/tainted.js" 2>/dev/null || true

  # ── Arm 3: exceção declarada com arquivo ausente → deve reprovar ──────────
  # Afirma: entrada na lista cujo arquivo não existe → exceção obsoleta (a) → rc≠0.
  EXC_MISSING=$(mktemp "${TMPDIR:-/tmp}/checknul-exc.XXXXXX")
  printf 'src/ghost.js\t1\tarquivo que não existe\n' > "$EXC_MISSING"
  arm3_rc=0
  _CHECKNUL_ROOT="$WORK" _CHECKNUL_EXCEPTIONS="$EXC_MISSING" \
    bash "${BASH_SOURCE[0]}" > /dev/null 2>&1 || arm3_rc=$?
  if [[ $arm3_rc -ne 0 ]]; then
    echo "Arm 3 PASS: exceção obsoleta (arquivo ausente) → reprovou"
  else
    echo "Arm 3 FAIL: exceção obsoleta (arquivo ausente) → deveria reprovar"
    FAIL=1
  fi

  # ── Arm 4: exceção declarada, arquivo sem NUL → deve reprovar ─────────────
  # Afirma: arquivo na lista com contagem > 0 mas NUL real = 0
  #         → exceção desnecessária (b) → rc≠0.
  EXC_ZERO=$(mktemp "${TMPDIR:-/tmp}/checknul-exc.XXXXXX")
  printf 'src/clean.js\t1\tfalso positivo\n' > "$EXC_ZERO"
  arm4_rc=0
  _CHECKNUL_ROOT="$WORK" _CHECKNUL_EXCEPTIONS="$EXC_ZERO" \
    bash "${BASH_SOURCE[0]}" > /dev/null 2>&1 || arm4_rc=$?
  if [[ $arm4_rc -ne 0 ]]; then
    echo "Arm 4 PASS: exceção desnecessária (NUL real = 0) → reprovou"
  else
    echo "Arm 4 FAIL: exceção desnecessária (NUL real = 0) → deveria reprovar"
    FAIL=1
  fi

  # ── Arm 5: contagem declarada diverge da real → deve reprovar ────────────
  # Afirma: arquivo na lista com 2 NUL reais mas apenas 1 declarado
  #         → contagem divergente (c) → rc≠0.
  printf 'dois\x00nuls\x00aqui\n' > "$WORK/src/two_nul.js"
  git -C "$WORK" add .
  EXC_WRONG=$(mktemp "${TMPDIR:-/tmp}/checknul-exc.XXXXXX")
  printf 'src/two_nul.js\t1\tdeclarei 1 mas tem 2\n' > "$EXC_WRONG"
  arm5_rc=0
  _CHECKNUL_ROOT="$WORK" _CHECKNUL_EXCEPTIONS="$EXC_WRONG" \
    bash "${BASH_SOURCE[0]}" > /dev/null 2>&1 || arm5_rc=$?
  if [[ $arm5_rc -ne 0 ]]; then
    echo "Arm 5 PASS: contagem divergente (declarou 1, real é 2) → reprovou"
  else
    echo "Arm 5 FAIL: contagem divergente → deveria reprovar"
    FAIL=1
  fi

  echo "=== self-test concluído ==="
  if [[ $FAIL -ne 0 ]]; then
    echo -e "${RED}SELF-TEST FALHOU em um ou mais arms.${NC}"
    exit 1
  fi
  echo -e "${GRN}SELF-TEST PASSOU: todos os 5 arms verificados.${NC}"
  exit 0
fi

# ─────────────────────────────────────────────────────────────────────────────
# GATE PRINCIPAL
# ─────────────────────────────────────────────────────────────────────────────

FAIL=0
examined=0
skipped=0
declare -a skipped_files=()
nul_violations=0

# 1. Verificar que o arquivo de exceções existe
if [[ ! -f "$EXCEPTIONS_FILE" ]]; then
  echo -e "${RED}ERRO: arquivo de exceções não encontrado: $EXCEPTIONS_FILE${NC}"
  exit 1
fi

# 2. Carregar exceções
declare -a EXC_PATHS=()
declare -a EXC_COUNTS=()
declare -a EXC_REASONS=()
while IFS=$'\t' read -r exc_path exc_count exc_reason || [[ -n "${exc_path:-}" ]]; do
  exc_path="${exc_path:-}"
  [[ -z "$exc_path" || "${exc_path:0:1}" == "#" ]] && continue
  EXC_PATHS+=("$exc_path")
  EXC_COUNTS+=("${exc_count:-0}")
  EXC_REASONS+=("${exc_reason:-sem razão declarada}")
done < "$EXCEPTIONS_FILE"

# 3. Validar cada exceção declarada (três modos de obsolescência)
for i in "${!EXC_PATHS[@]}"; do
  exc_path="${EXC_PATHS[$i]}"
  exc_count="${EXC_COUNTS[$i]}"
  exc_reason="${EXC_REASONS[$i]}"
  full_path="$REPO_ROOT/$exc_path"

  # (a) Arquivo deve existir e não ser symlink
  if [[ -L "$full_path" ]] || [[ ! -f "$full_path" ]]; then
    echo -e "${RED}FALHA: exceção obsoleta (a) — '$exc_path' não existe mais no disco.${NC}"
    echo "        A Wave 3 (ML-3A) provavelmente removeu este arquivo."
    echo "        Remova a entrada de: $EXCEPTIONS_FILE"
    FAIL=1
    continue
  fi

  # (b) e (c) Medir contagem real de NUL
  real_count=""
  measure_ok=0
  real_count=$(LC_ALL=C tr -d -c '\000' < "$full_path" 2>/dev/null | wc -c | tr -d ' \t') \
    && measure_ok=1 || true
  if [[ $measure_ok -eq 0 ]] || [[ -z "$real_count" ]]; then
    echo -e "${RED}ERRO: falha ao medir '$exc_path' — verificar manualmente.${NC}"
    FAIL=1
    continue
  fi

  if [[ "$real_count" -eq 0 ]]; then
    # (b) NUL desapareceu — escape \0 foi aplicado ou arquivo limpo
    echo -e "${RED}FALHA: exceção obsoleta (b) — '$exc_path' não contém mais NUL.${NC}"
    echo "        Corrija o código-fonte usando '\\0' (escape) e remova a entrada de: $EXCEPTIONS_FILE"
    FAIL=1
    continue
  fi

  if [[ "$real_count" != "$exc_count" ]]; then
    # (c) Contagem divergiu
    echo -e "${RED}FALHA: exceção obsoleta (c) — '$exc_path' contagem diverge.${NC}"
    echo "        Declarado: $exc_count NUL | Real: $real_count NUL"
    echo "        Novo NUL introduzido, ou remoção parcial não declarada. Atualize: $EXCEPTIONS_FILE"
    FAIL=1
    continue
  fi

  echo -e "${YEL}aviso (excecao declarada): $exc_path — $real_count NUL (${exc_reason})${NC}"
done

# 4. Varredura de arquivos-fonte classificados como texto em .gitattributes
while IFS= read -r rel; do
  [[ -z "$rel" ]] && continue
  full_path="$REPO_ROOT/$rel"

  # Symlinks: pular (git rastreia o link, não o alvo)
  if [[ -L "$full_path" ]]; then
    skipped=$((skipped + 1))
    skipped_files+=("$rel (symlink)")
    continue
  fi

  # Não-regular (FIFO, dispositivo, entrada de diretório falsa, etc.)
  if [[ ! -f "$full_path" ]]; then
    skipped=$((skipped + 1))
    skipped_files+=("$rel (não-regular ou ausente)")
    continue
  fi

  # Ilegível → falha fechada (não silenciosa)
  if [[ ! -r "$full_path" ]]; then
    echo -e "${RED}ERRO: '$rel' não é legível — tratado como violação.${NC}"
    FAIL=1
    nul_violations=$((nul_violations + 1))
    continue
  fi

  examined=$((examined + 1))

  # Medir NUL — capturar rc explicitamente sem abortar o loop inteiro
  nul_count=""
  measure_rc=0
  nul_count=$(LC_ALL=C tr -d -c '\000' < "$full_path" 2>/dev/null | wc -c | tr -d ' \t') \
    || measure_rc=$?
  if [[ $measure_rc -ne 0 ]] || [[ -z "$nul_count" ]]; then
    echo -e "${RED}ERRO: falha ao medir '$rel' (rc=$measure_rc) — tratado como violação.${NC}"
    FAIL=1
    nul_violations=$((nul_violations + 1))
    continue
  fi

  if [[ "$nul_count" -gt 0 ]]; then
    # Verificar se é exceção declarada
    is_declared=0
    for i in "${!EXC_PATHS[@]}"; do
      if [[ "$rel" == "${EXC_PATHS[$i]}" ]]; then
        is_declared=1
        break
      fi
    done

    if [[ $is_declared -eq 0 ]]; then
      # Reportar com offsets — passagem do caminho via argv, sem interpolação no código Python
      offsets=$(python3 - "$full_path" << 'PYEOF' | strip_cr
import sys
with open(sys.argv[1], 'rb') as f:
    data = f.read()
offs = [str(i) for i, b in enumerate(data) if b == 0]
more = (' ... (e mais %d)' % (len(offs) - 5)) if len(offs) > 5 else ''
print(', '.join(offs[:5]) + more)
PYEOF
)
      echo -e "${RED}FALHA: $rel — $nul_count byte(s) NUL literal(is) em offset(s): $offsets${NC}"
      nul_violations=$((nul_violations + 1))
      FAIL=1
    fi
  fi
done < <(_text_files_from_repo "$REPO_ROOT")

# 5. Guarda de vacuidade
if [[ $examined -eq 0 ]]; then
  echo -e "${RED}FALHA: guarda de vacuidade — nenhum arquivo-fonte rastreado examinado.${NC}"
  echo "        git ls-files --eol não retornou fontes com text= em .gitattributes."
  echo "        Verificar .gitattributes e o repositório."
  if [[ $skipped -gt 0 ]]; then
    echo "        Pulados ($skipped):"
    for sf in "${skipped_files[@]:0:10}"; do echo "          $sf"; done
    [[ ${#skipped_files[@]} -gt 10 ]] && echo "          ... (e mais)"
  fi
  exit 1
fi

echo "Examinados (texto): $examined | Pulados (não-regular/symlink): $skipped | Violações: $nul_violations"

if [[ $FAIL -ne 0 ]]; then
  echo -e "${RED}FALHA: gate reprovou.${NC}"
  exit 1
fi

echo -e "${GRN}OK: nenhum fonte rastreado contém byte NUL literal não declarado.${NC}"
exit 0
