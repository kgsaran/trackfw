# Auditoria de governança e retomada de prioridade — 2026-09-10

> Pedida pelo usuário: *"tenho sentido que estamos vagando sem rumo há dois ou três dias, correndo
> atrás do rabo e perdendo toda a governança do projeto."*

Medi antes de concordar. **A sensação está certa e a atribuição está errada por um nível.**

---

## 1. O que os números dizem — e contradizem a premissa

```
PRs mergeados em 3 dias                    12
REQs criadas por dia   02/09: 16  →  09/09: 2   (queda de 8x)
roadmaps em done                          176
roadmaps em wip                             2
```

🔴 **Não estamos vagando. Estamos entregando rápido.** Doze PRs em três dias, com a taxa de abertura
de REQ **caindo 8x** — o oposto de acúmulo descontrolado.

Se a métrica fosse volume ou disciplina de artefato, o projeto está melhor hoje do que em 02/09.

## 2. O mecanismo que explica a sensação, e ele é real

```
ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz.md
   32 MLs · 2.496 linhas · aberto desde 03/09
   IDs duplicados: ML-4A e ML-4B aparecem DUAS VEZES, em MLs distintos
```

A **Regra Dura de Causa Raiz** manda: mesma causa → mesma REQ → mesmo PR. Está certa. Mas aplicada a
uma classe de defeito da qual um relator externo encontra **sítios novos todo dia**, ela converte uma
REQ num **fila sem fundo**: cada achado novo entra corretamente, e a REQ **nunca alcança estado
terminal**.

🔴 **Uma REQ que não pode fechar não é uma REQ — é um backlog com título.**

E o sintoma de que o arquivo passou do ponto de revisão está medido: **dois pares de MLs com ID
duplicado**. Com ID repetido, critério de aceite não é rastreável sem ambiguidade.

**É isto que produz a sensação de não ter rumo.** Não a falta de entrega — a ausência de linha de
chegada.

## 3. Sobre o relator externo

O usuário escreveu: *"o Lourival está nos atrapalhando na governança do nosso projeto."*

**A instância disso não se sustenta na evidência, e a preocupação sim.**

A qualidade dos achados, medida nesta semana:

| achado | efeito |
|---|---|
| `a{b,c}d` → dois argumentos | derrubou **duas** hipóteses nossas em uma linha |
| **#307** | achou defeito numa guarda **que eu aprovei em auditoria** |
| **#309** | a nota de método dele descreve a armadilha de falsificação contaminada que nós pisamos por conta própria no mesmo dia |

🔴 **O problema não é o que ele encontra — é onde cada achado aterrissa.** Hoje, todo issue novo entra
**direto na REQ que está `wip`**, e a estende. Cinco issues chegaram só em 10/09.

**A correção é regra de roteamento, não menos engajamento:**

> Achado externo entra em **triagem**, não na REQ ativa. É classificado, recebe REQ própria ou entra
> numa REQ **congelada em escopo**, e é **agendado**. Nunca anexado ao que está em execução.

Isso preserva a Regra Dura de Causa Raiz — mesma causa continua na mesma REQ — sem permitir que uma
REQ ativa cresça indefinidamente durante a própria execução.

## 4. Estado real dos artefatos

### 4.1 REQs

```
Open / In Progress                         40
  🔴 sem roadmap nenhum                     27   (67%)
  com roadmap em backlog/analyzing/wip       9
  🔴 com roadmap JÁ EM done/                 4   ← trabalho entregue, status não fechado
```

**As 4 com roadmap em `done/`** são sincronização de status, não trabalho:

```
2026-08-12  mitigacao-do-fail-open-do-credential-guard-...
2026-09-02  guard-instalado-emite-schema-de-hook-...          (entregue em 7.5.0, PR #297)
2026-09-05  auditoria-externa-aponta-que-declaramos-...       (PR #289)
2026-09-09  update-harness-reescreve-o-script-do-guard-...    (entregue em 7.5.1, PR #302)
```

