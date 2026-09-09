#!/usr/bin/env bash
# check-git-branch-guard-hook-schema.sh — prova, POR EXECUÇÃO e decode JSON
# estruturado (nunca por substring), que o script git-branch-guard emite a
# forma `hookSpecificOutput` que o Claude Code aceita hoje, no script real do
# repo e nos 3 geradores (Go/Node/Python) — ML-2A,
# ROADMAP-2026-09-09-guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-
# nos-3-clis.md.
#
# --- Por que este gate existe -----------------------------------------------
# ML-1A/1B da mesma REQ corrigiram sete sítios que emitiam
# `{"decision":"block","reason":"..."}` — schema que o Claude Code rejeita na
# RAIZ ("Hook JSON output validation failed — (root): Invalid input") — para
# `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":
# "deny","permissionDecisionReason":"..."}}`. Sem gate, o schema regride na
# próxima mudança e ninguém percebe: `exit 2` mantém o bloqueio funcionando
# (fail-closed), então a regressão é invisível para qualquer teste que só
# pergunte "o comando foi bloqueado?" — ver
# vault/notes/git-branch-guard-schema-decision-block-rejeitado-pelo-claude-code-2026-09-09.md.
#
# --- Por que EXECUÇÃO + decode estruturado, nunca substring -----------------
# Um gate que casa a string "hookSpecificOutput" aprovaria
# `{"nota":"contem hookSpecificOutput no texto mas nao e json valido"` ou um
# JSON tecnicamente válido mas com a forma errada (ex.: chave certa, valor
# errado) — reproduzindo, no próprio gate, o defeito que ele existe para
# pegar. Este gate roda o script de verdade com um comando git bloqueável, lê
# o stdout produzido em runtime, e decodifica com `json.loads` (nunca
# regex/grep no texto do JSON) contra os 4 campos que o Claude Code exige:
# `.hookSpecificOutput.hookEventName == "PreToolUse"`,
# `.hookSpecificOutput.permissionDecision == "deny"`,
# `.hookSpecificOutput.permissionDecisionReason` string não-vazia.
#
# --- Nível único: execução do script real + dos 3 gerados -------------------
# check-attention-scripts-parity.sh já prova, por execução real de
# `discover --init` nos 3 runtimes, byte-identidade entre
# scripts/trackfw-git-branch-guard.sh (repo) e o que Go/Node/Python geram —
# e os testes unitários de cada stack (TestGitBranchGuardScriptReference_
# MatchesGenerator, test_reference_e_byte_identico_ao_gerador_real, "..."
# node test, ver ML-1B) já provam que as 3 cópias-referência do `validate`
# (internal/validator/validator_git_branch_guard_reference.go,
# pypi/trackfw/validator.py, npm/src/validator/index.js) são byte-idênticas
# ao gerador do próprio stack. Cobrir os 3 sítios de referência aqui TAMBÉM,
# de novo por execução, seria redundante: eles não têm caminho de execução
# próprio (nunca rodam como hook — só existem para checar integridade de
# um script já instalado), e sua correção de forma já está implicada pela
# combinação "byte-idêntico ao gerador" (testes acima) + "o gerador emite a
# forma certa" (este gate). Decisão: este gate exercita só os sítios que
# EMITEM o JSON em runtime — o script real e os 3 geradores — e depende dos
# testes de byte-identidade já existentes (rodados em `make test`/
# `test-node`/`test-python`, parte de `make quality`) para os 3 sítios de
# referência. Se um dia esses testes de byte-identidade forem removidos sem
# substituição, os sítios de referência ficam descobertos — não é o caso
# hoje.
#
# --- Lista de sítios: DERIVADA, não congelada --------------------------------
# `derive_sites()` abaixo faz grep -rlas por "permissionDecisionReason" (o
# campo mais específico da forma nova — não corresponde a nenhum outro
# schema deste repositório) em todo o código-fonte (.go/.js/.py/.sh, exceto
# scripts/testdata/ e arquivos de TESTE), e falha se a contagem cair abaixo
# do piso medido (4: o script real + 1 emissor por stack) ou se algum dos 3
# stacks não aparecer na lista. Isso significa que um sítio novo que passe a
# emitir esta forma entra na cobertura automaticamente da próxima vez que
# alguém rodar este gate com uma lista maior — o piso é o mínimo plausível,
# não a lista inteira; ver docs/agents-working-context.md/relatório deste ML
# para a lista medida em 2026-09-09.
#
# --- Convenções seguidas (mesmas de scripts/check-attention-scripts-parity.sh) --
# set -euo pipefail, mktemp -d com trap de limpeza, ROOT_DIR relativo a
# BASH_SOURCE, resolução de GO_BIN (builda binário descartável se ausente),
# override de ROOT_DIR via TRACKFW_ROOT_DIR (mesmo mecanismo de
# check-gates-falsify.sh) — usado pelo --self-test para exercitar a guarda
# de vacuidade contra uma árvore sintética, sem duplicar a lógica do modo
# padrão.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
export PYTHONIOENCODING=utf-8

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=${TRACKFW_ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-git-branch-guard-hook-schema.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# decode_shape.py — único ponto de decisão sobre "a forma está certa?".
# Lê JSON em STDIN, decodifica ESTRUTURADO (json.loads, nunca regex/grep no
# texto), valida os 4 campos exigidos. exit 0 = forma válida (mensagem em
# stdout) | exit 1 = forma inválida (motivo em stdout, para aparecer no
# FAIL do chamador).
# ---------------------------------------------------------------------------
DECODER="$WORK/decode_shape.py"
cat >"$DECODER" <<'PY_EOF'
# -*- coding: utf-8 -*-
import json
import sys


def main():
    raw = sys.stdin.buffer.read()
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        print("INVALID_UTF8: %s" % exc)
        return 1
    try:
        obj = json.loads(text)
    except Exception as exc:
        print("INVALID_JSON: %s (stdout bruto: %r)" % (exc, text[:200]))
        return 1
    if not isinstance(obj, dict):
        print("NOT_OBJECT: raiz e %s, nao objeto" % type(obj).__name__)
        return 1
    hso = obj.get("hookSpecificOutput")
    if not isinstance(hso, dict):
        print("MISSING_hookSpecificOutput: chaves da raiz sao %s" % sorted(obj.keys()))
        return 1
    event = hso.get("hookEventName")
    if event != "PreToolUse":
        print("BAD_hookEventName: obtido %r, esperado 'PreToolUse'" % (event,))
        return 1
    decision = hso.get("permissionDecision")
    if decision != "deny":
        print("BAD_permissionDecision: obtido %r, esperado 'deny'" % (decision,))
        return 1
    reason = hso.get("permissionDecisionReason")
    if not isinstance(reason, str) or not reason.strip():
        print("EMPTY_permissionDecisionReason: obtido %r" % (reason,))
        return 1
    print("VALID: hookSpecificOutput.permissionDecision=deny, permissionDecisionReason com %d caracteres" % len(reason))
    return 0


if __name__ == "__main__":
    sys.exit(main())
PY_EOF

decode_shape() { python3 "$DECODER"; }

# ---------------------------------------------------------------------------
# check_site LABEL WORKDIR SCRIPT_RELPATH BLOCK_ARG
# Roda o script de verdade (bash "$SCRIPT_RELPATH" "$BLOCK_ARG", cwd=WORKDIR,
# igual ao runtime real do hook), exige rc=2 (fail-closed preservado — este
# gate NUNCA deve aprovar uma mudança que afrouxe o exit 2), e decodifica o
# stdout produzido contra o schema. Acumula FAIL global via fail().
# ---------------------------------------------------------------------------
check_site() {
  local label=$1 workdir=$2 script_rel=$3 block_arg=$4
  local out status decode_out decode_status
  local label_file=${label//\//_}

  # </dev/null: o comando testado chega ao guard via argumento posicional
  # ($block_arg, "$#" -gt 0), nunca via stdin — a etapa 0 do guard drena
  # stdin de qualquer forma, ANTES de olhar $#, então sem isso o `cat`
  # interno bloqueia para sempre quando o chamador (make quality) não é um
  # terminal (-t 0 falso) e não fecha a stdin. Redirecionar de /dev/null
  # fornece EOF imediato sem alterar o que este gate mede (a forma do JSON
  # de saída), porque o payload de stdin nunca é lido neste modo de
  # invocação.
  set +e
  out=$(cd "$workdir" && bash "$script_rel" "$block_arg" 2>"$WORK/$label_file.stderr" </dev/null)
  status=$?
  set -e

  if [[ "$status" -ne 2 ]]; then
    fail "$label" "esperava rc=2 (fail-closed), obteve rc=$status; stdout=$(printf '%s' "$out" | head -c 300)"
    return
  fi

  set +e
  decode_out=$(printf '%s' "$out" | decode_shape)
  decode_status=$?
  set -e

  if [[ "$decode_status" -ne 0 ]]; then
    fail "$label" "$decode_out"
    return
  fi

  ok "$label: $decode_out"
}

# ---------------------------------------------------------------------------
# derive_sites SCAN_ROOT — grep -rlas (com -a: um dos sítios,
# npm/src/validator/index.js, contém byte NUL; sem -a o grep pula o arquivo
# em SILÊNCIO, exatamente o jeito como o ML-1A perdeu esse sítio na primeira
# medição) por "permissionDecisionReason" em .go/.js/.py/.sh, excluindo
# scripts/testdata/ (corpus congelado) e arquivos de TESTE (que afirmam
# sobre a forma, não a emitem).
# ---------------------------------------------------------------------------
derive_sites() {
  local scan_root=$1
  grep -rlas "permissionDecisionReason" \
    --include='*.go' --include='*.js' --include='*.py' --include='*.sh' \
    "$scan_root" 2>/dev/null \
    | grep -v '/scripts/testdata/' \
    | grep -v '/scripts/check-' \
    | grep -Ev '(_test\.go|test_[^/]+\.py|\.test\.js)$' \
    | sort
}

# ---------------------------------------------------------------------------
# run_discover_init RUNTIME DIR — mesmo entry point de produção usado por
# check-attention-scripts-parity.sh (nunca uma API interna do gerador): prova
# que o schema novo também sobrevive ao caminho real de instalação
# (`trackfw discover --init`), não só a uma chamada direta da função Go/JS/
# Python.
# ---------------------------------------------------------------------------
run_discover_init() {
  local runtime=$1 dir=$2
  mkdir -p "$dir"
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" discover --init)                              >/dev/null 2>"$WORK/$runtime.discover.err" ;;
    node) (cd "$dir" && node "$NODE_CLI" discover --init)                       >/dev/null 2>"$WORK/$runtime.discover.err" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw discover --init) >/dev/null 2>"$WORK/$runtime.discover.err" ;;
    *)    echo "run_discover_init: runtime desconhecido '$runtime'" >&2; exit 1 ;;
  esac
}

