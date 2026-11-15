# Wave 2 — Red Team: Guard Hook Forma K (ML-2H)

> Autor: hades-tf | Data: 2026-11-15
> REQ: REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
> Roadmap: ROADMAP-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
> ML-2H — Red team da forma K

---

## Veredicto

**REPROVA.** A1 é uma **regressão introduzida por esta REQ**: forma K (e forma H antes dela)
adicionam `set +eu` = `Set-Variable "+eu"` a uma linha que antes não continha nenhum cmdlet
antes do guard. Com `$ConfirmPreference <= Medium` sem `-NonInteractive`, o prompt de
Confirmação drena o payload de stdin, o guard recebe stdin vazio e retorna rc=0 (fail-open
para deny). O `main` não contém o problema (medido).

Ressalva 1 — A1 **não alcança o Claude Code** (inferido do binário; canal de execução não
rastreado até o fim para outros CLIs). Para outros CLIs, o alcance permanece não medido.

Ressalva 2 — A2 (alias `set` substituído no perfil): residual da classe R2, confirmado.

**Fix candidate validado** (ver §A1 abaixo): `set ConfirmPreference None -Force -WhatIf:0
-Confirm:0` como segmento 1b, entre EAP reset e `set +eu`. POSIX-safe (20 células option
parsing; 20 células full-form; rc=0/2/normalization corretos). Real trackfw + Confirm=Low
no-NI + deny payload → rc=2 (guard correto). Proposta de ML nesta REQ.

**Achado bônus:** guarda `write-scope` falha fechada em stdin vazio (`rc=2`), enquanto
`git-branch`, `credential` e `credential --global` falham abertas (`rc=0`). A assimetria abre
uma segunda direção de fix (guard-side) independente da string do hook; sem ADR ou decisão
documentada que proíba alinhar o comportamento de git-branch/credential ao de write-scope.

---

## Metodologia

### Fonte da forma K

Extraída de `internal/guardforms/guard_generations.go` constante `GitBranchCurrentPSPOSIX`.
Verificada idêntica em `internal/validator/validator_guard_binary_probe.go` e nos arquivos de
configuração de hook:

```bash
grep -cF 'set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0' \
    internal/validator/validator_guard_binary_probe.go   # → 1
grep -cF 'set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0' \
    .claude/settings.json .codex/hooks.json              # → 1 cada
```

Forma K (git-branch; credential/write-scope/credential-global são estruturalmente idênticas):

```
set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0; set +eu; null=/dev/null;
Set-StrictMode -Off 2>$null; emulate -R sh 2>$null; $LASTEXITCODE=2 2>$null;
trackfw guard git-branch; LASTEXITCODE=$((2*!!$?));
$LASTEXITCODE=2*!!$LASTEXITCODE 2>$null; exit $LASTEXITCODE
```

Delta H→K: `set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0` no lugar de
`set +eu; set ErrorActionPreference Continue`, com EAP reset ANTES do `set +eu`.

### Critério de classificação

Declarado explicitamente:

- **IN SCOPE**: estado de preferência herdável via perfil sem código especificamente adversarial
  (EAP, WhatIfPreference, ConfirmPreference, StrictMode, ReadOnly/AllScope nesses atributos,
  PSDefaultParameterValues genérico, LanguageMode).
- **RESIDUAL**: código no perfil que redefine especificamente um nome usado pela linha de hook
  (`set`, `emulate`), variável marcada como readonly via BASH_ENV, trap ERR via BASH_ENV —
  mesma classe dos residuais R1/R2/R3 declarados na Wave 0 e aceitos pela auditoria do Zeus.

### Harness

Diretório de medição: scratchpad de sessão
(`/private/tmp/claude-501/.../scratchpad/ml2h/`), inteiramente fora do repo.

Fake `trackfw`: nos testes POSIX e PS básicos, o fake escreve `printf 'ran: %s\n' "$*"` em
`$TRACKFW_MARKER` e sai com `$FAKE_RC`. **Nos testes de stdin**: o fake escreve
`ran: %s stdin_bytes=%s` lendo `wc -c <&0`. Para o achado A1, o real `./bin/trackfw` foi
usado para confirmar o fail-open.

PATH de teste: `$SCRATCH/fakebin:/usr/bin:/bin` (sem `/opt/homebrew/bin`, sem o `trackfw`
instalado).

Shells medidos:
- `/bin/bash` = bash 3.2.57 (macOS arm64)
- `/opt/homebrew/bin/bash` = bash 5.3.20
- `/bin/sh` = bash 3.2.57 (symlink)
- `/bin/zsh` = zsh 5.9
- `/bin/dash` = dash 0.5.12
- `/opt/homebrew/bin/pwsh` = PowerShell 7.6.9

