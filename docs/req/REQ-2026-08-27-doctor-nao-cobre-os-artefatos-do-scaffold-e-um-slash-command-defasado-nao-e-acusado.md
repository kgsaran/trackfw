---
status: done
date: 2026-08-27
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-com-propriedade-dada-pelo-caminho.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template.md"
---

# REQ: `doctor` não cobre os artefatos do scaffold, e um slash command defasado não é acusado

> Date: 2026-08-27 | Status: done
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

### Acrescentados pela Wave 0 (`docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md`)

- [ ] **AC11** — Inventário são **17** artefatos, não 13: faltavam `.gitlab-ci-trackfw.yml`
      (`ci: gitlab-ci`) e os arquivos de hook (husky/lefthook) — todos **condicionais**.
- [ ] **AC12** — 🔴 **`scripts/trackfw-validate.sh` tem conteúdo dependente de config**
      (`buildValidateScript` varia com `cfg.Backend`/`cfg.Frontend`). O `doctor` precisa renderizar o
      template a partir do **`trackfw.yaml` do projeto**, não de um cfg padrão embutido — senão
      **qualquer projeto com `backend:` configurado vira falso-positivo imediato** e o AC4 reprova.
- [ ] **AC13** — Artefato **condicional** (CI workflow, hooks) só é in-scope quando o `trackfw.yaml`
      o declara. Ausência de artefato não configurado **não** é achado.
- [ ] **AC14** — 🔴 **`discover --init` é um terceiro escritor** (`internal/discover/discover.go:49`):
      escreve validate-script, CI workflow, attention scripts e guards, **mas não slash commands**.
      Projeto inicializado só por ele tem ausência **legítima** de `.claude/commands/trackfw/*.md`.
- [ ] **AC15** — 🔴 **`ClassifyDoctor` não tem case para `!Registered && StateModified`** — hoje cai
      no `default` implícito e **não gera finding nenhum**. Sem esse case, o falso-negativo é
      garantido justamente para os artefatos que motivaram a REQ.
- [ ] **AC16** — **AC5 não é satisfazível por conteúdo:** nenhum artefato de scaffold carrega stamp de
      versão. A mensagem deve ser **neutra quanto à culpa** — *"difere do template que a versão X.Y.Z
      instalada geraria; se o projeto foi inicializado com uma versão mais nova, atualize o binário"*.

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
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template.md`
