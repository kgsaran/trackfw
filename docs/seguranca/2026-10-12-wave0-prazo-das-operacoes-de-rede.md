# Wave 0 — Threat Model: Prazo das Operações de Rede do trackfw

> Autor: `hades-tf` · Data: 2026-10-12
> REQ: `docs/req/REQ-2026-10-12-operacoes-de-rede-do-trackfw-nao-tem-prazo-e-um-fetch-pendurado-trava-push-ship-prune-e-release-sem-limite.md`
> ADR: `docs/adr/ADR-2026-10-12-operacao-de-rede-do-trackfw-tem-prazo-e-o-estouro-nunca-vira-sucesso-silencioso.md`
> Issue: #556

---

## 1. Completude da População

A enumeração partiu dos **executores** (todos os sítios `exec.Command` sem contexto), rastreou
todos os chamadores por grep e leitura de código, classificou cada ponto, e buscou `net/http` e
`internal/discover/` separadamente.

### 1.1 Executores de produção e seus sítios de rede

| Executor | Definição | Sítios de rede |
|---|---|---|
| `defaultGitExec` | `ship.go:166` — `exec.Command("git", args...)`, sem contexto | #1 #2 #3 #4 #5 #11 |
| `defaultGhExec` | `branch_prune_forge.go:56` — `exec.Command("gh", args...)`, sem contexto | #6 #9 |
| `defaultExecForgeAPI` | `release.go:229` — `exec.Command(name, args...)`, sem contexto | #7a #7b |
| `defaultCheckPROpen` | `ship.go:224` — `exec.Command(adapter.CLIName, args...)`, sem contexto | #8 |
| `defaultExecForgeCLI` | `ship.go:186` — `exec.Command(name, args...)`, stdin/stdout/stderr herdados | #10 |

### 1.2 Operações de rede — população fechada

| # | Arquivo:linha | Subcomando / endpoint | Classe | Caminho de falha atual |
|---|---|---|---|---|
| 1 | `push.go:238` | `git fetch origin --prune` | M | aviso, pula squash-merge check |
| 2 | `ship.go:392` | `git fetch origin --prune` | M | aviso, pula squash-merge check |
| 3 | `branch_prune.go:390` | `git fetch origin --prune` | M | aviso, dados possivelmente velhos |
| 4 | `release.go:278` | `git fetch origin --prune` | M | falha fechada (`releaseTagFetchFailedFmt`) |
| 5 | `release.go:310` | `git ls-remote --tags origin` | M | 🔴 **ERRO DESCARTADO** (`remoteTagOut, _ :=`): falha → "tag ausente no remoto" |
| 6 | `branch_prune_forge.go:97` | `gh pr list --repo ... --state all` | M | degradação D2 (design intencional) |
| 7a | `release.go:462` | `gh api POST repos/{owner}/{repo}/git/tags` | **W** | falha fechada nomeada |
| 7b | `release.go:482` | `gh api POST repos/{owner}/{repo}/git/refs` | **W** | falha fechada nomeada |
| 8 | `ship.go:224` | `gh pr list --head ... --state open` (forge-check) | M | cannot-verify error propagado |
| 9 | `ship.go:801` | `gh pr list` via `queryForgePRs` em `detectPendingSquashMerges` | M | degradação D2; **razão descartada** — AC5 não alcança este ponto |
| 10 | `ship.go:555` | `gh pr create` / `glab mr create` / `az repos pr create` | **I** | stdin/stdout/stderr herdados — interativo |
| 11 | `push.go:252`, `ship.go:455` | `git push [--force-with-lease]` | **T** | erro propagado |

Legenda: M = leitura de metadados; W = escrita de estado remoto; T = transferência de dados;
I = interativo.

### 1.3 Sítios NÃO de rede excluídos da população

`git remote get-url origin`, `git symbolic-ref`, `git rev-parse`, `git diff --cached`, `git
status`, `git log`, `git commit`, `git branch -r`, `git checkout -b`, `git show ref:path`, `git
ls-tree ref`, `git cat-file`, `git config user.name/email` — leem object store local ou config
local, não abrem sockets.

`internal/forge/resolve.go:192` (`git remote get-url origin`) e todos os exec de
`internal/auditsurface/auditsurface.go` (linhas 277, 295, 481: `git show`, `git ls-tree`, `git
ls-tree`) — locais.

`defaultBaseBranch` (`ship.go:624`): chama apenas `git symbolic-ref refs/remotes/origin/HEAD`
— local.

`buildPushArgs` (`ship.go:866`): chama apenas `git rev-parse --abbrev-ref --symbolic-full-name
@{u}` — local.

`internal/discover/discover.go:50-62`: usa `exec.CommandContext` com `externalCommandTimeout = 30
* time.Second` — já tem prazo. Fora da população de defeitos.

### 1.4 `net/http` — população separada

| Arquivo | Cliente | Timeout atual | Risco |
|---|---|---|---|
| `internal/thirdparty/fetch.go:30` | `fetchClient` — `http.Client{Timeout: 30s}` | 30s | OK — já tem prazo |
| `internal/sync/jira.go:187` | `newJiraHTTPClient` — `http.Client{Timeout: 30s}` | 30s | OK |
| `internal/sync/linear.go:76` | `http.DefaultClient` — **sem timeout** | **nenhum** | 🔴 Mesma causa que #556; `linear sync` trava indefinidamente se a API Linear não responder |
| `internal/serve/` | handlers HTTP servindo localmente | — | Fora do escopo (servidor, não cliente) |

`linear.go:76` usa `http.DefaultClient.Do(req)` sem contexto nem timeout. Mesma causa (exec/http
sem prazo) do issue #556, mecanismo diferente (HTTP puro, não exec.Command). Pela Regra Dura de
Causa Raiz: mesma causa → mesma REQ. Recomendado: adicionar como ML nesta REQ.

### 1.5 Erros descartados em chamadas de rede

