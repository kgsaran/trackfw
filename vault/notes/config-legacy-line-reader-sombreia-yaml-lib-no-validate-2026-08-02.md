# `trackfw validate`'s `wip_limit`/`governance_mode`/`lenient_until` continuam lidos por um parser artesanal linha-a-linha, à parte da migração para biblioteca YAML

> Data: 2026-08-02 | Autor: Ártemis (ML-2A, barreira do ROADMAP-substituir-os-parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis) | Domínio: config / validator — os 3 CLIs

## O achado

A Wave 1 desta REQ substituiu o parser artesanal de `trackfw.yaml` por uma biblioteca
YAML real nos três CLIs — mas **só** em `internal/config/config.go` /
`npm/src/config/index.js` / `pypi/trackfw/config.py` (a struct/dict `ProjectConfig`).

`internal/validator/validator.go` (e os gêmeos `npm/src/validator/index.js` /
`pypi/trackfw/validator.py`) têm um **segundo parser, independente**, que também lê
`trackfw.yaml` diretamente do disco — mas com `strings.Split`/`strings.HasPrefix` +
`fmt.Sscanf("%d", ...)` (Go), regex/`split` linha-a-linha (Node) e equivalente em
Python. Esse segundo parser nunca passa pela biblioteca YAML nem pela normalização
para string na fronteira que esta REQ introduziu — é exatamente o tipo de "parser
artesanal" que a REQ existe para eliminar, só que ele sobreviveu à Wave 1 porque não
está em nenhum dos três arquivos que o ML-1A tocou.

Funções afetadas: `readWIPConfig()` e `readGovernanceMode()` em
`internal/validator/validator.go` (linhas ~160–189 e ~282–312), que alimentam as
regras `wip_limit` (`validateWIPLimit()`) e o modo `lenient` (`IsLenient()` /
`LenientUntilDate()`) — os efeitos SÃO visíveis em `trackfw validate` (mensagem de
warning de WIP limit, e a suspensão condicional de regras em modo lenient).

`ProjectConfig.WipLimit` / `.WipBySquad` / `.GovernanceMode` / `.LenientUntil` (os
campos homônimos na struct/dict, alimentados corretamente pela biblioteca YAML e
cobertos por `internal/config/config_yaml_fidelity_test.go`) **existem e estão
corretos**, mas são lidos apenas por `internal/generators/scaffold.go` (para GERAR um
`trackfw.yaml` novo) e por testes — nunca pelo caminho real de `validate`. Ou seja: o
código que prova AC3 do ADR (fidelidade textual `010`→`10`, `yes` preservado) está
correto e testado, mas é **inatingível** a partir do comando `validate` para essas
quatro chaves especificamente.

## Como foi confirmado (evidência, não inferência)

`wip_limit: "3"` (valor **citado/quoted** — YAML válido, resolve para a string `"3"`
sem as aspas) num projeto com 2 roadmaps em `wip/`:

```
$ trackfw validate   # Go, Node — idêntico
⚠  2 roadmaps in wip/ (limit: 1) — consider focusing
```

Se o valor passasse pela biblioteca YAML (como `ProjectConfig.WipLimit` corretamente
faz — `parseInt("3", ...)` = 3), o limite seria 3 e nenhum warning apareceria. O
`(limit: 1)` prova que o texto `"3"` (com as aspas) chegou inteiro em
`fmt.Sscanf("%d", ...)`, que falha em parsear `"3"` (não é um inteiro válido com
aspas) e cai no default (`Limit: 1`). Reproduzido idêntico em Go e Node; Python não
imprime o warning de `wip_limit` nesse fluxo específico (formato de saída diferente,
não investigado — fora do escopo deste achado), mas a mesma leitura por
`strings.HasPrefix` está presente em `pypi/trackfw/validator.py`.

## Por que isso não bloqueia o ML-2A

- **Não é regressão desta REQ** — o comportamento (parser separado, artesanal) já
  existia antes da Wave 1 e continua idêntico nos 3 CLIs (sem divergência cross-CLI
  introduzida).
- **Está fora do escopo declarado do ML-1A** — arquivos afetados listados no roadmap
  são só `config.go`/`config/index.js`/`config.py`; `validator.go` e equivalentes não
  estão na lista.
- Portanto não é um "defeito de paridade" a corrigir aqui — é uma dívida a registrar
  para quem tocar `wip_limit`/`governance_mode`/`lenient_until` a seguir.

## Impacto no critério de aceite do roadmap

O AC "Os 3 carregam com biblioteca YAML; escalares chegam aos consumidores como
string" é verdadeiro para o caminho `config.Load()`/`ProjectConfig` — mas **falso**
para essas quatro chaves quando consumidas via `trackfw validate`, porque
`validate` não usa `ProjectConfig` para elas. Um roadmap futuro que queira fechar essa
lacuna deveria: (a) remover `readWIPConfig`/`readGovernanceMode` (e os gêmeos Node/
Python) e (b) fazer `validateWIPLimit()`/`IsLenient()`/`LenientUntilDate()` lerem
`config.Load().WipLimit` etc. diretamente — os campos já existem e já estão corretos,
só não são chamados.

## Como reconhecer se isso mudar (regride ou é corrigido)

- Se corrigido: `wip_limit: "3"` citado passaria a respeitar o limite 3 em `validate`.
- Se uma fixture de falsificação futura testar `wip_limit`/`governance_mode`/
  `lenient_until` esperando que a normalização YAML os alcance via `validate`, ela
  será **vácua** até essa dívida ser paga — é por isso que o Cenário 36
  (`scripts/check-gates-falsify.sh`) usa `roadmap_dir`/`req_dir`/`adr_dirs` em vez
  dessas quatro chaves para provar a fidelidade de schema (AC3) por `trackfw status`.

Relacionado: `vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`,
`vault/notes/falsificacao-fixture-vacua-contra-reversao-total-vs-parcial-2026-08-02.md`.
