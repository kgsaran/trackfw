---
title: "trackfw no PATH dos shells do Windows por canal de instalacao"
date: 2026-10-04
author: ares-tf
status: medido
roadmap: ROADMAP-2026-09-22-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md
ml: ML-0B
---

# trackfw no PATH dos shells do Windows por canal — ML-0B

## Vereditos (no topo)

| Questao | Veredito |
|---------|---------|
| Shim do npm sob Restricted no PowerShell bloqueia? | **SIM — quando npm esta primeiro no PATH** |
| Shim do npm sob Restricted no PowerShell bloqueia? (pip primeiro no PATH) | NAO — resolve .exe da pip |
| Binario sem `guard` (9.2.0): exit code por shell | **1 em todos os shells** (falha aberta) |
| pwsh (PowerShell 7) instalado na VM? | **NAO** |
| jq no Git Bash? | **NAO** |

## VM e ambiente

- **Maquina:** UTM ARM64 (Windows 11), `ssh Lab@192.168.64.6`
- **Data:** 2026-10-04
- **Canais presentes antes da medicao:** pip `8.0.0-rc2` (Python312-arm64), npm `8.0.0-rc2`
- **Canais apos atualizacao:** pip `9.2.0` (atualizado via `pip install --upgrade trackfw==9.2.0`), npm `9.2.0` (instalado via `cmd /c npm install -g trackfw@9.2.0`, pois `npm.ps1` e bloqueado pelo Restricted), GitHub `.exe` `9.2.0` (baixado de `v9.2.0/trackfw_9.2.0_windows_arm64.tar.gz`, copiado para `C:\Users\Lab\trackfw-github-9.2.0.exe`)
- **Observacao:** `C:\Users\Lab\bin\trackfw` contem um binario ARM64 antigo (`8.0.0-rc2`, `2026-09-16`); nao desinstalado pois o enunciado nao pediu; ele e o responsavel pelo `8.0.0-rc2` no Git Bash (ver secao 4).

---

## Secao 1 — ExecutionPolicy

**Comando:** `powershell.exe -NoProfile -Command "Get-ExecutionPolicy -List"` via SSH

```
Scope            ExecutionPolicy
-----            ---------------
MachinePolicy    Undefined
UserPolicy       Undefined
Process          Undefined
CurrentUser      Undefined
LocalMachine     Undefined
```

**Effective (confirmado via cmd parent):**
`cmd /c powershell.exe -NoProfile -Command "Get-ExecutionPolicy"` → `Restricted`

**Nota importante — heranca de Bypass:** quando `measure.ps1` foi executado com `-ExecutionPolicy Bypass`, os processos filhos lancados via `System.Diagnostics.ProcessStartInfo` herdaram `$env:PSExecutionPolicyPreference=Bypass` pelo ambiente do processo. O resultado S6 da medicao de script (exit:0 para `.ps1`) e INVALIDO por essa razao. Os resultados validos sao os medidos com `cmd` como processo pai (sem heranca de Bypass).

---

## Secao 2 — Shims npm gerados

**Comando:** `cmd /c dir C:\Users\Lab\AppData\Roaming\npm\trackfw*`

```
04/10/2026  17:21               413 trackfw        (sem extensao, shebang #!/usr/bin/env node)
04/10/2026  17:21               337 trackfw.cmd    (cmd shim -> node .../bin/trackfw.js)
04/10/2026  17:21               853 trackfw.ps1    (PS shim -> node .../bin/trackfw.js)
```

**Conteudo relevante do `trackfw.ps1`:**
- Shebang: `#!/usr/bin/env pwsh`
- Localiza `node.exe` relativo a `$basedir` e invoca `node_modules/trackfw/bin/trackfw.js`
- Requer PowerShell 6+ (`pwsh`) pelo shebang, mas executavel por `powershell.exe` 5.x via `-File`
- Sob `Restricted`, e **bloqueado** pelo PSSecurityException em qualquer versao do PS

---

## Secao 3 — Tabela canal x shell: `trackfw --version`

