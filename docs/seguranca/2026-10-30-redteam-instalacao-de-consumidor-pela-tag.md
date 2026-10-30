# Red Team — Instalação de Consumidor pela Tag

> Roadmap: ROADMAP-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md
> REQ: REQ-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md
> ADR: ADR-2026-10-30-go-install-nao-e-canal-de-instalacao-do-trackfw.md
> Issue de origem: #637 · Wave 2 ML-2A
> Hades (security) — 2026-10-30

Branch: `fix/go-install-module-path-sufixo-de-major`
Commits auditados: `b6c1a3d8` (ML-1A + ML-1B, único commit de produto após `9167f8ad`).

---

## 1. Tabela de Mutações

Para cada mutação: arquivo editado, resultado de `go test ./internal/generators/ ./internal/discover/ -count=1`
e `bash scripts/check-ci-workflow-pin-parity.sh`, e o veredito.

| ID | Mutação | Arquivo | Gate Go | Gate Script | Veredito |
|---|---|---|---|---|---|
| M1 | `version.Version` → `"11.2.0"` literal em `scaffold.go:1796` (gate consumer) | `scaffold.go` | PASSA | PASSA | NÃO PEGA |
| M2 | env block + install step movidos para YAML comment em `scaffold_doctor.go` | `scaffold_doctor.go` | PASSA | FALHA¹ | NÃO PEGA (mecanismo primário) |
| M3 | `go install -v github.com/kgsaran/trackfw/cmd/trackfw@v11.2.0` no validate consumer | `scaffold_doctor.go` | `NoGoInstall` PASSA; `VersionPin` FALHA | FALHA | PEGA |
| M4 | Espaço extra em `go build` no template de produtor | `scaffold_doctor.go` | `ProducerHashesUnchanged` FALHA | — | PEGA |
| M5 (revert go install) | `go install ...@v11.2.0` no validate consumer (regressão direta) | `scaffold_doctor.go` | `NoGoInstall` FALHA + `VersionPin` FALHA | FALHA | PEGA |
| M6 (revert releases/latest) | `releases/latest/download/install.sh` no gate consumer | `scaffold.go` | `NoReleasesLatest` FALHA | FALHA | PEGA |
| M7 (URL literal) | `releases/download/v11.2.0/install.sh` (sem `${TRACKFW_VERSION}`) | qualquer consumer | `VersionPin` FALHA² | FALHA | PEGA |
| M8 (remove TRACKFW_VERSION) | remover bloco `env: TRACKFW_VERSION` do validate consumer | `scaffold_doctor.go` | `VersionPin` FALHA | FALHA | PEGA |

¹ O script falha na falsificação vetor (5) com mensagem divergente ("URL pinada ... ausente" em vez de "releases/latest presente"), não no check positivo `pin-parity/*/consumer-install-pin`.

² `TestConsumerInstall_VersionPinEqualsVersionDotVersion` verifica a presença da string literal `v${TRACKFW_VERSION}` na URL; uma URL com `v11.2.0` não a contém.

---

## 2. Achados

### F1 — MEDIUM: versão literal `"11.2.0"` em `scaffold.go` não detectada hoje

**Superfícies**: `scaffold.go:1796` (gate consumer GitHub Actions) e `scaffold.go:1848` (GitLab consumer). Ambas usam `"` + version.Version + `"` correto hoje; o achado é sobre falta de trava que detectaria uma regressão.

**Evidência (M1):**
```
Mutation: substituir version.Version → "11.2.0" literal em scaffold.go:1796
go test ./internal/generators/ ./internal/discover/ -count=1 → ok / ok
bash scripts/check-ci-workflow-pin-parity.sh → 25 cenários OK
```

**Por que não pega**: `TestConsumerInstall_VersionPinEqualsVersionDotVersion` usa
`fmt.Sprintf('TRACKFW_VERSION: "%s"', version.Version)` como valor esperado. Com
`version.Version == "11.2.0"` e o literal `"11.2.0"` no template, os dois concordam.
`TestWriteCIWorkflow_VersionNotHardcoded` em `discover_test.go` faz o stub correto
(`version.Version = "9.9.9-stub"`) mas só cobre `scaffold_doctor.go`
(`BuildDiscoverGitHubActionsWorkflowContent`). Os builders de `scaffold.go`
(`buildGitHubActionsWorkflowContent(false)`, `buildGitLabCIWorkflowContent`) não têm
equivalente. O script lê `GO_VERSION` de `internal/version/version.go`, que também seria
`"11.2.0"` — também não pega.

A regressão sobreviveria silenciosamente até o próximo bump de versão.

