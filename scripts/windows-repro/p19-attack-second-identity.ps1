# p19-attack-second-identity.ps1 — sonda de ataque por segunda identidade local não-admin.
#
# Pergunta 19 do windows-probe.yml (ML-W2a-bis, REQ-2026-09-03).
#
# O que esta sonda mede:
#   Com Authenticated Users:(I)(M) herdado de C:\ no diretório do projeto,
#   um usuário local não-admin (p19atk, criado e removido aqui) consegue:
#
#     (i)   sobrescrever ou excluir thirdparty-provenance.json protegido por
#           DACL explícita (fsperm/pathguard)?
#     (ii)  renomear C:\p19-proj e recriar um substituto com conteúdo forjado?
#           Inclui variante: vítima tem processo com cwd em C:\p19-proj.
#     (iii) pré-criar C:\p19-projb\.trackfw (R-A) antes que a vítima escreva
#           a proveniência lá; se a gravação da vítima funcionar, consegue
#           mudar a DACL e sobrescrever?
#     (iv)  junction (R-B): substituir .trackfw por junction para dir próprio
#           enquanto a vítima grava em loop (N=200 iterações). Quantas vezes
#           a gravação segue a junction?
#
# Saída grep-friendly:
#   PROBE w2x <braço> result=<...> detail=<...>
#
# LIMITAÇÕES:
#   - Runner elevado (admin): criamos p19atk sem admin, mas o runner é admin.
#   - Start-Process -Credential requer seclogon; registramos se não disponível.
#   - Junction arm (iv): não é corrida verdadeira — teste sequencial aproximado.
#   - Apenas x64; ARM64 não medido aqui.
#
# NAO corrige nada. Sai 0 sempre.
# -----------------------------------------------------------------------

param(
    [Parameter(Mandatory)][string]$Bin,
    [string]$Workspace = $env:GITHUB_WORKSPACE
)

if (-not $env:RUNNER_TEMP) {
    $env:RUNNER_TEMP = Join-Path ([System.IO.Path]::GetTempPath()) "trackfw-windows-repro"
    New-Item -ItemType Directory -Force -Path $env:RUNNER_TEMP | Out-Null
    Write-Host "P19_RUNNER_TEMP_FALLBACK: $env:RUNNER_TEMP"
}

$ErrorActionPreference = 'Continue'

$currentUser = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
Write-Host "P19_CURRENT_USER=$currentUser"
Write-Host "P19_BIN=$Bin"
Write-Host "P19_WORKSPACE=$Workspace"

# Shared dir: p19atk precisa escrever aqui para que o processo principal leia.
$sharedDir = Join-Path $env:RUNNER_TEMP "p19-shared"
if (Test-Path $sharedDir) { Remove-Item -Recurse -Force $sharedDir }
New-Item -ItemType Directory -Force -Path $sharedDir | Out-Null

# -----------------------------------------------------------------------
# Compilar o helper Go que escreve a proveniência real via
# thirdparty.WriteProvenance (função exportada).
#
# Pacotes sob internal/ só podem ser importados de dentro do mesmo módulo.
# Portanto o helper é criado como um cmd temporário DENTRO do workspace
# ($Workspace/cmd/p19-helper-probe/), compilado de lá, e removido depois.
# Não usa módulo auxiliar externo — usa o go.mod raiz do trackfw.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P19: compilando helper Go (WriteProvenance) ==="

# Dir temporário DENTRO do módulo trackfw para o helper source
$helperDir = Join-Path $Workspace "cmd\p19-helper-probe"
if (Test-Path $helperDir) { Remove-Item -Recurse -Force $helperDir }
New-Item -ItemType Directory -Force -Path $helperDir | Out-Null

