#!/usr/bin/env bash
# check-artifact-closed-cycle.sh — CICLO FECHADO gerador → verificador, por artefato.
#
# A pergunta que este gate responde é uma só: **o que o gerador escreve, o verificador enxerga?**
#
# Ele existe para impedir a QUARTA ocorrência do mesmo padrão de defeito neste repositório —
# *gerador e verificador discordando do contrato*, sem nenhum teste entre os dois:
#
#   1. Cabeçalho de aceite  — o gerador escrevia em português, o `barrier` casava só inglês.
#   2. Vocabulário de status — o gerador escrevia emoji, o verificador esperava `pending`.
#   3. Layout de REQ         — `req new` gravava flat, o validator procurava `<agente>/<estado>/`
#                              (REQ-2026-08-30: em `by_agent`, QUATRO dos seis layouts eram vácuos).
#
# A causa comum das três é a ausência de teste de ciclo fechado. Prova dura disso: no ML-1A da
# ROADMAP-2026-09-03, as três suítes passaram **sem edição** depois da correção — nenhum teste
# existente codificava o comportamento antigo. O defeito nunca foi testado em nenhuma direção.
#
# 🔴 ESTE GATE EXECUTA O CLI, NÃO O MÓDULO. Ciclo fechado com mock não é ciclo fechado: o defeito
# do `trackfw context` do CLI Node sobreviveu desde a origem exatamente porque o teste importava o
# módulo em vez de executar o binário — o defeito morava na FRONTEIRA, dentro de nenhum dos dois
# (vault/notes/promise-flutuante-em-action-do-cli-node-e-invisivel-na-fronteira-2026-09-02.md).
#
# Cobertura: 3 artefatos (req/adr/note) × 3 CLIs (Go/Node/Python) × 2 layouts (flat/by_agent) = 18
# combinações. Ressalva honesta: `vault/notes` é constante hardcoded no gerador de nota
# (`internal/generators/note.go:12` e equivalentes) — não tem dimensão de layout. As 2 execuções do
# braço de nota exercem o mesmo caminho de código; o eixo de layout ali é degenerado por construção,
# e está declarado aqui para não inflar a contagem.
#
# O que este gate NÃO duplica: `check-artifact-parity.sh` já roda os 3 CLIs gerando req/adr/note e
# compara byte-a-byte entre runtimes, e já tem ciclo E2E de `roadmap move` em flat/by_agent. O que
# ele nunca fez é **alimentar o verificador com a saída do gerador** — essa é a delta deste arquivo.
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

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
# Caminho absoluto — o Makefile pode passar `bin/trackfw` relativo, que ficaria inválido dentro do
# `cd "$dir"` das subshells de execução.
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-closed-cycle.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# $HOME isolado e sintético para o gate inteiro — nunca o real. Sem isto, todo `trackfw validate`
# daqui enxergaria o escopo GLOBAL de guards de quem roda o gate: desde
# ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-
# fiacao (ML-3A), credential_guard_script_integrity/git_branch_guard_script_integrity disparam pela
# EXISTÊNCIA do script em ~/.trackfw/scripts/. Um $HOME real com o harness instalado poluiria a
# saída de validate com warnings que não têm nada a ver com ciclo fechado de artefato. Mesmo
# precedente do Cenário 46 e de check-artifact-parity.sh.
export HOME="$WORK/home"
mkdir -p "$HOME"

# Título com acento e cedilha: mantém o slug sob normalização NFKD nos 3 runtimes dentro do escopo
# do que este gate exercita (o contrato de slug em si é de check-artifact-parity.sh).
TITLE="Ciclo Fechado de Governança"

FAIL=0

# ── Guarda de rollover de meia-noite ────────────────────────────────────────
# O nome dos artefatos leva a data. Se a data virar no meio da execução, os basenames capturados
# antes deixam de bater com os de depois — falha explícita em vez de diagnóstico misterioso.
DATE_BEFORE=$(date +%F)

run_trackfw() {
  local runtime=$1
  local dir=$2
  shift 2
  case "$runtime" in
    go)     (cd "$dir" && "$GO_BIN" "$@") ;;
    node)   (cd "$dir" && node "$ROOT_DIR/npm/bin/trackfw" "$@") ;;
    python) (cd "$dir" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw "$@") ;;
    *)      echo "check-artifact-closed-cycle: runtime desconhecido: $runtime" >&2; return 1 ;;
  esac
}

