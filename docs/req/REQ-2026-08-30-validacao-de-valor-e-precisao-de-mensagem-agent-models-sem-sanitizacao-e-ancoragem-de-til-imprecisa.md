---
status: Done
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-22-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-imprecisa.md"
---

# REQ: Validação de valor e precisão de mensagem — `agent_models` sem sanitização e ancoragem de `~` imprecisa

> Date: 2026-08-30 | Status: Done

## Motivation

Dois resíduos recortados de REQs anteriores, ambos sobre **valor que entra sem ser validado** ou
**mensagem que descreve mal o que foi detectado**.

**1. `agent_models` sem sanitização de valor.** Recortado da `REQ-2026-08-21`. O valor lido de
`~/.trackfw/trackfw.yaml` é interpolado no frontmatter dos arquivos de agente gerados. A nota
`vault/notes/rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21.md` registra que a
injeção por newline **foi corrigida** com `containsControlChar` — falta a validação do **formato** do
valor. Um `agent_models: { opus: "5 && algo" }` produz frontmatter com conteúdo arbitrário.

**2. Ancoragem de `~` imprecisa no classificador de hook.** A regra de ancoragem
(`ADR-2026-08-22`) classifica corretamente, mas as **mensagens** para `~usuario/` e para `"~/"` entre
aspas descrevem mal o caso: `filepath.IsAbs("~/")` é `false`, e a mensagem sugere um motivo que não
é o real. Quem lê o aviso procura o problema errado.

Nenhum dos dois é explorável hoje. Os dois custam tempo quando alguém investiga.

## Acceptance Criteria

- [x] **AC1** — Valor de `agent_models` validado por formato antes de interpolar; formato inválido
      falha com mensagem nomeando a chave e o esperado, sem gerar arquivo.
- [x] **AC2** — Falsificação nas duas direções: valores legítimos (`5`, `4.6`, `4-5-sonnet`) aceitos;
      valores com metacaractere, espaço ou controle recusados.
- [x] **AC3** — Mensagens de `~usuario/` e `"~/"` descrevem o motivo **real** da classificação.
- [x] **AC4** — A classificação em si **não muda** — o `ADR-2026-08-22` está correto; o que muda é o
      texto.
- [x] **AC5** — Paridade nos 3; gate falsificável para AC1 e AC2.
- [x] **AC6** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** reabrir a decisão de ancoragem do `ADR-2026-08-22`.
- **Não** ampliar o vocabulário aceito em `agent_models` — o escopo é validar o que já existe.

## Linked ADR
<!-- none: aplica ADR-2026-08-21 e ADR-2026-08-22. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-22-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-imprecisa.md`

## ✅ Encerramento — 2026-10-18

Entregue no #582 (`6dfef256`). AC1/AC2: valor fora de `[a-zA-Z0-9._:@/+-]` falha sem gravar arquivo nos 5 call sites; aceitos gravados como string entre aspas. AC3/AC4: já corrigidos no #271 (mensagens de `~`). AC5: paridade sumiu com a v8. AC6: `make quality` e CI verdes. Absorveu o resíduo da REQ-2026-08-21 e, por decisão do usuário, o YAML do `identity.json` (mesma causa).