$goMain = @'
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/kgsaran/trackfw/internal/thirdparty"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: p19helper <project-root>")
		os.Exit(1)
	}
	root := os.Args[1]

	prov := thirdparty.Provenance{
		Entries: map[string]thirdparty.ProvenanceEntry{
			".trackfw/installed-agent.md": {
				URL:             "https://example.com/p19-probe",
				ChecksumSHA256:  "p19probechecksum0000000000000000000000000000000000000000000000001",
				InstalledSHA256: "p19probechecksum0000000000000000000000000000000000000000000000001",
				InstalledAt:     time.Now().UTC().Format(time.RFC3339),
				ApprovedBy:      "p19-victim",
				ReviewReference: "probe-test-p19",
				Scope:           "project",
			},
		},
	}

	if err := thirdparty.WriteProvenance(root, prov); err != nil {
		fmt.Fprintf(os.Stderr, "P19_HELPER_ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("P19_HELPER_OK root=%s\n", root)
}
'@
Set-Content -Path (Join-Path $helperDir "main.go") -Value $goMain -Encoding UTF8

$helperBin = Join-Path $env:RUNNER_TEMP "p19-helper.exe"
# Compilar a partir do workspace raiz para usar o go.mod do módulo
Push-Location $Workspace
go build -o $helperBin ".\cmd\p19-helper-probe" 2>&1 | Write-Host
$helperBuildExit = $LASTEXITCODE
Pop-Location

# Remover source temporário (não deve entrar em nenhum commit)
Remove-Item -Recurse -Force $helperDir -ErrorAction SilentlyContinue

if ($helperBuildExit -ne 0) {
    Write-Host "P19_GUARD_FAIL: HELPER_BUILD_FAILED exit=$helperBuildExit"
    Write-Host "PROBE w2x setup result=NO_HELPER detail=helper_build_failed_exit=$helperBuildExit"
    exit 0
}
Write-Host "P19_HELPER_BIN=$helperBin"

# -----------------------------------------------------------------------
# Criar usuário não-admin p19atk via New-LocalUser (sem prompt interativo).
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P19: criando usuário local não-admin p19atk ==="

# Gerar senha aleatória forte ≤ 14 chars + complexidade (New-LocalUser aceita
# passwords longas, mas usamos New-LocalUser mesmo assim pois não tem prompt).
$pwBytes = New-Object byte[] 9   # 9 bytes → base64 de 12 chars
[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($pwBytes)
$atkPassword = [Convert]::ToBase64String($pwBytes) + "Aa1"   # 15 chars, complexidade OK
$securePassword = ConvertTo-SecureString $atkPassword -AsPlainText -Force

# Remover se existir de execução anterior
try { Remove-LocalUser -Name "p19atk" -ErrorAction SilentlyContinue } catch {}

try {
    New-LocalUser -Name "p19atk" `
        -Password $securePassword `
        -Description "p19 probe attacker non-admin" `
        -PasswordNeverExpires `
        -AccountNeverExpires `
        -ErrorAction Stop | Out-Null
    Write-Host "P19_USER_CREATED: p19atk (non-admin)"
} catch {
    Write-Host "PROBE w2x setup result=NO_USER detail=$($_.Exception.Message -replace '[\r\n]',' ')"
    exit 0
}
$userCreateExit = 0

# Verificar que p19atk NÃO está em Administrators
$isAdmin = (Get-LocalGroupMember -Group "Administrators" -ErrorAction SilentlyContinue).Name -contains ".\p19atk"
Write-Host "P19_ATK_IS_ADMIN=$isAdmin"
if ($isAdmin) {
    Write-Host "PROBE w2x setup result=ATK_IS_ADMIN detail=unexpected_admin_membership"
    Remove-LocalUser -Name "p19atk" -ErrorAction SilentlyContinue
    exit 0
}

# Garantir que seclogon está rodando (necessário para Start-Process -Credential)
Write-Host "P19_SECLOGON: tentando iniciar serviço seclogon..."
try {
    Start-Service seclogon -ErrorAction SilentlyContinue
    $svc = Get-Service seclogon -ErrorAction SilentlyContinue
    Write-Host "P19_SECLOGON_STATUS=$($svc.Status)"
} catch {
    Write-Host "P19_SECLOGON_ERR=$($_.Exception.Message)"
}

$cred = New-Object PSCredential("p19atk", $securePassword)

# Função auxiliar: executar script ps1 como p19atk e capturar saída
function Invoke-AsAttacker {
    param(
        [string]$ScriptContent,
        [string]$ArmName
    )
    $atkScript = Join-Path $env:RUNNER_TEMP "p19-atk-$ArmName.ps1"
    $atkOut    = Join-Path $sharedDir "$ArmName-out.txt"
    $atkErr    = Join-Path $sharedDir "$ArmName-err.txt"
    Set-Content -Path $atkScript -Value $ScriptContent -Encoding UTF8
    # Garantir que p19atk possa ler o script e escrever no sharedDir
    icacls $sharedDir /grant "p19atk:(OI)(CI)(M)" 2>&1 | Out-Null
    icacls $atkScript /grant "p19atk:(R)" 2>&1 | Out-Null

    try {
        $proc = Start-Process pwsh `
            -Credential $cred `
            -ArgumentList "-NoProfile","-NonInteractive","-File",$atkScript `
            -NoNewWindow `
            -RedirectStandardOutput $atkOut `
            -RedirectStandardError $atkErr `
            -Wait `
            -PassThru `
            -ErrorAction Stop
        $exitCode = $proc.ExitCode
    } catch {
        # Tentar com -LoadUserProfile se Start-Process falhou
        try {
            $proc = Start-Process pwsh `
                -Credential $cred `
                -ArgumentList "-NoProfile","-NonInteractive","-File",$atkScript `
                -NoNewWindow `
                -LoadUserProfile `
                -RedirectStandardOutput $atkOut `
                -RedirectStandardError $atkErr `
                -Wait `
                -PassThru `
                -ErrorAction Stop
            $exitCode = $proc.ExitCode
        } catch {
            Write-Host "PROBE w2x $ArmName result=NO_SECOND_IDENTITY detail=$($_.Exception.Message)"
            return $null
        }
    }

    $outText = if (Test-Path $atkOut) { Get-Content $atkOut -Raw } else { "" }
    $errText = if (Test-Path $atkErr) { Get-Content $atkErr -Raw } else { "" }
    Write-Host "--- P19 [$ArmName] attacker exit=$exitCode ---"
    if ($outText) { Write-Host $outText }
    if ($errText) { Write-Host "STDERR: $errText" }
    return $outText
}

