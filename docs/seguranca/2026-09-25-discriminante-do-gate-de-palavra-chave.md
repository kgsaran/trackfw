# Discriminante do gate de palavra-chave de fechamento — ML-0B (Wave 0 ampliada)

> **Autor:** Hades (revisor de segurança) · **Data:** 2026-09-24 (arquivo nomeado `2026-09-25` pelo ML)
> **Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md`
> **REQ:** `docs/req/REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md`
> **Issue:** `#258` · **PRs relevantes:** `#416` (mergeado hoje), `#247`, `#293`, `#312`, `#325`, `#330`, `#417`, `#424`
> **Escopo:** medição. **Nenhuma linha de implementação de produto.** Único arquivo de produto lido: `scripts/check-pr-closing-keyword.sh`.

---

## 0. Instrumentos e corpora usados

| corpus | o que é | tamanho |
|---|---|---|
| **A — real** | corpo **vivo** dos PRs mergeados deste repositório, via `gh pr list --state merged --limit 500 --json number,body` | **358 PRs, 357 com corpo** |
| **B — sintético** | frases por **forma**, incluindo as **frases reais** de #417, #424, #293 e #258 (não paráfrases — AC8) | 41 casos |
| **verdade-terreno** | `gh pr view <n> --json closingIssuesReferences` + `gh issue view <i> --json state,closedAt` | 4 PRs, 6 issues |

O gate foi invocado **pelo caminho real** (`PR_BODY_FILE=<arquivo> bash scripts/check-pr-closing-keyword.sh`),
um corpo por invocação, com `$?` lido **sem cano antes** (regra operacional desta casa). Protótipos de
discriminante candidato viveram **só no scratchpad** e estão reproduzidos aqui como texto, não como patch.

