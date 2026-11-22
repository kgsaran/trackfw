---
status: Open
date: 2026-11-22
author: "zeus-tf (decisões do KG em 2026-11-22)"
adr: "docs/adr/ADR-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-site-em-ingles-na-raiz-e-pinado-por-teste.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md"
---

# REQ: Onboarding verificável: init honra as flags sem TTY e README e site batem com o binário

> Date: 2026-11-22 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
A auditoria de onboarding de 2026-11-22 (`docs/qualidade/2026-11-22-auditoria-onboarding.md`) seguiu o README e o site
com o binário 11.8.1, como um usuário novo, e encontrou 25 achados: 3 bloqueantes, 14 atritos e 8 cosméticos. O
arquiteto reproduziu os bloqueantes no binário:

- sem TTY, `trackfw init --brownfield` sai com rc=0 e não grava o modo lenient. Agentes de IA rodam sem TTY;
- o guia do site manda rodar `adr new` e `req new` sem título (rc=1) e `roadmap move REQ-...`, que não existe.

O site não tem teste. O teste do Quick start guarda uma cópia da sequência e diverge do README: o teste afirma 2 avisos,
e o README promete `✓ No violations found.`. O produto é inglês primeiro, mas o site serve pt-BR na raiz.

**Decisões do KG (2026-11-22):** uma REQ para o onboarding inteiro (defeito do `init`, bloqueantes e atritos do site
e do README, site pinado por teste); inglês na raiz do site e pt-BR em `/pt/`. ADR-2026-11-22, D1–D6.

## Acceptance Criteria
- [ ] AC1: toda flag de `trackfw init` tem o mesmo efeito com e sem TTY (`--brownfield`, `--ai-tools` e as demais que
  a varredura da Wave 0 encontrar), com teste de braço e contra-braço por caminho; os limites do brownfield (ADR D2)
  são afirmados por teste
- [ ] AC2: o teste do Quick start afirma que cada comando que executa aparece literalmente no README e no guia de
  início do site em inglês, e que a saída prometida no documento é a que o `validate` produz (ADR D4)
- [ ] AC3: o site serve inglês em `/trackfw/` e pt-BR em `/trackfw/pt/`; `npm ci && npm run build` em `site/` passa,
  e as capturas em Chrome headless das duas raízes mostram as páginas sem asset quebrado (ADR D3, D6)
- [ ] AC4: os bloqueantes e os atritos da auditoria estão corrigidos, ou justificados no roadmap um a um
- [ ] AC5: no README, o Quick start e a continuação dele ficam contíguos e o bloco de Windows sai de *Installation*,
  sem conteúdo removido e sem âncora interna quebrada (ADR D5)
- [ ] AC6: `make quality` rc=0 e CI verde no PR

## Negative scope
- Não cria job novo de CI para o site (gates novos congelados; ADR D6).
- Não muda o conteúdo de `docs/` (documentação interna continua em português).
- Não muda o comportamento de `trackfw update`, `discover` nem das regras do `validate`, a não ser o que o D2 exigir
  para o brownfield.
- Não reescreve a referência de comandos do site além do que a auditoria apontou.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-site-em-ingles-na-raiz-e-pinado-por-teste.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md
