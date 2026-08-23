---
status: Open
date: 2026-08-22
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md"
---

# REQ: Wave 0 de modelo de ameaça no harness, e o asset do arquiteto ensina `trackfw push`

> Date: 2026-08-22 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

**Duas lacunas do mesmo lugar — o harness — descobertas no mesmo dia.**

### 1. A revisão de segurança chega tarde demais

Decisão do KG, registrada em
`ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md`:
toda REQ ganha uma **Wave 0 de modelo de ameaça** antes da primeira linha de implementação.

Evidência medida em três REQs consecutivas (2026-08-21/22): **duas** foram retrabalhadas por achados
de barreira que eram **completude de enumeração** — pergunta que não precisa de código para ser
respondida. O caso mais caro: `~/…` sendo acusado indevidamente, o que faria o `validate` acusar o
caminho do **harness global do próprio trackfw** e empurrar o usuário a quebrar o próprio hook.

### 2. O guard ensina `trackfw push`; o asset do arquiteto não sabe que ele existe

Medido em 2026-08-22, depois de `trackfw update harness` e `agents update --force` com a 7.2.0
instalada:

```
git push bruto  -> "Use `trackfw push` (para empurrar commits já criados)..."   OK
grep -c "trackfw push" ~/.claude/agents/trackfw-architect.md  ->  0
```

O asset descreve a autoridade de Git como *"pushes to the working branch"*, sem nomear o comando. O
#202 fechou o beco sem saída no CLI; o agente que mais empurra código continua sem saber que a saída
existe — e volta a cair nele a cada contexto novo.

## Acceptance Criteria

- [ ] **AC1** — O gerador de roadmap emite **Wave 0 — Modelo de ameaça** antes da Wave 1, nos **3
      CLIs**, tanto em `roadmap new` quanto em `roadmap new --from-req`, com as quatro seções do ADR:
      completude de enumeração · modelo de ameaça · alvos de falsificação nas duas direções ·
      residual declarado.
- [ ] **AC2** — Os três geradores produzem template **byte-idêntico**, provado por gate.
- [ ] **AC3** — `trackfw barrier <roadmap> --wave 0` **é aceito** nos 3 CLIs. Hoje é recusado com
      `invalid --wave` porque a gramática exige `>= 1` (`internal/commands/barrier.go:89` e
      equivalentes). Sem isso a wave nova nasce inavaliável pela própria ferramenta.
- [ ] **AC4** — O parser de waves reconhece o cabeçalho `## Wave 0 — …` nos 3 CLIs.
- [ ] **AC5** — Asset do arquiteto (`internal/integrations/assets/agents/architect.md` + as cópias em
      `npm/src/` e `pypi/trackfw/`) exige Wave 0 antes de despachar implementação **e** nomeia
      `trackfw push` na autoridade de Git, distinguindo os três comandos.
- [ ] **AC6** — Asset do papel de segurança descreve o entregável da Wave 0.
- [ ] **AC7** — `CLAUDE.md` semeado (`internal/generators/claudemd.go:70` e equivalentes): a diretiva
      *Security wave* deixa de descrever só a barreira final.
- [ ] **AC8** — Paridade byte-a-byte dos assets entre os 3 CLIs mantida (gate existente verde).
- [ ] **AC9** — Falsificação em **duas direções**: (a) gerador que deixa de emitir Wave 0 é detectado;
      (b) `barrier --wave 0` voltando a ser recusado é detectado.
- [ ] **AC10** — `docs/cli-parity.md` atualizado onde o contrato mudar; checker de cobertura exit 0.
- [ ] **AC11** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0, com **exit code medido**.

## Negative scope

- **Não** cria regra bloqueante em `trackfw validate` para roadmap sem Wave 0 — o ADR rejeita, com o
  motivo (`guard que atrapalha é guard que o usuário desliga`).
- **Não** altera a barreira final: ela continua existindo e respondendo outra pergunta.
- **Não** muda a semântica de nenhum comando de entrega (`commit`/`push`/`ship`) — apenas o texto que
  os ensina.
- **Não** renumera waves de roadmaps existentes.
- **Não** resolve a disciplina de medição (entregas declaradas verdes sem exit code). O ADR declara
  isso fora de escopo, e continua fora.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md`
