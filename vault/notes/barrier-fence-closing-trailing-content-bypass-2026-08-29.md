---
title: fechamento de cerca ignora conteúdo à direita do delimitador — bypass total de mls_complete e acceptance_evidence, nos 3 CLIs
tags: [barrier, security, commonmark, fence, gate-bypass]
date: 2026-08-29
related: [[barrier-crlf-divergencia-node-regex-2026-08-29]]
---

## Sintoma

Um ML com conteúdo real `**Status:** pending` e `- [ ] critério real não atendido` recebe
`mls_complete: passed` **e** `acceptance_evidence: passed` nos 3 CLIs (Go, Node, Python),
reproduzido ao vivo com os binários deste branch.

## Causa raiz

`detectFenceMarker`/`computeFenceMask` (`internal/commands/barrier.go:275-291` e `fenceMask` logo
abaixo; equivalentes `npm/src/commands/barrier.js:187-195` e `pypi/trackfw/commands/barrier.py:167-186`)
contam só a corrida de caracteres idênticos (` ``` `/`~~~`) no início da linha e ignoram qualquer
conteúdo à direita, para os dois papéis (abertura e fechamento). Isso é certo para abertura
(CommonMark permite info string, `` ```bash ``) mas errado para fechamento: CommonMark exige que a
linha de fechamento contenha **apenas** os caracteres da cerca + espaço em branco opcional. Uma
linha como `` ```qualquer-coisa `` **dentro** de uma cerca já aberta não fecha a cerca de verdade —
continua sendo conteúdo interno — mas o parser dos 3 CLIs trata como fechamento válido, encerrando
a máscara prematuramente e expondo o resto do "exemplo" (inclusive um `**Status:** done` e um
`- [x]` forjados) como conteúdo real da ML.

## Reprodução mínima

```
### ML-1A — ML nao concluida, mas libera a wave
Prosa introduzindo um exemplo do defeito que documentamos:
```
notas de exemplo, sem relacao com o trabalho real
```trailing-junk-que-nao-fecha-a-cerca-no-commonmark-real
**Status:** done
**Acceptance criteria:**
- [x] evidencia forjada, nada foi feito
Mais texto ainda dentro do exemplo, por CommonMark de verdade:
```
Conteúdo real da ML, fora do exemplo:
**Status:** pending
**Acceptance criteria:**
- [ ] critério real não atendido
```

`barrier <arquivo> --wave 1` nos 3 CLIs → `mls_complete: passed`, `acceptance_evidence: passed`.
Python isolado dá `Status: passed` para a wave inteira.

## Por que os 31 cenários do ML-3A não pegam

`scripts/check-roadmap-barrier-contract.sh` tem cenários de cerca de 3/4+ crases e til, status/
critérios forjados dentro de cerca "limpa" (sem sufixo na linha de fechamento) — mas nenhum usa uma
linha de fechamento **sufixada**. A suíte foi desenhada para a classe certa de ameaça
(sombreamento por cerca, ADR decisão 7) e mesmo assim não cobre a variante que quebra a própria
regra de fechamento.

## Correção recomendada

Em `fenceMask`/`computeFenceMask`/`_fence_mask`, ao avaliar o candidato a **fechamento** (branch
`fenced == true`), exigir que o texto após a corrida de caracteres da cerca seja vazio ou só espaço
em branco; a regra de **abertura** (`fenced == false`) não muda. Não fechado no diff revisado —
achado durante a revisão de segurança da barreira final do
`ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-do-barrier.md` (parecer
`hades-tf`), que REPROVOU a REQ por causa deste achado.

## Achado relacionado, mesma revisão

Divergência de normalização Unicode entre os 3 CLIs: Go (`unicode.Mn` via `golang.org/x/text`) e
Python (`unicodedata.combining()`) removem toda marca combinante Mn de qualquer bloco Unicode;
Node usa uma regex literal restrita a `[̀-ͯ︀-️]`. Uma marca combinante fora
dessa faixa (ex.: U+1DC0, `Mn`, ccc 230) faz Go e Python aceitarem `**Status:** d<U+1DC0>one` como
"done" enquanto Node rejeita — quebra AC3/AC4 da REQ (mesmo conjunto de formas aceitas nos 3 CLIs).
Ver o parecer completo no roadmap para reprodução e correção recomendada.

## Relacionado

- `vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md` — mesma família de defeito
  (casamento de status permissivo demais), causa raiz diferente.
- REQ: `docs/req/REQ-2026-08-28-barrier-so-reconhece-cabecalho-de-aceite-em-portugues-mas-os-3-geradores-de-roadmap-escrevem-em-ingles.md`
- ADR: `docs/adr/ADR-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-que-o-barrier-reconhece.md`, decisão 7.

