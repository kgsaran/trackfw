#!/usr/bin/env bash
# check-ci-workflow-job-id-collision.sh — reprova a reintroducao da colisao de job id
# entre os dois workflows de CI que o produto gera (ML-1A,
# ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md).
#
# O defeito original: buildGitHubActionsWorkflowContent (trackfw-gate.yml, escrito por
# init/update) e BuildDiscoverGitHubActionsWorkflowContent (trackfw-validate.yml,
# escrito por discover --init) declaravam o MESMO job id `governance` nos 3 CLIs. O
# GitHub casa check exigido por NOME — dois workflows com o mesmo job id produzem
# check-runs homonimos num PR, tornando `required_status_checks` ambiguo (satisfeito
# por qualquer um dos dois, imprevisivelmente). Confirmado ao vivo no PR #241:
# "governance=SUCCESS" x3 no mesmo push.
#
# Os dois workflows verificam a MESMA propriedade (`trackfw validate` passa) por dois
# mecanismos de instalacao diferentes (install.sh vs `go install`) — por isso os ids
# escolhidos (governance-install-script / governance-go-install) nomeiam o mecanismo,
# nao o arquivo de origem: quem le required_status_checks precisa distinguir os dois
# sem abrir o YAML.
#
# Seis pontos exigem checagem (2 workflows x 3 CLIs) — assert_count, nao assert_has,
# porque a assinatura de um workflow pode aparecer mais de uma vez por engano
# (ex.: alguem copia o job id do outro workflow para os dois blocos do mesmo arquivo)
# e um assert_has simples nao distingue "apareceu 1 vez, correto" de "apareceu 2 vezes,
# um deles e o id errado do outro workflow colado aqui".
set -euo pipefail

ROOT="${1:-.}"

fail=0
checked=0

# assert_count <label> <file> <exact-string> <expected-occurrences>
assert_count() {
  local label=$1 file=$2 needle=$3 expected_n=$4 got
  checked=$((checked + 1))
  if [[ ! -f "$ROOT/$file" ]]; then
    echo "check-ci-workflow-job-id-collision: $label — arquivo ausente: $file" >&2
    fail=1
    return
  fi
  got=$(grep -cF -- "$needle" "$ROOT/$file" || true)
  if [[ "$got" -ne "$expected_n" ]]; then
    echo "check-ci-workflow-job-id-collision: $label — esperava $expected_n ocorrencia(s), achou $got em $file" >&2
    echo "  esperado: $needle" >&2
    fail=1
  fi
}

GATE_ID="governance-install-script"
VALIDATE_ID="governance-go-install"
OLD_ID_LINE="  governance:"

# --- trackfw-gate.yml (buildGitHubActionsWorkflowContent) — 3 CLIs ---------------
assert_count "Go: trackfw-gate.yml usa $GATE_ID" \
  "internal/generators/scaffold.go" "$GATE_ID:" 1
assert_count "Node: trackfw-gate.yml usa $GATE_ID" \
  "npm/src/generators/init.js" "$GATE_ID:" 1
assert_count "Python: trackfw-gate.yml usa $GATE_ID" \
  "pypi/trackfw/generators/init_gen.py" "$GATE_ID:" 1

# --- trackfw-validate.yml (BuildDiscoverGitHubActionsWorkflowContent) — 3 CLIs ----
assert_count "Go: trackfw-validate.yml usa $VALIDATE_ID" \
  "internal/generators/scaffold_doctor.go" "$VALIDATE_ID:" 1
assert_count "Node: trackfw-validate.yml usa $VALIDATE_ID" \
  "npm/src/commands/discover.js" "$VALIDATE_ID:" 1
assert_count "Python: trackfw-validate.yml usa $VALIDATE_ID" \
  "pypi/trackfw/commands/discover.py" "$VALIDATE_ID:" 1

# --- Anti-regressao: o id antigo colidente (`  governance:`, com indentacao e
# dois-pontos) nao pode reaparecer no corpo do template escrito por nenhum dos 6
# geradores. Ancorado com os dois espacos de indentacao do YAML + dois-pontos para
# nao casar `governance_mode:` (internal/discover), `"id":"governance"` (catalog.json
# de skills) nem a prosa dos comentarios que este ML acrescentou (que citam o id
# antigo entre crases, nunca como "  governance:" com essa indentacao exata). -------
for f in \
  "internal/generators/scaffold.go" \
  "internal/generators/scaffold_doctor.go" \
  "npm/src/generators/init.js" \
  "npm/src/commands/discover.js" \
  "pypi/trackfw/generators/init_gen.py" \
  "pypi/trackfw/commands/discover.py"; do
  assert_count "nao reintroduz o job id colidente '$OLD_ID_LINE' em $f" \
    "$f" "$OLD_ID_LINE" 0
done

# --- Os dois ids nunca podem ser o mesmo string — trava contra alguem "corrigir" a
# colisao dando o MESMO nome descritivo novo aos dois workflows (reintroduz o defeito
# original sob um nome diferente). ---------------------------------------------------
checked=$((checked + 1))
if [[ "$GATE_ID" == "$VALIDATE_ID" ]]; then
  echo "check-ci-workflow-job-id-collision: GATE_ID e VALIDATE_ID sao identicos ($GATE_ID) — a colisao seria reintroduzida sob nome novo" >&2
  fail=1
fi

# --- Falsificacao nas duas direções -------------------------------------------------
# (a) Controle: uma fixture com o job id colidente antigo tem que ser CONTADA pela
#     mesma assinatura usada acima — prova que a checagem detecta a regressao, nao
#     so a ausencia de arquivo.
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-job-id-collision.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

cat > "$WORK/regressed_scaffold.go" <<'EOF'
jobs:
  governance:
    runs-on: ubuntu-latest
EOF
checked=$((checked + 1))
got=$(grep -cF -- "$OLD_ID_LINE" "$WORK/regressed_scaffold.go" || true)
if [[ "$got" -ne 1 ]]; then
  echo "check-ci-workflow-job-id-collision: falsify/controle — fixture com a colisao reintroduzida deveria contar 1 ocorrencia de '$OLD_ID_LINE', contou $got" >&2
  fail=1
else
  echo "OK falsify/controle: fixture com job id colidente antigo e detectada (1 ocorrencia)"
fi

# (b) O arquivo real, ja corrigido, tem que ter ZERO ocorrencias do id antigo — repete
#     a mesma assinatura contra o arquivo de producao, provando que o par
#     controle/real nao e so o fixture sintetico passando.
checked=$((checked + 1))
got=$(grep -cF -- "$OLD_ID_LINE" "$ROOT/internal/generators/scaffold.go" || true)
if [[ "$got" -ne 0 ]]; then
  echo "check-ci-workflow-job-id-collision: falsify/real — internal/generators/scaffold.go ainda contem o job id colidente antigo" >&2
  fail=1
else
  echo "OK falsify/real: internal/generators/scaffold.go nao contem mais '$OLD_ID_LINE'"
fi

# --- Guarda de vacuidade — ancorada no mesmo $ROOT usado na varredura acima ---------
expected=15
if [[ "$checked" -ne "$expected" ]]; then
  echo "check-ci-workflow-job-id-collision: vacuidade — esperava checar $expected assinaturas, checou $checked" >&2
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "check-ci-workflow-job-id-collision: FALHOU — job id colidente entre trackfw-gate.yml e trackfw-validate.yml" >&2
  exit 1
fi

echo "check-ci-workflow-job-id-collision: OK — $checked assinaturas confirmadas, sem colisao de job id nos 3 CLIs"
