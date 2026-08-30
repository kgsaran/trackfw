---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `trackfw context` do CLI Node falha sempre porque `validate()` assíncrono é chamado sem `await`

> Date: 2026-08-30 | Status: Open

## Motivation

`npm/src/commands/context.js:136` chama `validate()`, que é `async`, **sem `await`**. O resultado é
uma `Promise`, e o acesso subsequente às propriedades quebra com
`Cannot read properties of undefined`.

Achado pelo `apolo-tf` durante o ML-1A da REQ do namespace, e **reproduz em projeto `flat` mínimo** —
não tem relação com `by_agent`. Ou seja: **o `trackfw context` do CLI Node falha sempre**, para
qualquer usuário, no primeiro comando que o protocolo de agentes deste projeto manda rodar
(*"Before starting: run `trackfw context`"*).

Que isso tenha sobrevivido diz algo sobre a cobertura: nenhum teste exercita o `context` do Node pelo
CLI real.

## Acceptance Criteria

- [ ] **AC1** — `trackfw context` do Node executa sem erro em projeto `flat` e em `by_agent`.
- [ ] **AC2** — Saída equivalente à de Go e Python para o mesmo projeto. Se houver divergência
      pré-existente de formatação, **documentar**, não silenciar.
- [ ] **AC3** — Teste que exercita o comando pelo **CLI real**, não pela função interna — foi a
      ausência disso que deixou o defeito passar.
- [ ] **AC4** — Varrer os outros comandos do Node por `async` chamado sem `await`. Se o defeito é de
      padrão, corrigir a classe, não a instância.
- [ ] **AC5** — `npm test --prefix npm` → 0; `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** reescrever o `context` nem alterar seu formato de saída além do necessário.

## Linked ADR
<!-- none -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
