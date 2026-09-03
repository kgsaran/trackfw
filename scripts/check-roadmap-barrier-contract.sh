#!/usr/bin/env bash
# check-roadmap-barrier-contract.sh — gate falsificável do contrato gerador↔`barrier`
# (ML-3A, ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-do-barrier.md;
# REQ-2026-08-28-barrier-so-reconhece-cabecalho-de-aceite-em-portugues-mas-os-3-geradores-de-
# roadmap-escrevem-em-ingles.md, AC6/AC10/AC12).
#
# Três partes, nesta ordem:
#   A. AC12 — ciclo fechado com CLI REAL nos 3 runtimes: roadmap new -> preencher SÓ pelo que o
#      template ensina -> roadmap move wip -> barrier --wave 1 --trust-local-gates. Nenhuma
#      chamada de função interna — foi assim que o ML-2G da REQ anterior escapou da auditoria
#      (provou o mecanismo, não o efeito).
#   B. AC10 — não reclassificação do corpus. Congela a tabela de vereditos (mls_complete +
#      acceptance_evidence, por ML, por wave, por roadmap) do corpus REAL em
#      docs/roadmaps/**, fixado em um SNAPSHOT versionado (scripts/testdata/roadmap-barrier-
#      corpus-snapshot/, ver "Por que snapshot versionado" abaixo), como um hash SHA-256
#      pinado. Uma mudança futura no parser que reclassifique qualquer ML desse corpus muda o
#      hash e reprova o gate.
#   C. Falsificação — cenários com assert_fails_with mirando a razão que o PRÓPRIO `barrier`
#      emite (pinada em docs/cli-parity.md), cobrindo os alvos do ML-0A/ML-3A: cabeçalho
#      bilíngue, vocabulário de status por token, `⬜ Pendente ✅`, ML fantasma dentro de cerca,
#      marcador indentado (cross-runtime), sombreamento de status e de evidência por cerca de
#      código, e regressão do template (legenda ausente, `pending` de volta).
#
# Segue as convenções de scripts/check-ci-workflow-pin-parity.sh: set -euo pipefail, mktemp -d
# com trap de limpeza, "OK [cenário]"/"FAIL [cenário]: motivo", guarda de vacuidade no fim, e o
# padrão "mutar uma CÓPIA do conteúdo real e chamar a MESMA função de checagem" para as
# falsificações do template (a mesma função valida o conteúdo real E rejeita a mutação).
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-roadmap-barrier-contract.XXXXXX")
# Resolve para o caminho FÍSICO (pwd -P), não o simbólico. $TMPDIR no macOS é
# /var/folders/... -> /private/var/folders/... (symlink). Sem isto, o `roadmapTrustForGates`
# do barrier (internal/commands/barrier.go) calcula `git rev-parse --show-toplevel` (que git
# sempre resolve para o caminho físico) contra um `roadmapPath` absolutizado a partir de um
# cwd NÃO resolvido — o `filepath.Rel` resultante fica errado, a leitura de blob que o próprio
# barrier faz contra `origin/main:<relpath errado>` (via git, internamente) falha com uma
# mensagem que NÃO bate em nenhum dos dois padrões esperados
# ("does not exist in" / "exists on disk, but not in"), e o trust-check cai no ramo
# fail-open: `gates` deixa de reportar not_evaluated e EXECUTA de verdade os comandos que os
# 144 roadmaps históricos declaram (inclusive `make quality`, ~25 min, e potencialmente
# qualquer shell arbitrário) — achado ao vivo escrevendo este ML, documentado em
# vault/notes/barrier-trust-check-fail-open-em-tmpdir-simbolico-2026-08-29.md. Este `pwd -P`
# é a mitigação NESTE gate; o defeito em si é pré-existente em barrier.go (não introduzido
# pelas Waves 1/2 deste roadmap) e fica fora do escopo deste ML — reportado, não corrigido.
WORK=$(cd "$WORK" && pwd -P)
# chmod -u+w antes do rm -rf: o build Go abaixo (quando GO_BIN não é passado) pode deixar o
# módulo cache com arquivos read-only por baixo de $WORK se GOPATH cair lá — mesmo padrão de
# scripts/check-barrier.sh.
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve os 3 runtimes — mesmo padrão de scripts/check-barrier.sh. O build Go roda ANTES de
# isolar $HOME: usar o $HOME real aqui deixa o module cache (`~/go/pkg/mod`) intacto entre
# execuções (evita redownload) e evita que o build grave arquivos read-only dentro de $WORK,
# que o trap de limpeza acima teria que desfazer.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

# $HOME isolado e sintético — nunca o real (mesmo precedente de check-barrier.sh e
# check-artifact-parity.sh: o gate "validate" embutido no barrier, e `trackfw validate` direto,
# enxergariam o escopo GLOBAL de guards do usuário rodando o gate). Isolado só a partir daqui —
# depois do build Go acima.
export HOME="$WORK/home"
mkdir -p "$HOME"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-roadmap-barrier-contract: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-roadmap-barrier-contract: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
SCENARIOS_RUN=0
ok()   { echo "OK   [$1]"; SCENARIOS_RUN=$((SCENARIOS_RUN + 1)); }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; SCENARIOS_RUN=$((SCENARIOS_RUN + 1)); }

# run_cli RUNTIME DIR ARGS... — invoca o CLI real do runtime pedido a partir de DIR.
# Comandos reais (bin/trackfw, node npm/bin/trackfw, PYTHONPATH=pypi python3 -m trackfw) — nunca
# chamada de função interna. Seta CLI_EXIT/CLI_STDOUT/CLI_STDERR.
run_cli() {
  local runtime=$1 dir=$2
  shift 2
  local out_file="$WORK/out.$$.$RANDOM" err_file="$WORK/err.$$.$RANDOM"
  set +e
  case "$runtime" in
  go) (cd "$dir" && "$GO_BIN" "$@") >"$out_file" 2>"$err_file" ;;
  node) (cd "$dir" && node "$NODE_CLI" "$@") >"$out_file" 2>"$err_file" ;;
  py) (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw "$@") >"$out_file" 2>"$err_file" ;;
  *) echo "run_cli: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  CLI_EXIT=$?
  set -e
  CLI_STDOUT=$(cat "$out_file")
  CLI_STDERR=$(cat "$err_file")
  rm -f "$out_file" "$err_file"
}

# doc_check_json DOC NAME FIELD — imprime o valor (JSON) de um campo de um check nomeado, ou
# 'null' se o check não existir. Mesmo padrão de check-barrier.sh (check_field_json).
doc_check_json() {
  local doc=$1 name=$2 field=$3
  python3 -c "
import json, sys
d = json.loads(sys.argv[1])
name, field = sys.argv[2], sys.argv[3]
for c in d['checks']:
    if c['name'] == name:
        print(json.dumps(c.get(field)))
        raise SystemExit(0)
print('null')
" "$doc" "$name" "$field"
}

