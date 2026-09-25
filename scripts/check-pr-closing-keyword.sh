#!/usr/bin/env bash
# Gate — palavra-chave de fechamento de issue no corpo do PR precisa ser INGLESA
# (REQ-2026-09-02-prs-usam-palavra-chave-de-fechamento-em-portugues-e-nenhuma-issue-fecha-automaticamente)
#
# ============================================================================
# O DEFEITO QUE ESTE GATE GUARDA
# ============================================================================
# O GitHub fecha uma issue no merge apenas se o corpo do PR contiver uma destas
# palavras-chave seguida da referencia da issue:
#     close|closes|closed · fix|fixes|fixed · resolve|resolves|resolved
# "Fecha #246." nao fecha nada. O merge tem sucesso, o texto AFIRMA que fechou,
# e a issue continua aberta -- falha silenciosa e INVERTIDA (o artefato se
# reporta saudavel estando inerte).
#
# Medido em 2026-09-02 sobre os 241 PRs mergeados deste repositorio: apenas 4
# fecharam issue de verdade (confirmado por `closingIssuesReferences`), e os 4
# usaram `Fixes #N`. O PR #247 abriu com "Fecha #246." e a issue ficou aberta.
#
# ============================================================================
# FORMAS ACEITAS E RECUSADAS -- DECLARADAS, com o motivo
# (vault/notes/gate-literal-regex-syntax-equivalent-bypass-2026-09-01.md exige
#  que um gate baseado em regex diga o que cobre e o que NAO cobre)
# ============================================================================
# RECUSA (exit 1) -- frase que AFIRMA fechamento em portugues, com a referencia
#   da issue, e SEM forma inglesa valida para AQUELE MESMO numero:
#     "Fecha #246."            "Corrige o #239."      "Encerra a issue #12"
#     "**Corrigido** #12"      "Fecham #12"           "Resolvido #12"
#     "Fecha **#274**"         "Fecha: #421"          "Fecha [#274](url)"
#     "Fecho a #12"            "Fechara a #12"        "Fechando a #12"
#     "Fecha kgsaran/trackfw#12"  "Fecha https://github.com/o/r/issues/12"
#
#   >>> "AFIRMA" e a palavra load-bearing, e e a correcao de 2026-09-25: ate
#       aqui o gate media ADJACENCIA LEXICAL e nunca perguntava se a frase
#       afirmava fechamento. Medido em 358 corpos reais de PR mergeado, isso
#       dava precisao de 1 acerto em 3 acusacoes e cobertura de 1 em 4.
#       Entre o verbo e a referencia agora cabe qualquer caractere de uma lista
#       FECHADA de nao-palavra (`* _ ~ : , - -- [` e espaco/tab) -- nunca uma
#       PALAVRA, que continua descaracterizando a declaracao (forma aceita 3).
#
# ACEITA (exit 0), deliberadamente:
#   1. Qualquer forma inglesa valida para o mesmo numero, em qualquer lugar do
#      corpo: `Closes #12`, `Fixes #12`, `resolved #12`, `Fixes owner/repo#12`,
#      `Closes https://github.com/o/r/issues/12`.
#      >>> A isencao e POR NUMERO DE ISSUE, nao "existe alguma palavra inglesa
#          no corpo". `Fecha #246` + `Fixes #999` RECUSA. Esta clausula e
#          load-bearing: e ela que salva os PRs #238 e #240 (que escrevem
#          "Corrige o #237." na 1a linha E `Fixes #237` no rodape, e de fato
#          fecharam) de virarem falso positivo.
#   2. `Resolve|Resolves|Resolved #N` -- grafia identica em ingles e portugues,
#      e o INGLES E VALIDO no GitHub. Recusar isso seria reprovar um corpo que
#      funciona: o pior falso positivo possivel. Por isso `resolve` esta FORA
#      da lista portuguesa, ao contrario do que o enunciado da REQ sugeria.
#      (`Resolvido`/`Resolvida`/`Resolvem`/`Resolver` continuam recusados --
#      sao inequivocamente portugueses e nao fecham nada.)
#   3. Mencao a issue/PR em PROSA, com palavras intervenientes:
#      "o mesmo sitio do #238", "portado do #223",
#      "Fecha o **item 4** da issue #216", "Corrige os tres defeitos do #232",
#      "Fecha a governanca do PR #145".
#      >>> A ADJACENCIA e o unico motivo de o falso positivo ser ZERO em 240
#          corpos reais. Afrouxar para "keyword em qualquer lugar da linha"
#          sobe de 1 para 43 linhas reprovadas neste mesmo corpus -- um gate
#          ruidoso e desligado, e ai nao guarda nada.
#   4. Trechos em ZONA DE CITACAO -- as SEIS zonas dos dois baldes abaixo:
#      cerca (```), code span (`...`), bloco indentado por 4 espacos/tab,
#      blockquote (`> ...`), linha de tabela (`| ... |`) e span entre aspas
#      RETAS ("..."). Motivo: DOCUMENTAR a forma errada e o que este proprio PR
#      faz. Um exemplo citado nao e uma declaracao de intencao.
#      >>> A indentacao e medida no corpo COMO O AUTOR ESCREVEU, nunca depois
#          do mascaramento de code span: apagar um span no inicio da linha
#          FABRICA 4 espacos a esquerda, e ``trackfw validate` -- Fecha #246.`
#          saia rc=0 por isso -- falso negativo medido e corrigido no ML-N2.
#      >>> 🔴 As aspas sao o QUINTO de cinco mecanismos de zona, nao o unico.
#          O AC3 da REQ proibia inferir citacao "de aspas apenas"; o arquiteto
#          ampliou-o no ML-N2 com a razao medida: a frase canonica do issue
#          #258 (`O corpo da minha PR #247 dizia "Fecha #246" ...`) -- a propria
#          descricao deste defeito -- passa EXCLUSIVAMENTE pela zona de aspas.
#          Retiradas as aspas nao sobra sinal nenhum na frase (medido: rc=1),
#          e o gate ficaria acusando para sempre o texto que documenta o seu
#          proprio bug.
#   5. Frase que NEGA o fechamento na mesma clausula: "Este PR nao fecha a
#      #363", "Entrega sem fechar a #12", "Deixa de fechar a #12".
#      >>> 🔴 Toda supressao por polaridade SAI NO LOG, com a frase e o token.
#          Ver o bloco VACUIDADE abaixo -- e obrigacao, nao cortesia.
#
# ============================================================================
# OS DOIS BALDES DE ZONA -- comportamento OPOSTO, e MEDIDO (nao deduzido)
# ============================================================================
# Medido em 2026-09-24 com 7 PRs sonda contra 2 issues descartaveis deste
# repositorio, lendo `closingIssuesReferences` (secao 10 do parecer
# docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md):
#
#   CODIGO     cerca · code span · bloco indentado    #428/#429/#431 = []
#              O GitHub IGNORA a palavra-chave aqui. A isencao inglesa seria
#              FALSA -> a mascara e subtraida dos DOIS matchers.
#              🔴 Logo `Fecha #246.` cuja unica forma inglesa vive em cerca
#              RECUSA, e isso esta CERTO: aquele corpo nao fecha a issue. Fazer
#              a isencao valer ali instalaria FALSO NEGATIVO -- silencio sobre
#              uma declaracao falsa, que e pior que o incomodo de hoje.
#   NAO-CODIGO blockquote · celula de tabela · aspas   #432/#433/#434 = [430]
#              O GitHub HONRA a palavra-chave aqui. A isencao e REAL -> passe
#              PROPRIO, subtraido so do matcher portugues.
#              >>> IMPLEMENTADO no ML-N2 (`NONCODE_ZONE_RES`). As tres zonas
#                  sao falsificadas INDIVIDUALMENTE e nos DOIS lados: cada uma
#                  suprime a acusacao portuguesa E deixa a isencao inglesa
#                  atravessar. A zona-sonda `@@PROBE@@` do ML-N1 foi removida:
#                  as zonas reais exercitam a mesma propriedade, com
#                  verdade-terreno medida na API.
#
# NAO COBERTO (limite declarado, nao acidente):
#   - Parafrase com palavras intervenientes: "este PR fecha, por fim, a #246".
#     Deliberado -- ver item 3 acima; cobrir isso custa o falso positivo zero.
#   - Segunda referencia de uma declaracao coordenada: "Fecha **#274** e
#     **#275**" acusa o #274 e NAO o #275, porque "e" e PALAVRA e a lacuna as
#     proibe. A LINHA e acusada, entao o defeito e visto; alargar a lacuna para
#     palavras custaria o contra-braco "Fecha o **item 4** da issue #216".
#   - `Fecha (#274)` com PARENTESE. Medido: admitir `(` produz falso positivo
#     inedito no corpus (`issues ja fechados** (#335, #336)`). `[` sozinho e
#     seguro e esta DENTRO.
#   - `Fecha a issue 12` sem `#`. Medido: numero nu custa +6 falsos positivos.
#   - PRETERITO PERFEITO (`fechou`, `fechei`, `corrigiu`). Medido: relatar acao
#     passada nao e declarar fechamento (`Sei que voce fechou as #222...`).
#   - Verbos fora das 4 familias (`Sana`, `Soluciona`, `Conserta`, `Elimina`).
#     Lista fechada POR ESCOLHA, nao por completude.
#   - 🔴 SUPERFICIE DE SILENCIAMENTO DA POLARIDADE. Nao fecha em regex: um
#     intensificador afirmativo pode conter token de negacao. "Nao e verdade
#     que fecha #12" (deve acusar) e "Sem contar o #99, fecha a #12" (nao deve)
#     cabem na MESMA janela e querem vereditos OPOSTOS -- nenhum ajuste de
#     janela as separa. Por isso o residual e tornado VISIVEL no log, nao
#     aceito em silencio.
#     >>> O ML-N2 ESTREITA essa superficie por um efeito colateral medido: a
#         zona de aspas apaga DENTRO da linha, entao um token de negacao que
#         vive numa CITACAO deixa de suprimir a declaracao do autor.
#         `Ele disse "nao" e fecha a #246` saia rc=0 e passa a sair rc=1 --
#         unica divergencia em 7 arranjos de negacao x zona exercitados, e na
#         direcao fail-closed. Presa pelo par `costura-negacao-*`.
#   - Zonas nao medidas, que NAO sao extrapoladas por analogia: comentario
#     HTML, <pre>/<code>, ASPAS CURVAS, cerca com atributo de linguagem.
#     🔴 As aspas CURVAS sao residual DECLARADO, nao esquecimento: a sonda #434
#     mediu so as RETAS, e o corpus A tem ZERO ocorrencia de curvas -- inclui-
#     las nao compraria nada mensuravel e custaria uma premissa nao medida,
#     que e exatamente a analogia refutada duas vezes na Wave 0 desta REQ.
#     Preso por cenario (`residual-aspas-curvas-nao-sao-zona`): se um dia forem
#     medidas e entrarem, o autoteste fica vermelho e a decisao volta a ser
#     consciente.
#     ⚠️ `~~~` e caso diferente e fica REGISTRADO como tal: o FENCE_RE ja o
#     casa, entao o gate o trata como zona de CODIGO -- mas a sonda mediu so
#     ```` ``` ````. E uma analogia HERDADA, nao medida; esta declarada aqui
#     em vez de passar por cobertura.
#   - Largura da zona de TABELA: `| ... |` apaga 1354 linhas do corpus A, de
#     longe a mais larga das seis. Divida DECLARADA e medida inerte aqui: ZERO
#     das 1354 carrega declaracao portuguesa com `#N`, e a forma estrita (com
#     o pipe de fechamento) apaga exatamente as MESMAS 1354 que a forma larga.
#   - FALSO NEGATIVO por construcao nas 6 zonas: quem escrever a declaracao de
#     fechamento DENTRO de um blockquote, de uma tabela ou entre aspas nao sera
#     avisado. E o preco escolhido para nao acusar quem apenas CITA -- e nas
#     tres zonas de codigo o silencio e duplamente correto, porque ali o
#     GitHub nao fecha de qualquer forma.
#   - Evasao adversaria (o autor do corpo nao e um adversario: e alguem que
#     quer fechar a issue e erra o idioma).
#   Anotado como `partial=` em docs/cli-parity.md, nunca `gate=`.
#
# ============================================================================
# VACUIDADE -- este gate NUNCA sai 0 em silencio
# ============================================================================
#   exit 0 = corpo lido E avaliado E limpo
#   exit 1 = defeito encontrado (linha nomeada)
#   exit 2 = not_evaluated: corpo vazio, evento sem payload, execucao fora de
#            `pull_request`, `gh` ausente. Non-zero de proposito.
#   🔴 E o exit 0 tambem nao e silencioso quando houve SUPRESSAO por
#      polaridade: o gate imprime a linha suprimida e o token que a suprimiu,
#      nos DOIS caminhos de saida (0 e 1). Sem isso a leitura de polaridade
#      seria um buraco invisivel no meio do invariante desta secao.
#
# ============================================================================
# MODOS
# ============================================================================
#   --self-test  autoteste por fixtures (usado por `make parity`, onde nao ha
#                contexto de PR). Falsifica nas DUAS direcoes pelo MESMO codigo
#                que o CI usa -- nao ha segunda copia do matcher.
#   (padrao)     le o corpo do PR de, nesta ordem:
#                  PR_BODY_FILE  -> caminho de arquivo
#                  GITHUB_EVENT_PATH + GITHUB_EVENT_NAME=pull_request, e dentro dele:
#                     corpo VIVO pela API (`gh pr view` do .pull_request.number)
#                     corpo do payload, se a API nao estiver disponivel
#                  --pr <n> / PR_NUMBER -> `gh pr view`
set -euo pipefail