# `validate --json` do Go imprime o JSON e DEPOIS uma linha "N violation(s) found" quando há
# violação; Node e Python imprimem JSON multi-linha. `json.load` puro quebra no primeiro caso
# ("Extra data") — daí raw_decode a partir da primeira chave.
PARSER="$WORK/parse-validate.py"
cat >"$PARSER" <<'PYEOF'
"""Lê `trackfw validate --json` do stdin e responde a UMA pergunta de ciclo fechado.

Uso: parse-validate.py <kind> <rule> <needle> [extra-needle]

  kind = names   -> exit 0 se EXISTE entrada da regra citando <needle> em file/message
  kind = absent  -> exit 0 se NAO existe entrada da regra citando <needle>

`needle` é sempre o BASENAME EXATO do arquivo que o gerador acabou de escrever. É essa a métrica
que discrimina — ver o comentário longo no gate.
"""
import json
import sys

kind, rule, needle = sys.argv[1], sys.argv[2], sys.argv[3]
extra = sys.argv[4] if len(sys.argv) > 4 else None

raw = sys.stdin.read()
start = raw.find("{")
if start < 0:
    print("saida de validate --json nao contem JSON:\n" + raw, file=sys.stderr)
    sys.exit(2)
try:
    payload, _ = json.JSONDecoder().raw_decode(raw[start:])
except ValueError as exc:
    print("JSON invalido de validate --json (%s):\n%s" % (exc, raw), file=sys.stderr)
    sys.exit(2)

entries = list(payload.get("violations") or []) + list(payload.get("warnings") or [])
hits = []
for entry in entries:
    if entry.get("rule") != rule:
        continue
    haystack = "%s %s" % (entry.get("file") or "", entry.get("message") or "")
    if needle not in haystack:
        continue
    if extra is not None and extra not in haystack:
        continue
    hits.append(entry)

if kind == "names":
    if hits:
        sys.exit(0)
    print("regra %r nao produziu nenhuma entrada citando %r%s" % (
        rule, needle, "" if extra is None else " + %r" % extra), file=sys.stderr)
    print("entradas vistas: %s" % json.dumps(entries, ensure_ascii=False), file=sys.stderr)
    sys.exit(1)
if kind == "absent":
    if not hits:
        sys.exit(0)
    print("regra %r citou %r e nao deveria: %s" % (
        rule, needle, json.dumps(hits, ensure_ascii=False)), file=sys.stderr)
    sys.exit(1)
print("kind desconhecido: %s" % kind, file=sys.stderr)
sys.exit(2)
PYEOF

# ── Métrica: por que "a violação NOMEIA o basename gerado" e não "a regra apareceu" ──────────────
#
# A métrica óbvia — "a regra disparou na saída" — MENTE. Medido no ML-1A: sobre a árvore SABOTADA
# (caso canônico removido do resolvedor), 6 de 6 regras ainda apareciam na saída, porque
# `ref_targets_exist` e `traceid` disparam pelo lado do ROADMAP, sem ler REQ nenhuma. A métrica
# fraca daria verde com o defeito presente.
#
# A métrica deste gate é: **a entrada de `validate --json` cita, em `file` ou `message`, o basename
# EXATO do arquivo que o gerador acabou de escrever.** Ela discrimina porque:
#
#   (a) o basename carrega data + slug do título — nenhum outro artefato do projeto o contém, então
#       nenhuma outra regra pode satisfazê-la por acidente;
#   (b) só há um caminho para esse basename chegar à saída: o verificador ter RESOLVIDO o arquivo no
#       caminho onde o gerador o gravou. Se o leitor procurar noutro layout, a regra continua
#       existindo, continua rodando, e simplesmente não cita nada — que é exatamente a forma que o
#       defeito tinha em `by_agent`;
#   (c) `file` OU `message`, porque no Go `blocked_by_draft_adr` sai com `file: ""` e nomeia a REQ só
#       no texto — casar só em `file` perderia regra legítima.
assert_validate() {
  local label=$1 kind=$2 rule=$3 needle=$4 runtime=$5 dir=$6
  local extra=${7:-}
  local out status
  set +e
  out=$(run_trackfw "$runtime" "$dir" validate --json 2>&1)
  set -e
  set +e
  if [[ -n "$extra" ]]; then
    printf '%s' "$out" | python3 "$PARSER" "$kind" "$rule" "$needle" "$extra"
  else
    printf '%s' "$out" | python3 "$PARSER" "$kind" "$rule" "$needle"
  fi
  status=$?
  set -e
  if [[ $status -ne 0 ]]; then
    echo "closed cycle broken: $label — o verificador nao enxerga o que o gerador escreveu" >&2
    FAIL=1
    return 1
  fi
  echo "OK   [closed-cycle/$label]"
  return 0
}

