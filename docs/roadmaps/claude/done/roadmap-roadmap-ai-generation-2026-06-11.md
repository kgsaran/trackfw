# Roadmap: Geração de Roadmap por IA

> Criado em: 2026-06-11 | Status: 🔄 WIP

## Diagnóstico / Contexto

O comando `trackfw roadmap new <title>` gera apenas um template vazio. O usuário solicitou que o roadmap seja gerado automaticamente por IA com base em uma REQ existente, seguindo os preceitos de microlotes com paralelização prevista. Quando nenhuma chave de API estiver configurada, o comportamento atual (template vazio) deve ser mantido como fallback.

---

## Wave 1 — Foundation: Pacote AI + Config (2 MLs em paralelo)

> Dependências: Independente

### ML-1A — Criar pacote `internal/ai/`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/ai/client.go` (novo)
- `internal/ai/anthropic.go` (novo)
- `internal/ai/openai.go` (novo)
- `internal/ai/fake.go` (novo)
- `internal/ai/config.go` (novo)
- `internal/ai/client_test.go` (novo)
- `go.mod` / `go.sum` (adição de `anthropic-sdk-go`)

**Ações:**
1. `internal/ai/client.go`: definir interface `Client { Generate(ctx, prompt string) (string, error) }` e factory `NewClient(provider, model, apiKey string) (Client, error)`
2. `internal/ai/anthropic.go`: struct `AnthropicClient`; usar `anthropic.NewClient()` com `option.WithAPIKey(apiKey)` se apiKey != ""; chamar `client.Messages.New(ctx, anthropic.MessageNewParams{Model: model, MaxTokens: 4096, Messages: [...]})` e retornar `content[0].Text`
3. `internal/ai/openai.go`: struct `OpenAIClient`; usar apenas stdlib `net/http` + `encoding/json`; POST para `https://api.openai.com/v1/chat/completions` com header `Authorization: Bearer <apiKey>`; extrair `choices[0].message.content`
4. `internal/ai/fake.go`: `FakeClient` com campo `Response string`; `Generate` retorna `f.Response, nil`
5. `internal/ai/config.go`: `ReadConfig(path string) (provider, model, apiKey string, err error)` — abrir arquivo, scanner linha a linha, extrair valor após `: ` para chaves `ai_provider`, `ai_model`, `ai_api_key`; retornar strings vazias se arquivo ausente (sem erro)
6. `go get github.com/anthropics/anthropic-sdk-go`
7. Testes: `TestReadConfig_Empty`, `TestReadConfig_WithValues`, `TestFakeClient_Generate`

**Critérios de aceite:**
- [ ] `go build ./internal/ai/...` sem erros
- [ ] `go vet ./internal/ai/...` limpo
- [ ] 3 novos testes verdes

**Comandos de validação:**
```bash
go build ./internal/ai/...
go test ./internal/ai/... -v
```

---

### ML-1B — Estender `generators.Config` + `scaffold.go` + `init.go`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/scaffold.go`
- `internal/commands/init.go`

**Ações:**
1. `scaffold.go`: adicionar `AIProvider string`, `AIApiKey string` em `Config`; em `writeTrackfwConfig`, acrescentar ao template YAML:
   ```
   ai_provider: %s
   ai_model: 
   ai_api_key: %s
   ```
   passando `cfg.AIProvider` e `cfg.AIApiKey`
2. `init.go`: no Grupo 4 (ou novo Grupo 5) do wizard, adicionar:
   - `huh.Select().Title("AI provider").Options(huh.NewOption("none","none"), huh.NewOption("anthropic","anthropic"), huh.NewOption("openai","openai")).Value(&cfg.AIProvider)`
   - `huh.Input().Title("AI API Key (opcional)").Value(&cfg.AIApiKey)`
3. Garantir que `cfg.AIProvider` padrão seja `"none"` antes do wizard

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go vet ./...` limpo
- [ ] 26 testes existentes continuam verdes

**Comandos de validação:**
```bash
go build ./...
go test ./... -v
```

---

## Wave 2 — Geração de Roadmap com IA (depende de Wave 1)

> Dependências: ML-1A e ML-1B completos

### ML-2A — Modificar `generators/roadmap.go` + testes
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/roadmap.go`
- `internal/generators/roadmap_test.go`