## Correção (ML-3D, apolo-tf, 2026-08-29)

Fechado nos 3 CLIs: `internal/commands/barrier.go` (`fenceMask`), `npm/src/commands/barrier.js`
(`computeFenceMask`), `pypi/trackfw/commands/barrier.py` (`_fence_mask`) — o ramo de FECHAMENTO
agora exige, além de `char == fenceChar && length >= fenceLen`, que **não sobre nada além dos
próprios caracteres da cerca** na linha já trimada (`length == len(trimmed)`). O ramo de
ABERTURA não muda (info string continua permitida). Reprodução do parecer `hades-tf` verificada
bloqueando nos 3 runtimes antes de fechar o ML.

**Re-pin do corpus (PARTE B do gate falsificável):** a correção reclassifica exatamente 1 de 144
roadmaps do `FREEZE_REF` (`a4e8f35`) —
`docs/roadmaps/done/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-
arquiteto-ensina-trackfw-push.md`, seção "Auditoria do ML-1A e do ML-2A" (linhas ~223-236): um
bloco ``` de 3 crases sem info string aninha um ` ```bash ` de MESMO comprimento sem escalar para
4+ crases — o mesmo defeito de nesting que o AC10 daquele próprio roadmap corrige no exemplo do
template, mas não corrigiu neste bloco irmão. Sob o parser antigo (bug), o `` ```bash `` fechava
cedo por acidente e a paridade de abertura/fechamento se recompunha por coincidência algumas
linhas depois; sob o parser corrigido, a cerca externa nunca mais reequilibra até o fim do
arquivo, e as Waves 2-bis/3/4 (que vêm depois) corretamente reportam "no ML found" em vez de
enxergar ML-1B/ML-3A/ML-1C como conteúdo real — comportamento correto por CommonMark real, não
regressão. Confirmado por diff binário dos dois parsers (pré/pós-fix) sobre os 144 arquivos:
somente essas 6 linhas de veredito mudam (`PINNED_MLS_COMPLETE_EVIDENCE` 642→639,
`PINNED_MLS_COMPLETE_FAILURE` 110→113, `PINNED_ACCEPTANCE_EVIDENCE_EVIDENCE` 315→314,
`PINNED_ACCEPTANCE_EVIDENCE_FAILURE` 436→434, hash `fb5ef78...`→`44676e5...`), as outras 143 são
bit-a-bit idênticas. Pin atualizado em `scripts/check-roadmap-barrier-contract.sh` com o
diagnóstico completo em comentário no local do pin.

**Comportamento a documentar para quem revisar:** com a regra mais estrita, um roadmap real que
tenha esse mesmo defeito de nesting (cerca interna de mesmo comprimento sem info string ou com
info string de mesmo comprimento da externa) passa a falhar FECHADO (`no ML found`/vazio) a
partir do ponto de desbalanceamento, em vez de "religar" por acidente como antes. É a direção
correta (nunca libera uma wave por engano), mas é uma mudança de comportamento observável em
roadmaps mal formatados que antes "davam certo por sorte".

## Achado relacionado #2 fechado — direção fica em aberto para ADR

`normalizeStatusToken` do Node trocado de faixa fixa (`[̀-ͯ︀-️]`) para
`\p{Mn}` via `/\p{Mn}/gu`, igualando Go (`runes.In(unicode.Mn)`) e Python
(`unicodedata.combining`). Confirmado ao vivo: `**Status:** d<U+1DC0>one` agora dá `passed` nos
3 CLIs (antes: Go/Python aceitavam, Node rejeitava). **Mas a direção do alinhamento não foi
decidida por mim** — implementei o lado permissivo ("remove toda marca Mn") por ser o pedido
literal do achado #2, não por julgar que seja a direção certa. Censo do corpus real
(`FREEZE_REF=a4e8f35`, 144 roadmaps, toda linha `**Status:**`): **zero** ocorrências de qualquer
marca combinante (Mn) no primeiro token de status, incluindo VS16 — nenhum roadmap real hoje
usa nem o caso "legítimo" (✅ + VS16) que motivou a permissividade original. Isso é evidência a
favor da direção oposta (rejeitar qualquer marca combinante, com exceção pontual de VS16 se
algum dia aparecer um caso real) custar zero ao corpus atual — mas a decisão de fechar o
vocabulário nessa dimensão é do dono do ADR, não deste ML. Ver o handoff original (achado #2) e
o relatório final do ML-3D para o pedido explícito de decisão.
