# Wave 0 — Threat Model: Cobertura e compatibilidade completas no Windows

> Autor: hades-tf | Data: 2026-11-15
> REQ: REQ-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md
> Roadmap: ROADMAP-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md
> ML-0A — Threat model

---

## Veredicto

**Aprovado para início da implementação, com nove requisitos de implementação adicionados (§5)
derivados dos achados abaixo. Os três itens de maior severidade são: (1) retorno silencioso no
PSInheritedState que não reporta skip em Windows; (2) HOME de 6 CLIs com global hooks não resolvido
via USERPROFILE; (3) cmd.exe guard form nunca execution-testado sob cmd.exe real.**

---

## 1 — Completude da enumeração

A enumeração da REQ está substancialmente correta. As lacunas abaixo foram encontradas e precisam
entrar no inventário da ML-0B.

### 1.1 Skips reportados versus skips totais

A REQ cita 71 skips únicos extraídos do log do `windows-full-suites`. Esse número vem
exclusivamente de linhas `--- SKIP` no `go test -v`. Existem categorias de cobertura ausente que
nunca geram essa linha:

**a) Arquivos com build tag `!windows`** — compilados no macOS/Linux, nunca na CI do Windows. No
runner `windows-latest`, esses arquivos não são sequer compilados: a build tag exclui o arquivo
inteiramente do binário de teste, então a CI não reporta skip nem PASS para eles — eles
simplesmente não existem na execução:

| Arquivo | Funcionalidade |
|---|---|
| `internal/validator/regularfile_fifo_unix_test.go` | comportamento de FIFO no regularfile |
| `internal/validator/validator_credential_guard_integrity_fifo_unix_test.go` | FIFO no credential guard |
| `internal/commands/netexec_orphan_unix_test.go` | processo órfão no netexec |
| `internal/commands/netexec_groupkill_unix_test.go` | group kill no netexec |
| `internal/identity/identity_notwindows_test.go` | identidade (plataforma não-Windows) |

**b) Retorno silencioso que reporta PASS** — o caso mais crítico encontrado:

`internal/generators/guard_line_shell_test.go:2315-2318`:
```go
if runtime.GOOS == "windows" {
    t.Log("perfil via XDG_CONFIG_HOME pulado no Windows")
    return  // ← não é t.Skip; reporta PASS, não SKIP
}
```

Isso significa que o braço "perfil" do `TestGuardLineShellMatrix_PSInheritedState` — que testa os
7 estados de EAP/WhatIf/Confirm/StrictMode impostos pelo perfil do usuário — é declarado PASS no
Windows sem executar nenhum assertion. Esta é a superfície mais perigosa: o problema que a D18
(REQ-2026-11-13) corrigiu para o braço `-Command` pode ainda existir no braço "perfil" no Windows.

### 1.2 Artefatos `.sh` do consumidor — lista incompleta

A REQ lista 3 scripts gerados:
- `scripts/trackfw-attention-signal.sh`
- `scripts/trackfw-attention-cleanup.sh`
- `scripts/trackfw-validate.sh`

Faltam dois arquivos adicionais com dependência de POSIX shell:

- `.lefthook/commit-msg/trackfw-req-check.sh` — gerado por `scaffold.go:1850` com
  `runner: sh` em `scaffold.go:1834`. O `lefthook.yml` aponta para `runner: sh`, que falha em
  Windows sem Git Bash.
- Husky pre-commit hook — gerado por `scaffold.go:1887` com `#!/usr/bin/env sh`. PowerShell
  não interpreta shebang.

E dois scripts de guard que ainda são gerados via `trackfw init` (não apenas migration targets):
- `scripts/trackfw-credential-guard.sh` (`scaffold.go:1286, 1339`)
- `scripts/trackfw-git-branch-guard.sh` (`scaffold.go:1415, 1468`)

### 1.3 Resolução de HOME global — 6 de 7 CLIs não resolvidos

`agentconfig.go:128-132` — apenas Claude Code recebe resolução via `USERPROFILE` no Windows:

```go
func (OSCLIHomeResolver) HomeFor(cli string) (string, error) {
    if cli == CLIClaude && effectiveGOOS() == "windows" {
        return effectiveUserHomeDir()
    }
    return homedir.Dir()   // ← HOME primeiro para todos os outros CLIs
}
```

Os 6 CLIs com global hooks que usam `homedir.Dir()` são: Codex, Gemini, Cursor, Copilot, Windsurf
e Kiro (confirmado por `update.go:683-727` — `harnessCredentialGuardTarget*` para cada um). Amazon
Q NÃO tem global hooks (`harnessCredentialGuardTargetAmazonQ` não existe) — só project hooks em
`.amazonq/cli-agents/`; logo, HOME resolution não se aplica a ele.

Consequência: quando `HOME ≠ USERPROFILE` no Windows (ex.: Git Bash), o `trackfw update harness`
grava os guards globais para 6 CLIs no diretório errado, e o `validate` reporta saúde do arquivo
que esses CLIs nunca leram. Este é o defeito exato que ADR-2026-10-14 descreveu e corrigiu apenas
para Claude.

