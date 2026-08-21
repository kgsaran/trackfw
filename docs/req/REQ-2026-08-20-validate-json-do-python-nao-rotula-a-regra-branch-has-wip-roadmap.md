---
status: Open
date: 2026-08-20
author: ""
adr: ""
roadmap: ""
---

# REQ: `validate --json` do Python não rotula a regra `branch_has_wip_roadmap`

> Date: 2026-08-20 | Status: Open (backlog, sem roadmap)

## Motivação

**7ª divergência real** desta série, achada pelo gate do ML-2A no exato momento em que ele passou a
exercitar a regra cross-CLI. Confirmada por mim, por medição:

```
TRACKFW_BRANCH=feat/inexistente  trackfw validate --json
  Go      ->  rule: "branch_has_wip_roadmap"
  Python  ->  rule: None
```

Causa: `validate_branch_has_wip_roadmap` (`pypi/trackfw/validator.py:1436`) devolve `list[str]` em
vez da forma de dicionário que o `_enrich_items` enriquece. Mensagem e código de saída seguem
byte-idênticos — **só o campo `rule` do JSON diverge**.

### Por que isso importa mais do que parece

`--json` é a superfície que integrações de CI consomem. Quem filtra ou suprime por `rule` **perde
esta regra silenciosamente no Python** — e é a regra que sustenta todo `branch new`, `commit` e
`ship`. O modo de falha é o pior tipo: a violação aparece, mas não é atribuível, então uma automação
que decide por `rule` a trata como desconhecida.

**O gate do ML-2A fixou o comportamento divergente de propósito**, para que qualquer deriva — em
qualquer direção — reprove alto. Isso **detecta**, não corrige: enquanto esta REQ não fechar, o gate
está protegendo um contrato que sabemos estar errado.

## Escopo

1. `validate_branch_has_wip_roadmap` passa a devolver a forma que o `_enrich_items` enriquece, de
   modo que o `rule` saia rotulado.
2. **Antes de corrigir, varrer as demais regras do Python** procurando a mesma forma. Se
   `validate_branch_has_wip_roadmap` devolve `list[str]` por descuido, é provável que não esteja
   sozinha — e corrigir uma de cada vez pagaria N ciclos.
3. Atualizar o gate do `check-validate-parity.sh`, que hoje fixa a divergência.

## O que **não** é escopo

- Mudar mensagem ou código de saída — os dois já são byte-idênticos e não têm defeito.
- Mudar Go ou Node: os dois estão corretos.

## Acceptance Criteria

- [ ] AC1 — `validate --json` do Python rotula `branch_has_wip_roadmap`, igual a Go e Node.
- [ ] AC2 — **Varredura feita**: as demais regras do Python verificadas quanto à mesma forma;
      encontradas, corrigidas no mesmo lote; nenhuma encontrada, registrado por escrito.
- [ ] AC3 — O gate deixa de fixar a divergência e passa a fixar a **convergência**.
- [ ] AC4 — Cenário P4 com baseline e detecção.
- [ ] AC5 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **A varredura do AC2 é o grosso do valor.** Corrigir só a regra reportada e descobrir mais três
  depois seria repetir o padrão "condição estreita demais" já nomeado sete vezes nesta série.
- **Não mexer na mensagem.** Ela é comparada byte a byte por gate; alterá-la de passagem quebra o
  contrato certo enquanto conserta o errado.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- nenhum; e correcao de forma de retorno -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
