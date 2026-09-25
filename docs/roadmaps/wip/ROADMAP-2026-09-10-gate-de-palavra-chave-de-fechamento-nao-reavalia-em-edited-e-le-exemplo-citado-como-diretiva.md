---
status: wip
date: 2026-09-10
req: "docs/req/REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md"
squad: ""
---

# Roadmap: gate de palavra-chave de fechamento nao reavalia em edited e le exemplo citado como diretiva

> Created: 2026-09-10 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md -->
REQ: docs/req/REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído por absorção no `ML-0B` (2026-09-25) — as quatro seções abaixo foram
respondidas com medição em `docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md`;
o `ML-0B` é a versão ampliada deste ML, não um ML paralelo
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
      → §2.4 (enumeração, com busca ativa: 8 famílias), §4.2 (quem esvazia), §3.4 (falsificação nas duas
      direções), §6 (residual declarado)
- [x] No implementation line written for this ML
      → `scripts/check-pr-closing-keyword.sh` intocado em toda a Wave 0

### ML-0C — 🔴 O GitHub honra palavra-chave de fechamento dentro de bloco de código?
**Owner:** `hades-tf`
**Status:** ⬜ Pendente · **bloqueia a forma 8 do `ML-N1`**

**Por que este ML existe.** O parecer declarou esta premissa **não verificada** e mediu que o corpus A
**não a decide em nenhuma direção** (zero PRs cuja única keyword inglesa viva em cerca ou span). E ela
não é um detalhe: **ela inverte o veredito esperado da forma 8.**

| se o GitHub **ignora** keyword em cerca | se o GitHub **honra** |
|---|---|
| `Closes #246` em cerca **não fecha nada** → o aviso do gate está **certo**, e `esp` da 2ª linha da tabela de §2.4 é **1**, não 0 | a isenção é **real** → mascará-la é falso positivo, e os **dois passes** são o fix |

🔴 **Sem esta medição, "corrigir a forma 8" pode ser instalar o defeito na direção oposta.** É a Regra
Dura de Reconciliação aplicada antes da entrega, não depois.

**Ações:**
1. Medir empiricamente — é o único caminho, o corpus já foi esgotado. Use o instrumento que esta campanha
   já usou para os falsos negativos #312/#325/#330:
   `gh pr view <n> --json closingIssuesReferences`.
2. Abra uma issue de teste e um PR de teste **neste** repositório, contra uma branch descartável, cujo
   corpo tenha a keyword inglesa **exclusivamente** dentro de cerca — e uma segunda variante com ela
   exclusivamente dentro de **code span** (as duas zonas são tratadas juntas por `blank_code` hoje, e
   podem divergir no GitHub).
   🔴 **Sem `git` local, e isto é uma autorização explícita, não uma sugestão.** A autoridade Git é do
   arquiteto e continua sendo: crie a branch e o PR de teste **pela API** (`gh api` para a Contents API,
   `gh pr create --head`). Nenhum `git checkout -b`, nenhum `git push`, nenhum commit na branch de
   trabalho — o hook do projeto intercepta esses comandos por desenho, e artefato de medição descartável
   não entra no histórico desta REQ.
3. Leia `closingIssuesReferences` de cada uma. **Feche a issue e o PR de teste ao final**, e registre os
   números no parecer para que a medição seja reauditável.
4. Delete a branch de teste pela API ao final (`gh api -X DELETE repos/:owner/:repo/git/refs/heads/<b>`).
5. Escreva o resultado como **adendo** ao parecer `docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md`,
   substituindo a ressalva "⚠️ premissa NÃO VERIFICADA" de §2.4 pela medição.

**Critérios de aceite:**
- [ ] `closingIssuesReferences` medido para **cerca** e para **code span**, separadamente
- [ ] O veredito da forma 8 declarado: **é defeito** (dois passes) ou **é comportamento correto** (e então
      a tabela de §2.4 tem `esp` errado, e isso fica escrito)
- [ ] 🔴 Se as duas zonas divergirem entre si, isso fica escrito — o fix não pode tratá-las juntas
- [ ] Artefatos de teste (issue + PR) **fechados**, com os números citados
- [ ] Nenhuma alteração em `scripts/check-pr-closing-keyword.sh` — este ML **mede**, não corrige

