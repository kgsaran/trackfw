#!/usr/bin/env bash
#
# Executa os gates que são SÓ NOSSOS, e reprova se algum reprovar.
#
# Por que existe: derivado em 2026-09-10, **10 scripts em `scripts/` são só
# nossos e NENHUM era invocado por automação nenhuma** — nem `Makefile`, nem CI.
# O `check-upstream-content.sh` ficou VERMELHO por semanas com 7 arquivos de
# governança do upstream em `docs/`, e só apareceu porque alguém o rodou à mão
# numa auditoria de outra REQ.
#
# 🔴 Um gate que ninguém executa é pior que gate nenhum, porque produz a sensação
# de cobertura.
#
# POR QUE NÃO ESTÁ NO `Makefile`: o `Makefile` e os 7 workflows são arquivos
# COMPARTILHADOS com o upstream — derivado por `git ls-tree`, não por
# `git cat-file` (ver nota de conversão do MSYS abaixo). Modificá-los criaria
# divergência de produto que todo merge futuro pagaria, e hoje ela é ZERO.
# Este arquivo e o `Makefile.local` são ADIÇÕES, e adição não é divergência.
#
# Ver REQ-2026-09-10-o-check-upstream-content-nao-roda-em-lugar-nenhum.
set -uo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

# ---------------------------------------------------------------------------
# EXECUTADOS — a lista é explícita, e a GUARDA DE COMPLETUDE abaixo garante que
# um script só-nosso novo não fique de fora em silêncio. Foi assim que o
# `check-upstream-content` sumiu de vista.
#
# 🔴 Os nomes vão COM `.sh`, de propósito (2026-09-11). O `check-orphan-gates.sh`
# do upstream (#323) procura o nome LITERAL do gate numa linha de execução; sem a
# extensão, ele acusava os nossos 9 gates como órfãos, embora rodem a cada push.
# O `FORA` abaixo continua SEM `.sh`: ali nada é executado, e com a extensão o
# gate do upstream contaria um script declarado fora como consumido — a busca
# ingênua por substring que ele mesmo documenta evitar.
# ---------------------------------------------------------------------------
EXECUTAR="
check-upstream-content.sh
check-req-layout.sh
check-referential-integrity.sh
check-inherited-req.sh
check-req-done-com-criterio-aberto.sh
check-os-predicate-classification.sh
check-slug-inventory.sh
check-upstream-sync-falsify.sh
measure-os-predicate-sites.sh
"

# ---------------------------------------------------------------------------
# DECLARADOS FORA — cada um COM MOTIVO. Exclusão sem motivo escrito é o mesmo
# que não ter regra.
# ---------------------------------------------------------------------------
FORA="
check-platform-predicates|so diz algo no Windows: 14 das 20 linhas do corpus divergem entre 'esperado' e 'nativo_windows', e a linha do execbit reprovaria em ubuntu-latest por fato de NTFS. Decisao medida, escrita no CLAUDE.md
upstream-sync|nao e gate, e ferramenta: faz merge e MODIFICA a arvore. Rodar num CI de verificacao seria absurdo
run-local-gates|e o proprio agregador: executa-lo a partir de si mesmo seria recursao infinita. A guarda de completude o pegou na PRIMEIRA execucao, e esta entrada e a prova de que ela funciona
"

# ---------------------------------------------------------------------------
# GUARDA DE COMPLETUDE — o coracao deste script.
#
# Deriva os scripts SO NOSSOS por `git ls-tree` e exige que cada um esteja em
# EXECUTAR ou em FORA. Um script novo que ninguem listar REPROVA aqui.
#
# 🔴 `git ls-tree`, nunca `git cat-file -e "$ref:$path"`. Medido em 2026-09-10:
# o MSYS converte `ref:.caminho` em `ref;\caminho` quando o caminho apos os
# dois-pontos COMECA COM PONTO, e a checagem devolve "so nosso" para arquivo que
# e do upstream. Deu falso em todos os 7 workflows de uma vez. Mesma familia do
# achado que levamos ao upstream na issue #308.
# ---------------------------------------------------------------------------
UPSTREAM_REF=${UPSTREAM_REF:-upstream/main}

if ! git rev-parse --verify --quiet "$UPSTREAM_REF" >/dev/null; then
  echo "run-local-gates: FALHA — ref '$UPSTREAM_REF' nao existe." >&2
  echo "  Sem ela nao da para derivar o que e so nosso, e a guarda de completude" >&2
  echo "  passaria sem verificar nada. Rode 'git fetch upstream'." >&2
  exit 1
fi

TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
git ls-tree -r --name-only "$UPSTREAM_REF" -- scripts > "$TMP/upstream.txt" 2>/dev/null || true

if [ ! -s "$TMP/upstream.txt" ]; then
  echo "run-local-gates: GUARDA — 'git ls-tree $UPSTREAM_REF -- scripts' devolveu VAZIO." >&2
  echo "  Isso faria TODO script parecer 'so nosso'. Nao e resultado, e a derivacao quebrada." >&2
  exit 1
