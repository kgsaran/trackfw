---
title: OpenCode - "tools:" no frontmatter de agente e chave reservada e derruba o carregamento INTEIRO do projeto
date: 2026-08-05
tags: [opencode, integrations, render, frontmatter, breaking-behavior]
---

## Contexto

REQ `docs/req/REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`
(OpenCode como 10º target do catalogo de integracoes). Waves 1-4, PRs #134 e #135.

## Achado nao obvio

O frontmatter canonico dos agentes trackfw (`internal/integrations/assets/agents/*.md`) carrega
`tools:` como uma lista plana de nomes de ferramenta (`tools: Agent, Read, Edit, Write, Bash, ...`)
— formato compativel com Claude Code. No schema de agente do OpenCode (`.opencode/agents/*.md`),
`tools:` e uma **chave reservada** que espera um objeto de overrides por-ferramenta
(`tools: { bash: false }`), nao uma lista/string.

O ponto que nao e obvio: alimentar o formato lista nao falha silenciosamente so naquele campo, nem
so naquele agente. **O OpenCode recusa carregar o projeto inteiro** — nenhum agente, skill ou
comando customizado carrega, com o erro `Configuration is invalid at .../agents/<arquivo>.md`. Um
unico arquivo de agente mal formado derruba toda a integracao do OpenCode com aquele projeto, nao
so aquele agente especifico.

Isso descarta de saida qualquer estrategia de "reaproveitar o frontmatter original e so trocar o
que for incompativel campo a campo" — a reconstrucao tem que ser feita do zero, emitindo apenas os
campos que o OpenCode aceita.

## Como foi confirmado

Reproduzido experimentalmente contra o binario real `opencode` 1.18.13 (instalado na maquina),
duas vezes, em runtimes diferentes (Go e Python, na auditoria da Wave 3):
- Copiando o frontmatter canonico (com `tools:` lista) para `.opencode/agents/trackfw-backend.md`
  num projeto de teste isolado em `/tmp` → `opencode agent list` retorna `exit=1`,
  `Configuration is invalid`, zero agentes carregados (nem os que nao tinham `tools:`).
- Como controle diferencial (Python, Wave 3B): escrevendo o mesmo frontmatter quebrado (`tools:`
  lista) num segundo projeto `/tmp` isolado, confirmando o mesmo erro; e escrevendo o frontmatter
  reconstruido (so `description:` + `mode: subagent`) num terceiro projeto, confirmando
  `opencode agent list` exit=0 com o agente listado como `(subagent)`.
- Verificado tambem via `GET /agent` de um `opencode serve` rodando: o JSON resolvido do agente
  instalado tem `mode: "subagent"` e nenhuma chave `model` (ausente, nao `null`) — confirma que a
  omissao e honrada ate no runtime resolvido, nao so no template.

## Resolucao

`internal/integrations/render.go` (case `"opencode-agent"`, canonico), replicado byte-a-byte em
`npm/src/integrations/render.js` e `pypi/trackfw/integrations/renderers.py`: frontmatter
reconstruido do zero com apenas `description:` e `mode: subagent` (fixo, nunca omitido — sem ele
o OpenCode assume `mode: "all"`, tornando o agente selecionavel como persona primaria de chat).
`model:`/`tools:`/`memory:` sao omitidos deliberadamente (nao mapeados) — ver
`docs/cli-parity.md`, secao "OpenCode agent representation (`opencode-agent`)", para o raciocinio
completo de cada omissao.

## Licao para futuros targets de CLI

Ao integrar um novo AI-CLI ao catalogo, **testar o frontmatter canonico sem modificacao contra o
binario real primeiro**, antes de decidir se da para reaproveitar frontmatter ou se precisa
reconstruir do zero. Um campo com o mesmo nome em dois schemas diferentes (`tools:` aqui) pode ter
semantica incompativel a ponto de causar falha total, nao so degradacao daquele campo — o raio de
impacto de um frontmatter malformado varia por CLI (alguns ignoram campo desconhecido, outros
recusam o arquivo, o OpenCode recusa o projeto inteiro) e so o teste contra o binario real revela
isso.
