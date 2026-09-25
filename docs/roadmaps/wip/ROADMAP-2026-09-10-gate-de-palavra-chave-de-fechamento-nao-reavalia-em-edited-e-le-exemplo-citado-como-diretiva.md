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
**Status:** ✅ Concluído — auditado em 2026-09-25 · 🔴 **a forma 8 NÃO era defeito, e as zonas se partem
em dois baldes com comportamento OPOSTO**

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
- [x] `closingIssuesReferences` medido para **cerca** e para **code span**, separadamente
- [x] O veredito da forma 8 declarado: **é defeito** (dois passes) ou **é comportamento correto** (e então
      a tabela de §2.4 tem `esp` errado, e isso fica escrito)
- [x] 🔴 Se as duas zonas divergirem entre si, isso fica escrito — o fix não pode tratá-las juntas
- [x] Artefatos de teste (issue + PR) **fechados**, com os números citados
- [x] Nenhuma alteração em `scripts/check-pr-closing-keyword.sh` — este ML **mede**, não corrige

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

## 🔴 Resultado do ML-0C — a medição refutou o plano, e é por isso que ela veio antes

**Auditei as 7 sondas eu mesmo, por `gh pr view --json closingIssuesReferences`:**

| sonda | zona da keyword inglesa | GitHub fecha? |
|---|---|---|
| **#427** | prosa (**controle**) | **[426]** ✅ |
| #428 | cerca ``` | `[]` |
| #429 | code span `` ` `` | `[]` |
| #431 | bloco indentado 4 espaços | `[]` |
| #432 | blockquote | **[430]** |
| #433 | célula de tabela | **[430]** |
| #434 | aspas retas | **[430]** |

Issues **#426** e **#430** `closed`, as 7 sondas `closed`, `matching-refs/heads/probe` devolve **0**.
🔴 **O braço de controle é o que valida o instrumento** — sem o #427, `[]` seria indistinguível de
medição quebrada. Ele o incluiu sem que eu pedisse, e está certo.

**As zonas divergem — mas não no eixo que eu previ.** Cerca e code span **não** divergem entre si;
ambas `[]`. A divergência está em outro eixo, e é **oposta entre baldes**:

| balde | zonas | GitHub |
|---|---|---|
| **CÓDIGO** | cerca · code span · bloco indentado | **ignora** |
| **NÃO-CÓDIGO** | blockquote · tabela · aspas | **honra** |

### Duas consequências, e as duas corrigem o que eu escrevi

**1. A forma 8 sai da lista de defeitos: o gate de hoje ACERTA.** `Fecha #246.` em prosa com `Closes
#246` só dentro de cerca **não fecha a issue** — logo o `esp` daquela linha é **1**, não 0, e acusar é o
trabalho do gate. **8 famílias → 7 defeitos + 1 comportamento correto.** Executar o passo 1 do `ML-N1`
como eu o redigi instalaria **falso negativo**: silêncio sobre um corpo cuja única forma inglesa vive
numa zona que o GitHub ignora. 🔴 **É a direção oposta do defeito, e é pior — o incômodo de hoje é um
aviso verdadeiro.**

**2. O 3º critério de aceite do `ML-N2` estava ERRADO.** Eu listei `bloco indentado por 4 espaços/tab`
entre as "zonas novas" e exigi que *"a isenção inglesa continua valendo"* nelas. A sonda **#431** mede o
contrário. Cumprir aquele AC como escrito instalaria falso negativo — **um AC meu produzindo o defeito
que a REQ existe para remover.** Corrigido abaixo.

Ele também corrigiu, no parecer, a afirmação de §3.5-iv de que *"o GitHub fecha nos cinco casos"*: são
**3 de 5**, pelo mesmo mecanismo. E declarou uma precisão contra si mesmo: a sonda #431 é **consistente
com**, mas **não discrimina**, a linha `esp = 0` da forma 2 — porque keyword **portuguesa** não fecha
issue em zona nenhuma; aquele `esp` se sustenta no argumento semântico de §3.3, não na medição de §10.
🔴 **Isso é a Regra Dura de Reconciliação exercida sem ninguém cobrar.**

