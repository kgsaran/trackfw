---
status: Proposed
date: 2026-08-04
author: "Zeus"
---

# ADR: req move/list — reusar roadmap_namespacing para REQ e mover fisicamente o arquivo

> Date: 2026-08-04 | Status: Proposed

## Context

`REQ-2026-08-03-req-move-list-nao-suportam-subpastas-e-req-move-nao-move-arquivo.md` documenta dois
defeitos confirmados em `internal/generators/req.go`:

1. `ListREQs` (`filepath.Glob(dir/*.md)`) e `findREQ` (`os.ReadDir(dir)`, pulando diretórios) só
   enxergam REQs soltas direto em `REQDir/` — nunca em `REQDir/<estado>/` nem em
   `REQDir/<agente>/<estado>/`. `trackfw context` já usa um scanner recursivo diferente e enxerga os
   mesmos arquivos, o que torna a divergência difícil de diagnosticar em campo.
2. `MoveREQ` só reescreve `status:` no frontmatter, sem `os.Rename`/`os.MkdirAll` — o arquivo nunca sai
   fisicamente de onde estava. Isso diverge de `MoveRoadmap` (`internal/generators/roadmap.go:337`), que
   resolve o diretório-alvo via `stateDir`/`agentStateDir`, move com `os.Rename`, sincroniza `status:` no
   destino e registra a transição em `.trackfw-log`.

O schema de config (`internal/config/config.go`) modela `RoadmapNamespacing` (`"flat"` default ou
`"by_agent"`, ver `agentStateDir`/`stateDir` em `roadmap.go:30-53`) mas não tem campo equivalente para
REQ. Nada impede — nem a documentação avisa — que um projeto organize `req_dir` no mesmo padrão do
roadmap (é o que aconteceu na sessão real do CMDB que originou a REQ). Duas decisões de design precisam
ser tomadas antes da implementação: (a) REQ reusa `roadmap_namespacing` ou ganha campo próprio
`req_namespacing`; (b) sob quais condições `req move` passa a mover fisicamente o arquivo, preservando o
comportamento legado onde REQs vivem soltas em `REQDir/`.

## Decision

**D1 — REQ reusa `config.RoadmapNamespacing`, sem campo novo no schema.** Não existe caso de uso
conhecido em que um projeto queira `roadmap_namespacing: by_agent` e REQs em layout flat (ou vice-versa)
— REQ e Roadmap são artefatos do mesmo fluxo de governança (`ADR → REQ → ROADMAP`) e tendem a seguir a
mesma convenção de organização por time/agente. Introduzir `req_namespacing` duplicaria uma decisão de
config sem benefício demonstrado, na direção oposta à Regra de Paridade e ao princípio de "menor
superfície de config possível" já seguido pelo projeto (ADR-036, unificação do loader de
`trackfw.yaml` em `d9380c0`). Se um caso real exigir namespacing independente entre REQ e Roadmap,
essa é matéria de ADR sucessor com evidência de campo — não uma antecipação especulativa aqui.

**D2 — `req list`/`findREQ` ganham busca recursiva nos três layouts, espelhando `findRoadmap`.**
Passam a resolver, na ordem: `REQDir/*.md` (flat legado), `REQDir/<estado>/*.md` (por-estado, sem
namespacing por agente) e `REQDir/<agente>/<estado>/*.md` (by_agent, reusando `cfg.RoadmapNamespacing`
via D1). A lógica de descoberta é adaptada de `findRoadmap`/`roadmapStateOrder`
(`internal/generators/roadmap.go:406-440`), não duplicada do zero — extrair um helper compartilhado
sempre que a duplicação REQ/Roadmap ultrapassar ~10 linhas idênticas.

**D3 — `req move` move fisicamente o arquivo quando `REQDir/<estado>/` (ou `<agente>/<estado>/`) já
existe como estrutura corrente; permanece in-place quando não existe.** Critério de decisão: se
`findREQ` localizou o arquivo dentro de uma subpasta de estado reconhecida (`roadmapValidStateNames`),
`MoveREQ` resolve o diretório-alvo (`stateDir`/`agentStateDir`), cria com `os.MkdirAll`, move com
`os.Rename`, sincroniza `status:` no destino e registra a transição em `.trackfw-log` — replicando os
passos 1-4 de `MoveRoadmap` (o passo 5, `syncREQReferences`, é exclusivo de Roadmap e não se aplica
aqui). Se o arquivo foi encontrado solto em `REQDir/` sem subpasta de estado, `MoveREQ` mantém o
comportamento atual: reescreve `status:` in-place e não cria estrutura de pastas nova por conta própria.
Isso preserva 100% de compatibilidade com projetos que nunca organizaram REQs em subpastas, sem exigir
migração ou flag.

**D4 — `TestMoveREQ_RewritesStatusInPlace` é preservado como cobertura do modo legado (REQ solta em
`REQDir/`)**, e um novo teste cobre o modo de move físico (REQ em `REQDir/<estado>/` e em
`REQDir/<agente>/<estado>/`). Nenhum teste existente é removido; a REQ (AC6) já exige fixtures para os
dois modos.

## Consequences

**Positivas:**
- `req list`/`req move` passam a ter o mesmo alcance de `trackfw context` para REQs — elimina a
  divergência entre comandos que tornou o bug difícil de diagnosticar em campo.
- `req move` passa a manter "pasta é a fonte de verdade do estado" também para REQs, eliminando a
  divergência pasta-vs-status que o `validate` (regra R8) reporta como erro após um move.
- Reuso de `RoadmapNamespacing` e da lógica de `findRoadmap`/`MoveRoadmap` mantém o schema de config
  enxuto e o comportamento de REQ previsível para quem já entende o namespacing de Roadmap.

**Negativas / riscos:**
- Acopla o namespacing de REQ ao de Roadmap: um projeto que um dia precise de valores diferentes para
  os dois exigirá um ADR sucessor e migração de config (`req_namespacing` novo). Risco aceito por falta
  de evidência de necessidade hoje.
- `MoveREQ` ganha um branch condicional (in-place vs. move físico) que aumenta levemente a complexidade
  ciclomática da função — mitigado por replicar o padrão já testado de `MoveRoadmap` em vez de inventar
  lógica nova.
- Precisa ser implementado nos 3 CLIs (Go, Node, Python) para manter a Regra Dura de Paridade — escopo
  já coberto pelo AC5 da REQ.

## Alternatives Considered

- **Campo `req_namespacing` independente no schema.** Rejeitado por D1: nenhum caso de uso conhecido
  demanda valores divergentes entre REQ e Roadmap; adicionar o campo agora seria especulativo. Fica
  como opção para um ADR sucessor caso surja evidência real.
- **`req move` sempre mover fisicamente, mesmo em `REQDir/` flat sem subpastas de estado.** Rejeitado:
  criaria estrutura de pastas nova (`REQDir/backlog/`, `REQDir/wip/`, ...) em todo projeto existente sem
  aviso nem opt-in, quebrando expectativa de projetos que hoje mantêm REQs deliberadamente soltas.
  Violaria a diretriz de migração implícita silenciosa.
- **Unificar os scanners de REQ e Roadmap num único componente genérico agora.** Rejeitado nesta REQ —
  a REQ já declara em Negative Scope que não propõe unificar os scanners internos além do necessário
  para paridade com `context`. Fica registrado como possível trabalho futuro se a duplicação
  REQ/Roadmap crescer.
