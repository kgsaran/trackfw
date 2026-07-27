---
title: "roadmap-move-nao-atualiza-frontmatter"
tags: [roadmap, governance, dod, validator, bug, parity]
date: 2026-07-27
related: [branch_has_wip_roadmap-conflita-com-a-definition-of-done-2026-07-26]
---

# roadmap-move-nao-atualiza-frontmatter

## Problem

`trackfw roadmap move <nome> done` move o arquivo de pasta mas **não reescreve o
`status:` do frontmatter**. O resultado imediato:

```
$ trackfw roadmap move ROADMAP-... done
✓ moved ROADMAP-....md → docs/roadmaps/done

$ trackfw validate
⚠  roadmap "ROADMAP-....md": folder is "done" but status declares "wip"
```

O comando que existe para cumprir a Definition of Done produz um estado que o próprio
validador acusa. Quem encerra um roadmap pelo caminho oficial recebe um warning e tem
que editar o frontmatter na mão — sem que nada avise que esse segundo passo existe.

## Root cause

A pasta é a fonte de verdade do estado (ADR-036), e a regra `folder_status` compara
pasta × frontmatter. O `move` implementa só metade do contrato: reposiciona o arquivo e
delega ao humano a sincronização do campo que ele acabou de invalidar.

## Impact

Não é cosmético. É o mesmo formato do defeito D4 desta REQ
([[branch_has_wip_roadmap-conflita-com-a-definition-of-done-2026-07-26]]): **a ferramenta
pune quem segue o processo que ela mesma prega**. Nos dois casos o agente que cumpre a DoD
vê o validador reclamar, e a saída natural é achar que o processo está errado — ou pior,
parar de mover roadmaps.

## ✅ CORRIGIDO — 2026-07-27

Resolvido pela `REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato`. `move` agora reescreve
`status:` do frontmatter **e** a linha de cabeçalho `> Created: … | Status: …`, nos 3 CLIs, gravando
bytes idênticos (valor minúsculo, igual ao nome do estado).

A reescrita é **escopada ao bloco de frontmatter** — um `status:` no corpo do documento não é tocado.
Isso importa porque a implementação parcial que o Python tinha usava `re.sub` com `MULTILINE` sem
escopo, e corrompia qualquer roadmap que mencionasse `status:` numa tabela ou bloco de código.

O workaround abaixo **não é mais necessário**. Mantido apenas como registro histórico.

## Workaround (histórico — não use)

Depois de `roadmap move`, editar o frontmatter no mesmo commit:

```
status: done
```

E a linha de cabeçalho `> Created: YYYY-MM-DD | Status: done`, se o roadmap a tiver.

## Correção aplicada

Implementado em `rewriteRoadmapStatus` (Go, `internal/generators/roadmap.go`), `rewriteRoadmapStatus`
(Node, `npm/src/generators/roadmap.js`) e `_rewrite_roadmap_status` (Python,
`pypi/trackfw/generators/roadmap.py`) — as três espelhando a semântica de `rewriteFrontmatterFields`.

O teste P4 existe nos 3 CLIs: roda `validate` **depois** do `move` e prova ausência do warning. Node
ganhou sua primeira suíte de `moveRoadmap` (`npm/tests/roadmap_move.test.js`) — a ausência total de
teste ali era o que permitia o defeito sobreviver no runtime Node.

## Débito remanescente

A correção resolveu o `move`, mas a investigação expôs cinco divergências adjacentes entre os 3 CLIs,
todas registradas no escopo negativo da REQ e ainda abertas:

1. `roadmap new` do Python gera frontmatter divergente de Go/Node (`name`/`title`/`created`/`author`
   vs `date`/`req`/`squad`, header PT-BR com emoji) — **a maior divergência de paridade do repo hoje**
2. Estado `analyzing` não é movível por nenhum CLI, apesar de válido no scaffold e no `folder_status`
3. `parse_frontmatter` do Python não remove aspas → `status: "wip"` avisa em Python, passa em Go/Node
4. `findRoadmap` do Go retorna o primeiro match sem erro de ambiguidade e não filtra `.md`
5. Log de transição em `by_agent`: Go e Node prefixam o agente, Python não
