# check-install-ps1.ps1 -- Gate for scripts/install.ps1
# Asserts the security contract of the PowerShell installer:
#   - no banned tokens in source (static check)
#   - invalid TRACKFW_VERSION and TRACKFW_ARCH are rejected before any network call
#   - checksum absent/duplicate/divergent each abort without installing the binary
#   - throw (not exit) is used inside & { }, so iex callers survive a failure
#   - happy path installs the binary and prints version (Windows CI only)
#   - PATH modification is idempotent; TRACKFW_NO_MODIFY_PATH=1 disables it (Windows CI only)
#
# Usage:
#   pwsh -File scripts/check-install-ps1.ps1                  # local (macOS/Linux)
#   pwsh -File scripts/check-install-ps1.ps1 -ExpectEdition Core
#   powershell -ExecutionPolicy Bypass -File scripts/check-install-ps1.ps1 -ExpectEdition Desktop
#
# ASCII-only source: PS 5.1 reads BOM-less UTF-8 as cp1252; accented chars break the tokenizer.

param(
    [string]$InstallScript = '',
    [string]$ExpectEdition = ''
)

$ErrorActionPreference = 'Stop'
$ProgressPreference    = 'SilentlyContinue'
$PSNativeCommandUseErrorActionPreference = $false

if ($InstallScript -eq '') {
    $InstallScript = Join-Path $PSScriptRoot 'install.ps1'
}

# ---------------------------------------------------------------------------
# Edition assertion
# ---------------------------------------------------------------------------
if ($ExpectEdition -ne '') {
    $actual = $PSVersionTable.PSEdition
    if ($actual -ne $ExpectEdition) {
        Write-Error "Expected PSEdition='$ExpectEdition', found '$actual'. Aborting gate."
        exit 1
    }
    Write-Host "Edition OK: $actual"
}

# ---------------------------------------------------------------------------
# Platform detection
# PS 5.1: $IsWindows is $null; use PSEdition='Desktop' as the sentinel.
# ---------------------------------------------------------------------------
$onWindows = ($PSVersionTable.PSEdition -eq 'Desktop') -or ($IsWindows -eq $true)

# ---------------------------------------------------------------------------
# Current PowerShell executable (used to spawn children with the same runtime)
# ---------------------------------------------------------------------------
$script:psExe = (Get-Process -Id $PID).Path
Write-Host "Gate: $($script:psExe) (PS $($PSVersionTable.PSVersion))"

# ---------------------------------------------------------------------------
# Score tracking
# ---------------------------------------------------------------------------
$script:PASS = 0
$script:FAIL = 0

function Pass-Scenario {
    param([string]$Name)
    $script:PASS++
    Write-Host "  PASS  $Name"
}

function Fail-Scenario {
    param([string]$Name, [string]$Reason)
    $script:FAIL++
    Write-Host "  FAIL  $Name -- $Reason"
}

# ---------------------------------------------------------------------------
# Helper: run a PowerShell script block in an isolated child process.
# EnvOverrides are set AFTER stripping TRACKFW_* and GATE_* from the parent env,
# so scenarios cannot interfere with each other.
# Returns @{ ExitCode; Stdout; Stderr }
# ---------------------------------------------------------------------------
function Invoke-PSChild {
    param(
        [string]   $Code,
        [hashtable]$Env = @{}
    )
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($Code))

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName               = $script:psExe
    $psi.Arguments              = "-NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand $encoded"
    $psi.UseShellExecute        = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError  = $true
    $psi.CreateNoWindow         = $true

    # Materialize keys before removal to avoid modifying-while-iterating
    $envDict  = $psi.EnvironmentVariables
    $toRemove = @($envDict.Keys | Where-Object { $_ -like 'TRACKFW_*' -or $_ -like 'GATE_*' })
    foreach ($k in $toRemove) { $envDict.Remove($k) }

    foreach ($kv in $Env.GetEnumerator()) { $envDict[$kv.Key] = $kv.Value }

    $proc        = [System.Diagnostics.Process]::Start($psi)
    $stdoutAsync = $proc.StandardOutput.ReadToEndAsync()
    $stderrAsync = $proc.StandardError.ReadToEndAsync()

    if (-not $proc.WaitForExit(120000)) {
        $proc.Kill()
        throw "Gate: child process timed out after 120 s"
    }

    @{
        ExitCode = $proc.ExitCode
        Stdout   = $stdoutAsync.Result
        Stderr   = $stderrAsync.Result
    }
}

# ---------------------------------------------------------------------------
# Helper: expect a child to fail (exit != 0) with a message containing a keyword.
# Also checks call-log count and that the binary is absent from INSTALL_DIR.
# ---------------------------------------------------------------------------
function Assert-ChildFails {
    param(
        [string]$ScenarioName,
        [hashtable]$Env,
        [string]$Keyword,         # expected substring in the error output
        [int]   $ExpectCalls = -1 # -1 = do not assert
    )

    $childCode = @'
$ErrorActionPreference = 'Stop'

function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    $base = 'https://github.com/kgsaran/trackfw/releases/download/v99.0.0/'
    if ($Uri -notmatch '^https://(api\.github\.com|github\.com)/') {
        throw "Shadow: unexpected URI base: $Uri"
    }
    if ($Uri -match 'api\.github\.com') {
        if ($Uri -notmatch '^https://api\.github\.com/repos/kgsaran/trackfw/releases/latest$') {
            throw "Shadow: unexpected API URI: $Uri"
        }
    } else {
        if (-not $Uri.StartsWith($base)) {
            throw "Shadow: URI does not start with expected base: $Uri (expected: $base)"
        }
    }
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    $sd = $env:GATE_SERVE_DIR
    if ($Uri -match 'api\.github\.com') {
        Copy-Item (Join-Path $sd 'api.json') $OutFile -Force
    } elseif ($Uri -match '[.]tar[.]gz$') {
        $file = ([Uri]$Uri).Segments[-1]
        Copy-Item (Join-Path $sd $file) $OutFile -Force
    } elseif ($Uri -match 'checksums[.]txt$') {
        Copy-Item (Join-Path $sd 'checksums.txt') $OutFile -Force
    } else {
        throw "Shadow: unknown URI: $Uri"
    }
}

$ok   = $false
$msg  = ''
try {
    . $env:GATE_INSTALL_SCRIPT
} catch {
    $ok  = $true
    $msg = $_.Exception.Message
}
if ($ok) {
    Write-Output "EXPECTED_FAILURE: $msg"
    exit 0
} else {
    Write-Output 'UNEXPECTED_SUCCESS'
    exit 1
}
'@

    $callLog = Join-Path ([IO.Path]::GetTempPath()) ("gate-calls-$([IO.Path]::GetRandomFileName()).txt")
    try {
        $mergedEnv = @{ GATE_INSTALL_SCRIPT = $InstallScript; GATE_CALL_LOG = $callLog }
        foreach ($kv in $Env.GetEnumerator()) { $mergedEnv[$kv.Key] = $kv.Value }

        $r = Invoke-PSChild -Code $childCode -Env $mergedEnv

        # (a) Exit code
        if ($r.ExitCode -ne 0) {
            Fail-Scenario $ScenarioName "child exited $($r.ExitCode); stdout=$($r.Stdout.Trim()); stderr=$($r.Stderr.Trim())"
            return
        }
        $out = $r.Stdout + $r.Stderr
        # (b) keyword in output
        if ($out -notmatch [regex]::Escape($Keyword)) {
            Fail-Scenario $ScenarioName "expected keyword '$Keyword' not found in output: $($out.Trim())"
            return
        }
        # (c) call-log count
        if ($ExpectCalls -ge 0) {
            $calls = 0
            if (Test-Path $callLog) { $calls = (Get-Content $callLog | Where-Object { $_ -ne '' }).Count }
            if ($calls -ne $ExpectCalls) {
                Fail-Scenario $ScenarioName "expected $ExpectCalls shadow call(s), got $calls"
                return
            }
        }
        # (d) binary absent from install dir
        if ($Env.ContainsKey('TRACKFW_INSTALL_DIR')) {
            $destBin = Join-Path $Env['TRACKFW_INSTALL_DIR'] 'trackfw.exe'
            if (Test-Path $destBin) {
                Fail-Scenario $ScenarioName "trackfw.exe found in INSTALL_DIR after expected failure"
                return
            }
        }
        Pass-Scenario $ScenarioName

    } finally {
        Remove-Item $callLog -ErrorAction SilentlyContinue
    }
}

# ---------------------------------------------------------------------------
# Helper: build a ustar+gzip tarball with multiple entries.
# Entry names are stored byte-literally (including '../' traversal paths).
# Used by the traversal scenario (GAP 1 of ML-1A audit).
# ---------------------------------------------------------------------------
function New-TarGzMultiEntry {
    param(
        [string]  $OutputPath,
        [object[]]$Entries     # each: @{ Name = '...'; Content = [byte[]] }
    )
    Add-Type -AssemblyName System.IO.Compression

    function New-UstarHdr {
        param([string]$Name, [int]$Size)
        $h  = [byte[]]::new(512)
        $nb = [Text.Encoding]::ASCII.GetBytes($Name)
        [Array]::Copy($nb, 0, $h, 0, [Math]::Min($nb.Length, 99))
        $mo = [Text.Encoding]::ASCII.GetBytes('0000755' + [char]0)
        [Array]::Copy($mo, 0, $h, 100, 8)
        $zi = [Text.Encoding]::ASCII.GetBytes('0000000' + [char]0)
        [Array]::Copy($zi, 0, $h, 108, 8); [Array]::Copy($zi, 0, $h, 116, 8)
        $sz = [Convert]::ToString($Size, 8).PadLeft(11, '0') + [char]0
        [Array]::Copy([Text.Encoding]::ASCII.GetBytes($sz), 0, $h, 124, 12)
        $ep = [DateTimeOffset]::UtcNow - [DateTimeOffset]::new(1970,1,1,0,0,0,[TimeSpan]::Zero)
        $mt = [Convert]::ToString([long]$ep.TotalSeconds, 8).PadLeft(11, '0') + [char]0
        [Array]::Copy([Text.Encoding]::ASCII.GetBytes($mt), 0, $h, 136, 12)
        for ($i = 148; $i -lt 156; $i++) { $h[$i] = 32 }
        $h[156] = [byte][char]'0'
        [Array]::Copy([Text.Encoding]::ASCII.GetBytes('ustar' + [char]0), 0, $h, 257, 6)
        $h[263] = [byte][char]'0'; $h[264] = [byte][char]'0'
        $sum = 0; for ($i = 0; $i -lt 512; $i++) { $sum += $h[$i] }
        $ck = [Convert]::ToString($sum, 8).PadLeft(6, '0') + [char]0 + ' '
        [Array]::Copy([Text.Encoding]::ASCII.GetBytes($ck), 0, $h, 148, 8)
        return $h
    }

    $tarMs = New-Object System.IO.MemoryStream
    foreach ($e in $Entries) {
        $hdr   = New-UstarHdr -Name $e.Name -Size $e.Content.Length
        $tarMs.Write($hdr, 0, 512)
        $padSz = [int]([Math]::Ceiling($e.Content.Length / 512.0) * 512)
        $pad   = [byte[]]::new([Math]::Max($padSz, 0))
        if ($e.Content.Length -gt 0) { [Array]::Copy($e.Content, 0, $pad, 0, $e.Content.Length) }
        $tarMs.Write($pad, 0, $pad.Length)
    }
    $eoa = [byte[]]::new(1024); $tarMs.Write($eoa, 0, 1024)
    $tarBytes = $tarMs.ToArray(); $tarMs.Dispose()

    $gzMs = New-Object System.IO.MemoryStream
    $gz   = New-Object System.IO.Compression.GzipStream($gzMs, [IO.Compression.CompressionMode]::Compress, $true)
    $gz.Write($tarBytes, 0, $tarBytes.Length); $gz.Close()
    [IO.File]::WriteAllBytes($OutputPath, $gzMs.ToArray())
    $gzMs.Dispose()
}

# ===========================================================================
# SCENARIO 1: Static check -- no banned tokens in install.ps1
# ===========================================================================
Write-Host ''
Write-Host '--- static-check ---'

# Case-insensitive pattern per Wave 0 threat model.
# Mixed-case 'Irm' matches '\birm\b' with -match (case-insensitive by default).
$bannedPattern = 'TRACKFW_BASE_URL|TRACKFW_MIRROR|TRACKFW_INSTALL_DRYRUN|Microsoft\.PowerShell\.Utility\\|WebClient|HttpClient|BitsTransfer|Invoke-RestMethod|\birm\b|curl\.exe|wget\.exe'

$hits = Select-String -Path $InstallScript -Pattern $bannedPattern
if ($hits) {
    Fail-Scenario 'static-check-clean' "Banned token in install.ps1: $($hits[0].Line.Trim())"
} else {
    Pass-Scenario 'static-check-clean'
}

# Sabotage arm: inject 'Irm' (mixed case, matches '\birm\b') and verify Select-String catches it
$tmpSabotage = Join-Path ([IO.Path]::GetTempPath()) ("install_sab_$([IO.Path]::GetRandomFileName()).ps1")
try {
    ((Get-Content $InstallScript -Raw) + "`n# gate-sabotage: Irm https://evil.example.com`n") |
        Set-Content $tmpSabotage -Encoding ASCII
    $hits2 = Select-String -Path $tmpSabotage -Pattern $bannedPattern
    if ($hits2) { Pass-Scenario 'static-check-sabotage' }
    else        { Fail-Scenario 'static-check-sabotage' 'Select-String failed to detect injected Irm token' }
} finally {
    Remove-Item $tmpSabotage -ErrorAction SilentlyContinue
}

# Sabotage arm: sab-shadow-not-invoked
# Inject Microsoft.PowerShell.Utility\ qualification on the first Invoke-WebRequest call.
# The static-check-clean scenario must detect the banned token.
# Affirms: if install.ps1 uses the module-qualified form, the shadow in the child process
# is bypassed (real network call); static-check-clean catches this before any run.
Write-Host ''
Write-Host '--- sab-shadow-not-invoked ---'
$tmpSabShadow = Join-Path ([IO.Path]::GetTempPath()) ("install_sab_sw_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabSwOrig = Get-Content $InstallScript -Raw
    $sabSwNew  = $sabSwOrig.Replace(
        'Invoke-WebRequest -Uri $apiUrl -OutFile $tmpJson -UseBasicParsing',
        'Microsoft.PowerShell.Utility\Invoke-WebRequest -Uri $apiUrl -OutFile $tmpJson -UseBasicParsing'
    )
    if ($sabSwOrig -eq $sabSwNew) {
        Fail-Scenario 'sab-shadow-not-invoked-nonvacuous' `
            'anchor not found in install.ps1 -- verify Invoke-WebRequest -Uri $apiUrl call'
    } else {
        [IO.File]::WriteAllText($tmpSabShadow, $sabSwNew, [Text.Encoding]::ASCII)
        $hitsSwSab = Select-String -Path $tmpSabShadow -Pattern $bannedPattern
        if ($hitsSwSab) {
            Pass-Scenario 'sab-shadow-not-invoked'
        } else {
            Fail-Scenario 'sab-shadow-not-invoked' `
                'static-check failed to detect Microsoft.PowerShell.Utility\ in sabotaged copy'
        }
    }
} finally {
    Remove-Item $tmpSabShadow -ErrorAction SilentlyContinue
}

