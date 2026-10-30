# Wave 0 — Modelo de Ameaça: Instalação de Consumidor pela Tag

> Roadmap: ROADMAP-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md
> REQ: REQ-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md
> ADR: ADR-2026-10-30-go-install-nao-e-canal-de-instalacao-do-trackfw.md
> Issue de origem: #637
> Hades (security) — 2026-10-30

Mapeamento de ações do ML-0A para as quatro seções mandatórias:
- Ação 1 (superfícies externas) → §2.2; ação 2 (injeção) → §3 Injeção; ação 3 (enumeração) → §1; ações 4 e 5 (AC5 chain e falsificação) → §3; ação 6 (prescrições e residual) → Prescrições + §4

---

## 1. Completude da Enumeração

A REQ enumera os sítios com `go install` ou `releases/latest`. Esta seção fecha a lista.

### Sítios ativos — mecanismo de instalação em CI

Saída bruta (combinação dos dois greps, sem truncagem):

```
$ grep -rn 'kgsaran/trackfw/cmd/trackfw@\|go install github\.com/kgsaran\|releases/latest' \
    --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' . \
    | grep -vE 'testdata/corpus|vault/notes|docs/roadmaps|docs/req|docs/adr|docs/agents-working|docs/seguranca' \
    | grep -v '_test\.go' | grep -v '\.git/'
(saída abreviada: 57 linhas)

$ grep -rn 'kgsaran/trackfw/cmd/trackfw@\|go install github\.com/kgsaran\|releases/latest' \
    --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' . \
    | grep -vE 'testdata/corpus|vault/notes|docs/roadmaps|docs/req|docs/adr|docs/agents-working|docs/seguranca' \
    | grep -v '_test\.go' | grep -v '\.git/' | wc -l
57
```

Classificação dos 57 resultados:

- **No escopo** (tabela abaixo): `scaffold_doctor.go:152`, `scaffold.go:1801,1850`, `README.md:106`, `VISION.md:206`, `site/index.md:53`, `site/guide/getting-started.md:43`, `site/en/index.md:53`, `site/en/guide/getting-started.md:43`, `docs/cli-parity.md:7550` — 9 sítios.
- **Canais corretos** (`releases/latest/download/install.sh` ou `install.ps1` em documentação de usuário — este é o padrão esperado, não o defeito): `README.md:94,168,174,197`, `site/index.md:49`, `site/guide/getting-started.md:29,35`, `site/en/index.md:49`, `site/en/guide/getting-started.md:29,35`, `VISION.md:202,207`, `CHANGELOG.md:20`, `docs/cli-parity.md:31` — fora do escopo.
- **Slash-command templates** (`.claude/commands/`, `.gemini/commands/`): usam `releases/latest` por design, sem pin, não são CI (NEG SCOPE) — fora do escopo.
- **Internos de scripts** (`scripts/install.sh:163,167` chamando API `releases/latest`; `scripts/check-ci-workflow-pin-parity.sh`; `scripts/check-install-version-pin.sh`): vetores de teste e lógica interna dos scripts — fora do escopo.
- **Corpus / docs históricos** (`scripts/testdata/`, `docs/portabilidade/`, `docs/fila-de-execucao.md`, `.claude/agent-memory/`): histórico — fora do escopo.
- **scaffold.go:378–431** (slash-command templates gerados): rótulo "scripts de hook" na NEG SCOPE está errado; escopo está correto — fora do escopo, informado ao arquiteto.

| sítio | linha | mecanismo atual | efeito |
|---|---|---|---|
| `internal/generators/scaffold_doctor.go` | 152 | `go install ...@v<version>` (consumer `trackfw-validate.yml`) | rc=1 em v11.x — gate nunca roda |
| `internal/generators/scaffold.go` | 1801 | `curl releases/latest/download/install.sh \| sh` + `TRACKFW_VERSION` (consumer `trackfw-gate.yml`) | roda; script não pinado à tag |
| `internal/generators/scaffold.go` | 1850 | `curl releases/latest/download/install.sh \| sh` + `TRACKFW_VERSION` (consumer `.gitlab-ci-trackfw.yml`) | roda; script não pinado à tag |
| `README.md` | 106 | `go install .../cmd/trackfw@latest` | instala v1.1.0 em silêncio |
| `docs/visao-projeto/VISION.md` | 206 | `go install .../cmd/trackfw@latest` | idem; fora da lista da REQ — adicionar ao ML-1B |
| `site/index.md` | 53 | idem | idem |
| `site/guide/getting-started.md` | 43 | idem | idem |
| `site/en/index.md` | 53 | idem | idem |
| `site/en/guide/getting-started.md` | 43 | idem | idem |
| `docs/cli-parity.md` | 7550 | descreve o mecanismo quebrado | documentação desatualizada |

