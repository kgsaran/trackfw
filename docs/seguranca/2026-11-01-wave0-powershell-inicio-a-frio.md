---
roadmap: ROADMAP-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md
ml: ML-0A
date: 2026-11-01
author: hades-tf
---

# Wave 0 — Threat Model: PowerShell início a frio na matriz de guard

REQ: `docs/req/REQ-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md`
Issue: #649

---

## 1. Completude da enumeração

**Pergunta:** existe outro site no repositório que chame PowerShell (`pwsh`, `powershell`) sob
prazo fixo ou com risco de bloqueio indefinido? Inclui chamadas por variável.

### Varredura executada

```bash
grep -rn "CommandContext\|WithTimeout\|pwsh\|powershell" --include='*_test.go' .
grep -rn "CommandContext\|WithTimeout\|pwsh\|powershell" --include='*.go' . | grep -v '_test.go'
grep -rn "WaitForExit\|-Timeout\|TimeoutSec\|Wait-Process" scripts/
grep -rn "p21" scripts/windows-repro/run.ps1
# Executores por variável:
# discover.go:57 exec.CommandContext(ctx, name, ...) — callers: lefthook, npm, npx
# netexec.go:275 exec.CommandContext(ctx, name, ...) — callers: gh, adapter.CLIName (gh/gitlab-cli)
# validator.go:4781/4788 gitCommandCtx(...) — executa "git check-ignore" (não PS)
```

Fechamento do item "por variável": os dois executores por variável (`runExternalCommand` em
`discover.go:57` e `runNetworkCmd` em `netexec.go:275`) recebem apenas `lefthook`, `npm`, `npx`,
`gh` ou `adapter.CLIName` (gh/gitlab-cli). Nenhum recebe `powershell` ou `pwsh`. A única
ocorrência literal de `"powershell"` em Go não-teste fora da lista abaixo está em
`internal/guard/credential.go:431` (comparação de nome, não execução).

### Sites classificados

| Arquivo | Linha | Prazo | Binário | Classificação |
|---|---|---|---|---|
| `internal/generators/guard_line_shell_test.go` | 248 | 15 s | `pwsh`/`powershell` | **MESMO MECANISMO — site corrigido** |
| `internal/validator/validator_guard_binary_probe.go` | 253 | **5 s** | `powershell` | **MESMO MECANISMO — código de produto** |
| `internal/generators/fail_closed_d11_behavior_test.go` | 155 | **nenhum** (`exec.Command`) | `pwsh` | DIFERENTE (mecanismo: hang indefinido) |
| `internal/pathguard/main_test.go` | 56, 74 | 30 s | `os.Args[0]` (binário de teste) | não é PS |
| `internal/validator/main_test.go` | 133, 165 | 30 s | `os.Args[0]` | não é PS |
| `internal/auditsurface/main_test.go` | 62, 80 | 30 s | `os.Args[0]` | não é PS |
| `internal/commands/attention_test.go` | 240, 253 | 6 s | binário `trackfw` | não é PS |
| `internal/generators/attention_hooks_test.go` | 204 | nenhum | `sh`/`bash`/`zsh` | não é PS |
| `scripts/check-install-ps1.ps1` | 104 | 120 s | subprocesso PS | não se aplica: 120 s >> 15 s medido |
| `scripts/windows-repro/p21-attention-native.ps1` | 154–155 | 5–18 s | `powershell` | não corre em CI: `grep p21 scripts/windows-repro/run.ps1` não retornou nada |

**Resultado: a enumeração NÃO está fechada com a afirmação "sítio único".**

O REQ varreu apenas `*_test.go`. O site de produto `validator_guard_binary_probe.go:253` não
estava no escopo. A REQ deve ser ampliada ou a Regra Dura de Causa Raiz aplicada (ver P-1).

### Site de produto: `guardGetPSPolicy` — mesma causa, fail-open de segurança

`internal/validator/validator_guard_binary_probe.go:253`:

```go
var guardProbeTimeout = 5 * time.Second

var guardGetPSPolicy = func() string {
    ctx, cancel := context.WithTimeout(context.Background(), guardProbeTimeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", "Get-ExecutionPolicy")
    out, err := cmd.Output()
    if err != nil {
        return ""   // falha silenciosa em qualquer erro, inclusive timeout
    }
    return strings.TrimSpace(string(out))
}
```

