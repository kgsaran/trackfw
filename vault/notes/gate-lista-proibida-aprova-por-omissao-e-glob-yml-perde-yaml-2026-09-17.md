# gate-lista-proibida-aprova-por-omissao-e-glob-yml-perde-yaml

> Descoberto: 2026-09-17 | REQ-2026-09-17 | ML-1C (corretivo do ML-1B)

## Defeito

O gate `check-ci-workflow-binary-provenance.sh` entregue no ML-1B tinha dois defeitos independentes:

**Defeito 1 — lógica de lista proibida aprova por omissão.**
O gate verificava padrões explicitamente proibidos (`install.sh | sh`, `go install …@v`, `go install …@latest`) e aprovava qualquer workflow que não os contivesse. A classe de bypasses: npm install-g, pip install, brew install, download-artifact, imagem pré-construída — todos os canais de entrega da v8 e outros mecanismos não listados — eram aprovados automaticamente. Medido em repositório de fixtures: 4 workflows com binários publicados por npm/pip/brew/artifact aprovaram com RC=0.

**Defeito 2 — `git ls-files '*.yml'` não casa `*.yaml`.**
GitHub Actions aceita tanto `.yml` quanto `.yaml`. O gate enumerava apenas `*.yml`. Renomear um fixture de `.yml` para `.yaml` reduzia a contagem de 4 para 3 sem nenhum aviso. A guarda de vacuidade só disparava em `CHECKED == 0`, não em redução silenciosa.

## Causa raiz

A lógica de lista proibida tem o problema estrutural de que qualquer mecanismo desconhecido ou não antecipado passa por omissão. A restrição correta é **prova positiva de procedência** — exigir que exista um passo `go build .../cmd/trackfw`, e reprovar quem não o tiver. A enumeração por glob único (`*.yml`) é fragil — GitHub Actions documenta que aceita ambas as extensões.

## Correção (ML-1C)

**Lógica invertida:** o gate agora exige prova positiva de `go build .*/cmd/trackfw` (comment-stripped) em todo workflow que execute `trackfw validate`. Qualquer mecanismo sem esta prova reprova por omissão da prova — não por reconhecimento do mecanismo.

**Enumeração dupla:** `*.yml` e `*.yaml` enumerados via `git ls-files`. Contagem de todos os arquivos em `.github/workflows/` comparada com a contagem enumerada — extensão inesperada gera aviso.

**Contra-braço completo:** 5 fixtures reprovam (npm, pip, brew, artifact, `.yaml`); 1 fixture passa (go build do fonte). Cada fixture em git repo isolado, testado com `SKIP_COUNTER_ARMS=1 bash $0 $dir`.

## Residuais declarados no script

1. Workflows que rodam `trackfw doctor`/`status` com binário publicado estão fora do escopo (não executam `trackfw validate`).
2. GitLab CI (`buildGitLabCIWorkflowContent`, `scaffold.go:2021`) emite `install.sh | sh` sem braço de produtor. O arquivo `.gitlab-ci-trackfw.yml` não está commitado neste repositório (medido: `git ls-files '.gitlab-ci*'` vazio). Cobertura de template é responsabilidade do `check-ci-workflow-pin-parity.sh`.

## Lição

Um gate que mantém uma lista de padrões proibidos mede mecanismos conhecidos — e aprova o próximo mecanismo por construção. A pergunta correta é: "qual prova positiva de procedência é necessária?" Isso fecha a classe, não os exemplos.
