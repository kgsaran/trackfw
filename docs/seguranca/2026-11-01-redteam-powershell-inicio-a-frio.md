---
roadmap: ROADMAP-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md
ml: Wave 2 (Red Team)
date: 2026-11-01
author: hades-tf
---

# Red Team — #649: PowerShell início a frio na matriz de guard

REQ: `docs/req/REQ-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md`
Branch: `fix/powershell-inicio-a-frio-na-matriz-de-guard`
CI confirmado: job 124955795693 / run 41342668248 — merge commit `05df3d4`
(`HEAD is now at 05df3d4 Merge fca28fb028ae964dad1a480fa676035118956c19 into bca55ae0...`)
— a árvore testada inclui o HEAD da branch (`fca28fb0`), ou seja, inclui ML-1B.

---

## Metodologia

Todos os vereditos abaixo são sustentados por leitura independente do diff (`git diff
origin/main...HEAD -- internal/`) ou por medição em sondas descartáveis no scratchpad. Onde
o relatório do executor coincide com a medição, isso é notado como corroboração, não como
evidência primária.

---

## 1. Falsificação dos alvos da Wave 0 contra o diff

### AC1 — Aquecimento por binário

**Alvo FN-1: warmup chaveado por guard ou posição, não por binário.**

`psWarmupFns` é `map[string]func() (time.Duration, error)` chaveado pelo caminho absoluto de
`psBin` (retorno de `exec.LookPath`). `ensurePSWarmup(t, psBin)` é chamado antes de qualquer
`t.Run("RC0-allow", ...)`, dentro do arm `ps.name`.

Medição P-8 com arm não-inicial:

```
go test ./internal/generators/ -run 'TestGuardLineShellMatrix/credential/pwsh' -v -count=1
    guard_line_shell_test.go:526: aquecimento de /opt/homebrew/bin/pwsh: 0.37s       ← FRESCO
    guard_line_shell_test.go:526: aquecimento de /opt/homebrew/bin/pwsh: resultado em cache (0.37s)
```

`credential/pwsh` dispara o aquecimento fresco (não depende de `git-branch` ter rodado antes).
`credential-global/pwsh` recebe o cache. Chaveamento por binário confirmado. FECHADO.

**Alvo FN-2: `sync.Once` + `t.Fatal` silencia arms seguintes (medido na Wave 0).**

Implementação usa `sync.OnceValues` (não `sync.Once`): armazena `(time.Duration, error)` por
binário. `ensurePSWarmup` sempre verifica `err != nil` — mesmo em chamada com resultado em
cache — e chama `t.Fatalf(...)`. Arms seguintes cujo warmup teria falhado recebem a mensagem
de falha. FECHADO.

**Alvo FP: PS retorna != 0 para `exit 0`, warmup bloqueia toda a matriz.**

O warmup detecta isso via `if runErr != nil { return d, runErr }` e propaga o erro. A consequência
seria falha ruidosa de todos os arms PS. Risco residual LOW — idêntico ao R-3 da Wave 0.

---

### AC2 — Timeout distinguível

**Alvo FN-1: discriminante usa exit code em vez de `ctx.Err()`.**

`execPSCommand` calcula `r.timedOut = ctx.Err() == context.DeadlineExceeded` após `cmd.Run()`.
Não usa exit code. FECHADO.

**Alvo FN-2: `cmd.Run()` trava após kill sem `WaitDelay`.**

`execPSCommand` define `cmd.WaitDelay = psWaitDelay` (5 s). FECHADO.

**Alvo FP: `ErrWaitDelay` com saída natural (code 0) → falso verde RC0-allow.**

Switch de classificação três vias em `execPSCommand`:

```go
case errors.Is(err, exec.ErrWaitDelay): r.runErr = err
```

`runGuardLinePS` lê `r.runErr != nil` e chama `t.Fatalf`. Não há caminho silencioso. FECHADO
para falso verde. Ver INFO-3 abaixo para a nota sobre atribuição de mensagem.

---

### AC3 — Teste do discriminante com contra-braço

Sonda de mutação executada em `/scratchpad/hades-649-w2/mutation.go` contra
`/opt/homebrew/bin/pwsh`:

