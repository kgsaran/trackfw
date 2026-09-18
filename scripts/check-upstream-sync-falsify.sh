#!/usr/bin/env bash
# check-upstream-sync-falsify.sh — falsifica scripts/upstream-sync.sh contra merges
# HISTÓRICOS reais, nos dois extremos da distribuição produto/governança.
#
# AC4 da REQ-2026-09-05-procedimento-de-merge-do-upstream-com-retencao-da-governanca-local.
#
# Por que contra merges históricos, e não fixtures sintéticas
# ----------------------------------------------------------
# O modo de falha que este script existe para pegar é a detecção de renomeação de
# DIRETÓRIO do git casando os roadmaps flat do upstream com os nossos em by_agent. Uma
# fixture sintética não reproduz isso — depende do conteúdo real dos dois acervos e do
# heurístico de similaridade do git. Os dois casos abaixo são merges que de fato
# aconteceram, com o resultado conferido à mão na época.
#
# A propriedade verificada NÃO é "quantos arquivos", e sim a INVARIANTE:
#
#   retido      ⊆  docs/ ∪ vault/         (nunca suprime produto)
#   trazido     =  tudo que não é docs/ nem vault/   (nunca deixa produto para trás)
#
# Contar arquivos foi a primeira formulação do AC4 e estava ERRADA nos dois casos: os
# números vieram de contagem à mão que só olhava docs/{adr,req,roadmaps} e ignorava
# vault/, docs/qualidade e docs/cli-parity.md. A medição corrigiu 32→37 e 0→10. A
# invariante sobrevive à correção; a contagem não sobreviveria.

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
SYNC="$ROOT/scripts/upstream-sync.sh"
# WT precisa ser CURTO: o snapshot do barrier estoura o limite de nome no
# Windows (MAX_PATH). `/c/tfwfalsify` resolve isso la -- e NAO EXISTE em Linux.
#
# 🔴 Medido em 2026-09-10, na primeira corrida deste gate em CI: o caminho
# cravado fazia `git worktree add` falhar em ubuntu-latest, e o gate reportava
# "FAIL: worktree em bfeea12" -- mensagem que aponta para o commit, quando a
# causa era o DESTINO. O `2>/dev/null` da linha do worktree escondia o
# `fatal:` que teria dito isso na hora.
#
# A raiz curta so se aplica onde ela existe. Em POSIX, TMPDIR ja e curto.
if [ -d /c ]; then
	WT="${WT_FALSIFY:-/c/tfwfalsify}"
else
	WT="${WT_FALSIFY:-${TMPDIR:-/tmp}/tfwfalsify}"
fi
FAILED=0

# base<TAB>ref<TAB>rotulo
# O terceiro caso é o merge REAL que motivou reter o trackfw.yaml (2026-09-18): o #393 do
# upstream mudou o trackfw.yaml dele, as linhas colidiram com as nossas, e o sync antigo
# abortava. É o único dos três que toca o arquivo.
CASES=$'bfeea12\t4f0ad33\tgovernanca pesada (41 arquivos, 4 de produto)\n01086b5\t6b3ba49\tproduto puro (52 arquivos, 42 de produto)\n65cb024b\t9651f905\ttrackfw.yaml em conflito (#393 do upstream)'

cleanup() { git worktree remove --force "$WT" >/dev/null 2>&1 || true; git worktree prune >/dev/null 2>&1 || true; }
trap cleanup EXIT