# -----------------------------------------------------------------------
# BRAÇO (i) — overwrite
#   Vítima grava proveniência protegida em C:\p19-proj;
#   atacante tenta sobrescrever e excluir o arquivo.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P19 [overwrite]: vítima grava; atacante tenta sobrescrever/excluir ==="

$projI = "C:\p19-proj"
if (Test-Path $projI) { Remove-Item -Recurse -Force $projI -ErrorAction SilentlyContinue }
New-Item -ItemType Directory -Force -Path $projI | Out-Null
git -C $projI init -q 2>&1 | Out-Null

# Vítima grava proveniência
& $helperBin $projI 2>&1
$victimExitI = $LASTEXITCODE
$provFileI = Join-Path $projI ".trackfw\thirdparty-provenance.json"

Write-Host ""
Write-Host "=== P19 [overwrite]: icacls antes do ataque ==="
icacls $provFileI 2>&1 | Write-Host
icacls (Join-Path $projI ".trackfw") 2>&1 | Write-Host

if ($victimExitI -ne 0 -or -not (Test-Path $provFileI)) {
    Write-Host "PROBE w2x overwrite result=VICTIM_WRITE_FAILED detail=helper_exit=$victimExitI"
} else {
    $atkScriptI = @"
`$prov = "$provFileI"
`$sharedDir = "$sharedDir"

# Tentativa 1: sobrescrever com WriteAllText
try {
    [System.IO.File]::WriteAllText(`$prov, '{"schema_version":2,"entries":{"forged":"ATTACKER"}}')
    "PROBE w2x overwrite result=OVERWRITE_SUCCEEDED detail=WriteAllText_ok" | Out-File (Join-Path `$sharedDir "i-result.txt") -Append
} catch {
    "PROBE w2x overwrite result=OVERWRITE_DENIED detail=`$(`$_.Exception.GetType().Name): `$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "i-result.txt") -Append
}

# Tentativa 2: excluir arquivo
try {
    Remove-Item `$prov -Force -ErrorAction Stop
    "PROBE w2x overwrite-delete result=DELETE_SUCCEEDED" | Out-File (Join-Path `$sharedDir "i-result.txt") -Append
} catch {
    "PROBE w2x overwrite-delete result=DELETE_DENIED detail=`$(`$_.Exception.GetType().Name): `$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "i-result.txt") -Append
}
"@
    $outI = Invoke-AsAttacker -ScriptContent $atkScriptI -ArmName "i"
    $resultsI = if (Test-Path (Join-Path $sharedDir "i-result.txt")) { Get-Content (Join-Path $sharedDir "i-result.txt") } else { @() }
    foreach ($line in $resultsI) { Write-Host $line }
    if (-not $resultsI) {
        Write-Host "PROBE w2x overwrite result=NO_OUTPUT detail=attacker_script_produced_no_lines"
    }
}
Write-Host ""
Write-Host "=== P19 [overwrite]: icacls após ataque ==="
icacls $provFileI 2>&1 | Write-Host
if (Test-Path $provFileI) {
    Write-Host "P19 [overwrite] arquivo ainda existe: $(Get-Item $provFileI | Select-Object -ExpandProperty Length) bytes"
}
Remove-Item -Recurse -Force $projI -ErrorAction SilentlyContinue

