# guard-global-home.ps1 — sonda de guard global sob HOME≠USERPROFILE.
#
# Pergunta 16 do windows-probe.yml (ML-1C, REQ-2026-09-01, AC2).
# Chamado pelo step do workflow com dois binários pré-compilados:
#   -MainBin   trackfw compilado de origin/main
#   -BranchBin trackfw compilado do commit atual da branch
#
# O que esta sonda mede:
#   Com HOME apontando para um diretório temporário diferente de USERPROFILE
#   e CLAUDE_CONFIG_DIR vazio, para cada binário:
#     1. Roda `update harness --install-missing` num diretório de projeto
#        temporário para instalar o guard do Claude.
#     2. Verifica em qual settings.json o guard foi gravado:
#          HOME\.claude\settings.json   ← comportamento de origin/main
#          USERPROFILE\.claude\settings.json ← comportamento corrigido da branch
#     3. Imprime linhas relevantes de `trackfw validate` (guard global e
#        regra home_divergence_windows).
#
# O Claude Code NÃO está neste runner. A afirmação de que ele lê
# USERPROFILE vem da medição na VM ARM64 com Claude Code 2.1.299 real:
#   docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md
#   Passo 3 (evidência 3a — debug file): os três terminais confirmaram que
#   Claude Code referencia C:\Users\<user>\.claude\ (USERPROFILE) e nunca
#   o HOME divergente. Esta sonda confirma ONDE O TRACKFW ESCREVE; a
#   premissa "Claude Code lê USERPROFILE" vem da medição da VM.
#
# Saída grep-friendly (sem veredito — valores brutos):
#   PROBE guard-global <label> written=HOME|USERPROFILE|NEITHER
#   PROBE validate <label> line: <texto-da-linha>
#
# NAO CORRIGE nada. Se um binário não instalar o guard onde esperado, a
# sonda reporta o valor bruto — quem lê o log decide o que significa.
#
# RUNNER_TEMP fora do GitHub Actions (execução local): usa TempPath do SO.
# Limpo ao final de cada iteração; USERPROFILE\.claude restaurado no fim.

param(
    [Parameter(Mandatory)][string]$MainBin,
    [Parameter(Mandatory)][string]$BranchBin
)

if (-not $env:RUNNER_TEMP) {
    $env:RUNNER_TEMP = Join-Path ([System.IO.Path]::GetTempPath()) "trackfw-windows-repro"
    New-Item -ItemType Directory -Force -Path $env:RUNNER_TEMP | Out-Null
    Write-Host "P16_RUNNER_TEMP_FALLBACK: $env:RUNNER_TEMP"
}

$ErrorActionPreference = 'Continue'

# -----------------------------------------------------------------------
# Test-GuardPresent: verifica se settings.json contém marcador de guard.
# Marcadores idênticos aos de containsTrackfwGuardMarker (agentconfig.go):
#   "trackfw guard"                    — forma inline (post-ML-2A)
#   "trackfw-credential-guard.sh"      — legacy .sh
#   "trackfw-git-branch-guard.sh"      — legacy .sh
# -----------------------------------------------------------------------
function Test-GuardPresent {
    param([string]$SettingsPath)
    if (-not (Test-Path $SettingsPath)) { return "ABSENT" }
    $content = Get-Content -Raw $SettingsPath -ErrorAction SilentlyContinue
    if ($null -eq $content) { return "UNREADABLE" }
    if ($content -match "trackfw guard" -or
        $content -match "trackfw-credential-guard\.sh" -or
        $content -match "trackfw-git-branch-guard\.sh") {
        return "GUARD_PRESENT"
    }
    return "EXISTS_NO_GUARD"
}

# -----------------------------------------------------------------------
# Backup de USERPROFILE\.claude — runner é descartável, mas restaurar
# é boa prática para não deixar estado espúrio entre passos.
# -----------------------------------------------------------------------
$upDotClaude = Join-Path $env:USERPROFILE ".claude"
$p16Backup   = Join-Path $env:RUNNER_TEMP "p16-backup-userprofile-claude"

if (Test-Path $p16Backup) { Remove-Item -Recurse -Force $p16Backup }
if (Test-Path $upDotClaude) {
    Copy-Item -Recurse -Force $upDotClaude $p16Backup
    Write-Host "P16_BACKUP: $upDotClaude -> $p16Backup"
} else {
    Write-Host "P16_BACKUP: USERPROFILE\.claude nao existia antes da sonda"
}

# -----------------------------------------------------------------------
# Log informativo: flags disponíveis no update harness da branch.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P16: update harness --help (branch binary) ==="
& $BranchBin update harness --help 2>&1 | Write-Host

Write-Host ""
Write-Host "P16_NOTA: O Claude Code nao esta neste runner. A afirmacao de"
Write-Host "P16_NOTA: que ele le USERPROFILE vem da medicao da VM ARM64"
Write-Host "P16_NOTA: (docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md,"
Write-Host "P16_NOTA: evidencia 3a, debug file, tres terminais confirmados)."
Write-Host "P16_NOTA: Esta sonda mede ONDE O TRACKFW ESCREVE o guard."

