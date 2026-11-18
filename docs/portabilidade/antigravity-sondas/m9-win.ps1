# ML-1B Windows - 3 bracos (3/3 chamadas). Escreve %USERPROFILE%\.gemini\config\hooks.json e apaga no fim.
$ErrorActionPreference = 'Continue'
$P = "$env:USERPROFILE\agyprobe"; $L = 'C:\agyprobe\logs'; $M = 'C:\agyprobe\markers'
$RealProfile = $env:USERPROFILE
$G = "$RealProfile\.gemini\config\hooks.json"
$Alt = 'C:\agyprobe\altprofile'; $GA = "$Alt\.gemini\config\hooks.json"
if (Test-Path $G) { Write-Host "ABORT: $G ja existe"; exit 3 }
New-Item -ItemType Directory -Force -Path (Split-Path $G), (Split-Path $GA) | Out-Null
Set-Location $P
function Hooks($name, $arm, $path) {
  $cmd = "powershell -NoProfile -ExecutionPolicy Bypass -File C:/agyprobe/scripts/probe.ps1 -Arm $arm"
  $j = '{"' + $name + '":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"' + $cmd + '","timeout":10}]}]}}'
  [IO.File]::WriteAllText($path, $j, [Text.Encoding]::ASCII)
}
function Run($a) {
  Remove-Item "$M\marker_$a.txt" -ErrorAction SilentlyContinue
  agy -p "Run exactly this shell command and nothing else, then stop: echo MARKER_$a > C:/agyprobe/markers/marker_$a.txt" --dangerously-skip-permissions --output-format stream-json --print-timeout 180s 1> "$L\agy_$a.ndjson" 2> "$L\agy_$a.err"
  $rc = $LASTEXITCODE
  "agy_rc=$rc marker=" + $(if (Test-Path "$M\marker_$a.txt") { 'yes' } else { 'no' })
}
function Cnt($pat) { @(Get-ChildItem "$L\probe_${pat}_*.log" -ErrorAction SilentlyContinue).Count }
try {
  Remove-Item "$L\probe_*global*.log","$L\probe_*ws*.log","$L\probe_*alt*.log" -ErrorAction SilentlyContinue
  Write-Host "[1/3] A1 so global"
  Remove-Item "$P\.agents\hooks.json" -ErrorAction SilentlyContinue
  Hooks g allow-global $G
  Write-Host ("A1 " + (Run a1) + " global_fired=" + (Cnt 'allow-global'))
  Remove-Item "$L\probe_*global*.log" -ErrorAction SilentlyContinue
  Write-Host "[2/3] A2 global + projeto"
  Hooks g allow-global $G; Hooks w allow-ws "$P\.agents\hooks.json"
  Write-Host ("A2 " + (Run a2) + " global_fired=" + (Cnt 'allow-global') + " ws_fired=" + (Cnt 'allow-ws'))
  Remove-Item "$L\probe_*global*.log","$L\probe_*ws*.log","$P\.agents\hooks.json" -ErrorAction SilentlyContinue
  Write-Host "[3/3] A3 USERPROFILE alternativo"
  Hooks g allow-global $G; Hooks a allow-alt $GA
  $env:USERPROFILE = $Alt
  Write-Host ("A3 " + (Run a3) + " real_global_fired=" + (Cnt 'allow-global') + " alt_fired=" + (Cnt 'allow-alt'))
} finally {
  $env:USERPROFILE = $RealProfile
  Remove-Item $G, $GA -ErrorAction SilentlyContinue
  Write-Host "cleanup: $G e $GA removidos; existe=$(Test-Path $G)"
}
