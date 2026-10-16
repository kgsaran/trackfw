# p17-acl-0600-ntfs.ps1 — sonda de ACL NTFS para arquivos gravados pelo trackfw com 0o600.
#
# Pergunta 17 do windows-probe.yml (ML-W2a, REQ-2026-09-03).
#
# O que esta sonda mede:
#   O Go no Windows ignora os 9 bits POSIX de permissão: os.Chmod / os.WriteFile(…,0o600)
#   só alterna o bit somente-leitura (READONLY attribute). A ACL real do arquivo é herdada
#   do diretório pai em tempo de criação (os.CreateTemp) e não é alterada depois disso.
#
#   Esta sonda mede — em dois braços — quais identidades além do dono/SYSTEM/Administrators
#   recebem Write ou Read no arquivo resultante:
#
#     Braço (i)  home-userprofile — %USERPROFILE%\.trackfw\identity.json
#                (homedir.Dir() sem $HOME → os.UserHomeDir() → %USERPROFILE%)
#     Braço (ii) root-c — C:\p17-proj\.trackfw\integrations-manifest.json
#                (C:\ raiz costuma ter Modify para Authenticated Users herdado)
#
#   Para cada arquivo e diretório pai:
#     1. icacls <path>              — saída bruta
#     2. Get-Acl + FileSystemRights — identifica foreign_write / foreign_read
#     3. Linha PROBE grep-friendly
#
# Saída grep-friendly:
#   PROBE acl <braço> <artefato> owner=<owner> foreign_write=<lista|none>
#   PROBE acl <braço> <artefato-dir> owner=<owner> foreign_write=<lista|none>
#   PROBE acl root <dir-raiz> foreign_write_inheritable=<lista|none>
#
# LIMITAÇÕES MEDIDAS:
#   - Runner x64 é ADMIN elevado; ACLs podem diferir em perfil de usuário padrão.
#   - Apenas Windows x64; ARM64 não medido aqui.
#   - $HOME pode estar definido no runner (ver P17_ENV_HOME abaixo); quando está,
#     homedir.Dir() usa $HOME e não %USERPROFILE% — ambos registrados.
#
# NAO corrige nada. Sai 0 sempre.
# -----------------------------------------------------------------------

param(
    [Parameter(Mandatory)][string]$Bin
)

if (-not $env:RUNNER_TEMP) {
    $env:RUNNER_TEMP = Join-Path ([System.IO.Path]::GetTempPath()) "trackfw-windows-repro"
    New-Item -ItemType Directory -Force -Path $env:RUNNER_TEMP | Out-Null
    Write-Host "P17_RUNNER_TEMP_FALLBACK: $env:RUNNER_TEMP"
}

$ErrorActionPreference = 'Continue'

# -----------------------------------------------------------------------
# Registrar valores de HOME e USERPROFILE no runner antes de qualquer alteração.
# Em pwsh, $HOME é uma variável automática (sempre definida); usar $env:HOME
# para ver a variável de ambiente que o Go lê via os.Getenv("HOME").
# -----------------------------------------------------------------------
$origHome       = $env:HOME
$origUserProfile= $env:USERPROFILE
Write-Host ""
Write-Host "P17_ENV_HOME=$origHome"
Write-Host "P17_ENV_USERPROFILE=$origUserProfile"
Write-Host "P17_NOTE: homedir.Dir() usa os.Getenv('HOME') primeiro; fallback os.UserHomeDir() -> USERPROFILE"
if ($origHome -and $origHome -ne $origUserProfile) {
    Write-Host "P17_NOTE: HOME e USERPROFILE DIVERGEM — braço (i) usará HOME='<temp>' para forçar USERPROFILE"
} elseif ($origHome) {
    Write-Host "P17_NOTE: HOME definido e igual a USERPROFILE"
} else {
    Write-Host "P17_NOTE: HOME nao definido — os.UserHomeDir() usara USERPROFILE"
}

# -----------------------------------------------------------------------
# Get-ForeignAccess: identifica ACEs Allow que concedem Write ou Read a
# identidades que NÃO sejam dono/SYSTEM/Administrators/CREATOR OWNER/usuário atual.
#
# Notas de implementação:
#   - FileSystemRights é um enum de 32 bits com sinal. Valores genéricos
#     (GENERIC_WRITE 0x40000000, GENERIC_ALL 0x10000000) ficam positivos em
#     [long]; GENERIC_READ 0x80000000 fica negativo em [int], daí [long].
#   - InheritOnly ACEs num arquivo seriam incomuns; verificamos mesmo assim.
#   - Para diretórios, InheritOnly ACEs não se aplicam ao diretório em si,
#     mas se aplicam a arquivos criados dentro dele — reportamos separadamente.
# -----------------------------------------------------------------------
$writeMask = [long]0x00000002 -bor  # WriteData / AddFile
             [long]0x00000004 -bor  # AppendData / AddSubdirectory
             [long]0x00000100 -bor  # WriteAttributes
             [long]0x00000040 -bor  # WriteDAC  (não é escrita de dados, mas conta para nosso escopo)
             [long]0x00000010 -bor  # WriteExtendedAttributes
             [long]0x00000116 -bor  # Write (enum value)
             [long]0x000301BF -bor  # Modify (enum value)
             [long]0x001F01FF -bor  # FullControl (enum value)
             [long]0x40000000 -bor  # GENERIC_WRITE
             [long]0x10000000       # GENERIC_ALL

