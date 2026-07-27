---
status: Open
date: 2026-07-27
author: "zeus"
adr: "docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md"
---

# REQ: Débitos técnicos pós-release de robustez e manutenibilidade

> Date: 2026-07-27 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Três débitos não impedem a próxima versão, mas reduzem configurabilidade, observabilidade de falhas e
manutenibilidade dos gates:

1. `stale_wip` usa limite hardcoded de sete dias e a referência temporal mistura `git log` com
   fallback para `mtime`, sem contrato configurável.
2. Validators silenciam erros de walk/leitura (`adr_orphan` e múltiplos `ReadFile → continue`),
   permitindo degradação silenciosa quando o filesystem não pode ser inspecionado.
3. `scripts/check-identity-parity.sh` mantém `TARGETS` hardcoded, podendo deixar novas superfícies do
   catálogo fora do gate.

São melhorias de robustez e arquitetura interna. Devem permanecer formalizadas, mas não bloquear a
tag enquanto os gates e funcionalidades existentes estiverem verdes.

## Acceptance Criteria

- [ ] Política de idade do `stale_wip` documentada e configurável nos três CLIs.
- [ ] Fonte temporal e fallback possuem semântica explícita e testes determinísticos.
- [ ] Erros relevantes de I/O deixam de ser convertidos silenciosamente em sucesso.
- [ ] Severidade/diagnóstico de falhas de inspeção é consistente nos três CLIs.
- [ ] Alvos do gate de identidade são derivados do catálogo ou validados contra ele.
- [ ] Provas negativas demonstram que arquivo ilegível e alvo novo não passam silenciosamente.
- [ ] Compatibilidade com configurações existentes preservada.

## Priority

Não bloqueante para a próxima release. Planejar após os roadmaps de release gate e contrato de
roadmap/analyzing.

## Scope

### Included

- Configuração e semântica de `stale_wip`.
- Política sistêmica de erros de I/O do validator.
- Derivação/validação do catálogo no gate de identidade.
- Paridade Go/Node/Python e documentação.

### Excluded

- Bloqueadores de release formalizados em REQ própria.
- Novos estados do roadmap.
- Mudanças de UX no board.
- Refatorações sem relação com os três débitos.

## Linked ADR

ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`

## Blocked by ADRs

<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md`
