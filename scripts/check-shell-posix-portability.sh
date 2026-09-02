#!/usr/bin/env bash
# Gate: barrier.js e barrier.py continuam invocando `sh -c` explicito, resolvido
# via $PATH, para executar o bloco `Gates da wave:` — nunca voltam a
# `shell: true` / `shell=True` (o shell do SO, que no Windows e cmd.exe e nao
# entende os 83 idiomas POSIX medidos pelo ADR). REQ-2026-09-01, AC5; ver
# docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md
# e docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md.
#
# Por que este gate mira SUBSTRINGS de codigo especificos, nunca "shell: true"/
# "shell=True" soltos no arquivo inteiro: os proprios comentarios de
# barrier.js/barrier.py, escritos pelo ML-1A para documentar POR QUE a chamada
# antiga foi trocada, citam essas duas strings em prosa ("... NOT spawnSync's
# `shell: true`, which is pinned to a fixed /bin/sh ...", "... NOT
# subprocess.run(cmd, shell=True), which is pinned ..."). Um grep ingenuo por
# "shell: true"/"shell=True" no arquivo inteiro reprovaria a arvore CORRETA por
# causa do proprio comentario que explica a correcao — falso positivo medido
# nesta sessao (grep -n confirma as duas ocorrencias, ambas em linha `//`/`#`).
# A checagem negativa abaixo, por isso, exclui linhas de comentario ANTES de
# procurar o padrao de codigo — mesmo padrao ja usado em
# scripts/check-homedir-parity.sh (`grep -v '//'`) e
# scripts/check-tty-detection.sh (`grep -vE ':\s*#'`) deste repositorio.
#
# Por que assert_count(1), nao assert_has, nas assinaturas centrais: a licao
# registrada em scripts/check-ref-separator-portability.sh (achado real:
# `expandedRef := config.ExpandPath(normalizeRefSeparator(ref))` aparece em
# DOIS pontos distintos de validator.go, e um assert_has comum passaria com
# so UM dos dois corrigido) se aplica aqui por analogia, mesmo apos medir que
# neste caso especifico cada assinatura de chamada `sh -c` ocorre exatamente 1
# vez por arquivo hoje (confirmado com grep -c e python3 str.count antes de
# escrever este gate) — assert_count(1) trava tanto a remocao quanto uma
# duplicacao futura que diluiria a cobertura sem que a assinatura suma. Para o
# terceiro estado `not_evaluated`, que aparece DUAS vezes por design (uma vez
# no ramo de trust do roadmap, outra no ramo de spawn de `sh` ausente),
# assert_count(2) e o numero certo — um assert_has bastaria para "existe pelo
# menos um", mas nao provaria que os DOIS ramos ainda reportam o terceiro
# estado depois de uma edicao futura que colapsasse um deles em `blocked`.
#
# Escopo: SO os dois arquivos e o ponto de execucao de gate da wave
# (evalGates/_check_gates). Nao mira serve.js/serve.py — que tem `shell: true`/
# `shell=True` legitimo e REQ propria por injecao de comando (ML-0A, achado 4.2)
# — nem qualquer outro uso de shell fora deste ponto.
set -euo pipefail

ROOT="${1:-.}"

fail=0
checked=0

# assert_has <label> <file> <exact-string>
# Reprova se o arquivo nao existir OU se a string exata nao aparecer nele.
assert_has() {
  local label=$1 file=$2 needle=$3
  checked=$((checked + 1))
  if [[ ! -f "$ROOT/$file" ]]; then
    echo "check-shell-posix-portability: $label — arquivo ausente: $file" >&2
    fail=1
    return
  fi
  if ! grep -qF -- "$needle" "$ROOT/$file"; then
    echo "check-shell-posix-portability: $label — assinatura ausente em $file" >&2
    echo "  esperado: $needle" >&2
    fail=1
  fi
}

# assert_count <label> <file> <exact-string> <expected-occurrences>
# Como assert_has, mas exige um numero exato de ocorrencias — ver nota de
# cabecalho sobre por que assert_count e preferido aqui.
assert_count() {
  local label=$1 file=$2 needle=$3 expected_n=$4 got
  checked=$((checked + 1))
  if [[ ! -f "$ROOT/$file" ]]; then
    echo "check-shell-posix-portability: $label — arquivo ausente: $file" >&2
    fail=1
    return
  fi
  got=$(grep -cF -- "$needle" "$ROOT/$file" || true)
  if [[ "$got" -ne "$expected_n" ]]; then
    echo "check-shell-posix-portability: $label — esperava $expected_n ocorrencia(s), achou $got em $file" >&2
    echo "  esperado: $needle" >&2
    fail=1
  fi
}