| Mutação | arm curto (`Start-Sleep 60`, 2s) | arm normal (`exit 1`, 15s) | Status |
|---|---|---|---|
| original (`ctx.Err() == DeadlineExceeded`) | timedOut=true PASS | timedOut=false PASS | baseline |
| M1 `timedOut = false` (sempre) | timedOut=false **FAIL** | timedOut=false PASS | MORTA por curto |
| M2 `timedOut = (exitCode != 0)` | timedOut=true PASS | timedOut=true **FAIL** | MORTA por normal (P-4) |
| M3 `timedOut = (runErr != nil)` | timedOut=false **FAIL** | timedOut=false PASS | MORTA por curto |
| M4 `timedOut = (ctx.Err() != nil)` | timedOut=true PASS | timedOut=false PASS | EQUIVALENTE |

M4 é mutante equivalente: `context.Background()` com `WithTimeout` só produz
`DeadlineExceeded`; nunca `Canceled`. Indistinguível do original com os casos testados. Não
pode ser morta por esta bateria sem introduzir um terceiro cenário de contexto cancelado
explicitamente. Aceito — sem impacto prático (ver INFO-2).

`r.started = cmd.ProcessState != nil` (P-3): confirmado `true` nos cinco cenários da sonda —
o processo chegou a iniciar antes do kill.

---

### AC4 — Matriz inalterada

macOS local (`go test ./internal/generators/ -run 'TestGuardLineShellMatrix$' -v -count=1`):
8 casos pwsh PASS; 8 casos `powershell` SKIP (binário ausente no macOS — comportamento
esperado pelo AC4, que diz "passa no macOS"). Nenhuma asserção removida ou alterada.

Windows CI (job 124955795693): 16 casos PS PASS — `grep -c '--- PASS:
TestGuardLineShellMatrix/.*/powershell/'` retorna 16, incluindo
`credential-global/powershell/absent` confirmado na última linha. FECHADO.

---

### AC5 — Evidência no CI de Windows

**Números confirmados no log (job 124955795693, árvore fca28fb0):**

| Evento | Log | Timestamp |
|---|---|---|
| aquecimento pwsh.exe | "aquecimento de `C:\Program Files\PowerShell\7\pwsh.exe`: 0.34s" | 19:50:47.2093469Z |
| aquecimento powershell.exe | "aquecimento de `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`: 1.69s" | 19:50:47.2099896Z |
| git-branch/powershell (arm inteiro) | 5.33s | 19:50:47.2216569Z |
| git-branch/powershell/RC0-allow | **1.37s** | 19:50:47.2217702Z |
| TestRunGuardLinePS_TimeoutDiscriminado/curto | PASS 2.01s | 19:50:47.2295678Z |
| TestRunGuardLinePS_TimeoutDiscriminado/normal | PASS 0.30s | 19:50:47.2296597Z |

**Aritmética de integridade:** `git-branch/powershell` = 1.37 + 0.73 + 0.73 + 0.81 ≈ 3.64s
de chamadas afirmadas + aquecimento de 1.69s que precede o arm = total de ~5.33s. O log
reporta 5.33s. Consistente.

**AC5 satisfeito literalmente.** A linha do warmup do `powershell` é visível; RC0-allow está
em 1.37s, abaixo do limiar de 3s.

**Análise da amostra única e residual:**

O aquecimento de 1.69s está abaixo do piso histórico de cold start para `RC0-allow` (2.91s
nos 17 jobs verdes da REQ). Isso é compatível com o custo de cold start ter sido deslocado
para o warmup, e com o RC0-allow restante sendo o custo aquecido (~0.7s de guard + ~0.6s de
residual de primeiro spawn do stub ou cmd.exe). O delta +0.6s entre RC0-allow (1.37s) e os
demais casos aquecidos (0.71–0.81s) é compatível com a hipótese de que `exit 0` não aquece
todos os componentes do cold start do PS — mas uma única amostra não distingue entre causas.

