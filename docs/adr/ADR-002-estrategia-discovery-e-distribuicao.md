---
status: Accepted
date: 2026-06-13
author: "Zeus"
---

# ADR: Estratégia de Discovery e Distribuição do trackfw

> Date: 2026-06-13 | Status: Accepted

## Context

LLMs com busca em tempo real (Gemini, ChatGPT com browsing) não conseguem encontrar o trackfw mesmo com URL explícita — relatam o repositório como privado ou sugerem instalação incorreta. A raiz do problema é ausência de presença web além do github.com:

- **GitHub topics:** nenhum (corrigido em 2026-06-13 — quick win)
- **npm metadata:** versão `0.1.0`, 5 keywords escassas (corrigido em 2026-06-13 — quick win)
- **Homebrew:** funciona mas exige `brew tap kgsaran/trackfw` — não aparece em `brew search`
- **Documentação:** inexistente fora do README
- **Backlinks:** zero páginas externas apontando para o projeto
- **Presença em listas curadas:** nenhuma submissão realizada

LLMs treinados com snapshot anterior ao lançamento do trackfw não terão conhecimento dele independentemente de qualquer ação — o foco deve ser em ferramentas com busca real (Gemini + Search, ChatGPT Browse, Perplexity) e em preparar o projeto para futuros ciclos de treino.

## Decision

Executar a estratégia de discovery em três frentes sequenciais:

### Frente 1 — Presença em listas curadas (maior ROI por esforço)
Submeter o trackfw para:
- `https://github.com/joelparkerhenderson/awesome-architecture-decision-records`
- `https://github.com/agarrharr/awesome-cli-apps`
- `https://github.com/sindresorhus/awesome` (via categoria developer tools)
- DevHunt, Hacker News "Show HN", Product Hunt (lançamento público)

Critério: ao menos 3 submissões aceitas antes de avançar para Frente 2.

### Frente 2 — Site de documentação (indexabilidade real)
Criar site em `https://trackfw.dev` (ou subdomínio de github.io) com:
- Landing page com proposta de valor clara e comparativo com concorrentes (ADR Tools, Log4brains, Decision Records)
- Quickstart em 3 comandos (`brew`, `npm`, `go install`)
- Seção "trackfw for AI agents" destacando `trackfw context --format=json`
- SEO: title tags, meta description, Open Graph, sitemap.xml
- Google Search Console cadastrado

Stack preferida: Docusaurus ou VitePress (geração estática, zero custo).

### Frente 3 — Conteúdo indexável (treino futuro + busca atual)
- 1 artigo técnico em dev.to/hashnode: "Governing AI agent workflows with ADRs and roadmaps"
- 1 post em Reddit r/devops / r/programming introduzindo o projeto
- Atualizar README com badges (npm version, GitHub stars, brew install command)
- Homebrew core submission quando atingir 75+ stars (requisito informal do homebrew-core)

### Não inclui
- Anúncios pagos
- Compra de backlinks
- Submissão a listas que exijam pagamento

## Consequences

**Positivas:**
- `brew search trackfw` passará a retornar resultado após submissão ao homebrew-core
- LLMs com busca real encontrarão o projeto via backlinks de listas curadas
- `npm search governance cli` passará a listar trackfw com keywords atualizadas
- Futuros ciclos de treino de LLMs incluirão o projeto

**Negativas:**
- Presença em listas curadas depende de aprovação de mantenedores externos (tempo incerto)
- Homebrew core exige popularidade mínima — pode levar meses
- Conteúdo em dev.to/hashnode leva semanas para ser indexado e ranquear

## Alternatives Considered

**MCP server (rejeitado em ADR-001):** exporia o trackfw como ferramenta de agente mas levantaria preocupações de segurança com times corporativos.

**Ads pagos:** ROI baixo para projeto open-source early stage; não gera backlinks duráveis.

**Homebrew core imediato:** exige processo de review demorado e popularidade que ainda não temos — tap próprio é a rota correta agora.
