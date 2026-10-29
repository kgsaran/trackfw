---
status: Proposed
date: 2026-10-29
author: "zeus (trackfw_architect), decisão do KG"
---

# ADR: barrier ml_scope lê a declaração em lista e une os roadmaps tocados pela branch

> Date: 2026-10-29 | Status: Proposed

Revisa a D2 do `ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md`
em dois pontos: a gramática de `**Files affected:**` e o conjunto de roadmaps que formam o escopo da branch. O resto da D2
continua valendo: roadmap lido de `git show HEAD:`, base `merge-base(origin/main, HEAD)`, corte de 2026-07-18, fail-closed
sem `origin/main`.

## Context

Medido em 2026-10-29 na branch `feat/documentacao-reflete-produto-11-windows-install-ps1` (PR #627):

1. **A declaração em lista é lida como vazia.** `parseMLScopeFilesAffected` (`internal/commands/barrier.go`) encerra a
   declaração no primeiro item `- ` ou `* `. O contrato em `docs/cli-parity.md` (seção `ml_scope`, gramática) diz o mesmo.
   O gerador (`internal/generators/roadmap.go`) emite o rótulo vazio, e quem preenche quase sempre usa lista. Nos roadmaps
   com data ≥ 2026-07-18, **191 de 836 declarações, em 62 de 169 roadmaps**, estão em lista. Todas resultam em escopo zero.
   O barrier da Wave 1 do roadmap de documentação 11.x reprovou os 10 arquivos que os dois MLs declararam.
2. **O escopo vem de um roadmap só.** A branch carrega duas REQs (a de documentação e esta), como a prática de
   "uma branch por objetivo" permite. O barrier do roadmap de documentação já reprova a REQ e o roadmap desta correção, e
   vai reprovar `internal/commands/barrier.go`. O barrier desta REQ reprovaria o `install.ps1`.

Nenhum dos dois casos tem teste: `TestMLScopeFilesAffectedParsing` cobre só a forma em linha.

## Decision

1. **Lista faz parte da declaração.** Os itens de lista que vêm logo depois do rótulo `**Files affected:**` (ou
   `**Arquivos afetados:**`) pertencem a ela. A declaração termina no primeiro dos seguintes: linha em branco, novo campo
   em negrito (`**`), cabeçalho (`#`) ou cerca de código. A regra de quais crases de cada item viram padrão é definida pela
   Wave 0. Candidatos: só a primeira crase do item, ou todas. Ela precisa resolver o caso medido no ML-1B do roadmap de
   documentação, em que `pypi/pyproject.toml` aparece num item que diz que o arquivo **fica**. A forma em linha continua
   como está.
2. **O escopo da branch é a união dos roadmaps que ela toca.** Um roadmap entra na união quando aparece no diff
   `merge-base(origin/main, HEAD)..HEAD`, existe em `HEAD` e está numa pasta de estado do `roadmap_dir`. Cada roadmap
   contribui com:
   - seus `Files affected`, com o recorte de waves definido pela Wave 0 (o alvo do barrier contribui com as waves 0..N);
   - seu conjunto de orquestração (o próprio arquivo, REQ, ADR).

   **Por que é a mesma confiança de hoje:** a D2 revisada já confia no conteúdo de `HEAD` porque só o arquiteto commita.
   Um roadmap a mais em `HEAD` foi commitado pela mesma autoridade que commita edições no roadmap-alvo.
3. **Contrato e gerador acompanham.** A seção `ml_scope` de `docs/cli-parity.md` descreve as duas regras, com os
   `trackfw-contract` atualizados. O gerador não muda de formato: o rótulo vazio seguido de lista passa a ser lido.

## Consequences

- **Positivas:** os 62 roadmaps em lista deixam de reprovar tudo. Uma branch com várias REQs passa no barrier sem que um
  roadmap precise declarar os arquivos do outro.
- **Negativas:**
  - A união amplia o que a branch pode tocar. Um roadmap commitado com um glob largo (`**`) amplia o escopo de todos os
    outros na mesma branch. Isso já é possível hoje dentro do roadmap-alvo, e a Wave 0 avalia se é preciso rejeitar
    padrões que cobrem o repositório inteiro.
  - Itens de lista com prosa carregam crases que não são arquivos (`extra_files`, nomes de função). Um token que não é
    caminho não casa arquivo nenhum, mas um token que é caminho e aparece só como menção amplia o escopo.
- **Residual:** o barrier continua confiando no que o arquiteto commitou. Ele não protege contra um arquiteto que declara
  escopo largo de propósito.

## Alternatives Considered

- **Um roadmap por branch, com a correção como ML do roadmap de documentação:** rejeitada pelo KG em 2026-10-29. Mistura
  as causas na mesma REQ e mantém o defeito para qualquer branch com mais de uma REQ.
- **Escopo pelos commits que citam o ML:** rejeitada. Depende de convenção na mensagem de commit, e o diff da branch
  inteira deixa de ser verificado.
- **Exigir declaração em linha e reescrever os 62 roadmaps:** rejeitada. O gerador e a prática produzem lista, e o
  formato natural do Markdown não deveria reprovar.