# assert_check_status LABEL DOC CHECK_NAME EXPECTED — confere checks[CHECK_NAME].status.
assert_check_status() {
  local label=$1 doc=$2 name=$3 expected=$4
  local status
  status=$(doc_check_json "$doc" "$name" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  if [[ "$status" != "$expected" ]]; then
    fail "$label" "check '$name' status='$status', esperado '$expected'; doc=$doc"
    return 1
  fi
  ok "$label"
  return 0
}

# assert_check_reason_contains LABEL DOC CHECK_NAME FIELD(evidence|failures) PATTERN — confere
# que algum item do array contém PATTERN (a razão que o PRÓPRIO barrier emite, pinada em
# docs/cli-parity.md).
assert_check_reason_contains() {
  local label=$1 doc=$2 name=$3 field=$4 pattern=$5
  local arr
  arr=$(doc_check_json "$doc" "$name" "$field")
  if ! python3 -c "
import json, sys
items = json.loads(sys.argv[1])
pattern = sys.argv[2]
raise SystemExit(0 if any(pattern in x for x in (items or [])) else 1)
" "$arr" "$pattern"; then
    fail "$label" "'$pattern' não encontrado em checks[$name].$field; doc=$doc"
    return 1
  fi
  ok "$label"
  return 0
}

# assert_check_json_equals LABEL DOC CHECK_NAME FIELD EXPECTED_JSON — igualdade exata (usado
# para provar ausência de ML fantasma: evidence == ["ML-1A: ✅"], nada mais).
assert_check_json_equals() {
  local label=$1 doc=$2 name=$3 field=$4 expected=$5
  local arr
  arr=$(doc_check_json "$doc" "$name" "$field")
  if ! python3 -c "
import json, sys
got = json.loads(sys.argv[1])
want = json.loads(sys.argv[2])
raise SystemExit(0 if got == want else 1)
" "$arr" "$expected"; then
    fail "$label" "checks[$name].$field=$arr, esperado $expected; doc=$doc"
    return 1
  fi
  ok "$label"
  return 0
}

# json_field_equals GOT_JSON WANT_JSON — igualdade estrutural (não byte-a-byte: json.dumps
# escapa não-ASCII como \uXXXX, então comparar strings cruas faz falso-negativo em valores
# com acento/emoji). Usado pelos laços cross-runtime que comparam o mesmo campo nos 3 CLIs
# sem passar por assert_check_json_equals (que já faz ok/fail por conta própria).
json_field_equals() {
  python3 -c "
import json, sys
got = json.loads(sys.argv[1])
want = json.loads(sys.argv[2])
raise SystemExit(0 if got == want else 1)
" "$1" "$2"
}

# ═══════════════════════════════════════════════════════════════════════════
# PARTE A (AC12) — ciclo fechado com CLI real, nos 3 runtimes.
#   roadmap new -> preencher SÓ pelo que o template ensina -> roadmap move wip ->
#   barrier --wave 1 --trust-local-gates --json -> mls_complete e acceptance_evidence
#   PASSED. "validate: blocked" é esperado (sonda sem REQ vinculada) e NÃO é exigido passed —
#   só os dois checks que este ML corrige.
# ═══════════════════════════════════════════════════════════════════════════

CYCLE_GO_ROADMAP=""   # capturado para a Parte C (falsificação de regressão do template)

run_closed_cycle() {
  local runtime=$1
  local dir="$WORK/cycle-$runtime"
  mkdir -p "$dir"

  run_cli "$runtime" "$dir" roadmap new "Sonda ML-3A $runtime"
  if [[ "$CLI_EXIT" -ne 0 ]]; then
    fail "closed-cycle/$runtime/roadmap-new" "exit=$CLI_EXIT stderr=$CLI_STDERR"
    return
  fi
  local file
  file=$(find "$dir/docs/roadmaps/backlog" -name '*.md' | head -1)
  if [[ -z "$file" ]]; then
    fail "closed-cycle/$runtime/roadmap-new" "nenhum roadmap criado em $dir/docs/roadmaps/backlog"
    return
  fi
  local name
  name=$(basename "$file" .md)

  # Preencher SEGUINDO APENAS o que o template ensina: a legenda diz que ✅ Concluído é o
  # estado terminal (ML-2A), e o marcador de item concluído é "- [x]". Âncora por linha
  # inteira (^\*\*Status:\*\* ⬜ Pendente$) para não tocar a linha da legenda em si (que cita
  # os quatro estados na mesma linha, sem o prefixo "**Status:**").
  sed -i.bak 's/^\*\*Status:\*\* ⬜ Pendente$/**Status:** ✅ Concluído/' "$file"
  sed -i.bak 's/^- \[ \] /- [x] /' "$file"
  rm -f "$file.bak"

  run_cli "$runtime" "$dir" roadmap move "$name" wip
  if [[ "$CLI_EXIT" -ne 0 ]]; then
    fail "closed-cycle/$runtime/roadmap-move" "exit=$CLI_EXIT stderr=$CLI_STDERR"
    return
  fi

  if [[ "$runtime" == "go" ]]; then
    # Captura o caminho PÓS-move (wip/) — a Parte C usa este arquivo para as falsificações
    # de regressão do template. O caminho em backlog/ acima de já não existe depois do move.
    CYCLE_GO_ROADMAP="$dir/docs/roadmaps/wip/$name.md"
  fi

  run_cli "$runtime" "$dir" barrier "$name" --wave 1 --trust-local-gates --json
  # Não exigimos CLI_EXIT==0: "validate: blocked" (sonda sem REQ vinculada) faz o exit ser 1
  # mesmo com os dois checks do contrato PASSED — isso é esperado e não é o achado deste ML.
  local doc="$CLI_STDOUT"
  if [[ -z "$doc" ]]; then
    fail "closed-cycle/$runtime/barrier" "sem saída JSON; exit=$CLI_EXIT stderr=$CLI_STDERR"
    return
  fi
  assert_check_status "closed-cycle/$runtime/mls-complete-passed" "$doc" "mls_complete" "passed"
  assert_check_status "closed-cycle/$runtime/acceptance-evidence-passed" "$doc" "acceptance_evidence" "passed"
}

run_closed_cycle go
run_closed_cycle node
run_closed_cycle py

# ═══════════════════════════════════════════════════════════════════════════
# PARTE A (continuação) — regressão do TEMPLATE, provada mutando uma CÓPIA do conteúdo
# REAL gerado pelo ciclo Go acima e chamando a MESMA função de checagem (padrão de
# check-ci-workflow-pin-parity.sh). Cobre "template deixa de trazer a legenda" e
# "template volta a escrever pending".
# ═══════════════════════════════════════════════════════════════════════════

# check_legend_present FILE — a legenda dos 4 estados precisa existir, byte-idêntica
# (AC11/ML-2A). Emite a razão em stdout e retorna != 0 quando ausente.
check_legend_present() {
  local file=$1
  if ! grep -qF '⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado' "$file"; then
    echo "legenda dos 4 estados ausente do roadmap gerado"
    return 1
  fi
  return 0
}

# check_status_not_pending FILE — o template não pode voltar a escrever a forma antiga
# "**Status:** pending", que o barrier não reconhece como conclusão nem como nenhum dos 4
# estados do vocabulário novo.
check_status_not_pending() {
  local file=$1
  if grep -qE '^\*\*Status:\*\* pending$' "$file"; then
    echo "template regrediu: linha '**Status:** pending' encontrada (forma pré-ADR-2026-08-29)"
    return 1
  fi
  return 0
}

if [[ -z "$CYCLE_GO_ROADMAP" || ! -f "$CYCLE_GO_ROADMAP" ]]; then
  fail "template-regression/setup" "roadmap Go do ciclo fechado indisponível para as falsificações do template"
else
  # Validação real — o conteúdo REAL gerado pelo `roadmap new` (antes da edição manual do
  # ciclo, capturada abaixo em RAW_TEMPLATE) tem que passar nas duas checagens hoje.
  RAW_TEMPLATE="$WORK/raw-template.md"
  cp "$CYCLE_GO_ROADMAP" "$RAW_TEMPLATE"
  # Desfaz a edição do ciclo A (✅ Concluído / [x]) para inspecionar o template TAL COMO O
  # GERADOR ESCREVE, não como o ciclo o deixou depois de preenchido.
  sed -i.bak 's/^\*\*Status:\*\* ✅ Concluído$/**Status:** ⬜ Pendente/' "$RAW_TEMPLATE"
  sed -i.bak 's/^- \[x\] /- [ ] /' "$RAW_TEMPLATE"
  rm -f "$RAW_TEMPLATE.bak"

  if out=$(check_legend_present "$RAW_TEMPLATE" 2>&1); then
    ok "template/legend-present"
  else
    fail "template/legend-present" "$out"
  fi
  if out=$(check_status_not_pending "$RAW_TEMPLATE" 2>&1); then
    ok "template/status-not-pending"
  else
    fail "template/status-not-pending" "$out"
  fi

  # Falsificação 1 — template deixa de trazer a legenda: mutar CÓPIA removendo a linha,
  # chamar a MESMA função, exigir que ela reprove com a razão nomeada.
  NO_LEGEND="$WORK/no-legend.md"
  cp "$RAW_TEMPLATE" "$NO_LEGEND"
  sed -i.bak '/⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado/d' "$NO_LEGEND"
  rm -f "$NO_LEGEND.bak"
  if out=$(check_legend_present "$NO_LEGEND" 2>&1); then
    fail "falsify/template/legend-missing" "esperava reprovação (legenda ausente), mas passou silenciosamente"
  elif ! grep -qF "legenda dos 4 estados ausente" <<<"$out"; then
    fail "falsify/template/legend-missing" "reprovou mas sem a razão esperada; saída: $out"
  else
    ok "falsify/template/legend-missing"
  fi

  # Falsificação 2 — template volta a escrever "pending": mutar CÓPIA reintroduzindo a forma
  # antiga, chamar a MESMA função, exigir reprovação com a razão nomeada.
  BACK_TO_PENDING="$WORK/back-to-pending.md"
  cp "$RAW_TEMPLATE" "$BACK_TO_PENDING"
  sed -i.bak 's/^\*\*Status:\*\* ⬜ Pendente$/**Status:** pending/' "$BACK_TO_PENDING"
  rm -f "$BACK_TO_PENDING.bak"
  if out=$(check_status_not_pending "$BACK_TO_PENDING" 2>&1); then
    fail "falsify/template/status-pending-regression" "esperava reprovação ('pending' reintroduzido), mas passou silenciosamente"
  elif ! grep -qF "template regrediu" <<<"$out"; then
    fail "falsify/template/status-pending-regression" "reprovou mas sem a razão esperada; saída: $out"
  else
    ok "falsify/template/status-pending-regression"
  fi
fi

# ═══════════════════════════════════════════════════════════════════════════
# PARTE B (AC10) — não reclassificação do corpus. Congela a tabela de vereditos do corpus
# REAL de docs/roadmaps/** num SNAPSHOT VERSIONADO (não o working tree, não história do git
# — ver "Por que snapshot versionado" abaixo).
#
# Por que snapshot versionado, não o working tree: o corpus cresce a cada roadmap novo
# mesclado — hashar o working tree ao vivo faria este gate reprovar em TODO commit futuro
# que adicionasse ou completasse um roadmap, o que não é o que AC10 pede ("nenhum ML HOJE
# reconhecido pode deixar de ser").
#
# Por que snapshot versionado, não história do git (ML-3G, corretivo — a abordagem anterior
# lia blobs históricos via `git`, pinados no commit "feat(roadmap): template escreve a forma
# canonica de status e ensina a legenda (ML-2A)" desta mesma branch, sha curto a4e8f35): o job
# `parity` do CI reprovava com "fatal: Not a valid object name a4e8f35" por dois motivos
# independentes. (a) `actions/checkout@v7` usa `fetch-depth: 1` — nenhum SHA histórico é
# alcançável no clone raso do CI. (b) a4e8f35 é commit DESTA branch, confirmado não-ancestral
# de origin/main; o projeto faz squash-merge, então o SHA vira órfão no merge e o gate
# quebraria na main permanentemente — `fetch-depth: 0` não corrige, só adia a falha por horas
# (até o squash-merge). A correção: os MESMOS 144 arquivos, byte-a-byte idênticos ao que a
# leitura histórica produzia (extraídos uma única vez, no momento de autoria deste ML, e
# commitados como scripts/testdata/roadmap-barrier-corpus-snapshot/<basename>.md), viram o
# "congelamento" — sem nenhuma dependência de história do git em tempo de execução do gate.
# Chaveado por basename (não por caminho completo) porque um roadmap muda de pasta o tempo
# todo (backlog -> wip -> done, operação diária); snapshot por caminho reprovaria a cada
# transição de estado, que não é o que AC10 protege.
#
# Política de basename: um basename do snapshot AUSENTE do disco (docs/roadmaps/**) reprova —
# indica que um arquivo do corpus congelado foi apagado/renomeado sem atualizar o snapshot.
# Um roadmap NOVO (basename ausente do snapshot) é ignorado — preserva a imunidade ao
# crescimento do corpus, que era a intenção original do congelamento e continua certa.
# Colisão de basename: hoje não há nenhuma no corpus real (`find docs/roadmaps -name '*.md'
# | xargs -n1 basename | sort | uniq -d` vazio, medido em 2026-08-29) — mas o mecanismo NÃO
# detecta colisão silenciosamente: dois arquivos-fonte com o mesmo basename sobrescreveriam um
# ao outro na extração do snapshot (um `set -e` sobre contagem de origem vs. contagem escrita,
# abaixo, é o detector). Basename, não caminho, é uma escolha que aceita esse risco em troca de
# não reprovar a cada `roadmap move` — julgado correto porque o corpus é de 144 arquivos com
# nomes descritivos únicos, não uma fonte de dados adversarial.
#
# gates de cada wave copiada NÃO são executados: o roadmap copiado nunca existe em
# origin/main do sandbox (bare repo vazio), então o trust-check do barrier falha FECHADO
# (não fail-open) e reporta "gates: not_evaluated" — determinístico e seguro mesmo que
# algum dos 144 roadmaps históricos declare um gate perigoso ou lento.
# ═══════════════════════════════════════════════════════════════════════════

# Re-pinado em ML-3D (hades-tf security review, 2026-08-29, achado #1 / vault/notes/
# barrier-fence-closing-trailing-content-bypass-2026-08-29.md), único ponto do corpus inteiro
# de 144 arquivos/432 waves que muda de veredito com a correção de fechamento de cerca —
# investigado linha a linha antes de re-pinar, não apenas aceito porque "o script mandou":
# `docs/roadmaps/done/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-
# arquiteto-ensina-trackfw-push.md`, seção "Auditoria do ML-1A e do ML-2A" (linhas ~223-236 no
# snapshot): um bloco ``` (3 crases, sem info string) abre em "$ trackfw roadmap new..." e
# aninha um "```bash...```" de MESMO comprimento sem escalar para 4+ crases — o próprio defeito
# de nesting que o ML-0A/AC10 deste roadmap identifica e corrige no exemplo do template
# (comentário "Fence externo... alargado de 3 para 4 crases", poucas linhas acima no mesmo
# arquivo) mas NÃO corrigiu neste bloco de auditoria irmão, que cita o mesmo padrão. Sob o
# parser ANTIGO (achado #1, bug), o "```bash" fechava cedo o bloco externo por acidente,
# reabria e refechava logo depois — por coincidência de paridade par/ímpar, o resto do
# documento (Waves 2-bis/3/4) voltava a parsear normalmente. Sob o parser CORRIGIDO, o
# "```bash" não fecha mais (tem sufixo) e a cerca externa só fecha no próximo "```" bare — que
# deixa a contagem de abertura/fechamento REALMENTE desbalanceada dali até o fim do arquivo
# (nenhum "```" bare subsequente reequilibra), então as Waves 2-bis/3/4 caem dentro de uma
# cerca nunca fechada e `mls_complete`/`acceptance_evidence` corretamente reportam "no ML
# found"/vazio em vez de ler ML-1B/ML-3A/ML-1C como conteúdo real. Isto é uma correção, não uma
# regressão: um renderer CommonMark real trataria o mesmo texto exatamente assim. O arquivo é
# histórico (`done/`, já mesclado antes desta REQ existir) — não é reaberto nem editado por
# este ML. Confirmado por comparação binária dos dois parsers (pré/pós-fix) sobre os 144
# arquivos do snapshot: SOMENTE estas 6 linhas (3 evidence + 3 failure em mls_complete; 1
# evidence + 2 failures em acceptance_evidence) mudam; as outras 143 roadmaps são bit-a-bit
# idênticos.
#
# CORPUS_SNAPSHOT_DIR: extraído uma única vez (ML-3G, 2026-08-29) do commit que era referenciado
# como FREEZE_REF="a4e8f35" nesta mesma branch — leitura histórica feita FORA deste gate, no
# momento de autoria, e commitada como bytes versionados. O gate, a partir daqui, nunca mais
# consulta história do git.
#
# Re-pinado em ML-3H (2026-08-29, corretivo): as três ordenações que alimentam este pin (a
# listagem de basenames do snapshot, os labels de wave por arquivo, e as linhas de veredito
# que produzem CORPUS_HASH) rodavam sob o `sort` do locale ATIVO do processo — determinístico
# na máquina de quem gerou o pin, mas não entre ambientes: `LC_ALL=C sort` intercala dígitos/
# símbolos antes de letras e ignora maiúsculas/minúsculas por posição de byte puro, enquanto
# `en_US.UTF-8` intercala por regra de colação linguística (ex.: "ROADMAP-" ordena antes de
# "npm-"/"pypi-" em C, depois em en_US). O CI (runner Linux) roda sob `C`/`POSIX`; macOS de
# desenvolvimento comumente roda `en_US.UTF-8` — o pin anterior carregava a ordem de quem o
# gerou, não uma ordem fixa. Todas as três ordenações (linhas 463, 484, 523) passam a fixar
# `LC_ALL=C` no próprio comando (convenção já usada em check-integration-assets.sh,
# check-static-assets.sh e check-identity-parity.sh — não um `export LC_ALL=C` global no topo
# do script, que afetaria também o locale herdado pelos 3 subprocessos CLI invocados por
# run_cli, incluindo eventual saída textual i18n não coberta por este gate). O CONTEÚDO da
# tabela de vereditos (quais linhas existem) não mudou — só a ORDEM em que aparecem no arquivo
# hasheado; PINNED_CORPUS_FILES/WAVES/EXIT2 e as quatro contagens de evidence/failure abaixo
# permanecem idênticas ao ML-3G. Só PINNED_CORPUS_HASH muda.
CORPUS_SNAPSHOT_DIR="$ROOT_DIR/scripts/testdata/roadmap-barrier-corpus-snapshot"
CORPUS_VERDICTS_PIN="$ROOT_DIR/scripts/testdata/roadmap-barrier-corpus-verdicts.tsv"
PINNED_CORPUS_HASH="4fe2e7a4d0b6bf51a25515dec1d45671b84cf9d2b0c722cc0f35192bf59ca311"
PINNED_CORPUS_FILES=144
PINNED_CORPUS_WAVES=432
PINNED_CORPUS_EXIT2=14
PINNED_CORPUS_LINES=1500
PINNED_MLS_COMPLETE_EVIDENCE=639
PINNED_MLS_COMPLETE_FAILURE=113
PINNED_ACCEPTANCE_EVIDENCE_EVIDENCE=314
PINNED_ACCEPTANCE_EVIDENCE_FAILURE=434

if [[ -z "${HASH_CMD_BIN:-}" ]]; then
  if command -v sha256sum >/dev/null 2>&1; then
    HASH_CMD_BIN=(sha256sum)
  else
    HASH_CMD_BIN=(shasum -a 256)
  fi
fi

CORPUS_SANDBOX="$WORK/corpus-sandbox"
CORPUS_ORIGIN="$WORK/corpus-origin.git"
mkdir -p "$CORPUS_SANDBOX" "$CORPUS_ORIGIN"
git init -q --bare "$CORPUS_ORIGIN"
(
  cd "$CORPUS_SANDBOX"
  git init -q
  git config user.email "corpus-scan@trackfw.local"
  git config user.name "corpus-scan"
  mkdir -p docs/roadmaps/wip docs/roadmaps/done docs/roadmaps/backlog \
    docs/roadmaps/blocked docs/roadmaps/abandoned docs/roadmaps/analyzing docs/req docs/adr
  echo "corpus-scan sandbox — never contains any real roadmap on origin/main by design" >README.md
  git add README.md
  git commit -q -m "corpus-scan sandbox init"
  git branch -M main
  git remote add origin "$CORPUS_ORIGIN"
  git push -q origin main
)

CORPUS_LINES_FILE="$WORK/corpus-lines.txt"
>"$CORPUS_LINES_FILE"
CORPUS_FILES=0
CORPUS_WAVES=0
CORPUS_EXIT2=0

if [[ ! -d "$CORPUS_SNAPSHOT_DIR" ]]; then
  fail "corpus/non-vacuous" "$CORPUS_SNAPSHOT_DIR não existe — guarda de vacuidade do corpus"
else
  CORPUS_FILELIST=$(find "$CORPUS_SNAPSHOT_DIR" -maxdepth 1 -type f -name '*.md' | LC_ALL=C sort)
fi
if [[ -d "$CORPUS_SNAPSHOT_DIR" && -z "${CORPUS_FILELIST:-}" ]]; then
  fail "corpus/non-vacuous" "$CORPUS_SNAPSHOT_DIR não contém nenhum .md — guarda de vacuidade do corpus"
elif [[ -n "${CORPUS_FILELIST:-}" ]]; then
  MISSING_FROM_DISK=""
  while IFS= read -r snapshot_path; do
    base=$(basename "$snapshot_path")
    # Basename ausente do disco (docs/roadmaps/**) reprova: o corpus congelado referencia um
    # arquivo que já não existe na árvore de trabalho em NENHUM estado (wip/done/backlog/...).
    on_disk=$(find "$ROOT_DIR/docs/roadmaps" -type f -name "$base" | head -n1)
    if [[ -z "$on_disk" ]]; then
      MISSING_FROM_DISK="${MISSING_FROM_DISK}${MISSING_FROM_DISK:+, }$base"
      # NAO pula o arquivo. O veredito e computado sobre os BYTES DO SNAPSHOT — o disco
      # so prova existencia, como o comentario logo abaixo diz. Pular aqui removia o
      # arquivo do corpus CONGELADO, e ai as contagens e o hash da AC10 mudavam por
      # truncamento em vez de por reclassificacao. Medido: apagando UM roadmap do disco,
      # o gate emitia 6 falhas — a legitima (basename-missing-from-disk) mais CINCO
      # falsas, entre elas "corpus reclassificado: hash da tabela de vereditos mudou",
      # quando nada tinha sido reclassificado. A tripwire de disco e a AC10 sao
      # contratos distintos e agora reprovam em separado.
    fi
    CORPUS_FILES=$((CORPUS_FILES + 1))
    # Conteúdo vem do SNAPSHOT (bytes congelados), nunca do disco — o disco só prova
    # existência acima, o veredito é sempre computado sobre o conteúdo pinado.
    cp "$snapshot_path" "$CORPUS_SANDBOX/docs/roadmaps/wip/$base"
    name="${base%.md}"
    labels=$( (grep -oE '^## Wave [^ ]+ ' "$CORPUS_SANDBOX/docs/roadmaps/wip/$base" || true) \
      | sed -E 's/^## Wave ([^ ]+) $/\1/' | LC_ALL=C sort -u)
    for label in $labels; do
      CORPUS_WAVES=$((CORPUS_WAVES + 1))
      set +e
      out=$(cd "$CORPUS_SANDBOX" && "$GO_BIN" barrier "$name" --wave "$label" --json 2>/dev/null)
      rc=$?
      set -e
      if [[ "$rc" -eq 2 ]]; then
        # Cabeçalho de wave malformado pré-grammar (ADR-2026-08-22), não relacionado ao
        # contrato deste ML — pinado abaixo em PINNED_CORPUS_EXIT2 para que uma mudança
        # futura que faça esses 14 casos resolverem silenciosamente (ou que introduza
        # novos malformados) também mude a contagem e seja notada.
        CORPUS_EXIT2=$((CORPUS_EXIT2 + 1))
        continue
      fi
      python3 - "$out" "$base" "$label" >>"$CORPUS_LINES_FILE" <<'PYEOF'
import json, sys

# stdout em UTF-8 explicito. Este bloco imprime evidence/failures do barrier,
# que contem os tokens de status do roadmap (checkmark, quadrado branco). Num
# console cp1252 o print estoura com UnicodeEncodeError e o gate morre no 11o
# check. E mesmo sem estourar, a codificacao entraria no CORPUS_HASH (linha 542
# hasheia este arquivo), fazendo o mesmo corpus dar hash diferente por SO.
sys.stdout.reconfigure(encoding="utf-8", errors="replace", newline="\n")

doc = json.loads(sys.argv[1])
base, label = sys.argv[2], sys.argv[3]
for c in doc["checks"]:
    if c["name"] in ("mls_complete", "acceptance_evidence"):
        for e in c.get("evidence", []):
            print(f"{base}\t{label}\t{c['name']}\tevidence\t{e}")
        for e in c.get("failures", []):
            print(f"{base}\t{label}\t{c['name']}\tfailure\t{e}")
PYEOF
    done
    rm -f "$CORPUS_SANDBOX/docs/roadmaps/wip/$base"
  done <<<"$CORPUS_FILELIST"

  if [[ -n "$MISSING_FROM_DISK" ]]; then
    fail "corpus/basename-missing-from-disk" "basename(s) do snapshot ausente(s) de docs/roadmaps/**: $MISSING_FROM_DISK"
  else
    ok "corpus/basename-missing-from-disk"
  fi

  if [[ ! -s "$CORPUS_LINES_FILE" ]]; then
    fail "corpus/non-vacuous" "tabela de vereditos do corpus ficou vazia — guarda de vacuidade"
  else
    LC_ALL=C sort "$CORPUS_LINES_FILE" -o "$CORPUS_LINES_FILE"
    CORPUS_LINES=$(wc -l <"$CORPUS_LINES_FILE" | tr -d ' ')
    CORPUS_HASH=$("${HASH_CMD_BIN[@]}" "$CORPUS_LINES_FILE" | awk '{print $1}')

    MLS_EVIDENCE=$(awk -F'\t' '$3=="mls_complete" && $4=="evidence"' "$CORPUS_LINES_FILE" | wc -l | tr -d ' ')
    MLS_FAILURE=$(awk -F'\t' '$3=="mls_complete" && $4=="failure"' "$CORPUS_LINES_FILE" | wc -l | tr -d ' ')
    ACC_EVIDENCE=$(awk -F'\t' '$3=="acceptance_evidence" && $4=="evidence"' "$CORPUS_LINES_FILE" | wc -l | tr -d ' ')
    ACC_FAILURE=$(awk -F'\t' '$3=="acceptance_evidence" && $4=="failure"' "$CORPUS_LINES_FILE" | wc -l | tr -d ' ')

    echo "corpus (snapshot=$CORPUS_SNAPSHOT_DIR): files=$CORPUS_FILES waves=$CORPUS_WAVES exit2=$CORPUS_EXIT2 lines=$CORPUS_LINES"
    echo "  mls_complete: $MLS_EVIDENCE evidence / $MLS_FAILURE failure"
    echo "  acceptance_evidence: $ACC_EVIDENCE evidence / $ACC_FAILURE failure"
    echo "  hash: $CORPUS_HASH"

    if [[ "$CORPUS_FILES" -ne "$PINNED_CORPUS_FILES" ]]; then
      fail "corpus/files-count" "arquivos varridos no snapshot: $CORPUS_FILES, pinado $PINNED_CORPUS_FILES — snapshot mudou de conteúdo?"
    else
      ok "corpus/files-count"
    fi
    if [[ "$CORPUS_WAVES" -ne "$PINNED_CORPUS_WAVES" ]]; then
      fail "corpus/waves-count" "waves varridas: $CORPUS_WAVES, pinado $PINNED_CORPUS_WAVES"
    else
      ok "corpus/waves-count"
    fi
    if [[ "$CORPUS_EXIT2" -ne "$PINNED_CORPUS_EXIT2" ]]; then
      fail "corpus/exit2-count" "waves malformadas (exit 2): $CORPUS_EXIT2, pinado $PINNED_CORPUS_EXIT2"
    else
      ok "corpus/exit2-count"
    fi
    if [[ "$MLS_EVIDENCE" -ne "$PINNED_MLS_COMPLETE_EVIDENCE" || "$MLS_FAILURE" -ne "$PINNED_MLS_COMPLETE_FAILURE" ]]; then
      fail "corpus/mls-complete-verdict-counts" "evidence=$MLS_EVIDENCE failure=$MLS_FAILURE, pinado evidence=$PINNED_MLS_COMPLETE_EVIDENCE failure=$PINNED_MLS_COMPLETE_FAILURE"
    else
      ok "corpus/mls-complete-verdict-counts"
    fi
    if [[ "$ACC_EVIDENCE" -ne "$PINNED_ACCEPTANCE_EVIDENCE_EVIDENCE" || "$ACC_FAILURE" -ne "$PINNED_ACCEPTANCE_EVIDENCE_FAILURE" ]]; then
      fail "corpus/acceptance-evidence-verdict-counts" "evidence=$ACC_EVIDENCE failure=$ACC_FAILURE, pinado evidence=$PINNED_ACCEPTANCE_EVIDENCE_EVIDENCE failure=$PINNED_ACCEPTANCE_EVIDENCE_FAILURE"
    else
      ok "corpus/acceptance-evidence-verdict-counts"
    fi
    if [[ "$CORPUS_HASH" != "$PINNED_CORPUS_HASH" ]]; then
      # Nomeia QUAL entrada mudou: diff contra a tabela de vereditos versionada
      # (CORPUS_VERDICTS_PIN, mesmas 1500 linhas ordenadas que produzem PINNED_CORPUS_HASH).
      # O hash pinado no script protege a tabela de adulteração; a tabela em si dá o nome.
      DIFF_OUT=""
      if [[ -f "$CORPUS_VERDICTS_PIN" ]]; then
        DIFF_OUT=$(diff "$CORPUS_VERDICTS_PIN" "$CORPUS_LINES_FILE" || true)
      fi
      if [[ -n "$DIFF_OUT" ]]; then
        fail "corpus/non-reclassification" "corpus reclassificado: hash da tabela de vereditos mudou (pinado $PINNED_CORPUS_HASH, obtido $CORPUS_HASH). Linhas divergentes vs. $CORPUS_VERDICTS_PIN: $DIFF_OUT"
      else
        fail "corpus/non-reclassification" "corpus reclassificado: hash da tabela de vereditos mudou (pinado $PINNED_CORPUS_HASH, obtido $CORPUS_HASH), mas $CORPUS_VERDICTS_PIN está ausente ou idêntico — não foi possível nomear a linha divergente"
      fi
    else
      ok "corpus/non-reclassification"
    fi
  fi
fi

# ═══════════════════════════════════════════════════════════════════════════
# PARTE C — falsificação nas duas direções, mirando a razão que o PRÓPRIO barrier emite
# (pinada em docs/cli-parity.md). Fixtures mínimas escritas diretamente em wip/, mesmo
# padrão de scripts/check-barrier.sh.
# ═══════════════════════════════════════════════════════════════════════════

FALSIFY_DIR="$WORK/falsify"
mkdir -p "$FALSIFY_DIR/docs/roadmaps/wip"

write_fixture() {
  local name=$1
  cat >"$FALSIFY_DIR/docs/roadmaps/wip/$name.md"
}

# --- Cenário: cabeçalho EN aceito hoje; forma incorreta (não-canônica) continua rejeitada,
# provando que a checagem de header discrimina por texto exato, não por "qualquer coisa
# parecida" — o que seria o mesmo defeito que esta REQ corrigiu, só que na direção oposta.
write_fixture en-header-ok <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅
**Acceptance criteria:**
- [x] a
EOF
run_cli go "$FALSIFY_DIR" barrier en-header-ok --wave 1 --json
assert_check_status "barrier/en-header-accepted" "$CLI_STDOUT" "acceptance_evidence" "passed"

write_fixture en-header-wrong-spelling <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅
**Acceptance Criteria (EN):**
- [x] a
EOF
run_cli go "$FALSIFY_DIR" barrier en-header-wrong-spelling --wave 1 --json
assert_check_status "falsify/en-header-wrong-spelling/blocked" "$CLI_STDOUT" "acceptance_evidence" "blocked"
assert_check_reason_contains "falsify/en-header-wrong-spelling/reason" "$CLI_STDOUT" "acceptance_evidence" "failures" "no acceptance block"

# --- Cenário: cabeçalho PT aceito hoje (99/143 roadmaps do corpus). ---
write_fixture pt-header-ok <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅
**Critérios de aceite:**
- [x] a
EOF
run_cli go "$FALSIFY_DIR" barrier pt-header-ok --wave 1 --json
assert_check_status "barrier/pt-header-accepted" "$CLI_STDOUT" "acceptance_evidence" "passed"

# --- Cenário: **Status:** ⬜ Pendente ✅ (marca fora da posição inicial) NÃO é reconhecido
# como concluído (ADR decisão 8/AC14 — falso-positivo por substring, hoje corrigido por
# primeiro-token).
write_fixture trailing-checkmark <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ⬜ Pendente ✅
**Acceptance criteria:**
- [x] a
EOF
run_cli go "$FALSIFY_DIR" barrier trailing-checkmark --wave 1 --json
assert_check_status "falsify/trailing-checkmark/blocked" "$CLI_STDOUT" "mls_complete" "blocked"
assert_check_reason_contains "falsify/trailing-checkmark/reason" "$CLI_STDOUT" "mls_complete" "failures" "not complete (status: ⬜ Pendente ✅)"

# --- Cenário: ### ML-XX dentro de cerca de código não vira ML fantasma (AC13, ML-1B) —
# evidência exata (exact-match) prova que só o ML real aparece, nada mais, cobrindo os 3
# estilos de cerca fixados no ML-1B (3 crases, til, 4+ crases aninhadas).
write_fixture fence-phantom-backtick3 <<'EOF'
## Wave 1 — X
### ML-1A — real
**Status:** ✅
**Acceptance criteria:**
- [x] a

```
### ML-9Z — fake
**Status:** ✅
```
EOF
run_cli go "$FALSIFY_DIR" barrier fence-phantom-backtick3 --wave 1 --json
assert_check_json_equals "falsify/fence-phantom/backtick3-no-ghost" "$CLI_STDOUT" "mls_complete" "evidence" '["ML-1A: ✅"]'

write_fixture fence-phantom-tilde <<'EOF'
## Wave 1 — X
### ML-1A — real
**Status:** ✅
**Acceptance criteria:**
- [x] a

~~~
### ML-9Z — fake
**Status:** ✅
~~~
EOF
run_cli go "$FALSIFY_DIR" barrier fence-phantom-tilde --wave 1 --json
assert_check_json_equals "falsify/fence-phantom/tilde-no-ghost" "$CLI_STDOUT" "mls_complete" "evidence" '["ML-1A: ✅"]'

write_fixture fence-phantom-backtick4 <<'EOF'
## Wave 1 — X
### ML-1A — real
**Status:** ✅
**Acceptance criteria:**
- [x] a

````
```
### ML-9Z — fake
**Status:** ✅
```
````
EOF
run_cli go "$FALSIFY_DIR" barrier fence-phantom-backtick4 --wave 1 --json
assert_check_json_equals "falsify/fence-phantom/backtick4-nested-no-ghost" "$CLI_STDOUT" "mls_complete" "evidence" '["ML-1A: ✅"]'

# --- Cenário cross-runtime (mesmo achado do TestBarrierParity_TildeFenceEvasion que este ML
# remove de barrier_test.go): ML fantasma escondido numa cerca ~~~ não escapa em NENHUM dos
# 3 runtimes — o ML real fica bloqueado pelo motivo real (⬜ Pendente), e mls_complete.failures
# é EXATAMENTE esse motivo, nada do fantasma. Complementa fence-phantom-tilde acima (que só
# roda no Go e cobre o caso "passa"): este cobre o caso "bloqueia", que é onde um vazamento
# do fantasma mudaria o conteúdo de failures sem necessariamente mudar mls_complete.status.
write_fixture fence-phantom-tilde-blocked-cross-runtime <<'EOF'
## Wave 1 — X
### ML-1A — real
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] real unmet criterion