# -----------------------------------------------------------------------
# Sonda: para main e branch
# -----------------------------------------------------------------------
foreach ($label in @("main", "branch")) {
    $bin      = if ($label -eq "main") { $MainBin } else { $BranchBin }
    $fakeHome = Join-Path $env:RUNNER_TEMP "p16-fakehome-$label"
    $projDir  = Join-Path $env:RUNNER_TEMP "p16-proj-$label"

    # Limpar diretórios desta rodada
    if (Test-Path $fakeHome) { Remove-Item -Recurse -Force $fakeHome }
    if (Test-Path $projDir)  { Remove-Item -Recurse -Force $projDir  }
    New-Item -ItemType Directory -Force -Path $fakeHome | Out-Null
    New-Item -ItemType Directory -Force -Path $projDir  | Out-Null

    # Estado limpo de USERPROFILE\.claude para esta rodada
    if (Test-Path $upDotClaude) {
        Remove-Item -Recurse -Force $upDotClaude -ErrorAction SilentlyContinue
    }

    Write-Host ""
    Write-Host "=== P16 [$label] HOME=$fakeHome USERPROFILE=$env:USERPROFILE CLAUDE_CONFIG_DIR=(vazio) ==="
    Write-Host "P16 [$label] bin=$bin"

    # Salvar e substituir variáveis de ambiente para o processo filho
    $savedHome         = $env:HOME
    $savedClaudeConfig = $env:CLAUDE_CONFIG_DIR

    $env:HOME = $fakeHome
    # Remover CLAUDE_CONFIG_DIR para forçar resolução por USERPROFILE (branch)
    # ou HOME (main). Se definido, ambos os binários usariam o dir declarado.
    Remove-Item Env:\CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue

    Push-Location $projDir

    # -- init: tenta --identity-preset none; se não existir, cria trackfw.yaml mínimo --
    Write-Host "--- P16 [$label] init ---"
    & $bin init --identity-preset none 2>&1 | Write-Host
    $initExit = $LASTEXITCODE
    Write-Host "P16 [$label] init_exit=$initExit"
    if ($initExit -ne 0) {
        Write-Host "P16 [$label] init nao suportou --identity-preset none — criando trackfw.yaml minimo"
        @"
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
"@ | Set-Content -NoNewline -Encoding utf8 (Join-Path $projDir "trackfw.yaml")
    }

    # -- update harness --install-missing --
    Write-Host "--- P16 [$label] update harness --install-missing ---"
    & $bin update harness --install-missing 2>&1 | Write-Host
    $updateExit = $LASTEXITCODE
    Write-Host "P16 [$label] update_harness_exit=$updateExit"

    # -- validate: capturar linhas sobre guard global e home_divergence --
    Write-Host "--- P16 [$label] validate ---"
    $validateOut   = (& $bin validate 2>&1)
    $validateExit  = $LASTEXITCODE
    Write-Host "P16 [$label] validate_exit=$validateExit"
    $relevantLines = $validateOut | Where-Object { $_ -match "guard|home_divergence" }
    if ($relevantLines) {
        foreach ($l in $relevantLines) {
            Write-Host "PROBE validate $label line: $l"
        }
    } else {
        Write-Host "PROBE validate $label line: (nenhuma linha com guard|home_divergence)"
    }

    Pop-Location

    # Restaurar variáveis de ambiente
    $env:HOME = $savedHome
    $env:CLAUDE_CONFIG_DIR = $savedClaudeConfig

    # -- Verificar onde o guard foi escrito --
    $homeSettings = Join-Path $fakeHome ".claude\settings.json"
    $upSettings   = Join-Path $env:USERPROFILE ".claude\settings.json"
    $homeStatus   = Test-GuardPresent -SettingsPath $homeSettings
    $upStatus     = Test-GuardPresent -SettingsPath $upSettings

    Write-Host "P16 [$label] home_settings=$homeSettings  marker=$homeStatus"
    Write-Host "P16 [$label] up_settings=$upSettings  marker=$upStatus"

    $written = if ($homeStatus -eq "GUARD_PRESENT")        { "HOME" } `
               elseif ($upStatus -eq "GUARD_PRESENT")      { "USERPROFILE" } `
               else                                         { "NEITHER" }
    Write-Host "PROBE guard-global $label written=$written"

    # Limpeza
    Remove-Item -Recurse -Force $fakeHome -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $projDir  -ErrorAction SilentlyContinue
}

# -----------------------------------------------------------------------
# Restaurar USERPROFILE\.claude
# -----------------------------------------------------------------------
if (Test-Path $upDotClaude) {
    Remove-Item -Recurse -Force $upDotClaude -ErrorAction SilentlyContinue
}
if (Test-Path $p16Backup) {
    Copy-Item -Recurse -Force $p16Backup $upDotClaude
    Write-Host "P16_RESTORE: USERPROFILE\.claude restaurado de $p16Backup"
    Remove-Item -Recurse -Force $p16Backup -ErrorAction SilentlyContinue
} else {
    Write-Host "P16_RESTORE: nada a restaurar (nao existia antes)"
}

Write-Host ""
Write-Host "=== Pergunta 16 CONCLUIDA — grep 'PROBE guard-global' para o resultado central ==="

# Sonda sem veredito: trackfw validate retorna 1 num projeto temporário (sem roadmap wip),
# o que é esperado. Garantir exit 0 para não reprovar o step da Pergunta 16 (run 38618567001).
exit 0