# -----------------------------------------------------------------------
# BRAÇO (ii) — swap-root
#   Atacante tenta renomear C:\p19-proj2 e recriar com conteúdo forjado.
#   Variante: vítima tem processo com cwd em C:\p19-proj2.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P19 [swap-root]: atacante tenta renomear raiz do projeto ==="

$projII = "C:\p19-proj2"
if (Test-Path $projII)    { Remove-Item -Recurse -Force $projII -ErrorAction SilentlyContinue }
if (Test-Path "${projII}-old") { Remove-Item -Recurse -Force "${projII}-old" -ErrorAction SilentlyContinue }
New-Item -ItemType Directory -Force -Path $projII | Out-Null
git -C $projII init -q 2>&1 | Out-Null
& $helperBin $projII 2>&1
$victimExitII = $LASTEXITCODE
$provFileII = Join-Path $projII ".trackfw\thirdparty-provenance.json"

Write-Host "P19 [swap-root] victim_write_exit=$victimExitII prov_exists=$(Test-Path $provFileII)"
icacls $projII 2>&1 | Write-Host

$atkScriptII = @"
`$proj = "$projII"
`$old  = "${projII}-old"
`$sharedDir = "$sharedDir"

# Sub-braço 2a: renomear sem processo preso
try {
    Rename-Item `$proj `$old -ErrorAction Stop
    "PROBE w2x swap-root-rename result=RENAME_SUCCEEDED" | Out-File (Join-Path `$sharedDir "ii-result.txt") -Append
    # Tentar recriar com conteúdo forjado
    try {
        New-Item -ItemType Directory -Force (Join-Path `$proj ".trackfw") | Out-Null
        [System.IO.File]::WriteAllText((Join-Path `$proj ".trackfw\thirdparty-provenance.json"), '{"schema_version":2,"entries":{"forged":"ATTACKER"}}')
        "PROBE w2x swap-root-recreate result=RECREATE_SUCCEEDED" | Out-File (Join-Path `$sharedDir "ii-result.txt") -Append
    } catch {
        "PROBE w2x swap-root-recreate result=RECREATE_FAILED detail=`$(`$_.Exception.GetType().Name): `$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "ii-result.txt") -Append
    }
    # Restaurar
    try { Rename-Item `$old `$proj -ErrorAction SilentlyContinue } catch {}
} catch {
    "PROBE w2x swap-root-rename result=RENAME_DENIED detail=`$(`$_.Exception.GetType().Name): `$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "ii-result.txt") -Append
}
"@
$outII = Invoke-AsAttacker -ScriptContent $atkScriptII -ArmName "ii"
$resultsII = if (Test-Path (Join-Path $sharedDir "ii-result.txt")) { Get-Content (Join-Path $sharedDir "ii-result.txt") } else { @() }
foreach ($line in $resultsII) { Write-Host $line }
if (-not $resultsII) {
    Write-Host "PROBE w2x swap-root-rename result=NO_OUTPUT detail=attacker_script_produced_no_lines"
}