Chamador `guardCheckPS1RestrictedPolicy` (linha 322–338):

```go
func guardCheckPS1RestrictedPolicy() []string {
    resolved := guardFindTrackfwForPS()
    if resolved == "" || !strings.HasSuffix(strings.ToLower(resolved), ".ps1") {
        return nil   // retorna nil antes de chamar guardGetPSPolicy se não é .ps1
    }
    policy := guardGetPSPolicy()
    if strings.EqualFold(policy, "Restricted") {
        return []string{"... Restricted execution policy ..."}
    }
    return nil   // nil quando policy == "" (timeout)
}
```

A guarda de `.ps1` (linhas 327–329) descarta o caminho quando trackfw não resolve para `.ps1`.
Portanto o fail-open é restrito à condição: **trackfw resolve para `.ps1` E o cold start de
`powershell.exe` excede 5 s**. Nessa condição, `strings.EqualFold("", "Restricted") == false`
e o aviso de política Restricted é silenciosamente omitido — um usuário com `.ps1` + Restricted
não recebe o diagnóstico. **Fail-open para o aviso de política PS.**

O prazo de 5 s é mais curto que o de 15 s que causa a falha do #649; a mesma cauda (2,91–9,86 s)
cruza esse limiar. O discriminante `guardProbeEndedByDeadline` existe na mesma função mas não é
aplicado aqui. A função é "Replaced in tests" — nenhum teste real exercita o caminho com processo
vivo. Severidade: **MEDIUM**. Pendente da decisão do arquiteto — ver P-1.

---

## 2. Modelo de ameaça

### Ameaça A: aquecimento mascara uma regressão real do guard (linha que pende sob PS)

O aquecimento executa `<psBin> -NoProfile -NonInteractive -Command "exit 0"`. Esse comando não
aciona nenhuma lógica de guard, não spawna stub, não abre pipe para o `trackfw`. Uma linha de guard
que pendurasse sob PS ainda atingiria o prazo de 15 s na matriz; com o discriminante do AC2, o
teste reportaria "timeout" em vez de "exit 1". O aquecimento não mascara regressões reais.

**Veredito: ameaça não confirmada.**

### Ameaça B: aquecimento falha em silêncio e os arms seguintes rodam a frio

**Medido** no scratchpad `oncetest/once_test.go` (código na Seção Apêndice):

```
go test -v -run TestOnceDoWithTFatal ./oncetest/

=== RUN   TestOnceDoWithTFatal/primeiro-arm-fatal
    once_test.go: warmup executou: callCount=1
    once_test.go: warm-up simulado falhou
=== RUN   TestOnceDoWithTFatal/segundo-arm-sem-warmup
    once_test.go: segundo arm: callCount=1 (esperado 1)
--- FAIL: TestOnceDoWithTFatal/primeiro-arm-fatal
--- PASS: TestOnceDoWithTFatal/segundo-arm-sem-warmup
```

`t.Fatal` chama `runtime.Goexit`. Dentro de `sync.Once.Do`, `doSlow` usa
`defer o.done.Store(1)` — o estado "feito" é marcado mesmo quando `Goexit` é propagado. O
primeiro arm falha com a mensagem do warmup. Arms seguintes chamam `once.Do`, encontram "feito"
e pulam a função — **rodam a frio sem mensagem de warmup**.

Raio de explosão correto para a estrutura da matriz: o warmup ocorre em `t.Run(ps.name, ...)`.
Se `t.Fatalf` é chamado ali, ele encerra todo o arm `ps.name` (incluindo os 4 casos RC0/RC2/RC1/
absent) com uma única falha. Os 3 arms restantes do mesmo binário (`credential/powershell`,
`write-scope/powershell`, `credential-global/powershell`) — 12 casos no total — chamam `once.Do`,
encontram "feito" e executam frios sem mensagem. Apenas o primeiro desses 12 casos paga o cold
start (os demais estão aquecidos pelo OS após a tentativa de warmup).

