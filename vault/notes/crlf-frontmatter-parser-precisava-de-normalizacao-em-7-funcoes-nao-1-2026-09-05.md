---
title: "CRLF no frontmatter de agent-md: a fronteira de entrada não é uma função, são 7 por runtime — e o barrier já estava tolerante antes de qualquer fix"
tags: [crlf, windows, frontmatter, render, barrier, paridade, gotcha]
date: 2026-09-05
related: [[barrier-crlf-divergencia-node-regex-2026-08-29]]
---

## Sintoma

`ADR-2026-09-04-parser-de-frontmatter-tolera-crlf-na-fronteira-de-entrada` decidiu normalizar
CRLF→LF "na fronteira de entrada" (D1/D3), citando `TestRenderOpenCodeAgent` como sintoma e falando
em singular: "o parser de frontmatter". Na prática, em `internal/integrations/render.go` (e nos
espelhos Node/Python), **não existe um parser** — existem 7 funções independentes
(`markdownParts`, `insertBodyPrefix`, `rewriteSignatureLine`, `rewriteFrontmatterFields`,
`frontmatterName`, `rewriteFrontmatterModelLine`, `removeFrontmatterModelLine`), cada uma fazendo
seu próprio `strings.TrimSpace(string(source))` + `strings.HasPrefix(trimmed, "---\n")`.

## Armadilha para quem for corrigir isso de novo

Normalizar só a primeira função que a fixture de teste exercita **passa no teste** e deixa as
outras 6 quebradas — porque cada representação de `Render()` usa um subconjunto diferente dessas
funções:

- "opencode-agent"/"agent-directory"/"custom-agent-toml" (Rota A): só passam por `markdownParts`.
- "subagent"/"agent-markdown" com identidade (Rota B): encadeiam `markdownParts` + `insertBodyPrefix`
  + `rewriteFrontmatterFields` + `rewriteSignatureLine` — as 4 juntas.

Um teste de Rota A sozinho (o óbvio, o que a ADR cita) **não pega** a lacuna nas outras 6 funções.
Só um teste de Rota B com identidade configurada (`TestRenderSubagentRouteInjectsIdentity`-shaped)
exercita a cadeia completa. Isso aconteceu de verdade durante a implementação desta ML: a primeira
passada normalizou só `markdownParts`, o teste de Rota A ficou verde, e só o teste de Rota B pegou
o resto.

## G1-bis (barrier/gates) — já estava corrigido, por acidente de outra REQ

A re-triagem de 2026-09-04 (`docs/portabilidade/2026-09-04-retriagem-do-residuo-de-windows-por-mecanismo.md`)
registrou "G1-bis: CRLF cega o parser de gates do barrier, site não localizado, precisa de patch
adicional". Medido em 2026-09-05: **não precisava**. Toda comparação de linha em
`parseGates`/`_find_gates` (Go/Node/Python) passa por `TrimSpace`/`.trim()`/`.strip()`, que absorvem
um `\r` sobrando **independente** de `splitRoadmapLines`/`_split_roadmap_lines` — que, aliás, já
existiam nos 3 runtimes desde os commits `d4e286e` (29/08) e `fce709f` (01/09), por uma REQ
completamente diferente (dialeto do roadmap / gate `sh -c`). O teste Python que a re-triagem citou
como prova de falha (`test_barrier_cli_crlf_roadmap_gates_da_wave_e_reconhecido_e_comando_roda_e2e`)
**passa hoje, local** — mas o único dado de Windows real (CI run posterior a `fce709f`) registra
esse mesmo teste falhando. Não resolvido: não achei causa mundana em duas checagens rápidas
(cópia obsoleta em `pypi/build/lib/` não tem a função — mas o job faz `pip install pypi/` do fonte
antes de testar, então não deveria ser isso). **Lição**: quando um dado de CI e uma medição local
direta discordam e a medição local é sólida (falsificada nas duas direções), declarar a
discrepância em vez de escolher um lado por "a maioria bate".

## Python: universal-newlines mascara a função, não corrige ela

`open(path, "r")`/`Path.read_text()` fazem universal-newlines translation (`\r\n`→`\n`) ANTES do
parser existir — isso é comportamento do `io` do Python, não do SO, então vale igual em Windows. Por
isso "Python: 0 nesta forma" na re-triagem. Mas isso é uma propriedade do **caminho de leitura**, não
da **função de parsing** — `_parts`, testada isoladamente com uma string Python que já tem `\r\n`
embutido (sem passar por `open()`), diverge exatamente como Go/Node. E existe pelo menos um caminho
de produção real que NÃO passa por `open()`/`read_text()`: `manager.py::_frontmatter_name` recebe
`candidate.read_bytes()` + `.decode("utf-8")` — `.decode()` não faz universal-newlines. Esse é o
único sítio genuinamente load-bearing em Python nesta correção.

## Como falsificar isso localmente (macOS, sem Windows)

Nunca mute o asset em disco (`go:embed`/`read_text` são sempre LF no repo local) — injete CRLF no
`source []byte`/string EM MEMÓRIA depois de lido, e compare a saída de `Render()`/`render()` para
LF vs CRLF do mesmo conteúdo. Para falsificar que uma normalização é realmente chamada (não só que
existe), reverta cada call site individualmente com `sed` e rode só o teste afetado — não confie em
"a função existe" como prova de "a função é usada".
