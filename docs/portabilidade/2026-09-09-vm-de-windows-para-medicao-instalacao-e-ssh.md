# VM de Windows para medição — instalação, SSH por chave e armadilhas medidas

> 2026-09-09 · escrito depois de perder a VM anterior. O que está aqui não é receita genérica de
> internet: cada armadilha abaixo custou tempo numa sessão real deste projeto, e está anotada com o
> sintoma que ela produz.

## Para que esta VM serve — e para que NÃO serve

Serve para **investigação interativa** de comportamento de Windows: rodar um gate até o obstáculo,
instrumentar, sondar, iterar em minutos.

🔴 **Não serve como fonte de verdade de medição.** Ela é **ARM64**; o runner do CI é **x64**. Em
2026-09-09 metade do ML-R2a foi gasta provando que o que a VM media valia para o runner — trabalho
que não existiria se a medição tivesse nascido no CI.

**Regra prática:** investigue na VM, **meça no `windows-latest`**. O instrumento de medição é
`.github/workflows/windows-census.yml` (`workflow_dispatch`, sem veredito).

---

## 1. Onde colocar o disco — a lição que custou a VM anterior

🔴 **Disco da VM no SSD interno. Nunca em volume externo.**

A VM anterior tinha o `.qcow2` em `/Volumes/External/`. Ela caiu no meio de uma sessão de trabalho e
não subiu mais. O diagnóstico final:

```
chkdsk C:      FS íntegro · 0 setores defeituosos
               1732 arquivos · 2,1 GB usados · 38 GB livres   ← num volume de 38 GB
DISM  Erro 50  "não é uma imagem Windows válida"
bcdboot        "falha ao copiar arquivos de inicialização"
```

Volume saudável, **conteúdo ausente**. Não havia instalação para reparar. Um soluço de conexão do
drive externo vira erro de I/O no convidado, e o Windows não sobrevive a isso no volume de boot.

**E tire snapshot** logo depois de instalar e configurar. A noite inteira de recuperação teria sido
dois cliques.

---

## 2. Criar a VM (UTM, Apple Silicon)

Windows 11 **ARM64**. Duas formas de obter a imagem:

- **UTM Gallery** → Windows 11 ARM (baixa e configura); ou
- **CrystalFetch** (App Store, gratuito) → gera ISO oficial ARM64 direto da Microsoft.

Configuração recomendada:

| item | valor | por quê |
|---|---|---|
| CPU | 4 núcleos | a suíte de falsificação é serial e longa |
| RAM | 8 GB | `go build` + `npm ci` + `pytest` convivendo |
| Disco | 64 GB | a de 40 GB apertou; `npm/node_modules` + cache do Go pesam |
| Local | **SSD interno** | ver §1 |
| Rede | Shared Network | dá IP em `192.168.64.x`, alcançável do host |

Depois de instalar, monte **UTM Guest Tools** (menu da UTM) e instale — sem ele o mouse e o
redimensionamento ficam ruins.

---

## 3. OpenSSH Server — tudo em PowerShell (como Administrador)

```powershell
# 1. Instalar o servidor (já vem como recurso opcional no Windows 11)
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0

# 2. Subir e habilitar no boot
Start-Service sshd
Set-Service -Name sshd -StartupType Automatic

# 3. Confirmar que está ouvindo
Get-Service sshd | Format-Table Name, Status, StartType
Get-NetTCPConnection -LocalPort 22 -State Listen | Format-Table LocalAddress, LocalPort

# 4. Liberar no firewall (o instalador normalmente já cria; confirme)
Get-NetFirewallRule -Name *ssh* | Format-Table Name, Enabled, Direction, Action
# se não existir:
New-NetFirewallRule -Name sshd -DisplayName 'OpenSSH Server (sshd)' `
  -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22

# 5. Descobrir o IP para o host
Get-NetIPAddress -AddressFamily IPv4 |
  Where-Object { $_.IPAddress -like '192.168.*' } |
  Format-Table IPAddress, InterfaceAlias
```

---

## 4. 🔴 Chave pública — a armadilha do usuário administrador

**Se o usuário da VM for administrador, o `~/.ssh/authorized_keys` é IGNORADO.** O `sshd` do Windows
lê um arquivo diferente, com dono e ACL específicos. É o erro nº 1 e o sintoma é cruel: o SSH
continua pedindo senha e **nenhuma mensagem explica por quê**.

O arquivo correto é:

```
C:\ProgramData\ssh\administrators_authorized_keys
```

**No host (macOS)** — gere a chave, se ainda não tiver, e mostre a pública:

```bash
ssh-keygen -t ed25519 -C "kg@mac"          # se já existir, pule
cat ~/.ssh/id_ed25519.pub                   # copie esta linha inteira
```

**Na VM (PowerShell como Administrador)** — cole a linha no lugar de `COLE_A_CHAVE_AQUI`:

```powershell
$f   = 'C:\ProgramData\ssh\administrators_authorized_keys'
$key = 'COLE_A_CHAVE_AQUI'