### 1.4 Barrier depende de `sh` — fail-closed, mas não declarado

`barrier.go:311` chama `exec.Command("sh", "-n")` para validação de fragmento de gate.
`barrier.go:1462` chama `exec.Command("sh")` para execução de gate.

Quando `sh` está ausente do PATH (Windows sem Git Bash), `evalGateCommands` retorna
`"not_evaluated"` → `overallStatus = "blocked"` → `os.Exit(1)`. Isso é fail-closed e correto.

O barrier exibe diagnóstico legível: `barrier.go:486` define
`shMissingMsg = "gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates"`,
que é incluído no output `barrier` quando `sh` não pode ser invocado (`barrier.go:1489`).

O que falta: `validate` e `doctor` não emitem aviso *proativo* sobre a ausência de `sh`. Um
consumidor que nunca roda `barrier` não descobre a dependência de `sh` via `validate` ou `doctor`.
A REQ não enumera esta superfície.

### 1.5 cmd.exe guard form — executado mas nunca testado sob cmd.exe real

`internal/guardforms/guard_generations.go:80`:
`GitBranchCurrentCmdExe = "trackfw guard git-branch || exit 2"` (linha 80)
`CredentialCurrentCmdExe = "trackfw guard credential || exit 2"` (linha 71)

Estas formas são usadas por Kiro e Amazon Q. `guard_subprocess_test.go:90-105`
(`TestGuardParentCmd_SemicolonArg_ExitsTwo`) testa que `trackfw guard 'git-branch;'` retorna rc=2
— este é o vetor de tokenização cmd.exe (arg `;` injetado), e o teste roda em todas as
plataformas. O que não existe: um teste que execute
`cmd /c "trackfw guard git-branch || exit 2"` sob cmd.exe real no `windows-latest` com payload de
deny e allow. Os testes que cobrem execução da forma D11 pulam no Windows:

- `fail_closed_d11_behavior_test.go:91-93`: `t.Skip("linha D11 PS/POSIX não se aplica diretamente ao cmd.exe")`
- `fail_closed_d11_behavior_test.go:132-133`: `t.Skip("teste usa stub #!/bin/sh — não aplicável em cmd.exe/PS")`

### 1.6 Per-CLI -NonInteractive — cobertura do D18 A1

O residual A1 da D18 (REQ-2026-11-13) é: forma L sem `-NonInteractive` pode permitir que PS drain
stdin via Confirm prompt se `ConfirmPreference <= Medium`. A forma L preamble corrige isso
inserindo `set ConfirmPreference None -Force` ANTES do guard. Para cada CLI que usa PS, se
`-NonInteractive` é passado, o preamble é redundante; se não é, o preamble é a única defesa.

| CLI | Shell no Windows | `-NonInteractive` passado | Grau de certeza | Impacto A1 sem forma L |
|---|---|---|---|---|
| Cursor | `pwsh`→`powershell` | **Sim** (`-NoProfile -NonInteractive -ExecutionPolicy Bypass`) | MEDIDO (bundle Cursor 3.19.7, `workbench.desktop.main.js:334`) | mitigado pelo flag |
| Codex CLI | `pwsh`→`powershell` | **Não encontrado** na leitura de código | INFERIDO | forma L é a única defesa |
| Gemini CLI | `pwsh`→`powershell` | **Não encontrado** (`spawn` sem argsPrefix NI) | MEDIDO no código-fonte | forma L é a única defesa |
| Claude Code | Git Bash / PS fallback | **Não encontrado** para PS | INFERIDO (binário fechado) | A1 não alcança Claude Code segundo forma-K §Ressalva1 |
| Windsurf | PS | **Não encontrado** | INFERIDO | forma L é a única defesa |
| Copilot | hook ausente no Windows (campo `bash` apenas) | — | MEDIDO (schema doc) | guard nunca dispara |
| Kiro | `cmd.exe` | — | MEDIDO (bundle Kiro 1.0.437) | cmd.exe não interpreta PS; forma cmd.exe não afetada |
| Amazon Q | `cmd.exe` (project hooks only) | — | MEDIDO (update.go — sem global hooks) | idem Kiro |

Cobertura da forma L: o preamble `set ConfirmPreference None -Force -WhatIf:0 -Confirm:0` está em
cada linha gerada (credential e git-branch) para PS/POSIX (linhas 65-80 de `guard_generations.go`).
Isso fecha A1 para todos os CLIs PS sem `-NonInteractive`, desde que a linha seja executada pelo
PS do CLI — o que não é garantido para CLIs cujo shell no Windows não foi medido (Codex, Windsurf).

### 1.7 Release: workflows de release nunca executam no Windows

`release.yml:33-397` — todos os jobs têm `runs-on: ubuntu-latest`. O binário
`windows/arm64` é compilado e empacotado pelo GoReleaser mas nunca executado em nenhum workflow.
O `quality.yml` tem runners `windows-latest` mas não está no caminho de release.