Example of a phantom ML hidden inside a tilde fence:
~~~
### ML-9Z — phantom
**Status:** done
**Acceptance criteria:**
- [x] fake
~~~
EOF
FENCE_TILDE_BLOCKED_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier fence-phantom-tilde-blocked-cross-runtime --wave 1 --json
  mls_status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  mls_failures=$(doc_check_json "$CLI_STDOUT" "mls_complete" "failures")
  if [[ "$mls_status" != "blocked" ]] || ! json_field_equals "$mls_failures" '["ML-1A: not complete (status: ⬜ Pendente)"]'; then
    FENCE_TILDE_BLOCKED_DIVERGENT="${FENCE_TILDE_BLOCKED_DIVERGENT}${rt}(status=$mls_status,failures=$mls_failures) "
  fi
done
if [[ -n "$FENCE_TILDE_BLOCKED_DIVERGENT" ]]; then
  fail "falsify/fence-phantom-tilde-blocked-cross-runtime" "runtime(s) vazaram o ML fantasma da cerca ~~~ em failures: $FENCE_TILDE_BLOCKED_DIVERGENT"
else
  ok "falsify/fence-phantom-tilde-blocked-cross-runtime"
fi

# --- Cenário cross-runtime (mesmo achado do TestBarrierParity_FourBacktickFenceEvasion que
# este ML remove de barrier_test.go): ML fantasma aninhado numa cerca de 3 crases dentro de
# uma cerca de 4+ crases não escapa em NENHUM dos 3 runtimes — mesma lógica do cenário acima,
# para o outro estilo de cerca fixado no ML-1B.
write_fixture fence-phantom-backtick4-blocked-cross-runtime <<'EOF'
## Wave 1 — X
### ML-1A — real
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] real unmet criterion