```
release.go:310  remoteTagOut, _ := deps.execGit("ls-remote", "--tags", "origin", ...)
```

Único erro descartado em operação de **rede** (operação #5). Os demais `_ :=` são chamadas
locais documentadas no código (`remote get-url`, `config user.name/email`).

Em `ship.go:801`, `snapshot, _ := queryForgePRs(...)` descarta o segundo retorno (`string`, não
`error`). É design intencional (D2, ADR-2026-10-03): degradação esperada. Mas a razão descartada
impede que a mensagem AC5 (operação, prazo, `TRACKFW_NETWORK_TIMEOUT`) alcance o usuário no
caminho do `detectPendingSquashMerges`. Deve ser corrigido (ver A7).

**Veredito de completude:** população fechada para a base de código atual. 11 pontos (7 M + 2 W
+ 1 T + 1 I) + 1 ponto `net/http` sem prazo (`linear.go:76`).
Nota: o roadmap paralelo `branch prune --remote` (ROADMAP-2026-10-12) adiciona `git push --delete
<branch>` (operação T) ao conjunto; esse ponto herda o limite automaticamente se usar
`defaultGitExec` com classificador (A9), sem nova entrada na população deste Wave 0.

---

## 2. Mecanismo

### 2.1 Por que `exec.Command` sem contexto trava indefinidamente

O trackfw roda `git fetch` como filho direto. O git roda `ssh` como neto. Quando o ssh fica
pendurado sem timeout de rede, o git bloqueia esperando-o. O trackfw bloqueia esperando o git.
Sem `context.WithTimeout`, o encadeamento espera para sempre.

### 2.2 Por que `exec.CommandContext` sozinho não basta

Quando o contexto expira, Go envia SIGKILL ao processo filho direto (git). O git morre. Mas o
neto (ssh, rodado por `GIT_SSH_COMMAND`) herdou o file descriptor de **stderr** de git — que
aponta para a ponta-de-escrita do pipe `cmd.Stderr`. Com o git morto mas o neto vivo, o pipe
de stderr permanece aberto. A goroutine interna de `cmd.Wait()` aguarda EOF nesse pipe — que
nunca chega enquanto o neto viver.

**Evidência do mecanismo** (`test_pipe_null.go`, modo A, 2s timeout, `hang-ssh-v2.sh`):

```
cmd.Stderr = &bytes.Buffer (pipe):   BLOCKED at 6s  ← stderr goroutine bloqueia em EOF
cmd.Stderr = nil:                     BLOCKED at 6s  ← cmd.Output() instala pipe interno
cmd.Stderr = /dev/null (raw fd):      Returned in 2.003s  ← sem pipe de stderr, retorna no timeout
```

Concluindo: o stdout pipe fecha quando git morre porque o SSH helper usa seu PRÓPRIO pipe de
protocolo para fd1 — o fd1 de git (pipe do test) não é repassado para o helper. O pipe de stderr
de git (fd2) SIM é herdado pelo helper. É o stderr pipe, portanto, que segura o bloqueio.

Implicação para testes: `cmd.Stderr` deve ser um pipe (e não `/dev/null`), senão o teste passa
no modo A mesmo sem `WaitDelay` — regressão mascarada. `cmd.Stderr = &bytes.Buffer` ou
`cmd.Stderr = nil` + `cmd.Output()` são ambos corretos. `/dev/null` mascara.

**Evidência medida** (`$SCRATCH/gotest/test_git_variant.go`, modo A: `exec.CommandContext`,
sem WaitDelay, `GIT_SSH_COMMAND=hang-ssh-v2.sh`, `GIT_SSH_VARIANT=ssh` para evitar a sonda `-G`,
remoto `ssh://x.invalid/fake.git`, timeout=2s):

```
Mode A (git real): CommandContext, NO WaitDelay + GIT_SSH_VARIANT=ssh
  BLOCKED at 6s
  orphan: kgsaran  34040  sleep 600           ← neto vivo
  orphan: kgsaran  34039  /bin/bash hang-ssh-v2.sh  ← filho intermediário
```

`GIT_SSH_VARIANT=ssh` é necessário para forçar o git a passar o remote diretamente ao
`GIT_SSH_COMMAND` sem antes rodar a sonda de variante (`ssh -G`), que completaria rapidamente e
não travaria o pipe.

### 2.3 WaitDelay força o retorno (neto sobrevive)

Com `cmd.WaitDelay`, após o processo filho morrer, Go aguarda o pipe fechar por `WaitDelay`; se
não fechar, encerra os I/Os forçadamente e retorna com erro. O neto sobrevive como órfão, mas o
**trackfw retorna dentro de `timeout + WaitDelay`**.

**Evidência medida** (modo B, WaitDelay=500ms, mesmo setup):

```
Mode B (git real): CommandContext + WaitDelay=500ms + GIT_SSH_VARIANT=ssh
  Returned in 2.504s, err=signal: killed
  orphan: sleep 600        ← neto vivo
  orphan: hang-ssh-v2.sh  ← sobreviveu
```

Retornou em 2.504s = timeout(2s) + WaitDelay(0.5s). Neto sobreviveu.

### 2.4 `Setpgid: true` sem kill explícito do grupo NÃO mata o neto

`Setpgid: true` coloca o filho num process group próprio. O Go padrão (`CommandContext`) envia
SIGKILL ao PID do filho, não ao grupo. O neto (que herdou o pgid do filho) sobrevive como órfão.

**Evidência medida** (`$SCRATCH/gotest/test_mode_c.go`, modo C: WaitDelay=500ms, Setpgid=true,
sem `cmd.Cancel` explícito, mesmos `GIT_SSH_COMMAND`/`GIT_SSH_VARIANT` do modo A, timeout=2s):

```
Mode C: CommandContext + WaitDelay + Setpgid, NO explicit Cancel (pgid kill)
  Returned in 2.503s, err=signal: killed
  orphan: kgsaran  39495  sleep 600              ← neto vivo — Setpgid sozinho não encerrou o grupo
  orphan: kgsaran  39494  /bin/bash hang-ssh-v2.sh  ← filho intermediário sobreviveu
  orphan count: 2  (espera >0 — Setpgid sem pgid kill deixa netos)
```

Retornou em 2.503s = timeout(2s) + WaitDelay(0.5s). O `Setpgid` protege o trackfw do grupo do
filho (SIGKILL no grupo do filho não afeta o processo pai), mas não encerra o grupo.

### 2.5 `Setpgid: true` + kill explícito do grupo mata o neto (macOS/Linux)

Usando `cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }`:

**Evidência medida** com re-exec helper (`time.Sleep`, não stdin-read), detecção por caminho do
binário (env vars não aparecem em `ps aux`):

```
Mode A (re-exec helper, sem WaitDelay): BLOCKED at 6s  ← helper segura pipe de stderr
Mode B (WaitDelay=500ms, sem pgid kill):
  Returned in 2.504s, err=signal: killed
  orphan count: 1   ← helper (pid 42550) sobreviveu — WaitDelay retorna, mas não mata neto
Mode D (WaitDelay + Setpgid + pgid kill):
  [kill -SIGKILL -pgid=42556]
  Returned in 2.004s, err=signal: killed
  orphan count: 0   ← helper encerrado com o grupo
```

O re-exec helper dorme em `time.Sleep`, NÃO lê stdin. Garante que sobrevive à morte do git e
mantém o pipe de stderr aberto (o pipe que o bloqueia, cf. seção 2.2).

```
Mode A (cat helper, lê stdin): Returned in 2.005s  ← cat sai no EOF do pipe de protocolo
                                                       → regressão mascarada
```

Nota: `cat` recebe EOF porque o pipe de protocolo (fd0 do helper ↔ git) fecha quando git morre.
`time.Sleep` não lê stdin, então sobrevive independentemente. O pipe de stderr (herdado de git
via fd2) permanece aberto.

### 2.6 `Setpgid` em contexto de terminal: SIGTTIN e Ctrl-C (comportamento documentado, não medido)

`Setpgid: true` move git e seus filhos para um novo process group. Duas consequências quando
executado em uma sessão interativa com terminal de controle:

**(a) SIGTTIN em prompts que leem `/dev/tty`:** se ssh precisar de passphrase ou confirmar host
key, abrirá `/dev/tty` diretamente. Um processo em process group distinto do foreground recebe
`SIGTTIN` ao ler do terminal de controle e fica suspenso (estado `T`). A operação então espera
o timeout e é morta — sem dar ao usuário a chance de autenticar.

**(b) Ctrl-C não alcança o grupo do neto:** em terminal interativo, SIGINT do Ctrl-C vai para o
foreground group (trackfw). git e ssh estão em outro grupo e não o recebem. O usuário perde o
escape normal do tipo #556.

**Atenção:** `os.Stdin.Stat().Mode() & os.ModeCharDevice` pode retornar true para `/dev/null`
(que também é `ModeCharDevice`). Um discriminador mais robusto é verificar se `/dev/tty` abre
com sucesso: se abrir, há terminal de controle; se falhar (ENXIO), não há.

**Comportamento documentado, não medido aqui:** o ambiente de execução desta análise não tem
terminal de controle real, e um teste de SIGTTIN requer pty. O advisor recomendou `pty.spawn` ou
`script -q /dev/null` — não executado por falta de tty. O risco é bem documentado na especificação
POSIX (`tcsetpgrp(3)`, SIGTTIN).

**Mitigação proposta para A2:**
```go
// Abre /dev/tty: se tiver ctty, evita Setpgid para não bloquear passphrase
_, cttyErr := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
hasCtty := cttyErr == nil
if !hasCtty {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}  // build: !windows
    cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
```

Declarar como RS-6 quando há ctty. Registrar no ADR D2 como ponto a medir em AC2.

### 2.7 WaitDelay exige um contexto para ativar

Isso é crítico para o `git push`. A documentação oficial Go (`go doc os/exec.Cmd.WaitDelay`)
estabelece:

> "The WaitDelay timer starts when either the associated Context is done or a call to Wait observes
> that the child process has exited, whichever occurs first."
>
> "If WaitDelay is zero (the default), I/O pipes will be read until EOF, which might not occur
> until orphaned subprocesses of the command have also closed their descriptors for the pipes."

Se não há contexto (execução via `exec.Command` sem `CommandContext`), não há cancelamento;
`WaitDelay` nunca ativa. Um `git push` sem `CommandContext` e um neto ssh pendurado travará
indefinidamente, independente do valor de `WaitDelay`.

### 2.8 Hook `pre-push` é neto: pgid kill o encerra (T2 medido)

O hook `.git/hooks/pre-push` é executado por `git push` como filho direto; qualquer processo que
o hook spawne se torna neto do trackfw. Com Setpgid + pgid kill (modo D), todo o grupo é encerrado
quando o contexto expira — incluindo o hook.

**Evidência medida** (`test_hooks.go`: bare remote, hook `sleep 5`, timeout=2s, modo D):

```
Mode D push com hook pre-push lento (sleep 5, timeout=2s):
  [kill -SIGKILL -pgid=41538]
  Returned in 2.004s, err=signal: killed
  stderr: "pre-push hook started\n"   ← hook iniciou, mas foi encerrado pelo pgid kill
  orphan count: 0   ← hook não sobreviveu
```

Isso confirma T2: com `TRACKFW_PUSH_TIMEOUT=2s`, um hook de 5s seria morto. O usuário deve
configurar `TRACKFW_PUSH_TIMEOUT` maior que o tempo máximo esperado de hooks + push. Documentar
explicitamente no ADR D3 e na mensagem de estouro.

### 2.9 O que sobra no Windows

Não há grupos de processos POSIX no Windows. `Setpgid` não existe; `syscall.Kill(-pgid, ...)`
não funciona. O equivalente em termos de intenção seria `taskkill /T /F /PID <pid>` (encerra
árvore de processos), mas exige um helper Go com `Job Objects`. O **WaitDelay ainda garante** que
o trackfw retorna dentro de `timeout + WaitDelay` — o neto pode sobreviver como órfão, mas o CLI
termina. Fonte: https://pkg.go.dev/os/exec#Cmd.WaitDelay e #Cmd.Cancel.

---

## 3. Natureza do Limite e Valor Padrão

### 3.1 Medições

Repositório `kgsaran/trackfw` (463 PRs retornados), conexão saudável, macOS Darwin 27.0.0:

| Operação | Tempo medido | Payload |
|---|---|---|
| `git fetch origin --prune` | **1.579s** | refs only (sem objetos novos) |
| `gh pr list --state all --limit 3000 --json ...` | **2.960s** | 463 PRs |

### 3.2 Leitura e escrita de metadados (operações #1–#9, incluindo #7a/#7b)

Orçamento **total** é aceitável para leitura de metadados cujo payload é limitado por refs ou
`--limit`. O `http.lowSpeedLimit`/`lowSpeedTime` é para transferências de dados de tamanho
arbitrário, não para consultas de metadados de tamanho definido.

Operações #7a e #7b são **escritas** (POST). Se essas escritas estourarem o prazo, o estado no
GitHub fica incerto (tag pode ter sido criada ou não). A mensagem de estouro deve nomear isso
explicitamente: "operação de escrita excedeu o prazo; o estado remoto é incerto; verifique
manualmente."

**Justificativa dos 60s — por caminho de falha:**
- Operações opcionais (#1, #2, #3): estouro → aviso, continua. Impacto baixo; 60s é uma
  janela razoável antes de desistir de uma operação opcional.
- `release.go:278` (#4): estouro → falha fechada. Um `git fetch` que traz objetos novos pode
  ultrapassar 60s em repositório grande — falso positivo aceito; o usuário configura
  `TRACKFW_NETWORK_TIMEOUT` maior. Declarado no residual.
- `release.go:310` (#5): estouro → nunca "tag ausente" (A5). `ls-remote` lista apenas refs;
  60s é generoso para esse payload.
- Operações gh (#6, #8, #9): degradação. 60s é generoso para APIs com SLA < 10s.
- Medição: 2.960s (`gh pr list`, 463 PRs) e 1.579s (`git fetch`). Margem de 20× sobre o máximo
  medido → **60s como padrão**.

Fonte para variáveis de env: https://git-scm.com/docs/git-config (seção `http.lowSpeedLimit`,
`http.lowSpeedTime`, e variantes de env `GIT_HTTP_LOW_SPEED_LIMIT`, `GIT_HTTP_LOW_SPEED_TIME`).

### 3.3 Transferência de dados (`git push` — operação #11): decisão AC7

Orçamento total não é aceitável: um push de repositório grande pode levar minutos ou horas. O
mecanismo de estouro por inatividade (ocioso) é o correto.

**HTTPS — disponível sem sobrescrever config SSH do usuário (não medido aqui):**
As variáveis `GIT_HTTP_LOW_SPEED_LIMIT=1` e `GIT_HTTP_LOW_SPEED_TIME=30` aplicam limite ocioso
(1 byte/s por 30s = sem progresso) sem sobrescrever `GIT_SSH_COMMAND` nem `core.sshCommand`.
**Atenção:** se o usuário já tem `http.lowSpeedLimit` configurado em `~/.gitconfig`, sobrescrever
com `GIT_HTTP_LOW_SPEED_LIMIT` faz o mesmo que fazer para SSH — substitui a preferência do
usuário. Aplicar somente quando `git config --get http.lowSpeedLimit` retornar vazio.
Não medido com link real estrangulado; documentado a partir de man git-config.
Fonte: https://git-scm.com/docs/git-config (`http.lowSpeedLimit`/`http.lowSpeedTime`).

**SSH — injeção de `ServerAliveInterval` exige sobrescrever config do usuário:**
A ordem de resolução do comando SSH é: `GIT_SSH_COMMAND` > `core.sshCommand` > `GIT_SSH` > `ssh`.
Qualquer forma de acrescentar `-o ServerAliveInterval=N` exige substituir completamente o comando
na posição mais alta da cadeia — que pode conter chaves, ProxyJump, ControlMaster ou outros flags.
Não existe mecanismo de *composição parcial* sem substituição completa.
Verificado: `ssh -G localhost | grep serveraliveinterval` → `serveraliveinterval 0`; o valor
padrão é 0 (desabilitado), confirmando que injeção via composição não ajudaria na configuração
padrão.
Fonte: `git help config` seção `core.sshCommand`; `ssh(1)` manual.

**Consequência para WaitDelay no git push:** sem um `CommandContext`, `WaitDelay` não ativa
(seção 2.7). Para `git push` SSH travado em neto, o WaitDelay sozinho não basta.

**Recomendação AC7 (razão escrita, não "fora do escopo"):** o ML-1B aplica
`GIT_HTTP_LOW_SPEED_LIMIT=1 GIT_HTTP_LOW_SPEED_TIME=30` ao executor de `git push` quando
`remote.origin.url` é HTTPS. Para SSH: declarar RS-1. Além disso, aplicar `CommandContext` com
timeout `TRACKFW_PUSH_TIMEOUT` (default: `max(TRACKFW_NETWORK_TIMEOUT × 10, 30min)`) para que
`WaitDelay` ative em absoluto e o CLI nunca fique preso por mais que esse teto — incluindo o
caso SSH. Um push legítimo de 30min é extremo; um usuário que precise de mais tempo configura
`TRACKFW_PUSH_TIMEOUT`.

### 3.4 Interativo (operação #10 — `gh pr create`)

Sem prazo. O usuário está presente. Confirmado: `defaultExecForgeCLI` hereda
stdin/stdout/stderr. Matar o prompt seria defeito. O AC6 da REQ exige teste afirmando isso.

---

## 4. Threat Model

### 4.1 Adversário

O **implementador apressado e o arquiteto otimista** — não um atacante externo. Os vetores
abaixo descrevem como a proteção esvaziaria sem quebrar nenhuma regra escrita.

### 4.2 Vetores

**T1 — `TRACKFW_NETWORK_TIMEOUT=0` desliga o prazo silenciosamente**
Se a implementação tratar `0` como "sem prazo", qualquer usuário pode escrever
`TRACKFW_NETWORK_TIMEOUT=0 trackfw push` e regredir ao comportamento do issue #556. Valor
absurdo (`168h`) produz prazo nominal sem proteção real.
**Mitigação:** parser rejeita `0` (emite aviso, usa padrão) e impõe teto máximo.

**T2 — Hook `pre-push` lento estoura o timeout do `git push`**
O hook `.git/hooks/pre-push` roda como neto do trackfw (trackfw → git push → hook). Se um teto
total for aplicado ao `git push` (`TRACKFW_PUSH_TIMEOUT`), um hook que execute testes estourará.
**Mitigação:** `TRACKFW_PUSH_TIMEOUT` é separado de `TRACKFW_NETWORK_TIMEOUT` e documentado
como incluindo o tempo dos hooks. O usuário configura conscientemente.

**T3 — Erro descartado em `release.go:310`**
`remoteTagOut, _ := deps.execGit("ls-remote", ...)` — timeout ou falha de rede → variável vazia
→ código conclui silenciosamente "tag não existe no remoto" → tenta criar tag que já existe,
ou cria tag duplicada em repos sem proteção.
**Mitigação:** AC4 — falha ou timeout do `ls-remote` → falha fechada nomeada.

**T4 — Executor de rede novo criado diretamente (sem o classificador)**
Se um ML futuro criar `exec.Command("git", ...)` diretamente para uma operação de rede, nasce
sem prazo. O padrão já existe (validator, barrier, auditsurface — todos locais, mas a tentação
de copiar é real).
**Mitigação:** o gate real é um teste que varre `internal/` procurando literais de subcomandos
de rede (`"fetch"`, `"ls-remote"`, `"push"`) em chamadas diretas a `exec.Command` (sem passar
por `defaultGitExec`). Falha se qualquer match for encontrado. Esse teste também converte RS-7
de sistema de honra em gate. O AC2 (executor de produção + hang helper) também pega a regressão
para o sítio específico que ele testa, mas não varre sítios futuros.

**T5 — Interativo recebe prazo por acidente**
Um refactor que unificar `defaultExecForgeCLI` com os executores de `gh` (`defaultGhExec`,
`defaultCheckPROpen`) pode acidentalmente adicionar contexto ao `gh pr create`. O prompt
desaparece após 60s.
**Mitigação:** `defaultExecForgeCLI` é identificado por herdado stdin/stdout/stderr (interativo);
os outros executores de `gh` são não-interativos e recebem contexto normalmente. AC6 testa que
`defaultExecForgeCLI` não retorna antes de ser cancelado pelo caller.

**T6 — Escrita com estado incerto não é distinguida de falha limpa**
`release.go:462` e `:482` são `POST` — se estourarem, não sabemos se a tag foi criada no
GitHub. A mensagem genérica de timeout pode levar o usuário a re-executar, criando uma segunda
tag se a primeira existiu.
**Mitigação:** a mensagem de estouro nos sítios de escrita deve nomear explicitamente a
incerteza do estado remoto e pedir verificação manual antes de re-executar.

---

## 5. Alvos de Falsificação nas Duas Direções

### 5.1 Direção R — prazo regride (trava)

| Ponto | O que o teste faz | Reprova como |
|---|---|---|
| Executores #1–#4, #5–#9 | `defaultGitExec` de produção + remoto `ssh://invalid` + `GIT_SSH_COMMAND=hang-script` + `GIT_SSH_VARIANT=ssh`; assert retorno dentro de `timeout + WaitDelay + 1s` | `time.After(...)` expira antes de `done` |
| `WaitDelay` ausente | Mesmo setup, sem WaitDelay; assert bloqueio além de `timeout + 1s` | passa quando deveria bloquear |
| `release.go:310` | stub `execGit` retorna `("", error)` para "ls-remote"; assert `runReleaseTag` retorna **erro nomeado** | retorna `nil` — falso "tag ausente" |

**Desenho do teste principal (AC2):**

```go
// TestNetworkTimeoutGrandchildHang — executor de produção contra neto pendurado
func TestNetworkTimeoutGrandchildHang(t *testing.T) {
    // helper via re-exec: time.Sleep, NÃO stdin-read (cat exits on EOF, masking regression)
    t.Setenv("GIT_SSH_COMMAND", os.Args[0])
    t.Setenv("TRACKFW_TEST_HANG_HELPER", "1")
    t.Setenv("GIT_SSH_VARIANT", "ssh")  // obrigatório: evita sonda -G
    repoDir := initFakeRepoWithSSHRemote(t)

    start := time.Now()
    // -C repodir exercita o skip logic do classificador (twoTokenGlobalFlags["-C"])
    _, err := defaultGitExec("-C", repoDir, "fetch", "origin")
    elapsed := time.Since(start)

    assert.GreaterOrEqual(t, elapsed, networkTimeout)  // lower bound: esperou o prazo
    maxExpected := networkTimeout + waitDelay + 1*time.Second
    assert.Less(t, elapsed, maxExpected)  // upper bound: não bloqueou além de prazo + WaitDelay
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "TRACKFW_NETWORK_TIMEOUT")  // AC5: mensagem nomeia variável
    // Sem WaitDelay: reprovar removendo cmd.WaitDelay e assert elapsed > timeout + 5s
}
```

**Para CI de Windows sem `sleep` POSIX garantido:** o helper deve bloquear em `time.Sleep`,
NÃO em `os.Stdin.Read`. A razão: `os.Stdin.Read` recebe EOF quando git morre (a pipe de protocolo
entre git e o helper fecha na morte do git), e o helper sai — sem órfão segurando o pipe, o teste
passa no modo A mesmo sem `WaitDelay`, mascarando a regressão. Medido:

```
Test with 'cat' helper (reads stdin, exits on stdin EOF):
  Returned in 2.005s, err=signal: killed   ← retorna sem WaitDelay — regressão mascarada
```

O padrão correto (usado pela própria stdlib Go nos testes de `os/exec`) é re-executar o binário
de teste com um env var de guarda:

```go
// TestMain: se helper mode, apenas dorme
if os.Getenv("TRACKFW_TEST_HANG_HELPER") == "1" {
    time.Sleep(600 * time.Second)
    os.Exit(0)
}
// No teste:
t.Setenv("GIT_SSH_COMMAND", os.Args[0])          // re-executa o próprio binário
t.Setenv("TRACKFW_TEST_HANG_HELPER", "1")
t.Setenv("GIT_SSH_VARIANT", "ssh")
```

Assim o helper dorme independentemente; a pipe de stderr persiste após a morte do git; modo A
bloqueia corretamente. Medido em macOS (Darwin 27.0.0). Linux: comportamento esperado (POSIX, não medido neste Wave 0).
Windows: braço negativo do AC2 (modo A com re-exec helper deve bloquear) **não medido aqui** —
declarado em RS-2 e deve ser confirmado em CI `windows-full-suites`.

Notar: `GIT_SSH` (exec'd sem shell) seria mais robusto que `GIT_SSH_COMMAND` para o helper
Windows (evita problemas com backslashes no path em `sh -c`). Como o setup de test usa
`GIT_SSH_VARIANT=ssh`, `GIT_SSH_COMMAND` é suficiente em Unix; `GIT_SSH` deve ser avaliado
no Windows como alternativa.

### 5.2 Direção FP — prazo mata legítimo ou estouro vira sucesso

| Ponto | O que o teste faz | Reprova como |
|---|---|---|
| Metadata lenta mas dentro do prazo | `TRACKFW_NETWORK_TIMEOUT=3s`, re-exec helper dorme 1s e sai com código 0; assert executor **retorna sem erro** (não timeout) | retorna `DeadlineExceeded` — prazo muito curto ou WaitDelay mal configurado |
| Interativo AC6 (`gh pr create`) | executor de produção `defaultExecForgeCLI` com helper que dura 2×timeout; assert comando **não retorna** antes de ser cancelado pelo caller | retorna `context.DeadlineExceeded` — interativo recebeu contexto |
| `ls-remote` timeout → sucesso falso | stub retorna `("", error)`; assert `runReleaseTag` retorna **erro** | retorna `nil` |
| `TRACKFW_NETWORK_TIMEOUT=0` | parser com valor 0; assert usa **padrão** e emite aviso | usa 0 → deadline expired imediatamente |
| `TRACKFW_NETWORK_TIMEOUT=invalid` | parser com "banana"; assert usa **padrão** e emite aviso | panic ou silêncio |
| `TRACKFW_NETWORK_TIMEOUT=-1s` | `time.ParseDuration` aceita durações negativas sem erro; assert parser rejeita e usa padrão | timeout negativo → contexto expirado imediatamente antes de qualquer exec |
| Escrita de tag com timeout | stub `execForgeAPI` retorna deadline exceeded; assert mensagem de erro **nomeia incerteza** | mensagem genérica sem aviso de estado incerto |
| Degradation reason em `detectPendingSquashMerges` | `ghExec` timeout; assert que mensagem **inclui `TRACKFW_NETWORK_TIMEOUT`** | mensagem silenciosa (nenhuma, ou genérica sem variável) |
| Setpgid com terminal de controle | chamar executor com `/dev/tty` abrindo (ctty presente); assert `cmd.SysProcAttr.Setpgid` é `false` (somente WaitDelay ativado) | Setpgid aplicado → SIGTTIN suspende processo que lê `/dev/tty` (não medido neste Wave 0; confirmar em CI pty) |
| Classificador: `git commit` não recebe contexto | classificador com `["commit", "-m", "msg"]`; assert retorno é `exec.Command` (sem `CommandContext`) | pre-commit hook morto em 60s |
| Classificador: `git fetch origin` recebe contexto | classificador com `["fetch", "origin"]`; assert retorno tem contexto | fetch sem prazo — T4 ativado |
| `release` com fetch lento legítimo | `git fetch` em repo grande pode ultrapassar 60s; declarado como FP aceito (residual); saída: `TRACKFW_NETWORK_TIMEOUT` maior. O release falha closed conforme D1 — comportamento esperado, não bug. |

---

## 6. Resíduo Declarado

| # | Resíduo | Razão |
|---|---|---|
| RS-1 | **SSH idle detection no `git push`:** sem `ServerAliveInterval` injetado, a não ser que o usuário configure `~/.ssh/config` | `GIT_SSH_COMMAND` e `core.sshCommand` substituem completamente o comando ssh; não há opção de composição. Sobrescrever a config do usuário (chaves, ProxyJump, ControlMaster) é trade-off inaceitável sem opt-in explícito. O backstop é `TRACKFW_PUSH_TIMEOUT`. |
| RS-2 | **Windows: neto ssh pode sobreviver ao estouro** | Não há kill de grupo POSIX. `WaitDelay` garante retorno do trackfw; o neto pode ficar órfão. `taskkill /T` com Job Objects é possível mas fora de escopo. |
| RS-3 | **Hook `pre-push` de terceiro que roda rede** | Neto do trackfw; incluído no `TRACKFW_PUSH_TIMEOUT`. Se o hook for legítimo e lento, o usuário configura `TRACKFW_PUSH_TIMEOUT` maior. Aceitável. |
| RS-4 | **Sítios fora de `internal/commands/`** com `exec.Command` local (forge/resolve, auditsurface, barrier, generators, validator) | Todos verificados como locais (object store, git config). Sem risco de travamento por rede. |
| RS-5 | **`TRACKFW_PUSH_TIMEOUT` contradiz ADR D4 ("variável única")** | Os contratos são distintos: `TRACKFW_NETWORK_TIMEOUT` é teto de metadados (60s), `TRACKFW_PUSH_TIMEOUT` é teto de transferência (default 30min). Unificá-los forçaria o usuário a escolher entre travar no push e matar fetch legítimo. A segunda variável é aceita; o ADR D4 deve ser ajustado pelo Wave 1 para documentar a exceção. |
| RS-6 | **Setpgid quando há terminal de controle:** processo em novo process group pode receber SIGTTIN ao abrir `/dev/tty` (passphrase ssh, confirmação de host key), ficando suspenso | Mitigação (não medida, comportamento POSIX documentado): verificar se `/dev/tty` abre antes de aplicar Setpgid. Quando ctty presente, usar somente WaitDelay (neto pode sobreviver, mas usuário consegue Ctrl-C). |
| RS-7 | **Subcomandos git de rede não listados** (`remote show`, `remote prune`, `pull`, `clone`, `submodule update`) | Não usados na base atual; classificados como locais (sem contexto). ML que adicionar um deles deve registrar no mapa `networkSubcmds` e na população. |

---

## Veredito

O ADR está **correto nos mecanismos essenciais** (D1, D2, D5, D6) e **aprovado condicionado** aos
ajustes abaixo. Os pontos que o ADR deixou em aberto para o Wave 0 estão respondidos por
medição.

**Ajustes obrigatórios antes do Wave 1:**

**A1 — `git push` precisa de `TRACKFW_PUSH_TIMEOUT` para que o `WaitDelay` ative.** Sem um
`CommandContext` no executor de push, `WaitDelay` não ativa (seção 2.7): o trackfw travaria
indefinidamente num push SSH pendurado, o que contradiz o objetivo da REQ. Criar
`TRACKFW_PUSH_TIMEOUT` (default: `max(TRACKFW_NETWORK_TIMEOUT × 10, 30min)`); para HTTPS,
complementar com `GIT_HTTP_LOW_SPEED_LIMIT=1 GIT_HTTP_LOW_SPEED_TIME=30` somente quando `git
config --get http.lowSpeedLimit` retornar vazio (evitar sobrescrever config do usuário — mesma
razão pela qual SSH injection foi rejeitada). Para SSH push: RS-1. Registrar no ADR D3.
⚠️ **Atenção:** `TRACKFW_PUSH_TIMEOUT` introduz uma segunda variável de ambiente, o que contradiz
o ADR D4 ("variável de ambiente única"). O Wave 0 deliberadamente recomenda a segunda variável
porque os contratos são distintos: `TRACKFW_NETWORK_TIMEOUT` é um teto de metadados (60s),
`TRACKFW_PUSH_TIMEOUT` é um teto de transferência (30min). Conflapsá-los num único valor force
um dos dois a estar errado. Requer emenda ao ADR D4 documentando a segunda variável e os
motivos da distinção.

**A2 — `Setpgid: true` sem `cmd.Cancel` explícito NÃO mata o neto (medido); aplicar só quando
não há terminal de controle.** O ADR D2 diz "o estouro encerra o grupo" — correto na intenção,
incompleto na implementação.

Modo C medido (Setpgid sem Cancel, WaitDelay=500ms, timeout=2s): retorna em 2.503s mas 2 órfãos.
Modo D medido (Setpgid + Cancel + pgid kill): retorna em 2.004s, 0 órfãos (re-exec time.Sleep helper).

O ML-1A usa `/dev/tty` como discriminador (mais preciso que `os.Stdin.Stat()`, que retorna
`ModeCharDevice` para `/dev/null`; SIGTTIN depende do ctty, não de stdin):
```go
tty, cttyErr := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
if cttyErr == nil { tty.Close() }
hasCtty := cttyErr == nil
if !hasCtty {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}  // build: !windows
    cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
```
**Atenção:** SIGTTIN com Setpgid em terminal de controle é comportamento POSIX documentado mas
não medido aqui (requer pty — não disponível neste ambiente). O MVP é: declarar RS-6 e confirmar
em AC2 CI com `script -q /dev/null` no pipeline de macOS/Linux.
Quando `hasCtty=true`: somente WaitDelay — neto pode sobreviver (RS-6), mas o usuário mantém
Ctrl-C e passphrase ssh funcionam.
No Windows: somente WaitDelay em qualquer caso (RS-2).
`GIT_SSH_VARIANT=ssh` é necessário nos testes. Registrar no ADR D2.

**A3 — `TRACKFW_NETWORK_TIMEOUT=0` é inválido; teto máximo obrigatório.** O ADR D4 diz "0 só
desliga o prazo se o Wave 0 justificar". Decisão: `0` é inválido — o defeito documentado (#556,
70min) aconteceu exatamente por ausência de prazo. Parser deve rejeitar `0` com aviso e usar
padrão. Adicionar teto máximo (ex.: 24h) para valores absurdos. Registrar no ADR D4.

**A4 — Padrão de 60s para operações #1–#9.** Baseado em medição: `gh pr list` com 463 PRs em
2.960s; `git fetch` de refs em 1.579s (refs apenas, sem objetos); margem de 20× sobre o máximo
medido. `release.go:278` (#4) com fetch que traz objetos pode ultrapassar 60s — FP aceito
declarado em 3.2. Registrar no ADR D3.

**A5 — `release.go:310` (operação #5): falha fechada, nunca "tag ausente".** Timeout, erro de
rede ou falha de auth no `ls-remote` → falha fechada nomeada, ex.:
`"could not confirm tag %s is absent from remote: %s"`. Nunca interpretar erro como "tag ausente".
AC4 da REQ. Registrar no ADR D1.

**A6 — Testes com neto pendurado: `GIT_SSH_VARIANT=ssh` é obrigatório; helper deve usar
`time.Sleep`, não `os.Stdin.Read`.** Dois requisitos:
(a) `GIT_SSH_VARIANT=ssh` — sem esta variável o git faz sonda `-G` que completa rapidamente
sem bloquear o pipe; o modo A passaria mesmo sem `WaitDelay`, mascarando a regressão.
(b) O helper de neto não deve ler stdin. Um helper que lê stdin (ex.: `cat`) sai quando git morre
(EOF do pipe de protocolo) — sem órfão, sem bloqueio, modo A passa → regressão mascarada. Medido:
`cat` helper retornou em 2.005s no modo A (ver seção 5.1). Helper correto: `time.Sleep(600s)` via
re-exec de `os.Args[0]` com env var de guarda (padrão Go stdlib).
Registrar nos comentários dos testes.

**A7 — `ship.go:801`: degradação de `gh pr list` deve emitir a mensagem AC5.** `queryForgePRs`
descarta o segundo retorno (`string` de razão). No estouro do prazo, o usuário não vê a mensagem
AC5 (operação, prazo, `TRACKFW_NETWORK_TIMEOUT`). O ML-1B deve propagar a razão para aviso no
output quando o timeout for a causa.

**A8 — Escritas (`release.go:462`, `release.go:482`): mensagem de estouro deve nomear incerteza
de estado.** Timeout num POST ao GitHub (`git/tags`, `git/refs`) deixa estado remoto incerto.
Mensagem deve dizer: "operação de escrita excedeu o prazo; verifique manualmente se a tag foi
criada no GitHub antes de re-executar." Não o mesmo texto de uma leitura que estourou.

**A9 — O executor classifica por subcomando; não há dois executores.** O ADR D2 rejeitou "prazo
em cada ponto de chamada" porque "ponto novo nasce sem prazo". Um design com dois executores
(`defaultGitExec` e `defaultGitExec`) tem o mesmo defeito: a escolha do executor vira um
controle em cada ponto de chamada — opt-in, não por construção.

A implementação correta: **um único executor** (`defaultGitExec`) com classificador interno que
atribui o contexto por subcomando. O classificador salta os valores dos flags globais de dois
tokens antes de encontrar o subcomando:

```go
var networkSubcmds = map[string]bool{
    "fetch": true, "ls-remote": true, "push": true,
}
// Flags globais git que consomem o próximo token como valor (não é subcomando):
var twoTokenGlobalFlags = map[string]bool{
    "-C": true, "-c": true, "--git-dir": true, "--work-tree": true,
    "--namespace": true, "--super-prefix": true,
}
func classifyGitArgs(args []string) string {
    skip := false
    for _, a := range args {
        if skip { skip = false; continue }
        if twoTokenGlobalFlags[a] { skip = true; continue }
        if strings.HasPrefix(a, "-") { continue }
        return a  // primeiro token não-flag após os flags globais = subcomando
    }
    return ""
}
```

Subcomandos não listados (`remote show`, `remote prune`, `pull`, `clone`, `submodule update`)
são classificados como locais pela ausência no mapa — declarado RS-7. Se um ML futuro
adicionar um desses ao produto, deve registrar o subcomando e atualizar a população.

**Falsificação FN (classificador não deve errar rede como local):**
- `["-C", "d", "fetch", "origin"]` → `classifyGitArgs` → `"fetch"` → `TRACKFW_NETWORK_TIMEOUT`
- `["-c", "k=v", "push", "origin"]` → `"push"` → `TRACKFW_PUSH_TIMEOUT`
- `["-C", "d", "ls-remote", "--tags", "origin"]` → `"ls-remote"` → `TRACKFW_NETWORK_TIMEOUT`

**Falsificação FP (classificador não deve errar local como rede):**
- `["commit", "-m", "msg"]` → `"commit"` → sem contexto (pré-commit hook seguro)
- `["log", "--oneline"]` → `"log"` → sem contexto
- `["rev-parse", "HEAD"]` → `"rev-parse"` → sem contexto

Testes unitários do classificador cobrem ambas as direções. Registrar no ADR D2.

**Executores de `gh`:** `defaultGhExec`, `defaultCheckPROpen` e `defaultExecForgeAPI` recebem
sempre `CommandContext` + `WaitDelay` (sem classificador — `gh` tem apenas operações de rede).
`defaultExecForgeCLI` nunca recebe contexto (interativo, stdin/stdout herdados). O risco T5 é
um refactor que unificar os executores de `gh` com `defaultExecForgeCLI`.

**A10 — `linear.go:76`: `http.DefaultClient` sem timeout (mesma causa, Regra Dura de Causa Raiz).**
`internal/sync/linear.go:76` chama `http.DefaultClient.Do(req)` sem timeout. Mesma causa (cliente
de rede sem prazo) que o issue #556, mecanismo diferente (HTTP puro vs exec.Command). Pela Regra
Dura de Causa Raiz: mesma causa → mesma REQ → mesmo ML. Não pode permanecer no resíduo como
"aceito". O ML correspondente usa `http.Client{Timeout: 30*time.Second}` ou
`http.NewRequestWithContext(ctx, ...)` com o mesmo `TRACKFW_NETWORK_TIMEOUT`. O `jira.go` e
`thirdparty/fetch.go` já têm timeout; a inconsistência é pontual.

---

*Parecer concluído em 2026-10-12. Nenhum arquivo de produto alterado. Gates de wave:*

```bash
test -s docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md
grep -q "Veredito" docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md
git diff --quiet HEAD -- internal/
```
