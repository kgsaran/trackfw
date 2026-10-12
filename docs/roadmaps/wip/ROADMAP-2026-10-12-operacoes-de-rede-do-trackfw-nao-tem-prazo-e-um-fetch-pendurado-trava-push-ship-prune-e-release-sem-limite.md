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
**Status:** ⬜ Pendente
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
- [ ] As seis seções respondidas com evidência (comando + resultado, ou arquivo:linha), não asserção
- [ ] Nenhuma linha de implementação escrita; nenhum arquivo além do parecer alterado
- [ ] Toda medição em scratch próprio; nenhum `git commit`/`push` bruto (o guard bloqueia, e não se contorna)

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md
grep -q "Veredito" docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md
git diff --quiet HEAD -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditado e ajustes incorporados ao ADR. Arquivos e valores exatos confirmados
> após o parecer. ML-1A e ML-1B são **sequenciais** (compartilham os executores em `internal/commands/`).

### ML-1A — Executor de rede com prazo
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Files affected (provisório):** `internal/commands/ship.go` (`defaultGitExec`, `defaultCheckPROpen`), `internal/commands/branch_prune_forge.go` (`defaultGhExec`), `internal/commands/release.go` (executor `:229`), novo helper `internal/commands/netexec*.go` (+ variantes por SO para o grupo de processos), testes.
**Acceptance criteria:**
- [ ] Prazo nos executores de produção (ADR D2), com `WaitDelay`; `TRACKFW_NETWORK_TIMEOUT` (D4)
- [ ] Teste do executor real contra neto pendurado: termina dentro de prazo + `WaitDelay`; sem o `WaitDelay`, reprova (mordida mostrada)
- [ ] Interativo (`gh pr create` com stdin herdado) sem prazo, com teste
- [ ] `go build ./...` EXIT=0; `go test ./internal/commands/` EXIT=0

### ML-1B — Caminho de falha por ponto e `release.go:310`
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Files affected (provisório):** `internal/commands/push.go`, `ship.go`, `branch_prune.go`, `release.go`, testes; `docs/cli-parity.md` (seção com `trackfw-contract`)
**Acceptance criteria:**
- [ ] Um teste por ponto afirmando o caminho de falha do ADR D1 no estouro
- [ ] `ls-remote` com falha/estouro → falha fechada nomeada no `release tag`
- [ ] Mensagem nomeia operação, prazo e variável (D5)
- [ ] Limite do `git push` conforme o Wave 0, ou razão escrita
- [ ] `go build ./...` EXIT=0; `go test ./internal/commands/` EXIT=0; `make parity-rest` EXIT=0

## Wave 2 — Revisão e fechamento
> Dependencies: Wave 1 deste roadmap **e** do ROADMAP-2026-10-10 (uma revisão para a branch inteira).

### ML-2A — Revisão de qualidade, red-team, `make quality` e CI
**Owner:** `hefesto-tf` e `hades-tf` (em sequência)
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] Parecer do `hefesto-tf` em `docs/qualidade/`; parecer de red-team do `hades-tf` em `docs/seguranca/`
- [ ] Defeitos achados viram ML corretivo neste roadmap
- [ ] `make quality` EXIT=0 com máquina ociosa; CI do PR verde, inclusive `windows-full-suites`