**PATH do Windows (ordem relevante):**
`...\Python312-arm64\Scripts` (pip) → `...\AppData\Roaming\npm` (npm, ULTIMO)

| Canal | Shell | Arquivo resolvido | stdout | stderr | exit |
|-------|-------|-------------------|--------|--------|------|
| pip | `powershell.exe -NoProfile -Command` (Restricted, pip primeiro) | `...\Python312-arm64\Scripts\trackfw.exe` (Application) | `trackfw 9.2.0` | — | 0 |
| pip | `cmd /c trackfw --version` | `...\Python312-arm64\Scripts\trackfw.exe` | `trackfw 9.2.0` | — | 0 |
| pip | Git Bash `bash -lc` | N/A (ver nota) | N/A | N/A | N/A |
| npm | `cmd /c "...trackfw.cmd --version"` (direto) | `...\AppData\Roaming\npm\trackfw.cmd` | `trackfw 9.2.0` | — | 0 |
| npm .ps1 | `powershell.exe -NoProfile -File ...trackfw.ps1 --version` (cmd pai, Restricted) | `...\AppData\Roaming\npm\trackfw.ps1` | — | `O arquivo ... nao pode ser carregado porque a execucao de scripts foi desabilitada` (PSSecurityException) | 1 |
| npm (PATH first) | `powershell.exe -NoProfile -Command trackfw --version` (npm primeiro no PATH) | `...\AppData\Roaming\npm\trackfw.ps1` (ExternalScript) | — | `O arquivo ... nao pode ser carregado...` | 1 |
| npm (PATH second) | `powershell.exe -NoProfile -Command trackfw --version` (pip primeiro no PATH) | `...\Python312-arm64\Scripts\trackfw.exe` (Application) | `trackfw 9.2.0` | — | 0 |
| GitHub exe | direto (`.\trackfw-github-9.2.0.exe --version`) | `C:\Users\Lab\trackfw-github-9.2.0.exe` | `trackfw 9.2.0` | — | 0 |
| Git Bash | `bash -lc "trackfw --version"` | `/c/Users/Lab/bin/trackfw` (ARM64 exe, 8.0.0-rc2, instalacao antiga) | `trackfw 8.0.0-rc2` | — | 0 |
| pwsh | N/A | N/A — pwsh nao esta instalado na VM | N/A | N/A | N/A |

**Comandos exatos usados para a tabela:**
- `ssh Lab@192.168.64.6 "cmd /c \"powershell.exe -NoProfile -Command \"\"(Get-Command trackfw).Source; (Get-Command trackfw).CommandType\"\"\""` → `C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\trackfw.exe / Application`
- `ssh Lab@192.168.64.6 "cmd /c \"set PATH=C:\Users\Lab\AppData\Roaming\npm;%PATH% && powershell.exe -NoProfile -Command \"\"(Get-Command trackfw).Source; (Get-Command trackfw).CommandType; trackfw --version 2>&1\"\"\""` → bloqueado com PSSecurityException, exit 1
- `ssh Lab@192.168.64.6 "\"C:\\Program Files\\Git\\bin\\bash.exe\" -lc \"which trackfw; trackfw --version\""` → `/c/Users/Lab/bin/trackfw / trackfw 8.0.0-rc2`

---

## Secao 4 — Git Bash: PATH e resolucao

**PATH no Git Bash (primeiros diretorios relevantes):**
```
/c/Users/Lab/bin                                                  ← PRIMEIRO (contem trackfw 8.0.0-rc2)
/c/WINDOWS/system32
...
/c/Users/Lab/AppData/Local/Programs/Python/Python312-arm64/Scripts  ← pip (9.2.0)
/c/Users/Lab/AppData/Roaming/npm                                    ← npm (9.2.0)
```

O Git Bash prefixa `/c/Users/Lab/bin` ao PATH. Esse diretorio contem um binario ARM64 `trackfw` antigo (`8.0.0-rc2`, `PE32+ executable for MS Windows 6.01 (console), ARM64`, `2026-09-16 15:41`), instalado por uma instalacao anterior do GitHub release. Ele nao foi atualizado por `pip install` nem por `npm install -g`.

