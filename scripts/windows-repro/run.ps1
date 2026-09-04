# run.ps1 — suite de reproducao de defeito (camada 2 do instrumento).
#
# ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-
# demanda, ML-1A. Uma verificacao por defeito conhecido da issue #216,
# mapeada 1:1, exercitando o CAMINHO REAL de producao — sem mock (AC2).
#
# NAO CORRIGE nada (ADR decisao 7). Se uma verificacao nao reproduzir o
# defeito esperado, o script reporta ABSENT ou INCONCLUSIVE com a evidencia
# medida — nunca silencia, nunca "conserta" para passar.
#
# Mapeamento completo dos 11 itens da issue #216 (numeracao da tabela do
# ML-0A / Wave 0, hades-tf):
#   1  cp1252 no cli.py (--help de topo)          -> checado aqui (REAL)
#   2  $HOME ignorado nos 3 runtimes                -> checado aqui (REAL)
#   3  bit de execucao sempre "presente" no Windows -> checado aqui
#      (confirmatorio; evidencia primaria = camada 1 / go test ./...)
#   4  gate de cobertura crasha em cp1252            -> checado aqui via
#      mecanismo compartilhado com o item 1 (NAO via o wrapper .sh, para
#      nao confundir com o item 7 no mapeamento)
#   5  CRLF na escrita dos geradores Python          -> checado aqui (REAL,
#      via `trackfw init` de verdade + varredura de bytes). ML-1C: medido
#      com o item 1 (cp1252) neutralizado SO neste subprocesso via
#      PYTHONIOENCODING=utf-8, para nao mascarar o item 5 atras do crash
#      do item 1 — documentado no proprio veredito.
#   6  isatty() mente para NUL no Windows            -> checado aqui (REAL,
#      via `trackfw init` com stdin=NUL, sem monkeypatch). ML-1C: mesma
#      neutralizacao do item 5.
#   7  sh -c hardcodado no Go (barrier.go:729)       -> checado aqui. ML-1C:
#      reclassificado — a pergunta "sh existe?" ja foi respondida (ABSENT);
#      agora compara o VEREDITO do mesmo gate nos 3 runtimes (Go via sh -c
#      POSIX, Node/Python via cmd.exe no Windows).
#   8  postura divergente com \ (manager.go/js)      -> NAO checado aqui.
#      resolve() e nao-exportado em internal/integrations/manager.go — nao
#      da para chamar de fora sem tocar internal/ (fora do escopo desta
#      ML). Exercitar via CLI completo exige fixture de instalacao de
#      integracao, fora do escopo desta ML. Residual declarado — Wave 0 ja
#      recomendou "teste dedicado", nao coberto aqui.
#   9  ref_targets_exist vazio em by_agent           -> NAO checado aqui.
#      Nao e defeito de Windows (reproduz em qualquer SO) — tem REQ propria
#      (ver ML-0A). Fora do escopo desta REQ.
#  10  separador de SO vazando no roadmap move       -> checado aqui (REAL,
#      fixture minima + CLI completo dos 3 runtimes)
#  11  12 testes de symlink sem privilegio            -> NAO checado aqui.
#      Ja e exposto pela camada 1 (go test ./..., npm test, pytest pypi/tests
#      incluem esses arquivos de teste) — Wave 0: "SIM, mas mal-mapeado".
#      O skip explicito com mensagem e a Wave 2 (ML-2A), fora desta ML.

# RUNNER_TEMP so existe dentro do GitHub Actions. Fora dele era $null, e
# `Join-Path $null "x"` devolve STRING VAZIA no PowerShell 5.1 — sem erro. O
# efeito era pior que falhar: o item 2 comparava a saida dos runtimes contra ""
# e, como nunca sao iguais, emitia REPRODUCED **incondicionalmente** — inclusive
# numa arvore onde o defeito ja esta corrigido. Os itens 5, 6 e 10 iam a
# INCONCLUSIVE pelo mesmo caminho.
#
# Medido: `Join-Path $null "item2-fake-HOME"` -> [] (vazio, sem erro).
if (-not $env:RUNNER_TEMP) {
    $env:RUNNER_TEMP = Join-Path ([System.IO.Path]::GetTempPath()) "trackfw-windows-repro"
    New-Item -ItemType Directory -Force -Path $env:RUNNER_TEMP | Out-Null
    Write-Host "RUNNER_TEMP ausente (execucao fora do GitHub Actions) - usando $env:RUNNER_TEMP"
}

$ErrorActionPreference = "Continue"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

$env:TRACKFW_PYPI_SRC = (Join-Path $repoRoot "pypi")

$results = @()

function Add-Result {
    param([string]$Item, [string]$Title, [string]$Verdict, [string]$Detail)
    $script:results += [pscustomobject]@{
        Item     = $Item
        Title    = $Title
        Verdict  = $Verdict
        Detail   = $Detail
    }
    Write-Host ""
    Write-Host "## ITEM $Item — $Title"
    Write-Host $Detail
    Write-Host "RESULT: $Verdict"
}

