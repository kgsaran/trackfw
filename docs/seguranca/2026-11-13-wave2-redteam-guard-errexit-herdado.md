# Wave 2 — Red Team: Guard Hook Forma H

> Autor: hades-tf | Data: 2026-11-15
> REQ: REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
> Roadmap: ROADMAP-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
> ML-2A — Red team da forma nova (forma H)

---

## Veredicto

**APROVA COM RESSALVAS.** Um achado IN SCOPE encontrado (V3.6a — EAP ReadOnly-Stop). Recomenda-se
ML corretivo nesta REQ antes de merge. Dois achados GRAY marcados para decisão do Zeus. Nenhum
achado no validador ou no migrador.

---

## Metodologia

**Fonte da forma H.** Extraída de três sítios canônicos e confirmada idêntica por comparação de
string:

1. `internal/guardforms/guard_generations.go` constante `GitBranchCurrentPSPOSIX`
2. `internal/validator/validator_guard_binary_probe.go` função `guardExpectedLine("git-branch", ...)`
3. `.claude/settings.json` no repositório

Forma H (git-branch):
```
set +eu; set ErrorActionPreference Continue; null=/dev/null; Set-StrictMode -Off 2>$null; emulate -R sh 2>$null; $LASTEXITCODE=2 2>$null; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>$null; exit $LASTEXITCODE
```

**Harness.** Diretório temporário em `mktemp -d` no scratchpad de sessão. Fake `trackfw` grava
`printf 'ran: %s\n' "$*"` num marker file (via `TRACKFW_MARKER_FILE`) e sai com `$FAKE_RC`. PATH de
teste: `$HDIR:/usr/bin:/bin:/usr/sbin:/sbin` (sem `/opt/homebrew/bin`, sem o `trackfw` instalado).
Confirmado: `command -v trackfw` no PATH de teste aponta apenas para o fake.

**Shells.**
- `/bin/sh` = bash 3.2.57 (macOS)
- `/bin/bash` = bash 3.2.57; `/opt/homebrew/bin/bash` = bash 5.3.20
- `/bin/zsh` = zsh 5.9
- `/bin/dash` = dash 0.5.12
- `/opt/homebrew/bin/pwsh` = PowerShell 7.6.9 (macOS, sem Windows VM neste ML)

**Baseline confirmada** (forma H sem estado herdado):

| shell | deny rc | guard | allow rc | guard |
|---|---|---|---|---|
| bash | 2 | sim | 0 | sim |
| bash -e | 2 | sim | 0 | sim |
| bash -u | 2 | sim | 0 | sim |
| sh (bash 3.2) | 2 | sim | 0 | sim |
| sh -e | 2 | sim | 0 | sim |
| dash | 2 | sim | 0 | sim |
| dash -e | 2 | sim | 0 | sim |
| zsh | 2 | sim | 0 | sim |
| zsh -e | 2 | sim | 0 | sim |
| zsh -u | 2 | sim | 0 | sim |

---

## Vetores Medidos

### V1 — Segmento `null=/dev/null`

#### V1a — `readonly null` via BASH_ENV (bash)

**Comando exato:**
```bash
env PATH="$TESTPATH" BASH_ENV="<(printf 'readonly null=\"/tmp/clobber\"\n')" FAKE_RC=2 ... \
    /opt/homebrew/bin/bash -c "$FORM_H"
```

**rc medido:** deny rc=1 | allow rc=1  
**guard ran:** NÃO (guard=no para ambos)  
**Stderr:** `bash: line 1: null: readonly variable`

**Mecanismo.** BASH_ENV define `null` como readonly antes da execução do script. O segmento
`null=/dev/null` falha por tentar atribuir a uma variável readonly. Com `set +eu` já ativo no
ponto, bash imprime o erro e continua, mas `null` permanece com o valor do BASH_ENV. Os redirects
seguintes (`2>$null`) vão para o arquivo apontado pelo BASH_ENV, não para `/dev/null`. Subsequente:
`Set-StrictMode -Off 2>$null` e `emulate -R sh 2>$null` imprimem para o arquivo errado. Com `emulate
-R sh` em zsh, o guard rodaria; em bash/sh/dash, `emulate` é command-not-found e continua. O guard
não chega a rodar porque a linha aborta com rc=1 após o erro da variável readonly.