**Consequencia:** em Git Bash, `which trackfw` resolve para o binario antigo, nao para o canal pip ou npm atualizado. Isso e independente dos canais atualizados na sessao atual.

---

## Secao 5 — jq no Git Bash

**Comando:** `ssh Lab@192.168.64.6 "\"C:\\Program Files\\Git\\bin\\bash.exe\" -lc \"command -v jq; echo jq_exit=$?\""` 

Resultado: `jq not found` (exit 1). O Git for Windows (2.x) nao inclui `jq` na instalacao padrao.

---

## Secao 6 — trackfw-git-branch-guard.sh no Git Bash (sem jq)

**Projeto temporario criado:** `C:\Users\Lab\tmp-guard-test\` com `trackfw.yaml` minimo (req_dir/roadmap_dir). Script copiado por scp do Mac.

**Comando:** `bash run-guard2.sh` no Git Bash

**Teste com `{"tool_input":{"command":"git push origin main"}}` (sem jq):**
```
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"trackfw: git push bruto bloqueado. ..."}}
trackfw: git push bruto bloqueado. Use `trackfw push` ...
rc=2
```

**Teste com comando benigno `ls -la` (sem jq):**
```
rc=0
```

**Conclusao:** o script usa o fallback `awk` quando `jq` esta ausente, e funciona corretamente: bloqueia `git push` (exit 2) e libera comandos inocuos (exit 0). O fallback da ADR-2026-10-02 esta operacional no Git Bash sem jq.

**Teste com jq:** nao realizado (jq ausente). Para testar com jq seria necessario instalar manualmente; o enunciado pede apenas registrar o que acontece sem jq.

---

## Secao 7 — Falha aberta: `trackfw guard git-branch` com binario 9.2.0 (sem subcomando `guard`)

**Binarios testados:** pip `9.2.0`, GitHub exe `9.2.0`, npm `.cmd` `9.2.0`, Git Bash (8.0.0-rc2)

| Shell | Binario | stdin | stdout | stderr | exit |
|-------|---------|-------|--------|--------|------|
| cmd /c | pip 9.2.0 | `{}` | — | `Error: unknown command "guard" for "trackfw"\nRun 'trackfw --help' for usage.` | 1 |
| direto (PS, via System.Diagnostics.Process) | GitHub 9.2.0 | `{}` | — | `Error: unknown command "guard" for "trackfw"\nRun 'trackfw --help' for usage.` | 1 |
| cmd /c via .cmd | npm 9.2.0 | `{}` | — | `Error: unknown command "guard" for "trackfw"\nRun 'trackfw --help' for usage.` | 1 |
| Git Bash | bin/ 8.0.0-rc2 | `{}` | — | `Error: unknown command "guard" for "trackfw"\nRun 'trackfw --help' for usage.` | 1 |

**Conclusao:** exit code **1** em todos os casos (cobra command padrao para subcomando desconhecido). Isso e FALHA ABERTA: CLIs de agente interpretam qualquer exit code != 2 como "permitir". O D5 da ADR-2026-10-04 esta confirmado como risco real.

**Nota:** o mesmo resultado ocorre com o `guard credential` (nao medido separadamente, mesmo mecanismo).

---

## Secao 8 — Custo de startup por canal (20 execucoes, mediana)

**Metodo:** `Measure-Command { & <exe> --version | Out-Null }` no PowerShell (com `-ExecutionPolicy Bypass` para executar o script de medicao).

**Nota de locale:** os valores no output original usam virgula como separador decimal (locale PT-BR). A tabela abaixo usa ponto.

| Canal | Mediana (ms) | Notas |
|-------|-------------|-------|
| pip (`trackfw.exe` Go puro, via Python Scripts) | **20.6** | `pip median ms:20.6335` |
| GitHub exe (`trackfw.exe` Go puro) | **20.8** | `github median ms:20.7608` |
| npm via `trackfw.cmd` | **137.9** | `npm median ms:137.8945`; inclui cmd overhead + Node startup + shim Node que executa o Go binary |

**Comentario:** o overhead do canal npm e ~117ms por chamada de ferramenta. Para um hook que roda a cada uso de ferramenta pelos CLIs de agente, isso e significativo. O pip e o GitHub exe tem startup identico (~20ms, tipico para Go ARM64).

**Primeiro run (warmup):** pip `32.4ms` / npm `136.0ms` / github `28.5ms` — o primeiro run e levemente mais lento que a mediana, mas nao dramaticamente.

---

## Secao 9 — Linha de base POSIX (macOS)

**Commit:** `d53ebfe61e4efa70c8c145a156a8d3782443e001`  
**Branch:** `feat/hooks-de-guard-executam-no-windows`

**Comando 1:** `go test ./internal/generators/ -run 'Guard|Credential' -count=1`
```
ok  	github.com/kgsaran/trackfw/internal/generators	24.014s
```
Exit: 0

**Comando 2:** `go test ./internal/commands/ -count=1`
```
ok  	github.com/kgsaran/trackfw/internal/commands	16.170s
```
Exit: 0

Ambos passam. Esta e a linha de base do AC5 para o ML-3A comparar apos a implementacao.

---

## Criterios de aceite — conferencia

- [x] Tabela canal x shell com exit code e arquivo resolvido, medida e nao inferida
- [x] Veredito explicito: "o shim do npm sob Restricted no PowerShell bloqueia: SIM (quando npm e o primeiro no PATH)"
- [x] Exit code do binario velho por shell: exit 1 em todos os shells
- [x] Nenhum arquivo do repositorio alterado alem deste documento e `docs/agents-working-context.md`

---

## Alteracoes na VM

- `pip install --upgrade trackfw==9.2.0` → pip channel atualizado de `8.0.0-rc2` para `9.2.0`
- `cmd /c npm install -g trackfw@9.2.0` → npm channel atualizado de `8.0.0-rc2` para `9.2.0`
- `C:\Users\Lab\trackfw-github-9.2.0.exe` copiado (binario novo, nao instalado em PATH)
- `C:\Users\Lab\tmp-guard-test\` criado com `trackfw.yaml` minimo e `trackfw-git-branch-guard.sh`
- `C:\Users\Lab\measure.ps1`, `measure2.ps1`, `measure3.ps1`, `run-guard.sh`, `run-guard2.sh` copiados (scripts de medicao temporarios)
- `C:\Users\Lab\bin\trackfw` (8.0.0-rc2) NAO alterado — binario antigo permanece

---

## Observacoes que contradizem ou complementam a ADR-2026-10-04

### D5 risco 1 — Shim do npm no PowerShell (Restricted): confirmado, com nuance

A ADR diz: "Se o PowerShell resolver o `.ps1` sob Restricted, o hook falha."

**Medido:**
- Com npm PRIMEIRO no PATH: PowerShell resolve `trackfw.ps1` (ExternalScript) → BLOQUEADO, exit 1, `PSSecurityException: UnauthorizedAccess`. A afirmacao da ADR e VERDADEIRA.
- Com pip PRIMEIRO no PATH (como nesta VM): PowerShell resolve `trackfw.exe` (Application) → OK. O bloqueio e PATH-order-dependente.

**Nuance nao antecipada:** em um ambiente com apenas o canal npm instalado (sem pip), o PATH do usuario teria apenas `C:\Users\<User>\AppData\Roaming\npm`, e o PowerShell resolveria `trackfw.ps1` → bloqueado. Em um ambiente misto (pip + npm), a ordem do PATH determina o comportamento. Esta VM tem pip antes de npm, o que mascara o bloqueio.

**Para o desenho futuro:** o `trackfw guard git-branch` como binario Go (sem `.ps1`) resolve o bloqueio para todos os canais que entregam `.exe`, incluindo pip e GitHub. O canal npm continua exposto ao bloqueio enquanto depender do shim `.ps1` para a resolucao no PowerShell.

### D5 risco 2 — Binario sem `guard`: falha aberta confirmada

Exit code 1 (nao 2) em todos os shells e canais. A ADR esta correta ao classificar isso como risco.

### D5 risco 3 — `trackfw` ausente do PATH do processo do CLI de agente

Git Bash tem `/c/Users/Lab/bin` primeiro, com um binario antigo. O PATH do shell hospede do CLI de agente pode divergir do PATH do Windows padrao. Nao medido o PATH dos CLIs de agente reais.

### Surpresa — heranca de PSExecutionPolicyPreference

Quando PowerShell e iniciado com `-ExecutionPolicy Bypass`, ele exporta `$env:PSExecutionPolicyPreference=Bypass`. Processos filhos PowerShell lancados via `System.Diagnostics.ProcessStartInfo` (sem `UseShellExecute`) herdam essa variavel de ambiente e efetivamente rodam sob Bypass, mascarando o bloqueio do Restricted. Testes de Restricted devem usar um processo pai sem Bypass (ex: cmd.exe).

### Surpresa — Git Bash tem binario antigo em ~/bin

O Git Bash prepende `/c/Users/Lab/bin` ao PATH do Windows, e esse diretorio contem um `trackfw` `8.0.0-rc2` de setembro. Esse binario NAO e atualizado por `pip install` nem `npm install -g`. Um deployment que atualiza apenas os canais pip/npm deixa o Git Bash com versao desatualizada, a menos que `C:\Users\Lab\bin` seja limpo.

---

## ML-1D — Medicoes residuais

**Data:** 2026-10-04 | **Squad:** ares-tf | **VM:** UTM ARM64 (Windows 11, PS 5.1.26100, cmd.exe)

### 1. Mark-of-the-Web do shim do npm

**Comando:**
```powershell
Get-Item 'C:\Users\Lab\AppData\Roaming\npm\trackfw.ps1' -Stream Zone.Identifier -ErrorAction Stop
```

**Saida literal:**
```
ZONE_STREAM_NOT_FOUND: Não foi possível abrir fluxo de dados alternados 'Zone.Identifier' do arquivo
'C:\Users\Lab\AppData\Roaming\npm\trackfw.ps1'.
```

**Veredito:** O shim `trackfw.ps1` gerado por `npm install -g` NAO tem Mark-of-the-Web (sem stream Zone.Identifier). Como `RemoteSigned` trata arquivos sem ZoneId como locais (nao baixados da internet), o shim **roda** sob RemoteSigned.

---

**Teste RemoteSigned — cmd como pai (sem heranca de PSExecutionPolicyPreference=Bypass):**

Arquivo `m1b_exact.cmd` (linha a linha; `%ERRORLEVEL%` nunca na mesma linha com `&`):
```cmd
powershell.exe -NoProfile -ExecutionPolicy RemoteSigned -Command "$env:PATH = '%PROBEPATH%;' + $env:PATH; probe; exit $LASTEXITCODE"
set REMOTESIGNED_EXIT=%ERRORLEVEL%
echo REMOTESIGNED_EXIT: %REMOTESIGNED_EXIT%
```

**Saida:**
```
ARGS_JSON: ["C:\\Users\\Lab\\probe-shim\\probe.exe"]
STDIN_FIRST8_HEX: []
STDIN_TOTAL_BYTES: 0
STDIN_EOF_ELAPSED_MS: 0
REMOTESIGNED_EXIT: 2
```

**Veredito:** Sob RemoteSigned (sem MoTW no shim), `probe.ps1` executa e o exit code 2 e propagado corretamente via `exit $LASTEXITCODE`. O canal npm e viavel com `RemoteSigned`.

---

**Teste Restricted — cmd como pai:**

```cmd
powershell.exe -NoProfile -Command "$env:PATH = '%PROBEPATH%;' + $env:PATH; probe; exit $LASTEXITCODE"
set RESTRICTED_EXIT=%ERRORLEVEL%
echo RESTRICTED_EXIT: %RESTRICTED_EXIT%
```

**Saida:**
```
probe : O arquivo C:\Users\Lab\probe-shim\probe.ps1 não pode ser carregado porque a execução de scripts foi
desabilitada neste sistema.
    + FullyQualifiedErrorId : UnauthorizedAccess