⚠️ **Viés declarado do corpus A:** ele guarda o corpo **vivo**, não o de abertura. Nas duas ocorrências
da forma 4 citadas pela REQ (#417 e #424) o remédio aplicado foi **reescrever a frase**
(`"A #421 permanece aberta"`, `"a #363 permanece aberta"`), logo **as frases que dispararam o gate não
estão mais no corpus**. O corpus A **subestima a forma 4** por construção. As frases originais entraram
no corpus B a partir da REQ e do issue.

---

## 1. O discriminante de HOJE, em uma frase

> **O gate acusa quando um verbo de fechamento em português aparece *lexicalmente adjacente* a `#N`
> — separado no máximo por ênfase markdown colada, artigo definido e/ou a palavra "issue" — e o
> mesmo `N` não tem palavra-chave inglesa em nenhum lugar do corpo; ele nunca pergunta se a frase
> *afirma* fechamento.**

A segunda metade da frase é o achado. A adjacência e a isenção por número são **escolhas conscientes e
documentadas** no cabeçalho do script. A **ausência de qualquer noção de asserção** — polaridade,
citação, autoria da frase — **não está declarada em lugar nenhum**, e é a causa única das quatro formas.
Por isso a Regra Dura de Causa Raiz está correta em mantê-las na mesma REQ: não são quatro defeitos,
é **um** discriminante medindo a coisa errada, errando em quatro direções.

---

## 2. Enumeração PELA FORMA — e a busca ativa pelo que não estava na lista

### 2.1 Comando

```bash
# corpus B, forma por forma, gate real, sem cano antes do $?
printf '%s' "$corpo" > "$D/b.md"
PR_BODY_FILE="$D/b.md" bash scripts/check-pr-closing-keyword.sh > "$D/out" 2>&1; rc=$?
```

```bash
# corpus A, 357 corpos, mesmo caminho
gh pr list --state merged --limit 500 --json number,body   # -> um arquivo por PR
for f in bodies/*.md; do PR_BODY_FILE=$f bash scripts/check-pr-closing-keyword.sh; echo $?; done
```

### 2.2 Baseline medido no corpus A (357 corpos reais)

```
355 rc=0     2 rc=1     1 rc=2 (PR #49, corpo vazio — not_evaluated, correto)
```

As **3 linhas acusadas** hoje, em 2 PRs:

| PR | linha | é defeito? |
|---|---|---|
| **#247** | `Fecha #246.` | ✅ **verdadeiro positivo** — `closingIssuesReferences: []`, #246 fechada à mão |
| **#293** | `**Não fecha #290** (usage / …)` | ❌ **falso positivo** — forma 4 |
| **#293** | `**Não fecha #275** (ratchet por nome …)` | ❌ **falso positivo** — forma 4 |

🔴 **Precisão de hoje sobre o corpus inteiro do repositório: 1 acerto em 3 acusações.** Dois terços do que
o gate acusa é a forma 4. Isto não é uma anedota de duas ocorrências: **é a maioria do output do gate.**

E os **verdadeiros positivos que ele não vê** (medidos abaixo, todos com `closingIssuesReferences` sem o
número em questão, e todas as issues fechadas **à mão** depois do merge):

| PR | linha | issues que ficaram abertas no merge |
|---|---|---|
| **#312** | `Fecha **#274** e **#275**.` | #274, #275 |
| **#325** | `Fecha **#314** e **#319** — ambos sobre o …` | #314, #319 |
| **#330** | `Fechar o **#320** (…)` | #320 (o PR fechou a #315 por forma inglesa — a isenção por número está certa) |

🔴 **Cobertura de hoje: 1 de 4 declarações de fechamento em português do corpus real.** O gate vê 25% do
que afirma guardar, e 67% do que ele diz está errado.

### 2.3 Tabela por forma (corpus B — gate real)

Legenda: **esp** = o que o gate *deveria* fazer; **hoje** = o que ele faz. `1` = acusa, `0` = passa.

#### Forma 3 — markdown/pontuação entre a palavra e o `#N` (falso NEGATIVO)

| frase | esp | hoje |
|---|---|---|
| `Fecha **#274** e **#275**.` (PR #312, real) | 1 | **0** ❌ |
| `Fecha *#274*.` | 1 | **0** ❌ |
| `Fecha o **#274**.` | 1 | **0** ❌ |
| `Fecha: #274.` | 1 | **0** ❌ |
| `Fecha, #274.` | 1 | **0** ❌ |
| `Fecha -- #274.` | 1 | **0** ❌ |
| `Fecha (#274).` | 1 | **0** ❌ |
| `Fecha [#274](https://github.com/…/issues/274).` | 1 | **0** ❌ |
| `Fecha\n#274.` (quebra de linha) | 1 | **0** ❌ |
| `Fecha a **issue** **#274**.` | 1 | **0** ❌ |

**11 de 11 escapam.** A forma não é "negrito": é **qualquer caractere não-palavra** entre o verbo e o `#N`.
O `PT_FILLER` de hoje só admite `*{0,2}` **colado** à palavra-chave, `[ \t]*`, artigo e `issue`.

#### Forma 4 — negação lida como afirmação (falso POSITIVO, dano dirigido)

| frase | esp | hoje |
|---|---|---|
| `Este PR **não** fecha a #363; ela continua aberta.` (#417, real) | 0 | **1** ❌ |
| `**Não fecha a #421** — o cluster cai de 11 para 2.` (#424, real) | 0 | **1** ❌ |
| `**Não fecha #290** (usage / …)` (#293, real, **mergeado**) | 0 | **1** ❌ |
| `Não corrige o #239.` | 0 | **1** ❌ |
| `Não encerra a issue #12.` | 0 | **1** ❌ |
| `Nem fecha a #12 nem fecha a #13.` | 0 | **1** ❌ |
| `Não é verdade que fecha #12.` (negação distante) | 0 | **1** ❌ |
| `Este roadmap nunca fecha a #12 sozinho.` | 0 | **1** ❌ |
| `Entrega sem fechar a #12.` | 0 | **1** ❌ |
| `Deixa de fechar a #12 nesta entrega.` | 0 | **1** ❌ |
| `This PR does not close #12.` | 0 | 0 ✅ (por acidente: o gate nunca acusa inglês) |

**10 de 11 falham.** A negação **distante** (`Não é verdade que fecha #12`) falha pelo mesmo mecanismo que
a colada: **não existe leitura de polaridade nenhuma**, perto ou longe.

#### Forma 2 — exemplo citado (falso POSITIVO) — **maior do que a REQ descreve**

| frase | esp | hoje |
|---|---|---|
| `A forma errada e \`Fecha #246\`.` (code span) | 0 | 0 ✅ |
| cerca ``` … `Fecha #246.` … ``` | 0 | 0 ✅ |
| `O corpo da minha PR #247 dizia "Fecha #246" …` (**#258, a frase original**) | 0 | **1** ❌ |
| `O corpo dizia “Fecha #246” …` (aspas curvas) | 0 | **1** ❌ |
| `> Fecha #246` (blockquote) | 0 | **1** ❌ |
| `\| Fecha #246 \| nao fecha \|` (célula de tabela) | 0 | **1** ❌ |
| `    Fecha #246.` (**bloco de código indentado por 4 espaços**) | 0 | **1** ❌ |

🔴 **A forma 2 não está resolvida por cerca+span.** O bloco indentado por 4 espaços é **bloco de código
Markdown legítimo** e o GitHub o ignora para fechamento. ⚠️ **Precisão sobre a evidência, exigida pela
reconciliação:** a sonda **#431** do `ML-0C` mediu a keyword **inglesa** dentro de bloco indentado (`[]`, §10)
— ela é **consistente com** esta linha, mas **não a discrimina**, porque uma keyword **portuguesa** não fecha
issue em zona nenhuma. O `esp` = 0 desta linha se sustenta no argumento **semântico** de §3.3 (é **citação**,
não declaração), não na medição de §10. O gate o lê como prosa. A zona não-assertiva de hoje implementa **metade da gramática** que ela
afirma cobrir. A frase canônica do #258 **continua sendo acusada hoje**.

#### Forma 1 — não reavalia: 🔴 **conferida, e o #416 está INERTE no CI**

O PR **#416** (mergeado 2026-09-24T10:33Z) passou a preferir o **corpo vivo pela API** ao payload imutável.
A leitura do código confirma a mudança. **A medição no CI a desmente.**

`.github/workflows/quality.yml` continua com `on: pull_request` **sem `types:`** (default
`opened, synchronize, reopened` — `edited` fora), e o job `pr-closing-keyword` **não define `GH_TOKEN`**.
Os outros workflows que usam `gh` definem (`check-annotations.yml:39`, `release.yml:419`).

Log real do run mais recente do gate — `36074650903`, PR #425, `2026-09-24T23:49Z`, **depois** do merge do #416:

```
fonte: payload do evento (corpo de ABERTURA do PR #425).
`gh` indisponivel ou sem permissao: se o corpo foi editado depois, reexecutar
este job NAO muda o veredito -- feche e reabra o PR, ou rode com `--pr 425`.
```

```
env:
  FALSIFY_SHARD_COUNT: 4          ← o step NÃO recebe GH_TOKEN
```

> 🔴 **Veredito sobre a forma 1: não corrigida no CI.** O `fallback` para payload é o caminho **único** em
> produção; o caminho da API **nunca executa**. A tabela de medição do #416 (`payload antigo + número de PR
> que existe → rc=0`) foi produzida **localmente, com `gh` autenticado** — ambiente que o CI não tem.
> Isto é exatamente a classe da **Regra Dura de Reconciliação**: o artefato entregue afirma uma coisa que
> a medição no ambiente real nega. O gate ainda **diz no log** que está lendo o corpo de abertura, e isso
> é o que salva o achado de ser silencioso — a honestidade do log é o que permitiu medi-lo.
>
> **Correção mínima (não implementada aqui):** `GH_TOKEN: ${{ github.token }}` no step, `pull-requests: read`
> nas permissões, e `types: [opened, synchronize, reopened, edited]` — este último é o AC1/AC2, e **AC2 exige
> workflow dedicado**, porque `edited` dispara também em mudança de título e rerodaria o `Quality` inteiro.

#### 🔴 A forma 1 está MASCARANDO a forma 4 — medido

O `pr-closing-keyword` de **#293** foi **`SUCCESS`** e o PR mergeou em 2026-09-08. Mas:

```
gate DA ÉPOCA de #293  (git cat-file -p 9bc26ef6^1:scripts/check-pr-closing-keyword.sh)
   contra o corpo VIVO de #293  →  rc=1
   linha 98:  **Não fecha #290** …
   linha 101: **Não fecha #275** …
```

O matcher **daquele commit** acusa. O check ficou verde. A única explicação restante é que as duas linhas
entraram por **edição posterior ao último evento com payload** — ou seja, **a forma 1 é o que impede a forma
4 de reprovar hoje**.

🔴 **Consequência de desenho, e é a mais importante deste parecer:** corrigir a forma 1 (AC1) **sozinha**
transforma um falso positivo latente em falso positivo **ativo em todo PR que declara escopo negativo**.
Hoje o dano da forma 4 é limitado pelo defeito da forma 1. **AC1 não pode entrar antes da forma 4 fechar.**

### 2.4 🔴 Busca ativa — formas que NÃO estavam na lista das 4

A lista das 4 é **piso**. Busquei além dela e achei **três famílias novas**, todas falso negativo, todas da
mesma causa (o discriminante é uma **lista fechada de tokens**, não uma forma):

**(5) Conjugação fora da lista** — `PT_KEYWORDS` enumera sufixos à mão:

| frase | hoje |
|---|---|
| `Fecho a #12 com este PR.` (1ª pessoa) | **0** ❌ |
| `Fechará a #12 no merge.` (futuro) | **0** ❌ |
| `Fechando a #12.` (gerúndio) | **0** ❌ |
| `Vai fechar a #12.` (perifrástico) | 1 ✅ (só porque `fechar` está na lista) |

**(6) Gramática de referência assimétrica entre PT e EN.** `EN_REF` aceita `owner/repo#N` e URL completa;
o lado português aceita **só `#(\d+)`**:

| frase | hoje |
|---|---|
| `Fecha kgsaran/trackfw#12` | **0** ❌ |
| `Fecha https://github.com/kgsaran/trackfw/issues/12` | **0** ❌ |

🔴 Esta é uma **assimetria de segurança**, não uma lacuna de conveniência: o gate reconhece a referência
cruzada quando ela **isenta** e não a reconhece quando ela **acusa**. O lado permissivo é mais expressivo
que o lado restritivo — o padrão clássico de fail-open.

**(7) Verbos de fechamento fora das 4 famílias** — `Sana o #12`, `Soluciona o #12`, `Conserta o #12`,
`Elimina o #12`, `Fixa o #12`: todos **passam**. Classifico como **residual aceito**, não defeito: a lista
de 4 famílias é declarada no cabeçalho, e ampliá-la sem medir custo de falso positivo é o caminho do gate
ruidoso. Mas fica **escrito** que a lista é fechada por escolha, não por completude.

**(8) ❌ RETIRADA da lista de defeitos pelo `ML-0C` — "a zona de código apaga a isenção inglesa" é
COMPORTAMENTO CORRETO, e o erro era meu.** Eu havia classificado isto como falso positivo do gate de hoje.
Medido em 2026-09-24 (§10): **está certo.**

`blank_code` é aplicado **uma vez** e o resultado (`scan`) alimenta `EN_RE.findall` **e** o laço português.
As duas frases, com a coluna `esp` **corrigida pela medição do `ML-0C`**:

| frase | esp (corrigido) | hoje | fonte |
|---|---|---|---|
| `Fecha #246.` + ``` ```\nCloses #246\n``` ``` | **1** | **1** ✅ | PR sonda **#428** → `closingIssuesReferences = []` |
| `Fecha #246. A forma certa seria \`Closes #246\`.` | **1** | **1** ✅ | PR sonda **#429** → `closingIssuesReferences = []` |

A segunda frase é o registro do #258 — alguém explicando qual seria a linha certa. **E o gate está certo em
acusá-la**: aquele corpo, exatamente como está, **não fecha a #246 no merge**. O aviso é desconfortável, não
falso. O que o #258 tem de legítimo é a **forma 2** (a frase entre aspas, §3.3), não esta.

✅ **A premissa deixou de ser premissa.** A afirmação *"o GitHub ignora palavra-chave de fechamento dentro
de bloco de código"* — que o cabeçalho do gate usa para justificar o `blank_code` — foi **medida
empiricamente** no `ML-0C` com 7 PRs sonda contra 2 issues descartáveis deste repositório. Resultado, em
§10: **cerca e code span são ignorados pelo GitHub** (`[]` nos dois, contra `[426]` no braço de controle em
prosa), **e as duas zonas não divergem entre si**. O corpus A não decidia a questão (zero PRs com a única
keyword inglesa dentro de zona de código); a sonda decidiu. A simetria do `blank_code` de hoje **tem
respaldo medido** para as zonas de código.

🔴 **Consequência para o `ML-N1`, passo 1:** ver a diretriz única e sem ambiguidade em **§10.4**.

**Formas que busquei e o gate acerta** (registrado para não ser reenumerado):
`FECHA #12` (caixa alta) ✅ · `Fecha#12` (sem espaço) ✅ · `Fecha a issue 12` (sem `#`) → passa, **correto**
(`#` é o que mantém o falso positivo em zero — ver §3.4).

**Total enumerado: 8 famílias, não 4** — e, depois do `ML-0C`, **7 defeitos + 1 comportamento correto**: a
forma 8 caiu por medição (§10). O número de famílias que o `ML-N1`/`ML-N2` tem de fechar é **7**.

---

## 3. 🔴 A TENSÃO entre as correções — a tarefa central

### 3.1 Veredito

> **As quatro caem com um discriminante só. Medido, não afirmado.** Mas o discriminante único **troca o
> modo de falha**: sai de *"acusa demais e vê de menos"* e entra em *"silenciável por uma palavra"*.
> A fronteira nova está medida em §3.5 e é o que o threat model tem de aceitar explicitamente.

### 3.2 O candidato medido (existência, **não** o patch)

Protótipo de scratchpad, reproduzido aqui como **prova de existência**. 🔴 **Não colar no gate** — a
superfície de silenciamento de §3.5 embarcaria sem exame.

1. **Gramática de referência simétrica** entre PT e EN: `(?:owner/repo)?#N` **ou** URL completa de issue.
   🔴 **O `#` continua obrigatório no lado português.**
2. **Lacuna entre verbo e referência aberta a caracteres não-palavra** (`* _ ~ : , - — – [` e espaço),
   **nunca a palavras** — o que preserva `Fecha o **item 4** da issue #216` como prosa.
3. **Conjugação ampliada** (`fecho`, `fechará`, `fechando`, …) dentro das 4 famílias declaradas.
4. **Zona não-assertiva ampliada**: cerca, code span, **bloco indentado por 4 espaços/tab**, blockquote,
   linha de tabela — e span entre aspas (retas e curvas). Ver a ressalva de §3.3.
5. **Polaridade**: negação (`não · nem · nunca · jamais · sem · deixa de · em vez de · ao invés de`) à
   esquerda do verbo **dentro da mesma cláusula** — quebradores de cláusula: `. ; : ! ?` e fim de linha.
6. 🔴 **Dois passes de mascaramento, não um — mas SÓ para as zonas NÃO-código.** ⚠️ **Corrigido pelo
   `ML-0C`:** a versão original deste item dizia "a zona não-assertiva" sem qualificar, e isso está **errado
   para cerca, span e bloco indentado**. O que foi medido (§10): o GitHub **ignora** a keyword nas zonas de
   **código** (cerca, span, indentado) e **honra** nas zonas **não-código** (blockquote, célula de tabela,
   aspas). Logo:
   - **zonas de código** → máscara **única**, subtraída dos **dois** matchers (é o `blank_code` de hoje, e
     está correto — a forma 8 não é defeito);
   - **zonas não-código** → máscara em **passe próprio**, subtraída **só** do matcher português.
   Aplicar o passe duplo às zonas de código instalaria **falso negativo**: o gate calaria sobre um corpo que
   o GitHub **não** vai fechar. Ver a diretriz de execução em **§10.4**.

### 3.3 🔴 Ressalva contra o AC3: a frase do #258 é salva **só pelas aspas**

O AC3 diz literalmente: *"Contrato próprio para 'exemplo citado', não heurística de aspas… não deve ser
inferido de aspas apenas"*, e nomeia o vocabulário permitido como *"bloco de código, cerca, seção declarada"*.

Medido, com e sem a zona de aspas:

| frase | com zona de aspas | sem |
|---|---|---|
| `O corpo da minha PR #247 dizia "Fecha #246" …` (**#258**) | 0 ✅ | **1** ❌ |
| `O corpo dizia “Fecha #246” …` | 0 ✅ | **1** ❌ |
| `O corpo da minha PR dizia Fecha #246 …` (sem aspas nenhuma) | **1** | **1** |

🔴 **A frase canônica da forma 2 passa exclusivamente pela heurística que o AC3 proíbe.** A terceira linha
mostra por quê: retiradas as aspas, **não sobra sinal nenhum** naquela frase — nem cerca, nem indentação,
nem seção declarada. O `#258` já havia dito isso, com outras palavras: as duas saídas que ele ofereceu eram
*"ignorar span de código/aspas"* **ou** *"documentar `#NNN` no template"*, e o autor contornou pela segunda.

**Escolha que o arquiteto tem de fazer antes do ML de implementação — não é minha:**

- **(a)** ampliar o vocabulário do AC3 para incluir **span entre aspas** como zona não-assertiva, com o
  argumento medido: aspas delimitam **citação**, que é a mesma categoria semântica de "exemplo citado", e o
  custo no corpus A é **zero** (§3.4). A objeção do AC3 — *"não deve ser inferido de aspas apenas"* — é
  contra aspas como **único** mecanismo; aqui elas são o quinto de cinco.
- **(b)** manter o AC3 estrito e **declarar a frase do #258 como residual**, com o remédio documentado no
  `PULL_REQUEST_TEMPLATE.md` (`#NNN` ou crase). Custo: a forma 2 fecha **para o caso do gate que fala de si**
  e **não fecha para o caso que abriu o issue**.

Escolher (b) em silêncio deixaria o AC3 marcado `[x]` com a frase original do issue ainda sendo acusada.

### 3.4 Falsificação nas duas direções do candidato

**Corpus A — 357 corpos reais:**

```
hoje (C0)     : 3 linhas acusadas  → 1 verdadeiro positivo, 2 falsos positivos
candidato (C5): 4 linhas acusadas  → 4 verdadeiros positivos, 0 falsos positivos
  #247 L1   Fecha #246.
  #312 L9   Fecha **#274** e **#275**.
  #325 L1   Fecha **#314** e **#319** — …
  #330 L3   Fechar o **#320** (…)
```

Os 4 confirmados por `closingIssuesReferences` vazio (ou sem o número) e por `closedAt` posterior ao merge.
**Cobertura 1/4 → 4/4; precisão 1/3 → 4/4. Falso positivo permanece zero sobre o corpus inteiro.**

**Corpus B — 40 casos por forma: 40/40**, incluindo as frases reais de #417, #424, #293 e #258, e
incluindo os **contra-braços** que não podem regredir: `Fecha #123` acusa · `Corrige o #239` acusa ·
`Fecha #246` + `Fixes #999` acusa (isenção é por número) · `Fecha #246` + `Fixes #246` passa ·
`Fecha o **item 4** da issue #216` passa · `Corrige os tres defeitos do #232` passa ·
`Fecha a governanca do PR #145` passa · `Fecha 2 dos 3 elos (#148, #149)` passa.

**Por que o `#` obrigatório no lado português é load-bearing.** Medi a variante que espelha o `EN_REF`
literalmente (aceitando número nu): sobe de 4 para **10 acusações** no corpus A, com **6 falsos positivos**
inéditos — `Corrige os 2 achados registrados como REQ…`, `Fecha 2 dos 3 elos`, `fecha ~50 vermelhos`,
`corrigido **1 sítio de 20**`. **A simetria PT↔EN é desejável na referência com `#`, e proibida no número nu.**

**Por que parêntese e colchete ficam fora dos delimitadores.** Com `(` e `[` no conjunto, aparece 1 falso
positivo inédito no corpus A: `#337 L11 — "issues já fechados** (#335, #336)"`. Com colchete só (para
`Fecha [#N](url)`), o falso positivo não reaparece. **`Fecha (#274).` fica como residual declarado (§5).**

### 3.5 🔴 A tensão real, com os casos que a provam

**(i) A negação é uma regra de SUPRESSÃO — e supressão é a superfície de ataque.** Medido contra o
candidato, esperado **acusar**:

| frase | candidato |
|---|---|
| `Sem dúvida, fecha a #12.` | **silenciada** ❌ |
| `Não só fecha a #12, mas também corrige o #13.` | **silenciada** ❌ |
| `Não apenas fecha a #12 — também encerra a #13.` | **silenciada** ❌ |
| `Não fecha a #12, mas fecha a #13.` (polaridade mista) | **silenciada** ❌ |
| `Este PR, que não é refactor, fecha a #12.` (aposto encaixado) | **silenciada** ❌ |
| `Nem só fecha a #12.` | **silenciada** ❌ |
| `Sem contar o #99, fecha a #12.` | **silenciada** ❌ |
| `Sem mais delongas: fecha a #12.` | acusa ✅ (o `:` quebra a cláusula) |
| `Isto não é refactor. Fecha #421.` | acusa ✅ |
| `Fecha a #12, não a #13.` (negação à direita) | acusa ✅ |

🔴 **6 de 10 silenciam.** `Sem dúvida` é o pior: é um **intensificador afirmativo** que contém um token de
negação. `Não só … mas também` é afirmativo por construção. Nenhuma dessas é evasão adversária — são
português normal, e o gate hoje as acusaria **corretamente**.

🔴 **E a janela `{0,40}` é constante afinada sem base principiada** — do mesmo formato que a lista de
exceções por offset que esta casa já pagou. Tem de ser declarada como o que é: um ponto de calibração
que **falseia nos dois lados** (longa demais silencia por acidente; curta demais deixa a negação distante
passar). `Não é verdade que fecha #12` tem 21 caracteres de janela; `Sem contar o #99, fecha a #12` tem 18.
**As duas cabem na mesma janela e querem vereditos opostos.** Nenhum ajuste de `{0,N}` separa as duas.

**(ii) Ordenação obrigatória: forma 3 antes da forma 4 cria falso positivo que hoje não existe.** Uma
string prova, melhor que qualquer contagem de corpus:

| frase | hoje | **só a forma 3 corrigida** | candidato completo |
|---|---|---|---|
| `Não fecha **#421**.` | 0 | **1** 🔴 **FP novo** | 0 ✅ |
| `Não fecha: #421.` | 0 | **1** 🔴 **FP novo** | 0 ✅ |
| `**Não fecha **#421**.**` | 0 | **1** 🔴 **FP novo** | 0 ✅ |
| `Não fecha #421.` | 1 | 1 | 0 ✅ |

A adjacência estreita de hoje é o que **esconde** parte da forma 4. Alargá-la primeiro **desenterra**
falsos positivos. No corpus A isso não apareceu — as 3 linhas novas de C1 eram todas verdadeiro positivo —
e é justamente por isso que a contagem de corpus **não** é suficiente como prova de ordenação: a frase real
do #424 (`**Não fecha a #421**`) **já não está no corpus**, porque foi reescrita.

**(iii) O mesmo vale para AC1** (§2.3, "a forma 1 está mascarando a forma 4"): reavaliar em `edited`
ativa o falso positivo latente. **Três correções travadas em ordem:** forma 4 e forma 3 no **mesmo commit**,
e AC1 **depois** delas.

**(iv) 🔴 Ampliar a zona não-assertiva num passe único ATACA a isenção inglesa — segundo custo nomeado,
e o `ML-0C` corrigiu 2 das 5 linhas desta tabela.** A versão do `ML-0B` assumia que *"o GitHub fecha a #246
nos cinco casos"*. **Isso era premissa, e 2 dos 5 estavam errados.** A tabela abaixo traz a coluna de
verdade-terreno medida (§10):

| frase | candidato | GitHub fecha? (medido) | veredito |
|---|---|---|---|
| `Fecha #246. A linha certa é "Closes #246".` | **1** | **sim** — sonda **#434** `[430]` | ❌ falso positivo do candidato — **confirmado** |
| `Fecha #246.` + `> Closes #246` | **1** | **sim** — sonda **#432** `[430]` | ❌ falso positivo do candidato — **confirmado** |
| `Fecha #246.` + linha de tabela com `Closes #246` | **1** | **sim** — sonda **#433** `[430]` | ❌ falso positivo do candidato — **confirmado** |
| `Fecha #246.` + bloco indentado com `Closes #246` | **1** | **NÃO** — sonda **#431** `[]` | ✅ **acusar é CORRETO** — linha refutada |
| `Fecha #246.` + cerca com `Closes #246` | **1** | **NÃO** — sonda **#428** `[]` | ✅ **acusar é CORRETO** — linha refutada (era a "forma 8") |

**3 de 5, não 5 de 5.** Nos três primeiros o corpo **fecha de verdade** e o candidato acusaria — falso
positivo real. Nos dois últimos o corpo **não fecha**, e acusar é o trabalho do gate. 🔴 **Isto não apareceu
nos 357 corpos** — é falso positivo do **candidato**, não do corpus, e por isso o "0 falso positivo" de §3.4
é propriedade do corpus, não do discriminante. **Correção de desenho obrigatória, agora com a fronteira
medida:** dois passes **apenas nas zonas não-código** (aspas, blockquote, tabela); nas zonas de **código**
(cerca, span, indentado) a máscara continua **única**, subtraída dos dois matchers. Ver **§10.4**.

**Censo do que cada zona nova apaga, no corpus A (o que valida quais zonas são seguras):**

| zona | linhas apagadas | com verbo PT + `#N` | com verbo EN + `#N` |
|---|---|---|---|
| `INDENT` (4 espaços/tab) | 80 | **0** | **0** |
| `BQ` (blockquote) | 62 | 1 (`> **Empilhada sobre a [#238](…/pull/238)**` — PR, não issue) | **0** |
| `TBL` (linha de tabela) | 1354 | **0** | **8** (`\| #86 \| \`fix\` \| …` — coluna de *tipo de commit*, não palavra-chave) |
| `QUOTE` (aspas) | 831 | **0** | **0** |

Duas leituras: **(a)** `INDENT` não é a máscara larga que se temia neste corpus — 80 linhas, nenhuma com
referência, então o bloco indentado de §2.3 pode fechar; **(b)** `TBL` apaga 1354 linhas e 8 delas têm
`fix`+`#N` — todas em tabelas de changelog onde `fix` é **tipo de commit**, não verbo. Apagar essas é
inofensivo hoje, **mas é exatamente o mecanismo do item (iv)**: uma tabela com a forma certa de fechamento
perde a isenção. Reforça a exigência de dois passes — ✅ e o `ML-0C` **mediu** que essa tabela fecha de
verdade (sonda **#433** = `[430]`), então o passe duplo é obrigatório **nesta** zona. 🔴 **Já a leitura (a)
muda de balde:** `INDENT` é zona de **código** e o GitHub **não** fecha dentro dela (sonda **#431** = `[]`),
logo a máscara de `INDENT` vai para o **passe único** (os dois matchers), junto com cerca e span — nunca
para o passe que preserva a isenção inglesa (§10.4).

### 3.6 Veredito de compatibilidade, forma a forma

| par | compatível? | evidência |
|---|---|---|
| 3 × 4 | ✅ **sim**, no mesmo commit | 40/40 em B; e §3.5(ii) prova que **separados** não são |
| 3 × 2 | ✅ sim | alargar só para **não-palavra** preserva `Fecha o **item 4** da issue #216` (medido) |
| 4 × 2 | ✅ sim — **mecanismos disjuntos** | polaridade lê **à esquerda do verbo**; citação lê **zona do texto**. Nenhuma frase do corpus B aciona as duas |
| 3 × isenção por número | ✅ sim | `Fecha #246` + `Fixes #999` continua acusando (medido) |
| 1 × 4 | 🔴 **incompatível se 1 vier primeiro** | §2.3: hoje 1 mascara 4 |
| 3 × 4, separados | 🔴 **incompatível** | §3.5(ii): `Não fecha **#421**.` |
| simetria PT↔EN de referência | ⚠️ **parcial** | com `#`: sim. Número nu: **proibido**, +6 FP medidos |
| 2 × isenção por número | 🔴 **incompatível em passe único — só nas zonas NÃO-código** | §3.5(iv): **3 de 5** após o `ML-0C`. Exige dois passes para aspas/blockquote/tabela; cerca/span/indentado ficam em passe único (§10.4) |

**Nenhum remendo por forma. Um discriminante, um commit, e a ordem importa.**

### 3.7 Os dois custos nomeados do discriminante único

1. **Superfície de silenciamento da polaridade** (§3.5-i) — 6 de 10 frases afirmativas com token de negação
   silenciam. **Não tem solução dentro de regex.**
2. **Mascaramento de passe único destrói a isenção inglesa nas zonas não-código** (§3.5-iv) — **3 de 5**
   após a medição do `ML-0C`. **Tem solução exata:** dois passes, **restritos a aspas/blockquote/tabela**.
   É dívida de desenho, não fronteira.

A diferença entre os dois importa: o segundo se fecha; o primeiro só se **declara e se torna visível**.

---

## 4. Threat model — o gate como controle de PROCESSO

### 4.1 Qual garantia ele oferece

Ele **não** oferece garantia de segurança. Oferece uma garantia de **integridade de declaração**:

> *"Se o corpo deste PR declara fechar uma issue, o mecanismo do GitHub vai de fato fechá-la."*

O ativo protegido é a **confiabilidade do rastro de governança** — a cadeia `ADR → REQ → ROADMAP → issue`
depende de o estado das issues corresponder ao que os PRs afirmam. O modo de falha original é **invertido**:
o artefato se reporta saudável estando inerte (241 PRs, 4 fechamentos reais). É a mesma classe de
fail-open que esta casa persegue nos guards.

**Não é** defesa contra adversário, e está declarado assim no cabeçalho e no `partial=` do
`docs/cli-parity.md`. **Isso muda com a forma 4**, e é o ponto do threat model:

### 4.2 Quem esvazia este gate sem quebrar nenhuma regra escrita

O adversário aqui é **o implementador com pressa e o arquiteto otimista**, não um atacante.

1. 🔴 **O implementador que corrige a forma 4 com a janela de negação.** Ele passa nos 40 casos do corpus
   B, no autoteste, no corpus A, e **entrega um gate silenciável pela palavra `sem`** (§3.5-i). Não quebra
   nenhuma regra escrita: o AC6 pede que a negação deixe de ser acusada, e ela deixa. O que ele entrega é
   um gate que **qualquer autor apressado desliga sem saber** — e, pior, sem que o log diga que desligou.
   **A supressão é silenciosa por construção**: uma linha não acusada é indistinguível de uma linha limpa.
2. 🔴 **Quem fecha o AC1 primeiro.** É o caminho mais fácil (`types: [… edited]`, três palavras) e ativa
   o falso positivo da forma 4 em todo PR que declara escopo negativo — que esta casa **exige** em toda REQ.
   O resultado previsível é a pressão para afrouxar, e o afrouxamento chega pela supressão do item 1.
3. 🔴 **Quem marca o AC1 `[x]` pela leitura do código.** Já aconteceu: o #416 está mergeado, o código está
   correto, e **no CI o caminho da API nunca executa** (§2.3). O auditor que ler o diff conclui "corrigido".
   Só o log do run diz a verdade — e só porque o #416 teve o cuidado de imprimir a fonte.
4. **Quem marca o AC3 `[x]`** com cerca+span, sem medir a frase do #258 (que é o caso do issue) nem o
   bloco indentado por 4 espaços (§2.3, §3.3).
5. **Quem fecha o AC8** com as 4 frases da REQ. Medi **6 ocorrências** da forma 4, não 4 — as duas de #293
   estão num PR **mergeado com o check verde** e não estão citadas em nenhum artefato.
6. **O corpus que se autolimpa.** O remédio social da forma 4 é reescrever a frase; logo, cada ocorrência
   **apaga a própria evidência**. Um corpus coletado do corpo vivo **tende a zero falso positivo com o
   tempo, sem que nada tenha sido corrigido.** O corpus B tem de guardar as frases **literais**, versionadas
   (AC8), porque o corpus A vai perdê-las.

### 4.3 O que se perde se ele for afrouxado demais — onde está a fronteira

O número que define a fronteira está no próprio cabeçalho do gate, e eu o reconfirmei: afrouxar para
*"keyword em qualquer lugar da linha"* sobe de 1 para **43 linhas reprovadas** no corpus de 240 corpos.
Um gate com 43 acusações em 240 PRs é desligado em uma semana — e o `#293` mostra que **a erosão já começou**:
ele mergeou com `pr-closing-keyword` verde e duas linhas que a versão da época acusaria, e o job **não está
em `required_status_checks`** por decisão explícita do arquiteto. **O gate hoje é um conselho, não um bloqueio.**
Conselho com 67% de erro é conselho ignorado.

**A fronteira, em três invariantes verificáveis:**

| # | invariante | como se mede |
|---|---|---|
| **I1** | **Falso positivo = 0 no corpus A inteiro** | 357 corpos, nenhuma linha acusada que não seja declaração real de fechamento |
| **I2** | **Nenhuma correção reduz a cobertura.** As 4 declarações reais (#247, #312, #325, #330) continuam acusadas | as 4 no corpus B, por número |
| **I3** | 🔴 **Toda supressão é declarada e falsificada nas duas direções, NOS DOIS MATCHERS.** Cada zona não-assertiva e a regra de polaridade têm caso de **não-supressão** no autoteste — e cada zona tem também o caso de **palavra-chave inglesa dentro dela**, ⚠️ **com o veredito DEPENDENTE DO BALDE medido no `ML-0C`**: nas zonas **não-código** (aspas, blockquote, tabela) a inglesa **continua isentando**; nas zonas de **código** (cerca, span, indentado) a inglesa **NÃO isenta**, e o autoteste tem de afirmar essa recusa explicitamente (§10.4) | §3.5-i e §3.5-iv inteiras, como cenários permanentes |

🔴 **A segunda metade de I3 é fruto direto de §3.5(iv):** I1 é uma contagem de corpus e a classe de falso
positivo do passe único **não ocorre em 357 corpos**. Invariante que só se verifica por contagem de corpus
aprova por ausência de exemplo. **I3 é o único dos três que não passa por sorte.**

**I3 é a que o remendo viola.** I1 e I2 são contagens que qualquer implementação passa por sorte; I3 exige
que a regra de silenciamento **prove que não silencia demais**. Sem I3, o gate converge para o
*"nunca acusa"* — que o parecer mede como pior que o *"acusa sempre"*, porque o *"acusa sempre"* é visível
e o *"nunca acusa"* devolve o repositório ao estado de 241-PRs-4-fechamentos **com um check verde em cima**.

### 4.4 O efeito de processo que a REQ nomeou, e que confirmo

O remédio praticado para a forma 4 foi **reescrever a frase** (medido: #417 e #424, corpo vivo). Um gate cujo
remédio é **piorar a redação** treina o time a **não declarar escopo negativo** — exatamente o que esta casa
exige em toda REQ e em todo parecer de Wave 0. Isto não é dano estético: o escopo negativo é o mecanismo pelo
qual esta casa impede que um roadmap seja fechado com sítio conhecido e não corrigido. **A forma 4 corrói o
controle de governança mais forte do projeto, usando o gate de governança como instrumento.**

---

## 5. Frases de fechamento, por forma

Formato exigido: *"corrijo esta causa, exatamente estas frases deixam de ser acusadas, e nenhuma outra."*

**Forma 1 — não reavalia.**
> Corrijo a fonte do corpo no CI (`GH_TOKEN` + `pull-requests: read` + `types: [… edited]` em workflow
> dedicado) e exatamente estes vereditos deixam de descrever estado inexistente: os de PR cujo corpo mudou
> **depois** do último `opened`/`synchronize`. Nenhum veredito sobre corpo não editado muda. 🔴 **E nenhum
> falso positivo novo aparece — desde que as formas 3 e 4 já tenham fechado**, porque hoje esta correção
> ativa o falso positivo da forma 4 (§2.3).

**Forma 2 — exemplo citado.**
> Amplio a zona não-assertiva para bloco indentado por 4 espaços/tab, blockquote, linha de tabela e span
> entre aspas, e exatamente estas frases deixam de ser acusadas: as que citam a forma errada **dentro** de
> uma dessas zonas — inclusive a frase original do #258. Nenhuma palavra-chave **fora** dessas zonas deixa
> de ser acusada. 🔴 **E a palavra-chave inglesa dentro das zonas NÃO-CÓDIGO (aspas, blockquote, tabela)
> continua isentando** — essas três suprimem a acusação portuguesa em passe próprio e **não** são subtraídas
> do scan da isenção inglesa (§3.5-iv, **3 de 5** medidos). ⚠️ **Corrigido pelo `ML-0C`:** cerca, span e
> bloco indentado **não** entram nesse passe próprio — dentro delas o GitHub não fecha (§10), então a
> inglesa **não** pode isentar. ⚠️ **A frase do #258 depende da zona de aspas, que o AC3 proíbe hoje** —
> §3.3 exige decisão do arquiteto antes do ML.

**Forma 8 — ❌ RETIRADA (medido no `ML-0C`, 2026-09-24): não é defeito.**
> O gate de hoje apaga cerca e span dos **dois** matchers, e isso está **correto**: medido, o GitHub
> **ignora** palavra-chave de fechamento dentro de cerca (sonda **#428** = `[]`) e dentro de code span
> (sonda **#429** = `[]`), contra o controle em prosa (sonda **#427** = `[426]`). As duas zonas **não
> divergem entre si**. Logo `Fecha #246. A forma certa seria \`Closes #246\`.` **não fecha nada no merge**, e
> o aviso do gate é verdadeiro. 🔴 **Nenhuma frase deixa de ser acusada por conta desta forma** — o passo 1
> do `ML-N1`, na redação original, instalaria **falso negativo**. Diretriz de execução em **§10.4**.

**Forma 3 — markdown quebra a adjacência.**
> Abro a lacuna entre verbo e referência a caracteres **não-palavra** (`* _ ~ : , - — –` e `[`, espaço,
> tab) e amplio a gramática de referência para `(?:owner/repo)?#N` e URL de issue, e exatamente estas
> frases **passam a ser acusadas**: as que declaram fechamento em português com ênfase, pontuação, link ou
> referência cruzada entre a palavra e o número — `Fecha **#274** e **#275**` (#312), `Fecha **#314** e
> **#319**` (#325), `Fechar o **#320**` (#330). Nenhuma frase com **palavra** interveniente passa a ser
> acusada — `Fecha o **item 4** da issue #216` continua prosa —, e **número sem `#` continua fora**
> (+6 FP medidos se entrar). 🔴 **Esta correção não pode entrar sem a forma 4 no mesmo commit** (§3.5-ii).

**Forma 4 — negação lida como afirmação.**
> Leio a polaridade à esquerda do verbo dentro da mesma cláusula e exatamente estas frases deixam de ser
> acusadas: as que negam o fechamento na mesma cláusula — `Este PR **não** fecha a #363` (#417),
> `**Não fecha a #421**` (#424), `**Não fecha #290**` e `**Não fecha #275**` (#293), `Entrega sem fechar a
> #12`, `Deixa de fechar a #12`, `Não é verdade que fecha #12`. Nenhuma declaração afirmativa deixa de ser
> acusada quando a negação está em **outra cláusula** (`Isto não é refactor. Fecha #421`) ou **à direita**
> (`Fecha a #12, não a #13`). 🔴 **Mas deixam de ser acusadas, e não deveriam, sete frases afirmativas que
> contêm token de negação** (`Sem dúvida, fecha a #12`, `Não só fecha a #12, mas também…`, `Este PR, que
> não é refactor, fecha a #12`, `Nem só fecha a #12`, `Sem contar o #99, fecha a #12`, `Não apenas fecha a
> #12 — …`, `Não fecha a #12, mas fecha a #13`). **Esta é a fronteira, não um detalhe de calibração:
> nenhum ajuste da janela separa `Não é verdade que fecha #12` de `Sem contar o #99, fecha a #12`** — as
> duas têm ~20 caracteres de janela e querem vereditos opostos (§3.5-i).

**Formas 5/6 (novas) — conjugação e referência cruzada.**
> Amplio os sufixos dentro das 4 famílias declaradas e igualo a gramática de referência à do inglês **na
> parte que exige `#`**, e exatamente estas frases passam a ser acusadas: `Fecho a #12`, `Fechará a #12`,
> `Fechando a #12`, `Fecha kgsaran/trackfw#12`, `Fecha https://github.com/o/r/issues/12`. Nenhuma prosa do
> corpus A passa a ser acusada (medido: 4 → 4).

---

## 6. Residual declarado

1. **`Fecha (#274).` e `Fecha [#274](url)` com parêntese** continuam passando. Motivo **medido**: admitir
   `(` gera o falso positivo `#337 L11 — "issues já fechados** (#335, #336)"`. Colchete sozinho é seguro e
   entra; parêntese fica fora. **Custo aceito: um falso negativo conhecido para manter I1.**
2. **`Fecha a issue 12`** (sem `#`) continua passando. O `#` é o que mantém o falso positivo em zero
   (+6 FP medidos sem ele). **Deliberado.**
3. **Verbos fora das 4 famílias** (`Sana`, `Soluciona`, `Conserta`, `Elimina`, `Fixa` + `#N`) continuam
   passando. Lista fechada por escolha, não por completude.
4. **Paráfrase com palavras intervenientes** (`este PR fecha, por fim, a #246`) continua fora — residual já
   declarado no cabeçalho do gate e no `partial=` do `docs/cli-parity.md`. Confirmado, não ampliado.
5. 🔴 **A superfície de silenciamento da polaridade (§3.5-i) não tem solução dentro de regex.** Separar
   `Sem dúvida, fecha` de `Sem fechar` exige análise sintática, que este gate não deve ter. **O residual
   correto não é "aceitar o silenciamento em silêncio", é torná-lo visível**: quando a polaridade suprime
   uma acusação, o gate **diz no log** qual linha foi suprimida e por qual token — o mesmo padrão de
   honestidade que o #416 usou para a fonte do corpo, e que é o que permitiu medir a forma 1. Sem isso, a
   supressão viola o invariante de vacuidade declarado no cabeçalho (*"este gate NUNCA sai 0 em silêncio"*).
6. **Inglês negado** (`This PR does not close #12`) passa, e o gate acerta **por acidente** — ele nunca
   acusa inglês. Se algum dia acusar, a forma 4 volta pela porta inglesa. Registrado.
7. **`edited` dispara em mudança de título** (AC2). Não medi o custo de CI do workflow dedicado; é trabalho
   do ML de implementação.
8. ✅ **RESOLVIDO pelo `ML-0C` (2026-09-24) — deixou de ser residual.** A premissa *"o GitHub ignora
   palavra-chave de fechamento dentro de bloco de código"* era load-bearing e o corpus A não a decidia
   (zero PRs com a única keyword inglesa dentro de zona de código). Foi **medida empiricamente** com 7 PRs
   sonda: **verdadeira para cerca, code span e bloco indentado por 4 espaços; FALSA para blockquote, célula
   de tabela e span entre aspas** — nessas três o GitHub fecha normalmente. Evidência literal, números dos
   artefatos e a diretriz derivada estão em **§10**. O residual que **sobra** desta família está redefinido
   em **§10.5** (zonas que não medi: `<!-- comentário HTML -->`, `<pre>`/`<code>` em HTML, aspas curvas,
   `~~~` como cerca alternativa, cerca com til/atributo de linguagem).
9. **`TBL` apaga 1354 linhas do corpus A** (censo em §3.5-iv) — é a zona mais larga de todas. Nenhuma delas
   carrega declaração PT de fechamento hoje, mas a largura é dívida: uma tabela é zona não-assertiva por
   convenção deste repositório (tabelas de "forma errada × efeito"), não por regra do Markdown.

---

## 7. Atualização do contrato exigida (não feita aqui)

O `docs/cli-parity.md:7318` declara `partial=` com a lista de formas cobertas e não cobertas. Ele hoje diz
que *"trechos em cerca de código e code span são removidos antes de casar, então a forma errada CITADA como
exemplo não reprova"* — 🔴 **e isso está medido como falso** para aspas, blockquote, tabela e bloco
indentado por 4 espaços (§2.3). A declaração de contrato precisa ser corrigida **no mesmo commit** da
correção, junto com: a gramática de referência assimétrica, a lista fechada de conjugações, e a nova
superfície de silenciamento da polaridade.

➕ **Acrescentado pelo `ML-0C`:** o mesmo commit tem de declarar no contrato **o balde de cada zona** — que
a isenção inglesa **vale** dentro de aspas/blockquote/tabela e **não vale** dentro de cerca/span/indentado,
com a razão medida (o GitHub fecha nas primeiras e não fecha nas segundas, §10). Sem isso o contrato descreve
um comportamento uniforme que o gate corretamente **não** tem.

---

## 8. Reconciliação — o que cada medição deste parecer afirma

| medição | conclusão que ela sustenta |
|---|---|
| 357 corpos, 3 acusadas, 1 TP | a precisão de hoje é 1/3; a forma 4 é a **maioria** do output do gate |
| `closingIssuesReferences` de #247/#312/#325/#330 | os 4 são declarações reais; a cobertura de hoje é 1/4 |
| log do run 36074650903 (`fonte: payload`) | a forma 1 **não** está corrigida no CI; o #416 é inerte lá |
| gate de `9bc26ef6^1` × corpo vivo de #293 = rc=1, com check `SUCCESS` | a forma 1 **mascara** a forma 4 hoje |
| 40/40 no corpus B + 4/4 sem FP no corpus A | as 4 formas caem com **um** discriminante |
| `Não fecha **#421**` → 0 hoje, 1 com só-forma-3 | forma 3 e forma 4 **têm de entrar no mesmo commit** |
| 6/10 evasões de supressão | o discriminante único **troca** o modo de falha; I3 é obrigatório |
| C5 sem zona de aspas → #258 acusada | a forma 2 canônica depende do mecanismo que o AC3 proíbe |
| variante com número nu → 10 acusações, 6 FP | a simetria PT↔EN é **parcial** por medição, não por descuido |
| 5 de 5 com EN dentro de zona → acusa, **cruzado com a sonda do `ML-0C`: 3 de 5 são FP, 2 são acerto** | passe único destrói a isenção inglesa **só nas zonas não-código**: dois passes para aspas/blockquote/tabela, passe único para cerca/span/indentado. O "0 FP" de §3.4 é do corpus, não do discriminante |
| censo das zonas (80/62/1354/831 linhas) | `INDENT` e `QUOTE` são seguras neste corpus; `TBL` é a mais larga e é dívida declarada |
| **zero** PRs com EN só dentro de cerca no corpus A | o corpus **não decide** a premissa "o GitHub ignora keyword em código" — foi por isso que o `ML-0C` teve de medir por sonda |
| **7 PRs sonda** (#427–#429, #431–#434) contra 2 issues descartáveis (#426, #430) | o GitHub **ignora** keyword nas zonas de **código** (cerca, span, indentado) e **honra** nas **não-código** (blockquote, tabela, aspas) → a forma 8 **não é defeito**, e o passe duplo é **por balde de zona** (§10) |

---

## 9. O que este parecer NÃO cobre

- ✅ **Coberto depois, pelo `ML-0C` (§10):** o comportamento real do GitHub para palavra-chave dentro de
  cerca, code span, bloco indentado, blockquote, célula de tabela e aspas retas foi medido por 7 PRs sonda
  neste próprio repositório. As zonas que **continuam** sem medição estão nomeadas em §10.5. O que este
  parecer (`ML-0B`) não cobria, e ficou escrito como premissa, era exatamente isto.
- **Não medi custo de CI** do workflow dedicado do AC2.
- **Não implementei nada.** O candidato de §3.2 é **prova de existência**; colá-lo no gate embarcaria a
  superfície de silenciamento de §3.5-i sem exame e o passe único de §3.5-iv sem correção.
- **Não corri `make quality`** (barreira do arquiteto) nem `git` de escrita.

---

## 10. ADENDO `ML-0C` (2026-09-24) — o GitHub honra palavra-chave de fechamento dentro de bloco de código?

> **Autor:** Hades · **ML:** `ML-0C` da Wave 0 · **Escopo:** medição. **Zero linha de implementação** —
> `scripts/check-pr-closing-keyword.sh` **não foi tocado** neste ML.
> **Por que este adendo existe:** a premissa era load-bearing e o corpus A não a decidia em nenhuma direção
> (zero PRs cuja única palavra-chave inglesa vive dentro de zona de código). O único caminho restante era
> empírico.

### 10.1 🔴 Uma premissa minha do `ML-0B` foi REFUTADA — isto vem primeiro

**Refutado:** a linha 5 da tabela de §3.5-iv e a família **(8)** de §2.4. No `ML-0B` eu escrevi que
`Fecha #246.` + `Closes #246` dentro de cerca é **falso positivo do gate de hoje** (`esp` = 0) e que
*"o GitHub fecha a #246 nos cinco casos"*.

**Medido: o GitHub NÃO fecha nesse caso.** O `esp` correto é **1**, o gate de hoje **acerta**, e a forma 8
**não é defeito** — é comportamento correto. O erro era meu, não do gate. **Também refutada** a linha 4 da
mesma tabela (bloco indentado por 4 espaços): o GitHub também não fecha ali.

**Não refutado, e confirmado por medição nova:** as linhas 1, 2 e 3 (aspas, blockquote, célula de tabela) —
nessas três o GitHub **fecha**, logo mascará-las do scan inglês **seria** falso positivo, e o passe duplo é
o remédio certo **para elas**.

### 10.2 Instrumento, e o braço de controle que valida a medição

`gh pr view <n> --json closingIssuesReferences` — o mesmo instrumento de verdade-terreno usado no `ML-0B`
para os falsos negativos #312/#325/#330.

**7 PRs sonda** (rascunho, base `main`, **nunca mergeados**) contra **2 issues descartáveis**, todos criados
e destruídos **pela API** (`gh api` para refs e Contents, `gh pr create`, `gh pr close --delete-branch`) —
**sem `git` de escrita**, por desenho: a autoridade Git é do arquiteto e artefato de medição descartável não
entra no histórico desta REQ. Cada corpo continha a palavra-chave inglesa **exclusivamente** na zona sob
teste — nunca no título, nunca em prosa, sem segundo `#N`.

🔴 **O braço de controle é o que torna a medição interpretável.** Sem ele, `[]` nos braços é indistinguível
de instrumento quebrado. Ele voltou **não-vazio**, logo o instrumento funciona neste repositório e nestas
condições (PR rascunho, base `main`).

### 10.3 O resultado literal

Comando, sonda 1 (`--jq '[.closingIssuesReferences[].number]|tostring'`, issue alvo **#426**):

```
#427 [426]      ← CONTROLE: `Closes #426` em prosa
#428 []         ← braço A: `Closes #426` só dentro de cerca ```
#429 []         ← braço B: `Closes #426` só dentro de code span `…`
```

JSON bruto do controle (#427), para reauditoria:

```json
{"closingIssuesReferences":[{"id":"I_kwDOS3zsKc8AAAABTGIfGw","number":426,
"repository":{"id":"R_kgDOS3zsKQ","name":"trackfw","owner":{"login":"kgsaran"}},
"url":"https://github.com/kgsaran/trackfw/issues/426"}]}
```

Sonda 2 (issue alvo **#430**) — as quatro zonas restantes que o `ML-N2` pretende adicionar:

```
#431 []         ← bloco indentado por 4 espaços
#432 [430]      ← blockquote  `> Closes #430`
#433 [430]      ← célula de tabela  `| Closes #430 | alvo |`
#434 [430]      ← aspas retas  a linha certa seria "Closes #430"
```

Todos os 7 foram lidos **duas vezes**, com a segunda leitura depois do intervalo de criação, para excluir
consistência eventual: **valores idênticos nas duas leituras.**

**Veredito por zona, e é um veredito de DOIS BALDES:**

| zona | `closingIssuesReferences` | o GitHub fecha? | balde |
|---|---|---|---|
| prosa (controle) | `[426]` | **sim** | — (referência normal) |
| cerca ```` ``` ```` | `[]` | **não** | **CÓDIGO** |
| code span `` ` `` | `[]` | **não** | **CÓDIGO** |
| bloco indentado 4 espaços | `[]` | **não** | **CÓDIGO** |
| blockquote `>` | `[430]` | **sim** | **NÃO-CÓDIGO** |
| célula de tabela | `[430]` | **sim** | **NÃO-CÓDIGO** |
| aspas retas `"…"` | `[430]` | **sim** | **NÃO-CÓDIGO** |

**Cerca e code span NÃO divergem entre si** — era o risco que o `ML-0C` tinha de excluir, e está excluído:
as duas devolveram `[]`. 🔴 **A divergência real está em outro eixo:** não é cerca × span, é **código ×
não-código**. As três zonas de código se comportam igual entre si, as três não-código se comportam igual
entre si, e os dois baldes se comportam de forma **oposta**. Um fix que tratasse as seis zonas juntas — em
qualquer das duas direções — erraria metade delas.

### 10.4 🔴 Diretriz de execução para o `ML-N1`, passo 1 — sem ambiguidade

O passo 1 do `ML-N1` diz *"só execute este passo na direção que o `ML-0C` medir"*. A direção é esta, e é uma
instrução, não um veredito a derivar:

> **Implemente os dois passes de mascaramento — mas a máscara das zonas de CÓDIGO (cerca, code span e bloco
> indentado por 4 espaços/tab) continua subtraída dos DOIS matchers, exatamente como o `blank_code` de hoje
> faz. Só a máscara das zonas NÃO-CÓDIGO (blockquote, célula de tabela, span entre aspas) vai para o passe
> próprio que suprime a acusação portuguesa e NÃO é subtraído do scan da isenção inglesa.**
>
> 🔴 **E a "forma 8" sai da lista de correções do `ML-N1`.** Não há nada a corrigir ali. Fazer a isenção
> inglesa valer dentro de cerca/span instalaria **falso negativo**: o gate ficaria calado sobre um corpo que
> declara `Fecha #246` e cuja única forma inglesa está numa zona que o GitHub ignora — ou seja, um corpo que
> **não fecha a issue**. É a direção oposta do defeito, e é pior que o defeito: o incômodo de hoje é um aviso
> verdadeiro; o defeito novo seria silêncio sobre uma declaração falsa.

**Falsificação obrigatória, nas duas direções** (entra no autoteste do `ML-N1`/`ML-N2`, por I3):

| caso | veredito exigido | por quê |
|---|---|---|
| `Fecha #246.` + cerca com `Closes #246` | **acusa** (rc=1) | #428: não fecha |
| `Fecha #246. A forma certa seria \`Closes #246\`.` | **acusa** (rc=1) | #429: não fecha |
| `Fecha #246.` + bloco indentado com `Closes #246` | **acusa** (rc=1) | #431: não fecha |
| `Fecha #246.` + `> Closes #246` | **cala** (rc=0) | #432: fecha de verdade |
| `Fecha #246.` + célula de tabela com `Closes #246` | **cala** (rc=0) | #433: fecha de verdade |
| `Fecha #246. A linha certa é "Closes #246".` | **cala** (rc=0) | #434: fecha de verdade |

E o efeito colateral que o `ML-N2` **não** pode perder de vista: a acusação **portuguesa** dentro das zonas
de código continua suprimida (`Fecha #246` citado em cerca não reprova) — isso permanece correto, e por dois
motivos independentes: é citação, **e** não fecha nada de qualquer forma.

### 10.5 Residual redefinido — o que este ML aceita não cobrir

1. **Zonas não medidas, nomeadas:** `<!-- comentário HTML -->`, `<pre>`/`<code>` em HTML bruto, aspas
   **curvas** (`“…”` — medi só as retas), cerca com `~~~` em vez de ```` ``` ````, cerca com atributo de
   linguagem (```` ```bash ````), lista/`<details>`. **Não extrapolo o balde por analogia** — a analogia é
   exatamente o que produziu a premissa errada que este ML refutou. Se o `ML-N2` for adicionar qualquer uma
   dessas zonas à gramática, ela precisa da sua própria sonda.
2. **Condições da medição:** PR **rascunho**, base `main`, mesmo repositório, autor = dono do repositório,
   `closingIssuesReferences` lido com o PR **aberto**. Não medi PR de fork, nem PR entre branches não-default,
   nem o efeito do merge real (não mergeei nenhuma sonda — de propósito).
3. **`closingIssuesReferences` é a leitura do parser do GitHub, não o merge.** É o mesmo instrumento que o
   `ML-0B` usou como verdade-terreno para os 4 PRs que fecharam issue de verdade, e a concordância entre os
   dois usos é o que o autoriza aqui. Não observei um merge fechando issue nesta sonda.
4. **Comportamento do GitHub é externo e pode mudar.** A medição tem data. Se o parser mudar, a diretriz de
   §10.4 muda com ele — e o sinal de alerta é o autoteste da tabela acima virando vermelho sem mudança no
   gate.

### 10.6 Artefatos de medição — todos fechados, citados para reauditoria

| artefato | número | estado final |
|---|---|---|
| issue alvo da sonda 1 | **#426** | `closed` |
| PR sonda — controle (prosa) | **#427** | `closed`, branch `probe/ml0c-control` deletada |
| PR sonda — cerca | **#428** | `closed`, branch `probe/ml0c-fence` deletada |
| PR sonda — code span | **#429** | `closed`, branch `probe/ml0c-span` deletada |
| issue alvo da sonda 2 | **#430** | `closed` |
| PR sonda — bloco indentado | **#431** | `closed`, branch `probe/ml0c2-indent` deletada |
| PR sonda — blockquote | **#432** | `closed`, branch `probe/ml0c2-bq` deletada |
| PR sonda — célula de tabela | **#433** | `closed`, branch `probe/ml0c2-tbl` deletada |
| PR sonda — aspas retas | **#434** | `closed`, branch `probe/ml0c2-quote` deletada |

Nenhuma sonda foi mergeada. Nenhum arquivo `.probe-ml0c*.txt` existe na `main` — eles viveram só nas branches
descartáveis, que foram deletadas (`gh api repos/:owner/:repo/git/matching-refs/heads/probe` devolve vazio).
Os runs de CI disparados pela abertura das sondas foram **cancelados** (13 runs), por custo de máquina.