# assert_no_code_match <label> <file> <comment-prefix-regex> <bad-pattern-regex>
# Reprova se o padrao de codigo aparecer FORA de linha de comentario. Exclui
# linhas de comentario antes de procurar — ver nota de cabecalho.
assert_no_code_match() {
  local label=$1 file=$2 comment_re=$3 bad_re=$4 got
  checked=$((checked + 1))
  if [[ ! -f "$ROOT/$file" ]]; then
    echo "check-shell-posix-portability: $label — arquivo ausente: $file" >&2
    fail=1
    return
  fi
  got=$(grep -vE -- "$comment_re" "$ROOT/$file" | grep -cE -- "$bad_re" || true)
  if [[ "$got" -ne 0 ]]; then
    echo "check-shell-posix-portability: $label — regrediu para shell do SO ($got ocorrencia(s) fora de comentario)" >&2
    echo "  arquivo: $file" >&2
    fail=1
  fi
}

# --- AC5 — barrier.js executa gate com sh -c explicito, nunca shell: true ----
assert_count "Node: chamada de gate via sh -c explicito, resolvido por \$PATH" \
  "npm/src/commands/barrier.js" \
  "spawnSync('sh', ['-c', command], {" \
  1
assert_count "Node: mensagem pinada de sh ausente esta definida" \
  "npm/src/commands/barrier.js" \
  "const SH_MISSING_MSG =" \
  1
assert_count "Node: spawn falho (sh ausente) reporta not_evaluated com a mensagem pinada" \
  "npm/src/commands/barrier.js" \
  "failures: [SH_MISSING_MSG]," \
  1
assert_count "Node: gates check reporta not_evaluated nos dois ramos (trust E sh ausente)" \
  "npm/src/commands/barrier.js" \
  "status: 'not_evaluated'," \
  2
assert_no_code_match "Node: nenhuma chamada de gate volta a usar shell do SO" \
  "npm/src/commands/barrier.js" \
  '^\s*//' \
  'shell\s*:\s*true'

# --- AC5 — barrier.py executa gate com sh -c explicito, nunca shell=True -----
assert_count "Python: chamada de gate via sh -c explicito, resolvido por \$PATH" \
  "pypi/trackfw/commands/barrier.py" \
  '["sh", "-c", cmd],' \
  1
assert_count "Python: mensagem pinada de sh ausente esta definida" \
  "pypi/trackfw/commands/barrier.py" \
  '_SH_MISSING_MSG = (' \
  1
assert_count "Python: spawn falho (sh ausente) reporta not_evaluated com a mensagem pinada" \
  "pypi/trackfw/commands/barrier.py" \
  '"failures": [_SH_MISSING_MSG],' \
  1
assert_count "Python: gates check reporta not_evaluated nos dois ramos (trust E sh ausente)" \
  "pypi/trackfw/commands/barrier.py" \
  '"status": "not_evaluated",' \
  2
assert_no_code_match "Python: nenhuma chamada de gate volta a usar shell do SO" \
  "pypi/trackfw/commands/barrier.py" \
  '^\s*#' \
  'shell\s*=\s*True'

# --- Guarda de vacuidade -----------------------------------------------------
# Duas formas cobertas, como em check-ref-separator-portability.sh:
# 1. Contagem de asserts abaixo — pega alguem removendo uma chamada assert_*
#    do corpo deste script sem que nenhum grep individual reprove.
# 2. `[[ ! -f "$ROOT/$file" ]]` dentro de cada assert_* — pega arquivo/
#    diretorio movido, renomeado ou ausente, apontando qual arquivo falta em
#    cada linha, nunca um "0 encontrados, gate passa" silencioso.
#
# 10 e o numero de chamadas assert_* acima (4 assert_count + 1 assert_no_code_match
# por arquivo, x2 arquivos) — nomeado, nao magico.
expected=10
if [[ "$checked" -ne "$expected" ]]; then
  echo "check-shell-posix-portability: vacuidade — esperava checar $expected assinaturas, checou $checked" >&2
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "check-shell-posix-portability: FALHOU — barrier.js/barrier.py regrediram para shell do SO na execucao de gate, ou perderam a distincao not_evaluated" >&2
  exit 1
fi

echo "check-shell-posix-portability: OK — $checked assinaturas de execucao de gate via sh -c confirmadas em barrier.js e barrier.py"