# Variante 2b: vítima com processo dormindo em cwd=C:\p19-proj2
Write-Host ""
Write-Host "=== P19 [swap-root-cwd]: variante com processo da vítima com cwd=$projII ==="
if (Test-Path "${projII}-old") { Remove-Item -Recurse -Force "${projII}-old" -ErrorAction SilentlyContinue }
# Garantir que projII existe para o processo dormir lá
if (-not (Test-Path $projII)) { New-Item -ItemType Directory -Force -Path $projII | Out-Null }

$sleepProc = Start-Process pwsh `
    -ArgumentList "-NoProfile","-NonInteractive","-Command","Start-Sleep 30" `
    -WorkingDirectory $projII `
    -NoNewWindow `
    -PassThru `
    -ErrorAction SilentlyContinue

Write-Host "P19 [swap-root-cwd] sleeping_process_pid=$($sleepProc.Id)"

$atkScriptIIb = @"
`$proj = "$projII"
`$old  = "${projII}-old"
`$sharedDir = "$sharedDir"
try {
    Rename-Item `$proj `$old -ErrorAction Stop
    "PROBE w2x swap-root-cwd result=RENAME_SUCCEEDED detail=cwd_process_did_not_prevent" | Out-File (Join-Path `$sharedDir "iib-result.txt") -Append
    try { Rename-Item `$old `$proj -ErrorAction SilentlyContinue } catch {}
} catch {
    "PROBE w2x swap-root-cwd result=RENAME_DENIED detail=`$(`$_.Exception.GetType().Name): `$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "iib-result.txt") -Append
}
"@
$outIIb = Invoke-AsAttacker -ScriptContent $atkScriptIIb -ArmName "iib"
$resultsIIb = if (Test-Path (Join-Path $sharedDir "iib-result.txt")) { Get-Content (Join-Path $sharedDir "iib-result.txt") } else { @() }
foreach ($line in $resultsIIb) { Write-Host $line }
if (-not $resultsIIb) {
    Write-Host "PROBE w2x swap-root-cwd result=NO_OUTPUT detail=attacker_script_produced_no_lines"
}

if ($sleepProc -and -not $sleepProc.HasExited) {
    Stop-Process -Id $sleepProc.Id -Force -ErrorAction SilentlyContinue
}
Remove-Item -Recurse -Force $projII     -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "${projII}-old" -ErrorAction SilentlyContinue

# -----------------------------------------------------------------------
# BRAÇO (iii) — pre-create (R-A)
#   Atacante cria C:\p19-projb\.trackfw antes que a vítima escreva lá.
#   Depois a vítima chama WriteProvenance.
#   Esperado: SetNamedSecurityInfo em .trackfw falha com ACCESS_DENIED
#   (vítima não é dona de .trackfw). Se a gravação funcionar, o atacante
#   tenta mudar a DACL e sobrescrever.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P19 [pre-create]: atacante pré-cria .trackfw antes da vítima ==="

$projIII = "C:\p19-projb"
if (Test-Path $projIII) { Remove-Item -Recurse -Force $projIII -ErrorAction SilentlyContinue }

# Atacante pré-cria C:\p19-projb\.trackfw
$atkScriptIII_pre = @"
`$dir = "$projIII\.trackfw"
`$sharedDir = "$sharedDir"
try {
    New-Item -ItemType Directory -Force -Path `$dir | Out-Null
    "P19_PRE_CREATE_OK dir=`$dir" | Out-File (Join-Path `$sharedDir "iii-pre.txt") -Append
    `$acl = (Get-Acl `$dir).Owner
    "P19_PRE_CREATE_OWNER=`$acl" | Out-File (Join-Path `$sharedDir "iii-pre.txt") -Append
} catch {
    "P19_PRE_CREATE_FAILED detail=`$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "iii-pre.txt") -Append
}
"@

