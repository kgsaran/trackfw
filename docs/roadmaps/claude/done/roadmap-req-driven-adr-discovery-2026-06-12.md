# Roadmap: REQ-Driven ADR Discovery

> Criado em: 2026-06-12 | Status: 🔄 WIP

## Diagnóstico / Contexto

O wizard `trackfw req new` atual coleta título, motivação, critérios e links manuais. Não há nenhuma análise de contexto nem geração guiada de ADRs. O novo fluxo deve:

1. Detectar domínios técnicos na descrição da intenção (keywords em pt/en)
2. Exibir perguntas-chave por domínio detectado via `huh.Select`
3. Gerar ADRs Draft automaticamente para decisões não-resolvidas
4. Vincular ADRs à REQ no frontmatter
5. Fazer `trackfw validate` e `trackfw status` reconhecerem o vínculo

## Arquivos afetados

```
internal/generators/req.go          → REQContent + NewREQ (novo campo DependsOnADRs)
internal/generators/adr.go          → NewADRDraft (variant sem wizard, só título + status Draft)
internal/generators/probes.go       → NOVO — catálogo de domínios + lógica de detecção
internal/commands/req.go            → wizard req new com Etapa 2 de probes
internal/validator/validator.go     → nova regra: REQ Open com ADR Draft vinculado
internal/generators/req_test.go     → testes dos novos campos
internal/generators/probes_test.go  → NOVO — testes de detecção de domínio
internal/generators/adr_test.go     → teste de NewADRDraft
```

---

## Wave 1 — Fundação (independentes entre si, podem rodar em paralelo)

### ML-1A — Catálogo de probes e detecção de domínio

**Status:** ✅ Concluído
**Arquivo:** `internal/generators/probes.go` (novo)

**Estrutura:**
```go
type Probe struct {
    Domain   string
    Keywords []string   // pt + en, lowercase
    Questions []Question
}

type Question struct {
    Text    string
    Options []Option
}

type Option struct {
    Label     string
    ADRSlug   string  // vazio = decisão tomada; não-vazio = gera ADR Draft
    Decided   bool    // true = não gera ADR
}
```

**Catálogo a implementar:**

```go
var ProbesCatalog = []Probe{
    {
        Domain: "authentication",
        Keywords: []string{"login", "auth", "senha", "password", "sso", "jwt", "session", "token", "autenticação", "autenticar"},
        Questions: []Question{
            {
                Text: "How will users authenticate?",
                Options: []Option{
                    {Label: "Local login (email + password)", Decided: true},
                    {Label: "SSO (Google, Azure AD, Okta...)", ADRSlug: "sso-provider"},
                    {Label: "Both", ADRSlug: "authentication-strategy"},
                    {Label: "Not decided yet", ADRSlug: "authentication-strategy"},
                },
            },
            {
                Text: "How will sessions be managed?",
                Options: []Option{
                    {Label: "JWT (stateless)", Decided: true},
                    {Label: "Server-side sessions (cookies)", Decided: true},
                    {Label: "Not decided yet", ADRSlug: "session-management"},
                },
            },
        },
    },
    {
        Domain: "ui",
        Keywords: []string{"tela", "screen", "ui", "frontend", "componente", "component", "design", "layout", "interface"},
        Questions: []Question{
            {
                Text: "Is there an existing UI framework or design system?",
                Options: []Option{
                    {Label: "Yes, already chosen", Decided: true},
                    {Label: "No, need to choose", ADRSlug: "ui-framework"},
                    {Label: "Not relevant for this REQ", Decided: true},
                },
            },
        },
    },
    {
        Domain: "persistence",
        Keywords: []string{"banco", "database", "db", "tabela", "table", "migração", "migration", "modelo", "model", "persistência", "persist"},
        Questions: []Question{
            {
                Text: "Which database engine will be used?",
                Options: []Option{
                    {Label: "Already decided", Decided: true},
                    {Label: "Not decided yet", ADRSlug: "database-engine"},
                },
            },
        },
    },
    {
        Domain: "api",
        Keywords: []string{"api", "endpoint", "rest", "grpc", "graphql", "rota", "route", "http"},
        Questions: []Question{
            {
                Text: "Which API protocol will be used?",
                Options: []Option{
                    {Label: "REST (already decided)", Decided: true},
                    {Label: "gRPC (already decided)", Decided: true},
                    {Label: "GraphQL (already decided)", Decided: true},
                    {Label: "Not decided yet", ADRSlug: "api-protocol"},
                },
            },
        },
    },
    {
        Domain: "deploy",
        Keywords: []string{"deploy", "cloud", "container", "kubernetes", "k8s", "docker", "infra", "aws", "gcp", "azure"},
        Questions: []Question{
            {
                Text: "Is the deployment infrastructure already defined?",
                Options: []Option{
                    {Label: "Yes, fully defined", Decided: true},
                    {Label: "Cloud provider not decided", ADRSlug: "cloud-provider"},
                    {Label: "Container strategy not decided", ADRSlug: "container-strategy"},
                },
            },
        },
    },
    {
        Domain: "events",
        Keywords: []string{"kafka", "fila", "queue", "notificação", "notification", "evento", "event", "pubsub", "pub/sub", "broker", "sqs", "redis"},
        Questions: []Question{
            {
                Text: "Which event broker will be used?",
                Options: []Option{
                    {Label: "Already decided", Decided: true},
                    {Label: "Not decided yet", ADRSlug: "event-broker"},
                },
            },
        },
    },
}
```

