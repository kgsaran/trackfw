# Wave 0 — Threat Model: Isolamento de Ambiente dos Testes

> Roadmap: `ROADMAP-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md`
> REQ: `REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md`
> Data: 2026-10-29 | ML-0A | Hades-tf

---

## 1 — Completude da enumeração

### 1.1 Inventário — pacotes cujos testes leem HOME, USERPROFILE ou CLAUDE_CONFIG_DIR

Comando para fechar a lista (sem `| head`):
```bash
for p in $(go list ./...); do
  go list -test -deps "$p" 2>/dev/null \
    | grep -qx 'github.com/kgsaran/trackfw/internal/homedir' && echo "$p"
done
```

12 pacotes na closure de `internal/homedir`:
```
github.com/kgsaran/trackfw/cmd/trackfw
github.com/kgsaran/trackfw/internal/agentconfig
github.com/kgsaran/trackfw/internal/commands
github.com/kgsaran/trackfw/internal/config
github.com/kgsaran/trackfw/internal/discover
github.com/kgsaran/trackfw/internal/generators
github.com/kgsaran/trackfw/internal/guard
github.com/kgsaran/trackfw/internal/homedir
github.com/kgsaran/trackfw/internal/integrations
github.com/kgsaran/trackfw/internal/serve
github.com/kgsaran/trackfw/internal/sync
github.com/kgsaran/trackfw/internal/validator
```

Dois desses não têm arquivos de teste — confirmado via `go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...`:

- `cmd/trackfw` — sem arquivos `_test.go`
- `internal/homedir` — sem arquivos `_test.go`

Restam 10 da closure com testes + 13 pacotes adicionais com testes (não na closure), totalizando **23 pacotes com arquivos de teste** no módulo.

### 1.2 Estado de isolamento por pacote (closure + pacotes afetados pelo AC3)

| Pacote | TestMain | Isola HOME/USERPROFILE | Isola CLAUDE_CONFIG_DIR | Resultado medido (HOME fake + PATH sem trackfw) |
|---|---|---|---|---|
| `internal/validator` | sim | sim (MkdirTemp + Setenv HOME+USERPROFILE) | sim (Unsetenv) | `ok 40s` |
| `internal/generators` | sim | sim (MkdirTemp + Setenv HOME+USERPROFILE) | sim (Unsetenv) | `ok 55s` |
| `internal/commands` | sim (parcial) | **não** | sim (Unsetenv linha 89) | **FAIL 15 testes** |
| `internal/guard` | não | não | não | `ok 3s` |
| `internal/serve` | não | não | não | `ok 0.4s` |
| `internal/sync` | não | não | não | `ok 5s` |
| `internal/agentconfig` | não | por `t.Setenv` por teste | por `t.Setenv` por teste | `ok 0.4s` |
| `internal/integrations` | não | por `t.Setenv` por teste | por `t.Setenv` por teste | `ok 8s` |
| `internal/config` | não | por `t.Setenv` por teste | não necessário | `ok 0.2s` |
| `internal/discover` | não | não | não | `ok 1s` |
| `internal/auditsurface` | sim (GIT_CONFIG_PARAMETERS) | não | não | `ok 0.5s` |
| `internal/pathguard` | sim (GIT_CONFIG_PARAMETERS) | não | não | `ok 1.1s` |

**Por que guard, serve, sync passam sem isolamento:** nenhum desses pacotes lê HOME em código de produção ou em código de teste (grep de `homedir\.` e `os.Getenv("HOME")` nos respectivos `.go` excluindo `_test.go` → sem resultado). A dependência transitiva de `homedir` existe (verificada via `go list -test -deps`), mas nenhum dos exercícios de teste percorre o caminho que chama `homedir.Dir()`.

**Por que pacotes sem TestMain sobrevivem com fake HOME incluindo `.trackfw/scripts/`:**
`validateGuardGlobalScriptIntegrity` — a função que lê `~/.trackfw/scripts/*.sh` e falha quando há deriva — reside em `internal/validator`, que tem HOME isolation. Nenhum dos pacotes sem TestMain na closure (guard, serve, sync, agentconfig, integrations, config, discover) invoca essa função em seus testes. O `~/.trackfw/scripts/` real contém `trackfw-git-branch-guard.sh` e `trackfw-credential-guard.sh` (verificado via `ls ~/.trackfw/scripts/`). Re-seedar o fake HOME com esses arquivos não alteraria o resultado desses pacotes.