O piso histórico de cold start (2.91s para RC0-allow "aquecido naturalmente" pelo OS) sugere
uma estimativa de ~2.2s de custo pós-warmup natural (~2.91 − ~0.7s de guard); o warmup de
1.69s está abaixo desse piso, o que indica que este runner foi rápido ou que `exit 0` reduziu
parcialmente o custo. O cenário original de 9.86–15.12s de cold start (cauda pesada) não foi
reproduzido nesta execução.

A estrutura garante a mitigação por construção (warmup com 60s de prazo antes de qualquer
chamada afirmada). O trigger de P-7 (RC0-allow > 3s) não foi acionado. Confirmar em próxima
execução de runner lento.

---

### AC6 — Sonda de política sem fail-open

Diff confirmado: `guardPSPolicyTimeout = 20 * time.Second` (era `guardProbeTimeout` de 5s).
`guardCheckPS1RestrictedPolicy` retorna `([]string, bool)`. Para `policy == ""`: retorna
`(["... could not be read ..."], false)`. `guardBinaryProbeOnce` continua para a sonda do
Git Bash quando `definitive=false` (AC6 ML-1B).

Testes passam localmente e no CI (CI usa `CurrentGOOS` overrideado para "windows" — não
chama binários reais; os seams estão todos mockados, portanto o comportamento de runtime real
do Windows não é exercitado pelos testes unitários):
- `TestGuardBinaryProbeOnce_WindowsPS1PolicyUnreadable_Warning` — "could not be read" para policy=""
- `TestGuardBinaryProbeOnce_WindowsPS1PolicyUnreadableWithGitBashViolation_BothMessagesAppear` — 2 msgs
- `TestGuardBinaryProbeOnce_WindowsPS1Restricted_GitBashNaoRoda` — sonda do Git Bash não roda

**MEDIUM-1: severidade da mensagem "ilegível" contradiz o texto do AC6.**
Ver seção de achados.

---

## 2. Veredito por prescrição

| Prescrição | Atendida? | Evidência |
|---|---|---|
| P-1 (Regra Dura: tratamento do `guardGetPSPolicy`) | Atendida como AC6 | `guardPSPolicyTimeout=20s`; `guardCheckPS1RestrictedPolicy` retorna `(msgs, definitive)` |
| P-2 (`sync.OnceValues` + re-verificação por arm) | Atendida | `psWarmupFns map[string]func()(time.Duration,error)`; `ensurePSWarmup` checa `err!=nil` sempre |
| P-3 (arm curto: `Start-Sleep 60` + `ProcessState!=nil`) | Atendida | Sonda M1-M4: `r.started=true` em todos; `Start-Sleep 60` confirmado no diff; P-3 checa `r.started` |
| P-4 (arm normal: `exit 1`) | Atendida | Sonda M2: mutação `timedOut=(exitCode!=0)` falha no arm normal com `exit 1` |
| P-5 (classificação três vias + `WaitDelay`) | Atendida | Switch em `execPSCommand`; `cmd.WaitDelay=psWaitDelay(5s)` |
| P-6 (AC5 com `-v` confirmado) | Atendida | Log mostra linhas de `t.Logf`; quality.yml usa `-v` |
| P-7 (trocar warmup se RC0-allow > 3s) | Não acionada | 1.37s << 3s |
| P-8 (warmup chaveado pelo binário) | Atendida | `-run TestGuardLineShellMatrix/credential/pwsh` mostra warmup fresco no arm credential |
| P-9 (AC3 usa só `pwsh`) | Atendida | `TestRunGuardLinePS_TimeoutDiscriminado`: `exec.LookPath("pwsh")` apenas |

---

## 3. Achados classificados

### MEDIUM-1 — Mensagem "política ilegível" tem severidade de violation, não aviso

**Severidade:** MEDIUM

**Evidência — rastreamento da mensagem "could not be read":**

1. `guardCheckPS1RestrictedPolicy()` → `(["... could not be read ..."], false)` quando `policy == ""`
2. `guardBinaryProbeOnce` → `return append(policyMsgs, guardGitBashBinaryProbe(bin)...)`
   — retorna `[]string`, sem distinção de severidade
