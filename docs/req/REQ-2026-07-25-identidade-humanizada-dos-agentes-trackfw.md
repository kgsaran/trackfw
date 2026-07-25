---
status: Open
date: 2026-07-25
author: "Zeus (Principal Software Architect)"
adr: "ADR-2026-07-25-identidade-personalizavel-de-agentes.md"
roadmap: "ROADMAP-2026-07-25-identidade-humanizada-dos-agentes.md"
---

# REQ: Identidade humanizada dos agentes trackfw

> Date: 2026-07-25 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Os 10 agentes distribuidos pelo trackfw se apresentam com identidade neutra
(`trackfw-architect`, `trackfw-backend`, ...). Nao ha como o usuario:

- dar um nome proprio a cada agente ("Zeus", "Apolo");
- invocar o agente por esse nome (`@agent-zeus-tf`);
- ser tratado por um apelido nas respostas.

O resultado e uma interacao impessoal. A personalizacao e viavel a baixo custo
porque `Render()` e o funil unico de geracao de artefatos e a documentacao
oficial confirma que `name` nao precisa coincidir com o nome do arquivo.

Decisoes arquiteturais, evidencias e alternativas rejeitadas estao no ADR
vinculado.

## Acceptance Criteria

- [ ] `~/.trackfw/identity.json` (schema_version 1) persiste `user_nickname` e,
      por `id` de agente, `display_name` + `slug`.
- [ ] Ausencia do arquivo, `agents` vazio ou entrada ausente produz artefatos
      **byte a byte identicos** aos atuais (nao-regressao).
- [ ] `name` do frontmatter recebe o slug com sufixo `-tf`; `display_name` sem
      sufixo aparece no `description` e no corpo.
- [ ] `description` personalizado preserva o sufixo de papel
      (ex: `Zeus — Principal Software Architect | ...`).
- [ ] `id` canonico e path de instalacao (`trackfw-{{id}}`) permanecem
      inalterados; nenhuma chave de manifest muda.
- [ ] `agentTools` decide SET_ARCH por `item.ID == "architect"`, nao por
      `strings.HasSuffix(name, "architect")`.
- [ ] Slugificacao de texto livre e identica nos 3 CLIs, validada por tabela de
      vetores replicada (acentos, maiusculas, espacos, emoji, vazio, colisao).
- [ ] Slug invalido (vazio pos-normalizacao ou > 40 chars) e rejeitado com erro
      explicito, nunca corrigido silenciosamente.
- [ ] Slugs duplicados entre os 10 agentes sao rejeitados.
- [ ] Preset `greek` usa slugs hardcoded, sem depender de normalizacao runtime.
- [ ] Colisao de `name` no diretorio de destino gera aviso e exige `--force`.
- [ ] Todos os callers de `BuildPlans` resolvem identidade (Go: 4 pontos;
      Node: 2; Python: equivalentes) — `update` nao reverte a personalizacao.
- [ ] `init --identity-preset greek|neutral|none` funciona; ramo `!IsTerminal`
      nunca bloqueia em prompt; re-executar `init` reutiliza o config.
- [ ] Agente nao le configuracao em runtime: nenhuma instrucao de leitura de
      arquivo e injetada no corpo.
- [ ] Paridade verde nos 3 CLIs: `make quality` passa
      (`check-cli-parity.sh`, `check-integration-assets.sh`).
- [ ] Chaves i18n novas presentes em `pt-BR`, `en-US` e `es-ES` nos 3 CLIs.
- [ ] Documentacao atualizada (`docs/cli-parity.md` e README dos 3 pacotes).

## Linked ADR

ADR: docs/adr/ADR-2026-07-25-identidade-personalizavel-de-agentes.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-25-identidade-humanizada-dos-agentes.md