# ===========================================================================
# SCENARIOS 2-6: Version and arch validation
# These fail before the INSTALL_DIR check and before any network call.
# Runnable on macOS and Windows.
# ===========================================================================
Write-Host ''
Write-Host '--- version and arch validation ---'

# Helper: run install.ps1 with an env var set; assert it throws before any download.
# The shadow is included to count calls; version/arch failures must reach 0 calls.
function Assert-ValidationFail {
    param(
        [string]$ScenarioName,
        [hashtable]$EnvVars,
        [string]$Keyword
    )
    $childCode = @'
$ErrorActionPreference = 'Stop'

function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    throw "Shadow: Invoke-WebRequest must not be called in validation-failure scenario: $Uri"
}

$ok  = $false
$msg = ''
try {
    . $env:GATE_INSTALL_SCRIPT
} catch {
    $ok  = $true
    $msg = $_.Exception.Message
}
if ($ok) {
    Write-Output "EXPECTED_FAILURE: $msg"
    exit 0
} else {
    Write-Output 'UNEXPECTED_SUCCESS'
    exit 1
}
'@
    $callLog = Join-Path ([IO.Path]::GetTempPath()) ("gate-val-$([IO.Path]::GetRandomFileName()).txt")
    try {
        $env2 = @{ GATE_INSTALL_SCRIPT = $InstallScript; GATE_CALL_LOG = $callLog }
        foreach ($kv in $EnvVars.GetEnumerator()) { $env2[$kv.Key] = $kv.Value }
        $r = Invoke-PSChild -Code $childCode -Env $env2
        if ($r.ExitCode -ne 0) {
            Fail-Scenario $ScenarioName "child exited $($r.ExitCode); stderr=$($r.Stderr.Trim())"
            return
        }
        $out = $r.Stdout + $r.Stderr
        if ($out -notmatch [regex]::Escape($Keyword)) {
            Fail-Scenario $ScenarioName "expected keyword '$Keyword' not found; output=$($out.Trim())"
            return
        }
        # Assert zero download calls -- validation must fire before any network attempt
        $calls = 0
        if (Test-Path $callLog) { $calls = (Get-Content $callLog | Where-Object { $_ -ne '' }).Count }
        if ($calls -ne 0) {
            Fail-Scenario $ScenarioName "expected 0 download calls, got $calls"
            return
        }
        Pass-Scenario $ScenarioName
    } finally {
        Remove-Item $callLog -ErrorAction SilentlyContinue
    }
}

# INSTALL_DIR path with ';': the leaf component contains ';' so the check fires first.
# The parent has no ';' so no NTFS write risk if the check regresses on Windows.
$semiInstDir = Join-Path ([IO.Path]::GetTempPath()) ("gate-ids-$([IO.Path]::GetRandomFileName())")
$semiPath    = [IO.Path]::Combine($semiInstDir, 'tf;evil')

# Trailing newline in version (passes ^...$, rejected by \A...\z)
Assert-ValidationFail 'version-invalid-newline'    @{ TRACKFW_VERSION = "11.0.0`n" }      'invalida'
# Semicolon (command injection attempt)
Assert-ValidationFail 'version-invalid-semicolon'  @{ TRACKFW_VERSION = '11.0.0;id' }     'invalida'
# Path traversal attempt
Assert-ValidationFail 'version-invalid-dotdot'     @{ TRACKFW_VERSION = '../11.0.0' }     'invalida'
# Uppercase V (case-sensitive match)
Assert-ValidationFail 'version-invalid-uppercase-v' @{ TRACKFW_VERSION = 'V11.0.0' }      'invalida'
# Invalid arch
Assert-ValidationFail 'arch-invalid'               @{ TRACKFW_ARCH = 'x86' }              'invalido'
# Semicolon in INSTALL_DIR: rejected before any write (F-1, ML-4C).
# VERSION pinned so the API call is skipped and calls=0 is enforceable.
# Counter-arm: c1-intact (legitimate path without ';' installs -- see Windows section).
Assert-ValidationFail 'install-dir-semicolon' `
    @{ TRACKFW_INSTALL_DIR = $semiPath; TRACKFW_VERSION = 'v99.0.0' } `
    'ponto-e-virgula'
# Dir-absence: no write (directory, file, registry) must have occurred before the rejection.
if (Test-Path -LiteralPath $semiInstDir) {
    Fail-Scenario 'install-dir-semicolon-no-dir' "INSTALL_DIR parent was created before rejection: '$semiInstDir'"
    Remove-Item -Recurse -Force $semiInstDir -ErrorAction SilentlyContinue
} else {
    Pass-Scenario 'install-dir-semicolon-no-dir'
}

