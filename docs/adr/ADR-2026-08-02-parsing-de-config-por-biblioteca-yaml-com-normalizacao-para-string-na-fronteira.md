---
status: Accepted
date: 2026-08-02
author: "Zeus"
---

# ADR: Parsing de config por biblioteca YAML com normalizacao para string na fronteira

> Date: 2026-08-02 | Status: Accepted

## Context

Os três CLIs leem `trackfw.yaml` com parsers **artesanais** — cerca de 1085 linhas somadas
(`internal/config/config.go` 461, `npm/src/config/index.js` 280, `pypi/trackfw/config.py` 344).

Ao longo dos ciclos de 2026-08-01/02 esses parsers produziram **quatro** defeitos, todos
silenciosos:

1. Lista em bloco **não indentada** descartada por Go e Node (YAML válido)
2. Lista **inline** descartada pelos três
3. Delimitador não pareado tratado diferente entre CLIs
4. Fallback de agentes com ordenação divergente

Cada um foi corrigido pontualmente. Mas o parser continua sendo um **subconjunto** de YAML:
listas aninhadas inline, mapas inline e âncoras seguem sem suporte **e sem aviso**. KG determinou
que não se pode seguir sabendo do defeito.

### A descoberta que redefine a decisão

Adotar bibliotecas YAML **sem mais nada** não resolve — **troca** a divergência artesanal por
divergência de schema. Medido empiricamente em 2026-08-02:

| Entrada | Go `yaml.v3` | Python `PyYAML` | Node `js-yaml` 5.2.3 | Node `yaml` 2.9.0 |
|---|---|---|---|---|
| `k: yes` | `"yes"` string | **`True` bool** | `"yes"` string | `"yes"` string |
| `k: no` / `k: on` | string | **bool** | string | string |
| `k: 010` | **`8` int (octal)** | **`8` int (octal)** | **`10` int** | **`10` int** |
| `k: 1.0` | `1` float64 | `1.0` float | `1` number | `1` number |
| `k: null` / `k: ~` | `nil` | `None` | `null` | `null` |
| `k: 2026-08-02` | **`time.Time`** | **`datetime.date`** | **`"2026-08-02"` string** | **`"2026-08-02"` string** |
| `k: true` | `true` bool | `True` bool | `true` bool | `true` bool |

PyYAML implementa **YAML 1.1** (onde `yes`/`no`/`on`/`off` são booleanos); `yaml.v3` implementa
1.2. **Go e Python divergem entre si** em `yes`.

Consequências concretas para o trackfw:

- **`lenient_until` quebra no dia 1.** Está declarado `string // date string YYYY-MM-DD`
  (`config.go:24`); as duas bibliotecas convertem para tipo data.
- **`wip_limit: 010` viraria 8**, não 10 — e o `parseInt(val, 1)` atual nunca veria o problema.
- Os booleanos hoje são `val == "true"`; sob PyYAML, `yes` já chegaria como bool.

**Node medido no ML-0A** (2026-08-02), e trouxe três achados que a tabela original não previa:

1. **O octal é divisão de três vias, não duas.** Go e Python dão `8`; **as duas libs Node dão
   `10`** — YAML 1.2 core não reconhece a notação octal legada. Ou seja, nenhum par de CLIs
   concorda por padrão nesse caso.
2. **As libs Node NÃO convertem data** — `2026-08-02` volta string. Consequência para o teste:
   um caso de `lenient_until` **passaria no Node mesmo sem normalização alguma**, porque não há
   coerção a reverter ali. **É o octal que discrimina o Node**, não a data.
3. **Âncoras são um risco real.** Em `yaml` 2.x, o nó de `b` em `a: &x 3` / `b: *x` é um `Alias`,
   não um `Scalar`: `.source` devolve o **nome da âncora**, não o valor. Normalização por
   caminhada de nós corrompe o campo se não resolver o alias antes.

Esses três reforçam a decisão: a normalização precisa ser **textual sobre o nó bruto**, não
reversão do valor tipado — porque os valores tipados divergem em três direções.