function Run-Capture {
    param([string]$Exe, [string[]]$ArgList, [string]$WorkDir = $null, [hashtable]$EnvVars = @{})
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Exe
    # ProcessStartInfo.ArgumentList so existe no .NET Core (pwsh 7+). No Windows
    # PowerShell 5.1 (.NET Framework, PSEdition=Desktop) a propriedade e $null, e
    # `$psi.ArgumentList.Add($a)` estoura com "nao e possivel chamar um metodo em
    # uma expressao de valor nulo" — o processo entao roda SEM ARGUMENTO NENHUM e
    # toda medicao vira vazia. Medido: $psi.PSObject.Properties["ArgumentList"]
    # e $null em CLR 4.0.30319.
    #
    # O fallback monta a linha de comando com as regras de aspas do Windows, em
    # vez de interpolar — argumento com espaco ou com aspas passa intacto.
    if ($null -ne $psi.PSObject.Properties["ArgumentList"]) {
        foreach ($a in $ArgList) { $psi.ArgumentList.Add($a) }
    } else {
        $psi.Arguments = ($ArgList | ForEach-Object {
            $s = [string]$_
            if ($s -eq "") { '""' }
            elseif ($s -match '[\s"]') { '"' + ($s -replace '(\*)"', '$1$1\"' -replace '(\+)$', '$1$1') + '"' }
            else { $s }
        }) -join " "
    }
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    if ($WorkDir) { $psi.WorkingDirectory = $WorkDir }
    foreach ($k in $EnvVars.Keys) { $psi.Environment[$k] = $EnvVars[$k] }
    $p = New-Object System.Diagnostics.Process
    $p.StartInfo = $psi
    $p.Start() | Out-Null
    $stdout = $p.StandardOutput.ReadToEnd()
    $stderr = $p.StandardError.ReadToEnd()
    $p.WaitForExit()
    return [pscustomobject]@{ ExitCode = $p.ExitCode; Stdout = $stdout; Stderr = $stderr }
}

# ---------------------------------------------------------------------
# item 1 e item 4 (mecanismo compartilhado) — cp1252
# ---------------------------------------------------------------------
$r1 = Run-Capture -Exe "python" -ArgList @("scripts/windows-repro/python/checks.py", "help")
Add-Result -Item "1" -Title "cp1252 no cli.py --help de topo" `
    -Verdict ($(if ($r1.Stdout -match "VERDICT=REPRODUCED") { "REPRODUCED" } elseif ($r1.Stdout -match "VERDICT=ABSENT") { "ABSENT" } else { "INCONCLUSIVE" })) `
    -Detail $r1.Stdout

$r4 = Run-Capture -Exe "python" -ArgList @("scripts/windows-repro/python/checks.py", "cp1252-print")
Add-Result -Item "4" -Title "gate de cobertura crasha em cp1252 (mecanismo compartilhado c/ item 1, sem o wrapper .sh)" `
    -Verdict ($(if ($r4.Stdout -match "VERDICT=REPRODUCED") { "REPRODUCED" } elseif ($r4.Stdout -match "VERDICT=ABSENT") { "ABSENT" } else { "INCONCLUSIVE" })) `
    -Detail $r4.Stdout

# ---------------------------------------------------------------------
# item 2 — $HOME ignorado nos 3 runtimes
# ---------------------------------------------------------------------
$fakeHome = Join-Path $env:RUNNER_TEMP "item2-fake-HOME"
$fakeProfile = Join-Path $env:RUNNER_TEMP "item2-fake-USERPROFILE"
New-Item -ItemType Directory -Force -Path $fakeHome | Out-Null
New-Item -ItemType Directory -Force -Path $fakeProfile | Out-Null

$goHome = Run-Capture -Exe "go" -ArgList @("run", "scripts/windows-repro/go/checks.go", "home") `
    -EnvVars @{ HOME = $fakeHome; USERPROFILE = $fakeProfile }
$nodeHome = Run-Capture -Exe "node" -ArgList @("-e", "console.log(require('os').homedir())") `
    -EnvVars @{ HOME = $fakeHome; USERPROFILE = $fakeProfile }
