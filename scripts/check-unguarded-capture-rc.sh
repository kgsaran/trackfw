#!/usr/bin/env bash
# check-unguarded-capture-rc.sh — gate irmão de check-emitting-capture-fallback.sh
# (ML-2H, ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-
#  ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md)
#
# ============================================================================
# POR QUE ESTE GATE EXISTE, E POR QUE É UM ARQUIVO PRÓPRIO
# ============================================================================
#   O gate do ML-1B (check-emitting-capture-fallback.sh) caça a captura
#   $(cmd ... || echo N) sobre comando que JÁ emite no caminho de falha. Ele
#   exige a COEXISTÊNCIA de comando emissor E fallback emissor.
#
#   A forma caçada AQUI não tem fallback nenhum:
#
#       v=$(cmd_a | grep PAT | cmd_b)        # grep em posição NÃO-FINAL, sob pipefail
#       v=$(cmd_a | grep PAT)                # grep em posição FINAL
#       v=$(grep PAT arquivo)                # sem cano nenhum
#
#   Em todas, o rc não-zero de "não casou" PROPAGA para a substituição, e sob
#   `set -e` mata o script — trocando o diagnóstico que a linha seguinte já
#   escreveu por MORTE MUDA. É a mesma raiz da Wave 1 desta REQ, na terceira
#   forma. Medido nos ML-2E/2G/2I: 10 sítios com cano (5 defeituosos) e
#   13 sítios sem cano (5 defeituosos).
#
#   🔴 Alargar o gate do ML-1B para cobrir isto foi MEDIDO e REPROVADO
#   (veredito do ML-2E, aceito pelo arquiteto): sem a exigência de fallback,
#   a regex daquele gate acusaria todo $(a | b) legítimo. Gate irmão, arquivo
#   próprio, discriminante próprio.
#
# ============================================================================
# DISCRIMINANTE
# ============================================================================
#   Violação = substituição de comando em POSIÇÃO DE ATRIBUIÇÃO — `VAR=$( … )` —
#   onde um `grep` (ou `egrep`/`fgrep`) ocupa uma posição do corpo cujo rc
#   PROPAGA para a substituição, e nenhuma guarda existe.
#
#   O rc propaga quando:
#     (a) o corpo NÃO tem cano             -> o rc do comando É o rc da substituição;
#     (b) o comando é o elo FINAL do cano  -> idem, e `pipefail` é irrelevante;
#     (c) o comando é elo NÃO-FINAL        -> só quando `pipefail` está ativo
#                                             NO ESCOPO do sítio (ver ESCOPOS).
#
#   🔴 O caso (b) é MAIS LARGO que o enunciado do ML-2H, que fala em "posição
#   não-final". Alarguei de propósito, e a razão está medida na nota de vault
#   `nem-todo-grep-em-captura-e-defeito-a-posicao-do-sitio-decide-2026-09-24.md`
#   §4: `.github/workflows/check-annotations.yml:80` foi descrito pelo ML-2E
#   como "não-final" e NÃO era — o `grep -oE '[0-9]+$'` era o elo final. Isso o
#   torna PIOR, não melhor: em posição final o `set -e` sozinho já mata, sem
#   `pipefail` participar. Um gate restrito a "não-final" deixaria voltar
#   exatamente o sítio que esta REQ acabou de corrigir.
#
# ============================================================================
# ESCOPOS — por que `pipefail` e `set -e` não são propriedades do ARQUIVO
# ============================================================================
#   A nota de vault §5 mediu o defeito do filtro do ML-2E: "`pipefail in
#   arquivo` é heurística de ARQUIVO, não de ESCOPO". Foi ela que trouxe
#   check-gates-falsify.sh:2021 de volta à lista, onde o sítio vive dentro de
#   SIMPLE_REQ_FIELD_SCRIPT — uma string entre aspas simples rodada por
#   `bash -c`, que declara `set -e` mas NÃO `pipefail`.
#
#   Este gate segmenta cada arquivo em ESCOPOS DE TEXTO DE SHELL e calcula
#   errexit/pipefail POR ESCOPO:
#     * arquivo .sh                       -> escopo base
#     * atribuição multi-linha com aspas   -> escopo aninhado (o corpo será
#       simples (VAR='...\n...\n')            rodado por `bash -c "$VAR"`)
#     * corpo de heredoc (<<EOF / <<'EOF') -> escopo aninhado
#     * bloco `run:` de .github/workflows  -> escopo próprio. Default do GitHub
#                                             Actions é `bash -e {0}` (errexit
#                                             SIM, pipefail NÃO); com `shell: bash`
#                                             explícito é `bash -eo pipefail {0}`
#                                             (os dois). Linhas `set -…` no corpo
#                                             do bloco somam a isso.
#
# ============================================================================
# 🔴 AS 6 CLASSES DE ISENÇÃO — todas MEDIDAS (ML-2G/2I), nenhuma suposta
# ============================================================================
#   1  POSIÇÃO DE ARGUMENTO — `f "$(grep … | head -5)"`.  SINTÁTICA, DECIDÍVEL.
#      O rc de uma substituição em posição de argumento é DESCARTADO; só o rc
#      do comando externo conta. Medido: `f "$(grep -o ausente <<<x)"` sob
#      `set -euo pipefail` -> sobrevive.
#      Sítio real que exercita: check-serve-api-file-security.sh (`fail … "$(grep -n …)"`).
#
#   2  GUARDA DEPOIS DO FECHA-PARÊNTESES — `v=$( … ) || true`.  SINTÁTICA.
#      Medido: `rv=$(echo x | grep -o ausente) || true` -> sobrevive, rv="".
#      É a classe que mais confunde filtro automático: o `|| true` EXISTE, mas
#      fora do corpo. Também entram aqui `&&`, e a substituição usada como
#      CONDIÇÃO (`if v=$( … ); then`), onde o rc é consumido pelo `if`.
#      Sítios reais: check-ci-workflow-pin-parity.sh (dois `rv=$(grep …) || true`).
#
#   3  `pipefail` NÃO ATRAVESSA A FRONTEIRA DO ESCOPO — corpo em `bash -c`/`sh -c`.
#      SINTÁTICA (via segmentação de escopo acima). `SHELLOPTS` não é exportado
#      em lugar nenhum desta árvore — conferido por varredura.
#      Sítio real: check-gates-falsify.sh, corpo de SIMPLE_REQ_FIELD_SCRIPT
#      (`value=$(grep -m1 "^req: " … | sed -E …)`), escopo com `set -e` e SEM
#      `pipefail` -> o elo não-final não propaga.
#
#   4  ARQUIVO/ESCOPO SEM `set -e` — `set -uo pipefail` só.  SINTÁTICA.
#      Sem errexit o rc não-zero não mata nada.
#      Sítios reais: check-orphan-gates.sh e check-raw-read-ban.sh (`set -uo pipefail`).
#
#   5  `local v=$(cmd)` NA MESMA LINHA — o rc é o do builtin `local`, que é 0.
#      SINTÁTICA. Vale igualmente para `declare`/`export`/`readonly`/`typeset`.
#      Medido: `local v=$(grep -o zzz <<<x)` sob `set -e` -> rc=0; com `local v;`
#      em linha SEPARADA -> rc=1.
#      🔴 NÃO tem sítio real nesta árvore — MEDIDO, não presumido:
#         grep -rnE 'local[[:space:]]+[A-Za-z_][A-Za-z0-9_]*=\$\(' --include='*.sh' . \
#           | grep -vc '\$(('                                    -> 0
#      🔴 E NÃO é esta a classe que isenta check-orphan-gates.sh:86,102 — ali o
#      `local` está em linha SEPARADA, logo não mascara nada; quem isenta é a
#      classe 4. Confundir as duas produz veredito certo por razão errada, que
#      é pior que errar. Provada por injeção (braço J do Cenário 199).
#
#   6  🔴 NÃO-CASAMENTO PRÉ-EXCLUÍDO POR CHECAGEM ANTERIOR QUE ENCERRA O SCRIPT.
#      🔴🔴 SEMÂNTICA — **NÃO DECIDÍVEL** POR ESTE GATE, E ELE NÃO FINGE DECIDIR.
#      Decidi-la exigiria provar inalcançabilidade de um ramo, o que nenhuma
#      varredura estática de shell faz. Esta casa já pagou uma vez por gate que
#      AFIRMA sem PROVAR — é a origem da REQ anterior.
#
#      FORMA DE ALEGAÇÃO (duas, ambas exigem razão escrita):
#        (i)  PREFERIDA, para sítios NOVOS — marcador na linha imediatamente
#             acima do sítio:
#                 # unguarded-capture-rc-allowed: <razão, na forma ESTREITA>
#        (ii) BOOTSTRAP, para os sítios que já existiam quando este gate nasceu —
#             a tabela ALLEGATIONS abaixo, chaveada por (basename, nome da
#             variável atribuída). Chave por NOME, não por número de linha:
#             as coordenadas do ML-2E/2G já se deslocaram uma vez (§7 da nota).
#
#      🔴 GUARDA DE OBSOLESCÊNCIA: toda entrada de ALLEGATIONS que NÃO casar
#      nenhum sítio REPROVA o gate. Uma alegação que não casa nada é um
#      comentário, não uma afirmação por sítio — e é a mesma classe de defeito
#      do gate que examina zero e reporta sucesso.
#
#      🔴 A razão vai na FORMA ESTREITA. A nota de vault §2 mede por quê: sob
#      `pipefail`, `grep | head -1` tem um SEGUNDO caminho não-zero — o `head`
#      fecha o cano, o `grep` recebe SIGPIPE e sai 141. "Este sítio não pode
#      morrer" é a afirmação larga, falsificável com corpus grande o bastante.
#      "O laço anterior já validou o casamento E a saída é pequena demais para
#      encher o pipe" é a estreita.
#
# ============================================================================
# FAMÍLIA DE CANDIDATOS (larga) vs. FAMÍLIA DE VIOLAÇÃO (estreita)
# ============================================================================
#   VIOLAÇÃO: só `grep` / `egrep` / `fgrep`.
#   CANDIDATO (classificado e impresso, NUNCA acusado quando não-grep):
#            grep · egrep · fgrep · `command -v` · find · `git rev-parse` ·
#            `jq -e` · diff · cmp · which · type
#
#   A família larga é deliberadamente mais larga que "sítio defeituoso" —
#   mesmo princípio do gate irmão do ML-1B. Ela existe por DOIS motivos:
#     (a) o piso de não-vacuidade não pode cair a zero quando os sítios forem
#         corrigidos ( `{ grep … || true; }` continua candidato, só deixa de
#         violar );
#     (b) 🔴 sem ela as classes 4 e 5 ficariam SEM TESTEMUNHA impressa na árvore
#         real: check-orphan-gates.sh:86,102 usa `find`, que a família estreita
#         filtraria ANTES de a classe 4 ser avaliada.
#
#   POR QUE OS NÃO-GREP NÃO SÃO ACUSADOS — medido no ML-2I, §4 da nota:
#     * `command -v` — rc não-zero significa AMBIENTE INVIÁVEL, não "não casou".
#       4 dos 6 sítios interpolam o resultado num shim gerado; com `|| true` o
#       shim nasce com `*) "" "$@"` — instrumento quebrado que ainda parece
#       funcionar. Ali o `|| true` é ATIVAMENTE NOCIVO.
#     * `find` — sai não-zero em ERRO DE ACESSO, não em "não encontrou nada".
#       Mecanismo diferente.
#   Acusá-los produziria falso positivo em 7 sítios já medidos e isentados.
#
# ============================================================================
# FORMAS NÃO COBERTAS (declaradas — "não medido" não é aceitável neste projeto)
# ============================================================================
#   * Classe 6 (acima) — semântica, não decidível. Alegação por sítio.
#
#   * `Makefile` — FORA DO CORPUS. Make roda cada linha de receita no seu
#     próprio `sh -c`, sem `set -e` e sem `pipefail`: a morte, quando há, é
#     semântica de make (falha o ALVO), não de errexit. Mecanismo diferente.
#     MEDIDO 2026-09-24, custo de não cobrir = 0:
#       grep -cE '=\$\(' Makefile              -> 17  (todas expansão de var do make)
#       grep -nE '=\$\(.*grep' Makefile | wc -l ->  0
#
#   * Crase como captura — `` v=`grep …` ``. O scanner só reconhece $( … ).
#     MEDIDO no gate irmão (ML-1B, 2026-09-23): 34 ocorrências de crase com
#     `grep` no corpus, as 34 em prosa/comentário. Sítios reais: 0.
#
#   * Captura INDIRETA — `v=$(minha_funcao)` com o `grep` dentro da função.
#     Exigiria grafo de chamadas. Mesmo limite declarado pelo gate irmão
#     check-crlf-normalize-capture.sh para $(func_calling_python3).
#
#   * `sed -n` / `awk` / `cut` — comandos que saem 0 MESMO sem produzir saída.
#     Fora da forma deste gate POR CONSTRUÇÃO: não há rc para propagar. Mesmo
#     SINTOMA (aprovação vácua), mecanismo diferente — é o ML-2J desta REQ,
#     onde o único discriminante é o CONTEÚDO, nunca o rc.
#
#   * `|` dentro de aspas. O separador de cano é reconhecido sobre o corpo com
#     as regiões entre aspas apagadas (ver strip_quoted). Aspas desbalanceadas
#     dentro de um corpo fazem a região sobrevivente ser conservadora.
#
# ============================================================================
# NÃO-VACUIDADE
# ============================================================================
#   Piso sobre CANDIDATOS — toda substituição (em qualquer posição) cujo corpo
#   invoca um comando da família larga, violando ou não.
#   Comando que produziu o número:
#     UNGUARDED_RC_GATE_MIN_CANDIDATES=0 bash scripts/check-unguarded-capture-rc.sh
#   Resultado e piso: ver a constante MIN_CANDIDATES abaixo; a medição de
#   2026-09-24 está no relatório do ML-2H e foi conferida por um SEGUNDO
#   CAMINHO independente (grep -rnE sobre o mesmo corpus).
#
# ============================================================================
# STRINGS DE DIAGNÓSTICO (assert_fails_with casa nelas — precisam ficar distintas)
# ============================================================================
#   Violação   : "captura sem guarda cujo rc propaga"
#   Vacuidade  : "guarda de vacuidade disparou"
#   Alegação   : "alegacao obsoleta"
#
# AUTO-REFERÊNCIA: este arquivo é excluído da própria varredura (SELF_NAME).
# As formas acima aparecem aqui só em comentário e em ERE, nunca como código.
# ============================================================================

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SELF_NAME="check-unguarded-capture-rc.sh"
SCAN_ROOT="${UNGUARDED_RC_GATE_SCAN_ROOT:-$REPO_ROOT}"
MIN_CANDIDATES="${UNGUARDED_RC_GATE_MIN_CANDIDATES:-60}"

