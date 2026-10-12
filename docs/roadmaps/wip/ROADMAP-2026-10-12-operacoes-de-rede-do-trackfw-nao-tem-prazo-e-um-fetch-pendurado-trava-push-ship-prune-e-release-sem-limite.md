---
status: wip
date: 2026-10-12
req: "docs/req/REQ-2026-10-12-operacoes-de-rede-do-trackfw-nao-tem-prazo-e-um-fetch-pendurado-trava-push-ship-prune-e-release-sem-limite.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: operações de rede do trackfw não têm prazo e um fetch pendurado trava push, ship, prune e release sem limite

> Created: 2026-10-12 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-12-operacoes-de-rede-do-trackfw-nao-tem-prazo-e-um-fetch-pendurado-trava-push-ship-prune-e-release-sem-limite.md
ADR: docs/adr/ADR-2026-10-12-operacao-de-rede-do-trackfw-tem-prazo-e-o-estouro-nunca-vira-sucesso-silencioso.md
Issue: #556 (fechada por este roadmap — `Closes #556` no PR)

Branch compartilhada com a REQ-2026-10-10 (prune remoto), mesmo objetivo: as operações de remoto do
trackfw terminam e são governadas. **Esta REQ implementa primeiro**: o prune remoto depende do prazo
do fetch (ADR do prune, D4) e as duas tocam `internal/commands/branch_prune*.go`.

## Acceptance Criteria
- [ ] AC1 — Wave 0 auditado (parecer do `hades-tf`)
- [ ] AC2 — executor de produção não bloqueia além de prazo + `WaitDelay`, provado contra neto pendurado, em 3 SOs
- [ ] AC3/AC4 — caminho de falha por ponto; `release.go:310` falha fechada
- [ ] AC5/AC6/AC7 — mensagem, interativo sem prazo, decisão do `git push`
- [ ] AC8/AC9/AC10 — contrato, reconciliação dos testes, `make quality` + CI

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation. Roda em paralelo com o Wave 0 do
> ROADMAP-2026-10-10 (arquivos disjuntos, sem build pesado).

### ML-0A — Threat model do ADR de prazo das operações de rede
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12 · aprovado com ajustes A1–A10 (incorporados ao ADR, D7)
**Files affected:** `docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md` (novo; único arquivo que pode ser escrito)
**Actions:**
1. **Completude da população.** Enumerar toda operação de rede de produção a partir dos
   **executores** (`defaultGitExec` em `ship.go:166`, `defaultGhExec` em `branch_prune_forge.go:56`,
   `defaultExecForgeCLI` em `ship.go:186`, `defaultCheckPROpen` em `ship.go:224`, executor de
   `release.go:229`, `internal/forge/`, `internal/discover/`) e de **todos os chamadores** deles, não
   só dos literais `"fetch"`/`"ls-remote"`/`"push"`. Classificar cada um em leitura de metadados,
   transferência de dados ou interativo (ADR D3).
2. **Mecanismo do travamento (ADR D2).** Em scratch próprio, reproduzir um neto pendurado
   (ex.: `GIT_SSH_COMMAND` apontando para um script que dorme, remoto `ssh://x/y`). Medir:
   - se `exec.CommandContext` sozinho retorna;
   - se `WaitDelay` faz retornar;
   - se o grupo de processos encerra o neto em macOS.
   Declarar o que sobra no Windows. Precedente: `internal/validator/validator_guard_binary_probe.go:138-147`.
3. **Natureza do limite (ADR D3) e valor padrão.**
   - Medir `git fetch origin --prune` e `gh pr list --state all --limit 3000` neste repositório
     (tempo real).
   - Verificar na documentação oficial do git se `http.lowSpeedLimit`/`http.lowSpeedTime` e o keepalive
     de `ssh` (`ServerAliveInterval`) servem de limite **ocioso**, e se dá para aplicá-los **sem
     sobrescrever** `core.sshCommand`/`GIT_SSH_COMMAND` do usuário.
   - Recomendar o limite do `git push` (AC7 da REQ) com razão.