Nota: PS 5.1 (Windows VM) está coberto pelo ML-2G (paralelo).

### Positive controls

- VK-1 marker confirmado: `EAP=Stop OPTS=ReadOnly` (antes de executar a forma K).
- VK-11 marker confirmado: `LM=ConstrainedLanguage`.
- VK-2: state reporter no perfil → `EAP=Stop OPTS=None WIF=True CONF=High`.
- VK-3: state reporter → `EAP=Continue OPTS=None WIF=True CONF=High`.
- VK-8 (re-testado, ver abaixo): state reporter → estado correto.
- VK-10R e VK-10D: `(Get-Command set).CommandType` / `.Definition` confirmados antes.
- VK-12: state = `EAP=Stop OPTS=AllScope`.
- VK-13: state = `EAP=Stop OPTS=Private`.
- Stdin-consumption: `stdin_bytes` contados pelo fake; verificação final com real trackfw.

### Contra-braços

Três vetores incluem H como contra-braço (VK-1c, VK-2c, VK-4c) para confirmar que a mesma
condição que K resolve ainda causa rc=1, guard=no na forma H. Todos os três confirmados.

Achado A1: forma legacy/main com Confirm=Low no-NI medida como contra-braço principal
(stdin_bytes=71, rc=2 — confirma A1 como regressão desta REQ). Forma H também medida
(stdin_bytes=0 — H é a primeira ocorrência da regressão dentro desta REQ).

---

## Seção 1 — POSIX: tokens novos `-Force -WhatIf:0 -Confirm:0`

### P1 — Probe do segmento 1 (option parsing)

**Objetivo:** confirmar que `-Force -WhatIf:0 -Confirm:0` são tratados como parâmetros
posicionais (não como opções de `set`) em todos os shells.

**Resultados (20 combinações — 5 shells × none/-e/-u/-eu):**

Todos os 20 casos: rc=0, n=5, `$3=-Force`, `$4=-WhatIf:0`, `$5=-Confirm:0`, `$-` inalterado.

| shell | versão | opts | rc | n | $3 | $4 | $5 |
|---|---|---|---|---|---|---|---|
| /bin/bash | 3.2.57 | todos ×4 | 0 | 5 | -Force | -WhatIf:0 | -Confirm:0 |
| /opt/homebrew/bin/bash | 5.3.20 | todos ×4 | 0 | 5 | -Force | -WhatIf:0 | -Confirm:0 |
| /bin/sh | 3.2.57 | todos ×4 | 0 | 5 | -Force | -WhatIf:0 | -Confirm:0 |
| /bin/zsh | 5.9 | todos ×4 | 0 | 5 | -Force | -WhatIf:0 | -Confirm:0 |
| /bin/dash | 0.5.12 | todos ×4 | 0 | 5 | -Force | -WhatIf:0 | -Confirm:0 |

**Mecanismo:** `ErrorActionPreference` não começa com `-` ou `+`; o builtin `set` trata todos
os argumentos como parâmetros posicionais. Nenhum shell processa `-Force` como opção.

---

### P2 — Matriz completa da forma K em POSIX

**Resultados (40 combinações):**

Todas as 40 células: deny rc=2, guard=yes; allow rc=0, guard=yes; 0 bytes de stderr.

| shell | opts | deny rc | allow rc | stderr |
|---|---|---|---|---|
| bash 3.2 / 5.3 / sh | (none)/-e/-u/-eu | 2 | 0 | 0 B |
| zsh / dash | (none)/-e/-u/-eu | 2 | 0 | 0 B |

---

### P3 — BASH_ENV com `set -e` / `set -u`

BASH_ENV que ativa `set -e` antes do hook: deny rc=2, guard=yes; allow rc=0, guard=yes.
BASH_ENV que ativa `set -u` antes do hook: deny rc=2, guard=yes; allow rc=0, guard=yes.

---

### P4 — SHELLOPTS=errexit

Comando exato (env prefix para contornar readonly no harness):
```bash
env FAKE_RC=2 TRACKFW_MARKER=... PATH=... SHELLOPTS=errexit /opt/homebrew/bin/bash -c "$FORM_K"
```

| shell | SHELLOPTS | deny rc | allow rc | guard |
|---|---|---|---|---|
| /opt/homebrew/bin/bash | errexit | 2 | 0 | sim |
| /bin/bash | errexit | 2 | 0 | sim |
| /bin/sh | errexit | 2 | 0 | sim |

