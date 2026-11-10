---
name: bin-trackfw-not-refreshed-by-go-build-dot
description: go build ./... não atualiza bin/trackfw; check-gates-falsify e check-roadmap-barrier-contract usam bin/trackfw — rodar make build antes dos scripts
metadata:
  type: feedback
---

`go build ./...` compila todos os pacotes mas NÃO produz `bin/trackfw`. Os scripts `check-gates-falsify.sh` e `check-roadmap-barrier-contract.sh` usam `$ROOT_DIR/bin/trackfw` diretamente.

**Por que isso importa:** em ML-3A, o primeiro `check-gates-falsify.sh` falhou com `closed-cycle/go/mls-complete-passed` porque o `bin/trackfw` antigo ainda emitia `⬜ Pendente`. Só após `go build -o bin/trackfw ./cmd/trackfw` o binário refletiu as mudanças do gerador.

**How to apply:** sempre que modificar qualquer gerador, rodar `go build -o bin/trackfw ./cmd/trackfw` (ou `make build`) ANTES dos scripts de contrato. `go build ./...` é suficiente só para confirmar que o código compila.

Ligado a: [[make quality excede o timeout da tool]]
