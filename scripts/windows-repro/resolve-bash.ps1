# resolve-bash.ps1 — localiza um bash com IDENTIDADE PROVADA (GNU bash de
# verdade), nunca o primeiro processo que atende pelo nome "bash".
#
# Origem da correcao: run 33986718256 (job windows-defect-reproduction, apos
# o merge do PR #280). O item 4, retargetado no ML-2C para invocar
# scripts/check-parity-contract-coverage.sh REAL via `bash`, mediu
# INCONCLUSIVE. A saida (tail) trazia texto com espacamento caractere-a-
# caractere — assinatura de UTF-16 decodificado como se fosse texto de byte
# unico — dizendo "Windows Subsystem for Linux has no installed
# distributions.": o `bash` resolvido pelo nome cru era o STUB DO WSL em
# C:\Windows\System32\bash.exe, nao o Git Bash. O stub atende pelo nome
# "bash" e VENCE a resolucao por nome cru quando nao ha distro WSL instalada
# — sem executar nada do que foi pedido.
#
# `env=` NAO resolve isto: medido que o CreateProcess do Windows resolve
# nomes de executavel sem diretorio pelo %PATH% do PROCESSO PAI, nunca por
# uma variavel de ambiente passada ao filho.
#
# `shutil.which`/`where.exe` sozinhos tambem NAO bastam como fonte unica de
# verdade — foi exatamente o que a sonda do item 12 (ML-0B, ver run.ps1)
# recusou: os dois listam o PRIMEIRO nome que casa no PATH, e no Windows
# real esse primeiro pode ser o stub. A defesa e provar IDENTIDADE
# (`--version` contendo "GNU bash") antes de usar qualquer candidato —
# nunca confiar no primeiro achado, seja ele de which, where.exe ou do PATH
# cru.
#
# Este mesmo padrao (identidade em vez de nome) ja existe para os testes
# Python do grupo B — ver pypi/tests/bash_path.py (bash_cmd) e
# docs/qualidade/2026-09-04-grupo-b-bash-do-python-em-windows.md.

function Resolve-ProvenBash {
    <#
    .SYNOPSIS
    Enumera candidatos a "bash" por caminho ABSOLUTO (PATH do runner +
    locais canonicos do Git for Windows, que ficam FORA do %PATH% por
    padrao) e prova a identidade de cada um com `--version` antes de
    devolver um vencedor.

    .OUTPUTS
    [pscustomobject] com:
      Path  — caminho absoluto do bash PROVADO, ou $null se nenhum provou
      Tried — lista de candidatos testados, cada um com Path/ExitCode/
              Output/IsProven, para diagnostico quando Path for $null
    #>
    param([string[]]$ExtraCandidates = @())

    $pathEntries = @()
    if ($env:PATH) {
        $pathEntries = $env:PATH -split [System.IO.Path]::PathSeparator | Where-Object { $_ }
    }

    $candidates = New-Object System.Collections.Generic.List[string]
    foreach ($entry in $pathEntries) {
        foreach ($name in @("bash.exe", "bash")) {
            $cand = $null
            try { $cand = Join-Path $entry $name } catch { continue }
            if ($cand -and (Test-Path -LiteralPath $cand -PathType Leaf) -and (-not $candidates.Contains($cand))) {
                $candidates.Add($cand)
            }
        }
    }

    # Locais canonicos do Git for Windows: o instalador padrao NAO adiciona
    # usr\bin ao %PATH%. Sem semear estes, a ausencia no PATH devolveria
    # "nenhum candidato provado" quando um Git Bash de verdade estava
    # alcancavel por caminho absoluto. C:\Windows\System32\bash.exe (o stub
    # do WSL) entra na lista de PROPOSITO — precisa ser testado e
    # REPROVADO, nao apenas evitado por convencao.
    $canonical = @(
        "C:\Program Files\Git\bin\bash.exe",
        "C:\Program Files\Git\usr\bin\bash.exe",
        "C:\Program Files\Git\usr\bin\bash",
        "C:\Windows\System32\bash.exe"
    ) + $ExtraCandidates
    foreach ($extra in $canonical) {
        if ((Test-Path -LiteralPath $extra -PathType Leaf) -and (-not $candidates.Contains($extra))) {
            $candidates.Add($extra)
        }
    }

    $tried = New-Object System.Collections.Generic.List[pscustomobject]
    $winner = $null
    foreach ($cand in $candidates) {
        $out = $null
        $exitCode = $null
        try {
            $out = (& $cand "--version" 2>&1 | Out-String)
            $exitCode = $LASTEXITCODE
        } catch {
            $out = "EXCEPTION: $($_.Exception.Message)"
            $exitCode = -1
        }
        $isProven = ($exitCode -eq 0) -and ($out -match "GNU bash")
        $tried.Add([pscustomobject]@{
            Path     = $cand
            ExitCode = $exitCode
            Output   = $out
            IsProven = $isProven
        })
        if ($isProven -and (-not $winner)) {
            $winner = $cand
        }
    }

    return [pscustomobject]@{ Path = $winner; Tried = $tried }
}