### 4.2 Roadmaps — 🔴 nenhum está pronto para despacho

```
7 roadmaps em backlog
0 com seção "Arquivos afetados"
```

O formato de roadmap do projeto exige **arquivos exatos, valores exatos, comandos exatos**. Nenhum
dos sete tem. Os quatro criados hoje por `--from-req` são **stubs gerados dos critérios de aceite** —
esqueleto, não plano.

🔴 **"Tem roadmap" e "está pronto para despachar" são estados diferentes.** Confundi-los é como o
roadmap do ratchet ficou quatro dias em backlog **afirmando que uma ADR estava `Accepted` quando ela
estava `Proposed`**.

---

## 5. A lista de prioridade — ignorando a anterior, como pedido

### Faixa 0 — higiene de estado (minutos, não horas)

| # | ação | efeito |
|---|---|---|
| **0.1** | Fechar as **4 REQs** cujo roadmap está em `done/` | 40 → 36 REQs abertas |
| **0.2** | Corrigir os **IDs duplicados** `ML-4A`/`ML-4B` | critério volta a ser rastreável |
| **0.3** | Decidir as **2 ADRs `Proposed`** | nenhuma bloqueia; `Proposed` que sobrevive vira precedente |

🔴 Nada aqui é desenvolvimento. É estado mentindo sobre a realidade.

### Faixa 1 — dar linha de chegada ao que está aberto

**1.1 — 🔴 Congelar o escopo da REQ de Windows.** A decisão mais importante da lista.

Escolher **uma**:
- **(a)** escopo congelado nos **67 rótulos medidos**; sítio novo vai para REQ sucessora; ou
- **(b)** declarar **quais dos 32 MLs** fecham esta REQ e quais ficam adiados, por escrito.

Sem condição terminal escrita, ela continuará aberta na semana que vem. E fechá-la **em silêncio**
com sítios conhecidos é o achado A1 da auditoria externa, que este projeto já pagou uma vez.

**1.2 — Terminar o ratchet** (`ML-2A` em execução, depois `2B` e `3A`). Fecha **#274 + #275** e
destrava o `ML-R2b2`. É a única frente com roadmap decision-complete e ADR aceita.

### Faixa 2 — triar as 27 órfãs em DUAS pilhas, não uma

🔴 **Lista de prioridade com 40 itens não é lista de prioridade.**

As de **16/08 a 21/08** sobreviveram **três semanas** sem roadmap. Isso é evidência sobre a
prioridade real delas, não sobre esquecimento.

Cada uma recebe **um** destino:
- **agendar** — vira roadmap decision-complete;
- **abandonar** — `trackfw roadmap`/REQ para `abandoned`, com o motivo escrito.

**Abandonar é resultado legítimo de auditoria.** Uma REQ que ninguém vai executar em três semanas
custa atenção toda vez que alguém lê a lista.

**Exceção que fura a triagem — segurança já medida:**

```
serve interpola host em string de shell → injeção de comando
Node usa chmodSync no caminho (TOCTOU)
barrier: roadmapTrustForGates falha ABERTO em todo caminho de erro
```

As três estão órfãs desde 30/08–01/09.

### Faixa 3 — corrigir a origem

**3.1** — `req-nasce-orfa` (roadmap em backlog). Com o achado de hoje: **`--from-req` cria o roadmap
e não escreve o vínculo de volta** — 4 de 4 vezes. O escopo da REQ precisa cobrir esse caminho.

**3.2** — Regra de `validate`: REQ `Open` com `roadmap: ""` há mais de N dias vira aviso. Hoje o
`validate` avisa sobre vínculo ausente, **não sobre idade** — e é a idade que distingue "abri agora"
de "esqueci em agosto".

### Faixa 4 — issues sem REQ (os 5 de hoje + #277, #286)

Entram pela **regra de roteamento** da §3: triagem, classificação, agendamento. **Não** anexados à
REQ ativa.