# UTF-8 no stdio do python3 -- mesmo motivo de check-gates-falsify.sh: sob
# console cp1252 um print() de "sitio"/"adjacencia" acentuado estouraria
# UnicodeEncodeError e o gate reprovaria por motivo alheio ao que mede.
export PYTHONIOENCODING=utf-8
export LC_ALL="${LC_ALL:-C.UTF-8}" 2>/dev/null || true

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-prclose.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

MATCHER="$WORK/matcher.py"
cat >"$MATCHER" <<'PY_EOF'
# -*- coding: utf-8 -*-
"""Matcher unico do gate. Le UM arquivo com o corpo bruto do PR.
exit 0 = limpo | exit 1 = defeito | exit 2 = not_evaluated"""
import re
import sys

# Palavras-chave que o GitHub REALMENTE reconhece (docs oficiais).
# `resolve*` esta aqui e, por isso, deliberadamente ausente da lista PT abaixo.
EN_KEYWORDS = r"(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)"
# Referencia aceita pelo GitHub: #N, owner/repo#N, URL completa da issue.
EN_REF = (
    r"(?:[-\w.]+/[-\w.]+)?"
    r"(?:https?://github\.com/[-\w.]+/[-\w.]+/issues/)?"
    r"#?(\d+)"
)
EN_RE = re.compile(
    r"(?i)(?<![\w/])" + EN_KEYWORDS + r"\b[ \t:]*" + EN_REF
)

