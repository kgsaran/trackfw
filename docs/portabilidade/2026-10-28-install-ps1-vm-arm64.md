# Medição: install.ps1 e canais Windows — VM ARM64

**Data:** 2026-10-29
**VM:** Windows 11 ARM64, build `10.0.26300.9711` (`Microsoft Windows [versão 10.0.26300.9711]`)
**PowerShell:** 5.1.26100.9711 (Desktop edition)
**Commit da branch:** `daa348bb6d5dfec00ed9043ec503ae05d8b54620`
**Branch:** `feat/documentacao-reflete-produto-11-windows-install-ps1`
**Agente:** Ares (ML-4A)

---

## Ação 1 — Inventário

**Comando:** `ml4a-inventory.ps1` via `powershell.exe -NoProfile -ExecutionPolicy Bypass -File`

**Saída literal:**
```
=== PSVersion ===
Major         : 5
Minor         : 1
Build         : 26100
Revision      : 9711
MajorRevision : 0
MinorRevision : 9711

=== PSEdition ===
Desktop

=== ARCH ===
PROCESSOR_ARCHITECTURE=ARM64
PROCESSOR_ARCHITEW6432=
RuntimeInformation.OSArchitecture=Arm64

=== TRACKFW ===
Get-Command trackfw: NOT FOUND
where.exe trackfw: INFORMAÇÕES: não foi possível localizar arquivos para o(s) padrão(ões) especificado(s).

=== HKCU PATH ===
PathKind=String
PathValue=C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\;C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\;C:\Users\Lab\AppData\Local\Programs\Python\Launcher\;%USERPROFILE%\AppData\Local\Microsoft\WindowsApps;%USERPROFILE%\go\bin;C:\Users\Lab\AppData\Roaming\npm

=== TAR ===
tar.exe found: C:\WINDOWS\system32\tar.exe
System32\tar.exe exists: True
```

**RC:** não aplicável (inventário apenas)

**Achados:**
- PS 5.1 Desktop edition; `RuntimeInformation.OSArchitecture` disponível mesmo no PS 5.1 e retornou `Arm64`.
- `PROCESSOR_ARCHITECTURE=ARM64` — processo PS é nativo ARM64 (não WoW64); `PROCESSOR_ARCHITEW6432` vazia (confirma ARM64 nativo, não emulado x64).
- trackfw não instalado antes do teste.
- `HKCU:\Environment Path` tipo **`String` (REG_SZ)**, mas contém referências `%USERPROFILE%` (pré-existente na VM; REG_SZ não expande variáveis — estado anterior ao teste, não alterado pelo instalador).
- `tar.exe` presente em `C:\WINDOWS\system32\tar.exe`.

---

## Ação 2 — install.ps1 sob PowerShell 5.1 com TRACKFW_VERSION=v11.1.0

**Comando:** `$env:TRACKFW_VERSION = 'v11.1.0'; & C:\Users\Lab\ml4a\install.ps1`
via `powershell.exe -NoProfile -ExecutionPolicy Bypass -File ml4a-run-install.ps1`

**Saída literal:**
```
=== STARTING INSTALL ===
TRACKFW_VERSION=v11.1.0
Instalando trackfw v11.1.0 (windows/arm64)...
URL: https://github.com/kgsaran/trackfw/releases/download/v11.1.0/trackfw_11.1.0_windows_arm64.tar.gz
Checksum OK: 4171D7F79EC818BFDFD52E3C767BFC6106197997641DE98C2750CA17EDE46910

trackfw 11.1.0
Adicionado ao PATH do usuario: C:\Users\Lab\AppData\Local\Programs\trackfw
Abra um novo terminal para que o trackfw esteja disponivel no PATH.
NAO MEDIDO: janelas do Explorer abertas antes desta instalacao podem nao ver o PATH atualizado.

trackfw v11.1.0 instalado com sucesso em C:\Users\Lab\AppData\Local\Programs\trackfw\trackfw.exe
=== LASTEXITCODE=0 ===
=== DOLLAR_QUESTION=True ===
```

**RC:** `$LASTEXITCODE=0`, `$?=True`
**Arquitetura detectada:** `arm64` (via `PROCESSOR_ARCHITECTURE=ARM64` — ramo `elseif` do PS 5.1)
**Arquivo baixado:** `trackfw_11.1.0_windows_arm64.tar.gz`
**Checksum:** `4171D7F79EC818BFDFD52E3C767BFC6106197997641DE98C2750CA17EDE46910`

---

## Ação 3 — Verificações pós-instalação

**Comando:** `powershell.exe -NoProfile -ExecutionPolicy Bypass -File ml4a-post-install.ps1`