### 1.8 Código de produto com tratamento condicional por GOOS

Além dos testes, o código de produto aplica tratamento diferenciado no Windows. Estes caminhos
precisam de cobertura no inventário ML-0B:

| Arquivo | Linha(s) | Tipo | Comportamento diferenciado no Windows | Teste em windows-full-suites |
|---|---|---|---|---|
| `internal/guard/credential.go` | 508 | **desativa** | backslash escaping desativado | exercitado por testes do pacote credential |
| `internal/guard/credential.go` | 896 | **adiciona** | resolução de Alternate Data Streams | exercitado apenas se fixture ADS existir |
| `internal/guard/writescope.go` | 688, 696 | **adapta** | comparação case-insensitive de caminhos | exercitado por testes do pacote writescope |
| `internal/agentconfig/agentconfig.go` | 129 | **adapta** | apenas `CLIClaude` usa USERPROFILE | exercitado por agentconfig tests |
| `internal/agentconfig/agentconfig.go` | 180-190 | **adiciona** | `HomeDivergencePaths()` Windows-only diagnostic | exercitado por agentconfig tests |
| `internal/agentconfig/agentconfig.go` | 292 | **adapta** | `ClaudeConfigDir` via `OSHome` | exercitado por agentconfig tests |
| `internal/integrations/doctor.go` | 79 (comment) | **desativa** | `DoctorScaffoldWrongMode` não emitido no Windows (execute bit) | comportamento verificável via doctor tests |
| `internal/validator/validator_guard_binary_probe.go` | 352 | **adiciona** | Git Bash binary probe quando PS/POSIX form presente | exercitado por validator tests com `CurrentGOOS="windows"` |
| `internal/commands/netexec_windows.go` | — | **adiciona** | stub Windows para controle de grupo de processos | exercitado por netexec tests |
| `internal/validator/regularfile_windows.go` | — | **adiciona** | stub Windows para tipo de arquivo regular | exercitado por regularfile tests |
| `internal/validator/validator_project_foreign_write_windows.go` | — | **adiciona** | verificação de escrita em projeto alheio | exercitado por validator tests |
| `internal/translog/regularfile_windows.go` | — | **adiciona** | stub para regularfile no translog | exercitado por translog tests |

Os caminhos em `credential.go`, `writescope.go` e `agentconfig.go` executam nos testes de pacote
do `windows-full-suites`. O que falta nos testes: cenários de integração end-to-end com inputs
Windows-específicos (caminhos com backslash, ADS como `C:\file:stream`, HOME≠USERPROFILE). Os
stubs `*_windows.go` compilam e executam apenas no runner Windows.

### 1.9 Enumeration verdict

A lista da REQ cobre os itens principais. As lacunas adicionais são:

| Item faltante | Arquivo/evidência | Severidade |
|---|---|---|
| Build-tag `!windows` (5 arquivos) | arquivos listados em §1.1a | LOW |
| Retorno silencioso no PSInheritedState | `guard_line_shell_test.go:2315` | HIGH |
| `.lefthook` runner sh e husky shebang | `scaffold.go:1834,1887` | MEDIUM |
| Scripts guard gerados por init | `scaffold.go:1286,1415` | MEDIUM |
| HOME global — 6 CLIs com global hooks não resolvidos | `agentconfig.go:128-132` + `update.go:683-727` | HIGH |
| `sh` ausente no barrier — sem check proativo em validate/doctor | `barrier.go:486,1489` | LOW |
| PSInheritedState deny arm usa os.Stat (não stdin_bytes) | `guard_line_shell_test.go:2256` | HIGH |
| cmd.exe guard nunca execution-testado sob cmd.exe real | `fail_closed_d11_behavior_test.go:91-133` | HIGH |
| Release: binário arm64 nunca executado em CI | `release.yml:33-397` | MEDIUM |
| Per-CLI -NonInteractive — Codex/Windsurf não medidos | §1.6 | LOW |
| Caminhos de produto GOOS-condicional (cenários end-to-end) | §1.8 (12 entradas) | MEDIUM |

---

## 2 — Threat Model

O adversário aqui é o implementador apressado e o arquiteto otimista — não um atacante externo.

### T1 — Reclassificar skip como (c) sem prova

**Mecanismo**: A REQ define (c) como "impossível na plataforma, com prova escrita." Mas o gate AC3
não existe ainda. O ratchet (`check-windows-known-failures.py`) valida apenas nomes em
`windows-known-failures.json` — não inspeciona código-fonte em busca de `t.Skip` sem classificação.

Um implementador pode acrescentar:
```go
if runtime.GOOS == "windows" {
    t.Skip("not applicable on Windows")  // sem referência a (a), (b) ou (c)
}
```
e a suíte Windows-full-suites passa, o ratchet passa, o barrier passa — sem nenhuma prova de que
o teste é de fato impossível na plataforma.