Example nesting a 3-backtick fence inside a 4-backtick fence:
````
outer fence, then a nested doc block:
```
### ML-9Z — nested phantom
**Status:** done
**Acceptance criteria:**
- [x] fake
```
still inside the outer fence
````
EOF
FENCE_B4_BLOCKED_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier fence-phantom-backtick4-blocked-cross-runtime --wave 1 --json
  mls_status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  mls_failures=$(doc_check_json "$CLI_STDOUT" "mls_complete" "failures")
  if [[ "$mls_status" != "blocked" ]] || ! json_field_equals "$mls_failures" '["ML-1A: not complete (status: ⬜ Pendente)"]'; then
    FENCE_B4_BLOCKED_DIVERGENT="${FENCE_B4_BLOCKED_DIVERGENT}${rt}(status=$mls_status,failures=$mls_failures) "
  fi
done
if [[ -n "$FENCE_B4_BLOCKED_DIVERGENT" ]]; then
  fail "falsify/fence-phantom-backtick4-blocked-cross-runtime" "runtime(s) vazaram o ML fantasma da cerca de 4 crases em failures: $FENCE_B4_BLOCKED_DIVERGENT"
else
  ok "falsify/fence-phantom-backtick4-blocked-cross-runtime"
fi

# --- Cenário: marcador indentado não conta como status em NENHUM dos 3 runtimes (ML-1B,
# achado 2: Node era o permissivo antes da correção). Cruza os 3 e nomeia qual diverge, se
# algum divergir.
write_fixture indented-status <<EOF
## Wave 1 — X
### ML-1A — x
  **Status:** ✅
