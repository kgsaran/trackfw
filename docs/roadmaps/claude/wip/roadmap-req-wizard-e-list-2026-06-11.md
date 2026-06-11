---
name: roadmap-req-wizard-e-list-2026-06-11
status: wip
req: REQ-req-wizard-e-list-2026-06-11
---

# Roadmap: REQ — Wizard Interativo e req list

> Criado em: 2026-06-11 | Status: 🔄 WIP

## Diagnóstico / Contexto

O comando `trackfw req new` cria arquivo com seções vazias. Evolução espelha o que foi feito para `trackfw adr` (PR #1): wizard interativo nas seções + `req list`.

**Restrição arquitetural:** wizard no command layer; generator recebe `REQContent` struct (sem I/O).

## Wave 1 — Implementação (ML único)

### ML-1A — Wizard + req list (Apolo)

**Status:** ⬜ Pendente
**Agente:** Apolo
**Branch:** `feat/req-wizard-e-list`

**Arquivos afetados:**
- `internal/generators/req.go` — struct `REQContent`, assinatura `NewREQ(REQContent)`, `ListREQs`, `parseREQMeta`
- `internal/commands/req.go` — wizard huh + fallback não-TTY + `newReqListCmd()`
- `internal/generators/req_test.go` — arquivo novo, 7 testes

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde (≥27 testes)
- [ ] `go vet ./...` limpo
- [ ] `trackfw req new` em TTY: wizard com 4 campos
- [ ] `trackfw req new` não-TTY: arquivo com placeholders
- [ ] `trackfw req list` sem `docs/req/`: mensagem amigável
- [ ] `trackfw req list` com REQs: lista filename e status

**Comandos de validação:**
```bash
go build ./...
go test ./...
go vet ./...
```
