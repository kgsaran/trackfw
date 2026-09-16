#!/usr/bin/env bash
#
# Lint do AC2 da REQ-2026-09-05-onda-2: predicado de SO em sítio de
# CLASSIFICAÇÃO.
#
# O discriminante é a `ADR-2026-09-09-predicado-de-so-em-sitio-de-classificacao`,
# escrita a partir da nossa medição depois que se descobriu que o AC delegava a
# decisão ao D2 de uma ADR do upstream — que não existe neste repositório.
#
#   D1  argumento e ERRO de chamada de sistema  -> travessia, FORA de escopo
#   D2  argumento e STRING AUTORADA             -> classificacao, EM escopo
#   D3  plataforma lida UMA vez para constante  -> a costura, nao a violacao
#   D4  o onus e de quem quer excluir           -> exclusao escrita
#   D5  com teste e sem teste, SEPARADOS        -> misturar falseou 3x
#
# ML-1H (2026-09-11): a lista passou a ter NOVE predicados. Entraram
# `runtime.GOOS`, `sys.platform` e `platform.system()`, que ficavam invisiveis
# -- 44 sitios, 18 deles em produto fora de teste. E o D3 passou a reconhecer a
# costura tambem quando a plataforma e PASSADA COMO ARGUMENTO
# (`openBrowser(process.platform, url)`), nao so quando e atribuida a constante:
# a #321 do upstream reescreveu a MESMA costura de uma forma para a outra, e o
# lint mudou de veredito sem o codigo mudar de natureza.
#
# 🔴 LIMITE DECLARADO: o reconhecedor de docstring do Python conta aspas triplas
# DUPLAS ("""); docstring com aspas simples nao e reconhecida. Medido em
# 2026-09-11: zero ocorrencias de ''' no escopo varrido, entao a regra estreita
# nao esconde nada hoje -- e o dia em que esconder, o sitio aparece como
# classificacao nao declarada, que e o lado seguro de errar.
#
# ESTE GATE E UM RATCHET, NAO UMA VARREDURA DE LIMPEZA.
#
# Medido em 2026-09-10: dos 110 sitios sem arquivo de teste, 59 saem por D1 (o
# `os.IsNotExist` recebe `err` em TODOS os sitios de codigo), 12 sao plataforma
# (4 costura + 8 comentario), e ~26 sao classificacao — TODOS em produto do
# upstream.
#
# 🔴 Corrigi-los aqui criaria divergencia de produto, que hoje e ZERO. O escopo
# negativo da REQ decide: achado vira ISSUE, nao correcao local. Entao o gate
# congela os conhecidos, COM MOTIVO, e reprova o proximo.
#
# Ver ADR-2026-09-09 e REQ-2026-09-05-onda-2.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

# 🔴 ESCOPO sem `npm/src` e `pypi/trackfw` desde 2026-09-16: a v8.0.0 do upstream
# (#365) removeu as duas reimplementacoes. Com elas no escopo, o `git grep` varre
# diretorio que nao existe, e o denominador parece so ter encolhido. Os tres
# predicados de Node e Python continuam na lista: custam nada, e um sitio desses
# reaparecendo em `internal/` ou `cmd/` seria achado, nao ruido.
# Ver REQ-2026-09-16-gates-so-nossos-depois-da-v8.
ESCOPO=(internal cmd)
PREDICADOS='filepath\.IsAbs|os\.path\.isabs|path\.isAbsolute|process\.platform|os\.name|os\.IsNotExist|runtime\.GOOS|sys\.platform|platform\.system\(\)'

# Os predicados que leem a PLATAFORMA (nao um caminho, nao um erro). So estes
# podem ser costura: o D3 decide sobre a origem da plataforma, e `IsAbs` ou
# `IsNotExist` nao carregam plataforma nenhuma.
PLATAFORMA='process\.platform|os\.name|runtime\.GOOS|sys\.platform|platform\.system\(\)'

# Funcoes que COMPARAM a plataforma. Receber a plataforma como argumento aqui nao
# e costura: e a mesma classificacao do `==`, escrita de outro jeito.
COMPARADORES='strings\.(Contains|HasPrefix|HasSuffix|EqualFold|Index)|startswith|endswith|startsWith|endsWith|includes|indexOf|re\.(match|search)'