$pyHome = Run-Capture -Exe "python" -ArgList @("-c", "import os; print(os.path.expanduser('~'))") `
    -EnvVars @{ HOME = $fakeHome; USERPROFILE = $fakeProfile }

$item2Detail = @"
HOME=$fakeHome (deliberadamente diferente)
USERPROFILE=$fakeProfile (deliberadamente diferente)
Go   os.UserHomeDir()      -> $($goHome.Stdout.Trim())
Node os.homedir()          -> $($nodeHome.Stdout.Trim())
Py   os.path.expanduser(~) -> $($pyHome.Stdout.Trim())
"@
$goIgnoresHome = $goHome.Stdout.Trim() -ne $fakeHome
$nodeIgnoresHome = $nodeHome.Stdout.Trim() -ne $fakeHome
$pyIgnoresHome = $pyHome.Stdout.Trim() -ne $fakeHome
# Guarda de vacuidade: sem as tres saidas nao ha o que comparar, e um
# "REPRODUCED" aqui afirmaria o defeito sem medicao. O item 1 ja reporta
# INCONCLUSIVE quando o processo morre antes do codigo medido; este faz o mesmo.
$item2Medido = $goHome.Stdout.Trim() -and $nodeHome.Stdout.Trim() -and $pyHome.Stdout.Trim() -and $fakeHome
$item2Verdict = if (-not $item2Medido) { "INCONCLUSIVE" }
                elseif ($goIgnoresHome -or $nodeIgnoresHome -or $pyIgnoresHome) { "REPRODUCED" }
                else { "ABSENT" }
Add-Result -Item "2" -Title 'HOME ignorado nos 3 runtimes no Windows' -Verdict $item2Verdict -Detail $item2Detail

# ---------------------------------------------------------------------
# item 3 — bit de execucao (confirmatorio; primario = camada 1)
# ---------------------------------------------------------------------
$r3 = Run-Capture -Exe "go" -ArgList @("run", "scripts/windows-repro/go/checks.go", "execbit")
$item3Verdict = if ($r3.Stdout -match "bit0111=0") { "REPRODUCED" } elseif ($r3.ExitCode -ne 0) { "INCONCLUSIVE" } else { "ABSENT" }
Add-Result -Item "3" -Title "info.Mode()&0111==0 sempre verdadeiro no Windows (confirmatorio)" -Verdict $item3Verdict -Detail $r3.Stdout

# ---------------------------------------------------------------------
# item 5 — CRLF na escrita dos geradores Python
#
# USERPROFILE isolado e PROPRIO (nao compartilhado com o item 6): o guard
# do wizard de identidade em init.py e
# `skip_identity_wizard = preset_changed or _identity_file_exists(home)` —
# se este check e o do item 6 dividissem o mesmo home sintetico, um arquivo
# de identidade deixado por este `init --identity-preset none` faria o
# item 6 pular o wizard por _identity_file_exists()==True, nao por
# preset_changed, mascarando silenciosamente o proprio isatty() que o
# item 6 precisa medir (falso negativo dependente de ordem).
# ---------------------------------------------------------------------
$item5Home = Join-Path $env:RUNNER_TEMP "item5-fake-USERPROFILE"
New-Item -ItemType Directory -Force -Path $item5Home | Out-Null
$r5 = Run-Capture -Exe "python" -ArgList @("scripts/windows-repro/python/checks.py", "crlf") `
    -EnvVars @{ RUNNER_TEMP = $env:RUNNER_TEMP; USERPROFILE = $item5Home; HOME = $item5Home }
$item5Verdict = if ($r5.Stdout -match "VERDICT=REPRODUCED") { "REPRODUCED" } elseif ($r5.Stdout -match "VERDICT=ABSENT") { "ABSENT" } elseif ($r5.Stdout -match "VERDICT=BLOCKED-BY-ITEM-1") { "BLOCKED-BY-ITEM-1" } else { "INCONCLUSIVE" }
Add-Result -Item "5" -Title "geradores Python escrevem CRLF (open sem newline=; ML-1C: medido com item 1 neutralizado via PYTHONIOENCODING=utf-8)" -Verdict $item5Verdict -Detail $r5.Stdout

# ---------------------------------------------------------------------
# item 6 — isatty() mente para NUL
#
# USERPROFILE isolado e PROPRIO, vazio (sem arquivo de identidade) — e
# precondicao da medicao: o check so e valido se _identity_file_exists(home)
# for False, senao skip_identity_wizard vira True por um motivo que nao e
# o que este item mede (ver comentario do item 5 acima).
# ---------------------------------------------------------------------
$item6Home = Join-Path $env:RUNNER_TEMP "item6-fake-USERPROFILE"
New-Item -ItemType Directory -Force -Path $item6Home | Out-Null
$r6 = Run-Capture -Exe "python" -ArgList @("scripts/windows-repro/python/checks.py", "isatty") `
    -EnvVars @{ USERPROFILE = $item6Home; HOME = $item6Home }
$item6Verdict = if ($r6.Stdout -match "VERDICT=REPRODUCED") { "REPRODUCED" } elseif ($r6.Stdout -match "VERDICT=ABSENT") { "ABSENT" } elseif ($r6.Stdout -match "VERDICT=BLOCKED-BY-ITEM-1") { "BLOCKED-BY-ITEM-1" } else { "INCONCLUSIVE" }
Add-Result -Item "6" -Title "sys.stdin.isatty() mente True para NUL (ML-1C: medido com item 1 neutralizado via PYTHONIOENCODING=utf-8)" -Verdict $item6Verdict -Detail $r6.Stdout

# ---------------------------------------------------------------------
# item 7 — sh -c hardcodado (reclassificado, ML-1C)
#
# A pergunta "sh existe no PATH do runner?" ja foi respondida (ABSENT, ML-1A
# via checks.go shc) e essa evidencia auxiliar e mantida abaixo. A pergunta
# que falta e a que a Wave 0 apontou: Go avalia gates via `sh -c` (POSIX,
# barrier.go:729) enquanto Node (spawnSync shell:true, barrier.js:561) e
# Python (subprocess.run shell=True, barrier.py:582) resolvem para cmd.exe
# no Windows — o MESMO texto de `**Gates da wave:**` produz o MESMO
# veredito visivel nos 3? Roda o MESMO literal de comando (aspas simples +
# redirecionamento POSIX /dev/null, que diverge de proposito entre sh e
# cmd.exe) via os 3 primitivos reais e compara o stdout bruto.
# ---------------------------------------------------------------------
$r7shPresence = Run-Capture -Exe "go" -ArgList @("run", "scripts/windows-repro/go/checks.go", "shc")
$r7ShPresenceVerdict = if ($r7shPresence.Stdout -match "sh-not-found") { "ausente" } elseif ($r7shPresence.Stdout -match "sh-ran-ok") { "presente" } else { "indeterminado" }

$r7Go = Run-Capture -Exe "go" -ArgList @("run", "scripts/windows-repro/go/checks.go", "gatequote")
$r7Node = Run-Capture -Exe "node" -ArgList @("scripts/windows-repro/node/checks.js", "gatequote")
$r7Py = Run-Capture -Exe "python" -ArgList @("scripts/windows-repro/python/checks.py", "gatequote")

function Get-GateQuoteToken {
    param([string]$Stdout)
    # Normaliza CRLF/LF (artefato de captura, nao de semantica de shell) e
    # extrai o texto entre os marcadores STDOUT_BEGIN/STDOUT_END.
    $normalized = ($Stdout -replace "`r`n", "`n").Trim()
    if ($normalized -match "(?s)STDOUT_BEGIN\n(.*)\nSTDOUT_END") {
        return $matches[1].Trim()
    }
    return "<sem-STDOUT_BEGIN/END: $normalized>"
}