# 1. garantir que o arquivo existe E que você tem acesso a ele
if (-not (Test-Path $f)) { New-Item -ItemType File -Path $f -Force | Out-Null }
takeown /f $f

# 2. ACL POR SID, nunca por nome:
#    *S-1-5-32-544 = Administradores/Administrators   ·   *S-1-5-18 = SYSTEM
icacls $f /inheritance:r
icacls $f /grant '*S-1-5-32-544:F' /grant '*S-1-5-18:F'

# 3. SÓ AGORA escrever — UTF-8 SEM BOM
[IO.File]::WriteAllText($f, "$key`n", (New-Object Text.UTF8Encoding $false))

# 4. conferir: icacls deve mostrar só DUAS entradas; Get-Content, UMA linha
icacls $f
Get-Content $f
Restart-Service sshd
```

🔴 **Cinco detalhes que fazem falhar em silêncio** — os dois primeiros foram medidos em 2026-09-10,
recriando esta VM, e **nenhum dos dois produz erro que pareça ter a ver com SSH**:

1. 🔴 **Nome de grupo é localizado.** Em Windows pt-BR o grupo é **`Administradores`**, não
   `Administrators` — e o `icacls` responde *"não foi feito mapeamento entre os nomes de conta e as
   identificações de segurança"*. **Use SID** (`*S-1-5-32-544`), que é igual em qualquer idioma.
2. 🔴 **A ordem importa: ACL ANTES da escrita.** Escrever primeiro dá
   `UnauthorizedAccessException` — o arquivo já existe com ACL restritiva e nem administrador
   escreve nele. Foi assim que os dois passos falharam em cascata aqui.
3. **BOM.** `Set-Content`/`Out-File` gravam UTF-8 **com BOM** e o `sshd` recusa o arquivo sem dizer
   nada. Por isso o `[IO.File]::WriteAllText`.
4. **Herança de ACL.** Qualquer entrada além de Administradores/SYSTEM faz o `sshd` ignorar o
   arquivo — mesma falha silenciosa.
5. **Quebra de linha.** Uma chave partida em duas linhas não é lida. Cole a linha inteira.

**Diagnóstico quando mesmo assim pedir senha** — na VM, pare o serviço e rode em modo verboso:

```powershell
Stop-Service sshd
& 'C:\Windows\System32\OpenSSH\sshd.exe' -d -d -d
# tente conectar do host noutra aba; a saída diz exatamente qual arquivo foi lido e por que recusou
```

**No host**, registre em `~/.ssh/config`:

```
Host windows-vm
    HostName 192.168.64.X
    User <seu-usuario>
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes
```

Teste: `ssh windows-vm 'hostname'` — tem que entrar **sem pedir senha**.

---

## 5. 🔴 O shell padrão do SSH é `cmd.exe` — e isso quebra comando com aspas

Ao entrar por SSH você cai no **`cmd.exe`** (medido em 2026-09-10; numa instalação anterior era
PowerShell — **não presuma, teste**: mande `echo teste` e veja se volta ecoado literalmente, que
é a assinatura do `cmd`). Em nenhum dos dois há bash. Comandos POSIX (`ls`, `which`, `head`) não
existem, e `2>/dev/null` vira erro `Could not find a part of the path 'C:\dev\null'`.

**Para rodar bash pelo SSH:**

```bash
ssh windows-vm '& "C:\Program Files\Git\bin\bash.exe" -c "bash /c/Users/<user>/script.sh"'
```

🔴 **Mande o script por `scp` em vez de lutar com aspas aninhadas.** Aspas atravessando
ssh → PowerShell → bash é onde o tempo se perde:

```bash
scp -q ./probe.sh windows-vm:'C:/Users/<user>/probe.sh'
ssh windows-vm '& "C:\Program Files\Git\bin\bash.exe" -c "bash /c/Users/<user>/probe.sh"'
```

**Alternativa:** tornar o bash o shell padrão do SSH. Facilita o dia a dia, mas **muda o ambiente
que estamos medindo** — se a VM existe para reproduzir o que um usuário de Windows vive, deixe o
padrão como está.

```powershell
# só se você quiser MESMO mudar o padrão
New-ItemProperty -Path 'HKLM:\SOFTWARE\OpenSSH' -Name DefaultShell `
  -Value 'C:\Program Files\Git\bin\bash.exe' -PropertyType String -Force
```

---

## 6. Toolchain para rodar os gates

```powershell
winget install --id Git.Git             -e --accept-package-agreements
winget install --id GoLang.Go           -e
winget install --id OpenJS.NodeJS.LTS   -e
winget install --id Python.Python.3.12  -e
```