Sítios FORA DO ESCOPO confirmados:

- `scaffold.go:378–431` — templates de slash-commands (`.claude/commands/`). Usam `releases/latest` por design, sem pin de versão, não são templates de CI. O NEG SCOPE da REQ os rotula "scripts de hook" — o rótulo está errado; o escopo está correto. Informado ao arquiteto.
- `.github/workflows/` do produtor — não é template de consumidor.

### Testes e golden files que afirmam o template antigo (ML-1A precisa atualizar)

```
$ grep -rn 'releases/latest' internal/generators/testdata/ internal/discover/
$ grep -rn 'go install github.com/kgsaran' internal/ --include='*.go' | grep -v 'testdata/corpus'
```

| arquivo | linha | afirmação |
|---|---|---|
| `internal/discover/discover_test.go` | 797 | `TestWriteCIWorkflow_PinsGoInstallToBinaryVersion` — afirma `go install ...@v<version>` no template |
| `internal/discover/discover_test.go` | 825 | `TestWriteCIWorkflow_VersionNotHardcoded` — stubs versão e afirma `trackfw@v9.9.9-stub` |
| `internal/generators/update_test.go` | 2243 | conteúdo "stale" com `go install @v0.0.1` |
| `internal/generators/update_test.go` | 2400 | idem |
| `internal/generators/testdata/ci-trigger/discover-consumer.golden` | inteiro | golden com `go install @vTRACKFW_VERSION_PLACEHOLDER` |
| `internal/generators/testdata/ci-trigger/gate-consumer.golden` | 17 | `releases/latest/download/install.sh` |
| `internal/generators/testdata/ci-trigger/gitlab.golden` | 10 | `releases/latest/download/install.sh` |

### Gate que quebra em `make quality` após ML-1A

`scripts/check-ci-workflow-pin-parity.sh` roda em `make quality` (Makefile:115). Três pontos de quebra após ML-1A:

| chamada / linha | problema pós-ML-1A |
|---|---|
| `check_discover_pin "$RUN1/dv_go_consumer.yml"` L271 | verifica `@v<version>` — ausente no novo consumer |
| `check_action_pins "$RUN1/dv_go_consumer.yml" 7` L273 | verifica `setup-go` — ausente no novo consumer (install.sh não precisa); `check_checkout_pin` (L205) já existe para este caso |
| `sed "s/@v${GO_VERSION}/@latest/" "$FALS/dv-consumer-latest.yml"` L341 | no-op: `@v11.2.0` não aparece no novo template; `assert_check_fails` emite "falhou mas sem o motivo esperado" (FAIL explícito — verificado) |

A vacuidade do vetor (5) não passa silenciosamente. A ameaça real é o implementador que apaga o vetor.

Guarda de vacuidade: invocações de `run_check`/`assert_check_fails` (âncoras `^[[:space:]]*(run_check|assert_check_fails) "`) totalizam **21** hoje — contados por grep, não por execução observada. O `SCENARIOS_RUN` impresso ao final pode ser maior que 21 se algum `run_check` interno chamar `ok()`/`fail()` mais de uma vez por invocação. Por isso, a guarda correta é: **ML-1A executa o script antes de editar qualquer linha, registra o valor impresso de `SCENARIOS_RUN` na linha "Cenários executados:" do rodapé, e depois da edição o valor impresso deve ser pelo menos esse.** Se cair, um vetor foi removido. A verificação de idempotência é `run_check "pin-parity/idempotency"` e está incluída no total.

`! cmp -s a b` não é uma guarda correta sob `set -e` — verificado: `bash -c 'set -euo pipefail; ! cmp -s /etc/hosts /etc/hosts; echo STILL_RUNNING'` → imprime `STILL_RUNNING`. O padrão correto está prescrito em P1.

**`internal/discover/discover_test.go` está em `internal/discover/`, fora dos gates declarados no roadmap (`go test ./internal/generators/`). O gate do ML-1A precisa incluir `go test ./internal/discover/`.**