**Veredito: falência parcialmente silenciosa confirmada com `sync.Once` + `t.Fatal`.** Ver P-2.

### Ameaça C: discriminante confunde kill por prazo com saída legítima exit 1

Medido no scratchpad `main.go` com `pwsh` (/opt/homebrew/bin/pwsh, Go 1.25.2, macOS arm64):

```
go run main.go

[timeout-kill]          elapsed=0.103s  err=signal: killed   ctx.Err()=context deadline exceeded  → timedOut=true
[exit-1-natural]        elapsed=0.310s  err=exit status 1    ctx.Err()=<nil>                      → timedOut=false
[exit-0-natural]        elapsed=0.185s  err=<nil>            ctx.Err()=<nil>                      → timedOut=false
[timeout-before-start]  elapsed=0.001s  err=signal: killed   ctx.Err()=context deadline exceeded  → timedOut=true
```

`ctx.Err() == context.DeadlineExceeded` é `true` após kill por prazo e `false` após saída natural
com qualquer código. O discriminante AC2 é correto.

Janela de corrida: irrelevante para cold start >>15 s. Residual aceitável.

**Veredito: discriminante é confiável para o mecanismo do #649.**

### Ameaça D: `sync.Once` com `-count=N` e caminhos diferentes para o mesmo binário

`-count=N > 1`: warmup roda na iteração 1; o cache do SO aquece o binário. Iterações 2..N não
sofrem cold start. Linha do `t.Logf` do warmup aparece apenas na iteração 1.
Dois caminhos para o mesmo binário: dois aquecimentos, inócuo.

**Veredito: sem risco.**

### Ameaça E: `exit 0` pode não aquecer componentes além do carregador de processo

Hipótese: comandos não-resolvidos (`emulate`) podem disparar busca de módulos em PS 5.1
(varredura de `PSModulePath` → `ModuleAnalysisCache`). `exit 0` não aciona esse mecanismo.

**Medição no macOS com `pwsh`** (scratchpad `warmup.go`). Nota: a linha de guard foi digitada
à mão (não lida da constante `guardGitBranchCmdPSPOSIX`); o cache de módulos do `pwsh` 7 no
macOS pode já estar quente de execuções anteriores; números absolutos são < 0,6 s.

```
[cold-exit0]   exit 0:                                                   0.361s
[cold-guard]   emulate -R sh 2>${null-/dev/null}; trackfw guard ...:     0.592s

[warm-exit0]   exit 0 (após exit 0):                                     0.191s
[warmed-guard] guard line (após exit 0):                                 0.536s   ← delta = −56 ms

[warm-guard2]  guard line (após guard):                                  0.409s   ← delta vs. exit-0-aquecido = −127 ms
```

`exit 0` reduz pouco a latência da linha de guard (56 ms vs. 127 ms de redução com guard warmup).
O impacto no Windows PS 5.1 não é mensurável aqui.

**Veredito: hipótese não refutada. AC5 é o gate. Ver P-7.**

### Ameaça F: grandchild segura o stderr pipe após kill do pai

Quando a linha de guard é executada pelo PS, o stub (`trackfw.cmd` ou `trackfw` via shebang) é
um grandchild. Se o PS é morto pelo deadline de 15 s com o stub ainda rodando, o grandchild herda
o fd do stderr pipe. Sem `WaitDelay`, `cmd.Run()` bloqueia até o grandchild fechar o pipe.

Sonda anterior (`waitdelay3.go`) com `bash + pwsh` não reproduziu porque `cmd.Stderr` não estava
definido — sem pipe, nada para drenar. Ao definir `cmd.Stderr = &bytes.Buffer{}` (exatamente como
`runGuardLinePS` faz), o grandchild de `bash` bloqueia o pai:

**Medido** (`waitdelay5.go` e `waitdelay6.go`, processo pai `bash`):

```
go run waitdelay5.go   (pai sai naturalmente; grandchild "sleep 10" segura pipe)

[bash-long-no-waitdelay]    elapsed=10.029s  err=<nil>
[bash-long-with-waitdelay]  elapsed= 2.024s  err=exec: WaitDelay expired before I/O complete
```

