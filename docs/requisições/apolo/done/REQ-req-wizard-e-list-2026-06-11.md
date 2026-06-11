# REQ: REQ — Wizard Interativo nas Seções e req list

> Criado em: 2026-06-11 | Status: WIP | Agente: Apolo

## Solicitação

Evoluir o comando `trackfw req` com dois novos comportamentos, espelhando exatamente o padrão já implementado em `trackfw adr` (PR #1):
1. **Wizard interativo**: `req new` pergunta Motivation, Acceptance Criteria, Linked ADR e Linked Roadmap via terminal antes de gerar o arquivo.
2. **req list**: novo subcomando que lista REQs existentes com título e status.

## Escopo

### Mudança 1 — Wizard interativo (`req new`)

**Arquitetura obrigatória (mesma do ADR):**
- O wizard `huh` fica **exclusivamente em `internal/commands/req.go`** (command layer).
- O generator `internal/generators/req.go` passa a receber uma struct `REQContent` com os campos preenchidos — nunca faz I/O.

**Struct a criar em `internal/generators/req.go`:**
```go
type REQContent struct {
    Title        string
    Motivation   string
    Criteria     string  // acceptance criteria como texto livre
    LinkedADR    string
    LinkedRoadmap string
}
```

**Assinatura do generator:**
```go
func NewREQ(content REQContent) error
```

**Template gerado:** se campo preenchido, inserir conteúdo; se vazio, manter placeholder HTML original.

**Wizard em `internal/commands/req.go`** (usar `huh.NewText` para campos multiline):
- "Motivation" — `Description("Why is this requirement needed? What problem does it solve?")`
- "Acceptance Criteria" — `Description("List acceptance criteria, one per line")`
- "Linked ADR" — `huh.NewInput().Title("Linked ADR").Description("ADR filename or slug (leave blank if none)")`
- "Linked Roadmap" — `huh.NewInput().Title("Linked Roadmap").Description("Roadmap filename or slug (leave blank if none)")`

**Fallback não-TTY:** detectar via `charmbracelet/x/term` (já no go.mod), chamar `generators.NewREQ(REQContent{Title: title})` com campos vazios.

### Mudança 2 — `req list`

**Subcomando:** `trackfw req list`

**Comportamento (espelhar `ListADRs` do generators/adr.go):**
- Ler `docs/req/*.md` via `filepath.Glob`.
- Se ausente/vazio: `"No REQs found in docs/req/"`, retornar nil.
- Extrair título da linha `# REQ: <título>` e status da linha `> Date: … | Status: <status>`.
- Output: `fmt.Printf("%-60s %s\n", filename, status)`.
- Registrar `newReqListCmd()` em `newReqCmd()`.

Reutilizar ou referenciar `parseADRMeta` como padrão — criar `parseREQMeta` análogo em `internal/generators/req.go`.

### Mudança 3 — Testes em `internal/generators/req_test.go` (arquivo novo)

- `TestNewREQ_CreatesFile` — `NewREQ(REQContent{Title: "My Req"})` cria arquivo em `docs/req/`
- `TestNewREQ_SlugInFilename` — slug correto no nome do arquivo
- `TestNewREQ_WithContent` — campos preenchidos aparecem no arquivo gerado
- `TestNewREQ_EmptyFields` — campos vazios geram placeholders HTML
- `TestListREQs_Empty` — sem `docs/req/` → mensagem amigável, sem erro
- `TestListREQs_WithFiles` — 2 REQs criados → lista com filename e status
- `TestListREQs_ParsesMeta` — extração correta de título e status

## Restrições
- Stdlib Go + `charmbracelet/x/term` (já no go.mod) + `huh` (já no go.mod).
- `go build ./...` sem erros.
- `go test ./...` verde (incluindo os 20 testes existentes).

## Roadmap
Roadmap: roadmap-req-wizard-e-list-2026-06-11