4. **Threat model.** Quem esvazia o prazo sem quebrar regra escrita:
   - `TRACKFW_NETWORK_TIMEOUT` absurdo;
   - hook `pre-push` lento;
   - erro descartado em outro ponto, além de `release.go:310` (procure `, _ :=` e `_, _ =` em chamadas
     de rede).
5. **Alvos de falsificação nas duas direções**, por ponto:
   - o que reprova quando o prazo regride (trava);
   - o que reprova quando regride ao contrário (prazo mata operação legítima; interativo ganha prazo;
     estouro vira sucesso).
6. **Resíduo declarado** e **veredito sobre o ADR** com ajustes numerados (A1, A2, …).
**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (comando + resultado, ou arquivo:linha), não asserção
      ✅ Evidência: `docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md` (população de 11 pontos + `linear.go:76`; mecanismo medido em 4 modos; 1,58 s / 2,96 s). `linear.go:76` conferido pelo arquiteto
- [x] Nenhuma linha de implementação escrita; nenhum arquivo além do parecer alterado
      ✅ Evidência: `git diff --quiet HEAD -- internal/` EXIT=0
- [x] Toda medição em scratch próprio; nenhum `git commit`/`push` bruto (o guard bloqueia, e não se contorna)
      ✅ Evidência: medições com `git init` + `remote add` + `fetch` contra `ssh://` falso; sem commit nem push

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md
grep -q "Veredito" docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md
git diff --quiet HEAD -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditado (✅). ML-1A → ML-1B → ML-1C, sequenciais: ML-1B compartilha os
> arquivos de `internal/commands/` com o ML-1A; ML-1C importa o parser que o ML-1A cria em pacote
> próprio (`internal/netdeadline`), para `internal/sync` não importar `internal/commands`.

### ML-1A — Executor de rede com prazo (ADR D2, D4, D7/A1–A3, A6, A9)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12 após uma devolução (ramo morto de `ErrWaitDelay`; teste do AC6 decorativo). `go build ./...`, `go vet` (inclusive `GOOS=windows`), `go test ./internal/commands/ ./internal/netdeadline/` EXIT=0, rodados pelo arquiteto
**Files affected:** novo pacote `internal/netdeadline/` (parser de `TRACKFW_NETWORK_TIMEOUT`/`TRACKFW_PUSH_TIMEOUT`, padrões, limites; com testes), novo `internal/commands/netexec.go` (classificador e executor), `internal/commands/netexec_unix.go` e `netexec_windows.go` (grupo de processos e terminal de controle, com build tags), `internal/commands/ship.go` (`defaultGitExec`, `defaultCheckPROpen`), `internal/commands/branch_prune_forge.go` (`defaultGhExec`), `internal/commands/release.go` (executor `:229`), testes `internal/commands/netexec*_test.go`.
**Acceptance criteria:**
- [x] `defaultGitExec` único com `classifyGitArgs`: `fetch`/`ls-remote` → `TRACKFW_NETWORK_TIMEOUT` (60 s); `push` → `TRACKFW_PUSH_TIMEOUT` (30 min) + `GIT_HTTP_LOW_SPEED_*` em HTTPS quando o usuário não configurou `http.lowSpeedLimit`; resto sem prazo. Teste do classificador nas duas direções (`-C d fetch` → rede; `commit`/`log`/`rev-parse` → local)
      ✅ `netexec.go`; 6 testes `TestClassifyGitArgs_*` nas duas direções
- [x] Executores de `gh` não interativos com prazo; `defaultExecForgeCLI` sem prazo, com teste (AC6)
      ✅ `runNetworkCmd` em `defaultGhExec`/`defaultCheckPROpen`/`defaultExecForgeAPI`; `TestDefaultExecForgeCLI_AC6_NoDeadline` (dorme 2,5 s com prazo de 1 s; reprova trocando por `runNetworkCmd`)
