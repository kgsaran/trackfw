---
title: npm/src/validator/index.js é detectado como binário por `file`; grep -rln sem -a pula o arquivo em silêncio
tags: [node, grep, sweep, gotcha, by_agent]
date: 2026-08-29
related: [[gates-da-wave-sao-um-comando-por-linha-2026-08-29]]
---

## O defeito de sweep

`npm/src/validator/index.js` (3305 linhas — o maior arquivo do CLI Node, contém as 6 funções de regra
de `validate`: `resolveReqFiles`, `resolveStateDirs`, `validateWIPLimit`,
`validateFolderStatusCoherence`, `validateFilenameUniqueness`, `buildInventorySection`) é classificado
como **`data`** por `file(1)`:

```bash
file npm/src/validator/index.js   # → npm/src/validator/index.js: data
```

Motivo: o arquivo contém bytes não-ASCII em algum ponto (não identificado a fundo — não é BOM UTF-8
nem CRLF puro; suficiente para o heurístico de `file` desistir de classificar como texto).

**Consequência**: `grep -rln "padrão" npm/src/` **sem `-a`** trata o arquivo como binário e não emite
nenhuma linha correspondente — sem aviso, sem erro, o arquivo simplesmente não aparece na lista de
resultados. Rodei o sweep de `roadmap_namespacing: by_agent` nos 3 runtimes e o primeiro passe (`grep
-rln "by_agent\|byAgent\|BY_AGENT" npm/src/`) devolveu 10 arquivos — sem o `validator/index.js`, que é
exatamente onde vivem os pontos mais graves (as 3 regras de `validate` que decidem violação).

## Como percebi

Contagem batia com a nota do ADR ("11 sítios no Node") só depois de incluir `validator/index.js` — o
sweep sem `-a` fechava em 10 arquivos e ficava plausível o suficiente para não suspeitar. Só notei
porque fui confirmar `require('../validator')` em `commands/validate.js` e o arquivo importado não
aparecia em nenhum grep anterior.

## A forma correta

```bash
# Encontrar arquivos de código misclassificados como binário antes de confiar em grep -rl:
for f in $(find npm/src -name "*.js" -not -path "*/node_modules/*"); do
  file "$f" | grep -q "data" && echo "$f"
done
# → npm/src/validator/index.js, npm/src/integrations/doctor.js

# Sempre usar -a em sweeps sobre este diretório:
grep -a -rln "padrão" npm/src/
```

## Por que importa mais do que um grep perdido qualquer

O arquivo pulado é o que concentra as regras de `validate` — exatamente o lugar onde uma auditoria de
segurança sobre `roadmap_namespacing` precisa olhar primeiro. Um sweep que "fecha" em 10 arquivos sem
aviso de exclusão passa a impressão de enumeração completa quando não é. Vale como instância concreta
da instrução "vá pelo consumidor, não pelo padrão de texto, e justifique por que a lista fecha" — a
lista só fecha se o grep que a produziu não estiver descartando arquivos em silêncio.

## Recomendação

Qualquer sweep de segurança em `npm/src/` deve rodar o loop de detecção de binário acima primeiro, ou
usar `-a` por padrão em todo `grep -rl` sobre este diretório.