fi

so_nossos=""
for f in scripts/*.sh; do
  grep -qx "$f" "$TMP/upstream.txt" || so_nossos="$so_nossos $(basename "$f" .sh)"
done

n_so_nossos=$(printf '%s' "$so_nossos" | wc -w | tr -d ' ')
if [ "$n_so_nossos" -eq 0 ]; then
  echo "run-local-gates: GUARDA — zero scripts so nossos derivados." >&2
  echo "  Este fork tem gates proprios; zero significa que a derivacao quebrou." >&2
  exit 1
fi

nao_listados=0
for s in $so_nossos; do
  if printf '%s' "$EXECUTAR" | grep -qx "${s}.sh"; then continue; fi
  if printf '%s' "$FORA" | grep -q "^${s}|"; then continue; fi
  echo "  ✗ COMPLETUDE: 'scripts/${s}.sh' e so nosso e nao esta nem em EXECUTAR nem em FORA" >&2
  nao_listados=$((nao_listados + 1))
done

if [ "$nao_listados" -gt 0 ]; then
  echo "" >&2
  echo "${nao_listados} script(s) so nosso(s) fora da lista." >&2
  echo "Acrescente a EXECUTAR, ou a FORA COM O MOTIVO ESCRITO." >&2
  echo "🔴 Ficar de fora em silencio e exatamente o defeito que este script existe para fechar." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# PRE-CONDICAO DO BINARIO — converte morte silenciosa em falha NOMEADA.
#
# Medido na primeira corrida em CI (2026-09-10): sem `npm ci`, o
# check-subcommand-parity saiu exit 1 com ZERO linhas de saida. Ele invocava
# `node npm/bin/trackfw --help 2>/dev/null`; sem os modulos o node falhava, o
# stderr era descartado, e `set -euo pipefail` matava o script sem dizer nada.
#
# Um gate que morre calado e indistinguivel de um gate que reprovou. Esta guarda
# faz a diferenca aparecer ANTES, com o nome do que falta.
#
# 🔴 Ate 2026-09-16 ela exigia tambem `node npm/bin/trackfw` e
# `python3 -m trackfw`. A v8.0.0 do upstream (#365) removeu as duas
# reimplementacoes, e com elas o unico gate nosso que as invocava
# (check-subcommand-parity, retirado). Exigir runtime que nenhum gate usa
# reprovaria o agregador inteiro por motivo nenhum -- foi o que o sync mediu.
# Ver REQ-2026-09-16-gates-so-nossos-depois-da-v8.
# ---------------------------------------------------------------------------
faltando=""
[ -x "$ROOT_DIR/bin/trackfw" ] || faltando="$faltando bin/trackfw"

if [ -n "$faltando" ]; then
  echo "run-local-gates: FALHA — pre-requisito(s) indisponivel(is):$faltando" >&2
  echo "  Gates que chamam o binario da arvore descartam o stderr dele. Sem ele," >&2
  echo "  eles morrem com exit 1 e nenhuma mensagem -- indistinguivel de reprovacao." >&2
  echo "  Rode 'go build -o bin/trackfw ./cmd/trackfw'." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Execucao
# ---------------------------------------------------------------------------
echo "run-local-gates: ${n_so_nossos} script(s) so nosso(s) derivado(s) de $(ls scripts/*.sh | wc -l | tr -d ' ') em scripts/"
echo ""

falhas=0
executados=0
for s in $EXECUTAR; do
  [ -f "scripts/${s}" ] || { echo "  ✗ ${s%.sh}: LISTADO em EXECUTAR mas o arquivo nao existe" >&2; falhas=$((falhas+1)); continue; }
  saida=$(bash "scripts/${s}" 2>&1); rc=$?
  executados=$((executados + 1))
  if [ "$rc" -eq 0 ]; then
    printf '  ok   %-38s\n' "${s%.sh}"
  else
    printf '  FAIL %-38s exit=%s\n' "${s%.sh}" "$rc"
    printf '%s\n' "$saida" | sed 's/^/         /' >&2
    falhas=$((falhas + 1))
  fi
done

# GUARDA DE VACUIDADE: zero executados com exit 0 seria verde por nao ter feito nada.
if [ "$executados" -eq 0 ]; then
  echo "run-local-gates: GUARDA — zero gates executados." >&2
  exit 1
fi

echo ""
echo "  declarados FORA, com motivo:"
printf '%s\n' "$FORA" | while IFS='|' read -r nome motivo; do
  [ -n "$nome" ] || continue
  printf '    %-28s %s\n' "$nome" "$motivo"
done

echo ""
if [ "$falhas" -gt 0 ]; then
  echo "run-local-gates: ${executados} executado(s) · ${falhas} REPROVOU(RAM)" >&2
  exit 1
fi
echo "run-local-gates: ${executados} executado(s) · 0 falha(s)"
