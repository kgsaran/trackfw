---
status: wip
date: 2026-09-02
squad: ares-tf
req: "docs/req/REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md"
---

# Roadmap: `.gitattributes` com `merge=union` para o `.trackfw-log` nos 3 CLIs

> Criado em: 2026-09-02 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md

## Diagnóstico

O `docs/roadmaps/.trackfw-log` é append-only e **toda escrita cai na última linha**. Duas branches
que movam qualquer roadmap conflitam **sempre**. Não é azar: é propriedade do formato.

Medido em dois PRs consecutivos do reporter externo, no mesmo dia:

```
#238  ele acrescenta 10:46 ; a main acrescenta 10:45 e 11:21  -> CONFLITO
#240  ele acrescenta 10:51 ; a main ja tinha as tres          -> CONFLITO
```

Nos dois casos **zero disputa semântica** — todas as linhas deviam sobreviver, e a resolução manual
foi literalmente "mantenha as quatro em ordem cronológica". Conflito em arquivo de log é ruído puro.

**E não é defeito só deste repositório.** Verificado: não existe `.gitattributes` aqui, e o
`trackfw init` **não gera nenhum** em nenhum dos 3 CLIs. Todo projeto que adota o trackfw herda um
arquivo que conflita sempre que duas pessoas trabalham em paralelo — e o produto do trackfw é
exatamente coordenar trabalho paralelo governado.