## Decision

**Carregar com biblioteca YAML e normalizar todo escalar para string na fronteira do parser.**

A decisão central **não** é "adotar biblioteca" — é a normalização. Sem ela, três bibliotecas
diferentes produzem três resultados diferentes para o mesmo arquivo.

1. **Biblioteca por CLI:**
   - Go: `gopkg.in/yaml.v3` — já está no `go.sum` como *indirect*; passa a direta.
   - Node: **`yaml` 2.x** (eemeli). Escolhida no ML-0A sobre `js-yaml` por dois motivos: expõe o
     texto original do escalar via **API pública e documentada** (`Scalar.source`), enquanto
     `js-yaml` exige combinar `parseEvents` + `eventsToAst` — não documentado e frágil; e tem
     **zero dependências de runtime**, enquanto `js-yaml` arrasta `argparse`. Em schema as duas
     empatam.
   - Python: `PyYAML` — **primeira dependência de runtime** do pacote, que hoje é zero-dep.
2. **Normalização imediata:** após o `load`, a árvore é convertida para strings —
   escalares viram sua representação textual; listas viram listas de string; `null` vira string
   vazia ou ausência, conforme o campo. **Datas e números não chegam tipados aos consumidores.**
3. **Os consumidores não mudam.** `parseInt(val, 1)`, `val == "true"` e afins continuam fazendo a
   tipagem que já fazem. O contrato de `ProjectConfig` e equivalentes é preservado.
4. **Formato de data e número preservados textualmente:** `2026-08-02` volta como
   `"2026-08-02"`, não como `2026-08-02 00:00:00 +0000 UTC`. `010` volta como `"010"`, não `"8"`.
   Este é o ponto mais delicado da implementação.

### Escopo

Apenas `config`. O parser de **frontmatter** (`parse_frontmatter` /
`extractFrontmatterField`) é **separado** e fica fora — converter também triplicaria o raio e
aplicaria coerção de data em **todo** campo `date:` de **todo** ADR e REQ, risco muito maior que
o de config.

## Consequences

**Positivas**

- Elimina a classe de defeito de vez: qualquer YAML válido passa a ser aceito no config, incluindo
  as formas que os quatro ciclos anteriores não cobriram.
- Remove ~1085 linhas de parser próprio, com quatro defeitos conhecidos em dois meses.
- A normalização faz as três bibliotecas concordarem **por construção**, não por coincidência.

**Negativas / aceitas**

- **O pacote Python perde o zero-dependency.** É vantagem real de distribuição, e KG aceitou o
  custo conscientemente.
- Node ganha a terceira dependência de runtime.
- A camada de normalização é código novo — menor que os parsers que substitui, mas é onde mora o
  risco. Se ela divergir entre CLIs, o defeito volta com outra cara.
- Reescrita ampla imediatamente antes de uma tag. Mitigado por: executor único, gates de paridade
  existentes, e fixture de falsificação com os casos que discriminam schema.

## Alternatives Considered

**Adotar biblioteca sem normalizar** — o caminho óbvio. **Rejeitado pela medição:** Go e Python
divergem em `yes`, e ambos convertem data e octal, quebrando `lenient_until` e `wip_limit`.
Seria trocar divergência conhecida por divergência nova.

**Manter o parser artesanal e falhar alto no não suportado** — zero dependência nova, mudança
pequena. **Rejeitado por KG:** acaba com o silêncio, mas YAML válido continua não funcionando.

**Normalizar configurando o schema de cada biblioteca** (ex.: PyYAML com loader restrito a 1.2)
em vez de converter depois — mais elegante em teoria. **Rejeitado:** exige domínio fino de três
APIs de schema diferentes, e a equivalência entre elas seria difícil de provar. Converter a árvore
depois do load é grosseiro, mas **verificável** — e verificabilidade é o que este projeto tem
valorizado.

**Migrar também o frontmatter** — unificaria os dois parsers. **Rejeitado:** triplica o raio e
aplica coerção de data em todos os artefatos. Candidato a ADR próprio depois que o config estiver
estável.
