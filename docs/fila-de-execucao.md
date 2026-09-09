# Fila de execução

> Ordem acordada com o usuário em 2026-09-06. **Uma frente por vez (WIP=1).**
> Este arquivo é a ordem; o **estado real** vive nos roadmaps (`docs/roadmaps/<estado>/`) e no
> `trackfw serve`. Se os dois divergirem, **o roadmap manda** — aqui é intenção, lá é fato.

## Regras que governam esta fila

1. 🔴 **Terminar o que está em andamento antes de começar o próximo.** Trocar de frente no meio já
   produziu bagunça neste projeto — decisão do usuário, 2026-09-06.
2. 🔴 **Segurança fura a fila.** Achado interno de segurança não espera issue.
3. **Issue antes de achado interno** (não-segurança): *um issue tem alguém do outro lado esperando;
   um achado nosso, não.* E o consumidor externo é hoje a nossa única fonte independente de
   descoberta.
4. 🔴 **Descoberta nova de mesma causa fecha no MESMO roadmap.** Mesmo sintoma investiga junto;
   separar exige **medição escrita**. Ver `CLAUDE.md`, Regra Dura de Causa Raiz.

## A fila

| # | O quê | Tipo | Artefato | Fecha | Estado |
|---|---|---|---|---|---|
| **0** | Reconciliação pós-auditoria (3 MLs) | concluído | `ROADMAP-2026-09-05-reconciliar-...` | — | ✅ **PR #289 mergeado** |
| **1** | 🔴 **Guard emite schema antigo — Claude Code rejeita, a razão do bloqueio se perde** | **degrada o uso AGORA** | `REQ-2026-09-02-guard-instalado-emite-schema-de-hook-...` | — | ⬜ |
| **2** | `barrier`: `roadmapTrustForGates` fail-open em todo caminho de erro | 🔴 segurança | `REQ-2026-08-30-barrier-executa-gate-...` | — | ⬜ |
| **3** | `serve` interpola host em string de shell → injeção de comando | 🔴 segurança | `REQ-2026-09-01-serve-interpola-host-...` | — | ⬜ |
| **4** | Node usa `chmodSync` no caminho em vez de `fchmodSync` no descritor (TOCTOU) | 🔴 segurança | `REQ-2026-09-01-cli-node-usa-chmodsync-...` | — | ⬜ |
| **5** | `validate_unfiltered` + `validate --json` do Python (mesma função) | issue | `REQ-2026-09-05-validate-unfiltered-...` + `REQ-2026-08-20-validate-json-...` | **#261** | ⬜ |
| **6** | CI distingue "suíte não carregou" de "teste reprovou" + ratchet por nome | issue | ADR 🔴 **Proposed** (não aceita) + `REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-...` 🔴 **órfã** · **bloqueado por `ML-R2`** | **#274 · #275** | ⬜ |
| **7** | `status` do Python conta REQ por listagem flat | issue | `REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-...` | **#268** | ⬜ |
| **8** | Gate de palavra-chave: evento `edited` + contrato para exemplo citado | issue | `REQ-2026-09-05-gate-de-palavra-chave-...` | **#258** | ⬜ |
| **9** | `branch_has_wip_roadmap` erra nas duas direções | issue · **decisão** | `REQ-2026-08-20-branch-has-wip-roadmap-...` | **#273** | ⬜ |
| **10** | Corpus do barrier-contract acoplado à governança do repo | issue · dívida | `REQ-2026-09-03-check-gates-falsify-...` | **#277** | ⬜ |
| **11** | Windows: hooks nativos (`.ps1`) — desenho já medido | interno | `ADR-2026-09-05-hook-de-windows-roda-no-windows-...` + REQ | — | ⬜ |
| **12** | Windows: falhas restantes — **número desatualizado**, ver correção abaixo | interno | `ML-R2` da `REQ-2026-09-03-as-217-falhas-reais-...` | — | ⬜ |
| **13** | Windows: jornada de instalação (README, `install.sh`, ARM64) | interno | `REQ-2026-09-05-a-instalacao-em-windows-...` | — | ⬜ |
| **14** | Guard de `git add -A` (staging com escopo implícito) | interno | `ADR-2026-09-05-staging-com-escopo-implicito-...` + REQ | — | ⬜ |

**Fora da fila, aguardando terceiros:** `#276` (premissa refutada por medição, comentado com a
análise — depende de resposta do autor).

### Por que o item 1 furou a fila, na frente até da segurança

Reproduzido **ao vivo na sessão do usuário**, 2026-09-06: todo comando bloqueado pelo guard produz
`Hook JSON output validation failed — (root): Invalid input`.

```
emitimos    {"decision":"block","reason":"..."}                       <- schema antigo
esperado    {"hookSpecificOutput":{"hookEventName":"PreToolUse",
                                   "permissionDecision":"deny",
                                   "permissionDecisionReason":"..."}}
```

🔴 **O bloqueio funciona — o que se perde é a razão.** O usuário vê `Invalid input` em vez de
*"use `trackfw push`"*. E `hookSpecificOutput` **não existe em lugar nenhum do repositório**, nos 3
CLIs.

**Um guard que bloqueia sem explicar é um guard que o usuário desliga.** É a mesma preocupação que a
barreira levantou no fail-open — só que aqui não é hipótese, está acontecendo.

## Como acompanhar

