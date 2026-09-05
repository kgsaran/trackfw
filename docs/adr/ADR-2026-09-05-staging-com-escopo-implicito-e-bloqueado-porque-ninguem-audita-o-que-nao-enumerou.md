---
status: Proposed
date: 2026-09-05
author: ""
---

## Contexto

Em 2026-09-05 o arquiteto rodou `git add -A` **com um subagente ainda editando a árvore**. O commit
`228fea5`, rotulado `chore(governance)`, carregou junto um arquivo de teste de produto que o agente
estava escrevendo naquele instante. A mensagem do commit descreve metade do que ele contém.

**Existia regra escrita contra isso** — "nunca commitar com subagente vivo" — em instruções de
projeto e em memória do agente orquestrador. **A regra não impediu nada.** Quem detectou foi o
próprio subagente, ao terminar, conferindo por `git hash-object` que o conteúdo em disco batia com o
já commitado.

🔴 **Uma regra que depende de alguém lembrar não é um controle.** É a mesma constatação que atravessa
a campanha inteira deste repositório: gate vácuo, teste que passa por consequência do bug, contagem
que esconde regressão. Em todos, o que falhou não foi a intenção — foi **não haver nada que
verificasse**.

E há a razão específica deste projeto: **o trackfw vende governança de entrega.** Um produto que
oferece guard de `git push` e de credencial, e cujo próprio orquestrador viola por esquecimento uma
regra de staging, tem um buraco no argumento — não só no processo.

### O mecanismo, em uma frase

`git add -A`, `git add .`, `git add --all` e `git add -u` estagiam **o que o autor não enumerou**.
No instante do commit, o conjunto real de arquivos é **desconhecido para quem assina a mensagem**.
Tudo o mais decorre disso: mensagem que não descreve o conteúdo, trabalho alheio varrido junto,
segredo capturado por acidente, artefato gerado entrando sem revisão.

## Decisão

### D1 — Staging com **escopo implícito** é bloqueado

O guard passa a reconhecer e bloquear `git add` quando o escopo não é enumerado: `-A`, `--all`, `.`,
`-u`, `--update`, e a forma sem operando algum.

Staging válido é **por caminho explícito**. O autor enumera o que vai commitar; é isso que torna a
mensagem verificável contra o diff.

### D2 — 🔴 A regra **NÃO** depende de detectar agente ativo

A tentação óbvia é bloquear só "quando houver subagente trabalhando". **Recusado.**

Detecção de atividade de agente é frágil e **falha aberto**: se o sinal não existir, não for escrito,
ficar obsoleto ou o agente morrer sem limpar, o guard libera exatamente no caso perigoso. Seria um
controle cuja falha é silenciosa e favorável ao erro — o padrão que este repositório vem catalogando
a semana toda.

O escopo implícito é danoso **por si**, com ou sem agente: quem commita sem enumerar não sabe o que
commitou. A regra é sobre **enumeração**, não sobre concorrência.

### D3 — O guard oferece a alternativa, sempre

Toda mensagem de bloqueio mostra o caminho válido: `git add <caminhos>`, `git status --short` para
enumerar, e o comando `trackfw` correspondente quando houver.

🔴 **Guard sem alternativa vira guard desligado na semana seguinte.** É a lição registrada nos guards
existentes deste projeto, que sempre apontam `trackfw ship`/`commit`/`branch new`.

### D4 — Regra dura de paridade: os 3 CLIs

O script é emitido por Go, Node e Python. Comportamento e mensagem **byte-idênticos**, como já vale
para `credential_guard` e `git_branch_guard`.

### D5 — Os limites de tokenização são **declarados**, não fingidos

O guard atual já registra que formas que exigem tokenizar como o shell (`git${IFS}add`, `{git,add}`,
`g""it add`) permanecem fora do alcance. O mesmo limite vale aqui e **fica escrito**.

🔴 É tripwire para o caso real — o dedo escorregando — **não** defesa contra adversário competente.
Vender como defesa seria repetir o erro do checker de markers de terceiros, cujo limite está
documentado justamente por isso.

## Consequências

**Muda o hábito de quem usa o trackfw, não só o nosso.** `git add -A` é reflexo em quase todo fluxo
de trabalho. A mensagem de bloqueio precisa ensinar, não só recusar.

**Não impede commit de arquivo não auditado** — só impede que ele entre **sem ser nomeado**. Quem
enumerar caminho a caminho e não olhar continua livre para errar. O controle move o erro de
"invisível" para "declarado", que é o máximo que um guard de staging alcança.

🔴 **Este ADR nasce de um erro do próprio arquiteto, e isso é parte da decisão, não uma nota de
rodapé.** A regra existia em prosa e falhou. O que muda não é a regra: é ela deixar de depender de
memória.

## Verificação exigida de quem implementar

- Falsificação **nas duas direções**: `git add -A`, `git add .`, `git add --all`, `git add -u` e
  `git add` sem operando → **bloqueiam**; `git add caminho/arquivo.go` e múltiplos caminhos
  explícitos → **passam**.
- 🔴 **Controle de não-afrouxamento:** enumerar o que passou a ser bloqueado e mostrar que **nenhuma
  forma legítima de staging entrou no conjunto**. Um guard de git que bloqueia demais é desativado
  pelo usuário, e aí protege zero.
- 🔴 **Guarda de vacuidade:** com a regra desligada, os cenários de bloqueio **têm** de falhar. Um
  gate que aprova com a regra ausente não está medindo.
- Os 3 runtimes exercitados separadamente, com a mensagem comparada byte a byte.
- O caso de **mensagem de commit que contém o texto `git add -A`** não pode ser tratado como comando
  — é o mesmo falso-positivo que o guard existente já enfrentou e resolveu para `commit`/`push`.