# ---------------------------------------------------------------------------
# BASELINE — sitios de classificacao conhecidos em 2026-09-10, COM MOTIVO.
#
# Formato: <arquivo>:<predicado>|<motivo>
#
# 🔴 SO linhas '<arquivo>|<motivo>' aqui dentro: o laco de declaracao obsoleta le a
# variavel linha a linha, e uma linha de comentario vira uma declaracao fantasma.
# Medido em 2026-09-11, ao escrever a nota abaixo dentro da variavel: cinco avisos
# de obsolescencia para cinco linhas de prosa.
#
# O pypi/trackfw/validator.py SAIU deste baseline no ML-1H, e o motivo importa: os
# cinco sitios que o punham aqui (2136, 2144, 2145, 2146, 2149) sao PROSA dentro de
# uma docstring que explica a ADR citando os.path.isabs e os.name. O classificador
# anterior so olhava o inicio da linha, entao contava prosa como classificacao -- e o
# arquivo foi declarado por um sitio que nunca existiu.
# Sete declaracoes SAIRAM em 2026-09-16 -- os espelhos Node e Python e as duas leituras
# inline de plataforma (homedir.py, tty.py) --, todas de arquivo que a v8.0.0 do upstream
# removeu. O gate as acusou como obsoletas no primeiro run depois do sync.
# Granularidade de ARQUIVO, nao de linha: numero de linha muda a cada merge do
# upstream e o baseline viraria ruido. O que importa e "este arquivo ja tinha
# sitio de classificacao"; um arquivo NOVO com sitio novo e o que o gate pega.
# ---------------------------------------------------------------------------
BASELINE="
internal/integrations/manager.go|resolucao de caminho de instalacao de integracao; produto do upstream, e o maior sitio unico (10 ocorrencias)
internal/validator/validator.go|validacao de caminho relativo em artefato; produto do upstream
"

esta_no_baseline() {
  printf '%s' "$BASELINE" | grep -q "^${1}|"
}

# ---------------------------------------------------------------------------
# Classificadores. Cada um implementa uma decisao da ADR.
# ---------------------------------------------------------------------------
eh_comentario() {  # a linha e comentario?
  case "$(printf '%s' "$1" | sed 's/^[[:space:]]*//')" in
    '//'*|'#'*|'*'*|'/*'*) return 0 ;;
    *) return 1 ;;
  esac
}

eh_d1_travessia() {  # D1: o predicado recebe um valor de ERRO
  printf '%s' "$1" | grep -qE 'IsNotExist\((err|readErr|[a-zA-Z_][a-zA-Z0-9_]*[Ee]rr)\)'
}

eh_d3_costura() {  # D3: a plataforma entra por UM ponto -- constante ou argumento
  # (a) atribuida a uma constante/variavel nomeada, com ou sem anotacao de tipo:
  #       var CurrentGOOS = runtime.GOOS
  #       _current_platform: str = sys.platform
  #       const platform = process.platform
  printf '%s' "$1" | grep -qE "(let|const|var)?[[:space:]]*[_a-zA-Z][_a-zA-Z0-9]*([[:space:]]*:[[:space:]]*[_a-zA-Z][_a-zA-Z0-9]*)?[[:space:]]*=[[:space:]]*(${PLATAFORMA})" && return 0
  # (b) PASSADA COMO ARGUMENTO para a funcao que costura:
  #       openBrowser(process.platform, url)   ·   _browser_argv(system, url)
  #     Exige o predicado como argumento INTEIRO -- entre '(' ou ',' e ',' ou ')'.
  #     `foo(runtime.GOOS == "windows")` NAO casa: ali a plataforma e comparada
  #     no proprio sitio, e comparar e classificar. Medido em 2026-09-11.
  #
  #     🔴 EXCECAO, achada por sonda ANTES de mesclar: se quem recebe a plataforma
  #     e um COMPARADOR (`strings.Contains(runtime.GOOS, "win")`), a forma e de
  #     argumento mas o ato e comparacao -- e a primeira versao desta regra aceitava,
  #     abrindo um caminho para fechar o ratchet so trocando `==` por um ajudante de
  #     string. A lista de comparadores e literal e curta de proposito: um comparador
  #     novo e nao listado volta a ser aceito como costura, e o remedio e acrescenta-lo
  #     aqui -- nunca alargar a regra (b).
  printf '%s' "$1" | grep -qE "(${COMPARADORES})[^)]*(${PLATAFORMA})" && return 1
  printf '%s' "$1" | grep -qE "[,(][[:space:]]*(${PLATAFORMA})[[:space:]]*[,)]" && return 0
  return 1
}

eh_docstring_py() {  # <arquivo> <linha>: a linha esta DENTRO de uma docstring?
  # Existe porque tres sitios do pypi/ vivem em docstring de modulo, que nao
  # comeca com '#' e por isso escapava do eh_comentario -- seriam acusados como
  # classificacao. Conta aspas triplas ate a linha anterior: impar = dentro.
  case "$1" in *.py) ;; *) return 1 ;; esac
  [ "$2" -gt 1 ] || return 1
  local n
  n=$(head -n "$(($2 - 1))" "$1" | grep -o '"""' | wc -l | tr -d ' ')
  [ $((n % 2)) -eq 1 ]
}

