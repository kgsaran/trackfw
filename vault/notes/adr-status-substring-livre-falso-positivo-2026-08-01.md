# Detecção de "ADR não aceito" por `content.includes('Status: X')` gera falso-positivo em ADRs que citam o próprio literal na prosa

> Data: 2026-08-01 | Autor: Apolo (ML-1B, Node) | Domínio: validator — regra `adr_accepted_when_req_done` / `blocked_by_draft_adr`

## Sintoma

Ao implementar o helper canônico `adrNotAcceptedStatusForRule` (Draft **ou** Proposed) para
`ROADMAP-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida`, a primeira versão
copiou o mecanismo de detecção já existente em `adrDraftStatusForRule` (código herdado):

```js
content.includes('Status: Draft')   // ou 'Status: Proposed'
```

Isso classifica **qualquer arquivo cuja prosa contenha o literal** como "não aceito" —
independente do status real declarado no cabeçalho. O próprio
`docs/adr/ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito-e-regra-de-aceite-exigido-por-req-concluida.md`
(status real: `Accepted`) dispara o bug: seu `## Context` cita literalmente
`strings.Contains(content, "Status: Draft")` ao descrever o defeito antigo do Go, e a REQ que ele
referencia menciona `Status: Proposed` como exemplo de saída do gerador. Verificado:

```js
node -e "const v=require('./npm/src/validator');console.log(v.adrIsDraft('ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito-e-regra-de-aceite-exigido-por-req-concluida.md'))"
// → true (deveria ser false; status real é Accepted)
```

## Por que não disparou antes

A `blocked_by_draft_adr` original só olhava ADRs referenciados na seção `## Blocked by ADRs` de
REQs `Open` — nenhuma REQ `Open` referenciava esse ADR nessa seção. A regra nova
`adr_accepted_when_req_done` olha o campo `ADR:` de **toda** REQ `Done`. No momento em que a
REQ-2026-08-01 (que referencia este ADR) transicionar para `Done` — o próprio fechamento deste
roadmap — `validate` ficaria vermelho contra um ADR genuinamente aceito, só porque o ADR documenta
o bug citando o literal problemático.

## Causa raiz

Detecção por substring livre sobre o **documento inteiro**, não ancorada a uma linha. Qualquer
menção em prosa, bloco de código, citação ou exemplo ao literal `"Status: Draft"` /
`"Status: Proposed"` conta como sinal, mesmo fora da linha de cabeçalho que declara o status real.

## Correção aplicada (Node)

`extractAdrHeaderStatus(content)` — ancora na linha (`> Date: ... | Status: X`) usando o mesmo
padrão de marcador `'| Status: '` que `reqStatusEquals`/`reqStatusIsOpen` já usam do lado da REQ.
Extrai o valor declarado daquela linha específica; compara com `'draft'`/`'proposed'`
case-insensitive. `adrNotAcceptedStatusForRule` passou a usar essa extração em vez de
`content.includes(...)`.

## Impacto para Go e Python (ML-1A / ML-1C, mesma REQ/roadmap)

**Este mesmo defeito provavelmente existe no código herdado dos outros dois CLIs** —
`internal/validator/validator.go` (`adrDraftStatusForRule`, `strings.Contains(content, "Status:
Draft")`) e `pypi/trackfw/validator.py:396` usam a mesma estratégia de substring livre. Se o
helper canônico `adrNotAccepted`/`adr_not_accepted` for implementado copiando esse mecanismo sem
ancorar à linha, o mesmo falso-positivo vai aparecer assim que qualquer ADR (deste roadmap ou
outro) mencionar os literais na prosa — e este roadmap especificamente cria essa condição ao
documentar o próprio bug. **Recomendo ancorar por linha (`| Status: X`) nos três CLIs**, não só
no Node.

## Teste de regressão adicionado

`npm/tests/validator.test.js`: `'adr_accepted_when_req_done: ADR Accepted cuja PROSA cita "Status:
Draft"/"Proposed" -> sem violation (anchoring, nao substring livre)'` — fixture com cabeçalho
`Status: Accepted` e corpo citando os dois literais problemáticos; prova que a implementação
correta não dispara. Falha contra a versão `content.includes(...)` (confirmado manualmente antes
do fix).