# --------------------------------------------------------------------------
# FORMA 5 -- conjugacao. A lista de SUFIXOS foi ampliada dentro das 4 FAMILIAS
# ja declaradas (fech*, corrig*, resolv*, encerr*). A lista de VERBOS continua
# fechada POR ESCOLHA: `Sana`/`Soluciona`/`Conserta`/`Elimina` sao residual
# aceito e declarado -- amplia-la sem medir custo de falso positivo e o caminho
# do gate ruidoso que alguem desliga.
# `resolve` PURO continua FORA (grafia identica ao ingles valido; recusa-lo
# seria reprovar um corpo que funciona -- ver forma aceita 2 no cabecalho).
# --------------------------------------------------------------------------
# 🔴 O PRETERITO PERFEITO fica de FORA (`fechou`, `fechei`, `fecharam`,
# `corrigiu`, `resolveu`, ...), e isso foi MEDIDO, nao estilo: com `ou` na
# lista, o corpus A ganha um falso positivo inedito --
#   #233 L34 `Sei que voce fechou as #222-#225 por conflito de governanca`.
# O preterito perfeito e o tempo de RELATAR acao ja ocorrida (de terceiro,
# inclusive), nao o de DECLARAR o fechamento que este PR vai fazer.
# O PARTICIPIO (`fechado`/`corrigido`) continua DENTRO: ja estava na lista
# antiga e e a forma de `**Corrigido** #12`.
PT_KEYWORDS = (
    r"(?:fech(?:a|am|amos|ando|ar|ara|ará|arao|arão|arei|em|e|o"
    r"|ado|ada|ados|adas)"
    r"|corrig(?:e|em|imos|indo|ir|ira|irá|irao|irão|irei|ido|ida|idos|idas)"
    r"|resolv(?:em|emos|endo|er|era|erá|erao|erão|erei|o|ido|ida|idos|idas)"
    r"|encerr(?:a|am|amos|ando|ar|ara|ará|arao|arão|arei|em|e|o"
    r"|ado|ada|ados|adas))"
)

# --------------------------------------------------------------------------
# FORMA 3 -- a LACUNA entre o verbo e a referencia.
# Antes: so `\*{0,2}` (negrito colado). 11 de 11 grafias escapavam -- e foi
# assim que #312, #325 e #330 mergearam verdes com as issues abertas.
# Agora: qualquer caractere desta lista FECHADA de nao-palavra, mais os
# opcionais artigo definido e "issue(s)".
# 🔴 A lista e ENUMERADA, nao `\W`. Dois motivos MEDIDOS:
#   - `(` produz falso positivo inedito no corpus A
#     (#337 L11 -- `issues ja fechados** (#335, #336)`). Fica FORA.
#     `[` sozinho e seguro (`Fecha [#N](url)`) e fica DENTRO.
#   - `_` e caractere de palavra (`\w`), entao `\W` nem seria superconjunto.
# 🔴 PALAVRA na lacuna continua descaracterizando a declaracao:
# `Fecha o **item 4** da issue #216` e prosa e NAO reprova.
# --------------------------------------------------------------------------
PT_GAP = r"[ \t*_~:,\-—–\[]*"
PT_FILLER = (
    PT_GAP
    + r"(?:(?:o|a|os|as)\b" + PT_GAP + r")?"
    + r"(?:issues?\b" + PT_GAP + r")?"
)

# --------------------------------------------------------------------------
# FORMA 6 -- gramatica de referencia SIMETRICA a do ingles.
# Antes o lado PT aceitava so `#(\d+)`: o gate reconhecia `owner/repo#N` e a
# URL quando ela ISENTAVA (EN_REF) e nao a reconhecia quando ela ACUSAVA.
# Isso e fail-open de gramatica -- o lado permissivo mais expressivo que o
# restritivo -- e nao uma lacuna de conveniencia.
# 🔴 O `#` continua OBRIGATORIO nas formas `#N` e `owner/repo#N`. Espelhar o
# `#?` do EN_REF (numero nu) mediu +6 falsos positivos no corpus A
# (`Fecha 2 dos 3 elos`, `fecha ~50 vermelhos`, ...). A URL completa de issue e
# a unica alternativa sem `#`, por ser inequivoca.
# --------------------------------------------------------------------------
PT_REF = (
    r"(?:[-\w.]+/[-\w.]+#"
    r"|https?://github\.com/[-\w.]+/[-\w.]+/issues/"
    r"|#)(\d+)"
)
PT_RE = re.compile(
    r"(?i)(?<![\w/])\*{0,2}" + PT_KEYWORDS + r"\b" + PT_FILLER + PT_REF
)

# --------------------------------------------------------------------------
# FORMA 4 -- POLARIDADE. O discriminante antigo nunca perguntava se a frase
# AFIRMA fechamento: `**Nao fecha #290**` era acusado como se afirmasse.
# Negacao a ESQUERDA do verbo e DENTRO DA MESMA CLAUSULA suprime a acusacao.
# 🔴 A supressao e SEMPRE registrada no log (ver `report_suppressions`): a
# superficie de silenciamento nao fecha em regex -- `Nao e verdade que fecha
# #12` (deve acusar) e `Sem contar o #99, fecha a #12` (nao deve) cabem na
# MESMA janela e querem vereditos OPOSTOS. Residual nao eliminavel tem de ser
# VISIVEL, senao viola o invariante "este gate NUNCA sai 0 em silencio".
# --------------------------------------------------------------------------
NEG_RE = re.compile(
    r"(?i)(?<![\w])(?:n[aã]o|nem|nunca|jamais|sem"
    r"|deixa(?:m|ram|va|vam)?[ \t]+de|deixou[ \t]+de"
    r"|em[ \t]+vez[ \t]+de|ao[ \t]+inv[eé]s[ \t]+de)(?![\w])"
)
CLAUSE_BREAK_RE = re.compile(r"[.;:!?]")

# --------------------------------------------------------------------------
# BALDES DE ZONA -- os dois tem comportamento OPOSTO, e isso foi MEDIDO
# (ML-0C, 7 PRs sonda contra 2 issues descartaveis; ver secao 10 do parecer
# docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md):
#
#   CODIGO     cerca ``` · code span `...` · bloco indentado 4 espacos/tab
#              O GitHub IGNORA a palavra-chave aqui (#428/#429/#431 = []).
#              Logo a isencao inglesa seria FALSA, e a mascara e subtraida dos
#              DOIS matchers -- exatamente como o `blank_code` antigo fazia.
#              🔴 NAO "corrigir" isso: `Fecha #246.` com `Closes #246` so
#              dentro de cerca NAO fecha a issue, entao acusar e o trabalho do
#              gate. Fazer a isencao valer ali instalaria FALSO NEGATIVO --
#              silencio sobre uma declaracao falsa, que e pior que o incomodo
#              de hoje (o aviso atual e VERDADEIRO).
#
#   NAO-CODIGO blockquote · celula de tabela · span entre aspas
#              O GitHub HONRA a palavra-chave aqui (#432/#433/#434 = [430]).
#              A isencao e REAL, entao esta mascara vai num PASSE PROPRIO que
#              suprime so a acusacao PORTUGUESA e NAO e subtraida do scan da
#              isencao inglesa.
#              >>> A lista abaixo esta VAZIA de proposito: as zonas nao-codigo
#                  sao entrega do ML-N2. O que o ML-N1 entrega e o ESQUELETO
#                  de dois passes, exercitado pela zona-sonda abaixo.
# --------------------------------------------------------------------------
FENCE_RE = re.compile(r"(?ms)^[ \t]*(?:```|~~~).*?^[ \t]*(?:```|~~~)[ \t]*$")
SPAN_RE = re.compile(r"`[^`\n]*`")
INDENT_RE = re.compile(r"^(?: {4,}|\t)")