```
go run waitdelay6.go   (pai morto em 300ms por deadline; grandchild persiste)

[no-waitdelay]       elapsed=10.029s  err=signal: killed  ctx.Err()=context deadline exceeded
[with-waitdelay-2s]  elapsed= 2.304s  err=signal: killed  ctx.Err()=context deadline exceeded
```

Sem `WaitDelay`: o pai morto no deadline de 300 ms não libera o pipe; `cmd.Run()` bloqueou 10 s
além do kill. Com `WaitDelay=2 s`: retornou em 2,3 s. `ctx.Err()` permanece
`context.DeadlineExceeded` em ambos os casos — o discriminante AC2 continua correto.

Consequência adicional (sem `WaitDelay`, saída natural): `waitdelay5` mostra que quando o pai sai
naturalmente (err=nil) mas o grandchild segura o pipe, com `WaitDelay` o `err` torna-se
`exec.ErrWaitDelay` (não `nil`, não `*exec.ExitError`). O classificador existente em
`runGuardLinePS` (`if exitErr, ok := runErr.(*exec.ExitError); ok { code = exitErr.ExitCode() }`)
ignora `ErrWaitDelay` e mantém `code = 0`. RC0-allow passaria com falso verde quando o stub saiu
com 0 mas o grandchild travou o pipe. Ver P-5 (classificação três vias).

Para o #649: o cold start mata o PS antes do guard executar — nenhum stub é spawado. Ameaça F
afeta cenário futuro (guard que pende sob PS). Medido e confirmado no macOS.

**Veredito: risco confirmado. P-5 obrigatório.**

---

## 3. Alvos de falsificação nos dois sentidos

### AC1 — Aquecimento por binário

| Direção | Falha se | Onde entra | Gate que pega |
|---|---|---|---|
| **Falso negativo** (warmup não protege) | Warmup chamado após o primeiro caso; ou chaveado por guard em vez de binário | Setup do loop | AC5: log de CI não mostra linha do warmup antes de RC0-allow; P-8 |
| **Falso negativo** (Once.Do + Goexit oculta) | arms seguintes rodam frios sem mensagem (medido em `once_test.go`) | `Do` + `t.Fatalf` | Ausência da linha de warmup no log do arm afetado; P-2 |
| **Falso positivo** (warmup bloqueia toda a matriz) | PS retorna != 0 para `exit 0` em alguma variante; warmup usa `t.Fatalf` para code != 0 | Constante de saída do PS | AC4: matrix reprova no macOS |

### AC2 — Timeout distinguível

| Direção | Falha se | Onde entra | Gate que pega |
|---|---|---|---|
| **Falso negativo** (timeout mascarado) | Discriminante usa exit code em vez de `ctx.Err()`. Kill → code 1; indistinguível de `exit 1` natural. | Função extraída | AC3 contra-braço: sem `ctx.Err()`, o teste falha |
| **Falso negativo** (cmd.Run() trava após kill) | Sem `WaitDelay`; grandchild segura pipe; `cmd.Run()` não retorna após kill (medido: 10 s extra) | Função extraída sem `WaitDelay` | P-5: `WaitDelay` obrigatório |
| **Falso positivo** (ErrWaitDelay → code 0) | `WaitDelay` expira com saída natural (código 0 do stub) e grandchild segura pipe → `err=ErrWaitDelay` → classificador padrão: `code=0` → falso verde RC0-allow | Função extraída com classificação dois vias | P-5: classificação três vias |
| **Falso positivo** (exit natural misclassificado) | `ctx.Err()` verificado antes de `cmd.Run()` retornar | Função extraída | AC3 arm normal: `exit 1` com prazo longo → `timedOut=false` |

### AC3 — Teste do discriminante com contra-braço