**Função principal:**
```go
// DetectDomains retorna probes relevantes para a intenção descrita.
func DetectDomains(intention string) []Probe
```

**Critérios de aceite:**
- [ ] `DetectDomains("tela de login")` retorna probes de authentication + ui
- [ ] `DetectDomains("endpoint de pagamento")` retorna probe de api
- [ ] `DetectDomains("algo sem keyword")` retorna slice vazio
- [ ] keywords case-insensitive (Login == login == LOGIN)

**Comandos de validação:** `go test ./internal/generators/ -run TestDetectDomains`

---

### ML-1B — NewADRDraft no generator de ADR

**Status:** ✅ Concluído
**Arquivo:** `internal/generators/adr.go`

**Função a adicionar:**
```go
// NewADRDraft cria um ADR com Status: Draft a partir de um slug.
// Usado pelo wizard req new para registrar decisões pendentes.
// Retorna o nome do arquivo criado (sem path).
func NewADRDraft(slug string) (string, error)
```

- Status no header: `Draft` (em vez de `Proposed`)
- Título derivado do slug: `slug-to-title(slug)` (hífens → espaços, title case)
- Seções Context/Decision/Consequences/Alternatives com placeholder HTML
- Salvo em `docs/adr/ADR-<date>-<slug>.md`
- Retorna o basename para que o caller possa construir o link

**Critérios de aceite:**
- [ ] `NewADRDraft("authentication-strategy")` cria `docs/adr/ADR-<date>-authentication-strategy.md`
- [ ] Arquivo gerado contém `Status: Draft`
- [ ] Função retorna o basename correto
- [ ] Segunda chamada com mesmo slug NÃO sobrescreve (idempotência)

**Comandos de validação:** `go test ./internal/generators/ -run TestNewADRDraft`

---

## Wave 2 — REQContent estendido e wizard

> Dependências: ML-1A e ML-1B completos

### ML-2A — REQContent + NewREQ com DependsOnADRs

**Status:** ✅ Concluído
**Arquivo:** `internal/generators/req.go`

**Mudança em REQContent:**
```go
type REQContent struct {
    Title         string
    Motivation    string
    Criteria      string
    LinkedADR     string
    LinkedRoadmap string
    DependsOnADRs []string  // NOVO — lista de basenames de ADRs Draft vinculados
}
```

**Mudança no template de saída — nova seção após `## Linked ADR`:**
```markdown
## Blocked by ADRs
<!-- ADRs in Draft status that must be Accepted before a roadmap can be created -->
- ADR-2026-06-12-authentication-strategy.md (Draft)
- ADR-2026-06-12-ui-framework.md (Draft)
```

Quando `DependsOnADRs` está vazio, a seção aparece com placeholder:
```markdown
## Blocked by ADRs
<!-- none -->
```

Linha de status atualizada:
```
> Date: 2026-06-12 | Status: Open | Blocked by ADRs: 2
```
(contador = len(DependsOnADRs); 0 omite o trecho "| Blocked by ADRs: 0")

**Critérios de aceite:**
- [ ] REQ gerada com 2 ADRs vinculados contém seção "## Blocked by ADRs" com 2 entradas
- [ ] REQ sem ADRs vinculados contém `<!-- none -->` na seção
- [ ] Linha de status exibe `| Blocked by ADRs: 2` quando há vinculados
- [ ] `go test ./internal/generators/ -run TestNewREQ` verde

---

### ML-2B — Wizard `req new` com Etapa 2

**Status:** ✅ Concluído
**Arquivo:** `internal/commands/req.go`

**Novo fluxo `runReqNew`:**

```
1. Input: título da REQ (existente)
2. Input: motivação (existente)  
3. Input: critérios de aceite (existente)
4. [NOVO] DetectDomains(título + motivação)
   → se probes vazios: pular para passo 5
   → se probes não-vazios: para cada probe, exibir huh.Select com as opções
     Opções com Decided=false → chamar NewADRDraft(option.ADRSlug) → coletar basename
5. Resumo: print das ADRs geradas (se houver)
6. NewREQ(content com DependsOnADRs preenchido)
```

