# P21 is an observation instrument, always exit 0. It does not fire agent events.
param([Parameter(Mandatory)][string]$Bin, [Parameter(Mandatory)][string]$Workspace)
$ErrorActionPreference = 'Stop'
$owned = $null
$server = $null
function Probe([string]$Label, $Data) {
    Write-Host ('PROBE attention ' + $Label + ' ' + ($Data | ConvertTo-Json -Compress -Depth 8))
}
function Start-Owned([string]$File, [string[]]$Arguments) {
    $si = [Diagnostics.ProcessStartInfo]::new()
    $si.FileName = $File
    $si.WorkingDirectory = $script:fixture
    $si.UseShellExecute = $false
    $si.RedirectStandardInput = $true
    $si.RedirectStandardOutput = $true
    $si.RedirectStandardError = $true
    $si.StandardInputEncoding = [Text.UTF8Encoding]::new($false)
    foreach ($arg in $Arguments) { $si.ArgumentList.Add($arg) }
    $si.Environment['PATH'] = $script:nativePath
    $si.Environment['HOME'] = $script:homeDir
    $si.Environment['USERPROFILE'] = $script:homeDir
    $si.Environment['XDG_CONFIG_HOME'] = $script:homeDir
    $si.Environment['TRACKFW_DISABLE_EXTERNAL_COMMANDS'] = '1'
    $p = [Diagnostics.Process]::new()
    $p.StartInfo = $si
    [void]$p.Start()
    # Drain both streams concurrently; never read one stream to EOF first.
    return @{ Process=$p; Out=$p.StandardOutput.ReadToEndAsync(); Err=$p.StandardError.ReadToEndAsync() }
}
function Stop-Owned($Handle) {
    if ($null -ne $Handle) {
        if (-not $Handle.Process.HasExited) { $Handle.Process.Kill($true) }
        [void]$Handle.Process.WaitForExit(2000)
        $Handle.Process.Dispose()
    }
}
function Invoke-Owned([string]$File, [string[]]$Arguments, [string]$InputText = '') {
    $h = $null
    try {
        $h = Start-Owned $File $Arguments
        $h.Process.StandardInput.Write($InputText)
        $h.Process.StandardInput.Close()
        $done = $h.Process.WaitForExit(5000)
        if (-not $done) { return @{ exit=$null; timeout=$true } }
        # Fixture output is intentionally not logged (only counts and exit).
        return @{ exit=$h.Process.ExitCode; timeout=$false; stdout_bytes=$h.Out.Result.Length; stderr_bytes=$h.Err.Result.Length }
    } finally { Stop-Owned $h }
}
function Read-Attention([string]$Url, [bool]$Expected, [string]$Message) {
    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    $lastActive = $null
    $lastExact = $null
    $lastError = $null
    do {
        try {
            $r = Invoke-RestMethod -Uri $Url -TimeoutSec 1 -NoProxy
            $lastActive = $r.active
            $lastExact = $r.message -ceq $Message
            if (($r.active -eq $Expected) -and ((-not $Expected) -or ($r.message -ceq $Message))) {
                return @{ active=$r.active; message_exact=((-not $Expected) -or ($r.message -ceq $Message)); observed=$true }
            }
        } catch { $lastError = $_.Exception.GetType().FullName }
        Start-Sleep -Milliseconds 100
    } while ([DateTime]::UtcNow -lt $deadline)
    return @{ observed=$false; expected_active=$Expected; actual_active=$lastActive; message_exact=$lastExact; last_error=$lastError }
}
try {
    Probe 'scope' @{ measurement='native_shell_api_smoke'; agent_event=$false; ac5_ac7='pending' }
    if (-not $IsWindows) { Probe 'setup' @{ result='UNMEASURED'; reason='requires_windows' }; exit 0 }
    $sourceBin = (Resolve-Path -LiteralPath $Bin).Path
    $sourceRoot = (Resolve-Path -LiteralPath $Workspace).Path
    $owned = Join-Path ([IO.Path]::GetTempPath()) ('p21 atenção ' + [Guid]::NewGuid().ToString('N'))
    $fixture = Join-Path $owned 'projeto ação'
    $binDir = Join-Path $owned 'binário nativo'
    $homeDir = Join-Path $owned 'home isolado'
    $roadmaps = Join-Path $fixture 'docs/planos ação'
    foreach ($dir in @($fixture,$binDir,$homeDir,$roadmaps)) { [void](New-Item -ItemType Directory -Path $dir -Force) }
    $nativeBin = Join-Path $binDir 'trackfw.exe'
    Copy-Item -LiteralPath $sourceBin -Destination $nativeBin
    $utf8 = [Text.UTF8Encoding]::new($false)
    [IO.File]::WriteAllText((Join-Path $fixture 'trackfw.yaml'), "project: p21`r`nroadmap_dir: 'docs/planos ação'`r`n", $utf8)
    $credential = Join-Path $roadmaps '.trackfw-credential-guard.json'
    [IO.File]::WriteAllText($credential, '{"fixture":"preserve ação"}', $utf8)
    $credentialBefore = [Convert]::ToBase64String([IO.File]::ReadAllBytes($credential))
    # Shells use absolute paths; omit System32 too (it may contain WSL bash.exe).
    $nativePath = $binDir
    $psExe = (Get-Process -Id $PID).Path
    $cmdExe = Join-Path $env:SystemRoot 'System32/cmd.exe'
    Probe 'binary' @{ sha256=(Get-FileHash -LiteralPath $nativeBin).Hash; spaces_accent=$true; config_eol='CRLF' }
    # Resolve helpers in the same restricted child environment, not the host PATH.
    $resolutionCode = 'foreach($n in @("bash","jq","python","python3","trackfw")){ $c=Get-Command $n -ErrorAction SilentlyContinue; [Console]::WriteLine($n+"="+($null -ne $c)) }'
    $h = Start-Owned $psExe @('-NoProfile','-NonInteractive','-Command',$resolutionCode)
    try {
        $h.Process.StandardInput.Close()
        if ($h.Process.WaitForExit(5000)) { Probe 'resolution' @{ exit=$h.Process.ExitCode; results=$h.Out.Result.Trim() } }
        else { Probe 'resolution' @{ timeout=$true } }
    } finally { Stop-Owned $h }
    # Source constants are checked; this intentionally does not claim generated-hook execution.
    $source = [IO.File]::ReadAllText((Join-Path $sourceRoot 'internal/generators/agentfiles.go'))
    $signal = 'trackfw attention signal'
    $cleanup = 'trackfw attention cleanup'
    Probe 'command_origin' @{ kind='constant_command_smoke'; signal_source_exact=$source.Contains('const attentionSignalCmd = "trackfw attention signal"'); cleanup_source_exact=$source.Contains('const attentionCleanupCmd = "trackfw attention cleanup"'); generated_fixture=$false }
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
    $listener.Start()
    $port = $listener.LocalEndpoint.Port
    $listener.Stop()
    $server = Start-Owned $nativeBin @('serve','--host','127.0.0.1','--port',"$port")
    $server.Process.StandardInput.Close()
    $url = "http://127.0.0.1:$port/api/attention"
    Probe 'serve_initial' (Read-Attention $url $false '')
    # JSON is stdin only, never interpolated into command text or argv.
    $message = 'ação "aspas" & | < > %PATH% ! ^ ; $(fixture) 😀'
    $payload = @{ tool_name='fixture'; tool_input=@{ question=$message } } | ConvertTo-Json -Compress
    foreach ($shell in @('powershell','cmd')) {
        if ($shell -eq 'powershell') { $exe=$psExe; $prefix=@('-NoProfile','-NonInteractive','-Command'); $suffix='; exit $LASTEXITCODE' }
        else { $exe=$cmdExe; $prefix=@('/d','/s','/c'); $suffix='' }
        Probe "$shell-signal" (Invoke-Owned $exe ($prefix + @($signal+$suffix)) $payload)
        Probe "$shell-api-signal" (Read-Attention $url $true $message)
        Probe "$shell-cleanup" (Invoke-Owned $exe ($prefix + @($cleanup+$suffix)))
        Probe "$shell-api-cleanup" (Read-Attention $url $false '')
        Probe "$shell-credential" @{ bytes_identical=([Convert]::ToBase64String([IO.File]::ReadAllBytes($credential)) -ceq $credentialBefore) }
    }
    Probe 'serve_exit' @{ running=(-not $server.Process.HasExited); exit=$(if ($server.Process.HasExited) { $server.Process.ExitCode } else { $null }) }
} catch {
    # Do not print exception text: paths/payload from outside fixtures may be sensitive.
    Probe 'error' @{ result='UNMEASURED_OR_INCOMPLETE'; type=$_.Exception.GetType().FullName }
} finally {
    try { Stop-Owned $server } catch { Probe 'server_cleanup' @{ error_type=$_.Exception.GetType().FullName } }
    try {
        if ($null -ne $owned -and (Test-Path -LiteralPath $owned)) { Remove-Item -LiteralPath $owned -Recurse -Force }
    } catch { Probe 'fixture_cleanup' @{ error_type=$_.Exception.GetType().FullName } }
}
exit 0
