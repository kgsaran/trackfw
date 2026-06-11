# REQ: Testes Unitários Go — validator e generators

> Criado em: 2026-06-11 | Status: WIP | Agente: Artemis

## Solicitação

Escrever testes unitários Go para os pacotes `internal/validator` e `internal/generators` do projeto trackfw.

## Escopo

### Arquivo 1: `internal/validator/validator_test.go`
- `TestValidate_Clean` — estrutura vazia sem violações
- `TestValidate_WIPMissingREQ` — roadmap em wip sem "REQ:" → 1 violation
- `TestValidate_WIPMissingAcceptanceCriteria` — roadmap em wip com REQ mas sem critérios → 1 violation
- `TestValidate_MultipleWIP` — 2 roadmaps em wip → 1 warning
- `TestValidate_REQMissingADR` — req sem "ADR:" → violation
- `TestValidate_BlockedMissingREQ` — roadmap em blocked sem REQ → violation
- `TestGetStatus_Empty` — sem arquivos → retorna string sem panic

### Arquivo 2: `internal/generators/roadmap_test.go`
- `TestNewRoadmap_CreatesFile`
- `TestMoveRoadmap_Valid`
- `TestMoveRoadmap_InvalidState`
- `TestMoveRoadmap_NotFound`
- `TestContainsIgnoreCase`

### Arquivo 3: `internal/generators/adr_test.go`
- `TestNewADR_CreatesFile`
- `TestNewADR_SlugInFilename`

## Restrições
- Apenas stdlib Go (testing, os, path/filepath, strings)
- TempDir + Chdir para isolamento
- Package white-box para cada pacote

## Roadmap
Roadmap: ROADMAP-testes-unitarios-go-2026-06-11