---

## 2. Modelo de Ameaça

### 2.1 Adversário

O adversário desta Wave 0 é o implementador apressado e o arquiteto otimista.

**Ameaça A — ML-1A entrega template novo mas não atualiza o gate de paridade.**

Medido com o novo template de consumidor que ML-1A produziria (`TRACKFW_VERSION: "11.2.0"` + `curl releases/download/v${TRACKFW_VERSION}/install.sh | sh`):

```
$ source <(sed -n '53,68p' scripts/check-ci-workflow-pin-parity.sh)
$ source <(sed -n '152,163p' scripts/check-ci-workflow-pin-parity.sh)
$ check_discover_pin new_consumer.yml "11.2.0" && echo "PASS" || echo "FAILED"
go install não contém @v11.2.0 pinada
FAILED

$ cp new_consumer.yml consumer-latest.yml
$ sed -i.bak "s/@v11.2.0/@latest/" consumer-latest.yml
$ diff new_consumer.yml consumer-latest.yml && echo "(no diff)"
(no diff)
```

`check_discover_pin` reprova o novo template com mensagem diferente da que `assert_check_fails` espera. O no-op é confirmado. `assert_check_fails` emite "falhou mas sem o motivo esperado" (FAIL explícito). O implementador pode apagar o vetor para fazer o gate ficar verde.

**Ameaça B — ML-1A usa URL literal `releases/download/v11.2.0/install.sh` em vez de `${TRACKFW_VERSION}`.**
`TRACKFW_VERSION` e URL ficam desacoplados. Uma atualização futura do `TRACKFW_VERSION` não atualiza a URL; `check_version_pin` passa mas o URL ainda carrega o literal antigo.

**Ameaça C — `internal/discover/discover_test.go` não está nos gates do ML-1A; ML-1A vai verde, ML-2A falha (`make quality`).**
`TestWriteCIWorkflow_PinsGoInstallToBinaryVersion` (L797) e `TestWriteCIWorkflow_VersionNotHardcoded` (L825) ficam vermelhos após ML-1A. Gates declarados no roadmap (`go test ./internal/generators/`) não cobrem `internal/discover/`.

### 2.2 Superfícies externas

**`releases/latest/download/install.sh`** (estado atual de todos os 3 consumer templates):

O script é baixado de qualquer que seja a release mais recente no momento do CI. `TRACKFW_VERSION` pina o binário, mas não o script. Qualquer pessoa com permissão de `write` pode sobrescrever `install.sh` numa release existente via `gh release upload --clobber`. Com `releases/latest`: uma nova release muda o alvo, atingindo todos os consumidores na próxima execução.

```
$ gh release upload --help 2>&1 | grep clobber
  --clobber   Delete and re-upload existing assets of the same name

$ gh api repos/kgsaran/trackfw/releases/tags/v11.2.0 --jq '.immutable'
false
```

Release assets são **mutáveis por quem tem write access**. `signs:` ausente do `.goreleaser.yaml`. `checksums.txt` oferece integridade de transporte mas não autenticidade.

**`releases/download/v<version>/install.sh`** (AC3 proposto — ML-1A aplica a todos os 3 consumer templates):

Com a tag pinada, o vetor de "nova release alcança todos os consumidores" é fechado. O vetor de `--clobber` permanece com blast radius limitado à versão afetada. A pinagem reduz exposição; não garante imutabilidade.

---

## 3. Alvos de Falsificação nas Duas Direções

### AC1 — Consumer `trackfw-validate.yml` instala por `install.sh`, não por `go install`

| direção | onde entra | gate que deveria pegar |
|---|---|---|
| FN | Template tem `run: # go install ...` (comentário) | Teste âncora em `run: go install` como prefixo de campo YAML, não em linha com `#` |
| FN | Regressão: `IsProducerGoMod` retorna `false` no produtor (go.mod com CRLF) | `check_producer_no_go_install` + AC6 |
| FP | Teste rejeita `go install ./cmd/trackfw` legítimo no template do produtor | Guard `isProducer=true` em `buildDiscoverGitHubActionsWorkflowContentPR` |

### AC2 — Teste que reprova emissão de `go install` em consumidor

