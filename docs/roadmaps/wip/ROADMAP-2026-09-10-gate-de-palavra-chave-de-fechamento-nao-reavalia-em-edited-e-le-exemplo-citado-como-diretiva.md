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
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — **AC1** — O gate reavalia no evento edited, não só na abertura.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — O gate reavalia no evento edited, não só na abertura.
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **Sem repetir as suítes.** Um edited que dispare o quality inteiro faz o custo
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **Sem repetir as suítes.** Um edited que dispare o quality inteiro faz o custo
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Contrato próprio para "exemplo citado", não heurística de aspas.** A auditoria é
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Contrato próprio para "exemplo citado", não heurística de aspas.** A auditoria é
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — Falsificação nas duas direções para o edited: PR aberto sem palavra-chave e
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — Falsificação nas duas direções para o edited: PR aberto sem palavra-chave e
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
- [ ] build passes
- [ ] tests green

### ML-NOVO — A adjacência quebra com markdown entre a palavra e o `#N`

**Status:** ⬜ Pendente · **medido pelo arquiteto em 2026-09-10, contra o próprio PR #312**

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