**Saída literal:**
```
=== VERSION (absolute path) ===
trackfw 11.1.0
LASTEXITCODE=0

=== PE MACHINE (header bytes) ===
PE offset: 0x80
PE signature: PE   (hex: 50-45-00-00)
Machine word: 0xAA64
Machine: ARM64 (IMAGE_FILE_MACHINE_ARM64)

=== PATH HKCU AFTER INSTALL ===
PathKind=String
PathValue=C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\;...;C:\Users\Lab\AppData\Roaming\npm;C:\Users\Lab\AppData\Local\Programs\trackfw

=== OTHER TRACKFW COPIES WARNING ===
Other copies found:
  C:\Users\Lab\AppData\Local\Programs\trackfw\trackfw.exe
C:\Users\Lab\bin\trackfw.exe exists: True
```

**Achados:**
- `trackfw version` pelo caminho absoluto: `trackfw 11.1.0`, RC=0.
- PE machine `0xAA64` = `IMAGE_FILE_MACHINE_ARM64` — binário nativo ARM64. ✓
- PATH preservado como **`String` (REG_SZ)** com nova entrada appended. Tipo não alterado. ✓
- `C:\Users\Lab\bin\trackfw.exe` existe no disco (instalação prévia do Git Bash), mas não estava no PATH da sessão PS durante o install — nenhum aviso de "outra cópia" foi emitido pelo instalador nesta execução. Comportamento correto: o aviso só cobre o PATH da sessão atual.

---

## Ação 4 — Reexecução idempotente

**Comando:** `powershell.exe -NoProfile -ExecutionPolicy Bypass -File ml4a-idempotency.ps1`

**Saída literal (trecho relevante):**
```
Count of installDir before: 1
=== RERUNNING INSTALL ===
Instalando trackfw v11.1.0 (windows/arm64)...
URL: https://github.com/kgsaran/trackfw/releases/download/v11.1.0/trackfw_11.1.0_windows_arm64.tar.gz
Checksum OK: 4171D7F79EC818BFDFD52E3C767BFC6106197997641DE98C2750CA17EDE46910

trackfw 11.1.0
C:\Users\Lab\AppData\Local\Programs\trackfw ja esta no PATH do usuario.

trackfw v11.1.0 instalado com sucesso em C:\Users\Lab\AppData\Local\Programs\trackfw\trackfw.exe
LASTEXITCODE=0
Count of installDir after: 1
IDEMPOTENCY: OK - installDir appears exactly once
```

**RC:** LASTEXITCODE=0
**Resultado:** PATH não duplicou. Ocorrências de `C:\Users\Lab\AppData\Local\Programs\trackfw` antes=1, depois=1. ✓

---

## Ação 5 — TRACKFW_NO_MODIFY_PATH=1

**Comando:** `powershell.exe -NoProfile -ExecutionPolicy Bypass -File ml4a-no-modify-path.ps1`

Antes da execução: entrada do installDir removida manualmente do PATH do registro (para testar que o script não a readiciona).

**Saída literal (trecho relevante):**
```
PathKind=String
PathBefore=...;C:\Users\Lab\AppData\Local\Programs\trackfw
PathAfterRemoval=C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\;...;C:\Users\Lab\AppData\Roaming\npm

=== STEP 5B: Run install.ps1 with TRACKFW_NO_MODIFY_PATH=1 ===
Instalando trackfw v11.1.0 (windows/arm64)...
URL: https://github.com/kgsaran/trackfw/releases/download/v11.1.0/trackfw_11.1.0_windows_arm64.tar.gz
Checksum OK: 4171D7F79EC818BFDFD52E3C767BFC6106197997641DE98C2750CA17EDE46910

trackfw 11.1.0

TRACKFW_NO_MODIFY_PATH=1: PATH nao foi alterado.
Para usar o trackfw, adicione manualmente ao PATH do usuario: C:\Users\Lab\AppData\Local\Programs\trackfw

trackfw v11.1.0 instalado com sucesso em C:\Users\Lab\AppData\Local\Programs\trackfw\trackfw.exe
LASTEXITCODE=0

PathKindAfter=String
PathAfter=C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\;...;C:\Users\Lab\AppData\Roaming\npm
NO_MODIFY_PATH: OK - PATH does not contain installDir (as expected)
PATH_UNCHANGED: OK - path matches pre-install state
```

**RC:** LASTEXITCODE=0
**Resultado:** PATH não foi alterado quando `TRACKFW_NO_MODIFY_PATH=1`. Tipo preservado. ✓

---

## Ação 6 — Reversão documentada

O README (seção Windows, `install.ps1`) documenta: *"To reverse the PATH addition: open System Properties → Environment Variables, find the user PATH, and remove the trackfw entry."*

**Simulação via registro** (equivalente ao que a GUI faz):

**Comando:** `powershell.exe -NoProfile -ExecutionPolicy Bypass -File ml4a-reversal.ps1`

