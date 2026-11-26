---
status: wip
date: 2026-11-26
req: "docs/req/REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: Repositório novo não tem rota governada para publicar a branch padrão e o init não inicializa o git

> Created: 2026-11-26 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md -->
REQ: docs/req/REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] D1: `init` cria o repositório git quando não há work tree
- [ ] D2: `branch new` recusa `HEAD` sem commit; `trackfw commit` aceita só o primeiro commit na branch padrão
- [ ] D3: `trackfw push` publica a branch padrão uma vez, pela URL de push e com lease de ausência
- [ ] D4: feature não sobe para remoto vazio
- [ ] D5: mensagens e templates descrevem o bootstrap; `cli-parity.md` atualizado
- [ ] Replay do fluxo do relato termina com `main` e feature no remoto
- [ ] `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap (hades-tf)
**Status:** ✅ Done — auditado pelo arquiteto. Achados de bloqueio incorporados ao ADR ("Ajustes do Wave 0"): D1 em
quatro estados, predicado de D2, D3 com remoto vazio, todas as URLs de push e um commit, D4 também no `ship`, D6 novo
(guard). Conferi no binário: `gh repo create ... --push`, `git switch --orphan` e `git fetch . feat/x:main` passam
pelo guard com rc=0.
**Files affected:** `docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` (novo);
`vault/notes/bootstrap-repo-novo-achados-wave0-2026-11-26.md` (novo); `vault/notes/index.md`
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
5. Específicos desta REQ, com medição no binário em sandbox (HOME isolado, remoto bare local):
   - `pushurl` divergente de `url`: a consulta pela URL de push fecha? O lease `--force-with-lease=refs/heads/main:`
     é honrado pelo git instalado e recusa quando a branch existe no destino?
   - `gh repo create --source . --push` e `gh repo create ... --push` passam pelo guard git-branch hoje?
   - `commit_unborn_branch_test.go` já trata `HEAD` sem commit: o que ele afirma e como D2 interage?
   - D4 sobre remotos reais: o que `ls-remote --heads` devolve num remoto vazio do GitHub e num remoto com só tags?
   - Censo das superfícies: todo emissor do bloco "Agent Protocol" e toda mensagem que manda usar
     `trackfw branch new` ou `trackfw push` na branch padrão.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Wave gates:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
test -s docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md
```

## Wave 1 — Implementation
> Dependencies: Wave 0 audited. ML-1A, ML-1B e ML-1D têm arquivos disjuntos, mas rodam em sequência num só agente
> (teto de CPU e o mesmo pacote `internal/commands` em ML-1A/1B). ML-1C fecha a wave, porque escreve
> `docs/cli-parity.md` e os templates a partir do comportamento final.

### ML-1A — `init` com `git init`; `branch new` e `commit` no bootstrap (apolo-tf)
**Status:** ✅ Done
**Files affected:** `internal/commands/init.go`; `internal/commands/init_test.go`; `internal/commands/branch.go`;
`internal/commands/branch_test.go`; `internal/commands/commit.go`; `internal/commands/commit_test.go`;
`internal/commands/commit_unborn_branch_test.go`
**Actions:** D1 (quatro estados) e D2 (predicados separados de `commit` e `branch new`) do ADR-2026-11-26, seção
"Ajustes do Wave 0". Relatório: `docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` §6.
**Acceptance criteria:**
- [x] `go test ./internal/commands/ -run 'Init|Branch|Commit' -count=1` rc=0; braço e contra-braço por decisão; frase de
      reconciliação por teste novo
- [x] nenhum arquivo fora de "Files affected" alterado

### ML-1B — `push` e `ship`: publicação única da branch padrão e feature barrada em remoto vazio (apolo-tf)
**Status:** ✅ Done
**Files affected:** `internal/commands/push.go`; `internal/commands/push_test.go`; `internal/commands/ship.go`;
`internal/commands/ship_test.go`; `internal/commands/push_bootstrap_test.go` (novo; acrescentado na auditoria)
**Actions:** D3 (remoto vazio em todas as URLs de push, um commit, lease de ausência, recusa da flag
`--force-with-lease`; substitui o bloqueio em `push.go:143`) e D4 (helper usado por `runPush` e por `runShip`,
`ship.go:417`) do ADR-2026-11-26, seção "Ajustes do Wave 0". Testes com remotos bare em `t.TempDir()`.
**Acceptance criteria:**
- [x] `go test ./internal/commands/ -run 'Push|Ship' -count=1` rc=0; recusam os vetores de D3 (`pushurl` ≠ `url`, várias
      `pushurl`, branch criada entre consulta e push, remoto ausente/inacessível, `--force-with-lease`, `main` com 2
      commits); D4 com remoto vazio recusa em `push` e `ship`; com remoto só `develop`, passa
