---
title: União agents:+disco (REQ-2026-08-29) desarma gates de falsificação que discriminavam por presença/ausência — só ORDEM sobrevive
tags: [gate, falsify, by_agent, roadmap_namespacing, config, ordering, vault]
date: 2026-08-29
related: [[armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12]]
---

## O que mudou e por que isso quebra gates antigos

`REQ-2026-08-29-namespace-de-agente-nao-declarado...` fez `agents:` **complementar** o disco em vez de
**substituí-lo** (`resolveAgentNamespaces`/`resolve_agent_namespaces`, ML-1A). Antes: se `agents:` era
declarada, o disco nunca era lido — um namespace em disco e não declarado ficava invisível. Depois: o
disco é **sempre** lido, união com o declarado.

Consequência para `scripts/check-gates-falsify.sh`: **todo cenário cujo braço de detecção verificava
"o item X aparece ou desaparece da saída" para provar que `agents:` foi corretamente lido ficou vácuo**
— porque agora X aparece **de qualquer forma**, tenha o parser de `agents:` funcionado ou não, desde
que X exista fisicamente em disco. Dois cenários deste repositório quebraram assim:

- **Cenário 34** (`config-unindented-agents`): baseline pinado assumia `zeus` invisível (era
  literalmente o defeito que a REQ corrige). Com a união, `zeus` passa a aparecer sempre — a asserção
  `grep -qF "[zeta]"` no braço de detecção nunca mais falha, porque `[zeta]` aparece no baseline
  correto TAMBÉM agora.
- **Cenário 35** (`config-inline-comma-in-quotes`): pior ainda — o diretório físico de teste
  (`docs/roadmaps/ka, tsu/`) tem o MESMO nome literal que o item declarado em `agents:`. A união
  encontra esse namespace via varredura de disco **mesmo que a atribuição de `cfg.Agents` seja
  completamente apagada** — a propriedade que o cenário existia para provar (parsing correto de vírgula
  dentro de aspas) ficou estruturalmente impossível de discriminar por presença/ausência.

## A técnica de retarget que sobrevive: ORDEM, não presença

O resolvedor devolve **agentes declarados primeiro** (na ordem de `agents:`, deduplicada), seguidos dos
**extras encontrados só em disco** (ordem alfabética). Essa é uma propriedade que ainda diverge entre
"parser leu `agents:` corretamente" e "parser descartou `agents:` (lista vira vazia, cai para
alfabético puro)" — **desde que a fixture seja desenhada para que a ordem declarada NÃO coincida com a
ordem alfabética**. Cenário 34 foi corrigido invertendo o item declarado (`agents: [zeus]`, disco tem
`apolo`+`zeus` — declarado-primeiro dá `[zeus, apolo]`, alfabético dá `[apolo, zeus]`, ordens diferentes
→ discriminante). Cenário 35 precisou de reordenação mais elaborada (`agents: ["obi", "ka, tsu"]`, com
`obi` ganhando um roadmap próprio para não ser filtrado da saída) para que a mesma técnica funcionasse.

**Armadilha ao aplicar esta técnica**: se o item declarado por acaso já é o primeiro em ordem
alfabética entre os candidatos (ex.: fixture original do Cenário 35, onde tanto a ordem declarada
quanto a alfabética colocavam "ka, tsu" antes de "zeta"), a fixture parece discriminante mas não é —
`corrupt_literal` corrompe a implementação e o teste "passa" só porque a saída corrompida também bate
com o padrão esperado por coincidência de ordenação, não porque a corrupção foi detectada. **Sempre
verificar empiricamente**, rodando o braço de detecção contra a implementação REALMENTE corrompida,
que a saída diverge do baseline pela razão esperada (aqui, ordem invertida) — não assumir que reordenar
o fixture "deveria" funcionar.

## Lição para quem tocar `roadmap_namespacing: by_agent` de novo

Antes de escrever ou revisar um cenário de `check-gates-falsify.sh` que envolva `agents:` + disco,
pergunte: **"esta asserção ainda discrimina depois da união (REQ-2026-08-29)?"** Se a asserção é
presença/ausência de um item, e esse item existe (ou pode existir) fisicamente em disco, a resposta é
não — a união o torna sempre visível. A única classe de propriedade que sobrevive de forma confiável é
**ordem relativa** entre itens declarados e itens só-disco, e mesmo essa exige desenhar a fixture para
que as duas ordens (declarada vs. alfabética) genuinamente divirjam.

## Onde isso ainda está pendente

`config-inline-comma-in-quotes` (Cenário 35), após o retarget, prova apenas "`agents:` inline é lido",
não mais "vírgula dentro de aspas é tratada como um único item" — essa propriedade específica (caso 8
do contrato de 9 casos) está coberta só por teste unitário por CLI, não por gate cross-CLI. Registrado
como dívida para `ML-3A`/`artemis-tf` (Wave 3 desta REQ), que já possui mandato para criar
`scripts/check-agent-namespace-union.sh` — um cenário novo e dedicado a "vírgula dentro de aspas +
união" resolveria a lacuna sem sobrecarregar o Cenário 35 histórico.
