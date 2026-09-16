#!/usr/bin/env bash
# upstream-sync.sh — merge do upstream com RETENÇÃO da governança local.
#
# REQ-2026-09-05-procedimento-de-merge-do-upstream-com-retencao-da-governanca-local
# Governado por ADR-2026-08-29-adotar-upstream-como-base: produto vem do upstream;
# docs/ e vault/ são locais e NUNCA são importados — com UMA exceção nomeada,
# `docs/cli-parity.md`, que é contrato de produto lido por gate do produto (ver
# PRODUTO_EM_DOCS abaixo).
#
# Por que existe
# -------------
# Com `roadmap_namespacing: by_agent`, o git detecta os roadmaps flat do upstream
# (docs/roadmaps/wip/) como RENOMEAÇÃO dos nossos (docs/roadmaps/claude/done/) e produz
# uma enxurrada de conflitos rename/delete — 23 no merge de 4f0ad33. Resolver um a um é
# caro e erra fácil, e a ADR já decidiu o resultado: não há julgamento a fazer.
#
# O que este script NÃO faz, de propósito
# ---------------------------------------
#   - não faz push
#   - não commita por padrão (o commit carrega a medição, e quem mede é quem escreve)
#   - não resolve conflito de PRODUTO: se sobrar algum, ABORTA e devolve a árvore
#
# Falsificação (AC4): scripts/check-upstream-sync-falsify.sh exercita este script contra
# merges HISTÓRICOS reais, nos dois extremos, mais dois controles negativos (árvore suja e
# ref inexistente). A propriedade verificada é a INVARIANTE — retido ⊆ docs/ ∪ vault/, e
# todo o resto trazido —, não a contagem de arquivos: a contagem à mão errou nos DOIS casos.

# ── Produto que mora em docs/ — TRAZIDO, não retido ─────────────────────────────
# 🔴 Lista literal e curta DE PROPÓSITO. Cada entrada precisa de um gate do upstream que a
# leia; "é documentação de produto" sozinho não basta, senão a lista vira depósito.
#
# docs/cli-parity.md — lido por scripts/check-parity-contract-coverage.sh, que reprova
# seção cuja anotação nomeia gate inexistente. Retido desde o #259, o nosso ficou parado;
# a v8.0.0 (#365) removeu os gates que ele citava, e o gate de cobertura passou a reprovar
# o parity-other-gates em Makefile:35 — antes do barrier —, e o item 4 do
# windows-defect-reproduction travou até o timeout. Medido em 2026-09-16, PR #135.
# Decisão do usuário: trazer e manter trazendo.
# Ver REQ-2026-09-16-gates-so-nossos-depois-da-v8, ML-3A.
PRODUTO_EM_DOCS="docs/cli-parity.md"

set -euo pipefail

REF="upstream/main"
DO_COMMIT=0
SKIP_VERIFY=0

usage() {
	cat <<'USAGE'
uso: scripts/upstream-sync.sh [--ref <git-ref>] [--commit] [--skip-verify]

  --ref <git-ref>   o que mesclar. default: upstream/main
  --commit          além de preparar, commita (mensagem gerada; revise antes do PR)
  --skip-verify     pula a verificação pós-merge (build + paridade de validate)

Prepara o merge com retenção de docs/ e vault/, PROVA a retenção por efeito,
reporta a proporção produto/governança, e verifica que o validate não mexeu.
Aborta se sobrar conflito ou se a retenção não puder ser provada.
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
		--ref) REF="${2:?--ref exige um valor}"; shift 2 ;;
		--commit) DO_COMMIT=1; shift ;;
		--skip-verify) SKIP_VERIFY=1; shift ;;
		-h|--help) usage; exit 0 ;;
		*) echo "upstream-sync: opção desconhecida: $1" >&2; usage >&2; exit 2 ;;
	esac
done

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

die() { echo "upstream-sync: $*" >&2; exit 1; }
say() { printf '%s\n' "$*"; }

# ── Pré-condições ────────────────────────────────────────────────────────────────
[ -z "$(git status --porcelain)" ] || die "árvore suja. Commite ou guarde antes de sincronizar."
git rev-parse --verify --quiet "$REF" >/dev/null || die "ref não existe: $REF (fez 'git fetch upstream'?)"