- [x] `WaitDelay` em todos; `Setpgid` + `cmd.Cancel` matando o grupo só sem terminal de controle (`/dev/tty`), nunca no Windows
      ✅ `netexec_unix.go`/`netexec_windows.go`; `TestGitExec_AC2_GroupKill` 3,07 s, 0 órfãos; `resolveNetworkErr` com `ErrWaitDelay`+`Success()` (3 testes, mordida mostrada)
- [x] Parser: `0`, negativo, > 24 h e texto inválido → aviso e padrão; teste por caso
      ✅ 11 testes em `internal/netdeadline`
- [x] Teste AC2 do executor real: neto = binário de teste reexecutado e dormindo (nunca `cat`), `GIT_SSH_VARIANT=ssh`, prazo de 3 s; afirma ≥ prazo e < prazo + `WaitDelay` + margem; mordida mostrada (sem `WaitDelay`, reprova). Roda em Linux, macOS e Windows
      ✅ `TestGitExec_AC2_WaitDelayOnly` 8,08 s (3 s + 5 s); sem `WaitDelay` → proteção de 15 s reprova (EXIT=1). Ressalva: a mordida e o teste de hook são pulados no Windows; o AC2 roda lá e será visto no CI
- [x] `go build ./...` EXIT=0; `go vet ./internal/commands/` EXIT=0; `go test ./internal/commands/` EXIT=0
      ✅ rodados pelo arquiteto

### ML-1B — Caminho de falha por ponto (ADR D1, D5, D7/A5, A7, A8)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12. Mordidas dos pontos 4 e 6 rodadas pelo arquiteto (o relatório só as descreveu); `make parity-rest` EXIT=0 com Python 3.12 (o agente não achou o venv). Ressalva para o ROADMAP-2026-10-10: `fetchFailed`/`fetchTimedOut` ficam numa cópia de `branchPruneDeps` — o ML-1B de lá deve trocar por variável local de `runBranchPrune`
**Files affected:** `internal/commands/push.go`, `ship.go`, `branch_prune.go`, `release.go`, testes; `docs/cli-parity.md` (seção com `trackfw-contract`)
**Acceptance criteria:**
- [x] Um teste por ponto da população afirmando o caminho de falha do D1 no estouro
      ✅ 9 testes em `netfailpath_test.go` (conferidos por `go test -list`)
- [x] `release.go:310`: falha ou estouro do `ls-remote` → `could not confirm tag … is absent from remote` (AC4)
      ✅ `releaseTagRemoteCheckFailedFmt`; mordida: com o erro descartado, `TestRelease_LSRemote{Error,Timeout}_FailsClosed` reprovam
- [x] `ship.go:801`: o motivo da degradação chega ao usuário (A7)
      ✅ `queryForgePRs` devolve a causa bruta; mordida: sem o aviso, `TestShip_DetectSquashMerges_GhTimeout_WarnsDegradation` reprova
- [x] Estouro nas escritas `release.go:462,482` diz que o estado remoto é incerto (A8)
      ✅ `TestRelease_{TagObject,Ref}PostTimeout_UncertainState`
- [x] Mensagem nomeia operação, prazo e variável (D5)
      ✅ texto do `TimeoutError` nos avisos; mensagem não-timeout idêntica à anterior (paridade)
- [x] `go build ./...` EXIT=0; `go test ./internal/commands/` EXIT=0; `make parity-rest` EXIT=0
      ✅ build, `go test ./internal/commands/` (39,5 s), `GOOS=windows go vet`, `make parity-rest` EXIT=0

