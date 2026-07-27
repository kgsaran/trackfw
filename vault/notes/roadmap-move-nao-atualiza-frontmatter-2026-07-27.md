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

## Workaround (até a correção)

Depois de `roadmap move`, editar o frontmatter no mesmo commit:

```
status: done
```

E a linha de cabeçalho `> Created: YYYY-MM-DD | Status: done`, se o roadmap a tiver.

## Fix pendente

O `move` deve reescrever `status:` para o estado de destino, **nos 3 CLIs** (Go, Node.js,
Python) — regra dura de paridade. Registrado como débito nº 5 no roadmap
`docs/roadmaps/done/ROADMAP-2026-07-27-robustez-dos-gates-de-governanca-e-paridade.md`;
candidato a REQ própria.

Ao corrigir, escrever o teste de falsificação junto (P4 de
`docs/gate-design-principles.md`): mover um roadmap e afirmar que `validate` fica **sem
warnings** — não basta afirmar que o arquivo mudou de pasta.