# Atacante precisa de direito de criar diretórios em C:\p19-projb.
# Criar o diretório raiz primeiro (como admin) e garantir Modify para p19atk.
New-Item -ItemType Directory -Force -Path $projIII | Out-Null
git -C $projIII init -q 2>&1 | Out-Null
icacls $projIII /grant "p19atk:(OI)(CI)(M)" 2>&1 | Write-Host
Write-Host "=== P19 [pre-create] icacls C:\p19-projb antes ==="
icacls $projIII 2>&1 | Write-Host

$outIII_pre = Invoke-AsAttacker -ScriptContent $atkScriptIII_pre -ArmName "iii-pre"
$preResults = if (Test-Path (Join-Path $sharedDir "iii-pre.txt")) { Get-Content (Join-Path $sharedDir "iii-pre.txt") } else { @() }
foreach ($line in $preResults) { Write-Host $line }

$trackfwDirIII = Join-Path $projIII ".trackfw"
$preCreated = $preResults -match "P19_PRE_CREATE_OK"
Write-Host "P19 [pre-create] .trackfw pre-criado pelo atacante: $preCreated"

if ($preCreated) {
    Write-Host ""
    Write-Host "=== P19 [pre-create] icacls .trackfw pré-criado ==="
    icacls $trackfwDirIII 2>&1 | Write-Host

    # Vítima tenta gravar proveniência — esperado: falha em SetNamedSecurityInfo
    Write-Host "=== P19 [pre-create]: vítima tenta WriteProvenance em $projIII ==="
    & $helperBin $projIII 2>&1
    $victimExitIII = $LASTEXITCODE
    $provFileIII = Join-Path $projIII ".trackfw\thirdparty-provenance.json"

    Write-Host "P19 [pre-create] victim_write_exit=$victimExitIII prov_exists=$(Test-Path $provFileIII)"
    icacls $provFileIII 2>&1 | Write-Host
    icacls $trackfwDirIII 2>&1 | Write-Host

    if ($victimExitIII -ne 0) {
        Write-Host "PROBE w2x pre-create result=VICTIM_WRITE_BLOCKED detail=helper_exit=$victimExitIII"
    } else {
        Write-Host "PROBE w2x pre-create result=VICTIM_WRITE_SUCCEEDED detail=unexpected_success"
        # Atacante tenta mudar DACL de .trackfw e sobrescrever o arquivo
        $atkScriptIII_post = @"
`$trackfwDir = "$trackfwDirIII"
`$provFile = "$provFileIII"
`$sharedDir = "$sharedDir"

# Tentar mudar DACL de .trackfw (deve falhar: atacante não é dono)
try {
    icacls `$trackfwDir /grant "p19atk:F" 2>&1 | Out-Null
    "PROBE w2x pre-create-dacl result=DACL_CHANGED" | Out-File (Join-Path `$sharedDir "iii-post.txt") -Append
} catch {
    "PROBE w2x pre-create-dacl result=DACL_DENIED detail=`$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "iii-post.txt") -Append
}
# Tentar sobrescrever o arquivo
try {
    [System.IO.File]::WriteAllText(`$provFile, '{"schema_version":2,"entries":{"forged":"ATTACKER_POST"}}')
    "PROBE w2x pre-create-overwrite result=OVERWRITE_SUCCEEDED" | Out-File (Join-Path `$sharedDir "iii-post.txt") -Append
} catch {
    "PROBE w2x pre-create-overwrite result=OVERWRITE_DENIED detail=`$(`$_.Exception.GetType().Name): `$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "iii-post.txt") -Append
}
"@
        $outIII_post = Invoke-AsAttacker -ScriptContent $atkScriptIII_post -ArmName "iii-post"
        $postResults = if (Test-Path (Join-Path $sharedDir "iii-post.txt")) { Get-Content (Join-Path $sharedDir "iii-post.txt") } else { @() }
        foreach ($line in $postResults) { Write-Host $line }
    }
} else {
    Write-Host "PROBE w2x pre-create result=PRE_CREATE_FAILED detail=attacker_could_not_create_trackfw_dir"
}
Remove-Item -Recurse -Force $projIII -ErrorAction SilentlyContinue