Espera: `exit $LASTEXITCODE` = `exit 1` (rc do último comando com falha).

**Classificação: GRAY.** Requer código no BASH_ENV (atribuição de readonly), mesma classe do
R1 declarado na Wave 0 ("exige controle sobre BASH_ENV"). O mecanismo não é "estado de preferência"
puro, mas código executado antes do hook. Zeus decide se promove a residual aceito ou se abre ML
corretivo. Impacto se em escopo: deny → rc=1 → 5 CLIs (exit-2-only) falham abertos.

---

#### V1b — `null` exportado apontando para arquivo (sem readonly)

**Comando exato:**
```bash
env PATH="$TESTPATH" null="/tmp/null_target" FAKE_RC=2 ... \
    /opt/homebrew/bin/bash -c "$FORM_H"
```

**rc medido:** deny rc=2  
**guard ran:** SIM

**Classificação: SEM EFEITO.** O segmento `null=/dev/null` sobrescreve o valor herdado do ambiente.
A atribuição em POSIX shell NÃO é readonly por padrão — um valor exportado sem readonly é apenas uma
variável de ambiente que o shell sobrescreve na execução. Não há bypass.

---

#### V1c — `typeset -r null` em zsh via ZDOTDIR/.zshenv

**Comando exato:**
```bash
printf 'typeset -r null="/tmp/clobber"\n' > "$ZDOTDIR_TMP/.zshenv"
env PATH="$TESTPATH" ZDOTDIR="$ZDOTDIR_TMP" FAKE_RC=2 ... /bin/zsh -c "$FORM_H"
```

**rc medido:** deny rc=1  
**guard ran:** NÃO  
**Stderr:** `zsh:1: read-only variable: null`

**Classificação: RESIDUAL (classe R3).** Requer código em `.zshenv` (definição de variável readonly),
mesma classe do R3 declarado ("requer sobrescrever builtin `emulate` em dotfiles"). O .zshenv
é código do usuário executado antes do hook; não é estado de preferência. Improvável em produção.
Impacto se em escopo: deny → rc=1 → 5 CLIs falham abertos.

---

#### V1d — `readonly LASTEXITCODE=0` via BASH_ENV (bash)

**Comando exato:**
```bash
printf 'readonly LASTEXITCODE=0\n' > "$BENV"
env PATH="$TESTPATH" BASH_ENV="$BENV" FAKE_RC=2 ... /opt/homebrew/bin/bash -c "$FORM_H"
```

**rc medido:** deny rc=1 | allow rc=1  
**guard ran:** SIM (nos dois casos)

**Mecanismo (confirmado por xtrace).** BASH_ENV define `LASTEXITCODE=0` como readonly. No form H:
- `set +eu` desativa errexit/nounset
- `$LASTEXITCODE=2 2>$null` → expande para `0=2 2>/dev/null` (command-not-found, bash continua)
- `trackfw guard git-branch` RODA (FAKE_RC=2)
- `LASTEXITCODE=$((2*!!$?))` = `LASTEXITCODE=2` → FALHA (readonly); bash imprime erro e sai com rc=1
  antes de processar os segmentos seguintes

`exit $LASTEXITCODE` não chega a ser executado na invocação `-c`. O rc=1 é do erro de readonly.

Para deny (FAKE_RC=2): guard roda mas rc=1 (não 2) → 5 CLIs (exit-2-only) falham abertos.  
Para allow (FAKE_RC=0): guard roda, rc=1 → Copilot (any-non-zero) bloqueia indevidamente (falso positivo).

**Classificação: GRAY.** Requer código em BASH_ENV, mesma classe R1. Diferença de V1a: o guard
roda, mas a normalização falha. Zeus decide entre residual aceito ou ML corretivo.

---

