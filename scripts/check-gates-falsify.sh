#!/usr/bin/env bash
# P4 — Falsificação dos gates de paridade (REQ-2026-07-26-robustez-gates)
#
# Cada gate deve reprovar um cenário negativo concreto — "CI verde" sem essa
# prova é um gate não-verificado. Este script monta o cenário quebrado, afirma
# que o gate retorna exit != 0 E que a saída contém o diagnóstico esperado.
# Roda dentro de `make quality`, após os gates positivos.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

# ML-2D (ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify): o
# gerador de chunks (scripts/gen-falsify-chunks.py) materializa este preâmbulo
# (byte a byte, sem edição) fora de $ROOT_DIR/scripts/ — sem o override,
# ${BASH_SOURCE[0]} aponta para o chunk em tmp e ROOT_DIR resolve errado
# (reproduzido: `cp: .../cmd/.: No such file or directory`). O driver
# (scripts/run-gates-falsify-parallel.sh) exporta TRACKFW_ROOT_DIR antes de
# invocar cada chunk; a invocação direta do script original (sem a env var)
# continua resolvendo pelo próprio caminho, sem mudança de comportamento.
ROOT_DIR=${TRACKFW_ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$ROOT_DIR/scripts/lib-crlf-normalize.sh"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-falsify.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# $HOME sintético e isolado por padrão para o script INTEIRO — nunca o real. Sem isto, qualquer
# cenário que rode `trackfw validate` (ou qualquer comando que passe por Validate()/
# ValidateTagged()) sem controlar $HOME explicitamente enxerga o escopo GLOBAL de guards de quem
# roda o gate: desde ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
# integridade-independente-de-fiacao (ML-3A),
# git_branch_guard_script_integrity/credential_guard_script_integrity disparam pela EXISTÊNCIA do
# script em ~/.trackfw/scripts/, não mais só quando há fiação — um $HOME real com o harness
# instalado e o script desatualizado (o próprio caso que motivou aquela REQ) faria dezenas de
# cenários pré-existentes, que nunca tiveram nada a ver com guards, reportar um warning
# inesperado. Mesmo precedente do Cenário 46.
#
# GOPATH/GOCACHE/GOMODCACHE são fixados nos valores REAIS antes de isolar $HOME: os binários
# isolados dos Cenários 25+ chamam `go build`, que resolve esses três a partir de $HOME por
# padrão — sem fixá-los explicitamente, um $HOME sintético novo a cada run forçaria `go build` a
# rebaixar o módulo inteiro (lento) e o cache do Go grava arquivos read-only que
# `trap rm -rf "$WORK"` não consegue apagar (permission denied). Isolar só $HOME, com
# GOPATH/GOCACHE/GOMODCACHE reais, dá o melhor dos dois mundos: nenhum `trackfw validate` deste
# script enxerga guards globais reais, e `go build` continua rápido e usa o cache real.
export GOPATH="${GOPATH:-$(go env GOPATH)}"
export GOCACHE="${GOCACHE:-$(go env GOCACHE)}"
export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"
export HOME="$WORK/home"
mkdir -p "$HOME"

# ---------------------------------------------------------------------------
# ML-1A (ROADMAP-2026-09-07-gates-rodam-no-windows-resolucao-de-interpretador-
# e-binario): resolução de PY_BIN — ponto único de escolha do interpretador
# Python usado por TODO python3 executado por este script (nunca dentro de
# heredoc/corpus comparado, só o que é de fato invocado).
#
# Medido na VM Windows 10 Pro ARM64, 2026-09-07: `python3` no PATH resolve
# para o stub da Microsoft Store (Microsoft/WindowsApps/python3), que
# imprime "Python was not found..." e sai com rc=49 mesmo com um Python
# real instalado e funcional em outro ponto do PATH (`python`/`py`). Trocar
# `python3` por `python` por `sed` não é seguro em geral (em algumas
# instalações Linux `python` não existe, ou é Python 2) — a escolha exige
# DETECÇÃO, não substituição literal.
#
# Critério de rejeição do stub: candidato só é aceito se
# `"$cand" -c 'import sys; print(sys.version_info[0])'` sair com 0 E
# imprimir "3". O stub reprova nos dois pontos (rc=49, sem stdout "3");
# python/py reais passam (rc=0, stdout "3"). Provado na VM:
#   python3 -c '...'  -> rc=49, stderr "Python was not found..."
#   python  -c '...'  -> rc=0,  stdout "3"
#   py      -c '...'  -> rc=0,  stdout "3"
#
# Ordem de candidatos: python3, python, py -3 — python3 primeiro porque é o
# nome universal em Linux/macOS (onde não há stub); python/py entram como
# fallback só quando python3 reprova o critério, cobrindo o caso Windows
# medido sem mudar nada no comportamento de hoje em Linux/macOS (lá python3
# já passa no critério e é escolhido na primeira tentativa).
resolve_py_bin() {
  local cand
  for cand in python3 python "py -3"; do
    # shellcheck disable=SC2086 -- "py -3" é dois tokens deliberadamente
    if $cand -c 'import sys; print(sys.version_info[0])' 2>/dev/null | grep -qx 3; then
      echo "$cand"
      return 0
    fi
  done
  return 1
}
if ! PY_BIN=$(resolve_py_bin); then
  echo "FAIL [falsify/setup]: nenhum interpretador Python funcional encontrado (tentados: python3, python, py -3) -- candidatos no PATH podem ser o stub da Microsoft Store (Windows) ou estar ausentes" >&2
  exit 1
fi
export PY_BIN

# ML-2A (mesma ROADMAP, Wave 2): PATH shim para `python3` bare -- os ~40
# scripts/check-*.sh copiados para dentro de fixtures (ex.
# check-identity-parity.sh:40,164) chamam `python3` literal e não passam por
# PY_BIN. Medido na VM Windows: com PY_BIN já resolvido, o gate ainda
# reprovava no primeiro desses sub-scripts com a mensagem do stub da Store
# (rc=49), porque o sub-script nunca viu PY_BIN.
#
# Em vez de editar os ~40 arquivos (risco medido no ML-1A: um `sed` ingênuo
# quebra sintaxe dentro de `bash -c "...python3..."` já entre aspas, e um
# heredoc-tracker ingênuo pode descartar substituições reais em silêncio),
# um diretório na FRENTE do PATH intercepta toda chamada por nome nu -- sem
# tocar nenhum sub-script, inclusive os que chamam python3 dentro de heredoc
# ou string já citada. Todo `bash sub-script.sh` invocado por este script
# (serial ou, via gen-falsify-chunks.py, cada chunk paralelo -- o preâmbulo é
# copiado byte a byte) herda este PATH.
#
# No-op em Linux/macOS: lá python3 já É o PY_BIN escolhido (primeira
# tentativa de resolve_py_bin), então o shim reexecuta o mesmo binário.
#
# 🔴 Armadilha medida ao vivo (auto-recursão): PY_BIN pode ser literalmente
# "python3" (nome nu -- resolve_py_bin() só devolve o CANDIDATO que passou,
# não o caminho resolvido). Escrever um shim chamado "python3" que faz
# `exec python3 "$@"` e SÓ DEPOIS prefixar o PATH com o diretório do shim faz
# a resolução de `python3` dentro do próprio shim CAIR NELE MESMO -- todo
# processo filho herda o PATH já modificado. Reproduzido: o gate trava sem
# nunca terminar (nenhuma chunk chega ao sentinela). A correção resolve o
# PRIMEIRO token de PY_BIN para um caminho ABSOLUTO via `command -v` -- feito
# ANTES de tocar o PATH -- para que o shim nunca aponte para si mesmo.
# 🔴 2ª armadilha medida ao vivo: alguns sub-scripts (check-release-tag-parity.sh,
# check-push-force-parity.sh, check-doctor-remote-parity.sh) fazem
# `REAL_PYTHON3=$(command -v python3)` e depois symlinkam para dentro de um
# PATH PRÓPRIO e ESTREITO que NUNCA herda o PATH do chamador (isolamento
# deliberado deles, contra vazamento de `gh`/`git` do host). Nesse PATH
# estreito não sobra `/usr/bin`/`/bin` -- então um shim com shebang
# `#!/usr/bin/env bash` falha ali com "env: bash: No such file or
# directory", porque `env` não acha `bash` nesse PATH restrito. `#!/bin/sh`
# não tem esse problema: o kernel carrega o interpretador pelo caminho
# ABSOLUTO da shebang, sem nenhuma busca em PATH -- funciona em qualquer
# PATH que o processo filho receber, restrito ou não.
FALSIFY_PY_SHIM_DIR="$WORK/py-shim"
mkdir -p "$FALSIFY_PY_SHIM_DIR"
FALSIFY_PY_FIRST_TOKEN="${PY_BIN%% *}"
FALSIFY_PY_REST="${PY_BIN#"$FALSIFY_PY_FIRST_TOKEN"}"
FALSIFY_PY_FIRST_ABS=$(command -v "$FALSIFY_PY_FIRST_TOKEN")
printf '#!/bin/sh\nexec %s%s "$@"\n' "$FALSIFY_PY_FIRST_ABS" "$FALSIFY_PY_REST" > "$FALSIFY_PY_SHIM_DIR/python3"
chmod +x "$FALSIFY_PY_SHIM_DIR/python3"
export PATH="$FALSIFY_PY_SHIM_DIR:$PATH"

# ---------------------------------------------------------------------------
# ML-1B (mesma ROADMAP): resolução do binário Go do CLI (FALSIFY_GO_BIN) —
# ponto único usado por todo `GO_BIN="$FALSIFY_GO_BIN"` deste script.
# Variáveis Txx_BIN/T*_GO_BIN (binários isolados construídos pelo próprio
# script via build_go_or_fail, ex. Cenário 8) NÃO passam por aqui -- eles já
# existem no ponto de uso, construídos explicitamente contra sua própria
# cópia de módulo (com go.mod).
#
# Causa raiz medida (VM Windows, 2026-09-07): quando GO_BIN não existe, os
# scripts check-*-parity.sh copiados para dentro de uma fixture ($Tn/scripts/)
# caem num fallback que builda com `cd "$ROOT_DIR"` -- mas o ROOT_DIR ali é
# LOCAL à cópia (resolvido via ${BASH_SOURCE[0]}), uma fixture sem go.mod.
# Reproduzido: `go: go.mod file not found in current directory or any parent
# directory`. Sintoma sem relação com a causa real (binário ausente) --
# reproduzido nas duas hipóteses anteriores do arquiteto, ambas falsificadas.
#
# Honra o sufixo de plataforma (`go env GOEXE` -- ".exe" no Windows, vazio
# em Linux/macOS) e, se o binário não existir em NENHUMA forma, falha alto
# aqui -- antes de qualquer cenário tentar usá-lo -- nomeando a causa real,
# em vez de deixar o fallback quebrado de cada sub-script produzir o erro de
# go.mod enganoso.
GO_EXE_SUFFIX=$(go env GOEXE 2>/dev/null || true)
if [[ -n "$GO_EXE_SUFFIX" && -x "$ROOT_DIR/bin/trackfw$GO_EXE_SUFFIX" ]]; then
  FALSIFY_GO_BIN="$ROOT_DIR/bin/trackfw$GO_EXE_SUFFIX"
elif [[ -x "$ROOT_DIR/bin/trackfw" ]]; then
  FALSIFY_GO_BIN="$ROOT_DIR/bin/trackfw"
else
  echo "FAIL [falsify/setup]: binário ausente -- procurado em '$ROOT_DIR/bin/trackfw$GO_EXE_SUFFIX' e '$ROOT_DIR/bin/trackfw', nenhum existe/é executável. Rode 'go build -o bin/trackfw ./cmd/trackfw' (ou 'make build') antes de check-gates-falsify.sh." >&2
  exit 1
fi
export FALSIFY_GO_BIN

# ---------------------------------------------------------------------------
# ML-2B (mesma ROADMAP-2026-09-07-gates-rodam-no-windows..., Wave 2 revisada
# pelo arquiteto): modo de ENUMERAÇÃO REPRODUZÍVEL. Substitui a sonda
# descartável `check-gates-falsify-PROBE.sh` (nunca commitada -- ver ML-2A) --
# em vez de neutralizar `exit 1` numa cópia paralela do arquivo, o próprio
# script sabe enumerar, sob uma flag explícita.
#
# Default (TRACKFW_FALSIFY_ENUMERATE não setada, ou setada para qualquer
# valor != "1"): DESLIGADO. Comportamento byte-idêntico ao script de antes
# deste ML -- toda reprovação de cenário aborta imediatamente via `exit 1`,
# como sempre. Nenhuma linha nova é emitida em stdout/stderr, nenhum branch
# novo é tomado: os ~199 pontos de `exit 1` abaixo (fora os 2 pré-flight
# acima, linhas 98 e 178, que continuam `exit 1` puro -- resolução de
# interpretador/binário não é resultado de cenário) passam a chamar
# falsify_fail_point / falsify_fail_point_fn, que só fazem `exit 1` quando
# a flag está desligada -- primeira linha de cada uma, sem nenhum efeito
# colateral antes disso.
#
# Ligado (TRACKFW_FALSIFY_ENUMERATE=1): cada reprovação é CONTADA em vez de
# abortar o processo, e a execução segue para o próximo cenário. As três
# guardas inegociáveis (parecer do hades-tf sobre TRACKFW_FALSIFY_SCRIPT):
#
# 1. NUNCA torna o gate verde: o exit code final do script (ver o bloco de
#    fechamento no fim do arquivo) é != 0 sempre que houver qualquer
#    reprovação contada em $FALSIFY_ENUM_TALLY, independentemente do modo --
#    contrato idêntico ao de hoje (exit 1 na primeira reprovação também já
#    garantia isso; aqui é o acumulado).
#
#    🔴 Medido por falsificação (relatório do ML-2B) -- corrigido depois de
#    reprovar na primeira tentativa: um contador em VARIÁVEL DE SHELL
#    (`FALSIFY_ENUM_FAILURES=$((...))`) só sobrevive no processo/subshell
#    onde a atribuição rodou. Boa parte dos ~184 pontos de
#    `falsify_fail_point` está dentro de `( ... )`, `$( ... )` ou estágio de
#    pipeline -- a atribuição nesses casos muta a cópia da SUBSHELL e some
#    quando ela termina; o processo pai nunca vê o incremento, a checagem
#    final encontra 0 e sai 0 com `FAIL` no log -- exatamente "transformar
#    vermelho em verde". Por isso a contagem é um ARQUIVO em $WORK (que já
#    existe desde a linha ~31, antes deste bloco): escrita em arquivo
#    atravessa fronteira de subshell; `mktemp -d` é por processo, então cada
#    chunk mantém sua própria contagem sem cruzar com a de outro chunk.
# 2. Rastro em stderr sempre que ativado, com o valor efetivo e o default --
#    ver o `echo` logo abaixo, mesmo padrão do ML-2E para os overrides do
#    driver paralelo.
# 3. Não é caminho de produção: nenhuma referência à variável entra no
#    Makefile; `make quality`/`make parity` continuam fail-fast porque
#    nunca setam TRACKFW_FALSIFY_ENUMERATE -- confirmado nesta sessão via
#    `grep -rn TRACKFW_FALSIFY_ENUMERATE Makefile .github/workflows/ scripts/`
#    (nenhuma ocorrência fora deste arquivo e do gerador de chunks).
TRACKFW_FALSIFY_ENUMERATE=${TRACKFW_FALSIFY_ENUMERATE:-0}
FALSIFY_ENUM_TALLY="$WORK/enum-failures"
FALSIFY_SUCCESS_TALLY="$WORK/success-count"
: > "$FALSIFY_SUCCESS_TALLY"
# Piso mínimo de cenários de falsificação bem-sucedidos. Medido em
# 2026-09-16: 201 cenários. Se o número medido ficar abaixo deste valor o
# gate reprova com diagnóstico explícito em vez de imprimir "passed" com
# zero cenários -- a classe de defeito que este gate existe para eliminar.
# Ao remover cenários legitimamente (consolidação, renomeação), atualize
# este valor no mesmo commit que remove os cenários. Sem esse passo o piso
# fica pessimista e o gate começará a reprovar em execuções limpas.
FALSIFY_SUCCESS_FLOOR=208
if [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]]; then
  : > "$FALSIFY_ENUM_TALLY"
  echo "[falsify/enumerate] modo de enumeração ATIVO (TRACKFW_FALSIFY_ENUMERATE=1, default=0) -- reprovações são contadas e a execução continua para o próximo cenário; o exit code final permanece != 0 se qualquer cenário reprovar. Ferramenta de diagnóstico -- não usada por make quality/parity." >&2
fi

# Ponto de reprovação usado pelo código de cenário FLAT (fora de função,
# ~184 dos ~199 pontos -- a maioria do arquivo, de "Cenário 1" em diante):
# desligado, sai igual a `exit 1` de sempre. Ligado, conta (arquivo, ver
# nota acima -- sobrevive subshell) e RETORNA -- como é chamada de função a
# partir de escopo de script plano, o `return` só retorna desta função para
# o call site, que é exatamente o comportamento desejado (o `if/else ...
# fi` do cenário termina normalmente e o script segue para o próximo
# cenário).
falsify_fail_point() {
  if [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]]; then
    printf 'x\n' >> "$FALSIFY_ENUM_TALLY"
    return 0
  fi
  exit 1
}

# Ponto de reprovação usado DENTRO dos helpers de asserção reutilizáveis
# (assert_fails_with e as 9 funções irmãs, ~15 dos ~199 pontos): diferente
# de falsify_fail_point, aqui o chamador é o CORPO DA PRÓPRIA FUNÇÃO -- só
# `return` (statement, não chamada de função) devolve o controle direto ao
# call site do helper, pulando o `echo "OK ..."` que senão rodaria em
# sequência e imprimiria um OK contraditório logo após o FAIL já emitido.
# Por isso cada um dos ~15 pontos usa o padrão de 2 linhas abaixo em vez de
# chamar esta função sozinha -- ela só existe para centralizar a contagem:
#   falsify_count_failure; [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
#   exit 1
#
# 🔴 `return 0`, NUNCA `return 1`, no ramo habilitado -- medido por
# falsificação (ver relatório do ML-2B): quase todo call site destes helpers
# é uma chamada NUA de topo de script (ex. `assert_fails_with "label" ...`,
# sem `if`/`&&`/`||` em volta), sob `set -euo pipefail`. Se o helper
# devolvesse `return 1`, o PRÓPRIO `set -e` abortaria o chunk no call site --
# byte a byte o mesmo efeito do `exit 1` que este ML existe para evitar,
# só que reintroduzido pela porta dos fundos. O arquivo $FALSIFY_ENUM_TALLY
# (via falsify_count_failure, já chamada antes deste `return`) é o único
# sinal de reprovação que sobrevive -- o valor de retorno da função em si
# tem de ser 0 para o `set -e` do call site não disparar.
falsify_count_failure() {
  printf 'x\n' >> "$FALSIFY_ENUM_TALLY"
}

# Conta uma asserção bem-sucedida (helper passou). Usa arquivo como
# FALSIFY_ENUM_TALLY: sobrevive fronteira de subshell.
falsify_count_success() {
  printf 'x\n' >> "$FALSIFY_SUCCESS_TALLY"
}

# ---------------------------------------------------------------------------
# Helper: assert que o comando retorna exit != 0 E a saída contém o diagnóstico.
# Uso: assert_fails_with LABEL DIAGNOSTIC_PATTERN CMD [ARGS...]
# ---------------------------------------------------------------------------
assert_fails_with() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "FAIL [falsify/$label]: saiu com 0, esperava != 0" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  if ! grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label]: saiu com $status mas falta diagnóstico '$pattern'" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  falsify_count_success
  echo "OK   [falsify/$label]"
}

# ---------------------------------------------------------------------------
# Helper: prova de não-vacuidade quando a regra sob teste é desligável via
# `rules: <nome>: off` (não por edição de código-fonte + rebuild — usado
# pelos Cenários 49/50, que não têm permissão de tocar internal/validator).
# Roda o MESMO comando/critério de assert_fails_with (exit != 0 E mensagem
# presente) contra um fixture com a regra desligada, mas aqui o resultado
# ESPERADO é que esse critério NÃO seja atendido — ou seja, que o braço de
# detecção, se rodasse contra este fixture, ecoaria a mesma linha de FAIL
# que assert_fails_with produziria ("saiu com 0, esperava != 0"). Se o
# critério FOR atendido mesmo com a regra desligada, isso prova que o braço
# de detecção real não depende desta regra — e o cenário reprova aqui.
# ---------------------------------------------------------------------------
assert_would_now_fail() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -ne 0 ]] && grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label/non-vacuity]: com a regra desligada (rules: ...: off), o braço de detecção AINDA passaria (saiu $status e contém '$pattern') — a asserção de detecção não depende desta regra estar ativa" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  echo "PROOF [falsify/$label/non-vacuity]: com a regra desligada, o braço de detecção FALHARIA — assert_fails_with ecoaria \"FAIL [falsify/$label/detected]: saiu com $status, esperava != 0\" (mensagem '$pattern' ausente, exit=$status). Saída real da árvore desligada:"
  echo "$out"
}

# ---------------------------------------------------------------------------
# Helpers: mesma checagem de assert_fails_with/assert_lacks_pattern, mas
# INDIFERENTES ao exit code — usados pelo Cenário 193 (ML-3B): a asserção
# não é "o fixture é totalmente limpo" (aviso de ciclo de vida é warning
# esperado, não violação), é "o padrão X está/não está presente", com o
# exit code verificado à parte quando faz sentido.
# ---------------------------------------------------------------------------
assert_output_contains() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if ! grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label]: esperava conter '$pattern' (exit=$status)" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  falsify_count_success
  echo "OK   [falsify/$label]"
}

assert_output_lacks() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label]: NÃO esperava conter '$pattern' (exit=$status)" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  falsify_count_success
  echo "OK   [falsify/$label]"
}

# ---------------------------------------------------------------------------
# Helper: compila um binário Go isolado e falha com diagnóstico explícito.
# Sem isso, `set -e` aborta o harness antes dos cenários seguintes e esconde
# stderr do `go build`, tornando a prova P4 opaca.
# ---------------------------------------------------------------------------
build_go_or_fail() {
  local label=$1
  local module_dir=$2
  local output_bin=$3
  local log_file="$WORK/${label}.log"

  set +e
  (
    cd "$module_dir" &&
      env GOCACHE="$WORK/go-build-cache" go build -o "$output_bin" ./cmd/trackfw
  ) >"$log_file" 2>&1
  local status=$?
  set -e

  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: go build saiu com $status" >&2
    echo "  command: (cd \"$module_dir\" && GOCACHE=\"$WORK/go-build-cache\" go build -o \"$output_bin\" ./cmd/trackfw)" >&2
    echo "  output:" >&2
    sed 's/^/    /' "$log_file" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Helper: emite scripts/trackfw-git-branch-guard.sh a partir de uma cópia de
# módulo Go isolada (mesmo padrão de build_go_or_fail: copia cmd/+internal/+
# go.mod/go.sum, corrompe UM arquivo, reconstrói) — usado pelos Cenários 58/59
# (ML-1A, ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-
# plugins-e-da-release-7-0-0.md). Em vez de reconstruir o binário `trackfw`
# inteiro (que exigiria simular um wizard interativo de `init` só para chegar
# ao script), adiciona um `cmd/` efêmero PRÓPRIO DA CÓPIA ISOLADA (nunca no
# ROOT_DIR real) que chama generators.GenerateGitBranchGuardScript
# diretamente — mesmo princípio de isolamento de build_go_or_fail, sem
# depender de nenhum subcomando CLI existir.
run_go_guard_dump() {
  local label=$1
  local module_dir=$2
  local out_dir=$3
  local log_file="$WORK/${label}.log"

  mkdir -p "$module_dir/zz_dumpguard" "$out_dir"
  cat > "$module_dir/zz_dumpguard/main.go" <<'GOEOF'
package main

import (
	"os"

	"github.com/kgsaran/trackfw/internal/generators"
)

func main() {
	if err := generators.GenerateGitBranchGuardScript(os.Args[1]); err != nil {
		panic(err)
	}
}
GOEOF

  set +e
  (
    cd "$module_dir" &&
      env GOCACHE="$WORK/go-build-cache" go run ./zz_dumpguard "$out_dir"
  ) >"$log_file" 2>&1
  local status=$?
  set -e

  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: go run ./zz_dumpguard saiu com $status" >&2
    echo "  output:" >&2
    sed 's/^/    /' "$log_file" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Helper: invoca scripts/trackfw-git-branch-guard.sh com um payload JSON via
# stdin e afirma o exit code esperado (0 = allow silencioso, 2 = block). Os
# helpers assert_fails_with/assert_succeeds não servem aqui porque não
# oferecem stdin — o guard só lê o comando via stdin (formato de hook real).
assert_guard_exit() {
  local label=$1
  local script=$2
  local payload=$3
  local want=$4
  local out status
  set +e
  out=$(bash "$script" <<<"$payload" 2>&1)
  status=$?
  set -e
  if [[ "$status" -ne "$want" ]]; then
    echo "FAIL [falsify/$label]: exit $status, esperava $want" >&2
    echo "  payload: $payload" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  falsify_count_success
  echo "OK   [falsify/$label]: exit $status"
}

# assert_writer_no_epipe: reproduz o repro exato da auditoria do arquiteto
# (ML-1B, ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
# integridade-independente-de-fiacao.md) -- um ESCRITOR EXTERNO real
# (subprocesso python3, não here-string do bash, que via <<< pode mascarar o
# EPIPE por já escrever num arquivo temporário) grava o payload JSON no pipe
# de stdin do guard e captura o stderr do PRÓPRIO ESCRITOR (não o do guard).
# want_writer_ok=1 -> escritor deve terminar sem erro (stderr vazio) e o
# guard deve sair com want_guard_exit; want_writer_ok=0 -> escritor DEVE
# receber EPIPE (stderr não-vazio), provando que o cenário de detecção é
# genuíno (não vácuo) antes de testar o build corrompido.
assert_writer_no_epipe() {
  local label=$1
  local script=$2
  local payload=$3
  local want_guard_exit=$4
  local want_writer_ok=$5
  local werr guard_status writer_status
  werr="$WORK/${label//\//_}.werr"
  set +e
  # O payload NUNCA vai por argv (ARG_MAX/MAX_ARG_STRLEN do Linux: 128 KB por
  # argumento -- um payload de 200 KB embutido na fonte do -c estourava e o
  # escritor nem chegava a nascer, tornando o braço vácuo). Ele entra via a
  # here-string do bash (<<<), que é implementada com um arquivo temporário e
  # redirecionamento de fd -- não conta para o limite de argv/envp. O python3
  # lê do PRÓPRIO stdin (o arquivo temporário do bash) e escreve no seu
  # stdout, que É o pipe real para o guard -- preserva o mecanismo que este
  # helper existe para provar (escritor externo, pipe de verdade, EPIPE
  # observável via BrokenPipeError no stderr do escritor).
  "$PY_BIN" -c "
import sys
data = sys.stdin.read()
sys.stdout.write(data)
sys.stdout.flush()
" <<<"$payload" 2>"$werr" | bash "$script" >/dev/null 2>&1
  guard_status=${PIPESTATUS[1]}
  writer_status=${PIPESTATUS[0]}
  set -e
  if [[ "$guard_status" -ne "$want_guard_exit" ]]; then
    echo "FAIL [falsify/$label]: guard exit $guard_status, esperava $want_guard_exit" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  local writer_had_error=0
  [[ -s "$werr" ]] && writer_had_error=1
  if [[ "$want_writer_ok" -eq 1 && "$writer_had_error" -eq 1 ]]; then
    echo "FAIL [falsify/$label]: escritor recebeu erro (EPIPE esperado ausente)" >&2
    echo "  writer stderr:" >&2
    sed 's/^/    /' "$werr" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  if [[ "$want_writer_ok" -eq 0 && "$writer_had_error" -eq 0 ]]; then
    echo "FAIL [falsify/$label]: escritor terminou limpo, EPIPE esperado não ocorreu (cenário vácuo)" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  falsify_count_success
  echo "OK   [falsify/$label]: guard exit $guard_status, writer_status=$writer_status, escritor_erro=$writer_had_error"
}

# Fixture de REQ válida (ADR/Roadmap preenchidos) reaproveitada nos sandboxes
# — evita disparar wip_has_req/req_has_adr/req_has_roadmap, que reprovariam
# por motivo diferente do heading e confundiriam o diagnóstico.

write_roadmap_acceptance_req_fixture() {
  local dest=$1
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<'REQEOF'
---
status: Open
date: 2026-08-01
adr: ""
roadmap: ""
---

# REQ: Flag Source

## Acceptance Criteria
- [ ] Something

## Linked ADR
ADR: none

## Linked Roadmap
Roadmap: none
REQEOF
}

# Remove a n-ésima ocorrência (0-based) do bloco de heading consolidado.
# O bloco é byte-idêntico nas 2 ocorrências (template simples e --from-req)
# em Go/Node/Python — só o texto QUE SEGUE difere — então localizamos por
# índice de ocorrência em vez de âncora de sufixo (frágil e específica por
# linguagem).

remove_roadmap_acceptance_heading() {
  local src_file=$1
  local dest_file=$2
  local occurrence=$3   # 0 = template simples, 1 = --from-req
  local label=$4
  "$PY_BIN" - "$src_file" "$dest_file" "$occurrence" <<'PY'
import pathlib
import sys

src_path, dest_path, occurrence = sys.argv[1], sys.argv[2], int(sys.argv[3])
source = pathlib.Path(src_path).read_text(encoding="utf-8")
block = ("## Acceptance Criteria\n"
         "<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->\n"
         "- [ ]\n- [ ]\n\n")
positions = [i for i in range(len(source)) if source.startswith(block, i)]
if len(positions) != 2:
    raise SystemExit(f"expected 2 occurrences of the heading block, got {len(positions)}")
start = positions[occurrence]
end = start + len(block)
pathlib.Path(dest_path).write_text(source[:start] + source[end:], encoding="utf-8")
PY
  if cmp -s "$src_file" "$dest_file"; then
    echo "FAIL [falsify/setup-s24-$label]: heading não removido — prova P4 inválida" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
}

# Substitui a única ocorrência de `old` por `new` em todo o arquivo. Falha se
# a contagem de ocorrências não for exatamente 1 — evita corromper o alvo
# errado silenciosamente e evita "passar" sem corromper nada.

corrupt_literal() {
  local src=$1 dest=$2 old=$3 new=$4 label=$5
  "$PY_BIN" - "$src" "$dest" "$old" "$new" "$label" <<'PY'
import pathlib
import sys

src, dest, old, new, label = sys.argv[1:6]
source = pathlib.Path(src).read_text(encoding="utf-8")
count = source.count(old)
if count != 1:
    raise SystemExit(f"[{label}] expected exactly 1 occurrence of pattern, got {count}")
pathlib.Path(dest).write_text(source.replace(old, new, 1), encoding="utf-8")
PY
}

# Substitui a primeira ocorrência de `old` por `new`, restrita ao corpo de
# `func_name` (de `def func_name(` até o próximo `\ndef ` ou fim de arquivo).
# Necessário no Python: o literal `req: "{req_path}"` ocorre IDÊNTICO em duas
# funções distintas (_roadmap_template para --req simples,
# generate_roadmap_from_req para --from-req) — sem escopo de função, corromper
# uma corromperia as duas ao mesmo tempo.

corrupt_python_func_literal() {
  local src=$1 dest=$2 func_name=$3 old=$4 new=$5
  "$PY_BIN" - "$src" "$dest" "$func_name" "$old" "$new" <<'PY'
import pathlib
import re
import sys

src, dest, func_name, old, new = sys.argv[1:6]
source = pathlib.Path(src).read_text(encoding="utf-8")
marker = f"def {func_name}("
start = source.index(marker)
tail = source[start + 1:]
next_def = re.search(r"\ndef ", tail)
end = start + 1 + next_def.start() if next_def else len(source)
segment = source[start:end]
if segment.count(old) != 1:
    raise SystemExit(f"[{func_name}] expected exactly 1 occurrence of pattern, got {segment.count(old)}")
new_segment = segment.replace(old, new, 1)
pathlib.Path(dest).write_text(source[:start] + new_segment + source[end:], encoding="utf-8")
PY
}

# Helper: assert que o comando retorna exit 0 E a saída NÃO contém `pattern`.
# Usado para provar que o ciclo LIMPO (código correto, sem corrupção) não
# emite o diagnóstico da corrupção — sem esta prova, o braço de detecção
# (assert_fails_with) sozinho não descarta a hipótese de que o ciclo já
# reprovaria por qualquer outro motivo (seam inativo mascarado por ruído
# alheio à corrupção).

assert_lacks_pattern() {
  local label=$1
  local pattern=$2
  shift 2
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: ciclo limpo saiu com $status, esperava 0" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  if grep -qF "$pattern" <<<"$out"; then
    echo "FAIL [falsify/$label]: seam inativo — o ciclo LIMPO já emite '$pattern'; o cenário de corrupção passaria mesmo sem a corrupção" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  falsify_count_success
  echo "OK   [falsify/$label]"
}

# Helper: assert que o comando retorna exit 0 (prova positiva). Espelha
# assert_fails_with, mas na direção inversa — necessário porque o
# Cenário 26 primeiro precisa provar "código correto não regride" antes de
# provar "código corrompido é detectado".

assert_succeeds() {
  local label=$1
  shift
  local out
  set +e
  out=$("$@" 2>&1)
  local status=$?
  set -e
  if [[ $status -ne 0 ]]; then
    echo "FAIL [falsify/$label]: saiu com $status, esperava 0" >&2
    echo "  output: $out" >&2
    falsify_count_failure
    [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
    exit 1
  fi
  falsify_count_success
  echo "OK   [falsify/$label]: $out"
}

# Scaffold mínimo de projeto trackfw (docs/adr, docs/req, docs/roadmaps/*,
# trackfw.yaml) — mesma estrutura de check-validate-parity.sh.

scaffold_adr_req_project() {
  local dest=$1
  mkdir -p "$dest/docs/adr" "$dest/docs/req" \
    "$dest/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}
  cat > "$dest/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF
}

# ADR fixture com status alinhado entre frontmatter e cabeçalho (caso
# canônico bem formado) — mesmo padrão de adrFixtureContent (validator_test.go).

write_adr_status_fixture() {
  local dest=$1 status=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: $status
date: 2026-08-01
author: ""
---

# ADR: fixture

> Date: 2026-08-01 | Status: $status

## Context
ctx

## Decision
decision
EOF
}

# REQ Done referenciando o ADR via frontmatter \`adr:\` e via a seção
# "## Linked ADR" — mesmo padrão de reqDoneFixtureContent (validator_test.go).
#
# $3 (roadmap_rel, opcional, default "none") — ML-1D (issue #278, rescaldo):
# antes do ML-1B, `Roadmap:` sem valor era um "vazio" que `contentHasMarker`
# (por literal) não detectava, então este fixture passava despercebido pela
# regra `req_has_roadmap` mesmo sem vínculo real. Pós-ML-1B
# (`contentHasMarkerValue`, por VALOR) o vazio passou a ser corretamente
# acusado — o que quebra QUALQUER cenário que precise do ciclo TOTALMENTE
# limpo, não só `assert_succeeds`: `assert_lacks_pattern` (usada nos braços
# "-detects-regression" dos Cenários 27/28) também exige exit 0 do processo
# inteiro, não apenas a ausência do padrão sob prova — uma suposição inicial
# deste ML de que ela "tolera violações extras" estava errada (achado ao
# rodar este script isolado, não coberto por make quality até então rodar
# até essa asserção). Por isso o default de $roadmap_rel deixou de ser vazio
# e passou a ser o literal "none" — mesmo placeholder inofensivo já usado em
# write_roadmap_acceptance_req_fixture (não termina em ".md", então
# ref_targets_exist não tenta resolvê-lo no disco, e tem valor não-branco,
# então req_has_roadmap não o acusa). Cenários que precisam de um alvo REAL
# (ex.: adr-not-accepted/*/superseded-not-a-violation-baseline) continuam
# passando $3 explicitamente.

write_req_done_fixture() {
  local dest=$1 adr_rel=$2 roadmap_rel=${3:-none}
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Done
date: 2026-08-01
author: ""
adr: "$adr_rel"
roadmap: "$roadmap_rel"
---

# REQ: fixture

> Date: 2026-08-01 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: $adr_rel

## Linked Roadmap
Roadmap: $roadmap_rel
EOF
}

# Roadmap mínimo, usado apenas como ALVO real de `write_req_done_fixture $3`
# nos cenários que precisam de ZERO violações (`assert_succeeds`) — sem isto
# o REQ apontaria para um Roadmap que não existe no disco.
#
# $2 (req_rel) — ML-1D (rescaldo, achado ao rodar este script isolado): este
# fixture fica em docs/roadmaps/wip/, então ELE MESMO é varrido por
# wip_has_req e wip_acceptance (mesmo diretório que dispara essas duas
# regras nos Cenários 1-N deste script). Sem req_rel real e sem heading
# "## Acceptance Criteria", os cenários adr-not-accepted/*/superseded-
# not-a-violation-baseline (assert_succeeds) reprovavam com DUAS violações
# NOVAS ("is in wip but has no linked REQ" + "has no acceptance criteria
# block") — o mesmo defeito de "vazio disfarçado" do req_rel original,
# só que no lado Roadmap→REQ em vez de REQ→Roadmap. Verificado rodando o
# fixture isolado contra o binário Go antes desta correção.

write_roadmap_link_target_fixture() {
  local dest=$1 req_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: wip
date: 2026-08-01
req: "$req_rel"
---

# Roadmap: fixture

> Created: 2026-08-01 | Status: wip

## Context
REQ: $req_rel

## Acceptance Criteria
- [x] feito

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
\`\`\`bash
exit 0
\`\`\`
**Critérios de aceite:**
- [x] threat model complete
EOF
}

# REQ Open bloqueada pelo ADR via a seção "## Blocked by ADRs" — mesmo padrão
# do fixture de TestBlockedByDraftADR_REQOpen_ProposedADR_Violates.
# ML-1D (issue #278, rescaldo): "ADR: none" / "Roadmap: none" — mesmo placeholder de
# write_roadmap_acceptance_req_fixture — evitam disparar req_has_adr/req_has_roadmap sem
# apontar para um arquivo real; a seção "## Blocked by ADRs" (não "Linked ADR") é a fonte
# real de $adr_basename para blocked_by_draft_adr, então nenhuma das duas fica sem cobertura.

write_req_open_blocked_fixture() {
  local dest=$1 adr_basename=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Open
date: 2026-08-01
author: ""
adr: ""
roadmap: ""
---

# REQ: bloqueada

> Date: 2026-08-01 | Status: Open

## Motivation
motivo

## Acceptance Criteria
- [ ] pendente

## Linked ADR
ADR: none

## Blocked by ADRs
- $adr_basename (Proposed)

## Linked Roadmap
Roadmap: none
EOF
}

# REQ Done SEM `adr:` no frontmatter, referenciando o ADR só via backtick na
# seção "## Linked ADR" — a forma real usada em REQs do repositório.
#
# "Roadmap: none" (ML-1D, mesmo placeholder de write_roadmap_acceptance_req_fixture):
# contentHasMarkerValue (req_has_roadmap) é independente de extractRefPath — não é afetado
# pela corrupção deste Cenário — então um "Roadmap:" verdadeiramente vazio dispararia
# req_has_roadmap nos dois braços (baseline E detects-regression) e quebraria
# assert_lacks_pattern, que exige exit 0 do processo inteiro, não só a ausência do padrão
# sob prova.

write_req_done_fixture_backtick_body_only() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Done
date: 2026-08-02
author: ""
adr: ""
roadmap: ""
---

# REQ: fixture com backtick

> Date: 2026-08-02 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: \`$adr_rel\` (prosa)

## Linked Roadmap
Roadmap: none
EOF
}

# REQ mínima com status controlado — só o frontmatter importa para o bloco
# Inventory (contagem por status), mas o corpo segue o mesmo esqueleto das
# demais fixtures de REQ do harness (write_req_done_fixture etc.).

write_req_status_fixture() {
  local dest=$1 status=$2 title=$3
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: $status
date: 2026-08-02
author: ""
adr: ""
roadmap: ""
---

# REQ: $title

> Date: 2026-08-02 | Status: $status

## Motivation
motivo

## Acceptance Criteria
- [ ] item

## Linked ADR
ADR:

## Linked Roadmap
Roadmap:
EOF
}

# Roadmap mínimo com status controlado, para popular um estado específico
# (ex: analyzing/) na contagem do bloco Inventory.

write_roadmap_state_fixture() {
  local dest=$1 status=$2 title=$3
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: $status
date: 2026-08-02
---

# Roadmap: $title

> Status: $status
EOF
}

# "Roadmap: none" (ML-1D, mesmo placeholder das demais fixtures deste script): evita
# req_has_roadmap num "Roadmap:" que, de outra forma, ficaria vazio nos dois braços
# (assert_fails_with/assert_lacks_pattern) — assert_lacks_pattern exige exit 0 do
# processo inteiro, não só a ausência do padrão sob prova.

write_req_done_fixture_unpaired_delimiter_body_only() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Done
date: 2026-08-02
author: ""
adr: ""
roadmap: ""
---

# REQ: fixture com delimitador não pareado

> Date: 2026-08-02 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: "$adr_rel'

## Linked Roadmap
Roadmap: none
EOF
}

write_wip_roadmap_fixture() {
  local dest=$1 title=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
# Roadmap: $title

REQ: REQ-001

## Acceptance Criteria
- [ ] item
EOF
}

write_update_hooks_discriminant_fixture() {
  local dest=$1
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<'FIXEOF'
hooks: lefthook
legacy_project_settings:
  hooks: husky
FIXEOF
}

s47_write_claude_guard_hook() {
  local dest=$1
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"}]}]}}
EOF
}

s49_write_fixture() {
  local dest=$1 severity=$2
  scaffold_adr_req_project "$dest"
  cat >> "$dest/trackfw.yaml" <<EOF
rules:
  credential_guard_script_integrity: $severity
EOF
  mkdir -p "$dest/scripts"
  cp "$S49_REF_SCRIPT" "$dest/scripts/trackfw-credential-guard.sh"
  chmod +x "$dest/scripts/trackfw-credential-guard.sh"
}

# rules_severity vazio (padrão) omite o bloco `rules:` inteiro — usado pelos
# braços que não commitam/escrevem nenhum override de severidade.

s50_yaml_content() {
  local mode=$1 rules_severity=${2:-}
  cat <<EOF
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
credential_guard:
  mode: $mode
EOF
  if [[ -n "$rules_severity" ]]; then
    printf 'rules:\n  credential_guard_mode_downgrade: %s\n' "$rules_severity"
  fi
}

# Generalizado (ML-2A) para aceitar o conteúdo do trackfw.yaml commitado —
# antes só commitava s50_yaml_content block; os Cenários 51/52/53 precisam
# commitar HEADs diferentes (com/sem rules: off junto, com/sem
# credential_guard nenhum).

s50_commit_fixture() {
  local dest=$1 yaml_content=$2 commit_msg=$3
  scaffold_adr_req_project "$dest"
  printf '%s' "$yaml_content" > "$dest/trackfw.yaml"
  (
    cd "$dest"
    git init -q
    git config user.email "falsify@trackfw.test"
    git config user.name "trackfw falsify"
    # Isolamento contra config global ambiente do executor (vault/notes/
    # check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md,
    # mesma classe de problema): sem isto, um `commit.gpgsign=true` global
    # falharia sem chave disponível, e um `core.hooksPath` global rodaria
    # hooks do usuário dentro deste fixture descartável.
    git config commit.gpgsign false
    git config core.hooksPath /dev/null
    git add -A
    git commit -q -m "$commit_msg"
  )
}

# ML-1B (Defect 1) — variante de s50_commit_fixture que também configura um
# origin/main. Usada pelos Cenários 51 e 54, onde o ataque combinado
# (mode: warn + rules: off, ambos só em disco) precisa ser detectado mesmo
# com o `rules: off` tentando silenciar a regra.
#
# Sem origin, ruleSeverity cai em diskRuleSeverity → "off" → silencia. Com
# origin/main apontando para o commit (que NÃO tem rules: off), stricter-wins
# retorna "error" → a regra dispara. O bare-repo é criado em mktemp -d (fora
# de $dest e $ROOT_DIR) para não contaminar git status do repositório raiz
# (Cenário 18) nem contar nos objetos do fixture descartável.
#
# O que se perdeu (garantia anterior, agora mais estreita):
#   Sob credentialGuardRuleSeverity() (pré-ML-1A), um repositório git local
#   SEM remote "origin" ainda detectava a edição combinada, pois a âncora era
#   HEAD, sempre disponível. Com o novo modelo (origin/main), um repositório
#   sem origin cai em originAnchorNoRemote → disk-only → "off" silencia.
#   A garantia sobrevive em CI (fetch step presente), mas não em desenvolvimento
#   local sem remote configurado. Estender stricter(default,disk) para
#   originAnchorNoRemote foi avaliado e rejeitado: silenciaria todo projeto
#   local com `rules: {x: off}` legítimo, reproduzindo o mesmo over-reach que
#   o Defeito 3 existe para corrigir.
s5x_commit_fixture_with_origin() {
  local dest=$1 yaml_content=$2 commit_msg=$3
  s50_commit_fixture "$dest" "$yaml_content" "$commit_msg"
  local bare_origin
  bare_origin="$(mktemp -d)"
  (
    cd "$bare_origin"
    git init -q --bare
    git config core.hooksPath /dev/null
  )
  (
    cd "$dest"
    git remote add origin "file://$bare_origin"
    git push -q origin "HEAD:main"
    git fetch -q --depth=1 --no-tags origin "+refs/heads/main:refs/remotes/origin/main"
  )
}

s68_write_project() {
  local dest=$1 rule=$2 severity=$3
  scaffold_adr_req_project "$dest"
  cat >> "$dest/trackfw.yaml" <<EOF
rules:
  $rule: $severity
EOF
}

write_s77_fixture() {
  local dest=$1 gate_line=$2 partial_line=$3 gap_line=$4 none_line=$5
  cat > "$dest" <<EOF
# Fixture cli-parity

## Gate section

$gate_line

Prosa qualquer da seção com gate pleno.

### Gate section com partial

$partial_line

Prosa qualquer da seção com cobertura parcial.

#### Gap section

$gap_line

Prosa qualquer da seção sem gate.

## None section

$none_line

Prosa qualquer da seção que não é contrato.
EOF
}

# $2 (roadmap_rel) precisa ser um alvo REAL (existente no disco): ref_targets_exist tem
# severidade default "error" (não está em ruleDefaults) — um Roadmap: apontando para um
# arquivo inexistente reprovaria o ciclo por um motivo alheio ao seam sob prova aqui, e
# quebraria assert_lacks_pattern (exige exit 0 do processo inteiro). ADR: fica com o
# placeholder de comentário HTML — a única falsa-positiva sob prova nesta fixture.

write_req_adr_placeholder_fixture() {
  local dest=$1 roadmap_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Open
date: 2026-09-06
author: ""
adr: ""
roadmap: "$roadmap_rel"
---

# REQ: fixture de placeholder de ADR

> Date: 2026-09-06 | Status: Open

## Motivation
motivo

## Acceptance Criteria
- [ ] pendente

## Linked ADR
ADR: <!-- preencher depois -->

## Linked Roadmap
Roadmap: $roadmap_rel
EOF
}

# $2 (adr_rel) precisa ser um alvo REAL, mesmo motivo de write_req_adr_placeholder_fixture
# acima — aqui é o Roadmap: que fica com a prosa no meio da frase, a única falsa-positiva
# sob prova nesta fixture.

write_req_roadmap_prose_fixture() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Open
date: 2026-09-06
author: ""
adr: "$adr_rel"
roadmap: ""
---

# REQ: fixture de prosa no meio da linha

> Date: 2026-09-06 | Status: Open

## Motivation
motivo

## Acceptance Criteria
- [ ] pendente

## Linked ADR
ADR: $adr_rel

## Linked Roadmap
veja a secao Roadmap: mais abaixo para detalhes
EOF
}

# Fixture A/C: REQ Open (com ADR Accepted válido) apontando, via
# `roadmap:`/`Roadmap:`, para o caminho ANTIGO ".../wip/<nome>.md" do
# roadmap fixture, que fisicamente já foi movido para docs/roadmaps/done/ —
# reproduz exatamente o defeito medido (o campo grava a pasta de estado, e
# ela ficou velha depois de um `roadmap move`).

write_s193_lifecycle_req_fixture() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Open
date: 2026-08-01
author: ""
adr: "$adr_rel"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-06-s193-fixture.md"
---

# REQ: s193 fixture

> Date: 2026-08-01 | Status: Open

## Motivation
motivo

## Acceptance Criteria
- [ ] pendente

## Linked ADR
ADR: $adr_rel

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-06-s193-fixture.md
EOF
}

write_s193_done_roadmap_fixture() {
  local dest=$1 req_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: done
date: 2026-08-01
req: "$req_rel"
---

# Roadmap: s193 fixture

> Created: 2026-08-01 | Status: done

## Context
REQ: $req_rel

## Acceptance Criteria
- [x] feito
EOF
}

# Fixture B: REQ Done (evita interferência com req_roadmap_lifecycle, que só
# olha REQ Open) referenciando um basename que NÃO existe em nenhum dos 6
# diretórios de estado — vínculo genuinamente quebrado.

write_s193_vacuity_req_fixture() {
  local dest=$1 adr_rel=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Done
date: 2026-08-01
author: ""
adr: "$adr_rel"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-06-s193-vacuity-absent.md"
---

# REQ: s193 vacuity fixture

> Date: 2026-08-01 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: $adr_rel

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-06-s193-vacuity-absent.md
EOF
}

run_node_chain_probe() {
  # $1 = diretório src do npm a exercitar; $2 = raiz das fixtures. Ambos
  # recebidos como ARGUMENTO, não capturados de variável de ambiente do
  # script pai: esta função é reconstruída via `declare -f` e chamada dentro
  # de `bash -c` num subshell novo — uma variável do script pai não-exportada
  # (T194_FIX) não existiria ali, e o valor interpolado silenciosamente
  # viraria string vazia, quebrando o cenário sem diagnóstico (medido: sem
  # este parâmetro explícito, EDGE_A dava false mesmo no baseline correto).
  local npm_src_dir=$1
  local fixdir=$2
  node -e "
const { handleChain } = require('$npm_src_dir/serve/api_chain.js');
const cfg = { adrDirs: ['$fixdir/adr'], reqDir: '$fixdir/req', roadmapDir: '$fixdir/roadmaps', roadmapNamespacing: 'flat' };
const res = { writeHead(){}, end(body){
  const d = JSON.parse(body);
  const roadmapNode = d.nodes.find(n => n.type === 'roadmap');
  const edgeFound = roadmapNode ? d.edges.some(e => e.to === roadmapNode.id) : false;
  console.log('EDGE_A=' + edgeFound);
  const orfaTarget = '$fixdir/roadmaps/wip/ROADMAP-s194-nunca-existiu.md';
  const inventedNode = d.nodes.some(n => n.id === orfaTarget);
  console.log('INVENTED_B=' + inventedNode);
}};
handleChain(cfg, {}, res);
"
}

run_python_chain_probe() {
  # Mesmo motivo do parâmetro explícito de run_node_chain_probe acima.
  local pypi_dir=$1
  local fixdir=$2
  "$PY_BIN" - "$pypi_dir" "$fixdir" <<'PY'
import sys
pypi_dir, fixdir = sys.argv[1:3]
sys.path.insert(0, pypi_dir)
from trackfw.serve.api_chain import get_chain
cfg = {"adr_dirs": [f"{fixdir}/adr"], "req_dir": f"{fixdir}/req", "roadmap_dir": f"{fixdir}/roadmaps", "roadmap_namespacing": "flat"}
d = get_chain(cfg)
roadmap_node = next((n for n in d["nodes"] if n["type"] == "roadmap"), None)
edge_found = any(e["to"] == roadmap_node["id"] for e in d["edges"]) if roadmap_node else False
print(f"EDGE_A={edge_found}")
orfa_target = f"{fixdir}/roadmaps/wip/ROADMAP-s194-nunca-existiu.md"
node_ids = {n["id"] for n in d["nodes"]}
invented_node = orfa_target in node_ids
print(f"INVENTED_B={invented_node}")
PY
}

# ---------------------------------------------------------------------------
# Cenário 12 — check-referential-integrity.sh: REQ com roadmap quebrado →
#              gate detecta referência inexistente no frontmatter.
#
# Objetivo (P4): provar que o gate de integridade referencial reprova uma
# referência canônica quebrada sem deixar resíduo no workspace real.
# ---------------------------------------------------------------------------
T12="$WORK/s12"
mkdir -p "$T12/scripts" "$T12/docs"
cp "$ROOT_DIR/scripts/check-referential-integrity.sh" "$T12/scripts/"
cp -r "$ROOT_DIR/docs/req" "$T12/docs/req"
cp -r "$ROOT_DIR/docs/roadmaps" "$T12/docs/roadmaps"
cp -r "$ROOT_DIR/docs/adr" "$T12/docs/adr"

# Corromper: quebrar uma referência existente em cópia temporária.
cat > "$T12/docs/req/REQ-adr-wizard-e-list-2026-06-11.md" <<'EOF'
---
status: Done
adr: ""
roadmap: "docs/roadmaps/done/MISSING-roadmap-adr-wizard-e-list-2026-06-11.md"
---

# REQ quebrada para prova P4
EOF

assert_fails_with "referential-integrity/missing-roadmap" \
  "referential integrity failed" \
  bash "$T12/scripts/check-referential-integrity.sh"

# ---------------------------------------------------------------------------
# Cenário 13 — check-barrier.sh: a própria prova E2E da barrier é falsificável.
#
# Objetivo (P4): check-barrier.sh (ML-4A) não implementa `trackfw barrier` — ele
# delega aos três runtimes. Falsificar seu conteúdo não é corromper a
# implementação (isso é escopo do ML-2A/2B/2C), mas provar que a asserção do
# próprio harness ("Wave 2 continua bloqueada antes da correção") tem poder de
# reprovação. BARRIER_SELFTEST_BREAK=1 é um seam dedicado (documentado no
# cabeçalho de check-barrier.sh) que corrompe deliberadamente a fixture da
# Wave 2 do cenário 1 para já vir ✅ — reproduzindo a classe exata de defeito
# que a checagem `mls_complete` deveria capturar — e o script deve reportar
# essa reprovação com diagnóstico explícito em vez de sair verde.
# ---------------------------------------------------------------------------
assert_fails_with "barrier/blocked-not-detected" \
  "FAIL [barrier/two-wave-flow/wave2-blocked]: expected exit 1 for Wave 2, got 0" \
  env BARRIER_SELFTEST_BREAK=1 GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenário 18 (AC1/AC2/AC3 — REQ #366) — não-mutação: nenhum gate que invoca
# o binário trackfw pode escrever no working tree do repositório.
#
# Histórico: ML-6I (2026-07-29) criou esta guarda com uma lista fixa de 4
# gates. check-tty-detection.sh nunca foi adicionado à lista e era culpado
# da mutação que a guarda existia para detectar.
#
# AC2 — cobertura por enumeração (opt-out), não por lista fixa:
#   1. Enumera todos os scripts/check-*.sh em tempo de execução via find.
#   2. Filtra: executa apenas os que invocam o binário (contêm \bGO_BIN\b
#      ou bin/trackfw) — classificação automática por leitura do código-fonte.
#   3. Aplica lista de exclusões declaradas (opt-out) — cada exclusão exige
#      motivo explícito separado por '|'; ausência de motivo falha a guarda.
#   4. Um gate novo que invoca o binário entra na cobertura por construção,
#      sem ninguém editar nada.
#   🔴 R6: se a cobertura for outra lista — ainda que gerada — o defeito
#      de fundo continua. A enumeração acontece na execução.
#
# AC3 — detecção de conteúdo via OID de árvore (worktree cópia limpa):
#   1. Copia o repositório para um worktree temporário (sem .git) e inicia
#      um repositório git com commit de baseline — árvore limpa por construção.
#   2. Executa cada gate na cópia (ROOT resolves dentro da cópia; GO_BIN
#      aponta para o binário real $FALSIFY_GO_BIN).
#   3. Após cada gate: git add -A && write-tree → OID de conteúdo.
#   4. Divergência de OID → mutação de conteúdo detectada, gate nomeado.
#   Razão: git status --porcelain reporta estado, não conteúdo. Se a árvore
#   já estiver suja antes da medição (como no run local de make parity), a
#   saída é byte-idêntica antes e depois de nova escrita — a guarda antiga
#   seria satisfeita vacuamente. O OID fecha esse buraco.
# ---------------------------------------------------------------------------

# Exclusões declaradas (opt-out) — formato obrigatório: "basename.sh|motivo"
_MUTATION_EXCLUSIONS=(
  "check-gates-falsify.sh|circular: este script é o executor do cenário de mutação; incluí-lo causaria recursão infinita"
  "check-falsify-shard-coverage.sh|requer três argumentos posicionais e artefatos de shard baixados de CI; invocação sem esses pré-requisitos falha por configuração, não por mutação"
)

# Valida formato das exclusões — uma entrada mal formada falha a guarda
for _excl_entry in "${_MUTATION_EXCLUSIONS[@]}"; do
  if [[ "$_excl_entry" != *"|"* ]]; then
    echo "FAIL [falsify/no-repo-mutation]: exclusão mal formada (falta '|' separando nome de motivo): '$_excl_entry'" >&2
    falsify_fail_point
  fi
  _excl_reason="${_excl_entry#*|}"
  if [[ -z "${_excl_reason//[[:space:]]/}" ]]; then
    echo "FAIL [falsify/no-repo-mutation]: motivo de exclusão vazio: '$_excl_entry'" >&2
    falsify_fail_point
  fi
done

# Monta conjunto de exclusões para lookup O(1)
declare -A _MUT_EXCL=()
for _excl_entry in "${_MUTATION_EXCLUSIONS[@]}"; do
  _MUT_EXCL["${_excl_entry%%|*}"]=1
done

# Enumeração em tempo de execução — opt-out: todos check-*.sh, exceto os declarados
_MUTATION_CANDIDATES=()
while IFS= read -r _gate_path; do
  _bname="$(basename "$_gate_path")"
  # Aplica exclusões declaradas
  [[ "${_MUT_EXCL[$_bname]+set}" == "set" ]] && continue
  # Classifica: invoca o binário trackfw? (GO_BIN ou bin/trackfw no fonte)
  if grep -qE '\bGO_BIN\b|bin/trackfw' "$_gate_path" 2>/dev/null; then
    _MUTATION_CANDIDATES+=("$_gate_path")
  fi
done < <(find "$ROOT_DIR/scripts" -maxdepth 1 -name 'check-*.sh' | sort)

# Guarda de vacuidade — enumeração zero é defeito de cobertura
if [[ ${#_MUTATION_CANDIDATES[@]} -eq 0 ]]; then
  echo "FAIL [falsify/no-repo-mutation]: enumeração em tempo de execução retornou zero gates — cobertura vazia é defeito" >&2
  falsify_fail_point
fi

# Cria worktree limpo para a medição de mutação (AC3 — worktree próprio com checkout limpo)
_MUTATION_COPY="$WORK/mutation-clean"
mkdir -p "$_MUTATION_COPY"
cp -R "$ROOT_DIR/." "$_MUTATION_COPY/"
rm -rf "$_MUTATION_COPY/.git"
git -C "$_MUTATION_COPY" init -q
git -C "$_MUTATION_COPY" -c user.email="mutation-check@localhost" \
    -c user.name="Mutation Check" add -A 2>/dev/null
git -C "$_MUTATION_COPY" -c user.email="mutation-check@localhost" \
    -c user.name="Mutation Check" commit -q -m "mutation-check-baseline"

# Verifica worktree limpo (deve ser sempre por construção — guarda de robustez interna)
_initial_porcelain=$(git -C "$_MUTATION_COPY" status --porcelain)
if [[ -n "$_initial_porcelain" ]]; then
  echo "FAIL [falsify/no-repo-mutation]: worktree de medição não ficou limpo após commit de baseline — erro interno:" >&2
  echo "$_initial_porcelain" >&2
  falsify_fail_point
fi

# OID de referência da árvore limpa (conteúdo, não apenas status)
_baseline_oid=$(git -C "$_MUTATION_COPY" write-tree)

for _gate_path in "${_MUTATION_CANDIDATES[@]}"; do
  _bname="$(basename "$_gate_path")"
  _log="$WORK/mutation-check.$_bname.log"

  _gate_exit=0
  (cd "$_MUTATION_COPY" && GO_BIN="$FALSIFY_GO_BIN" bash "scripts/$_bname") \
    >"$_log" 2>&1 || _gate_exit=$?

  # Computa OID após execução — git add -A captura modificações e arquivos novos não gitignored
  git -C "$_MUTATION_COPY" add -A 2>/dev/null
  _after_oid=$(git -C "$_MUTATION_COPY" write-tree)

  if [[ "$_after_oid" != "$_baseline_oid" ]]; then
    echo "FAIL [falsify/no-repo-mutation]: $_bname alterou o conteúdo da árvore do repositório:" >&2
    git -C "$_MUTATION_COPY" diff HEAD >&2
    # Restaura worktree para não contaminar medição do gate seguinte (enumerate mode)
    git -C "$_MUTATION_COPY" checkout -q -- . 2>/dev/null || true
    git -C "$_MUTATION_COPY" clean -qfd 2>/dev/null || true
    git -C "$_MUTATION_COPY" reset -q HEAD 2>/dev/null || true
    falsify_fail_point
  elif [[ "$_gate_exit" -ne 0 ]]; then
    # Gate falhou mas não mutou a árvore — reporta para diagnóstico; não é falha de mutação
    # (make parity-rest já captura falhas de gate individualmente)
    echo "WARN [falsify/no-repo-mutation]: $_bname saiu com rc=$_gate_exit mas não alterou a árvore" >&2
    sed 's/^/    /' "$_log" >&2
  fi
done

falsify_count_success
echo "OK   [falsify/no-repo-mutation]"

# ---------------------------------------------------------------------------
# Cenário 19 — check-barrier.sh: o gate de heading-malformada-after-target
# (Cenário 9) é falsificável com respeito à classe de bug early-break.
#
# Objetivo (ML-3A, ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis):
# O Cenário 9 de check-barrier.sh cobre a posição "depois da wave alvo" —
# a posição crítica que uma implementação com early-break NÃO detecta.
# Sem esta prova, o cenário seria vacuoso: mesmo que todos os runtimes
# tivessem o bug de early-break (voltando exit 0), o cenário passaria
# verde (cli-parity.md §detection-is-a-full-pre-pass, regra "both positions").
#
# BARRIER_BIS_SELFTEST_BREAK=1 ativa o seam dedicado em check-barrier.sh:
# o Cenário 9 escreve uma fixture válida (sem o heading malformado), fazendo
# o barrier retornar exit 0. A asserção ML-1E (exit 1, wave_headings blocked)
# falha com o diagnóstico explícito abaixo — provando que o cenário tem poder
# de reprovação sobre a classe de defeito de early-break.
#
# Diagnóstico atualizado por ML-1E (REQ #392): antes da ML-1E o Cenário 9
# esperava exit 0 e usava uma guarda de vacuidade de stderr para detectar
# ausência do warning; após ML-1E o cenário espera exit 1 (wave_headings
# blocked), então a falha é "expected exit 1 (blocked: wave_headings check),
# got 0; stderr:" — mais direta e independente de stderr.
#
# Nota: o seam corrompe a FIXTURE, nunca a asserção (mesmo padrão que
# BARRIER_SELFTEST_BREAK do Cenário 13) — não é uma mudança tautológica.
# ---------------------------------------------------------------------------
assert_fails_with "barrier/early-break-after-target-not-detected" \
  'FAIL [barrier/wave-label/malformed-after-target/go]: expected exit 1 (blocked: wave_headings check), got 0; stderr:' \
  env BARRIER_BIS_SELFTEST_BREAK=1 GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"


# ---------------------------------------------------------------------------
# Cenário 24 — ciclo `roadmap new` → `roadmap move ... wip` → `validate`:
#              gerador de roadmap sem o heading `## Acceptance Criteria` faz
#              o roadmap movido para wip reprovar em `validate` com o
#              diagnóstico `wip_acceptance`
#              (ROADMAP-2026-07-31-alinhar-marcador-de-criterios-de-aceite-do-gerador-de-roadmap).
#
# Objetivo (ML-2A): nenhum gate de paridade existente detecta a remoção
# COORDENADA do heading nos três geradores — check-artifact-parity.sh só
# compara os runtimes ENTRE si (byte-a-byte), nunca contra o contrato do
# validador. Sem esta prova, os três geradores poderiam voltar a perder o
# heading simultaneamente (o defeito original, contornado manualmente em
# três ciclos consecutivos — ver Wave 1 do roadmap) e `make quality`
# continuaria verde. Reproduz o ciclo real (`init` → `roadmap new` →
# `roadmap move ... wip` → `validate`) num sandbox isolado por runtime, e
# exige que `validate` reprove com o diagnóstico exato do validador — texto
# idêntico nos três (internal/validator/validator.go:989,
# npm/src/validator/index.js:415, pypi/trackfw/validator.py:669).
#
# Cobre os DOIS caminhos de geração por CLI (AC3 da REQ: "vale também para
# roadmap new --from-req") — não apenas o template simples. O heading ocorre
# 2x, byte-idêntico, em cada gerador (simples e --from-req); sem cobrir os
# dois, alguém poderia remover só o bloco do --from-req nos três CLIs e este
# cenário continuaria verde.
#
# Nota sobre o caminho --from-req (HISTÓRICA — obsoleta a partir da Wave 1 do
# ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req):
# até essa Wave 1, o ciclo com REQ NUNCA reprovava "limpo" — NewRoadmapFromREQ
# gravava `req: "<basename>"` no frontmatter e `ref_targets_exist` sempre
# reprovava essa referência co-ocorrendo com wip_acceptance. Isso NÃO
# invalidava a prova daquele momento: com o gerador correto (pré-Wave 1) a
# violação de wip_acceptance estava ausente da saída (só aparecia a de
# ref_targets_exist); com o gerador corrompido as duas apareciam juntas. O
# padrão buscado por assert_fails_with é o diagnóstico específico de
# wip_acceptance, não a ausência de outras violações — a prova de vivacidade
# abaixo confirmava isso empiricamente.
#
# A partir da Wave 1, o `req:` do frontmatter passou a gravar o caminho
# relativo completo (não mais o basename), então o ciclo `--from-req` AGORA
# reprova limpo sem `ref_targets_exist` co-ocorrente — o Cenário 25 (braço
# de linha de base `*/from-req-baseline`, via assert_lacks_pattern) prova
# isso diretamente. Este parágrafo permanece para explicar por que o
# Cenário 24 nunca precisou de um braço de linha de base equivalente: quando
# foi escrito, o ciclo `--from-req` nunca vinha "limpo" e a ausência de
# `wip_acceptance` já bastava como sinal.
#
# Corrompe a IMPLEMENTAÇÃO (gerador), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21. Cobre os três CLIs: cada runtime tem seu próprio
# gerador disjunto e portanto sua própria prova de vivacidade.
# ---------------------------------------------------------------------------

# Executa o ciclo completo init → roadmap new (simples ou --from-req) →
# roadmap move wip → validate contra um binário/runtime já preparado no
# sandbox $1, e imprime a saída de `validate` (stdout+stderr) preservando o
# exit code — para ser usado dentro de assert_fails_with. $1 é o workdir; o
# restante dos argumentos ("$@" após o shift) é o comando do runtime como
# argv (ex: "$T24G_BIN", ou "node" "npm/bin/trackfw") — sem eval, mesmo
# idioma de invocação direta usado no resto do script (Cenário 23).
ROADMAP_CYCLE_SCRIPT_SIMPLE='
  set -e
  cd "$1"
  shift
  "$@" init >/dev/null
  "$@" roadmap new --title "Falsify Test" --req docs/req/REQ-flag-source.md >/dev/null
  name=$(basename "$(find docs/roadmaps/backlog -name "*.md")")
  "$@" roadmap move "$name" wip >/dev/null
  exec "$@" validate
'
ROADMAP_CYCLE_SCRIPT_FROM_REQ='
  set -e
  cd "$1"
  shift
  "$@" init >/dev/null
  "$@" roadmap new --from-req docs/req/REQ-flag-source.md >/dev/null
  name=$(basename "$(find docs/roadmaps/backlog -name "*.md")")
  "$@" roadmap move "$name" wip >/dev/null
  exec "$@" validate
'

# --- Go -------------------------------------------------------------------
# Cópia enxuta do módulo (cmd/ + internal/ + go.mod/go.sum), não o repo
# inteiro — mesmo padrão do Cenário 23; evita I/O desnecessário e não
# arrasta node_modules/pypi/build para dentro do sandbox de compilação.
for occ_label in "0:simple" "1:from-req"; do
  occ="${occ_label%%:*}"
  path_name="${occ_label##*:}"

  T24G_MOD="$WORK/s24-go-mod-$path_name"
  mkdir -p "$T24G_MOD/cmd" "$T24G_MOD/internal"
  cp -r "$ROOT_DIR/cmd/." "$T24G_MOD/cmd/"
  cp -r "$ROOT_DIR/internal/." "$T24G_MOD/internal/"
  cp "$ROOT_DIR/go.mod" "$T24G_MOD/go.mod"
  cp "$ROOT_DIR/go.sum" "$T24G_MOD/go.sum"
  remove_roadmap_acceptance_heading \
    "$ROOT_DIR/internal/generators/roadmap.go" "$T24G_MOD/internal/generators/roadmap.go" \
    "$occ" "go-$path_name"

  T24G_BIN="$WORK/s24-go-bin-$path_name/trackfw"
  mkdir -p "$(dirname "$T24G_BIN")"
  build_go_or_fail "setup-s24-go-$path_name-build" "$T24G_MOD" "$T24G_BIN"

  T24G="$WORK/s24-go-$path_name"
  mkdir -p "$T24G"
  write_roadmap_acceptance_req_fixture "$T24G/docs/req/REQ-flag-source.md"

  script_var="ROADMAP_CYCLE_SCRIPT_SIMPLE"
  [[ "$path_name" == "from-req" ]] && script_var="ROADMAP_CYCLE_SCRIPT_FROM_REQ"

  assert_fails_with "roadmap-acceptance-heading/go/$path_name" \
    "is in wip but has no acceptance criteria block" \
    bash -c "${!script_var}" _ "$T24G" "$T24G_BIN"
done

# ---------------------------------------------------------------------------
# Helpers reused pelos Cenários 25 e 26 abaixo.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 25 — ciclo `roadmap new --from-req` → `roadmap move ... wip` →
# `validate`: revertendo os 3 geradores para gravar `filepath.Base`/`basename`
# (em vez do caminho relativo completo) no campo `req:` do frontmatter — o
# bug corrigido por
# ADR-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-parametro-roots-morto
# (ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req)
# — o ciclo deve reprovar em `validate` com `ref_targets_exist`.
#
# Objetivo (ML-2A): nenhum gate de paridade existente cobre o CONTRATO
# gerador→validador para o campo `req:` — check-artifact-parity.sh só compara
# os runtimes ENTRE si (byte-a-byte), nunca contra o `os.Stat`/
# `referenceExists` do validador. Sem esta prova, os três geradores poderiam
# voltar a gravar basename simultaneamente (o defeito original deste
# roadmap, "a ferramenta reprova o que ela mesma gerou" pela terceira vez) e
# `make quality` continuaria verde.
#
# Reusa write_roadmap_acceptance_req_fixture e ROADMAP_CYCLE_SCRIPT_FROM_REQ
# (definidos no Cenário 24) — mesma fixture de REQ válida, mesmo idioma de
# ciclo E2E. O diagnóstico esperado ("which does not exist") é a substring
# estática da mensagem de ref_targets_exist nos três runtimes (`roadmap "%s"
# links to REQ "%s" which does not exist` / equivalentes) quando o `req:` do
# frontmatter aponta para um caminho que a validação estrita (sem `roots`,
# conforme o ADR) não resolve — exatamente o que acontece quando o campo
# grava só o basename em vez do caminho relativo completo docs/req/....
#
# Corrompe a IMPLEMENTAÇÃO (gerador), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21/24. Cobre os três CLIs.
# ---------------------------------------------------------------------------

# Diagnóstico estático e discriminante: com a fixture REQ-flag-source.md, o
# ref corrompido é sempre filepath.Base("docs/req/REQ-flag-source.md") =
# "REQ-flag-source.md" — mensagem byte-idêntica nos 3 runtimes
# (validator.go:1463, index.js:758, validator.py:940). Mais específico que
# "which does not exist" isolado, que também casa com as mensagens de
# req→ADR e req→Roadmap ausentes (não aplicáveis aqui, mas indistinguíveis
# por um grep genérico).
S25_PATTERN='links to REQ "REQ-flag-source.md" which does not exist'

# --- Go -----------------------------------------------------------------
# Braço de linha de base (ciclo LIMPO, sem corrupção): prova que o gerador
# correto (pós-Wave 1) não emite mais o diagnóstico — sem isto, o braço de
# detecção abaixo não descartaria "o ciclo já reprovava por outro motivo".
T25G_BASE_BIN="$WORK/s25-go-base-bin/trackfw"
mkdir -p "$(dirname "$T25G_BASE_BIN")"
build_go_or_fail "setup-s25-go-baseline-build" "$ROOT_DIR" "$T25G_BASE_BIN"

T25G_BASE="$WORK/s25-go-base"
mkdir -p "$T25G_BASE"
write_roadmap_acceptance_req_fixture "$T25G_BASE/docs/req/REQ-flag-source.md"

assert_lacks_pattern "roadmap-req-frontmatter-path/go/from-req-baseline" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25G_BASE" "$T25G_BASE_BIN"

# Braço de detecção: gerador revertido para gravar basename.
T25G_MOD="$WORK/s25-go-mod"
mkdir -p "$T25G_MOD/cmd" "$T25G_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T25G_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T25G_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T25G_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T25G_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/roadmap.go" "$T25G_MOD/internal/generators/roadmap.go" \
  'date, reqPath, squadVal, title, date, filepath.Base(reqPath), reqPath, adrRef, mlSection.String())' \
  'date, filepath.Base(reqPath), squadVal, title, date, filepath.Base(reqPath), reqPath, adrRef, mlSection.String())' \
  "s25-go"

T25G_BIN="$WORK/s25-go-bin/trackfw"
mkdir -p "$(dirname "$T25G_BIN")"
build_go_or_fail "setup-s25-go-build" "$T25G_MOD" "$T25G_BIN"

T25G="$WORK/s25-go"
mkdir -p "$T25G"
write_roadmap_acceptance_req_fixture "$T25G/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/go/from-req" \
  "$S25_PATTERN" \
  bash -c "$ROADMAP_CYCLE_SCRIPT_FROM_REQ" _ "$T25G" "$T25G_BIN"

# ---------------------------------------------------------------------------
# Cenário 26 — AC2b: o caminho SIMPLES (`roadmap new --title <t> --req
# <path>`) também deve gravar o caminho completo no `req:` do frontmatter.
#
# Diferente do Cenário 25, uma regressão aqui NÃO produz uma violação de
# `validate` — `extractRefPath` tem early-return para valor vazio, então
# `req: ""` é um falso-NEGATIVO silencioso (documentado no roadmap como "bug
# irmão AC2b": este próprio ciclo de trabalho foi gerado com `--req` e saiu
# com `req: ""` antes da Wave 1). `assert_fails_with` não serve para provar
# a REGRESSÃO em si — validate não reprova nem antes nem depois — então este
# cenário inspeciona o artefato gerado diretamente:
#   1. prova positiva: com o gerador correto, o campo `req:` sai não-vazio
#      nos 3 CLIs (regressão NÃO presente);
#   2. prova de detecção: revertendo o gerador para gravar `req: ""` sempre
#      (o defeito original), a MESMA checagem reprova com diagnóstico
#      explícito — provando que a checagem tem poder de reprovação, não é
#      vácua.
# Sem o passo 2, o passo 1 sozinho não provaria nada: um `grep` que sempre
# retorna "ok" também "passaria" o passo 1.
#
# Corrompe a IMPLEMENTAÇÃO (gerador), nunca a asserção. Cobre os três CLIs.
# ---------------------------------------------------------------------------

# Ciclo simples (--req) num sandbox $1 usando o runtime dado em "$@": roda
# `init` + `roadmap new --req`, localiza o arquivo gerado e extrai o valor
# do campo `req:` do frontmatter. Compara contra o caminho EXATO passado a
# --req (não apenas "não-vazio") — uma regressão que gravasse o basename em
# vez do caminho completo no caminho simples (a mesma classe de defeito do
# Cenário 25, só que aqui) passaria despercebida por um teste de
# não-vazio. Sai com exit 1 e diagnóstico explícito se o campo divergir —
# usado tanto para a prova positiva (código correto, chamado diretamente)
# quanto para a prova de detecção (código corrompido, via assert_fails_with).
SIMPLE_REQ_FIELD_SCRIPT='
  set -e
  cd "$1"
  shift
  "$@" init >/dev/null
  "$@" roadmap new --title "AC2b Flag Source" --req docs/req/REQ-flag-source.md >/dev/null
  name=$(basename "$(find docs/roadmaps/backlog -name "*.md")")
  value=$(grep -m1 "^req: " "docs/roadmaps/backlog/$name" | sed -E "s/^req: \"?([^\"]*)\"?\$/\1/")
  if [[ "$value" != "docs/req/REQ-flag-source.md" ]]; then
    echo "req: field mismatch in roadmap generated via --req simple path (AC2b regression — expected docs/req/REQ-flag-source.md, got $value; validate does not flag this silently)"
    falsify_fail_point
  fi
  echo "req: field = $value (matches --req path, AC2b holds)"
'

# --- Go: prova positiva --------------------------------------------------
# Binário isolado (não $ROOT_DIR/bin/trackfw): a prova não pode depender de
# `make build` já ter rodado antes deste script — mesmo padrão de
# auto-suficiência do braço de detecção logo abaixo.
T26_BASE_GO_BIN="$WORK/s26-base-go-bin/trackfw"
mkdir -p "$(dirname "$T26_BASE_GO_BIN")"
build_go_or_fail "setup-s26-go-baseline-build" "$ROOT_DIR" "$T26_BASE_GO_BIN"

T26_BASE_GO="$WORK/s26-base-go"
mkdir -p "$T26_BASE_GO"
write_roadmap_acceptance_req_fixture "$T26_BASE_GO/docs/req/REQ-flag-source.md"
assert_succeeds "roadmap-req-frontmatter-path/go/simple-baseline" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26_BASE_GO" "$T26_BASE_GO_BIN"

# --- Go: prova de detecção (gerador corrompido para req: "" sempre) -------
T26C_GO_MOD="$WORK/s26-corrupt-go-mod"
mkdir -p "$T26C_GO_MOD/cmd" "$T26C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T26C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T26C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T26C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T26C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/roadmap.go" "$T26C_GO_MOD/internal/generators/roadmap.go" \
  ', date, content.REQPath, squadVal, content.Title, date, content.REQPath)' \
  ', date, "", squadVal, content.Title, date, content.REQPath)' \
  "s26-go"

T26C_GO_BIN="$WORK/s26-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T26C_GO_BIN")"
build_go_or_fail "setup-s26-go-build" "$T26C_GO_MOD" "$T26C_GO_BIN"

T26C_GO="$WORK/s26-corrupt-go"
mkdir -p "$T26C_GO"
write_roadmap_acceptance_req_fixture "$T26C_GO/docs/req/REQ-flag-source.md"

assert_fails_with "roadmap-req-frontmatter-path/go/simple-detects-regression" \
  "AC2b regression" \
  bash -c "$SIMPLE_REQ_FIELD_SCRIPT" _ "$T26C_GO" "$T26C_GO_BIN"

# ---------------------------------------------------------------------------
# Cenário 27 — validate: adr_accepted_when_req_done + blocked_by_draft_adr
# (ROADMAP-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida,
# ML-2A). Sem este cenário, `check-validate-parity.sh` passava vacuamente
# neste repositório — nenhum artefato aqui viola as regras novas, então um
# gate "verde" não discriminava a existência das regras de sua ausência. O
# mesmo valia para a correção da cegueira de `blocked_by_draft_adr` a
# `Status: Proposed` (o caminho normal de `adr new`) — nenhuma REQ Open deste
# repositório é bloqueada por ADR Proposed.
#
# Cobre as DUAS regras × os TRÊS CLIs, com dois braços por CLI:
#   - baseline: projeto-fixture com ADR Proposed + REQ Done referenciando-o
#     (deve violar adr_accepted_when_req_done) e REQ Open bloqueada pelo
#     mesmo ADR (deve violar blocked_by_draft_adr) — código correto,
#     assert_fails_with nos dois diagnósticos; e um segundo projeto com ADR
#     Superseded (aceito por exclusão) + REQ Done referenciando-o — não deve
#     violar, assert_succeeds.
#   - detecção: neutraliza o helper de resolução de status do ADR
#     (resolveAdrStatus/resolveAdrStatus/_extract_adr_status→_adr_not_accepted,
#     conforme o CLI) para sempre resolver "aceito"; roda validate contra o
#     MESMO projeto-fixture violador e prova, via assert_lacks_pattern (exige
#     exit 0 E ausência do diagnóstico), que as duas violações desaparecem —
#     a checagem tem poder de reprovação, não é vácua.
#
# Corrompe a IMPLEMENTAÇÃO (validador), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21/24/26.
# ---------------------------------------------------------------------------

S27_MSG_ACCEPTED='is not accepted (status: Proposed)'
S27_MSG_BLOCKED='is blocked by not-accepted ADR: ADR-2026-08-01-proposed-fixture.md'

# --- Go: prova positiva (projeto violador + projeto não-violador) ---------
T27_GO_BIN="$WORK/s27-go-bin/trackfw"
mkdir -p "$(dirname "$T27_GO_BIN")"
build_go_or_fail "setup-s27-go-baseline-build" "$ROOT_DIR" "$T27_GO_BIN"

T27_GO_VIOLATING="$WORK/s27-go-violating"
scaffold_adr_req_project "$T27_GO_VIOLATING"
write_adr_status_fixture "$T27_GO_VIOLATING/docs/adr/ADR-2026-08-01-proposed-fixture.md" "Proposed"
write_req_done_fixture "$T27_GO_VIOLATING/docs/req/REQ-2026-08-01-done-fixture.md" \
  "docs/adr/ADR-2026-08-01-proposed-fixture.md"
write_req_open_blocked_fixture "$T27_GO_VIOLATING/docs/req/REQ-2026-08-01-blocked-fixture.md" \
  "ADR-2026-08-01-proposed-fixture.md"

assert_fails_with "adr-not-accepted/go/adr_accepted_when_req_done-baseline" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27_GO_BIN' validate"
assert_fails_with "adr-not-accepted/go/blocked_by_draft_adr-baseline" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27_GO_BIN' validate"

T27_GO_CLEAN="$WORK/s27-go-clean"
scaffold_adr_req_project "$T27_GO_CLEAN"
write_adr_status_fixture "$T27_GO_CLEAN/docs/adr/ADR-2026-08-01-superseded-fixture.md" "Superseded"
write_roadmap_link_target_fixture "$T27_GO_CLEAN/docs/roadmaps/wip/ROADMAP-2026-08-01-superseded-fixture.md" \
  "docs/req/REQ-2026-08-01-done-superseded-fixture.md"
write_req_done_fixture "$T27_GO_CLEAN/docs/req/REQ-2026-08-01-done-superseded-fixture.md" \
  "docs/adr/ADR-2026-08-01-superseded-fixture.md" \
  "docs/roadmaps/wip/ROADMAP-2026-08-01-superseded-fixture.md"

assert_succeeds "adr-not-accepted/go/superseded-not-a-violation-baseline" \
  bash -c "cd '$T27_GO_CLEAN' && exec '$T27_GO_BIN' validate"

# --- Go: prova de detecção (resolveAdrStatus neutralizado) -----------------
T27C_GO_MOD="$WORK/s27-corrupt-go-mod"
mkdir -p "$T27C_GO_MOD/cmd" "$T27C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T27C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T27C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T27C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T27C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T27C_GO_MOD/internal/validator/validator.go" \
  'func resolveAdrStatus(content string) string {
	if status := extractFrontmatterField(content, "status"); status != "" {
		return status
	}' \
  'func resolveAdrStatus(content string) string {
	return "Accepted"
	if status := extractFrontmatterField(content, "status"); status != "" {
		return status
	}' \
  "s27-go"

T27C_GO_BIN="$WORK/s27-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T27C_GO_BIN")"
build_go_or_fail "setup-s27-go-build" "$T27C_GO_MOD" "$T27C_GO_BIN"

assert_lacks_pattern "adr-not-accepted/go/adr_accepted_when_req_done-detects-regression" \
  "$S27_MSG_ACCEPTED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27C_GO_BIN' validate"
assert_lacks_pattern "adr-not-accepted/go/blocked_by_draft_adr-detects-regression" \
  "$S27_MSG_BLOCKED" \
  bash -c "cd '$T27_GO_VIOLATING' && exec '$T27C_GO_BIN' validate"


# ---------------------------------------------------------------------------
# Cenário 28 — extractRefPath (e equivalentes) removem backtick da referência
# (REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-
# validate-no-python)
#
# `` ADR: `docs/adr/X.md` (prosa) `` é a forma real usada em REQs do próprio
# repositório. SEM remoção de backtick, o token extraído é "`docs/adr/X.md`"
# — não termina em ".md" — e a referência fica invisível EM SILÊNCIO: nenhuma
# regra que use extractRefPath a alcança. É especialmente grave quando a REQ
# NÃO tem `adr:` no frontmatter (só a prosa do corpo referencia o ADR) — o
# cenário aqui reproduz exatamente essa forma, sem fixture com backtick a
# checagem seria vácua (vault/notes/deteccao-de-status-de-adr-divergencias-
# entre-clis-2026-08-01.md).
#
# Cobre os TRÊS CLIs, dois braços cada:
#   - baseline: REQ Done SEM `adr:` no frontmatter, referenciando o ADR só via
#     `` ADR: `docs/adr/X.md` (prosa) `` na seção "## Linked ADR"; ADR alvo
#     Proposed. Código correto → assert_fails_with adr_accepted_when_req_done.
#   - detecção: reverte a remoção do backtick no extrator do CLI (mesmo ponto
#     de código alterado pela Wave 1, revertido ao estado anterior) e roda
#     validate contra o MESMO projeto-fixture violador — prova, via
#     assert_lacks_pattern, que a violação desaparece (a referência volta a
#     ficar invisível), confirmando que a checagem tem poder de reprovação.
#
# Corrompe a IMPLEMENTAÇÃO (extrator), nunca a asserção — mesmo padrão do
# Cenário 27.
# ---------------------------------------------------------------------------

S28_MSG_ACCEPTED='is not accepted (status: Proposed)'

# --- Go: prova positiva -----------------------------------------------------
# Reusa T27_GO_BIN (binário Go limpo, construído a partir do ROOT_DIR sem
# corrupção) — não precisa recompilar. Se o Cenário 27 for removido, mova a
# compilação para cá.
T28_GO_VIOLATING="$WORK/s28-go-violating"
scaffold_adr_req_project "$T28_GO_VIOLATING"
write_adr_status_fixture "$T28_GO_VIOLATING/docs/adr/ADR-2026-08-02-proposed-fixture.md" "Proposed"
write_req_done_fixture_backtick_body_only "$T28_GO_VIOLATING/docs/req/REQ-2026-08-02-backtick-fixture.md" \
  "docs/adr/ADR-2026-08-02-proposed-fixture.md"

assert_fails_with "backtick-ref/go/adr_accepted_when_req_done-baseline" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_GO_VIOLATING' && exec '$T27_GO_BIN' validate"

# --- Go: prova de detecção (backtick reintroduzido em extractRefPath) ------
T28C_GO_MOD="$WORK/s28-corrupt-go-mod"
mkdir -p "$T28C_GO_MOD/cmd" "$T28C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T28C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T28C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T28C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T28C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T28C_GO_MOD/internal/validator/validator.go" \
  'v := strings.Trim(fields[0], "\"'"'"'`")' \
  'v := strings.Trim(fields[0], "\"'"'"'")' \
  "s28-go"

T28C_GO_BIN="$WORK/s28-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T28C_GO_BIN")"
build_go_or_fail "setup-s28-go-build" "$T28C_GO_MOD" "$T28C_GO_BIN"

assert_lacks_pattern "backtick-ref/go/adr_accepted_when_req_done-detects-regression" \
  "$S28_MSG_ACCEPTED" \
  bash -c "cd '$T28_GO_VIOLATING' && exec '$T28C_GO_BIN' validate"

# ---------------------------------------------------------------------------
# Cenário 30 — `trackfw status`: bloco 📊 Inventory byte-idêntico nos 3 CLIs
# no modo flat, com fixture DISCRIMINANTE (roadmap em analyzing/ + REQs
# Open/Done/Closed) — prova ROADMAP-2026-08-02-convergir-o-comando-status-
# dos-tres-clis-num-formato-unico (ML-3A), AC2 (analyzing contado, antes
# omitido em 5 de 6 pontos de enumeração no Python) e AC3 (REQs
# discriminadas por status real, antes Done/Closed agrupados no Python).
#
# O repositório real deste projeto tem "analyzing 0" — não exercitaria a
# correção principal. A fixture PRECISA ter >=1 roadmap em analyzing/ e as
# 3 combinações de status de REQ, senão o cenário não discrimina nada.
#
# Mesmo padrão dos Cenários 28/29: compara contra um LITERAL PINADO, não os
# 3 CLIs entre si — um diff três-a-três passaria se todos derivassem juntos
# do mesmo bug (ex: todos omitindo analyzing), ou se todos imprimissem
# vazio.
#
# Corrompe a IMPLEMENTAÇÃO (Go: a lista de estados enumerados em
# inventoryBlock), nunca a asserção — mesmo padrão dos Cenários
# 14/16/17/20/21/24/25/26/27/28/29.
# ---------------------------------------------------------------------------

S30_PROJECT="$WORK/s30-status-project"
scaffold_adr_req_project "$S30_PROJECT"
write_req_status_fixture "$S30_PROJECT/docs/req/REQ-open.md" "Open" "open fixture"
write_req_status_fixture "$S30_PROJECT/docs/req/REQ-done.md" "Done" "done fixture"
write_req_status_fixture "$S30_PROJECT/docs/req/REQ-closed.md" "Closed" "closed fixture"
write_roadmap_state_fixture "$S30_PROJECT/docs/roadmaps/analyzing/ROADMAP-analyzing.md" "analyzing" "analyzing fixture"

S30_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        3  (1 Open · 1 Done · 1 Closed)\n   Roadmaps    1\n     backlog 0 · analyzing 1 · wip 0\n     blocked 0 · done 0 · abandoned 0\n\n🔄 WIP (0)\n\n❌ Blocked (0)\n\n✅ Done (last 5)\n\n────────────────────────────────────────\n'

# --- prova positiva: os 3 CLIs, contra o literal pinado ---------------------
s30_go_out=$(cd "$S30_PROJECT" && "$T27_GO_BIN" status)$'\n'

if [[ "$s30_go_out" == "$S30_EXPECTED" ]]; then
  falsify_count_success
  echo "OK   [falsify/status-inventory/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/status-inventory/baseline-byte-identical-and-pinned]: esperava '$S30_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s30_go_out")" >&2
  falsify_fail_point
fi

# --- braço de detecção: Go reverte a enumeração de analyzing (5 de 6 -------
# estados) — reproduz o defeito histórico do Python pré-Wave-1 num CLI
# concreto e prova que a comparação byte-a-byte reprova a omissão.
T30C_GO_MOD="$WORK/s30-corrupt-go"
mkdir -p "$T30C_GO_MOD/cmd" "$T30C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T30C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T30C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T30C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T30C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T30C_GO_MOD/internal/validator/validator.go" \
  'states := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}' \
  'states := []string{"backlog", "wip", "blocked", "done", "abandoned"}' \
  "s30-go"

T30C_GO_BIN="$WORK/s30-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T30C_GO_BIN")"
build_go_or_fail "setup-s30-go-corrupt-build" "$T30C_GO_MOD" "$T30C_GO_BIN"

s30c_go_out=$(cd "$S30_PROJECT" && "$T30C_GO_BIN" status)$'\n'
if [[ "$s30c_go_out" != "$S30_EXPECTED" ]]; then
  falsify_count_success
  echo "OK   [falsify/status-inventory/go-detects-analyzing-omission]"
else
  echo "FAIL [falsify/status-inventory/go-detects-analyzing-omission]: enumeração de analyzing revertida mas a comparação continuou passando (checagem vácua)" >&2
  falsify_fail_point
fi

# ---------------------------------------------------------------------------
# Cenário 34 — item 3 do ROADMAP-2026-08-02-fechar-as-duas-divergencias-de-
# parsing-remanescentes-no-python: Go e Node aceitam sequência YAML em bloco
# NÃO indentada ("agents:\n- apolo") — antes (pré-d208971) tratavam a linha
# "- apolo" como top-level (por falta de indentação) e DESCARTAVAM a lista
# em silêncio, caindo no fallback de varrer subdiretórios. O Python já lia
# corretamente (referência).
#
# RETARGET (ML-2A, 2026-08-02, Ártemis): a Wave 1 do ROADMAP-substituir-os-
# parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis substituiu o
# scanner linha-a-linha do Go e do Node por gopkg.in/yaml.v3 / `yaml` 2.x —
# os literais originais (`isListItem`/`continuesOpenList`) não existem mais;
# rodar os 82 cenários herdados ANTES de editar (exigido pelo ML-2A) falhou
# no setup deste cenário com "expected exactly 1 occurrence of pattern, got
# 0", o sintoma descrito em vault/notes/cenarios-de-falsificacao-quebram-em-
# refactor-do-alvo-2026-08-02.md. Diferença deste caso para o do vault: ali
# havia um ponto de código NOVO equivalente para retargetar a corrupção
# preservando a MESMA propriedade (suporte a backtick). Aqui não há mais
# nenhum código que trate "sequência não-indentada" como caso especial — uma
# biblioteca YAML de verdade não tem esse conceito, a fixture não-indentada é
# só YAML válido comum. A corrupção foi retargetada para o ponto genérico
# mais próximo que ainda preserva a intenção operacional do cenário — "a
# lista `agents:` é lida, não descartada" — corrompendo a atribuição de
# `cfg.Agents`/`cfg.agents` a partir do valor já parseado. Isso já NÃO prova
# mais nada sobre indentação especificamente (yaml.v3/`yaml` tratam bloco
# indentado e não-indentado de forma idêntica — não há mais um branch de
# código que só dispara para um dos dois); prova que a leitura da chave
# `agents:` (SEQUÊNCIA em bloco, incluindo a forma não-indentada que a
# fixture já usa) ainda popula `cfg.Agents`, e que removê-la reproduz o
# MESMO sintoma observável de antes (fallback devolve `zeus` extra) — a
# fixture não-indentada permanece no cenário como o vestígio do caso
# histórico original, mas o BRAÇO de detecção não é mais seletivo por
# indentação.
# Fixture discriminante: `agents:` em lista de bloco NÃO indentada,
# configurando SÓ `zeus`, enquanto `docs/roadmaps/` no disco tem `apolo`
# **e** `zeus` (ambos com roadmap em wip/). Os cenários existentes (ex. 31)
# usam forma indentada — não exercitam o defeito.
#
# RETARGET 2 (ML-1A, REQ-2026-08-29, apolo-tf): resolveAgentNamespaces/
# resolve_agent_namespaces (o resolvedor canônico desta REQ) passou a devolver
# a UNIÃO entre `agents:` e o disco — não mais a substituição. Com isso,
# `zeus` deixou de ficar invisível quando `agents: [apolo]` e `zeus/` existe
# só em disco (é exatamente o comportamento que a REQ corrige), e a
# divergência "zeus aparece ou não" que discriminava este cenário até
# REQ-2026-08-29 ficou vácua: zeus agora aparece nos dois braços (parser
# correto OU quebrado), porque o resolvedor sempre lê o disco também.
# RETARGET 3 (ML-3A, REQ-2026-08-29, artemis-tf): a Wave 2 (apolo-tf) somou a
# violação `agent_namespace_undeclared` sobre a mesma união — e essa violação
# devolve um discriminante mais forte que ORDEM, sobrevivendo à mesma
# corrupção testada aqui sem depender de posição relativa na saída (uma
# propriedade de apresentação que qualquer refatoração de UI do `status`
# poderia mudar sem tocar no parsing). `agent_namespace_undeclared` é gated
# por DECLARAÇÃO, não pela união: `zeus`, declarado em `agents: [zeus]`, NUNCA
# aparece como "não declarado" enquanto a lista for lida corretamente — só
# `apolo` (só-disco) aparece. Se `cfg.Agents`/`cfg.agents` for corrompido
# (lista descartada, fica vazia), o resolvedor cai para união vazia + disco:
# TODOS os namespaces em disco — `zeus` INCLUÍDO — viram "não declarados".
# Violação em massa citando `zeus` por nome é o sinal, e ele não pode ocorrer
# por acidente de disco (diferente da presença de um item na saída, `zeus`
# só entra na lista de violação se `agents:` genuinamente falhou ao lê-lo).
# Reproduzido ao vivo nos 3 CLIs (Go/Node — Python fora de escopo deste
# cenário, ver nota abaixo) antes de codar este gate: baseline `validate`
# emite só a violação de `apolo`; corrompido emite as de `apolo` E `zeus`.
#
#   - baseline: `trackfw validate` roda sobre `$S34_PROJECT` com os binários
#     LIMPOS (Go real, Node real) — a violação `agent namespace "zeus" ...
#     is not declared` está AUSENTE (zeus é declarado); a de `apolo` está
#     presente (não é o alvo deste cenário, só ruído esperado).
#   - detecção: Go e Node revertem o mesmo ponto de corrupção genérico já
#     documentado acima (`cfg.Agents = items` / `cfg.agents = items` — a
#     atribuição final a partir do valor já parseado) e provam, por
#     asserção POSITIVA, que a violação de `zeus` PASSA A APARECER na saída
#     corrompida — a ausência no baseline e a presença no corrompido são as
#     duas metades da prova de não-vacuidade (sem a metade "ausente no
#     baseline", uma implementação que sempre acusasse `zeus` passaria por
#     acidente).
#
# Corrompe a IMPLEMENTAÇÃO (o mesmo ponto genérico do braço original, nunca
# reintroduzido — ver RETARGET acima), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21/24/25/26/27/28/29/30/31/32/33. Não toca em pypi/ —
# o Python já está correto neste ponto (item fora do escopo negativo deste
# roadmap, decisão herdada do Cenário 34 original).
# ---------------------------------------------------------------------------

S34_PROJECT="$WORK/s34-config-unindented-agents-project"
mkdir -p "$S34_PROJECT/docs/adr" "$S34_PROJECT/docs/req"
mkdir -p "$S34_PROJECT/docs/roadmaps/zeus"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S34_PROJECT/docs/roadmaps/apolo"/{backlog,analyzing,wip,blocked,done,abandoned}
cat > "$S34_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
- zeus
EOF
write_roadmap_state_fixture "$S34_PROJECT/docs/roadmaps/zeus/wip/ROADMAP-zeus-wip.md" "wip" "zeus wip fixture (declarado em agents:)"
write_roadmap_state_fixture "$S34_PROJECT/docs/roadmaps/apolo/wip/ROADMAP-apolo-wip.md" "wip" "apolo wip fixture (só em disco)"

S34_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    2\n     backlog 0 · analyzing 0 · wip 2\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [zeus] WIP (1)\n    ROADMAP-zeus-wip.md\n  [apolo] WIP (1)\n    ROADMAP-apolo-wip.md\n\n────────────────────────────────────────\n'

s34_go_out=$(cd "$S34_PROJECT" && "$T27_GO_BIN" status)$'\n'

if [[ "$s34_go_out" == "$S34_EXPECTED" ]]; then
  falsify_count_success
  echo "OK   [falsify/config-unindented-agents/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/config-unindented-agents/baseline-byte-identical-and-pinned]: esperava '$S34_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s34_go_out")" >&2
  falsify_fail_point
fi

# Diagnóstico estático e discriminante: byte-idêntico à mensagem da regra
# `agent_namespace_undeclared` (validator.go:1157/index.js/config.py) —
# only ocorre se o resolvedor genuinamente perder `zeus` de `agents:`.
S34_ZEUS_UNDECLARED='agent namespace "zeus" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'
S34_APOLO_UNDECLARED='agent namespace "apolo" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'

# --- braço de baseline: com os binários LIMPOS, zeus (declarado) nunca
# aparece como não-declarado — só apolo (só-disco). Prova de não-vacuidade EM
# DUAS PONTAS: a ausência de zeus sozinha não distingue "validate rodou e a
# regra corretamente poupou zeus" de "validate não rodou nada" (binário
# quebrado, `cd` engolido pelo `; true`, saída vazia) — por isso a asserção
# de apolo PRESENTE é obrigatória aqui: prova que o validate rodou, varreu o
# disco e a regra disparou de verdade, só não para o namespace declarado.
s34_validate_go_out=$(cd "$S34_PROJECT" && "$T27_GO_BIN" validate 2>&1; true)
if ! grep -qF "$S34_APOLO_UNDECLARED" <<<"$s34_validate_go_out"; then
  echo "FAIL [falsify/config-unindented-agents/go/agent-namespace-undeclared-baseline]: apolo (só-disco) deveria estar 'não declarado' no ciclo LIMPO e não está — validate pode não ter rodado (cenário vácuo)" >&2
  echo "  output: $(printf '%q' "$s34_validate_go_out")" >&2
  falsify_fail_point
fi
if grep -qF "$S34_ZEUS_UNDECLARED" <<<"$s34_validate_go_out"; then
  echo "FAIL [falsify/config-unindented-agents/go/agent-namespace-undeclared-baseline]: zeus (declarado em agents:) já aparece como não-declarado no ciclo LIMPO — o cenário seria vácuo" >&2
  echo "  output: $(printf '%q' "$s34_validate_go_out")" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/config-unindented-agents/go/agent-namespace-undeclared-baseline]"

# --- braço de detecção: Go deixa de atribuir cfg.Agents a partir da lista --
# lida (RETARGET — ver comentário no topo do Cenário 34: isListItem/
# continuesOpenList não existem mais pós-yaml.v3)
T34C_GO_MOD="$WORK/s34-corrupt-go"
mkdir -p "$T34C_GO_MOD/cmd" "$T34C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T34C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T34C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T34C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T34C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T34C_GO_MOD/internal/config/config.go" \
  $'\t\t\tcfg.Agents = items\n' \
  $'\t\t\t_ = items\n' \
  "s34-go"

T34C_GO_BIN="$WORK/s34-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T34C_GO_BIN")"
build_go_or_fail "setup-s34-go-corrupt-build" "$T34C_GO_MOD" "$T34C_GO_BIN"

s34c_validate_go_out=$(cd "$S34_PROJECT" && "$T34C_GO_BIN" validate 2>&1; true)
if grep -qF "$S34_ZEUS_UNDECLARED" <<<"$s34c_validate_go_out"; then
  falsify_count_success
  echo "OK   [falsify/config-unindented-agents/go-detects-list-discarded]"
else
  echo "FAIL [falsify/config-unindented-agents/go-detects-list-discarded]: cfg.Agents descartado, mas zeus não virou 'não declarado' na violação agent_namespace_undeclared — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s34c_validate_go_out")" >&2
  falsify_fail_point
fi

# ---------------------------------------------------------------------------
# Cenário 35 — ROADMAP-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-
# config-dos-tres-clis (ML-2A): `agents:` em lista YAML INLINE cujo item
# contém vírgula DENTRO de aspas ("caso 8" do contrato — `["a, b", "c"]` são
# DOIS itens, não três) precisa ser preservado como um único nome de agente
# nos 3 CLIs.
#
# Nenhum cenário existente exercita este caso: o 34 cobre lista em BLOCO não
# indentada (defeito de outro parser, já corrigido antes desta Wave); os
# Cenários 30/31/33 usam `agents:` em bloco, sem flow-style. A tabela de 9
# casos do ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-
# dos-tres-clis foi verificada por teste unitário em cada CLI (ML-1A), mas
# nenhum gate de PARIDADE cross-CLI cobria o caso 8 especificamente — e é o
# único dos nove que uma separação ingênua por vírgula quebra.
#
# Fixture discriminante: agente real chamado `ka, tsu` — diretório em disco
# `docs/roadmaps/ka, tsu/` (vírgula+espaço é caractere válido em nome de
# diretório Unix) contendo um roadmap em wip/. `trackfw.yaml` configura
# `agents: ["ka, tsu", "obi"]` (flow-style, item citado com vírgula
# embutida). Escolhido deliberadamente para não ser vácuo por acidente: um
# parser que separa a vírgula ingenuamente (fora de aspas) produz os
# fragmentos "ka" e "tsu" como agentes SEPARADOS — nenhum dos dois casa com
# o diretório real `ka, tsu` no disco, então o roadmap correspondente
# desaparece INTEIRO da saída (não apenas o nome do agente muda formatação
# — a seção "⚙ WIP by Agent" fica vazia e o Inventory some a contagem).
# Uma fixture só com `[a, b]` (sem vírgula em item) não teria essa
# propriedade: qualquer separação, ingênua ou correta, produziria os mesmos
# dois nomes.
#
# Segunda camada de discriminação, decisiva contra reversão TOTAL do suporte
# inline (não só o ramo de aspas): `docs/roadmaps/zeta/` também existe no
# disco, com wip roadmap PRÓPRIO, mas `zeta` NÃO está na lista configurada.
# Com `agents:` corretamente parseado (inline, não-vazio), `resolveStateDirs`
# itera só os agentes configurados — `zeta` nunca entra na conta, igual ao
# papel de `zeus` no Cenário 34. Se alguém revertesse `isInlineList` por
# inteiro (não só o scanner de aspas), `agents: [...]` cairia no modo bloco,
# não encontraria `- item` nas linhas seguintes, produziria `cfg.Agents`
# vazio, e o CÓDIGO cairia no fallback de varrer `docs/roadmaps/*` — que
# encontraria `ka, tsu`, `obi` E `zeta`. Sem `zeta` no disco, esse fallback
# reproduziria por acidente a MESMA saída do parser correto (mesmo conjunto
# efetivo de agentes com wip), e os três braços de detecção abaixo
# morreriam no setup com "expected exactly 1 occurrence... got 0" — o
# defeito descrito em vault/notes/cenarios-de-falsificacao-quebram-em-
# refactor-do-alvo-2026-08-02.md, aqui por reversão total em vez de
# refactor. Com `zeta` presente e fora da lista, o fallback reintroduziria
# `[zeta] WIP (1)` na saída — divergência inequívoca do pinado.
#
#   - baseline: os 3 CLIs, contra o literal PINADO (capturado rodando os 3
#     CLIs reais contra a fixture), byte-idênticos — `[ka, tsu] WIP (1)`
#     aparece com o roadmap listado, Inventory Roadmaps total 1 (wip 1).
#   - detecção: originalmente revertia, em cada CLI, o ramo de detecção de
#     aspas de `splitTopLevelCommas`/`_split_top_level_commas` (`case r ==
#     '"' || r == '\''`/`ch === '"' || ch === "'"`/`ch in ('"', "'")`).
#
# RETARGET (ML-2A, 2026-08-02, Ártemis): rodar os 82 cenários herdados ANTES
# de editar (exigido pelo ML-2A) — depois de consertado o Cenário 34 (ver
# comentário no topo dele) — revelou o MESMO sintoma aqui: a Wave 1 desta
# REQ eliminou `splitTopLevelCommas`/`_split_top_level_commas` por inteiro
# nos 3 CLIs (`grep -rn splitTopLevelCommas internal/ npm/src/ pypi/` não
# encontra mais nada) — uma biblioteca YAML de verdade faz o parsing de
# sequência em fluxo (incluindo vírgula dentro de aspas) nativamente, sem
# precisar de scanner próprio. Não há mais um "ramo de detecção de aspas"
# para reverter seletivamente — é a MESMA classe de obsolescência do
# Cenário 34 (vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-
# alvo-2026-08-02.md), desta vez mascarada porque `set -euo pipefail` fazia
# o script abortar no Cenário 34 antes de alcançar este. Retargetado para o
# mesmo ponto genérico usado no Cenário 34 (`cfg.Agents = items` /
# `cfg.agents = items` / `cfg["agents"] = items` — a atribuição final a
# partir do valor já parseado pela biblioteca). A fixture (vírgula dentro de
# aspas) continua no cenário como vestígio do caso histórico original, mas o
# braço de detecção deixou de ser seletivo por aspas — prova "a chave
# `agents:` inline é lida", não mais "vírgula dentro de aspas
# especificamente" (que agora é responsabilidade estrutural da biblioteca,
# sem código próprio para corromper).
#
# Corrompe a IMPLEMENTAÇÃO (o ponto de atribuição final de `agents:`, não
# mais `splitTopLevelCommas` — ver RETARGET acima), nunca a asserção — mesmo
# padrão dos Cenários 14/16/17/20/21/24/25/26/27/28/29/30/31/32/33/34. Não
# amplia o suporte YAML (mapas inline, listas aninhadas) — fora de escopo,
# registrado no ADR.
#
# RETARGET 2 (ML-1A, REQ-2026-08-29, apolo-tf): o resolvedor canônico
# (resolveAgentNamespaces) passou a devolver a UNIÃO entre `agents:` e o
# disco. Isso desarma a asserção "presença/ausência de [zeta]" da mesma forma
# que no Cenário 34 (zeta agora aparece nos dois braços) — E piora aqui
# especificamente: o diretório físico `docs/roadmaps/ka, tsu/` já existe em
# disco com esse nome LITERAL, então a união encontra "ka, tsu" via
# varredura de disco mesmo que a atribuição de `cfg.Agents`/`cfg.agents`
# seja corrompida por completo — a fixture original ficou incapaz de provar
# qualquer coisa por presença/ausência. A propriedade usada neste retarget
# (ML-1A) foi ORDEM — ver RETARGET 3 abaixo para o motivo de ter sido
# substituída.
#
# RETARGET 3 (ML-3A, REQ-2026-08-29, artemis-tf): a violação
# `agent_namespace_undeclared` (Wave 2, apolo-tf) devolve um sinal mais forte
# que ORDEM — a mesma razão do Cenário 34, ver o comentário RETARGET 3 lá
# para a justificativa completa. Aqui a fixture (`agents: ["obi", "ka,
# tsu"]`, `zeta` só-disco) já é exatamente o desenho que a violação precisa:
# com o parser correto, `obi` e `ka, tsu` são DECLARADOS — nunca aparecem
# como "não declarados"; só `zeta` aparece. Se `cfg.Agents`/`cfg.agents`/
# `cfg["agents"]` for corrompido (lista descartada), os três — `obi`, `ka,
# tsu` E `zeta` — viram "não declarados". A violação citando `ka, tsu` por
# nome (com a vírgula preservada, vinda do nome do diretório em disco, não
# do parsing de `agents:` — a união sempre soletra o nome como está no
# disco) prova que o parser perdeu a declaração, sem depender de posição na
# saída. Cobre os 3 CLIs (diferente do Cenário 34, que exclui Python por
# decisão herdada — aqui o Python já tinha arm próprio e o mantém).
# ---------------------------------------------------------------------------

S35_PROJECT="$WORK/s35-config-inline-comma-in-quotes-project"
mkdir -p "$S35_PROJECT/docs/adr" "$S35_PROJECT/docs/req"
mkdir -p "$S35_PROJECT/docs/roadmaps/ka, tsu"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S35_PROJECT/docs/roadmaps/obi"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S35_PROJECT/docs/roadmaps/zeta"/{backlog,analyzing,wip,blocked,done,abandoned}
cat > "$S35_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents: ["obi", "ka, tsu"]
EOF
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/ka, tsu/wip/ROADMAP-ka-tsu-wip.md" "wip" "ka tsu wip fixture (caso 8)"
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/obi/wip/ROADMAP-obi-wip.md" "wip" "obi wip fixture (declarado primeiro, discrimina ordem)"
write_roadmap_state_fixture "$S35_PROJECT/docs/roadmaps/zeta/wip/ROADMAP-zeta-wip.md" "wip" "zeta wip fixture (fora da lista configurada — só disco, union)"

S35_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        0\n   REQs        0  (0 Open · 0 Done · 0 Closed)\n   Roadmaps    3\n     backlog 0 · analyzing 0 · wip 3\n     blocked 0 · done 0 · abandoned 0\n\n⚙ WIP by Agent\n  [obi] WIP (1)\n    ROADMAP-obi-wip.md\n  [ka, tsu] WIP (1)\n    ROADMAP-ka-tsu-wip.md\n  [zeta] WIP (1)\n    ROADMAP-zeta-wip.md\n\n────────────────────────────────────────\n'

s35_go_out=$(cd "$S35_PROJECT" && "$T27_GO_BIN" status)$'\n'

if [[ "$s35_go_out" == "$S35_EXPECTED" ]]; then
  falsify_count_success
  echo "OK   [falsify/config-inline-comma-in-quotes/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/baseline-byte-identical-and-pinned]: esperava '$S35_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s35_go_out")" >&2
  falsify_fail_point
fi

# Diagnóstico estático e discriminante: byte-idêntico à mensagem da regra
# `agent_namespace_undeclared`, com a vírgula do nome do diretório em disco
# preservada — vem da UNIÃO (varredura de disco), não do parsing de `agents:`.
S35_KATSU_UNDECLARED='agent namespace "ka, tsu" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'
S35_OBI_UNDECLARED='agent namespace "obi" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'
S35_ZETA_UNDECLARED='agent namespace "zeta" exists in roadmap_dir but is not declared in agents: — add it to trackfw.yaml'

# --- braço de baseline: com os binários LIMPOS, `obi` e `ka, tsu`
# (declarados) nunca aparecem como não-declarados — só `zeta` (só-disco).
# Prova de não-vacuidade para os dois alvos, nos 3 CLIs: a ausência de
# obi/"ka, tsu" sozinha não distingue "validate rodou e a regra corretamente
# poupou os declarados" de "validate não rodou nada" (saída vazia engolida
# pelo `; true`) — a asserção de zeta PRESENTE é obrigatória, prova que o
# validate rodou, varreu o disco e a regra disparou de verdade.
s35_validate_go_out=$(cd "$S35_PROJECT" && "$T27_GO_BIN" validate 2>&1; true)
for pair in "go:$s35_validate_go_out"; do
  runtime="${pair%%:*}"
  out="${pair#*:}"
  if ! grep -qF "$S35_ZETA_UNDECLARED" <<<"$out"; then
    echo "FAIL [falsify/config-inline-comma-in-quotes/$runtime/agent-namespace-undeclared-baseline]: zeta (só-disco) deveria estar 'não declarado' no ciclo LIMPO e não está — validate pode não ter rodado (cenário vácuo)" >&2
    echo "  output: $(printf '%q' "$out")" >&2
    falsify_fail_point
  fi
  if grep -qF "$S35_KATSU_UNDECLARED" <<<"$out" || grep -qF "$S35_OBI_UNDECLARED" <<<"$out"; then
    echo "FAIL [falsify/config-inline-comma-in-quotes/$runtime/agent-namespace-undeclared-baseline]: obi ou 'ka, tsu' (declarados em agents:) já aparecem como não-declarados no ciclo LIMPO — o cenário seria vácuo" >&2
    echo "  output: $(printf '%q' "$out")" >&2
    falsify_fail_point
  fi
  falsify_count_success
  echo "OK   [falsify/config-inline-comma-in-quotes/$runtime/agent-namespace-undeclared-baseline]"
done

# --- braço de detecção: Go deixa de atribuir cfg.Agents a partir da lista --
# lida (RETARGET — ver comentário no topo do Cenário 35: splitTopLevelCommas
# não existe mais pós-yaml.v3; mesmo ponto usado no Cenário 34)
T35C_GO_MOD="$WORK/s35-corrupt-go"
mkdir -p "$T35C_GO_MOD/cmd" "$T35C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T35C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T35C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T35C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T35C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T35C_GO_MOD/internal/config/config.go" \
  $'\t\t\tcfg.Agents = items\n' \
  $'\t\t\t_ = items\n' \
  "s35-go"

T35C_GO_BIN="$WORK/s35-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T35C_GO_BIN")"
build_go_or_fail "setup-s35-go-corrupt-build" "$T35C_GO_MOD" "$T35C_GO_BIN"

s35c_validate_go_out=$(cd "$S35_PROJECT" && "$T35C_GO_BIN" validate 2>&1; true)
if grep -qF "$S35_KATSU_UNDECLARED" <<<"$s35c_validate_go_out" && grep -qF "$S35_OBI_UNDECLARED" <<<"$s35c_validate_go_out"; then
  falsify_count_success
  echo "OK   [falsify/config-inline-comma-in-quotes/go-detects-agents-discarded]"
else
  echo "FAIL [falsify/config-inline-comma-in-quotes/go-detects-agents-discarded]: cfg.Agents descartado, mas obi e/ou 'ka, tsu' não viraram 'não declarados' na violação agent_namespace_undeclared — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s35c_validate_go_out")" >&2
  falsify_fail_point
fi

# ---------------------------------------------------------------------------
# Cenário 36 — ML-2A do ROADMAP-2026-08-02-substituir-os-parsers-artesanais-
# de-config-por-biblioteca-yaml-nos-tres-clis: fidelidade textual de escalar
# YAML (AC3 do roadmap) — a normalização para string na fronteira, feita em
# normalizeNode (Go) / normalizeNode (Node) / _normalize_node (Python), não
# pode regredir para o valor JÁ TIPADO que cada biblioteca resolveria por
# padrão (int/bool/date), ou os 3 CLIs voltam a divergir por schema — a
# MESMA divergência de 3 vias medida no ML-0A (ver Wave 0 do roadmap):
#
#   entrada            Go (typed)      Node (typed)    Python (typed)
#   "010"              int 8           number 10       int 8
#   "2026-08-02"       time.Time       string (nao      date
#                                       converte)
#   "yes"              string (nao     string (nao      bool True
#                       coage)          coage)
#
# Os TRÊS valores do contrato (octal, data nua, "yes") são necessários na
# MESMA fixture — cada um cobre um CLI diferente, e nenhum par prova os 3:
#   - octal ("010") é o ÚNICO dos três que produz tipo não-string em Node
#     (number). Sem ele, uma regressão de normalização em Node passaria
#     despercebida pela fixture inteira (nem a data nem "yes" mudam de tipo
#     em Node — ver tabela acima).
#   - data nua ("2026-08-02") produz tipo não-string em Go E Python
#     (time.Time / date) — mas em Node o valor já chega como string mesmo
#     sem normalização (Node não tem resolver de data), então sozinha ela
#     não prova nada sobre Node.
#   - "yes" produz tipo não-string SÓ em Python (bool True, resolução YAML
#     1.1) — Go e `yaml` (Node) seguem o núcleo YAML 1.2 e não coagem
#     yes/no para booleano, então "yes" chega como string nos dois mesmo
#     sem normalização.
#
# Cada CLI usa o guard de tipo já existente (stringVal: `v.(string)` em Go,
# `typeof v === 'string'` em Node, `isinstance(v, str)` em Python) para só
# aceitar escalares vindos como string. Quando a normalização é removida (a
# corrupção abaixo faz normalizeNode devolver o valor TIPADO em vez do texto
# bruto), um valor que se tipifica como não-string faz o guard reprovar
# SILENCIOSAMENTE — a chave é descartada e o default do campo prevalece, sem
# erro. É exatamente esse efeito observável (queda para o default) que os
# três braços de detecção abaixo verificam.
#
# NÃO usa wip_limit/governance_mode/lenient_until (os campos citados
# literalmente no ADR): esses três são lidos, para o propósito de
# `trackfw validate`, por um leitor artesanal linha-a-linha SEPARADO
# (readWIPConfig/readGovernanceMode em internal/validator/validator.go, e os
# gêmeos em npm/src/validator/index.js e pypi/trackfw/validator.py) que a
# Wave 1 desta REQ NÃO tocou — ProjectConfig.WipLimit/GovernanceMode/
# LenientUntil (o caminho que passa pela biblioteca YAML) fica sombreado e
# nunca chega ao `validate` real. Uma fixture nessas chaves seria vácua para
# este cenário porque não exercitaria normalizeNode nenhuma. Ver achado
# registrado em vault/notes/config-legacy-line-reader-sombreia-yaml-lib-no-
# validate-2026-08-02.md. Em vez disso, a fixture usa `roadmap_dir`,
# `req_dir` e `adr_dirs` — os três são lidos via config.Load() (o caminho
# real da biblioteca YAML) e aparecem, cada um, no bloco Inventory de
# `trackfw status`, dando um sinal visível e determinístico por campo.
#
# Fixture: `roadmap_dir: 010`, `req_dir: 2026-08-02`, `adr_dirs: [yes]` —
# cada um aponta para um diretório NO DISCO com nome literal igual ao valor
# bruto ("010/", "2026-08-02/", "yes/"), cada um com exatamente 1 item
# (roadmap wip, REQ, ADR). Os caminhos DEFAULT (docs/roadmaps, docs/req,
# docs/adr) também têm conteúdo — mas em quantidade DIFERENTE (2 roadmaps,
# 2 REQs, 2 ADRs) — para que, se a corrupção fizer o parser cair no default,
# a divergência apareça como um número POSITIVO diferente do pinado, nunca
# como zero-por-coincidência (mesma lição do Cenário 35 e de
# vault/notes/falsificacao-fixture-vacua-contra-reversao-total-vs-parcial-
# 2026-08-02.md).
#
# Medido empiricamente (não apenas deduzido) contra os binários reais antes
# de fechar o cenário: a matriz de divergência por CLI corrompido é
#   - Go corrompido:     REQs 1→2, Roadmaps 1→2 (backlog 0→1, wip 1→1);
#                        ADRs PERMANECE 1 (Go não diverge em "yes")
#   - Node corrompido:   Roadmaps 1→2 (backlog 0→1); ADRs e REQs PERMANECEM
#                        1 (Node não diverge em data nua nem em "yes")
#   - Python corrompido: ADRs 1→0 (adr_dirs vira lista vazia, não cai no
#     default — stringList filtra o item não-string e devolve [] "presente
#     e vazio", contrato herdado do fix de lista inline), REQs 1→2,
#     Roadmaps 1→2
# Isso prova, por CLI, exatamente a matriz de discriminação do contrato:
# Node só diverge por causa do octal; Go e Python divergem por causa da
# data; só Python diverge por causa do "yes".
#
# Corrompe a IMPLEMENTAÇÃO (o branch de escalar de normalizeNode/
# _normalize_node), nunca a asserção — mesmo padrão dos cenários anteriores.
# ---------------------------------------------------------------------------

S36_PROJECT="$WORK/s36-config-schema-discriminant-project"
mkdir -p "$S36_PROJECT/docs/adr" "$S36_PROJECT/docs/req"
mkdir -p "$S36_PROJECT/docs/roadmaps"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S36_PROJECT/010"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$S36_PROJECT/2026-08-02"
mkdir -p "$S36_PROJECT/yes"
cat > "$S36_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
roadmap_dir: 010
req_dir: 2026-08-02
adr_dirs:
  - yes
EOF
write_roadmap_state_fixture "$S36_PROJECT/010/wip/ROADMAP-s36-custom-wip.md" "wip" "s36 custom wip fixture"
write_req_status_fixture "$S36_PROJECT/2026-08-02/REQ-s36-custom.md" "Open" "s36 custom req fixture"
write_adr_status_fixture "$S36_PROJECT/yes/ADR-s36-custom.md" "Accepted"

# Conteúdo diferente (em quantidade) nos caminhos DEFAULT — ver comentário
# acima sobre por que isso é necessário para não mascarar a corrupção.
write_roadmap_state_fixture "$S36_PROJECT/docs/roadmaps/wip/ROADMAP-s36-default-wip.md" "wip" "s36 default wip fixture"
write_roadmap_state_fixture "$S36_PROJECT/docs/roadmaps/backlog/ROADMAP-s36-default-backlog.md" "backlog" "s36 default backlog fixture"
write_req_status_fixture "$S36_PROJECT/docs/req/REQ-s36-default-1.md" "Open" "s36 default req 1"
write_req_status_fixture "$S36_PROJECT/docs/req/REQ-s36-default-2.md" "Open" "s36 default req 2"
write_adr_status_fixture "$S36_PROJECT/docs/adr/ADR-s36-default-1.md" "Accepted"
write_adr_status_fixture "$S36_PROJECT/docs/adr/ADR-s36-default-2.md" "Accepted"

S36_EXPECTED=$'── trackfw status ──────────────────────\n\n📊 Inventory\n   ADRs        1\n   REQs        1  (1 Open · 0 Done · 0 Closed)\n   Roadmaps    1\n     backlog 0 · analyzing 0 · wip 1\n     blocked 0 · done 0 · abandoned 0\n\n🔄 WIP (1)\n   ROADMAP-s36-custom-wip.md\n\n❌ Blocked (0)\n\n✅ Done (last 5)\n\n────────────────────────────────────────\n'

s36_go_out=$(cd "$S36_PROJECT" && "$T27_GO_BIN" status)$'\n'

if [[ "$s36_go_out" == "$S36_EXPECTED" ]]; then
  falsify_count_success
  echo "OK   [falsify/config-schema-discriminant/baseline-byte-identical-and-pinned]"
else
  echo "FAIL [falsify/config-schema-discriminant/baseline-byte-identical-and-pinned]: esperava '$S36_EXPECTED' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s36_go_out")" >&2
  falsify_fail_point
fi

# --- braço de detecção: Go devolve o valor TIPADO em vez do texto bruto ----
# (octal "010" -> int 8: roadmap_dir cai no default; data nua -> time.Time:
# req_dir cai no default; "yes" -> string "yes": adr_dirs NÃO diverge — Go
# não coage yes/no para booleano)
T36C_GO_MOD="$WORK/s36-corrupt-go"
mkdir -p "$T36C_GO_MOD/cmd" "$T36C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T36C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T36C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T36C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T36C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T36C_GO_MOD/internal/config/config.go" \
  $'\tcase yaml.ScalarNode:\n\t\treturn n.Value\n' \
  $'\tcase yaml.ScalarNode:\n\t\tvar typed interface{}\n\t\tn.Decode(&typed)\n\t\treturn typed\n' \
  "s36-go"

T36C_GO_BIN="$WORK/s36-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T36C_GO_BIN")"
build_go_or_fail "setup-s36-go-corrupt-build" "$T36C_GO_MOD" "$T36C_GO_BIN"

s36c_go_out=$(cd "$S36_PROJECT" && "$T36C_GO_BIN" status)$'\n'
if [[ "$s36c_go_out" == "$S36_EXPECTED" ]]; then
  echo "FAIL [falsify/config-schema-discriminant/go-detects-typed-scalar-regression]: normalizeNode revertido mas a comparação continuou passando (checagem vácua)" >&2
  falsify_fail_point
fi
if grep -qF "ADRs        1" <<<"$s36c_go_out" && grep -qF "REQs        2" <<<"$s36c_go_out" && grep -qF "backlog 1" <<<"$s36c_go_out"; then
  falsify_count_success
  echo "OK   [falsify/config-schema-discriminant/go-detects-typed-scalar-regression]"
else
  echo "FAIL [falsify/config-schema-discriminant/go-detects-typed-scalar-regression]: saída corrompida diverge do pinado, mas não no padrão esperado (ADRs deveria permanecer 1; REQs e Roadmaps deveriam cair para o default) — diagnóstico pelo motivo errado" >&2
  echo "  output: $(printf '%q' "$s36c_go_out")" >&2
  falsify_fail_point
fi
# ---------------------------------------------------------------------------
# Cenário 37 — ML-2A: caminho de erro de config malformada — os 3 CLIs
# imprimem a MESMA mensagem em stderr (MalformedConfigMessage /
# MALFORMED_CONFIG_MESSAGE) e saem com o MESMO exit code (1) quando
# trackfw.yaml existe mas não é YAML válido. Comportamento NOVO desta REQ
# (ML-1B, addendum ao ML-1A) — nada em CI garantia isso antes deste cenário;
# as suítes unitárias por CLI provam a mensagem isoladamente, mas nenhum
# gate cruzava os 3 binários reais contra a MESMA fixture malformada.
#
# Fixture: sequência de fluxo (`[...]`) aberta e nunca fechada — inválida
# nas 3 bibliotecas (confirmado empiricamente: gopkg.in/yaml.v3, `yaml` 2.x
# e PyYAML rejeitam as 3, cada uma com sua própria mensagem nativa
# diferente — exatamente por isso a mensagem trackfw é estática, não
# derivada do erro da biblioteca, ver comentário de MalformedConfigMessage
# em internal/config/config.go).
#
# Braço de detecção: só Go (a checagem de erro de sintaxe do Go, além de ser
# a mais recente/complexa das 3 — soma o probe de yaml.Unmarshal com
# hasMultipleDocuments — é também o ponto onde uma regressão de "parei de
# tratar erro de sintaxe como fatal" é mais fácil de introduzir sem querer
# ao mexer no probe). Não repete o braço nos 3 CLIs: o mecanismo (parse ->
# erro -> flag "malformed" -> stderr fatal + exit 1) é estruturalmente
# idêntico nos 3 (mesmo comentário-fonte, ver MALFORMED_CONFIG_MESSAGE nos 3
# arquivos), e o objetivo deste cenário é provar que o gate cruzado existe e
# pega uma regressão — não re-provar a suíte unitária de cada CLI.
# ---------------------------------------------------------------------------

S37_PROJECT="$WORK/s37-config-malformed-project"
mkdir -p "$S37_PROJECT/docs/adr" "$S37_PROJECT/docs/req" \
  "$S37_PROJECT/docs/roadmaps"/{backlog,analyzing,wip,blocked,done,abandoned}
printf 'agents: [a, b\ngovernance_mode: strict\n' > "$S37_PROJECT/trackfw.yaml"

S37_EXPECTED_STDERR='trackfw: erro ao carregar "trackfw.yaml": YAML malformado. Corrija a sintaxe do arquivo antes de continuar.'

set +e
s37_go_out=$(cd "$S37_PROJECT" && "$T27_GO_BIN" status 2>&1)
s37_go_status=$?
set -e

if [[ "$s37_go_status" -eq 1 && "$s37_go_out" == "$S37_EXPECTED_STDERR" ]]; then
  falsify_count_success
  echo "OK   [falsify/config-malformed-error-path/baseline-byte-identical-exit-1-3-clis]"
else
  echo "FAIL [falsify/config-malformed-error-path/baseline-byte-identical-exit-1-3-clis]: esperava stderr '$S37_EXPECTED_STDERR' e exit 1 nos 3 CLIs" >&2
  echo "  go:     status=$s37_go_status out=$(printf '%q' "$s37_go_out")" >&2
  falsify_fail_point
fi

# --- braço de detecção: Go deixa de tratar erro de sintaxe/multi-documento
# como fatal (probe sempre "ok") ------------------------------------------
T37C_GO_MOD="$WORK/s37-corrupt-go"
mkdir -p "$T37C_GO_MOD/cmd" "$T37C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T37C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T37C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T37C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T37C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/config/config.go" "$T37C_GO_MOD/internal/config/config.go" \
  $'\t\tif err := yaml.Unmarshal(data, &probe); err != nil || hasMultipleDocuments(data) {\n' \
  $'\t\tif err := yaml.Unmarshal(data, &probe); err == nil && false {\n' \
  "s37-go"

T37C_GO_BIN="$WORK/s37-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T37C_GO_BIN")"
build_go_or_fail "setup-s37-go-corrupt-build" "$T37C_GO_MOD" "$T37C_GO_BIN"

set +e
s37c_go_out=$(cd "$S37_PROJECT" && "$T37C_GO_BIN" status 2>&1)
s37c_go_status=$?
set -e
if [[ "$s37c_go_status" -eq 1 && "$s37c_go_out" == "$S37_EXPECTED_STDERR" ]]; then
  echo "FAIL [falsify/config-malformed-error-path/go-detects-fatal-check-removed]: checagem de erro de sintaxe revertida mas a comparação continuou passando (checagem vácua)" >&2
  falsify_fail_point
fi
if [[ "$s37c_go_status" -eq 0 ]]; then
  falsify_count_success
  echo "OK   [falsify/config-malformed-error-path/go-detects-fatal-check-removed]"
else
  echo "FAIL [falsify/config-malformed-error-path/go-detects-fatal-check-removed]: saída corrompida diverge do pinado, mas o exit não caiu para 0 — diagnóstico pelo motivo errado" >&2
  echo "  status=$s37c_go_status output: $(printf '%q' "$s37c_go_out")" >&2
  falsify_fail_point
fi

# ---------------------------------------------------------------------------
# Cenário 38 — wipConfigFrom (e equivalentes _wip_config_from/JS) volta a ler
# trackfw.yaml artesanalmente em vez de consumir o cfg já normalizado por
# config.Load() — regressão descoberta na auditoria do ML-3A (elimina os
# leitores readWIPConfig/readGovernanceMode em 74d70ee). Nenhum cenário deste
# harness fixava essa regressão: o teste unitário existente
# (TestValidateWIPLimit_Global_HighLimit) usa `wip_limit: 3` SEM aspas — valor
# que um leitor artesanal (Sscanf %d / parseInt / int()) lê corretamente,
# igual à biblioteca YAML. Sem aspas o cenário é vácuo: os dois caminhos
# concordam e nenhuma regressão é detectada.
#
# Fixture discriminante: `wip_limit: "3"` — COM aspas. Um leitor artesanal
# falha ao interpretar o valor citado (Sscanf/parseInt/int() encontram `"3"`,
# não `3`) e cai no default 1; a biblioteca YAML resolve o escalar tipado
# normalmente para 3.
#
#   - baseline: projeto com wip_limit: "3" (citado) e 4 roadmaps em wip/ → os
#     3 CLIs devem reportar o warning "4 roadmaps in wip/ (limit: 3) —
#     consider focusing" (cfg.WipLimit == 3, valor lido pela biblioteca).
#   - detecção: reintroduz, em cada CLI, exatamente o padrão do
#     readWIPConfig/wipConfigFrom eliminado por 74d70ee — releitura artesanal
#     de trackfw.yaml em vez de consumir o cfg já carregado — e prova que a
#     saída volta a "(limit: 1)" nos 3 CLIs.
#
# Corrompe a IMPLEMENTAÇÃO (wipConfigFrom/_wip_config_from e equivalente
# Node), nunca a asserção — mesmo padrão dos cenários anteriores.
# ---------------------------------------------------------------------------


S38_PROJECT="$WORK/s38-wip-limit-quoted-project"
scaffold_adr_req_project "$S38_PROJECT"
cat > "$S38_PROJECT/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
wip_limit: "3"
wip_by_squad: false
EOF
for n in 1 2 3 4; do
  write_wip_roadmap_fixture "$S38_PROJECT/docs/roadmaps/wip/ROADMAP-wip-$n.md" "wip fixture $n"
done

S38_EXPECTED_WARNING='4 roadmaps in wip/ (limit: 3) — consider focusing'
S38_REGRESSED_WARNING='4 roadmaps in wip/ (limit: 1) — consider focusing'

# --- prova positiva: os 3 CLIs, com a fixture citada -------------------------
set +e
s38_go_out=$(cd "$S38_PROJECT" && "$T27_GO_BIN" validate 2>&1)
set -e

if grep -qF "$S38_EXPECTED_WARNING" <<<"$s38_go_out"; then
  falsify_count_success
  echo "OK   [falsify/wip-limit-quoted/baseline-3-clis]"
else
  echo "FAIL [falsify/wip-limit-quoted/baseline-3-clis]: esperava '$S38_EXPECTED_WARNING' nos 3 CLIs" >&2
  echo "  go:     $(printf '%q' "$s38_go_out")" >&2
  falsify_fail_point
fi

# --- Go: prova de detecção (wipConfigFrom volta a ler trackfw.yaml direto) --
GO_S38_OLD=$'func wipConfigFrom(cfg config.ProjectConfig) WIPConfig {\n\treturn WIPConfig{Limit: cfg.WipLimit, BySquad: cfg.WipBySquad}\n}'
GO_S38_NEW=$'func wipConfigFrom(cfg config.ProjectConfig) WIPConfig {\n\twc := WIPConfig{Limit: 1, BySquad: cfg.WipBySquad}\n\tcontent, err := os.ReadFile("trackfw.yaml")\n\tif err != nil {\n\t\treturn wc\n\t}\n\tfor _, line := range strings.Split(string(content), "\\n") {\n\t\tline = strings.TrimSpace(line)\n\t\tif strings.HasPrefix(line, "wip_limit:") {\n\t\t\tval := strings.TrimSpace(strings.TrimPrefix(line, "wip_limit:"))\n\t\t\tfields := strings.Fields(val)\n\t\t\tif len(fields) > 0 {\n\t\t\t\tvar n int\n\t\t\t\tif _, err := fmt.Sscanf(fields[0], "%d", &n); err == nil && n > 0 {\n\t\t\t\t\twc.Limit = n\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\treturn wc\n}'

T38C_GO_MOD="$WORK/s38-corrupt-go"
mkdir -p "$T38C_GO_MOD/cmd" "$T38C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T38C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T38C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T38C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T38C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T38C_GO_MOD/internal/validator/validator.go" \
  "$GO_S38_OLD" "$GO_S38_NEW" "s38-go"

T38C_GO_BIN="$WORK/s38-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T38C_GO_BIN")"
build_go_or_fail "setup-s38-go-corrupt-build" "$T38C_GO_MOD" "$T38C_GO_BIN"

set +e
s38c_go_out=$(cd "$S38_PROJECT" && "$T38C_GO_BIN" validate 2>&1)
set -e
if grep -qF "$S38_REGRESSED_WARNING" <<<"$s38c_go_out" && ! grep -qF "$S38_EXPECTED_WARNING" <<<"$s38c_go_out"; then
  falsify_count_success
  echo "OK   [falsify/wip-limit-quoted/go-detects-artisanal-reader-reintroduced]"
else
  echo "FAIL [falsify/wip-limit-quoted/go-detects-artisanal-reader-reintroduced]: leitor artesanal reintroduzido mas a saída não voltou a '(limit: 1)' — checagem vácua" >&2
  echo "  output: $(printf '%q' "$s38c_go_out")" >&2
  falsify_fail_point
fi


# ---------------------------------------------------------------------------
# Cenários 39/40/41 — ML-3A (REQ-2026-08-02-unificar-a-leitura-do-trackfw-
# yaml-em-um-unico-carregador-nos-tres-clis): `trackfw update` volta a ler
# hooks/ci/backend/frontend/pkg_manager com um scanner artesanal em vez de
# consumir o namespace `Update` já resolvido pelo carregador único
# (config.Load()/projectConfig.load()/project_config.load() — ver
# internal/generators/update.go:loadUpdateConfig,
# npm/src/commands/update.js:loadUpdateConfig,
# pypi/trackfw/commands/update.py:_load_update_config).
#
# Fixture discriminante (AC4/AC7 — chave aninhada homônima, o candidato mais
# forte da REQ): `hooks: lefthook` na raiz do YAML e uma seção NÃO relacionada
# com uma chave `hooks:` homônima aninhada por baixo dela.
#
#   hooks: lefthook
#   legacy_project_settings:
#     hooks: husky
#
# O carregador único respeita a estrutura do mapeamento — só a chave `hooks`
# da RAIZ é lida (ProjectConfig.Update.Hooks == "lefthook"). Um scanner
# artesanal reintroduzido (mesmo padrão eliminado pelo ML-2A: itera linha a
# linha, casa o prefixo `hooks:` em QUALQUER indentação, ignora nesting)
# sobrescreve o valor a cada ocorrência — a última linha que casa vence — e
# termina com "husky" em vez de "lefthook".
#
# Guarda de vivacidade: o efeito não é só "o arquivo lido mudou" — é
# observável no comportamento de `updateHooksSurgical`/`_update_hooks_surgical`.
# Go e Python GRAVAM o arquivo incondicionalmente e imprimem "✓ <arquivo> —
# trackfw[-]validate injetado" com hooks=lefthook (correto) ou hooks=husky
# (regredido). Node.js, na invocação bare (sem --install-missing), reporta o
# alvo `git-hooks` como `missing` — a escrita real fica atrás de
# --install-missing (runFileTarget não chama `apply` quando o arquivo ainda
# não existe e installMissing é false) — mas o CAMPO `path` do relatório
# ainda diverge (`lefthook.yml` vs `.husky/pre-commit`), então o sinal
# continua genuíno e não-vácuo: é o mesmo `cfg.hooks` resolvido pelo scanner
# que decide qual nome aparece, escrito ou não. Os três braços verificam qual
# dos dois nomes aparece na saída de `trackfw update` bare (sem flags — ver
# constraint da barreira ML-2A/Hefesto para o braço Python, que possui um
# segundo caminho, `_run_project`, atrás de --dry-run/--json/--targets/
# --install-missing, que NUNCA chama o carregador — fora do escopo desta REQ).
#
# Duas provas foram feitas para cada CLI, complementares: (1) corrupção de
# uma CÓPIA isolada em $WORK (os braços de detecção abaixo, que rodam sempre
# dentro da suíte) e (2) corrupção do ARQUIVO REAL do working tree, rodada
# manualmente uma única vez durante o desenvolvimento deste ML para confirmar
# que os braços de baseline (que consomem `$T27_GO_BIN`/`$ROOT_DIR/npm/bin/
# trackfw`/`PYTHONPATH=$ROOT_DIR/pypi` — código real, não corrompido) de fato
# flipam se alguém regredir o código real — não só a cópia. Revertida
# (`git checkout --`) e confirmada limpa (`git status --porcelain`) em
# seguida; não faz parte da execução normal do gate (custaria 3 rebuilds/
# reverts a cada corrida). Uma corrupção real também dispara um segundo
# mecanismo independente do `corrupt_literal`/`assert_fails_with` normal: se
# o literal-alvo mudar de forma (refactor), `corrupt_literal` falha primeiro
# com "expected exactly 1 occurrence… got 0" — sintoma de setup, não de
# veredito do gate (ver vault/notes/cenarios-de-falsificacao-quebram-em-
# refactor-do-alvo-2026-08-02.md).
#
# Corrompe a IMPLEMENTAÇÃO (loadUpdateConfig/_load_update_config), nunca a
# asserção — mesmo padrão dos cenários anteriores.
# ---------------------------------------------------------------------------


S39_EXPECTED_MSG='✓ lefthook.yml — trackfw-validate injetado'
S39_REGRESSED_MSG='✓ .husky/pre-commit — trackfw validate injetado'

# --- Cenário 39 — Go --------------------------------------------------------

S39_BASE="$WORK/s39-go-baseline"
mkdir -p "$S39_BASE"
write_update_hooks_discriminant_fixture "$S39_BASE/trackfw.yaml"
set +e
s39_base_out=$(cd "$S39_BASE" && "$T27_GO_BIN" update 2>&1)
s39_base_status=$?
set -e
if [[ $s39_base_status -eq 0 ]] \
    && grep -qF "$S39_EXPECTED_MSG" <<<"$s39_base_out" \
    && ! grep -qF "$S39_REGRESSED_MSG" <<<"$s39_base_out"; then
  falsify_count_success
  echo "OK   [falsify/update-config-loader/go-baseline]"
else
  echo "FAIL [falsify/update-config-loader/go-baseline]: esperava exit 0 e '$S39_EXPECTED_MSG'" >&2
  echo "  status: $s39_base_status" >&2
  echo "  output: $(printf '%q' "$s39_base_out")" >&2
  falsify_fail_point
fi

GO_S39_OLD=$'func loadUpdateConfig() Config {\n\tu := config.Load().Update\n\treturn Config{\n\t\tHooks:      u.Hooks,\n\t\tCI:         u.CI,\n\t\tBackend:    u.Backend,\n\t\tFrontend:   u.Frontend,\n\t\tPkgManager: u.PkgManager,\n\t}\n}'
GO_S39_NEW=$'func loadUpdateConfig() Config {\n\t// [falsified] artisanal line-by-line scanner reintroduced — matches the "hooks:" prefix at\n\t// ANY indentation and keeps overwriting, so the LAST matching line wins regardless of nesting.\n\t// config.Load() is still invoked (kept referenced) but its Update namespace is discarded.\n\t_ = config.Load()\n\tdata, err := os.ReadFile("trackfw.yaml")\n\tif err != nil {\n\t\treturn Config{}\n\t}\n\tcfg := Config{}\n\tfor _, line := range strings.Split(string(data), "\\n") {\n\t\tline = strings.TrimSpace(line)\n\t\tif strings.HasPrefix(line, "#") {\n\t\t\tcontinue\n\t\t}\n\t\tidx := strings.Index(line, ":")\n\t\tif idx < 0 {\n\t\t\tcontinue\n\t\t}\n\t\tkey := strings.TrimSpace(line[:idx])\n\t\tval := strings.TrimSpace(line[idx+1:])\n\t\tswitch key {\n\t\tcase "hooks":\n\t\t\tcfg.Hooks = val\n\t\tcase "ci":\n\t\t\tcfg.CI = val\n\t\tcase "backend":\n\t\t\tcfg.Backend = val\n\t\tcase "frontend":\n\t\t\tcfg.Frontend = val\n\t\tcase "pkg_manager":\n\t\t\tcfg.PkgManager = val\n\t\t}\n\t}\n\treturn cfg\n}'

T39C_GO_MOD="$WORK/s39-corrupt-go"
mkdir -p "$T39C_GO_MOD/cmd" "$T39C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T39C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T39C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T39C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T39C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/update.go" "$T39C_GO_MOD/internal/generators/update.go" \
  "$GO_S39_OLD" "$GO_S39_NEW" "s39-go"

T39C_GO_BIN="$WORK/s39-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T39C_GO_BIN")"
build_go_or_fail "setup-s39-go-corrupt-build" "$T39C_GO_MOD" "$T39C_GO_BIN"

S39C="$WORK/s39-go-corrupt"
mkdir -p "$S39C"
write_update_hooks_discriminant_fixture "$S39C/trackfw.yaml"
set +e
s39c_out=$(cd "$S39C" && "$T39C_GO_BIN" update 2>&1)
set -e
if grep -qF "$S39_REGRESSED_MSG" <<<"$s39c_out" && ! grep -qF "$S39_EXPECTED_MSG" <<<"$s39c_out"; then
  falsify_count_success
  echo "OK   [falsify/update-config-loader/go-detects-artisanal-scanner-reintroduced]"
else
  echo "FAIL [falsify/update-config-loader/go-detects-artisanal-scanner-reintroduced]: scanner artesanal reintroduzido mas a saída não regrediu para hooks=husky — checagem vácua" >&2
  echo "  output: $(printf '%q' "$s39c_out")" >&2
  falsify_fail_point
fi

# ---------------------------------------------------------------------------
# Cenário 47 — internal/validator: prova de não-vacuidade da regra
#              "credential_guard_hook_resolvable" (ROADMAP-2026-08-12-
#              mitigacao-do-fail-open-do-credential-guard-..., ML-1A, Apolo;
#              este cenário é o ML-2A, Ártemis) — a regra ACUSA quando existe
#              hook de credential-guard de PROJETO registrado (aqui,
#              .claude/settings.json) e o script referenciado não existe, e
#              NÃO acusa quando o script está presente e executável.
#
# ÂNCORA DE MANUTENÇÃO / RETARGET: $S47_MSG_MISSING abaixo é um TRECHO do
# literal exato emitido por validateCredentialGuardHookResolvable em
# internal/validator/validator_credential_guard.go:163-166 ("... but the
# script does not exist — run `trackfw update` to regenerate it"). Se essa
# mensagem mudar de forma (wording, ordem dos campos, ou deixar de citar
# "trackfw update"), reaponte $S47_MSG_MISSING para o novo literal — os
# equivalentes Node (npm/src/validator/index.js, função
# validateCredentialGuardHookResolvable, ~linha 1231) e Python
# (pypi/trackfw/validator.py:1447, validate_credential_guard_hook_resolvable)
# precisam mudar a mensagem junto, por regra de paridade (ADR-2026-08-05) —
# mas este cenário, por desenho, exercita só o CLI Go (ver abaixo).
#
# Por que só o CLI Go: o roadmap (ML-2A) permite testar um subconjunto dos 3
# stacks quando o cenário não precisa exercitar os outros para provar
# não-vacuidade. Os testes unitários dos 3 stacks já cobrem paridade de
# comportamento (internal/validator/validator_credential_guard_test.go;
# pypi/tests/test_validator.py:1001-1122; equivalente em npm/src/validator
# via npm test) — este cenário é a prova P4 (black-box, via `trackfw
# validate` de verdade) de que a regra Go não é vácua, o que já é suficiente
# para satisfazer o critério de aceite do ML.
#
# Por que não precisa isolar $HOME (ao contrário do Cenário 46): esta regra
# só lê arquivos de hook de PROJETO sob a raiz do projeto corrente
# (os.Getwd(), ver validateCredentialGuardHookResolvable) — nunca consulta
# $HOME ou o guard GLOBAL. Não existe vetor de vazamento ambiental a
# discriminar aqui, diferente do dedup globalCredentialGuardInstalled*() que
# o Cenário 46 testa.
#
# Braço autodiscriminante: a asserção de detecção usa assert_fails_with com
# o literal EXATO da mensagem desta regra, não um "saiu != 0" genérico.
# `grep -rn` em internal/validator/*.go confirma que nenhuma outra regra
# emite essa frase — então este braço só pode ser satisfeito pela regra sob
# teste disparando, nunca por uma violação incidental de outra regra. Reforço
# adicional: o fixture (scaffold_adr_req_project) é um projeto vazio sem
# docs/adr, docs/req ou docs/roadmaps/* — o mesmo fixture que o Cenário 29
# prova imprimir "✓ No violations found." byte-a-byte quando íntegro — então
# nenhuma OUTRA regra tem material para disparar neste projeto além da
# sabotagem deliberada (script ausente) que este cenário introduz.
#
# O que prova que a SABOTAGEM (não um acidente de fixture) é a causa: os dois
# braços usam s47_write_claude_guard_hook — o MESMO gerador de fixture — com
# um único delta entre eles (scripts/trackfw-credential-guard.sh criado e
# marcado +x no baseline, omitido na detecção). Isso encadeia os dois braços
# um no outro: o braço de detecção passando prova que a cadeia inteira está
# viva até o ponto de falha (JSON parseado → marcador
# "trackfw-credential-guard.sh" encontrado → prefixo $CLAUDE_PROJECT_DIR/
# resolvido → os.Stat alcançado e retornando "não existe") — se qualquer elo
# dessa cadeia estivesse quebrado (ex: typo no prefixo, marcador não
# reconhecido), a regra pularia o arquivo em silêncio e a DETECÇÃO teria
# falhado, não o contrário. O braço baseline então prova que o mesmo caminho
# de código, com o único delta "script presente e executável", fica em
# silêncio — atribuindo a diferença de resultado ao os.Stat, não a alguma
# outra causa incidental no fixture.
#
# Limite de cobertura conhecido (fora do escopo deste ML): a regra tem 2
# pontos de wiring em internal/validator/validator.go — applyRule (:418,
# usado por Validate(), o caminho de texto exercitado aqui) e
# applyRuleTagged (:604, usado por ValidateTagged()/`validate --json`). Este
# cenário e a prova de não-vacuidade abaixo cobrem só :418; uma regressão que
# remova a chamada em :604 sem tocar em :418 passaria por este gate em
# silêncio. Reportado a Zeus para decisão (ML novo ou aceitar o gap).
# ---------------------------------------------------------------------------

# ROADMAP-2026-08-17 ML-4B: the fixture must carry "type":"command" like the
# real writer (mergeClaudeHookArray, internal/generators/agentfiles.go) always
# emits -- credential_guard_hook_resolvable now treats a matched command
# WITHOUT that sibling field as a structurally malformed entry (see
# hookArrayHasCommand's ML-4B doc comment), not a validly-wired one, so a
# fixture missing it would make this scenario's own baseline arm fail for the
# wrong reason (masking the "does the script exist" check this cenário exists
# to prove) instead of exercising it.

S47_MSG_MISSING='but the script does not exist — run `trackfw update` to regenerate it'

# --- braço baseline: script presente e executável -> validate passa --------
T47_OK="$WORK/s47-script-present"
scaffold_adr_req_project "$T47_OK"
s47_write_claude_guard_hook "$T47_OK/.claude/settings.json"
mkdir -p "$T47_OK/scripts"
cat > "$T47_OK/scripts/trackfw-credential-guard.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$T47_OK/scripts/trackfw-credential-guard.sh"

set +e
s47ok_out=$(cd "$T47_OK" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s47ok_status=$?
set -e
if [[ $s47ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-hook-resolvable/baseline]: árvore íntegra (script presente e executável) deveria passar, saiu com $s47ok_status" >&2
  echo "  output: $s47ok_out" >&2
  falsify_fail_point
fi
# Nota: o modo texto do `validate` (exercitado aqui) nunca imprime o nome
# interno da regra ("credential_guard_hook_resolvable") — só a mensagem. Só
# `validate --json` exporia o rule key (ver RuleItem.Rule em
# internal/validator/result.go), caminho não coberto por este cenário (ver
# comentário "Limite de cobertura conhecido" acima). A asserção aqui checa
# apenas a AUSÊNCIA da mensagem desta regra, que é o que o modo texto pode
# provar.
if grep -qF "$S47_MSG_MISSING" <<<"$s47ok_out"; then
  echo "FAIL [falsify/credential-guard-hook-resolvable/baseline]: script presente e executável mas a regra disparou mesmo assim" >&2
  echo "  output: $s47ok_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-hook-resolvable/baseline]"

# --- braço detecção: script ausente -> validate acusa esta regra -----------
T47_MISSING="$WORK/s47-script-missing"
scaffold_adr_req_project "$T47_MISSING"
s47_write_claude_guard_hook "$T47_MISSING/.claude/settings.json"
# scripts/trackfw-credential-guard.sh deliberadamente OMITIDO — é a sabotagem.

assert_fails_with "credential-guard-hook-resolvable/detected" \
  "$S47_MSG_MISSING" \
  bash -c "cd '$T47_MISSING' && exec '$ROOT_DIR/bin/trackfw' validate"

# ---------------------------------------------------------------------------
# Cenário 49 — internal/validator: prova de não-vacuidade da regra
#              "credential_guard_script_integrity" (ROADMAP-2026-08-12-
#              deteccao-de-adulteracao-do-credential-guard-regra-de-validate,
#              ML-1A, Apolo; este cenário é o ML-2A, Ártemis) — a regra ACUSA
#              quando scripts/trackfw-credential-guard.sh diverge do template
#              que ESTE binário trackfw geraria, e NÃO acusa quando o
#              conteúdo é byte-idêntico ao template.
#
# ÂNCORA DE MANUTENÇÃO / RETARGET: $S49_MSG abaixo é um TRECHO do literal
# exato emitido por validateCredentialGuardScriptIntegrity em
# internal/validator/validator_credential_guard_integrity.go:53-57 ("%s
# content diverges from the template this version of trackfw generates — if
# you did not edit this file by hand, run `trackfw update` to regenerate
# it"). Se essa mensagem mudar de forma (wording, ordem dos campos, ou
# deixar de citar "content diverges from the template"), reaponte $S49_MSG
# para o novo literal — os equivalentes Node (npm/src/validator/index.js) e
# Python (pypi/trackfw/validator.py) precisam mudar junto, por regra de
# paridade, mas este cenário testa só o CLI Go (mesma justificativa do
# Cenário 47: testes unitários dos 3 stacks já cobrem paridade de
# comportamento; este cenário é a prova P4 black-box de que a regra Go não
# é vácua).
#
# Severidade: o default é "warning" (ADR-2026-08-12 Emenda 3 — o script não
# carrega marcador de versão, então a regra não discrimina drift legítimo de
# adulteração), e "warning" NÃO derruba o exit code de `trackfw validate`
# (internal/commands/validate.go: só violations viram Errorf; warnings só
# imprimem "⚠"). Os fixtures abaixo fixam `rules:
# credential_guard_script_integrity: error` para poder usar
# assert_fails_with (que exige exit != 0) — recomendação de Apolo/Zeus
# repassada no despacho deste ML; registrado aqui para não parecer
# inconsistente com o default real.
#
# Braço autodiscriminante: s49_write_fixture é o MESMO gerador para os dois
# fixtures principais (T49_OK/T49_BAD) — a única diferença entre eles é uma
# linha "# tampered..." apensada ao script DEPOIS de copiado do template
# real (gerado por uma execução isolada de `bin/trackfw discover --init` no
# início deste bloco, então byte-idêntico ao que ESTE binário produziria —
# não um literal reconstruído à mão que poderia divergir do binário sob
# teste). O braço baseline prova que o mesmo caminho de código, com o script
# intocado, fica em silêncio; o braço de detecção prova que o MESMO fixture
# com essa única linha a mais dispara — atribuindo a diferença de resultado
# ao conteúdo do script, não a alguma outra causa incidental: o fixture não
# tem .claude/settings.json, então credential_guard_hook_resolvable nunca
# tem material para disparar aqui, e nenhuma outra regra tem material no
# mesmo scaffold_adr_req_project que o Cenário 29 prova imprimir "✓ No
# violations found." byte-a-byte quando íntegro.
#
# Prova de não-vacuidade: T49_OFF é o MESMO fixture corrompido de T49_BAD,
# mas com `rules: credential_guard_script_integrity: off` em vez de `error`
# — o único delta é a severidade configurada, não o conteúdo do script.
# assert_would_now_fail roda EXATAMENTE o mesmo comando/critério que
# assert_fails_with usaria no braço de detecção (exit != 0 E mensagem
# presente) e exige que ele NÃO seja atendido aqui — provando que, com a
# regra desligada, o braço de detecção acima FALHARIA (não apenas que a
# mensagem some, que por si só provaria só que o knob `rules:` funciona, sem
# dizer nada sobre a asserção de detecção depender da regra). Não é preciso
# reconstruir bin/trackfw aqui: a sabotagem é inteiramente por config de
# fixture (`rules:`), nunca por edição de internal/validator/*.go — este ML
# não tem permissão de tocar internal/ (ver "Arquivos permitidos" no
# despacho).
#
# Limite de cobertura conhecido (mesmo do Cenário 47): a regra tem 2 pontos
# de wiring em internal/validator/validator.go — applyRule (usado por
# Validate(), o caminho de texto exercitado aqui) e applyRuleTagged (usado
# por ValidateTagged()/`validate --json`). Este cenário cobre só o primeiro;
# uma regressão isolada no segundo passaria por este gate em silêncio.
# ---------------------------------------------------------------------------
S49_MSG='content diverges from the template this version of trackfw generates'

S49_REF_DIR="$WORK/s49-ref"
mkdir -p "$S49_REF_DIR"
(cd "$S49_REF_DIR" && "$ROOT_DIR/bin/trackfw" discover --init </dev/null >/dev/null 2>&1)
S49_REF_SCRIPT="$S49_REF_DIR/scripts/trackfw-credential-guard.sh"
if [[ ! -s "$S49_REF_SCRIPT" ]]; then
  echo "FAIL [falsify/credential-guard-script-integrity/setup]: 'trackfw discover --init' não gerou scripts/trackfw-credential-guard.sh" >&2
  falsify_fail_point
fi


# --- braço baseline: script byte-idêntico ao template -> validate passa ----
T49_OK="$WORK/s49-script-identical"
s49_write_fixture "$T49_OK" error

set +e
s49ok_out=$(cd "$T49_OK" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s49ok_status=$?
set -e
if [[ $s49ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-script-integrity/baseline]: árvore íntegra (script byte-idêntico ao template) deveria passar, saiu com $s49ok_status" >&2
  echo "  output: $s49ok_out" >&2
  falsify_fail_point
fi
if grep -qF "$S49_MSG" <<<"$s49ok_out"; then
  echo "FAIL [falsify/credential-guard-script-integrity/baseline]: script íntegro mas a regra disparou mesmo assim" >&2
  echo "  output: $s49ok_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-script-integrity/baseline]"

# --- braço detecção: script corrompido -> validate acusa esta regra --------
T49_BAD="$WORK/s49-script-corrupted"
s49_write_fixture "$T49_BAD" error
printf '# tampered by check-gates-falsify.sh Cenario 49\n' >> "$T49_BAD/scripts/trackfw-credential-guard.sh"

assert_fails_with "credential-guard-script-integrity/detected" \
  "$S49_MSG" \
  bash -c "cd '$T49_BAD' && exec '$ROOT_DIR/bin/trackfw' validate"

# --- prova de não-vacuidade: mesma corrupção, regra desligada -> o braço de
# detecção FALHARIA (assert_would_now_fail exige exit==0 OU mensagem
# ausente; se o critério de assert_fails_with fosse satisfeito mesmo assim,
# reprova aqui) --------------------------------------------------------------
T49_OFF="$WORK/s49-script-corrupted-rule-off"
s49_write_fixture "$T49_OFF" off
printf '# tampered by check-gates-falsify.sh Cenario 49\n' >> "$T49_OFF/scripts/trackfw-credential-guard.sh"

assert_would_now_fail "credential-guard-script-integrity" \
  "$S49_MSG" \
  bash -c "cd '$T49_OFF' && exec '$ROOT_DIR/bin/trackfw' validate"

# ---------------------------------------------------------------------------
# Cenário 50 — internal/validator: prova de não-vacuidade da regra
#              "credential_guard_mode_downgrade" (ROADMAP-2026-08-12-
#              deteccao-de-adulteracao-do-credential-guard-regra-de-validate,
#              ML-1A, Apolo; este cenário é o ML-2A, Ártemis) — a regra ACUSA
#              quando credential_guard.mode era "block" no commit HEAD do
#              git e o trackfw.yaml em disco não resolve mais para "block",
#              e NÃO acusa quando disco e HEAD concordam.
#
# ÂNCORA DE MANUTENÇÃO / RETARGET: $S50_MSG abaixo é um TRECHO do literal
# exato emitido por credentialGuardModeDowngradeMessage em
# internal/validator/validator_credential_guard_integrity.go:174-177. Se
# essa mensagem mudar de forma (wording, ou deixar de citar "does not
# resolve to block"), reaponte $S50_MSG para o novo literal — Node
# (npm/src/validator/index.js) e Python (pypi/trackfw/validator.py) precisam
# mudar junto, por regra de paridade, mas este cenário testa só o CLI Go
# (mesma justificativa do Cenário 47/49: prova P4 black-box de
# não-vacuidade; paridade de comportamento já coberta pelos testes unitários
# dos 3 stacks). $S50_FULL_MSG é o literal COMPLETO (não só o trecho),
# usado pelo Cenário 52 abaixo como entrada de .trackfw-baseline.json — o
# filtro de baseline compara a mensagem inteira (validator.go:527), não uma
# substring.
#
# Severidade: o default já é "error" (credential_guard_mode_downgrade está
# deliberadamente AUSENTE de ruleDefaults em internal/validator/validator.go
# — cai no default de ruleSeverity) — diferente do Cenário 49, não precisa
# de override em `rules:` para usar assert_fails_with.
#
# Encanamento novo exigido por este cenário (achado de Apolo, repassado por
# Zeus no despacho): nenhum fixture existente em check-gates-falsify.sh
# fazia `git init`/commit antes deste — s50_commit_fixture é o primeiro a
# criar um repo git de verdade dentro de $WORK por cenário, necessário
# porque esta regra só lê `git show HEAD:./trackfw.yaml` (sem HEAD, fica em
# silêncio por desenho — não há como disparar sem essa âncora). Generalizado
# aqui (ML-2A) para aceitar o conteúdo do trackfw.yaml commitado como
# parâmetro — os Cenários 51/52/53 abaixo reusam o mesmo helper com HEADs
# diferentes.
#
# Braço autodiscriminante: s50_commit_fixture é o MESMO gerador para os dois
# fixtures principais (T50_OK/T50_BAD) — ambos commitam
# credential_guard.mode: block no HEAD via o MESMO conteúdo de trackfw.yaml
# (s50_yaml_content block). A ÚNICA diferença entre os dois braços é uma
# reescrita NÃO commitada do trackfw.yaml em disco depois do commit: o braço
# baseline não toca o disco (disco == HEAD, mode: block); o braço de
# detecção sobrescreve só o disco para mode: warn, sem novo commit (git
# status ficaria "dirty" — é exatamente o "relaxamento legítimo não
# commitado" que o ADR (Emenda 3) trata como o único falso positivo
# aceitável, e que esta mensagem converte no próprio rastro auditável: "if
# this was intentional, commit the change"). Isso atribui a diferença de
# resultado à divergência disco-vs-HEAD, não a alguma outra causa
# incidental — nenhuma outra regra tem material neste
# scaffold_adr_req_project além do credential_guard.mode adicionado (mesma
# garantia do Cenário 47/49: fixture base é o que o Cenário 29 prova "✓ No
# violations found." byte-a-byte).
#
# 🔴 ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard,
# ML-2A (Ártemis) — MECANISMO DE NÃO-VACUIDADE SUBSTITUÍDO (ADR Emenda 2):
# a prova original sabotava a regra escrevendo `rules:
# credential_guard_mode_downgrade: off` em disco SEM commit — exatamente o
# comportamento PRÉ-ADR (auto-silenciamento sem rastro) que o M4 (ML-1A)
# fecha. Com M4 em produção esse sabotage NÃO tem mais efeito nenhum: o
# HEAD (sem `rules:`) resolve para o default "error", que vence a
# comparação direcional "mais estrita entre HEAD e disco"
# (credentialGuardRuleSeverity, validator_credential_guard_integrity.go:252)
# mesmo com "off" em disco — então o braço de detecção continuaria
# disparando, e o braço antigo ficaria PERMANENTEMENTE vermelho. Isso não é
# regressão: é o gate provando o próprio bug que este ADR corrige (ver
# vault/notes/scenario-50-non-vacuity-obsoleta-pelo-anchoring-no-head-2026-08-12.md).
#
# Substituído por um sabotage que CONTINUA funcionando por desenho: `rules:
# credential_guard_mode_downgrade: off` COMMITADO junto com mode: block no
# MESMO commit de HEAD — o "desligamento legítimo" do ADR §Decision point 5
# (mesmo padrão de
# TestCredentialGuardModeDowngrade_ConfiguravelViaRules/off_commitado em
# validator_credential_guard_integrity_test.go:356-371: HEAD e disco
# concordam em "off", então "mais estrita entre HEAD e disco" resolve para
# "off" e a regra silencia de verdade). T50_OFF commita mode: block +
# rules: ...: off juntos e depois baixa SÓ o mode para "warn" em disco (sem
# novo commit, rules: off permanece em disco também — nenhuma outra
# variável muda). Este ML não tem permissão de editar
# internal/validator/*.go (ver "Arquivos permitidos" no despacho), então
# `_ = credentialGuardModeMsgs` (o outro sabotage sugerido, usado por Zeus
# em auditoria) não é uma opção AQUI — só o "rules: off commitado" fica
# disponível dentro do escopo deste ML.
#
# Limite de cobertura conhecido (mesmo do Cenário 47/49): cobre só o wiring
# applyRule (Validate()/texto), não applyRuleTagged (ValidateTagged()/
# `validate --json`) em internal/validator/validator.go.
# ---------------------------------------------------------------------------
S50_MSG='current file does not resolve to block'
S50_FULL_MSG='trackfw.yaml sets credential_guard.mode: block at the git HEAD commit, but the current file does not resolve to block — if this was intentional, commit the change; otherwise investigate before treating the credential guard as active'

# --- braço baseline: disco concorda com HEAD (mode: block) -> validate passa
T50_OK="$WORK/s50-mode-matches-head"
s50_commit_fixture "$T50_OK" "$(s50_yaml_content block)" \
  "trackfw.yaml with credential_guard.mode: block"

set +e
s50ok_out=$(cd "$T50_OK" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s50ok_status=$?
set -e
if [[ $s50ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-mode-downgrade/baseline]: disco == HEAD (mode: block) deveria passar, saiu com $s50ok_status" >&2
  echo "  output: $s50ok_out" >&2
  falsify_fail_point
fi
if grep -qF "$S50_MSG" <<<"$s50ok_out"; then
  echo "FAIL [falsify/credential-guard-mode-downgrade/baseline]: disco == HEAD mas a regra disparou mesmo assim" >&2
  echo "  output: $s50ok_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-mode-downgrade/baseline]"

# --- braço detecção: disco diverge do HEAD (mode: warn, não commitado) -----
T50_BAD="$WORK/s50-mode-downgraded"
s50_commit_fixture "$T50_BAD" "$(s50_yaml_content block)" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn > "$T50_BAD/trackfw.yaml"

assert_fails_with "credential-guard-mode-downgrade/detected" \
  "$S50_MSG" \
  bash -c "cd '$T50_BAD' && exec '$ROOT_DIR/bin/trackfw' validate"

# --- prova de não-vacuidade (mecanismo NOVO, ML-2A): mesma divergência de
# mode, mas HEAD commita `rules: credential_guard_mode_downgrade: off`
# JUNTO com mode: block — "mais estrita entre HEAD e disco" resolve para
# "off" (ambos concordam) e a regra silencia de verdade. Isso prova que o
# braço de detecção acima (T50_BAD, HEAD SEM rules: off) depende
# genuinamente da regra estar ativa — não apenas que o knob `rules:`
# funciona (esse era o furo do mecanismo antigo). Helper trocado de
# assert_would_now_fail (usado pelo mecanismo antigo, e ainda usado pelo
# Cenário 49) para assert_lacks_pattern: aqui o fixture representa
# comportamento CORRETO de produção (desligamento legítimo commitado
# silenciando de verdade), não uma árvore sabotada por config — não faz
# sentido pedir que o critério de assert_fails_with "não seja atendido"
# (disjunção: exit==0 OU mensagem ausente) quando o esperado é exit==0 E
# mensagem ausente ao mesmo tempo. assert_lacks_pattern exige as DUAS
# (conjunção) — critério estritamente mais forte, não uma divergência de
# padrão sem motivo. NÃO reverter para assert_would_now_fail aqui: o
# `rules: ...: off` deste braço está COMMITADO (ver acima), diferente do
# uso de assert_would_now_fail no Cenário 49, onde o `off` é só em disco. -
T50_OFF="$WORK/s50-mode-downgraded-rule-off-commitado"
s50_commit_fixture "$T50_OFF" "$(s50_yaml_content block off)" \
  "trackfw.yaml with credential_guard.mode: block and rules: credential_guard_mode_downgrade: off"
s50_yaml_content warn off > "$T50_OFF/trackfw.yaml"

assert_lacks_pattern "credential-guard-mode-downgrade/non-vacuity" \
  "$S50_MSG" \
  bash -c "cd '$T50_OFF' && exec '$ROOT_DIR/bin/trackfw' validate"

# ---------------------------------------------------------------------------
# Cenário 51 — internal/validator: o CENÁRIO DECISIVO do M4
#              (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-
#              credential-guard, ML-1A/ML-2A) — a edição COMBINADA e NÃO
#              commitada (`credential_guard.mode: warn` + `rules:
#              credential_guard_mode_downgrade: off`, ambos só em disco)
#              CONTINUA sendo reportada. É exatamente o furo que o
#              ML-0A/Barreira B0 descreveram: antes do M4, essa única edição
#              rebaixava o controle E desligava quem avisaria, sem deixar
#              rastro (ver Context do roadmap).
#
# Braço de detecção: T51_BAD commita SÓ mode: block no HEAD (sem rules: —
# o caso comum, "nenhuma decisão tomada sobre a severidade desta regra").
# Em disco, SEM novo commit, o ataque combinado sobrescreve tanto mode
# quanto rules: na MESMA edição — s50_yaml_content warn off produz as duas
# chaves de uma vez, então não há como a edição ser parcial. A mensagem
# TEM que aparecer: se HEAD-informed severity não vencesse aqui, o disco
# "off" desligaria a regra que deveria denunciar a própria mudança de
# disco — o auto-silenciamento que o ADR fecha.
S51_BAD_HEAD="$(s50_yaml_content block)"

T51_BAD="$WORK/s51-combined-uncommitted"
# ML-1B (Defect 1): usa s5x_commit_fixture_with_origin para que origin/main
# aponte para o commit (sem rules: off) — stricter-wins retorna "error" e a
# edição combinada é detectada mesmo com `rules: off` só em disco.
s5x_commit_fixture_with_origin "$T51_BAD" "$S51_BAD_HEAD" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn off > "$T51_BAD/trackfw.yaml"

assert_fails_with "credential-guard-anchoring-combined-edit/detected" \
  "$S50_MSG" \
  bash -c "cd '$T51_BAD' && exec '$ROOT_DIR/bin/trackfw' validate"

# Braço autodiscriminante/contraste: T51_OFF_COMMITTED aplica o MESMO
# ataque de disco (mode: warn + rules: off) — mas desta vez `rules:
# credential_guard_mode_downgrade: off` também está COMMITADO no HEAD
# (junto com mode: block, no mesmo commit — desligamento legítimo, ADR
# §Decision point 5, mesmíssima construção do braço de não-vacuidade do
# Cenário 50 acima). A ÚNICA variável entre T51_BAD e T51_OFF_COMMITTED é
# se o "off" estava commitado — o resultado muda de "reportado" para
# "silenciado" SÓ por causa dessa variável, isolando exatamente o que o M4
# promete: desligar continua possível, mas só via commit (rastro
# auditável), nunca por edição de disco sozinha.
# ML-1B: também usa s5x_commit_fixture_with_origin para que origin/main
# carregue o rules: off commitado — stricter(off, off) = off → silencia. --
T51_OFF_COMMITTED="$WORK/s51-combined-off-committed"
s5x_commit_fixture_with_origin "$T51_OFF_COMMITTED" "$(s50_yaml_content block off)" \
  "trackfw.yaml with credential_guard.mode: block and rules: credential_guard_mode_downgrade: off"
s50_yaml_content warn off > "$T51_OFF_COMMITTED/trackfw.yaml"

assert_lacks_pattern "credential-guard-anchoring-combined-edit/legitimate-committed-off-silences" \
  "$S50_MSG" \
  bash -c "cd '$T51_OFF_COMMITTED' && exec '$ROOT_DIR/bin/trackfw' validate"

# 🔴 Prova de não-vacuidade do M4 em si (não apenas do knob `rules:`, já
# provado acima): sabotagem TEMPORÁRIA e NÃO commitada de
# internal/validator/validator_credential_guard_integrity.go — dentro de
# credentialGuardRuleSeverity, trocar `return
# credentialGuardStricterSeverity(headSeverity, diskSeverity)` por `return
# diskSeverity` (i.e., voltar ao comportamento pré-ADR, disco vence
# sempre), reconstruir bin/trackfw
# (vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12.md
# — go build ./... sozinho NÃO regenera o binário) e rodar T51_BAD de novo:
# o braço de detecção acima DEVE falhar (mensagem ausente, exit 0), porque
# o disco "off" venceria. Restaurar o arquivo e reconstruir antes de
# prosseguir — este ML não tem permissão de deixar internal/ tocado no
# diff final; a sabotagem é só para a prova de auditoria, nunca commitada.
# Saída colada no relatório final desta execução.
#
# Limite de cobertura conhecido (mesmo do Cenário 50): cobre só applyRule
# (Validate()/texto), não applyRuleTagged (ValidateTagged()/`validate
# --json`).
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 52 — internal/validator: o CARVE-OUT do .trackfw-baseline.json
#              (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-
#              credential-guard, Barreira B0/ML-1A/ML-2A) — uma violação de
#              regra de credential-guard listada em .trackfw-baseline.json
#              CONTINUA sendo reportada (filterBaselineTagged,
#              validator.go:500-511: as 3 regras em
#              credentialGuardAnchoredRules nunca são toleradas por
#              baseline, "regardless of what .trackfw-baseline.json contains
#              for it"). O canal do baseline é diferente do canal `rules:`
#              fechado pelos Cenários 50/51 — o arquivo é .gitignore'd
#              DELIBERADAMENTE (.gitignore:14-15), então "exigir commit" não
#              se aplica; o fechamento é excluir as 3 regras da elegibilidade
#              do ratchet, não comparar HEAD-vs-disco.
#
# Formato do .trackfw-baseline.json verificado contra a implementação (não
# escrito de improviso a partir da prosa do ADR — armadilha "prova que não
# prova" de outra forma): BaselineFile{Created, Violations []string,
# Warnings []string} (validator.go:18-23), e filterBaselineTagged compara
# CADA violação pelo texto INTEIRO da mensagem (v.Msg, validator.go:527) —
# não uma tag de regra, não um hash. $S50_FULL_MSG é esse literal completo
# para a regra de guarda; a chave para o carve-out entrar em jogo é o NOME
# da regra (credentialGuardAnchoredRules[v.Rule]), não o conteúdo da
# mensagem — mas o filtro só reconhece a mensagem se o texto bater
# EXATAMENTE, por isso a mensagem completa (não $S50_MSG, que é só um
# trecho) é o que entra no JSON.
#
# Braço autodiscriminante (mesmo fixture, uma única execução de `validate`,
# controle embutido em vez de scenario separado): T52 tem DUAS violações
# reais simultâneas — a de credential-guard (mode: warn não commitado,
# igual ao T50_BAD/T51_BAD) e uma de filename_uniqueness (regra NÃO-guard,
# "docs/roadmaps/backlog/dup.md" e "docs/roadmaps/done/dup.md" — mesmo nome
# (backlog+done, não wip/blocked: essas duas têm regras extras — "roadmap X is
# in wip but has no linked REQ/acceptance criteria block" — que poluiriam o exit
# code deste cenário sem relação com filename_uniqueness)
# em dois estados, validator.go:1938-2012) — e .trackfw-baseline.json lista
# as DUAS pelo texto completo. Se o carve-out funcionar: a de
# credential-guard continua aparecendo (não tolerada), a de
# filename_uniqueness some (tolerada normalmente). Isso prova as DUAS
# metades na mesma prova: (a) o formato do baseline realmente suprime
# quando a regra NÃO é de credential-guard — sem isso, a violação de guarda
# "aparecer" não provaria nada, porque o baseline poderia estar
# simplesmente mal-formado e não suprimir NADA; (b) o carve-out é
# ESPECÍFICO da regra de guarda, não uma falha geral do mecanismo de
# baseline. Sem o braço de filename_uniqueness, este cenário seria a
# "prova que não prova" documentada em
# vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12.md.
#
# 🔴 Prova de não-vacuidade: sabotagem TEMPORÁRIA e NÃO commitada de
# filterBaselineTagged (validator.go) — remover a condição
# `&& !credentialGuardAnchoredRules[v.Rule]` (deixando só `if tolerated {
# continue }`, i.e., o carve-out nunca existiu), reconstruir bin/trackfw e
# rodar T52 de novo: a violação de credential-guard deve DESAPARECER
# também (as duas ficariam suprimidas, exit 0) — provando que este cenário
# depende genuinamente do carve-out, não de alguma outra causa. Restaurar
# o arquivo e reconstruir antes de prosseguir. Saída colada no relatório.
# ---------------------------------------------------------------------------
S52_FILENAME_MSG='roadmap "dup.md" appears in multiple states: [backlog done]'

T52="$WORK/s52-baseline-carveout"
s50_commit_fixture "$T52" "$(s50_yaml_content block)" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn > "$T52/trackfw.yaml"
printf '# dup\n' > "$T52/docs/roadmaps/backlog/dup.md"
printf '# dup\n' > "$T52/docs/roadmaps/done/dup.md"
cat > "$T52/.trackfw-baseline.json" <<EOF
{
  "created": "2026-08-12T00:00:00Z",
  "violations": [
    "$S50_FULL_MSG",
    "roadmap \"dup.md\" appears in multiple states: [backlog done]"
  ],
  "warnings": []
}
EOF

set +e
s52_out=$(cd "$T52" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s52_status=$?
set -e
if [[ $s52_status -eq 0 ]]; then
  echo "FAIL [falsify/credential-guard-baseline-carveout]: baseline listando a violação de credential-guard deveria continuar reprovando (carve-out), saiu com 0" >&2
  echo "  output: $s52_out" >&2
  falsify_fail_point
fi
if ! grep -qF "$S50_MSG" <<<"$s52_out"; then
  echo "FAIL [falsify/credential-guard-baseline-carveout]: violação de credential-guard listada no baseline foi suprimida — carve-out não está funcionando" >&2
  echo "  output: $s52_out" >&2
  falsify_fail_point
fi
if grep -qF "$S52_FILENAME_MSG" <<<"$s52_out"; then
  echo "FAIL [falsify/credential-guard-baseline-carveout]: violação NÃO-guard (filename_uniqueness) listada no MESMO baseline não foi suprimida — o formato do baseline não está funcionando neste fixture (prova vácua: a linha acima passaria mesmo com um baseline mal-formado)" >&2
  echo "  output: $s52_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-baseline-carveout]: guarda reportada apesar do baseline, não-guarda suprimida pelo MESMO baseline"

# ---------------------------------------------------------------------------
# Cenário 53 — internal/validator: NÃO-REGRESSÃO — a regra "zero delta" do
#              ADR-2026-08-12 (Decision point, "zero delta para as outras
#              ~38 regras") não vazou para regras que NÃO são de
#              credential-guard. Uma regra comum (filename_uniqueness, não
#              listada em credentialGuardAnchoredRules) continua sendo
#              desligável por `rules: <nome>: off` NÃO commitado — o mesmo
#              comportamento de SEMPRE, disco-only (diskRuleSeverity), sem
#              nenhuma consulta a HEAD. Este é o cenário mais importante
#              para a confiança no M4: sem ele, uma regressão que ampliasse
#              o âncoramento por engano (ex.: alguém adiciona
#              filename_uniqueness a credentialGuardAnchoredRules "só por
#              via das dúvidas") passaria por TODOS os outros cenários deste
#              arquivo em silêncio.
#
# HEAD real é obrigatório aqui, mesmo esta regra nunca consultando HEAD
# hoje: sem HEAD, credentialGuardRuleSeverity cairia direto em
# diskSeverity mesmo que a regra FOSSE (por engano) adicionada ao mapa
# âncorado — mascarando exatamente o vazamento que este cenário existe
# para pegar (headTrackfwYAML retornando ok=false é um dos "sem âncora,
# cai no disco" do ADR §Decision point 4 — ver credentialGuardRuleSeverity,
# validator_credential_guard_integrity.go:252-270). Por isso T53 commita um
# trackfw.yaml (sem `rules:`, sem credential_guard — irrelevante para esta
# regra) via o MESMO s50_commit_fixture usado acima.
#
# Braço baseline (violação DEVE aparecer, sem override): T53_BASE só
# commita o scaffold padrão e cria os mesmos dois "dup.md" do Cenário 52 —
# filename_uniqueness não tem entrada em ruleDefaults (validator.go:101-109
# — só note_orphan e credential_guard_script_integrity estão lá), então o
# default é "error" e a violação derruba o exit code sem precisar de
# `rules:` no fixture (diferente do Cenário 49 — ver aviso da armadilha #2
# no despacho).
S53_BASE_HEAD="$(cat <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF
)"

T53_BASE="$WORK/s53-non-guard-baseline"
s50_commit_fixture "$T53_BASE" "$S53_BASE_HEAD" \
  "trackfw.yaml without any rules: override"
printf '# dup\n' > "$T53_BASE/docs/roadmaps/backlog/dup.md"
printf '# dup\n' > "$T53_BASE/docs/roadmaps/done/dup.md"

assert_fails_with "credential-guard-anchoring-non-regression/filename-uniqueness-baseline" \
  "$S52_FILENAME_MSG" \
  bash -c "cd '$T53_BASE' && exec '$ROOT_DIR/bin/trackfw' validate"

# Braço de detecção (silêncio esperado): MESMO HEAD (sem rules: commitado),
# MESMA divergência de dup.md — mas agora o disco acrescenta, SEM commit,
# `rules: filename_uniqueness: off`. Como esta regra NÃO está em
# credentialGuardAnchoredRules, ruleSeverity() (validator.go:111-128) usa
# diskRuleSeverity() puro — o mesmo caminho de SEMPRE, alheio a HEAD — e a
# violação deve sumir por completo (exit 0), provando que o M4 não alterou
# este caminho. -----------------------------------------------------------
T53_OFF="$WORK/s53-non-guard-off-uncommitted"
s50_commit_fixture "$T53_OFF" "$S53_BASE_HEAD" \
  "trackfw.yaml without any rules: override"
printf '# dup\n' > "$T53_OFF/docs/roadmaps/backlog/dup.md"
printf '# dup\n' > "$T53_OFF/docs/roadmaps/done/dup.md"
{
  printf '%s\n' "$S53_BASE_HEAD"
  printf 'rules:\n  filename_uniqueness: off\n'
} > "$T53_OFF/trackfw.yaml"

assert_lacks_pattern "credential-guard-anchoring-non-regression/filename-uniqueness-off-uncommitted-still-silences" \
  "$S52_FILENAME_MSG" \
  bash -c "cd '$T53_OFF' && exec '$ROOT_DIR/bin/trackfw' validate"

# 🔴 Prova de não-vacuidade: sabotagem TEMPORÁRIA e NÃO commitada de
# internal/validator/validator_credential_guard_integrity.go — acrescentar
# "filename_uniqueness": true ao mapa credentialGuardAnchoredRules
# (simulando um vazamento de escopo do M4 para uma regra comum),
# reconstruir bin/trackfw e rodar T53_OFF de novo: o braço acima DEVE
# passar a FALHAR (a mensagem volta a aparecer, exit != 0), porque
# credentialGuardRuleSeverity entraria em jogo — HEAD (sem `rules:` para
# filename_uniqueness) resolveria para o default "error", que venceria o
# "off" do disco pela comparação "mais estrita", exatamente o vazamento que
# este cenário existe para detectar. Restaurar o arquivo e reconstruir
# antes de prosseguir. Saída colada no relatório final desta execução.
#
# Escolha da regra: filename_uniqueness é a mesma usada como controle no
# Cenário 52 (reaproveita $S52_FILENAME_MSG e o fixture "dup.md"), tem
# default "error" (assert_fails_with exige exit != 0 — armadilha #2 do
# despacho, evitada de propósito: um default "warning" não derrubaria o
# exit code e a prova de baseline seria vácua).
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 54 — internal/validator: o bypass por variáveis de ambiente GIT_*
#              (ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-
#              credential-guard, ML-1B/ML-2B) CONTINUA fechado — nenhum dos
#              dois vetores achados pelo ML-3B (Hades) e reproduzidos por
#              Zeus consegue derrotar o M4 herdando GIT_* do processo pai.
#
# Os dois vetores (vault/notes/validador-git-env-bypass-filtre-por-prefixo-
# 2026-08-12.md): (1) REDIRECIONAMENTO — GIT_DIR/GIT_WORK_TREE apontando
# para outro repositório git, fazendo headTrackfwYAML() ler o HEAD ERRADO;
# (2) FALHA INDUZIDA — GIT_CONFIG_COUNT=abc, que não redireciona nada, só
# faz o subprocesso git sair 128 por config malformada em linha de comando
# — e headTrackfwYAML() trata QUALQUER falha do git como "sem âncora,
# silêncio", então basta fazer o git falhar por QUALQUER motivo. Cobrir só
# o vetor (1) deixaria uma regressão de volta para uma denylist enumerada
# (o erro de enquadramento original que não fechava o problema) passar
# despercebida — ver a nota acima, "O erro de enquadramento".
#
# RETARGET: se gitCommand()/cleanGitEnv() (internal/validator/
# validator_git_exec.go) mudarem de arquivo, assinatura ou deixarem de ser
# o único ponto de invocação de git deste pacote, reaponte este cenário — o
# invariante que importa é "nenhuma chamada de git deste pacote herda
# GIT_* do processo pai", não a função específica.
#
# Fixture: MESMO ataque combinado do Cenário 51 (T51_BAD) — HEAD commita
# SÓ credential_guard.mode: block (sem rules:), disco (sem novo commit)
# sobrescreve para mode: warn + rules: credential_guard_mode_downgrade: off
# — mas agora rodado com as variáveis de ambiente GIT_* do adversário
# injetadas no processo. Se cleanGitEnv() regredisse (parasse de filtrar,
# ou voltasse a ser denylist enumerada), qualquer um dos dois braços
# abaixo silenciaria a regra por completo — reabrindo o furo que este
# roadmap inteiro combate.
#
# Cobertura da tabela de reprodução manual de Zeus (despacho deste ML — 4
# linhas: "sem manipulação", "GIT_DIR + GIT_WORK_TREE", "GIT_CONFIG_COUNT=abc",
# "GIT_CEILING_DIRECTORIES", todas reportando violação POST-fix, ou seja: a
# tabela de Zeus é verificação de que a defesa segura vale nas 4 condições,
# não uma lista de vetores pré-fix): "sem manipulação" já é o braço de
# detecção do Cenário 51 (T51_BAD, fixture idêntico, sem env injetado) — não
# duplicado aqui de propósito. GIT_DIR/GIT_WORK_TREE e GIT_CONFIG_COUNT são
# os dois braços abaixo. GIT_CEILING_DIRECTORIES foi investigado e
# DELIBERADAMENTE NÃO virou um terceiro braço de detecção: testado cru
# contra `git -C <dir>` com <dir> já sendo a raiz do repo (a MESMA forma que
# gitCommand(".", ...) sempre usa neste código-base — cwd já é a raiz do
# projeto/repo, nunca um subdiretório) — o ceiling só bloqueia caminhada
# PARA CIMA na descoberta, e uma descoberta que já começa num diretório com
# `.git` não caminha para lugar nenhum. Confirmado empiricamente com/sem
# `cleanGitEnv()` sabotado, ceiling = o próprio <dir> e ceiling = o pai de
# <dir>: as duas vezes `git show HEAD:./f.yaml` teve sucesso normal, exit 0.
# Incluir um assert_fails_with para essa variável seria vácuo — o próprio
# controle autodiscriminante embutido (braço "-is-real" abaixo, aplicado ao
# mesmo padrão) rejeitaria a inclusão. Reportado a Zeus; se a tabela dele
# reproduziu um bypass real via GIT_CEILING_DIRECTORIES, foi contra uma
# forma de invocação diferente desta (ex.: cwd num subdiretório do repo),
# que não é como este código-base invoca git hoje.
#
# Limite de escopo: este cenário exercita só a limpeza de ambiente
# (cleanGitEnv() removendo GIT_*), NÃO o ancoramento `-C dir` em si — contra
# ESTE fixture (cwd == $T54 == raiz do repo), remover `-C` deixaria o
# cenário verde do mesmo jeito, porque o processo já está no diretório
# certo por padrão. O ancoramento `-C` é validado por leitura de código
# (gitCommand() sempre passa `-C dir` explícito, nunca confia só em cwd),
# não por um braço de falsificação dedicado neste arquivo.
S54_HEAD="$(s50_yaml_content block)"

T54="$WORK/s54-git-env-bypass"
# ML-1B (Defect 1): mesmo motivo do Cenário 51 — sem origin/main, disk "off"
# silenciaria a regra e os dois braços de bypass (GIT_DIR e GIT_CONFIG_COUNT)
# não reportariam S50_MSG, tornando as provas vácuas.
s5x_commit_fixture_with_origin "$T54" "$S54_HEAD" \
  "trackfw.yaml with credential_guard.mode: block"
s50_yaml_content warn off > "$T54/trackfw.yaml"

# Repositório-isca para o vetor de redirecionamento: um segundo repo git,
# em outro diretório, SEM trackfw.yaml commitado — se GIT_DIR/GIT_WORK_TREE
# vencessem o `-C $T54` explícito de gitCommand(), `git show
# HEAD:./trackfw.yaml` passaria a mirar este repo, onde o path não existe
# em HEAD, e headTrackfwYAML() retornaria ok=false — caindo no MESMO
# fallback disco-only que o Cenário 51 usa para provar o desligamento
# legítimo (mode: warn + rules: off, ambos só em disco aqui): a severidade
# silenciaria por inteiro.
T54_DECOY="$WORK/s54-decoy-repo"
mkdir -p "$T54_DECOY"
(
  cd "$T54_DECOY"
  git init -q
  git config user.email "falsify@trackfw.test"
  git config user.name "trackfw falsify"
  git config commit.gpgsign false
  git config core.hooksPath /dev/null
  printf 'decoy: true\n' > not-trackfw.yaml
  git add -A
  git commit -q -m "decoy repo without trackfw.yaml"
)

# Braço autodiscriminante embutido: prova, ANTES de testar o binário do
# trackfw, que o repositório-isca é um vetor de ataque genuíno contra um
# `git -C` cru — sem isto, um braço de detecção "verde" abaixo não
# provaria que cleanGitEnv()/`-C` é quem defende; provaria só que o
# ataque nunca funcionou contra nada.
set +e
s54_raw_out=$(GIT_DIR="$T54_DECOY/.git" GIT_WORK_TREE="$T54_DECOY" git -C "$T54" show HEAD:./trackfw.yaml 2>&1)
s54_raw_status=$?
set -e
if [[ $s54_raw_status -eq 0 ]] && grep -qF "mode: block" <<<"$s54_raw_out"; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/attack-inert]: GIT_DIR/GIT_WORK_TREE NÃO desviaram um \`git -C\` cru para o repositório-isca — o vetor de ataque em si está inerte neste ambiente, a prova abaixo não provaria nada" >&2
  echo "  output: $s54_raw_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-git-env-bypass/redirect-attack-is-real]: GIT_DIR/GIT_WORK_TREE realmente desviam um \`git -C\` cru (saiu $s54_raw_status, sem 'mode: block' do HEAD real) — confirma que o vetor é genuíno, não teatro"

set +e
s54_rawcfg_out=$(GIT_CONFIG_COUNT=abc git -C "$T54" rev-parse --is-inside-work-tree 2>&1)
s54_rawcfg_status=$?
set -e
if [[ $s54_rawcfg_status -eq 0 ]]; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/config-attack-inert]: GIT_CONFIG_COUNT=abc NÃO derrubou um \`git -C\` cru — o vetor de falha induzida está inerte neste ambiente, a prova abaixo não provaria nada" >&2
  echo "  output: $s54_rawcfg_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-git-env-bypass/config-attack-is-real]: GIT_CONFIG_COUNT=abc realmente derruba um \`git -C\` cru (saiu $s54_rawcfg_status) — confirma que o vetor é genuíno"

# Braço de detecção 1/2 — REDIRECIONAMENTO: mesmo GIT_DIR/GIT_WORK_TREE do
# repositório-isca acima, agora contra o binário trackfw. gitCommand()
# ancora com `-C $T54` e limpa GIT_* do ambiente do processo filho — se
# isso continuar funcionando, `git show HEAD:./trackfw.yaml` de
# headTrackfwYAML() lê o HEAD de $T54 (mode: block), "mais estrita entre
# HEAD e disco" vence sobre o disco (warn + off), e $S50_MSG aparece.
assert_fails_with "credential-guard-git-env-bypass/redirect-detected" \
  "$S50_MSG" \
  bash -c "cd '$T54' && GIT_DIR='$T54_DECOY/.git' GIT_WORK_TREE='$T54_DECOY' exec '$ROOT_DIR/bin/trackfw' validate"

# Braço de detecção 2/2 — FALHA INDUZIDA: GIT_CONFIG_COUNT=abc não
# redireciona nada, só faz o subprocesso git sair 128 por config malformada
# em linha de comando — o vetor que quebrou a denylist original de 8 nomes.
# cleanGitEnv() precisa removê-la do ambiente do processo filho pelo MESMO
# mecanismo (prefixo GIT_), sem tratamento especial por nome de variável.
assert_fails_with "credential-guard-git-env-bypass/config-count-detected" \
  "$S50_MSG" \
  bash -c "cd '$T54' && GIT_CONFIG_COUNT=abc exec '$ROOT_DIR/bin/trackfw' validate"

# Braço "worktree legítimo continua funcionando" — git worktree add cria
# uma segunda working tree cujo .git é um ARQUIVO (não diretório) apontando
# de volta para o gitdir principal; o comentário de validator_git_exec.go
# promete que isso continua resolvendo o MESMO repositório porque a
# descoberta normal a partir de `-C dir` já chega lá sem depender de nada
# herdado. Prova aqui SEM injetar GIT_* manualmente (o cenário decisivo de
# bypass já está provado acima) — só confirma que `-C` para dentro de uma
# worktree vinculada funciona no caminho feliz, nos dois sentidos (baseline
# silenciosa e detecção).
T54_WT_MAIN="$WORK/s54-worktree-main"
s50_commit_fixture "$T54_WT_MAIN" "$S54_HEAD" \
  "trackfw.yaml with credential_guard.mode: block"
T54_WT_LINKED="$WORK/s54-worktree-linked"
(cd "$T54_WT_MAIN" && git worktree add -q -b s54-wt-branch "$T54_WT_LINKED")

set +e
s54wt_ok_out=$(cd "$T54_WT_LINKED" && "$ROOT_DIR/bin/trackfw" validate 2>&1)
s54wt_ok_status=$?
set -e
if [[ $s54wt_ok_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/worktree-baseline]: worktree vinculada com disco == HEAD (mode: block) deveria passar, saiu com $s54wt_ok_status" >&2
  echo "  output: $s54wt_ok_out" >&2
  falsify_fail_point
fi
if grep -qF "$S50_MSG" <<<"$s54wt_ok_out"; then
  echo "FAIL [falsify/credential-guard-git-env-bypass/worktree-baseline]: worktree vinculada com disco == HEAD, mas a regra disparou mesmo assim" >&2
  echo "  output: $s54wt_ok_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-git-env-bypass/worktree-legitimate-baseline]"

s50_yaml_content warn > "$T54_WT_LINKED/trackfw.yaml"
assert_fails_with "credential-guard-git-env-bypass/worktree-legitimate-detection" \
  "$S50_MSG" \
  bash -c "cd '$T54_WT_LINKED' && exec '$ROOT_DIR/bin/trackfw' validate"

# 🔴 Prova de não-vacuidade: sabotagem TEMPORÁRIA e NÃO commitada de
# internal/validator/validator_git_exec.go — dentro de cleanGitEnv(),
# trocar o corpo por `return os.Environ()` (i.e., voltar a herdar GIT_* do
# processo pai sem filtro nenhum, o comportamento PRÉ-ML-1B), reconstruir
# bin/trackfw (vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-
# falsify-2026-08-12.md — go build ./... sozinho NÃO regenera esse
# binário) e rodar os dois braços de detecção acima de novo: AMBOS devem
# passar a FALHAR (mensagem ausente, exit 0) — o redirecionamento faz
# headTrackfwYAML() ler o repositório-isca (sem trackfw.yaml em HEAD,
# ok=false) e a falha induzida faz `git show HEAD:./trackfw.yaml` sair 128
# (ok=false também) — os dois caem no fallback disco-only, onde `rules:
# credential_guard_mode_downgrade: off` (só em disco, não commitado)
# silencia a regra por inteiro. Restaurar o arquivo e reconstruir antes de
# prosseguir — este ML não tem permissão de deixar internal/ tocado no
# diff final; a sabotagem é só para a prova de auditoria, nunca commitada.
# Saída colada no relatório final desta execução.
#
# Limite de cobertura conhecido (mesmo do Cenário 50/51): cobre só o wiring
# applyRule (Validate()/texto), não applyRuleTagged (ValidateTagged()/
# `validate --json`) em internal/validator/validator.go.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenários 55/56/57 — check-unknown-command-parity.sh (criado pelo ML-2A,
# ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw, sem prova de
# falsificação — lacuna reportada pelo próprio ML-2A) — cada cenário sabota UM
# CLI por vez, prova que o gate reprova (braço de detecção) e prova que o
# ciclo LIMPO passa (braço de linha de base), fechando o P4 de
# docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md que
# faltava para este gate.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Cenário 60 — item 2 (brecha de contorno): `git switch -c` (forma
# alternativa a `checkout -b` para criar branch) deve ser bloqueado. Braço
# baseline prova que o guard LIMPO bloqueia; braço de detecção corrompe o
# case "-c|-C|--create|--create=*|--force-create|--force-create=*)" do
# gerador Go para um padrão que nunca casa, reconstrói o script a partir de
# um módulo Go isolado, e prova que SEM essa linha o mesmo comando escapa
# (exit 0 em vez de 2) — não-vacuidade: a detecção depende exatamente do
# literal que o ML-1A adicionou, não de qualquer outro efeito colateral.
# ---------------------------------------------------------------------------
T60_BASE_MOD="$WORK/s60-base-mod"
mkdir -p "$T60_BASE_MOD/cmd" "$T60_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T60_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T60_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T60_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T60_BASE_MOD/go.sum"

T60_BASE_OUT="$WORK/s60-base-out"
run_go_guard_dump "setup-s60-go-baseline-build" "$T60_BASE_MOD" "$T60_BASE_OUT"

assert_guard_exit "git-branch-guard/switch-c/baseline-blocks" \
  "$T60_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git switch -c feat/x"}}' \
  2

T60_MOD="$WORK/s60-mod"
mkdir -p "$T60_MOD/cmd" "$T60_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T60_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T60_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T60_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T60_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T60_MOD/internal/generators/scaffold.go" \
  '-c|-C|--create|--create=*|--force-create|--force-create=*)' \
  '--never-matches-anything-s58)' \
  "s60-go-switch-c-detection-removed"

T60_OUT="$WORK/s60-out"
run_go_guard_dump "setup-s60-go-corrupted-build" "$T60_MOD" "$T60_OUT"

assert_guard_exit "git-branch-guard/switch-c/detection-catches-bypass" \
  "$T60_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git switch -c feat/x"}}' \
  0

# ---------------------------------------------------------------------------
# Cenário 61 — item 1 (falso-positivo por prosa): uma mensagem de commit cuja
# linha COMEÇA com "git checkout -b" (reprodução literal do
# vault/notes/git-branch-guard-falso-positivo-em-linha-de-mensagem-de-commit-
# 2026-08-16.md) não deve bloquear. Braço baseline prova que o guard LIMPO
# permite; braço de detecção corrompe match_subcommand() para voltar a usar o
# `sed` cego antigo (em vez de strip_heredoc_bodies + quote_aware_split),
# reconstrói o script isolado, e prova que SEM a correção o mesmo comando
# volta a ser bloqueado — não-vacuidade: a permissividade depende exatamente
# do pré-processamento quote-aware introduzido por este ML, não de qualquer
# outro efeito colateral. A prova de não-regressão complementar (comandos
# reais encadeados atrás de um `-m` corretamente fechado continuam
# bloqueados) está em internal/generators/git_branch_guard_test.go
# (TestGitBranchGuard_ChainedCommand_SecondGitBlocked e as novas
# TestGitBranchGuard_QuotedMessageThenRealChainedCommand_StillBlocks/
# TestGitBranchGuard_SwitchDashC_Blocks/TestGitBranchGuard_SwitchWithoutCreateFlag_Allows).
# ---------------------------------------------------------------------------
# Reprodução literal do incidente real (vault note): `-m "$(cat <<'EOF' ...
# EOF)"` (convenção de mensagem de commit multi-linha do próprio CLAUDE.md
# deste repositório) — a quebra de linha REAL logo após `<<'EOF'` faz a
# primeira linha do corpo (`  git checkout -b ...`) virar, depois de
# stripar espaço à esquerda, seu PRÓPRIO segmento com "git" como primeiro
# token. Um `-m "..."` sem heredoc, sem quebra de linha antes de "git", não
# reproduz o bug (a linha inteira permanece grudada no segmento que começa
# com "bin/trackfw").
PROSE_PAYLOAD=$("$PY_BIN" -c '
import json
cmd = ("bin/trackfw commit -m \"$(cat <<'"'"'EOF'"'"'\n"
       "  git checkout -b            -> bloqueado pelo guard\n"
       "  trackfw branch new chore/  -> recusado\n"
       "EOF\n"
       ")\"")
print(json.dumps({"tool_input": {"command": cmd}}))
' | strip_cr)

T61_BASE_MOD="$WORK/s61-base-mod"
mkdir -p "$T61_BASE_MOD/cmd" "$T61_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T61_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T61_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T61_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T61_BASE_MOD/go.sum"

T61_BASE_OUT="$WORK/s61-base-out"
run_go_guard_dump "setup-s61-go-baseline-build" "$T61_BASE_MOD" "$T61_BASE_OUT"

assert_guard_exit "git-branch-guard/prose-in-message/baseline-allows" \
  "$T61_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  "$PROSE_PAYLOAD" \
  0

T61_MOD="$WORK/s61-mod"
mkdir -p "$T61_MOD/cmd" "$T61_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T61_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T61_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T61_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T61_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T61_MOD/internal/generators/scaffold.go" \
  'normalized=$(strip_heredoc_bodies "$1")
  normalized=$(quote_aware_split "$normalized")' \
  'normalized=$(printf '"'"'%s'"'"' "$1" | sed -e '"'"'s/&&/\n/g'"'"' -e '"'"'s/||/\n/g'"'"' -e '"'"'s/[;|]/\n/g'"'"')' \
  "s61-go-quote-aware-split-reverted"

T61_OUT="$WORK/s61-out"
run_go_guard_dump "setup-s61-go-corrupted-build" "$T61_MOD" "$T61_OUT"

assert_guard_exit "git-branch-guard/prose-in-message/detection-catches-regression" \
  "$T61_OUT/scripts/trackfw-git-branch-guard.sh" \
  "$PROSE_PAYLOAD" \
  2

# ---------------------------------------------------------------------------
# Cenário 62 — scripts/trackfw-git-branch-guard.sh (ML-4B,
# ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-
# da-release-7-0-0.md), corretivo do veredito BLOQUEAR do hades-tf
# (docs/seguranca/2026-08-16-revisao-do-git-branch-guard.md): duas evasões
# reproduzidas e fechadas nesse ML — prefixo `env`/`command` antes de `git`, e
# flag do `checkout -b` fora da primeira posição de token (`-q -b`,
# `--no-track -b`). Mesmo padrão baseline+detecção dos Cenários 60/61: braço
# baseline prova que o guard LIMPO bloqueia; braço de detecção corrompe UM
# literal isolado do gerador Go, reconstrói o script a partir de um módulo Go
# isolado, e prova que sem essa linha o mesmo comando escapa (exit 0 em vez
# de 2) — não-vacuidade: a detecção depende exatamente do literal que o
# ML-4B adicionou, não de qualquer outro efeito colateral.
# ---------------------------------------------------------------------------

# --- 62a — prefixo env/command antes de git ---------------------------------
T62A_BASE_MOD="$WORK/s62a-base-mod"
mkdir -p "$T62A_BASE_MOD/cmd" "$T62A_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62A_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62A_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62A_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62A_BASE_MOD/go.sum"

T62A_BASE_OUT="$WORK/s62a-base-out"
run_go_guard_dump "setup-s62a-go-baseline-build" "$T62A_BASE_MOD" "$T62A_BASE_OUT"

assert_guard_exit "git-branch-guard/env-command-prefix/baseline-blocks-env" \
  "$T62A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env git commit -m \"x\""}}' \
  2

assert_guard_exit "git-branch-guard/env-command-prefix/baseline-blocks-command" \
  "$T62A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"command git push"}}' \
  2

T62A_MOD="$WORK/s62a-mod"
mkdir -p "$T62A_MOD/cmd" "$T62A_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62A_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62A_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62A_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62A_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T62A_MOD/internal/generators/scaffold.go" \
  'while [ "$base" = "env" ] || [ "$base" = "command" ]; do' \
  'while [ "$base" = "__never-matches-s62a__" ]; do' \
  "s62a-go-env-command-prefix-stripping-removed"

T62A_OUT="$WORK/s62a-out"
run_go_guard_dump "setup-s62a-go-corrupted-build" "$T62A_MOD" "$T62A_OUT"

assert_guard_exit "git-branch-guard/env-command-prefix/detection-catches-bypass-env" \
  "$T62A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env git commit -m \"x\""}}' \
  0

assert_guard_exit "git-branch-guard/env-command-prefix/detection-catches-bypass-command" \
  "$T62A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"command git push"}}' \
  0

# Auto-discriminação: a corrupção acima é um `while` que nunca casa nenhum
# `base` — se ela silenciasse o guard inteiro (em vez de só o stripping de
# env/command), as duas asserções acima "provariam" bypass por um motivo
# errado. `git push` puro (sem prefixo) contra o MESMO build corrompido
# precisa continuar bloqueado — isola a corrupção ao stripping de
# env/command, não a uma quebra geral do matcher.
assert_guard_exit "git-branch-guard/env-command-prefix/detection-does-not-break-plain-push" \
  "$T62A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git push"}}' \
  2

# --- 62b — flag do checkout -b fora da primeira posição de token -----------
T62B_BASE_MOD="$WORK/s62b-base-mod"
mkdir -p "$T62B_BASE_MOD/cmd" "$T62B_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62B_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62B_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62B_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62B_BASE_MOD/go.sum"

T62B_BASE_OUT="$WORK/s62b-base-out"
run_go_guard_dump "setup-s62b-go-baseline-build" "$T62B_BASE_MOD" "$T62B_BASE_OUT"

assert_guard_exit "git-branch-guard/checkout-flag-position/baseline-blocks-q-b" \
  "$T62B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -q -b nova"}}' \
  2

assert_guard_exit "git-branch-guard/checkout-flag-position/baseline-blocks-no-track" \
  "$T62B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout --no-track -b nova"}}' \
  2

T62B_MOD="$WORK/s62b-mod"
mkdir -p "$T62B_MOD/cmd" "$T62B_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T62B_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T62B_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T62B_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T62B_MOD/go.sum"

# Corrupção retargetada para a forma EXATA pré-ML-4B (não um pattern
# "nunca casa" genérico): reverte o for-loop de varredura de tokens para o
# `if [ "${1:-}" = "-b" ]; then` original, que só olha o token IMEDIATAMENTE
# seguinte a `checkout`. Isso preserva a detecção de `git checkout -b nova`
# (a flag na primeira posição) e derruba só `-q -b`/`--no-track -b` — o
# discriminante preciso do que o ML-4B mudou, não uma corrupção que também
# apagaria a detecção de `checkout -b` simples (o que tornaria o cenário
# indistinguível de "checkout detection sumiu inteira").
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T62B_MOD/internal/generators/scaffold.go" \
  '      checkout)
        for tok2 in "$@"; do
          case "$tok2" in
            -b|-B|--orphan|--orphan=*)
              echo "checkout-b"
              return 0
              ;;
          esac
        done
' \
  '      checkout)
        if [ "${1:-}" = "-b" ]; then
          echo "checkout-b"
          return 0
        fi
' \
  "s62b-go-checkout-token-scan-reverted-to-pre-ml4b"

T62B_OUT="$WORK/s62b-out"
run_go_guard_dump "setup-s62b-go-corrupted-build" "$T62B_MOD" "$T62B_OUT"

assert_guard_exit "git-branch-guard/checkout-flag-position/detection-catches-bypass-q-b" \
  "$T62B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -q -b nova"}}' \
  0

assert_guard_exit "git-branch-guard/checkout-flag-position/detection-catches-bypass-no-track" \
  "$T62B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout --no-track -b nova"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido (revertido para a
# forma pré-ML-4B), `git checkout -b nova` — flag na primeira posição —
# precisa continuar bloqueado. Prova que a corrupção isola exatamente o
# token-scan que o ML-4B acrescentou, não a detecção de checkout -b como um
# todo.
assert_guard_exit "git-branch-guard/checkout-flag-position/detection-does-not-break-plain-checkout-b" \
  "$T62B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -b nova"}}' \
  2

# ---------------------------------------------------------------------------
# Cenário 63 — scripts/trackfw-git-branch-guard.sh (ML-4C,
# ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-
# da-release-7-0-0.md), corretivo da reverificação do hades-tf após levantar
# o bloqueio do ML-4A: três evasões apontadas e fechadas neste ML —
# `git branch <nome>` (e -c/-C/-m/-M), `git worktree add -b`, e
# `env CHAVE=valor git ...`. Mesmo padrão baseline+detecção dos Cenários
# 60/61/62: braço baseline prova que o guard LIMPO bloqueia; braço de
# detecção corrompe UM literal isolado do gerador Go, reconstrói o script a
# partir de um módulo Go isolado, e prova que sem essa linha o mesmo comando
# escapa (exit 0 em vez de 2) — não-vacuidade: a detecção depende exatamente
# do literal que o ML-4C adicionou, não de qualquer outro efeito colateral.
# ---------------------------------------------------------------------------

# --- 63a — git branch <nome> (argumento posicional puro cria a branch) -----
T63A_BASE_MOD="$WORK/s63a-base-mod"
mkdir -p "$T63A_BASE_MOD/cmd" "$T63A_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63A_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63A_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63A_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63A_BASE_MOD/go.sum"

T63A_BASE_OUT="$WORK/s63a-base-out"
run_go_guard_dump "setup-s63a-go-baseline-build" "$T63A_BASE_MOD" "$T63A_BASE_OUT"

assert_guard_exit "git-branch-guard/branch-create/baseline-blocks-positional" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch nova"}}' \
  2

assert_guard_exit "git-branch-guard/branch-create/baseline-blocks-dash-c" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -c origem nova"}}' \
  2

assert_guard_exit "git-branch-guard/branch-create/baseline-allows-list" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -a"}}' \
  0

assert_guard_exit "git-branch-guard/branch-create/baseline-allows-delete" \
  "$T63A_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -d nome"}}' \
  0

T63A_MOD="$WORK/s63a-mod"
mkdir -p "$T63A_MOD/cmd" "$T63A_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63A_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63A_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63A_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63A_MOD/go.sum"

# Corrupção isolada: neutraliza SÓ a marcação de "argumento posicional puro
# cria branch" (saw_positional=1 -> saw_positional=0), sem tocar
# branch_action (-c/-C/-m/-M) nem has_delete — prova que a detecção depende
# exatamente desse literal, e não derruba -c/-C/-m/-M nem -d/-D (asserção de
# auto-discriminação abaixo).
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T63A_MOD/internal/generators/scaffold.go" \
  '            *)
              saw_positional=1
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then' \
  '            *)
              saw_positional=0
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then' \
  "s63a-go-branch-positional-detection-removed"

T63A_OUT="$WORK/s63a-out"
run_go_guard_dump "setup-s63a-go-corrupted-build" "$T63A_MOD" "$T63A_OUT"

assert_guard_exit "git-branch-guard/branch-create/detection-catches-bypass-positional" \
  "$T63A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch nova"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido, `git branch -c origem
# nova` (via branch_action, não saw_positional) e `git branch -d nome`
# (leitura/delete, nunca deveria bloquear) precisam se comportar
# exatamente como antes — prova que a corrupção isola só o caminho de
# argumento posicional puro.
assert_guard_exit "git-branch-guard/branch-create/detection-does-not-break-dash-c" \
  "$T63A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -c origem nova"}}' \
  2

assert_guard_exit "git-branch-guard/branch-create/detection-does-not-break-delete" \
  "$T63A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git branch -d nome"}}' \
  0

# --- 63b — git worktree add -b (forma direta de criar branch) --------------
T63B_BASE_MOD="$WORK/s63b-base-mod"
mkdir -p "$T63B_BASE_MOD/cmd" "$T63B_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63B_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63B_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63B_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63B_BASE_MOD/go.sum"

T63B_BASE_OUT="$WORK/s63b-base-out"
run_go_guard_dump "setup-s63b-go-baseline-build" "$T63B_BASE_MOD" "$T63B_BASE_OUT"

assert_guard_exit "git-branch-guard/worktree-add-b/baseline-blocks" \
  "$T63B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git worktree add -b nova ../nova"}}' \
  2

assert_guard_exit "git-branch-guard/worktree-add-b/baseline-allows-without-b" \
  "$T63B_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git worktree add ../nova existing-branch"}}' \
  0

T63B_MOD="$WORK/s63b-mod"
mkdir -p "$T63B_MOD/cmd" "$T63B_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63B_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63B_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63B_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63B_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T63B_MOD/internal/generators/scaffold.go" \
  '      worktree)
        if [ "${1:-}" = "add" ]; then' \
  '      worktree)
        if [ "${1:-}" = "__never-matches-s63b__" ]; then' \
  "s63b-go-worktree-add-detection-removed"

T63B_OUT="$WORK/s63b-out"
run_go_guard_dump "setup-s63b-go-corrupted-build" "$T63B_MOD" "$T63B_OUT"

assert_guard_exit "git-branch-guard/worktree-add-b/detection-catches-bypass" \
  "$T63B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git worktree add -b nova ../nova"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido, `git push` puro
# continua bloqueado — isola a corrupção ao worktree add -b, não uma
# quebra geral do matcher.
assert_guard_exit "git-branch-guard/worktree-add-b/detection-does-not-break-plain-push" \
  "$T63B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git push"}}' \
  2

# --- 63c — env CHAVE=valor git ... (stripping de atribuição de variável) ---
T63C_BASE_MOD="$WORK/s63c-base-mod"
mkdir -p "$T63C_BASE_MOD/cmd" "$T63C_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63C_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63C_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63C_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63C_BASE_MOD/go.sum"

T63C_BASE_OUT="$WORK/s63c-base-out"
run_go_guard_dump "setup-s63c-go-baseline-build" "$T63C_BASE_MOD" "$T63C_BASE_OUT"

assert_guard_exit "git-branch-guard/env-var-assignment/baseline-blocks-single" \
  "$T63C_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar git push"}}' \
  2

assert_guard_exit "git-branch-guard/env-var-assignment/baseline-blocks-multiple" \
  "$T63C_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar BAZ=qux git commit -m x"}}' \
  2

assert_guard_exit "git-branch-guard/env-var-assignment/baseline-still-evades-flag-form" \
  "$T63C_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env -i git push"}}' \
  0

T63C_MOD="$WORK/s63c-mod"
mkdir -p "$T63C_MOD/cmd" "$T63C_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T63C_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T63C_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T63C_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T63C_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T63C_MOD/internal/generators/scaffold.go" \
  '      if [ "$is_env" = "env" ]; then' \
  '      if [ "$is_env" = "__never-matches-s63c__" ]; then' \
  "s63c-go-env-var-assignment-stripping-removed"

T63C_OUT="$WORK/s63c-out"
run_go_guard_dump "setup-s63c-go-corrupted-build" "$T63C_MOD" "$T63C_OUT"

assert_guard_exit "git-branch-guard/env-var-assignment/detection-catches-bypass-single" \
  "$T63C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar git push"}}' \
  0

assert_guard_exit "git-branch-guard/env-var-assignment/detection-catches-bypass-multiple" \
  "$T63C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env FOO=bar BAZ=qux git commit -m x"}}' \
  0

# Auto-discriminação: contra o MESMO build corrompido, a forma NUA (sem
# atribuição de variável) `env git push`, fechada pelo ML-4B, precisa
# continuar bloqueada — isola a corrupção ao stripping de CHAVE=valor, não
# ao stripping de env/command como um todo.
assert_guard_exit "git-branch-guard/env-var-assignment/detection-does-not-break-bare-env-prefix" \
  "$T63C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"env git push"}}' \
  2

# ---------------------------------------------------------------------------
# Cenário 64 — scripts/trackfw-git-branch-guard.sh (ML-1A,
# ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
# integridade-independente-de-fiacao.md): o guard vira no-op (exit 0) fora
# de projeto trackfw (sem trackfw.yaml em nenhum ancestral do cwd) — decisão
# de ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md,
# pré-requisito para cabear o guard em escopo global (Wave 2 do mesmo
# roadmap) sem quebrar git commit/push em toda a máquina. Quatro braços,
# mesmo padrão dos Cenários 62/63: baseline sem trackfw.yaml prova o no-op;
# baseline COM trackfw.yaml prova reverse-vacuity (o 0 acima veio do no-op,
# não de um build quebrado); detecção corrompe o literal isolado do
# gerador Go que fecha o probe (`[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0`) e
# prova que sem essa linha o guard bloqueia INCONDICIONALMENTE mesmo fora de
# projeto trackfw; auto-discriminação prova que, contra o MESMO build
# corrompido, dentro de projeto trackfw o comportamento de bloqueio
# permanece — isola a corrupção ao probe do no-op, não ao matcher inteiro.
# ---------------------------------------------------------------------------
T64_NO_YAML_DIR="$WORK/s64-no-trackfw-yaml"
mkdir -p "$T64_NO_YAML_DIR"
T64_WITH_YAML_DIR="$WORK/s64-with-trackfw-yaml"
mkdir -p "$T64_WITH_YAML_DIR"
echo "project_name: s64-fixture" > "$T64_WITH_YAML_DIR/trackfw.yaml"

T64_BASE_MOD="$WORK/s64-base-mod"
mkdir -p "$T64_BASE_MOD/cmd" "$T64_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T64_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T64_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T64_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T64_BASE_MOD/go.sum"

T64_BASE_OUT="$WORK/s64-base-out"
run_go_guard_dump "setup-s64-go-baseline-build" "$T64_BASE_MOD" "$T64_BASE_OUT"

(
  cd "$T64_NO_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/baseline-noop-without-trackfw-yaml" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    0
)

(
  cd "$T64_WITH_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/baseline-blocks-with-trackfw-yaml" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    2
)

T64_MOD="$WORK/s64-mod"
mkdir -p "$T64_MOD/cmd" "$T64_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T64_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T64_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T64_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T64_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T64_MOD/internal/generators/scaffold.go" \
  '[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0' \
  '[ "$_TRACKFW_FOUND" -eq 1 ] || true' \
  "s64-go-noop-probe-removed"

T64_OUT="$WORK/s64-out"
run_go_guard_dump "setup-s64-go-corrupted-build" "$T64_MOD" "$T64_OUT"

(
  cd "$T64_NO_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/detection-catches-bypass-without-trackfw-yaml" \
    "$T64_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    2
)

# Auto-discriminação: contra o MESMO build corrompido, DENTRO de projeto
# trackfw o guard precisa continuar bloqueando exatamente como antes —
# isola a corrupção ao probe do no-op, não a uma quebra geral do matcher.
(
  cd "$T64_WITH_YAML_DIR" && assert_guard_exit "git-branch-guard/no-op-outside-project/detection-does-not-break-inside-project" \
    "$T64_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    2
)

# ---------------------------------------------------------------------------
# Cenário 65 — scripts/trackfw-git-branch-guard.sh (ML-1B, mesma ROADMAP-
# 2026-08-17): a auditoria do arquiteto reprovou o ML-1A porque o no-op
# saía com exit 0 ANTES de ler o stdin -- quem escreve o payload JSON no
# pipe (o próprio harness) recebia EPIPE, reprodutível em 100% das chamadas
# fora de projeto trackfw. Três braços: baseline com o build ATUAL (já
# corrigido pelo ML-1B) prova que o escritor termina limpo; baseline com
# payload GRANDE (>64KB, estoura o buffer do pipe) prova que o dreno
# funciona mesmo sob pressão de buffer; detecção corrompe o literal isolado
# do dreno de stdin (`IFS= read -r -t 2 -d '' _TRACKFW_STDIN || true` ->
# `true`, deixando a inicialização `_TRACKFW_STDIN=""` intacta mas
# neutralizando a leitura real -- literal atualizado no ML-3A da ROADMAP-
# 2026-09-09-guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-nos-3-
# clis.md, que trocou o discriminante `-t 0` por dreno com orçamento de
# tempo) e prova que, sem o dreno, o EPIPE volta -- isolando a regressão ao
# dreno em si, não a uma mudança geral no probe do no-op.
# ---------------------------------------------------------------------------
T65_NO_YAML_DIR="$WORK/s65-no-trackfw-yaml"
mkdir -p "$T65_NO_YAML_DIR"

(
  cd "$T65_NO_YAML_DIR" && assert_writer_no_epipe \
    "git-branch-guard/stdin-drain-before-noop/baseline-writer-clean" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    0 1
)

T65_BIG_PAYLOAD=$("$PY_BIN" -c "import json; print(json.dumps({'tool_input':{'command':'git push','pad':'x'*200000}}))" | strip_cr)
(
  cd "$T65_NO_YAML_DIR" && assert_writer_no_epipe \
    "git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload" \
    "$T64_BASE_OUT/scripts/trackfw-git-branch-guard.sh" \
    "$T65_BIG_PAYLOAD" \
    0 1
)

T65_MOD="$WORK/s65-mod"
mkdir -p "$T65_MOD/cmd" "$T65_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T65_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T65_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T65_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T65_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T65_MOD/internal/generators/scaffold.go" \
  "IFS= read -r -t 2 -d '' _TRACKFW_STDIN || true" \
  "true" \
  "s65-go-stdin-drain-removed"

T65_OUT="$WORK/s65-out"
run_go_guard_dump "setup-s65-go-corrupted-build" "$T65_MOD" "$T65_OUT"

(
  cd "$T65_NO_YAML_DIR" && assert_writer_no_epipe \
    "git-branch-guard/stdin-drain-before-noop/detection-catches-epipe-regression" \
    "$T65_OUT/scripts/trackfw-git-branch-guard.sh" \
    '{"tool_input":{"command":"git push"}}' \
    0 0
)

# ---------------------------------------------------------------------------
# Cenário 67 — dedup projeto+global para o `git-branch-guard`
#              (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
#              projeto-e-integridade-independente-de-fiacao.md, Wave 2/ML-2B):
#              com a fiação global do git-branch-guard instalada para o
#              Claude, `trackfw discover --init` NÃO deve escrever a entrada
#              de projeto ($CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-
#              guard.sh) — senão o guard roda duas vezes por chamada Bash e
#              a mensagem de bloqueio dobra (o sintoma medido que abriu esta
#              ML). globalGitBranchGuardInstalledClaude() (Go internal/
#              generators/agentfiles.go) é o seam.
#
# Por que Go sozinho: mesmo precedente dos Cenários 57 (Go)/59 (Python) —
# sabotagem de um único stack quando a paridade estrutural entre os 3 já é
# garantida em outro lugar (make quality roda `go test`/`node --test`/
# `pytest` nos 3, cada um com seus próprios testes de dedup do
# git-branch-guard criados por este ML — internal/generators/
# git_branch_guard_dedup_test.go, npm/tests/git_branch_guard_dedup.test.js,
# pypi/tests/test_git_branch_guard_dedup.py — que já provam a mesma
# propriedade nos 3 stacks via unidade). Este cenário prova o mecanismo
# fim-a-fim contra o BINÁRIO REAL (`discover --init`), o que nenhum teste de
# unidade cobre (eles chamam os injetores diretamente, nunca o comando CLI).
#
# Três braços:
#   1. Baseline — $HOME sintético com ~/.claude/settings.json apontando
#      PreToolUse[Bash] para o caminho EXATO que
#      globalGitBranchGuardScriptPath() resolveria
#      ($HOME/.trackfw/scripts/trackfw-git-branch-guard.sh); roda
#      `discover --init` de verdade contra um fixture de projeto (marcador
#      CLAUDE.md) e prova que .claude/settings.json do PROJETO não contém a
#      entrada de git-branch-guard, mas CONTÉM a de credential-guard — prova
#      que o skip é específico do guard, não um apagão geral de PreToolUse.
#   2. Reverse-vacuity — mesmo fixture de projeto, $HOME vazio (nenhuma
#      fiação global) → a entrada de projeto do git-branch-guard aparece
#      normalmente, provando que a ausência no braço 1 veio da fiação
#      global, não de alguma quebra geral do injector.
#   3. Detecção — corrompe o CORPO INTEIRO de globalGitBranchGuardInstalledClaude
#      (não só um trecho — o texto `hookArrayHasCommand(hooks["PreToolUse"],
#      "Bash", scriptPath)` também aparece, idêntico, dentro de
#      globalCredentialGuardInstalledClaude, então corrupt_literal com esse
#      recorte sozinho falharia a asserção de ocorrência única) para
#      `return false` incondicional, numa cópia isolada de internal/+cmd/
#      (mesmo padrão do Cenário 46); reconstrói e roda o MESMO fixture do
#      braço 1 (mesmo $HOME sintético com a fiação global instalada) contra
#      o binário corrompido — a entrada de projeto REAPARECE, reproduzindo o
#      sintoma exato de mensagem duplicada que motivou esta ML.
# ---------------------------------------------------------------------------
T67_PROJECT_DIR="$WORK/s67-project"
mkdir -p "$T67_PROJECT_DIR"
: > "$T67_PROJECT_DIR/CLAUDE.md"

# T67_FAKE_HOME is built from a slash-collapsed form of $WORK (WORK67_CLEAN),
# not $WORK itself -- $TMPDIR ends in "/" on macOS, so $WORK carries an
# embedded "//" that used to make this baseline arm's outcome depend on the
# exact bug ML-2C fixes (ROADMAP-2026-08-17, "Diagnostico do arquiteto"). The
# // tolerance itself is now exercised EXPLICITLY by braco 4 below with a
# deliberately corrupted HOME, so this baseline stays deterministic across
# platforms instead of accidentally depending on TMPDIR's shape.
WORK67_CLEAN=$(printf '%s' "$WORK" | sed 's#//*#/#g')
T67_FAKE_HOME="$WORK67_CLEAN/s67-fake-home-installed"
mkdir -p "$T67_FAKE_HOME/.claude"
cat >"$T67_FAKE_HOME/.claude/settings.json" <<EOF
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$T67_FAKE_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"}]}]}}
EOF

set +e
(cd "$T67_PROJECT_DIR" && HOME="$T67_FAKE_HOME" "$ROOT_DIR/bin/trackfw" discover --init) >"$WORK/s67-baseline-discover.log" 2>&1
s67b_status=$?
set -e
if [[ $s67b_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/baseline-setup]: discover --init saiu com $s67b_status" >&2
  cat "$WORK/s67-baseline-discover.log" >&2
  falsify_fail_point
fi

s67b_settings="$T67_PROJECT_DIR/.claude/settings.json"
if grep -qF 'trackfw-git-branch-guard.sh' "$s67b_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/baseline-skips-project-entry]: entrada de git-branch-guard presente em $s67b_settings com a fiação global instalada" >&2
  cat "$s67b_settings" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-dedup/baseline-skips-project-entry]"

if ! grep -qF 'trackfw-credential-guard.sh' "$s67b_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/baseline-credential-guard-unaffected]: entrada de credential-guard ausente — o skip não deveria afetar o outro guard" >&2
  cat "$s67b_settings" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-dedup/baseline-credential-guard-unaffected]"

# --- braço 2: reverse-vacuity, $HOME vazio -> entrada de projeto normal ---
T67_PROJECT_DIR_RV="$WORK/s67-project-reverse-vacuity"
mkdir -p "$T67_PROJECT_DIR_RV"
: > "$T67_PROJECT_DIR_RV/CLAUDE.md"
T67_EMPTY_HOME="$WORK/s67-empty-home"
mkdir -p "$T67_EMPTY_HOME"

set +e
(cd "$T67_PROJECT_DIR_RV" && HOME="$T67_EMPTY_HOME" "$ROOT_DIR/bin/trackfw" discover --init) >"$WORK/s67-rv-discover.log" 2>&1
s67rv_status=$?
set -e
if [[ $s67rv_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/reverse-vacuity-setup]: discover --init saiu com $s67rv_status" >&2
  cat "$WORK/s67-rv-discover.log" >&2
  falsify_fail_point
fi

s67rv_settings="$T67_PROJECT_DIR_RV/.claude/settings.json"
if ! grep -qF 'trackfw-git-branch-guard.sh' "$s67rv_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/reverse-vacuity]: entrada de git-branch-guard ausente com \$HOME vazio (sem fiação global) — o skip não deveria acontecer aqui" >&2
  cat "$s67rv_settings" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-dedup/reverse-vacuity]"

# --- braço 3: detecção — dedup neutralizado, entrada de projeto reaparece ---
T67_MOD="$WORK/s67-corrupt-go"
mkdir -p "$T67_MOD/cmd" "$T67_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T67_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T67_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T67_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T67_MOD/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/agentfiles.go" "$T67_MOD/internal/generators/agentfiles.go" \
  $'func globalGitBranchGuardInstalledClaude() bool {\n\tscriptPath, ok := globalGitBranchGuardScriptPath()\n\tif !ok {\n\t\treturn false\n\t}\n\troot, ok := readGlobalHookJSON(".claude", "settings.json")\n\tif !ok {\n\t\treturn false\n\t}\n\thooks, _ := root["hooks"].(map[string]interface{})\n\treturn hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath)\n}' \
  $'func globalGitBranchGuardInstalledClaude() bool {\n\treturn false\n}' \
  "s67-go-claude-git-branch-guard-dedup-always-false"

T67_BIN="$WORK/s67-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T67_BIN")"
build_go_or_fail "setup-s67-go-corrupt-build" "$T67_MOD" "$T67_BIN"

T67_PROJECT_DIR_DET="$WORK/s67-project-detection"
mkdir -p "$T67_PROJECT_DIR_DET"
: > "$T67_PROJECT_DIR_DET/CLAUDE.md"

set +e
(cd "$T67_PROJECT_DIR_DET" && HOME="$T67_FAKE_HOME" "$T67_BIN" discover --init) >"$WORK/s67-detection-discover.log" 2>&1
s67d_status=$?
set -e
if [[ $s67d_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/detection-setup]: discover --init (binário corrompido) saiu com $s67d_status" >&2
  cat "$WORK/s67-detection-discover.log" >&2
  falsify_fail_point
fi

s67d_settings="$T67_PROJECT_DIR_DET/.claude/settings.json"
if ! grep -qF 'trackfw-git-branch-guard.sh' "$s67d_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/detection-catches-regression]: com o dedup neutralizado (sempre 'não instalado'), a entrada de projeto deveria REAPARECER mesmo com a fiação global instalada — não reapareceu" >&2
  cat "$s67d_settings" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-dedup/detection-catches-regression]"

# --- braço 4 (ML-2C) — tolerância a "//" no comando gravado no config global ---
# Constrói um HOME sintético com barra dupla EMBUTIDA no meio do caminho
# (reproduzindo exatamente a forma que $TMPDIR/$WORK assume no macOS) e grava
# o comando do PreToolUse[Bash] usando essa forma crua, sem passar por
# filepath.Join — como um config editado à mão, ou capturado quando $HOME
# tinha barra dupla, produziria. globalGitBranchGuardScriptPath() computa o
# caminho já normalizado a partir do MESMO HOME; antes do ML-2C a comparação
# de string crua entre os dois falhava e o dedup não disparava (refs=1); a
# correção normaliza os dois lados antes de comparar (refs=0 esperado).
T67_FAKE_HOME_SLASH="${WORK67_CLEAN}//s67-fake-home-installed-slash"
mkdir -p "$T67_FAKE_HOME_SLASH/.claude"
cat >"$T67_FAKE_HOME_SLASH/.claude/settings.json" <<EOF
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$T67_FAKE_HOME_SLASH/.trackfw/scripts/trackfw-git-branch-guard.sh"}]}]}}
EOF

T67_PROJECT_DIR_SLASH="$WORK67_CLEAN/s67-project-double-slash"
mkdir -p "$T67_PROJECT_DIR_SLASH"
: > "$T67_PROJECT_DIR_SLASH/CLAUDE.md"

set +e
(cd "$T67_PROJECT_DIR_SLASH" && HOME="$T67_FAKE_HOME_SLASH" "$ROOT_DIR/bin/trackfw" discover --init) >"$WORK/s67-slash-discover.log" 2>&1
s67s_status=$?
set -e
if [[ $s67s_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-dedup/double-slash-tolerance-setup]: discover --init saiu com $s67s_status" >&2
  cat "$WORK/s67-slash-discover.log" >&2
  falsify_fail_point
fi

s67s_settings="$T67_PROJECT_DIR_SLASH/.claude/settings.json"
if grep -qF 'trackfw-git-branch-guard.sh' "$s67s_settings"; then
  echo "FAIL [falsify/git-branch-guard-dedup/double-slash-tolerance]: entrada de git-branch-guard presente em $s67s_settings mesmo com // no comando gravado do HOME global — a comparação deveria normalizar antes de comparar" >&2
  cat "$s67s_settings" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-dedup/double-slash-tolerance]"

# ---------------------------------------------------------------------------
# Cenário 68 — internal/validator: "git_branch_guard_script_integrity" (e,
#              não-regressão, "credential_guard_script_integrity") em ESCOPO
#              GLOBAL disparam pela EXISTÊNCIA do artefato em
#              ~/.trackfw/scripts/, não pela fiação (ROADMAP-2026-08-17-
#              guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-
#              independente-de-fiacao, ML-3A).
#
# Ponto cego fechado: antes deste ML, validateGuardGlobalScriptIntegrity só
# avaliava os 6 arquivos de config global QUE REFERENCIAM scriptMarker — sem
# fiação, o laço nunca entrava e a regra nunca rodava. Medido na máquina de
# KG (motivação da REQ): o script global do git-branch-guard ficou 3
# versões atrasado (123 linhas vs 369) com `validate` verde o tempo todo,
# porque nada o cabeava.
#
# $HOME é sempre um diretório sintético isolado (t.TempDir/$WORK-equivalente
# em shell), nunca o real — precedente de vazamento de ambiente é o
# Cenário 46.
# ---------------------------------------------------------------------------
S68_MSG='content diverges from the template this version of trackfw generates'


# --- fixture: $HOME sintético e vazio, 'trackfw update harness' (SEM
# --targets, SEM --install-missing) escreve os dois scripts globais
# incondicionalmente (internal/generators/update.go:489-496) mas não
# instala NENHUM alvo — sem nada já instalado e sem --install-missing, todo
# target reporta "missing" e fica intocado. Resultado: os scripts existem em
# $T68_HOME/.trackfw/scripts/trackfw-{credential,git-branch}-guard.sh e ZERO
# arquivo de config nesse $HOME referencia nenhum dos dois — exatamente o
# estado que fazia a regra antiga nunca disparar. -----------------------------
T68_HOME="$WORK/s68-fake-home"
mkdir -p "$T68_HOME"

(HOME="$T68_HOME" "$ROOT_DIR/bin/trackfw" update harness) >"$WORK/s68-update-harness.log" 2>&1
T68_GBG_SCRIPT="$T68_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
if [[ ! -s "$T68_GBG_SCRIPT" ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/setup]: 'trackfw update harness' não escreveu $T68_GBG_SCRIPT" >&2
  cat "$WORK/s68-update-harness.log" >&2
  falsify_fail_point
fi

for cfg in .claude/settings.json .codex/hooks.json .gemini/settings.json \
           .cursor/hooks.json .copilot/settings.json .kiro/hooks/trackfw-git-branch-guard.json; do
  if [[ -e "$T68_HOME/$cfg" ]]; then
    echo "FAIL [falsify/git-branch-guard-global-script-integrity/setup]: $T68_HOME/$cfg existe — fixture deveria ter ZERO fiação para provar independência da fiação" >&2
    falsify_fail_point
  fi
done

# --- braço baseline: script global íntegro, ZERO fiação -> validate passa --
T68_OK="$WORK/s68-project-ok"
s68_write_project "$T68_OK" git_branch_guard_script_integrity error

set +e
s68ok_out=$(cd "$T68_OK" && HOME="$T68_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
s68ok_status=$?
set -e
if [[ $s68ok_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/baseline]: script global íntegro e SEM fiação deveria passar, saiu com $s68ok_status" >&2
  echo "  output: $s68ok_out" >&2
  falsify_fail_point
fi
if grep -qF "$S68_MSG" <<<"$s68ok_out"; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/baseline]: script global íntegro mas a regra disparou mesmo assim" >&2
  echo "  output: $s68ok_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-global-script-integrity/baseline]"

# --- braço de ausência: $HOME onde NENHUM script foi instalado -> silêncio -
# (não ter rodado 'trackfw update harness' nesse $HOME é estado legítimo, não
# é erro — falso-positivo aqui afetaria todo usuário que nunca instalou o
# harness global) -------------------------------------------------------------
T68_ABSENT_HOME="$WORK/s68-absent-home"
mkdir -p "$T68_ABSENT_HOME"
T68_ABSENT="$WORK/s68-project-absent"
s68_write_project "$T68_ABSENT" git_branch_guard_script_integrity error

set +e
s68absent_out=$(cd "$T68_ABSENT" && HOME="$T68_ABSENT_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
s68absent_status=$?
set -e
if [[ $s68absent_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/absent-is-not-a-violation]: script global nunca instalado ($T68_ABSENT_HOME) não pode reprovar validate, saiu com $s68absent_status" >&2
  echo "  output: $s68absent_out" >&2
  falsify_fail_point
fi
if grep -qF "$S68_MSG" <<<"$s68absent_out"; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/absent-is-not-a-violation]: script global nunca instalado, mas a regra disparou (falso-positivo de ausência)" >&2
  echo "  output: $s68absent_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-global-script-integrity/absent-is-not-a-violation]"

# --- braço de detecção: script global corrompido, ZERO config referenciando
# ele -> validate acusa mesmo assim — o discriminante central deste ML -----
T68_BAD="$WORK/s68-project-bad"
s68_write_project "$T68_BAD" git_branch_guard_script_integrity error
printf '# tampered by check-gates-falsify.sh Cenario 68\n' >> "$T68_GBG_SCRIPT"

assert_fails_with "git-branch-guard-global-script-integrity/detected-without-wiring" \
  "$S68_MSG" \
  bash -c "cd '$T68_BAD' && exec env HOME='$T68_HOME' '$ROOT_DIR/bin/trackfw' validate"

# --- prova de não-vacuidade: mesma árvore corrompida ($T68_HOME já ficou
# corrompido pelo braço acima), regra desligada -> o braço de detecção
# FALHARIA -------------------------------------------------------------------
T68_OFF="$WORK/s68-project-off"
s68_write_project "$T68_OFF" git_branch_guard_script_integrity off

assert_would_now_fail "git-branch-guard-global-script-integrity" \
  "$S68_MSG" \
  bash -c "cd '$T68_OFF' && exec env HOME='$T68_HOME' '$ROOT_DIR/bin/trackfw' validate"

# --- braço de não-duplicação (git-branch-guard): o MESMO script referenciado
# por 2 configs de CLI diferentes (Claude + Codex, via 'update harness
# --install-missing') -> exatamente 1 mensagem, nunca 2. Prova central de
# "sem dupla emissão" agora que git-branch-guard tem DOIS caminhos possíveis
# de disparo (existência do artefato E fiação, desde a Wave 2 deste
# roadmap) -------------------------------------------------------------------
T68_DUP_HOME="$WORK/s68-dup-home-gbg"
mkdir -p "$T68_DUP_HOME"
set +e
(HOME="$T68_DUP_HOME" "$ROOT_DIR/bin/trackfw" update harness \
  --targets claude-git-branch-guard,codex-git-branch-guard,git-branch-guard-script --install-missing) \
  >"$WORK/s68-dup-update-harness-gbg.log" 2>&1
s68dupgbg_setup_status=$?
set -e
if [[ $s68dupgbg_setup_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/no-double-report-setup]: update harness saiu com $s68dupgbg_setup_status" >&2
  cat "$WORK/s68-dup-update-harness-gbg.log" >&2
  falsify_fail_point
fi
T68_DUP_SCRIPT="$T68_DUP_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
if ! grep -qF 'trackfw-git-branch-guard.sh' "$T68_DUP_HOME/.claude/settings.json" || \
   ! grep -qF 'trackfw-git-branch-guard.sh' "$T68_DUP_HOME/.codex/hooks.json"; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/no-double-report-setup]: fiação em Claude E Codex não foi instalada — não é o fixture de 2 configs pretendido" >&2
  falsify_fail_point
fi
printf '# tampered by check-gates-falsify.sh Cenario 68 (dup gbg)\n' >> "$T68_DUP_SCRIPT"

T68_DUP="$WORK/s68-project-dup-gbg"
s68_write_project "$T68_DUP" git_branch_guard_script_integrity error

set +e
s68dup_out=$(cd "$T68_DUP" && HOME="$T68_DUP_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
set -e
s68dup_count=$(grep -oF "$S68_MSG" <<<"$s68dup_out" | wc -l | tr -d ' ')
if [[ "$s68dup_count" -ne 1 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-script-integrity/no-double-report]: esperado exatamente 1 ocorrência da mensagem de integridade (2 configs referenciam o MESMO script), obteve $s68dup_count" >&2
  echo "  output: $s68dup_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-global-script-integrity/no-double-report]"

# --- braço de não-regressão + não-duplicação (credential-guard): mesmo
# padrão acima, mas para o guard que HOJE já é verificado via fiação — prova
# que ele continua sendo verificado (AC5, não-regressão) e que, mesmo
# referenciado por 2 CLIs, não passa a duplicar agora que a regra checa o
# artefato uma única vez por script em vez de uma vez por config ------------
T68_DUP_HOME_CG="$WORK/s68-dup-home-cg"
mkdir -p "$T68_DUP_HOME_CG"
set +e
(HOME="$T68_DUP_HOME_CG" "$ROOT_DIR/bin/trackfw" update harness \
  --targets claude-credential-guard,codex-credential-guard,credential-guard-script --install-missing) \
  >"$WORK/s68-dup-update-harness-cg.log" 2>&1
s68dupcg_setup_status=$?
set -e
if [[ $s68dupcg_setup_status -ne 0 ]]; then
  echo "FAIL [falsify/credential-guard-global-script-integrity/no-double-report-setup]: update harness saiu com $s68dupcg_setup_status" >&2
  cat "$WORK/s68-dup-update-harness-cg.log" >&2
  falsify_fail_point
fi
T68_DUP_SCRIPT_CG="$T68_DUP_HOME_CG/.trackfw/scripts/trackfw-credential-guard.sh"
if ! grep -qF 'trackfw-credential-guard.sh' "$T68_DUP_HOME_CG/.claude/settings.json" || \
   ! grep -qF 'trackfw-credential-guard.sh' "$T68_DUP_HOME_CG/.codex/hooks.json"; then
  echo "FAIL [falsify/credential-guard-global-script-integrity/no-double-report-setup]: fiação em Claude E Codex não foi instalada — não é o fixture de 2 configs pretendido" >&2
  falsify_fail_point
fi
printf '# tampered by check-gates-falsify.sh Cenario 68 (dup cg)\n' >> "$T68_DUP_SCRIPT_CG"

T68_DUP_CG="$WORK/s68-project-dup-cg"
s68_write_project "$T68_DUP_CG" credential_guard_script_integrity error

set +e
s68dupcg_out=$(cd "$T68_DUP_CG" && HOME="$T68_DUP_HOME_CG" "$ROOT_DIR/bin/trackfw" validate 2>&1)
set -e
s68dupcg_count=$(grep -oF "$S68_MSG" <<<"$s68dupcg_out" | wc -l | tr -d ' ')
if [[ "$s68dupcg_count" -ne 1 ]]; then
  echo "FAIL [falsify/credential-guard-global-script-integrity/no-double-report]: esperado exatamente 1 ocorrência (não-regressão + sem duplicar), obteve $s68dupcg_count" >&2
  echo "  output: $s68dupcg_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/credential-guard-global-script-integrity/no-double-report]"

# ---------------------------------------------------------------------------
# Cenário 69 — internal/validator: "git_branch_guard_hook_resolvable" em
#              ESCOPO GLOBAL passa a inspecionar o arquivo DEDICADO do Kiro
#              (~/.kiro/hooks/trackfw-git-branch-guard.json), não apenas
#              ~/.kiro/hooks/trackfw-credential-guard.json
#              (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
#              projeto-e-integridade-independente-de-fiacao, ML-3B).
#
# Ponto cego fechado: antes deste ML, globalGuardConfigFiles (validador, 3
# stacks) só apontava o Kiro para trackfw-credential-guard.json, mesmo para
# a checagem do git-branch-guard — então o arquivo dedicado que a Wave 2
# (ML-2A) passou a escrever para o git-branch-guard do Kiro nunca era lido
# por nenhuma checagem de resolvibilidade: um hook Kiro apontando para
# script ausente/não-executável passava limpo, o mesmo defeito ("instalado
# e não verificado") que esta REQ inteira existe para corrigir.
#
# $HOME é sempre um diretório sintético isolado, nunca o real — precedente
# de vazamento de ambiente é o Cenário 46.
# ---------------------------------------------------------------------------
T69_HOME="$WORK/s69-fake-home"
mkdir -p "$T69_HOME"

set +e
(HOME="$T69_HOME" "$ROOT_DIR/bin/trackfw" update harness \
  --targets kiro-git-branch-guard,kiro-credential-guard,git-branch-guard-script,credential-guard-script --install-missing) \
  >"$WORK/s69-update-harness.log" 2>&1
s69setup_status=$?
set -e
if [[ $s69setup_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/setup]: update harness saiu com $s69setup_status" >&2
  cat "$WORK/s69-update-harness.log" >&2
  falsify_fail_point
fi

T69_GBG_HOOKS="$T69_HOME/.kiro/hooks/trackfw-git-branch-guard.json"
T69_CG_HOOKS="$T69_HOME/.kiro/hooks/trackfw-credential-guard.json"
T69_GBG_SCRIPT="$T69_HOME/.trackfw/scripts/trackfw-git-branch-guard.sh"
if [[ ! -s "$T69_GBG_HOOKS" ]] || [[ ! -s "$T69_CG_HOOKS" ]] || [[ ! -x "$T69_GBG_SCRIPT" ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/setup]: fixture incompleta — esperava $T69_GBG_HOOKS, $T69_CG_HOOKS e $T69_GBG_SCRIPT (executável)" >&2
  falsify_fail_point
fi

# --- braço baseline: os dois arquivos dedicados do Kiro, ambos apontando
# para scripts presentes e executáveis -> validate passa em silêncio ------
T69_OK="$WORK/s69-project-ok"
scaffold_adr_req_project "$T69_OK"

set +e
s69ok_out=$(cd "$T69_OK" && HOME="$T69_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1)
s69ok_status=$?
set -e
if [[ $s69ok_status -ne 0 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline]: fiação Kiro íntegra deveria passar, saiu com $s69ok_status" >&2
  echo "  output: $s69ok_out" >&2
  falsify_fail_point
fi
if grep -qF 'trackfw-git-branch-guard.json' <<<"$s69ok_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline]: fiação Kiro íntegra mas a regra disparou mesmo assim" >&2
  echo "  output: $s69ok_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline]"

# --- braço de detecção: script referenciado pelo arquivo DEDICADO do Kiro
# some do disco -> validate deve acusar, citando o arquivo do Kiro — o
# discriminante central deste ML (antes dele, esta checagem nunca lia esse
# arquivo, então nenhuma acusação era possível aqui) -----------------------
rm -f "$T69_GBG_SCRIPT"

T69_BAD="$WORK/s69-project-bad"
scaffold_adr_req_project "$T69_BAD"

assert_fails_with "git-branch-guard-global-hook-resolvable/kiro-dedicated-file/detected" \
  "does not exist" \
  bash -c "cd '$T69_BAD' && exec env HOME='$T69_HOME' '$ROOT_DIR/bin/trackfw' validate"

s69bad_out=$(cd "$T69_BAD" && HOME="$T69_HOME" "$ROOT_DIR/bin/trackfw" validate 2>&1 || true)
if ! grep -qF 'trackfw-git-branch-guard.json' <<<"$s69bad_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/detected]: mensagem não cita o arquivo dedicado do Kiro (trackfw-git-branch-guard.json)" >&2
  echo "  output: $s69bad_out" >&2
  falsify_fail_point
fi
if ! grep -qF 'Kiro' <<<"$s69bad_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/detected]: mensagem não cita o CLI (Kiro)" >&2
  echo "  output: $s69bad_out" >&2
  falsify_fail_point
fi

# --- não-regressão + não-duplicação: com o script do git-branch-guard
# ausente, o credential-guard do Kiro (arquivo separado, script intacto)
# continua em silêncio, E a violation do git-branch-guard aparece exatamente
# 1 vez (não uma vez por arquivo/guard) -------------------------------------
s69bad_gbg_count=$(grep -oF 'trackfw-git-branch-guard.json' <<<"$s69bad_out" | wc -l | tr -d ' ')
if [[ "$s69bad_gbg_count" -ne 1 ]]; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/no-double-report]: esperado exatamente 1 ocorrência da violation do Kiro, obteve $s69bad_gbg_count" >&2
  echo "  output: $s69bad_out" >&2
  falsify_fail_point
fi
if grep -qF 'trackfw-credential-guard.json' <<<"$s69bad_out"; then
  echo "FAIL [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/no-regression]: credential-guard do Kiro (arquivo intacto) não deveria disparar, mas apareceu na saída" >&2
  echo "  output: $s69bad_out" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/no-double-report-and-no-regression]"

# ---------------------------------------------------------------------------
# Cenário 74 — scripts/trackfw-git-branch-guard.sh (ML-3A, ROADMAP-2026-08-19-
# caminho-governado-para-push-forcado-e-tag-de-release.md / REQ-2026-08-19-
# guard-nao-bloqueia-comandos-destrutivos-de-working-tree-em-repo-compartilhado-
# por-agentes.md) — a nova classe de comandos destrutivos de working tree
# (stash/reset --hard/clean -f|-x/restore <path>/checkout -- <path>|checkout .)
# ganha um par baseline+detecção POR COMANDO, cobrindo as DUAS direções que
# a REQ nomeia como risco: (a) o comando FICA bloqueado — corrompe o rótulo
# do case que reconhece o subcomando, provando que o bloqueio depende do
# literal novo, não de coincidência; (b) o comando LIBERADO continua livre —
# corrompe o discriminante que separa a forma segura da perigosa (o próprio
# risco DOMINANTE que a REQ nomeia: super-bloquear é pior que sub-bloquear),
# provando que a liberação também depende de um literal específico, não de
# um match "solto" que por acaso deixa passar. Mesmo padrão baseline+detecção
# dos Cenários 60-65: braço baseline prova que o guard LIMPO se comporta como
# esperado nos dois sentidos; braço de detecção corrompe UM literal isolado
# do gerador Go, reconstrói o script a partir de um módulo Go isolado, e
# prova que a mudança de comportamento é exatamente a esperada.
# ---------------------------------------------------------------------------

T74_BASE_MOD="$WORK/s74-base-mod"
mkdir -p "$T74_BASE_MOD/cmd" "$T74_BASE_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74_BASE_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74_BASE_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74_BASE_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74_BASE_MOD/go.sum"

T74_BASE_OUT="$WORK/s74-base-out"
run_go_guard_dump "setup-s74-go-baseline-build" "$T74_BASE_MOD" "$T74_BASE_OUT"
T74_BASE_SCRIPT="$T74_BASE_OUT/scripts/trackfw-git-branch-guard.sh"

# --- 74a — git stash: bare/push/save/clear/drop bloqueiam, list/show livres ---
assert_guard_exit "git-branch-guard/stash/baseline-blocks-bare" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash"}}' 2
assert_guard_exit "git-branch-guard/stash/baseline-blocks-drop" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash drop"}}' 2
assert_guard_exit "git-branch-guard/stash/baseline-frees-list" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash list"}}' 0
assert_guard_exit "git-branch-guard/stash/baseline-frees-show" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git stash show"}}' 0

T74A_MOD="$WORK/s74a-mod"
mkdir -p "$T74A_MOD/cmd" "$T74A_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74A_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74A_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74A_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74A_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74A_MOD/internal/generators/scaffold.go" \
  '      stash)
' \
  '      __never_matches_s74a__)
' \
  "s74a-go-stash-case-label-removed"
T74A_OUT="$WORK/s74a-out"
run_go_guard_dump "setup-s74a-go-corrupted-build" "$T74A_MOD" "$T74A_OUT"
assert_guard_exit "git-branch-guard/stash/detection-catches-bypass" \
  "$T74A_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git stash"}}' 0

T74B_MOD="$WORK/s74b-mod"
mkdir -p "$T74B_MOD/cmd" "$T74B_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74B_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74B_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74B_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74B_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74B_MOD/internal/generators/scaffold.go" \
  '          list|show)
' \
  '          __never_matches_s74b__)
' \
  "s74b-go-stash-allowlist-removed"
T74B_OUT="$WORK/s74b-out"
run_go_guard_dump "setup-s74b-go-corrupted-build" "$T74B_MOD" "$T74B_OUT"
assert_guard_exit "git-branch-guard/stash/detection-catches-overblock-list" \
  "$T74B_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git stash list"}}' 2

# --- 74c — git reset --hard bloqueia; --soft/--mixed/sem flag livres --------
assert_guard_exit "git-branch-guard/reset-hard/baseline-blocks" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git reset --hard"}}' 2
assert_guard_exit "git-branch-guard/reset-hard/baseline-frees-soft" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git reset --soft HEAD~1"}}' 0
assert_guard_exit "git-branch-guard/reset-hard/baseline-frees-bare" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git reset"}}' 0

T74C_MOD="$WORK/s74c-mod"
mkdir -p "$T74C_MOD/cmd" "$T74C_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74C_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74C_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74C_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74C_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74C_MOD/internal/generators/scaffold.go" \
  '      reset)
' \
  '      __never_matches_s74c__)
' \
  "s74c-go-reset-case-label-removed"
T74C_OUT="$WORK/s74c-out"
run_go_guard_dump "setup-s74c-go-corrupted-build" "$T74C_MOD" "$T74C_OUT"
assert_guard_exit "git-branch-guard/reset-hard/detection-catches-bypass" \
  "$T74C_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git reset --hard"}}' 0

T74D_MOD="$WORK/s74d-mod"
mkdir -p "$T74D_MOD/cmd" "$T74D_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74D_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74D_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74D_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74D_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74D_MOD/internal/generators/scaffold.go" \
  '            --hard)
' \
  '            *)
' \
  "s74d-go-reset-hard-discriminant-widened"
T74D_OUT="$WORK/s74d-out"
run_go_guard_dump "setup-s74d-go-corrupted-build" "$T74D_MOD" "$T74D_OUT"
assert_guard_exit "git-branch-guard/reset-hard/detection-catches-overblock-soft" \
  "$T74D_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git reset --soft HEAD~1"}}' 2

# --- 74e — git clean -f/-x bloqueia; -n/--dry-run livre (inclusive quando -n
# aparece JUNTO com -f: -n vence, git nunca apaga nada com --dry-run presente) --
assert_guard_exit "git-branch-guard/clean-force/baseline-blocks-f" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git clean -fd"}}' 2
assert_guard_exit "git-branch-guard/clean-force/baseline-frees-dry-run" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git clean -n"}}' 0
assert_guard_exit "git-branch-guard/clean-force/baseline-frees-dry-run-plus-force" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git clean -n -f"}}' 0

T74E_MOD="$WORK/s74e-mod"
mkdir -p "$T74E_MOD/cmd" "$T74E_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74E_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74E_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74E_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74E_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74E_MOD/internal/generators/scaffold.go" \
  '      clean)
' \
  '      __never_matches_s74e__)
' \
  "s74e-go-clean-case-label-removed"
T74E_OUT="$WORK/s74e-out"
run_go_guard_dump "setup-s74e-go-corrupted-build" "$T74E_MOD" "$T74E_OUT"
assert_guard_exit "git-branch-guard/clean-force/detection-catches-bypass" \
  "$T74E_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git clean -fd"}}' 0

T74F_MOD="$WORK/s74f-mod"
mkdir -p "$T74F_MOD/cmd" "$T74F_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74F_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74F_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74F_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74F_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74F_MOD/internal/generators/scaffold.go" \
  '            -n|--dry-run)
' \
  '            __never_matches_s74f__)
' \
  "s74f-go-clean-dry-run-guard-removed"
T74F_OUT="$WORK/s74f-out"
run_go_guard_dump "setup-s74f-go-corrupted-build" "$T74F_MOD" "$T74F_OUT"
assert_guard_exit "git-branch-guard/clean-force/detection-catches-overblock-dry-run-plus-force" \
  "$T74F_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git clean -n -f"}}' 2

# --- 74g — git restore <path> bloqueia; --staged livre ----------------------
assert_guard_exit "git-branch-guard/restore-path/baseline-blocks" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git restore foo.txt"}}' 2
assert_guard_exit "git-branch-guard/restore-path/baseline-frees-staged" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git restore --staged foo.txt"}}' 0

T74G_MOD="$WORK/s74g-mod"
mkdir -p "$T74G_MOD/cmd" "$T74G_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74G_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74G_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74G_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74G_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74G_MOD/internal/generators/scaffold.go" \
  '      restore)
' \
  '      __never_matches_s74g__)
' \
  "s74g-go-restore-case-label-removed"
T74G_OUT="$WORK/s74g-out"
run_go_guard_dump "setup-s74g-go-corrupted-build" "$T74G_MOD" "$T74G_OUT"
assert_guard_exit "git-branch-guard/restore-path/detection-catches-bypass" \
  "$T74G_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore foo.txt"}}' 0

T74H_MOD="$WORK/s74h-mod"
mkdir -p "$T74H_MOD/cmd" "$T74H_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74H_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74H_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74H_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74H_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74H_MOD/internal/generators/scaffold.go" \
  '            --staged)
' \
  '            __never_matches_s74h__)
' \
  "s74h-go-restore-staged-guard-removed"
T74H_OUT="$WORK/s74h-out"
run_go_guard_dump "setup-s74h-go-corrupted-build" "$T74H_MOD" "$T74H_OUT"
assert_guard_exit "git-branch-guard/restore-path/detection-catches-overblock-staged" \
  "$T74H_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore --staged foo.txt"}}' 2

# --- 74h-bis — --staged NUNCA basta sozinho para liberar quando --worktree/-W
# também aparece: "--staged --worktree" restaura os DOIS (afeta o working
# tree), então deve continuar bloqueado mesmo com --staged presente — achado
# do arquiteto (hades-tf/advisor): "git restore --staged" sozinho é o único
# caso liberado pela REQ, não "qualquer --staged".
assert_guard_exit "git-branch-guard/restore-path/baseline-blocks-staged-plus-worktree" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git restore --staged --worktree foo.txt"}}' 2

T74H2_MOD="$WORK/s74h2-mod"
mkdir -p "$T74H2_MOD/cmd" "$T74H2_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74H2_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74H2_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74H2_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74H2_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74H2_MOD/internal/generators/scaffold.go" \
  '            --worktree|-W)
' \
  '            __never_matches_s74h2__)
' \
  "s74h2-go-restore-worktree-discriminant-removed"
T74H2_OUT="$WORK/s74h2-out"
run_go_guard_dump "setup-s74h2-go-corrupted-build" "$T74H2_MOD" "$T74H2_OUT"
assert_guard_exit "git-branch-guard/restore-path/detection-catches-underblock-staged-plus-worktree" \
  "$T74H2_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore --staged --worktree foo.txt"}}' 0

# Auto-discriminação: contra o MESMO build corrompido, "--staged" sozinho
# (sem --worktree) precisa continuar livre — prova que a corrupção isola só o
# discriminante --worktree/-W, não a liberação de --staged como um todo.
assert_guard_exit "git-branch-guard/restore-path/detection-does-not-break-staged-alone" \
  "$T74H2_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git restore --staged foo.txt"}}' 0

# --- 74i — git checkout -- <path> | checkout . bloqueia; checkout <branch> livre
assert_guard_exit "git-branch-guard/checkout-path/baseline-blocks-dashdash" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git checkout -- foo.txt"}}' 2
assert_guard_exit "git-branch-guard/checkout-path/baseline-blocks-dot" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git checkout ."}}' 2
assert_guard_exit "git-branch-guard/checkout-path/baseline-frees-branch" \
  "$T74_BASE_SCRIPT" '{"tool_input":{"command":"git checkout main"}}' 0

T74I_MOD="$WORK/s74i-mod"
mkdir -p "$T74I_MOD/cmd" "$T74I_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74I_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74I_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74I_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74I_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74I_MOD/internal/generators/scaffold.go" \
  '            --|.)
' \
  '            __never_matches_s74i__)
' \
  "s74i-go-checkout-path-discriminant-removed"
T74I_OUT="$WORK/s74i-out"
run_go_guard_dump "setup-s74i-go-corrupted-build" "$T74I_MOD" "$T74I_OUT"
assert_guard_exit "git-branch-guard/checkout-path/detection-catches-bypass" \
  "$T74I_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout -- foo.txt"}}' 0

T74J_MOD="$WORK/s74j-mod"
mkdir -p "$T74J_MOD/cmd" "$T74J_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T74J_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T74J_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T74J_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T74J_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/generators/scaffold.go" "$T74J_MOD/internal/generators/scaffold.go" \
  '            --|.)
' \
  '            *)
' \
  "s74j-go-checkout-path-discriminant-widened"
T74J_OUT="$WORK/s74j-out"
run_go_guard_dump "setup-s74j-go-corrupted-build" "$T74J_MOD" "$T74J_OUT"
assert_guard_exit "git-branch-guard/checkout-path/detection-catches-overblock-branch" \
  "$T74J_OUT/scripts/trackfw-git-branch-guard.sh" \
  '{"tool_input":{"command":"git checkout main"}}' 2

# ---------------------------------------------------------------------------
# Cenário 75 — check-release-tag-parity.sh (ML-2B, ROADMAP-2026-08-19-caminho-governado-para-
# push-forcado-e-tag-de-release.md) is falsifiable against the exact regression the ADR exists to
# prevent: an annotated tag silently degrading to a LIGHTWEIGHT tag — the ref pointing straight
# at the commit instead of at the tag object created by the first `gh api` call
# (internal/commands/release.go's `refPayload` marshal, the single `SHA: tagObj.SHA` field). A
# single-literal corruption in an isolated Go copy (`SHA: tagObj.SHA` → `SHA: objectSHA`),
# exercised end to end through the real `release tag` command by the gate's own scenario
# "success" fixture: the gate's `gh` stub returns a tag-object sha deliberately different from
# the fixture's commit sha, so the ref-creation payload's `sha` field is the discriminant — the
# correct binary links it to the tag-object sha, the sabotaged binary links it to the commit sha
# directly (a lightweight tag wearing an annotated tag's success message: `git describe`/`git tag
# -l` would still find it, and the loss is invisible until someone looks for the message on the
# tag object). Baseline arm proves check-release-tag-parity.sh passes clean against the
# unmodified Go binary before the paired detection arm proves the single-literal corruption makes
# it fail — same single-delta design as Cenário 73.
# ---------------------------------------------------------------------------
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s75]: check-release-tag-parity.sh failed against the UNMODIFIED Go binary — baseline must be green before the detection arm means anything" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/release-tag-parity/success/baseline-clean]"

T75C_GO_MOD="$WORK/s75-corrupt-go"
mkdir -p "$T75C_GO_MOD/cmd" "$T75C_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T75C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T75C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T75C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T75C_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" "$T75C_GO_MOD/internal/commands/release.go" \
  '}{Ref: "refs/tags/" + tagName, SHA: tagObj.SHA})' \
  '}{Ref: "refs/tags/" + tagName, SHA: objectSHA})' \
  "s75-go"

T75C_GO_BIN="$WORK/s75-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T75C_GO_BIN")"
build_go_or_fail "setup-s75-go-corrupt-build" "$T75C_GO_MOD" "$T75C_GO_BIN"

assert_fails_with "release-tag-parity/success-lightweight-tag-false-negative" \
  "LIGHTWEIGHT-TAG REGRESSION: ref payload 'sha' must equal the tag-object sha" \
  env GO_BIN="$T75C_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 76 — check-release-tag-parity.sh's Scenarios 11-13 (ML-4B, ROADMAP-2026-08-19-
# caminho-governado-para-push-forcado-e-tag-de-release.md, Emenda 1 do ADR) are falsifiable
# against the exact regression Emenda 1 exists to prevent: the commit-target divergence check
# silently disabled, so `release tag` falls back to trusting a LOCAL (forged/stale) ref instead
# of refusing when it disagrees with the forge. A single-literal corruption in an isolated Go
# copy — internal/commands/release.go's `if forgeLocalSHA != "" && forgeLocalSHA != commitObj.SHA {`
# guarded with a `false &&` prefix that never evaluates true — neuters the divergence check
# without touching the forge GET calls or the objectSHA assignment themselves, so the corrupted
# binary still resolves and even PRINTS the forge's sha correctly; only the refusal is gone.
# Exercised end to end through the real `release tag` command by the gate's own Scenario 12
# fixture (origin/main forged via `git update-ref` under a narrowed refspec, refs/heads/main
# reset to match it so the pre-existing local-branch-staleness check cannot discriminate this
# corruption either) — baseline arm proves check-release-tag-parity.sh passes clean against the
# unmodified Go binary before the paired detection arm proves the single-literal corruption
# makes it fail. Same single-delta design as Cenários 73/75.
# ---------------------------------------------------------------------------
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s76]: check-release-tag-parity.sh failed against the UNMODIFIED Go binary — baseline must be green before the detection arm means anything" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/release-tag-parity/forge-commit-diverges-update-ref/baseline-clean]"

T76_GO_MOD="$WORK/s76-corrupt-go"
mkdir -p "$T76_GO_MOD/cmd" "$T76_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$T76_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T76_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T76_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T76_GO_MOD/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" "$T76_GO_MOD/internal/commands/release.go" \
  'if forgeLocalSHA != "" && forgeLocalSHA != commitObj.SHA {' \
  'if false && forgeLocalSHA != "" && forgeLocalSHA != commitObj.SHA {' \
  "s76-go"

T76_GO_BIN="$WORK/s76-corrupt-go-bin/trackfw"
mkdir -p "$(dirname "$T76_GO_BIN")"
build_go_or_fail "setup-s76-go-corrupt-build" "$T76_GO_MOD" "$T76_GO_BIN"

assert_fails_with "release-tag-parity/forge-commit-diverges-false-negative" \
  "expected non-zero exit when origin/main is forged via update-ref" \
  env GO_BIN="$T76_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

# ---------------------------------------------------------------------------
# Cenário 77 — scripts/check-parity-contract-coverage.sh (ROADMAP-2026-08-20-
# contrato-pinado-no-cli-parity-sem-gate-nomeado.md, ML-1B) — gate created by
# this ML without its own falsification scenario, closing the reported gap:
# a meta-checker without a P4 scenario would be the exact irony the REQ
# exists to prevent one level up ("gate sem cenário de falsificação é gate
# não-verificado").
#
# Baseline fixture covers all 3 heading levels (##/###/####) and all 4 valid
# annotation shapes (gate=, gate=+partial=, gap reason=, none reason=) plus
# one unannotated section, proving the report-mode counts are honest (the
# real docs/cli-parity.md today has zero `none` sections, so that counting
# path is otherwise unexercised). Six single-delta corruptions off that same
# baseline prove each of the 5 documented failure classes plus the unknown-
# key/malformed-prefix parsing rule from the ADR's Emenda 1/Nota de parsing.
# Non-vacuity: neutering the gate-existence check on an isolated copy proves
# the "gate nomeado inexistente" detection arm actually depends on it.
# ---------------------------------------------------------------------------
T77="$WORK/s77"
mkdir -p "$T77/scripts"
cp -r "$ROOT_DIR/scripts/." "$T77/scripts/"


# --- 77a — baseline: as 4 formas válidas anotadas, exit 0 -------------------
# ML-3A (2026-08-20): a 5ª seção ("Unannotated section") que existia aqui até
# a triagem fechar foi REMOVIDA do fixture compartilhado — ela agora reprova
# (ver 77p), e um baseline que se propõe a passar com exit 0 não pode conter
# uma seção que o próprio ML-3A tornou reprovável, ou o baseline de 77b-77o
# passaria a testar "reprova, mas pelo motivo errado" sem que ninguém notasse.
# A cobertura de "seção sem anotação" ganhou fixture dedicado em 77p.
T77A="$T77/s77a-baseline.md"
write_s77_fixture "$T77A" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=cobre so a mecanica -->" \
  "<!-- trackfw-contract: gap reason=nada protege isto ainda -->" \
  "<!-- trackfw-contract: none reason=e so prosa de contexto -->"

T77A_OUT="$WORK/s77a-out"
set +e
T77A_STDOUT=$(bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77A" 2>"$T77A_OUT")
T77A_STATUS=$?
set -e
if [[ $T77A_STATUS -ne 0 ]]; then
  echo "FAIL [falsify/parity-contract-coverage/baseline]: saiu com $T77A_STATUS, esperava 0" >&2
  echo "  stdout: $T77A_STDOUT" >&2
  echo "  stderr: $(cat "$T77A_OUT")" >&2
  falsify_fail_point
fi
for expected in \
  "total de seções reais (##/###/####), fora de fences: 4  (## 2 · ### 1 · #### 1)" \
  "gate= (cobertura plena):  1" \
  "gate= com partial=:       1" \
  "gap (contrato SEM gate):  1" \
  "none (não-contrato):      1" \
  "sem anotação:             0" \
  "anotação inválida:        0"; do
  if ! grep -qF "$expected" <<<"$T77A_STDOUT"; then
    echo "FAIL [falsify/parity-contract-coverage/baseline]: relatório não contém '$expected'" >&2
    echo "  stdout: $T77A_STDOUT" >&2
    falsify_fail_point
  fi
done
falsify_count_success
echo "OK   [falsify/parity-contract-coverage/baseline]: 3 níveis de título + 4 estados válidos, todas anotadas, contagens corretas"

# --- 77b — gate= sem caminho nomeado (vazio) — regra GERAL da Emenda 2:
#           chave PRESENTE com valor vazio reprova, mensagem nomeia a chave --
T77B="$T77/s77b-gate-empty.md"
write_s77_fixture "$T77B" \
  "<!-- trackfw-contract: gate= -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/gate-empty" \
  "gate= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77B"

# --- 77c — gate nomeado que não existe no disco — aponta para o vazio -----
T77C="$T77/s77c-gate-missing.md"
write_s77_fixture "$T77C" \
  "<!-- trackfw-contract: gate=scripts/does-not-exist-anywhere.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/gate-missing-on-disk" \
  "gate nomeado não existe no disco: scripts/does-not-exist-anywhere.sh" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77C"

# --- 77d — gap sem reason= --------------------------------------------------
T77D="$T77/s77d-gap-no-reason.md"
write_s77_fixture "$T77D" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/gap-no-reason" \
  "gap sem reason= (motivo obrigatório)" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77D"

# --- 77e — none sem reason= -------------------------------------------------
T77E="$T77/s77e-none-no-reason.md"
write_s77_fixture "$T77E" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none -->"
assert_fails_with "parity-contract-coverage/none-no-reason" \
  "none sem reason= (motivo obrigatório)" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77E"

# --- 77f — chave desconhecida (Nota de parsing da Emenda 1: `reson=` não
#           vira parte do valor anterior em silêncio) -----------------------
T77F="$T77/s77f-unknown-key.md"
write_s77_fixture "$T77F" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap reson=motivo com erro de digitação -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/unknown-key" \
  "chave desconhecida na anotação" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77F"

# --- 77g — anotação malformada: prefixo trackfw-contract: sem estado
#           reconhecido (nem gate=, nem gap, nem none) ---------------------
T77G="$T77/s77g-malformed.md"
write_s77_fixture "$T77G" \
  "<!-- trackfw-contract: estado-nao-reconhecido foo -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/malformed-state" \
  "prefixo trackfw-contract sem estado reconhecido" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77G"

# --- 77h — não-vacuidade: neutraliza a checagem de existência do gate numa
#           cópia isolada do checker e prova que o caso 77c fica mudo -------
T77H_SCRIPT="$T77/scripts/check-parity-contract-coverage-neutered.sh"
corrupt_literal \
  "$T77/scripts/check-parity-contract-coverage.sh" "$T77H_SCRIPT" \
  'if not os.path.isfile(os.path.join(ROOT, path)):' \
  'if False:' \
  "s77h-neuter-gate-existence-check"
chmod +x "$T77H_SCRIPT"
assert_succeeds "parity-contract-coverage/gate-missing/non-vacuity" \
  bash "$T77H_SCRIPT" "$T77C"

# --- 77i — partial= presente com valor vazio (ADR Emenda 2, 6º caso de
#           reprovação): a regra é GERAL — não é um `if` dedicado a
#           `partial`, é o mesmo laço "toda chave presente exige valor
#           não-vazio" que 77b já exercita para `gate=`. Este cenário prova
#           que a MESMA regra também pega `partial=` sem precisar de código
#           novo por chave — o que 77b sozinho não provaria. -----------------
T77I="$T77/s77i-partial-empty.md"
write_s77_fixture "$T77I" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial= -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/partial-empty" \
  "partial= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77I"

# --- 77j — reason= presente com valor vazio no estado `gap`, distinto de
#           reason= AUSENTE (77d testa a chave nem escrita; aqui a chave
#           está escrita e vazia) — single-delta: só o `gap_line` muda em
#           relação ao baseline 77a. -----------------------------------------
T77J="$T77/s77j-reason-empty-gap.md"
write_s77_fixture "$T77J" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason= -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/reason-empty/gap" \
  "reason= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77J"

# --- 77k — mesmo caso, estado `none` (77e testa a chave AUSENTE; este
#           fixture prova que o laço geral também cobre o outro dos dois
#           estados que compartilham o branch `state in ("gap", "none")`,
#           não só `gap`) — single-delta relativo ao baseline. --------------
T77K="$T77/s77k-reason-empty-none.md"
write_s77_fixture "$T77K" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=x -->" \
  "<!-- trackfw-contract: none reason= -->"
assert_fails_with "parity-contract-coverage/reason-empty/none" \
  "reason= presente com valor vazio" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77K"

# --- 77l — não-vacuidade da regra GERAL da Emenda 2: neutraliza o laço
#           "toda chave presente exige valor não-vazio" numa cópia isolada
#           do checker (o mesmo laço que 77i/77j/77k dependem, não um `if`
#           por chave) e prova que 77i fica mudo — isolando que é o LAÇO,
#           não os checks específicos de gate=/reason= já removidos deste
#           checker, quem sustenta a detecção. --------------------------------
T77L_SCRIPT="$T77/scripts/check-parity-contract-coverage-empty-neutered.sh"
corrupt_literal \
  "$T77/scripts/check-parity-contract-coverage.sh" "$T77L_SCRIPT" \
  'for key in sorted(kv):' \
  'for key in []:' \
  "s77l-neuter-general-empty-check"
chmod +x "$T77L_SCRIPT"
assert_succeeds "parity-contract-coverage/empty-value/non-vacuity" \
  bash "$T77L_SCRIPT" "$T77I"

# --- 77m — chave desconhecida DEPOIS de uma chave real, dentro do valor
#           dela (ML-1B-ter, conformidade sobre 77f: aquele fixture só prova
#           chave desconhecida ANTES da primeira chave real — 'reson=' logo
#           após 'gap ', sem nenhuma chave reconhecida vindo antes dele no
#           corpo. Aqui 'reson=' vem DEPOIS de 'reason=', posição que
#           extract_kv() engolia em silêncio como parte do valor de
#           'reason=' antes deste ML — exatamente o cenário que a Nota de
#           parsing do ADR descreve como o motivo de existir a regra.) ------
T77M="$T77/s77m-unknown-key-positional.md"
write_s77_fixture "$T77M" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=motivo qualquer reson=erro de digitacao -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_fails_with "parity-contract-coverage/unknown-key/positional-after-known-key" \
  "chave desconhecida na anotação: 'reson='" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77M"

# --- 77n — controle de não-regressão do heurístico do 77m: texto livre
#           contendo '=' que NÃO é typo de gate=/partial=/reason= não pode
#           reprovar — 'LANG=' (maiúsculo, nunca bate no heurístico) e
#           '--flag=' (o 'f' de 'flag' é precedido por '-', não por espaço,
#           então nunca é considerado candidato a chave). Ambos citados no
#           handoff do ML como o risco central deste heurístico: reprovar um
#           motivo legítimo é pior que deixar passar um typo raro. ---------
T77N="$T77/s77n-freetext-equals-ok.md"
write_s77_fixture "$T77N" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh -->" \
  "<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=x -->" \
  "<!-- trackfw-contract: gap reason=o comportamento sob LANG=pt_BR roda com --flag=valor e nao e comparado entre os 3 CLIs -->" \
  "<!-- trackfw-contract: none reason=x -->"
assert_succeeds "parity-contract-coverage/unknown-key/freetext-equals-non-regression" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77N"

# --- 77o — não-vacuidade do 77m: neutraliza o laço find_unknown_key_typos()
#           numa cópia isolada do checker (o mecanismo que 77m depende, e
#           que 77f sozinho NÃO exercitaria, já que 77f é pego pelo
#           `leading` antigo) e prova que 77m fica mudo. -------------------
T77O_SCRIPT="$T77/scripts/check-parity-contract-coverage-positional-neutered.sh"
corrupt_literal \
  "$T77/scripts/check-parity-contract-coverage.sh" "$T77O_SCRIPT" \
  'for token in sorted(set(find_unknown_key_typos(body, positions))):' \
  'for token in sorted(set([])):' \
  "s77o-neuter-positional-unknown-key-check"
chmod +x "$T77O_SCRIPT"
assert_succeeds "parity-contract-coverage/unknown-key/positional-non-vacuity" \
  bash "$T77O_SCRIPT" "$T77M"

# --- 77p — ML-3A: seção nova SEM anotação agora reprova (a triagem fechou,
#           177/177 — deixou de ser "ainda não triada" e passou a ser
#           regressão). Single-delta contra o baseline 77a: acrescenta UM
#           cabeçalho a mais, sem anotação nenhuma, e prova que só ele muda
#           o resultado de exit 0 para exit 1. Braço de baseline embutido:
#           77a acima já prova que as 4 formas válidas, sozinhas, passam —
#           este cenário isola que é especificamente a AUSÊNCIA de anotação
#           na 5ª seção que derruba o exit code, não alguma outra diferença
#           entre os fixtures. -------------------------------------------
T77P="$T77/s77p-new-section-unannotated.md"
cat "$T77A" > "$T77P"
cat >> "$T77P" <<'EOF'

## Seção nova sem passar pela triagem

Prosa qualquer — ninguém anotou esta seção. Antes do ML-3A isso só entrava
no relatório; agora é regressão e precisa reprovar.
EOF
assert_fails_with "parity-contract-coverage/unannotated-section/blocks-since-ml3a" \
  "seção sem anotação trackfw-contract" \
  bash "$T77/scripts/check-parity-contract-coverage.sh" "$T77P"


# Cenário 85 — nil map em ProjectConfig.AgentModels: parse() sem initConfigMaps()
#              causa panic com "assignment to entry in nil map" quando
#              ParseRulesFromContent é chamado com conteúdo contendo agent_models:.
#
# Objetivo (P4, ML-2C ROADMAP-2026-08-21-versao-do-modelo-por-tier-com-
# composicao-por-alvo): provar que a chamada initConfigMaps(cfg) no início de
# parse() é indispensável. Sem ela, ParseRulesFromContent cria um
# ProjectConfig{Rules: make(...)} com AgentModels nil; parse() então escreve
# cfg.AgentModels[k] = s → panic "assignment to entry in nil map".
#
# Estratégia: corromper config.go removendo a chamada initConfigMaps(cfg), e
# executar o próprio TestParseRulesFromContentWithAgentModels_NoPanic na cópia
# corrompida via `go test`. Ao contrário de execução via CLI + git HEAD
# (que depende de git subprocess), o `go test` exerce a função diretamente —
# sem dependência de ambiente.
#
# Seam: internal/config/config.go — remover a chamada initConfigMaps(cfg)
# na primeira linha de parse(). A função initConfigMaps permanece (sem erro
# "declared and not used" para reflect), mas não é chamada — suficiente para
# restaurar o nil map.
# ---------------------------------------------------------------------------
T85="$WORK/s85"
mkdir -p "$T85/cmd" "$T85/internal"
cp -r "$ROOT_DIR/cmd/." "$T85/cmd/"
cp -r "$ROOT_DIR/internal/." "$T85/internal/"
cp "$ROOT_DIR/go.mod" "$T85/go.mod"
cp "$ROOT_DIR/go.sum" "$T85/go.sum"

sed 's/\tinitConfigMaps(cfg) \/\/ guarantee: all map fields are non-nil before any write/\t\/\/ [falsified] initConfigMaps(cfg) removed — nil map panic restored/' \
  "$ROOT_DIR/internal/config/config.go" > "$T85/internal/config/config.go"

if cmp -s "$ROOT_DIR/internal/config/config.go" "$T85/internal/config/config.go"; then
  echo "FAIL [falsify/setup-s85]: sed não alterou config.go — padrão não encontrado; prova P4 inválida" >&2
  falsify_fail_point
fi

# Liveness check: o módulo corrompido ainda compila (initConfigMaps existe mas
# não é chamada — reflect continua usado).
T85_BIN="$WORK/s85-bin/trackfw"
mkdir -p "$(dirname "$T85_BIN")"
build_go_or_fail "setup-s85-liveness-build" "$T85" "$T85_BIN"

# Braço de baseline: go test passes no código real
if ! (cd "$ROOT_DIR" && env GOCACHE="$WORK/go-build-cache" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/config/ -run TestParseRulesFromContentWithAgentModels_NoPanic) >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s85-baseline]: go test falhou no código real — prova P4 inválida" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/nil-map-init/parse-with-agent-models-nopanic-baseline]"

# Braço de detecção: go test panica na cópia corrompida
assert_fails_with "nil-map-init/parse-missing-causes-panic-on-agent-models" \
  "assignment to entry in nil map" \
  bash -c "cd \"$T85\" && env GOCACHE=\"$WORK/go-build-cache\" TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/config/ -run TestParseRulesFromContentWithAgentModels_NoPanic"

# Cenário 86 — namespace leak via remoção da guarda de allowlist em render.go:
#              a condição `targetID == "claude" && len(agentModels) > 0` protege
#              todos os targets não-Claude de receberem model IDs compostos quando
#              agent_models está configurado.  Sem ela, targets como Gemini (que
#              usam o case `default:` do switch de representação) recebem o valor
#              composto (ex.: "model: claude-sonnet-4-6") em vez do alias canônico
#              (ex.: "model: sonnet"), quebrando o agente em produção.
#
# Objetivo (P4, ML-3A ROADMAP-2026-08-21-versao-do-modelo-por-tier-com-
# composicao-por-alvo): provar que a guarda `targetID == "claude" &&` em
# internal/integrations/render.go é o load-bearing literal que impede o
# vazamento de namespace.  A fronteira de namespace é um GATE (ADR-2026-08-21
# §4), não um cuidado — este cenário prova que o gate a detecta.
#
# Alvo de sabotagem escolhido: não-vazamento (Gemini), não composição.
# Um P4 que só provasse que a composição deixou de funcionar deixaria o
# vazamento sem falsificação — e vazamento é o defeito caro: o usuário só
# descobre quando o agente não sobe, com a causa a duas camadas de distância.
#
# Por que Gemini e não Codex:
#   - Codex usa case "custom-agent-toml" (retorna cedo do switch de
#     representação) — a guarda no case `default:` não o alcança de forma
#     alguma.  Sabotá-la não causaria leak no Codex, tornando o P4 inválido.
#   - Gemini usa case `default:` (representation "agent-markdown") sem
#     proteção específica por targetID — é exatamente o alvo que a guarda
#     protege e que vazaria se ela fosse removida.
#
# Estratégia: copiar a árvore Go para $T86, remover `targetID == "claude" && `
# da condição em render.go com sed, compilar um binário isolado e executar
# check-agent-models-parity.sh apontando GO_BIN para ele.  O braco de detecção
# espera a mensagem "namespace leak" no stderr/stdout.  Verificação de
# vivacidade: confirmar que o arquivo foi de fato modificado (sed não foi
# no-op) e que o binário corrompido ainda compila (a guarda é um predicado,
# não uma declaração — sem ela o código ainda é Go válido).
#
# Seam: internal/integrations/render.go, condição no case `default:`:
#   } else if targetID == "claude" && len(agentModels) > 0 {
# →  } else if len(agentModels) > 0 {
# ---------------------------------------------------------------------------
T86="$WORK/s86"
mkdir -p "$T86/cmd" "$T86/internal" "$T86/bin" "$T86/scripts"
cp -r "$ROOT_DIR/cmd/." "$T86/cmd/"
cp -r "$ROOT_DIR/internal/." "$T86/internal/"
cp "$ROOT_DIR/go.mod" "$T86/go.mod"
cp "$ROOT_DIR/go.sum" "$T86/go.sum"
cp "$ROOT_DIR/scripts/check-agent-models-parity.sh" "$T86/scripts/"

# Aplicar patch: remover a guarda de allowlist claude
sed 's/} else if targetID == "claude" \&\& len(agentModels) > 0 {/} else if len(agentModels) > 0 {/' \
  "$ROOT_DIR/internal/integrations/render.go" > "$T86/internal/integrations/render.go"

# Verificação de vivacidade: confirmar que o patch foi aplicado
if cmp -s "$ROOT_DIR/internal/integrations/render.go" "$T86/internal/integrations/render.go"; then
  echo 'FAIL [falsify/setup-s86-liveness]: sed nao modificou render.go — seam pode ter mudado' >&2
  falsify_fail_point
fi
echo 'OK   [falsify/agent-models-parity/namespace-guard-liveness]'

# Compilar binário corrompido — a guarda é um predicado puro; sem ela o código
# continua válido Go e compila normalmente.
if ! (cd "$T86" && env GOCACHE="$WORK/go-build-cache" go build -o "$T86/bin/trackfw" ./cmd/trackfw) >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s86-build]: binário corrompido nao compilou — verificar o patch' >&2
  falsify_fail_point
fi
echo 'OK   [falsify/agent-models-parity/namespace-guard-sabotaged-build]'

# Braco de baseline: gate deve PASSAR com o binário real
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s86-baseline]: check-agent-models-parity.sh ja reprova com binario real — prova P4 invalida' >&2
  falsify_fail_point
fi
echo 'OK   [falsify/agent-models-parity/namespace-guard-baseline]'

# Braco de detecção: gate deve FALHAR com o binário corrompido,
# especificamente relatando "namespace leak" no output.
#
# Nota: usamos $ROOT_DIR/scripts/check-agent-models-parity.sh (não a cópia
# em $T86/scripts/).  A cópia T86 existe para que o T86 seja uma árvore Go
# válida (go build usa o ROOT_DIR dos arquivos Go do check-agent-models-
# parity.sh para descobrir o pacote — se o script não estiver em T86 a
# compilação do binário corrompido não depende dele).  Mas ao EXECUTAR o
# gate, ROOT_DIR deve resolver para o projeto real, pois NODE_CLI e PY_ROOT
# são derivados de ROOT_DIR no script e npm/pypi não existem em $T86.
# Resultado: GO_BIN aponta para o binário corrompido (que vaza namespace);
# Node.js e Python usam os CLIs reais e ficam limpos — como esperado, dado
# que o leak é exclusivo da guarda removida do Go.  set -euo pipefail faria
# o script morrer antes de atingir a mensagem "namespace leak" se o Node
# tentasse chamar um binário inexistente em $T86/npm/.
assert_fails_with "agent-models-parity/namespace-guard-removed-causes-gemini-leak" \
  "namespace leak" \
  env GO_BIN="$T86/bin/trackfw" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh"

echo '# ---------------------------------------------------------------------------'
# Cenário 87 — check-release-tag-parity.sh Scenario 16 (content-from-commit-
#              provenance): CHANGELOG read bypassed from forge commit back to
#              local HEAD — false-negative proof.
#
# Alvo de sabotagem:
#   deps.readCommittedFile(objectSHA, "CHANGELOG.md")
#   → deps.readCommittedFile("HEAD", "CHANGELOG.md")
#   in internal/commands/release.go.
#
# Por que este literal:
#   - readFile foi REMOVIDO do struct releaseDeps (ML-2A); substituir por
#     deps.readFile() não compila. O único fallback possível é passar um
#     argumento sha diferente a readCommittedFile.
#   - "HEAD" é sintaticamente válido (git show HEAD:<path> resolve para o tip
#     local) e compila sem alterações adicionais.
#   - P3 (leitura dos version files) ainda usa objectSHA → ainda passa (9.9.9).
#   - P4 (leitura do CHANGELOG) agora lê de HEAD → ## [9.9.9] encontrado em
#     HEAD também → P4 ainda passa → exit 0.
#   - MAS message = conteúdo de HEAD's CHANGELOG ("head-only"), não do forge
#     commit ("forge-only") → asserção de proveniência do Scenario 16 dispara.
#   - O literal aparece EXATAMENTE UMA VEZ em release.go (verificado por grep):
#     corrupt_literal reprova se count ≠ 1.
#
# Não-vacuidade: braço de baseline prova que o gate passa com o binário real
# antes de qualquer sabotagem. Braço de detecção prova que o binário corrompido
# faz o gate falhar com a mensagem exata da asserção de proveniência.
#
# Seam (único em release.go):
#   changelogContent, err := deps.readCommittedFile(objectSHA, "CHANGELOG.md")
# →  changelogContent, err := deps.readCommittedFile("HEAD", "CHANGELOG.md")
# ---------------------------------------------------------------------------
T87="$WORK/s87"
T87C_GO_MOD="$T87/corrupt-go"
T87C_GO_BIN="$T87/corrupt-go-bin/trackfw"
mkdir -p "$T87C_GO_MOD/cmd" "$T87C_GO_MOD/internal" "$T87/corrupt-go-bin"
cp -r "$ROOT_DIR/cmd/." "$T87C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T87C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T87C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T87C_GO_MOD/go.sum"

# Baseline: gate deve PASSAR com o binário real
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s87-baseline]: check-release-tag-parity.sh ja reprova com binario real — prova P4 invalida' >&2
  falsify_fail_point
fi
echo 'OK   [falsify/release-tag-parity/content-from-commit-baseline]'

# Aplicar sabotagem: substituir objectSHA por "HEAD" na leitura do CHANGELOG
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" \
  "$T87C_GO_MOD/internal/commands/release.go" \
  'deps.readCommittedFile(objectSHA, "CHANGELOG.md")' \
  'deps.readCommittedFile("HEAD", "CHANGELOG.md")' \
  'setup-s87-changelog-read-corrupt'

build_go_or_fail "setup-s87-go-corrupt-build" "$T87C_GO_MOD" "$T87C_GO_BIN"

# Detecção: gate deve FALHAR com a mensagem exata da asserção de proveniência
assert_fails_with "release-tag-parity/content-from-commit-false-negative" \
  "provenance: tag message must contain 'forge-only'" \
  env GO_BIN="$T87C_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

echo '# ---------------------------------------------------------------------------'
# Cenário 158 — check-release-tag-parity.sh Scenario 17 (refs-replace-bypass):
#               --no-replace-objects flag removed from git show call in
#               defaultReleaseReadCommittedFile — false-negative proof (P4).
#
# Alvo de sabotagem:
#   exec.Command("git", "--no-replace-objects", "show", sha+":"+path)
#   → exec.Command("git", "show", sha+":"+path)
#   (removes the flag that blocks refs/replace/ object-identity substitution)
#
# Mecanismo do falso-negativo:
#   - Sem --no-replace-objects, git show $FORGE_SHA:CHANGELOG.md segue a
#     refs/replace/ ref e lê o commit do atacante ("refs-replace-forged").
#   - P3 (version files) ainda lê de objectSHA → 9.9.9 ✓ → exit 0.
#   - P4 (CHANGELOG) também retorna ## [9.9.9] section (do commit atacante,
#     que herda a estrutura do forge commit via s17-attacker branch) → P4 ✓.
#   - Mas a mensagem da tag = "refs-replace-forged" (conteúdo do atacante),
#     não "forge-only" (conteúdo do forge commit) → asserção de proveniência
#     do Scenario 17 dispara: 'provenance: tag message must contain forge-only'.
#   - A asserção por runtime é o que torna o scenario resistente a revert
#     correlacionado: todos os 3 stacks revertem → cada runtime dispara
#     individualmente; assert_three_way pega o revert de stack único.
#
# Seam (único em release.go — verificado por grep):
#   "--no-replace-objects", "show"  →  "show"
# ---------------------------------------------------------------------------
T88="$WORK/s158"
T88C_GO_MOD="$T88/corrupt-go"
T88C_GO_BIN="$T88/corrupt-go-bin/trackfw"
mkdir -p "$T88C_GO_MOD/cmd" "$T88C_GO_MOD/internal" "$T88/corrupt-go-bin"
cp -r "$ROOT_DIR/cmd/." "$T88C_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$T88C_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$T88C_GO_MOD/go.mod"
cp "$ROOT_DIR/go.sum" "$T88C_GO_MOD/go.sum"

# Baseline: gate deve PASSAR com o binário real (independente do braço S87)
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s158-baseline]: check-release-tag-parity.sh ja reprova com binario real — prova P4 invalida' >&2
  falsify_fail_point
fi
echo 'OK   [falsify/release-tag-parity/refs-replace-bypass-baseline]'

# Aplicar sabotagem: remover --no-replace-objects da chamada git show
corrupt_literal \
  "$ROOT_DIR/internal/commands/release.go" \
  "$T88C_GO_MOD/internal/commands/release.go" \
  '"--no-replace-objects", "show"' \
  '"show"' \
  'setup-s158-no-replace-objects-removed'

build_go_or_fail "setup-s158-go-corrupt-build" "$T88C_GO_MOD" "$T88C_GO_BIN"

# Detecção: gate deve FALHAR com a mensagem exata da asserção de proveniência
# do Scenario 17. O binário corrompido segue refs/replace/ e lê o CHANGELOG
# do commit do atacante ("refs-replace-forged") em vez do forge commit
# ("forge-only") — Scenario 17's provenance assertion fires.
assert_fails_with "release-tag-parity/refs-replace-bypass-false-negative" \
  "provenance: tag message must contain 'forge-only'" \
  env GO_BIN="$T88C_GO_BIN" bash "$ROOT_DIR/scripts/check-release-tag-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 167 -- Direcao B (AC9, mesmo roadmap, ML-2A): parseWaves do
#                lower bound volta a tratar "0" como malformado (regressao
#                de internal/roadmapdoc/roadmapdoc.go, intVal < 0 ->
#                intVal < 1).
#
# ML-4C (REQ #392): todos os fixtures de check-barrier.sh ganharam Wave 0.
# O ponto de deteccao mudou: S1 (--wave 1 com Wave 0 no fixture) falha
# primeiro -- wave_headings fica bloqueado porque Wave 0 e malformada,
# barrier sai com 1 e stderr contem "malformed wave heading". O padrao do
# assert_fails_with foi atualizado para corresponder a este novo ponto.
# (O ponto anterior era S11/--wave 0 -> exit 2; Cenario 168 continua
# cobrindo o segundo guarda em barrier.go com o mesmo padrao antigo.)
#
# Sabotagem Go-only (mesmo padrao dos Cenarios 164/165): so parseWaves e
# exercitado -- a validacao do FLAG em newBarrierCmd nao entra em jogo
# (coberta pelo Cenario 168, que usa waveInt < 1 em barrier.go).
# ---------------------------------------------------------------------------
T97="$WORK/s167"
mkdir -p "$T97/cmd" "$T97/internal"
cp -r "$ROOT_DIR/cmd/." "$T97/cmd/"
cp -r "$ROOT_DIR/internal/." "$T97/internal/"
cp "$ROOT_DIR/go.mod" "$T97/go.mod"
cp "$ROOT_DIR/go.sum" "$T97/go.sum"

sed 's/intVal < 0 {/intVal < 1 {/' \
  "$ROOT_DIR/internal/roadmapdoc/roadmapdoc.go" > "$T97/internal/roadmapdoc/roadmapdoc.go"

if cmp -s "$ROOT_DIR/internal/roadmapdoc/roadmapdoc.go" "$T97/internal/roadmapdoc/roadmapdoc.go"; then
  echo "FAIL [falsify/setup-s167]: sed nao alterou roadmapdoc.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T97_BIN="$WORK/s167-bin/trackfw"
mkdir -p "$(dirname "$T97_BIN")"
build_go_or_fail "setup-s167-build" "$T97" "$T97_BIN"

# Baseline -- binario REAL contra o fixture real de check-barrier.sh (BIS_SELFTEST_BREAK
# desligado): a suite inteira precisa passar antes de provar a deteccao.
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s167-baseline]: check-barrier.sh ja reprova com o binario real -- prova P4 invalida" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/barrier/wave-zero-rejected-again-baseline]"

assert_fails_with "barrier/wave-zero-rejected-again-detected" \
  "malformed wave heading" \
  env GO_BIN="$T97_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenario 168 -- Direcao B, SEGUNDO guarda (AC9, mesmo roadmap, ML-2A):
#                complementa o Cenario 167. ML-1A's audit finding #1 (docs/
#                agents-working-context.md, hades-tf FIM: ML-0A) e o modelo
#                de ameaca (Sec3 F2) sao explicitos: "barrier tem DOIS
#                guardas contra --wave 0, nao um" -- a validacao do FLAG em
#                newBarrierCmd (linha ~92, waveInt < 0) e o guarda dentro de
#                parseWaves (linha ~208, intVal < 0, coberto pelo Cenario
#                167). Uma regressao que reverta SO o guarda do flag nunca
#                chega a ler o roadmap -- sai em "trackfw barrier: invalid
#                --wave \"0\" -- not a valid wave label" (mensagem distinta
#                da de parseWaves, "malformed wave heading..."), mas ainda
#                assim exit 2, e ainda assim capturado pelo mesmo assert do
#                Cenario 11 invertido ("expected exit 0 or 1 (never 2"). Sem
#                este cenario, o segundo guarda ficaria sem prova de
#                falsificacao dedicada -- um dos dois pontos do achado
#                ficaria fechado so por inspecao, nao por gate.
# ---------------------------------------------------------------------------
T99="$WORK/s168"
mkdir -p "$T99/cmd" "$T99/internal"
cp -r "$ROOT_DIR/cmd/." "$T99/cmd/"
cp -r "$ROOT_DIR/internal/." "$T99/internal/"
cp "$ROOT_DIR/go.mod" "$T99/go.mod"
cp "$ROOT_DIR/go.sum" "$T99/go.sum"

sed 's/waveInt < 0 {/waveInt < 1 {/' \
  "$ROOT_DIR/internal/commands/barrier.go" > "$T99/internal/commands/barrier.go"

if cmp -s "$ROOT_DIR/internal/commands/barrier.go" "$T99/internal/commands/barrier.go"; then
  echo "FAIL [falsify/setup-s168]: sed nao alterou barrier.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T99_BIN="$WORK/s168-bin/trackfw"
mkdir -p "$(dirname "$T99_BIN")"
build_go_or_fail "setup-s168-build" "$T99" "$T99_BIN"

# Baseline ja provado pelo Cenario 167 (mesmo binario real, mesmo
# check-barrier.sh) -- reexecutar aqui seria redundante; a garantia de
# nao-vacuidade do braco de deteccao vem do assert_fails_with abaixo.
falsify_count_success
echo "OK   [falsify/barrier/wave-zero-flag-guard-rejected-again-baseline]: reaproveita a baseline do Cenario 167 (mesmo binario real, mesmo check-barrier.sh)"

assert_fails_with "barrier/wave-zero-flag-guard-rejected-again-detected" \
  "expected exit 0 or 1 (never 2" \
  env GO_BIN="$T99_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"


# ---------------------------------------------------------------------------
# Cenario 169 -- Direcao A: escopo global volta a ler do cwd em vez de
#                ~/.trackfw/trackfw.yaml (seam: integrations_flags.go:225,
#                config.ResolveAgentModels(opts.scope, ...) substituido por
#                config.Load().AgentModels, "").
#                Deteccao: check-agent-models-parity.sh Case 6 (dois cwds com
#                global pin identico mas cwd-b com sonnet:9.9 distinto --
#                se o escopo global le o cwd, cwd-a produz model: sonnet e
#                cwd-b produz claude-sonnet-9-9; a comparacao cross-cwd reprova
#                com "global scope reads cwd instead of global config").
# ---------------------------------------------------------------------------
T169="$WORK/s169"
mkdir -p "$T169/cmd" "$T169/internal"
cp -r "$ROOT_DIR/cmd/." "$T169/cmd/"
cp -r "$ROOT_DIR/internal/." "$T169/internal/"
cp "$ROOT_DIR/go.mod" "$T169/go.mod"
cp "$ROOT_DIR/go.sum" "$T169/go.sum"

sed 's/config\.ResolveAgentModels(opts\.scope, manager\.HomeDir, manager\.ProjectRoot)/config.Load().AgentModels, ""/' \
  "$ROOT_DIR/internal/commands/integrations_flags.go" > "$T169/internal/commands/integrations_flags.go"

if cmp -s "$ROOT_DIR/internal/commands/integrations_flags.go" "$T169/internal/commands/integrations_flags.go"; then
  echo "FAIL [falsify/setup-s169]: sed nao alterou integrations_flags.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T169_BIN="$WORK/s169-bin/trackfw"
mkdir -p "$(dirname "$T169_BIN")"
build_go_or_fail "setup-s169-build" "$T169" "$T169_BIN"

# Baseline -- binario REAL: check-agent-models-parity.sh deve passar antes da deteccao
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s169-baseline]: check-agent-models-parity.sh ja reprova com o binario real -- prova P4 invalida" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/global-scope/direction-a-reads-cwd-baseline]"

assert_fails_with "global-scope/direction-a-reads-cwd-detected" \
  "from global pin" \
  env GO_BIN="$T169_BIN" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 170 -- Direcao B: escopo de projeto passa a ler do global em vez
#                do projeto (seam: integrations_flags.go:225, opts.scope
#                substituido por "global" em config.ResolveAgentModels).
#                Deteccao: check-agent-models-parity.sh Case 9 (projeto com
#                sonnet:9.9, global com sonnet:4.6 -- se o escopo de projeto
#                ler do global, o modelo seria claude-sonnet-4-6 em vez de
#                claude-sonnet-9-9; a vacuity guard reprova com
#                "claude-sonnet-9-9' (project pin)" — aspas simples faz parte
#                da mensagem diag: "missing 'model: claude-sonnet-9-9' (project pin)").
#                Baseline reaproveita do Cenario 169 (mesmo binario real,
#                mesmo check-agent-models-parity.sh).
# ---------------------------------------------------------------------------
T170="$WORK/s170"
mkdir -p "$T170/cmd" "$T170/internal"
cp -r "$ROOT_DIR/cmd/." "$T170/cmd/"
cp -r "$ROOT_DIR/internal/." "$T170/internal/"
cp "$ROOT_DIR/go.mod" "$T170/go.mod"
cp "$ROOT_DIR/go.sum" "$T170/go.sum"

sed 's/config\.ResolveAgentModels(opts\.scope,/config.ResolveAgentModels("global",/' \
  "$ROOT_DIR/internal/commands/integrations_flags.go" > "$T170/internal/commands/integrations_flags.go"

if cmp -s "$ROOT_DIR/internal/commands/integrations_flags.go" "$T170/internal/commands/integrations_flags.go"; then
  echo "FAIL [falsify/setup-s170]: sed nao alterou integrations_flags.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T170_BIN="$WORK/s170-bin/trackfw"
mkdir -p "$(dirname "$T170_BIN")"
build_go_or_fail "setup-s170-build" "$T170" "$T170_BIN"

# Baseline reaproveita do Cenario 169 (mesmo binario real, mesmo gate)
falsify_count_success
echo "OK   [falsify/global-scope/direction-b-reads-global-baseline]: reaproveita baseline do Cenario 169"

assert_fails_with "global-scope/direction-b-reads-global-detected" \
  "claude-sonnet-9-9' (project pin)" \
  env GO_BIN="$T170_BIN" bash "$ROOT_DIR/scripts/check-agent-models-parity.sh"


# ---------------------------------------------------------------------------
# Cenario 171 -- Direcao A: sanitizacao do titulo removida do roadmap.go
#                (ML-1A da REQ-2026-08-23) faz o Cenario 13 do check-barrier.sh
#                falhar. Sabotagem Go-only: strings.ContainsAny(content.Title,
#                "\n\r") substituido por false -- o bloco de sanitizacao vira dead
#                code e roadmap new com titulo forjado passa a retornar exit 0.
#                Deteccao: check-barrier.sh Scenario 13 reprova com
#                "expected 'roadmap title must be a single line'".
# ---------------------------------------------------------------------------
T171="$WORK/s171"
mkdir -p "$T171/cmd" "$T171/internal"
cp -r "$ROOT_DIR/cmd/." "$T171/cmd/"
cp -r "$ROOT_DIR/internal/." "$T171/internal/"
cp "$ROOT_DIR/go.mod" "$T171/go.mod"
cp "$ROOT_DIR/go.sum" "$T171/go.sum"

sed 's/strings\.ContainsAny(content\.Title, "\\n\\r")/false/' \
  "$ROOT_DIR/internal/generators/roadmap.go" > "$T171/internal/generators/roadmap.go"

if cmp -s "$ROOT_DIR/internal/generators/roadmap.go" "$T171/internal/generators/roadmap.go"; then
  echo "FAIL [falsify/setup-s171]: sed nao alterou roadmap.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T171_BIN="$WORK/s171-bin/trackfw"
mkdir -p "$(dirname "$T171_BIN")"
build_go_or_fail "setup-s171-build" "$T171" "$T171_BIN"

# Baseline -- binario REAL: check-barrier.sh deve passar antes da deteccao
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s171-baseline]: check-barrier.sh ja reprova com o binario real -- prova P4 invalida" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/ac2-sanitization/direction-a-baseline]"

assert_fails_with "ac2-sanitization/direction-a-detected" \
  "expected exit non-0 for forged title" \
  env GO_BIN="$T171_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenario 172 -- Direcao B: verificacao de confianca removida do barrier.go
#                (ML-2A da REQ-2026-08-23) faz o Cenario 14 do check-barrier.sh
#                falhar. Sabotagem Go-only: `if !verdict.trusted {` substituido
#                por `if false {` -- o caminho not_evaluated nunca e tomado e o
#                gate sempre executa, mesmo sem --trust-local-gates.
#                Deteccao: check-barrier.sh Scenario 14 reprova com
#                "hostile gate EXECUTED -- sentinel was created" (AC14: a prova
#                e pela ausencia do arquivo, nao pelo exit code).
#                Baseline reaproveita do Cenario 171 (mesmo binario real,
#                mesmo check-barrier.sh).
# ---------------------------------------------------------------------------
T172="$WORK/s172"
mkdir -p "$T172/cmd" "$T172/internal"
cp -r "$ROOT_DIR/cmd/." "$T172/cmd/"
cp -r "$ROOT_DIR/internal/." "$T172/internal/"
cp "$ROOT_DIR/go.mod" "$T172/go.mod"
cp "$ROOT_DIR/go.sum" "$T172/go.sum"

sed 's/if !verdict\.trusted {/if false {/' \
  "$ROOT_DIR/internal/commands/barrier.go" > "$T172/internal/commands/barrier.go"

if cmp -s "$ROOT_DIR/internal/commands/barrier.go" "$T172/internal/commands/barrier.go"; then
  echo "FAIL [falsify/setup-s172]: sed nao alterou barrier.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T172_BIN="$WORK/s172-bin/trackfw"
mkdir -p "$(dirname "$T172_BIN")"
build_go_or_fail "setup-s172-build" "$T172" "$T172_BIN"

# Baseline reaproveita do Cenario 171 (mesmo binario real, mesmo gate)
falsify_count_success
echo "OK   [falsify/trust-check/direction-b-baseline]: reaproveita baseline do Cenario 171"

assert_fails_with "trust-check/direction-b-detected" \
  "hostile gate EXECUTED" \
  env GO_BIN="$T172_BIN" bash "$ROOT_DIR/scripts/check-barrier.sh"

# ---------------------------------------------------------------------------
# Cenario 175 -- Direcao A: add("trackfw.yaml") removido de
#                buildSandboxInclusion (internal/generators/update.go) faz o
#                Cenario 11 do check-update-parity.sh falhar: dry-run reporta
#                skipped onde run real reporta updated para fixture com
#                agent_conventions (trackfw.yaml ausente do sandbox =>
#                ReadAgentConventions retorna vazio => hash de CLAUDE.md difere
#                do run real).
#                Deteccao: check-update-parity.sh reprova com
#                "sandbox/gap-e/dry-vs-real".
# ---------------------------------------------------------------------------
T175="$WORK/s175"
mkdir -p "$T175/cmd" "$T175/internal"
cp -r "$ROOT_DIR/cmd/." "$T175/cmd/"
cp -r "$ROOT_DIR/internal/." "$T175/internal/"
cp "$ROOT_DIR/go.mod" "$T175/go.mod"
cp "$ROOT_DIR/go.sum" "$T175/go.sum"

corrupt_literal \
  "$ROOT_DIR/internal/generators/update.go" \
  "$T175/internal/generators/update.go" \
  'add("trackfw.yaml")' \
  '_ = "trackfw.yaml" // SABOTAGE-S175: trackfw.yaml removed from sandbox' \
  'setup-s175'

T175_BIN="$WORK/s175-bin/trackfw"
mkdir -p "$(dirname "$T175_BIN")"
build_go_or_fail "setup-s175-build" "$T175" "$T175_BIN"

# Baseline -- binario REAL: check-update-parity.sh deve passar antes da deteccao
if ! GO_BIN="$FALSIFY_GO_BIN" bash "$ROOT_DIR/scripts/check-update-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s175-baseline]: check-update-parity.sh ja reprova com o binario real -- prova P4 invalida" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/sandbox-gap-e/direction-a-baseline]"

assert_fails_with "sandbox-gap-e/direction-a-detected" \
  "sandbox/gap-e/dry-vs-real" \
  env GO_BIN="$T175_BIN" bash "$ROOT_DIR/scripts/check-update-parity.sh"

# ---------------------------------------------------------------------------
# Cenario 176 -- Direcao B: corpo de copyProjectTree substituido por
#                filepath.WalkDir + os.ReadFile (reintroduzindo a travessia da
#                arvore inteira que aborta em symlinks pendurados fora do
#                conjunto declarado — o incidente do CMDB do KG).
#                Baseline reaproveita do Cenario 175 (mesmo gate, mesmo binario real).
#                Deteccao: check-update-parity.sh reprova com
#                "sandbox/dangling-outside-set/exit-zero".
# ---------------------------------------------------------------------------
T176="$WORK/s176"
mkdir -p "$T176/cmd" "$T176/internal"
cp -r "$ROOT_DIR/cmd/." "$T176/cmd/"
cp -r "$ROOT_DIR/internal/." "$T176/internal/"
cp "$ROOT_DIR/go.mod" "$T176/go.mod"
cp "$ROOT_DIR/go.sum" "$T176/go.sum"

"$PY_BIN" - "$ROOT_DIR/internal/generators/update.go" "$T176/internal/generators/update.go" <<'PY'
import pathlib, sys

src_path, dest_path = sys.argv[1], sys.argv[2]
text = pathlib.Path(src_path).read_text(encoding="utf-8")

old = (
    "\tfor _, rel := range paths {\n"
    "\t\tif err := copyPath(filepath.Join(src, rel), filepath.Join(dst, rel)); err != nil {\n"
    "\t\t\treturn fmt.Errorf(\"sandbox: copying %s: %w\", rel, err)\n"
    "\t\t}\n"
    "\t}\n"
    "\treturn nil"
)

new = (
    "\treturn filepath.WalkDir(src, func(fpath string, d fs.DirEntry, err error) error {\n"
    "\t\tif err != nil {\n"
    "\t\t\treturn err\n"
    "\t\t}\n"
    "\t\trel, _ := filepath.Rel(src, fpath)\n"
    "\t\tif d.IsDir() {\n"
    "\t\t\treturn os.MkdirAll(filepath.Join(dst, rel), 0o755)\n"
    "\t\t}\n"
    "\t\tdata, err := os.ReadFile(fpath)\n"
    "\t\tif err != nil {\n"
    "\t\t\treturn err\n"
    "\t\t}\n"
    "\t\treturn os.WriteFile(filepath.Join(dst, rel), data, 0o644)\n"
    "\t})"
)

count = text.count(old)
if count != 1:
    raise SystemExit(f"[setup-s176] expected exactly 1 occurrence of loop body, got {count}")
pathlib.Path(dest_path).write_text(text.replace(old, new, 1), encoding="utf-8")
PY

if cmp -s "$ROOT_DIR/internal/generators/update.go" "$T176/internal/generators/update.go"; then
  echo "FAIL [falsify/setup-s176]: python3 nao alterou update.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T176_BIN="$WORK/s176-bin/trackfw"
mkdir -p "$(dirname "$T176_BIN")"
build_go_or_fail "setup-s176-build" "$T176" "$T176_BIN"

falsify_count_success
echo "OK   [falsify/sandbox-walkdir-reintroduced/direction-b-baseline]: reaproveita baseline do Cenario 175"

assert_fails_with "sandbox-walkdir-reintroduced/direction-b-detected" \
  "sandbox/dangling-outside-set/exit-zero" \
  env GO_BIN="$T176_BIN" bash "$ROOT_DIR/scripts/check-update-parity.sh"

# Cenario 181 -- Direcao C: os.Chmod removido de generateValidateScript
#                (scaffold.go). Sem o Chmod, trackfw update reescreve o
#                conteudo (apply() roda via runFileTarget para arquivo
#                existente) mas nao restaura o bit de execucao — o ciclo
#                "doctor acusa → update nao remedia → doctor acusa de novo"
#                reaparece (AC9). O cenario verifica o EFEITO: o bit foi
#                restaurado no braco de baseline, e permanece ausente no
#                braco de deteccao APOS o update reescrever o conteudo.
#                Braco de baseline: baixa o bit → binario REAL update
#                --targets validate-script → test -x confirma o bit voltou.
#                Braco de deteccao: corrompe conteudo + baixa o bit → binario
#                sabotado update --targets validate-script → cmp -s confirma
#                conteudo restaurado (apply() rodou) → test ! -x confirma bit
#                ainda ausente (Chmod nao rodou).
#                ROADMAP: ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-
#                dos-artefatos-de-scaffold, ML-2A, AC7/AC8 direcao C.
# ---------------------------------------------------------------------------
T181="$WORK/s181"
mkdir -p "$T181/cmd" "$T181/internal"
cp -r "$ROOT_DIR/cmd/." "$T181/cmd/"
cp -r "$ROOT_DIR/internal/." "$T181/internal/"
cp "$ROOT_DIR/go.mod" "$T181/go.mod"
cp "$ROOT_DIR/go.sum" "$T181/go.sum"

"$PY_BIN" - "$ROOT_DIR/internal/generators/scaffold.go" \
          "$T181/internal/generators/scaffold.go" <<'PY'
import pathlib, sys

src_path, dest_path = sys.argv[1], sys.argv[2]
text = pathlib.Path(src_path).read_text(encoding="utf-8")

old = (
    '\tif err := os.Chmod(path, 0755); err != nil {\n'
    '\t\treturn fmt.Errorf("setting execute bit on validate script: %w", err)\n'
    '\t}\n'
)
new = '\t// falsify: os.Chmod removed (AC9 regression probe -- Direction C)\n'

count = text.count(old)
if count != 1:
    raise SystemExit(f"[setup-s181] expected exactly 1 occurrence of Chmod block, got {count}")
pathlib.Path(dest_path).write_text(text.replace(old, new, 1), encoding="utf-8")
PY

if cmp -s "$ROOT_DIR/internal/generators/scaffold.go" "$T181/internal/generators/scaffold.go"; then
  echo "FAIL [falsify/setup-s181]: python3 nao alterou scaffold.go -- padrao nao encontrado; prova P4 invalida" >&2
  falsify_fail_point
fi

T181_BIN="$WORK/s181-bin/trackfw"
mkdir -p "$(dirname "$T181_BIN")"
build_go_or_fail "setup-s181-build" "$T181" "$T181_BIN"

_S181_ID='{"schema_version":1,"user_nickname":"KG","agents":{"backend":{"display_name":"Apolo","slug":"apolo"}}}'
_S181_CFG='governance_mode: lenient
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
'

# Braco de baseline: REAL binary restaura o bit apos ser baixado
T181_BASE_PROJ="$WORK/s181-base/project"
T181_BASE_HOME="$WORK/s181-base/home"
mkdir -p "$T181_BASE_PROJ" "$T181_BASE_HOME/.trackfw"
printf '%s\n' "$_S181_ID" >"$T181_BASE_HOME/.trackfw/identity.json"
printf '%s' "$_S181_CFG" >"$T181_BASE_PROJ/trackfw.yaml"
(cd "$T181_BASE_PROJ" && HOME="$T181_BASE_HOME" "$ROOT_DIR/bin/trackfw" \
  update --install-missing --targets validate-script) >/dev/null
chmod 0644 "$T181_BASE_PROJ/scripts/trackfw-validate.sh"
(cd "$T181_BASE_PROJ" && HOME="$T181_BASE_HOME" "$ROOT_DIR/bin/trackfw" \
  update --targets validate-script) >/dev/null
if test -x "$T181_BASE_PROJ/scripts/trackfw-validate.sh"; then
  falsify_count_success
  echo "OK   [falsify/scaffold-update-chmod-removed/direction-c-baseline]"
else
  echo "FAIL [falsify/scaffold-update-chmod-removed/direction-c-baseline]: binario real nao restaurou o bit de execucao apos update" >&2
  ls -la "$T181_BASE_PROJ/scripts/trackfw-validate.sh" >&2
  falsify_fail_point
fi

# Braco de deteccao: sabotaged binary restaura o conteudo mas NAO o bit
T181_DET_PROJ="$WORK/s181-det/project"
T181_DET_HOME="$WORK/s181-det/home"
mkdir -p "$T181_DET_PROJ" "$T181_DET_HOME/.trackfw"
printf '%s\n' "$_S181_ID" >"$T181_DET_HOME/.trackfw/identity.json"
printf '%s' "$_S181_CFG" >"$T181_DET_PROJ/trackfw.yaml"
(cd "$T181_DET_PROJ" && HOME="$T181_DET_HOME" "$ROOT_DIR/bin/trackfw" \
  update --install-missing --targets validate-script) >/dev/null
T181_SCRIPT="$T181_DET_PROJ/scripts/trackfw-validate.sh"
# Salva conteudo canonico antes de corromper
cp "$T181_SCRIPT" "$WORK/s181-canonical.sh"
# Corrompe conteudo (apply() deve detectar e restaurar) e baixa o bit
printf 'X' >>"$T181_SCRIPT"
chmod 0644 "$T181_SCRIPT"
# Roda binario sabotado
(cd "$T181_DET_PROJ" && HOME="$T181_DET_HOME" "$T181_BIN" \
  update --targets validate-script) >/dev/null
# Verifica: conteudo restaurado (apply() rodou)
if ! cmp -s "$WORK/s181-canonical.sh" "$T181_SCRIPT"; then
  echo "FAIL [falsify/scaffold-update-chmod-removed/direction-c-detected]: binario sabotado nao restaurou o conteudo -- apply() nao rodou" >&2
  falsify_fail_point
fi
# Verifica: bit ainda ausente (Chmod nao rodou)
if test -x "$T181_SCRIPT"; then
  echo "FAIL [falsify/scaffold-update-chmod-removed/direction-c-detected]: binario sabotado restaurou o bit de execucao -- os.Chmod nao foi removido" >&2
  ls -la "$T181_SCRIPT" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/scaffold-update-chmod-removed/direction-c-detected]"

# ---------------------------------------------------------------------------
# ML-1A-D6write — normalização semver→PEP 440 no nome da wheel (auditoria 2026-09-13)
# Direção 1: nome não-normalizado (8.0.0-rc1) é rejeitado por parse_wheel_filename.
# Direção 2: nome normalizado (8.0.0rc1) é aceito.
# Regra Dura de Reconciliação:
#   Cenário 182: parse_wheel_filename rejeita nome com versão semver crua — a gate
#     detectaria a regressão se a normalização fosse removida de build_wheel.py.
#   Cenário 183: parse_wheel_filename aceita nome com versão PEP 440 — confirma que
#     a forma normalizada é suficiente.
# ---------------------------------------------------------------------------
bash "$ROOT_DIR/scripts/check-wheel-filename.sh" --falsify-raw
falsify_count_success
echo "OK   [falsify/wheel-filename/raw]: nome nao-normalizado rejeitado (cenario 182)"

bash "$ROOT_DIR/scripts/check-wheel-filename.sh" --falsify-normalized
falsify_count_success
echo "OK   [falsify/wheel-filename/normalized]: nome normalizado aceito (cenario 183)"

# ---------------------------------------------------------------------------
# Cenario 184 — check-static-assets.sh: guarda de vacuidade (v8 single-runtime)
#               O gate v8 verifica apenas que a fonte canonica
#               internal/serve/static existe e e nao-vazia (assets passaram
#               para go:embed no ML-3A; npm/src e pypi/trackfw/serve/static
#               foram apagados). A guarda de vacuidade impede que um embed vazio
#               compile sem erro mas sirva assets quebrados.
#               Direcao A: fonte canonica vazia → gate falha com
#               "Static assets: no files found in canonical source".
#               Nao-vacuidade: guarda neutralizada → gate passa mesmo com dir
#               vazio — isola que e especificamente o bloco `if [[ ! -s ... ]]`
#               que produz o sinal.
#               Regra Dura de Reconciliacao:
#                 Cenario 184-A: afirma que a guarda detecta fonte vazia —
#                 necessario porque embed vazio compila sem erro.
#                 Cenario 184-NV: afirma que a guarda e load-bearing —
#                 sem ela a fonte vazia passaria em silencio.
# ---------------------------------------------------------------------------

# Baseline: gate deve passar com a fonte canonica real (nao vazia)
if ! bash "$ROOT_DIR/scripts/check-static-assets.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s184-baseline]: check-static-assets.sh ja reprova com a fonte real -- prova invalida" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/static-assets/vacuity-baseline]"

# Direcao A: fonte canonica VAZIA -> gate falha
T184A="$WORK/s184a"
mkdir -p "$T184A/scripts" "$T184A/internal/serve/static"
cp "$ROOT_DIR/scripts/check-static-assets.sh" "$T184A/scripts/check-static-assets.sh"
assert_fails_with "static-assets/vacuity-guard/direction-a-detected" \
  "Static assets: no files found in canonical source" \
  bash "$T184A/scripts/check-static-assets.sh"

# Nao-vacuidade: neutraliza o bloco de verificacao numa copia isolada — gate
# deve PASSAR com dir vazio, provando que e a guarda que detecta.
T184NV="$WORK/s184nv"
mkdir -p "$T184NV/scripts" "$T184NV/internal/serve/static"
corrupt_literal \
  "$ROOT_DIR/scripts/check-static-assets.sh" "$T184NV/scripts/check-static-assets.sh" \
  'if [[ ! -s "$TMP_ROOT/canonical-files" ]]; then' \
  'if false; then # SABOTAGE-S184: vacuity guard neutralized' \
  "s184-nv-vacuity-guard-removed"
assert_succeeds "static-assets/vacuity-guard/non-vacuity" \
  bash "$T184NV/scripts/check-static-assets.sh"
echo "PROOF [falsify/static-assets/vacuity-guard/non-vacuity]: sem a guarda, fonte vazia passa em silencio -- a guarda e load-bearing"

# ---------------------------------------------------------------------------
# Cenario 185 — check-integration-assets.sh: duas direcoes (v8 shim)
#               Direcao A: catalog.json ausente → gate falha com
#               "Canonical integration assets are missing".
#               Direcao B: npm/package.json sem "bin/trackfw.js" → gate falha
#               com "must list bin/trackfw.js in files".
#               Regra Dura de Reconciliacao:
#                 Cenario 185-A: afirma que a verificacao de presenca do
#                 catalog.json e load-bearing — embed quebrado seria silencioso.
#                 Cenario 185-B: afirma que a verificacao do shim npm e
#                 load-bearing — um package.json v7 passaria sem ela.
# ---------------------------------------------------------------------------

# Baseline: gate deve passar com todos os artefatos presentes
if ! bash "$ROOT_DIR/scripts/check-integration-assets.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s185-baseline]: check-integration-assets.sh ja reprova com artefatos reais -- prova invalida" >&2
  falsify_fail_point
fi
falsify_count_success
echo "OK   [falsify/integration-assets/baseline]"

# Direcao A: catalog.json ausente (dir de assets existe mas sem catalog.json)
T185A="$WORK/s185a"
mkdir -p "$T185A/scripts" "$T185A/internal/integrations/assets" \
         "$T185A/npm" "$T185A/pypi"
cp "$ROOT_DIR/scripts/check-integration-assets.sh" "$T185A/scripts/check-integration-assets.sh"
cp "$ROOT_DIR/npm/package.json" "$T185A/npm/package.json"
cp "$ROOT_DIR/pypi/pyproject.toml" "$T185A/pypi/pyproject.toml"
# catalog.json deliberadamente ausente — apenas o dir existe
assert_fails_with "integration-assets/direction-a-catalog-absent" \
  "Canonical integration assets are missing" \
  bash "$T185A/scripts/check-integration-assets.sh"

# Direcao B: npm/package.json sem "bin/trackfw.js" (shim v8 ausente)
T185B="$WORK/s185b"
mkdir -p "$T185B/scripts" "$T185B/internal/integrations/assets" \
         "$T185B/npm" "$T185B/pypi"
cp "$ROOT_DIR/scripts/check-integration-assets.sh" "$T185B/scripts/check-integration-assets.sh"
cp "$ROOT_DIR/pypi/pyproject.toml" "$T185B/pypi/pyproject.toml"
echo '{"description":"trackfw"}' > "$T185B/internal/integrations/assets/catalog.json"
# package.json sem "bin/trackfw.js" em files
python3 -c "
import json, sys
with open('$ROOT_DIR/npm/package.json') as f:
    d = json.load(f)
d['files'] = [x for x in d.get('files', []) if x != 'bin/trackfw.js']
print(json.dumps(d, indent=2))
" > "$T185B/npm/package.json"
assert_fails_with "integration-assets/direction-b-shim-absent" \
  "must list bin/trackfw.js in files" \
  bash "$T185B/scripts/check-integration-assets.sh"


# ---------------------------------------------------------------------------
# Cenario 182 -- check-pr-closing-keyword.sh: a isencao e POR NUMERO DE ISSUE.
#
# (ML-1A, ROADMAP-2026-09-02-gate-e-template-de-pr-exigem-palavra-chave-de-
#  fechamento-em-ingles.md)
#
# O gate reprova palavra-chave de fechamento em portugues no corpo do PR
# ("Fecha #246.") quando NAO ha forma inglesa valida PARA AQUELE MESMO NUMERO.
# A clausula "para aquele mesmo numero" e a parte que pode ser implementada
# errada sem que nada quebre: trocar por "existe alguma palavra inglesa no
# corpo" deixa o gate verde sobre o defeito real, porque corpos deste
# repositorio citam `Fixes #N` de OUTRAS issues o tempo todo. O gate ficaria
# oco e o `make quality` continuaria verde -- exatamente a classe dos 4 gates
# vacuos medidos na auditoria de 2026-09-02.
#
# Este cenario nao le o codigo do gate: SABOTA uma copia dele (a comparacao
# por numero vira uma comparacao global) e prova, por EXECUCAO, que a copia
# sabotada passa sobre o corpo em que o gate real reprova.
# ---------------------------------------------------------------------------
S182_REAL="$ROOT_DIR/scripts/check-pr-closing-keyword.sh"
S182_SAB="$WORK/s182-sabotado.sh"
S182_BODY="$WORK/s182-body.md"
S182_PROSE="$WORK/s182-prose.md"
S182_EMPTY="$WORK/s182-empty.md"

# Corpo do defeito: portugues fechando a #246, com uma forma inglesa presente
# no corpo mas apontando para OUTRA issue (#999). Forma real: e o corpo do
# PR #247 acrescido de uma citacao `Fixes #N` como as dos PRs #238/#240.
printf 'Fecha #246.\n\nTexto qualquer sobre o defeito.\n\nFixes #999\n' >"$S182_BODY"
# Prosa real, extraida de corpos de PR mergeados deste repositorio.
printf 'A regressao esta no mesmo sitio do #238.\n\nFecha o **item 4** da issue #216.\n\nCorrige os tres defeitos do #232.\n' >"$S182_PROSE"
: >"$S182_EMPTY"

# Sabotagem: `if num not in english:` -> `if not english:` (isencao global).
sed 's/if num not in english:/if not english:/' "$S182_REAL" >"$S182_SAB"
chmod +x "$S182_SAB"
if cmp -s "$S182_REAL" "$S182_SAB"; then
  echo "FAIL [falsify/setup-s182]: sabotagem nao alterou nada -- a linha 'if num not in english:' sumiu do gate (renomeada?). Este cenario parou de medir o que promete." >&2
  falsify_fail_point
fi

# Baseline: o gate REAL reprova o corpo, nomeando a linha e a issue.
assert_fails_with "pr-closing-keyword/isencao-por-numero-baseline" \
  'nao fecha a issue #246' \
  env PR_BODY_FILE="$S182_BODY" bash "$S182_REAL"

# Direcao A: o gate SABOTADO passa sobre o MESMO corpo -> a clausula por
# numero e o que segura o defeito. Se esta asserção falhar, ou a sabotagem
# nao representa mais a regressao, ou o gate ganhou outra defesa.
set +e
s182_sab_out=$(env PR_BODY_FILE="$S182_BODY" bash "$S182_SAB" 2>&1)
s182_sab_status=$?
set -e
if [[ $s182_sab_status -eq 0 ]]; then
  falsify_count_success
  echo "OK   [falsify/pr-closing-keyword/isencao-por-numero-sabotada-fica-verde]"
else
  echo "FAIL [falsify/pr-closing-keyword/isencao-por-numero-sabotada-fica-verde]: gate sabotado saiu $s182_sab_status, esperava 0 -- a sabotagem deixou de representar a regressao" >&2
  printf '%s\n' "$s182_sab_out" | sed 's/^/    /' >&2
  falsify_fail_point
fi

# Direcao B: o gate real NAO reprova prosa que so menciona issue/PR.
set +e
s182_prose_out=$(env PR_BODY_FILE="$S182_PROSE" bash "$S182_REAL" 2>&1)
s182_prose_status=$?
set -e
if [[ $s182_prose_status -eq 0 ]]; then
  falsify_count_success
  echo "OK   [falsify/pr-closing-keyword/prosa-nao-reprova]"
else
  echo "FAIL [falsify/pr-closing-keyword/prosa-nao-reprova]: gate reprovou prosa real de PR mergeado (exit $s182_prose_status) -- falso positivo" >&2
  printf '%s\n' "$s182_prose_out" | sed 's/^/    /' >&2
  falsify_fail_point
fi

# Vacuidade: corpo vazio -> not_evaluated (exit 2), nunca 0 em silencio.
assert_fails_with "pr-closing-keyword/vacuidade-corpo-vazio" \
  'not_evaluated: corpo do PR vazio' \
  env PR_BODY_FILE="$S182_EMPTY" bash "$S182_REAL"

# Vacuidade: fora de pull_request -> not_evaluated (exit 2).
assert_fails_with "pr-closing-keyword/vacuidade-fora-de-pull-request" \
  'nao e pull_request' \
  env -u PR_BODY_FILE -u PR_NUMBER GITHUB_EVENT_NAME=push \
      GITHUB_EVENT_PATH="$S182_BODY" bash "$S182_REAL"

# Autoteste do proprio gate (deteccao + prosa + isencao + vacuidade), pelo
# MESMO matcher que o CI usa.
if bash "$S182_REAL" --self-test >/dev/null 2>&1; then
  falsify_count_success
  echo "OK   [falsify/pr-closing-keyword/self-test-verde]"
else
  echo "FAIL [falsify/pr-closing-keyword/self-test-verde]: o autoteste do gate reprovou" >&2
  bash "$S182_REAL" --self-test 2>&1 | sed 's/^/    /' >&2
  falsify_fail_point
fi

# Cenário 192 — contentHasMarkerValue passa a exigir vínculo ESTRUTURAL
# (ROADMAP-2026-09-05-reconciliar-o-que-declaramos-com-o-que-medimos-apos-a-
# auditoria-externa, ML-2A — achado A2 da auditoria externa).
#
# Antes: o marcador (ADR:/REQ:/Roadmap:) era casado como SUBSTRING em
# qualquer lugar do conteúdo. Dois defeitos mergeados:
#   1. comentário HTML usado como placeholder ("ADR: <!-- preencher depois
#      -->") contava como valor real.
#   2. prosa que menciona o marcador no meio de uma frase ("veja a secao
#      ADR: mais abaixo") contava como vínculo real.
#
# Este cenário prova as DUAS direções cross-CLI, cada uma com seu próprio
# seam, contra o MESMO fixture (REQ com "ADR: <!-- preencher depois -->" e
# "Roadmap:" só mencionado em prosa) — sem o fixture nunca deveria emitir
# nenhuma das duas violações, com QUALQUER dos dois guards desativado ele
# deveria voltar a emitir a violação correspondente. Vacuidade descartada:
# a baseline (código real, sem corrupção) é limpa (assert_succeeds); só a
# corrupção reintroduz a violação (assert_fails_with).
#
# Direção A — guard do comentário HTML neutralizado (Go: `if
# isHTMLCommentOnlyValue(rest)` → `if false`; Node/Python equivalentes):
# reintroduz o defeito 1 — placeholder volta a contar como vínculo real, a
# violação "has no linked ADR" desaparece.
#
# Direção B — ancoragem por linha neutralizada (Go: `if
# !strings.HasPrefix(leading, marker)` → `if !strings.Contains(line,
# marker)`; Node/Python equivalentes): reintroduz o defeito 2 — prosa com o
# marcador no meio volta a contar como vínculo real, a violação "has no
# linked Roadmap" desaparece.
# ---------------------------------------------------------------------------

S192_MSG_ADR='has no linked ADR'
S192_MSG_ROADMAP='has no linked Roadmap'

# --- Go: baseline limpo (nenhuma das duas falsas-positivas) ---------------
T192_GO_BIN="$WORK/s192-go-bin/trackfw"
mkdir -p "$(dirname "$T192_GO_BIN")"
build_go_or_fail "setup-s192-go-baseline-build" "$ROOT_DIR" "$T192_GO_BIN"

# Fixture A: só a falsa-positiva do ADR (Roadmap: aponta para um alvo real).
T192_GO_PROJECT_A="$WORK/s192-go-project-a"
scaffold_adr_req_project "$T192_GO_PROJECT_A"
write_roadmap_link_target_fixture \
  "$T192_GO_PROJECT_A/docs/roadmaps/wip/ROADMAP-2026-09-06-s192-target.md" \
  "docs/req/REQ-2026-09-06-adr-placeholder-fixture.md"
write_req_adr_placeholder_fixture \
  "$T192_GO_PROJECT_A/docs/req/REQ-2026-09-06-adr-placeholder-fixture.md" \
  "docs/roadmaps/wip/ROADMAP-2026-09-06-s192-target.md"

# Fixture B: só a falsa-positiva do Roadmap (ADR: aponta para um alvo real).
T192_GO_PROJECT_B="$WORK/s192-go-project-b"
scaffold_adr_req_project "$T192_GO_PROJECT_B"
write_adr_status_fixture "$T192_GO_PROJECT_B/docs/adr/ADR-2026-09-06-s192-target.md" "Accepted"
write_req_roadmap_prose_fixture \
  "$T192_GO_PROJECT_B/docs/req/REQ-2026-09-06-roadmap-prose-fixture.md" \
  "docs/adr/ADR-2026-09-06-s192-target.md"

assert_fails_with "structural-marker-value/go/adr-placeholder-baseline" \
  "$S192_MSG_ADR" \
  bash -c "cd '$T192_GO_PROJECT_A' && exec '$T192_GO_BIN' validate"
assert_fails_with "structural-marker-value/go/roadmap-prose-baseline" \
  "$S192_MSG_ROADMAP" \
  bash -c "cd '$T192_GO_PROJECT_B' && exec '$T192_GO_BIN' validate"

# --- Go: direção A — guard do comentário HTML neutralizado ----------------
T192C_GO_A="$WORK/s192-corrupt-go-a"
mkdir -p "$T192C_GO_A/cmd" "$T192C_GO_A/internal"
cp -r "$ROOT_DIR/cmd/." "$T192C_GO_A/cmd/"
cp -r "$ROOT_DIR/internal/." "$T192C_GO_A/internal/"
cp "$ROOT_DIR/go.mod" "$T192C_GO_A/go.mod"
cp "$ROOT_DIR/go.sum" "$T192C_GO_A/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T192C_GO_A/internal/validator/validator.go" \
  'if isHTMLCommentOnlyValue(rest) {' \
  'if false { // [falsified] was: isHTMLCommentOnlyValue(rest)' \
  "s192-go-direction-a"
T192C_GO_A_BIN="$WORK/s192-corrupt-go-a-bin/trackfw"
mkdir -p "$(dirname "$T192C_GO_A_BIN")"
build_go_or_fail "setup-s192-go-a-build" "$T192C_GO_A" "$T192C_GO_A_BIN"

assert_lacks_pattern "structural-marker-value/go/adr-placeholder-detects-regression" \
  "$S192_MSG_ADR" \
  bash -c "cd '$T192_GO_PROJECT_A' && exec '$T192C_GO_A_BIN' validate"

# --- Go: direção B — ancoragem por linha neutralizada ----------------------
T192C_GO_B="$WORK/s192-corrupt-go-b"
mkdir -p "$T192C_GO_B/cmd" "$T192C_GO_B/internal"
cp -r "$ROOT_DIR/cmd/." "$T192C_GO_B/cmd/"
cp -r "$ROOT_DIR/internal/." "$T192C_GO_B/internal/"
cp "$ROOT_DIR/go.mod" "$T192C_GO_B/go.mod"
cp "$ROOT_DIR/go.sum" "$T192C_GO_B/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T192C_GO_B/internal/validator/validator.go" \
  'if !strings.HasPrefix(leading, marker) {' \
  'if !strings.Contains(line, marker) { // [falsified] was: !strings.HasPrefix(leading, marker)' \
  "s192-go-direction-b"
T192C_GO_B_BIN="$WORK/s192-corrupt-go-b-bin/trackfw"
mkdir -p "$(dirname "$T192C_GO_B_BIN")"
build_go_or_fail "setup-s192-go-b-build" "$T192C_GO_B" "$T192C_GO_B_BIN"

assert_lacks_pattern "structural-marker-value/go/roadmap-prose-detects-regression" \
  "$S192_MSG_ROADMAP" \
  bash -c "cd '$T192_GO_PROJECT_B' && exec '$T192C_GO_B_BIN' validate"
# ---------------------------------------------------------------------------
# Cenário 193 — ROADMAP-2026-09-05-reconciliar-o-que-declaramos-com-o-que-
# medimos-apos-a-auditoria-externa, ML-3B: vínculo Roadmap: guarda o caminho
# COM a pasta de estado, e `trackfw roadmap move` quebra o vínculo por
# construção (medido pelo arquiteto em 2026-09-06 via `./bin/trackfw
# validate`: 2 REQs apontavam para roadmaps que existiam em docs/roadmaps/
# done/ com "which does not exist", porque o campo gravado ainda dizia
# .../wip/...).
#
# A correção resolve o campo `Roadmap:` pelo BASENAME nos diretórios de
# ESTADO (backlog/analyzing/wip/blocked/done/abandoned) quando o caminho
# literal falha E o segmento imediatamente anterior ao arquivo é um nome de
# estado reconhecido — internal/validator/validator.go:
# resolveRoadmapRef/resolveRoadmapRefByBasename/isStaleRoadmapStateRef (e
# equivalentes em npm/src/validator/index.js e pypi/trackfw/validator.py).
#
# Escopo DELIBERADAMENTE restrito ao campo Roadmap: — não REQ:, não ADR:.
# REQ não tem dimensão de estado (ADR-2026-09-03, invariante D1) e um
# fallback por basename ali tornaria vácuo o Cenário 25 acima (que depende
# de "REQ-flag-source.md", sem diretório, NUNCA resolver por basename — é
# exatamente a regressão coberta por
# ADR-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-
# parametro-roots-morto). A árvore de ADR é flat (sem pastas de estado —
# ver `docs/adr/`), então não há hierarquia equivalente a resolver ali.
#
# Três direções de falsificação, nos 3 CLIs:
#   A) roadmap movido de wip/ para done/, REQ intacta ⇒ SEM aviso de
#      vínculo quebrado (antes do fix: "which does not exist"), MAS COM um
#      aviso verdadeiro de "stale state path" — resolver em silêncio total
#      faria validate discordar do serve (internal/serve/api_chain.go casa
#      edge.To pelo caminho LITERAL e desenharia aresta órfã para o mesmo
#      vínculo). Corrigido em 2026-09-06 após auditoria apontar que a forma
#      original ("sem aviso nenhum") contradizia esse achado.
#   B) REQ apontando para um basename que não existe em estado algum ⇒
#      AVISA "which does not exist" (guarda de vacuidade: prova que o
#      fallback por basename não virou um "sempre encontra").
#   C) REQ com status Open e Roadmap (caminho gravado desatualizado) em
#      done/ ⇒ AVISA "is Open but linked Roadmap ... is in done/", mesmo
#      com o caminho gravado velho (prova que req_roadmap_lifecycle deixou
#      de ser fail-open — antes, o Stat no caminho velho falhava e a regra
#      dava `continue`, desligando-se exatamente no caso que existe para
#      pegar).
#
# Corrompe a IMPLEMENTAÇÃO (validador), nunca a asserção — mesmo padrão dos
# Cenários 14/16/17/20/21/24/26/192. Direções A+C são cobertas pela MESMA
# corrupção (isStaleRoadmapStateRef sempre false neutraliza o fallback por
# completo, reproduzindo o comportamento pré-fix nos dois pontos que o
# consomem); direção B usa uma corrupção distinta (o fallback por basename
# passa a "encontrar" qualquer coisa, mascarando um vínculo genuinamente
# ausente).
# ---------------------------------------------------------------------------

S193_MSG_BROKEN='links to Roadmap "docs/roadmaps/wip/ROADMAP-2026-09-06-s193-fixture.md" which does not exist'
S193_MSG_STALE='req "REQ-2026-09-06-s193-fixture.md" links to Roadmap "docs/roadmaps/wip/ROADMAP-2026-09-06-s193-fixture.md" but the file is now in done/ (stale state path)'
S193_MSG_LIFECYCLE='req "REQ-2026-09-06-s193-fixture.md" is Open but linked Roadmap "docs/roadmaps/wip/ROADMAP-2026-09-06-s193-fixture.md" is in done/'
S193_MSG_VACUITY='links to Roadmap "docs/roadmaps/wip/ROADMAP-2026-09-06-s193-vacuity-absent.md" which does not exist'


# --- Go: fixtures compartilhadas (baseline usa o binário real; corrupção usa cópia isolada) ---
T193_G_LIFECYCLE="$WORK/s193-go-lifecycle"
mkdir -p "$T193_G_LIFECYCLE"
scaffold_adr_req_project "$T193_G_LIFECYCLE"
write_adr_status_fixture "$T193_G_LIFECYCLE/docs/adr/ADR-2026-09-06-s193-fixture.md" "Accepted"
write_s193_lifecycle_req_fixture \
  "$T193_G_LIFECYCLE/docs/req/REQ-2026-09-06-s193-fixture.md" \
  "docs/adr/ADR-2026-09-06-s193-fixture.md"
write_s193_done_roadmap_fixture \
  "$T193_G_LIFECYCLE/docs/roadmaps/done/ROADMAP-2026-09-06-s193-fixture.md" \
  "docs/req/REQ-2026-09-06-s193-fixture.md"

T193_G_VACUITY="$WORK/s193-go-vacuity"
mkdir -p "$T193_G_VACUITY"
scaffold_adr_req_project "$T193_G_VACUITY"
write_adr_status_fixture "$T193_G_VACUITY/docs/adr/ADR-2026-09-06-s193-fixture.md" "Accepted"
write_s193_vacuity_req_fixture \
  "$T193_G_VACUITY/docs/req/REQ-2026-09-06-s193-vacuity-fixture.md" \
  "docs/adr/ADR-2026-09-06-s193-fixture.md"

T193_GO_BASE_BIN="$WORK/s193-go-base-bin/trackfw"
mkdir -p "$(dirname "$T193_GO_BASE_BIN")"
build_go_or_fail "setup-s193-go-baseline-build" "$ROOT_DIR" "$T193_GO_BASE_BIN"

# Direção A (baseline): sem aviso de vínculo quebrado, mas COM o aviso
# verdadeiro de stale state path (silêncio total já foi provado errado pela
# auditoria — ver comentário do cenário).
assert_output_lacks "roadmap-ref-stale-state/go/broken-link-baseline" \
  "$S193_MSG_BROKEN" \
  bash -c "cd '$T193_G_LIFECYCLE' && exec '$T193_GO_BASE_BIN' validate"
assert_output_contains "roadmap-ref-stale-state/go/stale-warning-baseline" \
  "$S193_MSG_STALE" \
  bash -c "cd '$T193_G_LIFECYCLE' && exec '$T193_GO_BASE_BIN' validate"
# Direção C (baseline): aviso de ciclo de vida presente, exit 0 (é warning).
assert_output_contains "roadmap-ref-stale-state/go/lifecycle-baseline" \
  "$S193_MSG_LIFECYCLE" \
  bash -c "cd '$T193_G_LIFECYCLE' && exec '$T193_GO_BASE_BIN' validate"
# Direção B (baseline): vínculo genuinamente ausente ainda reprova.
assert_fails_with "roadmap-ref-stale-state/go/vacuity-baseline" \
  "$S193_MSG_VACUITY" \
  bash -c "cd '$T193_G_VACUITY' && exec '$T193_GO_BASE_BIN' validate"

# Direções A+C — corrupção: isStaleRoadmapStateRef sempre false (neutraliza
# o fallback por completo).
T193C_GO_AC="$WORK/s193-corrupt-go-ac"
mkdir -p "$T193C_GO_AC/cmd" "$T193C_GO_AC/internal"
cp -r "$ROOT_DIR/cmd/." "$T193C_GO_AC/cmd/"
cp -r "$ROOT_DIR/internal/." "$T193C_GO_AC/internal/"
cp "$ROOT_DIR/go.mod" "$T193C_GO_AC/go.mod"
cp "$ROOT_DIR/go.sum" "$T193C_GO_AC/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T193C_GO_AC/internal/validator/validator.go" \
  $'\tdir := filepath.Base(filepath.Dir(filepath.ToSlash(ref)))\n\treturn agentNamespaceStateNames[dir]\n}' \
  $'\tdir := filepath.Base(filepath.Dir(filepath.ToSlash(ref)))\n\t_ = dir\n\treturn false // [falsified] fallback por basename desligado\n}' \
  "s193-go-direction-ac"

T193C_GO_AC_BIN="$WORK/s193-corrupt-go-ac-bin/trackfw"
mkdir -p "$(dirname "$T193C_GO_AC_BIN")"
build_go_or_fail "setup-s193-go-ac-build" "$T193C_GO_AC" "$T193C_GO_AC_BIN"

assert_output_contains "roadmap-ref-stale-state/go/broken-link-detects-regression" \
  "$S193_MSG_BROKEN" \
  bash -c "cd '$T193_G_LIFECYCLE' && exec '$T193C_GO_AC_BIN' validate"
assert_output_lacks "roadmap-ref-stale-state/go/lifecycle-detects-regression" \
  "$S193_MSG_LIFECYCLE" \
  bash -c "cd '$T193_G_LIFECYCLE' && exec '$T193C_GO_AC_BIN' validate"
assert_output_lacks "roadmap-ref-stale-state/go/stale-warning-detects-regression" \
  "$S193_MSG_STALE" \
  bash -c "cd '$T193_G_LIFECYCLE' && exec '$T193C_GO_AC_BIN' validate"

# Direção B — corrupção: resolveRoadmapRefByBasename sempre "encontra" algo.
T193C_GO_B="$WORK/s193-corrupt-go-b"
mkdir -p "$T193C_GO_B/cmd" "$T193C_GO_B/internal"
cp -r "$ROOT_DIR/cmd/." "$T193C_GO_B/cmd/"
cp -r "$ROOT_DIR/internal/." "$T193C_GO_B/internal/"
cp "$ROOT_DIR/go.mod" "$T193C_GO_B/go.mod"
cp "$ROOT_DIR/go.sum" "$T193C_GO_B/go.sum"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$T193C_GO_B/internal/validator/validator.go" \
  $'\tsort.Strings(found)\n\treturn found\n}\n\n// isStaleRoadmapStateRef' \
  $'\tfound = append(found, "/dev/null/s193-falsified-always-found")\n\tsort.Strings(found)\n\treturn found\n}\n\n// isStaleRoadmapStateRef' \
  "s193-go-direction-b"

T193C_GO_B_BIN="$WORK/s193-corrupt-go-b-bin/trackfw"
mkdir -p "$(dirname "$T193C_GO_B_BIN")"
build_go_or_fail "setup-s193-go-b-build" "$T193C_GO_B" "$T193C_GO_B_BIN"

assert_output_lacks "roadmap-ref-stale-state/go/vacuity-detects-regression" \
  "$S193_MSG_VACUITY" \
  bash -c "cd '$T193_G_VACUITY' && exec '$T193C_GO_B_BIN' validate"

falsify_count_success
echo "OK   [falsify/roadmap-ref-stale-state/go]: as 3 direções (A/B/C) provadas"
falsify_count_success
echo "OK   [falsify/roadmap-ref-stale-state/python]: as 3 direções (A/B/C) provadas"

# ---------------------------------------------------------------------------
# Cenário 194 — check-write-containment.sh: gate de contenção de escrita
#               nasce falsificável (ML-2A, ROADMAP-2026-08-31-guarda-de-folha).
#
# Três braços, obrigatórios pelo roadmap ML-2A:
#
# Braço A (write-containment/unguarded-write):
#   Afirma que o gate DETECTA um sítio de escrita sem marcador — prova que
#   o padrão de varredura (os.WriteFile etc.) funciona e o critério de
#   reprovação dispara.
#
# Braço B (write-containment/marker-accepted):
#   Afirma que o gate ACEITA o mesmo sítio quando o marcador
#   write-containment-allowed: está presente na linha imediatamente acima —
#   prova que a lógica de isenção por marcador funciona sem falso positivo.
#
# Braço C (write-containment/vacuous-scan):
#   Afirma que o gate REPROVA corpus vazio — prova que a guarda de
#   vacuidade dispara e o gate não reporta aprovação silenciosa.
# ---------------------------------------------------------------------------
T194="$WORK/s194"
mkdir -p "$T194/scripts"
cp "$ROOT_DIR/scripts/check-write-containment.sh" "$T194/scripts/"

# Braço A — os.WriteFile cru sem marcador → gate REPROVA
T194A="$T194/arm-a"
mkdir -p "$T194A/internal/pkg"
cat > "$T194A/internal/pkg/example.go" <<'GOEOF'
package pkg

import "os"

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
GOEOF

assert_fails_with "write-containment/unguarded-write" \
  "unjustified write" \
  bash -c "WRITE_CONTAINMENT_SCAN_DIR='$T194A' bash '$T194/scripts/check-write-containment.sh'"

# Braço B — mesmo sítio COM o marcador → gate PASSA
T194B="$T194/arm-b"
mkdir -p "$T194B/internal/pkg"
cat > "$T194B/internal/pkg/example.go" <<'GOEOF'
package pkg

import "os"

func writeFile(path string, data []byte) error {
	// write-containment-allowed: test fixture, fixed path not derived from user root
	return os.WriteFile(path, data, 0644)
}
GOEOF

# assert_output_lacks prova que o braço (b) NÃO produz "unjustified write"
# e que o gate sai com 0 (aceita o marcador).
assert_output_lacks "write-containment/marker-accepted" \
  "unjustified write" \
  bash -c "WRITE_CONTAINMENT_SCAN_DIR='$T194B' bash '$T194/scripts/check-write-containment.sh'"

# Braço C — corpus vazio → gate REPROVA por vácuo
T194C="$T194/arm-c"
mkdir -p "$T194C/internal"

assert_fails_with "write-containment/vacuous-scan" \
  "recusando reportar aprovação silenciosa" \
  bash -c "WRITE_CONTAINMENT_SCAN_DIR='$T194C' bash '$T194/scripts/check-write-containment.sh'"

falsify_count_success
echo "OK   [falsify/write-containment]: os 3 braços (A/B/C) provados"

# ---------------------------------------------------------------------------
# Cenário 195 — check-parity-call-site-pins.sh: asserção negativa de que
#               make quality não alcança o pin TRACKFW_SELF_GOVERNED= (ML-1C).
#
# Objetivo: provar que a nova verificação VARS_FORBIDDEN_IN_QUALITY detecta a
# regressão exata do ML-1B-bis — o pin reintroduzido em parity-rest, ou movido
# para outro alvo alcançável por make quality (ex.: parity-falsify).
#
# Reconciliação (Regra Dura):
#   call-site-pin/self-governed-in-quality: o gate REPROVA quando o pin
#     TRACKFW_SELF_GOVERNED=1 é reintroduzido no alvo parity-rest. O arquiteto
#     mediu RC=0 no gate antigo com a mesma mutação (roadmap ML-1B-bis, seção
#     "achado da auditoria") — a nova verificação é a causa única da reprovação.
#   call-site-pin/self-governed-moved: o gate REPROVA quando o pin é movido
#     para parity-falsify, alvo alcançado por quality através de parity mas
#     não lido por varredura textual de parity-rest. O discriminante make -n
#     resolve dependências entre alvos e captura o caso que leitura direta do
#     Makefile não pegaria.
#   call-site-pin/self-governed-clean: o gate PASSA na árvore correta, onde o
#     pin existe exclusivamente em self-governance (fora da cadeia quality).
# ---------------------------------------------------------------------------
T195="$WORK/s195"
mkdir -p "$T195/scripts"
cp "$ROOT_DIR/Makefile"    "$T195/"
cp "$ROOT_DIR/scripts/"*.sh "$T195/scripts/"

# Braço C — árvore correta: gate PASSA (baseline antes das mutações).
if ! bash "$T195/scripts/check-parity-call-site-pins.sh" "$T195" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s195-baseline]: check-parity-call-site-pins.sh já reprova com fonte real -- prova inválida" >&2
  falsify_count_failure
  [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] || exit 1
fi

assert_succeeds "call-site-pin/self-governed-clean" \
  bash "$T195/scripts/check-parity-call-site-pins.sh" "$T195"

# Braço A — pin reintroduzido em parity-rest: gate REPROVA.
# Mutação: acrescenta TRACKFW_SELF_GOVERNED=1 na linha de parity-rest que
# invoca check-roadmap-barrier-contract.sh (a regressão exata do ML-1B-bis).
T195A="$WORK/s195-arm-a"
mkdir -p "$T195A/scripts"
cp "$T195/scripts/"*.sh "$T195A/scripts/"
python3 - "$T195/Makefile" "$T195A/Makefile" <<'PYEOF'
import sys
src, dst = sys.argv[1], sys.argv[2]
with open(src) as f:
    lines = f.readlines()
out = []
for line in lines:
    # Target: a única linha de recipe que invoca check-roadmap-barrier-contract.sh
    # SEM o pin (alvo parity-rest) — adicionar o pin para simular regressão.
    stripped = line.lstrip('\t')
    if ('scripts/check-roadmap-barrier-contract.sh' in stripped
            and 'TRACKFW_SELF_GOVERNED' not in stripped
            and stripped.startswith('GO_BIN=')):
        line = line.replace('GO_BIN=', 'TRACKFW_SELF_GOVERNED=1 GO_BIN=', 1)
    out.append(line)
with open(dst, 'w') as f:
    f.writelines(out)
PYEOF

assert_fails_with "call-site-pin/self-governed-in-quality" \
  "forbidden-in-quality" \
  bash "$T195A/scripts/check-parity-call-site-pins.sh" "$T195A"

# Braço B — pin movido para parity-falsify: gate REPROVA.
# Mutação: insere uma linha após a invocação de run-gates-falsify-parallel.sh
# no alvo parity-falsify, adicionando uma chamada pinada ao script consumidor.
# Este alvo é alcançado por quality→parity→parity-falsify mas não seria
# detectado por varredura textual de parity-rest.
T195B="$WORK/s195-arm-b"
mkdir -p "$T195B/scripts"
cp "$T195/scripts/"*.sh "$T195B/scripts/"
python3 - "$T195/Makefile" "$T195B/Makefile" <<'PYEOF'
import sys
src, dst = sys.argv[1], sys.argv[2]
with open(src) as f:
    lines = f.readlines()
out = []
for line in lines:
    out.append(line)
    # Após a linha de parity-falsify que invoca run-gates-falsify-parallel.sh,
    # inserir uma chamada pinada ao script consumidor.
    stripped = line.lstrip('\t')
    if 'scripts/run-gates-falsify-parallel.sh' in stripped and stripped.startswith('GO_BIN='):
        out.append('\tTRACKFW_SELF_GOVERNED=1 GO_BIN=$(BUILD_DIR)/$(BINARY) HASH_CMD_BIN="$(HASH_CMD)" scripts/check-roadmap-barrier-contract.sh\n')
with open(dst, 'w') as f:
    f.writelines(out)
PYEOF

assert_fails_with "call-site-pin/self-governed-moved" \
  "forbidden-in-quality" \
  bash "$T195B/scripts/check-parity-call-site-pins.sh" "$T195B"

echo "OK   [falsify/call-site-pin]: 3 braços (clean/in-quality/moved) provados"

# ---------------------------------------------------------------------------
# Cenário 196 — check-parity-call-site-pins.sh: verificação de que o CI
#               workflow invoca make self-governance (ML-2A, REQ #396).
#
# Objetivo: provar que a nova verificação ci-workflow/self-governance-invoked
# detecta a remoção silenciosa do step `make self-governance` do CI yaml,
# incluindo o caso onde a linha `run:` é substituída por um comentário.
# Medido por hades-tf R-C e hefesto-tf Q4 em 2026-09-22: a remoção do step
# não reprova parity (required check) nem check-parity-call-site-pins.sh antes
# deste cenário.
#
# Reconciliação (Regra Dura):
#   ci-workflow/self-governance-invoked/clean: o gate PASSA na árvore correta,
#     onde quality.yml contém `run: make self-governance` em linha não-comentário.
#   ci-workflow/self-governance-invoked/run-removed: o gate REPROVA quando a
#     linha `run: make self-governance` é removida, mantendo o `name:` do step —
#     exatamente o cenário de remoção silenciosa que motivou este check.
#   ci-workflow/self-governance-invoked/run-commented: o gate REPROVA quando a
#     linha `run:` é substituída por comentário `# make self-governance` — prova
#     que o grep filtro de comentário funciona (sem ele, este caso passaria).
# ---------------------------------------------------------------------------
T196="$WORK/s196"
mkdir -p "$T196/scripts" "$T196/.github/workflows"
cp "$ROOT_DIR/Makefile"      "$T196/"
cp "$ROOT_DIR/scripts/"*.sh  "$T196/scripts/"
cp "$ROOT_DIR/.github/workflows/quality.yml" "$T196/.github/workflows/"

# Braço C — árvore correta: gate PASSA (baseline antes das mutações).
if ! bash "$T196/scripts/check-parity-call-site-pins.sh" "$T196" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s196-baseline]: check-parity-call-site-pins.sh já reprova com fonte real -- prova inválida" >&2
  falsify_count_failure
  [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] || exit 1
fi

assert_succeeds "ci-workflow/self-governance-invoked/clean" \
  bash "$T196/scripts/check-parity-call-site-pins.sh" "$T196"

# Braço A — run: line removed, name: line kept: gate REPROVA.
# Mutação: remove a linha `run: make self-governance` do workflow, mantendo
# o `name:` acima. É o cenário exato de remoção silenciosa do step.
T196A="$WORK/s196-arm-a"
mkdir -p "$T196A/scripts" "$T196A/.github/workflows"
cp "$T196/Makefile"     "$T196A/"
cp "$T196/scripts/"*.sh "$T196A/scripts/"
grep -v '^\s*run: make self-governance' "$T196/.github/workflows/quality.yml" \
  > "$T196A/.github/workflows/quality.yml"

assert_fails_with "ci-workflow/self-governance-invoked/run-removed" \
  "ci-workflow/self-governance-invoked" \
  bash "$T196A/scripts/check-parity-call-site-pins.sh" "$T196A"

# Braço B — run: replaced with comment: gate REPROVA.
# Mutação: substitui `run: make self-governance` por `# make self-governance`.
# Prova que o filtro de linha-comentário não pode ser iludido.
T196B="$WORK/s196-arm-b"
mkdir -p "$T196B/scripts" "$T196B/.github/workflows"
cp "$T196/Makefile"     "$T196B/"
cp "$T196/scripts/"*.sh "$T196B/scripts/"
sed 's/^\( *\)run: make self-governance$/\1# make self-governance/' \
  "$T196/.github/workflows/quality.yml" \
  > "$T196B/.github/workflows/quality.yml"

assert_fails_with "ci-workflow/self-governance-invoked/run-commented" \
  "ci-workflow/self-governance-invoked" \
  bash "$T196B/scripts/check-parity-call-site-pins.sh" "$T196B"

echo "OK   [falsify/ci-workflow-self-governance]: 3 braços (clean/run-removed/run-commented) provados"

# ---------------------------------------------------------------------------
# Cenário 197 — check-crlf-normalize-capture.sh: gate anti-reintrodução de
#               captura de stdout de python3 sem normalização CRLF (ML-1B,
#               ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf...).
#
# Braço A (crlf-normalize/unnormalized-capture):
#   Afirma que o gate REPROVA um script novo com $(python3 ...) sem strip_cr.
#   Prova que o discriminante estrutural (ausência de strip_cr no bloco) dispara.
#
# Braço B (crlf-normalize/normalized-capture):
#   Afirma que o gate APROVA o mesmo padrão quando strip_cr está no pipeline.
#   Prova que o braço de normalização correto passa sem falso positivo.
#
# Braço C (crlf-normalize/vacuous-scan):
#   Afirma que o gate REPROVA corpus abaixo do piso — guarda de vacuidade.
#   Diagnostico distinto de "vacuity guard tripped" (não confunde com braço A).
#
# Nota sobre auto-referência: o arquivo sintetico com a captura ruim é montado
# por concatenação (variável PY + printf) para que o literal $(python3 nunca
# apareça verbatim neste source — o gate escaneia scripts/*.sh incluindo este.
# ---------------------------------------------------------------------------
T197="$WORK/s197"
mkdir -p "$T197/scripts"
cp "$ROOT_DIR/scripts/check-crlf-normalize-capture.sh" "$T197/scripts/"
# Lib necessária para que o script sintetico "passe de verdade" no braço B.
cp "$ROOT_DIR/scripts/lib-crlf-normalize.sh" "$T197/scripts/"

# ---------------------------------------------------------------------------
# Braço A — captura sem strip_cr → gate REPROVA
# Monta a linha com variável para que $(python3 não apareça verbatim aqui.
# ---------------------------------------------------------------------------
T197A="$WORK/s197/arm-a"
mkdir -p "$T197A/scripts"
cp "$T197/scripts/lib-crlf-normalize.sh" "$T197A/scripts/"
PY=python3
printf '#!/usr/bin/env bash\nSCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"\n. "$SCRIPT_DIR/lib-crlf-normalize.sh"\n' \
  > "$T197A/scripts/check-bad-capture.sh"
# Append the violating capture assembled from variable — literal $(python3 never appears below.
printf 'BAD_VAR=$(%s -c '"'"'print("hello")'"'"')\necho "$BAD_VAR"\n' "$PY" \
  >> "$T197A/scripts/check-bad-capture.sh"

# CRLF_GATE_MIN_CAPTURES=1: corpus has exactly 1 capture; without the override
# the floor (50) would trip first and give "vacuity guard tripped" instead of
# "python3 capture without strip_cr" — the two diagnostics are distinct strings
# by gate design so assert_fails_with can discriminate between them.
assert_fails_with "crlf-normalize/unnormalized-capture" \
  "python3 capture without strip_cr" \
  env CRLF_GATE_MIN_CAPTURES=1 bash "$T197/scripts/check-crlf-normalize-capture.sh" \
  --scan-root "$T197A"

# ---------------------------------------------------------------------------
# Braço B — captura com strip_cr → gate PASSA
# ---------------------------------------------------------------------------
T197B="$WORK/s197/arm-b"
mkdir -p "$T197B/scripts"
cp "$T197/scripts/lib-crlf-normalize.sh" "$T197B/scripts/"
printf '#!/usr/bin/env bash\nSCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"\n. "$SCRIPT_DIR/lib-crlf-normalize.sh"\n' \
  > "$T197B/scripts/check-good-capture.sh"
printf 'GOOD_VAR=$(%s -c '"'"'print("hello")'"'"' | strip_cr)\necho "$GOOD_VAR"\n' "$PY" \
  >> "$T197B/scripts/check-good-capture.sh"

assert_succeeds "crlf-normalize/normalized-capture" \
  env CRLF_GATE_MIN_CAPTURES=1 bash "$T197/scripts/check-crlf-normalize-capture.sh" \
  --scan-root "$T197B"

# ---------------------------------------------------------------------------
# Braço C — corpus abaixo do piso → gate REPROVA por vacuidade
# Cria um script com uma captura normalizada mas seta MIN_CAPTURES=5 (>1).
# ---------------------------------------------------------------------------
T197C="$WORK/s197/arm-c"
mkdir -p "$T197C/scripts"
cp "$T197/scripts/lib-crlf-normalize.sh" "$T197C/scripts/"
printf '#!/usr/bin/env bash\nSCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"\n. "$SCRIPT_DIR/lib-crlf-normalize.sh"\n' \
  > "$T197C/scripts/check-ok-capture.sh"
printf 'VAR=$(%s -c '"'"'print("hi")'"'"' | strip_cr)\necho "$VAR"\n' "$PY" \
  >> "$T197C/scripts/check-ok-capture.sh"

assert_fails_with "crlf-normalize/vacuous-scan" \
  "vacuity guard tripped" \
  env CRLF_GATE_MIN_CAPTURES=5 bash "$T197/scripts/check-crlf-normalize-capture.sh" \
  --scan-root "$T197C"

echo "OK   [falsify/crlf-normalize]: 3 braços (A/B/C) provados"

# ---------------------------------------------------------------------------
# ML-2B — fechamento do modo de enumeração. Desligado (default): este bloco
# inteiro é pulado (a condição é falsa) e a saída do processo é a do último
# comando acima -- 0, exatamente como antes deste ML (byte-idêntico: nenhuma
# linha nova, nenhum exit code novo). Ligado: se qualquer cenário reprovou
# (contado em $FALSIFY_ENUM_TALLY -- arquivo, não variável, ver nota do
# bloco de definição acima sobre subshell), o exit final é != 0 -- guarda 1
# (nunca torna o gate verde) também no ponto de saída, não só em cada ponto
# de falha.
if [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]]; then
  falsify_enum_n=$(wc -l < "$FALSIFY_ENUM_TALLY" 2>/dev/null || echo 0)
  falsify_enum_n=${falsify_enum_n//[[:space:]]/}
  if [[ "${falsify_enum_n:-0}" -gt 0 ]]; then
    echo "[falsify/enumerate] $falsify_enum_n cenário(s) reprovaram (enumerados acima, cada um prefixado FAIL) -- exit 1" >&2
    exit 1
  fi
  echo "[falsify/enumerate] 0 cenários reprovaram -- exit 0" >&2
fi

# Imprime o número medido de asserções bem-sucedidas. Usa o mesmo arquivo
# usado por falsify_count_success — medição da execução, não grep de fonte.
# O contador cobre todas as linhas "OK   [falsify/..." emitidas por este
# script (helpers + blocos inline). Sub-scripts externos como
# check-wheel-filename.sh emitem suas próprias linhas OK sem incrementar
# este contador — são ~2-7 linhas adicionais que não alteram a contagem aqui.
falsify_success_n=$(wc -l < "$FALSIFY_SUCCESS_TALLY" 2>/dev/null || echo 0)
falsify_success_n=${falsify_success_n//[[:space:]]/}

# Guarda de vacuidade: o número medido deve ser pelo menos FALSIFY_SUCCESS_FLOOR.
# Se ficar abaixo, algo removeu chamadas de falsify_count_success ou o
# arquivo de tally ficou inacessível -- cenário que produz "passed" sem medir
# nada, exatamente a classe de defeito que este script existe para eliminar.
# Vale nos dois modos (normal e TRACKFW_FALSIFY_ENUMERATE=1): no modo de
# enumeração já chegamos aqui sem reprovações de cenário (o bloco acima sairia
# com exit 1 se tivesse), mas o piso ainda se aplica porque o tally pode estar
# zerado por razão diferente (tally inacessível, refator removeu contadores).
#
# NÃO se aplica em chunk: gen-falsify-chunks.py injeta a função
# __falsify_timing_mark em cada chunk gerado (preâmbulo do chunk), mas NUNCA
# no script original. A presença dessa função é o sinal confiável de que
# este código está rodando dentro de um chunk paralelo -- cada chunk
# carrega apenas uma fração dos ~201 cenários e, por construção, ficaria
# abaixo do piso mesmo num run limpo. A guarda de completude do driver
# paralelo (sentinela CHUNK_COMPLETE + rótulos esperados) cobre o chunk;
# a guarda de vacuidade abaixo cobre o run direto do script completo.
if ! declare -f __falsify_timing_mark &>/dev/null; then
  if [[ "${falsify_success_n:-0}" -lt "$FALSIFY_SUCCESS_FLOOR" ]]; then
    echo "FAIL [falsify/vacuity-guard] apenas ${falsify_success_n:-0} cenário(s) contados, piso é $FALSIFY_SUCCESS_FLOOR -- provável remoção de chamadas falsify_count_success por refator ou tally inacessível; atualize FALSIFY_SUCCESS_FLOOR no mesmo commit que remover cenários" >&2
    exit 1
  fi
fi
echo "Falsification checks passed (${falsify_success_n:-0} scenarios)"