# -----------------------------------------------------------------------
# BRAÇO (iv) — junction (R-B), teste sequencial aproximado
#   Atacante remove .trackfw e cria junction para dir próprio.
#   Vítima grava N=200 vezes. Contamos quantas gravações foram para o dir
#   do atacante (i.e., seguiram a junction).
#
#   Nota: não é corrida verdadeira (não há dois threads concorrentes).
#   Testamos se o atacante consegue inserir a junction ANTES de uma
#   gravação da vítima, verificando um único ciclo de: junction → write → check.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P19 [junction]: teste sequencial junction R-B ==="

$projIV = "C:\p19-projc"
$atkJunctionTarget = Join-Path $env:RUNNER_TEMP "p19-junction-target"
if (Test-Path $projIV) { Remove-Item -Recurse -Force $projIV -ErrorAction SilentlyContinue }
if (Test-Path $atkJunctionTarget) { Remove-Item -Recurse -Force $atkJunctionTarget }
New-Item -ItemType Directory -Force -Path $projIV | Out-Null
New-Item -ItemType Directory -Force -Path $atkJunctionTarget | Out-Null
git -C $projIV init -q 2>&1 | Out-Null

# Dar ao atacante Modify no projIV e acesso total ao atkJunctionTarget
icacls $projIV /grant "p19atk:(OI)(CI)(M)" 2>&1 | Out-Null
icacls $atkJunctionTarget /grant "p19atk:(OI)(CI)(F)" 2>&1 | Out-Null

# Fase 1: vítima grava proveniência inicial (cria .trackfw com DACL correta)
& $helperBin $projIV 2>&1
$victimExitIV0 = $LASTEXITCODE
Write-Host "P19 [junction] initial_victim_write exit=$victimExitIV0"
$trackfwDirIV = Join-Path $projIV ".trackfw"
$provFileIV   = Join-Path $projIV ".trackfw\thirdparty-provenance.json"
icacls $trackfwDirIV 2>&1 | Write-Host

if ($victimExitIV0 -ne 0) {
    Write-Host "PROBE w2x junction result=INITIAL_VICTIM_WRITE_FAILED detail=exit=$victimExitIV0"
} else {
    # Fase 2: atacante remove .trackfw e cria junction
    $atkScriptIV = @"
`$proj = "$projIV"
`$junctionTarget = "$atkJunctionTarget"
`$trackfwDir = Join-Path `$proj ".trackfw"
`$sharedDir = "$sharedDir"

# Remover .trackfw (requer Delete Child no pai = Modify herdado)
try {
    Remove-Item -Recurse -Force `$trackfwDir -ErrorAction Stop
    "P19_JCT_REMOVE_OK" | Out-File (Join-Path `$sharedDir "iv-atk.txt") -Append
} catch {
    "P19_JCT_REMOVE_FAILED detail=`$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "iv-atk.txt") -Append
    exit 0
}

