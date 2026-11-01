---
status: Open
date: 2026-11-01
author: "Zeus (trackfw_architect)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md"
---

# REQ: Teste da matriz de guard sob PowerShell 5.1 estoura o prazo no início a frio do powershell.exe

> Date: 2026-11-01 | Status: Open
| Linear Issue: 
| Jira Issue: 
| GitHub Issue: #649

## Motivation

O check obrigatório `windows-full-suites` reprova de forma intermitente em
`TestGuardLineShellMatrix/git-branch/powershell/RC0-allow`
(`internal/generators/guard_line_shell_test.go`). Um PR sem código fica bloqueado até alguém
reexecutar o job.

### Medição do arquiteto (2026-11-01)

- **Ordem de execução:** `guardShellMatrix` começa por `git-branch`, e `psEntries` vai de `pwsh` para
  `powershell`. Por isso o subteste `git-branch/powershell/RC0-allow` é a **primeira** chamada ao
  `powershell.exe` dentro do processo de teste do pacote.
- **Duração do subteste em 18 jobs distintos do `windows-full-suites`** (workflow `quality.yml`,
  2026-10-29 a 2026-11-01, extraída com `gh run view --job <id> --log`):
  - nos 17 verdes, o `RC0-allow` levou de 2,91 a 9,86 s;
  - no job 124173776270 (run 41087894283), levou 15,12 s e falhou;
  - nos 18 jobs, as chamadas seguintes ao mesmo binário levaram de 0,4 a 2,2 s;
  - o `pwsh` frio, no `git-branch/pwsh/RC0-allow`, levou 0,90 s.
- **A segunda falha não entra na amostra:** é o job 124854992839 (run 41308911347, tentativa 1), com
  15,04 s registrados no issue. O `gh run view --job` devolveu o log da reexecução, então só o issue
  registra esse número.
- **A causa:** o custo está no início a frio do `powershell.exe` 5.1, não no guard. A cauda já chega
  a 9,9 s e passou de 15 s duas vezes.
- **Por que o erro parece outro:** no Windows, o kill do `exec.CommandContext` por prazo sai com
  código 1. A falha aparece como asserção (`esperado exit 0, obteve 1`, stdin vazio), não como
  timeout.
- **Sítio único:** nenhum outro teste chama `pwsh` ou `powershell` com prazo fixo. Os outros
  `CommandContext` com prazo nos testes (`grep -rn --include='*_test.go' 'CommandContext('`) chamam o
  próprio binário de teste, o `trackfw` ou shells POSIX, que são pulados no Windows.
- **Varredura de issues abertos por mecanismo** (timeout, intermitente, windows, frio, flaky,
  powershell): só o #649.

### Por que não só aumentar o prazo

Aumentar o prazo apenas move o limiar de uma cauda que já foi de 9,9 a 15,1 s. Aquecer o binário tira
o custo a frio da janela que o teste afirma.

## Acceptance Criteria

- [x] **AC1 — Aquecimento por binário:** antes do primeiro caso afirmado de cada braço PS (`pwsh` e
  `powershell`), o teste executa uma vez por processo `<psBin> -NoProfile -NonInteractive -Command
  "exit 0"`.
  - O aquecimento tem prazo próprio de 60 s e registra a duração com `t.Logf`.
  - Se o prazo estourar ou o código de saída não for 0, ele falha com mensagem própria, distinta da
    falha de asserção.
  - O prazo de 15 s das chamadas afirmadas não muda.
- [x] **AC2 — Timeout distinguível:** quando o prazo de uma chamada afirmada estoura, o teste falha com
  mensagem que diz "timeout", o binário e o prazo. Ele nunca deixa a falha aparecer como `esperado
  exit N, obteve 1`.
  - O discriminante é `ctx.Err() == context.DeadlineExceeded`, não o código de saída: o kill sai com
    1 no Windows e com -1 no macOS.
- [x] **AC3 — Teste do discriminante com contra-braço:**
  - um teste novo chama a execução com prazo de poucos milissegundos sobre um PS disponível
    (`pwsh` no macOS) e afirma que o timeout foi detectado;
  - com o prazo normal, ele afirma que não foi;
  - sem a checagem de `ctx.Err()`, o teste reprova (contra-braço medido e restaurado).
- [x] **AC4 — Matriz inalterada:** os 16 casos PS da matriz (4 formas × 4 cenários) e as asserções
  deles continuam os mesmos. `go test ./internal/generators/ -run 'TestGuardLineShellMatrix' -count=1
  -v` passa no macOS.
- [x] **AC5 — Evidência no CI de Windows:** no `windows-full-suites` do PR, o log mostra a linha do
  `t.Logf` do aquecimento do `powershell`. O `git-branch/powershell/RC0-allow` fica abaixo de 3 s,
  na faixa das chamadas aquecidas.
- [x] **AC6 — Sonda de política do PowerShell sem fail-open no início a frio** (achado P-1 da Wave 0,
  mesma causa):
  - em `internal/validator/validator_guard_binary_probe.go`, o `guardGetPSPolicy` chama o `powershell`
    com o prazo de 5 s do `guardProbeTimeout` e devolve `""` quando o prazo estoura;
  - o `guardCheckPS1RestrictedPolicy` lê esse `""` como política diferente de Restricted e fica em
    silêncio, mesmo com o `trackfw` resolvendo para `.ps1`;
  - a sonda de política passa a ter prazo próprio, de 20 s, porque o início a frio medido chega a
    15 s;
  - com o `trackfw` resolvendo para `.ps1`, uma política ilegível (`""`) emite um aviso de que a
    política não pôde ser lida e de que o hook falha aberto se ela for Restricted, com o comando
    `Get-ExecutionPolicy` para conferir;
  - os testes cobrem os três casos: Restricted, legível e diferente de Restricted, e ilegível.

## Negative scope

- Não muda nenhum guard nem nenhuma linha de guard gerada. O único código de produto alterado é a
  sonda de política do AC6, em `internal/validator/validator_guard_binary_probe.go`.
  - O escopo original excluía qualquer código de produto. A Wave 0 achou a mesma causa nessa sonda, e
    a Regra Dura de Causa Raiz manda trazê-la para esta REQ.
- Não muda o workflow `quality.yml` nem o ratchet de falhas conhecidas. Pôr o teste na lista de
  conhecidos esconderia o sintoma.
- Não muda os prazos de 10 s dos braços POSIX (`runGuardLine`, `_DefeitoAntigo`,
  `_UnsetoptInsuficiente`), que não rodam no Windows.
- Não investiga a lentidão do PowerShell 5.1 no runner nem tenta reduzi-la.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md
