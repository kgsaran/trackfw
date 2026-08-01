---
status: Accepted
date: 2026-07-26
author: "KG"
---

# ADR: principios de design de gates verificaveis

> Date: 2026-07-26 | Status: Accepted
> Accepted: 2026-08-01

## Context

O trackfw vende **governança verificável**. Seus próprios gates, portanto, são o produto — não
infraestrutura acessória. Durante a execução de duas REQs consecutivas em 2026-07-26, **três gates
falharam**, e nenhum foi detectado pelo CI. Todos apareceram em auditoria manual, em cenário real.

| # | Gate | Defeito | Como apareceu |
|:-:|---|---|---|
| 1 | `scripts/check-integration-cli-parity.sh` | `assert len(items) == (10 if kind == "agents" else 5)` — número mágico | Quebrou ao acrescentar 2 agentes; quebraria de novo ao acrescentar skills |
| 2 | `scripts/check-cli-parity.sh` | `grep` de nome de comando falha com a ajuda colorida do `argparse` (Python 3.13+) | Reprova em máquina com Python 3.14; passa no CI com 3.10/3.12 |
| 3 | `branch_has_wip_roadmap` | Só enxerga `wip/`; mover o roadmap para `done/` na branch — como a Definition of Done exige — reprova | Ao encerrar a REQ anterior |

O defeito 2 tem um agravante que nomeia o padrão: os comandos `agents` e `skills` **passavam por
acaso**, porque essas palavras aparecem sem cor em texto descritivo (`"List and manage trackfw
agents"`). O gate casava na descrição, não no nome do subcomando. Ou seja, **estava verde validando
menos do que aparentava** — e a diferença dependia de coincidência de vocabulário.

Os três defeitos compartilham a mesma raiz: **um gate verde não é evidência de que o gate funciona.**

## Decision

Adotar quatro princípios para todo gate do trackfw — scripts de paridade, regras do validator e
verificações de CI.

**P1 — Nenhum número mágico.** Contagens e listas esperadas derivam da fonte de verdade
(`catalog.json`, `KnownAgentIDs`, config), nunca de constante no gate. Um gate que precisa ser
editado sempre que o produto cresce será esquecido.

**P2 — Falha explícita, nunca degradação silenciosa.** Se o gate não consegue obter o que precisa
para verificar (arquivo ausente, comando indisponível, parse falho), ele **falha** dizendo o porquê.
Um gate que degrada para "sempre passa" é pior que um gate quebrado: quebrado é visível.

**P3 — Independência de ambiente.** O resultado não pode variar com versão de runtime, cor de
terminal, locale, `PATH` ou presença de ferramenta externa. Onde a dependência é inevitável, ela é
neutralizada explicitamente (por exemplo, remover ANSI antes de comparar texto) e não delegada a
variável de ambiente que o runtime pode ignorar.

**P4 — Falsificabilidade obrigatória.** Todo gate novo ou corrigido só é aceito com **demonstração de
que ele ainda reprova** o caso que deveria reprovar. "Ficou verde" não é aceite; "reprovou quando
deveria e passou quando deveria" é. Essa prova é registrada no roadmap ou no teste.

Além dos princípios, decide-se que **o `branch_has_wip_roadmap` passa a aceitar roadmap em `done/`
cujo slug case com a branch**, resolvendo a contradição com a Definition of Done. A intenção original
da regra — pegar branch de feature sem governança — é preservada: reprova apenas quando não há
roadmap correspondente em `wip/` **nem** em `done/`.

### Adendo 2026-07-27 — contrato de idade `stale_wip` e erros de inspeção

Para a regra `stale_wip`, **idade significa tempo desde a entrada mais recente do roadmap em `wip/`**,
não tempo desde o último commit nem desde a última alteração do arquivo. A fonte canônica é
`docs/roadmaps/.trackfw-log`, usando a linha mais recente do artefato atual cujo destino seja
`wip` (`backlog → wip`, `analyzing → wip`, `blocked → wip` etc.). Em
`roadmap_namespacing: by_agent`, o identificador inclui o prefixo do agente exatamente como gravado
no log, por exemplo `zeus/ROADMAP-...md`.

Fallback documentado: se o projeto ainda não possui `.trackfw-log`, ou se o artefato em `wip/` não
tem transição de entrada em WIP parseável, os três runtimes usam o `mtime` do arquivo como
compatibilidade retroativa. `git log` deixa de ser fonte contratual para idade de WIP, porque mede
edição/commit do arquivo e não permanência no estado. O limite default permanece 7 dias e a
severidade default da regra permanece `warning`.

Política de inspeção para validators:

| Caso | Contrato |
|---|---|
| `ENOENT` de diretórios de estado opcionais (`wip/`, `blocked/`, `done/` etc.) | Sem finding; diretório ausente continua significando estado vazio. |
| Permissão negada, `ENOTDIR` ou erro de walk em diretório configurado/existente | Emitir diagnóstico da regra, com caminho e causa; severidade segue a configuração da regra (`stale_wip` default `warning`). |
| Arquivo esperado mas ilegível (`stat`/read falha) | Emitir diagnóstico da regra para o arquivo; não converter em sucesso silencioso. |
| Arquivo de apoio inválido ou linha de log inválida | Emitir diagnóstico da regra e aplicar fallback documentado quando houver; nunca ocultar parse falho. |

Essas decisões preservam P2: ausência esperada não é erro, mas falha real de inspeção não pode
parecer gate verde.

## Consequences

### Positivas
- Os gates passam a ser tão auditáveis quanto o que eles auditam — coerente com o que o produto vende.
- P4 muda o critério de aceite de MLs que tocam gates, e transfere o ônus da prova para quem altera.
- Corrigir o `branch_has_wip_roadmap` destrava o encerramento de roadmap na própria branch, que hoje
  força escolher entre CI verde e Definition of Done cumprida.

### Negativas e custos
- P4 encarece cada ML que mexe em gate: exige um passo de falsificação, com montagem e desmontagem
  do cenário negativo.
- P3 pode exigir normalização defensiva em vários pontos (ANSI, fim de linha, locale), aumentando o
  código dos scripts.
- Afrouxar o `branch_has_wip_roadmap` cria um caminho novo: mover o roadmap para `done/` cedo demais
  e continuar codando sem gate. Mitigação a avaliar na REQ: exigir que o slug case, e não apenas que
  exista algum roadmap em `done/`.

## Alternatives Considered

- **Corrigir os três defeitos sem registrar princípio** — rejeitada: os três têm a mesma raiz, e sem
  o princípio o quarto defeito aparece igual.
- **Cobrir os gates com testes automatizados em vez de exigir falsificação manual** — desejável, mas
  insuficiente sozinho: os três defeitos existiam com o CI verde. Teste de gate também precisa provar
  que reprova. Não é alternativa, é complemento — deve entrar como escopo da REQ.
- **Manter `branch_has_wip_roadmap` como está e mover o roadmap só após o merge** — rejeitada:
  contraria a Definition of Done que o próprio produto passou a pregar e cria passo manual
  pós-merge, exatamente o "rabo" que o kanban existe para evitar.
