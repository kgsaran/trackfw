#!/usr/bin/env bash
# check-interpolated-path-in-python.sh — gate anti-reintroducao do ML-1B (Wave 1)
# ROADMAP-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md
#
# ============================================================================
# POR QUE ESTE GATE EXISTE
# ============================================================================
#   O Git Bash (MSYS) converte caminho POSIX -> caminho do Windows em `argv`,
#   e NAO dentro de uma string de codigo. Quando o shell interpola `$VAR` no
#   TEXTO do programa Python, o Python nativo do Windows recebe `/d/a/...` ou
#   `/tmp/...` como literal e o `open()` morre com FileNotFoundError num
#   diretorio que o `mkdir` acabou de criar.
#
#   Efeito MEDIDO em producao, run 36017761462 (windows-census.yml, main):
#     FileNotFoundError: … '/d/a/trackfw/trackfw/npm/package.json'
#     CHUNK_ABORT rc=1 line=3609 … cmd=python3 -c "
#   O aborto removeu o rotulo `falsify/integration-assets/direction-b-shim-absent`
#   do censo inteiro — dano do tipo REMOVE MEDICAO, nao do tipo produz vermelho.
#
#   O ML-0A (docs/seguranca/2026-09-24-caminhos-interpolados-no-codigo-python.md)
#   enumerou o universo e fechou em 6 interpolacoes (a), em 5 blocos, 3 arquivos.
#   O ML-1A as roteou por `sys.argv[1]`. Este gate impede a setima.
#
#   FORMA CORRETA, precedente VIVO: scripts/check-thirdparty-parity.sh:167
#       installed_sha256=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))…)" \
#         "$project/.trackfw/thirdparty-provenance.json" | strip_cr)
#   e scripts/check-thirdparty-parity.sh:176 (heredoc CITADO + argv):
#       python3 - "$project/.trackfw/integrations-manifest.json" "$project" <<'PY'
#   🔴 NAO cite `check-doctor-parity.sh` como precedente: ele foi DELETADO em
#   `2eae0a44` (v8.0.0, #365). O ML-0A mediu (`ls` -> No such file or directory);
#   a citacao errada ja custou um handoff nesta REQ.
#
# ============================================================================
# DISCRIMINANTE — duas condicoes, as duas decidiveis por LEITURA
# ============================================================================
#   Condicao 1 — CLASSE DE CITACAO: a expansao do shell acontece dentro do
#   texto do programa. Decidida pela forma de abertura do corpo Python:
#
#     EXPANSIVEL (o shell expande; o gate EXAMINA)
#       python3 -c "…"        aspas DUPLAS
#       python3 … <<PY        delimitador de heredoc NAO citado
#
#     LITERAL (o `$` chega intacto ao Python; o gate NUNCA acusa)
#       python3 -c '…'        aspas SIMPLES
#       python3 … <<'PY'      delimitador CITADO
#       python3 … <<"PY"      delimitador CITADO
#
#   Condicao 2 — USO COMO CAMINHO: o valor interpolado alimenta uma chamada
#   Python que toca o filesystem (lista PATH_CALL_RE abaixo).
#
#   Violacao = condicao 1 E condicao 2. Nenhuma das duas exige saber o que o
#   script faz.
#
# ============================================================================
# 🔴 OS DOIS CASOS DE NAO-FLAG — obrigatorios, nomeados pelo ML-0A §1.4
# ============================================================================
#   Os dois sao ARMADILHAS REAIS e cada um e poupado por uma razao DIFERENTE.
#   Duas razoes independentes: se uma quebrar, a outra nao mascara a falha.
#
#   1. scripts/check-validate-rule-pins.sh:371
#        'command': '$PWD/scripts/trackfw-credential-guard.sh'
#      Esta dentro de `<<'PY'` — heredoc CITADO. O `$PWD` e LITERAL, e a
#      fixture (`cg-claude-pwd`) existe justamente para testar um `$PWD` NAO
#      expandido num hook. 🔴 Um gate que casasse o token `$` reprovaria a
#      LINHA QUE O PR #417 ACABOU DE CONSERTAR.
#      Poupado por CONDICAO 1 (classe de citacao = LITERAL).
#
#   2. scripts/check-serve-browser-security.sh:97
#        url = '$ZONE_VECTOR_URL'
#      Esta num corpo EXPANSIVEL (`python3 -c "`), entao a condicao 1 VALE.
#      Mas e URL, nao caminho: o corpo so chama `subprocess.list2cmdline` e
#      `print`, nunca toca o FS. 🔴 E o teste DEPENDE da nao-conversao para
#      preservar o vetor `fe80::1%eth0&calc.exe&echo` intacto — acusa-lo
#      levaria alguem a "corrigir" um gate de seguranca e DESTRUIR o vetor
#      que ele exercita.
#      Poupado por CONDICAO 2 (nenhuma chamada de caminho no corpo).
#
# ============================================================================
# TIER 1 e TIER 2 — por que a forma dividida tambem e caçada
# ============================================================================
#   TIER 1 (mesma linha): a expansao aparece na REGIAO DE ARGUMENTO de uma
#   chamada de caminho. Regiao delimitada por CASAMENTO DE PARENTESES com
#   consciencia de aspas, nao por regex de linha.
#     Pega os 6 sitios do ML-0A: `open('$ROOT_DIR/npm/package.json')`,
#     `json.load(open('$manifest'))`, `open('$GO_API_FILE')`, `open('$VULN_GO','w')`.
#
#   TIER 2 (forma dividida): variavel Python ligada a um LITERAL DE STRING que
#   contem expansao, e essa variavel aparece depois na regiao de argumento de
#   uma chamada de caminho, NO MESMO CORPO.
#       p = '$DIR/x'
#       open(p)
#   🔴 O TIER 2 existe porque o ML-0A §2.3 mediu que a varredura de mesma
#   linha "acerta por uma COINCIDENCIA DA ARVORE — nenhum defeito atual tem a
#   interpolacao e o open() em linhas separadas. Como floor, era fragil."
#   A varredura F do ML-0A mediu 0 sitios desta forma HOJE; o tier 2 existe
#   para o amanha, e e o unico braco do gate sem sitio real que o exercite.
#
#   ESTREITAMENTO DELIBERADO DO TIER 2 — ligacao a LITERAL DE STRING, UM SALTO,
#   sem transitividade. Sem ele, `check-validate-rule-pins.sh:371` viraria
#   FALSO POSITIVO VIVO se alguem trocasse `<<'PY'` por `<<PY`: ali o valor
#   interpolado entra num DICT (`d = {… '$PWD/…'}`), e o `d` vai para
#   `json.dump(d, f)` — o caminho aberto e `sys.argv[1]`, nao `d`.
#   Por isso: (i) so rastreia RHS que COMECA com literal de string; um RHS que
#   comeca com `{`/`[` nao contamina; (ii) `json.load`/`json.dump`/`.read()`/
#   `.write()` NAO estao na lista de chamadas de caminho — `json.load(open('$m'))`
#   ja e pego pelo `open(` interno.
#
# ============================================================================
# FORMAS NAO COBERTAS (declaradas com a razao MEDIDA — "nao medido" nao vale)
# ============================================================================
#   Todas as medicoes abaixo sao do ML-0A (parecer §3, dez varreduras + leitura
#   das 39 linhas de corpo Python com `$`), nao refeitas por aproximacao aqui.
#
#   * CAMINHO CONSTRUIDO POR EXPRESSAO — `os.path.join('$DIR', 'x')` em posicao
#     que o tier 1 pega, mas `p = os.path.join(a, b)` seguido de `open(p)` com
#     a expansao em `a` NAO: o tier 2 so rastreia ligacao a LITERAL DE STRING.
#     MEDIDO (varredura E do ML-0A): `os\.path\.join\([^)]*\$|Path\([^)]*\$`
#     -> VAZIO. Custo de nao cobrir hoje = 0. Reconhecivel so por leitura.
#
#   * CORPO PYTHON GUARDADO EM VARIAVEL SHELL — `CODE='…'` … `python3 -c "$CODE"`.
#     O corpo nao esta no sitio da invocacao. MEDIDO (varredura G do ML-0A):
#     `STRIP_TS`, `STRIP_TS_TRUST`, `STRIP_TS_GATES` — os tres em ASPAS SIMPLES
#     e SEM caminho; `python3 -c "$(…)"` -> vazio. Sitios reais: 0.
#
#   * CAPTURA INDIRETA — `python3` chamado dentro de uma funcao shell cujo corpo
#     mora noutro arquivo. Exigiria grafo de chamadas. Mesmo limite declarado
#     pelos gates irmaos check-crlf-normalize-capture.sh ($(func_calling_python3))
#     e check-unguarded-capture-rc.sh.
#
#   * ARQUIVOS `.md` — fora do corpus (ML-0A §3.2, residual 2). A varredura H
#     cobriu os templates de PRODUTO (`internal/**/*.go`, `*.tmpl`) e voltou
#     limpa: `claudemd.go:258`, `scaffold.go:901,902,2102` usam
#     `pathlib.Path('.')` literal ou `json.load(sys.stdin)`. 🔴 O defeito NAO e
#     distribuido ao consumidor — medido, nao presumido. Um `.md` que venha a
#     ser materializado em script no futuro nao esta sob este criterio.
#
#   * VARIAVEL DE AMBIENTE — `HOME="$dir" python3 -c "… os.environ['HOME'] …"`.
#     (b) POR CONSTRUCAO: o MSYS converte valor de env var por ser token
#     INTEIRO. MEDIDO (varredura I do ML-0A, e nota de vault
#     `msys-nao-converte-caminho-embutido-em-string-maior-2026-09-07.md`).
#     Acusar essa forma seria falso positivo.
#
#   * `.ps1` — `scripts/windows-repro/*.ps1`. PowerShell nao passa por MSYS, e
#     o corpo Python vai em ARQUIVO. MEDIDO (varredura J do ML-0A): nada.
#
#   * ✅ NAO e residual — FECHADO por este gate: "diretorio novo". O ML-0A §3.2
#     (residual 3) declarou que nao havia gate impedindo o padrao de nascer fora
#     de `scripts/`, e atribuiu o fechamento AO ML-1B. O corpus aqui e TODO
#     `*.sh` rastreado + `.github/workflows/*.y{a,}ml`, em qualquer diretorio
#     (ver secao CORPUS no corpo). Custo medido: 0 arquivos novos entram hoje.
#     Ficam de fora so `*/testdata/*` (corpus congelado, nunca executado).
#
#   * ASPAS DESBALANCEADAS no corpo — o casamento de parenteses trata aspas;
#     um corpo com aspas desbalanceadas faz a regiao de argumento ir ate o fim
#     da linha, o que e CONSERVADOR (mais larga, nunca mais estreita).
#
# ============================================================================
# NAO-VACUIDADE — dois pisos, e o segundo e o que carrega peso
# ============================================================================
#   Um gate que examina zero e reporta sucesso e o defeito que originou duas
#   REQs deste projeto. Aqui a contagem e sobre o que o discriminante JA
#   ENXERGA, nao sobre violacoes (que devem tender a zero por sucesso).
#
#   PISO 1 — CORPOS PYTHON reconhecidos (expansiveis + literais). Cai a zero se
#            o detector de abertura de bloco quebrar.
#   PISO 2 — CORPOS EXPANSIVEIS, o subconjunto que o predicado de violacao pode
#            examinar. 🔴 E o load-bearing: um classificador de citacao quebrado
#            (que declare tudo LITERAL) deixa o PISO 1 alto e leva o numero de
#            corpos EXAMINADOS a zero — exatamente a vacuidade silenciosa.
#
#   COMANDO QUE PRODUZIU OS NUMEROS (cole e reproduza):
#     INTERP_PATH_GATE_MIN_BODIES=0 INTERP_PATH_GATE_MIN_EXPANDING=0 \
#       bash scripts/check-interpolated-path-in-python.sh
#
#   MEDICAO 2026-09-24, arvore ja corrigida pelo ML-1A (70 arquivos):
#     Corpos Python reconhecidos : 140      dos quais EXPANSIVEIS : 83
#   SEGUNDO CAMINHO, independente do awk deste gate (corrobora os 140):
#     cat scripts/*.sh .github/workflows/*.yml | grep -vE '^[[:space:]]*#' \
#       | grep -cE "(python3|python|\\\$PY_BIN)[^|]*(-c[[:space:]]*[\"']|<<-?[[:space:]]*[\"']?[A-Za-z_])"
#     -> 140  (identico; contagem conferida por dois caminhos, como manda a campanha)
#
#   🔴 OS PISOS ESTAO ABAIXO DO PIOR DE DOIS CENARIOS MEDIDOS, nao do atual:
#     cenario A — arvore de hoje (ML-1A usou `-c "` + argv) : 140 corpos / 83 expansiveis
#     cenario B — os 5 blocos do ML-1A migram para heredoc
#                 CITADO (`<<'PY'`, precedente :176)        : 130 corpos / 70 expansiveis
#   O cenario B e real: se alguem preferir o precedente :176 ao :167, ate 5
#   corpos saem de EXPANSIVEL e entram em LITERAL, e um piso calibrado so pelo
#   cenario A comecaria a reprovar por CALIBRAGEM, nao por defeito.
#   Pisos: MIN_BODIES=100 (77% de 130) · MIN_EXPANDING=55 (79% de 70).
#
#   FALSIFICACAO DAS DUAS GUARDAS (medida, nao presumida — 2026-09-24):
#     corpus vazio (1 script sem Python)    -> as DUAS reprovam (0 e 0)
#     classificador de citacao quebrado
#       (todo `-c "` virando `<<'PY'`)      -> 65 corpos / 5 expansiveis;
#                                             o PISO 2 reprova, e e ele que
#                                             carrega o peso nesse cenario
#
# ============================================================================
# STRINGS DE DIAGNOSTICO (assert_fails_with casa com grep -qF: literal e
# contiguo — nenhuma pode ser subsequencia contigua de outra)
# ============================================================================
#   Violacao tier 1 : "caminho interpolado no texto do programa Python"
#   Violacao tier 2 : "variavel Python ligada a caminho interpolado"
#   Vacuidade corpos: "guarda de vacuidade disparou"
#   Vacuidade expans: "nenhum corpo expansivel para examinar"
#
# AUTO-REFERENCIA: este arquivo e excluido da propria varredura (SELF_NAME).
# As formas acima aparecem aqui so em comentario e em ERE, nunca como codigo.
# ============================================================================