**Porta de entrada**: qualquer PR que adicione um novo `t.Skip` com texto arbitrário.
**Gate que deve fechar**: AC3 — um linter estático ou grep no CI que rejeite `t.Skip` com
`runtime.GOOS == "windows"` sem comentário no formato `// classification: (a|b|c) <motivo>`.

### T2 — Mover para known-failures em vez de corrigir

**Mecanismo**: O roadmap ML-1B aceita "ou cada um entra em `windows-known-failures.json` com causa
medida." O ratchet aceita qualquer entrada com `reason` preenchido (ASCII). Um implementador pode
adicionar todos os 41 `TestMLScope*` ao `windows-known-failures.json` com razão "git command output
uses backslash separator on Windows (MEASURED)" e marcar ML-1B como concluído.

Isso contradiz AC2 ("zero skip com justificativa falsa") — mas os critérios não dizem que
known-failures não satisfaz AC2. O roadmap abre a saída ele mesmo.

**Porta de entrada**: ML-1B criando entradas em `windows-known-failures.json` em vez de remover os
skips.
**Gate que deve fechar**: AC2 deve ser explicitado: "nenhum `TestMLScope*` ou `TestBarrierReentry_*`
com skip ou known-failure não-medido"; ML-1B deve exigir evidência de execução no Windows (PASS ou
FAIL medido), não apenas adição ao ratchet.

### T3 — Smoke de canal vacuo

**Mecanismo**: AC5 pede `trackfw --version` e `trackfw validate` no `windows-latest`. Da memória
do projeto (nota `validate-no-git-init-needed`): `trackfw validate` em diretório vazio retorna
rc=0. Um smoke que apenas executa esses dois comandos em um diretório temporário vazio:
1. Prova que o binário inicia
2. Não prova nada sobre as funções de guard

**Porta de entrada**: PR que adiciona um step no `quality.yml` executando apenas `trackfw --version`
e `trackfw validate` num diretório recém-criado.
**Gate que deve fechar**: AC5 deve exigir além de `--version` e `validate` os dois braços de
stdin abaixo (medidos, sem projeto configurado):
(a) `echo 'git branch some-branch' | trackfw guard git-branch` → rc=2 (stdin preservado confirma deny; rc=0 seria drop ou skip)
(b) `echo '{"tool_name":"Write","tool_input":{"file_path":"/tmp/x"},"cwd":"/tmp"}' | trackfw guard write-scope` → rc=0 (stdin preservado confirma allow; rc=2 seria drop)

### T4 — Substituir braço de perfil pelo braço -Command

**Mecanismo**: AC6 diz "guard via perfil e sem -NonInteractive no PS 5.1 e no pwsh 7 do Windows."
O teste existente já cobre o braço `-Command` para pwsh 7 (7 estados × allow/deny). Um
implementador pode adicionar o braço `-Command` para PS 5.1 e declarar AC6 satisfeito, sem nunca
testar a herança via perfil — que é o caminho mais perigoso (perfis carregam automaticamente,
o usuário não sabe).

No Windows, o pwsh 7 lê `%APPDATA%\PowerShell\Microsoft.PowerShell_profile.ps1`.
O PS 5.1 lê `%USERPROFILE%\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1`.
Nenhum dos dois usa `XDG_CONFIG_HOME`, que é o mecanismo do teste atual.

**Porta de entrada**: PR que adiciona PS 5.1 ao `psEntries` em `guard_line_shell_test.go` mas não
modifica o braço de perfil (linha 2315).
**Gate que deve fechar**: AC6 deve exigir explicitamente: (a) PS 5.1 no braço `-Command` com todos
os estados; (b) pwsh 7 E PS 5.1 no braço "perfil" usando `$PROFILE` real no runner Windows, não
`XDG_CONFIG_HOME`.

### T5 — Declarar guard não-funcional como satisfatório sob AC4

**Mecanismo**: AC4 diz "executa no Windows sem Git Bash, ou o `validate`/`doctor` declara que não
executa." ADR-2026-09-05 D4 diz que cortes caem em conveniência, nunca em guards. Mas AC4 no
roadmap não distingue: um implementador pode emitir um aviso do `validate` para o guard de
credential do Kiro e declarar AC4 satisfeito.

**Porta de entrada**: PR que modifica o `validate` para emitir um aviso (não erro) sobre o guard
do Kiro/Amazon Q e marca AC4 como concluído.
**Gate que deve fechar**: AC4 deve ter dois braços distintos: (i) artefatos de conveniência
(`attention-*.sh`, `trackfw-validate.sh`) podem ser declarados via `validate`/`doctor`; (ii) guards
(`credential`, `git-branch`, `write-scope`) devem executar — declaração não satisfaz.

### T6 — Fail-open por stdin vazio em git-branch e credential

**Mecanismo**: Medido neste ML-0A com o binário compilado:

```
$ guard git-branch </dev/null; echo RC=$?
RC=0

$ guard credential </dev/null; echo RC=$?
RC=0

$ guard credential --global </dev/null; echo RC=$?
RC=0

$ guard write-scope </dev/null; echo RC=$?
trackfw guard write-scope: denied: payload is not JSON
RC=2
```

