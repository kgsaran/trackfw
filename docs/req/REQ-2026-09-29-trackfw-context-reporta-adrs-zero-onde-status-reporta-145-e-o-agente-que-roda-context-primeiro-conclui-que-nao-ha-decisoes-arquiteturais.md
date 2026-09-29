---
status: Done
date: 2026-09-29
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-09-29-a-enumeracao-de-adr-e-uniao-de-layouts-e-context-e-status-consomem-o-mesmo-ponto-unico.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-29-trackfw-context-reporta-adrs-0-onde-status-reporta-145.md"
---

# REQ: `trackfw context` reporta `ADRs (0)` onde `status` reporta 145

> Date: 2026-09-29 | Status: Done
| Linear Issue:
| Jira Issue:

Origem: **#450**, consumidor externo, com reprodução e uma hipótese explicitamente marcada como
não confirmada — **confirmada pelo arquiteto** nesta REQ.

## Motivation

No mesmo repositório e versão, dois comandos discordam sobre o mesmo dado. Reproduzido em fixture:

```
ADRs no disco:  4
trackfw status:   ADRs 4        ← correto
trackfw context:  ## ADRs (0)   ← zero
```

Causa em `internal/generators/context.go:39`: `os.ReadDir(adrDir)` lê **apenas a raiz** de cada
`adr_dirs`, sem descer nas subpastas de estado onde os ADRs vivem.

### 🔴 A contradição está dentro da mesma saída

```
## ADRs (0)
- (none)
...
## Warnings (6)
- adr "ADR-2026-09-01-teste.md" is not referenced by any REQ
```

O mesmo comando, na mesma execução, **declara zero ADRs e nomeia um ADR**. Não é ambiguidade de
layout: são **duas implementações da mesma pergunta**, e uma está errada.

### Por que pesa mais que um número errado

O `context` é o comando que a documentação instalada manda o agente rodar **primeiro**
(*"always run first"*). Quem obedece recebe `ADRs: (none)` e conclui que **não há decisões
arquiteturais registradas** — quando há 145.

🔴 **É a pior forma de dado errado: a que induz confiança na ausência.** Um agente assim não viola
uma ADR conscientemente — ele nem sabe que ela existe. E este projeto tem uma diretriz que manda
*"inspecionar e respeitar todos os ADRs antes de propor alterações de arquitetura"*; o comando que
deveria servi-la é o que a nega.

### A hipótese do reportante sobre o score — CONFIRMADA e quantificada

Ele escreveu: *"provavelmente é calculado a partir da mesma leitura… não confirmei, é inferência"*.
Medido em `context.go:121`:

```go
if len(adrs) > 0 { score += 20 }
```

`adrs` vem do enumerador quebrado. O defeito custa **exatamente 20 pontos** de `Governance score`.

### O precedente que ficou pela metade

`context.go:56`: *"REQs — pelo PONTO ÚNICO de leitura (ADR-2026-09-03, D3/D4)"*. As REQs já foram
migradas; **os ADRs ficaram para trás**. É o achado **A1** da auditoria externa de 2026-09-05 outra
vez — ADR de ponto único marcada satisfeita com sítio sobrando.

## Acceptance Criteria

- [x] **Enumeração real** de todos os sítios que enumeram ADR — entregável da Wave 0
      → **9 sítios**, dos quais **3 são classe (iii), errados**. 🔴 **A Wave 0 refutou esta REQ**, que
      supunha 2 (`context` + `status`):

      | sítio | mecanismo | efeito |
      |---|---|---|
      | `context.go:38` `GetContext` | `os.ReadDir` raiz | `ADRs (0)` + score −20 |
      | `adr.go:189` `ListADRs` | `filepath.Glob` raiz | 🔴 `trackfw adr list` → **"No ADRs found"** |
      | `adr.go:316` `NewADRDraft` | `filepath.Glob` raiz | 🔴 **rascunho duplicado** se o twin está em subpasta |

- [ ] 🔴 **Os TRÊS sítios (iii) corrigidos no mesmo ML** (Regra Dura, mesma causa). `adr list`
      silenciar **todos** os ADRs é, para o usuário, tão grave quanto o `context`
- [ ] 🔴 **Dedup por caminho absoluto no resolvedor de nível-`cfg`.** Medido: `adr_dirs` **aninhadas**
      (`[docs/adr/zeus, docs/adr/zeus/done]`) fazem o `status` reportar **7** onde o correto é **4** —
      defeito que já existe hoje. Sem dedup, a correção faria `context` e `status` **concordarem no
      número errado**
