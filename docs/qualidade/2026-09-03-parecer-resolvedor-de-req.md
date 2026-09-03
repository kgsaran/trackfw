# Barreira final de qualidade — resolvedor de REQ no layout canônico e ciclo fechado por artefato

> Autor: hefesto-tf (Code Quality) | Data: 2026-09-03
> Escopo: `git diff origin/main...HEAD` da branch
> `fix/resolvedor-de-req-cobre-o-layout-canonico-e-ciclo-fechado-por-artefato` — 31 arquivos,
> ML-1A (ponto único de leitura/escrita de REQ nos 3 CLIs) e ML-2A
> (`scripts/check-artifact-closed-cycle.sh` + Cenários 183/184/185 em `check-gates-falsify.sh`).
> Fronteira do papel: nenhum arquivo de produto tocado, nenhuma operação de git.

## Veredito

**APROVA COM RESSALVAS.**

O núcleo do ML-1A está **correto e verificado por execução independente**, não por leitura. O
ML-2A entrega um gate que **discrimina de verdade** — reproduzi as três sabotagens dos Cenários
183/184/185 fora do harness e as três reprovam, com o literal exato que cada cenário assevera.
Este não é mais um dos 4 gates vácuos que a auditoria de hoje mediu.

**Um único item bloqueia o merge**, e ele **não é código de produto**: `docs/cli-parity.md:385`
afirma uma cobertura que não existe — exatamente a falha que o pedido mandou procurar neste
documento. Custo do remédio: ~4 linhas de markdown, um handoff só.

O defeito de produto que essa afirmação esconde (`serve`/`api_chain` sem delegar, **vácuo** em
Node e Python no layout canônico) é **pré-existente e não é regressão deste lote** — vira REQ de
acompanhamento, não bloqueio.

---

## O que verifiquei por execução, e o que passou

Nada abaixo é aceito por declaração do roadmap.

**Ponto único de leitura — fixture `by_agent` canônico (`docs/req/apolo/REQ-….md`), 3 CLIs:**

```
go    -> 4 violações  (req_has_adr, req_has_roadmap, 2× ref_targets_exist)
node  -> 4 violações  (mesmas)
py    -> 4 violações  (mesmas)
```

**Dedup por caminho normalizado — fixture `by_agent` com REQ em `docs/req/backlog/` (o caso em que
`backlog` entra na lista de agentes vinda do disco):** cada violação aparece **1×** nos 3
runtimes, não 2×. O `seen`/`_add` faz o que o comentário promete.

**`context` no canônico:** `## REQs (1)` nos 3 CLIs. Antes deste lote os três montavam a própria
árvore e devolveriam 0.

**Gate de ciclo fechado:** `rc=0`, 36 asserções, **4,4 s** de parede.

**Cenários de falsificação — reproduzidos por mim, cada um numa árvore isolada:**

| Cenário | Sabotagem aplicada | Resultado | Literal casado |
|---|---|---|---|
| 183 | caso `<agente>/*.md` comentado em `ResolveREQFiles` (Go) + `go build` | `rc=1` | `closed cycle broken: req/go/by_agent/req_has_adr-names-generated` |
| 184 | `note new` do Node escreve `(notes/<arq>.md)` no index | `rc=1` | `closed cycle broken: note/node/flat/note_orphan-silent-for-indexed` |
| 185 | `default` do `--status` do `adr new` do Python → `Rascunho` (com `choices`) | `rc=1` | `closed cycle broken: adr/python/flat/status-literal-read-back` |

No 183, `go/flat` **continua verde** (6/6) enquanto `go/by_agent` reprova em 3 asserções — a
sabotagem é específica do layout, não uma quebra genérica do binário. É o controle negativo certo.

**Fiação:** `scripts/check-artifact-closed-cycle.sh` entra no alvo `parity` (`Makefile:30`), e
`check-gates-falsify.sh` já estava lá (`Makefile:54`); `quality: … parity` (`Makefile:82`) e o CI
roda `make parity` inteiro (`.github/workflows/quality.yml:468`). Os três cenários estão de fato
ligados.

**Código morto:** nenhum no Go — `collectTraceIdEntries`/`collectTraceIdEntriesByAgent`
continuam sendo os indexadores do lado **roadmap** (`validator_traceid.go:160,162`). O Python
**removeu** `_index_reqs`/`_index_reqs_by_agent` em vez de deixá-las órfãs. Correto.

---

## BLOQUEIA O MERGE

### B1 — `docs/cli-parity.md:385` declara um ponto único universal que não existe: `serve` reconstrói a árvore nos 3 runtimes, e a lista de resíduos declarados o omite

**Arquivo:** `docs/cli-parity.md:385-387` (afirmação) e `:425-437` (lista "Declared residuals").
**Severidade:** ALTA — cobertura declarada e inexistente, num documento que é entregável do ML-1A.

