---
status: Done
date: 2026-09-11
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-11-self-test-do-ratchet-escreve-no-canal-do-gate-real-e-quebra-em-cp1252-e-no-runner.md"
---

# REQ: self-test do ratchet escreve no canal do gate real e quebra em cp1252 e no runner

> Date: 2026-09-11 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-11-self-test-do-ratchet-escreve-no-canal-do-gate-real-e-quebra-em-cp1252-e-no-runner.md`
## Motivation

Dois issues do consumidor externo, **ambos sobre código que entregamos em 2026-09-10**, e ambos no
`scripts/check-windows-known-failures.py` — o verificador do ratchet.

### #314 — o `--self-test` morre em `cp1252`

```
=== T15: baseline positive confirmation line -> present with baseline, absent without ===
Traceback (most recent call last):
  File "scripts/check-windows-known-failures.py", line 970, in check
    print(f"  SELF-TEST PASS: {description}", flush=True)
UnicodeEncodeError: 'charmap' codec can't encode character '\u2192' in position 39
```

`sys.stdout.encoding` = `cp1252` (Windows 11, Git Bash). O relator observa: **entrada tratada em 10
sítios, saída em nenhum**.

### #319 — job verde com 10 anotações de erro

```
job parity-other-gates · conclusão: success · anotações failure: 10
  - ML-3A: NEW Go suite-load-failure not in known list: '.../internal/badpkg'
  - ML-2B D4: removed entry 'TestFoo' has removal_note='renamed' ...
```

`badpkg` e `TestFoo` são **fixtures do próprio self-test**. Os casos sintéticos exercitam os caminhos
de erro do ratchet — imprimir `::error::` é o comportamento que eles testam. Mas o runner lê o stdout
do passo e converte cada `::error::` em anotação.

## 🔴 Hipótese de causa comum — a MEDIR, não a presumir

**O `--self-test` escreve no mesmo canal que o gate de produção.** Por isso herda o encoding do
console (#314) e é interpretado pelo runner (#319).

Se for isso, corrigir o canal fecha os dois — e é o teste de agrupamento do projeto:
*"se eu corrigir esta causa, exatamente estas falhas fecham — e nenhuma outra."*

⚠️ **Se a medição mostrar causas distintas** — encoding é do `print`, marcador é do conteúdo —,
**separam-se**, com a medição escrita. Agrupar por sintoma parecido foi o erro do grupo do `IsAbs`
nesta campanha: estimado em 14 falhas, entregou 2.

## Por que isto é prioritário

É **ferramental de governança**, não produto. Um self-test que **não roda no Windows** não protege
o ratchet no único sistema em que o ratchet existe para proteger. E um job **verde com 10 anotações
de erro** treina o leitor a ignorar anotação — que é como o `continue-on-error` virou invisível.

## Acceptance Criteria

- [ ] **AC1** — 🔴 medir se a causa é comum **antes** de corrigir; a decisão de agrupar ou separar
      fica **escrita**, com a medição ao lado.
- [ ] **AC2** — o `--self-test` roda até o fim com `sys.stdout.encoding = cp1252`. 🔴 **Provado com o
      encoding forçado**, não com `LANG` ajustado — o relator mediu no console real.
- [ ] **AC3** — saída de **fixture** do self-test não vira anotação do runner. O job deixa de exibir
      erro que não é erro.
- [ ] **AC4** — 🔴 **a distinção não pode custar o sinal real**: quando o gate **de produção** emite
      `::error::`, ele continua virando anotação. Falsificação nas duas direções.
- [ ] **AC5** — guarda de não-vacuidade: se o self-test deixar de exercitar os caminhos de erro para
      não poluir, ele deixou de testar. **Prove que os braços de erro continuam sendo executados.**
- [ ] **AC6** — varredura dos **outros** `check-*.sh` com self-test: quantos têm o mesmo defeito?
      Derivar, não presumir isolamento.
