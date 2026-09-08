#!/usr/bin/env bash
# check-parity-call-site-pins.sh — reprova se um pin de call site do Makefile
# (ML-2E, ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify-sem-
# perder-cobertura.md, ML-2F) deixar de existir, ou se o rastro em stderr dos
# overrides TRACKFW_FALSIFY_* (mesmo ML) deixar de ser emitido pelo script que
# os consome.
#
# Contexto (parecer hades-tf, seção "Parecer de segurança" do roadmap): o
# problema nunca foi a env var existir — é o CALL SITE não pinar o valor.
# HASH_CMD_BIN e PYTHON_BIN são seguros porque o Makefile os sobrescreve em
# toda invocação (mesmo desenho de GO_BIN); TRACKFW_FALSIFY_SCRIPT/_GEN/_JOBS
# não são pinados por desenho (permitem sabotagem controlada do harness), mas
# precisam deixar rastro em stderr sempre que setados — senão um valor
# esquecido num .envrc silencia o gate mais caro do CI sem deixar sinal. O
# ML-2E entregou os três controles como medição manual, não versionada; este
# gate é o que falta para que a remoção de qualquer um dos dois seja
# acusada, não descoberta por auditoria.
#
# --- Por que a lista de variáveis é CONGELADA, e não derivada -------------
# O gerador de rótulos do ML-2D derivava o conjunto esperado do MESMO script
# que substituía — o hades-tf achou isso auto-referencial: sabotar o alvo
# também apaga o requisito, e o gate aprova por não achar nada (vacuidade
# disfarçada). O mesmo vale aqui: se a lista de variáveis fosse "toda env var
# lida via ${VAR:-...} em algum script chamado pelo Makefile", ela incluiria
# GO_BIN em check-validate-parity.sh (linha ~139), que NÃO é pinado no
# Makefile por desenho — ele cai para um binário próprio em tmp quando
# ausente, um site legítimo e já auditado, fora do escopo deste ML. Uma
# derivação ingênua produziria um FAIL de dia zero, ou pior, um site
# adicionado por engano nunca seria filtrado.
#
# Por isso as variáveis são a lista fechada abaixo (VARS_PIN / VARS_TRACE) —
# exatamente as que o ML-2E criou/documentou, com o mesmo comentário "ML-2E,
# mesma família de HASH_CMD_BIN" que este arquivo também usa. Manutenção:
# quando um novo controle desta família for criado, ele deve repetir esse
# comentário (convenção já em uso em Makefile:82-84 e
# scripts/check-roadmap-barrier-contract.sh:444) e seu nome deve entrar em
# VARS_PIN/VARS_TRACE abaixo, no mesmo PR que o introduz — é uma linha, não
# um mecanismo à parte a manter.
#
# O que É derivado, não hardcoded: para cada variável da lista fechada, o
# SCRIPT que a consome (via grep no corpo de scripts/*.sh) e a LINHA de
# recipe do Makefile que o invoca são descobertos em tempo de execução —
# nenhum caminho de arquivo nem número de linha está hardcoded abaixo. Isso
# significa que mover a chamada para outro alvo do Makefile, ou renomear o
# script consumidor, não quebra o gate por si só — só a ausência real do pin
# ou do rastro quebra.
set -euo pipefail

ROOT="${1:-.}"
MAKEFILE="$ROOT/Makefile"
SCRIPTS_DIR="$ROOT/scripts"
SELF="$(basename "${BASH_SOURCE[0]}")"

# Variáveis que exigem PIN no call site do Makefile (VAR=valor na mesma linha
# de recipe que invoca o script consumidor).
VARS_PIN=(HASH_CMD_BIN PYTHON_BIN)

# Variáveis que exigem RASTRO em stderr no script consumidor quando setadas,
# mas que são intencionalmente NÃO pinadas no Makefile (ML-2D/ML-2E: servem
# para sabotagem controlada do harness de falsificação).
VARS_TRACE=(TRACKFW_FALSIFY_SCRIPT TRACKFW_FALSIFY_GEN TRACKFW_FALSIFY_JOBS)

FAIL=0
CHECKED=0
ok()   { echo "OK   [$1]"; CHECKED=$((CHECKED + 1)); }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; CHECKED=$((CHECKED + 1)); }

# --- Guarda de vacuidade: Makefile ausente ou vazio -------------------------
if [[ ! -s "$MAKEFILE" ]]; then
  echo "check-parity-call-site-pins: Makefile ausente ou vazio em $MAKEFILE" >&2
  exit 1
fi

TAB=$(printf '\t')

# recipe_lines — só linhas de RECEITA do Makefile (iniciadas por TAB),
# excluindo linhas de comentário puro. Necessário porque Makefile:82-84 tem
# um comentário tab-indentado que MENCIONA "PYTHON_BIN pinado" dentro do
# corpo do alvo package-smoke — sem este filtro, o comentário sozinho
# convenceria o gate de que o pin real (linha 85) ainda existe mesmo que
# tenha sido apagado. Mesma classe de falso-positivo por prosa que custou
# uma reentrega ao ML-2D (comentário "# Cenario 166 -- ..." casando como
# fronteira de cenário real).
recipe_lines() {
  grep -E "^${TAB}" "$MAKEFILE" | grep -vE "^${TAB}[[:space:]]*#"
}

