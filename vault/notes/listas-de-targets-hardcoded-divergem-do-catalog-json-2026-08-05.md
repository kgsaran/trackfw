---
title: catalog.json e canonico, mas listas de targets hardcoded em consumidores divergem silenciosamente dele
date: 2026-08-05
tags: [integrations, catalog, paridade, targets, checklist]
---

## Contexto

REQ `docs/req/REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`
— adicionar OpenCode como 10º target ao catalogo. `internal/integrations/assets/catalog.json`
(espelhado byte-a-byte em `npm/` e `pypi/` via `scripts/sync-integration-assets.sh`) e a fonte
canonica dos targets, mas **nem todo consumidor deriva dele em tempo de execucao** — alguns modulos
mantem a propria lista fixa de ids de target, escrita a mao, que so fica correta se alguem lembrar
de atualiza-la manualmente sempre que um target for adicionado ou removido do catalogo.

## Achado nao obvio

Encontrado duas vezes na mesma REQ, em dois lugares diferentes, com o mesmo padrao:

1. **Harness de `update`** (`internal/generators/update.go:harnessCatalogTargetOrder`,
   `pypi/trackfw/commands/update_harness.py:_CATALOG_TARGET_ORDER`) — lista fixa de 9 ids
   (`claude` ... `kiro`), documentada como "pinned list" em `docs/cli-parity.md` por design (nao
   deve ser derivada de filesystem nem reordenada). A Wave 2 (Go) atualizou `catalog.json` para 10
   targets mas nao tocou essa lista separada — `check-update-parity.sh` (Wave 3) pegou a
   divergencia porque o Node.js (`npm/src/commands/update_harness.js` ou equivalente) **ja** deriva
   a lista dinamicamente do catalogo, entao so Go e Python ficaram desatualizados, nao os 3.
2. **Site publico** (`site/guide/commands.md`, `site/en/guide/commands.md`) — frase solta
   "Targets suportados: claude, codex, ..., amazonq e kiro" escrita a mao em prosa Markdown, sem
   nenhuma ligacao mecanica com o catalogo. Escapou das 4 waves da REQ e das duas auditorias
   (Zeus, apos Wave 3 e apos Wave 4) porque `site/` roda um workflow proprio
   (`.github/workflows/deploy-docs.yml`), fora do `make quality`/`make parity` — nenhum gate
   automatizado compara o conteudo do site com o catalogo.

O ponto comum: `catalog.json` ter um contrato de paridade byte-a-byte entre os 3 runtimes (via
`check-integration-assets.sh`) **nao garante** que todo consumidor derive dele — cada lista
hardcoded e um ponto de divergencia silenciosa independente, com seu proprio raio de blast (harness
de update quebra uma feature real; site so fica com doc desatualizada, mas nenhum dos dois emite
erro).

## Como foi confirmado

`check-update-parity.sh` (achado #1) pegou a divergencia automaticamente via diff de JSON entre
os 3 runtimes rodando o harness de verdade. O achado #2 (site) so apareceu por uma varredura manual
`grep -rlE "amazonq.*kiro|kiro.*amazonq"` no repo inteiro, feita a pedido do usuario apos a REQ ja
estar fechada e mergeada (PRs #134 e #135) — nao foi pego por nenhum gate automatizado nem pela
auditoria padrao de Zeus (build+test+gate+leitura de diff), porque a auditoria padrao so cobre os
arquivos que os agentes tocaram, nao uma varredura pelo repo inteiro atras do mesmo padrao em outros
lugares.

## Resolucao

Achado #1: `opencode` inserido entre `amazonq` e `kiro` nas duas listas fixas (ordem real do
catalogo), contagem 19→21 ids atualizada nos comentarios e em `docs/cli-parity.md`. Achado #2:
`opencode` adicionado as duas paginas do site (PT e EN).

## Licao para o checklist de "adicionar um novo target ao catalogo"

Ao adicionar ou remover um target de `catalog.json`, rodar uma varredura pelo repo inteiro por
listas hardcoded de ids de target **antes** de considerar a REQ fechada, nao confiar so em
`make quality`/`make parity` passarem (eles so cobrem o que ja tem gate escrito) nem so na
auditoria diff-por-diff dos arquivos tocados pelos agentes. Comando usado nesta sessao:
`grep -rlE "amazonq.*kiro|kiro.*amazonq" --include="*.go" --include="*.js" --include="*.py"
--include="*.sh" --include="*.md" .` (ajustar o par de targets vizinhos conforme a ordem real do
catalogo). Vale considerar formalizar isso como um ML fixo em toda REQ futura de "adicionar target",
ja que o padrao se repetiu duas vezes so nesta REQ.