**Detalhe da etapa 4 com huh:**
```go
// Para cada probe detectada, criar um Group com um Select
for _, probe := range detectedProbes {
    for _, question := range probe.Questions {
        var answer string
        groups = append(groups, huh.NewGroup(
            huh.NewSelect[string]().
                Title(question.Text).
                Options(/* mapear question.Options → huh.Option */).
                Value(&answer),
        ))
        // Mapear answer → ADRSlug após form.Run()
    }
}
```

**Observação:** como `huh.NewForm` precisa receber os grupos antes de `Run()`, os grupos de probes são construídos dinamicamente após o input do título/motivação. Isso requer dois `huh.NewForm` em sequência: Form1 coleta título+motivação, Form2 exibe as probes (se houver).

**Critérios de aceite:**
- [ ] `trackfw req new "login screen"` exibe perguntas de autenticação e UI
- [ ] Selecionar "Not decided yet" → arquivo ADR Draft criado + mensagem "created docs/adr/..."
- [ ] Selecionar opção decidida → nenhum ADR gerado para aquela probe
- [ ] Pular todas as probes (todas "decided") → REQ criada sem DependsOnADRs
- [ ] Mensagem final lista ADRs gerados

---

## Wave 3 — Validate e Status

> Dependências: Wave 2 completa

### ML-3A — Nova regra em validate

**Status:** ✅ Concluído
**Arquivo:** `internal/validator/validator.go`

**Nova função a adicionar:**
```go
// validateREQsNotBlockedByDraftADRs verifica se REQs Open têm ADRs Draft vinculados.
func validateREQsNotBlockedByDraftADRs() ([]string, error)
```

**Lógica:**
1. Glob `docs/req/*.md`
2. Para cada arquivo: verificar status `Open` e seção `## Blocked by ADRs`
3. Para cada ADR listado na seção: verificar se o arquivo existe em `docs/adr/` e se contém `Status: Draft`
4. Violação: `"REQ <filename> is blocked by Draft ADR: <adr-filename>"`

**Chamada adicionada em `Validate()`:**
```go
blockedByDraftViolations, e := validateREQsNotBlockedByDraftADRs()
violations = append(violations, blockedByDraftViolations...)
```

**Critérios de aceite:**
- [ ] REQ Open com ADR Draft vinculado → violação retornada
- [ ] REQ Open com ADR Accepted vinculado → sem violação
- [ ] REQ sem seção `## Blocked by ADRs` → sem violação (retrocompatível)

---

### ML-3B — Nova seção em status

**Status:** ✅ Concluído
**Arquivo:** `internal/validator/validator.go` (função `GetStatus`)

**Nova seção a adicionar após `❌ Blocked`:**
```
⏳ REQs blocked by Draft ADRs (N)
   REQ-2026-06-12-login-screen.md
     → ADR-2026-06-12-authentication-strategy.md (Draft)
     → ADR-2026-06-12-ui-framework.md (Draft)
```

Quando não há REQs bloqueadas: seção omitida (para não poluir output).

**Critérios de aceite:**
- [ ] Status exibe seção quando há REQs bloqueadas
- [ ] Seção omitida quando não há bloqueios
- [ ] Formato indentado legível

---

## Wave 4 — Testes

> Podem rodar em paralelo com Wave 3 para os testes de Wave 1 e 2

### ML-4A — Testes de probes

**Status:** ✅ Concluído
**Arquivo:** `internal/generators/probes_test.go` (novo)

Testes: `TestDetectDomains_Authentication`, `TestDetectDomains_UI`, `TestDetectDomains_NoMatch`, `TestDetectDomains_MultiDomain`, `TestDetectDomains_CaseInsensitive`

### ML-4B — Testes de ADR Draft

**Status:** ✅ Concluído
**Arquivo:** `internal/generators/adr_test.go`

Testes: `TestNewADRDraft_CriaArquivo`, `TestNewADRDraft_StatusDraft`, `TestNewADRDraft_Idempotente`, `TestNewADRDraft_TituloDerivado`

### ML-4C — Testes de REQ com DependsOnADRs

**Status:** ✅ Concluído
**Arquivo:** `internal/generators/req_test.go`

Testes: `TestNewREQ_ComADRsVinculados`, `TestNewREQ_SemADRsVinculados`, `TestNewREQ_ContadorNoStatus`

### ML-4D — Testes de validator

**Status:** ✅ Concluído
**Arquivo:** `internal/validator/validator_test.go`

Testes: `TestValidateREQsNotBlockedByDraftADRs_Violação`, `TestValidateREQsNotBlockedByDraftADRs_SemViolação`, `TestValidateREQsNotBlockedByDraftADRs_Retrocompatível`

---

## Protocolo de conclusão

```
1. go build ./...
2. go test ./...
3. git commit -m "feat(req): REQ-driven ADR discovery — wizard contextual + validate + status"
4. git push origin feat/req-driven-adr-discovery
5. Mover roadmap para done/
```