# As TRES zonas do balde NAO-CODIGO (ML-N2). Cada uma corresponde a UMA sonda
# da secao 10.3 do parecer, e as tres devolveram `[430]` -- o GitHub FECHA a
# issue quando a palavra-chave inglesa vive ali. Logo esta lista vai SO para o
# passe portugues; apagar estas zonas do scan ingles instalaria falso positivo.
#
#   BLOCKQUOTE_RE  sonda #432  `> Closes #430`
#   TABLE_ROW_RE   sonda #433  `| Closes #430 | alvo |`
#   QUOTED_SPAN_RE sonda #434  aspas RETAS -- `a linha certa seria "Closes #430"`
#
# 🔴 A linha de tabela exige o pipe de FECHAMENTO (`^|...|$`), e nao so o de
# abertura. Medido no corpus A: as duas grafias apagam exatamente as MESMAS
# 1354 linhas, entao a forma estrita nao custa nada e nao arrasta prosa que
# apenas COMECE com `|`. Das 1354, ZERO carrega declaracao portuguesa com `#N`
# (medido neste ML, nao citado do parecer) -- a divida de largura do `TBL` e
# declarada e, neste corpus, inerte.
#
# ⚠️ ASPAS CURVAS (`“…”`) ficam de FORA, e isso e residual DECLARADO, nao
# esquecimento. A sonda #434 mediu so as RETAS; tratar as curvas junto seria
# extrapolacao por analogia -- e foi a analogia que produziu as duas premissas
# refutadas na Wave 0 desta REQ (secao 10.1 do parecer). Medido: o corpus A tem
# ZERO ocorrencia de aspas curvas, entao incluí-las nao compraria nada
# mensuravel e custaria uma premissa nao medida. Preso por cenario de autoteste.
BLOCKQUOTE_RE = re.compile(r"(?m)^[ \t]*>.*$")
TABLE_ROW_RE = re.compile(r"(?m)^[ \t]*\|[^\n]*\|[ \t]*$")
QUOTED_SPAN_RE = re.compile(r"\"[^\"\n]*\"")

NONCODE_ZONE_RES = [BLOCKQUOTE_RE, TABLE_ROW_RE, QUOTED_SPAN_RE]


def _blank(match):
    """Espacos no lugar do trecho, PRESERVANDO quebras de linha (o numero de
    linha reportado continua batendo com o corpo original)."""
    return re.sub(r"[^\n]", " ", match.group(0))


def mask_indented_code(text, ref=None):
    """Bloco indentado por 4 espacos/tab -- balde de CODIGO (sonda #431 = []).
    Um bloco so COMECA depois de linha em branco; isso evita mascarar
    continuacao de paragrafo e de item de lista, que nao sao codigo.

    🔴 `ref` e a DIVIDA DE LARGURA do ML-N1, reconciliada e CORRIGIDA aqui.
    O ML-N1 declarou que esta mascara apagava 304 linhas do corpus A contra as
    80 do censo do parecer, e registrou que nao sabia reconstruir a regra do
    censo. Reconstruida por medicao neste ML: as 80 sao a contagem linha-a-
    linha sobre o corpo CRU; as 304 apareciam porque a indentacao era medida
    DEPOIS do mascaramento de code span, e uma linha que COMECA com crase
    (``trackfw validate` -- Fecha #246.`) vira 4+ espacos a esquerda quando o
    span e apagado. Indentacao FABRICADA pelo passe anterior, nao escrita pelo
    autor.

    Isso nao era contabilidade: era FALSO NEGATIVO vivo. Medido antes do fix, o
    corpo `Texto.\\n\\n`trackfw validate` -- Fecha #246.\\n` saia rc=0 -- o gate
    calava sobre uma declaracao portuguesa real. O "0 das 304 carrega
    declaracao" do ML-N1 era propriedade do CORPUS, nao da regra, exatamente
    como o "0 falso positivo" da secao 3.4 do parecer.

    A decisao de indentacao passa a sair de `ref` (o corpo como o autor
    escreveu) e o apagamento continua acontecendo em `text` (ja sem cerca e sem
    span). As duas arvores tem o MESMO numero de linhas porque `_blank`
    preserva as quebras -- e se algum dia nao tiverem, o fallback e medir no
    proprio `text`, que e o comportamento antigo, nunca um IndexError."""
    lines = text.split("\n")
    ref_lines = lines if ref is None else ref.split("\n")
    if len(ref_lines) != len(lines):
        ref_lines = lines
    out = []
    prev_blank = True
    in_block = False
    for line, ref_line in zip(lines, ref_lines):
        blank = not ref_line.strip()
        indented = bool(INDENT_RE.match(ref_line))
        if in_block:
            if blank:
                out.append(line)
                continue
            if indented:
                out.append(re.sub(r"[^\n]", " ", line))
                continue
            in_block = False
        elif indented and prev_blank and not blank:
            in_block = True
            out.append(re.sub(r"[^\n]", " ", line))
            prev_blank = False
            continue
        out.append(line)
        prev_blank = blank
    return "\n".join(out)


def mask_code_zones(text):
    """Passe 1 -- subtraido dos DOIS matchers (PT e EN)."""
    masked = SPAN_RE.sub(_blank, FENCE_RE.sub(_blank, text))
    return mask_indented_code(masked, ref=text)


def mask_noncode_zones(text):
    """Passe 2 -- subtraido SO do matcher portugues. A isencao inglesa
    atravessa este passe intacta, porque nestas zonas o GitHub fecha mesmo.

    A zona-sonda `@@PROBE@@` do ML-N1 (ativada por PRCLOSE_SELFCHECK_NONCODE)
    foi REMOVIDA aqui, com a razao escrita: ela existia porque a lista de zonas
    estava vazia e um esqueleto que ninguem exercita e um esqueleto que o ML
    seguinte recomeca. As tres zonas reais exercitam o mesmo par de
    propriedades (suprime PT · preserva EN), cada uma individualmente e com
    verdade-terreno medida na API -- a sonda virou redundante, e uma via de
    codigo ligada por variavel de ambiente que so o autoteste usa e superficie
    a menos quando nao e mais necessaria."""
    for rx in NONCODE_ZONE_RES:
        text = rx.sub(_blank, text)
    return text


def negation_in_clause(line, verb_start):
    """Token de negacao a esquerda do verbo, DENTRO da mesma clausula.
    Quebradores de clausula: . ; : ! ? -- e o inicio da linha, porque o laco de
    acusacao ja e por linha (fim de linha quebra clausula de graca).
    Devolve o token que suprime, ou None."""
    left = line[:verb_start]
    cut = 0
    for br in CLAUSE_BREAK_RE.finditer(left):
        cut = br.end()
    clause = left[cut:]
    token = None
    for neg in NEG_RE.finditer(clause):
        token = neg.group(0)
    return token


