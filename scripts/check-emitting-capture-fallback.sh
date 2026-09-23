#!/usr/bin/env bash
# check-emitting-capture-fallback.sh — gate anti-reintrodução de ML-1B
# (ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md)
#
# POR QUE ISTO EXISTE:
#   A apuração do censo de Windows morreu no primeiro shard SEM falha por causa
#   desta forma de captura:
#
#       CNT=$(grep -ac '^FAIL' "$LOG" 2>/dev/null || echo 0)
#
#   `grep -c` JÁ IMPRIME `0` quando não casa nada — e sai com 1. O rc 1 dispara
#   o `|| echo 0`, que acrescenta um SEGUNDO `0`. A variável fica $'0\n0' e o
#   `$(( ))` seguinte quebra com "arithmetic syntax error", matando o laço na
#   primeira iteração de shard limpo.
#
#   Medido em bash (macOS/ARM64, 2026-09-23), nas duas direções:
#     sem match  -> $'0\n0'  -> $(( )) arithmetic syntax error
#     com match  -> 2        -> ok        (controle)
#     { grep -ac ... || true; } -> 0      (forma correta)
#
#   🔴 Nota sobre o caminho de erro que NÃO produz o defeito:
#   `grep -c PAT /arquivo/ausente` sai com rc=2 e stdout VAZIO. Logo a captura
#   viraria apenas o `0` do fallback. O $'0\n0' nasce exclusivamente do caminho
#   de NÃO-CASAMENTO (rc=1 com stdout "0") — que é justamente o caminho do
#   shard limpo, o único que o censo precisava contar corretamente.
#
#   🔴 Contagem cruzada NÃO protege contra isto: a defesa que já existia no
#   censo comparava `grep` com `awk` e reportou `grep=0\n0 awk=0`, uma
#   divergência inexistente. O que se corrige é a FORMA DE CAPTURA.
#
# DISCRIMINANTE ESTRUTURAL:
#   Violação = uma substituição de comando $( ... ) em que coexistem:
#     (1) COMANDO QUE EMITE NO CAMINHO DE FALHA — `grep` com `-c` no pacote de
#         flags (`-c`, `-ac`, `-rc`, `-a -c`) ou `--count`; e
#     (2) FALLBACK QUE TAMBÉM EMITE — `|| echo ...` ou `|| printf ...`.
#   Quando (1) e (2) coexistem, a captura recebe DUAS linhas onde o autor
#   esperava uma. A correção é trocar o fallback por um que não emite:
#       CNT=$( { grep -ac '^FAIL' "$LOG" || true; } )
#
#   Se (1) for falso (o comando não escreve em stdout ao falhar), `|| echo N`
#   é a forma CORRETA e o sítio é ISENTO. Medido em bash, 2026-09-23:
#     grep -c   / -ac / --count / -a -c  sem match -> rc=1  stdout="0"   EMITE
#     grep sem -c                        sem match -> rc=1  stdout=""    não emite
#     wc -l </ausente                              -> rc=1  stdout=""    não emite
#     jq length /ausente                           -> rc=2  stdout=""    não emite
#     sort /ausente | uniq -c /ausente | cat | find-> rc=1/2 stdout=""   não emite
#   Comando que produziu a tabela (bash explícito — zsh não é o mesmo shell):
#     probe() { local out rc; out=$( "$@" 2>/dev/null ); rc=$?; \
#               printf '%s rc=%s stdout=%q\n' "$1" "$rc" "$out"; }
#     printf 'alpha\nbeta\n' > f.txt
#     probe grep -c ZZZ f.txt ; probe grep -ac ZZZ f.txt ; probe grep --count ZZZ f.txt
#     probe grep -a -c ZZZ f.txt ; probe grep ZZZ f.txt ; probe wc -l /nao/existe
#     probe jq length /nao/existe ; probe sort /nao/existe ; probe cat /nao/existe
#
# FORMAS COBERTAS (cada uma provada por injeção — check-gates-falsify.sh, Cenário 198):
#   C1  $(grep -c PAT f || echo 0)                — flag isolada
#   C2  $(grep -ac PAT f 2>/dev/null || echo "0") — flag empacotada + redirect + aspas
#   C3  $(grep --count PAT f || echo 0)           — flag longa
#   C4  $(grep -a -c PAT f || printf '0\n')       — -c separada + fallback printf
#   C5  $(cat f | grep -c PAT || echo 0)          — grep no fim de um pipeline
#   C6  a mesma forma dentro de .github/workflows/*.yml (o sítio real do defeito)
#
# FORMAS NÃO COBERTAS (resíduos DECLARADOS — "não medido" não é aceitável neste
#                      projeto; abaixo o limite é medido e o comando está colado):
#   `crase` como captura — `` `grep -c ... || echo 0` ``
#       Não coberta: o scanner só reconhece $( ... ). MEDIDO 2026-09-23:
#       34 ocorrências de crase com `grep` no corpus, e as 34 são prosa —
#       crase de citação em comentário/markdown, nenhuma é captura POSIX.
#       ML-0A §2.4.3 mediu o mesmo por outro caminho (só markdown e
#       interpolação PowerShell `n). Sítios reais não cobertos: 0.
#       Comando: grep -rn '`[^`]*grep[^`]*`' .github/workflows/ scripts/ Makefile \
#                  | grep -v testdata | wc -l          -> 34 (todas em comentário)
#
#   Continuação de linha — o `||` numa linha e a captura na anterior (`\` no fim)
#       Não coberta: o scanner é orientado a linha, como todo `grep`. Declarada
#       por ML-0A §2.4.1, que também não varreu por ela. MEDIDO 2026-09-23:
#       28 capturas terminam a linha em `\`, e NENHUMA delas contém `grep`.
#       Sítios reais não cobertos: 0.
#       Comandos: grep -rnE '\$\([^()]*\\$' .github/workflows/ scripts/ Makefile \
#                   | grep -v testdata | wc -l                     -> 28
#                 grep -rnE '\$\([^()]*grep[^()]*\\$' .github/workflows/ scripts/ Makefile \
#                   | grep -v testdata | wc -l                     ->  0
#
#   Captura ANINHADA — $( ... $( ... ) ... || echo 0)
#       Não coberta: a extração usa [^()]* e por construção não entra em
#       substituição aninhada. Declarada por ML-0A §2.4.2. MEDIDO 2026-09-23:
#       1 ocorrência, e é a linha de comentário deste próprio cabeçalho.
#       Sítios reais não cobertos: 0.
#       Comando: grep -rnE '\$\([^()]*\$\([^()]*\)[^()]*\|\|[[:space:]]*(echo|printf)' \
#                  .github/workflows/ scripts/ Makefile | grep -v testdata   -> 1 (comentário)
#
#   Captura INDIRETA — CNT=$(minha_funcao) com o `grep -c || echo` dentro da função
#       Não coberta: exigiria análise de grafo de chamadas, não varredura
#       estática de linha. Mesmo limite declarado pelo gate irmão
#       check-crlf-normalize-capture.sh para $(func_calling_python3).
#
#   `diff` como comando emissor — $(diff a b || echo "difere")
#       MEDIDA e deliberadamente NÃO coberta: `diff f /dev/null` sai rc=1 COM
#       stdout não-vazio ($'1,2d0\n< alpha\n< beta'), logo pertence à MESMA
#       classe do `grep -c`. Não entra no discriminante porque o corpus tem
#       ZERO capturas de `diff` com fallback emissor: MEDIDO 2026-09-23, das
#       2 ocorrências uma é este comentário e a outra é
#       check-roadmap-barrier-contract.sh:641, que usa `|| true` — a forma
#       correta. Cobri-la custaria risco de falso positivo (nesse idioma o
#       fallback é mensagem, não número) sem fechar sítio nenhum. Se um sítio
#       aparecer, a correção é reconhecer `diff` como comando emissor e
#       acrescentar um braço de injeção.
#       Comando: grep -rnE '\$\([^()]*diff[^()]*\|\|' .github/workflows/ scripts/ Makefile \
#                  | grep -v testdata        -> 2 (1 comentário + 1 com || true)
#
#   O lado do CONSUMO — $(( CNT + 0 )) com CNT já corrompido
#       Fora do escopo por decisão: o critério é sobre a EMISSÃO. Enumerar
#       consumidores aritméticos dá 118 `$(( ))` e 387 comparações `-eq/-lt/...`
#       no corpus (ML-0A §2.3) — população grande demais para um discriminante
#       estrutural. Fechar a emissão fecha a classe na origem.
#
#   Sobre-aproximação conhecida (falha para o lado seguro): um `-c` literal
#       DENTRO do padrão do grep (ex.: grep "flag -c" f || echo x) seria lido
#       como flag. Nenhuma ocorrência no corpus; o gate reportaria, não
#       silenciaria — o modo de erro aceitável para um gate.
#
# NÃO-VACUIDADE:
#   Piso sobre CANDIDATOS, contando só o que o discriminante JÁ enxerga: toda
#   substituição de comando que contenha `||` OU um `grep` contador. Essa
#   definição é deliberadamente mais larga que "sítio defeituoso" para que o
#   piso NÃO caia quando o ML-1A corrigir os 4 sítios de windows-census.yml:
#   `{ grep -ac ... || true; }` continua candidato, só deixa de violar.
#   Comando que produziu o número:
#     EMIT_FALLBACK_GATE_MIN_CANDIDATES=0 bash scripts/check-emitting-capture-fallback.sh
#   Resultado 2026-09-23: 87 candidatos, 87 isentos, 0 violações, 77 arquivos
#   varridos (76 antes do Cenário 198 deste ML entrar em check-gates-falsify.sh;
#   os 11 novos são os braços sintéticos, todos isentos por construção).
#   Piso = 50 (≈57% do medido). O piso é folgado de propósito: o corpus varrido
#   inclui .github/workflows/, que muda com frequência, e o objetivo da guarda é
#   pegar corpus VAZIO ou podado (o defeito da REQ anterior: gate que examina
#   zero e reporta sucesso), não congelar a contagem exata.
#
# STRINGS DE DIAGNÓSTICO (precisam continuar distintas — assert_fails_with casa nelas):
#   Violação : "captura com fallback emissor sobre comando que ja emite"
#   Vacuidade: "guarda de vacuidade disparou"
#
# AUTO-REFERÊNCIA:
#   Este arquivo é excluído da própria varredura (ver "Skip self"). As formas
#   acima aparecem aqui apenas em comentário e em ERE, nunca como código.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SELF_NAME="check-emitting-capture-fallback.sh"
SCAN_ROOT="${EMIT_FALLBACK_GATE_SCAN_ROOT:-$REPO_ROOT}"
MIN_CANDIDATES="${EMIT_FALLBACK_GATE_MIN_CANDIDATES:-50}"

