---
status: Open
date: 2026-08-16
author: "Zeus (Arquiteto)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md"
---

# REQ: Higiene — débitos acumulados na entrega de remoção de plugins e na release 7.0.0

> Date: 2026-08-16 | Status: Open (backlog, sem roadmap)
| Linear Issue:
| Jira Issue:

## Motivation

REQ **agregadora**, criada a pedido de KG para tirar da memória os débitos levantados durante a
entrega de remoção do subsistema de plugins (#176) e da release 7.0.0 (#179).

**Nenhum destes bloqueou a entrega**, e nenhum é resíduo de trabalho mal feito: são divergências
pré-existentes descobertas por acidente, ou efeitos colaterais de features recentes que só
apareceram ao exercitar o fluxo de ponta a ponta. Cada item já tem nota de vault ou registro em
roadmap com causa raiz e correção sugerida — esta REQ apenas os reúne num lugar priorizável.

**Critério para estar aqui:** ninguém deveria precisar reler roadmaps fechados para redescobrir
estes pontos.

## Itens

### 1 — `git-branch-guard`: falso-positivo por prosa (severidade: média)

Linha de **mensagem de commit** que **começa** com `git <subcomando>` é interpretada como comando e
**bloqueia o commit**. O matcher remove o espaço à esquerda e testa se o primeiro token é `git`; o
comentário do script afirma tratar prosa, mas só cobre menção no meio da frase.

Não é falha de segurança (erra para o lado restritivo), mas é falso-positivo em caminho quente: a
mensagem de erro cita um comando que a pessoa não executou. Contorno conhecido: não iniciar a linha
com `git`. Correção sugerida: descartar o conteúdo de `-m`/`--message` e de heredocs antes de
segmentar.

📎 `vault/notes/git-branch-guard-falso-positivo-em-linha-de-mensagem-de-commit-2026-08-16.md`
> Descoberto ao escrever a mensagem de commit que documentava este próprio problema.

### 2 — `git-branch-guard`: brecha de contorno (severidade: média)

O guard cobre uma das formas de criar branch, mas **não a alternativa equivalente**. Quem conhecer a
segunda forma contorna o bloqueio inteiro. Registrado durante a entrega e deliberadamente **não**
explorado.

### 3 — `ship`: divergência de mensagem e de stream entre CLIs (severidade: média)

A mensagem de violação de `checkShipGovernance` difere entre Go (`"...wip/ nor done/..."`) e
Node/Python (`"...wip/..."`). Além disso, `ship.go` nunca seta `SilenceErrors`, então erros do
passo 1 saem em **stream e prefixo diferentes** (`Error:` em stderr no Go; `error:` em stdout no
Node/Python). Pré-existente — não introduzido pelo #178.

📎 `vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md`

### 4 — ADR desatualizado após o #178 (severidade: baixa)

`docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md:58` ainda descreve o passo 1 do `ship`
com o vocabulário antigo (`feat|fix|refactor`), sem `chore`/`docs`. O ADR aceito não foi reescrito,
seguindo o precedente do #177. Correção sugerida: **emenda** ao ADR, não reescrita.

### 5 — i18n: `errors.notFound` divergente entre os 3 locales (severidade: baixa)

A chave existe em Node e Python e **não** no Go. **Pré-existente**, comprovado por
`git show main:internal/i18n/locales/en-US.json` durante a entrega — o bloco `errors` do Go já
continha apenas a chave de plugins, hoje removida. Decidir: alinhar os três, ou remover se estiver
órfã nos três.

### 6 — Deriva de documentação em `site/` (severidade: baixa)

`site/guide/commands.md` e `site/en/guide/commands.md` documentam `trackfw plugins`, que não existe
mais, e também estão atrás de `changelog` e `commit`. É **deriva anterior** a esta entrega
(último toque em PR #136), não resíduo da remoção — foi por isso que ficou fora do escopo dela.

### 7 — `trackfw` sem argumento diverge entre Go e Node (severidade: baixa)

Go sai com **exit 0** e help em **stdout**; Node sai com **exit 1** e help em **stderr**. É default
do commander quando o comando raiz não tem `.action()`. **Pré-existente** e sem relação com plugins —
preservado deliberadamente durante a entrega, para não esconder mudança de comportamento não
relacionada dentro de um commit de deleção.

## Acceptance Criteria

- [ ] Cada item acima é resolvido **ou** explicitamente declarado como não-será-corrigido, com o
      motivo registrado.
- [ ] Itens 3, 5 e 7 (divergências entre CLIs) ganham cobertura no contrato de paridade, para não
      reaparecerem.
- [ ] Item 1 tem cenário de falsificação, conforme **P4** do `ADR-2026-07-26-principios-de-design-de-gates-verificaveis`.
- [ ] `make quality` verde.

## Escopo negativo

- **Não** reabre nenhuma decisão da remoção de plugins nem do gate de artefato markdown.
- **Não** transforma isto num refactor de i18n ou de documentação do `site/` — cada item é pontual.
- **Não** é bloqueante para nenhuma release; pode ser fatiado e priorizado item a item.

## Linked ADR

ADR: (não requerido — nenhum item exige decisão arquitetural nova; o item 4 pede **emenda** a um ADR
existente)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md`
