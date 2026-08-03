---
status: Done
date: 2026-08-02
author: "Zeus"
adr: "docs/adr/ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-tipados.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md"
---

# REQ: Unificar a leitura do trackfw.yaml em um unico carregador nos tres CLIs

> Date: 2026-08-02 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

O ciclo #106 substituiu os parsers artesanais de `trackfw.yaml` por biblioteca YAML nos três CLIs.
Resolveu o **carregador**. Não resolveu os consumidores que nunca o usaram.

Sobrevivem **cinco** scanners linha-a-linha, em `update` e `sync`, lendo o mesmo arquivo com uma
gramática diferente da do carregador:

| CLI | Local | Função |
|---|---|---|
| Go | `internal/generators/update.go:19` | `ReadUpdateConfig` |
| Go | `internal/sync/linear.go:119` | `readConfigField` (compartilhada com `jira.go`) |
| Node | `npm/src/commands/update.js:15` | `readUpdateConfig` |
| Node | `npm/src/commands/sync.js:18` | `readConfigField` |
| Python | `pypi/trackfw/commands/sync.py:22` | `_read_config_field` |

Essa é a mesma classe de defeito que o #106 eliminou, viva em outro endereço: com `wip_limit: "3"`
o carregador lia `3` e o `validate` reportava `1`. Hoje, `config.Load()` e `update`/`sync` entendem
gramáticas distintas do mesmo arquivo — nenhum dos scanners entende lista, mapa inline, âncora ou
escalar multi-linha, e `readConfigField` casa a **primeira** linha com o prefixo em qualquer
indentação, de modo que uma chave aninhada homônima sequestra o valor em silêncio.

**E o levantamento achou uma lacuna funcional que ninguém tinha registrado:** o `update` do Python
não lê nenhum desses campos. Go e Node leem `hooks`/`ci`/`backend`/`frontend`/`pkg_manager` e
decidem com base neles quais git hooks e qual workflow de CI gerar; o Python não tem o leitor
(`grep -rn pkg_manager pypi/trackfw` retorna vazio). Não é divergência de implementação — é
funcionalidade **ausente** em um dos três CLIs, invisível porque nenhum gate compara o
comportamento de `update` entre eles.

## Acceptance Criteria

- [ ] **AC1 — Caminho único de parsing.** Nos três CLIs, nenhum módulo fora de `config` abre, lê ou
      parseia `trackfw.yaml`. Verificável por busca: `ReadFile`/`readFileSync`/`open(` sobre
      `trackfw.yaml` retorna **exatamente uma** ocorrência por CLI — a do carregador. As cinco
      funções da tabela acima deixam de existir.
- [ ] **AC2 — Namespaces tipados.** `ProjectConfig` ganha `Update` (`hooks`, `ci`, `backend`,
      `frontend`, `pkg_manager`) e `Sync` (`linear_api_key`, `linear_team_id`, `jira_base_url`,
      `jira_email`, `jira_token`, `jira_project`), com equivalentes em Node e Python. As chaves no
      arquivo YAML permanecem **planas na raiz**, com os mesmos nomes de hoje.
- [ ] **AC3 — Paridade de valor lida.** Para um `trackfw.yaml` de fixture contendo os onze campos,
      os três CLIs resolvem **o mesmo valor** para cada um. Provado executando os três binários e
      diferenciando a saída — não comparando literais no código.
- [ ] **AC4 — YAML válido passa a funcionar.** Casos que hoje falham nos scanners e devem passar:
      valor entre aspas (`ci: "github"`), comentário à direita (`hooks: husky  # comentário`),
      chave aninhada homônima em outro mapa **não** sequestra o valor de raiz, e escalar citado com
      dois-pontos interno (`jira_base_url: "https://x.atlassian.net:443"`) resolve íntegro.
- [ ] **AC5 — Precedência preservada.** `sync` mantém a ordem atual: valor de `trackfw.yaml`
      primeiro, variável de ambiente como fallback. Mesma ordem nos três. Nenhuma mensagem de erro
      de `sync`/`update` muda de texto.
- [ ] **AC6 — Python alcança Go e Node no `update`.** `trackfw update` do Python lê os cinco campos
      e age sobre eles. Teste **demonstra a mudança**: com `hooks: husky` no fixture, o `update` do
      Python produz o mesmo efeito observável que Go e Node — não basta que os testes existentes
      continuem verdes.
- [ ] **AC7 — Proteção de falsificação.** Cenários novos em `scripts/check-gates-falsify.sh`, um
      por CLI, com braço de baseline e braço de detecção: reintroduzir a leitura artesanal faz o
      gate **falhar**. Cada cenário precisa de fixture **discriminante** — um valor que o scanner
      artesanal resolve errado e o carregador resolve certo (AC4 fornece os candidatos). Cenário
      cuja fixture ambos resolvem igual é vacuoso e não conta.
- [ ] **AC8 — Documentação.** `docs/cli-parity.md` e a documentação de configuração listam os onze
      campos. A lacuna do `update` do Python deixa de ser exceção não registrada.

## Negative Scope — o que esta REQ NÃO faz

- **Não decide se segredo pode morar em arquivo versionado.** `linear_api_key` e `jira_token` são
  roteados pelo carregador como **preservação mecânica** — sem isso um scanner artesanal
  sobreviveria e a AC1 seria falsa. Isso **não** endossa o desenho. A decisão fica com o sucessor
  nomeado no ADR (*material secreto de integração exclusivamente por variável de ambiente*), com
  revisão do Hades. Nenhum comportamento observável desses dois campos muda aqui.
- **Não altera o comportamento de `update` e `sync`** além do exigido pela AC6. Não muda o que os
  campos significam, nem quais hooks/CI são gerados, nem os endpoints chamados.
- **Não toca nas checagens de existência** (`os.Stat`/`existsSync` em `configure`, `update`,
  `discover`) — não parseiam nada.
- **Não toca nos produtores** — `configure`, `init` e `scaffold` **geram** o `trackfw.yaml`.
- **Não toca no shell gerado.** Os git hooks emitidos por `scaffold.go:704,731`,
  `hooks.js:77,104` e `init_gen.py:790,818` extraem `roadmap_dir` com `grep`/`sed`. É um sexto
  caminho de parsing, deliberadamente mantido: roda **sem o CLI presente**, e roteá-lo pelo
  carregador exigiria invocar o binário dentro do hook. Registrado para não ser lido como omissão.
- **Não muda o formato do `trackfw.yaml`.** Nenhuma chave é renomeada, aninhada ou removida.

## Linked ADR

ADR: docs/adr/ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-tipados.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md
