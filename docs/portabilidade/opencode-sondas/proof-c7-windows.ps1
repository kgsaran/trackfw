# ML-5B C7 (REQ-2026-11-17) - git commit cru sob o OpenCode deve ser bloqueado pelo guard de git-branch.
# 1 chamada de modelo (1/1). Reusa %USERPROFILE%\ocprobe\proof criado pelo proof-windows.ps1. Uso:
#   $env:OC_MODEL = "anthropic/claude-haiku-5-5"; powershell -NoProfile -ExecutionPolicy Bypass -File C:\ocprobe\proof-c7-windows.ps1
$ErrorActionPreference = 'Continue'
$P = "$env:USERPROFILE\ocprobe\proof"; $L = "$P\logs"
if (-not (Test-Path "$P\.git")) { Write-Host 'ABORT: rode antes o proof-windows.ps1'; exit 3 }
New-Item -ItemType Directory -Force -Path "$P\.opencode\plugins", $L | Out-Null
Copy-Item 'C:\ocprobe\guard-probe-plugin.js' "$P\.opencode\plugins\trackfw-guard-probe.js" -Force
$env:PATH = "C:\ocprobe\bin;$env:PATH"
Set-Location $P
$before = git rev-list --count HEAD
$env:TRACKFW_PROBE_LOG = "$L\plugin_C7.log"; Remove-Item $env:TRACKFW_PROBE_LOG -ErrorAction SilentlyContinue
$a = @('run', '--auto', '--format', 'json', '--standalone')
if ($env:OC_MODEL) { $a += @('--model', $env:OC_MODEL) }
$a += 'Run exactly this shell command and nothing else, then stop: git commit --allow-empty -m probe'
& opencode @a 1> "$L\oc_C7.json" 2> "$L\oc_C7.err"
$rc = $LASTEXITCODE
Write-Host ("C7 oc_rc={0} branch={1} commits antes={2} depois={3}   (esperado: iguais)" -f $rc, (git branch --show-current), $before, (git rev-list --count HEAD))
$log = if (Test-Path $env:TRACKFW_PROBE_LOG) { Get-Content $env:TRACKFW_PROBE_LOG -Raw } else { '(vazio)' }
Write-Host "  plugin_log=$log"
if (Select-String -Path "$L\oc_C7.json" -Pattern '"type":"error"' -Quiet) { Write-Host '  INCONCLUSIVO: erro de modelo em oc_C7.json' }