# ---------------------------------------------------------------------------
# ALLEGATIONS — classe 6, bootstrap. Formato:
#     "<basename>|<nome-da-variavel>|<razao na forma ESTREITA>"
# Toda entrada que não casar nenhum sítio REPROVA o gate (guarda de obsolescência).
# ---------------------------------------------------------------------------
ALLEGATIONS=(
  # VAZIA desde o ML-2K (2026-09-24): as duas entradas de bootstrap
  # (check-agent-namespace-union.sh: alfa_ln, zulu_ln) migraram para a forma
  # PREFERIDA — marcador inline `# unguarded-capture-rc-allowed:` no proprio
  # sitio. A tabela existia porque o ML-2H estava proibido de editar arquivo
  # de produto; o ML-2K nao estava.
  # 🔴 ACHADO REGISTRADO NO RELATORIO DO ML-2K: com a tabela vazia, a guarda de
  # obsolescencia abaixo itera ZERO entradas e reporta verde SEM EXAMINAR NADA,
  # e o braco P do Cenario 199 (que usa esta tabela como unica fixture) deixa de
  # falsificar. Corrigir isso exige editar logica de gate ou o braco P — os dois
  # fora da fronteira de escrita do ML-2K.
)

FAIL=0
TOTAL_CANDIDATES=0
TOTAL_VIOLATIONS=0
SCANNED_FILES=0
declare -a EXEMPT_COUNT_KEYS=()
declare -A EXEMPT_COUNT=()
declare -A ALLEGATION_HITS=()