set -uo pipefail

# Mencao MORTA, nao invocacao: `python3` aparece neste arquivo so em ERE e em
# comentario. Este export satisfaz check-output-encoding-declared.sh, cujo
# discriminante e deliberadamente de ARQUIVO e conservador (trade-off
# documentado la, linhas ~207-212) — mesmo caso de
# check-crlf-normalize-capture.sh.
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SELF_NAME="check-interpolated-path-in-python.sh"
SCAN_ROOT="${INTERP_PATH_GATE_SCAN_ROOT:-$REPO_ROOT}"
MIN_BODIES="${INTERP_PATH_GATE_MIN_BODIES:-100}"
MIN_EXPANDING="${INTERP_PATH_GATE_MIN_EXPANDING:-55}"

while [ $# -gt 0 ]; do
  case "$1" in
    --scan-root) SCAN_ROOT="$2"; shift 2 ;;
    *) echo "check-interpolated-path-in-python: argumento desconhecido: $1" >&2; exit 2 ;;
  esac
done

# ---------------------------------------------------------------------------
# Varredor. Emite registros em stdout, um por linha:
#   BODY|<arquivo>|<linha-de-abertura>|EXP|LIT
#   VIOL|<arquivo>|<linha>|1|2|<texto>
# ---------------------------------------------------------------------------
SCANNER='
# --- regiao de argumento por casamento de parenteses, com consciencia de aspas
function arg_region(s, openpos,   i, depth, ch, q, out, n) {
  n = length(s); depth = 0; q = ""; out = "";
  for (i = openpos; i <= n; i++) {
    ch = substr(s, i, 1);
    if (q != "") { out = out ch; if (ch == q) q = ""; continue; }
    if (ch == "\047" || ch == "\"") { q = ch; out = out ch; continue; }
    if (ch == "(") { depth++; if (depth == 1) continue; }
    else if (ch == ")") { depth--; if (depth == 0) return out; }
    if (depth >= 1) out = out ch;
  }
  return out;   # desbalanceado -> conservador: vai ate o fim
}

