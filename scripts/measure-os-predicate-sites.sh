#!/usr/bin/env bash
#
# Mede os sítios de predicado de SO no produto, de forma REPRODUZÍVEL, e compara
# com um baseline POR NOME.
#
# Isto NÃO é o lint do AC2 da REQ-2026-09-05-onda-2. É o pré-requisito dele: o
# AC2 não pôde ser decidido porque o tamanho da superfície mudava de valor
# conforme quem digitava o grep.
#
#   publicado em 2026-09-08     105 sítios
#   redigitado em 2026-09-09    196, 110, 45 ou 30 — as QUATRO leituras plausíveis
#                               (ocorrências ou arquivos) x (com ou sem teste)
#
# Nenhuma das quatro reproduz 105, e o método de 08/09 não ficou escrito. Por
# isso o entregável aqui é o método, não o número: as quatro leituras saem
# nomeadas, e o inventário sai como `arquivo:linha:predicado` para que a próxima
# comparação seja de CONJUNTO, não de contagem.
#
# 🔴 Por que comparar por nome: em 2026-09-08 uma queda de 33 para 32 escondia
# uma regressão — dois sítios sumiram e um apareceu. Contagem igual não é
# conjunto igual, e contagem diferente não diz QUEM mudou.
#
# ATENÇÃO ao critério de exclusão do AC2. Ele diz que sítios de TRAVESSIA ficam
# fora "por decisão do D2 da ADR-2026-09-04". Essa ADR é do UPSTREAM — três
# arquivos com aquela data em `upstream/main:docs/adr/`, e a `ADR-2026-08-29`
# decide que a governança dele não é importada. Este script portanto NÃO separa
# classificação de travessia: ele mede a superfície inteira e diz isso. Separar
# exige decidir antes qual ADR governa aqui.
#
# Ver REQ-2026-09-05-onda-2-de-contribuicao-ao-upstream-fechar-as-classes-de-defeito-em-vez-dos-casos.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

BASELINE="${BASELINE:-$ROOT_DIR/scripts/testdata/os-predicate-sites-baseline.txt}"
# 🔴 ESCOPO sem `npm/src` e `pypi/trackfw` desde 2026-09-16: a v8.0.0 do upstream
# (#365) removeu as duas reimplementacoes. Com elas no escopo, o `git grep` varre
# diretorio que nao existe, e o denominador parece so ter encolhido. Os tres
# predicados de Node e Python continuam na lista: custam nada, e um sitio desses
# reaparecendo em `internal/` ou `cmd/` seria achado, nao ruido.
# Ver REQ-2026-09-16-gates-so-nossos-depois-da-v8.
# 🔴 E só arquivos .go desde 2026-09-18: o #395 do upstream trouxe 196 roadmaps .md como
# testdata em internal/roadmapdoc/testdata, e o git grep sobre `internal` passou a contar
# prosa desses .md como sítio (13 arquivos, 6 "classificações" no primeiro run). Na main
# anterior, zero sítios fora de .go — a restrição não tira nenhum sítio real.
ESCOPO=(':(glob)internal/**/*.go' ':(glob)cmd/**/*.go')

# Predicados que a REQ nomeia. Lista literal DE PROPÓSITO: ela é o contrato do
# AC2, não uma heurística a derivar. Acrescentar um aqui é mudar o escopo da
# REQ, e tem de ser deliberado.
#
# Eram seis. O ML-1H (AC8, 2026-09-11) acrescentou três — `runtime.GOOS`,
# `sys.platform` e `platform.system()` —, e a mudança de escopo está escrita na
# REQ. Os três eram invisíveis aqui e no lint ao mesmo tempo, porque os dois
# compartilham esta lista: um inventário que não vê o predicado não pode
# reconciliar com um lint que também não o vê.
PREDICADOS='os\.IsNotExist|filepath\.IsAbs|os\.path\.isabs|process\.platform|path\.isAbsolute|os\.name|runtime\.GOOS|sys\.platform|platform\.system\(\)'