### V2 — `set ErrorActionPreference Continue` em POSIX com opções herdadas

Testadas: `SHELLOPTS=posix` (bash), `NO_UNSET`, `ERR_EXIT`, `POSIX_BUILTINS`, `emulate sh` em
`.zshenv` (zsh via ZDOTDIR).

**Resultados em todos os casos:** deny rc=2, guard=sim; allow rc=0, guard=sim.

**Mecanismo.** `set ErrorActionPreference Continue` em POSIX define positional params `$1=ErrorActionPreference`
e `$2=Continue`, com rc=0. Não há opção herdada que faça esse `set` falhar; o shell no ponto já
tem `set +eu` ativo. As opções de zsh são resetadas pelo `emulate -R sh` que vem depois.

**Classificação: SEM EFEITO.** Nenhum bypass encontrado neste vetor.

---

### V3 — PowerShell: estado herdado via perfil

Todos os testes usaram HOME temporário com `XDG_CONFIG_HOME` apontando para o perfil criado,
`PSExecutionPolicyPreference=` (vazio para remover bypass de execução), sem `-NoProfile` (para que
o perfil carregue).

#### V3.1 — Redefinição de `set` como função/alias no perfil

**Testados:**
- `function set { param([string]$Name,[string]$Value) }` — função vazia
- `Set-Alias set Write-Error`

**Resultado:** deny rc=2, guard=sim; allow rc=0, guard=sim. SEM EFEITO.

---

#### V3.2 — Redefinição de `Set-StrictMode` como função/alias

**Resultado:** deny rc=2, guard=sim. SEM EFEITO.

---

#### V3.3 — Redefinição de `null` como função ou variável

**Testados:**
- `function null { exit 2 }` — função que exits 2
- `Set-Variable -Name null -Value "clobber_path" -Scope Global`

**Resultado:** deny rc=2, guard=sim; allow rc=0, guard=sim. SEM EFEITO.

**Mecanismo.** `null=/dev/null` em POSIX é uma atribuição de variável shell, não uma chamada a
`null`. Em PS, `null=/dev/null` é um command-not-found (nome contém `=`) suprimido a seguir por
`Set-StrictMode -Off 2>$null` que já está ativo. A função `null` em PS não interfere.

---

#### V3.4 — `$PSDefaultParameterValues` para `Set-Variable`

**Testados:**
- `$PSDefaultParameterValues = @{"Set-Variable:WhatIf" = $true}`
- `$PSDefaultParameterValues = @{"Set-Variable:Option" = "Constant"}`

**Resultado:** deny rc=2, guard=sim; allow rc=0, guard=sim. SEM EFEITO.

**Nota para WhatIf.** Com `WhatIf=$true`, `set ErrorActionPreference Continue` fica em modo
WhatIf (imprime "What if: ...") mas NÃO altera EAP. Se o perfil não tiver definido EAP=Stop
explicitamente, EAP permanece Continue (default), e a forma H funciona normalmente. Medido: rc=2
guard=sim. Combinação com Stop explícito — ver V3.7.

---

#### V3.5 — ConstrainedLanguage via `__PSLockdownPolicy=4`

**Resultado:** deny rc=2, guard=sim. SEM EFEITO.

**Nota.** `__PSLockdownPolicy=4` é ignorado no macOS (sem enforcing do sistema); o PS 7 no macOS
roda em FullLanguage independente. Não medido na VM Windows.

---

#### V3.6a — EAP ReadOnly=Stop via perfil ⚠ ACHADO IN SCOPE

**Perfil:** `Set-Variable -Name ErrorActionPreference -Value Stop -Scope Global -Option ReadOnly`

**Comando exato:**
```
HOME=$HOME_TMP XDG_CONFIG_HOME=$HOME_TMP/.config PSExecutionPolicyPreference= \
    /opt/homebrew/bin/pwsh -NonInteractive -Command "$FORM_H"
```
(com perfil acima em `$HOME_TMP/.config/powershell/Microsoft.PowerShell_profile.ps1`)