def main():
    if len(sys.argv) != 2:
        print("not_evaluated: uso: matcher.py <arquivo-com-corpo-do-pr>")
        return 2
    try:
        with open(sys.argv[1], "rb") as fh:
            raw = fh.read()
    except OSError as exc:
        print("not_evaluated: nao consegui ler o corpo do PR (%s)" % exc)
        return 2
    body = raw.decode("utf-8", errors="replace")
    if not body.strip():
        print("not_evaluated: corpo do PR vazio -- nada a avaliar.")
        print("  Um PR sem corpo nao pode fechar issue nenhuma e nao passa por")
        print("  este gate em silencio. Use .github/PULL_REQUEST_TEMPLATE.md.")
        return 2

    # DOIS PASSES, por BALDE -- ver o bloco "BALDES DE ZONA" acima.
    # O scan do INGLES leva so a mascara de CODIGO: nas zonas nao-codigo a
    # isencao e real (medido) e nao pode ser apagada.
    scan_en = mask_code_zones(body)
    # O scan do PORTUGUES leva as duas mascaras: nas zonas nao-codigo a
    # declaracao portuguesa e citacao, nao intencao.
    scan_pt = mask_noncode_zones(scan_en)
    original_lines = body.splitlines()

    english = {int(n) for n in EN_RE.findall(scan_en)}

    offenders = []
    suppressed = []
    for idx, line in enumerate(scan_pt.split("\n"), 1):
        for m in PT_RE.finditer(line):
            num = int(m.group(1))
            # 🔴 NAO reescrever esta condicao como `if num in english: continue`.
            # O cenario s182 do check-gates-falsify.sh sabota EXATAMENTE esta
            # linha (`if num not in english:` -> `if not english:`) para provar
            # que a isencao e POR NUMERO e nao global. Renomea-la faz aquele
            # cenario parar de medir o que promete -- e ele reprova dizendo
            # isso, em vez de passar verde. Medido em 2026-09-25.
            if num not in english:
                shown = original_lines[idx - 1] if idx <= len(original_lines) else line
                neg = negation_in_clause(line, m.start())
                if neg:
                    suppressed.append((idx, m.group(0).strip(), num, shown.strip(), neg))
                else:
                    offenders.append((idx, m.group(0).strip(), num, shown.strip()))

    def report_suppressions():
        # 🔴 A supressao por polaridade SAI NO LOG, sempre, nos dois caminhos de
        # saida. A superficie de silenciamento nao se fecha em regex (um
        # intensificador afirmativo como "Sem duvida, fecha a #12" contem token
        # de negacao); como o residual nao e eliminavel, ele tem de ser VISIVEL.
        if not suppressed:
            return
        print("")
        print("NOTA [pr-closing-keyword]: %d acusacao(oes) SUPRIMIDA(S) por"
              % len(suppressed))
        print("     polaridade -- o gate leu a frase como NEGANDO o fechamento.")
        for idx, frag, num, shown, neg in suppressed:
            print("  linha %d: %s" % (idx, shown))
            print("           ^ \"%s\" nao foi acusada: o token \"%s\" nega o"
                  % (frag, neg))
            print("             fechamento na mesma clausula, a esquerda do verbo.")
        print("")
        print("  🔴 Se alguma destas frases AFIRMA fechamento, o gate errou aqui e")
        print("     a issue #%s vai continuar aberta apos o merge. A leitura de"
              % ", #".join(str(s[2]) for s in suppressed))
        print("     polaridade e heuristica declarada, nao analise sintatica.")

    if not offenders:
        print("OK   [pr-closing-keyword]: nenhuma palavra-chave de fechamento em")
        print("     portugues sem a forma inglesa correspondente.")
        if english:
            print("     Issues que o GitHub vai fechar no merge: %s"
                  % ", ".join("#%d" % n for n in sorted(english)))
        report_suppressions()
        return 0

    print("FAIL [pr-closing-keyword]: palavra-chave de fechamento em PORTUGUES.")
    print("     O GitHub NAO fecha issue com ela. O merge passa, o texto afirma")
    print("     que fechou, e a issue continua aberta.")
    print("")
    for idx, frag, num, shown in offenders:
        print("  linha %d: %s" % (idx, shown))
        print("           ^ \"%s\" nao fecha a issue #%d." % (frag, num))
        print("           Troque por: Closes #%d   (ou Fixes/Resolves #%d)"
              % (num, num))
    print("")
    print("  A linha de fechamento e SINTAXE DO GITHUB, nao prosa: o corpo do PR")
    print("  continua em portugues. Ver .github/PULL_REQUEST_TEMPLATE.md.")
    report_suppressions()
    return 1


if __name__ == "__main__":
    sys.exit(main())
PY_EOF

# ---------------------------------------------------------------------------
# UNICO ponto de avaliacao. CI e --self-test passam os dois por aqui.
# ---------------------------------------------------------------------------
evaluate_body_file() {
  python3 "$MATCHER" "$1"
}

