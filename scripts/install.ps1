# trackfw installer for Windows
# PowerShell 5.1 (Windows PowerShell) and PowerShell 7+.
# No external modules required.
#
# Uso padrao:
#   Invoke-WebRequest -Uri https://github.com/kgsaran/trackfw/releases/latest/download/install.ps1 -OutFile install.ps1
#   # Leia o script antes de executar.
#   powershell -ExecutionPolicy Bypass -File install.ps1
#
# Opcoes (via variaveis de ambiente antes de invocar):
#   TRACKFW_VERSION     - pinar versao (ex.: v11.1.0). Se ausente, resolve a mais recente.
#   TRACKFW_ARCH        - sobrepor arquitetura: amd64 ou arm64.
#   TRACKFW_INSTALL_DIR - diretorio de instalacao absoluto. Padrao: LOCALAPPDATA\Programs\trackfw
#   TRACKFW_NO_MODIFY_PATH=1 - nao modifica o PATH do usuario (apenas exibe instrucao).
#
# Este script nao aceita parametro de origem alternativa de download.
# O binario e o checksums.txt vem sempre do mesmo release do GitHub.

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference    = 'SilentlyContinue'
    # Em PS 7.4+, $PSNativeCommandUseErrorActionPreference padrao e $true, o que converte saida != 0
    # de comandos nativos em erro terminante. Desabilitar aqui para verificar $LASTEXITCODE manualmente.
    $PSNativeCommandUseErrorActionPreference = $false

    # --- TLS 1.2 para Windows PowerShell 5.1 (Desktop edition) ---
    if ($PSVersionTable.PSEdition -eq 'Desktop') {
        [Net.ServicePointManager]::SecurityProtocol = `
            [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    }

    $REPO = 'kgsaran/trackfw'
    $OS   = 'windows'
    $BIN  = 'trackfw.exe'

    # -------------------------------------------------------------------------
    # Validar TRACKFW_VERSION se definida
    # -------------------------------------------------------------------------
    # Usa -cmatch com \A e \z (nao ^ e $): \A ancora no inicio do buffer inteiro;
    # \z ancora no fim do buffer inteiro. ^ e $ em modo padrao casam por LINHA --
    # um valor "v11.0.0" + newline passaria ^ e $ mas e rejeitado por \z.
    # Usa [0-9] em vez de \d: \d em .NET casa digitos Unicode (ex.: arabe-indico).
    # Usa -cmatch (case-sensitive): "V11.0.0" seria invalido, como no install.sh.
    $VERSION = ''
    if ($env:TRACKFW_VERSION) {
        $raw = $env:TRACKFW_VERSION
        if (-not ($raw -cmatch '\Av?[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]{1,3}|-beta[0-9]{1,3})?\z')) {
            throw "Erro: TRACKFW_VERSION invalida: '$raw'. Formato esperado: v?MAJOR.MINOR.PATCH[-rcN|-betaN] (ex.: 11.1.0, v11.1.0, v11.0.0-rc2)"
        }
        if ($raw.StartsWith('v')) { $VERSION = $raw } else { $VERSION = "v$raw" }
    }

    # -------------------------------------------------------------------------
    # Detectar arquitetura
    # -------------------------------------------------------------------------
    # TRACKFW_ARCH: lista fechada, comparacao case-sensitive.
    # Deteccao automatica:
    #   PS 7 (.NET 5+): RuntimeInformation.OSArchitecture. Medido em PS 5.1 ARM64 nativo
    #   (docs/portabilidade/2026-10-28-install-ps1-vm-arm64.md, Acao 1): retornou Arm64.
    #   NAO MEDIDO: PS 7 emulado em ARM64 (processo x64 emulado no host).
    #   PS 5.1 (Desktop, ARM64 nativo ou x64 emulado em ARM64):
    #     PROCESSOR_ARCHITECTURE pode reportar AMD64 mesmo num host ARM64 (descreve o processo emulado).
    #     PROCESSOR_ARCHITEW6432 pode indicar a arquitetura real do host (variavel WoW64/bridge).
    #     MEDIDO: vazia em PS 5.1 ARM64 nativo (Acao 1 -- ARM64 nativo nao e WoW64, variavel nao preenchida).
    #     NAO MEDIDO: comportamento em PS 5.1 de processo x64 emulado em host ARM64.
    #     Use TRACKFW_ARCH=arm64 para sobrepor se a deteccao automatica retornar amd64 num host ARM64.
    $ARCH = ''
    if ($env:TRACKFW_ARCH) {
        if ($env:TRACKFW_ARCH -cne 'amd64' -and $env:TRACKFW_ARCH -cne 'arm64') {
            throw "Erro: TRACKFW_ARCH invalido: '$($env:TRACKFW_ARCH)'. Valores aceitos: amd64, arm64"
        }
        $ARCH = $env:TRACKFW_ARCH
    }

    if (-not $ARCH) {
        if ($PSVersionTable.PSVersion.Major -ge 7) {
            $osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower()
            if ($osArch -ceq 'arm64')   { $ARCH = 'arm64' }
            elseif ($osArch -ceq 'x64') { $ARCH = 'amd64' }
            else { throw "Arquitetura nao suportada: $osArch. Use TRACKFW_ARCH=amd64 ou TRACKFW_ARCH=arm64." }
        } elseif ($env:PROCESSOR_ARCHITEW6432 -ceq 'ARM64') {
            $ARCH = 'arm64'
        } elseif ($env:PROCESSOR_ARCHITECTURE -ceq 'AMD64') {
            $ARCH = 'amd64'
        } elseif ($env:PROCESSOR_ARCHITECTURE -ceq 'ARM64') {
            $ARCH = 'arm64'
        } else {
            throw "Arquitetura nao reconhecida. PROCESSOR_ARCHITECTURE='$($env:PROCESSOR_ARCHITECTURE)'. Use TRACKFW_ARCH=amd64 ou TRACKFW_ARCH=arm64."
        }
    }

    # -------------------------------------------------------------------------
    # Resolver versao mais recente via API do GitHub (pulado se VERSION ja pinada)
    # -------------------------------------------------------------------------
    if (-not $VERSION) {
        $apiUrl  = "https://api.github.com/repos/$REPO/releases/latest"
        $tmpJson = Join-Path ([System.IO.Path]::GetTempPath()) ("trackfw-api-" + [System.IO.Path]::GetRandomFileName() + ".json")
        try {
            Invoke-WebRequest -Uri $apiUrl -OutFile $tmpJson -UseBasicParsing
            $json    = Get-Content $tmpJson -Raw | ConvertFrom-Json
            $tagName = $json.tag_name
        } finally {
            if (Test-Path $tmpJson) { Remove-Item $tmpJson -Force -ErrorAction SilentlyContinue }
        }
        if (-not $tagName) { throw "Erro: nao foi possivel determinar a versao mais recente via API." }
        # Validar tag_name com a mesma regex de TRACKFW_VERSION
        if (-not ($tagName -cmatch '\Av?[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]{1,3}|-beta[0-9]{1,3})?\z')) {
            throw "Erro: tag_name invalida recebida da API: '$tagName'"
        }
        if ($tagName.StartsWith('v')) { $VERSION = $tagName } else { $VERSION = "v$tagName" }
    }

    # VERSION_BARE: sem 'v' inicial, usado no nome do arquivo (GoReleaser convention)
    $VERSION_BARE = $VERSION.TrimStart('v')
    $FILENAME     = "trackfw_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"
    $BASE_URL     = "https://github.com/$REPO/releases/download/$VERSION"
    $DL_URL       = "$BASE_URL/$FILENAME"
    $CS_URL       = "$BASE_URL/checksums.txt"

    # -------------------------------------------------------------------------
    # Diretorio de instalacao
    # -------------------------------------------------------------------------
    $INSTALL_DIR = if ($env:TRACKFW_INSTALL_DIR) { $env:TRACKFW_INSTALL_DIR } `
                   else { Join-Path $env:LOCALAPPDATA "Programs\trackfw" }

    if (-not $INSTALL_DIR) {
        throw "Erro: nao foi possivel determinar o diretorio de instalacao. Defina TRACKFW_INSTALL_DIR."
    }
    # Rejeitar INSTALL_DIR com ponto-e-virgula (ASCII U+003B) ou variante Unicode:
    #   U+037E GREEK QUESTION MARK  -- NFC => U+003B; best-fit CP1252: 0x3B (unicode.org/MICSFT/bestfit1252.txt)
    #   U+FE54 SMALL SEMICOLON      -- NFKC => U+003B; sem mapeamento CP1252 confirmado
    #   U+FF1B FULLWIDTH SEMICOLON  -- NFKC => U+003B; best-fit CP1252: 0x3B (confirmado)
    # U+037E e U+FF1B convertem para 0x3B via best-fit CP1252 (apps ANSI legados); U+FE54 rejeitado
    # por equivalencia NFKC (=> U+003B) mesmo sem mapeamento CP1252 confirmado.
    # Verificado antes do caminho absoluto e antes de qualquer escrita (diretorio, arquivo, registro).
    if ($INSTALL_DIR -match '[;\u037E\uFE54\uFF1B]') {
        throw "Erro: TRACKFW_INSTALL_DIR nao pode conter ponto-e-virgula ou variante Unicode: '$INSTALL_DIR'"
    }
    # Validar caminho absoluto: deve comecar com letra:\ ou \\ (UNC).
    # [System.IO.Path]::IsPathRooted aceita '\foo' (raiz relativa) e 'C:foo' (relativo ao drive).
    # Exigimos a forma totalmente qualificada.
    if (-not ($INSTALL_DIR -cmatch '\A[A-Za-z]:[/\\]' -or $INSTALL_DIR.StartsWith('\\'))) {
        throw "Erro: TRACKFW_INSTALL_DIR deve ser um caminho absoluto totalmente qualificado: '$INSTALL_DIR'"
    }
    # Aviso (nao aborta) se o destino for um ponto de reanexacao (junction/symlink)
    $dirItem = Get-Item $INSTALL_DIR -ErrorAction SilentlyContinue
    if ($dirItem -and ($dirItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
        Write-Warning "$INSTALL_DIR e um ponto de reanexacao (junction ou symlink)."
    }
    if (-not (Test-Path $INSTALL_DIR)) {
        New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
    }

    Write-Host "Instalando trackfw $VERSION (windows/$ARCH)..."
    Write-Host "URL: $DL_URL"

    # -------------------------------------------------------------------------
    # Diretorio temporario aleatorio
    # -------------------------------------------------------------------------
    # New-Item sem -Force: falha se o path ja existir (protecao contra corrida em %TEMP%).
    # GetRandomFileName gera ~57 bits de entropia (base-32, 11 caracteres).
    $TMP_DIR = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path $TMP_DIR -ErrorAction Stop | Out-Null

    try {
        # --- Download do tarball ---
        $tarball = Join-Path $TMP_DIR $FILENAME
        Invoke-WebRequest -Uri $DL_URL -OutFile $tarball -UseBasicParsing

        # --- Download do checksums.txt da mesma tag ---
        $checksumFile = Join-Path $TMP_DIR "checksums.txt"
        Invoke-WebRequest -Uri $CS_URL -OutFile $checksumFile -UseBasicParsing

        # --- Verificar checksum SHA-256 antes de extrair ---
        # Tres estados de falha com mensagem propria: ausente, duplicado, divergente.
        # Usa campo exato (case-sensitive): evita que '.sig' case com o nome base.
        # Trim de CR para tolerar CRLF no checksums.txt.
        $expectedHash = $null
        $matchCount   = 0
        foreach ($line in (Get-Content $checksumFile)) {
            $line = $line.TrimEnd("`r")
            if ($line.Trim() -eq '') { continue }
            $parts = ($line -split '\s+', 2)
            if ($parts.Count -ge 2 -and $parts[1] -ceq $FILENAME) {
                $matchCount++
                $expectedHash = $parts[0]
            }
        }

        if ($matchCount -eq 0) {
            throw "Erro: checksum ausente -- '$FILENAME' nao encontrado em checksums.txt. Instalacao abortada."
        } elseif ($matchCount -gt 1) {
            throw "Erro: checksum duplicado -- '$FILENAME' aparece $matchCount vezes em checksums.txt. Instalacao abortada."
        }

        $actualHash = (Get-FileHash -Path $tarball -Algorithm SHA256).Hash
        # Comparacao case-insensitive para o hash (SHA256 pode ser maiusculo ou minusculo)
        if ($actualHash -ine $expectedHash) {
            throw "Erro: checksum divergente -- o tarball nao corresponde ao hash publicado.`n  Esperado:  $expectedHash`n  Calculado: $actualHash`nInstalacao abortada."
        }
        Write-Host "Checksum OK: $actualHash"

        # --- Extrair com tar.exe do System32 ---
        # Usar caminho absoluto para evitar que um tar.exe anterior no PATH (ex.: Git/MSYS2)
        # substitua o tar nativo. O tar.exe nativo e bsdtar (libarchive), que descarta membros
        # '..' e caminhos absolutos por padrao. NAO MEDIDO: comportamento exato no runner
        # windows-latest do CI (R2 no modelo de ameaca Wave 0).
        $tarExe = Join-Path $env:SystemRoot "System32\tar.exe"
        if (-not (Test-Path $tarExe)) {
            throw "tar.exe nao encontrado em '$tarExe'. Para instalar sem tar.exe: npm install -g trackfw  ou  pip install trackfw"
        }
        & $tarExe -xzf $tarball -C $TMP_DIR
        if ($LASTEXITCODE -ne 0) { throw "Extracao falhou com codigo $LASTEXITCODE" }

        # Confirmar que trackfw.exe esta dentro de TMP_DIR (nunca extrair direto no destino)
        $extracted = Join-Path $TMP_DIR "trackfw.exe"
        if (-not (Test-Path $extracted)) {
            throw "trackfw.exe nao encontrado em '$TMP_DIR' apos extracao."
        }

        # --- Copiar somente trackfw.exe para o destino (nunca o conteudo inteiro do tar) ---
        $destBin = Join-Path $INSTALL_DIR "trackfw.exe"
        Copy-Item -Path $extracted -Destination $destBin -Force

        # --- Executar o binario instalado para confirmar a versao ---
        # Chamada pelo caminho absoluto para evitar que um binario anterior no PATH responda.
        Write-Host ""
        & $destBin version
        if ($LASTEXITCODE -ne 0) { Write-Warning "trackfw.exe version retornou codigo $LASTEXITCODE" }

        # -------------------------------------------------------------------------
        # Modificar PATH do usuario no registro
        # -------------------------------------------------------------------------
        # IMPORTANTE: nao usar [Environment]::SetEnvironmentVariable, que escreve REG_SZ
        # incondicionalmente, destruindo entradas REG_EXPAND_SZ (ex.: %USERPROFILE%\...).
        # A ADR exige preservar o tipo do valor existente.
        $noModifyPath = $env:TRACKFW_NO_MODIFY_PATH -ceq '1'
        if ($noModifyPath) {
            Write-Host ""
            Write-Host "TRACKFW_NO_MODIFY_PATH=1: PATH nao foi alterado."
            Write-Host "Para usar o trackfw, adicione manualmente ao PATH do usuario: $INSTALL_DIR"
        } else {
            $regKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
            if ($null -eq $regKey) { throw "Nao foi possivel abrir HKCU:\Environment" }
            try {
                # GetValue com DoNotExpandEnvironmentNames: le o valor bruto (sem expandir %VAR%).
                $existingRaw = $regKey.GetValue('PATH', $null,
                    [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
                $valueKind = [Microsoft.Win32.RegistryValueKind]::ExpandString
                $existingPath = ''
                if ($null -ne $existingRaw) {
                    try { $valueKind = $regKey.GetValueKind('PATH') } catch {}
                    $existingPath = [string]$existingRaw
                }

                # Verificar se INSTALL_DIR ja esta no PATH (OrdinalIgnoreCase, sem barra final).
                # OrdinalIgnoreCase evita falso-positivo no locale tr-TR ('I'.ToLower() => dotless i U+0131).
                $installTrim = $INSTALL_DIR.TrimEnd('\', '/')
                $alreadyPresent = $false
                foreach ($entry in ($existingPath -split ';')) {
                    if ($entry -eq '') { continue }
                    $entryTrim  = $entry.TrimEnd('\', '/')
                    $entryExpan = [System.Environment]::ExpandEnvironmentVariables($entry).TrimEnd('\', '/')
                    if ([string]::Equals($entryTrim, $installTrim, [System.StringComparison]::OrdinalIgnoreCase) -or [string]::Equals($entryExpan, $installTrim, [System.StringComparison]::OrdinalIgnoreCase)) {
                        $alreadyPresent = $true
                        break
                    }
                }

                if ($alreadyPresent) {
                    Write-Host "$INSTALL_DIR ja esta no PATH do usuario."
                } else {
                    # TrimEnd(';'): PATH existente com ';' final geraria ';;' na concatenacao (F-2).
                    $cleanPath = $existingPath.TrimEnd(';')
                    $newPath = if ($cleanPath -ne '') { "$cleanPath;$INSTALL_DIR" } else { $INSTALL_DIR }
                    $regKey.SetValue('PATH', $newPath, $valueKind)
                    Write-Host "Adicionado ao PATH do usuario: $INSTALL_DIR"
                    Write-Host "Abra um novo terminal para que o trackfw esteja disponivel no PATH."
                    Write-Host "NAO MEDIDO: janelas do Explorer abertas antes desta instalacao podem nao ver o PATH atualizado."
                }
            } finally {
                $regKey.Close()
            }
        }

        # -------------------------------------------------------------------------
        # Avisar sobre outras copias de trackfw.exe no PATH atual
        # -------------------------------------------------------------------------
        $otherCopies = @()
        foreach ($dir in ($env:PATH -split ';')) {
            if ($dir -eq '') { continue }
            $dirTrimmed     = $dir.TrimEnd('\', '/')
            $installTrimmed = $INSTALL_DIR.TrimEnd('\', '/')
            if ([string]::Equals($dirTrimmed, $installTrimmed, [System.StringComparison]::OrdinalIgnoreCase)) { continue }
            $candidate = [IO.Path]::Combine($dir, 'trackfw.exe')
            if (Test-Path $candidate -PathType Leaf) { $otherCopies += $candidate }
        }
        if ($otherCopies.Count -gt 0) {
            Write-Warning "Outra copia do trackfw.exe esta no PATH e pode responder no lugar desta:"
            foreach ($c in $otherCopies) { Write-Warning "  $c" }
            Write-Warning "Verifique com: where.exe trackfw.exe"
        }

        # -------------------------------------------------------------------------
        # Avisar sobre USERPROFILE\bin\trackfw.exe (~/bin no Git Bash)
        # -------------------------------------------------------------------------
        # O Git Bash coloca ~/bin (USERPROFILE\bin) no inicio do PATH por padrao.
        # Um binario ali pode ocultar a versao instalada nesta sessao PS.
        # Nao executa, nao le e nao remove o arquivo -- apenas verifica existencia.
        if ($env:USERPROFILE) {
            # [IO.Path]::Combine nao lanca em caminhos sem qualificador de drive (ex: UNC sem provider PS).
            # Join-Path (Join-Path ...) lancava "value of argument 'drive' is null" nesses casos (A-2, ML-4E).
            $gitBashBin = [IO.Path]::Combine($env:USERPROFILE, 'bin', 'trackfw.exe')
            # OrdinalIgnoreCase: evita falso-positivo no locale tr-TR onde 'I'.ToLower() => dotless i (A-4).
            $same = [string]::Equals($gitBashBin.TrimEnd('\', '/'), $destBin.TrimEnd('\', '/'), [System.StringComparison]::OrdinalIgnoreCase)
            if (-not $same -and (Test-Path -LiteralPath $gitBashBin -PathType Leaf)) {
                Write-Warning "Atencao: $gitBashBin existe."
                Write-Warning "No Git Bash, ~/bin esta no PATH antes das outras entradas e pode ocultar esta versao."
                Write-Warning "Verifique com: type -a trackfw   (no Git Bash)"
            }
        }

        # --- Sucesso ---
        Write-Host ""
        Write-Host "trackfw $VERSION instalado com sucesso em $destBin"

    } finally {
        # Limpeza do diretorio temporario (executada em qualquer saida: normal ou por erro)
        if (Test-Path $TMP_DIR) {
            Remove-Item -Recurse -Force -Path $TMP_DIR -ErrorAction SilentlyContinue
        }
    }
}