> 🔴 O `barrier` executa **uma linha por vez**, sem estado entre elas — variável definida numa linha não
> existe na seguinte. Cada linha abaixo é auto-contida por isso, e não por estilo.

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md
! grep -q 'NÃO VERIFICADA' docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md
git diff --quiet "$(git merge-base origin/main HEAD)" HEAD -- scripts/check-pr-closing-keyword.sh
```
O segundo braço é o que **bloqueia a forma 8**: enquanto a premissa do bloco de código estiver marcada
não verificada em §2.4, a Wave 0 não fecha. O terceiro é a falsificação de que a Wave 0 **mede e não
corrige**.

## Wave 1 — ❌ SUBSTITUÍDA (2026-09-25) — leia a Wave 2, não esta
> 🔴 **Nada aqui deve ser executado.** Esta wave foi escrita antes da Wave 0 ampliada e está
> **contraditória com o que foi medido**, em três pontos:
>
> | ML antigo | por que caiu |
> |---|---|
> | `ML-1A` (AC1 primeiro) | a ordem travada põe o AC1 **por último** — o #293 prova que o contrário red-lina todo PR com escopo negativo |
> | `ML-1C` ("não heurística de aspas") | a **Decisão (a)** amplia o AC3 **para incluir** a zona de aspas — é o único mecanismo que salva a frase canônica do #258 |
> | `ML-NOVO` (adjacência markdown) | é a **forma 3**, absorvida no `ML-N1`; mantido abaixo só como registro da medição de 2026-09-10 |
>
> Um executor que lê este arquivo de cima para baixo pegaria a ordem errada. **A wave viva é a Wave 2.**

### ML-1A — **AC1** — O gate reavalia no evento edited, não só na abertura.
**Status:** ❌ Substituído por `ML-N1`/`ML-N2`/`ML-N3` (ordem travada, 2026-09-25)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — O gate reavalia no evento edited, não só na abertura.
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **Sem repetir as suítes.** Um edited que dispare o quality inteiro faz o custo
**Status:** ❌ Substituído por `ML-N1`/`ML-N2`/`ML-N3` (ordem travada, 2026-09-25)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **Sem repetir as suítes.** Um edited que dispare o quality inteiro faz o custo
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Contrato próprio para "exemplo citado", não heurística de aspas.** A auditoria é
**Status:** ❌ Substituído por `ML-N1`/`ML-N2`/`ML-N3` (ordem travada, 2026-09-25)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Contrato próprio para "exemplo citado", não heurística de aspas.** A auditoria é
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — Falsificação nas duas direções para o edited: PR aberto sem palavra-chave e
**Status:** ❌ Substituído por `ML-N1`/`ML-N2`/`ML-N3` (ordem travada, 2026-09-25)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — Falsificação nas duas direções para o edited: PR aberto sem palavra-chave e
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
**Status:** ❌ Substituído por `ML-N1`/`ML-N2`/`ML-N3` (ordem travada, 2026-09-25)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
- [ ] build passes
- [ ] tests green

### ML-NOVO — A adjacência quebra com markdown entre a palavra e o `#N`

**Status:** ❌ Substituído pela **forma 3** do `ML-N1` · a medição abaixo continua válida e é a fonte dela

```
Fecha #274.                   → gate RECUSA   ✅ correto
Fecha **#274** e **#275**.    → gate PASSA    🔴 defeito
```

O `**` do negrito entre a palavra e o `#N` quebra a adjacência que o gate procura. **Não foi ataque:
é markdown normal**, escrito sem intenção de driblar nada.

**Consequência medida:** o PR **#312** foi mergeado com *"Fecha **#274** e **#275**"*, o gate ficou
**verde**, e os dois issues **continuaram abertos**. Fechados à mão depois.

🔴 **É a mesma classe dos outros dois defeitos deste issue:** o gate mede uma forma mais estreita do
que a que afirma cobrir. Ver
`vault/notes/guarda-que-reporta-ausencia-precisa-distinguir-nao-achei-de-nao-consegui-procurar-2026-09-10.md`.

**Falsificação obrigatória — as formas que um humano escreve sem pensar:**

