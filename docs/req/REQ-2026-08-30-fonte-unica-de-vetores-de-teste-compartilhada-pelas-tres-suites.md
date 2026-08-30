---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: Fonte única de vetores de teste compartilhada pelas três suítes

> Date: 2026-08-30 | Status: Open

## Motivation

Proposta pelo `hefesto-tf` na barreira final da REQ do dialeto do `barrier`, respondendo a uma
pergunta minha que estava mal formulada.

Eu perguntei se dava para reduzir as **três implementações** da regra de conclusão de ML. A resposta
dele foi melhor que a pergunta: três implementações são **inevitáveis** sem runtime compartilhado —
o problema não é esse. O problema são **três listas de vetores de teste escritas à mão**.

Foi exatamente isso que produziu, num único ciclo, duas divergências:

- **VS16** (`✅️` com `U+FE0F`): Go aceitava, Node e Python rejeitavam
- **`.trim()`**: Node liberava wave que Go e Python bloqueavam

Nos dois casos as três implementações estavam corretas segundo a lista de casos que **cada uma**
tinha. Nenhum teste falhou, porque cada suíte testava um conjunto diferente.

E se repetiu depois: a normalização Unicode com faixa fixa no Node contra categoria `Mn` em Go e
Python, achada pelo `hades-tf` só na barreira final.

**A hipótese a testar:** um arquivo de vetores versionado (entrada → veredito esperado), consumido
pelas três suítes, faz qualquer divergência de comportamento aparecer como **teste vermelho** em vez
de achado de revisão.

## Acceptance Criteria

- [ ] **AC1** — Formato de vetores legível pelos 3 runtimes sem dependência nova (JSON ou TSV).
- [ ] **AC2** — Piloto na regra de conclusão de status do `barrier` — a que produziu as três
      divergências. **Não** migrar tudo de uma vez.
- [ ] **AC3** — As 3 suítes consomem o mesmo arquivo; acrescentar um vetor cobre os três de uma vez.
- [ ] **AC4** — **Prova de valor, obrigatória**: reintroduzir a divergência de VS16 numa cópia
      descartável e mostrar que o mecanismo a pega **como teste vermelho**. Sem isso, é refatoração
      de teste sem evidência de que resolve o problema que motivou a REQ.
- [ ] **AC5** — Guarda de vacuidade: suíte que deixe de consumir o arquivo **falha**. Um consumidor
      que silenciosamente para de ler os vetores é o mesmo defeito com outra roupa.
- [ ] **AC6** — Decidir e documentar o critério de expansão: quais regras migram depois, e por quê.
- [ ] **AC7** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** unificar as implementações — a regra dos 3 CLIs permanece.
- **Não** migrar todas as regras nesta REQ. Piloto e critério de expansão.
- **Não** substituir os gates de paridade existentes; os vetores são camada adicional.

## Linked ADR
<!-- Provável: onde vetores compartilhados são autoridade e onde os gates continuam sendo. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