**Correção** (`internal/generators/consumer_install_test.go`):
Adicionar um teste que faça stub de `version.Version = "9.9.9-stub"` com `t.Cleanup`
e chame `allConsumerBuilderOutputs()`, verificando que cada output contém
`TRACKFW_VERSION: "9.9.9-stub"` — o mesmo padrão de `TestWriteCIWorkflow_VersionNotHardcoded`,
estendido aos outros dois builders.

### F2 — LOW: asserções positivas não filtram comentários YAML — bypass por comentário passa todos os checks positivos

**Superfície**: qualquer template de consumidor — os três builders cobertos.

**Evidência (M2):**
Template de `scaffold_doctor.go` consumer com env block e install step movidos para
comentários YAML:
```yaml
    steps:
      - uses: actions/checkout@v7
        # env:
        #   TRACKFW_VERSION: "11.2.0"
        # - name: Install trackfw
        #   run: curl -sSfL "https://github.com/kgsaran/trackfw/releases/download/v${TRACKFW_VERSION}/install.sh" | sh

      - run: trackfw validate
```

Resultados:
```
TestConsumerInstall_VersionPinEqualsVersionDotVersion → PASS
TestConsumerInstall_ManualTriggerHasWorkflowDispatch  → PASS
TestConsumerInstall_NoGoInstall_NoBuildersConsumers   → PASS
TestConsumerInstall_NoReleasesLatest                  → PASS
TestConsumerInstall_StaleFixture_DoctorDetects_UpdateFixes → PASS
pin-parity/discover-workflow/consumer-install-pin    → OK
pin-parity/discover-workflow/consumer-version-pin    → OK
```

O script falhou, mas não por este check: o vetor de falsificação (5) esperava
"releases/latest presente" e recebeu "URL pinada ... ausente" — mensagens divergentes.
O check positivo `consumer-install-pin` passou porque `grep -qF
'releases/download/v${TRACKFW_VERSION}/install.sh' "$file"` pesquisa o arquivo bruto,
incluindo linhas de comentário.

**Por que a falha indireta do script não é proteção suficiente**: um mantenedor que
corrigisse apenas a mensagem esperada no `assert_check_fails` do vetor (5) (para
"URL pinada ... ausente") tornaria o script verde novamente, enquanto o template
continuaria sem instalar o trackfw.

**Correções** (dois arquivos):
- `internal/generators/consumer_install_test.go`: nas asserções positivas de
  `TestConsumerInstall_VersionPinEqualsVersionDotVersion`, usar `nonCommentLines()` antes
  de `strings.Contains`:
  ```go
  nc := nonCommentLines(trigger.content)
  if !strings.Contains(nc, wantEnv) { ... }
  if !strings.Contains(nc, wantURL) { ... }
  ```
- `scripts/check-ci-workflow-pin-parity.sh`, função `check_consumer_no_latest`: mover o
  `grep -qF 'releases/download/v${TRACKFW_VERSION}/install.sh'` para usar `$stripped` em
  vez de `$file`:
  ```bash
  if ! echo "$stripped" | grep -qF 'releases/download/v${TRACKFW_VERSION}/install.sh'; then
  ```
  (e `check_version_pin` analogamente — `grep -m1 'TRACKFW_VERSION: "'` no `$stripped`).

### F3 — LOW: `go install -v` com flag entre `install` e o module path escapa `TestConsumerInstall_NoGoInstall_NoBuildersConsumers`

**Evidência (M3):**
```
Template com: run: go install -v github.com/kgsaran/trackfw/cmd/trackfw@v11.2.0
TestConsumerInstall_NoGoInstall_NoBuildersConsumers → PASS
TestConsumerInstall_VersionPinEqualsVersionDotVersion → FAIL (URL ausente)
check-ci-workflow-pin-parity.sh → FAIL (URL ausente)
```

O check usa `strings.Contains(nc, "go install github.com/kgsaran/trackfw")` — o flag
`-v` quebra a adjacência. Os outros dois gates (VersionPin e o script) ainda pegam porque
a URL pinada está ausente. Severidade baixa: o bypass de um gate é coberto por dois
outros.

**Correção** (`internal/generators/consumer_install_test.go`): substituir a verificação
de string exata por regexp:
```go
goModuleRe := regexp.MustCompile(`\bgo\s+(install|run|get)\s+(?:-\S+\s+)*github\.com/kgsaran/trackfw`)
if goModuleRe.MatchString(nc) { ... }
```

### F4 — INFORMACIONAL: fixture AC5 cobre apenas o PR-trigger

