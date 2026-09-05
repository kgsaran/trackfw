---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: ""
---

# REQ: validate_unfiltered do Python devolve lista de tipo misto e o consumidor estoura

> Date: 2026-09-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue **`#261`**, aberta pelo consumidor externo em 2026-09-03. 🔴 **A auditoria externa de 2026-09-05
registrou que esta issue não tinha REQ nem ADR — nenhum artefato.** Ficou três dias só como issue.

No Python, `validate_unfiltered` devolve lista **heterogênea**: quase toda regra devolve `dict`, e
`branch_has_wip_roadmap` devolve **string crua**.

```python
# pypi/trackfw/validator.py — branch_has_wip_roadmap
return [branch_governance_orientation(branch)]              # str
return [branch_no_matching_roadmap_message(branch, ...)]    # str

# toda outra regra:
violations.append({"type": "violation", "message": ...})    # dict
```

E `_enrich_items` tem um `else` que **deixa o não-dict passar sem embrulhar**:

```python
else:
    result.append(item)     # a string sobrevive ate o consumidor
```

Qualquer consumidor que faça `item["message"]` estoura. **Não é hipótese: derruba 7 testes** que não
têm relação nenhuma com a regra.

Confirmado por medição independente na triagem
(`docs/portabilidade/2026-09-05-triagem-dos-sete-issues-do-lourival.md`) e ainda presente na `main`.

## Acceptance Criteria

- [ ] **AC1** — `validate_unfiltered` devolve **um único tipo**. O contrato fica escrito, não
      implícito no formato que a maioria das regras usa por hábito.
- [ ] **AC2** — 🔴 **O `else` do `_enrich_items` deixa de ser fail-open silencioso.** Ele é o que
      permitiu a divergência sobreviver até o consumidor. Item fora do contrato deve **falhar alto**
      ali, não passar adiante — senão a próxima regra que devolver o tipo errado repete o defeito.
- [ ] **AC3** — Falsificação nas duas direções: com o contrato violado de propósito, o gate acusa;
      com a regra correta, passa. Os 7 testes citados na issue voltam a passar **pela razão certa**.
- [ ] **AC4** — 🔴 **Paridade verificada, não presumida.** Go e Node podem já estar corretos por
      construção (tipagem) ou ter a mesma divergência. **Medir os três** e declarar.
- [ ] **AC5** — 🔴 **Guarda contra a próxima instância:** um teste que exercite o contrato de retorno
      de **todas** as regras, não só das que já divergiram. O defeito não é a regra — é não haver
      nada que verifique o formato.

## Negative Scope
- ❌ **Não** tratar o isolamento da resolução de branch, que a auditoria apontou como assunto
  separado.
- ❌ **Não** mexer no comportamento da regra `branch_has_wip_roadmap` em si — o casamento por
  substring é o `#273`, com REQ própria.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