| Direção | Falha se | Onde entra | Gate que pega |
|---|---|---|---|
| **Falso negativo arm curto** (timedOut nunca true) | Deadline longo demais (pwsh inicia antes); ou contexto pré-cancelado → `ProcessState==nil` | Valor do deadline, comando | Asserção `timedOut==true` reprova; `ProcessState!=nil` falha; P-3 |
| **Falso negativo arm normal** (mutação sobrevive) | Usa `exit 0`: mutação `timedOut=(code!=0)` produz `false` e coincide com esperado | Comando do arm normal | Mutação não falsificada; P-4 |
| **Falso positivo** (AC3 recria #649 no CI de Windows) | AC3 usa `powershell` sem warmup → primeira chamada PS fria → falha com code 1 em vez de "timedOut" | Posição do AC3 e binário alvo | P-9 |
| **Contra-braço fraco** | Remoção de `ctx.Err()` não faz o teste reprovar | Lógica do contra-braço | Contra-braço medido, removido e restaurado; diff confirma reversão |

---

## 4. Residual declarado

**R-1 — `guardGetPSPolicy` pendente de decisão do arquiteto.**
Mesmo mecanismo. Fail-open restrito a: trackfw resolve para `.ps1` + cold start > 5 s + política
Restricted. Fora do escopo de implementação desta REQ (escopo negativo). Não pode ser descartado
pelo escopo: a Regra Dura exige ou novo ML ou afirmação escrita com medição que distingue a causa.
Ver P-1.

**R-2 — `fail_closed_d11_behavior_test.go:155` sem prazo.**
Mecanismo diferente (hang indefinido, não cold-start com prazo fixo). Não é a mesma causa.
`pwsh` no macOS inicia em < 0,5 s; o teste pula no Windows. Baixo risco. Não bloqueante.

**R-3 — `exit 0` pode não aquecer todos os componentes de cold start do PS 5.1 no Windows.**
Hipótese, não confirmada. Medição macOS com `pwsh` mostra delta de 127 ms entre warmups.
Impacto no Windows PS 5.1 não mensurável aqui. AC5 é o gate; trigger de P-7: RC0-allow > 3 s.

**R-4 — Janela de corrida do discriminante (±µs no deadline).**
Irrelevante para o #649 (cold start >>15 s). Aceito.

---

## Prescrições

P-1: **Regra Dura: o arquiteto declara o tratamento de `guardGetPSPolicy`.**
Ou (a) acrescentar ML à REQ para aplicar `guardProbeEndedByDeadline` em `guardGetPSPolicy`:
quando o prazo estoura, emitir "política PS não determinada (timeout 5 s)" no diagnóstico em vez de
retornar `""` — ruído limitado a: trackfw é `.ps1` + cold start > 5 s; ou (b) escrever na REQ por
que a causa difere, com a medição que sustenta a diferença. Silêncio viola a Regra Dura de Causa
Raiz.

P-2: **Usar `sync.OnceValues` ou mapa com resultado — não `sync.Once` + `t.Fatal`.**
Medido (`once_test.go`): `t.Fatal` dentro de `Once.Do` marca o Once como "feito"; 3 arms e 12
casos seguintes rodam frios sem mensagem. Usar `sync.OnceValues` (Go 1.21+) para armazenar
`(dur time.Duration, err error)` por binário. Todo arm PS re-verifica o resultado armazenado e
chama `t.Fatalf("warm-up de %s falhou: %v (%.2fs)", psBin, warmupErr, warmupDur.Seconds())` antes
de qualquer asserção. Todos os arms afetados emitem a mensagem de falha do warmup.

P-3: **AC3 arm curto: usar `Start-Sleep 60` e afirmar `cmd.ProcessState != nil`.**
Deadline de poucos milissegundos sobre `Start-Sleep 60` garante o caminho
`cmd.Start → kill → cmd.Wait`. Afirmar `cmd.ProcessState != nil` verifica que o processo foi
spawado antes do kill (contexto pré-cancelado causa `cmd.Start` a falhar com
`ctx.Err()=DeadlineExceeded` sem spawnar, mascarando o arm curto como falso verde).

P-4: **AC3 arm normal: usar `-Command "exit 1"`, não `exit 0`.**
Com `exit 0`, mutação `timedOut=(code!=0)` produz `false`, que coincide com esperado — sobrevive.
Com `exit 1`, mesma mutação produziria `true`, divergindo. Medido: `exit-1-natural` →
`ctx.Err()=nil` → `timedOut=false` é o resultado correto.

P-5: **Classificação três vias na função de execução extraída (obrigatório).**
Medição (`waitdelay5.go`, `waitdelay6.go`): (a) sem `WaitDelay`, grandchild segura pipe: `cmd.Run`
bloqueia 10 s após kill; (b) com `WaitDelay=2 s`, retorna em 2,3 s; (c) com `WaitDelay`, saída
natural + grandchild: `err=exec.ErrWaitDelay`, `code=0` no classificador padrão → falso verde.
Implementação obrigatória:
```
cmd.WaitDelay = 5 * time.Second   // padrão do projeto (cmdWaitDelay em netexec.go)
// Após cmd.Run():
switch {
case err == nil:                           // code 0
case errors.Is(err, exec.ErrWaitDelay):   // t.Fatalf("WaitDelay: grandchild travou pipe — %v", err)
case exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr): // code = exitErr.ExitCode()
default:                                   // t.Fatalf("erro de execução inesperado: %v", err)
}
```

P-6: **AC5 com `-v` confirmado.**
`windows-full-suites` usa `go test -v -p 1 -parallel 1 -timeout 20m ./...` (quality.yml linha 516).
`t.Logf` é visível para testes que passam. Nenhuma ação adicional.

P-7: **Se AC5 mostrar RC0-allow > 3 s após warmup de `exit 0`, trocar o comando de warmup.**
Substituir por um comando que inclua um nome não-resolvido (ex.: `emulate -R sh 2>$null; exit 0`)
para acionar a varredura de módulos PS antes da matriz. Trigger: RC0-allow > 3 s no CI de Windows.

P-8: **Verificar que o warmup é chaveado pelo binário, não pelo guard ou posição.**
`go test ./internal/generators/ -run 'TestGuardLineShellMatrix/credential/pwsh' -v` deve exibir
a linha do `t.Logf` do warmup do `pwsh`, confirmando que dispara em qualquer arm do binário.

P-9: **O teste AC3 não deve causar cold start no CI de Windows.**
Se o AC3 usa `powershell` e roda antes de `git-branch/powershell/RC0-allow` da matriz, seria a
primeira chamada fria — recriando o #649 no próprio teste de guarda. Mitigação: o AC3 reutiliza
a estrutura de warmup (`OnceValues` por binário) ou usa apenas `pwsh`.

---

## Apêndice — Probes que sustentam os vereditos

### A1. Discriminante (`main.go`)

```go
func probe(label, bin string, args []string, deadline time.Duration) {
    ctx, cancel := context.WithTimeout(context.Background(), deadline)
    defer cancel()
    cmd := exec.CommandContext(ctx, bin, args...)
    err := cmd.Run()
    ctxErr := ctx.Err()
    fmt.Printf("ctx.Err()=%v → timedOut=%v\n", ctxErr, ctxErr == context.DeadlineExceeded)
}
// Casos testados: deadline=100ms/Start-Sleep 5 → timedOut=true; deadline=30s/exit 1 → timedOut=false
```

Resultado completo na Ameaça C.

### A2. `sync.Once` + `t.Fatal` (`oncetest/once_test.go`)

```go
var once sync.Once
var callCount int

func TestOnceDoWithTFatal(t *testing.T) {
    callCount = 0
    once = sync.Once{}
    t.Run("primeiro-arm-fatal", func(t *testing.T) {
        once.Do(func() {
            callCount++
            t.Fatal("warm-up simulado falhou")
        })
    })
    t.Run("segundo-arm-sem-warmup", func(t *testing.T) {
        once.Do(func() { callCount++ })
        // callCount permanece 1: a função não re-executou
    })
}
```

Resultado completo na Ameaça B.

### A3. Grandchild + pipe (`waitdelay6.go`)

```go
script := `(sleep 10 &); sleep 20`   // bash spawna sleep 10 em background, dorme 20s

ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
cmd := exec.CommandContext(ctx, "bash", "-c", script)
cmd.Stderr = &bytes.Buffer{}   // cria pipe — grandchild herda write end

// Sem WaitDelay: elapsed=10.029s (bloqueia até sleep 10 terminar)
// Com WaitDelay=2s: elapsed=2.304s (retorna com "signal: killed")
```

Resultado completo na Ameaça F.