FAIL=0
TOTAL_CANDIDATES=0
TOTAL_EXEMPT=0
TOTAL_VIOLATIONS=0
SCANNED_FILES=0

# ---------------------------------------------------------------------------
# EMITTING_FLAG_RE — pacote de flags do grep que contém `c` (contagem).
#   Casa: -c | -ac | -rc | -cv | --count
#   NÃO casa: --color=never — o pacote de flags precisa ser um TOKEN INTEIRO
#             (precedido de espaço/início e terminado em espaço/`=`/fim). Um
#             token que apenas CONTÉM a letra `c` não é flag de contagem.
#             Braço negativo provado no Cenário 198/J.
#   Medido 2026-09-23 sobre a varredura do ML-0A ('grep +-[a-z]*c[a-z]* '):
#             ela também NÃO casa --color=never (rc=1), mas TAMBÉM não casa
#             --count (rc=1) — casa só -ac (rc=0). Ou seja: a forma longa
#             ficaria fora daquela enumeração; aqui ela é coberta (braço C).
# ---------------------------------------------------------------------------
EMITTING_FLAG_RE='(^|[[:space:]])(-[A-Za-z]*c[A-Za-z]*|--count)([[:space:]=]|$)'

# Fallback que TAMBÉM emite em stdout.
EMITTING_FALLBACK_RE='\|\|[[:space:]]*\{?[[:space:]]*(echo|printf)([[:space:]]|$)'