**Saída literal:**
```
=== STEP 6A: Primeiro re-adicionar installDir ao PATH ===
PathKind=String
CurrentPath=...;C:\Users\Lab\AppData\Roaming\npm
PathWithInstall=...;C:\Users\Lab\AppData\Roaming\npm;C:\Users\Lab\AppData\Local\Programs\trackfw

=== STEP 6B: Aplicar reversao documentada (remover entrada do PATH) ===
PathKindBeforeReversal=String
PathBeforeReversal=...;C:\Users\Lab\AppData\Local\Programs\trackfw
PathAfterReversal=...;C:\Users\Lab\AppData\Roaming\npm

=== STEP 6C: Confirmar que voltou ao valor do passo 1 ===
FinalPathKind=String
FinalPath=C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\;C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\;C:\Users\Lab\AppData\Local\Programs\Python\Launcher\;%USERPROFILE%\AppData\Local\Microsoft\WindowsApps;%USERPROFILE%\go\bin;C:\Users\Lab\AppData\Roaming\npm
REVERSAL_PATH: OK - matches step 1 value
REVERSAL_KIND: OK - REG_SZ (String) preserved
```

**Resultado:** PATH voltou ao valor exato do passo 1. Tipo REG_SZ preservado. ✓
**VM ao final desta ação:** PATH do usuário igual ao estado pré-teste. ✓

---

## Ação 7 — install.sh sob Git Bash

**Comando:**
```
"C:\Program Files\Git\bin\bash.exe" -c "export TRACKFW_VERSION=v11.1.0;
  export TRACKFW_INSTALL_DIR=/c/Users/Lab/ml4a/sh-bin;
  mkdir -p /c/Users/Lab/ml4a/sh-bin;
  bash /c/Users/Lab/ml4a/install.sh; echo RC=$?"
```

**Saída literal:**
```
Instalando trackfw v11.1.0 (windows/arm64)...
URL: https://github.com/kgsaran/trackfw/releases/download/v11.1.0/trackfw_11.1.0_windows_arm64.tar.gz
Checksum OK: 4171d7f79ec818bfdfd52e3c767bfc6106197997641de98c2750ca17ede46910

Atencao: /c/Users/Lab/ml4a/sh-bin nao esta no seu PATH.
Adicione ao seu shell profile:
  export PATH="$PATH:/c/Users/Lab/ml4a/sh-bin"
Atencao: outra copia do trackfw esta no PATH e pode responder no lugar desta:
  /c/Users/Lab/bin/trackfw
  /c/Users/Lab/bin/trackfw.exe
Confira com: type -a trackfw   (no Windows tambem: where.exe trackfw)
Mantenha uma unica copia para que hooks de agente executem a versao instalada.

trackfw v11.1.0 instalado com sucesso em /c/Users/Lab/ml4a/sh-bin/trackfw.exe
trackfw 11.1.0
RC=0
```

**Verificação PE machine** (pós execução via PS):
```
trackfw 11.1.0
LASTEXITCODE=0
Machine word: 0xAA64
Machine: ARM64 (IMAGE_FILE_MACHINE_ARM64)
```

**RC:** 0
**Arquitetura detectada pelo install.sh:** `arm64`
**PE machine:** `0xAA64` = ARM64 nativo. ✓

**Observação:** install.sh emitiu aviso sobre `C:\Users\Lab\bin\trackfw.exe` que já existia no PATH do Git Bash (`~/bin` está no PATH de bash por convenção MSYS2/Git Bash).

---

## Ação 8 — npm e pip a partir do registro

**Ferramentas encontradas na VM:**
- `npm 11.17.0` em `C:\Program Files\nodejs\npm.ps1`, Node.js v24.19.0
- `python 3.12.10` (ARM64) em `C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\python.exe`

### npm

**Comando:** `npm install --prefix C:\Users\Lab\ml4a\npm trackfw@11.1.0`
seguido de: `C:\Users\Lab\ml4a\npm\node_modules\.bin\trackfw.cmd version`

**Saída literal:**
```
added 2 packages in 11s
npm notice
npm notice New major version of npm available! 11.17.0 -> 12.2.1
npm notice Changelog: https://github.com/npm/cli/releases/tag/v12.2.1
npm notice To update run: npm install -g npm@12.2.1
npm notice
npm install LASTEXITCODE=0
Found: C:\Users\Lab\ml4a\npm\node_modules\.bin\trackfw.cmd
trackfw 11.1.0
trackfw.cmd LASTEXITCODE=0
```

**RC:** LASTEXITCODE=0
**Resultado:** `trackfw 11.1.0` via `trackfw.cmd`. ✓

### pip

**Comandos:**
```
python -m venv C:\Users\Lab\ml4a\venv
C:\Users\Lab\ml4a\venv\Scripts\pip.exe install trackfw==11.1.0
C:\Users\Lab\ml4a\venv\Scripts\trackfw.exe version
```

