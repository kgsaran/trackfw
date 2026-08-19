---
status: Accepted
date: 2026-08-19
author: "Zeus (Arquiteto)"
---

# ADR: caminho governado para push forçado e tag de release

> Date: 2026-08-19 | Status: Accepted

## Context

Medido duas vezes na entrega da `7.1.0`, com uma hora de intervalo: o caminho governado **acaba**
em dois pontos, e nos dois o único recurso foi sair do trilho.

O `case push)` do guard (`scripts/trackfw-git-branch-guard.sh:392`) é **incondicional**. O `ship`
cobre **uma** forma de push — `push [-u] origin <branch-atual>` (`buildPushArgs`,
`internal/commands/ship.go:595-603`). Entre as duas coisas sobra um vão: push pós-rebase, push de
tag, remoção de branch remota, push de branch que não é a atual, push para fork.

O caso mais grave é o da tag, porque **o protocolo de release documentado no `CLAUDE.md` deste
projeto é inexecutável dentro dos guardrails deste projeto**: o passo 4 manda `git push origin
v<x.y.z>`, e o guard recusa.

## Decision

**Dois caminhos distintos, porque são dois defeitos distintos.**

### 1. `trackfw ship --force-with-lease` — para o push pós-rebase

Rebasear é o desfecho normal de um conflito. O defeito aqui é *"o push do `ship` é estreito
demais"*, e a correção é alargar o `ship`.

**Sempre `--force-with-lease`, nunca `--force` cru.** O primeiro recusa quando o remoto avançou
desde o último `fetch`; o segundo destrói trabalho alheio sem perguntar. A diferença não é de estilo.

### 2. `trackfw release tag <versão>` — para a tag

**Tag não é operação de branch.** O portão do `ship` é "REQ + roadmap em `wip` para
feat/fix/refactor"; pendurar `--tag` num comando com esse portão é erro de categoria. Release é
protocolo próprio, com regras próprias, e o `CLAUDE.md` já o descreve assim.

Implementação de referência já validada em produção (a tag da `7.1.0` foi publicada assim), com
tag **anotada** preservada:

```
POST /repos/{owner}/{repo}/git/tags   {tag, message, object: <commit>, type: "commit", tagger}
   -> sha do OBJETO de tag
POST /repos/{owner}/{repo}/git/refs   {ref: "refs/tags/v<x.y.z>", sha: <sha do objeto>}
```

## A postura de segurança, declarada e não escondida

Todo escape hatch é também caminho para agente induzido: em vez de `git push --force`, ele roda
`trackfw ship --force-with-lease`. Pelo `ADR-2026-08-12` **não se pretende prevenir** agente
induzido — mas é obrigatório dizer o que se faz a respeito.

**Decisão: `ship --force-with-lease` só opera em branch que já tem PR aberto.**

O raciocínio: o caso legítimo é sempre o mesmo — resolver conflito de um PR em revisão. Fora dele,
não há motivo para reescrever história de uma branch remota. Amarrar a permissão ao PR aberto reduz
a superfície ao caso real, e tem uma propriedade que um rastro dentro do repositório não tem: o PR
mora **no forge**, fora do alcance de escrita do agente. Um force-push destrutivo passa a exigir,
antes, abrir um PR — que é visível, atribuível e não reescrevível por quem tem escrita no repo.

**Consequências aceitas, ditas por extenso:**

- **Dependência do forge no caminho de push.** Sem CLI de forge disponível, o `--force-with-lease`
  **recusa com orientação** em vez de degradar para push permissivo. Recusar é o lado seguro.
- **Não é prevenção.** Um agente induzido pode abrir o PR e então forçar. Isso é aceito e coerente
  com o `ADR-2026-08-12`: o ganho é **detecção ancorada fora do repositório**, não bloqueio.
- **Atrito no caso raro legítimo** de force-push em branch sem PR. Aceito: é raro, e a saída é abrir
  o PR — que é o que se deveria fazer de qualquer forma.

## Consequences

**Positivas**
- O protocolo de release do projeto passa a ser executável dentro dos próprios guardrails. Hoje não é.
- Some o contorno que a `7.1.0` exigiu (republicar em branch nova, PR órfão, tag por API à mão).
- O force-push ganha uma âncora fora do repositório, que é a direção que o `ADR-2026-08-12` prescreve.

**Negativas / riscos**
- Superfície nova de CLI em 3 CLIs, com custo de paridade — e paridade aqui **exige gate comparando
  saídas reais**, porque esta série já provou três vezes que teste por stack não pega divergência.
- `release tag` toca publicação: um defeito nele produz tag errada em repositório público, que é
  caro de desfazer.

## Alternatives Considered

- **`ship --tag`** — rejeitada: erro de categoria. O portão do `ship` não se aplica a release.
- **`trackfw release` cobrindo o ciclo inteiro** (bump + CHANGELOG + PR + tag) — adiada, não
  rejeitada. Resolve mais, mas o escopo é várias vezes maior, e o vão medido é o push da tag. Fazer
  o `release tag` primeiro não impede o resto depois.
- **Afrouxar o `case push)` do guard** — rejeitada explicitamente: ele ser incondicional é o que o
  torna honesto. A correção mora no `ship`/`release`, nunca em enfraquecer a tripwire.
- **Aceitar o force-push sem restrição, só declarando** — rejeitada em favor da amarração ao PR
  aberto, que custa pouco e move a âncora para fora do repositório.