eh_teste() {
  case "$1" in *_test.go|*.test.js|*test_*.py|*/tests/*|*/test/*) return 0 ;; *) return 1 ;; esac
}

# ---------------------------------------------------------------------------
# Varredura
# ---------------------------------------------------------------------------
varridos=0; com_teste=0; d1=0; d3=0; coment=0
classificacao=0; nao_declarados=0
declare -a arq_vistos=()

while IFS=: read -r f l pred; do
  [ -n "${f:-}" ] || continue
  varridos=$((varridos + 1))
  if eh_teste "$f"; then com_teste=$((com_teste + 1)); continue; fi

  linha=$(sed -n "${l}p" "$f" 2>/dev/null || true)
  [ -n "$linha" ] || continue

  if eh_comentario "$linha" || eh_docstring_py "$f" "$l"; then coment=$((coment + 1)); continue; fi
  if eh_d1_travessia "$linha"; then d1=$((d1 + 1)); continue; fi
  if eh_d3_costura "$linha";   then d3=$((d3 + 1)); continue; fi

  classificacao=$((classificacao + 1))
  if ! esta_no_baseline "$f"; then
    echo "  ✗ CLASSIFICACAO NAO DECLARADA: ${f}:${l} — ${pred}" >&2
    echo "      $(printf '%s' "$linha" | sed 's/^[[:space:]]*//' | cut -c1-92)" >&2
    nao_declarados=$((nao_declarados + 1))
  else
    ja=0; for v in "${arq_vistos[@]:-}"; do [ "$v" = "$f" ] && ja=1; done
    [ "$ja" -eq 0 ] && arq_vistos+=("$f")
  fi
done < <({ git grep -a -n -o -E "$PREDICADOS" -- "${ESCOPO[@]}" 2>/dev/null || true; } | sort -u)

# ---------------------------------------------------------------------------
# Guardas de vacuidade
# ---------------------------------------------------------------------------
if [ "$varridos" -eq 0 ]; then
  echo "check-os-predicate-classification: GUARDA — zero sitios varridos em ${ESCOPO[*]}" >&2
  echo "  Seis predicados sumirem de uma vez e a busca ter quebrado, nao limpeza." >&2
  exit 1
fi

if [ "$d1" -eq 0 ]; then
  echo "check-os-predicate-classification: GUARDA — zero sitios classificados como D1." >&2
  echo "  O os.IsNotExist recebe 'err' em TODOS os sitios de codigo medidos; zero aqui" >&2
  echo "  significa que o classificador D1 parou de casar, e tudo viraria classificacao." >&2
  exit 1
fi

if [ "$nao_declarados" -gt 0 ]; then
  echo "" >&2
  echo "${nao_declarados} sitio(s) de CLASSIFICACAO em arquivo nao declarado." >&2
  echo "" >&2
  echo "Pela ADR-2026-09-09, predicado de SO aplicado a string AUTORADA decide" >&2
  echo "significado que tem de ser o mesmo em todo SO. Duas saidas:" >&2
  echo "  1. o sitio passa a usar o resolvedor canonico (internal/pathanchor e espelhos)" >&2
  echo "  2. o arquivo entra no BASELINE deste script COM O MOTIVO ESCRITO" >&2
  echo "" >&2
  echo "🔴 Baseline sem motivo escrito e o mesmo que nao ter regra (D4)." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Denominador. D5: com teste e sem teste, SEPARADOS.
# ---------------------------------------------------------------------------
echo "check-os-predicate-classification: ${varridos} sitio(s) varrido(s) em ${ESCOPO[*]}"
echo "  com arquivo de teste (D5, reportado a parte) : ${com_teste}"
echo "  D1 travessia   (argumento e erro)            : ${d1}"
echo "  D3 costura     (constante nomeada ou argumento): ${d3}"
echo "  comentario                                    : ${coment}"
echo "  D2 CLASSIFICACAO                              : ${classificacao}  em ${#arq_vistos[@]} arquivo(s) declarado(s)"

# Declaracao que nao corresponde mais a nada e lixo: avisa, mas nao reprova --
# some sozinha quando o sitio for corrigido. Mesmo tratamento que o
# check-subcommand-parity dava, antes de ser retirado na v8.
obsoletas=0
printf '%s
' "$BASELINE" | while IFS='|' read -r arq motivo; do
  [ -n "$arq" ] || continue
  achou=0
  for v in "${arq_vistos[@]:-}"; do [ "$v" = "$arq" ] && achou=1; done
  [ "$achou" -eq 0 ] && echo "  ⚠ declaracao obsoleta: '${arq}' nao tem mais sitio de classificacao — remova do BASELINE"
done
echo "Nenhum sitio de classificacao fora do baseline."