- [ ] 🔴 **`context` e `status` consomem o MESMO resolvedor** (ADR D3). Não basta o `context`
      passar a descer: dois enumeradores que concordam hoje divergem amanhã
- [ ] Layout **plano** (ADRs na raiz de `adr_dirs`) **continua funcionando** — é o layout deste
      repositório, e quebrá-lo trocaria um defeito por outro
- [ ] Layout com **subpastas de estado** passa a ser enumerado
- [ ] 🔴 **O contra-braço que mede a contradição:** numa fixture com ADRs em subpastas, a saída do
      `context` **não** pode conter `## ADRs (0)` e, ao mesmo tempo, um warning nomeando um ADR.
      Este é o AC que mede o **efeito** — os outros são meios
- [ ] **`Governance score` medido antes/depois**: diferença de exatamente **20 pontos**, nem mais
      (medido na Wave 0: 40/100 → 60/100)
      ⚠️ **Este AC NÃO discrimina sozinho:** num repositório sem `adr_dirs` aninhadas ele passa mesmo
      sem o dedup do D4. Por isso o AC do dedup é **separado**, com fixture aninhada própria
- [ ] 🔴 **O critério de identificação NÃO muda:** o resolvedor mantém `HasSuffix(".md")`, **sem**
      filtro de prefixo `ADR-` — idêntico ao comportamento atual (`validator.go:3017`,
      `context.go:44`). Esta REQ corrige **alcance da varredura**, não critério. Um teste deve fixar
      isso, senão um refator futuro "melhora" o filtro e muda contagens em silêncio
- [ ] 🔴 **Gate que impede a reintrodução**, falsificável nas duas direções
      ⚠️ **AC AJUSTADO pela Wave 2 (2026-09-29) — a redação original prometia mais do que qualquer
      gate textual entrega.** O gate detecta o padrão **direto e nominal**
      (`os.ReadDir(<var de ADRDirs>)`, `filepath.Glob` em `adr.go`) **e** a **variável intermediária**
      (`dirs := cfg.ADRDirs`, medida como evasão e coberta no ML-1E). 🔴 **Não** detecta enumeração
      via **helper em outro escopo** — isso exigiria análise de fluxo, e é **limite da técnica**,
      declarado no header do gate em vez de prometido e não entregue
- [ ] 🔴 **Warnings não duplicam com `adr_dirs` aninhadas.** Medido na Wave 2 e confirmado pelo
      arquiteto: 3 ADRs reais → contagem **3** (correta) mas **6 warnings**, cada ADR repetido. É
      **uma nova contradição interna** — `## ADRs (3)` ao lado de 6 avisos sobre ADRs — e **a mesma
      causa** do D4: enumerador que itera `cfg.ADRDirs` sem deduplicar
- [x] `make quality` **RC=0** — rodado pelo arquiteto. CI: ver PR

## Negative scope — o que esta REQ NÃO faz

- **Não** trata o **#435** (`traceid_orphan_req` dispara para REQ em `backlog/`). Medição escrita:
  aplicando o teste da Regra Dura — *"se eu corrigir esta causa, exatamente estas falhas fecham"* —
  corrigir o enumerador de ADR do `context` **não fecha** o #435, que é uma **regra de validação
  que não filtra por estado**, não um **enumerador que não desce em subpastas**. Correções disjuntas,
  causas distintas. REQ própria.
- **Não** muda o **formato** da saída do `context`, nem o critério dos 20 pontos por categoria. O
  score está errado porque o **insumo** está errado; mexer na fórmula mascararia a causa.
- 🔴 **Não** corrige o **filtro frouxo**: com `HasSuffix(".md")`, um `NOTAS.md` na raiz de `adr_dir`
  **é contado como ADR**. Medido pelo arquiteto em 2026-09-29: 4 ADRs reais → `status` reporta **8**
  com um `NOTAS.md` presente e `adr_dirs` aninhadas. **Causa distinta** — critério de identificação,
  não alcance de varredura —, e corrigir os dois juntos tornaria impossível atribuir qualquer
  variação de contagem a uma das causas. **Candidato a issue própria.**
- **Não** migra layout de ADR de ninguém. `adr_dirs` com ADRs na raiz continua válido — **D1** é
  união, não substituição.
- **Não** estende a **ADR-2026-09-03** por analogia: o invariante dela é que *REQ não tem dimensão de
  estado*, e ADR **tem**. Ver **D2** da ADR desta REQ.

## Linked ADR
ADR: docs/adr/ADR-2026-09-29-a-enumeracao-de-adr-e-uniao-de-layouts-e-context-e-status-consomem-o-mesmo-ponto-unico.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-29-trackfw-context-reporta-adrs-0-onde-status-reporta-145.md
