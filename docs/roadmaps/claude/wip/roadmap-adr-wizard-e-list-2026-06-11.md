---
name: roadmap-adr-wizard-e-list-2026-06-11
status: wip
req: REQ-adr-wizard-e-list-2026-06-11
---

# Roadmap: ADR — Wizard Interativo e adr list

> Criado em: 2026-06-11 | Status: 🔄 WIP

## Diagnóstico / Contexto

O comando `trackfw adr new` atualmente cria um arquivo com seções vazias (placeholders HTML). O usuário preenche manualmente no editor. A evolução é tornar o preenchimento interativo via wizard `huh`, e adicionar `adr list` para visibilidade dos ADRs existentes.

**Restrição arquitetural crítica:** o wizard deve ficar no command layer (`commands/adr.go`); o generator deve ser puro/não-interativo (recebe struct). Isso protege os testes existentes de depender de stdin.

## Wave 1 — Implementação (ML único, sem dependências externas)

### ML-1A — Wizard + adr list (Apolo)

**Status:** ⬜ Pendente  
**Agente:** Apolo  
**Branch:** `feat/adr-wizard-e-list`

**Arquivos afetados:**
- `internal/generators/adr.go` — adicionar struct `ADRContent`, alterar assinatura de `NewADR`
- `internal/commands/adr.go` — adicionar wizard huh + fallback não-TTY + registrar `newADRListCmd()`
- `internal/generators/adr_test.go` — atualizar 2 testes existentes + adicionar testes de `adr list`

**Ações:**

1. Em `internal/generators/adr.go`:
   - Adicionar struct `ADRContent { Title, Context, Decision, Consequences, Alternatives string }`
   - Alterar `NewADR(title string)` → `NewADR(content ADRContent) error`
   - Usar `content.Context`, `content.Decision`, etc. no template (sem comentários HTML se preenchidos; manter comentários se vazio)

2. Em `internal/commands/adr.go`:
   - Importar `huh`, `golang.org/x/term`, `os`
   - Em `newADRNewCmd()`: após pegar `args[0]` como título, detectar TTY
   - Se TTY: rodar `huh.NewForm` com 4 campos `huh.NewText` para as seções
   - Se não-TTY: pular wizard, chamar `generators.NewADR(ADRContent{Title: title})`
   - Adicionar `newADRListCmd()` que lê `docs/adr/*.md`, extrai título e status, imprime tabela simples
   - Registrar ambos em `newADRCmd()`

3. Em `internal/generators/adr_test.go`:
   - Atualizar `TestNewADR_CreatesFile` e `TestNewADR_SlugInFilename` para `NewADR(ADRContent{Title: "..."})`
   - Adicionar `TestListADRs_Empty` e `TestListADRs_WithFiles`

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (≥16 testes, incluindo os 14 existentes)
- [ ] `trackfw adr new "título"` em TTY: mostra wizard com 4 campos e gera arquivo preenchido
- [ ] `trackfw adr new "título"` com stdin redirecionado (não-TTY): gera arquivo com seções vazias (comportamento atual)
- [ ] `trackfw adr list` sem `docs/adr/`: imprime "No ADRs found in docs/adr/" sem error
- [ ] `trackfw adr list` com ADRs existentes: lista título e status de cada um

**Comandos de validação:**
```bash
go build ./...
go test ./...
go vet ./...
```
