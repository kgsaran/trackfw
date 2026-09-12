---
status: Done
date: 2026-09-02
author: "kgsaran"
adr: ""
roadmap: ""
---

# REQ: PRs usam palavra-chave de fechamento em português, e nenhuma issue fecha automaticamente

> Date: 2026-09-02 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

O PR #247 abre com **"Fecha #246."** na primeira linha do corpo. Ele foi mergeado em 2026-09-02 e a
issue #246 **continuou aberta** — fechada à mão depois, quando alguém percebeu.

O GitHub só reconhece palavras-chave de fechamento em **inglês**: `close`/`closes`/`closed`,
`fix`/`fixes`/`fixed`, `resolve`/`resolves`/`resolved`. `Fecha`, `Corrige` e `Resolve` (esta última
é homógrafa, mas o GitHub casa a forma inglesa `resolves`, não a portuguesa `resolve` isolada) não
disparam nada.

Como escrevemos **todos** os PRs em português, **nenhum PR deste repositório jamais fechou uma issue
automaticamente**. Todas foram fechadas manualmente, ou ficaram abertas até alguém reparar.

### Por que isso é pior do que parece

O modo de falha é **silencioso e invertido**: você escreve a intenção de fechar, o PR mergeia com
sucesso, e nada acontece. Não há erro, não há aviso, e o texto do PR **afirma** que a issue foi
fechada. Quem lê o histórico depois acredita no texto.

É o mesmo padrão que dominou a auditoria de backlog desta sprint — **artefato que se reporta
saudável estando inerte**: gate vácuo, contrato de paridade documentando estado inexistente, índice
de vault dizendo "NÃO CORRIGIDO" sobre algo corrigido. Aqui é o corpo do PR dizendo "Fecha #246"
sobre uma issue que segue aberta.

### Não existe template de PR neste repositório

Verificado: não há `.github/PULL_REQUEST_TEMPLATE.md` nem `.github/pull_request_template.md`. Cada
corpo de PR é escrito do zero, em português — o que explica por que a forma correta nunca se firmou
por hábito.

## Acceptance Criteria

- [ ] **AC1** — Existe `.github/PULL_REQUEST_TEMPLATE.md` com uma linha de fechamento em inglês
      pronta para preencher (ex.: `Closes #`), e uma nota curta explicando **por que** tem de ser em
      inglês. A explicação importa mais que a linha: sem ela, o próximo tradutor "conserta" para
      português.
- [ ] **AC2** — 🔴 Gate no CI, em `pull_request`, que **reprova** quando o corpo do PR referencia uma
      issue com palavra-chave em português (`Fecha #`, `Fechado #`, `Corrige #`, `Corrigido #`,
      `Resolve #`, `Encerra #`) **e não** há uma forma inglesa válida para a mesma issue.
- [ ] **AC3** — 🔴 **Falsificação nas duas direções, por execução:** corpo com `Fecha #123` →
      **reprova nomeando a linha**; corpo com `Closes #123` → **passa**. Um gate que passa nos dois
      casos não mede nada.
- [ ] **AC4** — 🔴 **Controle de falso positivo:** corpo que menciona `#123` em **prosa**, sem
      intenção de fechar (ex.: *"o mesmo sítio do #238"*, *"portado do #223"*), **não** pode
      reprovar. Este repositório referencia issues e PRs em prosa o tempo todo — um gate ruidoso
      demais é desligado, e aí não guarda nada.
- [ ] **AC5** — Guarda de vacuidade: se o gate não conseguir **ler** o corpo do PR (evento sem
      payload, execução fora de `pull_request`, corpo vazio), ele **falha** ou reporta
      `not_evaluated` — nunca passa em silêncio.
- [ ] **AC6** — O gate está ligado no workflow e **verificado por execução**, não presumido. Se
      entrar em `required_status_checks`, isso é decisão explícita registrada, não efeito colateral.

## Negative Scope

- 🔴 **Não é feature de produto.** Escopo local a este repositório: nada em `internal/`, `npm/src/`
  ou `pypi/trackfw/`, e o `trackfw init` **não** passa a gerar template de PR em projeto adotante.
  Decidido explicitamente com o KG em 2026-09-02. Se virar produto depois, é REQ própria e cai sob a
  regra dura de paridade dos 3 CLIs.