O texto diz, sem qualificação:

> **One single point decides the path, and both sides consume it** (D4). Per runtime there is
> exactly one read function and one write function; **every rule, generator and command calls them
> instead of rebuilding the tree**

`serve` é um comando, e `api_chain` reconstrói a árvore nos **três** runtimes:

- `npm/src/serve/api_chain.js:104-115` — `if (namespacing === 'by_agent')` varre **só**
  `req_dir/<agente>/<estado>/`; senão, só `req_dir/*.md`. É literalmente o `if/else` que o ML-1A
  existe para eliminar.
- `pypi/trackfw/serve/api_chain.py:144-156` — mesma estrutura.
- `internal/serve/api_chain.go:49-52` → `scanChainDir` (`:81`) com `filepath.WalkDir` — recursivo,
  **superconjunto**, nunca vácuo, mas ainda assim uma segunda noção de layout no runtime.

**Falsificado por execução** — os 3 `serve` contra o mesmo fixture `by_agent` canônico
(`docs/req/apolo/REQ-2026-09-03-fixture.md`), `GET /api/chain`, nós de `type: "req"`:

```
go    :4801 -> [{"id":"docs/req/apolo/REQ-2026-09-03-fixture.md","type":"req",…}]   1 nó
node  :4802 -> []                                                                   0 nós
py    :4803 -> []                                                                   0 nós
```

**Por que isto bloqueia, sendo "só" documentação:**

1. **É a falha exata que este repositório acabou de medir hoje** — `docs/cli-parity.md` afirmando
   cobertura que o código não tem. A instrução de escopo desta barreira é explícita: *se ele
   declarar algo, confira no código*. Conferi, e a afirmação é falsa.
2. **A seção tem uma lista de "Declared residuals"** (`:425-437`) com `traceid.js`, `sync` e
   `req move`. A ausência de `serve/api_chain` nessa lista não é omissão neutra: transforma um
   resíduo conhecido-por-ninguém em "coberto" aos olhos do próximo auditor.
3. **A distinção que importa está invertida.** O resíduo que o agente **declarou**
   (`npm/src/validator/traceid.js`) é um **superconjunto** — nunca vácuo, benigno, e ele explicou
   por que convergi-lo estreitaria o Node. O resíduo que ele **não declarou**
   (`serve/api_chain` em Node e Python) é **vácuo**. São de espécies diferentes, e a barata é a
   que ficou de fora do documento.

**Remédio (documentação, ~4 linhas):**

- Trocar "every rule, generator and command calls them" por uma enumeração honesta: as regras de
  `validate`, os geradores de REQ/roadmap, `context` e o wizard de `roadmap new`. Fecha a
  afirmação universal.
- Acrescentar à lista de `Declared residuals`:
  > **`serve` (`/api/chain`) does not go through the resolver in any runtime.** Node
  > (`npm/src/serve/api_chain.js:104`) and Python (`pypi/trackfw/serve/api_chain.py:144`) still
  > branch `flat` vs `<agent>/<state>/` and are therefore **vacuous in the canonical layout** —
  > measured: 1 `req` node in Go vs 0 in Node/Python on the same `by_agent` tree. Go
  > (`internal/serve/api_chain.go:81`, `WalkDir`) is a recursive superset. Tracked as its own REQ.

**Handoff:** `apolo-tf` (dono do documento e do ML-1A). Apenas `docs/cli-parity.md` — não é
necessário tocar código para desbloquear o merge.

---

## VIRA REQ DE ACOMPANHAMENTO (não bloqueia)

### A1 — `trackfw serve` perde todas as REQs do layout canônico no grafo da cadeia, em Node e Python

**Arquivos:** `npm/src/serve/api_chain.js:104-115`, `pypi/trackfw/serve/api_chain.py:144-156`.
**Severidade:** ALTA em impacto de usuário, **BAIXA em urgência de merge**.

Evidência: a tabela de `curl` em B1. Num projeto `by_agent`, a visão de cadeia `ADR → REQ →
ROADMAP` do board mostra **zero** REQs em 2 dos 3 runtimes.

**Por que não bloqueia este merge, e quero ser preciso nisso:** `git diff origin/main...HEAD --stat`
sobre `npm/src/serve/`, `pypi/trackfw/serve/` e `internal/serve/` é **vazio** — este lote não tocou
`serve`. E o raio de alcance para REQ **nova** não mudou: antes, `req new` em `by_agent` gravava
`req_dir/*.md`, que o ramo `by_agent` desses dois arquivos também não lê; agora grava
`req_dir/<agente>/*.md`, que ele também não lê. Invisível antes, invisível depois. É defeito
pré-existente, não regressão introduzida aqui.