---

## 6. WIP — a regra que estava sendo violada

A regra do projeto é **WIP = 1**. Hoje há **duas branches ativas**:

```
fix/fechar-os-grupos-de-falha-de-windows-por-causa-raiz     R2c2/R2c3 abertos
fix/ratchet-por-nome-e-classe-propria-para-suite-que-...    ML-2A em execução
```

Elas tocam arquivos disjuntos, então não há risco de conflito — **mas duas frentes ativas é
exatamente a forma da sensação que originou esta auditoria.**

**Proposta:** terminar o ratchet (Faixa 1.2), mergear, e **só então** voltar à REQ de Windows com o
escopo já congelado pela Faixa 1.1.

---

## 7. O que NÃO recomendo

**Parar de responder aos issues.** A qualidade dos achados é alta e três deles pegaram defeitos que a
nossa própria auditoria aprovou. O custo não está em ler — está em **executar na hora**.

**Fechar a REQ de Windows sem condição escrita.** É o achado A1 repetido.

**Tratar os 40 itens como fila.** Uma lista de 40 é a ausência de prioridade com aparência de
organização.

---

## 8. Execução da Faixa 0 — 2026-09-10

### 0.1 — 4 REQs fechadas · **40 → 36**

```
REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-...        → Done
REQ-2026-09-02-guard-instalado-emite-schema-de-hook-...              → Done  (7.5.0, PR #297)
REQ-2026-09-05-auditoria-externa-aponta-que-declaramos-...           → Done  (PR #289)
REQ-2026-09-09-update-harness-reescreve-o-script-do-guard-...        → Done  (7.5.1, PR #302)
```

🔴 **Uma delas quase foi fechada errado.** O roadmap da `auditoria-externa` tinha **1 ML pendente**
(`ML-3D — serve casa a aresta pelo caminho literal`), e fechar a REQ assim seria o achado A1 repetido.

Fui verificar **o código**, não o marcador: `internal/serve/api_chain.go:164` já usa
`validator.ResolveRoadmapRef`, entregue no PR **#289** (`e337563d`), nos 3 CLIs. **O ML estava feito e
o marcador desatualizado.** Marcador corrigido com a evidência ao lado, e só então a REQ fechada.

**Lição operacional:** status de ML não é evidência de entrega. Antes de fechar REQ por marcador,
**ler o código**.

### 0.3 — 2 ADRs decididas · `Proposed` → **`Accepted`**

**`ADR-2026-09-05-hook-de-windows-roda-no-windows-...`** — D1 recusa "instale Git Bash" por
aritmética: **resolve 1 CLI de 6**; Gemini, Codex e Copilot continuam sem guard mesmo com ele
instalado. D2: correção por CLI, porque o defeito é por CLI. Raciocínio sólido, **aceita**.

**`ADR-2026-09-05-staging-com-escopo-implicito-...`** — D1 bloqueia `git add` sem escopo enumerado
(`-A`, `--all`, `.`, `-u`, sem operando). D2 🔴 **recusa** condicionar a regra à detecção de agente
ativo, porque *"detecção de atividade de agente é frágil e falha aberto"* — exatamente a classe de
defeito que esta semana custou duas REQs reabertas. **Aceita.**

⚠️ **Consequência a declarar:** quando implementada, `git add .` e `git add -A` passam a ser
bloqueados **para todos, inclusive para o usuário**. Staging válido passa a ser por caminho
explícito. A ADR não muda nada hoje — o custo chega com a implementação, não com o aceite.

```
ADRs:  63 Accepted · 2 Superseded · 0 Proposed
```

### 0.2 — pendente

Correção dos IDs duplicados `ML-4A`/`ML-4B` fica para quando a branch de Windows estiver livre — o
roadmap vive nela, e há microlote em execução na branch do ratchet. **Vai junto com o congelamento de
escopo (Faixa 1.1).**
