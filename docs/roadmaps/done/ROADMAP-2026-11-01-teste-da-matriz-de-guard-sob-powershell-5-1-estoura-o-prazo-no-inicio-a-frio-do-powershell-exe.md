---
status: done
date: 2026-11-01
req: "docs/req/REQ-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md"
squad: "hades-tf, artemis-tf, apolo-tf"
---

# Roadmap: Teste da matriz de guard sob PowerShell 5.1 estoura o prazo no início a frio do powershell.exe

> Created: 2026-11-01 | Status: done

## Context
REQ: docs/req/REQ-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md
Issue: #649

A medição do arquiteto está detalhada na REQ. Em 18 jobs distintos, a primeira chamada ao
`powershell.exe` levou de 2,91 a 9,86 s nos verdes e 15,12 s na falha; as chamadas seguintes, de 0,4
a 2,2 s. A causa é o início a frio, e a correção é aquecer o binário e distinguir a falha por timeout.

Sítio único: `internal/generators/guard_line_shell_test.go`. A varredura de issues abertos achou só o
#649.

## Acceptance Criteria
- [x] AC1–AC6 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-01. O discriminante `ctx.Err()` foi medido: kill por prazo dá `DeadlineExceeded`, e saída natural com 1 ou 0 dá `nil`. As prescrições P-2 a P-9 foram aceitas e entram no ML-1A. O P-1 foi conferido pelo arquiteto na leitura de `internal/validator/validator_guard_binary_probe.go`, linhas 253 e 325 a 337: o `guardGetPSPolicy` chama `powershell` com 5 s e devolve `""`, e o `guardCheckPS1RestrictedPolicy` lê `""` como não Restricted. É a mesma causa, então vira o AC6 e o ML-1B nesta REQ (Regra Dura de Causa Raiz).
**Files affected:** `docs/seguranca/2026-11-01-wave0-powershell-inicio-a-frio.md`
**Actions:**
1. Completude da enumeração: confirmar, por grep de mecanismo (`CommandContext`, `WithTimeout`, `pwsh`, `powershell`), que nenhum outro teste do repositório chama PowerShell sob prazo fixo. Incluir `scripts/` e os testes `.ps1`, se houver.
2. Modelo de ameaça:
   - o aquecimento pode mascarar uma regressão real do guard, por exemplo uma linha que pendura sob PowerShell?
   - o aquecimento pode falhar em silêncio, com skip ou `t.Logf` sem falha, e deixar a matriz rodar a frio?
   - o discriminante de timeout pode confundir kill por prazo com saída legítima?