3. `validateGuardHookResolvable` (`validator_credential_guard.go:730`) →
   `msgs = append(msgs, probeMsgs...)`
4. `validator.go:1059` → `applyRule("credential_guard_hook_resolvable", append(credentialGuardHookMsgs,
   credentialGuardGlobalHookMsgs...), &violations, &warnings)`
5. `ruleSeverity("credential_guard_hook_resolvable")` → não está em `ruleDefaults` → retorna `"error"`
6. Resultado: a mensagem cai em `violations`, não em `warnings`

O AC6 da REQ diz: *"emite um aviso de que a política não pôde ser lida"*. "Aviso" implica
warning. A implementação a roteia com severidade error, tornando-a um bloqueador de
`trackfw validate`.

**Impacto:** Windows com `.ps1` trackfw no PATH onde `powershell.exe` não executa (AppLocker,
GPO, path bloqueado) ou onde o cold start excede 20s em runner lento recebe falha de validate
em vez de aviso. Os usuários-alvo do canal npm são exatamente empresas com restrições de
segurança — o cenário não é hipotético para esse público.

**Por que os testes não pegaram:** `TestGuardBinaryProbeOnce_Windows*` para em `guardBinaryProbeOnce`'s `[]string`, que não carrega informação de severidade. O caminho `applyRule` → `violations` não é exercitado por nenhum teste novo.

**Escape hatch via `trackfw.yaml` não é mitigação suficiente:** `credential_guard_hook_resolvable` está em `credentialGuardAnchoredRules`. Quando o anchor (`origin/main`) está disponível, `ruleSeverity` usa `credentialGuardStricterSeverity(mainSev, diskSev)` — se `origin/main` tem a regra em "error", o downgrade via `trackfw.yaml` é ineficaz. E mesmo quando o escape funciona (`originAnchorNotSet` ou `originAnchorRefUnreadable`), um `rules: { credential_guard_hook_resolvable: warning }` no `trackfw.yaml` também rebaixa as violações definitivas (Restricted, Git Bash) — não há granularidade por mensagem.

**Número de chamadas a `guardGetPSPolicy` por `validate()`:** `guardBinaryProbeOnce` tem dois
call sites de produção sem memoização própria:
- `validator_credential_guard.go:730` (em `validateGuardHookResolvable`)
- `validator_git_branch_guard.go:418` (em `validateGuardGlobalHookResolvable`)

Cada um pode ser chamado por até dois callers por scope (credential e git-branch → project +
global = 4 callers totais via as funções wrapper). Se `powershell.exe` trava em todas as
chamadas: até 4 × 20s = 80s de `validate()` bloqueado, e a mensagem "ilegível" duplicada 4
vezes. Severidade desta sub-observação: LOW.

**Correção proposta para ML-1C:**

O contrato de `guardBinaryProbeOnce` retorna `[]string` (um canal, sem severidade). Para
rotear a mensagem ilegível via `applyRuleWarnOnly`, o contrato precisa expor dois canais.
Mudança mínima em `guardBinaryProbeOnce`:

```go
// Antes: func guardBinaryProbeOnce(hasPSPosixEntry bool) []string
// Depois: func guardBinaryProbeOnce(hasPSPosixEntry bool) (violations, warnings []string)
```

Arquivos afetados:
- `internal/validator/validator_guard_binary_probe.go`: assinatura e retorno de `guardBinaryProbeOnce`; `guardCheckPS1RestrictedPolicy` já separa `(msgs, definitive)` — o caso `definitive=false` vai para `warnings`, o caso `definitive=true` e os msgs do Git Bash vão para `violations`
- `internal/validator/validator_credential_guard.go:730`: consumer do retorno (agora dois valores)
- `internal/validator/validator_git_branch_guard.go:418`: idem
- `validator.go:1059`: `applyRule(...)` para violations; novo `applyRuleWarnOnly(...)` para warnings (ambas as regras, project + global)
- `validator.go:1593` (ValidateTagged): mesma mudança para `applyRuleTagged` / `applyRuleWarnOnlyTagged`