# ---------------------------------------------------------------------------
# sub_has_counting_grep <texto-da-substituicao>
#   Verdadeiro (0) quando existe uma invocação de `grep` cujo pacote de flags
#   contém contagem. Examina cada ocorrência de `grep` e olha apenas até o
#   próximo `|` ou `;` — as flags de um grep vêm antes do fim do seu próprio
#   comando, nunca depois de um pipe.
# ---------------------------------------------------------------------------
sub_has_counting_grep() {
  local rest="$1"
  local head_part
  while [[ "$rest" == *grep* ]]; do
    rest="${rest#*grep}"
    head_part="${rest%%|*}"
    head_part="${head_part%%;*}"
    if [[ "$head_part" =~ $EMITTING_FLAG_RE ]]; then
      return 0
    fi
  done
  return 1
}

# ---------------------------------------------------------------------------
# scan_line <arquivo-base> <numero> <linha>
#   Extrai toda substituição de comando NÃO ANINHADA da linha e classifica.
# ---------------------------------------------------------------------------
SUB_RE='\$\(([^()]*)\)'

scan_line() {
  local fname="$1" lineno="$2" line="$3"
  local rest="$line" sub

  while [[ "$rest" =~ $SUB_RE ]]; do
    sub="${BASH_REMATCH[1]}"
    rest="${rest#*"${BASH_REMATCH[0]}"}"

    # Candidato = a substituição tem fallback `||` OU um grep contador.
    local has_pipe_or=0 has_grep_c=0 has_emit_fb=0
    [[ "$sub" == *"||"* ]] && has_pipe_or=1
    sub_has_counting_grep "$sub" && has_grep_c=1
    [[ "$sub" =~ $EMITTING_FALLBACK_RE ]] && has_emit_fb=1

    if [ $has_pipe_or -eq 0 ] && [ $has_grep_c -eq 0 ]; then
      continue
    fi

    ((TOTAL_CANDIDATES++))

    if [ $has_grep_c -eq 1 ] && [ $has_emit_fb -eq 1 ]; then
      echo "FAIL [$fname:$lineno] captura com fallback emissor sobre comando que ja emite: $sub"
      echo "     grep -c imprime \"0\" E sai 1 no caminho de nao-casamento; o || echo acrescenta uma SEGUNDA linha (\$'0\\n0') e o \$(( )) seguinte quebra."
      echo "     CORRIJA a forma de captura: VAR=\$( { grep -ac 'PAT' \"\$FILE\" || true; } )   # devolve 0, uma linha"
      echo "     NAO acrescente contagem cruzada por cima — a comparacao grep-vs-awk ja existia no censo e foi derrotada por este mesmo defeito."
      FAIL=1
      ((TOTAL_VIOLATIONS++))
      continue
    fi

    if [ $has_grep_c -eq 1 ] && [ $has_emit_fb -eq 0 ]; then
      echo "OK   [exempt/non-emitting-fallback/$fname:$lineno] grep contador com fallback que nao emite (|| true / || : / sem fallback)"
      ((TOTAL_EXEMPT++))
      continue
    fi

    echo "OK   [exempt/non-emitting-command/$fname:$lineno] fallback emissor sobre comando que nao escreve em stdout ao falhar"
    ((TOTAL_EXEMPT++))
  done
}