```
Fecha **#N**        Fecha o **#N**        Fecha [#N](url)
Fecha `#N`          Fecha: #N             Fecha os #N e #M
```

E o contra-braço: prosa legítima que **cita** um issue sem pretender fechá-lo **não** pode reprovar —
senão o gate vira ruído e alguém o desliga.

---

## 🔴 Ampliação (2026-09-24): a REQ passou de 2 para 4 formas

As formas 3 (markdown quebra a adjacência) e 4 (negação lida como afirmação) foram medidas **depois**
deste roadmap. Pela Regra Dura, entram aqui.

**A forma 4 é a que muda a prioridade**, e não por frequência: as outras três produzem **ruído**; ela
produz **dano dirigido** — o conselho do gate fecharia uma issue viva. Quatro ocorrências, e a última
foi **minha**, no PR #424, **um dia depois** de eu documentar a forma no #258.

⚠️ **E o contorno atual treina o time a não declarar escopo negativo** — nas duas vezes o remédio foi
reescrever a frase para não conter a palavra proibida. Um gate cujo remédio é **piorar a redação**
desincentiva o que esta casa exige em toda REQ.

### ML-0B — Wave 0 ampliada: o discriminante é um só, e precisa cobrir as 4
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-25 · **8 famílias, não 4**
**Entregue:** `docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md`
**Arquivos afetados:** nenhum de produto — parecer em `docs/seguranca/`

**Ações:**
1. **Leia o gate** (`scripts/check-pr-closing-keyword.sh`) e diga **qual é o discriminante hoje** —
   em uma frase, não em paráfrase do código.
2. **Enumere as formas** que ele erra, **pela forma e não pelo token** — esta campanha teve **cinco**
   enumerações que se revelaram limite inferior por caçar token. As 4 conhecidas são o piso, não o
   teto. Procure ativamente: negação distante (`não é verdade que fecha #N`), verbo em outra pessoa,
   `#N` em bloco de código, referência cruzada (`owner/repo#N`), e a palavra dentro de link.
3. 🔴 **Meça a tensão entre as correções.** Corrigir a negação com uma janela à esquerda pode
   reintroduzir a forma 2 (exemplo citado *tem* contexto à esquerda). **As quatro têm de cair com um
   discriminante só** — se não caírem, diga **quais são incompatíveis** e por quê.
4. **Threat model:** o gate é um **controle de processo**. Qual garantia ele oferece, e o que se
   perde se ele for afrouxado demais? 🔴 Um gate que nunca acusa é tão inútil quanto um que acusa
   sempre — e o caminho do remendo leva ao primeiro.
5. Frase de fechamento por forma: *"corrijo esta causa, exatamente estas frases deixam de ser
   acusadas, e nenhuma outra."*

**Critérios de aceite:**
- [ ] O discriminante atual dito em **uma frase**
- [ ] Enumeração **pela forma**, com o comando e com a busca ativa pelo que não está na lista
- [ ] 🔴 Tensão entre as correções **medida** — as 4 caem juntas, ou as incompatibilidades ficam escritas
- [ ] Threat model do gate como controle de processo
- [ ] Frase de fechamento por forma
- [ ] 🔴 Nenhuma linha de implementação
- [ ] 🔴 **NÃO rodar `make quality`**

**O discriminante de hoje, em uma frase — e a segunda metade é o achado:**

> O gate acusa quando um verbo de fechamento em português aparece **lexicalmente adjacente** a `#N`
> — separado no máximo por ênfase markdown colada, artigo e/ou "issue" — e o mesmo `N` não tem
> palavra-chave inglesa em nenhum lugar do corpo; **ele nunca pergunta se a frase *afirma*
> fechamento.**

A adjacência é escolha **declarada**. A ausência de noção de **asserção** (polaridade, citação,
autoria) **não está declarada em lugar nenhum** — e é a causa única das 8 formas. 🔴 **Não são 8
defeitos: é 1 discriminante medindo a coisa errada**, o que confirma a Regra Dura em mantê-las juntas.

## 🔴 Três achados que mudam o plano, os dois primeiros auditados por mim

**1. O defeito 1 NÃO está corrigido no CI — e o artefato do #416 afirma o que o ambiente nega.**

| medição | resultado |
|---|---|
| `grep -c 'GH_TOKEN' .github/workflows/quality.yml` | **0** |
| existe em outros workflows? | sim — `check-annotations.yml`, `release.yml` |
| o job do gate | `run: scripts/check-pr-closing-keyword.sh`, **sem `env:`** |