$readMask  = [long]0x00000001 -bor  # ReadData / ListDirectory
             [long]0x00000020 -bor  # ReadAttributes
             [long]0x00000008 -bor  # ReadExtendedAttributes
             [long]0x00020000 -bor  # ReadPermissions (ReadControl)
             [long]0x000301BF -bor  # Modify
             [long]0x001F01FF -bor  # FullControl
             [long]0x10000000       # GENERIC_ALL
             # Note: GENERIC_READ 0x80000000 would be -2147483648 in [long] which is fine

function Get-ForeignAccess {
    param(
        [string]$Path,
        [string]$CurrentUser,
        [switch]$InheritableOnly   # para diretórios: relatar só ACEs que se propagam para filhos
    )

    $acl = Get-Acl -LiteralPath $Path -ErrorAction SilentlyContinue
    if ($null -eq $acl) {
        return [PSCustomObject]@{ write='(ERR-NO-ACL)'; read='(ERR-NO-ACL)'; owner='(unknown)' }
    }

    $owner = $acl.Owner

    # Identidades sempre excluídas da lista "foreign"
    $excluded = @(
        'NT AUTHORITY\SYSTEM',
        'BUILTIN\Administrators',
        'CREATOR OWNER',
        $owner,
        $CurrentUser
    )

    $foreignWrite = [System.Collections.Generic.List[string]]::new()
    $foreignRead  = [System.Collections.Generic.List[string]]::new()

    foreach ($ace in $acl.Access) {
        if ($ace.AccessControlType -ne [System.Security.AccessControl.AccessControlType]::Allow) { continue }

        # InheritableOnly: pular ACEs que não se propagam (InheritOnly aplica-se só a filhos)
        $isInheritOnly = ($ace.PropagationFlags -band
            [System.Security.AccessControl.PropagationFlags]::InheritOnly) -ne 0
        if ($InheritableOnly -and -not $isInheritOnly) { continue }
        if (-not $InheritableOnly -and $isInheritOnly) { continue }

        $id = $ace.IdentityReference.ToString()
        $isExcluded = $false
        foreach ($ex in $excluded) {
            if ($ex -and ($id -ieq $ex -or $id -ilike "*$($ex -replace '\\','\\\\')")) {
                $isExcluded = $true; break
            }
        }
        if ($isExcluded) { continue }

        $r = [long]([int]$ace.FileSystemRights)  # cast to int first to preserve bit pattern

        if (($r -band $writeMask) -ne 0 -and -not $foreignWrite.Contains($id)) {
            $foreignWrite.Add($id)
        }
        if (($r -band $readMask) -ne 0 -and -not $foreignRead.Contains($id)) {
            $foreignRead.Add($id)
        }
    }

    $fw = if ($foreignWrite.Count -gt 0) { $foreignWrite -join ',' } else { '(none)' }
    $fr = if ($foreignRead.Count  -gt 0) { $foreignRead  -join ',' } else { '(none)' }
    return [PSCustomObject]@{ write=$fw; read=$fr; owner=$owner }
}

function Print-IcaclsAndProbe {
    param(
        [string]$Arm,
        [string]$Artifact,
        [string]$Path,
        [string]$CurrentUser,
        [switch]$IsDir
    )
    Write-Host ""
    Write-Host "=== P17 [$Arm] icacls $Path ==="
    icacls $Path 2>&1 | Write-Host
    $fa = Get-ForeignAccess -Path $Path -CurrentUser $CurrentUser
    Write-Host "PROBE acl $Arm $Artifact owner=$($fa.owner) foreign_write=$($fa.write) foreign_read=$($fa.read)"
    if ($IsDir) {
        $fi = Get-ForeignAccess -Path $Path -CurrentUser $CurrentUser -InheritableOnly
        Write-Host "PROBE acl ${Arm} ${Artifact}-inherit foreign_write_inheritable=$($fi.write) foreign_read_inheritable=$($fi.read)"
    }
}

$currentUser = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
Write-Host "P17_CURRENT_USER=$currentUser"

# -----------------------------------------------------------------------
# Medir ACL das raízes ANTES de criar arquivos — é a fonte da herança.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P17: ACL de raízes (fonte de herança) ==="

