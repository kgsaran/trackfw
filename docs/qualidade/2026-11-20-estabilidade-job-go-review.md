# Revisão de Qualidade — Estabilidade do job go (ML-2A)

> Autor: hefesto-tf | Data: 2026-11-20
> REQ: `docs/req/REQ-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md`
> Branch: `fix/estabilidade-do-job-go-timeout-e-etxtbsy`

---

## Veredito por AC

| AC | Veredito | Achados |
|----|----------|---------|
| AC1 — `make test`: teto por pacote | **APROVA** | F1 (não-bloqueante) |
| AC2 — `writeExecutableForTest`: ETXTBSY | **APROVA** | — |
| AC3 — cache pwsh isolado | **APROVA** | F2, F3 (não-bloqueantes) |
| AC4 — causa do exit -1 medida | **APROVA** | — |
| AC5 — `make quality` rc=0, CI verde | fora do escopo de ML-2A (ML-FB) | — |

---

## Comandos executados e saídas

```
$ go vet ./internal/generators/
(saída vazia — rc=0)

$ GOOS=windows go vet ./internal/generators/
(saída vazia — rc=0)

$ go test -count=1 -timeout 5m ./internal/generators/
ok  	github.com/kgsaran/trackfw/internal/generators	86.517s
(rc=0)

$ make -n test
# ML-1A (REQ-2026-11-20, #690): teto por pacote — internal/generators em 5m,
# demais pacotes em 2m. ...
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/generators/
pkgs=$(go list ./... | grep -v '^github\.com/kgsaran/trackfw/internal/generators$'); \
[ -n "$pkgs" ] || { echo 'ERROR: lista de pacotes vazia — verificar go list'; exit 1; }; \
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m $pkgs

$ go list ./... | wc -l
31

$ go list ./... | grep -v '^github\.com/kgsaran/trackfw/internal/generators$' | wc -l
30
```

---

## AC1 — `make test`: teto por pacote

**Verificado:**