# ---------------------------------------------------------------------------
# Autoteste por fixtures -- falsificacao nas duas direcoes
# ---------------------------------------------------------------------------
self_test() {
  local failures=0

  assert_body() { # assert_body LABEL EXPECTED_EXIT EXPECTED_SUBSTR BODY
    local label=$1 expected=$2 needle=$3 body=$4
    local f="$WORK/fixture.md" out status
    printf '%s' "$body" >"$f"
    set +e
    out=$(evaluate_body_file "$f" 2>&1)
    status=$?
    set -e
    if [[ $status -ne $expected ]]; then
      echo "FAIL [pr-closing-keyword/self-test/$label]: exit $status, esperava $expected" >&2
      printf '%s\n' "$out" | sed 's/^/    /' >&2
      failures=$((failures + 1))
      return
    fi
    if [[ -n $needle ]] && ! grep -qF -- "$needle" <<<"$out"; then
      echo "FAIL [pr-closing-keyword/self-test/$label]: exit $status correto, mas falta '$needle'" >&2
      printf '%s\n' "$out" | sed 's/^/    /' >&2
      failures=$((failures + 1))
      return
    fi
    echo "OK   [pr-closing-keyword/self-test/$label]"
  }

  # --- Direcao A: DETECTA a forma portuguesa (exit 1, nomeando a linha) ------
  assert_body "detecta-fecha"      1 'linha 1: Fecha #123.'          'Fecha #123.'
  assert_body "detecta-corrige-artigo" 1 'Corrige o #239'            'Corrige o #239.'
  assert_body "detecta-issue-explicita" 1 'Encerra a issue #12'      'Encerra a issue #12'
  assert_body "detecta-enfase-md"  1 'nao fecha a issue #12'         '**Corrigido** #12'
  assert_body "detecta-resolvido"  1 'nao fecha a issue #12'         'Resolvido #12'
  assert_body "sugere-forma-certa" 1 'Closes #123'                   'Fecha #123.'

  # --- 🔴 A isencao e POR NUMERO, nao "tem ingles em algum lugar" -----------
  assert_body "ingles-de-outra-issue-nao-isenta" 1 'nao fecha a issue #246' \
    $'Fecha #246.\n\nAlgum texto.\n\nFixes #999\n'
  assert_body "ingles-da-mesma-issue-isenta"     0 '' \
    $'Fecha #246.\n\nAlgum texto.\n\nFixes #246\n'

  # --- Direcao B: NAO reprova o que e valido ou e prosa (exit 0) -------------
  assert_body "aceita-closes"        0 'Issues que o GitHub vai fechar' 'Closes #123'
  assert_body "aceita-fixes"         0 ''  'Fixes #123'
  assert_body "aceita-resolve-ingles" 0 '' 'Resolve #123'
  assert_body "aceita-owner-repo"    0 ''  $'Fecha #12\n\nFixes kgsaran/trackfw#12\n'
  assert_body "aceita-url-completa"  0 ''  $'Fecha #12\n\nCloses https://github.com/kgsaran/trackfw/issues/12\n'
  # Prosa real, extraida de corpos de PR mergeados deste repositorio:
  assert_body "prosa-mesmo-sitio"    0 ''  'A regressao esta no mesmo sitio do #238.'
  assert_body "prosa-portado-de"     0 ''  'Cenario portado do #223, sem alteracao.'
  assert_body "prosa-item-da-issue"  0 ''  'Fecha o **item 4** da issue #216 -- o ultimo dos onze defeitos.'
  assert_body "prosa-tres-defeitos"  0 ''  'Corrige os tres defeitos do #232. Um arquivo.'
  assert_body "prosa-governanca-pr"  0 ''  'Fecha a governanca do PR #145 (ja mergeado).'
  assert_body "prosa-req-sem-numero" 0 ''  'Fecha a REQ-2026-08-04-json-marshalindent-do-go-escapa-html.'
  # Exemplo citado em code span / cerca -- e o que ESTE PR faz.
  assert_body "code-span-nao-reprova" 0 '' 'A forma errada e `Fecha #246`, que nao fecha nada.'
  assert_body "cerca-nao-reprova"     0 '' $'Exemplo do defeito:\n\n```\nFecha #246.\n```\n\nFim.\n'

  # ==========================================================================
  # ML-N1 -- formas 3, 4, 5 e 6 + baldes de zona.
  # 🔴 Regra Dura de Reconciliacao: cada bloco declara, em UMA frase, qual
  # conclusao do ML-N1 aquele teste afirma. Teste para o qual a frase nao pode
  # ser escrita nao deveria existir.
  # ==========================================================================

  # --- BALDE DE CODIGO (cerca · code span · bloco indentado) ----------------
  # AFIRMA: o GitHub IGNORA a palavra-chave inglesa nestas tres zonas (sondas
  # #428/#429/#431 = []), entao a isencao ali seria FALSA e o corpo NAO fecha a
  # issue -- acusar e o trabalho do gate, e nao um falso positivo a corrigir.
  assert_body "balde-codigo-cerca-nao-isenta"  1 'nao fecha a issue #246' \
    $'Fecha #246.\n\n```\nCloses #246\n```\n'
  assert_body "balde-codigo-span-nao-isenta"   1 'nao fecha a issue #246' \
    $'Fecha #246. A forma certa seria `Closes #246`.\n'
  # 🔴 Zona NOVA do ML-N1 -- era rc=0 (falso negativo silencioso) ate aqui.
  assert_body "balde-codigo-indentado-nao-isenta" 1 'nao fecha a issue #246' \
    $'Fecha #246.\n\n    Closes #246\n\nFim.\n'
  # Contra-braco do bloco indentado: a acusacao PORTUGUESA continua suprimida
  # ali, por dois motivos independentes -- e citacao, E nao fecha nada.
  # AFIRMA: o balde de CODIGO nao foi desfeito por este ML; so a decisao de
  # ONDE a indentacao e medida mudou (ver `indent-*` abaixo).
  assert_body "balde-codigo-indentado-suprime-acusacao-pt" 0 '' \
    $'Exemplo do defeito:\n\n    Fecha #246\n\nFim.\n'

  # ==========================================================================
  # ML-N2 -- as TRES zonas do balde NAO-CODIGO + a divida de largura do INDENT.
  # 🔴 Regra Dura de Reconciliacao: cada bloco declara, em UMA frase, qual
  # conclusao do ML-N2 aquele teste afirma.
  # ==========================================================================

  # --- BALDE NAO-CODIGO -- os DOIS lados, zona por zona ---------------------
  # AFIRMA (lado 1): nas tres zonas que o GitHub HONRA (#432/#433/#434 = [430])
  # a declaracao portuguesa e CITACAO, nao intencao, e o passe proprio a
  # suprime -- cada zona exercitada INDIVIDUALMENTE, nunca em bloco.
  assert_body "naocodigo-blockquote-suprime-acusacao-pt" 0 '' \
    $'O reporter escreveu:\n\n> Fecha #246.\n'
  assert_body "naocodigo-tabela-suprime-acusacao-pt" 0 '' \
    $'| forma | veredito |\n| --- | --- |\n| Fecha #246 | nao fecha nada |\n'
  # 🔴 A frase CANONICA do issue #258 -- a propria descricao do defeito. Ela
  # passa EXCLUSIVAMENTE pela zona de aspas: retiradas as aspas nao sobra sinal
  # nenhum (secao 3.3 do parecer). E a razao medida de o AC3 ter sido ampliado.
  assert_body "naocodigo-aspas-frase-canonica-do-258" 0 '' \
    'O corpo da minha PR #247 dizia "Fecha #246" e a issue continuou aberta.'
  # Controle da frase do #258: SEM as aspas nao sobra sinal, e o gate acusa.
  # AFIRMA: a isencao vem da ZONA, nao da prosa em volta -- se este braco
  # calasse, a zona de aspas estaria comprando silencio que nao e dela.
  assert_body "naocodigo-aspas-controle-sem-aspas-acusa" 1 'nao fecha a issue #246' \
    'O corpo da minha PR dizia Fecha #246 e a issue continuou aberta.'

  # AFIRMA (lado 2): nas MESMAS tres zonas a isencao INGLESA atravessa intacta
  # -- o GitHub fecha de verdade ali, entao apagar a zona do scan ingles seria
  # falso positivo (secao 3.5-iv, 3 de 5 linhas confirmadas pelo ML-0C).
  # O needle e load-bearing: ele prova que o numero entrou no conjunto
  # `english`, e nao apenas que a acusacao sumiu por outro caminho.
  # 🔴 Este cenario ja existia no ML-N1 e passava por AUSENCIA de mascara;
  # agora passa porque a mascara esta no passe CERTO. Mesma assercao, virou
  # load-bearing.
  assert_body "balde-naocodigo-blockquote-isenta" 0 'Issues que o GitHub vai fechar' \
    $'Fecha #246.\n\n> Closes #246\n'
  assert_body "balde-naocodigo-tabela-isenta" 0 'Issues que o GitHub vai fechar' \
    $'Fecha #246.\n\n| Closes #246 | alvo |\n'
  # A linha da secao 10.4 do parecer. 🔴 Ela passa por um mecanismo DIFERENTE
  # dos dois acima e da frase do #258: a palavra-chave inglesa dentro das aspas
  # NAO e apagada do scan ingles, entao a isencao POR NUMERO isenta a
  # declaracao portuguesa da MESMA linha. E o unico cenario que exercita a
  # INTERACAO entre os dois passes, em vez de cada mascara isolada.
  assert_body "balde-naocodigo-aspas-isenta" 0 'Issues que o GitHub vai fechar' \
    'Fecha #246. A linha certa e "Closes #246".'

  # --- RESIDUAL DECLARADO: aspas CURVAS ------------------------------------
  # AFIRMA: aspas curvas NAO sao zona, e isso e escolha medida, nao omissao --
  # a sonda #434 mediu so as retas, o corpus A tem ZERO ocorrencia de curvas, e
  # trata-las junto seria a extrapolacao por analogia que a Wave 0 ja viu ser
  # refutada duas vezes. Se um dia forem medidas e entrarem, este cenario fica
  # vermelho e a decisao volta a ser consciente, em vez de silenciosa.
  assert_body "residual-aspas-curvas-nao-sao-zona" 1 'nao fecha a issue #246' \
    'O corpo dizia “Fecha #246” e a issue continuou aberta.'

  # --- DIVIDA DE LARGURA DO INDENT, reconciliada ---------------------------
  # AFIRMA: a indentacao passa a ser medida no corpo COMO O AUTOR ESCREVEU, e
  # nao depois do mascaramento de code span -- que fabricava 4 espacos a
  # esquerda numa linha iniciada por crase e SILENCIAVA a declaracao. Medido
  # rc=0 antes do fix e rc=1 depois; e o mesmo defeito desta REQ, na direcao do
  # falso negativo.
  assert_body "indent-span-inicial-nao-fabrica-bloco" 1 'nao fecha a issue #246' \
    $'Texto.\n\n`trackfw validate` — Fecha #246.\n'
  # Contra-braco: o bloco indentado de VERDADE continua sendo zona de codigo --
  # a correcao nao desligou a mascara, so mudou onde ela le a indentacao.
  assert_body "indent-bloco-real-continua-mascarado" 0 '' \
    $'Exemplo:\n\n    Fecha #246\n    Fecha #247\n\nFim.\n'

  # --- COSTURA entre o passe de nao-codigo e a FORMA 4 (polaridade) ---------
  # AFIRMA: a zona de aspas apaga DENTRO da linha, e por isso ela alcanca a
  # leitura de polaridade -- um token de negacao que vive numa CITACAO deixa de
  # suprimir a declaracao do autor. Medido: este corpo saia rc=0 antes do ML-N2
  # e sai rc=1 depois, e e a UNICA divergencia entre os dois gates em 7 arranjos
  # de negacao x aspas x blockquote x tabela que eu exercitei.
  # 🔴 A direcao nova e a CORRETA, e e a fail-closed: a clausula, como o autor a
  # escreveu, AFIRMA o fechamento; quem nega e a frase citada, nao ele. De
  # quebra, fecha uma superficie de SILENCIAMENTO -- e silencio e o que esta
  # secao inteira existe para nao produzir.
  assert_body "costura-negacao-citada-nao-suprime" 1 'nao fecha a issue #246' \
    'Ele disse "nao" e fecha a #246'
  # Contra-braco: quando a negacao E o verbo vivem DENTRO das mesmas aspas, o
  # corpo continua saindo 0 -- mas por outro mecanismo (a ZONA apaga os dois),
  # e nao pela polaridade. Mesmo veredito, causa diferente: e o par que prova
  # que o cenario acima mede a costura, e nao a zona sozinha.
  assert_body "costura-negacao-e-verbo-na-mesma-citacao" 0 '' \
    'Ele disse "nao fecha a #246" no corpo'

  # --- FORMA 3 -- a lacuna entre o verbo e a referencia ---------------------
  # AFIRMA: a lacuna aberta a nao-palavra passa a acusar as declaracoes reais
  # que mergearam verdes com a issue aberta (#312, #325, #330) -- e PALAVRA na
  # lacuna continua descaracterizando a declaracao.
  assert_body "forma3-negrito"      1 'nao fecha a issue #274' 'Fecha **#274** e **#275**.'
  assert_body "forma3-dois-pontos"  1 'nao fecha a issue #421' 'Fecha: #421.'
  assert_body "forma3-link-md"      1 'nao fecha a issue #274' 'Fecha [#274](https://x/274)'
  assert_body "forma3-travessao"    1 'nao fecha a issue #314' 'Fecha — #314'
  assert_body "forma3-real-325"     1 'nao fecha a issue #314' \
    'Fecha **#314** e **#319** — ambos sobre o mesmo script.'
  assert_body "forma3-real-330"     1 'nao fecha a issue #320' 'Fechar o **#320** pela causa raiz.'
  # Contra-bracos: PALAVRA na lacuna e PARENTESE continuam fora.
  assert_body "forma3-contra-item4" 0 '' \
    'Fecha o **item 4** da issue #216 -- o ultimo dos onze defeitos.'
  assert_body "forma3-contra-parenteses" 0 '' 'issues ja fechados** (#335, #336)'

  # --- FORMA 4 -- polaridade ------------------------------------------------
  # AFIRMA: negacao a esquerda do verbo na MESMA clausula suprime a acusacao --
  # e e por isso que a forma 3 nao pode entrar sozinha, sob pena de criar falso
  # positivo onde hoje ha silencio.
  assert_body "forma4-nega-simples"  0 'SUPRIMIDA' 'Não fecha #421.'
  assert_body "forma4-nega-negrito"  0 'SUPRIMIDA' 'Não fecha **#421**.'
  assert_body "forma4-nega-real-293" 0 'SUPRIMIDA' '**Não fecha #290** (usage sujo no Go)'
  assert_body "forma4-sem-fechar"    0 'SUPRIMIDA' 'Entrega sem fechar a #12.'
  assert_body "forma4-deixa-de"      0 'SUPRIMIDA' 'Deixa de fechar a #12.'
  # Contra-bracos: negacao em OUTRA clausula ou A DIREITA nao suprime.
  assert_body "forma4-contra-outra-clausula" 1 'nao fecha a issue #421' \
    'Isto não é refactor. Fecha #421.'
  assert_body "forma4-contra-dois-pontos"    1 'nao fecha a issue #12' \
    'Sem mais delongas: fecha a #12.'
  assert_body "forma4-contra-nega-a-direita" 1 'nao fecha a issue #12' \
    'Fecha a #12, não a #13.'

  # --- I3: a supressao por polaridade e falsificada nos DOIS matchers -------
  # AFIRMA: a leitura de polaridade e UNILATERAL -- roda so no laco PORTUGUES.
  # O matcher INGLES nao tem nocao de polaridade, entao `Closes #12` dentro de
  # uma clausula negada CONTINUA isentando, e o corpo sai 0.
  # 🔴 Isto e residual DECLARADO (secao 6.6 do parecer), nao defeito: o parser
  # do GitHub tambem ignora a negacao e fecha a #12 nesse corpo -- o gate
  # acerta o veredito. O cenario existe para PRENDER esse comportamento: se um
  # dia a polaridade for aplicada tambem ao ingles, este teste fica vermelho e
  # a decisao volta a ser consciente, em vez de silenciosa.
  assert_body "i3-polaridade-nao-alcanca-o-matcher-ingles" 0 'Issues que o GitHub vai fechar' \
    $'Fecha #12.\n\nNão fecha #12 em ingles: Closes #12\n'

  # --- FORMA 5 -- conjugacao ------------------------------------------------
  # AFIRMA: as conjugacoes que a lista antiga perdia passam a ser acusadas
  # DENTRO das 4 familias declaradas -- e o preterito perfeito fica de fora
  # porque relatar acao passada nao e declarar fechamento (#233 L34 medido).
  assert_body "forma5-1a-pessoa"  1 'nao fecha a issue #12' 'Fecho a #12 com este PR.'
  assert_body "forma5-futuro"     1 'nao fecha a issue #12' 'Fechará a #12 no merge.'
  assert_body "forma5-gerundio"   1 'nao fecha a issue #12' 'Fechando a #12.'
  assert_body "forma5-encerrando" 1 'nao fecha a issue #12' 'Encerrando a #12.'
  assert_body "forma5-contra-preterito" 0 '' \
    'Sei que voce fechou as #222 por conflito de governanca.'
  # A lista de VERBOS continua fechada -- residual aceito e DECLARADO.
  assert_body "forma5-residual-verbo-fora-da-lista" 0 '' 'Sana o #12.'

  # --- FORMA 6 -- gramatica de referencia simetrica -------------------------
  # AFIRMA: o lado portugues passa a reconhecer as mesmas referencias que o
  # lado ingles ja reconhecia para ISENTAR -- fechando o fail-open de gramatica
  # -- mas o `#` continua obrigatorio, porque numero nu mediu +6 FP.
  assert_body "forma6-owner-repo" 1 'nao fecha a issue #12' 'Fecha kgsaran/trackfw#12'
  assert_body "forma6-url-issue"  1 'nao fecha a issue #12' \
    'Fecha https://github.com/kgsaran/trackfw/issues/12'
  assert_body "forma6-contra-numero-nu" 0 '' 'Fecha 2 dos 3 elos (#148, #149)'
  assert_body "forma6-contra-issue-sem-cerquilha" 0 '' 'Fecha a issue 12'
  # A isencao POR NUMERO continua valendo nas formas novas.
  assert_body "forma6-isencao-owner-repo" 0 '' \
    $'Fecha kgsaran/trackfw#12\n\nFixes #12\n'

  # --- Vacuidade ------------------------------------------------------------
  assert_body "vacuidade-corpo-vazio"    2 'not_evaluated: corpo do PR vazio' ''
  assert_body "vacuidade-so-espacos"     2 'not_evaluated: corpo do PR vazio' $'   \n\n\t\n'
  set +e
  out=$(evaluate_body_file "$WORK/nao-existe-este-arquivo.md" 2>&1); status=$?
  set -e
  if [[ $status -eq 2 ]] && grep -qF 'not_evaluated' <<<"$out"; then
    echo "OK   [pr-closing-keyword/self-test/vacuidade-arquivo-ausente]"
  else
    echo "FAIL [pr-closing-keyword/self-test/vacuidade-arquivo-ausente]: exit $status" >&2
    failures=$((failures + 1))
  fi
  # Fora de pull_request / sem payload: o resolvedor de corpo tem de dar 2.
  set +e
  out=$(env -u PR_BODY_FILE -u PR_NUMBER -u GITHUB_EVENT_PATH \
        GITHUB_EVENT_NAME=push "$ROOT_DIR/scripts/check-pr-closing-keyword.sh" 2>&1); status=$?
  set -e
  if [[ $status -eq 2 ]] && grep -qF 'not_evaluated' <<<"$out"; then
    echo "OK   [pr-closing-keyword/self-test/vacuidade-fora-de-pull-request]"
  else
    echo "FAIL [pr-closing-keyword/self-test/vacuidade-fora-de-pull-request]: exit $status" >&2
    printf '%s\n' "$out" | sed 's/^/    /' >&2
    failures=$((failures + 1))
  fi

  if [[ $failures -gt 0 ]]; then
    echo "FAIL [pr-closing-keyword]: $failures cenario(s) de autoteste falharam." >&2
    exit 1
  fi
  echo "OK   [pr-closing-keyword]: autoteste completo (deteccao + prosa + isencao por numero +"
  echo "     baldes de zona + formas 3/4/5/6 nas duas direcoes + vacuidade)."
  exit 0
}