# ===========================================================================
# --self-test — falsificação nas duas direções + guarda de vacuidade
# (padrão de scripts/check-pr-closing-keyword.sh: mesmo decode_shape() que o
# modo padrão usa, nunca uma segunda cópia da lógica de validação).
# ===========================================================================
self_test() {
  local failures=0

  # --- Cenário A: schema ERRADO ⇒ reprova, nomeando o sítio -----------------
  # Cópia de scratch do script real, com a linha do printf revertida para o
  # schema legado que o Claude Code rejeita ({"decision":"block","reason":
  # "..."}) — o schema exato que causou o defeito original desta REQ.
  local mut_dir="$WORK/self-test/wrong-schema"
  mkdir -p "$mut_dir/scripts"
  echo "namespace: prometeu-tf" >"$mut_dir/trackfw.yaml"
  cp "$ROOT_DIR/scripts/trackfw-git-branch-guard.sh" "$mut_dir/scripts/trackfw-git-branch-guard.sh"
  sed -i.bak \
    's/^printf .{"hookSpecificOutput":.*$/printf '"'"'{"decision":"block","reason":"%s"}\\n'"'"' "$REASON"/' \
    "$mut_dir/scripts/trackfw-git-branch-guard.sh"
  rm -f "$mut_dir/scripts/trackfw-git-branch-guard.sh.bak"
  if ! grep -q '"decision":"block"' "$mut_dir/scripts/trackfw-git-branch-guard.sh"; then
    echo "FAIL [self-test/wrong-schema/setup]: a mutação por sed não pegou — a linha do printf mudou de forma no script real e este self-test precisa ser atualizado" >&2
    failures=$((failures + 1))
  else
    set +e
    out=$(cd "$mut_dir" && bash scripts/trackfw-git-branch-guard.sh "git commit -m x" 2>/dev/null </dev/null)
    status=$?
    set -e
    if [[ "$status" -ne 2 ]]; then
      echo "FAIL [self-test/wrong-schema]: esperava rc=2 mesmo com schema errado (fail-closed não deveria depender do JSON), obteve rc=$status" >&2
      failures=$((failures + 1))
    else
      set +e
      decode_out=$(printf '%s' "$out" | decode_shape)
      decode_status=$?
      set -e
      if [[ "$decode_status" -eq 0 ]]; then
        echo "FAIL [self-test/wrong-schema]: decode_shape aprovou um schema que deveria reprovar — o próprio gate está cego para a regressão que ele existe para pegar" >&2
        failures=$((failures + 1))
      else
        echo "OK   [self-test/wrong-schema]: gate reprova o schema legado, nomeando o sítio ($mut_dir/scripts/trackfw-git-branch-guard.sh) — motivo: $decode_out"
      fi
    fi
  fi

  # --- Cenário B: schema CERTO ⇒ aprova --------------------------------------
  local ok_dir="$WORK/self-test/right-schema"
  mkdir -p "$ok_dir/scripts"
  echo "namespace: prometeu-tf" >"$ok_dir/trackfw.yaml"
  cp "$ROOT_DIR/scripts/trackfw-git-branch-guard.sh" "$ok_dir/scripts/trackfw-git-branch-guard.sh"
  set +e
  out=$(cd "$ok_dir" && bash scripts/trackfw-git-branch-guard.sh "git commit -m x" 2>/dev/null </dev/null)
  status=$?
  set -e
  if [[ "$status" -ne 2 ]]; then
    echo "FAIL [self-test/right-schema]: esperava rc=2, obteve rc=$status" >&2
    failures=$((failures + 1))
  else
    set +e
    decode_out=$(printf '%s' "$out" | decode_shape)
    decode_status=$?
    set -e
    if [[ "$decode_status" -ne 0 ]]; then
      echo "FAIL [self-test/right-schema]: gate reprovou o script real sem mutação nenhuma — falso positivo. Motivo: $decode_out" >&2
      failures=$((failures + 1))
    else
      echo "OK   [self-test/right-schema]: gate aprova a forma correta — $decode_out"
    fi
  fi

  # --- Cenário C1: guarda de vacuidade — NENHUM SÍTIO ENCONTRADO ------------
  # Aponta o gate inteiro (via TRACKFW_ROOT_DIR, mesmo mecanismo de
  # check-gates-falsify.sh) para uma árvore com o script real PRESENTE (para
  # isolar esta guarda da guarda C2, "script ausente") mas cujo conteúdo não
  # contém "permissionDecisionReason" nem em nenhum outro arquivo — e roda o
  # SCRIPT DE VERDADE como subprocesso, não uma cópia da lógica de vacuidade.
  local empty_root="$WORK/self-test/empty-root"
  mkdir -p "$empty_root/scripts"
  printf '#!/usr/bin/env bash\nexit 0\n' >"$empty_root/scripts/trackfw-git-branch-guard.sh"
  set +e
  out=$(TRACKFW_ROOT_DIR="$empty_root" bash "$ROOT_DIR/scripts/check-git-branch-guard-hook-schema.sh" 2>&1)
  status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    echo "FAIL [self-test/vacuidade-nenhum-sitio]: árvore vazia deveria reprovar (nenhum sítio encontrado), gate saiu 0" >&2
    failures=$((failures + 1))
  elif ! grep -qF 'nenhum sítio' <<<"$out"; then
    echo "FAIL [self-test/vacuidade-nenhum-sitio]: reprovou (rc=$status), mas sem nomear a causa ('nenhum sítio')" >&2
    printf '%s\n' "$out" | sed 's/^/    /' >&2
    failures=$((failures + 1))
  else
    echo "OK   [self-test/vacuidade-nenhum-sitio]: gate reprova (rc=$status) uma árvore sem nenhum sítio, nomeando a causa"
  fi

  # --- Cenário C2: guarda de vacuidade — SCRIPT AUSENTE ---------------------
  # Árvore sintética com sítios plausíveis o bastante para passar da guarda de
  # contagem (arquivos com "permissionDecisionReason" nos 3 stacks), mas SEM
  # scripts/trackfw-git-branch-guard.sh — o gate precisa reprovar por ausência
  # do script real, não passar por já ter achado "sítios" suficientes.
  local missing_script_root="$WORK/self-test/missing-script-root"
  mkdir -p "$missing_script_root/internal/generators" "$missing_script_root/npm/src/generators" "$missing_script_root/pypi/trackfw/generators"
  printf 'permissionDecisionReason\n' >"$missing_script_root/internal/generators/scaffold.go"
  printf 'permissionDecisionReason\n' >"$missing_script_root/npm/src/generators/hooks.js"
  printf 'permissionDecisionReason\n' >"$missing_script_root/pypi/trackfw/generators/init_gen.py"
  # scripts/trackfw-git-branch-guard.sh deliberadamente AUSENTE.
  set +e
  out=$(TRACKFW_ROOT_DIR="$missing_script_root" bash "$ROOT_DIR/scripts/check-git-branch-guard-hook-schema.sh" 2>&1)
  status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    echo "FAIL [self-test/vacuidade-script-ausente]: script real ausente deveria reprovar, gate saiu 0" >&2
    failures=$((failures + 1))
  elif ! grep -qF 'script real ausente' <<<"$out"; then
    echo "FAIL [self-test/vacuidade-script-ausente]: reprovou (rc=$status), mas sem nomear a causa ('script real ausente')" >&2
    printf '%s\n' "$out" | sed 's/^/    /' >&2
    failures=$((failures + 1))
  else
    echo "OK   [self-test/vacuidade-script-ausente]: gate reprova (rc=$status) com o script real ausente, nomeando a causa"
  fi

  # --- Cenário C3: guarda de vacuidade — SÍTIO NOVO NÃO CONTABILIZADO -------
  # Árvore sintética com o script real de verdade (copiado de $ROOT_DIR, para
  # passar as guardas 1/2) MAIS um arquivo extra com o marcador
  # "permissionDecisionReason" num caminho que NÃO está em EXECUTED_HERE nem
  # em COVERED_BY_BYTE_IDENTITY_TEST — simula um 8º emissor real (ex.: um 4º
  # gerador, uma segunda cópia do script). Sem esta guarda, a derivação da
  # lista de sítios seria decorativa: listaria o sítio novo, a guarda de
  # CONTAGEM (que só cresce) passaria, e a EXECUÇÃO abaixo continuaria fixa
  # nos 4 alvos de hoje — o sítio novo ficaria listado e nunca verificado.
  local unaccounted_root="$WORK/self-test/unaccounted-site-root"
  mkdir -p "$unaccounted_root/scripts" "$unaccounted_root/internal/generators" "$unaccounted_root/npm/src/generators" "$unaccounted_root/pypi/trackfw/generators"
  cp "$ROOT_DIR/scripts/trackfw-git-branch-guard.sh" "$unaccounted_root/scripts/trackfw-git-branch-guard.sh"
  printf 'permissionDecisionReason\n' >"$unaccounted_root/internal/generators/scaffold.go"
  printf 'permissionDecisionReason\n' >"$unaccounted_root/npm/src/generators/hooks.js"
  printf 'permissionDecisionReason\n' >"$unaccounted_root/pypi/trackfw/generators/init_gen.py"
  printf 'permissionDecisionReason\n' >"$unaccounted_root/internal/generators/second_copy_not_wired_here.go"
  set +e
  out=$(TRACKFW_ROOT_DIR="$unaccounted_root" bash "$ROOT_DIR/scripts/check-git-branch-guard-hook-schema.sh" 2>&1)
  status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    echo "FAIL [self-test/vacuidade-sitio-nao-contabilizado]: sítio novo sem cobertura deveria reprovar, gate saiu 0" >&2
    failures=$((failures + 1))
  elif ! grep -qF 'second_copy_not_wired_here.go' <<<"$out"; then
    echo "FAIL [self-test/vacuidade-sitio-nao-contabilizado]: reprovou (rc=$status), mas sem nomear o sítio novo" >&2
    printf '%s\n' "$out" | sed 's/^/    /' >&2
    failures=$((failures + 1))
  else
    echo "OK   [self-test/vacuidade-sitio-nao-contabilizado]: gate reprova (rc=$status) um sítio derivado que não está executado nem coberto por teste de byte-identidade, nomeando-o"
  fi

  if [[ "$failures" -gt 0 ]]; then
    echo
    echo "check-git-branch-guard-hook-schema.sh --self-test: $failures cenário(s) FALHARAM." >&2
    exit 1
  fi
  echo
  echo "check-git-branch-guard-hook-schema.sh --self-test: os 3 cenários de falsificação passaram (schema errado reprova nomeando o sítio, schema certo aprova, guarda de vacuidade reprova nas 3 formas: nenhum sítio, script ausente, sítio novo não contabilizado)."
  exit 0
}