# --- a linha tem expansao de shell dentro da regiao de argumento de uma
#     chamada de caminho? devolve a regiao, ou "" .
function io_arg(s,   rest, off, m, p, reg) {
  rest = s; off = 0;
  while (match(rest, CALL_RE)) {
    p = off + RSTART + RLENGTH - 1;            # posicao do "("
    reg = arg_region(s, p);
    if (reg != "") { LAST_REG = reg; return reg }
    off = off + RSTART + RLENGTH - 1;
    rest = substr(s, off + 1);
  }
  return "";
}

# --- percorre TODAS as chamadas de caminho da linha, testando um predicado
function scan_calls(s, mode,   rest, off, p, reg, v) {
  rest = s; off = 0;
  while (match(rest, CALL_RE)) {
    p = off + RSTART + RLENGTH - 1;
    reg = arg_region(s, p);
    if (mode == "EXPAND") {
      if (reg ~ /\$[A-Za-z_{(]/) return reg;
    } else {
      for (v in TAINT) {
        if (reg ~ ("(^|[^A-Za-z0-9_.])" v "([^A-Za-z0-9_]|$)")) return v "  ->  " reg;
      }
    }
    off = p; rest = substr(s, off + 1);
  }
  return "";
}

function flush_body(   k, reg, m, name, rhs) {
  if (NB == 0) { return }
  print "BODY|" FILENAME "|" BSTART "|" BKIND;
  if (BKIND == "LIT") { NB = 0; delete TAINT; return }

  # TIER 1 — mesma linha
  for (k = 1; k <= NB; k++) {
    reg = scan_calls(BL[k], "EXPAND");
    if (reg != "") {
      print "VIOL|" FILENAME "|" BLN[k] "|1|" BL[k];
      T1[k] = 1;
    }
  }
  # TIER 2 — ligacao a literal de string com expansao, um salto
  delete TAINT;
  for (k = 1; k <= NB; k++) {
    if (match(BL[k], /^[ \t]*[A-Za-z_][A-Za-z0-9_]*[ \t]*=[ \t]*[rbfuRBFU]*("[^"]*\$|\047[^\047]*\$)/)) {
      m = substr(BL[k], RSTART, RLENGTH);
      sub(/^[ \t]*/, "", m); sub(/[ \t]*=.*$/, "", m);
      if (m != "") TAINT[m] = 1;
    }
  }
  for (k = 1; k <= NB; k++) {
    if (T1[k]) continue;
    reg = scan_calls(BL[k], "TAINT");
    if (reg != "") print "VIOL|" FILENAME "|" BLN[k] "|2|" BL[k] "   [" reg "]";
  }
  NB = 0; delete TAINT; delete T1;
}

BEGIN {
  CALL_RE = "(^|[^A-Za-z0-9_])(open|chdir|listdir|makedirs|mkdir|rmdir|unlink|rename|scandir|walk|glob|iglob|rmtree|copytree|copyfile|copy|copy2|move|read_text|write_text|read_bytes|write_bytes|ZipFile|TarFile|symlink|samefile|touch|realpath|abspath|expanduser|isfile|isdir|islink|getsize|getmtime|stat|lstat|remove|Path|PurePath|PosixPath|WindowsPath|os\\.path\\.join|posixpath\\.join|ntpath\\.join)[ \t]*\\(";
  PYI = "(^|[^A-Za-z0-9_/.-])(python3|python|\\$PY_BIN|\\$\\{PY_BIN\\}|\"\\$PY_BIN\")([ \t]|$)";
  INBODY = 0; NB = 0;
}

FNR == 1 { if (INBODY) flush_body(); INBODY = 0; NB = 0 }

{
  line = $0;

  if (INBODY) {
    if (TERMKIND == "heredoc") {
      t = line; sub(/^[ \t]*/, "", t); sub(/[ \t]*$/, "", t);
      if (t == TERMWORD) { flush_body(); INBODY = 0; next }
    } else {
      # fechamento de -c "…" / -c \047…\047 : aspas na primeira coluna util
      if (line ~ ("^[ \t]*" TERMQ)) { flush_body(); INBODY = 0; next }
    }
    NB++; BL[NB] = line; BLN[NB] = FNR;
    next;
  }

  # linha de comentario puro nunca abre corpo
  if (line ~ /^[ \t]*#/) next;
  if (line !~ PYI) next;

  # ---- abertura por HEREDOC
  if (match(line, /<<-?[ \t]*("[A-Za-z_][A-Za-z0-9_]*"|\047[A-Za-z_][A-Za-z0-9_]*\047|[A-Za-z_][A-Za-z0-9_]*)/)) {
    d = substr(line, RSTART, RLENGTH);
    sub(/^<<-?[ \t]*/, "", d);
    quoted = (d ~ /^["\047]/);
    gsub(/["\047]/, "", d);
    INBODY = 1; TERMKIND = "heredoc"; TERMWORD = d;
    BKIND = quoted ? "LIT" : "EXP"; BSTART = FNR; NB = 0;
    next;
  }

  # ---- abertura por -c
  if (match(line, /-c[ \t]*"/))      { q = "\""; }
  else if (match(line, /-c[ \t]*\047/)) { q = "\047"; }
  else next;

  qpos  = RSTART + RLENGTH;           # posicao logo APOS a aspa de abertura
  rest  = substr(line, qpos);
  if (index(rest, q) > 0) {
    # corpo de UMA LINHA: recorta ate a ULTIMA ocorrencia da aspa
    last = 0;
    for (i = length(rest); i >= 1; i--) { if (substr(rest, i, 1) == q) { last = i; break } }
    body = substr(rest, 1, last - 1);
    BKIND = (q == "\"") ? "EXP" : "LIT"; BSTART = FNR;
    NB = 1; BL[1] = body; BLN[1] = FNR;
    flush_body();
    next;
  }
  # corpo MULTI-LINHA
  INBODY = 1; TERMKIND = "quote"; TERMQ = q;
  BKIND = (q == "\"") ? "EXP" : "LIT"; BSTART = FNR; NB = 0;
  next;
}

END { if (INBODY) flush_body() }
'

# ---------------------------------------------------------------------------
# Corpus
# ---------------------------------------------------------------------------
# CORPUS = TODO `*.sh` + `.github/workflows/*.y{a,}ml`, em QUALQUER diretorio.
# Nao so `scripts/`: o ML-0A §3.2 (residual 3) deixou explicito que "nao ha gate
# hoje que impeca o padrao de nascer num diretorio novo. ISSO E TRABALHO DO
# ML-1B". Custo de fechar, medido em 2026-09-24:
#   git ls-files '*.sh' | grep -v '^scripts/[^/]*\.sh$' | grep -v '/testdata/'  -> 0
# Zero arquivos hoje; a largura e de graca e o diretorio novo ja nasce coberto.
#
# 🔴 `git ls-files`, NAO varredura da arvore de trabalho — a nota de vault
# `gate-deriva-sitio-de-arvore-de-trabalho-em-vez-de-git-ls-files-2026-09-09.md`
# mede o custo: `pypi/build/lib/...`, ignorado pelo .gitignore e materializado
# por `pip install`, entrava na contagem e o CI divergia do local. Arvores
# sinteticas (fixtures do falsify, --scan-root em /tmp) nao sao repositorio;
# para elas o fallback e `find`, e ali nao existe artefato de build para poluir.
#
# `*/testdata/*` fica FORA: corpus congelado, nunca executado — mesma exclusao
# que e o discriminante de check-orphan-gates.sh.
CANDIDATES=()
if git -C "$SCAN_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  CORPUS_SOURCE="git ls-files"
  while IFS= read -r rel; do
    [ -n "$rel" ] && CANDIDATES+=("$SCAN_ROOT/$rel")
  done < <(git -C "$SCAN_ROOT" ls-files -- '*.sh' '.github/workflows/*.yml' '.github/workflows/*.yaml' 2>/dev/null)
else
  CORPUS_SOURCE="find (arvore sem git)"
  while IFS= read -r p; do
    [ -n "$p" ] && CANDIDATES+=("$p")
  done < <(find "$SCAN_ROOT" \( -name .git -o -name node_modules \) -prune -o \
             \( -name '*.sh' -o -path '*/.github/workflows/*.yml' -o -path '*/.github/workflows/*.yaml' \) \
             -print 2>/dev/null)
fi

FILES=()
for f in ${CANDIDATES[@]+"${CANDIDATES[@]}"}; do
  [ -f "$f" ] || continue
  case "$f" in */testdata/*) continue ;; esac
  [ "$(basename "$f")" = "$SELF_NAME" ] && continue
  FILES+=("$f")
done

echo "=== check-interpolated-path-in-python: ${#FILES[@]} arquivo(s) sob $SCAN_ROOT (corpus por $CORPUS_SOURCE) ==="
echo ""

RECORDS=""
if [ "${#FILES[@]}" -gt 0 ]; then
  RECORDS=$(awk "$SCANNER" "${FILES[@]}" || true)
fi

FAIL=0
TOTAL_BODIES=0
EXPANDING=0
LITERAL=0
VIOLATIONS=0

while IFS='|' read -r kind file lineno tag text; do
  [ -z "$kind" ] && continue
  base=$(basename "$file")
  case "$kind" in
    BODY)
      TOTAL_BODIES=$((TOTAL_BODIES + 1))
      if [ "$tag" = "EXP" ]; then
        EXPANDING=$((EXPANDING + 1))
      else
        LITERAL=$((LITERAL + 1))
        echo "OK   [literal/$base:$lineno] corpo Python com citacao LITERAL — o \$ chega intacto ao Python, o shell nao expande"
      fi
      ;;
    VIOL)
      VIOLATIONS=$((VIOLATIONS + 1))
      FAIL=1
      if [ "$tag" = "1" ]; then
        echo "FAIL [$base:$lineno] caminho interpolado no texto do programa Python: ${text# }"
      else
        echo "FAIL [$base:$lineno] variavel Python ligada a caminho interpolado, usada como caminho: ${text# }"
      fi
      echo "     COMO CORRIGIR: passe o caminho por argumento, nunca dentro do texto do programa."
      echo "     O MSYS/Git-Bash converte caminho POSIX -> Windows em \`argv\`, e NAO dentro de string de codigo."
      echo "         antes:  python3 -c \"… open('\$VAR') …\""
      echo "         depois: python3 -c \"… open(sys.argv[1]) …\" \"\$VAR\""
      echo "     Precedente VIVO nesta arvore: scripts/check-thirdparty-parity.sh:167"
      echo "         json.load(open(sys.argv[1]))  +  o caminho como argumento"
      echo "     Heredoc: scripts/check-thirdparty-parity.sh:176 — delimitador CITADO (<<'PY') + sys.argv."
      ;;
  esac
done <<< "$RECORDS"

# ---------------------------------------------------------------------------
# Guarda de nao-vacuidade — dois pisos
# ---------------------------------------------------------------------------
echo ""
echo "=== guarda de nao-vacuidade ==="
echo "Corpos Python reconhecidos      : $TOTAL_BODIES  (piso $MIN_BODIES)"
echo "  dos quais EXPANSIVEIS         : $EXPANDING  (piso $MIN_EXPANDING) <- o subconjunto examinavel"
echo "  dos quais LITERAIS            : $LITERAL"
echo "Violacoes                       : $VIOLATIONS"

if [ "$TOTAL_BODIES" -lt "$MIN_BODIES" ]; then
  echo "FAIL guarda de vacuidade disparou: $TOTAL_BODIES corpo(s) Python reconhecido(s), piso e $MIN_BODIES — corpus vazio, podado, ou o detector de abertura de bloco quebrou; use INTERP_PATH_GATE_MIN_BODIES para arvores sinteticas"
  FAIL=1
else
  echo "OK   vacuidade (corpos): $TOTAL_BODIES >= $MIN_BODIES"
fi

if [ "$EXPANDING" -lt "$MIN_EXPANDING" ]; then
  echo "FAIL nenhum corpo expansivel para examinar: $EXPANDING abaixo do piso $MIN_EXPANDING — o classificador de citacao pode ter quebrado e declarado tudo LITERAL, que e a vacuidade silenciosa deste gate; use INTERP_PATH_GATE_MIN_EXPANDING para arvores sinteticas"
  FAIL=1
else
  echo "OK   vacuidade (expansiveis): $EXPANDING >= $MIN_EXPANDING"
fi

echo ""
if [ "$FAIL" -ne 0 ]; then
  echo "check-interpolated-path-in-python: FAIL"
  exit 1
fi
echo "check-interpolated-path-in-python: OK — nenhum caminho interpolado dentro do texto de programa Python"
exit 0