O teste abrange os 3 builders de CI em `internal/generators/`: `buildDiscoverGitHubActionsWorkflowContentPR`, `buildGitHubActionsWorkflowContent` (consumer path), `buildGitLabCIWorkflowContent` (consumer path). Não abrange os slash-command templates em `scaffold.go:378–431` (fora do NEG SCOPE).

| direção | onde entra | gate que deveria pegar |
|---|---|---|
| FN | Teste verifica apenas trigger `pull_request`; trigger `manual` usa `ciTrigger="manual"` com output diferente | Iterar os dois triggers; assertar `workflow_dispatch` presente no output manual |
| FN | `go run github.com/kgsaran/trackfw` ou `go get github.com/kgsaran/trackfw` não testados | Rejeitar `go run` e `go get` do mesmo module path em consumer templates |
| FP | Comentário menciona `go install` → teste rejeita template válido | Ancoragem em `run: go install` (prefixo de campo YAML, não em linha com `#`) |

### AC3 — Script baixado da tag pinada, não de `releases/latest`

| direção | onde entra | gate que deveria pegar |
|---|---|---|
| FN | Gate verifica `TRACKFW_VERSION` mas não a ausência de `releases/latest` na URL | Gate verifica ausência de `releases/latest` em step `run:` AND presença de `releases/download/v` para todos os 3 consumer templates |
| FN | URL usa `releases/download/v${TRACKFW_VERSION}/` mas segundo step ainda usa `releases/latest` | Gate varre todos os steps `run:` do job, não apenas o primeiro |
| FP | Gate procura literal `@v11.2.0` em vez de padrão de URL | Gate correto busca ausência de `releases/latest`; não exige literal de versão |

### AC5 — `trackfw doctor` detecta template antigo; `trackfw update` corrige

```
$ command -v trackfw; trackfw --version
/opt/homebrew/bin/trackfw
trackfw 11.2.0
```

Fixture em `mktemp -d`: `github.com/acme/cmdb` com `trackfw-validate.yml` antigo (go install @v11.2.0):

```
$ HOME=$(mktemp -d) trackfw doctor 2>&1 | grep -E 'scaffold-divergent|finding'
trackfw doctor: 6 finding(s) -- 0 unregistered-write, 0 hand-modified, 0 unknown-content, 1 scaffold-divergent, 5 scaffold-missing, 0 scaffold-wrong-mode, 0 required-status-checks-missing, 0 enforce-admins-disabled, 0 hooks-path-neutralized, 0 not-evaluated, 0 scaffold-workflow-duplicated
[scaffold-divergent] .github/workflows/trackfw-validate.yml

$ HOME=$(mktemp -d) trackfw update --targets ci-workflow 2>&1
trackfw update
✓ ci-workflow: updated (.github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml, .github/workflows/trackfw-validate.yml)
updated=1 skipped=0 missing=0 failed=0

$ HOME=$(mktemp -d) trackfw doctor 2>&1 | grep -E 'scaffold-divergent|finding'
trackfw doctor: 5 finding(s) -- 0 unregistered-write, 0 hand-modified, 0 unknown-content, 0 scaffold-divergent, 5 scaffold-missing, 0 scaffold-wrong-mode, 0 required-status-checks-missing, 0 enforce-admins-disabled, 0 hooks-path-neutralized, 0 not-evaluated, 0 scaffold-workflow-duplicated
```

O doctor detecta o template stale. O update reescreve. O doctor confirma 0 divergências.

**O que esta medição prova:** o mecanismo doctor/update funciona no binário pré-fix (11.2.0 com template `go install`). Não prova a transição old→new porque ML-1A ainda não foi executado; o binário usado aqui GERA o template `go install`. O `shasum -a 256` do `trackfw-validate.yml` após update retornou `aadf2cb1b4eac30dd2fe3a94f55538b4864ff826a5eaba59230975ee96cca996` — esse é o hash do template ATUAL (pré-fix). Após ML-1A, esse valor se torna o hash da fixture "stale" para ML-2A — um consumidor que ainda usa o template antigo exibirá `scaffold-divergent` ao ser inspecionado pelo binário pós-fix. P7 carrega essa verificação para ML-2A.

**Severity anchor:** O consumer template não tem step de `git fetch origin/main`. O validador emite WARNING (`originAnchorRefUnreadable`, não bloqueante após ML-1B fix). O bloqueio do CMDB era o `go install` (rc=1 antes de qualquer validação), não o severity anchor.