write_project() {
  local dir=$1 layout=$2 runtime=$3
  rm -rf "$dir"
  mkdir -p "$dir"
  run_trackfw "$runtime" "$dir" init >/dev/null 2>&1

  if [[ "$layout" == "by_agent" ]]; then
    # `req new` em by_agent grava no canônico da ADR-2026-09-03: req_dir/<primeiro agente>/*.md.
    python3 - "$dir/trackfw.yaml" <<'PYEOF'
import re
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as fh:
    text = fh.read()
if re.search(r"^roadmap_namespacing:", text, re.M):
    text = re.sub(r"^roadmap_namespacing:.*$", "roadmap_namespacing: by_agent",
                  text, count=1, flags=re.M)
else:
    text += "\nroadmap_namespacing: by_agent\n"
if not re.search(r"^agents:", text, re.M):
    text += "\nagents:\n  - zeus\n"
with open(path, "w", encoding="utf-8") as fh:
    fh.write(text)
PYEOF
  fi
}

# ── Ciclo fechado de UMA combinação runtime × layout ────────────────────────────────────────────
closed_cycle() {
  local runtime=$1 layout=$2
  local dir="$WORK/$runtime-$layout"
  local tag="$runtime/$layout"

  write_project "$dir" "$layout" "$runtime"

  run_trackfw "$runtime" "$dir" req  new "$TITLE" >/dev/null
  run_trackfw "$runtime" "$dir" adr  new "$TITLE" >/dev/null
  run_trackfw "$runtime" "$dir" note new "$TITLE" >/dev/null

  # ── Guarda de vacuidade: sem artefato gerado, todas as asserções abaixo seriam sobre o vazio ──
  local req_rel adr_rel note_rel req_base adr_base note_base
  req_rel=$(cd "$dir" && find docs/req -type f -name 'REQ-*.md' | sort | head -1)
  adr_rel=$(cd "$dir" && find docs/adr -type f -name 'ADR-*ciclo-fechado*.md' | sort | head -1)
  note_rel=$(cd "$dir" && find vault/notes -type f -name '*ciclo-fechado*.md' | sort | head -1)
  if [[ -z "$req_rel" || -z "$adr_rel" || -z "$note_rel" ]]; then
    echo "closed cycle vacuity guard: $tag — gerador nao produziu artefato (req=$req_rel adr=$adr_rel note=$note_rel)" >&2
    FAIL=1
    return 0
  fi
  req_base=$(basename "$req_rel")
  adr_base=$(basename "$adr_rel")
  note_base=$(basename "$note_rel")

  # O layout canônico é parte do contrato: em by_agent o gerador tem de gravar em
  # req_dir/<agente>/*.md (ADR-2026-09-03 D2). Se gravar flat, o ciclo pode fechar por acidente
  # (o leitor lê a união) e a regressão de escrita passaria batido — daí a asserção de CAMINHO.
  if [[ "$layout" == "by_agent" && "$req_rel" != "docs/req/zeus/"* ]]; then
    echo "closed cycle write-path drift: $tag — req new gravou em '$req_rel', esperado docs/req/zeus/ (canonico D2)" >&2
    FAIL=1
  fi
  if [[ "$layout" == "flat" && "$req_rel" != "docs/req/REQ-"* ]]; then
    echo "closed cycle write-path drift: $tag — req new gravou em '$req_rel', esperado docs/req/ plano" >&2
    FAIL=1
  fi

  # ── (1) REQ: o verificador leu a REQ que o gerador escreveu ────────────────────────────────
  # `req new` emite o template sem marcador de ADR no corpo, então `req_has_adr` DEVE citá-la.
  # Se o resolvedor não achar o arquivo, a regra roda e não cita ninguém — a forma exata do defeito.
  assert_validate "req/$tag/req_has_adr-names-generated" \
    names req_has_adr "$req_base" "$runtime" "$dir" || true

  # ── (2) ADR: o verificador enumerou o ADR que o gerador escreveu ───────────────────────────
  # `adr new` grava um ADR que nenhuma REQ referencia — `adr_orphan` DEVE citá-lo pelo basename.
  assert_validate "adr/$tag/adr_orphan-names-generated" \
    names adr_orphan "$adr_base" "$runtime" "$dir" || true

  # ── (3) NOTE: o link que o gerador escreveu no index é o link que o verificador reconhece ──
  # `note new` cria a nota E a linka no index.md. `note_orphan` NÃO pode citá-la. Sozinha esta
  # asserção seria fraca (passa também se a regra estiver morta) — a asserção (4) é o antídoto.
  assert_validate "note/$tag/note_orphan-silent-for-indexed" \
    absent note_orphan "$note_base" "$runtime" "$dir" || true

  # ── (4) NOTE, liveness: a regra do (3) está viva e olha ESTE diretório ─────────────────────
  # Nota escrita à mão no mesmo diretório, ausente do index: `note_orphan` DEVE citá-la. Sem este
  # braço, (3) passaria com a regra desligada, com o diretório errado, ou com o vault inexistente.
  local orphan_base="nota-orfa-de-liveness-$DATE_BEFORE.md"
  printf -- '---\ntitle: "liveness"\n---\n\n# liveness\n' >"$dir/vault/notes/$orphan_base"
  assert_validate "note/$tag/note_orphan-fires-for-unindexed" \
    names note_orphan "$orphan_base" "$runtime" "$dir" || true
  rm -f "$dir/vault/notes/$orphan_base"

  # ── (5) ADR, vocabulário de status: o literal que o gerador escreveu é o que o verificador lê ─
  # Esta é a asserção que reproduz o padrão de defeito nº2 (gerador escreve um vocabulário, o
  # verificador espera outro). Ligamos a REQ gerada ao ADR gerado e marcamos a REQ como Done:
  # `adr_accepted_when_req_done` tem de citar o ADR **e o literal de status que `adr new` gravou**
  # (`Proposed`). Se o gerador passasse a emitir `status: Rascunho`, ou trocasse a chave
  # `status:` por outra, esta asserção cai — e é justamente a classe de regressão que hoje ninguém
  # pegaria.
  python3 - "$dir/$req_rel" "$adr_rel" <<'PYEOF'
import re
import sys

req_path, adr_rel = sys.argv[1], sys.argv[2]
with open(req_path, encoding="utf-8") as fh:
    text = fh.read()
text = re.sub(r"^status:.*$", "status: Done", text, count=1, flags=re.M)
if re.search(r"^adr:", text, re.M):
    text = re.sub(r"^adr:.*$", 'adr: "%s"' % adr_rel, text, count=1, flags=re.M)
else:
    text = text.replace("---\n", '---\nadr: "%s"\n' % adr_rel, 1)
with open(req_path, "w", encoding="utf-8") as fh:
    fh.write(text)
PYEOF

  assert_validate "adr/$tag/status-literal-read-back" \
    names adr_accepted_when_req_done "$adr_base" "$runtime" "$dir" "status: Proposed" || true

  # ── (6) ADR, o link passou a ser reconhecido ───────────────────────────────────────────────
  # Depois de (5), a REQ referencia o ADR — `adr_orphan` NÃO pode mais citá-lo. Junto com (2),
  # isso é um par: (2) prova que o ADR é enumerado, (6) prova que a referência é entendida. Nenhum
  # dos dois isolado prova as duas coisas.
  assert_validate "adr/$tag/adr_orphan-clears-after-link" \
    absent adr_orphan "$adr_base" "$runtime" "$dir" || true
}

for RUNTIME in go node python; do
  for LAYOUT in flat by_agent; do
    closed_cycle "$RUNTIME" "$LAYOUT"
  done
done

DATE_AFTER=$(date +%F)
if [[ "$DATE_BEFORE" != "$DATE_AFTER" ]]; then
  echo "check-artifact-closed-cycle: a data virou durante a execucao ($DATE_BEFORE -> $DATE_AFTER) — nomes de artefato inconsistentes, resultado descartado" >&2
  exit 1
fi

if [[ "$FAIL" -ne 0 ]]; then
  echo "check-artifact-closed-cycle: FALHOU — ha artefato que o gerador escreve e o verificador nao enxerga" >&2
  exit 1
fi

echo "Closed-cycle checks passed (3 artefatos x 3 CLIs x 2 layouts = 18 combinacoes; 6 assercoes por combinacao)"