RESTRICTED_EXIT: 0
```

**Veredito:** Sob Restricted, `probe.ps1` e bloqueado (PSSecurityException). Mas `RESTRICTED_EXIT: 0` — **FALHA ABERTA**. Motivo: a excecao nao atualiza `$LASTEXITCODE` (que permanece 0 do estado anterior); a instrucao `exit $LASTEXITCODE` a seguir executa normalmente e retorna 0. Isto contrasta com o resultado do ML-0B (sem o sufixo `; exit $LASTEXITCODE`, a excecao resulta em exit 1). O sufixo **piora** o comportamento sob Restricted: de exit 1 (falha fechada) para exit 0 (falha aberta). Impacto para ML-2A: CLIs que usam PS como shell de hook e recebem a linha `trackfw guard git-branch; exit $LASTEXITCODE` — se `trackfw` resolver para `.ps1` (npm, Restricted) — sairao com exit 0 ao inves de exit 1.

---

### 2. argv recebido por `probe.exe` em `cmd /c "probe.exe git-branch; exit $LASTEXITCODE"`

Arquivo `m2_argv.cmd`:
```cmd
cmd /c "C:\Users\Lab\probe.exe git-branch; exit $LASTEXITCODE" < nul
set M2_EXIT=%ERRORLEVEL%
echo M2_EXIT: %M2_EXIT%
```

**Saida literal:**
```
ARGS_JSON: ["C:\\Users\\Lab\\probe.exe","git-branch;","exit","$LASTEXITCODE"]
STDIN_FIRST8_HEX: []
STDIN_TOTAL_BYTES: 0
STDIN_EOF_ELAPSED_MS: 0
M2_EXIT: 2
```

**Baseline (sem ponto-e-virgula):** `cmd /c C:\Users\Lab\probe.exe git-branch` → `ARGS_JSON: ["C:\\Users\\Lab\\probe.exe","git-branch"]`, `M2B_EXIT: 2`.

**Veredito:** Em `cmd.exe`, `;` NAO e separador de comandos — e um caractere literal. A linha `"probe.exe git-branch; exit $LASTEXITCODE"` passa os tokens `"git-branch;"`, `"exit"`, `"$LASTEXITCODE"` como argumentos ao binario. Se a linha de hook `trackfw guard git-branch; exit $LASTEXITCODE` for executada diretamente por `cmd.exe` (Kiro, Amazon Q), o guard recebera esses tres argumentos extras → cobra nao reconhece → help → exit 0 → **falha aberta**. CONFIRMA (por medicao primaria na VM) a hipotese do ML-0B e do working context: o sufixo `; exit $LASTEXITCODE` NAO deve ser emitido para CLIs que usam `cmd.exe` como executor de hook.

---

### 3. stdin vindo do PowerShell — encoding, BOM e EOF

Todos os testes usam `probe.exe` (exit 2 por padrao via `PROBE_EXIT`).

**3a — Pipe basico (encoding padrao do PS 5.1):**
```powershell
'{"a":1}' | & $probe
```
Saida: `STDIN_FIRST8_HEX: [7B 22 61 22 3A 31 7D 0D]` | `STDIN_TOTAL_BYTES: 9` | `STDIN_EOF_ELAPSED_MS: 0`

Bytes: `{"a":1}` (7 bytes) + `0D 0A` (CRLF). Sem BOM.

**3b — Com `$OutputEncoding=[System.Text.UTF8Encoding]::new($false)` (UTF-8 sem BOM):**
```powershell
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
'{"a":1}' | & $probe
```
Saida: `STDIN_FIRST8_HEX: [7B 22 61 22 3A 31 7D 0D]` | `STDIN_TOTAL_BYTES: 9` | `STDIN_EOF_ELAPSED_MS: 0`

Identico a 3a. Sem BOM.

**3e — Forma exata do Cursor (arquivo escrito sem BOM por Node.js, `Get-Content -Raw` com `$OutputEncoding = [System.Text.Encoding]::UTF8`):**

Arquivo gravado com `[System.IO.File]::WriteAllText(path, '{"a":1}', UTF8NoBom)`:
```
FILE_BYTES_HEX: [7B 22 61 22 3A 31 7D]   ← sem BOM no arquivo
```

Script Cursor executado:
```powershell
$OutputEncoding = [System.Text.Encoding]::UTF8
Get-Content -LiteralPath $tmpFile -Raw | & { $input | & $probe }
```

Saida: `STDIN_FIRST8_HEX: [EF BB BF 7B 22 61 22 3A]` | `STDIN_TOTAL_BYTES: 12` | `STDIN_EOF_ELAPSED_MS: 0`

**BOM UTF-8 (EF BB BF) presente, mesmo com arquivo sem BOM.** Total: BOM(3) + `{"a":1}`(7) + CRLF(2) = 12 bytes.

**Motivo:** `[System.Text.Encoding]::UTF8` em .NET e a variante UTF-8 COM BOM. Ao serializar a string obtida do `Get-Content -Raw` para o pipe externo, o PS prefixa o BOM. O `[System.Text.UTF8Encoding]::new($false)` (sem BOM) nao causa este problema — mas o Cursor usa a propriedade estatica `.UTF8` que tem BOM.

**Veredito 3:** O Cursor injeta BOM UTF-8 (EF BB BF) no stdin do guard, independentemente do conteudo do arquivo temporario, porque `$OutputEncoding = [System.Text.Encoding]::UTF8` e a variante com BOM. O guard Go **deve descartar o BOM** (se presente) antes de decodificar JSON. EOF chega em <1ms em todos os casos — sem risco de timeout no lado do PS.

Contexto arquiteto (nao refazer): PS 5.1.26100 — `powershell -NoProfile -Command "cmd /c exit 2"` retorna exit 1; `"cmd /c exit 2; exit $LASTEXITCODE"` retorna exit 2 (LASTEXITCODE e atualizado por processos nativos externos, confirmando que o sufixo propaga corretamente para executaveis — o problema do item 1 e especifico a excecoes PS, nao a processos externos).

---

### Resumo dos tres vereditos

| Item | Comando chave | Veredito |
|------|--------------|---------|
| 1. MoTW + RemoteSigned | `Get-Item ... -Stream Zone.Identifier` | Sem MoTW: `.ps1` roda sob RemoteSigned (exit 2 propagado). Sob Restricted com sufixo `; exit $LASTEXITCODE`: **falha aberta** (exit 0, nao 1) |
| 2. argv em cmd | `cmd /c "probe.exe git-branch; exit $LASTEXITCODE"` | `;` e literal: `os.Args = ["probe.exe","git-branch;","exit","$LASTEXITCODE"]` — confirmacao primaria: sufixo proibido para Kiro/Amazon Q |
| 3. stdin BOM/EOF | Forma exata do Cursor | **BOM UTF-8 (EF BB BF) sempre presente** via `[System.Text.Encoding]::UTF8`; EOF <1ms |

### O que muda para ML-2A e ML-2B

- **ML-2A (emissao):** confirma que a linha de hook para Kiro e Amazon Q deve ser `trackfw guard <nome>` **sem** `; exit $LASTEXITCODE`. Para CLIs PS (Claude Code, Codex, Gemini, Cursor, Copilot, Windsurf), o sufixo e necessario para propagar o exit de processos externos — mas nao resolve o bloqueio do `.ps1` sob Restricted (que ja era conhecido). RemoteSigned mitiga o canal npm (sem MoTW = roda).
- **ML-1A (guard Go):** deve descartar BOM UTF-8 (EF BB BF) no inicio do stdin antes de decodificar JSON — obrigatorio para o Cursor. Sem este tratamento, `json.Unmarshal` falha com "invalid character" no primeiro byte e o guard nega (falha fechada — erro na direcao segura, mas rejeita comandos legitimos do Cursor).
