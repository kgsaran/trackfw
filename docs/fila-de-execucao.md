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
| **6** | CI distingue "suíte não carregou" de "teste reprovou" + ratchet por nome | issue | `ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-...` (**aceita, sem roadmap**) | **#274 · #275** | ⬜ |
| **7** | `status` do Python conta REQ por listagem flat | issue | `REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-...` | **#268** | ⬜ |
| **8** | Gate de palavra-chave: evento `edited` + contrato para exemplo citado | issue | `REQ-2026-09-05-gate-de-palavra-chave-...` | **#258** | ⬜ |
| **9** | `branch_has_wip_roadmap` erra nas duas direções | issue · **decisão** | `REQ-2026-08-20-branch-has-wip-roadmap-...` | **#273** | ⬜ |
| **10** | Corpus do barrier-contract acoplado à governança do repo | issue · dívida | `REQ-2026-09-03-check-gates-falsify-...` | **#277** | ⬜ |
| **11** | Windows: hooks nativos (`.ps1`) — desenho já medido | interno | `ADR-2026-09-05-hook-de-windows-roda-no-windows-...` + REQ | — | ⬜ |
| **12** | Windows: 39 falhas restantes, mapeadas por mecanismo | interno | `docs/portabilidade/2026-09-04-retriagem-...` | — | ⬜ |
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