# ---------------------------------------------------------------------------
# Resolucao do corpo do PR (modo padrao)
# ---------------------------------------------------------------------------
not_evaluated() {
  echo "not_evaluated [pr-closing-keyword]: $1" >&2
  echo "  Este gate so mede o corpo de um pull request. Ele NAO passa em" >&2
  echo "  silencio quando nao consegue medir -- sai 2, de proposito." >&2
  echo "  Fontes aceitas: PR_BODY_FILE=<arquivo> | GITHUB_EVENT_PATH com" >&2
  echo "  GITHUB_EVENT_NAME=pull_request | --pr <n> (exige \`gh\` autenticado)." >&2
  echo "  Sem contexto de PR (ex.: \`make parity\`), use --self-test." >&2
  exit 2
}

PR_NUMBER_ARG=""
case "${1:-}" in
  --self-test) self_test ;;
  --pr) PR_NUMBER_ARG="${2:-}"; [[ -n $PR_NUMBER_ARG ]] || not_evaluated "--pr exige um numero" ;;
  "") ;;
  *) echo "uso: $0 [--self-test | --pr <n>]" >&2; exit 2 ;;
esac

BODY_FILE="$WORK/body.md"

if [[ -n ${PR_BODY_FILE:-} ]]; then
  [[ -f $PR_BODY_FILE ]] || not_evaluated "PR_BODY_FILE=$PR_BODY_FILE nao existe"
  cp "$PR_BODY_FILE" "$BODY_FILE"