**Acceptance criteria:**
- [x] a
EOF
INDENT_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier indented-status --wave 1 --json
  status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  if [[ "$status" != "blocked" ]]; then
    INDENT_DIVERGENT="${INDENT_DIVERGENT}${rt}(status=$status) "
  fi
done
if [[ -n "$INDENT_DIVERGENT" ]]; then
  fail "falsify/indented-marker-rejected-cross-runtime" "runtime(s) aceitaram marcador indentado como status válido: $INDENT_DIVERGENT"
else
  ok "falsify/indented-marker-rejected-cross-runtime"
fi

# --- Cenário (ML-0A/hades-tf): status forjado dentro de cerca ("**Status:** done") não
# vence sobre o status real fora da cerca ("**Status:** ⬜ Pendente") — reprovado, usando o
# status real.
write_fixture shadow-status <<'EOF'
## Wave 1 — X
### ML-1A — x
Example:
```
**Status:** done
```
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] a
EOF
run_cli go "$FALSIFY_DIR" barrier shadow-status --wave 1 --json
assert_check_status "falsify/shadow-status/uses-real-status-blocked" "$CLI_STDOUT" "mls_complete" "blocked"
assert_check_reason_contains "falsify/shadow-status/reason" "$CLI_STDOUT" "mls_complete" "failures" "not complete (status: ⬜ Pendente)"

