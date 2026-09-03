---
status: Open
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-02-separar-a-tripwire-de-disco-do-contrato-da-ac10.md"
---

# REQ: a tripwire de disco trunca o corpus congelado, e a AC10 acusa reclassificação que não houve

> Date: 2026-09-02 | Status: Open

## Motivation

`scripts/check-roadmap-barrier-contract.sh` congela um corpus de 144 roadmaps em
`scripts/testdata/roadmap-barrier-corpus-snapshot/` e verifica, pela AC10, que **nenhum ML hoje
reconhecido deixe de ser** — contagens pinadas mais um hash SHA-256 da tabela de vereditos.

O próprio comentário do gate diz de onde vem o conteúdo:

> Conteúdo vem do SNAPSHOT (bytes congelados), nunca do disco — o disco só prova existência
> acima, o veredito é sempre computado sobre o conteúdo pinado.

Mas quando o basename **não** está em `docs/roadmaps/**`, o laço faz `continue` — e o arquivo sai do
scan. O disco deixa de "só provar existência" e passa a **decidir o que entra no corpus congelado**.

## A consequência, medida

Apaguei **um** roadmap de `docs/roadmaps/done/` numa árvore limpa do `main` e rodei o gate:

```
FAIL [corpus/basename-missing-from-disk]  ← legítima
corpus: files=143 waves=426 exit2=14 lines=1476
FAIL [corpus/files-count]                    143, pinado 144
FAIL [corpus/waves-count]                    426, pinado 432
FAIL [corpus/mls-complete-verdict-counts]    evidence=627, pinado 639
FAIL [corpus/acceptance-evidence-verdict-counts] evidence=306, pinado 314
FAIL [corpus/non-reclassification]           "corpus reclassificado: hash da tabela mudou"
```

**Seis falhas. Uma legítima e cinco falsas.** A última é a pior: ela afirma que o corpus foi
**reclassificado**, e não foi — nenhuma linha do parser mudou. O corpus foi **truncado**.

A AC10 existe para responder uma pergunta: *"alguma mudança no parser reclassificou algum ML?"*.
Hoje ela responde "sim" a um `git rm` de roadmap. É um falso positivo na exata asserção que o gate
existe para fazer, e ele custa caro: manda alguém caçar mudança de parser que não houve.

## E contradiz a justificativa do próprio snapshot

O bloco de comentário que introduz o congelamento diz, com todas as letras:

> Por que snapshot versionado, não o working tree: o corpus cresce a cada roadmap novo mesclado —
> hashar o working tree ao vivo faria este gate reprovar em TODO commit futuro

O snapshot existe **para desacoplar do working tree**. A política de basename reacopla, e o `continue`
faz esse acoplamento atingir justamente as contagens e o hash que o congelamento protegia.

## Acceptance Criteria

- [ ] **AC1** — O scan varre **todos** os arquivos do snapshot, independentemente do disco. O
      veredito já vinha dos bytes congelados; só a contagem estava sujeita ao disco.
- [ ] **AC2** — A tripwire `corpus/basename-missing-from-disk` **continua reprovando**. Ela é um
      contrato legítimo e separado; o que muda é que ela deixa de arrastar a AC10 junto.
- [ ] **AC3** — 🔴 **Falsificação.** Apagando um roadmap do disco, o gate passa a emitir
      **exatamente uma** falha (a tripwire), com `files=144`, `waves=432`, `exit2=14` e o **hash
      pinado batendo**.
- [ ] **AC4** — 🔴 **Controle.** A AC10 continua acendendo pelo motivo certo: uma mudança que
      reclassifique qualquer ML muda o hash e reprova.
- [ ] **AC5** — Sem regenerar o snapshot nem o `.tsv` de vereditos. Nenhum dado pinado é reescrito.

## Negative Scope

**Não remove a tripwire.** Ela pergunta algo legítimo — "um arquivo do corpus congelado foi apagado
sem atualizar o snapshot?" — e a resposta interessa a quem mantém o corpus.

**Não decide se a tripwire deveria ser fatal.** Ela compara um *fixture de teste* com o conteúdo
**vivo** do repositório, e por isso não pode passar em nenhum fork ou downstream cuja árvore de
governança seja legitimamente outra. Isso é uma questão de desenho e é de vocês; esta REQ corrige o
defeito mecânico, que é a AC10 mentir.

**Não regenera nada.** Todo dado pinado fica byte a byte como está.

## Linked ADR
<!-- Correção de defeito mecânico; sem decisão nova a registrar. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-02-separar-a-tripwire-de-disco-do-contrato-da-ac10.md