if [[ "${1:-}" == "--self-test" ]]; then
  self_test
fi

# ===========================================================================
# Modo padrão — verificação real, cabeada no `parity` via Makefile.
# ===========================================================================

# --- Guarda de vacuidade 1: script real precisa existir no disco -----------
REAL_SCRIPT="$ROOT_DIR/scripts/trackfw-git-branch-guard.sh"
if [[ ! -f "$REAL_SCRIPT" ]]; then
  echo "check-git-branch-guard-hook-schema: script real ausente em $REAL_SCRIPT — recusando aprovar em silêncio" >&2
  exit 1
fi

# --- Guarda de vacuidade 2: a lista de sítios derivada não pode ficar vazia,
# nem deixar de cobrir os 3 stacks -------------------------------------------
set +e
SITES=$(derive_sites "$ROOT_DIR")
set -e
SITE_COUNT=0
if [[ -n "$SITES" ]]; then
  SITE_COUNT=$(printf '%s\n' "$SITES" | grep -c . || true)
fi
if [[ "$SITE_COUNT" -eq 0 ]]; then
  echo "check-git-branch-guard-hook-schema: nenhum sítio encontrado com 'permissionDecisionReason' sob $ROOT_DIR — recusando aprovar em silêncio (vacuidade)" >&2
  exit 1
