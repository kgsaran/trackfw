---
title: "ship-roadmap-dir-default-divergencia"
tags: [ship, governance, parity, bug, config]
date: 2026-07-27
related: []
---

# ship-roadmap-dir-default-divergencia

## Problem

O gate de governança do `trackfw ship` usava defaults diferentes por runtime:

| Runtime | Default (bugado) |
|---|---|
| Go | `docs/roadmaps` (correto) |
| Node.js | `docs/roadmaps/claude` (errado) |
| Python | `docs/roadmaps/claude` (errado) |

Num projeto sem `roadmap_dir:` no `trackfw.yaml`, o gate passava no Go e bloqueava no
Node.js/Python — ou vice-versa, dependendo de onde o roadmap estava. Isso quebra
exatamente a promessa do produto: garantia de governança consistente entre runtimes.

O agravante: a seção "Known divergence — default `roadmap_dir`" foi adicionada ao
`docs/cli-parity.md` afirmando que *"This divergence is intentional and preserved"*.
Isso transformou um defeito em contrato e teria impedido qualquer correção futura.

## Root cause

Os runners de npm e PyPI implementaram uma função `resolveRoadmapDir()` local com o
default `docs/roadmaps/claude`, que era o layout por-agente descartado pela REQ anterior
(decisão D4: layout flat). Enquanto isso, os próprios módulos de config de npm e PyPI
(e o validator de ambos) já usavam `docs/roadmaps`. O runner duplicou a lógica com
o valor errado, e a duplicação não foi detectada porque os testes injetavam
`checkGovernance` como dependência, nunca exercitando o caminho real.

## Solution

1. Substituir a função `resolveRoadmapDir()` local (em ambos os runners) por delegação
   ao módulo de config existente (`loadConfig().roadmapDir` no npm,
   `_config.load()["roadmap_dir"]` no Python). Elimina a duplicação.
2. Migrar testes de integração de `docs/roadmaps/claude/wip/` para `docs/roadmaps/wip/`.
3. Remover a seção "Known divergence" do `cli-parity.md`.
4. Adicionar teste de paridade em cada runtime travando o default.

## Lição de processo

Quando um teste de integração ou um gate falha por divergência entre runtimes,
a hipótese padrão deve ser **bug**, não **contrato intencional**. Documentar
divergência como "intencional e preservada" sem evidência de decisão explícita
(sem ADR, sem REQ) é um anti-padrão: torna o defeito invisível para quem vier
depois. Se a divergência fosse realmente intencional, deveria estar registrada
em um ADR com justificativa, não apenas em um comentário de cli-parity.md.