**Evidência**: `staleValidateYML` (hash `aadf2cb1...`) é a variante `pull_request`.
O hash de `manual` (`bf7942d2...`) está documentado no Apêndice A da Wave 0, mas não
há arm correspondente no teste. A transição binária (old→new) foi verificada para o
PR-trigger:

```
Fixture stale SHA-256: aadf2cb1...cca996  (matches)
trackfw-postfix doctor → 1 scaffold-divergent (.github/workflows/trackfw-validate.yml)
trackfw-postfix update → ✓ CI workflow (discover) atualizado
Novo SHA-256: 55f3648f...cad5ddc
grep TRACKFW_VERSION → TRACKFW_VERSION: "11.2.0"
grep releases/download → releases/download/v${TRACKFW_VERSION}/install.sh
trackfw-postfix doctor (após) → no mismatches found
```

Consumidores com `ci_trigger: manual` não têm este caminho testado em fixture.

**Correção** (`internal/generators/consumer_install_test.go`): adicionar arm 2 ao
`TestConsumerInstall_StaleFixture_DoctorDetects_UpdateFixes` com `staleValidateManualYML`
(hash `bf7942d2...`) e `ci_trigger: manual` no `trackfw.yaml` do fixture.

---

## 3. Decorativo? Por mutação que reprova cada teste

| Teste | Mutação que o reprova | Afirmação do teste (docstring) |
|---|---|---|
| `TestConsumerInstall_NoGoInstall_NoBuildersConsumers` | M5 (go install @v... no template) | AC2: nenhum builder consumer emite `go install/run/get github.com/kgsaran/trackfw` em linha não-comentário |
| `TestConsumerInstall_NoReleasesLatest` | M6 (releases/latest no template) | AC3: nenhum builder consumer referencia `releases/latest` em linha não-comentário |
| `TestConsumerInstall_VersionPinEqualsVersionDotVersion` | M7 (URL com versão literal), M3 (go install sem URL), M8 (sem env block) | AC1+AC3: TRACKFW_VERSION = version.Version; URL usa `${TRACKFW_VERSION}` (não literal) |
| `TestConsumerInstall_ManualTriggerHasWorkflowDispatch` | regressão de trigger: manual → pull_request no output | AC2 extension: variant manual contém workflow_dispatch (GH) ou web pipeline (GL) |
| `TestConsumerInstall_ProducerHashesUnchanged` | M4 (espaço extra no `go build` do producer) | AC6: SHA-256 dos 4 templates de produtor bate com golden pré-fix do Apêndice A |
| `TestConsumerInstall_StaleFixture_DoctorDetects_UpdateFixes` | (a) mudar staleValidateYML → t.Fatalf no hash guard; (b) update no-op → doctor pós-update ainda divergente; (c) doctor always-pass → foundDivergent=false, t.Fatalf | AC5: doctor detecta validate.yml pré-fix como scaffold-divergent; update corrige; 0 divergências depois |

Nenhum teste é decorativo. Cada um reprova com uma mutação concreta que a Regra Dura de
Reconciliação exigiria.

Observação: `TestConsumerInstall_NoGoInstall_NoBuildersConsumers` tem gap de adjacência
(F3). A afirmação principal ainda é verdadeira — o teste reprova M5 (regressão direta).
O gap é que ele não reprova M3 (flag como filtro), coberto pelos outros dois gates.

---

## 4. Veredito por Prescrição

