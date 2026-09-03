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
# RECUSA (exit 1) -- palavra-chave PORTUGUESA de fechamento ADJACENTE a `#N`,
#   opcionalmente com artigo definido e/ou a palavra "issue" no meio, e SEM
#   forma inglesa valida para AQUELE MESMO numero:
#     "Fecha #246."            "Corrige o #239."      "Encerra a issue #12"
#     "**Corrigido** #12"      "Fecham #12"           "Resolvido #12"
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
#   4. Trechos em cerca de codigo (```) e em code span (`...`) sao removidos
#      antes de casar. Motivo: DOCUMENTAR a forma errada e o que este proprio
#      PR faz. Um exemplo citado nao e uma declaracao de intencao.
#
# NAO COBERTO (limite declarado, nao acidente):
#   - Parafrase com palavras intervenientes: "este PR fecha, por fim, a #246".
#     Deliberado -- ver item 3 acima; cobrir isso custa o falso positivo zero.
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
#
# ============================================================================
# MODOS
# ============================================================================
#   --self-test  autoteste por fixtures (usado por `make parity`, onde nao ha
#                contexto de PR). Falsifica nas DUAS direcoes pelo MESMO codigo
#                que o CI usa -- nao ha segunda copia do matcher.
#   (padrao)     le o corpo do PR de, nesta ordem:
#                  PR_BODY_FILE  -> caminho de arquivo
#                  GITHUB_EVENT_PATH + GITHUB_EVENT_NAME=pull_request
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

# Portugues: SEM `resolve` puro (ver cabecalho do .sh, forma aceita 2).
PT_KEYWORDS = (
    r"(?:fecha(?:m|r|do|da|dos|das)?"
    r"|corrig(?:e|em|ir|ido|ida|idos|idas)"
    r"|resolv(?:em|er|ido|ida|idos|idas)"
    r"|encerra(?:m|r|do|da|dos|das)?)"
)
# Entre a palavra-chave e o `#N` so pode haver: enfase markdown, artigo
# definido (o/a/os/as), e/ou a palavra "issue(s)". Qualquer outra palavra
# (item, defeito, governanca, tres, ultimo...) descaracteriza a declaracao de
# fechamento e vira prosa -> nao reprova.
PT_FILLER = (
    r"(?:[ \t]*\*{0,2}\b(?:o|a|os|as)\b\*{0,2})?"
    r"(?:[ \t]*\*{0,2}\b(?:a[ \t]+)?issues?\b\*{0,2})?"
    r"[ \t]*"
)
PT_RE = re.compile(
    r"(?i)(?<![\w/])\*{0,2}" + PT_KEYWORDS + r"\b\*{0,2}" + PT_FILLER + r"#(\d+)"
)

FENCE_RE = re.compile(r"(?ms)^[ \t]*(?:```|~~~).*?^[ \t]*(?:```|~~~)[ \t]*$")
SPAN_RE = re.compile(r"`[^`\n]*`")


def blank_code(text):
    """Substitui cerca/span de codigo por espacos, PRESERVANDO quebras de linha
    (numero de linha reportado continua batendo com o corpo original)."""
    def keep_newlines(m):
        return re.sub(r"[^\n]", " ", m.group(0))
    return SPAN_RE.sub(keep_newlines, FENCE_RE.sub(keep_newlines, text))


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

    scan = blank_code(body)
    original_lines = body.splitlines()

    english = {int(n) for n in EN_RE.findall(scan)}

    offenders = []
    for idx, line in enumerate(scan.splitlines(), 1):
        for m in PT_RE.finditer(line):
            num = int(m.group(1))
            if num not in english:
                shown = original_lines[idx - 1] if idx <= len(original_lines) else line
                offenders.append((idx, m.group(0).strip(), num, shown.strip()))

    if not offenders:
        print("OK   [pr-closing-keyword]: nenhuma palavra-chave de fechamento em")
        print("     portugues sem a forma inglesa correspondente.")
        if english:
            print("     Issues que o GitHub vai fechar no merge: %s"
                  % ", ".join("#%d" % n for n in sorted(english)))
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
  echo "OK   [pr-closing-keyword]: autoteste completo (deteccao + prosa + isencao por numero + vacuidade)."
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
