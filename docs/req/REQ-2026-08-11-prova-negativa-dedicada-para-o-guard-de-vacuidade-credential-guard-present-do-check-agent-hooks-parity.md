---
status: Done
date: 2026-08-11
author: "Zeus (Arquiteto)"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-12-prova-negativa-dedicada-para-o-guard-de-vacuidade-credential-guard-present.md"
---

# REQ: Prova negativa dedicada para o guard de vacuidade credential-guard-present do check-agent-hooks-parity

> Date: 2026-08-11 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

O gate `scripts/check-agent-hooks-parity.sh` tem um guard de vacuidade P2 chamado
`credential-guard-present`: ele existe para impedir que o comparador estrutural passe sobre arquivos
de hook **vazios ou sem o credential-guard**, dando falso verde.

Esse guard **não tem prova negativa própria** em `scripts/check-gates-falsify.sh`. O Cenário 44 — a
única prova P4 hoje associada a esse gate — falsifica apenas o **comparador estrutural**
(`compare_json`, corrompendo um matcher do Kiro no Node), **nunca o guard de vacuidade**. Ou seja: se
o guard de vacuidade parasse de funcionar, nenhum cenário de falsificação acusaria.

Isso é exatamente a classe de problema que o projeto já provou ser real duas vezes:

- 2026-08-08 — o próprio guard capturou um falso negativo **ambiental** (o gate rodava
  `discover --init` sem isolar `$HOME`; o credential-guard global instalado na máquina fazia o dedup
  pular a entrada de projeto). Registrado em
  `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md`.
- 2026-08-11 — no ML-1A do `ROADMAP-2026-08-11`, um teste aparentemente correto se mostrou incapaz de
  distinguir "migração ligada" de "migração ausente"; só a sabotagem revelou. A partir daí a prova
  negativa virou critério de aceite bloqueante nos MLs seguintes.

O achado foi reportado por Hefesto em duas sessões distintas (2026-08-08 e 2026-08-11, ML-8A) sem
ter sido endereçado. Esta REQ existe para que ele pare de ser carregado adiante.

## Acceptance Criteria

- [ ] Existe cenário em `scripts/check-gates-falsify.sh` que falsifica **especificamente** o guard de
      vacuidade `credential-guard-present` — não o comparador estrutural.
- [ ] O cenário segue o padrão do arquivo: baseline (gate passa na árvore íntegra) + detecção (gate
      **falha** na árvore sabotada) — não basta o gate passar.
- [ ] A sabotagem escolhida é a que o guard existe para pegar: arquivo de hook gerado **sem** a
      entrada de credential-guard, com o comparador estrutural ainda satisfeito.
- [ ] `docs/cli-parity.md` atualizado, removendo a ressalva de que o guard não tem prova negativa.
- [ ] `make quality` verde, com o total de cenários incrementado.

### Escopo negativo

- **Não** altera o comportamento do gate `check-agent-hooks-parity.sh` em si — só acrescenta a prova
  de que ele não é vácuo.
- **Não** altera código de produto (`internal/`, `npm/src/`, `pypi/trackfw/`).

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-12-prova-negativa-dedicada-para-o-guard-de-vacuidade-credential-guard-present.md
