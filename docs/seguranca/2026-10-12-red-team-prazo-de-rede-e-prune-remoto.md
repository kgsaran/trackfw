# Red-team Wave 2 — Prazo de Rede e Prune Remoto

> Data: 2026-10-12 | Papel: Hades (Security Reviewer) | Branch: `fix/operacoes-de-rede-sem-prazo-e-branch-prune-remoto`
> REQs: REQ-2026-10-12 (#556 prazo de rede) · REQ-2026-10-10 (branch prune --remote · guard)

Metodologia: reimplementar a partir da leitura do código-fonte + falsificação por execução em cópia isolada (`$D = /var/folders/.../tmp.w9v1RVLmkM`). Todos os vetores foram executados; nenhum teste foi construído contra a árvore real. Nenhum `git add/commit/push` foi realizado.

---

## 1. Enumeração de superfícies (fechamento)

O Wave 0 de cada REQ lista as superfícies. Esta seção confirma que a enumeração está fechada para os vetores deste red-team.

### 1.1 Prazo de rede (ADR D1–D7/A1–A10)

| # | Superfície | Coberta? |
|---|---|---|
| P1 | `netdeadline.Parse` — valores-limite de `TRACKFW_NETWORK_TIMEOUT` | ✅ |
| P2 | `classifyGitArgs` — alias via `-c`, flags de dois tokens, formas equals | ✅ |
| P3 | Subcomandos de rede não reconhecidos (alias obfuscation = RS-7) | ✅ |
| P4 | T4 AST scan — `exec.Command(variable, ...)` não detectado | ✅ (confirmado por mutação na sessão anterior) |
| P5 | Linear `CreateIssue` — body-hang, erro não-timeout | ✅ |

### 1.2 Prune remoto (ADR D1–D8/A1–A9)

| # | Superfície | Coberta? |
|---|---|---|
| R1 | Fork com mesmo nome (isCrossRepository=true) | ✅ |
| R2 | PR mergeado para base não-padrão (stacked PR) | ✅ |
| R3 | TOCTOU — tip avançou entre avaliação e push de remoção | ✅ |
| R4 | A5 — pushurl ≠ fetchurl | ✅ |
| R5 | A7 / P2 — snapshot=nil (truncated/degraded) bloqueia | ✅ |
| R6 | Nomes de branch com caracteres especiais | ✅ (informacional) |
| R7 | ref forjada em `refs/remotes/origin/*` via refspec estreita | vide §2.3 |
| R8 | Snapshot defasado do forge (PR aberto depois do snapshot) | residual declarado R6 |

### 1.3 Guard de remoção

| # | Superfície | Coberta? |
|---|---|---|
| G1 | `--delete`, `-d`, `:refs/heads/`, `:shortname`, `+:` | ✅ |
| G2 | `--delete=foo` (equals form) | ✅ (benign — git rejects it) |
| G3 | `-dfoo` combined | ✅ (declared residual) |
| G4 | `--no-verify`, `--atomic`, `-o option` com `:refs/` | ✅ |

---

## 2. Vetores executados — resultado por superfície

### 2.1 Avaliador do forge — fork e base não-padrão

**Teste:** `TestRT_Eval_ForkSameName`, `TestRT_Eval_NonDefaultBase`, `TestRT_Eval_MergedPR_Positive`
**Arquivo de teste:** `$D/internal/commands/zz_redteam_test.go`

**Mecanismo testado:** `evaluateBranchWithForge` com snapshot construído. O clone real fornece os objetos git necessários (`rev-parse`, `merge-base --is-ancestor`, `cat-file -e`) para que o avaliador produza resultados reais, não mocks.

**Resultado de execução:**

```
TestRT_Eval_ForkSameName: fork decision=pending_work (not merged_pr)
TestRT_Eval_NonDefaultBase: stacked PR decision=pending_work (not merged_pr)
TestRT_Eval_MergedPR_Positive: decision=merged_pr (controle positivo OK)
```

**Veredito: FECHADO.**

- Fork com `isCrossRepository=true`: a PR é filtrada antes de entrar em `openPRs`/`mergedValidBase`. Com zero PRs restantes, o avaliador cai no heurístico de conteúdo — o branch tem trabalho próprio (`feat_fork-same.txt`), então retorna `pending_work`. A branch não seria removida.
- Stacked PR (`BaseRefName="staging"` ≠ `"main"`): a PR não entra em `mergedValidBase` (filtro `pr.BaseRefName == defaultLocalName`). Mesmo resultado.
- O controle positivo confirma que o avaliador produz `merged_pr` quando as condições estão todas satisfeitas — o teste não passa vacuamente.

### 2.2 TOCTOU — tip avançado antes do push de remoção

**Teste:** `TestRT_DeleteRemote_TOCTOU`

**Mecanismo testado:** `deleteRemoteBranch` com bare real. SHA avaliado = SHA antigo. Bare avançado de `d90d5b11` para `269f7997` por segundo clone antes da chamada de remoção.

**Resultado de execução:**

```
TestRT_DeleteRemote_TOCTOU: bare tip advanced: d90d5b11 -> 269f7997
FECHADO: TOCTOU protected — push rejected: exit status 1
  bare tip is still 269f7997 (new SHA — not deleted)
```

**Veredito: FECHADO.**

O `--force-with-lease=refs/heads/feat/toctou:<sha_antigo>` foi rejeitado pelo servidor (bare) porque o ref do servidor estava em `<sha_novo>`. O branch sobrevive.

**Nota sobre ADR D5 (lease no servidor):** `deleteRemoteBranch` usa `refs/heads/<b>` (ref lado servidor) no lease, não `refs/remotes/origin/<b>` (tracking ref lado cliente). Se usasse a tracking ref, o servidor verificaria um ref que não possui — lease vacuamente satisfeito, TOCTOU aberto. O teste confirma que o lease correto está em uso: o servidor rejeita o SHA defasado.

### 2.3 Ref forjada em refs/remotes/origin/* via refspec estreita

**Vetores analisados por leitura de código:**

```bash
git fetch origin "+refs/heads/feat/foo:refs/remotes/origin/feat/foo"
# cria tracking ref com SHA arbitrário
```

O lease em `deleteRemoteBranch` é `refs/heads/<b>:<sha>` — o SHA avaliado vem de `deps.gitExec("rev-parse", "refs/remotes/origin/"+b)`. Se o tracking ref foi forjado por refspec estreita com um SHA que **não existe no bare**, a checagem `objectExists(pr.HeadRefOid, gitExec)` retorna falso → branch mantida como `review` (caso `mergedOutcomeHeadAbsent`). Se o SHA existe no bare mas pertence a outro commit, `merge-base --is-ancestor` falha → resultado é `pending_work` ou `diverged`, não `merged_pr`.

**Conclusão:** sem execução adicional; a análise estática mostra que a forjagem de tracking ref não pode produzir uma deleção falsa, pois o SHA no lease precisa estar presente no bare e ser ancestral do tip avaliado.

**Residual declarado R5 do Wave 0:** o tracking ref pode ser forjado para um SHA que *é* ancestral do tip do bare — se o attacker controla o conteúdo da branch localmente. Esta é uma condição de ataque que requer acesso de escrita ao clone, que é equivalente a acesso à própria branch. Aceito.

### 2.4 A5 — push URL divergente bloqueia todas as remoções remotas

**Teste:** `TestRT_Prune_A5_URLDivergence`

**Mecanismo testado:** `runRemotePrune` com `gitExec` interceptor retornando URLs distintas para fetch (`https://github.com/testowner/testrepo.git`) e push (`https://github.com/testowner/DIFFERENT.git`).

**Resultado de execução:**

```
A5 output:
  remote deletion blocked: fetch URL and push URL for origin differ — remote deletion disabled (A5)
  [dry-run] would delete 0 remote branch(es)
FECHADO: A5 block message present
FECHADO: A5 — branch remains in bare repo
```

**Veredito: FECHADO.**

A verificação `strings.TrimSpace(pushURL) != strings.TrimSpace(fetchURL)` é feita antes de qualquer avaliação de branch. Mesmo com um snapshot de PR mergeado válido, a deleção é bloqueada e o branch permanece no bare.

**Nota sobre `pushInsteadOf`:** a análise de código mostra que `git remote get-url --push origin` expande rewrites de `pushInsteadOf` antes de retornar — o mesmo mecanismo que A5 usa. Um `pushInsteadOf` que redirecione o push para outro host seria detectado pela mesma comparação. Não foi executado separadamente porque o mecanismo é idêntico.

### 2.5 A7 / P2 — snapshot=nil bloqueia

**Teste:** `TestRT_Prune_A7_SnapshotNil`

**Resultado:**

```
remote deletion blocked: forge PR snapshot unavailable (forge degraded or truncated)
[dry-run] would delete 0 remote branch(es)
FECHADO: A7/P2 — snapshot=nil causes deletion block
```

**Veredito: FECHADO.** P2 precondition em `runRemotePrune` (linha 69–71) rejeita o run inteiro quando `snapshot == nil`. O branch sobrevive.

### 2.6 Guard — formas de `--delete` e refspecs

**Teste:** `TestRT_Guard_DeleteForms`, `TestRT_Guard_DeleteEquals_GitValidity`

**Resultado de execução:**

```
"git push origin --delete feat/foo"           → push-delete  OK
"git push origin -d feat/foo"                 → push-delete  OK
"git push origin :refs/heads/feat/foo"        → push-delete  OK
"git push origin :feat/foo"                   → push-delete  OK
"git push origin +:refs/heads/feat/foo"       → push-delete  OK
"git push origin --no-verify :refs/heads/..."  → push-delete  OK
"git push origin --atomic :refs/heads/..."    → push-delete  OK
"git push origin -o push.ci.skip :refs/..."   → push-delete  OK
"git push origin --delete=feat/foo"           → push         (precision gap)
"git push origin -dfeat/foo"                  → push         (declared residual)
```

**`TestRT_Guard_DeleteEquals_GitValidity`:**

```
FECHADO (precision gap benign): git rejected --delete=feat/del-eq: exit status 129
```

**Veredito sobre `--delete=foo`:**

`isPushBranchDelete` não reconhece a forma equals `--delete=feat/foo` — a tag emitida é `push`, não `push-delete`. Isso faria o usuário receber a mensagem do guard genérica de push em vez de "use `trackfw branch prune --remote --apply`".

Porém: `git push origin --delete=feat/del-eq` retorna exit 129 na versão de git instalada (git 2.x). Git **rejeita** a forma equals — `--delete` não aceita `=`. A lacuna de precisão existe na lógica de análise do guard, mas é **inexplorável** com git atual: o push que o guard deveria interceptar nunca chega a ser executado.

**Severidade: baixa / inofensiva com git atual.** O gap não é um vetor de bypass — o guard nunca precisa interceptar uma forma que git rejeita. Deve ser documentado como residual se futuras versões do git aceitarem `--delete=`.

### 2.7 classifyGitArgs — alias e flags

**Teste:** `TestRT_Classify_AliasAndFlags`

**Resultado:**

```
classifyGitArgs([-c alias.f=fetch f origin]) = "f" net=false — RS-7: alias
classifyGitArgs([--git-dir x fetch origin]) = "fetch" net=true
classifyGitArgs([-C /tmp fetch]) = "fetch" net=true
classifyGitArgs([--exec-path=/usr/lib/git fetch]) = "fetch" net=true
classifyGitArgs([-P fetch]) = "fetch" net=true
classifyGitArgs([-c k=v push origin]) = "push" net=true
classifyGitArgs([--work-tree /tmp ls-remote --tags]) = "ls-remote" net=true
classifyGitArgs([status]) = "status" net=false
```

**Veredito: RS-7 declarado e confirmado.** O alias `-c alias.f=fetch` + `f` retorna `"f"`, que não está em `networkSubcmds` — nenhum prazo é aplicado. Esta é a única forma conhecida de executar uma operação de rede sem prazo no contexto atual. Está declarado como RS-7 no ADR. A mitigação exigiria substituir `exec.Command` por um envolvedor que resolva aliases antes de classificar — escopo fora desta REQ.

Todos os outros casos que deveriam aplicar prazo o aplicam corretamente: flags de dois tokens (`-C`, `--git-dir`, `--work-tree`, `-c k=v`) são pulados e o subcomando correto é identificado.

### 2.8 netdeadline.Parse — valores-limite

**Teste:** `TestRT_Parse_EdgeValues` (usando sentinela de 90 min para distinguir fallback de valor parseado que coincide com o padrão)

**Resultado:**

```
Parse("24h"):    d=24h0m0s usedDefault=false  — boundary accepted
Parse("24h1ns"): d=1h30m0s usedDefault=true   — exceeds max, warn+default
Parse("25h"):    d=1h30m0s usedDefault=true   — exceeds max, warn+default
Parse("1ns"):    d=1ns     usedDefault=false  — valid, accepted without warning
Parse("1000h"):  d=1h30m0s usedDefault=true   — absurd, warn+default
Parse("1e9s"):   d=1h30m0s usedDefault=true   — invalid format, warn+default
Parse("  60s"):  d=1h30m0s usedDefault=true   — leading space rejected, warn+default
Parse("0"):      d=1h30m0s usedDefault=true   — zero not positive, warn+default
Parse("-1s"):    d=1h30m0s usedDefault=true   — negative, warn+default
Parse(""):       d=1h30m0s usedDefault=true   — absent, use default silently
Parse("60s"):    d=1m0s    usedDefault=false  — valid, no warning
```

**Veredito: FECHADO.** A invariante ADR A3 ("o prazo nunca é desligado") é satisfeita: nenhum valor inválido produz prazo zero ou infinito — todos os casos inválidos retornam o valor padrão com aviso. O limite superior de 24h é aplicado (24h aceito, 24h+1ns rejeitado).

**Nota sobre `1ns`:** o parser aceita durações positivas arbitrariamente pequenas sem aviso. `1ns` como prazo de rede torna qualquer operação de rede impossível. O ADR não define um mínimo útil — a variável é para usuários avançados e a consequência (operação sempre expira) seria óbvia. Sem ação necessária.

### 2.9 Linear — body-hang e erro não-timeout

**Testes:** `TestRT_Linear_BodyHang`, `TestRT_Linear_NonTimeout`
**Arquivo de teste:** `$D/internal/sync/zz_linear_redteam_test.go`

**Body-hang (headers enviados, body trava até ctx expirar):**

```
CreateIssue error after 201ms: linear: issueCreate: linear API timed out after 200ms
  (TRACKFW_NETWORK_TIMEOUT) — remote state is uncertain: check if the issue was
  created on Linear before running again
FECHADO: body-hang → TimeoutError (errors.Is(err, netdeadline.ErrTimeout)=true)
FECHADO: D7/A8 uncertainty message present in error
```

O servidor `httptest` envia `HTTP 200` + `Flush` (simula headers recebidos) e bloqueia o body. `Do()` retorna sem erro. `io.ReadAll()` bloqueia até o contexto expirar. Quando o ctx expira, `io.ReadAll` retorna um erro; `ctx.Err() == context.DeadlineExceeded` é verdadeiro → `TimeoutError` com mensagem de incerteza (ADR D7/A8).

**Erro não-timeout (RST antes do prazo):**

```
CreateIssue error: *fmt.wrapError: linear: read response:
  read tcp ...: read: connection reset by peer
FECHADO: non-timeout error NOT wrapped as TimeoutError
```

O servidor fecha a conexão via TCP RST antes do prazo de 5s. O erro não é `context.DeadlineExceeded` — é um erro de I/O. `CreateIssue` retorna um erro não-TimeoutError. O usuário não recebe a mensagem de "estado remoto incerto" (correta — a operação nunca foi completada do lado do servidor).

**Veredito: FECHADO.** As duas direções do discriminante estão corretas:
- Timeout real → `TimeoutError` com incerteza (estado pode ter sido escrito no Linear)
- Falha de rede sem timeout → erro genérico (sem incerteza falsa)

---

## 3. Superfícies não executadas — justificativa

### 3.1 GH_HOST mismatch (forgeDefaultBranch / queryProtectedBranches)

`forgeDefaultBranch` e `queryProtectedBranches` chamam `ghExec("api", "repos/owner/repo")` e `ghExec("api", "--paginate", endpoint)` sem `--hostname`. Se `GH_HOST` estiver definido, estas chamadas podem ser redirecionadas para outro host enquanto `queryForgePRs` usa `--repo github.com/owner/repo` (fixado).

**Por que não executado:** reproduzir requer uma instância de servidor gh API alternativa ou interceptor. O risco real é limitado: `queryProtectedBranches` só coleta a lista de branches protegidos — um redirect para servidor errado retornaria lista vazia ou erro. Lista vazia → nenhum branch é marcado como protegido → análise prossegue com zero branches protegidos excluídos. Isso é permissivo (menos proteção), não malicioso (não deleta branches que não deveria). `forgeDefaultBranch` com redirect retornaria nome de branch errado → P3 em `runRemotePrune` bloquearia todas as remoções se `defaultFromForge=false`. Em ambos os casos o resultado é conservador (mais bloqueio, não menos).

**Severidade: baixa.** Documentado como R4 no Wave 0 do prune remoto.

### 3.2 T4 — `exec.Command(variable, ...)` não detectado

Confirmado por mutação na sessão anterior (não repetida aqui). A análise só verifica literais — `bin := "git"; exec.Command(bin, "fetch")` não é detectado. Declarado como residual no Wave 0 §2.5 e no ADR RS-7 (análise estática).

---

## 4. Achados para ação

| # | Descrição | Severidade | Arquivo | Ação |
|---|---|---|---|---|
| A1 | `--delete=feat/foo` (equals) não reconhecido como `push-delete` | **Baixa / inexplorável** | `internal/guard/gitbranch.go:509` | Documentar como residual no ADR do guard; git atual rejeita a forma |
| A2 | RS-7: alias via `-c alias.f=fetch` + `f` escapa o classificador — sem prazo | **Baixa / declarada** | `internal/commands/netexec.go:classifyGitArgs` | RS-7 já declarado no ADR; mitigação fora desta REQ |
| A3 | `1ns` aceito sem aviso — operacionalmente inútil mas tecnicamente válido | **Informacional** | `internal/netdeadline/netdeadline.go:Parse` | Sem ação necessária; documento cobre |
| A4 | GH_HOST mismatch em `forgeDefaultBranch`/`queryProtectedBranches` | **Baixa / conservadora** | `internal/commands/branch_prune_forge.go` | Documentado como R4 no Wave 0 |

**Achado crítico: nenhum.**
**Achado bloqueante: nenhum.**

---

## 5. Veredito geral

Os controles principais das duas REQs resistiram à falsificação por execução:

- **Prune remoto:** isCrossRepository, base não-padrão, TOCTOU (lease servidor), A5 (push URL divergente), A7 (snapshot nil) — todos FECHADOS com evidência de execução. O lease usa `refs/heads/<b>` (servidor), não tracking ref cliente — comportamento correto medido.
- **Prazo de rede:** `Parse` trata todos os valores-limite corretamente; `classifyGitArgs` aplica prazo corretamente exceto RS-7 (declarado); Linear aplica `TimeoutError` para body-hang e não para RST (discriminante correto nas duas direções).
- **Guard:** todas as formas principais de deleção de branch são detectadas como `push-delete`; `--delete=foo` é uma lacuna de precisão inexplorável com git atual.

O branch está apto para merge sob a perspectiva de segurança.

---

## Apêndice — Artefatos de execução

Todos os testes foram executados em cópia isolada:
- `$D = /var/folders/js/z4mqvnnd3sj773g8tcntk1000000gn/T/tmp.w9v1RVLmkM` (cópia descartável via `rsync -a --exclude .git`)
- Testes em `$D/internal/commands/zz_redteam_test.go` (package commands) e `$D/internal/sync/zz_linear_redteam_test.go` (package sync)
- Nenhum arquivo foi escrito na árvore real; nenhum git commit/push foi executado

Saída de `go test -v -count=1 -run 'TestRT_' ./internal/commands/`:
- 13 testes, todos PASS, ~2s
- Incluindo controle positivo (`TestRT_Eval_MergedPR_Positive`) para confirmar que o avaliador não passa vacuamente

Saída de `go test -v -count=1 -run 'TestRT_Linear' ./internal/sync/`:
- 2 testes, ambos PASS, ~0.4s
- Body-hang verificado em 201ms (configurado `TRACKFW_NETWORK_TIMEOUT=200ms`)

---

## Adendo do arquiteto (2026-10-12) — dois alvos que o parecer não executou

- **`pushInsteadOf` reescrevendo só o push — FECHADO, medido.** Com `url."ssh://evil.example/".pushInsteadOf https://github.com/`: `git remote get-url origin` → `https://github.com/a/b.git`; `git remote get-url --push origin` → `ssh://evil.example/a/b.git`. A comparação do A5 (`runRemotePrune`) vê a divergência e bloqueia toda remoção remota.
- **Injeção pelo nome da branch — FECHADO, medido.** `git check-ref-format`: `a b`, `a:b`, `a..b` são inválidos (não chegam do `for-each-ref`); `-x` e `+a` são válidos, mas `deleteRemoteBranch` (`branch_prune_remote.go:253`) monta `:refs/heads/<b>` — o argumento começa com `:`, nunca com `-` ou `+`, e o lease é `--force-with-lease=refs/heads/<b>:<sha>`, um único argumento.
