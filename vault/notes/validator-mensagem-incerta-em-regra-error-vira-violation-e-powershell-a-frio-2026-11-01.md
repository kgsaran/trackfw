---
domain: validator
problem: mensagem-incerta-em-regra-error-vira-violation
date: 2026-11-01
roadmap: ROADMAP-2026-11-01-teste-da-matriz-de-guard-sob-powershell-5-1-estoura-o-prazo-no-inicio-a-frio-do-powershell-exe.md
severity: MEDIUM
---

# Mensagem de aviso devolvida em `[]string` de regra `error` vira violação, e o PowerShell 5.1 a frio passa de 15 s (#649)

## 1. O `[]string` apaga a severidade

As funções de checagem do `internal/validator` devolvem `[]string`. Quem decide a severidade é o
`applyRule(nome, msgs, ...)`, por regra e não por mensagem: tudo o que entra numa regra sem entrada
em `ruleDefaults` vira `error`. Esse é o caso de `credential_guard_hook_resolvable` e de
`git_branch_guard_hook_resolvable`.

**Medido no #649 (ML-1B, depois ML-1C):** um aviso **incerto** ("a política do PowerShell não pôde
ser lida") foi acrescentado ao mesmo `[]string` das violações **certas** e passou a bloquear o
`trackfw validate`. Os testes unitários de `guardBinaryProbeOnce` passavam, porque afirmavam a
presença da mensagem, não o balde em que ela caía. Só um teste de ponta a ponta por `Validate`,
`ValidateTagged` e `ValidateUnfiltered` enxerga o balde.

**Regra:** uma função que produz mensagens de natureza mista devolve dois canais,
`(violations, warnings)`. O canal incerto sai por `applyRuleWarnOnly`/`applyRuleWarnOnlyTagged`. E
são **três** pontos de entrada, todos de produção: `ValidateUnfiltered`, usado pelo
`trackfw baseline`, e `validateUnfilteredTagged`, usado por `Validate` e `ValidateTagged`. Teste
cada um.

**Pegadinha relacionada:** a sonda tinha dois chamadores por validação (projeto e global). Sem cache
por execução, um `powershell.exe` travado custava 2 × 20 s e duplicava a mensagem.

## 2. PowerShell 5.1 a frio

A primeira chamada ao `powershell.exe` 5.1 no processo de teste do CI de Windows levou de 2,91 a
9,86 s em 17 jobs verdes e passou de 15 s em dois. As seguintes levaram de 0,4 a 2,2 s. O `pwsh` 7
frio levou cerca de 0,9 s.

- No Windows, o kill do `exec.CommandContext` por prazo sai com **código 1**; no macOS, com -1.
  Uma falha por prazo se disfarça de asserção ("esperado 0, obteve 1"). O discriminante é
  `ctx.Err() == context.DeadlineExceeded`.
- Aumentar o prazo só move o limiar. A correção da matriz de guard foi aquecer cada binário uma vez
  (`sync.OnceValues`, 60 s, `t.Logf`). Depois disso, o `RC0-allow` mediu 1,37 s.
- No **produto**, a mesma causa fazia o `guardGetPSPolicy` (prazo de 5 s) devolver `""`, lido como
  "não é Restricted": fail-open silencioso. Hoje o prazo é de 20 s, e a política ilegível gera
  aviso.
- Sem `cmd.WaitDelay`, um neto que segura o pipe de stderr prende o `cmd.Run()` muito depois do
  kill. Medido pelo hades-tf: 10 s a mais.
