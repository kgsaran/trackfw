# Wave 0 — Threat Model: gatilho configurável dos workflows gerados (`ci_trigger`)

> Roadmap: ROADMAP-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md
> REQ: REQ-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md
> Data: 2026-10-24 | Agente: hades-tf

---

## Seção 1 — Enumeração: completude da superfície

### 1.0 Evidência de fechamento da enumeração

Grep de `pull_request|merge_requests` em `internal/` (excluindo `corpus-pre-fix/`, excluindo testes):

```
internal/forge/adapter.go:97            — URL de MR do GitLab (não é template CI)
internal/generators/scaffold.go:1721    — pull_request trigger (produtor GitHub)
internal/generators/scaffold.go:1733    — comentário inline sobre pull_request events
internal/generators/scaffold.go:1775    — pull_request trigger (consumidor GitHub)
internal/generators/scaffold.go:1815    — merge_requests trigger (GitLab)
internal/generators/scaffold_doctor.go:64  — comentário de gatilho (produtor discover)
internal/generators/scaffold_doctor.go:69  — pull_request trigger (produtor discover)
internal/generators/scaffold_doctor.go:105 — comentário de gatilho (consumidor discover)
internal/generators/scaffold_doctor.go:110 — pull_request trigger (consumidor discover)
```

Total: 9 hits. `adapter.go:97` é URL de MR, não template CI. Os 8 restantes mapeiam para as 3 funções da §1.1 e confirma que não há quarto sítio de escrita. Testes (excluídos acima): `discover_workflow_trigger_test.go:51-52` fixa a asserção `pull_request:` existente e será afetado por ML-1A.

### 1.1 As três funções geradoras de template (com arquivo:linha)

| Função | Arquivo:linha | Assinatura atual | Trigger hardcoded |
|---|---|---|---|
| `buildGitHubActionsWorkflowContent` | `scaffold.go:1717` | `(isProducer bool)` | `pull_request: branches: [main]` (produtor:1721-1722, consumidor:1775-1776) |
| `buildGitLabCIWorkflowContent` | `scaffold.go:1801` | `(_ Config)` — ignora Config | `only:\n  - merge_requests` (1814-1815) |
| `BuildDiscoverGitHubActionsWorkflowContent` | `scaffold_doctor.go:60` | `(isProducer bool)` | `push: branches: [main]` + `pull_request:` (produtor:64-70, consumidor:104-110) |

`ci_trigger` / `CITrigger` não existe em lugar nenhum de `internal/` hoje (grep retornou vazio).

### 1.2 Matriz completa: template × variante × plataforma × comando × doctor

| Template gerado | Variante | Plataforma | Gerador de escrita | Atualizado por | Doctor compara (arquivo:linha) |
|---|---|---|---|---|---|
| `trackfw-gate.yml` | produtor | GitHub | `init`, `update` | `generateGitHubActionsWorkflow` (scaffold.go:1855) | scaffold_doctor.go:319 |
| `trackfw-gate.yml` | consumidor | GitHub | `init`, `update` | idem | scaffold_doctor.go:319 |
| `.gitlab-ci-trackfw.yml` | único | GitLab | `init`, `update` | `generateGitLabCIWorkflow` (scaffold.go:1872) | scaffold_doctor.go:337 |
| `trackfw-validate.yml` | produtor | GitHub | `discover --init`, `update` (refresh) | `refreshDiscoverGitHubActionsWorkflowIfPresent` (update.go:2829→2852) e `discover.go:392` | scaffold_doctor.go:403 |
| `trackfw-validate.yml` | consumidor | GitHub | `discover --init`, `update` (refresh) | idem | scaffold_doctor.go:403 |

### 1.3 Config bridge — onde `CITrigger` deve morar

```
trackfw.yaml  →  config.Config.UpdateConfig.CI (config.go:102, lido em config.go:542-543)
              →  generators.Config.CI (scaffold.go:59)
              via loadUpdateConfig() (update.go:31-40): bridge explícita, campo a campo
```