**Saída literal:**
```
venv create LASTEXITCODE=0
Collecting trackfw==11.1.0
  Downloading trackfw-11.1.0-py3-none-win_arm64.whl.metadata (191 bytes)
Downloading trackfw-11.1.0-py3-none-win_arm64.whl (5.0 MB)
   ---------------------------------------- 5.0/5.0 MB 6.2 MB/s eta 0:00:00
Installing collected packages: trackfw
Successfully installed trackfw-11.1.0
[notice] A new release of pip is available: 25.0.1 -> 26.2.2
pip install LASTEXITCODE=0
Found: C:\Users\Lab\ml4a\venv\Scripts\trackfw.exe
trackfw 11.1.0
trackfw.exe LASTEXITCODE=0
```

**RC:** LASTEXITCODE=0
**Wheel baixado:** `trackfw-11.1.0-py3-none-win_arm64.whl` (wheel win_arm64 nativo)
**Resultado:** `trackfw 11.1.0` via `trackfw.exe`. ✓

---

## Ação 9 — Limpeza

**Comandos:**
```powershell
Remove-Item -Recurse -Force "C:\Users\Lab\ml4a"
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\Programs\trackfw"
```

**Saída literal:**
```
=== VERIFICAR PATH ANTES DA LIMPEZA ===
PathKind=String
PathValue=C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\;...;C:\Users\Lab\AppData\Roaming\npm
PATH_CHECK: OK - matches step 1

=== APAGAR C:\Users\Lab\ml4a\ ===
ml4a: REMOVED OK

=== APAGAR %LOCALAPPDATA%\Programs\trackfw ===
trackfw dir: REMOVED OK

=== VERIFICAR PATH FINAL ===
FinalPathKind=String
FinalPath=C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\;C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\;C:\Users\Lab\AppData\Local\Programs\Python\Launcher\;%USERPROFILE%\AppData\Local\Microsoft\WindowsApps;%USERPROFILE%\go\bin;C:\Users\Lab\AppData\Roaming\npm
FINAL_PATH_CHECK: OK - matches step 1
```

**VM ao final:** PATH do usuário idêntico ao passo 1 (tipo REG_SZ, valor exato). ✓
`C:\Users\Lab\ml4a\` removido. ✓
`%LOCALAPPDATA%\Programs\trackfw\` removido. ✓

---

## Achados transversais

| Achado | Detalhe |
|---|---|
| `PROCESSOR_ARCHITEW6432` vazia em ARM64 nativo | Em PS 5.1 de 64 bits nativo ARM64, a variável WoW64 não é preenchida. O ramo `elseif ($env:PROCESSOR_ARCHITECTURE -ceq 'ARM64')` é o que dispara. |
| `RuntimeInformation.OSArchitecture` disponível no PS 5.1 | O comentário no script diz "NAO MEDIDO em PS 7 emulado em ARM64"; aqui foi medido em PS 5.1 nativo ARM64: retorna `Arm64`. |
| PATH é REG_SZ mas contém `%USERPROFILE%` | Estado pré-existente da VM. REG_SZ não expande variáveis de ambiente; as entradas `%USERPROFILE%\go\bin` e `%USERPROFILE%\AppData\Local\Microsoft\WindowsApps` ficam literais. O instalador preservou o tipo REG_SZ corretamente. |
| Aviso de outra cópia só cobre PATH da sessão | `C:\Users\Lab\bin\trackfw.exe` existe, mas `C:\Users\Lab\bin` não está no PATH da sessão PS — nenhum aviso emitido. Aviso foi emitido pelo install.sh (Git Bash tem `~/bin` no PATH por padrão). |
| npm.ps1 bloqueado sob Restricted | `npm` é resolvido como `.ps1` em PS. Sob `ExecutionPolicy Restricted` isso bloquearia. A VM está com `Bypass` — não foi testado o cenário Restricted neste ML (fora do escopo). |

---

## Tabela-Resumo

| Canal | Cenário | Resultado |
|---|---|---|
| `install.ps1` PS 5.1 ARM64 | Download, checksum, extração, PATH, version | **medido — OK** |
| `install.ps1` idempotência | Segunda execução não duplica PATH | **medido — OK** |
| `install.ps1` TRACKFW_NO_MODIFY_PATH=1 | PATH não alterado | **medido — OK** |
| `install.ps1` reversão (README) | Remover entrada do PATH via registro | **medido — OK** |
| `install.sh` Git Bash ARM64 | Download, checksum, extração, version | **medido — OK** |
| npm ARM64 a partir do registro | `npm install` local, `trackfw.cmd version` | **medido — OK** |
| pip ARM64 a partir do registro | `pip install` em venv, `trackfw.exe version` | **medido — OK** |

> Esta tabela é a fonte para a matriz do README (seção Windows).