- **Não** reescrever PRs já mergeados nem reabrir issues fechadas à mão. O passado fica como está.
- **Não** proibir português no corpo do PR. A exigência é **só** sobre a palavra-chave de fechamento
  — que é sintaxe do GitHub, não prosa.
- **Não** adicionar o gate a `required_status_checks` sem decisão explícita. Um gate novo em
  obrigatório bloqueia todo PR se nascer com defeito, e hoje temos 4 gates recém-criados medidos como
  vácuos.

## Linked ADR
<!-- Convenção de processo do repositório; sem decisão de arquitetura de produto. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Roadmap a criar quando a branch do `context` do CLI Node fechar — não despachar dois agentes
     sobre a mesma árvore. -->
Roadmap:

---

## Ampliação decidida pelo KG — 2026-09-12: a REQ carrega o issue, e o gate cruza

> *"não seria interessante termos na REQ a referência ao issue, quando essa nascer de um issue, e
> termos um gate validando quando uma REQ for para done sem a grafia no PR?"*

### O caso que o gate atual NÃO pega — e por quê

O gate de hoje resolve **grafia**: `Fecha **#320**`, `Corrigido #12`, ênfase markdown em três posições.
Verificado em 2026-09-12: **está coberto**, com contra-braço (outra palavra entre a chave e o `#N`
descaracteriza e vira prosa, sem reprovar).

🔴 **O que ele não pega é a AUSÊNCIA.** No PR #330 escrevi `Closes #315` na abertura e **nunca
acrescentei** `Closes #320` / `Closes #328` quando o trabalho entrou. Merge feito, dois issues abertos,
fechados à mão depois. **O gate não tinha como saber quais issues aquele PR deveria fechar** — essa
informação não existe em lugar algum legível por máquina.

**Medido:** **21 REQs** já citam issue em prosa (`Closes #`, `issue #N`). A informação **existe** e é
escrita rotineiramente — só não em campo estruturado.

### ⚠️ Correção de desenho: o gate NÃO pode disparar no `done`

O KG propôs validar *"quando uma REQ for para done"*. 🔴 **Tarde demais.** A REQ vai para `done`
**depois** do merge — e depois do merge não dá mais para acrescentar a palavra-chave ao corpo do PR.
Foi literalmente o que aconteceu no #330.

**O gate dispara no PR, antes do merge:**

```
roadmap em wip nesta branch  →  REQ vinculada tem issue: N  →  corpo do PR contem Closes #N
```

### Critérios acrescentados

- [ ] **AC-N1** — campo `issue:` no frontmatter da REQ, opcional, preenchido quando a REQ nasce de um
      issue. 🔴 **UMA fonte de verdade** — não repetir o número em prosa e em campo, como aconteceu com
      `roadmap:` (frontmatter) × marcador de corpo, que produziu contagens de **27 vs 57** órfãs.
      Escolher o campo e fazer o `validate` ler **só ele**.
- [ ] **AC-N2** — `trackfw req new --issue <N>` preenche o campo. Nos **3 CLIs**.
- [ ] **AC-N3** — gate no PR: para cada roadmap em `wip` da branch, se a REQ vinculada tem `issue: N`
      e o corpo do PR **não** contém palavra-chave válida com `#N`, **reprova nomeando o número**.
- [ ] **AC-N4** — 🔴 **contra-braço, e sem ele o gate vira atrito:** PR cuja REQ **não** tem `issue:`
      **passa**. É o caso normal de achado interno — o #332 (H-01/M-03) veio de auditoria própria e
      corretamente não fecha issue nenhum.
- [ ] **AC-N5** — 🔴 **segundo contra-braço:** issue **já fechado** por outro PR não reprova. Senão o
      gate bloqueia trabalho legítimo de continuação.
- [ ] **AC-N6** — migração dos **21** que já citam issue em prosa: preencher o campo. ⚠️ **Derivar a
      lista**, não presumir que são 21 — o número é de 2026-09-12 e a régua foi um `grep`.

### Por que aqui e não em REQ nova

O mecanismo **é outro** (grafia errada × ausência de declaração), e a `Regra Dura de Causa Raiz`
autorizaria REQ própria. **Mas o desfecho é o mesmo e esta REQ já é dele:** *"nenhuma issue fecha
automaticamente"*. Abrir outra dividiria a mesma promessa em duas filas — e o backlog já tem 92% das
REQs sem roadmap. A diferença de mecanismo fica escrita acima, que é o que a regra exige.