bump_exempt() {
  local k="$1"
  if [ -z "${EXEMPT_COUNT[$k]:-}" ]; then
    EXEMPT_COUNT[$k]=0
    EXEMPT_COUNT_KEYS+=("$k")
  fi
  EXEMPT_COUNT[$k]=$(( EXEMPT_COUNT[$k] + 1 ))
}

# ---------------------------------------------------------------------------
# strip_quoted <texto> -> ecoa o texto com o CONTEÚDO de regiões entre aspas
#   simples/duplas trocado por `_`. Usado só para achar separadores de cano e
#   fronteiras de comando; nunca para diagnóstico.
# ---------------------------------------------------------------------------
strip_quoted() {
  # NOTA: `local s="$1" n=${#s}` NAO funciona — o builtin `local` declara TODOS
  # os nomes (como unset) antes de executar as atribuicoes, entao `${#s}` na
  # mesma instrucao ve `s` UNSET e, sob `set -u`, aborta. Medido em bash 5.3.
  local s="$1"
  local out="" i=0 ch q=""
  local n=${#s}
  while [ $i -lt $n ]; do
    ch="${s:$i:1}"
    if [ -n "$q" ]; then
      # Dentro de aspas DUPLAS a barra invertida escapa o proximo caractere —
      # sem isto, `tr -d "\"'"` desalinha o rastreador de aspas e um `|| true`
      # LEGITIMO depois dele e lido como se estivesse dentro de aspas. Medido em
      # scripts/trackfw-credential-guard.sh:121 (unico falso positivo da arvore).
      if [ "$q" = '"' ] && [ "$ch" = '\' ]; then
        out+="__"; ((i+=2)); continue
      fi
      if [ "$ch" = "$q" ]; then q=""; out+="$ch"; else out+="_"; fi
    elif [ "$ch" = "'" ] || [ "$ch" = '"' ]; then
      q="$ch"; out+="$ch"
    else
      out+="$ch"
    fi
    ((i++))
  done
  printf '%s' "$out"
}

# ---------------------------------------------------------------------------
# element_command <texto-do-elo> -> ecoa o nome do comando do elo, já
#   descascado de prefixos que não são o comando (`!`, `env`, `time`, `exec`,
#   `{`, `(` e atribuições `VAR=...`). Para `command -v` e `git rev-parse`
#   ecoa o par, porque o rc só é "não achou" nessa forma.
# ---------------------------------------------------------------------------
element_command() {
  local e="$1"
  # shellcheck disable=SC2206
  local -a t=( $e )
  local i=0
  while [ $i -lt ${#t[@]} ]; do
    case "${t[$i]}" in
      '!'|env|time|exec|'{'|'('|nohup|stdbuf|builtin) ((i++));;
      *=*) ((i++));;
      *) break;;
    esac
  done
  [ $i -ge ${#t[@]} ] && { printf ''; return; }
  local cmd="${t[$i]}"
  cmd="${cmd##*/}"
  local nxt="${t[$((i+1))]:-}"
  case "$cmd" in
    command) [ "$nxt" = "-v" ] && printf 'command -v' || printf '%s' "$cmd" ;;
    git)     [ "$nxt" = "rev-parse" ] && printf 'git rev-parse' || printf '%s' "$cmd" ;;
    jq)      [ "$nxt" = "-e" ] && printf 'jq -e' || printf '%s' "$cmd" ;;
    *)       printf '%s' "$cmd" ;;
  esac
}

