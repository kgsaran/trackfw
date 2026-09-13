---
status: blocked
date: 2026-09-11
req: "docs/requisições/claude/REQ-2026-09-11-o-gitattributes-do-fork-mascara-um-defeito-de-produto-que-o-upstream-mantem-exposto-de-proposito.md"
squad: ""
---

# Roadmap: o .gitattributes do fork mascara um defeito de produto que o upstream mantém exposto de propósito

> Created: 2026-09-11 | Status: blocked

## Context

REQ: docs/requisições/claude/REQ-2026-09-11-o-gitattributes-do-fork-mascara-um-defeito-de-produto-que-o-upstream-mantem-exposto-de-proposito.md

A REQ decide o que o `.gitattributes` deste fork cobre — **depois** de medir. Nasce do ML-0A da
`REQ-2026-09-10-baselines-de-suite-nunca-foram-versionados-e-doze-acs-ficaram-inauditaveis-por-construcao`, que achou a causa dos 8 "sumidos": o nosso `.gitattributes` mascara, no checkout
Windows, um defeito de produto que o upstream mantém exposto de propósito.

## Acceptance Criteria
- [ ] Os seis ACs da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Medição, antes de qualquer decisão

### ML-0A — O que muda no checkout, por arquivo (AC1)
**Status:** ✅ Concluído
**Files affected:** —
**Acceptance criteria:**
- [ ] Para as opções A, B e C, a lista de arquivos rastreados que mudam de fim de linha no checkout
      Windows, por nome e agrupada por área
- [ ] Medido em worktree próprio, nunca na árvore de trabalho: a medição não pode ser contaminada por
      edição em curso

### ML-0B — O efeito nos nossos gates (AC2)
**Status:** ✅ Concluído
**Files affected:** —
**Acceptance criteria:**
- [ ] Cada gate local e o `validate` rodados sobre docs em CRLF, com controle em LF
- [ ] Separado, gate por gate: quebra, reprova pelo motivo errado, ou passa

### Medição do ML-0A (AC1) — 2026-09-11

Derivada por `git check-attr eol` sobre os **1211 arquivos rastreados**, em worktree próprio, com o
`.gitattributes` de cada opção no lugar. Nunca na árvore de trabalho.

| opção | `eol=lf` | `eol=unspecified` | arquivos que mudam vs. hoje |
|---|---|---|---|
| **B** — o nosso, hoje | 1193 | 18 | — |
| **A** — o do upstream | 667 | 544 | 526 |
| **C** — upstream + `docs/**` e `vault/**` | 847 | 364 | 346 |

🔴 **O dado que decide o AC1:** as **238 entradas que o produto parseia** — `*/integrations/assets/**`,
`internal/integrations/testdata/*.golden.*` e `scripts/testdata/roadmap-barrier-corpus-snapshot/**` —
ficam **todas `unspecified`** sob a opção C. Ou seja, C devolve a elas exatamente o comportamento do
upstream, e ainda mantém a nossa governança em LF.

**A divergência, derivada por conjunto e não a olho:** 24 regras existem só no nosso arquivo e
**zero** só no dele. Entre as nossas está `*.md text eol=lf`, que contradiz o bloco herdado logo
abaixo — aquele que declara, por escrito, que *"não existe regra `*.md` nesta raiz, e isso é
intencional"*. Como regra posterior vence e o bloco dele não tem `*.md`, a nossa linha de 2026-08-16
permanece em vigor.

**Prova por efeito, em arquivo de PRODUTO e não só na governança:**
`git check-attr` devolve `eol: lf` para `internal/integrations/assets/claude/commands/trackfw/req.md`
— exatamente o tipo de arquivo que o upstream mantém fora de propósito.

### Correção de número — são 9, não 8

A REQ e o ML-2A falam em "os 8". Medido em 2026-09-11 nos runs `34660260112` e `34659208572` da nossa
`main`, o conjunto é de **9 entradas de 38**, e é **idêntico por nome nos dois runs**:

| classe | entradas |
|---|---|
| identificador Go | `TestRenderOpenCodeAgent_CRLFSourceMatchesLF`, `TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF`, `TestRenderWithoutIdentityMatchesFrozenGoldens`, `TestResolveAgentModelMatchesRender` |
| identificador Python | `test_barrier.py::test_barrier_cli_crlf_roadmap_gates_da_wave_e_reconhecido_e_comando_roda_e2e` |
| descrição Node (`class: assertion`) | `renderers produce native deterministic formats`, `opencode-agent renderer: source CRLF renderiza byte-idêntico ao source LF (ADR CRLF)`, `gemini e kiro (mesma representação agent-markdown do cursor) permanecem bit-a-bit inalterados`, `sem identidade — saída idêntica ao comportamento pré-existente (não-regressão)` |

