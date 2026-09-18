---
status: Superseded
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: `pypi/trackfw/tty.py` não tem teste unitário direto e o stub esconde a função inteira

> Date: 2026-09-01 | Status: Superseded

## Motivation

Achado do `hefesto-tf` na barreira final da `REQ-2026-08-31`
(`docs/qualidade/2026-09-01-barreira-do-port-do-reporter-da-issue-216.md`).

O `pypi/trackfw/tty.py` entrou no port do #224 e é **superfície nova de FFI** — chama
`GetConsoleMode` do Windows via `ctypes` para estreitar o `isatty()`. **Não tem teste unitário
direto**: o `pypi/tests/test_scope_resolution.py` faz **stub da função inteira** em vez de
exercitá-la, então o comportamento real nunca é executado por teste.

Lacuna herdada do PR original, não introduzida pelo port.

**Por que importa mais do que parece:** o `hades-tf` descobriu na barreira que
`stdin_is_interactive()` governa também o **portão de confiança** do
`trackfw thirdparty install --yes-i-trust-this-source`, não apenas o wizard do `init`. É caminho de
segurança sem teste próprio, e um stub que substitui a função inteira **garante** que nenhuma
regressão nela será detectada.

## Acceptance Criteria

- [ ] **AC1** — `pypi/tests/test_tty.py` exercitando `stdin_is_interactive()` de verdade.
- [ ] **AC2** — Os três caminhos de falha que o `hades-tf` verificou por leitura passam a ser
      cobertos por teste: stream sem `fileno()`, handle inválido, DLL/plataforma indisponível.
- [ ] **AC3** — 🔴 **A direção segura é falsificada.** O módulo declara falhar para `False` em
      qualquer exceção. Prove que é verdade **em todos os caminhos**, e que `False` é a direção certa
      — um `init` que **não** entra no wizard é melhor que um que entra em contexto não interativo.
- [ ] **AC4** — O no-op em não-Windows é testado, não presumido.
- [ ] **AC5** — Avaliar se o stub em `test_scope_resolution.py` deve ser estreitado. Um stub que
      substitui a função inteira torna aquele teste cego a qualquer mudança nela.

## Negative Scope

- **Não** altera o comportamento do `tty.py`. Ele foi aprovado nas duas barreiras.
- **Não** remove o stub do `test_scope_resolution.py` sem entender o que ele protege.

## Linked ADR

ADR: <!-- nenhum. Cobertura de teste. -->

## Linked Roadmap

Roadmap:

## Fechamento — triagem de 2026-09-18

**Veredito:** Superseded. **Motivo medido:** alvo removido pela v8: pypi/trackfw/tty.py deletado em 2eae0a44

Fechada na triagem por evidência das REQs abertas (`docs/triagem-reqs-abertas-2026-09-18.md`),
executada a pedido do KG. Esta triagem é o **AC12 da `REQ-2026-09-12-v8-um-binario-muitos-canais`**
e os **AC10-AC12** da REQ irmã, que pediam classificar as REQs abertas e fechar aquelas cuja causa a
v8 removeu — e que nunca haviam sido executados.
