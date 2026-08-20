---
status: Open
date: 2026-08-20
author: ""
adr: ""
roadmap: ""
---

# REQ: `note_orphan` existe em Go e Python e está ausente do CLI Node

> Date: 2026-08-20 | Status: Open (backlog, sem roadmap)

## Motivação

Achado **lateral** do ML-1A do contrato pinado, ao anotar a seção `## Vault de conhecimento` como
contrato sem gate. Confirmado por mim, por medição:

```
grep -rc note_orphan internal/validator/validator.go   -> 3
grep -rc note_orphan pypi/trackfw/validator.py         -> 4
grep -rn note_orphan npm/src/                          -> NENHUMA ocorrência
```

E `docs/cli-parity.md:147` documenta a regra como contrato, com tabela de severidade padrão,
escalação via `rules: { note_orphan: error }` e comportamento na ausência de `vault/`.

**Violação viva da regra dura de paridade dos 3 CLIs.** Um projeto governado pelo CLI Node não tem
detecção de nota órfã, e `trackfw validate` passa verde onde os outros dois acusariam.

### O que este achado prova sobre a REQ que o gerou

A `REQ-2026-08-18-contrato-pinado-sem-gate-nomeado` sustenta que contrato sem gate nomeado é
contrato não-aplicado. Aqui está a demonstração, e ela não foi construída: **a violação apareceu no
primeiro contato com o mecanismo, antes mesmo de o mecanismo existir** — bastou alguém perguntar
"qual gate protege esta seção?" para descobrir que não há gate, e que por isso a seção descreve algo
que um dos três CLIs não faz.

Vale registrar como evidência a favor de priorizar o mecanismo, não só as instâncias.

## Escopo

1. Implementar `note_orphan` em `npm/src/validator/`, com paridade de comportamento: severidade
   padrão, escalação por `rules:`, comportamento sem `vault/`, e os formatos de detecção de link.
2. **Gate comparando as três saídas reais** — teste por stack não fecha. Esta série já provou cinco
   vezes que cada runtime concorda consigo mesmo.
3. Cenário P4 com braço de baseline e de detecção.

## O que **não** é escopo

- Mudar a semântica da regra em Go ou Python. A referência é o comportamento existente; se houver
  divergência entre os dois, **isso é achado**, e vira decisão antes de codificar.
- Varrer as demais regras em busca de ausências parecidas. É trabalho legítimo, mas é o ML-2A do
  contrato pinado que vai revelá-las de forma sistemática — fazer manualmente aqui duplicaria o
  esforço e ainda ficaria incompleto.

## Acceptance Criteria

- [ ] AC1 — `note_orphan` implementada no CLI Node, com paridade de comportamento.
- [ ] AC2 — **Antes de codificar**, verificado se Go e Python concordam entre si; divergência é decisão.
- [ ] AC3 — Gate comparando as **três saídas reais** — não por leitura de fonte.
- [ ] AC4 — Cenário P4, baseline + detecção.
- [ ] AC5 — Seção do `docs/cli-parity.md` atualizada, **nomeando o gate**.
- [ ] AC6 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **Falso-positivo em nota legítima** treina o usuário a ignorar o `validate`. Os formatos de
  detecção de link precisam ser exatamente os mesmos dos outros dois CLIs.
- **Não presumir que Go e Python concordam** — a REQ existe justamente porque uma suposição de
  paridade estava errada.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- nenhum; é implementação de regra já decidida -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