As quatro últimas **não são ruído**: a lista do ratchet nomeia entradas de Node por descrição de
asserção, e as quatro constam do `.github/windows-known-failures.json` com `runtime: node`.

🔴 O **8** veio do ML-0A da `REQ-2026-09-10-baselines...`, medido em 2026-09-10 contra outro run, e
**nunca foi listado por nome** — então não dá para dizer qual seria a nona. Dá para dizer qual é o
conjunto de hoje, e que ele se repete. O número é corrigido nomeando as nove, não trocando o dígito.

### Medição parcial do ML-0B (AC2) — 2026-09-11

**Declarada incompleta de propósito.** Sob a opção A, com `core.autocrlf=true` e **110 de 180**
arquivos de `docs/` em CRLF no working copy:

| gate | veredito |
|---|---|
| `check-req-layout` | rc=0 — 70 REQs, 0 fora do layout |
| `check-referential-integrity` | rc=0 |
| `check-inherited-req` | rc=0 |
| `check-req-done-com-criterio-aberto` | rc=0 |
| `trackfw validate` | ✓ sem violações |

🔴 **Os outros 6 gates não foram medidos na primeira tentativa, e isso quase virou conclusão falsa.**
O agregador abortou na guarda de runtimes (`bin/trackfw` e `npm/bin/trackfw` ausentes no worktree) e
**saiu com código 0**: lido pelo código de saída, pareceria "10 gates passam com docs em CRLF", a
partir de uma execução que não rodou gate nenhum. Fica registrado como *não medido*, nunca como
*passou*.

### Medição final do ML-0B (AC2) — 2026-09-11

🔴 **A primeira medição foi retratada.** Ela dizia "110 de 180 docs em CRLF"; o laço que produziu esse
número lia os caminhos citados pelo `git ls-files` e errava com acento. Refeito com `git ls-files -z`,
guarda de vacuidade (confirma que os 180 foram abertos) e recusa de medir se a fase CRLF não montar:

```
antes (.gitattributes nosso)            2 de 180 com CR   (demo.gif e .trackfw-log)
fase CRLF (.gitattributes do upstream) 180 de 180 com CR
```

Com **180 de 180** comprovados por nome, sobre a opção A:

| gate | veredito |
|---|---|
| `check-req-layout` · `check-referential-integrity` · `check-inherited-req` | rc=0 |
| `check-req-done-com-criterio-aberto` · `check-upstream-content` · `check-slug-inventory` | rc=0 |
| `check-upstream-sync-falsify` · `check-os-predicate-classification` · `measure-os-predicate-sites` | rc=0 |
| `trackfw validate` | ✓ sem violações |

**O custo de alinhar, que a REQ dizia não estar medido, é ZERO nos nossos gates.**

O décimo gate, `check-subcommand-parity`, saiu `exit 1` mudo no worktree — e isso **não é veredito
sobre CRLF**: falta `node_modules` ali, e o próprio agregador documenta a assinatura desde 2026-09-10
(*"um gate que morre calado é indistinguível de um gate que reprovou"*). Na árvore principal o CLI
Node responde normalmente.

## Wave 1 — A decisão (AC3)

### ML-1A — ADR
**Status:** ✅ Concluído
**Files affected:** `docs/adr/`
**Acceptance criteria:**
- [ ] ADR nova, ou emenda à `ADR-2026-08-29`, com as opções e o motivo medido

## Wave 2 — Aplicar e provar (AC4 a AC6)

### ML-2A — O `.gitattributes`, e as 9 no CI
**Status:** ✅ Concluído no que dependia de nós — 8 de 9 desmascaradas; a nona é imune por construção (ver o veredito do AC4 na REQ)
**Files affected:** `.gitattributes`
**Acceptance criteria:**
- [ ] Os 8 falham no nosso `windows-full-suites` (38 de 38 observados) — ou ficam declarados por nome,
      se a decisão for manter
- [ ] Nenhum arquivo de produto tocado; `validate` e gates verdes

## Residual declarado

- O defeito de produto — o parser de frontmatter e o CRLF — continua sendo do upstream. Esta REQ decide
  só se o fork o **vê**.

## Bloqueio — 2026-09-13

Movido para `blocked/` porque o único critério em aberto, o AC4, depende de
[kgsaran/trackfw#329](https://github.com/kgsaran/trackfw/issues/329) — e o mantenedor classificou
esse issue, em 2026-09-12, como *"deixa de ser possível"* na v8, que remove a suíte Python.
Todos os MLs daqui estão concluídos. O bloqueio é externo, não pendência nossa: sai daqui quando a
#329 fechar ou quando a Wave 3 da v8 remover a suíte, e aí o AC4 recebe o veredito `(c) caducou`.