**rc medido:** deny rc=1 | allow rc=1  
**guard ran:** NÃO  
**Stderr (deny):** `Set-Variable: Cannot overwrite variable ErrorActionPreference because it is read-only or constant.`

**Mecanismo.** O perfil define EAP=Stop com `-Option ReadOnly`. Na forma H, `set ErrorActionPreference
Continue` = `Set-Variable -Name ErrorActionPreference -Value Continue` falha porque a variável é
readonly. EAP permanece Stop. O comando seguinte (`null=/dev/null` → command-not-found em PS) lança
um erro terminante (EAP=Stop), encerrando o script antes do guard. O guard nunca roda.

**Impacto.** deny → rc=1 → 5 CLIs (exit-2-only) falham abertos. allow → rc=1 → Copilot bloqueia
indevidamente.

**Mitigação confirmada:** `Set-Variable -Name ErrorActionPreference -Value Continue -Scope Global -Force`
(com `-Force`) sobrescreve o ReadOnly. Medido: deny rc=2, guard=sim. A forma H atual usa `set
ErrorActionPreference Continue` (alias de `Set-Variable` sem `-Force`), insuficiente para ReadOnly.

**Classificação: IN SCOPE.** O mecanismo é "opção herdada encerra a linha antes do guard" — o mesmo
que R4/ML-1C. O ML-1C corrigiu EAP=Stop simples; este vetor mostra que EAP=Stop+ReadOnly
contorna a correção. Um perfil com ReadOnly é uma medida de hardening plausível (não é código de
troca de comportamento — é uma mudança de atributo de variável de preferência). Recomenda-se ML
corretivo nesta REQ, com medição de que a correção não introduz regressão nos shells POSIX.

---

#### V3.6b — EAP Constant=Stop via perfil

**Perfil:** `Set-Variable -Name ErrorActionPreference -Value Stop -Scope Global -Option Constant`

**rc medido:** deny rc=2, guard=sim. SEM EFEITO.

**Mecanismo.** PS não permite tornar Constant uma variável existente (EAP existe desde o início da
sessão): `Existing variable ErrorActionPreference cannot be made constant. Variables can be made
constant only at creation time.` O perfil falha silenciosamente, EAP permanece Continue (default).

---

#### V3.7 — `$WhatIfPreference=$true` + EAP=Stop no perfil

**Perfil:** `$WhatIfPreference = $true; $ErrorActionPreference = "Stop"`

**rc medido:** deny rc=1 | allow rc=1  
**guard ran:** NÃO

**Mecanismo.** `$WhatIfPreference=$true` faz `Set-Variable` rodar em modo What-If (sem efeito real).
O `set ErrorActionPreference Continue` não muda EAP. EAP=Stop (do perfil) faz o script abortar
antes do guard.

**Classificação: RESIDUAL.** `$WhatIfPreference=$true` num perfil de produção é altamente improvável;
usada em scripts de teste, não como configuração de ambiente permanente. Mesma classe do R2 (exige
configuração de comportamento incomum). Se Zeus decidir que WhatIfPreference é "estado de
preferência" in scope, o fix de V3.6a (usar `-Force`) NÃO resolve este vetor (WhatIf suprime
a chamada completamente). Necessita de abordagem diferente.

---

#### V3.8 — `$ConfirmPreference='Low'` + EAP=Stop

**rc medido:** deny rc=1 | allow rc=1 | guard=NÃO

**Classificação: RESIDUAL.** Mesma raiz de V3.7. ConfirmPreference=Low num perfil de produção é
improvável sem `-NonInteractive` explícito. Com `-NonInteractive` (que a maioria dos CLIs usa), o
PS suprime confirmações; sem ele, poderia causar um hang (timeout → permite). Não testado sem
`-NonInteractive` (risco de hang na sessão de medição).

---

### V4 — zsh ZDOTDIR/.zshenv combinado com `emulate -R sh`

Positive control confirmada: `.zshenv` é carregado para `zsh -c` via ZDOTDIR.

#### V4.1 a V4.4 — opções herdadas via .zshenv

