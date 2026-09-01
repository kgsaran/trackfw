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
