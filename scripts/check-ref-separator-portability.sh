#!/usr/bin/env bash
# Gate: caminho escrito DENTRO de artefato versionado usa sempre "/", e a leitura
# tolera "\" ja gravado (REQ-2026-08-30, AC1/AC3/AC5; ver
# docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md).
#
# Por que este gate precisa provar a escrita SEM maquina Windows: em Linux/macOS
# filepath.Join/path.join/os.path.join sempre produzem "/", entao rodar o comando
# de verdade neste SO nunca reproduz o defeito (ele so aparece com o separador
# nativo do Windows). Falsificar em runtime exigiria um runner Windows no CI, que
# roda uma vez por push na melhor das hipoteses. Este gate prova a mesma coisa por
# ESTRUTURA: os pontos de escrita conhecidos (ML-0A/ML-1A) tem que passar o valor
# por normalizeRefSeparator/_normalize_ref_separator (ou concatenacao explicita
# com "/") antes de ele ser escrito em conteudo versionado — nunca o dst/newPath
# nativo cru. Se alguem reverter o fix (voltar a escrever o valor nativo, ou
# remover a normalizacao na leitura), a assinatura de codigo que este gate procura
# desaparece e ele reprova, no Linux, todo dia, sem precisar de Windows nenhum.
#
# O gate mira SUBSTRINGS de chamada de funcao especificas em arquivos especificos
# — nunca grepa "\\" solto em docs/**, porque este proprio repositorio tem "\"
# literal legitimo em exemplo, regex e prosa (inclusive a REQ e o parecer que
# descrevem este defeito). Ver docs/cli-parity.md ("Quatro propriedades exigidas
# de todo gate novo de paridade").
#
# As assinaturas miram o SUBSTRING da propriedade (ex.: "filepath.Base(normalizeRefSeparator(fmVal))"),
# nao a linha inteira com condicional/chave: pinar a linha completa quebra por
# reformatacao inofensiva (gofmt, clausula extra, `== ""` virando `len()`) sem
# que a normalizacao em si tenha sumido — falso positivo ja falsificado neste
# repositorio (vault: falsify-cenario-pina-linha-de-fonte-por-sed-guard-de-plataforma-quebra-2026-08-31,
# Cenario 81: "o remedio e mirar o substring").
set -euo pipefail

ROOT="${1:-.}"

fail=0
checked=0

# assert_has <label> <file> <exact-string>
# Reprova se o arquivo nao existir OU se a string exata nao aparecer nele.
# Falha nomeando o arquivo e a assinatura esperada — nunca silenciosa.
assert_has() {
  local label=$1 file=$2 needle=$3
  checked=$((checked + 1))
  if [[ ! -f "$ROOT/$file" ]]; then
    echo "check-ref-separator-portability: $label — arquivo ausente: $file" >&2
    fail=1
    return
  fi
  if ! grep -qF -- "$needle" "$ROOT/$file"; then
    echo "check-ref-separator-portability: $label — assinatura ausente em $file" >&2
    echo "  esperado: $needle" >&2
    fail=1
  fi
}

# assert_count <label> <file> <exact-string> <expected-occurrences>
# Como assert_has, mas exige um numero exato de ocorrencias — necessario quando
# dois pontos de escrita/leitura DIFERENTES no mesmo arquivo produzem, coincidentemente,
# a mesma linha de codigo: um assert_has comum passaria se so UM dos dois sobrevivesse,
# escondendo a regressao no outro. Achado real: referenceExists e
# validateREQRoadmapLifecycle em internal/validator/validator.go tem a mesma linha
# `expandedRef := config.ExpandPath(normalizeRefSeparator(ref))` — um grep -qF simples
# nao distingue "os dois normalizam" de "so um normaliza".
assert_count() {
  local label=$1 file=$2 needle=$3 expected_n=$4 got
  checked=$((checked + 1))
  if [[ ! -f "$ROOT/$file" ]]; then
    echo "check-ref-separator-portability: $label — arquivo ausente: $file" >&2
    fail=1
    return
  fi
  got=$(grep -cF -- "$needle" "$ROOT/$file" || true)
  if [[ "$got" -ne "$expected_n" ]]; then
    echo "check-ref-separator-portability: $label — esperava $expected_n ocorrencia(s), achou $got em $file" >&2
    echo "  esperado: $needle" >&2
    fail=1
  fi
}

# --- AC1 — escrita sempre com "/" nos 3 runtimes ---------------------------
# O valor gravado no frontmatter da REQ pareada (roadmap:/Roadmap:) tem que vir
# de normalizeRefSeparator/_normalize_ref_separator sobre o dst nativo — nunca o
# dst cru, que carregaria o separador do SO em que `roadmap move` roda.
assert_has "Go: escrita usa portableDst normalizado" \
  "internal/generators/roadmap.go" \
  'portableDst := normalizeRefSeparator(dst)'
assert_has "Go: sync recebe o valor normalizado, nao o dst nativo" \
  "internal/generators/roadmap.go" \
  'syncREQReferences(filepath.Base(src), portableDst)'

assert_has "Node: sync recebe normalizeRefSeparator(dst)" \
  "npm/src/generators/roadmap.js" \
  'syncReqReferences(basename, normalizeRefSeparator(dst), cfg)'

assert_has "Python: escrita usa portable_path normalizado" \
  "pypi/trackfw/commands/roadmap.py" \
  'portable_path = _normalize_ref_separator(new_path)'
assert_has "Python: sync recebe o valor normalizado, nao o new_path nativo" \
  "pypi/trackfw/commands/roadmap.py" \
  'sync_paired_req_references(portable_path, cfg)'