**O caminho da API que o #416 entregou nunca executa em produção.** A tabela de medição daquele PR
foi produzida **localmente**, com `gh` autenticado. É a classe exata da Regra Dura de Reconciliação:
o artefato afirma o que o ambiente real nega — e só o log salvou o achado de ser silencioso.

**2. 🔴 O defeito 1 está MASCARANDO a forma 4.** Auditei: o **#293 está `MERGED`** e o corpo vivo
contém `**Não fecha #290**` e `**Não fecha #275**` — duas linhas que a versão do gate da época
**acusaria**, e o check foi **verde**. Entraram por **edição posterior** ao último evento com payload.

**Consequência operacional:** corrigir o AC1 **sozinho** ativa falso positivo em **todo** PR que
declara escopo negativo. **Ordem travada:** formas 3+4+8 no **mesmo commit**, AC1 **depois**.

**3. As métricas de hoje, medidas em 357 corpos de PR mergeado:**

```
baseline:  355 rc=0 · 2 rc=1 · 1 rc=2
acusações: #247 (1 verdadeiro) + #293 ×2 (falsos positivos, PR MERGEADO)
precisão: 1 acerto em 3 acusações      cobertura: 1 de 4 declarações reais
```

Os 3 falsos negativos são reais e verificados por `closingIssuesReferences`: **#312**, **#325**,
**#330** — issues fechadas **à mão** depois do merge. 🔴 **Conselho com 67% de erro é conselho
ignorado**, e o job **não está em `required_status_checks`**.

## Veredito sobre a tensão: as 4 caem com um discriminante só — com dois custos nomeados

**Custo 1 — a superfície de silenciamento NÃO se fecha em regex.** 6 de 10 frases **afirmativas** com
token de negação silenciam. E a prova de impossibilidade é limpa:

```
"Não é verdade que fecha #12"   (21 chars)  → deve acusar
"Sem contar o #99, fecha a #12" (18 chars)  → não deve acusar
```

**Mesma janela, vereditos opostos.** 🔴 **Decisão minha: o residual é tornar a supressão VISÍVEL no
log**, não fechá-la. É o mesmo padrão de honestidade do #416 — que foi o que permitiu medir a forma 1
— e o cabeçalho do próprio gate declara *"este gate NUNCA sai 0 em silêncio"*.

**Custo 2 — passe único destrói a isenção inglesa.** 5 de 5 casos. Fecha com **dois passes**. ⚠️ E ele
foi honesto no limite: *"não ocorre nos 357 corpos, logo o '0 falso positivo' é propriedade do
**corpus**, não do discriminante"*.

**Incompatibilidades, com o caso que as prova:**

| par | veredito |
|---|---|
| forma 3 separada da 4 | **incompatível** — `Não fecha **#421**.`: só-forma-3 cria FP novo. **Mesmo commit** |
| AC1 antes da forma 4 | **incompatível** — #293 |
| forma 2 em passe único | **incompatível** — 5 de 5 |
| simetria PT↔EN sem exigir `#` | **+6 FP medidos** (`Fecha 2 dos 3 elos`) — exigir `#` |

## Decisões minhas

**(a) O AC3 será AMPLIADO para incluir span entre aspas.** Ele proíbe hoje o que **salva a frase
canônica do #258** — e ser acusado pela própria descrição do defeito é auto-contraditório. O custo é
**zero no corpus A** medido, e aspas são o **quinto de cinco** mecanismos de zona, não o único. A
alternativa (manter estrito e declarar residual) deixaria a frase do issue acusada para sempre.

**(b) O `GH_TOKEN` entra no `quality.yml`** — sem ele o #416 é código morto. 🔴 **Com escopo mínimo e
declarado**: o job lê corpo de PR, nada mais. Aumentar superfície de token exige razão escrita, e
essa é a razão.

**(c) A correção de `docs/cli-parity.md:7318` vai no MESMO commit.** Ele afirma que *"a forma errada
citada como exemplo não reprova"* — **medido como falso** para aspas, blockquote, tabela e bloco
indentado. Contrato que mente é pior que contrato ausente.

## Fronteira do afrouxamento, e como a Wave 0 pode ser esvaziada

Ele mapeou **seis** caminhos para passar sem quebrar regra escrita. O que eu levo para os handoffs:

- fechar a forma 4 com janela de negação **passa em tudo** e entrega um gate **silenciável pela
  palavra "sem"**, em silêncio;