**Urgência agora:** há **4 PRs abertos** do reporter (#245, #247, #248, #249), cada um com sua linha
no log. Qualquer movimentação nossa de roadmap conflita com os quatro. Este roadmap entra primeiro,
sozinho, justamente para parar a hemorragia antes da arrumação dos 5 roadmaps órfãos em `wip`.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — O arquivo, neste repositório e nos 3 geradores
> Dependências: nenhuma.

### ML-1A — `.gitattributes` no repositório e no `init` dos 3 CLIs
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.gitattributes` (novo, raiz), `internal/generators/scaffold.go`,
`npm/src/generators/init.js`, `pypi/trackfw/generators/init_gen.py`, `docs/cli-parity.md`

**Conteúdo mínimo do arquivo** — o caminho é relativo ao `roadmap_dir` configurado, que **varia por
projeto** (`trackfw.yaml`), então a regra tem de casar o **basename** e não um caminho fixo:

```
.trackfw-log merge=union
```

🔴 **Verificar antes de escrever:** o `req_dir` também tem um `.trackfw-log`
(`internal/generators/req.go:380`). Uma regra por basename cobre os dois — confirme que é isso que
se quer, e escreva o motivo.

🔴 **`merge=union` é driver nativo do git, mas não é mágico.** Ele nunca conflita — e é justamente
por isso que precisa de controle: um merge que "passou" pode ter embaralhado a ordem ou duplicado
linha, e ninguém olharia. O AC2 existe para isso.

**Critérios de aceite:**
- [ ] Duas branches que movem roadmaps distintos mergeiam **sem conflito** no `.trackfw-log`
- [ ] 🔴 **Controle (AC7 da REQ):** as linhas dos **dois** lados sobrevivem — verificado por
      **igualdade de conjunto**, não por ausência de conflito
- [ ] Falsificação na direção oposta: **sem** o `.gitattributes`, o mesmo cenário **conflita**
- [ ] `trackfw init` gera o arquivo nos **3 CLIs**, byte-idêntico (regra dura de paridade)
- [ ] 🔴 Projeto que **já** tem `.gitattributes` não tem o arquivo sobrescrito — append idempotente,
      e rodar `init` duas vezes não duplica a linha
- [ ] `make quality` verde e `trackfw validate` exit 0

**Comandos de validação:** `make quality`, e o cenário de merge de duas branches reproduzido em
repositório de rascunho.

## Wave 2 — Arrumação dos roadmaps órfãos
> Dependências: Wave 1 mergeada — antes disso, cada movimentação conflita com os 4 PRs abertos.

### ML-2A — Mover para `done` os 5 roadmaps concluídos
**Status:** ✅ Concluído
**Agente:** `trackfw_architect` (governança)
**Verificado em 2026-09-02:** os 5 têm **zero** MLs pendentes.

```
ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md      (3 ✅)
ROADMAP-2026-09-02-gate-do-barrier-escreve-utf-8-explicito-no-heredoc.md       (1 ✅)  reporter, #238
ROADMAP-2026-09-02-gerador-de-fixture-le-stdin-em-binario.md                   (1 ✅)  reporter, #240
ROADMAP-2026-09-02-governanca-dos-prs-238-e-240-do-reporter.md                 (1 ✅)
ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate.md  (5 ✅)
```

**Critérios de aceite:**
- [x] Os 5 em `done/`, com `status:` do frontmatter e linha de cabeçalho sincronizados
      (usar `trackfw roadmap move`, nunca `git mv`)
- [x] `wip/` contém **apenas** roadmaps com trabalho em andamento de fato
- [x] A política de **quem** move roadmap trazido por PR externo está escrita (AC5 da REQ)

**Executado.** Os 5 movidos com `trackfw roadmap move`, que sincronizou também o link da REQ em
cada um. `wip/` ficou com **apenas** este roadmap. `trackfw validate` → exit 0.

**Fechadas junto — 6 REQs cujo trabalho já estava entregue com o status aberto:** as 5 dos roadmaps
acima, mais `REQ-2026-08-21-nil-map-em-construcao-de-projectconfig...`, que a auditoria de backlog
mostrou corrigida **no nível da classe** — `initConfigMaps` por reflexão roda como primeira linha de
`parse()` (`internal/config/config.go:356,375`), e `go test ./internal/config/ -run "NoPanic|MapFields"`
passa, incluindo o gate anti-regressão do invariante. Verificado por mim antes de fechar.

## Política — quem move roadmap trazido por PR externo (AC5 da REQ)

**O mantenedor que faz o merge, no mesmo dia, e não o contribuidor.**

Razão medida: um PR externo traz o roadmap em `wip/` **já com os MLs marcados ✅**, porque para o
contribuidor o trabalho terminou quando ele abriu o PR. Pedir que ele mova para `done/` é impossível
— no momento em que ele escreve, o merge ainda não aconteceu, e um roadmap em `done/` num PR aberto
seria mentira. Então a transição **só pode** ser feita depois do merge, e quem tem essa informação é
quem mergeia.

Hoje isso rendeu **5 roadmaps órfãos acumulados**, 2 deles vindos de PR externo. Sem dono explícito,
`wip/` deixa de significar "trabalho em andamento" e vira depósito — e aí o
`branch_has_wip_roadmap` e o board do `serve` passam a descrever um estado que não existe.

Esta política é candidata natural ao `CONTRIBUTING.md`
(`REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-para-prs-e-nao-tem-contributing.md`),
com a metade que interessa ao contribuidor escrita do lado dele: *"deixe o roadmap em `wip/` com os
MLs concluídos; a transição para `done/` é do mantenedor"*.

## Acceptance Criteria

- [ ] Duas branches que movem roadmaps distintos mergeiam **sem conflito** no `.trackfw-log`
- [ ] 🔴 **Controle:** as linhas dos **dois** lados sobrevivem — igualdade de conjunto, sem perda e
      sem duplicação
- [ ] Falsificação na direção oposta: **sem** o `.gitattributes`, o mesmo cenário **conflita**
- [ ] `trackfw init` gera o arquivo nos **3 CLIs**, byte-idêntico
- [ ] Append idempotente sobre `.gitattributes` preexistente; `init` duas vezes não duplica a regra
- [ ] Os 5 roadmaps concluídos estão em `done/` (Wave 2)
- [ ] `make quality` verde e `trackfw validate` exit 0

## Verificação

O merge dos próximos PRs do reporter **sem conflito no log** é a prova final — não há gate que feche
isto sozinho.

## Barreira final

Arquiteto. **Sem `hades-tf`** — não há superfície de ataque; `merge=union` não executa código.
`hefesto-tf` só se a Wave 1 crescer além do arquivo e dos 3 geradores.