# --- Cenário (ML-0A/hades-tf): critérios forjados dentro de cerca, sem bloco real de aceite
# fora dela — reprovado com "no acceptance block", não vira evidência.
write_fixture shadow-acceptance <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅
Example:
```
**Critérios de aceite:**
- [x] fake evidence, nothing built
```
EOF
run_cli go "$FALSIFY_DIR" barrier shadow-acceptance --wave 1 --json
assert_check_status "falsify/shadow-acceptance/no-real-block-blocked" "$CLI_STDOUT" "acceptance_evidence" "blocked"
assert_check_reason_contains "falsify/shadow-acceptance/reason" "$CLI_STDOUT" "acceptance_evidence" "failures" "no acceptance block"

# --- Cenário CRÍTICO (hades-tf, parecer de segurança 2026-08-29, achado #1 / vault/notes/
# barrier-fence-closing-trailing-content-bypass-2026-08-29.md): uma linha de "fechamento" com
# sufixo (` ```trailing-junk `) DENTRO de uma cerca já aberta não fecha a cerca no CommonMark
# real — permanece conteúdo interno — mas antes da correção os 3 CLIs a tratavam como
# fechamento válido, encerrando a máscara cedo e liberando um "**Status:** done" e um
# "- [x]" forjados como se fossem conteúdo real da ML. Reprodução mínima do parecer,
# verbatim (sonda-fence-full-bypass.md), testada nos 3 runtimes: deve bloquear usando o
# status/critério REAIS (fora do exemplo), nunca os forjados.
write_fixture fence-close-with-trailing-content-bypass <<'EOF'
## Wave 1 — X
### ML-1A — x
Prosa introduzindo um exemplo do defeito que documentamos:
```
notas de exemplo, sem relacao com o trabalho real
```trailing-junk-que-nao-fecha-a-cerca-no-commonmark-real
**Status:** done
**Acceptance criteria:**
- [x] evidencia forjada, nada foi feito
Mais texto ainda dentro do exemplo, por CommonMark de verdade:
```
Conteúdo real da ML, fora do exemplo:
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] criterio real nao atendido
EOF
FENCE_BYPASS_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier fence-close-with-trailing-content-bypass --wave 1 --json
  mls_status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  ev_status=$(doc_check_json "$CLI_STDOUT" "acceptance_evidence" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  if [[ "$mls_status" != "blocked" || "$ev_status" != "blocked" ]]; then
    FENCE_BYPASS_DIVERGENT="${FENCE_BYPASS_DIVERGENT}${rt}(mls_complete=$mls_status,acceptance_evidence=$ev_status) "
  fi
done
if [[ -n "$FENCE_BYPASS_DIVERGENT" ]]; then
  fail "falsify/fence-closing-line-with-trailing-content-does-not-close-cross-runtime" "runtime(s) liberaram a wave via fechamento forjado com sufixo: $FENCE_BYPASS_DIVERGENT"
else
  ok "falsify/fence-closing-line-with-trailing-content-does-not-close-cross-runtime"
fi
# Confirma, no Go (razão exata pinada em docs/cli-parity.md), que é o status/critério REAIS
# (fora do exemplo) que determinam o bloqueio — não um efeito colateral não relacionado.
run_cli go "$FALSIFY_DIR" barrier fence-close-with-trailing-content-bypass --wave 1 --json
assert_check_reason_contains "falsify/fence-closing-line-with-trailing-content/real-status-used" "$CLI_STDOUT" "mls_complete" "failures" "not complete (status: ⬜ Pendente)"
assert_check_reason_contains "falsify/fence-closing-line-with-trailing-content/real-criterion-used" "$CLI_STDOUT" "acceptance_evidence" "failures" "1 unmet acceptance criteria"

# --- Cenário de regressão (deve continuar ABRINDO): a linha de ABERTURA de uma cerca aceita
# conteúdo à direita do delimitador (info string CommonMark, ex.: ` ```bash `) — a correção do
# achado #1 muda só a regra de FECHAMENTO. Um bloco aberto com info string ainda mascara o
# "**Status:** done" forjado no seu interior.
write_fixture fence-open-with-info-string-still-opens <<'EOF'
## Wave 1 — X
### ML-1A — x
Example:
```bash
**Status:** done
```
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] a
EOF
run_cli go "$FALSIFY_DIR" barrier fence-open-with-info-string-still-opens --wave 1 --json
assert_check_status "falsify/fence-open-info-string-still-masks/blocked" "$CLI_STDOUT" "mls_complete" "blocked"
assert_check_reason_contains "falsify/fence-open-info-string-still-masks/uses-real-status" "$CLI_STDOUT" "mls_complete" "failures" "not complete (status: ⬜ Pendente)"