Testes afetados:
- `TestGuardBinaryProbeOnce_WindowsPS1PolicyUnreadableWithGitBashViolation_BothMessagesAppear`:
  afirmação `len(msgs) != 2` (sobre um único canal) precisa de atualização para verificar
  violations e warnings separadamente. O aviso ilegível deve aparecer em `warnings`; a
  violação do Git Bash em `violations`.
- Teste novo obrigatório (reconciliação do ML-1C): chamar `Validate` ou `ValidateTagged` com
  os seams configurados para `.ps1` + `policy == ""` e afirmar que a mensagem "could not be
  read" aparece em `warnings`, não em `violations`. Frase de reconciliação: "este teste afirma
  que a mensagem de política ilegível não bloqueia `trackfw validate` sob a severidade padrão
  de `credential_guard_hook_resolvable`".

---

### INFO-1 — Residual de P-7 não exercitado nesta execução

Trigger: RC0-allow > 3s após warmup de `exit 0`. Não disparado (1.37s). Permanece como
residual de R-3. Confirmar em próxima execução de runner lento.

---

### INFO-2 — Mutante equivalente M4 (`ctx.Err() != nil`) sobrevive à bateria AC3

`timedOut = (ctx.Err() != nil)` é comportalmente equivalente a `ctx.Err() ==
context.DeadlineExceeded` para `context.Background()` com `WithTimeout`. Nenhum teste pode
matar este mutante sem introduzir um cenário de contexto cancelado. Aceito — sem impacto
prático.

---

### INFO-3 — Atribuição de mensagem incorreta: ErrWaitDelay + ctx expirado (não medido)

Cenário hipotético: processo sai naturalmente em t=14.9s; contexto expira em t=15.0s;
grandchild segura pipe; `cmd.Run()` retorna `ErrWaitDelay` em t≈20s.

Nesse estado: `ctx.Err() == DeadlineExceeded` (true) e `r.runErr = ErrWaitDelay`. Em
`runGuardLinePS`, a verificação de `r.timedOut` precede a de `r.runErr`, então a mensagem
seria `"timeout: ... prazo da chamada afirmada esgotado"` em vez de `"ErrWaitDelay: grandchild
travou pipe"`. Não medido. O teste falha fechado — não há pass silencioso. Atribuição
enganosa mas inofensiva. Aceito como residual.

---

## 4. Veredito final

| AC | Status |
|---|---|
| AC1 | ATENDIDO — warmup correto por binário, sync.OnceValues, P-8 confirmado |
| AC2 | ATENDIDO — três vias, WaitDelay, discriminante por ctx.Err() |
| AC3 | ATENDIDO — M1/M2/M3 mortos; M4 equivalente (INFO-2) |
| AC4 | ATENDIDO — 8 pwsh PASS + 8 powershell SKIP macOS; 16 PS PASS Windows CI |
| AC5 | ATENDIDO com residual — RC0-allow 1.37s < 3s; amostra de runner rápido (P-7 não acionado) |
| AC6 | PARCIALMENTE ATENDIDO — prazo corrigido para 20s, aviso emitido, sonda do Git Bash continua; severidade incorreta (MEDIUM-1) |

**Recomendação:** não mergear enquanto MEDIUM-1 não estiver tratado. O AC6 da REQ declara
"aviso"; a implementação produz violation (bloqueador de validate). ML-1C é o tratamento —
pequeno em linhas mas afeta a assinatura de `guardBinaryProbeOnce` e todos os seus consumers.
O ML deve incluir um teste que passa pelo caminho `applyRule` → `warnings` para cobrir o
defeito que os testes AC6 atuais não podem ver.

---

## Nota para o arquiteto — candidato a vault

Padrão estrutural que este achado representa: uma função que retorna `[]string` apaga
informação de severidade. Quando esse `[]string` entra em `applyRule` de uma regra com
severidade "error", qualquer mensagem de aviso vira violação. Este padrão está potencialmente
presente em todos os callers de `guardBinaryProbeOnce`. A regra de design seria: funções que
retornam mensagens mistas (algumas definitivas, outras incertas) devem separar os dois canais
no retorno, não deixar o caller inferir pela posição ou pelo conteúdo da string.