fi
if ! printf '%s\n' "$SITES" | grep -q '\.go$'; then
  echo "check-git-branch-guard-hook-schema: nenhum sítio Go (.go) encontrado — recusando aprovar em silêncio (vacuidade parcial)" >&2
  exit 1
fi
if ! printf '%s\n' "$SITES" | grep -q '\.js$'; then
  echo "check-git-branch-guard-hook-schema: nenhum sítio Node (.js) encontrado — recusando aprovar em silêncio (vacuidade parcial)" >&2
  exit 1
fi
if ! printf '%s\n' "$SITES" | grep -q '\.py$'; then
  echo "check-git-branch-guard-hook-schema: nenhum sítio Python (.py) encontrado — recusando aprovar em silêncio (vacuidade parcial)" >&2
  exit 1
fi
echo "check-git-branch-guard-hook-schema: $SITE_COUNT sítio(s) derivado(s) (permissionDecisionReason, 3 stacks confirmados):"
printf '%s\n' "$SITES" | sed 's/^/  /'
echo

# ---------------------------------------------------------------------------
# Guarda de vacuidade 3 — RECONCILIAÇÃO: todo sítio DERIVADO precisa estar
# contabilizado em uma das duas listas fechadas abaixo, ou o gate reprova
# nomeando o sobra. Sem isto, a derivação do passo anterior é decorativa —
# ela lista sítios novos, mas a EXECUÇÃO abaixo continua fixa nos 4 alvos de
# hoje (script real + 3 geradores), então um 8º sítio real (4º gerador, novo
# runtime, segunda cópia do script) apareceria listado e NUNCA verificado, e
# a guarda de contagem acima (que só cresce) não pegaria isso — exatamente o
# "lista congelada que envelhece em silêncio" que este gate existe para
# evitar (ver cabeçalho, "Lista de sítios: DERIVADA, não congelada").
#
# EXECUTED_HERE: emitem o JSON em runtime — verificados abaixo por execução
# real + decode estruturado.
# COVERED_BY_BYTE_IDENTITY_TEST: cópias de REFERÊNCIA do `validate` (nunca
# rodam como hook — só existem para checar integridade de um script já
# instalado); cada uma tem um teste unitário nomeado, no próprio stack, que
# prova byte-identidade contra o gerador real (ver comentário do cabeçalho
# deste arquivo para a lista completa e o motivo de não duplicar a execução
# aqui).
# ---------------------------------------------------------------------------
EXECUTED_HERE="scripts/trackfw-git-branch-guard.sh internal/generators/scaffold.go npm/src/generators/hooks.js pypi/trackfw/generators/init_gen.py"
COVERED_BY_BYTE_IDENTITY_TEST="internal/validator/validator_git_branch_guard_reference.go pypi/trackfw/validator.py npm/src/validator/index.js"