elif [[ -n ${GITHUB_EVENT_PATH:-} ]]; then
  [[ ${GITHUB_EVENT_NAME:-} == pull_request || ${GITHUB_EVENT_NAME:-} == pull_request_target ]] \
    || not_evaluated "GITHUB_EVENT_NAME='${GITHUB_EVENT_NAME:-<vazio>}' nao e pull_request"
  [[ -f $GITHUB_EVENT_PATH ]] || not_evaluated "GITHUB_EVENT_PATH=$GITHUB_EVENT_PATH nao existe"

  # O payload do evento e IMUTAVEL: ele guarda o corpo de quando o PR foi ABERTO.
  # Corrigir o corpo no GitHub e reexecutar o job devolve o mesmo veredito, porque o
  # re-run reexecuta com o MESMO payload -- medido no PR #409 (issue #258). Por isso,
  # quando da para perguntar ao GitHub qual e o corpo AGORA, o corpo vivo vence.
  #
  # Ordem: corpo vivo pela API -> payload. Nunca o contrario, e nunca so a API: sem
  # `gh` autenticado (fork sem segredo, execucao local) o payload ainda mede algo.
  # O numero vai para ARQUIVO, nao para stdout capturado. Dois motivos, os dois
  # medidos: (1) no Windows o python3 traduz \n em \r\n no stdout, e um numero com
  # \r invisivel passaria no [[ -n ]] e viraria argumento invalido para o gh —
  # exatamente o silencio da REQ do CRLF; (2) o cenario s182 do check-gates-falsify
  # COPIA este gate para fora de scripts/, onde um `source` da lib de normalizacao
  # nao resolveria. `newline="\n"` resolve (1) na origem, e a escrita em arquivo
  # evita (2) sem duplicar helper.
  PR_NUMBER_FILE="$WORK/event-pr-number.txt"
  python3 - "$GITHUB_EVENT_PATH" "$PR_NUMBER_FILE" <<'PY' || true
import json, sys
try:
    with open(sys.argv[1], encoding="utf-8") as fh:
        ev = json.load(fh)
except Exception:
    raise SystemExit(0)
n = (ev.get("pull_request") or {}).get("number")
if isinstance(n, int):
    with open(sys.argv[2], "w", encoding="utf-8", newline="\n") as out:
        out.write(str(n))
PY
  EVENT_PR_NUMBER=""
  [[ -f $PR_NUMBER_FILE ]] && EVENT_PR_NUMBER=$(tr -d '\r' <"$PR_NUMBER_FILE")
  if [[ -n ${EVENT_PR_NUMBER:-} ]] && command -v gh >/dev/null 2>&1 \
     && gh pr view "$EVENT_PR_NUMBER" --json body -q .body >"$BODY_FILE" 2>/dev/null; then
    echo "  fonte: API (corpo vivo do PR #$EVENT_PR_NUMBER) -- o payload do evento e imutavel"
    evaluate_body_file "$BODY_FILE"
    exit 0
  fi
  # Sem API: o payload volta a ser a fonte, e o gate DIZ que esta lendo o corpo de
  # abertura -- para quem le o log nao concluir que reexecutar resolveria.
  if [[ -n ${EVENT_PR_NUMBER:-} ]]; then
    echo "  fonte: payload do evento (corpo de ABERTURA do PR #$EVENT_PR_NUMBER)."
    echo "  \`gh\` indisponivel ou sem permissao: se o corpo foi editado depois, reexecutar"
    echo "  este job NAO muda o veredito -- feche e reabra o PR, ou rode com \`--pr $EVENT_PR_NUMBER\`."
  fi
  # json.load, nunca grep/sed: um corpo com \n escapado destroi extracao
  # orientada a linha e o resultado seria uma leitura parcial SILENCIOSA.
  python3 - "$GITHUB_EVENT_PATH" "$BODY_FILE" <<'PY' || not_evaluated "payload sem .pull_request.body legivel"
import json, sys
with open(sys.argv[1], encoding="utf-8") as fh:
    ev = json.load(fh)
body = (ev.get("pull_request") or {}).get("body")
if body is None:
    sys.exit(1)
with open(sys.argv[2], "w", encoding="utf-8", newline="\n") as out:
    out.write(body)
PY
elif [[ -n $PR_NUMBER_ARG || -n ${PR_NUMBER:-} ]]; then
  command -v gh >/dev/null 2>&1 || not_evaluated "\`gh\` nao esta no PATH"
  gh pr view "${PR_NUMBER_ARG:-$PR_NUMBER}" --json body -q .body >"$BODY_FILE" \
    || not_evaluated "\`gh pr view\` falhou para o PR ${PR_NUMBER_ARG:-$PR_NUMBER}"
else
  not_evaluated "nenhuma fonte de corpo de PR disponivel"
fi

evaluate_body_file "$BODY_FILE"