**Testados:** `setopt ERR_EXIT`, `setopt NOUNSET`, `setopt ERR_EXIT NOUNSET`, `emulate sh`.

**Resultado:** deny rc=2, guard=sim. SEM EFEITO.

**Mecanismo.** `emulate -R sh 2>$null` na forma H reseta todas as opções zsh para os defaults de
sh antes do guard. Mesmo que `.zshenv` ative ERR_EXIT/NOUNSET, `emulate -R sh` as desfaz. Os
segmentos anteriores (`set +eu`, `null=/dev/null`) não falham sob essas opções: `set +eu` é
builtin especial (não falha), `null=/dev/null` é atribuição (não aciona ERR_EXIT/NOUNSET).

---

#### V4.5 — TRAPZERR em .zshenv

**Perfil:** `printf 'TRAPZERR() { exit 0 }\n' > "$ZDOTDIR_TMP/.zshenv"`

**rc medido:** deny rc=0 | allow rc=0  
**guard ran:** NÃO

**Mecanismo.** `TRAPZERR` é invocada em zsh sempre que um comando retorna não-zero. Na forma H, o
segmento `Set-StrictMode -Off 2>$null` = command-not-found (não-zero) **antes** de `emulate -R sh`
desabilitar as traps zsh. TRAPZERR dispara com `exit 0`. O script sai 0 sem rodar o guard.

Impacto: deny → rc=0 → **todos os 6 CLIs** falham abertos (rc=0 não é bloqueado por nenhum CLI).
Allow → rc=0 → passa (correto).

**Classificação: RESIDUAL (classe R3).** Requer código em `.zshenv` (definição de TRAPZERR),
mesma classe do R3 declarado. TRAPZERR com `exit 0` num perfil de produção é um uso de engenharia
muito específico. Não é estado de preferência puro.

**Nota:** a ordem importa. `emulate -R sh` (segmento 5) vem DEPOIS de `Set-StrictMode -Off`
(segmento 4). Se `Set-StrictMode -Off` falha (command-not-found), TRAPZERR dispara antes de
`emulate -R sh` desabilitar as traps. Uma reordenação dos segmentos (colocar `emulate -R sh`
primeiro) encerraria este vetor, mas pode ter outras consequências nos demais shells.

---

### V5 — Migração

#### V5.1 — Forma H exata no validate

**Resultado:** `trackfw validate` não reporta nenhuma violation nem warning relacionada ao guard
git-branch. Comportamento correto. ✓

#### V5.2 — Forma H com espaço extra (mutação de um byte)

**Forma mutada:** `set +eu;  set ErrorActionPreference Continue; ...` (espaço duplo após `;`)

**Resultado:** `validate` reporta violation:
```
.claude/settings.json (Claude Code) has "set +eu;  set ErrorActionPreference Continue; ..."
but the expected form for this CLI is "set +eu; set ErrorActionPreference Continue; ..."
— run `trackfw update` to regenerate
```

Comparação por igualdade exata funcionando corretamente. Um byte de diferença detectado. ✓

#### V5.3 — Forma D18 legada no validate

**Resultado:** `validate` reporta **warning** (não violation):
```
.claude/settings.json (Claude Code) has "emulate -R sh 2>${null-/dev/null}; ..."
— D18 legacy form pre-REQ-2026-11-13, exits 127 under inherited errexit/nounset in POSIX shells;
run `trackfw update` to migrate to the form with the `set +eu` prefix
```

Gate correto: D18 legada dá aviso de migração, não violation. ✓

#### V5.4 — Laço explícito D18 vs dedup por registro (questão de código morto)

O laço explícito de migração D18 em `agentfiles.go` (linhas 376-383) chama `migrateHookCommand`
com as constantes `legacyD18{Credential,GitBranch,WriteScope}CmdPSPOSIX`. Essas mesmas constantes
estão nas listas do registro da D12 (`CredentialLegacyPSPOSIX`, `GitBranchLegacyPSPOSIX`,
`WriteScopeLegacyPSPOSIX`), consumidas por `migrateDedupGuardInNestedArray` (linhas 404-411).

