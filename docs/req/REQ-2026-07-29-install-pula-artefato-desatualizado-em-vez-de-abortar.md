---
status: Open
date: 2026-07-29
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md"
---

# REQ: install pula artefato desatualizado em vez de abortar

> Date: 2026-07-29 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

`trackfw init --ai-tools gemini`, executado dentro de um projeto novo, **aborta o scaffold inteiro**
quando o harness global do usuário contém um artefato trackfw desatualizado:

```
artifact "/Users/<user>/.gemini/agents/trackfw-architect.md" is outdated; use update
```

Constatado empiricamente durante a validação da Wave 5 do roadmap da barrier. A origem é o preflight
de `mutationInstall` em `IntegrationManager`: um artefato `outdated` **e** `owned` retorna erro, e
como `mutate` é um lote atômico com rollback, o erro descarta a operação completa. O usuário fica
impedido de inicializar um projeto novo por causa do estado de um artefato que não pertence a esse
projeto.

`install` não é caminho de upgrade — `update` é. Pular um artefato `owned`+`outdated` não perde
informação alguma: seus bytes são um template trackfw anterior, não conteúdo do usuário. Abortar o
lote é a resposta desproporcional.

## Premissa anterior invalidada — não reabrir

A primeira versão desta REQ afirmava que o defeito era `init --ai-tools` **alcançar o HOME do
usuário**, e propunha torná-lo project-scope. Essa premissa foi **verificada e refutada** antes de
qualquer implementação:

1. `ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md` decide o contrário, de
   forma deliberada — **D1**: sem TTY e sem `--scope`, default `global`, explicitamente registrado
   como breaking change em relação ao comportamento anterior (`project`); **D4**: `trackfw init`
   pergunta o escopo no wizard, e sem TTY → `global`. A consequência positiva declarada no ADR é
   "elimina instalação surpresa no repositório de trabalho do usuário". O código em
   `internal/commands/init.go` é implementação fiel do D4, não regressão.
2. O contrato invocado pela versão anterior — `## trackfw update vs trackfw update harness` em
   `docs/cli-parity.md` — é escopado à **família `update`** ("Update is split by scope") e pina 5
   targets de projeto e 19 de harness, todos do domínio `update`. Não menciona `init` e não estabelece
   fronteira projeto/global geral para todos os comandos.
3. A evidência empírica citada prova que `init` alcança o HOME, o que o D4 manda. Não prova que
   alcançá-lo seja errado. O que a evidência realmente expõe é o abort desproporcional acima.

Reverter D1/D4 seria decisão consciente com consequências não mapeadas (`--scope` em `init` com
detecção por flag-set do D3, pré-seleção do prompt do D2, default de `list` do D6, migração de
artefatos órfãos) e exigiria ADR de emenda. **Foi decidido não reverter.** Esta REQ trata apenas do
defeito de robustez.

## Acceptance Criteria
- [ ] `install` sobre artefato `outdated` + `owned` sem `--force` pula o artefato, preserva seus bytes,
      aplica os demais itens do lote e retorna exit 0.
- [ ] `install` sobre artefato `modified` continua sendo erro sem `--force` — comportamento inalterado.
- [ ] `trackfw init --ai-tools <tool>` completa o scaffold com exit 0 mesmo com artefato global
      desatualizado, provado em teste com HOME isolado nos três runtimes.
- [ ] Aviso emitido em stderr, uma linha por artefato pulado, com caminho tilde-abreviado e comando de
      remediação correto por escopo (`update harness` para global, `update` para projeto).
- [ ] Strings de aviso byte-idênticas entre Go, Node.js e Python.
- [ ] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Linked ADR
ADR: `docs/adr/ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`

D1 e D4 permanecem em vigor e **não** são alterados por esta REQ. O ADR é referenciado porque define
o escopo de instalação cuja fronteira foi verificada durante a análise deste defeito.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`