| direção | onde entra | gate que deveria pegar |
|---|---|---|
| FN | `refreshDiscoverGitHubActionsWorkflowIfPresent` segue symlink para sair do root | `pathguard.RejectAndReport` bloqueia |
| FP | Doctor reporta divergência em template recém-atualizado | Contra-braço: arquivo correto → zero `scaffold-divergent` |

### AC6 — Template do produtor inalterado

| direção | onde entra | gate que deveria pegar |
|---|---|---|
| FN | `IsProducerGoMod` falha em go.mod com CRLF | `strings.TrimSpace` por linha já aplicado; coberto por teste existente |
| FN | ML-1A edita `buildDiscoverGitHubActionsWorkflowContentPR(true)` indiretamente | SHA-256 do template de produtor pré-fix = SHA-256 pós-fix (Apêndice A) |
| FP | Consumidor com `module github.com/kgsaran/trackfw` recebe template de produtor | Comportamento correto — esse projeto é o produtor |

Templates de produtor não embarcam o número de versão (confirmado: `grep -c '11.2.0' dv_go_producer.yml` → `0`; idem `gh_go_producer.yml`). O check "não muda" é incondicional. Templates de consumidor embarcam a versão; ML-2A deve emparelhar a mudança de hash com asserções de conteúdo (ausência de `go install`, presença de `releases/download/v`), não comparação de hash isolada.

### Injeção — `version.Version` → YAML e URL

`version.Version` é uma `var` Go sem validação no código, inicializada via ldflags:
```
-X 'github.com/kgsaran/trackfw/internal/version.Version={{.Version}}'
```
`{{.Version}}` vem da tag git validada pelo GoReleaser. Tags git permitem `/` (verificado: `git check-ref-format refs/tags/a/b` → OK), mas o parser semver do GoReleaser rejeita `/`.

**Superfícies de interpolação:**

1. `TRACKFW_VERSION: "` + version.Version + `"` no bloco `env:` do YAML.
2. URL no step `run:` — com P2, usa `${TRACKFW_VERSION}` (expansão de parâmetro do shell do CI), eliminando o segundo ponto de interpolação Go.

**Vectors medidos (`TRACKFW_INSTALL_DRYRUN=1`):**

```
$ for v in "11.2.0" "11.2.0-rc1" "11.2.0-beta12" "11.2.0-alpha1" \
           "11.2.0-rc1000" "11.2" '11.2.0"injected' $'11.2.0\nnewline' \
           "11.2.0.1" "v11.2.0"; do
    result=$(TRACKFW_INSTALL_DRYRUN=1 TRACKFW_VERSION="$v" bash scripts/install.sh 2>&1)
    rc=$?
    safe_v=$(printf '%s' "$v" | cat -v)
    if [ $rc -eq 0 ]; then
      echo "ACCEPT rc=0 | '$safe_v'"
    else
      echo "REJECT rc=$rc | '$safe_v' | $(echo "$result" | head -1)"
    fi
  done
ACCEPT rc=0 | '11.2.0'
ACCEPT rc=0 | '11.2.0-rc1'
ACCEPT rc=0 | '11.2.0-beta12'
REJECT rc=1 | '11.2.0-alpha1' | Erro: TRACKFW_VERSION invalida: '11.2.0-alpha1'
REJECT rc=1 | '11.2.0-rc1000' | Erro: TRACKFW_VERSION invalida: '11.2.0-rc1000'
REJECT rc=1 | '11.2' | Erro: TRACKFW_VERSION invalida: '11.2'
REJECT rc=1 | '11.2.0"injected' | Erro: TRACKFW_VERSION invalida: '11.2.0"injected'
REJECT rc=1 | '11.2.0
newline' | Erro: TRACKFW_VERSION invalida: '11.2.0
REJECT rc=1 | '11.2.0.1' | Erro: TRACKFW_VERSION invalida: '11.2.0.1'
ACCEPT rc=0 | 'v11.2.0'
```