BASE="$(git rev-parse HEAD)"
BASE_SHORT="$(git rev-parse --short HEAD)"
REF_SHORT="$(git rev-parse --short "$REF")"

if [ "$(git rev-list --count "HEAD..$REF")" = "0" ]; then
	say "upstream-sync: nada a trazer — HEAD já contém $REF ($REF_SHORT)."
	exit 0
fi

# ── Baseline do validate, ANTES de mexer na árvore ───────────────────────────────
# Medido com o binário DA ÁRVORE, nunca o do PATH — o do PATH é outra instalação.
BIN="./bin/trackfw"
VAL_BEFORE="n/a"
if [ "$SKIP_VERIFY" = "0" ]; then
	go build -o bin/trackfw ./cmd/trackfw 2>/dev/null || die "não consegui construir o binário da árvore para medir o baseline"
	# No Windows o `-o bin/trackfw` grava exatamente esse nome, e o bin/trackfw.exe -- o que o
	# PowerShell executa -- ficava uma versão atrás a cada sync. A extensão vem do toolchain, não de
	# predicado de SO: GOEXE é ".exe" no Windows e vazio no resto, onde nada muda.
	GOEXE_EXT="$(go env GOEXE 2>/dev/null)"
	[ -z "$GOEXE_EXT" ] || go build -o "bin/trackfw$GOEXE_EXT" ./cmd/trackfw 2>/dev/null || die "não consegui construir bin/trackfw$GOEXE_EXT para medir o baseline"
	VAL_BEFORE="$("$BIN" validate 2>&1 | grep -c '^✗' || true)"
fi

# ── Merge ────────────────────────────────────────────────────────────────────────
say "upstream-sync: mesclando $REF ($REF_SHORT) sobre $BASE_SHORT…"
# 🔴 O merge PODE falhar por motivo que nao e conflito -- historias nao
# relacionadas, identidade ausente, ref irresolvivel, worktree em estado ruim. A
# versao anterior descartava o stderr E ignorava o codigo de saida, e o efeito
# era o pior possivel: o script SEGUIA, `git diff --cached` saia VAZIO, e ele
# reportava "0 arquivos de produto trazidos" como se fosse resultado.
#
# Medido em 2026-09-10: em ubuntu-latest o gate de falsificacao acusou 4 arquivos
# de produto "suprimidos" -- e a supressao era o merge nao ter acontecido. No
# Windows o mesmo caso passa. Silencio no stderr transformou falha de merge em
# medicao plausivel.
#
# Conflito continua sendo esperado e tratado adiante (--diff-filter=U). O que
# esta guarda pega e falha SEM conflito: nao ha o que resolver, e prosseguir
# produziria numero mentiroso.
set +e
MERGE_OUT="$(git merge --no-commit --no-ff "$REF" 2>&1)"
MERGE_RC=$?
set -e

if [ "$MERGE_RC" -ne 0 ] && [ -z "$(git diff --name-only --diff-filter=U)" ]; then
	printf '%s
' "$MERGE_OUT" >&2
	git merge --abort 2>/dev/null || git reset --hard "$BASE" >/dev/null 2>&1
	die "git merge falhou (exit $MERGE_RC) e NAO ha conflito para resolver. Mensagem acima. Arvore devolvida."
fi

# ── Retenção: docs/ e vault/ voltam a ser EXATAMENTE os nossos ───────────────────
# Não se resolve conflito a conflito. A ADR já decidiu o resultado.
git rm -r -q --ignore-unmatch --force docs vault >/dev/null 2>&1 || true
git checkout "$BASE" -- docs >/dev/null 2>&1 || true
# vault/ não existe na nossa árvore: fica deletado, que é o estado correto.

# Produto em docs/: volta a ser o do REF. Se o REF não tem o arquivo, ele sai.
EXCLUI_PRODUTO=()
for p in $PRODUTO_EM_DOCS; do
	EXCLUI_PRODUTO+=(":(exclude)$p")
	if git cat-file -e "$REF:$p" 2>/dev/null; then
		git checkout "$REF" -- "$p" >/dev/null 2>&1 || die "não consegui trazer $p de $REF."
	else
		git rm -q --ignore-unmatch --cached -- "$p" >/dev/null 2>&1 || true
		rm -f -- "$p"
	fi
done