- [x] nenhum arquivo fora de "Files affected" alterado

### ML-1C — Mensagens, templates e contrato (apolo-tf)
**Status:** ✅ Done
**Files affected:** `internal/commands/barrier.go`; `internal/commands/barrier_ml_scope_test.go`;
`internal/validator/validator_credential_guard_integrity.go`; `internal/generators/agentfiles.go`;
`internal/generators/agentfiles_test.go`; `internal/generators/nextsteps.go`;
`internal/integrations/assets/agents/architect.md`; `docs/cli-parity.md`
**Actions:** D5 do ADR-2026-11-26, sobre a lista de superfícies do censo do Wave 0 (`docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` §1).
**Acceptance criteria:**
- [x] `go test ./internal/commands/ ./internal/validator/ ./internal/generators/ -count=1` rc=0
- [x] nenhum arquivo fora de "Files affected" alterado

### ML-1D — Guard git-branch: `gh repo create --push`, `--orphan`, `symbolic-ref HEAD` (apolo-tf)
**Status:** ✅ Done
**Files affected:** `internal/guard/gitbranch.go`; `internal/guard/gitbranch_test.go`
**Actions:** D6 do ADR-2026-11-26. Braço (rc=0 antes, rc=2 depois) por comando; contra-braço: `gh repo create x
--private` sem `--push`, `git switch feat/x` e `git symbolic-ref --short HEAD` (leitura) continuam com rc=0.
**Acceptance criteria:**
- [x] `go test ./internal/guard/ -count=1` rc=0; frase de reconciliação por teste novo
- [x] nenhum arquivo fora de "Files affected" alterado

### ML-1E — Corretivo da auditoria: instrução de bootstrap compatível com o guard (apolo-tf)
**Status:** ✅ Done — auditado: nenhum emissor com `git add .`, `-A` ou `&&`; `init` imprime os passos.
**Motivo (auditoria do arquiteto, binário da branch):** as instruções de bootstrap mandam
`git add . && trackfw commit ... && trackfw push`. O guard git-branch gerado nega `git add .` (rc=2) e nega o comando
composto inteiro; o agente que seguir a instrução é bloqueado. `git add <caminhos explícitos>` passa (rc=0). E o
`init` que acabou de rodar `git init` não imprime os próximos passos do bootstrap.
**Files affected:** `internal/commands/branch.go`; `internal/commands/push.go`; `internal/commands/ship.go`;
`internal/commands/barrier.go`; `internal/commands/init.go`; `internal/commands/init_test.go`;
`internal/validator/validator_credential_guard_integrity.go`; `internal/generators/agentfiles.go`;
`internal/generators/agentfiles_test.go`; `internal/generators/nextsteps.go`;
`internal/integrations/assets/agents/architect.md`; `docs/cli-parity.md`
**Acceptance criteria:**
- [x] nenhum emissor de instrução de bootstrap contém `git add .`, `git add -A` ou cadeia com `&&`; a instrução é
      `git status --short`, `git add <cada caminho listado>`, `trackfw commit -m "..."` e `trackfw push`, em comandos
      separados
- [x] um teste fixa a ausência de `git add .` no bloco "Agent Protocol" gerado (braço: reprova no texto atual)
- [x] `init` que roda `git init` imprime os passos do bootstrap; teste com braço e contra-braço (repo existente não
      imprime)

### ML-1F — Corretivo da auditoria do ML-1E: lista de arquivos do bootstrap e testes dos vetores (apolo-tf)
**Status:** ✅ Done — auditado pelo replay: o `git add` impresso pelo `init` deixa 0 não rastreados, e a `main` publicada contém `.claude/settings.json`. Os testes de D3 (`pushurl`, várias `pushurl`, lease) e de D6 existem e passam.
**Motivo (auditoria do arquiteto, binário da branch):**
- A instrução fixa `git add trackfw.yaml .gitignore .gitattributes docs scripts vault` deixa de fora o que o `init`
  gera para as ferramentas de IA (`.claude/`, `CLAUDE.md`, `AGENTS.md`...). Medido: depois dela, `git status` mostra
  `?? .claude/`. Como D2 só deixa um commit na `main`, os hooks e as instruções do agente não entram na `main`
  publicada.
- D3 e D6 estão certos no binário, mas sem teste que os fixe: `pushurl` ≠ `url` e várias `pushurl` recusam (rc=1,
  medido), assim como `git checkout --orphan`, `git worktree add --orphan` e `gh repo create ... --push=true` (rc=2,
  medido). Não há teste para nenhum deles, nem para o lease de ausência.