O relatório do ML-1A2 confirma: "o contra-braço só reprova removendo os dois." Cada caminho não é
independentemente necessário pelo teste, mas ambos existem.

**Classificação: INFORMACIONAL.** O laço explícito é redundante com o dedup por registro para os
casos cobertos por ambos, mas os dois operam em estruturas de dados diferentes
(`migrateHookCommand` em arrays planos, `migrateDedupGuardInNestedArray` em arrays aninhados com
dedup). Não é código morto no sentido estrito; não é defeito de segurança. Recomenda-se revisão
de engenharia (hefesto-tf) para declarar qual caminho é necessário em cada estrutura de config.

---

### V6 — `env IFS=2` (vetor escolhido por hades-tf)

**Motivação.** A forma H usa `exit $LASTEXITCODE` não citado. Com `IFS=2` (o caractere `2` como
separador de campos), `$LASTEXITCODE` expandido para `2` poderia ser removido por field-splitting,
dando `exit` sem argumento e saindo com o $? do último comando.

**Resultado:** deny rc=2, guard=sim; allow rc=0, guard=sim. **Sem bypass em nenhum shell.**

```
IFS=2 bash       : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 bash -e    : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 bash -u    : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 sh         : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 sh -e      : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 dash       : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 dash -e    : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 zsh        : deny rc=2  guard=sim | allow rc=0  guard=sim
IFS=2 zsh -e     : deny rc=2  guard=sim | allow rc=0  guard=sim
```

**Mecanismo (confirmado por xtrace bash).** `exit $LASTEXITCODE` com IFS=2 e LASTEXITCODE=2:
- bash trata o argumento de `exit` como valor numérico, não como lista de palavras sujeita a IFS
- O field-splitting não remove o `2`; `exit 2` é executado corretamente

Efeito paralelo observado: `$LASTEXITCODE=2 2>$null` com LASTEXITCODE não-definido e IFS=2 vira
`=2 2>/dev/null` (command-not-found). O guard ainda roda. Sem impacto no resultado final.

**Classificação: SEM EFEITO.**

---

## Tabela Resumo

| Vetor | rc deny | guard | rc allow | guard | Classificação |
|---|---|---|---|---|---|
| V1a: BASH_ENV readonly null (bash) | 1 | NÃO | 1 | NÃO | GRAY |
| V1b: null exportado como arquivo | 2 | sim | 0 | sim | SEM EFEITO |
| V1c: zsh ZDOTDIR typeset -r null | 1 | NÃO | 1 | NÃO | RESIDUAL (R3) |
| V1d: BASH_ENV readonly LASTEXITCODE=0 | 1 | sim | 1 | sim | GRAY |
| V2: set EAP Continue em POSIX (todas variações) | 2 | sim | 0 | sim | SEM EFEITO |
| V3.1: profile redefine set | 2 | sim | 0 | sim | SEM EFEITO |
| V3.2: profile redefine Set-StrictMode | 2 | sim | 0 | sim | SEM EFEITO |
| V3.3: profile redefine null | 2 | sim | 0 | sim | SEM EFEITO |
| V3.4a: PSDefaultParameterValues WhatIf | 2 | sim | 0 | sim | SEM EFEITO |
| V3.4b: PSDefaultParameterValues Constant | 2 | sim | 0 | sim | SEM EFEITO |
| V3.5: __PSLockdownPolicy=4 | 2 | sim | — | — | SEM EFEITO |
| **V3.6a: EAP ReadOnly=Stop via perfil** | **1** | **NÃO** | **1** | **NÃO** | **IN SCOPE** |
| V3.6b: EAP Constant=Stop via perfil | 2 | sim | 0 | sim | SEM EFEITO |
| V3.7: WhatIfPreference+Stop | 1 | NÃO | 1 | NÃO | RESIDUAL |
| V3.8: ConfirmPreference+Stop | 1 | NÃO | 1 | NÃO | RESIDUAL |
| V4.1–V4.4: zsh ZDOTDIR opções diversas | 2 | sim | 0 | sim | SEM EFEITO |
| V4.5: zsh ZDOTDIR TRAPZERR(){exit 0} | 0 | NÃO | 0 | NÃO | RESIDUAL (R3) |
| V5.2: forma H + 1 byte → validate | violação detectada | — | — | — | CORRETO |
| V5.3: D18 legada → validate | aviso (não violação) | — | — | — | CORRETO |
| V5.4: laço D18 vs registro dedup | — | — | — | — | INFORMACIONAL |
| V6: IFS=2 | 2 | sim | 0 | sim | SEM EFEITO |

