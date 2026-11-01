---
status: wip
date: 2026-11-01
req: "docs/req/REQ-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md"
squad: "hades-tf, artemis-tf"
---

# Roadmap: Teste da matriz de guard sob PowerShell 5.1 estoura o prazo no início a frio do powershell.exe

> Created: 2026-11-01 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md
Issue: #649

A medição do arquiteto está detalhada na REQ. Em 18 jobs distintos, a primeira chamada ao
`powershell.exe` levou de 2,91 a 9,86 s nos verdes e 15,12 s na falha; as chamadas seguintes, de 0,4
a 2,2 s. A causa é o início a frio, e a correção é aquecer o binário e distinguir a falha por timeout.

Sítio único: `internal/generators/guard_line_shell_test.go`. A varredura de issues abertos achou só o
#649.

## Acceptance Criteria
- [ ] AC1–AC5 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ⬜ Pendente
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
- [ ] As quatro seções respondidas com evidência (comando e resultado)
- [ ] Prescrições em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-01-wave0-powershell-inicio-a-frio.md
grep -q '^## Prescrições' docs/seguranca/2026-11-01-wave0-powershell-inicio-a-frio.md
```

## Wave 1 — Implementação (1 executor)
> Dependencies: Wave 0 auditada.

### ML-1A — Aquecimento e timeout distinguível na matriz PS (artemis-tf) — AC1 a AC4
**Status:** ⬜ Pendente
**Files affected:** `internal/generators/guard_line_shell_test.go`
**Actions:**
1. Separar a execução de `runGuardLinePS` numa função que recebe o prazo e devolve, além de código, stderr e stdin capturado, um `timedOut bool` (`ctx.Err() == context.DeadlineExceeded`). O `runGuardLinePS` chama essa função com 15 s e dá `t.Fatalf` com "timeout", o binário e o prazo quando `timedOut` for verdadeiro (AC2).
2. Aquecimento por binário, uma vez por processo (AC1):
   - executar `<psBin> -NoProfile -NonInteractive -Command "exit 0"` com prazo de 60 s, logo depois do `LookPath` no `t.Run(ps.name)`;
   - registrar a duração com `t.Logf`;
   - falhar com mensagem própria se o prazo estourar ou se o código de saída não for 0.
3. Teste novo do discriminante (AC3): prazo de poucos milissegundos sobre o PS disponível afirma `timedOut`; o prazo normal afirma `!timedOut`; pular com motivo se nenhum PS existir. Contra-braço: sem a checagem de `ctx.Err()`, o teste reprova.
**Acceptance criteria:**
- [ ] `go vet ./internal/generators/` limpo
- [ ] `go test ./internal/generators/ -run 'TestGuardLineShellMatrix|TestRunGuardLinePS' -count=1 -v` verde no macOS, com a linha do aquecimento do `pwsh` no log
- [ ] contra-braço do AC3 medido, restaurado e conferido com `cmp`
- [ ] nenhum arquivo fora de `internal/generators/guard_line_shell_test.go` alterado

**Gates da wave:**
```bash
go build ./...
go vet ./internal/generators/
go test ./internal/generators/ -count=1
```

## Wave 2 — Red team e evidência no CI (hades-tf)
> Dependencies: Wave 1 auditada e o PR com o `windows-full-suites` verde.

### ML-2A — Red team do ML-1A e do log do CI de Windows (hades-tf) — AC5
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-01-redteam-powershell-inicio-a-frio.md`
**Actions:**
1. Reimplementar, a partir da leitura, os alvos de falsificação do ML-0A contra o diff do ML-1A.
2. Ler o log do `windows-full-suites` do PR. Registrar a duração do aquecimento do `powershell` e a do `git-branch/powershell/RC0-allow` (AC5).
**Acceptance criteria:**
- [ ] Parecer com achados classificados e evidência
- [ ] AC5 com os números do log citados

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-01-redteam-powershell-inicio-a-frio.md
```