# ===========================================================================
# SABOTAGE ARMS (cross-platform): version regex and arch validation
# Affirms: version-invalid-* catches via '$raw -cmatch' regex; arch-invalid catches
# via the explicit -cne 'amd64'/arm64 guard. Each arm proves self-falsifying behavior.
# On macOS: the sabotaged script fails at INSTALL_DIR validation (different error), so
# 'invalida'/'invalido' is absent from output -- protection confirmed removed.
# ===========================================================================
Write-Host ''
Write-Host '--- sab-version-regex (cross-platform) ---'
$tmpSabVR = Join-Path ([IO.Path]::GetTempPath()) ("install_sab_vr_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabVROrig = Get-Content $InstallScript -Raw
    $sabVRNew  = $sabVROrig.Replace(
        "(`$raw -cmatch '\Av?[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]{1,3}|-beta[0-9]{1,3})?\z')",
        '($raw -cmatch ' + "'.*')"
    )
    if ($sabVROrig -eq $sabVRNew) {
        Fail-Scenario 'sab-version-regex-nonvacuous' `
            'anchor not found in install.ps1 -- verify line containing $raw -cmatch'
    } else {
        [IO.File]::WriteAllText($tmpSabVR, $sabVRNew, [Text.Encoding]::ASCII)
        $sabVRCode = @'
$ErrorActionPreference = 'Stop'
function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    throw "Shadow: must not be called in validation-failure scenario"
}
$ok = $false; $msg = ''
try { . $env:GATE_INSTALL_SCRIPT } catch { $ok = $true; $msg = $_.Exception.Message }
if ($ok) { Write-Output "EXPECTED_FAILURE: $msg"; exit 0 } else { Write-Output 'UNEXPECTED_SUCCESS'; exit 1 }
'@
        # Pin INSTALL_DIR always:
        #   Windows: valid absolute path -> passes format check -> script proceeds to download -> Shadow
        #   non-Windows: '/tmp/...' (no drive letter) -> fails format check -> 'caminho absoluto'
        # Without this pin, macOS Join-Path $null (LOCALAPPDATA unset) throws 'Path.*null' before
        # reaching format validation, making the positive marker platform-unpredictable.
        $sabVRInstDir = if ($onWindows) {
            Join-Path $env:TEMP ("sab-vr-inst-$([IO.Path]::GetRandomFileName())")
        } else {
            Join-Path ([IO.Path]::GetTempPath()) ("sab-vr-inst-$([IO.Path]::GetRandomFileName())")
        }
        $sabVREnv = @{
            TRACKFW_VERSION        = "11.0.0`n"
            GATE_INSTALL_SCRIPT    = $tmpSabVR
            TRACKFW_NO_MODIFY_PATH = '1'
            TRACKFW_INSTALL_DIR    = $sabVRInstDir
        }
        $sabVRr   = Invoke-PSChild -Code $sabVRCode -Env $sabVREnv
        $sabVROut = $sabVRr.Stdout + $sabVRr.Stderr
        # Positive marker:
        #   non-Windows: '/tmp/...' has no drive letter -> install.ps1 throws 'caminho absoluto'
        #   Windows: valid path, script reaches download, IW shadow throws -> 'Shadow'
        $sabVRMarker = if ($onWindows) { 'Shadow' } else { 'caminho absoluto' }
        if ($sabVROut -notmatch 'invalida' -and $sabVROut -match $sabVRMarker) {
            Pass-Scenario 'sab-version-regex'
        } elseif ($sabVROut -match 'invalida') {
            Fail-Scenario 'sab-version-regex' `
                "keyword 'invalida' present after sabotage -- anchor not applied: $($sabVROut.Trim())"
        } else {
            Fail-Scenario 'sab-version-regex' `
                "positive marker '$sabVRMarker' absent -- unexpected path: $($sabVROut.Trim())"
        }
    }
} finally {
    Remove-Item $tmpSabVR -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host '--- sab-arch-validation (cross-platform) ---'
$tmpSabArch = Join-Path ([IO.Path]::GetTempPath()) ("install_sab_arch_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabArchOrig = Get-Content $InstallScript -Raw
    $sabArchNew  = $sabArchOrig.Replace(
        "(`$env:TRACKFW_ARCH -cne 'amd64' -and `$env:TRACKFW_ARCH -cne 'arm64')",
        '($false)'
    )
    if ($sabArchOrig -eq $sabArchNew) {
        Fail-Scenario 'sab-arch-validation-nonvacuous' `
            'anchor not found -- verify TRACKFW_ARCH guard in install.ps1'
    } else {
        [IO.File]::WriteAllText($tmpSabArch, $sabArchNew, [Text.Encoding]::ASCII)
        $sabArchCode = @'
$ErrorActionPreference = 'Stop'
function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    throw "Shadow: must not be called in validation-failure scenario"
}
$ok = $false; $msg = ''
try { . $env:GATE_INSTALL_SCRIPT } catch { $ok = $true; $msg = $_.Exception.Message }
if ($ok) { Write-Output "EXPECTED_FAILURE: $msg"; exit 0 } else { Write-Output 'UNEXPECTED_SUCCESS'; exit 1 }
'@
        # Pin TRACKFW_VERSION to skip API call (without it, macOS reaches API shadow and gets
        # 'Shadow' marker; Windows does too -- but with VERSION pinned both platforms reach
        # INSTALL_DIR, and the platform-specific INSTALL_DIR pin gives a cleaner branch:
        #   non-Windows: 'caminho absoluto' (no drive letter)
        #   Windows: 'Shadow' (IW shadow at checksums download)
        $sabArchInstDir = if ($onWindows) {
            Join-Path $env:TEMP ("sab-arch-inst-$([IO.Path]::GetRandomFileName())")
        } else {
            Join-Path ([IO.Path]::GetTempPath()) ("sab-arch-inst-$([IO.Path]::GetRandomFileName())")
        }
        $sabArchEnv = @{
            TRACKFW_ARCH           = 'x86'
            TRACKFW_VERSION        = 'v99.0.0'
            GATE_INSTALL_SCRIPT    = $tmpSabArch
            TRACKFW_NO_MODIFY_PATH = '1'
            TRACKFW_INSTALL_DIR    = $sabArchInstDir
        }
        $sabArchR   = Invoke-PSChild -Code $sabArchCode -Env $sabArchEnv
        $sabArchOut = $sabArchR.Stdout + $sabArchR.Stderr
        # Positive marker: same structure as sab-version-regex.
        $sabArchMarker = if ($onWindows) { 'Shadow' } else { 'caminho absoluto' }
        if ($sabArchOut -notmatch 'invalido' -and $sabArchOut -match $sabArchMarker) {
            Pass-Scenario 'sab-arch-validation'
        } elseif ($sabArchOut -match 'invalido') {
            Fail-Scenario 'sab-arch-validation' `
                "keyword 'invalido' present after sabotage -- anchor not applied: $($sabArchOut.Trim())"
        } else {
            Fail-Scenario 'sab-arch-validation' `
                "positive marker '$sabArchMarker' absent -- unexpected path: $($sabArchOut.Trim())"
        }
    }
} finally {
    Remove-Item $tmpSabArch -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host '--- sab-install-dir-semicolon (cross-platform) ---'
$tmpSabIDS = Join-Path ([IO.Path]::GetTempPath()) ("install_sab_ids_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabIDSOrig = Get-Content $InstallScript -Raw
    $sabIDSNew  = $sabIDSOrig.Replace(
        "(`$INSTALL_DIR -match '[;\u037E\uFE54\uFF1B]')",
        '($false)'
    )
    if ($sabIDSOrig -eq $sabIDSNew) {
        Fail-Scenario 'sab-install-dir-semicolon-nonvacuous' `
            'anchor not found in install.ps1 -- verify INSTALL_DIR semicolon/Unicode check'
    } else {
        [IO.File]::WriteAllText($tmpSabIDS, $sabIDSNew, [Text.Encoding]::ASCII)
        $sabIDSCode = @'
$ErrorActionPreference = 'Stop'
function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    throw "Shadow: Invoke-WebRequest must not be called in validation-failure scenario: $Uri"
}
$ok = $false; $msg = ''
try { . $env:GATE_INSTALL_SCRIPT } catch { $ok = $true; $msg = $_.Exception.Message }
if ($ok) { Write-Output "EXPECTED_FAILURE: $msg"; exit 0 } else { Write-Output 'UNEXPECTED_SUCCESS'; exit 1 }
'@
        $sabIDSr = Invoke-PSChild -Code $sabIDSCode -Env @{
            TRACKFW_VERSION     = 'v99.0.0'
            TRACKFW_INSTALL_DIR = $semiPath
            GATE_INSTALL_SCRIPT = $tmpSabIDS
        }
        $sabIDSOut = $sabIDSr.Stdout + $sabIDSr.Stderr
        # macOS: absolute-path check fires on '/tmp/.../tf;evil' (no drive letter) -> 'caminho absoluto'
        # Windows: absolute-path check passes ('C:\...\tf;evil' has drive letter) -> Shadow fires -> 'Shadow'
        $sabIDSMarker = if ($onWindows) { 'Shadow' } else { 'caminho absoluto' }
        if ($sabIDSOut -notmatch 'ponto-e-virgula' -and $sabIDSOut -match $sabIDSMarker) {
            Pass-Scenario 'sab-install-dir-semicolon'
        } elseif ($sabIDSOut -match 'ponto-e-virgula') {
            Fail-Scenario 'sab-install-dir-semicolon' `
                "keyword 'ponto-e-virgula' present after sabotage -- anchor not applied: $($sabIDSOut.Trim())"
        } else {
            Fail-Scenario 'sab-install-dir-semicolon' `
                "positive marker '$sabIDSMarker' absent -- unexpected path: $($sabIDSOut.Trim())"
        }
    }
} finally {
    Remove-Item $tmpSabIDS -ErrorAction SilentlyContinue
    # Backstop: on Windows, New-Item -Force may create the dir if check is missing
    if (Test-Path -LiteralPath $semiInstDir) {
        Remove-Item -Recurse -Force $semiInstDir -ErrorAction SilentlyContinue
    }
}

# ===========================================================================
# A-1 (ML-4E): Unicode variants of ';' in INSTALL_DIR are rejected.
# U+037E GREEK QUESTION MARK, U+FE54 SMALL SEMICOLON, U+FF1B FULLWIDTH SEMICOLON.
# These are best-fit mapped to 0x3B (ASCII ';') in CP1252, so ANSI-legacy apps
# reading PATH via GetEnvironmentVariableA would treat them as path separators.
# The Unicode check fires before the absolute-path check -> cross-platform.
# ===========================================================================
Write-Host ''
Write-Host '--- install-dir-semicolon-unicode (cross-platform) ---'
# Temp parent without Unicode chars; the leaf carries the variant.
# On macOS: /tmp/... (no drive letter); on Windows: C:\...\Temp\...
$ucBase = Join-Path ([IO.Path]::GetTempPath()) ("gate-ids-uc-$([IO.Path]::GetRandomFileName())")
# U+037E GREEK QUESTION MARK -- NFC => U+003B; best-fit CP1252 0x3B.
Assert-ValidationFail 'install-dir-semicolon-unicode-037E' `
    @{ TRACKFW_INSTALL_DIR = [IO.Path]::Combine($ucBase, "tf$([char]0x037E)evil"); TRACKFW_VERSION = 'v99.0.0' } `
    'ponto-e-virgula'
# U+FE54 SMALL SEMICOLON -- NFKC => U+003B; no confirmed CP1252 best-fit.
Assert-ValidationFail 'install-dir-semicolon-unicode-FE54' `
    @{ TRACKFW_INSTALL_DIR = [IO.Path]::Combine($ucBase, "tf$([char]0xFE54)evil"); TRACKFW_VERSION = 'v99.0.0' } `
    'ponto-e-virgula'
# U+FF1B FULLWIDTH SEMICOLON -- NFKC => U+003B; best-fit CP1252 0x3B.
Assert-ValidationFail 'install-dir-semicolon-unicode-FF1B' `
    @{ TRACKFW_INSTALL_DIR = [IO.Path]::Combine($ucBase, "tf$([char]0xFF1B)evil"); TRACKFW_VERSION = 'v99.0.0' } `
    'ponto-e-virgula'

Write-Host ''
Write-Host '--- sab-install-dir-semicolon-unicode (cross-platform) ---'
# Sabotage: revert the Unicode regex to ASCII ';' only.
# Anchor built at runtime ([char]0x5C = '\') to keep this gate source ASCII-clean.
$tmpSabUC = Join-Path ([IO.Path]::GetTempPath()) ("install_sab_uc_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabUCOrig = Get-Content $InstallScript -Raw
    # Anchor: ($INSTALL_DIR -match '[;\u037E\uFE54\uFF1B]')  -- built without literal \uXXXX.
    $bs = [string][char]0x5C
    $sabUCAnchor = "(`$INSTALL_DIR -match '[;" + $bs + "u037E" + $bs + "uFE54" + $bs + "uFF1B]')"
    $sabUCNew    = $sabUCOrig.Replace($sabUCAnchor, "(`$INSTALL_DIR -match ';')")
    if ($sabUCOrig -eq $sabUCNew) {
        Fail-Scenario 'sab-install-dir-semicolon-unicode-nonvacuous' `
            'anchor not found in install.ps1 -- verify Unicode semicolon regex'
    } else {
        Pass-Scenario 'sab-install-dir-semicolon-unicode-nonvacuous'
        [IO.File]::WriteAllText($tmpSabUC, $sabUCNew, [Text.Encoding]::ASCII)
        $sabUCCode = @'
$ErrorActionPreference = 'Stop'
function Invoke-WebRequest {
    [CmdletBinding()]
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    throw "Shadow: Invoke-WebRequest called unexpectedly: $Uri"
}
$ok = $false; $msg = ''
try { . $env:GATE_INSTALL_SCRIPT } catch { $ok = $true; $msg = $_.Exception.Message }
if ($ok) { Write-Output "EXPECTED_FAILURE: $msg"; exit 0 } else { Write-Output 'UNEXPECTED_SUCCESS'; exit 1 }
'@
        # Test with U+FF1B (fullwidth semicolon, CP1252 best-fit confirmed).
        # Same parent dir as positive scenarios.
        $sabUCPath = [IO.Path]::Combine($ucBase, "tf$([char]0xFF1B)evil")
        $sabUCr    = Invoke-PSChild -Code $sabUCCode -Env @{
            TRACKFW_VERSION     = 'v99.0.0'
            TRACKFW_INSTALL_DIR = $sabUCPath
            GATE_INSTALL_SCRIPT = $tmpSabUC
        }
        $sabUCOut  = $sabUCr.Stdout + $sabUCr.Stderr
        # macOS: /tmp/.../tfFF1Bevil has no drive letter -> absolute-path guard fires -> 'caminho absoluto'
        # Windows: C:\...\Temp\...\tfFF1Bevil passes absolute-path check ->
        #          New-Item -Force creates dir -> download shadow fires -> 'Shadow'
        $sabUCMarker = if ($onWindows) { 'Shadow' } else { 'caminho absoluto' }
        if ($sabUCOut -notmatch 'ponto-e-virgula' -and $sabUCOut -match $sabUCMarker) {
            Pass-Scenario 'sab-install-dir-semicolon-unicode'
        } elseif ($sabUCOut -match 'ponto-e-virgula') {
            Fail-Scenario 'sab-install-dir-semicolon-unicode' `
                "keyword 'ponto-e-virgula' still present after sabotage -- regex not reverted: $($sabUCOut.Trim())"
        } else {
            Fail-Scenario 'sab-install-dir-semicolon-unicode' `
                "positive marker '$sabUCMarker' absent -- unexpected output: $($sabUCOut.Trim())"
        }
    }
} finally {
    Remove-Item $tmpSabUC -ErrorAction SilentlyContinue
    # Backstop: on Windows, sabotaged install creates the dir via New-Item -Force.
    if (Test-Path -LiteralPath $ucBase) {
        Remove-Item -Recurse -Force $ucBase -ErrorAction SilentlyContinue
    }
}


# ===========================================================================
# SCENARIOS 7-8: iex survival -- throw vs exit
# install.ps1 must use throw inside & { }, not exit, so that iex callers survive a failure.
# Runnable on macOS and Windows.
# ===========================================================================
Write-Host ''
Write-Host '--- iex throw/exit contract ---'

# Scenario 7: real install.ps1 with an invalid version via iex -> AFTER-IEX must appear
$iexThrowCode = @'
$ErrorActionPreference = 'Stop'
$env:TRACKFW_VERSION = 'V11.0.0'
try { iex (Get-Content $env:GATE_INSTALL_SCRIPT -Raw) } catch {}
Write-Output 'AFTER-IEX'
exit 0
'@
$r7 = Invoke-PSChild -Code $iexThrowCode -Env @{ GATE_INSTALL_SCRIPT = $InstallScript }
if ($r7.Stdout -match 'AFTER-IEX') {
    Pass-Scenario 'iex-throw-survives'
} else {
    Fail-Scenario 'iex-throw-survives' "AFTER-IEX absent; install.ps1 may be using exit instead of throw"
}

# Scenario 8 (sabotage): replace throw with exit 1 -> AFTER-IEX must be ABSENT
# This is the negative baseline proving that exit would be harmful.
$sabotageExitScript = Join-Path ([IO.Path]::GetTempPath()) ("install_exit_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabotaged = (Get-Content $InstallScript -Raw)
    # Replace the specific version-validation throw so exit 1 fires at the same point
    $sabotaged = $sabotaged.Replace(
        'throw "Erro: TRACKFW_VERSION invalida:',
        'exit 1 # GATE_SABOTAGE:'
    )
    [IO.File]::WriteAllText($sabotageExitScript, $sabotaged, [Text.Encoding]::ASCII)

    $iexExitCode = @'
$ErrorActionPreference = 'Stop'
$env:TRACKFW_VERSION = 'V11.0.0'
try { iex (Get-Content $env:GATE_INSTALL_SCRIPT -Raw) } catch {}
Write-Output 'AFTER-IEX'
exit 0
'@
    $r8 = Invoke-PSChild -Code $iexExitCode -Env @{ GATE_INSTALL_SCRIPT = $sabotageExitScript }
    # exit 1 kills the child process; AFTER-IEX should NOT appear
    if ($r8.Stdout -notmatch 'AFTER-IEX') {
        Pass-Scenario 'iex-exit-kills-baseline'
    } else {
        Fail-Scenario 'iex-exit-kills-baseline' "AFTER-IEX appeared despite exit 1 in sabotaged script"
    }
} finally {
    Remove-Item $sabotageExitScript -ErrorAction SilentlyContinue
}

# ===========================================================================
# TRAVERSAL FIXTURE: build + listing verification + macOS counter-arm (GAP 1)
# The fixture contains trackfw.exe and a '../escape-gate-traversal.txt' entry.
# 'tar -tzf' non-vacuity check: name must be stored byte-literally by New-TarGzMultiEntry.
# macOS counter-arm: bsdtar (/usr/bin/tar) is the same libarchive family as System32\tar.exe.
#   - without -P: escape file must NOT appear outside the target dir (stripped/ignored).
#   - with -P: escape file SHOULD appear outside the target dir (fixture is real).
# This is macOS bsdtar evidence, labelled as indication, not direct Windows proof.
# The definitive Windows test (actual install.ps1 run) is in the Windows-only section.
# ===========================================================================
Write-Host ''
Write-Host '--- traversal-fixture-build-and-listing ---'

$travWorkDir     = Join-Path ([IO.Path]::GetTempPath()) ("trav-$([IO.Path]::GetRandomFileName())")
New-Item -ItemType Directory -Path $travWorkDir | Out-Null
$travTarName     = 'trackfw_99.0.0_windows_amd64.tar.gz'
$travFixturePath = Join-Path $travWorkDir $travTarName
# Random names: roadmap requires random suffix to catch hardcoded path assumptions.
$travEscapeName  = "escape-gate-traversal-$([IO.Path]::GetRandomFileName()).txt"
$travAbsName     = "escape-gate-abs-$([IO.Path]::GetRandomFileName()).txt"

try {
    $travFakeExe    = [Text.Encoding]::ASCII.GetBytes('fake-trackfw-exe-traversal-gate-test')
    $travEscapeCont = [Text.Encoding]::ASCII.GetBytes('gate-traversal-escape-proof')
    New-TarGzMultiEntry -OutputPath $travFixturePath -Entries @(
        @{ Name = 'trackfw.exe';              Content = $travFakeExe    },
        @{ Name = "../$travEscapeName";        Content = $travEscapeCont },
        @{ Name = "/$travAbsName";             Content = [Text.Encoding]::ASCII.GetBytes('gate-abs-escape-proof') }
    )

    $travHash = (Get-FileHash $travFixturePath -Algorithm SHA256).Hash.ToLower()
    [IO.File]::WriteAllText((Join-Path $travWorkDir 'checksums.txt'),
        "$travHash  $travTarName`n")
    [IO.File]::WriteAllText((Join-Path $travWorkDir 'api.json'), '{"tag_name":"v99.0.0"}')

    # Non-vacuity: tar -tzf must show the relative escape entry (contains $travEscapeName base)
    if ($onWindows) {
        $travTarBin = Join-Path $env:SystemRoot 'System32\tar.exe'
    } else {
        $travTarBin = '/usr/bin/tar'
    }
    $travListing = & $travTarBin -tzf $travFixturePath 2>&1
    Write-Host "tar -tzf listing (non-vacuity check):"
    foreach ($ln in $travListing) { Write-Host "  $ln" }
    $travListingStr = $travListing -join ''
    if ($travListingStr -match 'escape-gate-traversal' -and $travListingStr -match 'escape-gate-abs') {
        Pass-Scenario 'traversal-fixture-listing-nonvacuous'
    } else {
        Fail-Scenario 'traversal-fixture-listing-nonvacuous' `
            "listing did not show both escape entries. listing=$($travListingStr.Trim())"
    }

    # macOS bsdtar counter-arm (skipped on Windows; Windows test is below)
    if (-not $onWindows) {
        Write-Host ''
        Write-Host '--- macOS bsdtar counter-arm (indicacao, nao prova do Windows) ---'

        # Counter-arm fixture: only '../<escapeName>' -- no absolute member to avoid writing
        # to the filesystem root when -P is used. The absolute member is only in $travFixturePath.
        $travCAOnlyFixturePath = Join-Path $travWorkDir 'ca-only.tar.gz'
        New-TarGzMultiEntry -OutputPath $travCAOnlyFixturePath -Entries @(
            @{ Name = 'trackfw.exe';          Content = $travFakeExe    },
            @{ Name = "../$travEscapeName";    Content = $travEscapeCont }
        )

        $caSafe = Join-Path $travWorkDir 'ca-safe'
        New-Item -ItemType Directory -Path $caSafe | Out-Null
        & $travTarBin -xzf $travCAOnlyFixturePath -C $caSafe 2>&1 | Out-Null
        $travSafeRC = $LASTEXITCODE
        $escapeOutside = Join-Path $travWorkDir $travEscapeName
        $escapeInside  = Join-Path $caSafe      $travEscapeName
        Write-Host "  macOS bsdtar, indicio, nao prova do Windows:"
        Write-Host "  tar -xzf (sem -P) exit: $travSafeRC"
        Write-Host "  tar -xzf (sem -P): escape em travWorkDir/ (fora) = $(Test-Path $escapeOutside)"
        Write-Host "  tar -xzf (sem -P): escape em ca-safe/ (dentro)   = $(Test-Path $escapeInside)"

        $caP = Join-Path $travWorkDir 'ca-p'
        New-Item -ItemType Directory -Path $caP | Out-Null
        & $travTarBin -xzf $travCAOnlyFixturePath -P -C $caP 2>&1 | Out-Null
        # '../<escapeName>' relative to ca-p/ lands in travWorkDir/
        $escapeWithP = Join-Path $travWorkDir $travEscapeName
        Write-Host "  tar -xzf -P (com -P): escape em travWorkDir/ = $(Test-Path $escapeWithP)"
        if (Test-Path $escapeWithP) {
            Pass-Scenario 'traversal-counter-arm-macos'
        } else {
            Fail-Scenario 'traversal-counter-arm-macos' `
                "escape absent with -P -- bsdtar did not honor '../' with -P; fixture may not carry the path entry"
        }
    }
} finally {
    Remove-Item -Recurse -Force $travWorkDir -ErrorAction SilentlyContinue
}