| P | Prescrição | Veredito | Evidência |
|---|---|---|---|
| P1 | `check_consumer_no_latest` substitui `check_discover_pin`; no-op guard com `if cmp -s`; `go test ./internal/discover/`; contagem de cenários ≥ anterior (21) | APROVA | `check_consumer_no_latest` em 3 chamadas `run_check`. No-op guard testado: sed com padrão inerte → `FAIL consumer-latest-noop`. Script mostra "25 cenários OK" > 21. `go test ./internal/discover/` nos gates do roadmap (auditado). |
| P2 | URL usa `${TRACKFW_VERSION}` entre aspas duplas nos 3 templates | APROVA | `scaffold_doctor.go:158`, `scaffold.go:1802`, `scaffold.go:1851` têm `"https://github.com/kgsaran/trackfw/releases/download/v${TRACKFW_VERSION}/install.sh"`. M7 reprova se literalizado (VersionPin). |
| P3 | `timeout-minutes: 10` no validate consumer | APROVA | `scaffold_doctor.go:151` tem `timeout-minutes: 10`. `check_timeout_minutes` para `dv_go_consumer.yml`: `pin-parity/discover-workflow/consumer-timeout-minutes → OK`. |
| P4 | Teste AC2 cobre 3 builders, ambos triggers, não-comentário, rejeita `go install/run/get`; não rejeita produtor | APROVA COM RESSALVAS | `allConsumerBuilderOutputs()` cobre 3 builders. `nonCommentLines()` implementado. Produtor protegido por `isProducer=true`. Slash-commands excluídos (fora do NEG SCOPE). Gap F3: `go install -v` com flag não capturado por `NoGoInstall`; outros 2 gates cobrem. |
| P5 | Asserções positivas: TRACKFW_VERSION = version.Version; URL usa `${TRACKFW_VERSION}`; ausência de `releases/latest` em step run | APROVA COM RESSALVAS | `TestConsumerInstall_VersionPinEqualsVersionDotVersion` verifica as duas primeiras. `TestConsumerInstall_NoReleasesLatest` verifica a terceira. Gap F1: builders de `scaffold.go` não têm stub de versão — literal `"11.2.0"` passa hoje. Gap F2: asserções positivas no conteúdo bruto (comentários não filtrados). |
| P6 | Teste AC5 usa fixture byte-idêntica ao SHA-256 `aadf2cb1...cca996`; hash guard no início | APROVA COM RESSALVAS | Hash guard: `staleHash != wantHash → t.Fatalf`. Transição binária: `/opt/homebrew/bin/trackfw 11.2.0` (pré-fix) gerou arquivo com SHA-256 `aadf2cb1...cca996` (verificado com `sha256sum`). Post-fix binary detecta, corrige, dr confirma 0 divergências. Gap F4: apenas PR-trigger; manual não testado. |
| P7 | Hashes de produtor (Apêndice A) inalterados | APROVA | `TestConsumerInstall_ProducerHashesUnchanged` reprova com qualquer mudança de byte (M4: espaço extra → falha imediata com hash divergente). Os 4 golden hashes do Apêndice A conferem com `BuildDiscoverGitHubActionsWorkflowContent(true)`, `buildGitHubActionsWorkflowContent(true)` e variantes manual. |
| P8 | Remover `go install` dos 6 arquivos de doc | APROVA | `grep -rn "cmd/trackfw@" README.md site docs/visao-projeto --include=*.md` → sem resultado. Scan mais amplo `grep -rn 'go install github.com/kgsaran' --include=*.md .` → único resultado em `scripts/testdata/roadmap-barrier-corpus-snapshot/` (histórico de corpus, não documentação ativa). |

---

## 5. Veredito Geral

**APROVA COM RESSALVAS.**

Os quatro caminhos de regressão primários — `go install`, `releases/latest`, URL com versão
literal desacoplada de TRACKFW_VERSION, e perda de TRACKFW_VERSION — são bloqueados por
pelo menos um gate em cada caso. Nenhum caminho silencioso de regressão foi encontrado.

As ressalvas F1 e F2 são falhas de cobertura de teste que expõem uma janela de regressão
silenciosa para os builders de `scaffold.go` (F1) e qualquer template via bypass de
comentário (F2). Ambas exigem correção no mesmo PR antes do merge, pela Regra Dura de
Causa Raiz.

---

## 6. Residual Declarado

Residuais herdados da Wave 0 que permanecem:

1. **Assets de release mutáveis** (`.immutable=false`, sem Sigstore). Tag pinada limita o
   blast radius ao consumidor que usa a versão afetada, mas não garante imutabilidade.
   Comando: `gh release upload --clobber` continua disponível para quem tem `write` access.
   Ação: wave futura se o projeto adotar cosign/Sigstore.

2. **Consumidores sem `trackfw update` retêm o template quebrado.** Doctor expõe o achado;
   update é opt-in. Residual de design.

3. **Job id `governance-go-install` descreve o mecanismo antigo.** ADR D3 aceitou: o id
   é contrato de `required_status_checks`.

4. **`version.Version` sem validação no lado Go.** A defesa primária é o GoReleaser
   + ldflags + `install.sh case`. P2 remove a interpolação Go da URL. Fora do escopo desta REQ.

Residuais novos identificados neste red team:

5. **Bypass de comentário YAML (F2): bloqueio indireto.** O único obstáculo hoje é o
   `assert_check_fails` do vetor (5) no script, que falha por mensagem divergente. Um
   mantenedor que corrija apenas a mensagem esperada torna o script verde enquanto o
   template permanece inerte. Fix em F2 resolve.

6. **Stub de versão ausente para builders de `scaffold.go` (F1).** A janela fecha no
   próximo bump de versão, mas existe silenciosamente entre bumps. Fix em F1 resolve.

7. **AC5 cobre apenas PR-trigger (F4).** Consumidores com `ci_trigger: manual` têm o
   mecanismo funcionando (testado via binary transition), mas sem cobertura de fixture
   in-process.