| vetor | resultado medido | install.sh valida? |
|---|---|---|
| `11.2.0` | ACCEPT rc=0 | sim |
| `11.2.0-rc1` | ACCEPT rc=0 | sim |
| `11.2.0-beta12` | ACCEPT rc=0 | sim |
| `11.2.0-alpha1` | REJECT rc=1 | não |
| `11.2.0-rc1000` | REJECT rc=1 (4 dígitos) | não |
| `11.2` | REJECT rc=1 | não |
| `11.2.0"injected` | REJECT rc=1 | não |
| `11.2.0\nnewline` | REJECT rc=1 | não |
| `11.2.0.1` | REJECT rc=1 | não |
| `v11.2.0` | ACCEPT rc=0 (strip v) | sim |
| `9.9.9-stub` | REJECT rc=1 | não |

---

## Prescrições

### Para ML-1A (apolo-tf, `internal/generators/`)

**P1.** Adicionar à lista de "Files affected" do ML-1A:
- `scripts/check-ci-workflow-pin-parity.sh`
- `internal/discover/discover_test.go`
- `internal/generators/testdata/ci-trigger/discover-consumer.golden`
- `internal/generators/testdata/ci-trigger/gate-consumer.golden`
- `internal/generators/testdata/ci-trigger/gitlab.golden`

Adicionar aos **Gates da wave** do ML-1A:
```bash
go test ./internal/discover/ -count=1
bash scripts/check-ci-workflow-pin-parity.sh
```

As três quebras de gate e suas correções:

| item | problema | correção |
|---|---|---|
| `check_discover_pin` L152–163 | busca `@v<version>` ausente no novo template | substituir por check que verifica ausência de `releases/latest` em step `run:` AND `TRACKFW_VERSION: "<version>"` (reutilizar `check_version_pin`); estender a ausência de `releases/latest` para `gh_go_consumer.yml` e `gl_go.yml` também (AC3 cobre os 3) |
| `check_action_pins` do dv_go_consumer L273 | busca `setup-go` ausente no novo template | substituir por `check_checkout_pin` (L205, já existente para este caso) |
| falsificação (5) L338–343 | `sed "@v${GO_VERSION}/@latest"` é no-op | substituir sed por `sed 's/releases\/download\/v\${TRACKFW_VERSION}\//releases\/latest\/download\//g'`; confirmado não ser no-op (diff produziu linha alterada); atualizar expected error |

Guarda de no-ops via o counter existente do script (não usar `! cmp -s`, que é no-op sob `set -e`):
```sh
if cmp -s "$FALS/dv-consumer-latest.yml" "$RUN1/dv_go_consumer.yml"; then
  fail "falsify/discover-workflow/consumer-latest-noop" "sed foi no-op — arquivo falsificado igual ao original"
fi
```
(`fail()` incrementa `SCENARIOS_RUN` e `FAIL`.)

**P2.** Na URL do step `run:` do novo template, usar `${TRACKFW_VERSION}` (não literal) e double-quotar a URL. ML-1A aplica este padrão aos 3 consumer templates (hoje todos usam `releases/latest`):
```yaml
run: curl -sSfL "https://github.com/kgsaran/trackfw/releases/download/v${TRACKFW_VERSION}/install.sh" | sh
```

**P3.** O `scaffold_doctor.go` (L152) não tem `timeout-minutes`; o `scaffold.go:1739,1793` (gate/gitlab) já tem. Adicionar `timeout-minutes: 10` ao job `governance-go-install` no novo template do validate consumer para manter consistência com os outros dois. `check_timeout_minutes` (L130) já existe e cobre o novo template.

**P4.** O teste AC2 deve:
- Abranger os 3 builders de CI por nome de função. Não varrer slash-command templates.
- Rejeitar `run: go install github.com/kgsaran/trackfw` (qualquer forma), `run: go run ...`, `run: go get ...` em consumer.
- Não rejeitar no produtor.
- Não casar linhas com `#`.
- Iterar os dois triggers e assertar `workflow_dispatch` no output manual.

**P5.** Asserções positivas no teste AC2:
- `TRACKFW_VERSION` = `version.Version` em todos os consumer templates.
- URL contém `${TRACKFW_VERSION}` (não literal) — detecta se P2 não foi aplicado.
- Ausência de `releases/latest` em step `run:` de qualquer consumer template.

**P6.** O teste de AC5 deve usar conteúdo byte-idêntico ao SHA-256 `aadf2cb1b4eac30dd2fe3a94f55538b4864ff826a5eaba59230975ee96cca996` (validate consumer PR pré-fix) como fixture "stale" contra o binário pós-fix. Confirmado: o binário `/opt/homebrew/bin/trackfw 11.2.0` produz esse hash para o template atual (medido via `trackfw update` em fixture scratchpad e `shasum -a 256`). Após ML-1A, esse hash representa o template antigo; o binário pós-fix deve detectá-lo como `scaffold-divergent` e o update deve substituí-lo pelo template novo.

