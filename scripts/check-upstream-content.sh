#!/usr/bin/env bash
# Gate: conteudo do upstream nao entra em docs/ nem em vault/.
#
# A ADR-2026-08-29-adotar-upstream-como-base diz que produto vem do upstream e que a
# governanca e o conteudo dele nao sao importados. A politica NAO e auto-aplicavel:
# entrou tres vezes em 2026-08-29 — vault/notes/index.md, duas notas de vault e uma
# ADR — sempre SEM gerar conflito, porque caminho novo nao colide com nada.
#
# Sinal usado: proveniencia por caminho. Arquivo sob docs/ ou vault/ que exista
# tambem em upstream/main e conteudo do upstream, a menos que esteja em KEEP abaixo.
# Lista de caminhos proibidos envelheceria a cada release; a interseccao nao.
#
# ESCOPO: so docs/ e vault/. Em internal/ e cmd/ (e, ate a v8, npm/src/ e pypi/trackfw/) coincidir com o
# upstream e o comportamento DESEJADO — um gate que reclamasse disso estaria invertido.
#
# Ver REQ-2026-08-29-politica-de-conteudo-do-upstream-sem-gate.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

REF="upstream/main"

if ! git rev-parse --verify --quiet "$REF" >/dev/null; then
  echo "conteudo do upstream: nao consigo checar — \`$REF\` nao existe localmente."
  echo "  rode \`git fetch upstream\` e tente de novo."
  echo "  Este gate REPROVA em vez de passar: verde por nao conseguir checar seria pior"
  echo "  que vermelho, porque pareceria cobertura."
  exit 1
fi

# Arquivos que coincidem com o upstream DE PROPOSITO. Cada um com o motivo — sem isso
# a lista vira deposito, com uma linha acrescentada a cada reprovacao, e a politica
# evapora sem ninguem decidir nada.
KEEP=$(cat <<'LIST'
docs/agents-working-context.md                          # handoff entre sessoes; escrevemos nele
docs/adr/ADR-2026-09-03-layout-canonico-de-req-em-by-agent-e-o-invariante-de-que-req-nao-tem-dimensao-de-estado.md  # DELE, e FICA: 5 REQs nossas a citam no campo `adr:` do frontmatter, e o `req_has_adr` do validate cobra. Remover quebraria o validate em 5 pontos. Procedencia declarada no frontmatter dela (upstream_origin), como as 28 REQs herdadas. Ver REQ-2026-09-09-governanca-do-upstream, ML-3A.
docs/cli-parity.md                                      # doc de produto
docs/gate-design-principles.md                          # doc de produto
docs/demo.gif                                           # doc de produto
docs/visao-projeto/VISION.md                            # doc de produto
docs/schema/adr.schema.json                             # schema de produto
docs/schema/req.schema.json                             # schema de produto
docs/schema/roadmap.schema.json                         # schema de produto
docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md # lido por internal/thirdparty (teste quebra sem ele)
docs/roadmaps/.trackfw-log                              # nosso log; coincide so no caminho

# As 3 abaixo sao FIXTURE, nao governanca. internal/validator/validator_test.go
# (TestExtractRefPath_TresREQsReaisDoRepositorio) e pypi/tests/test_validator.py
# (test_extract_ref_path_resolve_reqs_reais_com_backtick, removido com pypi/tests/ na v8) leem estes arquivos do
# repositorio por caminho fixo, a partir da raiz. Sem eles as suites Go e Python
# reprovam na CI deste fork — e o motivo e a propria ADR: a governanca do upstream
# nao e importada, entao docs/req/ nao existe aqui.
#
# Sao inertes para a governanca daqui: req_dir e docs/requisicoes, entao o validate
# e o status nao varrem docs/req/. Ficam so como fixture.
#
# A causa raiz e teste acoplado ao conteudo do proprio repositorio, e a correcao e
# de la (usar fixture em tempdir). Reportada no kgsaran/trackfw#216. Quando entrar,
# estas 3 linhas saem.
docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md          # fixture de TestExtractRefPath
docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md    # fixture de TestExtractRefPath
docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md  # fixture de TestExtractRefPath
LIST
)

keep_list=$(printf '%s\n' "$KEEP" | sed 's/[[:space:]]*#.*$//' | sed '/^[[:space:]]*$/d' | sort)

meus=$(git ls-files docs vault | sort)
deles=$(git ls-tree -r --name-only "$REF" docs vault | sort)
coincidem=$(comm -12 <(printf '%s\n' "$meus") <(printf '%s\n' "$deles"))
vazou=$(comm -23 <(printf '%s\n' "$coincidem") <(printf '%s\n' "$keep_list"))

if [ -n "$vazou" ]; then
  echo "conteudo do upstream em docs/ ou vault/:"
  printf '%s\n' "$vazou" | sed 's/^/  /'
  echo
  echo "Remova, ou — se for mesmo para ficar — acrescente ao KEEP deste script COM O MOTIVO."
  echo "Ver ADR-2026-08-29-adotar-upstream-como-base."
  exit 1
fi

echo "Conteudo do upstream: nada indevido em docs/ nem em vault/."
