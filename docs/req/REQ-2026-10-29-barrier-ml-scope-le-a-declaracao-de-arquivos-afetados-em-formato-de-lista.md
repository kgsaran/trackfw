---
status: Open
date: 2026-10-29
author: "zeus (trackfw_architect), decisão do KG"
adr: "docs/adr/ADR-2026-10-29-barrier-ml-scope-le-a-declaracao-em-lista-e-une-os-roadmaps-tocados-pela-branch.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-29-barrier-ml-scope-le-a-declaracao-de-arquivos-afetados-em-formato-de-lista.md"
---

# REQ: barrier ml_scope lê a declaração de arquivos afetados em formato de lista

> Date: 2026-10-29 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

O `trackfw barrier` reprovou a Wave 1 do roadmap de documentação 11.x (PR #627) por dois defeitos do `ml_scope`, medidos
em 2026-10-29:

1. **Declaração em lista lida como vazia.** `parseMLScopeFilesAffected` (`internal/commands/barrier.go`) para no primeiro
   item `- `. Nos roadmaps com data ≥ 2026-07-18 do próprio repositório, **191 de 836 declarações, em 62 de 169
   roadmaps**, estão em lista. Contagem feita com a lógica da regra (rótulo sem crase seguido de item de lista), não por
   grep.
2. **Escopo de um roadmap só.** Com duas REQs na mesma branch, o barrier de uma reprova os arquivos da outra. Medido: a
   REQ e o roadmap desta correção já aparecem como `out of scope` no barrier do roadmap de documentação.

Os dois juntos fazem o barrier reprovar trabalho legítimo, o que é exatamente o atrito de adoção que a direção atual quer
reduzir (Discussion #525). Decisão do KG em 2026-10-29: corrigir no mesmo PR, com escopo como união dos roadmaps da branch.

## Acceptance Criteria

- [ ] AC1. Uma declaração `**Files affected:**` seguida de lista produz os padrões dos itens, segundo a regra de crases
  definida na Wave 0. Teste com nome conferido por `go test -run <nome> -v`.
- [ ] AC2. A declaração termina em linha em branco, campo `**`, cabeçalho `#` ou cerca. Teste em que a lista de
  `**Acceptance criteria:**` depois do rótulo **não** entra no escopo.
- [ ] AC3. A forma em linha continua igual: os casos de `TestMLScopeFilesAffectedParsing` passam sem alteração de
  expectativa.
- [ ] AC4. Com dois roadmaps tocados pela branch, o barrier de cada um aceita os arquivos declarados pelo outro e a
  orquestração do outro (roadmap, REQ, ADR). Teste com repositório git em `t.TempDir()`.
- [ ] AC5. Um roadmap que **não** aparece no diff da branch não contribui com escopo (contra-braço do AC4).
- [ ] AC6. Arquivo fora de todas as declarações continua `blocked` (contra-braço geral).
- [ ] AC7. Seção `ml_scope` de `docs/cli-parity.md` descreve as duas regras; os `trackfw-contract` da seção apontam para os
  testes que as afirmam.
- [ ] AC8. Com o binário da branch, `trackfw barrier` passa no `ml_scope` da Wave 1 do roadmap de documentação 11.x e da
  Wave 1 deste roadmap, sem reescrever nenhuma declaração existente.
- [ ] AC9. `go test ./internal/commands/` verde e `scripts/check-barrier.sh` verde.

## Negative scope

- Não muda a base do diff, o corte de 2026-07-18, nem a leitura de `HEAD` em vez do disco.
- Não muda o gerador de roadmap (`internal/generators/roadmap.go`): o rótulo vazio segue igual.
- Não reescreve nenhum roadmap existente para a forma em linha.
- Não muda o vínculo escrito branch↔roadmap (`.trackfw-branch-links.json`) nem a resolução de `commit`/`push`/`ship`.
- Não cria regra nova de `validate`.

## Linked ADR
ADR: docs/adr/ADR-2026-10-29-barrier-ml-scope-le-a-declaracao-em-lista-e-une-os-roadmaps-tocados-pela-branch.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-29-barrier-ml-scope-le-a-declaracao-de-arquivos-afetados-em-formato-de-lista.md