is_wide_family() {
  case "$1" in
    grep|egrep|fgrep|'command -v'|find|'git rev-parse'|'jq -e'|diff|cmp|which|type) return 0;;
    *) return 1;;
  esac
}
is_grep_family() {
  case "$1" in grep|egrep|fgrep) return 0;; *) return 1;; esac
}

# ---------------------------------------------------------------------------
# classify_body — preenche as globais BODY_CMDS / BODY_NELEM a partir do corpo.
#   Divide o corpo em elos de cano de nível superior: `|` que NÃO faz parte de
#   `||`, fora de aspas e fora de parênteses/chaves aninhados.
# ---------------------------------------------------------------------------
BODY_CMDS=()
BODY_GUARDED=()
BODY_NELEM=0
BODY_HAS_INNER_OR=0
split_body() {
  local body="$1"
  local masked; masked="$(strip_quoted "$body")"
  BODY_CMDS=(); BODY_GUARDED=(); BODY_NELEM=0; BODY_HAS_INNER_OR=0
  local i=0 n=${#masked} depth=0 start=0 ch nxt prv
  local -a spans=()
  while [ $i -lt $n ]; do
    ch="${masked:$i:1}"
    nxt="${masked:$((i+1)):1}"
    prv=""; [ $i -gt 0 ] && prv="${masked:$((i-1)):1}"
    case "$ch" in
      '('|'{') ((depth++));;
      ')'|'}') [ $depth -gt 0 ] && ((depth--));;
      '|')
        if [ "$nxt" = "|" ]; then
          [ $depth -eq 0 ] && BODY_HAS_INNER_OR=1
          ((i++))
        elif [ "$prv" != "|" ] && [ $depth -eq 0 ]; then
          spans+=("$start:$((i-start))")
          start=$((i+1))
        fi
        ;;
    esac
    ((i++))
  done
  spans+=("$start:$((n-start))")
  local sp off len el
  for sp in "${spans[@]}"; do
    off="${sp%%:*}"; len="${sp##*:}"
    el="${body:$off:$len}"
    BODY_CMDS+=("$(element_command "$el")")
    # Elo com `||` PROPRIO — a forma CORRETA `{ grep … || true; }` — nao propaga
    # rc. O `||` vive em profundidade 1 (dentro das chaves), logo nao aparece em
    # BODY_HAS_INNER_OR, que so conta o nivel superior.
    if [[ "${masked:$off:$len}" == *"||"* ]]; then
      BODY_GUARDED+=(1)
    else
      BODY_GUARDED+=(0)
    fi
    ((BODY_NELEM++))
  done
}

# ---------------------------------------------------------------------------
# Segmentação de escopos. Preenche LINE_SCOPE[] e SCOPE_E[] / SCOPE_PF[].
# ---------------------------------------------------------------------------
declare -a LINE_SCOPE=()
declare -a SCOPE_E=()
declare -a SCOPE_PF=()
declare -a SCOPE_KIND=()
declare -a LINE_E=()
declare -a LINE_PF=()
declare -a CUR_E=()
declare -a CUR_PF=()