**Remédio:** delegar aos pontos únicos (`resolveReqFiles` / `resolve_req_files`), derivando `state`
do nome da pasta-pai quando ela for um estado legado — exatamente o que
`internal/generators/context.go:60` e `pypi/trackfw/commands/context.py:56` já fazem neste lote. No
Go, trocar o `WalkDir` de `scanChainDir` só para a árvore de REQ, ou registrar o superconjunto.
**Handoff:** `apolo-tf`. Isto é paridade dos 3 CLIs (regra dura), então a REQ tem de listar os três.

### A2 — Uma degeneração de cobertura **não declarada** no braço de ADR do gate

**Arquivos:** `scripts/check-artifact-closed-cycle.sh:267` (asserção 2) e `:336` (linha de resumo).
**Severidade:** MÉDIA-BAIXA — honestidade de métrica, não falha de detecção.

A ressalva declarada — o eixo de layout do braço de **nota** é degenerado porque `vault/notes` é
constante do gerador — está correta e verifiquei a causa (`internal/generators/note.go:12`).

Há **uma segunda, não declarada**. A asserção `adr_orphan-names-generated` (`:267`) também é
insensível ao eixo de layout:

1. **O gerador de ADR não tem dependência de namespacing.** `grep -a "amespacing\|agent"` em
   `internal/generators/adr.go`, `npm/src/generators/adr.js` e `pypi/trackfw/generators/adr.py`
   devolve **nada** relevante — `adr new` escreve no mesmo lugar em `flat` e em `by_agent`.
2. **A asserção é monótona na direção errada em relação ao resolvedor de REQ.** `adr_orphan` acusa
   o ADR quando *nenhuma REQ o referencia*; quebrar o resolvedor deixa o ADR **mais** órfão. A nota
   de vault já registra isso ("fica MAIS forte").
3. **Nenhum dos três cenários a falsifica.** Confirmei nas minhas três execuções: 183 derruba
   `req_has_adr-names-generated`, `status-literal-read-back` e `adr_orphan-clears-after-link` (só em
   `by_agent`); 184 derruba `note_orphan-silent-for-indexed`; 185 derruba
   `status-literal-read-back`. `adr_orphan-names-generated` fica verde nas **quatro** árvores —
   são 6 asserções (3 runtimes × 2 layouts) sem falsificação demonstrada.

Ela não é inútil (pegaria uma regressão no caminho de escrita de `adr new`, e é o par da asserção 6),
mas está **contada como cobertura sem prova de discriminação**, que é precisamente o que a ressalva
do braço de nota se propôs a evitar.

**Segundo ponto, menor, no mesmo arquivo:** a linha 336 imprime `3 artefatos x 3 CLIs x 2 layouts =
18 combinacoes` para o operador. A ressalva honesta está no **cabeçalho** do script, que ninguém lê
na saída do CI. A contagem distinta é 15, não 18.

**Remédio:** uma frase no comentário de `:267` declarando a insensibilidade ao layout e ao
resolvedor, e um `(eixo de layout degenerado no braço de nota — ver cabeçalho)` na linha de resumo.
**Handoff:** `artemis-tf`.

### A3 — `pypi/trackfw/traceid.py:262` — atribuição morta e comentário que este lote tornou falso

**Arquivo:** `pypi/trackfw/traceid.py:262-264`.
**Severidade:** BAIXA.

```python
req_state = req_ids[tid]["state"]      # sempre "req"
```

Duas coisas:

1. `req_state` é atribuído e **nunca lido** — o bloco compara `status:` do frontmatter contra a
   pasta do roadmap (`:280-291`). Variável morta.
2. O comentário `# sempre "req"` **passou a ser falso neste lote**. `_index_req_files`
   (`pypi/trackfw/traceid.py:88-90`) agora deriva o estado do nome da pasta-pai quando ela é um
   estado legado, então uma REQ em `req_dir/done/` chega aqui com `state == "done"`. O comentário
   logo abaixo (`:264`, "req_dir não tem sub-pastas") também descreve um mundo que a união de
   leitura acabou de acabar.

O comportamento observável **não muda** (o valor é morto), então não é bug — é uma armadilha para
quem for consertar a divergência `traceid_state_mismatch` Go×Python que a nota de vault já
registrou. **Remédio:** apagar a linha e corrigir os dois comentários. **Handoff:** `apolo-tf`.

---

## Respostas diretas às cinco perguntas

**1. O "ponto único" é realmente único?** Nos consumidores que o ML-1A se propôs a converter,
**sim**, e verifiquei por execução (violações, dedup, `context`, os 3 runtimes). Os 11 delegantes
existem e chamam o ponto único: Go 12 call sites em `validator.go` + `validator_traceid.go:156` +
`commands/roadmap.go:56` + `generators/{context,req,roadmap}.go`; Node 12 em `validator/index.js` +
`commands/context.js:98` + `generators/{req,roadmap}.js`; Python 8 em `validator.py` +
`traceid.py:180` + `commands/context.py:56` + `generators/req.py`.