$goToken = Get-GateQuoteToken -Stdout $r7Go.Stdout
$nodeToken = Get-GateQuoteToken -Stdout $r7Node.Stdout
$pyToken = Get-GateQuoteToken -Stdout $r7Py.Stdout

$item7Detail = @"
Evidencia auxiliar (ML-1A): sh no PATH do runner -> $r7ShPresenceVerdict
$($r7shPresence.Stdout)

Comparacao de veredito do MESMO gate nos 3 runtimes (ML-1C):
Go   (sh -c, POSIX)          -> $goToken
Node (spawnSync shell:true)  -> $nodeToken
Python (subprocess shell=True) -> $pyToken
"@

$item7Verdict = if (($goToken -ne $nodeToken) -or ($goToken -ne $pyToken) -or ($nodeToken -ne $pyToken)) { "REPRODUCED" } else { "ABSENT" }
Add-Result -Item "7" -Title "Go (sh POSIX) vs Node/Python (cmd.exe) avaliam o MESMO gate de wave diferente" -Verdict $item7Verdict -Detail $item7Detail

# ---------------------------------------------------------------------
# item 8 — declarado, nao checado (residual)
# ---------------------------------------------------------------------
Add-Result -Item "8" -Title "postura divergente com \ no destino resolvido (manager.go nao rejeita, manager.js rejeita)" `
    -Verdict "DECLARED-OUT-OF-SCOPE" `
    -Detail "Manager.resolve() em internal/integrations/manager.go e nao-exportado; chama-lo exigiria tocar internal/ (fora do escopo desta ML) ou montar uma fixture completa de instalacao de integracao via CLI (fora do escopo desta ML). Confirmado por leitura direta do codigo (nao medido em runtime): manager.go:672 usa path.Clean (semantica POSIX, nao rejeita backslash); npm/src/integrations/manager.js:48 rejeita explicitamente com destination.includes(chr(92)). Wave 0 ja recomendou teste dedicado — nao e este instrumento."

# ---------------------------------------------------------------------
# item 9 — declarado, fora de escopo (nao e defeito de Windows)
# ---------------------------------------------------------------------
Add-Result -Item "9" -Title "ref_targets_exist vazio em roadmap_namespacing: by_agent" `
    -Verdict "OUT-OF-SCOPE" `
    -Detail "Nao e defeito de Windows — reproduz em qualquer SO (confirmado pelo autor da issue e por ML-0A). Tem REQ propria. Nao e checado por este instrumento."

# ---------------------------------------------------------------------
# item 10 — separador de SO vazando no roadmap move
# ---------------------------------------------------------------------
$trackfwBinPath = Join-Path $env:RUNNER_TEMP "trackfw-item10-bin.exe"
$buildResult = Run-Capture -Exe "go" -ArgList @("build", "-o", $trackfwBinPath, "./cmd/trackfw") -WorkDir $repoRoot.Path
if ($buildResult.ExitCode -ne 0) {
    Write-Host "AVISO: falha ao compilar trackfw para o item 10 (go): $($buildResult.Stderr)"
}

function Test-Item10 {
    param([string]$Runtime)

    $fixture = Join-Path $env:RUNNER_TEMP "item10-$Runtime"
    Remove-Item -Recurse -Force $fixture -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path (Join-Path $fixture "docs\req") | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $fixture "docs\roadmaps\backlog") | Out-Null

    @"
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
"@ | Set-Content -NoNewline -Encoding utf8 (Join-Path $fixture "trackfw.yaml")

    @"
---
status: backlog
date: 2026-08-30
---
# Roadmap: item10 fixture
"@ | Set-Content -NoNewline -Encoding utf8 (Join-Path $fixture "docs\roadmaps\backlog\ROADMAP-item10.md")

    @"