`CITrigger` precisa ser adicionado a:
- `config.Config.UpdateConfig` (config.go) — chave YAML `ci_trigger:` (irmã de `ci:`)
- `generators.Config` (scaffold.go) — novo campo
- `loadUpdateConfig()` (update.go:33-38) — linha `CITrigger: u.CITrigger`

O caminho `--targets ci-workflow` (update.go:2955-2966) usa o mesmo `cfg` de `loadUpdateConfig()` — herda o campo sem mudança estrutural adicional.

### 1.4 Caminhos que NÃO passam por loadUpdateConfig()

| Sítio | Arquivo:linha | Recebe Config? | Motivo da lacuna |
|---|---|---|---|
| `refreshDiscoverGitHubActionsWorkflowIfPresent` | update.go:2829 | não — só `rootDir string` | lê config.Load().Update internamente; deve ler também `CITrigger` |
| `discover.go:392` | discover.go:392 | não — só `isProducer bool` | chama `BuildDiscoverGitHubActionsWorkflowContent` direto; precisa de config.Load().Update.CITrigger |
| `trackfw init` wizard | init.go:353-363 | sim — mas construído do wizard, não do trackfw.yaml | ver Seção 1.5 |

### 1.5 Lacuna estrutural de `trackfw init`

`init` constrói `generators.Config` do wizard/flags (init.go:353-363) — nunca lê o trackfw.yaml existente. O template de criação (scaffold.go:948) inclui `ci: %s` mas **não inclui `ci_trigger:`**. Consequência:

- Se `trackfw.yaml` já existe e tem `ci_trigger: manual`: o writer faz merge textual (scaffold.go:1001-1033) e **preserva** o campo existente no arquivo — mas o `generators.Config` passado para `generateGitHubActionsWorkflow` não tem `CITrigger` → a workflow é regenerada sem a variante manual.
- Se `trackfw.yaml` não existe: o template cria o arquivo sem `ci_trigger:` → campo ausente.

AC1 exige que `init` respeite `ci_trigger: manual`. Isso requer escolha de implementação:
- (a) wizard adiciona pergunta sobre `ci_trigger`
- (b) flag `--ci-trigger` no `init`
- (c) `init` lê trackfw.yaml pré-existente no início e popula `generators.Config.CITrigger`

Hades declara o gap; não decide a opção.

### 1.6 Sítios periféricos com claims sobre o trigger

| Arquivo:linha | Claim | Impacto com `manual` |
|---|---|---|
| `claudemd.go:158` | "runs `trackfw validate` on every pull request to main." | False — CLAUDE.md/AGENTS.md/GEMINI.md gerados mentem ao agente |
| `claudemd.go:160` | "runs `trackfw validate` on every merge request." | False — mesmo problema no GitLab |
| `discover_workflow_trigger_test.go:51-52` | asserta `pull_request:` DEVE existir em `BuildDiscoverGitHubActionsWorkflowContent` | **Quebra** ao adicionar variante manual sem atualizar |
| `ci_workflow_version_pin_test.go:17,32,57-62` | asserta pin de versão | Quebra se assinatura das funções mudar |
| `discover_test.go:866,892,964,965` | byte-exact contra `BuildDiscoverGitHubActionsWorkflowContent` | **Quebra** ao qualquer mudança de conteúdo ou assinatura |
| `scripts/check-workflow-yaml.py:111` | lê `pr-closing-keyword.yml` (não trackfw-gate.yml) | Não afetado por esta REQ |
| `scripts/check-ci-workflow-binary-provenance.py:390ff` | corpus de teste interno com `on: [pull_request]` | Corpus fechado; não valida o trigger gerado em produção |

