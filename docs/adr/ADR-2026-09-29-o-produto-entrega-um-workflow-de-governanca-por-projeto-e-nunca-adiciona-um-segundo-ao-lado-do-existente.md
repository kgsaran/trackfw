---
status: Accepted
date: 2026-09-29
author: "trackfw_architect"
---

# ADR: o produto entrega UM workflow de governança por projeto, e nunca adiciona um segundo ao lado do existente

> Date: 2026-09-29 | Status: Accepted

## Context

O trackfw escreve **dois** workflows de CI que executam o **mesmo** `trackfw validate`:

| workflow | escrito por | instala com | job id (= nome do check) |
|---|---|---|---|
| `trackfw-gate.yml` | `init` / `update` (`generateGitHubActionsWorkflow`) | `curl \| sh` + `TRACKFW_VERSION` | `governance-install-script` |
| `trackfw-validate.yml` | `discover --init` (`InstallGates`) | `go install @vX.Y.Z` via `setup-go@v7` | `governance-go-install` |

Um consumidor externo (**#451**, repositório privado em plano free, 2.000 min/mês) mediu **duas
execuções** do mesmo veredito por evento, cada uma pagando o próprio `checkout` + instalação do
binário. Este projeto já havia descartado job-splitting por medição, com o motivo escrito no topo do
`quality-gates.yml`: *"Cota é a causa-raiz desta família de defeitos."* Entregar o veredito duplicado
contraria a nossa própria decisão.

### 🔴 Esta decisão nunca havia sido tomada — e uma citação falsa fingia que sim

Seis sítios do repositório afirmavam que *"a coexistência dos dois arquivos está decidida na
`ADR-2026-08-28`"*. **Falso**, medido: aquela ADR tem **zero** ocorrências de `trackfw-validate.yml`
— ela decide pino de versão e `TRACKFW_VERSION`. A citação nasceu num comentário de código
(`scaffold_doctor.go:334`) e propagou para um comentário de teste, dois pareceres e — o caso mais
caro — para o **escopo negativo da `REQ-2026-09-28`**, onde serviu para concluir que o #451 seria
"REQ própria". Uma citação que ninguém conferiu produziu uma decisão de governança errada, na regra
que existe para impedir trabalho paralelo sobre a mesma causa.

Esta ADR existe porque **não havia ADR**. Nota de vault:
`vault/notes/governanca-citacao-de-adr-em-comentario-de-codigo-2026-09-29.md`.

### O que a Wave 0 mediu (2026-09-29)

1. **A janela exclusiva existe, mas é transiente.** `discover --init` em projeto sem `trackfw.yaml`
   escreve só o `validate.yml` — e grava `ci: github-actions`, então o **primeiro `update` escreve o
   `gate.yml`** e fecha a janela. Não há adotante permanentemente atendido por apenas um.
2. 🔴 **Os dois NÃO são equivalentes: os job ids diferem**, e job id é o nome do check.
   `.github/required-status-checks.txt:29-30` declara **os dois**. Remover um workflow **quebra a
   branch protection** de quem configurou aquele nome — e o trackfw não tem como saber, lendo o
   repositório, o que está no `required_status_checks` da API.
3. **O instalador não discrimina público.** `actions/setup-go@v7` provisiona o toolchain; não há
   runner gerenciado do GitHub em que o `go install` falhe por ausência de Go. O argumento
   *"atendem públicos diferentes"* não se sustenta no ambiente onde os workflows rodam.

**Conclusão:** a coexistência não tem razão de produto. É sedimentação histórica.

## Decision

**D1 — Um workflow de governança por projeto.** O canônico é **`trackfw-gate.yml`**: instala com
`TRACKFW_VERSION` pinado (o que a `ADR-2026-08-28` de fato decide), declara
`timeout-minutes: 10` e não exige toolchain Go no runner.

**D2 — 🔴 O gerador pergunta antes de escrever.** `generateGitHubActionsWorkflow` **não escreve** o
`trackfw-gate.yml` quando `.github/workflows/trackfw-validate.yml` já existe. O caminho cuidadoso já
existia no código — `refreshDiscoverGitHubActionsWorkflowIfPresent` só **atualiza** o que encontra e
nunca cria. **A assimetria era o defeito**: um lado pergunta, o outro não.

**D3 — Nada é removido automaticamente.** Quem já tem os dois instalados **mantém os dois**. O
`doctor` **avisa**, e a remoção é escolha do consumidor.

> Por que não remover: o nome do job removido pode ser um required check. Apagar por nós quebraria o
> merge protection de quem adotou, sem que o produto tenha como verificar. 🔴 **Trocar duplicação por
> merge protection quebrada é um defeito pior que o que se corrige** — e a assimetria de
> consequência é o que decide: duplicação custa cota, branch protection quebrada deixa passar código
> não validado.

**D4 — O `doctor` acompanha na MESMA entrega.** Hoje ele cobra o `trackfw-gate.yml` sempre que
`ci: github-actions`. Pular a escrita sem ajustar o `doctor` trocaria uma duplicação por um **achado
falso**. Novo comportamento: com o `validate.yml` presente e o `gate.yml` ausente, **não** acusa
ausência; com **os dois** presentes, emite o aviso de migração do D3.

**D5 — Comentário de código não cita ADR sem que a ADR decida aquilo.** Os 6 sítios são corrigidos.
O sítio em comentário de **teste** (`discover_workflow_trigger_test.go:17`) é obrigatório: um teste
cujo comentário afirma decisão inexistente viola a Regra Dura de Reconciliação assim que o produto
mudar.

## Consequences

**Positivas**
- Projeto novo recebe **um** veredito por evento — fecha o #451 na direção da cota.
- A assimetria do gerador desaparece: os dois sítios de escrita passam a olhar o disco antes.
- O `doctor` deixa de ser cego para a duplicação: passa a nomeá-la.
- Existe, finalmente, uma decisão **escrita** sobre quantos workflows o produto entrega.

**Negativas, declaradas**
- 🔴 **Quem já tem os dois continua pagando 2 execuções até agir.** O D3 troca alívio automático por
  segurança do required check. É consciente, e é a metade do #451 que esta ADR **não** fecha sozinha.
- O `validate.yml` permanece como artefato vivo que o produto sabe atualizar e não sabe criar —
  assimetria remanescente, agora **decidida** em vez de acidental.
- O `go install` deixa de ser exercitado no consumidor. Cobrir os dois métodos de instalação passa a
  ser responsabilidade do CI **deste** repositório, onde já é.

**Residual declarado**
- Não decidimos quais checks entram no `required_status_checks` de cada consumidor — não é nosso, e
  é justamente por isso que o D3 não remove nada.
- Se um segundo par de artefatos divergentes aparecer (literal embutido vs. cópia versionada de
  workflow), a guarda de paridade se justifica. Hoje é um par só; a dívida está declarada no roadmap
  da `REQ-2026-09-02`.

## Alternatives Considered

**A — `update` remove o workflow obsoleto automaticamente.** Resolveria a cota de todos, sem ação do
consumidor. **Rejeitada:** quebra a branch protection de quem configurou o job id removido, e o
produto não tem como verificar isso lendo o repositório. Custo do erro é assimétrico.

**B — Manter `trackfw-validate.yml` como canônico.** Ele cobre `push` em `main` além de
`pull_request`, e `governance-go-install` é o nome mais visto em required checks. **Rejeitada:**
exige toolchain Go e não declara `timeout-minutes`; o pino de versão via `TRACKFW_VERSION` é o que a
`ADR-2026-08-28` decide, e é o `gate.yml` que o implementa.

**C — Um arquivo com os dois jobs.** Preservaria os dois nomes de check. **Rejeitada:** não reduz
execução nenhuma — dois jobs são dois runners, dois checkouts, duas instalações. Resolveria a
contagem de arquivos, que não é o problema.

**D — Fechar a REQ documentando a coexistência como legítima.** Era o desfecho previsto pelo AC1 se
houvesse razão de produto. **Rejeitada pela medição:** a janela exclusiva é transiente e o argumento
do instalador não se sustenta.

## Linked REQ

REQ: `docs/req/REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md`