**Files affected:** `internal/generators/nextsteps.go`; `internal/commands/init.go`; `internal/commands/init_test.go`;
`internal/commands/branch.go`; `internal/commands/push.go`; `internal/commands/ship.go`; `internal/commands/barrier.go`;
`internal/validator/validator_credential_guard_integrity.go`; `internal/generators/agentfiles.go`;
`internal/generators/agentfiles_test.go`; `internal/integrations/assets/agents/architect.md`; `docs/cli-parity.md`;
`internal/commands/push_bootstrap_test.go`; `internal/guard/gitbranch_test.go`
**Acceptance criteria:**
- [x] nenhum emissor traz lista fixa de caminhos; as mensagens genéricas dizem "adicione explicitamente cada caminho
      listado por `git status --short`"; o `init` que rodou `git init` imprime a lista real dos caminhos que gerou
      (teste: com `--ai-tools claude`, a lista contém `.claude` e `CLAUDE.md`)
- [x] testes com git real para D3: `pushurl` ≠ `url`, várias `pushurl` (a primeira vazia) e o lease de ausência (a
      branch criada no destino depois da consulta faz o push falhar)
- [x] testes de D6 para `git checkout --orphan`, `git worktree add --orphan` e `gh repo create ... --push=true`

**Wave gates:**
```bash
go build ./...
go test ./internal/commands/ ./internal/validator/ ./internal/generators/ ./internal/guard/ ./internal/integrations/ ./internal/winskips/ -count=1
```

## Wave 2 — Review
> Dependencies: Wave 1 audited. Exceção num guard: revisão de qualidade e red team.

### ML-2A — Revisão de qualidade (hefesto-tf)
**Status:** ✅ Done — aprova com ressalvas (A1–A5); todas corrigidas no ML-2C.
**Files affected:** `docs/seguranca/2026-11-26-revisao-bootstrap-de-repositorio-novo.md` (novo)
**Acceptance criteria:**
- [x] revisão escrita; nenhum achado bloqueante aberto

### ML-2B — Red team da exceção de bootstrap (hades-tf)
**Status:** ✅ Done — reprovou por D3-bypass; corrigido no ML-2C e conferido no binário (Forma 2 rc=1, sem `main` no destino).
**Files affected:** `docs/seguranca/2026-11-26-red-team-bootstrap-de-repositorio-novo.md` (novo)
**Acceptance criteria:**
- [x] vetores do Wave 0 medidos antes e depois no binário; nenhum escape aberto

### ML-2C — Corretivo da Wave 2: achados do Hefesto (A1–A5) e do Hades (apolo-tf)
**Status:** ✅ Done — auditado no binário: Forma 2 recusa citando `insteadOf`; contra-braço publica; `git add --` impresso não executa `$(touch PWNED)` e adiciona `-A` literal; guard `--push=false` rc=0, `bash -c 'gh repo create x --push'` rc=2.
**Motivo:** o Hefesto aprovou com ressalvas (A1 médio, A2–A5 baixos). O Hades reprovou por D3-bypass (médio): com
`insteadOf` encadeado, ou com `pushInsteadOf` + `insteadOf`, `ls-remote --heads <push_url>` reescreve a URL uma segunda
vez. A consulta vê um remoto vazio, e o push chega a outro remoto, que já tem branches. Também apontou quatro achados
baixos: D1-6, D6-FP, D6-wrapper e M-X1. Decisão do arquiteto: corrigir na origem (opção A do Hades), não declarar
residual.

Residuais declarados pelo arquiteto no ML-3A, na ADR (não são deste ML):
- TOCTOU de ref com outro nome;
- escrita de git por hook ou script fora do PreToolUse (classe do §4 do Wave 0);
- alias via `-c alias.*` ou `git config alias.*` (classe Residual D5 do guard);
- falso positivo do `insteadOf` em cascata legítima;
- aspas do PowerShell em nome de arquivo com `'`.

**Files affected:** `internal/commands/push.go`; `internal/commands/push_bootstrap_test.go`; `internal/commands/push_test.go`;
`internal/commands/ship.go`; `internal/commands/ship_test.go`; `internal/commands/init.go`; `internal/commands/init_test.go`;
`internal/generators/nextsteps.go`; `internal/commands/commit.go`; `internal/commands/commit_test.go`;
`internal/commands/branch.go`; `internal/commands/branch_test.go`; `internal/guard/gitbranch.go`; `internal/guard/gitbranch_test.go`
**Acceptance criteria:**
- [x] D3-bypass: `checkRemoteEmpty` roda `git ls-remote --get-url <u>` para cada URL de push. Se o resultado
      (sem espaços nas pontas) diferir de `<u>`, devolve erro com uma mensagem que cita `insteadOf`. Como D3 é
      fail-closed, o push da `main` recusa. Há testes com git real para a Forma 1 (`pushInsteadOf` + `insteadOf`) e
      a Forma 2 (`insteadOf` encadeado): rc≠0, a mensagem cita `insteadOf`, e o remoto com branches não recebe a
      `main`. Contra-braço: um remoto vazio sem `insteadOf`, e um `pushurl` explícito sem `insteadOf`, continuam
      passando.