eh_teste() {  # eh_teste <caminho>
  case "$1" in
    *_test.go|*.test.js|*test_*.py|*/tests/*|*/test/*) return 0 ;;
    *) return 1 ;;
  esac
}

# ---------------------------------------------------------------------------
# Inventário: arquivo:linha:predicado — a unidade que se compara por nome.
# ---------------------------------------------------------------------------
inventario() {  # inventario <incluir_teste: sim|nao>
  local incluir="$1" f l pred
  # `|| true` obrigatório: `git grep` sai 1 quando não casa nada, e sob
  # `set -euo pipefail` isso mata o script AQUI — antes da guarda de vacuidade
  # conseguir dizer o que houve. Medido em 2026-09-09: a falsificação da guarda
  # dava `exit 1` sem mensagem nenhuma, ou seja, o código certo pelo motivo
  # errado, e a guarda seguia sem nunca ter sido exercitada.
  { git grep -a -n -o -E "$PREDICADOS" -- "${ESCOPO[@]}" 2>/dev/null || true; } \
  | while IFS=: read -r f l pred; do
    if [ "$incluir" = "nao" ] && eh_teste "$f"; then continue; fi
    printf '%s:%s:%s\n' "$f" "$l" "$pred"
  done | sort -u
}

TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
inventario sim > "$TMP/com.txt"
inventario nao > "$TMP/sem.txt"

varridos=$(git ls-files -- "${ESCOPO[@]}" | wc -l | tr -d ' ')
ocorr_com=$(grep -c . < "$TMP/com.txt" || true)
ocorr_sem=$(grep -c . < "$TMP/sem.txt" || true)
arq_com=$(cut -d: -f1 < "$TMP/com.txt" | sort -u | grep -c . || true)
arq_sem=$(cut -d: -f1 < "$TMP/sem.txt" | sort -u | grep -c . || true)

# ---------------------------------------------------------------------------
# Guardas de vacuidade — verde sobre zero não é evidência.
# ---------------------------------------------------------------------------
if [ "$varridos" -eq 0 ]; then
  echo "measure-os-predicate-sites: GUARDA — zero arquivos varridos em ${ESCOPO[*]}" >&2
  exit 1
fi
if [ "$ocorr_com" -eq 0 ]; then
  echo "measure-os-predicate-sites: GUARDA — zero sítios encontrados em ${varridos} arquivos." >&2
  echo "  Não é 'limpamos tudo': seis predicados desaparecerem de uma vez é a busca ter quebrado." >&2
  exit 1
fi

echo "measure-os-predicate-sites: ${varridos} arquivo(s) de produto varrido(s) em ${ESCOPO[*]}"
echo ""
echo "  AS QUATRO LEITURAS — nomeadas, porque a ambiguidade entre elas foi o defeito"
echo "  ────────────────────────────────────────────────────────────────────────────"
printf '    ocorrencias, com teste : %s\n' "$ocorr_com"
printf '    ocorrencias, SEM teste : %s\n' "$ocorr_sem"
printf '    arquivos,    com teste : %s\n' "$arq_com"
printf '    arquivos,    SEM teste : %s\n' "$arq_sem"
echo ""
echo "  POR PREDICADO (ocorrências, com teste · sem teste)"
echo "  ──────────────────────────────────────────────────"
for p in 'os\.IsNotExist' 'filepath\.IsAbs' 'os\.path\.isabs' 'process\.platform' 'path\.isAbsolute' 'os\.name' 'runtime\.GOOS' 'sys\.platform' 'platform\.system()'; do
  nome=$(printf '%s' "$p" | sed 's/\\//g')
  c=$(grep -c ":${nome}$" "$TMP/com.txt" 2>/dev/null || true)
  s=$(grep -c ":${nome}$" "$TMP/sem.txt" 2>/dev/null || true)
  printf '    %-18s %4s · %4s\n' "$nome" "${c:-0}" "${s:-0}"
done

# ---------------------------------------------------------------------------
# Comparação com o baseline — POR NOME, nos dois sentidos.
# ---------------------------------------------------------------------------
# A gravação vem ANTES da checagem de ausência, senão `--gravar-baseline` nunca
# alcança o código que grava — defeito cometido e pego na primeira execução.
if [ "${1:-}" = "--gravar-baseline" ]; then
  mkdir -p "$(dirname "$BASELINE")"
  cp "$TMP/com.txt" "$BASELINE"
  echo ""
  echo "  baseline gravado: $(grep -c . < "$BASELINE") sítio(s) em $BASELINE"
  exit 0
fi

if [ ! -f "$BASELINE" ]; then
  echo ""
  echo "  BASELINE AUSENTE em $BASELINE"
  echo "  Gere com: bash scripts/measure-os-predicate-sites.sh --gravar-baseline"
  exit 0
fi

# NORMALIZACAO DE FIM DE LINHA — obrigatoria, nao cosmetica.
#
# Medido em 2026-09-09, logo depois de mesclar este script: o baseline foi
# gravado com LF, o git o devolveu com CRLF no checkout seguinte, e a comparacao
# acusou `novos 196 · sumidos 196` -- o acervo INTEIRO como mudado, quando nada
# mudou. O `.gitattributes` deste repo e arquivo COMPARTILHADO com o upstream, e
# mexer nele criaria divergencia de produto que todo merge futuro pagaria. Entao
# a normalizacao mora aqui, no lado que le.
#
# A guarda funcionou -- gritou em vez de passar em silencio --, mas um alarme que
# dispara sozinho a cada checkout treina quem le a ignora-lo.
BASE_NORM="$TMP/baseline-normalizado.txt"
tr -d '\r' < "$BASELINE" | sort -u | grep . > "$BASE_NORM" || true

base_n=$(grep -c . < "$BASE_NORM" || true)

# GUARDA: baseline existente mas VAZIO nao e "tudo novo" -- e o baseline ter sido
# truncado. Medido em 2026-09-09: com o arquivo zerado, a comparacao devolvia
# `novos: 196 · sumidos: 0` e `exit 0`, ou seja, o acervo inteiro reportado como
# mudanca legitima. Um baseline que se apaga sozinho passaria despercebido como
# "grande refatoracao".
if [ "$base_n" -eq 0 ]; then
  echo "measure-os-predicate-sites: GUARDA — o baseline existe mas esta VAZIO ($BASELINE)." >&2
  echo "  Isso nao e 'tudo novo': e o baseline truncado. Regrave com --gravar-baseline" >&2
  echo "  SO se voce souber que o acervo de fato mudou." >&2
  exit 1
fi

novos=$(comm -13 "$BASE_NORM" "$TMP/com.txt" | grep -c . || true)
sumidos=$(comm -23 "$BASE_NORM" "$TMP/com.txt" | grep -c . || true)

echo ""
echo "  CONTRA O BASELINE (${base_n} sítios) — comparado por NOME, nos dois sentidos"
echo "  ────────────────────────────────────────────────────────────────────────────"
printf '    novos   : %s\n' "$novos"
printf '    sumidos : %s\n' "$sumidos"

if [ "$novos" -gt 0 ]; then
  echo "    ── novos ──"
  comm -13 "$BASE_NORM" "$TMP/com.txt" | sed 's/^/      + /'
fi
if [ "$sumidos" -gt 0 ]; then
  echo "    ── sumidos ──"
  comm -23 "$BASE_NORM" "$TMP/com.txt" | sed 's/^/      - /'
fi

# 🔴 O saldo zero NÃO é sinônimo de conjunto igual. Se novos == sumidos, a
# contagem não muda e o conjunto mudou — é o caso medido em 2026-09-08.
if [ "$novos" -gt 0 ] || [ "$sumidos" -gt 0 ]; then
  echo ""
  echo "    A superfície MUDOU. Saldo (${novos} - ${sumidos}) não é o dado: os nomes acima são."
  echo "    Atualize o baseline com --gravar-baseline SÓ depois de decidir o que cada mudança significa."
fi

echo ""
echo "Medicao concluida."
