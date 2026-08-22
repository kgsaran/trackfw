---
status: Accepted
date: 2026-08-22
author: "Zeus (Arquiteto)"
---

# ADR: Modelo de ameaça no desenho — Wave 0 de red team antes da implementação, no harness

> Date: 2026-08-22 | Status: Accepted

## Context

O harness manda hoje (`CLAUDE.md`, *Architecture Directives*): *"Security wave: include a red-team
review wave in every feature roadmap"*. Na prática essa wave é sempre a **última** — a barreira roda
depois que o código existe, e o que ela acha vira retrabalho.

**Evidência medida nesta série (2026-08-21/22), três REQs consecutivas:**

| REQ | O que a barreira achou | Natureza | Custo |
|---|---|---|---|
| `validate` cego ao hook relativo | 3 formas não capturadas (`$PWD/`, `$UNDEFINED/`, aspas) | **completude de enumeração** | virou REQ nova |
| `trackfw push` | AC2 não cumprido (duplicação em vez de reuso) · faltava gate de runtime do `--force-with-lease` | deriva plano×implementação · lacuna de detecção | 2 MLs extras |
| `validate` cego ao `$PWD` | `~/…` acusado (falso-positivo) · `${PWD}/…` silencioso · mensagem errada | **completude de enumeração** | REPROVADO, PR bloqueado |

**Duas das três eram completude de enumeração** — a pergunta *"quais formas existem, e qual o veredito
de cada uma?"*. Essa pergunta **não precisa de código para ser respondida**: o ADR do `$PWD` já trazia
uma tabela de classes; faltou alguém adversarial olhar para a tabela e dizer *"e o `~/`? e o
`${PWD}`?"*. Ambos foram achados depois, com o código pronto e o PR parado.

O caso do `~/` é o argumento mais forte: a regra criada para impedir que o guard fique inerte
passaria a **acusar o caminho do harness global do próprio trackfw**, empurrando o usuário a rodar
`trackfw update` e quebrar o próprio hook. Um adversário lendo a tabela de classes teria perguntado
por `~/` antes de qualquer linha de código.

## Decision

**Toda REQ ganha uma Wave 0 de modelo de ameaça, executada pelo papel de segurança antes da primeira
linha de implementação — e isso vive no harness do trackfw, não na prática deste projeto.**

### 1. Escopo: produto, não convenção local

A mudança é no **harness**, entregue a todo projeto que usa trackfw:

- **Gerador de roadmap** (`internal/generators/roadmap.go` e os equivalentes em `npm/src/` e
  `pypi/trackfw/` — regra dura de paridade): o template passa a emitir **Wave 0** antes da Wave 1,
  tanto em `roadmap new` quanto em `roadmap new --from-req`.
- **Asset do papel de arquiteto**: o protocolo de dispatch passa a exigir Wave 0 antes de despachar
  implementação.
- **`CLAUDE.md` semeado**: a diretiva *Security wave* deixa de descrever só a barreira final.
- **`trackfw barrier`**: reconhece Wave 0 como wave avaliável, sem tratamento especial.

Aplicação: `trackfw update harness` nos projetos existentes.

### 2. O entregável da Wave 0 é estreito, e é isso que a torna útil

Wave 0 **não** é "o Hades opina sobre o plano". É um artefato com quatro seções:

1. **Completude de enumeração** — dada a tabela/lista fechada do ADR ou da REQ, o que falta? Formas,
   estados, tipos e casos-limite ausentes, cada um com o veredito proposto.
2. **Modelo de ameaça** — quem é o adversário, o que ele controla, e qual é a fronteira de confiança.
3. **Alvos de falsificação** — quais sabotagens o gate terá de detectar, **nas duas direções**
   (acusar de menos e acusar de mais). Vira insumo direto do ML de gate.
4. **O que ficará fora, declarado** — o residual que a entrega aceita, nomeado antes e não depois.

### 3. A barreira final **permanece**, com escopo reduzido

Wave 0 responde *"a decisão está completa?"*. A barreira final responde *"a implementação faz o que a
decisão disse?"* — e essa só pode ser respondida **medindo o artefato**. As duas perguntas são
diferentes e nenhuma substitui a outra.

## Consequences

**Positivas**
- A completude de enumeração — 2 das 3 reprovações desta série — passa a ser resolvida quando custa
  um parágrafo, não um PR bloqueado.
- Os alvos de falsificação chegam ao ML de gate **já enumerados**, em vez de serem inventados pelo
  implementador.
- O residual é declarado antes da implementação, o que muda a conversa de "descobrimos uma lacuna"
  para "aceitamos esta lacuna".

**Negativas e riscos aceitos**
- **Um ciclo a mais em toda entrega**, inclusive nas que hoje passam limpas. Decisão do KG: aplicar a
  todas, sem triagem — regra que depende do julgamento otimista do arquiteto é a que ele burla.
- **Risco de virar ritual.** Mitigado pelo entregável estreito da §2: quatro seções, verificáveis. Um
  parecer de Wave 0 sem enumeração e sem alvos de falsificação é uma Wave 0 reprovada.
- **O revisor de desenho não mede** — ele raciocina sobre um artefato que ainda não existe. É o limite
  estrutural da Wave 0, e a razão de a barreira final continuar existindo.
- **Wave 0 não conserta o que mais custou tempo nesta série:** entregas declaradas verdes sem medição
  do exit code (seis ocorrências, duas delas vermelhas de verdade). Isso é disciplina de medição e
  precisa de controle próprio — não confundir os dois problemas nem esperar que este ADR resolva
  aquele.

## Alternatives Considered

**Wave 0 só quando houver gatilho** (tabela de casos, parser, permissão, segredo, fronteira de
confiança). Mais barata e mirada exatamente onde as reprovações nasceram. **Rejeitada pelo KG**: a
triagem devolve ao arquiteto a decisão de dispensar a revisão, e é justamente esse julgamento que a
barreira existe para corrigir.

**Manter só a barreira final.** É o estado atual, e o custo está medido acima: duas das três REQs
desta série retrabalharam por perguntas que não precisavam de código para serem respondidas.

**Wave 0 como regra bloqueante do `trackfw validate`.** Rejeitada nesta forma: `validate` roda o tempo
todo e uma regra que acusa todo roadmap sem Wave 0 recria o padrão *"guard que atrapalha é guard que o
usuário desliga"* (`ADR-2026-08-17`). O caminho é **gerador emite** + **barreira avalia**; se um sinal
em `validate` for desejável depois, que nasça como `warning`.