UNACCOUNTED=""
while IFS= read -r site; do
  [[ -n "$site" ]] || continue
  rel="${site#"$ROOT_DIR"/}"
  accounted=0
  for known in $EXECUTED_HERE $COVERED_BY_BYTE_IDENTITY_TEST; do
    if [[ "$rel" == "$known" ]]; then
      accounted=1
      break
    fi
  done
  if [[ "$accounted" -eq 0 ]]; then
    UNACCOUNTED="$UNACCOUNTED$rel"$'\n'
  fi
done <<<"$SITES"

if [[ -n "$UNACCOUNTED" ]]; then
  echo "check-git-branch-guard-hook-schema: sítio(s) derivado(s) SEM cobertura (nem executado por este gate, nem coberto por teste de byte-identidade nomeado) — acrescente à execução deste gate ou à lista COVERED_BY_BYTE_IDENTITY_TEST com o teste que o cobre:" >&2
  printf '%s' "$UNACCOUNTED" | sed 's/^/  /' >&2
  exit 1
fi
echo "check-git-branch-guard-hook-schema: reconciliação ok — todos os $SITE_COUNT sítio(s) derivado(s) estão contabilizados (execução direta ou teste de byte-identidade nomeado)."
echo

# ---------------------------------------------------------------------------
# Resolve os 3 runtimes — mesmo padrão de check-attention-scripts-parity.sh.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-git-branch-guard-hook-schema: binário Go não encontrado/executável em $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-git-branch-guard-hook-schema: CLI Node não encontrado em $NODE_CLI" >&2
  exit 1
