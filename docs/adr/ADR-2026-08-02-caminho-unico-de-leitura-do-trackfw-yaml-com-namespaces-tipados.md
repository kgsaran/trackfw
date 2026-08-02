---
status: Accepted
date: 2026-08-02
author: "Zeus"
---

# ADR: Caminho unico de leitura do trackfw.yaml com namespaces tipados

> Date: 2026-08-02 | Status: Accepted

## Context

O `ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-string-na-fronteira`
substituiu os parsers artesanais de `trackfw.yaml` por biblioteca YAML real nos três CLIs, com
normalização para string na fronteira. Ele resolveu o carregador — **mas não resolveu os
consumidores que nunca passaram pelo carregador.**

Durante a execução daquele trabalho o executor parou e reportou: `update` e `sync` leem
`trackfw.yaml` por conta própria, com scanners linha-a-linha, para campos que **não existem** no
`ProjectConfig`. Ampliar o contrato do struct é decisão de arquitetura, não de microlote — o
reporte estava correto e é o que este ADR decide.

### Levantamento — o que efetivamente parseia `trackfw.yaml` fora do carregador

Categoria (a), leitura **e parsing** fora de `config`:

| CLI | Local | Função | Campos |
|---|---|---|---|
| Go | `internal/generators/update.go:19` | `ReadUpdateConfig` | `hooks`, `ci`, `backend`, `frontend`, `pkg_manager` |
| Go | `internal/sync/linear.go:119` | `readConfigField` (usada também por `jira.go`) | `linear_api_key`, `linear_team_id`, `jira_base_url`, `jira_email`, `jira_token`, `jira_project` |
| Node | `npm/src/commands/update.js:15` | `readUpdateConfig` | idem Go |
| Node | `npm/src/commands/sync.js:18` | `readConfigField` | idem Go |
| Python | `pypi/trackfw/commands/sync.py:22` | `_read_config_field` | idem Go |
| Python | `pypi/trackfw/commands/update.py` | **não existe** | — |

São **cinco** scanners artesanais sobreviventes, não dois. E o levantamento revelou uma divergência
de paridade que ninguém tinha registrado:

> **O `update` do Python não lê nenhum desses campos.** Go e Node leem
> `hooks`/`ci`/`backend`/`frontend`/`pkg_manager` e usam esses valores para decidir quais hooks e
> qual workflow de CI gerar. O Python não tem o leitor — `grep -rn pkg_manager pypi/trackfw`
> retorna vazio. Não é "mesma lógica, loader diferente": é funcionalidade ausente em um dos três
> CLIs, mascarada por não haver gate que compare o comportamento de `update` entre eles.

Categorias fora de escopo, separadas deliberadamente:

- **(b) checagem de existência** — `os.Stat`/`existsSync` em `configure.go:23`, `update.go:65,581`,
  `discover.js:28`, `discover.py:86`, `update.py:351`. Não parseiam nada.
- **(c) escrita** — `configure` e `init`/`scaffold` **geram** o arquivo. São produtores, não
  consumidores.
- **(d) shell gerado** — os git hooks emitidos por `scaffold.go:704,731`, `hooks.js:77,104` e
  `init_gen.py:790,818` extraem `roadmap_dir` com `grep`/`sed`. É um sexto caminho de parsing, mas
  roda **sem o CLI presente**, por construção: rotear pelo carregador exigiria invocar o binário
  dentro do hook. Fora de escopo, registrado para não ser confundido com omissão.

### Por que isso não é cosmético

É exatamente a classe de defeito que o ciclo anterior eliminou no carregador, sobrevivendo intacta
em outro lugar. Com `wip_limit: "3"` o carregador lia `3` e o `validate` reportava `1` — o mesmo
formato de divergência hoje é possível entre o que `config.Load()` entende e o que `update`/`sync`
leem, porque são gramáticas diferentes do mesmo arquivo. Concretamente, os scanners sobreviventes
não entendem lista, mapa inline, âncora, escalar multi-linha ou chave aninhada, e `readConfigField`
casa a **primeira** linha com o prefixo em qualquer nível de indentação — uma chave aninhada
homônima sequestra o valor em silêncio.