---

## Seção 2 — PowerShell 7.6.9: vetores do delta

**Baseline** (sem perfil, forma K, -NonInteractive): rc=2 (deny), rc=0 (allow), guard=yes, 5
linhas de stderr.

**Mecanismo dos 5 stderr lines (PS, baseline):**
- `null=/dev/null` (segmento 3): 2 linhas (command-not-found + sugestão)
- `emulate -R sh 2>$null` (segmento 4): 1 linha (command-not-found; redirecionado parcialmente
  pelo `2>$null` que usa `$null` automático do PS)
- `LASTEXITCODE=$((2*!!$?))` (segmento 7): 2 linhas (command-not-found, sem redirect 2>$null)

**ConfirmImpact de Set-Variable (medido via reflection PS):** Medium.
Implica: `$ConfirmPreference <= Medium` (Low=1 ou Medium=2) aciona prompt quando `-Confirm:0`
não está presente explicitamente na chamada. Default de `$ConfirmPreference` é High (4) → sem
prompt para a maioria dos usuários.

---

### VK-1 — EAP ReadOnly=Stop (A1 do ML-2A, agora fixo)

**Perfil:** `Set-Variable -Name ErrorActionPreference -Value Stop -Scope Global -Option ReadOnly`
**State confirmado:** `EAP=Stop OPTS=ReadOnly`

| forma | deny rc | guard | allow rc | guard |
|---|---|---|---|---|
| K | 2 | sim | 0 | sim |
| H (contra-braço) | **1** | **não** | **1** | **não** |

**Classificação: FIXO.**

---

### VK-2 — WhatIfPreference=$true + EAP=Stop

**Perfil:** `$WhatIfPreference = $true; $ErrorActionPreference = "Stop"`
**State confirmado:** `EAP=Stop OPTS=None WIF=True`

| forma | deny rc | guard | allow rc | guard |
|---|---|---|---|---|
| K | 2 | sim | 0 | sim |
| H (contra-braço) | **1** | **não** | **1** | **não** |

**Classificação: FIXO.**

---

### VK-3 — WhatIfPreference ReadOnly=$true

**Perfil:** `Set-Variable -Name WhatIfPreference -Value $true -Option ReadOnly -Force`
**State confirmado:** `EAP=Continue WIF=True`

deny rc=2, guard=sim; allow rc=0, guard=sim. Combinado com EAP=Stop: deny rc=2, guard=sim.

**Mecanismo.** `-WhatIf:0` como parâmetro explícito da chamada desabilita WhatIf para aquela
invocação independentemente de `$WhatIfPreference` ser ReadOnly.

**Classificação: SEM EFEITO.**

---

### VK-4 — ConfirmPreference='Low' + EAP=Stop (com -NonInteractive)

**Perfil:** `$ConfirmPreference = "Low"; $ErrorActionPreference = "Stop"`
**Com `-NonInteractive`, stdin_bytes=71 (payload preservado)**

| forma | deny rc | guard | allow rc | guard |
|---|---|---|---|---|
| K | 2 | sim | 0 | sim |
| H (contra-braço) | **1** | **não** | **1** | **não** |

**Com `-NonInteractive`:** o prompt de confirmação de `set +eu` é suprimido. Stdin não é
consumido.

**Sem `-NonInteractive`:** ver A1 abaixo.

**Classificação: FIXO com -NonInteractive. IN SCOPE sem -NonInteractive (A1, regressão desta REQ).**

---

### VK-5 — ConfirmPreference='Medium' + EAP=Stop (com -NonInteractive)

**Com -NonInteractive:** deny rc=2, guard=sim. Stdin_bytes=71 preservado.
**Sem -NonInteractive:** A1 (stdin_bytes=0). Ver seção A1.

**Classificação: SEM EFEITO com -NonInteractive. IN SCOPE sem -NonInteractive (A1, regressão desta REQ).**

---

### VK-6 — ConfirmPreference ReadOnly='Low'

**Perfil:** `Set-Variable -Name ConfirmPreference -Value Low -Option ReadOnly -Force`

deny rc=2, guard=sim; allow rc=0, guard=sim. Com -NI, stdin_bytes=71.
Sem -NI: A1 (stdin_bytes=0).

**Classificação: SEM EFEITO com -NonInteractive. IN SCOPE sem -NonInteractive (A1, regressão desta REQ).**

---

### A1 — ACHADO IN SCOPE: Stdin do guard consumido por prompt de Confirmação

