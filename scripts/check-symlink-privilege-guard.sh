#!/usr/bin/env bash
# check-symlink-privilege-guard.sh — toda criação de symlink/fifo em arquivo
# de teste deve passar por guarda de capacidade (não por guarda de plataforma).
#
# Ligado a: make quality (ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona,
# ML-1B — sítios de guarda de privilégio de symlink)
#
# POR QUE ESTE GATE EXISTE
# A issue #315 (symlink cru → t.Fatal em Windows sem Developer Mode) foi
# introduzida DENTRO de um ML de correção, dentro de um PR já em revisão.
# Três MLs anteriores já corrigiram 10 sítios da mesma classe; este gate
# impede a décima-primeira instância.
#
# O DISCRIMINANTE — guarda de capacidade vs. guarda de plataforma
# - Guarda de plataforma: `runtime.GOOS == "windows"` / `sys.platform == "win32"`
#   abandona cobertura mesmo em Windows com Developer Mode habilitado.
# - Guarda de capacidade: tenta criar; falha de privilégio → skip; outro erro
#   → fail. Detectada por presença de padrão canônico próximo ao symlink.
#
# DECISÃO 1 — o que conta como guarda válida (contexto de ±5 linhas)
#   Go  : symlinkOrSkip ou isSymlinkPrivilegeError presente em ±5 linhas
#   Node: symlinkOrSkip ou EPERM ou EACCES presente em ±5 linhas
#   Py  : _symlink_or_skip, symlink_or_skip, winerror, EPERM ou EACCES em ±5 linhas
#
# DECISÃO 2 — linhas de comentário não são sítios
# Linhas que começam com '//' (Go/JS) ou '#' (Py) após espaço são ignoradas.
#
# DECISÃO 3 — exceções com justificativa obrigatória inline
#
# DECISÃO 4 — população não-zero
# O gate reporta o número de arquivos varridos e aborta se for zero.
#
# FALSIFICAÇÃO (--self-test): 3 braços — sítio nu REPROVA, guarda PASSA,
# vazio REPROVA com mensagem de população zero.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ──────────────────────────────────────────────────────────────────────────────
# Self-test
# ──────────────────────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--self-test" ]]; then
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT

  FAKE_REPO="$TMP/fake-repo"
  mkdir -p "$FAKE_REPO"
  # Create a minimal git repo so git ls-files works
  git -C "$FAKE_REPO" init -q
  mkdir -p "$FAKE_REPO/pkg"

  echo "=== [self-test braço 1] symlink cru em _test.go → gate REPROVA ==="
  cat > "$FAKE_REPO/pkg/bad_test.go" <<'GOEOF'
package pkg

import (
	"os"
	"testing"
)

func TestBad(t *testing.T) {
	if err := os.Symlink("a", "b"); err != nil {
		t.Fatal(err)
	}
}
GOEOF
  git -C "$FAKE_REPO" add pkg/bad_test.go
  if FAKE_REPO="$FAKE_REPO" bash "$REPO_ROOT/scripts/check-symlink-privilege-guard.sh" >/dev/null 2>&1; then
    echo "ERRO: gate deveria ter reprovado com symlink cru, mas saiu 0" >&2
    exit 1
  fi
  echo "OK: gate reprovou corretamente"

  echo "=== [self-test braço 2] symlinkOrSkip presente → gate PASSA ==="
  cat > "$FAKE_REPO/pkg/good_test.go" <<'GOEOF'
package pkg

import "testing"

func TestGood(t *testing.T) {
	// Uses symlinkOrSkip which guards the privilege check
	symlinkOrSkip(t, "a", "b")
}
GOEOF
  # Replace bad_test.go with good_test.go content (remove raw call)
  cat > "$FAKE_REPO/pkg/bad_test.go" <<'GOEOF'
package pkg