## Decision

**Existe um único caminho de leitura de `trackfw.yaml` em cada CLI: o carregador de `config`.**
Nenhum outro módulo abre, lê ou parseia o arquivo. Os cinco scanners artesanais são removidos.

Os campos hoje lidos fora do carregador passam a integrar o contrato de configuração em **dois
namespaces tipados**, não diluídos no `ProjectConfig` raiz:

- `ProjectConfig.Update` — `hooks`, `ci`, `backend`, `frontend`, `pkg_manager`
- `ProjectConfig.Sync` — `linear_api_key`, `linear_team_id`, `jira_base_url`, `jira_email`,
  `jira_token`, `jira_project`

Os nomes das chaves no arquivo **não mudam**: seguem planas na raiz do YAML, como hoje. O namespace
é da estrutura em memória, não do formato.

O `update` do Python passa a ler os cinco campos e a usá-los, alcançando os outros dois CLIs.

### Por que namespaces, e não campos planos no ProjectConfig

O `ProjectConfig` tem hoje 20 campos, todos de **governança** — diretórios, limites de WIP,
severidades, marcadores. Os onze campos novos são de **ferramentaria**: scaffolding de projeto e
credenciais de tracker. Achatá-los junto faz o struct deixar de ter um assunto, e torna o próximo
leitor incapaz de distinguir "isto governa a cadeia ADR→REQ→ROADMAP" de "isto configura um cliente
HTTP". Os namespaces preservam a distinção sem custo: o parse continua sendo **um só**, e é o parse
único — não o formato do struct — que é o invariante desta decisão.

### Alternativa rejeitada — escotilha genérica `Raw map[string]string`

Expor o mapa cru do YAML e deixar cada consumidor colher a chave que quiser resolveria o mesmo
problema com menos código. Foi rejeitada porque **recria a divergência que o ciclo #106 eliminou**,
apenas sob outro nome: sem tipo declarado, cada consumidor volta a decidir sozinho como interpretar
o valor — e a normalização para string na fronteira só garante o *tipo de entrada*, não a
*interpretação*. O critério que separa as opções é: *a alternativa deixa exatamente um caminho de
parsing?* A escotilha deixa um caminho de **parsing** e N caminhos de **interpretação**, que é
metade da propriedade que se quer.

## Consequences

Positivas:

- Um único parser por CLI. Toda correção de parsing passa a valer para todos os consumidores.
- Fecha uma lacuna funcional real: `trackfw update` do Python passa a respeitar `hooks`/`ci`.
- Qualquer YAML válido passa a funcionar nesses campos — hoje só o subconjunto plano funciona.

Negativas e riscos:

- Onze campos novos no contrato de configuração. É superfície que precisa de documentação e de
  gate de paridade, senão volta a divergir.
- O `update` do Python muda de comportamento — hoje ignora `hooks`/`ci` e passará a agir sobre eles.
  Alinhar ao comportamento correto **é** o objetivo, mas é mudança observável e precisa de teste que
  a demonstre, não apenas de teste que não quebre.

### O que esta decisão explicitamente NÃO decide

`linear_api_key` e `jira_token` são **material secreto**, e hoje são lidos de um arquivo que os
projetos versionam. Rotear esses dois campos pelo carregador é **preservação mecânica** do
comportamento atual — necessária para que nenhum scanner artesanal sobreviva — e **não** é
endosso ao desenho.

Se segredo deve ser lido de arquivo versionado é uma decisão de segurança **em aberto**, com revisão
do Hades pendente. Sucessor nomeado: *ADR — material secreto de integração exclusivamente por
variável de ambiente*. Até lá, nada muda no comportamento observável desses dois campos.

Registrar isto aqui é o ponto: ampliar o contrato sem esta ressalva ratificaria em silêncio, por
omissão, um desenho que ninguém avaliou.