# ===========================================================================
# trtr-culture-probe (cross-platform, A-4/ML-4E)
# Confirms [Threading.Thread] culture override works in the child process and
# that .ToLower() under tr-TR returns dotless i (code 305). Also measures -ieq
# sensitivity so the report can state whether site-2b is a defect fix or a
# consistency change. The probe does NOT run install.ps1 -- it is
# runtime-only and has no Windows-specific dependency.
# ===========================================================================
Write-Host ''
Write-Host '--- trtr-culture-probe ---'
# Confirm the premise: tr-TR culture makes 'I'.ToLower() return dotless i (decimal 305).
# Also measure whether -ieq is culture-sensitive under tr-TR:
#   ieq-Ivan-ivan: 'Ivan' -ieq 'ivan' (both ASCII; should be true regardless of culture)
#   ieq-Ivan-dotless: 'Ivan' -ieq (dotless-i + 'van') (should be false under Ordinal but
#                     may be true if -ieq uses CurrentCulture in this PS build)
$trTRProbeCode = @'
[Threading.Thread]::CurrentThread.CurrentCulture = [Globalization.CultureInfo]::GetCultureInfo('tr-TR')
$v = [int][char]('I'.ToLower()[0])
$ieqAscii   = ('Ivan' -ieq 'ivan')
$ieqDotless = ('Ivan' -ieq ([char]0x131 + 'van'))
Write-Output "dotless-i=$v"
Write-Output "ieq-Ivan-ivan=$ieqAscii"
Write-Output "ieq-Ivan-dotless=$ieqDotless"
'@
$rProbe = Invoke-PSChild -Code $trTRProbeCode -Env @{}
$probeOut = $rProbe.Stdout + $rProbe.Stderr
Write-Host "    probe output: $($probeOut.Trim())"
if ($probeOut -match 'dotless-i=305') {
    Pass-Scenario 'trtr-culture-probe'
} else {
    Fail-Scenario 'trtr-culture-probe' "Expected dotless-i=305; got: $($probeOut.Trim())"
}

# ===========================================================================
# Windows-only scenarios
# C2-C5 require a valid Windows INSTALL_DIR and tar.exe.
# PATH and registry tests require the Windows registry.
# Windows sabotage arms: each listed below with named SKIP when not on Windows.
# ===========================================================================
if (-not $onWindows) {
    Write-Host ''
    Write-Host "Skipping Windows-only scenarios (not on Windows):"
    Write-Host "  SKIP  c2-tampered (Windows)"
    Write-Host "  SKIP  c3-absent (Windows)"
    Write-Host "  SKIP  c4-duplicate (Windows)"
    Write-Host "  SKIP  c5-wrong-hash (Windows)"
    Write-Host "  SKIP  c1-intact (Windows)"
    Write-Host "  SKIP  no-modify-path (Windows)"
    Write-Host "  SKIP  path-already-present (Windows)"
    Write-Host "  SKIP  reg-expand-sz (Windows + CI)"
    Write-Host "  SKIP  traversal-windows-nonvacuous (Windows)"
    Write-Host "  SKIP  traversal-windows (Windows)"
    Write-Host "  SKIP  traversal-counter-arm-windows (Windows)"
    Write-Host "  SKIP  sab-checksum-comparison (Windows)"
    Write-Host "  SKIP  sab-missing-entry-check (Windows)"
    Write-Host "  SKIP  sab-duplicate-check (Windows)"
    Write-Host "  SKIP  sab-path-idempotence (Windows)"
    Write-Host "  SKIP  sab-no-modify-path (Windows)"
    Write-Host "  SKIP  sab-reg-expand-sz (Windows + CI)"
    Write-Host "  SKIP  path-trailing-semicolon (Windows)"
    Write-Host "  SKIP  path-trailing-semicolon-idempotent (Windows)"
    Write-Host "  SKIP  sab-trailing-semicolon-nonvacuous (Windows)"
    Write-Host "  SKIP  sab-trailing-semicolon (Windows)"
    Write-Host "  SKIP  gitbash-bin-warning (Windows)"
    Write-Host "  SKIP  gitbash-bin-no-warning (Windows)"
    Write-Host "  SKIP  sab-gitbash-bin-warning-nonvacuous (Windows)"
    Write-Host "  SKIP  sab-gitbash-bin-warning (Windows)"
    Write-Host "  SKIP  unc-userprofile-nonvacuous (Windows -- A-2: Join-Path probe)"
    Write-Host "  SKIP  unc-userprofile (Windows -- A-2: [IO.Path]::Combine vs Join-Path)"
    Write-Host "  SKIP  sab-unc-userprofile-nonvacuous (Windows -- A-2)"
    Write-Host "  SKIP  sab-unc-userprofile (Windows -- A-2)"
    Write-Host "  SKIP  trtr-ordinal-gitbash (Windows -- A-4: OrdinalIgnoreCase vs ToLower)"
    Write-Host "  SKIP  sab-trtr-tolower-nonvacuous (Windows -- A-4)"
    Write-Host "  SKIP  sab-trtr-tolower (Windows -- A-4)"
    Write-Host ''
    Write-Host "Results: $script:PASS passed, $script:FAIL failed (non-Windows scenarios only)"
    if ($script:FAIL -gt 0) { exit 1 } else { exit 0 }
}

Write-Host ''
Write-Host '--- Windows-only scenarios ---'

# ---------------------------------------------------------------------------
# Temp workspace for Windows fixtures
# ---------------------------------------------------------------------------
$gateWorkDir = Join-Path $env:TEMP "gate-install-ps1-$([IO.Path]::GetRandomFileName())"
New-Item -ItemType Directory -Path $gateWorkDir | Out-Null
$gateInstallDir = Join-Path $gateWorkDir 'install-target'
New-Item -ItemType Directory -Path $gateInstallDir | Out-Null