**Condições:** `$ConfirmPreference <= Medium` (Low ou Medium, não default High) no perfil PS
AND hook executado **sem `-NonInteractive`**.

**Evidência medida:**

Medição com fake trackfw (stdin_bytes), PS 7.6.9, payload=71 bytes:

| perfil | NonInteractive | stdin_bytes | rc (fake_rc=2) |
|---|---|---|---|
| (sem perfil) | sim | 71 | 2 |
| (sem perfil) | não | 71 | 2 |
| Confirm=Low | sim | 71 | 2 |
| **Confirm=Low** | **não** | **0** | **2*** |
| **Confirm=Medium** | **não** | **0** | **2*** |
| Confirm ReadOnly=Low | não | **0** | 2* |
| Confirm=Low+Stop | não | **0** | 2* |
| Confirm=Low+WhatIf | não | 71 | 2 (ver nota) |

*rc=2 apenas porque o fake ignora stdin e usa FAKE_RC. Com o guard real, stdin_bytes=0 → rc=0.

**Nota sobre Confirm=Low+WhatIf:** quando `$WhatIfPreference=$true`, PS executa `set +eu` em
modo WhatIf (imprime "What if: Performing...") sem invocar o cmdlet de fato. O modo WhatIf
suprime o prompt de Confirmação. Stdin não é consumido neste caso.

**Medição com real `./bin/trackfw` (confirmação do fail-open):**

Verificação de empty stdin:
```bash
$ printf '' | ./bin/trackfw guard git-branch; echo $?
0
```

Medição real (Confirm=Low, no-NI, payload piped):
```bash
printf '{"tool_name":"Bash","tool_input":{"command":"git checkout -b feature"}}' | \
    HOME=$HOME_TMP XDG_CONFIG_HOME=... pwsh -Command "$FORM_K"
```
Output inclui o prompt de Confirmação consumindo o payload JSON, finalizando com rc=0.

Baseline (sem perfil Confirm, no-NI, real trackfw + payload): rc=2 (deny correto).

**Mecanismo.** `set +eu` = `Set-Variable "+eu"`. ConfirmImpact(Set-Variable)=Medium (medido).
`$ConfirmPreference <= Medium` → PS exibe prompt de confirmação. O prompt chama
`Console.ReadLine()` que lê da stdin herdada. ReadLine lê o JSON completo (payload sem `\n` →
uma linha). PS usa o JSON como resposta Y/N; falha, re-pede. Segunda leitura: EOF. Set-Variable
falha com erro não-terminante (EAP=Continue desde segmento 1). Guard roda com stdin em EOF →
rc=0 (allow, fail-open para deny).

**A1 é uma regressão desta REQ.** Contra-braço com real `./bin/trackfw`, Confirm=Low no-NI,
payload piped (deny):

| forma | Confirm=Low no-NI | rc (real trackfw) | nota |
|---|---|---|---|
| Legacy/main (`emulate...`) | não | **2** | correto |
| H (esta REQ) | não | **0** | fail-open |
| K (esta REQ) | não | **0** | fail-open |
| K-fix | não | **2** | correto |

Forma legacy/main: `emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null};
trackfw guard git-branch; ...` — não contém nenhum cmdlet antes do guard. ConfirmPreference
não é acionado. rc=2 (medido com real trackfw).

Forma H e K: `set +eu` = `Set-Variable "+eu"` é a primeira chamada de cmdlet; dispara o prompt
sob ConfirmPreference <= Medium.

**Reachability per CLI (inferido; canal de hook não rastreado até o fim):**

Fonte: `docs/portabilidade/2026-11-13-guard-errexit-windows.md` linhas 220-225 (texto exato):
> `Zt=["-NoProfile","-NonInteractive","-ExecutionPolicy","Bypass"]`
> (lista de argumentos usada em invocações PS do Claude Code; inferido do binário)
> Para hooks de ferramenta PowerShell, o shell não foi rastreado até o fim naquele documento.

| CLI | Inferência | A1 reachable |
|---|---|---|
| Claude Code | -NoProfile e -NonInteractive (inferido do binário) | **não (inferido)** |
| Windsurf / Copilot / Cursor | não medido | **não medido** |
| Lefthook | não rastreado | **não medido** |

**Impacto por guard em stdin vazio (medido com real trackfw):**

| guard | rc (empty stdin) | direção |
|---|---|---|
| git-branch | 0 | fail-open |
| credential | 0 | fail-open |
| credential --global | 0 | fail-open |
| write-scope | **2** | **fail-closed** |

