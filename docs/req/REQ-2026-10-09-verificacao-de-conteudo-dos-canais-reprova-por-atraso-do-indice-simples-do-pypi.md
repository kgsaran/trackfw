---
status: Open
date: 2026-10-09
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md"
---

# REQ: verificacao de conteudo dos canais reprova por atraso do indice simples do PyPI

> Date: 2026-10-09 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Duas releases seguidas (9.3.3 em 2026-10-08 e 9.4.1 em 2026-10-09) tiveram o job `Verify all release channels` do
`release.yml` reprovado no passo `Verify channel content (D7)`: `pip download trackfw==<v>` falhou ~3 min após o upload
("wheels may not be published yet"). Nas duas vezes os 3 canais estavam corretos e a reexecução manual passou.

Mecanismo lido: o passo anterior, `scripts/verify-pypi-channel.py --retry` (ML-4H), confirma a versão pela JSON API
(`/pypi/trackfw/<v>/json`), que fica consistente cedo; o comentário do workflow conclui que "read-after-write is
resolved". Mas o `check-channels-content.sh --published` usa `pip download`, que lê o **índice simples**
(`/simple/trackfw/`, servido por CDN) — outro endpoint, com outro atraso, e sem retry. A premissa do comentário vale
para um endpoint e é aplicada a outro. O `npm pack` do mesmo passo tem a mesma forma (sem retry), embora não tenha
falhado ainda.

**Escopo negativo:** não muda o que é verificado (conteúdo dos pacotes); não muda publicação; não relaxa a verificação
para "aviso".

## Acceptance Criteria
- [ ] AC1 — Wave 0: medição dos endpoints (JSON API vs índice simples do PyPI; registry npm vs `npm pack`) e enumeração de todo passo de verificação pós-publicação com a mesma forma (leitura sem retry de endpoint diferente do confirmado)
- [ ] AC2 — `check-channels-content.sh --published` tenta de novo, com backoff e prazo total declarado, antes de reprovar `pip download` e `npm pack`; esgotado o prazo, reprova como hoje (publicação que de fato falhou continua vermelha)
- [ ] AC3 — Teste/self-test que prova as duas direções (indisponível e depois disponível → passa; indisponível até o prazo → reprova) sem rede real
- [ ] AC4 — Comentário do `release.yml` corrigido (a premissa "read-after-write is resolved" deixa de ser afirmada para o índice simples)

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md
