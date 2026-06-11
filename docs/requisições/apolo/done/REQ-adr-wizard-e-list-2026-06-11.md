# REQ: ADR — Wizard Interativo nas Seções e adr list

> Criado em: 2026-06-11 | Status: WIP | Agente: Apolo

## Solicitação

Evoluir o comando `trackfw adr` com dois novos comportamentos:
1. **Wizard interativo**: `adr new` passa a perguntar Context, Decision, Consequences e Alternatives via terminal antes de gerar o arquivo.
2. **adr list**: novo subcomando que lista ADRs existentes com título e status.

## Escopo

### Mudança 1 — Wizard interativo (`adr new`)

**Arquitetura obrigatória (não negociável):**
- O wizard `huh` fica **exclusivamente em `internal/commands/adr.go`** (command layer).
- O generator `internal/generators/adr.go` passa a receber uma struct `ADRContent` com os campos preenchidos — nunca faz I/O.
- Isso preserva testabilidade: os testes chamam o generator diretamente com dados fixos, sem depender de stdin.

**Struct a criar em `internal/generators/adr.go`:**
```go
type ADRContent struct {
    Title       string
    Context     string
    Decision    string
    Consequences string
    Alternatives string
}
```

**Assinatura do generator:**
```go
func NewADR(content ADRContent) error
```

**Wizard em `internal/commands/adr.go`** (usar `huh.NewText` para campos multiline, espelhando o padrão de `internal/commands/init.go`):
- Campo 1: "Context" — `huh.NewText().Title("Context").Description("What is the situation that motivates this decision?")`
- Campo 2: "Decision" — `huh.NewText().Title("Decision").Description("What was decided?")`
- Campo 3: "Consequences" — `huh.NewText().Title("Consequences").Description("What are the positive and negative consequences?")`
- Campo 4: "Alternatives Considered" — `huh.NewText().Title("Alternatives Considered").Description("What other options were evaluated and why were they rejected?")`

**Fallback não-TTY:**
- Detectar `!term.IsTerminal(int(os.Stdin.Fd()))` (ou `os.Getenv("CI") != ""`).
- Se não-TTY: chamar `generators.NewADR(ADRContent{Title: title})` com campos vazios (comportamento atual, mantendo compatibilidade com scripts/CI).
- Usar `golang.org/x/term` para detecção — já disponível ou importar se necessário.

**Atualizar testes existentes** em `internal/generators/adr_test.go`:
- `TestNewADR_CreatesFile` e `TestNewADR_SlugInFilename` devem chamar `NewADR(ADRContent{Title: "..."})`.
- Não quebrar nenhum teste existente.

### Mudança 2 — `adr list`

**Subcomando:** `trackfw adr list`

**Comportamento:**
- Ler todos os arquivos `docs/adr/*.md` via `filepath.Glob`.
- Se o diretório não existir ou estiver vazio: imprimir mensagem amigável `"No ADRs found in docs/adr/"` e retornar nil (não error).
- Para cada arquivo, extrair:
  - **Título**: da linha `# ADR: <título>` (primeiro `# ADR:` do arquivo).
  - **Status**: da linha `> Date: … | Status: <status>` (regex ou strings.Split).
- Ordenar por nome de arquivo (glob já retorna ordenado alfabeticamente = ordem cronológica).
- Output tabular simples (sem lipgloss obrigatório), ex:
  ```
  ADR-2026-06-11-adotar-postgresql.md   Proposed
  ADR-2026-06-11-usar-cobra.md          Accepted
  ```
- Registrar `newADRListCmd()` em `newADRCmd()`.

**Adicionar teste** em `internal/generators/adr_test.go` (ou novo `adr_list_test.go`):
- `TestListADRs_Empty` — diretório ausente → sem panic, retorna mensagem amigável.
- `TestListADRs_WithFiles` — 2 ADRs criados → lista com título e status corretos.

## Restrições
- Stdlib Go + `golang.org/x/term` (para detecção TTY) + `huh` (já no go.mod).
- Nenhum novo framework ou dependência pesada.
- `go build ./...` sem erros após a mudança.
- `go test ./...` verde (incluindo os 14 testes existentes).

## Roadmap
Roadmap: roadmap-adr-wizard-e-list-2026-06-11