A assimetria write-scope vs. os demais é uma segunda direção de fix independente da string do
hook: fazer git-branch e credential também retornar rc=2 em stdin vazio (consistency with
write-scope).

**Fix candidate validado:** inserir `set ConfirmPreference None -Force -WhatIf:0 -Confirm:0`
como segmento 1b (entre EAP reset e `set +eu`).

Forma K-fix:
```
set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0;
set ConfirmPreference None -Force -WhatIf:0 -Confirm:0;
set +eu; null=/dev/null; Set-StrictMode -Off 2>$null; emulate -R sh 2>$null;
$LASTEXITCODE=2 2>$null; trackfw guard git-branch;
LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>$null; exit $LASTEXITCODE
```

Resultados P1-FIX (option parsing POSIX): 20 células (5 shells × none/-e/-u/-eu):
rc=0, n=5, $3=-Force, $4=-WhatIf:0, $5=-Confirm:0. POSIX-safe.

Resultados P1-POSIX (forma K-fix completa, stdin=/dev/null): 20 células:
- 5 shells × FAKE_RC=2 (opts padrão): rc=2, guard=yes
- 5 shells × FAKE_RC=0 (opts padrão): rc=0, guard=yes (allow pass-through)
- 5 shells × FAKE_RC=1 (opts padrão): rc=2, guard=yes (rc=1 normalizado para 2)
- 5 shells × FAKE_RC=2 (opts -eu): rc=2, guard=yes

Resultados PS stdin — K-fix vs K, Confirm variants (fake trackfw, PS 7.6.9):

| forma | perfil | NI | stdin_bytes | rc |
|---|---|---|---|---|
| K | Confirm=Low | não | 0 | 2* |
| **K-fix** | **Confirm=Low** | **não** | **71** | **2** |
| K | Confirm=Medium | não | 0 | 2* |
| K-fix | Confirm=Medium | não | **71** | 2 |
| K | Confirm ReadOnly=Low | não | 0 | 2* |
| K-fix | Confirm ReadOnly=Low | não | **71** | 2 |
| K | Confirm=Low+Stop | não | 0 | 2* |
| K-fix | Confirm=Low+Stop | não | **71** | 2 |
| K | Confirm=Low | sim | 71 | 2 |
| K-fix | Confirm=Low | sim | 71 | 2 |

*rc=2 porque fake ignora stdin (FAKE_RC=2). Com real trackfw, stdin=0 → rc=0 (fail-open).

Resultados PS K-fix — vetores críticos da matriz VK (fake trackfw, -NonInteractive, PS 7.6.9):

| vetor | perfil | K-fix deny rc | K-fix allow rc | guard |
|---|---|---|---|---|
| VK-1: EAP ReadOnly=Stop | ReadOnly=Stop | 2 | 0 | sim |
| VK-2: WhatIf+Stop | WhatIf=true+Stop | 2 | 0 | sim |
| VK-4: Confirm=Low+Stop -NI | Low+Stop | 2 | 0 | sim |

Verificado: segmento 1b não quebra nenhum dos fixes anteriores da forma K.
Matrix VK-5..14 com K-fix: não medida (PS 5.1 e os outros 3 guards cobertos como pendente
no ML).

**End-to-end com real trackfw:**

| forma | Confirm=Low no-NI | payload piped | rc (real trackfw) |
|---|---|---|---|
| Legacy/main | não | deny | **2** |
| H | não | deny | **0** |
| K | não | deny | **0** |
| **K-fix** | **não** | **deny** | **2** |

Medido com `printf '...' | PATH=.../bin trackfw guard git-branch` via pwsh -Command.

**Mecanismo do fix.** `-Confirm:0` no segmento 1b suprime o prompt para aquela chamada mesmo
com ConfirmPreference=Low. `-Force` sobrescreve ReadOnly se ConfirmPreference foi marcado
ReadOnly no perfil. Resultado: `$ConfirmPreference = 'None'`. PS nunca prompta quando
ConfirmPreference='None'. Confirmado por stdin=71 em todos os casos medidos.

**Escopo do ML (fix candidate).** K-fix é uma nova geração de forma. Ao adotar K-fix como
`Current*PSPOSIX`, a forma K move para o registro legado D12. Os artefatos afetados:
- As 4 constantes em `internal/guardforms/guard_generations.go`
- O probe do validator em `internal/validator/validator_guard_binary_probe.go`
- As entradas de hook em `.claude/settings.json` e `.codex/hooks.json`
- Os testes de paridade (se cobrirem a string exata)

**Proposta de ML nesta REQ:** "Adicionar segmento `set ConfirmPreference None` entre EAP reset
e `set +eu` em todas as 4 formas K".