`git-branch`, `credential` e `credential --global` falham abertos quando stdin é vazio. Apenas
`write-scope` falha fechado. A assimetria está pré-existente (documentada no parecer da forma K,
ML-2H). Qualquer caminho no canal npm/pypi/instalação direta que feche ou redirecione
stdin antes de chamar o guard produz fail-open silencioso.

A forma PS/POSIX atual (`set ErrorActionPreference Continue -Force ...`) não protege contra isso —
o preamble executa antes do guard ser chamado, mas se o stdin do processo CLI do agente não
foi propagado até o guard, o guarda recebe stdin vazio e retorna 0.

**Porta de entrada**: qualquer wrapper que não propague stdin (`{stdio: 'pipe'}` em vez de
`'inherit'` no npm, ou um batch file que não use `<&0`).
**Gate que deve fechar**: AC6 deve exigir medição explícita do comportamento com stdin vazio para
todos os quatro subcomandos no Windows. A assimetria deve ser declarada como residual ou corrigida.

### T7 — Inventário incompleto permite AC1 vacuo

**Mecanismo**: AC1 pede "inventário completo com classificação (a)/(b)/(c) por item." Se o
inventário (ML-0B) não capturar os itens de §1.2-1.6 acima (os 5 `.sh` extras, a HOME de 7 CLIs,
o `sh` no barrier, o cmd.exe), um implementador pode marcar AC1 como concluído com um inventário
que cobre apenas os 71 skips do log.

**Porta de entrada**: ML-0B que extrai itens apenas do log de CI, sem varredura de código.
**Gate que deve fechar**: O inventário deve ser auditado contra os itens deste §1.

---

## 3 — Alvos de falsificação nas duas direções

### Superfície A: TestMLScope* (41 testes, barrier_ml_scope_test.go)

**Entrada da sabotagem**: `barrier_ml_scope_test.go:268-271` — bloco `if runtime.GOOS == "windows" { t.Skip(...) }`.

**Falso negativo (gate não pega regressão)**: Os 41 testes que cobrem casamento de path no
ml_scope (incluindo separador backslash/forward-slash, caminhos relativos, escopo fora da onda)
nunca rodam no Windows. Um bug no `checkMLScope` que aceite caminhos Windows como sempre fora do
escopo deixaria o barrier fail-open no Windows para qualquer ML.