RECIPE_LINES_FILE=$(mktemp "${TMPDIR:-/tmp}/trackfw-parity-pins-recipe.XXXXXX")
trap 'rm -f "$RECIPE_LINES_FILE"' EXIT
recipe_lines >"$RECIPE_LINES_FILE" || true

if [[ ! -s "$RECIPE_LINES_FILE" ]]; then
  echo "check-parity-call-site-pins: Makefile não tem nenhuma linha de recipe (alvo ausente ou vazio)" >&2
  exit 1
fi

# find_consuming_scripts VAR — lista, em scripts/*.sh (exceto este gate),
# arquivos que leem VAR via ${VAR:-...} ou ${VAR}. Fonte de verdade
# independente do Makefile: o script declara, no próprio corpo, que aceita
# override desta variável.
find_consuming_scripts() {
  local var=$1
  grep -lE "\\\$\\{${var}(:-|[}:])" "$SCRIPTS_DIR"/*.sh 2>/dev/null \
    | grep -vF "/$SELF" || true
}

for var in "${VARS_PIN[@]}"; do
  consumers=$(find_consuming_scripts "$var")
  if [[ -z "$consumers" ]]; then
    fail "call-site-pin/$var/consumer-found" \
      "nenhum script em scripts/*.sh lê \${$var:-...} ou \${$var} -- o consumidor desapareceu, ou foi renomeado sem atualizar o gate"
    continue
  fi
  while IFS= read -r consumer; do
    [[ -z "$consumer" ]] && continue
    base=$(basename "$consumer")
    invocations=$(grep -F "scripts/${base}" "$RECIPE_LINES_FILE" || true)
    if [[ -z "$invocations" ]]; then
      fail "call-site-pin/$var/$base/invoked" \
        "scripts/${base} não é chamado por nenhuma linha de recipe do Makefile -- alvo removido ou script deixou de ser invocado"
      continue
    fi
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      if grep -qE "(^${TAB}|[[:space:]])${var}=" <<<"$line"; then
        ok "call-site-pin/$var/$base"
      else
        fail "call-site-pin/$var/$base" \
          "linha de recipe invoca ${base} sem pinar ${var}= -- pin removido: $line"
      fi
    done <<<"$invocations"
  done <<<"$consumers"
done

# --- TRACKFW_FALSIFY_* — rastro no script consumidor, não pin no Makefile --
for var in "${VARS_TRACE[@]}"; do
  consumers=$(find_consuming_scripts "$var")
  if [[ -z "$consumers" ]]; then
    fail "call-site-trace/$var/consumer-found" \
      "nenhum script em scripts/*.sh lê \${$var:-...} -- o consumidor desapareceu, ou foi renomeado sem atualizar o gate"
    continue
  fi
  while IFS= read -r consumer; do
    [[ -z "$consumer" ]] && continue
    base=$(basename "$consumer")
    guard_line=$(grep -nE -- "-n \"\\\$\\{${var}:-\\}\"" "$consumer" | head -1 | cut -d: -f1)
    if [[ -z "$guard_line" ]]; then
      fail "call-site-trace/$var/$base/guard" \
        "guarda 'if [[ -n \"\${${var}:-}\" ]]' não encontrada em ${base} -- rastro pode ter sido removido junto com a guarda"
      continue
    fi
    # Janela de 6 linhas: medida no arquivo real -- o echo de TRACKFW_FALSIFY_JOBS
    # fica na guarda+5 (4 linhas de cálculo de DEFAULT_JOBS entre a guarda e o
    # echo), as outras duas em guarda+2. Número fixo, não "resto do arquivo": uma
    # busca sem janela reintroduziria a mesma armadilha da Sabotagem 4 (um
    # comentário distante que MENCIONA a variável seria aceito como rastro). Se
    # alguém inserir mais de 2 linhas de lógica entre a guarda e o echo desta
    # família, o gate falha fechado (nomeando a variável) até a janela ser
    # ajustada aqui -- efeito colateral aceito, é a direção segura do erro.
    window_end=$((guard_line + 6))
    window=$(sed -n "${guard_line},${window_end}p" "$consumer")
    if grep -qE "echo .*${var}.*>&2" <<<"$window"; then
      ok "call-site-trace/$var/$base"
    else
      fail "call-site-trace/$var/$base" \
        "guarda de ${var} em ${base}:${guard_line} não é seguida por um echo para stderr mencionando ${var} nas 6 linhas seguintes -- rastro pode ter sido removido"
    fi
  done <<<"$consumers"
done

echo "check-parity-call-site-pins: ${CHECKED} verificação(ões) -- ${#VARS_PIN[@]} pin(s) + ${#VARS_TRACE[@]} rastro(s) na lista"

if [[ "$CHECKED" -eq 0 ]]; then
  echo "check-parity-call-site-pins: nenhuma verificação executada -- guarda de vacuidade final" >&2
  exit 1
fi

exit "$FAIL"
