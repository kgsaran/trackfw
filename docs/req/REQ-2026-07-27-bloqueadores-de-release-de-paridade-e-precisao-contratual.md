---
status: Open
date: 2026-07-27
author: "zeus"
adr: "docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md"
---

# REQ: Bloqueadores de release de paridade e precisão contratual

> Date: 2026-07-27 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Quatro divergências impedem afirmar que a próxima versão oferece comportamento equivalente nos três
CLIs e documentação verdadeira:

1. Python não expõe `roadmap new --from-req`, `--req` e `--title`, presentes em Go/Node.
2. `parse_frontmatter` do validator Python mantém aspas nos valores, gerando resultados diferentes
   para o mesmo artefato (`status: "wip"`).
3. O transition log Python em layout `by_agent` omite o agente, enquanto Go/Node preservam essa
   dimensão usada por métricas e auditoria.
4. `site/guide/ai-agents.md` afirma que `trackfw validate` valida frontmatter contra
   `docs/schema/*.json`, mas o validator não consome esses schemas.

Os três primeiros violam a regra dura de paridade. O quarto publica um contrato inexistente. A
release só pode ser considerada pronta quando os comportamentos forem convergentes e protegidos por
provas negativas, ou quando a documentação declarar explicitamente uma exceção aprovada.

## Acceptance Criteria

- [ ] Python oferece `--from-req`, `--req` e `--title` com a mesma precedência e saída observável de
      Go/Node.
- [ ] Um frontmatter com valores entre aspas produz o mesmo resultado nos três validators.
- [ ] Transition logs `by_agent` carregam a mesma identificação de agente nos três runtimes.
- [ ] Métricas leem logs Python/Go/Node sem perder atribuição por agente.
- [ ] A documentação deixa de afirmar validação automática por JSON Schema, ou a funcionalidade é
      implementada nos três CLIs mediante decisão explícita.
- [ ] Cada defeito possui teste negativo anterior à correção e proteção contra XPASS silencioso.
- [ ] Gates de ajuda, artefato e log exercitam comportamento real, não apenas presença de strings.
- [ ] `make quality`, package smoke e `trackfw validate` passam sem violations ou warnings.

## Release Gate

Esta REQ é **bloqueante para a próxima versão**. Não gerar tag enquanto qualquer critério acima
estiver pendente.

## Scope

### Included

- Paridade do comando `roadmap new` no Python.
- Normalização de aspas no parser Python.
- Paridade do transition log `by_agent`.
- Correção do contrato público sobre JSON Schemas.
- Testes/gates/documentação necessários.

### Excluded

- Tornar `stale_wip` configurável.
- Refatorar todos os caminhos silenciosos de I/O do validator.
- Derivar dinamicamente alvos do gate de identidade.
- Corrigir o slash-command de roadmap e o estado `analyzing` (REQ separada).

## Linked ADR

ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`

## Blocked by ADRs

<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md`