**Falso positivo (gate bloqueia trabalho legítimo)**: Remover os skips sem adaptar as fixtures
para produção de caminhos com `filepath.FromSlash` causaria falhas porque `git diff --name-only`
retorna `\` no Windows. Um gate que apenas remove os skips sem adaptar o código produz falhas
legítimas.

**Gate correto**: `go test -run TestMLScope ./internal/commands/... -v` deve passar no
`windows-latest`; a suíte deve ter um subset de testes que não dependem de git e podem rodar
incondicionalmente (classificação a).

### Superfície B: TestBarrierReentry_T1/T2/T3 (barrier_reentry_test.go)

**Entrada**: `barrier_reentry_test.go:186-187,230-231,279-280`.

**Falso negativo**: Skip alega "Windows coberto por windows-known-failures.json" mas a lista não
tem nenhuma entrada de reentry. O mecanismo de detecção de reentrada (`TRACKFW_BARRIER_STACK`)
nunca foi testado no Windows — se um bug de propagação de variável de ambiente existe no
PowerShell, isso é invisível.

**Falso positivo**: Adicionar T1/T2/T3 ao `windows-known-failures.json` com razão plausível
(ex.: "shell gate requires POSIX sh (MEASURED)") satisfaz o ratchet sem medir nada.

**Gate correto**: ML-1B deve medir rc real de T1/T2/T3 no Windows. Se passam sem `sh`, classificar
(a); se falham por `sh`, medir qual rc e por qual motivo (clasificar b ou c com evidência CI).

### Superfície C: Forma PS/POSIX sob perfil do usuário (Windows)

**Entrada**: `guard_line_shell_test.go:2315` — `t.Log(...); return`.

**Falso negativo (crítico)**: O braço de perfil retorna silenciosamente no Windows. Qualquer
regressão que o preamble da forma L não neutralize via perfil (ex.: um estado PS que a forma L
não cobre, semelhante ao `ConfirmPreference = Low` que a forma K não cobria) seria PASS no Windows.
Isso é exatamente o bug que a D18 corrigiu para o braço `-Command` mas que pode ainda existir no
braço "perfil".

**Falso positivo**: Substituir o retorno silencioso por um teste que usa `XDG_CONFIG_HOME` no
Windows (tentando a mesma abordagem do macOS) produziria falso PASS, pois o pwsh no Windows não
lê `XDG_CONFIG_HOME` como local do perfil.

**Achado adicional — deny arm usa `os.Stat(stdin.out)` (não `stdin_bytes`)**: O teste existente
no braço `-Command` (`guard_line_shell_test.go:2256`) verifica apenas que `stdin.out` EXISTE
(o guard rodou), mas não verifica o CONTEÚDO de `stdin.out`. O stub `cat > stdin.out` cria o
arquivo mesmo com stdin vazio (A1: stdin drenado pelo Confirm prompt). Resultado: o teste passa
mesmo quando stdin foi drenado — o guard rodou com stdin vazio, escreveu `stdin.out` vazio, e
`os.Stat` vê o arquivo. A forma correta é verificar `len(stdin.out) == len(payload)`, como o
teste principal `TestGuardLineShellMatrix` faz em `guard_line_shell_test.go:832,849`. Este gap
existe no braço `-Command` existente e é duplamente crítico para o braço "perfil" no Windows.

**Gate correto**: AC6 deve exigir teste com `$PROFILE` real com assertion de `stdin_bytes` (não
apenas `os.Stat`). No `windows-latest`, o profile path correto é `$PROFILE` (pwsh 7) e
`$env:USERPROFILE\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1` (PS 5.1). O
runner `windows-latest` tem PS 5.1 (`powershell.exe`) instalado — braço PS 5.1 é OBRIGATÓRIO,
não opcional.

### Superfície D: Forma cmd.exe (`trackfw guard ... || exit 2`)

**Entrada**: `internal/guardforms/guard_generations.go:71,80,102`.

**Falso negativo**: A forma `trackfw guard credential || exit 2` em `cmd.exe` assume que `||` em
cmd propagará o exit code 2. Se o comando for chamado como `cmd /c "trackfw guard credential || exit 2"`,
e se `trackfw guard credential` retorna rc=0 (fail-open por stdin vazio), `exit 2` não executa —
correto. Mas se stdin é fechado antes da chamada, rc=0 fail-open não é capturado. Nunca medido
em cmd.exe real.

**Falso positivo**: O formato `||` em cmd.exe é bem documentado; a forma estrutural provavelmente
está correta. Um falso positivo seria bloquear a forma por suspeita de comportamento sem medir.

**Gate correto**: Um test no `windows-full-suites` que execute `cmd /c "trackfw guard git-branch || exit 2"`
com payload de deny e payload de allow, verificando rc.

### Superfície E: HOME global para 6 CLIs com global hooks no Windows

**Entrada**: `agentconfig.go:128-132` — apenas `CLIClaude` tem resolução via `USERPROFILE`.

**Falso negativo**: Guards escritos em `HOME/.codex/hooks.json`, `HOME/.gemini/settings.json`,
etc., quando `HOME ≠ USERPROFILE`, são invisíveis para o CLI do agente. O `validate` reporta
saúde verde, o consumidor acredita que está protegido, e o guard nunca dispara.

**Falso positivo**: Mudar `homedir.Dir()` para `os.UserHomeDir()` globalmente quebraria Linux/macOS
onde `os.UserHomeDir()` retorna `HOME` (correto, mas sem o fallback que `homedir.Dir()` oferece
para ambientes sem `HOME` definido).

**Gate correto**: D3 do ADR-2026-10-14 — escritor e auditor devem usar o mesmo resolvedor por CLI.
A implementação deve estender o `OSCLIHomeResolver.HomeFor()` para Codex, Gemini, Cursor, Copilot,
Windsurf, Kiro e Amazon Q com evidência medida por CLI (ou declarar como INFERIDO com dívida
documentada). A divergência `HOME ≠ USERPROFILE` medida pelo `validate` (D4) é diagnóstico
auxiliar, não a correção.

### Superfície F: AC3 gate de skip sem classificação

**Entrada**: ausência de qualquer gate que verifique `t.Skip` com `GOOS == "windows"` em
código-fonte.

**Falso negativo**: Um PR que adiciona `t.Skip("not tested")` não é rejeitado por nenhum gate CI
existente. O ratchet só lida com resultados de execução, não com código-fonte.

**Falso positivo**: Um gate textual (grep) que busca `GOOS == "windows"` em contexto de skip pode
flagrar skips legítimos do tipo `if ps.bin == "powershell" && runtime.GOOS != "windows"` (linha
888 — skipa no macOS, não no Windows) ou o skip de junction em `containment_junction_windows_test.go`
que é condicionado por permissão de mklink. Um gate textual precisa distinguir:
- `if runtime.GOOS == "windows" { t.Skip(...) }` → requer classificação
- `if runtime.GOOS != "windows" { t.Skip(...) }` → skip em não-Windows, não requer

**Gate correto**: grep/AST que casa `runtime.GOOS == "windows"` antes de `t.Skip` ou `t.Skipf`.
Build-tag `!windows` também requer classificação (não é capturado pelo grep).

### Superfície G: Canais npm/pypi no Windows (stdin e exit code)

**Entrada**: `npm/bin/trackfw.js` — `spawnSync(binPath, argv, {stdio: "inherit"})`.

**Falso negativo**: O shim npm usa `stdio: "inherit"`, que propaga stdin. Correto. Mas a linha
`process.exit(result.status ?? 1)` propaga `1` quando o guard é morto por sinal (status null).
No Windows, o guard nunca chega a ser killed por sinal no caminho normal, mas se o processo
recebe `CTRL_C_EVENT` durante uma verificação, o exit code seria `1` (não `2`). CLIs que
interpretam `exit 1` como "deny" funcionam corretamente; CLIs que interpretam apenas `exit 2`
como "deny" falham abertos.

**Hipótese adicional — PATHEXT e resolução npm `.cmd`/`.ps1`**: npm no Windows cria shims
`trackfw.cmd` e `trackfw.ps1` no diretório global de binários npm. O PATHEXT padrão do Windows
inclui `.EXE;.CMD;.BAT;.PS1` nessa ordem, logo `trackfw` em cmd.exe resolve para `trackfw.cmd`
(correto — chama o shim Node.js). Mas `.ps1` requer `ExecutionPolicy >= AllSigned` ou `Bypass` em
PS 5.1 sem `-ExecutionPolicy Bypass`; se o CLI chama `trackfw` via `powershell.exe` (PS 5.1) sem
Bypass, `trackfw.ps1` pode ser bloqueado pela execution policy padrão (`Restricted`). Esta hipótese
exige medição na VM — NÃO foi confirmada empiricamente. Registrada para o inventário ML-0B.

**Falso positivo**: O pypi channel instala o binário Go diretamente em `scripts/trackfw[.exe]` —
sem wrapper Python. O stdin é preservado pela execução direta. Não há risco aqui.

**Gate correto**: AC5 deve incluir dois braços que DETECTAM stdin drop — sem eles, um canal que
silencia stdin passa no smoke. Os dois braços abaixo foram medidos sem projeto configurado:

| Braço | Comando | Stdin preservado | Stdin dropado |
|---|---|---|---|
| Deny | `echo 'git branch some-branch' \| guard git-branch` | rc=2 ✓ | rc=0 — detecta o drop |
| Allow | `echo '{"tool_name":"Write","tool_input":{"file_path":"/tmp/x"},"cwd":"/tmp"}' \| guard write-scope` | rc=0 ✓ | rc=2 — detecta o drop |

Medição de referência (`trackfw` compilado de main, macOS, sem trackfw.yaml):
- `echo 'git branch some-branch' | trackfw guard git-branch` → `RC=2` (bloqueia raw branch)
- `echo '{"tool_name":"Write","tool_input":{"file_path":"/tmp/x"},"cwd":"/tmp"}' | trackfw guard write-scope` → `RC=0` (permite write fora de scope configurado)

O smoke usa esses dois braços no runner Windows (npm shim, pwsh/cmd); se o canal dropa stdin, o
braço de deny vira rc=0 (falso allow) e o de allow vira rc=2 (falso deny) — ambos reprovam.

---

## 4 — Residual declarado

Este Wave 0 aceita não cobrir os seguintes pontos — os demais são in scope e estão nos requisitos
de implementação (§5):

**R1 — cmd.exe com chaining de processos (`cmd /c "... && ...`)**:
O comportamento de propagação de exit code em cadeias cmd.exe longas com `&&`/`||` em processos
aninhados foi documentado como hipótese ML-0D arm 8 na nota de memória
`powershell_command_converts_exit2_to_1`. Exige medição na VM Windows. Aceito como residual;
não bloqueia esta REQ, mas deve ser enumerado na tabela do inventário.