fi

BLOCK_ARG="git commit -m teste-check-git-branch-guard-hook-schema"

# --- 1. Script real do repo (o que está de fato instalado/dogfooded) -------
check_site "git-branch-guard-hook-schema/script-real" "$ROOT_DIR" "scripts/trackfw-git-branch-guard.sh" "$BLOCK_ARG"

# --- 2/3/4. Os 3 geradores, via `discover --init` (entry point de produção) -
GO_DIR="$WORK/gen-go"
NODE_DIR="$WORK/gen-node"
PY_DIR="$WORK/gen-py"

set +e
run_discover_init go "$GO_DIR"
GO_DISCOVER_STATUS=$?
run_discover_init node "$NODE_DIR"
NODE_DISCOVER_STATUS=$?
run_discover_init py "$PY_DIR"
PY_DISCOVER_STATUS=$?
set -e

for pair in "go:$GO_DISCOVER_STATUS:$GO_DIR" "node:$NODE_DISCOVER_STATUS:$NODE_DIR" "py:$PY_DISCOVER_STATUS:$PY_DIR"; do
  runtime=${pair%%:*}
  rest=${pair#*:}
  discover_status=${rest%%:*}
  gen_dir=${rest#*:}
  if [[ "$discover_status" -ne 0 ]]; then
    fail "git-branch-guard-hook-schema/$runtime/discover-init" \
      "discover --init saiu $discover_status; stderr: $(cat "$WORK/$runtime.discover.err" 2>/dev/null)"
    continue
  fi
  gen_script="$gen_dir/scripts/trackfw-git-branch-guard.sh"
  if [[ ! -s "$gen_script" ]]; then
    fail "git-branch-guard-hook-schema/$runtime/discover-init" "script gerado ausente ou vazio: $gen_script"
    continue
  fi
  check_site "git-branch-guard-hook-schema/gerador-$runtime" "$gen_dir" "scripts/trackfw-git-branch-guard.sh" "$BLOCK_ARG"
done

echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-git-branch-guard-hook-schema.sh scenarios passed."
else
  echo "check-git-branch-guard-hook-schema.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
