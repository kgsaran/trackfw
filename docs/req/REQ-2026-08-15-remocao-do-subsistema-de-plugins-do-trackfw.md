---
status: Open
date: 2026-08-15
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md"
---

# REQ: Remoção do subsistema de plugins do trackfw

> Date: 2026-08-15 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

**Substitui** a REQ anterior
`REQ-2026-08-15-gate-de-seguranca-para-trackfw-plugins-install-download-de-binario-de-terceiro-sem-parecer-previo.md`
(fechada), que propunha **cercar** o download de binário de terceiro com um gate de duas fases.

**Proposta de KG (2026-08-15): não cercar — remover.** Se o trackfw não baixa, não instala e não
executa código de terceiro, o vetor deixa de existir em vez de ser mitigado.

O parecer de segurança que motivou a mudança de rumo está preservado em
`docs/seguranca/2026-08-15-gate-de-plugins-binario.md`. Ele demonstrou que o gate entregaria
**menos** do que o equivalente para markdown:

- a detecção de **instalação sem aprovação** é **estruturalmente impossível** com
  `~/.trackfw/plugins` global e compartilhado entre projetos — qualquer regra dispararia
  falso-positivo entre projetos não relacionados;
- o `chmod` tardio é **redução de janela, não controle** — nada impede um agente com `Bash` de dar
  `chmod +x`;
- sem verificação de origem, seria gate de **revisão**, não de **supply-chain**;
- e o revisor **não consegue ler um binário** como lê markdown, então o parecer certificaria
  proveniência aceita, nunca ausência de malícia.

Ou seja: o gate custaria uma superfície nova inteira (quarentena de binário, artefato de revisão,
índice de checksums, script de paridade) para entregar uma garantia parcial. **Remover custa menos e
garante mais.**

### O que a remoção resolve

- Elimina o download de binário de terceiro e o `chmod 0755`.
- Elimina a execução de código de terceiro pelo trackfw — incluindo o fallback de
  `internal/commands/root.go:71-74`, que hoje faz **qualquer argumento desconhecido** virar execução
  de plugin (`trackfw vaildate` executa `trackfw-vaildate`). Isso fecha o débito D9 do ADR anterior.
- **Alinha os 3 CLIs por remoção, não por adição:** o Python nunca teve instalação de plugin, e
  passará a não ter execução também. Some a necessidade de exceção de paridade documentada.

### O que a remoção NÃO faz

- Não impede o usuário de instalar e rodar ferramentas `trackfw-*` por conta própria — **essa passa
  a ser inteiramente responsabilidade dele**, invocando o binário direto no shell. É o objetivo,
  não um efeito colateral.
- Não remove nada de `agents`/`skills`, que são subsistemas distintos (`internal/integrations/`).

## Acceptance Criteria

- [ ] **AC1** — Removidos, nos CLIs onde existirem: `plugins add`, `plugins search`,
      `plugins list`, `plugins remove`, e todo o código de download/registry
      (`internal/plugins/`, a parte equivalente em `npm/src/commands/plugins.js`,
      `pypi/trackfw/commands/plugins.py`).
- [ ] **AC2** — Removida a execução de plugin: `RunPlugin` e o fallback de argumento desconhecido em
      `internal/commands/root.go:71-74`, mais os equivalentes Node e Python.
- [ ] **AC3** — Argumento desconhecido passa a produzir **erro de comando desconhecido**, com
      mensagem idêntica nos 3 CLIs — nunca execução de binário.
- [ ] **AC4** — Nenhuma referência remanescente a `~/.trackfw/plugins` nem ao registry externo
      (`RegistryURL`) em código de produto.
- [ ] **AC5** — `README.md`, `CLAUDE.md` e `docs/cli-parity.md` atualizados; `check-cli-parity.sh`
      deixa de exigir `plugins` na lista de comandos de piso.
- [ ] **AC6** — `make quality` verde; testes de plugins removidos junto com o código que cobriam
      (não deixar teste órfão).
- [ ] **AC7** — **Breaking change registrado**: `CHANGELOG.md` com seção de Breaking Changes e bump
      para **7.0.0** — em **PR próprio**, separado do PR de remoção.

## Escopo negativo

- **Não** mexe em `agents`/`skills` nem no gate de artefato markdown já mergeado.
- **Não** propõe substituto para plugins (sem sistema de extensão novo). Se um dia fizer sentido,
  nasce de REQ própria, com o gate desenhado desde o início.
- **Não** remove o repositório externo `kgsaran/trackfw-plugins` — decisão de fora deste repo.

## Linked ADR

ADR: `docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`