**P7.** O teste de AC6 deve comparar o SHA-256 do output pós-ML-1A para os templates de produtor contra o golden pré-fix (Apêndice A). Se qualquer hash de produtor mudar, o PR falha.

### Para ML-1B (afrodite-tf, docs)

**P8.** Remover `go install github.com/kgsaran/trackfw/cmd/trackfw@latest` dos seis arquivos:
`README.md:106`, `docs/visao-projeto/VISION.md:206`, `site/index.md:53`, `site/guide/getting-started.md:43`, `site/en/index.md:53`, `site/en/guide/getting-started.md:43`.
`VISION.md` não estava na lista da REQ; o ML-1B e seu AC grep devem ser atualizados:
```bash
grep -rn "cmd/trackfw@" README.md site docs/visao-projeto --include=*.md
```

---

## Apêndice A — Hashes Pré-Fix (Golden para ML-2A)

**Método de captura:** `scripts/check-ci-workflow-pin-parity.sh` usa `dump_go` (L72–106) que escreve um test body temporário em `internal/generators/zz_dump_ci_workflow_pin_parity_test.go`, executa `go test`, apaga. O mesmo mecanismo foi usado para os 10 hashes abaixo, com um segundo test body para os 5 triggers `manual`. O arquivo temporário não existe na árvore (confirmado).

Commit base: `9167f8ad6ed0544588092cf16c6c8a6a3ebcb7f6` · `var Version = "11.2.0"` (`internal/version/version.go`). Binário: `trackfw 11.2.0` (`/opt/homebrew/bin/trackfw`). Binário e árvore de código concordam: `shasum -a 256` do template gerado pelo binário via `trackfw update` = `aadf2cb1b4eac30dd2fe3a94f55538b4864ff826a5eaba59230975ee96cca996` (idêntico ao hash da fonte).

**Direção para ML-2A:**
- Hashes de consumidor (6) devem **MUDAR** após ML-1A. ML-2A emparelha a mudança com asserções de conteúdo (ausência de `go install`, presença de `releases/download/v`). Hash mudado isolado não garante AC1/AC3.
- Hashes de produtor (4) devem **PERMANECER** idênticos após ML-1A. Produtores não embarcam número de versão — check é incondicional.

| template | trigger | SHA-256 pré-fix | deve mudar? |
|---|---|---|---|
| `trackfw-validate.yml` consumer | `pull_request` | `aadf2cb1b4eac30dd2fe3a94f55538b4864ff826a5eaba59230975ee96cca996` | SIM (AC1) |
| `trackfw-validate.yml` consumer | `manual` | `bf7942d24b84045c2aa9b81f4fc302ecfc0521728a65d0c2c1ff4ac261c1e357` | SIM (AC1) |
| `trackfw-validate.yml` producer | `pull_request` | `824938490c0748adc02374cf60ee29e369267cb64a9e63035abb475f97f76b61` | NÃO (AC6) |
| `trackfw-validate.yml` producer | `manual` | `c6d21639633cdeb1d0ba11febe1341f5aaafedd7cba629b45e00dc7d0baa0f65` | NÃO (AC6) |
| `trackfw-gate.yml` consumer | `pull_request` | `038321ebedf876793a40e7f58c4372f1af9cd7d3ec2c175604e1134f9e48b6ef` | SIM (AC3) |
| `trackfw-gate.yml` consumer | `manual` | `810b77f21338777a96a0683c8f279af8d00a4694366bdc9fdaf1efc584939802` | SIM (AC3) |
| `trackfw-gate.yml` producer | `pull_request` | `017ab0256086924825555ea3af01da498310f8b6382cdb38f529315b227593db` | NÃO (AC6) |
| `trackfw-gate.yml` producer | `manual` | `bb8439421c28e83420d24786d7d0499b82f6c6557fe4c125161007edf4049166` | NÃO (AC6) |
| `.gitlab-ci-trackfw.yml` | `pull_request` | `ca4f636c3b38bbf409f7d8bea8d4706ecb62bda0947c4335d457aa1481a659c7` | SIM (AC3) |
| `.gitlab-ci-trackfw.yml` | `manual` | `cb0be674d8b83a4009d5297c3359d7e7c8821908d62f8e0e46ab81d51ec1c598` | SIM (AC3) |

