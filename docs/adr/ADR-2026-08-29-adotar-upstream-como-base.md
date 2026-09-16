---
status: Accepted
date: 2026-08-29
author: claude
---

# ADR: Adotar o upstream como base e sincronizar por merge de históricos

> Date: 2026-08-29 | Status: Accepted

REQ: REQ-2026-08-29-migrar-para-upstream-7.3.0

## Context

Este repositório nasceu de um **ZIP** do `kgsaran/trackfw`, não de um `clone`. O commit inicial
(2026-06-28) importou a árvore da v2.12.2 como se fosse código próprio.

Consequências medidas em 2026-08-29:

- O upstream está na **v7.3.0**: 26 releases e **cinco majors** à frente, 420 commits contra 90.
- `git merge upstream/main` responde `fatal: refusing to merge unrelated histories` — os dois
  históricos não têm **nenhum** ancestral comum.
- Sem ancestral, cada atualização exigia baixar um ZIP e reconciliar à mão. Foi o que aconteceu.

O custo disso ficou explícito: das 18 PRs desta sessão, **sete** reimplementaram correções que o
upstream já tinha — slug com dobra NFKD, `req move`, `req list`, `adr list`, sincronização de status
no `roadmap move`, aviso de lista inline no config, e o estado `analyzing`, que cheguei a reportar
como quebrado quando na verdade só estava desatualizado aqui.

O repositório também **não é fork no GitHub** (`isFork: false`), e o dono do upstream é outra pessoa
(`kgsaran`), então não há botão "Sync fork" nem relação de PR entre os dois.

## Decision

Adotar o upstream como base do produto e estabelecer a relação de ancestralidade **de uma vez**, com
um merge de históricos:

```
git merge v7.3.0 --allow-unrelated-histories
```

O commit de merge passa a ser o ancestral comum. A partir dele, atualizar é o fluxo normal de fork:

```
git fetch upstream && git merge upstream/<tag>
```

**Política de resolução**, por área:

| área | fica com | motivo |
|---|---|---|
| `internal/`, `npm/`, `pypi/`, `cmd/`, `scripts/`, `site/`, `go.mod`, `go.sum`, `README.md`, `.gitignore` | **upstream** | é o produto; manter versão local é o que gerou a dívida |
| `docs/schema/`, `docs/visao-projeto/`, `docs/cli-parity.md`, `docs/gate-design-principles.md`, `docs/agents-working-context.md` | **upstream** | documentação do produto |
| `docs/adr/`, `docs/requisições/`, `docs/roadmaps/<agente>/`, `docs/roadmaps/.trackfw-log` | **local** | governança deste repositório |
| `trackfw.yaml`, `.trackfw-baseline.json`, `.gitattributes` | **local** | configuração deste repositório |
| `CLAUDE.md` | **mesclado** | instruções do produto vêm do upstream; a seção de dogfooding é local |

**A governança do upstream não é importada.** São 379 arquivos — 52 ADRs, 140 REQs e 142 roadmaps —
e o argumento é concreto: o `trackfw.yaml` local declara `adr_dirs: [docs/adr]`, então as 52 ADRs
dele cairiam exatamente onde estão as 6 daqui, poluindo `adr list` e `validate`. O mesmo vale para
`docs/roadmaps/`, que aqui é `by_agent` e lá é flat.

## Consequences

**Positivas.** Atualizar deixa de ser download e reconciliação manual. A divergência passa a ser
visível como conflito de merge, no arquivo exato, em vez de silenciosa. E o custo recorrente fica
proporcional ao quanto se diverge — o que dá um incentivo correto: mudanças próprias confinadas a
`docs/` e a poucos arquivos custam quase nada por release.

**Negativas.** O merge inicial é grande: 237 arquivos em conflito, 985 novos. E o salto de cinco
majors traz quebras que atingem este repositório — o subsistema de `plugins` foi removido na 7.0.0
por ser superfície de supply chain, os cinco aliases de integração saíram na 4.0.0, o `--scope`
inverteu o default na 3.0.0 e o `version` perdeu o prefixo `v` na 6.0.0.

O `.trackfw-baseline.json` foi gerado contra o validator da v2.12 e provavelmente não corresponde ao
da 7.3.0 — precisa ser regravado depois da migração.

**Sobre o trabalho local.** As correções desta sessão que o upstream **não** tem — o forçamento de
UTF-8 no CLI Python, o gate de paridade de subcomando, o `.gitattributes` e o `pytest` em
dev-dependencies — sobrevivem à migração e são candidatas naturais a contribuição para o upstream.
Isso é decisão do mantenedor deste repositório, não desta ADR.

## Nota de 2026-09-16 — uma exceção nomeada em `docs/`

**`docs/cli-parity.md` passa a vir do upstream a cada sync.** Não é governança: é o contrato de
comportamento do produto, e o `scripts/check-parity-contract-coverage.sh` do upstream o lê e reprova a
seção cuja anotação cita gate que não existe.

Retido desde o #259, o nosso ficou parado. A v8.0.0 (#365) apagou os gates que ele citava, e o efeito
foi medido na PR #135 deste fork: o `parity-other-gates` passou a reprovar em `Makefile:35`, antes do
barrier — tirando do CI todos os gates depois dessa linha —, e o `windows-defect-reproduction` travou
até o timeout de 20 minutos, porque o item 4 dele roda o mesmo gate e o `run.ps1:148` lê a saída
padrão antes da de erro.

A decisão é do mantenedor deste repositório (2026-09-16): trazer e manter trazendo. O critério de
entrada na lista `PRODUTO_EM_DOCS` do `scripts/upstream-sync.sh` é **um gate do upstream que leia o
arquivo** — "é documentação de produto" sozinho não basta. O resto de `docs/` continua retido.

## Alternatives Considered

**Re-fundar sobre a história do upstream**, com a governança local aplicada por cima. Histórico final
mais limpo. Descartada porque tiraria os 90 commits locais da linha do `main` — incluindo as 18 PRs
desta sessão, que são registro de decisão e de medição, não só código.

**Continuar sincronizando por ZIP.** É o estado atual. Descartada pela evidência: em dois meses
produziu cinco majors de atraso e sete reimplementações do que já existia.

**Recriar o repositório como fork de verdade no GitHub**, ganhando o botão "Sync fork". Descartada
porque o fork do GitHub exige repositório novo e as PRs #1–#18 não migram. O `git merge` pela linha
de comando entrega o mesmo resultado técnico.