# ── AC2: PROVAR a retenção por efeito, não afirmá-la ─────────────────────────────
RETENTION_DIFF="$(git diff --cached "$BASE" --stat -- docs vault "${EXCLUI_PRODUTO[@]}")"
if [ -n "$RETENTION_DIFF" ]; then
	echo "$RETENTION_DIFF" >&2
	git merge --abort 2>/dev/null || git reset --hard "$BASE" >/dev/null 2>&1
	die "RETENÇÃO NÃO PROVADA: docs/ ou vault/ diferem da base. Árvore devolvida."
fi
# ...e a exceção prova o lado oposto: o produto em docs/ ficou IGUAL ao do REF.
TRAZIDO_DIFF="$(git diff --cached "$REF" --stat -- $PRODUTO_EM_DOCS)"
if [ -n "$TRAZIDO_DIFF" ]; then
	echo "$TRAZIDO_DIFF" >&2
	git merge --abort 2>/dev/null || git reset --hard "$BASE" >/dev/null 2>&1
	die "PRODUTO EM docs/ NÃO TRAZIDO: difere de $REF. Árvore devolvida."
fi

# ── Conflito remanescente é de PRODUTO: aborta ───────────────────────────────────
LEFT="$(git diff --name-only --diff-filter=U)"
if [ -n "$LEFT" ]; then
	echo "$LEFT" >&2
	git merge --abort 2>/dev/null || git reset --hard "$BASE" >/dev/null 2>&1
	die "conflito de PRODUTO remanescente (acima). Resolva à mão. Árvore devolvida."
fi

# ── AC3: a proporção é o discriminante ───────────────────────────────────────────
PRODUCT_N="$(git diff --cached --name-only "$BASE" | wc -l | tr -d ' ')"
TOTAL_N="$(git diff --name-only "$BASE...$REF" | wc -l | tr -d ' ')"
GOV_N=$(( TOTAL_N - PRODUCT_N ))
[ "$GOV_N" -lt 0 ] && GOV_N=0

say ""
say "  arquivos de PRODUTO trazidos : $PRODUCT_N"
say "  de governança RETIDOS        : $GOV_N   (de $TOTAL_N no merge)"
say ""
git diff --cached --name-only "$BASE" | sed 's/^/    /'
say ""

# ── AC5: verificação pós-merge ───────────────────────────────────────────────────
if [ "$SKIP_VERIFY" = "0" ]; then
	say "upstream-sync: verificando…"
	go build ./... >/dev/null 2>&1 || die "go build ./... reprovou depois do merge."
	go build -o bin/trackfw ./cmd/trackfw >/dev/null 2>&1 || die "não consegui reconstruir o binário da árvore."
	[ -z "${GOEXE_EXT:-}" ] || go build -o "bin/trackfw$GOEXE_EXT" ./cmd/trackfw >/dev/null 2>&1 || die "não consegui reconstruir bin/trackfw$GOEXE_EXT."
	VAL_AFTER="$("$BIN" validate 2>&1 | grep -c '^✗' || true)"
	say "  go build ./...   exit 0"
	say "  validate         $VAL_BEFORE antes · $VAL_AFTER depois"
	if [ "$VAL_BEFORE" != "$VAL_AFTER" ]; then
		die "o validate MUDOU ($VAL_BEFORE -> $VAL_AFTER). O merge não deveria tocar governança — investigue antes de commitar."
	fi
fi

# ── Commit (opcional) ────────────────────────────────────────────────────────────
if [ "$DO_COMMIT" = "1" ]; then
	git commit --no-edit -q -F - <<EOF
merge: $REF ($REF_SHORT) — $PRODUCT_N arquivos de produto

Preparado por scripts/upstream-sync.sh, com retenção de docs/ e vault/ pela
ADR-2026-08-29: a governança do upstream não é importada.

  produto trazido      $PRODUCT_N
  governança retida    $GOV_N  (de $TOTAL_N no merge)
  validate             $VAL_BEFORE antes · ${VAL_AFTER:-n/a} depois

Retenção PROVADA por efeito: git diff --cached $BASE_SHORT -- docs vault saiu vazio,
exceto o produto em docs/ ($PRODUTO_EM_DOCS), provado igual ao de $REF_SHORT.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
	say "upstream-sync: commitado. Revise a mensagem antes de abrir o PR."
else
	say "upstream-sync: preparado e NÃO commitado."
	say "  revise com: git diff --cached $BASE_SHORT --stat"
fi