**Ações:**
1. Adicionar struct:
   ```go
   type RoadmapContent struct {
       Title   string
       REQPath string
       Body    string
   }
   ```
2. Adicionar `NewRoadmapFromContent(content RoadmapContent) error`:
   - Se `content.Body == ""`: usar template vazio existente (igual ao `NewRoadmap` atual)
   - Se `content.Body != ""`: salvar body diretamente em `docs/roadmaps/backlog/<date>-<slug>.md`
   - Imprimir `created <path>\n` ao stdout
3. Refatorar `NewRoadmap(title string)` para chamar `NewRoadmapFromContent(RoadmapContent{Title: title})`
4. Adicionar testes: `TestNewRoadmapFromContent_CreatesFile`, `TestNewRoadmapFromContent_EmptyBody`

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] Todos os testes verdes (incluindo os 5 existentes de roadmap)

**Comandos de validação:**
```bash
go build ./...
go test ./internal/generators/... -v
```

---

### ML-2B — Reescrever `commands/roadmap.go` (wizard + IA)
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/commands/roadmap.go`

**Ações:**
1. Definir constante `roadmapPromptTemplate` com o prompt de geração (ver REQ para o texto exato)
2. Reescrever `newRoadmapNewCmd()`:
   ```
   a. filepath.Glob("docs/req/*.md") → lista de REQs disponíveis
   b. Se TTY e len(reqs) > 0: huh.Select com os basenames como opções
   c. Se não-TTY ou len(reqs) == 0: usar args[0] como título, body vazio
   d. os.ReadFile(reqPath) → reqContent
   e. ai.ReadConfig("trackfw.yaml") → provider, model, apiKey
   f. Se provider != "" && provider != "none" && apiKey != "":
        client, _ := ai.NewClient(provider, model, apiKey)
        body, err := client.Generate(ctx, fmt.Sprintf(roadmapPromptTemplate, time.Now().Format("2006-01-02"), reqContent))
        Se err != nil: fmt.Fprintln(os.Stderr, "⚠ AI indisponível, usando template vazio"); body = ""
      Senão: body = ""
   g. generators.NewRoadmapFromContent(RoadmapContent{Title: reqTitle, REQPath: reqPath, Body: body})
   ```
3. Manter `newRoadmapMoveCmd()` e `newRoadmapListCmd()` (se existir) sem alteração
4. Imports: `context`, `fmt`, `os`, `path/filepath`, `time`, `internal/ai`, `internal/generators`

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go vet ./...` limpo
- [ ] Em TTY sem AI configurada: cria arquivo com template vazio
- [ ] Em não-TTY (CI): `trackfw roadmap new "minha feature"` cria arquivo sem travar

**Comandos de validação:**
```bash
go build ./...
go vet ./...
go test ./... -v
```

---

## Wave 3 — Commit e Push

> Dependências: Wave 2 completa, todos os testes verdes

### ML-3A — Commit e push na branch `feat/roadmap-ai-generation`
**Status:** ⬜ Pendente

**Ações:**
1. `go build ./...` — confirmação final
2. `go test ./... -v` — todos verdes
3. `go vet ./...` — limpo
4. `git add internal/ai/ internal/generators/roadmap.go internal/generators/roadmap_test.go internal/commands/roadmap.go internal/generators/scaffold.go internal/commands/init.go go.mod go.sum`
5. `git commit -m "feat(roadmap): geração por IA via huh.Select + Anthropic/OpenAI + fallback template"`
6. `git push origin feat/roadmap-ai-generation`
7. Atualizar `docs/agents-working-context.md` com sessão concluída

**Critérios de aceite:**
- [ ] Push bem-sucedido
- [ ] Branch visível no remoto

**Comandos de validação:**
```bash
git log --oneline -3
git status
```