- marcar AC1 `[x]` pelo diff — **já aconteceu no #416**;
- fechar o AC8 com 4 frases quando são **6 ocorrências** (as 2 do #293 não estão em artefato nenhum).

**Três invariantes**, e só o terceiro não passa por sorte:

| | |
|---|---|
| **I1** | falso positivo = 0 no corpus inteiro |
| **I2** | as 4 declarações reais continuam acusadas |
| **I3** | 🔴 toda supressão declarada e falsificada **nos dois matchers** |

⚠️ **Premissa load-bearing NÃO verificada, e declarada:** *"o GitHub ignora palavra-chave dentro de
bloco de código"*. Ele buscou PRs cuja única keyword inglesa vive em cerca: **zero**. O corpus não
decide — e é essa premissa que determina se suprimir a acusação dentro de cerca está certo.

**Critérios de aceite:**
- [x] Discriminante em uma frase — e a segunda metade é o achado
- [x] Enumeração pela forma: **8 famílias**, com busca ativa e as frases **reais** das ocorrências
- [x] 🔴 Tensão **medida**: caem juntas, com 2 custos e 4 incompatibilidades, cada uma com o caso
- [x] Threat model com a fronteira e os 6 caminhos de esvaziamento
- [x] 8 frases de fechamento
- [x] Estado do defeito 1 **conferido** — e **refutado**: não corrigido no CI
- [x] 🔴 Nenhuma implementação

⚠️ **Fronteira de escrita:** ele escreveu o `agents-working-context.md` além do parecer e **declarou o
conflito** com o meu AC em vez de resolver em silêncio. **A fronteira era minha e estava estreita** —
já reconheci isso em REQ anterior. Mantido.

---

## Wave 2 — Implementação, em ORDEM TRAVADA
> Dependências: `ML-0B` ✅ · `ML-0C` (só a forma 8 do `ML-N1`)
> 🔴 **A ordem é consequência de medição, não de preferência.** Ver §"Três achados" e §"Veredito".
> Os três MLs são **estritamente sequenciais** — todos tocam
> `scripts/check-pr-closing-keyword.sh`. Não há paralelismo a extrair aqui.

**O gate de aceitação é o mesmo nos três, e é a re-execução contra o corpus A (357 corpos):**

```
baseline de hoje:  355 rc=0 · 2 rc=1 · 1 rc=2   |  precisão 1/3  ·  cobertura 1/4
alvo:              0 falso positivo  ·  as 4 declarações reais acusadas (#247, #312, #325, #330)
```

⚠️ **E a ressalva do parecer entra escrita no AC, não no rodapé:** *"0 falso positivo é propriedade
**deste corpus**, não do discriminante"* — a simetria PT↔EN sem `#` obrigatório, por exemplo, mediu **+6
FP**. Marcar `[x]` por um número que não transfere é o que a Regra Dura de Reconciliação proíbe.

### ML-N1 — Esqueleto de dois passes + formas 3, 4, 5, 6 e 8 — **um único commit**
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente

🔴 **Por que num commit só, e não um ML por forma.** Medido em §3.6: `Não fecha **#421**.` casa a forma 3
(markdown na lacuna) **e** a forma 4 (negação). Fechar só a forma 3 faz o gate **passar a ver** a frase e
**acusá-la** — cria falso positivo novo onde hoje há silêncio. A forma 3 isolada é uma **regressão**.

**A ordem interna também é travada, para não reescrever trabalho:** o esqueleto de **dois passes** vem
**primeiro**, e só então os discriminantes. Se os discriminantes vierem antes, a chegada do `ML-N2` (que
exige dois passes) reescreve o que este ML acabou de entregar.

**Ações, na ordem:**
1. **Dois passes de mascaramento** (o fix da forma 8): a zona não-assertiva suprime a **acusação
   portuguesa** e **não** é subtraída do scan da **isenção inglesa**. Hoje `blank_code` é aplicado uma vez
   e o `scan` alimenta os dois. 🔴 **Só execute este passo na direção que o `ML-0C` medir.**
2. **Forma 3** — lacuna entre verbo e referência aberta a caracteres **não-palavra** (`* _ ~ : , - — – [`
   e espaço), **nunca a palavras**. O contra-braço obrigatório é `Fecha o **item 4** da issue #216`, que
   deve continuar passando como prosa.
3. **Forma 4** — polaridade: negação (`não · nem · nunca · jamais · sem · deixa de · em vez de · ao invés
   de`) à esquerda do verbo **dentro da mesma cláusula**; quebradores de cláusula `. ; : ! ?` e fim de linha.
4. **Forma 6** — gramática de referência **simétrica**: o lado português passa a aceitar `owner/repo#N` e
   URL completa de issue, como o `EN_REF` já aceita. 🔴 **O `#` continua obrigatório no lado português** —
   sem ele, +6 FP medidos (`Fecha 2 dos 3 elos`).
5. **Forma 5** — conjugações (`fecho`, `fechará`, `fechando`, …) **dentro das 4 famílias já declaradas**.
   Não ampliar a lista de verbos: a **forma 7** (`Sana`, `Soluciona`, `Conserta`, `Elimina`) fica
   **residual aceito e declarado**, por decisão do parecer que eu ratifico — ampliar sem medir custo de FP
   é o caminho do gate ruidoso que alguém desliga.
6. 🔴 **A supressão por polaridade sai no log, sempre.** A superfície de silenciamento **não fecha em
   regex** — prova em §3.5: `Não é verdade que fecha #12` (21 chars, deve acusar) e `Sem contar o #99,
   fecha a #12` (18 chars, não deve) têm a **mesma janela e vereditos opostos**. Como o residual não é
   eliminável, ele tem de ser **visível**: toda vez que o gate deixar de acusar por negação, imprima a
   frase e o token que suprimiu. O cabeçalho do próprio gate já promete *"este gate NUNCA sai 0 em
   silêncio"*.

**Critérios de aceite:**
- [ ] Corpus A: **0 falso positivo** — em particular o **#293** (`Não fecha #290` / `Não fecha #275`)
      deixa de ser acusado
- [ ] Corpus A: **#312**, **#325** e **#330** passam a ser acusados (hoje são falsos negativos medidos)
- [ ] `Fecha o **item 4** da issue #216` continua **não** acusado — contra-braço da forma 3
- [ ] `Fecha 2 dos 3 elos` continua **não** acusado — o `#` obrigatório
- [ ] Autoteste do gate cobre as 5 formas, **nas duas direções** (acusa quando deve, cala quando deve)
- [ ] 🔴 Toda supressão por polaridade **aparece no log**, com a frase e o token
- [ ] 🔴 **Invariante I3 do threat model:** toda supressão declarada **e falsificada nos dois matchers**
      (PT e EN) — não só no português
- [ ] Reconciliação: **uma frase por teste novo**, dizendo qual conclusão deste ML ele afirma
- [ ] `make quality` verde · `go build ./...` verde

### ML-N2 — Forma 2 (exemplo citado) — AC3 **ampliado** para span entre aspas
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente · **depende de `ML-N1` (dois passes)**

🔴 **Este ML altera o AC3 da REQ, por decisão minha, e a razão fica escrita.** O AC3 original proíbe
*"inferir de aspas apenas"*. Mas foi medido que a **frase canônica do #258** — a própria descrição do
defeito — passa **exclusivamente** pela zona de aspas: retiradas as aspas, **não sobra sinal nenhum** na
frase. Manter o AC estrito significaria deixar o gate acusando o texto que documenta o seu próprio bug.

**Ações:**
1. Ampliar a zona não-assertiva: cerca, code span, **bloco indentado por 4 espaços/tab**, blockquote,
   linha de tabela **e span entre aspas** (retas **e** curvas — `"…"` e `“…”`).
2. 🔴 **Corrigir `docs/cli-parity.md:7318` neste mesmo commit.** Ele afirma que *"a forma errada **citada
   como exemplo** não reprova"* — **medido como falso** hoje para aspas, blockquote, tabela e bloco
   indentado. O contrato e o comportamento passam a concordar **no mesmo commit**: contrato que mente é
   pior que contrato ausente, e um commit depois é uma janela em que alguém lê a mentira.

**Critérios de aceite:**
- [ ] A frase do **#258** (`O corpo da minha PR #247 dizia "Fecha #246" …`) **não** é acusada — nas duas
      grafias de aspas
- [ ] As 5 zonas falsificadas individualmente: cada uma suprime, e **cada uma** é exercitada
- [ ] 🔴 A isenção inglesa **continua valendo** dentro das zonas novas — o `ML-N1` entregou dois passes;
      este ML não pode reintroduzir o passe único por outro caminho
- [ ] `docs/cli-parity.md:7318` corrigido **neste commit**, e a afirmação nova é a medida
- [ ] Corpus A: 0 FP mantido, cobertura do `ML-N1` mantida
- [ ] `make quality` verde

### ML-N3 — **AC1 + `GH_TOKEN` juntos, e por ÚLTIMO** — a chave de ativação
**Owner:** `ares-tf`
**Status:** ⬜ Pendente · **depende de `ML-N1` e `ML-N2` MERGEADOS**

🔴 **Leia isto antes de qualquer coisa: não antecipe este ML, e não separe o `GH_TOKEN` do AC1.**

Parece um item de infra trivial (*"adicionar `env:` ao job"*), e é o oposto: **é a chave que liga tudo o
que está desligado.** Medições:

| medição | resultado |
|---|---|
| `grep -c 'GH_TOKEN' .github/workflows/quality.yml` | **0** |
| o job do gate (`quality.yml:58-60`) | `run:` **sem bloco `env:`** |
| `gh pr view 293 --json state` | **`MERGED`**, com `**Não fecha #290**` e `**Não fecha #275**` no corpo |

Sem o token, o gate lê o **payload do evento** — obsoleto — e por isso o #293 passou verde: a negação
entrou por **edição posterior**. Com o token, ele passa a ler o **corpo atual**. 🔴 **Logo: pôr o
`GH_TOKEN` antes do fix da forma 4 red-lina todo PR que declara escopo negativo — incluindo o PR desta
própria REQ**, que vai declarar escopo negativo, como o #424 já declarou (`- **Não fecha a #421**`) e foi
pego pelo próprio gate.

**Ações:**
1. `GH_TOKEN` no job do gate em `.github/workflows/quality.yml`, com **escopo mínimo e declarado em
   comentário no YAML**: o job lê corpo de PR, nada mais. Ampliar superfície de token exige razão escrita;
   esta é a razão.
2. Reavaliação no evento `edited` — **sem repetir as suítes** (era o AC2). Um `edited` que dispare o
   `quality` inteiro faz o custo pagar por cada correção de typo em descrição de PR, e o resultado
   previsível é alguém desligar o gate.
3. Falsificação nas duas direções do `edited`: PR aberto sem palavra-chave **e** PR que **ganha** a
   palavra-chave por edição (o caso do #293, invertido).
4. **Guarda de vacuidade** (era o AC5): o gate nunca sai 0 por não ter conseguido ler o corpo — apenas por
   ter lido e não achado. É a mesma classe do `guarda-aprova-quando-nao-conseguiu-ler-o-comando`.

**Critérios de aceite:**
- [ ] `GH_TOKEN` presente, com escopo mínimo **comentado no YAML**
- [ ] Reavaliação no `edited` **sem** disparar as suítes de `quality`
- [ ] 🔴 O **#293** é reavaliado com o corpo **atual** e sai **verde** — prova de que o `ML-N1` chegou antes
- [ ] Falsificação das duas direções do `edited`
- [ ] Vacuidade: "não consegui ler" ≠ "não achei", com rc distintos
- [ ] `make quality` e **CI** verdes

### ML-N4 — Decisão sobre `required_status_checks`
**Owner:** `trackfw_architect` (eu)
**Status:** ⬜ Pendente · **depende de `ML-N3` com 0 FP medido**

O parecer registrou que este job **não está em `required_status_checks`**. 🔴 **Enquanto não estiver,
tudo isto conserta um conselho que ninguém é obrigado a ler** — e a cobertura de 1/4 medida hoje é a
consequência disso.

**Decisão declarada agora, para não ficar implícita:** o job **entra** em `required_status_checks`, e
**só depois** de 0 FP medido no corpus A com o `ML-N3` no ar. A ordem importa — tornar obrigatório um
gate com precisão de 1 em 3 bloquearia merges legítimos e produziria pressão para desligá-lo.

**Critérios de aceite:**
- [ ] `make check-required-full` executado, com a concordância D/R/W verificada
- [ ] O job declarado nos três lugares (D, R, W), sem divergência
- [ ] Registrado no roadmap **quando** entrou e com qual medição de FP