`generateClaudeMD` tem um único caller no produto: `scaffold.go:173` (`Scaffold(cfg)`). O `cfg` já chega com todos os campos de `Config`. Quando `CITrigger` for adicionado a `Config`, `generateClaudeMD` o recebe sem necessidade de novo caller. A condicional nas linhas 158/160 funciona via `cfg.CITrigger` — mas o `Config` passado de `init.go` pode não ter `CITrigger` (ver §1.5).

Corpus congelado — não tocar: `internal/pathguard/testdata/corpus-pre-fix/` — contém snapshots de `.go.txt`; `scaffold.go.txt:2123,2177,2217` contém `pull_request` e `merge_requests`. Qualquer toque quebraria testes de contenção de escrita da pathguard.

### 1.7 Regras do doctor e do validate sobre o workflow de governança (ADR-2026-09-29)

O `trackfw validate` **não tem nenhuma regra que verifique conteúdo ou trigger** do workflow de CI. Todas as verificações de conteúdo de workflow são do `trackfw doctor`. Regras medidas:

| Regra | Tipo | Código | Condição de disparo |
|---|---|---|---|
| `DoctorScaffoldMissing` | `doctor.go:68` | `scaffold_doctor.go:319` (gate), `:337` (gitlab), `:402` (validate — mas nunca emite scaffold-missing para validate.yml: bloco presença-em-disco em :402 pula a check de ausência; o row em :403 só chega quando o arquivo existe) | Arquivo ausente + `cfg.CI` configurado (para gate/gitlab); validate.yml: presença-em-disco como guarda — DoctorScaffoldMissing nunca dispara para validate.yml por esse caminho |
| `DoctorScaffoldDivergent` | `doctor.go:62` | via `checkScaffoldArtifact` nas mesmas linhas | Arquivo presente mas conteúdo ≠ template do binário instalado |
| `DoctorScaffoldWorkflowDuplicated` | `doctor.go:123` | `scaffold_doctor.go:368-386` | gate.yml **e** validate.yml ambos presentes como arquivos regulares |
| `DoctorRequiredStatusChecksMissing` | `doctor.go:88` | `doctor_remote.go:150-196` | `--remote` + admin creds + sem required checks na branch protection |

A sentença "Manual continua sendo 'exists workflow'" (REQ negative scope) significa: `DoctorScaffoldMissing` não dispara para um workflow com `workflow_dispatch` — o arquivo existe. Mas `DoctorScaffoldDivergent` **dispara hoje** para qualquer workflow com trigger diferente do template, porque `checkScaffoldArtifact` faz comparação byte-a-byte contra `buildGitHubActionsWorkflowContent(isProducer)` (scaffold_doctor.go:319) — que hoje não aceita `ciTrigger`. Esse é o defeito pré-existente que AC3 quer fechar; não é ameaça nova introduzida por esta REQ.

### 1.8 Files affected recomendados para ML-1A