# --- Cenário de regressão (deve continuar FECHANDO): uma linha de fechamento com só espaço em
# branco após os caracteres da cerca (` ```   `, sem conteúdo real) ainda fecha normalmente —
# a correção do achado #1 exige "vazio OU só espaço em branco", não "vazio estrito".
python3 - "$FALSIFY_DIR/docs/roadmaps/wip/fence-close-with-trailing-whitespace-still-closes.md" <<'PYEOF'
import sys
path = sys.argv[1]
content = (
    "## Wave 1 — X\n"
    "### ML-1A — x\n"
    "Example:\n"
    "```\n"
    "**Status:** done\n"
    "```   \n"
    "**Status:** ⬜ Pendente\n"
    "**Acceptance criteria:**\n"
    "- [ ] a\n"
)
with open(path, "w", encoding="utf-8") as f:
    f.write(content)
PYEOF
run_cli go "$FALSIFY_DIR" barrier fence-close-with-trailing-whitespace-still-closes --wave 1 --json
assert_check_status "falsify/fence-close-trailing-whitespace-still-closes/blocked" "$CLI_STDOUT" "mls_complete" "blocked"
assert_check_reason_contains "falsify/fence-close-trailing-whitespace-still-closes/uses-real-status" "$CLI_STDOUT" "mls_complete" "failures" "not complete (status: ⬜ Pendente)"

# --- Cenário (hades-tf, achado #2): marca combinante Mn fora do bloco "Combining Diacritical
# Marks" (U+1DC0, COMBINING DOTTED GRAVE ACCENT, ccc 230) no primeiro token de status — os 3
# CLIs devem CONCORDAR (aceitar ou rejeitar juntos), nunca divergir (AC3/AC4). Antes da
# correção, Go/Python aceitavam ("done") e Node rejeitava — reproduzido ao vivo no parecer.
python3 - "$FALSIFY_DIR/docs/roadmaps/wip/combining-mark-out-of-range-u1dc0.md" <<'PYEOF'
import sys
path = sys.argv[1]
marker = "d" + "᷀" + "one"
content = (
    "## Wave 1 — X\n"
    "### ML-1A — x\n"
    f"**Status:** {marker}\n"
    "**Acceptance criteria:**\n"
    "- [x] a\n"
)
with open(path, "w", encoding="utf-8") as f:
    f.write(content)
PYEOF
COMBINING_VERDICTS=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier combining-mark-out-of-range-u1dc0 --wave 1 --json
  status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  COMBINING_VERDICTS="${COMBINING_VERDICTS}${rt}=${status} "
done
COMBINING_DIVERGENT=""
COMBINING_FIRST=""
for pair in $COMBINING_VERDICTS; do
  v="${pair#*=}"
  if [[ -z "$COMBINING_FIRST" ]]; then
    COMBINING_FIRST="$v"
  elif [[ "$v" != "$COMBINING_FIRST" ]]; then
    COMBINING_DIVERGENT="${COMBINING_DIVERGENT}${pair} "
  fi
done
if [[ -n "$COMBINING_DIVERGENT" ]]; then
  fail "falsify/combining-mark-u1dc0-cross-runtime-agreement" "runtimes divergiram: $COMBINING_VERDICTS"
else
  ok "falsify/combining-mark-u1dc0-cross-runtime-agreement"
fi
# Reforço pós ADR-2026-08-29 decisão 9 (AC15): a concordância acima não basta mais — o
# veredito CONCORDADO tem que ser especificamente "blocked". Antes da decisão 9 os 3
# CLIs concordavam em ACEITAR (dobrando Mn); a decisão inverteu a direção para REJEITAR.
if [[ "$COMBINING_FIRST" != "blocked" ]]; then
  fail "falsify/combining-mark-u1dc0-rejected-not-just-agreed" "veredito concordado foi '$COMBINING_FIRST', esperado 'blocked' (ADR decisão 9/AC15)"
else
  ok "falsify/combining-mark-u1dc0-rejected-not-just-agreed"
fi

# --- Cenário (AC15, exceção única): checkmark + VS16 (U+FE0F) — "✅️" — continua sendo
# reconhecido como concluído nos 3 CLIs. VS16 é a única marca de categoria Mn que a decisão 9
# ainda remove (não rejeita): é o seletor que teclados de emoji inserem depois de "✅",
# visualmente idêntico, sem valor semântico. Reproduz AC3/AC4 sob a regra nova.
write_fixture vs16-still-accepted <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅️
**Acceptance criteria:**
- [x] a
EOF
VS16_VERDICTS=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier vs16-still-accepted --wave 1 --json
  status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  VS16_VERDICTS="${VS16_VERDICTS}${rt}=${status} "
done
if [[ "$VS16_VERDICTS" != *"go=passed"* || "$VS16_VERDICTS" != *"node=passed"* || "$VS16_VERDICTS" != *"py=passed"* ]]; then
  fail "falsify/vs16-still-accepted-cross-runtime" "esperava 'passed' nos 3 CLIs (ADR decisão 9, exceção única); obtido: $VS16_VERDICTS"
else
  ok "falsify/vs16-still-accepted-cross-runtime"
fi

# --- Cenário (AC15, caso de ordem): "Concluído" acentuado (sem emoji) como único marcador de
# status continua sendo reconhecido como concluído nos 3 CLIs. É o caso que quebra se a
# checagem de Mn "sobrando" rodar DEPOIS da decomposição NFD em vez de ANTES: o "í" só vira
# combinante (U+0301) na forma decomposta — na forma autorada (NFC) não há Mn literal. Ver
# comentário de hasDisallowedCombiningMark/_has_disallowed_combining_mark nos 3 runtimes.
write_fixture accented-concluido-still-accepted <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** Concluído
**Acceptance criteria:**
- [x] a
EOF
ACCENTED_VERDICTS=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier accented-concluido-still-accepted --wave 1 --json
  status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  ACCENTED_VERDICTS="${ACCENTED_VERDICTS}${rt}=${status} "
done
if [[ "$ACCENTED_VERDICTS" != *"go=passed"* || "$ACCENTED_VERDICTS" != *"node=passed"* || "$ACCENTED_VERDICTS" != *"py=passed"* ]]; then
  fail "falsify/accented-concluido-still-accepted-cross-runtime" "esperava 'passed' nos 3 CLIs (ADR decisão 9, NFD deve rodar depois da checagem de Mn); obtido: $ACCENTED_VERDICTS"
else
  ok "falsify/accented-concluido-still-accepted-cross-runtime"
fi

# ═══════════════════════════════════════════════════════════════════════════
# ML-3F — cenários CRLF (issue #216) e casamento por prefixo do cabeçalho de gates
# (achado do apolo-tf no ML-1B). Movidos para cá a partir dos TestBarrierParity_* em
# internal/commands/barrier_test.go: aqueles testes faziam shell-out para node/python3 de
# dentro de `go test`, o que reprova o job "go" do CI (Go puro, sem os outros dois
# runtimes) — a paridade cross-CLI pertence a este gate, que já tem os 3 runtimes.
# ═══════════════════════════════════════════════════════════════════════════

# write_fixture_crlf NAME — como write_fixture, mas grava o conteúdo com terminador CRLF em
# TODAS as linhas (não só a última), simulando um roadmap editado no Windows (issue #216).
# O heredoc de entrada usa LF normal; a conversão para CRLF acontece na escrita.
write_fixture_crlf() {
  local name=$1
  python3 -c "
import sys
# stdin em BINARIO, decodificado como UTF-8 de forma explicita. sys.stdin.read()
# usa a codificacao do locale — cp1252 no Windows — e o heredoc chega em UTF-8.
# Medido: 'e2 ac 9c' (U+2B1C) entra, sai 'c3 a2 c2 ac c5 93' apos o .encode('utf-8')
# seguinte, porque a leitura ja tinha virado 3 caracteres. A fixture ia para o disco
# com DUPLA CODIFICACAO, e o CLI reportava fielmente o lixo — o defeito parecia do
# produto e era do gerador de fixture.
data = sys.stdin.buffer.read().decode('utf-8')
data = data.replace('\r\n', '\n').replace('\n', '\r\n')
with open(sys.argv[1], 'wb') as f:
    f.write(data.encode('utf-8'))
" "$FALSIFY_DIR/docs/roadmaps/wip/$name.md"
}