while IFS=$'\t' read -r BASE REF LABEL; do
	[ -n "${BASE:-}" ] || continue
	echo "── caso: $LABEL"
	cleanup
	git worktree add --detach -q "$WT" "$BASE" 2>/dev/null || { echo "  FAIL: worktree em $BASE"; FAILED=1; continue; }

	# 🔴 A saida do sync e CAPTURADA, nao descartada. A versao anterior fazia
	# `>/dev/null 2>&1` e imprimia so "upstream-sync abortou" -- que nao diz por
	# que. Medido em 2026-09-10: em ubuntu-latest o merge falhava e a mensagem
	# do git era a unica coisa capaz de dizer a causa. Quinto caso da mesma
	# familia nesta semana.
	SYNC_OUT="$( cd "$WT" && bash "$SYNC" --ref "$REF" --skip-verify 2>&1 )" || {
		echo "  FAIL: upstream-sync abortou. Saida:"
		printf '%s
' "$SYNC_OUT" | sed 's/^/      /'
		FAILED=1; continue; }

	BROUGHT="$(cd "$WT" && git diff --cached --name-only "$BASE" | sort)"
	ALL="$(cd "$WT" && git diff --name-only "$BASE...$REF" | sort)"
	RETAINED="$(comm -23 <(printf '%s\n' "$ALL") <(printf '%s\n' "$BROUGHT"))"

	# Produto em docs/ (a mesma lista do sync, lida dele: duas listas divergiriam).
	PRODUTO_EM_DOCS="$(sed -n 's/^PRODUTO_EM_DOCS="\(.*\)"$/\1/p' "$SYNC")"
	[ -n "$PRODUTO_EM_DOCS" ] || { echo "  FAIL: nao li PRODUTO_EM_DOCS do upstream-sync.sh"; FAILED=1; continue; }
	GOV_FORA="$(sed -n 's/^GOVERNANCA_FORA_DE_DOCS="\(.*\)"$/\1/p' "$SYNC")"
	[ -n "$GOV_FORA" ] || { echo "  FAIL: nao li GOVERNANCA_FORA_DE_DOCS do upstream-sync.sh"; FAILED=1; continue; }
	GOV_RE="$(printf '%s\n' $GOV_FORA | sed 's/[.]/\\./g' | paste -sd'|' -)"

	# INVARIANTE 1 — nada retido fora de docs/ ou vault/ (nao suprime produto)
	LEAK="$(printf '%s\n' "$RETAINED" | grep -vE "^(docs/|vault/|($GOV_RE)\$)" | grep -v '^$' || true)"
	if [ -n "$LEAK" ]; then
		echo "  FAIL: PRODUTO suprimido:"; printf '%s\n' "$LEAK" | sed 's/^/      /'; FAILED=1
	else
		echo "  ok  retido ⊆ docs/ ∪ vault/ ∪ {$GOV_FORA}  ($(printf '%s\n' "$RETAINED" | grep -c . || true) arquivos)"
	fi

	# INVARIANTE 2 — nada de produto ficou para tras
	ESPERADO_TRAZIDO="$( { printf '%s\n' "$ALL" | grep -vE "^(docs/|vault/|($GOV_RE)\$)" || true; for p in $PRODUTO_EM_DOCS; do printf '%s\n' "$ALL" | grep -xF "$p" || true; done; } | sort -u)"
	MISSED="$(comm -23 <(printf '%s\n' "$ESPERADO_TRAZIDO") <(printf '%s\n' "$BROUGHT"))"
	if [ -n "$(printf '%s\n' "$MISSED" | grep -c . || true)" ] && [ -n "$MISSED" ]; then
		echo "  FAIL: produto NAO trazido:"; printf '%s\n' "$MISSED" | sed 's/^/      /'; FAILED=1
	else
		echo "  ok  todo produto trazido            ($(printf '%s\n' "$BROUGHT" | grep -c . || true) arquivos)"
	fi

	# INVARIANTE 3 — docs/ e vault/ identicos a base, fora o produto em docs/
	EXCL=(); for p in $PRODUTO_EM_DOCS; do EXCL+=(":(exclude)$p"); done
	DIFF="$(cd "$WT" && git diff --cached "$BASE" --stat -- docs vault $GOV_FORA "${EXCL[@]}")"
	if [ -n "$DIFF" ]; then
		echo "  FAIL: docs/, vault/ ou $GOV_FORA divergem da base"; FAILED=1
	else
		echo "  ok  docs/, vault/ e $GOV_FORA identicos a base (fora $PRODUTO_EM_DOCS)"
	fi

	# INVARIANTE 4 — produto em docs/ identico ao REF
	DIFF4="$(cd "$WT" && git diff --cached "$REF" --stat -- $PRODUTO_EM_DOCS)"
	TOCOU="$(printf '%s\n' "$ALL" | grep -cxF -e "$(printf '%s\n' $PRODUTO_EM_DOCS)" || true)"
	if [ -n "$DIFF4" ]; then
		echo "  FAIL: produto em docs/ difere do REF"; printf '%s\n' "$DIFF4" | sed 's/^/      /'; FAILED=1
	else
		echo "  ok  produto em docs/ identico ao REF (o merge tocou ${TOCOU} dele)"
	fi
done <<< "$CASES"

# ── Controle negativo: o script tem de RECUSAR arvore suja ──────────────────────
echo "── controle: arvore suja tem de ser recusada"
cleanup
git worktree add --detach -q "$WT" bfeea12 2>/dev/null
echo "sujeira" > "$WT/ARQUIVO-NAO-RASTREADO.txt"
if ( cd "$WT" && bash "$SYNC" --ref 4f0ad33 --skip-verify ) >/dev/null 2>&1; then
	echo "  FAIL: aceitou arvore suja"; FAILED=1
else
	echo "  ok  recusou arvore suja"
fi

# ── Controle negativo: ref inexistente ──────────────────────────────────────────
echo "── controle: ref inexistente tem de ser recusada"
cleanup
git worktree add --detach -q "$WT" bfeea12 2>/dev/null
if ( cd "$WT" && bash "$SYNC" --ref nao-existe-esta-ref --skip-verify ) >/dev/null 2>&1; then
	echo "  FAIL: aceitou ref inexistente"; FAILED=1
else
	echo "  ok  recusou ref inexistente"
fi

echo ""
if [ "$FAILED" = "0" ]; then echo "check-upstream-sync-falsify: OK"; exit 0; fi
echo "check-upstream-sync-falsify: FAIL"; exit 1