Sobra **fora** do ponto único: o resíduo **declarado** (`npm/src/validator/traceid.js`, varredura
recursiva) e o **não declarado** (`serve/api_chain`, nos 3). Julgamento pedido:

- O resíduo **declarado é aceitável como declarado**. É superconjunto, nunca vácuo, e a justificativa
  para não convergi-lo agora é boa e mensurável (convergir *estreita* o Node e obriga os testes de
  traceid a declarar `roadmap_namespacing` — isso é escopo próprio, não passada de lado). **Não
  bloqueia.**
- O resíduo **não declarado bloqueia — mas só na forma de corrigir a declaração**, porque o código
  é pré-existente e não regrediu aqui. Ver B1/A1.

**2. Os 3 seams são manuteníveis?** **Sim, sem ressalva.** O cabeçalho dos Cenários 183/184/185
(`check-gates-falsify.sh:9457-9484`) explica em prosa por que são três e não um, com o número
medido (9 de 36 sob uma sabotagem só) e com o mecanismo estrutural (`adr_orphan` fica mais forte,
`note_orphan` nunca toca `req_dir`). Um leitor futuro entende sem arqueologia. E a objeção de custo
não se sustenta: o gate roda em **4,4 s**, então as 4 execuções dentro do falsify custam ~18 s de um
alvo `parity` de ~4 min. Cada cenário também carrega o *porquê* da sua armadilha específica
(rebuild obrigatório no 183; `default`+`choices` juntos no 185), que é onde um sucessor erraria.

**3. A métrica por basename é robusta?** **Sim, e a propriedade que a salva é ser fail-closed.** Se
o formato do `validate --json` mudar (`rule`/`file`/`message` renomeados, JSON reestruturado), o
`PARSER` (`:104-147`) não acha hits e as asserções `names` — que são 4 das 6 — **reprovam alto**.
Ninguém fica com verde falso. O basename também sobrevive a `file` virar caminho completo (é
substring) e a mensagem mudar de campo (casa `file` OU `message`). Duas ressalvas honestas:

- As duas asserções `absent` (3 e 6) são fail-**open** isoladamente — sem hits, passam. Estão salvas
  pelos pares de liveness (4) e (2), que são `names` no mesmo par. O desenho é deliberado e está
  comentado; só não está dito que a proteção é o *par*, não a asserção.
- O `extra` da asserção 5, `"status: Proposed"`, é acoplado à **prosa** do validador
  (`internal/validator/validator.go:2269`, `npm/src/validator/index.js:953`: `(status: %s)`).
  Reescrever essa mensagem quebra o gate — **fail-closed**, então é custo de manutenção, não furo.

**4. Cobertura declarada honestamente?** **Quase.** A degeneração declarada (braço de nota) é real e
verifiquei a causa. Encontrei **uma não declarada** — `adr_orphan-names-generated`, detalhada em A2 —
e a linha de resumo que o operador vê no CI diz 18 sem a ressalva que o cabeçalho traz.

**5. Código morto, duplicação, nome enganoso?** Um item, baixo: A3. Fora dele, o lote **removeu**
duplicação em vez de acrescentar — `listREQFiles`, `scanREQFiles`, o ramo `by_agent` de `context` e
`_index_reqs`/`_index_reqs_by_agent` do Python saíram. `resolveREQFiles`
(`internal/validator/validator.go:1468`) ficou como alias de 3 linhas para o exportado, o que é
legítimo (consumidores in-package) e está comentado como tal.

---

## Governança

- `trackfw validate` na raiz: **exit 0**, 0 violations (warnings pré-existentes).
- `make quality`: ver linha de fecho abaixo.
- Roadmap em `wip/`; a transição para `done/` é do `trackfw_architect`, não deste papel.
- Nenhum arquivo de produto (`internal/`, `npm/src/`, `pypi/trackfw/`) modificado por mim; nenhuma
  operação de git executada.

## Separação pedida, em uma linha cada

- **Bloqueia o merge:** B1 — `docs/cli-parity.md:385` + lista de resíduos. Doc-only, ~4 linhas,
  `apolo-tf`.
- **Vira REQ:** A1 (`serve/api_chain` vácuo em Node/Python — paridade dos 3 CLIs), A2 (degeneração
  não declarada no braço de ADR do gate), A3 (linha morta e comentário falso no `traceid.py`).
- **Não encontrei defeito bloqueante no produto.** O resolvedor, a dedup, a escrita canônica, o
  `context` e os 3 CLIs estão corretos e medidos. O gate do ML-2A discrimina nas três fronteiras,
  provado por execução independente das três sabotagens.