Critérios de aceite:
- `stdin_bytes = 71` com ConfirmPreference=Low e sem -NonInteractive (todos os 4 guards,
  PS 7.6.9 macOS; PS 5.1 na VM é pending)
- Contra-braço: forma K atual (sem o segmento): stdin_bytes=0 na mesma condição
- POSIX: 20 células option-parsing, 20 células full-form (FAKE_RC=0/1/2/-eu × 5 shells)
- End-to-end real trackfw + Confirm=Low no-NI + deny payload → rc=2

Direção complementar de fix (guard-side, independente do hook string): tornar git-branch e
credential fail-closed em stdin vazio (consistente com write-scope). Nenhum ADR ou decisão
documentada proíbe alinhar o comportamento. Fecha A1 mesmo para CLIs futuros que não usem
-NonInteractive.

**Nota sobre o impacto assimétrico do A1 em write-scope.** Sob A1, write-scope recebe stdin
vazio → rc=2 (deny ALL writes) → regressão de disponibilidade (falso-positivo, não
fail-open). Antes de propor o fix guard-side para write-scope, medir se CLIs ou eventos
invocam write-scope com stdin vazio em operação normal (ex: Kiro/Copilot deny-all; `--command`
flag bypassa stdin — ver guard.go).

---

### VK-7 — Hang test: ConfirmPreference='Low' stdin=/dev/null

(Ver A1 para o caso com payload. Este sub-teste usa stdin=/dev/null.)

Sem payload (stdin=/dev/null), no-NI, fake trackfw: rc=2 (fake), guard=sim. Sem hang.

**Mecanismo:** sem payload, PS lê EOF imediatamente na primeira ReadLine → erro não-terminante
(EAP=Continue) → script continua → guard roda com stdin vazio → fake retorna FAKE_RC.

**Classificação:** A1 cobre este vetor. Sem hang em nenhum caso medido.

---

### VK-8 — PSDefaultParameterValues {"*:WhatIf"=$true}