---

## Achados por Severidade

### A1 — EAP ReadOnly-Stop via perfil PowerShell (IN SCOPE, ALTA)

**Localização:** segmento `set ErrorActionPreference Continue` na forma H.

**Evidência:**
- Perfil: `Set-Variable -Name ErrorActionPreference -Value Stop -Scope Global -Option ReadOnly`
- Form H: `set ErrorActionPreference Continue` = alias `Set-Variable -Name EAP -Value Continue` sem `-Force`
- Resultado: `Set-Variable: Cannot overwrite variable ErrorActionPreference because it is read-only or constant.`
- rc=1, guard=no; 5 CLIs (exit-2-only) falham abertos para deny

**Distinção de R4/ML-1C.** O ML-1C (que corrigiu EAP=Stop simples) usou `set ErrorActionPreference
Continue`. Uma variável marcada com `-Option ReadOnly` não pode ser alterada sem `-Force`. O fix do
ML-1C não passou `-Force`, portanto não cobre este caso.

**Mitigação confirmada:** `Set-Variable -Name ErrorActionPreference -Value Continue -Scope Global -Force`
(rc=2, guard=sim medido). Não foi testado se `$ErrorActionPreference = 'Continue'` (direct PS
assignment) também bypassa ReadOnly — isso varia por versão; `-Force` é a forma documentada.

**Recomendação:** ML corretivo nesta REQ (ML-2A-C1), cobrindo: (a) substituição de `set ErrorActionPreference Continue` por forma que use -Force ou equivalente; (b) medição que a forma nova não introduz regressão POSIX; (c) verificação no PS 5.1 da VM (pois este ML não usou VM Windows).

---

### A2 — GRAY: BASH_ENV readonly null e readonly LASTEXITCODE (V1a, V1d)

Ambos os vetores requerem código em BASH_ENV. A decisão de classificação (residual aceito ou ML
corretivo) pertence ao Zeus.

- **V1a:** guard não roda, rc=1; 5 CLIs falham abertos para deny.
- **V1d:** guard roda mas normalização falha, rc=1; 5 CLIs falham abertos para deny; Copilot
  falso-positivo para allow.

Se classificados in scope, o fix em POSIX seria adicionar `unset null 2>/dev/null; unset LASTEXITCODE 2>/dev/null;` antes das atribuições. Isso precisa ser medido quanto a impacto em PS (onde `unset` não existe como builtin standalone).

---

### A3 — INFORMACIONAL: Laço D18 redundante (V5.4)

O laço explícito de migração D18 em `agentfiles.go` cobre o mesmo conjunto de hooks que o dedup
por registro. O contra-braço dos testes `TestD18Migration_*` só reprova quando AMBOS são removidos.
Não é defeito de segurança; pode ser limpo como tech-debt sem impacto funcional, mas requer
confirmação com testes: remover apenas o laço explícito e verificar que os testes passam.

---

## Estado do Repositório

```
$ git diff --cached --stat
(vazio — sem staged changes)

$ git status --short
?? docs/qualidade/2026-11-15-revisao-linha-de-hook-forma-h.md  ← hefesto-tf, não tocado
?? docs/seguranca/2026-11-13-wave2-redteam-guard-errexit-herdado.md  ← este relatório
```

Apenas este relatório e o arquivo do hefesto-tf estão presentes. Nenhum arquivo de produto
modificado.