### 1.3 A lista está fechada?

| AC | Superfície | Cobertura |
|---|---|---|
| AC5 (#624) | `internal/commands` lê `~/.claude/settings.json` real | medida — único pacote que FAIL sem isolamento de HOME |
| AC3 | `git` ausente → 109+ FAIL de produto | 4 pacotes com TestMain precisam de guarda |
| AC4 | `make quality` não qualifica pré-requisito | Makefile — disjunto |

Não há superfície ausente. `homedir` e `cmd/trackfw` não têm testes. `version` não aparece no `go list` de pacotes com testes.

---

## 2 — Modelo de ameaça

O adversário é o **implementador apressado** e o **arquiteto otimista** — não um atacante externo.

### 2.1 Implementador apressado — como esvaziar o ML sem quebrar nenhuma regra escrita

**AC5 (ML-1A) — arm do CLAUDE_CONFIG_DIR:**
O `TestMain` de `commands` já faz `Unsetenv("CLAUDE_CONFIG_DIR")` (linha 89). Um implementador que leu apenas essa linha poderia concluir que o isolamento está completo e pular o MkdirTemp/Setenv HOME+USERPROFILE. Os 15 FAIL desaparecem quando CLAUDE_CONFIG_DIR é o único vetor testado — mas com HOME real + PATH sem trackfw os mesmos 15 FAIL retornam. O sentinela `TestIsolatedHomeCoversBothChannels` não existiria ainda para capturar.

Medição do arm: `env HOME="$FAKE_HOME_SEM_CLAUDE" CLAUDE_CONFIG_DIR="$POISONED_DIR" GOCACHE=... go test ./internal/commands/ -count=1` → `ok 59.475s` (0 FAIL, rc=0). Esse é o falso verde.

**AC3 (ML-2A) — `LookPath` em vez de `git --version`:**
O roadmap (ML-2A ação 1) escreve `exec.LookPath("git")`. LookPath sucede para um executável `git` que exit 127 (medido: `path` preenchido, `err=nil`, mas `exec.Command("git","--version").Run()` retorna exit 127). Com LookPath puro, suíte prossegue → 109 FAIL de produto em `validator` isolado (medido). O sentinela passaria com CI (git funcional) e falharia no teste do falso sentinela, quando o fake-git enganar o LookPath.

**AC3 (ML-2A) — `os.Exit(0)` satisfaz AC escrito como "zero `--- FAIL`":**
O AC de ML-2A exige "zero `--- FAIL`". Com `os.Exit(0)` e `-test.run=^$`, a saída do `go test` reporta `ok` — sem um único `--- FAIL`, sem um único test rodado. O defeito sobrevive com todas as métricas verdes. O AC deve exigir `rc≠0` explicitamente.

**ML-1A — omissão da preservação de GOCACHE/GOMODCACHE:**
5 testes de `commands` chamam `exec.Command("go","build","./cmd/trackfw")` sem setar `cmd.Env`. Após HOME isolation, esses subprocessos herdariam `HOME=tempDir`. GOCACHE e GOMODCACHE não estão no ambiente de shell (confirmado: `printenv GOCACHE GOMODCACHE` → sem resultado). No macOS, GOCACHE é derivado de `os.UserCacheDir()` → `$HOME/Library/Caches/go-build`. Com `HOME=tempDir`, `go build` computa cache fria: falha com `GOPROXY=off` (rc=1, medido), leva 8.5s com rede (27 módulos, medido). Além disso, `os.RemoveAll(tempDir)` falha em arquivos somente-leitura do cache de módulos (Permission denied medido, ~4009 arquivos stranded).

**ML-1A — omissão do canal de produto no sentinela:**
O molde do `validator` verifica apenas HOME e USERPROFILE como variáveis de ambiente. O canal `agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)` — assinatura `func ClaudeConfigDir(r CLIHomeResolver) (string, error)` (verificada em `internal/agentconfig/agentconfig.go:307`) — é exercitado em código de produto (`validator_git_branch_guard.go:210`, 434, 494, 561, 637; `validator_credential_guard.go:1490`). Uma migração em `commands` que trocar `homedir.Dir()` por `agentconfig.ClaudeConfigDir()` não seria capturada por sentinela que verifica apenas `os.Getenv("HOME")`.

### 2.2 Arquiteto otimista — como fechar o roadmap sem fechar o defeito

- Move ML-0A para ✅ sem marcar as ACs `[x]`, porque `trackfw barrier --wave 0` verifica ML `✅` e ACs `[x]`, não a qualidade da análise.
- Aceita `make quality` verde como evidência que o isolamento está completo — CI tem `git` e HOME real do runner contém `.claude/settings.json`; o mesmo defeito que AC5 descreve passa despercebido.
- Fecha ML-2A com `t.Skip` por teste em vez de `os.Exit(2)` no TestMain: suíte reporta `ok` sem ter rodado nada.

---

## 3 — Alvos de falsificação em ambas as direções

### 3.1 Medições base

**Grupo A** (HOME fake seeded com hooks+USERPROFILE, PATH sem trackfw, GOCACHE/GOMODCACHE explícitos):
Pacotes: agentconfig, auditsurface, changelog, commands, config, discover, forge, generators, guard, guardforms, i18n, identity, integrations, metrics, netdeadline, pathanchor.
```
ok  internal/agentconfig   0.368s   ok  internal/forge         0.451s
ok  internal/auditsurface  0.479s   ok  internal/generators    55.227s
ok  internal/changelog     0.364s   ok  internal/guard         3.355s
FAIL internal/commands     61.656s  ok  internal/guardforms    0.175s
ok  internal/config        0.227s   ok  internal/i18n          0.173s
ok  internal/discover      1.157s   ok  internal/identity      0.228s
                                    ok  internal/integrations  8.475s
                                    ok  internal/metrics       0.202s
                                    ok  internal/netdeadline   0.186s
                                    ok  internal/pathanchor    0.173s
```

**Grupo B** (HOME fake, marcador renovado):
Pacotes: guard, pathguard, roadmapdoc, serve, sync, thirdparty, translog, validator.
```
ok  internal/guard         3.079s   ok  internal/thirdparty    0.469s
ok  internal/pathguard     1.120s   ok  internal/translog      0.643s
ok  internal/roadmapdoc    0.548s   ok  internal/validator     40.285s
ok  internal/serve         0.384s
ok  internal/sync          5.370s
```

Único FAIL: `internal/commands` (15 testes de barrier/validate). Os outros 22 pacotes medidos passam.

**Escrita no HOME fake:**
Run 1 — apenas telemetria Go runtime:
```
Library/Application Support/go/telemetry/local/compile@go1.25.2-darwin-arm64-2026-10-29.v1.count
Library/Application Support/go/telemetry/local/vet@…v1.count
Library/Application Support/go/telemetry/local/upload.token
Library/Application Support/go/telemetry/local/go@…v1.count
Library/Application Support/go/telemetry/local/asm@…v1.count
Library/Application Support/go/telemetry/local/link@…v1.count
Library/Application Support/go/telemetry/local/weekends
```
Run 2 — **nenhum arquivo novo** no HOME fake. O produto trackfw não criou nenhum arquivo no HOME fake.

**Escrita no HOME real:**
`~/.claude/settings.json` não modificado (last-modified: 2026-10-24, anterior à medição). `~/.trackfw/` e `~/.gemini/`: sem novos arquivos. `~/.codex/`: arquivos SQLite do Codex CLI (`models_cache.json`, `logs_2.sqlite-wal`) — nenhuma referência a esses nomes em `internal/` (grep confirmado). Os testes NÃO escrevem no `~/.claude/` real.

**CLAUDE_CONFIG_DIR arm:**
`env HOME="$FAKE_HOME_SEM_CLAUDE" CLAUDE_CONFIG_DIR="$POISONED_DIR" GOCACHE=... go test ./internal/commands/ -count=1` → `ok 59.475s` (0 FAIL, rc=0). O `Unsetenv("CLAUDE_CONFIG_DIR")` existente bloqueia esse canal. O braço em falta é HOME.

**Fake git — medição do `LookPath`:**
```go
// PATH inclui FAKE_BIN/git (script exit 127) antes de /usr/bin
exec.LookPath("git")  // → path preenchido, err=nil
exec.Command("git","--version").Run()  // → exit status 127
```
Com fake git + HOME sem claude: `go test ./internal/validator/ -count=1` → 109 FAIL.

**Exit code collapse:**
`go test ./exitpkg/ -count=1` → rc=1 quando TestMain chama `os.Exit(2)`.
`go test -c -o exitpkg_test ./exitpkg/ && ./exitpkg_test` → exit code=2 (preservado no binário).

**Re-exec em `commands` — apenas linha 472 é plain re-exec:**

| Linha | Contexto | Modo | Alcança isolamento ML-1A? |
|---|---|---|---|
| 58 | SPAWN grandchild | sai antes do isolamento | não |
| 197 | `defaultExecForgeCLI` | t.Setenv(SHORT_SLEEP) em L191 → sai antes | não |
| 472 | `exec.Command(os.Args[0], "-test.run=^$")` | plain re-exec, sem env auxiliar | sim |
| 555 | `runNetworkCmd(os.Args[0], ...)` | t.Setenv(SPAWN) antes → sai antes | não |
| 626 | hook script git push | t.Setenv(SPAWN) em L639 → sai antes | não |

Linha 472 após ML-1A: filho percorre TestMain completo, cria próprio HOME temporário, não encontra testes, faz RemoveAll do HOME próprio, exit 0. O pai usa apenas `probe.ProcessState`. Sem interferência.

**`go build` subprocessos em `commands`:**
5 testes chamam `exec.Command("go","build","./cmd/trackfw")` sem setar `cmd.Env`: `guard_subprocess_test.go:38`, `ship_test.go:1236`, `root_test.go:184`, `barrier_fragment_test.go:245`, `barrier_contract_test.go:105`. O `internal/validator` não chama `go build` em testes (grep confirmado: sem resultado). O problema de cache fria afeta exclusivamente `commands`.

### 3.2 AC5 — Isolamento de HOME (ML-1A)

**Falso negativo (isolamento ausente):**
- Removida `os.Setenv("HOME", home)` do novo TestMain → com HOME real (tem `.claude/settings.json` com hooks e guard global) e PATH sem trackfw → 15 FAIL.
- Capturado por: `TestIsolatedHomeCoversBothChannels` em `testenv_isolation_test.go`.
- Contra-braço verificável: revert Setenv HOME → sentinela reporta `HOME = "/Users/kgsaran", want the synthetic home`.

**Falso positivo (HOME do TestMain sobrescreve `t.Setenv("HOME", ...)` de teste específico):**
- `t.Setenv` salva e restaura o valor atual de HOME (após ML-1A: `isolatedTestHome`). Sequência: TestMain → `HOME=X`; `t.Setenv("HOME", Y)` → `HOME=Y`; restore → `HOME=X`. Sem conflito.
- `update_harness_test.go` tem 13 ocorrências de `t.Setenv("HOME", home)` — todas usam home local ao teste. Sem regressão.

**Lacuna no molde do validator — canal de produto:**
O sentinela do validator verifica apenas HOME e USERPROFILE. Uma migração de `homedir.Dir()` para `agentconfig.ClaudeConfigDir()` em `commands` não seria capturada. A Prescrição 3 fecha esse gap.

### 3.3 AC3 — Falha rápida sem git (ML-2A)

**Falso negativo (LookPath puro):**
LookPath sucede para script exit 127 (medido). Suíte prossegue → 109 FAIL em `validator` (medido). Sentinela captura: re-exec com PATH vazio → `ExitCode()` deve ser 2, não 0.

**Falso negativo (`os.Exit(0)` satisfaz AC incompleto):**
Com `os.Exit(0)` e `-test.run=^$`: `go test` reporta `ok`, sem um `--- FAIL`, sem um test rodado. O AC deve exigir `rc≠0` e `FAIL\t<pkg>` explicitamente (Prescrição 8 + mudança obrigatória no roadmap).

**Falso positivo (`git --version` transiente):**
Custo: rerun manual. Menor que 109 FAIL sem diagnóstico. Aceito (declarado no Residual).

---

## 4 — Residual declarado

1. **Go telemetria no HOME temporário:** `os.UserCacheDir()` no macOS grava telemetria em `$HOME/Library/Application Support/go/telemetry` via Go runtime (não produto). Com HOME isolado, vai para o diretório temporário (descartado com cleanup). Aceito.

2. **`GIT_CONFIG_GLOBAL` externo:** se o desenvolvedor tem `GIT_CONFIG_GLOBAL` no shell, git lê independentemente de HOME. Testes que commitam já neutralizam via `t.Setenv("GIT_CONFIG_GLOBAL", ...)`. Aceito.

3. **`git --version` transiente:** falso positivo raro. Custo = rerun manual. Menor que 109 FAIL sem diagnóstico. Aceito.

4. **Pacotes sem TestMain na closure (`guard`, `serve`, `sync`, `agentconfig`, `integrations`, `config`, `discover`):** passam com HOME fake hoje (medido) porque seus testes não exercitam o código que chama `homedir.Dir()`. Um teste novo sem `t.Setenv` vaza silenciosamente — nenhum sentinela de pacote captura isso. Aceito como risco de revisão de código.

5. **PATH no gate de ML-1A:** `$(dirname "$(command -v go)"):/usr/bin:/bin` quebra se go estiver em `/opt/homebrew/bin` junto com trackfw. O sentinela `TestIsolatedHomeCoversBothChannels` captura ausência de isolamento independentemente do PATH do gate.

6. **GOENV e GOPATH herdados apontam para HOME real:** a Prescrição 2 preserva GOCACHE, GOMODCACHE, GOENV e GOPATH antes de trocar HOME. Isso significa que `go env` rodado dentro de um teste isolado ainda aponta para `~/go` real via GOPATH e `~/Library/Application Support/go/env` via GOENV. O produto não usa GOENV/GOPATH diretamente. Aceito.

7. **Builds a frio no CI sem o step AC12:** `quality.yml` protege Windows via "Fixar caches fora do home sintético (AC12)" e Linux/macOS via `actions/setup-go@v7 cache: true`. A Prescrição 2 torna o comportamento correto também em ambientes de desenvolvimento onde essas vars não estão no shell. Risco teórico de CI novo sem setup-go adequado é aceito.

---

## Prescrições

### Para ML-1A (artemis-tf)

**Prescrição 1 — Ordem do TestMain de `commands` (combinado com ML-2A):**
Ordem obrigatória em `internal/commands/netexec_test.go`:
1. Modos auxiliares (HANG / SPAWN / SHORT_SLEEP) — unchanged.
2. **Guarda de git — primeira instrução após os modos auxiliares (ML-2A):** `exec.Command("git","--version").Run()` — ANTES do MkdirTemp para não vazar HOME temporário em caso de git ausente.
3. **Captura de caches Go — antes de mudar HOME:** capturar GOCACHE, GOMODCACHE, GOENV, GOPATH via `exec.Command("go","env","GOCACHE").Output()` (e análogos), armazenar em variáveis locais.
4. **Isolamento de HOME (ML-1A):** `MkdirTemp`, `os.Setenv("HOME", home)`, `os.Setenv("USERPROFILE", home)`, `os.Unsetenv("CLAUDE_CONFIG_DIR")`.
5. **Re-setar caches Go:** após Setenv HOME, `os.Setenv("GOCACHE", ...)`, etc.
6. GIT_CONFIG_PARAMETERS.
7. `code := m.Run()`.
8. `chmod -R u+w home`; `os.RemoveAll(home)`; `os.Exit(code)`.

Para `validator`, `auditsurface` e `pathguard` (ML-2A): a guarda de git é a **primeira instrução do TestMain** (sem MkdirTemp nesses três).

**Prescrição 2 — Preservar GOCACHE/GOMODCACHE (blocking):**
Sem preservação, os 5 `go build` subprocessos de `commands` sofrem cache fria após HOME isolation. Medido: build falha com `GOPROXY=off` (rc=1), leva 8.5s com rede (27 módulos). `os.RemoveAll` falha em arquivos somente-leitura do módulo cache. GOCACHE não está no ambiente de shell (confirmado).

Molde recomendado: `internal/generators/guard_parity_helper_test.go` — compila o binário trackfw **uma vez, antes de mudar HOME** (`TestMain`, linha 136+). O mesmo padrão elimina os 5 subprocessos `go build`: compilar UMA VEZ antes de `Setenv HOME`, armazenar caminho em variável de pacote, testes usam o binário pré-compilado. Alternativa conservadora: capturar GOCACHE/GOMODCACHE/GOENV/GOPATH via `go env` e re-injetar (passo 3/5 acima).

**Prescrição 3 — Sentinela com canal de produto:**
`TestIsolatedHomeCoversBothChannels` em `testenv_isolation_test.go` deve ter três verificações:
(a) `os.Getenv("HOME") == isolatedTestHome`;
(b) `os.Getenv("USERPROFILE") == isolatedTestHome`;
(c) `agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)` retorna `(string, error)` onde o primeiro valor começa com `isolatedTestHome` (assinatura: `internal/agentconfig/agentconfig.go:307: func ClaudeConfigDir(r CLIHomeResolver) (string, error)`).

O ponto (c) não está no molde do `validator` e fecha o gap entre variável de ambiente e canal de produto exercitado em `validator_git_branch_guard.go:210`.

**Prescrição 4 — PATH no gate do ML-1A:**
Documentar que o gate assume que go não está em `/opt/homebrew/bin` junto com trackfw, ou usar caminho explícito de go.

### Para ML-1B (ares-tf)

**Prescrição 5 — Usar `git --version` no `check-prereqs`:**
`git --version` (rc=0), não `command -v git`. Consistente com a guarda do TestMain de ML-2A.

**Prescrição 6 — Sem bypass via variável de ambiente:**
`check-prereqs` não aceita `SKIP_PREREQS=1` ou similar.

### Para ML-2A (artemis-tf)

**Prescrição 7 — `git --version` em vez de `LookPath`:**
`exec.Command("git","--version").Run()` nos 4 pacotes. LookPath falha em distinguir git funcional de git quebrado (medido).

**Prescrição 8 — Atualizar AC3 no roadmap (para o arquiteto):**
O AC3 positivo deve exigir: (a) `rc≠0`, (b) linha de diagnóstico no stderr, (c) `FAIL\t<pkg>` na saída, (d) ausência de `PASS` e `ok`. "Zero `--- FAIL`" sozinho é satisfeito por `os.Exit(0)` (medido).

**Prescrição 9 — Sentinela por pacote:**
Um sentinela em cada um dos 4 pacotes. Re-executa `os.Args[0] -test.run=^$` com `PATH=<t.TempDir() vazio>` (portável, funciona em Windows). Assertions no binário: `ExitCode()==2`, linha de diagnóstico no stderr, ausência de `PASS` no stdout.

**Prescrição 10 — Arm POSIX do sentinela:**
Segundo arm (marcado `if runtime.GOOS != "windows"`): PATH com script shell `exit 127` como fake git. Discrimina LookPath de `git --version`: LookPath encontraria o fake, `git --version` falharia — afirmando que a guarda usa `git --version` e não apenas presença de executável.

---

## Itens de roadmap para o arquiteto

1. **ML-0A:** marcar ✅ e as três ACs `[x]`.
2. **ML-2A ação 1:** substituir `exec.LookPath("git")` por `exec.Command("git","--version").Run()`.
3. **ML-2A AC3:** reescrever para exigir rc≠0 e `FAIL\t<pkg>` (não apenas zero `--- FAIL`).
4. **ML-2A `Files affected`:** adicionar sentinelas por pacote em `auditsurface`, `pathguard`, `validator`.
5. **ML-1A `Actions`:** documentar Prescrição 2 (preservação de GOCACHE/GOMODCACHE e molde `generators/guard_parity_helper_test.go`).

---

## Verificação dos gates da wave

```bash
test -f docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md
# rc=0

grep -q 'Prescrições' docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md
# rc=0
```