3. Alvos de falsificação nos dois sentidos para o AC1, o AC2 e o AC3.
4. Residual declarado.
**Acceptance criteria:**
- [x] As quatro seções respondidas com evidência (comando e resultado)
- [x] Prescrições em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-01-wave0-powershell-inicio-a-frio.md
grep -q '^## Prescrições' docs/seguranca/2026-11-01-wave0-powershell-inicio-a-frio.md
```

## Wave 1 — Implementação (2 executores em paralelo, arquivos disjuntos)
> Dependencies: Wave 0 auditada. O ML-1A só toca `internal/generators/guard_line_shell_test.go`; o ML-1B só toca `internal/validator/` e `docs/cli-parity.md`.

### ML-1A — Aquecimento e timeout distinguível na matriz PS (artemis-tf) — AC1 a AC4
**Status:** ✅ Concluído — auditado em 2026-11-01. Entregue: `execPSCommand` com prazo como parâmetro, `WaitDelay` de 5 s e classificação em três vias (`ErrWaitDelay` e erro que não é `ExitError` caem em `runErr`, que dá `t.Fatalf`). `timedOut` vem de `ctx.Err() == DeadlineExceeded`, e o `runGuardLinePS` dá `t.Fatalf` dizendo "timeout", o binário e o prazo. O aquecimento usa `sync.OnceValues` por binário, com 60 s, `t.Logf` e conferência do resultado em todo braço PS. `TestRunGuardLinePS_TimeoutDiscriminado` tem o braço `curto` (`Start-Sleep 60` com 2 s; `timedOut` e `started`) e o `normal` (`exit 1`; `!timedOut` e código 1), só com `pwsh`. Contra-braços do executor: `timedOut=false` reprova o `curto`, e `timedOut=(code!=0)` reprova o `normal`. A matriz não mudou. O aquecimento aparece no `credential/pwsh` rodado sozinho (P-8).
**Files affected:** `internal/generators/guard_line_shell_test.go`
**Actions:**
1. Separar a execução de `runGuardLinePS` numa função que recebe o prazo e devolve, além de código, stderr e stdin capturado, um `timedOut bool` (`ctx.Err() == context.DeadlineExceeded`). O `runGuardLinePS` chama essa função com 15 s e dá `t.Fatalf` com "timeout", o binário e o prazo quando `timedOut` for verdadeiro (AC2).
2. Aquecimento por binário, uma vez por processo (AC1):
   - executar `<psBin> -NoProfile -NonInteractive -Command "exit 0"` com prazo de 60 s, logo depois do `LookPath` no `t.Run(ps.name)`;
   - registrar a duração com `t.Logf`;
   - falhar com mensagem própria se o prazo estourar ou se o código de saída não for 0.
3. Teste novo do discriminante (AC3): prazo de poucos milissegundos sobre o PS disponível afirma `timedOut`; o prazo normal afirma `!timedOut`; pular com motivo se nenhum PS existir. Contra-braço: sem a checagem de `ctx.Err()`, o teste reprova.
4. Prescrições da Wave 0 (`docs/seguranca/2026-11-01-wave0-powershell-inicio-a-frio.md`):
   - **P-2:** `sync.OnceValues` por binário, e todo braço PS confere o resultado guardado, com `t.Fatalf` se o aquecimento falhou.
   - **P-3:** braço curto com `Start-Sleep 60` e prazo de cerca de 2 s; afirmar `ProcessState != nil`.
   - **P-4:** braço normal com `-Command "exit 1"`.
   - **P-5:** `cmd.WaitDelay` de 5 s e classificação em três vias: `nil` dá código 0; `*exec.ExitError` dá `ExitCode()`; qualquer outro erro, inclusive `ErrWaitDelay`, dá `t.Fatalf`.
   - **P-8:** o aquecimento também dispara num braço que não é o primeiro.
   - **P-9:** o teste do AC3 usa só `pwsh`.
**Acceptance criteria:**
- [x] `go vet ./internal/generators/` limpo
- [x] `go test ./internal/generators/ -run 'TestGuardLineShellMatrix|TestRunGuardLinePS' -count=1 -v` verde no macOS, com a linha do aquecimento do `pwsh` no log
- [x] contra-braço do AC3 medido, restaurado e conferido com `cmp`
- [x] nenhum arquivo fora de `internal/generators/guard_line_shell_test.go` alterado

**Gates da wave:**
```bash
go build ./...
go vet ./internal/generators/
go test ./internal/generators/ -count=1
go vet ./internal/validator/
go test ./internal/validator/ -count=1
```

### ML-1B — Sonda de política do PowerShell sem fail-open no início a frio (apolo-tf) — AC6
**Status:** ✅ Concluído — auditado em 2026-11-01. Entregue: `guardPSPolicyTimeout` de 20 s e aviso de política ilegível. A auditoria achou um retorno cedo: o aviso incerto pulava a sonda do Git Bash (ML-5D), que acusa um fail-open certo. Foi corrigido num segundo despacho: `guardCheckPS1RestrictedPolicy` passou a devolver `([]string, definitive bool)`. Com Restricted, o retorno continua cedo; com política ilegível, o aviso sai junto com as mensagens da sonda do Git Bash. Testes novos: `..._PolicyUnreadable_Warning`, `..._Restricted_GitBashNaoRoda` e `..._PolicyUnreadableWithGitBashViolation_BothMessagesAppear`. Os contra-braços do executor reprovam sem cada correção. Os testes foram rodados por nome em `-v` pelo arquiteto. Residual: o prazo de 20 s só é exercitado num runner Windows.
**Files affected:** `internal/validator/validator_guard_binary_probe.go`, `internal/validator/validator_guard_binary_probe_test.go`, `docs/cli-parity.md` (se a mensagem estiver documentada lá)
**Actions:**
1. Criar a var de pacote `guardPSPolicyTimeout = 20 * time.Second` e usá-la no `guardGetPSPolicy` no lugar do `guardProbeTimeout`.
2. No `guardCheckPS1RestrictedPolicy`, com o `trackfw` resolvendo para `.ps1` e `policy == ""`, emitir um aviso: a política não pôde ser lida; se for Restricted, o hook falha aberto; conferir com `Get-ExecutionPolicy`.
3. Testes: um novo para a política ilegível, que afirma o aviso; os de Restricted e RemoteSigned continuam passando. Contra-braço: sem o ramo do `""`, o teste novo reprova.
**Acceptance criteria:**
- [x] `go vet ./internal/validator/` limpo
- [x] `go test ./internal/validator/ -count=1` verde
- [x] contra-braço medido, restaurado e conferido com `cmp`
- [x] nenhum arquivo fora dos três listados alterado

### ML-1C — Aviso de política ilegível roteado como warning; sonda uma vez por validação (apolo-tf) — AC6 (achado MEDIUM-1 do red team)
**Status:** ✅ Concluído — auditado em 2026-11-01. `guardBinaryProbeOnce` devolve `(violations, warnings)`. `guardBinaryProbeCached` guarda um resultado por valor de `hasPSPosixEntry` em `currentGuardProbeRun`, zerado em `ValidateUnfiltered` e em `validateUnfilteredTagged`. Os warnings saem por `applyRuleWarnOnly`/`applyRuleWarnOnlyTagged` via `guardBinaryProbeFlushWarnings`. A auditoria achou o `ValidateUnfiltered`, caminho de produção do `trackfw baseline`, sem teste, e o artemis-tf acrescentou `TestML1C_ValidateUnfiltered_...`. Contra-braços medidos: rotear por `applyRule` e remover o flush reprovam os dois caminhos; desligar o cache dá `callCount = 2`. Residual aceito: fora de uma validação (`currentGuardProbeRun == nil`), o aviso incerto é descartado; hoje todos os chamadores estão dentro de `Validate`, `ValidateTagged` ou `ValidateUnfiltered`.
**Origem:** MEDIUM-1 e LOW do parecer `docs/seguranca/2026-11-01-redteam-powershell-inicio-a-frio.md`, conferidos pelo arquiteto em `internal/validator/validator.go:559` (`applyRule`) e `:1059`. O aviso volta pelo mesmo `[]string` das violações, entra em `applyRule("credential_guard_hook_resolvable")`, cuja severidade padrão é `error`, e bloqueia o `validate`. O AC6 pede aviso. A sonda tem dois chamadores por validação (`validator_credential_guard.go:730` e `validator_git_branch_guard.go:418`), o que pode somar 2 × 20 s e duplicar a mensagem.
**Files affected:** `internal/validator/validator_guard_binary_probe.go`, `internal/validator/validator_credential_guard.go`, `internal/validator/validator_git_branch_guard.go`, `internal/validator/validator.go`, `internal/validator/validator_guard_binary_probe_test.go`, `internal/validator/validator_guard_binary_probe_gitbash_test.go` (troca de assinatura de `guardBinaryProbeOnce`), `docs/cli-parity.md`
**Actions:**
1. O aviso não definitivo sai por `applyRuleWarnOnly` (e pela variante `Tagged`) nos dois pontos de entrada, `Validate` e `ValidateTagged`. Nunca entra em `violations`.
2. A sonda roda no máximo uma vez por validação para cada valor de `hasPSPosixEntry`, com o cache zerado no início de cada validação. Isso evita somar 20 s por chamador e duplicar a mensagem.
3. Teste de ponta a ponta por `Validate` e `ValidateTagged`: com `.ps1` e `policy == ""`, "could not be read" fica em `warnings` e fora de `violations`; com Restricted, a violação continua.
**Acceptance criteria:**
- [x] `go vet ./internal/validator/` limpo
- [x] `go test ./internal/validator/ -count=1` verde
- [x] contra-braço: rotear o aviso por `applyRule` faz o teste de ponta a ponta reprovar
- [x] contra-braço do cache: sem o cache, um contador de chamadas a `guardGetPSPolicy` passa de 1 por validação

## Wave 2 — Red team e evidência no CI (hades-tf)
> Dependencies: Wave 1 auditada e o PR com o `windows-full-suites` verde.

### ML-2A — Red team do ML-1A e do log do CI de Windows (hades-tf) — AC5
**Status:** ✅ Concluído — auditado em 2026-11-01. As prescrições P-1 a P-9 foram atendidas; o P-7 não foi acionado. AC5 medido no job 124955795693: aquecimento do `powershell.exe` em 1,69 s e `git-branch/powershell/RC0-allow` em 1,37 s, abaixo de 3 s. É uma amostra só, e o residual está declarado no parecer. O MEDIUM-1 (aviso incerto virando violação) e o LOW (sonda repetida) foram corrigidos no ML-1C. INFO-2 e INFO-3 ficam como residual aceito.
**Files affected:** `docs/seguranca/2026-11-01-redteam-powershell-inicio-a-frio.md`
**Actions:**
1. Reimplementar, a partir da leitura, os alvos de falsificação do ML-0A contra os diffs do ML-1A e do ML-1B.
2. Ler o log do `windows-full-suites` do PR. Registrar a duração do aquecimento do `powershell` e a do `git-branch/powershell/RC0-allow` (AC5).
**Acceptance criteria:**
- [x] Parecer com achados classificados e evidência
- [x] AC5 com os números do log citados

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-01-redteam-powershell-inicio-a-frio.md
```