- [x] A1: o teste do lease de ausência discrimina o lease. Ele exige, na saída do push, `stale info` (rejeição do
      lease), e não `fetch first` nem `non-fast-forward`. O contra-braço é medido e escrito no relatório: sem o
      lease, a mesma rejeição vem com outra mensagem.
- [x] A2: a linha `_ = generators.PrintBootstrapSteps` some de `init_test.go`.
- [x] A3: a dica de D4, hoje duplicada em `push.go` e `ship.go`, vira um helper único, `bootstrapStepsHint()`.
- [x] A4 e A5: o comentário de `parseLSRemoteHeads` deixa de descrever um caso impossível. Os comentários de
      `init.go` dizem que o `git init` roda depois do scaffold.
- [x] D1-6: a lista de caminhos vem de `git status -z --porcelain`, separada por NUL. Cada caminho é impresso entre
      aspas simples POSIX (`'` vira `'\''`), e a linha começa com `git add -- `. Teste: com os arquivos `$(touch PWNED)`
      e `-A` no diretório, a linha impressa, executada com `sh -c`, não cria `PWNED` e adiciona os dois arquivos
      literalmente.
- [x] D6-FP: `gh repo create x --push=false`, `--push=0`, `--push=no` e `--push=off` são permitidos.
      `--push`, `--push=true`, `--push=1`, `--push=yes` e qualquer outro valor são bloqueados (fail-closed).
- [x] D6-wrapper: `matchSubcommandPOSIX` aplica `applyGhRule` também a `gh`.
      `bash -c 'gh repo create x --push'` dá rc=2; `bash -c 'gh repo view'` dá rc=0.
- [x] M-X1: a recusa do `commit` na `main`/`master`, quando há commit, e a recusa do `branch new` com HEAD unborn
      incluem uma saída a mais: `git switch <existing-branch>`, ou o passo de bootstrap que faltar.
      Testes afirmam o texto.

### ML-2D — Corretivo do `make quality`: goldens do arquiteto e skips de Windows (apolo-tf)
**Status:** ⬜ Pending
**Motivo:** o `make quality` deu rc=2, com duas falhas.
- `TestRenderWithoutIdentityMatchesFrozenGoldens` e `_CRLFSource` falham. O ML-1E mudou
  `internal/integrations/assets/agents/architect.md` e não atualizou os goldens; o gate da Wave 1 não incluía
  `internal/integrations`.
- `TestGate_RealTree` (winskips) falha. O ML-2C pulou 4 testes no Windows sem registrar.

**Files affected:** `internal/integrations/testdata/architect.subagent.golden.md`;
`internal/integrations/testdata/architect.agent-directory.golden.md`; `internal/commands/push_bootstrap_test.go`;
`internal/commands/init_test.go`; `.github/windows-skips.json`
**Acceptance criteria:**
- [ ] os dois goldens refletem o `architect.md` atual (só as linhas do bootstrap mudam);
      `go test ./internal/integrations/ -count=1` rc=0
- [ ] `TestD3Bypass_*` deixam de pular no Windows. A URL `file://` é montada de forma portável
      (`file:///` + caminho com `/`, sem a barra inicial duplicada).
- [ ] `TestD1_6_RealGit_ShellInjectionSafe` deixa de pular por `runtime.GOOS`. O pulo por ausência de `sh` continua.
      Se ainda sobrar pulo de Windows, ele é registrado em `.github/windows-skips.json` com evidência.
- [ ] `go test ./internal/winskips/ ./internal/commands/ -count=1` rc=0

## Wave 3 — Gate final

### ML-3A — Replay, `make quality` e CI (zeus-tf)
**Status:** ⬜ Pending
**Files affected:** este roadmap; `docs/req/REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md`; `docs/adr/ADR-2026-11-26-bootstrap-de-repositorio-novo-init-roda-git-init-e-a-branch-padrao-e-publicada-uma-vez-pelo-trackfw.md`;
`docs/adr/ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md`;
`docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md`
(notas de emenda)
**Acceptance criteria:**
- [ ] replay do fluxo do relato no binário da branch; `make quality` rc=0; CI verde
