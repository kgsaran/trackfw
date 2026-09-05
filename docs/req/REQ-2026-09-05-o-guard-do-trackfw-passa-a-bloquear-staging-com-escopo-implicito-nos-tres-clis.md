---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: ""
---

# REQ: o guard do trackfw passa a bloquear staging com escopo implicito nos tres CLIs

> Date: 2026-09-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Governada por
`docs/adr/ADR-2026-09-05-staging-com-escopo-implicito-e-bloqueado-porque-ninguem-audita-o-que-nao-enumerou.md`.

**Origem: erro do próprio arquiteto, em 2026-09-05.** `git add -A` com subagente editando a árvore
varreu um teste de produto para dentro de um commit rotulado `chore(governance)` (`228fea5`). A regra
"nunca commitar com subagente vivo" **existia escrita** — em instruções de projeto e em memória — e
não impediu nada. Quem detectou foi o subagente.

**O pedido do usuário é preciso, e é o que define esta REQ:** *"precisamos bloquear no harness, não
na memória, e isso precisa fazer parte do produto."*

São duas exigências distintas:
1. **No harness** — controle executável, não prosa.
2. **No produto** — não é um remendo local deste repositório; é comportamento do `trackfw` para quem
   o instala.

O trackfw já emite guard de `git push`, `commit`, `checkout -b`, `stash` e credencial. **Staging com
escopo implícito é a lacuna.**

## Acceptance Criteria

- [ ] **AC1** — O guard bloqueia `git add` de escopo implícito: `-A`, `--all`, `.`, `-u`,
      `--update`, e a forma sem operando. Nos **3 CLIs**, com mensagem byte-idêntica.
- [ ] **AC2** — Staging por caminho explícito **continua passando**, inclusive múltiplos caminhos e
      caminhos com espaço/acento.
- [ ] **AC3** — 🔴 **Controle de não-afrouxamento**, e é o critério principal: enumerar o que passou a
      ser bloqueado e mostrar que **nenhuma forma legítima entrou no conjunto**. Guard que bloqueia
      demais é desligado pelo usuário — e aí protege zero. O modo de falha caro aqui é o **falso
      positivo**, ao contrário dos guards de detecção.
- [ ] **AC4** — 🔴 **Guarda de vacuidade:** com a regra desligada, os cenários de bloqueio **falham**.
      Cenário que aprova sem a regra não mede.
- [ ] **AC5** — Mensagem de commit contendo o texto `git add -A` **não** é tratada como comando —
      mesmo falso-positivo que o guard já resolveu para `commit`/`push`, e que tem tratamento a
      reaproveitar, não a reinventar.
- [ ] **AC6** — 🔴 **A mensagem ensina.** Mostra `git status --short` para enumerar e a forma válida.
      `git add -A` é reflexo na maioria dos fluxos; um "bloqueado" seco vira desativação.
- [ ] **AC7** — Os limites de tokenização ficam **declarados** (`git${IFS}add`, `{git,add}`,
      `g""it add` seguem fora de alcance): tripwire para o dedo escorregando, **não** defesa contra
      adversário. Vender como defesa repetiria o erro que o checker de terceiros já documenta.
- [ ] **AC8** — 🔴 **O gate de paridade cobre a regra nova.** Os guards existentes têm cobertura
      cross-CLI byte a byte; sem estendê-la, as 3 suítes internas podem concordar entre si com a
      mesma premissa errada — foi exatamente o que aconteceu no ML-3A desta semana, onde só o gate
      cross-CLI pegou o que as três suítes deixaram passar.

## Negative Scope

- ❌ **Não** implementar detecção de "agente ativo" — D2 da ADR recusa explicitamente: falharia
  aberto no caso perigoso.
- ❌ **Não** mexer nas regras já existentes (`push`, `commit`, `checkout`, `stash`) além do necessário
  para acomodar a nova.
- ❌ **Não** bloquear `git add -p` nem `git add -i` — são staging **interativo e enumerado**, o
  oposto do defeito.
- ❌ **Não** transformar isto em política de conteúdo (segredo, tamanho de arquivo, tipo). O escopo é
  **enumeração**.

## Linked ADR
ADR: docs/adr/ADR-2026-09-05-staging-com-escopo-implicito-e-bloqueado-porque-ninguem-audita-o-que-nao-enumerou.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