# --- Cenário CRLF: roadmap completo (ML concluído, critério atendido) passa nos 3 runtimes
# quando toda linha termina em CRLF (issue #216).
write_fixture_crlf crlf-full-roadmap-passes <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅
**Acceptance criteria:**
- [x] a
EOF
CRLF_FULL_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier crlf-full-roadmap-passes --wave 1 --json
  mls_status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  acc_status=$(doc_check_json "$CLI_STDOUT" "acceptance_evidence" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  if [[ "$mls_status" != "passed" || "$acc_status" != "passed" ]]; then
    CRLF_FULL_DIVERGENT="${CRLF_FULL_DIVERGENT}${rt}(mls_complete=$mls_status,acceptance_evidence=$acc_status) "
  fi
done
if [[ -n "$CRLF_FULL_DIVERGENT" ]]; then
  fail "crlf/full-roadmap-passes-cross-runtime" "runtime(s) não passaram um roadmap CRLF completo: $CRLF_FULL_DIVERGENT"
else
  ok "crlf/full-roadmap-passes-cross-runtime"
fi

# --- Cenário CRLF: ML pendente bloqueia nos 3 runtimes quando toda linha termina em CRLF.
write_fixture_crlf crlf-pending-ml-blocks <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] a
EOF
CRLF_PENDING_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier crlf-pending-ml-blocks --wave 1 --json
  mls_status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  if [[ "$mls_status" != "blocked" ]]; then
    CRLF_PENDING_DIVERGENT="${CRLF_PENDING_DIVERGENT}${rt}(mls_complete=$mls_status) "
  fi
done
if [[ -n "$CRLF_PENDING_DIVERGENT" ]]; then
  fail "crlf/pending-ml-blocks-cross-runtime" "runtime(s) não bloquearam um ML pendente em roadmap CRLF: $CRLF_PENDING_DIVERGENT"
else
  ok "crlf/pending-ml-blocks-cross-runtime"
fi
run_cli go "$FALSIFY_DIR" barrier crlf-pending-ml-blocks --wave 1 --json
assert_check_reason_contains "crlf/pending-ml-blocks-reason" "$CLI_STDOUT" "mls_complete" "failures" "not complete (status: ⬜ Pendente)"

# --- Cenário CRLF: a máscara de cerca (status forjado dentro de bloco de código não vence o
# status real fora dele — mesmo cenário de shadow-status acima) continua funcionando nos 3
# runtimes quando toda linha termina em CRLF.
write_fixture_crlf crlf-fence-mask-still-works <<'EOF'
## Wave 1 — X
### ML-1A — x
Example:
```
**Status:** done
```
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] a
EOF
CRLF_FENCE_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier crlf-fence-mask-still-works --wave 1 --json
  mls_status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  if [[ "$mls_status" != "blocked" ]]; then
    CRLF_FENCE_DIVERGENT="${CRLF_FENCE_DIVERGENT}${rt}(mls_complete=$mls_status) "
  fi
done
if [[ -n "$CRLF_FENCE_DIVERGENT" ]]; then
  fail "crlf/fence-mask-still-works-cross-runtime" "runtime(s) usaram o status forjado dentro da cerca em roadmap CRLF: $CRLF_FENCE_DIVERGENT"
else
  ok "crlf/fence-mask-still-works-cross-runtime"
fi
run_cli go "$FALSIFY_DIR" barrier crlf-fence-mask-still-works --wave 1 --json
assert_check_reason_contains "crlf/fence-mask-still-works-reason" "$CLI_STDOUT" "mls_complete" "failures" "not complete (status: ⬜ Pendente)"

# --- Cenário CRLF: marcador de status indentado (mesmo cenário de indented-status acima)
# continua NÃO reconhecido em NENHUM dos 3 runtimes quando toda linha termina em CRLF.
write_fixture_crlf crlf-indented-marker-still-rejected <<EOF
## Wave 1 — X
### ML-1A — x
  **Status:** ✅
**Acceptance criteria:**
- [x] a
EOF
CRLF_INDENT_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier crlf-indented-marker-still-rejected --wave 1 --json
  mls_status=$(doc_check_json "$CLI_STDOUT" "mls_complete" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  if [[ "$mls_status" != "blocked" ]]; then
    CRLF_INDENT_DIVERGENT="${CRLF_INDENT_DIVERGENT}${rt}(mls_complete=$mls_status) "
  fi
done
if [[ -n "$CRLF_INDENT_DIVERGENT" ]]; then
  fail "crlf/indented-marker-still-rejected-cross-runtime" "runtime(s) aceitaram marcador indentado como status válido em roadmap CRLF: $CRLF_INDENT_DIVERGENT"
else
  ok "crlf/indented-marker-still-rejected-cross-runtime"
fi

# --- Cenário CRLF: "**Gates da wave:**" é reconhecido e os comandos declarados
# EXECUTAM (não só "gates: passed" com zero comandos) nos 3 runtimes quando toda linha
# termina em CRLF. FALSIFY_DIR não é um repositório git → roadmapTrustForGates cai no ramo
# fail-open (trusted) em qualquer dos 3 runtimes, então os gates rodam sem
# --trust-local-gates — mesma premissa das fixtures de falsificação acima.
write_fixture_crlf crlf-gates-header-recognized <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅
**Acceptance criteria:**
- [x] a

**Gates da wave:**
```bash
true
```
EOF
CRLF_GATES_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier crlf-gates-header-recognized --wave 1 --json
  gates_status=$(doc_check_json "$CLI_STDOUT" "gates" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  gates_evidence=$(doc_check_json "$CLI_STDOUT" "gates" "evidence")
  if [[ "$gates_status" != "passed" || "$gates_evidence" != '["true: exit 0"]' ]]; then
    CRLF_GATES_DIVERGENT="${CRLF_GATES_DIVERGENT}${rt}(status=$gates_status,evidence=$gates_evidence) "
  fi
done
if [[ -n "$CRLF_GATES_DIVERGENT" ]]; then
  fail "crlf/gates-header-recognized-commands-run-cross-runtime" "runtime(s) não reconheceram '**Gates da wave:**' ou não executaram o comando declarado em roadmap CRLF: $CRLF_GATES_DIVERGENT"
else
  ok "crlf/gates-header-recognized-commands-run-cross-runtime"
fi

# --- Cenário (achado do apolo-tf no ML-1B): o cabeçalho "**Gates da wave:**" é casado por
# PREFIXO (regex ancorada só em ^, sem $) nos 3 runtimes — prosa na mesma linha
# ("**Gates da wave:** (obrigatórios)") continua reconhecendo o bloco e executando os
# comandos. Igualdade de linha inteira faria pelo menos um runtime ignorar o bloco em
# silêncio, reportando "gates: passed" com zero comandos executados — o bug que o
# apolo-tf autodescobriu.
write_fixture gates-header-prefix-match-with-trailing-prose <<'EOF'
## Wave 1 — X
### ML-1A — x
**Status:** ✅
**Acceptance criteria:**
- [x] a

**Gates da wave:** (obrigatórios)
```bash
true
```
EOF
GATES_PREFIX_DIVERGENT=""
for rt in go node py; do
  run_cli "$rt" "$FALSIFY_DIR" barrier gates-header-prefix-match-with-trailing-prose --wave 1 --json
  gates_status=$(doc_check_json "$CLI_STDOUT" "gates" "status" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")
  gates_evidence=$(doc_check_json "$CLI_STDOUT" "gates" "evidence")
  if [[ "$gates_status" != "passed" || "$gates_evidence" != '["true: exit 0"]' ]]; then
    GATES_PREFIX_DIVERGENT="${GATES_PREFIX_DIVERGENT}${rt}(status=$gates_status,evidence=$gates_evidence) "
  fi
done
if [[ -n "$GATES_PREFIX_DIVERGENT" ]]; then
  fail "falsify/gates-header-prefix-match-with-trailing-prose-cross-runtime" "runtime(s) ignoraram '**Gates da wave:** (obrigatórios)' em vez de casar por prefixo (gates: passed com zero comandos é o sintoma): $GATES_PREFIX_DIVERGENT"
else
  ok "falsify/gates-header-prefix-match-with-trailing-prose-cross-runtime"
fi

# ═══════════════════════════════════════════════════════════════════════════
# Guarda de vacuidade — obrigatória e provada empiricamente: um sub-shell roda a MESMA
# checagem com SCENARIOS_RUN=0 e confirma que ela reprova, antes de confiarmos nela para o
# run real acima.
# ═══════════════════════════════════════════════════════════════════════════
if (SCENARIOS_RUN=0; if [ "$SCENARIOS_RUN" -eq 0 ]; then exit 1; fi); then
  echo "FAIL [vacuity-guard/self-test]: guarda de vacuidade NÃO reprovou com SCENARIOS_RUN=0 — guarda quebrada" >&2
  FAIL=1
else
  ok "vacuity-guard/self-test"
fi

if [ "$SCENARIOS_RUN" -eq 0 ]; then
  echo "FAIL [check-roadmap-barrier-contract]: guarda de vacuidade — nenhum cenário rodou" >&2
  exit 1
fi

echo
if [ "$FAIL" -eq 0 ]; then
  echo "check-roadmap-barrier-contract: $SCENARIOS_RUN cenários OK"
else
  echo "check-roadmap-barrier-contract: um ou mais cenários FALHARAM ($SCENARIOS_RUN executados)" >&2
fi
exit "$FAIL"