**R2 — stdin fail-open para git-branch e credential em vazio é cross-platform**:
Medido neste ML-0A: `git-branch </dev/null` → rc=0; `credential </dev/null` → rc=0. Esta
assimetria é pré-existente e cross-platform (mesmo comportamento no macOS). O que esta REQ deve
cobrir: verificar que os canais Windows (npm shim, pypi, install.ps1) propagam stdin corretamente.
Se um canal não propaga, o fail-open é um defeito Windows-específico. Essa verificação entra como
REQ-IMP-3 abaixo.

---

## 5 — Requisitos que os MLs de implementação devem cumprir

Derivados dos achados acima; complementam os ACs da REQ:

**REQ-IMP-1 (derivado de T1)**: O gate AC3 não pode ser baseado em grep por texto livre
(`// classification: (a)`). Dois limites desta abordagem:
(1) Um comentário é copiável sem prova — o grep sempre passa.
(2) Um grep por `runtime.GOOS == "windows"` antes de `t.Skip` não detecta o padrão
    `guard_line_shell_test.go:2315` que faz `t.Log(...)` seguido de `return` — HIGH finding em §1.1b
    que é funcionalmente um skip mas nunca gera a string `t.Skip`.

O gate correto deve ser **AST-based** (não grep), keyed pela função de teste envolvente: o checker
percorre `_test.go`, encontra funções de teste que contêm qualquer `runtime.GOOS == "windows"`
(ou `effectiveGOOS()`, `testGOOS`, `CurrentGOOS` ou helpers de skip customizados) em uma branch
que pode retornar sem assertions, e exige que essa função de teste tenha entrada correspondente em
um **arquivo de registro separado** (distinto de `windows-known-failures.json`, que é ratchet de
nomes de testes que rodam e falham — misturar skips quebraria o ratchet).