$upRoot   = [System.IO.Path]::GetPathRoot($origUserProfile)
$tempRoot = [System.IO.Path]::GetPathRoot($env:RUNNER_TEMP)
$cRoot    = 'C:\'

foreach ($rootPath in (@($upRoot, $cRoot, $tempRoot) | Sort-Object -Unique)) {
    if (Test-Path $rootPath) {
        Print-IcaclsAndProbe -Arm 'root' -Artifact $rootPath -Path $rootPath -CurrentUser $currentUser -IsDir
    }
}

# -----------------------------------------------------------------------
# Braço (i) — home-userprofile
#   Forçar HOME vazio para que homedir.Dir() use os.UserHomeDir() → USERPROFILE.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P17 [home-userprofile]: HOME forçado vazio para usar USERPROFILE ==="

$savedHome = $env:HOME
Remove-Item Env:\HOME -ErrorAction SilentlyContinue

$p17HomeTmp = Join-Path $env:RUNNER_TEMP "p17-proj-i"
if (Test-Path $p17HomeTmp) { Remove-Item -Recurse -Force $p17HomeTmp }
New-Item -ItemType Directory -Force -Path $p17HomeTmp | Out-Null

# Remover identity.json pré-existente em USERPROFILE para garantir gravação nova
$upTrackfw    = Join-Path $env:USERPROFILE ".trackfw"
$upIdentity   = Join-Path $upTrackfw "identity.json"
$upIdBackup   = Join-Path $env:RUNNER_TEMP "p17-backup-identity.json"
if (Test-Path $upIdentity) {
    Copy-Item -Force $upIdentity $upIdBackup
    Remove-Item -Force $upIdentity
    Write-Host "P17 [home-userprofile] identity.json preexistente removido (backup: $upIdBackup)"
}

Push-Location $p17HomeTmp
Write-Host "P17 [home-userprofile] executando: $Bin init --identity-preset greek"
& $Bin init --identity-preset greek 2>&1 | Write-Host
$initExitI = $LASTEXITCODE
Pop-Location

Write-Host "P17 [home-userprofile] init_exit=$initExitI"

# Restaurar HOME
if ($savedHome) { $env:HOME = $savedHome } else { Remove-Item Env:\HOME -ErrorAction SilentlyContinue }

if (Test-Path $upIdentity) {
    Print-IcaclsAndProbe -Arm 'home-userprofile' -Artifact 'identity.json' -Path $upIdentity -CurrentUser $currentUser
    Print-IcaclsAndProbe -Arm 'home-userprofile' -Artifact '.trackfw-dir' -Path $upTrackfw  -CurrentUser $currentUser -IsDir
    Print-IcaclsAndProbe -Arm 'home-userprofile' -Artifact 'USERPROFILE-dir' -Path $env:USERPROFILE -CurrentUser $currentUser -IsDir
} else {
    Write-Host "PROBE acl home-userprofile identity.json foreign_write=(NOT-CREATED init_exit=$initExitI)"
}

# Restaurar identity.json se havia backup
if (Test-Path $upIdBackup) {
    Copy-Item -Force $upIdBackup $upIdentity
    Remove-Item -Force $upIdBackup
    Write-Host "P17 [home-userprofile] identity.json restaurado do backup"
}
Remove-Item -Recurse -Force $p17HomeTmp -ErrorAction SilentlyContinue

# -----------------------------------------------------------------------
# Braço (ii) — root-c
#   HOME apontado para C:\p17-proj\h para que identity.json vá para
#   C:\p17-proj\h\.trackfw\identity.json (cadeia C:\).
#   trackfw agents install --scope project → C:\p17-proj\.trackfw\integrations-manifest.json
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P17 [root-c]: HOME=C:\p17-proj\h (força identidade sob C:\) ==="

$cProjDir     = 'C:\p17-proj'
$cProjHome    = 'C:\p17-proj\h'
$cManifest    = Join-Path $cProjDir '.trackfw\integrations-manifest.json'
$cManifestDir = Join-Path $cProjDir '.trackfw'
$cIdentity    = Join-Path $cProjHome '.trackfw\identity.json'
$cIdentityDir = Join-Path $cProjHome '.trackfw'

# Limpar e criar C:\p17-proj
if (Test-Path $cProjDir) { Remove-Item -Recurse -Force $cProjDir -ErrorAction SilentlyContinue }
New-Item -ItemType Directory -Force -Path $cProjDir  | Out-Null
New-Item -ItemType Directory -Force -Path $cProjHome | Out-Null

# git init (opcional — trackfw init não exige mas pode checar)
Push-Location $cProjDir
Write-Host "P17 [root-c] git init $cProjDir"
git init 2>&1 | Write-Host