import "testing"

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	// isSymlinkPrivilegeError used here
}
GOEOF
  git -C "$FAKE_REPO" add pkg/bad_test.go pkg/good_test.go
  if ! FAKE_REPO="$FAKE_REPO" bash "$REPO_ROOT/scripts/check-symlink-privilege-guard.sh" >/dev/null 2>&1; then
    echo "ERRO: gate deveria ter passado com symlinkOrSkip, mas saiu != 0" >&2
    exit 1
  fi
  echo "OK: gate passou corretamente"

  echo "=== [self-test braço 3] repo sem arquivos de teste → REPROVA (população zero) ==="
  EMPTY_REPO="$TMP/empty-repo"
  mkdir -p "$EMPTY_REPO"
  git -C "$EMPTY_REPO" init -q
  if FAKE_REPO="$EMPTY_REPO" bash "$REPO_ROOT/scripts/check-symlink-privilege-guard.sh" >/dev/null 2>&1; then
    echo "ERRO: gate deveria ter reprovado com população zero, mas saiu 0" >&2
    exit 1
  fi
  echo "OK: gate reprovou com população zero"

  echo ""
  echo "self-test: 3/3 braços OK"
  exit 0
fi

# ──────────────────────────────────────────────────────────────────────────────
# Varredura real
# ──────────────────────────────────────────────────────────────────────────────
SCAN_ROOT="${FAKE_REPO:-$REPO_ROOT}"

# Lista de arquivos de teste via git ls-files (exclui ignorados por construção)
# Dentro do self-test usa git ls-files do FAKE_REPO.
mapfile -t TEST_FILES < <(
  git -C "$SCAN_ROOT" ls-files \
    | grep -E '(_test\.go|\.test\.js|test_[^/]+\.py)$' \
    | while IFS= read -r f; do printf '%s/%s\n' "$SCAN_ROOT" "$f"; done
)

