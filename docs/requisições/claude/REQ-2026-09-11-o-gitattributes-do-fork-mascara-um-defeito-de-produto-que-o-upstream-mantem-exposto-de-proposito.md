---
status: Open
date: 2026-09-11
author: ""
adr: "docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md"
roadmap: "docs/roadmaps/claude/blocked/ROADMAP-2026-09-11-o-gitattributes-do-fork-mascara-um-defeito-de-produto-que-o-upstream-mantem-exposto-de-proposito.md"
---

# REQ: o .gitattributes do fork mascara um defeito de produto que o upstream mantém exposto de propósito

> Date: 2026-09-11 | Status: Open

## Motivation

O ML-0A da `REQ-2026-09-10-baselines-de-suite-nunca-foram-versionados-e-doze-acs-ficaram-inauditaveis-por-construcao` mediu por que **8 dos 38** nomes da lista de falhas conhecidas do upstream não
falham no nosso CI. Não é diferença de runner, nem lista errada: é o **nosso** `.gitattributes`.

### O mecanismo, medido nas duas direções

Mesma máquina, mesmo git, mesmo `core.autocrlf=true`, dois worktrees:

| worktree | arquivos rastreados com `\r` (integrations + npm) | 4 testes Go | 4 testes Node |
|---|---|---|---|
| nosso, `9e7044e` | 0 de 172 | 4 PASS | 4 ok |
| do upstream, `e4d8349` | 65 de 172 — assets de agents e skills, nos dois runtimes | 4 FAIL | 4 not ok |

O nosso `.gitattributes` tem um bloco próprio, de 2026-08-16 (ML-4, `da4f439`, da
`REQ-2026-08-16-consistencias-template-saida-e-eol`), que força LF no checkout de `*.md`, `*.json`,
`*.toml`, `*.yml`, `*.html`, `*.css` e outros, com `* text=auto` por cima. O do upstream força LF só nos
**fontes** (`*.go`, `*.js`, `*.py`, `*.sh`…) e exclui o resto **de propósito, com medição escrita**: os
assets de integração e os goldens são entrada que o produto processa, e o parser de frontmatter não
lida com CRLF. Forçar LF ali esconderia o defeito em vez de curá-lo.

É exatamente o que o nosso bloco faz: no Windows, o checkout grava os assets em LF, e as **9 entradas**
da lista que expõem o defeito passam aqui — 5 identificadores de teste e 4 descrições de asserção do
Node. A lista nomeada está no roadmap, seção *"Correção de número — são 9, não 8"*.

🔴 Um remédio de uma linha (`internal/integrations/testdata/** text eol=lf`) foi falsificado no ML-0A:
os goldens ficaram LF e os 8 **continuaram falhando** — o renderizador lê os assets
(`//go:embed assets`), não só os goldens.

> **O "8" desta frase é o número daquela medição, de 2026-09-10, e fica como ela o escreveu.** Ele
> nunca foi listado por nome, então não dá para dizer qual entrada faltou. A medição de 2026-09-11,
> estável em dois runs (`34660260112` e `34659208572`), dá **9 de 38**, nomeadas no roadmap. Corrigir
> o número aqui seria reescrever o que a medição anterior afirmou.

### Por que isto é outra causa, e outra REQ

A REQ dos baselines trata de critério que depende de artefato não versionado. Esta trata de
**configuração local mascarando defeito de produto**. Corrigir uma não fecha a outra. Pela Regra de
Causa Raiz, REQ própria, com o mecanismo escrito.

### A premissa que a medição derruba

A `ADR-2026-08-29-adotar-upstream-como-base` classifica o `.gitattributes` como **local** —
"configuração deste repositório", ao lado do `trackfw.yaml`. A mesma ADR ainda o lista entre as
correções que seriam "candidatas naturais a contribuição para o upstream". A medição mostra as duas
coisas erradas: ele **não é inerte** para o produto (muda o resultado de 9 entradas da lista no
Windows), e a parte que mascara é justamente a que o upstream recusa por escrito.

### Onde o efeito existe, e onde não

- **Só em checkout Windows com `core.autocrlf=true`.** No Linux os blobs já são LF, e `eol=lf` não
  muda nada.
- Então ele afeta **os jobs Windows do nosso CI** (o `windows-full-suites` e os irmãos) e **esta
  máquina** — não o `ubuntu-latest`.

### O custo de alinhar, que ainda não foi medido

Alinhar o nosso `.gitattributes` ao do upstream desfaz o mascaramento, mas deixa os nossos `docs/*.md`
em CRLF no checkout Windows — e os nossos gates (`awk`, `grep`) e o `validate` leem esses arquivos.
**Qual deles quebra, e como, não foi medido.** É o que esta REQ mede antes de decidir.

## Acceptance Criteria

- [x] **AC1** — Medido **por arquivo**, no checkout Windows, o que muda sob cada opção: quais arquivos
      rastreados passam de LF a CRLF. Por nome e agrupado por área, nunca só por contagem.