### ML-1C — Cliente HTTP do Linear com prazo (ADR D7/A10)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12. Ressalva: o teste de estouro afirma retorno em até 10 s (o handler segura 5 s); a mordida vem de `errors.Is(ErrTimeout)` falso sem a correção, não do tempo
**Por que o escopo original não previa:** o Wave 0 achou `http.DefaultClient` sem prazo em `internal/sync/linear.go:76`. É a mesma causa (cliente de rede sem prazo) por outro mecanismo.
**Files affected:** `internal/sync/linear.go`, `internal/sync/linear_test.go`
**Acceptance criteria:**
- [x] Cliente com o prazo de metadados de `internal/netdeadline`
      ✅ `context.WithTimeout` + `NewRequestWithContext`; estouro → `TimeoutError` + "remote state is uncertain" (A8, é escrita); `TestLinearCreateIssue_{Timeout,Success}`; mordida: com `http.DefaultClient`, `errors.Is(err, ErrTimeout)` falso → FAIL; teste contra `httptest.Server` que não responde termina dentro do prazo e reprova sem a correção
- [x] `go build ./...` EXIT=0; `go test ./internal/sync/` EXIT=0
      ✅ rerodado pelo arquiteto

## Wave 2 — Revisão e fechamento
> Dependencies: Wave 1 deste roadmap **e** do ROADMAP-2026-10-10 (uma revisão para a branch inteira).

### ML-2A — Revisão de qualidade, red-team, `make quality` e CI
**Owner:** `hefesto-tf` e `hades-tf` (em sequência)
**Status:** 🔄 Em andamento — parecer do `hefesto-tf` aprovado com ajustes (`docs/qualidade/2026-10-12-revisao-prazo-de-rede-e-prune-remoto.md`); corretivo ML-2B ✅; red-team do `hades-tf` sem bloqueante (`docs/seguranca/2026-10-12-red-team-prazo-de-rede-e-prune-remoto.md`, com adendo do arquiteto medindo `pushInsteadOf` e injeção por nome); falta `make quality` e CI
**Acceptance criteria:**
- [x] Parecer do `hefesto-tf` em `docs/qualidade/`; parecer de red-team do `hades-tf` em `docs/seguranca/`
- [x] Defeitos achados viram ML corretivo neste roadmap
- [ ] `make quality` EXIT=0 com máquina ociosa; CI do PR verde, inclusive `windows-full-suites`

### ML-2B — Corretivo da revisão de qualidade (Q1, Q2, Q6, Q11)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12; testes rerodados pelo arquiteto
**Por que o escopo original não previa:** achados do parecer de qualidade; defeito conhecido se corrige no mesmo PR.
**Files affected:** `internal/commands/netexec_test.go`, novo teste de varredura em `internal/commands/`, `internal/netdeadline/netdeadline.go` e teste, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] Q1: `TestWaitDelay_ResolveNetworkErr` cobre `ctx.Err()!=nil` junto com `ErrWaitDelay`+sucesso → `(true,false)`; a inversão dos ramos reprova
      ✅ Case 5; mordida (ramos invertidos) → `want (true, false), got (false, true)`
- [x] Q2: teste de varredura reprova `exec.Command`/`CommandContext` de `git` com `fetch`/`ls-remote`/`push`, ou de `gh`, fora dos executores com prazo; mordida mostrada
      ✅ `TestT4_NoDirectNetworkExecCommand` (`go/ast`, todo `internal/`); mordida com 3 chamadas injetadas em `internal/forge/resolve.go` → 3 apontadas; nenhuma chamada de rede existente fora dos executores
- [x] Q6: limite superior do AC2 com margem para o CI de Windows
      ✅ três testes de tempo com +3 s
- [x] Q11: mensagem de estouro em inglês, como as vizinhas; contrato atualizado
      ✅ `"%s timed out after %s (%s)"`; contrato e exemplo do ADR D5 atualizados
- [x] `go test ./internal/commands/ ./internal/netdeadline/ ./internal/sync/` EXIT=0; `make parity-rest` EXIT=0
      ✅ EXIT=0; `make parity-rest` EXIT=0

