# O gerador perdia a REQ **dentro do Body**, e o Cenário 24 fixa o bloco de ACs como literal

> 2026-09-26 · REQ-2026-09-09 (AC7) · ML-1B · `internal/generators/roadmap.go`

## O que custava tempo

`trackfw roadmap new --from-req docs/req/REQ-x.md` criava o roadmap com `req: "docs/req/REQ-x.md"` e
deixava a REQ com `roadmap: ""`. O `validate` então acusava `req_has_roadmap` **contra quem seguiu o
caminho documentado**. A leitura natural do código ("falta chamar o sync") leva ao lugar errado, por
duas razões que só aparecem lendo as duas funções juntas.

## Causa 1 — o `REQPath` morria dentro do `Body`

`NewRoadmapFromREQ` monta o markdown inteiro (incluindo `req: "<caminho>"`) numa string e chama
`NewRoadmapFromContent(RoadmapContent{Title, Body, Agent})` — **sem `REQPath`**. O campo existe na
struct e é usado pelo caminho `--req`, mas o caminho `--from-req` o deixava vazio: o roadmap
**declarava** a REQ e o gerador, uma linha depois, não sabia mais qual era.

Consequência prática: qualquer correção colocada em `NewRoadmapFromContent` e condicionada a
`content.REQPath != ""` fica **inerte** no caminho `--from-req` até que a chamada passe o campo. Foi
exatamente o que aconteceu na primeira medição deste ML — código correto, saída idêntica à de antes,
sem erro nenhum.

## Causa 2 — `syncREQReferences` não serve, e o motivo é a DESCOBERTA

`syncREQReferences` (usada por `roadmap move`, que **já** anuncia `✓ synced …`) varre as REQs e
reescreve as que **já apontam** para o basename do roadmap movido:

```go
fmVal := extractFrontmatterRoadmap(string(content))
if fmVal == "" || filepath.Base(normalizeRefSeparator(fmVal)) != roadmapBasename {
    continue // sem referência ou aponta para outro roadmap
}
```

Uma REQ recém-criada tem `roadmap: ""` — o primeiro ramo do `if` a **descarta por construção**. Ou
seja: a função que "já faz o sync" é justamente a que nunca alcança o caso do `roadmap new`.

O que se reaproveita não é a descoberta (aqui a REQ é nomeada pelo usuário, não descoberta), é o
**escritor**: `rewriteREQRoadmapRefWith` passou a receber o critério de sobrescrita como predicado, e
os dois consumidores compartilham a preservação de estilo de aspas/backticks, a normalização de CRLF
e a de separador. Duas cópias do escritor seriam o defeito que a REQ-2026-08-31 gastou um microlote
inteiro desfazendo.

### A assimetria dos dois predicados é deliberada

| campo | sobrescreve quando | por quê |
|---|---|---|
| frontmatter `roadmap:` | valor **não** termina em `.md` (`""`, `none`, `-`, `<!-- … -->`) | campo escrito por máquina: o gerador de REQ emite sempre `roadmap: ""` |
| corpo `Roadmap:` | **só** vazio, traço ou comentário HTML | prosa humana: `Roadmap: a decidir depois do ADR` também "não é .md", e sobrescrever apagaria o que a pessoa escreveu |

Efeito medido e aceito: a fixture `Roadmap: none` de `scripts/check-gates-falsify.sh` fica intacta —
e **não** produz aviso `req_roadmap_sync`, porque esse aviso exige os dois lados com valor `.md` e
`extractBodyRefPath` não lê `none` como referência.

## 🔴 Armadilha de instrumento: o Cenário 24 fixa o bloco de ACs como literal de 4 linhas

`remove_roadmap_acceptance_heading` (em `scripts/check-gates-falsify.sh`) removia este bloco **exato**
e abortava se não encontrasse **exatamente 2 ocorrências** em `internal/generators/roadmap.go`:

```
## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]
```

Preencher o bloco consolidado com os ACs da REQ no caminho `--from-req` derruba a contagem de 2 para
1, e o setup do cenário morre com `expected 2 occurrences of the heading block, got 1` — reprovação
que **não** é sobre o comportamento medido. Correção: o bloco do cenário passou a ser
**heading + comentário**, que continua byte-idêntico nos dois templates. O que o Cenário 24 mede é a
ausência do heading (`cfg.AcceptanceMarkers = "## Acceptance Criteria"`), então os itens que sobram
órfãos não carregam marcador nenhum e o seam continua valendo — verificado construindo os dois
binários corrompidos (occ 0 e occ 1) e rodando o ciclo: 1 ocorrência da violação esperada em cada.

**Regra geral:** antes de editar qualquer linha de template em `internal/generators/`, rode
`grep -rn '<a linha que você tocou>' scripts/`. O acoplamento é invisível no Go e só aparece nos ~13
minutos de `make quality`.

## Divergência de governança registrada (não é bug)

A `ADR-2026-07-31` (Decisão 3) decidiu que a seção consolidada é *"placeholder a preencher, não
agregação automática dos critérios dos MLs"*. No caminho `--from-req` os MLs **são** os ACs da REQ,
então preencher o bloco é agregação pelas palavras da ADR. A `REQ-2026-09-09` (AC7), posterior e
explícita, pede o oposto. Implementado conforme a REQ e **restrito ao `--from-req`**; o template
simples continua emitindo o placeholder. A emenda à ADR é do arquiteto.

## Resíduo conhecido

`req move` em layout **por-estado** ou **by_agent** move o arquivo da REQ e **não** atualiza o `req:`
do roadmap pareado — o inverso exato de `syncREQReferences`. Em `flat` (o layout deste projeto) a
escrita é in-place e não há defasagem, por isso o corpus local não mostra o defeito.
`ref_targets_exist` **avisa** (`roadmap "X" links to REQ "Y" which does not exist`), mas nada repara.
Mesma causa, proposto como ML novo na REQ-2026-09-09.