O registro de skips deve ter campos: `test` (nome exato), `classification` (`"a"` | `"b"` | `"c"`),
`reason` (causa medida), `evidence` (CI run URL, arquivo:linha ou nota de vault). Build-tags
`!windows` também requerem entrada com `classification: "b"` e evidência de não-portabilidade.

O gate deve passar para todos os skips existentes antes de qualquer PR de implementação, o que
implica que ML-1A e ML-1B devem classificar e registrar cada skip/return como pré-condição.

**REQ-IMP-2 (derivado de T2 e T3)**: ML-1B deve exigir evidência de execução medida no CI
(`windows-full-suites`) para cada teste: PASS, FAIL (c) ou known-failure com log de CI citado.
Adicionar ao `windows-known-failures.json` sem execução no Windows não satisfaz AC2.

**REQ-IMP-3 (derivado de T3 e Superfície G)**: O smoke de AC5 deve incluir, além de
`trackfw --version` e `trackfw validate`, os dois braços de stdin que detectam drop (medidos
sem projeto configurado):
(a) `echo 'git branch some-branch' | trackfw guard git-branch` → rc=2 (drop: rc=0)
(b) `echo '{"tool_name":"Write","tool_input":{"file_path":"/tmp/x"},"cwd":"/tmp"}' | trackfw guard write-scope` → rc=0 (drop: rc=2)
Executados na mesma sessão PS/cmd do canal sendo testado. R2 do §4 referencia este requisito.

**REQ-IMP-4 (derivado de T4 e Superfície C)**: AC6 deve ter dois braços distintos por versão PS:
(a) braço `-Command`: PS 5.1 e pwsh 7, todos os 7 estados, deny/allow/absent;
(b) braço "perfil": pwsh 7 com `$PROFILE` real no `windows-latest`, todos os 7 estados,
    assertion por `stdin_bytes == len(payload)` (não só `os.Stat(stdin.out)` — ver Superfície C);
(c) PS 5.1 no braço "perfil": OBRIGATÓRIO — `windows-latest` inclui `powershell.exe` 5.1;
    o mecanismo de perfil PS 5.1 é `$env:USERPROFILE\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1`.

**REQ-IMP-5 (derivado de T5 e Superfície E)**: AC4 deve distinguir dois tipos de artefato:
(i) conveniência (`attention-*.sh`, `trackfw-validate.sh`, lefthook runner sh, husky) — aceito
declarar via `validate`/`doctor` que não executa sem Git Bash;
(ii) guards (`credential`, `git-branch`, `write-scope`) — devem executar; declaração não satisfaz.
O ML de portabilidade deve verificar que os guards do Kiro e Amazon Q (forma cmd.exe) executam
sob `cmd.exe` no `windows-latest`.

**REQ-IMP-6 (derivado de Superfície D)**: Adicionar ao `windows-full-suites` um teste que execute
a forma cmd.exe (`trackfw guard git-branch || exit 2`) sob `cmd.exe` real com payload de deny e
allow. Este teste não pode usar stub `#!/bin/sh`.

**REQ-IMP-7 (derivado de §1.8 e §4 R1 retirado)**: Adicionar job `windows-arm64` ao
`quality.yml` usando o label `windows-11-arm` (runner GA para repositórios públicos desde
2025-08-07: https://github.blog/changelog/2025-08-07-arm64-hosted-runners-for-public-repositories-are-now-generally-available/).
Verificar visibilidade do repo antes de usar: `gh repo view --json visibility` — o runner
`windows-11-arm` falha em repositórios privados. O job mínimo usa os dois braços de REQ-IMP-3:
`echo 'git branch some-branch' | trackfw guard git-branch` → rc=2 e
`echo '{"tool_name":"Write","tool_input":{"file_path":"/tmp/x"},"cwd":"/tmp"}' | trackfw guard write-scope` → rc=0.

**REQ-IMP-8 (derivado de Superfície E e §1.3)**: Estender `OSCLIHomeResolver.HomeFor()` para os 6
CLIs com global hooks que não têm tratamento Windows: Codex, Gemini, Cursor, Copilot, Windsurf,
Kiro. Amazon Q NÃO precisa de entrada — não tem global hooks. Para cada CLI, a evidência deve ser
medida (não inferida): o HOME real que o CLI usa no Windows deve ser capturado via VM ou
documentação primária lida. CLIs sem evidência ficam como INFERIDO com dívida documentada no
ADR-2026-10-14.

**REQ-IMP-9 (derivado de §1.4)**: Adicionar ao `validate` e ao `doctor` uma regra que detecta
ausência de `sh` no PATH e emite aviso (não erro) com o texto do `shMissingMsg`. Esta é uma
verificação proativa: o consumidor não precisa rodar `barrier` para saber que `sh` está faltando.