try {

# ---------------------------------------------------------------------------
# Build fake trackfw.exe with Go (needs its own go.mod to avoid polluting this module)
# ---------------------------------------------------------------------------
Write-Host 'Building fake trackfw.exe...'
$fakeGoDir = Join-Path $gateWorkDir 'fake-exe'
New-Item -ItemType Directory -Path $fakeGoDir | Out-Null
[IO.File]::WriteAllText((Join-Path $fakeGoDir 'go.mod'), "module fakeexe`n`ngo 1.21`n")
[IO.File]::WriteAllText((Join-Path $fakeGoDir 'main.go'), @'
package main

import "fmt"

func main() {
    fmt.Println("trackfw version v99.0.0")
}
'@)
$fakeExePath = Join-Path $fakeGoDir 'trackfw.exe'
Push-Location $fakeGoDir
try {
    & go build -o $fakeExePath .
    if ($LASTEXITCODE -ne 0) { throw "go build fake exe failed" }
} finally { Pop-Location }
Write-Host "Fake exe: $fakeExePath"

# ---------------------------------------------------------------------------
# Helper: build a valid ustar+gzip tarball with a single file entry
# ---------------------------------------------------------------------------
function New-TarGzFixture {
    param(
        [string]$OutputPath,
        [string]$EntryName,
        [byte[]]$EntryContent
    )
    Add-Type -AssemblyName System.IO.Compression

    # Build 512-byte ustar header
    $hdr = [byte[]]::new(512)

    # name (offset 0, 100 bytes)
    $nameBytes = [Text.Encoding]::ASCII.GetBytes($EntryName)
    [Array]::Copy($nameBytes, 0, $hdr, 0, [Math]::Min($nameBytes.Length, 99))

    # mode: 0000755\0 (offset 100, 8 bytes)
    $modeBytes = [Text.Encoding]::ASCII.GetBytes('0000755' + [char]0)
    [Array]::Copy($modeBytes, 0, $hdr, 100, 8)

    # uid/gid: 0000000\0 (offsets 108,116)
    $zeroId = [Text.Encoding]::ASCII.GetBytes('0000000' + [char]0)
    [Array]::Copy($zeroId, 0, $hdr, 108, 8)
    [Array]::Copy($zeroId, 0, $hdr, 116, 8)

    # size (offset 124, 12 bytes): 11 octal digits + null
    $sizeStr  = [Convert]::ToString($EntryContent.Length, 8).PadLeft(11, '0') + [char]0
    $sizeBytes = [Text.Encoding]::ASCII.GetBytes($sizeStr)
    [Array]::Copy($sizeBytes, 0, $hdr, 124, 12)

    # mtime (offset 136, 12 bytes)
    $epoch    = [DateTimeOffset]::UtcNow - [DateTimeOffset]::new(1970,1,1,0,0,0,[TimeSpan]::Zero)
    $mtimeStr = [Convert]::ToString([long]$epoch.TotalSeconds, 8).PadLeft(11, '0') + [char]0
    $mtimeBytes = [Text.Encoding]::ASCII.GetBytes($mtimeStr)
    [Array]::Copy($mtimeBytes, 0, $hdr, 136, 12)

    # checksum placeholder: 8 spaces (offset 148)
    for ($i = 148; $i -lt 156; $i++) { $hdr[$i] = 32 }

    # typeflag: '0' (offset 156)
    $hdr[156] = [byte][char]'0'

    # magic: "ustar\0" (offset 257, 6 bytes)
    $magicBytes = [Text.Encoding]::ASCII.GetBytes('ustar' + [char]0)
    [Array]::Copy($magicBytes, 0, $hdr, 257, 6)

    # version: "00" (offset 263)
    $hdr[263] = [byte][char]'0'
    $hdr[264] = [byte][char]'0'

    # Compute checksum: sum of all header bytes (spaces at 148-155 already set)
    $sum = 0; for ($i = 0; $i -lt 512; $i++) { $sum += $hdr[$i] }
    $chkStr   = [Convert]::ToString($sum, 8).PadLeft(6, '0') + [char]0 + ' '
    $chkBytes = [Text.Encoding]::ASCII.GetBytes($chkStr)
    [Array]::Copy($chkBytes, 0, $hdr, 148, 8)

    # Pad content to 512-byte boundary
    $padSize    = [Math]::Ceiling($EntryContent.Length / 512.0) * 512
    $padContent = [byte[]]::new($padSize)
    [Array]::Copy($EntryContent, 0, $padContent, 0, $EntryContent.Length)

    # End-of-archive: 2 x 512 zero blocks
    $eoa = [byte[]]::new(1024)

    # Assemble tar stream
    $tarMs = New-Object System.IO.MemoryStream
    $tarMs.Write($hdr,        0, $hdr.Length)
    $tarMs.Write($padContent, 0, $padContent.Length)
    $tarMs.Write($eoa,        0, $eoa.Length)
    $tarBytes = $tarMs.ToArray()
    $tarMs.Dispose()

    # Gzip (leaveOpen=true so we can call ToArray on the underlying MemoryStream)
    $gzMs = New-Object System.IO.MemoryStream
    $gz   = New-Object System.IO.Compression.GzipStream($gzMs, [IO.Compression.CompressionMode]::Compress, $true)
    $gz.Write($tarBytes, 0, $tarBytes.Length)
    $gz.Close()
    $result = $gzMs.ToArray()
    $gzMs.Dispose()

    [IO.File]::WriteAllBytes($OutputPath, $result)
}

# ---------------------------------------------------------------------------
# Helper: create a fixture directory for a given scenario variant
# ---------------------------------------------------------------------------
function New-FixtureDir {
    param(
        [string]$BaseDir,
        [string]$Tag    = 'v99.0.0',
        [string]$Arch   = 'amd64',
        [byte[]]$ExeContent = $null,   # $null = use fake trackfw.exe bytes
        [string]$CsMode = 'ok'        # ok | absent | duplicate | wronghash
    )
    $dir = Join-Path $BaseDir ([IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path $dir | Out-Null

    $bare    = $Tag.TrimStart('v')
    $tarName = "trackfw_${bare}_windows_${Arch}.tar.gz"
    $tarPath = Join-Path $dir $tarName

    if ($ExeContent -eq $null) {
        $ExeContent = [IO.File]::ReadAllBytes($fakeExePath)
    }
    New-TarGzFixture -OutputPath $tarPath -EntryName 'trackfw.exe' -EntryContent $ExeContent

    $realHash = (Get-FileHash $tarPath -Algorithm SHA256).Hash

    # api.json
    [IO.File]::WriteAllText((Join-Path $dir 'api.json'), "{`"tag_name`":`"$Tag`"}")

    # checksums.txt
    switch ($CsMode) {
        'absent'    { [IO.File]::WriteAllText((Join-Path $dir 'checksums.txt'), "abcdef0  other_file.tar.gz`n") }
        'duplicate' { [IO.File]::WriteAllText((Join-Path $dir 'checksums.txt'), "$realHash  $tarName`n$realHash  $tarName`n") }
        'wronghash' { [IO.File]::WriteAllText((Join-Path $dir 'checksums.txt'), ('0' * 64) + "  $tarName`n") }
        default     { [IO.File]::WriteAllText((Join-Path $dir 'checksums.txt'), "$realHash  $tarName`n") }
    }

    return @{ Dir = $dir; TarName = $tarName; Hash = $realHash; Tag = $Tag; Arch = $Arch }
}

# ---------------------------------------------------------------------------
# Registry: snapshot PATH before any Windows test so we can restore it.
# Back up raw value AND kind (REG_SZ vs REG_EXPAND_SZ).
# ---------------------------------------------------------------------------
$regKey     = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
$pathBefore = $regKey.GetValue('PATH', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
$pathKindBefore = if ($null -ne $pathBefore) { $regKey.GetValueKind('PATH') } else { $null }

function Restore-RegPath {
    if ($null -eq $pathBefore) {
        try { $regKey.DeleteValue('PATH') } catch {}
    } else {
        $regKey.SetValue('PATH', $pathBefore, $pathKindBefore)
    }
}

try {

# ---------------------------------------------------------------------------
# SCENARIO 9: C2 tampered -- tarball content altered after checksums.txt was built
# (checksums.txt has the correct hash of the ORIGINAL tarball; shadow serves ALTERED tarball)
# ---------------------------------------------------------------------------
Write-Host ''
Write-Host '--- c2-tampered ---'
$f2 = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
# Replace the tarball with VALID content that has a DIFFERENT hash.
# checksums.txt retains the original hash -> divergente fires when hash check runs.
# A valid tarball (not a corrupted gzip) is required so that, with the checksum check
# removed (sab-checksum-comparison), extraction succeeds and the binary is installed.
$tarPath2  = Join-Path $f2.Dir $f2.TarName
$altBytes  = [byte[]]::new([IO.File]::ReadAllBytes($fakeExePath).Length + 1)
[Array]::Copy([IO.File]::ReadAllBytes($fakeExePath), 0, $altBytes, 0, $altBytes.Length - 1)
# altBytes[-1] = 0x00 (extra null byte; different hash, but valid exe structure)
New-TarGzFixture -OutputPath $tarPath2 -EntryName 'trackfw.exe' -EntryContent $altBytes
# checksums.txt still has hash of the original tarball -> divergente

Assert-ChildFails 'c2-tampered' `
    @{ TRACKFW_VERSION = $f2.Tag; TRACKFW_ARCH = $f2.Arch; TRACKFW_INSTALL_DIR = $gateInstallDir;
       TRACKFW_NO_MODIFY_PATH = '1'; GATE_SERVE_DIR = $f2.Dir } `
    'divergente' `
    2

# ---------------------------------------------------------------------------
# SCENARIO 10: C3 absent -- checksums.txt does not contain the tarball name
# ---------------------------------------------------------------------------
Write-Host ''
Write-Host '--- c3-absent ---'
$f3 = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'absent'
Assert-ChildFails 'c3-absent' `
    @{ TRACKFW_VERSION = $f3.Tag; TRACKFW_ARCH = $f3.Arch; TRACKFW_INSTALL_DIR = $gateInstallDir;
       TRACKFW_NO_MODIFY_PATH = '1'; GATE_SERVE_DIR = $f3.Dir } `
    'ausente' `
    2

# ---------------------------------------------------------------------------
# SCENARIO 10: C4 duplicate -- filename appears twice in checksums.txt
# ---------------------------------------------------------------------------
Write-Host '--- c4-duplicate ---'
$f4 = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'duplicate'
Assert-ChildFails 'c4-duplicate' `
    @{ TRACKFW_VERSION = $f4.Tag; TRACKFW_ARCH = $f4.Arch; TRACKFW_INSTALL_DIR = $gateInstallDir;
       TRACKFW_NO_MODIFY_PATH = '1'; GATE_SERVE_DIR = $f4.Dir } `
    'duplicado' `
    2

# ---------------------------------------------------------------------------
# SCENARIO 11: C5 wrong hash -- hash in checksums.txt is all zeros
# ---------------------------------------------------------------------------
Write-Host '--- c5-wrong-hash ---'
$f5 = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'wronghash'
Assert-ChildFails 'c5-wrong-hash' `
    @{ TRACKFW_VERSION = $f5.Tag; TRACKFW_ARCH = $f5.Arch; TRACKFW_INSTALL_DIR = $gateInstallDir;
       TRACKFW_NO_MODIFY_PATH = '1'; GATE_SERVE_DIR = $f5.Dir } `
    'divergente' `
    2

# ---------------------------------------------------------------------------
# SCENARIO 12: C1 intact -- happy path; binary installed and version printed
# ---------------------------------------------------------------------------
Write-Host ''
Write-Host '--- c1-intact ---'
$f1 = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
$c1InstallDir = Join-Path $gateWorkDir 'c1-target'
New-Item -ItemType Directory -Path $c1InstallDir | Out-Null
$c1CallLog = Join-Path $gateWorkDir 'c1-calls.txt'

$c1Code = @'
$ErrorActionPreference = 'Stop'

function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    $base = 'https://github.com/kgsaran/trackfw/releases/download/v99.0.0/'
    if ($Uri -match 'api\.github\.com') {
        if ($Uri -notmatch '^https://api\.github\.com/repos/kgsaran/trackfw/releases/latest$') {
            throw "Shadow: unexpected API URI: $Uri"
        }
    } elseif (-not $Uri.StartsWith($base)) {
        throw "Shadow: URI does not start with expected base: $Uri (expected: $base)"
    }
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    $sd = $env:GATE_SERVE_DIR
    if ($Uri -match 'api\.github\.com') {
        Copy-Item (Join-Path $sd 'api.json') $OutFile -Force
    } elseif ($Uri -match '[.]tar[.]gz$') {
        $file = ([Uri]$Uri).Segments[-1]
        Copy-Item (Join-Path $sd $file) $OutFile -Force
    } elseif ($Uri -match 'checksums[.]txt$') {
        Copy-Item (Join-Path $sd 'checksums.txt') $OutFile -Force
    } else {
        throw "Shadow: unknown URI: $Uri"
    }
}

. $env:GATE_INSTALL_SCRIPT
'@

$r12 = Invoke-PSChild -Code $c1Code -Env @{
    TRACKFW_VERSION      = $f1.Tag
    TRACKFW_ARCH         = $f1.Arch
    TRACKFW_INSTALL_DIR  = $c1InstallDir
    TRACKFW_NO_MODIFY_PATH = '1'
    GATE_SERVE_DIR       = $f1.Dir
    GATE_CALL_LOG        = $c1CallLog
    GATE_INSTALL_SCRIPT  = $InstallScript
}

$c1Passed = $true
if ($r12.ExitCode -ne 0) {
    Fail-Scenario 'c1-intact-exit' "exit $($r12.ExitCode); stderr=$($r12.Stderr.Trim())"
    $c1Passed = $false
}

$destBin = Join-Path $c1InstallDir 'trackfw.exe'
if (-not (Test-Path $destBin)) {
    Fail-Scenario 'c1-intact-binary' "trackfw.exe not found in $c1InstallDir"
    $c1Passed = $false
}

$c1Calls = 0
if (Test-Path $c1CallLog) {
    $c1Calls = (Get-Content $c1CallLog | Where-Object { $_ -ne '' }).Count
}
if ($c1Calls -ne 2) {
    Fail-Scenario 'c1-intact-calls' "expected 2 shadow calls (tarball + checksums), got $c1Calls"
    $c1Passed = $false
}

$c1out = $r12.Stdout
if ($c1out -notmatch 'v99[.]0[.]0') {
    Fail-Scenario 'c1-intact-version' "version v99.0.0 not in output: $($c1out.Trim())"
    $c1Passed = $false
}

if ($c1Passed) { Pass-Scenario 'c1-intact' }

# ---------------------------------------------------------------------------
# SCENARIO 13: TRACKFW_NO_MODIFY_PATH=1 -- PATH registry unchanged
# ---------------------------------------------------------------------------
Write-Host '--- no-modify-path ---'
$f13 = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
$nmpDir = Join-Path $gateWorkDir 'nmp-target'
New-Item -ItemType Directory -Path $nmpDir | Out-Null
$pathSnapshot = $regKey.GetValue('PATH', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)

$nmpCode = $c1Code  # same shadow
$r13 = Invoke-PSChild -Code $nmpCode -Env @{
    TRACKFW_VERSION        = $f13.Tag
    TRACKFW_ARCH           = $f13.Arch
    TRACKFW_INSTALL_DIR    = $nmpDir
    TRACKFW_NO_MODIFY_PATH = '1'
    GATE_SERVE_DIR         = $f13.Dir
    GATE_INSTALL_SCRIPT    = $InstallScript
}
if ($r13.ExitCode -ne 0) {
    Fail-Scenario 'no-modify-path' "child exited $($r13.ExitCode)"
} else {
    $pathAfter = $regKey.GetValue('PATH', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    if ($pathAfter -eq $pathSnapshot) {
        Pass-Scenario 'no-modify-path'
    } else {
        Fail-Scenario 'no-modify-path' 'PATH registry was modified despite TRACKFW_NO_MODIFY_PATH=1'
        Restore-RegPath
    }
}

# ---------------------------------------------------------------------------
# SCENARIO 14: PATH already contains destination -- no duplicate entry added
# ---------------------------------------------------------------------------
Write-Host '--- path-already-present ---'
$f14 = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
$papDir = Join-Path $gateWorkDir 'pap-target'
New-Item -ItemType Directory -Path $papDir | Out-Null

# Pre-insert the target dir into PATH (with trailing backslash to test TrimEnd normalization)
$currentRaw = $regKey.GetValue('PATH', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
$currentKind = if ($null -ne $currentRaw) { $regKey.GetValueKind('PATH') } else { [Microsoft.Win32.RegistryValueKind]::ExpandString }
$newRaw = if ($null -ne $currentRaw -and [string]$currentRaw -ne '') {
    "$currentRaw;$papDir\"   # trailing backslash on purpose
} else {
    "$papDir\"
}
$regKey.SetValue('PATH', $newRaw, $currentKind)

$r14 = Invoke-PSChild -Code $c1Code -Env @{
    TRACKFW_VERSION     = $f14.Tag
    TRACKFW_ARCH        = $f14.Arch
    TRACKFW_INSTALL_DIR = $papDir
    GATE_SERVE_DIR      = $f14.Dir
    GATE_INSTALL_SCRIPT = $InstallScript
}
if ($r14.ExitCode -ne 0) {
    Fail-Scenario 'path-already-present' "child exited $($r14.ExitCode)"
} else {
    $pathAfter14 = [string]$regKey.GetValue('PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $count = ($pathAfter14 -split ';' | Where-Object { $_.TrimEnd('\', '/') -ieq $papDir }).Count
    if ($count -le 1) {
        Pass-Scenario 'path-already-present'
    } else {
        Fail-Scenario 'path-already-present' "INSTALL_DIR appears $count times in PATH (expected <=1)"
    }
}
Restore-RegPath  # restore to pre-scenario state

# ---------------------------------------------------------------------------
# SCENARIO 15: REG_EXPAND_SZ preserved (Windows + CI env only)
# Set PATH as REG_EXPAND_SZ with a %USERPROFILE% reference; verify kind is preserved.
# ---------------------------------------------------------------------------
Write-Host '--- reg-expand-sz ---'
if ($env:CI -eq 'true') {
    $f15    = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
    $rezDir = Join-Path $gateWorkDir 'rez-target'
    New-Item -ItemType Directory -Path $rezDir | Out-Null

    # Set PATH as REG_EXPAND_SZ with an env var reference
    $rezSeedRaw  = '%USERPROFILE%\AppData\Local\Programs\some-other-tool'
    $regKey.SetValue('PATH', $rezSeedRaw, [Microsoft.Win32.RegistryValueKind]::ExpandString)

    $r15 = Invoke-PSChild -Code $c1Code -Env @{
        TRACKFW_VERSION     = $f15.Tag
        TRACKFW_ARCH        = $f15.Arch
        TRACKFW_INSTALL_DIR = $rezDir
        GATE_SERVE_DIR      = $f15.Dir
        GATE_INSTALL_SCRIPT = $InstallScript
    }
    if ($r15.ExitCode -ne 0) {
        Fail-Scenario 'reg-expand-sz' "child exited $($r15.ExitCode)"
    } else {
        $finalKind = $regKey.GetValueKind('PATH')
        if ($finalKind -eq [Microsoft.Win32.RegistryValueKind]::ExpandString) {
            Pass-Scenario 'reg-expand-sz'
        } else {
            Fail-Scenario 'reg-expand-sz' "PATH kind is $finalKind (expected ExpandString)"
        }
    }
    Restore-RegPath
} else {
    Write-Host "  SKIP  reg-expand-sz (CI=true not set; skipped outside CI to avoid registry pollution)"
}

# ===========================================================================
# Windows traversal test (System32\tar.exe) + counter-arm  (GAP 1)
# Fixture: trackfw.exe + '../escape-gate-traversal.txt' (valid tarball, correct checksum).
# Main test: install.ps1 runs with TMP/TEMP overridden; assert escape absent in tmproot.
# Either outcome (installed or aborted) is acceptable; escape-absent is the gate condition.
# Counter-arm: System32\tar.exe -P; assert escape appears (proves fixture is real).
# ===========================================================================
Write-Host ''
Write-Host '--- traversal-windows ---'

$travWinDir     = Join-Path $gateWorkDir 'trav-win'
$travWinTmproot = Join-Path $gateWorkDir 'trav-tmproot'
$travWinInstDir = Join-Path $gateWorkDir 'trav-install-target'
New-Item -ItemType Directory -Path $travWinDir     | Out-Null
New-Item -ItemType Directory -Path $travWinTmproot | Out-Null
New-Item -ItemType Directory -Path $travWinInstDir | Out-Null

# Random names: roadmap requires random suffix so fixed-path assumptions are caught.
$travWinEscapeName = "escape-gate-traversal-$([IO.Path]::GetRandomFileName()).txt"
$travWinAbsName    = "escape-gate-abs-$([IO.Path]::GetRandomFileName()).txt"
# install.ps1 uses VERSION_BARE = VERSION.TrimStart('v') => 99.0.0 (no leading 'v')
$travWinTarName = 'trackfw_99.0.0_windows_amd64.tar.gz'
$travWinFixture = Join-Path $travWinDir $travWinTarName

New-TarGzMultiEntry -OutputPath $travWinFixture -Entries @(
    @{ Name = 'trackfw.exe';               Content = [IO.File]::ReadAllBytes($fakeExePath) },
    @{ Name = "../$travWinEscapeName";      Content = [Text.Encoding]::ASCII.GetBytes('gate-traversal-escape') },
    @{ Name = "/$travWinAbsName";           Content = [Text.Encoding]::ASCII.GetBytes('gate-abs-escape') }
)
$travWinHash = (Get-FileHash $travWinFixture -Algorithm SHA256).Hash.ToLower()
[IO.File]::WriteAllText((Join-Path $travWinDir 'checksums.txt'), "$travWinHash  $travWinTarName`n")
[IO.File]::WriteAllText((Join-Path $travWinDir 'api.json'), '{"tag_name":"v99.0.0"}')

$rTrav = Invoke-PSChild -Code $c1Code -Env @{
    TRACKFW_VERSION        = 'v99.0.0'
    TRACKFW_ARCH           = 'amd64'
    TRACKFW_INSTALL_DIR    = $travWinInstDir
    TRACKFW_NO_MODIFY_PATH = '1'
    GATE_SERVE_DIR         = $travWinDir
    GATE_INSTALL_SCRIPT    = $InstallScript
    TMP                    = $travWinTmproot
    TEMP                   = $travWinTmproot
    TMPDIR                 = $travWinTmproot
}

# Non-vacuity: child must have reached tar extraction (Checksum OK printed before & $tarExe).
# If this is absent, the child aborted early and the escape-absent result is trivially true.
$travChildOut = $rTrav.Stdout + $rTrav.Stderr
if ($travChildOut -notmatch 'Checksum OK') {
    Fail-Scenario 'traversal-windows-nonvacuous' `
        "child output lacks 'Checksum OK' -- tar never ran; escape-absent result would be vacuous. Output: $($travChildOut.Trim())"
} else {
    Pass-Scenario 'traversal-windows-nonvacuous'

    # Use GetPathRoot so the abs-escape check resolves to the correct drive root,
    # regardless of TEMP drive letter on the runner (typically C:\ but not guaranteed).
    $travDriveRoot    = [IO.Path]::GetPathRoot($travWinTmproot)
    $travEscapeFile   = Join-Path $travWinTmproot $travWinEscapeName
    $travAbsFile      = Join-Path $travDriveRoot  $travWinAbsName
    $travBinaryOk     = Test-Path (Join-Path $travWinInstDir 'trackfw.exe')
    $travEscapeAbsent = -not (Test-Path $travEscapeFile)
    $travAbsAbsent    = -not (Test-Path $travAbsFile)
    Write-Host "  traversal outcome: binary-installed=$travBinaryOk; escape-absent=$travEscapeAbsent; abs-absent=$travAbsAbsent; child-exit=$($rTrav.ExitCode)"

    if (-not $travEscapeAbsent) {
        Fail-Scenario 'traversal-windows' `
            "$travWinEscapeName found at '$travEscapeFile' -- System32\tar.exe honored '../'"
    } elseif (-not $travAbsAbsent) {
        Fail-Scenario 'traversal-windows' `
            "$travWinAbsName found at '$travAbsFile' -- System32\tar.exe honored absolute path"
    } else {
        Pass-Scenario 'traversal-windows'
    }
}

# Counter-arm: tar.exe -P; '../...' escape must appear outside inner/ (proves fixture carries the path).
# Uses a separate fixture with only trackfw.exe + '../<escapeName>' -- no absolute member,
# to avoid writing to the filesystem root (C:\) with -P.
# Affirms: the fixture's '../' entry is real; the protection comes from tar behavior (strip by default)
# and/or install.ps1's tar invocation (no -P), not from the fixture lacking the entry.
Write-Host '--- traversal-counter-arm-windows ---'
$travCADir   = Join-Path $gateWorkDir 'trav-ca'
$travCAInner = Join-Path $travCADir 'inner'
New-Item -ItemType Directory -Path $travCAInner | Out-Null
$travCAOnlyFixture = Join-Path $gateWorkDir 'trav-ca-only.tar.gz'
New-TarGzMultiEntry -OutputPath $travCAOnlyFixture -Entries @(
    @{ Name = 'trackfw.exe';          Content = [IO.File]::ReadAllBytes($fakeExePath) },
    @{ Name = "../$travWinEscapeName"; Content = [Text.Encoding]::ASCII.GetBytes('gate-traversal-escape') }
)
$tarExeForCA = Join-Path $env:SystemRoot 'System32\tar.exe'
& $tarExeForCA -xzf $travCAOnlyFixture -P -C $travCAInner 2>&1 | Out-Null
$travCAEscape = Join-Path $travCADir $travWinEscapeName
Write-Host "  counter-arm tar -P exit: $LASTEXITCODE"
Write-Host "  counter-arm escape at travCADir/: $(Test-Path $travCAEscape)"
if (Test-Path $travCAEscape) {
    Pass-Scenario 'traversal-counter-arm-windows'
} else {
    Fail-Scenario 'traversal-counter-arm-windows' `
        "escape absent with -P -- System32\tar.exe may strip '../' even with -P; fixture carrier status unproven"
}

# ===========================================================================
# Windows sabotage arms (GAP 2)
# Pattern: copy install.ps1, targeted mutation, assert diff, run the scenario
# that the protection covers, assert the scenario now reports failure.
# Shared fail-detection child code (same logic as Assert-ChildFails):
# ===========================================================================
Write-Host ''
Write-Host '--- Windows sabotage arms ---'

$sabWinFailCode = @'
$ErrorActionPreference = 'Stop'
function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    $base = 'https://github.com/kgsaran/trackfw/releases/download/v99.0.0/'
    if ($Uri -match 'api\.github\.com') {
        if ($Uri -notmatch '^https://api\.github\.com/repos/kgsaran/trackfw/releases/latest$') {
            throw "Shadow: unexpected API URI: $Uri"
        }
    } elseif (-not $Uri.StartsWith($base)) {
        throw "Shadow: URI does not start with expected base: $Uri"
    }
    if ($env:GATE_CALL_LOG) { Add-Content $env:GATE_CALL_LOG $Uri }
    $sd = $env:GATE_SERVE_DIR
    if ($Uri -match 'api\.github\.com') {
        Copy-Item (Join-Path $sd 'api.json') $OutFile -Force
    } elseif ($Uri -match '[.]tar[.]gz$') {
        $file = ([Uri]$Uri).Segments[-1]
        Copy-Item (Join-Path $sd $file) $OutFile -Force
    } elseif ($Uri -match 'checksums[.]txt$') {
        Copy-Item (Join-Path $sd 'checksums.txt') $OutFile -Force
    } else { throw "Shadow: unknown URI: $Uri" }
}
$ok = $false; $msg = ''
try { . $env:GATE_INSTALL_SCRIPT } catch { $ok = $true; $msg = $_.Exception.Message }
if ($ok) { Write-Output "EXPECTED_FAILURE: $msg"; exit 0 } else { Write-Output 'UNEXPECTED_SUCCESS'; exit 1 }
'@

# ---------------------------------------------------------------------------
# sab-checksum-comparison
# Mutation: remove the hash comparison guard (replace with $false).
# With valid tarball + wrong hash in checksums.txt (c2 fixture):
# protection absent -> extraction succeeds -> binary installed.
# Detection: binary PRESENT after child run (c2-tampered would report FAIL).
# Affirms: c2-tampered catches divergence via the hash comparison, not another mechanism.
# ---------------------------------------------------------------------------
Write-Host '--- sab-checksum-comparison ---'
$sabCSScript = Join-Path $gateWorkDir ("install_sab_cs_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabCSOrig = Get-Content $InstallScript -Raw
    $sabCSNew  = $sabCSOrig.Replace(
        '        if ($actualHash -ine $expectedHash) {',
        '        if ($false) {'
    )
    if ($sabCSOrig -eq $sabCSNew) {
        Fail-Scenario 'sab-checksum-comparison-nonvacuous' `
            'anchor not found -- verify hash comparison line in install.ps1'
    } else {
        $sabCSInstDir = Join-Path $gateWorkDir 'sab-cs-target'
        New-Item -ItemType Directory -Path $sabCSInstDir | Out-Null
        $fSabCS = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'wronghash'
        [IO.File]::WriteAllText($sabCSScript, $sabCSNew, [Text.Encoding]::ASCII)
        $rSabCS = Invoke-PSChild -Code $c1Code -Env @{
            TRACKFW_VERSION        = $fSabCS.Tag
            TRACKFW_ARCH           = $fSabCS.Arch
            TRACKFW_INSTALL_DIR    = $sabCSInstDir
            TRACKFW_NO_MODIFY_PATH = '1'
            GATE_SERVE_DIR         = $fSabCS.Dir
            GATE_INSTALL_SCRIPT    = $sabCSScript
        }
        $sabCSBin = Join-Path $sabCSInstDir 'trackfw.exe'
        if (Test-Path $sabCSBin) {
            Pass-Scenario 'sab-checksum-comparison'
        } else {
            Fail-Scenario 'sab-checksum-comparison' `
                "binary NOT found after removing hash comparison -- check if wronghash tarball is valid"
        }
    }
} finally {
    Remove-Item $sabCSScript -ErrorAction SilentlyContinue
}
Restore-RegPath

# ---------------------------------------------------------------------------
# sab-missing-entry-check
# Mutation: remove matchCount==0 guard (replace with $false).
# With c3 fixture (checksums.txt without tarball name):
# protection absent -> $expectedHash stays $null -> $actualHash -ine $null -> throws 'divergente'
# Detection: keyword 'ausente' ABSENT from output (c3-absent would FAIL on its keyword).
# Affirms: c3-absent specifically catches the missing-entry case via the matchCount==0 guard.
# ---------------------------------------------------------------------------
Write-Host '--- sab-missing-entry-check ---'
$sabMEScript = Join-Path $gateWorkDir ("install_sab_me_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabMEOrig = Get-Content $InstallScript -Raw
    $sabMENew  = $sabMEOrig.Replace(
        '        if ($matchCount -eq 0) {',
        '        if ($false) {'
    )
    if ($sabMEOrig -eq $sabMENew) {
        Fail-Scenario 'sab-missing-entry-check-nonvacuous' `
            'anchor not found -- verify matchCount==0 line in install.ps1'
    } else {
        $sabMEInstDir = Join-Path $gateWorkDir 'sab-me-target'
        New-Item -ItemType Directory -Path $sabMEInstDir | Out-Null
        $fSabME = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'absent'
        [IO.File]::WriteAllText($sabMEScript, $sabMENew, [Text.Encoding]::ASCII)
        $rSabME = Invoke-PSChild -Code $sabWinFailCode -Env @{
            TRACKFW_VERSION        = $fSabME.Tag
            TRACKFW_ARCH           = $fSabME.Arch
            TRACKFW_INSTALL_DIR    = $sabMEInstDir
            TRACKFW_NO_MODIFY_PATH = '1'
            GATE_SERVE_DIR         = $fSabME.Dir
            GATE_INSTALL_SCRIPT    = $sabMEScript
        }
        $sabMEOut = $rSabME.Stdout + $rSabME.Stderr
        # 'ausente' must be absent; 'divergente' must be present (predicted fall-through
        # when matchCount==0 check is removed: $expectedHash=$null, $actualHash -ine $null throws divergente).
        if ($sabMEOut -notmatch 'ausente' -and $sabMEOut -match 'divergente') {
            Pass-Scenario 'sab-missing-entry-check'
        } elseif ($sabMEOut -match 'ausente') {
            Fail-Scenario 'sab-missing-entry-check' `
                "keyword 'ausente' still present after sabotage -- guard was not removed"
        } else {
            Fail-Scenario 'sab-missing-entry-check' `
                "keyword 'divergente' absent -- unexpected fallthrough path: $($sabMEOut.Trim())"
        }
    }
} finally {
    Remove-Item $sabMEScript -ErrorAction SilentlyContinue
}

# ---------------------------------------------------------------------------
# sab-duplicate-check
# Mutation: remove matchCount>1 guard (replace with $false).
# With c4 fixture (both entries same hash):
# protection absent -> both entries parsed, $expectedHash = same hash, comparison passes
# -> extraction succeeds -> binary installed.
# Detection: binary PRESENT (c4-duplicate would FAIL -- keyword absent + binary present).
# Affirms: c4-duplicate catches duplicate entries via the matchCount>1 guard.
# ---------------------------------------------------------------------------
Write-Host '--- sab-duplicate-check ---'
$sabDCScript = Join-Path $gateWorkDir ("install_sab_dc_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabDCOrig = Get-Content $InstallScript -Raw
    $sabDCNew  = $sabDCOrig.Replace(
        '        } elseif ($matchCount -gt 1) {',
        '        } elseif ($false) {'
    )
    if ($sabDCOrig -eq $sabDCNew) {
        Fail-Scenario 'sab-duplicate-check-nonvacuous' `
            'anchor not found -- verify matchCount>1 line in install.ps1'
    } else {
        $sabDCInstDir = Join-Path $gateWorkDir 'sab-dc-target'
        New-Item -ItemType Directory -Path $sabDCInstDir | Out-Null
        $fSabDC = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'duplicate'
        [IO.File]::WriteAllText($sabDCScript, $sabDCNew, [Text.Encoding]::ASCII)
        $rSabDC = Invoke-PSChild -Code $c1Code -Env @{
            TRACKFW_VERSION        = $fSabDC.Tag
            TRACKFW_ARCH           = $fSabDC.Arch
            TRACKFW_INSTALL_DIR    = $sabDCInstDir
            TRACKFW_NO_MODIFY_PATH = '1'
            GATE_SERVE_DIR         = $fSabDC.Dir
            GATE_INSTALL_SCRIPT    = $sabDCScript
        }
        $sabDCBin = Join-Path $sabDCInstDir 'trackfw.exe'
        if (Test-Path $sabDCBin) {
            Pass-Scenario 'sab-duplicate-check'
        } else {
            Fail-Scenario 'sab-duplicate-check' `
                "binary NOT found after removing duplicate guard -- possibly duplicate hashes differ"
        }
    }
} finally {
    Remove-Item $sabDCScript -ErrorAction SilentlyContinue
}
Restore-RegPath

# ---------------------------------------------------------------------------
# sab-path-idempotence
# Mutation: replace alreadyPresent check with $false (always adds to PATH).
# Setup: pre-insert install dir into PATH (with trailing backslash).
# Detection: PATH has 2+ entries for the dir (path-already-present would FAIL).
# Affirms: path-already-present catches duplicates via the alreadyPresent guard.
# ---------------------------------------------------------------------------
Write-Host '--- sab-path-idempotence ---'
$sabPIScript = Join-Path $gateWorkDir ("install_sab_pi_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabPIOrig = Get-Content $InstallScript -Raw
    $sabPINew  = $sabPIOrig.Replace(
        '                if ($alreadyPresent) {',
        '                if ($false) {'
    )
    if ($sabPIOrig -eq $sabPINew) {
        Fail-Scenario 'sab-path-idempotence-nonvacuous' `
            'anchor not found -- verify alreadyPresent check in install.ps1'
    } else {
        $sabPIInstDir = Join-Path $gateWorkDir 'sab-pi-target'
        New-Item -ItemType Directory -Path $sabPIInstDir | Out-Null
        $fSabPI = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
        [IO.File]::WriteAllText($sabPIScript, $sabPINew, [Text.Encoding]::ASCII)

        # Pre-insert sabPIInstDir (with trailing backslash) into PATH
        $piCurrentRaw  = $regKey.GetValue('PATH', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $piCurrentKind = if ($null -ne $piCurrentRaw) { $regKey.GetValueKind('PATH') } else { [Microsoft.Win32.RegistryValueKind]::ExpandString }
        $piNewRaw      = if ($null -ne $piCurrentRaw -and [string]$piCurrentRaw -ne '') {
            "$piCurrentRaw;$sabPIInstDir\"
        } else { "$sabPIInstDir\" }
        $regKey.SetValue('PATH', $piNewRaw, $piCurrentKind)

        $rSabPI = Invoke-PSChild -Code $c1Code -Env @{
            TRACKFW_VERSION        = $fSabPI.Tag
            TRACKFW_ARCH           = $fSabPI.Arch
            TRACKFW_INSTALL_DIR    = $sabPIInstDir
            GATE_SERVE_DIR         = $fSabPI.Dir
            GATE_INSTALL_SCRIPT    = $sabPIScript
        }
        $piPathAfter = [string]$regKey.GetValue('PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $piCount = ($piPathAfter -split ';' | Where-Object { $_.TrimEnd('\','/') -ieq $sabPIInstDir }).Count
        Restore-RegPath
        if ($piCount -ge 2) {
            Pass-Scenario 'sab-path-idempotence'
        } else {
            Fail-Scenario 'sab-path-idempotence' `
                "install dir appears $piCount time(s) in PATH after sabotage (expected >=2)"
        }
    }
} finally {
    Remove-Item $sabPIScript -ErrorAction SilentlyContinue
}
Restore-RegPath

# ---------------------------------------------------------------------------
# sab-no-modify-path
# Mutation: hardcode $noModifyPath = $false (flag ignored).
# Run with TRACKFW_NO_MODIFY_PATH=1; PATH must be modified despite the flag.
# Detection: PATH changed (no-modify-path would FAIL).
# Affirms: no-modify-path catches bypass via the $noModifyPath flag check.
# ---------------------------------------------------------------------------
Write-Host '--- sab-no-modify-path ---'
$sabNMPScript = Join-Path $gateWorkDir ("install_sab_nmp_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabNMPOrig = Get-Content $InstallScript -Raw
    $sabNMPNew  = $sabNMPOrig.Replace(
        '$noModifyPath = $env:TRACKFW_NO_MODIFY_PATH -ceq ' + "'1'",
        '$noModifyPath = $false'
    )
    if ($sabNMPOrig -eq $sabNMPNew) {
        Fail-Scenario 'sab-no-modify-path-nonvacuous' `
            'anchor not found -- verify $noModifyPath assignment in install.ps1'
    } else {
        $sabNMPInstDir = Join-Path $gateWorkDir 'sab-nmp-target'
        New-Item -ItemType Directory -Path $sabNMPInstDir | Out-Null
        $fSabNMP = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
        [IO.File]::WriteAllText($sabNMPScript, $sabNMPNew, [Text.Encoding]::ASCII)
        $nmpSnap = $regKey.GetValue('PATH', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)

        $rSabNMP = Invoke-PSChild -Code $c1Code -Env @{
            TRACKFW_VERSION        = $fSabNMP.Tag
            TRACKFW_ARCH           = $fSabNMP.Arch
            TRACKFW_INSTALL_DIR    = $sabNMPInstDir
            TRACKFW_NO_MODIFY_PATH = '1'
            GATE_SERVE_DIR         = $fSabNMP.Dir
            GATE_INSTALL_SCRIPT    = $sabNMPScript
        }
        $nmpAfter = $regKey.GetValue('PATH', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        Restore-RegPath
        if ($nmpAfter -ne $nmpSnap) {
            Pass-Scenario 'sab-no-modify-path'
        } else {
            Fail-Scenario 'sab-no-modify-path' `
                'PATH unchanged after sabotage -- $noModifyPath = $false may not have taken effect'
        }
    }
} finally {
    Remove-Item $sabNMPScript -ErrorAction SilentlyContinue
}
Restore-RegPath

# ---------------------------------------------------------------------------
# sab-reg-expand-sz (CI=true only)
# Mutation: replace $regKey.SetValue with [Environment]::SetEnvironmentVariable (writes REG_SZ).
# Setup: seed PATH as REG_EXPAND_SZ; run scenario; check kind downgraded to REG_SZ.
# Detection: PATH kind is REG_SZ not ExpandString (reg-expand-sz would FAIL).
# Affirms: reg-expand-sz catches kind downgrade via explicit SetValue with $valueKind.
# ---------------------------------------------------------------------------
Write-Host '--- sab-reg-expand-sz ---'
if ($env:CI -eq 'true') {
    $sabREZScript = Join-Path $gateWorkDir ("install_sab_rez_$([IO.Path]::GetRandomFileName()).ps1")
    try {
        $sabREZOrig = Get-Content $InstallScript -Raw
        $sabREZNew  = $sabREZOrig.Replace(
            '                    $regKey.SetValue(' + "'PATH', `$newPath, `$valueKind)",
            "[Environment]::SetEnvironmentVariable('PATH', `$newPath, 'User')"
        )
        if ($sabREZOrig -eq $sabREZNew) {
            Fail-Scenario 'sab-reg-expand-sz-nonvacuous' `
                'anchor not found -- verify $regKey.SetValue line in install.ps1'
        } else {
            $sabREZInstDir = Join-Path $gateWorkDir 'sab-rez-target'
            New-Item -ItemType Directory -Path $sabREZInstDir | Out-Null
            $fSabREZ = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
            [IO.File]::WriteAllText($sabREZScript, $sabREZNew, [Text.Encoding]::ASCII)
            $regKey.SetValue('PATH', '%USERPROFILE%\AppData\Local\Programs\sab-rez-test', [Microsoft.Win32.RegistryValueKind]::ExpandString)

            $rSabREZ = Invoke-PSChild -Code $c1Code -Env @{
                TRACKFW_VERSION        = $fSabREZ.Tag
                TRACKFW_ARCH           = $fSabREZ.Arch
                TRACKFW_INSTALL_DIR    = $sabREZInstDir
                GATE_SERVE_DIR         = $fSabREZ.Dir
                GATE_INSTALL_SCRIPT    = $sabREZScript
            }
            $rezFinalKind = $regKey.GetValueKind('PATH')
            Restore-RegPath
            if ($rezFinalKind -ne [Microsoft.Win32.RegistryValueKind]::ExpandString) {
                Pass-Scenario 'sab-reg-expand-sz'
            } else {
                Fail-Scenario 'sab-reg-expand-sz' `
                    "PATH kind is still ExpandString after sabotage -- SetEnvironmentVariable may have preserved it"
            }
        }
    } finally {
        Remove-Item $sabREZScript -ErrorAction SilentlyContinue
    }
    Restore-RegPath
} else {
    Write-Host "  SKIP  sab-reg-expand-sz (CI=true not set)"
}

# ---------------------------------------------------------------------------
# path-trailing-semicolon (F-2, ML-4C)
# Seed PATH with a known value ending in ';'; verify the installer does not
# produce ';;' and produces exact expected value; run twice for idempotence.
# ---------------------------------------------------------------------------
Write-Host ''
Write-Host '--- path-trailing-semicolon ---'
$ptsSeedRaw = 'C:\Windows\System32;'   # trailing semicolon
$ptsDir     = Join-Path $gateWorkDir 'pts-target'
New-Item -ItemType Directory -Path $ptsDir | Out-Null
$f_pts  = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
$regKey.SetValue('PATH', $ptsSeedRaw, [Microsoft.Win32.RegistryValueKind]::String)

$r_pts = Invoke-PSChild -Code $c1Code -Env @{
    TRACKFW_VERSION     = $f_pts.Tag
    TRACKFW_ARCH        = $f_pts.Arch
    TRACKFW_INSTALL_DIR = $ptsDir
    GATE_SERVE_DIR      = $f_pts.Dir
    GATE_INSTALL_SCRIPT = $InstallScript
    TRACKFW_NO_MODIFY_PATH = '0'
}
$ptsPathAfter    = [string]$regKey.GetValue('PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
$ptsExpected     = $ptsSeedRaw.TrimEnd(';') + ';' + $ptsDir
if ($r_pts.ExitCode -ne 0) {
    Fail-Scenario 'path-trailing-semicolon' "child exited $($r_pts.ExitCode); stderr=$($r_pts.Stderr.Trim())"
} elseif ($ptsPathAfter -ne $ptsExpected) {
    Fail-Scenario 'path-trailing-semicolon' "PATH after = '$ptsPathAfter'; expected '$ptsExpected'"
} else {
    Pass-Scenario 'path-trailing-semicolon'
}

# Idempotence: second run must not add another entry or re-introduce ';;'
$r_pts2       = Invoke-PSChild -Code $c1Code -Env @{
    TRACKFW_VERSION     = $f_pts.Tag
    TRACKFW_ARCH        = $f_pts.Arch
    TRACKFW_INSTALL_DIR = $ptsDir
    GATE_SERVE_DIR      = $f_pts.Dir
    GATE_INSTALL_SCRIPT = $InstallScript
    TRACKFW_NO_MODIFY_PATH = '0'
}
$ptsPathAfter2 = [string]$regKey.GetValue('PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
if ($r_pts2.ExitCode -ne 0) {
    Fail-Scenario 'path-trailing-semicolon-idempotent' "second run exited $($r_pts2.ExitCode)"
} elseif ($ptsPathAfter2 -ne $ptsExpected) {
    Fail-Scenario 'path-trailing-semicolon-idempotent' "PATH after 2nd run = '$ptsPathAfter2'; expected '$ptsExpected'"
} else {
    Pass-Scenario 'path-trailing-semicolon-idempotent'
}
Restore-RegPath

# Sabotage: remove TrimEnd(';') and verify PATH gets ';;'
Write-Host '--- sab-trailing-semicolon ---'
$sabPTSScript = Join-Path $gateWorkDir ("install_sab_pts_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabPTSOrig = Get-Content $InstallScript -Raw
    # Mutate only the TrimEnd line; $cleanPath stays but equals $existingPath (no trimming)
    $sabPTSNew  = $sabPTSOrig.Replace(
        "                    `$cleanPath = `$existingPath.TrimEnd(';')",
        "                    `$cleanPath = `$existingPath  # sab: TrimEnd removed"
    )
    if ($sabPTSOrig -eq $sabPTSNew) {
        Fail-Scenario 'sab-trailing-semicolon-nonvacuous' `
            'anchor not found -- verify TrimEnd line in install.ps1'
    } else {
        $sabPTSInstDir = Join-Path $gateWorkDir 'sab-pts-target'
        New-Item -ItemType Directory -Path $sabPTSInstDir | Out-Null
        $f_sabPTS = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
        [IO.File]::WriteAllText($sabPTSScript, $sabPTSNew, [Text.Encoding]::ASCII)
        $regKey.SetValue('PATH', 'C:\Windows\System32;', [Microsoft.Win32.RegistryValueKind]::String)
        $rSabPTS = Invoke-PSChild -Code $c1Code -Env @{
            TRACKFW_VERSION        = $f_sabPTS.Tag
            TRACKFW_ARCH           = $f_sabPTS.Arch
            TRACKFW_INSTALL_DIR    = $sabPTSInstDir
            GATE_SERVE_DIR         = $f_sabPTS.Dir
            GATE_INSTALL_SCRIPT    = $sabPTSScript
            TRACKFW_NO_MODIFY_PATH = '0'
        }
        $sabPTSPathAfter = [string]$regKey.GetValue('PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        Restore-RegPath
        # After sabotage: 'C:\Windows\System32;' + ';' + installDir -> ';;' present
        if ($rSabPTS.ExitCode -ne 0) {
            Fail-Scenario 'sab-trailing-semicolon' `
                "child exited $($rSabPTS.ExitCode); stderr=$($rSabPTS.Stderr.Trim())"
        } elseif ($sabPTSPathAfter -match ';;') {
            Pass-Scenario 'sab-trailing-semicolon'
        } else {
            Fail-Scenario 'sab-trailing-semicolon' `
                "double semicolon absent after sabotage (PATH='$sabPTSPathAfter') -- TrimEnd sabotage not applied"
        }
    }
} finally {
    Remove-Item $sabPTSScript -ErrorAction SilentlyContinue
}
Restore-RegPath

# ---------------------------------------------------------------------------
# gitbash-bin-warning (ML-4C): installer warns when USERPROFILE\bin\trackfw.exe
# exists and is not the install destination.
# Windows-only: full install path (registry, tar.exe, absolute INSTALL_DIR) required.
# macOS: named SKIP above -- install.ps1 fails at INSTALL_DIR validation before
#        reaching the USERPROFILE\bin check.
# ---------------------------------------------------------------------------
Write-Host ''
Write-Host '--- gitbash-bin-warning ---'
$gbbFakeProfile = Join-Path $gateWorkDir 'fake-profile'
$gbbFakeBinDir  = Join-Path $gbbFakeProfile 'bin'
New-Item -ItemType Directory -Path $gbbFakeBinDir | Out-Null
# Plant a fake trackfw.exe in the fake ~/bin (simulates an old copy installed by install.sh)
[IO.File]::WriteAllBytes((Join-Path $gbbFakeBinDir 'trackfw.exe'),
    [IO.File]::ReadAllBytes($fakeExePath))

$gbbInstDir  = Join-Path $gateWorkDir 'gbb-target'
New-Item -ItemType Directory -Path $gbbInstDir | Out-Null
$f_gbb  = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'

$rGbb = Invoke-PSChild -Code $c1Code -Env @{
    TRACKFW_VERSION        = $f_gbb.Tag
    TRACKFW_ARCH           = $f_gbb.Arch
    TRACKFW_INSTALL_DIR    = $gbbInstDir
    TRACKFW_NO_MODIFY_PATH = '1'
    GATE_SERVE_DIR         = $f_gbb.Dir
    GATE_INSTALL_SCRIPT    = $InstallScript
    USERPROFILE            = $gbbFakeProfile
}
$gbbOut = $rGbb.Stdout + $rGbb.Stderr
if ($rGbb.ExitCode -ne 0) {
    Fail-Scenario 'gitbash-bin-warning' "child exited $($rGbb.ExitCode); stderr=$($rGbb.Stderr.Trim())"
} elseif ($gbbOut -notmatch 'Git Bash') {
    Fail-Scenario 'gitbash-bin-warning' "keyword 'Git Bash' absent; no warning emitted. output=$($gbbOut.Trim())"
} else {
    Pass-Scenario 'gitbash-bin-warning'
}

# Counter-arm: no bin\trackfw.exe -> no warning
Write-Host '--- gitbash-bin-no-warning ---'
$gbbNoProfile = Join-Path $gateWorkDir 'fake-profile-empty'
New-Item -ItemType Directory -Path $gbbNoProfile | Out-Null
# bin\ dir exists but no trackfw.exe inside
New-Item -ItemType Directory -Path (Join-Path $gbbNoProfile 'bin') | Out-Null

$gbbNoInstDir  = Join-Path $gateWorkDir 'gbb-no-target'
New-Item -ItemType Directory -Path $gbbNoInstDir | Out-Null
$f_gbbNo = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'

$rGbbNo = Invoke-PSChild -Code $c1Code -Env @{
    TRACKFW_VERSION        = $f_gbbNo.Tag
    TRACKFW_ARCH           = $f_gbbNo.Arch
    TRACKFW_INSTALL_DIR    = $gbbNoInstDir
    TRACKFW_NO_MODIFY_PATH = '1'
    GATE_SERVE_DIR         = $f_gbbNo.Dir
    GATE_INSTALL_SCRIPT    = $InstallScript
    USERPROFILE            = $gbbNoProfile
}
$gbbNoOut = $rGbbNo.Stdout + $rGbbNo.Stderr
if ($rGbbNo.ExitCode -ne 0) {
    Fail-Scenario 'gitbash-bin-no-warning' "child exited $($rGbbNo.ExitCode)"
} elseif ($gbbNoOut -match 'Git Bash') {
    Fail-Scenario 'gitbash-bin-no-warning' "keyword 'Git Bash' present when no bin\trackfw.exe: $($gbbNoOut.Trim())"
} else {
    Pass-Scenario 'gitbash-bin-no-warning'
}

# Sabotage: remove the Test-Path check; warning must be absent
Write-Host '--- sab-gitbash-bin-warning ---'
$sabGBBScript = Join-Path $gateWorkDir ("install_sab_gbb_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabGBBOrig = Get-Content $InstallScript -Raw
    $sabGBBNew  = $sabGBBOrig.Replace(
        '(Test-Path -LiteralPath $gitBashBin -PathType Leaf)',
        '($false)'
    )
    if ($sabGBBOrig -eq $sabGBBNew) {
        Fail-Scenario 'sab-gitbash-bin-warning-nonvacuous' `
            'anchor not found -- verify Test-Path -LiteralPath $gitBashBin check in install.ps1'
    } else {
        $sabGBBInstDir = Join-Path $gateWorkDir 'sab-gbb-target'
        New-Item -ItemType Directory -Path $sabGBBInstDir | Out-Null
        $f_sabGBB = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
        [IO.File]::WriteAllText($sabGBBScript, $sabGBBNew, [Text.Encoding]::ASCII)
        $rSabGBB = Invoke-PSChild -Code $c1Code -Env @{
            TRACKFW_VERSION        = $f_sabGBB.Tag
            TRACKFW_ARCH           = $f_sabGBB.Arch
            TRACKFW_INSTALL_DIR    = $sabGBBInstDir
            TRACKFW_NO_MODIFY_PATH = '1'
            GATE_SERVE_DIR         = $f_sabGBB.Dir
            GATE_INSTALL_SCRIPT    = $sabGBBScript
            USERPROFILE            = $gbbFakeProfile
        }
        $sabGBBOut = $rSabGBB.Stdout + $rSabGBB.Stderr
        if ($sabGBBOut -notmatch 'Git Bash') {
            Pass-Scenario 'sab-gitbash-bin-warning'
        } else {
            Fail-Scenario 'sab-gitbash-bin-warning' `
                "keyword 'Git Bash' present after sabotage -- check was not removed: $($sabGBBOut.Trim())"
        }
    }
} finally {
    Remove-Item $sabGBBScript -ErrorAction SilentlyContinue
}
# ===========================================================================
# A-2 (ML-4E): [IO.Path]::Combine does not throw for USERPROFILE paths without
# a recognized PSDrive qualifier (e.g. unmapped drive letter).
# Join-Path (Join-Path ...) threw "value of argument 'drive' is null" on macOS
# with UNC-style paths; behavior on Windows with unmapped letters is probed here.
# macOS SKIP: full installation cannot reach the ~/bin block on macOS (INSTALL_DIR
# format check, New-Item on C:\, missing tar.exe all fail first).
# ===========================================================================
Write-Host ''
Write-Host '--- unc-userprofile ---'
$unmappedLetter = $null
foreach ($L in @('D','E','F','G','H','I','J','K','L','M','N','O','P','Q','R','S','T','U','V','W','X','Y','Z')) {
    if ($null -eq $unmappedLetter -and -not (Test-Path "${L}:\")) { $unmappedLetter = $L }
}
if ($null -eq $unmappedLetter) {
    Write-Host "  SKIP  unc-userprofile-nonvacuous (no unmapped drive found on this host)"
    Write-Host "  SKIP  unc-userprofile (no unmapped drive found on this host)"
    Write-Host "  SKIP  sab-unc-userprofile-nonvacuous (no unmapped drive found)"
    Write-Host "  SKIP  sab-unc-userprofile (no unmapped drive found)"
} else {
    $uncFakeUP = "${unmappedLetter}:\Users\testuser"
    # Probe: does Join-Path throw for this path on this host?
    $jpThrows = $false
    $jpErrMsg = ''
    try { $null = Join-Path $uncFakeUP 'bin' } catch { $jpThrows = $true; $jpErrMsg = $_.Exception.Message }
    if (-not $jpThrows) {
        Write-Host "  SKIP  unc-userprofile-nonvacuous (Join-Path does not throw for '${unmappedLetter}:\' -- A-2 does not reproduce on this host)"
        Write-Host "  SKIP  unc-userprofile (see above)"
        Write-Host "  SKIP  sab-unc-userprofile-nonvacuous (see above)"
        Write-Host "  SKIP  sab-unc-userprofile (see above)"
    } else {
        Pass-Scenario 'unc-userprofile-nonvacuous'
        # Full install: USERPROFILE -> unmapped drive; [IO.Path]::Combine must not throw.
        $uncInstDir = Join-Path $gateWorkDir 'unc-install-target'
        New-Item -ItemType Directory -Path $uncInstDir | Out-Null
        $f_unc = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
        $rUnc = Invoke-PSChild -Code $c1Code -Env @{
            TRACKFW_VERSION        = $f_unc.Tag
            TRACKFW_ARCH           = $f_unc.Arch
            TRACKFW_INSTALL_DIR    = $uncInstDir
            TRACKFW_NO_MODIFY_PATH = '1'
            GATE_SERVE_DIR         = $f_unc.Dir
            GATE_INSTALL_SCRIPT    = $InstallScript
            USERPROFILE            = $uncFakeUP
        }
        $uncOut = $rUnc.Stdout + $rUnc.Stderr
        if ($rUnc.ExitCode -ne 0) {
            Fail-Scenario 'unc-userprofile' "child exited $($rUnc.ExitCode) -- [IO.Path]::Combine may still throw; stderr=$($rUnc.Stderr.Trim())"
        } else {
            Pass-Scenario 'unc-userprofile'
        }
        # Sabotage: revert [IO.Path]::Combine to Join-Path (Join-Path ...); must fail.
        Write-Host '--- sab-unc-userprofile ---'
        $sabUncScript = Join-Path $gateWorkDir ("install_sab_unc_$([IO.Path]::GetRandomFileName()).ps1")
        try {
            $sabUncOrig = Get-Content $InstallScript -Raw
            $sabUncNew  = $sabUncOrig.Replace(
                '            $gitBashBin = [IO.Path]::Combine($env:USERPROFILE, ''bin'', ''trackfw.exe'')',
                '            $gitBashBin = Join-Path (Join-Path $env:USERPROFILE ''bin'') ''trackfw.exe'''
            )
            if ($sabUncOrig -eq $sabUncNew) {
                Fail-Scenario 'sab-unc-userprofile-nonvacuous' `
                    'anchor not found -- verify [IO.Path]::Combine line for gitBashBin'
            } else {
                Pass-Scenario 'sab-unc-userprofile-nonvacuous'
                $sabUncInstDir2 = Join-Path $gateWorkDir 'unc-sab-target'
                New-Item -ItemType Directory -Path $sabUncInstDir2 | Out-Null
                $f_sabUnc = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
                [IO.File]::WriteAllText($sabUncScript, $sabUncNew, [Text.Encoding]::ASCII)
                $rSabUnc = Invoke-PSChild -Code $c1Code -Env @{
                    TRACKFW_VERSION        = $f_sabUnc.Tag
                    TRACKFW_ARCH           = $f_sabUnc.Arch
                    TRACKFW_INSTALL_DIR    = $sabUncInstDir2
                    TRACKFW_NO_MODIFY_PATH = '1'
                    GATE_SERVE_DIR         = $f_sabUnc.Dir
                    GATE_INSTALL_SCRIPT    = $sabUncScript
                    USERPROFILE            = $uncFakeUP
                }
                $sabUncOut = $rSabUnc.Stdout + $rSabUnc.Stderr
                # Derive a keyword from the probe error message (first word, lowercase) to avoid
                # a false-positive from a parse error: a parse error would not match the same
                # keyword that the Join-Path throw produces.
                $jpErrKw = if ($jpErrMsg -match '\b(\w{4,})\b') { $Matches[1].ToLower() } else { '' }
                $markerOk = ($jpErrKw -eq '') -or ($sabUncOut -match [regex]::Escape($jpErrKw))
                if ($rSabUnc.ExitCode -ne 0 -and $markerOk) {
                    Pass-Scenario 'sab-unc-userprofile'
                } elseif ($rSabUnc.ExitCode -eq 0) {
                    Fail-Scenario 'sab-unc-userprofile' `
                        "child exited 0 after sabotage -- Join-Path did not throw for '$uncFakeUP'"
                } else {
                    Fail-Scenario 'sab-unc-userprofile' `
                        "child exited $($rSabUnc.ExitCode) but output did not contain expected marker '$jpErrKw'; output=$($sabUncOut.Trim())"
                }
            }
        } finally {
            Remove-Item $sabUncScript -ErrorAction SilentlyContinue
        }
    }
}

# ===========================================================================
# A-4 (ML-4E): OrdinalIgnoreCase prevents false-positive 'Git Bash' warning
# when USERPROFILE and INSTALL_DIR paths differ only in case (Ivan vs ivan)
# under locale tr-TR where 'I'.ToLower() returns dotless i (U+0131 = 305).
# ===========================================================================

Write-Host ''
Write-Host '--- trtr-ordinal-gitbash ---'
# Setup: USERPROFILE with 'Ivan' (capital I), INSTALL_DIR with 'ivan' (lowercase).
# NTFS resolves both to the same physical dir; the STRING values differ in case.
# With [string]::Equals(OrdinalIgnoreCase): Ivan == ivan -> $same = $true -> no warning.
$trIvanProfile = Join-Path $gateWorkDir 'fake-profile-Ivan'
$trIvanBinDir  = Join-Path $trIvanProfile 'bin'
New-Item -ItemType Directory -Path $trIvanBinDir -Force | Out-Null
[IO.File]::WriteAllBytes((Join-Path $trIvanBinDir 'trackfw.exe'), [IO.File]::ReadAllBytes($fakeExePath))
# INSTALL_DIR uses lowercase 'ivan' (same physical dir on NTFS, different string).
$trIvanInstDir = Join-Path $gateWorkDir 'fake-profile-ivan\bin'
New-Item -ItemType Directory -Path $trIvanInstDir -Force | Out-Null
$f_trTR = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
# Prepend tr-TR culture set to the standard child code.
$trTRCode = "[Threading.Thread]::CurrentThread.CurrentCulture = [Globalization.CultureInfo]::GetCultureInfo('tr-TR')`n" + $c1Code
$rTrTR = Invoke-PSChild -Code $trTRCode -Env @{
    TRACKFW_VERSION        = $f_trTR.Tag
    TRACKFW_ARCH           = $f_trTR.Arch
    TRACKFW_INSTALL_DIR    = $trIvanInstDir
    TRACKFW_NO_MODIFY_PATH = '1'
    GATE_SERVE_DIR         = $f_trTR.Dir
    GATE_INSTALL_SCRIPT    = $InstallScript
    USERPROFILE            = $trIvanProfile
}
$trTROut = $rTrTR.Stdout + $rTrTR.Stderr
if ($rTrTR.ExitCode -ne 0) {
    Fail-Scenario 'trtr-ordinal-gitbash' "child exited $($rTrTR.ExitCode); output=$($trTROut.Trim())"
} elseif ($trTROut -match 'Git Bash') {
    Fail-Scenario 'trtr-ordinal-gitbash' "spurious 'Git Bash' warning under tr-TR when paths differ only in case: $($trTROut.Trim())"
} else {
    Pass-Scenario 'trtr-ordinal-gitbash'
}

Write-Host ''
Write-Host '--- sab-trtr-tolower ---'
# Sabotage: revert OrdinalIgnoreCase to .ToLower(); under tr-TR 'Ivan'.ToLower()='ivan' must trigger warning.
$sabTrTRScript = Join-Path $gateWorkDir ("install_sab_trtr_$([IO.Path]::GetRandomFileName()).ps1")
try {
    $sabTrTROrig = Get-Content $InstallScript -Raw
    $sabTrTRNew  = $sabTrTROrig.Replace(
        "[string]::Equals(`$gitBashBin.TrimEnd('\', '/'), `$destBin.TrimEnd('\', '/'), [System.StringComparison]::OrdinalIgnoreCase)",
        "(`$gitBashBin.TrimEnd('\', '/').ToLower() -eq `$destBin.TrimEnd('\', '/').ToLower())"
    )
    if ($sabTrTROrig -eq $sabTrTRNew) {
        Fail-Scenario 'sab-trtr-tolower-nonvacuous' `
            'anchor not found -- verify [string]::Equals line for $same in install.ps1'
    } else {
        Pass-Scenario 'sab-trtr-tolower-nonvacuous'
        # SAME INSTALL_DIR as the positive arm ($trIvanInstDir, lowercase 'ivan').
        # With .ToLower() under tr-TR: 'Ivan'.ToLower()='ivan' (dotless-i) != 'ivan'.ToLower()='ivan' -> warning.
        # A different INSTALL_DIR would fire the warning regardless of culture -> vacuous.
        $f_sabTrTR = New-FixtureDir -BaseDir $gateWorkDir -CsMode 'ok'
        [IO.File]::WriteAllText($sabTrTRScript, $sabTrTRNew, [Text.Encoding]::ASCII)
        $rSabTrTR = Invoke-PSChild -Code $trTRCode -Env @{
            TRACKFW_VERSION        = $f_sabTrTR.Tag
            TRACKFW_ARCH           = $f_sabTrTR.Arch
            TRACKFW_INSTALL_DIR    = $trIvanInstDir
            TRACKFW_NO_MODIFY_PATH = '1'
            GATE_SERVE_DIR         = $f_sabTrTR.Dir
            GATE_INSTALL_SCRIPT    = $sabTrTRScript
            USERPROFILE            = $trIvanProfile
        }
        $sabTrTROut = $rSabTrTR.Stdout + $rSabTrTR.Stderr
        if ($rSabTrTR.ExitCode -eq 0 -and $sabTrTROut -match 'Git Bash') {
            Pass-Scenario 'sab-trtr-tolower'
        } elseif ($sabTrTROut -notmatch 'Git Bash') {
            Fail-Scenario 'sab-trtr-tolower' `
                "'Git Bash' warning absent after sabotage -- ToLower did not trigger under tr-TR: $($sabTrTROut.Trim())"
        } else {
            Fail-Scenario 'sab-trtr-tolower' `
                "child exited $($rSabTrTR.ExitCode) after sabotage: $($sabTrTROut.Trim())"
        }
    }
} finally {
    Remove-Item $sabTrTRScript -ErrorAction SilentlyContinue
}

Restore-RegPath

} finally {
    # Always restore the registry PATH to its original state
    Restore-RegPath
    $regKey.Close()
    # Backstop: remove absolute-member escape files if System32\tar.exe wrote them
    # (only possible if extracted with --absolute-names, which install.ps1 does NOT do)
    if ($travWinAbsName -and $travWinTmproot) {
        $backstopRoot = [IO.Path]::GetPathRoot($travWinTmproot)
        Remove-Item (Join-Path $backstopRoot $travWinAbsName) -ErrorAction SilentlyContinue
    }
}

} finally {
    # Clean up gate work directory
    Remove-Item -Recurse -Force $gateWorkDir -ErrorAction SilentlyContinue
}

# ===========================================================================
# Final result
# ===========================================================================
Write-Host ''
Write-Host "Results: $script:PASS passed, $script:FAIL failed"
if ($script:FAIL -gt 0) { exit 1 } else { exit 0 }