# .trackfw-log (by_agent): concatenacao explicita com "/", nunca Join/os.path.join
# nativo — Go e Node ja seguiam este padrao (ML-0A achado 4); Python foi corrigido
# no ML-1A para igualar.
assert_has "Go: log_basename por concatenacao explicita com /" \
  "internal/generators/roadmap.go" \
  'logBasename = agent + "/" + filepath.Base(src)'
assert_has "Node: logBasename por concatenacao explicita com /" \
  "npm/src/generators/roadmap.js" \
  "logBasename = agent + '/' + basename"
assert_has "Python: log_basename por concatenacao explicita com /" \
  "pypi/trackfw/generators/roadmap.py" \
  'log_basename = agent + "/" + basename'

# --- AC3 — leitura tolerante a "\" ja gravado -------------------------------
# Cada ponto que resolve uma referencia de conteudo versionado no filesystem, ou
# compara chave de string contra conteudo sempre gravado com "/", tem que passar
# pelo normalizador antes.
#
# Go: referenceExists() e validateREQRoadmapLifecycle() em validator.go produzem,
# coincidentemente, a MESMA linha de normalizacao — assert_count exige as duas
# ocorrencias, nao so uma (ver comentario de assert_count acima).
assert_count "Go validate: referenceExists + validateREQRoadmapLifecycle normalizam antes de resolver" \
  "internal/validator/validator.go" \
  'expandedRef := config.ExpandPath(normalizeRefSeparator(ref))' \
  2
assert_has "Go validate: provenanceKey normalizado antes do lookup" \
  "internal/validator/validator_thirdparty_provenance.go" \
  'provenanceKey = normalizeRefSeparator(provenanceKey)'
assert_has "Go serve: node ID normalizado (api/chain)" \
  "internal/serve/api_chain.go" \
  'nodeID := normalizeRefSeparator(path)'
assert_has "Go serve: edge.To normalizado (api/chain)" \
  "internal/serve/api_chain.go" \
  'edges = append(edges, chainEdge{From: nodeID, To: normalizeRefSeparator(val)})'

assert_has "Python validate: _reference_exists normaliza antes do os.path.exists" \
  "pypi/trackfw/validator.py" \
  'return os.path.exists(expand_path(_normalize_ref_separator(ref)))'
assert_has "Python validate: validate_req_roadmap_lifecycle normaliza antes do os.path.isfile" \
  "pypi/trackfw/validator.py" \
  'expanded_ref = expand_path(_normalize_ref_separator(ref))'
assert_has "Python validate: provenance_key normalizado antes do lookup" \
  "pypi/trackfw/validator.py" \
  'provenance_key = _normalize_ref_separator(os.path.relpath(destination, root))'

# Cura de REQ ja suja: syncREQReferences/syncReqReferences/sync_paired_req_references compara o
# fmVal/currentRef existente contra o basename movido — sem normalizar antes, uma REQ ja gravada
# com "\" (por um roadmap move anterior, no Windows) nunca casa e fica orfa para sempre, mesmo
# apos este fix de escrita. Assinatura mira so o substring da propriedade (nao a linha
# condicional inteira) — ver nota de brittleness no cabecalho do script.
assert_has "Go: fmVal normalizado antes da comparacao de basename (cura de REQ suja)" \
  "internal/generators/roadmap.go" \
  'filepath.Base(normalizeRefSeparator(fmVal))'
assert_has "Node: currentRef normalizado antes da comparacao de basename (cura de REQ suja)" \
  "npm/src/generators/roadmap.js" \
  'path.basename(normalizeRefSeparator(currentRef))'
assert_has "Python: current_ref normalizado antes da comparacao de basename (cura de REQ suja)" \
  "pypi/trackfw/generators/roadmap.py" \
  'os.path.basename(_normalize_ref_separator(current_ref))'

# --- Guardas de vacuidade -----------------------------------------------------
# Duas guardas distintas, cada uma cobrindo uma forma diferente de "passar sem
# checar nada" (docs/cli-parity.md, "Quatro propriedades exigidas de todo gate
# novo de paridade", propriedade 2):
#
# 1. Contagem de asserts (abaixo): pega alguem REMOVENDO uma chamada assert_has/
#    assert_count do corpo deste script (reduz cobertura sem que nenhum grep
#    individual reprove). NAO pega a varredura visitando zero arquivos — cada
#    assert_has/assert_count e uma checagem pontual e nomeada, nunca um
#    `find`/`grep -r` que possa devolver lista vazia em silencio.
# 2. `[[ ! -f "$ROOT/$file" ]]` dentro de assert_has/assert_count (definido acima):
#    pega a OUTRA forma de vacuidade — arquivo/diretorio movido, renomeado ou
#    ausente. Cada checagem verifica a existencia do proprio arquivo antes do
#    grep, entao apontar ROOT para um diretorio vazio ou inexistente reprova em
#    todas as assinaturas, uma por uma, nomeando o arquivo ausente em cada
#    linha — nunca um "0 encontrados, gate passa" silencioso. Falsificado em
#    scratchpad/refsep/{empty-root,nonexistent-dir-xyz} (ver relatorio do ML).
#
# 18 e o numero de chamadas assert_has/assert_count acima (a assert_count do Go
# conta como 1 chamada mas verifica 2 ocorrencias na mesma linha) — nomeado, nao
# magico.
expected=18
if [[ "$checked" -ne "$expected" ]]; then
  echo "check-ref-separator-portability: vacuidade — esperava checar $expected assinaturas, checou $checked" >&2
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "check-ref-separator-portability: FALHOU — separador nativo vazando para conteudo versionado, ou leitura sem tolerancia a '\\'" >&2
  exit 1
fi

echo "check-ref-separator-portability: OK — $checked assinaturas de escrita/leitura portavel confirmadas"