segment_scopes() {
  local kind="$1"; shift
  local -n __src=$1
  local n=${#__src[@]}
  LINE_SCOPE=(); SCOPE_E=(); SCOPE_PF=(); SCOPE_KIND=()
  local nscopes=1
  SCOPE_E[0]=0; SCOPE_PF[0]=0; SCOPE_KIND[0]="file"
  local i=0 line trimmed
  local in_hd=0 hd_delim="" hd_scope=0
  local in_sq=0 sq_scope=0
  local in_run=0 run_indent=0 run_scope=0
  while [ $i -lt $n ]; do
    line="${__src[$i]}"
    if [ $in_hd -eq 1 ]; then
      trimmed="${line#"${line%%[![:space:]]*}"}"
      LINE_SCOPE[$i]=$hd_scope
      [ "$trimmed" = "$hd_delim" ] && in_hd=0
      ((i++)); continue
    fi
    if [ $in_sq -eq 1 ]; then
      LINE_SCOPE[$i]=$sq_scope
      [[ "$line" == *"'"* ]] && in_sq=0
      ((i++)); continue
    fi
    if [ $in_run -eq 1 ]; then
      local ind="${line%%[![:space:]]*}"
      if [ -z "${line//[[:space:]]/}" ] || [ ${#ind} -gt $run_indent ]; then
        LINE_SCOPE[$i]=$run_scope
        ((i++)); continue
      fi
      in_run=0
    fi
    LINE_SCOPE[$i]=0
    # heredoc abre?
    if [[ "$line" =~ \<\<-?[[:space:]]*[\'\"]?([A-Za-z_][A-Za-z0-9_]*)[\'\"]?[[:space:]]*$ ]]; then
      hd_delim="${BASH_REMATCH[1]}"
      hd_scope=$nscopes; SCOPE_E[$hd_scope]=0; SCOPE_PF[$hd_scope]=0; SCOPE_KIND[$hd_scope]="heredoc"
      ((nscopes++)); in_hd=1
    # atribuicao multi-linha entre aspas simples abre?
    elif [[ "$line" =~ ^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*=\'[^\']*$ ]]; then
      sq_scope=$nscopes; SCOPE_E[$sq_scope]=0; SCOPE_PF[$sq_scope]=0; SCOPE_KIND[$sq_scope]="quoted-script"
      ((nscopes++)); in_sq=1
    # bloco run: de workflow abre?
    elif [ "$kind" = "yaml" ] && [[ "$line" =~ ^([[:space:]]*)(-[[:space:]]+)?run:[[:space:]]*[\|\>] ]]; then
      run_indent=${#BASH_REMATCH[1]}
      run_scope=$nscopes; SCOPE_KIND[$run_scope]="run-block"
      # GitHub Actions roda `bash -e {0}` por padrao (errexit SIM, pipefail NAO);
      # com `shell: bash` explicito roda `bash -eo pipefail {0}` (os dois).
      SCOPE_E[$run_scope]=1; SCOPE_PF[$run_scope]=0
      local j=$((i-1)) back=0
      while [ $j -ge 0 ] && [ $back -lt 6 ]; do
        [[ "${__src[$j]}" =~ ^[[:space:]]*shell:[[:space:]]*bash[[:space:]]*$ ]] && { SCOPE_PF[$run_scope]=1; break; }
        [[ "${__src[$j]}" =~ ^[[:space:]]*-[[:space:]]*name: ]] && break
        ((j--)); ((back++))
      done
      ((nscopes++)); in_run=1
    fi
    ((i++))
  done
  # ------------------------------------------------------------------------
  # errexit/pipefail sao estado SEQUENCIAL, nao propriedade do arquivo: um
  # `set +e` no meio do arquivo desliga errexit para as linhas seguintes ate o
  # proximo `set -e` (check-install-version-pin.sh faz exatamente isso). Por
  # isso a opcao efetiva e calculada POR LINHA, caminhando em ordem, com um
  # estado corrente por escopo.
  # ------------------------------------------------------------------------
  local s
  for ((s=0; s<nscopes; s++)); do CUR_E[$s]=${SCOPE_E[$s]}; CUR_PF[$s]=${SCOPE_PF[$s]}; done
  LINE_E=(); LINE_PF=()
  for ((i=0; i<n; i++)); do
    line="${__src[$i]}"
    s=${LINE_SCOPE[$i]}
    if ! [[ "$line" =~ ^[[:space:]]*# ]]; then
      # ligar/desligar errexit — `set -e`, `set -euo …`, `set -o errexit`
      if [[ "$line" =~ (^|[[:space:]\;])set[[:space:]]+-[A-Za-z]*e ]] || [[ "$line" =~ (^|[[:space:]\;])set[[:space:]]+-o[[:space:]]+errexit ]]; then
        CUR_E[$s]=1
      elif [[ "$line" =~ (^|[[:space:]\;])set[[:space:]]+\+[A-Za-z]*e ]] || [[ "$line" =~ (^|[[:space:]\;])set[[:space:]]+\+o[[:space:]]+errexit ]]; then
        CUR_E[$s]=0
      fi
      # ligar/desligar pipefail — `set -o pipefail`, `set -eo pipefail`,
      # `set -euo pipefail` (o `-o` vem EMPACOTADO no pacote de flags: um ERE
      # que exija `-o` isolado NAO casa a forma mais comum desta arvore)
      if [[ "$line" =~ (^|[[:space:]\;])set[[:space:]]+-[A-Za-z]*o[[:space:]]+pipefail ]]; then
        CUR_PF[$s]=1
      elif [[ "$line" =~ (^|[[:space:]\;])set[[:space:]]+\+[A-Za-z]*o[[:space:]]+pipefail ]]; then
        CUR_PF[$s]=0
      fi
    fi
    LINE_E[$i]=${CUR_E[$s]}
    LINE_PF[$i]=${CUR_PF[$s]}
  done
}

# ---------------------------------------------------------------------------
# allegation_reason <basename> <varname> -> ecoa a razão, rc 0 se houver
# ---------------------------------------------------------------------------
# 🔴 Devolve a razao na GLOBAL ALLEG_REASON, nao em stdout: chamada dentro de
# $( ) ela rodaria em SUBSHELL e a contagem de ALLEGATION_HITS — que alimenta a
# guarda de obsolescencia — seria descartada no retorno. E a mesma raiz desta
# REQ (efeito perdido na fronteira do subshell), e ela morde de novo aqui.
ALLEG_REASON=""
allegation_reason() {
  local f="$1" v="$2" entry ef ev er
  ALLEG_REASON=""
  for entry in "${ALLEGATIONS[@]}"; do
    ef="${entry%%|*}"
    er="${entry#*|}"
    ev="${er%%|*}"
    er="${er#*|}"
    if [ "$ef" = "$f" ] && [ "$ev" = "$v" ]; then
      ALLEGATION_HITS["$ef|$ev"]=$(( ${ALLEGATION_HITS["$ef|$ev"]:-0} + 1 ))
      ALLEG_REASON="$er"
      return 0
    fi
  done
  return 1
}

emit_violation() {
  local fname="$1" lineno="$2" varname="$3" body="$4"
  echo "FAIL [$fname:$lineno] captura sem guarda cujo rc propaga: ${varname}=\$(${body})"
  echo "     O rc de 'nao casou' do grep PROPAGA para a substituicao e, sob set -e, mata o script:"
  echo "     o diagnostico que a linha seguinte escreveu para o caso vazio vira MORTE MUDA."
  echo "     CORRIJA assim:  ${varname}=\$( { grep 'PAT' \"\$F\" || true; } )"
  echo "     🔴 E ANTES DE COLAR O '|| true', RESPONDA:  \"o codigo DEPOIS da atribuicao distingue"
  echo "        VAZIO de valor legitimamente medido?\""
  echo "        DISTINGUE (ex.: if [ -z \"\$v\" ]; then ... fi)  -> a guarda de rc basta."
  echo "        NAO DISTINGUE (ex.: o valor e COMPARADO com outra captura) -> a guarda de rc SOZINHA"
  echo "        troca um falso-negativo barulhento por um FALSO-POSITIVO SILENCIOSO: duas capturas"
  echo "        vazias comparam IGUAIS e o cenario emite OK sobre medicao nenhuma. Medido no ML-2I."
  echo "        Nesse caso: guarda de rc MAIS guarda de nao-vacuidade que reprova COM ROTULO."
  echo "     Se o nao-casamento e impossivel por checagem anterior que encerra o script, alegue com"
  echo "        # unguarded-capture-rc-allowed: <razao na forma ESTREITA>   (linha acima do sitio)"
  FAIL=1
  ((TOTAL_VIOLATIONS++))
}

# ---------------------------------------------------------------------------
# scan_file
# ---------------------------------------------------------------------------
ASSIGN_RE='(^|[[:space:];&|]|do[[:space:]]|then[[:space:]])(local|declare|export|readonly|typeset)?[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=$'
DECL_RE='(^|[[:space:];&|])(local|declare|export|readonly|typeset)[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)=$'
COND_RE='(^|[[:space:];&|])(if|while|until|!)[[:space:]]'
# 🔴 `if`/`while` no prefixo NAO bastam: em `if [ -n "$x" ]; then v=$(grep …)` a
# atribuicao vem DEPOIS do `; then` e o rc PROPAGA normalmente. O teste de
# condicao so vale quando nenhum separador estrutural se interpoe entre a
# palavra-chave e o `NAME=`. Medido na arvore: 0 sitios desta forma hoje
#   grep -rnE '(^|[[:space:]])(if|while|until)[[:space:]].*(;[[:space:]]*(then|do)|&&|\|\|)[[:space:]]*[A-Za-z_][A-Za-z0-9_]*=\$\(' \
#     scripts/*.sh .github/workflows/*.yml | grep -vc check-unguarded-capture-rc.sh   -> 0
# — logo a correcao e preventiva, e o braço Q do Cenario 199 a falsifica.
COND_BREAK_RE='(;[[:space:]]*(then|do)|&&|\|\|)'

scan_file() {
  local file="$1" kind="$2"
  local fname; fname="$(basename "$file")"
  [ "$fname" = "$SELF_NAME" ] && return 0

  local -a L=()
  mapfile -t L < "$file"
  local n=${#L[@]}
  [ "$n" -eq 0 ] && { ((SCANNED_FILES++)); return 0; }

  segment_scopes "$kind" L

  ((SCANNED_FILES++))

  local i=0 col=0
  while [ $i -lt $n ]; do
    local line="${L[$i]}"
    # linha de comentario nunca inicia varredura (e nunca inicia uma juncao)
    if [[ "$line" =~ ^[[:space:]]*# ]]; then ((i++)); col=0; continue; fi

    local rest="${line:$col}"
    local pos="${rest%%\$(*}"
    if [ "$pos" = "$rest" ]; then ((i++)); col=0; continue; fi
    local at=$(( col + ${#pos} ))
    local prefix="${line:0:$at}"

    # ------------------------------------------------------------------------
    # Juncao por PARENTESES BALANCEADOS, atravessando linhas.
    #
    # 🔴 A contagem e CIENTE DE ASPAS. Sem isso, um parentese literal DENTRO do
    # padrao do grep desequilibra o contador e a substituicao inteira e
    # ENGOLIDA EM SILENCIO — falso negativo, o pior modo de erro de um gate.
    # Medido em .github/workflows/quality.yml:1254, onde o padrao e
    # "os\.Symlink(" : o contador ingenuo nunca voltava a zero e o sitio
    # (legitimamente guardado por `|| true` tres linhas abaixo) nao aparecia
    # nem como candidato.
    #
    # O estado de aspas comeca ZERADO no `(` de abertura: um `$( … )` abre um
    # contexto de citacao proprio, logo `f "$(grep …)"` e varrido corretamente
    # mesmo estando dentro de aspas duplas no texto externo.
    # ------------------------------------------------------------------------
    local depth=0 body="" j=$i k=$(( at + 1 )) found=0 suffix="" cur="$line" ch jq=""
    while [ $j -lt $n ]; do
      local m=${#cur}
      while [ $k -lt $m ]; do
        ch="${cur:$k:1}"
        if [ -n "$jq" ]; then
          if [ "$jq" = '"' ] && [ "$ch" = '\' ]; then
            body+="${cur:$k:2}"; ((k+=2)); continue
          fi
          [ "$ch" = "$jq" ] && jq=""
        elif [ "$ch" = "'" ] || [ "$ch" = '"' ]; then
          jq="$ch"
        elif [ "$ch" = "(" ]; then ((depth++));
        elif [ "$ch" = ")" ]; then
          ((depth--))
          if [ $depth -eq 0 ]; then found=1; break; fi
        fi
        [ $k -gt $(( at )) ] || [ $j -gt $i ] && body+="$ch"
        ((k++))
      done
      if [ $found -eq 1 ]; then suffix="${cur:$((k+1))}"; break; fi
      ((j++)); [ $j -ge $n ] && break
      cur="${L[$j]}"; k=0; body+=" "
      [ $(( j - i )) -gt 60 ] && break
    done

    if [ $found -ne 1 ]; then ((i++)); col=0; continue; fi
    # body comeca com '(' consumido: remove o '(' inicial que entrou na contagem
    body="${body#\(}"

    classify_site "$fname" "$((i+1))" "$prefix" "$body" "$suffix" "${LINE_SCOPE[$i]}" L "$i" "${LINE_E[$i]}" "${LINE_PF[$i]}"

    if [ $j -gt $i ]; then i=$j; col=$((k+1)); else col=$((k+1)); fi
    [ $col -ge ${#L[$i]} ] && { ((i++)); col=0; }
  done
}

classify_site() {
  local fname="$1" lineno="$2" prefix="$3" body="$4" suffix="$5" scope="$6"
  local -n __ll=$7
  local idx="$8"
  local eff_e="$9"
  local eff_pf="${10}"

  split_body "$body"

  # candidato? (qualquer posicao, familia larga)
  local c has_wide=0 has_wide_unguarded=0 has_grep=0 grep_final=0 grep_nonfinal=0 pos=0
  for c in "${BODY_CMDS[@]}"; do
    ((pos++))
    is_wide_family "$c" || continue
    has_wide=1
    [ "${BODY_GUARDED[$((pos-1))]}" -eq 1 ] && continue
    has_wide_unguarded=1
    if is_grep_family "$c"; then
      has_grep=1
      if [ $pos -eq $BODY_NELEM ]; then grep_final=1; else grep_nonfinal=1; fi
    fi
  done
  [ $has_wide -eq 0 ] && return 0
  ((TOTAL_CANDIDATES++))

  local tag=""
  # --- classe 1: posicao de argumento -------------------------------------
  local varname=""
  if [[ "$prefix" =~ $ASSIGN_RE ]]; then
    varname="${BASH_REMATCH[3]}"
  fi
  if [ -z "$varname" ]; then
    tag="exempt/class1-argument-position"
    echo "OK   [$tag/$fname:$lineno] rc da substituicao e DESCARTADO em posicao de argumento"
    bump_exempt "$tag"; return 0
  fi

  # --- classe 5: builtin de declaracao mascara o rc -----------------------
  if [[ "$prefix" =~ $DECL_RE ]]; then
    tag="exempt/class5-decl-builtin-masks-rc"
    echo "OK   [$tag/$fname:$lineno] rc e o do builtin de declaracao, sempre 0 (var $varname)"
    bump_exempt "$tag"; return 0
  fi

  # --- classe 2: guarda fora do parentese, ou rc consumido por condicao ---
  local strim="${suffix#"${suffix%%[![:space:]]*}"}"
  local is_cond=0
  if [[ "$prefix" =~ $COND_RE ]] && ! [[ "$prefix" =~ $COND_BREAK_RE ]]; then is_cond=1; fi
  if [[ "$strim" == "||"* ]] || [[ "$strim" == "&&"* ]] || [ $is_cond -eq 1 ]; then
    tag="exempt/class2-guard-outside-parens"
    echo "OK   [$tag/$fname:$lineno] guarda depois do fecha-parenteses (ou rc consumido por condicao): $varname"
    bump_exempt "$tag"; return 0
  fi
  if [ $BODY_HAS_INNER_OR -eq 1 ] || [ $has_wide_unguarded -eq 0 ]; then
    tag="exempt/guarded-inside-parens"
    echo "OK   [$tag/$fname:$lineno] guarda '||' dentro do corpo (a forma correta { cmd || true; } ou cano inteiro guardado): $varname"
    bump_exempt "$tag"; return 0
  fi

  # --- classe 4 / 3: escopo sem errexit -----------------------------------
  if [ "$eff_e" -ne 1 ]; then
    if [ "${SCOPE_KIND[$scope]}" = "file" ]; then
      tag="exempt/class4-no-errexit"
      echo "OK   [$tag/$fname:$lineno] escopo do arquivo nao tem set -e (var $varname)"
    else
      tag="exempt/class3-foreign-scope-no-errexit"
      echo "OK   [$tag/$fname:$lineno] escopo ${SCOPE_KIND[$scope]} nao tem set -e (var $varname)"
    fi
    bump_exempt "$tag"; return 0
  fi

  # --- o rc propaga? -------------------------------------------------------
  if [ $has_grep -eq 0 ]; then
    tag="exempt/semantic-family-not-grep"
    echo "OK   [$tag/$fname:$lineno] familia larga nao-grep (${BODY_CMDS[*]}): rc nao-zero significa ambiente inviavel ou erro de acesso, nao 'nao casou' — ML-2I"
    bump_exempt "$tag"; return 0
  fi
  if [ $grep_final -eq 0 ] && [ $grep_nonfinal -eq 1 ] && [ "$eff_pf" -ne 1 ]; then
    tag="exempt/class3-no-pipefail-in-scope"
    echo "OK   [$tag/$fname:$lineno] grep em elo NAO-FINAL e o escopo ${SCOPE_KIND[$scope]} nao tem pipefail (var $varname)"
    bump_exempt "$tag"; return 0
  fi

  # --- classe 6: alegacao por sitio ---------------------------------------
  local marker="" reason=""
  local p=$((idx-1))
  while [ $p -ge 0 ]; do
    local pl="${__ll[$p]}"
    [ -z "${pl//[[:space:]]/}" ] && { ((p--)); continue; }
    if [[ "$pl" =~ unguarded-capture-rc-allowed:[[:space:]]*(.*)$ ]]; then marker="${BASH_REMATCH[1]}"; fi
    break
  done
  if [ -n "$marker" ]; then
    tag="exempt/class6-alleged-inline"
    echo "OK   [$tag/$fname:$lineno] alegacao inline ($varname): $marker"
    bump_exempt "$tag"; return 0
  fi
  if allegation_reason "$fname" "$varname"; then
    tag="exempt/class6-alleged-table"
    reason="$ALLEG_REASON"
    echo "OK   [$tag/$fname:$lineno] alegacao da tabela ($varname): $reason"
    bump_exempt "$tag"; return 0
  fi

  emit_violation "$fname" "$lineno" "$varname" "$body"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --scan-root) SCAN_ROOT="$2"; shift 2;;
    *) echo "check-unguarded-capture-rc: argumento desconhecido: $1" >&2; exit 2;;
  esac
done

echo "=== check-unguarded-capture-rc: varrendo scripts/*.sh e .github/workflows/* sob $SCAN_ROOT ==="
echo ""

shopt -s nullglob
for f in "$SCAN_ROOT/scripts/"*.sh; do
  [ -f "$f" ] || continue
  case "$f" in */testdata/*) continue;; esac
  scan_file "$f" sh
done
for f in "$SCAN_ROOT/.github/workflows/"*.yml "$SCAN_ROOT/.github/workflows/"*.yaml; do
  [ -f "$f" ] || continue
  scan_file "$f" yaml
done
shopt -u nullglob

echo ""
echo "=== guarda de vacuidade ==="
echo "Candidatos (substituicao cujo corpo invoca a familia larga): $TOTAL_CANDIDATES"
for k in "${EXEMPT_COUNT_KEYS[@]}"; do
  printf '  %-42s %s\n' "$k" "${EXEMPT_COUNT[$k]}"
done
echo "Violacoes: $TOTAL_VIOLATIONS"
echo "Arquivos varridos: $SCANNED_FILES"
echo "Piso MIN_CANDIDATES: $MIN_CANDIDATES"

if [ "$TOTAL_CANDIDATES" -lt "$MIN_CANDIDATES" ]; then
  echo "FAIL guarda de vacuidade disparou: $TOTAL_CANDIDATES candidato(s), piso e $MIN_CANDIDATES — o corpus pode estar vazio ou podado; use UNGUARDED_RC_GATE_MIN_CANDIDATES para arvores sinteticas"
  FAIL=1
else
  echo "OK   vacuidade: $TOTAL_CANDIDATES >= $MIN_CANDIDATES"
fi

# ---------------------------------------------------------------------------
# Guarda de obsolescencia das alegacoes (classe 6): alegacao que nao casa nada
# e comentario, nao afirmacao por sitio. So se aplica na arvore real — numa
# arvore sintetica os sitios alegados nao existem por construcao.
# ---------------------------------------------------------------------------
# UNGUARDED_RC_GATE_FORCE_ALLEGATION_GUARD=1 liga a guarda tambem em arvore
# sintetica — sem isso ela seria INFALSIFICAVEL, que e a mesma classe de defeito
# que este gate existe para atacar (guarda que nunca pode reprovar nao e guarda).
if [ "$SCAN_ROOT" = "$REPO_ROOT" ] || [ "${UNGUARDED_RC_GATE_FORCE_ALLEGATION_GUARD:-0}" = "1" ]; then
  echo ""
  echo "=== guarda de obsolescencia das alegacoes (classe 6) ==="
  for entry in "${ALLEGATIONS[@]}"; do
    ef="${entry%%|*}"; er="${entry#*|}"; ev="${er%%|*}"
    hits="${ALLEGATION_HITS["$ef|$ev"]:-0}"
    if [ "$hits" -eq 0 ]; then
      echo "FAIL alegacao obsoleta: ($ef, $ev) nao casa nenhum sitio — o sitio foi corrigido, renomeado ou removido; apague a entrada de ALLEGATIONS no mesmo commit"
      FAIL=1
    else
      echo "OK   alegacao viva: ($ef, $ev) casa $hits sitio(s)"
    fi
  done
fi

echo ""
if [ "$FAIL" -ne 0 ]; then
  echo "check-unguarded-capture-rc: FAIL"
  exit 1
fi
echo "check-unguarded-capture-rc: OK — nenhuma captura sem guarda com rc que propaga"
exit 0