### Test bodies para reprodução

**PR variants** (corresponde ao `dump_go` da parity script):
```go
package generators

import (
    "os"
    "testing"
)

func TestZZDumpCIWorkflowPinParity(t *testing.T) {
    dest := "/path/to/output" // substituir pelo destino desejado
    if err := os.WriteFile(dest+"/gh_go_consumer.yml", []byte(buildGitHubActionsWorkflowContent(false)), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/gh_go_producer.yml", []byte(buildGitHubActionsWorkflowContent(true)), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/gl_go.yml", []byte(buildGitLabCIWorkflowContent(Config{})), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/dv_go_consumer.yml", []byte(BuildDiscoverGitHubActionsWorkflowContent(false)), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/dv_go_producer.yml", []byte(BuildDiscoverGitHubActionsWorkflowContent(true)), 0644); err != nil {
        t.Fatal(err)
    }
}
```

**Manual variants** (5 arquivos adicionais):
```go
func TestZZDumpCIWorkflowManualTrigger(t *testing.T) {
    dest := "/path/to/output" // substituir pelo destino desejado
    if err := os.WriteFile(dest+"/dv_go_consumer_manual.yml", []byte(BuildDiscoverGitHubActionsWorkflowContentWithTrigger(false, "manual")), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/dv_go_producer_manual.yml", []byte(BuildDiscoverGitHubActionsWorkflowContentWithTrigger(true, "manual")), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/gh_go_consumer_manual.yml", []byte(buildGitHubActionsWorkflowContentWithTrigger(false, "manual")), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/gh_go_producer_manual.yml", []byte(buildGitHubActionsWorkflowContentWithTrigger(true, "manual")), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dest+"/gl_go_manual.yml", []byte(buildGitLabCIWorkflowContent(Config{CITrigger: "manual"})), 0644); err != nil {
        t.Fatal(err)
    }
}
```

Ambos os test bodies foram escritos em `internal/generators/zz_dump_ci_workflow_pin_parity_test.go` durante esta sessão e removidos após execução. O arquivo não existe na árvore (confirmado: `ls internal/generators/zz_dump_*` → no matches).

---

## 4. Residual Declarado

1. **Sem assinatura de artefato.** `signs:` ausente do `.goreleaser.yaml`. `checksums.txt` garante integridade de transporte; não garante autenticidade contra comprometimento de credenciais de write access. `.immutable` = `false`; assets mutáveis via `gh release upload --clobber`. Ação necessária em wave futura se o projeto adotar Sigstore/cosign.

2. **Consumidores que não rodam `trackfw update` retêm o template quebrado.** `trackfw doctor` surfaça o achado; o update é opt-in. Residual de design.

3. **O job id `governance-go-install` descreve o mecanismo antigo.** A ADR documenta que o nome é contrato de `required_status_checks` e não pode ser renomeado (ADR decision 3). O nome fica tecnicamente incorreto após ML-1A. Aceito.

4. **Validação de `version.Version` no lado Go (Go-side write-site validator) não prescrita.** A defesa natural é que `version.Version` vem de ldflags sobre uma tag git (`-X .../version.Version={{.Version}}`), e o parser semver do GoReleaser já rejeita tags malformadas antes de emitir o binário; `install.sh` rejeita valores que chegam como `TRACKFW_VERSION` via `case` (medido: 9 vetores, incluindo `9.9.9-stub` rc=1). P2 remove a interpolação Go da URL — só o bloco `env:` do YAML interpola `version.Version` diretamente. Uma validação Go em `writeCIWorkflow`/`refreshDiscoverGitHubActionsWorkflowIfPresent`/`generateCIWorkflow` quebraria `TestWriteCIWorkflow_VersionNotHardcoded` (discover_test.go:825), que stuba `version.Version = "9.9.9-stub"` — rejeitado pelo mesmo regex que `install.sh` usa. Essa validação exigiria alterar o stub (e os 3 write-site files não constam do ML-1A). O residual aceito é: um binário de desenvolvimento compilado localmente com `version.Version` forjado pode emitir YAML inválido, mas o CI do consumidor rejeitará no `install.sh` antes de executar o binário. Fora do escopo desta REQ.