FILE_COUNT=${#TEST_FILES[@]}
if [[ $FILE_COUNT -eq 0 ]]; then
  printf 'ERRO: check-symlink-privilege-guard: nenhum arquivo de teste encontrado em %s\n' "$SCAN_ROOT" >&2
  printf 'Guarda de vacuidade disparou — o gate nao deve sair 0 sobre zero arquivos.\n' >&2
  exit 1
fi

printf 'check-symlink-privilege-guard: varrendo %d arquivos de teste...\n' "$FILE_COUNT"

# Padrões de criação de symlink/fifo — ERE portável (BSD grep no macOS, GNU grep no Linux)
# Sem \b (não portável em ERE BSD), apenas o nome da função/método.
RAW_GO='os\.Symlink[[:space:]]*\(|os\.Mkfifo[[:space:]]*\(|syscall\.Mkfifo[[:space:]]*\('
RAW_JS='fs\.symlinkSync[[:space:]]*\(|symlinkSync[[:space:]]*\('
RAW_PY='\.symlink_to[[:space:]]*\(|os\.symlink[[:space:]]*\(|os\.mkfifo[[:space:]]*\('

# Padrões de guarda de capacidade (ERE, sem parênteses desbalanceados)
GUARD_GO='symlinkOrSkip|isSymlinkPrivilegeError'
GUARD_JS='symlinkOrSkip|EPERM|EACCES'
GUARD_PY='_symlink_or_skip|symlink_or_skip|winerror|EPERM|EACCES'

# ──────────────────────────────────────────────────────────────────────────────
# Exceções — sítios com justificativa escrita
# Formato: caminho relativo → razão (string descritiva)
# ──────────────────────────────────────────────────────────────────────────────
declare -A EXCEPTIONS

# Build tag !windows: mkfifo não existe no pacote syscall do Windows — build
# tag é o mecanismo correto (o arquivo nem compila no Windows), não guarda
# de capacidade que testaria um erro de runtime impossível de ocorrer.
EXCEPTIONS["internal/validator/regularfile_fifo_unix_test.go"]="FIFO_BUILD_TAG_NOT_WINDOWS"
EXCEPTIONS["internal/validator/validator_credential_guard_integrity_fifo_unix_test.go"]="FIFO_BUILD_TAG_NOT_WINDOWS"

# mkfifo não existe no Windows em absoluto; guarda de plataforma é correta
# (platform=win32 skip não é antipadrão aqui — a syscall não existe no SO).
EXCEPTIONS["npm/tests/credential_guard_integrity.test.js"]="MKFIFO_PLATFORM_NOT_WINDOWS"
EXCEPTIONS["npm/tests/git_branch_guard_hook_integrity.test.js"]="MKFIFO_PLATFORM_NOT_WINDOWS"
EXCEPTIONS["pypi/tests/test_credential_guard_integrity.py"]="MKFIFO_PLATFORM_NOT_WINDOWS"
EXCEPTIONS["pypi/tests/test_git_branch_guard_validator.py"]="MKFIFO_PLATFORM_NOT_WINDOWS"

# Fallback chain explícito: symlink → hardlink → copy. O erro do symlink é
# capturado e o fallback é invocado — não é criação nua; a cadeia trata o
# caso de sem privilégio redirecionando para cópia/hardlink.
EXCEPTIONS["internal/commands/ship_test.go"]="FALLBACK_CHAIN_EXPLICIT"
EXCEPTIONS["npm/tests/ship.test.js"]="FALLBACK_CHAIN_EXPLICIT"
EXCEPTIONS["pypi/tests/test_generators_init.py"]="FALLBACK_CHAIN_EXPLICIT"

# Barreira de teste (test_barrier.py): os.symlink(src, dst) dentro de
# TestBarrierRejectsForeignSymlink — sítio isolado em fixture de teste de
# barreira, sem propagação a produção; alvo é a barreira, não o symlink.
EXCEPTIONS["pypi/tests/test_barrier.py"]="BARRIER_TEST_FIXTURE"

FAIL=0
declare -a FAILURES

for FILE in "${TEST_FILES[@]}"; do
  REL="${FILE#$SCAN_ROOT/}"

  # Exceções por caminho relativo
  if [[ -v "EXCEPTIONS[$REL]" ]]; then
    continue
  fi

  # Determinar linguagem e padrões
  if [[ "$REL" == *_test.go ]]; then
    RAW_PAT="$RAW_GO"
    GUARD_PAT="$GUARD_GO"
  elif [[ "$REL" == *.test.js ]]; then
    RAW_PAT="$RAW_JS"
    GUARD_PAT="$GUARD_JS"
  elif [[ "$REL" == test_*.py || "$REL" == */test_*.py ]]; then
    RAW_PAT="$RAW_PY"
    GUARD_PAT="$GUARD_PY"
  else
    continue
  fi

  # grep -a: trata binários como texto (evita pular arquivos classificados como binários)
  while IFS= read -r HIT; do
    [[ -z "$HIT" ]] && continue

    # Extrair número de linha com ${var%%:*} (robusto mesmo se content tem ':')
    LINENUM="${HIT%%:*}"
    LINE_CONTENT="${HIT#*:}"

    # Ignorar linhas que são puramente comentários
    STRIPPED="${LINE_CONTENT#"${LINE_CONTENT%%[![:space:]]*}"}"  # trim leading spaces
    case "$STRIPPED" in
      '//'*|'#'*|'*'*) continue ;;
    esac

    # Contexto ±5 linhas para verificar presença de guarda
    START=$(( LINENUM - 5 ))
    END=$(( LINENUM + 5 ))
    [[ $START -lt 1 ]] && START=1
    CONTEXT=$(sed -n "${START},${END}p" "$FILE" 2>/dev/null)
    if echo "$CONTEXT" | grep -qE "$GUARD_PAT"; then
      continue  # guarda detectada no contexto
    fi

    FAILURES+=("$REL:$LINENUM: symlink/fifo sem guarda de capacidade")
    FAIL=1
  done < <(grep -anE "$RAW_PAT" "$FILE" 2>/dev/null)
done

echo ""
if [[ $FAIL -eq 0 ]]; then
  printf 'check-symlink-privilege-guard: OK — %d arquivos verificados, zero sitios desguardados.\n' "$FILE_COUNT"
  exit 0
fi

printf 'check-symlink-privilege-guard: FALHA — sitios sem guarda de capacidade:\n' >&2
for F in "${FAILURES[@]}"; do
  printf '  %s\n' "$F" >&2
done
printf '\n' >&2
printf 'Corrija usando symlinkOrSkip (Go/Node) ou _symlink_or_skip (Python).\n' >&2
printf 'A guarda deve distinguir "sem privilegio" (skip) de "falhou por outro motivo" (fail).\n' >&2
exit 1