**Extensão de escopo declarada e aprovada:** pedi 2 braços, ele fez 6 + controle. Não é escopo crescido —
é a medição achando defeito no critério de aceite de um ML a jusante. Custo: 7 PRs rascunho, 13 runs
cancelados.

**Fronteira de escrita:** escreveu a nota de vault
`github-ignora-keyword-de-fechamento-em-zona-de-codigo-e-honra-em-zona-nao-codigo-2026-09-24.md` além do
parecer, e **declarou** em vez de resolver em silêncio. **Mantida** — é exatamente o caso que a regra
global exige (>10 min de outro agente amanhã), e minha fronteira estava estreita de novo.

**Residual declarado, e não extrapolado por analogia** — foi a analogia que produziu as duas presunções
refutadas: comentário HTML, `<pre>`/`<code>`, **aspas curvas** (só as retas foram medidas), cerca `~~~`,
cerca com atributo de linguagem, `<details>`, item de lista. 🔴 **Zona nova na gramática do gate exige
sonda própria.**

---

## Wave 2 — Implementação, em ORDEM TRAVADA
> Dependências: `ML-0B` ✅ · `ML-0C` (só a forma 8 do `ML-N1`)
> 🔴 **A ordem é consequência de medição, não de preferência.** Ver §"Três achados" e §"Veredito".
> Os três MLs são **estritamente sequenciais** — todos tocam
> `scripts/check-pr-closing-keyword.sh`. Não há paralelismo a extrair aqui.

**O gate de aceitação é o mesmo nos três, e é a re-execução contra o corpus A (**358** invocações — 358 PRs mergeados, 357 com corpo não vazio):**

```
baseline de hoje:  355 rc=0 · 2 rc=1 · 1 rc=2   |  precisão 1/3  ·  cobertura 1/4
alvo:              0 falso positivo  ·  as 4 declarações reais acusadas (#247, #312, #325, #330)
```

⚠️ **E a ressalva do parecer entra escrita no AC, não no rodapé:** *"0 falso positivo é propriedade
**deste corpus**, não do discriminante"* — a simetria PT↔EN sem `#` obrigatório, por exemplo, mediu **+6
FP**. Marcar `[x]` por um número que não transfere é o que a Regra Dura de Reconciliação proíbe.

### ML-N1 — Esqueleto de dois passes (por BALDE) + formas 3, 4, 5 e 6 — **um único commit**
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-25 · **precisão 1/3 → 4/4 · cobertura 1/4 → 4/4**

🔴 **Por que num commit só, e não um ML por forma.** Medido em §3.6: `Não fecha **#421**.` casa a forma 3
(markdown na lacuna) **e** a forma 4 (negação). Fechar só a forma 3 faz o gate **passar a ver** a frase e
**acusá-la** — cria falso positivo novo onde hoje há silêncio. A forma 3 isolada é uma **regressão**.

**A ordem interna também é travada, para não reescrever trabalho:** o esqueleto de **dois passes** vem
**primeiro**, e só então os discriminantes. Se os discriminantes vierem antes, a chegada do `ML-N2` (que
exige dois passes) reescreve o que este ML acabou de entregar.