---
status: Open
date: 2026-08-30
roadmap: docs/roadmaps/backlog/ROADMAP-item10.md
---
# REQ: item10 fixture
"@ | Set-Content -NoNewline -Encoding utf8 (Join-Path $fixture "docs\req\REQ-item10.md")

    switch ($Runtime) {
        "go" {
            $r = Run-Capture -Exe $trackfwBinPath -ArgList @("roadmap", "move", "ROADMAP-item10.md", "wip") -WorkDir $fixture
        }
        "node" {
            $r = Run-Capture -Exe "node" -ArgList @((Join-Path $repoRoot "npm\bin\trackfw"), "roadmap", "move", "ROADMAP-item10.md", "wip") -WorkDir $fixture
        }
        "python" {
            $r = Run-Capture -Exe "python" -ArgList @("-c", "from trackfw.cli import main; import sys; sys.argv=['trackfw','roadmap','move','ROADMAP-item10.md','wip']; main()") `
                -WorkDir $fixture -EnvVars @{ PYTHONPATH = $env:TRACKFW_PYPI_SRC }
        }
    }

    $reqPath = Join-Path $fixture "docs\req\REQ-item10.md"
    if (-not (Test-Path $reqPath)) {
        return [pscustomobject]@{ Runtime = $Runtime; Verdict = "INCONCLUSIVE"; Detail = "move exit=$($r.ExitCode) stdout=$($r.Stdout) stderr=$($r.Stderr)" }
    }
    $reqContent = Get-Content -Raw $reqPath
    $roadmapLine = ($reqContent -split "`n" | Where-Object { $_ -match "^roadmap:" })
    $hasBackslash = $roadmapLine -match "\\"
    $verdict = if ($hasBackslash) { "REPRODUCED" } elseif ($r.ExitCode -eq 0) { "ABSENT" } else { "INCONCLUSIVE" }
    return [pscustomobject]@{ Runtime = $Runtime; Verdict = $verdict; Detail = "move exit=$($r.ExitCode); roadmap-line=$roadmapLine" }
}

$item10Go = Test-Item10 -Runtime "go"
$item10Node = Test-Item10 -Runtime "node"
$item10Py = Test-Item10 -Runtime "python"
$item10Detail = ($item10Go, $item10Node, $item10Py | ForEach-Object { "$($_.Runtime): $($_.Verdict) — $($_.Detail)" }) -join "`n"
$item10Verdict = if (@($item10Go, $item10Node, $item10Py) | Where-Object { $_.Verdict -eq "REPRODUCED" }) { "REPRODUCED" } elseif (@($item10Go, $item10Node, $item10Py) | Where-Object { $_.Verdict -eq "INCONCLUSIVE" }) { "INCONCLUSIVE" } else { "ABSENT" }
Add-Result -Item "10" -Title "separador de SO (\) vazando para o frontmatter da REQ no roadmap move" -Verdict $item10Verdict -Detail $item10Detail

# ---------------------------------------------------------------------
# item 11 — declarado, coberto pela camada 1
# ---------------------------------------------------------------------
Add-Result -Item "11" -Title "12 testes de symlink sem privilegio (5 Python, 5 Node, 2 Go)" `
    -Verdict "COVERED-BY-CAMADA-1" `
    -Detail "Ja exposto por go test ./..., npm test e pytest pypi/tests (camada 1, job windows-full-suites) — sao os proprios arquivos de teste da suite. O skip explicito com mensagem nomeando a garantia nao exercitada e a Wave 2 (ML-2A), fora do escopo desta ML."

# ---------------------------------------------------------------------
# item 12 — SONDA OBSERVACIONAL (ML-0B, NAO e da issue #216)
#
# Separa as duas ramificacoes que sobraram da investigacao do grupo B
# (50 testes Python que lancam `bash` e falham com exit 1 e stderr vazio;
# ver docs/qualidade/2026-09-04-grupo-b-bash-do-python-em-windows.md):
#
#   (A) o `bash` que o Python lanca NUNCA executa o script  -> harness
#   (B) o script morre entre `set -euo pipefail` e a guarda -> SEGURANCA
#
# Por que a sonda IMPRIME stdout: das assinaturas testadas, a unica que
# reproduz `rc=1` com `stderr` vazio e "algo saiu 1 falando por stdout" —
# e stdout e exatamente o canal que os 50 testes descartam (`_out`).
# Repetir o que os testes fazem mediria o mesmo nada.
#
# NAO CORRIGE nada. Nenhum teste e tocado.
# ---------------------------------------------------------------------

# 12a — qual `bash` o Windows resolve, e em que ordem (where.exe lista TODAS
# as ocorrencias). Fora do processo Python de proposito: e a ordem do PATH do
# runner, o contraponto ao que o CreateProcess do CPython faz.
$r12where = Run-Capture -Exe "where.exe" -ArgList @("bash")
$r12whereExe = Run-Capture -Exe "where.exe" -ArgList @("bash.exe")

# 12b/12c/12d — corpo Python em ARQUIVO (here-string), nunca em `python -c`
# multilinha. Here-string SINGLE-quoted (@'...'@): o corpo contem `$0`,
# `$BASH_VERSION` e `>&2`, que uma here-string dupla interpolaria como
# variavel de PowerShell (vault: powershell-modo-argumento-nao-interpola-
# nem-divide-2026-08-31).
$item12Probe = Join-Path $env:RUNNER_TEMP "item12-probe.py"
@'
# Sonda ML-0B — ITEM 12. Observacional: mede, nao corrige.
# Todo texto medido sai por ascii(): repr() de str preserva nao-ASCII e
# morreria no stdout cp1252 do runner (item 1 desta mesma suite).
import json
import os
import shutil
import subprocess
import sys
import tempfile

# 2000, nao 400: sob (A) o texto que sai por stdout e a UNICA coisa que NOMEIA
# quem atendeu por 'bash' (ex.: a mensagem do stub do WSL). A mensagem do proprio
# guard ja mede ~380 chars — cortar em 400 arriscaria decapitar a evidencia.
MAX = 2000


def emit(key, value):
    print("ITEM12 " + key + "=" + str(value))


def cut(text):
    return ascii((text or "")[:MAX])


def run(argv, **kwargs):
    try:
        p = subprocess.run(argv, capture_output=True, text=True, **kwargs)
        return {"rc": p.returncode, "out": p.stdout or "", "err": p.stderr or "", "exc": None}
    except Exception as exc:
        return {"rc": None, "out": "", "err": "", "exc": repr(exc)}


# ---- 12a (lado Python) — o que o proprio CPython enxerga -------------
emit("py_executable", ascii(sys.executable))
emit("py_version", ascii(sys.version.replace("\n", " ")))
path_entries = [e for e in os.environ.get("PATH", "").split(os.pathsep) if e]
emit("path_head", ascii(path_entries[:10]))
emit("shutil_which_bash", ascii(shutil.which("bash")))

# shutil.which varre o %PATH% na ordem do PATH — que e PRECISAMENTE a ordem
# que a hipotese (A) diz nao ser a usada pelo CreateProcess com
# lpApplicationName=NULL. Por isso um unico which() NAO serve de braco
# "caminho absoluto": se ele devolver o mesmo binario que o nome nu resolve,
# a comparacao volta "identica" sem provar nada. Enumeramos TODOS os
# candidatos do PATH e exercitamos cada um.
candidates = []
for entry in path_entries:
    for name in ("bash.exe", "bash"):
        cand = os.path.join(entry, name)
        if os.path.isfile(cand) and cand not in candidates:
            candidates.append(cand)
# Locais canonicos fora do %PATH%: no Git for Windows o bash de `usr\bin` NAO
# esta no PATH do runner, e o stub do WSL em System32 esta. Sem semear estes,
# um INCONCLUSIVE diria "nenhum bash do %PATH% funciona" quando a resposta era
# alcancavel — e a ML pede que a sonda nao devolva dado ambiguo.
for extra in (r"C:\Program Files\Git\bin\bash.exe",
              r"C:\Program Files\Git\usr\bin\bash.exe",
              r"C:\Program Files\Git\usr\bin\bash",
              r"C:\Windows\System32\bash.exe"):
    if os.path.isfile(extra) and extra not in candidates:
        candidates.append(extra)
emit("bash_candidates", ascii(candidates))

# ---- 12b — controle minimo: o Python consegue rodar QUALQUER coisa por
# `bash`, pelo NOME NU, do jeito que os 50 testes lancam? stdout IMPRESSO.
PROBE_CMD = "echo PROBE_OUT; echo PROBE_ERR >&2"
bare = run(["bash", "-c", PROBE_CMD])
emit("bare_rc", bare["rc"])
emit("bare_out", cut(bare["out"]))
emit("bare_err", cut(bare["err"]))
emit("bare_exc", bare["exc"])

# 12b-bis — o mesmo, com os dois canais fundidos no MESMO pipe. Se aparecer
# texto aqui que nao apareceu acima, o canal e stdout (assinatura de (A)).
merged = {"rc": None, "out": "", "exc": None}
try:
    p = subprocess.run(["bash", "-c", PROBE_CMD], stdout=subprocess.PIPE,
                       stderr=subprocess.STDOUT, text=True)
    merged["rc"] = p.returncode
    merged["out"] = p.stdout or ""
except Exception as exc:
    merged["exc"] = repr(exc)
emit("bare_merged_rc", merged["rc"])
emit("bare_merged_out", cut(merged["out"]))
emit("bare_merged_exc", merged["exc"])

# 12b-ter — mesma medicao com redirecionamento para ARQUIVO em vez de pipe.
# Alguns lancadores do Windows (o stub do WSL em System32 e o caso conhecido)
# escrevem no console em vez dos handles redirecionados; se o arquivo tiver
# conteudo que o pipe nao teve, o "nada" medido pelos testes e artefato do
# pipe, e a resposta esta aqui.
filearm = {"rc": None, "data": "", "exc": None}
try:
    tmp_out = os.path.join(tempfile.mkdtemp(), "item12-filearm.txt")
    with open(tmp_out, "w", encoding="utf-8", errors="replace") as fh:
        filearm["rc"] = subprocess.call(["bash", "-c", PROBE_CMD],
                                        stdout=fh, stderr=subprocess.STDOUT)
    with open(tmp_out, "r", encoding="utf-8", errors="replace") as fh:
        filearm["data"] = fh.read()
except Exception as exc:
    filearm["exc"] = repr(exc)
emit("bare_file_rc", filearm["rc"])
emit("bare_file_data", cut(filearm["data"]))
emit("bare_file_exc", filearm["exc"])

# ---- identidade de cada candidato: e um GNU bash de verdade? ---------
# Gate obrigatorio para poder falar em (B): so um bash PROVADO executando o
# script e devolvendo 1 em silencio autoriza dizer "o script morre". Dois
# nao-bash devolvendo 1 sao (A), nao (B) — e confundir os dois transformaria
# um defeito de harness em alarme de seguranca.
bare_ident = run(["bash", "--version"])
emit("bare_version_rc", bare_ident["rc"])
emit("bare_version_out", cut(bare_ident["out"]))
emit("bare_version_err", cut(bare_ident["err"]))
bare_is_bash = bare_ident["rc"] == 0 and "GNU bash" in (bare_ident["out"] or "")
emit("bare_is_gnu_bash", bare_is_bash)

# ---- 12c/12d — o script REAL, invocado como os 50 testes invocam -----
# Fonte: pypi/tests/test_git_branch_guard.py::_run —
#   subprocess.run(['bash', script], input=json.dumps(payload),
#                  capture_output=True, text=True, cwd=tmpdir)
# Unica diferenca: aqui stdout e IMPRESSO.
sys.path.insert(0, os.environ.get("TRACKFW_PYPI_SRC", "pypi"))
script = None
workdir = None
try:
    from trackfw.generators.init_gen import _generate_git_branch_guard_script
    workdir = tempfile.mkdtemp()
    _generate_git_branch_guard_script(workdir)
    script = os.path.join(workdir, "scripts", "trackfw-git-branch-guard.sh")
    with open(os.path.join(workdir, "trackfw.yaml"), "w", encoding="utf-8") as fh:
        fh.write("project_name: fixture\n")
    emit("script_path", ascii(script))
    emit("script_exists", os.path.isfile(script))
    with open(script, "rb") as fh:
        head = fh.read(120)
    emit("script_head_bytes", ascii(head))
    emit("script_has_crlf", b"\r\n" in head)
except Exception as exc:
    emit("script_setup_exc", repr(exc))

PAYLOAD_BLOCK = json.dumps({"tool_input": {"command": "git push origin HEAD"}})
PAYLOAD_NOOP = json.dumps({"tool_input": {"command": "git status"}})

arms = []
if script:
    launchers = [("bare-name", "bash", bare_is_bash)]
    for i, cand in enumerate(candidates):
        ident = run([cand, "--version"])
        is_bash = ident["rc"] == 0 and "GNU bash" in (ident["out"] or "")
        emit("cand%d_path" % i, ascii(cand))
        emit("cand%d_version_rc" % i, ident["rc"])
        emit("cand%d_version_out" % i, cut(ident["out"]))
        emit("cand%d_is_gnu_bash" % i, is_bash)
        launchers.append(("cand%d" % i, cand, is_bash))

    for label, exe, is_bash in launchers:
        blk = run([exe, script], input=PAYLOAD_BLOCK, cwd=workdir)
        nop = run([exe, script], input=PAYLOAD_NOOP, cwd=workdir)
        emit("%s_block_rc" % label, blk["rc"])
        emit("%s_block_out" % label, cut(blk["out"]))
        emit("%s_block_err" % label, cut(blk["err"]))
        emit("%s_block_exc" % label, blk["exc"])
        emit("%s_noop_rc" % label, nop["rc"])
        emit("%s_noop_out" % label, cut(nop["out"]))
        emit("%s_noop_err" % label, cut(nop["err"]))
        arms.append({
            "label": label, "exe": exe, "is_bash": is_bash,
            "block_rc": blk["rc"], "block_err": blk["err"], "block_out": blk["out"],
            "noop_rc": nop["rc"],
        })

# Esperado (contrato do proprio script, medido no macOS e no Linux do CI):
#   'git push origin HEAD' dentro de projeto -> rc 2 + mensagem em stderr
#   'git status'                             -> rc 0 silencioso
def is_expected(arm):
    return arm["block_rc"] == 2 and arm["noop_rc"] == 0


def is_silent_one(arm):
    return arm["block_rc"] == 1 and not (arm["block_err"] or "").strip()


bare_arm = None
for arm in arms:
    if arm["label"] == "bare-name":
        bare_arm = arm
cand_arms = [a for a in arms if a["label"] != "bare-name"]
good = [a for a in cand_arms if a["is_bash"] and is_expected(a)]
proven_silent = [a for a in cand_arms if a["is_bash"] and is_silent_one(a)]

bare_control_ok = (bare["rc"] == 0 and "PROBE_OUT" in (bare["out"] or "")
                   and "PROBE_ERR" in (bare["err"] or ""))
emit("bare_control_ok", bare_control_ok)

# ---- 12e — sob (B), qual LINHA mata o script? -----------------------
# So faz sentido num bash PROVADO. Sob (A), um -x de nao-bash e ruido.
trace_src = (proven_silent or good)
if script and trace_src:
    xarm = run([trace_src[0]["exe"], "-x", script], input=PAYLOAD_BLOCK, cwd=workdir)
    emit("xtrace_exe", ascii(trace_src[0]["exe"]))
    emit("xtrace_rc", xarm["rc"])
    emit("xtrace_err_tail", ascii("\n".join((xarm["err"] or "").splitlines()[-25:])))
else:
    emit("xtrace_skipped", "nenhum GNU bash provado para tracar")

# ---- Veredito -------------------------------------------------------
if bare_arm is None:
    verdict = "INCONCLUSIVE"
    why = "o script nao pode ser gerado — nada foi invocado"
elif is_expected(bare_arm):
    verdict = "NOT-REPRODUCED"
    why = "o nome nu 'bash' executou o script e devolveu o contrato (2/0): a falha dos 50 nao reproduz nesta sonda"
elif not bare_control_ok:
    verdict = "BRANCH-A"
    why = "o nome nu 'bash' nem roda 'echo' pelo CPython: o processo que atende por 'bash' nao e um bash"
elif good:
    verdict = "BRANCH-A"
    why = ("um bash PROVADO (%s) executa o script e devolve 2/0, enquanto o nome nu devolve rc=%s: "
           "a divergencia esta na RESOLUCAO do executavel, nao no script"
           % (ascii(good[0]["exe"]), bare_arm["block_rc"]))
elif proven_silent:
    verdict = "BRANCH-B"
    why = ("um bash PROVADO (%s) executa o script e ele morre com rc=1 e stderr vazio: "
           "o script morre sob invocacao legitima — vira SEGURANCA (fail-open silencioso)"
           % ascii(proven_silent[0]["exe"]))
else:
    verdict = "INCONCLUSIVE"
    why = ("nenhum candidato do PATH se provou GNU bash, entao rc=1 nao pode ser atribuido nem a "
           "resolucao nem ao script sem ambiguidade")

emit("VERDICT", verdict)
emit("WHY", why)
'@ | Set-Content -Path $item12Probe -Encoding utf8

# PYTHONIOENCODING=utf-8: o item 1 (cp1252) esta VIVO nesta arvore e mataria a
# sonda no primeiro print — mesma neutralizacao pontual dos itens 5 e 6, e pelo
# mesmo motivo (nao mascarar a medicao atras do crash de outro defeito).
# TRACKFW_PYPI_SRC entra em sys.path[0]: os 50 testes importam a arvore do repo,
# nao a copia instalada por `pip install pypi/` — medir a copia errada mediria
# outro codigo.
$r12 = Run-Capture -Exe "python" -ArgList @($item12Probe) -WorkDir $repoRoot.Path `
    -EnvVars @{ PYTHONIOENCODING = "utf-8"; TRACKFW_PYPI_SRC = $env:TRACKFW_PYPI_SRC }

$item12Branch = if ($r12.Stdout -match "ITEM12 VERDICT=([A-Z0-9\-]+)") { $matches[1] } else { "SEM-VEREDITO" }

$item12Detail = @"
12a — where.exe bash (ordem de resolucao do Windows, TODAS as ocorrencias):
exit=$($r12where.ExitCode)
$($r12where.Stdout)$($r12where.Stderr)
12a — where.exe bash.exe:
exit=$($r12whereExe.ExitCode)
$($r12whereExe.Stdout)$($r12whereExe.Stderr)

12b/12c/12d/12e — medicoes do lado do CPython (stdout IMPRESSO):
exit=$($r12.ExitCode)
$($r12.Stdout)
--- stderr da sonda ---
$($r12.Stderr)
Ramificacao medida: $item12Branch
"@

# Veredito mapeado para o vocabulario da suite: as duas ramificacoes sao
# defeito confirmado (REPRODUCED) — a distincao (A)/(B) vive no Detail e no
# titulo, porque o gate de saida da suite compara por igualdade com
# "REPRODUCED" e um rotulo composto passaria despercebido.
$item12Verdict = switch ($item12Branch) {
    "BRANCH-A"       { "REPRODUCED" }
    "BRANCH-B"       { "REPRODUCED" }
    "NOT-REPRODUCED" { "ABSENT" }
    default          { "INCONCLUSIVE" }
}
Add-Result -Item "12" -Title "SONDA ML-0B (fora da issue #216): exit 1 uniforme do bash lancado pelo Python — (A) resolucao do executavel vs (B) o script morre no cabecalho [medido: $item12Branch]" `
    -Verdict $item12Verdict -Detail $item12Detail

# ---------------------------------------------------------------------
# Sumario
# ---------------------------------------------------------------------
Write-Host ""
Write-Host "===================================================================="
Write-Host "SUMARIO — suite de reproducao de defeito (11 itens da issue #216)"
Write-Host "===================================================================="
$results | Format-Table -AutoSize | Out-String | Write-Host

$reproduced = @($results | Where-Object { $_.Verdict -eq "REPRODUCED" })
$inconclusive = @($results | Where-Object { $_.Verdict -eq "INCONCLUSIVE" })
$blocked = @($results | Where-Object { $_.Verdict -eq "BLOCKED-BY-ITEM-1" })

Write-Host "Reproduzidos: $($reproduced.Count) | Inconclusivos: $($inconclusive.Count) | Bloqueados por dependencia (item 1): $($blocked.Count) | Total de linhas: $($results.Count)"

if ($env:GITHUB_STEP_SUMMARY) {
    $md = "## Suite de reproducao de defeito — AC2/AC2b/AC3`n`n"
    $md += "| Item | Titulo | Veredito |`n|---|---|---|`n"
    foreach ($r in $results) { $md += "| $($r.Item) | $($r.Title) | $($r.Verdict) |`n" }
    Add-Content -Path $env:GITHUB_STEP_SUMMARY -Value $md
}

# A suite PRECISA nascer vermelha (AC2): sai 1 se algum item reproduziu o
# defeito conhecido (esperado, pre-correcao), se algo ficou inconclusivo
# sem justificativa esperada, ou se um item ficou BLOQUEADO por dependencia
# de outro defeito ainda nao corrigido (ML-1C: informacao perdida != item
# resolvido — precisa continuar sinalizando vermelho ate a dependencia
# (item 1) ser corrigida).
if ($reproduced.Count -gt 0 -or $inconclusive.Count -gt 0 -or $blocked.Count -gt 0) {
    exit 1
}
exit 0
