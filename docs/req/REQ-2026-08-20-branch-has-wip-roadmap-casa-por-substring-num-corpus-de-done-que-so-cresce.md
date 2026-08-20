---
status: Open
date: 2026-08-20
author: ""
adr: ""
roadmap: ""
---

# REQ: `branch_has_wip_roadmap` casa por substring num corpus de `done/` que só cresce

> Date: 2026-08-20 | Status: Open (backlog, sem roadmap)

## Motivação

Achado da barreira do `hades-tf` (`docs/seguranca/2026-08-20-revisao-dos-gates-dos-tres-contratos.md`,
achado C-1), confirmado por mim no repositório real:

```
docs/roadmaps/done/          127 arquivos
"guard" casa por substring    11
"serve"                        3
```

A regra aceita a branch se o slug estiver **contido** no nome de algum roadmap em `wip/` **ou**
`done/`. Como `done/` **só cresce**, um slug curto como `fix/guard` é hoje satisfeito por 11
roadmaps sem nenhuma relação com o trabalho — e amanhã por mais.

**É uma regra de governança que enfraquece com a idade do projeto.** Quanto mais o time entrega,
menos o portão exige.

### Correção de atribuição

A barreira registrou que *"o ML-2A é a mudança que amplia o corpus"*. **Não é.** Verifiquei: o ML-2A
não tocou código de produto — só `scripts/` e docs. A aceitação de `done/` vem da
`REQ-2026-07-26`; a fraqueza é **pré-existente**.

O que o ML-2A fez foi **torná-la visível**, ao exercitar a regra cross-CLI pela primeira vez. É
exatamente para isso que gate serve, e vale registrar como evidência a favor da REQ do contrato
pinado: a lacuna estava lá desde julho e ninguém a tinha visto.

## Escopo

Fechar a folga do casamento, **sem** quebrar o caso de uso legítimo — retomar trabalho de um roadmap
já concluído.

Candidatos a avaliar (a decisão é do ADR, não deste texto):
1. **Casamento por fronteira** em vez de substring — `fix/guard` casa `...-guard-...` mas não
   `...-guardrails-...`. Mais estrito, e provavelmente suficiente.
2. **Janela de recência** em `done/` — só roadmaps concluídos há menos de N dias contam.
   Resolve o crescimento monotônico, mas introduz dependência de tempo num portão determinístico.
3. **Exigir `wip/` e tratar `done/` como aviso**, não aceitação.

## O que **não** é escopo

- Remover a aceitação de `done/`. Ela existe por um motivo — retomar trabalho concluído — e a
  `REQ-2026-07-26` a decidiu. Esta REQ ajusta a **precisão** do casamento, não a política.
- As demais ressalvas da barreira (A-1, B-1, B-3, D-2), que são de gate e documentação.

## 🔴 O risco dominante é o inverso do óbvio

Apertar demais quebra o fluxo de quem legitimamente retoma um roadmap concluído — e o portão é
atravessado por **todo** `branch new`, `commit` e `ship` do projeto. Um falso-positivo aqui não
irrita: **paralisa**. Vale o mesmo princípio do `ADR-2026-08-17`: guard que atrapalha é guard que o
usuário desliga.

Recomendo medir, antes de escolher, **quantos casamentos legítimos históricos** cada candidato
preservaria — os 127 roadmaps em `done/` são o corpus de teste pronto.

## Acceptance Criteria

- [ ] AC1 — Decisão registrada em **ADR**, com o motivo e os candidatos descartados.
- [ ] AC2 — Slug curto não é mais satisfeito por roadmap sem relação — provado com o caso `guard`.
- [ ] AC3 — Retomada legítima de roadmap concluído **continua funcionando** — provado por cenário.
- [ ] AC4 — Medição sobre os 127 roadmaps reais: quantos casamentos mudam de veredito.
- [ ] AC5 — Paridade nos 3 CLIs, com gate comparando saídas reais.
- [ ] AC6 — Cenário P4 com baseline e detecção.
- [ ] AC7 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **O gate do ML-2A fixa o comportamento atual.** Mudar a regra vai reprovar aquele gate — é
  esperado, e o gate deve ser atualizado junto, nunca afrouxado para caber.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- a criar: precisao do casamento slug-roadmap -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
