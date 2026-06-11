# Roadmap: Testes Unitários Go — validator e generators

> Criado em: 2026-06-11 | Status: WIP | Agente: Artemis
> REQ: REQ-testes-unitarios-go-2026-06-11

## Contexto

O projeto trackfw não possui testes automatizados. Esta tarefa cobre os pacotes
`internal/validator` e `internal/generators` com testes unitários Go usando
apenas stdlib, TempDir e Chdir para isolamento de filesystem.

## Wave 1 — Implementação dos 3 arquivos de teste

### ML-1A — validator_test.go
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/validator/validator_test.go`
**Ações:** 7 funções de teste cobrindo todas as regras do validator

### ML-1B — roadmap_test.go
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/generators/roadmap_test.go`
**Ações:** 5 funções de teste para NewRoadmap, MoveRoadmap e containsIgnoreCase

### ML-1C — adr_test.go
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/generators/adr_test.go`
**Ações:** 2 funções de teste para NewADR e slug

## Critérios de Aceite
- [x] `go test ./internal/validator/... ./internal/generators/... -v` passa sem falhas
- [x] Todos os 14 testes planejados implementados e verdes

## Resultado Final
14/14 testes passaram. Duração: validator 0.427s, generators 0.644s.