- `Makefile:14`: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/generators/` — teto 5m para o pacote lento. ✓
- `Makefile:15–17`: pipeline `pkgs=$(go list ./... | grep -v ...)` seguido de vacuidade e `go test -timeout 2m $pkgs`. ✓
- Contagem: `go list ./... | wc -l` = 31; sem generators = 30. Soma = 31. ✓
- `quality.yml:40` e `release.yml:43`: trocado de `go test -timeout 2m ./...` por `make test`. Env block `TRACKFW_DISABLE_EXTERNAL_COMMANDS: "1"` mantido nos dois. ✓
- Nenhum `go test -timeout ... ./...` restante nos dois yamls fora dos sítios explicitamente excluídos pela REQ (quality.yml:361/383/516). ✓

**F1 — não-bloqueante** | `Makefile:15`

`pkgs=$$(go list ./... | grep -v '...')` — sem `pipefail` no Makefile (nenhum `SHELL` ou
`.SHELLFLAGS` declarado). Se `go list` falhar parcialmente (emite alguns pacotes para stdout
e sai com rc≠0), `grep` processa o que recebeu, sai 0, `pkgs` fica não-vazio e a guarda de
vacuidade não dispara — subconjunto de pacotes testado silenciosamente. Não-bloqueante porque
(a) `go list ./...` não tem modo de falha parcial: ele sai com output completo ou sai com
erro e output vazio; (b) se sair com output vazio, grep sai 1, o assignment sai 1, e sem
`set -e` a guarda de vacuidade ainda executa e emite `exit 1`; (c) a primeira linha de
`make test` (`go test ./internal/generators/`) falha independentemente se o módulo estiver
quebrado. Risco teórico, sem cenário observável.

---

## AC2 — `writeExecutableForTest`: ETXTBSY

**Verificado:**

Sítios migrados (confirmados pelo diff e pelo grep `writeExecutableForTest`):

| Arquivo | Linha | Perm | Era `os.WriteFile` |
|---------|-------|------|--------------------|
| `guard_line_shell_test.go` | 375 | 0755 | S1 — único vetor ETXTBSY confirmado pelo CI |
| `guard_thin_wrapper_test.go` | 86 | 0755 | S2 — defesa em profundidade |
| `attention_hooks_test.go` | 230 | 0755 | S3 — defesa em profundidade |
| `fail_closed_d11_behavior_test.go` | 70, 81 | 0755 | S4 — defesa em profundidade |
| `guard_line_shell_test.go` | 475 | 0644 | `makeFakeTrackfwCmd` Windows — 0644, não execve target |

Sítios com `os.WriteFile(..., 0755)` restantes (NÃO migrados): `credential_guard_test.go:391`,
`git_branch_guard_test.go:160,1170`, `update_test.go:642,670,759,787`. Todos gravados com
`exec.Command("bash", scriptPath)` — bash lê o script; o target do `execve` é `bash`,
não o script. Wave 0 §1.2 os classifica como SEGUROS (NÃO execve targets). Nenhum tem
`t.Parallel()` ativo no arquivo (`git_branch_guard_test.go`: 0 hits; `update_test.go`:
0 hits; `credential_guard_test.go`: 1 hit em L837, em função diferente dos callers do helper).
Classificação inalterada após o diff. ✓

**RLock vs Lock:** `writeExecutableForTest` usa `syscall.ForkLock.RLock()`. Correto per
`$(go env GOROOT)/src/syscall/exec_unix.go:19–65`: quem cria fd usa RLock; quem faz fork
usa Lock. Múltiplos writes paralelos coexistem sob RLock; fork (Lock) bloqueia enquanto
qualquer RLock estiver ativo. Não há t.Fatalf dentro da zona travada (apenas o chamador
chama t.Fatalf após retorno do helper, fora da zona). ✓

**Grep de contra-braço** (executado manualmente durante a revisão):

```
$ grep -rn 'os\.WriteFile.*0755\|os\.WriteFile.*0700\|os\.Chmod.*0755\|os\.Chmod.*0700' \
    internal/generators/*_test.go
```

7 linhas restantes: todas SAFE per Wave 0. Nenhuma nova linha de execve target sem proteção.
✓

---

## AC3 — Cache pwsh isolado

**Verificado — 5 sítios de lançamento de pwsh em `guard_line_shell_test.go`:**

| Função | exec.CommandContext linha | newPwshCacheEnv linha | Status |
|--------|--------------------------|----------------------|--------|
| `psProbe` | 193 | 195 | ✓ |
| `execPSCommand` | 564 | 588 | ✓ |
| `psWarmup` (warmDir≠"") | 638 | usa `applyXDGCacheHome` direto (warmDir = template) | ✓ |
| `psWarmup` (warmDir=="") | 638 | 650 — `newPwshCacheEnv("", ...)` fallback ML-1D | ✓ |
| `execPSCommandOpts` | 2034 | 2070 | ✓ |
| `execPSCommandInteractive` | 2963 | 2997 | ✓ |

**`fail_closed_d11_behavior_test.go:~200`** lança pwsh SEM `newPwshCacheEnv`. Confirmado
seguro: `grep -c 't\.Parallel' internal/generators/fail_closed_d11_behavior_test.go` = 0.
Testes sequenciais terminam antes de qualquer goroutine paralela do pacote ser agendada.
Decisão do ML-1D confirmada. ✓

**Thread-safety do mapa `pwshWarmCacheByBin`:**
- `pwshWarmCacheMu sync.Mutex` guarda todas as operações. ✓
- `getPwshWarmCache`: Lock+Unlock; `setPwshWarmCache`: Lock+Unlock; `cleanupPwshWarmCaches`:
  Lock, copia dirs para slice, zera mapa, Unlock, depois remove fora do lock. ✓
- `psWarmup` usa `sync.OnceValues` por binário: somente uma goroutine executa o corpo; as
  demais bloqueiam até o retorno. `setPwshWarmCache` é chamado ao final, garantindo visibilidade
  pós-aquecimento. ✓

**Windows:** `newPwshCacheEnv` retorna env original (no-op) em `runtime.GOOS == "windows"`.
`newPwshWarmTemplate` retorna `("", nil)` no Windows — `psWarmup` cai no else (ML-1D fallback).
`GOOS=windows go vet` rc=0. ✓

**Limpeza:** `cleanupPwshWarmCaches()` chamada em `guard_parity_helper_test.go:183` (após
`cleanupGuardBinary()`, linha 182 do fonte original — confirmado em TestMain). ✓

**F2 — não-bloqueante** | `pwsh_cache_isolation_test.go:~100`

Comentário "Pendente: adicionar cleanupPwshWarmCaches() em guard_parity_helper_test.go:TestMain"
está DESATUALIZADO — a chamada já existe em `guard_parity_helper_test.go:183` (confirmado
por grep). Incoerência de documentação interna ao arquivo. Não afeta comportamento.

**F3 — não-bloqueante** | `pwsh_cache_isolation_test.go:getPwshWarmCache`

`getPwshWarmCache` usa `pwshWarmCacheMu.Lock()` (exclusivo) para leitura. Uma `sync.RWMutex`
com `RLock` para leituras simultâneas seria mais eficiente sob alta concorrência (32 goroutines
paralelas). Em contexto de testes com leituras rápidas (map lookup), a contenção é desprezível.
Correto funcionalmente; sub-ótimo em concorrência de leitura. Não-bloqueante.

---

## AC4 — Causa do exit -1 medida e escrita

**Verificado:**

ML-1C (run 44826888100): exit -1 (`obteve -1`) aparece nos dois braços before/after (105 no
shard 1 do before, 46 no shard 2 do after) — confirma causa independente do ETXTBSY.

ML-1D: causa identificada como corrida no StartupProfileData-* do JIT do .NET (ProfileOptimization)
quando múltiplos processos pwsh compartilham o mesmo `$XDG_CACHE_HOME/powershell` —
`System.IO.FileLoadException: The given assembly name was invalid` com nomes de assembly
embaralhados.

ML-1D2 (run 44842568713): `obteve -1` = 0, `Unhandled exception` = 0 em todos os 4 shards
do after. ✓

Causa escrita em roadmap (ML-1C, ML-1D, ML-1D2) com evidência de CI numerada. ✓

---

## Regra de Reconciliação

Nenhum novo `func Test...` foi entregue no diff. Os dois arquivos novos
(`exec_stub_write_test.go` e `pwsh_cache_isolation_test.go`) são exclusivamente helpers
(nenhuma função com prefixo `Test`). Não há frases de reconciliação a verificar por função
de teste. As modificações em testes existentes (substituição de `os.WriteFile` por
`writeExecutableForTest`) preservam as asserções originais: o helper produz o mesmo
arquivo executável, com a escrita serializada contra forks. Essa afirmação é suportada
pelo CI stress test (ML-1C, run 44826888100: 0 `Text file busy` no after arm).
