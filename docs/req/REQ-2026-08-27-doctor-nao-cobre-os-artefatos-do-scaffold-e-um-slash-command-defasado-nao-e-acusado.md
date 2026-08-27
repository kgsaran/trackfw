---
status: Open
date: 2026-08-27
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-com-propriedade-dada-pelo-caminho.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template.md"
---

# REQ: `doctor` não cobre os artefatos do scaffold, e um slash command defasado não é acusado

> Date: 2026-08-27 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Achado da barreira do ML-1B da REQ da Wave 0 (2026-08-23), **medido por mim em 2026-08-27**:

```
manifesto (~/.trackfw/integrations-manifest.json):  290 artefatos
artefatos de scaffold no manifesto:                   0

sem cobertura de integridade em lugar nenhum:
  .claude/commands/trackfw/*.md      (9 slash commands)
  scripts/trackfw-attention-*.sh
  scripts/trackfw-validate.sh
  .github/workflows/trackfw-gate.yml
```

O `doctor` compara disco **contra o manifesto**; sem entrada, não há o que comparar. O `validate`
cobre **dois** artefatos por regra dedicada (os dois guards) — o resto, nada.

**A consequência concreta:** um projeto pode ficar com o slash command **defasado** — ensinando a
estrutura antiga de roadmap, sem Wave 0 — e **nada acusa**. Só o `trackfw update` revela, e ele
**corrige no mesmo passo**: o usuário nunca fica sabendo que esteve defasado, nem por quanto tempo.

Decisão de desenho em
`ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-com-propriedade-dada-pelo-caminho.md`.

## Acceptance Criteria

- [ ] **AC1** — `doctor` reporta artefato de scaffold **divergente** do template da versão corrente,
      nos **3 CLIs**.
- [ ] **AC2** — Reporta artefato de scaffold **ausente** onde o projeto já é um projeto trackfw.
- [ ] **AC3** — Propriedade por **caminho** (`.claude/commands/trackfw/`, `scripts/trackfw-*.sh`,
      `.github/workflows/trackfw-gate.yml`) — **nenhum** registro novo no manifesto, **nenhuma**
      migração.
- [ ] **AC4** — 🔴 **Zero ruído em projeto íntegro:** projeto recém-atualizado reporta
      `no mismatches`. É o critério que reprova a entrega.
- [ ] **AC5** — A mensagem distingue **"o seu projeto está defasado"** de **"o seu binário está
      defasado"** — rodar binário antigo em projeto novo não pode acusar o projeto.
- [ ] **AC6** — `doctor` continua **informando, não corrigindo**; o remédio nomeado é
      `trackfw update`.
- [ ] **AC7** — Saída determinística e byte-idêntica nos 3 CLIs, com `--json`.
- [ ] **AC8** — Falsificação em **duas direções**: (a) artefato divergente que **deixa** de ser
      reportado; (b) artefato íntegro reportado como divergente.
- [ ] **AC9** — `docs/cli-parity.md` com o contrato e anotação `trackfw-contract`; checker de
      cobertura exit 0.
- [ ] **AC10** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0, com **exit code medido**.

## Negative scope

- **Não** registra artefatos de scaffold no manifesto — o ADR rejeita, pelo custo de migração.
- **Não** cria regra nova em `trackfw validate`.
- **Não** faz o `doctor` corrigir nada.
- **Não** cobre os arquivos de regra (`CLAUDE.md`, `AGENTS.md`, …): são **editáveis pelo usuário por
  desenho**, e acusá-los seria falso-positivo garantido.
- **Não** muda o que o `Scaffold` escreve.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-com-propriedade-dada-pelo-caminho.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template.md`