- [x] **AC2** — Medido o efeito de docs em CRLF em **cada** gate local nosso e no `validate`, com
      controle em LF: o que quebra, o que reprova pelo motivo errado, e o que passa.
- [x] **AC3** — Decisão escrita em ADR — nova, ou emenda à `ADR-2026-08-29` — sobre o que o
      `.gitattributes` deste fork cobre, com as opções consideradas e o motivo medido. O arquivo deixa
      de ser tratado como configuração inerte.
- [ ] **AC4** — Depois da decisão, as **9 entradas** (nomeadas no roadmap) **falham** no nosso
      `windows-full-suites` como no CI do upstream (o ratchet passa a observar 38 de 38) — **ou**, se a
      decisão for manter o mascaramento, as 9 ficam declaradas por nome, com o motivo, na REQ dos
      baselines.
- [x] **AC5** — Nenhum arquivo de **produto** tocado (`internal`, `npm`, `pypi`, `cmd`, `.github`,
      `Makefile`), e o `.github/windows-known-failures.json` em particular. O `.gitattributes` é o
      objeto da decisão, e a ADR-2026-08-29 já o declara local.
- [x] **AC6** — `validate` e os gates locais verdes ao fim, com o binário da árvore reconstruído.

## Veredito do AC4 — 2026-09-11, run 34664542939 (PR #115)

**O ratchet, na íntegra:**

```
ML-2A/2B: 37 observed / 38 active / 0 removed — DESEQUILÍBRIO POR CLASSE.
Go 14/14, Node-assert 10/10, Node-load 1/1, Python 12/13 [-1 resolvido].
```

**8 das 9 entradas foram desmascaradas.** Falsificado nas duas direções, comparando por nome o job
`windows-full-suites` da PR contra o da `main`:

| | resultado |
|---|---|
| falhas Go na PR | 14 |
| falhas Go na `main` | 10 |
| novas na PR | **exatamente as 4 desmascaradas** |
| sumidas em relação à `main` | **nenhuma** |

🔴 **A nona é imune por construção, e isso não é falha da decisão — é o critério ter presumido causa
única.** O `test_barrier.py::test_barrier_cli_crlf_roadmap_gates_da_wave_e_reconhecido_e_comando_roda_e2e`
monta a própria fixture em memória (`content = "
".join([...])`) e faz **zero leituras de arquivo
do repositório**. Nenhum `.gitattributes`, nosso ou do upstream, pode fazê-lo falhar ou passar. A
ausência dele na corrida tem outra causa, e essa causa não é nossa.

**Por que o AC4 fica ABERTO.** Ele exige "as 9 falham, 38 de 38"; o medido é 8 de 9 e 37 de 38.
Marcá-lo seria reescrever o critério para caber no resultado — o que o
`check-req-done-com-criterio-aberto.sh` recusa como saída, por escrito. Aposentar o nome exigiria
`removal_note` no `.github/windows-known-failures.json`, que é **arquivo compartilhado** e está no
escopo negativo desta REQ.

**Duas saídas, e a escolha é do mantenedor deste repositório:**

1. **Issue no upstream** pela nona entrada — a lista dele espera 13 falhas de Python e observa 12, e a
   causa não é fim de linha. Fechado isso lá, o AC4 fecha aqui sozinho.
2. **Veredito `(c) caducou`** para o AC4, escrevendo que a premissa de causa única foi falsificada
   pela medição, e o alvo real eram 8 entradas.

## Negative Scope

- **Não** corrigir o parser de frontmatter que não lida com CRLF. É produto do upstream, e ele já
  sabe: o `.gitattributes` dele documenta o defeito. Achado novo sobre ele vira issue.
- **Não** editar o `.github/windows-known-failures.json`: é compartilhado, e a lista está certa.
- **Não** decidir antes de medir o AC2. A opção "alinhar" tem um custo nos nossos gates que ninguém
  mediu.

## Opções já visíveis

| opção | desfaz o mascaramento? | docs em CRLF no Windows? | divergência do `.gitattributes` |
|---|---|---|---|
| **A** — alinhar ao do upstream | sim | sim | nenhuma |
| **B** — manter o nosso | não | não | a de hoje |
| **C** — `eol=lf` só em `docs/**` e `vault/**`; o resto como no upstream | sim | não | só a governança |

A **C** parece a candidata natural — a nossa governança em LF, as entradas do produto como no upstream
—, mas é hipótese até o AC1 e o AC2 medirem.

## Linked ADR
ADR: docs/adr/ADR-2026-09-11-o-gitattributes-do-fork-alinha-ao-upstream-porque-mascarava-defeito-de-produto-sem-custo-medido.md

> A `ADR-2026-08-29` continua valendo e é **emendada** por esta: o `.gitattributes` deixa de ser
> classificado como configuração local e passa a upstream, pelo motivo medido.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/claude/blocked/ROADMAP-2026-09-11-o-gitattributes-do-fork-mascara-um-defeito-de-produto-que-o-upstream-mantem-exposto-de-proposito.md
