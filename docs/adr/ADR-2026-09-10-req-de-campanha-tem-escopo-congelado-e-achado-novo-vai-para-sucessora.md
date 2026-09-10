---
name: ADR-2026-09-10-req-de-campanha-tem-escopo-congelado-e-achado-novo-vai-para-sucessora
status: Accepted
date: 2026-09-10
---

# ADR: REQ de campanha tem escopo congelado, e achado novo vai para sucessora

## Contexto

Medido na auditoria de governança de 2026-09-10
(`docs/qualidade/2026-09-10-auditoria-de-governanca-e-retomada-de-prioridade.md`):

```
ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz.md
   32 MLs · 2.496 linhas · aberto desde 03/09
   ML-4A e ML-4B aparecem DUAS vezes, em MLs distintos
```

Ao mesmo tempo, a entrega estava alta: **12 PRs mergeados em 3 dias**, e a taxa de abertura de REQ
caiu **8x** (16/dia em 02/09 → 2/dia em 09/09). **Não havia falta de progresso.**

O usuário descreveu a sensação como *"vagando sem rumo, correndo atrás do rabo"*. A medição mostrou
que a causa não era o ritmo — era a **ausência de linha de chegada**.

### O mecanismo

A **Regra Dura de Causa Raiz** (`CLAUDE.md`) manda: mesma causa → mesma REQ → mesmo PR. Ela está
certa, e existe porque o padrão oposto — abrir REQ nova por achado — produziu 59 ocorrências de
"REQ própria" espalhadas por 30 roadmaps, cada uma um defeito medido e não corrigido.

🔴 **Mas aplicada a uma classe de defeito da qual alguém encontra sítios novos todo dia, ela converte
uma REQ numa fila sem fundo.** Cada achado entra **corretamente**, e a REQ **nunca alcança estado
terminal**.

**Uma REQ que não pode fechar não é uma REQ — é um backlog com título.**

Na semana de 03/09 a 10/09, cinco issues externos chegaram só em 10/09, e cada um aterrissou
**direto na REQ que estava `wip`**, estendendo-a durante a própria execução.

## Decisão

### D1 — REQ de campanha declara escopo CONGELADO, por número medido

Uma **REQ de campanha** é a que agrupa muitos sítios da mesma causa. Ela passa a declarar, por
escrito, o **conjunto congelado** que a fecha — e o conjunto é **medido, nunca estimado**.

Para a `REQ-2026-09-03-as-217-falhas-reais-de-windows-...`, o escopo congelado é:

```
os 67 rótulos que persistiam no censo x64 do run 34470154546
evidência: ~/Documents/trackfw-evidencias/censo-x64-2026-09-09/
```

**A REQ fecha quando esses 67 estiverem triados e endereçados** — corrigidos, ou classificados com
motivo escrito. Nada além disso a mantém aberta.

🔴 **Congelar não é fechar com sítio conhecido em aberto.** O achado A1 da auditoria externa de
2026-09-05 foi marcar `✅` deixando sítios conhecidos por corrigir, **em silêncio**. Aqui o oposto é
exigido: o sítio novo é **registrado, nomeado e roteado** — apenas não estende a REQ em execução.

### D2 — Achado novo de mesma causa vai para a REQ SUCESSORA, não para a ativa

Quando um sítio novo da mesma causa aparece **depois do congelamento**, ele entra numa **REQ
sucessora**, explicitamente vinculada à congelada (`sucede: <REQ>`).

Isso preserva a Regra Dura de Causa Raiz — mesma causa continua tendo **uma** REQ ativa por vez, e a
sucessora herda a causa, a evidência e o histórico — **sem** permitir crescimento durante a execução.

**O teste continua o mesmo:** *"se eu corrigir esta causa, exatamente estas falhas fecham — e nenhuma
outra."* O que muda é **quando** o conjunto é fixado: no congelamento, não continuamente.

### D3 — Achado externo entra em TRIAGEM, nunca direto na REQ ativa

Todo achado de terceiro é **lido, respondido e classificado** — e então **agendado**. Nunca anexado
à REQ que está em execução.

🔴 **Isto não é reduzir engajamento com relator externo, e a evidência é explícita contra isso.** Na
semana medida, achados externos:

- derrubaram **duas hipóteses nossas em uma linha** (`a{b,c}d` → dois argumentos, issue #308);
- acharam defeito numa guarda **aprovada em auditoria pelo arquiteto** (#307);
- descreveram a armadilha de falsificação contaminada que nós pisamos por conta própria no mesmo dia
  (#309).

**O custo nunca esteve em ler. Esteve em executar na hora.** D3 separa as duas coisas.

### D4 — Roadmap de campanha tem teto de revisibilidade

Roadmap que passa de **~30 MLs** ou **~2.000 linhas** perde revisibilidade — e o sinal medido de que
isso aconteceu foi **ID de ML duplicado** (`ML-4A`/`ML-4B` duas vezes), que torna critério de aceite
não rastreável.

Ao cruzar o teto, o roadmap **é congelado com a REQ** e o trabalho seguinte vai para o roadmap da
sucessora.

## Consequências

**Positivas**

- A REQ de campanha volta a ter **condição terminal verificável por qualquer um** — o número é
  medido e a evidência é durável.
- Achado externo deixa de ter efeito colateral sobre o cronograma da frente ativa.
- O limite de revisibilidade impede que roadmap vire arquivo que ninguém audita inteiro.

**Negativas, declaradas**

- 🔴 **Mais REQs no acervo.** Uma campanha longa pode gerar sucessoras encadeadas. É o custo aceito em
  troca de cada uma poder fechar. **Mitigação:** a sucessora só nasce quando há sítio novo real —
  nunca preventivamente.
- **Risco de a sucessora virar cemitério**, que é a objeção óbvia. **Mitigação:** a sucessora é
  criada com `sucede:` preenchido e entra na fila com a prioridade da causa original, não no fim.
- **Julgamento sobre o que é "campanha"**. Não há regra automática; a decisão é do arquiteto, e fica
  escrita na REQ.

## Alternativas recusadas

**Abrir REQ nova por achado.** É o padrão que a Regra Dura de Causa Raiz existe para eliminar — 59
ocorrências, cada uma um defeito medido e não corrigido, perdido numa fila.

**Deixar a REQ crescer até a causa acabar.** Foi o que produziu esta ADR. Não existe "a causa acabar"
quando há alguém medindo em plataforma que nosso CI não cobre.

**Reduzir o engajamento com o relator externo.** Recusada pela evidência da §D3: a taxa de acerto dos
achados é alta e três deles pegaram defeitos que a nossa própria auditoria aprovou.