**Nota de metodologia:** O vetor original combinava `{"*:WhatIf"=$true}` com `Set-Variable EAP
Stop ReadOnly` no MESMO perfil. `{"*:WhatIf"=$true}` suprimia (WhatIf'd) a própria chamada
`Set-Variable EAP Stop ReadOnly`, então EAP permanecia Continue — o vetor media o caso trivial.

**Re-testado (3 variantes):**

| perfil | deny rc | guard | allow rc | guard |
|---|---|---|---|---|
| PDPv *:WhatIf=true (sozinho) | 2 | sim | 0 | sim |
| EAP=Stop → PDPv *:WhatIf=true | 2 | sim | 0 | sim |
| EAP ReadOnly=Stop → PDPv *:WhatIf=true | 2 | sim | 0 | sim |

**Mecanismo.** `-WhatIf:0` explícito no segmento 1 sobrepõe `$PSDefaultParameterValues["*:WhatIf"]`
para aquela chamada. EAP é resetado para Continue.

**Classificação: SEM EFEITO.**

---

### VK-9 — PSDefaultParameterValues {"*:ErrorAction"="Stop"}

deny rc=2, guard=sim; allow rc=0, guard=sim.

**Classificação: SEM EFEITO.**

---

### VK-10 — Alias `set` substituído no perfil

**Descoberta:** em PS, aliases têm precedência sobre funções (ordem: Alias > Function > Cmdlet).
Definir `function set { ... }` NÃO shadow o alias `set → Set-Variable`. Medido:
`(Get-Command set).CommandType` em perfil com `function set` ainda retorna `Alias`.

**VK-10R — `Set-Alias -Name set -Value Write-Error -Force` + EAP=Stop**

State confirmado: `(Get-Command set).Definition = Write-Error; EAP = Stop`
deny rc=1, guard=não; allow rc=1, guard=não.

**VK-10D — `Remove-Alias set` + função sem parâmetro -Force + EAP=Stop**

State confirmado: `(Get-Command set).CommandType = Function; EAP = Stop`
deny rc=1, guard=não; allow rc=1, guard=não.

**Subcaso Set-Item (medido):** `Set-Alias -Name set -Value Set-Item -Force` + EAP=Stop

```
set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0
  → Set-Item -Path ErrorActionPreference -Value Continue -Force
  → Throws: Provider does not support this operation.
  → EAP still: Stop → terminante → rc=1, guard=não
```

**Classificação: RESIDUAL (classe R2 — código de perfil targeting o nome `set`).**

Redefinir o alias `set` requer código explícito no perfil PS. Mesmo o subcaso Set-Item exige
`Set-Alias -Name set -Value Set-Item -Force`. Não é uma configuração de preferência passiva.
Mesma classe que BASH_FUNC_set (R2).

---

### VK-11 — ConstrainedLanguage via perfil

**State confirmado:** `LM=ConstrainedLanguage`.
CL + EAP ReadOnly=Stop: deny rc=2, guard=sim; allow rc=0, guard=sim.

**Classificação: SEM EFEITO** (macOS PS 7.6.9).

---

### VK-12 — EAP AllScope=Stop

**State confirmado:** `EAP=Stop OPTS=AllScope`.
deny rc=2, guard=sim; allow rc=0, guard=sim.

**Mecanismo.** AllScope não implica ReadOnly; Set-Variable sobrescreve normalmente.

**Classificação: SEM EFEITO.**

---

### VK-13 — EAP Private=Stop

**State confirmado:** `EAP=Stop OPTS=Private`.
deny rc=2, guard=sim.

**Classificação: SEM EFEITO.**

---

### VK-14 — PSDefaultParameterValues {"Set-Variable:Force"=$false} + ReadOnly Stop

`Set-Variable -Force` com `PSDefaultParameterValues["Set-Variable:Force"]=$false` + EAP
ReadOnly=Stop: ainda consegue sobrescrever → `EAP=Continue` (medido). Parâmetros explícitos
têm precedência sobre `$PSDefaultParameterValues`.

**Classificação: SEM EFEITO.**

---

## Tabela Resumo

| Vetor | K deny rc | guard | K allow rc | guard | Classificação |
|---|---|---|---|---|---|
| Baseline (sem perfil, -NI) | 2 | sim | 0 | sim | — |
| VK-1: EAP ReadOnly=Stop | 2 | sim | 0 | sim | FIXO (H: rc=1) |
| VK-2: WhatIf=true+Stop | 2 | sim | 0 | sim | FIXO (H: rc=1) |
| VK-3: WhatIf ReadOnly=true | 2 | sim | 0 | sim | SEM EFEITO |
| VK-3c: WhatIf ReadOnly+Stop | 2 | sim | — | — | SEM EFEITO |
| VK-4: Confirm=Low+Stop -NI | 2 | sim | 0 | sim | FIXO (H: rc=1) |
| VK-4/A1: Confirm=Low no-NI | stdin=0 | sim | — | — | IN SCOPE (A1, regressão desta REQ) |
| VK-5: Confirm=Medium+Stop -NI | 2 | sim | — | — | SEM EFEITO com -NI |
| VK-5/A1: Confirm=Medium no-NI | stdin=0 | sim | — | — | IN SCOPE (A1, regressão desta REQ) |
| VK-6: Confirm ReadOnly=Low -NI | 2 | sim | 0 | sim | SEM EFEITO com -NI |
| VK-6/A1: Confirm ReadOnly=Low no-NI | stdin=0 | — | — | — | IN SCOPE (A1, regressão desta REQ) |
| VK-7: Confirm=Low hang test | 2(fake) | sim | — | — | SEM HANG; A1 cobre |
| VK-8 (re-testado): PDPv *:WhatIf | 2 | sim | 0 | sim | SEM EFEITO |
| VK-9: PDPv *:ErrorAction=Stop | 2 | sim | 0 | sim | SEM EFEITO |
| VK-10 fn shadow | — | — | — | — | INERTE (alias > function em PS) |
| VK-10R: alias→Write-Error+Stop | 1 | não | 1 | não | RESIDUAL R2 |
| VK-10D: Remove+fn no-Force+Stop | 1 | não | 1 | não | RESIDUAL R2 |
| VK-10 subcaso Set-Item+Stop | 1 | não | — | — | RESIDUAL R2 |
| VK-11: ConstrainedLanguage+RO Stop | 2 | sim | 0 | sim | SEM EFEITO |
| VK-12: AllScope=Stop | 2 | sim | 0 | sim | SEM EFEITO |
| VK-13: Private=Stop | 2 | sim | — | — | SEM EFEITO |
| VK-14: PDPv Force=false+RO Stop | 2 | sim | — | — | SEM EFEITO |

---

## Achados por Severidade

### A1 — Stdin consumido por prompt de Confirmação de `set +eu` (IN SCOPE, PRE-EXISTENTE)

**Localização:** segmento `set +eu` introduzido nesta REQ (H e K), ausente na forma legacy/main.

**Evidência concreta:**
- Fake, Confirm=Low, no-NI, FORM_K: `stdin_bytes=0` (esperado 71)
- Fake, Confirm=Low, no-NI, FORM_LEGACY (main): `stdin_bytes=71` (correto)
- Real trackfw + empty stdin: `rc=0` (allow, fail-open) para git-branch e credential
- Real trackfw + Confirm=Low, no-NI, FORM_K, payload piped: `rc=0` (allow para comando negado)
- Real trackfw + Confirm=Low, no-NI, FORM_KFIX, payload piped: `rc=2` (deny correto)
- Baseline (sem ConfirmPreference=Low): `stdin_bytes=71`, rc=2 para deny

**Reachability:**
- Claude Code: -NoProfile e -NonInteractive (inferido do binário; canal de hook não rastreado
  até o fim) → **não alcançável (inferido)**
- Outros CLIs: não medido

**Impacto assimétrico por guard:**
- git-branch, credential, credential-global: fail-open (`rc=0` em stdin vazio)
- write-scope: fail-closed (`rc=2` em stdin vazio)

**Fix candidate validado:** segmento `set ConfirmPreference None -Force -WhatIf:0 -Confirm:0`
como segmento 1b. Validado em 20 células POSIX (option parsing) + 20 células POSIX full-form
(FAKE_RC=0/1/2/-eu × 5 shells) + 8 células PS stdin com variants de Confirm + end-to-end com real
trackfw (Confirm=Low no-NI deny payload → rc=2).

**Proposta de ML:** ver §A1 acima para critérios de aceite e direção complementar (guard-side).

---

### A2 — Alias `set` substituído por código de perfil (RESIDUAL R2)

rc=1, guard=não para deny e allow. Requer código explícito no perfil targeting `set`.
Mesmo subcaso Set-Item exige `Set-Alias -Name set` explícito. Mesma classe que BASH_FUNC_set.

---

## Verificação de Integridade

```
$ git diff --cached --stat
(vazio — sem staged changes)

$ git status --short
 M .claude/agent-memory/zeus-tf/MEMORY.md
 M docs/portabilidade/2026-11-13-guard-errexit-windows.md
?? .claude/agent-memory/zeus-tf/feedback_ler_o_log_antes_de_atribuir_falha.md
?? docs/seguranca/2026-11-15-wave2-redteam-forma-k.md
```

Apenas este relatório foi criado por este ML. Nenhum arquivo de produto, teste ou outro doc
modificado.

---

## Estado

**Começou:** red team da forma K (ML-2H), reimplementando a matriz sem conferir o diff do ML-2C.

**Mudou:** criação de
`/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/docs/seguranca/2026-11-15-wave2-redteam-forma-k.md`
com todos os vetores medidos e os achados classificados.

**Ficou pendente:**
- ML-2G (PS 5.1 na VM, em paralelo — ConfirmPreference=Low no PS 5.1 é variável adicional
  para A1; reachability para PS 5.1 não coberta aqui)
- Outros CLIs (Windsurf, Copilot, Cursor, Lefthook): reachability de A1 não medida
- Zeus decide: (a) status do A2 subcaso Set-Item, (b) se direciona fix para hook-string
  (K-fix), guard-side (fail-closed em stdin vazio) ou ambos
- Atualização do roadmap (ML-2H → status) é responsabilidade do trackfw_architect

**Notas de vault (não escritas, listadas para o arquiteto registrar se julgar necessário):**
- Em PS, o alias tem precedência sobre função de mesmo nome: Alias > Function > Cmdlet
- ConfirmImpact de Set-Variable = Medium (medido via reflection)
- `$ConfirmPreference <= Medium` + sem -NonInteractive + stdin piped = payload consumido pelo
  prompt de Confirmação de `set +eu` → guard recebe stdin vazio → rc=0 (allow, fail-open); A1
  regressão introduzida na forma H (esta REQ); forma legacy/main não afetada (stdin=71);
  Claude Code usa -NonInteractive (inferido) → não afetado; outros CLIs: não medido
- Fix candidate: `set ConfirmPreference None -Force -WhatIf:0 -Confirm:0` como segmento 1b;
  POSIX-safe (mesma semântica que segmento 1); PS correto (stdin=71 em todas variantes)
- Guard asymmetry: write-scope fail-closed em stdin vazio (rc=2); git-branch/credential/
  credential-global fail-open (rc=0); segunda direção de fix independente da string do hook
- `$WhatIfPreference=$true` + `$ConfirmPreference="Low"`: WhatIf mode suprime Confirm prompt →
  stdin não consumido (efeito colateral benigno; não coberto pela forma K explicitamente)