🔴 **`python3` no Windows resolve para o stub da Microsoft Store**, que imprime *"Python was not
found"* mesmo com Python instalado. Os gates deste projeto contornam isso (`resolve_py_bin()` valida
por **execução**), mas os scripts chamam `python3` — então conserte:

```powershell
$d = Split-Path (Get-Command python).Source
Copy-Item "$d\python.exe" "$d\python3.exe"
where python3     # o real tem de vir ANTES do WindowsApps
```

**O Python para Windows instala `python.exe` e NÃO instala `python3.exe`** — por isso o único
`python3` do sistema é o stub. **Cópia, no MESMO diretório**, nunca link noutro: as DLLs moram ao
lado do executável (é o `STATUS_DLL_NOT_FOUND` do ML-R2c).

Valide por **execução**, não por resolução — o stub resolve igual e só falha ao rodar:

```bash
python3 -c "import sys;print(sys.executable)"   # tem de apontar para o Python real
```

**Desligue o `core.autocrlf`** — ele corrompe fixture de teste que depende de bytes exatos:

```powershell
git config --global core.autocrlf false
```

**Compile o binário com extensão** — `bin/trackfw` sem `.exe` não é executável pelo `CreateProcess`:

```powershell
go build -o bin/trackfw.exe ./cmd/trackfw
```

---

## 7. Fatos de layout já medidos (não re-meça)

Medido em ARM64 (VM) **e** em `windows-latest` x64 (run 34406101512):

```
/usr/bin/git   NÃO existe          /bin/git   NÃO existe
git resolve em /mingw64/bin (x64) ou /clangarm64/bin (ARM64)
cygpath -w /bin      → C:\Program Files\Git\usr\bin
cygpath -w /usr/bin  → C:\Program Files\Git\usr\bin      ← o MESMO diretório
```

🔴 **`/bin` é alias de `/usr/bin`** no bash do Git for Windows. Um `PATH` com `"/usr/bin:/bin"` lista
o mesmo diretório duas vezes — a redundância que parece rede de segurança é uma entrada só.

🔴 **`command -v X` do bash NÃO prova que o produto acha `X`.** Um `ln -s` sem `.exe` é achado pelo
bash e **não** pelo processo filho nativo (`exec.Command`, `spawnSync`, `subprocess.run`), que usa
CreateProcess + PATHEXT. Detalhe completo:
[`vault/notes/bash-resolve-o-que-o-processo-filho-nativo-nao-resolve-no-windows-2026-09-09.md`](../../vault/notes/bash-resolve-o-que-o-processo-filho-nativo-nao-resolve-no-windows-2026-09-09.md).

**Isolar `git.exe` num diretório não funciona:** morre com `STATUS_DLL_NOT_FOUND` (0xC0000135) — as
DLLs moram ao lado do executável. Acrescente o **diretório**, não o arquivo.

---

## 7-bis. 🔴 Anote ONDE a VM foi gravada — o UTM não te ajuda depois

Medido em 2026-09-10, recriando esta VM: o UTM registrou a VM em

```
~/Library/Containers/com.utmapp.UTM/Data/Documents/Windows.utm
```

e **o bundle nunca foi escrito lá**. Ao ligar, o erro é só
*"The file couldn't be opened because it doesn't exist"* — sem dizer qual arquivo nem onde.

Agravantes que tornam a recuperação difícil:

- **`Show in Finder` não funciona** para VM fora do container: o UTM da App Store é *sandboxed* e não
  tem permissão para revelar aquele caminho. O item some ou não faz nada.
- **O Spotlight não indexa volume externo por padrão** — `mdfind` não acha o `.qcow2`.
- O caminho registrado fica em
  `~/Library/Containers/com.utmapp.UTM/Data/Library/Preferences/com.utmapp.UTM.plist`, legível com
  `plutil -p ... | grep '"Path"'`. **É a intenção registrada, não a prova de que o arquivo está lá.**

**Ao criar a VM, anote o caminho** e confirme que o bundle existe antes de instalar o SO:

```bash
ls -la "<caminho>/Windows.utm/Data/"     # tem de listar um .qcow2
```

## 8. Checklist final

- [ ] Disco no SSD interno
- [ ] UTM Guest Tools instalado
- [ ] `sshd` rodando, `StartupType Automatic`, porta 22 liberada
- [ ] `administrators_authorized_keys` com ACL **por SID**, sem BOM, ACL aplicada **antes** da escrita
- [ ] `ssh windows-vm 'hostname'` entra **sem senha**
- [ ] Git, Go, Node, Python instalados; `core.autocrlf false`
- [ ] `python3.exe` criado por cópia; `python3 -c "import sys;print(sys.executable)"` aponta para o real
- [ ] caminho do bundle **anotado** e o `.qcow2` conferido no disco (§7-bis)
- [ ] `go build -o bin/trackfw.exe ./cmd/trackfw` funciona
- [ ] 🔴 **Snapshot tirado**