O estado autoritativo é o do trackfw, não desta tabela:

```bash
trackfw status      # REQs e roadmaps por estado
trackfw serve       # board ao vivo em http://localhost:4080
gh issue list       # issues abertos
```

**Atualizar esta tabela** ao mover um item: marcar 🔄 ao iniciar, ✅ ao fechar, e registrar o PR.
Se um item crescer durante a execução — descoberta de mesma causa —, **ele NÃO vira linha nova**:
entra como ML no roadmap daquele item, pela regra 4.


## Atualização — 2026-09-08

**Fechado desde a última revisão da fila:**

| frente | resultado |
|---|---|
| Reconciliação pós-auditoria | ✅ PR **#289** mergeado |
| Aceleração do `parity` | ✅ PR **#291** mergeado — job de **20m41s → 15m00s** (−27%) |
| Gates rodam no Windows | ✅ REQ encerrada — censo **440 OK / 512 FAIL**, antes invisível |
| Separador no `update --json` (#292) | ✅ REQ reaberta e encerrada |
| Guarda `IsAbs` (segurança) | ✅ REQ reaberta, ML entregue |

🔴 **O item 1 continua `⬜` — e é o que degrada o uso todo dia.** Passamos a sessão em frentes
legítimas, quase todas vindas de issues externos, mas nenhuma delas era esta. **Próxima branch.**

**Entrou na fila, com artefato e endereço:**

| # | o quê | artefato |
|---|---|---|
| — | Triagem dos **512** por causa (pré-requisito do ratchet de CI em Windows) | `ML-R2`, REQ das 217 falhas |
| — | Guard `default:` com gramática só-POSIX (segurança, contido) | `ML-R3`, REQ das 217 falhas |
| — | Os 5 grupos restantes da triagem dos 12 do CI | REQ das 217 falhas |
| — | `validate` imprime usage / suja o `--json` (#290) | roadmap próprio, em `backlog` |
| — | Gate dos pins de call site · matriz de jobs · outros 45 gates | roadmap do `parity`, em `wip` |


## Item 1 — artefatos criados, 2026-09-09

A REQ existia desde 02/09 com `roadmap: ""` — **órfã**, o mesmo estado em que eu deixei a REQ do
issue #290. Roadmap criado e vinculado:

`ROADMAP-2026-09-09-guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-nos-3-clis.md` (backlog)

**Sítios medidos antes de escrever o roadmap** (não presumidos): o script real, os **3 geradores**
(Go/Node/Python), as **2 referências do `validate`**, e um teste. `hookSpecificOutput` tem **zero**
ocorrências no repositório. O corpus congelado de testdata está marcado como **não tocar**.

🔴 **Por que este item perdeu seis vezes seguidas, e o que fazer com isso:** *issue externo tem alguém
do outro lado esperando; atrito próprio não tem ninguém cobrando.* Não é falha de priorização
pontual — é viés estrutural da fila. A contramedida não é subir o item de posição, é **decidir
explicitamente** começar por ele quando a frente anterior fecha.


## Correção de dois itens da fila — arquiteto, 2026-09-09

Medidos, não presumidos. **Os dois estavam errados na própria fila, e um dos erros era meu de ontem.**

### Item 6 — a fila afirmava "ADR aceita"; ela está `Proposed`

```
ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-...   status: Proposed
REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-...           status: Open · roadmap: ""
```

🔴 **Duas coisas erradas de uma vez:** a ADR não está aceita, e a REQ é **órfã** — terceira órfã da
mesma família em dois dias (as outras: issue #290 e o item 1). O padrão é consistente: **abrir o
artefato dá a sensação de ter encerrado o assunto.**

🔴 **E ele está bloqueado por dependência real, não por prioridade:** o item 6 entrega *ratchet por
nome*, e um ratchet precisa da **lista de nomes triada por causa**. Essa lista é o `ML-R2` (triagem
dos 512). Sem ele, o ratchet nasce com 512 entradas — que é `continue-on-error` com passos extras, e
foi exatamente a crítica do autor do issue #275.

**Ordem correta:** `ML-R2` → item 6. Despachar o item 6 antes seria entregar o mecanismo sem o dado
que o torna útil.

### Item 12 — "39 falhas restantes" está desatualizado

```
antes (retriagem de 04/09)   39 falhas
hoje, medido                 ~26 no CI (15 Go + 11 Node, run da main)
                             + 512 no censo do gate que nunca rodava no Windows
```

O número **subiu** porque passamos a medir uma superfície que nunca havia sido exercitada — **subir
é acerto, não regressão**. Mas a linha da fila dizia 39 e induzia a estimativa errada.

**Substituído pelo `ML-R2`**, que é o pré-requisito real: triagem por causa. A campanha anterior
colapsou **246 sintomas em 6 causas**; sem a mesma triagem, 512 é número e não escopo.

### Próximo depois do item 1

**Item 2** — `barrier`: `roadmapTrustForGates` **fail-open** em todo caminho de erro.

🔴 Escolhido por **natureza**, não por ordem numérica. Um `barrier` que falha aberto significa que a
barreira que autoriza as waves pode aprovar **por falha, não por verificação** — a mesma forma do
defeito que custou duas REQs reabertas nesta campanha, mas no componente que decide se o resto pode
prosseguir. Os itens 3 e 4 são reais e têm pré-condição (host malicioso, janela de corrida); este
não precisa de adversário.
