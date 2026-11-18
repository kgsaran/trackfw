# ML-1A (REQ-2026-11-17) — sonda do contrato de hook do Antigravity no Windows.
# Uso: powershell -NoProfile -ExecutionPolicy Bypass -File run-arms-windows.ps1   (7 chamadas: 7/7)
# Só escreve em C:\agyprobe e %USERPROFILE%\agyprobe. Não toca em %USERPROFILE%\.gemini.
$ErrorActionPreference = 'Continue'
$Root = 'C:\agyprobe'; $L = "$Root\logs"; $M = "$Root\markers"; $P = "$env:USERPROFILE\agyprobe"
foreach ($d in @("$Root\scripts", $L, $M, "$P\.agents")) { New-Item -ItemType Directory -Path $d -Force | Out-Null }
Set-Location $P
if (-not (Test-Path "$P\.git")) { git init -q 2>$null }

$probe = @'
param([string]$Arm = "unknown")
$LogFile = "C:\agyprobe\logs\probe_${Arm}_$PID.log"
$StdinData = [Console]::In.ReadToEnd()
$Me = Get-CimInstance Win32_Process -Filter "ProcessId = $PID"
$Pa = Get-CimInstance Win32_Process -Filter "ProcessId = $($Me.ParentProcessId)"
$Gp = Get-CimInstance Win32_Process -Filter "ProcessId = $($Pa.ParentProcessId)"
$Lines = @("=== ARM: $Arm ===", "--- STDIN ---", $StdinData, "--- ENV ---",
  "PSVersion=$($PSVersionTable.PSVersion)", "CWD=$(Get-Location)", "HOME=$env:HOME",
  "USERPROFILE=$env:USERPROFILE", "PATH=$env:PATH", "--- PROCESS ---",
  "ME PID=$($Me.ProcessId) CMD=$($Me.CommandLine)",
  "PARENT PID=$($Pa.ProcessId) NAME=$($Pa.Name) CMD=$($Pa.CommandLine)",
  "GRANDPARENT PID=$($Gp.ProcessId) NAME=$($Gp.Name) CMD=$($Gp.CommandLine)", "=== END ===")
[IO.File]::WriteAllText($LogFile, ($Lines -join "`n") + "`n", [Text.Encoding]::ASCII)
switch -Wildcard ($Arm) {
  "allow*"  { [Console]::Out.Write('{"decision":"allow"}') }
  "deny*"   { [Console]::Out.Write('{"decision":"deny","reason":"probe deny arm"}') }
  "exit2"   { [Console]::Error.WriteLine("probe exit2 stderr"); exit 2 }
  "exit1"   { exit 1 }
  "empty"   { }
  "badjson" { [Console]::Out.Write('NOT_JSON') }
  default   { [Console]::Out.Write('{"decision":"allow"}') }
}
'@
[IO.File]::WriteAllText("$Root\scripts\probe.ps1", $probe, [Text.Encoding]::ASCII)

function Write-Hooks([string]$Arm) {
  $cmd = "powershell -NoProfile -ExecutionPolicy Bypass -File C:/agyprobe/scripts/probe.ps1 -Arm $Arm"
  $json = '{"probe":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"' + $cmd + '","timeout":10}]}]}}'
  [IO.File]::WriteAllText("$P\.agents\hooks.json", $json, [Text.Encoding]::ASCII)
}

$Out = "$Root\RESULT-windows.txt"
[IO.File]::WriteAllText($Out, "agy: $(agy --version 2>&1)`ndate: $((Get-Date).ToUniversalTime().ToString('s'))Z`n`n", [Text.Encoding]::ASCII)
$arms = @('allow','deny','exit2','empty','badjson','exit1','shellid'); $n = 0
foreach ($a in $arms) {
  $n++
  Remove-Item "$M\marker_$a.txt", "$L\probe_${a}_*.log", "$L\shellid.txt" -ErrorAction SilentlyContinue
  if ($a -eq 'shellid') {
    # cmd expande %COMSPEC%; PowerShell expande $PSVersionTable; sh expande $0
    $json = '{"probe":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"echo COMSPEC=%COMSPEC% PS=$PSVersionTable ZERO=$0 > C:/agyprobe/logs/shellid.txt","timeout":10}]}]}}'
    [IO.File]::WriteAllText("$P\.agents\hooks.json", $json, [Text.Encoding]::ASCII)
  } else { Write-Hooks $a }
  Write-Host "[$n/$($arms.Count)] braço $a ..."
  agy -p "Run exactly this shell command and nothing else, then stop: echo MARKER_$a > C:/agyprobe/markers/marker_$a.txt" `
      --dangerously-skip-permissions --output-format stream-json --print-timeout 180s `
      1> "$L\agy_$a.ndjson" 2> "$L\agy_$a.err"
  $rc = $LASTEXITCODE
  $mk = if (Test-Path "$M\marker_$a.txt") { 'yes' } else { 'no' }
  $lg = if ($a -eq 'shellid') { [int](Test-Path "$L\shellid.txt") } else { @(Get-ChildItem "$L\probe_${a}_*.log" -ErrorAction SilentlyContinue).Count }
  $line = "{0,-8} agy_rc={1} hook_fired={2} marker={3}" -f $a, $rc, $lg, $mk
  Write-Host $line; Add-Content -Encoding ASCII -Path $Out -Value $line
}
Write-Host "`nResultado: $Out  |  logs: $L"