scan_file() {
  local file="$1"
  local fname
  fname=$(basename "$file")
  [ "$fname" = "$SELF_NAME" ] && return 0

  local -a lines
  mapfile -t lines < "$file"
  local n=${#lines[@]}
  local i=0
  while [ $i -lt $n ]; do
    local line="${lines[$i]}"
    if ! [[ "$line" =~ ^[[:space:]]*# ]]; then
      scan_line "$fname" "$((i + 1))" "$line"
    fi
    ((i++))
  done
  ((SCANNED_FILES++))
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --scan-root)
      SCAN_ROOT="$2"
      shift 2
      ;;
    *)
      echo "check-emitting-capture-fallback: argumento desconhecido: $1" >&2
      exit 2
      ;;
  esac
done

echo "=== check-emitting-capture-fallback: varrendo workflows, scripts e Makefile sob $SCAN_ROOT ==="
echo ""

shopt -s nullglob
for f in "$SCAN_ROOT/.github/workflows/"*.yml "$SCAN_ROOT/.github/workflows/"*.yaml \
         "$SCAN_ROOT/scripts/"*.sh "$SCAN_ROOT/scripts/"*.py; do
  [ -f "$f" ] || continue
  scan_file "$f"
done
shopt -u nullglob
[ -f "$SCAN_ROOT/Makefile" ] && scan_file "$SCAN_ROOT/Makefile"

# ---------------------------------------------------------------------------
# Guarda de não-vacuidade
# ---------------------------------------------------------------------------
echo ""
echo "=== guarda de vacuidade ==="
echo "Candidatos (substituicao com || ou com grep contador): $TOTAL_CANDIDATES"
echo "Isentos    (comando nao emite, ou fallback nao emite): $TOTAL_EXEMPT"
echo "Violacoes: $TOTAL_VIOLATIONS"
echo "Arquivos varridos: $SCANNED_FILES"
echo "Piso MIN_CANDIDATES: $MIN_CANDIDATES"

if [ "$TOTAL_CANDIDATES" -lt "$MIN_CANDIDATES" ]; then
  echo "FAIL guarda de vacuidade disparou: $TOTAL_CANDIDATES candidato(s) encontrado(s), piso e $MIN_CANDIDATES — o corpus pode estar vazio ou podado; use EMIT_FALLBACK_GATE_MIN_CANDIDATES para arvores sinteticas"
  FAIL=1
else
  echo "OK   vacuidade: $TOTAL_CANDIDATES >= $MIN_CANDIDATES"
fi

echo ""
if [ "$FAIL" -ne 0 ]; then
  echo "check-emitting-capture-fallback: FAIL"
  exit 1
fi
echo "check-emitting-capture-fallback: OK — nenhuma captura combina comando emissor com fallback emissor"
exit 0