# Definir HOME para C:\p17-proj\h
$env:HOME = $cProjHome
Write-Host "P17 [root-c] HOME=$env:HOME USERPROFILE=$env:USERPROFILE"

# trackfw init — cria trackfw.yaml e identity.json em C:\p17-proj\h\.trackfw\
Write-Host "P17 [root-c] executando: $Bin init --identity-preset greek"
& $Bin init --identity-preset greek 2>&1 | Write-Host
$initExitII = $LASTEXITCODE
Write-Host "P17 [root-c] init_exit=$initExitII"

# trackfw agents install --scope project --items architect
# Catálogo embutido (go:embed assets) — sem rede.
Write-Host "P17 [root-c] executando: $Bin agents install --scope project --items architect"
& $Bin agents install --scope project --items architect 2>&1 | Write-Host
$installExit = $LASTEXITCODE
Write-Host "P17 [root-c] agents_install_exit=$installExit"

Pop-Location

# Restaurar HOME
if ($savedHome) { $env:HOME = $savedHome } else { Remove-Item Env:\HOME -ErrorAction SilentlyContinue }

# Medir identity.json sob C:\p17-proj\h\.trackfw\
if (Test-Path $cIdentity) {
    Print-IcaclsAndProbe -Arm 'root-c' -Artifact 'identity.json' -Path $cIdentity -CurrentUser $currentUser
    Print-IcaclsAndProbe -Arm 'root-c' -Artifact '.trackfw-h-dir' -Path $cIdentityDir -CurrentUser $currentUser -IsDir
} else {
    Write-Host "PROBE acl root-c identity.json foreign_write=(NOT-CREATED init_exit=$initExitII)"
}

# Medir integrations-manifest.json sob C:\p17-proj\.trackfw\
if (Test-Path $cManifest) {
    Print-IcaclsAndProbe -Arm 'root-c' -Artifact 'integrations-manifest.json' -Path $cManifest -CurrentUser $currentUser
    Print-IcaclsAndProbe -Arm 'root-c' -Artifact '.trackfw-proj-dir' -Path $cManifestDir -CurrentUser $currentUser -IsDir
    Print-IcaclsAndProbe -Arm 'root-c' -Artifact 'c-proj-dir' -Path $cProjDir -CurrentUser $currentUser -IsDir
} else {
    Write-Host "PROBE acl root-c integrations-manifest.json foreign_write=(NOT-CREATED install_exit=$installExit)"
}

# Limpeza do braço (ii)
Remove-Item -Recurse -Force $cProjDir -ErrorAction SilentlyContinue

# -----------------------------------------------------------------------
# Braço RUNNER_TEMP — repetição do padrão de identity.json sob RUNNER_TEMP.
# -----------------------------------------------------------------------
Write-Host ""
Write-Host "=== P17 [runner-temp]: HOME=$env:RUNNER_TEMP\p17-h-rt ==="

$rtHome    = Join-Path $env:RUNNER_TEMP "p17-h-rt"
$rtProj    = Join-Path $env:RUNNER_TEMP "p17-proj-rt"
$rtIdentity= Join-Path $rtHome ".trackfw\identity.json"
$rtIdDir   = Join-Path $rtHome ".trackfw"

if (Test-Path $rtHome) { Remove-Item -Recurse -Force $rtHome }
if (Test-Path $rtProj) { Remove-Item -Recurse -Force $rtProj }
New-Item -ItemType Directory -Force -Path $rtHome | Out-Null
New-Item -ItemType Directory -Force -Path $rtProj | Out-Null

Push-Location $rtProj
$env:HOME = $rtHome
Write-Host "P17 [runner-temp] HOME=$env:HOME"
& $Bin init --identity-preset greek 2>&1 | Write-Host
$initExitRT = $LASTEXITCODE
Write-Host "P17 [runner-temp] init_exit=$initExitRT"
Pop-Location

if ($savedHome) { $env:HOME = $savedHome } else { Remove-Item Env:\HOME -ErrorAction SilentlyContinue }

if (Test-Path $rtIdentity) {
    Print-IcaclsAndProbe -Arm 'runner-temp' -Artifact 'identity.json' -Path $rtIdentity -CurrentUser $currentUser
    Print-IcaclsAndProbe -Arm 'runner-temp' -Artifact '.trackfw-dir' -Path $rtIdDir -CurrentUser $currentUser -IsDir
} else {
    Write-Host "PROBE acl runner-temp identity.json foreign_write=(NOT-CREATED init_exit=$initExitRT)"
}

Remove-Item -Recurse -Force $rtHome -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $rtProj -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Pergunta 17 CONCLUIDA — grep 'PROBE acl' para o resultado central ==="
Write-Host "P17_LIMITES: runner elevado (admin); apenas x64; nao mede perfil nao-admin; nao mede ARM64"

# Sonda sem veredito — sai 0.
exit 0
