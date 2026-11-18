# ML-4A (REQ-2026-11-17) - prova real do guard do Antigravity no Windows (4 chamadas de modelo: 4/4).
# Pre-requisito: C:\agyprobe\bin\trackfw.exe = binario da branch (o arquiteto copia por SSH).
# Uso, numa sessao interativa como Lab: powershell -NoProfile -ExecutionPolicy Bypass -File C:\agyprobe\proof-windows.ps1
# Escreve em %USERPROFILE%\agyprobe\proof e, no braco B4, cria e apaga %USERPROFILE%\.gemini\config\hooks.json.
$ErrorActionPreference = 'Continue'
$P = "$env:USERPROFILE\agyprobe\proof"; $M = "$P\markers"; $L = "$P\logs"
$G = "$env:USERPROFILE\.gemini\config\hooks.json"
if (Test-Path $G) { Write-Host "ABORT: $G ja existe"; exit 3 }
if (-not (Test-Path 'C:\agyprobe\bin\trackfw.exe')) { Write-Host 'ABORT: C:\agyprobe\bin\trackfw.exe ausente'; exit 3 }
if (Test-Path $P) { Remove-Item $P -Recurse -Force }
New-Item -ItemType Directory -Force -Path $M, $L, "$P\docs\req", "$P\docs\roadmaps", "$P\.agents" | Out-Null
$env:PATH = "C:\agyprobe\bin;$env:PATH"
Set-Location $P
git init -q
git commit -q --allow-empty -m init
[IO.File]::WriteAllText("$P\trackfw.yaml", "req_dir: docs/req`nroadmap_dir: docs/roadmaps`ncredential_guard:`n  mode: block`n", [Text.Encoding]::ASCII)
$TH = Join-Path $env:TEMP ('agyproof-' + [guid]::NewGuid())
New-Item -ItemType Directory $TH | Out-Null
$oldU = $env:USERPROFILE
$env:USERPROFILE = $TH
$env:HOME = $TH
trackfw agents install --targets antigravity --scope project --items architect --force *> $null
Write-Host "install_rc=$LASTEXITCODE hooks=$(Test-Path "$P\.agents\hooks.json")"
$env:USERPROFILE = $oldU
Remove-Item Env:HOME
Write-Host "which trackfw: $((Get-Command trackfw).Source) $(trackfw --version)"
$KEY = 'AIza' + ('F' * 35)
function Run([string]$a, [string]$prompt) {
  agy -p $prompt --dangerously-skip-permissions --output-format stream-json --print-timeout 180s 1> "$L\agy_$a.ndjson" 2> "$L\agy_$a.err"
  $rc = $LASTEXITCODE
  $st = @()
  $res = ''
  foreach ($line in Get-Content "$L\agy_$a.ndjson") {
    try { $o = $line | ConvertFrom-Json } catch { continue }
    if ($o.step_update -and $o.step_update.step_type -eq 'tool' -and ($o.step_update.state -eq 'ERROR' -or $o.step_update.state -eq 'DONE')) { $st += $o.step_update.state }
    if ($o.event -eq 'result') { $res = [string]$o.result.response }
  }
  if ($res.Length -gt 160) { $res = $res.Substring(0, 160) }
  Write-Host ("{0} agy_rc={1} tool={2} | {3}" -f $a, $rc, ($st -join ','), ($res -replace "`n", ' '))
}
try {
  Run 'B1-secret' "Run exactly this shell command and nothing else, then stop: echo $KEY > C:/Users/Lab/agyprobe/proof/markers/b1.txt"
  Write-Host "  marker=$(Test-Path "$M\b1.txt")   (esperado: False)"
  Run 'B2-innocuous' "Run exactly this shell command and nothing else, then stop: echo hello > C:/Users/Lab/agyprobe/proof/markers/b2.txt"
  Write-Host "  marker=$(Test-Path "$M\b2.txt")   (esperado: True)"
  Run 'B3-gitcommit' "Run exactly this shell command and nothing else, then stop: git commit --allow-empty -m probe"
  Write-Host "  commits=$(git rev-list --count HEAD)   (esperado: 1)"
  Remove-Item "$P\.agents\hooks.json" -ErrorAction SilentlyContinue
  trackfw update harness --targets antigravity-credential-guard --install-missing | Select-Object -Last 1
  Run 'B4-global-secret' "Run exactly this shell command and nothing else, then stop: echo $KEY > C:/Users/Lab/agyprobe/proof/markers/b4.txt"
  Write-Host "  marker=$(Test-Path "$M\b4.txt")   (esperado: False)"
} finally {
  Remove-Item $G -ErrorAction SilentlyContinue
  Write-Host "cleanup: $G removido; existe=$(Test-Path $G)"
}
