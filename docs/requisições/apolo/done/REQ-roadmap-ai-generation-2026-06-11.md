# REQ: Geração de Roadmap por IA

> Criado em: 2026-06-11 | Status: Backlog | Agente: Apolo

## Solicitação

Implementar geração automática de roadmap via IA no comando `trackfw roadmap new`. O usuário seleciona uma REQ existente via wizard interativo; a IA gera o roadmap seguindo os preceitos de microlotes com paralelização prevista. Fallback para template vazio quando nenhuma API key estiver configurada.

## Escopo

### 1. Pacote `internal/ai/`

Criar interface `Client` + implementações:

**`internal/ai/client.go`**
- Interface `Client` com método `Generate(ctx context.Context, prompt string) (string, error)`
- `NewClient(provider, model, apiKey string) (Client, error)` — factory que retorna impl por provider

**`internal/ai/anthropic.go`**
- Struct `AnthropicClient` implementando `Client`
- Usa `github.com/anthropics/anthropic-sdk-go`
- Modelo padrão: `claude-haiku-4-5-20251001` (via constante `anthropic.ModelClaudeHaiku4_5_20251001`)
- Inicializar com `anthropic.NewClient()` + `option.WithAPIKey(apiKey)` quando apiKey fornecido, senão depende da env var

**`internal/ai/openai.go`**
- Struct `OpenAIClient` implementando `Client`
- Usa `net/http` + `encoding/json` (sem SDK externo de OpenAI — apenas stdlib)
- Endpoint: `https://api.openai.com/v1/chat/completions`
- Modelo: passado como parâmetro (default: `gpt-4o-mini`)
- Authorization: `Bearer <apiKey>`

**`internal/ai/fake.go`**
- Struct `FakeClient` implementando `Client` — retorna string estática para testes

### 2. Leitura de configuração AI em `trackfw.yaml`

Criar `internal/ai/config.go`:
- `ReadConfig(path string) (provider, model, apiKey string, err error)` — grep-based, sem yaml.v3
- Chaves esperadas no YAML: `ai_provider:`, `ai_model:`, `ai_api_key:`
- Retornar strings vazias se ausentes (não erro)

### 3. Modificar `internal/generators/roadmap.go`

Adicionar:
```go
type RoadmapContent struct {
    Title   string
    REQPath string
    Body    string // markdown gerado pela IA ou template vazio
}

func NewRoadmapFromContent(content RoadmapContent) error
```
- `NewRoadmapFromContent` salva em `docs/roadmaps/backlog/<slug>.md`
- Manter `NewRoadmap(title string)` existente chamando `NewRoadmapFromContent` internamente

### 4. Modificar `internal/commands/roadmap.go`

Reescrever `newRoadmapNewCmd()`:
1. Listar arquivos `docs/req/*.md` via `filepath.Glob`
2. Se TTY: exibir `huh.Select` com os nomes das REQs disponíveis
3. Ler o conteúdo do arquivo REQ selecionado
4. Ler config AI de `trackfw.yaml` (se presente)
5. Se provider configurado e apiKey disponível: chamar `ai.NewClient(...).Generate(ctx, prompt)`
6. Se não configurado ou erro: usar template vazio (log de aviso no stderr)
7. Chamar `generators.NewRoadmapFromContent(content)`

**Prompt para a IA** (constante em `internal/commands/roadmap.go`):
```
Você é um assistente de engenharia de software. Com base na REQ abaixo, gere um roadmap de implementação em Markdown seguindo ESTRITAMENTE este formato:

# Roadmap: <título>

> Criado em: <data> | Status: ⬜ Backlog

## Diagnóstico / Contexto
(resumo do problema a resolver)

## Wave 1 — <nome> (N MLs em paralelo)
> Dependências: Independente

### ML-1A — <título>
**Status:** ⬜ Pendente
**Arquivos afetados:** lista
**Ações:** lista detalhada
**Critérios de aceite:**
- [ ] build sem erros
- [ ] testes verdes
**Comandos de validação:** `go build ./...`

(Adicione quantas Waves e MLs forem necessários. Maximize paralelismo entre MLs sem dependência de arquivos.)

REQ:
---
<conteúdo da REQ>
---
```

### 5. Modificar `internal/generators/scaffold.go`

Em `writeTrackfwConfig(cfg)`, adicionar chaves AI ao YAML gerado:
```yaml
ai_provider: 
ai_model: 
ai_api_key: 
```

Em `buildValidateScript` / `writeTrackfwConfig`: sem mudança de lógica.

### 6. Modificar `internal/commands/init.go`

Adicionar Grupo AI no wizard `trackfw init`:
- `huh.Select` para `ai_provider`: opções `none`, `anthropic`, `openai`
- `huh.Input` para `ai_api_key` (placeholder, não obrigatório)
- Passar `AIProvider` e `AIApiKey` no `Config`

Adicionar campos `AIProvider string` e `AIApiKey string` em `generators.Config`.

### 7. Testes `internal/ai/`

**`internal/ai/client_test.go`**:
- `TestReadConfig_Empty` — sem arquivo → strings vazias, sem erro
- `TestReadConfig_WithValues` — YAML com as chaves → retorna valores corretos
- `TestFakeClient_Generate` — retorna string estática sem erro

**`internal/generators/roadmap_test.go`** — adicionar:
- `TestNewRoadmapFromContent_CreatesFile` — verifica arquivo criado com body
- `TestNewRoadmapFromContent_EmptyBody` — verifica que placeholder é inserido

## Restrições

- Apenas stdlib para OpenAI (sem SDK externo)
- `github.com/anthropics/anthropic-sdk-go` para Anthropic (já deve entrar no go.mod)
- Testes isolados com `os.TempDir()` + `os.Chdir()`
- Sem quebra dos 26 testes existentes
- `go build ./...` e `go vet ./...` devem passar ao final

## Roadmap

Roadmap: roadmap-roadmap-ai-generation-2026-06-11