```
# Produto
internal/config/config.go
  - novo campo CITrigger string em UpdateConfig
  - campos auxiliares CITriggerInvalid bool e CITriggerRawValue string
  - parse de ci_trigger: checagem de presença separada + tipo + enum
  - violation emitida por validate/update, não pelo parser (ver T1)

internal/generators/scaffold.go
  - generators.Config: novo campo CITrigger string
  - buildGitHubActionsWorkflowContent: nova assinatura (isProducer bool, ciTrigger string)
  - buildGitLabCIWorkflowContent: usa cfg.CITrigger (parâmetro Config já existe, usado com _)
  - generateGitHubActionsWorkflow: passa cfg.CITrigger para buildGitHubActionsWorkflowContent

internal/generators/scaffold_doctor.go
  - BuildDiscoverGitHubActionsWorkflowContent: nova assinatura (isProducer bool, ciTrigger string)
  - Doctor scaffold_doctor.go:319: passa ciTrigger de cfg para buildGitHubActionsWorkflowContent
  - Doctor scaffold_doctor.go:337: usa cfg.CITrigger (já tem cfg)
  - Doctor scaffold_doctor.go:403: passa ciTrigger de cfg para BuildDiscoverGitHubActionsWorkflowContent

internal/generators/update.go
  - loadUpdateConfig(): adiciona CITrigger: u.CITrigger
  - refreshDiscoverGitHubActionsWorkflowIfPresent: lê config.Load().Update.CITrigger; passa para
    BuildDiscoverGitHubActionsWorkflowContent

internal/generators/claudemd.go
  - linha 158: condicionada a cfg.CITrigger; variante manual omite "every pull request"
  - linha 160: idem para GitLab

internal/discover/discover.go
  - linha 392: lê config.Load().Update.CITrigger; passa para BuildDiscoverGitHubActionsWorkflowContent

internal/commands/init.go
  - decisão do arquiteto (a/b/c da Seção 1.5)

internal/validator/validator.go
  - novo WARNING quando CITrigger == "manual" (veja T3)

# Testes
internal/generators/ci_workflow_version_pin_test.go
  - atualizar chamadas para novas assinaturas
internal/generators/discover_workflow_trigger_test.go
  - atualizar: manter asserção anti-double-run; adicionar braço manual
internal/discover/discover_test.go
  - atualizar byte-exact checks para nova assinatura
  - ver nota sobre AC2 (Seção 4): testes contra a própria função não fixam a variante pull_request

# Documentação
docs/cli-parity.md (ML-2A, AC5)
```

---

## Seção 2 — Modelo de ameaça

O adversário é o **implementador apressado** e o **arquiteto otimista** — não um atacante externo.

### T1 — `stringVal`: sequence e mapping escapam silenciosamente; escalares não

**Medição direta** (programa com mirror exato de `normalizeNode` config.go:596-612 e `stringVal` config.go:616-623, rodado via `go run` no módulo trackfw, 9 vetores, usando chave `ci:` como proxy — chave `ci_trigger:` não existe ainda; o mecanismo de parse é idêntico para ambas. Vault note: `yaml-stringval-sequence-mapping-silent-drop-ci-trigger-2026-10-24`):

| Valor YAML | `normalizeNode` retorna | `stringVal` ok | Comportamento |
|---|---|---|---|
| `ci: github-actions` | `"github-actions"` (string) | true | ✓ |
| `ci: true` | `"true"` (string — raw n.Value) | true | capturado pelo enum check |
| `ci: 1` | `"1"` (string) | true | capturado pelo enum check |
| `ci: ~` | `"~"` (string) | true | capturado pelo enum check |
| `ci: null` | `"null"` (string) | true | capturado pelo enum check |
| `ci: [github-actions]` | `[]interface{}{"github-actions"}` | **false** | **campo ausente — violação AC4** |
| `ci: {a: b}` | `map[string]interface{}{"a":"b"}` | **false** | **campo ausente — violação AC4** |
| `ci:` (bare) | `""` (string — n.Value vazio) | true | `""` não está no enum — capturado |
| `ci: ''` | `""` (string) | true | idem |

`normalizeNode` (config.go:596) retorna `n.Value` (string) para ScalarNode, `[]interface{}` para SequenceNode, `map[string]interface{}` para MappingNode. O doc comment diz "**pre-coercion raw text** in Node.Value" — booleans/ints/nulls chegam como texto bruto, não como tipo Go. O `default: return nil` (config.go:610) só dispara para kinds exóticos nunca presentes em trackfw.yaml. Todos os escalares (incluindo null-tagged `~`/`null`/bare `ci:`) chegam como string e são capturados pelo enum check. **SequenceNode** e **MappingNode** → `stringVal` ok=false → campo ausente → **violação AC4** sem aviso.

**Canal de AC4:** o parser (config.go) não emite violations — é uma função de leitura silenciosa. A violation é emitida pelo **`trackfw validate`** (validator.go) e a recusa pelo **`trackfw update`** (update.go). O padrão para o parser:

```go
if raw, present := m["ci_trigger"]; present {
    if s, ok := raw.(string); ok {
        cfg.Update.CITriggerRawValue = s
        if s == "pull_request" || s == "manual" {
            cfg.Update.CITrigger = s
        } else {
            cfg.Update.CITriggerInvalid = true
        }
    } else {
        cfg.Update.CITriggerInvalid = true // sequence ou mapping
    }
}
```

Para `update`: hoje `update.go:110-111` imprime ⚠ e sai com rc=0 para config inválida. AC4 exige rc≠0 e arquivo original inalterado quando `ci_trigger` tem valor inválido.

### T2 — Injeção YAML via interpolação do valor

Se `buildGitHubActionsWorkflowContent` usa `fmt.Sprintf` com o valor de `ciTrigger`:

```go
fmt.Sprintf("on:\n  %s:\n    branches: [main]\n", ciTrigger)
```

então `ci_trigger: "workflow_dispatch\non: push"` injeta um segundo bloco `on:` no YAML gerado. O resultado é sintaticamente válido. O valor `"workflow_dispatch\non: push"` não está no enum e seria recusado pelo AC4 — mas a defesa em profundidade exige que o gerador nunca interpole o valor mesmo que o verificador de enum seja reescrito.

**Mitigação obrigatória:** use `switch ciTrigger` para selecionar entre dois templates completos e pré-escritos. Nunca interpole `ciTrigger` em string de formato.

### T3 — `ci_trigger: manual` desliga o gate automático de PR — dois vetores

**Vetor A — GitHub sem gate automático:** com `workflow_dispatch`, nenhum gate roda automaticamente em PRs. A documentação GitHub indica que runs de `workflow_dispatch` não são associadas ao check suite de um PR (per search result summary, page not fetched). Sem CI gate automático:
- Com branch protection (required_status_checks): ver T5 para o caso bloqueante.
- Sem branch protection: governance ignorada em todos os PRs.

**Vetor B — `branch_has_wip_roadmap` roda contra `main`, não contra o PR branch:** `validateBranchHasWIPRoadmap` (validator.go:4383) resolve o branch na ordem: (1) `TRACKFW_BRANCH` (validator.go:4384) — não setado pelos templates gerados (grep retornou vazio) → vazio; (2) `git symbolic-ref --short HEAD` (validator.go:4386-4390) — em runner de `workflow_dispatch` sem selecionar branch, a checkout é feita no branch default (`main`), então symbolic-ref retorna `"main"`. Se symbolic-ref falhar (detached HEAD), o fallback é (3) `GITHUB_HEAD_REF` → `CI_COMMIT_REF_NAME` → `GITHUB_REF_NAME` (validator.go:4392-4396), todos retornando `"main"` ou vazios nesse caso. Independente de qual caminho (2) ou (3) vença, `branch` = `"main"`. A verificação em validator.go:4399 (`!strings.HasPrefix(branch, "feat/")` etc.) retorna imediatamente nil → `branch_has_wip_roadmap` **não roda**, passa vacuamente. O PR branch nunca é validado. O PR branch nunca é validado.

**Recomendação Hades:** `trackfw validate` deve emitir **WARNING** (não violation) quando `CITrigger == "manual"`, com texto: "`ci_trigger: manual` — trackfw-gate.yml só dispara manualmente via workflow_dispatch; nenhum gate automático protege PRs. Se `governance-install-script` estiver em required_status_checks, os PRs serão bloqueados permanentemente (veja T5). Sob dispatch do branch default, `branch_has_wip_roadmap` não valida o PR branch." Isso honra a ADR e informa.

O `claudemd.go:158,160` deve emitir texto condicional que reflete o trigger real.

### T4 — ADR-2026-09-29 D2/D4: `validate.yml` fica com `pull_request` quando `manual` configurado

`refreshDiscoverGitHubActionsWorkflowIfPresent` (update.go:2829) e `discover.go:392` chamam `BuildDiscoverGitHubActionsWorkflowContent` sem `ciTrigger`. Se o consumidor tem `ci_trigger: manual`, o `update` regenera `trackfw-gate.yml` com `workflow_dispatch` mas `trackfw-validate.yml` fica com `pull_request` — conteúdo divergente.