**Ações, na ordem:**
1. **Dois passes de mascaramento — mas por BALDE, e os baldes têm comportamento oposto.** 🔴 **Reescrito
   pela medição do `ML-0C`; a redação anterior instalaria falso negativo.**

   | balde | zonas | máscara | por quê |
   |---|---|---|---|
   | **CÓDIGO** | cerca · code span · bloco indentado 4 espaços/tab | subtraída dos **DOIS** matchers, **como o `blank_code` de hoje já faz** | o GitHub **ignora** keyword aqui (#428/#429/#431 = `[]`) — a isenção inglesa é **falsa**, e suprimi-la está correto |
   | **NÃO-CÓDIGO** | blockquote · célula de tabela · span entre aspas | passe **próprio**, que suprime a acusação PT e **preserva** a isenção EN | o GitHub **honra** keyword aqui (#432/#433/#434) — a isenção é **real** |

   **A "forma 8" sai da lista de correções deste ML: não há nada a corrigir ali.** O que entra é a
   separação em baldes, que é pré-requisito do `ML-N2`.
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
- [x] Corpus A: **0 falso positivo** — em particular o **#293** (`Não fecha #290` / `Não fecha #275`)
      deixa de ser acusado
- [x] Corpus A: **#312**, **#325** e **#330** passam a ser acusados (hoje são falsos negativos medidos)
- [x] `Fecha o **item 4** da issue #216` continua **não** acusado — contra-braço da forma 3
- [x] `Fecha 2 dos 3 elos` continua **não** acusado — o `#` obrigatório
- [x] Autoteste do gate cobre as 4 formas (3, 4, 5, 6), **nas duas direções**
- [x] 🔴 Falsificação **por balde**: um corpo com `Fecha #N` em prosa e `Closes #N` só em **cerca**
      continua **acusado** (é aviso verdadeiro — #428); o mesmo corpo com `Closes #N` em **blockquote**
      **não** é acusado (#432). Os dois braços, ou o balde não está implementado
- [x] 🔴 Toda supressão por polaridade **aparece no log**, com a frase e o token
- [ ] 🔴 **Invariante I3 do threat model:** toda supressão declarada **e falsificada nos dois matchers**
      (PT e EN) — não só no português
- [x] Reconciliação: **uma frase por teste novo**, dizendo qual conclusão deste ML ele afirma
- [x] `make quality` verde · `go build ./...` verde


#### 🔴 Auditoria do ML-N1 — medido por mim, não aceito do relatório

Reconstruí o corpus (`gh pr list --state merged --limit 500`) e invoquei o gate pelo caminho real,
um corpo por invocação, `$?` sem cano antes. **`git show HEAD:scripts/...` para o baseline**, árvore
atual para o depois:

| | antes (HEAD) | depois |
|---|---|---|
| histograma | `rc=0 355 · rc=1 2 · rc=2 1` | `rc=0 353 · rc=1 4 · rc=2 1` |
| acusados | **#247** · **#293** (PR **mergeado** — FP) | **#247 · #312 · #325 · #330** — e mais ninguém |
| precisão · cobertura | **1/3 · 1/4** | **4/4 · 4/4** |

**12 contra-braços, 12 verdes**, incluindo os dois braços do balde no mesmo par de corpos:
`Closes #246` em **cerca** → `rc=1` (aviso **verdadeiro**, #428=`[]`) · em **blockquote** → `rc=0`
(isenção **real**, #432=`[430]`) · em **indentado** → `rc=1` (#431=`[]`).

**A supressão por polaridade aparece no log nos DOIS caminhos de saída** (verificado em `rc=0` e em
`rc=1`), nomeia o token que suprimiu e fecha com *"se alguma destas frases AFIRMA fechamento, o gate
errou aqui e a issue #N vai continuar aberta após o merge"*. `--self-test`: **61 OK, 0 FAIL** (eram 26).

**Refutação que ele trouxe e que eu ratifico — a forma 5 tinha uma fronteira que o parecer não viu.**
O parecer mediu 3 conjugações nomeadas e concluiu *"4 → 4, nenhuma prosa passa a ser acusada"*. Ele
mediu o **paradigma inteiro** e achou o caso que quebra:

```
#233 L34: Sei que você fechou as #222–#225 por conflito de governança com um ciclo já em curso
```

🔴 **Pretérito perfeito é o tempo de RELATAR ação passada — de terceiro, inclusive — não o de
DECLARAR o fechamento que este PR vai fazer.** Ficou fora, com o caso escrito no gate. Não contradiz
a medição do parecer: amplia o alcance dela. Confirmei `rc=0` para essa frase.

**Corroboração por caminho próprio da ordem travada:** com as formas 3/5/6 e a polaridade
**desligada**, `Não fecha **#421**.` vira `rc=1` — o falso positivo novo que eu previra por
aritmética. **Agora é medição de primeira mão, não herdada.**

**Precisão de contagem, para não ser recitada errada:** medi `gh pr list` → **358 PRs, 357 com corpo
não vazio**, e o gate é invocado **358 vezes** (o #49 tem corpo vazio e sai `rc=2`). O parecer estava
certo em §0; o rótulo *"corpus A (**358** invocações — 358 PRs mergeados, 357 com corpo não vazio)"* que **eu** escrevi neste roadmap é que estava impreciso.

**Achado de instrumento, com nota de vault:** o cenário `s182` do `check-gates-falsify.sh` sabota o
gate por `sed` sobre a **linha literal** `if num not in english:`. Renomeá-la matou o `chunk_2` sob
`set -euo pipefail` e produziu **+10 rótulos "AUSENTE" sem defeito nenhum** — *parece que você quebrou
11 coisas e quebrou uma*. Ele restaurou a linha (a sabotagem tem de **representar a regressão**, não
perseguir o código) e declarou a dependência no ponto exato.
`vault/notes/cenario-de-falsificacao-fixa-linha-literal-do-gate-e-renomea-la-mata-o-chunk-2026-09-25.md`

⚠️ **Ressalva que vai junto com o número, não no rodapé:** *"0 falso positivo é propriedade **deste
corpus**, não do discriminante"*. Os 3 FP do candidato nas zonas não-código (§3.5-iv) não aparecem em
nenhum dos 358 corpos.

⚠️ **Dívida declarada:** a máscara de bloco indentado dele apaga **304** linhas do corpus, contra as
**80** do censo do parecer. **0** das 304 carrega declaração PT ou keyword EN com `#N`, e a cobertura
4/4 prova que não houve perda — mas ele **não sabe reconstruir a regra do censo** e **registrou que
não sabe**, em vez de supor. Fica para o `ML-N2` reconciliar as duas larguras.

**Decisão minha sobre o item que ele deixou aberto:** ele não tocou `docs/cli-parity.md` por instrução
minha (era do `ML-N2`) e **declarou** que a partir deste commit o contrato passaria a mentir sobre o
bloco indentado. 🔴 **Ele está certo e minha instrução estava errada** — a Regra Dura diz que contrato
e comportamento concordam **no mesmo commit**. Reescrevi eu mesmo o bloco `trackfw-contract` da seção,
que agora declara os **dois baldes com as sondas**, a polaridade como heurística declarada e a lista de
não-coberto. O `ML-N2` atualiza de novo quando acrescentar as zonas de não-código.

### ML-N2 — Forma 2 (exemplo citado) — AC3 **ampliado** para span entre aspas
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-25 · e a reconciliação de uma dívida **expôs um falso negativo vivo**

🔴 **Este ML altera o AC3 da REQ, por decisão minha, e a razão fica escrita.** O AC3 original proíbe
*"inferir de aspas apenas"*. Mas foi medido que a **frase canônica do #258** — a própria descrição do
defeito — passa **exclusivamente** pela zona de aspas: retiradas as aspas, **não sobra sinal nenhum** na
frase. Manter o AC estrito significaria deixar o gate acusando o texto que documenta o seu próprio bug.

**Ações:**
1. Ampliar a zona não-assertiva do **balde NÃO-CÓDIGO**: blockquote, linha de tabela **e span entre
   aspas** (retas **e** curvas — `"…"` e `“…”`).
   🔴 **`bloco indentado por 4 espaços/tab` NÃO entra aqui — corrigido pela medição do `ML-0C`.** A sonda
   **#431** devolveu `[]`: o GitHub **não** fecha dentro de bloco indentado, logo ele pertence ao balde de
   **CÓDIGO** (máscara única, dois matchers), entregue no `ML-N1`. A redação anterior deste ML exigia o
   oposto e instalaria falso negativo.
   ⚠️ **Aspas curvas não foram medidas** — só as retas (#434). Se o gate as tratar junto, isso é
   extrapolação por analogia, e foi a analogia que produziu as duas presunções refutadas. Ou mede, ou
   declara como residual.
2. ⚠️ **O bloco `trackfw-contract` do `cli-parity.md` JÁ foi reescrito no `ML-N1`** — o arquiteto o fez
   porque aquele commit tornou falsa a afirmação sobre bloco indentado. O que resta aqui é **atualizá-lo
   de novo** para declarar as 3 zonas de não-código como implementadas (hoje ele diz *"passe próprio,
   ainda NÃO implementado — é do ML-N2"*). Instrução original, mantida para registro:
   🔴 **Corrigir `docs/cli-parity.md` neste mesmo commit.** Ele afirma que *"a forma errada **citada
   como exemplo** não reprova"* — **medido como falso** hoje para aspas, blockquote, tabela e bloco
   indentado. O contrato e o comportamento passam a concordar **no mesmo commit**: contrato que mente é
   pior que contrato ausente, e um commit depois é uma janela em que alguém lê a mentira.

**Critérios de aceite:**
- [x] A frase do **#258** (`O corpo da minha PR #247 dizia "Fecha #246" …`) **não** é acusada
      → medido por mim: **antes `rc=1`, depois `rc=0`**. ⚠️ **AC corrigido por mim:** dizia *"nas duas
      grafias de aspas"*; as **curvas não foram medidas** contra a API, e a ação 2 do handoff substituiu a
      exigência por *"mede ou declara residual"*. Ficaram **residuais declaradas e presas por cenário** —
      o corpus A tem **zero** ocorrência delas. Exigir o que não se mediu é o que produz AC marcado por
      presunção
- [x] As **3** zonas do balde NÃO-CÓDIGO falsificadas individualmente, cada uma exercitada
- [x] 🔴 A isenção inglesa **continua valendo** dentro das 3 zonas novas — medido em #432/#433/#434 — e o
      `ML-N1` não pode ter sido desfeito: dentro do balde de **CÓDIGO** ela continua **suprimida** (#428/
      #429/#431). Os dois lados, ou o AC está marcado por metade da medição
- [x] `docs/cli-parity.md:7318` corrigido **neste commit**, e a afirmação nova é a medida
- [x] Corpus A: 0 FP mantido, cobertura do `ML-N1` mantida
- [x] `make quality` verde


#### 🔴 Auditoria do ML-N2 — e a dívida que eu mandei "reconciliar ou declarar inerte" era um defeito

Rodei o corpus inteiro e **14 contra-braços**, comparando `git show HEAD:scripts/...` (pós-`ML-N1`)
contra a árvore. **14/14 no esperado.**

**O achado, e ele é da mesma causa desta REQ, na direção do silêncio:**

```
$ printf 'Texto.\n\n`trackfw validate` — Fecha #246.\n' > /tmp/c.md
$ PR_BODY_FILE=/tmp/c.md bash <gate pos-ML-N1> ; echo $?
0        ← falso negativo VIVO
$ ... árvore atual
1
```

**Causa:** `_blank()` troca o trecho casado por **espaços**. Uma linha que *começa* com code span vira
4+ espaços à esquerda depois do `SPAN_RE.sub`, o `INDENT_RE` casa, o bloco abre, e a linha inteira é
apagada **com a declaração portuguesa dentro**. 🔴 **Indentação fabricada pelo passe anterior, não
escrita pelo autor.** Corrigido aqui, não empurrado para REQ nova — Regra Dura de Causa Raiz. A
indentação passa a ser decidida em `ref` (corpo como o autor escreveu) e apagada em `text`; a ordem
alternativa (`INDENT` antes de `FENCE`) foi considerada e **recusada**, porque apagaria os marcadores
de uma cerca indentada.

**A reconciliação numérica fechou exatamente — e a conclusão é melhor que a que eu pedi:**

| medida | valor |
|---|---|
| censo do parecer (corpo **cru**) | **80** = 54 dentro de cerca + 26 fora |
| dessas 26, **abrindo bloco** (linha anterior em branco) | **0** |
| medida do `ML-N1` | **304** — **100% fabricadas** |

🔴 **As duas contagens nunca foram da mesma coisa; não havia "regra mais larga" a descobrir.** E o
*"0 das 304 carrega declaração"* do `ML-N1` era propriedade do **corpus**, não da **regra** — a mesma
distinção que o parecer faz sobre o "0 FP", agora aplicada contra o próprio trabalho anterior.

**Costura fora do escopo declarado, medida e presa.** A zona de aspas apaga **dentro** da linha, logo
alcança a leitura de polaridade (forma 4). Ele exercitou 7 arranjos; **uma divergência**, e confirmei:

```
antes=0 depois=1   Ele disse "nao" e fecha a #246        ← negação CITADA
antes=0 depois=0   Nao fecha a #246                      ← negação do AUTOR
```

Direção **fail-closed** e correta — quem nega é a frase **citada**, não o autor — e de quebra
**estreita a superfície de silenciamento** que o parecer declarou não-fechável.

**Corpus A, e a prova é mais forte que o agregado:**

```
ANTES (pós-ML-N1): rc=0 353 · rc=1 4 · rc=2 1   acusados #247 #312 #325 #330
DEPOIS:            rc=0 353 · rc=1 4 · rc=2 1   acusados #247 #312 #325 #330
diff pr-a-pr → VAZIO
```

Ele ainda **ablacionou cada mudança em separado** (só-zonas, só-INDENT) sobre os 358 corpos — ambas
idênticas ao baseline, **sem cancelamento entre elas**. `--self-test`: **70 OK, 0 FAIL** (eram 61).

**Decisões que ele tomou bem, e registro para não serem revisitadas:**

- **Aspas curvas → residuais declaradas**, com medição (zero ocorrências no corpus A), **presas** pelo
  cenário `residual-aspas-curvas-nao-sao-zona`: se um dia entrarem, o autoteste fica **vermelho**.
- **Zona-sonda `@@PROBE@@` → removida por inteiro** — `PROBE_ZONE_RE`, a leitura de
  `PRCLOSE_SELFCHECK_NONCODE`, o helper, os 3 cenários, o comentário **e** o `import os` que ficou sem
  uso. *"Meia remoção deixaria comentário que mente."*
- **Largura do `TBL` medida em vez de admitida:** a forma estrita (`^| … |$`) apaga **exatamente as
  mesmas 1354** linhas que a larga — ficou a estrita, que não arrasta prosa que apenas comece com `|`.
- **`~~~` declarado como analogia herdada, não como cobertura.** O `FENCE_RE` o casa, mas a sonda mediu
  só ```` ``` ````. 🔴 **Registrar a analogia como analogia é exatamente o que a Wave 0 desta REQ teve de
  aprender duas vezes.**

**Nota de vault:** `mascarar-code-span-fabrica-indentacao-e-a-mascara-seguinte-engole-a-linha-2026-09-25.md`.

### ML-N3 — **AC1 + `GH_TOKEN` juntos, e por ÚLTIMO** — a chave de ativação
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-25 · **o #293 sai verde com o corpo vivo**

⚠️ **Medição minha, acrescentada no despacho:** `on: pull_request` (linha 5) está **sem `types:`**, logo
o default é `opened · synchronize · reopened` — **`edited` não está lá**, e é a causa mecânica do AC1.
E 🔴 **o comentário do job (`quality.yml:46-49`) MENTE**: afirma que o script *"lê o corpo do próprio
payload… não usa `gh`, não precisa de token"*, enquanto a linha **901** do gate chama `gh pr view` com
**fallback silencioso** ao payload (linha 915). O comentário é anterior ao #416 e não foi atualizado —
é o mesmo "contrato que mente" que o `ML-N1` já pagou, agora em YAML.

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
- [x] `GH_TOKEN` presente, com escopo mínimo **comentado no YAML**
- [x] Reavaliação no `edited` **sem** disparar as suítes de `quality`
- [x] 🔴 O **#293** é reavaliado com o corpo **atual** e sai **verde** — prova de que o `ML-N1` chegou antes
- [x] Falsificação das duas direções do `edited`
- [x] Vacuidade: "não consegui ler" ≠ "não achei", com rc distintos
- [x] `make quality` e **CI** verdes


#### 🔴 Auditoria do ML-N3 — medido por mim

| verificação | resultado |
|---|---|
| `--pr 293` com o corpo **vivo** | **`rc=0`** — e o log nomeia as duas supressões nas linhas 98 e 101 |
| corpus A | `rc=0 353 · rc=1 4 · rc=2 1` — **idêntico PR a PR** ao pós-`ML-N2` |
| `--self-test` | **78 OK, 0 FAIL** (eram 70) |
| `make quality` | **exit 0** — 338 OK, 0 FAIL, guarda de conjunto OK |
| `.github/required-status-checks.txt` | **intocado** — é o `ML-N4`, e é meu |

🔴 **O `rc=0` do #293 é a prova de que a ordem travada era necessária, não preferência.** Com o
`GH_TOKEN` ligado o gate lê o corpo atual; se este ML tivesse vindo antes do `ML-N1`, esse mesmo corpo
seria **acusado** — e todo PR com escopo negativo junto.

**Correção que ele fez no meu handoff, e é de grau mas importa:** eu escrevi *"os outros jobs não têm
esse `if:`"*. Medido: **não são "os outros", são TODOS** — 11 dos 14 jobs sem `if:` e os 2 restantes com
`if: always()`, que **rodam mais**, não menos. Não havia subconjunto a salvar com `if:`. Daí a decisão
de **workflow próprio** (`.github/workflows/pr-closing-keyword.yml`) estar certa: `on.pull_request.types`
é do **workflow**, não do job, e escrever um `if:` em cada job deixaria *"o próximo job novo sem ele — um
contrato que se quebra por omissão"*.

🔴 **Achado que o handoff não previa, e que é a mesma lição do `ML-N1` em outra roupa.** O gate dizia,
no caminho de degradação, *"payload do evento (**corpo de ABERTURA** do PR #N)"*. Isso era verdade
**só enquanto `edited` não estivesse no gatilho** — num payload de `edited` o `.pull_request.body` é o
corpo **já editado**. **Ele instalaria uma mentira nova no mesmo commit que corrige a de
`quality.yml:46-49`.** Trocado por uma afirmação verdadeira em todo evento: *"o payload é imutável e
reflete o corpo no instante daquele evento (ação: X)"*.

**Decisões dele que eu ratifico:**

- **`permissions:` no JOB com os dois escopos**, porque bloco de job **substitui** o do workflow e não
  soma — omitir `contents: read` quebraria o `actions/checkout`. Verifiquei: `{contents: read,
  pull-requests: read}`, e o `GH_TOKEN` só no step do gate.
- **Job id `pr-closing-keyword` e SEM chave `name:`** — o nome do check é o id, que é a string que o
  `ML-N4` vai listar. Verifiquei: `name` ausente.
- **`check-annotations.yml` passou a observar `["Quality", "PR Closing Keyword"]`** no mesmo commit —
  tirar o job do "Quality" sem isso perderia a verificação de anotação **em silêncio**, que é a classe
  exata de defeito que esta REQ existe para remover. Verifiquei na linha 23.
- **Degradação ≠ vacuidade, e é deliberado.** O payload é corpo **real**: o gate mede e dá veredito; o
  que se perde é a **precisão da fonte**, e isso é **anunciado** (causa, rc e stderr do `gh`, e
  `::warning::` sob `GITHUB_ACTIONS`). Vacuidade é **não ter corpo** → `exit 2`. 🔴 *"Fazer degradação
  reprovar transformaria 'sem token' em 'PR vermelho', e é assim que se desliga um gate."*
- **`--scope dw` rodado de verdade** (o `--self-test` do `make quality` usa fixtures e não parseia
  workflows): `declared=8, workflow_checks=45`, `D\W=∅`. O `ML-N4` herda D/W limpo.

⚠️ **Residual declarado:** o cumprimento do gatilho pelo GitHub **não é observável localmente** — a
asserção estática sobre o YAML é a medição honesta. Mas o veredito ao vivo chega **neste PR**, porque
workflow `pull_request` roda a partir do **head**. E `edited` dispara também em edição de **título** e
de base — é o único tipo que o GitHub oferece, declarado no YAML.

⚠️ **Fronteira:** ele encontrou na árvore dois arquivos meus de `.claude/agent-memory/zeus-tf/`,
escritos enquanto ele trabalhava, e **declarou em vez de varrer para dentro do commit dele**. Commitei-os
**separado**, antes deste. Era eu editando árvore com agente vivo — a regra é minha e fui eu que a tensionei.

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

