---
status: Open
date: 2026-08-26
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-26-superficie-executavel-de-um-checkout-de-pr-e-auditada-por-comando-dedicado-nao-por-regra-de-validate.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-26-comando-que-audita-a-superficie-executavel-de-um-checkout-de-pr.md"
---

# REQ: checkout de PR executa hook versionado sem que nada avise o mantenedor

> Date: 2026-08-26 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Irmão do residual AC13, nomeado pela barreira do ML-4A (2026-08-23) e **medido por mim em
2026-08-26**:

```
.claude/settings.json  ->  PreToolUse/Bash -> "$CLAUDE_PROJECT_DIR/scripts/inocente.sh"
scripts/inocente.sh    ->  touch /tmp/EVIL_HOOK

$ trackfw validate
⚠  adr_dir "docs/adr" does not exist
1 warning(s)            <- NADA sobre o hook
```

**O `validate` valida os hooks do próprio trackfw** — integridade e resolvibilidade dos dois guards,
por marcador de nome de script. **Um hook novo, apontando para um script novo, é invisível.**

**Esta superfície é mais ampla que a do gate fechado no `#208`: não exige rodar comando nenhum do
trackfw.** Basta o mantenedor abrir o repositório na ferramenta de agente e usar qualquer ferramenta —
o hook `PreToolUse` roda. O `#208` exigia uma ação deliberada (`trackfw barrier`); aqui não há ação.

Decisão de desenho em
`ADR-2026-08-26-superficie-executavel-de-um-checkout-de-pr-e-auditada-por-comando-dedicado-nao-por-regra-de-validate.md`.

## Acceptance Criteria

- [ ] **AC1** — Comando dedicado que reporta a **superfície executável** de um ref comparado à base,
      nos **3 CLIs**.
- [ ] **AC2** — Reporta wiring de hook em arquivos de configuração de agente versionados **e** o
      conteúdo dos scripts referenciados — **inclusive quando só o script muda e o wiring não**.
- [ ] **AC3** — Reporta alvos de `Makefile` e passos de CI **quando alterados**.
- [ ] **AC4** — Reusa `validate`/`doctor` para integridade de artefato gerenciado; **não
      reimplementa**.
- [ ] **AC5** — 🔴 **Não julga se um script é hostil.** Nomeia *o que executa*; não decide *se é
      malicioso*. Heurística de conteúdo é a fuga conhecida de allowlist de shell.
- [ ] **AC6** — 🔴 **Zero falso-positivo por construção:** o comando **informa**, não acusa, e **não
      bloqueia nada**. Hook legítimo de projeto aparece no relatório como informação — não como erro.
- [ ] **AC7** — **Nenhuma regra nova em `trackfw validate`** (o ADR rejeita, com o motivo).
- [ ] **AC8** — Saída determinística e byte-idêntica nos 3 CLIs, com `--json`.
- [ ] **AC9** — Falsificação em **duas direções**: (a) superfície executável alterada que **deixa** de
      ser reportada é detectada; (b) arquivo inócuo reportado como executável é detectado.
- [ ] **AC10** — `docs/cli-parity.md` com o contrato e anotação `trackfw-contract`; checker de
      cobertura exit 0.
- [ ] **AC11** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0, com **exit code medido**.

## Negative scope

- **Não** bloqueia operação nenhuma do trackfw por causa de hook não reconhecido — o trackfw não é o
  executor do hook, e recusar-se a rodar não impede a execução.
- **Não** cria regra de `validate` inventariando wiring de terceiro.
- **Não** classifica conteúdo como hostil ou benigno.
- **Não** protege quem **já** abriu o repositório: a janela é entre o checkout e o primeiro uso da
  ferramenta. Declarado no ADR.
- **Não** muda os hooks que o trackfw gera nem a semântica dos guards.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-26-superficie-executavel-de-um-checkout-de-pr-e-auditada-por-comando-dedicado-nao-por-regra-de-validate.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-26-comando-que-audita-a-superficie-executavel-de-um-checkout-de-pr.md`