Se depois o doctor for atualizado para gerar com `ciTrigger="manual"` mas o refresh não o for, o doctor detecta `scaffold-divergent` para `validate.yml` (comparando disco `pull_request` contra template `workflow_dispatch`) — **falso positivo** até que o refresh seja corrigido.

**Solução obrigatória:** `BuildDiscoverGitHubActionsWorkflowContent` deve aceitar `ciTrigger string`. `refreshDiscoverGitHubActionsWorkflowIfPresent` lê `config.Load().Update.CITrigger` e passa. `discover.go:392` idem.

### T5 — `required_status_checks` apontando para job que nunca roda em PRs

A documentação GitHub indica: quando um required_status_check é configurado para um job que roda num evento diferente de `pull_request`, o check nunca é satisfeito no PR. E que runs de `workflow_dispatch` não aparecem no check suite de PRs (per search result summary, page not fetched from GitHub Docs troubleshooting required status checks).

Se `governance-install-script` está em `required_status_checks` e `ci_trigger: manual`, o PR fica **bloqueado permanentemente** — não "até alguém acionar manualmente", mas bloqueado por um check que nunca aparece no PR.

O `doctor --remote` (`DoctorRequiredStatusChecksMissing`, doctor.go:88, doctor_remote.go:150-196) verifica se há required checks configurados, mas **não** cruza com os triggers do workflow. Não detecta o caso "required check existe mas jamais roda em PRs".

**O produto não pode detectar isso sem `--remote` + credencial admin + lógica nova.** O WARNING de T3 é a única mitigação viável no produto atual.

### T6 — GitLab `when: manual`: semântica diverge do GitHub `workflow_dispatch`

O template GitLab atual usa `only: - merge_requests` (scaffold.go:1814-1815). Para a variante `manual`, há duas opções na mesma job:

**Opção A — Job-level `when: manual`** (fora de `rules:`):
Conforme documentação GitLab ([Control how jobs run](https://docs.gitlab.com/ci/jobs/job_control/)), quando `when: manual` é usado fora de `rules:`, `allow_failure` defaults a **true** → pipeline vai a verde sem que o job rode → gate é silenciosamente opcional — mesmo risco de T3 (governance silenciosa).

**Opção B — `rules: - when: manual`** (dentro de `rules:`):
Conforme documentação GitLab ([Specify when jobs run with rules](https://docs.gitlab.com/ci/jobs/job_rules/)), dentro de `rules:`, `allow_failure` defaults a **false** → pipeline bloqueia na stage até o job ser executado manualmente.

Restrição técnica: `only:` e `rules:` **não podem coexistir na mesma job** (GitLab CI/CD YAML syntax reference). Usar `rules:` exige remover `only:`.

**Arquiteto decide:** opção A (gate opcional) ou opção B (MR bloqueado até execução manual). Hades não recomenda entre as duas — são trade-offs de política do consumidor. A decisão deve ser documentada no template gerado como comentário.

---

## Seção 3 — Alvos de falsificação nos dois sentidos

### Falso negativo (variante manual escapa para pull_request)

| Vetor | Mecanismo | Gate que deve pegar |
|---|---|---|
| `ci_trigger: [manual]` (lista YAML) | SequenceNode → ok=false → ausente | Check de presença + tipo → CITriggerInvalid=true → violation AC4 |
| `ci_trigger: {a: b}` (mapeamento) | MappingNode → ok=false → ausente | Idem |
| `ci_trigger: "MANUAL"` | string não está no enum | Verificador de enum → violation AC4 |
| `ci_trigger: " manual"` (espaço) | string não está no enum | Idem |
| `ci_trigger: ""` (vazio) | string "" não está no enum | Idem |
| `ci_trigger: "manual\non: push"` (newline) | string não está no enum + tentativa de injeção | Enum rejeita; switch de template nunca interpola o valor |
| `update` com `ci_trigger: manual` + `validate.yml` presente | `refresh` não passa `ciTrigger` | `BuildDiscoverGitHubActionsWorkflowContent` recebe `ciTrigger` via bridge |
| `update` com valor inválido | `update.go:110-111` ⚠ rc=0 + arquivo antigo mantido | AC4 exige rc≠0 e arquivo inalterado; verificação: `echo $?` deve ser ≠ 0 |
| Doctor com `ci_trigger: manual` + disco com `workflow_dispatch` | doctor não passa `ciTrigger` | Doctor usa `ciTrigger` para gerar template de comparação → sem finding |

### Falso positivo (config correta flagged como divergente)

| Vetor | Mecanismo | Gate que deve pegar |
|---|---|---|
| `ci_trigger: pull_request` explícito | mesmo valor que ausente | AC2: as 5 variantes padrão byte a byte idênticas ao original |
| Ausência de `ci_trigger` | campo não presente | Comportamento idêntico ao de hoje; sem AC4 violation |
| `ci_trigger: manual` + disco com `workflow_dispatch` correto | doctor compara disco vs template certo com ciTrigger | Nenhum finding; AC3 |

### AC2 — advertência sobre testes byte-exact em discover_test.go

`discover_test.go:866,892,964,965` comparam a saída de disco contra a própria função `BuildDiscoverGitHubActionsWorkflowContent`. Se ML-1A mudar assinatura e conteúdo da variante `pull_request`, os testes são atualizados junto com a função — **e nunca fixam o que o conteúdo original era**. Uma refatoração que subtilmente mude o `pull_request` variant continua passando.

**Alvo de falsificação para AC2:** os 5 templates (`pull_request` produtor, `pull_request` consumidor, `manual` produtor, `manual` consumidor, GitLab manual) devem ser fixados em golden files capturados do binário `origin/main` **antes** de ML-1A. O pin de versão deve ser normalizado (substituído por placeholder constante) para estabilidade entre releases. O teste de AC2 diff esses golden files contra a saída gerada — não contra a própria função.

### AC3 em ambas as direções (falsos negativos do doctor)

O padrão `checkValidateScriptArtifact` (scaffold_doctor.go:418) usa set-membership: aceita qualquer membro do conjunto. O implementador apressado pode copiar esse padrão para o doctor de workflow e aceitar "qualquer trigger" — falso negativo para as cross-combinations.

| Cross-vector | Comportamento correto | Implementação errada (set-membership) |
|---|---|---|
| config=manual, disco=pull_request | scaffold-divergent | falso-negativo: aceita ambos |
| config=pull_request, disco=manual | scaffold-divergent | falso-negativo: aceita ambos |
| config=manual, disco=manual | sem finding | correto (coincidência) |
| config=pull_request, disco=pull_request | sem finding | correto (coincidência) |

O gate certo: `checkScaffoldArtifact` (existente, usado em scaffold_doctor.go:319,337,403) compara byte-a-byte contra o template **único** gerado para o `ciTrigger` configurado — não set-membership. Ao passar `ciTrigger` para os builders, esse comportamento já está correto; o risco está em adicionar um nível de set-membership em vez de usar `checkScaffoldArtifact` com o template certo.

---

## Seção 4 — Resíduo declarado

1. **`init` sem CITrigger:** AC1 não é satisfeito pelo `update` sozinho. Três opções (wizard/flag/brownfield-read) — decisão do arquiteto. Gap: template (scaffold.go:948) não inclui `ci_trigger:` e o `generators.Config` passado para os geradores de workflow não tem `CITrigger` mesmo que o merge textual preserve a chave no arquivo.

2. **GitLab opção A vs opção B:** `when: manual` job-level torna o gate opcional (`allow_failure: true`). `rules: - when: manual` torna o MR bloqueado. O produto não pode resolver a assimetria — deve documentá-la no template gerado como comentário. Decisão do arquiteto.

3. **`workflow_dispatch` não satisfaz `required_status_checks`:** o produto não pode verificar sem `--remote` + credencial admin + nova lógica. O WARNING de T3 é o único meio de informar.

4. **`discover_test.go` e AC2:** os testes byte-exact comparam contra a própria função, não golden files. Refatoração sutil na variante `pull_request` não é detectada. Golden files capturados de `origin/main` antes de ML-1A fecham esse gap.

5. **`doctor --remote` não detecta "required check que nunca roda":** gap de cobertura declarado. `DoctorRequiredStatusChecksMissing` (doctor.go:88) verifica presença de required checks, não se o job nomeado dispara em PRs. Expansão é trabalho separado.

6. **`ci: [X]` e `ci: {a: b}` na chave existente (Regra Dura — candidato para o arquiteto):** a mesma falha de `stringVal` afeta a chave `ci:` já existente. Medido: `ci: [github-actions]` → SequenceNode → ok=false → `cfg.Update.CI = ""` → fallback silencioso. `ci: {a: b}` → MappingNode → ok=false → idem. Mecanismo idêntico ao T1 de `ci_trigger:`. Por Regra Dura de Causa Raiz, o arquiteto deve decidir se isso entra nesta REQ como ML adicional ou se abre REQ separada com diferença de mecanismo escrita.

7. **T3 Vetor B — `branch_has_wip_roadmap` vacuosamente verdadeiro:** sob dispatch do branch default, `TRACKFW_BRANCH` vazio + symbolic-ref (validator.go:4386) retorna `"main"`. Se symbolic-ref falhar (detached HEAD), `GITHUB_REF_NAME` também retorna `"main"`. Qualquer caminho: validator.go:4399 → nil vacuosamente. O PR branch nunca é validado. Limitação declarada no WARNING de T3.

8. **`scripts/check-ci-workflow-binary-provenance.py`:** corpus interno com `on: [pull_request]` não valida o trigger gerado em produção. Não precisa ser atualizado para esta REQ.

### Gates medidos (ML-0A)

```bash
$ test -s docs/seguranca/2026-10-24-wave0-ci-trigger.md; echo "rc=$?"
rc=0

$ git diff --quiet origin/main -- internal/; echo "rc=$?"
rc=0
```

Ambos passam. Nenhuma linha de produto foi modificada.

---

Sources consultadas (todos via search result summary, página não obtida diretamente):
- GitLab CI: Control how jobs run — `when: manual` + `allow_failure` default (T6 Opção A)
- GitLab CI: `rules: - when: manual` + `allow_failure` default false (T6 Opção B)
- GitLab CI/CD YAML syntax reference: `only:` e `rules:` não podem coexistir (T6)
- GitHub Docs: Troubleshooting required status checks — `workflow_dispatch` não aparece em PR check suite (T3/T5)


## §Z — Auditoria do arquiteto (zeus-tf, 2026-10-24)

Aprovado. As três decisões pendentes:
1. **`init`:** opção (c). O `init` honra o `ci_trigger` de um `trackfw.yaml` existente. Não ganha flag nem pergunta nova.
2. **GitLab:** nem A nem B. `manual` vira `rules: - if: '$CI_PIPELINE_SOURCE == "web"'`, o equivalente do
   `workflow_dispatch`: pipeline disparado à mão pela interface. A opção A torna o gate opcional sem aviso
   (`allow_failure: true`). A opção B cria, em todo MR, um pipeline parado esperando clique, e consome a fila sem rodar.
3. **`ci: [X]`:** é a mesma causa do T1 (`stringVal` descarta não escalar), então entra nesta REQ como ML-1B, corrigida no
   parser em ponto único para toda chave escalar.

T3 e T5 ficam como aviso no `validate` e como contrato no `docs/cli-parity.md`. O cruzamento com `required_status_checks`
pelo doctor remoto é resíduo declarado.