# Criar junction .trackfw → atkJunctionTarget
try {
    New-Item -ItemType Junction -Path `$trackfwDir -Target `$junctionTarget -ErrorAction Stop | Out-Null
    "P19_JCT_CREATE_OK junction=`$trackfwDir target=`$junctionTarget" | Out-File (Join-Path `$sharedDir "iv-atk.txt") -Append
} catch {
    "P19_JCT_CREATE_FAILED detail=`$(`$_.Exception.Message -replace '[\r\n]',' ')" | Out-File (Join-Path `$sharedDir "iv-atk.txt") -Append
}
"@
    $outIV = Invoke-AsAttacker -ScriptContent $atkScriptIV -ArmName "iv"
    $jctResults = if (Test-Path (Join-Path $sharedDir "iv-atk.txt")) { Get-Content (Join-Path $sharedDir "iv-atk.txt") } else { @() }
    foreach ($line in $jctResults) { Write-Host $line }

    $jctCreated = $jctResults -match "P19_JCT_CREATE_OK"
    $jctRemoved = $jctResults -match "P19_JCT_REMOVE_OK"

    Write-Host "=== P19 [junction]: icacls .trackfw após junction ==="
    icacls $trackfwDirIV 2>&1 | Write-Host
    $item = Get-Item $trackfwDirIV -Force -ErrorAction SilentlyContinue
    if ($item) {
        Write-Host "P19 [junction] .trackfw Attributes=$($item.Attributes) LinkType=$($item.LinkType)"
    }

    if ($jctCreated) {
        # Fase 3: vítima grava 200 vezes e contamos quantas foram para o dir do atacante
        $junctionWins = 0
        $N = 200
        for ($i = 0; $i -lt $N; $i++) {
            & $helperBin $projIV 2>&1 | Out-Null
        }
        # Contar arquivos no atkJunctionTarget (evidência de gravações via junction)
        $leaked = (Get-ChildItem $atkJunctionTarget -ErrorAction SilentlyContinue).Count
        $junctionWins = $leaked
        Write-Host "PROBE w2x junction result=MEASURED detail=junction_wins=$junctionWins/${N} atk_dir_count=$leaked"
    } elseif ($jctRemoved) {
        Write-Host "PROBE w2x junction result=JUNCTION_CREATE_FAILED detail=remove_ok_but_create_failed"
    } else {
        Write-Host "PROBE w2x junction result=REMOVE_FAILED detail=attacker_could_not_remove_trackfw"
    }
}
Remove-Item -Recurse -Force $projIV         -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $atkJunctionTarget -ErrorAction SilentlyContinue

# -----------------------------------------------------------------------
# Limpeza: remover p19atk
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P19: removendo usuário p19atk ==="
try { Remove-LocalUser -Name "p19atk" -ErrorAction SilentlyContinue; Write-Host "P19_USER_DELETED: p19atk" } catch { Write-Host "P19_USER_DELETE_ERR: $_" }
Remove-Item -Recurse -Force $sharedDir -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Pergunta 19 CONCLUIDA — grep 'PROBE w2x' para resultados dos braços ==="
Write-Host "P19_LIMITES: (1) runner admin — DACL herança pode diferir em perfil não-admin puro; (2) junction arm é sequencial, não corrida verdadeira; (3) p19atk recebeu icacls Modify em projIV/projIII explicitamente para simular herança de C:\; (4) apenas x64; (5) Go helper usa replace directive, needs warm module cache"

# Sonda sem veredito — sai 0.
exit 0
