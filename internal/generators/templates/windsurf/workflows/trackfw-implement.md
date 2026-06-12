# trackfw: Implementar Feature

Guia de implementação seguindo a cadeia de governança trackfw.

## Pré-requisitos

- REQ criada em `docs/requisições/`
- Roadmap em `docs/roadmaps/` com microlotes definidos
- Branch criada: `git checkout -b feat/<descricao>`

## Passos

1. Mover roadmap para `wip/`.
2. Para cada microlote (ML) do roadmap:
   a. Implementar as mudanças especificadas.
   b. Rodar build: `go build ./...` (ou o build do projeto).
   c. Rodar testes: `go test ./...`.
   d. Commitar: `git commit -m "feat(<escopo>): <descrição>"`.
   e. Marcar ML como ✅ no roadmap.
3. Mover roadmap para `done/` ao concluir todos os MLs.
4. Criar PR quando solicitado.
